package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestContextIdentifierTermsValuesAndWeights(t *testing.T) {
	t.Run("TCP-V0-019", func(t *testing.T) {
		input := `{"EnvelopeField": {"another_key": ["\tDecodePacket\ncodec_helper in parser.go", 42, false]}, "last": "scalar_label"}`
		text := contextQueryValues(input)
		selected := selectContextTerms(text)
		want := map[string]float64{
			"decodepacket": 3, "decode": 1, "packet": 1,
			"codec_helper": 3, "codec": 1, "helper": 1, "parser": 3,
			"scalar_label": 3, "scalar": 1, "label": 1,
		}
		if !reflect.DeepEqual(selected.weights, want) {
			t.Fatalf("weights = %#v, want %#v", selected.weights, want)
		}
		if strings.Contains(text, "EnvelopeField") || strings.Contains(text, "another_key") {
			t.Fatalf("JSON keys became task text: %q", text)
		}
		for range 10 {
			if contextQueryValues(input) != text {
				t.Fatal("JSON map traversal is not deterministic")
			}
		}
		// Repeated identifiers and parts keep the maximum, never term frequency.
		duplicate := selectContextTerms("DecodePacket DecodePacket decode.go")
		if duplicate.weights["decode"] != 3 || duplicate.weights["decodepacket"] != 3 {
			t.Fatalf("duplicate maximum lost: %#v", duplicate.weights)
		}
	})
}

func TestContextIdentifierTermsProseAndMalformedFallback(t *testing.T) {
	t.Run("TCP-V0-019", func(t *testing.T) {
		for _, text := range []string{"find the parser error", "{broken input"} {
			if contextQueryValues(text) != text {
				t.Fatalf("plain task rewritten: %q", text)
			}
			if !reflect.DeepEqual(sortedStringKeys(selectContextTerms(text).weights), taskLexicalTerms(text)) {
				t.Fatalf("prose fallback differs: %q", text)
			}
		}
		selected := selectContextTerms(contextQueryValues(`{"MisleadingName": "find the parser error"}`))
		if !reflect.DeepEqual(sortedStringKeys(selected.weights), taskLexicalTerms("find the parser error")) {
			t.Fatalf("fallback brought back envelope keys: %#v", selected.weights)
		}
	})
}

func TestContextIdentifierTermsExactOutranksParts(t *testing.T) {
	t.Run("TCP-V0-019", func(t *testing.T) {
		t.Setenv("CORVINT_CONTEXT_TERMS", "ident")
		t.Setenv("CORVINT_CONTEXT_RECIPE", "")
		root := t.TempDir()
		testGit(t, root, "init", "-q")
		testGit(t, root, "config", "user.email", "corvint@example.test")
		testGit(t, root, "config", "user.name", "Corvint Test")
		for path, content := range map[string]string{
			"go.mod":      "module example.test/terms\n\ngo 1.27.0\n",
			"exact.go":    "package sample\n// DecodePacket\n",
			"parts.go":    "package sample\n// Decode Packet\n",
			"case.go":     "package sample\n// decodepacket\n",
			"envelope.go": "package sample\n// EnvelopeField EnvelopeField\n",
		} {
			writeTestFile(t, root, path, content)
		}
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "identifier fixture")
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		compiler := newTaskContextCompiler(index, `{"EnvelopeField":"DecodePacket"}`, "")
		hits := compiler.lexicalHits()
		scores := map[string]float64{}
		for _, hit := range hits {
			scores[hit.path] = hit.score
		}
		if scores["exact.go"] <= scores["parts.go"] || scores["exact.go"] <= scores["case.go"] || scores["parts.go"] <= 0 {
			t.Fatalf("exact identifier must dominate split and wrong-case matches: %#v", scores)
		}
		if scores["envelope.go"] != 0 {
			t.Fatalf("envelope key credited: %#v", scores)
		}
	})
}

func TestContextIdentifierTermsDefaultBytes(t *testing.T) {
	t.Run("TCP-V0-019", func(t *testing.T) {
		index := recipeFixtureIndex(t)
		golden, err := os.ReadFile("testdata/context-recipe-default-golden.json")
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range []string{"", "unknown"} {
			t.Setenv("CORVINT_CONTEXT_TERMS", flag)
			packet := recipePacket(t, index, "", recipeFixtureTask)
			encoded, err := json.MarshalIndent(packet, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(append(encoded, '\n'), golden) {
				t.Fatalf("flag %q changed golden packet bytes", flag)
			}
		}
	})
}
