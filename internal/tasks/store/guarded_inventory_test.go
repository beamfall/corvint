//go:build darwin || linux

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"sort"
)

func inventoryGuard(t *testing.T, repo *intent.Repository) *authority.ChangeGuard {
	t.Helper()
	g, err := authority.WatchChanges(repo)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { g.Close() })
	return g
}
func TestCALV0026_GuardedInventoryParityAndFailures(t *testing.T) {
	for _, kind := range []string{"valid", "empty-evidence", "unknown-file", "nested-receipt", "symlink", "missing-optional"} {
		t.Run(kind, func(t *testing.T) {
			repo := cachedLeaseRepo(t)
			switch kind {
			case "empty-evidence":
				fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(wire.Sum(nil))), nil)
			case "unknown-file":
				fixture.Write(t, filepath.Join(repo.StateDir, "receipts", "bogus"), []byte("x"))
			case "nested-receipt":
				fixture.Write(t, filepath.Join(repo.StateDir, "receipts", "nested", "x"), []byte("x"))
			case "symlink":
				if err := os.Symlink("head.json", filepath.Join(repo.StateDir, "barrier.json")); err != nil {
					t.Fatal(err)
				}
			case "missing-optional":
				os.Remove(filepath.Join(repo.StateDir, "barrier.json"))
			}
			a, ae := inventory(repo)
			g, gerr := authority.WatchChanges(repo)
			if gerr != nil {
				if kind != "symlink" || wire.CodeOf(gerr) != wire.CodeOf(ae) {
					t.Fatalf("guard %v inventory %v", gerr, ae)
				}
				return
			}
			defer g.Close()
			b, be, cleanup := guardedLeaseInventory(repo, g, hooksForInventory(context.Background()))
			if len(cleanup) != 0 || wire.CodeOf(ae) != wire.CodeOf(be) || (ae == nil) != (be == nil) {
				t.Fatalf("parity %v / %v cleanup%v", ae, be, cleanup)
			}
			if ae == nil && (!reflect.DeepEqual(a.Files(), b.Files()) || !reflect.DeepEqual(a.Directories(), b.Directories())) {
				t.Fatal("inventory differs")
			}
		})
	}
	for _, kind := range []string{"truncation", "same-size-mtime", "membership", "parent-swap", "ancestor-aba"} {
		t.Run(kind, func(t *testing.T) {
			repo := cachedLeaseRepo(t)
			g := inventoryGuard(t, repo)
			hooks := hooksForInventory(context.Background())
			original := hooks.read
			changed := false
			hooks.read = func(r *os.Root, path, name string, max int) ([]byte, error) {
				raw, err := original(r, path, name, max)
				if !changed && err == nil {
					changed = true
					switch kind {
					case "truncation":
						if err := os.Truncate(path, 0); err != nil {
							t.Fatal(err)
						}
					case "same-size-mtime":
						st, e := os.Stat(path)
						if e != nil {
							t.Fatal(e)
						}
						if len(raw) > 0 {
							raw[0] ^= 1
						}
						fixture.Write(t, path, raw)
						if e := os.Chtimes(path, st.ModTime(), st.ModTime()); e != nil {
							t.Fatal(e)
						}
					case "membership":
						fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(wire.Sum([]byte("new")))), []byte("new"))
					case "parent-swap":
						old := repo.StateDir + "-old"
						if e := os.Rename(repo.StateDir, old); e != nil {
							t.Fatal(e)
						}
						if e := os.Mkdir(repo.StateDir, 0700); e != nil {
							t.Fatal(e)
						}
						t.Cleanup(func() { os.RemoveAll(repo.StateDir); os.Rename(old, repo.StateDir) })
					case "ancestor-aba":
						old := repo.StateDir + "-old"
						if e := os.Rename(repo.StateDir, old); e != nil {
							t.Fatal(e)
						}
						if e := os.Rename(old, repo.StateDir); e != nil {
							t.Fatal(e)
						}
					}
				}
				return raw, err
			}
			inv, err, cleanup := guardedLeaseInventory(repo, g, hooks)
			if !changed || inv != nil || err == nil || len(cleanup) != 0 {
				t.Fatalf("race accepted: %v %v %+v", inv, err, cleanup)
			}
		})
	}
}

func TestCALV0026_GuardedInventoryParentLifetime(t *testing.T) {
	repo := cachedLeaseRepo(t)
	for i := 0; i < 32; i++ {
		raw := []byte(fmt.Sprint(i))
		fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(wire.Sum(raw))), raw)
	}
	g := inventoryGuard(t, repo)
	hooks := hooksForInventory(context.Background())
	live := map[*os.Root]bool{}
	opens, closes, peak, reads, runs := 0, 0, 0, 0, 0
	last := ""
	hooks.open = func(path string) (*os.Root, error) {
		r, err := safeopen.Root(path)
		if err == nil {
			opens++
			live[r] = true
			if len(live) > peak {
				peak = len(live)
			}
		}
		return r, err
	}
	hooks.close = func(r *os.Root) error {
		if !live[r] {
			t.Fatal("close without ownership")
		}
		delete(live, r)
		closes++
		return r.Close()
	}
	hooks.read = func(r *os.Root, path, name string, max int) ([]byte, error) {
		reads++
		parent := filepath.Dir(path)
		if parent != last {
			runs++
			last = parent
		}
		return intent.ReadFileFromRoot(r, path, name, max)
	}
	inv, err, cleanup := guardedLeaseInventory(repo, g, hooks)
	if err != nil || len(cleanup) > 0 || len(live) != 0 || peak > 2 || opens != 2*runs || closes != opens || reads < len(inv.Files()) || opens >= reads {
		t.Fatalf("inv%v err%v cleanup%v open%d close%d peak%d read%d runs%d live%d", inv, err, cleanup, opens, closes, peak, reads, runs, len(live))
	}
	t.Logf("actual parent opens=%d closes=%d retained-root peak=%d basename reads=%d parent runs=%d", opens, closes, peak, reads, runs)
}

// Build a real claimable store so removing the injected fault would commit a
// claim. The public Lease boundary is used in every integration refusal cell.
func inventoryClaimRepo(t *testing.T) (*intent.Repository, LeaseChoice) {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.WriteIntent(t, r)
	policy := fixture.PolicyValue()
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	if report, err := Init(context.Background(), repo, mutation.Binding{ID: "tester", Role: "OWNER"}, "inventory-init", fixture.Timestamp); err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("init %+v %v", report, err)
	}
	var envelopeFields any
	err = json.Unmarshal([]byte(`{"profile":"taskman-mutation/0","requestId":"inventory-create","actor":{"id":"tester","role":"OWNER"},"queueId":"queue:acme:main","targetId":null,"expectedRevision":null,"operation":"CREATE","issuedAt":"2026-09-06T12:00:00Z","payload":{"acceptanceCriteria":["it exists"],"body":null,"capabilities":[],"dependencies":[],"dueDate":null,"effects":{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":[]},"estimateMinutes":null,"executionClass":"AUTONOMOUS","kind":"FEATURE","labels":[],"milestone":null,"order":"0","owner":null,"priority":"P2","requiredGates":[],"requirementRefs":[],"source":{"kind":"NATIVE","sourceItemId":null,"sourceQueueId":"queue:acme:main","sourceRevisionSha256":null},"supersededBy":null,"supersedes":null,"title":"inventory"}}`), &envelopeFields)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(envelopeFields)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := wire.Parse(append(canonical, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	created, err := Mutate(context.Background(), repo, mutation.Binding{ID: "tester", Role: "OWNER"}, wire.EncodeFile(envelope), fixture.Timestamp)
	if err != nil || created.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create %+v %v", created, err)
	}
	gitroot := fixture.TempDirOutside(t)
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = gitroot
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v %s", err, out)
		}
	}
	return repo, LeaseChoice{QueueID: fixture.QueueID, RequestID: "inventory-claim", Root: gitroot, Lease: transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: created.Ticket, Holder: "agent", LeaseMinutes: "60", Scope: []string{"src/"}}}
}
func TestCALV0026_GuardedInventoryLeaseFailureOwnership(t *testing.T) {
	actor := mutation.Binding{ID: "tester", Role: "OWNER"}
	for _, kind := range []string{"transition-close", "final-close", "absent-close", "guard-close", "preparation-close", "parent-enoent", "parent-stat", "rebind-enoent", "rebind-stat"} {
		t.Run(kind, func(t *testing.T) {
			repo, choice := inventoryClaimRepo(t)
			primeAuditInventory(t, repo)
			before := fixture.TreeSnapshot(t, repo.StateDir)
			expected := append([]fixture.Entry(nil), before...)
			hooks := hooksForInventory(context.Background())
			openCounts := map[string]int{}
			roots := map[*os.Root]string{}
			live := map[*os.Root]bool{}
			reads, guardCloses, prepCloses, writerAcquires := 0, 0, 0, 0
			injected := false
			hooks.open = func(path string) (*os.Root, error) {
				openCounts[path]++
				if kind == "parent-enoent" && !injected {
					injected = true
					return nil, os.ErrNotExist
				}
				if kind == "rebind-enoent" && openCounts[path] == 2 && !injected {
					injected = true
					return nil, os.ErrNotExist
				}
				r, err := safeopen.Root(path)
				if err == nil {
					roots[r] = path
					live[r] = true
				}
				return r, err
			}
			hooks.stat = func(r *os.Root) (os.FileInfo, error) {
				if (kind == "parent-stat" || kind == "rebind-stat" && openCounts[roots[r]] == 2) && !injected {
					injected = true
					return nil, os.ErrNotExist
				}
				return r.Stat(".")
			}
			hooks.read = func(r *os.Root, path, name string, max int) ([]byte, error) {
				reads++
				if kind == "absent-close" && name == "VERSION" {
					return nil, os.ErrNotExist
				}
				return intent.ReadFileFromRoot(r, path, name, max)
			}
			hooks.close = func(r *os.Root) error {
				path := roots[r]
				if !live[r] {
					t.Fatal("double close")
				}
				delete(live, r)
				err := r.Close()
				if err != nil {
					return err
				}
				target := kind == "transition-close" || kind == "absent-close" || kind == "guard-close" || kind == "preparation-close" || kind == "final-close" && strings.HasSuffix(path, "/tickets")
				if target && !injected {
					injected = true // Dirty observation without changing accepted bytes.
					head := filepath.Join(repo.StateDir, "head.json")
					raw, e := os.ReadFile(head)
					if e != nil {
						t.Fatal(e)
					}
					if e = os.WriteFile(head, raw, 0600); e != nil {
						t.Fatal(e)
					}
					// Account only for the timestamp this injection changed.
					// Original path, mode, size, hash and every other entry stay fixed.
					info, e := os.Lstat(head)
					if e != nil {
						t.Fatal(e)
					}
					found := false
					for i := range expected {
						if expected[i].Path == "/head.json" {
							expected[i].MTime = info.ModTime().UnixNano()
							found = true
						}
					}
					if !found {
						t.Fatal("expected snapshot has no head.json")
					}
					return errors.New("injected root close")
				}
				return nil
			}
			hooks.closeGuard = func(g *authority.ChangeGuard) error {
				guardCloses++
				err := g.Close()
				if err == nil && kind == "guard-close" {
					return errors.New("injected guard close")
				}
				return err
			}
			hooks.closePreparation = func(p *authority.PreparationLock) error {
				prepCloses++
				err := p.Close()
				if err == nil && kind == "preparation-close" {
					return errors.New("injected preparation close")
				}
				return err
			}
			ctx := context.WithValue(context.Background(), inventoryHooksKey{}, hooks)
			ctx = authority.WithLockObserver(ctx, func(time.Duration) { writerAcquires++ })
			report, err := Lease(ctx, repo, actor, choice, fixture.Timestamp)
			if !injected {
				t.Error("intended fault hook was not reached")
			}
			if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || os.IsNotExist(err) {
				t.Errorf("terminal classification: %v", err)
			}
			if report.Receipt != "" {
				t.Errorf("unexpected receipt: %s", report.Receipt)
			}
			if len(live) != 0 {
				t.Errorf("unretired roots: %d", len(live))
			}
			if guardCloses != 1 || prepCloses != 1 {
				t.Errorf("cleanup counts: guard=%d preparation=%d", guardCloses, prepCloses)
			}
			actual := fixture.TreeSnapshot(t, repo.StateDir)
			actualByPath := make(map[string]fixture.Entry, len(actual))
			for _, entry := range actual {
				actualByPath[entry.Path] = entry
			}
			for _, want := range expected {
				got, ok := actualByPath[want.Path]
				if !ok {
					t.Errorf("snapshot missing %s", want.Path)
				} else if got != want {
					t.Errorf("snapshot %s: want %+v; got %+v", want.Path, want, got)
				}
				delete(actualByPath, want.Path)
			}
			for path, extra := range actualByPath {
				t.Errorf("unexpected snapshot entry %s: %+v", path, extra)
			}
			detail := fmt.Sprint(err)
			fatal := strings.Contains(kind, "close")
			if fatal && writerAcquires != 0 {
				t.Errorf("fatal cleanup acquired writer authority %d times", writerAcquires)
			}
			if fatal && (!strings.Contains(detail, "observation:") || !strings.Contains(detail, "injected root close")) {
				t.Errorf("missing fatal/observation diagnostic: %v", err)
			}
			if kind == "guard-close" && !strings.Contains(detail, "injected guard close") {
				t.Errorf("missing guard diagnostic: %v", err)
			}
			if kind == "preparation-close" && !strings.Contains(detail, "injected preparation close") {
				t.Errorf("missing preparation diagnostic: %v", err)
			}
			// Each stable parent's first + rebind opens happen once in a single round.
			for path, n := range openCounts {
				if n > 2 {
					t.Errorf("retried %s %d", path, n)
				}
			}
			t.Logf("injected=%t reads=%d writer-acquires=%d guard-closes=%d preparation-closes=%d live-roots=%d", injected, reads, writerAcquires, guardCloses, prepCloses, len(live))
		})
	}
	t.Run("clean-resource-race-retries-and-commits", func(t *testing.T) {
		repo, choice := inventoryClaimRepo(t)
		primeAuditInventory(t, repo)
		hooks := hooksForInventory(context.Background())
		read := hooks.read
		versionReads := 0
		hooks.read = func(r *os.Root, path, name string, max int) ([]byte, error) {
			raw, err := read(r, path, name, max)
			if name == "VERSION" {
				versionReads++
				if versionReads == 1 {
					if e := os.WriteFile(path, raw, 0600); e != nil {
						t.Fatal(e)
					}
				}
			}
			return raw, err
		}
		ctx := context.WithValue(context.Background(), inventoryHooksKey{}, hooks)
		report, err := Lease(ctx, repo, actor, choice, fixture.Timestamp)
		if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.Receipt == "" || versionReads != 2 {
			t.Fatalf("race did not retry and commit: %+v %v reads%d", report, err, versionReads)
		}
	})
	t.Run("defensive-commit-precedes-writer-and-dirty-guard", func(t *testing.T) {
		repo := cachedLeaseRepo(t)
		g := inventoryGuard(t, repo)
		path := filepath.Join(repo.StateDir, "head.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fixture.Write(t, path, raw)
		p := &preparedLease{guard: g, fatalCleanup: []inventoryCleanup{{"root close", errors.New("fatal")}}}
		called := false
		acquired := 0
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx = authority.WithLockObserver(ctx, func(time.Duration) { acquired++ })
		err = commitLease(ctx, repo, preparationReap("defensive"), p, &Report{}, func() error { called = true; return nil })
		if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || called || acquired != 0 {
			t.Fatalf("precedence: %v called%t acquired%d", err, called, acquired)
		}
	})
}

// Linear receipt/request fixture codec reused from the GH494 CLI corpus.
func inventoryProbeObject(kv ...any) wire.Value {
	o := wire.NewObject()
	for i := 0; i < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}
func inventoryProbeStr(s string) wire.Value { return wire.String(s) }
func inventoryProbeRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func inventoryProbeRemove(t *testing.T, p string) {
	t.Helper()
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
}
func inventoryProbePostSet(v wire.Value, posts map[string][]byte, pres map[string]*wire.Digest, blob bool) {
	var paths []string
	for p := range posts {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var before, after []wire.Value
	for _, p := range paths {
		pre := wire.Null()
		if pres[p] != nil {
			pre = inventoryProbeStr(string(*pres[p]))
		}
		before = append(before, inventoryProbeObject("path", inventoryProbeStr(p), "sha256", pre))
		raw := posts[p]
		rec := wire.Null()
		bd := wire.Null()
		d := wire.Null()
		if raw != nil {
			d = inventoryProbeStr(string(wire.Sum(raw)))
			if blob {
				bd = d
			} else {
				rec, _ = wire.Parse(raw)
			}
		}
		after = append(after, inventoryProbeObject("path", inventoryProbeStr(p), "sha256", d, "record", rec, "blobSha256", bd))
	}
	v.Obj.Set("pre", wire.Array(before...))
	v.Obj.Set("post", wire.Array(after...))
}
func inventoryProbeAppendReceipt(t *testing.T, repo *fixture.Repo, kind string, posts map[string][]byte, request string, project, advance, blob bool) uint64 {
	t.Helper()
	hp := filepath.Join(repo.StateDir, "head.json")
	h, err := snapshot.DecodeHead(inventoryProbeRead(t, hp))
	if err != nil {
		t.Fatal(err)
	}
	seq := h.LastSeq.Uint64() + 1
	v := fixture.ReceiptValue(seq, h.LastReceiptSha256, kind, h.Generation.Uint64())
	v.Obj.Set("recordedAt", inventoryProbeStr(string(WallClock())))
	if request != "" {
		v.Obj.Set("requestId", inventoryProbeStr(request))
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
				inventoryProbeRemove(t, dest)
			} else {
				fixture.Write(t, dest, raw)
			}
		}
	}
	inventoryProbePostSet(v, posts, pres, blob)
	raw := wire.EncodeFile(v)
	name, _ := snapshot.ReceiptName(seq)
	fixture.Write(t, filepath.Join(repo.StateDir, "receipts", name), raw)
	if advance {
		fixture.Write(t, hp, wire.EncodeFile(fixture.HeadValue(repo.Root, seq, wire.Sum(raw), h.Generation.Uint64(), h.InitSha256)))
	}
	return seq
}
func inventoryProbeRequestBytes(id string, seq uint64, digest wire.Digest, refused bool) []byte {
	s := wire.SizeOf(seq)
	out := mutation.Outcome{RequestID: id, Outcome: mutation.OutcomeCompleted, ReceiptSeq: &s, Codes: []string{}}
	if refused {
		out.Outcome = mutation.OutcomeBlocked
		out.ResultingRevision = nil
		out.ResultingAcceptanceRevision = nil
		out.Codes = []string{wire.CodePaused}
	}
	return wire.EncodeFile(inventoryProbeObject("requestId", inventoryProbeStr(id), "seq", inventoryProbeStr(string(s)), "mutationSha256", inventoryProbeStr(string(digest)), "outcome", out.Value()))
}

func TestCALV0026_GuardedInventoryAncestorCoverage(t *testing.T) {
	for _, kind := range []string{"common-aba", "worktree-aba", "common-replacement", "worktree-replacement"} {
		t.Run(kind, func(t *testing.T) {
			repo := cachedLeaseRepo(t)
			g := inventoryGuard(t, repo)
			hooks := hooksForInventory(context.Background())
			read := hooks.read
			injected := false
			target := repo.CommonDir
			if strings.HasPrefix(kind, "worktree") {
				target = repo.PrimaryWorktree
			}
			held := target + "-held"
			hooks.read = func(r *os.Root, path, name string, max int) ([]byte, error) {
				raw, err := read(r, path, name, max)
				if !injected && err == nil {
					injected = true
					if e := os.Rename(target, held); e != nil {
						t.Fatal(e)
					}
					if strings.HasSuffix(kind, "aba") {
						if e := os.Rename(held, target); e != nil {
							t.Fatal(e)
						}
					} else {
						// Restore the original directory before the fixture and guard cleanups.
						t.Cleanup(func() {
							if e := os.Remove(target); e != nil {
								t.Error(e)
							}
							if e := os.Rename(held, target); e != nil {
								t.Error(e)
							}
						})
						if e := os.Mkdir(target, 0700); e != nil {
							t.Fatal(e)
						}
					}
				}
				return raw, err
			}
			inv, err, cleanup := guardedLeaseInventory(repo, g, hooks)
			if !injected || inv != nil || err == nil || len(cleanup) != 0 {
				t.Errorf("ancestor accepted: injected=%t inv=%v error=%v cleanup=%v", injected, inv, err, cleanup)
			}
			if observed := g.Check(); wire.CodeOf(observed) != wire.CodeSnapshotMoved {
				t.Errorf("ancestor observation: %v", observed)
			}
		})
	}
}

func inventoryProbeAudit(t *testing.T, repo *intent.Repository) map[string]any {
	t.Helper()
	q, err := wire.ParseQueueID("queueId", fixture.QueueID)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	proof, err := (journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: repo.PrimaryWorktree}).Audit()
	elapsed := float64(time.Since(start)) / float64(time.Millisecond)
	if err != nil || proof == nil {
		t.Fatalf("full audit: %v %v", proof, err)
	}
	if proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "AGREES" || proof.Pending || proof.Mode != journal.ModeFull {
		t.Fatalf("full audit: %+v", proof)
	}
	return map[string]any{"wallMS": elapsed, "structural": proof.StructuralConsistency, "projection": proof.ProjectionAgreement, "headSeq": proof.LastSeq, "mode": proof.Mode}
}

func TestCALV0026_GuardedInventoryPopulated5000(t *testing.T) {
	setup := time.Now()
	repo, choice := inventoryClaimRepo(t)
	actor := mutation.Binding{ID: "tester", Role: "OWNER"}
	claimed, err := Lease(context.Background(), repo, actor, choice, fixture.Timestamp)
	if err != nil || claimed.Outcome.Outcome != mutation.OutcomeCompleted || claimed.AttemptID == "" {
		t.Fatalf("populated claim: %+v %v", claimed, err)
	}
	released, err := Lease(context.Background(), repo, actor, LeaseChoice{QueueID: fixture.QueueID, RequestID: "probe-release", Lease: transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: claimed.AttemptID, Generation: claimed.Generation}}, fixture.Timestamp)
	if err != nil || released.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("populated release: %+v %v", released, err)
	}
	str := inventoryProbeStr
	obj := inventoryProbeObject
	payload := obj("version", str("1"), "title", str("Populated inventory"), "ticketIds", wire.Array(), "predecessorReleaseIds", wire.Array(), "requiredGates", wire.Array(), "acceptanceCriteria", wire.Strings([]string{"accepted"}))
	envelope := wire.EncodeFile(obj("profile", str(release.MutationProfile), "requestId", str("probe-release-intent"), "actor", obj("id", str("tester"), "role", str("OWNER")), "queueId", str(fixture.QueueID), "releaseId", str("v1"), "expectedRevision", wire.Null(), "operation", str(release.OpCreate), "payload", payload, "issuedAt", str(fixture.Timestamp)))
	releaseReport, err := Release(context.Background(), repo, actor, envelope, fixture.Timestamp)
	if err != nil || releaseReport.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("populated release intent: %+v %v", releaseReport, err)
	}
	fr := &fixture.Repo{Root: repo.PrimaryWorktree, CommonDir: repo.CommonDir, StateDir: repo.StateDir, IntentDir: filepath.Join(repo.PrimaryWorktree, intent.Dir)}
	head, err := snapshot.DecodeHead(inventoryProbeRead(t, filepath.Join(repo.StateDir, "head.json")))
	if err != nil {
		t.Fatal(err)
	}
	for seq := head.LastSeq.Uint64() + 1; seq <= 5000; seq++ {
		id := fmt.Sprintf("h494-history-%d", seq)
		path, err := snapshot.RequestPath(id)
		if err != nil {
			t.Fatal(err)
		}
		// One blob-backed request ensures evidence is populated and referenced.
		inventoryProbeAppendReceipt(t, fr, "MUTATION", map[string][]byte{path: inventoryProbeRequestBytes(id, seq, wire.Sum([]byte(id)), false)}, id, true, true, seq == 5000)
	}
	setupMS := float64(time.Since(setup)) / float64(time.Millisecond)
	before := fixture.TreeSnapshot(t, repo.StateDir)
	intentBefore := fixture.TreeSnapshot(t, fr.IntentDir)
	auditBefore := inventoryProbeAudit(t, repo)
	legacyReads, legacyHashes, legacyBytes := 0, 0, 0
	start := time.Now()
	files, dirs, err := scanWithReader(repo, func(path string, max int) ([]byte, error) {
		legacyReads++
		raw, err := intent.ReadFile(path, max)
		if err == nil {
			legacyHashes++
			legacyBytes += len(raw)
		}
		return raw, err
	})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := transaction.NewInventory(files, dirs)
	if err != nil {
		t.Fatal(err)
	}
	legacyMS := float64(time.Since(start)) / float64(time.Millisecond)
	start = time.Now()
	g := inventoryGuard(t, repo)
	guardMS := float64(time.Since(start)) / float64(time.Millisecond)
	hooks := hooksForInventory(context.Background())
	live := map[*os.Root]bool{}
	opens, closes, peak, reads, hashes, bytes, runs := 0, 0, 0, 0, 0, 0, 0
	last := ""
	hooks.open = func(path string) (*os.Root, error) {
		r, err := safeopen.Root(path)
		if err == nil {
			opens++
			live[r] = true
			if len(live) > peak {
				peak = len(live)
			}
		}
		return r, err
	}
	hooks.close = func(r *os.Root) error {
		if !live[r] {
			t.Error("close without root ownership")
		}
		delete(live, r)
		closes++
		return r.Close()
	}
	hooks.read = func(r *os.Root, path, name string, max int) ([]byte, error) {
		reads++
		parent := filepath.Dir(path)
		if parent != last {
			runs++
			last = parent
		}
		raw, err := intent.ReadFileFromRoot(r, path, name, max)
		if err == nil {
			hashes++
			bytes += len(raw)
		}
		return raw, err
	}
	start = time.Now()
	guarded, err, cleanup := guardedLeaseInventory(repo, g, hooks)
	guardedMS := float64(time.Since(start)) / float64(time.Millisecond)
	if err != nil || len(cleanup) != 0 || guarded == nil {
		t.Fatalf("guarded inventory: %v %v", err, cleanup)
	}
	if closeErr := g.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if !reflect.DeepEqual(legacy.Files(), guarded.Files()) || !reflect.DeepEqual(legacy.Directories(), guarded.Directories()) {
		t.Error("populated inventory file/hash/byte/directory parity failed")
	}
	if opens != 2*runs || closes != opens || peak > 2 || len(live) != 0 || reads != legacyReads || hashes != legacyHashes || bytes != legacyBytes || hashes != len(guarded.Files()) {
		t.Errorf("counters differ: opens%d closes%d peak%d live%d reads%d/%d hashes%d/%d bytes%d/%d runs%d", opens, closes, peak, len(live), reads, legacyReads, hashes, legacyHashes, bytes, legacyBytes, runs)
	}
	counts := map[string]int{}
	for _, file := range guarded.Files() {
		for _, prefix := range []string{"receipts/", "requests/", "evidence/", "attempts/", "intent/tickets/", "intent/releases/", "pinned/"} {
			if strings.HasPrefix(file.Path, prefix) {
				counts[prefix]++
			}
		}
	}
	for _, prefix := range []string{"receipts/", "requests/", "evidence/", "attempts/", "intent/tickets/", "intent/releases/"} {
		if counts[prefix] == 0 {
			t.Errorf("missing populated prefix %s", prefix)
		}
	}
	if counts["receipts/"] != 5000 {
		t.Errorf("receipt count %d", counts["receipts/"])
	}
	auditAfter := inventoryProbeAudit(t, repo)
	if !reflect.DeepEqual(before, fixture.TreeSnapshot(t, repo.StateDir)) || !reflect.DeepEqual(intentBefore, fixture.TreeSnapshot(t, fr.IntentDir)) {
		t.Error("read-only probe changed fixture state")
	}
	metrics := map[string]any{"fixture": "regenerated-existing-linear-5000-recipe-plus-populated-records", "setupMS": setupMS, "auditBefore": auditBefore, "auditAfter": auditAfter, "legacyScanMS": legacyMS, "guardRegistrationMS": guardMS, "guardedScanMS": guardedMS, "basenameReadAttempts": reads, "successfulFileHashes": hashes, "bytesHashed": bytes, "rootOpens": opens, "rootCloses": closes, "retainedRootPeak": peak, "liveRootsAfter": len(live), "parentRuns": runs, "prefixCounts": counts, "stateDirectories": len(guarded.Directories()), "legacyInternalRootOpenCount": "NOT_OBSERVED", "transientDescriptorCount": "NOT_OBSERVED", "limits": []string{"single sample in legacy-then-guarded order, warm cache and counter overhead", "not a CLI throughput wave or fairness proof", "not the reporter's original binary/history"}}
	raw, err := json.Marshal(metrics)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("GH494_POPULATED_METRICS %s", raw)
}
