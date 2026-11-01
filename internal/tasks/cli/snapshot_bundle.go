package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Queue snapshots for lane hosts without a journal
// (docs/specs/corvint-tasks-cli-v0.md, TCL-V0-010 to TCL-V0-012): queue
// snapshot exports the intent tree with its digests, and SnapshotEnv serves
// the inventory reads from such a file and refuses everything else.

// SnapshotEnv names a `queue snapshot` file the inventory reads are served
// from (TCL-V0-011); every other verb is refused (TCL-V0-012).
const SnapshotEnv = "CORVINT_TASKS_SNAPSHOT"

const snapshotBundleSchema = "taskman-intent-snapshot/0"

// maxSnapshotBundleBytes bounds the snapshot file a lane host decodes.
const maxSnapshotBundleBytes = 256 << 20

type snapshotBundle struct {
	Profile         string                 `json:"profile"`
	SourceHost      string                 `json:"sourceHost"`
	PrimaryWorktree string                 `json:"primaryWorktree"`
	ProducedAt      string                 `json:"producedAt"`
	TreeSha256      string                 `json:"treeSha256"`
	Journal         *snapshotBundleJournal `json:"journal"`
	Files           []snapshotBundleFile   `json:"files"`
}

type snapshotBundleJournal struct {
	LastSeq           string  `json:"lastSeq"`
	LastReceiptSha256 *string `json:"lastReceiptSha256"`
}

type snapshotBundleFile struct {
	Path    string `json:"path"`
	Sha256  string `json:"sha256"`
	Content string `json:"content"`
}

// snapshotVerbs are the verbs SnapshotEnv serves (TCL-V0-011).
var snapshotVerbs = map[string]bool{
	"version": true, "--version": true, "roadmap": true, "queue status": true,
	"ticket search": true, "ticket show": true, "ticket blockers": true, "ticket export": true, "ticket similar": true, "ticket digest": true,
}

// queueSnapshot writes the intent tree as one snapshot object to --out, or
// raw to stdout (TCL-V0-010).
func queueSnapshot(env Env, args []string) int {
	cmd := []string{"queue", "snapshot"}
	values, _, bad := listFlags(cmd, args, []string{"out", "worktree"}, nil, nil)
	if bad != nil {
		return emit(env.Stdout, bad)
	}
	var raw []byte
	var b snapshotBundle
	read := withInventoryStore
	if v := values["worktree"]; len(v) == 1 {
		dir := v[0]
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(env.Cwd, dir)
		}
		read = func(_ Env, body func(*readCtx) error) (*readCtx, error) {
			return withWorktreeStore(filepath.Clean(dir), body)
		}
	}
	rc, err := read(env, func(rc *readCtx) error {
		host, _ := os.Hostname()
		b = snapshotBundle{Profile: snapshotBundleSchema, SourceHost: host, PrimaryWorktree: rc.repo.PrimaryWorktree,
			ProducedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"), TreeSha256: string(rc.store.Tree.Sha256)}
		for _, f := range rc.store.Tree.Files {
			if f.Raw == nil {
				return wire.Errorf(wire.CodeMalformed, f.Path, "the intent file's bytes were not read")
			}
			b.Files = append(b.Files, snapshotBundleFile{Path: f.Path, Sha256: string(f.Sha256), Content: string(f.Raw)})
		}
		if !rc.journalAbsent && rc.snap != nil && rc.snap.Head != nil {
			j := &snapshotBundleJournal{LastSeq: string(rc.snap.Head.LastSeq)}
			if d := rc.snap.Head.LastReceiptSha256; d != nil {
				s := string(*d)
				j.LastReceiptSha256 = &s
			}
			b.Journal = j
		}
		var e error
		raw, e = json.Marshal(b)
		raw = append(raw, '\n')
		return e
	})
	if err != nil {
		return emit(env.Stdout, failure(cmd, rc, err))
	}
	out := ""
	if v := values["out"]; len(v) == 1 {
		out = v[0]
	}
	if out == "" {
		if _, err := env.Stdout.Write(raw); err != nil {
			return 1
		}
		return 0
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(env.Cwd, out)
	}
	if err := os.WriteFile(out, raw, 0o600); err != nil {
		return emit(env.Stdout, failure(cmd, nil, wire.Errorf(wire.CodeUnsupportedFilesystem, out, "cannot write the snapshot: %v", err)))
	}
	res := success(cmd, rc)
	o := wire.NewObject().Set("path", wire.String(out)).Set("treeSha256", wire.String(b.TreeSha256))
	o.Set("files", count(len(b.Files))).Set("bytes", wire.String(string(wire.SizeOf(uint64(len(raw))))))
	o.Set("lastSeq", snapshotSeqValue(b.Journal))
	res.Items = []wire.Value{wire.ObjectValue(o)}
	return emit(env.Stdout, res)
}

// withWorktreeStore loads the intent tree at dir/.taskman with no
// repository resolution and no journal, for a linked or pruned worktree
// whose store the primary checkout would otherwise shadow (TCL-V0-869).
func withWorktreeStore(dir string, body func(*readCtx) error) (*readCtx, error) {
	if fi, err := os.Stat(filepath.Join(dir, intent.Dir)); err != nil || !fi.IsDir() {
		return nil, wire.Errorf(wire.CodeUninitialized, filepath.Join(dir, intent.Dir), "--worktree names no intent store directory")
	}
	rc := &readCtx{repo: &intent.Repository{PrimaryWorktree: dir}, journalAbsent: true}
	tree, err := intent.TreeDigest(dir)
	if err != nil {
		return rc, err
	}
	st, err := intent.LoadTree(dir, tree, "")
	if err != nil {
		return rc, err
	}
	rc.store = st
	return rc, body(rc)
}

func snapshotSeqValue(j *snapshotBundleJournal) wire.Value {
	if j == nil {
		return wire.String("NOT_OBSERVED")
	}
	return wire.String(j.LastSeq)
}

// snapshotBundlePath is SnapshotEnv's file, resolved against cwd, or "".
func snapshotBundlePath(cwd string) string {
	p := os.Getenv(SnapshotEnv)
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(cwd, p)
}

// readSnapshotBundle decodes path and verifies every file digest and the
// tree digest before anything is decoded as intent (TCL-V0-011).
func readSnapshotBundle(path string) (*snapshotBundle, intent.Tree, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, intent.Tree{}, wire.Errorf(wire.CodeMalformed, path, "cannot open the %s snapshot: %v", SnapshotEnv, err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxSnapshotBundleBytes+1))
	if err != nil || len(raw) > maxSnapshotBundleBytes {
		return nil, intent.Tree{}, wire.Errorf(wire.CodeMalformed, path, "the snapshot is unreadable or exceeds %d bytes", maxSnapshotBundleBytes)
	}
	var probe struct {
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal(raw, &probe); err == nil && probe.Profile != snapshotBundleSchema && strings.HasPrefix(probe.Profile, "taskman-intent-snapshot/") {
		return nil, intent.Tree{}, wire.Errorf(wire.CodeUnsupportedVersion, path, "the snapshot is %s; this build reads %s", probe.Profile, snapshotBundleSchema)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var b snapshotBundle
	if err := dec.Decode(&b); err != nil || b.Profile != snapshotBundleSchema {
		return nil, intent.Tree{}, wire.Errorf(wire.CodeMalformed, path, "not a %s object", snapshotBundleSchema)
	}
	tree := intent.Tree{}
	seen := map[string]bool{}
	for _, sf := range b.Files {
		content := []byte(sf.Content)
		if bound, ok := intent.BoundFor(sf.Path); !ok || len(content) > bound || seen[sf.Path] {
			return nil, intent.Tree{}, wire.Errorf(wire.CodeMalformed, path, "file %s is not an admissible intent file or is repeated", sf.Path)
		}
		seen[sf.Path] = true
		if sum := wire.Sum(content); string(sum) != sf.Sha256 {
			return nil, intent.Tree{}, wire.Errorf(wire.CodeMalformed, path, "file %s hashes to %s, not the declared %s", sf.Path, sum, sf.Sha256)
		}
		tree.Files = append(tree.Files, intent.File{Path: sf.Path, Sha256: wire.Digest(sf.Sha256), Bytes: len(content), Raw: content})
		tree.TotalBytes += len(content)
	}
	tree.Sha256 = intent.DigestOfFiles(tree.Files)
	if string(tree.Sha256) != b.TreeSha256 {
		return nil, intent.Tree{}, wire.Errorf(wire.CodeMalformed, path, "the files' tree digest %s differs from the declared %s", tree.Sha256, b.TreeSha256)
	}
	return &b, tree, nil
}

// withSnapshotStore serves an inventory read from a verified snapshot with
// every journal fact NOT_OBSERVED, as for an absent journal.
func withSnapshotStore(path string, body func(*readCtx) error) (*readCtx, error) {
	b, tree, err := readSnapshotBundle(path)
	if err != nil {
		return nil, err
	}
	rc := &readCtx{repo: &intent.Repository{PrimaryWorktree: b.PrimaryWorktree}, journalAbsent: true, bundle: b, bundlePath: path}
	st, err := intent.LoadTree(b.PrimaryWorktree, tree, tree.Sha256)
	if err != nil {
		return rc, err
	}
	rc.store = st
	return rc, body(rc)
}

// bundleWarning names where and when the served snapshot was produced.
func bundleWarning(rc *readCtx) string {
	return "served read-only from the " + SnapshotEnv + " snapshot " + rc.bundlePath + " (host " + rc.bundle.SourceHost +
		", primary worktree " + rc.bundle.PrimaryWorktree + ", produced " + rc.bundle.ProducedAt + ", journal seq " +
		snapshotSeqValue(rc.bundle.Journal).Str + "); intent written after it, journal history and liveness are NOT_OBSERVED"
}

// bundleNotFound reports a ticket the snapshot does not hold.
func bundleNotFound(rc *readCtx, id string) string {
	return "ticket " + id + " is not in the " + SnapshotEnv + " snapshot produced " + rc.bundle.ProducedAt +
		" on " + rc.bundle.SourceHost + "; it may be newer than the snapshot. Read it from the owning store: on " +
		rc.bundle.SourceHost + " run `cd " + rc.bundle.PrimaryWorktree + " && corvint-tasks ticket show " + id + "`"
}

// snapshotRefusal refuses, before any repository read, every verb
// SnapshotEnv does not serve, naming the owning store (TCL-V0-012).
func snapshotRefusal(env Env, args []string) *wire.Result {
	path := snapshotBundlePath(env.Cwd)
	if path == "" {
		return nil
	}
	verb := verbPath(args)
	if snapshotVerbs[verb] {
		return nil
	}
	owner := "the store the snapshot was produced from"
	if b, _, err := readSnapshotBundle(path); err == nil {
		owner = "the owning store on " + b.SourceHost + " (`cd " + b.PrimaryWorktree + " && corvint-tasks " + strings.Join(args, " ") + "`)"
	}
	return &wire.Result{Command: strings.Fields(verb), Outcome: wire.OutcomeError, Codes: []string{wire.CodeUnsupported}, NotRetryable: true, Warnings: []string{
		SnapshotEnv + " is set: this host reads a queue snapshot, which serves only " + strings.Join(snapshotVerbList(), ", ") +
			"; " + verb + " was refused before any repository read and nothing was written. Run it in " + owner + " (TCL-V0-012).",
	}}
}

func snapshotVerbList() []string {
	return []string{"ticket search", "ticket show", "ticket export", "ticket similar", "ticket digest", "roadmap", "queue status"}
}
