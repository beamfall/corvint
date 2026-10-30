package wp3codec

import (
	"bytes"
	"encoding/json"
	"testing"
)

// FuzzParseIsStrictJSONAndCanonicalizes checks Parse and Encode against the
// codec clause: Parse admits only a JSON subset, so every accepted input is
// valid JSON; the canonical encoding of an accepted value passes Verify and
// reparses to the same bytes; and Verify accepts exactly those canonical bytes.
func FuzzParseIsStrictJSONAndCanonicalizes(f *testing.F) {
	f.Add([]byte(`{"a":["1",true,null],"b":{}}`))
	f.Add([]byte("{\"b\": \"\\u000A\", \"a\": \"\\ud83d\\ude00\"}"))
	f.Add([]byte("\"\\ud83d\\ude00\""))
	f.Fuzz(func(t *testing.T, data []byte) {
		value, err := Parse(data)
		if err != nil {
			return
		}
		if !json.Valid(data) {
			t.Fatalf("Parse accepted %q, which is not JSON", data)
		}
		canonical, err := Encode(value)
		if err != nil {
			t.Fatalf("Encode of parsed %q failed: %v", data, err)
		}
		if err := Verify(canonical); err != nil {
			t.Fatalf("canonical form %q of %q fails Verify: %v", canonical, data, err)
		}
		if verifyErr := Verify(data); (verifyErr == nil) != bytes.Equal(data, canonical) {
			t.Fatalf("Verify(%q) = %v, canonical form %q", data, verifyErr, canonical)
		}
	})
}
