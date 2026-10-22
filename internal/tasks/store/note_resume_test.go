package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// writerNoteEnvelope is a NOTE_SET (or NOTE_CLEAR, with empty text) request
// for target.
func writerNoteEnvelope(id, target, supersedes, text string) []byte {
	s := wire.String
	op, payload := mutation.OpNoteSet, historyObject("supersedes", s(supersedes), "text", s(text))
	if text == "" {
		op, payload = mutation.OpNoteClear, historyObject("supersedes", s(supersedes))
	}
	return wire.EncodeFile(historyObject(
		"profile", s(mutation.Profile),
		"requestId", s(id),
		"actor", historyObject("id", s(historyActor.ID), "role", s(historyActor.Role)),
		"queueId", s(fixture.QueueID),
		"targetId", s(target),
		"expectedRevision", wire.Null(),
		"operation", s(op),
		"payload", payload,
		"issuedAt", s("2026-09-07T12:00:00Z"),
	))
}

// ON-V0-012 (proposed): a note receipt in a writer's tail whose ticket was
// last posted at or before the writer checkpoint keeps the write on the
// writer route. Consecutive notes, a note after a re-base, and the ordinary
// write after each note all commit without a decline.
func TestONV0012_WriterResumesAcrossNoteReceipts(t *testing.T) {
	t.Parallel()
	repo := writerStore(t, 70)
	created := writerMutate(t, repo, "resume-target", nil)
	if !created.completed() {
		t.Fatalf("create: %+v %v", created.rep, created.err)
	}
	steps := []struct {
		id, supersedes, text string
	}{
		{"resume-note-1", "0", "First."},
		{"resume-note-2", "1", "Second."},
		{"resume-note-3", "2", ""},
	}
	for _, step := range steps {
		note := stagedWrite(writerEnvelope(repo, writerNoteEnvelope(step.id, created.rep.Ticket, step.supersedes, step.text), WallClock()), nil)
		if !note.completed() || !note.fast() || note.declined() != "" {
			t.Fatalf("%s: %+v %v %v", step.id, note.rep, note.err, note.stages)
		}
		next := writerMutate(t, repo, "after-"+step.id, nil)
		if !next.completed() || !next.fast() || next.declined() != "" {
			t.Fatalf("write after %s: %+v %v %v", step.id, next.rep, next.err, next.stages)
		}
	}
}

// A policy update in the tail before a note on a prefix ticket: the note's
// pre-policy is the tail afterimage and its pre-ticket binds to the
// checkpoint entry, so the next write and a read from the read checkpoint
// both stay on their resumed routes (ON-V0-012, ON-V0-013).
func TestONV0012_PolicyChangedInTailBeforeNote(t *testing.T) {
	t.Parallel()
	repo := writerStore(t, 70)
	entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("tickets: %v", err)
	}
	target := fixture.TicketID(strings.TrimSuffix(entries[0].Name(), ".json"))
	holdLeasePolicy(t, repo)
	cpRaw, err := os.ReadFile(journal.CheckpointPath(repo.StateDir))
	if err != nil {
		t.Fatal(err)
	}
	cp, err := journal.DecodeCheckpoint(cpRaw)
	if err != nil {
		t.Fatal(err)
	}
	note := stagedWrite(writerEnvelope(repo, writerNoteEnvelope("policy-tail-note", target, "0", "After policy."), WallClock()), nil)
	if !note.completed() || note.declined() != "" {
		t.Fatalf("note: %+v %v %v", note.rep, note.err, note.stages)
	}
	head, err := writerGuards(repo, "MUTATE")
	if err != nil {
		t.Fatal(err)
	}
	reader := journalReader(repo, head)
	reader.Checkpoint = cp
	res, err := reader.Audit()
	if err != nil || res.Mode != journal.ModeCheckpoint {
		t.Fatalf("read: %v %v", res.Mode, err)
	}
	next := writerMutate(t, repo, "after-policy-tail-note", nil)
	if !next.completed() || !next.fast() || next.declined() != "" {
		t.Fatalf("write: %+v %v %v", next.rep, next.err, next.stages)
	}
}

// prefixNoteStore is writerStore with a NOTE_SET, served by the writer
// route, on a ticket last posted before the writer checkpoint. It returns
// the ticket's path and the receipt that holds its pre-note afterimage.
func prefixNoteStore(t *testing.T) (*intent.Repository, string, uint64) {
	t.Helper()
	repo := writerStore(t, 70)
	entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("tickets: %v", err)
	}
	path := "intent/tickets/" + entries[0].Name()
	wc := readWriterCheckpoint(repo)
	prior, err := wc.LastPosted(path)
	if err != nil || prior == 0 || prior > wc.Seq.Uint64() {
		t.Fatalf("%s last posted at %d, checkpoint %s", path, prior, wc.Seq)
	}
	target := fixture.TicketID(strings.TrimSuffix(entries[0].Name(), ".json"))
	if run := stagedWrite(writerEnvelope(repo, writerNoteEnvelope("prefix-note", target, "0", "Prefix."), WallClock()), nil); !run.completed() || !run.fast() {
		t.Fatalf("note: %+v %v %v", run.rep, run.err, run.stages)
	}
	return repo, path, prior
}

// editReceipt rewrites receipt seq's parsed value and relinks head.json when
// it names that receipt.
func editReceipt(t *testing.T, repo *intent.Repository, seq uint64, edit func(wire.Value)) {
	t.Helper()
	name, _ := snapshot.ReceiptName(seq)
	path := filepath.Join(repo.StateDir, "receipts", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	edit(v)
	forged := wire.EncodeFile(v)
	historyWrite(t, path, forged)
	hp := filepath.Join(repo.StateDir, "head.json")
	hraw, err := os.ReadFile(hp)
	if err != nil {
		t.Fatal(err)
	}
	h, err := wire.Parse(hraw)
	if err != nil {
		t.Fatal(err)
	}
	if last, _ := h.Obj.Get("lastReceiptSha256"); last.Str == string(wire.Sum(raw)) {
		h.Obj.Set("lastReceiptSha256", wire.String(string(wire.Sum(forged))))
		historyWrite(t, hp, wire.EncodeFile(h))
	}
}

// postOf returns receipt v's post entry for path.
func postOf(t *testing.T, v wire.Value, path string) *wire.Object {
	t.Helper()
	post, _ := v.Obj.Get("post")
	for _, p := range post.Arr {
		if dest, _ := p.Obj.Get("path"); dest.Str == path {
			return p.Obj
		}
	}
	t.Fatalf("receipt posts no %s", path)
	return nil
}

// ON-V0-014 (proposed): a note pre-state the writer route cannot bind to its
// checkpoint declines the route with nothing published, and the complete
// route then decides exactly as it does with no checkpoint. A pre-state
// that binds but a note transition that does not replay is still refused.
func TestONV0014_UnboundNotePreStateDeclinesTheWriter(t *testing.T) {
	t.Parallel()
	retitle := func(post *wire.Object, digest bool) {
		rec, _ := post.Get("record")
		rec.Obj.Set("title", wire.String("Forged prefix"))
		if digest {
			post.Set("sha256", wire.String(string(wire.Sum(wire.EncodeFile(rec)))))
		}
	}
	cases := []struct {
		name     string
		change   func(t *testing.T, repo *intent.Repository, path string, prior uint64)
		declined string
		// refused is the complete route's refusal; empty when it commits.
		refused string
	}{
		{"prefix-post-record", func(t *testing.T, repo *intent.Repository, path string, prior uint64) {
			editReceipt(t, repo, prior, func(v wire.Value) { retitle(postOf(t, v, path), false) })
		}, "checkpoint pre-state does not bind", wire.CodeJournalForked},
		{"prefix-post-digest", func(t *testing.T, repo *intent.Repository, path string, prior uint64) {
			editReceipt(t, repo, prior, func(v wire.Value) { retitle(postOf(t, v, path), true) })
		}, "checkpoint pre-state does not bind", wire.CodeJournalForked},
		{"prefix-seq", func(t *testing.T, repo *intent.Repository, _ string, prior uint64) {
			editReceipt(t, repo, prior, func(v wire.Value) { v.Obj.Set("seq", wire.String(strconv.FormatUint(prior+1, 10))) })
		}, "checkpoint pre-state does not bind", wire.CodeJournalForked},
		{"prefix-prev", func(t *testing.T, repo *intent.Repository, _ string, prior uint64) {
			if prior < 2 {
				t.Fatalf("prefix receipt %d has no previous digest to forge", prior)
			}
			editReceipt(t, repo, prior, func(v wire.Value) { v.Obj.Set("prev", wire.String(string(wire.Sum([]byte("forged prev"))))) })
		}, "checkpoint pre-state does not bind", wire.CodeJournalForked},
		{"prefix-elided", func(t *testing.T, repo *intent.Repository, _ string, prior uint64) {
			editReceipt(t, repo, prior, func(v wire.Value) { v.Obj.Set("profile", wire.String(snapshot.ProfileElidedReceipt)) })
		}, "checkpoint pre-state does not bind", "oracle"},
		{"prefix-removed", func(t *testing.T, repo *intent.Repository, _ string, prior uint64) {
			name, _ := snapshot.ReceiptName(prior)
			if err := os.Remove(filepath.Join(repo.StateDir, "receipts", name)); err != nil {
				t.Fatal(err)
			}
			// The receipt inventory declines before any pre-state read.
		}, "receipt names differ from head", "oracle"},
		{"invalidated", func(t *testing.T, repo *intent.Repository, _ string, _ uint64) {
			invalidateWriterCheckpoint(context.Background(), repo)
		}, "absent, unusable or invalidated", ""},
		{"pre-ticket-digest", func(t *testing.T, repo *intent.Repository, path string, _ uint64) {
			editReceipt(t, repo, headSeqOf(t, repo), func(v wire.Value) {
				pre, _ := v.Obj.Get("pre")
				for _, p := range pre.Arr {
					if dest, _ := p.Obj.Get("path"); dest.Str == path {
						p.Obj.Set("sha256", wire.String(string(wire.Sum([]byte("forged")))))
					}
				}
			})
		}, "", wire.CodeJournalForked},
		{"note-post-differs", func(t *testing.T, repo *intent.Repository, path string, _ uint64) {
			editReceipt(t, repo, headSeqOf(t, repo), func(v wire.Value) { retitle(postOf(t, v, path), true) })
		}, "", wire.CodeJournalForked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, path, prior := prefixNoteStore(t)
			tc.change(t, repo, path, prior)
			before := mutationPublished(t, repo)
			write := writerCreate(repo, "after-"+tc.name, WallClock())
			run := stagedWrite(write, nil)
			if run.fast() || run.declined() == "" || !strings.Contains(run.declined(), tc.declined) {
				t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
			}
			t.Logf("declined: %s", run.declined())
			switch tc.refused {
			case "":
				if !run.completed() {
					t.Fatalf("complete route: %+v %v", run.rep, run.err)
				}
			case "oracle":
			default:
				if run.completed() || wire.CodeOf(run.err) != tc.refused {
					t.Fatalf("complete route: %+v %v, want %s", run.rep, run.err, tc.refused)
				}
			}
			if !run.completed() {
				if after := mutationPublished(t, repo); after != before {
					t.Fatalf("published %s, before %s", after, before)
				}
				sameDecision(t, run, writerOracle(t, repo, write))
			}
		})
	}
	// The writer checkpoint carries a note reference for a ticket last
	// posted before it; one that differs from the pre-ticket's declines.
	t.Run("writer-note-digest", func(t *testing.T) {
		t.Parallel()
		repo, _, _ := prefixNoteStore(t)
		if err := os.Remove(journal.WriterCheckpointPath(repo.StateDir)); err != nil {
			t.Fatal(err)
		}
		if run := writerMutate(t, repo, "note-rebase", nil); !run.completed() || run.fast() {
			t.Fatalf("rebase: %+v %v %v", run.rep, run.err, run.stages)
		}
		entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
		if err != nil {
			t.Fatal(err)
		}
		target := fixture.TicketID(strings.TrimSuffix(entries[0].Name(), ".json"))
		if run := stagedWrite(writerEnvelope(repo, writerNoteEnvelope("prefix-note-2", target, "1", "Again."), WallClock()), nil); !run.completed() || !run.fast() {
			t.Fatalf("second note: %+v %v %v", run.rep, run.err, run.stages)
		}
		// The complete route derived the checkpoint with a fresh base, which
		// carries the one note last: its digest ends where the header begins.
		// The delta binds the base by digest, so it is rebound and resealed.
		wcPath := journal.WriterCheckpointPath(repo.StateDir)
		var forged [sha256.Size]byte
		rewriteFile(t, journal.WriterBasePath(repo.StateDir), func(raw []byte) []byte {
			at := len(journal.ProfileWriterBase) + 1 + 3*8
			if binary.BigEndian.Uint64(raw[at:]) != 1 {
				t.Fatalf("writer checkpoint base does not carry exactly one note: %d", binary.BigEndian.Uint64(raw[at:]))
			}
			// Version 1 trails the notes with the checkpoint header
			// (length-prefixed), ten aggregates and the segment digests;
			// this store is unsegmented, so the trailer ends in a zero count.
			if binary.BigEndian.Uint64(raw[len(raw)-8:]) != 0 {
				t.Fatal("writer checkpoint base carries segments")
			}
			end := len(raw) - 8 - 10*8
			note := -1
			for n := 0; n <= 16<<10 && end-n-8 >= 0; n++ {
				if binary.BigEndian.Uint64(raw[end-n-8:]) == uint64(n) {
					note = end - n - 8
					break
				}
			}
			if note < 0 {
				t.Fatal("writer checkpoint base header not found")
			}
			raw[note-1] ^= 0xff
			forged = sha256.Sum256(raw)
			return raw
		})
		rewriteFile(t, wcPath, func(raw []byte) []byte {
			copy(raw[len(journal.ProfileWriterCheckpoint)+1:], forged[:])
			sum := sha256.Sum256(raw[:len(raw)-sha256.Size])
			copy(raw[len(raw)-sha256.Size:], sum[:])
			return raw
		})
		if readWriterCheckpoint(repo) == nil {
			t.Fatal("forged writer checkpoint is not usable")
		}
		run := writerMutate(t, repo, "after-forged-note", nil)
		if run.fast() || !strings.Contains(run.declined(), "note reference differs from the writer checkpoint") || !run.completed() {
			t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
		}
	})
}

// ON-V0-013/ON-V0-014 (proposed): the read and writer resumes bind the
// same prefix state and decline the same forged tails, so neither passes
// what the complete audit refuses. Each forgery is made after both
// checkpoints hold ticket A's note:
//   - a tail REFINE of A drops its note reference without a note event, with
//     its digest and projection rewritten, followed by a valid note on B;
//   - the prefix receipt that last posted B's pre-note afterimage carries
//     another sequence, its posted record and digest unchanged.
//
// The read resume falls back and the complete audit refuses JOURNAL_FORKED;
// the writer route declines and the complete route refuses the same way,
// publishing nothing.
func TestONV0014_ReadAndWriterResumesBindPrefixState(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		forge    func(t *testing.T, repo *intent.Repository, a, b string, cp *journal.Checkpoint)
		declined string
	}{
		{"reference-dropped-without-event", func(t *testing.T, repo *intent.Repository, a, b string, _ *journal.Checkpoint) {
			noteOn(t, repo, b, "tail-note-b")
			projection := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", strings.TrimPrefix(a, "intent/tickets/"))
			rec, err := ticket.Decode(mustReadFile(t, projection))
			if err != nil {
				t.Fatal(err)
			}
			refine := historyObject(
				"profile", wire.String(mutation.Profile),
				"requestId", wire.String("tail-refine-a"),
				"actor", historyObject("id", wire.String(historyActor.ID), "role", wire.String(historyActor.Role)),
				"queueId", wire.String(fixture.QueueID),
				"targetId", wire.String(rec.TicketID.Raw),
				"expectedRevision", wire.String(string(rec.Revision)),
				"operation", wire.String(mutation.OpRefine),
				"payload", historyObject("title", wire.String("Refined in the tail")),
				"issuedAt", wire.String("2026-09-07T12:00:00Z"),
			)
			if run := stagedWrite(writerEnvelope(repo, wire.EncodeFile(refine), WallClock()), nil); !run.completed() || !run.fast() {
				t.Fatalf("refine: %+v %v %v", run.rep, run.err, run.stages)
			}
			editReceipt(t, repo, headSeqOf(t, repo), func(v wire.Value) {
				post := postOf(t, v, a)
				record, _ := post.Get("record")
				if _, ok := record.Obj.Get("operatorNote"); !ok {
					t.Fatal("tail REFINE does not carry the note reference")
				}
				delete(record.Obj.Vals, "operatorNote")
				keys := record.Obj.Keys[:0]
				for _, k := range record.Obj.Keys {
					if k != "operatorNote" {
						keys = append(keys, k)
					}
				}
				record.Obj.Keys = keys
				raw := wire.EncodeFile(record)
				post.Set("sha256", wire.String(string(wire.Sum(raw))))
				historyWrite(t, projection, raw)
			})
		}, "note reference differs from the writer checkpoint"},
		{"prefix-receipt-sequence", func(t *testing.T, repo *intent.Repository, _, b string, cp *journal.Checkpoint) {
			noteOn(t, repo, b, "tail-note-b")
			var prior uint64
			for _, e := range cp.Entries {
				if e.Path == b {
					prior = e.Seq.Uint64()
				}
			}
			editReceipt(t, repo, prior, func(v wire.Value) {
				v.Obj.Set("seq", wire.String(strconv.FormatUint(prior+1, 10)))
			})
		}, "checkpoint pre-state does not bind"},
		{"prefix-receipt-prev", func(t *testing.T, repo *intent.Repository, _, b string, cp *journal.Checkpoint) {
			noteOn(t, repo, b, "tail-note-b")
			var prior uint64
			for _, e := range cp.Entries {
				if e.Path == b {
					prior = e.Seq.Uint64()
				}
			}
			if prior < 2 {
				t.Fatalf("prefix receipt %d has no previous digest to forge", prior)
			}
			editReceipt(t, repo, prior, func(v wire.Value) {
				v.Obj.Set("prev", wire.String(string(wire.Sum([]byte("forged prev")))))
			})
		}, "checkpoint pre-state does not bind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := writerStore(t, 70)
			entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
			if err != nil || len(entries) < 2 {
				t.Fatalf("tickets: %v", err)
			}
			a, b := "intent/tickets/"+entries[0].Name(), "intent/tickets/"+entries[1].Name()
			noteOn(t, repo, a, "prefix-note-a")
			// Re-base the writer checkpoint past A's note, and take the read
			// checkpoint from the same settled journal.
			if err := os.Remove(journal.WriterCheckpointPath(repo.StateDir)); err != nil {
				t.Fatal(err)
			}
			if run := writerMutate(t, repo, "note-rebase", nil); !run.completed() || run.fast() {
				t.Fatalf("rebase: %+v %v %v", run.rep, run.err, run.stages)
			}
			cp := readResume(t, repo, nil).Checkpoint()
			tc.forge(t, repo, a, b, cp)

			res, err := readResumeErr(t, repo, cp)
			if wire.CodeOf(err) != wire.CodeJournalForked || (res != nil && res.Mode == journal.ModeCheckpoint) {
				t.Fatalf("read from the checkpoint: %+v %v, want the complete audit's %s", res, err, wire.CodeJournalForked)
			}
			if _, full := readResumeErr(t, repo, nil); wire.CodeOf(full) != wire.CodeJournalForked {
				t.Fatalf("complete audit: %v", full)
			}

			before := mutationPublished(t, repo)
			write := writerCreate(repo, "after-"+tc.name, WallClock())
			run := stagedWrite(write, nil)
			if run.fast() || !strings.Contains(run.declined(), tc.declined) || run.completed() || wire.CodeOf(run.err) != wire.CodeJournalForked {
				t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
			}
			if after := mutationPublished(t, repo); after != before {
				t.Fatalf("published %s, before %s", after, before)
			}
			sameDecision(t, run, writerOracle(t, repo, write))
		})
	}
}

// noteOn sets a note on the ticket at path through the writer route.
func noteOn(t *testing.T, repo *intent.Repository, path, id string) {
	t.Helper()
	target := fixture.TicketID(strings.TrimSuffix(strings.TrimPrefix(path, "intent/tickets/"), ".json"))
	if run := stagedWrite(writerEnvelope(repo, writerNoteEnvelope(id, target, "0", "Note "+id+"."), WallClock()), nil); !run.completed() || !run.fast() || run.declined() != "" {
		t.Fatalf("%s: %+v %v %v", id, run.rep, run.err, run.stages)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// readResumeErr audits repo's journal from cp, or completely when cp is nil.
func readResumeErr(t *testing.T, repo *intent.Repository, cp *journal.Checkpoint) (*journal.Result, error) {
	t.Helper()
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	reader := journalReader(repo, head)
	reader.Checkpoint = cp
	return reader.Audit()
}

func readResume(t *testing.T, repo *intent.Repository, cp *journal.Checkpoint) *journal.Result {
	t.Helper()
	res, err := readResumeErr(t, repo, cp)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestONV0012_NoteInterleavedHistoryProfile is an opt-in profile, not a
// timing assertion: on a settled history it times a NOTE_SET on a ticket
// last posted in the prefix, the CREATE that follows it, and a read from the
// retained read checkpoint, and records each write's route. Medians only;
// host load is captured separately.
func TestONV0012_NoteInterleavedHistoryProfile(t *testing.T) {
	output := os.Getenv("CORVINT_TASKS_NOTE_PROFILE")
	if output == "" {
		t.Skip("set CORVINT_TASKS_NOTE_PROFILE to a report path")
	}
	sizes := []int{2000}
	if raw := os.Getenv("CORVINT_TASKS_HISTORY_SIZES"); raw != "" {
		sizes = nil
		for _, f := range strings.Split(raw, ",") {
			n, err := strconv.Atoi(f)
			if err != nil || n <= historyTickets+1 {
				t.Fatalf("size %q", f)
			}
			sizes = append(sizes, n)
		}
	}
	const reps = 3
	type row struct {
		Receipts  int
		Phases    map[string]float64
		CPUPhases map[string]float64
		Declined  []string
		ReadModes []string
	}
	var rows []row
	for _, n := range sizes {
		repo := historyStore(t, n)
		historyMutate(t, repo, "note-profile-seed")
		entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
		if err != nil || len(entries) == 0 {
			t.Fatalf("tickets: %v", err)
		}
		target := fixture.TicketID(strings.TrimSuffix(entries[0].Name(), ".json"))
		r := row{Receipts: n, Phases: map[string]float64{}, CPUPhases: map[string]float64{}, Declined: []string{}, ReadModes: []string{}}
		samples, cpuSamples := map[string][]float64{}, map[string][]float64{}
		timed := func(name string, write func(context.Context) (*Report, error)) {
			cpu, cpuOK := processCPU()
			start := time.Now()
			run := stagedWrite(write, nil)
			samples[name] = append(samples[name], ms(time.Since(start)))
			if end, ok := processCPU(); cpuOK && ok {
				cpuSamples[name] = append(cpuSamples[name], ms(end-cpu))
			}
			if !run.completed() {
				t.Fatalf("%d %s: %+v %v %v", n, name, run.rep, run.err, run.stages)
			}
			if why := run.declined(); why != "" {
				r.Declined = append(r.Declined, name+": "+why)
			}
		}
		for rep := 0; rep < reps; rep++ {
			id := fmt.Sprintf("note-profile-%d-%d", n, rep)
			timed("note.set", writerEnvelope(repo, writerNoteEnvelope(id+"-note", target, strconv.Itoa(rep), fmt.Sprintf("Note %d.", rep)), WallClock()))
			cpRaw, err := os.ReadFile(journal.CheckpointPath(repo.StateDir))
			if err != nil {
				t.Fatal(err)
			}
			cp, err := journal.DecodeCheckpoint(cpRaw)
			if err != nil {
				t.Fatal(err)
			}
			head, err := writerGuards(repo, "MUTATE")
			if err != nil {
				t.Fatal(err)
			}
			reader := journalReader(repo, head)
			reader.Checkpoint = cp
			cpu, cpuOK := processCPU()
			start := time.Now()
			res, err := reader.Audit()
			if err != nil {
				t.Fatalf("%d read: %v", n, err)
			}
			samples["read.afterNote"] = append(samples["read.afterNote"], ms(time.Since(start)))
			if end, ok := processCPU(); cpuOK && ok {
				cpuSamples["read.afterNote"] = append(cpuSamples["read.afterNote"], ms(end-cpu))
			}
			r.ReadModes = append(r.ReadModes, string(res.Mode))
			timed("mutate.afterNote", writerCreate(repo, id+"-create", WallClock()))
		}
		for name, s := range samples {
			sort.Float64s(s)
			r.Phases[name] = s[len(s)/2]
		}
		for name, s := range cpuSamples {
			sort.Float64s(s)
			r.CPUPhases[name] = s[len(s)/2]
		}
		rows = append(rows, r)
		t.Logf("receipts=%d %v cpu %v declined %v reads %v", n, r.Phases, r.CPUPhases, r.Declined, r.ReadModes)
	}
	report := map[string]any{"ticket": "V1-1650", "requirement": "ON-V0-012", "fixture": fmt.Sprintf("historyStore: %d created tickets, one request plus one ~%d-byte ticket afterimage per receipt; each repetition is NOTE_SET on the first ticket (last posted in the prefix), a read from the retained read checkpoint, then a CREATE", historyTickets, historyBodySize), "repetitions": reps, "statistic": "median milliseconds: Phases wall time, CPUPhases process user plus system CPU time", "cpuCount": runtime.NumCPU(), "goVersion": runtime.Version(), "rows": rows, "hostLoad": hostLoad()}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(output, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
