package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
)

// WQO-V0-095: the private executable copy root is the checked scratch parent (the
// CORVINT_WORK_RUN_PARENT override in tests), never the fixed /tmp.
func TestWorkScratchRootFollowsOverride(t *testing.T) {
	override := os.Getenv("CORVINT_WORK_RUN_PARENT")
	if override == "" {
		t.Fatal("test environment lacks the scratch parent override")
	}
	want, err := filepath.EvalSymlinks(override)
	if err != nil {
		t.Fatal(err)
	}
	got, err := workScratchRoot()
	if err != nil || got != want {
		t.Fatalf("workScratchRoot = %q, %v; want %q", got, err, want)
	}
}

// WQO-V0-095: the standalone work-queue script honors a safe override and refuses an
// unsafe one before building anything.
func TestWorkScriptScratchParentOverride(t *testing.T) {
	t.Parallel()
	script, err := filepath.Abs(filepath.Join("..", "..", "script/corvint-work-queue"))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"private", "relative", "open-mode"} {
		t.Run(kind, func(t *testing.T) {
			caller := materializationFixture(t)
			parent, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			override := filepath.Join(parent, "scratch")
			if err := os.Mkdir(override, 0700); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "relative":
				override = "scratch"
			case "open-mode":
				if err := os.Chmod(override, 0755); err != nil {
					t.Fatal(err)
				}
			}
			fake := t.TempDir()
			witness := filepath.Join(fake, "build-storage")
			body := "#!/bin/bash\nprintf '%s\\n' \"$TMPDIR\" > " + witness + "\nexit 7\n"
			if err := execfile.WriteFile(filepath.Join(fake, "go"), []byte(body), 0755); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(script, "snapshot")
			command.Dir = caller
			command.Env = []string{"PATH=" + fake + ":/usr/bin:/bin:/usr/local/bin", "HOME=" + t.TempDir(), "CORVINT_WORK_RUN_PARENT=" + override}
			command.WaitDelay = 10 * time.Second
			workContain(command)
			t.Cleanup(func() { workKillGroup(command) })
			_ = command.Run()
			workKillGroup(command)
			raw, err := os.ReadFile(witness)
			if kind != "private" {
				if err == nil {
					t.Fatalf("unsafe override launched compiler in %s", raw)
				}
				return
			}
			if err != nil {
				t.Fatal("compiler did not run", err)
			}
			if got := strings.TrimSpace(string(raw)); !strings.HasPrefix(got, override+"/") {
				t.Fatalf("build scratch %s not under override %s", got, override)
			}
		})
	}
}
