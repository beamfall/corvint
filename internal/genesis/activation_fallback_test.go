package genesis

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// deadlineOnCancel reports an expired deadline once cancelled, so the test
// can end the activation at a point it chose instead of at a wall-clock
// deadline that host load could move before repository open.
type deadlineOnCancel struct{ context.Context }

func (ctx deadlineOnCancel) Err() error {
	if ctx.Context.Err() != nil {
		return context.DeadlineExceeded
	}
	return nil
}

// TestActivationFallsBackToABoundedReceiptWhenGitHangs pins GENESIS-025's
// fallback: a Git that stops answering after repository open ends the
// activation at the caller's deadline with a PARTIAL receipt that still pins
// revision and tree, names git-timeout and made no model call; the default
// activation budget stays under ten minutes.
func TestActivationFallsBackToABoundedReceiptWhenGitHangs(t *testing.T) {
	if limit := defaultLimits().TotalTimeout; limit >= 10*time.Minute {
		t.Fatalf("GENESIS-025: default activation budget %s is not under ten minutes", limit)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := genesisRepository(t, git, "sha1")
	bin := t.TempDir()
	marker := filepath.Join(bin, "hanging")
	// Repository open answers; every later Git read marks that it hangs, then hangs.
	script := "#!/bin/sh\ncase \" $* \" in *\" rev-parse \"*) exec " + git + " \"$@\" ;; esac\n: > '" + marker + "'\nexec sleep 60\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, activation := range []string{"init", "adopt"} {
		if err := os.RemoveAll(marker); err != nil {
			t.Fatal(err)
		}
		inner, cancel := context.WithCancel(context.Background())
		go func() {
			for inner.Err() == nil {
				if _, err := os.Stat(marker); err == nil {
					cancel()
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
		started := time.Now()
		receipt := CompileRepositoryInventory(deadlineOnCancel{inner}, root, activation, nil, "HEAD", nil)
		elapsed := time.Since(started)
		cancel()
		if elapsed > 20*time.Second {
			t.Fatalf("GENESIS-025 %s: hanging Git held the activation for %s", activation, elapsed)
		}
		gaps, _ := receipt["gaps"].([]any)
		repository, _ := receipt["repository"].(map[string]any)
		model, _ := receipt["model"].(map[string]any)
		if receipt["operationalState"] != "PARTIAL" || len(gaps) != 1 || gaps[0].(map[string]any)["code"] != "git-timeout" ||
			repository["revision"] == nil || repository["tree"] == nil || model["callCount"] != 0 {
			t.Fatalf("GENESIS-025 %s: receipt %#v", activation, receipt)
		}
	}
}
