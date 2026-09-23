package genesis

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

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
	// Repository open answers; every later Git read hangs.
	script := "#!/bin/sh\ncase \" $* \" in *\" rev-parse \"*) exec " + git + " \"$@\" ;; esac\nexec sleep 60\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, activation := range []string{"init", "adopt"} {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		started := time.Now()
		receipt := CompileRepositoryInventory(ctx, root, activation, nil, "HEAD", nil)
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
