package journal

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func object(kv ...any) wire.Value {
	o := wire.NewObject()
	for i := 0; i < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}
func str(s string) wire.Value { return wire.String(s) }
func setup(t *testing.T) (*fixture.Repo, Reader) {
	t.Helper()
	repo := fixture.TempRepo(t)
	fixture.WriteIntent(t, repo)
	fixture.WriteState(t, repo)
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	return repo, Reader{Source: Native{StateDir: repo.StateDir, PrimaryWorktree: repo.Root}, QueueID: q, PrimaryWorktree: repo.Root}
}
func read(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func value(t *testing.T, p string) wire.Value {
	t.Helper()
	v, err := wire.Parse(read(t, p))
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func remove(t *testing.T, p string) {
	t.Helper()
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
}
func requireCode(t *testing.T, err error, want string) {
	t.Helper()
	if wire.CodeOf(err) != want {
		t.Fatalf("got %v; want %s", err, want)
	}
}

// rewrite changes fixture receipt bytes and recomputes head linkage; semantic
// counterexamples therefore cannot pass/fail merely on a stale outer hash.
func rewrite(t *testing.T, repo *fixture.Repo, seq uint64, edit func(wire.Value)) {
	t.Helper()
	name, _ := snapshot.ReceiptName(seq)
	path := filepath.Join(repo.StateDir, "receipts", name)
	v := value(t, path)
	edit(v)
	raw := wire.EncodeFile(v)
	fixture.Write(t, path, raw)
	hp := filepath.Join(repo.StateDir, "head.json")
	h := value(t, hp)
	last, _ := h.Obj.Get("lastSeq")
	if last.Str == string(wire.SizeOf(seq)) {
		h.Obj.Set("lastReceiptSha256", str(string(wire.Sum(raw))))
	}
	if seq == 1 {
		h.Obj.Set("initSha256", str(string(wire.Sum(raw))))
	}
	fixture.Write(t, hp, wire.EncodeFile(h))
}
func postSet(v wire.Value, posts map[string][]byte, pres map[string]*wire.Digest, blob bool) {
	var paths []string
	for p := range posts {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var before, after []wire.Value
	for _, p := range paths {
		pre := wire.Null()
		if pres[p] != nil {
			pre = str(string(*pres[p]))
		}
		before = append(before, object("path", str(p), "sha256", pre))
		raw := posts[p]
		rec := wire.Null()
		bd := wire.Null()
		d := wire.Null()
		if raw != nil {
			d = str(string(wire.Sum(raw)))
			if blob {
				bd = d
			} else {
				rec, _ = wire.Parse(raw)
			}
		}
		after = append(after, object("path", str(p), "sha256", d, "record", rec, "blobSha256", bd))
	}
	v.Obj.Set("pre", wire.Array(before...))
	v.Obj.Set("post", wire.Array(after...))
}
func appendReceipt(t *testing.T, repo *fixture.Repo, kind string, posts map[string][]byte, request string, project, advance, blob bool) uint64 {
	t.Helper()
	hp := filepath.Join(repo.StateDir, "head.json")
	h, err := snapshot.DecodeHead(read(t, hp))
	if err != nil {
		t.Fatal(err)
	}
	seq := h.LastSeq.Uint64() + 1
	v := fixture.ReceiptValue(seq, h.LastReceiptSha256, kind, h.Generation.Uint64())
	if request != "" {
		v.Obj.Set("requestId", str(request))
	}
	pres := map[string]*wire.Digest{}
	for p, raw := range posts {
		dest := filepath.Join(repo.StateDir, p)
		if strings.HasPrefix(p, "intent/") {
			dest = filepath.Join(repo.IntentDir, strings.TrimPrefix(p, "intent/"))
		}
		pre, err := os.ReadFile(dest)
		if err == nil {
			d := wire.Sum(pre)
			pres[p] = &d
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if blob && raw != nil {
			fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(wire.Sum(raw))), raw)
		}
		if project {
			if raw == nil {
				remove(t, dest)
			} else {
				fixture.Write(t, dest, raw)
			}
		}
	}
	postSet(v, posts, pres, blob)
	raw := wire.EncodeFile(v)
	name, _ := snapshot.ReceiptName(seq)
	fixture.Write(t, filepath.Join(repo.StateDir, "receipts", name), raw)
	if advance {
		fixture.Write(t, hp, wire.EncodeFile(fixture.HeadValue(repo.Root, seq, wire.Sum(raw), h.Generation.Uint64(), h.InitSha256)))
	}
	return seq
}
func requestBytes(id string, seq uint64, digest wire.Digest, refused bool) []byte {
	s := wire.SizeOf(seq)
	out := mutation.Outcome{RequestID: id, Outcome: mutation.OutcomeCompleted, ReceiptSeq: &s, Codes: []string{}}
	if refused {
		out.Outcome = mutation.OutcomeBlocked
		out.ResultingRevision = nil
		out.ResultingAcceptanceRevision = nil
		out.Codes = []string{wire.CodePaused}
	}
	return wire.EncodeFile(object("requestId", str(id), "seq", str(string(s)), "mutationSha256", str(string(digest)), "outcome", out.Value()))
}

func TestTMV0007_AS08_CanonicalLatest(t *testing.T) {
	repo, r := setup(t)
	result, err := r.Audit("intent/queue.json")
	if err != nil {
		t.Fatal(err)
	}
	if result.LastSeq != "1" || result.StructuralConsistency != "CONSISTENT" || result.ProjectionAgreement != "AGREES" || result.HistoricalAcceptance != "NOT_OBSERVED" || result.RuntimeQualification != "NOT_OBSERVED" {
		t.Fatalf("%+v", result)
	}
	tk := fixture.Ticket("A")
	appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": tk.Encode()}, "", true, true, false)
	tk.Title = "latest"
	appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": tk.Encode()}, "", true, true, true)
	result, err = r.Audit("intent/tickets/A.json")
	if err != nil {
		t.Fatal(err)
	}
	rawReceipt, readErr := os.ReadFile(filepath.Join(repo.StateDir, "receipts", "000000000003.json"))
	if readErr != nil || result.LastReceiptSha256 != wire.Sum(rawReceipt) {
		t.Fatalf("terminal digest not bound: %v", readErr)
	}
	if result.LastSeq != "3" || result.Records["intent/tickets/A.json"].Seq != "3" || !bytes.Equal(result.Records["intent/tickets/A.json"].Raw, tk.Encode()) {
		t.Fatalf("wrong latest: %+v", result)
	}
	fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "A.json"), fixture.Ticket("A").Encode())
	_, err = r.Audit()
	requireCode(t, err, wire.CodeIntentDiverged)
}

func TestTMV0009_AS11_GenesisProofCounterexamples(t *testing.T) {
	edits := map[string]func(wire.Value){
		"missing VERSION": func(v wire.Value) { filterPost(v, "VERSION") },
		"extra ticket":    func(v wire.Value) { addPost(v, "intent/tickets/A.json", fixture.Ticket("A").Encode()) },
		"missing policy":  func(v wire.Value) { filterPost(v, "intent/policy.json") },
		"missing descriptor": func(v wire.Value) {
			posts, _ := v.Obj.Get("post")
			for _, p := range posts.Arr {
				path, _ := p.Obj.Get("path")
				if strings.HasPrefix(path.Str, "pinned/") {
					filterPost(v, path.Str)
					return
				}
			}
		},
		"initial pre": func(v wire.Value) {
			pre, _ := v.Obj.Get("pre")
			pre.Arr[0].Obj.Set("sha256", str(string(wire.Sum([]byte("old")))))
		},
		"wrong descriptor primary": func(v wire.Value) { alterDescriptor(v, "primaryWorktree", str("/different")) },
		"wrong descriptor version": func(v wire.Value) { alterDescriptor(v, "versionSha256", str(string(wire.Sum([]byte("other"))))) },
		"wrong descriptor queue":   func(v wire.Value) { alterDescriptor(v, "queueId", str("queue:other:q")) },
	}
	for name, edit := range edits {
		t.Run(name, func(t *testing.T) {
			repo, r := setup(t)
			rewrite(t, repo, 1, edit)
			remove(t, filepath.Join(repo.StateDir, "head.json"))
			_, err := r.Audit()
			if err == nil || wire.CodeOf(err) == wire.CodeRedoPending || wire.CodeOf(err) == wire.CodeUninitialized {
				t.Fatalf("bad genesis classified as usable/pending: %v", err)
			}
		})
	}
	for _, name := range []string{"missing blob", "corrupt blob", "head init hash", "head version", "descriptor path"} {
		t.Run(name, func(t *testing.T) {
			repo, r := setup(t)
			switch name {
			case "missing blob":
				remove(t, filepath.Join(repo.StateDir, "evidence", string(wire.Sum([]byte(snapshot.VersionBytes)))))
				remove(t, filepath.Join(repo.StateDir, "head.json"))
			case "corrupt blob":
				fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(wire.Sum([]byte(snapshot.VersionBytes)))), []byte("wrong\n"))
			case "head init hash", "head version":
				hp := filepath.Join(repo.StateDir, "head.json")
				h := value(t, hp)
				field := "initSha256"
				if name == "head version" {
					field = "versionSha256"
				}
				h.Obj.Set(field, str(string(wire.Sum([]byte("wrong")))))
				fixture.Write(t, hp, wire.EncodeFile(h))
			case "descriptor path":
				rewrite(t, repo, 1, func(v wire.Value) {
					pre, _ := v.Obj.Get("pre")
					post, _ := v.Obj.Get("post")
					for i, p := range post.Arr {
						path, _ := p.Obj.Get("path")
						if strings.HasPrefix(path.Str, "pinned/") {
							bad := str("pinned/" + strings.Repeat("0", 64) + ".json")
							p.Obj.Set("path", bad)
							pre.Arr[i].Obj.Set("path", bad)
						}
					}
				})
			}
			_, err := r.Audit()
			if err == nil || wire.CodeOf(err) == wire.CodeRedoPending {
				t.Fatalf("invalid genesis: %v", err)
			}
		})
	}
}
func filterPost(v wire.Value, path string) {
	pre, _ := v.Obj.Get("pre")
	post, _ := v.Obj.Get("post")
	var a, b []wire.Value
	for i, p := range post.Arr {
		pv, _ := p.Obj.Get("path")
		if pv.Str != path {
			a = append(a, pre.Arr[i])
			b = append(b, p)
		}
	}
	v.Obj.Set("pre", wire.Array(a...))
	v.Obj.Set("post", wire.Array(b...))
}
func addPost(v wire.Value, path string, raw []byte) {
	pre, _ := v.Obj.Get("pre")
	post, _ := v.Obj.Get("post")
	rv, _ := wire.Parse(raw)
	pre.Arr = append(pre.Arr, object("path", str(path), "sha256", wire.Null()))
	post.Arr = append(post.Arr, object("path", str(path), "sha256", str(string(wire.Sum(raw))), "record", rv, "blobSha256", wire.Null()))
	sort.Slice(pre.Arr, func(i, j int) bool {
		a, _ := pre.Arr[i].Obj.Get("path")
		b, _ := pre.Arr[j].Obj.Get("path")
		return a.Str < b.Str
	})
	sort.Slice(post.Arr, func(i, j int) bool {
		a, _ := post.Arr[i].Obj.Get("path")
		b, _ := post.Arr[j].Obj.Get("path")
		return a.Str < b.Str
	})
	v.Obj.Set("pre", pre)
	v.Obj.Set("post", post)
}
func alterDescriptor(v wire.Value, field string, val wire.Value) {
	posts, _ := v.Obj.Get("post")
	var old string
	var raw []byte
	for _, p := range posts.Arr {
		path, _ := p.Obj.Get("path")
		if strings.HasPrefix(path.Str, "pinned/") {
			old = path.Str
			rec, _ := p.Obj.Get("record")
			rec.Obj.Set(field, val)
			raw = wire.EncodeFile(rec)
		}
	}
	filterPost(v, old)
	addPost(v, "pinned/"+string(wire.Sum(raw))+".json", raw)
}

func TestTMV0009_AS11_ChainInventoryAndPending(t *testing.T) {
	for _, kind := range []string{"plus3", "gap", "seq", "prev", "generation", "late INIT", "pending", "two pending", "head ahead"} {
		t.Run(kind, func(t *testing.T) {
			repo, r := setup(t)
			fixture.Commit(t, repo, "MUTATION")
			switch kind {
			case "plus3":
				fixture.PlantReceipt(t, repo, 5)
			case "gap":
				remove(t, filepath.Join(repo.StateDir, "receipts", "000000000001.json"))
			case "seq":
				rewrite(t, repo, 2, func(v wire.Value) { v.Obj.Set("seq", str("9")) })
			case "prev":
				rewrite(t, repo, 2, func(v wire.Value) { v.Obj.Set("prev", str(string(wire.Sum([]byte("wrong"))))) })
			case "generation":
				rewrite(t, repo, 2, func(v wire.Value) { v.Obj.Set("headGeneration", str("2")) })
			case "late INIT":
				rewrite(t, repo, 2, func(v wire.Value) { v.Obj.Set("kind", str("INIT")) })
			case "pending":
				fixture.PlantReceipt(t, repo, 3)
			case "two pending":
				fixture.PlantReceipt(t, repo, 3)
				fixture.PlantReceipt(t, repo, 4)
			case "head ahead":
				remove(t, filepath.Join(repo.StateDir, "receipts", "000000000002.json"))
			}
			_, err := r.Audit()
			want := wire.CodeJournalForked
			if kind == "late INIT" {
				want = wire.CodeMalformed
			}
			if kind == "pending" {
				want = wire.CodeRedoPending
			}
			requireCode(t, err, want)
		})
	}
	repo, r := setup(t)
	remove(t, filepath.Join(repo.StateDir, "head.json"))
	result, err := r.Audit()
	requireCode(t, err, wire.CodeRedoPending)
	if !result.Pending || result.StructuralConsistency != "CONSISTENT" {
		t.Fatal("unproved pending")
	}
	remove(t, filepath.Join(repo.StateDir, "receipts", "000000000001.json"))
	fixture.Write(t, filepath.Join(repo.StateDir, "receipts", "000000000001.tmp-1"), []byte("not committed"))
	result, err = r.Audit()
	requireCode(t, err, wire.CodeUninitialized)
	if !result.StagingPresent {
		t.Fatal("staging hidden")
	}
}

func TestTMV0006_AS03_RequestBindingReplayAndErrors(t *testing.T) {
	for _, refusal := range []bool{false, true} {
		t.Run(fmt.Sprint(refusal), func(t *testing.T) {
			repo, r := setup(t)
			id := strings.Repeat("r", 64)
			digest := wire.Sum([]byte("exact canonical mutation bytes\n"))
			path, _ := snapshot.RequestPath(id)
			raw := requestBytes(id, 2, digest, refusal)
			appendReceipt(t, repo, "MUTATION", map[string][]byte{path: raw}, id, true, true, false)
			if refusal {
				rewrite(t, repo, 2, func(v wire.Value) {
					v.Obj.Set("outcome", str(mutation.OutcomeBlocked))
					v.Obj.Set("codes", wire.Strings([]string{wire.CodePaused}))
				})
			}
			index := &RequestIndex{Reader: r}
			entry, ok, err := index.Lookup(id)
			if err != nil || !ok {
				t.Fatalf("lookup: %v %v", ok, err)
			}
			expected, err := snapshot.DecodeRequest(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(entry, expected.Entry) || index.Identity.HeadSha256 == "" {
				t.Fatalf("outcome lost: %+v", entry)
			}
			if _, ok, err := index.Lookup("absent"); err != nil || ok {
				t.Fatalf("absence: %v %v", ok, err)
			}
			// The adapter re-audits after successful construction/use; I/O failure
			// must propagate, including for a previously verified absent request.
			boom := errors.New("injected EIO after initial audit")
			index.Reader.Source = failingSource{Source: r.Source, path: path, err: boom}
			if _, ok, err := index.Lookup("absent"); !errors.Is(err, boom) || ok {
				t.Fatalf("I/O became absence: %v %v", ok, err)
			}
			index.Reader = r
			remove(t, filepath.Join(repo.StateDir, path))
			_, _, err = index.Lookup(id)
			requireCode(t, err, wire.CodeJournalForked)
			fixture.Write(t, filepath.Join(repo.StateDir, path), requestBytes(id, 2, wire.Sum([]byte("altered")), refusal))
			_, _, err = index.Lookup(id)
			requireCode(t, err, wire.CodeJournalForked)
		})
	}
}

type failingSource struct {
	Source
	path string
	err  error
}

func (s failingSource) Read(p string, n int) ([]byte, error) {
	if p == s.path {
		return nil, s.err
	}
	return s.Source.Read(p, n)
}

type nilEmptySource struct{ Source }

func (s nilEmptySource) Read(p string, n int) ([]byte, error) {
	raw, err := s.Source.Read(p, n)
	if err == nil && len(raw) == 0 {
		return nil, nil
	}
	return raw, err
}

func TestTMV0002_AS10_EmptyRawEvidencePreservesPresence(t *testing.T) {
	emptyDigest := wire.Sum(nil)
	emptyPath := "evidence/" + string(emptyDigest)

	t.Run("valid selected empty is not deletion and request audit accepts it", func(t *testing.T) {
		repo, r := setup(t)
		appendReceipt(t, repo, "REVIEW", map[string][]byte{emptyPath: {}}, "", true, true, true)
		r.Source = nilEmptySource{Source: r.Source}

		result, err := r.Audit(emptyPath)
		if err != nil {
			t.Fatal(err)
		}
		record, ok := result.Records[emptyPath]
		if !ok || record.Sha256 == nil || *record.Sha256 != emptyDigest || record.Raw == nil || len(record.Raw) != 0 {
			t.Fatalf("selected empty evidence lost presence: %+v, present=%v", record, ok)
		}
		index := RequestIndex{Reader: r}
		if _, found, err := index.Lookup("absent"); err != nil || found {
			t.Fatalf("request audit did not preserve empty evidence: found=%v err=%v", found, err)
		}
	})

	t.Run("selected deletion remains nil", func(t *testing.T) {
		repo, r := setup(t)
		barrier := wire.EncodeFile(object("profile", str(snapshot.ProfileBarrier), "queueId", str(fixture.QueueID), "scope", str("ADMISSION"), "reason", str("OPERATOR"), "actor", str(fixture.Actor), "sinceSeq", str("2"), "since", str(fixture.Timestamp)))
		appendReceipt(t, repo, "PAUSE", map[string][]byte{"barrier.json": barrier}, "", true, true, false)
		appendReceipt(t, repo, "UNPAUSE", map[string][]byte{"barrier.json": nil}, "", true, true, false)

		result, err := r.Audit("barrier.json")
		if err != nil {
			t.Fatal(err)
		}
		record, ok := result.Records["barrier.json"]
		if !ok || record.Sha256 != nil || record.Raw != nil {
			t.Fatalf("deletion became present bytes: %+v, present=%v", record, ok)
		}
	})

	t.Run("pending empty post remains present", func(t *testing.T) {
		repo, r := setup(t)
		appendReceipt(t, repo, "REVIEW", map[string][]byte{emptyPath: {}}, "", true, false, true)
		r.Source = nilEmptySource{Source: r.Source}

		result, err := r.Audit(emptyPath)
		requireCode(t, err, wire.CodeRedoPending)
		record, ok := result.Records[emptyPath]
		if !result.Pending || !ok || record.Sha256 == nil || *record.Sha256 != emptyDigest || record.Raw == nil || len(record.Raw) != 0 {
			t.Fatalf("pending empty post lost presence: result=%+v record=%+v present=%v", result, record, ok)
		}
	})

	t.Run("missing retained empty evidence refuses", func(t *testing.T) {
		repo, r := setup(t)
		appendReceipt(t, repo, "REVIEW", map[string][]byte{emptyPath: {}}, "", true, true, true)
		remove(t, filepath.Join(repo.StateDir, emptyPath))
		_, err := r.Audit(emptyPath)
		requireCode(t, err, wire.CodeJournalForked)
	})

	t.Run("empty structured blob remains malformed", func(t *testing.T) {
		repo, r := setup(t)
		appendReceipt(t, repo, "PAUSE", map[string][]byte{"barrier.json": {}}, "", true, true, true)
		r.Source = nilEmptySource{Source: r.Source}
		_, err := r.Audit("barrier.json")
		requireCode(t, err, wire.CodeMalformed)
	})
}

func TestTMV0006_AS03_RequestCounterexamples(t *testing.T) {
	for _, kind := range []string{"65 bytes", "wrong filename", "missing post", "null request", "sequence", "outcome", "codes", "replayed", "duplicate history", "non-null revisions on refusal"} {
		t.Run(kind, func(t *testing.T) {
			repo, r := setup(t)
			id := "r1"
			path, _ := snapshot.RequestPath(id)
			raw := requestBytes(id, 2, wire.Sum([]byte("mutation")), false)
			appendReceipt(t, repo, "MUTATION", map[string][]byte{path: raw}, id, true, true, false)
			switch kind {
			case "65 bytes":
				rewrite(t, repo, 2, func(v wire.Value) { v.Obj.Set("requestId", str(strings.Repeat("r", 65))) })
			case "missing post":
				rewrite(t, repo, 2, func(v wire.Value) { filterPost(v, path) })
			case "null request":
				rewrite(t, repo, 2, func(v wire.Value) { v.Obj.Set("requestId", wire.Null()) })
			case "duplicate history":
				appendReceipt(t, repo, "MUTATION", map[string][]byte{path: requestBytes(id, 3, wire.Sum([]byte("mutation")), false)}, id, true, true, false)
			default:
				rewrite(t, repo, 2, func(v wire.Value) {
					posts, _ := v.Obj.Get("post")
					p := posts.Arr[0]
					rec, _ := p.Obj.Get("record")
					out, _ := rec.Obj.Get("outcome")
					switch kind {
					case "wrong filename":
						other, _ := snapshot.RequestPath("different")
						pre, _ := v.Obj.Get("pre")
						pre.Arr[0].Obj.Set("path", str(other))
						p.Obj.Set("path", str(other))
					case "sequence":
						out.Obj.Set("receiptSeq", str("3"))
					case "outcome":
						v.Obj.Set("outcome", str("BLOCKED"))
					case "codes":
						v.Obj.Set("codes", wire.Strings([]string{wire.CodePaused}))
					case "replayed":
						out.Obj.Set("replayed", wire.Bool(true))
					case "non-null revisions on refusal":
						out.Obj.Set("outcome", str("BLOCKED"))
						out.Obj.Set("resultingRevision", str("1"))
					}
					p.Obj.Set("sha256", str(string(wire.Sum(wire.EncodeFile(rec)))))
				})
			}
			_, err := r.Audit()
			if err == nil {
				t.Fatal("invalid request proof accepted")
			}
		})
	}
}

func TestTMV0016_AS27_DeletionRedoAndAudit(t *testing.T) {
	pre := wire.Sum([]byte("pre"))
	third := wire.Sum([]byte("third"))
	for _, tc := range []struct {
		current *wire.Digest
		want    string
		code    string
	}{{nil, "ALREADY_APPLIED", ""}, {&pre, "DELETE_ELIGIBLE", ""}, {&third, "", wire.CodeJournalForked}} {
		got, err := DeleteRedo(pre, tc.current)
		if got != tc.want {
			t.Fatal(got)
		}
		requireCode(t, err, tc.code)
	}
	repo, r := setup(t)
	barrier := wire.EncodeFile(object("profile", str(snapshot.ProfileBarrier), "queueId", str(fixture.QueueID), "scope", str("ADMISSION"), "reason", str("OPERATOR"), "actor", str(fixture.Actor), "sinceSeq", str("2"), "since", str(fixture.Timestamp)))
	appendReceipt(t, repo, "PAUSE", map[string][]byte{"barrier.json": barrier}, "", true, true, false)
	appendReceipt(t, repo, "UNPAUSE", map[string][]byte{"barrier.json": nil}, "", false, false, false)
	_, err := r.Audit()
	requireCode(t, err, wire.CodeRedoPending)
	remove(t, filepath.Join(repo.StateDir, "barrier.json"))
	_, err = r.Audit()
	requireCode(t, err, wire.CodeRedoPending)
	fixture.Write(t, filepath.Join(repo.StateDir, "barrier.json"), []byte("third\n"))
	_, err = r.Audit()
	requireCode(t, err, wire.CodeJournalForked)
}

func TestTMV0008_AS07_LedgerReadPurity(t *testing.T) {
	for _, state := range []string{"normal", "pending", "pending genesis", "staging", "forked"} {
		t.Run(state, func(t *testing.T) {
			repo, r := setup(t)
			switch state {
			case "pending":
				fixture.PlantReceipt(t, repo, 2)
			case "pending genesis":
				remove(t, filepath.Join(repo.StateDir, "head.json"))
			case "staging":
				remove(t, filepath.Join(repo.StateDir, "head.json"))
				remove(t, filepath.Join(repo.StateDir, "receipts", "000000000001.json"))
			case "forked":
				fixture.PlantReceipt(t, repo, 4)
			}
			before := fixture.TreeSnapshot(t, repo.StateDir)
			intentBefore := fixture.TreeSnapshot(t, repo.IntentDir)
			_, _ = r.Audit("intent/queue.json")
			index := RequestIndex{Reader: r}
			_, _, _ = index.Lookup("request")
			fixture.AssertUntouched(t, repo, before, intentBefore, "journal audit and request lookup")
		})
	}
}

// Modest generated fixtures with reduced internal limits exercise streaming
// without allocating a million receipts. Reader results retain no receipt bytes.
func TestTMV0002_AS10_LedgerBoundsAndStreaming(t *testing.T) {
	repo, r := setup(t)
	for i := 0; i < 40; i++ {
		fixture.Commit(t, repo, "MUTATION")
	}
	result, err := r.Audit()
	if err != nil {
		t.Fatal(err)
	}
	if result.LastSeq != "41" || len(result.Records) != 0 {
		t.Fatalf("retained unwanted data: %+v", result)
	}
	_, err = r.audit(nil, "", limits{scan: 20, selected: wire.MaxIntentTreeBytes}, true)
	requireCode(t, err, wire.CodeLimitExceeded)
	_, err = r.audit([]string{"intent/policy.json"}, "", limits{scan: 100, selected: 32}, true)
	requireCode(t, err, wire.CodeLimitExceeded)
	// Historical blob bytes are mandatory even if a later post supersedes them.
	tk := fixture.Ticket("A")
	appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": tk.Encode()}, "", true, true, true)
	old := wire.Sum(tk.Encode())
	tk.Title = "new"
	appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": tk.Encode()}, "", true, true, false)
	remove(t, filepath.Join(repo.StateDir, "evidence", string(old)))
	_, err = r.Audit()
	requireCode(t, err, wire.CodeJournalForked)
}

type movingSource struct {
	Source
	repo  *fixture.Repo
	t     *testing.T
	reads int
}

func (s *movingSource) Read(p string, n int) ([]byte, error) {
	if p == "receipts/000000000001.json" {
		s.reads++
		fixture.Commit(s.t, s.repo, "MUTATION")
		return nil, errors.New("body failed across movement")
	}
	return s.Source.Read(p, n)
}
func TestTMV0008_AS36_LedgerMovementAndBodyFailure(t *testing.T) {
	repo, r := setup(t)
	s := &movingSource{Source: r.Source, repo: repo, t: t}
	r.Source = s
	_, err := r.Audit()
	requireCode(t, err, wire.CodeSnapshotMoved)
	if s.reads != 4 {
		t.Fatalf("retries %d", s.reads)
	}
}

func TestTMV0002_AS01_UnknownSemanticCoverage(t *testing.T) {
	repo, r := setup(t)
	raw := []byte("{\"profile\":\"future-plan/9\",\"queueId\":\"queue:acme:main\"}\n")
	path := "pinned/" + string(wire.Sum(raw)) + ".json"
	appendReceipt(t, repo, "CONFIG_PIN", map[string][]byte{path: raw}, "", true, true, true)
	result, err := r.Audit()
	if err != nil {
		t.Fatal(err)
	}
	if result.SemanticCoverage != "UNKNOWN" || result.HistoricalAcceptance != "NOT_OBSERVED" {
		t.Fatalf("overclaim: %+v", result)
	}
}

// This observes reachable heap during the stream after forced collection,
// not total allocation churn or a qualified production RSS budget.
type heapSource struct {
	Source
	count      int
	base, peak uint64
}

func (s *heapSource) Read(p string, n int) ([]byte, error) {
	if strings.HasPrefix(p, "receipts/") {
		s.count++
		if s.count%8 == 0 {
			runtime.GC()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			if s.base == 0 {
				s.base = m.HeapAlloc
			}
			if m.HeapAlloc > s.peak {
				s.peak = m.HeapAlloc
			}
		}
	}
	return s.Source.Read(p, n)
}
func TestTMV0002_AS10_ReceiptBytesAreStreamed(t *testing.T) {
	repo, r := setup(t)
	for i := 0; i < 96; i++ {
		tk := fixture.Ticket("A")
		body := fmt.Sprintf("%03d", i) + strings.Repeat("x", 48*wire.KiB)
		tk.Body = &body
		appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": tk.Encode()}, "", true, true, false)
	}
	source := &heapSource{Source: r.Source}
	r.Source = source
	result, err := r.Audit()
	if err != nil {
		t.Fatal(err)
	}
	if source.count != 97 || len(result.Records) != 0 {
		t.Fatalf("unexpected stream shape: %d receipts, %d selected", source.count, len(result.Records))
	}
	if source.peak > source.base+2*wire.MiB {
		t.Fatalf("reachable heap grew %d bytes across >4MiB receipt history; expected bounded streaming", source.peak-source.base)
	}
	t.Logf("97 receipts, >4 MiB historical afterimages; observed reachable-heap growth %d bytes", source.peak-source.base)
}

func TestTMV0002_AS10_NativeEnumerationStopsBeforeInspectingFlood(t *testing.T) {
	repo, r := setup(t)
	dir := filepath.Join(repo.StateDir, "receipts")
	for i := 0; i < 300; i++ {
		fixture.Write(t, filepath.Join(dir, fmt.Sprintf("bad-%03d", i)), []byte("x"))
	}
	_, err := r.audit(nil, "", limits{scan: 100, selected: wire.MaxIntentTreeBytes}, true)
	requireCode(t, err, wire.CodeLimitExceeded)
}

func TestTMV0009_AS11_QueueScopeAndGenerationDecrease(t *testing.T) {
	for _, kind := range []string{"receipt ticket", "receipt attempt", "attempt filename", "decreasing generation", "later descriptor"} {
		t.Run(kind, func(t *testing.T) {
			repo, r := setup(t)
			switch kind {
			case "receipt ticket", "receipt attempt":
				fixture.Commit(t, repo, "MUTATION")
				rewrite(t, repo, 2, func(v wire.Value) {
					if kind == "receipt ticket" {
						v.Obj.Set("ticketId", str("ticket:other:q:A"))
					} else {
						v.Obj.Set("attemptId", str("attempt:other:q:"+strings.Repeat("a", 32)))
					}
				})
			case "attempt filename":
				raw := []byte("{\"profile\":\"future-attempt/1\"}\n")
				appendReceipt(t, repo, "ADMIT", map[string][]byte{"attempts/attempt:other:q:" + strings.Repeat("a", 32) + ".json": raw}, "", true, true, false)
			case "decreasing generation":
				fixture.Commit(t, repo, "MUTATION")
				rewrite(t, repo, 2, func(v wire.Value) { v.Obj.Set("headGeneration", str("2")) })
				hp := filepath.Join(repo.StateDir, "head.json")
				h := value(t, hp)
				h.Obj.Set("generation", str("2"))
				fixture.Write(t, hp, wire.EncodeFile(h))
				fixture.Commit(t, repo, "MUTATION")
				rewrite(t, repo, 3, func(v wire.Value) { v.Obj.Set("headGeneration", str("1")) })
				h = value(t, hp)
				h.Obj.Set("generation", str("1"))
				fixture.Write(t, hp, wire.EncodeFile(h))
			case "later descriptor":
				d := snapshot.Init{QueueID: r.QueueID, PrimaryWorktree: repo.Root, VersionSha256: wire.Sum([]byte(snapshot.VersionBytes))}
				raw := wire.EncodeFile(d.Value())
				appendReceipt(t, repo, "CONFIG_PIN", map[string][]byte{"pinned/" + string(wire.Sum(raw)) + ".json": raw}, "", true, true, false)
			}
			_, err := r.Audit()
			requireCode(t, err, wire.CodeJournalForked)
		})
	}
}

func TestTMV0002_AS10_PostBlobSizeAndNativeSymlinkRefusal(t *testing.T) {
	t.Run("blob destination bound", func(t *testing.T) {
		repo, r := setup(t)
		raw := []byte("{\"body\":\"" + strings.Repeat("x", wire.MaxBarrierBytes) + "\"}\n")
		appendReceipt(t, repo, "PAUSE", map[string][]byte{"barrier.json": raw}, "", false, false, true)
		_, err := r.Audit()
		requireCode(t, err, wire.CodeLimitExceeded)
	})
	for _, target := range []string{"receipt", "blob", "request directory"} {
		t.Run(target, func(t *testing.T) {
			repo, r := setup(t)
			path := filepath.Join(repo.StateDir, "receipts", "000000000001.json")
			if target == "blob" {
				path = filepath.Join(repo.StateDir, "evidence", string(wire.Sum([]byte(snapshot.VersionBytes))))
			}
			if target == "request directory" {
				path = filepath.Join(repo.StateDir, "requests")
			} else {
				remove(t, path)
			}
			if err := os.Symlink(repo.IntentDir, path); err != nil {
				t.Fatal(err)
			}
			_, err := r.Audit()
			requireCode(t, err, wire.CodeUnsupportedFilesystem)
		})
	}
}

func TestTMV0007_AS35_PhysicalPreIsNotCanonicalPredecessor(t *testing.T) {
	repo, r := setup(t)
	canonical := fixture.Ticket("A")
	path := "intent/tickets/A.json"
	appendReceipt(t, repo, "MUTATION", map[string][]byte{path: canonical.Encode()}, "", true, true, false)
	c := wire.Sum(canonical.Encode())
	edited := fixture.Ticket("A")
	edited.Title = "human D"
	d := wire.Sum(edited.Encode())
	fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "A.json"), edited.Encode())
	adopted := fixture.Ticket("A")
	adopted.Title = "adopted C-prime"
	adopted.Revision = "2"
	adopted.PreviousRecordSha256 = &c
	appendReceipt(t, repo, "RECONCILE", map[string][]byte{path: adopted.Encode()}, "", true, true, false)
	rc, err := snapshot.DecodeReceipt(read(t, filepath.Join(repo.StateDir, "receipts", "000000000003.json")))
	if err != nil {
		t.Fatal(err)
	}
	if rc.Pre[0].Sha256 == nil || *rc.Pre[0].Sha256 != d || d == c {
		t.Fatal("fixture did not distinguish physical and canonical pre")
	}
	result, err := r.Audit(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ticket.Decode(result.Records[path].Raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.PreviousRecordSha256 == nil || *got.PreviousRecordSha256 != c || result.HistoricalAcceptance != "NOT_OBSERVED" {
		t.Fatal("canonical predecessor/coverage lost")
	}
}

func TestTMV0006_AS03_IndexedInitAndExactTicketRevisions(t *testing.T) {
	t.Run("indexed minimal INIT", func(t *testing.T) {
		repo, r := setup(t)
		path, _ := snapshot.RequestPath("init")
		raw := requestBytes("init", 1, wire.Sum([]byte("INIT request")), false)
		rewrite(t, repo, 1, func(v wire.Value) { v.Obj.Set("requestId", str("init")); addPost(v, path, raw) })
		fixture.Write(t, filepath.Join(repo.StateDir, path), raw)
		result, err := r.Audit()
		if err != nil {
			t.Fatal(err)
		}
		if result.LastSeq != "1" {
			t.Fatal(result)
		}
		index := RequestIndex{Reader: r}
		entry, ok, err := index.Lookup("init")
		if err != nil || !ok || entry.Outcome.ResultingRevision != nil || entry.Outcome.ResultingAcceptanceRevision != nil || *entry.Outcome.ReceiptSeq != "1" {
			t.Fatalf("INIT outcome: %+v %v", entry, err)
		}
	})
	for _, kind := range []string{"exact", "mixed nullness", "ticket without revisions", "missing target", "99/99", "admin with revisions"} {
		t.Run(kind, func(t *testing.T) {
			repo, r := setup(t)
			base := fixture.Ticket("A")
			targetPath := "intent/tickets/A.json"
			appendReceipt(t, repo, "MUTATION", map[string][]byte{targetPath: base.Encode()}, "", true, true, false)
			previous := wire.Sum(base.Encode())
			base.Revision = "2"
			base.PreviousRecordSha256 = &previous
			reqPath, _ := snapshot.RequestPath("ticket-update")
			raw := requestBytes("ticket-update", 3, wire.Sum([]byte("update")), false)
			v, err := wire.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			out, _ := v.Obj.Get("outcome")
			if kind != "ticket without revisions" {
				out.Obj.Set("resultingRevision", str("2"))
				out.Obj.Set("resultingAcceptanceRevision", str("1"))
			}
			if kind == "mixed nullness" {
				out.Obj.Set("resultingAcceptanceRevision", wire.Null())
			}
			if kind == "99/99" {
				out.Obj.Set("resultingRevision", str("99"))
				out.Obj.Set("resultingAcceptanceRevision", str("99"))
			}
			posts := map[string][]byte{reqPath: wire.EncodeFile(v), targetPath: base.Encode()}
			if kind == "missing target" {
				delete(posts, targetPath)
			}
			appendReceipt(t, repo, "MUTATION", posts, "ticket-update", true, true, false)
			if kind != "admin with revisions" {
				rewrite(t, repo, 3, func(v wire.Value) { v.Obj.Set("ticketId", str(base.TicketID.Raw)) })
			}
			_, err = r.Audit()
			if kind == "exact" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("unbound revision assertion accepted")
			}
		})
	}
}

func TestTMV0006_AS35_IndexDuringStableIntentDivergence(t *testing.T) {
	for _, corruption := range []string{"missing", "corrupt"} {
		t.Run(corruption, func(t *testing.T) {
			repo, r := setup(t)
			canonical := fixture.Ticket("A")
			appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": canonical.Encode()}, "", true, true, false)
			path, _ := snapshot.RequestPath("R")
			original := requestBytes("R", 3, wire.Sum([]byte("R")), false)
			appendReceipt(t, repo, "MUTATION", map[string][]byte{path: original}, "R", true, true, false)
			otherPath, _ := snapshot.RequestPath("other")
			appendReceipt(t, repo, "MUTATION", map[string][]byte{otherPath: requestBytes("other", 4, wire.Sum([]byte("other")), false)}, "other", true, true, false)
			edited := fixture.Ticket("A")
			edited.Title = "stable human edit D"
			fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "A.json"), edited.Encode())
			// Both equality and untracked-intent loops must be skipped by the adapter.
			fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "B.json"), fixture.Ticket("B").Encode())
			before := fixture.TreeSnapshot(t, repo.StateDir)
			beforeIntent := fixture.TreeSnapshot(t, repo.IntentDir)
			_, err := r.Audit("intent/tickets/A.json")
			requireCode(t, err, wire.CodeIntentDiverged)
			index := RequestIndex{Reader: r}
			entry, ok, err := index.Lookup("R")
			if err != nil || !ok {
				t.Fatalf("stable divergence blocked request proof: %v", err)
			}
			want, err := snapshot.DecodeRequest(original)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(entry, want.Entry) {
				t.Fatal("original outcome changed")
			}
			if _, ok, err := index.Lookup("newID"); err != nil || ok {
				t.Fatalf("verified absence: %v %v", ok, err)
			}
			if index.IntentProjectionAgreement != "NOT_OBSERVED" {
				t.Fatal("adapter claimed intent agreement")
			}
			// Lookup creates no reconciliation grant and does not rewrite D or B.
			if wire.CodeOf(intent.Divergence("intent/tickets/A.json", wire.Sum(edited.Encode()), wire.Sum(canonical.Encode()), nil)) != wire.CodeIntentDiverged {
				t.Fatal("divergence grant changed")
			}
			fixture.AssertUntouched(t, repo, before, beforeIntent, "lookup through stable divergence")
			if corruption == "missing" {
				remove(t, filepath.Join(repo.StateDir, otherPath))
			} else {
				fixture.Write(t, filepath.Join(repo.StateDir, otherPath), []byte("{}\n"))
			}
			for _, id := range []string{"R", "newID"} {
				_, ok, err := index.Lookup(id)
				if err == nil || ok {
					t.Fatalf("ignored unrelated request corruption for %s: %v %v", id, ok, err)
				}
			}
		})
	}
}

func TestTMV0002_AS10_IntentDoesNotConsumeStateScanBudget(t *testing.T) {
	_, r := setup(t)
	o, err := r.capture(profileLimits)
	if err != nil {
		t.Fatal(err)
	}
	stateEntries := 0
	for p := range o.files {
		if p != "." && p != "intent" && !strings.HasPrefix(p, "intent/") {
			stateEntries++
		}
	}
	_, err = r.audit(nil, "", limits{scan: stateEntries, selected: wire.MaxIntentTreeBytes}, true)
	if err != nil {
		t.Fatalf("intent lowered the state scan budget: %v", err)
	}
}

func TestTMV0007_AS35_ReconciliationCanonicalReadIsTargetScoped(t *testing.T) {
	for _, discard := range [][]byte{[]byte{}, []byte("{malformed"), fixture.Ticket("A").Encode()} {
		t.Run(fmt.Sprint(len(discard)), func(t *testing.T) {
			repo, r := setup(t)
			a, b := fixture.Ticket("A"), fixture.Ticket("B")
			appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": a.Encode(), "intent/tickets/B.json": b.Encode()}, "", true, true, false)
			path := filepath.Join(repo.IntentDir, "tickets", "A.json")
			fixture.Write(t, path, discard)
			before := fixture.TreeSnapshot(t, repo.StateDir)
			intentBefore := fixture.TreeSnapshot(t, repo.IntentDir)
			result, err := r.Reconciliation(a.TicketID.Raw, "intent/queue.json", "intent/policy.json")
			if err != nil {
				t.Fatal(err)
			}
			got := result.Records["intent/tickets/A.json"]
			if !bytes.Equal(got.Raw, a.Encode()) || got.Sha256 == nil || *got.Sha256 != wire.Sum(a.Encode()) || result.ProjectionAgreement != "ALL_EXCEPT_TARGET_AGREE" || result.HistoricalAcceptance != "NOT_OBSERVED" {
				t.Fatalf("canonical authority: %+v", result)
			}
			fixture.AssertUntouched(t, repo, before, intentBefore, "canonical retrieval")
			if !bytes.Equal(discard, a.Encode()) {
				_, err = r.Audit()
				requireCode(t, err, wire.CodeIntentDiverged)
			}
			fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "B.json"), []byte("unrelated edit"))
			_, err = r.Reconciliation(a.TicketID.Raw)
			requireCode(t, err, wire.CodeIntentDiverged)
		})
	}
}

func TestTMV0007_AS35_ReconciliationReadRefusesUnprovedHistory(t *testing.T) {
	for _, kind := range []string{"private", "pending", "scope", "absent", "moving"} {
		t.Run(kind, func(t *testing.T) {
			repo, r := setup(t)
			a := fixture.Ticket("A")
			appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": a.Encode()}, "", true, true, false)
			target := a.TicketID.Raw
			want := wire.CodeJournalForked
			switch kind {
			case "private":
				fixture.Write(t, filepath.Join(repo.StateDir, "reservations.json"), []byte("corrupt"))
			case "pending":
				want = wire.CodeRedoPending
				appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": a.Encode()}, "", false, false, false)
			case "scope":
				want = wire.CodeOutOfScope
				target = "ticket:other:q:A"
			case "absent":
				want = wire.CodeMalformed
				target = "ticket:acme:main:MISSING"
			case "moving":
				want = wire.CodeSnapshotMoved
				count := 0
				r.afterCapture = func() {
					count++
					fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "A.json"), []byte(fmt.Sprint(count)))
				}
			}
			_, err := r.Reconciliation(target)
			requireCode(t, err, want)
		})
	}
}

func TestTMV0016_AS27_BarrierRemovalAuditRetainsStrictBoundaries(t *testing.T) {
	for _, kind := range []string{"divergent", "missing", "extra", "queue", "policy", "private", "pending", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			repo, r := setup(t)
			a, b := fixture.Ticket("A"), fixture.Ticket("B")
			appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": a.Encode(), "intent/tickets/B.json": b.Encode()}, "", true, true, false)
			fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "A.json"), []byte{})
			fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "B.json"), []byte("malformed edit"))
			switch kind {
			case "missing":
				remove(t, filepath.Join(repo.IntentDir, "tickets", "A.json"))
			case "extra":
				fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "EXTRA.json"), []byte("untracked"))
			case "queue", "policy":
				fixture.Write(t, filepath.Join(repo.IntentDir, kind+".json"), []byte("{}\n"))
			case "private":
				fixture.Write(t, filepath.Join(repo.StateDir, "reservations.json"), []byte("{}\n"))
			case "pending":
				appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": a.Encode()}, "", false, false, false)
			case "symlink":
				remove(t, filepath.Join(repo.IntentDir, "tickets", "A.json"))
				if err := os.Symlink("B.json", filepath.Join(repo.IntentDir, "tickets", "A.json")); err != nil {
					t.Fatal(err)
				}
			}
			before, intents := fixture.TreeSnapshot(t, repo.StateDir), fixture.TreeSnapshot(t, repo.IntentDir)
			result, err := r.BarrierRemoval("intent/tickets/A.json", "intent/tickets/B.json")
			if kind == "divergent" {
				if err != nil || result.ProjectionAgreement != "TICKETS_NOT_COMPARED" || !bytes.Equal(result.Records["intent/tickets/A.json"].Raw, a.Encode()) || result.RuntimeQualification != "NOT_OBSERVED" {
					t.Fatalf("unpause audit: %+v %v", result, err)
				}
				_, err = r.Audit()
				requireCode(t, err, wire.CodeIntentDiverged)
				_, err = r.Reconciliation(a.TicketID.Raw)
				requireCode(t, err, wire.CodeIntentDiverged)
			} else if err == nil {
				t.Fatalf("%s accepted", kind)
			}
			fixture.AssertUntouched(t, repo, before, intents, "barrier removal audit")
		})
	}
}
