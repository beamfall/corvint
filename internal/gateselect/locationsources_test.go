// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"context"
	"go/parser"
	"go/token"
	"reflect"
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
	// Can miss a test (fix round 1, P1-a): a frame root handed to a child
	// process as a NAME=value entry, which the shell lists.
	{name: "childenv", held: true, test: `func TestChildEnv(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	c := exec.Command("sh", "-c", ` + "`ls -R \"$ROOT\"`" + `)
	c.Env = append(os.Environ(), "ROOT="+filepath.Dir(filepath.Dir(file)))
	c.Run()
}`},
	// Can miss a test (fix round 1, P1-b): an environment list stored in a
	// user struct's Env field and read back.
	{name: "envstruct", held: true, test: `type config struct{ Env []string }

func TestEnvStruct(t *testing.T) {
	c := config{Env: os.Environ()}
	for _, e := range c.Env {
		if len(e) > 5 {
			os.ReadDir(e[5:])
		}
	}
}`},
	// Can miss a test (fix round 1, P2): the working directory from
	// filepath.Abs, climbed to the repository root.
	{name: "absdir", held: true, test: `func TestAbs(t *testing.T) {
	wd, _ := filepath.Abs(".")
	os.ReadDir(filepath.Dir(filepath.Dir(wd)))
}`},
	// Can miss a test (fix round 1, P3): another file of the package declares
	// len, so len(os.Getenv(...)) here is that function, which reads.
	{name: "shadowedlen", held: true, source: `func len(s string) int {
	entries, _ := os.ReadDir(s)
	if entries == nil {
		return 0
	}
	return 1
}`, test: `func TestLen(t *testing.T) {
	_ = len(os.Getenv("ROOT"))
}`},
	// Can miss a test (fix round 1, P1-a): a raw environment value bound
	// for a child under another name; on a CI runner GITHUB_WORKSPACE is
	// the checkout.
	{name: "renamedentry", held: true, test: `func TestRenamed(t *testing.T) {
	c := exec.Command("sh", "-c", "ls -R $DATA")
	c.Env = []string{"DATA=" + os.Getenv("GITHUB_WORKSPACE")}
	c.Run()
}`},
	// Can miss a test (fix round 1, R1): a bounded join opened, whose file
	// name filepath.Dir climbs above the join.
	{name: "openname", held: true, test: `func TestOpenName(t *testing.T) {
	f, _ := os.Open(filepath.Join(os.Getenv("FIXTURES"), "docs"))
	os.ReadDir(filepath.Dir(filepath.Dir(f.Name())))
}`},
	// Controls, bounded before and after: frame metadata that never becomes a
	// path (AFP-V0-810), an environment flag, environment handed whole to a
	// child process (and one variable re-exported under its own name), roots
	// joined to plain literal components whose tokens rule (c) already reads
	// (AFP-V0-2203, AFP-V0-2240), and filepath.Abs of literals.
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
	version := exec.Command("go", "version")
	version.Env = []string{"PATH=" + os.Getenv("PATH")}
	version.Run()
}`},
	{name: "literaljoin", test: `func TestJoin(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	os.ReadFile(filepath.Join(filepath.Dir(file), "testdata", "case.json"))
	os.ReadFile(os.Getenv("FIXTURES") + "/repo/lib.js")
	os.ReadDir(filepath.Join(os.Getenv("FIXTURES"), "repo"))
}`},
	{name: "absliteral", test: `func TestAbsLiteral(t *testing.T) {
	data, _ := filepath.Abs("testdata")
	os.ReadDir(data)
	filepath.Abs("/etc/hosts")
}`},
	// Fix round 2 P1: can miss a test. The variable re-exported to the
	// child is reassigned to the parent of its own value, so the shell
	// lists every file above the named directory.
	{name: "selfassign", held: true, test: `func TestSelfAssign(t *testing.T) {
	v := os.Getenv("ROOT")
	v = filepath.Dir(filepath.Dir(v))
	c := exec.Command("sh", "-c", "ls -R \"$ROOT\"")
	c.Env = []string{"ROOT=" + v}
	_ = c.Run()
}`},
	// Fix round 2 P3: can miss a test. With Env nil, Cmd.Environ appends
	// PWD=<Dir>, so the command's Caller-rooted Dir is read back and listed.
	{name: "environpwd", held: true, test: `func TestEnvironPWD(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	c := exec.Command("go", "list", "-deps", "./cmd/x")
	c.Dir = filepath.Dir(filepath.Dir(file))
	for _, e := range c.Environ() {
		if len(e) > 4 && e[:4] == "PWD=" {
			os.ReadDir(e[4:])
		}
	}
}`},
	// The literal join walked is held since fix round 1: WalkDir hands its
	// callback full paths filepath.Dir can climb (AFP-V0-3662).
	{name: "literalwalk", held: true, test: `func TestWalkJoin(t *testing.T) {
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
	{"environment entry", `func F() { c := exec.Command("go", "env"); c.Env = []string{"PATH=" + os.Getenv("PATH")}; c.Run() }`, ""},
	{"looked-up entry through a local", `func F() { c := exec.Command("go", "env"); env := os.Environ(); if v, ok := os.LookupEnv("GOPATH"); ok { env = append(env, "GOPATH="+v) }; c.Env = env; c.Output() }`, ""},
	{"entry returned", `func F() []string { return []string{"PATH=" + os.Getenv("PATH")} }`, "hold"},
	{"computed entry", `func F(key string) { c := exec.Command("x"); if v, ok := os.LookupEnv(key); ok { c.Env = append(c.Env, key+"="+v) }; c.Run() }`, "hold"},
	{"computed key literal", `func F() []string { return append([]string{"-v"}, "--root"+"="+os.Getenv("ROOT")) }`, "hold"},
	{"entry in argv", `func F() { exec.Command("env", []string{"ROOT=" + os.Getenv("ROOT"), "sh", "-c", "ls -R $ROOT"}...).Run() }`, "hold"},
	{"entry appended to Args", `func F() { c := exec.Command("env"); c.Args = append(c.Args, "ROOT="+os.Getenv("ROOT"), "ls"); c.Run() }`, "hold"},
	{"entry renamed", `func F() { c := exec.Command("x"); c.Env = []string{"DATA=" + os.Getenv("GITHUB_WORKSPACE")}; c.Run() }`, "hold"},
	{"entry of a frame file", `func F() { _, file, _, _ := runtime.Caller(0); c := exec.Command("sh"); c.Env = append(os.Environ(), "ROOT="+filepath.Dir(filepath.Dir(file))); c.Run() }`, "hold"},
	{"entry of a transformed value", `func F() { c := exec.Command("x"); c.Env = []string{"ROOT=" + filepath.Clean(os.Getenv("ROOT"))}; c.Run() }`, "hold"},
	{"entry of an absolute value", `func F() { abs, _ := filepath.Abs(os.Getenv("ROOT")); c := exec.Command("x"); c.Env = []string{"ROOT=" + abs}; c.Run() }`, "hold"},
	{"entry to a parameter command", `func F(c *exec.Cmd) { c.Env = []string{"PATH=" + os.Getenv("PATH")} }`, "hold"},
	{"entry outside a list", `func F() { os.ReadFile("X=" + os.Getenv("X")) }`, "hold"},
	{"environment to Env field", `func F() { c := exec.Command("go", "env"); c.Env = append(os.Environ(), "A=1"); c.Run() }`, ""},
	{"environment to Env key", `func F() { c := &exec.Cmd{Path: "go", Env: os.Environ()}; c.Run() }`, ""},
	{"environment through local", `func F(ctx context.Context) { c := exec.CommandContext(ctx, "go"); env := slices.Clip(append(os.Environ(), "A=1")); env = append(env, "B=2"); c.Env = env; c.Run() }`, ""},
	{"environment to a parameter's Env", `func F(c *exec.Cmd) { c.Env = append(os.Environ(), "A=1") }`, "hold"},
	{"environment in a returned literal", `func F() *exec.Cmd { return &exec.Cmd{Env: os.Environ()} }`, "hold"},
	{"environment in a user struct", `type cfg struct{ Env []string }; func F() { c := cfg{Env: os.Environ()}; for _, e := range c.Env { os.ReadDir(e[5:]) } }`, "hold"},
	{"environment in a user struct read by its method", `type cfg struct{ Env []string }; func (c cfg) run() { for _, e := range c.Env { os.ReadDir(e[5:]) } }; func F() { c := cfg{Env: os.Environ()}; c.run() }`, "hold"},
	{"environment assigned to a user struct read by its method", `type cfg struct{ Env []string }; func (c *cfg) run() { for _, e := range c.Env { os.ReadDir(e[5:]) } }; func F() { c := &cfg{}; c.Env = os.Environ(); c.run() }`, "hold"},
	{"environment in a user struct field", `type cfg struct{ Env []string }; func F(c *cfg) { c.Env = os.Environ() }`, "hold"},
	{"environment read back from Env", `func F() { c := exec.Command("x"); c.Env = os.Environ(); for _, e := range c.Env { os.ReadDir(e[5:]) } }`, "hold"},
	{"environment read back from Environ", `func F() { c := exec.Command("x"); c.Env = os.Environ(); for _, e := range c.Environ() { os.ReadDir(e[5:]) } }`, "hold"},
	{"environment extended from Env", `func F() { c := exec.Command("x"); c.Env = os.Environ(); c.Env = append(c.Env, "A=1"); c.Run() }`, "hold"},
	{"environment command escapes", `func run(*exec.Cmd) {}; func F() { c := exec.Command("x"); c.Env = os.Environ(); run(c) }`, "hold"},
	{"environment command reassigned", `func F(other *exec.Cmd) { c := exec.Command("x"); c = other; c.Env = os.Environ() }`, "hold"},
	{"environment in a map keyed by a constant", `const Env = "env"; func F() map[string][]string { return map[string][]string{Env: os.Environ()} }`, "hold"},
	{"environment length", `func F() bool { return len(os.Environ()) == 1 }`, ""},
	{"environment to helper", `func run([]string) {}; func F() { run(os.Environ()) }`, "hold"},
	{"environment ranged", `func F() { for _, e := range os.Environ() { os.ReadFile(e) } }`, "hold"},
	{"stored in field", `type s struct{ root string }; func F(v *s) { v.root = os.Getenv("R") }`, "hold"},
	{"address taken", `func F() *string { v := os.Getenv("R"); return &v }`, "hold"},
	{"appended to", `func F() string { v := os.Getenv("R"); v += "/x"; return "" }`, "hold"},
	{"returned", `func F() string { return os.Getenv("R") }`, "hold"},
	{"closure read", `func F() { v := os.Getenv("R"); func() { os.ReadDir(v) }() }`, "hold"},
	{"package-level binding", `var root = os.Getenv("R")`, "hold"},
	{"bounded read", `func F() { os.ReadFile(filepath.Join(os.Getenv("R"), "docs", "guide.md")) }`, ""},
	{"bounded read via local", `func F() { root := os.Getenv("R") + "/repo"; os.ReadDir(root) }`, ""},
	{"bounded walk via local", `func F() { root := os.Getenv("R") + "/repo"; filepath.WalkDir(root, nil) }`, "hold"},
	{"bounded join opened", `func F() { f, _ := os.Open(filepath.Join(os.Getenv("R"), "docs")); os.ReadDir(filepath.Dir(filepath.Dir(f.Name()))) }`, "hold"},
	{"absolute of the working directory", `func F() { wd, _ := filepath.Abs("."); os.ReadDir(filepath.Dir(wd)) }`, "hold"},
	{"absolute of a parameter", `func F(p string) { abs, _ := filepath.Abs(p); os.ReadDir(abs) }`, "hold"},
	{"absolute of a plain literal read", `func F() { data, _ := filepath.Abs("testdata"); os.ReadDir(data) }`, ""},
	{"absolute of a plain literal climbed", `func F() { data, _ := filepath.Abs("testdata"); os.ReadDir(filepath.Dir(data)) }`, "hold"},
	{"absolute of an absolute literal", `func F() string { p, _ := filepath.Abs("/etc/hosts"); return p }`, ""},
	{"absolute compared", `func F(p string) bool { abs, _ := filepath.Abs(p); return abs == "/" }`, ""},
	{"absolute dot import", "func F() { wd, _ := Abs(\".\"); os.ReadDir(wd) }", "hold"},
	{"getcwd", `func F(buf []byte) { syscall.Getcwd(buf) }`, "hold"},
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
	{"command path written", `func F() { _, file, _, _ := runtime.Caller(0); command := exec.Command("go", "list", "-deps", "./cmd/x"); command.Path = "/bin/ls"; command.Dir = filepath.Dir(file); command.Run() }`, "hold"},
	{"command passed to a helper", `func run(*exec.Cmd) {}; func F() { _, file, _, _ := runtime.Caller(0); command := exec.Command("go", "list", "-deps", "./cmd/x"); command.Dir = filepath.Dir(file); run(command) }`, "hold"},
	{"command returned", `func F() *exec.Cmd { _, file, _, _ := runtime.Caller(0); command := exec.Command("go", "list", "-deps", "./cmd/x"); command.Dir = filepath.Dir(file); return command }`, "hold"},
	{"command stored", `type s struct{ c *exec.Cmd }; func F(v *s) { _, file, _, _ := runtime.Caller(0); command := exec.Command("go", "list", "-deps", "./cmd/x"); command.Dir = filepath.Dir(file); v.c = command }`, "hold"},
	{"command directory read back", `func F() { _, file, _, _ := runtime.Caller(0); command := exec.Command("go", "list", "-deps", "./cmd/x"); command.Dir = filepath.Dir(file); os.ReadDir(command.Dir) }`, "hold"},
	{"command directory and a read", `func F() { _, file, _, _ := runtime.Caller(0); root := filepath.Dir(file); command := exec.Command("go", "list"); command.Dir = root; os.ReadDir(root) }`, "hold"},
	// Fix round 2 P1: a re-exported entry is safe only for the raw value; a
	// reassignment of its variable, directly or through another variable,
	// to a transform of itself carries a location value into the entry.
	{"entry of a self-reassigned value", `func F() { v := os.Getenv("ROOT"); v = filepath.Dir(filepath.Dir(v)); c := exec.Command("sh", "-c", "ls -R \"$ROOT\""); c.Env = []string{"ROOT=" + v}; _ = c.Run() }`, "hold"},
	{"entry of a self-extended value", `func F() { v := os.Getenv("ROOT"); v = v + "/.."; c := exec.Command("sh", "-c", "ls -R \"$ROOT\""); c.Env = []string{"ROOT=" + v}; _ = c.Run() }`, "hold"},
	{"entry of a self-reassigned lookup", `func F() { v, _ := os.LookupEnv("ROOT"); v = filepath.Dir(v); c := exec.Command("sh", "-c", "ls -R \"$ROOT\""); c.Env = []string{"ROOT=" + v}; _ = c.Run() }`, "hold"},
	{"entry of a two-variable cycle", `func F() { v := os.Getenv("ROOT"); w := v; v = filepath.Dir(w); c := exec.Command("sh", "-c", "ls -R \"$ROOT\""); c.Env = []string{"ROOT=" + v}; _ = c.Run() }`, "hold"},
	{"entry of a raw value to a shell", `func F() { c := exec.Command("sh", "-c", "ls -R \"$ROOT\""); c.Env = []string{"ROOT=" + os.Getenv("ROOT")}; _ = c.Run() }`, ""},
	{"entry of a copied raw value", `func F() { v := os.Getenv("ROOT"); w := v; c := exec.Command("sh", "-c", "ls -R \"$ROOT\""); c.Env = []string{"ROOT=" + w}; _ = c.Run() }`, ""},
	// Fix round 2 P3: with Env nil, Cmd.Environ appends PWD=<Dir>, so a
	// command whose Environ is read gives its Dir back.
	{"command directory read back through Environ", `func F() { _, file, _, _ := runtime.Caller(0); c := exec.Command("go", "list", "-deps", "./cmd/x"); c.Dir = filepath.Dir(filepath.Dir(file)); for _, e := range c.Environ() { if dir, ok := strings.CutPrefix(e, "PWD="); ok { os.ReadDir(dir) } } }`, "hold"},
	{"aliased import", "func F() { os.ReadDir(system.Getenv(\"R\")) }", "hold"},
	{"dot import", "func F() { os.ReadDir(Getenv(\"R\")) }", "hold"},
	{"shadowed qualifier", `func F() { os := struct{ Getenv func(string) string }{}; _ = os.Getenv("R") + "" }`, ""},
	{"unparsed", `func F() { os.ReadDir(os.Getenv("R") }`, "hold"},
}

// TestLocationSourceUses_AFPV03661 runs the location-source pass over each
// locationUseCases body (AFP-V0-3660 to AFP-V0-3664).
func TestLocationSourceUses_AFPV03661(t *testing.T) {
	const imports = "import (\n\t\"context\"\n\t\"os\"\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"runtime\"\n\t\"slices\"\n\t\"strconv\"\n\t\"strings\"\n\t\"syscall\"\n\t\"time\"\n\tsystem \"os\"\n\t. \"os\"\n\t. \"path/filepath\"\n)\n\n"
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
		{"func len", "package p\n\nfunc len(string) int { return 0 }\n", builtinSource},
		{"var append", "package p\n\nvar x, append = 1, 2\n", builtinSource},
		{"grouped var len", "package p\n\nvar (\n\tn   = 1\n\tlen (int)\n)\n", builtinSource},
		{"type append", "package p\n\ntype append []int\n", builtinSource},
		{"len unparsed", "package p\n\nvar len = \n", builtinSource},
		{"len used", "package p\n\nvar n = len(\"abc\")\n\nfunc F(s []int) []int { len := 2; return append(s, len) }\n\ntype T struct{ len int }\n\nfunc (T) len() {}\n", ""},
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

// TestLocationSourceReadKinds_AFPV03660 holds a location source of a test
// file that declares `//corvint:read-kinds fixtures` (AFP-V0-9005) in the
// planner as in this index, which reads no declaration: the declaration
// removes lexical reasons and a source is none.
func TestLocationSourceReadKinds_AFPV03660(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"go.mod":          "module example.com/fixture\n\ngo 1.22\n",
		"kinds/x.go":      "package kinds\n",
		"kinds/x_test.go": "//corvint:read-kinds fixtures\n\npackage kinds\n\nimport \"os\"\n\nfunc F() { os.ReadDir(os.Getenv(\"R\")) }\n",
		"plain/x.go":      "package plain\n",
		"plain/x_test.go": "//corvint:read-kinds fixtures\n\npackage plain\n\nimport \"os\"\n\nfunc F() { os.ReadDir(\"../..\") }\n",
	})
	index, err := indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	if got := sortedKeys(index.unresolved()); !reflect.DeepEqual(got, []string{"kinds", "plain"}) {
		t.Errorf("gateselect rule (d) = %v, want [kinds plain]", got)
	}
	// The planner honours the declaration for the literal climb alone.
	if got := plannerUnbounded(t, root); !reflect.DeepEqual(got, []string{"kinds"}) {
		t.Errorf("planner rule (d) = %v, want [kinds]", got)
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
