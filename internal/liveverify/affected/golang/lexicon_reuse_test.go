// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"math/rand/v2"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// AFP-V0-021: compare the allocation-reduced lexer against the regexp
// lexicon, including all byte values and short outer components. Each run
// keeps its written spelling beside the closing-tag, root-anchored and
// parent-backslash spellings (AFP-V0-505, AFP-V0-506, AFP-V0-2242).
func TestLiteralTokensMatchRegexpLexicon(t *testing.T) {
	pattern := regexp.MustCompile(`[A-Za-z0-9._~@+/-]+`)
	whole := func(name, value string, start, end int) string {
		parts := strings.Split(name, "/")
		if last := len(parts) - 1; last > 0 {
			if short(parts[0]) && runComponent(parts[1]) && delimited(value, start-1) {
				name = "./" + name
			}
			if short(parts[last]) && runComponent(parts[last-1]) && delimited(value, end) {
				name += "/"
			}
		}
		return name
	}
	spellings := func(result []string, value string, start, end int, module string) []string {
		name := value[start:end]
		result = append(result, whole(name, value, start, end))
		if anchored := rootAnchored(name, module); anchored != name {
			result = append(result, whole(anchored, value, start, end))
		}
		return result
	}
	spelled := func(value, module string) []string {
		var result []string
		for _, span := range pattern.FindAllStringIndex(value, -1) {
			start, end := span[0], span[1]
			result = spellings(result, value, start, end, module)
			if value[start] == '/' && end > start+1 && (start > 0 && value[start-1] == '<' || closesEscapedTag(value, start)) {
				result = spellings(result, value, start+1, end, module)
			}
		}
		return result
	}
	legacy := func(value, module string) []string {
		value = printfVerb.ReplaceAllString(value, verbMark)
		result := spelled(value, module)
		seen := map[string]bool{}
		for _, name := range result {
			seen[name] = true
		}
		for _, name := range spelled(parentBackslash.Replace(value), module) {
			if !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
		return result
	}
	values := []string{"", "</script> &LT;/script&gt;", `..\data\case`, "%03d.go %s/a %/bad %% ../x", "example.test/m/a/b", " s/a x//a a/./b a/../b /a/b/ ", "éa/b中"}
	random := rand.New(rand.NewPCG(4726, 21))
	for range 1000 {
		body := make([]byte, 256)
		for i := range body {
			body[i] = byte(random.UintN(256))
		}
		values = append(values, string(body))
	}
	for _, value := range values {
		for _, module := range []string{"", "example.test/m"} {
			if got, want := literalTokens(value, module), legacy(value, module); !reflect.DeepEqual(got, want) {
				t.Fatalf("tokens(%q, %q) = %q, want %q", value, module, got, want)
			}
		}
	}
}

// AFP-V0-551: appending later literals must not overwrite the token spans
// kept for fixture markers; an unmarked occurrence still blocks exemption.
func TestPathTokensFixtureSpansSurviveAppend(t *testing.T) {
	body := []byte("var a = `fixtures/only.json` " + FixturePathMarker + "\n" +
		"var b = `fixtures/shared.json` " + FixturePathMarker + "\n" +
		strings.Repeat("var c = `fixtures/other.json`\n", 100) +
		"var d = `fixtures/shared.json`\n")
	scan := pathTokens(body, "", nil)
	if scan.err != nil || len(scan.fixture) != 1 || !scan.fixture["fixtures/only.json"] {
		t.Fatalf("fixture tokens = %v, error = %v", scan.fixture, scan.err)
	}
}
