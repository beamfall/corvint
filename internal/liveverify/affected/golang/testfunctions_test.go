package golang

import (
	"reflect"
	"testing"
)

// STR-V0-063: a test file's declarations are read exactly: the test
// functions -run can name, every other top-level name, every identifier the
// file mentions, and whether code runs before any test.
func TestSTRV0063TestFileDeclarations(t *testing.T) {
	src := []byte(`package p

import "testing"

type fixture struct{ n int }

const limit = 3

var shared, other = 1, 2

func (f fixture) value() int { return f.n }

func helper() int { return limit }

func Testing(t *testing.T) {}
func Test(t *testing.T)    {}
func TestA(t *testing.T)   { _ = helper() }
func Test_b(t *testing.T)  {}
func ExampleA()            {}
func FuzzA(f *testing.F)   {}
func BenchmarkA(b *testing.B) {}
`)
	got, err := TestFileDeclarations("p_test.go", src)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Tests, []string{"ExampleA", "FuzzA", "Test", "TestA", "Test_b"}) {
		t.Fatalf("tests %v", got.Tests)
	}
	if !reflect.DeepEqual(got.Others, []string{"BenchmarkA", "Testing", "fixture", "helper", "limit", "other", "shared", "value"}) {
		t.Fatalf("others %v", got.Others)
	}
	for _, name := range []string{"helper", "limit", "testing", "TestA", "int"} {
		found := false
		for _, use := range got.Uses {
			found = found || use == name
		}
		if !found {
			t.Fatalf("uses %v lacks %s", got.Uses, name)
		}
	}
	if got.SideEffect != "" {
		t.Fatalf("side effect %q", got.SideEffect)
	}
	for src, want := range map[string]string{
		"package p\n\nfunc init() {}\n":                                            "declares init",
		"package p\n\nimport \"testing\"\n\nfunc TestMain(m *testing.M) {}\n":      "declares TestMain",
		"package p\n\nvar shared = compute()\n\nfunc compute() int { return 1 }\n": "initializes shared with a call",
		"package p\n\nvar plain = []int{1}\n":                                      "",
	} {
		got, err := TestFileDeclarations("p_test.go", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if got.SideEffect != want {
			t.Fatalf("%q: side effect %q, want %q", src, got.SideEffect, want)
		}
	}
	if _, err := TestFileDeclarations("p_test.go", []byte("package p\n\nfunc TestA(\n")); err == nil {
		t.Fatal("a file that does not parse was accepted")
	}
}

// STR-V0-071: the import paths a test file names are read sorted and once
// each, and a blank or dot import is an initialization effect.
func TestSTRV0071TestFileImports(t *testing.T) {
	got, err := TestFileDeclarations("p_test.go", []byte("package p\n\nimport (\n\t\"testing\"\n\tstr \"strings\"\n\t\"os\"\n)\n\nfunc TestA(t *testing.T) { _ = str.ToUpper(os.Args[0]) }\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Imports, []string{"os", "strings", "testing"}) || got.SideEffect != "" {
		t.Fatalf("imports %v side effect %q", got.Imports, got.SideEffect)
	}
	for src, want := range map[string]string{
		"package p\n\nimport _ \"example.test/m/a\"\n":               "imports example.test/m/a for its side effects",
		"package p\n\nimport . \"example.test/m/a\"\n":               "dot-imports example.test/m/a",
		"package p\n\nimport _ \"x/y\"\n\nfunc init() {}\n":          "declares init",
		"package p\n\nimport (\n\t_ \"x/y\"\n\t. \"x/z\"\n)\n":       "imports x/y for its side effects",
		"package p\n\nimport other \"x/y\"\n\nvar _ = other.Value\n": "",
	} {
		got, err := TestFileDeclarations("p_test.go", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if got.SideEffect != want {
			t.Fatalf("%q: side effect %q, want %q", src, got.SideEffect, want)
		}
	}
}
