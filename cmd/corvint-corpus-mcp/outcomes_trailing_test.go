package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// V1-3358: json.Decoder.More reports false before a trailing closing
// delimiter, so both strict decodes must refuse one.
func TestOutcomeDecodesRefuseTrailingDelimiter(t *testing.T) {
	valid, err := json.Marshal(outcomeFile{Schema: outcomeSchema, ArtifactSHA256: "a", LatencyBounds: latencyBuckets, Tools: map[string]*toolOutcome{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeOutcomes(valid); err != nil {
		t.Fatalf("valid outcome log refused: %v", err)
	}
	for _, trailer := range []string{"}", "]"} {
		if _, err := decodeOutcomes(append(append([]byte{}, valid...), trailer...)); err == nil {
			t.Errorf("outcome log with trailing %q accepted", trailer)
		}
		path := filepath.Join(t.TempDir(), "floor.json")
		if err := os.WriteFile(path, []byte(`{"answered_share":0.5,"tolerance":0.1}`+trailer), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, ok := readFloor(path); ok {
			t.Errorf("floor with trailing %q accepted", trailer)
		}
	}
	path := filepath.Join(t.TempDir(), "floor.json")
	if err := os.WriteFile(path, []byte("{\"answered_share\":0.5,\"tolerance\":0.1}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := readFloor(path); !ok {
		t.Error("valid floor refused")
	}
}
