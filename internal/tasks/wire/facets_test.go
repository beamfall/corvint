package wire

import (
	"bytes"
	"strings"
	"testing"
)

// CAL-V0-206 (amendment A26): the facets member is absent-only and optional.
// A result without it keeps its earlier bytes, a result with it round-trips,
// and a non-object facets member is refused.
func TestCALV0206_FacetsMemberIsAbsentOnlyAndOptional(t *testing.T) {
	plain := &Result{Command: []string{"roadmap"}, Outcome: OutcomeOK}
	raw, err := plain.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"facets"`)) {
		t.Fatalf("facets rendered without a summary: %s", raw)
	}
	if got, err := DecodeResult(raw); err != nil || got.Facets != nil {
		t.Fatalf("earlier bytes: %v %+v", err, got)
	}
	summary := ObjectValue(NewObject().Set("total", String("3")))
	with := &Result{Command: []string{"roadmap"}, Outcome: OutcomeOK, Facets: &summary}
	raw2, err := with.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeResult(raw2)
	if err != nil || got.Facets == nil || !bytes.Equal(Encode(*got.Facets), Encode(summary)) {
		t.Fatalf("round trip: %v %+v\n%s", err, got, raw2)
	}
	bad := strings.Replace(string(raw2), `"facets":{"total":"3"}`, `"facets":"3"`, 1)
	if _, err := DecodeResult([]byte(bad)); err == nil {
		t.Fatalf("non-object facets decoded: %s", bad)
	}
}
