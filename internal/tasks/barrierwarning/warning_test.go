// SPDX-License-Identifier: Apache-2.0
package barrierwarning_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/barrierwarning"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestPURV02210_BarrierAgePolicy(t *testing.T) {
	fixture.SkipUnsupportedPlatform(t)
	r := fixture.TempRepo(t)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	since, _ := time.Parse(time.RFC3339, fixture.Timestamp)
	snap := &snapshot.Snapshot{Barrier: &snapshot.Barrier{Since: wire.Timestamp(fixture.Timestamp), SinceSeq: "1"}}
	for _, tc := range []struct {
		limit wire.Count
		age   time.Duration
		want  bool
	}{
		{"", time.Hour, false}, {"", time.Hour + time.Second, true}, {"10", 10 * time.Second, false}, {"10", 11 * time.Second, true}, {"10", -time.Second, false},
	} {
		got := barrierwarning.Message(repo, &intent.Policy{BarrierWarningSeconds: tc.limit}, snap, since.Add(tc.age))
		if (got != "") != tc.want {
			t.Fatalf("%+v: %s", tc, got)
		}
		if tc.want && !strings.Contains(got, "pause request NOT_OBSERVED, age ") {
			t.Fatalf("invented request: %s", got)
		}
	}
	for _, value := range []string{"0", "604801", "-1", "01", "10"} {
		p := fixture.PolicyValue()
		p.Obj.Set("barrierWarningSeconds", wire.String(value))
		decoded, err := intent.DecodePolicy(wire.EncodeFile(p))
		if value == "10" {
			if err != nil || decoded.BarrierWarningSeconds != "10" {
				t.Fatalf("valid: %+v %v", decoded, err)
			}
		} else if err == nil {
			t.Fatalf("accepted invalid %s", value)
		}
	}
}

// A request label is shown only when the named PAUSE receipt actually posts
// these barrier bytes. Missing or contradictory evidence never invents a label.
func TestPURV02210_ReceiptBinding(t *testing.T) {
	fixture.SkipUnsupportedPlatform(t)
	r := fixture.TempRepo(t)
	fixture.WriteIntent(t, r)
	fixture.WriteState(t, r)
	b := wire.NewObject().Set("profile", wire.String("taskman-barrier/0")).Set("queueId", wire.String(fixture.QueueID)).Set("scope", wire.String("ADMISSION")).Set("reason", wire.String("OPERATOR")).Set("actor", wire.String("russell")).Set("sinceSeq", wire.String("2")).Set("since", wire.String(fixture.Timestamp))
	fixture.CommitPosts(t, r, "PAUSE", "evidence-request", map[string][]byte{"barrier.json": wire.EncodeFile(wire.ObjectValue(b))})
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := snapshot.Probe(r.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	since, _ := time.Parse(time.RFC3339, fixture.Timestamp)
	if got := barrierwarning.Message(repo, &intent.Policy{}, snap, since.Add(2*time.Hour)); !strings.Contains(got, "pause request evidence-request, age 7200s") {
		t.Fatalf("valid receipt: %s", got)
	}
	snap.BarrierRaw = append(append([]byte(nil), snap.BarrierRaw...), ' ')
	if got := barrierwarning.Message(repo, &intent.Policy{}, snap, since.Add(2*time.Hour)); !strings.Contains(got, "pause request NOT_OBSERVED") {
		t.Fatalf("mismatched post: %s", got)
	}
}
