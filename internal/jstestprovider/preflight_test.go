package jstestprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

func writePreflight(t *testing.T, dir string, declaration any) string {
	t.Helper()
	data, err := json.Marshal(declaration)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "preflight.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func preflightServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/ready":
			w.WriteHeader(http.StatusOK)
		case "/moved":
			http.Redirect(w, r, "/healthz", http.StatusFound)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func writeStorageState(t *testing.T, dir string, expires float64) {
	t.Helper()
	state := fmt.Sprintf(`{"cookies":[{"name":"session","value":"v","domain":"127.0.0.1","path":"/","expires":%v}],"origins":[{"origin":"http://127.0.0.1:3000","localStorage":[]}]}`, expires)
	if err := os.MkdirAll(filepath.Join(dir, "auth"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "auth", "admin.json"), []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
}

// PWP-V0-090: the declaration is closed, bounded, kind-exclusive and every refusal is typed.
func TestPWPV0090PreflightDeclarationValidation(t *testing.T) {
	dir := t.TempDir()
	valid := map[string]any{"schema": PreflightSchema, "baseUrl": "http://127.0.0.1:3000", "totalTimeoutMs": 1000, "probes": []map[string]any{
		{"name": "api", "kind": "http", "path": "/healthz", "timeoutMs": 100},
		{"name": "seed", "kind": "command", "argv": []string{"true"}, "timeoutMs": 100},
		{"name": "admin", "kind": "storage-state", "file": "auth/admin.json", "requireCookies": []string{"session"}, "timeoutMs": 100},
	}}
	p, err := LoadPreflight(writePreflight(t, dir, valid))
	if err != nil || len(p.Digest) != 64 || len(p.Declaration.Probes) != 3 || !filepath.IsAbs(p.File) {
		t.Fatalf("valid declaration: %+v %v", p, err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"schema":         func(d map[string]any) { d["schema"] = "other/0" },
		"base-url":       func(d map[string]any) { d["baseUrl"] = "ftp://host" },
		"base-userinfo":  func(d map[string]any) { d["baseUrl"] = "http://u:p@host" },
		"total-timeout":  func(d map[string]any) { d["totalTimeoutMs"] = 0 },
		"unknown-member": func(d map[string]any) { d["extra"] = true },
		"no-probes":      func(d map[string]any) { d["probes"] = []any{} },
		"probe-timeout":  func(d map[string]any) { d["probes"].([]map[string]any)[0]["timeoutMs"] = 0 },
		"duplicate-name": func(d map[string]any) { d["probes"].([]map[string]any)[1]["name"] = "api" },
		"bad-name":       func(d map[string]any) { d["probes"].([]map[string]any)[0]["name"] = "API ready" },
		"bad-kind":       func(d map[string]any) { d["probes"].([]map[string]any)[0]["kind"] = "tcp" },
		"mixed-members":  func(d map[string]any) { d["probes"].([]map[string]any)[0]["argv"] = []string{"true"} },
		"cross-origin":   func(d map[string]any) { d["probes"].([]map[string]any)[0]["path"] = "//evil/healthz" },
		"bad-status":     func(d map[string]any) { d["probes"].([]map[string]any)[0]["expectStatus"] = []int{99} },
		"empty-argv":     func(d map[string]any) { d["probes"].([]map[string]any)[1]["argv"] = []string{""} },
		"no-argv":        func(d map[string]any) { d["probes"].([]map[string]any)[1]["argv"] = []string{} },
		"absent-argv":    func(d map[string]any) { delete(d["probes"].([]map[string]any)[1], "argv") },
		"escaping-state": func(d map[string]any) { d["probes"].([]map[string]any)[2]["file"] = "../outside.json" },
		"absolute-state": func(d map[string]any) { d["probes"].([]map[string]any)[2]["file"] = "/etc/state.json" },
		"negative-valid": func(d map[string]any) { d["probes"].([]map[string]any)[2]["minValidSeconds"] = -1 },
		"secret-shaped": func(d map[string]any) {
			d["probes"].([]map[string]any)[1]["argv"] = []string{"seed", "--password=hunter2hunter2"}
		},
		"unknown-in-kind": func(d map[string]any) { d["probes"].([]map[string]any)[0]["method"] = "POST" },
	} {
		t.Run(name, func(t *testing.T) {
			var d map[string]any
			data, _ := json.Marshal(valid)
			_ = json.Unmarshal(data, &d)
			probes := []map[string]any{}
			for _, p := range d["probes"].([]any) {
				probes = append(probes, p.(map[string]any))
			}
			d["probes"] = probes
			mutate(d)
			_, err := LoadPreflight(writePreflight(t, t.TempDir(), d))
			var refusal *E2ERefusal
			if !errors.As(err, &refusal) || refusal.Code != "preflight-declaration-invalid" {
				t.Fatalf("not refused as preflight-declaration-invalid: %v", err)
			}
		})
	}
	if _, err := LoadPreflight(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("missing declaration loaded")
	}
}

// PWP-V0-091: probes run in declared order inside their bounds; the first failure stops the
// preflight and names a closed reason; later probes are not-run.
func TestPWPV0091PreflightProbesRunInOrderAndStopAtFirstFailure(t *testing.T) {
	server := preflightServer(t)
	dir := t.TempDir()
	now := time.Unix(1_800_000_000, 0)
	defer func(saved func() time.Time) { preflightNow = saved }(preflightNow)
	preflightNow = func() time.Time { return now }
	writeStorageState(t, dir, float64(now.Unix()+3600))
	probes := func(extra ...PreflightProbe) []PreflightProbe {
		return append([]PreflightProbe{
			{Name: "api", Kind: PreflightHTTP, Path: "/healthz", TimeoutMS: 500},
			{Name: "seed", Kind: PreflightCommand, Argv: []string{"sh", "-c", "exit 0"}, TimeoutMS: 5000},
			{Name: "admin", Kind: PreflightStorageState, File: "auth/admin.json", RequireCookies: []string{"session"}, RequireOrigins: []string{"http://127.0.0.1:3000"}, MinValidSeconds: 300, TimeoutMS: 100},
		}, extra...)
	}
	run := func(total int64, list []PreflightProbe) (*PreflightReceipt, *InfrastructureFailure) {
		d := PreflightDeclaration{Schema: PreflightSchema, BaseURL: server.URL, TotalTimeoutMS: total, Probes: list}
		if err := validatePreflight(d); err != nil {
			t.Fatal(err)
		}
		return runPreflight(context.Background(), &Preflight{File: "/decl.json", Digest: strings.Repeat("a", 64), Declaration: d}, dir)
	}
	receipt, failure := run(20000, probes())
	if failure != nil || receipt.Outcome != PreflightPassed || len(receipt.Probes) != 3 {
		t.Fatalf("passing preflight: %+v %+v", receipt, failure)
	}
	if receipt.Probes[0].Artefact != server.URL+"/healthz" || receipt.Probes[2].Artefact != "auth/admin.json" || len(receipt.Probes[2].ArtefactDigest) != 64 {
		t.Fatalf("artefacts: %+v", receipt.Probes)
	}
	for _, c := range []struct {
		name, reason, observed string
		probe                  PreflightProbe
		state                  func()
	}{
		{"http-status", "http-status-unexpected", "status 503", PreflightProbe{Name: "seeded", Kind: PreflightHTTP, Path: "/seeded", TimeoutMS: 150}, nil},
		{"http-redirect", "http-status-unexpected", "status 302", PreflightProbe{Name: "moved", Kind: PreflightHTTP, Path: "/moved", TimeoutMS: 150}, nil},
		{"http-declared-status", "", "", PreflightProbe{Name: "unavailable", Kind: PreflightHTTP, Path: "/down", ExpectStatus: []int{503}, TimeoutMS: 150}, nil},
		{"command-exit", "command-exit-nonzero", "exit 3", PreflightProbe{Name: "seed-db", Kind: PreflightCommand, Argv: []string{"sh", "-c", "exit 3"}, TimeoutMS: 5000}, nil},
		{"command-timeout", "command-timeout", "", PreflightProbe{Name: "slow", Kind: PreflightCommand, Argv: []string{"sleep", "5"}, TimeoutMS: 100}, nil},
		{"command-start", "command-start-failed", "", PreflightProbe{Name: "absent", Kind: PreflightCommand, Argv: []string{filepath.Join(dir, "no-such-command")}, TimeoutMS: 1000}, nil},
		{"state-missing", "storage-state-missing", "", PreflightProbe{Name: "gone", Kind: PreflightStorageState, File: "auth/none.json", TimeoutMS: 100}, nil},
		{"state-cookie-missing", "storage-state-cookie-missing", "cookie csrf", PreflightProbe{Name: "csrf", Kind: PreflightStorageState, File: "auth/admin.json", RequireCookies: []string{"csrf"}, TimeoutMS: 100}, nil},
		{"state-origin-missing", "storage-state-origin-missing", "origin http://other", PreflightProbe{Name: "origin", Kind: PreflightStorageState, File: "auth/admin.json", RequireOrigins: []string{"http://other"}, TimeoutMS: 100}, nil},
		{"state-expiring", "storage-state-cookie-expiring", "cookie session", PreflightProbe{Name: "expiring", Kind: PreflightStorageState, File: "auth/admin.json", RequireCookies: []string{"session"}, MinValidSeconds: 7200, TimeoutMS: 100}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			list := probes()
			list = append([]PreflightProbe{list[0], c.probe}, list[1:]...)
			receipt, failure := run(20000, list)
			got := receipt.Probes[1]
			if c.reason == "" {
				if failure != nil || got.State != PreflightPassed {
					t.Fatalf("declared status did not pass: %+v %+v", got, failure)
				}
				return
			}
			if failure == nil || got.State != PreflightFailed || got.Reason != c.reason || got.Observed != c.observed {
				t.Fatalf("probe: %+v failure %+v", got, failure)
			}
			if receipt.Outcome != PreflightFailed || receipt.Probes[0].State != PreflightPassed {
				t.Fatalf("outcome: %+v", receipt)
			}
			for _, later := range receipt.Probes[2:] {
				if later.State != PreflightNotRun || later.Reason != "earlier-probe-failed" || later.DurationMS != 0 {
					t.Fatalf("later probe ran: %+v", later)
				}
			}
			if failure.Reason != ReasonPreflightFailed || failure.Phase != InfrastructurePhasePreflight || failure.Probe != c.probe.Name || failure.Artefact != got.Artefact || failure.Detail != c.probe.Name+": "+c.reason {
				t.Fatalf("infrastructure: %+v", failure)
			}
		})
	}
	// A total budget smaller than the probes leaves the interrupted probe not-run as preflight-timeout.
	receipt, failure = run(100, []PreflightProbe{{Name: "never", Kind: PreflightHTTP, Path: "/seeded", TimeoutMS: 5000}, {Name: "after", Kind: PreflightHTTP, Path: "/healthz", TimeoutMS: 100}})
	if failure == nil || receipt.Probes[0].State != PreflightNotRun || receipt.Probes[0].Reason != "preflight-timeout" || receipt.Probes[1].Reason != "earlier-probe-failed" {
		t.Fatalf("total timeout: %+v %+v", receipt, failure)
	}
	if err := preflightShapeError(Receipt{Profile: ExternalProfile, External: &ExternalLifecycle{}, Preflight: receipt, Infrastructure: failure}); err != nil {
		t.Fatalf("runner output fails its own shape: %v", err)
	}
}

// PWP-V0-092: a failed preflight never starts Playwright and writes a typed infrastructure receipt
// with no tests whose run projection names the probe and artefact.
func TestPWPV0092PreflightFailureReceiptHasNoTestVerdicts(t *testing.T) {
	server := preflightServer(t)
	dir := t.TempDir()
	for _, name := range []string{"playwright.config.cjs", "example.spec.cjs"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("// fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(dir, "runner-started")
	declaration := map[string]any{"schema": PreflightSchema, "baseUrl": server.URL, "totalTimeoutMs": 5000, "probes": []map[string]any{
		{"name": "api", "kind": "http", "path": "/ready", "timeoutMs": 500},
		{"name": "admin", "kind": "storage-state", "file": "auth/admin.json", "requireCookies": []string{"session"}, "timeoutMs": 100},
		{"name": "after", "kind": "command", "argv": []string{"touch", marker}, "timeoutMs": 1000},
	}}
	preflight, err := LoadPreflight(writePreflight(t, dir, declaration))
	if err != nil {
		t.Fatal(err)
	}
	cfg := E2EConfig{Config: Config{Dir: dir, ConfigFile: filepath.Join(dir, "playwright.config.cjs"), RunnerName: "playwright", RunnerVersion: "1.64.0", TestFiles: []string{filepath.Join(dir, "example.spec.cjs")}, Timeout: time.Second},
		ExternalServer: true, AppIdentity: "fixture", ServerReadyURL: server.URL + "/healthz", ServerReadyLimit: time.Second, TestArgv: []string{"--project=chromium"}, Preflight: preflight}
	r, err := RunE2E(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Tests) != 0 || r.External.Runner != nil || r.Preflight == nil || r.Preflight.Outcome != PreflightFailed {
		t.Fatalf("receipt ran tests or lost preflight: %+v", r)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a probe after the failure ran")
	}
	want := InfrastructureFailure{Reason: ReasonPreflightFailed, Detail: "admin: storage-state-missing", Phase: InfrastructurePhasePreflight, Probe: "admin", Artefact: "auth/admin.json"}
	if *r.Infrastructure != want || r.Preflight.DeclarationDigest != preflight.Digest {
		t.Fatalf("infrastructure %+v", r.Infrastructure)
	}
	run := ReceiptRunProjection(r)
	if run.Execution.State != testvalidity.ExecutionInfrastructure || run.Execution.Reason != "INFRASTRUCTURE" ||
		strings.Join(run.Execution.Anchors, ",") != "preflight-probe:admin,preflight-artefact:auth/admin.json" {
		t.Fatalf("run projection %+v", run.Execution)
	}
	document, err := EncodeQualified(r)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !strings.Contains(string(document), `"phase":"preflight"`) || !strings.Contains(string(document), `"preflight-probe:admin"`) {
		t.Fatalf("document lost the preflight phase: %s", document)
	}
	// A run whose readiness answers and whose probes all pass would proceed to the runner; a
	// preflight is refused outside external mode.
	for _, c := range []E2EConfig{{Preflight: preflight}, {Preflight: preflight, ExternalServer: true, CallerOwnedServer: true}, {Preflight: preflight, ExternalServer: true, Freshness: &FreshnessConfig{}}} {
		_, err := RunE2E(context.Background(), c)
		var refusal *E2ERefusal
		if !errors.As(err, &refusal) || refusal.Code != "preflight-requires-external-server" {
			t.Fatalf("admission: %v", err)
		}
	}
}

// PWP-V0-093: the preflight member must agree with infrastructure and tests; any disagreement is a
// shape error, which removes passing authority.
func TestPWPV0093PreflightShapeRemovesPassingAuthority(t *testing.T) {
	passed := &PreflightReceipt{Schema: PreflightSchema, DeclarationFile: "/d.json", DeclarationDigest: strings.Repeat("b", 64), BaseURL: "http://127.0.0.1:3000", Outcome: PreflightPassed,
		Probes: []PreflightProbeResult{{Name: "api", Kind: PreflightHTTP, State: PreflightPassed, Artefact: "http://127.0.0.1:3000/healthz"}}}
	r := qualifiedFixture(t)
	r.Preflight = passed
	if err := qualifiedProfileShapeError(r); err != nil || ReceiptTestProjection(r, r.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatalf("passed preflight removed passing: %v", err)
	}
	failed := func() *PreflightReceipt {
		p := *passed
		p.Outcome = PreflightFailed
		p.Probes = []PreflightProbeResult{{Name: "api", Kind: PreflightHTTP, State: PreflightFailed, Reason: "http-unreachable", Artefact: "http://127.0.0.1:3000/healthz"}}
		return &p
	}
	infra := &InfrastructureFailure{Reason: ReasonPreflightFailed, Detail: "api: http-unreachable", Phase: InfrastructurePhasePreflight, Probe: "api", Artefact: "http://127.0.0.1:3000/healthz"}
	for name, mutate := range map[string]func(*Receipt){
		"failed-with-tests":       func(r *Receipt) { r.Preflight, r.Infrastructure = failed(), infra },
		"phase-without-preflight": func(r *Receipt) { r.Infrastructure = infra },
		"failed-without-infra":    func(r *Receipt) { r.Preflight, r.Tests = failed(), nil },
		"wrong-probe": func(r *Receipt) {
			x := *infra
			x.Probe = "other"
			r.Preflight, r.Infrastructure, r.Tests = failed(), &x, nil
		},
		"passed-outcome-failed": func(r *Receipt) {
			p := failed()
			p.Outcome = PreflightPassed
			r.Preflight, r.Infrastructure, r.Tests = p, infra, nil
		},
		"open-reason": func(r *Receipt) {
			p := failed()
			p.Probes[0].Reason = "made-up"
			r.Preflight, r.Infrastructure, r.Tests = p, infra, nil
		},
		"ran-after-failure": func(r *Receipt) {
			p := failed()
			p.Probes = append(p.Probes, PreflightProbeResult{Name: "seed", Kind: PreflightCommand, State: PreflightPassed, Artefact: "seed"})
			r.Preflight, r.Infrastructure, r.Tests = p, infra, nil
		},
	} {
		r := qualifiedFixture(t)
		mutate(&r)
		if err := qualifiedProfileShapeError(r); err == nil {
			t.Fatalf("%s: shape accepted", name)
		}
		for _, test := range r.Tests {
			if ReceiptTestProjection(r, test).Execution.State == testvalidity.ExecutionPassed {
				t.Fatalf("%s: projected passing", name)
			}
		}
	}
	// An unprofiled receipt cannot carry a preflight; encoding refuses it.
	r = qualifiedFixture(t)
	r.Profile, r.Preflight = "", passed
	if _, err := EncodeQualified(r); err == nil || err.Error() != "preflight-requires-external-profile" {
		t.Fatalf("unprofiled preflight encoded: %v", err)
	}
	r = qualifiedFixture(t)
	r.Preflight, r.Infrastructure, r.Tests = failed(), infra, nil
	if err := qualifiedProfileShapeError(r); err != nil {
		t.Fatalf("well-formed failure: %v", err)
	}
}
