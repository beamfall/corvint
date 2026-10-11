package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// requireCleanHonouring names the verbs that implement --require-clean
// (DIRTY-CACHE-015, AFP-V0-045). Every other verb must refuse it.
var requireCleanHonouring = map[string]bool{"impact": true, "affected": true}

// DIRTY-CACHE-016 (V1-1174): every dispatched verb, every prove mode and the
// feature form either honours --require-clean or refuses it with exit 2
// and a typed diagnostic; none accepts and ignores it.
func TestRequireCleanHonouredOrRefusedByEveryCommand(t *testing.T) {
	root := impactCLIRepository(t)
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	invoke := func(args ...string) (int, string, string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		var stdout, stderr bytes.Buffer
		code := runContext(ctx, append([]string{"--root", root}, args...), &forbiddenImpactReader{}, &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	refuses := func(args ...string) {
		t.Helper()
		before := treeDigest(t, root)
		code, stdout, stderr := invoke(args...)
		// Exit 2 with a diagnostic and no success document: most verbs report
		// invalid-arguments; work emits its ERROR result document and a few
		// verbs report their own input-error code.
		failed := stdout == "" || strings.Contains(stdout, `"state":"ERROR"`)
		if code != 2 || !failed || strings.TrimSpace(stdout+stderr) == "" {
			t.Errorf("%v: exit=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
		if treeDigest(t, root) != before {
			t.Errorf("%v wrote repository state", args)
		}
	}
	seen := 0
	for _, command := range topLevelCommands {
		if requireCleanHonouring[command] {
			seen++
			continue
		}
		refuses(command, "--require-clean")
	}
	if seen != len(requireCleanHonouring) {
		t.Fatalf("honouring verbs %v are not all dispatched", requireCleanHonouring)
	}
	// The forms the V1-1174 report named, and each prove mode routed through
	// the shared impact parser or another parser.
	refuses("feature", "impact", "--require-clean")
	refuses("prove", "pkg/main.go", "--require-clean")
	refuses("prove", "--task", "change main", "--require-clean")
	refuses("prove", "--cem", "change.cem.json", "--require-clean")
	refuses("prove", "--checkpoint", "checkpoint.json", "--require-clean")

	// The honouring verbs act on the dirty worktree instead of ignoring the flag.
	if code, stdout, _ := invoke("impact", "--require-clean", "pkg/main.go"); code != 0 || !strings.Contains(stdout, `"reason":"mixed-worktree"`) {
		t.Errorf("impact did not abstain: exit=%d stdout=%s", code, stdout)
	}
	if code, stdout, stderr := invoke("affected", "--require-clean"); code != 2 || stdout != "" || !strings.Contains(stderr, "unsupported-affected-mixed-worktree") {
		t.Errorf("affected did not refuse: exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}
