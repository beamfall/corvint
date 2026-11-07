// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"strings"
	"testing"
)

func anchoredDraft(t *testing.T, before, after map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	writeTree(t, root, before)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, after)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	var out, errOut bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &out, &errOut); code != 0 || err != nil {
		t.Fatalf("propose: %d %v\n%s", code, err, errOut.String())
	}
	return out.String(), errOut.String()
}

// DWC-V0-926: guesses through sibling and governing fallbacks stay unknown;
// full-path, basename >=6 bytes and exact requirement definitions remain useful.
func TestProposeCitationsDWCV0926OnlyAnchoredDrafts(t *testing.T) {
	for _, tc := range []struct{ name, file, evidence, change, want string }{
		{"Makefile status guess", "Makefile", "Intent status: accepted ASD-V0-1923.\nOther unrelated intent.\n", "all: changed\n", "unknown"},
		{"shell guess", "script/check-bash32.sh", "- `ASD-V0-1923`: enforce portable shell.\nOther unrelated intent.\n", "#!/bin/bash\ncheck_changed\n", "unknown"},
		{"full path", "script/check-bash32.sh", "Run `script/check-bash32.sh` before shipping.\nOther intent.\n", "#!/bin/bash\ncheck_changed\n", "docs/specs/contract.md"},
		{"basename", "script/check-bash32.sh", "Run `check-bash32.sh` before shipping.\nOther intent.\n", "#!/bin/bash\ncheck_changed\n", "docs/specs/contract.md"},
		{"short basename", "src/x.go", "Build x.go before shipping.\nOther intent.\n", "package x\n// changed\n", "unknown"},
		{"definition", "src/widget.go", "- `XYZ-V0-001`: widgets MUST frob.\nOther intent.\n", "package x\n// XYZ-V0-001 implements frobbing.\n", "docs/specs/contract.md"},
		{"mere ID mention", "src/widget.go", "Intent status: XYZ-V0-001 accepted.\nOther intent.\n", "package x\n// XYZ-V0-001 implements frobbing.\n", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, _ := anchoredDraft(t, map[string]string{tc.file: "old one\nold two\n", "docs/specs/contract.md": tc.evidence}, map[string]string{tc.file: tc.change})
			rows := textLines([]byte(out))
			if len(rows) != 1 || strings.Split(rows[0], "\t")[1] != tc.want {
				t.Fatalf("draft: %s", out)
			}
		})
	}
	// A new test used to inherit its sibling's first declaration with no anchor.
	out, _ := anchoredDraft(t, map[string]string{"src/widget.go": "package x\nfunc Widget() {}\n"}, map[string]string{"src/widget_test.go": "package x\n// test the widget\n"})
	if !strings.Contains(out, "\tunknown\t") {
		t.Fatalf("sibling guess: %s", out)
	}
}

// DWC-V0-927: a heading that names the exact path is still not substantive evidence.
func TestProposeCitationsDWCV0927NoHeadings(t *testing.T) {
	// The third heading's path appears only on the first line of a multiline setext heading.
	for _, heading := range []string{"# src/widget.go\n\n## Widget contract\n", "src/widget.go\n=============\n\n", "src/widget.go\nImplementation notes\n===\n\n"} {
		for _, substantive := range []bool{false, true} {
			t.Run(fmt.Sprint(heading, substantive), func(t *testing.T) {
				evidence := heading
				want := "unknown"
				if substantive {
					evidence += "The src/widget.go implementation MUST frob.\n"
					want = "docs/specs/contract.md"
				}
				out, _ := anchoredDraft(t, map[string]string{"src/widget.go": "old one\nold two\n", "docs/specs/contract.md": evidence}, map[string]string{"src/widget.go": "changed one\nchanged two\n"})
				fields := strings.Split(strings.TrimSpace(out), "\t")
				line := strings.Count(heading, "\n") + 1
				if fields[1] != want || substantive && fields[2] != fmt.Sprintf("%d:%d", line, line) {
					t.Fatalf("heading draft: %s", out)
				}
			})
		}
	}
}

// DWC-V0-928: use the unchanged BASE definition rather than edited body text,
// including a definition in hunk context. Removed definitions remain uncertain.
func TestProposeCitationsDWCV0928BaseGoverningDefinition(t *testing.T) {
	for _, mode := range []string{"stable", "continuation", "rewrite", "repeat"} {
		t.Run(mode, func(t *testing.T) {
			spec := "# Widgets\n\n## Requirements\n\n- `XYZ-V0-001`: widgets MUST frob.\n  XYZ-V0-001 currently frobs once.\n  Further details stay here.\n"
			changed := strings.Replace(spec, "currently frobs once", "now frobs twice", 1)
			if mode == "continuation" {
				changed = strings.Replace(changed, "XYZ-V0-001 now", "It now", 1)
			}
			if mode == "rewrite" {
				changed = strings.Replace(changed, "widgets MUST frob.", "widgets MUST frob twice.", 1)
			}
			if mode == "repeat" {
				changed += "\n- `XYZ-V0-001`: widgets MUST frob.\n"
			}
			out, notes := anchoredDraft(t, map[string]string{"docs/specs/widget.md": spec}, map[string]string{"docs/specs/widget.md": changed})
			fields := strings.Split(strings.TrimSpace(out), "\t")
			if (mode == "stable" || mode == "continuation") && (fields[1] != "docs/specs/widget.md" || fields[2] != "5:5") {
				t.Fatalf("definition: %s", out)
			}
			if mode == "continuation" && (!strings.Contains(notes, "CEM-V1-081 cannot anchor") || !strings.Contains(notes, "XYZ-V0-001")) {
				t.Fatalf("missing honest governing note: %s", notes)
			}
			if mode == "rewrite" && (!strings.Contains(notes, "governing requirement XYZ-V0-001") || !strings.Contains(notes, "review note only")) {
				t.Fatalf("missing rewritten-definition note: %s", notes)
			}
			if (mode == "rewrite" || mode == "repeat") && fields[1] == "docs/specs/widget.md" && fields[2] == "5:5" {
				t.Fatalf("edited definition: %s", out)
			}
		})
	}
}

// assertAnchoredDraft checks the real CEM anchor implementation against every
// emitted cite in the retained historical scenarios, including move/seal and
// sibling fallbacks. These scenarios used to require unanchored guesses.
func assertAnchoredDraft(t *testing.T, root, base, draft string) {
	t.Helper()
	raw := testGit(t, root, "diff", "--full-index", "--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers", "--unified=3", "--inter-hunk-context=0", base, "HEAD", "--", ".", ":(exclude).corvint/change.cem.json") + "\n"
	parsed, err := patch.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	rows := textLines([]byte(draft))
	if len(rows) != len(parsed.Hunks) {
		t.Fatalf("%d rows for %d hunks: %s", len(rows), len(parsed.Hunks), draft)
	}
	cites := 0
	for index, row := range rows {
		f := strings.Split(row, "\t")
		if len(f) != 4 {
			t.Fatalf("malformed row %q", row)
		}
		h := parsed.Hunks[index]
		if f[0] != h.ID && f[0] != fmt.Sprint(index+1) {
			t.Fatalf("wrong hunk key: %q", row)
		}
		if f[1] == "unknown" {
			if f[2] != "insufficient-evidence" || f[3] == "" {
				t.Fatalf("missing uncertainty: %q", row)
			}
			continue
		}
		cites++
		blob := []byte(testGit(t, root, "show", base+":"+f[1]) + "\n")
		byteSpan, ok := lineByteSpan(blob, f[2])
		if !ok {
			t.Fatalf("invalid BASE span: %q", row)
		}
		span := spanBytes(blob, byteSpan)
		if kinds, _ := verify.SpanAnchors(h, span); len(kinds) == 0 && f[1] != h.DisplayPath && !sealedCEMPath.MatchString(h.DisplayPath) {
			t.Errorf("unanchored guess: %q cites %q", row, span)
		}
		if strings.HasPrefix(strings.TrimSpace(string(span)), "#") && strings.HasSuffix(f[1], ".md") {
			t.Errorf("bare heading: %q", row)
		}
	} // A fully unknown draft must not vacuously satisfy this helper when
	// actual stable BASE naming evidence exists.
	if cites == 0 {
		for _, name := range strings.Fields(testGit(t, root, "ls-tree", "-r", "--name-only", base)) {
			blob := []byte(testGit(t, root, "show", base+":"+name) + "\n")
			for _, h := range parsed.Hunks {
				if kinds, _ := verify.SpanAnchors(h, blob); len(kinds) > 0 {
					t.Fatalf("all rows unknown despite BASE anchor in %s: %s", name, draft)
				}
			}
		}
	}

}

// DWC-V0-927: Markdown-looking source inside fences is content; list items
// and YAML keys cannot become setext paragraphs merely by preceding dashes.
func TestProposeCitationsDWCV0927MarkdownContexts(t *testing.T) {
	for _, evidence := range []string{
		"- src/widget.go\n---\n", "widget: src/widget.go\n---\n",
		"```sh\n# src/widget.go\n```\n", "~~~md\n# src/widget.go\n~~~\n",
		"1. src/widget.go\n---\n",
	} {
		out, _ := anchoredDraft(t, map[string]string{"src/widget.go": "old one\nold two\n", "docs/contract.md": evidence}, map[string]string{"src/widget.go": "new one\nnew two\n"})
		if !strings.Contains(out, "\tdocs/contract.md\t") {
			t.Fatalf("substantive content suppressed: %q: %s", evidence, out)
		}
	}
}

// DWC-V0-928 / DCW-V0-382: two separated continuation edits in one entry
// cannot consume the same governing definition twice.
func TestProposeCitationsDWCV0928DistinctGoverningSpans(t *testing.T) {
	spec := "# Widget\n\n## Requirements\n\n- `XYZ-V0-001`: widgets MUST frob.\n" + numbered("  detail", 35)
	after := strings.NewReplacer("  detail line 5\n", "  detail line 5 changed XYZ-V0-001\n", "  detail line 28\n", "  detail line 28 changed XYZ-V0-001\n").Replace(spec)
	out, notes := anchoredDraft(t, map[string]string{"docs/specs/widget.md": spec}, map[string]string{"docs/specs/widget.md": after})
	rows := textLines([]byte(out))
	if len(rows) != 2 {
		t.Fatalf("want two hunks: %s", out)
	}
	seen := map[string]bool{}
	for _, row := range rows {
		f := strings.Split(row, "\t")
		if f[1] == "unknown" {
			continue
		}
		key := f[1] + ":" + f[2]
		if seen[key] {
			t.Fatalf("duplicate governing span: %s\n%s", out, notes)
		}
		seen[key] = true
	}
	if !seen["docs/specs/widget.md:5:5"] {
		t.Fatalf("governing anchor missing: %s", out)
	}
}

// DWC-V0-926/928, DCW-V0-805/809: decision evidence may serve a rewritten
// definition when it names the exact spec, rather than sharing topic words.
func TestProposeCitationsDWCV0928DecisionNamesExactSpec(t *testing.T) {
	spec := "# Widgets\n\n## Requirements\n\n- `XYZ-V0-001`: (accepted, decision 0388) widgets MUST frob.\n  Frob within 20 seconds.\n"
	for _, text := range []string{"The contract for docs/specs/widget.md requires frobbing.\n", "- `XYZ-V0-001`: widgets MUST frob.\n"} {
		out, notes := anchoredDraft(t, map[string]string{
			"docs/specs/widget.md":          spec,
			"docs/decisions/0388-widget.md": "# Decision\n\n" + text,
		}, map[string]string{"docs/specs/widget.md": strings.Replace(spec, "widgets MUST frob.", "widgets MUST frob twice.", 1)})
		if !strings.Contains(out, "\tdocs/decisions/0388-widget.md\t3:3\tdecision") || !strings.Contains(notes, "governing requirement XYZ-V0-001") {
			t.Fatalf("lost evidenced decision: %s\n%s", out, notes)
		}
	}
}

// DWC-V0-926 / DCW-V0-962: a same-change ID reference cannot erase stable
// context in an existing file. The new definition still stays unknown.
func TestProposeCitationsDWCV0926SameChangeReferenceKeepsOwnContext(t *testing.T) {
	source := "package widget\n\nfunc Widget() {\n" + numbered(" // detail", 20) + "}\n"
	out, notes := anchoredDraft(t, map[string]string{"src/widget.go": source}, map[string]string{
		"src/widget.go":        strings.Replace(source, " // detail line 10\n", " // detail line 10 implements XYZ-V0-001\n", 1),
		"docs/specs/widget.md": "# Widget\n\n## Requirements\n\n- `XYZ-V0-001`: widgets MUST frob.\n",
	})
	if !strings.Contains(out, "\tsrc/widget.go\t") || !strings.Contains(out, "\tunknown\t") || !strings.Contains(notes, "drafted proposal:") {
		t.Fatalf("lost surviving context or new-intent uncertainty: %s\n%s", out, notes)
	}
}

// DWC-V0-926 / DCW-V0-548: once the repin contract span is consumed, a
// second repin hunk keeps its same-file context instead of an unrelated match.
func TestProposeCitationsDWCV0926SecondRepinUsesOwnContext(t *testing.T) {
	guide := "# Guide\n\n" + numbered("intro", 8) + "Read src/a.go:10@abcd.\n" + numbered("middle", 10) + "Read src/b.go:20@abcd.\n" + numbered("tail", 8)
	out, notes := anchoredDraft(t, map[string]string{
		"docs/guide.md":     guide,
		repinRuleSpec:       "# Gate\n\n" + definitionPattern(repinRequirement) + " relocate pinned spans.\n",
		"docs/unrelated.md": "# Index\n\nAn unrelated section lists docs/guide.md.\n",
	}, map[string]string{"docs/guide.md": strings.NewReplacer("src/a.go:10@abcd", "src/a.go:11@abcd", "src/b.go:20@abcd", "src/b.go:21@abcd").Replace(guide)})
	rows := textLines([]byte(out))
	if len(rows) != 2 || !strings.Contains(rows[0], "\t"+repinRuleSpec+"\t") || !strings.Contains(rows[1], "\tdocs/guide.md\t") || !proposalRow(notes, rows[1]) {
		t.Fatalf("second repin lost context: %s\n%s", out, notes)
	}
}
