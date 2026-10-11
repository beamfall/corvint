package wire

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// CEM-V1-018: the 0.x parser refuses an exact cem/1.0 root as unsupported-spec
// and names the stable read, never reporting it as malformed 0.x input.
func TestParseMapStableRefusalNamesStableRead(t *testing.T) {
	_, err := ParseMap([]byte(`{"spec":"cem/1.0","change":{}}`))
	if cemcode.CodeOf(err) != cemcode.UnsupportedSpec {
		t.Fatalf("code = %q, want %q", cemcode.CodeOf(err), cemcode.UnsupportedSpec)
	}
	for _, part := range []string{"cem/1.0 is read only by the stable verifier", "--artifacts", Spec01, Spec02, Spec03} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("refusal %q does not name %q", err.Error(), part)
		}
	}
}

// CEM-V1-019: every other refused spec, including the experimental candidate,
// keeps the unchanged 0.x refusal text.
func TestParseMapOtherSpecRefusalUnchanged(t *testing.T) {
	const want = `spec must be "cem/0.1", "cem/0.2", or "cem/0.3"`
	for _, document := range []string{
		`{"spec":"cem/1.0-experimental.1"}`, `{"spec":"cem/1.0 "}`, `{"spec":"CEM/1.0"}`, `{"spec":1}`, `{"spec":null}`, `{"spec":["cem/1.0"]}`,
	} {
		_, err := ParseMap([]byte(document))
		if cemcode.CodeOf(err) != cemcode.UnsupportedSpec || !strings.HasSuffix(err.Error(), want) {
			t.Errorf("%s: %v, want unsupported-spec %q", document, err, want)
		}
	}
}

// CEM-V1-019: DeclaresStable selects the stable reader only for one JSON
// object whose spec is exactly cem/1.0.
func TestDeclaresStable(t *testing.T) {
	for document, want := range map[string]bool{
		`{"spec":"cem/1.0"}`:                  true,
		`{"spec":"cem/1.0","change":{"x":1}}`: true,
		`{"spec":"cem/1.0-experimental.1"}`:   false,
		`{"spec":"cem/0.3"}`:                  false,
		`{"spec":"cem/1.0 "}`:                 false,
		`["cem/1.0"]`:                         false,
		`"cem/1.0"`:                           false,
		`{"spec":"cem/1.0"`:                   false,
		`{"spec":"cem/1.0","spec":"cem/0.3"}`: false,
		`{"change":{"spec":"cem/1.0"}}`:       false,
		`{"spec":{"value":"cem/1.0"}}`:        false,
	} {
		if got := DeclaresStable([]byte(document)); got != want {
			t.Errorf("DeclaresStable(%s) = %v, want %v", document, got, want)
		}
	}
	if DeclaresStable(append([]byte(`{"spec":"cem/1.0","pad":"`), append(make([]byte, MaxMapBytes), `"}`...)...)) {
		t.Error("DeclaresStable accepted a document over MaxMapBytes")
	}
}
