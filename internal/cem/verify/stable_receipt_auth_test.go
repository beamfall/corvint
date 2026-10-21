package verify

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// receiptAuthCaseRoot is the public receipt-authentication case of CEM-V1-084.
const receiptAuthCaseRoot = "protocol/cem-1.0/stable/receipt-authentication"

// receiptAuthSeedText derives each TEST-ONLY Ed25519 seed as the SHA-256 of
// this text plus the key name. The keys are public fixtures, never trusted
// outside the case.
const receiptAuthSeedText = "corvint CEM-V1-084 TEST-ONLY receipt attestation key "

func receiptAuthKey(name string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte(receiptAuthSeedText + name))
	return ed25519.NewKeyFromSeed(seed[:])
}

func receiptAuthJSON(t *testing.T, v any) []byte {
	t.Helper()
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

type receiptAuthSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// receiptAuthStatement is the in-toto Statement v1 payload; it carries no
// trailing newline because the signature covers exactly these bytes.
func receiptAuthStatement(t *testing.T, receipt, plan, tree string) []byte {
	t.Helper()
	b := receiptAuthJSON(t, struct {
		Type          string               `json:"_type"`
		Subject       []receiptAuthSubject `json:"subject"`
		PredicateType string               `json:"predicateType"`
		Predicate     struct{}             `json:"predicate"`
	}{wire.ReceiptAttestationStatementType, []receiptAuthSubject{
		{"runner-receipt", map[string]string{"sha256": receipt}},
		{"runner-plan", map[string]string{"sha256": plan}},
		{"target-tree", map[string]string{"gitTree": tree}},
	}, wire.ReceiptAttestationPredicateType, struct{}{}})
	return bytes.TrimSuffix(b, []byte("\n"))
}

func receiptAuthEnvelope(t *testing.T, payloadType string, payload []byte, keyID string, sig []byte) []byte {
	t.Helper()
	type signature struct {
		KeyID string `json:"keyid"`
		Sig   string `json:"sig"`
	}
	return receiptAuthJSON(t, struct {
		Payload     string      `json:"payload"`
		PayloadType string      `json:"payloadType"`
		Signatures  []signature `json:"signatures"`
	}{base64.StdEncoding.EncodeToString(payload), payloadType, []signature{{keyID, base64.StdEncoding.EncodeToString(sig)}}})
}

func receiptAuthSign(t *testing.T, key ed25519.PrivateKey, payload []byte) []byte {
	t.Helper()
	pub := key.Public().(ed25519.PublicKey)
	return receiptAuthEnvelope(t, wire.ReceiptAttestationPayloadType, payload, wire.Ed25519KeyID(pub), ed25519.Sign(key, wire.ReceiptAttestationPAE(payload)))
}

func receiptAuthTrustedKeys(t *testing.T, keys ...ed25519.PrivateKey) []byte {
	t.Helper()
	type entry struct {
		KeyID string `json:"keyid"`
		Key   string `json:"ed25519PublicKey"`
	}
	entries := []entry{}
	for _, k := range keys {
		pub := k.Public().(ed25519.PublicKey)
		entries = append(entries, entry{wire.Ed25519KeyID(pub), base64.StdEncoding.EncodeToString(pub)})
	}
	return receiptAuthJSON(t, struct {
		Profile string  `json:"profile"`
		Keys    []entry `json:"keys"`
	}{wire.TrustedKeysProfile, entries})
}

// receiptAuthFixtures derives every byte-exact file of the public case that
// carries a key, a digest or a signature from the neutral-criterion case,
// the fixed seeds and the recipe's base and target root trees.
func receiptAuthFixtures(t *testing.T, top, baseTree, targetTree string) map[string][]byte {
	t.Helper()
	neutral := filepath.Join(top, neutralCaseRoot)
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(neutral, name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	out := map[string][]byte{}
	for _, name := range []string{"repository.recipe.json", "artifacts/runner-plan.json", "artifacts/runner-receipt.json"} {
		out[name] = read(name)
	}
	receipt, plan := stableDigest(out["artifacts/runner-receipt.json"]), stableDigest(out["artifacts/runner-plan.json"])
	a, b := receiptAuthKey("signer-a"), receiptAuthKey("signer-b")
	keyA := wire.Ed25519KeyID(a.Public().(ed25519.PublicKey))
	payload := receiptAuthStatement(t, receipt, plan, targetTree)
	out["artifacts/receipt-attestation.json"] = receiptAuthSign(t, a, payload)
	out["artifacts/attestation-other-tree.json"] = receiptAuthSign(t, a, receiptAuthStatement(t, receipt, plan, baseTree))
	out["artifacts/attestation-other-key.json"] = receiptAuthSign(t, b, payload)
	wrong := ed25519.Sign(a, wire.ReceiptAttestationPAE(receiptAuthStatement(t, receipt, plan, baseTree)))
	out["artifacts/attestation-bad-signature.json"] = receiptAuthEnvelope(t, wire.ReceiptAttestationPayloadType, payload, keyA, wrong)
	out["artifacts/attestation-malformed.json"] = receiptAuthEnvelope(t, "application/json", payload, keyA, ed25519.Sign(a, wire.ReceiptAttestationPAE(payload)))
	out["trusted-keys.json"] = receiptAuthTrustedKeys(t, a)
	out["other-trusted-keys.json"] = receiptAuthTrustedKeys(t, b)
	type testKey struct {
		Name       string `json:"name"`
		Seed       string `json:"seedDerivation"`
		KeyID      string `json:"keyid"`
		PublicKey  string `json:"ed25519PublicKey"`
		TrustedVia string `json:"trustedKeysFile"`
	}
	keys := []testKey{}
	for _, k := range []struct{ name, file string }{{"signer-a", "trusted-keys.json"}, {"signer-b", "other-trusted-keys.json"}} {
		pub := receiptAuthKey(k.name).Public().(ed25519.PublicKey)
		keys = append(keys, testKey{k.name, "sha256(utf8(\"" + receiptAuthSeedText + k.name + "\"))", wire.Ed25519KeyID(pub), base64.StdEncoding.EncodeToString(pub), k.file})
	}
	out["test-only-keys.json"] = receiptAuthJSON(t, struct {
		Profile string    `json:"profile"`
		Warning string    `json:"warning"`
		Keys    []testKey `json:"keys"`
	}{"cem-stable-test-only-keys/1", "TEST ONLY: these Ed25519 key pairs are derived from public seeds; anyone can sign with them, so no verifier outside this case may trust them", keys})
	var m map[string]any
	d := json.NewDecoder(bytes.NewReader(read("stable.json")))
	d.UseNumber()
	if err := d.Decode(&m); err != nil {
		t.Fatal(err)
	}
	envelope := stableDigest(out["artifacts/receipt-attestation.json"])
	m["runnerReceipts"].([]any)[0].(map[string]any)["authentication"] = map[string]any{"method": wire.StableAuthenticationDSSE, "keyid": keyA, "envelopeSha256": envelope}
	m["artifacts"] = append(m["artifacts"].([]any), map[string]any{"kind": wire.ReceiptAttestationKind, "path": "artifacts/receipt-attestation.json", "sha256": envelope})
	out["stable.json"] = receiptAuthJSON(t, m)
	return out
}

type receiptAuthCase struct {
	root   string
	raw    []byte
	expect struct {
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
		BaseTree       string `json:"baseTree"`
		TargetTree     string `json:"targetTree"`
		Accept         struct {
			TrustedKeys string         `json:"trustedKeys"`
			Exit        int            `json:"exit"`
			Result      map[string]any `json:"resultExceptRuntime"`
		} `json:"accept"`
		AxisCases []receiptAuthMutation `json:"axisCases"`
		Refusals  []receiptAuthMutation `json:"refusals"`
	}
}

// receiptAuthMutation replaces whole top-level map members with Set and runs
// with TrustedKeys (a case file name, or none).
type receiptAuthMutation struct {
	Name           string                     `json:"name"`
	Set            map[string]json.RawMessage `json:"set"`
	TrustedKeys    string                     `json:"trustedKeys"`
	Exit           int                        `json:"exit"`
	Outcome        string                     `json:"outcome"`
	Stage          string                     `json:"stage"`
	Code           string                     `json:"code"`
	IssueCode      string                     `json:"issueCode"`
	Authentication string                     `json:"authentication"`
}

func loadReceiptAuthCase(t *testing.T) *receiptAuthCase {
	t.Helper()
	top := repositoryRoot(t)
	c := &receiptAuthCase{root: filepath.Join(top, receiptAuthCaseRoot)}
	checkStableCaseManifest(t, top, "receipt-authentication/")
	var err error
	if c.raw, err = os.ReadFile(filepath.Join(c.root, "stable.json")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(c.root, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &c.expect); err != nil {
		t.Fatal(err)
	}
	return c
}

func (c *receiptAuthCase) materialize(t *testing.T) (repo, base, target string) {
	t.Helper()
	n := &neutralCase{root: c.root}
	n.expect.BaseRevision, n.expect.TargetRevision = c.expect.BaseRevision, c.expect.TargetRevision
	repo, base, target = n.materialize(t)
	if tree := stableRepositoryGit(t, repo, "", "rev-parse", base+"^{tree}"); tree != c.expect.BaseTree {
		t.Fatalf("base tree %s, want %s", tree, c.expect.BaseTree)
	}
	if tree := stableRepositoryGit(t, repo, "", "rev-parse", target+"^{tree}"); tree != c.expect.TargetTree {
		t.Fatalf("target tree %s, want %s", tree, c.expect.TargetTree)
	}
	return repo, base, target
}

func (c *receiptAuthCase) trusted(t *testing.T, name string) map[string]ed25519.PublicKey {
	t.Helper()
	if name == "" {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(c.root, name))
	if err != nil {
		t.Fatal(err)
	}
	keys, err := wire.ParseTrustedKeys(b)
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

func (c *receiptAuthCase) mutate(t *testing.T, set map[string]json.RawMessage) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(c.raw, &m); err != nil {
		t.Fatal(err)
	}
	for k, v := range set {
		m[k] = v
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestCEMV1084ReceiptAuthenticationFixtures proves every key, attestation,
// trusted-keys file and the map of the public case are exactly what the fixed
// TEST-ONLY seeds and the recipe trees derive. Set
// CORVINT_CEM_V1084_REGENERATE=1 to rewrite them.
func TestCEMV1084ReceiptAuthenticationFixtures(t *testing.T) {
	t.Parallel()
	c := loadReceiptAuthCase(t)
	want := receiptAuthFixtures(t, repositoryRoot(t), c.expect.BaseTree, c.expect.TargetTree)
	regenerate := os.Getenv("CORVINT_CEM_V1084_REGENERATE") == "1"
	for name, b := range want {
		path := filepath.Join(c.root, filepath.FromSlash(name))
		if regenerate {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, b, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, b) {
			t.Errorf("%s differs from its seeded derivation (err %v)", name, err)
		}
	}
}

// TestCEMV1084ReceiptAuthenticationPublicCase runs the public case: the signed
// map is VERIFIED only under the consumer trusted-keys file naming its key,
// stays NOT_OBSERVED without one and UNTRUSTED_KEY under another key, never
// raises executionAtCommit, and every public refusal fails at its recorded
// stage and code (CEM-V1-084).
func TestCEMV1084ReceiptAuthenticationPublicCase(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	c := loadReceiptAuthCase(t)
	repo, base, target := c.materialize(t)
	options := StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: c.root, TrustedKeys: c.trusted(t, c.expect.Accept.TrustedKeys)}
	result, exit := Stable(context.Background(), c.raw, options)
	if got := neutralResult(t, result); exit != c.expect.Accept.Exit || !reflect.DeepEqual(got, c.expect.Accept.Result) {
		t.Fatalf("Stable exit=%d result=%s, want the public expected ACCEPT", exit, stableResultDebug(result))
	}
	if result.Axes["authentication"] != AuthenticationVerified || result.Axes["executionAtCommit"] != "NOT_OBSERVED" {
		t.Fatalf("axes = %v, want VERIFIED authentication and NOT_OBSERVED executionAtCommit", result.Axes)
	}
	run := func(m receiptAuthMutation) (StableResult, int) {
		o := options
		o.TrustedKeys = c.trusted(t, m.TrustedKeys)
		return Stable(context.Background(), c.mutate(t, m.Set), o)
	}
	for _, m := range c.expect.AxisCases {
		t.Run(m.Name, func(t *testing.T) {
			result, exit := run(m)
			if exit != m.Exit || result.Outcome != m.Outcome || result.Axes["authentication"] != m.Authentication || result.Axes["executionAtCommit"] != "NOT_OBSERVED" {
				t.Fatalf("Stable exit=%d result=%s, want %d %s authentication %s", exit, stableResultDebug(result), m.Exit, m.Outcome, m.Authentication)
			}
		})
	}
	for _, m := range c.expect.Refusals {
		t.Run(m.Name, func(t *testing.T) {
			result, exit := run(m)
			code := ""
			if result.Code != nil {
				code = *result.Code
			}
			issues := ""
			if len(result.IssueCodes) == 1 {
				issues = result.IssueCodes[0]
			}
			if exit != m.Exit || result.Outcome != m.Outcome || result.Stage != m.Stage || code != m.Code || issues != m.IssueCode || result.Axes["authentication"] != m.Authentication {
				t.Fatalf("Stable exit=%d result=%s, want %d %s %s code %q issue %q authentication %s", exit, stableResultDebug(result), m.Exit, m.Outcome, m.Stage, m.Code, m.IssueCode, m.Authentication)
			}
		})
	}
}
