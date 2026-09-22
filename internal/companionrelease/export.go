package companionrelease

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Export bounds, per the reviewed bundle plan: bounded 128 MiB total across
// at most 10,000 regular-file entries. These are refusals, not truncation —
// exceeding either aborts the export rather than silently dropping entries.
const (
	maxExportEntries    = 10_000
	maxExportTotalBytes = 128 << 20
)

// SourceFile is one exported, hash-verified regular file from a single Git
// commit tree.
type SourceFile struct {
	Path string // repository-relative, forward-slash, no leading "./"
	Mode string // "100644" or "100755" only
	OID  string // the blob object id ls-tree reported
	Data []byte
}

// Export is the exact, hash-verified regular-file content of one commit's
// tree, read twice by independent code paths that had to agree.
type Export struct {
	Root       string
	HeadCommit string
	HeadTree   string
	Files      []SourceFile // sorted by Path
	TotalBytes int64
}

// findExportFile looks up path (repository-relative, forward-slash) among
// export's hash-verified files. Files is sorted by Path, so this is a
// binary search rather than a scan.
func findExportFile(export Export, path string) (SourceFile, bool) {
	i := sort.Search(len(export.Files), func(i int) bool { return export.Files[i].Path >= path })
	if i < len(export.Files) && export.Files[i].Path == path {
		return export.Files[i], true
	}
	return SourceFile{}, false
}

// exportSource reads root's HEAD tree via `git ls-tree -rz` for the entry
// list and two independent `git cat-file --batch` passes (forward and
// reverse object order, each parsed by a separate routine below) for blob
// bytes. It refuses anything that is not a plain regular-file entry:
// symlinks, submodules, `.git` paths, path traversal, and duplicate paths
// are all hard failures, never silently skipped.
func exportSource(ctx context.Context, gitPath, root, scratchParent string) (Export, error) {
	env := closedGitEnv(scratchParent)
	git := func(timeout time.Duration, stdin []byte, args ...string) ([]byte, error) {
		observation := runWithStdin(ctx, root, env, timeout, stdin, append([]string{gitPath}, args...)...)
		return observation.Stdout, observation.err
	}

	headCommit, err := gitRevParse(git, "HEAD")
	if err != nil {
		return Export{}, fmt.Errorf("resolve HEAD: %w", err)
	}
	headTree, err := gitRevParse(git, "HEAD^{tree}")
	if err != nil {
		return Export{}, fmt.Errorf("resolve HEAD tree: %w", err)
	}

	entries, err := listTreeEntries(git, headCommit)
	if err != nil {
		return Export{}, err
	}
	if len(entries) > maxExportEntries {
		return Export{}, fmt.Errorf("export entries %d exceed bound %d", len(entries), maxExportEntries)
	}

	forward, err := batchBlobs(git, entries, forwardOrder)
	if err != nil {
		return Export{}, fmt.Errorf("primary export pass: %w", err)
	}
	reverse, err := batchBlobsAlt(git, entries, reverseOrder)
	if err != nil {
		return Export{}, fmt.Errorf("independent export pass: %w", err)
	}

	files := make([]SourceFile, 0, len(entries))
	var total int64
	for _, e := range entries {
		a, ok := forward[e.oid]
		if !ok {
			return Export{}, fmt.Errorf("primary pass missing object %s (%s)", e.oid, e.path)
		}
		b, ok := reverse[e.oid]
		if !ok {
			return Export{}, fmt.Errorf("independent pass missing object %s (%s)", e.oid, e.path)
		}
		if !bytes.Equal(a, b) {
			return Export{}, fmt.Errorf("export passes disagree on %s", e.path)
		}
		if err := verifyBlobDigest(e.oid, a); err != nil {
			return Export{}, fmt.Errorf("%s: %w", e.path, err)
		}
		total += int64(len(a))
		if total > maxExportTotalBytes {
			return Export{}, fmt.Errorf("export total bytes exceed bound %d", maxExportTotalBytes)
		}
		files = append(files, SourceFile{Path: e.path, Mode: e.mode, OID: e.oid, Data: a})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return Export{Root: root, HeadCommit: headCommit, HeadTree: headTree, Files: files, TotalBytes: total}, nil
}

func gitRevParse(git func(time.Duration, []byte, ...string) ([]byte, error), rev string) (string, error) {
	out, err := git(subprocessTimeout, nil, "rev-parse", "--verify", rev)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(out))
	if !looksLikeHex(id) {
		return "", fmt.Errorf("rev-parse %s returned non-hex %q", rev, id)
	}
	return id, nil
}

func looksLikeHex(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

type treeEntry struct {
	mode string
	oid  string
	path string
}

// listTreeEntries parses `git ls-tree -rz` output and refuses every entry
// this bundle is not prepared to ship: non-regular modes, non-blob types,
// `.git`/traversal paths, duplicate paths, and paths that collide once
// case-folded (PUB-V0-012).
func listTreeEntries(git func(time.Duration, []byte, ...string) ([]byte, error), commit string) ([]treeEntry, error) {
	out, err := git(subprocessTimeout, nil, "ls-tree", "-rz", commit)
	if err != nil {
		return nil, fmt.Errorf("ls-tree: %w", err)
	}
	var entries []treeEntry
	seen := make(map[string]struct{})
	for _, record := range splitNul(out) {
		if record == "" {
			continue
		}
		tab := strings.IndexByte(record, '\t')
		if tab < 0 {
			return nil, fmt.Errorf("ls-tree record missing path separator: %q", record)
		}
		header, path := record[:tab], record[tab+1:]
		fields := strings.SplitN(header, " ", 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("ls-tree header malformed: %q", header)
		}
		mode, kind, oid := fields[0], fields[1], fields[2]
		if err := refuseUnsafePath(path); err != nil {
			return nil, err
		}
		if mode != "100644" && mode != "100755" {
			return nil, fmt.Errorf("refusing non-regular entry mode %q at %s", mode, path)
		}
		if kind != "blob" {
			return nil, fmt.Errorf("refusing non-blob entry type %q at %s", kind, path)
		}
		if !looksLikeHex(oid) {
			return nil, fmt.Errorf("malformed object id %q at %s", oid, path)
		}
		if _, dup := seen[path]; dup {
			return nil, fmt.Errorf("duplicate export path %s", path)
		}
		seen[path] = struct{}{}
		entries = append(entries, treeEntry{mode: mode, oid: oid, path: path})
	}
	paths := make([]string, len(entries))
	for i, e := range entries {
		paths[i] = e.path
	}
	if err := refuseCaseFoldCollisions(paths); err != nil {
		return nil, err
	}
	return entries, nil
}

// refuseCaseFoldCollisions refuses any pair of paths a case-insensitive
// filesystem cannot tell apart: two paths equal under Unicode simple case
// folding (strings.EqualFold on the full path), or one file path whose
// case-folded form is a directory prefix of another path — e.g. "A" and
// "a/b.go". Left unrefused, materializing such a set to disk (fixtures.go's
// materializeExport, or extracting a shipped archive) collapses both onto
// one file, so this must run before any byte from the set is written.
//
// This is an O(n^2) scan; exportSource's maxExportEntries bound (10,000)
// caps the pair count, and EqualFold on short repository-relative paths is
// cheap, so this stays well under the export's own subprocess timeouts.
func refuseCaseFoldCollisions(paths []string) error {
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			a, b := paths[i], paths[j]
			if strings.EqualFold(a, b) {
				return fmt.Errorf("case-colliding export paths %q and %q", a, b)
			}
			if caseFoldedDirPrefix(a, b) || caseFoldedDirPrefix(b, a) {
				return fmt.Errorf("export path %q case-folds equal to a directory prefix of %q", a, b)
			}
		}
	}
	return nil
}

// caseFoldedDirPrefix reports whether file names, under Unicode simple case
// folding, the same location as a leading directory segment of other — for
// example file "A" and other "a/b.go".
func caseFoldedDirPrefix(file, other string) bool {
	if len(other) <= len(file) {
		return false
	}
	return other[len(file)] == '/' && strings.EqualFold(file, other[:len(file)])
}

func refuseUnsafePath(path string) error {
	if path == "" {
		return errors.New("empty export path")
	}
	if strings.HasPrefix(path, "/") || strings.Contains(path, "\\") {
		return fmt.Errorf("refusing unsafe export path %q", path)
	}
	for _, segment := range strings.Split(path, "/") {
		switch segment {
		case "", ".", "..":
			return fmt.Errorf("refusing export path with traversal segment: %q", path)
		}
		// Case-folded: on a case-insensitive filesystem `.GIT` is `.git`.
		if strings.EqualFold(segment, ".git") {
			return fmt.Errorf("refusing export path under .git: %q", path)
		}
	}
	return nil
}

func splitNul(data []byte) []string {
	parts := bytes.Split(data, []byte{0})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, string(p))
	}
	return out
}

type batchOrder int

const (
	forwardOrder batchOrder = iota
	reverseOrder
)

func orderedOIDs(entries []treeEntry, order batchOrder) []string {
	oids := make([]string, len(entries))
	for i, e := range entries {
		oids[i] = e.oid
	}
	if order == reverseOrder {
		for i, j := 0, len(oids)-1; i < j; i, j = i+1, j-1 {
			oids[i], oids[j] = oids[j], oids[i]
		}
	}
	return oids
}

// batchBlobs is the primary export pass: one `git cat-file --batch` process
// fed OIDs in ls-tree order, parsed with bufio.Reader against the documented
// "<oid> <type> <size>\n<content>\n" framing.
func batchBlobs(git func(time.Duration, []byte, ...string) ([]byte, error), entries []treeEntry, order batchOrder) (map[string][]byte, error) {
	oids := orderedOIDs(entries, order)
	stdin := []byte(strings.Join(oids, "\n") + "\n")
	out, err := git(120*time.Second, stdin, "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReaderSize(bytes.NewReader(out), 1<<20)
	result := make(map[string][]byte, len(entries))
	for range oids {
		header, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("batch header: %w", err)
		}
		oid, kind, size, err := parseBatchHeader(strings.TrimRight(header, "\n"))
		if err != nil {
			return nil, err
		}
		if kind != "blob" {
			return nil, fmt.Errorf("cat-file reported non-blob %q for %s", kind, oid)
		}
		body := make([]byte, size)
		if _, err := io.ReadFull(reader, body); err != nil {
			return nil, fmt.Errorf("batch body for %s: %w", oid, err)
		}
		if trailing, err := reader.ReadByte(); err != nil || trailing != '\n' {
			return nil, fmt.Errorf("batch body for %s missing trailing newline", oid)
		}
		result[oid] = body
	}
	if _, err := reader.ReadByte(); err != io.EOF {
		return nil, fmt.Errorf("batch: unexpected data after the last of %d objects", len(oids))
	}
	return result, nil
}

func parseBatchHeader(line string) (oid, kind string, size int64, err error) {
	fields := strings.SplitN(line, " ", 3)
	if len(fields) != 3 {
		return "", "", 0, fmt.Errorf("malformed batch header %q", line)
	}
	if !looksLikeHex(fields[0]) {
		return "", "", 0, fmt.Errorf("malformed batch header oid %q", fields[0])
	}
	size, convErr := strconv.ParseInt(fields[2], 10, 64)
	if convErr != nil || size < 0 || size > maxExportTotalBytes {
		return "", "", 0, fmt.Errorf("malformed or out-of-bound batch size %q", fields[2])
	}
	return fields[0], fields[1], size, nil
}

// batchBlobsAlt is the independent export pass: a separate `git cat-file
// --batch` invocation (object order reversed) parsed by manual byte
// scanning instead of bufio.Reader, so the two passes share neither process
// output nor decoding logic.
func batchBlobsAlt(git func(time.Duration, []byte, ...string) ([]byte, error), entries []treeEntry, order batchOrder) (map[string][]byte, error) {
	oids := orderedOIDs(entries, order)
	stdin := []byte(strings.Join(oids, "\n") + "\n")
	out, err := git(120*time.Second, stdin, "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	result := make(map[string][]byte, len(entries))
	pos := 0
	for range oids {
		nl := bytes.IndexByte(out[pos:], '\n')
		if nl < 0 {
			return nil, errors.New("independent batch: header truncated")
		}
		header := string(out[pos : pos+nl])
		pos += nl + 1
		oid, kind, size, err := parseBatchHeader(header)
		if err != nil {
			return nil, err
		}
		if kind != "blob" {
			return nil, fmt.Errorf("independent pass reported non-blob %q for %s", kind, oid)
		}
		if int64(len(out)-pos) < size+1 {
			return nil, fmt.Errorf("independent batch: body truncated for %s", oid)
		}
		body := out[pos : pos+int(size)]
		pos += int(size)
		if out[pos] != '\n' {
			return nil, fmt.Errorf("independent batch: missing trailing newline for %s", oid)
		}
		pos++
		result[oid] = append([]byte(nil), body...)
	}
	if pos != len(out) {
		return nil, fmt.Errorf("independent batch: unexpected data after the last of %d objects", len(oids))
	}
	return result, nil
}

// verifyBlobDigest recomputes the Git object id of data under the "blob"
// framing and requires it to match oid exactly, using SHA-1 or SHA-256
// depending on the id's length (a SHA-256 object database reports 64 hex
// characters; SHA-1 reports 40).
func verifyBlobDigest(oid string, data []byte) error {
	frame := fmt.Sprintf("blob %d\x00", len(data))
	var got string
	switch len(oid) {
	case 40:
		h := sha1.New()
		_, _ = io.WriteString(h, frame)
		h.Write(data)
		got = fmt.Sprintf("%x", h.Sum(nil))
	case 64:
		h := sha256.New()
		_, _ = io.WriteString(h, frame)
		h.Write(data)
		got = fmt.Sprintf("%x", h.Sum(nil))
	default:
		return fmt.Errorf("unrecognized object id length for %q", oid)
	}
	if got != oid {
		return fmt.Errorf("blob digest mismatch: declared %s, computed %s", oid, got)
	}
	return nil
}

// stdinObservation adapts procgroup's Observation to the small (stdout,err)
// shape exportSource's git closure needs.
type stdinObservation struct {
	Stdout []byte
	err    error
}

func runWithStdin(ctx context.Context, dir string, env []string, timeout time.Duration, stdin []byte, argv ...string) stdinObservation {
	stdout, stderr, err := runCapturedStdin(ctx, dir, env, timeout, stdin, argv...)
	if err != nil {
		return stdinObservation{Stdout: stdout, err: fmt.Errorf("%v: %w (stderr=%s)", argv, err, trimForError(stderr))}
	}
	return stdinObservation{Stdout: stdout}
}
