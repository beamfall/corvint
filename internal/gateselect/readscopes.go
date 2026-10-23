package gateselect

// The project-owned read-scope declaration (AFP-V0-023). It mirrors
// internal/liveverify/affected/golang/readscopes.go and InReadScope in
// internal/liveverify/affected/readers.go; the protected wrapper
// .github/testconfine enforces it. A declared package leaves rule (d): it is
// selected only when a dirty path is in its declared scope.

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	readScopesPath       = ".corvint/test-read-scopes.json"
	readScopesProfile    = "corvint-test-read-scopes/0"
	maxReadScopeBytes    = 1 << 20
	maxReadScopePackages = 4096
	maxReadScopeEntries  = 256
)

type readScopeFile struct {
	Profile  string              `json:"profile"`
	Packages map[string][]string `json:"packages"`
}

// readScopes returns the declared scopes by package directory, nil with no
// declaration, or an error for any declaration that is not exactly valid,
// including one naming a directory that holds no indexed package.
func (index *repositoryIndex) readScopes(root string) (map[string][]string, error) {
	name := filepath.Join(root, filepath.FromSlash(readScopesPath))
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", readScopesPath)
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxReadScopeBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxReadScopeBytes {
		return nil, fmt.Errorf("%s exceeds %d bytes", readScopesPath, maxReadScopeBytes)
	}
	scopes, err := parseReadScopes(body)
	if err != nil {
		return nil, err
	}
	for directory := range scopes {
		if index.packages[directory] == nil {
			return nil, fmt.Errorf("%s: package %q", readScopesPath, directory)
		}
	}
	return scopes, nil
}

// parseReadScopes decodes one declaration body and checks its exact grammar,
// without asking whether a declared directory holds a package.
func parseReadScopes(body []byte) (map[string][]string, error) {
	var declared readScopeFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != readScopesProfile || declared.Packages == nil || len(declared.Packages) > maxReadScopePackages {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d packages", readScopesPath, readScopesProfile, maxReadScopePackages)
	}
	for directory, entries := range declared.Packages {
		if !validRelativePath(directory) || entries == nil || len(entries) > maxReadScopeEntries {
			return nil, fmt.Errorf("%s: package %q", readScopesPath, directory)
		}
		previous := ""
		for _, entry := range entries {
			if !validReadScopeEntry(entry) || entry <= previous {
				return nil, fmt.Errorf("%s: package %q entry %q", readScopesPath, directory, entry)
			}
			previous = entry
		}
	}
	return declared.Packages, nil
}

// inReadScope reports whether dirtyPath bears on the declared package in
// directory: the declaration itself, a path in the package's subtree, or a
// declared entry, a path in a declared subtree, or an ancestor of an entry.
func inReadScope(directory string, scope []string, dirtyPath string) bool {
	return dirtyPath == readScopesPath || inDeclaredScope(directory, scope, dirtyPath)
}

// inDeclaredScope is inReadScope without the declaration itself: the
// package's subtree and its declared entries only.
func inDeclaredScope(directory string, scope []string, dirtyPath string) bool {
	if dirtyPath == directory || strings.HasPrefix(dirtyPath, directory+"/") {
		return true
	}
	for _, entry := range scope {
		target := strings.TrimSuffix(entry, "/")
		if dirtyPath == target || strings.HasPrefix(target, dirtyPath+"/") || (target != entry && strings.HasPrefix(dirtyPath, entry)) {
			return true
		}
	}
	return false
}

// validReadScopeEntry is a canonical path, optionally naming a subtree with
// one trailing "/", outside the root .git directory.
func validReadScopeEntry(entry string) bool {
	trimmed := strings.TrimSuffix(entry, "/")
	return validRelativePath(trimmed) && trimmed != ".git" && !strings.HasPrefix(trimmed, ".git/")
}

// validRelativePath mirrors affected.ValidRelativePath.
func validRelativePath(value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.HasPrefix(value, "/") || strings.ContainsRune(value, 0) || strings.Contains(value, `\`) {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	return true
}

// declaredBears reports whether dirtyPath bears on the package in directory
// through the declaration, given the current declaration scopes and prior, the
// declaration the plan was measured from (nil when unknown). With a known base
// the declaration path bears only on a package whose key was added, whose
// entries changed or whose key was removed, or whose subtree or declared
// entries hold that path (AFP-V0-050, as the planner's
// Graph.declarationChanged); with an unknown base, and for every other path,
// the AFP-V0-023 rule applies.
func declaredBears(directory string, scopes, prior map[string][]string, dirtyPath string) bool {
	scope, declared := scopes[directory]
	if dirtyPath != readScopesPath || prior == nil {
		return declared && inReadScope(directory, scope, dirtyPath)
	}
	before, wasDeclared := prior[directory]
	if !declared {
		return wasDeclared
	}
	return !wasDeclared || !slices.Equal(before, scope) || inDeclaredScope(directory, scope, dirtyPath)
}

// The base declaration (AFP-V0-050) is read from Git objects with the git
// executable, because this package stays on the standard library and the
// stdlib-only internal/taskprojection leaf: the tools/gate-ledger fixture
// compiles these files into a module with no other dependency. Its checks mirror CommittedReadScopes in
// internal/liveverify/affected/golang/readscopes.go.

const (
	maxBaseTreeBytes    = 64 << 20
	maxWorkspaceBytes   = 4 << 20
	baseGitDeadline     = 30 * time.Second
	regularFileMode     = "100644"
	executableFileMode  = "100755"
	treeMode            = "040000"
	workspaceManifest   = "go.work"
	moduleManifestName  = "go.mod"
	baseTreeEntryFields = 3
)

var fullObjectID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// skippedDirectories mirrors affected.SkippedDirectories less the names the Go
// plugin admits (build, dist, target): the observer's walk does not descend
// them.
var skippedDirectories = map[string]bool{
	"node_modules": true, "vendor": true, "__pycache__": true, "venv": true, "site-packages": true,
}

type baseTreeEntry struct{ mode, object string }

// baseReadScopes returns the declaration at the commit the plan was measured
// from: the receipt's range.base, else its revision (HEAD). A commit holding no
// declaration is an empty map. It is nil, unknown, for a --snapshot plan, when
// the commit is not a full object id, when the tree, the declaration or go.work
// cannot be read or is not a regular file, when the declaration is oversized or
// invalid, or when it declares a directory that holds no observed Go package at
// that commit.
func baseReadScopes(plan Receipt, root string) map[string][]string {
	commit := plan.Range.Base
	if commit == "" {
		commit = plan.Revision
	}
	if plan.Snapshot != nil || !fullObjectID.MatchString(commit) {
		return nil
	}
	listing, err := baseGit(root, maxBaseTreeBytes, "ls-tree", "-r", "-t", "-z", "--full-tree", commit+"^{commit}")
	if err != nil {
		return nil
	}
	entries, ok := parseBaseTree(listing)
	if !ok {
		return nil
	}
	scopes := map[string][]string{}
	if body, present, ok := baseBlob(root, entries, readScopesPath, maxReadScopeBytes); !ok {
		return nil
	} else if present {
		if scopes, err = parseReadScopes(body); err != nil {
			return nil
		}
	}
	listed, err := baseWorkspace(root, entries)
	if err != nil {
		return nil
	}
	goDirectories := map[string]bool{}
	for name, entry := range entries {
		if regularMode(entry.mode) && strings.HasSuffix(name, ".go") {
			goDirectories[path.Dir(name)] = true
		}
	}
	for directory := range scopes {
		if !walkedDirectory(directory) || !goDirectories[directory] {
			return nil
		}
		owner, observed := baseOwner(entries, listed, directory)
		if !observed || inTestdataComponent(relativeTo(directory, owner)) {
			return nil
		}
	}
	return scopes
}

// parseBaseTree reads `git ls-tree -r -t -z` records, `<mode> <type> <object>\t<path>`.
// Trees are listed too, so an empty directory is an entry, not an absence.
func parseBaseTree(listing []byte) (map[string]baseTreeEntry, bool) {
	entries := map[string]baseTreeEntry{}
	for _, record := range strings.Split(strings.TrimSuffix(string(listing), "\x00"), "\x00") {
		if record == "" {
			continue
		}
		header, name, found := strings.Cut(record, "\t")
		fields := strings.Fields(header)
		if _, duplicate := entries[name]; duplicate || !found || len(fields) != baseTreeEntryFields || len(entries) >= maxIndexEntries {
			return nil, false
		}
		entries[name] = baseTreeEntry{mode: fields[0], object: fields[2]}
	}
	return entries, true
}

// baseBlob reads the regular file name from the base tree, bounded by limit.
// An absent name is not present; a name that is not a regular file, or a
// directory, is not ok.
func baseBlob(root string, entries map[string]baseTreeEntry, name string, limit int64) (body []byte, present, ok bool) {
	entry, found := entries[name]
	if !found {
		for other := range entries {
			if strings.HasPrefix(other, name+"/") {
				return nil, false, false
			}
		}
		return nil, false, true
	}
	if !regularMode(entry.mode) || !fullObjectID.MatchString(entry.object) {
		return nil, true, false
	}
	body, err := baseGit(root, limit, "cat-file", "blob", entry.object)
	return body, true, err == nil
}

// baseWorkspace is workspaceDirectories over the base tree: the directories a
// root go.work lists, or the root alone. It reads go.work with the planner's
// strict manifest grammar (readManifest in
// internal/liveverify/affected/golang/nested.go): a raw string, a block
// comment, an invalid quoted string, an unknown or quoted directive, a block
// of a verb that admits none, text after a block close, an unterminated block,
// or a go, toolchain, godebug, use or replace directive whose arguments the
// planner refuses (AFP-V0-403) refuses the file with a WorkspaceRefusal, and
// the base is unknown, where the planner observes the root alone and raises
// its workspace frontier (V1-2317).
func baseWorkspace(root string, entries map[string]baseTreeEntry) (map[string]bool, error) {
	body, present, ok := baseBlob(root, entries, workspaceManifest, maxWorkspaceBytes)
	if !ok {
		return nil, &WorkspaceRefusal{Reason: WorkspaceUnreadable}
	}
	return parseBaseWorkspace(body, present)
}

// parseBaseWorkspace reads one base go.work body for baseWorkspace; an absent
// file lists the root alone.
func parseBaseWorkspace(body []byte, present bool) (map[string]bool, error) {
	listed := map[string]bool{}
	seen := map[string]bool{}
	block := ""
	for number, line := range strings.Split(string(body), "\n") {
		refuse := func(reason string) (map[string]bool, error) {
			return nil, &WorkspaceRefusal{Line: number + 1, Reason: reason}
		}
		tokens, quoted, ok := workspaceTokens(line)
		if !ok {
			return refuse(WorkspaceLexical)
		}
		if len(tokens) == 0 {
			continue
		}
		verb, args, argsQuoted := block, tokens, quoted
		switch {
		case block != "" && tokens[0] == ")" && !quoted[0]:
			if len(tokens) != 1 {
				return refuse(WorkspaceBlock)
			}
			block = ""
			continue
		case block == "":
			if quoted[0] || !workspaceVerbs[tokens[0]] {
				return refuse(WorkspaceUnknownDirective)
			}
			verb, args, argsQuoted = tokens[0], tokens[1:], quoted[1:]
			if len(args) == 1 && args[0] == "(" && !argsQuoted[0] {
				if !workspaceBlocks[verb] {
					return refuse(WorkspaceBlock)
				}
				block = verb
				continue
			}
		}
		if reason := workspaceDirectiveReason(verb, args, argsQuoted, seen); reason != "" {
			return refuse(reason)
		}
		if verb != "use" {
			continue
		}
		cleaned := path.Clean(filepath.ToSlash(args[0]))
		if cleaned == "." || validRelativePath(cleaned) {
			listed[cleaned] = true
		}
	}
	if block != "" {
		return nil, &WorkspaceRefusal{Reason: WorkspaceUnterminated}
	}
	if !present || len(listed) == 0 {
		listed["."] = true
	}
	return listed, nil
}

// workspaceVerbs and workspaceBlocks mirror the planner's workVerbs and
// workBlocks.
var (
	workspaceVerbs  = map[string]bool{"go": true, "toolchain": true, "godebug": true, "use": true, "replace": true}
	workspaceBlocks = map[string]bool{"godebug": true, "use": true, "replace": true}
)

// workspaceTokens is the planner's manifestLine lexer: it splits one go.work
// line into tokens, reporting which were quoted, and refuses what that lexer
// refuses.
func workspaceTokens(line string) ([]string, []bool, bool) {
	var tokens []string
	var quoted []bool
	for index := 0; index < len(line); {
		character, size := utf8.DecodeRuneInString(line[index:])
		switch {
		case character == ' ' || character == '\t' || character == '\r':
			index += size
		case strings.HasPrefix(line[index:], "//"):
			return tokens, quoted, true
		case strings.HasPrefix(line[index:], "/*"), character == '`':
			return nil, nil, false
		case strings.ContainsRune(workspacePunctuation, character):
			tokens, quoted = append(tokens, string(character)), append(quoted, false)
			index += size
		case character == '"':
			end := index + 1
			for end < len(line) && line[end] != '"' {
				if line[end] == '\\' {
					end++
				}
				end++
			}
			if end >= len(line) {
				return nil, nil, false
			}
			value, err := strconv.Unquote(line[index : end+1])
			if err != nil {
				return nil, nil, false
			}
			tokens, quoted = append(tokens, value), append(quoted, true)
			index = end + 1
		case !workspaceIdentifierRune(character, size):
			return nil, nil, false
		default:
			end := index
			for end < len(line) && !strings.HasPrefix(line[end:], "//") {
				next, width := utf8.DecodeRuneInString(line[end:])
				if !workspaceIdentifierRune(next, width) || strings.ContainsRune(workspacePunctuation, next) {
					break
				}
				if strings.HasPrefix(line[end:], "/*") {
					return nil, nil, false
				}
				end += width
			}
			tokens, quoted = append(tokens, line[index:end]), append(quoted, false)
			index = end
		}
	}
	return tokens, quoted, true
}

// workspacePunctuation is the planner's manifestPunctuation.
const workspacePunctuation = "()[]{},"

// workspaceIdentifierRune is the planner's identifierRune: printable, not
// space, and valid UTF-8.
func workspaceIdentifierRune(character rune, size int) bool {
	return !(character == utf8.RuneError && size == 1) && !unicode.IsSpace(character) && unicode.IsPrint(character)
}

// baseOwner is committedOwner over the base tree: the nearest listed module
// at or above directory. A regular go.mod below a listed module's testdata is
// fixture data and owns nothing; any other go.mod that is not listed, or that
// is not a regular file, owns its subtree for a module the observer drops.
func baseOwner(entries map[string]baseTreeEntry, listed map[string]bool, directory string) (string, bool) {
	for {
		if listed[directory] {
			return directory, true
		}
		// A directory named go.mod is no manifest: the observer collects files.
		if entry, found := entries[path.Join(directory, moduleManifestName)]; found && entry.mode != treeMode &&
			(!regularMode(entry.mode) || !baseFixtureModule(listed, directory)) {
			return "", false
		}
		if directory == "." {
			return "", false
		}
		directory = path.Dir(directory)
	}
}

// baseFixtureModule is fixtureOfObservedModule: directory lies below a
// testdata directory of the nearest listed module above it.
func baseFixtureModule(listed map[string]bool, directory string) bool {
	for owner := path.Dir(directory); ; owner = path.Dir(owner) {
		if listed[owner] {
			return inTestdataComponent(relativeTo(directory, owner))
		}
		if owner == "." {
			return false
		}
	}
}

// walkedDirectory reports whether the observer's walk descends into
// directory: no component is hidden or skipped.
func walkedDirectory(directory string) bool {
	for _, name := range strings.Split(directory, "/") {
		if strings.HasPrefix(name, ".") || skippedDirectories[name] {
			return false
		}
	}
	return true
}

func inTestdataComponent(relative string) bool {
	return slices.Contains(strings.Split(relative, "/"), "testdata")
}

func relativeTo(directory, moduleDirectory string) string {
	if moduleDirectory == "." {
		return directory
	}
	if directory == moduleDirectory {
		return "."
	}
	return strings.TrimPrefix(directory, moduleDirectory+"/")
}

func regularMode(mode string) bool { return mode == regularFileMode || mode == executableFileMode }

// baseGit runs one read-only git plumbing command in root with the hermetic
// configuration cmd/corvint uses (no system or global config, no replace
// objects, no lazy fetch), bounded by limit bytes of output and a deadline.
func baseGit(root string, limit int64, arguments ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), baseGitDeadline)
	defer cancel()
	prefix := []string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "credential.helper=",
		"-c", "submodule.recurse=false", "-c", "core.hooksPath=/dev/null", "-C", root}
	command := exec.CommandContext(ctx, "git", append(prefix, arguments...)...)
	command.Env = []string{
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GIT_ALLOW_PROTOCOL=", "LANG=C", "LC_ALL=C",
	}
	if _, err := os.Lstat(filepath.Join(root, ".git")); err == nil {
		command.Env = append(command.Env, "GIT_CEILING_DIRECTORIES="+filepath.Dir(root))
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(io.LimitReader(stdout, limit+1))
	if int64(len(body)) > limit {
		cancel()
	}
	waitErr := command.Wait()
	switch {
	case int64(len(body)) > limit:
		return nil, fmt.Errorf("git %s output exceeds %d bytes", arguments[0], limit)
	case readErr != nil:
		return nil, readErr
	case waitErr != nil:
		return nil, waitErr
	}
	return body, nil
}
