package genesis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

const (
	maxBatchInputBytes = 4 * 1024 * 1024
	layoutOutputBytes  = 8192
	oidOutputBytes     = 129
)

type genesisError string

func (err genesisError) Error() string { return string(err) }

type repository struct {
	root   string
	commit string
	limits Limits
	budget *gitrun.Budget
}

func openRepository(ctx context.Context, root string, limits Limits, revision string) (*repository, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, genesisError("invalid-repository")
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, genesisError("invalid-repository")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return nil, genesisError("invalid-repository")
	}
	repo := &repository{
		root: resolved, limits: limits,
		budget: gitrun.NewBudget(8, limits.TotalTimeout),
	}
	// Batch layout discovery with the first immutable commit resolution. The
	// separate tree lookup still uses that verified commit, never a live ref.
	layoutArgs := []string{"rev-parse", "--path-format=absolute", "--absolute-git-dir", "--git-common-dir", "--show-toplevel"}
	arguments := append([]string(nil), layoutArgs...)
	combined := validRevision(revision)
	outputLimit := layoutOutputBytes
	if combined {
		arguments = append(arguments, "--verify", "--end-of-options", revision+"^{commit}")
		outputLimit += oidOutputBytes
	}
	raw, err := repo.run(ctx, outputLimit, nil, arguments...)
	if err != nil && combined {
		// A failed revision must not hide an invalid repository. Only failure
		// classification pays this extra probe; successful reads keep the
		// original eight-process budget even with private status fallback.
		layout, layoutErr := repo.run(ctx, layoutOutputBytes, nil, layoutArgs...)
		if layoutErr != nil || validateRepositoryLayout(resolved, layout) != nil {
			return nil, genesisError("invalid-repository")
		}
		return nil, err
	}
	if !combined {
		if err != nil || validateRepositoryLayout(resolved, raw) != nil {
			return nil, genesisError("invalid-repository")
		}
		return nil, genesisError("invalid-revision")
	}
	commit, err := decodeLayoutCommit(resolved, raw)
	if err != nil {
		return nil, err
	}
	repo.commit = commit
	return repo, nil
}

func decodeLayoutCommit(root string, raw []byte) (string, error) {
	lines := strings.Split(string(raw), "\n")
	if len(lines) != 5 || lines[4] != "" {
		return "", genesisError("invalid-repository")
	}
	if err := validateRepositoryLayout(root, []byte(strings.Join(lines[:3], "\n")+"\n")); err != nil {
		return "", err
	}
	commit, ok := decodeOID([]byte(lines[3]), 0)
	if !ok || commit != lines[3] {
		return "", genesisError("invalid-commit-object")
	}
	return commit, nil
}

func validateRepositoryLayout(root string, raw []byte) error {
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != 3 || lines[0] == "" || lines[1] == "" || lines[2] == "" {
		return genesisError("invalid-repository")
	}
	gitDirectory, commonDirectory, declaredRoot := lines[0], lines[1], lines[2]
	if !filepath.IsAbs(gitDirectory) || !filepath.IsAbs(commonDirectory) || !filepath.IsAbs(declaredRoot) {
		return genesisError("invalid-repository")
	}
	if filepath.Clean(declaredRoot) != root {
		return genesisError("invalid-repository")
	}
	for _, directory := range []string{gitDirectory, commonDirectory} {
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return genesisError("invalid-repository")
		}
	}
	marker := filepath.Join(root, ".git")
	markerInfo, err := os.Lstat(marker)
	if err != nil || markerInfo.Mode()&os.ModeSymlink != 0 {
		return genesisError("invalid-repository")
	}
	if markerInfo.IsDir() {
		if filepath.Clean(gitDirectory) != marker || filepath.Clean(commonDirectory) != marker {
			return genesisError("invalid-repository")
		}
	} else if markerInfo.Mode().IsRegular() {
		declared, err := gitfileTarget(marker, root)
		if err != nil || declared != filepath.Clean(gitDirectory) {
			return genesisError("invalid-repository")
		}
		back, err := metadataPath(filepath.Join(gitDirectory, "gitdir"), gitDirectory)
		if err != nil || back != marker {
			return genesisError("invalid-repository")
		}
		common, err := metadataPath(filepath.Join(gitDirectory, "commondir"), gitDirectory)
		if err != nil || common != filepath.Clean(commonDirectory) {
			return genesisError("invalid-repository")
		}
	} else {
		return genesisError("invalid-repository")
	}
	objects := filepath.Join(commonDirectory, "objects")
	if err := requireRealDirectory(objects, true); err != nil {
		return err
	}
	for _, child := range []string{"info", "pack"} {
		if err := requireRealDirectory(filepath.Join(objects, child), false); err != nil {
			return err
		}
	}
	if _, err := os.Lstat(filepath.Join(objects, "info", "alternates")); !os.IsNotExist(err) {
		return genesisError("invalid-repository")
	}
	return nil
}

func gitfileTarget(marker, root string) (string, error) {
	data, err := readMetadata(marker)
	if err != nil {
		return "", err
	}
	value, found := strings.CutPrefix(strings.TrimSuffix(string(data), "\n"), "gitdir: ")
	if !found || value == "" || strings.ContainsAny(value, "\n\x00") {
		return "", genesisError("invalid-repository")
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(root, value)
	}
	return filepath.Clean(value), nil
}

func metadataPath(name, base string) (string, error) {
	data, err := readMetadata(name)
	if err != nil {
		return "", err
	}
	value := strings.TrimSuffix(string(data), "\n")
	if value == "" || strings.ContainsAny(value, "\n\x00") {
		return "", genesisError("invalid-repository")
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(base, value)
	}
	return filepath.Clean(value), nil
}

func readMetadata(name string) ([]byte, error) {
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > 4096 {
		return nil, genesisError("invalid-repository")
	}
	return os.ReadFile(name)
}

func requireRealDirectory(name string, required bool) error {
	info, err := os.Lstat(name)
	if os.IsNotExist(err) && !required {
		return nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return genesisError("invalid-repository")
	}
	return nil
}

func (repo *repository) run(ctx context.Context, outputLimit int, input []byte, arguments ...string) ([]byte, error) {
	return repo.runExpecting(ctx, outputLimit, 0, input, arguments...)
}

// runExpecting is run for a caller that already knows how many bytes the
// command will emit. gitrun's stdout buffer grows off nil capacity without a
// hint, so a hinted preallocation avoids the ~5x-payload allocation from Go's
// append growth ladder; sizeHint <= 0 keeps the prior unhinted behavior.
func (repo *repository) runExpecting(ctx context.Context, outputLimit, sizeHint int, input []byte, arguments ...string) ([]byte, error) {
	if len(arguments) > 0 && arguments[0] == "status" {
		run := func(ctx context.Context, root string, limit int, args ...string) ([]byte, error) {
			private := *repo
			private.root = root
			return private.runRaw(ctx, limit, 0, nil, args...)
		}
		raw, err := gitstatus.Status(ctx, repo.root, outputLimit, run, arguments...)
		if err != nil {
			var failure genesisError
			if errors.As(err, &failure) {
				return nil, err
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, genesisError("git-timeout")
			}
			return nil, genesisError("git-read-failed")
		}
		return raw, nil
	}
	return repo.runRaw(ctx, outputLimit, sizeHint, input, arguments...)
}

func (repo *repository) runRaw(ctx context.Context, outputLimit, sizeHint int, input []byte, arguments ...string) ([]byte, error) {
	if outputLimit < 0 || outputLimit > repo.limits.MaxTreeBytes+int(repo.limits.MaxTotalBlobBytes) {
		return nil, genesisError("invalid-git-output-budget")
	}
	if len(input) > maxBatchInputBytes {
		return nil, genesisError("git-input-budget-exceeded")
	}
	args := []string{
		"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
		"-c", "protocol.allow=never", "-c", "protocol.file.allow=never",
		"-c", "fetch.recurseSubmodules=false", "-c", "submodule.recurse=false", "-C", repo.root,
	}
	args = append(args, arguments...)
	raw, err := gitrun.Run(ctx, repo.budget, gitrun.Options{
		Env: sanitizedGitEnvironment(repo.root), Stdin: input,
		StdoutLimit: outputLimit, StdoutSizeHint: sizeHint, PerOpTimeout: repo.limits.GitTimeout,
	}, args...)
	if err == nil {
		return raw, nil
	}
	switch cemcode.CodeOf(err) {
	case cemcode.GitOutputExceeded:
		return nil, genesisError("git-output-budget-exceeded")
	case cemcode.GitTimeout:
		return nil, genesisError("git-timeout")
	case cemcode.GitStartFailed:
		return nil, genesisError("git-unavailable")
	default:
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, genesisError("git-timeout")
		}
		return nil, genesisError("git-read-failed")
	}
}

func sanitizedGitEnvironment(root string) []string {
	environment := make([]string, 0, 20)
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment,
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GIT_ATTR_NOSYSTEM=1", "GCM_INTERACTIVE=Never",
		"GIT_ASKPASS="+os.DevNull, "SSH_ASKPASS="+os.DevNull,
		"GIT_LITERAL_PATHSPECS=1", "GIT_CEILING_DIRECTORIES="+filepath.Dir(root),
	)
}

func (repo *repository) resolve(ctx context.Context, revision string) (string, string, string, error) {
	if !validRevision(revision) {
		return "", "", "", genesisError("invalid-revision")
	}
	commit := repo.commit
	if commit == "" {
		return "", "", "", genesisError("invalid-commit-object")
	}
	rawTree, err := repo.run(ctx, oidOutputBytes, nil,
		"rev-parse", "--verify", "--end-of-options", commit+"^{tree}")
	if err != nil {
		return "", "", "", err
	}
	tree, ok := decodeOID(rawTree, len(commit))
	if !ok {
		return "", "", "", genesisError("invalid-tree-object")
	}
	format := "sha1"
	if len(commit) == 64 {
		format = "sha256"
	}
	return commit, tree, format, nil
}

func validRevision(value string) bool {
	if value == "" || len([]byte(value)) > 256 || strings.HasPrefix(value, "-") {
		return false
	}
	for _, character := range value {
		if character <= 0x1f || character == 0x7f {
			return false
		}
	}
	return utf8.ValidString(value)
}

func decodeOID(raw []byte, width int) (string, bool) {
	value := strings.TrimSpace(string(raw))
	if width == 0 && len(value) != 40 && len(value) != 64 {
		return "", false
	}
	if width != 0 && len(value) != width {
		return "", false
	}
	if _, err := hex.DecodeString(value); err != nil || value != strings.ToLower(value) {
		return "", false
	}
	return value, true
}

func (repo *repository) treeEntries(ctx context.Context, commit, objectFormat string) ([]Entry, int, error) {
	raw, err := repo.run(ctx, repo.limits.MaxTreeBytes, nil,
		"ls-tree", "-r", "-l", "-z", "--full-tree", commit)
	if err != nil {
		return nil, 0, err
	}
	return parseTree(raw, repo.limits.MaxPathBytes, objectFormat, repo.limits.MaxEntries)
}

func parseTree(raw []byte, pathLimit int, objectFormat string, entryLimit int) ([]Entry, int, error) {
	oidWidth := 40
	if objectFormat == "sha256" {
		oidWidth = 64
	}
	entries := make([]Entry, 0, min(entryLimit, bytes.Count(raw, []byte{0})))
	var prior []byte
	tracked := 0
	for cursor := 0; cursor < len(raw); {
		end := bytes.IndexByte(raw[cursor:], 0)
		if end <= 0 {
			return nil, 0, genesisError("malformed-tree-entry")
		}
		end += cursor
		record := raw[cursor:end]
		cursor = end + 1
		pieces := bytes.SplitN(record, []byte{'\t'}, 2)
		if len(pieces) != 2 {
			return nil, 0, genesisError("malformed-tree-entry")
		}
		metadata := bytes.Fields(pieces[0])
		rawPath := pieces[1]
		if len(metadata) != 4 || prior != nil && bytes.Compare(rawPath, prior) <= 0 {
			return nil, 0, genesisError("malformed-tree-entry")
		}
		mode, objectType, oid := string(metadata[0]), string(metadata[1]), string(metadata[2])
		if !validMode(mode) || objectType != "blob" && objectType != "commit" {
			return nil, 0, genesisError("malformed-tree-entry")
		}
		if _, ok := decodeOID([]byte(oid), oidWidth); !ok {
			return nil, 0, genesisError("malformed-tree-entry")
		}
		var size *int64
		if !bytes.Equal(metadata[3], []byte{'-'}) {
			parsed, parseErr := strconv.ParseInt(string(metadata[3]), 10, 64)
			if parseErr != nil || parsed < 0 || parsed > 9_007_199_254_740_991 {
				return nil, 0, genesisError("malformed-tree-entry")
			}
			size = int64PointerCopy(parsed)
		}
		prior = append(prior[:0], rawPath...)
		tracked++
		if len(entries) >= entryLimit {
			continue
		}
		digest := sha256.Sum256(rawPath)
		entries = append(entries, Entry{
			Mode: mode, ObjectType: objectType, OID: oid, Path: safePath(rawPath, pathLimit),
			PathSHA256: hex.EncodeToString(digest[:]), Size: size,
		})
	}
	return entries, tracked, nil
}

func validMode(value string) bool {
	if len(value) != 6 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '7' {
			return false
		}
	}
	return true
}

func safePath(raw []byte, limit int) *string {
	if len(raw) == 0 || len(raw) > limit || !utf8.Valid(raw) {
		return nil
	}
	value := string(raw)
	if strings.HasPrefix(value, "/") || strings.Contains(value, "\\") || path.Clean(value) != value || value == "." {
		return nil
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." || part == "" {
			return nil
		}
	}
	for _, character := range value {
		if character <= 0x1f || character == 0x7f {
			return nil
		}
	}
	return stringPointer(value)
}

type selectedBlob struct {
	oid  string
	size int64
}

// readableBlob reports whether readBlobs requests the entry's blob from Git.
func readableBlob(entry Entry, exclusions []string, limits Limits) bool {
	if entry.Mode != "100644" && entry.Mode != "100755" || entry.ObjectType != "blob" || entry.Size == nil {
		return false
	}
	if entry.Path == nil {
		return false
	}
	if matchesExclusion(*entry.Path, exclusions) {
		return false
	}
	if contains(binaryAssetSuffixes, pathSuffix(*entry.Path)) {
		return false
	}
	return *entry.Size <= limits.MaxBlobBytes
}

func (repo *repository) readBlobs(ctx context.Context, entries []Entry, exclusions []string) (map[string][]byte, map[string]struct{}, error) {
	selected := make([]selectedBlob, 0)
	seen := make(map[string]struct{})
	exhausted := make(map[string]struct{})
	var total int64
	for _, entry := range entries {
		if !readableBlob(entry, exclusions, repo.limits) {
			continue
		}
		if _, duplicate := seen[entry.OID]; duplicate {
			continue
		}
		seen[entry.OID] = struct{}{}
		if total > repo.limits.MaxTotalBlobBytes-*entry.Size {
			exhausted[entry.OID] = struct{}{}
			continue
		}
		selected = append(selected, selectedBlob{entry.OID, *entry.Size})
		total += *entry.Size
	}
	if len(selected) == 0 {
		return map[string][]byte{}, exhausted, nil
	}
	var input bytes.Buffer
	for _, blob := range selected {
		input.WriteString(blob.oid)
		input.WriteByte('\n')
	}
	limit := total + int64(len(selected))*160 + 1
	raw, err := repo.runExpecting(ctx, int(limit), int(limit), input.Bytes(), "cat-file", "--batch")
	if err != nil {
		return nil, nil, err
	}
	result := make(map[string][]byte, len(selected))
	cursor := 0
	for _, expected := range selected {
		lineEnd := bytes.IndexByte(raw[cursor:], '\n')
		if lineEnd < 0 {
			return nil, nil, genesisError("malformed-blob-batch")
		}
		lineEnd += cursor
		fields := bytes.Fields(raw[cursor:lineEnd])
		if len(fields) != 3 || string(fields[0]) != expected.oid || string(fields[1]) != "blob" {
			return nil, nil, genesisError("malformed-blob-batch")
		}
		size, parseErr := strconv.ParseInt(string(fields[2]), 10, 64)
		cursor = lineEnd + 1
		end := cursor + int(size)
		if parseErr != nil || size != expected.size || end >= len(raw) || raw[end] != '\n' {
			return nil, nil, genesisError("malformed-blob-batch")
		}
		result[expected.oid] = append([]byte(nil), raw[cursor:end]...)
		cursor = end + 1
	}
	if cursor != len(raw) {
		return nil, nil, genesisError("malformed-blob-batch")
	}
	return result, exhausted, nil
}

func (repo *repository) dirty(ctx context.Context) (bool, error) {
	raw, err := repo.run(ctx, 1, nil, "status", "--porcelain=v1", "-z", "--untracked-files=normal")
	if err == genesisError("git-output-budget-exceeded") {
		return true, nil
	}
	return len(raw) != 0, err
}

func int64PointerCopy(value int64) *int64 { return &value }
func stringPointer(value string) *string  { return &value }

func defaultLimits() Limits {
	return Limits{
		MaxEntries: 50_000, MaxTreeBytes: 64 * 1024 * 1024,
		MaxBlobBytes: 2 * 1024 * 1024, MaxTotalBlobBytes: 64 * 1024 * 1024,
		MaxPathBytes: 1_024, MaxReceiptBytes: 64 * 1024 * 1024,
		GitTimeout: 30 * time.Second, TotalTimeout: 120 * time.Second,
	}
}
