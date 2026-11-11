// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"sort"
	"testing"
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
const locationImports = "import (\n\t\"fmt\"\n\t\"io/fs\"\n\t\"os\"\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"runtime\"\n\t\"testing\"\n)\n\n" +
	"var _, _, _, _, _ = fmt.Sprint, fs.ValidPath, exec.Command, filepath.Join, runtime.GOOS\nvar _ = os.Getpid\nvar _ testing.TB\n\n"

// locationSourceCases are the witnesses of V1-3999. Every one but the
// controls reads a repository path it builds at run time.
var locationSourceCases = []locationSourceCase{
	// Can miss a test: the repository root is rebuilt from the test file's
	// own name, with no `..` literal, and every file below it is read.
	{name: "callerroot", test: `func TestWalk(t *testing.T) {
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
	{name: "envroot", test: `func TestList(t *testing.T) {
	entries, _ := os.ReadDir(os.Getenv("DATA_ROOT"))
	for _, entry := range entries {
		os.ReadFile(filepath.Join(os.Getenv("DATA_ROOT"), entry.Name()))
	}
}`},
	// Can miss a test: the path is concatenated with + from a located root
	// and a name read from a listing.
	{name: "concat", test: `func TestConcat(t *testing.T) {
	root, _ := os.LookupEnv("SUITE_ROOT")
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		os.ReadFile(root + string(filepath.Separator) + entry.Name())
	}
}`},
	// Can miss a test: a non-test helper of the package returns the root,
	// and the test lists it. The helper is non-test code, so a dependent
	// whose tests call it reads the same root.
	{name: "helper", source: `func repositoryRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(file))
}`, test: `func TestHelper(t *testing.T) {
	os.ReadDir(repositoryRoot())
}`},
	// Can miss a test: the root is held in a struct field.
	{name: "field", test: `type suite struct{ root string }

func TestField(t *testing.T) {
	s := suite{root: os.Getenv("SUITE_ROOT")}
	os.ReadDir(s.root)
}`},
	// Can miss a test: the path is a command argument.
	{name: "execarg", test: `func TestArg(t *testing.T) {
	exec.Command("cat", os.Getenv("INPUT")).Run()
}`},
	// Can miss a test: the command runs in the rebuilt root and lists it.
	{name: "execdir", test: `func TestDir(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	command := exec.Command("ls", "-R")
	command.Dir = filepath.Dir(filepath.Dir(file))
	command.Run()
}`},
	// Can miss a test: the file of a frame from runtime.CallersFrames.
	{name: "frames", test: `func TestFrames(t *testing.T) {
	pcs := make([]uintptr, 4)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(1, pcs)])
	frame, _ := frames.Next()
	os.ReadDir(filepath.Dir(filepath.Dir(frame.File)))
}`},
	// Can miss a test: the file of runtime.FuncForPC(...).FileLine.
	{name: "fileline", test: `func TestFileLine(t *testing.T) {
	pc, _, _, _ := runtime.Caller(0)
	file, _ := runtime.FuncForPC(pc).FileLine(pc)
	os.ReadDir(filepath.Dir(filepath.Dir(file)))
}`},
	// Can miss a test: an entry parsed out of the whole environment.
	{name: "environ", test: `func TestEnviron(t *testing.T) {
	for _, entry := range os.Environ() {
		if len(entry) > 5 && entry[:5] == "ROOT=" {
			os.ReadDir(entry[5:])
		}
	}
}`},
	// Can miss a test: a path expanded from the environment.
	{name: "expandenv", test: `func TestExpand(t *testing.T) {
	os.ReadDir(os.ExpandEnv("$SUITE_ROOT/docs"))
}`},
	// Controls, bounded before and after: frame metadata that never becomes a
	// path (AFP-V0-810), an environment flag, and environment handed whole
	// to a child process.
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
