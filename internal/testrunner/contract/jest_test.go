// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"errors"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// supportedJest is a setup Unsupported admits: Jest 29 and an empty config,
// whose test files' source reads as one plain test.
func supportedJest(config string) *JestSetup {
	return &JestSetup{Version: "29.7.0", ConfigName: config, Config: []byte("module.exports = {}\n"), Source: plainSource}
}

// plainSource reads every test file as one plain test, naming no describe
// block Jest renames.
func plainSource(string) ([]byte, error) {
	return []byte("test('adds', () => {});\n"), nil
}

func jestRequest(root string, listed []string, tests ...string) Request {
	return Request{
		Instance: Instance{Runner: testid.Jest, Root: root, Config: root + "/jest.config.js"},
		Full:     []string{"npx", "jest"}, Report: "/private/r.json", Tier: TierLine,
		Tests: tests, Listed: listed, Jest: supportedJest(root + "/jest.config.js"),
	}
}

// pattern is the -t value of argv.
func pattern(t *testing.T, argv []string) string {
	t.Helper()
	index := slices.Index(argv, "-t")
	if index < 0 || index+1 >= len(argv) {
		t.Fatalf("no -t pattern in %q", argv)
	}
	return argv[index+1]
}

// JST-V1-003: selected tests pass their files by literal path and one -t
// pattern anchored at both ends over their full names (ancestorTitles and
// title joined by spaces), every regular expression character escaped.
// The pattern, compiled as Jest compiles it (case-insensitive), matches
// each selected full name and nothing else listed.
func TestJSTV1003NamePatternIsAnchoredAndEscaped(t *testing.T) {
	listed := []string{
		"jest:web/src/a.test.ts::cart::adds item",
		"jest:web/src/a.test.ts::handles (a+b)",
		"jest:web/src/a.test.ts::handles aab",
		"jest:web/src/a.test.ts::cart::adds item twice",
		"jest:web/src/b.test.ts::x",
	}
	got := command(t, jestRequest("web", listed, "jest:web/src/a.test.ts::cart::adds item", "jest:web/src/a.test.ts::handles (a+b)"))
	want := []string{"npx", "jest", "--json", "--outputFile=/private/r.json", "--runTestsByPath", "src/a.test.ts", "-t", `^(?:cart adds item|handles \(a\+b\))$`}
	if !slices.Equal(got.Argv, want) || !got.Exact || got.Dir != "web" || got.Tier != TierLine {
		t.Fatalf("command %+v\nwant argv %q", got, want)
	}
	compiled := regexp.MustCompile("(?i)" + pattern(t, got.Argv))
	for name, match := range map[string]bool{
		"cart adds item": true, "handles (a+b)": true, "HANDLES (A+B)": true,
		"handles aab": false, "cart adds item twice": false, "x cart adds item": false, "handles (a+b) ": false,
	} {
		if compiled.MatchString(name) != match {
			t.Errorf("pattern %s matches %q = %v, want %v", compiled, name, !match, match)
		}
	}
}

// JST-V1-003: exact is false when an unselected listed test of a selected
// file has a selected full name: the same name under other titles that
// join to it, a case variant (Jest's pattern ignores case), a duplicate
// suffix, a file listed only by file, or no listing at all. A whole-file
// selection mixed with tests passes no pattern.
func TestJSTV1003DuplicateNamesAreNotExact(t *testing.T) {
	cases := []struct {
		name   string
		listed []string
		tests  []string
		reason string
	}{
		{"titles that join to the same name", []string{"jest:a.test.js::a b::c", "jest:a.test.js::a::b c"}, []string{"jest:a.test.js::a b::c"}, "also matches jest:a.test.js::a::b c"},
		{"case variant", []string{"jest:a.test.js::Adds", "jest:a.test.js::adds"}, []string{"jest:a.test.js::adds"}, "also matches jest:a.test.js::Adds"},
		{"duplicate suffix", []string{"jest:a.test.js::adds", "jest:a.test.js::adds::#2"}, []string{"jest:a.test.js::adds::#2"}, "duplicate-test-id"},
		{"unsuffixed duplicate member", []string{"jest:a.test.js::adds", "jest:a.test.js::adds::#2", "jest:a.test.js::other"}, []string{"jest:a.test.js::adds"}, "duplicate-test-id"},
		{"file listed by file", []string{"jest:a.test.js"}, []string{"jest:a.test.js::adds"}, "only by file"},
		{"no listing", nil, []string{"jest:a.test.js::adds"}, "no inventory listing"},
		{"mixed whole file and test", []string{"jest:a.test.js::adds", "jest:b.test.js::x"}, []string{"jest:a.test.js::adds", "jest:b.test.js"}, "mixes whole files and tests"},
	}
	for _, c := range cases {
		got := command(t, jestRequest(".", c.listed, c.tests...))
		if got.Exact || !strings.Contains(got.Reason, c.reason) {
			t.Errorf("%s: %+v, want not exact with %q", c.name, got, c.reason)
		}
	}
	// The other file's same name does not widen: -t runs only in the
	// selected files.
	if got := command(t, jestRequest(".", []string{"jest:a.test.js::adds", "jest:b.test.js::adds"}, "jest:a.test.js::adds")); !got.Exact {
		t.Fatalf("unselected file widened: %+v", got)
	}
	// A duplicate group member, suffixed or not, runs its whole file
	// (CTI-V1-010, jst review P2): no -t pattern could keep only it, and the
	// file's other tests are not filtered out.
	for _, tests := range [][]string{{"jest:a.test.js::adds::#2"}, {"jest:a.test.js::adds"}} {
		got := command(t, jestRequest(".", []string{"jest:a.test.js::adds", "jest:a.test.js::adds::#2", "jest:a.test.js::other"}, tests...))
		if slices.Contains(got.Argv, "-t") || got.Exact || got.Tier != TierFile || !strings.HasPrefix(got.Reason, "duplicate-test-id: ") {
			t.Fatalf("duplicate %q: %+v", tests, got)
		}
	}
	// A whole file passes the file only.
	if got := command(t, jestRequest(".", nil, "jest:a.test.js")); slices.Contains(got.Argv, "-t") || !got.Exact {
		t.Fatalf("whole file: %+v", got)
	}
}

// JST-V1-003, jst review P1: for describe('ROOT_DESCRIBE_BLOCK', () =>
// test('fails', ...)) Jest drops the ancestor from its --json report, so
// the canonical ID is jest:f.test.js::fails, but keeps it in the full name
// -t matches, so `^(?:fails)$` would skip the selected test and the report
// would show it pending (skip), neither failed nor incomplete. The filter
// is used only when the file's source proves it lossless; this file names
// the block, so the selection widens to the whole file, as it does when
// the source was not read or cannot be read.
func TestJSTV1003RootDescribeBlockWidensToTheFile(t *testing.T) {
	source := "describe('ROOT_DESCRIBE_BLOCK', () => {\n  test('fails', () => { throw new Error('x'); });\n});\n"
	request := jestRequest(".", []string{"jest:f.test.js::fails", "jest:f.test.js::other"}, "jest:f.test.js::fails")
	request.Jest.Source = func(rel string) ([]byte, error) {
		if rel != "f.test.js" {
			t.Fatalf("read %q", rel)
		}
		return []byte(source), nil
	}
	want := []string{"npx", "jest", "--json", "--outputFile=/private/r.json", "--runTestsByPath", "f.test.js"}
	got := command(t, request)
	if !slices.Equal(got.Argv, want) || got.Exact || got.Tier != TierFile || !strings.Contains(got.Reason, "names ROOT_DESCRIBE_BLOCK") {
		t.Fatalf("command %+v\nwant argv %q", got, want)
	}
	// The whole file's report answers the selected test as failed.
	r, _ := For(testid.Jest)
	results, err := r.Parse(ParseRequest{Instance: Instance{Runner: testid.Jest, Root: "."}, Root: "/repo", Report: []byte(`{"success":false,"testResults":[{"name":"/repo/f.test.js","status":"failed","assertionResults":[` +
		`{"ancestorTitles":[],"title":"fails","status":"failed","duration":3},{"ancestorTitles":[],"title":"other","status":"passed","duration":1}]}]}`)})
	run := ReconcileParsed(request.Tests, results, err)
	if !run.Failed || len(run.NotRun) != 0 || !slices.ContainsFunc(run.Results, func(result Result) bool { return result.ID == "jest:f.test.js::fails" && result.Outcome == OutcomeFail }) {
		t.Fatalf("run %+v", run)
	}
	for name, read := range map[string]func(string) ([]byte, error){
		"source not read":   nil,
		"source unreadable": func(string) ([]byte, error) { return nil, errors.New("gone") },
	} {
		request.Jest.Source = read
		if got := command(t, request); slices.Contains(got.Argv, "-t") || got.Exact || got.Tier != TierFile {
			t.Fatalf("%s: %+v", name, got)
		}
	}
}

// JST-V1-004: the Jest --json report maps each assertion result, nested
// describe blocks included, to its canonical ID with its outcome and its
// duration in milliseconds. The fixture is hand-written in the
// AggregatedResult shape Jest's --json writes (@jest/test-result,
// https://jestjs.io/docs/cli#--json); no Jest install was run.
func TestJSTV1004ReportMapsNestedDescribesWithDurations(t *testing.T) {
	report, err := os.ReadFile("testdata/jest-nested.json")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := For(testid.Jest)
	got, err := r.Parse(ParseRequest{Instance: Instance{Runner: testid.Jest, Root: "apps/web"}, Root: "/repo", Report: report})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{
		{ID: "jest:apps/web/src/a.test.ts::cart::adds item", Outcome: OutcomePass, DurationMillis: 14, Granularity: testid.GranularityTest},
		{ID: "jest:apps/web/src/a.test.ts::cart::totals::rounds (half-up)", Outcome: OutcomeFail, DurationMillis: 3, Granularity: testid.GranularityTest},
		{ID: "jest:apps/web/src/a.test.ts::cart::totals::skips", Outcome: OutcomeSkip, Granularity: testid.GranularityTest, DurationMissing: true},
		{ID: "jest:apps/web/src/a.test.ts::top level", Outcome: OutcomePass, DurationMillis: 1, Granularity: testid.GranularityTest},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("results\n got %+v\nwant %+v", got, want)
	}
}

// JST-V1-005: a selected test the report does not name is NOT_RUN and the
// run INCOMPLETE; a missing (empty) or unparsable report makes every
// selected test NOT_RUN and the run INCOMPLETE. Keeping Jest's exit code
// is the caller's: the contract runs no process.
func TestJSTV1005MissingResultsAndReportsAreIncomplete(t *testing.T) {
	r, _ := For(testid.Jest)
	selected := []string{"jest:a.test.js::adds", "jest:a.test.js::gone"}
	results, err := r.Parse(ParseRequest{Instance: Instance{Runner: testid.Jest, Root: "."}, Root: "/repo", Report: []byte(
		`{"success":true,"testResults":[{"name":"/repo/a.test.js","status":"passed","assertionResults":[{"ancestorTitles":[],"title":"adds","status":"passed","duration":2}]}]}`)})
	run := ReconcileParsed(selected, results, err)
	if !run.Incomplete || !slices.Equal(run.NotRun, []string{"jest:a.test.js::gone"}) || run.Failed {
		t.Fatalf("missing test: %+v", run)
	}
	for name, report := range map[string][]byte{"missing": nil, "unparsable": []byte(`{"testResults": [`)} {
		results, err := r.Parse(ParseRequest{Instance: Instance{Runner: testid.Jest, Root: "."}, Root: "/repo", Report: report})
		if err == nil {
			t.Fatalf("%s report parsed", name)
		}
		run := ReconcileParsed(selected, results, err)
		if !run.Incomplete || !slices.Equal(run.NotRun, selected) {
			t.Fatalf("%s report: %+v", name, run)
		}
	}
}

// JST-V1-006, JST-V1-001: with no declared command Jest's full command is
// `npx jest`, run from the instance root, and it is a compiled-in runner
// whose lister is the inventory's executing Jest lister.
func TestJSTV1006DefaultCommandFromTheInstanceRoot(t *testing.T) {
	r, ok := For(testid.Jest)
	if !ok || r.Name() != testid.Jest || r.Lister().Kind != testinventory.KindExecuting {
		t.Fatalf("jest runner %v %+v", ok, r)
	}
	instance := Instance{Runner: testid.Jest, Root: "apps/web", Config: "apps/web/jest.config.js"}
	argv := r.Default(instance, nil)
	if !slices.Equal(argv, []string{"npx", "jest"}) {
		t.Fatalf("default %q", argv)
	}
	got := command(t, Request{Instance: instance, Full: argv, Report: "/r.json"})
	if got.Dir != "apps/web" || got.Tier != TierFull || !got.Exact || !slices.Equal(got.Argv, []string{"npx", "jest", "--json", "--outputFile=/r.json"}) {
		t.Fatalf("full %+v", got)
	}
}

// JST-V1-007: two Jest roots are two instances, each owning only its own
// tests and run from its own root; run in full (as CORVINT_SELECTION=
// full:jest makes every Jest instance, TIV-V1-003's ForceFull), each runs
// its whole command with no selection.
func TestJSTV1007TwoRootsRunInFull(t *testing.T) {
	r, _ := For(testid.Jest)
	for _, root := range []string{"apps/a", "apps/b"} {
		instance := Instance{Runner: testid.Jest, Root: root, Config: root + "/jest.config.js"}
		got := command(t, Request{Instance: instance, Full: []string{"npx", "jest"}, Report: "/r.json"})
		if got.Dir != root || got.Tier != TierFull || slices.Contains(got.Argv, "--runTestsByPath") {
			t.Fatalf("%s full %+v", root, got)
		}
		other := "apps/b"
		if root == "apps/b" {
			other = "apps/a"
		}
		if _, err := r.Command(Request{Instance: instance, Full: []string{"npx", "jest"}, Tests: []string{"jest:" + other + "/x.test.js"}}); err == nil {
			t.Fatalf("%s ran a test of %s", root, other)
		}
	}
	verdict := testinventory.Verdict{Instances: []testinventory.InstanceVerdict{{Key: "jest@apps/a", Runner: testid.Jest, Root: "apps/a"}, {Key: "jest@apps/b", Runner: testid.Jest, Root: "apps/b"}}}
	verdict.ForceFull(testid.Jest)
	for _, instance := range verdict.Instances {
		if !instance.Full || instance.Reason != testinventory.ReasonSelectionFull {
			t.Fatalf("full:jest left %+v narrowed", instance)
		}
	}
}

// JST-V1-008: a setup Corvint cannot read or does not support runs the
// full command with jest-unsupported-config: no setup, an unknown or old
// version, an unreadable config, a custom testRunner, runner or
// testResultsProcessor (the custom reporter that rewrites the --json
// report). Jest's own runners and plain reporters narrow.
func TestJSTV1008UnsupportedSetupsRunInFull(t *testing.T) {
	setup := func(version, name, config string) *JestSetup {
		s := &JestSetup{Version: version, ConfigName: name}
		if config != "\x00" {
			s.Config = []byte(config)
		}
		return s
	}
	refused := map[string]*JestSetup{
		"not read":                    nil,
		"version unknown":             setup("", "jest.config.js", ""),
		"version 26":                  setup("26.6.3", "jest.config.js", ""),
		"config unreadable":           setup("29.7.0", "jest.config.js", "\x00"),
		"custom testRunner (js)":      setup("29.7.0", "jest.config.js", "module.exports = { testRunner: 'jest-light-runner' }\n"),
		"computed testRunner (js)":    setup("29.7.0", "jest.config.ts", "export default { testRunner: require.resolve('x') }\n"),
		"custom runner (js)":          setup("29.7.0", "jest.config.js", "module.exports = {\n  runner: 'jest-runner-eslint',\n}\n"),
		"testResultsProcessor (js)":   setup("29.7.0", "jest.config.js", "module.exports = { testResultsProcessor: './strip.js' }\n"),
		"custom testRunner (json)":    setup("29.7.0", "jest.config.json", `{"testRunner": "jest-light-runner"}`),
		"testResultsProcessor (json)": setup("29.7.0", "jest.config.json", `{"testResultsProcessor": "./strip.js"}`),
		"project by path (json)":      setup("29.7.0", "jest.config.json", `{"projects": ["<rootDir>/a"]}`),
		"package.json without jest":   setup("29.7.0", "package.json", `{"name": "x"}`),
		"package.json custom runner":  setup("29.7.0", "package.json", `{"jest": {"runner": "jest-runner-tsc"}}`),
		"unknown config file kind":    setup("29.7.0", "jest.config.yaml", "a: b\n"),
		// jst review P2: an imported or spread config is not resolved.
		"required base config (js)": setup("29.7.0", "jest.config.js", "module.exports = require('./base');\n"),
		"imported base config (ts)": setup("29.7.0", "jest.config.ts", "import base from './base';\nexport default { ...base };\n"),
		"spread base config (js)":   setup("29.7.0", "jest.config.js", "const base = {};\nmodule.exports = { ...base, verbose: true };\n"),
	}
	for name, s := range refused {
		request := jestRequest("web", nil, "jest:web/a.test.js::adds")
		request.Jest = s
		got := command(t, request)
		if got.Exact || got.Tier != TierFull || slices.Contains(got.Argv, "--runTestsByPath") || !strings.HasPrefix(got.Reason, ReasonJestUnsupported+": ") {
			t.Errorf("%s: %+v, want the full command with %s", name, got, ReasonJestUnsupported)
		}
	}
	admitted := map[string]*JestSetup{
		"jest-circus (js)":      setup("29.7.0", "jest.config.js", "module.exports = { testRunner: \"jest-circus/runner\", reporters: ['default', 'jest-junit'] }\n"),
		"jest-runner (js)":      setup("30.0.0-alpha.6", "jest.config.mjs", "export default { runner: 'jest-runner', preset: 'ts-jest' }\n"),
		"value naming a runner": setup("27.0.0", "jest.config.js", "module.exports = { preset: 'jest-runner-like' }\n"),
		"inline project (json)": setup("29.7.0", "jest.config.json", `{"projects": [{"displayName": "a", "testRunner": "jest-jasmine2"}]}`),
		"package.json jest":     setup("29.7.0", "package.json", `{"jest": {"testEnvironment": "node"}}`),
		"type-only import (ts)": setup("29.7.0", "jest.config.ts", "import type { Config } from 'jest';\nconst config: Config = { verbose: true };\nexport default config;\n"),
	}
	for name, s := range admitted {
		if cause := s.Unsupported(); cause != "" {
			t.Errorf("%s refused: %s", name, cause)
		}
	}
}

// JST-V1-008: JestSetupFrom reads the nearest node_modules/jest version at
// or above the instance root and the config, through the caller's reader.
func TestJSTV1008SetupFromReader(t *testing.T) {
	files := map[string]string{
		"node_modules/jest/package.json": `{"name":"jest","version":"29.7.0"}`,
		"apps/web/jest.config.js":        "module.exports = {}\n",
	}
	read := func(rel string) ([]byte, error) {
		if text, ok := files[rel]; ok {
			return []byte(text), nil
		}
		return nil, errors.New("absent")
	}
	got := JestSetupFrom(Instance{Runner: testid.Jest, Root: "apps/web", Config: "apps/web/jest.config.js"}, read)
	if got.Version != "29.7.0" || string(got.Config) != files["apps/web/jest.config.js"] || got.Unsupported() != "" {
		t.Fatalf("setup %+v (%s)", got, got.Unsupported())
	}
	delete(files, "node_modules/jest/package.json")
	if got := JestSetupFrom(Instance{Runner: testid.Jest, Root: "apps/web", Config: "apps/web/jest.config.js"}, read); !strings.Contains(got.Unsupported(), "version cannot be read") {
		t.Fatalf("no jest install: %q", got.Unsupported())
	}
}

// RNC-V1-016: Jest takes no argument file, so a -t pattern over the
// argument limit widens the selection to its files (tier file, not exact,
// reason argv-limit), and file paths over the limit run the full command;
// no selected test is dropped.
func TestRNCV1016JestArgvLimitWidens(t *testing.T) {
	long := strings.Repeat("n", MaxArgBytes/4)
	var tests, listed []string
	for index := range 5 {
		id := "jest:a.test.js::" + long + string(rune('a'+index))
		tests, listed = append(tests, id), append(listed, id)
	}
	got := command(t, jestRequest(".", listed, tests...))
	want := []string{"npx", "jest", "--json", "--outputFile=/private/r.json", "--runTestsByPath", "a.test.js"}
	if !slices.Equal(got.Argv, want) || got.Exact || got.Tier != TierFile || !strings.HasPrefix(got.Reason, ReasonArgvLimit+": ") {
		t.Fatalf("over-long pattern: %+v", got)
	}
	var files []string
	for index := range 3000 {
		files = append(files, "jest:"+strings.Repeat("d", 100)+"/"+strconv.Itoa(index)+".test.js")
	}
	got = command(t, jestRequest(".", nil, files...))
	if got.Tier != TierFull || got.Exact || slices.Contains(got.Argv, "--runTestsByPath") || !strings.HasPrefix(got.Reason, ReasonArgvLimit+": ") {
		t.Fatalf("over-long paths: tier %s exact %v reason %q argc %d", got.Tier, got.Exact, got.Reason, len(got.Argv))
	}
	if !ArgvFits([]string{"npx", strings.Repeat("x", MaxArgBytes)}) || ArgvFits([]string{strings.Repeat("x", MaxArgBytes+1)}) {
		t.Fatal("ArgvFits per-argument bound")
	}
}
