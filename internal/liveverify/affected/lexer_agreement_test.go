package affected_test

import (
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// lexTokens lexes body with mode and returns each token's kind and text,
// leaving out comments when skip is set.
func lexTokens(body []byte, mode scanner.Mode, skip bool) []string {
	var lexer scanner.Scanner
	lexer.Init(token.NewFileSet().AddFile("", -1, len(body)), body, nil, mode)
	var tokens []string
	for {
		_, kind, text := lexer.Scan()
		if kind == token.EOF {
			return tokens
		}
		if skip && kind == token.COMMENT {
			continue
		}
		tokens = append(tokens, kind.String()+text)
	}
}

// TestTwoGoLexersAgreeBeyondComments_AFPV0765 (V1-2206) measures, over every
// tracked Go file of this repository, the comment-mode difference between the
// fast-tier index's lexer (internal/gateselect, ScanComments) and this graph's
// (mode 0): with comments dropped the two token streams are equal, and no
// comment splits a qualified identifier, so the only effect of that
// difference, a comment breaking a root-locating `os.Getwd` match there but
// not here, reaches no file today.
func TestTwoGoLexersAgreeBeyondComments_AFPV0765(t *testing.T) {
	root := repositoryRoot(t)
	listing, err := exec.Command("git", "-C", root, "ls-files", "-z", "--", "*.go").Output()
	if err != nil {
		t.Fatal(err)
	}
	files, split := 0, 0
	for _, name := range strings.Split(strings.TrimRight(string(listing), "\x00"), "\x00") {
		if strings.Contains("/"+name, "/testdata/") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			continue
		}
		files++
		withComments := lexTokens(body, scanner.ScanComments, false)
		if !slices.Equal(lexTokens(body, scanner.ScanComments, true), lexTokens(body, 0, false)) {
			t.Errorf("%s: token streams differ beyond comments", name)
		}
		for i := 2; i < len(withComments); i++ {
			if strings.HasPrefix(withComments[i-1], "COMMENT") && (withComments[i-2] == "." || strings.HasPrefix(withComments[i-2], "IDENT") && withComments[i] == ".") {
				split++
			}
		}
	}
	if files < 1000 {
		t.Fatalf("measured %d tracked Go files, want the whole repository", files)
	}
	if split != 0 {
		t.Errorf("%d comments split a qualified identifier, so the comment mode changes a root-locator match", split)
	}
	t.Logf("measured %d tracked Go files: equal streams without comments, %d split qualifiers", files, split)
}
