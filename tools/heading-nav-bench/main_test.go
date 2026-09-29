// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

func src(p, body string) source {
	lines := strings.SplitAfter(body, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return source{doccorpus.Input{Path: p, Blob: strings.Repeat("a", 40)}, lines, parse(lines)}
}

// HNE-V0-002: headings retain ancestry, preamble, parent intro and final residual.
func TestSections(t *testing.T) {
	s := src("a.md", "preamble\n# Top\nintro\n## Child\nchild\n### Leaf\nlast\n")
	want := []section{{1, 1, 0, -1, ""}, {2, 3, 1, 0, "Top"}, {4, 5, 2, 1, "Child"}, {6, 7, 3, 2, "Leaf"}}
	if !reflect.DeepEqual(s.sections, want) {
		t.Fatalf("got %#v", s.sections)
	}
	p, _, e := makePacket("rev", "Leaf", "heading", 6000, []string{"a.md"}, map[string]source{"a.md": s})
	if e != nil {
		t.Fatal(e)
	}
	if p.OmittedLines != 0 {
		t.Fatal(p)
	}
}

// HNE-V0-002: fenced pseudo-headings never become navigable sections.
func TestFencesAndSetext(t *testing.T) {
	s := src("a.md", "Title\n=====\n~~~go\n# Fake\n---\n~~~\n```md\n## Fake 2\n```\n## Real\nbody\n")
	if len(s.sections) != 3 || s.sections[1].title != "Title" || s.sections[2].title != "Real" {
		t.Fatalf("%#v", s.sections)
	}
	s = src("a.md", "````\n```\n# Hidden\n````\n# Visible\n")
	if len(s.sections) != 2 || s.sections[1].title != "Visible" {
		t.Fatalf("%#v", s.sections)
	}
}

// HNE-V0-002: duplicate labels disambiguate by ancestry and immutable line.
func TestDeterministicSectionSelection(t *testing.T) {
	s := src("a.md", "# Wrong\n## Shared\nbananas\n# Right\n## Shared\nquasar nebula\n")
	got := ordered(s, "quasar nebula")
	if !reflect.DeepEqual(got, []int{0, 3, 4, 1, 2}) {
		t.Fatalf("order %v", got)
	}
	for i := 0; i < 20; i++ {
		if !reflect.DeepEqual(got, ordered(s, "quasar nebula")) {
			t.Fatal("unstable")
		}
	}
}

// HNE-V0-003: exact serialized-byte envelope includes escaped text and metadata.
func TestBudget(t *testing.T) {
	all := map[string]source{"a.md": src("a.md", "# heading\n\"quoted\" <tag> \\ tab\t\n"+strings.Repeat("very long ", 300)+"\nend\n")}
	for _, arm := range []string{"baseline", "heading"} {
		for _, budget := range []int{1, 700, 900, 1100, 6000} {
			p, b, e := makePacket("rev", "quoted", arm, budget, []string{"a.md"}, all)
			if e != nil {
				if budget > 900 {
					t.Fatal(e)
				}
				continue
			}
			if len(b) > budget || len(b) != p.Bytes {
				t.Fatalf("%d %d %d", budget, len(b), p.Bytes)
			}
			var parsed packet
			if e = json.Unmarshal(b, &parsed); e != nil {
				t.Fatal(e)
			}
			for _, seg := range parsed.Segments {
				if seg.Text != strings.Join(all[seg.Path].lines[seg.Start-1:seg.End], "") {
					t.Fatal("partial line")
				}
			}
		}
	}
}

// HNE-V0-004: union coverage must include every line, labels do not count.
func TestFullSpanRecall(t *testing.T) {
	g := []span{{"a.md", 1, 4}}
	p := packet{Segments: []segment{{span: span{"a.md", 1, 2}}, {span: span{"a.md", 4, 4}}}, Outlines: []outline{{Path: "a.md", Line: 3}}}
	n, miss := recall(g, p)
	if n != 0 || len(miss) != 1 {
		t.Fatal(n, miss)
	}
	p.Segments = append(p.Segments, segment{span: span{"a.md", 3, 3}})
	n, miss = recall(g, p)
	if n != 1 || len(miss) != 0 {
		t.Fatal(n, miss)
	}
	n, miss = recall(nil, p)
	if n != 0 || len(miss) != 0 {
		t.Fatal("empty denominator cannot be success")
	}
}

// HNE-V0-001, HNE-V0-005: retrieval accepts no gold inputs or answer notes.
func TestGoldIsolation(t *testing.T) {
	all := map[string]source{"a.md": src("a.md", "# Topic\nsource\n")}
	a := trial{ID: "case", Query: "Topic", Gold: []span{{"a.md", 1, 1}}, AnswerNotes: "SECRET GOLD"}
	b := a
	b.Gold = []span{{"a.md", 2, 2}}
	b.AnswerNotes = "OTHER SECRET"
	_, one, e := makePacket("rev", a.Query, "heading", 6000, []string{"a.md"}, all)
	if e != nil {
		t.Fatal(e)
	}
	_, two, e := makePacket("rev", b.Query, "heading", 6000, []string{"a.md"}, all)
	if e != nil {
		t.Fatal(e)
	}
	if string(one) != string(two) || strings.Contains(string(one), "SECRET") {
		t.Fatal("gold leaked")
	}
}

// HNE-V0-001: immutable source survives worktree drift; altered manifests fail.
func TestImmutableSource(t *testing.T) {
	dir := t.TempDir()
	cmd := func(args ...string) string {
		t.Helper()
		b, e := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %s %v", args, b, e)
		}
		return strings.TrimSpace(string(b))
	}
	cmd("init", "-q")
	body := []byte("# Original\nbody\n")
	p := filepath.Join(dir, "a.md")
	if e := os.WriteFile(p, body, 0600); e != nil {
		t.Fatal(e)
	}
	cmd("add", "a.md")
	cmd("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fixture")
	in := doccorpus.Input{Path: "a.md", Revision: cmd("rev-parse", "HEAD"), Blob: cmd("rev-parse", "HEAD:a.md"), SHA256: digest(body)}
	if e := os.WriteFile(p, []byte("changed"), 0600); e != nil {
		t.Fatal(e)
	}
	s, e := load(context.Background(), dir, in)
	if e != nil || strings.Join(s.lines, "") != string(body) {
		t.Fatal(s, e)
	}
	in.SHA256 = "bad"
	if _, e = load(context.Background(), dir, in); e == nil {
		t.Fatal("accepted bad digest")
	}
	in.Blob = strings.Repeat("0", 40)
	if _, e = load(context.Background(), dir, in); e == nil {
		t.Fatal("accepted bad blob")
	}
}
