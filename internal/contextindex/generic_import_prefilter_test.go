package contextindex

import "testing"

// genericImportPrefilterCases are bodies on both sides of mayHoldGenericImport's
// boundary: each regex opening shape, Unicode and control spaces, invalid
// bytes, and prose whose keywords never reach a quote.
var genericImportPrefilterCases = []string{
	"",
	`import "x"`,
	`import"x"`,
	`import ( "x" )`,
	"import(\n\t'x'\n)",
	`from 'x'`,
	`from"x"`,
	"from \"x\"",
	"from 'x'",
	"from\x85\"x\"",
	"from\u0085\"x\"",
	"from\x1c\"x\"",
	"from\xff\"x\"",
	"import　(　\"x\"",
	"import ((\"x\"",
	`import ""`,
	`import "`,
	`afrom "x"`,
	"We import data from the store, then say \"done\".",
	"Copy the value from `config` and import it; see 'notes'.",
	`{"from": "a", "import": "b"}`,
	`{"from" : "a"}`,
	"from from from \"x\"",
	"importimport 'x'",
	"no keyword \"here\"",
	"import\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t",
}

// TestGenericImportPrefilterLosesNoMatch pins the V1-1640 gate's exactness:
// a body the prefilter rejects is one genericImport finds no match in, so
// skipping the regex leaves every import edge, and the index bytes, unchanged.
func TestGenericImportPrefilterLosesNoMatch(t *testing.T) {
	rejected := 0
	for _, text := range genericImportPrefilterCases {
		matches := genericImport.FindAllStringIndex(text, -1)
		got := mayHoldGenericImport(text)
		if !got && len(matches) != 0 {
			t.Errorf("mayHoldGenericImport(%q) = false, regex matches %d", text, len(matches))
		}
		if !got {
			rejected++
		}
	}
	if rejected < 8 {
		t.Fatalf("prefilter rejected %d bodies; the table no longer exercises the skip path", rejected)
	}
	// A prose body the old necessary condition admitted (quote plus keyword)
	// now skips the regex.
	prose := "We import data from the store, then say \"done\"."
	if mayHoldGenericImport(prose) {
		t.Fatalf("prose body still reaches the regex: %q", prose)
	}
}

// FuzzGenericImportPrefilter is the differential form of the same property
// over arbitrary bytes. The prefilter is exact on a match's opening, so it may
// admit a body only when an opening exists; it must never reject a body the
// regex matches.
func FuzzGenericImportPrefilter(f *testing.F) {
	for _, text := range genericImportPrefilterCases {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if !mayHoldGenericImport(text) && genericImport.MatchString(text) {
			t.Fatalf("prefilter rejected a body the regex matches: %q", text)
		}
	})
}
