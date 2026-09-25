package snapshot_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func receiptWithPost(path string, deleted bool) wire.Value {
	prev := wire.Sum([]byte("previous"))
	v := fixture.ReceiptValue(2, &prev, "MUTATION", 0)
	rec := wire.ObjectValue(wire.NewObject())
	digest := wire.String(string(wire.Sum(wire.EncodeFile(rec))))
	if deleted {
		rec = wire.Null()
		digest = wire.Null()
		v.Obj.Set("kind", wire.String("UNPAUSE"))
	}
	pre := wire.NewObject()
	pre.Set("path", wire.String(path))
	pre.Set("sha256", wire.String(string(prev)))
	post := wire.NewObject()
	post.Set("path", wire.String(path))
	post.Set("sha256", digest)
	post.Set("record", rec)
	post.Set("blobSha256", wire.Null())
	v.Obj.Set("pre", wire.Array(wire.ObjectValue(pre)))
	v.Obj.Set("post", wire.Array(wire.ObjectValue(post)))
	return v
}

func TestTMV0002_AS01_ClosedReceiptPathsPairsAndDeletion(t *testing.T) {
	for _, p := range []string{"head.json", "receipts/000000000002.json", "worktrees/x", "tickets/A.json", "intent/tickets/../A.json", "intent//queue.json", "intent/tickets/A\\B.json", "effects/" + strings.Repeat("a", 64) + ".boot", "effects/" + strings.Repeat("a", 64) + ".ack", "head.json.tmp", "attempts/invalid.json", "requests/ab/" + strings.Repeat("a", 64) + ".json"} {
		t.Run(p, func(t *testing.T) {
			_, err := snapshot.DecodeReceipt(wire.EncodeFile(receiptWithPost(p, false)))
			if err == nil {
				t.Fatal("forbidden path accepted")
			}
		})
	}
	for _, kind := range []string{"duplicate", "unpaired", "unsorted", "wrong outcome", "non-unpause", "other deletion", "null pre", "partial deletion", "inline digest", "blob digest", "blob and inline", "neither", "late INIT", "PRUNE"} {
		t.Run(kind, func(t *testing.T) {
			v := receiptWithPost("barrier.json", false)
			pre, _ := v.Obj.Get("pre")
			post, _ := v.Obj.Get("post")
			switch kind {
			case "duplicate":
				v.Obj.Set("pre", wire.Array(pre.Arr[0], pre.Arr[0]))
				v.Obj.Set("post", wire.Array(post.Arr[0], post.Arr[0]))
			case "unpaired":
				v.Obj.Set("pre", wire.Array())
			case "unsorted":
				p2 := receiptWithPost("VERSION", true)
				pre2, _ := p2.Obj.Get("pre")
				post2, _ := p2.Obj.Get("post")
				v.Obj.Set("pre", wire.Array(pre.Arr[0], pre2.Arr[0]))
				v.Obj.Set("post", wire.Array(post.Arr[0], post2.Arr[0]))
			case "wrong outcome":
				v.Obj.Set("outcome", wire.String("OK"))
			case "non-unpause":
				v = receiptWithPost("barrier.json", true)
				v.Obj.Set("kind", wire.String("MUTATION"))
			case "other deletion":
				v = receiptWithPost("intent/queue.json", true)
			case "null pre":
				v = receiptWithPost("barrier.json", true)
				p, _ := v.Obj.Get("pre")
				p.Arr[0].Obj.Set("sha256", wire.Null())
			case "partial deletion":
				post.Arr[0].Obj.Set("sha256", wire.Null())
			case "inline digest":
				post.Arr[0].Obj.Set("sha256", wire.String(strings.Repeat("0", 64)))
			case "blob digest":
				post.Arr[0].Obj.Set("record", wire.Null())
				post.Arr[0].Obj.Set("blobSha256", wire.String(strings.Repeat("0", 64)))
			case "blob and inline":
				post.Arr[0].Obj.Set("blobSha256", wire.String(strings.Repeat("0", 64)))
			case "neither":
				post.Arr[0].Obj.Set("record", wire.Null())
			case "late INIT":
				v.Obj.Set("kind", wire.String("INIT"))
			case "PRUNE":
				v.Obj.Set("kind", wire.String("PRUNE"))
			}
			if _, err := snapshot.DecodeReceipt(wire.EncodeFile(v)); err == nil {
				t.Fatal("malformed receipt accepted")
			}
		})
	}
	if _, err := snapshot.DecodeReceipt(wire.EncodeFile(receiptWithPost("barrier.json", true))); err != nil {
		t.Fatal("valid deletion", err)
	}
}

func TestTMV0002_AS10_ReceiptRequestIDBytesAndInitDescriptor(t *testing.T) {
	prev := wire.Sum([]byte("prev"))
	for _, id := range []string{strings.Repeat("a", 64), strings.Repeat("é", 32), strings.Repeat("a", 65), strings.Repeat("é", 33), ""} {
		v := fixture.ReceiptValue(2, &prev, "MUTATION", 0)
		v.Obj.Set("requestId", wire.String(id))
		_, err := snapshot.DecodeReceipt(wire.EncodeFile(v))
		if len(id) > 0 && len(id) <= 64 {
			if err != nil {
				t.Fatal(err)
			}
		} else if err == nil {
			t.Fatalf("bad request %d bytes", len(id))
		}
	}
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	init := snapshot.Init{QueueID: q, PrimaryWorktree: "/" + strings.Repeat("p", 4095), VersionSha256: wire.Sum([]byte(snapshot.VersionBytes))}
	raw := wire.EncodeFile(init.Value())
	got, err := snapshot.DecodeInit(raw)
	if err != nil || got.PrimaryWorktree != init.PrimaryWorktree {
		t.Fatal(err)
	}
	for _, edit := range []func(wire.Value){func(v wire.Value) { v.Obj.Set("unknown", wire.Null()) }, func(v wire.Value) { v.Obj.Set("profile", wire.String("taskman-init/1")) }, func(v wire.Value) { v.Obj.Set("primaryWorktree", wire.String("relative")) }} {
		v := init.Value()
		edit(v)
		if _, err := snapshot.DecodeInit(wire.EncodeFile(v)); err == nil {
			t.Fatal("invalid descriptor accepted")
		}
	}
}
