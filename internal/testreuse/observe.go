// SPDX-License-Identifier: Apache-2.0

package testreuse

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Declarations the snapshot reads from the committed tree.
const (
	ReadScopesPath = affected.ReadScopesPath
	SplitAllowPath = ".github/cishards/test-split-allow.json"
)

const (
	maxDeclarationBytes = 1 << 20
	maxGoListBytes      = 256 << 20
	maxGitBytes         = 64 << 20
	commandTimeout      = 5 * time.Minute
)

// Entry is one Git tree entry.
type Entry struct {
	Mode   string
	Object string
}

// GoPackage is the subset of one `go list -json` record the digest reads.
type GoPackage struct {
	ImportPath   string
	Dir          string
	Standard     bool
	ForTest      string
	GoFiles      []string
	CgoFiles     []string
	CFiles       []string
	CXXFiles     []string
	HFiles       []string
	SFiles       []string
	SysoFiles    []string
	EmbedFiles   []string
	TestGoFiles  []string
	XTestGoFiles []string
	Deps         []string
	Incomplete   bool
	Error        *struct{ Err string }
	DepsErrors   []*struct{ Err string }
}

func (p GoPackage) compiled() []string {
	var names []string
	for _, group := range [][]string{p.GoFiles, p.CgoFiles, p.CFiles, p.CXXFiles, p.HFiles, p.SFiles, p.SysoFiles, p.EmbedFiles} {
		names = append(names, group...)
	}
	return names
}

// Snapshot is one observation of a checkout: its commit, its tree, every
// tracked entry, every worktree path Git reports as not matching it, the Go
// packages and the declarations the digest reads (AFP-V0-2321).
type Snapshot struct {
	Root    string
	Head    string
	Tree    string
	Entries map[string]Entry
	// Dirty holds each modified, staged, deleted, untracked or ignored
	// worktree path; a directory entry ends in "/".
	Dirty []string
	// Packages holds every listed package by import path, test variants
	// included ("P [P.test]", "P.test").
	Packages map[string]GoPackage
	// Scopes holds the declared read scopes by package directory.
	Scopes map[string][]string
	// DeclarationError is set when the read-scope declaration is dirty or
	// invalid: no package is then eligible.
	DeclarationError string
	// Sliced holds the import paths CI may split into slices (AFP-V0-041).
	Sliced map[string]bool
	// Unbounded holds the affected graph's UNBOUNDED_READER import paths.
	Unbounded map[string]bool
	// Universe lists the module's packages in go list order.
	Universe []string
}

// GitObserveArgs precede every Git command an observation runs: no
// replacements, quoted paths, filesystem monitor or untracked cache, so status
// reflects the bytes on disk.
var GitObserveArgs = []string{"--no-replace-objects", "-c", "core.quotePath=false", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false"}

// StatusArgs is the status read a snapshot's StatusDigest names.
var StatusArgs = []string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching", "--no-renames"}

func (s *Snapshot) relative(directory string) (string, bool) {
	if directory == "" {
		return "", false
	}
	relative, err := filepath.Rel(s.Root, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

// under lists the tracked paths below directory ("." is the whole tree).
func (s *Snapshot) under(directory string) []string {
	var paths []string
	if directory == "." || directory == "" {
		for path := range s.Entries {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		return paths
	}
	prefix := directory + "/"
	for path := range s.Entries {
		if strings.HasPrefix(path, prefix) {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func (s *Snapshot) dirtyWithin(inputs map[string]Input, directories []string) (string, bool) {
	for _, dirty := range s.Dirty {
		trimmed := strings.TrimSuffix(dirty, "/")
		if _, ok := inputs[trimmed]; ok {
			return dirty, true
		}
		if _, ok := inputs[trimmed+"/"]; ok {
			return dirty, true
		}
		for _, directory := range directories {
			if directory == "." || strings.HasPrefix(trimmed, directory+"/") || trimmed == directory ||
				(strings.HasSuffix(dirty, "/") && strings.HasPrefix(directory, trimmed+"/")) {
				return dirty, true
			}
		}
		if strings.HasSuffix(dirty, "/") {
			for path := range inputs {
				if strings.HasPrefix(path, trimmed+"/") {
					return dirty, true
				}
			}
		}
	}
	return "", false
}

// closure lists the packages compiled into importPath's test binary, or into
// the package itself when it has no test files.
func (s *Snapshot) closure(importPath string) ([]GoPackage, error) {
	root, ok := s.Packages[importPath+".test"]
	if !ok {
		root, ok = s.Packages[importPath]
		if !ok {
			return nil, fmt.Errorf("%s: go list did not list %s", ErrorPackageUnlisted, importPath)
		}
	}
	members := []GoPackage{root}
	if root.ImportPath == importPath+".test" {
		members = nil // the generated test main holds no repository file
	}
	if err := listError(root); err != nil {
		return nil, err
	}
	for _, name := range root.Deps {
		member, ok := s.Packages[name]
		if !ok {
			return nil, fmt.Errorf("%s: %s imports %s, which go list did not list", ErrorClosureUnlisted, importPath, name)
		}
		if err := listError(member); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, nil
}

func listError(p GoPackage) error {
	switch {
	case p.Error != nil:
		return fmt.Errorf("%s: %s: %s", ErrorGoListPackage, p.ImportPath, p.Error.Err)
	case len(p.DepsErrors) != 0:
		return fmt.Errorf("%s: %s: dependency error", ErrorGoListPackage, p.ImportPath)
	case p.Incomplete:
		return fmt.Errorf("%s: %s: incomplete", ErrorGoListPackage, p.ImportPath)
	}
	return nil
}

// ObserveOptions configures Observe.
type ObserveOptions struct {
	Root string
	// Git and Go name the executables; empty looks each up on PATH.
	Git, Go string
	// Env is the environment go list and go env run with; nil is os.Environ.
	Env []string
	// Unbounded is the affected graph's UNBOUNDED_READER unit set, by
	// import path. The caller computes it from the same checkout.
	Unbounded map[string]bool
	// BuildFlags are the argv's build flags (BuildFlags), passed to every
	// go list so the closure is the one the test binary is built from.
	BuildFlags []string
}

var hexObject = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// Observe reads one snapshot of the checkout at Root.
func Observe(ctx context.Context, options ObserveOptions) (*Snapshot, error) {
	git, err := executable(options.Git, "git")
	if err != nil {
		return nil, err
	}
	gotool, err := executable(options.Go, "go")
	if err != nil {
		return nil, err
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return nil, err
	}
	if root, err = filepath.EvalSymlinks(root); err != nil {
		return nil, err
	}
	env := options.Env
	if env == nil {
		env = os.Environ()
	}
	snapshot := &Snapshot{Root: root, Entries: map[string]Entry{}, Packages: map[string]GoPackage{}, Sliced: map[string]bool{}, Unbounded: options.Unbounded}
	if snapshot.Unbounded == nil {
		snapshot.Unbounded = map[string]bool{}
	}
	gitRun := func(arguments ...string) ([]byte, error) {
		return runBounded(ctx, root, gitEnv(env), maxGitBytes, git, append(append([]string{}, GitObserveArgs...), arguments...)...)
	}
	head, err := gitRun("rev-parse", "--verify", "-q", "HEAD^{commit}")
	if err != nil {
		return nil, fmt.Errorf("HEAD: %w", err)
	}
	tree, err := gitRun("rev-parse", "--verify", "-q", "HEAD^{tree}")
	if err != nil {
		return nil, fmt.Errorf("HEAD tree: %w", err)
	}
	snapshot.Head, snapshot.Tree = strings.TrimSpace(string(head)), strings.TrimSpace(string(tree))
	if !hexObject.MatchString(snapshot.Head) || !hexObject.MatchString(snapshot.Tree) {
		return nil, errors.New("HEAD did not resolve to a full object id")
	}
	listing, err := gitRun("ls-tree", "-r", "-z", "--full-tree", snapshot.Head)
	if err != nil {
		return nil, fmt.Errorf("ls-tree: %w", err)
	}
	for _, record := range bytes.Split(listing, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		meta, path, ok := bytes.Cut(record, []byte{'\t'})
		fields := strings.Fields(string(meta))
		if !ok || len(fields) != 3 {
			return nil, errors.New("ls-tree: malformed record")
		}
		snapshot.Entries[string(path)] = Entry{Mode: fields[0], Object: fields[2]}
	}
	status, err := gitRun(StatusArgs...)
	if err != nil {
		return nil, fmt.Errorf("status: %w", err)
	}
	// A path marked assume-unchanged or skip-worktree can differ from its
	// blob while status omits it, so it counts as dirty (lower-case tag or S).
	flagged, err := gitRun("ls-files", "-v", "-z")
	if err != nil {
		return nil, fmt.Errorf("ls-files -v: %w", err)
	}
	for _, record := range bytes.Split(flagged, []byte{0}) {
		if len(record) > 2 && (record[0] == 'S' || (record[0] >= 'a' && record[0] <= 'z')) {
			snapshot.Dirty = append(snapshot.Dirty, string(record[2:]))
		}
	}
	for _, record := range bytes.Split(status, []byte{0}) {
		if len(record) < 4 {
			continue
		}
		snapshot.Dirty = append(snapshot.Dirty, string(record[3:]))
	}
	sort.Strings(snapshot.Dirty)
	snapshot.readDeclarations(gitRun)
	// GOFLAGS may come from the environment or Go's persistent configuration.
	// Refuse flags whose extra inputs are outside the modeled closure before listing.
	goFlags, err := runBounded(ctx, root, goEnv(env), 1<<20, gotool, "env", "GOFLAGS")
	if err != nil {
		return nil, fmt.Errorf("go env GOFLAGS: %w", err)
	}
	if err := CheckArgv(append([]string{"-count=1"}, strings.Fields(string(goFlags))...)); err != nil {
		return nil, fmt.Errorf("GOFLAGS: %w", err)
	}

	universe, err := runBounded(ctx, root, goEnv(env), maxGoListBytes, gotool, append(append([]string{"list", "-e"}, options.BuildFlags...), "./...")...)
	if err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}
	snapshot.Universe = strings.Fields(string(universe))
	fields := "ImportPath,Dir,Standard,ForTest,GoFiles,CgoFiles,CFiles,CXXFiles,HFiles,SFiles,SysoFiles,EmbedFiles,TestGoFiles,XTestGoFiles,Deps,Incomplete,Error,DepsErrors"
	listed, err := runBounded(ctx, root, goEnv(env), maxGoListBytes, gotool, append(append([]string{"list", "-e", "-deps", "-test", "-json=" + fields}, options.BuildFlags...), "./...")...)
	if err != nil {
		return nil, fmt.Errorf("go list -deps -test: %w", err)
	}
	decoder := jsontext.NewDecoder(bytes.NewReader(listed))
	for {
		var p GoPackage
		if err := json.UnmarshalDecode(decoder, &p); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("go list output: %w", err)
		}
		snapshot.Packages[p.ImportPath] = p
	}
	return snapshot, nil
}

// readDeclarations reads the read-scope and split declarations from the
// committed tree. A dirty or invalid read-scope declaration leaves no package
// eligible; a split declaration that cannot be read marks every package
// sliced, which also leaves none eligible.
func (s *Snapshot) readDeclarations(gitRun func(...string) ([]byte, error)) {
	for _, dirty := range s.Dirty {
		trimmed := strings.TrimSuffix(dirty, "/")
		if dirty == ReadScopesPath || (strings.HasSuffix(dirty, "/") && strings.HasPrefix(ReadScopesPath, trimmed+"/")) {
			s.DeclarationError = ErrorDeclarationDirty + ": " + ReadScopesPath
		}
		if dirty == SplitAllowPath || (strings.HasSuffix(dirty, "/") && strings.HasPrefix(SplitAllowPath, trimmed+"/")) {
			s.DeclarationError = ErrorDeclarationDirty + ": " + SplitAllowPath
		}
	}
	s.Scopes = map[string][]string{}
	if entry, ok := s.Entries[ReadScopesPath]; ok {
		body, err := gitRun("cat-file", "blob", entry.Object)
		if err == nil {
			s.Scopes, err = ParseReadScopes(body)
		}
		if err != nil && s.DeclarationError == "" {
			s.DeclarationError = ErrorDeclarationBroken + ": " + ReadScopesPath + ": " + err.Error()
		}
	}
	if entry, ok := s.Entries[SplitAllowPath]; ok {
		body, err := gitRun("cat-file", "blob", entry.Object)
		var declared struct {
			Profile  string            `json:"profile"`
			Packages map[string]string `json:"packages"`
		}
		if err == nil {
			err = json.Unmarshal(body, &declared)
		}
		if err != nil || declared.Profile != "corvint-ci-test-split-allow/0" {
			if s.DeclarationError == "" {
				s.DeclarationError = ErrorDeclarationBroken + ": " + SplitAllowPath
			}
			return
		}
		for name := range declared.Packages {
			s.Sliced[name] = true
		}
	}
}

// ParseReadScopes validates a read-scope declaration exactly as the planner
// and the confinement wrapper do (AFP-V0-023).
func ParseReadScopes(body []byte) (map[string][]string, error) {
	if len(body) > maxDeclarationBytes {
		return nil, fmt.Errorf("exceeds %d bytes", maxDeclarationBytes)
	}
	var declared struct {
		Profile  string              `json:"profile"`
		Packages map[string][]string `json:"packages"`
	}
	if err := json.Unmarshal(body, &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != "corvint-test-read-scopes/0" || declared.Packages == nil || len(declared.Packages) > 4096 {
		return nil, errors.New("profile must be corvint-test-read-scopes/0 with at most 4096 packages")
	}
	for directory, entries := range declared.Packages {
		if !affected.ValidRelativePath(directory) || entries == nil || len(entries) > 256 {
			return nil, fmt.Errorf("package %q", directory)
		}
		previous := ""
		for _, entry := range entries {
			if !affected.ValidReadScopeEntry(entry) || entry <= previous {
				return nil, fmt.Errorf("package %q entry %q", directory, entry)
			}
			previous = entry
		}
	}
	return declared.Packages, nil
}

// ObserveIdentity resolves the identity of argv and the harness paths over
// snapshot (AFP-V0-2322). A harness path must be tracked and clean.
func ObserveIdentity(ctx context.Context, snapshot *Snapshot, goExecutable string, env []string, argv, harness []string) (Identity, error) {
	if err := CheckArgv(argv); err != nil {
		return Identity{}, err
	}
	gotool, err := executable(goExecutable, "go")
	if err != nil {
		return Identity{}, err
	}
	if env == nil {
		env = os.Environ()
	}
	raw, err := runBounded(ctx, snapshot.Root, goEnv(env), 1<<20, gotool, append([]string{"env", "-json"}, IdentityEnv...)...)
	if err != nil {
		return Identity{}, fmt.Errorf("go env: %w", err)
	}
	identity := Identity{Profile: IdentityProfile, Env: map[string]string{}, Argv: append([]string{}, argv...), Harness: []Input{}}
	if err := json.Unmarshal(raw, &identity.Env); err != nil {
		return Identity{}, fmt.Errorf("go env: %w", err)
	}
	for _, name := range IdentityEnv {
		if _, ok := identity.Env[name]; !ok {
			identity.Env[name] = ""
		}
	}
	inputs := map[string]Input{}
	var directories []string
	for _, path := range harness {
		cleaned := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(path)), "/")
		if !affected.ValidRelativePath(cleaned) {
			return Identity{}, fmt.Errorf("harness path %q is not a clean repository-relative path", path)
		}
		if entry, ok := snapshot.Entries[cleaned]; ok {
			inputs[cleaned] = Input{Path: cleaned, Mode: entry.Mode, Object: entry.Object}
			continue
		}
		files := snapshot.under(cleaned)
		if len(files) == 0 {
			return Identity{}, fmt.Errorf("harness path %q is not tracked at HEAD", path)
		}
		directories = append(directories, cleaned)
		inputs[cleaned] = Input{Path: cleaned, Mode: "dir", Object: "dir"}
		for _, file := range files {
			entry := snapshot.Entries[file]
			inputs[file] = Input{Path: file, Mode: entry.Mode, Object: entry.Object}
		}
	}
	if path, dirty := snapshot.dirtyWithin(inputs, directories); dirty {
		return Identity{}, fmt.Errorf("harness input %s is dirty", path)
	}
	for _, key := range sortedKeys(inputs) {
		identity.Harness = append(identity.Harness, inputs[key])
	}
	return identity, nil
}

func executable(given, name string) (string, error) {
	if given != "" {
		return given, nil
	}
	return exec.LookPath(name)
}

// gitEnv runs Git without user or system configuration, replacements or
// optional locks, so observation never writes the repository (invariant 4).
func gitEnv(env []string) []string {
	kept := []string{}
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "GIT_") {
			continue
		}
		kept = append(kept, entry)
	}
	return append(kept, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_NO_REPLACE_OBJECTS=1", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
}

// goEnv keeps the caller's Go environment but never lets go list download.
func goEnv(env []string) []string {
	kept := []string{}
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if name == "GOPROXY" || name == "GOFLAGS_MODCACHE_RW" {
			continue
		}
		kept = append(kept, entry)
	}
	return append(kept, "GOPROXY=off")
}

func runBounded(ctx context.Context, dir string, env []string, limit int64, name string, arguments ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, name, arguments...)
	command.Dir = dir
	command.Env = env
	var stdout, stderr bytes.Buffer
	command.Stdout = &limitedWriter{buffer: &stdout, limit: limit}
	command.Stderr = &limitedWriter{buffer: &stderr, limit: 1 << 16}
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("%s %s: %w: %s", filepath.Base(name), strings.Join(arguments[:min(len(arguments), 3)], " "), err, strings.TrimSpace(stderr.String()))
	}
	if writer := command.Stdout.(*limitedWriter); writer.over {
		return nil, fmt.Errorf("%s output exceeds %d bytes", filepath.Base(name), limit)
	}
	return stdout.Bytes(), nil
}

type limitedWriter struct {
	buffer *bytes.Buffer
	limit  int64
	over   bool
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	remaining := w.limit - int64(w.buffer.Len())
	if int64(len(p)) > remaining {
		w.over = true
		if remaining > 0 {
			w.buffer.Write(p[:remaining])
		}
		return len(p), nil
	}
	return w.buffer.Write(p)
}
