//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRunCommandPreservesExitShapes(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []int{0, 23, -1} {
		t.Run("CRT-V0-011 "+strconv.Itoa(code), func(t *testing.T) {
			t.Setenv("TRIAL_COMMAND_MODE", "status")
			t.Setenv("TRIAL_COMMAND_STATUS", strconv.Itoa(code))
			stdout, got, stderr, err := trialRunCommand(context.Background(), t.TempDir(), 5*time.Second, executable, "-test.run=^TestTrialProcessHelper$")
			if err != nil || got != code || string(stdout) != "stdout\x00\n" {
				t.Fatalf("result: exit=%d stdout=%q error=%v", got, stdout, err)
			}
			if trialCapturesStderr && stderr != "stderr" {
				t.Fatalf("stderr = %q", stderr)
			}
		})
	}
}

func TestRunCommandNormalizesPathsAndEnvironment(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(executable, filepath.Join(directory, "trial-helper")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TRIAL_COMMAND_MODE", "environment")
	t.Setenv("TRIAL_VALUE", "explicit inherited value")
	for _, item := range []struct{ name, root, executable, directory string }{
		{"absolute", directory, executable, directory},
		{"absolute-unclean-executable", directory, directory + "/./trial-helper", directory},
		{"relative-root", relative, executable, directory},
		{"root-relative-executable", directory, "./trial-helper", directory},
		{"PATH-executable", directory, "trial-helper", directory},
		{"empty-root", "", executable, cwd},
	} {
		t.Run("CRT-V0-011 "+item.name, func(t *testing.T) {
			stdout, code, _, err := trialRunCommand(context.Background(), item.root, 5*time.Second, item.executable, "-test.run=^TestTrialProcessHelper$")
			if err != nil || code != 0 {
				t.Fatalf("exit=%d error=%v", code, err)
			}
			var result struct {
				Directory, Value string
				Input            int
			}
			if err := json.Unmarshal(stdout, &result); err != nil {
				t.Fatal(err)
			}
			if result.Directory != item.directory || result.Value != "explicit inherited value" || result.Input != 0 {
				t.Fatalf("directory/environment/EOF changed: %+v", result)
			}
		})
	}
}

func TestRunCommandRefusesBeforeDispatch(t *testing.T) {
	for _, trigger := range []string{"cancelled", "zero-timeout", "launch-failure"} {
		t.Run("CRT-V0-011 "+trigger, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			timeout := time.Second
			want := "does-not-exist"
			if trigger == "cancelled" {
				cancel()
				want = "context canceled after 1s"
			}
			if trigger == "zero-timeout" {
				timeout = 0
				want = "context deadline exceeded after 0s"
			}
			stdout, code, stderr, err := trialRunCommand(ctx, t.TempDir(), timeout, "./does-not-exist")
			if err == nil || !strings.Contains(err.Error(), want) || stdout != nil || code != 0 || stderr != "" {
				t.Fatalf("refusal: stdout=%q exit=%d stderr=%q error=%v", stdout, code, stderr, err)
			}
		})
	}
}
