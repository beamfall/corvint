package doccompiler

import (
	"strings"
	"testing"
)

func TestCanonicalJSONVerifierEnforcesHDCV0041(t *testing.T) {
	stringOfSize := func(size int) string { return `"` + strings.Repeat("a", size-3) + "\"\n" }
	accepted := []string{
		"{}\n", "[]\n", "null\n", "-9223372036854775808\n", "9223372036854775807\n",
		`{"Z":1,"a":[true,false,null],"b":{"c":"\"\\\b\t\n\f\r\u0001\u001f"},"` + "\u00e9" + `":0}` + "\n",
		"\"/\x7f\u2028\u2029\u00e9\"\n",
		strings.Repeat("[", 10000) + strings.Repeat("]", 10000) + "\n",
		stringOfSize(MaxCanonicalJSONBytes),
	}
	for _, raw := range accepted {
		if err := VerifyCanonicalJSON([]byte(raw)); err != nil {
			t.Errorf("VerifyCanonicalJSON(%.60q) = %v, want canonical", raw, err)
		}
	}
	refused := []struct{ raw, code string }{
		{stringOfSize(MaxCanonicalJSONBytes + 1), "canonical-json-too-large"},
		{"\"\xff\"\n", "canonical-json-invalid-utf8"},
		{"", "canonical-json-missing-final-lf"},
		{"{}", "canonical-json-missing-final-lf"},
		{"{\"a\":}\n", "canonical-json-syntax"},
		{strings.Repeat("[", 10001) + strings.Repeat("]", 10001) + "\n", "canonical-json-syntax"},
		{"{\"a\":1,\"a\":1}\n", "canonical-json-duplicate-key"},
		{"1.0\n", "canonical-json-non-integer"},
		{"1e3\n", "canonical-json-non-integer"},
		{"9223372036854775808\n", "canonical-json-non-integer"},
		{"{\"b\":1,\"a\":2}\n", "canonical-json-not-canonical"},
		{"{ }\n", "canonical-json-not-canonical"},
		{"{}\n\n", "canonical-json-not-canonical"},
		{"{}{}\n", "canonical-json-not-canonical"},
		{"-0\n", "canonical-json-not-canonical"},
		{`"\/"` + "\n", "canonical-json-not-canonical"},
		{`"\u00e9"` + "\n", "canonical-json-not-canonical"},
		{`"\u0008"` + "\n", "canonical-json-not-canonical"},
		{`"\u001F"` + "\n", "canonical-json-not-canonical"},
		{`"\u2028"` + "\n", "canonical-json-not-canonical"},
		{`"\ud800"` + "\n", "canonical-json-not-canonical"},
	}
	for _, test := range refused {
		if code := errorCode(VerifyCanonicalJSON([]byte(test.raw))); code != test.code {
			t.Errorf("VerifyCanonicalJSON(%.60q) code = %q, want %q", test.raw, code, test.code)
		}
	}
}

func TestCanonicalJSONEmitsBytesTheVerifierAccepts(t *testing.T) {
	type shape struct {
		Z string           `json:"z"`
		A []any            `json:"a"`
		M map[string]int64 `json:"m"`
	}
	documents := []any{
		nil, true, int64(-9223372036854775808), "",
		"<>&  /\x7f\x00\x1f\"\\\b\t\n\f\ré",
		shape{Z: "last", A: []any{1, "two", nil, map[string]any{"b": 1, "a": 2}}, M: map[string]int64{"é": 1, "Z": 2, "a": 3}},
		minimalReceipt(),
		PatchPlan{Profile: ExperimentalPatchPlanProfile, Documents: []DocumentPatch{{Path: "docs/<a&b>.md"}}},
	}
	for _, document := range documents {
		encoded, err := CanonicalJSON(document)
		if err != nil {
			t.Fatalf("CanonicalJSON(%#v) = %v", document, err)
		}
		if err := VerifyCanonicalJSON(encoded); err != nil {
			t.Errorf("VerifyCanonicalJSON(CanonicalJSON(%#v)) = %v", document, err)
		}
	}
	refused := []struct {
		value any
		code  string
	}{
		{1.5, "canonical-json-non-integer"},
		{make(chan int), "canonical-json-unencodable"},
		{strings.Repeat("a", MaxCanonicalJSONBytes), "canonical-json-too-large"},
	}
	for _, test := range refused {
		if encoded, err := CanonicalJSON(test.value); errorCode(err) != test.code || encoded != nil {
			t.Errorf("CanonicalJSON(%.40T) = %d bytes, %v; want %s", test.value, len(encoded), err, test.code)
		}
	}
}
