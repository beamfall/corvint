package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

const (
	fixtureFormat    = "atlas-cli-parity-fixture/0"
	fixtureGitConfig = "[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n"
	// The frozen manifest digests .git/config as macOS git init writes it; git on other hosts
	// omits ignorecase and precomposeunicode, so materialization pins these bytes after init.
	fixtureInitializedGitConfig = fixtureGitConfig + "\tbare = false\n\tlogallrefupdates = true\n\tignorecase = true\n\tprecomposeunicode = true\n"
	maximumGitInputBytes        = 128 << 20
	maximumGitOutputBytes       = 1 << 20
	maximumGitArchiveBytes      = 128 << 20
	gitProcessTimeout           = 30 * time.Second
	gitProcessShutdownTimeout   = 2 * time.Second
)

type fixtureSpec struct {
	Format string `json:"format"`
	// ParentFiles, when present, forms a first commit whose tree these Files then
	// replace. A single-commit fixture cannot exercise any command that compares a
	// base revision against a target, because the two are necessarily equal.
	ParentFiles []fixtureEntry `json:"parentFiles,omitempty"`
	Files       []fixtureEntry `json:"files"`
}

type fixtureEntry struct {
	Path    string `json:"path"`
	Type    string `json:"type"`
	Mode    string `json:"mode"`
	Content string `json:"content,omitempty"`
	Target  string `json:"target,omitempty"`
}

type fixtureMutation struct {
	Path    string `json:"path"`
	Mode    string `json:"mode,omitempty"`
	Content string `json:"content"`
}

type materializedFixture struct {
	Root           string
	CommitRevision string
	TreeRevision   string
	FixtureSHA256  string
}

func loadFixture(fixturesRoot, id string) (fixtureSpec, string, error) {
	if err := validateRelativePath(id); err != nil {
		return fixtureSpec{}, "", fmt.Errorf("fixture id: %w", err)
	}
	if strings.Contains(id, "/") {
		return fixtureSpec{}, "", errors.New("fixture id must be one path component")
	}
	path := filepath.Join(fixturesRoot, id, "fixture.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fixtureSpec{}, "", err
	}
	var spec fixtureSpec
	if err := decodeStrictJSON(raw, &spec); err != nil {
		return fixtureSpec{}, "", fmt.Errorf("decode fixture %q: %w", id, err)
	}
	if err := validateFixture(spec); err != nil {
		return fixtureSpec{}, "", fmt.Errorf("fixture %q: %w", id, err)
	}
	return spec, digest(raw), nil
}

func materializeFixture(ctx context.Context, fixturesRoot, id, destination string) (materializedFixture, error) {
	spec, fixtureSHA256, err := loadFixture(fixturesRoot, id)
	if err != nil {
		return materializedFixture{}, err
	}
	if err := os.Mkdir(destination, 0o755); err != nil {
		return materializedFixture{}, fmt.Errorf("create fixture root: %w", err)
	}
	if err := materializeEntries(destination, spec.Files); err != nil {
		return materializedFixture{}, err
	}
	template, err := os.MkdirTemp(filepath.Dir(destination), ".corvint-empty-git-template-")
	if err != nil {
		return materializedFixture{}, err
	}
	defer os.RemoveAll(template)
	if err := os.WriteFile(filepath.Join(template, "config"), []byte(fixtureGitConfig), 0o666); err != nil {
		return materializedFixture{}, err
	}
	git, err := newSanitizedGit(ctx, destination)
	if err != nil {
		return materializedFixture{}, err
	}
	if _, err := git.run(ctx, "init", "--quiet", "--object-format=sha1", "--initial-branch=main", "--template="+template); err != nil {
		return materializedFixture{}, err
	}
	if err := os.WriteFile(filepath.Join(destination, ".git", "config"), []byte(fixtureInitializedGitConfig), 0o666); err != nil {
		return materializedFixture{}, err
	}
	commit, err := importFixtureCommits(ctx, git, spec)
	if err != nil {
		return materializedFixture{}, err
	}
	tree, err := writeFixtureIndex(filepath.Join(destination, ".git"), commit, spec.Files)
	if err != nil {
		return materializedFixture{}, err
	}
	if err := restoreFixtureReflogs(destination, commit); err != nil {
		return materializedFixture{}, err
	}
	return materializedFixture{
		Root: destination, CommitRevision: commit, TreeRevision: tree, FixtureSHA256: fixtureSHA256,
	}, nil
}

func importFixtureCommits(ctx context.Context, git sanitizedGit, spec fixtureSpec) (string, error) {
	var stream bytes.Buffer
	mark := 1
	if len(spec.ParentFiles) != 0 {
		appendFixtureCommit(&stream, mark, 0, spec.ParentFiles, "atlas cli parity fixture parent\n")
		mark++
	}
	appendFixtureCommit(&stream, mark, mark-1, spec.Files, "atlas cli parity fixture\n")
	fmt.Fprintf(&stream, "get-mark :%d\ndone\n", mark)
	output, err := git.runInput(ctx, stream.Bytes(), "-c", "fastimport.unpackLimit=2147483647", "fast-import", "--quiet", "--depth=0", "--done")
	if err != nil {
		return "", err
	}
	commit := strings.TrimSuffix(string(output), "\n")
	if !lowerHex(commit, 40) {
		return "", fmt.Errorf("git fast-import returned invalid commit %q", output)
	}
	return commit, nil
}

func appendFixtureCommit(stream *bytes.Buffer, mark, parent int, entries []fixtureEntry, message string) {
	fmt.Fprintf(stream, "commit refs/heads/main\nmark :%d\nauthor Atlas Fixture <fixture@atlas.invalid> 946684800 +0000\ncommitter Atlas Fixture <fixture@atlas.invalid> 946684800 +0000\ndata %d\n%s", mark, len(message), message)
	if parent != 0 {
		fmt.Fprintf(stream, "from :%d\n", parent)
	}
	stream.WriteString("deleteall\n")
	for _, entry := range entries {
		if entry.Type == "directory" || entry.Type == "untracked-file" {
			continue
		}
		content := fixtureEntryContent(entry)
		fmt.Fprintf(stream, "M %s inline %s\ndata %d\n", fixtureEntryGitMode(entry), fastImportPath(entry.Path), len(content))
		stream.Write(content)
		stream.WriteByte('\n')
	}
	stream.WriteByte('\n')
}

func fastImportPath(path string) string {
	var quoted strings.Builder
	quoted.WriteByte('"')
	for index := 0; index < len(path); index++ {
		value := path[index]
		if value >= 0x20 && value < 0x7f && value != '"' && value != '\\' {
			quoted.WriteByte(value)
			continue
		}
		fmt.Fprintf(&quoted, "\\%03o", value)
	}
	quoted.WriteByte('"')
	return quoted.String()
}

func restoreFixtureReflogs(root, commit string) error {
	record := []byte(strings.Repeat("0", 40) + " " + commit + " Atlas Fixture <fixture@atlas.invalid> 946684800 +0000\n")
	gitDirectory := filepath.Join(root, ".git")
	for _, path := range []string{filepath.Join(gitDirectory, "logs", "HEAD"), filepath.Join(gitDirectory, "logs", "refs", "heads", "main")} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, record, 0o666); err != nil {
			return err
		}
	}
	return nil
}

// writeFixtureIndex writes, in-process, the exact index bytes that
// `git update-index --add -z --index-info` followed by `git write-tree` produce
// for the tracked entries of a fresh SHA-1 repository: version 2, zero stat
// data, one TREE cache-tree extension and the SHA-1 trailer. fast-import has
// already written every blob and tree as a loose object with Git's own zlib, so
// no object bytes are produced here. The computed root tree must equal the tree
// recorded in the imported commit, otherwise materialization fails closed.
func writeFixtureIndex(gitDirectory, commit string, entries []fixtureEntry) (string, error) {
	tracked := make([]fixtureEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Type == "directory" || entry.Type == "untracked-file" {
			continue
		}
		tracked = append(tracked, entry)
	}
	sort.Slice(tracked, func(i, j int) bool { return tracked[i].Path < tracked[j].Path })
	root := &fixtureTree{children: map[string]*fixtureTree{}}
	var index bytes.Buffer
	index.WriteString("DIRC")
	binary.Write(&index, binary.BigEndian, [2]uint32{2, uint32(len(tracked))})
	for _, entry := range tracked {
		object := gitObjectID("blob", fixtureEntryContent(entry))
		mode, _ := strconv.ParseUint(fixtureEntryGitMode(entry), 8, 32)
		root.add(strings.Split(entry.Path, "/"), fixtureTreeItem{mode: fixtureEntryGitMode(entry), object: object})
		index.Write(make([]byte, 24))
		binary.Write(&index, binary.BigEndian, uint32(mode))
		index.Write(make([]byte, 12))
		index.Write(object[:])
		binary.Write(&index, binary.BigEndian, uint16(min(len(entry.Path), 0xfff)))
		index.WriteString(entry.Path)
		index.Write(make([]byte, 8-(62+len(entry.Path))%8))
	}
	rootTree := root.hash()
	var cacheTree bytes.Buffer
	root.writeCacheTree(&cacheTree, "")
	index.WriteString("TREE")
	binary.Write(&index, binary.BigEndian, uint32(cacheTree.Len()))
	index.Write(cacheTree.Bytes())
	trailer := sha1.Sum(index.Bytes())
	index.Write(trailer[:])
	tree := hex.EncodeToString(rootTree[:])
	committed, err := looseCommitTree(gitDirectory, commit)
	if err != nil {
		return "", err
	}
	if committed != tree {
		return "", fmt.Errorf("fixture index tree %s differs from imported commit tree %s", tree, committed)
	}
	if err := os.WriteFile(filepath.Join(gitDirectory, "index"), index.Bytes(), 0o666); err != nil {
		return "", err
	}
	return tree, nil
}

type fixtureTreeItem struct {
	mode   string
	object [sha1.Size]byte
}

type fixtureTree struct {
	files      map[string]fixtureTreeItem
	children   map[string]*fixtureTree
	entryCount int
	object     [sha1.Size]byte
}

func (tree *fixtureTree) add(components []string, item fixtureTreeItem) {
	tree.entryCount++
	if len(components) == 1 {
		if tree.files == nil {
			tree.files = map[string]fixtureTreeItem{}
		}
		tree.files[components[0]] = item
		return
	}
	child := tree.children[components[0]]
	if child == nil {
		child = &fixtureTree{children: map[string]*fixtureTree{}}
		tree.children[components[0]] = child
	}
	child.add(components[1:], item)
}

// hash stores and returns the tree object identity. Git orders tree entries by
// name with a directory compared as if it ended in "/".
func (tree *fixtureTree) hash() [sha1.Size]byte {
	type named struct {
		key, name string
		item      fixtureTreeItem
	}
	items := make([]named, 0, len(tree.files)+len(tree.children))
	for name, item := range tree.files {
		items = append(items, named{name, name, item})
	}
	for name, child := range tree.children {
		items = append(items, named{name + "/", name, fixtureTreeItem{mode: "40000", object: child.hash()}})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].key < items[j].key })
	var content bytes.Buffer
	for _, item := range items {
		content.WriteString(item.item.mode + " " + item.name + "\x00")
		content.Write(item.item.object[:])
	}
	tree.object = gitObjectID("tree", content.Bytes())
	return tree.object
}

// writeCacheTree emits Git's cache-tree extension body. Git orders subtrees by
// name length first and bytes second, not by tree-entry order.
func (tree *fixtureTree) writeCacheTree(buffer *bytes.Buffer, name string) {
	fmt.Fprintf(buffer, "%s\x00%d %d\n", name, tree.entryCount, len(tree.children))
	buffer.Write(tree.object[:])
	names := make([]string, 0, len(tree.children))
	for child := range tree.children {
		names = append(names, child)
	}
	sort.Slice(names, func(i, j int) bool {
		if len(names[i]) != len(names[j]) {
			return len(names[i]) < len(names[j])
		}
		return names[i] < names[j]
	})
	for _, child := range names {
		tree.children[child].writeCacheTree(buffer, child)
	}
}

func gitObjectID(kind string, content []byte) [sha1.Size]byte {
	hash := sha1.New()
	fmt.Fprintf(hash, "%s %d\x00", kind, len(content))
	hash.Write(content)
	var object [sha1.Size]byte
	hash.Sum(object[:0])
	return object
}

// looseCommitTree reads the tree named by a loose commit object; fast-import
// with an unbounded unpack limit leaves every imported object loose.
func looseCommitTree(gitDirectory, commit string) (string, error) {
	file, err := os.Open(filepath.Join(gitDirectory, "objects", commit[:2], commit[2:]))
	if err != nil {
		return "", fmt.Errorf("read imported commit: %w", err)
	}
	defer file.Close()
	reader, err := zlib.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("inflate imported commit: %w", err)
	}
	defer reader.Close()
	header := make([]byte, 64)
	if _, err := io.ReadFull(reader, header); err != nil {
		return "", fmt.Errorf("inflate imported commit: %w", err)
	}
	headerless := bytes.TrimPrefix(header, []byte("commit "))
	_, body, _ := bytes.Cut(headerless, []byte{0})
	line, _, found := bytes.Cut(body, []byte("\n"))
	tree := string(bytes.TrimPrefix(line, []byte("tree ")))
	if !found {
		return "", fmt.Errorf("imported commit %s has no leading tree header", commit)
	}
	if !lowerHex(tree, 40) {
		return "", fmt.Errorf("imported commit %s has invalid tree %q", commit, tree)
	}
	return tree, nil
}

func fixtureEntryContent(entry fixtureEntry) []byte {
	if entry.Type == "symlink" {
		return []byte(entry.Target)
	}
	return []byte(entry.Content)
}

func fixtureEntryGitMode(entry fixtureEntry) string {
	if entry.Type == "symlink" {
		return "120000"
	}
	if entry.Mode == "0755" {
		return "100755"
	}
	return "100644"
}

func applyFixtureMutations(ctx context.Context, root string, mutations []fixtureMutation) error {
	seen := make(map[string]struct{}, len(mutations))
	git, err := newSanitizedGit(ctx, root)
	if err != nil {
		return err
	}
	for _, mutation := range mutations {
		if err := validateRelativePath(mutation.Path); err != nil {
			return fmt.Errorf("mutation path %q: %w", mutation.Path, err)
		}
		if _, duplicate := seen[mutation.Path]; duplicate {
			return fmt.Errorf("duplicate mutation path %q", mutation.Path)
		}
		seen[mutation.Path] = struct{}{}
		if _, err := git.run(ctx, "ls-files", "--error-unmatch", "--", mutation.Path); err != nil {
			return fmt.Errorf("mutation path %q is not tracked: %w", mutation.Path, err)
		}
		path := filepath.Join(root, filepath.FromSlash(mutation.Path))
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("mutation path %q: %w", mutation.Path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("mutation path %q is not a regular tracked file", mutation.Path)
		}
		mode := info.Mode().Perm()
		if mutation.Mode != "" {
			mode, err = parseMode(mutation.Mode)
			if err != nil {
				return fmt.Errorf("mutation path %q: %w", mutation.Path, err)
			}
		}
		if err := os.WriteFile(path, []byte(mutation.Content), mode); err != nil {
			return fmt.Errorf("mutate %q: %w", mutation.Path, err)
		}
		if err := os.Chmod(path, mode); err != nil {
			return fmt.Errorf("chmod mutation %q: %w", mutation.Path, err)
		}
	}
	return nil
}

func validateFixture(spec fixtureSpec) error {
	if spec.Format != fixtureFormat {
		return fmt.Errorf("format=%q, want %q", spec.Format, fixtureFormat)
	}
	if len(spec.Files) == 0 {
		return errors.New("files must not be empty")
	}
	if err := validateFixtureEntries(spec.Files); err != nil {
		return err
	}
	if len(spec.ParentFiles) == 0 {
		return nil
	}
	return validateFixtureEntries(spec.ParentFiles)
}

func validateFixtureEntries(entries []fixtureEntry) error {
	seen := make(map[string]string, len(entries))
	for _, entry := range entries {
		if err := validateFixtureEntry(entry); err != nil {
			return err
		}
		if _, duplicate := seen[entry.Path]; duplicate {
			return fmt.Errorf("duplicate path %q", entry.Path)
		}
		seen[entry.Path] = entry.Type
	}
	for path := range seen {
		for parent := parentSlash(path); parent != ""; parent = parentSlash(parent) {
			if kind, exists := seen[parent]; exists && kind != "directory" {
				return fmt.Errorf("path %q descends from %s %q", path, kind, parent)
			}
		}
	}
	return nil
}

func validateFixtureEntry(entry fixtureEntry) error {
	if err := validateRelativePath(entry.Path); err != nil {
		return fmt.Errorf("path %q: %w", entry.Path, err)
	}
	for _, productPath := range []string{"cmd/corvint", "internal/contextindex/", "internal/gokernel/", "src/corvint_", "src/context_corvint_"} {
		if strings.HasPrefix(entry.Path, productPath) {
			return fmt.Errorf("path %q is reserved for product source", entry.Path)
		}
	}
	if strings.Contains(entry.Content, "github.com/Beamfall/corvint") {
		return fmt.Errorf("path %q embeds the Corvint product module", entry.Path)
	}
	mode, err := parseMode(entry.Mode)
	if err != nil {
		return fmt.Errorf("path %q: %w", entry.Path, err)
	}
	switch entry.Type {
	case "file", "untracked-file":
		if entry.Target != "" {
			return fmt.Errorf("file %q has target", entry.Path)
		}
		if mode&0o111 != 0 && mode != 0o755 {
			return fmt.Errorf("file %q executable mode must be 0755", entry.Path)
		}
		if mode&0o111 == 0 && mode != 0o644 {
			return fmt.Errorf("file %q non-executable mode must be 0644", entry.Path)
		}
	case "directory":
		if entry.Content != "" || entry.Target != "" || mode != 0o755 {
			return fmt.Errorf("directory %q must have mode 0755 and no content or target", entry.Path)
		}
	case "symlink":
		if entry.Content != "" || mode != 0o777 {
			return fmt.Errorf("symlink %q must have mode 0777 and no content", entry.Path)
		}
		if err := validateSymlinkTarget(entry.Path, entry.Target); err != nil {
			return err
		}
	default:
		return fmt.Errorf("path %q has unsupported type %q", entry.Path, entry.Type)
	}
	return nil
}

func materializeEntries(root string, entries []fixtureEntry) error {
	ordered := append([]fixtureEntry(nil), entries...)
	sort.Slice(ordered, func(i, j int) bool {
		leftDepth := strings.Count(ordered[i].Path, "/")
		rightDepth := strings.Count(ordered[j].Path, "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return ordered[i].Path < ordered[j].Path
	})
	for _, entry := range ordered {
		path := filepath.Join(root, filepath.FromSlash(entry.Path))
		mode, _ := parseMode(entry.Mode)
		if entry.Type == "directory" {
			if err := os.Mkdir(path, mode); err != nil {
				return fmt.Errorf("create directory %q: %w", entry.Path, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create parent for %q: %w", entry.Path, err)
		}
		if entry.Type == "symlink" {
			if err := os.Symlink(filepath.FromSlash(entry.Target), path); err != nil {
				return fmt.Errorf("create symlink %q: %w", entry.Path, err)
			}
			continue
		}
		if err := os.WriteFile(path, []byte(entry.Content), mode); err != nil {
			return fmt.Errorf("create file %q: %w", entry.Path, err)
		}
		if err := os.Chmod(path, mode); err != nil {
			return fmt.Errorf("chmod file %q: %w", entry.Path, err)
		}
	}
	return nil
}

func validateRelativePath(value string) error {
	if value == "" || value == "." {
		return errors.New("path must not be empty")
	}
	if strings.IndexByte(value, 0) >= 0 {
		return errors.New("NUL is not permitted")
	}
	if strings.Contains(value, "\\") {
		return errors.New("backslash is not permitted")
	}
	if filepath.IsAbs(value) || strings.HasPrefix(value, "/") {
		return errors.New("absolute path is not permitted")
	}
	if filepath.ToSlash(filepath.Clean(filepath.FromSlash(value))) != value {
		return errors.New("path is not normalized")
	}
	for _, component := range strings.Split(value, "/") {
		if component == ".." || strings.EqualFold(component, ".git") {
			return fmt.Errorf("component %q is not permitted", component)
		}
		if strings.Contains(component, ":") {
			return fmt.Errorf("component %q is not portable", component)
		}
	}
	return nil
}

func validateSymlinkTarget(path, target string) error {
	if target == "" || strings.IndexByte(target, 0) >= 0 || strings.Contains(target, "\\") || filepath.IsAbs(target) {
		return fmt.Errorf("symlink %q has unsafe target %q", path, target)
	}
	for _, component := range strings.Split(target, "/") {
		if strings.Contains(component, ":") {
			return fmt.Errorf("symlink %q has non-portable target %q", path, target)
		}
	}
	resolved := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.FromSlash(parentSlash(path)), filepath.FromSlash(target))))
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return fmt.Errorf("symlink %q escapes fixture root", path)
	}
	if err := validateRelativePath(resolved); err != nil {
		return fmt.Errorf("symlink %q has unsafe target %q: %w", path, target, err)
	}
	return nil
}

func parseMode(value string) (fs.FileMode, error) {
	if len(value) != 4 || value[0] != '0' {
		return 0, fmt.Errorf("mode %q must be four octal digits", value)
	}
	parsed, err := strconv.ParseUint(value, 8, 12)
	if err != nil || parsed > 0o777 {
		return 0, fmt.Errorf("invalid mode %q", value)
	}
	return fs.FileMode(parsed), nil
}

func parentSlash(path string) string {
	index := strings.LastIndexByte(path, '/')
	if index < 0 {
		return ""
	}
	return path[:index]
}

type sanitizedGit struct {
	executable string
	dir        string
	env        []string
}

func newSanitizedGit(ctx context.Context, dir string) (sanitizedGit, error) {
	if ctx == nil {
		return sanitizedGit{}, errors.New("git context must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return sanitizedGit{}, err
	}
	executable, err := exec.LookPath("git")
	if err != nil {
		return sanitizedGit{}, fmt.Errorf("find git: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return sanitizedGit{}, fmt.Errorf("resolve git: %w", err)
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return sanitizedGit{}, fmt.Errorf("resolve git directory: %w", err)
	}
	allow := []string{"PATH", "SystemRoot", "WINDIR", "COMSPEC", "PATHEXT", "TMPDIR", "TMP", "TEMP"}
	environment := make([]string, 0, len(allow)+15)
	for _, key := range allow {
		if value, exists := os.LookupEnv(key); exists {
			environment = append(environment, key+"="+value)
		}
	}
	emptyHome := filepath.Join(dir, ".corvint-git-home")
	environment = append(environment,
		"HOME="+emptyHome,
		"XDG_CONFIG_HOME="+emptyHome,
		"LC_ALL=C", "LANG=C", "TZ=UTC",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_ATTR_NOSYSTEM=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1",
		"GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=Atlas Fixture", "GIT_AUTHOR_EMAIL=fixture@atlas.invalid",
		"GIT_COMMITTER_NAME=Atlas Fixture", "GIT_COMMITTER_EMAIL=fixture@atlas.invalid",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000",
		"GIT_CONFIG_COUNT=3",
		"GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0="+os.DevNull,
		"GIT_CONFIG_KEY_1=core.attributesFile", "GIT_CONFIG_VALUE_1="+os.DevNull,
		"GIT_CONFIG_KEY_2=protocol.file.allow", "GIT_CONFIG_VALUE_2=never",
	)
	return sanitizedGit{executable: filepath.Clean(executable), dir: filepath.Clean(dir), env: environment}, nil
}

func (git sanitizedGit) run(ctx context.Context, arguments ...string) ([]byte, error) {
	return git.runInput(ctx, nil, arguments...)
}

func (git sanitizedGit) runInput(ctx context.Context, input []byte, arguments ...string) ([]byte, error) {
	return git.runBounded(ctx, input, maximumGitOutputBytes, arguments...)
}

func (git sanitizedGit) runArchive(ctx context.Context, arguments ...string) ([]byte, error) {
	return git.runBounded(ctx, nil, maximumGitArchiveBytes, arguments...)
}

func (git sanitizedGit) runBounded(ctx context.Context, input []byte, outputLimit int, arguments ...string) ([]byte, error) {
	observation := procgroup.Run(ctx, procgroup.Spec{
		Argv: append([]string{git.executable}, arguments...), Dir: git.dir, Env: git.env, Stdin: input,
		Timeout: gitProcessTimeout, ShutdownTimeout: gitProcessShutdownTimeout,
		InputLimit: maximumGitInputBytes, OutputLimit: outputLimit,
	})
	operation := "git " + strings.Join(arguments, " ")
	if observation.Err != nil {
		return nil, fmt.Errorf("%s: %w: stderr=%q", operation, observation.Err, observation.Stderr)
	}
	if observation.ExitStatus != 0 {
		return nil, fmt.Errorf("%s: exit status %d: stderr=%q", operation, observation.ExitStatus, observation.Stderr)
	}
	if !observation.Started || !observation.WaitCompleted || !observation.PipesDrained || !observation.OwnedProcessGroupCleanup {
		return nil, fmt.Errorf("%s: incomplete process cleanup", operation)
	}
	return observation.Stdout, nil
}

func (git sanitizedGit) string(ctx context.Context, arguments ...string) (string, error) {
	return git.stringInput(ctx, nil, arguments...)
}

func (git sanitizedGit) stringInput(ctx context.Context, input []byte, arguments ...string) (string, error) {
	output, err := git.runInput(ctx, input, arguments...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func digest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
