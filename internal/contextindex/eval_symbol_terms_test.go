package contextindex

import (
	"reflect"
	"testing"
)

// symbolTermsDistributivityPairs are name/path pairs chosen for the shapes the
// distributivity argument turns on: a camelCase boundary that would be split
// differently if the two sides were joined without a space, an acronym run, a
// snake_case and a hyphenated identifier, digits abutting letters, a name that
// is entirely a stop word, an empty name, non-ASCII text, and the two runes
// Unicode lowercases into ASCII.
var symbolTermsDistributivityPairs = [][2]string{
	{"EnforceSessionRevocation", "internal/auth/session.go"},
	{"enforce_session_revocation", "internal/auth/session_test.go"},
	{"parseHTTPRequest", "internal/http/parse.go"},
	{"HTTPServerHandler", "cmd/corvint/main.go"},
	{"v2", "api/v2/client.go"},
	{"queries", "internal/queries/aliases.go"},
	{"the", "docs/specs/the-index.md"},
	{"", "internal/empty/empty.go"},
	{"r\u00e9vocationNa\u00efve", "docs/caf\u00e9/\u03a9.md"},
	{"\u212Aelvin\u0130dentifier", "internal/\u212Akey/session\u0130d.go"},
	{"X", "a/b"},
	{"AlreadySplit-Name", "already-split/path-name.go"},
	{"trailing", "path/with/trailing/"},
	{"leadingUnderscore", "_internal/_hidden/file_name.go"},
}

// TestSymbolTermsDistributivityIsSpecificToTheSeparator is the non-vacuity
// control for the property above. terms does not distribute over an arbitrary
// join -- concatenating the two sides with no separator lets camelSplit and
// asciiWords read across the seam -- so the property being asserted is a claim
// about the space, not a tautology about terms.
func TestSymbolTermsDistributivityIsSpecificToTheSeparator(t *testing.T) {
	name, sourcePath := "EnforceSessionRevocation", "internal/auth/session.go"
	union := terms(name)
	for term := range terms(sourcePath) {
		union[term] = struct{}{}
	}
	if reflect.DeepEqual(terms(name+sourcePath), union) {
		t.Fatal("terms distributes over an empty join too, so the space is not what the property tests")
	}
}
