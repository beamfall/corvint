package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
)

// TestProposeCitationsDCWV0843ResealFromSealedCEM pins V1-3502: a lane binds
// and seals its CEM at BASE B1, merges a mainline commit B2 that shifts the
// cited lines, and drafts its reseal plan with --from-cem instead of
// re-deriving every row. Each sealed hunk is carried by its ID with content
// unchanged, the cite row is re-pinned to the moved line, the unknown rows
// keep the CEM's reason, and the plan passes the precheck at B2. Naming both
// sources, or an unreadable CEM, is refused with exit 2.
func TestProposeCitationsDCWV0843ResealFromSealedCEM(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	spec := "# Widget\n\n" + numbered("intro", 2) + "The widget contract lives in a.go.\n" + numbered("outro", 4)
	source := func(name string) string {
		return "package w\n\n// " + name + " is a function.\nfunc " + name + "() int {\n" + numbered("\t// "+name, 40) + "\treturn 1\n}\n"
	}
	writeTree(t, root, map[string]string{"docs/spec.md": spec, "a.go": source("A"), "b.go": source("B")})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	b1 := testGit(t, root, "rev-parse", "HEAD")
	mainline := testGit(t, root, "rev-parse", "--abbrev-ref", "HEAD")
	testGit(t, root, "checkout", "-q", "-b", "lane")
	writeTree(t, root, map[string]string{
		"a.go": strings.NewReplacer("\t// A line 12\n", "\t// A line 12 lane\n", "\t// A line 24\n", "\t// A line 24 lane\n").Replace(source("A")),
		"b.go": strings.Replace(source("B"), "\t// B line 12\n", "\t// B line 12 lane\n", 1),
	})
	testGit(t, root, "commit", "-q", "-am", "lane")

	cem := func(args ...string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := cemcli.Run(context.Background(), root, args, &stdout, &stderr); code != 0 {
			t.Fatalf("cem %q = %d\n%s%s", args, code, &stdout, &stderr)
		}
	}
	const mapArg = ".corvint/change.cem.json"
	cem("prepare", "--base", b1, "--target", testGit(t, root, "rev-parse", "HEAD"))
	cem("cite", "--map", mapArg, "--hunk", "1", "--evidence-path", "docs/spec.md", "--lines", "5:5", "--relation", "specification")
	cem("mark", "--map", mapArg, "--hunk", "2", "--disposition", "unknown", "--reason", "insufficient-evidence")
	cem("mark", "--map", mapArg, "--hunk", "3", "--disposition", "unknown", "--reason", "insufficient-evidence")
	sealed, err := os.ReadFile(filepath.Join(root, mapArg))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, mapArg)); err != nil {
		t.Fatal(err)
	}
	cemPath := writeInput(t, string(sealed))

	testGit(t, root, "checkout", "-q", mainline)
	shift := "// mainline one\n// mainline two\n// mainline three\n"
	writeTree(t, root, map[string]string{
		"docs/spec.md": "# Widget\n\n" + shift + strings.TrimPrefix(spec, "# Widget\n\n"),
		"a.go":         "package w\n\n" + shift + strings.TrimPrefix(source("A"), "package w\n\n"),
		"b.go":         "package w\n\n" + shift + strings.TrimPrefix(source("B"), "package w\n\n"),
	})
	testGit(t, root, "commit", "-q", "-am", "mainline")
	b2 := testGit(t, root, "rev-parse", "HEAD")
	testGit(t, root, "checkout", "-q", "lane")
	testGit(t, root, "merge", "-q", "--no-edit", mainline)

	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: b2, FromCEM: cemPath}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations --from-cem = %d, %v\n%s", code, err, stderr.String())
	}
	output := stdout.String() + stderr.String()
	if !strings.Contains(stderr.String(), " from-cem=") || strings.Contains(output, "[stale:") || !strings.Contains(output, " dropped=0 ") {
		t.Errorf("the reseal draft is not a clean --from-cem refresh:\n%s", output)
	}
	if strings.Count(stderr.String(), "content unchanged") != 3 || !strings.Contains(stderr.String(), "docs/spec.md 5:5 re-pinned to 8:8") {
		t.Errorf("want three content-unchanged re-keys and the re-pinned cite row:\n%s", stderr.String())
	}
	rows := textLines(stdout.Bytes())
	cites, unknowns := 0, 0
	for _, row := range rows {
		_, rest, _ := strings.Cut(row, "\t")
		switch {
		case rest == "docs/spec.md\t8:8\tspecification":
			cites++
		case strings.HasPrefix(rest, "unknown\tinsufficient-evidence\tcarried from the sealed CEM "):
			unknowns++
		}
	}
	if len(rows) != 3 || cites != 1 || unknowns != 2 {
		t.Errorf("reseal rows = %q; want one re-pinned cite row and two carried unknown rows\n%s", rows, stderr.String())
	}
	next := filepath.Join(filepath.Dir(cemPath), "plan.new")
	if err := os.WriteFile(next, stdout.Bytes(), 0o666); err != nil {
		t.Fatal(err)
	}
	if code, out, errText, _ := preflightRun(t, root, ChangeOptions{Base: b2, Citations: next}); code != 0 {
		t.Fatalf("the reseal plan was refused: code=%d\nstdout=%s\nstderr=%s\nplan=%s", code, out, errText, stdout.String())
	}

	for name, options := range map[string]ProposeOptions{
		"refresh-source-conflict": {Root: root, Base: b2, FromCEM: cemPath, Refresh: next},
		"from-cem-unreadable":     {Root: root, Base: b2, FromCEM: writeInput(t, "not a cem\n")},
	} {
		stdout.Reset()
		stderr.Reset()
		if code, _ := ProposeCitations(context.Background(), options, &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "REFUSE "+name) {
			t.Errorf("%s: code=%d stdout=%q\n%s", name, code, stdout.String(), stderr.String())
		}
	}
}
