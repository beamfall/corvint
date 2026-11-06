// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"math/rand/v2"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// AFP-V0-021: compare the allocation-reduced lexer against the original
// regexp lexicon, including all byte values and short outer components.
func TestLiteralTokensMatchRegexpLexicon(t *testing.T) {
	pattern := regexp.MustCompile(`[A-Za-z0-9._~@+/-]+`)
	legacy := func(value, module string) []string {
		value = parentBackslash.Replace(printfVerb.ReplaceAllString(value, verbMark))
		var result []string
		for _, span := range pattern.FindAllStringIndex(value, -1) {
			start, end := span[0], span[1]
			if value[start] == '/' && (start > 0 && value[start-1] == '<' || closesEscapedTag(value, start)) {
				start++
				if start == end {
					continue
				}
			}
			name := rootAnchored(value[start:end], module)
			parts := strings.Split(name, "/")
			if last := len(parts) - 1; last > 0 {
				if short(parts[0]) && runComponent(parts[1]) && delimited(value, start-1) {
					name = "./" + name
				}
				if short(parts[last]) && runComponent(parts[last-1]) && delimited(value, end) {
					name += "/"
				}
			}
			result = append(result, name)
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
