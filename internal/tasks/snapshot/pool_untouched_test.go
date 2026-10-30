package snapshot

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"testing"
)

func untouchedAttempt() *Attempt {
	a := accountingAttempt()
	a.PoolAllocation = &PoolAllocation{PoolID: "db", MemberID: "a", AllocationID: wire.Sum([]byte("a")), DefinitionSha256: wire.Sum(nil), AllocatedSeq: "1"}
	a.DirectPoolAdmission = &DirectPoolAdmission{AttemptID: a.AttemptID, Generation: a.Generation, OriginalAdmissionSeq: "1", Allocation: a.PoolAllocation, Holder: a.Lease.Holder, Stage: a.Stage}
	a.RetryAccounting = &RetryAccounting{Disposition: "NONE"}
	a.Phase, a.Quiescence, a.PhaseSinceSeq = "CANCELLED", "FENCED", "2"
	a.LaneUntouchedAttestation = &LaneUntouchedAttestation{DirectPoolAdmission: *a.DirectPoolAdmission, ActorID: "owner", ActorRole: "OWNER", Evidence: "local:unused", RecordedSeq: "2", RecordedAt: "2026-09-01T00:00:00Z"}
	return a
}
func TestPoolLaneUntouched_Codec(t *testing.T) {
	t.Run("CAL-V0-067", func(t *testing.T) {
		legacy := accountingAttempt()
		raw, err := legacy.Encode()
		if err != nil {
			t.Fatal(err)
		}
		old, err := DecodeAttempt(raw)
		if err != nil {
			t.Fatal(err)
		}
		again, _ := old.Encode()
		if !bytes.Equal(raw, again) || bytes.Contains(raw, []byte("directPoolAdmission")) || bytes.Contains(raw, []byte("laneUntouchedAttestation")) {
			t.Fatal("legacy bytes changed")
		}
		a := untouchedAttempt()
		raw, err = a.Encode()
		if err != nil {
			t.Fatal(err)
		}
		b, err := DecodeAttempt(raw)
		if err != nil {
			t.Fatal(err)
		}
		again, err = b.Encode()
		if err != nil || !bytes.Equal(raw, again) {
			t.Fatal("roundtrip", err)
		}
		for _, pair := range [][2]string{{ProfileDirectPoolAdmission, "taskman-direct-pool-admission/99"}, {ProfileLaneUntouched, "taskman-lane-untouched-attestation/99"}, {`"noLaneAccess":true`, `"noLaneAccess":false`}, {`"physicalFacts":"NOT_OBSERVED"`, `"physicalFacts":"PROVED"`}, {`"actorRole":"OWNER"`, `"actorRole":"WORKER"`}, {`"evidence":"local:unused"`, `"evidence":""`}, {`"acknowledgements":{`, `"acknowledgements":{"extra":true,`}, {`"recordedSeq":"2"`, `"recordedSeq":"3"`}} {
			if _, err := DecodeAttempt(bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)); err == nil {
				t.Fatal("accepted invalid", pair)
			}
		}
		for name, change := range map[string]func(*Attempt){"live": func(a *Attempt) { a.Phase = "RUNNING" }, "proved": func(a *Attempt) { a.Quiescence = "PROVED" }, "origin": func(a *Attempt) { a.DirectPoolAdmission = nil }, "allocation": func(a *Attempt) { a.PoolAllocation = nil }, "sequence": func(a *Attempt) { a.Lease.GrantedSeq = "2" }, "binding": func(a *Attempt) { a.LaneUntouchedAttestation.Holder = "other" }, "failed": func(a *Attempt) { a.RetryAccounting.FailedOrUnknown = true }} {
			t.Run(name, func(t *testing.T) {
				a := untouchedAttempt()
				change(a)
				if _, err := a.Encode(); err == nil {
					t.Fatal("bad binding accepted")
				}
			})
		}
	})
}
