package jsonend

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDoneRefusesTrailingDelimitersMoreAccepts(t *testing.T) {
	for _, tc := range []struct {
		input string
		done  bool
	}{
		{`{"a":1}`, true},
		{"{\"a\":1}\n\t ", true},
		{`[1]`, true},
		{`{"a":1}}`, false},
		{`{"a":1}]`, false},
		{`[1]]`, false},
		{`[1]}`, false},
		{`{"a":1} {}`, false},
		{`{"a":1} x`, false},
		{`{"a":1},`, false},
	} {
		decoder := json.NewDecoder(strings.NewReader(tc.input))
		var value any
		if err := decoder.Decode(&value); err != nil {
			t.Fatalf("%q: %v", tc.input, err)
		}
		if got := Done(decoder); got != tc.done {
			t.Errorf("Done(%q) = %v, want %v", tc.input, got, tc.done)
		}
	}
	// The defect this package exists for: More accepts a trailing delimiter.
	decoder := json.NewDecoder(strings.NewReader(`{"a":1}}`))
	var value any
	if err := decoder.Decode(&value); err != nil || decoder.More() {
		t.Fatalf("json.Decoder.More no longer accepts a trailing delimiter (err %v); revisit V1-3358", err)
	}
}
