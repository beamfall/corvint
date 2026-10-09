package cli_test

import (
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/service"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// nextVersion is profile at the version after its own.
func nextVersion(t *testing.T, profile string) string {
	name, n, ok := strings.Cut(profile, "/")
	v, err := strconv.Atoi(n)
	if !ok || err != nil {
		t.Fatalf("profile %q has no version", profile)
	}
	return name + "/" + strconv.Itoa(v+1)
}

// newerRecord is the record a later build writes for profile: its profile
// names the next version and it carries a member this build does not know.
func newerRecord(t *testing.T, profile string) wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("profile", wire.String(nextVersion(t, profile))).Set("zzNewerMember", wire.Bool(true)))
}

// withMember is the canonical bytes of the valid record raw with member set
// to x, so a nested record from a later build sits inside a current one.
func withMember(t *testing.T, raw []byte, member string, x wire.Value) []byte {
	v, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	v.Obj.Set(member, x)
	return wire.EncodeFile(v)
}

// formatAttempt is a valid current attempt that also carries an operator note
// and retry accounting.
func formatAttempt(t *testing.T) []byte {
	id, _ := wire.ParseTicketID("ticket", "ticket:acme:main:AT-0001")
	d := wire.Sum(nil)
	budget := map[string]snapshot.BudgetField{}
	for _, name := range intent.LaneBudgetNames {
		budget[name] = snapshot.BudgetField{State: "NOT_OBSERVED"}
	}
	a := &snapshot.Attempt{AttemptID: "attempt:acme:main:57ddbdeca215924fd0ea543f91045dad", TicketID: id, TicketRevision: "1", TicketRecordSha256: d, Generation: "1", Phase: "RUNNING", PhaseSinceSeq: "1", Mode: "DEVELOPMENT", PolicySha256: d, ConfigSha256: d, RuntimeID: snapshot.RuntimeExternalAgent, CapabilityProfileSha256: d, BaseCommit: strings.Repeat("a", 40), Branch: "test", Quiescence: "UNPROVED", SpawnNoExecCount: "0", RetryCount: "0", RepairRound: "0", Budget: budget, ScopeCheck: "UNKNOWN", Lease: &snapshot.Lease{Holder: "agent", GrantedSeq: "1", ExpiresAt: "2026-10-01T00:00:00Z"}, Scope: &snapshot.Scope{Source: "REQUESTED"},
		// Independent and nested decoders that run after the nested formats
		// under test, so a refusal recorded first must survive them.
		OperatorNote: &ticket.OperatorNoteReference{Revision: "1", Current: &d, Head: d}, RetryAccounting: &snapshot.RetryAccounting{Disposition: "NONE"}}
	raw, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := snapshot.DecodeAttempt(raw); err != nil {
		t.Fatalf("current attempt fixture: %v", err)
	}
	return raw
}

func formatRelease(t *testing.T) []byte {
	raw := release.Encode(&release.Record{QueueID: wire.QueueID{Raw: "queue:acme:main"}, ReleaseID: "v1", Revision: "1", Version: "v1", Title: "Release"})
	if _, err := release.Decode(raw); err != nil {
		t.Fatalf("current release fixture: %v", err)
	}
	return raw
}

// TestCALV0131_EveryLiveFormatRefusesANewerVersion is the CAL-V0-131 table:
// for every format in LiveFormats, a record written by a later build (its
// profile at the next version, carrying a member this build does not know)
// is refused as UNSUPPORTED_VERSION by the top-level decoder that reads it,
// never as MALFORMED and never overwritten by a later check. A nested format
// is placed inside a valid current record. The dispatcher event log is the
// documented exception: its tail reader skips the newer line. A format added
// to LiveFormats without a row here fails the test.
func TestCALV0131_EveryLiveFormatRefusesANewerVersion(t *testing.T) {
	enc := func(t *testing.T, p string) []byte { return wire.EncodeFile(newerRecord(t, p)) }
	plain := func(decode func([]byte) error) func(*testing.T, string) error {
		return func(t *testing.T, p string) error { return decode(enc(t, p)) }
	}
	val := func(decode func(wire.Value) error) func(*testing.T, string) error {
		return func(t *testing.T, p string) error { return decode(newerRecord(t, p)) }
	}
	inAttempt := func(member string) func(*testing.T, string) error {
		return func(t *testing.T, p string) error {
			_, err := snapshot.DecodeAttempt(withMember(t, formatAttempt(t), member, newerRecord(t, p)))
			return err
		}
	}
	rows := map[string]func(*testing.T, string) error{
		strings.TrimSpace(snapshot.VersionBytes): func(t *testing.T, p string) error {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(nextVersion(t, p)+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := snapshot.Probe(dir)
			return err
		},
		wire.ProfileCommandResult:          plain(func(b []byte) error { _, e := wire.DecodeResult(b); return e }),
		wire.CriterionCaptureProfile:       plain(func(b []byte) error { _, e := wire.DecodeCriterionCapture(b); return e }),
		wire.CriterionVerificationProfile:  val(func(v wire.Value) error { _, e := wire.ReadCriterionVerification(v); return e }),
		wire.CriterionCaptureResultProfile: val(func(v wire.Value) error { _, _, e := wire.ReadCriterionCaptureResult(v); return e }),
		archive.Profile:                    plain(func(b []byte) error { _, e := archive.DecodeManifest(b); return e }),
		journal.ProfileCheckpoint:          plain(func(b []byte) error { _, e := journal.DecodeCheckpoint(b); return e }),
		journal.ProfileWriterCheckpoint: func(t *testing.T, p string) error {
			body := []byte(nextVersion(t, p) + "\nzzNewerLayout")
			sum := sha256.Sum256(body)
			_, err := journal.DecodeWriterCheckpoint(append(body, sum[:]...))
			return err
		},
		mutation.Profile:                    plain(func(b []byte) error { _, e := mutation.Decode(b); return e }),
		mutation.OutcomeProfile:             plain(func(b []byte) error { _, e := mutation.DecodeOutcome(b); return e }),
		intent.ProfileImportMap:             plain(func(b []byte) error { _, e := intent.DecodeImportMap(b); return e }),
		intent.ProfileQueue:                 plain(func(b []byte) error { _, e := intent.DecodeQueue(b); return e }),
		intent.ProfilePolicy:                plain(func(b []byte) error { _, e := intent.DecodePolicy(b); return e }),
		snapshot.ProfileHead:                plain(func(b []byte) error { _, e := snapshot.DecodeHead(b); return e }),
		snapshot.ProfileBarrier:             plain(func(b []byte) error { _, e := snapshot.DecodeBarrier(b); return e }),
		snapshot.ProfileReceipt:             plain(func(b []byte) error { _, e := snapshot.DecodeReceipt(b); return e }),
		snapshot.ProfileInit:                plain(func(b []byte) error { _, e := snapshot.DecodeInit(b); return e }),
		snapshot.ProfileAttempt:             plain(func(b []byte) error { _, e := snapshot.DecodeAttempt(b); return e }),
		snapshot.ProfileReservations:        plain(func(b []byte) error { _, e := snapshot.DecodeReservations(b); return e }),
		snapshot.ProfileRetryAccounting:     inAttempt("retryAccounting"),
		snapshot.ProfileDirectPoolAdmission: inAttempt("directPoolAdmission"),
		snapshot.ProfileLaneUntouched:       inAttempt("laneUntouchedAttestation"),
		snapshot.ProfileGateResult:          plain(func(b []byte) error { _, e := snapshot.DecodeGateResult(b); return e }),
		snapshot.ProfileManifest:            plain(func(b []byte) error { _, e := snapshot.DecodeManifest(b); return e }),
		snapshot.SupervisedProfile: func(t *testing.T, p string) error {
			programs := wire.ObjectValue(wire.NewObject().Set("profile", wire.String("taskman-programs/0")).Set("entries", wire.Array(newerRecord(t, p))))
			_, err := snapshot.DecodePrograms(wire.EncodeFile(programs))
			return err
		},
		snapshot.ProfileExternalReviewRequest: plain(func(b []byte) error { _, e := snapshot.DecodeExternalReviewRequest(b); return e }),
		snapshot.ProfileExternalReviewEvent:   plain(func(b []byte) error { _, e := snapshot.DecodeExternalReviewEvent(b); return e }),
		snapshot.ProfilePools:                 plain(func(b []byte) error { _, e := snapshot.DecodePools(b); return e }),
		"taskman-pool-observation/0":          plain(func(b []byte) error { _, e := snapshot.DecodePoolObservation(b); return e }),
		"taskman-pool-sweep-observation/0":    plain(func(b []byte) error { _, e := snapshot.DecodePoolSweepObservation(b); return e }),
		"taskman-pool-sweep-result/0":         plain(func(b []byte) error { _, _, e := transaction.DecodePoolSweepResult(b); return e }),
		"taskman-programs/0":                  plain(func(b []byte) error { _, e := snapshot.DecodePrograms(b); return e }),
		"taskman-stage/0":                     plain(func(b []byte) error { _, e := snapshot.DecodeStageDescriptor(b); return e }),
		"taskman-operator-note-cursor/0": plain(func(b []byte) error {
			_, e := store.DecodeOperatorNoteCursor(base64.RawURLEncoding.EncodeToString(b))
			return e
		}),
		ticket.Profile:                  plain(func(b []byte) error { _, e := ticket.Decode(b); return e }),
		ticket.OperatorNoteProfile:      plain(func(b []byte) error { _, e := ticket.DecodeOperatorNoteEvent(b); return e }),
		ticket.EscalationRequestProfile: plain(func(b []byte) error { _, e := ticket.DecodeEscalationRequest(b); return e }),
		ticket.EscalationEventProfile:   plain(func(b []byte) error { _, e := ticket.DecodeEscalationEvent(b); return e }),
		ticket.ObligationEventProfile:   plain(func(b []byte) error { _, e := ticket.DecodeObligationEvent(b); return e }),
		ticket.ObligationPlanProfile:    plain(func(b []byte) error { _, e := ticket.DecodeObligationPlan(b); return e }),
		release.Profile:                 plain(func(b []byte) error { _, e := release.Decode(b); return e }),
		release.AttestationProfile: func(t *testing.T, p string) error {
			_, err := release.Decode(withMember(t, formatRelease(t), "attestations", wire.Array(newerRecord(t, p))))
			return err
		},
		release.MutationProfile:        plain(func(b []byte) error { _, e := release.DecodeEnvelope(b); return e }),
		transaction.RunOutcomeProfile:  plain(func(b []byte) error { _, e := transaction.DecodeRunOutcome(b); return e }),
		"taskman-attempt-run-record/0": plain(cli.DecodeRunRecord),
		dispatch.ConfigProfile:         plain(func(b []byte) error { _, e := dispatch.DecodeConfig(b); return e }),
		dispatch.StateProfile: func(t *testing.T, p string) error {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "state.json"), enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.LoadLedger(dir, "prog")
			return err
		},
		"taskman-dispatch-reader-lifecycle/0": func(t *testing.T, p string) error {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "reader-lifecycle.json"), enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, held, why := dispatch.ReaderContainment(dir, "prog")
			if !held {
				t.Fatal("newer reader marker not held")
			}
			code, _, _ := strings.Cut(why, ":")
			return wire.Errorf(code, "", "%s", why)
		},
		"taskman-dispatch-detached-run/0": func(t *testing.T, p string) error {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "detached"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "detached", "0123456789abcdef.json"), enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, bad, err := dispatch.DetachedMarkers(dir)
			if err != nil {
				return err
			}
			return bad["0123456789abcdef.json"]
		},
		service.ProfileName:         plain(func(b []byte) error { _, e := service.DecodeProfile(b); return e }),
		service.ManifestName:        plain(func(b []byte) error { _, e := service.DecodeManifest(b); return e }),
		service.ControlName:         plain(func(b []byte) error { _, e := service.DecodeControl(b); return e }),
		service.PulseName:           plain(func(b []byte) error { _, e := service.DecodePulse(b); return e }),
		service.OperationName:       plain(func(b []byte) error { _, e := service.DecodeOperation(b); return e }),
		service.RequestsName:        plain(func(b []byte) error { _, e := service.DecodeRequests(b); return e }),
		service.HelperRecordName:    plain(func(b []byte) error { _, e := service.DecodeHelperRecord("helper", b, "prog", "h1"); return e }),
		service.ResumeOperationName: plain(func(b []byte) error { _, e := service.DecodeResumeOperation(b, "prog"); return e }),
	}
	live := cli.LiveFormats()
	for _, p := range live {
		if p == dispatch.EventProfile {
			// Documented exception: the tail reader skips a line it cannot read.
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := dispatch.ReadEvents(dir, 10); err != nil || len(got) != 0 {
				t.Errorf("%s: newer event line read as %v, %v", p, got, err)
			}
			continue
		}
		row, ok := rows[p]
		if !ok {
			t.Errorf("%s: no row decodes this live format", p)
			continue
		}
		if code := wire.CodeOf(row(t, p)); code != wire.CodeUnsupportedVersion {
			t.Errorf("%s: newer record refused as %q, want %s", p, code, wire.CodeUnsupportedVersion)
		}
	}
	for p := range rows {
		if !slices.Contains(live, p) {
			t.Errorf("%s: row for a format LiveFormats does not report", p)
		}
	}
}
