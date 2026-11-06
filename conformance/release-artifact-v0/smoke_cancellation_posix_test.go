//go:build unix

// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// GAG-V0-9962: inherited Git configuration can run hooks in both fixture
// creation and status. Canceling a suffix must retire their pipe-holding
// descendants before the collector joins and removes speculative outputs.
func TestParallelLooseCancellationReapsSmokeGitAndRemovesSuffix(t *testing.T) {
	for _, operation := range []string{"fixture", "status"} {
		t.Run(operation, func(t *testing.T) {
			t.Setenv("CORVINT_ARCHIVE_WORKERS", "2")
			// Keep the Unix socket path below the platform length limit.
			scratch, err := os.MkdirTemp(os.TempDir(), "gsg-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(scratch)
			listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(scratch, "ready"), Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			if err := listener.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GAG_SMOKE_GIT_SOCKET", listener.Addr().String())
			hook := filepath.Join(scratch, "pre-commit")
			mustWrite(t, hook, "#!/bin/sh\nexec '"+strings.ReplaceAll(os.Args[0], "'", "'\\''")+"' -test.run=^TestSmokeGitHookProcessHelper$\n")
			if err := os.Chmod(hook, 0o700); err != nil {
				t.Fatal(err)
			}
			manifest := workerManifest(t)
			native := Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
			manifest.Targets = []Target{{GOOS: "windows", GOARCH: "amd64", Suffix: ".exe"}, native}
			output := t.TempDir()
			suffix := native.GOOS + "-" + native.GOARCH
			workspace := filepath.Join(output, "tmp-a-"+suffix)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			var fixture string
			if operation == "status" {
				fixture, err = makeQueryFixture(ctx, output, manifest.Smoke)
				if err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("GIT_CONFIG_COUNT", "1")
			if operation == "fixture" {
				t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
				t.Setenv("GIT_CONFIG_VALUE_0", scratch)
			} else {
				t.Setenv("GIT_CONFIG_KEY_0", "core.fsmonitor")
				t.Setenv("GIT_CONFIG_VALUE_0", hook)
			}
			ready := make(chan struct{})
			done := make(chan error, 1)
			report := Report{}
			firstErr := errors.New("injected earlier-target error")
			artifact := filepath.Join(output, "build-a", manifest.Profile.BinaryName+"-"+suffix)
			cache := filepath.Join(output, "cache-a-"+suffix)
			go func() {
				done <- buildTargetsUsing(ctx, Options{Output: output}, manifest, &report,
					func(ctx context.Context, options Options, manifest Manifest, target Target, local *Report) (TargetReport, error) {
						if targetIndex(manifest, target) == 0 {
							waitForWorker(ctx, ready)
							return TargetReport{}, firstErr
						}
						for _, directory := range []string{filepath.Dir(artifact), cache, workspace} {
							if err := os.MkdirAll(directory, 0o700); err != nil {
								return TargetReport{}, err
							}
						}
						if err := os.WriteFile(artifact, []byte("speculative binary"), 0o600); err != nil {
							return TargetReport{}, err
						}
						if operation == "fixture" {
							_, err := makeQueryFixture(ctx, output, manifest.Smoke)
							return TargetReport{}, err
						}
						_, err := gitStatus(ctx, fixture)
						return TargetReport{}, err
					})
			}()
			connection, err := listener.Accept()
			if err != nil {
				close(ready)
				t.Fatal(err)
			}
			defer connection.Close()
			if err := connection.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
				t.Fatal(err)
			}
			var hookPID, childPID int
			if _, err := fmt.Fscanln(connection, &hookPID, &childPID); err != nil {
				t.Fatal(err)
			}
			// A broken cancellation path must also leave this test bounded: the
			// helper handles TERM by reaping its child, even if Git already died.
			defer syscall.Kill(hookPID, syscall.SIGTERM)
			close(ready)
			select {
			case err := <-done:
				if err != firstErr || len(report.Targets) != 0 {
					t.Fatalf("err=%v targets=%d", err, len(report.Targets))
				}
			case <-time.After(10 * time.Second):
				_ = syscall.Kill(hookPID, syscall.SIGTERM)
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Error("workers did not retire after emergency hook cleanup")
				}
				t.Fatal("suffix cancellation did not join the smoke Git command")
			}
			if err := syscall.Kill(childPID, 0); !errors.Is(err, syscall.ESRCH) {
				t.Fatalf("pipe-holding hook descendant %d survived: %v", childPID, err)
			}
			for _, path := range []string{artifact, cache, workspace, filepath.Join(output, "smoke-fixture"), filepath.Join(output, "SHA256SUMS")} {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("speculative output %s survived: %v", path, err)
				}
			}
		})
	}
}

func TestSmokeGitHookProcessHelper(t *testing.T) {
	socket := os.Getenv("GAG_SMOKE_GIT_SOCKET")
	if socket == "" {
		return
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	child := exec.Command("sleep", "60")
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		os.Exit(3)
	}
	connection, err := net.Dial("unix", socket)
	if err != nil {
		_ = child.Process.Kill()
		_ = child.Wait()
		os.Exit(4)
	}
	_, err = fmt.Fprintln(connection, os.Getpid(), child.Process.Pid)
	_ = connection.Close()
	if err == nil {
		<-signals
	}
	_ = child.Process.Kill()
	_ = child.Wait()
	os.Exit(0)
}
