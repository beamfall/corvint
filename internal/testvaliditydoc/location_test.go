package testvaliditydoc

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/rubytestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// playwrightReceipt is a stock Playwright receipt binding root/e2e/login.spec.ts, with one passed
// outcome anchored at its declaration, one failed outcome anchored at its failure frame and one
// unanchored outcome.
func playwrightReceipt(t *testing.T, root string) Input {
	t.Helper()
	key, err := json.Marshal(filepath.Join(root, "e2e", "login.spec.ts"))
	if err != nil {
		t.Fatal(err)
	}
	body := `{"receipt":{"kind":"e2e","identity":{"runnerName":"playwright","testFileDigests":{` + string(key) + `:"` + strings.Repeat("a", 64) + `"}},"tests":[` +
		`{"name":"login works","state":"passed","anchor":{"file":"e2e/login.spec.ts","line":4}},` +
		`{"name":"login fails","state":"failed","failureMessage":"boom","anchor":{"file":"e2e/login.spec.ts","line":12}},` +
		`{"name":"login unanchored","state":"passed"}]}}`
	input, err := Decode([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return input
}

// LPCV-V0-145, LPCV-V0-146: a stock Playwright test anchored at its declaration carries its line,
// and its repository-relative file once Locate binds the root; a failure-frame anchor and a missing
// anchor carry no file or line, each absent member named with one closed reason; no title path is
// claimed before source is read.
func TestLPCVV0145PlaywrightLocation(t *testing.T) {
	root := t.TempDir()
	input := playwrightReceipt(t, root)
	projected := Project(input)
	want := []*Location{
		{Line: 4, Absent: map[string]string{"file": LocationRootUnbound, "titlePath": LocationSourceNotRead}},
		{Absent: map[string]string{"file": LocationNotDeclaration, "line": LocationNotDeclaration, "titlePath": LocationSourceNotRead}},
		{Absent: map[string]string{"file": LocationNotRecorded, "line": LocationNotRecorded, "titlePath": LocationSourceNotRead}},
	}
	for index, test := range projected.Tests {
		if !reflect.DeepEqual(test.Location, want[index]) {
			t.Fatalf("test %d location=%+v want %+v", index, test.Location, want[index])
		}
	}

	located := Locate(projected, input, root)
	first := located.Tests[0].Location
	if first.File != "e2e/login.spec.ts" || first.Line != 4 || !reflect.DeepEqual(first.Absent, map[string]string{"titlePath": LocationSourceNotRead}) {
		t.Fatalf("located=%+v", first)
	}
	if projected.Tests[0].Location.File != "" || !reflect.DeepEqual(located.Tests[1:], projected.Tests[1:]) {
		t.Fatalf("Locate changed its input or a non-declaration test")
	}
	data, err := json.Marshal(located.Tests[0])
	if err != nil || !strings.Contains(string(data), `"location":{"file":"e2e/login.spec.ts","line":4,"absent":{"titlePath":"source-not-read"}}`) {
		t.Fatalf("location member: %s %v", data, err)
	}

	elsewhere := Locate(projected, input, t.TempDir())
	if got := elsewhere.Tests[0].Location; got.File != "" || got.Absent["file"] != LocationFileUnbound {
		t.Fatalf("unbound root location=%+v", got)
	}
}

// LPCV-V0-145: a profiled receipt's reporter anchors every test at its declaration, failed or not.
func TestLPCVV0145ProfiledPlaywrightAnchorsAreDeclarations(t *testing.T) {
	root := t.TempDir()
	input := playwrightReceipt(t, root)
	input.js.Profile = "corvint-playwright-external/0" // a profiled receipt decodes only canonically
	located := Locate(Project(input), input, root)
	failed := located.Tests[1].Location
	if failed.File != "e2e/login.spec.ts" || failed.Line != 12 || failed.Absent["file"] != "" {
		t.Fatalf("profiled failed location=%+v", failed)
	}
}

// LPCV-V0-146: a Go session event records no file, line or title path.
func TestLPCVV0146GoLocationNotRecorded(t *testing.T) {
	document := projectDocument(t, `{"profile":"corvint-go-live-session-event/0","state":"passed","identity":"abc","sequence":4,"scope":["./..."],"detail":"","projection":{},"testProjections":[{"package":"example/a","name":"TestAdds","action":"pass","projection":{}}],"testProjectionsOmitted":0}`)
	want := &Location{Absent: map[string]string{"file": LocationNotRecorded, "line": LocationNotRecorded, "titlePath": LocationNotRecorded}}
	if !reflect.DeepEqual(document.Tests[0].Location, want) {
		t.Fatalf("go location=%+v", document.Tests[0].Location)
	}
}

// LPCV-V0-145, LPCV-V0-148: an RSpec example carries the provider's worktree-relative file and
// line; a shared-group example carries its definition, its inclusion sites, and no title path,
// since its path depends on the inclusion.
func TestLPCVV0148RubyLocationAndSharedGroup(t *testing.T) {
	document := rubyDocument(t, "passed", "passed")
	document.Examples[1].SharedExample = &rubytestprovider.SharedExample{Origin: "spec/user_spec.rb:3", IncludedAt: []string{"spec/admin_spec.rb:7", "spec/root_spec.rb:2"}}
	input, err := Decode(encodeRuby(t, document))
	if err != nil {
		t.Fatal(err)
	}
	projected := Project(input)
	plain := &Location{File: "spec/user_spec.rb", Line: 2, Absent: map[string]string{"titlePath": LocationSourceNotRead}}
	shared := &Location{File: "spec/user_spec.rb", Line: 3, IncludedAt: []string{"spec/admin_spec.rb:7", "spec/root_spec.rb:2"}, Absent: map[string]string{"titlePath": LocationSharedExample}}
	if !reflect.DeepEqual(projected.Tests[0].Location, plain) || !reflect.DeepEqual(projected.Tests[1].Location, shared) {
		t.Fatalf("locations=%+v %+v", projected.Tests[0].Location, projected.Tests[1].Location)
	}
	data, err := json.Marshal(projected.Tests[1].Location)
	if err != nil || string(data) != `{"file":"spec/user_spec.rb","line":3,"includedAt":["spec/admin_spec.rb:7","spec/root_spec.rb:2"],"absent":{"titlePath":"shared-example-definition"}}` {
		t.Fatalf("shared location=%s %v", data, err)
	}

	shared.placeSource("spec/user_spec.rb", 3, testvalidity.ExampleSource{TitlePath: []string{"User", "reads"}})
	if shared.TitlePath != nil || shared.Absent["titlePath"] != LocationSharedExample {
		t.Fatalf("a shared example took a title path: %+v", shared)
	}
	plain.placeSource("spec/user_spec.rb", 2, testvalidity.ExampleSource{})
	if plain.Absent["titlePath"] != LocationTitleNotLiteral {
		t.Fatalf("a non-literal title path is not marked: %+v", plain)
	}
	plain.placeSource("spec/user_spec.rb", 2, testvalidity.ExampleSource{TitlePath: []string{"User", "saves"}})
	if !reflect.DeepEqual(plain.TitlePath, []string{"User", "saves"}) || len(plain.Absent) != 0 {
		t.Fatalf("a literal title path is not recorded: %+v", plain)
	}
}

// LPCV-V0-150, LPCV-V0-151: a static projection reads hygiene from source alone: skipped, focused
// and fixme examples are INELIGIBLE, an assertion-free example is named in sourceRules, an
// undecidable file abstains, and execution, freshness and strength stay NOT_OBSERVED in a
// non-promotable preview.
func TestLPCVV0151StaticProjection(t *testing.T) {
	finding := func(rule, at string) []testvalidity.SourceFinding {
		severity := testvalidity.SeverityInvalid
		if rule == testvalidity.RuleJSNoAssertion {
			severity = testvalidity.SeverityAdvisory
		}
		return []testvalidity.SourceFinding{{Rule: rule, Severity: severity, Test: at, Location: at, Cause: at}}
	}
	located := func(expectations int, findings []testvalidity.SourceFinding, title ...string) testvalidity.ExampleSource {
		if findings == nil {
			findings = []testvalidity.SourceFinding{}
		}
		return testvalidity.ExampleSource{Read: testvalidity.ExampleSourceLocated, Expectations: expectations, Findings: findings, TitlePath: title}
	}
	files := []StaticFile{
		{Path: "e2e/login.spec.ts", Examples: map[int]testvalidity.ExampleSource{
			13: located(1, finding(testvalidity.RuleJSSkipped, "e2e/login.spec.ts:13"), "login", "broken"),
			4:  located(1, nil, "login", "works"),
			16: located(0, finding(testvalidity.RuleJSNoAssertion, "e2e/login.spec.ts:16"), "login", "no assertion"),
			10: located(1, finding(testvalidity.RuleJSFocused, "e2e/login.spec.ts:10")),
		}},
		{Path: "spec/user_spec.rb", Examples: map[int]testvalidity.ExampleSource{
			6: located(1, finding(testvalidity.RuleRSpecSkipped, "spec/user_spec.rb:6"), "User", "skipped"),
			9: located(1, finding(testvalidity.RuleRSpecFocused, "spec/user_spec.rb:9"), "User", "focused"),
			3: located(0, nil, "User", "no expectation"),
		}},
		{Path: "spec/dynamic_spec.rb", Reason: "rspec-source-unparsed"},
	}
	document := ProjectStatic("rev", files)
	type row struct {
		id, name, hygiene, reason string
		expectations              int
	}
	got := []row{}
	for _, test := range document.Tests {
		p := test.Projection
		if test.State != StaticState || p.Execution.State != StateNotObserved || p.Freshness.State != StateNotObserved || p.Strength.State != StateNotObserved ||
			p.Association.State != testvalidity.StateUnsupported || test.Location == nil || test.Location.File == "" {
			t.Fatalf("static test=%+v", test)
		}
		expectations := -1
		if test.SourceRules != nil {
			expectations = test.SourceRules.Expectations
		}
		got = append(got, row{test.ID, test.Name, p.Hygiene.State, p.Hygiene.Reason, expectations})
	}
	want := []row{
		{"e2e/login.spec.ts:4", "works", testvalidity.HygieneEligible, testvalidity.HygieneReasonJSExampleRead, 1},
		{"e2e/login.spec.ts:10", "e2e/login.spec.ts:10", testvalidity.HygieneIneligible, testvalidity.RuleJSFocused, 1},
		{"e2e/login.spec.ts:13", "broken", testvalidity.HygieneIneligible, testvalidity.RuleJSSkipped, 1},
		{"e2e/login.spec.ts:16", "no assertion", testvalidity.HygieneEligible, testvalidity.HygieneReasonJSExampleRead, 0},
		{"spec/user_spec.rb:3", "no expectation", testvalidity.HygieneEligible, "rspec-example-source-read", 0},
		{"spec/user_spec.rb:6", "skipped", testvalidity.HygieneIneligible, testvalidity.RuleRSpecSkipped, 1},
		{"spec/user_spec.rb:9", "focused", testvalidity.HygieneIneligible, testvalidity.RuleRSpecFocused, 1},
		{"spec/dynamic_spec.rb", "spec/dynamic_spec.rb", testvalidity.HygieneAbstained, "rspec-source-unparsed", -1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("static rows\n%v\nwant\n%v", got, want)
	}
	if rules := document.Tests[3].SourceRules.Findings; len(rules) != 1 || rules[0].Rule != testvalidity.RuleJSNoAssertion {
		t.Fatalf("assertion-free finding=%+v", rules)
	}
	if loc := document.Tests[1].Location; loc.Line != 10 || loc.Absent["titlePath"] != LocationTitleNotLiteral {
		t.Fatalf("non-literal location=%+v", loc)
	}
	if loc := document.Tests[7].Location; loc.Absent["line"] != LocationSourceUnplaced || loc.Absent["titlePath"] != LocationSourceUnplaced {
		t.Fatalf("uncertain location=%+v", loc)
	}
	if document.Source != StaticSource || document.Kind != StaticKind || document.Tier != "preview" || document.Promotable == nil || *document.Promotable ||
		document.Run.Execution.State != StateNotObserved || document.Run.Hygiene.State != testvalidity.StateUnsupported ||
		!reflect.DeepEqual(document.Static, &Static{Revision: "rev", Files: []string{"e2e/login.spec.ts", "spec/user_spec.rb", "spec/dynamic_spec.rb"}}) {
		t.Fatalf("static document=%+v", document)
	}
}

// LPCV-V0-146, LPCV-V0-147: a test placed on a line that declares more than one test takes the
// file and line but no title path, which is absent with declaration-line-ambiguous, even when the
// reading carries one; the static projection reports the same.
func TestLPCVV0146AmbiguousLineTitlePathAbsent(t *testing.T) {
	ambiguous := testvalidity.ExampleSource{Read: testvalidity.HygieneReasonJSLineAmbiguous, Findings: []testvalidity.SourceFinding{}, TitlePath: []string{"b"}}
	location := newLocation()
	location.placeSource("e2e/two.spec.ts", 2, ambiguous)
	if want := (&Location{File: "e2e/two.spec.ts", Line: 2, Absent: map[string]string{"titlePath": LocationLineAmbiguous}}); !reflect.DeepEqual(location, want) {
		t.Fatalf("location=%+v want %+v", location, want)
	}
	document := ProjectStatic("rev", []StaticFile{{Path: "e2e/two.spec.ts", Examples: map[int]testvalidity.ExampleSource{2: ambiguous}}})
	if got := document.Tests[0].Location; got.TitlePath != nil || got.Absent["titlePath"] != LocationLineAmbiguous {
		t.Fatalf("static location=%+v", got)
	}
}

// LPCV-V0-292: stock Playwright names a spec relative to its rootDir. With rootDir e2e/ the anchor
// login.spec.ts binds the one retained key ending in /login.spec.ts and the location is the
// repository-relative e2e/login.spec.ts; with no root it stays repository-root-unbound, and when
// two retained keys share the rootDir-relative tail it is anchor-file-unbound, never a guess.
func TestLPCVV0292PlaywrightRootDirRelativeSpec(t *testing.T) {
	receipt := func(t *testing.T, keys ...string) Input {
		t.Helper()
		digests := map[string]string{}
		for _, key := range keys {
			digests[key] = strings.Repeat("a", 64)
		}
		data, err := json.Marshal(map[string]any{"receipt": map[string]any{"kind": "e2e", "identity": map[string]any{"runnerName": "playwright", "testFileDigests": digests},
			"tests": []map[string]any{{"name": "login works", "state": "passed", "anchor": map[string]any{"file": "login.spec.ts", "line": 4}}}}})
		if err != nil {
			t.Fatal(err)
		}
		input, err := Decode(data)
		if err != nil {
			t.Fatal(err)
		}
		return input
	}
	root := t.TempDir()
	bound := receipt(t, filepath.Join(root, "e2e", "login.spec.ts"), filepath.Join(root, "e2e", "cart.spec.ts"))
	located := Locate(Project(bound), bound, root).Tests[0].Location
	if !reflect.DeepEqual(located, &Location{File: "e2e/login.spec.ts", Line: 4, Absent: map[string]string{"titlePath": LocationSourceNotRead}}) {
		t.Fatalf("rootDir-relative location=%+v", located)
	}
	if got := Locate(Project(bound), bound, "").Tests[0].Location; got.File != "" || got.Absent["file"] != LocationRootUnbound {
		t.Fatalf("no-root location=%+v", got)
	}
	twice := receipt(t, filepath.Join(root, "e2e", "login.spec.ts"), filepath.Join(root, "e2e", "admin", "login.spec.ts"))
	if got := Locate(Project(twice), twice, root).Tests[0].Location; got.File != "" || got.Absent["file"] != LocationFileUnbound {
		t.Fatalf("shared-tail location=%+v", got)
	}
}
