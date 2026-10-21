package testsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/rubytestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

const (
	calcBefore = `RSpec.describe Calc do
  it 'adds' do
    expect(1 + 1).to eq(2)
  end
end
`
	calcAfter = `RSpec.describe Calc do
  let(:base) { 0 }

  it 'adds' do
    expect(1 + 1).to eq(2)
  end
end
`
)

func repoAt(commit, dirty string) *jstestprovider.ApplicationRepositoryIdentity {
	return &jstestprovider.ApplicationRepositoryIdentity{Revision: commit, DirtyState: dirty}
}

func blobAt(t *testing.T, root, commit, name string) string {
	t.Helper()
	return gitTest(t, root, "rev-parse", commit+":"+name)
}

func calcDocument(t *testing.T, root, commit string, line int) *rubytestprovider.Document {
	t.Helper()
	return &rubytestprovider.Document{
		RepositoryBefore: repoAt(commit, "clean"), RepositoryAfter: repoAt(commit, "clean"),
		Examples: []rubytestprovider.Example{{
			ID: "./spec/calc_spec.rb[1:1]", RSpecID: "./spec/calc_spec.rb[1:1]", File: "spec/calc_spec.rb", Line: line,
			Description: "Calc adds", Status: "passed", SpecBlob: blobAt(t, root, commit, "spec/calc_spec.rb"),
		}},
	}
}

func joinOne(t *testing.T, root string, options ExampleJoinOptions) ExampleJoinReport {
	t.Helper()
	report, err := ExampleJoin(OpenRepo(context.Background(), root), options)
	if err != nil {
		t.Fatalf("ExampleJoin: %v", err)
	}
	return report
}

// TestExampleJoinRubyShiftedLines pins TSA-V0-410..412 and TSA-V0-415: the same example after a
// line shift keeps one line-free identity and carries outcome, verdict and blob-bound source; an
// earlier receipt is refused against the later revision (TSA-V0-413) and a dirty receipt whose
// bound blob differs is refused (TSA-V0-414).
func TestExampleJoinRubyShiftedLines(t *testing.T) {
	root := t.TempDir()
	first := commitFiles(t, root, map[string]string{"spec/calc_spec.rb": calcBefore})
	second := commitFiles(t, root, map[string]string{"spec/calc_spec.rb": calcAfter})

	a := joinOne(t, root, ExampleJoinOptions{Revision: first, Ruby: calcDocument(t, root, first, 2)})
	b := joinOne(t, root, ExampleJoinOptions{Revision: second, Ruby: calcDocument(t, root, second, 4)})
	if a.Schema != ExampleJoinSchema || a.Language != "ruby" || len(a.Examples) != 1 || len(b.Examples) != 1 {
		t.Fatalf("reports = %+v / %+v", a, b)
	}
	ra, rb := a.Examples[0], b.Examples[0]
	if ra.Example != "spec/calc_spec.rb[1:1]" || rb.Example != ra.Example || ra.IdentityKind != ExampleIdentityRSpec {
		t.Fatalf("identities %q / %q, want one line-free identity", ra.Example, rb.Example)
	}
	if ra.Line != 2 || rb.Line != 4 || ra.Outcome != "passed" {
		t.Fatalf("lines/outcome = %d %d %q", ra.Line, rb.Line, ra.Outcome)
	}
	if ra.Verdict.Hygiene.State == testvalidity.HygieneAbstained || ra.Verdict.Hygiene.State != rb.Verdict.Hygiene.State {
		t.Fatalf("verdicts = %+v / %+v, want one located verdict", ra.Verdict, rb.Verdict)
	}
	if ra.Requirements != nil {
		t.Fatalf("requirements without a selection: %+v", ra.Requirements)
	}
	if rb.Source.Revision != second || len(rb.Source.Files) != 1 || rb.Source.Files[0].Blob != blobAt(t, root, second, "spec/calc_spec.rb") {
		t.Fatalf("source = %+v", rb.Source)
	}

	repo := OpenRepo(context.Background(), root)
	if _, err := ExampleJoin(repo, ExampleJoinOptions{Revision: second, Ruby: calcDocument(t, root, first, 2)}); !hasCode(err, "tests-join-revision-mismatch") {
		t.Fatalf("earlier receipt at later rev: %v", err)
	}
	dirty := calcDocument(t, root, second, 4)
	dirty.RepositoryAfter.DirtyState = "dirty"
	dirty.Examples[0].SpecBlob = blobAt(t, root, first, "spec/calc_spec.rb")
	if _, err := ExampleJoin(repo, ExampleJoinOptions{Ruby: dirty}); !hasCode(err, "tests-join-revision-mismatch") {
		t.Fatalf("dirty receipt with another blob: %v", err)
	}
	unbound := calcDocument(t, root, second, 4)
	unbound.Examples[0].SpecBlob = ""
	if r := joinOne(t, root, ExampleJoinOptions{Ruby: unbound}); r.Examples[0].Verdict.Hygiene.Reason != joinReasonSourceUnbound {
		t.Fatalf("unbound spec blob: %+v", r.Examples[0].Verdict)
	}
}

func widgetDocument(t *testing.T, root, commit, dirty string, bindSpec bool) *rubytestprovider.Document {
	t.Helper()
	shared := blobAt(t, root, commit, widgetShared)
	examples := []rubytestprovider.Example{
		{RSpecID: "./spec/widget_spec.rb[1:1:1:1]", File: widgetShared, Line: 2, Status: "passed", SpecBlob: shared,
			Description:   "Widget stub reader behaves like a widget reads the data",
			SharedExample: &rubytestprovider.SharedExample{Origin: widgetShared + ":1", IncludedAt: []string{"spec/widget_spec.rb:5"}}},
		{RSpecID: "./spec/widget_spec.rb[1:2:1]", File: widgetShared, Line: 2, Status: "failed", SpecBlob: shared,
			Description:   "Widget yielding reader reads the data",
			SharedExample: &rubytestprovider.SharedExample{Origin: widgetShared + ":1", IncludedAt: []string{"spec/widget_spec.rb:11"}}},
	}
	if bindSpec {
		examples = append(examples, rubytestprovider.Example{RSpecID: "./spec/widget_spec.rb[1:3]", File: "spec/widget_spec.rb", Line: 99,
			Status: "passed", SpecBlob: blobAt(t, root, commit, "spec/widget_spec.rb")})
	}
	helpers := []rubytestprovider.BoundFile{
		{Path: "spec/rails_helper.rb", Blob: blobAt(t, root, commit, "spec/rails_helper.rb")},
		{Path: widgetShared, Blob: shared},
	}
	return &rubytestprovider.Document{RepositoryBefore: repoAt(commit, dirty), RepositoryAfter: repoAt(commit, dirty), Helpers: helpers, Examples: examples}
}

// TestExampleJoinSharedGroup pins TSA-V0-411, TSA-V0-414 and TSA-V0-415: an example of a shared
// group is keyed by its including spec, read through its own inclusion and cites both files; a
// dirty receipt that did not bind the including spec abstains.
func TestExampleJoinSharedGroup(t *testing.T) {
	root := t.TempDir()
	commit := commitFiles(t, root, map[string]string{
		"spec/widget_spec.rb": widgetSpec, widgetShared: widgetExamples, "spec/rails_helper.rb": "require 'spec_helper'\n",
	})
	report := joinOne(t, root, ExampleJoinOptions{Ruby: widgetDocument(t, root, commit, "clean", false)})
	if len(report.Examples) != 2 {
		t.Fatalf("examples = %+v", report.Examples)
	}
	stub, yielding := report.Examples[0], report.Examples[1]
	if stub.Example != "spec/widget_spec.rb[1:1:1:1]" || yielding.Example != "spec/widget_spec.rb[1:2:1]" {
		t.Fatalf("identities = %q %q", stub.Example, yielding.Example)
	}
	if stub.Verdict.Hygiene.State != testvalidity.HygieneIneligible || yielding.Verdict.Hygiene.State != testvalidity.HygieneEligible {
		t.Fatalf("verdicts = %+v / %+v, want INELIGIBLE then ELIGIBLE", stub.Verdict.Hygiene, yielding.Verdict.Hygiene)
	}
	if yielding.Outcome != "failed" || stub.File != widgetShared || stub.Line != 2 {
		t.Fatalf("record = %+v", yielding)
	}
	var paths []string
	for _, f := range stub.Source.Files {
		if f.Blob == "" {
			t.Fatalf("unbound source blob: %+v", stub.Source)
		}
		paths = append(paths, f.Path)
	}
	if !slices.Equal(paths, []string{"spec/widget_spec.rb", widgetShared, "spec/rails_helper.rb"}) {
		t.Fatalf("source files = %v", paths)
	}

	dirty := joinOne(t, root, ExampleJoinOptions{Ruby: widgetDocument(t, root, commit, "dirty", false)})
	if got := dirty.Examples[0].Verdict.Hygiene; got.State != testvalidity.HygieneAbstained || got.Reason != testvalidity.InclusionSourceUnbound {
		t.Fatalf("dirty without the including spec: %+v", got)
	}
	bound := joinOne(t, root, ExampleJoinOptions{Ruby: widgetDocument(t, root, commit, "dirty", true)})
	if got := bound.Examples[1].Verdict.Hygiene; got.State != testvalidity.HygieneEligible {
		t.Fatalf("dirty with the including spec bound: %+v", got)
	}
}

func reportDocument(t *testing.T, root, commit, helpersUnbound string) *rubytestprovider.Document {
	t.Helper()
	return &rubytestprovider.Document{
		RepositoryBefore: repoAt(commit, "clean"), RepositoryAfter: repoAt(commit, "clean"), HelpersUnbound: helpersUnbound,
		Examples: []rubytestprovider.Example{{
			ID: "./spec/report_spec.rb[1:1]", RSpecID: "./spec/report_spec.rb[1:1]", File: "spec/report_spec.rb", Line: 4,
			Description: "Report reads the header", Status: "passed", SpecBlob: blobAt(t, root, commit, "spec/report_spec.rb"),
		}},
	}
}

// TestExampleJoinSupportFiles pins TSA-V0-412, TSA-V0-414 and TSA-V0-416 for the support set: a
// verdict read through a support helper cites that helper's blob, a receipt that did not bind its
// helper set abstains on the helper even when clean, and support reads draw on the join's file and
// size bounds.
func TestExampleJoinSupportFiles(t *testing.T) {
	root := t.TempDir()
	commit := commitFiles(t, root, map[string]string{
		"spec/report_spec.rb": neverYieldSpec, "spec/support/report_io.rb": neverYieldContext, "spec/rails_helper.rb": "require 'spec_helper'\n",
	})
	record := joinOne(t, root, ExampleJoinOptions{Ruby: reportDocument(t, root, commit, "")}).Examples[0]
	if got := record.Verdict.Hygiene; got.State != testvalidity.HygieneIneligible || got.Reason != testvalidity.RuleRBBlockNeverYielded {
		t.Fatalf("hygiene = %+v, want INELIGIBLE %s", got, testvalidity.RuleRBBlockNeverYielded)
	}
	want := []ExampleJoinFile{
		{Path: "spec/report_spec.rb", Blob: blobAt(t, root, commit, "spec/report_spec.rb")},
		{Path: "spec/rails_helper.rb", Blob: blobAt(t, root, commit, "spec/rails_helper.rb")},
		{Path: "spec/support/report_io.rb", Blob: blobAt(t, root, commit, "spec/support/report_io.rb")},
	}
	if !slices.Equal(record.Source.Files, want) {
		t.Fatalf("source files = %+v, want %+v", record.Source.Files, want)
	}

	unbound := joinOne(t, root, ExampleJoinOptions{Ruby: reportDocument(t, root, commit, "helper-not-regular-file")}).Examples[0]
	if got := unbound.Verdict.Hygiene; got.State != testvalidity.HygieneAbstained {
		t.Fatalf("clean receipt with an unbound helper set: %+v, want ABSTAINED", got)
	}
	if len(unbound.Source.Files) != 1 {
		t.Fatalf("unbound helper set cites support files: %+v", unbound.Source.Files)
	}

	many := map[string]string{}
	for i := range MaxJoinFiles + 44 {
		many[fmt.Sprintf("spec/support/helper_%03d.rb", i)] = "module H; end\n"
	}
	crowded := commitFiles(t, root, many)
	repo := OpenRepo(context.Background(), root)
	if _, err := ExampleJoin(repo, ExampleJoinOptions{Ruby: reportDocument(t, root, crowded, "")}); !hasCode(err, "tests-bound-exceeded") {
		t.Fatalf("support files past the join file bound: %v", err)
	}
	if r := joinOne(t, root, ExampleJoinOptions{Ruby: reportDocument(t, root, crowded, "helper-not-regular-file")}); r.Examples[0].Verdict.Hygiene.State != testvalidity.HygieneAbstained {
		t.Fatalf("unbound helper set read the support files: %+v", r.Examples[0])
	}

	listed := map[string]string{}
	for i := range MaxFiles + 1 {
		listed[fmt.Sprintf("spec/support/helper_%04d.rb", i)] = "module H; end\n"
	}
	overListed := commitFiles(t, root, listed)
	_, err := ExampleJoin(repo, ExampleJoinOptions{Ruby: reportDocument(t, root, overListed, "")})
	var refusal *Error
	if !errors.As(err, &refusal) || refusal.Code != "tests-bound-exceeded" || refusal.Message != fmt.Sprintf("more than %d source files", MaxJoinFiles) {
		t.Fatalf("support files past the tree listing budget: %v, want the join's tests-bound-exceeded refusal (V1-2088)", err)
	}

	other := t.TempDir()
	big := commitFiles(t, other, map[string]string{
		"spec/report_spec.rb": neverYieldSpec, "spec/support/report_io.rb": "# " + strings.Repeat("x", MaxFileBytes) + "\n",
	})
	if _, err := ExampleJoin(OpenRepo(context.Background(), other), ExampleJoinOptions{Ruby: reportDocument(t, other, big, "")}); !hasCode(err, "tests-bound-exceeded") {
		t.Fatalf("oversized support file: %v", err)
	}
}

func jsReceipt(t *testing.T, root, commit, text string) *jstestprovider.Receipt {
	t.Helper()
	sum := sha256.Sum256([]byte(text))
	file := filepath.Join(root, "tests", "cart.spec.ts")
	return &jstestprovider.Receipt{
		TestRepositoryAtStart: repoAt(commit, "clean"), TestRepositoryAtPublish: repoAt(commit, "clean"),
		Identity: jstestprovider.Identity{TestFileDigests: map[string]string{file: hex.EncodeToString(sum[:])}},
		Tests: []jstestprovider.TestOutcome{{
			Project: &jstestprovider.ProjectIdentity{Name: "chromium"}, Name: "adds an item", FullName: "cart adds an item",
			State: jstestprovider.StatePassed, Anchor: &jstestprovider.Anchor{File: "tests/cart.spec.ts", Line: 2},
		}},
	}
}

// TestExampleJoinJS pins TSA-V0-411, TSA-V0-412 and TSA-V0-414 for a JS receipt: a title-path
// identity, the retained SHA-256 bound to the committed blob, and a digest mismatch refused.
func TestExampleJoinJS(t *testing.T) {
	root := t.TempDir()
	commit := commitFiles(t, root, map[string]string{"tests/cart.spec.ts": cartA})
	report := joinOne(t, root, ExampleJoinOptions{JS: jsReceipt(t, root, commit, cartA),
		Selections: []RequirementSelection{{Selector: "tests/cart.spec.ts", IDs: []string{"REQ-2", "REQ-1"}}}})
	if report.Language != "js" || len(report.Examples) != 1 {
		t.Fatalf("report = %+v", report)
	}
	record := report.Examples[0]
	if record.Example != "tests/cart.spec.ts > chromium > cart adds an item" || record.IdentityKind != ExampleIdentityJS || record.Outcome != "passed" {
		t.Fatalf("record = %+v", record)
	}
	if record.Verdict.Hygiene.State != testvalidity.HygieneEligible || record.Source.Files[0].Blob != blobAt(t, root, commit, "tests/cart.spec.ts") {
		t.Fatalf("verdict/source = %+v %+v", record.Verdict, record.Source)
	}
	if r := record.Requirements; r == nil || r.Basis != RequirementBasisCallerAsserted || !slices.Equal(r.IDs, []string{"REQ-1", "REQ-2"}) {
		t.Fatalf("requirements = %+v", r)
	}
	if _, err := ExampleJoin(OpenRepo(context.Background(), root), ExampleJoinOptions{JS: jsReceipt(t, root, commit, cartB)}); !hasCode(err, "tests-join-revision-mismatch") {
		t.Fatalf("digest mismatch: %v", err)
	}
	outside := jsReceipt(t, root, commit, cartA)
	outside.Identity.TestFileDigests = map[string]string{"/elsewhere/tests/cart.spec.ts": "00"}
	if r := joinOne(t, root, ExampleJoinOptions{JS: outside}); r.Examples[0].Verdict.Hygiene.Reason != joinReasonFileOutsideRepository {
		t.Fatalf("file outside the root: %+v", r.Examples[0].Verdict)
	}
}

// TestExampleJoinJSDigestKeys pins V1-2085: an anchor naming several absolute digest keys joins
// only through a key that is the anchor itself, and abstains with a typed reason when no key is,
// or when its one key lies outside the repository. A reporter-relative anchor is not repository
// relative (the receipt retains no reporter root), so a repository-relative match does not win.
func TestExampleJoinJSDigestKeys(t *testing.T) {
	root := t.TempDir()
	commit := commitFiles(t, root, map[string]string{"tests/cart.spec.ts": cartA, "e2e/tests/cart.spec.ts": cartB})
	sum := sha256.Sum256([]byte(cartB))
	nested := filepath.Join(root, "e2e", "tests", "cart.spec.ts")
	receipt := jsReceipt(t, root, commit, cartA)
	receipt.Identity.TestFileDigests[nested] = hex.EncodeToString(sum[:])
	if r := joinOne(t, root, ExampleJoinOptions{JS: receipt}).Examples[0]; r.Verdict.Hygiene.Reason != joinReasonFileAmbiguous || r.Example != "" {
		t.Fatalf("reporter-relative anchor under two keys: %+v", r)
	}
	receipt.Tests[0].Anchor = &jstestprovider.Anchor{File: nested, Line: 1}
	if r := joinOne(t, root, ExampleJoinOptions{JS: receipt}).Examples[0]; r.Example != "e2e/tests/cart.spec.ts > chromium > cart adds an item" || r.Source.Files[0].Path != "e2e/tests/cart.spec.ts" {
		t.Fatalf("anchor equal to one key: %+v", r)
	}
	receipt.Tests[0].Anchor.File = "cart.spec.ts"
	if r := joinOne(t, root, ExampleJoinOptions{JS: receipt}).Examples[0]; r.Verdict.Hygiene.Reason != joinReasonFileAmbiguous || r.Example != "" {
		t.Fatalf("ambiguous anchor: %+v", r)
	}
	receipt.Tests[0].Anchor.File = "nowhere.spec.ts"
	if r := joinOne(t, root, ExampleJoinOptions{JS: receipt}).Examples[0]; r.Verdict.Hygiene.Reason != joinReasonIdentityUnbound {
		t.Fatalf("unmatched anchor: %+v", r)
	}
}

// TestExampleJoinRevisionRefusals pins TSA-V0-410, TSA-V0-413 and TSA-V0-416.
func TestExampleJoinRevisionRefusals(t *testing.T) {
	root := t.TempDir()
	first := commitFiles(t, root, map[string]string{"spec/calc_spec.rb": calcBefore})
	second := commitFiles(t, root, map[string]string{"spec/calc_spec.rb": calcAfter})
	repo := OpenRepo(context.Background(), root)
	cases := map[string]struct {
		edit func(*ExampleJoinOptions)
		code string
	}{
		"no receipt":        {func(o *ExampleJoinOptions) { o.Ruby = nil }, "tests-invalid-arguments"},
		"two receipts":      {func(o *ExampleJoinOptions) { o.JS = &jstestprovider.Receipt{} }, "tests-invalid-arguments"},
		"no revision":       {func(o *ExampleJoinOptions) { o.Ruby.RepositoryBefore = nil }, "tests-join-revision-unbound"},
		"empty revision":    {func(o *ExampleJoinOptions) { o.Ruby.RepositoryAfter.Revision = "" }, "tests-join-revision-unbound"},
		"moved during run":  {func(o *ExampleJoinOptions) { o.Ruby.RepositoryBefore = repoAt(first, "clean") }, "tests-join-revision-mismatch"},
		"another --rev":     {func(o *ExampleJoinOptions) { o.Revision = first }, "tests-join-revision-mismatch"},
		"unresolved --rev":  {func(o *ExampleJoinOptions) { o.Revision = "no-such-rev" }, "tests-invalid-revision"},
		"examples exceeded": {func(o *ExampleJoinOptions) { o.Ruby.Examples = make([]rubytestprovider.Example, MaxJoinExamples+1) }, "tests-bound-exceeded"},
	}
	for name, c := range cases {
		options := ExampleJoinOptions{Ruby: calcDocument(t, root, second, 4)}
		c.edit(&options)
		if _, err := ExampleJoin(repo, options); !hasCode(err, c.code) {
			t.Errorf("%s: %v, want %s", name, err, c.code)
		}
	}
}

// TestExampleJoinRequirements pins TSA-V0-417..419: selector grammar and mapping file, malformed
// and unknown selectors refused, and a sorted caller-asserted echo that leaves the verdict alone.
func TestExampleJoinRequirements(t *testing.T) {
	for _, pair := range []string{"spec/a.rb", "=REQ-1", "spec/a.rb=", "spec/a.rb=REQ 1", "/abs.rb=REQ-1", "../a.rb=REQ-1",
		"spec/a.rb[0]=REQ-1", "spec/a.rb[1:x]=REQ-1", ".git/config=REQ-1", "spec/a.rb=REQ-1,,REQ-2",
		"spec/a.rb=" + strings.Repeat("R,", MaxRequirementIDs) + "R"} {
		if _, err := ParseRequirementSelection(pair); !hasCode(err, "tests-join-selector-malformed") {
			t.Errorf("%q: %v, want malformed", pair, err)
		}
	}
	selections, err := ParseRequirementMap("# requirements\n\nspec/widget_spec.rb[1:2:1]=TSA-V0-415\n  spec/widget_spec.rb:5=REQ-B,REQ-A\n")
	if err != nil || len(selections) != 2 || selections[1].Selector != "spec/widget_spec.rb:5" {
		t.Fatalf("map = %+v, %v", selections, err)
	}
	if _, err := ParseRequirementMap("spec/a.rb=REQ-1\nbad line\n"); !hasCode(err, "tests-join-selector-malformed") || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("bad map line: %v", err)
	}

	root := t.TempDir()
	commit := commitFiles(t, root, map[string]string{
		"spec/widget_spec.rb": widgetSpec, widgetShared: widgetExamples, "spec/rails_helper.rb": "require 'spec_helper'\n",
	})
	plain := joinOne(t, root, ExampleJoinOptions{Ruby: widgetDocument(t, root, commit, "clean", false)})
	selections = append(selections, RequirementSelection{Selector: "spec/widget_spec.rb", IDs: []string{"REQ-A", "REQ-C"}},
		RequirementSelection{Selector: widgetShared + ":2", IDs: []string{"REQ-D"}})
	report := joinOne(t, root, ExampleJoinOptions{Ruby: widgetDocument(t, root, commit, "clean", false), Selections: selections})
	stub, yielding := report.Examples[0].Requirements, report.Examples[1].Requirements
	if stub == nil || stub.Basis != RequirementBasisCallerAsserted || !slices.Equal(stub.IDs, []string{"REQ-A", "REQ-B", "REQ-C", "REQ-D"}) ||
		!slices.Equal(stub.Selectors, []string{"spec/widget_spec.rb:5", "spec/widget_spec.rb", widgetShared + ":2"}) {
		t.Fatalf("stub requirements = %+v", stub)
	}
	if yielding == nil || !slices.Equal(yielding.IDs, []string{"REQ-A", "REQ-C", "REQ-D", "TSA-V0-415"}) {
		t.Fatalf("yielding requirements = %+v", yielding)
	}
	for index := range plain.Examples {
		if plain.Examples[index].Verdict.Hygiene.State != report.Examples[index].Verdict.Hygiene.State || plain.Examples[index].Outcome != report.Examples[index].Outcome {
			t.Fatalf("requirements changed the verdict or outcome of example %d", index)
		}
	}

	repo := OpenRepo(context.Background(), root)
	refusals := map[string]struct {
		selections []RequirementSelection
		code       string
	}{
		"unknown path":   {[]RequirementSelection{{Selector: "spec/other_spec.rb", IDs: []string{"R"}}}, "tests-join-selector-unknown"},
		"unknown id":     {[]RequirementSelection{{Selector: "spec/widget_spec.rb[1:9]", IDs: []string{"R"}}}, "tests-join-selector-unknown"},
		"unknown line":   {[]RequirementSelection{{Selector: "spec/widget_spec.rb:6", IDs: []string{"R"}}}, "tests-join-selector-unknown"},
		"repeated":       {[]RequirementSelection{{Selector: "spec/widget_spec.rb", IDs: []string{"R"}}, {Selector: "spec/widget_spec.rb", IDs: []string{"S"}}}, "tests-join-selector-malformed"},
		"no ids":         {[]RequirementSelection{{Selector: "spec/widget_spec.rb"}}, "tests-join-selector-malformed"},
		"bad id":         {[]RequirementSelection{{Selector: "spec/widget_spec.rb", IDs: []string{"-R"}}}, "tests-join-selector-malformed"},
		"too many pairs": {make([]RequirementSelection, MaxRequirementSelections+1), "tests-join-selector-malformed"},
	}
	for name, c := range refusals {
		if _, err := ExampleJoin(repo, ExampleJoinOptions{Ruby: widgetDocument(t, root, commit, "clean", false), Selections: c.selections}); !hasCode(err, c.code) {
			t.Errorf("%s: %v, want %s", name, err, c.code)
		}
	}
}
