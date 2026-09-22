package golang

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadModulePathMatchesGoModEdit pins readModulePath to the module path
// `go mod edit -json` reports for the same go.mod, including forms the naive
// line scan can misread: a trailing "// comment", a directive-shaped
// identifier ("module_test") that is not the module directive, and a
// directive whose value starts with "module" ("modulex foo").
func TestReadModulePathMatchesGoModEdit(t *testing.T) {
	cases := map[string]string{
		"module example.com/plain\n\ngo 1.27.0\n":                                   "example.com/plain",
		"module \"example.com/quoted\"\n\ngo 1.27.0\n":                              "example.com/quoted",
		"module example.com/commented // note\n\ngo 1.27.0\n":                       "example.com/commented",
		"modulex foo\n\nmodule example.com/real\n\ngo 1.27.0\n":                     "example.com/real",
		"module_test\n\nmodule example.com/real2\n\ngo 1.27.0\n":                    "example.com/real2",
		"// module fake.example.com/decoy\nmodule example.com/real3\n\ngo 1.27.0\n": "example.com/real3",
		"module example.com/crlf\r\n\r\ngo 1.27.0\r\n":                              "example.com/crlf",
		"go 1.27.0\n\nmodule example.com/afterGo\n":                                 "example.com/afterGo",
	}
	for content, want := range cases {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := readModulePath(dir)
		if err != nil {
			t.Errorf("readModulePath(%q) error = %v, want %q", content, err, want)
			continue
		}
		if got != want {
			t.Errorf("readModulePath(%q) = %q, want %q", content, got, want)
		}
	}
}

// TestReadModulePathAbstainsOnBOM pins the abstention `go mod edit -json`
// itself takes: a leading UTF-8 BOM is not a valid go.mod (the real toolchain
// reports an "unexpected input character" error for the BOM rune), so
// readModulePath must error rather than invent a module path.
func TestReadModulePathAbstainsOnBOM(t *testing.T) {
	dir := t.TempDir()
	content := "\xEF\xBB\xBFmodule example.com/bom\n\ngo 1.27.0\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := readModulePath(dir); err == nil {
		t.Errorf("readModulePath(BOM) = %q, want an error", got)
	}
}
