// Package fixture builds realistic on-disk fixtures for TCP-01 tests: a
// primary worktree with its `.git` common directory, a `.taskman/` intent
// store and a `<git-common-dir>/taskman/` state dir with a valid receipt
// chain. It is test support only (imported by `_test.go` files); it never
// touches a real repository and every path it writes is under a temp
// directory the caller owns. It also provides the byte/mode/mtime tree
// snapshot used to prove that read commands modify nothing (AS-07).
package fixture

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Queue identity shared by every fixture.
const (
	QueueID   = "queue:acme:main"
	RepoID    = "repo:acme"
	Prefix    = "AT"
	Actor     = "russell"
	Timestamp = "2026-09-06T12:00:00Z"
)

// TicketID renders the fixture queue's ticket ID for a local token.
func TicketID(local string) string { return "ticket:acme:main:" + local }

// Repo is a fixture repository on disk.
type Repo struct {
	Root      string // resolved absolute path of the primary worktree
	CommonDir string // <Root>/.git
	StateDir  string // <Root>/.git/taskman
	IntentDir string // <Root>/.taskman
}

// TempRepo creates a primary worktree with a `.git` directory under a
// symlink-resolved temp path (macOS's /var is a symlink; the §3.4 resolver
// refuses symlinked paths). `head.primaryWorktree` is a PathText bounded to
// 4096 bytes (§2). It removes everything at test cleanup.
func TempRepo(t *testing.T) *Repo {
	t.Helper()
	base, err := os.MkdirTemp("", "atm-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	real, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatalf("resolve temp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(real) })
	root := filepath.Join(real, "repo")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	Write(t, filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"))
	if _, err := wire.ParsePathText("", root); err != nil {
		t.Fatalf("temp path %q is not a valid head.primaryWorktree PathText: %v", root, err)
	}
	return &Repo{Root: root, CommonDir: filepath.Join(root, ".git"), StateDir: filepath.Join(root, ".git", "taskman"), IntentDir: filepath.Join(root, intent.Dir)}
}

// TempDirOutside returns a resolved temp directory that is neither inside
// the repository nor inside the state dir (a legal `--staging`).
func TempDirOutside(t *testing.T) string {
	t.Helper()
	base, err := os.MkdirTemp("", "atm-staging-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	real, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatalf("resolve temp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(real) })
	return real
}

// Write writes a file, creating parents, with mode 0644.
func Write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func obj(kv ...interface{}) wire.Value {
	o := wire.NewObject()
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}

func str(s string) wire.Value { return wire.String(s) }

// QueueValue is a valid taskman-queue/0 for the fixture queue.
func QueueValue() wire.Value {
	return obj(
		"profile", str(intent.ProfileQueue),
		"queueId", str(QueueID),
		"repositoryAuthorityId", str(RepoID),
		"prefix", str(Prefix),
		"nextSerial", str("2"),
		"schemaVersion", str("0"),
		"canonicalWriter", str("NATIVE"),
		"foreignAdapterId", wire.Null(),
		"intentBranch", str("main"),
		"fixture", wire.Bool(true),
		"executionCutover", wire.Null(),
		"importMapSha256", wire.Null(),
		"writeBarrier", obj("reason", str("NONE"), "since", wire.Null()),
	)
}

// QueueBytes is QueueValue on disk.
func QueueBytes() []byte { return wire.EncodeFile(QueueValue()) }

// PolicyValue is a valid taskman-policy/0 with one required COMMAND gate
// `verify` and serialFallback BLOCK.
func PolicyValue() wire.Value {
	gate := obj(
		"gateId", str("verify"),
		"kind", str("COMMAND"),
		"argv", wire.Strings([]string{"make", "verify"}),
		"cwd", str("WORKTREE"),
		"env", wire.Strings(nil),
		"timeoutSeconds", str("1800"),
		"expected", obj("exitCode", str("0"), "reducer", wire.Null()),
		"evidence", wire.Strings(nil),
		"inputs", wire.Strings(nil),
		"sharedResource", wire.Null(),
		"reusable", wire.Bool(true),
		"required", wire.Bool(true),
	)
	return obj(
		"profile", str(intent.ProfilePolicy),
		"policyVersion", str("1"),
		"roles", wire.ObjectValue(wire.NewObject()),
		"capacity", obj("maxActiveAttempts", str("1"), "maxWorkersTotal", str("4"), "classes", wire.Array()),
		"budgets", obj(
			"lane", obj("inputTokens", str("2000000"), "cacheCreationTokens", str("1000000"), "cacheReadTokens", str("100000000"),
				"outputTokens", str("400000"), "turns", str("400"), "wallClockMinutes", str("90")),
			"ticketMultiplier", str("4"),
			"requireEnforcedFields", wire.Strings([]string{"turns", "wallClockMinutes"}),
		),
		"retries", obj("admissionsPerRevision", str("3"), "repairRounds", str("2"), "malformedReviewRetry", str("1"),
			"gateRerunOnStale", str("1"), "reconcileAttempts", str("3")),
		"retention", obj("evidenceDays", str("90")),
		"gates", wire.Array(gate),
		"serialFallback", str("BLOCK"),
		"integrationRequiredKinds", wire.Strings(nil),
		"allowEmptyObligationsKinds", wire.Strings([]string{"CHORE"}),
		"reviewLane", obj("required", wire.Bool(true)),
		"docsLane", obj("required", wire.Bool(false)),
		"cemRequired", wire.Bool(false),
		"ocmRequired", wire.Bool(false),
		"runtimes", wire.Array(),
		"environment", obj("allowedEnvKeys", wire.Strings(nil)),
	)
}

// PolicyBytes is PolicyValue on disk.
func PolicyBytes() []byte { return wire.EncodeFile(PolicyValue()) }

// Ticket returns a minimal valid OPEN, AUTONOMOUS, QUALIFIED record for the
// local token, at revision 1. Callers mutate the struct before encoding.
func Ticket(local string) *ticket.Record {
	id, err := wire.ParseTicketID("", TicketID(local))
	if err != nil {
		panic(err)
	}
	return &ticket.Record{
		TicketID:           id,
		Revision:           "1",
		AcceptanceRevision: "1",
		Status:             ticket.StatusOpen,
		Title:              "Fixture ticket " + local,
		Kind:               "FEATURE",
		Priority:           "P2",
		Order:              "0",
		Labels:             []string{},
		Dependencies:       []ticket.Dependency{},
		AcceptanceCriteria: []string{"it decodes"},
		RequirementRefs:    []string{},
		Source:             ticket.Source{Kind: "NATIVE", SourceQueueID: QueueID},
		Effects:            ticket.Effects{Coverage: "QUALIFIED", TouchPaths: []string{}, Resources: []ticket.Resource{}},
		Capabilities:       []string{},
		RequiredGates:      []string{},
		Holds:              []ticket.Hold{},
		ExecutionClass:     "AUTONOMOUS",
		Approvals:          []ticket.Approval{},
		CreatedAt:          Timestamp,
		UpdatedAt:          Timestamp,
		UpdatedBy:          Actor,
	}
}

// Dep builds a COMPLETED dependency edge on a local token.
func Dep(local string) ticket.Dependency {
	id, err := wire.ParseTicketID("", TicketID(local))
	if err != nil {
		panic(err)
	}
	return ticket.Dependency{TicketID: id, Obligation: "COMPLETED"}
}

// GateDep builds a GATE_PASSED dependency edge.
func GateDep(local, gate string) ticket.Dependency {
	d := Dep(local)
	d.Obligation = "GATE_PASSED"
	g := gate
	d.GateID = &g
	return d
}

// WriteIntent writes queue.json, policy.json and the given tickets.
func WriteIntent(t *testing.T, r *Repo, tickets ...*ticket.Record) {
	t.Helper()
	Write(t, filepath.Join(r.IntentDir, intent.QueueFile), QueueBytes())
	Write(t, filepath.Join(r.IntentDir, intent.PolicyFile), PolicyBytes())
	if err := os.MkdirAll(filepath.Join(r.IntentDir, intent.TicketsDir), 0o755); err != nil {
		t.Fatalf("mkdir tickets: %v", err)
	}
	for _, rec := range tickets {
		Write(t, filepath.Join(r.IntentDir, intent.TicketsDir, rec.TicketID.Local+".json"), rec.Encode())
	}
}

// ReceiptValue builds a receipt of the given kind at seq with prev.
func ReceiptValue(seq uint64, prev *wire.Digest, kind string, headGeneration uint64) wire.Value {
	pv := wire.Null()
	if prev != nil {
		pv = str(string(*prev))
	}
	return obj(
		"profile", str(snapshot.ProfileReceipt),
		"seq", str(string(wire.SizeOf(seq))),
		"prev", pv,
		"kind", str(kind),
		"requestId", wire.Null(),
		"actor", obj("id", str(Actor), "role", str("OWNER")),
		"ticketId", wire.Null(),
		"attemptId", wire.Null(),
		"generation", wire.Null(),
		"expectedRevision", wire.Null(),
		"headGeneration", str(string(wire.SizeOf(headGeneration))),
		"pre", wire.Array(),
		"post", wire.Array(),
		"outcome", str("COMPLETED"),
		"codes", wire.Strings(nil),
		"recordedAt", str(Timestamp),
	)
}

// HeadValue builds head.json for the fixture queue.
func HeadValue(primary string, lastSeq uint64, last wire.Digest, generation uint64, initSha wire.Digest) wire.Value {
	return obj(
		"profile", str(snapshot.ProfileHead),
		"queueId", str(QueueID),
		"lastSeq", str(string(wire.SizeOf(lastSeq))),
		"lastReceiptSha256", str(string(last)),
		"generation", str(string(wire.SizeOf(generation))),
		"initSha256", str(string(initSha)),
		"primaryWorktree", str(primary),
		"versionSha256", str(string(wire.Sum([]byte(snapshot.VersionBytes)))),
	)
}

// WriteState initialises the state dir with VERSION, an INIT receipt at seq
// 1 and a matching head.json. This is fixture construction, not a writer.
func WriteState(t *testing.T, r *Repo) {
	t.Helper()
	Write(t, filepath.Join(r.StateDir, "VERSION"), []byte(snapshot.VersionBytes))
	rc := wire.EncodeFile(GenesisValue(t, r))
	name, _ := snapshot.ReceiptName(1)
	Write(t, filepath.Join(r.StateDir, "receipts", name), rc)
	d := wire.Sum(rc)
	Write(t, filepath.Join(r.StateDir, "head.json"), wire.EncodeFile(HeadValue(r.Root, 1, d, 0, d)))
}

// Commit appends one receipt of the given kind to the chain and advances
// head.json, simulating a committed transaction (test-only; the real
// commit protocol is TCP-02's).
func Commit(t *testing.T, r *Repo, kind string) {
	t.Helper()
	hraw, err := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
	if err != nil {
		t.Fatalf("read head: %v", err)
	}
	h, err := snapshot.DecodeHead(hraw)
	if err != nil {
		t.Fatalf("decode head: %v", err)
	}
	seq := h.LastSeq.Uint64() + 1
	gen := h.Generation.Uint64()
	rc := wire.EncodeFile(ReceiptValue(seq, h.LastReceiptSha256, kind, gen))
	name, _ := snapshot.ReceiptName(seq)
	Write(t, filepath.Join(r.StateDir, "receipts", name), rc)
	Write(t, filepath.Join(r.StateDir, "head.json"), wire.EncodeFile(HeadValue(h.PrimaryWorktree, seq, wire.Sum(rc), gen, h.InitSha256)))
}

// PlantReceipt writes a receipt file at seq without touching head.json
// (a pending redo at lastSeq+1, or a fork at lastSeq+2).
func PlantReceipt(t *testing.T, r *Repo, seq uint64) {
	t.Helper()
	hraw, err := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
	if err != nil {
		t.Fatalf("read head: %v", err)
	}
	h, err := snapshot.DecodeHead(hraw)
	if err != nil {
		t.Fatalf("decode head: %v", err)
	}
	rc := wire.EncodeFile(ReceiptValue(seq, h.LastReceiptSha256, "MUTATION", h.Generation.Uint64()))
	name, _ := snapshot.ReceiptName(seq)
	Write(t, filepath.Join(r.StateDir, "receipts", name), rc)
}

// Entry is one row of a tree snapshot.
type Entry struct {
	Path  string
	Mode  fs.FileMode
	Size  int64
	MTime int64
	Sha   string
}

// TreeSnapshot records every entry under root (path, mode, size, mtime and
// content digest for regular files). A missing root yields an empty
// snapshot, so "still absent" compares equal.
func TreeSnapshot(t *testing.T, root string) []Entry {
	t.Helper()
	var out []Entry
	if _, err := os.Lstat(root); err != nil {
		if os.IsNotExist(err) {
			return out
		}
		t.Fatalf("stat %s: %v", root, err)
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		e := Entry{Path: strings.TrimPrefix(p, root), Mode: info.Mode(), Size: info.Size(), MTime: info.ModTime().UnixNano()}
		if info.Mode().IsRegular() {
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			h := sha256.Sum256(raw)
			e.Sha = hex.EncodeToString(h[:])
		}
		out = append(out, e)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// SameTree reports whether two snapshots are identical.
func SameTree(a, b []Entry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// AssertUntouched fails the test when the state dir, the intent dir or the
// lock path changed relative to the snapshots taken before a read, or when
// a lock file appeared.
func AssertUntouched(t *testing.T, r *Repo, stateBefore, intentBefore []Entry, what string) {
	t.Helper()
	if !SameTree(stateBefore, TreeSnapshot(t, r.StateDir)) {
		t.Fatalf("%s: state dir changed (bytes, mode or mtime)", what)
	}
	if !SameTree(intentBefore, TreeSnapshot(t, r.IntentDir)) {
		t.Fatalf("%s: intent tree changed (bytes, mode or mtime)", what)
	}
	if _, err := os.Lstat(filepath.Join(r.CommonDir, "taskman.lock")); err == nil {
		t.Fatalf("%s: a lock file was created", what)
	}
}

// GenesisValue builds the minimal experimental J1 genesis, supplying actual
// retained VERSION blob bytes. Existing intent fixtures remain projections;
// callers needing a full ledger must post tickets in later receipts.
func GenesisValue(t *testing.T, r *Repo) wire.Value {
	t.Helper()
	q, _ := wire.ParseQueueID("", QueueID)
	init := snapshot.Init{QueueID: q, PrimaryWorktree: r.Root, VersionSha256: wire.Sum([]byte(snapshot.VersionBytes))}
	descriptor := wire.EncodeFile(init.Value())
	reservations := wire.EncodeFile(obj("profile", str("taskman-reservation-set/0"), "queueId", str(QueueID), "entries", wire.Array()))
	raws := map[string][]byte{"VERSION": []byte(snapshot.VersionBytes), "intent/queue.json": QueueBytes(), "intent/policy.json": PolicyBytes(), "reservations.json": reservations, "pinned/" + string(wire.Sum(descriptor)) + ".json": descriptor}
	var paths []string
	for path := range raws {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var pres, posts []wire.Value
	for _, path := range paths {
		raw := raws[path]
		d := wire.Sum(raw)
		rec := wire.Null()
		blob := wire.Null()
		if path == "VERSION" {
			blob = str(string(d))
			Write(t, filepath.Join(r.StateDir, "evidence", string(d)), raw)
		} else {
			var err error
			rec, err = wire.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
		}
		pres = append(pres, obj("path", str(path), "sha256", wire.Null()))
		posts = append(posts, obj("path", str(path), "sha256", str(string(d)), "record", rec, "blobSha256", blob))
		if !strings.HasPrefix(path, "intent/") {
			Write(t, filepath.Join(r.StateDir, path), raw)
		}
	}
	v := ReceiptValue(1, nil, "INIT", 0)
	v.Obj.Set("pre", wire.Array(pres...))
	v.Obj.Set("post", wire.Array(posts...))
	return v
}

// CommitPosts is test-only fixture assembly: receipt bytes and projections are
// written directly, without simulating durability or granting writer authority.
func CommitPosts(t *testing.T, r *Repo, kind, requestID string, posts map[string][]byte) {
	t.Helper()
	hraw, err := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := snapshot.DecodeHead(hraw)
	if err != nil {
		t.Fatal(err)
	}
	seq := h.LastSeq.Uint64() + 1
	v := ReceiptValue(seq, h.LastReceiptSha256, kind, h.Generation.Uint64())
	if requestID != "" {
		v.Obj.Set("requestId", str(requestID))
	}
	var paths []string
	for p := range posts {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var pres, after []wire.Value
	for _, p := range paths {
		raw := posts[p]
		rec, err := wire.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		// New fixture posts only: caller supplies fresh canonical paths.
		pres = append(pres, obj("path", str(p), "sha256", wire.Null()))
		after = append(after, obj("path", str(p), "sha256", str(string(wire.Sum(raw))), "record", rec, "blobSha256", wire.Null()))
		dest := filepath.Join(r.StateDir, p)
		if strings.HasPrefix(p, "intent/") {
			dest = filepath.Join(r.IntentDir, strings.TrimPrefix(p, "intent/"))
		}
		Write(t, dest, raw)
	}
	v.Obj.Set("pre", wire.Array(pres...))
	v.Obj.Set("post", wire.Array(after...))
	raw := wire.EncodeFile(v)
	name, _ := snapshot.ReceiptName(seq)
	Write(t, filepath.Join(r.StateDir, "receipts", name), raw)
	Write(t, filepath.Join(r.StateDir, "head.json"), wire.EncodeFile(HeadValue(r.Root, seq, wire.Sum(raw), h.Generation.Uint64(), h.InitSha256)))
}
