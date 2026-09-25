package contextindex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/gitstatus"
)

const (
	gitDeadline       = 30 * time.Second
	maxGitErrorBytes  = 64 * 1024
	maxIdentityBytes  = 4 * 1024
	maxStatusBytes    = 8 * 1024 * 1024
	maxTreeBytes      = 64 * 1024 * 1024
	maxBatchBytes     = 128 * 1024 * 1024
	maxDirtyPaths     = 100_000
	maxSourceBytes    = 1_000_000
	maxIndexedSources = 200_000
)

// Error is a stable read-command failure. Ordinary Python-compatible domain and
// Git errors omit a code; native admission and isolation failures carry one.
// Cause is the original structured error this Error was rebuilt from, when a
// caller outside this package flattened one into Code/Message; it carries no
// Unwrap method, so it never changes what errors.As/errors.Is find elsewhere
// in a chain through an Error, and a caller that wants the original back
// (e.g. tracerecordrepo.TruncatedAncestryOf) reads Cause explicitly.
type Error struct {
	Code, Message string
	Cause         error
	// ReasonClass is the closed status refusal class of decision 0383, set
	// only on a status refusal.
	ReasonClass string
	gitFailure  *GitFailure
}

func (err *Error) Error() string { return err.Message }

// GitFailure retains non-rendered subprocess details for command-scoped parity adapters.
type GitFailure struct {
	Arguments  []string
	Stderr     []byte
	ExitCode   int
	StartError error
}

// GitFailureDetails returns a defensive copy without changing Error's public bytes.
func GitFailureDetails(err error) (GitFailure, bool) {
	var failure *Error
	if !errors.As(err, &failure) || failure.gitFailure == nil {
		return GitFailure{}, false
	}
	details := *failure.gitFailure
	details.Arguments = append([]string(nil), details.Arguments...)
	details.Stderr = append([]byte(nil), details.Stderr...)
	return details, true
}

// boundedBuffer caps how many bytes one Git subprocess may return.
//
// The buffer is a named field and is never embedded. Embedding bytes.Buffer
// promotes ReadFrom, WriteString, WriteByte and WriteRune onto this type, and
// os/exec collects a non-*os.File stdout with io.Copy, which prefers an
// io.ReaderFrom destination over the Write below. An embedded buffer therefore
// bypasses the cap completely and never sets exceeded, leaving every declared
// limit inert against a hostile or very large repository.
type boundedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (buffer *boundedBuffer) Write(value []byte) (int, error) {
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining <= 0 {
		buffer.exceeded = true
		return len(value), nil
	}
	if len(value) > remaining {
		_, _ = buffer.buffer.Write(value[:remaining])
		buffer.exceeded = true
		return len(value), nil
	}
	return buffer.buffer.Write(value)
}

// ReadFrom is the cap-enforcing replacement for the method an embedded
// bytes.Buffer would have promoted. os/exec collects a non-*os.File stdout with
// io.Copy, which prefers an io.ReaderFrom destination; without this method
// io.Copy falls back to a freshly allocated 32 KiB staging buffer per Git
// subprocess and grows the destination from zero. Reading through a LimitReader
// keeps every declared limit enforced, and the remainder is drained rather than
// abandoned so the child never blocks writing into a full pipe.
func (buffer *boundedBuffer) ReadFrom(reader io.Reader) (int64, error) {
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining < 0 {
		remaining = 0
	}
	stored, err := buffer.buffer.ReadFrom(io.LimitReader(reader, int64(remaining)))
	if err != nil {
		return stored, err
	}
	discarded, err := io.Copy(io.Discard, reader)
	if discarded > 0 {
		buffer.exceeded = true
	}
	return stored + discarded, err
}

func (buffer *boundedBuffer) Grow(size int) { buffer.buffer.Grow(size) }

func (buffer *boundedBuffer) Bytes() []byte { return buffer.buffer.Bytes() }

func (buffer *boundedBuffer) String() string { return buffer.buffer.String() }

func sanitizedGitEnvironment() []string {
	environment := make([]string, 0, 16)
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment,
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GCM_INTERACTIVE=never", "GIT_ASKPASS=",
	)
}

func git(ctx context.Context, root string, outputLimit int, stdin []byte, arguments ...string) ([]byte, error) {
	return gitExpecting(ctx, root, outputLimit, 0, stdin, arguments...)
}

// gitExpecting is git for a caller that already knows how many bytes the
// command will emit. bytes.Buffer grows geometrically, so an unhinted read of
// an N-byte batch allocates about 2N; one hinted Grow allocates N once.
func gitExpecting(ctx context.Context, root string, outputLimit, expected int, stdin []byte, arguments ...string) ([]byte, error) {
	if len(arguments) > 0 && arguments[0] == "status" {
		run := func(ctx context.Context, root string, limit int, args ...string) ([]byte, error) {
			return gitRaw(ctx, root, limit, 0, nil, args...)
		}
		temporaryParent := os.TempDir()
		if execution, qualified := ctx.Value(gitExecutionKey{}).(gitExecution); qualified {
			temporaryParent = "/tmp" // The Unix default, independent of ambient TMPDIR.
			for _, entry := range execution.environment {
				if value, found := strings.CutPrefix(entry, "TMPDIR="); found {
					temporaryParent = value
					if temporaryParent == "" {
						temporaryParent = "/tmp"
					}
				}
			}
		}
		result, err := gitstatus.StatusIn(ctx, root, temporaryParent, outputLimit, run, arguments...)
		if err != nil {
			if ctx.Err() != nil {
				return nil, contextError(ctx)
			}
			var failure *Error
			if errors.As(err, &failure) {
				var probe *gitstatus.MetadataProbeError
				if failure.Code == "" && errors.As(err, &probe) {
					classified := *failure
					classified.Code = "repository-probe-failed"
					return nil, &classified
				}
				return nil, err
			}
			return nil, &Error{Code: "repository-probe-failed", Message: gitstatus.RefusalMessage(err), ReasonClass: gitstatus.RefusalClass(err)}
		}
		return result, nil
	}
	return gitRaw(ctx, root, outputLimit, expected, stdin, arguments...)
}

func gitRaw(ctx context.Context, root string, outputLimit, expected int, stdin []byte, arguments ...string) ([]byte, error) {
	commandArguments := []string{
		"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
		"-c", "core.excludesFile=", "-c", "credential.helper=", "-c", "submodule.recurse=false",
		"-C", root,
	}
	commandArguments = append(commandArguments, arguments...)
	executable := gitstatus.Executable()
	var environment []string
	if execution, qualified := ctx.Value(gitExecutionKey{}).(gitExecution); qualified {
		executable = execution.executable
		environment = append([]string(nil), execution.environment...)
	} else {
		environment = sanitizedGitEnvironment()
	}
	if gitstatus.Isolated(ctx) {
		environment = append(environment, "GIT_CEILING_DIRECTORIES="+filepath.Dir(root))
	}
	command := exec.CommandContext(ctx, executable, commandArguments...)
	command.Env = environment
	configureProcess(command)
	if stdin != nil {
		command.Stdin = bytes.NewReader(stdin)
	}
	stdout := &boundedBuffer{limit: outputLimit}
	if expected > 0 {
		stdout.Grow(min(expected, outputLimit))
	}
	stderr := &boundedBuffer{limit: maxGitErrorBytes}
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Start(); err != nil {
		if ctx.Err() != nil {
			return nil, contextError(ctx)
		}
		return nil, &Error{
			Message:    "Git error: cannot start Git",
			gitFailure: &GitFailure{Arguments: arguments, ExitCode: -1, StartError: err},
		}
	}
	processID := command.Process.Pid
	defer terminateProcessGroup(processID)
	err := command.Wait()
	if ctx.Err() != nil {
		return nil, contextError(ctx)
	}
	if stdout.exceeded {
		return nil, &Error{Message: "Git output exceeds its byte limit"}
	}
	if stderr.exceeded {
		return nil, &Error{Message: "Git error exceeds its byte limit"}
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		exitCode := -1
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		}
		return nil, &Error{
			Message: "Git error: " + detail,
			gitFailure: &GitFailure{
				Arguments: arguments, Stderr: stderr.Bytes(), ExitCode: exitCode,
			},
		}
	}
	return stdout.Bytes(), nil
}

func contextError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &Error{Message: "Git repository index exceeded its deadline"}
	}
	return &Error{Message: "Git repository index was cancelled"}
}

// repositoryIdentity is the object format, HEAD and its tree. historyCut
// reports a shallow repository or a grafts file: either changes what
// `git log HEAD` returns while HEAD stays put, so no stored walk keyed by the
// commit answers for it (IDX-SNAP-V0-015).
type repositoryIdentity struct {
	objectFormat, commitRevision, treeRevision string
	historyCut                                 bool
}

// readIdentity asks for the grafts path last, so a path holding a newline
// stays one field.
func readIdentity(ctx context.Context, root string) (repositoryIdentity, error) {
	raw, err := git(ctx, root, maxIdentityBytes, nil, "rev-parse", "--show-object-format", "--is-shallow-repository", "HEAD", "HEAD^{tree}", "--git-path", "info/grafts")
	if err != nil {
		return repositoryIdentity{}, classifyHeadFailure(ctx, root, err)
	}
	lines := strings.SplitN(strings.TrimSuffix(string(raw), "\n"), "\n", 5)
	if len(lines) != 5 {
		return repositoryIdentity{}, &Error{Message: "Git object identity is malformed"}
	}
	if lines[1] != "true" && lines[1] != "false" {
		return repositoryIdentity{}, &Error{Message: "Git object identity is malformed"}
	}
	switch lines[0] {
	case "sha1":
	case "sha256":
	default:
		format := lines[0]
		if format == "" {
			format = "<empty>"
		}
		return repositoryIdentity{}, &Error{Message: "unsupported Git object format: " + format}
	}
	if !validObjectID(lines[2], lines[0]) || !validObjectID(lines[3], lines[0]) {
		return repositoryIdentity{}, &Error{Message: "Git returned an invalid object identity"}
	}
	grafts := lines[4]
	if !filepath.IsAbs(grafts) {
		grafts = filepath.Join(root, grafts)
	}
	_, graftsErr := os.Lstat(grafts)
	historyCut := lines[1] == "true" || !errors.Is(graftsErr, os.ErrNotExist)
	return repositoryIdentity{lines[0], lines[2], lines[3], historyCut}, nil
}

func readStatusSnapshot(ctx context.Context, root string) ([]string, string, error) {
	raw, err := git(ctx, root, maxStatusBytes, nil, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, "", err
	}
	paths, err := parseStatus(raw)
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(raw)
	return paths, fmt.Sprintf("%x", digest), nil
}

func parseStatus(raw []byte) ([]string, error) {
	if len(raw) > 0 && raw[len(raw)-1] != 0 {
		return nil, &Error{Message: "Git status output is malformed"}
	}
	paths := make(map[string]struct{})
	fields := bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0})
	if len(raw) == 0 {
		fields = nil
	}
	for index := 0; index < len(fields); index++ {
		field := fields[index]
		if len(field) <= 3 || field[2] != ' ' {
			return nil, &Error{Message: "Git status output is malformed"}
		}
		if !utf8.Valid(field[3:]) {
			return nil, &Error{Message: "Git status path is not valid UTF-8"}
		}
		paths[string(field[3:])] = struct{}{}
		if bytes.ContainsAny(field[:2], "RC") {
			index++
			if index >= len(fields) || len(fields[index]) == 0 {
				return nil, &Error{Message: "Git status output contains an empty path"}
			}
			if !utf8.Valid(fields[index]) {
				return nil, &Error{Message: "Git status path is not valid UTF-8"}
			}
			paths[string(fields[index])] = struct{}{}
		}
		if len(paths) > maxDirtyPaths {
			return nil, &Error{Message: fmt.Sprintf("Git status exceeds the %d-path limit", maxDirtyPaths)}
		}
	}
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}

type treeEntry struct {
	path, oid, mode string
	size            int
}

func readTreeEntries(ctx context.Context, root string, identity repositoryIdentity, skipped ...map[string]struct{}) ([]treeEntry, error) {
	raw, err := git(ctx, root, maxTreeBytes, nil, "ls-tree", "-r", "-l", "-z", "--full-tree", identity.treeRevision)
	if err != nil {
		return nil, err
	}
	// One entry per NUL, so the slice is sized once instead of doubled ~12
	// times over a repository-sized tree.
	entries := make([]treeEntry, 0, bytes.Count(raw, []byte{0}))
	var metadata [4][]byte
	for _, item := range bytes.Split(raw, []byte{0}) {
		if len(item) == 0 {
			continue
		}
		tab := bytes.IndexByte(item, '\t')
		if tab < 0 || !utf8.Valid(item[tab+1:]) {
			return nil, &Error{Message: "Git tree output is malformed"}
		}
		if !splitTreeMetadata(item[:tab], &metadata) {
			return nil, &Error{Message: "Git tree output is malformed"}
		}
		if string(metadata[1]) != "blob" || string(metadata[0]) == "160000" {
			if len(skipped) != 0 {
				skipped[0][string(item[tab+1:])] = struct{}{}
			}
			continue
		}
		oid := string(metadata[2])
		if !validObjectID(oid, identity.objectFormat) {
			return nil, &Error{Message: "Git returned an invalid blob object identity"}
		}
		size, parseErr := strconv.Atoi(string(metadata[3]))
		if parseErr != nil || size < 0 {
			return nil, &Error{Message: "Git returned an invalid blob size"}
		}
		entries = append(entries, treeEntry{string(item[tab+1:]), oid, treeEntryMode(metadata[0]), size})
		if len(entries) > maxIndexedSources {
			return nil, &Error{Message: "Git tree exceeds the source-count limit"}
		}
	}
	return entries, nil
}

// splitTreeMetadata fills fields with the four space-separated columns that
// precede a `ls-tree -l` path, without the string conversion and []string that
// strings.Fields allocated for every entry in the tree. The size column is
// space-padded, and any byte at or below a space separates columns, so a
// malformed line splits exactly where strings.Fields split it.
func splitTreeMetadata(line []byte, fields *[4][]byte) bool {
	count, offset := 0, 0
	for offset < len(line) {
		for offset < len(line) && line[offset] <= ' ' {
			offset++
		}
		if offset == len(line) {
			break
		}
		end := offset
		for end < len(line) && line[end] > ' ' {
			end++
		}
		if count == len(fields) {
			return false
		}
		fields[count] = line[offset:end]
		count++
		offset = end
	}
	return count == len(fields)
}

// treeEntryMode interns the handful of modes a tree actually carries, so the
// per-entry mode costs no allocation.
func treeEntryMode(mode []byte) string {
	switch string(mode) {
	case "100644":
		return "100644"
	case "100755":
		return "100755"
	case "120000":
		return "120000"
	case "040000":
		return "040000"
	case "160000":
		return "160000"
	}
	return string(mode)
}

func validObjectID(value, objectFormat string) bool {
	length := 40
	if objectFormat == "sha256" {
		length = 64
	}
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

// blobBatchEntries counts every entry fetchBlobs has asked Git for. Nothing in
// the product reads it; the pin parity test reads it to prove its two branches
// really are different constructions rather than the same one run twice.
var blobBatchEntries atomic.Int64

func readBlobs(ctx context.Context, root string, entries []treeEntry) (map[string][]byte, error) {
	if err := validateBlobAdmission(entries); err != nil {
		return nil, err
	}
	unique, err := uniqueBlobEntries(entries)
	if err != nil {
		return nil, err
	}
	return fetchBlobs(ctx, root, unique)
}

// readResidualBlobs fetches only needed, while admitting and de-duplicating
// against all. The two checks answer for the whole candidate set because that
// is the set they are about: validateBlobAdmission is the repository's
// aggregate size refusal and uniqueBlobEntries rejects one oid recorded at two
// sizes, and neither verdict may depend on how many candidates the worktree
// happened to pin on its own.
func readResidualBlobs(ctx context.Context, root string, all, needed []treeEntry) (map[string][]byte, error) {
	if err := validateBlobAdmission(all); err != nil {
		return nil, err
	}
	if _, err := uniqueBlobEntries(all); err != nil {
		return nil, err
	}
	unique, err := uniqueBlobEntries(needed)
	if err != nil {
		return nil, err
	}
	return fetchBlobs(ctx, root, unique)
}

// fetchBlobs streams the bodies of already-admitted, already-unique entries.
// An empty set spawns no process: a batch of nothing costs a fork, a pipe pair
// and a wait to parse zero bytes.
func fetchBlobs(ctx context.Context, root string, unique []treeEntry) (map[string][]byte, error) {
	blobBatchEntries.Add(int64(len(unique)))
	if len(unique) == 0 {
		return map[string][]byte{}, nil
	}
	input := blobBatchInput(unique)
	raw, err := gitExpecting(ctx, root, maxBatchBytes, blobBatchOutputSize(unique), input, "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	return parseBlobBatch(raw, unique)
}

// blobBatchOutputSize is the exact byte count parseBlobBatch will consume:
// per blob an "<oid> blob <size>\n" header, the body, and a trailing newline.
func blobBatchOutputSize(entries []treeEntry) int {
	total := 0
	for _, entry := range entries {
		total += len(entry.oid) + len(" blob ") + len(strconv.Itoa(entry.size)) + 1 + entry.size + 1
	}
	return total
}

func blobBatchInput(entries []treeEntry) []byte {
	input := make([]byte, 0, len(entries)*65)
	for _, entry := range entries {
		input = append(input, entry.oid...)
		input = append(input, '\n')
	}
	return input
}

func parseBlobBatch(raw []byte, entries []treeEntry) (map[string][]byte, error) {
	result := make(map[string][]byte, len(entries))
	offset := 0
	for _, entry := range entries {
		lineEnd := bytes.IndexByte(raw[offset:], '\n')
		if lineEnd < 0 {
			return nil, &Error{Message: "Git blob batch output is malformed"}
		}
		lineEnd += offset
		size, valid := parseBlobBatchHeader(raw[offset:lineEnd], entry.oid)
		if !valid {
			return nil, &Error{Message: "Git blob batch output is malformed"}
		}
		if size != entry.size || lineEnd+1+size >= len(raw) {
			return nil, &Error{Message: "Git blob batch output is malformed"}
		}
		start := lineEnd + 1
		if raw[start+size] != '\n' {
			return nil, &Error{Message: "Git blob batch output is malformed"}
		}
		result[entry.oid] = raw[start : start+size : start+size]
		offset = start + size + 1
	}
	if offset != len(raw) {
		return nil, &Error{Message: "Git blob batch output is malformed"}
	}
	return result, nil
}

func uniqueBlobEntries(entries []treeEntry) ([]treeEntry, error) {
	seen := make(map[string]int, len(entries))
	unique := make([]treeEntry, 0, len(entries))
	for _, entry := range entries {
		if index, exists := seen[entry.oid]; exists {
			if unique[index].size != entry.size {
				return nil, &Error{Message: "Git blob batch output is malformed"}
			}
			continue
		}
		seen[entry.oid] = len(unique)
		unique = append(unique, entry)
	}
	return unique, nil
}

func parseBlobBatchHeader(line []byte, oid string) (int, bool) {
	oidLength := len(oid)
	if len(line) <= oidLength+6 || !equalBytesString(line[:oidLength], oid) || line[oidLength] != ' ' ||
		line[oidLength+1] != 'b' || line[oidLength+2] != 'l' || line[oidLength+3] != 'o' || line[oidLength+4] != 'b' || line[oidLength+5] != ' ' {
		return 0, false
	}
	digits := line[oidLength+6:]
	if len(digits) == 0 {
		return 0, false
	}
	size := 0
	for _, digit := range digits {
		if digit < '0' || digit > '9' || size > (maxBatchBytes-int(digit-'0'))/10 {
			return 0, false
		}
		size = size*10 + int(digit-'0')
	}
	return size, true
}

func equalBytesString(value []byte, expected string) bool {
	if len(value) != len(expected) {
		return false
	}
	for index := range value {
		if value[index] != expected[index] {
			return false
		}
	}
	return true
}

func validateBlobAdmission(entries []treeEntry) error {
	total := 0
	for _, entry := range entries {
		const framingAllowance = 128
		if entry.size > maxBatchBytes-framingAllowance || total > maxBatchBytes-entry.size-framingAllowance {
			return &Error{Code: "unsupported-impact-repository", Message: "native Go impact index exceeds the 128 MiB aggregate bound"}
		}
		total += entry.size + framingAllowance
	}
	return nil
}

func readQueryBlobs(ctx context.Context, root string, entries []treeEntry) (map[string][]byte, error) {
	if len(entries) == 0 {
		return map[string][]byte{}, nil
	}
	blobs, err := readBlobs(ctx, root, entries)
	return blobs, queryAdmissionError(err)
}

func validateQueryBlobAdmission(entries []treeEntry) error {
	return queryAdmissionError(validateBlobAdmission(entries))
}

func queryAdmissionError(err error) error {
	var indexErr *Error
	if errors.As(err, &indexErr) && indexErr.Code == "unsupported-impact-repository" {
		return &Error{Code: "unsupported-query-repository", Message: strings.Replace(indexErr.Message, "native Go impact index", "native Go authority-start query index", 1)}
	}
	return err
}

// readStatusSnapshotRaw is readStatusSnapshot that also returns the status
// bytes the digest covers, for range impact's untracked-path classification.
func readStatusSnapshotRaw(ctx context.Context, root string) ([]string, string, []byte, error) {
	raw, err := git(ctx, root, maxStatusBytes, nil, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, "", nil, err
	}
	paths, err := parseStatus(raw)
	if err != nil {
		return nil, "", nil, err
	}
	digest := sha256.Sum256(raw)
	return paths, fmt.Sprintf("%x", digest), raw, nil
}
