package gitauth

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// pinnedArgs is the frozen Git argument prefix: every operation is pinned to
// the resolved administrative directory with hostile configuration overridden.
func (r *Repository) pinnedArgs() []string {
	return []string{
		"--no-optional-locks", "--git-dir=" + r.GitDir,
		"-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
		"-c", "core.attributesFile=" + os.DevNull, "-c", "credential.helper=",
		"-c", "core.quotePath=false", "-c", "diff.suppressBlankEmpty=false",
		"-c", "diff.orderFile=" + os.DevNull, "-c", "diff.noprefix=false",
		"-c", "diff.mnemonicPrefix=false", "-c", "diff.renames=false",
		"-c", "diff.algorithm=myers", "-c", "diff.wsErrorHighlight=none",
		"-c", "diff.srcPrefix=a/", "-c", "diff.dstPrefix=b/",
		"-c", "diff.external=", "-c", "diff.ignoreSubmodules=none",
		"-c", "advice.graftFileDeprecated=false",
	}
}

// scrubbedEnv is the frozen allowlist environment for Git children.
func scrubbedEnv() []string {
	result := make([]string, 0, 17)
	for _, key := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, ok := os.LookupEnv(key); ok {
			result = append(result, key+"="+value)
		}
	}
	return append(result,
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0",
		"GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE="+os.DevNull, "GIT_ASKPASS=",
		"GIT_ATTR_NOSYSTEM=1", "GCM_INTERACTIVE=never",
	)
}

func (r *Repository) git(ctx context.Context, limit int, args ...string) ([]byte, error) {
	return r.gitInput(ctx, limit, nil, args...)
}

func (r *Repository) gitStdin(ctx context.Context, limit int, stdin []byte, args ...string) ([]byte, error) {
	return r.gitInput(ctx, limit, stdin, args...)
}

func (r *Repository) gitInput(ctx context.Context, limit int, stdin []byte, args ...string) ([]byte, error) {
	return gitrun.Run(ctx, r.budget, r.gitOptions(limit, stdin), append(r.pinnedArgs(), args...)...)
}

func (r *Repository) gitOptions(limit int, stdin []byte) gitrun.Options {
	env := scrubbedEnv()
	if r.objectView != nil {
		env = append(env, "GIT_OBJECT_DIRECTORY="+filepath.Join(r.objectView.CommonDir, "objects"), "GIT_ALTERNATE_OBJECT_DIRECTORIES=")
	}
	return gitrun.Options{Dir: r.Root, Env: env, Stdin: stdin, StdoutLimit: limit}
}

// BeginObjectSession scopes one `git cat-file --batch` co-process to the
// caller's pass. Inside the scope, tree and blob reads named by a full object
// ID share it; release closes it. A nested scope shares the outer one. Every
// other read, and every read outside a scope, keeps its own one-shot child.
func (r *Repository) BeginObjectSession() (release func()) {
	if r.objects != nil {
		return func() {}
	}
	session := &gitrun.Session{}
	r.objects = session
	return func() {
		session.Close()
		r.objects = nil
	}
}

// batchStdout names which one-shot stdout a session answer reproduces.
type batchStdout int

const (
	batchRecordOutput batchStdout = iota // `cat-file --batch`: header, body, LF per request
	batchBody                            // `cat-file <type> <oid>`: the bare body
	batchName                            // `rev-parse --verify`: the resolved object ID and LF
)

// A batchName answer is served only when the record's printed bytes hash to
// its object ID: a live child may resolve a name from objects it parsed
// earlier, where a one-shot `rev-parse` re-reads and re-verifies them.

// sessionRead answers one content-addressed invocation under exactly one
// operation charge, as the one-shot child it replaces would. Inside a scope the
// session serves every request, within one per-operation deadline, when admit
// accepts each record's header fields and body size and the reproduced stdout
// stays within limit. Otherwise the one-shot argv replays under the same
// reservation and classifies the failure itself. Outside a scope, or for a
// request not named by a full object ID, the one-shot child runs as before.
func (r *Repository) sessionRead(ctx context.Context, requests []string, stdout batchStdout, admit func(fields []string, size int) bool, limit int, stdin []byte, args ...string) ([]byte, error) {
	if r.objects == nil || slices.ContainsFunc(requests, lacksObjectID) {
		return r.gitInput(ctx, limit, stdin, args...)
	}
	perOp, err := r.budget.ReserveOperation(0)
	if err != nil {
		return nil, err
	}
	options := r.gitOptions(limit, stdin)
	batch := append(r.pinnedArgs(), "cat-file", "--batch")
	deadline := time.Now().Add(perOp)
	var out []byte
	for _, request := range requests {
		fits := func(header string, fields []string, size int) bool {
			return admit(fields, size) && len(out)+stdoutLength(stdout, header, fields, size) <= limit
		}
		header, body, ok, err := r.objects.Read(ctx, time.Until(deadline), options, batch, request, fits)
		if err != nil {
			return nil, err
		}
		if !ok || !servedBytesVerified(ctx, stdout, header, body) {
			return gitrun.RunReserved(ctx, perOp, options, append(r.pinnedArgs(), args...)...)
		}
		out = appendStdout(out, stdout, header, body)
	}
	return out, nil
}

// lacksObjectID reports a batch request not named by a full object ID; only
// ID-named requests use the session, so no scoped answer depends on a ref.
func lacksObjectID(request string) bool {
	name, _ := strings.CutSuffix(request, "^{tree}")
	name, _ = strings.CutSuffix(name, "^{commit}")
	return !wire.IsGitOid(name)
}

func servedBytesVerified(ctx context.Context, stdout batchStdout, header string, body []byte) bool {
	if stdout != batchName {
		return true
	}
	fields := strings.Fields(header)
	return requireObjectIdentity(ctx, fields[1], fields[0], body) == nil
}

func stdoutLength(stdout batchStdout, header string, fields []string, size int) int {
	switch stdout {
	case batchBody:
		return size
	case batchName:
		return len(fields[0]) + 1
	}
	return len(header) + size + 2
}

func appendStdout(out []byte, stdout batchStdout, header string, body []byte) []byte {
	switch stdout {
	case batchBody:
		return append(out, body...)
	case batchName:
		name, _, _ := strings.Cut(header, " ")
		return append(append(out, name...), '\n')
	}
	out = append(append(out, header...), '\n')
	return append(append(out, body...), '\n')
}

// validRevisionText bounds a caller-supplied revision expression before it
// reaches Git argv.
func validRevisionText(revision string) bool {
	if len(revision) == 0 || len(revision) > 256 || strings.HasPrefix(revision, "-") {
		return false
	}
	for _, c := range []byte(revision) {
		if c < 0x21 || c == 0x7f {
			return false
		}
	}
	return true
}

// Resolve resolves one revision expression to its exact full commit OID.
func (r *Repository) Resolve(ctx context.Context, revision string) (string, error) {
	if !validRevisionText(revision) {
		return "", cemcode.New(cemcode.InvalidArguments, "revision must be bounded printable text")
	}
	key, eligible, err := r.requestKey(memoResolve, revision, "")
	if err != nil {
		return "", err
	}
	if eligible {
		value, hit, err := r.recalled(ctx, key)
		if err != nil {
			return "", err
		}
		if hit {
			return value.oid, nil
		}
	}
	request := revision + "^{commit}"
	namedCommit := func(fields []string, size int) bool {
		return fields[0] == revision && fields[1] == "commit" && size <= MaxTreeBytes
	}
	out, err := r.sessionRead(ctx, []string{request}, batchName, namedCommit, 256, nil, "rev-parse", "--verify", "--end-of-options", request)
	if err != nil {
		if cemcode.CodeOf(err) == cemcode.GitExitFailure {
			// The oracle reports one fixed message here and never echoes the
			// caller's revision back.
			return "", cemcode.New(cemcode.GitReadFailed, "Git could not resolve pinned evidence")
		}
		return "", err
	}
	oid := strings.TrimSuffix(string(out), "\n")
	if !wire.IsGitOid(oid) {
		return "", unavailable("revision %q resolved to a malformed object ID", revision)
	}
	if eligible {
		if err := r.remember(ctx, key, memoValue{oid: oid}); err != nil {
			return "", err
		}
	}
	return oid, nil
}

// LoadObjectFormat records the repository's object format and must be called
// once before canonical derivation.
func (r *Repository) LoadObjectFormat(ctx context.Context) error {
	out, err := r.git(ctx, 64, "rev-parse", "--show-object-format")
	if err != nil {
		return err
	}
	format := strings.TrimSuffix(string(out), "\n")
	if format != "sha1" && format != "sha256" {
		return unavailable("unsupported object format %q", format)
	}
	r.ObjectFormat = format
	return nil
}

// Empty tree object IDs are fixed constants of each object format.
const (
	emptyTreeSha1   = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	emptyTreeSha256 = "6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321"
)

// EmptyTreeOID returns the empty tree OID for the repository's object format.
func (r *Repository) EmptyTreeOID() string {
	if r.ObjectFormat == "sha256" {
		return emptyTreeSha256
	}
	return emptyTreeSha1
}

// TreeEntry describes one path at one revision.
type TreeEntry struct {
	Mode string
	Type string
	OID  string
}

// LookupTreeEntry resolves path at revision; exists is false when the path is
// absent from that tree.
func (r *Repository) LookupTreeEntry(ctx context.Context, revision, path string) (TreeEntry, bool, error) {
	if err := wire.ValidatePath(path); err != nil {
		return TreeEntry{}, false, err
	}
	key, eligible, err := r.requestKey(memoLookup, revision, path)
	if err != nil {
		return TreeEntry{}, false, err
	}
	if eligible {
		value, hit, err := r.recalled(ctx, key)
		if err != nil {
			return TreeEntry{}, false, err
		}
		if hit {
			return value.entry, value.exists, nil
		}
	}
	entry, exists, err := r.lookupTreeEntry(ctx, revision, path)
	if err != nil {
		return TreeEntry{}, false, err
	}
	if eligible {
		if err := r.remember(ctx, key, memoValue{entry: entry, exists: exists}); err != nil {
			return TreeEntry{}, false, err
		}
	}
	return entry, exists, nil
}

// lookupTreeEntry resolves path by walking the revision's trees itself. Git's
// own tree reads (ls-tree, rev-parse <rev>:<path>) do not verify a loose or
// packed tree body against its object name, so a nested tree replaced under
// its original OID would silently redirect the path to another object. One
// cat-file batch streams the peeled commit, then returns the root tree and
// every intermediate directory tree. The verified commit pins the root;
// each body is self-hashed and each link is checked against the parent body
// before any entry is trusted, and the leaf comes from the last verified body.
func (r *Repository) lookupTreeEntry(ctx context.Context, revision, path string) (TreeEntry, bool, error) {
	if !validRevisionText(revision) {
		return TreeEntry{}, false, cemcode.New(cemcode.InvalidArguments, "revision must be bounded printable text")
	}
	components := strings.Split(path, "/")
	parents := components[:len(components)-1]
	requests := []string{revision + "^{tree}"}
	for i := range parents {
		requests = append(requests, revision+":"+strings.Join(parents[:i+1], "/"))
	}
	stream, err := r.streamCommitBatch(ctx, []string{revision + "^{commit}"}, requests)
	if err != nil {
		return TreeEntry{}, false, err
	}
	records, err := exactBatchRecords(stream.tail, len(requests))
	if err != nil {
		return TreeEntry{}, false, err
	}
	root := records[0]
	if root.missing || root.kind != "tree" {
		return TreeEntry{}, false, unavailable("revision does not resolve to a tree")
	}
	if stream.width != 0 && len(root.oid) != stream.width {
		return TreeEntry{}, false, unavailable("root tree object format differs")
	}
	if err := requireCommitTree(stream.objects[0], root.oid); err != nil {
		return TreeEntry{}, false, err
	}
	if err := requireObjectIdentity(ctx, "tree", root.oid, root.body); err != nil {
		return TreeEntry{}, false, err
	}
	body := root.body
	width := len(root.oid) / 2
	for i, name := range parents {
		entry, exists, err := findTreeEntry(body, name, width)
		if err != nil {
			return TreeEntry{}, false, err
		}
		child := records[i+1]
		if !exists || entry.Type != "tree" {
			return TreeEntry{}, false, nil
		}
		if child.missing || child.oid != entry.OID || child.kind != "tree" {
			return TreeEntry{}, false, unavailable("tree %s does not resolve locally", entry.OID)
		}
		if err := requireObjectIdentity(ctx, "tree", child.oid, child.body); err != nil {
			return TreeEntry{}, false, err
		}
		body = child.body
	}
	return findTreeEntry(body, components[len(components)-1], width)
}

// batchRecord is one cat-file --batch answer: "<oid> <type> <size>\n<body>\n"
// or "<input> missing\n".
type batchRecord struct {
	oid, kind string
	body      []byte
	missing   bool
}

type batchRecords struct {
	rest []byte
}

func exactBatchRecords(out []byte, count int) ([]batchRecord, error) {
	parser := batchRecords{rest: out}
	result := make([]batchRecord, 0, count)
	for range count {
		record, err := parser.next()
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	if len(parser.rest) != 0 {
		return nil, unavailable("cat-file batch has extra output")
	}
	return result, nil
}

func (b *batchRecords) next() (batchRecord, error) {
	header, rest, found := bytes.Cut(b.rest, []byte{'\n'})
	if !found {
		return batchRecord{}, unavailable("cat-file batch output is truncated")
	}
	// A missing record echoes the request verbatim, spaces included, so it
	// is recognised by its suffix rather than by field count.
	if bytes.HasSuffix(header, []byte(" missing")) {
		b.rest = rest
		return batchRecord{missing: true}, nil
	}
	fields := strings.Fields(string(header))
	if len(fields) != 3 || !wire.IsGitOid(fields[0]) {
		return batchRecord{}, unavailable("cat-file batch output is malformed")
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil || size < 0 || size >= len(rest) || rest[size] != '\n' {
		return batchRecord{}, unavailable("cat-file batch output is malformed")
	}
	b.rest = rest[size+1:]
	return batchRecord{oid: fields[0], kind: fields[1], body: rest[:size]}, nil
}

// findTreeEntry scans one verified tree body for name.
func findTreeEntry(raw []byte, name string, width int) (TreeEntry, bool, error) {
	entries, err := treeEntries(raw, width)
	if err != nil {
		return TreeEntry{}, false, err
	}
	entry, exists := entries[name]
	return entry, exists, nil
}

// nextTreeEntry decodes the first entry of a tree body and returns the rest.
// Entries are "<mode> <name>\0<raw oid>"; the mode is reported zero-padded to
// six octal digits and typed exactly as ls-tree reports it.
func nextTreeEntry(raw []byte, width int) (string, TreeEntry, []byte, error) {
	header, rest, found := bytes.Cut(raw, []byte{0})
	if !found || len(rest) < width {
		return "", TreeEntry{}, nil, unavailable("tree body is malformed")
	}
	mode, name, found := bytes.Cut(header, []byte{' '})
	if !found {
		return "", TreeEntry{}, nil, unavailable("tree body is malformed")
	}
	bits, err := strconv.ParseUint(string(mode), 8, 32)
	if err != nil {
		return "", TreeEntry{}, nil, unavailable("tree body is malformed")
	}
	entry := TreeEntry{Mode: canonicalMode(fmt.Sprintf("%06o", bits)), Type: treeEntryType(bits), OID: hex.EncodeToString(rest[:width])}
	return string(name), entry, rest[width:], nil
}

func treeEntryType(mode uint64) string {
	switch mode & 0o170000 {
	case 0o040000:
		return "tree"
	case 0o160000:
		return "commit"
	default:
		return "blob"
	}
}

// canonicalMode applies Git's own read-time normalization to a raw regular-file
// mode: a legacy entry such as 100664 is 100755 when its owner-execute bit is
// set and 100644 otherwise. Any other raw mode stays verbatim, so a mode Git
// lists differently still disagrees and fails closed (CEM-CB-023).
func canonicalMode(mode string) string {
	permissions, regular := strings.CutPrefix(mode, "100")
	if !regular || len(permissions) != 3 || strings.Trim(permissions, "01234567") != "" {
		return mode
	}
	if (permissions[0]-'0')&1 == 1 {
		return "100755"
	}
	return "100644"
}

// BlobBytes reads one blob through Git object identity with the per-blob and
// cumulative byte bounds. The cumulative budget charges each distinct blob
// exactly once by OID: repeated reads of an already-charged blob add nothing,
// so a verification bounded by distinct blob bytes never overcounts.
func (r *Repository) BlobBytes(ctx context.Context, oid string) ([]byte, error) {
	if !wire.IsGitOid(oid) {
		return nil, cemcode.New(cemcode.InvalidArguments, "blob OID must be a full lowercase Git OID")
	}
	if !r.chargedOids[oid] && r.blobBytes >= MaxTotalBlobBytes {
		return nil, unavailable("verification exceeded its %d-byte blob budget", MaxTotalBlobBytes)
	}
	key, eligible, err := r.requestKey(memoBlob, oid, "")
	if err != nil {
		return nil, err
	}
	var out []byte
	hit := false
	if eligible {
		var value memoValue
		value, hit, err = r.recalled(ctx, key)
		if err != nil {
			return nil, err
		}
		out = value.data
	}
	if !hit {
		namedBlob := func(fields []string, _ int) bool { return fields[0] == oid && fields[1] == "blob" }
		out, err = r.sessionRead(ctx, []string{oid}, batchBody, namedBlob, MaxBlobBytes, nil, "cat-file", "blob", oid)
		if err != nil {
			if cemcode.CodeOf(err) == cemcode.GitExitFailure {
				return nil, unavailable("blob %s does not resolve locally", oid)
			}
			return nil, err
		}
		if err := requireObjectIdentity(ctx, "blob", oid, out); err != nil {
			return nil, err
		}
	}
	if !r.chargedOids[oid] {
		if r.chargedOids == nil {
			r.chargedOids = map[string]bool{}
		}
		r.chargedOids[oid] = true
		r.blobBytes += int64(len(out))
	}
	if r.blobBytes > MaxTotalBlobBytes {
		return nil, unavailable("verification exceeded its %d-byte blob budget", MaxTotalBlobBytes)
	}
	if eligible && !hit {
		if err := r.remember(ctx, key, memoValue{data: out}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Git's streaming cat-file blob path does not necessarily verify a loose
// object's filename against its decompressed content. Verify the typed Git
// object ourselves before these bytes can become evidence or enter a cache.
func requireBlobIdentity(ctx context.Context, oid string, raw []byte) error {
	return requireObjectIdentity(ctx, "blob", oid, raw)
}

// requireObjectIdentity re-hashes one typed Git object (`kind <len>\0<body>`)
// with the object format implied by the OID width and requires equality.
func requireObjectIdentity(ctx context.Context, kind, oid string, raw []byte) error {
	var sum hash.Hash
	switch len(oid) {
	case 40:
		sum = sha1.New()
	case 64:
		sum = sha256.New()
	default:
		return unavailable("%s object format unavailable", kind)
	}
	sum.Write([]byte(kind + " " + strconv.Itoa(len(raw)) + "\x00"))
	for len(raw) > 0 {
		if ctx.Err() != nil {
			return cemcode.New(cemcode.GitCancelled, "%s identity verification cancelled", kind)
		}
		n := min(len(raw), 64<<10)
		sum.Write(raw[:n])
		raw = raw[n:]
	}
	if ctx.Err() != nil {
		return cemcode.New(cemcode.GitCancelled, "object identity verification cancelled")
	}
	if hex.EncodeToString(sum.Sum(nil)) != oid {
		return unavailable("%s content does not match its object identity", kind)
	}
	return nil
}
