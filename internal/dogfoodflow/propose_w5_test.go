package dogfoodflow

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// DCW-V0-901 (V1-3335): an abbreviated BASE resolves to its full ID, and a
// BASE that names no commit is refused on stderr by name with its fix, never
// a silent exit 2.
func TestProposeCitationsDCWV0901ResolvesAnAbbreviatedBase(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{"a.txt": "a\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{"a.txt": "b\n"})
	testGit(t, root, "commit", "-q", "-am", "change")
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base[:8]}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("abbreviated BASE: exit %d, %v\n%s", code, err, stderr.String())
	}
	if !strings.Contains(stderr.String(), " base="+base+" ") {
		t.Fatalf("PROPOSED line does not name the full BASE %s:\n%s", base, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code, _ := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: "0badc0de"}, &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "dogfood-propose-citations: REFUSE base-unavailable BASE=0badc0de\n  fix: ") ||
		!strings.Contains(stderr.String(), "git rev-parse --verify 0badc0de^{commit}") {
		t.Fatalf("unknown BASE: exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
}

// DCW-V0-902 (V1-3444): the patch-unavailable fix line no longer tells the
// author to drop a binary file, which binds beside a textual hunk; it names
// the binary shapes cem prepare still refuses.
func TestProposeCitationsDCWV0902NamesOnlyRefusedBinaryShapes(t *testing.T) {
	var stderr bytes.Buffer
	p := &proposal{flow: flow{ctx: context.Background(), prefix: "dogfood-propose-citations", stderr: &stderr}, options: ProposeOptions{Base: "b0"}}
	code, _ := boundary(context.Background(), func() int { p.refusePatchUnavailable("binary-patch", "the patch has a binary section"); return 0 }, func() {})
	out := stderr.String()
	if code != 2 || strings.Contains(out, "drop a binary file") {
		t.Fatalf("exit %d, fix still drops binaries:\n%s", code, out)
	}
	for _, want := range []string{"a binary file beside a textual hunk binds (CEM-V1-980)", "binary-only patch", "binary rename or copy", "binary mode change", "GIT binary patch", "abbreviated object IDs", "drop a symbolic link"} {
		if !strings.Contains(out, want) {
			t.Errorf("fix line lacks %q:\n%s", want, out)
		}
	}
}

// DCW-V0-906 (V1-3483): the defining spec's hunk that adds a requirement whose
// entry refines a BASE requirement is drafted as a proposal of that BASE
// definition; a new requirement whose entry names no BASE requirement stays
// unknown.
func TestProposeCitationsDCWV0906DraftsTheRefinedBaseForADefiningHunk(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	spec := func(prefix, added string) string {
		return "# " + prefix + " v0\n\n## Requirements\n\n- `" + prefix + "-V0-001`: the widget MUST frob.\n" + numbered("requirement", 8) + added + "\n## Rollback\n\nNone.\n"
	}
	writeTree(t, root, map[string]string{"docs/specs/x-v0.md": spec("XYZ", ""), "docs/specs/y-v0.md": spec("YYY", "")})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"docs/specs/x-v0.md": spec("XYZ", "- `XYZ-V0-900`: (experimental; refines `XYZ-V0-001`) the widget MUST frob twice.\n"),
		"docs/specs/y-v0.md": spec("YYY", "- `YYY-V0-900`: (experimental) the gadget MUST spin.\n"),
	})
	testGit(t, root, "commit", "-q", "-am", "change")
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	if len(rows) != 2 {
		t.Fatalf("rows = %q, want two", rows)
	}
	var refined, plain string
	for _, row := range rows {
		switch {
		case strings.Contains(row, "\tdocs/specs/x-v0.md\t5:5\t"):
			refined = row
		case strings.Contains(row, "\tunknown\t") && strings.Contains(row, "YYY-V0-900"):
			plain = row
		}
	}
	if refined == "" || plain == "" {
		t.Fatalf("want the XYZ-V0-900 hunk drafted to its refined XYZ-V0-001 line and the YYY-V0-900 hunk unknown:\n%s\n%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "the BASE definition of XYZ-V0-001, which the entry of the same-change requirement XYZ-V0-900 refines") {
		t.Fatalf("the drafted proposal does not say why:\n%s", stderr.String())
	}
}
