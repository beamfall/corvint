package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var verdictHex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// handVerdictDigest recomputes CEM-V1-066 without verdictDigest:
// encoding/json sorts map keys and, without HTML escaping, writes the same
// bytes as the canonical JSON value for these ASCII results.
func handVerdictDigest(t *testing.T, encoded []byte) string {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"runtime", "repositoryEnvelope", "verdictSha256"} {
		delete(object, key)
	}
	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(object); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bytes.TrimSuffix(canonical.Bytes(), []byte("\n")))
	return hex.EncodeToString(sum[:])
}

func emittedVerdict(t *testing.T, name string, r stableResult) string {
	t.Helper()
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil || len(object) != 23 {
		t.Fatalf("%s: result shape %s", name, encoded)
	}
	var got string
	if err := json.Unmarshal(object["verdictSha256"], &got); err != nil || !verdictHex.MatchString(got) || got != handVerdictDigest(t, encoded) {
		t.Fatalf("%s: verdictSha256 %s, hand %s", name, object["verdictSha256"], handVerdictDigest(t, encoded))
	}
	return got
}

// TestCEMV1066PortableSealedVerdictDigests computes the portable digest of
// every sealed case, envelope cell and public no-Tasks ACCEPT result. The
// sealed values are the native verifier's captured digests, so equality here
// is native/portable agreement on every public case (K6).
func TestCEMV1066PortableSealedVerdictDigests(t *testing.T) {
	check := func(name string, result map[string]any) {
		t.Helper()
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		got, err := verdictDigest(encoded)
		if err != nil || got != result["verdictSha256"] || got != handVerdictDigest(t, encoded) {
			t.Errorf("%s: portable %s (%v), sealed %v", name, got, err, result["verdictSha256"])
		}
	}
	var cases, cells struct {
		Cases []struct {
			ID             string         `json:"id"`
			ExpectedResult map[string]any `json:"expectedResult"`
		} `json:"cases"`
	}
	s0eJSON(t, "FULL-RESULT-CASES.json", &cases)
	s0eJSON(t, "inherited/s0e/REPOSITORY-ENVELOPE.cells.json", &cells)
	if len(cases.Cases) != 56 || len(cells.Cases) == 0 {
		t.Fatalf("%d cases, %d cells", len(cases.Cases), len(cells.Cases))
	}
	for _, c := range cases.Cases {
		check(c.ID, c.ExpectedResult)
	}
	for _, c := range cells.Cases {
		check("cell "+c.ID, c.ExpectedResult)
	}
	for _, name := range []string{"neutral-criterion", "receipt-authentication"} {
		raw, err := os.ReadFile(filepath.Join(stableCaseFiles(t, name), "EXPECTED.json"))
		if err != nil {
			t.Fatal(err)
		}
		var want neutralExpected
		if err := json.Unmarshal(raw, &want); err != nil {
			t.Fatal(err)
		}
		check(name, want.Accept.Result)
	}
}

// TestCEMV1066PortableVerdictDigestCoversOnlyTheVerdict changes each
// environment member, which keeps the digest, and each verdict member, which
// changes it.
func TestCEMV1066PortableVerdictDigestCoversOnlyTheVerdict(t *testing.T) {
	r := newStableResult(strings.Repeat("1", 40), strings.Repeat("2", 40), stableLegacyEnvelope)
	before := emittedVerdict(t, "refusal", r)
	moved := r
	moved.Runtime.GoVersion = "go1.27.1"
	if moved.Runtime.GoVersion == r.Runtime.GoVersion {
		moved.Runtime.GoVersion = "go1.27.2"
	}
	moved.RepositoryEnvelope = "other-envelope"
	moved.VerdictSHA256 = "not-a-digest"
	if got := emittedVerdict(t, "environment", moved); got != before {
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
}

// TestCEMV1066PortableEveryOutcomeCarriesVerdictDigest checks ACCEPT, REJECT,
// drift REJECT and UNSUPPORTED results of one fixture: each carries its own
// digest of its verdict.
func TestCEMV1066PortableEveryOutcomeCarriesVerdictDigest(t *testing.T) {
	f := newCitationFixture(t, patch24Tree())
	m := f.stableMap(t, patch24Careful())
	accepted, _, _, _ := f.run(t, m)
	bad := m
	bad.PatchSHA256 = strings.Repeat("0", 64)
	raw, _ := json.Marshal(bad)
	rejected, _ := runStable(context.Background(), f.repo, raw, f.base, f.target, t.TempDir())
	raw, _ = json.Marshal(m)
	refused, _ := runStable(context.Background(), "relative/repository", raw, f.base, f.target, t.TempDir())
	tree := patch24Tree()
	tree.Target["docs/notes.md"] = strings.Replace(tree.Target["docs/notes.md"], "Note 16: keep the retry budget bounded.\n", "", 1)
	g := newCitationFixture(t, tree)
	raw, _ = json.Marshal(g.stableMap(t, patch24Careful()))
	drifted, _ := runStable(context.Background(), g.repo, raw, g.base, g.target, t.TempDir())
	seen := map[string]string{}
	for _, c := range []struct {
		name, outcome string
		r             stableResult
	}{
		{"accept", "ACCEPT", accepted},
		{"reject", "REJECT", rejected},
		{"drift", "REJECT", drifted},
		{"refusal", "UNSUPPORTED", refused},
	} {
		if c.r.Outcome != c.outcome {
			t.Fatalf("%s: outcome %s, want %s", c.name, c.r.Outcome, c.outcome)
		}
		digest := emittedVerdict(t, c.name, c.r)
		if other, ok := seen[digest]; ok {
			t.Errorf("%s and %s share verdict digest %s", c.name, other, digest)
		}
		seen[digest] = c.name
	}
}

// TestStableResultEnvelopeFollowsThePath proves that each portable stable path
// labels its result with its own repository envelope rather than a default:
// the legacy verify-stable (R1) path with primary-clean-config-bounded/1 and
// the canonical path with canonical-repository-bounded/1, on refusals too
// (V1-2142).
func TestStableResultEnvelopeFollowsThePath(t *testing.T) {
	ctx := context.Background()
	repo, base, target := t.TempDir(), strings.Repeat("1", 40), strings.Repeat("2", 40)
	legacy, _ := runStable(ctx, repo, []byte("{"), base, target, t.TempDir())
	canonical, _ := runStableCanonical(ctx, repo, []byte("{"), base, target, t.TempDir(), nil)
	if legacy.RepositoryEnvelope != "primary-clean-config-bounded/1" {
		t.Fatalf("legacy envelope %q", legacy.RepositoryEnvelope)
	}
	if canonical.RepositoryEnvelope != "canonical-repository-bounded/1" {
		t.Fatalf("canonical envelope %q", canonical.RepositoryEnvelope)
	}
	legacyCLI, _ := executeStableCLI(ctx, []string{"--bogus"})
	if legacyCLI.RepositoryEnvelope != stableLegacyEnvelope {
		t.Fatalf("legacy CLI refusal envelope %q", legacyCLI.RepositoryEnvelope)
	}
}
