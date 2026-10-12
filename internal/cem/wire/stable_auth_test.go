package wire

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func receiptAuthCaseFile(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "protocol", "cem-1.0", "stable", "receipt-authentication", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestCEMV1084AuthenticationForms proves the closed member forms: the legacy
// sealed string and {"method":"none"} parse and re-encode byte-exactly, the
// dsse-ed25519 form keeps keyid and envelopeSha256, and the public signed map
// round-trips losslessly.
func TestCEMV1084AuthenticationForms(t *testing.T) {
	for _, form := range []string{`"NOT_OBSERVED"`, `{"method":"none"}`, `{"envelopeSha256":"` + strings.Repeat("a", 64) + `","keyid":"` + strings.Repeat("b", 64) + `","method":"dsse-ed25519"}`} {
		var a StableAuthentication
		if err := json.Unmarshal([]byte(form), &a); err != nil {
			t.Fatalf("%s: %v", form, err)
		}
		out, err := json.Marshal(a)
		if err != nil || string(out) != form {
			t.Fatalf("%s re-encodes as %s (%v)", form, out, err)
		}
		if a.DSSE() != strings.Contains(form, "dsse") {
			t.Fatalf("%s DSSE() = %v", form, a.DSSE())
		}
	}
	d, err := ParseStable(receiptAuthCaseFile(t, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := d.Receipts[0].Authentication
	if !a.DSSE() || a.KeyID != "4a5f2fe7ce8b5573a8352755fdc4468d1b4fb84630230338f3a732cebfb695dd" || a.EnvelopeSha256 != d.Artifacts[2].Sha256 || d.Artifacts[2].Kind != ReceiptAttestationKind {
		t.Fatalf("public receipt authentication = %+v, artifacts %+v", a, d.Artifacts)
	}
	whole, err := EncodeStable(d)
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseStable(whole)
	if err != nil {
		t.Fatal(err)
	}
	d.original, again.original = nil, nil
	if !reflect.DeepEqual(d, again) {
		t.Fatal("lossy signed stable round trip")
	}
}

// TestCEMV1084AuthenticationRefusals proves the refusal order (missing, then
// unknown, then invalid) and the receipt-attestation graph rules.
func TestCEMV1084AuthenticationRefusals(t *testing.T) {
	base := receiptAuthCaseFile(t, "stable.json")
	digest := strings.Repeat("c", 64)
	for _, test := range []struct {
		name   string
		mutate func(m, receipt map[string]any)
		code   string
	}{
		{"legacy-other-string", func(_, r map[string]any) { r["authentication"] = "VERIFIED" }, cemcode.InvalidField},
		{"not-an-object", func(_, r map[string]any) { r["authentication"] = []any{} }, cemcode.InvalidField},
		{"missing-method", func(_, r map[string]any) { r["authentication"] = map[string]any{} }, cemcode.MissingField},
		{"missing-before-unknown", func(_, r map[string]any) {
			r["authentication"] = map[string]any{"method": "dsse-ed25519", "keyid": digest, "extra": 1}
		}, cemcode.MissingField},
		{"unknown-before-invalid", func(_, r map[string]any) {
			r["authentication"] = map[string]any{"method": "dsse-ed25519", "keyid": "x", "envelopeSha256": digest, "extra": 1}
		}, cemcode.UnknownField},
		{"none-with-keyid", func(_, r map[string]any) { r["authentication"] = map[string]any{"method": "none", "keyid": digest} }, cemcode.UnknownField},
		{"unknown-method", func(_, r map[string]any) { r["authentication"] = map[string]any{"method": "sigstore"} }, cemcode.InvalidField},
		{"invalid-keyid", func(_, r map[string]any) {
			r["authentication"].(map[string]any)["keyid"] = strings.Repeat("A", 64)
		}, cemcode.InvalidField},
		{"unknown-artifact-kind", func(m, _ map[string]any) {
			m["artifacts"].([]any)[2].(map[string]any)["kind"] = "dsse-envelope"
		}, cemcode.InvalidField},
		{"missing-attestation-artifact", func(m, _ map[string]any) { m["artifacts"] = m["artifacts"].([]any)[:2] }, "artifact-role-closure"},
		{"attestation-wrong-kind", func(_, r map[string]any) {
			r["authentication"].(map[string]any)["envelopeSha256"] = r["planSha256"]
		}, "artifact-role-closure"},
		{"unreferenced-attestation", func(_, r map[string]any) { r["authentication"] = map[string]any{"method": "none"} }, "artifact-role-closure"},
		{"duplicate-envelope", func(m, r map[string]any) {
			second := map[string]any{}
			for k, v := range r {
				second[k] = v
			}
			second["sha256"] = strings.Repeat("7", 64)
			m["runnerReceipts"] = append(m["runnerReceipts"].([]any), second)
			link := m["criterionLinks"].([]any)[0].(map[string]any)
			link["runnerReceiptSha256s"] = append(link["runnerReceiptSha256s"].([]any), second["sha256"])
		}, "duplicate-reference"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal(base, &m); err != nil {
				t.Fatal(err)
			}
			test.mutate(m, m["runnerReceipts"].([]any)[0].(map[string]any))
			raw, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseStable(raw); cemcode.CodeOf(err) != test.code {
				t.Fatalf("ParseStable error %v, want %s", err, test.code)
			}
		})
	}
}

// TestCEMV1084ReceiptAttestationEnvelope proves the public envelope parses to
// its keyid, subjects and a signature that verifies over the DSSE PAE, and
// that shape failures are all attestation-malformed.
func TestCEMV1084ReceiptAttestationEnvelope(t *testing.T) {
	raw := receiptAuthCaseFile(t, "artifacts/receipt-attestation.json")
	att, err := ParseReceiptAttestation(raw)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := ParseTrustedKeys(receiptAuthCaseFile(t, "trusted-keys.json"))
	if err != nil {
		t.Fatal(err)
	}
	key, ok := keys[att.KeyID]
	if !ok || !ed25519.Verify(key, ReceiptAttestationPAE(att.Payload), att.Signature) {
		t.Fatalf("public attestation keyid %s does not verify under trusted-keys.json", att.KeyID)
	}
	plan := sha256.Sum256(receiptAuthCaseFile(t, "artifacts/runner-plan.json"))
	receipt := sha256.Sum256(receiptAuthCaseFile(t, "artifacts/runner-receipt.json"))
	if att.Subjects[0] != hex.EncodeToString(receipt[:]) || att.Subjects[1] != hex.EncodeToString(plan[:]) || att.Subjects[2] != "c1949cc4344b78776b6691be676a5f911d2ea27d" {
		t.Fatalf("subjects = %v", att.Subjects)
	}
	if !bytes.HasPrefix(ReceiptAttestationPAE([]byte("{}")), []byte("DSSEv1 28 application/vnd.in-toto+json 2 {}")) {
		t.Fatalf("PAE = %q", ReceiptAttestationPAE([]byte("{}")))
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	statement := func(edit func(map[string]any)) string {
		var st map[string]any
		if err := json.Unmarshal(att.Payload, &st); err != nil {
			t.Fatal(err)
		}
		edit(st)
		b, _ := json.Marshal(st)
		return base64.StdEncoding.EncodeToString(b)
	}
	for name, edit := range map[string]func(map[string]any){
		"not-json":        func(e map[string]any) { e["payload"] = base64.StdEncoding.EncodeToString([]byte("{")) },
		"payload-type":    func(e map[string]any) { e["payloadType"] = "application/json" },
		"extra-member":    func(e map[string]any) { e["extra"] = true },
		"two-signatures":  func(e map[string]any) { e["signatures"] = append(e["signatures"].([]any), e["signatures"].([]any)[0]) },
		"short-signature": func(e map[string]any) { e["signatures"].([]any)[0].(map[string]any)["sig"] = "AAAA" },
		"url-safe-payload": func(e map[string]any) {
			e["payload"] = strings.NewReplacer("+", "-", "/", "_").Replace(e["payload"].(string)) + "!"
		},
		"predicate-type": func(e map[string]any) {
			e["payload"] = statement(func(st map[string]any) { st["predicateType"] = "https://slsa.dev/provenance/v1" })
		},
		"non-empty-predicate": func(e map[string]any) {
			e["payload"] = statement(func(st map[string]any) { st["predicate"] = map[string]any{"x": 1} })
		},
		"subject-order": func(e map[string]any) {
			e["payload"] = statement(func(st map[string]any) {
				s := st["subject"].([]any)
				s[0], s[1] = s[1], s[0]
			})
		},
		"tree-as-sha256": func(e map[string]any) {
			e["payload"] = statement(func(st map[string]any) {
				st["subject"].([]any)[2] = map[string]any{"name": "target-tree", "digest": map[string]any{"sha256": strings.Repeat("d", 64)}}
			})
		},
	} {
		t.Run(name, func(t *testing.T) {
			var e map[string]any
			b, _ := json.Marshal(env)
			_ = json.Unmarshal(b, &e)
			edit(e)
			mutated, _ := json.Marshal(e)
			if _, err := ParseReceiptAttestation(mutated); cemcode.CodeOf(err) != "attestation-malformed" {
				t.Fatalf("ParseReceiptAttestation error %v, want attestation-malformed", err)
			}
		})
	}
}

// TestCEMV1084TrustedKeysFile proves the closed consumer trusted-keys file:
// the public files parse to their single key, and any other shape is
// invalid-arguments.
func TestCEMV1084TrustedKeysFile(t *testing.T) {
	keys, err := ParseTrustedKeys(receiptAuthCaseFile(t, "trusted-keys.json"))
	if err != nil || len(keys) != 1 {
		t.Fatalf("trusted-keys.json = %v, %v", keys, err)
	}
	for id, key := range keys {
		if Ed25519KeyID(key) != id {
			t.Fatalf("keyid %s is not the digest of its key", id)
		}
	}
	good := string(receiptAuthCaseFile(t, "trusted-keys.json"))
	var doc struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.Unmarshal([]byte(good), &doc); err != nil {
		t.Fatal(err)
	}
	entry, _ := json.Marshal(doc.Keys[0])
	for name, raw := range map[string]string{
		"empty-keys":      `{"profile":"corvint-trusted-keys/1","keys":[]}`,
		"other-profile":   `{"profile":"corvint-trusted-keys/2","keys":[` + string(entry) + `]}`,
		"extra-member":    `{"profile":"corvint-trusted-keys/1","keys":[` + string(entry) + `],"x":1}`,
		"duplicate-keyid": `{"profile":"corvint-trusted-keys/1","keys":[` + string(entry) + `,` + string(entry) + `]}`,
		"wrong-keyid":     strings.Replace(good, doc.Keys[0]["keyid"], strings.Repeat("0", 64), 1),
		"short-key":       strings.Replace(good, doc.Keys[0]["ed25519PublicKey"], "AAAA", 1),
		"oversized":       good + strings.Repeat(" ", MaxTrustedKeysBytes),
	} {
		if _, err := ParseTrustedKeys([]byte(raw)); cemcode.CodeOf(err) != "invalid-arguments" {
			t.Errorf("%s: error %v, want invalid-arguments", name, err)
		}
	}
}
