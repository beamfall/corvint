package dogfoodflow

import (
	"strings"
	"testing"
)

// migrate commits a fixture migration: the named sealed CEMs rewritten in
// place, appended to cemMigrationCommits for the test.
func (r *batchRepo) migrate(binds ...string) string {
	r.t.Helper()
	for _, bind := range binds {
		r.write(".corvint/changes/"+bind+".cem.json", `{"spec": "cem/1.0", "migrated": "`+bind+`"}`+"\n")
	}
	commit := r.commit("migrate committed CEM maps to cem/1.0")
	previous := cemMigrationCommits
	cemMigrationCommits = append(append([]string{}, previous...), commit)
	r.t.Cleanup(func() { cemMigrationCommits = previous })
	return commit
}

// DCW-V0-050: the migration commit is the seal baseline. Its rewrites of
// sealed CEMs BASE already had are admitted (not constituents, not counted)
// and the commit itself is bound; a later edit of a migrated map is
// sealed-cem-modified; a constituent the batch adds before the migration
// rewrote it fails closed with the split-the-batch recovery.
func TestBatchCheckDCWV0050MigrationCommitIsTheSealBaseline(t *testing.T) {
	repo := newBatchRepo(t)
	cemMigrationCommits = nil
	bindA, sealA := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	bindB, sealB := repo.lane("lane-b", "b.txt", "b1\nB2\nb3\n", "")
	repo.batch(sealA, sealB)
	premigration := testGit(t, repo.root, "rev-parse", "HEAD")
	migration := repo.migrate(bindA, bindB)

	repo.base = premigration
	code, stdout, stderr := repo.check()
	if code != 0 || len(repo.args) != 0 || !strings.Contains(stdout, "dogfood-batch-check: NOTE sealed-cem-migrated maps=2 commit="+migration+"\n") ||
		!strings.Contains(stdout, "dogfood-batch-check: PASS constituents=0 commits=1\n") {
		t.Fatalf("migrated range exit=%d verifier=%v stdout=%s stderr=%s", code, repo.args, stdout, stderr)
	}
	if code, stderr := repo.admit(premigration); code != 0 || len(repo.args) != 0 ||
		stderr != "dogfood-change: NOTE sealed-cem-migrated maps=2 commit="+migration+"\n" {
		t.Fatalf("admit exit=%d stderr=%q", code, stderr)
	}

	// Without the migration commit as baseline the same range is modified.
	cemMigrationCommits = nil
	if code, stdout, stderr := repo.check(); code == 0 || !strings.Contains(stdout+stderr, "sealed-cem-modified") {
		t.Fatalf("no baseline exit=%d stdout=%s", code, stdout)
	}
	cemMigrationCommits = []string{migration}

	// A constituent the range adds before the migration commit rewrote it.
	repo.base = testGit(t, repo.root, "rev-list", "--max-parents=0", "HEAD")
	if code, stdout, stderr := repo.check(); code == 0 || !strings.Contains(stdout+stderr, "FAIL constituent-migrated count=2\n  bind=") ||
		!strings.Contains(stdout+stderr, "split the batch at the migration commit") {
		t.Fatalf("migrated constituent exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}

	// Any later edit of a migrated map is refused against the migration blobs.
	repo.write(".corvint/changes/"+bindA+".cem.json", `{"spec": "cem/1.0", "edited": true}`+"\n")
	repo.commit("edit a migrated map")
	repo.base = premigration
	if code, stdout, stderr := repo.check(); code == 0 || !strings.Contains(stdout+stderr, ".corvint/changes/"+bindA+".cem.json status=M") {
		t.Fatalf("edited exit=%d stdout=%s", code, stdout)
	}
	if code, stderr := repo.admit(premigration); code != 2 || !strings.Contains(stderr, bindA+".cem.json: sealed-cem-modified status=M") {
		t.Fatalf("edited admit exit=%d stderr=%q", code, stderr)
	}
}

// DCW-V0-050: a merge that keeps the pre-migration tree hides the migration's
// rewrites from the net BASE..HEAD diff; the rewritten paths must still hold
// the migration's bytes, so the range fails sealed-cem-migration-reverted.
func TestBatchCheckDCWV0050RevertedMigrationFails(t *testing.T) {
	repo := newBatchRepo(t)
	cemMigrationCommits = nil
	bindA, sealA := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	repo.batch(sealA)
	premigration := testGit(t, repo.root, "rev-parse", "HEAD")
	migration := repo.migrate(bindA)
	testGit(t, repo.root, "checkout", "-q", "-b", "restore", premigration)
	testGit(t, repo.root, "merge", "-q", "-s", "ours", "--no-edit", migration)

	repo.base = premigration
	if code, stdout, stderr := repo.check(); code == 0 || !strings.Contains(stdout+stderr, "sealed-cem-migration-reverted") ||
		!strings.Contains(stdout+stderr, bindA+".cem.json") {
		t.Fatalf("reverted exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if code, stderr := repo.admit(premigration); code != 2 || !strings.Contains(stderr, bindA+".cem.json: sealed-cem-migration-reverted") {
		t.Fatalf("reverted admit exit=%d stderr=%q", code, stderr)
	}
}

// DCW-V0-050: a follow-up migration commit, converting a map a pre-cutover
// branch landed after the first migration, is a second seal baseline.
func TestBatchCheckDCWV0050FollowUpMigration(t *testing.T) {
	repo := newBatchRepo(t)
	cemMigrationCommits = nil
	bindA, sealA := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	_, sealB := repo.lane("lane-b", "b.txt", "b1\nB2\nb3\n", "")
	repo.batch(sealA)
	first := repo.migrate(bindA)
	testGit(t, repo.root, "merge", "-q", "--no-ff", "--no-edit", sealB)
	landed := testGit(t, repo.root, "rev-parse", "HEAD")
	bindB := testGit(t, repo.root, "rev-parse", sealB+"^")
	second := repo.migrate(bindB)
	if len(cemMigrationCommits) != 2 || cemMigrationCommits[0] != first || cemMigrationCommits[1] != second {
		t.Fatalf("migration commits=%v", cemMigrationCommits)
	}

	repo.base = landed
	if code, stdout, stderr := repo.check(); code != 0 || !strings.Contains(stdout, "NOTE sealed-cem-migrated maps=1 commit="+second+"\n") ||
		!strings.Contains(stdout, "PASS constituents=0 commits=1\n") {
		t.Fatalf("follow-up exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	// The map the follow-up rewrote was added within the range: split the batch.
	repo.base = first
	if code, stdout, stderr := repo.check(); code == 0 || !strings.Contains(stdout+stderr, "FAIL constituent-migrated count=1\n  bind="+bindB) {
		t.Fatalf("follow-up constituent exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}
