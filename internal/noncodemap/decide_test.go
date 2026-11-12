// SPDX-License-Identifier: Apache-2.0

package noncodemap

import (
	"slices"
	"strings"
	"testing"
)

// allFacts confirms every test and ancestor and knows no fault case.
var allFacts = Env{TestExists: func(string) bool { return true }, IsAncestor: func(string) (bool, error) { return true, nil }}

func mustParse(t *testing.T, body string) Map {
	t.Helper()
	read, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return read
}

func acceptedEntry(key, tests string) string {
	return `{` + key + `,"tests":[` + tests + `],"evidence":"reviewed","accepted_by":"owner","accepted_at":"2026-11-11","pinned_commit":"` + head + `","proof":"FIH-NC-0007"}`
}

func wantFull(t *testing.T, decision Decision, reason, p string) {
	t.Helper()
	line, full := decision.FullRun()
	if !full || line.Reason != reason || line.Path != p {
		t.Fatalf("FullRun = %+v %v, want %s %s; audit:\n%s", line, full, reason, p, strings.Join(decision.Audit(), "\n"))
	}
	if want := "noncode " + reason + " " + p; !slices.Contains(decision.Audit(), want) {
		t.Fatalf("audit lacks %q:\n%s", want, strings.Join(decision.Audit(), "\n"))
	}
}

func wantNoFull(t *testing.T, decision Decision) {
	t.Helper()
	if line, full := decision.FullRun(); full {
		t.Fatalf("FullRun = %+v, want no full run; audit:\n%s", line, strings.Join(decision.Audit(), "\n"))
	}
}

func TestNCMV1001UnmappedFixtureSelectsEverything(t *testing.T) {
	decision := Decide(Input{Dirty: []string{"tests/fixtures/users.json"}, Unowned: []string{"tests/fixtures/users.json"}})
	wantFull(t, decision, ReasonUnowned, "tests/fixtures/users.json")
}

// TestNCMV1001AttributedPathStaysOwned: the orchestrator ruling of
// 2026-11-11. A non-code path a Go rule attributes (data beside a package,
// its testdata, a go:embed input) is owned: it gets no line and keeps the
// selector's attribution, even when an unproven entry covers it. Only a
// path nothing attributes runs everything, as unowned-path.
func TestNCMV1001AttributedPathStaysOwned(t *testing.T) {
	attributed := func(p string) bool { return strings.HasPrefix(p, "internal/app/") }
	read := mustParse(t, `{"profile":"corvint-data-map/0","entries":[`+acceptedEntry(`"glob":"internal/app/testdata/*"`, `"go:internal/app::TestLoad"`)+`]}`)
	paths := []string{"internal/app/data.json", "internal/app/testdata/case.json", "notes/outside.xyz"}
	decision := Decide(Input{Dirty: paths, Unowned: paths, Attributed: attributed, Map: read, Env: allFacts})
	wantFull(t, decision, ReasonUnowned, "notes/outside.xyz")
	want := []string{
		"noncode unowned-path notes/outside.xyz",
		"data-map internal/app/testdata/* unproven: fault case FIH-NC-0007 is unknown: FIH-V1 is not linked (V1-5405)",
	}
	if got := decision.Audit(); !slices.Equal(got, want) {
		t.Fatalf("audit = %q, want %q", got, want)
	}
	wantNoFull(t, Decide(Input{Dirty: paths[:2], Unowned: paths[:2], Attributed: attributed}))
}

func TestNCMV1001UnownedPathSelectsEverything(t *testing.T) {
	decision := Decide(Input{Dirty: []string{"tools/notes.xyz"}, Unowned: []string{"tools/notes.xyz"}})
	wantFull(t, decision, ReasonUnowned, "tools/notes.xyz")
}

func TestNCMV1001DeclaredDocumentationAddsNothing(t *testing.T) {
	decision := Decide(Input{Dirty: []string{"docs/guide.md"}, Documentation: []string{"docs/guide.md"}})
	wantNoFull(t, decision)
	if len(decision.Lines) != 1 || decision.Lines[0].Reason != ReasonDocumentation {
		t.Fatalf("lines = %+v, want one DECLARED_DOCUMENTATION line", decision.Lines)
	}
	if audit := decision.Audit(); len(audit) != 0 {
		t.Fatalf("audit = %q, want none: the selector prints its own documentation line", audit)
	}
	// Inside an embedding package the declaration still wins: declared
	// documentation adds no test (review P2-1).
	embedded := Decide(Input{Dirty: []string{"internal/ui/assets/README.md"}, Unowned: []string{"internal/ui/assets/README.md"}, Documentation: []string{"internal/ui/assets/README.md"}, Attributed: func(string) bool { return true }})
	wantNoFull(t, embedded)
	if len(embedded.Lines) != 1 || embedded.Lines[0].Reason != ReasonDocumentation {
		t.Fatalf("lines = %+v, want one DECLARED_DOCUMENTATION line", embedded.Lines)
	}
}

func TestNCMV1001RemovedRunnerSelectsEverything(t *testing.T) {
	decision := Decide(Input{Dirty: []string{"web/vitest.config.ts"}, Deleted: []string{"web/vitest.config.ts"}})
	wantFull(t, decision, ReasonRunnerRemoved, "web/vitest.config.ts")
	unavailable := Decide(Input{Dirty: []string{"internal/a/a.go"}, Unavailable: []string{"go: provider.go.state is UNAVAILABLE"}})
	wantFull(t, unavailable, ReasonRunnerRemoved, "go: provider.go.state is UNAVAILABLE")
	// Code with every runner available gets no line.
	wantNoFull(t, Decide(Input{Dirty: []string{"internal/a/a.go"}}))
}

func TestNCMV1004UnprovenEntryDoesNotNarrow(t *testing.T) {
	read := mustParse(t, `{"profile":"corvint-data-map/0","entries":[`+acceptedEntry(`"path":"config/app.yml"`, `"go:internal/app::TestLoad"`)+`]}`)
	input := Input{Dirty: []string{"config/app.yml"}, Unowned: []string{"config/app.yml"}, Map: read, Env: allFacts}
	for name, fault := range map[string]func(string) (FaultCase, bool){
		"unlinked":     nil,
		"unknown case": func(string) (FaultCase, bool) { return FaultCase{}, false },
		"not killed":   func(string) (FaultCase, bool) { return FaultCase{Path: "config/app.yml"}, true },
		"other killer": func(string) (FaultCase, bool) {
			return FaultCase{Path: "config/app.yml", Killed: true, Killers: []string{"go:x::TestX"}}, true
		},
		"other file": func(string) (FaultCase, bool) {
			return FaultCase{Path: "config/db.yml", Killed: true, Killers: []string{"go:internal/app::TestLoad"}}, true
		},
	} {
		input.Env.FaultCase = fault
		decision := Decide(input)
		wantFull(t, decision, ReasonUnowned, "config/app.yml")
		if len(decision.Entries) != 1 || decision.Entries[0].Status != StatusUnproven {
			t.Fatalf("%s: entries = %+v, want one unproven", name, decision.Entries)
		}
	}
	input.Env.FaultCase = nil
	if got := Decide(input).Entries[0].Detail; got != "fault case FIH-NC-0007 is unknown: FIH-V1 is not linked (V1-5405)" {
		t.Fatalf("unlinked detail = %q", got)
	}
	// The narrowing side, reachable once FIH-V1 is linked (change task 6).
	input.Env.FaultCase = func(string) (FaultCase, bool) {
		return FaultCase{Path: "config/app.yml", Killed: true, Killers: []string{"go:internal/app::TestLoad"}}, true
	}
	decision := Decide(input)
	wantNoFull(t, decision)
	if decision.Lines[0].Reason != ReasonProvenEntry || !slices.Equal(decision.Lines[0].Tests, []string{"go:internal/app::TestLoad"}) {
		t.Fatalf("lines = %+v, want a proven-entry line with the entry's test", decision.Lines)
	}
}

// neverNarrowedCases is the NCM-V1-005 list, path by path.
var neverNarrowedCases = map[string]string{
	"go.mod": CategoryLockfile, "go.sum": CategoryLockfile, "go.work": CategoryLockfile, "go.work.sum": CategoryLockfile,
	"web/package-lock.json": CategoryLockfile, "npm-shrinkwrap.json": CategoryLockfile, "yarn.lock": CategoryLockfile,
	"pnpm-lock.yaml": CategoryLockfile, "bun.lock": CategoryLockfile, "bun.lockb": CategoryLockfile,
	"app/Gemfile.lock": CategoryLockfile, "poetry.lock": CategoryLockfile, "Pipfile.lock": CategoryLockfile,
	"uv.lock": CategoryLockfile, "pdm.lock": CategoryLockfile, "Cargo.lock": CategoryLockfile, "composer.lock": CategoryLockfile,

	".go-version": CategoryToolchain, ".tool-versions": CategoryToolchain, ".nvmrc": CategoryToolchain,
	".node-version": CategoryToolchain, ".python-version": CategoryToolchain, ".ruby-version": CategoryToolchain,
	"rust-toolchain": CategoryToolchain, "rust-toolchain.toml": CategoryToolchain, "mise.toml": CategoryToolchain,
	".mise.toml": CategoryToolchain,

	".github/workflows/ci.yml": CategoryCI, ".github/cishards/package-costs.json": CategoryCI,

	"Makefile": CategoryMake, "sub/makefile": CategoryMake, "GNUmakefile": CategoryMake, "build/rules.mk": CategoryMake,

	"web/package.json": CategoryRunnerConfig, "tsconfig.json": CategoryRunnerConfig, "web/tsconfig.build.json": CategoryRunnerConfig,
	"web/vitest.config.ts": CategoryRunnerConfig, "vitest.workspace.json": CategoryRunnerConfig, "vite.config.mjs": CategoryRunnerConfig,
	"jest.config.cjs": CategoryRunnerConfig, "e2e/playwright.config.ts": CategoryRunnerConfig, "pytest.ini": CategoryRunnerConfig,
	"pyproject.toml": CategoryRunnerConfig, "setup.cfg": CategoryRunnerConfig, "tox.ini": CategoryRunnerConfig,
	"tests/conftest.py": CategoryRunnerConfig, "Gemfile": CategoryRunnerConfig, ".rspec": CategoryRunnerConfig,
	"Rakefile": CategoryRunnerConfig,

	".goreleaser.yml": CategoryRelease, ".goreleaser.yaml": CategoryRelease, "tools/release-binary/binaries.json": CategoryRelease,

	"api/service.proto": CategoryGenerator, "buf.yaml": CategoryGenerator, "buf.gen.yaml": CategoryGenerator,
	"sqlc.yaml": CategoryGenerator, "sqlc.yml": CategoryGenerator, "gqlgen.yml": CategoryGenerator,

	"web/__snapshots__/app.test.ts.snap": CategorySnapshot, "__snapshots__/x.txt": CategorySnapshot,
	"spec/a.snap": CategorySnapshot, "internal/x/testdata/out.golden": CategorySnapshot,

	".corvint/data-map.json": CategoryCorvint, ".corvint/test-read-scopes.json": CategoryCorvint,

	".env": CategorySecret, ".env.production": CategorySecret, "app/.envrc": CategorySecret,
	"config/credentials.yml.enc": CategorySecret, "config/master.key": CategorySecret, "config/secrets.yml": CategorySecret,
}

func TestNCMV1005NeverNarrowedPaths(t *testing.T) {
	for p, category := range neverNarrowedCases {
		got, never := NeverNarrowed(p)
		if !never || got != category {
			t.Errorf("NeverNarrowed(%q) = %q %v, want %q", p, got, never, category)
			continue
		}
		decision := Decide(Input{Dirty: []string{p}, Unowned: []string{p}})
		line, full := decision.FullRun()
		want := ReasonNeverNarrowed
		if strings.HasPrefix(p, ".corvint/") {
			want = ReasonCorvintConfig
		}
		if !full || line.Reason != want || line.Path != p {
			t.Errorf("%s: FullRun = %+v %v, want %s", p, line, full, want)
		}
	}
	for _, p := range []string{"config/app.yml", "docs/guide.md", "internal/a/a.go", "tests/fixtures/users.json", "web/src/app.ts", "envoy.yaml", "golden.txt"} {
		if category, never := NeverNarrowed(p); never {
			t.Errorf("NeverNarrowed(%q) = %q, want not on the list", p, category)
		}
	}
	// A go:embed input is attributed to its embedding package, which owns it
	// (review P2-1): it narrows to that attribution, not to a full run.
	embedded := Decide(Input{Dirty: []string{"internal/ui/assets/app.css"}, Unowned: []string{"internal/ui/assets/app.css"}, Attributed: func(p string) bool { return p == "internal/ui/assets/app.css" }})
	wantNoFull(t, embedded)
	if len(embedded.Lines) != 0 {
		t.Fatalf("lines = %+v, want none", embedded.Lines)
	}
}

func TestNCMV1005LockfileIgnoresEntry(t *testing.T) {
	read := mustParse(t, `{"profile":"corvint-data-map/0","entries":[`+acceptedEntry(`"path":"package-lock.json"`, `"vitest:a.test.ts","vitest:b.test.ts"`)+`]}`)
	proving := allFacts
	proving.FaultCase = func(string) (FaultCase, bool) {
		return FaultCase{Path: "package-lock.json", Killed: true, Killers: []string{"vitest:a.test.ts"}}, true
	}
	decision := Decide(Input{Dirty: []string{"package-lock.json"}, Unowned: []string{"package-lock.json"}, Map: read, Env: proving})
	wantFull(t, decision, ReasonNeverNarrowed, "package-lock.json")
	status, detail := Evaluate(read.Entries[0], proving)
	if status != StatusIgnored || !strings.Contains(detail, CategoryLockfile) {
		t.Fatalf("Evaluate = %s %q, want ignored as a lockfile", status, detail)
	}
}

func TestNCMV1006CorvintConfigSelectsEverything(t *testing.T) {
	for _, p := range []string{
		".corvint/data-map.json", ".corvint/runtime-readers.json", ".corvint/test-read-scopes.json",
		// The orchestrator's call on the design's open question 1.
		".corvint/changes/x.cem.json", ".corvint/deferred-intents/a.json", ".corvint/unplanned-reads.enabled", ".corvint/test-inventory.json",
	} {
		wantFull(t, Decide(Input{Dirty: []string{p}}), ReasonCorvintConfig, p)
	}
	// A declared documentation path under .corvint/ keeps its class.
	doc := Decide(Input{Dirty: []string{".corvint/README.md"}, Documentation: []string{".corvint/README.md"}})
	wantNoFull(t, doc)
	if doc.Lines[0].Reason != ReasonDocumentation {
		t.Fatalf("lines = %+v, want DECLARED_DOCUMENTATION", doc.Lines)
	}
	// A sealed CEM named by a full commit is change evidence: no line, so the
	// selector's evidence attribution stands (ADN-V0-011). Any other name
	// under changes/ (x.cem.json above) stays configuration.
	for _, sealed := range []string{".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json", ".corvint/changes/" + strings.Repeat("ab", 32) + ".cem.json"} {
		if got := Decide(Input{Dirty: []string{sealed}}); len(got.Lines) != 0 {
			t.Fatalf("%s: lines = %+v, want none", sealed, got.Lines)
		}
	}
	for _, forged := range []string{".corvint/changes/0123456789ABCDEF0123456789abcdef01234567.cem.json", ".corvint/changes/x/0123456789abcdef0123456789abcdef01234567.cem.json"} {
		wantFull(t, Decide(Input{Dirty: []string{forged}}), ReasonCorvintConfig, forged)
	}
}

func TestNCMV1006FlowFileAddsLinkedTestsOnly(t *testing.T) {
	for _, p := range []string{".corvint/flows.json", ".corvint/e2e-flows.json", ".corvint/flows/checkout.json"} {
		decision := Decide(Input{Dirty: []string{p}, FlowsDirectory: ".corvint/flows", FlowsJoined: true})
		wantNoFull(t, decision)
		if want := "noncode flow-linked " + p; !slices.Equal(decision.Audit(), []string{want}) {
			t.Fatalf("%s: audit = %q, want %q", p, decision.Audit(), want)
		}
	}
	// A flow directory outside .corvint/ holds no exclusion: its files are
	// ordinary paths for the other rules.
	outside := Decide(Input{Dirty: []string{".corvint/flows/a.json"}, FlowsDirectory: "flows", FlowsJoined: true})
	wantFull(t, outside, ReasonCorvintConfig, ".corvint/flows/a.json")
	// The changed flows cannot be computed: everything, with a named reason.
	// The deprecated flow file too (review P2-3).
	for _, p := range []string{".corvint/flows.json", ".corvint/e2e-flows.json"} {
		wantFull(t, Decide(Input{Dirty: []string{p}}), ReasonFlowsUnreadable, p)
	}
	invalid := Decide(Input{Dirty: []string{".corvint/e2e-flows.json"}, FlowsJoined: true, FlowsInvalid: "E2E_FLOWS_INVALID"})
	wantFull(t, invalid, ReasonFlowsUnreadable, ".corvint/e2e-flows.json")
}

func TestNCMV1006LedgerAppendAddsNothing(t *testing.T) {
	for _, p := range []string{".corvint/self-observations.jsonl", ".corvint/unplanned-reads.jsonl"} {
		decision := Decide(Input{Dirty: []string{p}})
		wantNoFull(t, decision)
		if want := "noncode private-ledger " + p; !slices.Equal(decision.Audit(), []string{want}) {
			t.Fatalf("audit = %q, want %q", decision.Audit(), want)
		}
	}
}

func TestNCMV1008StaleEntryDoesNotNarrow(t *testing.T) {
	read := mustParse(t, `{"profile":"corvint-data-map/0","entries":[`+acceptedEntry(`"glob":"config/*.yml"`, `"go:internal/app::TestLoad","go:internal/app::TestGone"`)+`]}`)
	proving := Env{
		TestExists: func(test string) bool { return test != "go:internal/app::TestGone" },
		IsAncestor: func(string) (bool, error) { return true, nil },
		FaultCase: func(string) (FaultCase, bool) {
			return FaultCase{Path: "config/app.yml", Killed: true, Killers: []string{"go:internal/app::TestLoad"}}, true
		},
	}
	input := Input{Dirty: []string{"config/app.yml"}, Unowned: []string{"config/app.yml"}, Map: read, Env: proving}
	decision := Decide(input)
	wantFull(t, decision, ReasonUnowned, "config/app.yml")
	if want := "data-map config/*.yml stale: cited test go:internal/app::TestGone is gone or cannot be confirmed"; !slices.Contains(decision.Audit(), want) {
		t.Fatalf("audit lacks %q:\n%s", want, strings.Join(decision.Audit(), "\n"))
	}
	proving.TestExists = func(string) bool { return true }
	proving.IsAncestor = func(string) (bool, error) { return false, nil }
	input.Env = proving
	decision = Decide(input)
	wantFull(t, decision, ReasonUnowned, "config/app.yml")
	if decision.Entries[0].Status != StatusStale || !strings.Contains(decision.Entries[0].Detail, "not an ancestor") {
		t.Fatalf("entries = %+v, want stale: not an ancestor", decision.Entries)
	}
	input.Env = Env{FaultCase: proving.FaultCase}
	if got := Decide(input).Entries[0].Status; got != StatusStale {
		t.Fatalf("unreadable facts: status = %s, want stale", got)
	}
}

func TestNCMV1009DecisionIsDeterministic(t *testing.T) {
	read := mustParse(t, `{"profile":"corvint-data-map/0","entries":[`+
		acceptedEntry(`"path":"b.json"`, `"go:b::TestB"`)+`,`+acceptedEntry(`"glob":"*.json"`, `"go:a::TestA"`)+`]}`)
	input := Input{
		Dirty:         []string{"z.txt", "b.json", ".corvint/flows.json", "a.json", "docs/x.md", "b.json"},
		Unowned:       []string{"a.json", "b.json", "z.txt"},
		Documentation: []string{"docs/x.md"},
		Map:           read,
		Env:           allFacts,
	}
	first := strings.Join(Decide(input).Audit(), "\n")
	slices.Reverse(input.Dirty)
	slices.Reverse(input.Unowned)
	if second := strings.Join(Decide(input).Audit(), "\n"); first != second {
		t.Fatalf("input order changed the decision:\n%s\n--\n%s", first, second)
	}
	want := strings.Join([]string{
		"noncode flows-unreadable .corvint/flows.json",
		"noncode unowned-path a.json",
		"noncode unowned-path b.json",
		"noncode unowned-path z.txt",
		"data-map *.json unproven: fault case FIH-NC-0007 is unknown: FIH-V1 is not linked (V1-5405)",
		"data-map b.json unproven: fault case FIH-NC-0007 is unknown: FIH-V1 is not linked (V1-5405)",
	}, "\n")
	if first != want {
		t.Fatalf("audit:\n%s\nwant:\n%s", first, want)
	}
}
