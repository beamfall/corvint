package testacceptance

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// NEA-V0-011: a provider-infrastructure run names the provider's cause as a closed code, never its
// free-text detail.
func TestNEAV0011InfrastructureCause(t *testing.T) {
	for _, c := range []struct {
		reason, detail, want string
	}{
		{"report-identity-unknown", "project-location-unknown", "report-identity-unknown:project-location-unknown"},
		{"server-not-ready", "Get \"http://127.0.0.1:4173/\": dial tcp: connection refused", "server-not-ready"},
		{"report-unparseable", "stderr: /Users/someone/secret path\nmore", "report-unparseable"},
		{"application attestation failed: open /x", "application attestation provider could not be prepared", "unrecognized"},
	} {
		got := infrastructureCause(&jstestprovider.InfrastructureFailure{Reason: c.reason, Detail: c.detail})
		if got != c.want {
			t.Fatalf("%q/%q: cause %q, want %q", c.reason, c.detail, got, c.want)
		}
	}
	run := Run{Reasons: []string{}}
	markInfrastructure(&run, &jstestprovider.InfrastructureFailure{Reason: "server-not-ready", Detail: "dial tcp: connection refused"})
	b, err := json.Marshal(run)
	if err != nil || !strings.Contains(string(b), `"reasons":["provider-infrastructure"]`) || !strings.Contains(string(b), `"infrastructure":"server-not-ready"`) {
		t.Fatalf("run JSON %s (%v)", b, err)
	}
	clean := Run{Reasons: []string{}}
	markInfrastructure(&clean, nil)
	if b, _ := json.Marshal(clean); strings.Contains(string(b), "infrastructure") {
		t.Fatalf("a run without infrastructure failure names one: %s", b)
	}
}
