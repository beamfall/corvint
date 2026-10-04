package localcompletion

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
	if len(entries) != 1 || entries[0].baseRevision != "406f9dc3cb80d6af527de5f160370e132e324e9a" || entries[0].heldRevision != "a79439afd1a8dd0650e598b7a1ec51d79c5c6075" ||
		entries[0].baseBinary != "sha256:0a68e6fe4cdf1adfe0029852ae1e073a3cc38f211161c7df5f23d5d1df42fb16" || entries[0].heldBinary != "sha256:e308360e3ef3f3dbae8ffde39f6309c427856cba80fab7188d2492175b521d66" {
		t.Fatalf("admissions: %+v", entries)
	}
	previous := transportRecoveryTestAdmissions
	t.Cleanup(func() { transportRecoveryTestAdmissions = previous })
	transportRecoveryTestAdmissions = "a,b,c"
	if entries = transportAdmissions(); entries != nil {
		t.Fatalf("malformed admissions: %+v", entries)
	}
}

// ALO-V0-024: the compiled-in row admits its provenance only with its pinned
// adapted executables; a request naming any other binary digest is refused
// before any repository read, and the pinned pair proceeds to the BASE check.
func TestTransportRecoveryAdmissionPinsVerifierBinaries(t *testing.T) {
	row := admittedTransportAdaptations[0]
	role := func(path, revision, tree, patch, binary string) TransportRecoveryVerifier {
		return TransportRecoveryVerifier{AdapterPatchSHA256: patch, HistoricalRevision: revision, HistoricalTree: tree, Path: path, SHA256: binary}
	}
	request := TransportRecoveryRequest{
		Base:       role("/opt/base/corvint", row.baseRevision, row.baseTree, row.basePatch, row.baseBinary),
		Tree:       role("/opt/held/corvint", row.heldRevision, row.heldTree, row.heldPatch, row.heldBinary),
		PlanDigest: row.planDigest, Profile: TransportAdaptedRecoveryProfile, Qualification: TransportAdaptedQualification, Session: row.session,
	}
	// A plan base other than the row's makes the pinned pair stop at the BASE
	// check, which needs no repository.
	saved := &state{Session: row.session, PlanDigest: row.planDigest, Plan: Plan{Base: strings.Repeat("0", 40)}}
	admit := func(change func(*TransportRecoveryRequest)) error {
		value := request
		if change != nil {
			change(&value)
		}
		parsed, err := parseTransportRecovery(transportRequestBytes(t, value))
		if err != nil {
			t.Fatal(err)
		}
		return parsed.admit(context.Background(), nil, saved, snapshot{})
	}
	if err := admit(nil); err == nil || err.Error() != "transport-recovery-base-mismatch" {
		t.Fatalf("pinned binaries: %v", err)
	}
	wrong := "sha256:" + strings.Repeat("ab", 32)
	for name, change := range map[string]func(*TransportRecoveryRequest){
		"base-binary": func(r *TransportRecoveryRequest) { r.Base.SHA256 = wrong },
		"held-binary": func(r *TransportRecoveryRequest) { r.Tree.SHA256 = wrong },
		"swapped":     func(r *TransportRecoveryRequest) { r.Base.SHA256, r.Tree.SHA256 = r.Tree.SHA256, r.Base.SHA256 },
	} {
		if err := admit(change); err == nil || err.Error() != "transport-recovery-verifier-not-admitted" {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// ALO-V0-025: a verifier that is the running executable is refused even when
// its digest matches the request.
func TestTransportRecoveryRefusesRunningExecutable(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	selfSHA, err := regularFileSHA256(self)
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "corvint")
	if err = os.WriteFile(other, []byte("other verifier"), 0700); err != nil {
		t.Fatal(err)
	}
	otherSHA, err := regularFileSHA256(other)
	if err != nil {
		t.Fatal(err)
	}
	recovery := &transportRecovery{request: TransportRecoveryRequest{
		Base: TransportRecoveryVerifier{Path: self, SHA256: selfSHA},
		Tree: TransportRecoveryVerifier{Path: other, SHA256: otherSHA},
	}}
	if err = recovery.verifyIdentities(); err == nil || err.Error() != "transport-recovery-fixed-verifier" {
		t.Fatalf("running executable: %v", err)
	}
	recovery.request.Base = TransportRecoveryVerifier{Path: other, SHA256: selfSHA}
	if err = recovery.verifyIdentities(); err == nil || err.Error() != "transport-recovery-identity-drift" {
		t.Fatalf("drift: %v", err)
	}
}
