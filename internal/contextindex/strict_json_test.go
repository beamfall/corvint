package contextindex

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestStrictJSONMatchesWireParse: the local strict reader accepts exactly what
// internal/cem/wire.Parse accepts (NGI-V0-007), so the registry dialect refuses
// the manifests the application map refuses without importing a package
// outside the offline Tasks archive closure (PUB-V0-012, PUB-V0-013).
func TestStrictJSONMatchesWireParse(t *testing.T) {
	cases := []string{
		`{}`, `[]`, `null`, `true`, `false`, `0`, `7`, `"x"`, ` {"a": [1, {"b": null}]} `,
		`{"a": 1, "a": 2}`, `{"a": 1, "a": 2}`, `{"a": {"b": 1}, "b": {"b": 2}}`,
		`-1`, `1.5`, `1e3`, `01`, `9007199254740991`, `9007199254740992`, `12345678901234567`,
		`"😀"`, `"\ud83d"`, `"\ude00"`, `"\ud83dA"`, `"\uZZZZ"`, `"\q"`, `"\/\b\f\n\r\t"`,
		"\"a\x01b\"", "\"\xff\"", `{"a": 1,}`, `[1,]`, `[1 2]`, `{"a" 1}`, `{1: 2}`, `{} {}`, `nul`, `tru`, ``,
		`"unterminated`, `"\u12`, `{"a": 1`, `[`, "\v{}",
		strings.Repeat("[", 65) + strings.Repeat("]", 65),
		strings.Repeat("[", 66) + strings.Repeat("]", 66),
		`{"schema": "application-map-manifest\/0", "registries": []}`,
	}
	for _, text := range cases {
		_, err := wire.Parse([]byte(text))
		if got, want := strictJSON(text), err == nil; got != want {
			t.Errorf("strictJSON(%q) = %v, wire.Parse accepts = %v", text, got, want)
		}
	}
}

func FuzzStrictJSONMatchesWireParse(f *testing.F) {
	for _, seed := range []string{`{"a": [1, "😀"]}`, `{"a": 1, "a": 2}`, `[-0.5e1]`, "\"\x01\""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		_, err := wire.Parse([]byte(text))
		if got, want := strictJSON(text), err == nil; got != want {
			t.Errorf("strictJSON(%q) = %v, wire.Parse accepts = %v", text, got, want)
		}
	})
}
