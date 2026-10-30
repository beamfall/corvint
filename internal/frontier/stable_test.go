package frontier

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

const (
	stableCEMBytes = `{"spec":"cem/1.0"}`
	stableOCMBytes = `{"cem":{"spec":"cem/1.0"},"spec":"ocm/0.1-experimental"}`
)

// oidOf derives a deterministic 40-hex object id from a seed.
func oidOf(seed string) string { return digest([]byte(seed))[:40] }

type stubViewer struct {
	view  func(StableViewRequest) StableView
	err   error
	calls *int
}

func (viewer stubViewer) StableView(_ context.Context, request StableViewRequest) (StableView, error) {
	if viewer.calls != nil {
		*viewer.calls++
	}
	if viewer.err != nil {
		return StableView{}, viewer.err
	}
	return viewer.view(request), nil
}

func acceptedView(request StableViewRequest) StableView {
	return StableView{
		FrontierState: StableFrontierState, Spec: "cem/1.0", Verification: "ACCEPT", Sidecar: "EXACT",
		MapSHA256: digest(request.CEMBytes), PatchSHA256: strings.Repeat("a", 64), OCMSHA256: digest(request.OCMBytes),
		BaseRevision: oidOf("base"), TargetRevision: oidOf("target"), ObjectFormat: "sha1",
		TCQProfile: "tcq-stable/0", TCQID: "tcq-stable:sha256:" + strings.Repeat("b", 64), TestMode: TestModeStatic,
		UnknownHunks: []StableHunk{{ID: "hunk-1", Path: "add.go", Reason: "insufficient-evidence"}},
		Obligations:  []Obligation{{ID: "obligation-1", Disposition: OCMUnknown, Reason: "unassessed"}},
	}
}

func stableRequest() StableViewRequest {
	return StableViewRequest{
		CEMBytes: []byte(stableCEMBytes), OCMBytes: []byte(stableOCMBytes),
		ExpectedBase: oidOf("base"), Target: oidOf("target"), ArtifactRoot: "/artifacts",
	}
}

// TestStableViewAcceptsVerifiedMap: an accepted stable binding renders the
// frontier-stable-view/0 bytes, NOT_PRODUCED, exit 1, never EMPTY.
func TestStableViewAcceptsVerifiedMap(t *testing.T) {
	view, encoded, err := ComputeStable(context.Background(), stubViewer{view: acceptedView}, stableRequest())
	if err != nil {
		t.Fatal(err)
	}
	if view.ExitCode() != 1 || view.FrontierState != "NOT_PRODUCED" {
		t.Fatalf("view = %+v", view)
	}
	for _, want := range []string{`"profile":"frontier-stable-view/0"`, `"frontierState":"NOT_PRODUCED"`,
		`"verification":"ACCEPT"`, `"disposition":"unknown"`, `"hunkIds":[]`,
		`"unknownHunks":[{"id":"hunk-1","path":"add.go","reason":"insufficient-evidence"}]`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("encoded view lacks %s: %s", want, encoded)
		}
	}
	if !strings.HasSuffix(string(encoded), "}\n") || strings.Count(string(encoded), "\n") != 1 {
		t.Fatalf("encoded view is not one canonical line: %q", encoded)
	}
	if human := RenderStableHuman(view); !strings.Contains(human, "NOT_PRODUCED") || !strings.Contains(human, "closes nothing") {
		t.Fatalf("human view: %s", human)
	}
}

// TestStableViewAdmissionOrder: profile, revision and artifact-root refusals
// are decided before the viewer runs, each with a closed code.
func TestStableViewAdmissionOrder(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*StableViewRequest)
		code   string
	}{
		{"cem 0.2 map", func(r *StableViewRequest) { r.CEMBytes = []byte(`{"spec":"cem/0.2"}`) }, CodeUnsupportedContext},
		{"ocm bound to cem 0.2", func(r *StableViewRequest) {
			r.OCMBytes = []byte(`{"cem":{"spec":"cem/0.2"},"spec":"ocm/0.1-experimental"}`)
		}, CodeUnsupportedContext},
		{"partial dynamic tuple", func(r *StableViewRequest) { r.Command = []byte("x") }, CodeInvalidInput},
		{"missing ocm", func(r *StableViewRequest) { r.OCMBytes = nil }, CodeInvalidInput},
		{"missing expected base", func(r *StableViewRequest) { r.ExpectedBase = "" }, "expected-base-required"},
		{"missing target", func(r *StableViewRequest) { r.Target = "" }, "target-required"},
		{"missing artifact root", func(r *StableViewRequest) { r.ArtifactRoot = "" }, CodeInvalidInput},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			calls := 0
			request := stableRequest()
			testCase.mutate(&request)
			_, encoded, err := ComputeStable(context.Background(), stubViewer{view: acceptedView, calls: &calls}, request)
			if CodeOf(err) != testCase.code || encoded != nil || calls != 0 {
				t.Fatalf("code %q calls %d, want %q before the viewer", CodeOf(err), calls, testCase.code)
			}
		})
	}
}

// TestStableViewRefusalsAreClosed: a verifier refusal passes through only from
// the closed allowlists; an unknown code is frontier-internal-error; a view
// that binds other bytes or is not an ACCEPT never surfaces.
func TestStableViewRefusalsAreClosed(t *testing.T) {
	for code, want := range map[string]string{
		"base-revision-mismatch":     "base-revision-mismatch",
		"excluded-artifact-mismatch": "excluded-artifact-mismatch",
		"artifact-digest-mismatch":   "artifact-digest-mismatch",
		"never-declared-code":        CodeInternalError,
	} {
		_, encoded, err := ComputeStable(context.Background(), stubViewer{err: cemcode.New(code, "refused")}, stableRequest())
		if CodeOf(err) != want || encoded != nil {
			t.Fatalf("%s: code %q, want %q", code, CodeOf(err), want)
		}
	}
	forged := map[string]func(StableViewRequest) StableView{
		"other map bytes": func(r StableViewRequest) StableView {
			v := acceptedView(r)
			v.MapSHA256 = strings.Repeat("c", 64)
			return v
		},
		"reject outcome": func(r StableViewRequest) StableView { v := acceptedView(r); v.Verification = "REJECT"; return v },
		"empty state":    func(r StableViewRequest) StableView { v := acceptedView(r); v.FrontierState = "EMPTY"; return v },
		"absent sidecar": func(r StableViewRequest) StableView { v := acceptedView(r); v.Sidecar = "ABSENT"; return v },
	}
	for name, view := range forged {
		if _, encoded, err := ComputeStable(context.Background(), stubViewer{view: view}, stableRequest()); CodeOf(err) != CodeInternalError || encoded != nil {
			t.Fatalf("%s: code %q", name, CodeOf(err))
		}
	}
}

// TestStableVerifierCodesCoverTheResultSchema: every public code the stable
// verifier result schema declares passes through as itself, so foreseeable
// stable refusals (for example duplicate-reference) never become
// frontier-internal-error.
func TestStableVerifierCodesCoverTheResultSchema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "protocol", "cem-1.0", "stable", "RESULT.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			Code       struct{ Enum []*string } `json:"code"`
			IssueCodes struct {
				Items struct{ Enum []string } `json:"items"`
			} `json:"issueCodes"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	codes := append([]string(nil), schema.Properties.IssueCodes.Items.Enum...)
	for _, code := range schema.Properties.Code.Enum {
		if code != nil {
			codes = append(codes, *code)
		}
	}
	if len(codes) < 30 {
		t.Fatalf("schema enums not read: %d codes", len(codes))
	}
	for _, code := range codes {
		if got := CodeOf(translateStable(cemcode.New(code, "refused"))); got != code {
			t.Errorf("stable code %s translated to %s", code, got)
		}
	}
}
