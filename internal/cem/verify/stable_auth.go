package verify

import (
	"crypto/ed25519"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Authentication axis values (CEM-V1-084). The axis rises above NOT_OBSERVED
// only through a consumer-supplied trusted-keys file; a key the map or its
// artifacts carry is never trusted, and no value raises executionAtCommit.
const (
	AuthenticationNotObserved = "NOT_OBSERVED"
	AuthenticationUntrusted   = "UNTRUSTED_KEY"
	AuthenticationVerified    = "VERIFIED"
	AuthenticationFailed      = "FAILED"
)

// stableHasSignedReceipt reports whether any receipt names an attestation,
// so receipts without one cost no Git read.
func stableHasSignedReceipt(d *wire.StableMap) bool {
	for _, r := range d.Receipts {
		if r.Authentication.DSSE() {
			return true
		}
	}
	return false
}

// stableAuthentication binds every dsse-ed25519 receipt to its verified
// receipt-attestation bytes and returns the authentication axis. Per receipt,
// in map order: envelope and Statement shape (attestation-malformed), the
// signature keyid against the receipt keyid (attestation-key-mismatch), the
// three subjects against the receipt digest, the plan digest and the target
// root tree (attestation-subject-mismatch), then, only for a trusted keyid,
// the Ed25519 signature over the DSSE PAE (attestation-signature-invalid).
func stableAuthentication(d *wire.StableMap, contents map[string][]byte, targetTree string, trusted map[string]ed25519.PublicKey) (string, error) {
	signed, unsigned, untrusted := 0, 0, false
	for _, r := range d.Receipts {
		if !r.Authentication.DSSE() {
			unsigned++
			continue
		}
		a, err := wire.ParseReceiptAttestation(contents[r.Authentication.EnvelopeSha256])
		if err != nil {
			return "", err
		}
		if a.KeyID != r.Authentication.KeyID {
			return "", cemcode.New("attestation-key-mismatch", "attestation keyid differs from the receipt keyid")
		}
		if a.Subjects != [3]string{r.Sha256, r.PlanSha256, targetTree} {
			return "", cemcode.New("attestation-subject-mismatch", "attestation subjects differ from the receipt, plan or target tree")
		}
		key, ok := trusted[r.Authentication.KeyID]
		if !ok {
			untrusted = true
			continue
		}
		if !ed25519.Verify(key, wire.ReceiptAttestationPAE(a.Payload), a.Signature) {
			return "", cemcode.New("attestation-signature-invalid", "attestation signature does not verify")
		}
		signed++
	}
	switch {
	case trusted == nil || signed == 0 && !untrusted:
		return AuthenticationNotObserved, nil
	case untrusted:
		return AuthenticationUntrusted, nil
	case unsigned > 0:
		return AuthenticationNotObserved, nil
	}
	return AuthenticationVerified, nil
}
