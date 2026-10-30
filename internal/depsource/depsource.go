// Package depsource answers one read-only question: what are the pinned bytes
// of a third-party Go module this repository depends on, and do they match the
// checksum the committed go.sum records?
//
// The package never downloads, never writes to the module cache, and never
// reads the dirty worktree for evidence. When the pin cannot be proven it
// abstains with an explicit reason instead of returning unverified content
// (AGENTS.md invariants 1, 2, 4 and 7).
package depsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Error is a depsource failure. An abstention is not an Error: it is a
// successful answer whose Verified member is false.
type Error struct{ Message string }

func (err *Error) Error() string { return err.Message }

const (
	// DefaultLimit bounds the file listing when no --limit is given.
	DefaultLimit = 200
	// maxExcerptBytes bounds one file's emitted content.
	maxExcerptBytes = 64 << 10
	// maxZipHashBytes bounds the cache's small textual checksum record.
	maxZipHashBytes = 1 << 10
	maxSumBytes     = 4 << 20
)

// Options is one depsource request. ModuleCache is injectable so a test can
// point at a fixture cache without mutating process environment.
type Options struct {
	Root        string
	Module      string
	Version     string // Empty means "whatever the committed go.mod pins".
	File        string
	Limit       int
	ModuleCache string
}

// FileEntry is one bounded listing row.
type FileEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Blob string `json:"blob,omitempty"`
}

// FileExcerpt is one file's pinned content, capped at 64 KiB.
type FileExcerpt struct {
	Path      string   `json:"path"`
	SHA256    string   `json:"sha256"`
	Size      int64    `json:"size"`
	Blob      string   `json:"blob,omitempty"`
	Truncated bool     `json:"truncated"`
	Lines     []string `json:"lines"`
}

// Result is the emitted answer. Field order is the emitted member order.
type Result struct {
	Module       string       `json:"module"`
	Version      string       `json:"version"`
	Revision     string       `json:"revision"`
	Resolution   string       `json:"resolution"`
	GoSumHash    string       `json:"go_sum_hash"`
	ComputedHash string       `json:"computed_hash,omitempty"`
	Verified     bool         `json:"verified"`
	Reason       string       `json:"reason,omitempty"`
	FileCount    int          `json:"file_count"`
	Truncated    bool         `json:"truncated"`
	Files        []FileEntry  `json:"files,omitempty"`
	File         *FileExcerpt `json:"file,omitempty"`
}

// Render resolves one module and writes a single canonical JSON line.
func Render(ctx context.Context, options Options, stdout io.Writer) error {
	result, err := Resolve(ctx, options)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return nil
}

// Resolve produces the answer without emitting it.
func Resolve(ctx context.Context, options Options) (Result, error) {
	if options.Module == "" {
		return Result{}, &Error{Message: "module path is required"}
	}
	if options.Limit < 1 {
		options.Limit = DefaultLimit
	}
	revision, err := committedRevision(ctx, options.Root)
	if err != nil {
		return Result{}, err
	}
	goMod, err := showBlob(ctx, options.Root, revision, "go.mod")
	if err != nil {
		return Result{}, &Error{Message: "committed go.mod is unreadable: " + err.Error()}
	}
	result := Result{Module: options.Module, Version: options.Version, Revision: revision, Resolution: "require"}
	parsed, parseErr := parseGoMod(goMod)
	if parseErr != nil {
		return abstain(result, "unparsable-go-mod"), nil
	}
	version, versionErr := parsed.requiredVersion(options.Module)
	if versionErr != nil {
		return abstain(result, "ambiguous-version"), nil
	}
	if version == "" {
		return abstain(result, "module-not-required"), nil
	}
	if options.Version != "" && options.Version != version {
		return abstain(result, "version-mismatch"), nil
	}
	if parsed.excluded(options.Module, version) {
		return abstain(result, "excluded-version"), nil
	}
	result.Version = version

	target := requirement{options.Module, version}
	if replaced, found := parsed.replacementFor(options.Module, version); found {
		if isLocalPath(replaced.NewPath) {
			return localReplace(ctx, options, result, replaced.NewPath)
		}
		result.Resolution = "replace"
		if replaced.NewVersion == "" {
			return abstain(result, "replace-without-version"), nil
		}
		target = requirement{replaced.NewPath, replaced.NewVersion}
		result.Module, result.Version = target.Path, target.Version
	}
	return cacheEvidence(ctx, options, result, target, revision)
}

func abstain(result Result, reason string) Result {
	result.Verified = false
	result.Reason = reason
	return result
}

func isLocalPath(value string) bool {
	return strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") ||
		value == "." || value == ".." || filepath.IsAbs(value)
}

// cacheEvidence verifies the extracted module directory against the committed
// go.sum h1 hash and, when present, the cache's own ziphash record.
func cacheEvidence(ctx context.Context, options Options, result Result, target requirement, revision string) (Result, error) {
	goSum, err := showBlob(ctx, options.Root, revision, "go.sum")
	if err != nil {
		if !blobAbsent(ctx, options.Root, revision, "go.sum") {
			return Result{}, &Error{Message: "committed go.sum is unreadable: " + err.Error()}
		}
		return abstain(result, "missing-go-sum"), nil
	}
	if len(goSum) > maxSumBytes {
		return abstain(result, "go-sum-exceeds-read-limit"), nil
	}
	recorded := sumHash(goSum, target.Path, target.Version)
	if recorded == "" {
		return abstain(result, "missing-go-sum-entry"), nil
	}
	result.GoSumHash = recorded

	cache, err := moduleCache(options)
	if err != nil {
		return abstain(result, "module-cache-unavailable"), nil
	}
	escapedPath, err := escapeModulePath(target.Path)
	if err != nil {
		return abstain(result, "unescapable-module-path"), nil
	}
	escapedVersion, err := escapeModulePath(target.Version)
	if err != nil {
		return abstain(result, "unescapable-module-version"), nil
	}
	directory := filepath.Join(cache, filepath.FromSlash(escapedPath)+"@"+escapedVersion)
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return abstain(result, "missing-cache-entry"), nil
	}

	cleanedFile := ""
	if options.File != "" {
		cleanedFile = cleanModuleFilePath(options.File)
	}
	prefix := target.Path + "@" + target.Version
	computed, files, captured, err := hashDirectory(ctx, directory, prefix, cleanedFile)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Result{}, ctxErr
	}
	if err != nil {
		return abstain(result, "cache-entry-unreadable"), nil
	}
	result.ComputedHash = computed
	if computed != recorded {
		return abstain(result, "hash-mismatch"), nil
	}
	if ziphash, found, overflow := readZipHash(cache, escapedPath, escapedVersion); found && (overflow || ziphash != recorded) {
		return abstain(result, "ziphash-mismatch"), nil
	}
	result.Verified = true

	if options.File != "" {
		return cacheFile(result, cleanedFile, captured), nil
	}
	return listing(result, files, options.Limit), nil
}

func cleanModuleFilePath(requested string) string {
	return path.Clean(strings.TrimPrefix(filepath.ToSlash(requested), "./"))
}

// readZipHash reads the record inside the cache root without following a final
// symlink or blocking on a FIFO. Only an absent record is "not found"; any other
// unopenable or non-regular record is found with an empty value, a mismatch.
func readZipHash(cache, escapedPath, escapedVersion string) (string, bool, bool) {
	root, err := os.OpenRoot(cache)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, false
	}
	if err != nil {
		return "", true, false
	}
	defer root.Close()
	name := filepath.Join("cache", "download", filepath.FromSlash(escapedPath), "@v", escapedVersion+".ziphash")
	// os.Root resolves a symlink that stays inside the root even with O_NOFOLLOW,
	// so the record must be regular before the open and the same file after it.
	before, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, false
	}
	if err != nil {
		return "", true, false
	}
	if !before.Mode().IsRegular() {
		return "", true, false
	}
	file, err := root.OpenFile(name, cacheFileOpenFlags, 0)
	if err != nil {
		return "", true, false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", true, false
	}
	if !os.SameFile(before, info) {
		return "", true, false
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxZipHashBytes+1))
	if err != nil {
		return "", true, false
	}
	overflow := len(raw) > maxZipHashBytes
	if overflow {
		raw = raw[:maxZipHashBytes]
	}
	return strings.TrimSpace(string(raw)), true, overflow
}

func listing(result Result, files []moduleFile, requested int) Result {
	result.FileCount = len(files)
	limit := min(requested, len(files))
	result.Truncated = limit < len(files)
	result.Files = make([]FileEntry, 0, limit)
	for _, file := range files[:limit] {
		result.Files = append(result.Files, FileEntry{Path: file.Rel, Size: file.Size})
	}
	return result
}

// cacheFile builds the excerpt from content captured off the same handle that
// fed the directory hash (see hashDirectory), never by reopening the cache
// path: a nil captured content means the requested file matched none of the hashed files.
func cacheFile(result Result, cleanedFile string, captured *fileCapture) Result {
	if captured == nil {
		return abstain(result, "file-not-in-module")
	}
	result.FileCount = 1
	result.File = makeExcerpt(cleanedFile, "", captured.sha256, captured.size, captured.content,
		captured.size > int64(len(captured.content)))
	result.Truncated = result.File.Truncated
	return result
}

func excerpt(relative, blob string, content []byte) *FileExcerpt {
	digest := sha256.Sum256(content)
	truncated := len(content) > maxExcerptBytes
	body := content
	if truncated {
		body = body[:maxExcerptBytes]
	}
	return makeExcerpt(relative, blob, hex.EncodeToString(digest[:]), int64(len(content)), body, truncated)
}

func makeExcerpt(relative, blob, digest string, size int64, body []byte, truncated bool) *FileExcerpt {
	lines := strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
	return &FileExcerpt{
		Path:      relative,
		SHA256:    digest,
		Size:      size,
		Blob:      blob,
		Truncated: truncated,
		Lines:     lines,
	}
}

// localReplace pins a directory replacement by Git blob hash, and only when
// the replaced directory is inside this repository's committed tree. A target
// outside the tree has no immutable pin available, so the verb abstains.
func localReplace(ctx context.Context, options Options, result Result, target string) (Result, error) {
	result.Resolution = "local-replace"
	absolute := target
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(options.Root, target)
	}
	relative, err := filepath.Rel(options.Root, filepath.Clean(absolute))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return abstain(result, "replace-outside-repository"), nil
	}
	entries, err := treeBlobs(ctx, options.Root, result.Revision, filepath.ToSlash(relative))
	if err != nil || len(entries) == 0 {
		return abstain(result, "replace-path-not-committed"), nil
	}
	// A blob at the target path itself is a committed file or symlink, not a
	// module directory; its lone entry would read as a verified module listing.
	if entries[0].Path == filepath.ToSlash(relative) {
		return abstain(result, "replace-path-not-committed"), nil
	}
	result.Verified = true
	if options.File != "" {
		return localReplaceFile(ctx, options, result, filepath.ToSlash(relative), entries)
	}
	result.FileCount = len(entries)
	limit := min(options.Limit, len(entries))
	result.Truncated = limit < len(entries)
	result.Files = make([]FileEntry, 0, limit)
	for _, entry := range entries[:limit] {
		result.Files = append(result.Files, FileEntry{
			Path: strings.TrimPrefix(entry.Path, filepath.ToSlash(relative)+"/"),
			Size: entry.Size,
			Blob: entry.OID,
		})
	}
	return result, nil
}

func localReplaceFile(ctx context.Context, options Options, result Result, base string, entries []treeBlob) (Result, error) {
	wanted := path.Join(base, path.Clean(strings.TrimPrefix(filepath.ToSlash(options.File), "./")))
	for _, entry := range entries {
		if entry.Path != wanted {
			continue
		}
		content, err := showBlob(ctx, options.Root, result.Revision, entry.Path)
		if err != nil {
			return abstain(result, "file-unreadable"), nil
		}
		result.FileCount = 1
		result.File = excerpt(strings.TrimPrefix(entry.Path, base+"/"), entry.OID, content)
		result.Truncated = result.File.Truncated
		return result, nil
	}
	return abstain(result, "file-not-in-module"), nil
}

func treeBlobs(ctx context.Context, root, revision, relative string) ([]treeBlob, error) {
	raw, err := git(ctx, root, "ls-tree", "-r", "-l", "-z", revision, "--", relative)
	if err != nil {
		return nil, err
	}
	entries := []treeBlob{}
	for _, record := range strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00") {
		if record == "" {
			continue
		}
		metadata, name, found := strings.Cut(record, "\t")
		if !found {
			continue
		}
		fields := strings.Fields(metadata)
		if len(fields) != 4 || fields[1] != "blob" {
			continue
		}
		size, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			size = 0
		}
		entries = append(entries, treeBlob{Path: name, OID: fields[2], Size: size})
	}
	return entries, nil
}

// moduleCache resolves the read-only cache root. An injected path wins so a
// test never mutates process environment; otherwise GOMODCACHE, then the
// toolchain's own answer.
func moduleCache(options Options) (string, error) {
	if options.ModuleCache != "" {
		return options.ModuleCache, nil
	}
	if value := os.Getenv("GOMODCACHE"); value != "" {
		return value, nil
	}
	command := exec.Command("go", "env", "GOMODCACHE")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod", "GOPROXY=off")
	raw, err := command.Output()
	if err != nil {
		return "", &Error{Message: "cannot resolve GOMODCACHE"}
	}
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "", &Error{Message: "GOMODCACHE is empty"}
	}
	return value, nil
}
