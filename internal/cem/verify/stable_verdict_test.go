package verify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

var verdictHex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// handVerdictDigest recomputes CEM-V1-066 without the verifier's code path:
// encoding/json sorts map keys and, without HTML escaping, writes the same
// bytes as canonical-json-value for these ASCII results.
func handVerdictDigest(t *testing.T, encoded []byte) string {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		t.Fatal(err)
	}
	delete(object, "runtime")
	delete(object, "repositoryEnvelope")
	delete(object, "verdictSha256")
	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(object); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bytes.TrimSuffix(canonical.Bytes(), []byte("\n")))
	return hex.EncodeToString(sum[:])
}

// requireVerdictDigest checks one emitted result: verdictSha256 is lowercase
// hex, equals the hand-computed digest, and equals VerdictDigest.
func requireVerdictDigest(t *testing.T, name string, r StableResult) string {
	t.Helper()
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var emitted struct {
		VerdictSha256 *string `json:"verdictSha256"`
	}
	if err := json.Unmarshal(encoded, &emitted); err != nil || emitted.VerdictSha256 == nil || !verdictHex.MatchString(*emitted.VerdictSha256) {
		t.Fatalf("%s: verdictSha256 %v is not lowercase hex sha256 (%v)", name, emitted.VerdictSha256, err)
	}
	digest, err := r.VerdictDigest()
	if err != nil || digest != *emitted.VerdictSha256 || digest != handVerdictDigest(t, encoded) {
		t.Fatalf("%s: emitted %s, VerdictDigest %s (%v), hand %s", name, *emitted.VerdictSha256, digest, err, handVerdictDigest(t, encoded))
	}
	return digest
}

// TestCEMV1066SealedVerdictDigests recomputes verdictSha256 by hand for every
// sealed case, every envelope cell and both public no-Tasks cases.
func TestCEMV1066SealedVerdictDigests(t *testing.T) {
	t.Parallel()
	packet := loadS0EPacket(t)
	check := func(name string, result map[string]any) {
		t.Helper()
		got, _ := result["verdictSha256"].(string)
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		if !verdictHex.MatchString(got) || got != handVerdictDigest(t, encoded) {
			t.Errorf("%s: sealed verdictSha256 %q, hand %s", name, got, handVerdictDigest(t, encoded))
		}
	}
	outcomes := map[string]int{}
	for _, c := range packet.cases {
		check(c.ID, c.ExpectedResult)
		outcomes[c.ExpectedResult["outcome"].(string)]++
	}
	if outcomes["ACCEPT"] == 0 || outcomes["UNSUPPORTED"] == 0 {
		t.Fatalf("sealed outcomes %v; want ACCEPT and UNSUPPORTED", outcomes)
	}
	raw, err := os.ReadFile(filepath.Join(packet.root, "inherited", "s0e", "REPOSITORY-ENVELOPE.cells.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cells struct {
		Cases []struct {
			ID             string         `json:"id"`
			ExpectedResult map[string]any `json:"expectedResult"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &cells); err != nil || len(cells.Cases) == 0 {
		t.Fatalf("cells: %v", err)
	}
	for _, c := range cells.Cases {
		check("cell "+c.ID, c.ExpectedResult)
	}
	check("neutral-criterion", loadNeutralCase(t).expect.Accept.Result)
	check("receipt-authentication", loadReceiptAuthCase(t).expect.Accept.Result)
}

// TestCEMV1066VerdictDigestCoversOnlyTheVerdict changes each environment
// member, which keeps the digest, and each verdict member, which changes it.
func TestCEMV1066VerdictDigestCoversOnlyTheVerdict(t *testing.T) {
	t.Parallel()
	r := NewStableResult(StableOptions{ExpectedBase: "1111111111111111111111111111111111111111", Target: "2222222222222222222222222222222222222222"})
	r.Refuse("binding", "verification-timeout", true)
	before := requireVerdictDigest(t, "refusal", r)

	moved := r
	moved.Runtime.GoVersion = "go1.27.1"
	if moved.Runtime.GoVersion == r.Runtime.GoVersion {
		moved.Runtime.GoVersion = "go1.27.2"
	}
	moved.Runtime.StructuralQualification = "other"
	moved.RepositoryEnvelope = "other-envelope"
	moved.VerdictSha256 = "not-a-digest"
	if got := requireVerdictDigest(t, "environment", moved); got != before {
		t.Fatalf("environment change moved the digest: %s -> %s", before, got)
	}

	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	if len(object) != 23 {
		t.Fatalf("stable result has %d keys, want 23", len(object))
	}
	for key := range object {
		mutated := map[string]any{}
		for k, v := range object {
			mutated[k] = v
		}
		mutated[key] = map[string]any{"mutated": key}
		raw, err := json.Marshal(mutated)
		if err != nil {
			t.Fatal(err)
		}
		got, err := verdictDigest(raw)
		if err != nil {
			t.Fatal(err)
		}
		environment := key == "runtime" || key == "repositoryEnvelope" || key == "verdictSha256"
		if environment != (got == before) {
			t.Errorf("changing %s: digest moved=%v, environment=%v", key, got != before, environment)
		}
	}
	accepted := r
	accepted.Outcome = "ACCEPT"
	if requireVerdictDigest(t, "outcome", accepted) == before {
		t.Fatal("changing outcome kept the digest")
	}
}

// TestCEMV1066EveryOutcomeCarriesVerdictDigest runs both public no-Tasks
// cases and their public mutations: ACCEPT and every refusal carry the digest,
// and the ACCEPT digests equal the public expectations.
func TestCEMV1066EveryOutcomeCarriesVerdictDigest(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	n := loadNeutralCase(t)
	repo, base, target := n.materialize(t)
	options := StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: n.root}
	accepted, _ := Stable(context.Background(), n.raw, options)
	if got := requireVerdictDigest(t, "neutral-criterion", accepted); got != n.expect.Accept.Result["verdictSha256"] {
		t.Fatalf("neutral-criterion verdictSha256 %s, public expectation %v", got, n.expect.Accept.Result["verdictSha256"])
	}
	for _, r := range n.expect.Refusals {
		var m, criterion map[string]any
		if err := json.Unmarshal(n.raw, &m); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(r.Criterion, &criterion); err != nil {
			t.Fatal(err)
		}
		m["criterionBindings"] = []any{criterion}
		if r.RebindLink {
			m["criterionLinks"].([]any)[0].(map[string]any)["criterionId"] = criterion["id"]
		}
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		result, _ := Stable(context.Background(), raw, options)
		if result.Outcome != r.Outcome {
			t.Fatalf("neutral-criterion/%s outcome %s, want %s", r.Name, result.Outcome, r.Outcome)
		}
		requireVerdictDigest(t, "neutral-criterion/"+r.Name, result)
	}

	c := loadReceiptAuthCase(t)
	repo, base, target = c.materialize(t)
	options = StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: c.root}
	for _, m := range append(append([]receiptAuthMutation(nil), c.expect.AxisCases...), c.expect.Refusals...) {
		o := options
		o.TrustedKeys = c.trusted(t, m.TrustedKeys)
		result, _ := Stable(context.Background(), c.mutate(t, m.Set), o)
		if result.Outcome != m.Outcome {
			t.Fatalf("receipt-authentication/%s outcome %s, want %s", m.Name, result.Outcome, m.Outcome)
		}
		requireVerdictDigest(t, "receipt-authentication/"+m.Name, result)
	}
}
