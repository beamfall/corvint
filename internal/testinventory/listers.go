// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"bufio"
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
	"github.com/Beamfall/corvint/internal/testid"
)

// The bounds one lister works within.
const (
	// maxSourceBytes bounds one test file a static lister parses; a longer
	// one is listed at file level.
	maxSourceBytes = 4 << 20
	// maxListerOutput bounds what an executing lister may print.
	maxListerOutput = 64 << 20
	// listerTimeout bounds one executing lister.
	listerTimeout = 10 * time.Minute
)

// Exec runs one executing lister: argv from dir, returning its bounded
// stdout. The error of a failed run carries the tail of its stderr.
type Exec func(ctx context.Context, dir string, argv []string) ([]byte, error)

// listInput is what a lister reads: the repository root (absolute), the
// tree it lists from, its files, every declared instance and the commit.
type listInput struct {
	Root     string
	Tree     Tree
	Files    map[string]string
	Declared []Declared
	Commit   string
	Exec     Exec
}

// listing is one lister's result before it becomes a record.
type listing struct {
	named   []testid.Named
	sources []string
	command []string
	receipt *Receipt
}

// add records one named test and the file it lives in.
func (l *listing) add(id testid.ID, reason, file string) {
	l.named = append(l.named, testid.Named{ID: id, Reason: reason})
	l.sources = append(l.sources, file)
}

// ListInstance lists one declared instance into its record (TIV-V1-004,
// TIV-V1-005): the runner's lister names its tests by canonical ID, and the
// record binds them to the commit, the config and lockfile blobs, the
// lister identity and every candidate or listed file's blob.
func listInstance(ctx context.Context, input listInput, instance Declared) (Instance, Lister, error) {
	identity, ok := CurrentLister(instance.Runner)
	if !ok {
		return Instance{}, Lister{}, fmt.Errorf("no lister for runner %q", instance.Runner)
	}
	lister := Lister{Name: identity.Name, Version: identity.Version, Kind: identity.Kind}
	candidates := Candidates(input.Files, instance, input.Declared)
	var result listing
	var err error
	switch instance.Runner {
	case testid.Go:
		result, err = listGo(input, candidates)
	case testid.RSpec:
		result, err = listFiles(testid.RSpec, candidates)
	case testid.Minitest:
		result, err = listMinitest(input, candidates)
	case testid.Vitest:
		result, err = listVitest(ctx, input, instance)
	case testid.Jest:
		result, err = listJest(ctx, input, instance)
	case testid.Playwright:
		result, err = listPlaywright(ctx, input, instance)
	case testid.Pytest:
		result, err = listPytest(ctx, input, instance)
	}
	lister.Command, lister.Receipt = result.command, result.receipt
	if lister.Kind == KindExecuting && lister.Command == nil {
		lister.Command = []string{identity.Name}
	}
	if err != nil {
		return Instance{}, lister, err
	}
	record := Instance{
		Key: instance.Key, Runner: instance.Runner, Root: instance.Root, Config: instance.Config,
		ConfigDigest: input.Files[instance.Config], Lockfile: instance.Lockfile, Lister: lister,
		Commit: input.Commit, Files: []File{}, Tests: []Test{},
	}
	if instance.Lockfile != "" {
		record.LockfileDigest = input.Files[instance.Lockfile]
	}
	seen := map[string]bool{}
	for _, name := range append(append([]string{}, candidates...), result.sources...) {
		if seen[name] {
			continue
		}
		seen[name] = true
		blob, ok := input.Files[name]
		if !ok || !under(instance.Root, name) {
			return Instance{}, lister, fmt.Errorf("the lister names %s, which is not a listed file under %s", name, instance.Root)
		}
		record.Files = append(record.Files, File{Path: name, Blob: blob})
	}
	// testid.List keeps the order of its input, so entry i is named[i]
	// and lives in sources[i].
	for index, entry := range testid.List(result.named) {
		file := result.sources[index]
		if !seen[file] {
			return Instance{}, lister, fmt.Errorf("test %s lives in %s, which is not a listed file under %s", entry.ID, file, instance.Root)
		}
		record.Tests = append(record.Tests, Test{ID: entry.ID, File: file, Duplicate: entry.Duplicate, Reason: entry.Reason})
	}
	return record, lister, nil
}

// listGo is the static Go lister: every top-level Test, Fuzz and runnable
// Example function of each candidate _test.go file, parsed with go/ast and
// named in its package directory (CTI-V1-003). Build constraints are not
// evaluated, so the listing is the same on every host. A file that does
// not parse is named by its package-level ID with test-id-unmapped.
func listGo(input listInput, candidates []string) (listing, error) {
	result := listing{}
	for _, name := range candidates {
		pkg, err := testid.New(testid.Go, path.Dir(name))
		if err != nil {
			return result, err
		}
		names, ok := goTestNames(input.Tree, name)
		if !ok {
			result.add(pkg, testid.ReasonUnmapped, name)
			continue
		}
		for _, test := range names {
			result.add(pkg.Child(test), "", name)
		}
	}
	return result, nil
}

// goTestNames is the top-level test function names of one Go test file in
// declaration order, false when it cannot be read or parsed.
func goTestNames(tree Tree, name string) ([]string, bool) {
	source, err := tree.Read(name, maxSourceBytes)
	if err != nil {
		return nil, false
	}
	file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, false
	}
	runnable := map[string]bool{}
	for _, example := range doc.Examples(file) {
		if example.Output != "" || example.EmptyOutput {
			runnable["Example"+example.Name] = true
		}
	}
	var names []string
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok || function.Recv != nil {
			continue
		}
		name := function.Name.Name
		switch {
		case goTestName(name, "Test") && goTestParam(function, "T"):
			names = append(names, name)
		case goTestName(name, "Fuzz") && goTestParam(function, "F"):
			names = append(names, name)
		case goTestName(name, "Example") && runnable[name]:
			names = append(names, name)
		}
	}
	return names, true
}

// goTestName reports whether name is prefix followed by nothing or by a
// character that is not a lower-case letter, as `go test` requires.
func goTestName(name, prefix string) bool {
	rest, ok := strings.CutPrefix(name, prefix)
	if !ok {
		return false
	}
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLower(r)
}

// goTestParam reports whether function takes exactly one *testing.<kind>
// parameter, under whatever name the file imports testing as, and returns
// nothing.
func goTestParam(function *ast.FuncDecl, kind string) bool {
	params := function.Type.Params
	if function.Type.Results != nil || params == nil || len(params.List) != 1 || len(params.List[0].Names) > 1 {
		return false
	}
	star, ok := params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := star.X.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == kind
}

// listFiles names every candidate file at file level with
// test-id-unmapped: the static RSpec lister (RBR-V1-001), whose files
// carry no statically certain example names.
func listFiles(runner string, candidates []string) (listing, error) {
	result := listing{}
	for _, name := range candidates {
		id, err := testid.New(runner, name)
		if err != nil {
			return result, err
		}
		result.add(id, testid.ReasonUnmapped, name)
	}
	return result, nil
}

var (
	minitestClass    = regexp.MustCompile(`^class\s+([A-Z][A-Za-z0-9_]*(?:::[A-Z][A-Za-z0-9_]*)*)\s*<\s*(\S+)\s*$`)
	minitestMethod   = regexp.MustCompile(`^\s+def\s+(test_[A-Za-z0-9_]*[?!]?)\s*(?:\(\s*\))?\s*(?:#.*)?$`)
	minitestDynamic  = regexp.MustCompile(`^\s*(?:module\s|describe[\s(]|it[\s(]|test[\s(]|include\s|extend\s|prepend\s|define_method\b|[A-Za-z0-9_:]*\.new\b|class\s+<<)`)
	minitestNested   = regexp.MustCompile(`^\s+class\s`)
	minitestSupers   = map[string]bool{"Minitest::Test": true, "MiniTest::Test": true}
	minitestTopLevel = regexp.MustCompile(`^def\s`)
)

// listMinitest is the static Minitest lister (RBR-V1-001): a file whose
// every test is a `def test_...` method of a top-level class deriving
// directly from Minitest::Test is listed method by method, `Class#method`
// (CTI-V1-007). Any construct that can add, inherit or rename tests
// (modules, nested classes, include, define_method, spec DSL, another
// superclass) lists the file at file level with test-id-unmapped, as does a
// file in which no test is found.
func listMinitest(input listInput, candidates []string) (listing, error) {
	result := listing{}
	for _, name := range candidates {
		methods, ok := minitestMethods(input.Tree, name)
		if !ok || len(methods) == 0 {
			id, err := testid.New(testid.Minitest, name)
			if err != nil {
				return result, err
			}
			result.add(id, testid.ReasonUnmapped, name)
			continue
		}
		for _, method := range methods {
			id, err := testid.MinitestMethod(name, method[0], method[1])
			if err != nil {
				return result, err
			}
			result.add(id, "", name)
		}
	}
	return result, nil
}

func minitestMethods(tree Tree, name string) ([][2]string, bool) {
	source, err := tree.Read(name, maxSourceBytes)
	if err != nil || !utf8.Valid(source) {
		return nil, false
	}
	var methods [][2]string
	class := ""
	for _, line := range strings.Split(string(source), "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case minitestDynamic.MatchString(line), minitestNested.MatchString(line), minitestTopLevel.MatchString(line):
			return nil, false
		case strings.HasPrefix(line, "class"):
			match := minitestClass.FindStringSubmatch(line)
			if match == nil || !minitestSupers[match[2]] {
				return nil, false
			}
			class = match[1]
		case strings.Contains(line, "def test_"):
			match := minitestMethod.FindStringSubmatch(line)
			if match == nil || class == "" {
				return nil, false
			}
			methods = append(methods, [2]string{class, match[1]})
		}
	}
	return methods, true
}

// nodeBin is the repository-relative path of a JavaScript runner's local
// executable: node_modules/.bin/<name> in the instance root or the nearest
// directory above it.
func nodeBin(input listInput, root, name string) (string, error) {
	if runtime.GOOS == "windows" {
		name += ".cmd"
	}
	for dir := root; ; dir = path.Dir(dir) {
		candidate := path.Join(dir, "node_modules", ".bin", name)
		if info, err := os.Stat(filepath.Join(input.Root, filepath.FromSlash(candidate))); err == nil && !info.IsDir() {
			return candidate, nil
		}
		if dir == "." {
			return "", fmt.Errorf("%s is not installed: no node_modules/.bin/%s in %s or above it", name, name, root)
		}
	}
}

// run runs argv, whose first word is repository-relative when it holds a
// slash, from the instance root.
func run(ctx context.Context, input listInput, root string, argv []string) ([]byte, error) {
	resolved := append([]string{}, argv...)
	if strings.Contains(resolved[0], "/") {
		resolved[0] = filepath.Join(input.Root, filepath.FromSlash(resolved[0]))
	}
	return input.Exec(ctx, filepath.Join(input.Root, filepath.FromSlash(root)), resolved)
}

// relativeTo is the configuration path relative to the instance root, the
// form a lister run from that root names it by.
func relativeTo(root, file string) string {
	if root == "." {
		return file
	}
	return strings.TrimPrefix(file, root+"/")
}

// repoFile is reported, an absolute path or one relative to base (an
// absolute directory), as a clean repository-relative slash path.
func repoFile(input listInput, base, reported string) (string, error) {
	reported = filepath.FromSlash(strings.ReplaceAll(reported, `\`, "/"))
	if !filepath.IsAbs(reported) {
		reported = filepath.Join(base, reported)
	}
	for _, root := range repoRoots(input.Root) {
		if rel, err := filepath.Rel(root, reported); err == nil {
			rel = filepath.ToSlash(rel)
			if testid.CleanFile(rel) && rel != "." {
				return rel, nil
			}
		}
	}
	return "", fmt.Errorf("test file %q is not under the repository root", reported)
}

func repoRoots(root string) []string {
	roots := []string{root}
	if resolved, err := filepath.EvalSymlinks(root); err == nil && resolved != root {
		roots = append(roots, resolved)
	}
	return roots
}

// listVitest runs `vitest list --json` (TIV-V1-004): one record per test,
// `name` its title path joined by " > " and `file` its absolute path. The
// title path is split on " > ", the separator Vitest joins it with.
func listVitest(ctx context.Context, input listInput, instance Declared) (listing, error) {
	bin, err := nodeBin(input, instance.Root, "vitest")
	result := listing{command: []string{"node_modules/.bin/vitest", "list", "--json", "--config", relativeTo(instance.Root, instance.Config)}}
	if err != nil {
		return result, err
	}
	result.command[0] = bin
	out, err := run(ctx, input, instance.Root, result.command)
	if err != nil {
		return result, err
	}
	var records []struct {
		Name string `json:"name"`
		File string `json:"file"`
	}
	if err := json.Unmarshal(jsonPayload(out, '['), &records); err != nil {
		return result, fmt.Errorf("vitest list --json: %v", err)
	}
	base := filepath.Join(input.Root, filepath.FromSlash(instance.Root))
	for _, record := range records {
		file, err := repoFile(input, base, record.File)
		if err != nil {
			return result, err
		}
		titles := strings.Split(record.Name, " > ")
		id, err := testid.JestTest(testid.Vitest, file, titles[:len(titles)-1], titles[len(titles)-1])
		if err != nil {
			return result, err
		}
		result.add(id, "", file)
	}
	return result, nil
}

// listJest runs `jest --listTests` (TIV-V1-004): one absolute test file
// per line, each named at file level with test-id-unmapped, since the
// listing carries no titles.
func listJest(ctx context.Context, input listInput, instance Declared) (listing, error) {
	bin, err := nodeBin(input, instance.Root, "jest")
	result := listing{command: []string{"node_modules/.bin/jest", "--listTests", "--config", relativeTo(instance.Root, instance.Config)}}
	if err != nil {
		return result, err
	}
	result.command[0] = bin
	out, err := run(ctx, input, instance.Root, result.command)
	if err != nil {
		return result, err
	}
	base := filepath.Join(input.Root, filepath.FromSlash(instance.Root))
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64<<10), maxListerOutput)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		file, err := repoFile(input, base, line)
		if err != nil {
			return result, err
		}
		id, err := testid.New(testid.Jest, file)
		if err != nil {
			return result, err
		}
		result.add(id, testid.ReasonUnmapped, file)
	}
	return result, scanner.Err()
}

// listPlaywright runs `playwright test --list --reporter=json`
// (TIV-V1-004), converts the listing to the playwright-discovery/0 receipt
// bound to the commit, the config bytes and the source digest (TJAA-V0-019),
// which refuses a filtered, sharded or erroneous listing and one whose files
// the config does not select, and names each test by project, describe
// titles and title (CTI-V1-005).
func listPlaywright(ctx context.Context, input listInput, instance Declared) (listing, error) {
	bin, err := nodeBin(input, instance.Root, "playwright")
	result := listing{command: []string{"node_modules/.bin/playwright", "test", "--list", "--reporter=json", "--config=" + relativeTo(instance.Root, instance.Config)}}
	if err != nil {
		return result, err
	}
	result.command[0] = bin
	out, err := run(ctx, input, instance.Root, result.command)
	if err != nil {
		return result, err
	}
	out = jsonPayload(out, '{')
	raw, err := typescript.PlaywrightDiscoveryFromList(input.Root, instance.Config, input.Commit, out)
	if err != nil {
		return result, fmt.Errorf("playwright-discovery/0: %v", err)
	}
	var receipt typescript.PlaywrightDiscovery
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return result, err
	}
	result.receipt = &Receipt{Profile: receipt.Profile, SourceDigest: receipt.SourceDigest}
	var named []testid.Named
	for _, root := range repoRoots(input.Root) {
		if named, err = testid.PlaywrightJSON(bytes.NewReader(out), root); err == nil {
			break
		}
	}
	if err != nil {
		return result, err
	}
	for _, n := range named {
		result.add(n.ID, n.Reason, n.ID.File)
	}
	return result, nil
}

// pythonFor is the Python a pytest instance runs: the project's own
// .venv or venv interpreter in the instance root or above, else python3 on
// PATH.
func pythonFor(input listInput, root string) string {
	interpreter := "bin/python"
	if runtime.GOOS == "windows" {
		interpreter = "Scripts/python.exe"
	}
	for dir := root; ; dir = path.Dir(dir) {
		for _, venv := range []string{".venv", "venv"} {
			candidate := path.Join(dir, venv, interpreter)
			if _, err := os.Stat(filepath.Join(input.Root, filepath.FromSlash(candidate))); err == nil {
				return candidate
			}
		}
		if dir == "." {
			return "python3"
		}
	}
}

// listPytest runs `pytest --collect-only -q` with the instance root as its
// rootdir and no cache writes (TIV-V1-004): every printed node ID becomes
// one test (CTI-V1-006). Exit status 5 (nothing collected) is an empty
// listing; any other failure, including a collection error, fails the
// lister.
func listPytest(ctx context.Context, input listInput, instance Declared) (listing, error) {
	result := listing{command: []string{pythonFor(input, instance.Root), "-m", "pytest", "--collect-only", "-q", "-p", "no:cacheprovider", "--rootdir=.", "-c", relativeTo(instance.Root, instance.Config)}}
	out, err := run(ctx, input, instance.Root, result.command)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 5 {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			break
		}
		if !strings.Contains(line, "::") {
			continue
		}
		id, err := testid.PytestNodeID(instance.Root, line)
		if err != nil {
			return result, err
		}
		result.add(id, "", id.File)
	}
	return result, nil
}

// jsonPayload drops any line a tool prints before its JSON document, which
// starts with open at the beginning of a line.
func jsonPayload(out []byte, open byte) []byte {
	if len(out) > 0 && out[0] == open {
		return out
	}
	if index := bytes.Index(out, []byte{'\n', open}); index >= 0 {
		return out[index+1:]
	}
	return out
}

// RunLister is the default Exec: argv from dir with the caller's
// environment plus no bytecode or colour output, bounded by listerTimeout
// and maxListerOutput. A failed run's error carries its stderr tail.
func RunLister(ctx context.Context, dir string, argv []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, listerTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Dir = dir
	command.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1", "NO_COLOR=1", "FORCE_COLOR=0", "CI=1")
	var stderr bytes.Buffer
	command.Stderr = &limitedBuffer{buffer: &stderr, limit: MaxErrorBytes}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	out, readErr := io.ReadAll(io.LimitReader(stdout, maxListerOutput+1))
	waitErr := command.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if len(out) > maxListerOutput {
		return nil, fmt.Errorf("%s printed more than %d bytes", argv[0], maxListerOutput)
	}
	if waitErr != nil {
		return out, &listerError{err: waitErr, stderr: strings.TrimSpace(stderr.String())}
	}
	return out, nil
}

type listerError struct {
	err    error
	stderr string
}

func (e *listerError) Error() string {
	if e.stderr == "" {
		return e.err.Error()
	}
	return e.err.Error() + ": " + e.stderr
}

func (e *listerError) Unwrap() error { return e.err }
