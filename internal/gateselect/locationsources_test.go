// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"context"
	"go/parser"
	"go/token"
	"sort"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// locationSourceCase is one fixture package of V1-3999: a test file (and, for
// a helper, a non-test file) that reads a path no string literal names,
// because the path is built at run time from a source that discloses a
// location. held says whether rule (d) of AFP-V0-012 holds the package
// unbounded; a case that is not held can miss a test, because a change to a
// file it reads selects nothing.
type locationSourceCase struct {
	name, test, source string
	held               bool
}

// locationImports is the import block every fixture file shares; blank uses
// keep each file compiling whichever imports its case needs.
const locationImports = "import (\n\t\"fmt\"\n\t\"io/fs\"\n\t\"os\"\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"runtime\"\n\t\"syscall\"\n\t\"testing\"\n)\n\n" +
	"var _, _, _, _, _ = fmt.Sprint, fs.ValidPath, exec.Command, filepath.Join, runtime.GOOS\nvar _, _ = os.Getpid, syscall.Getpid\nvar _ testing.TB\n\n"

// locationSourceCases are the witnesses of V1-3999. Every one but the
// controls reads a repository path it builds at run time.
var locationSourceCases = []locationSourceCase{
	// Can miss a test: the repository root is rebuilt from the test file's
	// own name, with no `..` literal, and every file below it is read.
	{name: "callerroot", held: true, test: `func TestWalk(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(file))
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			os.ReadFile(p)
		}
		return nil
	})
}`},
	// Can miss a test: the directory is named by the environment.
	{name: "envroot", held: true, test: `func TestList(t *testing.T) {
	entries, _ := os.ReadDir(os.Getenv("DATA_ROOT"))
	for _, entry := range entries {
		os.ReadFile(filepath.Join(os.Getenv("DATA_ROOT"), entry.Name()))
	}
}`},
	// Can miss a test: the path is concatenated with + from a located root
	// and a name read from a listing.
	{name: "concat", held: true, test: `func TestConcat(t *testing.T) {
	root, _ := os.LookupEnv("SUITE_ROOT")
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		os.ReadFile(root + string(filepath.Separator) + entry.Name())
	}
}`},
	// Can miss a test: a non-test helper of the package returns the root,
	// and the test lists it. The helper is non-test code, so a dependent
	// whose tests call it reads the same root.
	{name: "helper", held: true, source: `func repositoryRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(file))
}`, test: `func TestHelper(t *testing.T) {
	os.ReadDir(repositoryRoot())
}`},
	// Can miss a test: the root is held in a struct field.
	{name: "field", held: true, test: `type suite struct{ root string }

func TestField(t *testing.T) {
	s := suite{root: os.Getenv("SUITE_ROOT")}
	os.ReadDir(s.root)
}`},
	// Can miss a test: the path is a command argument.
	{name: "execarg", held: true, test: `func TestArg(t *testing.T) {
	exec.Command("cat", os.Getenv("INPUT")).Run()
}`},
	// Can miss a test: the command runs in the rebuilt root and lists it.
	{name: "execdir", held: true, test: `func TestDir(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	command := exec.Command("ls", "-R")
	command.Dir = filepath.Dir(filepath.Dir(file))
	command.Run()
}`},
	// Can miss a test: the file of a frame from runtime.CallersFrames.
	{name: "frames", held: true, test: `func TestFrames(t *testing.T) {
	pcs := make([]uintptr, 4)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(1, pcs)])
	frame, _ := frames.Next()
	os.ReadDir(filepath.Dir(filepath.Dir(frame.File)))
}`},
	// Can miss a test: the file of runtime.FuncForPC(...).FileLine.
	{name: "fileline", held: true, test: `func TestFileLine(t *testing.T) {
	pc, _, _, _ := runtime.Caller(0)
	file, _ := runtime.FuncForPC(pc).FileLine(pc)
	os.ReadDir(filepath.Dir(filepath.Dir(file)))
}`},
	// Can miss a test: an entry parsed out of the whole environment.
	{name: "environ", held: true, test: `func TestEnviron(t *testing.T) {
	for _, entry := range os.Environ() {
		if len(entry) > 5 && entry[:5] == "ROOT=" {
			os.ReadDir(entry[5:])
		}
	}
}`},
	// Can miss a test: a path expanded from the environment.
	{name: "expandenv", held: true, test: `func TestExpand(t *testing.T) {
	os.ReadDir(os.ExpandEnv("$SUITE_ROOT/docs"))
}`},
	// Can miss a test: a frame file bound to a package-level variable, which
	// any file of the package may read.
	{name: "packagelevel", held: true, test: `var _, file, _, _ = runtime.Caller(0)

func TestPackageLevel(t *testing.T) {}`},
	// Can miss a test: an environment root joined to computed text, which
	// need not start a new component.
	{name: "computedjoin", held: true, test: `func TestComputed(t *testing.T) {
	os.ReadFile(os.Getenv("FIXTURES") + fmt.Sprint("notes"))
}`},
	// Can miss a test: a literal join widened again by filepath.Dir.
	{name: "widened", held: true, test: `func TestWidened(t *testing.T) {
	os.ReadDir(filepath.Dir(filepath.Join(os.Getenv("FIXTURES"), "docs")))
}`},
	// Can miss a test: a join that climbs with a literal "..".
	{name: "climbjoin", held: true, test: `func TestClimb(t *testing.T) {
	os.ReadDir(os.Getenv("FIXTURES") + "/..")
}`},
	// Can miss a test: the root of syscall.Getwd, the working directory.
	{name: "syscallgetwd", held: true, test: `func TestGetwd(t *testing.T) {
	root, _ := syscall.Getwd()
	_ = root
}`},
	// Controls, bounded before and after: frame metadata that never becomes a
	// path (AFP-V0-810), an environment flag, environment handed whole to a
	// child process, and roots joined to plain literal components whose
	// tokens rule (c) already reads (AFP-V0-2203, AFP-V0-2240).
	{name: "framelog", test: `func TestLog(t *testing.T) {
	_, file, line, _ := runtime.Caller(0)
	_ = file
	_, _, line, _ = runtime.Caller(1)
	t.Log(line)
}`},
	{name: "envflag", test: `func TestFlag(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("local only")
	}
}`},
	{name: "envchild", test: `func TestChild(t *testing.T) {
	command := exec.Command("go", "env")
	command.Env = append(os.Environ(), "GOFLAGS=")
	command.Run()
	command = exec.Command("go", "version")
	command.Env = []string{"PATH=" + os.Getenv("PATH")}
	command.Run()
}`},
	{name: "literaljoin", test: `func TestJoin(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	os.ReadFile(filepath.Join(filepath.Dir(file), "testdata", "case.json"))
	os.ReadFile(os.Getenv("FIXTURES") + "/repo/lib.js")
	filepath.WalkDir(filepath.Join(os.Getenv("FIXTURES"), "repo"), func(string, fs.DirEntry, error) error { return nil })
}`},
}

// TestLocationSourceReads_AFPV03660 indexes one fixture package per V1-3999
// case and holds the rule (d) set of this index, cold and warm, equal to the
// planner's and to the cases marked held. A dependent of the helper case is
// held with it when the helper is.
func TestLocationSourceReads_AFPV03660(t *testing.T) {
	files := map[string]string{
		"go.mod":              "module example.com/fixture\n\ngo 1.22\n",
		"other/o.go":          "package other\n",
		"other/o_test.go":     "package other\n",
		"dependent/d.go":      "package dependent\n\nimport _ \"example.com/fixture/helper\"\n",
		"dependent/d_test.go": "package dependent\n",
	}
	want := []string{}
	for _, c := range locationSourceCases {
		files[c.name+"/x_test.go"] = "package " + c.name + "\n\n" + locationImports + c.test + "\n"
		files[c.name+"/x.go"] = "package " + c.name + "\n"
		if c.source != "" {
			files[c.name+"/x.go"] = "package " + c.name + "\n\n" + locationImports + c.source + "\n"
		}
		if c.held {
			want = append(want, c.name)
			if c.name == "helper" {
				want = append(want, "dependent")
			}
		}
	}
	sort.Strings(want)
	agree(t, writeFixture(t, files), want)
}

// locationUseCases pin each use AFP-V0-3661 lists as safe, each one it does
// not, and the AFP-V0-3664 command directory. want is "" for a file whose
// sources are all safe, "hold" for one that holds the package and
// "contingent" for one whose only unproven use is a command directory.
var locationUseCases = []struct{ name, body, want string }{
	{"discarded call", `func F() { os.Getenv("X") }`, ""},
	{"blank assignment", `func F() { _ = os.Getenv("X") }`, ""},
	{"discarded function value", `func F() { _ = os.Getenv }`, ""},
	{"function value", `func F() func(string) string { return os.Getenv }`, "hold"},
	{"function value bound", `func F() { get := os.Getenv; os.ReadDir(get("X")) }`, "hold"},
	{"comparison", `func F() bool { return os.Getenv("X") == "1" || os.Getenv("Y") != "" }`, ""},
	{"local compared", `func F() bool { v := os.Getenv("X"); if v == "" { return false }; return v < "m" }`, ""},
	{"switch tag", `func F() int { switch os.Getenv("MODE") { case "a": return 1 }; return 0 }`, ""},
	{"length", `func F() int { return len(os.Getenv("X")) }`, ""},
	{"parsed", `func F() { strconv.Atoi(os.Getenv("N")); strconv.ParseBool(os.Getenv("B")); time.ParseDuration(os.Getenv("D")) }`, ""},
	{"lookup presence", `func F() bool { _, ok := os.LookupEnv("X"); return ok }`, ""},
	{"lookup value read", `func F() { if v, ok := os.LookupEnv("X"); ok { os.ReadFile(v) } }`, "hold"},
	{"environment entry", `func F() []string { return []string{"PATH=" + os.Getenv("PATH")} }`, ""},
	{"computed entry", `func F(env []string, key string) []string { if v, ok := os.LookupEnv(key); ok { env = append(env, key+"="+v) }; return env }`, ""},
	{"entry outside a list", `func F() { os.ReadFile("X=" + os.Getenv("X")) }`, "hold"},
	{"environment to Env field", `func F(c *exec.Cmd) { c.Env = append(os.Environ(), "A=1") }`, ""},
	{"environment to Env key", `func F() *exec.Cmd { return &exec.Cmd{Env: os.Environ()} }`, ""},
	{"environment through local", `func F(c *exec.Cmd) { env := slices.Clip(append(os.Environ(), "A=1")); env = append(env, "B=2"); c.Env = env }`, ""},
	{"environment to helper", `func run([]string) {}; func F() { run(os.Environ()) }`, "hold"},
	{"environment ranged", `func F() { for _, e := range os.Environ() { os.ReadFile(e) } }`, "hold"},
	{"stored in field", `type s struct{ root string }; func F(v *s) { v.root = os.Getenv("R") }`, "hold"},
	{"address taken", `func F() *string { v := os.Getenv("R"); return &v }`, "hold"},
	{"appended to", `func F() string { v := os.Getenv("R"); v += "/x"; return "" }`, "hold"},
	{"returned", `func F() string { return os.Getenv("R") }`, "hold"},
	{"closure read", `func F() { v := os.Getenv("R"); func() { os.ReadDir(v) }() }`, "hold"},
	{"package-level binding", `var root = os.Getenv("R")`, "hold"},
	{"bounded read", `func F() { os.ReadFile(filepath.Join(os.Getenv("R"), "docs", "guide.md")) }`, ""},
	{"bounded walk via local", `func F() { root := os.Getenv("R") + "/repo"; filepath.WalkDir(root, nil) }`, ""},
	{"bounded join with dot", `func F() { os.ReadFile(filepath.Join(os.Getenv("R"), ".", "guide.md")) }`, "hold"},
	{"bounded join with glob", `func F() { os.ReadFile(os.Getenv("R") + "/*.md") }`, "hold"},
	{"bounded join with backslash", `func F() { os.ReadFile(os.Getenv("R") + "/..\\x") }`, "hold"},
	{"bounded join without separator", `func F() { os.ReadFile(os.Getenv("R") + "notes") }`, "hold"},
	{"bounded join of spaces only", `func F() { os.ReadFile(os.Getenv("R") + "/a b") }`, "hold"},
	{"bounded join returned", `func F() string { return filepath.Join(os.Getenv("R"), "docs") }`, "hold"},
	{"bounded join to command", `func F() { exec.Command("cat", filepath.Join(os.Getenv("R"), "docs")).Run() }`, "hold"},
	{"bounded join globbed", `func F() { filepath.Glob(filepath.Join(os.Getenv("R"), "docs")) }`, "hold"},
	{"caller discarded file", `func F() int { _, _, line, _ := runtime.Caller(1); return line }`, ""},
	{"caller file logged by blank", `func F() { _, file, _, _ := runtime.Caller(0); _ = file }`, ""},
	{"caller file returned", `func F() string { _, file, _, _ := runtime.Caller(0); return file }`, "hold"},
	{"caller passed whole", `func use(...any) {}; func F() { use(runtime.Caller(0)) }`, "hold"},
	{"func name only", `func F(pc uintptr) string { return runtime.FuncForPC(pc).Name() }`, ""},
	{"func file line", `func F(pc uintptr) { file, _ := runtime.FuncForPC(pc).FileLine(pc); os.ReadFile(file) }`, "hold"},
	{"func line only", `func F(pc uintptr) int { _, line := runtime.FuncForPC(pc).FileLine(pc); return line }`, ""},
	{"frame function only", `func F(pcs []uintptr) string { frames := runtime.CallersFrames(pcs); frame, _ := frames.Next(); return frame.Function }`, ""},
	{"frame file read", `func F(pcs []uintptr) { frames := runtime.CallersFrames(pcs); for { frame, more := frames.Next(); os.ReadFile(frame.File); if !more { break } } }`, "hold"},
	{"frame passed whole", `func use(any) {}; func F(pcs []uintptr) { frame, _ := runtime.CallersFrames(pcs).Next(); use(frame) }`, "hold"},
	{"working directory", `func F() { syscall.Getwd() }`, "hold"},
	{"expanded", `func F() { os.ReadFile(os.ExpandEnv("$R/x")) }`, "hold"},
	{"command directory", `func F() { _, file, _, _ := runtime.Caller(0); command := exec.Command("go", "list", "-deps", "./cmd/x"); command.Dir = filepath.Dir(filepath.Dir(file)); command.Output() }`, "contingent"},
	{"command reassigned", `func F() { _, file, _, _ := runtime.Caller(0); command := exec.Command("go", "list"); command = exec.Command("ls", "-R"); command.Dir = filepath.Dir(file); command.Run() }`, "hold"},
	{"command args named", `func F() { _, file, _, _ := runtime.Caller(0); command := exec.Command("go", "list", "./..."); command.Args[2] = "./x"; command.Dir = filepath.Dir(file); command.Run() }`, "hold"},
	{"command literal", `func F() { _, file, _, _ := runtime.Caller(0); command := &exec.Cmd{Path: "ls", Dir: filepath.Dir(file)}; command.Run() }`, "hold"},
	{"command directory and a read", `func F() { _, file, _, _ := runtime.Caller(0); root := filepath.Dir(file); command := exec.Command("go", "list"); command.Dir = root; os.ReadDir(root) }`, "hold"},
	{"aliased import", "func F() { os.ReadDir(system.Getenv(\"R\")) }", "hold"},
	{"dot import", "func F() { os.ReadDir(Getenv(\"R\")) }", "hold"},
	{"shadowed qualifier", `func F() { os := struct{ Getenv func(string) string }{}; _ = os.Getenv("R") + "" }`, ""},
	{"unparsed", `func F() { os.ReadDir(os.Getenv("R") }`, "hold"},
}

// TestLocationSourceUses_AFPV03661 runs the location-source pass over each
// locationUseCases body (AFP-V0-3660 to AFP-V0-3664).
func TestLocationSourceUses_AFPV03661(t *testing.T) {
	const imports = "import (\n\t\"os\"\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"runtime\"\n\t\"slices\"\n\t\"strconv\"\n\t\"syscall\"\n\t\"time\"\n\tsystem \"os\"\n\t. \"os\"\n)\n\n"
	for _, c := range locationUseCases {
		t.Run(c.name, func(t *testing.T) {
			body := []byte("package p\n\n" + imports + c.body + "\n")
			file, err := parser.ParseFile(token.NewFileSet(), "", body, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			every, dotted := qualifiedImports(file.Imports, func(string) bool { return true })
			got := ""
			for _, uses := range locationSourceUses(body, every, dotted) {
				switch {
				case locationHolds(uses):
					got = "hold"
				case got == "":
					got = "contingent"
				}
			}
			if got != c.want {
				t.Fatalf("got %q, want %q: %v", got, c.want, locationSourceUses(body, every, dotted))
			}
		})
	}
}

// TestLocationSourceUnseenCalls_AFPV03660 holds a file that can call a
// location source without naming it: a //go:linkname directive or cgo.
func TestLocationSourceUnseenCalls_AFPV03660(t *testing.T) {
	for _, c := range []struct{ name, body, want string }{
		{"linkname", "package p\n\nimport _ \"unsafe\"\n\n//go:linkname env syscall.Getenv\nfunc env(string) (string, bool)\n", linknameSource},
		{"cgo", "package p\n\n// #include <stdlib.h>\nimport \"C\"\n\nfunc F() { C.getenv(nil) }\n", cgoSource},
		{"neither", "package p\n\nimport \"os\"\n\nfunc F() { os.Getpid() }\n", ""},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "", c.body, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		every, dotted := qualifiedImports(file.Imports, func(string) bool { return true })
		got := locationSources([]byte(c.body), every, dotted)
		if c.want == "" && len(got) != 0 || c.want != "" && (len(got) != 1 || got[0] != c.want) {
			t.Errorf("%s: sources %v, want %q", c.name, got, c.want)
		}
	}
}

// TestLocationSourceReasonsFollowLiterals_AFPV03663 keeps a package's
// literal or locator reason ahead of its location-source reason, in this
// index and the planner, even when the source's file sorts first, and names
// the source when it is the only one.
func TestLocationSourceReasonsFollowLiterals_AFPV03663(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"go.mod":           "module example.com/fixture\n\ngo 1.22\n",
		"both/x.go":        "package both\n",
		"both/x_test.go":   "package both\n\nimport \"os\"\n\nfunc F() { os.ReadDir(os.Getenv(\"R\")); os.ReadDir(\"../..\") }\n",
		"source/x.go":      "package source\n",
		"across/a.go":      "package across\n\nimport \"os\"\n\nfunc A() { os.ReadDir(os.Getenv(\"R\")) }\n",
		"across/b.go":      "package across\n\nimport \"os\"\n\nfunc B() string { root, _ := os.Getwd(); return root }\n",
		"across/x_test.go": "package across\n",
		"source/x_test.go": "package source\n\nimport \"os\"\n\nfunc F() { os.ReadDir(os.Getenv(\"R\")) }\n",
	})
	index := agree(t, root, []string{"across", "both", "source"})
	want := map[string]string{
		"across": "across/b.go calls os.Getwd",
		"both":   `both/x_test.go carries the literal "../..", which reaches the repository root`,
		"source": "source/x_test.go calls os.Getenv, whose value may name a repository path",
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	for directory, reason := range want {
		if got := index.unresolved()[directory]; got != reason {
			t.Errorf("%s: reason %q, want %q", directory, got, reason)
		}
		if _, got := graph.UnboundedChain("go:" + fixtureModule + "/" + directory); got != reason {
			t.Errorf("%s: planner reason %q, want %q", directory, got, reason)
		}
	}
}

// TestLinknameInsideALiteralIsNoDirective_AFPV03660 keeps a //go:linkname
// line inside a raw string, a fixture's text, from holding its package.
func TestLinknameInsideALiteralIsNoDirective_AFPV03660(t *testing.T) {
	body := "package p\n\nvar fixture = `package q\n//go:linkname launch os/exec.Command\n`\n"
	if got := locationSources([]byte(body), map[string]string{}, map[string]bool{}); len(got) != 0 {
		t.Fatalf("sources %v, want none", got)
	}
}
