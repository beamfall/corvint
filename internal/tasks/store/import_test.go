package store_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/importer"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// importStore initializes the fixture repository with the given queue
// canonicalWriter.
func importStore(t *testing.T, writer string) *intent.Repository {
	t.Helper()
	repo := fixture.TempRepo(t)
	q := fixture.QueueValue()
	q.Obj.Set("canonicalWriter", str(writer))
	fixture.Write(t, filepath.Join(repo.IntentDir, "queue.json"), wire.EncodeFile(q))
	fixture.Write(t, filepath.Join(repo.IntentDir, "policy.json"), fixture.PolicyBytes())
	resolved, err := intent.Resolve(repo.Root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err = store.Init(context.Background(), resolved, operator(), "req-init", now(t)); err != nil {
		t.Fatalf("init: %v", err)
	}
	return resolved
}

// importItem is one export line: the item's mapped ticket with defaults the
// edit may change, its body the verbatim block.
func importItem(id, block string, edit func(*wire.Object)) string {
	t := obj(
		"acceptanceCriteria", wire.Strings([]string{"it exists"}),
		"archivedFrom", wire.Null(),
		"body", str(block),
		"capabilities", wire.Strings(nil),
		"completion", wire.Null(),
		"dependencies", wire.Array(),
		"dueDate", wire.Null(),
		"effects", obj("coverage", str("QUALIFIED"), "externalUnbounded", wire.Bool(false), "resources", wire.Array(), "touchPaths", wire.Strings(nil)),
		"estimateMinutes", wire.Null(),
		"executionClass", str("AUTONOMOUS"),
		"holds", wire.Array(),
		"kind", str("FEATURE"),
		"labels", wire.Strings([]string{"alias:" + strings.ToLower(id)}),
		"milestone", wire.Null(),
		"order", str("1"),
		"owner", wire.Null(),
		"priority", str("P2"),
		"requiredGates", wire.Strings(nil),
		"requirementRefs", wire.Strings(nil),
		"status", str("OPEN"),
		"supersededBy", wire.Null(),
		"supersedes", wire.Null(),
		"title", str("Imported "+id),
	)
	if edit != nil {
		edit(t.Obj)
	}
	return string(wire.Encode(obj("sourceItemId", str(id), "block", str(block), "ticket", t)))
}

func importExport(items ...string) []byte {
	header := string(wire.Encode(obj("profile", str(importer.Profile), "sourceQueueId", str("queue:beamfall:main"))))
	return []byte(strings.Join(append([]string{header}, items...), "\n") + "\n")
}

func runImport(t *testing.T, repo *intent.Repository, export []byte) *store.ImportReport {
	t.Helper()
	report, err := store.Import(context.Background(), repo, operator(), fixture.QueueID, export, now(t))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	for _, b := range report.Batches {
		if b.Outcome.Outcome != mutation.OutcomeCompleted || b.Receipt == "" {
			t.Fatalf("batch: %+v", b)
		}
	}
	return report
}

func readImported(t *testing.T, repo *intent.Repository, local string) (*ticket.Record, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", local+".json"))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := ticket.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return rec, raw
}

func TestCTSV0003_ImportWritesShadowRecordsAndReimportIsIdempotent(t *testing.T) {
	repo := importStore(t, "ROADMAP")
	dep := func(o *wire.Object) {
		o.Set("dependencies", wire.Array(obj("gateId", wire.Null(), "obligation", str("COMPLETED"), "ticketId", str(fixture.TicketID("BF-1")))))
	}
	export := importExport(importItem("BF-2", "## BF-2\nsecond\n", dep), importItem("BF-1", "## BF-1\nfirst\n", nil))
	report := runImport(t, repo, export)
	if report.Items != 2 || report.Planned != 2 || len(report.Batches) != 1 {
		t.Fatalf("report: %+v", report)
	}
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "receipts", report.Batches[0].Receipt))
	if err != nil {
		t.Fatal(err)
	}
	if rc, err := snapshot.DecodeReceipt(raw); err != nil || rc.Kind != "IMPORT_APPLY" || rc.TicketID != nil {
		t.Fatalf("receipt: %+v %v", rc, err)
	}
	rec, _ := readImported(t, repo, "BF-1")
	if rec.Source.Kind != "IMPORT" || *rec.Source.SourceItemID != "BF-1" || rec.Source.SourceQueueID != "queue:beamfall:main" ||
		*rec.Source.SourceRevisionSha256 != wire.Sum([]byte("## BF-1\nfirst\n")) || !rec.ShadowOverlay || rec.Revision != "1" {
		t.Fatalf("record: %+v", rec)
	}
	auditOK(t, repo)

	before := storeDigest(t, repo)
	again := runImport(t, repo, export)
	if again.Planned != 0 || len(again.Batches) != 0 || storeDigest(t, repo) != before {
		t.Fatalf("re-import wrote: %+v", again)
	}
}

func TestCTSV0003_ChangedBlockWritesNextRevision(t *testing.T) {
	repo := importStore(t, "ROADMAP")
	runImport(t, repo, importExport(importItem("BF-1", "one\n", nil), importItem("BF-2", "two\n", nil)))
	old, oldRaw := readImported(t, repo, "BF-1")
	// BF-2 is absent from the later export and must not be deleted.
	report := runImport(t, repo, importExport(importItem("BF-1", "one, changed\n", nil)))
	if report.Planned != 1 || len(report.Batches) != 1 {
		t.Fatalf("report: %+v", report)
	}
	rec, _ := readImported(t, repo, "BF-1")
	if rec.Revision != "2" || rec.PreviousRecordSha256 == nil || *rec.PreviousRecordSha256 != wire.Sum(oldRaw) ||
		rec.CreatedAt != old.CreatedAt || *rec.Source.SourceRevisionSha256 != wire.Sum([]byte("one, changed\n")) || rec.Body == nil || *rec.Body != "one, changed\n" {
		t.Fatalf("next revision: %+v", rec)
	}
	if kept, _ := readImported(t, repo, "BF-2"); kept.Revision != "1" {
		t.Fatalf("absent item changed: %+v", kept)
	}
	auditOK(t, repo)
}

func TestCTSV0003_ImportRefusesOverNativeRecord(t *testing.T) {
	repo := importStore(t, "ROADMAP")
	created := mutate(t, repo, envelope("req-native", "CREATE", "", "", createPayload("native")))
	if created.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create: %+v", created)
	}
	id, err := wire.ParseTicketID("ticket", created.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	before := storeDigest(t, repo)
	_, err = store.Import(context.Background(), repo, operator(), fixture.QueueID, importExport(importItem("BF-1", "one\n", nil), importItem(id.Local, "clash\n", nil)), now(t))
	if wire.CodeOf(err) != wire.CodeDuplicateID || storeDigest(t, repo) != before {
		t.Fatalf("native clash: %v", err)
	}
}

func TestCTSV0003_ImportRefusesWithNothingWritten(t *testing.T) {
	cases := []struct {
		name, writer string
		export       []byte
		want         string
	}{
		{"NATIVE writer", "NATIVE", importExport(importItem("BF-1", "one\n", nil)), wire.CodeUnsupported},
		{"duplicate item", "ROADMAP", importExport(importItem("BF-1", "one\n", nil), importItem("BF-1", "again\n", nil)), wire.CodeDuplicateID},
		{"unknown gate", "ROADMAP", importExport(importItem("BF-1", "one\n", nil), importItem("BF-2", "two\n", func(o *wire.Object) {
			o.Set("requiredGates", wire.Strings([]string{"beamfall-core-gate"}))
		})), wire.CodeGateUnknown},
		{"missing dependency", "ROADMAP", importExport(importItem("BF-1", "one\n", func(o *wire.Object) {
			o.Set("dependencies", wire.Array(obj("gateId", wire.Null(), "obligation", str("COMPLETED"), "ticketId", str(fixture.TicketID("BF-9")))))
		})), wire.CodeDependencyMissing},
		{"invalid record", "ROADMAP", importExport(importItem("BF-1", "one\n", func(o *wire.Object) { o.Set("status", str("HELD")) })), wire.CodeMalformed},
	}
	for _, c := range cases {
		repo := importStore(t, c.writer)
		before := storeDigest(t, repo)
		_, err := store.Import(context.Background(), repo, operator(), fixture.QueueID, c.export, now(t))
		if wire.CodeOf(err) != c.want || storeDigest(t, repo) != before {
			t.Errorf("%s: code %q, want %q (%v)", c.name, wire.CodeOf(err), c.want, err)
		}
	}
}

// TestCTSV0003_ImportBatchesWithinStageLimits imports more records than one
// IMPORT_APPLY stage holds, including one over the inline post bound.
func TestCTSV0003_ImportBatchesWithinStageLimits(t *testing.T) {
	repo := importStore(t, "ROADMAP")
	items := []string{importItem("BF-0", strings.Repeat("x", wire.MaxBodyBytes), nil)}
	for i := 1; i <= 24; i++ {
		id := "BF-" + string(wire.CountOf(int64(i)))
		items = append(items, importItem(id, id+"\n", func(o *wire.Object) {
			o.Set("status", str("COMPLETED"))
			o.Set("completion", obj("actor", str("beamfall"), "reason", str("legacy checkoff")))
		}))
	}
	export := importExport(items...)
	report := runImport(t, repo, export)
	if report.Planned != 25 || len(report.Batches) < 3 {
		t.Fatalf("report: planned %d batches %d", report.Planned, len(report.Batches))
	}
	if rec, _ := readImported(t, repo, "BF-7"); rec.Completion == nil || rec.Completion.Kind != "MANUAL" {
		t.Fatalf("completion: %+v", rec.Completion)
	}
	auditOK(t, repo)
	before := storeDigest(t, repo)
	if again := runImport(t, repo, export); again.Planned != 0 || storeDigest(t, repo) != before {
		t.Fatalf("re-import wrote: %+v", again)
	}
}

// TestCALV0018_LaterBatchesCheckTheirHeadAndPosts: the audit is carried
// across batches, yet a batch still refuses a head other than the one the
// previous batch wrote, and a ticket file that changed before it is posted.
func TestCALV0018_LaterBatchesCheckTheirHeadAndPosts(t *testing.T) {
	items := []string{}
	for i := 0; i < 24; i++ {
		id := "BF-" + string(wire.CountOf(int64(i)))
		items = append(items, importItem(id, id+"\n", nil))
	}
	export := importExport(items...)
	want := map[string]string{"head": "SNAPSHOT_MOVED", "post": "INTENT_DIVERGED"}
	for name, spoil := range map[string]func(t *testing.T, repo *intent.Repository, first []byte, batch [][]byte){
		"head": func(t *testing.T, repo *intent.Repository, first []byte, _ [][]byte) {
			fixture.Write(t, filepath.Join(repo.StateDir, "head.json"), first)
		},
		"post": func(t *testing.T, repo *intent.Repository, _ []byte, batch [][]byte) {
			rec, err := ticket.Decode(batch[0])
			if err != nil {
				t.Fatal(err)
			}
			fixture.Write(t, filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", rec.TicketID.Local+".json"), []byte("{}\n"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			repo := importStore(t, "ROADMAP")
			var first []byte
			seen := 0
			hook := func(batch [][]byte) error {
				seen++
				raw, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
				if err != nil {
					return err
				}
				if seen == 1 {
					first = raw
				}
				if seen == 2 {
					spoil(t, repo, first, batch)
				}
				return nil
			}
			report, err := store.ImportBeforeBatchForTest(context.Background(), repo, operator(), fixture.QueueID, export, now(t), hook)
			if err == nil || len(report.Batches) != 1 {
				t.Fatalf("second batch committed: %d batches, %v", len(report.Batches), err)
			}
			if !strings.Contains(err.Error(), want[name]) {
				t.Fatalf("refusal = %v, want %s", err, want[name])
			}
		})
	}
}
