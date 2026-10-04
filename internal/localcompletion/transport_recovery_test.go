package localcompletion

import (
	"encoding/json"
	"strings"
	"testing"
)

func transportRequestFixture() TransportRecoveryRequest {
	role := func(path, fill string) TransportRecoveryVerifier {
		return TransportRecoveryVerifier{AdapterPatchSHA256: "sha256:" + strings.Repeat(fill, 64), HistoricalRevision: strings.Repeat(fill, 40), HistoricalTree: strings.Repeat("f", 40), Path: path, SHA256: "sha256:" + strings.Repeat(fill+"0", 32)}
	}
	return TransportRecoveryRequest{Base: role("/opt/base/corvint", "a"), Tree: role("/opt/held/corvint", "b"), PlanDigest: strings.Repeat("c", 64), Profile: TransportAdaptedRecoveryProfile, Qualification: TransportAdaptedQualification, Session: strings.Repeat("d", 64)}
}

func transportRequestBytes(t *testing.T, value TransportRecoveryRequest) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

// ALO-V0-024 and ALO-V0-025: the request is one closed canonical value with
// absolute clean independent paths, valid digests and lowercase object IDs.
func TestTransportRecoveryRequestClosedCanonical(t *testing.T) {
	good := transportRequestBytes(t, transportRequestFixture())
	parsed, err := parseTransportRecovery(good)
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.provenance(); got.Qualification != "TRANSPORT_ADAPTED_HISTORICAL" || got.RequestSHA256 != prefixedDigest(good) || got.BaseSHA256 != parsed.request.Base.SHA256 || got.TreeSHA256 != parsed.request.Tree.SHA256 {
		t.Fatalf("provenance: %+v", got)
	}
	mutate := func(change func(*TransportRecoveryRequest)) []byte {
		value := transportRequestFixture()
		change(&value)
		return transportRequestBytes(t, value)
	}
	unknown := strings.Replace(string(good), `{"base"`, `{"extra":1,"base"`, 1)
	cases := map[string]struct {
		raw    []byte
		reason string
	}{
		"empty":            {nil, "transport-recovery-request-invalid"},
		"no-final-lf":      {good[:len(good)-1], "transport-recovery-request-invalid"},
		"leading-space":    {append([]byte(" "), good...), "transport-recovery-request-invalid"},
		"two-values":       {append(append([]byte{}, good...), good...), "transport-recovery-request-invalid"},
		"unknown-field":    {[]byte(unknown), "transport-recovery-request-invalid"},
		"oversize":         {[]byte(strings.Repeat(" ", maxTransportRecoveryBytes+1)), "transport-recovery-request-invalid"},
		"profile":          {mutate(func(r *TransportRecoveryRequest) { r.Profile = "corvint-transport-adapted-held-recovery/1" }), "transport-recovery-request-invalid"},
		"pristine":         {mutate(func(r *TransportRecoveryRequest) { r.Qualification = "PRISTINE_HISTORICAL" }), "transport-recovery-request-invalid"},
		"session":          {mutate(func(r *TransportRecoveryRequest) { r.Session = "key" }), "transport-recovery-request-invalid"},
		"plan-prefixed":    {mutate(func(r *TransportRecoveryRequest) { r.PlanDigest = "sha256:" + r.PlanDigest }), "transport-recovery-request-invalid"},
		"relative-path":    {mutate(func(r *TransportRecoveryRequest) { r.Base.Path = "corvint" }), "transport-recovery-request-invalid"},
		"unclean-path":     {mutate(func(r *TransportRecoveryRequest) { r.Tree.Path = "/opt/held/../held/corvint" }), "transport-recovery-request-invalid"},
		"unprefixed-sha":   {mutate(func(r *TransportRecoveryRequest) { r.Base.SHA256 = strings.Repeat("a0", 32) }), "transport-recovery-request-invalid"},
		"patch-digest":     {mutate(func(r *TransportRecoveryRequest) { r.Tree.AdapterPatchSHA256 = "" }), "transport-recovery-request-invalid"},
		"uppercase-commit": {mutate(func(r *TransportRecoveryRequest) { r.Base.HistoricalRevision = strings.Repeat("A", 40) }), "transport-recovery-request-invalid"},
		"short-tree":       {mutate(func(r *TransportRecoveryRequest) { r.Tree.HistoricalTree = strings.Repeat("f", 39) }), "transport-recovery-request-invalid"},
		"same-path":        {mutate(func(r *TransportRecoveryRequest) { r.Tree.Path = r.Base.Path }), "transport-recovery-fixed-verifier"},
		"same-identity":    {mutate(func(r *TransportRecoveryRequest) { r.Tree.SHA256 = r.Base.SHA256 }), "transport-recovery-fixed-verifier"},
	}
	for name, test := range cases {
		if _, err := parseTransportRecovery(test.raw); err == nil || err.Error() != test.reason {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// ALO-V0-024: only the compiled-in #443 admission exists in product builds; a
// malformed link-time test value admits nothing.
func TestTransportRecoveryAdmissionsClosed(t *testing.T) {
	entries := transportAdmissions()
	if len(entries) != 1 || entries[0].baseRevision != "406f9dc3cb80d6af527de5f160370e132e324e9a" || entries[0].heldRevision != "a79439afd1a8dd0650e598b7a1ec51d79c5c6075" {
		t.Fatalf("admissions: %+v", entries)
	}
	previous := transportRecoveryTestAdmissions
	t.Cleanup(func() { transportRecoveryTestAdmissions = previous })
	transportRecoveryTestAdmissions = "a,b,c"
	if entries = transportAdmissions(); entries != nil {
		t.Fatalf("malformed admissions: %+v", entries)
	}
}
