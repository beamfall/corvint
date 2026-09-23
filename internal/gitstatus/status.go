// Package gitstatus observes status with frozen configuration and explicit
// repository paths. Git status must never execute repository-owned filters.
package gitstatus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Runner func(context.Context, string, int, ...string) ([]byte, error)

type isolationKey struct{}

// WithIsolation also confines Git's repository discovery at external consumption
// boundaries. Status always uses private metadata, including standalone calls.
func WithIsolation(ctx context.Context) context.Context {
	return context.WithValue(ctx, isolationKey{}, true)
}

func Isolated(ctx context.Context) bool {
	isolated, _ := ctx.Value(isolationKey{}).(bool)
	return isolated
}

type probeReuseKey struct{}

// WithProbeReuse lets a status read answer a metadata probe from an earlier
// successful probe in this process over the same captured bytes (proposed
// GPK-V0-065; opt-in, never the default). Each probe is a pure function of the
// private copy: `config --list` of one config file, and `ls-files --stage`
// plus `rev-parse --shared-index-path` of one index under one config pair, all
// against the same root and Git directory. Only a success is remembered, so a
// failure of any kind is asked again, and the status run itself is never
// reused. The reuse assumes one Git executable for the whole process.
func WithProbeReuse(ctx context.Context) context.Context {
	return context.WithValue(ctx, probeReuseKey{}, true)
}

func probeReuse(ctx context.Context) bool {
	reuse, _ := ctx.Value(probeReuseKey{}).(bool)
	return reuse
}

// validatedProbes holds the digests of the probe inputs that succeeded. Two
// slot holders may consult it at once, so probesMu guards it.
var (
	probesMu        sync.Mutex
	validatedProbes = map[[sha256.Size]byte]struct{}{}
)

// probeMemo is one probe's place in validatedProbes; without reuse it neither
// answers nor records.
type probeMemo struct {
	key   [sha256.Size]byte
	reuse bool
}

func probeDigest(parts ...[]byte) [sha256.Size]byte {
	hash := sha256.New()
	for _, part := range parts {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(part)))
		hash.Write(length[:])
		hash.Write(part)
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}

func (memo probeMemo) known() bool {
	if !memo.reuse {
		return false
	}
	probesMu.Lock()
	defer probesMu.Unlock()
	_, known := validatedProbes[memo.key]
	return known
}

func (memo probeMemo) remember() {
	if memo.reuse {
		probesMu.Lock()
		validatedProbes[memo.key] = struct{}{}
		probesMu.Unlock()
	}
}

const metadataLimit = 32 << 20
const snapshotLimit = 64 << 20

var errUnsafe = errors.New("Git status repository configuration or metadata is not supported safely")

// MetadataProbeError identifies failure while Git validates a private metadata
// copy. Final status execution errors remain unwrapped for caller compatibility.
type MetadataProbeError struct{ cause error }

func (err *MetadataProbeError) Error() string { return err.cause.Error() }
func (err *MetadataProbeError) Unwrap() error { return err.cause }

func metadataProbeError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return &MetadataProbeError{cause: err}
}

// Bound temporary metadata across concurrent MCP calls, not merely per request:
// at most two private copies exist at once, so a query's trace-store and
// learn-bracket observations can overlap while a third waiter still queues.
var isolationSlot = make(chan struct{}, 2)

type capturedFile struct {
	path    string
	data    []byte
	present bool
	modTime time.Time
}

// Status runs only against private metadata. All failures are closed: callers
// must not retry status against the live Git directory.
func Status(ctx context.Context, root string, limit int, run Runner, arguments ...string) ([]byte, error) {
	return StatusIn(ctx, root, os.TempDir(), limit, run, arguments...)
}

// StatusIn preserves a caller's existing scratch ownership instead of consulting
// ambient TMPDIR. The parent must remain outside the observed repository.
func StatusIn(ctx context.Context, root, temporaryParent string, limit int, run Runner, arguments ...string) ([]byte, error) {
	if !Isolated(ctx) {
		ctx = WithIsolation(ctx)
	}
	select {
	case isolationSlot <- struct{}{}:
		defer func() { <-isolationSlot }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, errUnsafe
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, errUnsafe
	}
	reader := metadataReader{}
	defer reader.Close()
	gitdir, common, pointers, err := directories(root, &reader)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(temporaryParent) {
		return nil, errUnsafe
	}
	tempRoot, err := filepath.EvalSymlinks(temporaryParent)
	if err != nil || within(root, tempRoot) || within(gitdir, tempRoot) || within(common, tempRoot) {
		return nil, errUnsafe
	}
	if err := ScratchOutside(ctx, tempRoot, root, gitdir, common); err != nil {
		return nil, err
	}
	temp, err := os.MkdirTemp(tempRoot, "corvint-git-status-")
	if err != nil {
		return nil, errUnsafe
	}
	defer os.RemoveAll(temp)
	private := filepath.Join(temp, "metadata")
	if err := os.Mkdir(private, 0o700); err != nil {
		return nil, errUnsafe
	}
	files := append([]capturedFile(nil), pointers...)
	simpleConfigs, sha1Config := true, false
	reuse := probeReuse(ctx)
	var index []byte
	var configs [][]byte
	capturedBytes := 0
	for _, file := range pointers {
		capturedBytes += len(file.data)
	}
	for _, item := range []struct{ from, name string }{
		{common, "config"}, {gitdir, "config.worktree"}, {gitdir, "HEAD"}, {gitdir, "index"},
		{common, "packed-refs"}, {common, "shallow"}, {common, "info/attributes"}, {common, "info/exclude"},
	} {
		file, readErr := reader.captureLimited(filepath.Join(item.from, filepath.FromSlash(item.name)), snapshotLimit-capturedBytes)
		if readErr != nil {
			return nil, readErr
		}
		capturedBytes += len(file.data)
		files = append(files, file)
		if item.name == "index" {
			index = file.data
		}
		if !file.present {
			continue
		}
		target := filepath.Join(private, filepath.FromSlash(item.name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return nil, errUnsafe
		}
		if err := os.WriteFile(target, file.data, 0o600); err != nil {
			return nil, errUnsafe
		}
		// Git uses the index timestamp to detect racily clean entries. A fresh
		// timestamp on the copy could hide worktree changes or change status
		// between otherwise identical observations.
		if item.name == "index" {
			if err := os.Chtimes(target, file.modTime, file.modTime); err != nil {
				return nil, errUnsafe
			}
			info, err := os.Stat(target)
			if err != nil || !info.ModTime().Equal(file.modTime) {
				return nil, errUnsafe
			}
		}
		if item.name == "config" || item.name == "config.worktree" {
			configs = append(configs, file.data)
			simple, sha1Format := simpleConfig(file.data)
			if item.name == "config" {
				sha1Config = sha1Format
			}
			if simple {
				continue
			}
			simpleConfigs = false
			memo := probeMemo{probeDigest([]byte("config"), []byte(root), []byte(gitdir), []byte(item.name), file.data), reuse}
			if memo.known() {
				continue
			}
			// --file plus --no-includes parses the owned copy without following
			// even a hostile include path or consulting a live repository.
			raw, err := run(ctx, temp, 2<<20, "config", "--file", target, "--no-includes", "--null", "--list")
			if err != nil {
				return nil, metadataProbeError(err)
			}
			if !safeConfig(raw, root, gitdir) {
				return nil, errUnsafe
			}
			// A core.worktree answer is resolved on the live filesystem, not
			// from the bytes, so it is asked again every time.
			if !bytes.Contains(raw, []byte("core.worktree=")) {
				memo.remember()
			}
		}
	}
	for _, name := range []string{"objects", "refs"} {
		source := filepath.Join(common, name)
		info, err := os.Lstat(source)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, errUnsafe
		}
		if err := os.Symlink(source, filepath.Join(private, name)); err != nil {
			return nil, errUnsafe
		}
	}
	// The private config copy may name a core.fsmonitor hook; the command-line
	// override keeps it from executing whatever the caller's runner passes.
	prefix := []string{"--git-dir=" + private, "--work-tree=" + root, "-c", "status.submoduleSummary=false", "-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false"}
	if !(simpleConfigs && sha1Config && simpleIndex(index)) {
		parts := append([][]byte{[]byte("index"), []byte(root), []byte(gitdir)}, configs...)
		memo := probeMemo{probeDigest(append(parts, index)...), reuse}
		if !memo.known() {
			if err := validateIndex(ctx, temp, prefix, run); err != nil {
				return nil, err
			}
			memo.remember()
		}
	}
	if err := reader.unchanged(files); err != nil {
		return nil, err
	}
	statusArgs := append(append([]string(nil), prefix...), arguments...)
	statusArgs = append(statusArgs, "--ignore-submodules=all")
	result, err := run(ctx, temp, limit, statusArgs...)
	if err != nil {
		return nil, err
	}
	if err := reader.unchanged(files); err != nil {
		return nil, err
	}
	return result, nil
}

func validateIndex(ctx context.Context, temp string, prefix []string, run Runner) error {
	// The shared index is absent from private metadata, so a reference to one
	// fails closed before any worktree or submodule traversal.
	raw, err := run(ctx, temp, metadataLimit, append(append([]string(nil), prefix...), "ls-files", "--stage", "-z")...)
	if err != nil {
		return metadataProbeError(err)
	}
	for row := range bytes.SplitSeq(raw, []byte{0}) {
		if bytes.HasPrefix(row, []byte("160000 ")) {
			return errUnsafe
		}
	}
	shared, err := run(ctx, temp, 4096, append(append([]string(nil), prefix...), "rev-parse", "--shared-index-path")...)
	if err != nil {
		return metadataProbeError(err)
	}
	if len(bytes.TrimSpace(shared)) != 0 {
		return errUnsafe
	}
	return nil
}

func safeConfig(raw []byte, root, gitdir string) bool {
	for record := range bytes.SplitSeq(raw, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		if bytes.Count(record, []byte{'\n'}) > 1 {
			return false
		}
		key, value, _ := strings.Cut(string(record), "\n")
		for _, character := range key {
			if character < 0x20 || character == 0x7f {
				return false
			}
		}
		key = strings.ToLower(key)
		if strings.HasPrefix(key, "include.") || strings.HasPrefix(key, "includeif.") {
			return false
		}
		if strings.HasPrefix(key, "filter.") && (strings.HasSuffix(key, ".clean") || strings.HasSuffix(key, ".process")) && value != "" {
			return false
		}
		switch key {
		case "core.attributesfile":
			if value != "" {
				return false
			}
		case "core.worktree":
			path := value
			if !filepath.IsAbs(path) {
				path = filepath.Join(gitdir, path)
			}
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil || resolved != root {
				return false
			}
		case "core.bare":
			value = strings.ToLower(value)
			if value != "false" && value != "no" && value != "off" && value != "0" {
				return false
			}
		case "extensions.refstorage":
			if value != "files" {
				return false
			}
		}
	}
	return true
}

// CommonDirectory resolves root's Git common directory from the `.git` marker
// and `commondir` pointer the way Status does, and spawns no Git process. It
// refuses a `.git` marker that is a symlink and reads each pointer file with a
// bounded no-follow open, but it follows symlinks when it resolves the gitdir
// and commondir paths those pointers name. A plain clone's common directory
// is its `.git`.
func CommonDirectory(root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", errUnsafe
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", errUnsafe
	}
	reader := metadataReader{}
	defer reader.Close()
	_, common, _, err := directories(root, &reader)
	return common, err
}

func directories(root string, reader *metadataReader) (string, string, []capturedFile, error) {
	gitdir := filepath.Join(root, ".git")
	info, err := os.Lstat(gitdir)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return "", "", nil, errUnsafe
	}
	var pointers []capturedFile
	if !info.IsDir() {
		file, err := reader.capture(gitdir)
		if err != nil || !file.present || len(file.data) > 4096 || !bytes.HasPrefix(file.data, []byte("gitdir: ")) {
			return "", "", nil, errUnsafe
		}
		pointers = append(pointers, file)
		// Git strips every trailing CR and LF from the pointer, so a CRLF file is valid.
		gitdir = strings.TrimRight(string(file.data[len("gitdir: "):]), "\r\n")
		if strings.ContainsAny(gitdir, "\r\n\x00") || gitdir == "" {
			return "", "", nil, errUnsafe
		}
		if !filepath.IsAbs(gitdir) {
			gitdir = filepath.Join(root, gitdir)
		}
	}
	gitdir, err = filepath.EvalSymlinks(gitdir)
	if err != nil {
		return "", "", nil, errUnsafe
	}
	common := gitdir
	file, err := reader.capture(filepath.Join(gitdir, "commondir"))
	if err != nil {
		return "", "", nil, err
	}
	pointers = append(pointers, file)
	if file.present {
		if len(file.data) > 4096 {
			return "", "", nil, errUnsafe
		}
		common = strings.TrimRight(string(file.data), "\r\n")
		if strings.ContainsAny(common, "\r\n\x00") || common == "" {
			return "", "", nil, errUnsafe
		}
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitdir, common)
		}
		common, err = filepath.EvalSymlinks(common)
		if err != nil {
			return "", "", nil, errUnsafe
		}
	}
	return gitdir, common, pointers, nil
}

func (reader *metadataReader) capture(path string) (capturedFile, error) {
	return reader.captureLimited(path, metadataLimit)
}

func captureLimited(path string, budget int) (capturedFile, error) {
	reader := metadataReader{}
	defer reader.Close()
	return reader.captureLimited(path, budget)
}

func (reader *metadataReader) captureLimited(path string, budget int) (capturedFile, error) {
	limit := metadataLimit
	switch filepath.Base(path) {
	case "config", "config.worktree":
		limit = 1 << 20
	case ".git", "commondir", "HEAD":
		limit = 4096
	}
	data, present, modTime, err := reader.readRegular(path, min(limit, budget))
	if err != nil {
		return capturedFile{}, errUnsafe
	}
	return capturedFile{path: path, data: data, present: present, modTime: modTime}, nil
}

func (reader *metadataReader) unchanged(files []capturedFile) error {
	for _, before := range files {
		after, err := reader.capture(before.path)
		if err != nil || before.present != after.present || !bytes.Equal(before.data, after.data) || (filepath.Base(before.path) == "index" && !before.modTime.Equal(after.modTime)) {
			return fmt.Errorf("Git status repository metadata changed during observation")
		}
	}
	return reader.unchangedDirectories()
}

func within(parent, path string) bool {
	relative, err := filepath.Rel(parent, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// ScratchOutside refuses an existing, symlink-resolved candidate directory
// that is, or lies under, any of roots. EvalSymlinks can preserve case and
// Unicode aliases, so directory identity is compared throughout the bounded
// ancestry before any private metadata is created.
func ScratchOutside(ctx context.Context, candidate string, roots ...string) error {
	identities := make([]os.FileInfo, len(roots))
	for index, path := range roots {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return errUnsafe
		}
		identities[index] = info
	}
	// Every parent step shortens this resolved absolute path until filesystem
	// root, so path length bounds the walk without refusing valid deep parents.
	for remaining := len(candidate); remaining > 0; remaining-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			return errUnsafe
		}
		for _, identity := range identities {
			if os.SameFile(info, identity) {
				return errUnsafe
			}
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return nil
		}
		candidate = parent
	}
	return errUnsafe
}
