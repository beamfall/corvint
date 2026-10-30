package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCTSV0007_ReimportKeepsRefinedPoolAndRoles (V1-0759): REFINE can set
// requiresPool and requiredRoles on a shadow IMPORT record, and a re-import
// with a changed block keeps both byte-identical without bumping
// acceptanceRevision for them.
func TestCTSV0007_ReimportKeepsRefinedPoolAndRoles(t *testing.T) {
	t.Parallel()
	tmp := fixture.TempRepo(t)
	q := fixture.QueueValue()
	q.Obj.Set("canonicalWriter", str("ROADMAP"))
	fixture.Write(t, filepath.Join(tmp.IntentDir, "queue.json"), wire.EncodeFile(q))
	p := fixture.PolicyValue()
	p.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a"}))))
	fixture.Write(t, filepath.Join(tmp.IntentDir, "policy.json"), wire.EncodeFile(p))
	resolved, err := intent.Resolve(tmp.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Init(context.Background(), resolved, operator(), "req-init", now(t)); err != nil {
		t.Fatalf("init: %v", err)
	}
	repo := resolved

	runImport(t, repo, importExport(importItem("BF-1", "one\n", nil)))
	imported, _ := readImported(t, repo, "BF-1")
	roles := obj("implement", wire.Strings([]string{"BUILDER"}), "review", wire.Strings([]string{"REVIEWER"}), "integrate", wire.Strings([]string{"VERIFIER"}))
	refined := mutate(t, repo, envelope("req-refine", mutation.OpRefine, imported.TicketID.Raw, "1", obj("requiresPool", str("db"), "requiredRoles", roles)))
	if refined.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("REFINE on an IMPORT record = %s %s", refined.Outcome.Outcome, refined.Detail)
	}
	carrier, _ := readImported(t, repo, "BF-1")
	if carrier.Source.Kind != "IMPORT" || carrier.RequiresPool != "db" || len(carrier.RequiredRoles) != 3 || carrier.Revision != "2" {
		t.Fatalf("IMPORT record cannot carry the keys: %+v", carrier)
	}

	runImport(t, repo, importExport(importItem("BF-1", "one, changed\n", nil)))
	next, _ := readImported(t, repo, "BF-1")
	if next.Revision != "3" || *next.PreviousRecordSha256 != carrier.FileDigest() {
		t.Fatalf("next revision: %+v", next)
	}
	for _, k := range []string{"requiresPool", "requiredRoles"} {
		a, _ := carrier.Value().Obj.Get(k)
		b, ok := next.Value().Obj.Get(k)
		if !ok || string(wire.EncodeFile(a)) != string(wire.EncodeFile(b)) {
			t.Fatalf("re-import changed %s: %s -> %s", k, wire.EncodeFile(a), wire.EncodeFile(b))
		}
	}
	// The changed block changes source (acceptance-relevant) exactly once;
	// the kept keys add no further bump.
	if next.AcceptanceRevision != wire.CountOf(carrier.AcceptanceRevision.Int()+1) {
		t.Fatalf("acceptanceRevision %s -> %s", carrier.AcceptanceRevision, next.AcceptanceRevision)
	}
	auditOK(t, repo)
}
