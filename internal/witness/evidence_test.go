package witness

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

const evidenceRootCommit = "0123456789abcdef0123456789abcdef01234567"

// boundReceipt is an unprofiled provider receipt that ran on exactly the
// index's head commit and tree. One test imports the changed a/a.go; the
// other names only a file outside the range. The a/a.go digest is the head
// blob's, so the AGW-V0-009 comparison admits it; the escaping and foreign
// absolute paths resolve to no head file and are not compared.
func boundReceipt(index *contextindex.Index) jstestprovider.Receipt {
	identity := &jstestprovider.ApplicationRepositoryIdentity{RootCommit: evidenceRootCommit, Revision: index.CommitRevision, Tree: index.Revision, DirtyState: "clean"}
	publish := *identity
	return jstestprovider.Receipt{
		Kind: "unit", TestRepositoryAtStart: identity, TestRepositoryAtPublish: &publish,
		Tests: []jstestprovider.TestOutcome{
			{Name: "adds", FullName: "a adds", State: jstestprovider.StatePassed, Anchor: &jstestprovider.Anchor{File: "a/a.test.js", Line: 3},
				ImportedFiles: map[string]string{"a/a.go": headDigest(index, "a/a.go"), "../escape.go": "sha256:00", "/abs/a/a.go": "sha256:00"}},
			{Name: "elsewhere", FullName: "z elsewhere", State: jstestprovider.StateFailed, Anchor: &jstestprovider.Anchor{File: "z/z.test.js", Line: 1}},
		},
	}
}

// headDigest is the lowercase hex sha256 of a file in the fixture's clean
// checkout, which is its head blob.
func headDigest(index *contextindex.Index, file string) string {
	data, err := os.ReadFile(filepath.Join(index.Root, filepath.FromSlash(file)))
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeEvidence(t *testing.T, document any) string {
	t.Helper()
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "evidence.json")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

func providerDocument(receipt jstestprovider.Receipt) map[string]any {
	return map[string]any{"receipt": receipt}
}

// TestEvidenceAttachesNonClosingCallerReportedWitnesses pins AGW-V0-008: a
// receipt bound to the head tree attaches CALLER_REPORTED witnesses that close
// nothing, every verdict stays what it was without evidence, and the summary
// separates an open obligation with a candidate from one with nothing.
func TestEvidenceAttachesNonClosingCallerReportedWitnesses(t *testing.T) {
	index, base := packetFixture(t, "example.test/fixture")
	without, err := Compile(context.Background(), index, Options{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	file := writeEvidence(t, providerDocument(boundReceipt(index)))
	if _, err := testvaliditydoc.Decode(mustRead(t, file)); err != nil {
		t.Fatalf("the fixture is not an admitted provider document: %v", err)
	}
	report, err := Compile(context.Background(), index, Options{Base: base, Evidence: []string{file}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Sources) != 2 || report.Sources[0].Name != "cem" {
		t.Fatalf("sources = %+v", report.Sources)
	}
	evidence := report.Sources[1]
	if evidence.Name != "evidence" || evidence.Status != "BOUND" || evidence.Path != file || evidence.Hunks != 2 || evidence.Basis != 1 || evidence.Unattached != 1 {
		t.Fatalf("evidence source = %+v", evidence)
	}
	attached := 0
	for position, item := range report.Obligations {
		if item.Verdict != without.Obligations[position].Verdict || item.Path != without.Obligations[position].Path {
			t.Fatalf("evidence changed %s from %s to %s", item.Path, without.Obligations[position].Verdict, item.Verdict)
		}
		for _, candidate := range item.Witnesses {
			if candidate.Source != "evidence" {
				continue
			}
			attached++
			if item.Path != "a/a.go" || candidate.Authority != "CALLER_REPORTED" || candidate.Closing || candidate.Relation != "test-run:passed" || candidate.Test != "a adds" || candidate.EvidencePath != "a/a.test.js" {
				t.Fatalf("evidence witness on %s = %+v", item.Path, candidate)
			}
		}
	}
	if attached != 1 {
		t.Fatalf("attached %d evidence witnesses, want 1", attached)
	}
	summary := report.Summary
	if summary.Closed != without.Summary.Closed || summary.CandidateWitnessed < 1 || summary.CandidateWitnessed+summary.Unwitnessed != summary.Opened-summary.Closed {
		t.Fatalf("summary = %+v (without evidence %+v)", summary, without.Summary)
	}
	if summary.CandidateWitnessed != without.Summary.CandidateWitnessed+1 {
		t.Fatalf("candidate-witnessed = %d, want one more than %d", summary.CandidateWitnessed, without.Summary.CandidateWitnessed)
	}
	rendered := Render(report)
	for _, want := range []string{
		"open-with-candidate    ",
		"open-with-nothing      ",
		"evidence status=BOUND    path=" + file + " hunks=2 basis=1 unattached=1",
		"witness evidence hunk=- relation=test-run:passed authority=CALLER_REPORTED closing=false evidence=a/a.test.js test=\"a adds\"",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered report is missing %q\n%s", want, rendered)
		}
	}
	// The no-evidence report keeps its bytes apart from the two new counts.
	withoutJSON, _ := json.Marshal(without)
	if bytes.Contains(withoutJSON, []byte(`"test"`)) || len(without.Sources) != 1 {
		t.Fatalf("a report without --evidence carries evidence members: %s", withoutJSON)
	}
}

// TestEvidenceNotBoundToTheHeadTreeAttachesNothing pins the AGW-V0-008 binding:
// only a clean run recorded at start and publish on exactly the head commit
// and tree binds; every other document is UNBOUND with its reason.
func TestEvidenceNotBoundToTheHeadTreeAttachesNothing(t *testing.T) {
	index, base := packetFixture(t, "example.test/fixture")
	mutate := func(change func(*jstestprovider.Receipt)) any {
		receipt := boundReceipt(index)
		change(&receipt)
		return providerDocument(receipt)
	}
	cases := []struct {
		name     string
		document any
		reason   string
	}{
		{"tree", mutate(func(r *jstestprovider.Receipt) {
			r.TestRepositoryAtStart.Tree, r.TestRepositoryAtPublish.Tree = evidenceRootCommit, evidenceRootCommit
		}), "not the report head tree"},
		{"revision", mutate(func(r *jstestprovider.Receipt) {
			r.TestRepositoryAtStart.Revision, r.TestRepositoryAtPublish.Revision = evidenceRootCommit, evidenceRootCommit
		}), "not the report head " + index.CommitRevision},
		{"dirty", mutate(func(r *jstestprovider.Receipt) {
			r.TestRepositoryAtStart.DirtyState, r.TestRepositoryAtPublish.DirtyState = "dirty", "dirty"
		}), "not clean during the run"},
		{"no publish identity", mutate(func(r *jstestprovider.Receipt) { r.TestRepositoryAtPublish = nil }), "both run start and publish"},
		{"identity moved", mutate(func(r *jstestprovider.Receipt) { r.TestRepositoryAtPublish.Revision = evidenceRootCommit }), "changed between run start and publish"},
		{"projection without receipt", map[string]any{"schema": testvaliditydoc.Schema, "source": "corvint-go-test-provider", "tests": []any{}}, "carries no Playwright receipt or RSpec provider document"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			report, err := Compile(context.Background(), index, Options{Base: base, Evidence: []string{writeEvidence(t, testCase.document)}})
			if err != nil {
				t.Fatal(err)
			}
			source := report.Sources[1]
			if source.Status != "UNBOUND" || !strings.Contains(source.Detail, testCase.reason) || source.Basis != 0 {
				t.Fatalf("source = %+v, want UNBOUND naming %q", source, testCase.reason)
			}
			for _, item := range report.Obligations {
				for _, candidate := range item.Witnesses {
					if candidate.Source == "evidence" {
						t.Fatalf("an unbound document attached %+v", candidate)
					}
				}
			}
		})
	}
}

// TestEvidenceRefusesAnUnverifiableDocument pins the AGW-V0-008 failure mode:
// a document that cannot be read or decoded refuses the report and names the
// file rather than reading as "no evidence".
func TestEvidenceRefusesAnUnverifiableDocument(t *testing.T) {
	index, base := packetFixture(t, "example.test/fixture")
	directory := t.TempDir()
	write := func(name string, data []byte) string {
		file := filepath.Join(directory, name)
		if err := os.WriteFile(file, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return file
	}
	cases := map[string]string{
		"missing":       filepath.Join(directory, "absent.json"),
		"not json":      write("text.json", []byte("not json")),
		"wrong schema":  write("schema.json", []byte(`{"schema":"other/0"}`)),
		"unknown kind":  write("kind.json", []byte(`{"something":1}`)),
		"over the size": write("large.json", bytes.Repeat([]byte(" "), testvaliditydoc.MaxInputBytes+1)),
		"relative":      "evidence.json",
	}
	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Compile(context.Background(), index, Options{Base: base, Evidence: []string{file}})
			if err == nil || !strings.Contains(err.Error(), "--evidence "+file) {
				t.Fatalf("err = %v, want a refusal naming %s", err, file)
			}
		})
	}
	tooMany := make([]string, MaxEvidenceFiles+1)
	if _, err := Compile(context.Background(), index, Options{Base: base, Evidence: tooMany}); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("err = %v, want the evidence bound", err)
	}
}

// TestAttachEvidenceKeepsNotRunSticky pins that evidence never upgrades an
// undetermined obligation: it is counted as open with a candidate and stays
// NOT_RUN.
func TestAttachEvidenceKeepsNotRunSticky(t *testing.T) {
	obligations := []Obligation{
		{ID: "change:x", Kind: KindChange, Path: "x/x.py", Verdict: NotRun, Reason: "LANGUAGE_COVERAGE_NONE", Witnesses: []Witness{}},
		{ID: "change:y", Kind: KindChange, Path: "y/y.go", Verdict: Unproven, Witnesses: []Witness{}},
	}
	receipt := &jstestprovider.Receipt{Tests: []jstestprovider.TestOutcome{
		{FullName: "x", State: jstestprovider.StatePassed, ImportedFiles: map[string]string{"x/x.py": "", "x/./x.py": "", "../x/x.py": ""}},
	}}
	attached, unattached := attachEvidence(obligations, receiptTests(receipt, evidencePaths{}))
	decide(obligations)
	if attached != 1 || unattached != 0 || obligations[0].Verdict != NotRun || obligations[0].Reason != "LANGUAGE_COVERAGE_NONE" {
		t.Fatalf("attached=%d unattached=%d obligation=%+v", attached, unattached, obligations[0])
	}
	summary := summarize(obligations)
	if summary.NotRun != 1 || summary.CandidateWitnessed != 1 || summary.Unwitnessed != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}

// reporterReceipt is shaped as the qualified reporter writes a receipt: test
// files and imported files are absolute paths under the checkout the run
// used, and the anchor is relative to the Playwright test directory "b".
func reporterReceipt(index *contextindex.Index, imported string) jstestprovider.Receipt {
	receipt := boundReceipt(index)
	root := filepath.ToSlash(index.Root)
	receipt.Identity.TestFileDigests = map[string]string{root + "/b/b.go": headDigest(index, "b/b.go")}
	receipt.Tests = []jstestprovider.TestOutcome{{
		Name: "adds", FullName: "b adds", State: jstestprovider.StatePassed,
		Anchor:        &jstestprovider.Anchor{File: "b.go", Line: 3},
		ImportedFiles: map[string]string{root + "/a/a.go": imported, "/elsewhere/node_modules/x/index.js": strings.Repeat("0", 64)},
	}}
	return receipt
}

// TestEvidenceResolvesReporterPaths pins AGW-V0-009 (2): absolute receipt
// paths resolve under the inferred checkout root and a relative anchor under
// the inferred test directory, so a real reporter receipt attaches.
func TestEvidenceResolvesReporterPaths(t *testing.T) {
	index, base := packetFixture(t, "example.test/fixture")
	file := writeEvidence(t, providerDocument(reporterReceipt(index, headDigest(index, "a/a.go"))))
	report, err := Compile(context.Background(), index, Options{Base: base, Evidence: []string{file}})
	if err != nil {
		t.Fatal(err)
	}
	evidence := report.Sources[1]
	if evidence.Status != "BOUND" || evidence.Basis < 1 || evidence.Unattached != 0 {
		t.Fatalf("evidence source = %+v", evidence)
	}
	found := false
	for _, item := range report.Obligations {
		for _, candidate := range item.Witnesses {
			if candidate.Source != "evidence" {
				continue
			}
			if candidate.EvidencePath != "b/b.go" || candidate.Closing || candidate.Test != "b adds" {
				t.Fatalf("evidence witness on %s = %+v", item.Path, candidate)
			}
			found = found || item.Path == "a/a.go"
		}
	}
	if !found {
		t.Fatalf("no evidence witness on a/a.go: %+v", report.Obligations)
	}
	paths := resolveEvidencePaths(index, func() *jstestprovider.Receipt { r := reporterReceipt(index, ""); return &r }())
	for written, want := range map[string]string{
		filepath.ToSlash(index.Root) + "/a/a.go": "a/a.go",
		"b.go":                                   "b/b.go",
		"/elsewhere/a/a.go":                      "",
		"../a/a.go":                              "",
		"not/tracked.go":                         "not/tracked.go",
	} {
		if got := paths.resolve(written); got != want {
			t.Errorf("resolve(%q) = %q, want %q", written, got, want)
		}
	}
}

// TestEvidenceDigestMismatchIsUnbound pins AGW-V0-009 (3): a resolved file
// whose recorded digest is malformed or differs from the head blob makes the
// whole receipt UNBOUND, so it attaches nothing.
func TestEvidenceDigestMismatchIsUnbound(t *testing.T) {
	index, base := packetFixture(t, "example.test/fixture")
	other := sha256.Sum256([]byte("not the head bytes"))
	for name, testCase := range map[string]struct{ digest, reason string }{
		"different": {hex.EncodeToString(other[:]), "digest for a/a.go differs from the head blob"},
		"malformed": {"sha256:00", "for head file a/a.go, which is not a lowercase hex sha256"},
		"uppercase": {strings.ToUpper(headDigest(index, "a/a.go")), "not a lowercase hex sha256"},
	} {
		t.Run(name, func(t *testing.T) {
			file := writeEvidence(t, providerDocument(reporterReceipt(index, testCase.digest)))
			report, err := Compile(context.Background(), index, Options{Base: base, Evidence: []string{file}})
			if err != nil {
				t.Fatal(err)
			}
			source := report.Sources[1]
			if source.Status != "UNBOUND" || !strings.Contains(source.Detail, testCase.reason) || source.Basis != 0 || source.Hunks != 0 {
				t.Fatalf("source = %+v, want UNBOUND naming %q", source, testCase.reason)
			}
		})
	}
}

func mustRead(t *testing.T, file string) []byte {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestRepositoryEvidenceAdmitsOnlyPathsInsideTheCheckout pins AGW-V0-011: a
// repository-relative evidence path binds when it names a file inside the
// checkout, directly or through a link that stays inside, and every path that
// is absolute, escapes with "..", is not clean, or resolves through a link
// outside the checkout refuses the report with its reason named.
func TestRepositoryEvidenceAdmitsOnlyPathsInsideTheCheckout(t *testing.T) {
	index, base := packetFixture(t, "example.test/fixture")
	root := index.Root
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(".corvint/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(providerDocument(boundReceipt(index)))
	if err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(root, ".corvint", "evidence", "run.json")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inside, data, 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "run.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	links := true
	for link, target := range map[string]string{
		".corvint/inside.json":  filepath.Join("evidence", "run.json"),
		".corvint/escape.json":  filepath.Join(outside, "run.json"),
		".corvint/outdir":       outside,
		".corvint/up.json":      filepath.Join("..", "..", filepath.Base(outside), "run.json"),
		".corvint/evidence/rel": filepath.Join("..", "..", ".."),
	} {
		if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(link))); err != nil {
			links = false
		}
	}

	compile := func(name string) (*Report, error) {
		return Compile(context.Background(), index, Options{Base: base, RepositoryEvidence: []string{name}})
	}
	admitted := []string{".corvint/evidence/run.json"}
	if links {
		admitted = append(admitted, ".corvint/inside.json")
	}
	for _, name := range admitted {
		report, err := compile(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if source := report.Sources[1]; source.Status != "BOUND" || source.Path != name || source.Basis == 0 {
			t.Fatalf("%s: source = %+v, want BOUND", name, source)
		}
	}
	refusals := map[string]string{
		"":                                 "the path is empty",
		filepath.Join(outside, "run.json"): "the path is absolute",
		"/etc/passwd":                      "the path is absolute",
		"../run.json":                      "leaves the checkout through ..",
		".corvint/../../run.json":          "leaves the checkout through ..",
		"./.corvint/evidence/run.json":     "not a clean relative file path",
		".corvint//evidence/run.json":      "not a clean relative file path",
		".corvint/missing.json":            "--evidence-in-repo .corvint/missing.json",
	}
	if links {
		refusals[".corvint/escape.json"] = "outside the checkout"
		refusals[".corvint/up.json"] = "outside the checkout"
		refusals[".corvint/outdir/run.json"] = "outside the checkout"
		refusals[".corvint/evidence/rel/"+filepath.Base(outside)+"/run.json"] = "outside the checkout"
	}
	for name, reason := range refusals {
		if _, err := compile(name); err == nil || !strings.Contains(err.Error(), "--evidence-in-repo "+name) || !strings.Contains(err.Error(), reason) {
			t.Fatalf("%q: err = %v, want a refusal naming %q", name, err, reason)
		}
	}
	tooMany := Options{Base: base, Evidence: make([]string, MaxEvidenceFiles/2), RepositoryEvidence: make([]string, MaxEvidenceFiles/2+1)}
	if _, err := Compile(context.Background(), index, tooMany); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("err = %v, want the combined evidence bound", err)
	}
}
