package contextindex

import (
	"strings"
)

// termsMatchingCorpus holds the shapes terms() treats specially, so a divergence
// in any of them fails here rather than as a receipt diff three layers up.
var termsMatchingCorpus = []string{
	"",
	"a",
	"ab",
	"abc",
	"func EnforceSessionRevocation() bool { return true }",
	"session_revocation_enforced",
	"HTTPServerHandler parseHTTPRequest",
	"v2 v10 API2Client",
	"queries parses busses aliases status radius analysis",
	"stringify classify verify identify",
	"argument arguments parameter parameters maximum minimum generate generator sync synchronous",
	"the and with from where which feature find change code",
	"trailing- -leading --double--  a--b",
	"kebab-case-identifier and snake_case_identifier",
	"9Digit lowerUPPER UPPERlower ABCDef",
	"// comment: retrieval budget, packet(size)=2000; path/to/file.go:14",
	"café naïve Ω identifier",
	// The two runes ToLower maps into ASCII, which is why the streaming path
	// keeps a fallback at all: each joins tokens a byte scan reads as separated.
	"\u212Aelvin sessionRevocation",
	"session\u212Aey and \u0130dentifier",
	"ba\u0130s ba\u212Aes",
	"KELVIN K sign",
	"İstanbul dotted capital",
	"emoji \U0001f600 between words",
	"mixed café_camelCase-hyphen",
	strings.Repeat("longIdentifierSegment", 40),
	strings.Repeat("x", 300) + "Boundary",
}

var termsMatchingKeepSets = []string{
	"session revocation enforce enforced device expiry",
	"http server handler parse request",
	"query queries parse parses bus busses alias aliases",
	"stringify string classify class verify identify",
	"argument parameter max maximum min minimum gen generate sync synchronous",
	"the and with feature",
	"trailing leading double kebab case identifier snake",
	"digit lower upper abcdef abc def v2 v10 api2 client api",
	"comment retrieval budget packet size path file go",
	"caf cafe naive identifier kelvin kelvi sign stanbul dotted capital emoji between words mixed hyphen camel",
	"session sessionkey key identifier idisentifier bais bakes ba session-revocation revocation",
	"longidentifiersegment long segment boundary x",
}
