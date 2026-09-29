package store_test

import (
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"testing"
)

func TestCALV0018_ImportPublicationFailureRestarts(t *testing.T) {
	t.Run("CAL-V0-018 failed publication", testCALV0018ImportFailure)
}
func testCALV0018ImportFailure(t *testing.T) {
	for _, role := range []string{"RECEIPT", "POST", "HEAD"} {
		t.Run(role, func(t *testing.T) {
			repo := importStore(t, "ROADMAP")
			items := []string{}
			for n := 0; n < 24; n++ {
				id := "BF-" + string(wire.CountOf(int64(n)))
				items = append(items, importItem(id, id+"\n", nil))
			}
			export := importExport(items...)
			seen := 0
			injected := errors.New("injected publication failure")
			restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
				if a.Role == role {
					seen++
					if seen == 2 {
						return injected
					}
				}
				return nil
			})
			_, err := store.Import(context.Background(), repo, operator(), fixture.QueueID, export, now(t))
			restore()
			if !errors.Is(err, injected) {
				t.Fatalf("fault missing: %v", err)
			}
			completed := runImport(t, repo, export)
			if len(completed.Batches) == 0 {
				t.Fatal("restart did not finish remaining import")
			}
			before := storeDigest(t, repo)
			again := runImport(t, repo, export)
			if again.Planned != 0 || storeDigest(t, repo) != before {
				t.Fatal("restart/reimport was not idempotent")
			}
		})
	}
}

func TestCALV0018_ImportCancellationAndProjectionDrift(t *testing.T) {
	t.Run("CAL-V0-018 bound import observations", testCALV0018ImportDrift)
}
func testCALV0018ImportDrift(t *testing.T) {
	for _, mode := range []string{"initial", "delete", "replace", "already-post", "type", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			repo := importStore(t, "ROADMAP")
			items := []string{}
			for n := 0; n < 24; n++ {
				id := "BF-" + string(wire.CountOf(int64(n)))
				items = append(items, importItem(id, id+"\n", nil))
			}
			runImport(t, repo, importExport(items...))
			for n := 0; n < 24; n++ {
				id := "BF-" + string(wire.CountOf(int64(n)))
				items[n] = importItem(id, id+" changed\n", nil)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			seen := 0
			var editedDigest wire.Digest
			receiptCount := 0
			hook := func(batch [][]byte) error {
				seen++
				target := 2
				if mode == "initial" {
					target = 1
				}
				if seen != target {
					return nil
				}
				entries, e := os.ReadDir(filepath.Join(repo.StateDir, "receipts"))
				if e != nil {
					return e
				}
				receiptCount = len(entries)
				if mode == "cancel" {
					cancel()
					return nil
				}
				rec, err := ticket.Decode(batch[0])
				if err != nil {
					return err
				}
				path := filepath.Join(repo.PrimaryWorktree, ".taskman", "tickets", rec.TicketID.Local+".json")
				if mode == "already-post" {
					err = os.WriteFile(path, batch[0], 0600)
				} else if mode == "type" {
					if err = os.Remove(path); err == nil {
						err = os.Mkdir(path, 0700)
					}
				} else if mode == "delete" {
					err = os.Remove(path)
				} else {
					err = os.WriteFile(path, []byte("{}\n"), 0600)
				}
				if mode == "initial" {
					editedDigest = storeDigest(t, repo)
				}
				return err
			}
			report, err := store.ImportBeforeBatchForTest(ctx, repo, operator(), fixture.QueueID, importExport(items...), now(t), hook)
			entries, e := os.ReadDir(filepath.Join(repo.StateDir, "receipts"))
			if e != nil {
				t.Fatal(e)
			}
			if len(entries) != receiptCount {
				t.Fatal("fresh receipt linked after failed guard")
			}
			if mode == "initial" {
				if err == nil && len(report.Batches) > 0 && report.Batches[0].Receipt != "" {
					t.Fatal("initial binding accepted drift")
				}
				if storeDigest(t, repo) != editedDigest {
					t.Fatal("initial refusal wrote")
				}
				return
			}
			if len(report.Batches) != 1 || err == nil {
				t.Fatalf("later batch retained: %+v %v", report, err)
			}
			if mode == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}
