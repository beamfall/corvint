package analyzerenvelope

import (
	"strings"
	"testing"
)

func TestDecodedLengthIsExactForPaddedBase64(t *testing.T) {
	for _, test := range []struct {
		encoded string
		size    int
		exact   bool
	}{
		{"", 0, true},
		{"QQ==", 1, true},
		{"QUI=", 2, true},
		{"QUJD", 3, true},
		{"QUJDRA==", 4, true},
		{"====", 1, true},
		{"Q", 0, false},
		{"QQ=", 0, false},
		{"QUJDR", 0, false},
	} {
		size, exact := DecodedLength(test.encoded)
		if size != test.size || exact != test.exact {
			t.Errorf("DecodedLength(%q) = %d, %v; want %d, %v", test.encoded, size, exact, test.size, test.exact)
		}
	}
}

func TestDuplicateJSONNameRefusesDuplicatesTrailingDataAndBounds(t *testing.T) {
	nested := func(depth int) string { return strings.Repeat("[", depth) + strings.Repeat("]", depth) }
	array := func(values int) string { return "[" + strings.TrimSuffix(strings.Repeat("0,", values), ",") + "]" }
	for _, test := range []struct {
		name string
		raw  string
		want bool
	}{
		{"object", `{"a":1,"b":{"a":[1,2]}}`, false},
		{"scalar", `1`, false},
		{"empty object", `{}`, false},
		{"duplicate top level", `{"a":1,"a":2}`, true},
		{"duplicate nested", `{"a":{"b":1,"b":2}}`, true},
		{"duplicate inside array", `[{"x":1},{"x":1,"x":2}]`, true},
		{"same name in sibling objects", `[{"x":1},{"x":2}]`, false},
		{"trailing value", `{"a":1} {}`, true},
		{"empty", ``, true},
		{"truncated", `{"a":`, true},
		{"depth at bound", nested(4), false},
		{"depth over bound", nested(5), true},
		{"tokens at bound", array(7), false},
		{"tokens over bound", array(8), true},
	} {
		if got := DuplicateJSONName([]byte(test.raw), 3, 8); got != test.want {
			t.Errorf("%s: DuplicateJSONName(%q) = %v, want %v", test.name, test.raw, got, test.want)
		}
	}
}

// The known bytes of a refused body are whatever the scan had copied when it
// stopped; they are pinned only so a refactor cannot change them silently.
func TestWithoutExtensionsStripsOnlySafeCanonicalExtensions(t *testing.T) {
	const target = `"target":{"os":"linux","architecture":"amd64","abi":"none","features":[]}`
	const input = `{"handle":"h","family":"f","path":"p","sha256":"s","content_base64":""}`
	head := `{"profile":"p","family":"f","request_id":"r","scope_id":"s","compilation_unit_id":"u",` + target
	for _, test := range []struct {
		name  string
		body  string
		known string
		ok    bool
	}{
		{"no extension", head + `,"inputs":[` + input + `]}`, head + `,"inputs":[` + input + `]}`, true},
		{"top-level extension", head + `,"inputs":[],"x":1}`, head + `,"inputs":[]}`, true},
		{"ordered extensions", head + `,"inputs":[],"x":1,"y":{"z":[true,null,"s"]}}`, head + `,"inputs":[]}`, true},
		{"target extension", `{"target":{"os":"linux","zz":-1}}`, `{"target":{"os":"linux"}}`, true},
		{"input extension", `{"inputs":[{"handle":"h","x":0}]}`, `{"inputs":[{"handle":"h"}]}`, true},
		{"space in name is an extension", `{"inputs":[],"target inputs":1}`, `{"inputs":[]}`, true},
		{"unordered extensions", head + `,"inputs":[],"y":1,"x":1}`, head + `,"inputs":[]:1}`, false},
		{"schema field after extension", `{"x":1,"inputs":[]}`, `{,"inputs":[]}`, false},
		{"float extension", `{"inputs":[],"x":1.5}`, `{"inputs":[]}`, false},
		{"negative zero extension", `{"inputs":[],"x":-0}`, `{"inputs":[]}`, false},
		{"exponent extension", `{"inputs":[],"x":1e2}`, `{"inputs":[]}`, false},
		{"noncanonical whitespace", `{"inputs":[], "x":1}`, `{"inputs":[]}`, false},
		{"noncanonical escape", `{"inputs":[],"x":"\/"}`, `{"inputs":[]}`, false},
		{"truncated", `{"inputs":[`, `{"inputs":[`, false},
	} {
		known, ok := WithoutExtensions([]byte(test.body))
		if string(known) != test.known || ok != test.ok {
			t.Errorf("%s: WithoutExtensions(%q) = %q, %v; want %q, %v", test.name, test.body, known, ok, test.known, test.ok)
		}
	}
}
