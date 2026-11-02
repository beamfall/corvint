// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
	"github.com/Beamfall/corvint/internal/trace"
)

// TestDWCV0003PreflightRefusalPrintsOneCommand (V1-4318): when the pre-flight
// still refuses, here a stale sidecar and a stranded trace whose plan also
// rekeys a reachable legacy trace, it prints exactly one fix line that pastes
// as one command: the sidecar move and the migrate-traces apply joined by &&.
// It still names the reset case and its recovery (DCW-V0-844), notes no
// stranded trace, and the sidecar part runs as printed.
func TestDWCV0003PreflightRefusalPrintsOneCommand(t *testing.T) {
	root, stale, run := staleSidecarLane(t)
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(".corvint/change.ocm*\n.context-corvint/\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	tree := testGit(t, root, "rev-parse", "HEAD^{tree}")
	stranded := testGit(t, root, "commit-tree", "-m", "reset away", tree)
	traces := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(traces, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, revision := range []string{stranded, tree} {
		record, err := trace.NewRecord(trace.Input{Producer: trace.ProducerCLI, Revision: revision, Task: "preflight", Outcome: "passed"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		row, err := trace.Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(traces, revision+".jsonl"), row, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	code, calls, out := run(true)
	var fixes []string
	for _, line := range strings.Split(out, "\n") {
		if fix, ok := strings.CutPrefix(line, "  fix: "); ok {
			fixes = append(fixes, fix)
		}
	}
	one := regexp.MustCompile(`^mkdir -p '[^']*/corvint/stale-ocm' && mv \.corvint/change\.ocm-status\.json '[^']*/corvint/stale-ocm'/ && corvint migrate-traces --apply --plan-digest [0-9a-f]{64}$`)
	if code != 2 || calls != 0 || !strings.HasPrefix(out, "dogfood-inputs-check: REFUSE reused-worktree-state\n") ||
		len(fixes) != 1 || !one.MatchString(fixes[0]) ||
		!strings.Contains(out, "or this lane's own bind/seal commits reset away before push") ||
		!strings.Contains(out, "one-step recovery after a reset of this lane's own unpushed bind/seal commits (DCW-V0-844)") ||
		strings.Contains(out, "NOTE stranded trace") {
		t.Fatalf("exit=%d step calls=%d fix lines=%q stderr=%q, want one refusal with one copy-paste fix line", code, calls, fixes, out)
	}
	testplatform.SkipPOSIXShell(t, "the printed fix line is a POSIX shell command")
	move, _, _ := strings.Cut(fixes[0], " && corvint ")
	cmd := exec.Command("sh", "-c", move)
	cmd.Dir = root
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the printed move %q failed: %v\n%s", move, err, data)
	}
	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatalf("the printed move left %s: %v", stale, err)
	}
}
