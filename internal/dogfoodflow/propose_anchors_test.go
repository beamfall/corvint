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
	for _, heading := range []string{"# src/widget.go\n\n## Widget contract\n", "src/widget.go\n=============\n\n"} {
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
				if fields[1] != want || substantive && fields[2] != "4:4" {
					t.Fatalf("heading draft: %s", out)
				}
			})
		}
	}
}

// DWC-V0-928: use the unchanged BASE definition rather than edited body text,
// including a definition in hunk context. Removed definitions remain uncertain.
func TestProposeCitationsDWCV0928BaseGoverningDefinition(t *testing.T) {
	for _, mode := range []string{"stable", "rewrite", "repeat"} {
		t.Run(mode, func(t *testing.T) {
			spec := "# Widgets\n\n## Requirements\n\n- `XYZ-V0-001`: widgets MUST frob.\n  XYZ-V0-001 currently frobs once.\n  Further details stay here.\n"
			changed := strings.Replace(spec, "currently frobs once", "now frobs twice", 1)
			if mode == "rewrite" {
				changed = strings.Replace(changed, "widgets MUST frob.", "widgets MUST frob twice.", 1)
			}
			if mode == "repeat" {
				changed += "\n- `XYZ-V0-001`: widgets MUST frob.\n"
			}
			out, _ := anchoredDraft(t, map[string]string{"docs/specs/widget.md": spec}, map[string]string{"docs/specs/widget.md": changed})
			fields := strings.Split(strings.TrimSpace(out), "\t")
			if mode == "stable" && (fields[1] != "docs/specs/widget.md" || fields[2] != "5:5") {
				t.Fatalf("definition: %s", out)
			}
			if mode != "stable" && fields[1] != "unknown" {
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
		blob := []byte(testGit(t, root, "show", base+":"+f[1]) + "\n")
		byteSpan, ok := lineByteSpan(blob, f[2])
		if !ok {
			t.Fatalf("invalid BASE span: %q", row)
		}
		span := spanBytes(blob, byteSpan)
		if kinds, _ := verify.SpanAnchors(h, span); len(kinds) == 0 {
			t.Errorf("unanchored guess: %q cites %q", row, span)
		}
		if strings.HasPrefix(strings.TrimSpace(string(span)), "#") && strings.HasSuffix(f[1], ".md") {
			t.Errorf("bare heading: %q", row)
		}
	}
}
