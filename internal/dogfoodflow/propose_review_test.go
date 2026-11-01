package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
)

// repinSpec is the cited spec of the re-pin fixtures: its contract sentence
// is line 5, and repinShift moves it to line 8 at the mainline BASE.
const (
	repinSpecHead = "# Widget\n\nintro one\nintro two\n"
	repinContract = "The widget contract lives in a.go.\n"
	repinSpecTail = "outro one\noutro two\noutro three\noutro four\n"
	repinShift    = "// mainline one\n// mainline two\n// mainline three\n"
)

// repinFixture is a lane that drafts its plan at BASE B1, citing span of
// docs/spec.md for its a.go hunk, then merges a mainline commit B2 whose spec
// is mainSpec of the B1 spec. The prepared map still names B1; when bound,
// the citation-plan binding records the plan against that map, as the last
// bind would. It returns the root, B2 and the plan's path.
func repinFixture(t *testing.T, span string, mainSpec func(string) string, bound bool) (string, string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	spec := repinSpecHead + repinContract + repinSpecTail
	source := "package w\n\n" + numbered("\t// A", 30)
	writeTree(t, root, map[string]string{"docs/spec.md": spec, "a.go": source})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	b1 := testGit(t, root, "rev-parse", "HEAD")
	mainline := testGit(t, root, "rev-parse", "--abbrev-ref", "HEAD")
	testGit(t, root, "checkout", "-q", "-b", "lane")
	writeTree(t, root, map[string]string{"a.go": strings.Replace(source, "\t// A line 20\n", "\t// A line 20 lane\n", 1)})
	testGit(t, root, "commit", "-q", "-am", "lane")
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: b1}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	drafted := textLines(stdout.Bytes())
	if len(drafted) != 1 {
		t.Fatalf("lane draft = %q", drafted)
	}
	id, _, _ := strings.Cut(drafted[0], "\t")
	plan := id + "\tdocs/spec.md\t" + span + "\tspecification\n"
	planPath := writeInput(t, plan)

	testGit(t, root, "checkout", "-q", mainline)
	writeTree(t, root, map[string]string{"docs/spec.md": mainSpec(spec)})
	testGit(t, root, "commit", "-q", "-am", "mainline")
	b2 := testGit(t, root, "rev-parse", "HEAD")
	testGit(t, root, "checkout", "-q", "lane")
	testGit(t, root, "merge", "-q", "--no-edit", mainline)
	writeTree(t, root, map[string]string{".corvint/change.cem.json": `{"baseRevision":"` + b1 + `","hunks":[]}` + "\n"})
	if bound {
		bindPlan(t, root, plan)
	}
	return root, b2, planPath
}

// bindPlan records plan in the citation-plan binding against an empty map.
func bindPlan(t *testing.T, root, plan string) {
	t.Helper()
	dir := filepath.Join(root, ".git", "corvint")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+citationBinding, []byte(sha256Hex([]byte(plan))+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// refreshAt runs propose-citations --refresh plan at base and returns its
// rows and stderr, with the fake map removed so a precheck can follow.
func refreshAt(t *testing.T, root, base, plan string) ([]string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base, Refresh: plan}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations --refresh = %d, %v\n%s", code, err, stderr.String())
	}
	return textLines(stdout.Bytes()), stderr.String()
}

// prechecked fails t unless the plan rows pass the precheck at base.
func prechecked(t *testing.T, root, base string, rows []string) {
	t.Helper()
	_ = os.Remove(filepath.Join(root, ".corvint", "change.cem.json"))
	next := writeInput(t, strings.Join(rows, "\n")+"\n")
	if code, out, errText, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: next}); code != 0 {
		t.Fatalf("the refreshed plan was refused: code=%d\nstdout=%s\nstderr=%s\nplan=%q", code, out, errText, rows)
	}
}

// TestProposeCitationsDCWV0842VanishedCiteRowGoesUnknown pins review finding
// 1 of cit-w12: when mainline rewrites the cited sentence, --refresh never
// keeps the old coordinates, which now hold other text the vet would admit;
// the row is carried unknown, its NOTE marked stale and naming the span.
func TestProposeCitationsDCWV0842VanishedCiteRowGoesUnknown(t *testing.T) {
	root, b2, plan := repinFixture(t, "5:5", func(spec string) string {
		return strings.Replace(spec, repinContract, "A different sentence now.\n", 1)
	}, true)
	rows, stderr := refreshAt(t, root, b2, plan)
	if len(rows) != 1 {
		t.Fatalf("refreshed rows = %q\n%s", rows, stderr)
	}
	_, rest, _ := strings.Cut(rows[0], "\t")
	if !strings.HasPrefix(rest, "unknown\tinsufficient-evidence\t"+staleCitePrefix+"docs/spec.md 5:5, review] ") || !strings.Contains(rest, "no longer occur at BASE "+b2) {
		t.Errorf("the vanished cite row was not carried unknown and stale: %q\n%s", rows[0], stderr)
	}
	if !strings.Contains(stderr, "refresh row 1 made unknown: ") || strings.Contains(stderr, " re-pinned to ") {
		t.Errorf("the carry did not name the vanished span:\n%s", stderr)
	}
	prechecked(t, root, b2, rows)
}

// TestProposeCitationsDCWV0842UnboundPlanIsNotRepinnedFromMapBase pins review
// finding 2 of cit-w12: a plan already refreshed to B2 is refreshed again
// while the map still names B1. No binding records that plan against the map,
// so its rows are not re-pinned from B1, which would move the valid 8:8 onto
// B1's line-8 text at B2 line 11; a NOTE says why.
func TestProposeCitationsDCWV0842UnboundPlanIsNotRepinnedFromMapBase(t *testing.T) {
	root, b2, plan := repinFixture(t, "5:5", func(spec string) string {
		return strings.Replace(spec, repinSpecHead, repinSpecHead+repinShift, 1)
	}, true)
	rows, stderr := refreshAt(t, root, b2, plan)
	_, rest, _ := strings.Cut(rows[0], "\t")
	if len(rows) != 1 || rest != "docs/spec.md\t8:8\tspecification" {
		t.Fatalf("the bound plan was not re-pinned to 8:8: %q\n%s", rows, stderr)
	}
	again := writeInput(t, strings.Join(rows, "\n")+"\n")
	rows, stderr = refreshAt(t, root, b2, again)
	_, rest, _ = strings.Cut(rows[0], "\t")
	if len(rows) != 1 || rest != "docs/spec.md\t8:8\tspecification" || strings.Contains(stderr, " re-pinned to ") {
		t.Errorf("an unbound B2 plan was re-pinned from the map's B1: %q\n%s", rows, stderr)
	}
	if !strings.Contains(stderr, "NOTE refresh-base-unproven") {
		t.Errorf("no NOTE said the map's BASE is unproven:\n%s", stderr)
	}
	prechecked(t, root, b2, rows)
}

// TestProposeCitationsDCWV0842ByteSpanRowIsRepinnedAndVetted pins review
// finding 3 of cit-w12: a valid bytes:START:END row is re-pinned by bytes and
// admitted by the vet, never turned into an unknown row.
func TestProposeCitationsDCWV0842ByteSpanRowIsRepinnedAndVetted(t *testing.T) {
	start := len(repinSpecHead)
	span := fmt.Sprintf("%s%d:%d", bytesSpanPrefix, start, start+len(repinContract))
	moved := fmt.Sprintf("%s%d:%d", bytesSpanPrefix, start+len(repinShift), start+len(repinShift)+len(repinContract))
	root, b2, plan := repinFixture(t, span, func(spec string) string {
		return strings.Replace(spec, repinSpecHead, repinSpecHead+repinShift, 1)
	}, true)
	rows, stderr := refreshAt(t, root, b2, plan)
	_, rest, _ := strings.Cut(rows[0], "\t")
	if len(rows) != 1 || rest != "docs/spec.md\t"+moved+"\tspecification" || !strings.Contains(stderr, span+" re-pinned to "+moved) {
		t.Fatalf("the byte-span row was not re-pinned to %s: %q\n%s", moved, rows, stderr)
	}
	// Refreshed again at the same BASE, the vet admits the byte span as is.
	_ = os.Remove(filepath.Join(root, ".corvint", "change.cem.json"))
	rows, stderr = refreshAt(t, root, b2, writeInput(t, strings.Join(rows, "\n")+"\n"))
	_, rest, _ = strings.Cut(rows[0], "\t")
	if len(rows) != 1 || rest != "docs/spec.md\t"+moved+"\tspecification" {
		t.Errorf("the vet replaced a valid byte-span row: %q\n%s", rows, stderr)
	}
	prechecked(t, root, b2, rows)
}

// TestProposeCitationsDCWV0843NoEvidenceUnknownIsAPlanRow pins review finding
// 4 of cit-w12: a sealed CEM hunk left unknown with the prepared no-evidence
// reason becomes an insufficient-evidence row that keeps the CEM's reason in
// its NOTE, so the reseal plan passes the precheck.
func TestProposeCitationsDCWV0843NoEvidenceUnknownIsAPlanRow(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{"docs/spec.md": repinSpecHead + repinContract + repinSpecTail, "a.go": "package w\n\n" + numbered("\t// A", 30)})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	b1 := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"a.go": strings.NewReplacer("\t// A line 5\n", "\t// A line 5 lane\n", "\t// A line 25\n", "\t// A line 25 lane\n").Replace("package w\n\n" + numbered("\t// A", 30)),
	})
	testGit(t, root, "commit", "-q", "-am", "lane")
	const mapArg = ".corvint/change.cem.json"
	var out, errs bytes.Buffer
	for _, args := range [][]string{
		{"prepare", "--base", b1, "--target", testGit(t, root, "rev-parse", "HEAD")},
		{"cite", "--map", mapArg, "--hunk", "1", "--evidence-path", "docs/spec.md", "--lines", "5:5", "--relation", "specification"},
	} {
		if code := cemcli.Run(context.Background(), root, args, &out, &errs); code != 0 {
			t.Fatalf("cem %q = %d\n%s%s", args, code, &out, &errs)
		}
	}
	sealed, err := os.ReadFile(filepath.Join(root, mapArg))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(sealed, []byte(`"no-evidence"`)) {
		t.Fatalf("the prepared map has no no-evidence hunk:\n%s", sealed)
	}
	if err := os.Remove(filepath.Join(root, mapArg)); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: b1, FromCEM: writeInput(t, string(sealed))}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations --from-cem = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	unknowns := 0
	for _, row := range rows {
		_, rest, _ := strings.Cut(row, "\t")
		if strings.HasPrefix(rest, "unknown\t") {
			unknowns++
			if !strings.HasPrefix(rest, "unknown\tinsufficient-evidence\tcarried from the sealed CEM ") || !strings.Contains(rest, " with reason no-evidence ") {
				t.Errorf("a no-evidence hunk's row = %q", row)
			}
		}
	}
	if len(rows) != 2 || unknowns != 1 {
		t.Fatalf("reseal rows = %q; want one cite row and one unknown row\n%s", rows, stderr.String())
	}
	prechecked(t, root, b1, rows)
}
