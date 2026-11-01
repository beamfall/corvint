package contextindex

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// The regexes documentReferences and genericImportSpecifiers replaced
// (V1-2746), kept as their oracle.
var (
	documentReference = regexp.MustCompile(`(?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+`)
	genericImport     = regexp.MustCompile(`(?:from[\t-\r \x1c-\x1f\x{0085}\p{Z}]+|import[\t-\r \x1c-\x1f\x{0085}\p{Z}]*(?:\([\t-\r \x1c-\x1f\x{0085}\p{Z}]*)?)[\x22\x27]([^\x22\x27]+)[\x22\x27]`)
)

// genericImportOracle is the regex loop sourceImports ran before V1-2746.
func genericImportOracle(text string) map[string]struct{} {
	result := map[string]struct{}{}
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

var referenceScanEdgeCases = []string{
	"", "/", "a", "a/", "/a", "a/b", "a//b", "a/b/", "/a/b/", "a/b//c/d", "x a/b/c y", "a/b/c/d/e/f",
	"..//../a/.b", "-/-", "é/a/b", "a/é/b", "a/b\x80/c", "docs/specs/x.md.", "(see `cmd/corvint/main.go:12`)",
	"https://example.com/a/b?c=d/e", "a/b c/d\ne/f", strings.Repeat("a/", 300) + "z",
	`from "x"`, `from"x"`, `import"x"`, `import ( 'a/b' )`, `import ('a')`, `import ("")`, `from  'a'"`,
	`xfrom "a"`, `_import "a"`, `éimport "a"`, `.import "a"`, "import \"a\"", "from 'a'", "from\x85'a'",
	"import \"a\nb\"", `import "a`, `import "a'`, `from from "a"`, `import import "a"`, `fromimport "a"`,
	`import "a" from "b" import 'c'`, "import\t(\t\"q\")", `import "x"import "y"`, "from \xff'a'",
}

func TestReferenceScannersMatchTheirRegexes(t *testing.T) {
	check := func(name, text string) {
		t.Helper()
		if got, want := documentReferences(text), documentReference.FindAllString(text, -1); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: documentReferences = %q, regex %q", name, got, want)
		}
		got := map[string]struct{}{}
		genericImportSpecifiers(text, got)
		if want := genericImportOracle(text); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: genericImportSpecifiers = %v, regex %v", name, got, want)
		}
	}
	for index, text := range referenceScanEdgeCases {
		check("edge "+string(rune('A'+index%26)), text)
	}
	// The self corpus: every tracked file of this checkout up to the source bound.
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	listed, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Skipf("no checkout to read: %v", err)
	}
	checked := 0
	for _, name := range strings.Split(string(listed), "\x00") {
		if name == "" {
			continue
		}
		info, err := os.Lstat(filepath.Join(root, name))
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxSourceBytes {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		check(name, string(data))
		checked++
	}
	if checked < 1000 {
		t.Fatalf("checked %d tracked files, want the whole checkout", checked)
	}
}
