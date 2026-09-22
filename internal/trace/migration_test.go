package trace

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyMigrationAtCandidateLimitDoesNotCountOperationLock(t *testing.T) {
	root, authority, legacy, target := migrationAtCandidateLimitFixture(t)

	plan, err := PlanMigration(root, authority, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Candidates) != MaxTraceFiles-1 {
		t.Fatalf("candidate count = %d, want %d", len(plan.Candidates), MaxTraceFiles-1)
	}
	temporary := filepath.Join(root, ".context-corvint", "traces", ".migrate-"+target+".tmp")
	if err := os.WriteFile(temporary, plan.Entries[0].targetBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Run("LTPM-V0-003 LTPM-V0-005 LTPM-V0-006 LTPM-V0-009 valid interrupted temporary resumes at candidate bound", func(t *testing.T) {
		if _, err := ApplyMigration(root, authority, plan.Digest, func() error { return nil }); err != nil {
			t.Fatalf("ApplyMigration() at candidate limit: %v", err)
		}
	})
	if _, err := os.Stat(StorePath(root, target)); err != nil {
		t.Fatalf("canonical target missing: %v", err)
	}
	if _, err := os.Stat(StorePath(root, legacy)); !os.IsNotExist(err) {
		t.Fatalf("legacy source survived: %v", err)
	}
}

func TestPlanMigrationCountsOrphanTemporaryAtCandidateLimit(t *testing.T) {
	t.Run("LTPM-V0-003 orphan migration temporary counts toward candidate bound", func(t *testing.T) {
		root, authority, _, _ := migrationAtCandidateLimitFixture(t)
		orphan := filepath.Join(root, ".context-corvint", "traces", ".migrate-"+strings.Repeat("c", 40)+".tmp")
		if err := os.WriteFile(orphan, []byte("orphan"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := PlanMigration(root, authority, func() error { return nil }); err == nil || !strings.Contains(err.Error(), "trace migration staging exceeds 1000 directory entries") {
			t.Fatalf("PlanMigration() error = %v, want candidate-bound refusal", err)
		}
	})
}

func migrationAtCandidateLimitFixture(t *testing.T) (string, MigrationAuthority, string, string) {
	t.Helper()
	root := t.TempDir()
	authority := MigrationAuthority{
		ObjectFormat: "sha1", ProfileID: "generic",
		Commits: map[string]Revision{}, Trees: map[string][]string{},
	}
	for index := 1; index < MaxTraceFiles-1; index++ {
		revision := fmt.Sprintf("%040x", index)
		tree := fmt.Sprintf("%040x", MaxTraceFiles+index)
		authority.Commits[revision] = Revision{TreeRevision: tree}
		writeTraceFixture(t, root, revision, nil)
	}
	legacy := strings.Repeat("e", 40)
	target := strings.Repeat("d", 40)
	authority.CommitRevision = target
	authority.TreeRevision = legacy
	authority.Commits[target] = Revision{TreeRevision: legacy}
	authority.Trees[legacy] = []string{target}
	record := mustRecord(t, Input{Revision: legacy, Task: "migrate at candidate limit", Outcome: "passed"}, nil)
	row, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	writeTraceFixture(t, root, legacy, row)
	return root, authority, legacy, target
}

func TestMigrationPlanRejectsCandidateLinksWithoutReadingOutside(t *testing.T) {
	revision := strings.Repeat("a", 40)
	tree := strings.Repeat("b", 40)
	authority := MigrationAuthority{
		CommitRevision: revision, TreeRevision: tree, ObjectFormat: "sha1", ProfileID: "generic",
		Commits: map[string]Revision{revision: {TreeRevision: tree}}, Trees: map[string][]string{tree: {revision}},
	}
	for _, test := range []struct {
		name string
		link func(string, string) error
	}{
		{"symlink", func(outside, candidate string) error { return os.Symlink(outside, candidate) }},
		{"hardlink", func(outside, candidate string) error { return os.Link(outside, candidate) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, ".context-corvint", "traces")
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(root, "outside")
			original := []byte("outside bytes")
			if err := os.WriteFile(outside, original, 0o600); err != nil {
				t.Fatal(err)
			}
			candidate := filepath.Join(directory, revision+".jsonl")
			if err := test.link(outside, candidate); err != nil {
				t.Fatal(err)
			}
			if _, err := PlanMigration(root, authority, func() error { return nil }); err == nil || !strings.Contains(err.Error(), "unsafe") {
				t.Fatalf("error=%v", err)
			}
			got, err := os.ReadFile(outside)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatalf("outside bytes=%q error=%v", got, err)
			}
		})
	}
}

func TestMigrationCandidateDriftCheckCoversWholePlan(t *testing.T) {
	root := t.TempDir()
	first := strings.Repeat("1", 40)
	second := strings.Repeat("2", 40)
	authority := MigrationAuthority{
		CommitRevision: first, TreeRevision: strings.Repeat("a", 40), ObjectFormat: "sha1", ProfileID: "generic",
		Commits: map[string]Revision{
			first:  {TreeRevision: strings.Repeat("a", 40)},
			second: {TreeRevision: strings.Repeat("b", 40)},
		},
		Trees: map[string][]string{
			strings.Repeat("a", 40): {first}, strings.Repeat("b", 40): {second},
		},
	}
	for _, revision := range []string{first, second} {
		record := mustRecord(t, Input{Revision: revision, Task: revision, Outcome: "passed"}, nil)
		row, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		writeTraceFixture(t, root, revision, row)
	}
	plan, err := PlanMigration(root, authority, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(StorePath(root, second), []byte("drift\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	directory, err := openTraceDirectory(root, false)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.close()
	if err := confirmPlanCandidates(plan, directory, nil, false); err == nil || !strings.Contains(err.Error(), "candidate drifted") {
		t.Fatalf("error=%v", err)
	}
}

func TestMigrationQuarantineBindingDetectsReplacement(t *testing.T) {
	root := t.TempDir()
	quarantine, err := openPrivateDirectory(root, "legacy-traces", true)
	if err != nil {
		t.Fatal(err)
	}
	defer quarantine.close()
	corvint := filepath.Join(root, ".context-corvint")
	if err := os.Rename(filepath.Join(corvint, "legacy-traces"), filepath.Join(corvint, "legacy-traces.old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(corvint, "legacy-traces"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := quarantine.confirm(); err == nil || !strings.Contains(err.Error(), "changed while pinned") {
		t.Fatalf("error=%v", err)
	}
}
