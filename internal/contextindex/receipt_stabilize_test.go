package contextindex

import "testing"

// GPK-V0-028 requires a twice-stabilized `packet_bytes`. The property that
// matters is the fixed point itself: the recorded count must describe the
// receipt's own encoded length, not merely the last length the loop happened to
// see.
func TestGPKV0028StabilizePacketBytesReachesAFixedPoint(t *testing.T) {
	receipt := map[string]any{
		"state":    "BUDGETED",
		"coverage": map[string]any{"packet_bytes": 0},
	}
	if err := stabilizePacketBytes(receipt); err != nil {
		t.Fatalf("stabilizePacketBytes: %v", err)
	}
	encoded, err := CanonicalJSON(receipt)
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	if got := receipt["coverage"].(map[string]any)["packet_bytes"]; got != len(encoded) {
		t.Fatalf("packet_bytes %v does not describe the %d encoded bytes", got, len(encoded))
	}
}

// A count the comparison can never match leaves the loop without a fixed point.
// Reporting success there would ship a receipt whose self-description is wrong,
// so the exhausted loop must refuse instead.
func TestGPKV0028StabilizePacketBytesRefusesWithoutAFixedPoint(t *testing.T) {
	// A JSON-decoded count arrives as float64, which never compares equal to
	// the int length, so no iteration can converge.
	receipt := map[string]any{
		"state":    "BUDGETED",
		"coverage": map[string]any{"packet_bytes": float64(0)},
	}
	if err := stabilizePacketBytes(receipt); err == nil {
		t.Fatal("expected a refusal when packet_bytes cannot reach a fixed point")
	}
}
