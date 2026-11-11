package sessiontest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

const (
	providerSpec  = "typescript:playwright:e2e/login.spec.ts"
	providerSpec2 = "typescript:playwright:e2e/cart.spec.ts"
	providerUnitA = "typescript:vitest:web/src/a.test.ts"
)

var (
	playwrightArgv = []string{"corvint-js-test-provider", "e2e", "--test-arg", "npx", "--test-arg", "playwright", "--test-arg", "test"}
	vitestArgv     = []string{"corvint-js-test-provider", "unit", "--dir", "web"}
	typescriptArgv = LanguageCommand{Language: "typescript", Argv: []string{"make", "ts-test"}}
)

// withProviders is parsedDeclared with both providers declared.
func withProviders() Declared {
	declared := parsedDeclared()
	declared.Providers = []ProviderCommand{
		{Argv: playwrightArgv, Runner: ProviderRunnerPlaywright},
		{Argv: vitestArgv, Runner: ProviderRunnerVitest},
	}
	return declared
}

// otherUnits lists every unit a Go package does not own, as "adapter unit".
func otherUnits(m Manifest) []string {
	ids := []string{}
	for _, unit := range m.Units {
		if !strings.HasPrefix(unit.Unit, "go:") {
			ids = append(ids, fmt.Sprintf("%s %d %s %s", unit.Adapter, unit.Shard, unit.Unit, unit.Slice))
		}
	}
	return ids
}

// STR-V0-8701: the optional providers member is read strictly, sorted by
// runner, and absent it changes neither the declaration nor its digest.
func TestSTRV08701DeclaredProviders(t *testing.T) {
	declaration := func(providers string) []byte {
		body := `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}`
		if providers != "" {
			body += `, "providers": ` + providers
		}
		return []byte(body + "}")
	}
	got := ParseDeclared(declaration(`{"vitest": ["corvint-js-test-provider", "unit", "--dir", "web"], "playwright": ["/opt/bin/corvint-js-test-provider", "e2e", "--test-arg", "npx", "--test-arg", "playwright", "--test-arg", "test"]}`))
	want := []ProviderCommand{
		{Argv: []string{"/opt/bin/corvint-js-test-provider", "e2e", "--test-arg", "npx", "--test-arg", "playwright", "--test-arg", "test"}, Runner: "playwright"},
		{Argv: []string{"corvint-js-test-provider", "unit", "--dir", "web"}, Runner: "vitest"},
	}
	if got.Unreadable != "" || !reflect.DeepEqual(got.Providers, want) {
		t.Fatalf("declared providers %#v (unreadable %q), want %#v", got.Providers, got.Unreadable, want)
	}
	if argv, ok := got.Provider("vitest"); !ok || !reflect.DeepEqual(argv, want[1].Argv) {
		t.Fatalf("Provider(vitest) = %v, %v", argv, ok)
	}
	if argv, ok := got.Provider("jest"); ok || argv != nil {
		t.Fatalf("Provider(jest) = %v, %v, want none", argv, ok)
	}

	// A declaration without the member keeps its canonical bytes, and so its
	// digest; an empty member is the same declaration.
	absent := ParseDeclared(declaration(""))
	encoded, err := gokernel.CanonicalJSON(absent)
	if err != nil {
		t.Fatal(err)
	}
	if absent.Unreadable != "" || bytes.Contains(encoded, []byte("providers")) {
		t.Fatalf("a declaration without providers encodes as %s (unreadable %q)", encoded, absent.Unreadable)
	}
	absentDigest, _ := Digest(absent)
	if emptyDigest, _ := Digest(ParseDeclared(declaration(`{}`))); emptyDigest != absentDigest {
		t.Fatalf("an empty providers member changes the digest: %s, want %s", emptyDigest, absentDigest)
	}
	if declaredDigest, _ := Digest(got); declaredDigest == absentDigest {
		t.Fatal("a declared provider does not change the declaration digest")
	}

	for name, test := range map[string]struct{ providers, reason string }{
		"unsupported runner": {`{"jest": ["corvint-js-test-provider", "unit"]}`, `providers has an unsupported runner "jest": want playwright or vitest`},
		"empty argv":         {`{"vitest": []}`, "providers.vitest: argv must start with corvint-js-test-provider unit"},
		"other executable":   {`{"vitest": ["npx", "unit"]}`, "providers.vitest: argv must start with corvint-js-test-provider unit"},
		"wrong subcommand":   {`{"playwright": ["corvint-js-test-provider", "unit"]}`, "providers.playwright: argv must start with corvint-js-test-provider e2e"},
		"word with space":    {`{"vitest": ["corvint-js-test-provider", "unit", "--dir", "my web"]}`, `providers.vitest: every word must be non-empty without whitespace or control characters, got "my web"`},
		"empty word":         {`{"playwright": ["corvint-js-test-provider", "e2e", ""]}`, `providers.playwright: every word must be non-empty without whitespace or control characters, got ""`},
		"watch flag":         {`{"vitest": ["corvint-js-test-provider", "unit", "--watch"]}`, "providers.vitest: a watch run never ends; declare the one-shot command"},
		"watch assignment":   {`{"playwright": ["corvint-js-test-provider", "e2e", "-watch=true"]}`, "providers.playwright: a watch run never ends; declare the one-shot command"},
		"flag terminator":    {`{"playwright": ["corvint-js-test-provider", "e2e", "--", "npx", "playwright", "test"]}`, "providers.playwright: a bare -- ends the provider's flags, so the selected files appended after it would be ignored; pass each command word as a flag value"},
		"terminator in dir":  {`{"vitest": ["corvint-js-test-provider", "unit", "--dir", "web", "--"]}`, "providers.vitest: a bare -- ends the provider's flags, so the selected files appended after it would be ignored; pass each command word as a flag value"},
		"dir without value":  {`{"vitest": ["corvint-js-test-provider", "unit", "--dir"]}`, "providers.vitest: --dir must be a repository-relative directory"},
		"absolute dir":       {`{"vitest": ["corvint-js-test-provider", "unit", "--dir", "/srv/web"]}`, "providers.vitest: --dir must be a repository-relative directory"},
		"dir leaves repo":    {`{"vitest": ["corvint-js-test-provider", "unit", "--dir=../web"]}`, "providers.vitest: --dir must be a repository-relative directory"},
	} {
		refused := ParseDeclared(declaration(test.providers))
		if refused.Unreadable != DeclarationPath+": "+test.reason {
			t.Errorf("%s: unreadable %q, want %q", name, refused.Unreadable, DeclarationPath+": "+test.reason)
		}
		if len(refused.Providers)+len(refused.Languages) != 0 {
			t.Errorf("%s: a refused declaration kept entries: %#v", name, refused)
		}
	}
	if refused := ParseDeclared(declaration(`{"vitest": "corvint-js-test-provider unit"}`)); refused.Unreadable == "" {
		t.Error("a providers argv that is not an array was read")
	}
	// A "--" that is one word of the server command is a flag value.
	if read := ParseDeclared(declaration(`{"playwright": ["corvint-js-test-provider", "e2e", "--server-arg", "npm", "--server-arg", "start", "--server-arg", "--", "--server-arg", "--port=3000"]}`)); read.Unreadable != "" || len(read.Providers) != 1 {
		t.Errorf("a -- that is a --server-arg value was refused: %q", read.Unreadable)
	}
}

// STR-V0-8702: a selected Vitest or Playwright file with a declared provider
// is one provider unit; anything else that names the language keeps its
// declared command, and nothing selected is dropped.
func TestSTRV08702BuildProviderUnits(t *testing.T) {
	undeclared := "typescript: no qualified adapter and " + DeclarationPath + " declares no typescript full command"
	frontier := Unknown{Reason: "LANGUAGE_FRONTIER", Detail: "typescript:e2e-runtime-dependency:e2e/login.spec.ts:3 page.goto"}
	for name, test := range map[string]struct {
		edit       func(*Input)
		scope      string
		units      []string
		checks     []string
		incomplete []string
	}{
		"playwright file runs through its provider": {
			edit:  func(in *Input) { in.Selected = []string{providerSpec} },
			scope: ScopeNarrowed,
			units: []string{"provider 0 " + providerSpec + " whole"},
		},
		"provider units follow Go and precede declared, in unit order": {
			edit: func(in *Input) {
				declare(in, rustArgv)
				in.Selected = []string{"rust:crate/a", providerUnitA, providerSpec, providerSpec2}
			},
			scope: ScopeNarrowed,
			units: []string{
				"provider 0 " + providerSpec2 + " whole", "provider 0 " + providerSpec + " whole",
				"provider 0 " + providerUnitA + " whole", "declared 0 declared:rust whole",
			},
		},
		"a provider-only selection is NARROWED, never NOTHING": {
			edit: func(in *Input) {
				in.GoSelection = "NOTHING no Go package is affected"
				in.Selected = []string{providerSpec}
			},
			scope: ScopeNarrowed,
			units: []string{"provider 0 " + providerSpec + " whole"},
		},
		"no declared provider keeps the declared command": {
			edit: func(in *Input) {
				in.Declared = parsedDeclared()
				declare(in, typescriptArgv)
				in.Selected = []string{providerSpec, providerUnitA}
			},
			scope: ScopeNarrowed,
			units: []string{"declared 0 declared:typescript whole"},
		},
		"no declared provider and no declared command is INCOMPLETE": {
			edit: func(in *Input) {
				in.Declared = parsedDeclared()
				in.Selected = []string{providerSpec}
			},
			scope:      ScopeNarrowed,
			units:      []string{},
			incomplete: []string{undeclared},
		},
		"another runner keeps the declared command": {
			edit: func(in *Input) {
				declare(in, typescriptArgv)
				in.Selected = []string{"typescript:jest:web/src/b.test.ts", providerSpec}
			},
			scope: ScopeNarrowed,
			units: []string{"provider 0 " + providerSpec + " whole", "declared 0 declared:typescript whole"},
		},
		"a Vitest file outside the declared directory keeps the declared command": {
			edit: func(in *Input) {
				declare(in, typescriptArgv)
				in.Selected = []string{"typescript:vitest:api/src/c.test.ts", "typescript:vitest:webapp/d.test.ts"}
			},
			scope: ScopeNarrowed,
			units: []string{"declared 0 declared:typescript whole"},
		},
		"a path that reads as an option or leaves the tree is not handed to the provider": {
			edit: func(in *Input) {
				in.Selected = []string{
					"typescript:playwright:--reporter=null", "typescript:playwright:../e2e/x.spec.ts",
					"typescript:playwright:e2e/../x.spec.ts", "typescript:playwright:/abs/x.spec.ts",
					`typescript:playwright:e2e\x.spec.ts`, "typescript:playwright:e2e/x:y.spec.ts",
					"typescript:playwright:e2e/a\nb.spec.ts",
				}
			},
			scope:      ScopeNarrowed,
			units:      []string{},
			incomplete: []string{undeclared},
		},
		"the e2e-runtime-dependency frontier still adds the declared suite": {
			edit: func(in *Input) {
				declare(in, typescriptArgv)
				in.Selected = []string{providerSpec}
				in.Unknown = []Unknown{frontier}
			},
			scope: ScopeNarrowed,
			units: []string{"provider 0 " + providerSpec + " whole", "declared 0 declared:typescript whole"},
		},
		"FULL keeps the provider unit beside every declared command": {
			edit: func(in *Input) {
				declare(in, typescriptArgv)
				in.GoSelection = "FALLBACK a deleted file has no owner"
				in.Selected = []string{providerSpec}
			},
			scope: ScopeFull,
			units: []string{"provider 0 " + providerSpec + " whole", "declared 0 declared:typescript whole"},
		},
		"the ci profile plans no provider unit": {
			edit: func(in *Input) {
				declare(in, typescriptArgv)
				in.RunnerProfile = ProfileCI
				in.Selected = []string{providerSpec, providerUnitA}
			},
			scope:  ScopeNarrowed,
			units:  []string{},
			checks: []string{"make ts-test"},
		},
		"selection incomplete": {
			edit: func(in *Input) {
				in.Selected = []string{providerSpec}
				in.SelectionIncomplete = []string{`advice.test_selection is "blocked" (no evidence); its Playwright tests cannot be selected`}
			},
			scope:      ScopeNarrowed,
			units:      []string{"provider 0 " + providerSpec + " whole"},
			incomplete: []string{`advice.test_selection is "blocked" (no evidence); its Playwright tests cannot be selected`},
		},
	} {
		t.Run(name, func(t *testing.T) {
			in := fixtureInput()
			in.Declared = withProviders()
			test.edit(&in)
			m, err := Build(in)
			if err != nil {
				t.Fatal(err)
			}
			if m.Scope != test.scope {
				t.Errorf("scope %s (%s), want %s", m.Scope, m.Reason, test.scope)
			}
			if got := otherUnits(m); !reflect.DeepEqual(got, test.units) {
				t.Errorf("units %q, want %q", got, test.units)
			}
			if !reflect.DeepEqual(m.Checks, nonNil(test.checks)) {
				t.Errorf("checks %q, want %q", m.Checks, test.checks)
			}
			if !reflect.DeepEqual(m.Incomplete, nonNil(test.incomplete)) {
				t.Errorf("incomplete %q, want %q", m.Incomplete, test.incomplete)
			}
			for index, unit := range m.Units {
				if index != 0 && adapterOrder(m.Units[index-1].Adapter) > adapterOrder(unit.Adapter) {
					t.Errorf("unit %s (%s) follows a %s unit", unit.Unit, unit.Adapter, m.Units[index-1].Adapter)
				}
			}
		})
	}
}

// STR-V0-8703: one invocation per runner after the Go and declared ones, in
// runner-name order, naming each selected file; Playwright also gets each
// file's escaped filter.
func TestSTRV08703ProviderInvocations(t *testing.T) {
	in := fixtureInput()
	in.Declared = withProviders()
	declare(&in, rustArgv)
	in.Selected = []string{"rust:crate/a", providerUnitA, "typescript:playwright:e2e/[id]/cart.spec.ts", providerSpec}
	m, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	planned := Invocations(m, in.Declared, 0, t.TempDir(), "")
	if len(planned) != 4 {
		t.Fatalf("%d invocations, want go, declared, playwright, vitest: %+v", len(planned), planned)
	}
	adapters := []string{}
	for _, invocation := range planned {
		adapters = append(adapters, invocation.Adapter)
	}
	if want := []string{AdapterGo, AdapterDeclared, AdapterProvider, AdapterProvider}; !reflect.DeepEqual(adapters, want) {
		t.Fatalf("invocation adapters %q, want %q", adapters, want)
	}
	playwright, vitest := planned[2], planned[3]
	wantPlaywright := []string{
		"corvint-js-test-provider", "e2e", "--test-arg", "npx", "--test-arg", "playwright", "--test-arg", "test",
		"--test-file", "e2e/[id]/cart.spec.ts", "--test-file", "e2e/login.spec.ts",
		"--test-arg", `e2e/\[id\]/cart\.spec\.ts`, "--test-arg", `e2e/login\.spec\.ts`,
	}
	if !reflect.DeepEqual(playwright.Argv, wantPlaywright) {
		t.Errorf("playwright argv %q, want %q", playwright.Argv, wantPlaywright)
	}
	wantVitest := []string{"corvint-js-test-provider", "unit", "--dir", "web", "--test-file", "web/src/a.test.ts"}
	if !reflect.DeepEqual(vitest.Argv, wantVitest) {
		t.Errorf("vitest argv %q, want %q", vitest.Argv, wantVitest)
	}
	for _, invocation := range []Planned{playwright, vitest} {
		if invocation.Dir != "" || len(invocation.Env) != 0 {
			t.Errorf("provider invocation %q has dir %q and env %q, want the repository root and no variable", invocation.Argv, invocation.Dir, invocation.Env)
		}
		if key := runnerKey(invocation); key != "provider corvint-js-test-provider" {
			t.Errorf("runner key %q", key)
		}
	}
	if len(playwright.Units) != 2 || len(vitest.Units) != 1 || vitest.Units[0].Unit != providerUnitA {
		t.Errorf("provider units %+v and %+v", playwright.Units, vitest.Units)
	}
	// The declared argv is not aliased: a second derivation is identical.
	if again := Invocations(m, in.Declared, 0, t.TempDir(), ""); !reflect.DeepEqual(again[2].Argv, wantPlaywright) || len(in.Declared.Providers[0].Argv) != len(playwrightArgv) {
		t.Errorf("a second derivation changed the argv: %q", again[2].Argv)
	}

	// A unit whose runner the declaration no longer names has no argv, and
	// runner-name order holds whatever the manifest's unit order is.
	reordered := Manifest{ShardCount: 1, Units: []Unit{
		{Adapter: AdapterProvider, Slice: SliceWhole, Unit: providerUnitA},
		{Adapter: AdapterProvider, Slice: SliceWhole, Unit: providerSpec},
		{Adapter: AdapterProvider, Shard: 1, Slice: SliceWhole, Unit: providerSpec2},
	}}
	onlyVitest := parsedDeclared()
	onlyVitest.Providers = []ProviderCommand{{Argv: vitestArgv, Runner: ProviderRunnerVitest}}
	got := providerInvocations(reordered, onlyVitest, 0)
	if len(got) != 2 || got[0].Units[0].Unit != providerSpec || len(got[0].Argv) != 0 || got[0].Argv == nil || !reflect.DeepEqual(got[1].Argv, wantVitest) {
		t.Fatalf("provider invocations %+v", got)
	}
	if key := runnerKey(got[0]); key != AdapterProvider {
		t.Errorf("undeclared runner key %q, want %q", key, AdapterProvider)
	}
}

// providerRun reads one provider document as launch does.
func providerRun(units []string, exit int, document string) ([]Outcome, []string, string) {
	planned := Planned{Adapter: AdapterProvider}
	for _, unit := range units {
		planned.Units = append(planned.Units, Unit{Adapter: AdapterProvider, Slice: SliceWhole, Unit: unit})
	}
	var capture providerCapture
	_, _ = capture.Write([]byte(document))
	var out bytes.Buffer
	outcomes, reasons := providerOutcomes(planned, exit, &capture, &out)
	return outcomes, reasons, out.String()
}

func outcomeNames(outcomes []Outcome) []string {
	names := []string{}
	for _, outcome := range outcomes {
		names = append(names, outcome.Outcome)
	}
	return names
}

func providerTest(file, name, state string) string {
	quote := func(text string) string {
		encoded, _ := json.Marshal(text)
		return string(encoded)
	}
	anchor := ""
	if file != "" {
		anchor = `, "anchor": {"file": ` + quote(file) + `}`
	}
	return `{"fullName": ` + quote(name) + `, "state": ` + quote(state) + anchor + `}`
}

func providerReceipt(kind string, tests ...string) string {
	return fmt.Sprintf(`{"receipt": {"kind": %q, "tests": [%s]}}`, kind, strings.Join(tests, ", "))
}

// STR-V0-8704: the outcome of each unit of a counted provider run.
func TestSTRV08704ProviderOutcomes(t *testing.T) {
	two := []string{providerSpec, providerSpec2}
	for name, test := range map[string]struct {
		units    []string
		document string
		outcomes []string
		reasons  []string
		output   string
	}{
		"each Playwright file passes on its own test": {
			units: two,
			document: providerReceipt("e2e",
				providerTest("login.spec.ts", "login > signs in", "passed"),
				providerTest("/repo/e2e/cart.spec.ts", "cart > adds", "flaky")),
			outcomes: []string{OutcomePass, OutcomePass},
			output:   "provider playwright: 2 tests: 2 passed, 0 failed, 0 skipped\n",
		},
		"a failing test fails its file only": {
			units: two,
			document: providerReceipt("e2e",
				providerTest("e2e/login.spec.ts", "login > signs in", "failed"),
				providerTest("e2e/login.spec.ts", "login > signs out", "passed"),
				providerTest("e2e/cart.spec.ts", "cart > adds", "passed")),
			outcomes: []string{OutcomeFail, OutcomePass},
			output:   "provider playwright: failed login > signs in\nprovider playwright: 3 tests: 2 passed, 1 failed, 0 skipped\n",
		},
		"a failing test in no selected file fails every unit": {
			units: two,
			document: providerReceipt("e2e",
				providerTest("e2e/login.spec.ts", "login > signs in", "passed"),
				providerTest("e2e/cart.spec.ts", "cart > adds", "passed"),
				providerTest("e2e/global.setup.ts", "setup", "timedOut")),
			outcomes: []string{OutcomeFail, OutcomeFail},
			output:   "provider playwright: timedOut setup\nprovider playwright: 3 tests: 2 passed, 1 failed, 0 skipped\n",
		},
		"an unknown state is failing": {
			units:    []string{providerSpec},
			document: providerReceipt("e2e", providerTest("e2e/login.spec.ts", "login > signs in", "pending")),
			outcomes: []string{OutcomeFail},
			output:   "provider playwright: pending login > signs in\nprovider playwright: 1 tests: 0 passed, 1 failed, 0 skipped\n",
		},
		"only skipped tests is NO_TESTS": {
			units: two,
			document: providerReceipt("e2e",
				providerTest("e2e/login.spec.ts", "login > signs in", "skipped"),
				providerTest("e2e/cart.spec.ts", "cart > adds", "passed")),
			outcomes: []string{OutcomeNoTests, OutcomePass},
			output:   "provider playwright: 2 tests: 1 passed, 0 failed, 1 skipped\n",
		},
		"a Playwright file with no test of its own is NOT_RUN": {
			units:    two,
			document: providerReceipt("e2e", providerTest("e2e/login.spec.ts", "login > signs in", "passed")),
			outcomes: []string{OutcomePass, OutcomeNotRun},
			reasons:  []string{"provider playwright: its receipt names no test of e2e/cart.spec.ts alone; the unit is NOT_RUN"},
			output:   "provider playwright: 1 tests: 1 passed, 0 failed, 0 skipped\n",
		},
		"a passed test two selected paths could own counts for neither": {
			units:    []string{"typescript:playwright:a/x.spec.ts", "typescript:playwright:b/x.spec.ts"},
			document: providerReceipt("e2e", providerTest("x.spec.ts", "x", "passed")),
			outcomes: []string{OutcomeNotRun, OutcomeNotRun},
			reasons: []string{
				"provider playwright: its receipt names no test of a/x.spec.ts alone; the unit is NOT_RUN",
				"provider playwright: its receipt names no test of b/x.spec.ts alone; the unit is NOT_RUN",
			},
			output: "provider playwright: 1 tests: 1 passed, 0 failed, 0 skipped\n",
		},
		"a Vitest unit without a failure passes with the suite": {
			units: []string{providerUnitA, "typescript:vitest:web/src/b.test.ts"},
			document: providerReceipt("unit",
				providerTest("", "a > adds", "passed"), providerTest("", "b > subtracts", "passed")),
			outcomes: []string{OutcomePass, OutcomePass},
			output:   "provider vitest: 2 tests: 2 passed, 0 failed, 0 skipped\n",
		},
		"a Vitest failure fails its file, or every unit when it names none": {
			units: []string{providerUnitA, "typescript:vitest:web/src/b.test.ts"},
			document: providerReceipt("unit",
				providerTest(`C:\repo\web\src\a.test.ts`, "a > adds", "failed"), providerTest("", "b > subtracts", "passed")),
			outcomes: []string{OutcomeFail, OutcomePass},
			output:   "provider vitest: failed a > adds\nprovider vitest: 2 tests: 1 passed, 1 failed, 0 skipped\n",
		},
		"an unattributed Vitest failure fails every unit": {
			units:    []string{providerUnitA, "typescript:vitest:web/src/b.test.ts"},
			document: providerReceipt("unit", providerTest("", "a > adds", "failed"), providerTest("", "b > subtracts", "passed")),
			outcomes: []string{OutcomeFail, OutcomeFail},
			output:   "provider vitest: failed a > adds\nprovider vitest: 2 tests: 1 passed, 1 failed, 0 skipped\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			outcomes, reasons, output := providerRun(test.units, 0, test.document)
			if got := outcomeNames(outcomes); !reflect.DeepEqual(got, test.outcomes) {
				t.Errorf("outcomes %q, want %q", got, test.outcomes)
			}
			for index, outcome := range outcomes {
				if outcome.Unit != test.units[index] || outcome.Slice != SliceWhole {
					t.Errorf("outcome %d is for %s %s", index, outcome.Unit, outcome.Slice)
				}
			}
			if !reflect.DeepEqual(reasons, test.reasons) {
				t.Errorf("reasons %q, want %q", reasons, test.reasons)
			}
			if output != test.output {
				t.Errorf("output %q, want %q", output, test.output)
			}
		})
	}

	// At most providerFailureLines failing tests are named; the rest are counted.
	tests := []string{}
	for index := 0; index < providerFailureLines+7; index++ {
		tests = append(tests, providerTest("e2e/login.spec.ts", fmt.Sprintf("login > case %d\twith\x1b[31m control", index), "failed"))
	}
	_, _, output := providerRun([]string{providerSpec}, 0, providerReceipt("e2e", tests...))
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != providerFailureLines+2 || lines[providerFailureLines] != "provider playwright: +7 more failing tests" {
		t.Fatalf("%d output lines: %q", len(lines), output)
	}
	if strings.ContainsAny(output, "\t\x1b") {
		t.Fatal("provider-authored control characters reached the output")
	}
}

// STR-V0-8705: a provider run that observed no usable result leaves every
// unit NOT_RUN with the reason; none is read as a pass.
func TestSTRV08705ProviderRefusalsAreNotRun(t *testing.T) {
	passing := providerTest("e2e/login.spec.ts", "login > signs in", "passed")
	for name, test := range map[string]struct {
		exit     int
		document string
		reason   string
	}{
		"no document":            {1, "", "it exited 1 without a receipt document"},
		"exit 0 and no document": {0, "", "it exited 0 without a receipt document"},
		"not JSON":               {0, "Running 3 tests using 1 worker", "it exited 0 without a receipt document"},
		"no receipt member":      {0, `{"tests": []}`, "it exited 0 without a receipt document"},
		"usage refusal":          {2, providerReceipt("e2e", passing), "it exited 2"},
		"admission refusal":      {3, providerReceipt("e2e", passing), "it exited 3"},
		"infrastructure": {1, `{"receipt": {"kind": "e2e", "tests": [], "infrastructure": {"reason": "test-file-unbound", "detail": "e2e/global.setup.ts\nis not named"}}}`,
			"infrastructure failure test-file-unbound: e2e/global.setup.ts is not named"},
		"infrastructure with exit 0": {0, `{"receipt": {"kind": "e2e", "tests": [` + passing + `], "infrastructure": {"reason": "server-exited", "detail": "exit 1"}}}`,
			"infrastructure failure server-exited: exit 1"},
		"another kind":   {0, providerReceipt("unit", passing), `its receipt kind is "unit", want "e2e"`},
		"cancelled":      {0, `{"receipt": {"kind": "e2e", "cancelled": true, "tests": [` + passing + `]}}`, "the run was cancelled"},
		"stale app":      {0, `{"receipt": {"kind": "e2e", "staleAppBuild": true, "tests": [` + passing + `]}}`, "the app build changed while the tests ran"},
		"no test at all": {0, providerReceipt("e2e"), "its receipt reports no test"},
	} {
		outcomes, reasons, output := providerRun([]string{providerSpec, providerSpec2}, test.exit, test.document)
		if got := outcomeNames(outcomes); !reflect.DeepEqual(got, []string{OutcomeNotRun, OutcomeNotRun}) {
			t.Errorf("%s: outcomes %q, want every unit NOT_RUN", name, got)
		}
		want := []string{"provider playwright: " + test.reason + "; its selected tests are NOT_RUN"}
		if !reflect.DeepEqual(reasons, want) {
			t.Errorf("%s: reasons %q, want %q", name, reasons, want)
		}
		if output != "" {
			t.Errorf("%s: a refused run wrote %q", name, output)
		}
	}

	// A document over the bound is drained, not read.
	var capture providerCapture
	chunk := bytes.Repeat([]byte("x"), 1<<20)
	for written := 0; written <= providerCaptureLimit; written += len(chunk) {
		if n, err := capture.Write(chunk); n != len(chunk) || err != nil {
			t.Fatalf("capture write = %d, %v", n, err)
		}
	}
	planned := Planned{Adapter: AdapterProvider, Units: []Unit{{Adapter: AdapterProvider, Slice: SliceWhole, Unit: providerSpec}}}
	outcomes, reasons := providerOutcomes(planned, 0, &capture, &bytes.Buffer{})
	want := fmt.Sprintf("provider playwright: its receipt document exceeds %d bytes; its selected tests are NOT_RUN", providerCaptureLimit)
	if !reflect.DeepEqual(outcomeNames(outcomes), []string{OutcomeNotRun}) || !reflect.DeepEqual(reasons, []string{want}) {
		t.Fatalf("oversized document: outcomes %q, reasons %q", outcomeNames(outcomes), reasons)
	}

	// An invocation that never launched is recorded with every unit NOT_RUN.
	vitest := Planned{Adapter: AdapterProvider, Argv: []string{"corvint-js-test-provider", "unit"}, Units: []Unit{{Adapter: AdapterProvider, Slice: SliceWhole, Unit: providerUnitA}}}
	other := Planned{Adapter: AdapterDeclared, Argv: []string{"corvint-js-test-provider", "unit"}, Units: []Unit{{Adapter: AdapterDeclared, Slice: SliceWhole, Unit: "declared:typescript"}}}
	recorded := providerUnavailable([]Planned{planned, other, vitest}, "provider corvint-js-test-provider", AdapterManifests())
	if len(recorded) != 1 || recorded[0].Exit != -1 || recorded[0].Adapter != AdapterProvider || recorded[0].AdapterManifest != AdapterManifestProfile ||
		!reflect.DeepEqual(recorded[0].Outcomes, []Outcome{{Outcome: OutcomeNotRun, Slice: SliceWhole, Unit: providerUnitA}}) {
		t.Fatalf("unavailable provider recorded %+v", recorded)
	}
}
