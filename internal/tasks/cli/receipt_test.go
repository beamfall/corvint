package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func receiptFixture(t *testing.T) *fixture.Repo {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	if x := atm(t, r.Root, nil, "ticket", "create", "--request-id", "audit-create", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %+v", x.res)
	}
	// Reads must not create even the lock left behind by a completed writer.
	if err := os.Remove(filepath.Join(r.CommonDir, "taskman.lock")); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestTMV0008_AS07_ReceiptAuditReportsBoundedEvidence(t *testing.T) {
	r := receiptFixture(t)
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	x := atm(t, r.Root, nil, "receipt", "audit")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("audit: %+v", x.res)
	}
	item := x.res.Items[0]
	raw, err := os.ReadFile(filepath.Join(r.StateDir, "receipts", "000000000002.json"))
	if err != nil {
		t.Fatal(err)
	}
	if field(item, "headSeq").Str != "2" || field(item, "lastReceiptSha256").Str != string(wire.Sum(raw)) {
		t.Fatalf("unbound receipt: %+v", item)
	}
	for key, want := range map[string]string{"structuralConsistency": "CONSISTENT", "projectionAgreement": "AGREES", "semanticCoverage": "KNOWN_CODECS", "historicalAcceptance": "NOT_OBSERVED", "actorAuthentication": "NOT_OBSERVED", "liveness": "NOT_OBSERVED", "runtimeQualification": "NOT_OBSERVED"} {
		if field(item, key).Str != want {
			t.Errorf("%s = %q, want %s", key, field(item, key).Str, want)
		}
	}
	if x.res.Snapshot == nil || x.res.Mutation != nil || x.res.Page != nil || x.res.Untrusted || field(item, "stagingPresent").Bool {
		t.Fatalf("unexpected claims: %+v", x.res)
	}
	fixture.AssertUntouched(t, r, state, intents, "receipt audit")
	if _, err := os.Lstat(filepath.Join(r.CommonDir, "taskman.lock")); !os.IsNotExist(err) {
		t.Fatalf("audit created lock: %v", err)
	}
}

func TestTMV0008_AS11_ReceiptAuditRefusesWithoutRecovery(t *testing.T) {
	for _, kind := range []string{"pending", "history", "diverged", "restore", "primary"} {
		t.Run(kind, func(t *testing.T) {
			r := receiptFixture(t)
			code := wire.CodeJournalForked
			switch kind {
			case "pending":
				code = wire.CodeRedoPending
				first, err := os.ReadFile(filepath.Join(r.StateDir, "receipts", "000000000001.json"))
				if err != nil {
					t.Fatal(err)
				}
				headRaw, err := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
				if err != nil {
					t.Fatal(err)
				}
				head, err := snapshot.DecodeHead(headRaw)
				if err != nil {
					t.Fatal(err)
				}
				receipt, err := snapshot.DecodeReceipt(first)
				if err != nil {
					t.Fatal(err)
				}
				digest := wire.Sum(first)
				head.LastSeq = "1"
				head.LastReceiptSha256 = &digest
				head.Generation = receipt.HeadGeneration
				fixture.Write(t, filepath.Join(r.StateDir, "head.json"), wire.EncodeFile(head.Value()))
				entries, err := os.ReadDir(filepath.Join(r.IntentDir, "tickets"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(r.IntentDir, "tickets", entries[0].Name())); err != nil {
					t.Fatal(err)
				}
			case "history":
				path := filepath.Join(r.StateDir, "receipts", "000000000001.json")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				v, err := wire.Parse(raw)
				if err != nil {
					t.Fatal(err)
				}
				v.Obj.Set("recordedAt", wire.String("2020-01-01T00:00:00Z"))
				fixture.Write(t, path, wire.EncodeFile(v))
			case "diverged":
				code = wire.CodeIntentDiverged
				path := filepath.Join(r.IntentDir, "queue.json")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				v, err := wire.Parse(raw)
				if err != nil {
					t.Fatal(err)
				}
				v.Obj.Set("intentBranch", wire.String("other"))
				fixture.Write(t, path, wire.EncodeFile(v))
			case "restore":
				code = wire.CodeRestoreIncomplete
				fixture.Write(t, filepath.Join(r.StateDir, "RESTORE_INCOMPLETE"), []byte("incomplete\n"))
			case "primary":
				code = wire.CodeUnsupportedFilesystem
				path := filepath.Join(r.StateDir, "head.json")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				v, err := wire.Parse(raw)
				if err != nil {
					t.Fatal(err)
				}
				v.Obj.Set("primaryWorktree", wire.String("/private/tmp/other"))
				fixture.Write(t, path, wire.EncodeFile(v))
			}
			state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
			x := atm(t, r.Root, nil, "receipt", "audit")
			if !hasCode(x.res, code) || len(x.res.Items) != 0 || x.res.Outcome == wire.OutcomeOK {
				t.Fatalf("%s: %+v", kind, x.res)
			}
			fixture.AssertUntouched(t, r, state, intents, "receipt audit refusal")
			if _, err := os.Lstat(filepath.Join(r.CommonDir, "taskman.lock")); !os.IsNotExist(err) {
				t.Fatalf("audit created lock: %v", err)
			}
		})
	}
}

func TestTMV0008_AS07_ReceiptAuditUsageAndUninitialized(t *testing.T) {
	r := fixture.TempRepo(t)
	for _, args := range []string{"receipt", "receipt unknown", "receipt audit extra", "receipt audit --limit 1"} {
		x := atm(t, r.Root, nil, strings.Fields(args)...)
		if x.res.Outcome != wire.OutcomeError || x.res.Snapshot != nil {
			t.Fatalf("usage: %+v", x.res)
		}
	}
	x := atm(t, r.Root, nil, "receipt", "audit")
	if !hasCode(x.res, wire.CodeUninitialized) || x.res.Snapshot != nil {
		t.Fatalf("uninitialized: %+v", x.res)
	}
	if _, err := os.Lstat(r.StateDir); !os.IsNotExist(err) {
		t.Fatalf("read initialized store: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(r.CommonDir, "taskman.lock")); !os.IsNotExist(err) {
		t.Fatalf("read created lock: %v", err)
	}
}
