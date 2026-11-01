package main

import (
	"errors"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// V1-3358: json.Decoder.More reports false before a trailing closing
// delimiter, so these strict single-value decodes must refuse one.
func TestStrictDecodesRefuseTrailingDelimiter(t *testing.T) {
	if _, err := decodeJSONObject([]byte(`{"a":1}` + "\n")); err != nil {
		t.Fatalf("valid object refused: %v", err)
	}
	refusal := `{"schema":"` + negateRefusalSchema + `","code":"step-not-negatable","message":"m"}`
	var typed *gokernel.Error
	if err := decodeNegateRefusal([]byte(refusal)); !errors.As(err, &typed) || typed.Code != "step-not-negatable" {
		t.Fatalf("valid refusal = %v", err)
	}
	for _, trailer := range []string{"}", "]"} {
		if _, err := decodeJSONObject([]byte(`{"a":1}` + trailer)); err == nil {
			t.Errorf("object with trailing %q accepted", trailer)
		}
		if err := decodeNegateRefusal([]byte(refusal + trailer)); !errors.As(err, &typed) || typed.Code != "negate-provider-failed" {
			t.Errorf("refusal with trailing %q = %v, want negate-provider-failed", trailer, err)
		}
	}
}
