package linecov

import (
	"errors"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// HeadTestNames lists the top-level tests `go test` would run of the
// _test.go files in each directory, read under any build constraint by a
// token scan (GLS-V0-016): a name only a tagged or other-platform build
// compiles is listed too, so it is kept, never excluded. A directory with a
// _test.go file but no named test (a TestMain-only package) lists an empty,
// non-nil slice: go test still builds and runs its binary. A directory
// without one, or that does not exist, lists nothing (nil); an unreadable
// file is an error.
func HeadTestNames(root string, dirs []string) (map[string][]string, error) {
	out := make(map[string][]string, len(dirs))
	for _, dir := range dirs {
		full := filepath.Join(root, filepath.FromSlash(dir))
		entries, err := os.ReadDir(full)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		var names []string
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, "_test.go") {
				continue
			}
			if names == nil {
				names = []string{} // a test binary, even without a named test
			}
			src, err := os.ReadFile(filepath.Join(full, name))
			if err != nil {
				return nil, err
			}
			names = append(names, testFuncNames(src)...)
		}
		sort.Strings(names)
		out[dir] = slices.Compact(names)
	}
	return out, nil
}

// testFuncNames returns the receiver-less functions declared at the top
// level of src whose names go test runs: Test, Example and Fuzz followed by
// nothing or by a character that is not a lower-case letter.
func testFuncNames(src []byte) []string {
	fset := token.NewFileSet()
	tf := fset.AddFile("", -1, len(src))
	var sc scanner.Scanner
	sc.Init(tf, src, nil, 0) // errors are skipped: a name is kept, never lost
	var out []string
	depth := 0 // braces, brackets and parentheses
	afterFunc := false
	for {
		_, tok, lit := sc.Scan()
		if tok == token.EOF {
			return out
		}
		switch tok {
		case token.LBRACE, token.LPAREN, token.LBRACK:
			if afterFunc {
				afterFunc = false // a receiver: a method, not a test
			}
			depth++
			continue
		case token.RBRACE, token.RPAREN, token.RBRACK:
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth != 0 {
			afterFunc = false
			continue
		}
		if tok == token.FUNC {
			afterFunc = true
			continue
		}
		if afterFunc && tok == token.IDENT && testName(lit) {
			out = append(out, lit)
		}
		afterFunc = false
	}
}

// testName applies go test's rule: TestX and Test_x run, Testhelper does
// not, and TestMain is the package's main, not a test.
func testName(name string) bool {
	if name == "TestMain" {
		return false
	}
	for _, prefix := range []string{"Test", "Example", "Fuzz"} {
		rest, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		r, _ := utf8.DecodeRuneInString(rest)
		return rest == "" || !unicode.IsLower(r)
	}
	return false
}
