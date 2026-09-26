package cli_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type reconciliationInput struct{ reads int }

func (r *reconciliationInput) Read([]byte) (int, error) {
	r.reads++
	return 0, io.EOF
}

func TestTMV0007_AS35_InapplicableReconcileFlagRefusesBeforeInput(t *testing.T) {
	for _, digest := range []string{"", string(wire.Sum([]byte("inapplicable")))} {
		var output bytes.Buffer
		input := &reconciliationInput{}
		code := cli.Run(cli.Env{Cwd: t.TempDir(), Args: []string{"reconcile", "intent", "--target", "ticket:acme:main:AT-0002", "--request-id", "bad", "--adopt-file", "--canonical-sha256", digest, "--file", "-"}, Stdin: input, Stdout: &output})
		result, err := wire.DecodeResult(output.Bytes())
		if err != nil || code != 1 || !hasCode(result, wire.CodeMalformed) || input.reads != 0 {
			t.Fatalf("inapplicable flag consumed input: reads=%d result=%+v err=%v", input.reads, result, err)
		}
	}
}

func TestTMV0007_AS35_CLIInspectReconcileRetryWorkflow(t *testing.T) {
	for _, choice := range []string{"keep-malformed", "keep-empty", "adopt"} {
		t.Run(choice, func(t *testing.T) {
			r := receiptFixture(t)
			path := filepath.Join(r.IntentDir, "tickets", "AT-0002.json")
			canonical, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			discard := []byte("{malformed edit")
			if choice == "keep-empty" {
				discard = []byte{}
			}
			if choice == "adopt" {
				v, err := wire.Parse(canonical)
				if err != nil {
					t.Fatal(err)
				}
				v.Obj.Set("title", wire.String("adopted from CLI"))
				discard = wire.EncodeFile(v)
			}
			fixture.Write(t, path, discard)
			state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
			x := atm(t, r.Root, nil, "reconcile", "inspect", "ticket:acme:main:AT-0002")
			if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 || !x.res.Untrusted || x.res.Snapshot == nil {
				t.Fatalf("inspect: %+v", x.res)
			}
			item := x.res.Items[0]
			digest := field(item, "canonicalRecordSha256").Str
			if digest != string(wire.Sum(canonical)) || !bytes.Equal(wire.EncodeFile(field(item, "canonicalRecord")), canonical) || field(item, "projectionAgreement").Str != "ALL_EXCEPT_TARGET_AGREE" {
				t.Fatalf("canonical mismatch: %+v", item)
			}
			for _, key := range []string{"historicalAcceptance", "actorAuthentication", "liveness", "runtimeQualification"} {
				if field(item, key).Str != "NOT_OBSERVED" {
					t.Fatalf("invented %s", key)
				}
			}
			fixture.AssertUntouched(t, r, state, intents, "reconcile inspect")
			args := []string{"reconcile", "intent", "--target", "ticket:acme:main:AT-0002", "--file", "-", "--request-id", "repair"}
			if choice == "adopt" {
				args = append(args, "--adopt-file")
			} else {
				args = append(args, "--keep-journal", "--canonical-sha256", digest)
			}
			if choice == "keep-malformed" {
				fixture.Write(t, filepath.Join(r.Root, "preserved-edit"), discard)
				args[5] = "preserved-edit"
			}
			first := atm(t, r.Root, discard, args...)
			if first.res.Outcome != wire.OutcomeOK || field(first.res.Items[0], "ticketId").Str != "ticket:acme:main:AT-0002" || field(first.res.Items[0], "receipt").Str == "" {
				t.Fatalf("reconcile: %+v", first.res)
			}
			audit := atm(t, r.Root, nil, "receipt", "audit")
			if audit.res.Outcome != wire.OutcomeOK {
				t.Fatalf("audit after reconcile: %+v", audit.res)
			}
			revision := "1"
			if choice == "adopt" {
				revision = "2"
			}
			next := atm(t, r.Root, nil, "ticket", "prioritize", "--target", "ticket:acme:main:AT-0002", "--expected-revision", revision, "--request-id", "after-repair", "--issued-at", "2026-09-07T12:00:00Z", "--payload", `{"order":"0","priority":"P1"}`)
			if next.res.Outcome != wire.OutcomeOK {
				t.Fatalf("subsequent mutation: %+v", next.res)
			}
			fixture.Write(t, path, []byte("later edit must survive replay"))
			state, intents = fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
			again := atm(t, r.Root, discard, args...)
			if again.res.Outcome != wire.OutcomeOK || !field(again.res.Items[0], "replayed").Bool || field(again.res.Items[0], "ticketId").Str != "ticket:acme:main:AT-0002" {
				t.Fatalf("replay: %+v", again.res)
			}
			if !reflect.DeepEqual(state, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
				t.Fatal("retry changed later state")
			}
		})
	}
}

func TestTMV0007_AS35_CLIReconcileInputFailuresWriteNothing(t *testing.T) {
	for _, kind := range []string{"missing-choice", "both-choices", "duplicate", "adopt-digest", "invalid-role", "missing-file", "oversize", "symlink", "stale-digest"} {
		t.Run(kind, func(t *testing.T) {
			r := receiptFixture(t)
			path := filepath.Join(r.IntentDir, "tickets", "AT-0002.json")
			canonical, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			discard := []byte("manual edit")
			fixture.Write(t, path, discard)
			args := []string{"reconcile", "intent", "--target", "ticket:acme:main:AT-0002", "--file", "-", "--request-id", "bad", "--keep-journal", "--canonical-sha256", string(wire.Sum(canonical))}
			switch kind {
			case "missing-choice":
				args = args[:8]
			case "both-choices":
				args = append(args, "--adopt-file")
			case "duplicate":
				args = append(args, "--file", "-")
			case "adopt-digest":
				args[8] = "--adopt-file"
			case "invalid-role":
				args = append(args, "--role", "WORKER")
			case "missing-file":
				args[5] = "missing-copy"
			case "oversize":
				discard = []byte(strings.Repeat("x", wire.MaxTicketFileBytes+1))
			case "symlink":
				if err := os.Symlink(path, filepath.Join(r.Root, "linked-input")); err != nil {
					t.Fatal(err)
				}
				args[5] = "linked-input"
			case "stale-digest":
				args[10] = string(wire.Sum([]byte("wrong")))
			}
			state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
			x := atm(t, r.Root, discard, args...)
			if x.res.Outcome == wire.OutcomeOK {
				t.Fatalf("%s accepted", kind)
			}
			if !reflect.DeepEqual(state, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
				t.Fatal("bad input changed store")
			}
		})
	}
}

func TestTMV0008_AS35_CLIReconcileInspectFailureHasNoItem(t *testing.T) {
	for _, kind := range []string{"missing-target", "other-drift", "private-corruption", "restore", "scope", "usage"} {
		t.Run(kind, func(t *testing.T) {
			r := receiptFixture(t)
			args := []string{"reconcile", "inspect", "ticket:acme:main:AT-0002"}
			switch kind {
			case "missing-target":
				args[2] = "ticket:acme:main:MISSING"
			case "other-drift":
				fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), []byte("{}\n"))
			case "private-corruption":
				fixture.Write(t, filepath.Join(r.StateDir, "reservations.json"), []byte("{}\n"))
			case "restore":
				fixture.Write(t, filepath.Join(r.StateDir, "RESTORE_INCOMPLETE"), []byte("incomplete\n"))
			case "scope":
				args[2] = "ticket:other:main:AT-0002"
			case "usage":
				args = args[:2]
			}
			state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
			x := atm(t, r.Root, nil, args...)
			if x.res.Outcome == wire.OutcomeOK || len(x.res.Items) != 0 {
				t.Fatalf("unproved item: %+v", x.res)
			}
			fixture.AssertUntouched(t, r, state, intents, "inspect refusal")
		})
	}
}

func TestTMV0016_AS27_ReconcileInspectReportsJournalledBarrier(t *testing.T) {
	r := receiptFixture(t)
	headRaw, err := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	head, err := wire.Parse(headRaw)
	if err != nil {
		t.Fatal(err)
	}
	previous := wire.Digest(field(head, "lastReceiptSha256").Str)
	barrier := wire.NewObject()
	for key, value := range map[string]string{"profile": "taskman-barrier/0", "queueId": fixture.QueueID, "scope": "ALL", "reason": "EMERGENCY", "actor": "fixture", "sinceSeq": "3", "since": "2026-09-07T12:00:00Z"} {
		barrier.Set(key, wire.String(value))
	}
	value := wire.ObjectValue(barrier)
	raw := wire.EncodeFile(value)
	pre := wire.NewObject()
	pre.Set("path", wire.String("barrier.json"))
	pre.Set("sha256", wire.Null())
	post := wire.NewObject()
	post.Set("path", wire.String("barrier.json"))
	post.Set("sha256", wire.String(string(wire.Sum(raw))))
	post.Set("record", value)
	post.Set("blobSha256", wire.Null())
	receipt := fixture.ReceiptValue(3, &previous, "PAUSE", wire.Size(field(head, "generation").Str).Uint64())
	receipt.Obj.Set("pre", wire.Array(wire.ObjectValue(pre)))
	receipt.Obj.Set("post", wire.Array(wire.ObjectValue(post)))
	receiptRaw := wire.EncodeFile(receipt)
	fixture.Write(t, filepath.Join(r.StateDir, "barrier.json"), raw)
	fixture.Write(t, filepath.Join(r.StateDir, "receipts", "000000000003.json"), receiptRaw)
	head.Obj.Set("lastSeq", wire.String("3"))
	head.Obj.Set("lastReceiptSha256", wire.String(string(wire.Sum(receiptRaw))))
	fixture.Write(t, filepath.Join(r.StateDir, "head.json"), wire.EncodeFile(head))
	fixture.Write(t, filepath.Join(r.IntentDir, "tickets", "AT-0002.json"), []byte("divergent"))
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	x := atm(t, r.Root, nil, "reconcile", "inspect", "ticket:acme:main:AT-0002")
	if x.res.Outcome != wire.OutcomeOK || x.res.Snapshot == nil || x.res.Snapshot.Barrier == nil || x.res.Snapshot.Barrier.Scope != "ALL" || x.res.Snapshot.Barrier.Reason != "EMERGENCY" {
		t.Fatalf("barrier lost: %+v", x.res)
	}
	fixture.AssertUntouched(t, r, state, intents, "inspection under ALL")
}
