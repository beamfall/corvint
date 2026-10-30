// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"strings"
	"testing"
)

// AFP-V0-828 (V1-3137): a path prefix that a declared package's tests spell only
// inside temp-root-only functions, such as the subject ID ".github/workflows/" +
// file of a fixture written under t.TempDir(), is not resolved from the
// repository root, so it needs no recorded literal. Every way a function could
// still reach the root keeps the read flagged, as does any other occurrence.
func TestTempRootOnlyLiteralsAreNoRepositoryRead_V1_3137(t *testing.T) {
	parent := strings.TrimSuffix(up, "/")
	climb := "\"" + parent + "\", \"" + parent + "\""
	header := "package doccorpus\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"runtime\"\n\t\"testing\"\n)\n\n" +
		"var _, _, _ = exec.Command, runtime.Caller, os.ReadFile\n\n" +
		// A package-level climb makes the package a rule (d) root reader.
		"var guide = filepath.Join(" + climb + ", \"docs\")\n\n" +
		"func writeFixture(t *testing.T, root string, files map[string]string) {\n\tfor name, content := range files {\n" +
		"\t\t_ = os.WriteFile(filepath.Join(root, name), []byte(content), 0o600)\n\t}\n}\n\n"
	inventory := func(extra string) string {
		return "func TestInventory(t *testing.T) {\n\twriteFixture(t, t.TempDir(), map[string]string{\".github/workflows/ci.yml\": \"on: push\\n\"})\n" + extra +
			"\tfor _, file := range []string{\"ci.yml\"} {\n\t\tid := \"native:deployable:.github/workflows/\" + file + \":deploy\"\n\t\t_ = id\n\t}\n}\n\n"
	}
	realRead := "func TestReal(t *testing.T) {\n\t_, _ = os.ReadFile(filepath.Join(" + climb + ", \".github/workflows/ci.yml\"))\n}\n\n"
	repoRoot := "func repoRoot() string { return filepath.Join(" + climb + ") }\n"
	const ok = "test-read-scopes-check: ok"
	const fakeOS = "type fakeOS struct{}\n\nfunc (fakeOS) ReadFile(string) ([]byte, error) { return nil, nil }\n\n" +
		"func (fakeOS) MkdirTemp(string, string) (string, error) { return \"\", nil }\n"
	const workflow = "internal/doccorpus/inventory_test.go names .github/workflows/ci.yml, outside its entries"
	const prefix = "internal/doccorpus/inventory_test.go names .github/workflows, outside its entries"
	for name, c := range map[string]struct {
		test, other, imports string
		code                 int
		want                 string
	}{
		"temp-root-only fixture and subject ID": {test: inventory(""), want: ok},
		"raw-string quoting":                    {test: inventory("\t_ = `.github/workflows/` + \"x\"\n"), want: ok},
		"temp root made inside a closure": {test: "func TestClosure(t *testing.T) {\n\tfunc() {\n\t\troot := t.TempDir()\n" +
			"\t\t_ = filepath.Join(root, \".github/workflows/ci.yml\")\n\t}()\n}\n", want: ok},
		"os.MkdirTemp root": {test: "func TestMkdir(t *testing.T) {\n\troot, _ := os.MkdirTemp(\"\", \"x\")\n" +
			"\t_ = filepath.Join(root, \".github/workflows/ci.yml\")\n}\n", want: ok},

		// Refusals: a real read through the same prefix, ordering, indirection,
		// aliasing and unobserved state each keep the literal flagged.
		"real read in another function":    {test: inventory("") + realRead, code: 1, want: workflow},
		"real read first, fixture second":  {test: realRead + inventory(""), code: 1, want: workflow},
		"same function climbs to the root": {test: inventory("\t_ = filepath.Join(" + climb + ")\n"), code: 1, want: workflow},
		"closure climbs to the root":       {test: inventory("\tfunc() { _ = filepath.Join(" + climb + ") }()\n"), code: 1, want: workflow},
		"helper climbs to the root":        {test: inventory("\t_ = repoRoot()\n") + repoRoot, code: 1, want: workflow},
		"transitive helper climbs": {test: inventory("\t_ = fixtureRoot()\n") + "func fixtureRoot() string { return repoRoot() }\n\n" + repoRoot,
			code: 1, want: workflow},
		"helper in another test file": {test: inventory("\t_ = repoRoot()\n"),
			other: "package doccorpus\n\nimport \"path/filepath\"\n\n" + repoRoot, code: 1, want: workflow},
		"method helper climbs": {test: inventory("\t_ = harness{}.root()\n") + "type harness struct{}\n\n" +
			"func (harness) root() string { return filepath.Join(" + climb + ") }\n", code: 1, want: workflow},
		"package variable climbs": {test: inventory("\t_ = guide\n"), code: 1, want: workflow},
		"package constant prefix": {test: "const workflows = \".github/workflows/\"\n\nfunc TestConst(t *testing.T) {\n" +
			"\troot := t.TempDir()\n\t_ = filepath.Join(root, workflows+\"ci.yml\")\n}\n", code: 1, want: prefix},
		"runtime.Caller locates the source": {test: inventory("\t_, _, _, _ = runtime.Caller(0)\n"), code: 1, want: workflow},
		"git root lookup":                   {test: inventory("\t_ = exec.Command(\"git\", \"rev-parse\", \"" + rootLocatorToken + "\")\n"), code: 1, want: workflow},
		"go env GOMOD":                      {test: inventory("\t_ = exec.Command(\"go\", \"env\", \"GOMOD\")\n"), code: 1, want: workflow},
		"Chdir anywhere in the package": {test: inventory(""), other: "package doccorpus\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\n" +
			"func TestMain(m *testing.M) {\n\t_ = os.Chdir(\"/\")\n\tos.Exit(m.Run())\n}\n", code: 1, want: workflow},
		"no temp root": {test: "func TestPlain(t *testing.T) {\n\tid := \"native:deployable:.github/workflows/\" + t.Name()\n\t_ = id\n}\n",
			code: 1, want: prefix},
		"reassigned testing parameter": {test: "func TestShadow(t *testing.T) {\n\tvar other *testing.T\n\tt = other\n\troot := t.TempDir()\n" +
			"\t_ = filepath.Join(root, \".github/workflows/ci.yml\")\n}\n", code: 1, want: workflow},
		"TempDir on a non-testing receiver": {test: "type fakeFS struct{}\n\nfunc (fakeFS) TempDir() string { return \"\" }\n\n" +
			"func TestFake(t *testing.T) {\n\tvar fs fakeFS\n\t_ = filepath.Join(fs.TempDir(), \".github/workflows/ci.yml\")\n}\n", code: 1, want: workflow},
		"cross-package helper":    {test: inventory("\t_ = repo.Root()\n"), imports: "\t\"example.test/m/internal/repo\"\n", code: 1, want: workflow},
		"non-standard dot import": {test: inventory("\t_ = Root()\n"), imports: "\t. \"example.test/m/internal/repo\"\n", code: 1, want: workflow},
		// Review repairs: a shadowed import, a standard dot import and an
		// import whose package name the path does not spell all withhold it.
		"shadowed import receiver": {test: inventory("\tos := fakeOS{}\n\t_, _ = os.ReadFile(\"x\")\n") + fakeOS, code: 1, want: workflow},
		"shadowed os.MkdirTemp is no temp root": {test: "func TestShadowMkdir(t *testing.T) {\n\tos := fakeOS{}\n\troot, _ := os.MkdirTemp(\"\", \"x\")\n" +
			"\t_ = filepath.Join(root, \".github/workflows/ci.yml\")\n}\n" + fakeOS, code: 1, want: workflow},
		"dot-imported runtime.Caller":  {test: inventory("\t_, _, _, _ = Caller(0)\n"), imports: "\t. \"runtime\"\n", code: 1, want: workflow},
		"dot-imported os.Getwd":        {test: inventory("\t_, _ = Getwd()\n"), imports: "\t. \"os\"\n", code: 1, want: workflow},
		"dot-imported filepath.Abs":    {test: inventory("\t_, _ = Abs(\".\")\n"), imports: "\t. \"path/filepath\"\n", code: 1, want: workflow},
		"import named unlike its path": {test: inventory("\t_ = yaml.Unmarshal\n"), imports: "\t\"gopkg.in/yaml.v3\"\n", code: 1, want: workflow},
		"local value selector":         {test: inventory("\th := struct{ root string }{t.TempDir()}\n\t_ = filepath.Join(h.root, \"ci.yml\")\n"), want: ok},
	} {
		test := strings.Replace(header, "\t\"testing\"\n)", "\t\"testing\"\n"+c.imports+")", 1) + c.test
		files := map[string]string{
			"go.mod":                               "module example.test/m\n\ngo 1.27\n",
			".github/workflows/ci.yml":             "on: push\n",
			"docs/guide.md":                        "guide\n",
			"internal/repo/r.go":                   "package repo\n\nfunc Root() string { return \"\" }\n",
			"internal/doccorpus/d.go":              "package doccorpus\n",
			"internal/doccorpus/inventory_test.go": test,
			".corvint/test-read-scopes.json":       `{"profile":"corvint-test-read-scopes/0","packages":{"internal/doccorpus":["docs/"]}}`,
		}
		if c.other != "" {
			files["internal/doccorpus/other_test.go"] = c.other
		}
		var stdout, stderr bytes.Buffer
		code := run(writeRepository(t, files), &stdout, &stderr)
		output := stdout.String() + stderr.String()
		if code != c.code || !strings.Contains(output, c.want) {
			t.Errorf("%s: code=%d want %d; output %q lacks %q", name, code, c.code, output, c.want)
		}
	}
}

// A test file that does not parse is an error, never an exemption.
func TestTempRootOnlyRefusesAnUnparsableFile_V1_3137(t *testing.T) {
	if _, err := tempRootOnly([]string{"x_test.go"}, [][]byte{[]byte("package x\n\nfunc TestX(t *testing.T) {\n")}); err == nil {
		t.Fatal("an unparsable test file yielded an exemption set")
	}
}
