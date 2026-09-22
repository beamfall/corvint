package wire

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

// FuzzParseAgreesWithEncodingJSON checks the strict reader against its package
// contract: every accepted document is valid JSON that encoding/json decodes to
// the same tree, and the canonical form of an accepted value reparses to the
// same canonical bytes. ParseMap must refuse rather than panic on any input.
func FuzzParseAgreesWithEncodingJSON(f *testing.F) {
	seeds, _ := filepath.Glob(filepath.Join("..", "..", "..", "interop", "cem-0.1", "maps", "*", "*.json"))
	for _, seed := range seeds {
		if data, err := os.ReadFile(seed); err == nil {
			f.Add(data)
		}
	}
	f.Add([]byte("{\"a\":[1,true,null,\"\\ud83d\\ude00\\u0000\"],\"b\":{}}"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParseMap(data)
		value, err := Parse(data)
		if err != nil {
			return
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		var decoded any
		if err := decoder.Decode(&decoded); err != nil || !json.Valid(data) {
			t.Fatalf("Parse accepted %q that encoding/json rejects: %v", data, err)
		}
		if !reflect.DeepEqual(plainValue(value), decoded) {
			t.Fatalf("Parse(%q) = %#v, encoding/json = %#v", data, plainValue(value), decoded)
		}
		canonical := CanonicalValue(value)
		reparsed, err := Parse(canonical)
		if err != nil {
			t.Fatalf("canonical form %q of %q does not reparse: %v", canonical, data, err)
		}
		if again := CanonicalValue(reparsed); !bytes.Equal(again, canonical) {
			t.Fatalf("canonical form is not stable: %q then %q", canonical, again)
		}
	})
}

func plainValue(value Value) any {
	switch value.Kind {
	case KindBool:
		return value.Bool
	case KindInt:
		return json.Number(strconv.FormatInt(value.Int, 10))
	case KindString:
		return value.Str
	case KindArray:
		items := []any{}
		for _, item := range value.Arr {
			items = append(items, plainValue(item))
		}
		return items
	case KindObject:
		members := map[string]any{}
		for _, key := range value.Obj.Keys {
			members[key] = plainValue(value.Obj.Values[key])
		}
		return members
	}
	return nil
}
