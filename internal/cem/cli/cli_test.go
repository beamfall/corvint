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

// CCF-V1-004 (accepted 2026-09-26, decision 0422; from decision 0398): the two
// CEM read failures keep the oracle's fixed text and add their code.
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

// CEM-PILOT-028,029: the native argument parser admits the new formats and
// optional OCM input without changing the existing report default.
func TestReviewProjectionArguments(t *testing.T) {
	t.Run("CEM-PILOT-028 admitted formats retain report default", func(t *testing.T) {
		spec := cemActions["report"]
		for _, args := range [][]string{{"--map", "map.json", "--format", "json", "--ocm", "map.ocm.json"}, {"--map", "map.json", "--format", "markdown"}, {"--map", "map.json"}} {
			flags, unrecognized, err := parseCEMFlags(spec, args)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateParsed(spec, flags, unrecognized); err != nil {
				t.Fatal(err)
			}
		}
		flags, unrecognized, err := parseCEMFlags(spec, []string{"--map", "map.json", "--format", "html"})
		if err == nil {
			err = validateParsed(spec, flags, unrecognized)
		}
		if err == nil {
			t.Fatal("unsupported report format admitted")
		}
	})
}
