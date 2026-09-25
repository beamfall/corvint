package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestTMV0008_AS36_ReceiptAuditDiscardsMovedResults(t *testing.T) {
	for _, kind := range []string{"intent", "head"} {
		t.Run(kind, func(t *testing.T) {
			r := fixture.TempRepo(t)
			fixture.WriteState(t, r)
			fixture.WriteIntent(t, r)
			calls := 0
			res := receiptAudit(Env{Cwd: r.Root, afterRead: func() {
				calls++
				if kind == "intent" {
					q := fixture.QueueValue()
					q.Obj.Set("intentBranch", wire.String("branch"+strconv.Itoa(calls)))
					fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), wire.EncodeFile(q))
				} else {
					path := filepath.Join(r.StateDir, "head.json")
					raw, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					v, err := wire.Parse(raw)
					if err != nil {
						t.Fatal(err)
					}
					v.Obj.Set("generation", wire.String(strconv.Itoa(calls)))
					fixture.Write(t, path, wire.EncodeFile(v))
				}
			}}, nil)
			if calls != 4 || res.Outcome != wire.OutcomeNotRun || len(res.Codes) != 1 || res.Codes[0] != wire.CodeSnapshotMoved || len(res.Items) != 0 {
				t.Fatalf("mixed/stale evidence: calls=%d result=%+v", calls, res)
			}
			if _, err := os.Lstat(filepath.Join(r.CommonDir, "taskman.lock")); !os.IsNotExist(err) {
				t.Fatalf("audit created lock: %v", err)
			}
		})
	}
}
