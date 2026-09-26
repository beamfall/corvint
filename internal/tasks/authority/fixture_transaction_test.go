//go:build darwin || linux

package authority

// Disposable, sole-writer integration only (owner decision 0002). None of these
// helpers accepts a repository from a caller or establishes an actor issuer.
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const ftTicket = "intent/tickets/AT-01.json"

type ftHarness struct {
	t            *testing.T
	r            *fixture.Repo
	s            *fixtureSession
	root, common os.FileInfo // ownership survives session release; paths do not grant it
	era          wire.Digest // issued descriptor identity, never cached recovery payload
	selected     bool
	bases        map[*transaction.Plan]*transaction.Inventory // immutable copied model-time inventory
}

func ftObject(kv ...any) wire.Value {
	o := wire.NewObject()
	for i := 0; i < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}

func ftNew(t *testing.T, seeded bool) *ftHarness {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.Root, "source.txt"), []byte("untrusted source; never a command or authority\n"))
	// A disposable primary Git layout, with no subprocess or ambient Git config.
	fixture.Write(t, filepath.Join(r.CommonDir, "HEAD"), []byte("ref: refs/heads/main\n"))
	for _, d := range []string{"objects", "refs/heads"} {
		fixtureMust(t, os.MkdirAll(filepath.Join(r.CommonDir, d), 0700))
	}
	if seeded {
		ftSeed(t, r)
	}
	h := &ftHarness{t: t, r: r}
	var err error
	h.root, err = os.Lstat(r.Root)
	fixtureMust(t, err)
	h.common, err = os.Lstat(r.CommonDir)
	fixtureMust(t, err)
	fixtureMust(t, h.open())
	t.Cleanup(func() { h.s.before = nil; fixtureMust(t, h.s.close()) })
	return h
}

func (h *ftHarness) open() error {
	r, err := os.Lstat(h.r.Root)
	if err != nil || !os.SameFile(r, h.root) {
		return fixtureRefused
	}
	c, err := os.Lstat(h.r.CommonDir)
	if err != nil || !os.SameFile(c, h.common) {
		return fixtureRefused
	}
	repo, err := intent.Resolve(h.r.Root)
	if err != nil {
		return err
	}
	l, err := AcquireLock(context.Background(), repo, LockOptions{})
	if err != nil {
		return err
	}
	s, err := newFixtureSession(repo, l)
	if err != nil {
		return errors.Join(err, l.Close())
	}
	h.s = s
	return nil
}

func (h *ftHarness) restart() {
	h.t.Helper()
	h.s.before = nil
	h.bases = nil // restart discards all original Plans and model-time observations
	fixtureMust(h.t, h.s.close())
	fixtureMust(h.t, h.open())
	if len(h.s.stages) != 0 {
		h.t.Fatal("new session inherited tokens")
	}
}

// The ticket-bearing seed is construction, not an ADD executor. Its selected C
// deliberately requires a real blob, so a missing blob cannot fall back to D.
func ftSeed(t *testing.T, r *fixture.Repo) {
	fixture.WriteIntent(t, r)
	fixture.WriteState(t, r)
	c := fixture.Ticket("AT-01")
	raw := c.Encode()
	seq := wire.Size("2")
	out := mutation.Outcome{RequestID: "seed", Outcome: mutation.OutcomeCompleted, ReceiptSeq: &seq, ResultingRevision: &c.Revision, ResultingAcceptanceRevision: &c.AcceptanceRevision, Codes: []string{}}
	req := wire.EncodeFile(ftObject("requestId", wire.String("seed"), "seq", wire.String("2"), "mutationSha256", wire.String(string(wire.Sum([]byte("fixture seed")))), "outcome", out.Value()))
	rp, err := snapshot.RequestPath("seed")
	fixtureMust(t, err)
	fixture.CommitPosts(t, r, "MUTATION", "seed", map[string][]byte{ftTicket: raw, rp: req})
	name, _ := snapshot.ReceiptName(2)
	rcpath := filepath.Join(r.StateDir, "receipts", name)
	rcraw, err := os.ReadFile(rcpath)
	fixtureMust(t, err)
	v, err := wire.Parse(rcraw)
	fixtureMust(t, err)
	v.Obj.Set("ticketId", wire.String(c.TicketID.Raw))
	posts, _ := v.Obj.Get("post")
	for _, p := range posts.Arr {
		pv, _ := p.Obj.Get("path")
		if pv.Str == ftTicket {
			p.Obj.Set("record", wire.Null())
			p.Obj.Set("blobSha256", wire.String(string(wire.Sum(raw))))
		}
	}
	fixture.Write(t, filepath.Join(r.StateDir, "evidence", string(wire.Sum(raw))), raw)
	rcraw = wire.EncodeFile(v)
	fixture.Write(t, rcpath, rcraw)
	hraw, err := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
	fixtureMust(t, err)
	head, err := snapshot.DecodeHead(hraw)
	fixtureMust(t, err)
	fixture.Write(t, filepath.Join(r.StateDir, "head.json"), wire.EncodeFile(fixture.HeadValue(r.Root, 2, wire.Sum(rcraw), 0, head.InitSha256)))
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	beforeState, beforeIntent := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	audit, err := (journal.Reader{Source: journal.Native{StateDir: r.StateDir, PrimaryWorktree: r.Root}, QueueID: q, PrimaryWorktree: r.Root}).Audit(ftTicket)
	fixtureMust(t, err)
	if !bytes.Equal(audit.Records[ftTicket].Raw, raw) {
		t.Fatal("seed canonical record")
	}
	repo, err := intent.Resolve(r.Root)
	fixtureMust(t, err)
	var stream bytes.Buffer
	_, err = archive.Export(archive.ExportOptions{Repo: repo, Staging: fixture.TempDirOutside(t), Stdout: &stream})
	fixtureMust(t, err)
	_, err = archive.Verify(bytes.NewReader(stream.Bytes()))
	fixtureMust(t, err)
	fixture.AssertUntouched(t, r, beforeState, beforeIntent, "seed audit/export/verify")
}

func ftRequest(op, id string) transaction.Request {
	return transaction.Request{Operation: op, QueueID: fixture.QueueID, RequestID: id, Actor: mutation.Binding{ID: fixture.Actor, Role: "OWNER"}}
}

func (h *ftHarness) request(op, id string) transaction.Request {
	r := ftRequest(op, id)
	if op == transaction.Init {
		r.PrimaryWorktree = h.r.Root
		r.Queue = fixture.QueueBytes()
		r.Policy = fixture.PolicyBytes()
	}
	if op == transaction.KeepJournal || op == transaction.AdoptFile {
		h.selected = true
		o, err := h.observe(false)
		fixtureMust(h.t, err)
		r.TargetID = fixture.TicketID("AT-01")
		r.File = bytes.Clone(o.disk.raw[ftTicket])
		if op == transaction.KeepJournal {
			r.CanonicalSha256 = wire.Sum(o.latest[ftTicket])
		}
	}
	return r
}

func (h *ftHarness) model(r transaction.Request) (transaction.Result, error) {
	o, err := h.observe(false)
	if err != nil {
		return transaction.Result{}, err
	}
	if len(o.disk.stage) != 0 {
		return transaction.Result{}, fixtureRefused
	}
	inv, err := o.disk.inventory()
	if err != nil {
		return transaction.Result{}, err
	}
	in := transaction.Input{Inventory: inv, Head: o.disk.raw["head.json"], Queue: o.disk.raw["intent/queue.json"], Policy: o.disk.raw["intent/policy.json"], Barrier: o.disk.raw["barrier.json"], Reservations: o.disk.raw["reservations.json"], Branch: o.disk.branch, Premise: transaction.FixtureNoRuntime, RecordedAt: fixture.Timestamp, Replay: transaction.ReplayObservation{State: "ABSENT"}}
	if c := o.latest[ftTicket]; c != nil {
		in.CanonicalTickets = [][]byte{c}
	}
	rp, err := snapshot.RequestPath(r.RequestID)
	if err != nil {
		return transaction.Result{}, err
	}
	if raw := o.latest[rp]; raw != nil {
		in.Replay = transaction.ReplayObservation{State: "FOUND", Record: bytes.Clone(raw)}
	}
	result := transaction.Model(r, in)
	if result.Kind == "Transaction" {
		base, e := transaction.NewInventory(inv.Files(), inv.Directories())
		if e != nil {
			return transaction.Result{}, e
		}
		if h.bases == nil {
			h.bases = map[*transaction.Plan]*transaction.Inventory{}
		}
		h.bases[result.Plan] = base
	}
	return result, nil
}

func (h *ftHarness) plan(r transaction.Request) *transaction.Plan {
	h.t.Helper()
	result, err := h.model(r)
	fixtureMust(h.t, err)
	if result.Kind != "Transaction" {
		h.t.Fatalf("%s: %s %s", r.Operation, result.Kind, result.Detail)
	}
	_, err = transaction.CheckCapacity(result.Plan)
	fixtureMust(h.t, err)
	return result.Plan
}

// Controlled edits are only to this harness's one selected projection. Bytes
// are offered data, never a filename, branch, actor grant, argv or environment.
func (h *ftHarness) edit(raw []byte) {
	h.t.Helper()
	if len(raw) > wire.MaxTicketFileBytes {
		h.t.Fatal("fixture edit bound")
	}
	h.selected = true
	fixtureMust(h.t, os.WriteFile(filepath.Join(h.r.IntentDir, "tickets/AT-01.json"), raw, 0644))
}

func ftTarget(p string) (fixtureTarget, error) {
	t := fixtureTarget{name: path.Base(p)}
	switch {
	case p == "head.json":
		t.role = fixtureHead
	case p == "VERSION":
		t.role = fixtureVersion
	case p == "barrier.json":
		t.role = fixtureBarrier
	case p == "reservations.json":
		t.role = fixtureReservations
	case p == "intent/queue.json":
		t.role = fixtureQueue
	case p == "intent/policy.json":
		t.role = fixturePolicy
	case p == ftTicket:
		t.role = fixtureTicket
	case strings.HasPrefix(p, "receipts/"):
		t.role = fixtureReceipt
	case strings.HasPrefix(p, "requests/"):
		t.role = fixtureRequest
	case strings.HasPrefix(p, "evidence/"):
		t.role = fixtureEvidence
	case strings.HasPrefix(p, "pinned/"):
		t.role = fixturePin
	default:
		return t, fixtureRefused
	}
	key, name, _, err := t.location()
	want := key + "/" + name
	if key == "state" {
		want = name
	}
	if key == "tickets" {
		want = "intent/tickets/" + name
	}
	if want != p {
		return t, fixtureRefused
	}
	return t, err
}

func (h *ftHarness) directory(key string) error {
	if h.s.parents[key] != nil {
		return nil
	}
	p, _, ok := fixtureDirectory(key)
	if !ok {
		return fixtureRefused
	}
	if err := h.directory(p); err != nil {
		return err
	}
	return h.s.mkdir(key)
}

// Ticket replacement deliberately lives in the owned harness, not on the
// production role surface. Same checks/lifetime as private replace; no CAS claim.
func (h *ftHarness) replaceTicket(stage *fixtureStage, pre *wire.Digest, op string) error {
	if !h.selected || pre == nil || (op != transaction.KeepJournal && op != transaction.AdoptFile) {
		return fixtureRefused
	}
	s := h.s
	return s.operation(func() (err error) {
		branch, err := ftRead(s, "common", "HEAD", 256)
		if err != nil {
			return err
		}
		if string(branch.raw) != "ref: refs/heads/main\n" {
			return fixtureRefused
		}
		source, key, name, err := s.source(stage, fixtureTarget{fixtureTicket, "AT-01.json"})
		if err != nil {
			return err
		}
		performed := false
		defer func() {
			err = errors.Join(err, s.closeFile(source))
			if performed {
				err = fixtureAfter("ticket rename", err)
			}
		}()
		dest, info, digest, err := s.readCurrent(key, name, wire.MaxTicketFileBytes)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, s.closeFile(dest)) }()
		if digest != *pre {
			return fixtureRefused
		}
		if err = s.boundary("rename"); err != nil {
			return err
		}
		if err = s.checkStageFile(stage, source); err != nil {
			return err
		}
		if err = s.checkCurrent(key, name, dest, info, digest); err != nil {
			return err
		}
		if err = s.boundary("native:rename"); err != nil {
			return err
		}
		if err = fixtureRename(s.parents["staging"].file, string(stage.slot), s.parents[key].file, name, source, dest); err != nil {
			return err
		}
		performed = true
		delete(s.stages, stage)
		check := func() error { return s.checkCurrent(key, name, source, stage.info, stage.digest) }
		return errors.Join(s.syncParent(key, check), s.syncParent("staging", check, func() error { return s.absent("staging", string(stage.slot)) }))
	})
}

func (h *ftHarness) confirm(p string, raw []byte) error {
	target, err := ftTarget(p)
	if err != nil {
		return err
	}
	key, name, bound, _ := target.location()
	return h.s.operation(func() (err error) {
		f, info, digest, err := h.s.readCurrent(key, name, bound)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, h.s.closeFile(f)) }()
		if digest != wire.Sum(raw) || info.Size() != int64(len(raw)) {
			return fixtureRefused
		}
		if err = h.s.boundary("already-file-sync"); err != nil {
			return err
		}
		if err = syncFile(f); err != nil {
			return err
		}
		return h.s.syncParent(key, func() error { return h.s.checkCurrent(key, name, f, info, digest) })
	})
}

type ftStep struct {
	name string
	run  func() error
}

// One finite publication schedule for all five operations. A test callback can
// stop after any completed step; such a stop is not a killed process.
func (h *ftHarness) publish(p *transaction.Plan, after func(string) error) (committed bool, err error) {
	o, err := h.observe(false)
	if err != nil {
		return false, err
	}
	if len(o.disk.stage) != 0 || h.era != "" {
		return false, fixtureRefused
	}
	cap, err := transaction.CheckCapacity(p)
	if err != nil {
		return false, err
	}
	d, err := snapshot.DecodeStageDescriptor(p.Descriptor())
	if err != nil {
		return false, err
	}
	// Compare the original immutable observed inventory, never a later reconstruction.
	base := h.bases[p]
	current, e := o.disk.inventory()
	if e != nil {
		return false, e
	}
	if base == nil || !reflect.DeepEqual(base.Files(), current.Files()) || !reflect.DeepEqual(base.Directories(), current.Directories()) {
		return false, fixtureRefused
	}
	// A plan from an earlier disk base is not authority to start a new era.
	if d.Base == nil {
		if o.disk.raw["head.json"] != nil {
			return false, fixtureRefused
		}
	} else {
		base, e := snapshot.DecodeHead(o.disk.raw["head.json"])
		if e != nil || base.LastSeq != d.Base.LastSeq || *base.LastReceiptSha256 != d.Base.LastReceiptSha256 {
			return false, fixtureRefused
		}
	}
	for _, pre := range mustReceipt(h.t, p.Receipt()).Pre {
		if !ftEqualDigest(pre.Sha256, ftDigest(o.disk.raw[pre.Path])) {
			return false, fixtureRefused
		}
	}
	if (d.Operation == transaction.Init || d.Operation == transaction.KeepJournal || d.Operation == transaction.AdoptFile) && o.disk.branch != "main" {
		return false, fixtureRefused
	}
	h.bases = nil // recovery retains no original Plan through the harness
	h.era = wire.Sum(p.Descriptor())
	stages := map[string]*fixtureStage{}
	var descriptor *fixtureStage
	steps := []ftStep{
		{"directories", func() error { return h.directory("staging") }},
		{"descriptor-temp", func() error {
			var e error
			descriptor, e = h.s.prepare("active.json.tmp", fixtureDescriptor, p.Descriptor())
			return e
		}},
		{"descriptor-link", func() error { return h.s.link(descriptor, fixtureTarget{fixtureDescriptor, "active.json"}) }},
		{"descriptor-temp-clean", func() error { return h.s.removeStage("active.json.tmp") }},
	}
	for _, a := range p.Artifacts() {
		steps = append(steps, ftStep{"slot:" + a.Slot, func() error {
			target, e := ftTarget(a.Target)
			if e != nil {
				return e
			}
			stages[a.Slot], e = h.s.prepare(fixtureSlot(a.Slot), target.role, a.Data)
			return e
		}})
	}
	for _, a := range p.Artifacts() {
		if !strings.HasPrefix(a.Target, "evidence/") {
			continue
		}
		steps = append(steps, ftStep{"durable-link:" + a.Target, func() error { return h.apply(a, stages[a.Slot], nil, d.Operation) }})
	}
	for _, a := range p.Artifacts() {
		if a.Role != "RECEIPT" {
			continue
		}
		steps = append(steps, ftStep{"durable-receipt-link", func() error {
			// Re-read base and evidence immediately before the commit producer.
			current, e := h.observe(false)
			if e != nil {
				return e
			}
			if !ftSameBase(o.disk, current.disk) {
				return fixtureRefused
			}
			for _, post := range mustReceipt(h.t, p.Receipt()).Post {
				if post.BlobSha256 != nil {
					if e = h.confirm("evidence/"+string(*post.BlobSha256), current.disk.raw["evidence/"+string(*post.BlobSha256)]); e != nil {
						return e
					}
				}
			}
			target, e := ftTarget(a.Target)
			if e != nil {
				return e
			}
			key, _, _, _ := target.location()
			if e = h.directory(key); e != nil {
				return e
			}
			e = h.s.link(stages[a.Slot], target)
			var effect *fixtureEffectError
			committed = e == nil || errors.As(e, &effect)
			return e
		}})
	}
	rc := mustReceipt(h.t, p.Receipt())
	pres := map[string]*wire.Digest{}
	for _, pre := range rc.Pre {
		pres[pre.Path] = pre.Sha256
	}
	posts := []transaction.Artifact{}
	for _, a := range p.Artifacts() {
		if a.Role == "POST" && !strings.HasPrefix(a.Target, "evidence/") {
			posts = append(posts, a)
		}
	}
	sort.Slice(posts, func(i, j int) bool {
		gi := len(posts[i].Data) >= len(o.disk.raw[posts[i].Target])
		gj := len(posts[j].Data) >= len(o.disk.raw[posts[j].Target])
		if gi != gj {
			return gi
		}
		return posts[i].Target < posts[j].Target
	})
	for _, a := range posts {
		steps = append(steps, ftStep{"durable-post:" + a.Target, func() error { return h.apply(a, stages[a.Slot], pres[a.Target], d.Operation) }})
	}
	for _, post := range rc.Post {
		if post.Sha256 == nil {
			steps = append(steps, ftStep{"durable-barrier-unlink", func() error {
				return h.s.removeBarrier(fixtureTarget{fixtureBarrier, "barrier.json"}, *pres[post.Path])
			}})
		}
	}
	for _, a := range p.Artifacts() {
		if a.Role == "HEAD" {
			steps = append(steps, ftStep{"durable-head", func() error { return h.apply(a, stages[a.Slot], ftDigest(o.disk.raw["head.json"]), d.Operation) }})
		}
	}
	steps = append(steps, ftStep{"durable-staging-cleanup", func() error {
		_, e := h.observe(true)
		if e != nil {
			return e
		}
		return h.cleanup(false)
	}})
	// Each prospective step is admitted only under its accepted prefix envelope.
	bound := cap.Prefixes[0].Cost
	for _, step := range steps {
		if err = h.costWithin(bound, p.Head(), false); err != nil {
			return committed, err
		}
		for _, prefix := range cap.Prefixes {
			if prefix.Step == step.name {
				bound = prefix.Cost
			}
		}
		if err = step.run(); err != nil {
			return committed, err
		}
		if err = h.costWithin(bound, p.Head(), step.name == "durable-staging-cleanup"); err != nil {
			return committed, err
		}
		if after != nil {
			if err = after(step.name); err != nil {
				return committed, err
			}
		}
	}
	finalDisk, err := h.capture()
	if err != nil {
		return committed, err
	}
	finalInventory, err := finalDisk.inventory()
	if err != nil {
		return committed, err
	}
	if !reflect.DeepEqual(finalInventory.Files(), cap.Final.Files()) || !reflect.DeepEqual(finalInventory.Directories(), cap.Final.Directories()) {
		return committed, fmt.Errorf("actual final inventory differs from accepted model")
	}
	return committed, nil
}

func mustReceipt(t *testing.T, raw []byte) *snapshot.Receipt {
	t.Helper()
	r, e := snapshot.DecodeReceipt(raw)
	fixtureMust(t, e)
	return r
}
func ftDigest(raw []byte) *wire.Digest {
	if raw == nil {
		return nil
	}
	d := wire.Sum(raw)
	return &d
}

func ftSameBase(a, b *ftDisk) bool {
	for p, raw := range a.raw {
		if !bytes.Equal(raw, b.raw[p]) || (raw == nil) != (b.raw[p] == nil) {
			return false
		}
	}
	for p := range b.raw {
		if _, ok := a.raw[p]; !ok && !strings.HasPrefix(p, "evidence/") {
			return false
		}
	}
	return true
}

func (h *ftHarness) apply(a transaction.Artifact, stage *fixtureStage, pre *wire.Digest, op string) error {
	target, err := ftTarget(a.Target)
	if err != nil {
		return err
	}
	key, name, bound, _ := target.location()
	if err = h.directory(key); err != nil {
		return err
	}
	var current []byte
	err = h.s.operation(func() error { f, e := ftRead(h.s, key, name, bound); current = f.raw; return e })
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && bytes.Equal(current, a.Data) {
		return h.confirm(a.Target, a.Data)
	}
	if target.role == fixtureTicket {
		return h.replaceTicket(stage, pre, op)
	}
	if target.role == fixtureHead || target.role == fixtureBarrier || target.role == fixtureReservations {
		return h.s.replace(stage, target, pre)
	}
	return h.s.link(stage, target)
}

func TestTMV0009_AS11_FixtureFoundationFiveOperations(t *testing.T) {
	for _, op := range []string{transaction.Init, transaction.Pause, transaction.Unpause, transaction.KeepJournal, transaction.AdoptFile} {
		t.Run(op, func(t *testing.T) {
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
				c.Title = "Owned edit"
				h.edit(c.Encode())
			}
			sourceBefore := fixture.TreeSnapshot(t, filepath.Join(h.r.Root, "source.txt"))
			r := h.request(op, "flow")
			p := h.plan(r)
			committed, err := h.publish(p, nil)
			fixtureMust(t, err)
			if !committed {
				t.Fatal("receipt not committed")
			}
			if !fixture.SameTree(sourceBefore, fixture.TreeSnapshot(t, filepath.Join(h.r.Root, "source.txt"))) {
				t.Fatal("source bytes/mode/mtime changed")
			}
			o, err := h.observe(false)
			fixtureMust(t, err)
			if len(o.disk.stage) != 0 || !bytes.Equal(o.disk.raw["head.json"], p.Head()) {
				t.Fatal("final projections/cleanup")
			}
			if op == transaction.Init && o.latest[ftTicket] != nil {
				t.Fatal("INIT minted a ticket")
			}
			if op == transaction.KeepJournal {
				if raw, ok := o.disk.raw["evidence/"+string(wire.Sum([]byte{}))]; !ok || len(raw) != 0 {
					t.Fatal("present-empty D lost")
				}
			}
			if op == transaction.AdoptFile {
				c, e := ticket.Decode(o.latest[ftTicket])
				fixtureMust(t, e)
				if c.Revision != "2" || c.AcceptanceRevision != "1" || *c.PreviousRecordSha256 != fixture.Ticket("AT-01").FileDigest() {
					t.Fatal("ADOPT reducer chain")
				}
			}
			before := fixture.TreeSnapshot(t, h.r.Root)
			replay, err := h.model(r)
			fixtureMust(t, err)
			if replay.Kind != "Replay" || !replay.Outcome.Replayed {
				t.Fatalf("replay: %+v", replay)
			}
			r.Actor.ID = "other"
			conflict, err := h.model(r)
			fixtureMust(t, err)
			if conflict.Kind != "Refused" || conflict.Outcome.Outcome != mutation.OutcomeRequestIDConflict {
				t.Fatal("request conflict")
			}
			if !fixture.SameTree(before, fixture.TreeSnapshot(t, h.r.Root)) {
				t.Fatal("replay/conflict wrote artifacts")
			}
		})
	}
}

// Capacity uses real retained bytes and the accepted archive encoder, never an
// export overlay. Transient manifests are sizing evidence, not native archives.
func (h *ftHarness) costWithin(bound transaction.Cost, headRaw []byte, final bool) error {
	disk, err := h.capture()
	if err != nil {
		return err
	}
	got, err := disk.cost(headRaw)
	if err != nil {
		return err
	}
	if final {
		if got != bound {
			return fmt.Errorf("final capacity differs: got %+v bound %+v", got, bound)
		}
		return nil
	}
	gv, bv := reflect.ValueOf(got), reflect.ValueOf(bound)
	for i := 0; i < gv.NumField(); i++ {
		if gv.Field(i).Uint() > bv.Field(i).Uint() {
			return fmt.Errorf("model dependency: %s live %d exceeds accepted %d", gv.Type().Field(i).Name, gv.Field(i).Uint(), bv.Field(i).Uint())
		}
	}
	return nil
}
