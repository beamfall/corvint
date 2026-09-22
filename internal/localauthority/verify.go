package localauthority

import (
	"crypto/ed25519"
	"encoding/hex"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Verify authenticates a complete execution observation, including PASS or FAIL.
// It requires independent current admission; authentication does not mean checks passed.
func Verify(receipt Receipt, expected Enrollment, policy PolicySnapshot, now time.Time) error {
	if !policy.Accepted || !policy.Current || policy.Fixture || policy.Revoked {
		return ErrUnavailable
	}
	return verify(receipt, expected, policy, now)
}

// VerifyFixture exercises the same relation with explicitly unadmitted test
// keys. It cannot produce an authoritative result and is never called by the
// live verifier or command adapter.
func VerifyFixture(receipt Receipt, expected Enrollment, policy PolicySnapshot, now time.Time) error {
	if !policy.Fixture || !policy.Current || policy.Revoked {
		return ErrUnavailable
	}
	return verify(receipt, expected, policy, now)
}

func verify(receipt Receipt, expected Enrollment, policy PolicySnapshot, now time.Time) error {
	if err := ValidateEnrollment(expected); err != nil {
		return err
	}
	if !reflect.DeepEqual(receipt.Payload.Enrollment, expected) {
		return ErrInvalid
	}
	if policy.RepositoryID != expected.RepositoryID || policy.PolicySHA256 != expected.PolicySHA256 {
		return ErrInvalid
	}
	if policy.RootID != expected.RootID || policy.Epoch != expected.Epoch || policy.Generation != expected.Generation || policy.Audience != expected.Audience {
		return ErrInvalid
	}
	generation, ok := decimal(policy.Generation)
	if !ok {
		return ErrInvalid
	}
	floor, ok := decimal(policy.MinimumGeneration)
	if !ok || generation < floor {
		return ErrInvalid
	}
	if !reflect.DeepEqual(policy.Checks, expected.Checks) {
		return ErrInvalid
	}
	if len(policy.PublicKey) != ed25519.PublicKeySize {
		return ErrUnavailable
	}
	if err := checkTime(receipt.Payload, now); err != nil {
		return err
	}
	if receipt.Payload.Cleanup != "COMPLETE" || receipt.Payload.ExitCode != "0" {
		return ErrInvalid
	}
	if len(receipt.Payload.Rows) != len(expected.Checks) {
		return ErrInvalid
	}
	for index, check := range expected.Checks {
		row := receipt.Payload.Rows[index]
		if row.Check != check || (row.Status != "PASS" && row.Status != "FAIL") {
			return ErrInvalid
		}
	}
	signed, err := SigningBytes(receipt.Payload)
	if err != nil {
		return err
	}
	signature, err := hex.DecodeString(receipt.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize || strings.ToLower(receipt.Signature) != receipt.Signature {
		return ErrInvalid
	}
	if !ed25519.Verify(policy.PublicKey, signed, signature) {
		return ErrInvalid
	}
	digest, err := Digest(receipt)
	if err != nil {
		return err
	}
	if policy.TerminalSHA256[expected.Nonce] != digest {
		return ErrInvalid
	}
	return nil
}

func ValidateEnrollment(e Enrollment) error {
	if !hexSize(e.RepositoryID, 64) || !hexSize(e.PolicySHA256, 64) {
		return ErrInvalid
	}
	if e.Profile != Profile || e.Selection != SelectionMode {
		return ErrInvalid
	}
	if !hexSize(e.Nonce, 64) || e.Audience == "" || e.RootID == "" {
		return ErrInvalid
	}
	if _, ok := decimal(e.Epoch); !ok {
		return ErrInvalid
	}
	if _, ok := decimal(e.Generation); !ok {
		return ErrInvalid
	}
	if !objectID(e.Binding.Base) || !objectID(e.Binding.Target) || !objectID(e.Binding.Tree) {
		return ErrInvalid
	}
	for _, digest := range []string{e.Binding.CEMSHA256, e.Binding.OCMSHA256, e.Binding.SelectionSHA256, e.Binding.SourceSHA256, e.Binding.RecipeSHA256, e.Binding.WasmSHA256, e.Binding.WorkerSHA256} {
		if !hexSize(digest, 64) {
			return ErrInvalid
		}
	}
	if len(e.Checks) == 0 || len(e.Checks) > 256 {
		return ErrInvalid
	}
	previous := ""
	for _, check := range e.Checks {
		if check.ClaimSelector == "" || check.ID <= previous || check.Profile != CheckProfile || !hexSize(check.DriverSHA256, 64) || check.DriverPath == "" || check.DriverUnit == "" || check.Invocation == "" || check.Subject == "" {
			return ErrInvalid
		}
		previous = check.ID
	}
	digest, err := Digest(e.Checks)
	if err != nil || digest != e.Binding.SelectionSHA256 {
		return ErrInvalid
	}
	return nil
}

func checkTime(p Payload, now time.Time) error {
	issued, ok := decimal(p.Enrollment.IssuedAt)
	if !ok {
		return ErrInvalid
	}
	expires, ok := decimal(p.Enrollment.ExpiresAt)
	if !ok {
		return ErrInvalid
	}
	completed, ok := decimal(p.CompletedAt)
	if !ok {
		return ErrInvalid
	}
	if expires < issued || expires-issued > 86400 || completed < issued || completed > expires {
		return ErrInvalid
	}
	if now.Unix() < 0 || uint64(now.Unix()) < completed || uint64(now.Unix()) > expires {
		return ErrInvalid
	}
	return nil
}

func decimal(text string) (uint64, bool) {
	n, err := strconv.ParseUint(text, 10, 64)
	return n, err == nil && strconv.FormatUint(n, 10) == text
}
func hexSize(s string, n int) bool {
	if len(s) != n || strings.ToLower(s) != s {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
func objectID(s string) bool { return hexSize(s, 40) || hexSize(s, 64) }
