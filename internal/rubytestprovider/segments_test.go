// SPDX-License-Identifier: Apache-2.0

package rubytestprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

// The committed fixture (testdata/segments) and the report the collector
// recorded over it (testdata/segments-report.json, rewritten by the live test
// with CORVINT_RECORD_RUBY_SEGMENTS=1).
const (
	segFixture  = "testdata/segments"
	segRecorded = "testdata/segments-report.json"
	segPrefix   = "shop"
)

// Test identities of the fixture.
const (
	tSearchA  = "shop/spec/features/search_spec.rb.[1:1]" // shared 'a catalog search'
	tSearchB  = "shop/spec/features/search_spec.rb.[2:1]" // the same shared example
	tGadget   = "shop/spec/features/search_spec.rb.[3:1]" // same steps, different checks
	tItem     = "shop/spec/features/search_spec.rb.[4:1]"
	tCartA    = "shop/spec/features/cart_spec.rb.[1:1]" // shared 'a cart add'
	tCartB    = "shop/spec/features/cart_spec.rb.[2:1]"
	tCartG    = "shop/spec/features/cart_spec.rb.[3:1]"
	tPriceA   = "shop/spec/unit/pricing_spec.rb.[1:1]" // shared 'a widget price'
	tPriceB   = "shop/spec/unit/pricing_spec.rb.[2:1]"
	tPriceGad = "shop/spec/unit/pricing_spec.rb.[3:1]"
)

func readRecorded(t *testing.T) SegmentReport {
	t.Helper()
	data, err := os.ReadFile(segRecorded)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ParseSegmentReport(data)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// fixtureOIDs pins every fixture file under segPrefix to a synthetic blob.
func fixtureOIDs(t *testing.T) map[string]string {
	t.Helper()
	oids := map[string]string{}
	err := filepath.WalkDir(segFixture, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(segFixture, p)
		oids[segPrefix+"/"+filepath.ToSlash(rel)] = "oid:" + filepath.ToSlash(rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return oids
}

func segOptions(oids map[string]string) SegmentIndexOptions {
	return SegmentIndexOptions{Module: "shop", Commit: "c0ffee", Runner: "ruby rspec corvint-ruby-segments/0", Prefix: segPrefix,
		OIDs: oids, Rules: []linecov.LevelRule{{Prefix: segPrefix + "/spec/unit", Level: linecov.LevelUnit}}}
}

func buildRecorded(t *testing.T, mutate func(*SegmentReport)) (*linecov.Index, map[string]string) {
	t.Helper()
	r := readRecorded(t)
	if mutate != nil {
		mutate(&r)
	}
	oids := fixtureOIDs(t)
	ix, err := BuildSegmentIndex(r, segOptions(oids))
	if err != nil {
		t.Fatal(err)
	}
	return ix, oids
}

func testIndex(ix *linecov.Index, id string) int32 {
	for i := range ix.Tests {
		if ix.TestID(int32(i)) == id {
			return int32(i)
		}
	}
	return -1
}

// MCJ-V0-041, MCJ-V0-042, MCJ-V0-040: the collector's report builds a
// linecov-index/3 index with one test per passing example, its segments in
// run order with their kinds, anchored identities bound at the commit, and
// levels from declaration, path rule and runner convention.
func TestMCJV0041SegmentReportBuildsIndex(t *testing.T) {
	ix, _ := buildRecorded(t, nil)
	if ix.Format != linecov.Format || len(ix.Tests) != 10 {
		t.Fatalf("format %s, %d tests", ix.Format, len(ix.Tests))
	}
	kinds := func(id string) []string {
		var out []string
		for _, s := range ix.TestSegments(testIndex(ix, id)) {
			seg := ix.Segment(s)
			if !seg.Bound {
				t.Errorf("%s: segment %s unbound", id, seg.Label())
			}
			out = append(out, string(seg.Kind)+" "+seg.ID)
		}
		return out
	}
	want := []string{
		"setup before@spec/spec_helper.rb:15",
		"setup before@spec/features/search_spec.rb:6",
		"step it@spec/features/search_spec.rb:8#1",
		"method HomePage#search_for@spec/pages/home_page.rb:8",
		"step it@spec/features/search_spec.rb:8#2",
		"method ResultsPage#titles@spec/pages/results_page.rb:7",
	}
	if got := kinds(tSearchA); !reflect.DeepEqual(got, want) {
		t.Fatalf("search segments\n got %q\nwant %q", got, want)
	}
	if got := kinds(tCartA); !reflect.DeepEqual(got, []string{"setup before@spec/spec_helper.rb:15",
		"step it@spec/features/cart_spec.rb:6#1", "step ShopSteps#add_to_cart@spec/support/steps/shop_steps.rb:10"}) {
		t.Fatalf("cart segments %q", got)
	}
	if got := kinds(tPriceA); !reflect.DeepEqual(got, []string{"test it@spec/unit/pricing_spec.rb:6"}) {
		t.Fatalf("pricing segments %q", got)
	}
	levels := map[string]string{tSearchA: linecov.LevelE2E, tCartG: linecov.LevelE2E, tPriceA: linecov.LevelUnit,
		tPriceGad: linecov.LevelIntegration} // declared metadata beats the unit path rule
	for id, l := range levels {
		if got := ix.Tests[testIndex(ix, id)].Level; got != l {
			t.Errorf("%s level %q, want %q", id, got, l)
		}
	}
	// A line no example ran is instrumented and uncovered; a run line names
	// its tests through segments.
	refs, ok := ix.BlocksAt(segPrefix+"/app/shop.rb", "oid:app/shop.rb", 33, 33)
	if !ok || len(refs) != 1 || len(ix.Files[refs[0].File].Blocks[refs[0].Block].Tests) != 1 {
		t.Fatalf("reset line blocks %v %v", refs, ok)
	}
	// Without a level rule the unit pricing example is unknown.
	o := segOptions(fixtureOIDs(t))
	o.Rules = nil
	ix2, err := BuildSegmentIndex(readRecorded(t), o)
	if err != nil {
		t.Fatal(err)
	}
	if got := ix2.Tests[testIndex(ix2, tPriceA)].Level; got != linecov.LevelUnknown {
		t.Fatalf("undeclared unit level %q", got)
	}
}

// MCJ-V0-042: an anchor outside the commit, a dirty anchor, a failed example
// and an executed uncommitted or dirty file are handled conservatively.
func TestMCJV0042BindingAndRefusals(t *testing.T) {
	r := readRecorded(t)
	oids := fixtureOIDs(t)
	o := segOptions(oids)
	o.Dirty = map[string]bool{segPrefix + "/spec/pages/home_page.rb": true}
	if _, err := BuildSegmentIndex(r, o); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("an executed dirty file built: %v", err)
	}
	o = segOptions(oids)
	delete(o.OIDs, segPrefix+"/app/shop.rb")
	if _, err := BuildSegmentIndex(r, o); err == nil || !strings.Contains(err.Error(), "no blob") {
		t.Fatalf("an executed uncommitted file built: %v", err)
	}
	ix, _ := buildRecorded(t, func(r *SegmentReport) {
		r.Examples[0].Segments[0].AnchorPath = "spec/elsewhere.rb"
		r.Examples[1].Status = "failed"
	})
	if len(ix.Tests) != 9 {
		t.Fatalf("a failed example recorded: %d tests", len(ix.Tests))
	}
	first := ix.Segment(ix.TestSegments(0)[0])
	if first.Bound {
		t.Fatal("a segment anchored outside the commit bound")
	}
	// A file first loaded inside an example is not mapped by line.
	ix, _ = buildRecorded(t, func(r *SegmentReport) { r.Examples[0].Loaded = []string{"spec/pages/item_page.rb"} })
	if _, ok := ix.BlocksAt(segPrefix+"/spec/pages/item_page.rb", "oid:spec/pages/item_page.rb", 8, 8); ok {
		t.Fatal("a file loaded inside an example was indexed")
	}
	if e := SegmentEffect(ix, change("spec/pages/item_page.rb", 8)); e.Fallback == "" {
		t.Fatal("a change to a file loaded inside an example did not keep the static plan")
	}
	o = segOptions(oids)
	if _, err := BuildSegmentIndex(r, SegmentIndexOptions{OIDs: oids}); err == nil {
		t.Fatal("an index without identity built")
	}
	bad := readRecorded(t)
	bad.Examples[0].Incomplete = []string{"made-up"}
	if _, err := BuildSegmentIndex(bad, o); err == nil {
		t.Fatal("an unnamed incomplete reason built")
	}
}

// MCJ-V0-041: a report that cannot be trusted decides nothing.
func TestMCJV0041ParseRefusesDamagedReports(t *testing.T) {
	data, err := os.ReadFile(segRecorded)
	if err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string]func(map[string]any){
		"format":   func(m map[string]any) { m["format"] = "other/1" },
		"kind":     func(m map[string]any) { seg(m)["kind"] = "loop" },
		"identity": func(m map[string]any) { seg(m)["id"] = "" },
		"absolute": func(m map[string]any) { seg(m)["lines"] = map[string]any{"/etc/passwd": []any{[]any{1, 1}}} },
		"line":     func(m map[string]any) { seg(m)["lines"] = map[string]any{"app/shop.rb": []any{[]any{0, 1}}} },
		"repeated": func(m map[string]any) {
			ex := m["examples"].([]any)
			m["examples"] = append(ex, ex[0])
		},
		"id": func(m map[string]any) { m["examples"].([]any)[0].(map[string]any)["id"] = "nope" },
	} {
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		edit(m)
		out, _ := json.Marshal(m)
		if _, err := ParseSegmentReport(out); err == nil {
			t.Errorf("%s: damaged report parsed", name)
		}
	}
}

func seg(m map[string]any) map[string]any {
	return m["examples"].([]any)[0].(map[string]any)["segments"].([]any)[0].(map[string]any)
}

// MCJ-V0-013: Capybara and SitePrism run under the RSpec collector; any other
// Ruby family keeps the static plan as coverage-collector-missing.
func TestMCJV0013CollectorStatus(t *testing.T) {
	for _, f := range []string{"rspec", "Capybara", "site_prism"} {
		if c, ok, why := CollectorStatus(f); !ok || c != SegmentsFormat || why != "" {
			t.Errorf("%s: %q %v %q", f, c, ok, why)
		}
	}
	for _, f := range []string{"minitest", "test-unit", "rails"} {
		if _, ok, why := CollectorStatus(f); ok || why != ReasonCollectorMissing {
			t.Errorf("%s: %v %q", f, ok, why)
		}
	}
}

func change(file string, line int) []linecov.FileDiff {
	p := segPrefix + "/" + file
	return []linecov.FileDiff{{Path: p, OldOID: "oid:" + file, NewOID: "new", Hunks: []linecov.Hunk{{Start: line, End: line}}}}
}

// cheaperOf is the representative MCJ-V0-006 must pick between two
// equivalent tests: the smaller recorded elapsed time, then test order.
func cheaperOf(ix *linecov.Index, a, b string) (rep, other string) {
	ia, ib := testIndex(ix, a), testIndex(ix, b)
	ea, eb := ix.Tests[ia].Elapsed, ix.Tests[ib].Elapsed
	if ea < eb || ea == eb && ia < ib {
		return a, b
	}
	return b, a
}

// segFaultCases is the MCJ-V0-014 matrix over the collector's own output: per
// kind, a change reaching a non-shared segment keeps its test, and a change
// reaching only an equivalent segment excludes all but the representative.
func segFaultCases(ix *linecov.Index) []linecov.FaultCase {
	sRep, sOther := cheaperOf(ix, tSearchA, tSearchB)
	cRep, cOther := cheaperOf(ix, tCartA, tCartB)
	pRep, pOther := cheaperOf(ix, tPriceA, tPriceB)
	return []linecov.FaultCase{
		{Name: "setup keep: the gadget group's reset hook", Kind: linecov.KindSetup, Diffs: change("app/shop.rb", 33), Keep: []string{tGadget}},
		{Name: "setup exclude: the shared example's warm hook", Kind: linecov.KindSetup, Diffs: change("app/shop.rb", 34),
			Representative: sRep, Excluded: []string{sOther}},
		{Name: "method keep: ItemPage#price_text", Kind: linecov.KindMethod, Diffs: change("spec/pages/item_page.rb", 8), Keep: []string{tItem}},
		{Name: "method exclude: ResultsPage#titles, the gadget search keeps its later checks", Kind: linecov.KindMethod,
			Diffs: change("spec/pages/results_page.rb", 8), Keep: []string{tGadget}, Representative: sRep, Excluded: []string{sOther}},
		{Name: "step keep: the gadget cart's own check", Kind: linecov.KindStep, Diffs: change("spec/features/cart_spec.rb", 22), Keep: []string{tCartG}},
		{Name: "step exclude: the shared add_to_cart step", Kind: linecov.KindStep, Diffs: change("spec/support/steps/shop_steps.rb", 11),
			Keep: []string{tCartG}, Representative: cRep, Excluded: []string{cOther}},
		{Name: "step exclude: the shared example body", Kind: linecov.KindStep, Diffs: change("spec/features/search_spec.rb", 9),
			Representative: sRep, Excluded: []string{sOther}},
		{Name: "test keep: the gadget price example", Kind: linecov.KindTest, Diffs: change("spec/unit/pricing_spec.rb", 21), Keep: []string{tPriceGad}},
		{Name: "test exclude: the shared widget price example", Kind: linecov.KindTest, Diffs: change("spec/unit/pricing_spec.rb", 7),
			Representative: pRep, Excluded: []string{pOther}},
	}
}

// runSegMatrix runs the matrix and every negative control over ix.
func runSegMatrix(t *testing.T, ix *linecov.Index) {
	t.Helper()
	cases := segFaultCases(ix)
	var results []linecov.FaultResult
	for _, c := range cases {
		r := SegmentFault(ix, c)
		if !r.Pass {
			t.Errorf("%s: %s", c.Name, r.Detail)
		}
		results = append(results, r)
	}
	matrix := linecov.FaultMatrix(cases, results)
	for _, k := range linecov.SegmentKinds {
		if !matrix[k] {
			t.Errorf("kind %s not qualified", k)
		}
	}
	// Negative controls.
	bad := cases[1]
	bad.Representative, bad.Excluded = bad.Excluded[0], []string{bad.Representative}
	if SegmentFault(ix, bad).Pass {
		t.Error("a wrong representative passed")
	}
	bad = cases[3]
	bad.Excluded = append(bad.Excluded, tGadget)
	if SegmentFault(ix, bad).Pass {
		t.Error("excluding the gadget search, whose later checks differ (condition 4), passed")
	}
	bad = cases[0]
	bad.Keep = []string{tSearchA}
	if SegmentFault(ix, bad).Pass {
		t.Error("an unreached keep passed")
	}
	for _, i := range []int{0, 1} {
		bad = cases[i]
		bad.Kind = linecov.KindMethod
		if SegmentFault(ix, bad).Pass {
			t.Errorf("%s relabelled as a method case passed", bad.Name)
		}
	}
	bad = cases[2]
	bad.Keep = nil
	if SegmentFault(ix, bad).Pass {
		t.Error("a keep case naming no test passed")
	}
	// A data-dependent step: HomePage#search_for runs the same lines for
	// both searches here, but the gadget search's later checks differ, so
	// a change to it keeps the gadget search and excludes only within the
	// shared class.
	r := SegmentFault(ix, linecov.FaultCase{Name: "search_for", Kind: linecov.KindMethod, Diffs: change("spec/pages/home_page.rb", 9),
		Keep: []string{tGadget}, Representative: cases[1].Representative, Excluded: cases[1].Excluded})
	if !r.Pass {
		t.Errorf("search_for: %s", r.Detail)
	}
	// A line no example ran, and a file the index does not record, keep
	// the static plan.
	for _, d := range [][]linecov.FileDiff{change("app/shop.rb", 1), change("Gemfile", 1),
		{{Path: segPrefix + "/app/new.rb", NewOID: "x"}}} {
		if e := SegmentEffect(ix, d); e.Fallback == "" {
			t.Errorf("%v: no fallback", d)
		}
	}
}

// MCJ-V0-044, MCJ-V0-014: the fault-injection matrix on the recorded output
// of the collector, with negative controls; an incomplete record or an
// unbound segment excludes nothing.
func TestMCJV0044FaultMatrixOnCollectorOutput(t *testing.T) {
	ix, _ := buildRecorded(t, nil)
	runSegMatrix(t, ix)

	exclude := segFaultCases(ix)[8]
	for name, mutate := range map[string]func(*SegmentReport){
		"incomplete": func(r *SegmentReport) {
			for i := range r.Examples {
				if strings.HasPrefix(r.Examples[i].ID, "./spec/unit/pricing_spec.rb[") {
					r.Examples[i].Incomplete = []string{ReasonSubprocess}
				}
			}
		},
		"unbound": func(r *SegmentReport) {
			for i := range r.Examples {
				for j := range r.Examples[i].Segments {
					if r.Examples[i].Segments[j].Kind == "test" {
						r.Examples[i].Segments[j].AnchorPath = "spec/unit/moved_spec.rb"
					}
				}
			}
		},
		"residual": func(r *SegmentReport) {
			for i := range r.Examples {
				if r.Examples[i].ID == "./spec/unit/pricing_spec.rb[2:1]" {
					r.Examples[i].Residual = map[string][][2]int64{"app/shop.rb": {{59, 1}}}
				}
			}
		},
	} {
		mix, _ := buildRecorded(t, mutate)
		if r := SegmentFault(mix, exclude); r.Pass {
			t.Errorf("%s: the exclude case still passed", name)
		}
		keep := exclude
		keep.Representative, keep.Excluded = "", nil
		keep.Keep = []string{tPriceA, tPriceB}
		if r := SegmentFault(mix, keep); !r.Pass && !strings.Contains(r.Detail, "not reached through") {
			t.Errorf("%s: both tests not kept: %s", name, r.Detail)
		}
	}
}

// MCJ-V0-013: a LIVE run of the collector with Ruby and Bundler over a
// committed copy of the fixture, built at that commit and qualified by the
// same matrix. Skipped without ruby, bundle or the fixture's gems.
func TestMCJV0013LiveRubySegments(t *testing.T) {
	requireHost(t)
	for _, tool := range []string{"ruby", "bundle", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("MCJ-V0-013 live run needs %s on PATH", tool)
		}
	}
	repo := t.TempDir()
	project := filepath.Join(repo, segPrefix)
	if err := os.CopyFS(project, os.DirFS(segFixture)); err != nil {
		t.Fatal(err)
	}
	bindir, err := exec.Command("ruby", "-e", "print Gem.bindir").Output()
	if err != nil {
		t.Skipf("ruby unusable: %v", err)
	}
	env := SetEnv(os.Environ(), "PATH", string(bindir)+string(os.PathListSeparator)+os.Getenv("PATH"))
	env = SetEnv(env, "BUNDLE_GEMFILE", filepath.Join(project, "Gemfile"))
	check := exec.Command("bundle", "check")
	check.Dir, check.Env = project, env
	if out, err := check.CombinedOutput(); err != nil {
		t.Skipf("MCJ-V0-013 live run needs the fixture's gems (bundle install in %s): %s", segFixture, out)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return string(out)
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".corvint/\nshop/.corvint/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "fixture")
	commit := strings.TrimSpace(git("rev-parse", "HEAD"))
	oids := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(git("ls-tree", "-r", "HEAD")), "\n") {
		meta, p, _ := strings.Cut(line, "\t")
		f := strings.Fields(meta)
		oids[p] = f[2]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	rep, raw, err := RunSegments(ctx, SegmentConfig{Dir: project, Env: env})
	if err != nil {
		t.Fatal(err)
	}
	if st := strings.TrimSpace(git("status", "--porcelain")); st != "" {
		t.Fatalf("the collector run left the worktree dirty: %s", st)
	}
	for _, e := range rep.Examples {
		if e.Status != "passed" || len(e.Incomplete) > 0 {
			t.Fatalf("%s: %s %v", e.ID, e.Status, e.Incomplete)
		}
	}
	o := segOptions(oids)
	o.Commit = commit
	o.Runner = "ruby " + rep.Ruby + " rspec " + rep.RSpec + " " + SegmentsFormat
	ix, err := BuildSegmentIndex(rep, o)
	if err != nil {
		t.Fatal(err)
	}
	// The live index has the recorded structure: same tests, segments and
	// reaches; only times differ.
	recorded := readRecorded(t)
	if got, want := shape(rep), shape(recorded); !reflect.DeepEqual(got, want) && os.Getenv("CORVINT_RECORD_RUBY_SEGMENTS") == "" {
		t.Errorf("live segments differ from %s:\n got %v\nwant %v", segRecorded, got, want)
	}
	// The live matrix runs against blob-pinned diffs at the live commit.
	live := map[string]string{}
	for p, oid := range oids {
		live[p] = oid
	}
	for _, c := range segFaultCases(ix) {
		for i := range c.Diffs {
			c.Diffs[i].OldOID = live[c.Diffs[i].Path]
		}
		if r := SegmentFault(ix, c); !r.Pass {
			t.Errorf("live %s: %s", c.Name, r.Detail)
		}
	}
	// Incomplete records: a child process and a non-rack_test driver.
	extra := "require_relative '../spec_helper'\n\nRSpec.describe 'uninstrumented' do\n" +
		"  it 'spawns a child' do\n    expect(system('true')).to be(true)\n  end\n\n" +
		"  it 'drives another driver', type: :feature do\n" +
		"    Capybara.register_driver(:other) { |app| Capybara::RackTest::Driver.new(app) }\n" +
		"    Capybara.current_driver = :other\n    visit '/'\n  end\nend\n"
	if err := os.MkdirAll(filepath.Join(project, "spec/extra"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "spec/extra/uninstrumented_spec.rb"), []byte(extra), 0o600); err != nil {
		t.Fatal(err)
	}
	inc, _, err := RunSegments(ctx, SegmentConfig{Dir: project, Env: env, Specs: []string{"spec/extra/uninstrumented_spec.rb"}})
	if err != nil {
		t.Fatal(err)
	}
	wantInc := map[string]string{"./spec/extra/uninstrumented_spec.rb[1:1]": ReasonSubprocess, "./spec/extra/uninstrumented_spec.rb[1:2]": ReasonDriverOutOfProcess}
	if len(inc.Examples) != 2 {
		t.Fatalf("incomplete run: %d examples", len(inc.Examples))
	}
	for _, e := range inc.Examples {
		if e.Status != "passed" || !reflect.DeepEqual(e.Incomplete, []string{wantInc[e.ID]}) {
			t.Errorf("%s: %s %v, want incomplete %s", e.ID, e.Status, e.Incomplete, wantInc[e.ID])
		}
	}
	t.Logf("live: ruby %s rspec %s, %d examples, %d segments, commit %s", rep.Ruby, rep.RSpec, len(rep.Examples), len(ix.Segments), commit)
	if os.Getenv("CORVINT_RECORD_RUBY_SEGMENTS") != "" {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		m["root"] = "/project"
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", " ")
		if err := enc.Encode(m); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(segRecorded, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// shape is a report without times or roots: the identities, kinds and
// reaches a rerun must reproduce.
func shape(r SegmentReport) []string {
	var out []string
	for _, e := range r.Examples {
		out = append(out, e.ID+" "+e.Status+" "+e.Type+" "+e.DeclaredLevel)
		for _, s := range e.Segments {
			b, _ := json.Marshal(s.Lines)
			out = append(out, "  "+s.Kind+" "+s.ID+" "+string(b))
		}
		b, _ := json.Marshal(e.Residual)
		out = append(out, "  residual "+string(b))
	}
	return out
}
