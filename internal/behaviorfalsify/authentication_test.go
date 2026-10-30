package behaviorfalsify

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

const storageSentinel = "disposable-session-value-4f1c2b"

// storageStateFixture writes a Playwright-shaped storage-state file outside every request root.
func storageStateFixture(t *testing.T, value string) StorageStateFile {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "state.json")
	data, _ := json.Marshal(map[string]any{
		"cookies": []any{map[string]any{"name": "sid", "value": value, "domain": "127.0.0.1", "path": "/", "sameSite": "Lax"}},
		"origins": []any{map[string]any{"origin": "http://127.0.0.1:41873", "localStorage": []any{map[string]any{"name": "user", "value": "disposable-user-" + value}}}},
	})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return StorageStateFile{Path: path, SHA256: digestBytes(data)}
}

func authenticatedRequest(t *testing.T, controls []ControlSpec) Request {
	t.Helper()
	request, _ := isolationRequest(t, controls)
	request.Authentication = &DeclaredAuthentication{Profile: AuthenticatedDisposable, StorageState: []StorageStateFile{storageStateFixture(t, storageSentinel)}}
	return request
}

func TestBBFV0217AuthenticationPlanIsBounded(t *testing.T) {
	request := authenticatedRequest(t, []ControlSpec{liveControl(t, "ok", "killed")})
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("BBF-V0-217 authentication is part of the plan digest", func(t *testing.T) {
		if plan.Request.Authentication == nil || plan.Request.Authentication.StorageState[0] != request.Authentication.StorageState[0] {
			t.Fatalf("plan authentication = %+v", plan.Request.Authentication)
		}
		other := request
		other.Authentication = &DeclaredAuthentication{Profile: AuthenticatedDisposable, StorageState: []StorageStateFile{storageStateFixture(t, "another-disposable-value")}}
		otherPlan, err := BuildPlan(other)
		if err != nil || otherPlan.Digest == plan.Digest {
			t.Fatalf("different storage state kept the digest: %v", err)
		}
		encoded, _ := json.Marshal(plan)
		if bytes.Contains(encoded, []byte(storageSentinel)) {
			t.Fatal("plan carries storage-state contents")
		}
	})
	inside := filepath.Join(request.DisposableRoot, "state.json")
	if err := os.WriteFile(inside, []byte(`{"cookies":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	shapeDir, _ := filepath.EvalSymlinks(t.TempDir())
	badShape := filepath.Join(shapeDir, "bad.json")
	if err := os.WriteFile(badShape, []byte(`{"cookies":[],"password":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		mutate func(*Request)
		code   string
	}{
		"no isolation":  {func(r *Request) { r.Isolation = nil; r.ExternalState = "none" }, "authentication-requires-isolation"},
		"wrong profile": {func(r *Request) { r.Authentication.Profile = "authenticated" }, "authentication-input-invalid"},
		"no files":      {func(r *Request) { r.Authentication.StorageState = nil }, "authentication-input-invalid"},
		"relative path": {func(r *Request) { r.Authentication.StorageState[0].Path = "state.json" }, "authentication-input-invalid"},
		"inside workspace": {func(r *Request) {
			r.Authentication.StorageState[0] = StorageStateFile{Path: inside, SHA256: mustFileDigest(t, inside)}
		}, "authentication-input-invalid"},
		"unknown key": {func(r *Request) {
			r.Authentication.StorageState[0] = StorageStateFile{Path: badShape, SHA256: mustFileDigest(t, badShape)}
		}, "authentication-input-invalid"},
		"duplicate file": {func(r *Request) {
			r.Authentication.StorageState = append(r.Authentication.StorageState, r.Authentication.StorageState[0])
		}, "authentication-input-invalid"},
		"digest mismatch":   {func(r *Request) { r.Authentication.StorageState[0].SHA256 = digestN("other") }, "authentication-input-drift"},
		"credential key":    {func(r *Request) { r.DeclaredEnvKeys = append(r.DeclaredEnvKeys, "E2E_API_TOKEN") }, "authentication-environment-secret-shaped"},
		"secret-shaped env": {func(r *Request) { r.DeclaredEnvKeys = append(r.DeclaredEnvKeys, "E2E_BASE_NAME") }, "authentication-environment-secret-shaped"},
	}
	secret := "ghp_" + strings.Repeat("A1b2C3d4E5", 4)[:36]
	if !secretscreen.MatchString("E2E_BASE_NAME=" + secret) {
		t.Fatal("fixture value is not secret-shaped")
	}
	t.Setenv("E2E_BASE_NAME", secret)
	t.Setenv("E2E_API_TOKEN", "plain")
	for name, tc := range cases {
		t.Run("BBF-V0-217 refuses "+name, func(t *testing.T) {
			candidate := request
			auth := *request.Authentication
			auth.StorageState = append([]StorageStateFile(nil), request.Authentication.StorageState...)
			candidate.Authentication = &auth
			candidate.DeclaredEnvKeys = append([]string(nil), request.DeclaredEnvKeys...)
			tc.mutate(&candidate)
			candidate.Runner.EnvironmentSHA256 = digestJSON(declaredEnvironment(candidate.DeclaredEnvKeys))
			if _, err := BuildPlan(candidate); err == nil || err.Error() != tc.code {
				t.Fatalf("err = %v, want %s", err, tc.code)
			}
		})
	}
}

func TestBBFV0218AuthenticationDriftAndMaterial(t *testing.T) {
	t.Run("BBF-V0-218 report labels the profile without contents", func(t *testing.T) {
		plan, err := BuildPlan(authenticatedRequest(t, []ControlSpec{liveControl(t, "ok", "killed")}))
		if err != nil {
			t.Fatal(err)
		}
		report, err := Execute(context.Background(), plan, plan.Digest)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(report)
		if report.Authentication == nil || report.Authentication.Profile != AuthenticatedDisposable || !strings.Contains(strings.Join(report.Limitations, "\n"), "caller-declared disposable") || bytes.Contains(encoded, []byte(storageSentinel)) {
			t.Fatalf("report authentication = %+v %v", report.Authentication, report.Limitations)
		}
	})
	t.Run("BBF-V0-218 drift before execution publishes nothing", func(t *testing.T) {
		plan, err := BuildPlan(authenticatedRequest(t, []ControlSpec{liveControl(t, "ok", "killed")}))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(plan.Request.Authentication.StorageState[0].Path, []byte(`{"cookies":[]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(context.Background(), plan, plan.Digest); err == nil || err.Error() != "authentication-input-drift" {
			t.Fatalf("err = %v", err)
		}
		if err := refuseAuthenticationMaterial(plan, Report{}); err == nil || err.Error() != "authentication-input-drift" {
			t.Fatalf("post-execution recheck err = %v", err)
		}
	})
	t.Run("BBF-V0-218 drift made by the reset hook skips the control", func(t *testing.T) {
		request, logPath := isolationRequest(t, []ControlSpec{liveControl(t, "swap-state", "killed")})
		request.Authentication = &DeclaredAuthentication{Profile: AuthenticatedDisposable, StorageState: []StorageStateFile{storageStateFixture(t, storageSentinel)}}
		plan, err := BuildPlan(request)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(logPath+".swap", []byte(request.Authentication.StorageState[0].Path), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(context.Background(), plan, plan.Digest); err == nil || err.Error() != "authentication-input-drift" {
			t.Fatalf("err = %v", err)
		}
		log, err := os.ReadFile(logPath)
		if err != nil || !strings.Contains(string(log), "reset swap-state") || strings.Contains(string(log), "control swap-state") {
			t.Fatalf("profile log = %q %v", log, err)
		}
	})
	t.Run("BBF-V0-218 storage-state material in a report is refused", func(t *testing.T) {
		request := authenticatedRequest(t, []ControlSpec{liveControl(t, "ok-"+storageSentinel, "killed")})
		plan, err := BuildPlan(request)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(context.Background(), plan, plan.Digest); err == nil || err.Error() != "authentication-material-in-report" {
			t.Fatalf("err = %v", err)
		}
		if _, err := ExecuteReceipt(context.Background(), plan, plan.Digest, receiptTool()); err == nil || err.Error() != "authentication-material-in-report" {
			t.Fatalf("receipt err = %v", err)
		}
	})
	t.Run("BBF-V0-218 material in retained receipt bytes is refused", func(t *testing.T) {
		plan, err := BuildPlan(authenticatedRequest(t, []ControlSpec{liveControl(t, "ok", "killed")}))
		if err != nil {
			t.Fatal(err)
		}
		clean := EvidenceReceipt{PlanPreimage: []byte(`{}`), RawAttempts: []RawAttempt{{HookBytes: []byte(`{}`), NativeBytes: []byte(`{"tests":[]}`)}}}
		if err := refuseAuthenticationMaterial(plan, clean); err != nil {
			t.Fatalf("clean receipt: %v", err)
		}
		for name, mutate := range map[string]func(*RawAttempt){
			"hook":   func(r *RawAttempt) { r.HookBytes = []byte("x " + storageSentinel) },
			"native": func(r *RawAttempt) { r.NativeBytes = []byte(`{"title":"` + storageSentinel + `"}`) },
			"reset":  func(r *RawAttempt) { r.IsolationResetBytes = []byte(storageSentinel) },
			"verify": func(r *RawAttempt) { r.IsolationVerifyBytes = []byte(storageSentinel) },
		} {
			receipt := clean
			receipt.RawAttempts = []RawAttempt{clean.RawAttempts[0]}
			mutate(&receipt.RawAttempts[0])
			if err := refuseAuthenticationMaterial(plan, receipt); err == nil || err.Error() != "authentication-material-in-report" {
				t.Fatalf("%s: err = %v", name, err)
			}
		}
	})
	t.Run("BBF-V0-218 escaped material is still found", func(t *testing.T) {
		material := AuthenticationMaterial([][]byte{[]byte(`{"cookies":[{"name":"sid","value":"a<b>&c\"quoted","domain":"127.0.0.1"}]}`)})
		escaped, _ := json.Marshal(map[string]string{"x": `a<b>&c"quoted`})
		if len(material) != 1 || !ContainsAuthenticationMaterial(escaped, material) || ContainsAuthenticationMaterial([]byte(`127.0.0.1 sid`), material) {
			t.Fatalf("material = %q", material)
		}
	})
}

func TestBBFV0218AuthenticationReceiptBinding(t *testing.T) {
	plan, err := BuildPlan(authenticatedRequest(t, []ControlSpec{liveControl(t, "ok", "killed")}))
	if err != nil {
		t.Fatal(err)
	}
	r, err := ExecuteReceipt(context.Background(), plan, plan.Digest, receiptTool())
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyReceipt(r, plan.Digest, receiptTool().Executable); err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeEvidence(r)
	if err != nil || bytes.Contains(encoded, []byte(storageSentinel)) {
		t.Fatalf("receipt carries storage-state contents: %v", err)
	}
	tampered := r
	tampered.Report.Authentication = nil
	tampered.Report.Digest = digestJSON(tampered.Report)
	tampered.Digest = evidenceDigest(tampered)
	if err := VerifyReceipt(tampered, plan.Digest, receiptTool().Executable); err == nil {
		t.Fatal("receipt without the declared authentication verified")
	}
}

func TestBBFV0219LegacyWireUnchanged(t *testing.T) {
	request, _ := isolationRequest(t, []ControlSpec{liveControl(t, "ok", "killed")})
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(plan)
	report, err := Execute(context.Background(), plan, plan.Digest)
	if err != nil {
		t.Fatal(err)
	}
	reportBytes, _ := json.Marshal(report)
	if bytes.Contains(encoded, []byte(`"authentication"`)) || bytes.Contains(reportBytes, []byte(`"authentication"`)) {
		t.Fatal("plan or report without authentication encodes the field")
	}
}
