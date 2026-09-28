package store_test

import (
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
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
