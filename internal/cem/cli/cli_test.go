package cli

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func TestEscapeASCIIJSONMatchesPythonSpelling(t *testing.T) {
	got := escapeASCIIJSON([]byte("{\"text\":\"caf\u00e9 \u2014 \U0001f642\u007f\"}"))
	want := []byte(`{"text":"caf\u00e9 \u2014 \ud83d\ude42\u007f"}`)
	if !bytes.Equal(got, want) {
		t.Fatalf("escaped JSON = %q, want %q", got, want)
	}
}

// CCF-V1-004 (proposed, decision 0398): the two CEM read failures keep the
// oracle's fixed text and add their code.
func TestReadFailuresKeepTheFixedTextAndAddTheirCode(t *testing.T) {
	for code, want := range map[string]string{
		cemcode.PatchUnavailable: `{"code": "patch-unavailable", "error": "cannot read patch", "ok": false}` + "\n",
		cemcode.MapUnavailable:   `{"code": "map-unavailable", "error": "cannot read CEM map", "ok": false}` + "\n",
	} {
		var stderr bytes.Buffer
		emitCEMError(&stderr, cemcode.New(code, "cannot open .corvint/detail"))
		if stderr.String() != want {
			t.Fatalf("stderr=%q, want %q", stderr.String(), want)
		}
	}
}
