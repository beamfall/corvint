// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func tgit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func commitSpec(t *testing.T, dir, file, body string) {
	t.Helper()
	p := filepath.Join(dir, "docs", "specs", file)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	tgit(t, dir, "add", "-A")
	tgit(t, dir, "commit", "-q", "-m", "spec "+file)
}

func snapshot(t *testing.T, dir string) string {
	t.Helper()
	return tgit(t, dir, "for-each-ref") + tgit(t, dir, "status", "--porcelain", "--ignored") + tgit(t, dir, "worktree", "list", "--porcelain")
}

// TestTLFV0886_ReproducesASDV0Collisions rebuilds the ASD-V0-032/036 case:
// two lanes cut from one base each took ASD-V0-032 for a different
// requirement, and a third defined ASD-V0-036 in another spec file than the
// base does. Both IDs are reported, exit 1, and nothing is written; a lane
// that only stacks on another's bullet, a merged branch, and a clean set
// exit 0; an unreadable base exits 2 (TLF-V0-886).
func TestTLFV0886_ReproducesASDV0Collisions(t *testing.T) {
	dir := t.TempDir()
	tgit(t, dir, "init", "-q", "-b", "main")
	const self = "corvint-self-development-v0.md"
	baseBody := "# Self\n\n- `ASD-V0-031`: reused per-slot remote worktree.\n- `ASD-V0-036`: sync-check names doc-gates commands.\n"
	commitSpec(t, dir, self, baseBody)
	tgit(t, dir, "branch", "merged")

	tgit(t, dir, "checkout", "-q", "-b", "lane-a", "main")
	commitSpec(t, dir, self, baseBody+"- `ASD-V0-032`: base-present batch-precheck failures.\n")
	tgit(t, dir, "checkout", "-q", "-b", "lane-a-stack", "lane-a")
	commitSpec(t, dir, "other.md", "- `OTH-V0-001`: unrelated.\n")
	tgit(t, dir, "checkout", "-q", "main")

	var out, errb bytes.Buffer
	if code := run([]string{"--repo", dir, "--base", "main"}, &out, &errb); code != 0 {
		t.Fatalf("one lane plus its stack: exit %d\n%s%s", code, out.String(), errb.String())
	}

	tgit(t, dir, "checkout", "-q", "-b", "lane-b", "main")
	commitSpec(t, dir, self, baseBody+"- `ASD-V0-032`: one runner with per-run telemetry.\n")
	tgit(t, dir, "checkout", "-q", "-b", "lane-c", "main")
	commitSpec(t, dir, "affected-plan-v0.md", "- `ASD-V0-036`: batch-precheck twin mirrors ci.yml.\n")
	tgit(t, dir, "checkout", "-q", "--orphan", "old-lineage")
	commitSpec(t, dir, self, "- `ASD-V0-032`: an unrelated private lineage.\n")
	tgit(t, dir, "checkout", "-q", "--detach", "main")

	before := snapshot(t, dir)
	out.Reset()
	errb.Reset()
	code := run([]string{"--repo", dir, "--base", "main"}, &out, &errb)
	if after := snapshot(t, dir); after != before {
		t.Fatalf("the check wrote state:\n%s\n---\n%s", before, after)
	}
	text := out.String()
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s%s", code, text, errb.String())
	}
	for _, want := range []string{
		"REQUIREMENT_ID_COLLISION ASD-V0-032 new on two refs with different definitions",
		// lane-a-stack contains lane-a, so it is scanned in lane-a's place (TLF-V0-960).
		"refs/heads/lane-a-stack:docs/specs/" + self, "refs/heads/lane-b:docs/specs/" + self,
		"REQUIREMENT_ID_COLLISION ASD-V0-036 defined in more than one file of one ref",
		"REQUIREMENT_ID_COLLISION ASD-V0-036 defined in a different file than on the base",
		"refs/heads/lane-c:docs/specs/affected-plan-v0.md",
		"scanned 3 lane head(s) against main",
		"skipped 1 lane head(s) another scanned head contains: refs/heads/lane-a@",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "refs/heads/merged") || strings.Contains(text, "old-lineage") || strings.Contains(text, "OTH-V0-001") {
		t.Errorf("merged branch or a unique ID reported:\n%s", text)
	}

	out.Reset()
	if code := run([]string{"--repo", dir, "--base", "no-such-ref"}, &out, &errb); code != 2 {
		t.Fatalf("missing base: exit %d", code)
	}
	if code := run([]string{"--repo", dir, "extra"}, &out, &errb); code != 2 {
		t.Fatalf("positional argument: exit %d", code)
	}
	errb.Reset()
	if code := run([]string{"--help"}, &out, &errb); code != 0 || !strings.Contains(errb.String(), "Example:") {
		t.Fatalf("help: exit %d %s", code, errb.String())
	}
}

// collidingRepo builds a base with ASD-V0-031 and two lanes that each take
// ASD-V0-032 for a different requirement.
func collidingRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	tgit(t, dir, "init", "-q", "-b", "main")
	base := "- `ASD-V0-031`: base.\n"
	commitSpec(t, dir, "s.md", base)
	tgit(t, dir, "checkout", "-q", "-b", "lane-a", "main")
	commitSpec(t, dir, "s.md", base+"- `ASD-V0-032`: lane a.\n")
	tgit(t, dir, "checkout", "-q", "-b", "lane-b", "main")
	commitSpec(t, dir, "s.md", base+"- `ASD-V0-032`: lane b.\n")
	tgit(t, dir, "checkout", "-q", "main")
	return dir
}

// TestTLFV0886_LineNumberConfigStillParses: repository-local
// grep.lineNumber and grep.column do not hide a collision (TLF-V0-886).
func TestTLFV0886_LineNumberConfigStillParses(t *testing.T) {
	dir := collidingRepo(t)
	tgit(t, dir, "config", "grep.lineNumber", "true")
	tgit(t, dir, "config", "grep.column", "true")
	var out, errb bytes.Buffer
	if code := run([]string{"--repo", dir, "--base", "main"}, &out, &errb); code != 1 || !strings.Contains(out.String(), "REQUIREMENT_ID_COLLISION ASD-V0-032") {
		t.Fatalf("exit %d\n%s%s", code, out.String(), errb.String())
	}
}

// TestTLFV0886_PartialCloneNeverFetches: in a blob-less partial clone whose
// promisor remote can serve the missing spec blobs, the check fails as a
// read failure (exit 2) and fetches nothing into .git (TLF-V0-886).
func TestTLFV0886_PartialCloneNeverFetches(t *testing.T) {
	origin := collidingRepo(t)
	tgit(t, origin, "config", "uploadpack.allowFilter", "true")
	clone := filepath.Join(t.TempDir(), "clone")
	tgit(t, filepath.Dir(clone), "clone", "-q", "--no-checkout", "--filter=blob:none", "file://"+origin, clone)
	tgit(t, clone, "branch", "lane-a", "origin/lane-a")
	tgit(t, clone, "branch", "lane-b", "origin/lane-b")
	objects := func() string {
		var b strings.Builder
		filepath.WalkDir(filepath.Join(clone, ".git", "objects"), func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				b.WriteString(p + "\n")
			}
			return nil
		})
		return b.String()
	}
	before := objects()
	var out, errb bytes.Buffer
	code := run([]string{"--repo", clone, "--base", "main"}, &out, &errb)
	if after := objects(); after != before {
		t.Fatalf("the check fetched objects (exit %d):\n%s\n---\n%s", code, before, after)
	}
	if code != 2 {
		t.Fatalf("exit %d, want 2 on missing promisor blobs\n%s%s", code, out.String(), errb.String())
	}
}

// TestTLFV0960_LaneHeadMergedIntoBatchIsSkipped (V1-3650): a lane head that
// a batch branch already contains is not scanned beside it, so the batch's
// later wording of the lane's bullet is no collision; a lane the batch does
// not contain is still compared with it.
func TestTLFV0960_LaneHeadMergedIntoBatchIsSkipped(t *testing.T) {
	dir := t.TempDir()
	tgit(t, dir, "init", "-q", "-b", "main")
	base := "- `ASD-V0-031`: base.\n"
	commitSpec(t, dir, "s.md", base)
	tgit(t, dir, "checkout", "-q", "-b", "lane-x", "main")
	commitSpec(t, dir, "s.md", base+"- `ASD-V0-500`: lane x as first written.\n")
	tgit(t, dir, "checkout", "-q", "-b", "batch", "main")
	tgit(t, dir, "merge", "-q", "--no-ff", "-m", "merge lane-x", "lane-x")
	commitSpec(t, dir, "s.md", base+"- `ASD-V0-500`: lane x as the batch reworded it.\n")
	tgit(t, dir, "checkout", "-q", "main")

	var out, errb bytes.Buffer
	if code := run([]string{"--repo", dir, "--base", "main"}, &out, &errb); code != 0 {
		t.Fatalf("lane head inside the batch: exit %d\n%s%s", code, out.String(), errb.String())
	}
	if text := out.String(); !strings.Contains(text, "scanned 1 lane head(s) against main: refs/heads/batch@") || !strings.Contains(text, "skipped 1 lane head(s) another scanned head contains: refs/heads/lane-x@") {
		t.Fatalf("output:\n%s", text)
	}

	tgit(t, dir, "checkout", "-q", "-b", "lane-y", "main")
	commitSpec(t, dir, "s.md", base+"- `ASD-V0-500`: lane y took the same ID.\n")
	tgit(t, dir, "checkout", "-q", "main")
	out.Reset()
	if code := run([]string{"--repo", dir, "--base", "main"}, &out, &errb); code != 1 || !strings.Contains(out.String(), "REQUIREMENT_ID_COLLISION ASD-V0-500 new on two refs with different definitions refs/heads/batch:docs/specs/s.md refs/heads/lane-y:docs/specs/s.md") {
		t.Fatalf("a lane outside the batch: exit %d\n%s%s", code, out.String(), errb.String())
	}
}
