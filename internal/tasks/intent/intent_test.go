package intent_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func code(err error) string { return wire.CodeOf(err) }

// TestTMV0008_AS08_IntentTreeDigestPreimage recomputes the SPEC §3.1 (R3)
// preimage independently of the package and compares.
func TestTMV0008_AS08_IntentTreeDigestPreimage(t *testing.T) {
	r := fixture.TempRepo(t)
	b := fixture.Ticket("B")
	a := fixture.Ticket("A")
	fixture.WriteIntent(t, r, b, a)
	tree, err := intent.TreeDigest(r.Root)
	if err != nil {
		t.Fatalf("TreeDigest: %v", err)
	}
	paths := []string{"policy.json", "queue.json", "tickets/A.json", "tickets/B.json"}
	h := sha256.New()
	for _, p := range paths {
		raw, err := os.ReadFile(filepath.Join(r.IntentDir, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write([]byte(hex.EncodeToString(sum[:])))
		h.Write([]byte{'\n'})
	}
	want := wire.Digest(hex.EncodeToString(h.Sum(nil)))
	if tree.Sha256 != want {
		t.Errorf("tree digest %s, want %s", tree.Sha256, want)
	}
	if len(tree.Files) != 4 {
		t.Fatalf("files %d", len(tree.Files))
	}
	for i, p := range paths {
		f := tree.Files[i]
		if f.Path != p || f.Raw == nil || wire.Sum(f.Raw) != f.Sha256 || f.Bytes != len(f.Raw) {
			t.Errorf("file %d: %+v", i, f)
		}
	}
	if intent.DigestOfFiles(tree.Files) != want {
		t.Errorf("DigestOfFiles differs")
	}
	rev := []intent.File{tree.Files[3], tree.Files[0], tree.Files[2], tree.Files[1]}
	if intent.DigestOfFiles(rev) != want {
		t.Errorf("DigestOfFiles must be order independent")
	}
	empty := wire.Digest(hex.EncodeToString(func() []byte { s := sha256.Sum256(nil); return s[:] }()))
	if intent.DigestOfFiles(nil) != empty {
		t.Errorf("empty tree must digest the empty string")
	}
}

// TestTMV0008_AS08_PrimaryWorktreeDigestPreimage: SHA-256 of the exact
// absolute path bytes.
func TestTMV0008_AS08_PrimaryWorktreeDigestPreimage(t *testing.T) {
	r := fixture.TempRepo(t)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	sum := sha256.Sum256([]byte(r.Root))
	if repo.PrimaryWorktreeSha256() != wire.Digest(hex.EncodeToString(sum[:])) {
		t.Errorf("primary worktree digest is not the digest of the path bytes")
	}
	if repo.PrimaryWorktree != r.Root || repo.CommonDir != r.CommonDir || repo.StateDir != r.StateDir {
		t.Errorf("resolution: %+v", repo)
	}
	if repo.LockPath != filepath.Join(r.CommonDir, "taskman.lock") {
		t.Errorf("lock path %q must be <git-common-dir>/taskman.lock", repo.LockPath)
	}
}

// TestTMV0001_AS29_ResolveFromSubdirAndLinkedWorktree: resolution walks up,
// follows a `.git` file with `commondir`, and refuses symlinked paths.
func TestTMV0001_AS29_ResolveFromSubdirAndLinkedWorktree(t *testing.T) {
	r := fixture.TempRepo(t)
	sub := filepath.Join(r.Root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(sub)
	if err != nil || repo.PrimaryWorktree != r.Root || repo.FromLinkedWorktree {
		t.Fatalf("subdir resolve: %+v %v", repo, err)
	}
	// Linked worktree layout: <root>/.git/worktrees/wt/{commondir,gitdir}
	// and a sibling checkout whose .git file names that gitdir.
	wtGit := filepath.Join(r.CommonDir, "worktrees", "wt")
	fixture.Write(t, filepath.Join(wtGit, "commondir"), []byte("../..\n"))
	linked := filepath.Join(filepath.Dir(r.Root), "linked")
	fixture.Write(t, filepath.Join(linked, ".git"), []byte("gitdir: "+wtGit+"\n"))
	repo, err = intent.Resolve(linked)
	if err != nil {
		t.Fatalf("linked resolve: %v", err)
	}
	if repo.PrimaryWorktree != r.Root || !repo.FromLinkedWorktree || repo.StateDir != r.StateDir {
		t.Errorf("linked worktree must resolve to the primary: %+v", repo)
	}
	// No .git anywhere up to the temp root: UNINITIALIZED (the temp base has
	// no .git; the walk stops at the filesystem root, so use a dir outside).
	outside := fixture.TempDirOutside(t)
	if _, err := intent.Resolve(outside); code(err) != wire.CodeUninitialized {
		// The machine's temp dir may itself sit under a repository; then the
		// answer is whatever that repository is, which is still not an error.
		if err != nil {
			t.Errorf("outside resolve: %v", err)
		}
	}
	// Symlinked .git is refused.
	r2 := fixture.TempRepo(t)
	link := filepath.Join(filepath.Dir(r2.Root), "viaLink")
	if err := os.Symlink(r2.Root, link); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, err := intent.Resolve(link); code(err) != wire.CodeUnsupportedFilesystem {
		t.Errorf("symlinked path accepted: %v", err)
	}
	if err := intent.CheckNoSymlink(filepath.Join(link, "x", "y")); code(err) != wire.CodeUnsupportedFilesystem {
		t.Errorf("CheckNoSymlink missed a symlink component: %v", err)
	}
	if err := intent.CheckNoSymlink(filepath.Join(r2.Root, "does", "not", "exist")); err != nil {
		t.Errorf("non-existent tail is not a symlink: %v", err)
	}
}

// TestTMV0002_AS10_TreeDigestFlatLayout: every inadmissible entry fails
// closed before any byte is read.
func TestTMV0002_AS10_TreeDigestFlatLayout(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, r *fixture.Repo)
		want  string
	}{
		{"unexpected root entry", func(t *testing.T, r *fixture.Repo) {
			fixture.Write(t, filepath.Join(r.IntentDir, "notes.md"), []byte("x\n"))
		}, wire.CodeMalformed},
		{"nested directory", func(t *testing.T, r *fixture.Repo) {
			fixture.Write(t, filepath.Join(r.IntentDir, "tickets", "sub", "x.json"), []byte("{}\n"))
		}, wire.CodeMalformed},
		{"non-json ticket entry", func(t *testing.T, r *fixture.Repo) {
			fixture.Write(t, filepath.Join(r.IntentDir, "tickets", "A.json.bak"), []byte("{}\n"))
		}, wire.CodeMalformed},
		{"bad local token", func(t *testing.T, r *fixture.Repo) {
			fixture.Write(t, filepath.Join(r.IntentDir, "tickets", "-A.json"), []byte("{}\n"))
		}, wire.CodeMalformed},
		{"empty ticket file", func(t *testing.T, r *fixture.Repo) {
			fixture.Write(t, filepath.Join(r.IntentDir, "tickets", "E.json"), nil)
		}, wire.CodeMalformed},
		{"empty queue file", func(t *testing.T, r *fixture.Repo) { fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), nil) }, wire.CodeMalformed},
		{"oversized ticket file", func(t *testing.T, r *fixture.Repo) {
			fixture.Write(t, filepath.Join(r.IntentDir, "tickets", "BIG.json"), []byte(strings.Repeat("x", wire.MaxTicketFileBytes+1)))
		}, wire.CodeLimitExceeded},
		{"oversized policy", func(t *testing.T, r *fixture.Repo) {
			fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), []byte(strings.Repeat("x", wire.MaxPolicyFileBytes+1)))
		}, wire.CodeLimitExceeded},
		{"symlink ticket", func(t *testing.T, r *fixture.Repo) {
			if err := os.Symlink(filepath.Join(r.IntentDir, "queue.json"), filepath.Join(r.IntentDir, "tickets", "L.json")); err != nil {
				t.Skip("symlinks unavailable")
			}
		}, wire.CodeUnsupportedFilesystem},
		{"symlink tickets dir", func(t *testing.T, r *fixture.Repo) {
			os.RemoveAll(filepath.Join(r.IntentDir, "tickets"))
			if err := os.Symlink(r.CommonDir, filepath.Join(r.IntentDir, "tickets")); err != nil {
				t.Skip("symlinks unavailable")
			}
		}, wire.CodeUnsupportedFilesystem},
		{"symlink escaping root", func(t *testing.T, r *fixture.Repo) {
			if err := os.Symlink("/etc/hosts", filepath.Join(r.IntentDir, "tickets", "H.json")); err != nil {
				t.Skip("symlinks unavailable")
			}
		}, wire.CodeUnsupportedFilesystem},
		{".taskman is a symlink", func(t *testing.T, r *fixture.Repo) {
			os.RemoveAll(r.IntentDir)
			if err := os.Symlink(r.CommonDir, r.IntentDir); err != nil {
				t.Skip("symlinks unavailable")
			}
		}, wire.CodeUnsupportedFilesystem},
		{"tickets is a file", func(t *testing.T, r *fixture.Repo) {
			os.RemoveAll(filepath.Join(r.IntentDir, "tickets"))
			fixture.Write(t, filepath.Join(r.IntentDir, "tickets"), []byte("x\n"))
		}, wire.CodeMalformed},
		{"root over five entries", func(t *testing.T, r *fixture.Repo) {
			// queue.json, policy.json, tickets/ plus three strays: the root
			// entry bound fires during the listing, before names are checked.
			fixture.Write(t, filepath.Join(r.IntentDir, "notes.md"), []byte("x\n"))
			fixture.Write(t, filepath.Join(r.IntentDir, "more.md"), []byte("x\n"))
			fixture.Write(t, filepath.Join(r.IntentDir, "extra.md"), []byte("x\n"))
		}, wire.CodeLimitExceeded},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := fixture.TempRepo(t)
			fixture.WriteIntent(t, r, fixture.Ticket("A"))
			c.setup(t, r)
			_, err := intent.TreeDigest(r.Root)
			if code(err) != c.want {
				t.Errorf("code %q, want %q (%v)", code(err), c.want, err)
			}
		})
	}
	r := fixture.TempRepo(t)
	if _, err := intent.TreeDigest(r.Root); code(err) != wire.CodeUninitialized {
		t.Errorf("absent .taskman: %v", err)
	}
	if err := os.MkdirAll(r.IntentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tree, err := intent.TreeDigest(r.Root)
	if err != nil || len(tree.Files) != 0 {
		t.Errorf("empty store must digest with zero files: %v", err)
	}
	if _, err := intent.Load(r.Root); code(err) != wire.CodeUninitialized {
		t.Errorf("Load of an empty store: %v", err)
	}
}

// TestTMV0002_AS10_TreeDigestTicketCountBound: 10,001 zero-byte entries are
// refused by the count bound before any entry is opened or its emptiness
// reported.
func TestTMV0002_AS10_TreeDigestTicketCountBound(t *testing.T) {
	if testing.Short() {
		t.Skip("creates 10,001 files")
	}
	r := fixture.TempRepo(t)
	fixture.WriteIntent(t, r)
	dir := filepath.Join(r.IntentDir, "tickets")
	for i := 0; i <= wire.MaxTicketsPerQueue; i++ {
		name := filepath.Join(dir, "T"+string(wire.CountOf(int64(i)))+".json")
		f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	_, err := intent.TreeDigest(r.Root)
	if code(err) != wire.CodeLimitExceeded || !strings.Contains(err.Error(), "entries under tickets/") {
		t.Errorf("count bound must fire first: %v", err)
	}
}

// TestTMV0002_AS10_ReadDirNamesBoundedEnumeration: a directory is listed
// in chunks and refused LIMIT_EXCEEDED as soon as the bound is passed,
// whatever the entries are named; within the bound every name is returned
// sorted. Nothing under the directory is opened.
func TestTMV0002_AS10_ReadDirNamesBoundedEnumeration(t *testing.T) {
	dir := t.TempDir()
	n := 2*intent.DirChunk + 5
	for i := 0; i < n; i++ {
		f, err := os.OpenFile(filepath.Join(dir, "e"+string(wire.CountOf(int64(i)))), os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	list := func(max int) ([]string, error) {
		d, err := os.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		return intent.ReadDirNames(d, dir, "x/", max)
	}
	names, err := list(n)
	if err != nil || len(names) != n || !sort.StringsAreSorted(names) {
		t.Errorf("bound equal to the count: %d names, sorted %v, %v", len(names), sort.StringsAreSorted(names), err)
	}
	for _, max := range []int{n - 1, intent.DirChunk, 1, 0} {
		names, err := list(max)
		if code(err) != wire.CodeLimitExceeded || names != nil || !strings.Contains(err.Error(), "entries under x/") {
			t.Errorf("bound %d: %v (%d names)", max, err, len(names))
		}
	}
	if _, err := list(n + 1); err != nil {
		t.Errorf("bound above the count: %v", err)
	}
}

// TestTMV0002_AS10_LaneBudgetPrimitives (B1 resolution): the four token
// budget fields are Size, turns and wallClockMinutes are Count (§2, §3.1).
func TestTMV0002_AS10_LaneBudgetPrimitives(t *testing.T) {
	set := func(field, val string) []byte {
		v := fixture.PolicyValue()
		b, _ := v.Obj.Get("budgets")
		lane, _ := b.Obj.Get("lane")
		lane.Obj.Set(field, wire.String(val))
		return wire.EncodeFile(v)
	}
	for _, f := range []string{"inputTokens", "cacheCreationTokens", "cacheReadTokens", "outputTokens"} {
		if _, err := intent.DecodePolicy(set(f, "4294967296")); err != nil {
			t.Errorf("%s above the Count maximum must be accepted as Size: %v", f, err)
		}
		if _, err := intent.DecodePolicy(set(f, "18446744073709551616")); code(err) != wire.CodeMalformed {
			t.Errorf("%s above uint64: %v", f, err)
		}
	}
	for _, f := range []string{"turns", "wallClockMinutes"} {
		if _, err := intent.DecodePolicy(set(f, "4294967296")); code(err) != wire.CodeMalformed {
			t.Errorf("%s is a Count and must refuse a Size: %v", f, err)
		}
	}
	if _, err := intent.DecodePolicy(set("wallClockMinutes", "241")); code(err) != wire.CodeLimitExceeded {
		t.Errorf("wallClockMinutes over the §1 max: %v", err)
	}
}

// TestTMV0002_AS01_LoadStore covers a valid store and its cross-file rules.
func TestTMV0002_AS01_LoadStore(t *testing.T) {
	r := fixture.TempRepo(t)
	a := fixture.Ticket("A")
	a.RequiredGates = []string{"verify"}
	b := fixture.Ticket("B")
	b.Dependencies = []ticket.Dependency{fixture.GateDep("A", "verify")}
	fixture.WriteIntent(t, r, a, b)
	st, err := intent.Load(r.Root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.Queue.QueueID.Raw != fixture.QueueID || st.Policy == nil || st.Inventory.Len() != 2 || st.ImportMap != nil {
		t.Errorf("store: %+v", st)
	}
	if st.Digests["tickets/A.json"] != wire.Sum(a.Encode()) {
		t.Errorf("digest map wrong")
	}
	if st.Policy.PolicySha256() != wire.ContentID("policy", intent.ProfilePolicy, []byte(strings.TrimSuffix(string(fixture.PolicyBytes()), "\n"))) {
		t.Errorf("policy identity is not the WQO content identity of the canonical body")
	}
	// Decoded records re-encode to the captured bytes: hash and decode used
	// the same bytes.
	for _, f := range st.Tree.Files {
		if strings.HasPrefix(f.Path, "tickets/") {
			rec, _ := st.Inventory.Get(fixture.TicketID(strings.TrimSuffix(strings.TrimPrefix(f.Path, "tickets/"), ".json")))
			if string(rec.Encode()) != string(f.Raw) {
				t.Errorf("%s: decoded record differs from captured bytes", f.Path)
			}
		}
	}
	// Unknown gate in requiredGates and in a dependency.
	c := fixture.Ticket("C")
	c.RequiredGates = []string{"nope"}
	fixture.WriteIntent(t, r, a, b, c)
	if _, err := intent.Load(r.Root); code(err) != wire.CodeGateUnknown {
		t.Errorf("unknown required gate: %v", err)
	}
	c.RequiredGates = []string{}
	c.Dependencies = []ticket.Dependency{fixture.GateDep("A", "nope")}
	fixture.WriteIntent(t, r, a, b, c)
	if _, err := intent.Load(r.Root); code(err) != wire.CodeGateUnknown {
		t.Errorf("unknown dependency gate: %v", err)
	}
	// File name must match the local token.
	os.Remove(filepath.Join(r.IntentDir, "tickets", "C.json"))
	fixture.Write(t, filepath.Join(r.IntentDir, "tickets", "X.json"), fixture.Ticket("Y").Encode())
	if _, err := intent.Load(r.Root); code(err) != wire.CodeMalformed || !strings.Contains(err.Error(), "file name") {
		t.Errorf("name mismatch: %v", err)
	}
	os.Remove(filepath.Join(r.IntentDir, "tickets", "X.json"))
	// A malformed ticket is reported with its path prefix.
	fixture.Write(t, filepath.Join(r.IntentDir, "tickets", "Z.json"), []byte("{}\n"))
	if _, err := intent.Load(r.Root); code(err) != wire.CodeMalformed || !strings.HasPrefix(err.Error(), "MALFORMED: tickets/Z.json") {
		t.Errorf("malformed ticket location: %v", err)
	}
	os.Remove(filepath.Join(r.IntentDir, "tickets", "Z.json"))
	// importMapSha256 must match import-map.json, and the map must exist
	// when named.
	q := fixture.QueueValue()
	imap := wire.EncodeFile(mapValue())
	fixture.Write(t, filepath.Join(r.IntentDir, intent.ImportMapFile), imap)
	if _, err := intent.Load(r.Root); code(err) != wire.CodeMalformed || !strings.Contains(err.Error(), "importMapSha256") {
		t.Errorf("import map without digest: %v", err)
	}
	q.Obj.Set("importMapSha256", wire.String(string(wire.Sum(imap))))
	fixture.Write(t, filepath.Join(r.IntentDir, intent.QueueFile), wire.EncodeFile(q))
	st, err = intent.Load(r.Root)
	if err != nil || st.ImportMap == nil || len(st.ImportMap.Entries) != 1 {
		t.Errorf("import map load: %v", err)
	}
	os.Remove(filepath.Join(r.IntentDir, intent.ImportMapFile))
	if _, err := intent.Load(r.Root); code(err) != wire.CodeMalformed {
		t.Errorf("named but absent import map: %v", err)
	}
	// Missing policy is UNINITIALIZED.
	fixture.Write(t, filepath.Join(r.IntentDir, intent.QueueFile), fixture.QueueBytes())
	os.Remove(filepath.Join(r.IntentDir, intent.PolicyFile))
	if _, err := intent.Load(r.Root); code(err) != wire.CodeUninitialized {
		t.Errorf("missing policy: %v", err)
	}
}

func mapValue() wire.Value {
	e := wire.NewObject()
	e.Set("sourceQueueId", wire.String("roadmap"))
	e.Set("sourceItemId", wire.String("R-1"))
	e.Set("ticketId", wire.String(fixture.TicketID("A")))
	e.Set("sourceRevisionSha256", wire.Null())
	e.Set("appliedSeq", wire.String("3"))
	o := wire.NewObject()
	o.Set("profile", wire.String(intent.ProfileImportMap))
	o.Set("queueId", wire.String(fixture.QueueID))
	o.Set("entries", wire.Array(wire.ObjectValue(e)))
	return wire.ObjectValue(o)
}

// TestTMV0008_AS36_LoadPinnedToSnapshotDigest is the deterministic A→B→A
// regression: a store mutated between the probe and the load must fail
// SNAPSHOT_MOVED even though the outer probes agree, because the load
// decodes and hashes one captured copy and compares it to the pinned digest.
func TestTMV0008_AS36_LoadPinnedToSnapshotDigest(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	a := fixture.Ticket("A")
	fixture.WriteIntent(t, r, a)
	pathA := filepath.Join(r.IntentDir, "tickets", "A.json")
	bytesA := a.Encode()
	b := fixture.Ticket("A")
	b.Title = "B version"
	bytesB := b.Encode()
	treeA, err := intent.TreeDigest(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	// Direct: pinned to A, on-disk B.
	fixture.Write(t, pathA, bytesB)
	if _, err := intent.LoadExpecting(r.Root, treeA.Sha256); code(err) != wire.CodeSnapshotMoved {
		t.Fatalf("B under A's digest must be SNAPSHOT_MOVED: %v", err)
	}
	fixture.Write(t, pathA, bytesA)
	if _, err := intent.LoadExpecting(r.Root, treeA.Sha256); err != nil {
		t.Fatalf("A under A's digest: %v", err)
	}
	// Through the protocol: the body swaps A→B, loads, and swaps B→A before
	// the re-probe, so both probes see A. The load must still refuse.
	rd := snapshot.Reader{StateDir: r.StateDir, IntentTree: func() (wire.Digest, error) {
		tr, err := intent.TreeDigest(r.Root)
		if err != nil {
			return "", err
		}
		return tr.Sha256, nil
	}}
	var loadErr error
	bodies := 0
	_, err = rd.Read(func(s *snapshot.Snapshot) error {
		bodies++
		if s.IntentTree != treeA.Sha256 {
			t.Fatalf("probe did not observe A")
		}
		fixture.Write(t, pathA, bytesB)
		_, loadErr = intent.LoadExpecting(r.Root, s.IntentTree)
		fixture.Write(t, pathA, bytesA)
		return loadErr
	})
	if bodies != 1 || code(err) != wire.CodeSnapshotMoved || code(loadErr) != wire.CodeSnapshotMoved {
		t.Errorf("A→B→A must be refused by the pinned load: bodies %d err %v", bodies, err)
	}
}

// TestTMV0007_AS11_PublicationAndDivergence covers the pure TM-V0-007 rules.
func TestTMV0007_AS11_PublicationAndDivergence(t *testing.T) {
	x, y, z := wire.Sum([]byte("x")), wire.Sum([]byte("y")), wire.Sum([]byte("z"))
	cases := []struct {
		file, head, journal wire.Digest
		want                string
	}{
		{x, x, x, intent.Published},
		{x, y, x, intent.Unpublished},
		{x, "", x, intent.Unpublished},
		{x, x, y, intent.Diverged},
		{x, y, z, intent.Diverged},
	}
	for _, c := range cases {
		if got := intent.Publication(c.file, c.head, c.journal); got != c.want {
			t.Errorf("Publication(%s,%s,%s) = %s, want %s", c.file[:6], c.head, c.journal[:6], got, c.want)
		}
	}
	if err := intent.Divergence("p", x, x, nil); err != nil {
		t.Errorf("file = journal: %v", err)
	}
	if err := intent.Divergence("p", x, y, &x); err != nil {
		t.Errorf("file = pending pre: %v", err)
	}
	if err := intent.Divergence("p", x, y, nil); code(err) != wire.CodeIntentDiverged {
		t.Errorf("file ≠ journal, no pending: %v", err)
	}
	if err := intent.Divergence("p", x, y, &z); code(err) != wire.CodeIntentDiverged {
		t.Errorf("file ≠ journal and ≠ pre: %v", err)
	}
}

// TestTMV0002_AS01_QueueAndPolicyRules covers the queue/policy relationship
// checks and the §1 policy caps at load.
func TestTMV0002_AS01_QueueAndPolicyRules(t *testing.T) {
	edit := func(v wire.Value, key string, val wire.Value) []byte {
		v.Obj.Set(key, val)
		return wire.EncodeFile(v)
	}
	if _, err := intent.DecodeQueue(fixture.QueueBytes()); err != nil {
		t.Fatalf("fixture queue: %v", err)
	}
	if _, err := intent.DecodeQueue(edit(fixture.QueueValue(), "repositoryAuthorityId", wire.String("repo:other"))); code(err) != wire.CodeMalformed {
		t.Errorf("authority mismatch: %v", err)
	}
	if _, err := intent.DecodeQueue(edit(fixture.QueueValue(), "foreignAdapterId", wire.String("beamfall"))); code(err) != wire.CodeMalformed {
		t.Errorf("adapter without FOREIGN writer: %v", err)
	}
	if _, err := intent.DecodeQueue(edit(fixture.QueueValue(), "schemaVersion", wire.String("1"))); code(err) != wire.CodeMalformed {
		t.Errorf("schema version: %v", err)
	}
	if _, err := intent.DecodeQueue(edit(fixture.QueueValue(), "profile", wire.String("taskman-queue/1"))); code(err) != wire.CodeUnsupportedVersion {
		t.Errorf("queue version: %v", err)
	}
	if _, err := intent.DecodeQueue(edit(fixture.QueueValue(), "nextSerial", wire.String("4294967296"))); code(err) != wire.CodeMalformed {
		t.Errorf("nextSerial as Size: %v", err)
	}
	if _, err := intent.DecodeQueue([]byte(strings.Repeat("x", wire.MaxQueueFileBytes+1))); code(err) != wire.CodeLimitExceeded {
		t.Errorf("oversized queue: %v", err)
	}
	if _, err := intent.DecodePolicy(fixture.PolicyBytes()); err != nil {
		t.Fatalf("fixture policy: %v", err)
	}
	setCap := func(field, val string) []byte {
		v := fixture.PolicyValue()
		capacity, _ := v.Obj.Get("capacity")
		capacity.Obj.Set(field, wire.String(val))
		return wire.EncodeFile(v)
	}
	if _, err := intent.DecodePolicy(setCap("maxActiveAttempts", "64")); err != nil {
		t.Errorf("maxActiveAttempts at max: %v", err)
	}
	if _, err := intent.DecodePolicy(setCap("maxActiveAttempts", "65")); code(err) != wire.CodeLimitExceeded {
		t.Errorf("maxActiveAttempts over max: %v", err)
	}
	if _, err := intent.DecodePolicy(setCap("maxActiveAttempts", "0")); code(err) != wire.CodeLimitExceeded {
		t.Errorf("maxActiveAttempts below min: %v", err)
	}
	if _, err := intent.DecodePolicy(setCap("maxWorkersTotal", "257")); code(err) != wire.CodeLimitExceeded {
		t.Errorf("maxWorkersTotal over max: %v", err)
	}
	v := fixture.PolicyValue()
	ret, _ := v.Obj.Get("retention")
	ret.Obj.Set("evidenceDays", wire.String("29"))
	if _, err := intent.DecodePolicy(wire.EncodeFile(v)); code(err) != wire.CodeLimitExceeded {
		t.Errorf("evidenceDays below min: %v", err)
	}
	v = fixture.PolicyValue()
	roles := wire.NewObject()
	roles.Set("WORKER", wire.Strings([]string{"HOLD"}))
	v.Obj.Set("roles", wire.ObjectValue(roles))
	if _, err := intent.DecodePolicy(wire.EncodeFile(v)); code(err) != wire.CodeMalformed {
		t.Errorf("policy may not add a role row: %v", err)
	}
	v = fixture.PolicyValue()
	gates, _ := v.Obj.Get("gates")
	gate := gates.Arr[0]
	gate.Obj.Set("env", wire.Strings([]string{"SECRET"}))
	if _, err := intent.DecodePolicy(wire.EncodeFile(v)); code(err) != wire.CodeMalformed {
		t.Errorf("gate env outside allowedEnvKeys: %v", err)
	}
	v = fixture.PolicyValue()
	gates, _ = v.Obj.Get("gates")
	gate = gates.Arr[0]
	gate.Obj.Set("timeoutSeconds", wire.String("7201"))
	if _, err := intent.DecodePolicy(wire.EncodeFile(v)); code(err) != wire.CodeLimitExceeded {
		t.Errorf("gate timeout over max: %v", err)
	}
}
