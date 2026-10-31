package cli_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/defects"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
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
	everyLiveFormatRefusesANewerVersion(t)
}

// TestCALV0131_SymlinkedTempRoot runs the CAL-V0-131 table with TMPDIR behind
// a symlink, as macOS's /var/folders is. The store readers refuse a symlinked
// ancestor as UNSUPPORTED_FILESYSTEM, so the table resolves its temp
// directories first; this case keeps that true on every host (V1-0960).
func TestCALV0131_SymlinkedTempRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TMPDIR does not select the Windows temp directory")
	}
	real, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(real, "target"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(real, "linked-tmp")
	if err := os.Symlink(filepath.Join(real, "target"), link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", link)
	// A subtest, because t.TempDir reuses the root its test created first.
	t.Run("table", everyLiveFormatRefusesANewerVersion)
}

// resolvedTempDir is t.TempDir with its symlinked ancestors resolved.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func everyLiveFormatRefusesANewerVersion(t *testing.T) {
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
			dir := resolvedTempDir(t)
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
			_, err := journal.DecodeWriterCheckpoint(append(body, sum[:]...), nil)
			return err
		},
		journal.ProfileWriterBase: func(t *testing.T, p string) error {
			// A current delta that binds the newer base, so the base's own
			// version check is the one that refuses.
			base := []byte(nextVersion(t, p) + "\nzzNewerLayout")
			bound := sha256.Sum256(base)
			body := append([]byte(journal.ProfileWriterCheckpoint+"\n"), bound[:]...)
			body = append(body, make([]byte, 8+7*8+5*8+8+8+sha256.Size)...)
			sum := sha256.Sum256(body)
			_, err := journal.DecodeWriterCheckpoint(append(body, sum[:]...), base)
			return err
		},
		journal.ProfileRetiredPrefix: func(t *testing.T, p string) error {
			body := []byte(nextVersion(t, p) + "\nzzNewerLayout")
			sum := sha256.Sum256(body)
			_, err := journal.DecodeRetiredPrefix(append(body, sum[:]...))
			return err
		},
		mutation.Profile:        plain(func(b []byte) error { _, e := mutation.Decode(b); return e }),
		mutation.OutcomeProfile: plain(func(b []byte) error { _, e := mutation.DecodeOutcome(b); return e }),
		intent.ProfileImportMap: plain(func(b []byte) error { _, e := intent.DecodeImportMap(b); return e }),
		intent.ProfileQueue:     plain(func(b []byte) error { _, e := intent.DecodeQueue(b); return e }),
		intent.ProfilePolicy:    plain(func(b []byte) error { _, e := intent.DecodePolicy(b); return e }),
		// A head newer than /0 is /1, which this build reads; the record a
		// later build writes after /1 is the one this build must refuse.
		snapshot.ProfileHead: func(t *testing.T, p string) error {
			_, e := snapshot.DecodeHead(enc(t, snapshot.ProfileHeadSegmented))
			return e
		},
		snapshot.ProfileHeadSegmented:   plain(func(b []byte) error { _, e := snapshot.DecodeHead(b); return e }),
		snapshot.ProfileSegmentManifest: plain(func(b []byte) error { _, e := snapshot.DecodeSegmentManifest(b, 1); return e }),
		// The segment and the seal marker are not JSON: a later build's
		// file starts with its next profile line.
		snapshot.ProfileSegment: func(t *testing.T, p string) error {
			_, e := snapshot.DecodeSegment([]byte(nextVersion(t, p)+"\n"+strings.Repeat("\x00", 24)), 1, snapshot.SegmentReceipts)
			return e
		},
		snapshot.ProfileSealMarker: func(t *testing.T, p string) error {
			_, e := snapshot.DecodeSealMarker([]byte(nextVersion(t, p) + "\n000000000001-000000000512\n"))
			return e
		},
		snapshot.ProfileBarrier:             plain(func(b []byte) error { _, e := snapshot.DecodeBarrier(b); return e }),
		snapshot.ProfileReceipt:             plain(func(b []byte) error { _, e := snapshot.DecodeReceipt(b); return e }),
		snapshot.ProfileElidedReceipt:       plain(func(b []byte) error { _, _, _, e := snapshot.DecodeRetainedReceipt(b); return e }),
		snapshot.ProfileInit:                plain(func(b []byte) error { _, e := snapshot.DecodeInit(b); return e }),
		snapshot.ProfileRelocate:            plain(func(b []byte) error { _, e := snapshot.DecodeRelocate(b); return e }),
		snapshot.ProfileAttempt:             plain(func(b []byte) error { _, e := snapshot.DecodeAttempt(b); return e }),
		snapshot.ProfileReservations:        plain(func(b []byte) error { _, e := snapshot.DecodeReservations(b); return e }),
		snapshot.ProfileRetryAccounting:     inAttempt("retryAccounting"),
		snapshot.ProfileDirectPoolAdmission: inAttempt("directPoolAdmission"),
		snapshot.ProfileLaneUntouched:       inAttempt("laneUntouchedAttestation"),
		snapshot.ProfileGateResult:          plain(func(b []byte) error { _, e := snapshot.DecodeGateResult(b); return e }),
		snapshot.ProfileCIRun:               plain(func(b []byte) error { _, e := snapshot.DecodeCIRun(b); return e }),
		snapshot.ProfileManifest:            plain(func(b []byte) error { _, e := snapshot.DecodeManifest(b); return e }),
		snapshot.SupervisedProfile: func(t *testing.T, p string) error {
			programs := wire.ObjectValue(wire.NewObject().Set("profile", wire.String("taskman-programs/0")).Set("entries", wire.Array(newerRecord(t, p))))
			_, err := snapshot.DecodePrograms(wire.EncodeFile(programs))
			return err
		},
		snapshot.ProfileExternalReviewRequest: plain(func(b []byte) error { _, e := snapshot.DecodeExternalReviewRequest(b); return e }),
		snapshot.ProfileExternalReviewEvent:   plain(func(b []byte) error { _, e := snapshot.DecodeExternalReviewEvent(b); return e }),
		snapshot.ProfilePools:                 plain(func(b []byte) error { _, e := snapshot.DecodePools(b); return e }),
		snapshot.ProfilePoolIdentity:          plain(func(b []byte) error { _, e := snapshot.DecodePoolIdentity(b); return e }),
		snapshot.ProfilePoolOperation: func(t *testing.T, p string) error {
			pools := wire.ObjectValue(wire.NewObject().Set("profile", wire.String(snapshot.ProfilePools)).Set("queueId", wire.String("queue:acme:main")).Set("entries", wire.Array()).Set("operations", wire.Array(newerRecord(t, p))))
			_, err := snapshot.DecodePools(wire.EncodeFile(pools))
			return err
		},
		"taskman-pool-observation/0":       plain(func(b []byte) error { _, e := snapshot.DecodePoolObservation(b); return e }),
		"taskman-pool-sweep-observation/0": plain(func(b []byte) error { _, e := snapshot.DecodePoolSweepObservation(b); return e }),
		"taskman-pool-sweep-result/0":      plain(func(b []byte) error { _, _, e := transaction.DecodePoolSweepResult(b); return e }),
		"taskman-programs/0":               plain(func(b []byte) error { _, e := snapshot.DecodePrograms(b); return e }),
		"taskman-stage/0":                  plain(func(b []byte) error { _, e := snapshot.DecodeStageDescriptor(b); return e }),
		"taskman-operator-note-cursor/0": plain(func(b []byte) error {
			_, e := store.DecodeOperatorNoteCursor(base64.RawURLEncoding.EncodeToString(b))
			return e
		}),
		ticket.Profile:                   plain(func(b []byte) error { _, e := ticket.Decode(b); return e }),
		ticket.OperatorNoteProfile:       plain(func(b []byte) error { _, e := ticket.DecodeOperatorNoteEvent(b); return e }),
		ticket.EscalationRequestProfile:  plain(func(b []byte) error { _, e := ticket.DecodeEscalationRequest(b); return e }),
		ticket.EscalationEventProfile:    plain(func(b []byte) error { _, e := ticket.DecodeEscalationEvent(b); return e }),
		ticket.ObligationEventProfile:    plain(func(b []byte) error { _, e := ticket.DecodeObligationEvent(b); return e }),
		ticket.ObligationPlanProfile:     plain(func(b []byte) error { _, e := ticket.DecodeObligationPlan(b); return e }),
		obligation.ProfileQualifications: plain(func(b []byte) error { _, e := obligation.DecodeAllowList(b); return e }),
		release.Profile:                  plain(func(b []byte) error { _, e := release.Decode(b); return e }),
		release.AttestationProfile: func(t *testing.T, p string) error {
			_, err := release.Decode(withMember(t, formatRelease(t), "attestations", wire.Array(newerRecord(t, p))))
			return err
		},
		release.MutationProfile:               plain(func(b []byte) error { _, e := release.DecodeEnvelope(b); return e }),
		transaction.RunOutcomeProfile:         plain(func(b []byte) error { _, e := transaction.DecodeRunOutcome(b); return e }),
		transaction.ProfilePoolRecoveryDetail: plain(func(b []byte) error { _, e := transaction.DecodePoolRecoveryDetail(b); return e }),
		"taskman-attempt-run-record/0":        plain(cli.DecodeRunRecord),
		"taskman-doctor-cache/0":              plain(cli.DecodeDoctorCache),
		"taskman-doctor-config/0":             plain(cli.DecodeDoctorConfig),
		dispatch.ConfigProfile:                plain(func(b []byte) error { _, e := dispatch.DecodeConfig(b); return e }),
		dispatch.AdmissionProfile:             plain(func(b []byte) error { _, e := dispatch.DecodeAdmission(b, "prog"); return e }),
		dispatch.StateProfile: func(t *testing.T, p string) error {
			dir := resolvedTempDir(t)
			if err := os.WriteFile(filepath.Join(dir, "state.json"), enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.LoadLedger(dir, "prog")
			return err
		},
		"taskman-dispatch-reader-lifecycle/1": func(t *testing.T, p string) error {
			dir := resolvedTempDir(t)
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
		dispatch.ReaderRecoveryProfile: func(t *testing.T, p string) error {
			dir := resolvedTempDir(t)
			if err := os.MkdirAll(filepath.Join(dir, "reader-recoveries"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "reader-recoveries", "reboot-0123456789abcdef0123456789abcdef.json"), enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.ReadReaderRecovery(dir, "reboot-0123456789abcdef0123456789abcdef.json")
			return err
		},
		dispatch.ReaderLeaderProfile: func(t *testing.T, p string) error {
			dir := resolvedTempDir(t)
			if err := os.WriteFile(filepath.Join(dir, "reader-leader.json"), enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.ReadReaderLeader(dir, "prog")
			return err
		},
		"taskman-dispatch-detached-run/1": func(t *testing.T, p string) error {
			dir := resolvedTempDir(t)
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
		dispatch.SessionChurnProfile: func(t *testing.T, p string) error {
			path := filepath.Join(resolvedTempDir(t), "session-churn.json")
			if err := os.WriteFile(path, enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.ReadSessionChurn(path)
			return err
		},
		dispatch.HandoffNoteProfile: func(t *testing.T, p string) error {
			path := filepath.Join(resolvedTempDir(t), "handoff-note.json")
			if err := os.WriteFile(path, enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.ReadHandoffNote(path)
			return err
		},
		dispatch.PoolHoldProfile: func(t *testing.T, p string) error {
			path := filepath.Join(resolvedTempDir(t), "pool-hold.json")
			if err := os.WriteFile(path, enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.ReadPoolHoldFile(path)
			return err
		},
		dispatch.CarryOverProfile: func(t *testing.T, p string) error {
			path := filepath.Join(resolvedTempDir(t), "carry-over.json")
			if err := os.WriteFile(path, enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.ReadCarryOver(path)
			return err
		},
		dispatch.ContextExhaustedProfile: func(t *testing.T, p string) error {
			path := filepath.Join(resolvedTempDir(t), "context-exhausted.json")
			if err := os.WriteFile(path, enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.ReadContextExhausted(path)
			return err
		},
		dispatch.HostSleepProfile: func(t *testing.T, p string) error {
			path := filepath.Join(resolvedTempDir(t), "host-sleep.json")
			if err := os.WriteFile(path, enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.ReadHostSleep(path)
			return err
		},
		dispatch.DrainSignalProfile: func(t *testing.T, p string) error {
			path := filepath.Join(resolvedTempDir(t), "drain-pending.json")
			if err := os.WriteFile(path, enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.ReadDrainSignal(path)
			return err
		},
		dispatch.WorkerBuildProfile: func(t *testing.T, p string) error {
			path := filepath.Join(resolvedTempDir(t), "build.json")
			if err := os.WriteFile(path, enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.ReadWorkerBuildFile(path, "w1")
			return err
		},
		dispatch.UpgradeRequestProfile: func(t *testing.T, p string) error {
			path := filepath.Join(resolvedTempDir(t), "upgrade-request.json")
			if err := os.WriteFile(path, enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.ReadUpgradeRequestFile(path)
			return err
		},
		dispatch.UpgradeStateProfile: func(t *testing.T, p string) error {
			dir := resolvedTempDir(t)
			if err := os.WriteFile(filepath.Join(dir, "upgrade.json"), enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.ReadUpgradeState(dir)
			return err
		},
		dispatch.ContextProfile: func(t *testing.T, p string) error {
			dir := resolvedTempDir(t)
			if err := os.MkdirAll(filepath.Dir(dispatch.ContextRecordPath(dir, "w1")), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dispatch.ContextRecordPath(dir, "w1"), enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := dispatch.ReadContextRecord(dir, "w1")
			return err
		},
		defects.Profile: func(t *testing.T, p string) error {
			file := filepath.Join(resolvedTempDir(t), defects.RegisterFile)
			if err := os.WriteFile(file, enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := defects.Read(file, true)
			return err
		},
		cli.LeaseDetachProfile: func(t *testing.T, p string) error {
			file := filepath.Join(resolvedTempDir(t), "request.json")
			if err := os.WriteFile(file, enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			return cli.ReadLeaseDetachRequest(file)
		},
		service.ProfileName:         plain(func(b []byte) error { _, e := service.DecodeProfile(b); return e }),
		service.ManifestName:        plain(func(b []byte) error { _, e := service.DecodeManifest(b); return e }),
		service.ControlName:         plain(func(b []byte) error { _, e := service.DecodeControl(b); return e }),
		service.PulseName:           plain(func(b []byte) error { _, e := service.DecodePulse(b); return e }),
		service.OperationName:       plain(func(b []byte) error { _, e := service.DecodeOperation(b); return e }),
		service.RequestsName:        plain(func(b []byte) error { _, e := service.DecodeRequests(b); return e }),
		service.HelperRecordName:    plain(func(b []byte) error { _, e := service.DecodeHelperRecord("helper", b, "prog", "h1"); return e }),
		service.ResumeOperationName: plain(func(b []byte) error { _, e := service.DecodeResumeOperation(b, "prog"); return e }),
		"taskman-intent-snapshot/0": func(t *testing.T, p string) error {
			path := filepath.Join(resolvedTempDir(t), "queue-snapshot.json")
			if err := os.WriteFile(path, enc(t, p), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv(cli.SnapshotEnv, path)
			var out bytes.Buffer
			cli.Run(cli.Env{Cwd: filepath.Dir(path), Args: []string{"queue", "status"}, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: io.Discard})
			res, err := wire.DecodeResult(out.Bytes())
			if err != nil || len(res.Codes) == 0 {
				t.Fatalf("snapshot read: %v %s", err, out.String())
			}
			return wire.Errorf(res.Codes[0], "snapshot", "%v", res.Warnings)
		},
	}
	live := cli.LiveFormats()
	for _, p := range live {
		if p == dispatch.EventProfile {
			// Documented exception: the tail reader skips a line it cannot read.
			dir := resolvedTempDir(t)
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
