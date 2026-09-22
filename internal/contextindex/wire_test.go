package contextindex

import (
	"bytes"
	"testing"
)

func TestCanonicalJSONMatchesPythonASCIIEscaping(t *testing.T) {
	encoded, err := CanonicalJSON(map[string]any{
		"bmp":    "café\u2028\u2029",
		"astral": "😀",
		"del":    "\x7f",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"astral":"\ud83d\ude00","bmp":"caf\u00e9\u2028\u2029","del":"\u007f"}`)
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded = %q, want %q", encoded, want)
	}
}
