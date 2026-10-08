package testplan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

func withState(edit func(*testvalidity.Projection)) testvalidity.Projection {
	p := passing()
	edit(&p)
	return p
}

// TestWitnessVerdicts covers every TCN-V0-004 rejection reason and a qualifying reuse that keeps
// all five axes unchanged.
func TestWitnessVerdicts(t *testing.T) {
	goSession := testvaliditydoc.Document{Schema: testvaliditydoc.Schema, Source: "corvint-go-test-provider", Kind: "go-session", Tier: "preview",
		Tests: []testvaliditydoc.Test{{ID: "go-preview", Name: "TestX", State: "pass", Projection: passing()}}}
	jsUnit := e2eDocument("js-unit", "", passing())
	jsUnit.Kind = "unit"
	documents := []testvaliditydoc.Document{
		e2eDocument("ok", "chromium", passing()),
		e2eDocument("failed", "", withState(func(p *testvalidity.Projection) { p.Execution.State = testvalidity.ExecutionFailed })),
		e2eDocument("stale", "", withState(func(p *testvalidity.Projection) { p.Freshness.State = testvalidity.FreshnessStale })),
		e2eDocument("unbound", "", withState(func(p *testvalidity.Projection) { p.Freshness.State = testvalidity.FreshnessUnknown })),
		e2eDocument("unassociated", "", withState(func(p *testvalidity.Projection) { p.Association.State = "UNSUPPORTED" })),
		e2eDocument("ineligible", "", withState(func(p *testvalidity.Projection) { p.Hygiene.State = "QUARANTINED" })),
		e2eDocument("split", "", passing()),
		e2eDocument("split", "", withState(func(p *testvalidity.Projection) { p.Execution.State = testvalidity.ExecutionFailed })),
		goSession, jsUnit,
	}
	cases := map[string]string{
		"absent":       WitnessNotFound,
		"failed":       WitnessNotPassed,
		"stale":        WitnessStale,
		"unbound":      WitnessUnbound,
		"unassociated": WitnessNotAssociated,
		"ineligible":   WitnessIneligible,
		"split":        WitnessConflicting,
		"go-preview":   WitnessPreview,
		"js-unit":      WitnessPreview,
		"ok@firefox":   WitnessNotFound,
		"ok@chromium":  "",
		"ok":           "",
	}
	for name, want := range cases {
		id, project, _ := strings.Cut(name, "@")
		projection, got := judgeWitness(Witness{TestID: id, Project: project, Basis: "declared"}, documents)
		if got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
		if want == "" && !reflect.DeepEqual(projection, passing()) {
			t.Errorf("%s: projection changed: %+v", name, projection)
		}
	}

	// End to end: a rejected witness leaves the variation to planning and is reported.
	p := build(t, Evidence{Revision: none, TestsDigest: none, Documents: documents}, 8,
		variation("V1", set("witnesses", []any{map[string]any{"test_id": "failed", "basis": "declared"}, map[string]any{"test_id": "absent", "basis": "reviewed"}})),
		variation("V2", set("witnesses", []any{map[string]any{"test_id": "ok", "project": "chromium", "basis": "declared"}})),
		variation("V3", set("action", []any{"element:V2.act"}), set("assertion", []any{"element:V2.see"}),
			set("witnesses", []any{map[string]any{"test_id": "stale", "basis": "declared"}})),
	)
	wantRejections := []Rejection{
		{VariationID: "V1", TestID: "absent", Reason: WitnessNotFound},
		{VariationID: "V1", TestID: "failed", Reason: WitnessNotPassed},
		{VariationID: "V3", TestID: "stale", Reason: WitnessStale},
	}
	if !reflect.DeepEqual(p.WitnessRejections, wantRejections) {
		t.Fatalf("rejections %+v", p.WitnessRejections)
	}
	if len(p.Tests) != 1 || p.Tests[0].Steps[0].VariationID != "V1" || len(p.Reused) != 1 || p.Reused[0].VariationID != "V2" {
		t.Fatalf("plan:\n%s", p.Table())
	}
	if got := rows(p)[1]; got != "| REUSE ok@chromium | tests/e2e/teesheet.spec.ts | V2 | reused strength=NOT_MEASURED |" {
		t.Fatalf("reuse row %q", got)
	}
	var decoded struct {
		WitnessRejections []map[string]any `json:"witness_rejections"`
	}
	if err := json.Unmarshal(p.JSON(), &decoded); err != nil || len(decoded.WitnessRejections[0]) != 3 {
		t.Fatalf("witness_rejections members: %v %v", decoded, err)
	}
}

// TestWitnessesKeptSorted: every qualifying witness is kept, sorted by test_id then project, and the
// output does not depend on witness or document order (TCN-V0-004, TCN-V0-008).
func TestWitnessesKeptSorted(t *testing.T) {
	documents := []testvaliditydoc.Document{
		e2eDocument("t2", "", passing()), e2eDocument("t1", "chromium", passing()), e2eDocument("t1", "webkit", passing()),
	}
	witnesses := []any{
		map[string]any{"test_id": "t2", "basis": "declared"},
		map[string]any{"test_id": "t1", "project": "webkit", "basis": "reviewed"},
		map[string]any{"test_id": "t1", "project": "chromium", "basis": "declared"},
	}
	first := build(t, Evidence{Revision: none, TestsDigest: none, Documents: documents}, 8, variation("V1", set("witnesses", witnesses)))
	got := []string{}
	for _, w := range first.Reused[0].Witnesses {
		got = append(got, w.TestID+"@"+w.Project)
	}
	if strings.Join(got, ",") != "t1@chromium,t1@webkit,t2@" {
		t.Fatalf("witnesses %v", got)
	}
	reversed := []testvaliditydoc.Document{documents[2], documents[1], documents[0]}
	second := build(t, Evidence{Revision: none, TestsDigest: none, Documents: reversed}, 8,
		variation("V1", set("witnesses", []any{witnesses[2], witnesses[0], witnesses[1]})))
	if string(first.JSON()) != string(second.JSON()) {
		t.Fatal("witness or document order changed the output")
	}
}

func receipt(kind string, digests map[string]string) string {
	data, _ := json.Marshal(map[string]any{"receipt": map[string]any{
		"kind": kind, "identity": map[string]any{"testFileDigests": digests},
		"tests": []any{map[string]any{"name": "books a slot", "state": "passed"}},
	}})
	return string(data)
}

func digestOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func freshness(t *testing.T, req Request) testvalidity.Axis {
	t.Helper()
	evidence, err := gather(context.Background(), req, decode(t, inputJSON(t, variation("V1"))))
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	return evidence.Documents[0].Tests[0].Projection.Freshness
}

// TestBoundFreshnessAtRevision recomputes freshness against the evaluated revision, never the
// receipt's own currency (TCN-V0-004, LPCV-V0-053).
func TestBoundFreshnessAtRevision(t *testing.T) {
	const old, changed = "test('books', () => {})\n", "test('books', () => { expect(1) })\n"
	root := gitRepo(t, map[string]string{"tests/e2e/book.spec.ts": old, "tests/e2e/other.spec.ts": "x\n"})
	first := strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))
	book := filepath.Join(root, "tests", "e2e", "book.spec.ts")
	other := filepath.Join(root, "tests", "e2e", "other.spec.ts")
	if err := os.Symlink("book.spec.ts", filepath.Join(root, "tests", "e2e", "link.spec.ts")); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, root, map[string]string{"tests/e2e/book.spec.ts": changed})
	second := commit(t, root)

	check := func(name string, req Request, state, reason string) {
		t.Helper()
		req.Root = root
		got := freshness(t, req)
		if got.State != state || got.Reason != reason {
			t.Errorf("%s: got %s/%s, want %s/%s", name, got.State, got.Reason, state, reason)
		}
	}
	tests := func(digests map[string]string) []string {
		return []string{writeTemp(t, "receipt.json", receipt("e2e", digests))}
	}
	check("current at HEAD", Request{Tests: tests(map[string]string{book: digestOf(changed)})}, testvalidity.FreshnessCurrent, "")
	check("stale at HEAD", Request{Tests: tests(map[string]string{book: digestOf(old)})}, testvalidity.FreshnessStale, "retained-digest-mismatch")
	check("current at the old revision", Request{Tests: tests(map[string]string{book: digestOf(old)}), Revision: first}, testvalidity.FreshnessCurrent, "")
	check("stale at the old revision", Request{Tests: tests(map[string]string{book: digestOf(changed)}), Revision: first}, testvalidity.FreshnessStale, "retained-digest-mismatch")
	check("absent at the revision", Request{Tests: tests(map[string]string{filepath.Join(root, "tests", "gone.spec.ts"): digestOf(old)})}, testvalidity.FreshnessStale, "retained-digest-mismatch")
	check("outside the worktree", Request{Tests: tests(map[string]string{filepath.Join(t.TempDir(), "a.spec.ts"): digestOf(old)})}, testvalidity.FreshnessUnknown, "retained-bound-path-outside-worktree")
	check("names .git", Request{Tests: tests(map[string]string{filepath.Join(root, ".git", "HEAD"): digestOf(old)})}, testvalidity.FreshnessUnknown, "retained-bound-path-outside-worktree")
	check("relative path", Request{Tests: tests(map[string]string{"tests/e2e/book.spec.ts": digestOf(changed)})}, testvalidity.FreshnessUnknown, "retained-bound-path-outside-worktree")
	check("symlink is unreadable", Request{Tests: tests(map[string]string{filepath.Join(root, "tests", "e2e", "link.spec.ts"): digestOf(changed)})}, testvalidity.FreshnessUnknown, "retained-bound-source-unreadable")
	if second == first {
		t.Fatal("fixture did not commit")
	}

	// A bound path dirty in the worktree is UNKNOWN even when the revision content matches.
	writeFiles(t, root, map[string]string{"tests/e2e/book.spec.ts": "edited\n", "tests/e2e/new.spec.ts": "new\n"})
	check("uncommitted edit", Request{Tests: tests(map[string]string{book: digestOf(changed)})}, testvalidity.FreshnessUnknown, reasonUncommitted)
	check("untracked file", Request{Tests: tests(map[string]string{filepath.Join(root, "tests", "e2e", "new.spec.ts"): digestOf("new\n")})}, testvalidity.FreshnessUnknown, reasonUncommitted)
	check("clean path beside a dirty one", Request{Tests: tests(map[string]string{other: digestOf("x\n")})}, testvalidity.FreshnessCurrent, "")
	// One mismatch is STALE even beside an UNKNOWN path.
	check("stale wins over unknown", Request{Tests: tests(map[string]string{book: digestOf(changed), other: digestOf("y\n")})}, testvalidity.FreshnessStale, "retained-digest-mismatch")

	// Reading changed nothing.
	if status := git(t, root, "status", "--porcelain"); !strings.Contains(status, "book.spec.ts") || strings.Contains(status, ".corvint") {
		t.Fatalf("status %q", status)
	}
}

// TestProviderDocumentRefusals: a corvint-test-validity/0 output document, undecodable input,
// a missing file or a directory refuse with invalid-test-validity-receipt; the tests digest
// does not depend on file order (TCN-V0-004, TCN-V0-009).
func TestProviderDocumentRefusals(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.spec.ts": "a\n"})
	output, _ := json.Marshal(e2eDocument("t1", "", passing()))
	cases := map[string]string{
		"test-validity output": writeTemp(t, "output.json", string(output)),
		"undecodable":          writeTemp(t, "broken.json", `{"receipt":`),
		"unknown kind":         writeTemp(t, "kind.json", receipt("integration", map[string]string{})),
		"missing":              filepath.Join(t.TempDir(), "absent.json"),
		"directory":            t.TempDir(),
	}
	for name, path := range cases {
		_, err := Run(context.Background(), Request{Root: root, Input: inputJSON(t, variation("V1")), Tests: []string{path}})
		if code(err) != "invalid-test-validity-receipt" {
			t.Errorf("%s: got %v", name, err)
		}
	}
	a := writeTemp(t, "a.json", receipt("e2e", map[string]string{filepath.Join(root, "a.spec.ts"): digestOf("a\n")}))
	b := writeTemp(t, "b.json", receipt("unit", map[string]string{filepath.Join(root, "a.spec.ts"): digestOf("b\n")}))
	in := inputJSON(t, variation("V1"))
	one, err := Run(context.Background(), Request{Root: root, Input: in, Tests: []string{a, b}})
	if err != nil {
		t.Fatal(err)
	}
	two, err := Run(context.Background(), Request{Root: root, Input: in, Tests: []string{b, a}})
	if err != nil {
		t.Fatal(err)
	}
	if string(one.JSON()) != string(two.JSON()) || one.TestsDigest == none || one.EvaluatedRevision == none {
		t.Fatalf("file order changed the plan or digests are absent:\n%s\n%s", one.JSON(), two.JSON())
	}
}

// TestRunArgumentRefusals: Run refuses out-of-range options before reading anything (TCN-V0-001).
func TestRunArgumentRefusals(t *testing.T) {
	root := gitRepo(t, map[string]string{"a": "a\n"})
	in := inputJSON(t, variation("V1"))
	nine := make([]string, MaxEvidenceFiles+1)
	cases := map[string]Request{
		"max-steps low":          {Root: root, Input: in, MaxSteps: 1},
		"max-steps high":         {Root: root, Input: in, MaxSteps: 33},
		"too many tests":         {Root: root, Input: in, Tests: nine},
		"too many maps":          {Root: root, Input: in, Maps: nine},
		"revision without files": {Root: root, Input: in, Revision: "HEAD"},
		"not a worktree":         {Root: t.TempDir(), Input: in, Tests: []string{writeTemp(t, "r.json", receipt("e2e", map[string]string{}))}},
		"unknown revision":       {Root: root, Input: in, Revision: "no-such-ref", Tests: []string{writeTemp(t, "r.json", receipt("e2e", map[string]string{}))}},
	}
	for name, req := range cases {
		if _, err := Run(context.Background(), req); code(err) != "test-plan-invalid-arguments" {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if _, err := Run(context.Background(), Request{Root: root, Input: []byte(`{}`)}); code(err) != "test-plan-invalid-input" {
		t.Errorf("invalid input: got %v", err)
	}
	p, err := Run(context.Background(), Request{Root: root, Input: in})
	if err != nil || p.EvaluatedRevision != none || p.MaxSteps != DefaultMaxSteps {
		t.Fatalf("plain run: %v %+v", err, p)
	}
}
