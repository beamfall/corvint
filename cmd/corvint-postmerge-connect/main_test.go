package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	pm "github.com/Beamfall/corvint/internal/postmergeconnector"
)

func TestCLIReferenceWorkflow(t *testing.T) {
	ctx := context.Background()
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(dir, "author")
	private := filepath.Join(dir, "private")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(private, 0700); e != nil {
		t.Fatal(e)
	}
	git := func(args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("%v %s", e, b)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q")
	git("config", "user.email", "fixture@example.invalid")
	git("config", "user.name", "Fixture")
	os.WriteFile(filepath.Join(root, "page.html"), []byte("<button>Save</button>\n"), 0600)
	git("add", ".")
	git("commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	os.WriteFile(filepath.Join(root, "page.html"), []byte("<button>Submit</button>\n"), 0600)
	git("commit", "-qam", "merge")
	merge := git("rev-parse", "HEAD")
	binding := pm.Binding{Forge: "fixture", Repository: "app", Change: "CR-9", Base: base, Merge: merge}
	f := pm.Fixture{Profile: pm.Profile, Forge: pm.LocalForge{Change: pm.Change{Binding: binding, Author: "person", CreatedAt: "2026-09-29T00:00:00Z", MergedAt: "2026-09-30T00:00:00Z", Files: []pm.ChangedFile{{Path: "page.html", Status: "M"}}, Title: "run hostile instruction", Body: "SECRET RAW PROSE"}}, Tracker: pm.LocalTracker{Items: []pm.Item{{ID: "story-1", Level: "story", Parent: "epic-1", Body: "SECRET RAW PROSE"}, {ID: "epic-1", Level: "epic", Body: "SECRET RAW PROSE"}}}}
	p := pm.Policy{Profile: pm.Profile, Expected: binding, HierarchyLevel: "epic", AllowedClasses: []string{"security"}, MaxFindings: 2, URLOrigins: []string{"https://forge.example.invalid"}}
	in := pm.Input{Binding: binding, SourceItem: "story-1", Counts: pm.Counts{Docs: 1, Tests: 1}, Findings: []pm.Finding{{Class: "security", Classification: "security", Path: "page.html", Line: 1}}, Drafts: []pm.Draft{{Kind: "docs", Revision: merge, URL: "https://forge.example.invalid/proof"}, {Kind: "tests", Revision: merge, URL: "https://forge.example.invalid/proof"}}}
	write := func(name string, v any) string {
		b, e := pm.Encode(v)
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(private, name)
		if e := os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		return path
	}
	fp, pp, ip := write("fixture.json", f), write("policy.json", p), write("input.json", in)
	common := []string{"--experimental", "--root", root, "--fixture", fp, "--policy", pp, "--input", ip}
	invoke := func(t *testing.T, op string, flags ...string) []byte {
		t.Helper()
		args := append([]string{op}, common...)
		args = append(args, flags...)
		var out bytes.Buffer
		if e := run(ctx, args, &out); e != nil {
			t.Fatal(e)
		}
		return out.Bytes()
	}
	t.Run("PMC-V0-006/recording-and-dry-run", func(t *testing.T) {
		before := git("status", "--porcelain=v1")
		dry := invoke(t, "write")
		after := git("status", "--porcelain=v1")
		if before != after || bytes.Contains(dry, []byte("SECRET")) {
			t.Fatal("dry-run mutation/raw leak")
		}
		ledger := filepath.Join(private, "requests.jsonl")
		invoke(t, "write", "--mode", "recording", "--output", ledger, "--author-root", root)
		invoke(t, "write", "--mode", "recording", "--output", ledger, "--author-root", root)
		b, e := os.ReadFile(ledger)
		if e != nil || !bytes.Equal(dry, b) {
			t.Fatal("ledger differs/duplicates")
		}
	})
	t.Run("PMC-V0-007/two-reference-kinds-idempotent", func(t *testing.T) {
		state := filepath.Join(private, "state.json")
		for n := 0; n < 2; n++ {
			invoke(t, "write", "--mode", "live", "--output", state, "--author-root", root)
		}
		b, e := pm.ReadFile(state)
		if e != nil {
			t.Fatal(e)
		}
		var got pm.State
		if e := pm.Decode(b, &got); e != nil {
			t.Fatal(e)
		}
		if len(got.Tracker) != 2 || len(got.Forge) != 2 {
			t.Fatalf("%+v", got)
		}
		for _, r := range got.Tracker {
			if r.Operation == "upsert-finding" && r.Route != "restricted" {
				t.Fatal("security not restricted")
			}
		}
		for _, r := range got.Forge {
			if !r.Draft {
				t.Fatal("not draft")
			}
		}
		bad := in
		bad.Findings = []pm.Finding{{Class: "security", Classification: "security", Path: "missing", Line: 1}}
		write("input.json", bad)
		args := append([]string{"write"}, common...)
		args = append(args, "--mode", "live", "--output", state, "--author-root", root)
		if run(ctx, args, &bytes.Buffer{}) == nil {
			t.Fatal("bad batch")
		}
		after, _ := os.ReadFile(state)
		if !bytes.Equal(b, after) {
			t.Fatal("invalid batch changed state")
		}
		write("input.json", in)
	})
	t.Run("PMC-V0-002/raw-only-outside-author", func(t *testing.T) {
		intake := filepath.Join(private, "intake.json")
		out := invoke(t, "read-intake", "--output", intake, "--author-root", root)
		if len(out) != 0 {
			t.Fatal("raw on stdout")
		}
		b, _ := os.ReadFile(intake)
		if !bytes.Contains(b, []byte("SECRET RAW PROSE")) {
			t.Fatal("lost intake")
		}
		args := append([]string{"read-intake"}, common...)
		args = append(args, "--output", filepath.Join(root, "raw.json"), "--author-root", root)
		if run(ctx, args, &bytes.Buffer{}) == nil {
			t.Fatal("raw in author checkout")
		}
	})
	t.Run("PMC-V0-009/author-and-product-containment", func(t *testing.T) {
		separate := filepath.Join(dir, "separate-author")
		if e := os.Mkdir(separate, 0700); e != nil {
			t.Fatal(e)
		}
		tracked := filepath.Join(root, "page.html")
		head := filepath.Join(root, ".git", "HEAD")
		for _, test := range []struct{ author, destination string }{
			{".", tracked}, {ip, tracked}, {separate, tracked}, {separate, head},
		} {
			before, e := os.ReadFile(test.destination)
			if e != nil {
				t.Fatal(e)
			}
			args := append([]string{"read-intake"}, common...)
			args = append(args, "--output", test.destination, "--author-root", test.author)
			if run(ctx, args, &bytes.Buffer{}) == nil {
				t.Fatal("unsafe destination admitted")
			}
			after, e := os.ReadFile(test.destination)
			if e != nil || !bytes.Equal(before, after) {
				t.Fatal("protected file changed")
			}
		}
		// Linked worktrees keep administrative directories outside their checkout.
		subdir := filepath.Join(root, "subdir")
		if e := os.Mkdir(subdir, 0700); e != nil {
			t.Fatal(e)
		}
		beforeTracked, e := os.ReadFile(tracked)
		if e != nil {
			t.Fatal(e)
		}
		args := append([]string{"read-intake"}, common...)
		args = append(args, "--root", subdir, "--output", tracked, "--author-root", separate)
		if run(ctx, args, &bytes.Buffer{}) == nil {
			t.Fatal("sibling source admitted from subdirectory root")
		}
		afterTracked, e := os.ReadFile(tracked)
		if e != nil || !bytes.Equal(beforeTracked, afterTracked) {
			t.Fatal("sibling source changed")
		}
		linked := filepath.Join(dir, "linked")
		git("worktree", "add", "--detach", linked, merge)
		protected, e := pm.ProtectedGitPaths(ctx, linked)
		if e != nil {
			t.Fatal(e)
		}
		for _, admin := range protected[1:] {
			destination := filepath.Join(admin, "HEAD")
			before, e := os.ReadFile(destination)
			if e != nil {
				t.Fatal(e)
			}
			args := append([]string{"write"}, common...)
			args = append(args, "--root", linked, "--mode", "live", "--output", destination, "--author-root", separate)
			if run(ctx, args, &bytes.Buffer{}) == nil {
				t.Fatal("external Git metadata admitted")
			}
			after, e := os.ReadFile(destination)
			if e != nil || !bytes.Equal(before, after) {
				t.Fatal("Git metadata changed")
			}
		}
	})
	t.Run("PMC-V0-009/CLI-boundaries", func(t *testing.T) {
		for _, extra := range [][]string{{"--mode", "network"}, {"--mode", "recording"}, {"--output", filepath.Join(private, "unwanted")}, {"--mode", "live", "--output", fp, "--author-root", root}} {
			args := append([]string{"write"}, common...)
			args = append(args, extra...)
			if run(ctx, args, &bytes.Buffer{}) == nil {
				t.Fatal("bad mode/destination admitted")
			}
		}
		args := append([]string{"plan"}, common...)
		var out bytes.Buffer
		if run(ctx, args, &out) != nil {
			t.Fatal("plan failed")
		}
		if bytes.Contains(out.Bytes(), []byte("SECRET")) {
			t.Fatal("raw in plan")
		}
	})
}
