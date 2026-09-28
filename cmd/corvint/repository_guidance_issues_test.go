package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

func TestRepositoryGuidanceMarkerBoundaries(t *testing.T) {
	// RGV-V0-004: whole marker values, never serialized string contents.
	root, _ := guidanceFixture(t)
	guidanceWrite(t, root, "markers.go", "package demo\n"+
		"var _ = `{"+`"note":"feature: tiny","authority":"accepted-spec"`+"}`\n"+
		"var _ = \"scenario: in a string\"\n"+
		"var _ = `\n// Feature: raw string\n`\n"+
		"// Feature: short-name (v2)\n"+
		"/* Scenario: block marker */\n"+
		"// Feature: "+strings.Repeat("x", 121)+"\n"+
		"//line fake.go:100\n// Feature: physical high\n//line fake.go:2\n// Scenario: physical low\n")
	guidanceGit(t, root, "add", ".")
	guidanceGit(t, root, "commit", "-qm", "markers")
	before := treeDigest(t, root)
	out, _ := guidanceRead(t, root, "features", "")
	labels := map[string]bool{}
	for _, f := range out.Features {
		labels[f.Label] = true
		if f.Label == "physical high" && f.Evidence[0].Start != 11 {
			t.Fatalf("wrong physical line: %+v", f)
		}
		if f.Label == "physical low" && f.Evidence[0].Start != 13 {
			t.Fatalf("wrong physical line: %+v", f)
		}
	}
	for _, name := range []string{"greeting", "short-name (v2)", "block marker", "physical high", "physical low"} {
		if !labels[name] {
			t.Errorf("missing valid marker %q: %v", name, labels)
		}
	}
	for name := range labels {
		if strings.Contains(name, "tiny") || strings.Contains(name, "string") || strings.HasPrefix(name, "xxxx") {
			t.Errorf("contaminated marker %q", name)
		}
	}
	if treeDigest(t, root) != before {
		t.Fatal("marker read mutated repository")
	}
}

func TestRepositoryGuidanceSourceSuffixes(t *testing.T) {
	// RGV-V0-005: suffixes carry only the language certainty they justify.
	for _, suffix := range []string{".cc", ".cxx", ".hpp", ".h", ".unknown"} {
		t.Run(suffix, func(t *testing.T) {
			root, _ := guidanceFixture(t)
			guidanceGit(t, root, "rm", "-r", "go.mod", "cmd", "web", "package.json")
			guidanceWrite(t, root, "main"+suffix, "int main() {}\n")
			guidanceGit(t, root, "add", ".")
			guidanceGit(t, root, "commit", "-qm", "source suffix")
			out, _ := guidanceRead(t, root, "overview", "")
			cpp := false
			for _, l := range out.Languages {
				cpp = cpp || l == "C++"
			}
			want := suffix == ".cc" || suffix == ".cxx" || suffix == ".hpp"
			if cpp != want {
				t.Fatalf("languages %v", out.Languages)
			}
			if suffix == ".h" && out.Omissions["ambiguous-header-language"] != 1 {
				t.Fatal("ambiguous header not reported")
			}
		})
	}
}

func TestRepositoryGuidanceIndexFreshness(t *testing.T) {
	// RGV-V0-005: an overview never builds or repairs optional index state.
	for _, state := range []string{"absent", "matching", "same-tree", "stale", "unreadable"} {
		t.Run(state, func(t *testing.T) {
			root, _ := guidanceFixture(t)
			if state != "absent" {
				idx, err := contextindex.Build(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				saved, err := contextindex.WriteSnapshot(idx)
				if err != nil {
					t.Fatal(err)
				}
				switch state {
				case "same-tree":
					guidanceGit(t, root, "commit", "--allow-empty", "-qm", "same tree")
				case "stale":
					guidanceWrite(t, root, "new.txt", "new tree\n")
					guidanceGit(t, root, "add", ".")
					guidanceGit(t, root, "commit", "-qm", "new tree")
				case "unreadable":
					if err := os.WriteFile(saved.Path, []byte("corrupt snapshot"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			before := treeDigest(t, root)
			out, _ := guidanceRead(t, root, "overview", "")
			if state == "matching" || state == "same-tree" {
				if out.Index["state"] != "MATCHING" || out.Index["tree"] != out.Tree || out.Index["revision"] != out.Revision {
					t.Fatalf("index %v", out.Index)
				}
			} else if out.Index["state"] != "UNKNOWN" || out.Omissions["index"] != 1 || strings.Contains(out.Index["reason"], "not-read") {
				t.Fatalf("missing explicit index gap: %v, %v", out.Index, out.Omissions)
			}
			if treeDigest(t, root) != before {
				t.Fatal("overview changed index/repository")
			}
		})
	}
}
