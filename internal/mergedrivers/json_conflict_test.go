// SPDX-License-Identifier: Apache-2.0
package mergedrivers

import (
	"encoding/json"
	"strings"
	"testing"
)

// ASD-V0-3792 (V1-5532): a refused JSON merge renders every independent key merged and
// each competing key between markers, so neither side's entries are silently dropped and
// no JSON reader accepts the file until a person resolves it.
func TestJSONConflictASDV03792(t *testing.T) {
	base := `{"profile":"p","packages":{"shared":["a"],"untouched":[]}}`
	ours := `{"profile":"p","packages":{"lane":["x"],"shared":["a","ours"],"untouched":[]}}`
	theirs := `{"profile":"p","packages":{"main":["y"],"shared":["a","theirs"],"untouched":[]}}`
	want := `{
  "packages": {
    "lane": [
      "x"
    ],
    "main": [
      "y"
    ],
<<<<<<< ours
    "shared": [
      "a",
      "ours"
    ],
||||||| base
    "shared": [
      "a"
    ],
=======
    "shared": [
      "a",
      "theirs"
    ],
>>>>>>> theirs
    "untouched": []
  },
  "profile": "p"
}
`
	_, err := JSON([]byte(base), []byte(ours), []byte(theirs))
	if err == nil || err.Error() != `competing registry key "shared" in "packages": base ["a"], ours ["a","ours"], theirs ["a","theirs"]` {
		t.Fatalf("refusal does not name the key and its values: %v", err)
	}
	got := string(JSONConflict([]byte(base), []byte(ours), []byte(theirs), 7))
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if json.Valid([]byte(got)) {
		t.Fatal("a conflict file parses as JSON")
	}
	// Resolving the block by keeping both sides' values yields the union a person intends.
	resolved := strings.Replace(got, got[strings.Index(got, "<<<<<<<"):strings.Index(got, ">>>>>>> theirs\n")+len(">>>>>>> theirs\n")], "    \"shared\": [\"a\", \"ours\", \"theirs\"],\n", 1)
	if !json.Valid([]byte(resolved)) {
		t.Fatalf("resolving the one marked key does not restore JSON:\n%s", resolved)
	}

	for _, tc := range []struct{ name, base, ours, theirs, wantErr, wantFile string }{
		{"top-level-scalar", `{"a":0,"z":1}`, `{"a":1,"z":1}`, `{"a":2,"z":1}`,
			`competing registry key "a": base 0, ours 1, theirs 2`,
			"{\n<<<<<<< ours\n  \"a\": 1,\n||||||| base\n  \"a\": 0,\n=======\n  \"a\": 2,\n>>>>>>> theirs\n  \"z\": 1\n}\n"},
		{"delete-edit-last-key", `{"a":0}`, `{}`, `{"a":2}`,
			`competing registry key "a": base 0, ours absent, theirs 2`,
			"{\n<<<<<<< ours\n||||||| base\n  \"a\": 0\n=======\n  \"a\": 2\n>>>>>>> theirs\n}\n"},
		{"two-keys-named", `{"tests":{"k":0},"x":0}`, `{"tests":{"k":1},"x":1}`, `{"tests":{"k":2},"x":2}`,
			"competing registry key \"k\" in \"tests\": base 0, ours 1, theirs 2\ncompeting registry key \"x\": base 0, ours 1, theirs 2", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := JSON([]byte(tc.base), []byte(tc.ours), []byte(tc.theirs))
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("error %v, want %q", err, tc.wantErr)
			}
			got := JSONConflict([]byte(tc.base), []byte(tc.ours), []byte(tc.theirs), 7)
			if tc.wantFile != "" && string(got) != tc.wantFile {
				t.Fatalf("got %q, want %q", got, tc.wantFile)
			}
			if json.Valid(got) {
				t.Fatalf("a conflict file parses as JSON: %s", got)
			}
		})
	}

	// An input that cannot be merged at all is kept whole on every side, with markers longer
	// than any marker run that already starts an input line, and with the requested size.
	t.Run("malformed-whole", func(t *testing.T) {
		malformed := "{\"a\":\n<<<<<<< stray\n"
		_, err := JSON([]byte(`{}`), []byte(`{"a":1}`), []byte(malformed))
		if err == nil || !strings.HasPrefix(err.Error(), "theirs registry: ") {
			t.Fatalf("refusal does not name the malformed side: %v", err)
		}
		got := string(JSONConflict([]byte(`{}`), []byte(`{"a":1}`), []byte(malformed), 7))
		want := "<<<<<<<< ours\n{\"a\":1}\n|||||||| base\n{}\n========\n" + malformed + ">>>>>>>> theirs\n"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
		if got := string(JSONConflict(nil, []byte(`[]`), []byte(`{}`), 9)); got != "<<<<<<<<< ours\n[]\n||||||||| base\n=========\n{}\n>>>>>>>>> theirs\n" {
			t.Fatalf("requested size or empty base: %q", got)
		}
	})
}
