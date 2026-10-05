//go:build darwin || linux

package cli_test

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0074_QueueStatusAdmissionPressure: queue status reports registered
// preparation writers and the would-be rank from live slots only, names the
// omitted wait estimate, and leaves the journal, intent and coordination
// files byte-identical.
func TestCALV0074_QueueStatusAdmissionPressure(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.WriteIntent(t, r)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	admission := func() wire.Value {
		t.Helper()
		x := atm(t, r.Root, nil, "queue", "status")
		if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
			t.Fatalf("queue status: %+v", x.res)
		}
		return field(x.res.Items[0], "preparationAdmission")
	}
	a := admission()
	if field(a, "snapshot").Str != "RACY" || field(a, "method").Str == "NOT_OBSERVED" || field(a, "notObservedReason").Kind != wire.KindNull ||
		field(a, "registeredWriters").Str != "0" || field(a, "unpublishedSlots").Str != "0" || field(a, "wouldBeRank").Str != "1" ||
		field(a, "registryActive").Kind != wire.KindBool || field(a, "capacity").Str != "64" ||
		field(a, "estimatedWait").Str != "NOT_OBSERVED" || field(a, "estimatedWaitBasis").Str != "no recorded per-mutation writer cost" {
		t.Fatalf("idle admission: %s", wire.Encode(a))
	}
	if _, err := os.Lstat(filepath.Join(repo.CommonDir, "taskman.prepare.registry.lock")); !os.IsNotExist(err) {
		t.Fatalf("read created the registry: %v", err)
	}
	// Three live registrations (ranks 4..6) and one stale record (rank 9).
	slot := func(i int, rank uint64, hold bool) {
		f, err := os.OpenFile(filepath.Join(repo.CommonDir, fmt.Sprintf("taskman.prepare.slot.%02d", i)), os.O_RDWR|os.O_CREATE, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		rec := make([]byte, 16)
		copy(rec, "CPA1")
		binary.BigEndian.PutUint64(rec[8:], rank)
		if _, err = f.WriteAt(rec, 0); err != nil {
			t.Fatal(err)
		}
		if hold {
			if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				t.Fatal(err)
			}
		}
	}
	slot(0, 9, false)
	slot(1, 4, true)
	slot(2, 5, true)
	slot(3, 6, true)
	prepare := func() map[string]string {
		out := map[string]string{}
		for i := 0; i < 4; i++ {
			p := filepath.Join(repo.CommonDir, fmt.Sprintf("taskman.prepare.slot.%02d", i))
			st, err := os.Stat(p)
			raw, e := os.ReadFile(p)
			if err != nil || e != nil {
				t.Fatal(err, e)
			}
			out[p] = st.ModTime().String() + string(raw)
		}
		return out
	}
	state, intended, slots := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir), prepare()
	a = admission()
	if field(a, "snapshot").Str != "RACY" || field(a, "registeredWriters").Str != "3" || field(a, "unpublishedSlots").Str != "0" || field(a, "wouldBeRank").Str != "7" {
		t.Fatalf("contended admission: %s", wire.Encode(a))
	}
	if !reflect.DeepEqual(state, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(intended, fixture.TreeSnapshot(t, r.IntentDir)) || !reflect.DeepEqual(slots, prepare()) {
		t.Fatal("queue status wrote journal, intent or coordination bytes")
	}
	if _, err := os.Lstat(filepath.Join(repo.CommonDir, "taskman.prepare.registry.lock")); !os.IsNotExist(err) {
		t.Fatalf("read created the registry: %v", err)
	}
}
