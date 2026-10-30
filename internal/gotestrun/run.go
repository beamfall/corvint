// Package gotestrun records one `go test -json` run as a
// corvint-go-test-run/0 document (AGW-V0-012): the Git repository identity
// observed on the host before and after the run, the module that maps package
// import paths to tree directories, and each reported test's outcome with the
// repository-relative file that declares its top-level function and that
// file's Git blob ID. The witness binds the document to a head tree only when
// that identity and every blob ID agree with the head; the document itself
// claims nothing about the head.
//
// It is a new profile beside the closed corvint-go-live-session-event/0
// stream, which it does not change.
package gotestrun

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

const (
	// Profile names the closed run document.
	Profile = "corvint-go-test-run/0"
	// MaxTests bounds the recorded tests; the rest are counted, not kept.
	MaxTests = 5000
	// DefaultTimeout bounds one run.
	DefaultTimeout = 30 * time.Minute
	// maxOutputBytes bounds the go test -json stream read.
	maxOutputBytes = 64 << 20
	// maxSourceBytes bounds one _test.go file parsed to locate a test.
	maxSourceBytes = 4 << 20
)

// Document is the closed corvint-go-test-run/0 document.
type Document struct {
	Profile          string                                        `json:"profile"`
	Command          []string                                      `json:"command,omitempty"`
	CommandWithheld  string                                        `json:"commandWithheld,omitempty"`
	Directory        string                                        `json:"directory"`
	Module           string                                        `json:"module"`
	ModuleDirectory  string                                        `json:"moduleDirectory"`
	ObjectFormat     string                                        `json:"objectFormat"`
	RepositoryBefore *jstestprovider.ApplicationRepositoryIdentity `json:"repositoryBefore,omitempty"`
	RepositoryAfter  *jstestprovider.ApplicationRepositoryIdentity `json:"repositoryAfter,omitempty"`
	Tests            []Test                                        `json:"tests"`
	TestsOmitted     int                                           `json:"testsOmitted,omitempty"`
	ExitStatus       *int                                          `json:"exitStatus,omitempty"`
	Infrastructure   *Infrastructure                               `json:"infrastructure,omitempty"`
}

// Test is one terminal test event. Package is the import path go test
// reports; Name is the full name, subtests included. File and Blob name the
// _test.go file in the package's tree directory that declares the top-level
// function, or Unbound says why none is named.
type Test struct {
	Package string `json:"package"`
	Name    string `json:"name"`
	Action  string `json:"action"`
	File    string `json:"file,omitempty"`
	Blob    string `json:"blob,omitempty"`
	Unbound string `json:"unbound,omitempty"`
}

// Infrastructure records why the run is not a complete observation.
type Infrastructure struct {
	Reason string `json:"reason"`
	Detail string `json:"detail"`
}

// Config configures one run. Dir is an absolute directory inside a Git
// worktree, under the module root; Go is the go test command (default
// go test -json -count=1, so every test executes rather than replaying a
// cached result, AGW-V0-040) and Packages its package patterns (default
// ./...).
type Config struct {
	Dir      string
	Go       []string
	Packages []string
	Timeout  time.Duration
}

var actions = map[string]bool{"pass": true, "fail": true, "skip": true}

// Run executes the command once and returns its document. An error is a
// refusal before the run; a run that could not complete is recorded as
// Infrastructure in the returned document.
func Run(ctx context.Context, config Config) (Document, error) {
	if !filepath.IsAbs(config.Dir) || filepath.Clean(config.Dir) != config.Dir {
		return Document{}, errors.New("--dir must be an absolute clean path")
	}
	dir, err := filepath.EvalSymlinks(config.Dir)
	if err != nil {
		return Document{}, err
	}
	worktree, err := enclosingWorktree(dir)
	if err != nil {
		return Document{}, err
	}
	moduleDir, module, err := enclosingModule(dir, worktree)
	if err != nil {
		return Document{}, err
	}
	directory, _ := filepath.Rel(worktree, dir)
	moduleRelative, _ := filepath.Rel(worktree, moduleDir)
	command := config.Go
	if len(command) == 0 {
		command = []string{"go", "test", "-json", "-count=1"}
	}
	packages := config.Packages
	if len(packages) == 0 {
		packages = []string{"./..."}
	}
	argv := append(append([]string{}, command...), packages...)
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	document := Document{Profile: Profile, Directory: filepath.ToSlash(directory), Module: module, ModuleDirectory: filepath.ToSlash(moduleRelative), ObjectFormat: "sha1", Tests: []Test{}}
	if secretscreen.MatchArgv(argv) {
		document.CommandWithheld = "secret-shaped"
	} else {
		document.Command = argv
	}
	document.RepositoryBefore = jstestprovider.ObserveTestRepositoryAtStart(ctx, dir)
	if document.RepositoryBefore != nil && len(document.RepositoryBefore.Revision) == 64 {
		document.ObjectFormat = "sha256"
	}
	observation := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(argv), Dir: dir, Env: os.Environ(), Timeout: timeout, OutputLimit: maxOutputBytes, OverflowPolicy: procgroup.OverflowTruncate})
	document.RepositoryAfter = jstestprovider.ObserveTestRepositoryAtStart(ctx, dir)
	if observation.ExitObserved {
		status := observation.ExitStatus
		document.ExitStatus = &status
	}
	if failure := runFailure(observation); failure != nil {
		document.Infrastructure = failure
		return document, nil
	}
	tests, cached, malformed := parseRun(observation.Stdout)
	if malformed != "" {
		document.Infrastructure = &Infrastructure{Reason: "malformed-output", Detail: malformed}
		return document, nil
	}
	if failure := cachedFailure(cached); failure != nil {
		document.Infrastructure = failure
		return document, nil
	}
	locate := newLocator(worktree, document.ModuleDirectory, module, document.ObjectFormat)
	for _, test := range tests {
		if len(document.Tests) == MaxTests {
			document.TestsOmitted++
			continue
		}
		test.File, test.Blob, test.Unbound = locate.find(test.Package, test.Name)
		document.Tests = append(document.Tests, test)
	}
	return document, nil
}

// event is the subset of a test2json event the document reads.
type event struct {
	Action  string
	Package string
	Test    string
	Output  string
}

// parseEvents keeps the last terminal action of each (package, test) pair,
// sorted. A non-JSON line, as go test prints before -json takes effect, or an
// output overflow makes the stream malformed rather than partly read.
func parseEvents(stdout []byte) ([]Test, string) {
	tests, _, malformed := parseRun(stdout)
	return tests, malformed
}

// cachedFailure records a run in which go test replayed at least one
// package's result from its build cache instead of executing it
// (AGW-V0-040). Its test events are a recording of an earlier run, possibly of
// other content, so the document records infrastructure and the witness binds
// none of its passes.
func cachedFailure(cached []string) *Infrastructure {
	if len(cached) == 0 {
		return nil
	}
	detail := "go test replayed the cached result of " + cached[0] + " instead of executing it"
	if len(cached) > 1 {
		detail += " and of " + strconv.Itoa(len(cached)-1) + " more package(s)"
	}
	return &Infrastructure{Reason: "cached-result", Detail: detail + "; run with -count=1"}
}

// parseRun is parseEvents that also lists, sorted, each package whose
// package-level output reports a result go test took from its cache.
func parseRun(stdout []byte) ([]Test, []string, string) {
	last := map[[2]string]string{}
	cached := map[string]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	scanner.Buffer(make([]byte, 0, 64<<10), maxOutputBytes)
	line := 0
	for scanner.Scan() {
		line++
		text := bytes.TrimSpace(scanner.Bytes())
		if len(text) == 0 {
			continue
		}
		var item event
		if err := json.Unmarshal(text, &item); err != nil || item.Action == "" {
			return nil, nil, "line " + strconv.Itoa(line) + " of the go test output is not a test2json event"
		}
		if item.Test != "" && actions[item.Action] {
			last[[2]string{item.Package, item.Test}] = item.Action
		}
		if item.Test == "" && item.Action == "output" && cachedResult(item.Output) {
			cached[item.Package] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, "the go test output could not be read: " + err.Error()
	}
	tests := make([]Test, 0, len(last))
	for key, action := range last {
		tests = append(tests, Test{Package: key[0], Name: key[1], Action: action})
	}
	sort.Slice(tests, func(i, j int) bool {
		if tests[i].Package != tests[j].Package {
			return tests[i].Package < tests[j].Package
		}
		return tests[i].Name < tests[j].Name
	})
	packages := make([]string, 0, len(cached))
	for name := range cached {
		packages = append(packages, name)
	}
	sort.Strings(packages)
	return tests, packages, ""
}

// cachedResult reports whether a package-level output line is go test's
// summary of a result taken from its cache, such as "ok  \tpkg\t(cached)".
func cachedResult(output string) bool {
	output = strings.TrimSpace(output)
	return strings.HasPrefix(output, "ok") && strings.Contains(output, "(cached)")
}

// locator maps an import path to its tree directory through the module path
// and finds the one _test.go file there that declares a test's top-level
// function.
type locator struct {
	worktree, moduleDir, module, format string
	declared                            map[string]map[string][]string
	blobs                               map[string]string
}

func newLocator(worktree, moduleDir, module, format string) *locator {
	return &locator{worktree: worktree, moduleDir: moduleDir, module: module, format: format, declared: map[string]map[string][]string{}, blobs: map[string]string{}}
}

func (l *locator) find(importPath, name string) (string, string, string) {
	directory, ok := l.directory(importPath)
	if !ok {
		return "", "", "the package is outside module " + l.module
	}
	declared, err := l.declarations(directory)
	if err != nil {
		return "", "", "the package directory " + directory + " cannot be read: " + err.Error()
	}
	function, _, _ := strings.Cut(name, "/")
	files := declared[function]
	switch len(files) {
	case 0:
		return "", "", "no _test.go file in " + directory + " declares " + function
	case 1:
		return files[0], l.blobs[files[0]], ""
	}
	return "", "", "more than one _test.go file in " + directory + " declares " + function
}

func (l *locator) directory(importPath string) (string, bool) {
	rest, ok := "", importPath == l.module
	if !ok {
		rest, ok = strings.CutPrefix(importPath, l.module+"/")
	}
	if !ok || (rest != "" && (path.Clean(rest) != rest || !filepath.IsLocal(rest))) {
		return "", false
	}
	return path.Join(l.moduleDir, rest), true
}

func (l *locator) declarations(directory string) (map[string][]string, error) {
	if declared, ok := l.declared[directory]; ok {
		return declared, nil
	}
	declared := map[string][]string{}
	root, err := os.OpenRoot(l.worktree)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	entries, err := fsReadDir(root, directory)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry, "_test.go") {
			continue
		}
		file := path.Join(directory, entry)
		data, err := readRegular(root, file)
		if err != nil {
			return nil, err
		}
		l.blobs[file] = BlobID(l.format, data)
		parsed, err := parser.ParseFile(token.NewFileSet(), file, data, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		for _, declaration := range parsed.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok && function.Recv == nil {
				declared[function.Name.Name] = append(declared[function.Name.Name], file)
			}
		}
	}
	l.declared[directory] = declared
	return declared, nil
}

func fsReadDir(root *os.Root, directory string) ([]string, error) {
	handle, err := root.Open(filepath.FromSlash(directory))
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	names, err := handle.Readdirnames(-1)
	sort.Strings(names)
	return names, err
}

// readRegular reads one regular file below root; a link or an oversize file is
// refused rather than followed or truncated.
func readRegular(root *os.Root, name string) ([]byte, error) {
	info, err := root.Lstat(filepath.FromSlash(name))
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxSourceBytes {
		return nil, fmt.Errorf("%s is not a regular file within the source bound", name)
	}
	return root.ReadFile(filepath.FromSlash(name))
}

// BlobID is Git's blob object ID of data in the named object format.
func BlobID(format string, data []byte) string {
	header := "blob " + strconv.Itoa(len(data)) + "\x00"
	if format == "sha256" {
		sum := sha256.Sum256(append([]byte(header), data...))
		return hex.EncodeToString(sum[:])
	}
	sum := sha1.Sum(append([]byte(header), data...))
	return hex.EncodeToString(sum[:])
}

var blobPattern = map[string]*regexp.Regexp{"sha1": regexp.MustCompile(`^[0-9a-f]{40}$`), "sha256": regexp.MustCompile(`^[0-9a-f]{64}$`)}

// Validate checks the closed shape of a decoded document.
func Validate(document Document) error {
	switch {
	case document.Profile != Profile:
		return errors.New("is not a " + Profile + " document")
	case blobPattern[document.ObjectFormat] == nil:
		return errors.New("names object format " + strconv.Quote(document.ObjectFormat))
	case document.Module == "":
		return errors.New("names no module")
	case !localSlash(document.ModuleDirectory) || !localSlash(document.Directory):
		return errors.New("names a directory that is not a clean repository-relative path")
	case document.Tests == nil:
		return errors.New("has no tests array")
	case len(document.Tests) > MaxTests:
		return errors.New("records more tests than the bound")
	}
	for _, test := range document.Tests {
		switch {
		case test.Package == "" || test.Name == "" || !actions[test.Action]:
			return errors.New("records a test without a package, name or terminal action")
		case test.File == "" && test.Unbound == "":
			return errors.New("records test " + test.Name + " with neither a file nor an unbound reason")
		case test.File != "" && (!localSlash(test.File) || !blobPattern[document.ObjectFormat].MatchString(test.Blob)):
			return errors.New("records test " + test.Name + " with an invalid file or blob ID")
		}
	}
	return nil
}

func localSlash(name string) bool {
	return name == "." || (name != "" && path.Clean(name) == name && filepath.IsLocal(filepath.FromSlash(name)) && !strings.Contains(name, "\\"))
}

// Decode reads and validates one document.
func Decode(data []byte) (Document, error) {
	var document Document
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return Document{}, errors.New("is not a " + Profile + " document: " + err.Error())
	}
	if err := Validate(document); err != nil {
		return Document{}, err
	}
	return document, nil
}

func enclosingWorktree(dir string) (string, error) {
	for current := dir; ; {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("no Git worktree encloses --dir")
		}
		current = parent
	}
}

var moduleDirective = regexp.MustCompile(`(?m)^module[ \t]+("(?:[^"\\]|\\.)*"|\S+)[ \t]*(?://.*)?$`)

// enclosingModule finds the nearest go.mod at or above dir and inside the
// worktree, and its module path.
func enclosingModule(dir, worktree string) (string, string, error) {
	for current := dir; ; {
		if data, err := os.ReadFile(filepath.Join(current, "go.mod")); err == nil {
			match := moduleDirective.FindSubmatch(data)
			if match == nil {
				return "", "", errors.New("go.mod in " + current + " declares no module path")
			}
			module := string(match[1])
			if strings.HasPrefix(module, `"`) {
				if module, err = strconv.Unquote(module); err != nil {
					return "", "", errors.New("go.mod in " + current + " declares an unreadable module path")
				}
			}
			return current, module, nil
		}
		if current == worktree {
			return "", "", errors.New("no go.mod encloses --dir inside its worktree")
		}
		current = filepath.Dir(current)
	}
}

func runFailure(observation procgroup.Observation) *Infrastructure {
	switch {
	case !observation.Started:
		return &Infrastructure{Reason: "start-failed", Detail: "the go test command could not be started"}
	case observation.TimedOut:
		return &Infrastructure{Reason: "timeout", Detail: "go test did not complete within the --timeout bound"}
	case observation.Cancelled:
		return &Infrastructure{Reason: "cancelled", Detail: "the run was cancelled"}
	case observation.Signal != "":
		return &Infrastructure{Reason: "runner-signalled", Detail: "go test was terminated by a signal"}
	case !observation.WaitCompleted || !observation.ExitObserved:
		return &Infrastructure{Reason: "runner-exit-unobserved", Detail: "the go test exit status was not observed"}
	case observation.OutputOverflow:
		return &Infrastructure{Reason: "output-overflow", Detail: "the go test -json stream exceeded the output bound"}
	}
	return nil
}

func resolveArgv(argv []string) []string {
	resolved, err := exec.LookPath(argv[0])
	if err != nil {
		return argv
	}
	if absolute, err := filepath.Abs(resolved); err == nil {
		return append([]string{absolute}, argv[1:]...)
	}
	return argv
}
