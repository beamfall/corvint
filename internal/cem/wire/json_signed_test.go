package wire

import "testing"

// TestParseSignedAdmitsNegativeIntegers pins the signed reader used for
// cem-stable-verification/1 result lines (CEM-V1-569): negative integers down
// to -MaxWireInteger parse, while Parse keeps refusing every negative number.
func TestParseSignedAdmitsNegativeIntegers(t *testing.T) {
	for input, want := range map[string]int64{`-2`: -2, `-9007199254740991`: -MaxWireInteger, `7`: 7} {
		value, err := ParseSigned([]byte(input))
		if err != nil || value.Kind != KindInt || value.Int != want {
			t.Fatalf("ParseSigned(%s) = %+v, %v; want %d", input, value, err, want)
		}
	}
	for _, input := range []string{`-0`, `-`, `--1`, `-01`, `-1.5`, `-9007199254740992`, `- 1`} {
		if _, err := ParseSigned([]byte(input)); err == nil {
			t.Fatalf("ParseSigned(%s) accepted", input)
		}
	}
	if _, err := Parse([]byte(`{"specificity":-2}`)); err == nil {
		t.Fatal("Parse accepted a negative integer")
	}
}
