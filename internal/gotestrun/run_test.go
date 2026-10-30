package gotestrun

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// realShape is go test -json output for one module, in the exact event shape
// test2json writes: package-qualified import paths, subtests, output lines and
// package-level events without a Test member.
const realShape = `{"Time":"2026-10-27T10:00:00.000000Z","Action":"start","Package":"example.test/app/calc"}
{"Time":"2026-10-27T10:00:00.100000Z","Action":"run","Package":"example.test/app/calc","Test":"TestAdd"}
{"Time":"2026-10-27T10:00:00.100100Z","Action":"output","Package":"example.test/app/calc","Test":"TestAdd","Output":"=== RUN   TestAdd\n"}
{"Time":"2026-10-27T10:00:00.100200Z","Action":"pass","Package":"example.test/app/calc","Test":"TestAdd","Elapsed":0}
{"Time":"2026-10-27T10:00:00.100300Z","Action":"run","Package":"example.test/app/calc","Test":"TestTable"}
{"Time":"2026-10-27T10:00:00.100400Z","Action":"run","Package":"example.test/app/calc","Test":"TestTable/negative"}
{"Time":"2026-10-27T10:00:00.100500Z","Action":"fail","Package":"example.test/app/calc","Test":"TestTable/negative","Elapsed":0}
{"Time":"2026-10-27T10:00:00.100600Z","Action":"fail","Package":"example.test/app/calc","Test":"TestTable","Elapsed":0}
{"Time":"2026-10-27T10:00:00.100700Z","Action":"output","Package":"example.test/app/calc","Output":"FAIL\n"}
{"Time":"2026-10-27T10:00:00.100800Z","Action":"fail","Package":"example.test/app/calc","Elapsed":0.01}
{"Time":"2026-10-27T10:00:00.200000Z","Action":"skip","Package":"example.test/app","Test":"TestRoot","Elapsed":0}
{"Time":"2026-10-27T10:00:00.300000Z","Action":"pass","Package":"example.test/other/pkg","Test":"TestElsewhere","Elapsed":0}
`

func TestParseEventsKeepsTerminalActionsOfRealShape(t *testing.T) {
	tests, malformed := parseEvents([]byte(realShape))
	if malformed != "" {
		t.Fatal(malformed)
	}
	got := []string{}
	for _, test := range tests {
		got = append(got, test.Package+" "+test.Name+" "+test.Action)
	}
	want := "example.test/app TestRoot skip|example.test/app/calc TestAdd pass|example.test/app/calc TestTable fail|example.test/app/calc TestTable/negative fail|example.test/other/pkg TestElsewhere pass"
	if strings.Join(got, "|") != want {
		t.Fatalf("tests = %q", got)
	}
	if _, malformed := parseEvents([]byte("ok  \texample.test/app\t0.01s\n")); malformed == "" {
		t.Fatal("plain go test output was read as test2json")
	}
}

func TestLocatorMapsImportPathsThroughTheModulePath(t *testing.T) {
	root := t.TempDir()
	write(t, root, "svc/go.mod", "module example.test/app\n")
	write(t, root, "svc/root_test.go", "package app\n\nimport \"testing\"\n\nfunc TestRoot(t *testing.T) {}\n")
	write(t, root, "svc/calc/add_test.go", "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {}\nfunc TestTable(t *testing.T) {}\n")
	write(t, root, "svc/calc/dup_a_test.go", "package calc\n\nfunc TestDup() {}\n")
	write(t, root, "svc/calc/dup_b_test.go", "package calc\n\nfunc TestDup() {}\n")
	locate := newLocator(root, "svc", "example.test/app", "sha1")
	data, _ := os.ReadFile(filepath.Join(root, "svc/calc/add_test.go"))
	for _, tc := range []struct{ pkg, name, file, unbound string }{
		{"example.test/app", "TestRoot", "svc/root_test.go", ""},
		{"example.test/app/calc", "TestTable/negative", "svc/calc/add_test.go", ""},
		{"example.test/app/calc", "TestMissing", "", "no _test.go file in svc/calc declares TestMissing"},
		{"example.test/app/calc", "TestDup", "", "more than one _test.go file in svc/calc declares TestDup"},
		{"example.test/other/pkg", "TestElsewhere", "", "the package is outside module example.test/app"},
		{"example.test/application", "TestX", "", "the package is outside module example.test/app"},
	} {
		file, blob, unbound := locate.find(tc.pkg, tc.name)
		if file != tc.file || unbound != tc.unbound {
			t.Fatalf("%s %s = %q %q", tc.pkg, tc.name, file, unbound)
		}
		if file == "svc/calc/add_test.go" && blob != BlobID("sha1", data) {
			t.Fatalf("blob = %s", blob)
		}
	}
}

func TestDecodeRefusesOpenOrUnanchoredDocuments(t *testing.T) {
	valid := `{"profile":"corvint-go-test-run/0","directory":".","module":"m","moduleDirectory":".","objectFormat":"sha1","tests":[{"package":"m","name":"TestA","action":"pass","file":"a_test.go","blob":"` + strings.Repeat("a", 40) + `"}]}`
	if _, err := Decode([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for name, document := range map[string]string{
		"unknown member": strings.Replace(valid, `"module":"m"`, `"module":"m","extra":1`, 1),
		"escaping file":  strings.Replace(valid, `"a_test.go"`, `"../a_test.go"`, 1),
		"short blob":     strings.Replace(valid, strings.Repeat("a", 40), "abc", 1),
		"no anchor":      strings.Replace(valid, `"file":"a_test.go",`, ``, 1),
		"other profile":  strings.Replace(valid, "corvint-go-test-run/0", "corvint-go-live-session-event/0", 1),
	} {
		if _, err := Decode([]byte(document)); err == nil {
			t.Fatalf("%s: decoded", name)
		}
	}
}

// TestRunRecordsARealGoTestRun runs the real go test -json on a committed
// module and checks identity, module mapping, anchors and outcomes.
func TestRunRecordsARealGoTestRun(t *testing.T) {
	for _, tool := range []string{"go", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " is not available")
		}
	}
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	write(t, root, "mod/go.mod", "module example.test/run\n\ngo 1.21\n")
	write(t, root, "mod/lib/lib_test.go", "package lib\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {\n\tt.Run(\"sub\", func(t *testing.T) {})\n}\n\nfunc TestFail(t *testing.T) { t.Fail() }\n")
	git(t, root, "init", "-q")
	git(t, root, "add", ".")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.test", "commit", "-q", "-m", "init")
	document, err := Run(context.Background(), Config{Dir: filepath.Join(root, "mod")})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(document)
	if document.Infrastructure != nil || document.ExitStatus == nil || *document.ExitStatus == 0 {
		t.Fatalf("document = %s", encoded)
	}
	if document.Module != "example.test/run" || document.ModuleDirectory != "mod" || document.Directory != "mod" {
		t.Fatalf("module = %s", encoded)
	}
	before, after := document.RepositoryBefore, document.RepositoryAfter
	if before == nil || after == nil || before.Revision != after.Revision || before.Tree == "" || before.DirtyState != "clean" {
		t.Fatalf("identity = %s", encoded)
	}
	data, _ := os.ReadFile(filepath.Join(root, "mod/lib/lib_test.go"))
	got := []string{}
	for _, test := range document.Tests {
		if test.File != "mod/lib/lib_test.go" || test.Blob != BlobID(document.ObjectFormat, data) {
			t.Fatalf("test = %+v", test)
		}
		got = append(got, test.Name+" "+test.Action)
	}
	if strings.Join(got, "|") != "TestFail fail|TestPass pass|TestPass/sub pass" {
		t.Fatalf("tests = %q", got)
	}
	if _, err := Decode(encoded); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, root, name, content string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, output)
	}
}
