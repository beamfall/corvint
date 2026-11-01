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
		"refs/heads/lane-a:docs/specs/" + self, "refs/heads/lane-b:docs/specs/" + self,
		"REQUIREMENT_ID_COLLISION ASD-V0-036 defined in more than one file of one ref",
		"REQUIREMENT_ID_COLLISION ASD-V0-036 defined in a different file than on the base",
		"refs/heads/lane-c:docs/specs/affected-plan-v0.md",
		"scanned 4 lane head(s) against main",
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
