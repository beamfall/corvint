package testvaliditydoc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// PWP-V0-092, PWP-V0-093: a retained preflight-failure document decodes canonically and its
// recomputed run projection names the failing probe and artefact behind INFRASTRUCTURE.
func TestPWPV0092RetainedPreflightFailureNamesProbe(t *testing.T) {
	failure := &jstestprovider.InfrastructureFailure{Reason: jstestprovider.ReasonPreflightFailed, Detail: "seed: command-exit-nonzero", Phase: jstestprovider.InfrastructurePhasePreflight, Probe: "seed", Artefact: "npm"}
	r := jstestprovider.Receipt{Profile: jstestprovider.ExternalProfile, Kind: "e2e", Tests: []jstestprovider.TestOutcome{},
		External: &jstestprovider.ExternalLifecycle{ReadyURL: "http://127.0.0.1:3000/healthz", DeclaredAppIdentity: "fixture", Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", ReadyAtStart: true, ConfigOverride: "config"},
		Preflight: &jstestprovider.PreflightReceipt{Schema: jstestprovider.PreflightSchema, DeclarationFile: "/repo/preflight.json", DeclarationDigest: strings.Repeat("c", 64), BaseURL: "http://127.0.0.1:3000", Outcome: jstestprovider.PreflightFailed,
			Probes: []jstestprovider.PreflightProbeResult{
				{Name: "api", Kind: jstestprovider.PreflightHTTP, State: jstestprovider.PreflightPassed, Artefact: "http://127.0.0.1:3000/healthz", Observed: "status 200", DurationMS: 3},
				{Name: "seed", Kind: jstestprovider.PreflightCommand, State: jstestprovider.PreflightFailed, Reason: "command-exit-nonzero", Artefact: "npm", Observed: "exit 1", DurationMS: 40},
				{Name: "admin", Kind: jstestprovider.PreflightStorageState, State: jstestprovider.PreflightNotRun, Reason: "earlier-probe-failed", Artefact: "auth/admin.json"},
			}},
		Infrastructure: failure}
	data, err := jstestprovider.EncodeQualified(r)
	if err != nil {
		t.Fatal(err)
	}
	input, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	projected, err := json.Marshal(Project(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(projected), `"state":"INFRASTRUCTURE","reason":"INFRASTRUCTURE","anchors":["preflight-probe:seed","preflight-artefact:npm"]`) {
		t.Fatalf("run projection does not name the probe: %s", projected)
	}
	// A preflight failure document carrying a test verdict is not canonical.
	r.Tests = []jstestprovider.TestOutcome{{Name: "t", FullName: "t", State: jstestprovider.StatePassed}}
	if _, err := jstestprovider.EncodeQualified(r); err == nil {
		t.Fatal("preflight failure with tests encoded")
	}
	// An unprofiled receipt cannot carry a preflight member.
	bare, _ := json.Marshal(map[string]any{"receipt": jstestprovider.Receipt{Kind: "e2e", Tests: []jstestprovider.TestOutcome{}, Preflight: r.Preflight}})
	if _, err := Decode(bare); err == nil {
		t.Fatal("unprofiled preflight decoded")
	}
}
