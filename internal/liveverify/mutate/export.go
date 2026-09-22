package mutate

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// workspace is one run's filesystem: the exported revision, which only the
// runner writes and the sandboxed tests may only read; a scratch directory
// recreated before every run so no run inherits another's state; and the
// build cache. The sandbox prefix confines each go test to scratch and cache.
type workspace struct {
	root, export, scratch, cache, moduleCache string
	prefix                                    []string
}

// newWorkspace materializes configuration.revision under a fresh temporary
// directory and binds the sandbox to it. The caller's repository is only ever
// read: git ls-tree and git cat-file stream the tree's objects out and they
// are written here, so no worktree state is touched.
func newWorkspace(ctx context.Context, configuration exportSettings, box sandbox) (workspace, func(), error) {
	root, err := os.MkdirTemp("", "corvint-mutate-")
	if err != nil {
		return workspace{}, nil, fmt.Errorf("mutate: create temporary directory: %w", err)
	}
	remove := func() { _ = os.RemoveAll(root) }
	space, err := layoutWorkspace(root, configuration.cacheDir, box)
	if err != nil {
		remove()
		return workspace{}, nil, err
	}
	space.moduleCache, err = hostModuleCache()
	if err != nil {
		remove()
		return workspace{}, nil, err
	}
	if err := exportInto(ctx, configuration, space.export); err != nil {
		remove()
		return workspace{}, nil, err
	}
	return space, remove, nil
}

func layoutWorkspace(root, cacheDir string, box sandbox) (workspace, error) {
	space := workspace{root: root, export: filepath.Join(root, "export"), scratch: filepath.Join(root, "scratch"), cache: cacheDir}
	if space.cache == "" {
		space.cache = filepath.Join(root, "cache")
	}
	for _, directory := range []string{space.export, space.scratch, space.cache} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return workspace{}, fmt.Errorf("mutate: create %s: %w", directory, err)
		}
	}
	prefix, err := box.confine([]string{space.scratch, space.cache})
	if err != nil {
		return workspace{}, err
	}
	space.prefix = prefix
	return space, nil
}

// resetScratch empties the scratch directory so a run starts from nothing a
// previous run left behind.
func resetScratch(space workspace) error {
	if err := os.RemoveAll(space.scratch); err != nil {
		return fmt.Errorf("mutate: reset scratch: %w", err)
	}
	if err := os.MkdirAll(space.scratch, 0o755); err != nil {
		return fmt.Errorf("mutate: reset scratch: %w", err)
	}
	return nil
}

// hostModuleCache resolves the host's module cache directory using the same
// precedence "go env GOMODCACHE" does (the GOMODCACHE environment variable,
// then the user go/env file, then the GOPATH/pkg/mod default), without
// executing "go". An unsandboxed "go env GOMODCACHE" was found to write the
// go command's telemetry counters under the host's config directory on every
// call, even this read-only one (docs/agent-memory bugs.md, 2026-09-12);
// resolving it in-process keeps the query genuinely read-only.
func hostModuleCache() (string, error) {
	if value := os.Getenv("GOMODCACHE"); value != "" {
		return value, nil
	}
	env := readGoEnvFile()
	if value := env["GOMODCACHE"]; value != "" {
		return value, nil
	}
	path := hostGOPATH(env)
	if path == "" {
		return "", errors.New("mutate: resolve host module cache: neither GOMODCACHE nor GOPATH is set")
	}
	if index := strings.IndexRune(path, filepath.ListSeparator); index >= 0 {
		path = path[:index]
	}
	return filepath.Join(path, "pkg", "mod"), nil
}

// hostGOPATH mirrors the go command's GOPATH precedence: the GOPATH
// environment variable, then the user go/env file, then $HOME/go (unless
// that equals GOROOT, which go also refuses to default to).
func hostGOPATH(env map[string]string) string {
	if value := os.Getenv("GOPATH"); value != "" {
		return value
	}
	if value := env["GOPATH"]; value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	def := filepath.Join(home, "go")
	if filepath.Clean(def) == filepath.Clean(hostGOROOT()) {
		return ""
	}
	return def
}

// hostGOROOT locates the root of the "go" the runner executes the way that go
// finds its own, without running it: the GOROOT variable the run passes
// through, else the first of the "go" on PATH's grandparent directory, before
// and after resolving symlinks, that holds pkg/tool. An unresolvable go yields
// "", which never equals a GOPATH default.
func hostGOROOT() string {
	if value := os.Getenv("GOROOT"); value != "" {
		return value
	}
	executable, err := exec.LookPath("go")
	if err != nil {
		return ""
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		resolved = executable
	}
	for _, candidate := range []string{executable, resolved} {
		root := filepath.Dir(filepath.Dir(candidate))
		if info, statErr := os.Stat(filepath.Join(root, "pkg", "tool")); statErr == nil && info.IsDir() {
			return root
		}
	}
	return ""
}

// readGoEnvFile reads the go command's user configuration file the way "go
// env" does: KEY=VALUE lines, comments and malformed lines ignored, and
// GOENV=off disabling the file entirely. A missing or unreadable file yields
// no entries, matching go's own silent fallback to defaults.
func readGoEnvFile() map[string]string {
	entries := map[string]string{}
	path := os.Getenv("GOENV")
	if path == "off" {
		return entries
	}
	if path == "" {
		dir, err := os.UserConfigDir()
		if err != nil || dir == "" {
			return entries
		}
		path = filepath.Join(dir, "go", "env")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return entries
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" || key[0] < 'A' || key[0] > 'Z' {
			continue
		}
		entries[key] = value
	}
	return entries
}

// exportDeadline bounds the export's git calls together. Sixty seconds is far
// past what streaming one revision of a repository this tool targets takes, so
// a wedged Git (a slow object store, a held lock) surfaces as an export failure
// within the invocation budget instead of holding the run until the caller's
// context ends.
const exportDeadline = 60 * time.Second

// treeFile is one tracked regular file of the revision and where it lands.
type treeFile struct {
	id, target string
	mode       os.FileMode
}

// exportInto writes the revision's tree from raw objects: git ls-tree lists it
// and git cat-file --batch streams its blobs. Neither applies an attribute from
// any source (committed, worktree, or the repository's own info/attributes) or
// a conversion setting such as core.autocrlf, so every file holds the committed
// bytes. The revision follows --end-of-options, so it is never read as an
// option.
func exportInto(ctx context.Context, configuration exportSettings, directory string) error {
	exportCtx, cancel := context.WithTimeout(ctx, exportDeadline)
	defer cancel()
	var files []treeFile
	layout := func(listing *bufio.Reader) (err error) {
		files, err = layoutTree(listing, directory)
		return err
	}
	if err := streamGit(exportCtx, configuration, nil, layout, "ls-tree", "-r", "-z", "--full-tree", "--end-of-options", configuration.revision); err != nil {
		return fmt.Errorf("mutate: unpack %s: %w", configuration.revision, err)
	}
	ids := make([]string, 0, len(files)+1)
	for _, file := range files {
		ids = append(ids, file.id)
	}
	input := strings.NewReader(strings.Join(append(ids, ""), "\n"))
	copyBlobs := func(stream *bufio.Reader) error { return writeBlobs(stream, files) }
	if err := streamGit(exportCtx, configuration, input, copyBlobs, "cat-file", "--batch"); err != nil {
		return fmt.Errorf("mutate: unpack %s: %w", configuration.revision, err)
	}
	return nil
}

// streamGit runs one git command in the repository, hands its stdout to
// consume, and reports consume's failure before git's own.
func streamGit(ctx context.Context, configuration exportSettings, input io.Reader, consume func(*bufio.Reader) error, arguments ...string) error {
	command := exec.CommandContext(ctx, configuration.git, append([]string{"-C", configuration.root, "--no-optional-locks"}, arguments...)...)
	command.Env = sanitizedGitEnvironment()
	command.Stdin = input
	stderr := &boundedBuffer{limit: outputLimit}
	command.Stderr = stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pipe git %s: %w", arguments[0], err)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start git %s: %w", arguments[0], err)
	}
	consumeErr := consume(bufio.NewReader(stdout))
	_, _ = io.Copy(io.Discard, stdout)
	waitErr := command.Wait()
	if consumeErr != nil {
		return consumeErr
	}
	if waitErr != nil {
		return fmt.Errorf("git %s: %w: %s", arguments[0], waitErr, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// layoutTree creates the revision's directories from a NUL-terminated
// ls-tree listing and returns its regular files. A gitlink is the empty
// directory a checkout without that submodule holds. A symlink, a path
// outside the copy, or any other entry the copy does not reproduce is refused
// rather than dropped, so the copy never silently differs from the revision.
func layoutTree(listing *bufio.Reader, directory string) ([]treeFile, error) {
	files := make([]treeFile, 0, 256)
	for {
		line, err := listing.ReadString(0)
		if errors.Is(err, io.EOF) && line == "" {
			return files, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read git ls-tree: %w", err)
		}
		meta, name, _ := strings.Cut(strings.TrimSuffix(line, "\x00"), "\t")
		fields := strings.Fields(meta)
		if len(fields) != 3 {
			return nil, fmt.Errorf("git ls-tree printed %q", line)
		}
		target, ok := safeTarget(directory, name)
		if !ok {
			return nil, fmt.Errorf("revision holds %q, a path outside the copy", name)
		}
		file, err := layoutEntry(fields[0], fields[1], fields[2], target)
		if err != nil {
			return nil, fmt.Errorf("revision holds %s: %w", name, err)
		}
		if file.id != "" {
			files = append(files, file)
		}
	}
}

func layoutEntry(mode, kind, id, target string) (treeFile, error) {
	if kind == "commit" {
		return treeFile{}, os.MkdirAll(target, 0o755)
	}
	if kind != "blob" {
		return treeFile{}, fmt.Errorf("a %s entry the copy does not reproduce", kind)
	}
	if mode == "120000" {
		return treeFile{}, errors.New("a symlink the copy does not reproduce")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return treeFile{}, err
	}
	permissions := os.FileMode(0o644)
	if strings.HasSuffix(mode, "755") {
		permissions = 0o755
	}
	return treeFile{id: id, target: target, mode: permissions}, nil
}

func safeTarget(directory, name string) (string, bool) {
	cleaned := path.Clean(filepath.ToSlash(name))
	if cleaned == "." || cleaned == "/" || path.IsAbs(cleaned) {
		return "", false
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	return filepath.Join(directory, filepath.FromSlash(cleaned)), true
}

// writeBlobs writes each file from git cat-file --batch output, which answers
// the requested ids in order as "<id> blob <size>", the bytes, and a newline.
func writeBlobs(stream *bufio.Reader, files []treeFile) error {
	for _, file := range files {
		header, err := stream.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read git cat-file: %w", err)
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[0] != file.id || fields[1] != "blob" {
			return fmt.Errorf("git cat-file answered %q for %s", strings.TrimSpace(header), file.id)
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return fmt.Errorf("git cat-file answered %q for %s", strings.TrimSpace(header), file.id)
		}
		if err := writeFile(stream, file.target, file.mode, size); err != nil {
			return err
		}
		if terminator, err := stream.ReadByte(); err != nil || terminator != '\n' {
			return fmt.Errorf("git cat-file output for %s is not newline-terminated", file.id)
		}
	}
	return nil
}

// writeFile creates target exclusively: a tree names each path once, so an
// existing file means two tracked paths a case- or normalization-insensitive
// volume folds into one, and the copy would silently lose one of them.
func writeFile(reader io.Reader, target string, mode os.FileMode, size int64) error {
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.CopyN(file, reader, size); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func sanitizedGitEnvironment() []string {
	environment := passthroughEnvironment([]string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"})
	return append(environment,
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GCM_INTERACTIVE=never", "GIT_ASKPASS=",
	)
}

func passthroughEnvironment(names []string) []string {
	environment := make([]string, 0, len(names)+16)
	for _, name := range names {
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	return environment
}

// boundedBuffer keeps at most limit bytes of captured output and silently drops
// the rest, so a runaway test cannot exhaust memory.
type boundedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (bounded *boundedBuffer) Write(chunk []byte) (int, error) {
	remaining := bounded.limit - bounded.buffer.Len()
	if remaining > 0 {
		if len(chunk) < remaining {
			remaining = len(chunk)
		}
		bounded.buffer.Write(chunk[:remaining])
	}
	return len(chunk), nil
}

func (bounded *boundedBuffer) String() string { return bounded.buffer.String() }
