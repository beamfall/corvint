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
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// GAG-V0-9962: a real loose-build subprocess with a descendant is canceled by
// a middle-target error. The collector joins it before removing suffix outputs.
func TestParallelLooseCancellationReapsBuildAndRemovesSuffix(t *testing.T) {
	t.Setenv("CORVINT_ARCHIVE_WORKERS", "4")
	scratch, err := os.MkdirTemp(os.TempDir(), "gag-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(scratch)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(scratch, "ready"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := listener.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(scratch, "go")
	// The shim runs the helper instead of compiling. buildOnce still owns the
	// real command, process group, environment, cancellation and cleanup path.
	mustWrite(t, shim, "#!/bin/sh\nexec '"+strings.ReplaceAll(os.Args[0], "'", "'\\''")+"' -test.run=^TestTargetWorkerProcessHelper$\n")
	if err := os.Chmod(shim, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", scratch+string(os.PathListSeparator)+os.Getenv("PATH"))
	manifest := workerManifest(t)
	manifest.Profile.Environment["GAG_WORKER_SOCKET"] = listener.Addr().String()
	output := t.TempDir()
	options := Options{Root: scratch, Output: output}
	report := Report{Commit: "commit", build: "1"}
	middleErr := errors.New("injected middle-target error")
	var childPID int
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	err = buildTargetsUsing(ctx, options, manifest, &report, func(ctx context.Context, options Options, manifest Manifest, target Target, local *Report) (TargetReport, error) {
		index := targetIndex(manifest, target)
		if index == 2 {
			connection, err := listener.Accept()
			if err != nil {
				return TargetReport{}, err
			}
			defer connection.Close()
			if err := connection.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
				return TargetReport{}, err
			}
			if _, err := fmt.Fscanln(connection, &childPID); err != nil {
				return TargetReport{}, err
			}
			return TargetReport{}, middleErr
		}
		if index == 3 {
			name := manifest.Profile.BinaryName + "-" + target.GOOS + "-" + target.GOARCH + target.Suffix
			artifact := filepath.Join(output, "build-a", name)
			if err := os.MkdirAll(filepath.Dir(artifact), 0o700); err != nil {
				return TargetReport{}, err
			}
			if err := os.WriteFile(artifact, []byte("partial output"), 0o600); err != nil {
				return TargetReport{}, err
			}
			err := buildOnce(ctx, scratch, manifest, target, "1", "commit", artifact, filepath.Join(output, "cache-a-"+target.GOOS+"-"+target.GOARCH), filepath.Join(output, "tmp-a-"+target.GOOS+"-"+target.GOARCH))
			return TargetReport{}, err
		}
		return TargetReport{GOOS: target.GOOS, GOARCH: target.GOARCH}, nil
	})
	if err != middleErr || len(report.Targets) != 2 || childPID == 0 {
		t.Fatalf("err=%v targets=%d child=%d", err, len(report.Targets), childPID)
	}
	if err := syscall.Kill(childPID, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("descendant %d survived: %v", childPID, err)
	}
	target := manifest.Targets[3]
	for _, name := range []string{"cache-a-" + target.GOOS + "-" + target.GOARCH, "tmp-a-" + target.GOOS + "-" + target.GOARCH, filepath.Join("build-a", manifest.Profile.BinaryName+"-"+target.GOOS+"-"+target.GOARCH+target.Suffix), "SHA256SUMS"} {
		if _, err := os.Stat(filepath.Join(output, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("speculative output %s survived: %v", name, err)
		}
	}
}

func TestTargetWorkerProcessHelper(t *testing.T) {
	socket := os.Getenv("GAG_WORKER_SOCKET")
	if socket == "" {
		return
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	child := exec.Command("sleep", "60")
	if err := child.Start(); err != nil {
		os.Exit(3)
	}
	connection, err := net.Dial("unix", socket)
	if err != nil {
		_ = child.Process.Kill()
		_ = child.Wait()
		os.Exit(4)
	}
	_, err = fmt.Fprintln(connection, strconv.Itoa(child.Process.Pid))
	_ = connection.Close()
	if err != nil {
		_ = child.Process.Kill()
		_ = child.Wait()
		os.Exit(5)
	}
	<-signals
	_ = child.Process.Kill()
	_ = child.Wait()
	os.Exit(0)
}
