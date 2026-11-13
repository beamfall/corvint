//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/sessiontest"
	"github.com/Beamfall/corvint/internal/testduration"
)

// sessionFakeProvider stands in for corvint-js-test-provider: it records its
// argv beside the fake go and make, answers --version and prints the receipt
// document the fixture wrote.
const sessionFakeProvider = `#!/bin/sh
echo "corvint-js-test-provider $*" >> "$SESSION_FAKE_LOG"
if [ "$1" = --version ]; then echo "corvint-js-test-provider 0.0.0-fixture"; exit 0; fi
cat "$SESSION_FAKE_PROVIDER_DOCUMENT"
`

const (
	providerSessionSpec = "e2e/login.spec.ts"
	providerSessionUnit = "typescript:playwright:" + providerSessionSpec
	providerSessionArgv = "corvint-js-test-provider e2e --test-arg npx --test-arg playwright --test-arg test --test-file e2e/login.spec.ts --test-arg e2e/login\\.spec\\.ts"
)

// providerSessionFixture is sessionFixture with a Playwright project and a
// declared Playwright provider: its HEAD changes Go package a and the
// Playwright spec file, so the plan selects both. The stand-in provider is
// on PATH only when install is set, and reports the spec's one test in the
// given state.
func providerSessionFixture(t *testing.T, install bool, state string) (root, base, log string) {
	t.Helper()
	root, _, log = sessionFixture(t, "pass")
	declaration := `{"profile": "corvint-test-commands/0", "go": {"env": {"GOTOOLCHAIN": "local"}, "flags": ["-p", "1", "-count=1"]}, ` +
		`"languages": {"typescript": ["make", "ts-test"]}, ` +
		`"providers": {"playwright": ["corvint-js-test-provider", "e2e", "--test-arg", "npx", "--test-arg", "playwright", "--test-arg", "test"]}}` + "\n"
	write := func(files map[string]string) {
		t.Helper()
		for name, body := range files {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	write(map[string]string{
		"package.json":           `{"name":"fixture","private":true,"devDependencies":{"@playwright/test":"1.50.0"}}` + "\n",
		"playwright.config.ts":   "import { defineConfig } from \"@playwright/test\";\nexport default defineConfig({ testDir: \"./e2e\" });\n",
		providerSessionSpec:      "import { test } from \"@playwright/test\";\n\ntest(\"login\", async ({ page }) => {\n  await page.goto(\"/login\");\n});\n",
		sessiontest.CommandsPath: declaration,
	})
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "add the Playwright project")
	base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	write(map[string]string{
		"a/a.go":            "package a\n\nfunc A() int { return 4 }\n",
		providerSessionSpec: "import { test } from \"@playwright/test\";\n\ntest(\"login\", async ({ page }) => {\n  await page.goto(\"/sign-in\");\n});\n",
	})
	affectedGit(t, root, "commit", "-qam", "change a and the login spec")
	scratch := t.TempDir()
	if install {
		if err := execfile.WriteFile(filepath.Join(scratch, sessiontest.ProviderExecutable), []byte(sessionFakeProvider), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", scratch+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	document := `{"receipt": {"kind": "e2e", "tests": [{"fullName": "login", "state": "` + state + `", "anchor": {"file": "` + providerSessionSpec + `"}}]}}` + "\n"
	if err := os.WriteFile(filepath.Join(scratch, "provider.json"), []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SESSION_FAKE_PROVIDER_DOCUMENT", filepath.Join(scratch, "provider.json"))
	return root, base, log
}

// providerSessionInvocations returns the receipt's Go and provider
// invocations and fails when either is missing.
func providerSessionInvocations(t *testing.T, receipt sessiontest.Receipt) (goRun, provider sessiontest.Invocation) {
	t.Helper()
	found := 0
	for _, invocation := range receipt.Invocations {
		switch invocation.Adapter {
		case sessiontest.AdapterGo:
			goRun = invocation
			found |= 1
		case sessiontest.AdapterProvider:
			provider = invocation
			found |= 2
		}
	}
	if found != 3 {
		t.Fatalf("the receipt lacks a go or a provider invocation: %+v", receipt.Invocations)
	}
	return goRun, provider
}

// STR-V0-8704 (with STR-V0-8702, STR-V0-8703): a change to a Go package and
// a Playwright spec runs the Go package and, through the declared provider,
// the selected spec, and reports both in one receipt.
func TestSTRV08704CLIMixedGoAndPlaywrightRun(t *testing.T) {
	root, base, log := providerSessionFixture(t, true, "passed")
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
	if code != 0 || !strings.Contains(stderr, "corvint-test: PASS selected units passed: 3 of 3;") {
		t.Fatalf("code %d stdout %q stderr %q", code, stdout, stderr)
	}
	receipt := readSessionReceipt(t, receiptPath)
	if receipt.Result != sessiontest.ResultPass || receipt.Scope != sessiontest.ScopeNarrowed || len(receipt.Incomplete) != 0 {
		t.Fatalf("receipt %+v", receipt)
	}
	goRun, provider := providerSessionInvocations(t, receipt)
	if len(goRun.Outcomes) != 1 || goRun.Outcomes[0] != (sessiontest.Outcome{DurationGranularity: testduration.GranularityFile, DurationSource: testduration.NotMeasured, Outcome: sessiontest.OutcomePass, Slice: sessiontest.SliceWhole, TestID: "go:a", Unit: "go:example.test/m/a"}) {
		t.Fatalf("go invocation %+v", goRun)
	}
	if provider.AdapterManifest != sessiontest.AdapterManifestProfile || provider.Exit != 0 || provider.Version != "corvint-js-test-provider 0.0.0-fixture" ||
		strings.Join(provider.Argv, " ") != providerSessionArgv || len(provider.Outcomes) != 1 ||
		provider.Outcomes[0] != (sessiontest.Outcome{DurationGranularity: testduration.GranularityFile, DurationSource: testduration.NotMeasured, Outcome: sessiontest.OutcomePass, Slice: sessiontest.SliceWhole, TestID: "playwright:" + providerSessionSpec, Unit: providerSessionUnit}) {
		t.Fatalf("provider invocation %+v", provider)
	}
	// The e2e-runtime-dependency frontier of the spec still selects the
	// declared TypeScript suite, which runs beside the provider.
	if len(receipt.Invocations) != 3 || !slices.ContainsFunc(receipt.Invocations, func(invocation sessiontest.Invocation) bool {
		return invocation.Adapter == sessiontest.AdapterDeclared && strings.Join(invocation.Argv, " ") == "make ts-test"
	}) {
		t.Fatalf("invocations %+v", receipt.Invocations)
	}
	calls := sessionRunnerLog(t, log)
	for _, want := range []string{"go test -json -p 1 -count=1 example.test/m/a\n", providerSessionArgv + "\n", "make ts-test\n"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("runner calls lack %q:\n%s", want, calls)
		}
	}
	if !strings.Contains(stdout+stderr, "provider playwright: 1 tests: 1 passed, 0 failed, 0 skipped") {
		t.Fatalf("the provider summary line is missing: stdout %q stderr %q", stdout, stderr)
	}

	// A failing Playwright test fails the run; the Go unit still passes.
	root, base, _ = providerSessionFixture(t, true, "failed")
	code, _, stderr = runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
	if code != 1 || !strings.Contains(stderr, "corvint-test: FAIL ") {
		t.Fatalf("failing run: code %d stderr %q", code, stderr)
	}
	receipt = readSessionReceipt(t, receiptPath)
	goRun, provider = providerSessionInvocations(t, receipt)
	if receipt.Result != sessiontest.ResultFail || goRun.Outcomes[0].Outcome != sessiontest.OutcomePass ||
		len(provider.Outcomes) != 1 || provider.Outcomes[0].Outcome != sessiontest.OutcomeFail {
		t.Fatalf("failing receipt %s: %+v", receipt.Result, receipt.Invocations)
	}
}

// STR-V0-8705: with the provider absent from PATH the selected spec is
// NOT_RUN with the reason and the command exits 3, never 0; the Go package
// still runs and passes.
func TestSTRV08705CLIUnavailableProviderIsNotRun(t *testing.T) {
	root, base, log := providerSessionFixture(t, false, "passed")
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
	reason := "provider corvint-js-test-provider: runner corvint-js-test-provider not found"
	if code != 3 || !strings.Contains(stderr, "corvint-test: INCOMPLETE ") || !strings.Contains(stderr, reason) {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	receipt := readSessionReceipt(t, receiptPath)
	if receipt.Result != sessiontest.ResultIncomplete || !slices.Contains(receipt.Incomplete, reason) {
		t.Fatalf("receipt %s incomplete %q", receipt.Result, receipt.Incomplete)
	}
	goRun, provider := providerSessionInvocations(t, receipt)
	if goRun.Exit != 0 || len(goRun.Outcomes) != 1 || goRun.Outcomes[0].Outcome != sessiontest.OutcomePass {
		t.Fatalf("the Go invocation did not run and pass: %+v", goRun)
	}
	if provider.Exit != -1 || len(provider.Outcomes) != 1 ||
		provider.Outcomes[0] != (sessiontest.Outcome{DurationSource: testduration.NotMeasured, Outcome: sessiontest.OutcomeNotRun, Slice: sessiontest.SliceWhole, TestID: "playwright:" + providerSessionSpec, Unit: providerSessionUnit}) {
		t.Fatalf("provider invocation %+v, want the unit NOT_RUN", provider)
	}
	if calls := sessionRunnerLog(t, log); strings.Contains(calls, "corvint-js-test-provider") || !strings.Contains(calls, "go test -json -p 1 -count=1 example.test/m/a\n") {
		t.Fatalf("runner calls:\n%s", calls)
	}
}

// STR-V0-8703, STR-V0-8707: a dry run lists the provider invocation with its
// argv and unit and launches nothing, and the help states the member, the
// NOT_RUN reporting and the exit code.
func TestSTRV08707CLIDryRunListsProviderInvocation(t *testing.T) {
	root, base, log := providerSessionFixture(t, true, "passed")
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--full")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	var printed struct {
		Invocations []struct {
			Adapter string
			Argv    []string
			Units   []struct{ Unit string }
		} `json:"invocations"`
		Scope string `json:"scope"`
	}
	if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
		t.Fatalf("%v: %s", err, stdout)
	}
	listed := false
	for _, invocation := range printed.Invocations {
		if invocation.Adapter == sessiontest.AdapterProvider {
			listed = strings.Join(invocation.Argv, " ") == providerSessionArgv && len(invocation.Units) == 1 && invocation.Units[0].Unit == providerSessionUnit
		}
	}
	if printed.Scope != sessiontest.ScopeNarrowed || len(printed.Invocations) != 3 || !listed {
		t.Fatalf("dry run printed %s", stdout)
	}
	if calls := sessionRunnerLog(t, log); calls != "" {
		t.Fatalf("a dry run launched runners:\n%s", calls)
	}
	for _, want := range []string{
		`When that file's "providers" member declares a corvint-js-test-provider`,
		"leaves those tests NOT_RUN with the reason and\nthe run INCOMPLETE (exit 3), never PASS (STR-V0-8701 to STR-V0-8707).",
	} {
		if !strings.Contains(testHelp, want) {
			t.Errorf("corvint test --help lacks %q", want)
		}
	}
}
