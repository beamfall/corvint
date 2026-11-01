package cli_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCTV0991_EveryRequestIDFlagIsLimitedBeforeStoreAccess: every command
// with --request-id states the limit in verbose help, and an over-long ID is
// refused LIMIT_EXCEEDED ahead of every parser, on a root with no store
// (TCT-V0-991, V1-3515).
func TestTCTV0991_EveryRequestIDFlagIsLimitedBeforeStoreAccess(t *testing.T) {
	root := t.TempDir()
	seen := 0
	for _, node := range cli.CommandTree().Nodes {
		if !strings.Contains(strings.Join(node.Flags, " "), "--request-id") {
			continue
		}
		seen++
		path := node.Path
		help := atm(t, root, nil, append(append([]string{}, path...), "--help", "--verbose")...)
		if !strings.Contains(string(help.stdout), `"requestId":"--request-id is the idempotency key: at most 64 bytes`) {
			t.Errorf("%v verbose help lacks requestId: %s", path, help.stdout)
		}
	}
	if seen < 10 {
		t.Fatalf("only %d command-tree nodes take --request-id", seen)
	}
	long := strings.Repeat("r", wire.MaxRequestIDBytes+1)
	for _, args := range [][]string{
		{"ticket", "note", "clear", "V1-0001", "--request-id", long},
		{"ticket", "note", "clear", "V1-0001", "--request-id=" + long},
		{"ticket", "note", "set", "V1-0001", "V1-0002", "--request-id", long, "--text", "x"},
		{"ticket", "create", "--batch", "--request-id", long, "--payload-stdin"},
	} {
		x := atm(t, root, nil, args...)
		w := strings.Join(x.res.Warnings, " ")
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeLimitExceeded) || hasCode(x.res, wire.CodeUninitialized) ||
			!strings.Contains(w, "--request-id is 65 bytes") || !strings.Contains(w, "at most 64 bytes") {
			t.Errorf("%v: %+v", args[:3], x.res)
		}
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("refusals wrote %v (%v)", entries, err)
	}
}
