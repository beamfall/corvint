package wire

import (
	"testing"
)

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
