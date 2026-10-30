package localauthority

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/Beamfall/corvint/internal/wp3codec"
)

var ErrInvalid = errors.New("invalid-protected-execution")
var ErrUnavailable = errors.New("authority-unavailable")

// Canonical reuses the exact WP3 codec, including its scalar and depth bounds.
func Canonical(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, ErrInvalid
	}
	parsed, err := wp3codec.Parse(raw)
	if err != nil {
		return nil, ErrInvalid
	}
	return wp3codec.Encode(parsed)
}

func Digest(value any) (string, error) {
	raw, err := Canonical(value)
	if err != nil {
		return "", err
	}
	return BytesDigest(raw), nil
}

func BytesDigest(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// Decode accepts closed, complete canonical objects only. Re-encoding the
// typed value rejects absent members, null arrays and duplicate members too.
func Decode(raw []byte, result any) error {
	return DecodeDocument(raw, result, MaxReceiptBytes)
}

// DecodeDocument applies a profile-owned byte ceiling to the same closed
// canonical decoder. Receipt/signing callers retain MaxReceiptBytes through Decode.
func DecodeDocument(raw []byte, result any, limit int) error {
	if limit <= 0 || len(raw) > limit {
		return ErrInvalid
	}
	if err := wp3codec.Verify(raw); err != nil {
		return ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(result); err != nil {
		return ErrInvalid
	}
	canonical, err := Canonical(result)
	if err != nil {
		return ErrInvalid
	}
	if !bytes.Equal(raw, canonical) {
		return ErrInvalid
	}
	return nil
}

func SigningBytes(payload Payload) ([]byte, error) {
	raw, err := Canonical(payload)
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxReceiptBytes {
		return nil, ErrInvalid
	}
	return append([]byte(Profile+"\x00"), raw...), nil
}
