package appflows

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

const (
	coveragePayJS  = "function pay(amount) {\n  if (amount > 0) {\n    return 'paid'\n  }\n  return 'refused'\n}\n\nmodule.exports = { pay }\n"
	coverageUnused = "function unused() {\n  return 1\n}\nmodule.exports = { unused }\n"
	coverageOrder  = "class Order\n  def total\n    42\n  end\n\n  def cancel\n    :cancelled\n  end\nend\n"
	coverageWidget = "export const widget: number = 1\n"
	coverageKey    = "tests/pay.spec.ts > chromium > pays"
)

type coverageFixture struct {
	root, commit, tree, dir string
}

// coverageRepo commits plain JavaScript, TypeScript and Ruby sources and returns their revision.
func coverageRepo(t *testing.T) coverageFixture {
	t.Helper()
	root := t.TempDir()
	writeRaw(t, root, "src/pay.js", []byte(coveragePayJS))
	writeRaw(t, root, "src/unused.js", []byte(coverageUnused))
	writeRaw(t, root, "src/widget.ts", []byte(coverageWidget))
	writeRaw(t, root, "app/models/order.rb", []byte(coverageOrder))
	gitTest(t, root, "init", "-q")
	commit := commitAll(t, root, "fixture")
	return coverageFixture{root: root, commit: commit, tree: gitOut(t, root, "rev-parse", commit+"^{tree}"), dir: t.TempDir()}
}

func (fx coverageFixture) run(key string, outcomes ...string) TestRunEvidence {
	r := stabilityRecord("run-1", key, "done", outcomes...)
	r.Source = RunSource{Commit: fx.commit, Tree: fx.tree, Clean: true}
	return r
}

// write puts an input file outside the repository so the ingest cannot dirty it.
func (fx coverageFixture) write(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(fx.dir, name)
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func (fx coverageFixture) evidence(t *testing.T, records ...TestRunEvidence) string {
	t.Helper()
	var b bytes.Buffer
	for _, r := range records {
		line, err := EncodeRunEvidence(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
	}
	return fx.write(t, fmt.Sprintf("evidence-%d.jsonl", len(records)), b.Bytes())
}

func ingestJSON(t *testing.T, in CoverageIngestInput) CoverageReport {
	t.Helper()
	raw, err := IngestCoverage(context.Background(), in)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	var report CoverageReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	return report
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// payV8 is one Playwright V8 entry for src/pay.js: the whole script ran once, the refusal did not.
func payV8(fx coverageFixture, source *string) map[string]any {
	zero := strings.Index(coveragePayJS, "return 'refused'")
	entry := map[string]any{"url": "file://" + filepath.ToSlash(filepath.Join(fx.root, "src/pay.js")), "functions": []any{
		map[string]any{"functionName": "", "isBlockCoverage": true, "ranges": []any{map[string]any{"startOffset": 0, "endOffset": len(coveragePayJS), "count": 1}}},
		map[string]any{"functionName": "pay", "isBlockCoverage": true, "ranges": []any{
			map[string]any{"startOffset": 0, "endOffset": strings.Index(coveragePayJS, "\n\n"), "count": 1},
			map[string]any{"startOffset": zero, "endOffset": zero + len("return 'refused'"), "count": 0}}},
	}}
	if source != nil {
		entry["source"] = *source
	}
	return entry
}

func relationFor(report CoverageReport, path, test string) *CoverageRelation {
	for i := range report.Relations {
		if report.Relations[i].Path == path && report.Relations[i].TestKey == test {
			return &report.Relations[i]
		}
	}
	return nil
}

func unknownReasons(report CoverageReport) []string {
	got := []string{}
	for _, u := range report.Unknowns {
		got = append(got, u.TestKey+"|"+u.Path+"|"+u.Reason)
	}
	return got
}

// checkCoverageAnchors recomputes every anchor's blob, file digest and span digest from Git.
func checkCoverageAnchors(t *testing.T, fx coverageFixture, e doccorpus.Evidence) {
	t.Helper()
	if e.Derivation != "observed" || e.Trust != "generated" || e.Freshness != "fresh" || len(e.Anchors) == 0 {
		t.Fatalf("evidence axes = %+v", e)
	}
	for _, a := range e.Anchors {
		data := gitOut(t, fx.root, "show", fx.commit+":"+a.Path) + "\n"
		lines := strings.SplitAfter(data, "\n")
		span := strings.Join(lines[a.Start-1:a.End], "")
		if a.Kind != "observed" || a.Authority != "external-provider" || a.Revision != fx.commit || a.Blob != gitOut(t, fx.root, "rev-parse", fx.commit+":"+a.Path) ||
			a.SHA256 != doccorpus.Digest([]byte(data)) || a.SpanSHA256 != doccorpus.Digest([]byte(span)) {
			t.Fatalf("anchor %+v does not recompute", a)
		}
	}
}

// AFU-V1-112
func TestAFUV1112CoverageIngestDeterministicPinned(t *testing.T) {
	fx := coverageRepo(t)
	source := coveragePayJS
	from := fx.write(t, "v8.json", mustJSON(t, []any{payV8(fx, &source)}))
	evidence := fx.evidence(t, fx.run(coverageKey, "passed"))
	in := CoverageIngestInput{Root: fx.root, Format: "v8", From: from, Evidence: []string{evidence}, Revision: fx.commit}
	status := gitOut(t, fx.root, "status", "--porcelain", "--ignored")
	first, err := IngestCoverage(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := IngestCoverage(context.Background(), in)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("ingest is not deterministic: %v", err)
	}
	if got := gitOut(t, fx.root, "status", "--porcelain", "--ignored"); got != status {
		t.Fatalf("ingest changed the repository: %q", got)
	}
	report := ingestJSON(t, in)
	fromRaw, _ := os.ReadFile(from)
	evidenceRaw, _ := os.ReadFile(evidence)
	want := []CoverageInput{{Role: "coverage", SHA256: Digest(fromRaw), Bytes: len(fromRaw)}, {Role: "run-evidence", SHA256: Digest(evidenceRaw), Bytes: len(evidenceRaw)}}
	if report.Schema != CoverageIngestSchema || report.Trust != "generated" || report.Revision != fx.commit || report.Tree != fx.tree || !slices.Equal(report.Inputs, want) {
		t.Fatalf("report header = %+v", report)
	}
	if len(report.Runs) != 1 || report.Runs[0].State != "joined" || report.Runs[0].TestKey != coverageKey || report.Runs[0].RecordSHA256 != Digest(evidenceRaw) || report.Runs[0].RunID != "run-1" {
		t.Fatalf("run binding = %+v", report.Runs)
	}
	if strings.Contains(string(first), fx.dir) {
		t.Fatal("report records a local input file name")
	}
	r := relationFor(report, "src/pay.js", coverageKey)
	if r == nil {
		t.Fatalf("no relation: %+v", report.Relations)
	}
	if r.Type != "tested_by" || r.From != "source:src/pay.js" || r.To != "test:"+coverageKey || r.Evidence.State != "supported" {
		t.Fatalf("relation = %+v", r)
	}
	checkCoverageAnchors(t, fx, r.Evidence)
}

// AFU-V1-113
func TestAFUV1113CoverageIngestV8(t *testing.T) {
	fx := coverageRepo(t)
	source, other := coveragePayJS, "transpiled()\n"
	entries := []any{
		payV8(fx, &source),
		map[string]any{"url": "https://app.example.invalid/static/bundle.js", "functions": []any{}},
		map[string]any{"url": "node:internal/main", "functions": []any{}},
		map[string]any{"url": "src/widget.ts", "functions": []any{map[string]any{"ranges": []any{map[string]any{"startOffset": 0, "endOffset": 5, "count": 1}}}}},
		map[string]any{"url": "src/unused.js", "source": other, "functions": []any{}},
	}
	from := fx.write(t, "v8.json", mustJSON(t, map[string]any{"result": entries}))
	report := ingestJSON(t, CoverageIngestInput{Root: fx.root, Format: "v8", From: from, Evidence: []string{fx.evidence(t, fx.run(coverageKey, "passed"))}, Revision: fx.commit})
	r := relationFor(report, "src/pay.js", coverageKey)
	if r == nil || !slices.Equal(r.ExecutedSpans, [][2]int{{1, 4}, {6, 6}, {8, 8}}) || r.Instrumented != 7 || r.ExecutedLines != 6 {
		t.Fatalf("pay.js relation = %+v", r)
	}
	checkCoverageAnchors(t, fx, r.Evidence)
	want := []string{
		coverageKey + "|https://app.example.invalid/static/bundle.js|" + coverageReasonSourceMap,
		coverageKey + "|node:internal/main|" + coverageReasonOutside,
		coverageKey + "|src/unused.js|" + coverageReasonSourceMismatch,
		coverageKey + "|src/widget.ts|" + coverageReasonTranspiled,
	}
	if got := unknownReasons(report); !slices.Equal(got, want) {
		t.Fatalf("unknowns = %q", got)
	}
	// Without a source, a plain JavaScript file's offsets apply to its committed blob.
	noSource := fx.write(t, "v8-plain.json", mustJSON(t, []any{payV8(fx, nil)}))
	if r := relationFor(ingestJSON(t, CoverageIngestInput{Root: fx.root, Format: "v8", From: noSource, Evidence: []string{fx.evidence(t, fx.run(coverageKey, "passed"))}, Revision: fx.commit}), "src/pay.js", coverageKey); r == nil {
		t.Fatal("plain JavaScript without source did not join")
	}
}

// AFU-V1-114
func TestAFUV1114CoverageIngestIstanbulAndLCOV(t *testing.T) {
	fx := coverageRepo(t)
	istanbul := map[string]any{filepath.Join(fx.root, "src/pay.js"): map[string]any{
		"path":         filepath.Join(fx.root, "src/pay.js"),
		"statementMap": map[string]any{"0": map[string]any{"start": map[string]any{"line": 2, "column": 2}, "end": map[string]any{"line": 4, "column": 3}}, "1": map[string]any{"start": map[string]any{"line": 3, "column": 4}, "end": map[string]any{"line": 3, "column": 17}}, "2": map[string]any{"start": map[string]any{"line": 5, "column": 2}, "end": map[string]any{"line": 5, "column": 18}}},
		"s":            map[string]any{"0": 1, "1": 1, "2": 0},
		"fnMap":        map[string]any{}, "f": map[string]any{}, "branchMap": map[string]any{}, "b": map[string]any{},
	}}
	evidence := fx.evidence(t, fx.run(coverageKey, "passed"))
	report := ingestJSON(t, CoverageIngestInput{Root: fx.root, Format: "istanbul", From: fx.write(t, "istanbul.json", mustJSON(t, istanbul)), Evidence: []string{evidence}, Revision: fx.commit, TestKey: coverageKey})
	if r := relationFor(report, "src/pay.js", coverageKey); r == nil || !slices.Equal(r.ExecutedSpans, [][2]int{{2, 3}}) || r.Instrumented != 3 {
		t.Fatalf("istanbul relation = %+v", report.Relations)
	} else {
		checkCoverageAnchors(t, fx, r.Evidence)
	}
	lcov := "TN:pays\nSF:/build/checkout/src/unused.js\nFN:1,unused\nDA:1,1\nDA:2,3\nDA:4,0\nend_of_record\n"
	report = ingestJSON(t, CoverageIngestInput{Root: fx.root, Format: "lcov", From: fx.write(t, "trace.lcov", []byte(lcov)), Evidence: []string{evidence}, Revision: fx.commit, StripPrefix: "/build/checkout/"})
	if r := relationFor(report, "src/unused.js", coverageKey); r == nil || !slices.Equal(r.ExecutedSpans, [][2]int{{1, 2}}) || r.Instrumented != 3 {
		t.Fatalf("lcov relation = %+v", report.Relations)
	} else {
		checkCoverageAnchors(t, fx, r.Evidence)
	}
}

// AFU-V1-115
func TestAFUV1115CoverageIngestRSpec(t *testing.T) {
	fx := coverageRepo(t)
	lines := []any{1, 1, 1, nil, nil, 1, 0, nil, nil}
	examples := map[string]any{
		"./spec/order_spec.rb[1:1]": map[string]any{filepath.Join(fx.root, "app/models/order.rb"): lines},
		"spec/order_spec.rb[1:2]":   map[string]any{"app/models/order.rb": map[string]any{"lines": []any{1, 1, 0, nil, nil, 1, 2, nil, nil}}},
		"spec/order_spec.rb[1:3]":   map[string]any{"app/models/order.rb": lines},
	}
	evidence := fx.evidence(t, fx.run("spec/order_spec.rb[1:1]", "passed"), fx.run("spec/order_spec.rb[1:2]", "failed"))
	report := ingestJSON(t, CoverageIngestInput{Root: fx.root, Format: "rspec", From: fx.write(t, "rspec.json", mustJSON(t, examples)), Evidence: []string{evidence}, Revision: fx.commit})
	first, second := relationFor(report, "app/models/order.rb", "spec/order_spec.rb[1:1]"), relationFor(report, "app/models/order.rb", "spec/order_spec.rb[1:2]")
	if first == nil || second == nil || !slices.Equal(first.ExecutedSpans, [][2]int{{1, 3}, {6, 6}}) || !slices.Equal(second.ExecutedSpans, [][2]int{{1, 2}, {6, 7}}) {
		t.Fatalf("rspec relations = %+v", report.Relations)
	}
	checkCoverageAnchors(t, fx, first.Evidence)
	if got := unknownReasons(report); !slices.Equal(got, []string{"spec/order_spec.rb[1:3]||" + coverageReasonNoRun}) {
		t.Fatalf("unknowns = %q", got)
	}
}

// AFU-V1-116
func TestAFUV1116CoverageIngestContradictedLinks(t *testing.T) {
	fx := coverageRepo(t)
	lcov := "SF:src/pay.js\nDA:1,1\nDA:5,0\nend_of_record\nSF:src/unused.js\nDA:1,0\nDA:2,0\nDA:4,0\nend_of_record\n"
	links := CoverageLinks{Schema: CoverageLinksSchema, Links: []CoverageLink{
		{Subject: "src/pay.js", Test: coverageKey, Source: "declared", Reference: "docs/pay.md"},
		{Subject: "src/unused.js", Test: coverageKey, Source: "supplied"},
		{Subject: "src/widget.ts", Test: coverageKey, Source: "declared"},
		{Subject: "src/pay.js", Test: "tests/other.spec.ts > chromium > other", Source: "declared"},
	}}
	report := ingestJSON(t, CoverageIngestInput{Root: fx.root, Format: "lcov", From: fx.write(t, "trace.lcov", []byte(lcov)), Evidence: []string{fx.evidence(t, fx.run(coverageKey, "passed"))},
		Revision: fx.commit, Links: fx.write(t, "links.json", mustJSON(t, links))})
	got := []string{}
	for _, l := range report.Links {
		got = append(got, l.Subject+"|"+l.TestKey+"|"+l.State+"|"+l.Reason)
	}
	want := []string{
		"src/pay.js|tests/other.spec.ts > chromium > other|unknown|" + coverageReasonNotCovered,
		"src/pay.js|" + coverageKey + "|corroborated|",
		"src/unused.js|" + coverageKey + "|conflicted|" + coverageReasonContradiction,
		"src/widget.ts|" + coverageKey + "|unknown|" + coverageReasonNotInstrumented,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("links = %q", got)
	}
	if len(report.Conflicts) != 1 {
		t.Fatalf("conflicts = %+v", report.Conflicts)
	}
	c := report.Conflicts[0]
	if c.Type != "conflicts_with" || c.Subject != "source:src/unused.js" || c.LinkSource != "supplied" || c.Reason != coverageReasonContradiction || c.Instrumented != 3 || c.Evidence.State != "conflicted" {
		t.Fatalf("conflict = %+v", c)
	}
	checkCoverageAnchors(t, fx, c.Evidence)
	if relationFor(report, "src/unused.js", coverageKey) != nil {
		t.Fatal("an unexecuted file produced a tested_by relation")
	}
}

// AFU-V1-117
func TestAFUV1117CoverageIngestStaleNotJoined(t *testing.T) {
	fx := coverageRepo(t)
	lcov := fx.write(t, "trace.lcov", []byte("SF:src/pay.js\nDA:1,1\nend_of_record\n"))
	links := fx.write(t, "links.json", mustJSON(t, CoverageLinks{Schema: CoverageLinksSchema, Links: []CoverageLink{{Subject: "src/pay.js", Test: coverageKey, Source: "declared"}}}))
	other := fx.run(coverageKey, "passed")
	other.Source.Commit = strings.Repeat("a", 40)
	dirty := fx.run(coverageKey, "passed")
	dirty.Source.Clean = false
	for _, tc := range []struct {
		record        TestRunEvidence
		state, reason string
	}{
		{other, "stale", coverageReasonStale},
		{dirty, "stale", coverageReasonDirty},
		{fx.run(coverageKey, "skipped"), "unjoined", coverageReasonNotRun},
	} {
		report := ingestJSON(t, CoverageIngestInput{Root: fx.root, Format: "lcov", From: lcov, Evidence: []string{fx.evidence(t, tc.record)}, Revision: fx.commit, Links: links})
		if len(report.Relations) != 0 || len(report.Conflicts) != 0 || report.Runs[0].State != tc.state || report.Runs[0].Reason != tc.reason ||
			report.Links[0].State != "unknown" || report.Links[0].Reason != tc.reason {
			t.Fatalf("%s: report = %+v", tc.reason, report)
		}
	}
	// A line past the committed blob, or an instrumented blank line, means the coverage is for other content.
	for name, trace := range map[string]string{coverageReasonOutOfRange: "SF:src/pay.js\nDA:99,1\nend_of_record\n", coverageReasonBlankLine: "SF:src/pay.js\nDA:7,1\nend_of_record\n"} {
		report := ingestJSON(t, CoverageIngestInput{Root: fx.root, Format: "lcov", From: fx.write(t, name+".lcov", []byte(trace)), Evidence: []string{fx.evidence(t, fx.run(coverageKey, "passed"))}, Revision: fx.commit})
		if len(report.Relations) != 0 || !slices.Equal(unknownReasons(report), []string{coverageKey + "|src/pay.js|" + name}) {
			t.Fatalf("%s: report = %+v", name, report)
		}
	}
}

// AFU-V1-118
func TestAFUV1118CoverageIngestRefusals(t *testing.T) {
	fx := coverageRepo(t)
	lcov := fx.write(t, "trace.lcov", []byte("SF:src/pay.js\nDA:1,1\nend_of_record\n"))
	one := fx.evidence(t, fx.run(coverageKey, "passed"))
	two := fx.evidence(t, fx.run(coverageKey, "passed"), fx.run("other", "passed"))
	many := map[string]any{}
	for i := range maxCoverageTests + 1 {
		many[fmt.Sprintf("spec/a_spec.rb[1:%d]", i)] = map[string]any{"app/models/order.rb": []any{1}}
	}
	for name, tc := range map[string]struct {
		in   CoverageIngestInput
		code string
	}{
		"format":         {CoverageIngestInput{Format: "clover", From: lcov, Evidence: []string{one}, Revision: fx.commit}, "coverage-refused"},
		"short revision": {CoverageIngestInput{Format: "lcov", From: lcov, Evidence: []string{one}, Revision: "HEAD"}, "coverage-refused"},
		"unknown commit": {CoverageIngestInput{Format: "lcov", From: lcov, Evidence: []string{one}, Revision: strings.Repeat("b", 40)}, "coverage-invalid-revision"},
		"ambiguous test": {CoverageIngestInput{Format: "lcov", From: lcov, Evidence: []string{two}, Revision: fx.commit}, "coverage-refused"},
		"malformed lcov": {CoverageIngestInput{Format: "lcov", From: fx.write(t, "bad.lcov", []byte("DA:1,1\n")), Evidence: []string{one}, Revision: fx.commit}, "coverage-refused"},
		"links schema":   {CoverageIngestInput{Format: "lcov", From: lcov, Evidence: []string{one}, Revision: fx.commit, Links: fx.write(t, "links.json", []byte(`{"schema":"x","links":[]}`))}, "coverage-refused"},
		"test bound":     {CoverageIngestInput{Format: "rspec", From: fx.write(t, "many.json", mustJSON(t, many)), Evidence: []string{one}, Revision: fx.commit}, "coverage-bound-exceeded"},
	} {
		tc.in.Root = fx.root
		_, err := IngestCoverage(context.Background(), tc.in)
		var coded *gokernel.Error
		if !errors.As(err, &coded) || coded.Code != tc.code {
			t.Fatalf("%s: err = %v, want %s", name, err, tc.code)
		}
	}
}
