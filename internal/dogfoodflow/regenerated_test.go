// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeBatchRegen stands in for script/batch-regen.sh: --mechanical repins
// receipt.txt to HEAD as repin-use-case-receipts.sh writes repositoryRevision,
// and the full run also derives count.txt from a.txt and stages it.
const fakeBatchRegen = `#!/bin/bash
root=${BATCH_REGEN_ROOT:-$PWD}
cd "$root" || exit 2
[ -z "${BATCH_REGEN_STEPS:-}" ] || exit 3
git rev-parse HEAD > receipt.txt
[ "$1" = --mechanical ] && exit 0
wc -l < a.txt | tr -d ' ' > count.txt
git add -A
`

// regenRepo is a batch repository whose base carries the fake batch-regen.sh
// and the DCW-V0-401 bullet a regenerated hunk is cited against.
func regenRepo(t *testing.T) *batchRepo {
	t.Helper()
	repo := newBatchRepo(t)
	repo.write("script/batch-regen.sh", fakeBatchRegen)
	if err := os.Chmod(filepath.Join(repo.root, "script/batch-regen.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	repo.write(sealAuthority, "# spec\n\n- `DCW-V0-400`: other\n- `DCW-V0-401`: a regenerated commit\n  is derived\n- `DCW-V0-398`: other\n")
	repo.base = repo.commit("regen script")
	return repo
}

// regenerate runs the fake regeneration in the repository itself, as a lane
// or the batch would, and commits the result under subject.
func (r *batchRepo) regenerate(mode, subject string) string {
	r.t.Helper()
	head := testGit(r.t, r.root, "rev-parse", "HEAD")
	r.write("receipt.txt", head+"\n")
	if mode == "full" {
		data, err := os.ReadFile(filepath.Join(r.root, "a.txt"))
		if err != nil {
			r.t.Fatal(err)
		}
		r.write("count.txt", strconv.Itoa(bytes.Count(data, []byte("\n")))+"\n")
	}
	return r.commit(subject)
}

// DCW-V0-401 (V1-2981): the batch-regen commit and a post-seal receipt repin
// equal to regeneration from their parents pass unbound-commits with a NOTE;
// a repin commit with a hand edit beside it stays unbound and says why.
func TestBatchCheckDCWV0401AcceptsRegeneratedCommits(t *testing.T) {
	repo := regenRepo(t)
	// The checker's own overrides never reach the replayed script: a step
	// override would make the fake script fail, and a startup file would write
	// into the checked repository.
	leaked := filepath.Join(repo.root, "leaked")
	startup := filepath.Join(t.TempDir(), "startup.sh")
	if err := os.WriteFile(startup, []byte("touch '"+leaked+"'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BATCH_REGEN_STEPS", "forged\tsteps")
	t.Setenv("BASH_ENV", startup)
	repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	repinA := repo.regenerate("mechanical", "repin use-case receipts after the seal")
	repo.batch(repinA)
	regen := repo.regenerate("full", batchRegenSubject)
	code, stdout, stderr := repo.check()
	for _, want := range []string{
		"NOTE COMMIT " + repinA + " derivation=regenerated mode=--mechanical (DCW-V0-401)\n",
		"NOTE COMMIT " + regen + " derivation=regenerated mode=--no-commit (DCW-V0-401)\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("exit=%d missing %q in\n%s%s", code, want, stdout, stderr)
		}
	}
	if code != 0 || strings.Contains(stderr, "unbound-commits") {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if _, err := os.Stat(leaked); !os.IsNotExist(err) {
		t.Fatalf("the replay ran the checker's BASH_ENV in the checked repository: %v", err)
	}

	// A repin with one hand-edited hunk is not a regeneration.
	repo.write("b.txt", "b1\nhand\nb3\n")
	head := testGit(t, repo.root, "rev-parse", "HEAD")
	repo.write("receipt.txt", head+"\n")
	hand := repo.commit("repin plus a hand edit")
	code, stdout, stderr = repo.check()
	if code != 1 || !strings.Contains(stderr, "FAIL unbound-commits count=1\n  "+hand+"\n") ||
		!strings.Contains(stdout, "NOTE COMMIT "+hand+" regeneration NOT_REPRODUCED tree-differs (DCW-V0-401)\n") {
		t.Fatalf("hand edit exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

// DCW-V0-398 (V1-3214): the citation preflight cites a hunk on a path whose
// whole change is one regenerated commit against the DCW-V0-401 bullet at
// BASE, with no hand row; a repin commit with any non-generated hunk still
// owes a row for every hunk it touches.
func TestChangeDCWV0398CitesRegeneratedHunksOnly(t *testing.T) {
	for _, hand := range []bool{false, true} {
		repo := regenRepo(t)
		repo.write("a.txt", "a1\nA2\na3\n")
		repo.commit("work")
		if hand {
			repo.write("b.txt", "b1\nhand\nb3\n")
		}
		repo.regenerate("mechanical", "repin receipts")
		repo.write(".corvint/change.cem.json", `{"hunks": [`+
			`{"id": "hunk:a", "path": "a.txt", "disposition": "unknown"}, `+
			`{"id": "hunk:r", "path": "receipt.txt", "disposition": "unknown"}]}`+"\n")
		var stderr bytes.Buffer
		c := &change{flow: flow{ctx: context.Background(), prefix: "dogfood-change", root: repo.root, stderr: &stderr, base: repo.base}}
		var reason string
		if code, err := boundary(context.Background(), func() int {
			reason = c.citationPlanMismatch([]byte("hunk:a\tdocs/x.md\t1:1\tspecification\n"))
			return 0
		}, func() {}); err != nil || code != 0 {
			t.Fatalf("hand=%v code=%d err=%v\n%s", hand, code, err, stderr.String())
		}
		cited := strings.Join(c.sealCiteRows, "\n")
		if !hand && (reason != "" || cited != "hunk:r\t"+sealAuthority+"\t4:5\tspecification") {
			t.Fatalf("regenerated: reason=%q detail=%q rows=%q", reason, c.mismatchDetail, cited)
		}
		if hand && (reason != "citation-plan-map-mismatch" || cited != "" || !strings.Contains(c.mismatchDetail, `path="receipt.txt"`)) {
			t.Fatalf("hand edit: reason=%q detail=%q rows=%q", reason, c.mismatchDetail, cited)
		}
	}
}

// DCW-V0-398: a merge that takes a path from a branch that regenerated it from
// an older parent is not a regenerated path: the listing keeps the merge, so
// the path's change is not one regenerated commit.
func TestChangeDCWV0398RefusesAMergedRegeneration(t *testing.T) {
	repo := regenRepo(t)
	fork := repo.base
	repo.write("receipt.txt", "hand\n")
	repo.base = repo.commit("base edits the receipt")
	testGit(t, repo.root, "checkout", "-q", "-b", "side", fork)
	repo.regenerate("mechanical", "repin receipts")
	testGit(t, repo.root, "checkout", "-q", "main")
	testGit(t, repo.root, "merge", "-q", "--no-edit", "-X", "theirs", "side")
	var stderr bytes.Buffer
	c := &change{flow: flow{ctx: context.Background(), prefix: "dogfood-change", root: repo.root, stderr: &stderr, base: repo.base}}
	var regeneratedPath bool
	if code, err := boundary(context.Background(), func() int {
		regeneratedPath = c.regeneratedPath("receipt.txt")
		return 0
	}, func() {}); err != nil || code != 0 || regeneratedPath {
		t.Fatalf("regeneratedPath=%v code=%d err=%v\n%s", regeneratedPath, code, err, stderr.String())
	}
}
