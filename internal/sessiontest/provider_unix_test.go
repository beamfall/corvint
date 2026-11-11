//go:build darwin || linux

package sessiontest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// fakeProvider stands in for corvint-js-test-provider: it answers --version,
// logs its argv, prints the document the test wrote and exits as it was told.
const fakeProvider = `#!/bin/sh
if [ "$1" = --version ]; then
  [ -f "$FAKE_DIR/provider.noversion" ] && exit 1
  echo "corvint-js-test-provider 0.0.0-fixture"
  exit 0
fi
echo "$@" >> "$FAKE_DIR/provider-argv"
[ -f "$FAKE_DIR/provider.json" ] && cat "$FAKE_DIR/provider.json"
[ -f "$FAKE_DIR/provider.exit" ] && exit "$(cat "$FAKE_DIR/provider.exit")"
exit 0
`

// providerFixture is runnerFixture with a declared Playwright provider and
// one selected spec file beside the Go unit. The provider executable is
// installed only when install is set.
func providerFixture(t *testing.T, install bool, document string) (*Runner, Manifest, string, string) {
	t.Helper()
	runner, m, _, dir := runnerFixture(t, "pass")
	runner.Declared = withProviders()
	m.Units = append(m.Units, Unit{Adapter: AdapterProvider, Shard: 0, Slice: SliceWhole, Unit: providerSpec})
	if install {
		if err := execfile.WriteFile(filepath.Join(dir, ProviderExecutable), []byte(fakeProvider), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if document != "" {
		if err := os.WriteFile(filepath.Join(dir, "provider.json"), []byte(document), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sourceDigest, err := Digest(m.Source)
	if err != nil {
		t.Fatal(err)
	}
	m.SourceDigest = sourceDigest
	digest, err := Digest(m)
	if err != nil {
		t.Fatal(err)
	}
	return runner, m, digest, dir
}

// STR-V0-8704: the provider invocation runs after the Go one and is recorded
// in the same receipt, each unit with its own outcome.
func TestSTRV08704RunnerReportsProviderWithGo(t *testing.T) {
	passing := providerReceipt("e2e", providerTest("e2e/login.spec.ts", "login > signs in", "passed"))
	runner, m, digest, dir := providerFixture(t, true, passing)
	var output bytes.Buffer
	runner.Output = &output
	receipt := runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result != ResultPass || len(receipt.Incomplete) != 0 || len(receipt.Invocations) != 2 {
		t.Fatalf("receipt %s, incomplete %q, invocations %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
	}
	goRun, provider := receipt.Invocations[0], receipt.Invocations[1]
	if goRun.Adapter != AdapterGo || goRun.Outcomes[0].Outcome != OutcomePass {
		t.Fatalf("go invocation %+v", goRun)
	}
	wantArgv := append(append([]string{}, playwrightArgv...), "--test-file", "e2e/login.spec.ts", "--test-arg", `e2e/login\.spec\.ts`)
	if provider.Adapter != AdapterProvider || provider.AdapterManifest != AdapterManifestProfile || provider.Exit != 0 ||
		provider.Runner != filepath.Join(dir, ProviderExecutable) || provider.Version != "corvint-js-test-provider 0.0.0-fixture" ||
		!reflect.DeepEqual(provider.Argv, wantArgv) ||
		!reflect.DeepEqual(provider.Outcomes, []Outcome{{Outcome: OutcomePass, Slice: SliceWhole, Unit: providerSpec}}) {
		t.Fatalf("provider invocation %+v", provider)
	}
	logged, _ := os.ReadFile(filepath.Join(dir, "provider-argv"))
	if string(logged) != strings.Join(wantArgv[1:], " ")+"\n" {
		t.Fatalf("provider argv %q", logged)
	}
	if got := output.String(); !strings.Contains(got, "provider playwright: 1 tests: 1 passed, 0 failed, 0 skipped\n") || strings.Contains(got, `"receipt"`) {
		t.Fatalf("output %q", got)
	}
	if !strings.Contains(receipt.Summary, "2 of 2") {
		t.Fatalf("summary %q", receipt.Summary)
	}
	if result, reasons := Verify(m, digest, []Receipt{receipt}, nil); result != ResultPass {
		t.Fatalf("verify %s %q", result, reasons)
	}

	// A failing Playwright test fails the run while the Go unit still passes.
	failing := providerReceipt("e2e", providerTest("e2e/login.spec.ts", "login > signs in", "failed"))
	runner, m, digest, _ = providerFixture(t, true, failing)
	output.Reset()
	runner.Output = &output
	receipt = runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result != ResultFail || len(receipt.Invocations) != 2 || receipt.Invocations[0].Outcomes[0].Outcome != OutcomePass ||
		receipt.Invocations[1].Outcomes[0].Outcome != OutcomeFail {
		t.Fatalf("failing receipt %s, incomplete %q, invocations %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
	}
	if !strings.Contains(output.String(), "provider playwright: failed login > signs in\n") {
		t.Fatalf("output %q", output.String())
	}
}

// STR-V0-8705: a provider that is unavailable, or whose run observed no
// usable result, leaves its units NOT_RUN with the reason; the run is
// INCOMPLETE and the Go invocation still runs and reports.
func TestSTRV08705RunnerProviderUnavailable(t *testing.T) {
	notRunOutcome := []Outcome{{Outcome: OutcomeNotRun, Slice: SliceWhole, Unit: providerSpec}}
	check := func(t *testing.T, receipt Receipt, m Manifest, digest, reason string, exit int) {
		t.Helper()
		if receipt.Result != ResultIncomplete || len(receipt.Invocations) != 2 {
			t.Fatalf("receipt %s, incomplete %q, invocations %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
		}
		var goRun, provider Invocation
		for _, invocation := range receipt.Invocations {
			if invocation.Adapter == AdapterGo {
				goRun = invocation
			} else {
				provider = invocation
			}
		}
		if len(goRun.Outcomes) != 1 || goRun.Outcomes[0].Outcome != OutcomePass || goRun.Exit != 0 {
			t.Fatalf("the Go invocation did not run and pass: %+v", goRun)
		}
		if provider.Adapter != AdapterProvider || provider.AdapterManifest != AdapterManifestProfile || provider.Exit != exit ||
			!reflect.DeepEqual(provider.Outcomes, notRunOutcome) {
			t.Fatalf("provider invocation %+v, want exit %d and the unit NOT_RUN", provider, exit)
		}
		if len(receipt.Incomplete) == 0 || receipt.Incomplete[0] != reason {
			t.Fatalf("incomplete %q, want first %q", receipt.Incomplete, reason)
		}
		if result, reasons := Verify(m, digest, []Receipt{receipt}, nil); result != ResultIncomplete {
			t.Fatalf("verify %s %q, want INCOMPLETE", result, reasons)
		}
	}

	t.Run("not found", func(t *testing.T) {
		runner, m, digest, dir := providerFixture(t, false, "")
		receipt := runner.Execute(context.Background(), m, digest, 0)
		check(t, receipt, m, digest, "provider corvint-js-test-provider: runner corvint-js-test-provider not found", -1)
		if _, err := os.Stat(filepath.Join(dir, "provider-argv")); err == nil {
			t.Fatal("a provider that was not found was launched")
		}
	})
	t.Run("no version", func(t *testing.T) {
		runner, m, digest, dir := providerFixture(t, true, "")
		if err := os.WriteFile(filepath.Join(dir, "provider.noversion"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		receipt := runner.Execute(context.Background(), m, digest, 0)
		check(t, receipt, m, digest, "provider corvint-js-test-provider: runner version cannot be read", -1)
		if _, err := os.Stat(filepath.Join(dir, "provider-argv")); err == nil {
			t.Fatal("a provider without a version was launched")
		}
	})
	t.Run("no longer declared", func(t *testing.T) {
		runner, m, digest, _ := providerFixture(t, true, "")
		runner.Declared = parsedDeclared()
		receipt := runner.Execute(context.Background(), m, digest, 0)
		check(t, receipt, m, digest, providerSpec+": "+DeclarationPath+" names no command for it", -1)
	})
	t.Run("infrastructure failure", func(t *testing.T) {
		document := `{"receipt": {"kind": "e2e", "tests": [], "infrastructure": {"reason": "browser-missing", "detail": "chromium is not installed"}}}`
		runner, m, digest, dir := providerFixture(t, true, document)
		if err := os.WriteFile(filepath.Join(dir, "provider.exit"), []byte("1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		receipt := runner.Execute(context.Background(), m, digest, 0)
		check(t, receipt, m, digest, "provider playwright: infrastructure failure browser-missing: chromium is not installed; its selected tests are NOT_RUN", 1)
	})
	t.Run("exit 0 without a receipt", func(t *testing.T) {
		runner, m, digest, _ := providerFixture(t, true, "")
		receipt := runner.Execute(context.Background(), m, digest, 0)
		check(t, receipt, m, digest, "provider playwright: it exited 0 without a receipt document; its selected tests are NOT_RUN", 0)
	})
	t.Run("the selected file was not observed", func(t *testing.T) {
		document := providerReceipt("e2e", providerTest("e2e/other.spec.ts", "other > runs", "passed"))
		runner, m, digest, _ := providerFixture(t, true, document)
		receipt := runner.Execute(context.Background(), m, digest, 0)
		check(t, receipt, m, digest, "provider playwright: its receipt names no test of e2e/login.spec.ts alone; the unit is NOT_RUN", 0)
	})
}
