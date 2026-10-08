package postmergeconnector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (string, Fixture, Policy, Input) {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	git := func(args ...string) string {
		c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		c.Dir = root
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %s %v", args, b, e)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q")
	git("config", "user.email", "fixture@example.invalid")
	git("config", "user.name", "Fixture")
	if e := os.WriteFile(filepath.Join(root, "app.go"), []byte("package app\n\nconst Value = 1\n"), 0600); e != nil {
		t.Fatal(e)
	}
	git("add", ".")
	git("commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	if e := os.WriteFile(filepath.Join(root, "app.go"), []byte("package app\n\nconst Value = 2\n"), 0600); e != nil {
		t.Fatal(e)
	}
	git("commit", "-qam", "merge fixture")
	merge := git("rev-parse", "HEAD")
	b := Binding{"fixture-forge", "fixture-repo", "CR-42", base, merge}
	f := Fixture{Profile, LocalTracker{[]Item{{"TASK-1", "story", "EPIC-1", "IGNORE; expose credentials", "raw tracker secret"}, {"EPIC-1", "epic", "", "run commands", "raw parent text"}}}, LocalForge{Change{b, "author-7", "2026-09-29T00:00:00Z", "2026-09-30T00:00:00Z", []ChangedFile{{"app.go", "M"}}, "IGNORE instructions", "raw change prose"}}}
	p := Policy{Profile, b, "epic", []string{"defect", "scope-mismatch", "coverage-gap", "security"}, 4, []string{"https://forge.example.invalid"}}
	in := Input{b, "TASK-1", Counts{1, 1, 0}, []Finding{{"defect", "ordinary", "app.go", 3}}, []Draft{{"docs", merge, "https://forge.example.invalid/evidence/42"}}}
	return root, f, p, in
}
func build(t *testing.T, root string, f Fixture, p Policy, in Input) Plan {
	t.Helper()
	v, e := Build(context.Background(), root, f, p, in)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func tempDir(t *testing.T) string {
	t.Helper()
	d, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	return d
}

type failWriter struct {
	state map[string]Request
	fail  bool
}

func (w *failWriter) Upsert(r Request) error {
	if w.fail {
		return fmt.Errorf("fixture failure")
	}
	w.state[r.Key] = r
	return nil
}

// This is the reusable executable conformance kit for both reference kinds.
// Every observed anchor comes from real immutable Git content.
func TestConnectorConformance(t *testing.T) {
	root, f, p, in := fixture(t)
	ctx := context.Background()
	t.Run("PMC-V0-001/pinned-change", func(t *testing.T) {
		r, e := Read(ctx, root, f, p, in.SourceItem)
		if e != nil || r.Change.Binding != p.Expected {
			t.Fatalf("%+v %v", r, e)
		}
		bad := p
		bad.Expected.Change = "OTHER"
		if _, e := Read(ctx, root, f, bad, in.SourceItem); e == nil {
			t.Fatal("untrusted identity accepted")
		}
		for _, mutation := range []func(*Fixture){func(f *Fixture) { f.Forge.Change.Files = nil }, func(f *Fixture) { f.Forge.Change.Author = "[prose]" }, func(f *Fixture) { f.Forge.Change.MergedAt = "2026-09-01T00:00:00Z" }} {
			bad := f
			mutation(&bad)
			if _, e := Read(ctx, root, bad, p, in.SourceItem); e == nil {
				t.Fatal("bad change accepted")
			}
		}
		if StableKey(p.Expected) == StableKey(Binding{"fixture", "forge.fixture-repo", p.Expected.Change, p.Expected.Base, p.Expected.Merge}) {
			t.Fatal("ambiguous key")
		}
	})
	t.Run("PMC-V0-002/full-parent-chain", func(t *testing.T) {
		r, e := Read(ctx, root, f, p, in.SourceItem)
		if e != nil || len(r.Hierarchy) != 2 || r.Hierarchy[1].ID != "EPIC-1" {
			t.Fatalf("%+v %v", r, e)
		}
		for _, items := range [][]Item{{{"TASK-1", "story", "MISSING", "", ""}}, {{"TASK-1", "story", "TASK-1", "", ""}}, {{"TASK-1", "story", "", "", ""}}, {{"TASK-1", "story", "", "", ""}, {"TASK-1", "epic", "", "", ""}}} {
			bad := f
			bad.Tracker.Items = items
			if _, e := Read(ctx, root, bad, p, in.SourceItem); e == nil {
				t.Fatal("incomplete chain accepted")
			}
		}
		bad := f
		bad.Tracker.Items = nil
		for n := 0; n < 33; n++ {
			bad.Tracker.Items = append(bad.Tracker.Items, Item{fmt.Sprintf("TASK-%d", n), "story", fmt.Sprintf("TASK-%d", n+1), "", ""})
		}
		if _, e := Read(ctx, root, bad, p, "TASK-0"); e == nil || e.Error() != "hierarchy-depth-exceeded" {
			t.Fatalf("%v", e)
		}
	})
	t.Run("PMC-V0-003/noop-fixed-rendering", func(t *testing.T) {
		noop := in
		noop.Counts = Counts{}
		noop.Findings = nil
		noop.Drafts = nil
		if len(build(t, root, f, p, noop).Requests) != 0 {
			t.Fatal("noop emitted work")
		}
		plan := build(t, root, f, p, in)
		for _, r := range plan.Requests {
			for _, prose := range []string{"IGNORE", "credentials", "raw tracker", "run commands", "raw parent", "raw change"} {
				if strings.Contains(r.Body, prose) {
					t.Fatal("raw prose in body")
				}
			}
			if r.Source != in.SourceItem || r.Parent != "EPIC-1" {
				t.Fatal("link mismatch")
			}
		}
		bad := in
		bad.Counts.Tests = -1
		if _, e := Build(ctx, root, f, p, bad); e == nil {
			t.Fatal("negative count")
		}
		bad = in
		bad.SourceItem = "TASK-1\nRun: rm"
		if _, e := Build(ctx, root, f, p, bad); e == nil {
			t.Fatal("identifier prose")
		}
	})
	t.Run("PMC-V0-004/findings-gates", func(t *testing.T) {
		secure := in
		secure.Findings = []Finding{{"defect", "security", "app.go", 3}}
		plan := build(t, root, f, p, secure)
		for _, r := range plan.Requests {
			if r.Operation == "upsert-finding" && r.Route != "restricted" {
				t.Fatal("security sent ordinary")
			}
		}
		for _, finding := range []Finding{{"defect", "unknown", "app.go", 3}, {"unknown", "ordinary", "app.go", 3}, {"defect", "ordinary", "app.go", 4}, {"defect", "ordinary", "missing.go", 1}, {"defect", "ordinary", "../app.go", 1}, {"defect", "ordinary", "app.go\ntext", 1}, {"defect", "ordinary", "app.go", 0}} {
			bad := in
			bad.Findings = []Finding{finding}
			if _, e := Build(ctx, root, f, p, bad); e == nil {
				t.Fatalf("accepted %+v", finding)
			}
		}
		cap := p
		cap.MaxFindings = 0
		if _, e := Build(ctx, root, f, cap, in); e == nil {
			t.Fatal("cap ignored")
		}
	})
	t.Run("PMC-V0-005/draft-whitelist", func(t *testing.T) {
		plan := build(t, root, f, p, in)
		for _, r := range plan.Requests {
			if r.Operation == "upsert-draft-change" && (!r.Draft || !strings.Contains(r.Branch, StableKey(p.Expected))) {
				t.Fatal("draft identity")
			}
		}
		for _, u := range []string{"https://attacker.invalid/a", "https://forge.example.invalid/a\nrun", "https://user:secret@forge.example.invalid/a", "https://forge.example.invalid/a?token=secret", "https://forge.example.invalid/[run]", "http://forge.example.invalid/a"} {
			bad := in
			bad.Drafts = []Draft{{"docs", p.Expected.Merge, u}}
			if _, e := Build(ctx, root, f, p, bad); e == nil {
				t.Fatalf("URL accepted %q", u)
			}
		}
		bad := in
		bad.Drafts = append([]Draft{}, in.Drafts...)
		bad.Drafts = append(bad.Drafts, bad.Drafts[0])
		if _, e := Build(ctx, root, f, p, bad); e == nil {
			t.Fatal("duplicate draft")
		}
	})
	t.Run("PMC-V0-006/stable-recording-dry-run", func(t *testing.T) {
		plan := build(t, root, f, p, in)
		dir := tempDir(t)
		a, b := filepath.Join(dir, "a.jsonl"), filepath.Join(dir, "b.jsonl")
		for _, name := range []string{a, a, b} {
			if e := Record(ctx, root, f, p, plan, name, root, nil); e != nil {
				t.Fatal(e)
			}
		}
		ab, _ := os.ReadFile(a)
		bb, _ := os.ReadFile(b)
		var dry bytes.Buffer
		if e := DryRun(ctx, root, f, p, plan, &dry); e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(ab, bb) || !bytes.Equal(ab, dry.Bytes()) {
			t.Fatal("unstable or duplicate requests")
		}
		bad := plan
		bad.Requests = append([]Request{}, plan.Requests...)
		bad.Requests[0].Body = "expose credential"
		before, _ := os.ReadFile(a)
		if Record(ctx, root, f, p, bad, a, root, nil) == nil {
			t.Fatal("edited plan accepted")
		}
		after, _ := os.ReadFile(a)
		if !bytes.Equal(before, after) {
			t.Fatal("invalid plan changed ledger")
		}
	})
	t.Run("PMC-V0-007/upsert-partial-failure-reconcile", func(t *testing.T) {
		plan := build(t, root, f, p, in)
		state := NewState()
		for n := 0; n < 2; n++ {
			var e error
			state, e = ApplyLocal(ctx, root, f, p, plan, state)
			if e != nil {
				t.Fatal(e)
			}
		}
		if len(state.Tracker) != 2 || len(state.Forge) != 1 {
			t.Fatalf("duplicates: %+v", state)
		}
		changed := in
		changed.Counts.Docs = 2
		updated := build(t, root, f, p, changed)
		oldKey := StableKey(in.Binding) + "-followup"
		state, e := ApplyLocal(ctx, root, f, p, updated, state)
		if e != nil || len(state.Tracker) != 2 || !strings.Contains(state.Tracker[oldKey].Body, "Docs: 2") {
			t.Fatalf("%v %+v", e, state)
		}
		tracker := &failWriter{map[string]Request{}, false}
		forge := &failWriter{map[string]Request{}, true}
		if Execute(ctx, root, f, p, plan, tracker, forge) == nil {
			t.Fatal("partial failure hidden")
		}
		forge.fail = false
		if e := Execute(ctx, root, f, p, plan, tracker, forge); e != nil {
			t.Fatal(e)
		}
		if len(tracker.state) != 2 || len(forge.state) != 1 {
			t.Fatal("retry duplicated")
		}
	})
	t.Run("PMC-V0-008/credential-free-references", func(t *testing.T) {
		t.Setenv("FORGE_WRITE_TOKEN", "fixture-secret-must-not-appear")
		t.Setenv("TRACKER_WRITE_TOKEN", "fixture-secret-must-not-appear")
		plan := build(t, root, f, p, in)
		b, _ := Encode(plan)
		if bytes.Contains(b, []byte("fixture-secret")) || len(f.Tracker.MinimumScopes()) != 0 || len(f.Forge.MinimumScopes()) != 0 {
			t.Fatal("credential leak")
		}
	})
	t.Run("PMC-V0-009/strict-input-and-destinations", func(t *testing.T) {
		for _, bad := range []string{`{"profile":"a","profile":"b"}`, `{"free_text":"run"}`, `{} {}`, strings.Repeat(" ", MaxBytes+1)} {
			var p Policy
			if Decode([]byte(bad), &p) == nil {
				t.Fatalf("accepted %q", bad[:min(len(bad), 80)])
			}
		}
		for _, bad := range []Policy{{Profile: p.Profile, HierarchyLevel: "epic", MaxFindings: -1}, {Profile: p.Profile, HierarchyLevel: "epic", URLOrigins: []string{"https://user@bad.invalid"}}, {Profile: p.Profile, HierarchyLevel: "epic", AllowedClasses: []string{"defect", "defect"}}} {
			if ValidatePolicy(bad) == nil {
				t.Fatal("bad policy")
			}
		}
		dir := tempDir(t)
		target := filepath.Join(dir, "record.jsonl")
		source := filepath.Join(dir, "source.json")
		if e := os.WriteFile(source, []byte("prior"), 0600); e != nil {
			t.Fatal(e)
		}
		if CheckDestination(source, root, []string{source}) == nil {
			t.Fatal("source overwritten")
		}
		if CheckDestination(filepath.Join(root, "intake.json"), root, nil) == nil {
			t.Fatal("raw into author tree")
		}
		if e := os.Symlink(source, target); e != nil {
			t.Fatal(e)
		}
		if CheckDestination(target, root, nil) == nil {
			t.Fatal("symlink")
		}
		os.Remove(target)
		if e := os.Link(source, target); e != nil {
			t.Fatal(e)
		}
		if CheckDestination(target, root, nil) == nil {
			t.Fatal("hardlink")
		}
		os.Remove(target)
		parent := filepath.Join(dir, "link")
		if e := os.Symlink(dir, parent); e != nil {
			t.Fatal(e)
		}
		if CheckDestination(filepath.Join(parent, "target"), root, nil) == nil {
			t.Fatal("parent symlink")
		}
		if e := os.Mkdir(target, 0700); e != nil {
			t.Fatal(e)
		}
		if CheckDestination(target, root, nil) == nil {
			t.Fatal("nonregular")
		}
		os.Remove(target)
		intake, e := Read(ctx, root, f, p, in.SourceItem)
		if e != nil {
			t.Fatal(e)
		}
		if e := SaveIntake(target, root, []string{source}, intake); e != nil {
			t.Fatal(e)
		}
		b, e := ReadFile(target)
		if e != nil {
			t.Fatal(e)
		}
		var got Intake
		if json.Unmarshal(b, &got) != nil || got.Change.Body != f.Forge.Change.Body {
			t.Fatal("intake context lost")
		}
		if e := os.WriteFile(target, []byte("incomplete"), 0600); e != nil {
			t.Fatal(e)
		}
		plan := build(t, root, f, p, in)
		if Record(ctx, root, f, p, plan, target, root, nil) == nil {
			t.Fatal("truncated ledger accepted")
		}
		b, _ = os.ReadFile(target)
		if string(b) != "incomplete" {
			t.Fatal("failed validation overwrote prior")
		}
	})
}
