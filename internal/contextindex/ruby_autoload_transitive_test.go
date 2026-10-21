package contextindex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rubyIssue1130Files reads the GitHub #1129/#1130 Rails fixture: FooClient,
// Bar::BazService calling it, and an RSpec spec for each.
func rubyIssue1130Files(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("testdata", "ruby-autoload-1130")
	files := map[string]string{}
	err := filepath.WalkDir(root, func(name string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, name)
		files[filepath.ToSlash(relative)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func requireRubyRow(t *testing.T, receipt map[string]any, id, kind string, score int, via string) map[string]any {
	t.Helper()
	row := rubyResultRows(receipt)[id]
	if row == nil || row["kind"] != kind || row["score"] != score || row["advisory"] != true {
		t.Fatalf("%s: want advisory %s/%d, got %#v in %#v", id, kind, score, row, receipt["results"])
	}
	if got, _ := row["via"].(string); got != via {
		t.Fatalf("%s: via = %q, want %q", id, got, via)
	}
	return row
}

// TestRubyAutoloadIssue1130DefaultTransitive: NGI-V0-033, NGI-V0-034,
// NGI-V0-035, NGI-V0-036. Default impact on FooClient reaches its caller and
// spec, and the caller's spec at the second hop, all advisory.
func TestRubyAutoloadIssue1130DefaultTransitive(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, rubyIssue1130Files(t)))
	if err != nil {
		t.Fatal(err)
	}
	const changed = "app/api_clients/foo_client.rb"
	receipt, err := Impact(index, []string{changed}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	requireRubyRow(t, receipt, "app/services/bar/baz_service.rb", "reference", rubyConstantReferenceScore, "")
	requireRubyRow(t, receipt, "spec/api_clients/foo_client_spec.rb", "test", rubyConstantTestScore, "")
	second := requireRubyRow(t, receipt, "spec/services/bar/baz_service_spec.rb", "test", rubyTransitiveTestScore, "app/services/bar/baz_service.rb")
	evidenceRows := mapsFromAny(second["evidence"])
	if len(evidenceRows) != 1 || evidenceRows[0]["line"] != 3 || evidenceRows[0]["confidence"] != "low" || evidenceRows[0]["authority"] != "syntax" ||
		!strings.Contains(stringValue(evidenceRows[0]["reason"]), "Bar::BazService") || !strings.Contains(stringValue(second["summary"]), "FooClient") {
		t.Fatalf("second-hop row = %#v", second)
	}
	// Spec helpers name no mapped constant and are not reached.
	for _, absent := range []string{"spec/rails_helper.rb", "spec/support/helper.rb", "spec/support/shared_examples/fetches.rb"} {
		if _, wrong := rubyResultRows(receipt)[absent]; wrong {
			t.Fatalf("%s must not be reached", absent)
		}
	}
	coverage := receipt["coverage"].(map[string]any)
	if coverage["advisory_results"] != 3 || coverage["authoritative_results"] != len(mapsFromAny(receipt["results"]))-3 {
		t.Fatalf("advisory counts = %#v", coverage)
	}
	if !stringIn(stringsField(coverage["uncertainty"]), rubyAutoloadAdvisoryUncertainty) ||
		stringIn(stringsField(coverage["uncertainty"]), "learned path relationships are advisory") {
		t.Fatalf("uncertainty = %#v", coverage["uncertainty"])
	}
	if rubyUnknownCount(receipt, "reverse-import-rule-unavailable") != 0 || rubyUnknownCount(receipt, "dynamic-dispatch-unresolved") != 1 {
		t.Fatalf("frontier = %#v", receipt["unknowns"])
	}
	// The second hop never ranks above a first hop and never goes further:
	// changing the caller's spec reaches nothing, and an unmapped path keeps
	// the unavailable-rule row.
	order := map[string]int{}
	for position, item := range mapsFromAny(receipt["results"]) {
		order[stringValue(item["id"])] = position
	}
	if order["spec/services/bar/baz_service_spec.rb"] < order["spec/api_clients/foo_client_spec.rb"] {
		t.Fatalf("second hop ranks above a first hop: %v", order)
	}
	spec, err := Impact(index, []string{"spec/services/bar/baz_service_spec.rb"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	if rubyUnknownCount(spec, "reverse-import-rule-unavailable") != 1 || rubyUnknownCount(spec, "ruby-constant-unmapped") != 1 {
		t.Fatalf("unmapped spec frontier = %#v", spec["unknowns"])
	}
}

// TestRubyAutoloadSecondHopOnce: NGI-V0-034. A path both hops reach is
// reported once at its first-hop rank, a path two first hops reach is
// reported once, and a third hop is never followed.
func TestRubyAutoloadSecondHopOnce(t *testing.T) {
	files := map[string]string{
		"app/models/a.rb":       "class A\nend\n",
		"app/models/b.rb":       "class B\n  def x = A.new\nend\n",
		"app/models/c.rb":       "class C\n  def x = A.new\nend\n",
		"app/models/d.rb":       "class D\n  def x = [A, B, C]\nend\n",
		"app/models/e.rb":       "class E\n  def x = [B, C]\nend\n",
		"app/models/f.rb":       "class F\n  def x = E.new\nend\n",
		"spec/models/e_spec.rb": "RSpec.describe E do\nend\n",
	}
	receipt := registryImpact(t, files, "app/models/a.rb")
	rows := rubyResultRows(receipt)
	for _, first := range []string{"app/models/b.rb", "app/models/c.rb", "app/models/d.rb"} {
		if rows[first] == nil || rows[first]["score"] != rubyConstantReferenceScore || rows[first]["via"] != nil {
			t.Fatalf("%s must be a first hop: %#v", first, rows[first])
		}
	}
	if row := rows["app/models/e.rb"]; row == nil || row["score"] != rubyTransitiveReferenceScore || row["via"] != "app/models/b.rb" {
		t.Fatalf("e.rb second hop = %#v", row)
	}
	for _, third := range []string{"app/models/f.rb", "spec/models/e_spec.rb"} {
		if _, wrong := rows[third]; wrong {
			t.Fatalf("%s is a third hop and must not be reported", third)
		}
	}
	seen := map[string]int{}
	for _, item := range mapsFromAny(receipt["results"]) {
		seen[stringValue(item["id"])]++
	}
	for id, count := range seen {
		if count != 1 {
			t.Fatalf("%s reported %d times", id, count)
		}
	}
}

// TestRubyAutoloadRangeImpact: NGI-V0-035, NGI-V0-037. Range impact over a committed
// change to FooClient attaches the same advisory rows, both hops, and the
// ruby_autoload block.
func TestRubyAutoloadRangeImpact(t *testing.T) {
	t.Parallel()
	files := rubyIssue1130Files(t)
	root := impactRepositoryWithFiles(t, files)
	base := testGit(t, root, "rev-parse", "HEAD")
	const changed = "app/api_clients/foo_client.rb"
	writeTestFile(t, root, changed, strings.Replace(files[changed], "ok: true", "ok: false", 1))
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "changed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	ranged, err := compileRangeImpact(context.Background(), index, base, maxLimit, false)
	if err != nil {
		t.Fatal(err)
	}
	path, err := Impact(index, []string{changed}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range rubyResultRows(path) {
		if want["advisory"] != true {
			continue
		}
		got := rubyResultRows(ranged)[id]
		gotBytes, _ := CanonicalJSON(got)
		wantBytes, _ := CanonicalJSON(want)
		if string(gotBytes) != string(wantBytes) {
			t.Fatalf("%s: range row %s, path row %s", id, gotBytes, wantBytes)
		}
	}
	requireRubyRow(t, ranged, "spec/services/bar/baz_service_spec.rb", "test", rubyTransitiveTestScore, "app/services/bar/baz_service.rb")
	rangeReport, _ := CanonicalJSON(ranged["ruby_autoload"])
	pathReport, _ := CanonicalJSON(path["ruby_autoload"])
	if ranged["ruby_autoload"] == nil || string(rangeReport) != string(pathReport) {
		t.Fatalf("range ruby_autoload = %s, path %s", rangeReport, pathReport)
	}
	rangeCoverage := ranged["coverage"].(map[string]any)
	if !stringIn(stringsField(rangeCoverage["uncertainty"]), rubyAutoloadAdvisoryUncertainty) {
		t.Fatalf("range uncertainty = %#v", rangeCoverage["uncertainty"])
	}
	if rubyUnknownCount(ranged, "reverse-import-rule-unavailable") != 0 || rangeCoverage["advisory_results"] != 3 {
		t.Fatalf("range frontier/coverage = %#v %#v", ranged["unknowns"], ranged["coverage"])
	}
}

// TestIsTestPathRSpec: NGI-V0-038. The Ruby reading's test-path rule
// classifies RSpec's layout as tests, for Ruby paths only; the shared rule
// keeps its base classification.
func TestIsTestPathRSpec(t *testing.T) {
	for _, name := range []string{"spec/models/thing_spec.rb", "spec/support/helper.rb", "spec/rails_helper.rb",
		"engines/billing/spec/models/invoice_spec.rb", "app/models/thing_spec.rb"} {
		if !isRubyTestPath(name) {
			t.Fatalf("%s must be a Ruby-reading test path", name)
		}
		if isTestPath(name) {
			t.Fatalf("%s must keep the shared rule's base classification", name)
		}
	}
	for _, name := range []string{"app/models/thing.rb", "app/models/spectrum.rb", "lib/specs.rb", "spec/fixtures/data.json", "docs/spec/readme.md", "spec/thing.go"} {
		if isRubyTestPath(name) || isTestPath(name) {
			t.Fatalf("%s must not be a test path", name)
		}
	}
	for _, name := range []string{"test/models/thing_test.rb", "spec/thing.spec.ts", "internal/x/x_test.go"} {
		if !isRubyTestPath(name) || !isTestPath(name) {
			t.Fatalf("%s must be a test path under both rules", name)
		}
	}
}

// TestRubyAutoloadMappedPathHasNoProfileGap: NGI-V0-036 (V1-1814). A Ruby path
// the autoload mapper resolves has a named reverse-reference rule, so neither
// the default receipt nor the budgeted one the CLI compiles carries the
// unruled-suffix profile-gap line; an unmapped non-test Ruby path still does.
func TestRubyAutoloadMappedPathHasNoProfileGap(t *testing.T) {
	files := rubyIssue1130Files(t)
	files["lib/tasks/unmapped.rb"] = "class Unmapped\nend\n"
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	budget := 64 * 1024
	gapLine := func(receipt map[string]any) string {
		for _, line := range stringsField(receipt["coverage"].(map[string]any)["uncertainty"]) {
			if strings.Contains(line, "no named resolution rule") {
				return line
			}
		}
		return ""
	}
	for _, test := range []struct {
		path string
		gap  bool
	}{{"app/api_clients/foo_client.rb", false}, {"lib/tasks/unmapped.rb", true}} {
		plain, err := Impact(index, []string{test.path}, maxLimit)
		if err != nil {
			t.Fatal(err)
		}
		budgeted, err := EvalImpact(index, []string{test.path}, maxLimit, &budget)
		if err != nil {
			t.Fatal(err)
		}
		for name, receipt := range map[string]map[string]any{"default": plain, "budgeted": budgeted} {
			if got := gapLine(receipt) != ""; got != test.gap {
				t.Fatalf("%s %s: profile gap line present = %v, want %v: %#v", name, test.path, got, test.gap, receipt["coverage"])
			}
		}
	}
}
