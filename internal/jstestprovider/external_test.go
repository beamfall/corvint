package jstestprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

func TestExternalReadiness(t *testing.T) {
	for _, status := range []int{200, 302, 401, 404, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			err := externalReady(context.Background(), server.URL, 80*time.Millisecond)
			if (err == nil) != (status == 200) {
				t.Fatalf("status %d: %v", status, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if externalReady(ctx, "http://127.0.0.1:1", time.Second) == nil {
		t.Fatal("cancelled readiness passed")
	}
}

func qualifiedFixture(t *testing.T) Receipt {
	t.Helper()
	r := Receipt{Profile: ExternalProfile, Kind: "e2e", Identity: Identity{ConfigFile: "/repo/config.cjs", ConfigDigest: "config", TestFileDigests: map[string]string{"/repo/test.cjs": "test"}, Argv: []string{"npx", "playwright", "--config=/tmp/a/config.cjs", "--project=one"}, NodeVersion: "v22", RunnerVersion: "1.60.0"}, External: &ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", ReadyAtStart: true, ReadyAtPublish: true, RunnerDescendantsGone: true, InputsUnchanged: true}}
	r.Identity.RunnerName = "playwright"
	r.External.ReadyURL = "http://127.0.0.1:3002"
	r.External.DeclaredAppIdentity = "fixture"
	r.External.ConfigOverride = "controlled-fixture-config"
	test := TestOutcome{Name: "pass", FullName: "one > pass", State: StatePassed, Anchor: &Anchor{File: "/repo/test.cjs", Line: 1}, Project: &ProjectIdentity{Name: "one", Browser: "chromium", Device: "unknown", Use: json.RawMessage(`{}`), ConfigDigest: "config"}, Attempts: []Attempt{{State: StatePassed, Retry: 0, FailureKind: "none"}}}
	r.Identity.ConfigInputDigests = map[string]string{r.Identity.ConfigFile: r.Identity.ConfigDigest}
	test.ID = qualifiedTestID(r.Identity, test)
	r.Tests = []TestOutcome{test}
	return r
}

func TestQualifiedReceiptProjection(t *testing.T) {
	r := qualifiedFixture(t)
	if ReceiptTestProjection(r, r.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("complete fixture did not pass")
	}
	for _, mutate := range []func(*Receipt){
		func(r *Receipt) { r.External = nil }, func(r *Receipt) { r.External.ReadyAtStart = false }, func(r *Receipt) { r.External.ReadyAtPublish = false }, func(r *Receipt) { r.External.RunnerDescendantsGone = false }, func(r *Receipt) { r.External.InputsUnchanged = false }, func(r *Receipt) { r.Tests[0].Project = nil }, func(r *Receipt) { r.Tests[0].ID = "forged" }, func(r *Receipt) { r.Tests[0].Project.Name = "" }, func(r *Receipt) { r.Tests[0].Attempts = nil }, func(r *Receipt) { r.Infrastructure = &InfrastructureFailure{Reason: "reporter"} },
		func(r *Receipt) { r.External.ReadyURL = "" }, func(r *Receipt) { r.External.DeclaredAppIdentity = "" }, func(r *Receipt) { r.External.ConfigOverride = "" }, func(r *Receipt) { r.Tests[0].Project.Use = json.RawMessage(`null`) }, func(r *Receipt) { r.Tests[0].Project.Use = json.RawMessage(`[]`) }, func(r *Receipt) { r.Tests[0].Attempts[0].State = "invented" }, func(r *Receipt) { r.Tests[0].Attempts[0].State = StateInfrastructure },
	} {
		r := qualifiedFixture(t)
		mutate(&r)
		if r.Tests[0].ID != "forged" {
			r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
		}
		if ReceiptTestProjection(r, r.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
			t.Fatal("unknown lifecycle/identity emitted pass")
		}
	}
	r.Cancelled = true
	r.Infrastructure = &InfrastructureFailure{Reason: "cancelled"}
	if ReceiptRunProjection(r).Execution.State != testvalidity.ExecutionCancelled {
		t.Fatal("cancellation was lost")
	}
	if ReceiptRunProjection(r).Freshness.State == testvalidity.FreshnessCurrent {
		t.Fatal("unknown external freshness became current")
	}
}

func TestPlaywright163UnqualifiedBrowserTupleAbstains(t *testing.T) {
	r := qualifiedFixture(t)
	r.Identity.RunnerVersion = "1.63.0"
	r.Identity.NodeVersion = "v22.23.2"
	r.Tests[0].Project.Use = json.RawMessage(`{"launchOptions":{"executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v22.23.2","browserType":"chromium","browserVersion":"Google Chrome 153.0.8010.48","channel":"","executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome","headlessShellAvailable":false}}`)
	r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
	if ReceiptTestProjection(r, r.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("qualified Playwright 1.63 browser tuple abstained")
	}
	for name, replacement := range map[string][2]string{
		"browser-version": {"Google Chrome 153.0.8010.48", "Google Chrome 153.0.8010.47"},
		"browser-type":    {`"browserType":"chromium"`, `"browserType":"firefox"`},
		"channel":         {`"channel":""`, `"channel":"chrome"`},
		"executable-path": {"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/tmp/chrome"},
		"node-version":    {"v22.23.2", "v22.23.1"},
	} {
		t.Run(name, func(t *testing.T) {
			mutated := r
			mutated.Tests = append([]TestOutcome(nil), r.Tests...)
			mutated.Tests[0].Project = &ProjectIdentity{}
			*mutated.Tests[0].Project = *r.Tests[0].Project
			mutated.Tests[0].Project.Use = json.RawMessage(strings.Replace(string(r.Tests[0].Project.Use), replacement[0], replacement[1], 1))
			mutated.Tests[0].ID = qualifiedTestID(mutated.Identity, mutated.Tests[0])
			if ReceiptTestProjection(mutated, mutated.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
				t.Fatal("mismatched Playwright 1.63 tuple projected green")
			}
		})
	}
}

func TestQualifiedIdentityStableAcrossScratchAndDistinctAcrossProjects(t *testing.T) {
	r := qualifiedFixture(t)
	original := r.Tests[0].ID
	r.Identity.Argv[2] = "--config=/tmp/b/config.cjs"
	if qualifiedTestID(r.Identity, r.Tests[0]) != original {
		t.Fatal("scratch changed stable test identity")
	}
	r.Tests[0].Project.Name = "two"
	if qualifiedTestID(r.Identity, r.Tests[0]) == original {
		t.Fatal("project identity collision")
	}
}

func TestExternalAdmission(t *testing.T) {
	base := E2EConfig{Config: Config{ConfigFile: "config.cjs", RunnerVersion: "1.60.0", TestFiles: []string{"test.cjs"}}, ExternalServer: true, AppIdentity: "fixture", ServerReadyURL: "http://127.0.0.1:3002"}
	if err := admitExternal(base); err != nil {
		t.Fatal(err)
	}
	base.RunnerVersion = "1.63.0"
	if err := admitExternal(base); err != nil {
		t.Fatal(err)
	}
	base.RunnerVersion = "1.62.0"
	if admitExternal(base) == nil {
		t.Fatal("unqualified Playwright version admitted")
	}
	base.RunnerVersion = "1.60.0"
	for _, arg := range []string{"--config=evil", "--reporter=json", "--global-setup=evil", "--ui", "--debug"} {
		c := base
		c.TestArgv = []string{arg}
		if admitExternal(c) == nil {
			t.Fatalf("admitted %s", arg)
		}
	}
	base.ServerArgv = []string{"server"}
	if admitExternal(base) == nil {
		t.Fatal("external mode admitted owned server")
	}
}

func TestQualifiedEncodingRejectsSecretAndBounds(t *testing.T) {
	r := qualifiedFixture(t)
	r.Tests[0].FailureMessage = "password=actual-private-value"
	if _, err := EncodeQualified(r); err == nil {
		t.Fatal("secret retained")
	}
	r = qualifiedFixture(t)
	r.Tests[0].FailureMessage = strings.Repeat("x", externalOutputLimit)
	if _, err := EncodeQualified(r); err == nil {
		t.Fatal("oversize retained")
	}
}
