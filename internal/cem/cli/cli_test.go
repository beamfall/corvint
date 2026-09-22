package cli

import (
	"bytes"
	"testing"
)

func TestEscapeASCIIJSONMatchesPythonSpelling(t *testing.T) {
	got := escapeASCIIJSON([]byte("{\"text\":\"caf\u00e9 \u2014 \U0001f642\u007f\"}"))
	want := []byte(`{"text":"caf\u00e9 \u2014 \ud83d\ude42\u007f"}`)
	if !bytes.Equal(got, want) {
		t.Fatalf("escaped JSON = %q, want %q", got, want)
	}
}
