package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/workqueue"
)

// WQO-V0-006, WQO-V0-034: retained profiles cannot stand in for raw transcripts.
func TestWorkLimitedBufferHashesAllObservedBytes(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-006", func(t *testing.T) {
		buffer := &workLimitedBuffer{limit: 5}
		overflows := 0
		buffer.overrun = func() { overflows++ }
		chunks := [][]byte{[]byte("abc"), []byte("defghi"), []byte("jkl\n")}
		for _, chunk := range chunks {
			_, _ = buffer.Write(chunk)
		}
		count, digest := buffer.rawIdentity()
		want := sha256.Sum256(bytes.Join(chunks, nil))
		if count != 13 || digest != hex.EncodeToString(want[:]) || string(buffer.data) != "abcde" || overflows != 1 {
			t.Fatalf("raw identity/retention: %d %s %q %d", count, digest, buffer.data, overflows)
		}
	})
}

func workTestRunner(t *testing.T, ctx context.Context, raw string) *workAdapterRunner {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "adapter")
	if err := os.WriteFile(path, []byte(raw), 0755); err != nil {
		t.Fatal(err)
	}
	runner, err := newWorkAdapterRunner(ctx, root, path, workSource{adapterRaw: []byte(raw), adapterMode: "100755", adapterOID: strings.Repeat("a", 40)}, &workqueue.Policy{ID: "test-policy"})
	if err != nil {
		t.Fatal(err)
	}
	runner.env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC", "NO_COLOR=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "HOME=" + root, "TMPDIR=" + root}
	t.Cleanup(runner.Close)
	return runner
}

// WQO-V0-006 and incorporated VPO-V0-024: identify the actual interpreter.
func TestWorkRunnerInterpreterQualification(t *testing.T) {
	t.Run("WQO-V0-006", func(t *testing.T) {
		for _, interpreter := range []string{"/bin/sh", "/bin/bash"} {
			t.Run(filepath.Base(interpreter), func(t *testing.T) {
				if _, err := os.Stat(interpreter); err != nil {
					t.Skip(err)
				}
				runner := workTestRunner(t, context.Background(), "#!"+interpreter+"\nprintf 'ok\\n'\n")
				raw, receipt, err := runner.run("snapshot", []string{}, 100)
				if err != nil {
					t.Fatal(err)
				}
				actual, err := os.ReadFile(interpreter)
				if err != nil {
					t.Fatal(err)
				}
				if string(raw) != "ok\n" || receipt.ExecutableQualification != "UNQUALIFIED" || len(receipt.InterpreterChain) != 1 || receipt.InterpreterChain[0].FileSHA256 != workqueue.SHA256Hex(actual) || receipt.InterpreterChain[0].PathSHA256 != workqueue.SHA256Hex([]byte(interpreter)) {
					t.Fatalf("receipt: %+v", receipt)
				}
			})
		}
		for _, line := range []string{"#!/usr/bin/env sh", "#!sh", "#!/bin/sh -e -u", "garbage"} {
			t.Run(line, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "adapter")
				raw := []byte(line + "\nexit 0\n")
				if err := os.WriteFile(path, raw, 0755); err != nil {
					t.Fatal(err)
				}
				runner, err := newWorkAdapterRunner(context.Background(), filepath.Dir(path), path, workSource{adapterRaw: raw, adapterMode: "100755"}, &workqueue.Policy{})
				if err == nil {
					runner.Close()
					t.Fatal("unsupported interpreter accepted")
				}
			})
		}
		t.Run("native", func(t *testing.T) {
			raw, err := os.ReadFile("/bin/echo")
			if err != nil {
				t.Fatal(err)
			}
			runner, err := newWorkAdapterRunner(context.Background(), t.TempDir(), "/bin/echo", workSource{adapterRaw: raw, adapterMode: "100755"}, &workqueue.Policy{})
			if err != nil {
				t.Fatal(err)
			}
			defer runner.Close()
			runner.env = []string{"PATH=/usr/bin:/bin"}
			_, receipt, err := runner.run("snapshot", []string{"native"}, 100)
			if err != nil || receipt.ExecutableQualification != "UNQUALIFIED" || len(receipt.InterpreterChain) != 0 {
				t.Fatalf("native: %+v %v", receipt, err)
			}
		})
		t.Run("envSymlink", func(t *testing.T) {
			root := t.TempDir()
			interpreter := filepath.Join(root, "launcher")
			if err := os.Symlink("/usr/bin/env", interpreter); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "adapter")
			raw := []byte("#!" + interpreter + " sh\nexit 0\n")
			if err := os.WriteFile(path, raw, 0755); err != nil {
				t.Fatal(err)
			}
			runner, err := newWorkAdapterRunner(context.Background(), root, path, workSource{adapterRaw: raw, adapterMode: "100755"}, &workqueue.Policy{})
			if err == nil {
				runner.Close()
				t.Fatal("env symlink accepted")
			}
		})
	})
}

// WQO-V0-006/034: pre- and post-execution path changes fail closed.
func TestWorkRunnerExecutableSwap(t *testing.T) {
	t.Run("before", func(t *testing.T) {
		runner := workTestRunner(t, context.Background(), "#!/bin/sh\nprintf ok\n")
		if err := os.Rename(runner.path, runner.path+".old"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(runner.path, runner.source.adapterRaw, 0755); err != nil {
			t.Fatal(err)
		}
		_, receipt, err := runner.run("snapshot", []string{}, 100)
		if err == nil || receipt.State != "INCOMPLETE" || receipt.ExitCode != nil {
			t.Fatalf("swap: %+v %v", receipt, err)
		}
	})
	t.Run("during", func(t *testing.T) {
		runner := workTestRunner(t, context.Background(), "#!/bin/sh\nprintf x >> \"$0\"\nprintf ok\n")
		_, receipt, err := runner.run("snapshot", []string{}, 100)
		if err == nil || receipt.State != "INCOMPLETE" {
			t.Fatalf("swap: %+v %v", receipt, err)
		}
	})
	t.Run("symlink", func(t *testing.T) {
		runner := workTestRunner(t, context.Background(), "#!/bin/sh\nprintf ok\n")
		if err := os.Rename(runner.path, runner.path+".old"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(runner.path+".old", runner.path); err != nil {
			t.Fatal(err)
		}
		_, receipt, err := runner.run("snapshot", []string{}, 100)
		if err == nil || receipt.ExitCode != nil {
			t.Fatalf("symlink swap: %+v %v", receipt, err)
		}
	})
	t.Run("interpreter", func(t *testing.T) {
		directory := t.TempDir()
		interpreter := filepath.Join(directory, "interpreter")
		raw, err := os.ReadFile("/bin/sh")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(interpreter, raw, 0755); err != nil {
			t.Fatal(err)
		}
		runner := workTestRunner(t, context.Background(), "#!"+interpreter+"\nprintf ok\n")
		if err := os.Rename(interpreter, interpreter+".old"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(interpreter, raw, 0755); err != nil {
			t.Fatal(err)
		}
		_, receipt, err := runner.run("snapshot", []string{}, 100)
		if err == nil || receipt.ExitCode != nil || receipt.State != "INCOMPLETE" {
			t.Fatalf("interpreter swap: %+v %v", receipt, err)
		}
	})
	t.Run("interpreterDuring", func(t *testing.T) {
		directory := t.TempDir()
		interpreter := filepath.Join(directory, "interpreter")
		if err := os.Symlink("/bin/sh", interpreter); err != nil {
			t.Fatal(err)
		}
		// Keep the old link allocated: Linux file systems reuse a freed inode number at once,
		// which would make an rm-then-ln replacement indistinguishable to os.SameFile.
		runner := workTestRunner(t, context.Background(), "#!"+interpreter+"\nmv '"+interpreter+"' '"+interpreter+".old'\nln -s /bin/sh '"+interpreter+"'\nprintf ok\n")
		_, receipt, err := runner.run("snapshot", []string{}, 100)
		if err == nil || receipt.State != "INCOMPLETE" {
			t.Fatalf("interpreter drift: %+v %v", receipt, err)
		}
	})
}

// WQO-V0-006/032/034: errors cannot retain a fabricated successful exit.
func TestWorkRunnerTranscripts(t *testing.T) {
	t.Run("WQO-V0-006", func(t *testing.T) {
		for _, tc := range []struct {
			name, body, state string
			limit             int
			wantErr           bool
		}{
			{"successLF", "printf 'ok\\n'", "PASSED", 100, false},
			{"nonzero", "exit 7", "FAILED", 100, true},
			{"signal", "kill -TERM $$", "INCOMPLETE", 100, true},
			{"overflow", "printf '0123456789\\n'", "INCOMPLETE", 5, true},
			{"closedStdin", "if read line; then exit 9; fi", "PASSED", 100, false},
			{"descendantPipe", "sleep 30 &\nexit 0", "INCOMPLETE", 100, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				runner := workTestRunner(t, context.Background(), "#!/bin/sh\n"+tc.body+"\n")
				start := time.Now()
				_, receipt, err := runner.run("snapshot", []string{}, tc.limit)
				if time.Since(start) > 3*time.Second {
					t.Fatal("unbounded runner")
				}
				if (err != nil) != tc.wantErr || receipt.State != tc.state {
					t.Fatalf("receipt: %+v error %v", receipt, err)
				}
				if tc.name == "signal" && (receipt.Signal == nil || receipt.ExitCode != nil) {
					t.Fatalf("signal evidence: %+v", receipt)
				}
				if tc.name == "successLF" && (receipt.StdoutBytes != 3 || receipt.StdoutRawSHA256 != workqueue.SHA256Hex([]byte("ok\n"))) {
					t.Fatalf("LF hash: %+v", receipt)
				}
				if tc.name == "overflow" && receipt.StdoutBytes != 11 {
					t.Fatalf("truncated count: %+v", receipt)
				}
			})
		}
		t.Run("cancel", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			runner := workTestRunner(t, ctx, "#!/bin/sh\nsleep 30\n")
			_, receipt, err := runner.run("snapshot", []string{}, 100)
			if !errors.Is(err, context.DeadlineExceeded) || receipt.State != "INCOMPLETE" {
				t.Fatalf("cancel: %+v %v", receipt, err)
			}
		})
		t.Run("launchFailure", func(t *testing.T) {
			runner := workTestRunner(t, context.Background(), "#!/bin/sh\nexit 0\n")
			runner.root = filepath.Join(runner.root, "missing")
			_, receipt, err := runner.run("snapshot", []string{}, 100)
			if err == nil || receipt.ExitCode != nil || receipt.Signal != nil || receipt.State != "INCOMPLETE" {
				t.Fatalf("start: %+v %v", receipt, err)
			}
		})
		t.Run("aggregate", func(t *testing.T) {
			runner := workTestRunner(t, context.Background(), "#!/bin/sh\nprintf abc\n")
			runner.total = workAggregateLimit - 2
			_, receipt, err := runner.run("snapshot", []string{}, 100)
			if err == nil || err.Error() != "limit" || receipt.State != "INCOMPLETE" || receipt.StdoutBytes != 3 {
				t.Fatalf("aggregate: %+v %v", receipt, err)
			}
		})
		t.Run("stderrOverflow", func(t *testing.T) {
			runner := workTestRunner(t, context.Background(), "#!/bin/sh\ni=0\nwhile [ \"$i\" -lt 12000 ]; do printf '"+strings.Repeat("x", 100)+"' >&2; i=$((i + 1)); done\n")
			_, receipt, err := runner.run("snapshot", []string{}, 100)
			if err == nil || err.Error() != "limit" || receipt.State != "INCOMPLETE" || receipt.StderrBytes <= workStderrLimit {
				t.Fatalf("stderr: %+v %v", receipt, err)
			}
		})
		t.Run("freshProcess", func(t *testing.T) {
			runner := workTestRunner(t, context.Background(), "#!/bin/sh\nprintf '%s' $$\n")
			first, _, err := runner.run("snapshot", []string{}, 100)
			if err != nil {
				t.Fatal(err)
			}
			second, _, err := runner.run("details", []string{}, 100)
			if err != nil || bytes.Equal(first, second) {
				t.Fatalf("process reuse: %q %q %v", first, second, err)
			}
		})
		t.Run("targetVerification", func(t *testing.T) {
			runner := workTestRunner(t, context.Background(), "#!/bin/sh\nprintf ok\n")
			checks := 0
			runner.verifyTarget = func(context.Context) error {
				checks++
				if checks == 2 {
					return errors.New("target changed")
				}
				return nil
			}
			_, receipt, err := runner.run("snapshot", []string{}, 100)
			if err == nil || checks != 2 || receipt.State != "INCOMPLETE" {
				t.Fatalf("target: %+v %v checks=%d", receipt, err, checks)
			}
		})
	})
}

// WQO-V0-006/032 and section 4.4: wall-time overflow is the closed input limit.
func TestWorkRunnerTerminalCommandMapping(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-015", func(t *testing.T) {
		runner := &workAdapterRunner{}
		if runner.commandError(context.DeadlineExceeded) != "INPUT_LIMIT" {
			t.Fatal("wall-time bound mapping")
		}
		if runner.commandError(context.Canceled) != "CANCELLED" {
			t.Fatal("caller cancellation mapping")
		}
		if runner.commandError(fmt.Errorf("%w: changed", errWorkBoundExecutableUnqualified)) != "SOURCE_UNQUALIFIED" {
			t.Fatal("bound executable drift mapping")
		}
	})
}

// workAssertFinalError is platform-neutral (no OS-specific behavior); it is used by both the
// unix-tagged work_final_check_test.go and this untagged file, so it lives here.
func workAssertFinalError(t *testing.T, output []byte, exit int, code string) {
	t.Helper()
	want := &workqueue.CommandResult{State: "ERROR", ErrorCode: &code}
	workqueue.RefreshCommandResult(want)
	if exit != 2 || !bytes.Equal(output, want.Canonical()) {
		t.Fatalf("want exit 2 canonical ERROR/%s with null payloads; got exit %d %s", code, exit, output)
	}
}

// WQO-V0-032 and section 4.4: the wall-time bound is a hang detector, so it is generous and
// its expiry says so on stderr while stdout keeps the closed INPUT_LIMIT result.
func TestWorkHangDetectorNamesItsCause(t *testing.T) {
	t.Run("WQO-V0-032", func(t *testing.T) {
		if workHangBound < 5*time.Minute {
			t.Fatalf("wall-time bound %s is a budget, not a hang detector", workHangBound)
		}
		root := workProductionFixture(t)
		original := workHangBound
		workHangBound = time.Nanosecond
		t.Cleanup(func() { workHangBound = original })
		var output, stderr bytes.Buffer
		exit := runWork(context.Background(), root, []string{"observe"}, &output, &stderr)
		workAssertFinalError(t, output.Bytes(), exit, "INPUT_LIMIT")
		if !strings.Contains(stderr.String(), "hang detector; no input bound was exceeded") {
			t.Fatalf("expiry does not name the hang detector: %q", &stderr)
		}
	})
}
