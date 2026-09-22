package contextindex

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestGoImportsFormsTheScannerMishandles pins the concrete forms that motivated
// replacing the line scanner. Each case is valid Go whose true import set the
// scanner reports incorrectly.
func TestGoImportsFormsTheScannerMishandles(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		source string
		want   []string
	}{
		{
			// The scanner treats a line that is exactly ")" as the end of the
			// block, so everything after the comment is dropped.
			name:   "block comment closing paren inside import block",
			source: "package p\n\nimport (\n\t\"a/one\"\n\t/*\n\t)\n\t*/\n\t\"a/two\"\n)\n",
			want:   []string{"a/one", "a/two"},
		},
		{
			// No space after the keyword: neither the "import (" nor the
			// "import " scanner prefix matches, so the whole block is invisible.
			name:   "grouped import with no space before paren",
			source: "package p\n\nimport(\n\t\"b/one\"\n)\n",
			want:   []string{"b/one"},
		},
		{
			name:   "single line grouped import",
			source: "package p\n\nimport (\"c/one\"; \"c/two\")\n",
			want:   []string{"c/one", "c/two"},
		},
		{
			// The scanner takes the first quoted string on the line, which here
			// is the commented-out path rather than the real one.
			name:   "comment inside block naming another path",
			source: "package p\n\nimport (\n\t// superseded by \"d/old\"\n\t\"d/new\"\n)\n",
			want:   []string{"d/new"},
		},
		{
			name:   "blank and dot imports",
			source: "package p\n\nimport (\n\t_ \"e/blank\"\n\t. \"e/dot\"\n\talias \"e/named\"\n)\n",
			want:   []string{"e/blank", "e/dot", "e/named"},
		},
		{
			// Nothing after the import declarations is an import, however it is
			// quoted; ImportsOnly stops, the scanner keeps matching "import ".
			name:   "quoted path in a later statement",
			source: "package p\n\nimport \"f/real\"\n\nvar Doc = \"import \\\"f/fake\\\"\"\n",
			want:   []string{"f/real"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := sortedImportSet(goImportSet(testCase.source)); !stringSlicesEqual(got, testCase.want) {
				t.Fatalf("goImports = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestGoImportsStaysTotalOnUnparseableSource fixes the parse-failure contract:
// a source the parser rejects must still surrender every import edge the legacy
// scanner could see, because an empty set reads as "imports nothing" and would
// let internal/workqueue omit a real collision group.
func TestGoImportsStaysTotalOnUnparseableSource(t *testing.T) {
	// A bare fragment with no package clause: the parser refuses it outright and
	// recovers no imports at all, so only the fallback can supply the edges.
	broken := "import (\n\t\"g/one\"\n\t\"g/two\"\n)\n"
	if _, err := parser.ParseFile(token.NewFileSet(), "source.go", broken, parser.ImportsOnly); err == nil {
		t.Fatal("fixture parses; it must not, or the fallback is untested")
	}
	got := sortedImportSet(goImportSet(broken))
	for _, want := range []string{"g/one", "g/two"} {
		if !containsString(got, want) {
			t.Fatalf("goImports on unparseable source = %v, missing %q", got, want)
		}
	}
	scanned := goImportsScan(broken)
	for value := range scanned {
		if !containsString(got, value) {
			t.Fatalf("parse-failure result %v drops scanner edge %q", got, value)
		}
	}
}

// TestGoImportsDifferentialAgainstScanner runs both implementations over every
// .go file in this repository and reports each disagreement, so the claim that
// the scanner is unsound is demonstrated on real input rather than asserted.
func TestGoImportsDifferentialAgainstScanner(t *testing.T) {
	root := repositoryRoot(t)
	missed, spurious, files, unparseable := 0, 0, 0, 0
	err := filepath.WalkDir(root, func(candidate string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		data, readErr := os.ReadFile(candidate)
		if readErr != nil {
			return readErr
		}
		files++
		relative, _ := filepath.Rel(root, candidate)
		if _, parseErr := parser.ParseFile(token.NewFileSet(), "source.go", data, parser.ImportsOnly); parseErr != nil {
			// goImports unions the scanner in on parse failure, so such a file
			// could not report a scanner miss even if one existed. Count them so
			// the differential's "missed" figure is known to be unconfounded.
			unparseable++
		}
		parsed, scanned := goImportSet(string(data)), goImportsScan(string(data))
		for value := range parsed {
			if _, ok := scanned[value]; !ok {
				missed++
				t.Logf("DIFFERENTIAL scanner MISSED %s: %q", relative, value)
			}
		}
		for value := range scanned {
			if _, ok := parsed[value]; !ok {
				spurious++
				t.Logf("DIFFERENTIAL scanner SPURIOUS %s: %q", relative, value)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if files == 0 {
		t.Fatal("differential scanned no Go files")
	}
	t.Logf("DIFFERENTIAL files=%d unparseable=%d missed-by-scanner=%d spurious-from-scanner=%d", files, unparseable, missed, spurious)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Join(working, "..", "..")
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repository root %s has no go.mod: %v", root, err)
	}
	return root
}

func sortedImportSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// goImportSet is goImports without its refusal reason, for the cases below that
// assert on the edge set alone. The reason itself is covered by
// TestGoImportsReportsRefusal.
func goImportSet(text string) map[string]struct{} {
	imports, _ := goImports(text)
	return imports
}
