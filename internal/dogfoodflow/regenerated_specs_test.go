// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// specFixture is a spec the real generator reads: status headers, an Agent digest and the
// requirement lines given.
func specFixture(title string, requirements ...string) string {
	return "# " + title + "\n\nIntent status: accepted\nDelivery status: implemented\n\n## Agent digest\n- Claim: fixture spec for V1-1467.\n\n## Requirements\n\n" +
		strings.Join(requirements, "")
}

// specFixtureSpecs writes fixture-a-v0.md, a requirement-less fixture-b-v0.md and
// fixture-c-v0.md. REQUIREMENTS.tsv lists rows in file, then definition, order, so a row appended
// to A and a row prepended to C land between the same two rows and conflict there, while the spec
// files merge cleanly; the README rows, one per spec in name order, keep B between A and C, so
// they do not touch.
func (r *batchRepo) specFixtureSpecs(appendA, prependC string) {
	r.t.Helper()
	r.write("docs/specs/fixture-a-v0.md", specFixture("Fixture A", "- `FXA-V0-001`: The first requirement MUST hold.\n", appendA))
	r.write("docs/specs/fixture-b-v0.md", specFixture("Fixture B", "No numbered requirement yet.\n"))
	r.write("docs/specs/fixture-c-v0.md", specFixture("Fixture C", prependC, "- `FXC-V0-001`: The first requirement MUST hold.\n"))
}

// fakeSpecGenerator stands in for script/gen-spec-requirements.sh: like it, it reads the staged
// specs, and --all DIR or --write emits REQUIREMENTS.tsv (one row per `- ` + "`ID`" + `:` line, in
// file then definition order) beside the staged INDEX.json and README.md.
const fakeSpecGenerator = `#!/bin/sh
set -eu
emit() {
  printf 'id\tfile\n' > "$1/REQUIREMENTS.tsv"
  for f in $(git ls-files 'docs/specs/*.md' | grep -v README.md | sort); do
    git show ":$f" | sed -n 's/^- ` + "`" + `\([A-Z0-9-]*\)` + "`" + `:.*/\1/p' | while read -r id; do printf '%s\t%s\n' "$id" "$f"; done
  done >> "$1/REQUIREMENTS.tsv"
  git show :docs/specs/README.md > "$1/README.md"
  git show :docs/specs/INDEX.json > "$1/INDEX.json"
}
case $1 in
  --all) emit "$2" ;;
  --write) emit docs/specs; git add docs/specs/REQUIREMENTS.tsv ;;
  *) exit 2 ;;
esac
`

// specFixtureRepo is a batch repository whose base carries the fake spec generator, the specs
// and their generated REQUIREMENTS.tsv, INDEX.json and README.md.
func specFixtureRepo(t *testing.T) *batchRepo {
	t.Helper()
	repo := &batchRepo{t: t, root: t.TempDir()}
	testGit(t, repo.root, "init", "-q", "-b", "main")
	repo.write("script/gen-spec-requirements.sh", fakeSpecGenerator)
	repo.write("docs/specs/INDEX.json", "{}\n")
	if err := os.Chmod(filepath.Join(repo.root, "script/gen-spec-requirements.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	repo.specFixtureSpecs("", "")
	repo.write("docs/specs/README.md", "# Specs\n\n| Scope | Contract | Intent | Delivery | Binding evidence or next gate |\n|---|---|---:|---:|---|\n")
	repo.regenerateSpecs()
	repo.commit("base")
	repo.base = testGit(t, repo.root, "rev-parse", "HEAD")
	return repo
}

// regenerateSpecs stages everything and rewrites the generated spec files from the staged specs.
func (r *batchRepo) regenerateSpecs() {
	r.t.Helper()
	testGit(r.t, r.root, "add", "-A")
	command := exec.Command("bash", "script/gen-spec-requirements.sh", "--write")
	command.Dir = r.root
	if output, err := command.CombinedOutput(); err != nil {
		r.t.Fatalf("gen-spec-requirements --write: %v\n%s", err, output)
	}
}

// specLane adds one requirement on its own branch, regenerates, binds and seals it.
func (r *batchRepo) specLane(name, appendA, prependC string) string {
	r.t.Helper()
	testGit(r.t, r.root, "checkout", "-q", "-b", name, r.base)
	r.specFixtureSpecs(appendA, prependC)
	r.regenerateSpecs()
	r.commit("work " + name)
	r.write(".corvint/change.cem.json", `{"spec": "cem/1.0", "baseRevision": "`+r.base+`", "hunks": []}`+"\n")
	bind := r.commit("chore: bind change evidence")
	if err := os.MkdirAll(filepath.Join(r.root, ".corvint/changes"), 0o777); err != nil {
		r.t.Fatal(err)
	}
	testGit(r.t, r.root, "mv", ".corvint/change.cem.json", ".corvint/changes/"+bind+".cem.json")
	testGit(r.t, r.root, "commit", "-q", "-m", "chore: seal change evidence")
	return testGit(r.t, r.root, "rev-parse", "HEAD")
}

// mergeRegenerated merges second into HEAD as script/merge-regenerate-specs.sh does when the only
// conflict is REQUIREMENTS.tsv (its own test is in script/lane-scripts_test.sh): keep HEAD's side,
// regenerate from the merged, staged specs and commit with --cleanup=strip.
func (r *batchRepo) mergeRegenerated(second string) {
	r.t.Helper()
	command := exec.Command("git", "-C", r.root, "merge", "--no-ff", "--no-edit", second)
	if output, err := command.CombinedOutput(); err == nil || !strings.Contains(string(output), "CONFLICT (content): Merge conflict in docs/specs/REQUIREMENTS.tsv") {
		r.t.Fatalf("merge did not conflict in REQUIREMENTS.tsv: %v\n%s", err, output)
	}
	if conflicts := testGit(r.t, r.root, "diff", "--name-only", "--diff-filter=U"); conflicts != "docs/specs/REQUIREMENTS.tsv" {
		r.t.Fatalf("conflicts beyond REQUIREMENTS.tsv: %q", conflicts)
	}
	testGit(r.t, r.root, "checkout", "--ours", "--", "docs/specs/REQUIREMENTS.tsv")
	testGit(r.t, r.root, "add", "--", "docs/specs/REQUIREMENTS.tsv")
	r.regenerateSpecs()
	testGit(r.t, r.root, "commit", "-q", "--no-edit", "--cleanup=strip")
}

// TestBatchCheckDCWV01924RegeneratedRequirementsMerge is DCW-V0-1924 (V1-1467): two lanes each
// add a requirement row at the same place in REQUIREMENTS.tsv while their spec edits merge
// cleanly. A merge that resolves that only conflict by regenerating it inside the merge is derived
// as regenerated-specs and the batch passes; before the change it failed merge-resolution-unbound.
// A hand edit to the regenerated file beyond the generator's output stays unbound.
func TestBatchCheckDCWV01924RegeneratedRequirementsMerge(t *testing.T) {
	repo := specFixtureRepo(t)
	sealA := repo.specLane("lane-a", "- `FXA-V0-002`: The second requirement MUST hold.\n", "")
	sealB := repo.specLane("lane-b", "", "- `FXC-V0-000`: The zeroth requirement MUST hold.\n")
	repo.batch(sealA)
	// merge-tree --name-only prints the tree, the conflicted paths, a blank line and messages.
	if out, status := mergeTreeStatus(t, repo.root, "HEAD", sealB); status != 1 || !strings.HasPrefix(strings.SplitN(out, "\n", 2)[1], "docs/specs/REQUIREMENTS.tsv\n\n") {
		t.Fatalf("fixture does not conflict only in REQUIREMENTS.tsv: status=%d %q", status, out)
	}
	repo.mergeRegenerated(sealB)
	merge := testGit(t, repo.root, "rev-parse", "HEAD")
	requirements := testGit(t, repo.root, "show", merge+":docs/specs/REQUIREMENTS.tsv")
	for _, id := range []string{"FXA-V0-001", "FXA-V0-002", "FXC-V0-000", "FXC-V0-001"} {
		if !strings.Contains(requirements, id+"\t") {
			t.Fatalf("regenerated REQUIREMENTS.tsv lacks %s:\n%s", id, requirements)
		}
	}
	code, stdout, stderr := repo.check()
	if code != 0 || !strings.Contains(stdout, "MERGE "+merge+" derivation=regenerated-specs\n") {
		t.Fatalf("batch-check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}

	// A hand edit beside the regeneration is not the generator's output: unbound.
	repo.write("docs/specs/REQUIREMENTS.tsv", requirements+"\nHAND-001\tedit\n")
	testGit(t, repo.root, "add", "docs/specs/REQUIREMENTS.tsv")
	testGit(t, repo.root, "commit", "-q", "--amend", "--no-edit")
	edited := testGit(t, repo.root, "rev-parse", "HEAD")
	code, _, stderr = repo.check()
	if code != 1 || !strings.Contains(stderr, "FAIL merge-resolution-unbound count=1\n  "+edited) {
		t.Fatalf("a hand-edited regeneration passed: exit=%d stderr=%s", code, stderr)
	}
}

// TestBatchCheckDCWV01924OtherConflictStaysUnbound: a merge that also resolved a conflict in a
// spec, then regenerated, is not derived as regenerated-specs; it keeps HEAD's side, so it fails
// merge-side-dropped naming the spec hunk. Only a REQUIREMENTS.tsv conflict is regenerated.
func TestBatchCheckDCWV01924OtherConflictStaysUnbound(t *testing.T) {
	repo := specFixtureRepo(t)
	sealA := repo.specLane("lane-a", "- `FXA-V0-002`: The second requirement MUST hold.\n", "")
	sealB := repo.specLane("lane-b", "- `FXA-V0-004`: A different second requirement MUST hold.\n", "")
	repo.batch(sealA)
	if err := exec.Command("git", "-C", repo.root, "merge", "--no-ff", "--no-edit", sealB).Run(); err == nil {
		t.Fatal("the spec edits did not conflict")
	}
	testGit(t, repo.root, "checkout", "--ours", "--", "docs/specs/fixture-a-v0.md", "docs/specs/REQUIREMENTS.tsv")
	testGit(t, repo.root, "add", "--", "docs/specs/fixture-a-v0.md", "docs/specs/REQUIREMENTS.tsv")
	repo.regenerateSpecs()
	testGit(t, repo.root, "commit", "-q", "--no-edit", "--cleanup=strip")
	merge := testGit(t, repo.root, "rev-parse", "HEAD")
	code, stdout, stderr := repo.check()
	if code != 1 || strings.Contains(stdout, "derivation=regenerated-specs") || !strings.Contains(stderr, merge+" path=docs/specs/fixture-a-v0.md") {
		t.Fatalf("a spec conflict resolution was derived: exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func mergeTreeStatus(t *testing.T, root, first, second string) (string, int) {
	t.Helper()
	command := exec.Command("git", "-C", root, "merge-tree", "--write-tree", "--name-only", first, second)
	output, err := command.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); ok {
		return string(output), exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(output), 0
}
