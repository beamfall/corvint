//go:build darwin || linux

package authority

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

var ftStop = errors.New("returned fixture fault; not process death")

func ftFlow(t *testing.T, op string) (*ftHarness, *transaction.Plan) {
	t.Helper()
	h := ftNew(t, op != transaction.Init)
	if op == transaction.Unpause {
		_, err := h.publish(h.plan(h.request(transaction.Pause, "pause-base")), nil)
		fixtureMust(t, err)
	}
	if op == transaction.KeepJournal {
		h.edit([]byte{})
	}
	if op == transaction.AdoptFile {
		c := fixture.Ticket("AT-01")
		c.Title = "offered"
		h.edit(c.Encode())
	}
	return h, h.plan(h.request(op, "fault-flow"))
}

func ftStopAt(h *ftHarness, p *transaction.Plan, step string) bool {
	h.t.Helper()
	committed, err := h.publish(p, func(got string) error {
		if got == step || (strings.HasPrefix(got, "durable-post:pinned/") && strings.HasPrefix(step, "durable-post:pinned/")) {
			return ftStop
		}
		return nil
	})
	if !errors.Is(err, ftStop) {
		h.t.Fatalf("stop %s: %v", step, err)
	}
	return committed
}

func ftUnchanged(t *testing.T, h *ftHarness, f func() error) {
	t.Helper()
	before := fixture.TreeSnapshot(t, h.r.Root)
	if err := f(); err == nil {
		t.Fatal("invalid actual bytes accepted")
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, h.r.Root)) {
		t.Fatal("refusal changed source/foreign bytes or metadata")
	}
}

func TestTMV0007_AS35_FixtureFoundationSeedRefusals(t *testing.T) {
	for _, broken := range []string{"blob-missing", "blob-corrupt", "request-missing", "request-corrupt", "ledger-malformed", "ledger-gap", "naked-ticket"} {
		t.Run(broken, func(t *testing.T) {
			h := ftNew(t, broken != "naked-ticket")
			blob := filepath.Join(h.r.StateDir, "evidence", string(fixture.Ticket("AT-01").FileDigest()))
			rp, _ := snapshot.RequestPath("seed")
			switch broken {
			case "blob-missing":
				fixtureMust(t, os.Remove(blob))
			case "blob-corrupt":
				fixtureMust(t, os.WriteFile(blob, []byte("foreign"), 0644))
			case "request-missing":
				fixtureMust(t, os.Remove(filepath.Join(h.r.StateDir, rp)))
			case "request-corrupt":
				fixtureMust(t, os.WriteFile(filepath.Join(h.r.StateDir, rp), []byte("{}\n"), 0644))
			case "ledger-malformed":
				fixtureMust(t, os.WriteFile(filepath.Join(h.r.StateDir, "receipts/000000000002.json"), []byte("{}\n"), 0644))
			case "ledger-gap":
				fixtureMust(t, os.Remove(filepath.Join(h.r.StateDir, "receipts/000000000001.json")))
			case "naked-ticket":
				fixture.Write(t, filepath.Join(h.r.IntentDir, "tickets/AT-01.json"), fixture.Ticket("AT-01").Encode())
				h.restart()
			}
			r := ftRequest(transaction.KeepJournal, "must-refuse")
			r.TargetID, r.File, r.CanonicalSha256 = fixture.TicketID("AT-01"), []byte{}, fixture.Ticket("AT-01").FileDigest()
			ftUnchanged(t, h, func() error { _, err := h.model(r); return err })
			fixtureAbsent(t, filepath.Join(h.r.StateDir, "staging"))
		})
	}
}

func TestTMV0006_AS03_FixtureFoundationNoArtifacts(t *testing.T) {
	for _, mode := range []string{"unpause-nochange", "pause-nochange", "keep-nochange", "wrong-actor", "protected-adopt", "adopt-empty-composition", "keep-malformed", "keep-valid-divergent", "stale-keep-choice"} {
		t.Run(mode, func(t *testing.T) {
			h := ftNew(t, true)
			op, want := transaction.Unpause, "NoChange"
			switch mode {
			case "pause-nochange":
				op = transaction.Pause
				_, err := h.publish(h.plan(h.request(op, "paused")), nil)
				fixtureMust(t, err)
			case "keep-nochange", "stale-keep-choice":
				op = transaction.KeepJournal
			case "wrong-actor":
				op, want = transaction.Pause, "Refused"
			case "protected-adopt":
				op, want = transaction.AdoptFile, "Refused"
				c := fixture.Ticket("AT-01")
				c.Revision = "2"
				c.PreviousRecordSha256 = ftDigest(c.Encode())
				h.edit(c.Encode())
			case "adopt-empty-composition":
				op, want = transaction.AdoptFile, "Transaction"
			case "keep-malformed":
				op, want = transaction.KeepJournal, "Transaction"
				h.edit([]byte("{malformed"))
			case "keep-valid-divergent":
				op, want = transaction.KeepJournal, "Transaction"
				c := fixture.Ticket("AT-01")
				c.Title = "human"
				h.edit(c.Encode())
			}
			r := h.request(op, "test")
			if mode == "wrong-actor" {
				r.Actor.Role = "WORKER"
			}
			if mode == "stale-keep-choice" {
				r.CanonicalSha256 = wire.Sum([]byte("stale C"))
				want = "Refused"
			}
			before := fixture.TreeSnapshot(t, h.r.Root)
			result, err := h.model(r)
			fixtureMust(t, err)
			if result.Kind != want {
				t.Fatalf("%s want %s: %s", result.Kind, want, result.Detail)
			}
			if !fixture.SameTree(before, fixture.TreeSnapshot(t, h.r.Root)) {
				t.Fatal("model path wrote artifacts")
			}
			if want != "Transaction" {
				if result.Plan != nil {
					t.Fatal("zero-artifact path returned plan")
				}
				return
			}
			_, err = h.publish(result.Plan, nil)
			fixtureMust(t, err)
			if mode == "adopt-empty-composition" {
				o, err := h.observe(false)
				fixtureMust(t, err)
				v, err := wire.Parse(o.latest[ftTicket])
				fixtureMust(t, err)
				rev, _ := v.Obj.Get("revision")
				if rev.Str != "2" {
					t.Fatal("unchanged ADOPT must bump revision")
				}
			}
		})
	}
}

// Enumerate the common table's actual finite boundaries, including every slot,
// evidence link, post, barrier deletion, head and descriptor cleanup.
func TestTMV0009_AS11_FixtureFoundationEveryPublicationPrefix(t *testing.T) {
	for _, op := range []string{transaction.Init, transaction.Pause, transaction.Unpause, transaction.KeepJournal, transaction.AdoptFile} {
		t.Run(op, func(t *testing.T) {
			steps := []string{}
			t.Run("enumerate", func(t *testing.T) {
				h, p := ftFlow(t, op)
				_, err := h.publish(p, func(step string) error { steps = append(steps, step); return nil })
				fixtureMust(t, err)
			})
			for _, step := range steps {
				t.Run(step, func(t *testing.T) {
					h, p := ftFlow(t, op)
					base, err := h.capture()
					fixtureMust(t, err)
					committed := ftStopAt(h, p, step)
					h.restart()
					if committed {
						fixtureMust(t, h.recover())
						ftFinal(t, h, p)
					} else {
						fixtureMust(t, h.cleanup(true))
						after, err := h.capture()
						fixtureMust(t, err)
						if !ftSameBase(base, after) || len(after.stage) != 0 {
							t.Fatal("abort changed base or lost cleanup")
						}
					}
				})
			}
		})
	}
}

func TestTMV0009_AS11_FixtureFoundationCommittedSyncFailure(t *testing.T) {
	for _, op := range []string{transaction.Init, transaction.Pause, transaction.Unpause, transaction.KeepJournal, transaction.AdoptFile} {
		t.Run(op, func(t *testing.T) {
			h, p := ftFlow(t, op)
			// The receipts directory may first be created here. Inject only when
			// the actual next receipt name exists, after native link succeeded.
			rc := mustReceipt(t, p.Receipt())
			name, _ := snapshot.ReceiptName(rc.Seq.Uint64())
			fired := false
			h.s.before = func(boundary string) error {
				if boundary != "sync:receipts" || fired {
					return nil
				}
				if _, err := os.Lstat(filepath.Join(h.r.StateDir, "receipts", name)); err == nil {
					fired = true
					return ftStop
				}
				return nil
			}
			committed, err := h.publish(p, nil)
			var effect *fixtureEffectError
			if !fired || !committed || !errors.Is(err, ftStop) || !errors.As(err, &effect) {
				t.Fatalf("lost performed receipt link: %v %v", committed, err)
			}
			h.restart()
			fixtureMust(t, h.recover())
			ftFinal(t, h, p)
		})
	}
}

func TestTMV0009_AS11_FixtureFoundationPendingRefusals(t *testing.T) {
	for _, mode := range []string{"blob-missing", "blob-corrupt", "ledger-malformed", "unaffected-private", "affected-private", "third-intent", "foreign-descriptor", "wrong-slot", "extra-receipt", "completed-post-mismatch"} {
		t.Run(mode, func(t *testing.T) {
			op := transaction.Pause
			if mode == "third-intent" {
				op = transaction.KeepJournal
			}
			h, p := ftFlow(t, op)
			stop := "durable-receipt-link"
			if mode == "completed-post-mismatch" {
				stop = "durable-head"
			}
			ftStopAt(h, p, stop)
			desc, err := snapshot.DecodeStageDescriptor(p.Descriptor())
			fixtureMust(t, err)
			switch mode {
			case "blob-missing":
				fixtureMust(t, os.Remove(filepath.Join(h.r.StateDir, "evidence", string(fixture.Ticket("AT-01").FileDigest()))))
			case "blob-corrupt":
				fixtureMust(t, os.WriteFile(filepath.Join(h.r.StateDir, "evidence", string(fixture.Ticket("AT-01").FileDigest())), []byte("foreign"), 0644))
			case "ledger-malformed":
				fixtureMust(t, os.WriteFile(filepath.Join(h.r.StateDir, "receipts/000000000002.json"), []byte("{}\n"), 0644))
			case "unaffected-private":
				fixtureMust(t, os.WriteFile(filepath.Join(h.r.StateDir, "reservations.json"), []byte("{}\n"), 0644))
			case "affected-private", "completed-post-mismatch":
				fixture.Write(t, filepath.Join(h.r.StateDir, "barrier.json"), []byte("third"))
			case "third-intent":
				h.edit([]byte("third human edit"))
			case "foreign-descriptor":
				desc.RecordedAt = "2026-09-06T12:00:01Z"
				raw, err := desc.Encode()
				fixtureMust(t, err)
				fixtureMust(t, os.WriteFile(filepath.Join(h.r.StateDir, "staging/active.json"), raw, 0600))
			case "wrong-slot":
				a := desc.Artifacts[0]
				raw := bytes.Repeat([]byte("x"), int(a.Bytes.Uint64()))
				fixtureMust(t, os.WriteFile(filepath.Join(h.r.StateDir, "staging", a.Slot), raw, 0600))
			case "extra-receipt":
				fixture.Write(t, filepath.Join(h.r.StateDir, "receipts/000000000005.json"), p.Receipt())
			}
			h.restart()
			ftUnchanged(t, h, h.recover)
		})
	}
}

func TestTMV0009_AS27_FixtureFoundationPartialAndForeignStage(t *testing.T) {
	for _, mode := range []string{"partial-slot", "truncated-temp", "malformed-active", "unknown", "symlink", "fifo", "directory", "wrong-complete-temp"} {
		t.Run(mode, func(t *testing.T) {
			h, p := ftFlow(t, transaction.Pause)
			if mode == "partial-slot" {
				writes := 0
				h.s.write = func(f *os.File, b []byte) (int, error) {
					writes++
					if writes == 2 {
						n, e := f.Write(b[:len(b)/2])
						return n, errors.Join(e, io.ErrShortWrite)
					}
					return f.Write(b)
				}
				h.s.before = func(b string) error {
					if b == "cleanup" && writes >= 2 {
						return ftStop
					}
					return nil
				}
				committed, err := h.publish(p, nil)
				if committed || !errors.Is(err, io.ErrShortWrite) {
					t.Fatalf("partial returned fault: %v", err)
				}
			} else {
				stop := "descriptor-link"
				if mode == "truncated-temp" {
					stop = "descriptor-temp"
				}
				ftStopAt(h, p, stop)
				stage := filepath.Join(h.r.StateDir, "staging")
				switch mode {
				case "truncated-temp":
					fixtureMust(t, os.WriteFile(filepath.Join(stage, "active.json.tmp"), []byte("{bad"), 0600))
				case "malformed-active":
					fixtureMust(t, os.WriteFile(filepath.Join(stage, "active.json"), []byte("{}\n"), 0600))
				case "unknown":
					fixture.Write(t, filepath.Join(stage, "foreign"), []byte("retain"))
				case "symlink":
					fixtureMust(t, os.Symlink("/no-such-fixture", filepath.Join(stage, "a00")))
				case "fifo":
					fixtureMust(t, syscall.Mkfifo(filepath.Join(stage, "a00"), 0600))
				case "directory":
					fixtureMust(t, os.Mkdir(filepath.Join(stage, "a00"), 0700))
				case "wrong-complete-temp":
					// Replace the temp inode; modifying its shared inode would also
					// corrupt active and would test a different boundary.
					fixtureMust(t, os.Remove(filepath.Join(stage, "active.json.tmp")))
					fixture.Write(t, filepath.Join(stage, "active.json.tmp"), bytes.Repeat([]byte("x"), len(p.Descriptor())))
				}
			}
			h.restart()
			if mode != "partial-slot" && mode != "truncated-temp" {
				ftUnchanged(t, h, func() error { return h.cleanup(true) })
				return
			}
			o, err := h.observe(false)
			fixtureMust(t, err)
			tokens, err := h.reopen(o, true)
			fixtureMust(t, err)
			for _, token := range tokens {
				if h.s.stages[token] {
					t.Fatal("partial/cleanup token can publish")
				}
				if h.s.link(token, fixtureTarget{fixtureEvidence, string(token.digest)}) == nil {
					t.Fatal("cleanup token used for publication")
				}
			}
			fixtureMust(t, h.cleanup(true))
		})
	}
}

func TestTMV0009_AS27_FixtureFoundationOrphansAndMissingSlots(t *testing.T) {
	t.Run("abort-to-unpause", func(t *testing.T) {
		h := ftNew(t, true)
		_, err := h.publish(h.plan(h.request(transaction.Pause, "paused")), nil)
		fixtureMust(t, err)
		discard := []byte("retained malformed D")
		h.edit(discard)
		p := h.plan(h.request(transaction.KeepJournal, "aborted-keep"))
		capacity, err := transaction.CheckCapacity(p)
		fixtureMust(t, err)
		var reserve *transaction.Cost
		for _, prefix := range capacity.Prefixes {
			if prefix.Step == "durable-link:evidence/"+string(wire.Sum(discard)) {
				reserve = prefix.AbortToUnpause
			}
		}
		if reserve == nil {
			t.Fatal("accepted abort-to-UNPAUSE reserve missing")
		}
		ftStopAt(h, p, "durable-link:evidence/"+string(wire.Sum(discard)))
		h.restart()
		fixtureMust(t, h.cleanup(true))
		fixtureBytes(t, filepath.Join(h.r.StateDir, "evidence", string(wire.Sum(discard))), string(discard))
		fixtureBytes(t, filepath.Join(h.r.IntentDir, "tickets/AT-01.json"), string(discard))
		r := h.request(transaction.Unpause, "unpause-after-abort")
		unpause := h.plan(r)
		fixtureMust(t, h.costWithin(*reserve, unpause.Head(), false))
		_, err = h.publish(unpause, func(string) error { return h.costWithin(*reserve, unpause.Head(), false) })
		fixtureMust(t, err)
		fixtureBytes(t, filepath.Join(h.r.StateDir, "evidence", string(wire.Sum(discard))), string(discard))
		before := fixture.TreeSnapshot(t, h.r.Root)
		replay, err := h.model(r)
		fixtureMust(t, err)
		r.RequestID = "already-unpaused"
		unchanged, err := h.model(r)
		fixtureMust(t, err)
		if replay.Kind != "Replay" || unchanged.Kind != "NoChange" || !fixture.SameTree(before, fixture.TreeSnapshot(t, h.r.Root)) {
			t.Fatal("abort retry artifacts")
		}
		orphanPath := filepath.Join(h.r.StateDir, "evidence", string(wire.Sum(discard)))
		orphan, err := os.Lstat(orphanPath)
		fixtureMust(t, err)
		_, err = h.publish(h.plan(h.request(transaction.KeepJournal, "reuse-orphan")), nil)
		fixtureMust(t, err)
		fixturePreserved(t, orphanPath, orphan, string(discard))
	})
	for _, op := range []string{transaction.Init, transaction.Pause, transaction.Unpause, transaction.KeepJournal} {
		t.Run("missing-slots/"+op, func(t *testing.T) {
			h, p := ftFlow(t, op)
			ftStopAt(h, p, "durable-receipt-link")
			// Deliberate owned fixture subset: no original slot survives. All
			// missing posts/head must come from the actual committed proof.
			for _, a := range p.Artifacts() {
				fixtureMust(t, h.s.removeStage(fixtureSlot(a.Slot)))
			}
			h.restart()
			done := ftRecoveryCostWitness(t, h, p)
			fixtureMust(t, h.recover())
			done()
			ftFinal(t, h, p)
		})
	}
}

func ftFinal(t *testing.T, h *ftHarness, p *transaction.Plan) {
	t.Helper()
	fixtureMust(t, h.costWithin(ftFinalCost(t, p), p.Head(), true))
	cap, err := transaction.CheckCapacity(p)
	fixtureMust(t, err)
	d, err := h.capture()
	fixtureMust(t, err)
	i, err := d.inventory()
	fixtureMust(t, err)
	if !reflect.DeepEqual(i.Files(), cap.Final.Files()) || !reflect.DeepEqual(i.Directories(), cap.Final.Directories()) || !bytes.Equal(d.raw["head.json"], p.Head()) {
		t.Fatal("actual final inventory/head differs from independent oracle")
	}
}

func TestTMV0014_AS10_FixtureFoundationHeadDigitGrowth(t *testing.T) {
	h := ftNew(t, true)
	var nine []byte
	for i := 3; i <= 10; i++ {
		op := transaction.Pause
		if i%2 == 0 {
			op = transaction.Unpause
		}
		p := h.plan(h.request(op, fmt.Sprintf("digit-%d", i)))
		_, err := h.publish(p, nil)
		fixtureMust(t, err)
		ftFinal(t, h, p)
		if i == 9 {
			nine = p.Head()
		}
		if i == 10 && len(p.Head()) != len(nine)+1 {
			t.Fatal("actual head digit growth not witnessed")
		}
	}
}

func TestTMV0001_AS10_FixtureFoundationSourceLifetime(t *testing.T) {
	for _, mode := range []string{"released-lock", "old-token", "source-replaced", "parent-replaced"} {
		t.Run(mode, func(t *testing.T) {
			h, p := ftFlow(t, transaction.KeepJournal)
			ftStopAt(h, p, "slot:a00")
			var token *fixtureStage
			for s := range h.s.stages {
				if s.slot == "a00" {
					token = s
				}
			}
			switch mode {
			case "released-lock":
				fixtureMust(t, h.s.lock.Close())
			case "old-token":
				h.restart()
			case "source-replaced":
				fixtureMust(t, os.Rename(filepath.Join(h.r.StateDir, "staging/a00"), filepath.Join(h.r.StateDir, "staging/saved")))
				fixture.Write(t, filepath.Join(h.r.StateDir, "staging/a00"), []byte("foreign"))
			case "parent-replaced":
				fixtureMust(t, os.Rename(filepath.Join(h.r.StateDir, "staging"), filepath.Join(h.r.StateDir, "saved-staging")))
				fixtureMust(t, os.Mkdir(filepath.Join(h.r.StateDir, "staging"), 0700))
			}
			ftUnchanged(t, h, func() error {
				return h.s.link(token, fixtureTarget{fixtureEvidence, string(token.digest)})
			})
		})
	}
	for _, branch := range []string{"main", "other"} {
		t.Run("ticket-branch-"+branch, func(t *testing.T) {
			h, p := ftFlow(t, transaction.KeepJournal)
			if !ftStopAt(h, p, "durable-receipt-link") {
				t.Fatal("not committed")
			}
			var token *fixtureStage
			for _, a := range p.Artifacts() {
				if a.Target != ftTicket {
					continue
				}
				for s := range h.s.stages {
					if string(s.slot) == a.Slot {
						token = s
					}
				}
			}
			if token == nil {
				t.Fatal("missing actual ticket token")
			}
			pre, err := os.ReadFile(filepath.Join(h.r.IntentDir, "tickets/AT-01.json"))
			fixtureMust(t, err)
			fixtureMust(t, os.WriteFile(filepath.Join(h.r.CommonDir, "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0644))
			if branch != "main" {
				ftUnchanged(t, h, func() error { return h.replaceTicket(token, ftDigest(pre), transaction.KeepJournal) })
				return
			}
			fixtureMust(t, h.replaceTicket(token, ftDigest(pre), transaction.KeepJournal))
			raw, err := os.ReadFile(filepath.Join(h.r.IntentDir, "tickets/AT-01.json"))
			fixtureMust(t, err)
			if wire.Sum(raw) != token.digest {
				t.Fatal("valid branch did not publish ticket")
			}
		})
	}
}

// Returned syscall errors run Go defers. The finite matrix does not assert the
// disk layout is the one a killed process or physical power loss would leave.
func TestTMV0009_AS11_FixtureFoundationReturnedFaults(t *testing.T) {
	for _, op := range []string{transaction.Init, transaction.Pause, transaction.Unpause, transaction.KeepJournal, transaction.AdoptFile} {
		t.Run(op, func(t *testing.T) {
			boundaries := map[string]bool{}
			t.Run("enumerate", func(t *testing.T) {
				h, p := ftFlow(t, op)
				h.s.before = func(b string) error { boundaries[b] = true; return nil }
				_, err := h.publish(p, nil)
				fixtureMust(t, err)
			})
			names := []string{}
			for name := range boundaries {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, boundary := range names {
				t.Run(boundary, func(t *testing.T) {
					h, p := ftFlow(t, op)
					fired := false
					h.s.before = func(b string) error {
						if !fired && b == boundary {
							fired = true
							return ftStop
						}
						return nil
					}
					committed, err := h.publish(p, nil)
					if !fired || !errors.Is(err, ftStop) {
						t.Fatalf("fault %s was not exercised: %v", boundary, err)
					}
					h.restart()
					if committed {
						fixtureMust(t, h.recover())
						return
					}
					if h.s.parents["staging"] != nil {
						fixtureMust(t, h.cleanup(true))
					}
				})
			}
		})
	}
}

func TestTMV0014_AS10_FixtureFoundationCapacityAndCleanupDebt(t *testing.T) {
	h, p := ftFlow(t, transaction.KeepJournal)
	cap, err := transaction.CheckCapacity(p)
	fixtureMust(t, err)
	// Keep an independent syscall-level byte witness. The accepted initial
	// envelope covers prepare's create/short-write/unlink/sync prefixes.
	checks := 0
	h.s.before = func(boundary string) error {
		if boundary != "write" && boundary != "file-sync" && boundary != "stage-close" {
			return nil
		}
		d, err := h.scan()
		if err != nil {
			return err
		}
		got, err := d.cost(p.Head())
		if err != nil {
			return err
		}
		bound := cap.Prefixes[0].Cost
		gv, bv := reflect.ValueOf(got), reflect.ValueOf(bound)
		for i := 0; i < gv.NumField(); i++ {
			if gv.Field(i).Uint() > bv.Field(i).Uint() {
				return fmt.Errorf("prepare cost %s", gv.Type().Field(i).Name)
			}
		}
		checks++
		return nil
	}
	ftStopAt(h, p, "slot:"+p.Artifacts()[len(p.Artifacts())-1].Slot)
	h.s.before = nil
	if checks < 3*len(p.Artifacts()) {
		t.Fatal("missing actual prepare cost evidence")
	}
	o, err := h.observe(false)
	fixtureMust(t, err)
	inv, err := o.disk.inventory()
	fixtureMust(t, err)
	obs := o.stageObservation(inv)
	state := transaction.CleanupState{}
	initial, err := transaction.Cleanup(obs, state)
	fixtureMust(t, err)
	// Interruption after unlink but before directory sync must retain the name
	// and byte debt, even though a new listing can no longer see that name.
	h.s.before = func(b string) error {
		if b == "sync:staging" {
			return ftStop
		}
		return nil
	}
	err = h.s.removeStage(fixtureSlot(p.Artifacts()[0].Slot))
	if !errors.Is(err, ftStop) {
		t.Fatal("cleanup sync fault missing")
	}
	state.Removed = []string{p.Artifacts()[0].Slot}
	debt, err := transaction.Cleanup(obs, state)
	fixtureMust(t, err)
	if debt.TemporaryBytes != initial.TemporaryBytes || debt.Entries != initial.Entries || debt.PromisedBytes != initial.PromisedBytes {
		t.Fatal("early unlink credit")
	}
	if _, err = transaction.Cleanup(obs, transaction.CleanupState{DescriptorUnlinked: true}); err == nil {
		t.Fatal("descriptor removed before payload sync")
	}
	h.restart()
	fixtureMust(t, h.cleanup(true))
	fixtureAbsent(t, filepath.Join(h.r.StateDir, "staging/active.json"))
}

// Replay preserves the complete original outcome, not a new no-change result.
func TestTMV0006_AS03_FixtureFoundationReplayOutcome(t *testing.T) {
	h, p := ftFlow(t, transaction.KeepJournal)
	r := h.request(transaction.KeepJournal, "fault-flow")
	_, err := h.publish(p, nil)
	fixtureMust(t, err)
	o, err := h.observe(false)
	fixtureMust(t, err)
	rp, _ := snapshot.RequestPath(r.RequestID)
	stored, err := snapshot.DecodeRequest(o.latest[rp])
	fixtureMust(t, err)
	got, err := h.model(r)
	fixtureMust(t, err)
	want := stored.Entry.Outcome
	want.Replayed = true
	if !reflect.DeepEqual(got.Outcome, want) || got.Kind != "Replay" {
		t.Fatal("full original replay outcome lost")
	}
	if got.Coverage.ActorAuthentication != transaction.NotObserved || got.Coverage.AdministrativeAuthorization != transaction.NotObserved || got.Coverage.RuntimeQualification != transaction.NotObserved {
		t.Fatal("fixture promoted model authority")
	}
	r.Actor.ID = "conflicting"
	conflict, err := h.model(r)
	fixtureMust(t, err)
	if conflict.Outcome.Outcome != mutation.OutcomeRequestIDConflict {
		t.Fatal("conflict became replay")
	}
	if strings.Contains(got.Detail, "qualified") {
		t.Fatal("unexpected qualification claim")
	}
}

func TestTMV0009_AS27_FixtureFoundationCleanupBoundaries(t *testing.T) {
	for _, op := range []string{transaction.Init, transaction.Pause, transaction.Unpause, transaction.KeepJournal, transaction.AdoptFile} {
		t.Run(op, func(t *testing.T) {
			// Count the actual retained names, since rename consumes some slots.
			var count int
			t.Run("inventory", func(t *testing.T) {
				h, p := ftFlow(t, op)
				ftStopAt(h, p, "durable-head")
				d, err := h.capture()
				fixtureMust(t, err)
				count = len(d.stage)
			})
			for cut := 1; cut <= count+1; cut++ {
				t.Run(fmt.Sprint(cut), func(t *testing.T) {
					h, p := ftFlow(t, op)
					ftStopAt(h, p, "durable-head")
					calls := 0
					h.s.before = func(b string) error {
						if b != "sync:staging" {
							return nil
						}
						calls++
						if calls == cut {
							return ftStop
						}
						return nil
					}
					err := h.cleanup(false)
					if !errors.Is(err, ftStop) {
						t.Fatalf("cleanup cut %d/%d: %v", cut, count, err)
					}
					h.s.before = nil
					// Even empty staging after a failed active unlink sync holds
					// the old era until a fresh recovery confirms directory sync.
					if h.era == "" {
						t.Fatal("premature cleanup discharge")
					}
					h.restart()
					fixtureMust(t, h.recover())
					ftFinal(t, h, p)
					if h.era != "" {
						t.Fatal("cleanup debt remained")
					}
				})
			}
		})
	}
}

func TestTMV0009_AS11_FixtureFoundationStalePlan(t *testing.T) {
	h, p := ftFlow(t, transaction.KeepJournal)
	h.edit([]byte("new D after Model"))
	ftUnchanged(t, h, func() error { _, err := h.publish(p, nil); return err })
	fixtureAbsent(t, filepath.Join(h.r.StateDir, "staging"))
}

// An external test observer can retain a Plan-derived capacity oracle, but it
// supplies no data to recovery. Every actual syscall prefix (including absent
// slots being rebuilt) must fit a single accepted conservative prefix.
func ftRecoveryCostWitness(t *testing.T, h *ftHarness, p *transaction.Plan) func() {
	t.Helper()
	cap, err := transaction.CheckCapacity(p)
	fixtureMust(t, err)
	head := p.Head()
	var debt transaction.Cost
	checks := 0
	last := ""
	h.s.before = func(boundary string) error {
		d, err := h.scan()
		if err != nil {
			return err
		}
		current, err := d.cost(head)
		if err != nil {
			return err
		}
		// This witness has no injected failures: reaching the next non-sync
		// boundary after a directory sync proves that sync returned success.
		// Consecutive destination/source syncs retain both rename-name debts.
		if strings.HasPrefix(last, "sync:") && !strings.HasPrefix(boundary, "sync:") {
			debt = current
		}
		a, b := reflect.ValueOf(&debt).Elem(), reflect.ValueOf(current)
		for i := 0; i < a.NumField(); i++ {
			if b.Field(i).Uint() > a.Field(i).Uint() {
				a.Field(i).SetUint(b.Field(i).Uint())
			}
		}
		covered := false
		for _, prefix := range cap.Prefixes {
			if prefix.Step == "reserved-fresh-UNPAUSE" {
				continue
			}
			bound := reflect.ValueOf(prefix.Cost)
			fits := true
			for i := 0; i < a.NumField(); i++ {
				if a.Field(i).Uint() > bound.Field(i).Uint() {
					fits = false
				}
			}
			covered = covered || fits
		}
		if !covered {
			return fmt.Errorf("model dependency before %s: actual bytes plus unsynced debt %+v exceed every accepted prefix", boundary, debt)
		}
		last = boundary
		checks++
		return nil
	}
	return func() {
		h.s.before = nil
		if checks == 0 {
			t.Fatal("no actual recovery capacity observations")
		}
	}
}

func TestTMV0009_AS11_FixtureFoundationRenameSyncAndRelease(t *testing.T) {
	for _, op := range []string{transaction.Pause, transaction.Unpause, transaction.KeepJournal, transaction.AdoptFile} {
		for _, sync := range []string{"destination", "source"} {
			t.Run(op+"/"+sync, func(t *testing.T) {
				h, p := ftFlow(t, op)
				rename, fired := false, false
				h.s.before = func(b string) error {
					if b == "native:rename" {
						rename = true
					}
					if !rename || fired || !strings.HasPrefix(b, "sync:") {
						return nil
					}
					if (sync == "source") != (b == "sync:staging") {
						return nil
					}
					fired = true
					return ftStop
				}
				committed, err := h.publish(p, nil)
				var effect *fixtureEffectError
				if !fired || !committed || !errors.Is(err, ftStop) || !errors.As(err, &effect) {
					t.Fatalf("rename sync: committed=%v %v", committed, err)
				}
				h.restart()
				fixtureMust(t, h.recover())
				ftFinal(t, h, p)
			})
		}
	}
	for _, kind := range []string{"file-close", "root-close"} {
		t.Run(kind, func(t *testing.T) {
			h, p := ftFlow(t, transaction.Pause)
			_, err := h.publish(p, nil)
			fixtureMust(t, err)
			old := h.s
			if kind == "file-close" {
				old.closeFile = func(f *os.File) error { return errors.Join(f.Close(), ftStop) }
			}
			if kind == "root-close" {
				old.closeRoot = func(r *os.Root) error { return errors.Join(r.Close(), ftStop) }
			}
			err = old.close()
			if !errors.Is(err, ftStop) || old.close() != err {
				t.Fatal("release error not sticky")
			}
			if old.operation(func() error { return nil }) == nil {
				t.Fatal("released session usable")
			}
			fixtureMust(t, h.open()) // all closes and actual flock release occurred
			fixtureMust(t, h.recover())
		})
	}
}

func TestTMV0009_AS11_FixtureFoundationStaleInventory(t *testing.T) {
	h := ftNew(t, true)
	orphan := []byte("orphan observed before Model")
	name := filepath.Join(h.r.StateDir, "evidence", string(wire.Sum(orphan)))
	fixture.Write(t, name, orphan)
	p := h.plan(h.request(transaction.Pause, "stale-inventory"))
	base := h.bases[p]
	// Accessor copies cannot alter the saved inventory.
	files, dirs := base.Files(), base.Directories()
	files[0].Path = "changed-copy"
	dirs[0] = "changed-copy"
	fixtureMust(t, os.Remove(name))
	ftUnchanged(t, h, func() error {
		committed, err := h.publish(p, nil)
		if committed || h.era != "" {
			t.Fatal("stale inventory started an era")
		}
		return err
	})
	if _, err := os.Lstat(filepath.Join(h.r.StateDir, "staging")); !os.IsNotExist(err) {
		t.Fatal("stale inventory created staging")
	}
}
