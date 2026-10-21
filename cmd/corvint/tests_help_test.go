package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// testsHelpCode is a finding code as testsHelp spells it: upper-case words joined by hyphens,
// the first at least two characters (so "lines N-M" is not one), optionally ending in a "*" wildcard (ENV-HARD-CODED-*).
var testsHelpCode = regexp.MustCompile(`\b[A-Z][A-Z0-9]+(?:-[A-Z0-9]+)+(?:-\*)?`)

// testsHelpNotACode matches the hyphenated upper-case tokens of testsHelp that are spec or
// ticket identifiers (TSA-V0, V1-1587), not finding codes.
var testsHelpNotACode = regexp.MustCompile(`^(?:[A-Z]+-V\d+|V\d+-\d+)$`)

// testSourceRuleCodes reads every finding code the test-source readers can emit: the
// string constants named Rule*, Code* or *Rule in internal/testvalidity's source_rules*.go
// files and in internal/testsource. It reads the declarations, so a rule added there without
// a help entry fails TestTestsHelpCodesMatchTheReaderCodes.
func testSourceRuleCodes(t *testing.T) map[string]string {
	t.Helper()
	codes := map[string]string{}
	fset := token.NewFileSet()
	for _, dir := range []struct{ path, glob string }{
		{filepath.Join("..", "..", "internal", "testvalidity"), "source_rules*.go"},
		{filepath.Join("..", "..", "internal", "testsource"), "*.go"},
	} {
		files, err := filepath.Glob(filepath.Join(dir.path, dir.glob))
		if err != nil || len(files) == 0 {
			t.Fatalf("no rule source under %s: %v", dir.path, err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := parser.ParseFile(fset, file, src, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, decl := range parsed.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.CONST {
					continue
				}
				for _, spec := range gen.Specs {
					value := spec.(*ast.ValueSpec)
					for i, name := range value.Names {
						if i >= len(value.Values) {
							continue
						}
						lit, ok := value.Values[i].(*ast.BasicLit)
						if !ok || lit.Kind != token.STRING {
							continue
						}
						n := name.Name
						if !strings.HasPrefix(n, "Rule") && !strings.HasPrefix(n, "Code") && !strings.HasSuffix(n, "Rule") {
							continue
						}
						code, err := strconv.Unquote(lit.Value)
						if err != nil || !testsHelpCode.MatchString(code) || testsHelpCode.FindString(code) != code {
							continue
						}
						codes[code] = filepath.Base(file) + ":" + n
					}
				}
			}
		}
	}
	return codes
}

// V1-1892: every finding code testsHelp names is one a test-source reader can emit, and every
// code a reader can emit is named in testsHelp, directly or by a trailing "*" wildcard.
func TestTestsHelpCodesMatchTheReaderCodes(t *testing.T) {
	emitted := testSourceRuleCodes(t)
	if len(emitted) < 40 {
		t.Fatalf("read only %d reader codes; the rule declarations moved", len(emitted))
	}
	help := strings.Join(strings.Fields(testsHelp), " ")
	var named, wildcards []string
	for _, code := range testsHelpCode.FindAllString(help, -1) {
		if testsHelpNotACode.MatchString(code) {
			continue
		}
		if prefix, ok := strings.CutSuffix(code, "*"); ok {
			wildcards = append(wildcards, prefix)
			continue
		}
		named = append(named, code)
	}
	var drift []string
	for _, code := range named {
		if _, ok := emitted[code]; !ok {
			drift = append(drift, "help names "+code+", which no test-source reader emits")
		}
	}
	for _, prefix := range wildcards {
		matched := false
		for code := range emitted {
			matched = matched || strings.HasPrefix(code, prefix)
		}
		if !matched {
			drift = append(drift, "help names "+prefix+"*, which matches no reader code")
		}
	}
	listed := map[string]bool{}
	for _, code := range named {
		listed[code] = true
	}
	for code, origin := range emitted {
		covered := listed[code]
		for _, prefix := range wildcards {
			covered = covered || strings.HasPrefix(code, prefix)
		}
		if !covered {
			drift = append(drift, "reader code "+code+" ("+origin+") is missing from testsHelp")
		}
	}
	sort.Strings(drift)
	for _, d := range drift {
		t.Error(d)
	}
}
