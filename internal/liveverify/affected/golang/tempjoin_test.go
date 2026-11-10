// SPDX-License-Identifier: Apache-2.0
package golang

import (
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

// tempJoinTokens scans one Go file as observeDirectory does and returns its
// path tokens, so each case states which literals survive the AFP-V0-9810
// proof.
func tempJoinTokens(t *testing.T, body string) []string {
	t.Helper()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "x_test.go", []byte(body), parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	scan := pathTokens([]byte(body)[importsEnd(fileSet, file):], "example.test/m", file.Imports)
	if scan.err != nil {
		t.Fatal(scan.err)
	}
	return scan.tokens
}

const tempJoinHeader = "package x\n\nimport (\n\t\"os\"\n\t\"path/filepath\"\n\t\"testing\"\n)\n\n"

// TestTempJoinComponents_AFP_V0_9810 drops a literal joined only to a
// test-scoped temporary root and keeps every other literal: a repository
// read, a literal spelled elsewhere, a join from any other root, a reassigned
// or addressed variable, a package-level variable, a receiver that is not a
// testing parameter, and a component that climbs or globs.
func TestTempJoinComponents_AFP_V0_9810(t *testing.T) {
	for _, tc := range []struct {
		name, body   string
		absent, kept []string
	}{
		{"direct-join", "func TestA(t *testing.T) { _ = filepath.Join(t.TempDir(), \"docs\", \"guide.md\") }\n", []string{"docs", "guide.md"}, nil},
		{"variable-root", "func TestA(t *testing.T) { root := t.TempDir(); _ = filepath.Join(root, \"docs/specs\", \"x.md\") }\n", []string{"docs/specs", "x.md"}, nil},
		{"var-declaration", "func TestA(t *testing.T) { var root = t.TempDir(); _ = filepath.Join(root, \"docs/x.md\") }\n", []string{"docs/x.md"}, nil},
		{"join-chain", "func TestA(t *testing.T) { root := t.TempDir(); dir := filepath.Join(root, \"repo\"); _ = filepath.Join(dir, \"docs\", \"y.md\") }\n", []string{"repo", "docs", "y.md"}, nil},
		{"benchmark-and-tb", "func BenchmarkA(b *testing.B) { _ = filepath.Join(b.TempDir(), \"docs/b.md\") }\nfunc helper(tb testing.TB) { _ = filepath.Join(tb.TempDir(), \"docs/tb.md\") }\n", []string{"docs/b.md", "docs/tb.md"}, nil},
		{"os-tempdir", "func TestA(t *testing.T) { _ = filepath.Join(os.TempDir(), \"docs/os.md\") }\n", []string{"docs/os.md"}, nil},
		{"repository-read-kept", "func TestA(t *testing.T) { root := t.TempDir(); _ = filepath.Join(root, \"docs/a.md\"); _, _ = os.ReadFile(\"docs/b.md\") }\n", []string{"docs/a.md"}, []string{"docs/b.md"}},
		{"spelled-elsewhere-kept", "func TestA(t *testing.T) { _ = filepath.Join(t.TempDir(), \"docs/c.md\"); _ = \"docs/c.md\" }\n", nil, []string{"docs/c.md"}},
		{"other-root-kept", "func TestA(t *testing.T) { root := \".\"; _ = filepath.Join(root, \"docs/d.md\") }\n", nil, []string{"docs/d.md"}},
		{"getwd-root-kept", "func TestA(t *testing.T) { root, _ := os.Getwd(); _ = filepath.Join(root, \"docs/e.md\") }\n", nil, []string{"docs/e.md"}},
		{"reassigned-kept", "func TestA(t *testing.T) { root := t.TempDir(); root = \".\"; _ = filepath.Join(root, \"docs/f.md\") }\n", nil, []string{"docs/f.md"}},
		{"addressed-kept", "func TestA(t *testing.T) { root := t.TempDir(); p := &root; _ = p; _ = filepath.Join(root, \"docs/g.md\") }\n", nil, []string{"docs/g.md"}},
		{"package-level-kept", "var root = os.TempDir()\nfunc TestA(t *testing.T) { _ = filepath.Join(root, \"docs/h.md\") }\n", nil, []string{"docs/h.md"}},
		{"custom-receiver-kept", "type fake struct{}\nfunc (fake) TempDir() string { return \".\" }\nfunc TestA(t *testing.T) { f := fake{}; _ = filepath.Join(f.TempDir(), \"docs/i.md\") }\n", nil, []string{"docs/i.md"}},
		{"reassigned-parameter-kept", "func TestA(t *testing.T) { t = nil; _ = filepath.Join(t.TempDir(), \"docs/j.md\") }\n", nil, []string{"docs/j.md"}},
		{"climbing-component-kept", "func TestA(t *testing.T) { _ = filepath.Join(t.TempDir(), \"..\", \"docs/k.md\") }\n", nil, []string{"docs/k.md"}},
		{"glob-component-kept", "func TestA(t *testing.T) { _ = filepath.Join(t.TempDir(), \"docs/*.md\") }\n", nil, []string{"docs/"}},
		{"non-literal-argument-kept", "func TestA(t *testing.T) { name := \"docs/l.md\"; _ = filepath.Join(t.TempDir(), name) }\n", nil, []string{"docs/l.md"}},
		{"shadowed-join-kept", "func TestA(t *testing.T) { filepath := struct{ Join func(...string) string }{}; _ = filepath.Join(t.TempDir(), \"docs/m.md\") }\n", nil, []string{"docs/m.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens := tempJoinTokens(t, tempJoinHeader+tc.body)
			for _, token := range tc.absent {
				if slices.Contains(tokens, token) {
					t.Errorf("%q is a token of %v; the literal is joined only to a temporary root", token, tokens)
				}
			}
			for _, token := range tc.kept {
				if !slices.Contains(tokens, token) {
					t.Errorf("%q is not a token of %v; the literal may name a repository read", token, tokens)
				}
			}
		})
	}
}

// TestTempJoinComponentsNeedsImports_AFP_V0_9810 keeps every literal when the
// file imports neither testing nor os, or no join package, and when it does
// not parse: the proof grants nothing it cannot read.
func TestTempJoinComponentsNeedsImports_AFP_V0_9810(t *testing.T) {
	body := []byte("func TestA(t *testing.T) { _ = filepath.Join(t.TempDir(), \"docs/n.md\") }\n")
	for name, aliases := range map[string]map[string]string{
		"no-testing": {"filepath": "path/filepath"},
		"no-join":    {"testing": "testing"},
		"nil":        nil,
	} {
		if got := tempJoinComponents(body, aliases); got != nil {
			t.Errorf("%s: components %v, want none", name, got)
		}
	}
	aliases := map[string]string{"testing": "testing", "filepath": "path/filepath"}
	if got := tempJoinComponents([]byte("func TestA(t *testing.T) { _ = filepath.Join(t.TempDir(), \"docs/o.md\"\n"), aliases); got != nil {
		t.Errorf("unparsable: components %v, want none", got)
	}
	if got := tempJoinComponents(body, aliases); len(got) != 1 {
		t.Errorf("components %v, want the one joined literal", got)
	}
}
