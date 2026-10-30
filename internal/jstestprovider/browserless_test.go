package jstestprovider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// browserlessReport is a bound 1.63.0 /0 run whose tests are built by the caller.
func browserlessReport(t *testing.T, tests ...TestOutcome) (Receipt, qualifiedReport) {
	t.Helper()
	dir := t.TempDir()
	config := filepath.Join(dir, "playwright.config.cjs")
	if err := os.WriteFile(config, []byte("module.exports = {};\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := digestFile(config)
	if err != nil {
		t.Fatal(err)
	}
	r := qualifiedFixture(t)
	r.Identity.RunnerVersion, r.Identity.NodeVersion = "1.63.0", "v22.23.2"
	r.Identity.ConfigFile, r.Identity.ConfigDigest = config, digest
	r.Identity.ConfigInputDigests = map[string]string{config: digest}
	r.Tests = tests
	return r, qualifiedReport{Version: r.Identity.RunnerVersion, Status: "passed", Tests: r.Tests, ConfigFiles: map[string]string{config: digest}, Files: map[string]string{"/repo/test.cjs": "test"}}
}

func browserlessTest(name, browser, use string, states ...ExecutionState) TestOutcome {
	test := TestOutcome{Name: name, FullName: "one > " + name, State: states[len(states)-1], Retries: len(states) - 1, Anchor: &Anchor{File: "/repo/test.cjs", Line: 1}, Project: &ProjectIdentity{Name: "one", Browser: browser, Device: "unknown", Use: json.RawMessage(use)}}
	for i, state := range states {
		test.Attempts = append(test.Attempts, Attempt{State: state, Retry: i, FailureKind: "none"})
	}
	return test
}

const qualifiedSystemChromeUse = `{"browserName":"chromium","channel":"","launchOptions":{"executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v22.23.2","browserType":"chromium","browserVersion":"Google Chrome 153.0.8010.48","channel":"","executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome","headlessShellAvailable":true}}`

// PWP-V3-015: an API-only, a skipped and a fixture-failed browserless test bind beside a browser
// test. Only the API-only tests qualify, on the runner and node tuple; the skipped and
// fixture-failed tests make no execution claim.
func TestPWPV3015BrowserlessTestsBindAndQualify(t *testing.T) {
	r, report := browserlessReport(t,
		browserlessTest("page", "chromium", qualifiedSystemChromeUse, StatePassed),
		browserlessTest("api", BrowserlessProject, `{"corvintBrowserless":"no-browser-use"}`, StatePassed),
		browserlessTest("api failed", BrowserlessProject, `{"corvintBrowserless":"no-browser-use"}`, StateFailed),
		browserlessTest("skipped", BrowserlessProject, `{"corvintBrowserless":"not-run"}`, StateSkipped),
		browserlessTest("auth fixture", BrowserlessProject, `{"corvintBrowserless":"fixture-failed-before-browser"}`, StateInfrastructure),
	)
	if err := bindQualifiedReport(&r, report); err != nil {
		t.Fatalf("mixed browser and browserless run refused: %v", err)
	}
	if named := ProjectLocationUnknownTests(r); len(named) != 0 {
		t.Fatalf("bound run named tests %v", named)
	}
	for i, wantUnknown := range []bool{false, false, false, true, true} {
		if got := qualifiedPrerequisitesUnknown(r, r.Tests[i]); got != wantUnknown {
			t.Fatalf("%s: prerequisites unknown=%v, want %v", r.Tests[i].Name, got, wantUnknown)
		}
	}
	for _, i := range []int{0, 1} {
		if state := ReceiptTestProjection(r, r.Tests[i]).Execution.State; state != testvalidity.ExecutionPassed {
			t.Fatalf("%s projected %s", r.Tests[i].Name, state)
		}
	}
	if state := ReceiptTestProjection(r, r.Tests[2]).Execution.State; state != testvalidity.ExecutionFailed {
		t.Fatalf("failed API test projected %s", state)
	}
	// The browserless record is part of the test identity: rewriting it breaks the binding.
	forged := r
	forged.Tests = append([]TestOutcome(nil), r.Tests...)
	project := *forged.Tests[1].Project
	project.Use = json.RawMessage(`{"corvintBrowserless":"not-run"}`)
	forged.Tests[1].Project = &project
	if !qualifiedPrerequisitesUnknown(forged, forged.Tests[1]) {
		t.Fatal("rewritten browserless reason kept its qualification")
	}
	// An unadmitted node leaves the API-only test unqualified.
	unadmitted := r
	unadmitted.Identity.NodeVersion = "v22.23.3"
	if !qualifiedPrerequisitesUnknown(unadmitted, unadmitted.Tests[1]) {
		t.Fatal("API-only test qualified on an unadmitted node")
	}
}

// PWP-V3-015, PWP-V3-016: a test that used a browser without a recorded identity, and a
// browserless record outside the closed shape, are refused as project-location-unknown with the
// detail unchanged; the refusal names the offending tests by pointer only.
func TestPWPV3016BrowserIdentityMissingRefusedAndNamed(t *testing.T) {
	cases := map[string]TestOutcome{
		"browser-used-identity-missing": browserlessTest("page", "", `null`, StatePassed),
		"not-run-but-passed":            browserlessTest("page", BrowserlessProject, `{"corvintBrowserless":"not-run"}`, StatePassed),
		"fixture-failed-but-retried":    browserlessTest("page", BrowserlessProject, `{"corvintBrowserless":"fixture-failed-before-browser"}`, StateInfrastructure, StatePassed),
		"reason-unknown":                browserlessTest("page", BrowserlessProject, `{"corvintBrowserless":"api"}`, StatePassed),
		"reason-missing":                browserlessTest("page", BrowserlessProject, `{}`, StatePassed),
		"extra-member":                  browserlessTest("page", BrowserlessProject, `{"corvintBrowserless":"no-browser-use","browserName":"chromium"}`, StatePassed),
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			r, report := browserlessReport(t, browserlessTest("api", BrowserlessProject, `{"corvintBrowserless":"no-browser-use"}`, StatePassed), bad)
			if err := bindQualifiedReport(&r, report); err == nil || err.Error() != "project-location-unknown" {
				t.Fatalf("err=%v, want project-location-unknown", err)
			}
			if named := ProjectLocationUnknownTests(r); !reflect.DeepEqual(named, []string{"/receipt/tests/1"}) {
				t.Fatalf("named %v", named)
			}
		})
	}
	t.Run("runner-1.60.0", func(t *testing.T) {
		r, report := browserlessReport(t, browserlessTest("api", BrowserlessProject, `{"corvintBrowserless":"no-browser-use"}`, StatePassed))
		r.Identity.RunnerVersion, report.Version = "1.60.0", "1.60.0"
		if err := bindQualifiedReport(&r, report); err == nil || err.Error() != "project-location-unknown" {
			t.Fatalf("1.60.0 browserless err=%v", err)
		}
	})
	t.Run("bounded", func(t *testing.T) {
		var tests []TestOutcome
		for i := range 11 {
			tests = append(tests, browserlessTest(fmt.Sprint(i), "", `null`, StatePassed))
		}
		r, _ := browserlessReport(t, tests...)
		named := ProjectLocationUnknownTests(r)
		if len(named) != externalMaxUnboundNamed+1 || named[0] != "/receipt/tests/0" || named[externalMaxUnboundNamed] != "+3 more" {
			t.Fatalf("named %v", named)
		}
	})
}
