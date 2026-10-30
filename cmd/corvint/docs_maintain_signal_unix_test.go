//go:build unix

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func TestDocsMaintainWatchSignalHelper(t *testing.T) {
	root := os.Getenv("CORVINT_DOCS_WATCH_SIGNAL_ROOT")
	if root == "" {
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer cancel()
	options := docsMaintainOptions{root: root, page: "docs/page.md", source: "owner.md", pkg: "cache", enable: true, apply: true, watch: true, maxWrites: 8, maxWallClock: time.Minute}
	os.Exit(runDocsMaintain(ctx, options, os.Stdout, os.Stderr))
}

func TestDocsMaintainWatchSignalJoinsGitDescendants(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			root := docsRepository(t)
			if err := os.MkdirAll(filepath.Join(root, "docs"), 0700); err != nil {
				t.Fatal(err)
			}
			wrapper := t.TempDir()
			pidFile := filepath.Join(wrapper, "pids")
			script := "#!/bin/sh\nsleep 30 &\nchild=$!\ntrap 'kill \"$child\" 2>/dev/null; wait \"$child\" 2>/dev/null' EXIT INT TERM\nprintf '%s %s' \"$$\" \"$child\" > '" + pidFile + "'\nwait \"$child\"\n"
			if err := os.WriteFile(filepath.Join(wrapper, "git"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(os.Args[0], "-test.run=^TestDocsMaintainWatchSignalHelper$")
			command.Env = append(os.Environ(), "CORVINT_DOCS_WATCH_SIGNAL_ROOT="+root, "PATH="+wrapper+string(os.PathListSeparator)+os.Getenv("PATH"))
			command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			joined := false
			pids := []int{}
			t.Cleanup(func() {
				if !joined {
					_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
					<-done
				}
				for _, pid := range pids {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
			})
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				data, _ := os.ReadFile(pidFile)
				fields := strings.Fields(string(data))
				if len(fields) == 2 {
					for _, field := range fields {
						pid, err := strconv.Atoi(field)
						if err != nil {
							t.Fatal(err)
						}
						pids = append(pids, pid)
					}
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if len(pids) != 2 {
				t.Fatal("bounded Git child never started")
			}
			if err := command.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				joined = true
				if err != nil {
					t.Fatalf("watch exit: %v %s", err, stderr.String())
				}
			case <-time.After(5 * time.Second):
				t.Fatal("watch did not stop")
			}
			var receipt struct {
				StoppedReason string `json:"stopped_reason"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil || receipt.StoppedReason != "interrupted" {
				t.Fatalf("receipt=%s error=%v", stdout.String(), err)
			}
			for _, pid := range pids {
				deadline = time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) && syscall.Kill(pid, 0) == nil {
					time.Sleep(10 * time.Millisecond)
				}
				if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
					t.Fatal(fmt.Sprintf("descendant %d survived: %v", pid, err))
				}
			}
		})
	}
}
