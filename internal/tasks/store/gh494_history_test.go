package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func h494Object(kv ...any) wire.Value {
	o := wire.NewObject()
	for i := 0; i < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}
func h494Str(s string) wire.Value { return wire.String(s) }
func h494Read(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func h494Remove(t *testing.T, p string) {
	t.Helper()
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
}
func h494PostSet(v wire.Value, posts map[string][]byte, pres map[string]*wire.Digest, blob bool) {
	var paths []string
	for p := range posts {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var before, after []wire.Value
	for _, p := range paths {
		pre := wire.Null()
		if pres[p] != nil {
			pre = h494Str(string(*pres[p]))
		}
		before = append(before, h494Object("path", h494Str(p), "sha256", pre))
		raw := posts[p]
		rec := wire.Null()
		bd := wire.Null()
		d := wire.Null()
		if raw != nil {
			d = h494Str(string(wire.Sum(raw)))
			if blob {
				bd = d
			} else {
				rec, _ = wire.Parse(raw)
			}
		}
		after = append(after, h494Object("path", h494Str(p), "sha256", d, "record", rec, "blobSha256", bd))
	}
	v.Obj.Set("pre", wire.Array(before...))
	v.Obj.Set("post", wire.Array(after...))
}
func h494AppendReceipt(t *testing.T, repo *fixture.Repo, kind string, posts map[string][]byte, request string, project, advance, blob bool) uint64 {
	t.Helper()
	hp := filepath.Join(repo.StateDir, "head.json")
	h, err := snapshot.DecodeHead(h494Read(t, hp))
	if err != nil {
		t.Fatal(err)
	}
	seq := h.LastSeq.Uint64() + 1
	v := fixture.ReceiptValue(seq, h.LastReceiptSha256, kind, h.Generation.Uint64())
	v.Obj.Set("recordedAt", h494Str(string(store.WallClock())))
	if request != "" {
		v.Obj.Set("requestId", h494Str(request))
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
				h494Remove(t, dest)
			} else {
				fixture.Write(t, dest, raw)
			}
		}
	}
	h494PostSet(v, posts, pres, blob)
	raw := wire.EncodeFile(v)
	name, _ := snapshot.ReceiptName(seq)
	fixture.Write(t, filepath.Join(repo.StateDir, "receipts", name), raw)
	if advance {
		fixture.Write(t, hp, wire.EncodeFile(fixture.HeadValue(repo.Root, seq, wire.Sum(raw), h.Generation.Uint64(), h.InitSha256)))
	}
	return seq
}
func h494RequestBytes(id string, seq uint64, digest wire.Digest, refused bool) []byte {
	s := wire.SizeOf(seq)
	out := mutation.Outcome{RequestID: id, Outcome: mutation.OutcomeCompleted, ReceiptSeq: &s, Codes: []string{}}
	if refused {
		out.Outcome = mutation.OutcomeBlocked
		out.ResultingRevision = nil
		out.ResultingAcceptanceRevision = nil
		out.Codes = []string{wire.CodePaused}
	}
	return wire.EncodeFile(h494Object("requestId", h494Str(id), "seq", h494Str(string(s)), "mutationSha256", h494Str(string(digest)), "outcome", out.Value()))
}

// Synthetic history uses the repository's journal fixture encoding, not live API
// mutations. Setup is O(N): appendReceipt reads only head and touched projections.
// The final audit validates the generated chain before any timed workload.
// This private qualification harness is not a production observer.
type h494Item struct {
	Outcome    string    `json:"outcome"`
	AttemptID  string    `json:"attemptId"`
	Generation wire.Size `json:"generation"`
	Receipt    string    `json:"receipt"`
	Replayed   bool      `json:"replayed"`
}
type h494Result struct {
	Outcome string     `json:"outcome"`
	Items   []h494Item `json:"items"`
	Codes   []string   `json:"codes"`
}

func h494Save(t *testing.T, dir, name string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Errorf("encode evidence %s: %v", name, err)
		return
	}
	if err = os.WriteFile(filepath.Join(dir, name), append(raw, '\n'), 0600); err != nil {
		t.Errorf("save evidence %s: %v", name, err)
	}
}
func h494CLI(t *testing.T, ctx context.Context, binary, root, evidence, name string, args []string) (h494Result, error) {
	t.Helper()
	began := time.Now()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = root
	cmd.WaitDelay = 2 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	elapsed := time.Since(began)
	var result h494Result
	decodeErr := json.Unmarshal(stdout.Bytes(), &result)
	exit := -1
	if cmd.ProcessState != nil {
		exit = cmd.ProcessState.ExitCode()
	}
	h494Save(t, evidence, name+".json", map[string]any{"argv": append([]string{binary}, args...), "cwd": root, "startedAt": began.UTC(), "elapsedMS": float64(elapsed) / float64(time.Millisecond), "exit": exit, "error": fmt.Sprint(err), "decodeError": fmt.Sprint(decodeErr), "stdout": stdout.String(), "stderr": stderr.String(), "decoded": result})
	if err != nil {
		return result, err
	}
	if decodeErr != nil {
		return result, decodeErr
	}
	return result, h494CLIResult(result)
}

func h494CLIResult(result h494Result) error {
	if result.Outcome != "OK" || len(result.Items) != 1 || result.Items[0].Outcome != mutation.OutcomeCompleted || (!result.Items[0].Replayed && result.Items[0].Receipt == "") {
		return fmt.Errorf("unsuccessful CLI result: %+v", result)
	}
	return nil
}
func h494Audit(t *testing.T, s *leaseStore, evidence, name string) bool {
	t.Helper()
	q, err := wire.ParseQueueID("queueId", fixture.QueueID)
	if err != nil {
		t.Error(err)
		return false
	}
	proof, err := (journal.Reader{Source: journal.Native{StateDir: s.repo.StateDir, PrimaryWorktree: s.repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: s.repo.PrimaryWorktree}).Audit()
	h494Save(t, evidence, name+".json", map[string]any{"proof": proof, "error": fmt.Sprint(err)})
	if err != nil || proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "AGREES" {
		t.Errorf("full audit %s: %+v %v", name, proof, err)
		return false
	}
	return true
}
func TestGH494HistoryScaling(t *testing.T) {
	if os.Getenv("GH494_ADMITTED") != "yes" {
		t.Skip("requires explicit native admission")
	}
	t.Run("receipts-5000", func(t *testing.T) {
		t.Run("workers-10", func(t *testing.T) {
			binary, evidence := os.Getenv("GH494_CLI"), os.Getenv("GH494_EVIDENCE_DIR")
			if !filepath.IsAbs(binary) || !filepath.IsAbs(evidence) {
				t.Fatal("absolute binary/evidence paths required")
			}
			if _, err := os.Stat(evidence); err != nil {
				t.Fatal(err)
			}
			s := newLeaseStore(t)
			policy := fixture.PolicyValue()
			policy.Obj.Set("policyVersion", str("3"))
			policy.Obj.Set("capacity", obj("maxActiveAttempts", str("10"), "maxWorkersTotal", str("10"), "classes", wire.Array()))
			budgets, _ := policy.Obj.Get("budgets")
			budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
			rep, err := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("h494-capacity", "2", wire.EncodeFile(policy)), now(t))
			if err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("policy %+v %v", rep, err)
			}
			ids := make([]string, 10)
			for i := range ids {
				ids[i] = s.ticket(t, fmt.Sprintf("h494-worker-%d", i))
			}
			fr := &fixture.Repo{Root: s.repo.PrimaryWorktree, CommonDir: s.repo.CommonDir, StateDir: s.repo.StateDir, IntentDir: filepath.Join(s.repo.PrimaryWorktree, ".taskman")}
			gitRun(t, fr.Root, "init", "-q", "-b", "main")
			gitRun(t, fr.Root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "h494 fixture")
			head, err := snapshot.DecodeHead(h494Read(t, filepath.Join(fr.StateDir, "head.json")))
			if err != nil {
				t.Fatal(err)
			}
			setup := time.Now()
			for seq := head.LastSeq.Uint64() + 1; seq <= 5000; seq++ {
				id := fmt.Sprintf("h494-history-%d", seq)
				path, _ := snapshot.RequestPath(id)
				h494AppendReceipt(t, fr, "MUTATION", map[string][]byte{path: h494RequestBytes(id, seq, wire.Sum([]byte(id)), false)}, id, true, true, false)
			}
			h494Save(t, evidence, "fixture.json", map[string]any{"root": fr.Root, "common": fr.CommonDir, "state": fr.StateDir, "intent": fr.IntentDir, "receipts": 5000, "setupMS": float64(time.Since(setup)) / float64(time.Millisecond), "binary": binary, "tickets": ids, "limits": []string{"synthetic fixture, reporter exact binary/history unknown", "one wave, no p95 or fairness guarantee", "no observer timing collected for fresh CLI", "synchronous evidence writes add overhead"}})
			if !h494Audit(t, s, evidence, "audit-before") {
				return
			}
			var wg sync.WaitGroup
			start := make(chan struct{})
			counts := make([]int, 10)
			attempts := make([]string, 10)
			generations := make([]wire.Size, 10)
			saved := make([][][]string, 10)
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()
			began := time.Now()
			for i, id := range ids {
				wg.Add(1)
				go func(i int, id string) {
					defer wg.Done()
					<-start
					for _, verb := range []string{"claim", "renew", "release"} {
						args := []string{verb, "--request-id", fmt.Sprintf("h494-%d-%s", i, verb)}
						if verb == "claim" {
							args = append(args, id, "--holder", fmt.Sprintf("agent-%d", i), "--lease-minutes", "60", "--scope", fmt.Sprintf("src/%d", i))
						} else {
							args = append(args, "--attempt", attempts[i], "--generation", string(generations[i]))
							if verb == "renew" {
								args = append(args, "--lease-minutes", "60")
							}
						}
						result, err := h494CLI(t, ctx, binary, fr.Root, evidence, fmt.Sprintf("worker-%02d-%s", i, verb), args)
						if err != nil {
							t.Errorf("worker %d %s: %v", i, verb, err)
							return
						}
						item := result.Items[0]
						if item.Replayed {
							t.Errorf("unexpected initial replay: worker %d %s", i, verb)
							return
						}
						if verb == "claim" {
							if item.AttemptID == "" || item.Generation == "" || item.Generation == "0" {
								t.Errorf("missing claim identity %+v", item)
								return
							}
							attempts[i] = item.AttemptID
							generations[i] = item.Generation
						} else if item.AttemptID != attempts[i] || item.Generation != generations[i] {
							t.Errorf("changed attempt identity worker %d: %+v", i, item)
							return
						}
						counts[i]++
						saved[i] = append(saved[i], append([]string(nil), args...))
					}
				}(i, id)
			}
			close(start)
			wg.Wait()
			elapsed := time.Since(began)
			cancel()
			completed := 0
			for _, n := range counts {
				completed += n
			}
			h494Save(t, evidence, "workload.json", map[string]any{"elapsedMS": float64(elapsed) / float64(time.Millisecond), "timeoutSeconds": 180, "lockWaitSeconds": 30, "expectedOperations": 30, "completedOperations": completed, "notCompletedOperations": 30 - completed, "perWorkerCompleted": counts, "attempts": attempts, "generations": generations, "success": completed == 30 && elapsed <= 180*time.Second})
			h494Audit(t, s, evidence, "audit-after-workload")
			raw := h494Read(t, filepath.Join(fr.StateDir, "reservations.json"))
			reservations, err := snapshot.DecodeReservations(raw)
			if err != nil {
				t.Fatal(err)
			}
			active := 0
			for _, en := range reservations.Entries {
				if en.State == "ACTIVE" {
					active++
				}
			}
			h494Save(t, evidence, "reservations-final.json", map[string]any{"active": active, "raw": string(raw)})
			for i, id := range attempts {
				if id == "" {
					continue
				}
				raw := h494Read(t, filepath.Join(fr.StateDir, "attempts", id+".json"))
				a, err := snapshot.DecodeAttempt(raw)
				h494Save(t, evidence, fmt.Sprintf("attempt-%02d-final.json", i), map[string]any{"raw": string(raw), "error": fmt.Sprint(err)})
				if err != nil {
					t.Error(err)
					continue
				}
				if counts[i] == 3 && (a.Generation != generations[i] || a.Phase != "CANCELLED" || a.Quiescence != "FENCED") {
					t.Errorf("worker %d final state %+v", i, a)
				}
			}
			if completed != 30 || elapsed > 180*time.Second || active != 0 {
				t.Errorf("qualification failed: completed=%d elapsed=%s active=%d", completed, elapsed, active)
				return
			}
			before := storeDigest(t, s.repo)
			for _, index := range []int{0, 2} {
				replayCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
				r, err := h494CLI(t, replayCtx, binary, fr.Root, evidence, fmt.Sprintf("postwave-replay-%d", index), saved[0][index])
				stop()
				if err != nil {
					t.Error(err)
					continue
				}
				if !r.Items[0].Replayed || r.Items[0].AttemptID != attempts[0] || r.Items[0].Generation != generations[0] {
					t.Errorf("bad replay %+v", r)
				}
			}
			after := storeDigest(t, s.repo)
			h494Save(t, evidence, "replay-state.json", map[string]any{"before": before, "after": after, "unchanged": before == after})
			if before != after {
				t.Error("exact-ID replay changed store")
			}
			h494Audit(t, s, evidence, "audit-after-replay")
		})
	})
}

// Replays return the original attempt without committing a new receipt.
func TestGH494CLIResultContract(t *testing.T) {
	fresh := h494Result{Outcome: "OK", Items: []h494Item{{Outcome: mutation.OutcomeCompleted, Receipt: "receipt-5001"}}}
	for _, tc := range []struct {
		name   string
		result h494Result
		wantOK bool
	}{
		{"fresh receipt", fresh, true},
		{"fresh missing receipt", h494Result{Outcome: "OK", Items: []h494Item{{Outcome: mutation.OutcomeCompleted}}}, false},
		{"refused envelope", h494Result{Outcome: "REFUSED", Items: fresh.Items}, false},
		{"refused replay", h494Result{Outcome: "OK", Items: []h494Item{{Outcome: "REFUSED", Replayed: true}}}, false},
		{"missing item", h494Result{Outcome: "OK"}, false},
		{"multiple items", h494Result{Outcome: "OK", Items: []h494Item{fresh.Items[0], fresh.Items[0]}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := h494CLIResult(tc.result); (err == nil) != tc.wantOK {
				t.Fatalf("result acceptance = %v, want %v: %v", err == nil, tc.wantOK, err)
			}
		})
	}
	// Captured claim/release responses from the original mixed wave. These
	// regress the decoder/helper and identity assertions; they do not rerun it.
	for _, tc := range []struct{ name, raw string }{
		{"claim replay", "{\"codes\":[],\"command\":[\"claim\"],\"items\":[{\"actorAuthentication\":\"NOT_OBSERVED\",\"attemptId\":\"attempt:acme:main:84cec7a91e18462c471b714d8ed55080\",\"durability\":\"NOT_OBSERVED\",\"expired\":[],\"generation\":\"4\",\"outcome\":\"COMPLETED\",\"reaped\":[],\"receipt\":\"\",\"releaseId\":\"\",\"replayed\":true,\"resultingAcceptanceRevision\":null,\"resultingRevision\":null,\"ticketId\":\"ticket:acme:main:AT-0002\"}],\"mutation\":null,\"outcome\":\"OK\",\"page\":null,\"profile\":\"taskman-command-result/0\",\"snapshot\":null,\"untrusted\":[],\"warnings\":[\"the actor binding is a recorded local-operator claim, not an authentication (decision 0003); a real queue still needs the §7.4 cutover record\"]}\n"},
		{"release replay", "{\"codes\":[],\"command\":[\"release\"],\"items\":[{\"actorAuthentication\":\"NOT_OBSERVED\",\"attemptId\":\"attempt:acme:main:84cec7a91e18462c471b714d8ed55080\",\"durability\":\"NOT_OBSERVED\",\"expired\":[],\"generation\":\"4\",\"outcome\":\"COMPLETED\",\"reaped\":[],\"receipt\":\"\",\"releaseId\":\"\",\"replayed\":true,\"resultingAcceptanceRevision\":null,\"resultingRevision\":null,\"ticketId\":\"\"}],\"mutation\":null,\"outcome\":\"OK\",\"page\":null,\"profile\":\"taskman-command-result/0\",\"snapshot\":null,\"untrusted\":[],\"warnings\":[\"the actor binding is a recorded local-operator claim, not an authentication (decision 0003); a real queue still needs the §7.4 cutover record\"]}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result h494Result
			if err := json.Unmarshal([]byte(tc.raw), &result); err != nil {
				t.Fatal(err)
			}
			if err := h494CLIResult(result); err != nil {
				t.Fatal(err)
			}
			item := result.Items[0]
			if !item.Replayed || item.Receipt != "" || item.AttemptID != "attempt:acme:main:84cec7a91e18462c471b714d8ed55080" || item.Generation != wire.Size("4") {
				t.Fatalf("changed replay identity: %+v", item)
			}
		})
	}
}
