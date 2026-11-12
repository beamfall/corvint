package contextindex

import (
	"math/rand"
	"slices"
	"strings"
	"testing"
)

// nameTokensOracle is the regex form nameTokens replaced (TCP-V0-015's
// camel-split token rule), kept as the differential reference.
func nameTokensOracle(name string) []string {
	spaced := contextCamel.ReplaceAllString(name, "$1 $2")
	tokens := contextToken.FindAllString(spaced, -1)
	for position := range tokens {
		tokens[position] = strings.ToLower(tokens[position])
	}
	return tokens
}

// TestNameTokensMatchesRegexOracle: the byte scanner and the regex agree on
// every explicit shape (empty, punctuation only, digits, acronyms, adjacent
// boundaries, non-ASCII and invalid UTF-8 separators) and on deterministic
// random byte strings, including whether an empty result is nil.
func TestNameTokensMatchesRegexOracle(t *testing.T) {
	t.Run("TCP-V0-015", checkNameTokensMatchesRegexOracle)
}

func checkNameTokensMatchesRegexOracle(t *testing.T) {
	explicit := []string{
		"", "_", "___", "-.-", " ", "a", "A", "Ab", "aB", "aBcD", "aBC", "ABCdef", "HTTPServer2Go",
		"parseJSONValue", "snake_case_name", "a1B2", "x9", "9x", "Test", "TestDrainQueueTwice",
		"describe 'Foo bar-Baz'", "ünicode Über", "日本語Text", "naïveCase", "\xff\xfeAbc", "a\xffB",
		"a\x00B", "Ab\x80cD", "\xc3", "é", "aé", "éA", "trailingUpperA", "lowerUPPERlower",
	}
	for _, name := range explicit {
		assertNameTokens(t, name)
	}
	random := rand.New(rand.NewSource(20260906))
	alphabet := []byte("abcxyzABCXYZ0189_ -./\x00\x7f\x80\x9c\xa9\xc3\xe2\xff")
	for run := 0; run < 5000; run++ {
		buffer := make([]byte, random.Intn(25))
		for position := range buffer {
			buffer[position] = alphabet[random.Intn(len(alphabet))]
		}
		assertNameTokens(t, string(buffer))
	}
}

func assertNameTokens(t *testing.T, name string) {
	t.Helper()
	got, want := nameTokens(name), nameTokensOracle(name)
	if !slices.Equal(got, want) || (got == nil) != (want == nil) {
		t.Fatalf("nameTokens(%q) = %#v, regex oracle %#v", name, got, want)
	}
}
