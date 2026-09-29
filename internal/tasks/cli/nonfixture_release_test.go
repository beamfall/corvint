package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-027: the actual executable admits release intent without granting execution.
func TestCALV0027_CompiledNonfixtureReleaseLifecycle(t *testing.T) {
	t.Run("CAL-V0-027 compiled lifecycle", testCALV0027_CompiledNonfixtureReleaseLifecycle)
}

func testCALV0027_CompiledNonfixtureReleaseLifecycle(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "corvint-tasks")
	build := exec.Command("go", "build", "-o", binary, "../../../cmd/corvint-tasks")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	r := fixture.TempRepo(t)
	q := fixture.QueueValue()
	q.Obj.Set("fixture", wire.Bool(false))
	queue := wire.EncodeFile(q)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), queue)
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	call := func(args ...string) *wire.Result {
		t.Helper()
		c := exec.Command(binary, args...)
		c.Dir = r.Root
		out, err := c.Output()
		result, decodeErr := wire.DecodeResult(out)
		if decodeErr != nil {
			t.Fatalf("%v: %v %v %s", args, err, decodeErr, out)
		}
		if (err == nil) != (result.Outcome == wire.OutcomeOK) {
			t.Fatalf("exit/envelope mismatch: %v", args)
		}
		return result
	}
	ok := func(args ...string) *wire.Result {
		t.Helper()
		x := call(args...)
		if x.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %+v", args, x)
		}
		return x
	}
	git(t, r.Root, "init", "-b", "main")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-m", "base")
	ok("init")
	ticket := ok("ticket", "create", "--request-id", "compiled-ticket", "--payload", createPayloadJSON)
	id := field(ticket.Items[0], "ticketId").Str
	queueBefore, err := os.ReadFile(filepath.Join(r.IntentDir, "queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	policyBefore, err := os.ReadFile(filepath.Join(r.IntentDir, "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	ok("release", "create", "--request-id", "compiled-release", "--target", "v1", "--payload", releaseCreatePayload("1", "Initial", id, ""))
	ok("release", "update", "--request-id", "compiled-update", "--target", "v1", "--expected-revision", "1", "--payload", releaseCreatePayload("1", "Updated", id, ""))
	show := ok("release", "show", "v1")
	if field(show.Items[0], "revision").Str != "2" || field(show.Items[0], "title").Str != "Updated" {
		t.Fatalf("show: %+v", show)
	}
	ok("receipt", "audit")
	claim := call("claim", id, "--holder", "compiled-holder", "--request-id", "compiled-claim")
	if !hasCode(claim, wire.CodeCutoverMissing) {
		t.Fatalf("claim authority changed: %+v", claim)
	}
	raw, err := os.ReadFile(filepath.Join(r.IntentDir, "queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, queueBefore) {
		t.Fatal("release changed queue bytes")
	}
	policyAfter, err := os.ReadFile(filepath.Join(r.IntentDir, "policy.json"))
	if err != nil || !bytes.Equal(policyBefore, policyAfter) {
		t.Fatal("release changed policy", err)
	}
	after, err := intent.DecodeQueue(raw)
	if err != nil || after.Fixture || after.ImportMapSha256 != nil || after.ExecutionCutover != nil {
		t.Fatalf("queue: %+v %v", after, err)
	}
}

func releaseQueueBytes(isFixture bool) []byte {
	q := fixture.QueueValue()
	q.Obj.Set("fixture", wire.Bool(isFixture))
	return wire.EncodeFile(q)
}

// CAL-V0-027: retain the same lifecycle, CAS, policy, source and evidence assertions.
func TestCALV0027_NonfixtureReleaseBindings(t *testing.T) {
	t.Run("CAL-V0-027 witness", testCALV0027_NonfixtureReleaseBindings)
}

func testCALV0027_NonfixtureReleaseBindings(t *testing.T) {
	t.Run("nonfixturetestPublicOutputDrivesOrderedReleasePromotion", func(t *testing.T) { nonfixturetestPublicOutputDrivesOrderedReleasePromotion(t, false) })
	t.Run("nonfixtureTestTMV0028_AS38_AttestedPromotionAndImmutability", func(t *testing.T) { nonfixtureTestTMV0028_AS38_AttestedPromotionAndImmutability(t, false) })
	t.Run("nonfixtureTestTMV0028_AS38_CandidateSurvivesTaskmanWritesAndSourceEditInvalidates", func(t *testing.T) {
		nonfixtureTestTMV0028_AS38_CandidateSurvivesTaskmanWritesAndSourceEditInvalidates(t, false)
	})
	t.Run("nonfixtureTestTMV0028_AS38_ReleaseDivergenceNeedsKeepJournal", func(t *testing.T) { nonfixtureTestTMV0028_AS38_ReleaseDivergenceNeedsKeepJournal(t, false) })
	t.Run("nonfixtureTestTMV0028_AS38_DurableTwoReleaseCreateAndReplay", func(t *testing.T) { nonfixtureTestTMV0028_AS38_DurableTwoReleaseCreateAndReplay(t, false) })
	t.Run("nonfixtureTestTMV0028_AS38_PolicyCanRemoveReleaseAuthorization", func(t *testing.T) { nonfixtureTestTMV0028_AS38_PolicyCanRemoveReleaseAuthorization(t, false) })
	t.Run("nonfixtureTestTMV0030_AS38_PolicyUpdateInvalidatesReleaseCandidate", func(t *testing.T) { nonfixtureTestTMV0030_AS38_PolicyUpdateInvalidatesReleaseCandidate(t, false) })
}

// CAL-V0-027: absent/failed gates, missing predecessors and changed acceptance remain blockers.
func TestCALV0027_NonfixtureReleaseReadinessRefusals(t *testing.T) {
	t.Run("CAL-V0-027 witness", testCALV0027_NonfixtureReleaseReadinessRefusals)
}

func testCALV0027_NonfixtureReleaseReadinessRefusals(t *testing.T) {
	for _, mode := range []string{"missing-gate", "failed-gate", "acceptance", "predecessor"} {
		t.Run(mode, func(t *testing.T) {
			r := fixture.TempRepo(t)
			fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), releaseQueueBytes(false))
			fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
			git(t, r.Root, "init", "-b", "main")
			git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-m", "base")
			if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
				t.Fatal(x.res)
			}
			id := createTicket(t, r.Root, "ticket")
			if mode != "acceptance" {
				completeReleaseTicket(t, r.Root, "done", id)
			}
			pred := ""
			if mode == "predecessor" {
				pred = "missing"
			}
			x := atm(t, r.Root, nil, "release", "create", "--request-id", "release", "--target", "v1", "--payload", releaseCreatePayload("1", "Release", id, pred))
			if mode == "predecessor" {
				if x.res.Outcome == wire.OutcomeOK {
					t.Fatal("missing predecessor accepted")
				}
				return
			}
			if x.res.Outcome != wire.OutcomeOK {
				t.Fatal(x.res)
			}
			if x := atm(t, r.Root, nil, "release", "candidate", "--request-id", "candidate", "--target", "v1", "--expected-revision", "1"); x.res.Outcome != wire.OutcomeOK {
				t.Fatal(x.res)
			}
			rev, digest, _ := publicReleaseCandidate(t, r.Root, "v1")
			if mode == "failed-gate" {
				payload := fmt.Sprintf(`{"attestation":{"actor":"ci","attestationId":"failed","candidateSha256":%q,"criteria":["0"],"evidence":[%q],"gateId":"verify","profile":"taskman-release-attestation/0","provenance":"EXTERNAL_ATTESTATION","recordedAt":"2026-09-20T12:04:00Z","result":"FAIL","sourceIdentity":"ci"}}`, digest, wire.Sum([]byte("failed")))
				if x := atm(t, r.Root, nil, "release", "record-gate", "--request-id", "failed", "--target", "v1", "--expected-revision", rev, "--payload", payload); x.res.Outcome != wire.OutcomeOK {
					t.Fatal(x.res)
				}
				rev, _, _ = publicReleaseCandidate(t, r.Root, "v1")
			}
			if mode == "acceptance" {
				if x := atm(t, r.Root, nil, "ticket", "refine", "--request-id", "refine", "--target", id, "--expected-revision", "1", "--payload", `{"acceptanceCriteria":["changed"]}`); x.res.Outcome != wire.OutcomeOK {
					t.Fatal(x.res)
				}
			}
			ready := atm(t, r.Root, nil, "release", "readiness", "v1")
			if ready.res.Outcome != wire.OutcomeOK || field(ready.res.Items[0], "readiness").Str != "BLOCKED" {
				t.Fatalf("readiness: %+v", ready.res)
			}
			want := "gate:verify"
			if mode == "acceptance" {
				want = "ticket:" + id
			}
			found := false
			for _, m := range field(ready.res.Items[0], "missing").Arr {
				found = found || m.Str == want
			}
			if !found {
				t.Fatalf("missing %s: %+v", want, ready.res)
			}
			if x := atm(t, r.Root, nil, "release", "promote", "--request-id", "promote", "--target", "v1", "--expected-revision", rev); x.res.Outcome == wire.OutcomeOK {
				t.Fatal("blocked release promoted")
			}
		})
	}
}
