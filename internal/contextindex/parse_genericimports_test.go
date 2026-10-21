package contextindex

import (
	"reflect"
	"testing"
	"unicode/utf8"
)

// genericImportsOracle is the pre-V1-1640 extraction: genericImport run over
// the whole body, dropping matches preceded by a Python word rune.
func genericImportsOracle(text string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, match := range genericImport.FindAllStringSubmatchIndex(text, -1) {
		if match[0] != 0 {
			previous, _ := utf8.DecodeLastRuneInString(text[:match[0]])
			if isPythonWordRune(previous) {
				continue
			}
		}
		result[text[match[2]:match[3]]] = struct{}{}
	}
	return result
}

var genericImportsSeeds = []string{
	"",
	"from",
	"import",
	`import "a"`,
	`import"a"`,
	`import ( "a" )`,
	`import("a")`,
	`import(  'b'`,
	`from "a"`,
	`from"a"`,
	`from  	 'a' import "b"`,
	`xfrom "a" ximport 'b' _from "c" 9import "d" -from "e"`,
	`import "from 'x'"`,
	`from "import 'y'" from 'z'`,
	`import ""`,
	`import "unterminated`,
	"import \"multi\nline\"",
	"from \"nbsp\" from 'ls' from\u0085\"nel\" from\u001c'fs' from​\"zwsp\"",
	"\xffimport \"invalid-lead\" \xe2import 'x'",
	"import \"\xff\xfe\"",
	"éfrom \"letter-before\" 1from 'digit' ·from \"middot\"",
	`importimport "a" fromfrom 'b' fromimport "c"`,
	`import ((("a")))`,
	`import ( 'a'`,
	"text with no keyword at all, but \"quotes\" and 'more'",
}

func TestGenericImportsMatchesRegexpOracle(t *testing.T) {
	for _, text := range genericImportsSeeds {
		if got, want := genericImports(text), genericImportsOracle(text); !reflect.DeepEqual(got, want) {
			t.Errorf("genericImports(%q) = %v, oracle %v", text, got, want)
		}
	}
}

func FuzzGenericImportsMatchesRegexpOracle(f *testing.F) {
	for _, seed := range genericImportsSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if got, want := genericImports(text), genericImportsOracle(text); !reflect.DeepEqual(got, want) {
			t.Fatalf("genericImports(%q) = %v, oracle %v", text, got, want)
		}
	})
}
