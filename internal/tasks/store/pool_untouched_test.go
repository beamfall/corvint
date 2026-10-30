package store_test

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// This selector uses durable disposable stores, but no archive producer or
// native fixture runner. Archive/crash qualification has its own terminal gate.
func TestPoolLaneUntouched_Replay(t *testing.T) {
	t.Parallel()
	t.Run("receipt-payload", untouchedReceiptPayload)
	t.Run("CAL-V0-067", func(t *testing.T) {
		for _, reason := range []string{"", wire.CodeHandoff, wire.CodeReviewReturned} {
			t.Run("reason-"+reason, func(t *testing.T) {
				s := newLeaseStore(t)
				marker := filepath.Join(t.TempDir(), "cleanup-must-not-run")
				command := obj("argv", wire.Strings([]string{"/usr/bin/touch", marker}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3"))
				policy := exclusionPolicy(t, s, obj("review", obj("cleanup", command)))
				id := s.ticket(t, "untouched")
				c := claimOf(id, "src")
				c.Pool, c.Stage = "db", "review"
				a := s.lease(t, "direct", c, 0, nil)
				if a.PoolAllocation == nil || s.attempt(t, a.AttemptID).DirectPoolAdmission == nil {
					t.Fatal("origin absent", a)
				}
				rel := releaseOf(a)
				rel.LaneUntouched = true
				rel.Evidence = "local:unused"
				rel.Reason = reason
				fresh := s.lease(t, "untouched-release", rel, 0, nil)
				if fresh.Outcome.Outcome != mutation.OutcomeCompleted || fresh.LaneUntouchedAttestation == nil || !reflect.DeepEqual(fresh.PoolAllocation, a.PoolAllocation) {
					t.Fatalf("fresh %+v", fresh)
				}
				original := wire.EncodeFile(snapshot.LaneUntouchedAttestationValue(fresh.LaneUntouchedAttestation))
				if len(s.entries(t)) != 0 {
					t.Fatal("reservation retained")
				}
				terminal := s.attempt(t, a.AttemptID)
				if terminal.Phase != "CANCELLED" || terminal.Quiescence != "FENCED" {
					t.Fatal("terminal", terminal)
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatal("cleanup ran", err)
				}
				successor := s.lease(t, "successor", c, 0, nil)
				if successor.PoolAllocation == nil || successor.Generation == a.Generation || successor.PoolAllocation.AllocationID == a.PoolAllocation.AllocationID {
					t.Fatal("successor", successor)
				}
				if s.attempt(t, successor.AttemptID).DirectPoolAdmission != nil {
					t.Fatal("retry inherited origin")
				}
				policy.Obj.Set("policyVersion", str("4"))
				updated, err := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("new-policy", "3", wire.EncodeFile(policy)), now(t))
				if err != nil || updated.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Fatal(updated, err)
				}
				before := fixtureState(t, s)
				replay := s.lease(t, "untouched-release", rel, 0, nil)
				if replay.Kind != "Replay" || !bytes.Equal(original, wire.EncodeFile(snapshot.LaneUntouchedAttestationValue(replay.LaneUntouchedAttestation))) || !reflect.DeepEqual(replay.PoolAllocation, a.PoolAllocation) || replay.Generation != a.Generation {
					t.Fatal("original payload changed", replay)
				}
				if !reflect.DeepEqual(before, fixtureState(t, s)) {
					t.Fatal("replay wrote state")
				}
				bad := rel
				bad.Evidence = "local:other"
				conflict := s.lease(t, "untouched-release", bad, 0, nil)
				if conflict.Outcome.Outcome == mutation.OutcomeCompleted {
					t.Fatal("changed evidence replayed")
				}
				bad = rel
				bad.LaneUntouched = false
				if bad.Reason == "" {
					bad.Evidence = ""
				}
				conflict = s.lease(t, "untouched-release", bad, 0, nil)
				if conflict.Outcome.Outcome == mutation.OutcomeCompleted {
					t.Fatal("changed flag replayed")
				}
			})
		}
	})
	t.Run("strict-policy-versus-compatible-handoff", func(t *testing.T) {
		for _, flag := range []bool{false, true} {
			s := newLeaseStore(t)
			policy := exclusionPolicy(t, s, wire.Null())
			id := s.ticket(t, "strict")
			c := claimOf(id, "src")
			c.Pool, c.Stage = "db", "review"
			a := s.lease(t, "claim", c, 0, nil)
			policy.Obj.Set("policyVersion", str("4"))
			up, err := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("compatible", "3", wire.EncodeFile(policy)), s.at(t, 0))
			if err != nil || up.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatal(up, err)
			}
			before := fixtureState(t, s)
			rel := releaseOf(a)
			rel.Reason = wire.CodeHandoff
			rel.Evidence = "local:unused"
			rel.LaneUntouched = flag
			r := s.lease(t, "handoff", rel, 0, nil)
			if flag {
				if !r.Outcome.HasCode(wire.CodeStalePolicy) || !reflect.DeepEqual(before, fixtureState(t, s)) {
					t.Fatal("flag accepted compatible change", r)
				}
			} else {
				if r.Outcome.Outcome != mutation.OutcomeCompleted || r.LaneUntouchedAttestation != nil {
					t.Fatal("ordinary handoff changed", r)
				}
				raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "pools.json"))
				if err != nil {
					t.Fatal(err)
				}
				p, err := snapshot.DecodePools(raw)
				if err != nil || len(p.Entries) != 1 || p.Entries[0].State != "QUARANTINED" {
					t.Fatal("ordinary quarantine", err)
				}
			}
		}
	})
}
func fixtureState(t *testing.T, s *leaseStore) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, path := range []string{"head.json", "pools.json", "reservations.json"} {
		raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, path))
		if err != nil {
			t.Fatal(err)
		}
		out[path] = raw
	}
	return out
}

// The ordinary writer inlines bounded attempts. This synthetic receipt-layout
// fixture exercises the equally legal bounded-blob representation; it is not
// a claim that the writer emitted a blob or a qualification of imported history.
func untouchedReceiptPayload(t *testing.T) {
	for _, form := range []string{"inline", "blob"} {
		t.Run(form, func(t *testing.T) {
			s := newLeaseStore(t)
			exclusionPolicy(t, s, wire.Null())
			id := s.ticket(t, "payload")
			c := claimOf(id, "src")
			c.Pool, c.Stage = "db", "review"
			a := s.lease(t, "claim", c, 0, nil)
			rel := releaseOf(a)
			rel.LaneUntouched = true
			rel.Evidence = "local:unused"
			fresh := s.lease(t, "release", rel, 0, nil)
			if fresh.LaneUntouchedAttestation == nil {
				t.Fatal(fresh)
			}
			receiptPath := filepath.Join(s.repo.StateDir, "receipts", fresh.Receipt)
			receiptRaw, err := os.ReadFile(receiptPath)
			if err != nil {
				t.Fatal(err)
			}
			v, err := wire.Parse(receiptRaw)
			if err != nil {
				t.Fatal(err)
			}
			posts, _ := v.Obj.Get("post")
			var payload []byte
			var target *wire.Object
			for _, p := range posts.Arr {
				path, _ := p.Obj.Get("path")
				if path.Str == "attempts/"+a.AttemptID+".json" {
					record, _ := p.Obj.Get("record")
					payload = wire.EncodeFile(record)
					target = p.Obj
				}
			}
			if target == nil {
				t.Fatal("no attempt post")
			}
			blobPath := filepath.Join(s.repo.StateDir, "evidence", string(wire.Sum(payload)))
			if form == "blob" {
				fixture.Write(t, blobPath, payload)
				target.Set("record", wire.Null())
				target.Set("blobSha256", str(string(wire.Sum(payload))))
				receiptRaw = wire.EncodeFile(v)
				fixture.Write(t, receiptPath, receiptRaw)
				editJSON(t, filepath.Join(s.repo.StateDir, "head.json"), func(h wire.Value) { h.Obj.Set("lastReceiptSha256", str(string(wire.Sum(receiptRaw)))) })
			}
			replay := s.lease(t, "release", rel, 0, nil)
			if replay.LaneUntouchedAttestation == nil || !reflect.DeepEqual(replay.LaneUntouchedAttestation, fresh.LaneUntouchedAttestation) {
				t.Fatal("payload lost", replay)
			}
			// A damaged or missing original payload must refuse before touching state.
			if form == "blob" {
				if err := os.Remove(blobPath); err != nil {
					t.Fatal(err)
				}
			} else {
				fixture.Write(t, receiptPath, bytes.Replace(receiptRaw, []byte(`"noLaneAccess":true`), []byte(`"noLaneAccess":false`), 1))
			}
			before := storeDigest(t, s.repo)
			rep, err := store.Lease(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "release", Root: s.root, Lease: rel}, s.at(t, 0))
			if err == nil && rep.Outcome.Outcome == mutation.OutcomeCompleted {
				t.Fatal("damaged payload accepted")
			}
			if storeDigest(t, s.repo) != before {
				t.Fatal("damaged replay wrote")
			}
		})
	}
}

// Terminal qualification only. The publication hook covers each emitted artifact;
// stage-file/descriptor writes are not artifact callbacks and are not injected here.
func TestPoolLaneUntouched_ArchiveCrash(t *testing.T) {
	s, a, rel := untouchedCrashFixture(t)
	var order []string
	restore := store.SetPublishFaultForTest(func(x transaction.Artifact) error {
		order = append(order, untouchedArtifactKey(x, a.AttemptID))
		return nil
	})
	var original *store.Report
	func() { defer restore(); original = s.lease(t, "release", rel, 0, nil) }()
	if original.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatal(original)
	}
	receiptAt := -1
	required := map[string]bool{"RECEIPT": false, "POST:attempt": false, "POST:pools.json": false, "POST:reservations.json": false, "POST:request": false, "HEAD:head.json": false}
	for i, key := range order {
		if _, ok := required[key]; !ok || required[key] {
			t.Fatalf("unexpected or duplicate opt-in publication %q in %v", key, order)
		}
		required[key] = true
		if key == "RECEIPT" {
			receiptAt = i
		}
	}
	for key, seen := range required {
		if !seen {
			t.Fatalf("missing publication discriminator %s: %v", key, order)
		}
	}
	t.Logf("actual opt-in artifact order: %v; EVIDENCE artifacts: none emitted; STAGE artifact role: none emitted (stage-file/descriptor internals NOT_INJECTED)", order)
	for k, key := range order {
		t.Run(fmt.Sprintf("%02d-%s", k, key), func(t *testing.T) {
			s, a, rel := untouchedCrashFixture(t)
			origin := s.attempt(t, a.AttemptID).DirectPoolAdmission
			head, count := journalState(t, s.repo)
			n := 0
			fired := false
			restore := store.SetPublishFaultForTest(func(x transaction.Artifact) error {
				if n >= len(order) || untouchedArtifactKey(x, a.AttemptID) != order[n] {
					t.Fatalf("publication order changed at %d: %+v", n, x.Description)
				}
				at := n
				n++
				if at == k {
					fired = true
					return errInjected
				}
				return nil
			})
			var err error
			func() {
				defer restore()
				_, err = store.Lease(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "release", Root: s.root, Lease: rel}, s.at(t, 0))
			}()
			if !fired || !errors.Is(err, errInjected) || n != k+1 {
				t.Fatal("fault not reached exactly", key, n, err)
			}
			committed := k > receiptAt
			gotHead, gotCount := journalState(t, s.repo)
			if gotHead != head || gotCount != count+btoi(committed) {
				t.Fatal("wrong receipt/head fence", key)
			}
			// Intermediate post files are not a readable committed snapshot. They must
			// also never make a lane free before its terminal attestation is published.
			current := s.attempt(t, a.AttemptID)
			pool := untouchedPoolState(t, s)
			if len(pool.Entries) == 0 || len(s.entries(t)) == 0 {
				untouchedTerminal(t, current, origin, a.PoolAllocation)
			}
			if current.Live() && (len(pool.Entries) != 1 || pool.Entries[0].State != "ALLOCATED" || len(s.entries(t)) != 1) {
				t.Fatal("live attempt lost occupancy or reservation")
			}
			if _, err := snapshot.Probe(s.repo.StateDir); committed {
				if wire.CodeOf(err) != wire.CodeRedoPending {
					t.Fatal("partial committed afterimages exposed", err)
				}
			} else if err != nil {
				t.Fatal("uncommitted interruption changed snapshot", err)
			}
			recovered := s.lease(t, "release", rel, 0, nil)
			if recovered.Outcome.Outcome != mutation.OutcomeCompleted || recovered.Redone != committed || recovered.Outcome.Replayed != committed || recovered.LaneUntouchedAttestation == nil {
				t.Fatal("receipt-bound recovery", recovered)
			}
			terminal := s.attempt(t, a.AttemptID)
			untouchedTerminal(t, terminal, origin, a.PoolAllocation)
			if !reflect.DeepEqual(recovered.LaneUntouchedAttestation, terminal.LaneUntouchedAttestation) || !reflect.DeepEqual(recovered.PoolAllocation, a.PoolAllocation) || len(s.entries(t)) != 0 || len(untouchedPoolState(t, s).Entries) != 0 {
				t.Fatal("incomplete recovered response/afterimages")
			}
			auditOK(t, s.repo)
			before := storeDigest(t, s.repo)
			again := s.lease(t, "release", rel, 0, nil)
			if !again.Outcome.Replayed || !bytes.Equal(untouchedResponse(recovered), untouchedResponse(again)) || storeDigest(t, s.repo) != before {
				t.Fatal("recovery replay changed payload/state")
			}
		})
	}
	t.Run("archive-historical-replay-after-successor", func(t *testing.T) { untouchedArchivedReplay(t, s, a, rel, original) })
}

func untouchedArtifactKey(a transaction.Artifact, id string) string {
	if a.Role == "RECEIPT" {
		return "RECEIPT"
	}
	if a.Target == "attempts/"+id+".json" {
		return a.Role + ":attempt"
	}
	if strings.HasPrefix(a.Target, "requests/") {
		return a.Role + ":request"
	}
	return a.Role + ":" + a.Target
}
func untouchedCrashFixture(t *testing.T) (*leaseStore, *store.Report, transaction.LeaseRequest) {
	t.Helper()
	s := newLeaseStore(t)
	exclusionPolicy(t, s, wire.Null())
	id := s.ticket(t, "crash")
	c := claimOf(id, "src")
	c.Pool, c.Stage = "db", "review"
	a := s.lease(t, "claim", c, 0, nil)
	if a.PoolAllocation == nil {
		t.Fatal("claim allocation missing")
	}
	rel := releaseOf(a)
	rel.LaneUntouched = true
	rel.Evidence = "local:unused"
	return s, a, rel
}
func untouchedPoolState(t *testing.T, s *leaseStore) *snapshot.PoolState {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "pools.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := snapshot.DecodePools(raw)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func untouchedTerminal(t *testing.T, a *snapshot.Attempt, origin *snapshot.DirectPoolAdmission, allocation *snapshot.PoolAllocation) {
	t.Helper()
	if a.Phase != "CANCELLED" || a.Quiescence != "FENCED" || a.LaneUntouchedAttestation == nil || !reflect.DeepEqual(a.DirectPoolAdmission, origin) || !reflect.DeepEqual(a.PoolAllocation, allocation) || !reflect.DeepEqual(&a.LaneUntouchedAttestation.DirectPoolAdmission, origin) {
		t.Fatal("missing complete terminal attestation/origin/allocation")
	}
}
func untouchedResponse(r *store.Report) []byte {
	return wire.EncodeFile(obj("attemptId", str(r.AttemptID), "generation", str(string(r.Generation)), "poolAllocation", snapshot.PoolAllocationValue(r.PoolAllocation), "laneUntouchedAttestation", snapshot.LaneUntouchedAttestationValue(r.LaneUntouchedAttestation)))
}

// Recreate a separate test-owned fixture at the recorded authority path, keeping
// the original fixture byte-preserved next door. This avoids rewriting immutable
// primaryWorktree/history bindings and does not implement a public restore API.
func untouchedArchivedReplay(t *testing.T, s *leaseStore, a *store.Report, rel transaction.LeaseRequest, original *store.Report) {
	t.Helper()
	originalResponse := untouchedResponse(original)
	originalAttempt, err := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", a.AttemptID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	originalReceipt, err := os.ReadFile(filepath.Join(s.repo.StateDir, "receipts", original.Receipt))
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := snapshot.DecodeAttempt(originalAttempt)
	if err != nil {
		t.Fatal(err)
	}
	claim := claimOf(terminal.TicketID.Raw, "src")
	claim.Pool, claim.Stage = "db", "review"
	successor := s.lease(t, "successor", claim, 0, nil)
	if successor.AttemptID != a.AttemptID || successor.Generation == a.Generation || successor.PoolAllocation == nil || successor.PoolAllocation.AllocationID == a.PoolAllocation.AllocationID {
		t.Fatal("successor did not overwrite current attempt", successor)
	}
	successorRaw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", a.AttemptID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(originalAttempt, successorRaw) {
		t.Fatal("historical discriminator absent")
	}
	var exported bytes.Buffer
	if _, err := archive.Export(archive.ExportOptions{Repo: s.repo, Stdout: &exported}); err != nil {
		t.Fatal(err)
	}
	verified, err := archive.Verify(bytes.NewReader(exported.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	tr := tar.NewReader(bytes.NewReader(exported.Bytes()))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		if h.Name != archive.ManifestName {
			files[h.Name] = raw
		}
	}
	if len(files) != len(verified.Manifest.Files) {
		t.Fatal("archive file inventory changed")
	}
	for _, f := range verified.Manifest.Files {
		if raw, ok := files[f.Path]; !ok || uint64(len(raw)) != f.Bytes.Uint64() || wire.Sum(raw) != f.Sha256 {
			t.Fatal("preserved member differs", f.Path)
		}
	}
	if !bytes.Equal(files["attempts/"+a.AttemptID+".json"], successorRaw) || !bytes.Equal(files["receipts/"+original.Receipt], originalReceipt) {
		t.Fatal("archive did not retain successor and immutable release receipt")
	}
	rc, err := snapshot.DecodeReceipt(files["receipts/"+original.Receipt])
	if err != nil {
		t.Fatal(err)
	}
	var historical []byte
	for _, p := range rc.Post {
		if p.Path == "attempts/"+a.AttemptID+".json" {
			if p.Record != nil {
				historical = wire.EncodeFile(*p.Record)
			} else if p.BlobSha256 != nil {
				historical = files["evidence/"+string(*p.BlobSha256)]
			}
			if p.Sha256 == nil || wire.Sum(historical) != *p.Sha256 {
				t.Fatal("archived historical payload digest")
			}
		}
	}
	if !bytes.Equal(historical, originalAttempt) {
		t.Fatal("archive lost original terminal afterimage")
	}
	decoded, err := snapshot.DecodeAttempt(historical)
	if err != nil {
		t.Fatal(err)
	}
	untouchedTerminal(t, decoded, terminal.DirectPoolAdmission, a.PoolAllocation)
	originalRoot := s.repo.PrimaryWorktree
	backup := originalRoot + "-preserved"
	preservedBefore := fixture.TreeSnapshot(t, originalRoot)
	if err := os.Rename(originalRoot, backup); err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, filepath.Join(originalRoot, ".git", "HEAD"), []byte("ref: refs/heads/main\n"))
	for path, raw := range files {
		dest := filepath.Join(originalRoot, ".git", "taskman", filepath.FromSlash(path))
		if strings.HasPrefix(path, "intent/") {
			dest = filepath.Join(originalRoot, intent.Dir, filepath.FromSlash(strings.TrimPrefix(path, "intent/")))
		}
		fixture.Write(t, dest, raw)
	}
	for _, dir := range []string{"staging", "receipts", "evidence", "pinned", "requests", "attempts"} {
		if err := os.MkdirAll(filepath.Join(originalRoot, ".git", "taskman", dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := intent.Resolve(originalRoot)
	if err != nil {
		t.Fatal(err)
	}
	for path, raw := range files {
		dest := filepath.Join(restored.StateDir, filepath.FromSlash(path))
		if strings.HasPrefix(path, "intent/") {
			dest = filepath.Join(restored.PrimaryWorktree, intent.Dir, filepath.FromSlash(strings.TrimPrefix(path, "intent/")))
		}
		got, err := os.ReadFile(dest)
		if err != nil || !bytes.Equal(raw, got) {
			t.Fatal("reconstruction changed archive bytes", path, err)
		}
	}
	restoredBefore := storeDigest(t, restored)
	replay, err := store.Lease(context.Background(), restored, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "release", Root: s.root, Lease: rel}, s.at(t, 0))
	if err != nil || replay == nil || replay.LaneUntouchedAttestation == nil || !replay.Outcome.Replayed || !bytes.Equal(untouchedResponse(replay), originalResponse) {
		t.Fatal("archived original receipt replay", replay, err)
	}
	if storeDigest(t, restored) != restoredBefore {
		t.Fatal("archived replay changed successor/history")
	}
	got, err := os.ReadFile(filepath.Join(restored.StateDir, "attempts", a.AttemptID+".json"))
	if err != nil || !bytes.Equal(got, successorRaw) {
		t.Fatal("successor overwritten by historical replay", err)
	}
	if !fixture.SameTree(preservedBefore, fixture.TreeSnapshot(t, backup)) {
		t.Fatal("separate source fixture modified")
	}
	auditOK(t, restored)
}
