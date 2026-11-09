package docmaintain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	commentJoin       = regexp.MustCompile(`\n[ \t]*(?://[ \t]?)?`)
	maturityStatement = regexp.MustCompile(`docs/specs/([A-Za-z0-9._-]+\.md)\)? remains ([a-z][a-z-]*)( \([^()]*\))?`)
)

// SDD-V0-634: every "docs/specs/NAME.md remains STATUS" statement in the
// non-test Go files of cmd/corvint and internal/docmaintain agrees with that
// spec's delivery in docs/specs/INDEX.json, including its parenthesized range.
func TestSDDV0634MaturityStringsMatchSpecIndex(t *testing.T) {
	root := filepath.Join("..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "docs", "specs", "INDEX.json"))
	if err != nil {
		t.Fatal(err)
	}
	var specs []struct{ Path, Delivery string }
	if err := json.Unmarshal(raw, &specs); err != nil {
		t.Fatal(err)
	}
	delivery := map[string]string{}
	for _, spec := range specs {
		delivery[filepath.Base(spec.Path)] = spec.Delivery
	}
	statements := 0
	for _, directory := range []string{"cmd/corvint", "internal/docmaintain"} {
		files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(directory), "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			for _, match := range maturityStatement.FindAllStringSubmatch(commentJoin.ReplaceAllString(string(source), " "), -1) {
				statements++
				want, known := delivery[match[1]]
				word, rest, _ := strings.Cut(want, " ")
				if !known || match[2] != word || (rest != "" && strings.HasPrefix(rest, "(") && match[3] != " "+rest) {
					t.Errorf("%s: %q disagrees with docs/specs/INDEX.json delivery %q", file, match[0], want)
				}
			}
		}
	}
	if statements < 2 {
		t.Fatalf("found %d maturity statements; the scan no longer sees the docmaintain statements", statements)
	}
}
