package snapshot

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func maximalRelocate() Relocate {
	q, _ := wire.ParseQueueID("", "queue:a:"+strings.Repeat("q", 120))
	return Relocate{
		QueueID:             q,
		RequestID:           strings.Repeat(`"`, wire.MaxRequestIDBytes),
		FromPrimaryWorktree: "/" + strings.Repeat(`"`, wire.MaxPathTextBytes-1),
		ToPrimaryWorktree:   "/" + strings.Repeat(`\`, wire.MaxPathTextBytes-1),
	}
}

// TRL-V0-002: the relocate descriptor names both paths, is closed and
// canonical, and its pinned POST cap is its exact canonical maximum.
func TestTRLV0002_RelocateDescriptorCodec(t *testing.T) {
	d := maximalRelocate()
	raw := wire.EncodeFile(d.Value())
	if len(raw) != MaxRelocateDescriptorBytes {
		t.Fatalf("maximal relocate descriptor %d bytes, want %d", len(raw), MaxRelocateDescriptorBytes)
	}
	got, err := DecodeRelocate(raw)
	if err != nil {
		t.Fatal(err)
	}
	if *got != d {
		t.Fatalf("round trip %+v want %+v", *got, d)
	}
	same := d
	same.ToPrimaryWorktree = same.FromPrimaryWorktree
	if _, err := DecodeRelocate(wire.EncodeFile(same.Value())); err == nil {
		t.Fatal("relocation to the same primary worktree accepted")
	}
	rel := d
	rel.ToPrimaryWorktree = "relative/path"
	if _, err := DecodeRelocate(wire.EncodeFile(rel.Value())); err == nil {
		t.Fatal("relative destination accepted")
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(ProfileRelocate))
	o.Set("queueId", wire.String(d.QueueID.Raw))
	o.Set("requestId", wire.String("r"))
	o.Set("fromPrimaryWorktree", wire.String("/a"))
	o.Set("toPrimaryWorktree", wire.String("/b"))
	o.Set("extra", wire.String("x"))
	if _, err := DecodeRelocate(wire.EncodeFile(wire.ObjectValue(o))); err == nil {
		t.Fatal("open relocate descriptor accepted")
	}
	if _, err := DecodeRelocate(make([]byte, MaxRelocateDescriptorBytes+1)); err == nil {
		t.Fatal("oversized relocate descriptor accepted")
	}
}
