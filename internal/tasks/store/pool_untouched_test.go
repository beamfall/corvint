package store_test

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/tasks/archive"
	"io"
	"os"
	"path/filepath"
	"reflect"
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
			up, err := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("compatible", "3", wire.EncodeFile(policy)), now(t))
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

// Terminal qualification only: real receipt-boundary interruption and archive
// production are deliberately outside the ZERO_GEN replay selector.
func TestPoolLaneUntouched_ArchiveCrash(t *testing.T) {
	for _, boundary := range []string{"RECEIPT", "POST"} {
		t.Run(boundary, func(t *testing.T) {
			s := newLeaseStore(t)
			exclusionPolicy(t, s, wire.Null())
			id := s.ticket(t, "crash")
			c := claimOf(id, "src")
			c.Pool, c.Stage = "db", "review"
			a := s.lease(t, "claim", c, 0, nil)
			rel := releaseOf(a)
			rel.LaneUntouched = true
			rel.Evidence = "local:unused"
			fired := false
			restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
				if !fired && a.Role == boundary {
					fired = true
					return errInjected
				}
				return nil
			})
			_, err := store.Lease(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "release", Root: s.root, Lease: rel}, s.at(t, 0))
			restore()
			if !fired || !errors.Is(err, errInjected) {
				t.Fatal("fault not reached", err)
			}
			// Before any afterimage is published, the live attempt still occupies its lane.
			if got := s.attempt(t, a.AttemptID); got.Phase != "RUNNING" || got.LaneUntouchedAttestation != nil {
				t.Fatal("partial terminal state")
			}
			raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "pools.json"))
			if err != nil {
				t.Fatal(err)
			}
			pool, err := snapshot.DecodePools(raw)
			if err != nil || len(pool.Entries) != 1 || pool.Entries[0].State != "ALLOCATED" {
				t.Fatal("freed before receipt recovery", err)
			}
			recovered := s.lease(t, "release", rel, 0, nil)
			if recovered.LaneUntouchedAttestation == nil || recovered.Outcome.Replayed != (boundary == "POST") {
				t.Fatal("recovery", recovered)
			}
			terminal := s.attempt(t, a.AttemptID)
			if terminal.Phase != "CANCELLED" || terminal.Quiescence != "FENCED" || len(s.entries(t)) != 0 {
				t.Fatal("incomplete recovery")
			}
			raw, err = os.ReadFile(filepath.Join(s.repo.StateDir, "pools.json"))
			if err != nil {
				t.Fatal(err)
			}
			pool, err = snapshot.DecodePools(raw)
			if err != nil || len(pool.Entries) != 0 {
				t.Fatal("occupancy remains", err)
			}
			var exported bytes.Buffer
			if _, err := archive.Export(archive.ExportOptions{Repo: s.repo, Stdout: &exported}); err != nil {
				t.Fatal(err)
			}
			verified, err := archive.Verify(bytes.NewReader(exported.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			_ = verified
			// Read tar member bytes independently of the verifier's returned claims.
			tr := tar.NewReader(bytes.NewReader(exported.Bytes()))
			want, err := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", a.AttemptID+".json"))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for {
				h, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if h.Name == "attempts/"+a.AttemptID+".json" {
					got, err := io.ReadAll(tr)
					if err != nil || !bytes.Equal(want, got) {
						t.Fatal("archive changed attestation", err)
					}
					found = true
				}
			}
			if !found {
				t.Fatal("archive omitted attempt")
			}
		})
	}
}
