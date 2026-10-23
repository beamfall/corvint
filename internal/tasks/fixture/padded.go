package fixture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PadOptions shapes PadLiveness.
type PadOptions struct {
	// Heartbeats is how many liveness receipts to append, spread round-robin
	// over the live attempts of the store.
	Heartbeats int
	// PerSecond is how many appended receipts share one recordedAt second
	// (default 100), so a long pad stays inside the attempts' leases.
	PerSecond int
	// Elide writes each superseded liveness receipt in its elided form
	// (taskman-receipt/1, CAL-V0-300), keeping an attempt's last receipt, and
	// any receipt that would lie snapshot.MaxElidedRun or more receipts after
	// the attempt's previous whole one, whole as liveness compaction does
	// (CAL-V0-302): the distance is in journal sequence, so a store with many
	// attempts keeps proportionally more of them whole.
	Elide bool
}

// PadReport counts what PadLiveness wrote.
type PadReport struct {
	Appended, Elided, Whole int
	Attempts                int
	LastSeq                 uint64
	// Elapsed is the wall time PadLiveness took.
	Elapsed time.Duration
}

// padAttempt is one live attempt the pad extends.
type padAttempt struct {
	path     string
	template *wire.Object // the attempt's latest whole liveness receipt
	record   *wire.Object
	bytes    []byte // the attempt file's current bytes
	whole    uint64 // sequence of the attempt's latest whole receipt
	left     int    // receipts still to append for this attempt
}

// PadLiveness appends opts.Heartbeats synthetic completed heartbeat receipts
// to the store at stateDir, in O(opts.Heartbeats) file writes and no audit,
// and returns what it wrote. It is the padded-journal builder that a
// large-history measurement needs where one real write costs about 0.2 s
// through the command line (V1-2137): each receipt is the attempt's latest
// whole liveness receipt re-sequenced, re-chained and re-timed, posting the
// attempt record with a new lastHeartbeatAt and its own request afterimage,
// so the store audits CONSISTENT and AGREES as one built by real heartbeats.
//
// The store must hold at least one live attempt (an attempt file with a
// lease) with a whole completed liveness receipt among its last 4,096
// receipts: do one real heartbeat per attempt first. It must be unsegmented
// and have no retained checkpoint newer than its head, which a store built
// by the fixture and the command line does not. Only the attempt files, the
// request files, the receipts and head.json change.
func PadLiveness(tb testing.TB, stateDir string, opts PadOptions) PadReport {
	tb.Helper()
	started := time.Now()
	if opts.PerSecond <= 0 {
		opts.PerSecond = 100
	}
	headPath := filepath.Join(stateDir, "head.json")
	raw, err := os.ReadFile(headPath)
	if err != nil {
		tb.Fatal(err)
	}
	head, err := snapshot.DecodeHead(raw)
	if err != nil {
		tb.Fatal(err)
	}
	if head.Segments != nil || head.LastReceiptSha256 == nil {
		tb.Fatal("PadLiveness needs an unsegmented store with a head receipt")
	}
	first := head.LastSeq.Uint64()
	attempts := padAttempts(tb, stateDir, first)
	if len(attempts) == 0 || opts.Heartbeats <= 0 {
		tb.Fatalf("PadLiveness needs live attempts and a positive count: %d attempts, %d heartbeats", len(attempts), opts.Heartbeats)
	}
	for i := range attempts {
		attempts[i].left = opts.Heartbeats / len(attempts)
		if i < opts.Heartbeats%len(attempts) {
			attempts[i].left++
		}
	}
	base := padRecordedAt(tb, stateDir, first)
	rep := PadReport{Attempts: len(attempts)}
	prev := *head.LastReceiptSha256
	for i := 0; i < opts.Heartbeats; i++ {
		seq := first + 1 + uint64(i)
		a := &attempts[i%len(attempts)]
		at := base.Add(time.Duration(i/opts.PerSecond) * time.Second).UTC().Format(time.RFC3339)
		id := "pad-" + string(wire.SizeOf(seq))
		requestPath, err := snapshot.RequestPath(id)
		if err != nil {
			tb.Fatal(err)
		}
		record := copyPadObject(a.record)
		record.Set("lastHeartbeatAt", str(at))
		recordBytes := wire.EncodeFile(wire.ObjectValue(record))
		request := obj(
			"mutationSha256", str(string(wire.Sum([]byte(id)))),
			"outcome", obj("codes", wire.Strings(nil), "outcome", str("COMPLETED"), "profile", str("taskman-outcome/0"), "receiptSeq", str(string(wire.SizeOf(seq))), "replayed", wire.Bool(false), "requestId", str(id), "resultingAcceptanceRevision", wire.Null(), "resultingRevision", wire.Null()),
			"requestId", str(id),
			"seq", str(string(wire.SizeOf(seq))),
		)
		requestBytes := wire.EncodeFile(request)
		receipt := copyPadObject(a.template)
		receipt.Set("seq", str(string(wire.SizeOf(seq))))
		receipt.Set("headGeneration", str(string(head.Generation)))
		receipt.Set("prev", str(string(prev)))
		receipt.Set("recordedAt", str(at))
		receipt.Set("requestId", str(id))
		receipt.Set("pre", wire.Array(
			obj("path", str(a.path), "sha256", str(string(wire.Sum(a.bytes)))),
			obj("path", str(requestPath), "sha256", wire.Null()),
		))
		receipt.Set("post", wire.Array(
			obj("blobSha256", wire.Null(), "path", str(a.path), "record", wire.ObjectValue(record), "sha256", str(string(wire.Sum(recordBytes)))),
			obj("blobSha256", wire.Null(), "path", str(requestPath), "record", request, "sha256", str(string(wire.Sum(requestBytes)))),
		))
		original := wire.EncodeFile(wire.ObjectValue(receipt))
		a.left--
		out, elided := original, false
		if opts.Elide && a.left > 0 && seq-a.whole < snapshot.MaxElidedRun {
			if tombstone, ok, err := snapshot.ElideLiveness(original, a.bytes); err != nil {
				tb.Fatalf("receipt %d: %v", seq, err)
			} else if ok {
				out, elided = tombstone, true
			}
		}
		name, _ := snapshot.ReceiptName(seq)
		Write(tb, filepath.Join(stateDir, "receipts", name), out)
		Write(tb, filepath.Join(stateDir, filepath.FromSlash(requestPath)), requestBytes)
		if elided {
			rep.Elided++
		} else {
			a.whole = seq
			rep.Whole++
		}
		a.record, a.bytes = record, recordBytes
		prev = wire.Sum(original)
	}
	for _, a := range attempts {
		Write(tb, filepath.Join(stateDir, filepath.FromSlash(a.path)), a.bytes)
	}
	last := first + uint64(opts.Heartbeats)
	head.LastSeq, head.LastReceiptSha256 = wire.SizeOf(last), &prev
	Write(tb, headPath, wire.EncodeFile(head.Value()))
	rep.Appended, rep.LastSeq, rep.Elapsed = opts.Heartbeats, last, time.Since(started)
	return rep
}

// padAttempts lists the live attempts of the store with their latest whole
// liveness receipt as a template.
func padAttempts(tb testing.TB, stateDir string, head uint64) []padAttempt {
	tb.Helper()
	entries, err := os.ReadDir(filepath.Join(stateDir, "attempts"))
	if err != nil {
		tb.Fatal(err)
	}
	var out []padAttempt
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(stateDir, "attempts", e.Name()))
		if err != nil {
			tb.Fatal(err)
		}
		v, err := wire.Parse(raw)
		if err != nil || v.Kind != wire.KindObject {
			tb.Fatalf("attempt %s: %v", e.Name(), err)
		}
		if lease, ok := v.Obj.Get("lease"); !ok || lease.Kind != wire.KindObject {
			continue // a released or ended attempt has no lease to keep
		}
		id, _ := v.Obj.Get("attemptId")
		path := "attempts/" + e.Name()
		a := padAttempt{path: path, record: v.Obj, bytes: raw}
		for seq := head; seq >= 1 && head-seq < 4096 && a.template == nil; seq-- {
			name, _ := snapshot.ReceiptName(seq)
			rraw, err := os.ReadFile(filepath.Join(stateDir, "receipts", name))
			if err != nil {
				tb.Fatal(err)
			}
			rv, err := wire.Parse(rraw)
			if err != nil || rv.Kind != wire.KindObject {
				continue
			}
			if p, _ := rv.Obj.Get("profile"); p.Str != snapshot.ProfileReceipt {
				continue
			}
			if got, _ := rv.Obj.Get("attemptId"); got.Str != id.Str {
				continue
			}
			if kind, _ := rv.Obj.Get("kind"); kind.Str != "TRANSITION" {
				continue
			}
			if post, _ := rv.Obj.Get("post"); len(post.Arr) != 2 {
				continue
			}
			a.template, a.whole = rv.Obj, seq
		}
		if a.template == nil {
			tb.Fatalf("attempt %s has no whole liveness receipt among the last 4096: do a real heartbeat first", id.Str)
		}
		out = append(out, a)
	}
	return out
}

// padRecordedAt is the head receipt's recordedAt, the pad's first second.
func padRecordedAt(tb testing.TB, stateDir string, head uint64) time.Time {
	tb.Helper()
	name, _ := snapshot.ReceiptName(head)
	raw, err := os.ReadFile(filepath.Join(stateDir, "receipts", name))
	if err != nil {
		tb.Fatal(err)
	}
	v, err := wire.Parse(raw)
	if err != nil || v.Kind != wire.KindObject {
		tb.Fatalf("head receipt: %v", err)
	}
	at, _ := v.Obj.Get("recordedAt")
	t, err := time.Parse(time.RFC3339, at.Str)
	if err != nil {
		tb.Fatalf("head receipt recordedAt: %v", err)
	}
	return t
}

func copyPadObject(o *wire.Object) *wire.Object {
	out := wire.NewObject()
	for _, k := range o.Keys {
		out.Set(k, o.Vals[k])
	}
	return out
}
