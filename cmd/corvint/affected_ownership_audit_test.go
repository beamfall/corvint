package main

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// AZU-V0-002 (V1-4685): `affected --ownership-audit` lists the tracked paths
// no claimant or declaration owns, exits 1 while that list is non-empty,
// writes nothing, and refuses a dirty worktree or an extra argument.
func TestAffectedOwnershipAuditListsUnownedReadOnly_AZU_V0_002(t *testing.T) {
	t.Parallel()
	root := compileAdviceRepository(t)
	before := affectedStatusSnapshot(t, root)
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "affected", "--ownership-audit"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr %s", code, stderr.String())
	}
	var report affectedOwnershipAuditReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if report.Profile != affectedOwnershipAuditProfile || report.Tracked != 11 || report.OK || report.Mutates || len(report.Unowned) == 0 {
		t.Fatalf("report %+v", report)
	}
	if !slices.Contains(report.Unowned, "testdata/fake/fake.txt") || slices.Contains(report.Unowned, "tested/tested.go") {
		t.Fatalf("unowned %q: want the fixture data file and no Go source", report.Unowned)
	}
	if after := affectedStatusSnapshot(t, root); after != before {
		t.Fatalf("the audit changed the worktree:\nbefore %s\nafter %s", before, after)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runContext(context.Background(), []string{"--root", root, "affected", "--ownership-audit", "--full"}, strings.NewReader(""), &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "takes no other argument") {
		t.Fatalf("extra argument: exit %d, stderr %s", code, stderr.String())
	}

	cemWrite(t, root, "tested/tested.go", "package tested\n\nconst X = 1\n")
	stdout.Reset()
	stderr.Reset()
	if code := runContext(context.Background(), []string{"--root", root, "affected", "--ownership-audit"}, strings.NewReader(""), &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "MIXED worktree") {
		t.Fatalf("dirty worktree: exit %d, stderr %s", code, stderr.String())
	}
}
