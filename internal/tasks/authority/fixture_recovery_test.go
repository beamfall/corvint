//go:build darwin || linux

package authority

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Finite fixture bounds, narrower than production: twelve linked receipts,
// 128 entries per directory, 4 MiB total captured bytes. No general observer API.
type ftFile struct {
	raw  []byte
	info os.FileInfo
}
type ftDisk struct {
	raw    map[string][]byte
	stage  map[string]ftFile
	files  map[string]os.FileInfo
	dirs   []string
	branch string
}

func ftRead(s *fixtureSession, key, name string, bound int) (out ftFile, err error) {
	f, info, digest, err := s.readCurrent(key, name, bound)
	if err != nil {
		return out, err
	}
	defer func() { err = errors.Join(err, s.closeFile(f)) }()
	out.raw, err = io.ReadAll(io.NewSectionReader(f, 0, int64(bound)+1))
	if err != nil {
		return out, err
	}
	if len(out.raw) != int(info.Size()) || wire.Sum(out.raw) != digest {
		return out, fixtureRefused
	}
	out.info = info
	return out, s.checkCurrent(key, name, f, info, digest)
}

func ftNames(s *fixtureSession, key string) (names []string, err error) {
	f, err := safeopen.InRoot(s.parents[key].root, ".", os.O_RDONLY, 0, true)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.closeFile(f)) }()
	names, err = f.Readdirnames(129)
	if len(names) > 128 {
		return nil, fixtureRefused
	}
	if err != nil && err != io.EOF {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

func (h *ftHarness) scan() (*ftDisk, error) {
	d := &ftDisk{raw: map[string][]byte{}, stage: map[string]ftFile{}, files: map[string]os.FileInfo{}}
	branch, err := ftRead(h.s, "common", "HEAD", 256)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(branch.raw, []byte("ref: refs/heads/")) || !bytes.HasSuffix(branch.raw, []byte("\n")) {
		return nil, fixtureRefused
	}
	d.branch = strings.TrimSuffix(strings.TrimPrefix(string(branch.raw), "ref: refs/heads/"), "\n")
	if _, err = wire.ParseLabel("branch", d.branch); err != nil {
		return nil, err
	}
	for _, name := range []string{"worktrees", "commondir"} {
		if _, err := h.s.parents["common"].root.Lstat(name); !os.IsNotExist(err) {
			return nil, fixtureRefused
		}
	}
	d.files["git:HEAD"] = branch.info
	var total int
	var scan func(string) error
	scan = func(key string) error {
		if h.s.parents[key] == nil {
			return nil
		}
		names, err := ftNames(h.s, key)
		if err != nil {
			return err
		}
		for _, name := range names {
			info, err := h.s.parents[key].root.Lstat(name)
			if err != nil {
				return err
			}
			p := key + "/" + name
			if key == "state" {
				p = name
			}
			if key == "tickets" {
				p = "intent/tickets/" + name
			}
			if info.IsDir() {
				child := p
				if p == "intent/tickets" {
					child = "tickets"
				}
				parent, _, ok := fixtureDirectory(child)
				if !ok || parent != key || h.s.parents[child] == nil {
					return fixtureRefused
				}
				if key != "intent" {
					d.dirs = append(d.dirs, p)
				}
				d.files["dir:"+p] = info
				if err = scan(child); err != nil {
					return err
				}
				continue
			}
			if !info.Mode().IsRegular() {
				return fixtureRefused
			}
			bound := wire.MaxReceiptFileBytes
			if key != "staging" {
				target, e := ftTarget(p)
				if e != nil {
					return e
				}
				_, _, bound, _ = target.location()
				// This helper is intentionally not a general large-blob reader.
				if bound > wire.MaxReceiptFileBytes {
					bound = wire.MaxReceiptFileBytes
				}
			} else if name == "active.json" || name == "active.json.tmp" {
				bound = snapshot.MaxStageDescriptorBytes
			}
			if info.Size() > int64(4*wire.MiB-total) {
				return fixtureRefused
			}
			f, e := ftRead(h.s, key, name, bound)
			if e != nil {
				return e
			}
			total += len(f.raw)
			d.files[p] = f.info
			if key == "staging" {
				d.stage[name] = f
			} else {
				d.raw[p] = f.raw
			}
		}
		return nil
	}
	if err = scan("state"); err != nil {
		return nil, err
	}
	if err = scan("intent"); err != nil {
		return nil, err
	}
	sort.Strings(d.dirs)
	return d, nil
}

func ftDiskEqual(a, b *ftDisk) bool {
	if a.branch != b.branch || len(a.files) != len(b.files) {
		return false
	}
	for p, x := range a.files {
		y := b.files[p]
		if y == nil || !os.SameFile(x, y) || x.Mode() != y.Mode() || x.Size() != y.Size() || x.ModTime() != y.ModTime() {
			return false
		}
	}
	for p, raw := range a.raw {
		if !bytes.Equal(raw, b.raw[p]) || (raw == nil) != (b.raw[p] == nil) {
			return false
		}
	}
	for p, f := range a.stage {
		if !bytes.Equal(f.raw, b.stage[p].raw) {
			return false
		}
	}
	return true
}

func (h *ftHarness) capture() (out *ftDisk, err error) {
	err = h.s.operation(func() error {
		a, e := h.scan()
		if e != nil {
			return e
		}
		b, e := h.scan()
		if e != nil {
			return e
		}
		if !ftDiskEqual(a, b) {
			return fixtureRefused
		}
		out = a
		return nil
	})
	return out, err
}

func (d *ftDisk) inventory() (*transaction.Inventory, error) {
	files := []archive.FileEntry{}
	for p, raw := range d.raw {
		files = append(files, archive.FileEntry{Path: p, Sha256: wire.Sum(raw), Bytes: wire.SizeOf(uint64(len(raw)))})
	}
	return transaction.NewInventory(files, d.dirs)
}

func (d *ftDisk) cost(headRaw []byte) (c transaction.Cost, err error) {
	inv, err := d.inventory()
	if err != nil {
		return c, err
	}
	head, err := snapshot.DecodeHead(headRaw)
	if err != nil {
		return c, err
	}
	hash := sha256.New()
	c.ScannedEntries = uint64(len(d.dirs) + len(d.stage))
	for _, f := range inv.Files() {
		n := f.Bytes.Uint64()
		if strings.HasPrefix(f.Path, "intent/") {
			c.IntentBytes += n
			fmt.Fprintf(hash, "%s%c%s\n", strings.TrimPrefix(f.Path, "intent/"), byte(0), f.Sha256)
		} else {
			c.ScannedEntries++
		}
		if strings.HasPrefix(f.Path, "receipts/") {
			c.JournalCount++
			c.JournalBytes += n
		}
		if strings.HasPrefix(f.Path, "evidence/") {
			c.EvidenceBytes += n
		}
	}
	for _, f := range d.stage {
		c.TemporaryBytes += uint64(len(f.raw))
	}
	if f, ok := d.stage["active.json"]; ok {
		desc, e := snapshot.DecodeStageDescriptor(f.raw)
		if e != nil {
			return c, e
		}
		for _, a := range desc.Artifacts {
			c.PromisedBytes += a.Bytes.Uint64()
		}
	}
	m := archive.Manifest{QueueID: head.QueueID, ExportedAtSeq: wire.SizeOf(c.JournalCount), HeadSha256: wire.Sum(headRaw), VersionSha256: head.VersionSha256, BarrierSha256: ftDigest(d.raw["barrier.json"]), PrimaryWorktree: head.PrimaryWorktree, IntentTreeSha256: wire.Digest(fmt.Sprintf("%x", hash.Sum(nil))), Files: inv.Files(), ReceiptCount: wire.SizeOf(c.JournalCount), LastReceiptSha256: *head.LastReceiptSha256, HeadGeneration: head.Generation, Complete: true}
	enc, err := archive.MeasureManifestEncoding(&m)
	c.Files, c.PayloadBytes, c.ManifestBytes, c.TarBytes = uint64(enc.Files), enc.PayloadBytes, enc.ManifestBytes, enc.TarBytes
	return c, err
}

// The shared observer validates structure; retained handles still mint tokens.
func (d *ftDisk) sharedStage() (*snapshot.StageObservation, error) {
	files := make([]snapshot.StageFile, 0, len(d.stage))
	for name, f := range d.stage {
		files = append(files, snapshot.StageFile{Name: name, Raw: f.raw})
	}
	return snapshot.ObserveStage(files)
}

type ftObservation struct {
	disk    *ftDisk
	latest  map[string][]byte
	head    []byte // derived only from the last actual linked receipt and genesis
	rc      *snapshot.Receipt
	desc    *snapshot.StageDescriptor
	pending bool
	posts   map[string][]byte
	pre     map[string]*wire.Digest
}

func ftCanonical(raw []byte) error {
	v, err := wire.Parse(raw)
	if err != nil {
		return err
	}
	if !bytes.Equal(wire.EncodeFile(v), raw) {
		return fixtureRefused
	}
	return nil
}
func ftEqualDigest(a, b *wire.Digest) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func ftPost(d *ftDisk, p snapshot.PostEntry) ([]byte, error) {
	if p.Sha256 == nil {
		return nil, nil
	}
	var raw []byte
	if p.Record != nil {
		raw = wire.EncodeFile(*p.Record)
	} else {
		var ok bool
		raw, ok = d.raw["evidence/"+string(*p.BlobSha256)]
		if !ok {
			return nil, fixtureRefused
		}
	}
	bound, err := snapshot.PostBound(p.Path)
	if err != nil {
		return nil, err
	}
	if len(raw) > bound || wire.Sum(raw) != *p.Sha256 {
		return nil, fixtureRefused
	}
	if err = snapshot.ContentPath(p.Path, raw); err != nil {
		return nil, err
	}
	return bytes.Clone(raw), nil
}

// Explicit stable and pending uses. This is a finite fixture proof over actual
// bytes, not journal.Audit with suppressed errors or a production decoder.
func (h *ftHarness) observe(recovery bool) (*ftObservation, error) {
	d, err := h.capture()
	if err != nil {
		return nil, err
	}
	shared, err := d.sharedStage()
	if err != nil {
		return nil, err
	}
	o := &ftObservation{disk: d, latest: map[string][]byte{}, posts: map[string][]byte{}, pre: map[string]*wire.Digest{}}
	var head *snapshot.Head
	var headSeq uint64
	if raw := d.raw["head.json"]; raw != nil {
		if err = ftCanonical(raw); err != nil {
			return nil, err
		}
		head, err = snapshot.DecodeHead(raw)
		if err != nil {
			return nil, err
		}
		headSeq = head.LastSeq.Uint64()
	}
	names := []string{}
	for p, raw := range d.raw {
		if strings.HasPrefix(p, "receipts/") {
			names = append(names, p)
		}
		if strings.HasPrefix(p, "evidence/") {
			if err = snapshot.ContentPath(p, raw); err != nil {
				return nil, err
			}
		}
	}
	sort.Strings(names)
	if len(names) > 12 || uint64(len(names)) < headSeq || uint64(len(names)) > headSeq+1 {
		return nil, fixtureRefused
	}
	if !recovery && uint64(len(names)) != headSeq {
		return nil, fixtureRefused
	}
	o.pending = uint64(len(names)) == headSeq+1
	var prev *wire.Digest
	var init *snapshot.Init
	var genesis wire.Digest
	for i, p := range names {
		name, _ := snapshot.ReceiptName(uint64(i + 1))
		if p != "receipts/"+name {
			return nil, fixtureRefused
		}
		raw := d.raw[p]
		if err = ftCanonical(raw); err != nil {
			return nil, err
		}
		rc, e := snapshot.DecodeReceipt(raw)
		if e != nil {
			return nil, e
		}
		if rc.Seq.Uint64() != uint64(i+1) || !ftEqualDigest(rc.Prev, prev) || rc.HeadGeneration != "0" || rc.AttemptID != nil || rc.Generation != nil || rc.Outcome != "COMPLETED" || len(rc.Codes) != 0 {
			return nil, fixtureRefused
		}
		posts := map[string][]byte{}
		for _, post := range rc.Post {
			b, e := ftPost(d, post)
			if e != nil {
				return nil, e
			}
			posts[post.Path] = b
		}
		if err = h.validateReceipt(rc, posts, o.latest); err != nil {
			return nil, err
		}
		if i == 0 {
			genesis = wire.Sum(raw)
			for p, b := range posts {
				if strings.HasPrefix(p, "pinned/") {
					init, err = snapshot.DecodeInit(b)
					if err != nil {
						return nil, err
					}
				}
			}
			if init == nil || init.QueueID.Raw != fixture.QueueID || init.PrimaryWorktree != h.r.Root || init.VersionSha256 != wire.Sum([]byte(snapshot.VersionBytes)) {
				return nil, fixtureRefused
			}
		}
		prev = ftDigest(raw)
		derived := snapshot.Head{QueueID: init.QueueID, LastSeq: rc.Seq, LastReceiptSha256: prev, Generation: rc.HeadGeneration, InitSha256: genesis, PrimaryWorktree: init.PrimaryWorktree, VersionSha256: init.VersionSha256}
		o.head = wire.EncodeFile(derived.Value())
		if rc.Seq.Uint64() == headSeq && !bytes.Equal(o.head, d.raw["head.json"]) {
			return nil, fixtureRefused
		}
		for p, b := range posts {
			o.latest[p] = b
		}
		if i == len(names)-1 {
			o.rc = rc
			o.posts = posts
			for _, pre := range rc.Pre {
				o.pre[pre.Path] = pre.Sha256
			}
		}
	}
	if len(names) == 0 {
		for p := range d.raw {
			if !strings.HasPrefix(p, "evidence/") {
				return nil, fixtureRefused
			}
		}
	} else {
		for p, raw := range o.latest {
			physical := d.raw[p]
			if ftEqualDigest(ftDigest(physical), ftDigest(raw)) {
				continue
			}
			if o.pending {
				if pre, affected := o.pre[p]; affected && ftEqualDigest(ftDigest(physical), pre) {
					continue
				}
			}
			if h.selected && p == ftTicket && physical != nil && (!recovery || o.rc.Kind == "UNPAUSE") {
				continue
			}
			if strings.HasPrefix(p, "intent/") {
				return nil, wire.Errorf(wire.CodeIntentDiverged, p, "third or unselected fixture intent value preserved")
			}
			return nil, fixtureRefused
		}
		for p := range d.raw {
			if _, ok := o.latest[p]; ok {
				continue
			}
			if p == "head.json" || strings.HasPrefix(p, "receipts/") || strings.HasPrefix(p, "evidence/") {
				continue
			}
			return nil, fixtureRefused
		}
	}
	if len(d.stage) != 0 && h.era == "" {
		return nil, fixtureRefused
	}
	if active, ok := d.stage["active.json"]; ok {
		if h.era == "" || wire.Sum(active.raw) != h.era {
			return nil, fixtureRefused
		}
		o.desc = shared.Descriptor
		if o.desc.QueueID != fixture.QueueID {
			return nil, fixtureRefused
		}
		if recovery {
			if err = o.bindDescriptor(); err != nil {
				return nil, err
			}
		}
	}
	if recovery && len(d.stage) != 0 && o.desc == nil {
		return nil, fixtureRefused
	}
	// Only validated actual genesis afterimages may supply an absent physical
	// queue. Staged proposed queue bytes are never a binding source.
	queue := d.raw["intent/queue.json"]
	if queue == nil && o.pending {
		queue = o.latest["intent/queue.json"]
	}
	if queue != nil {
		binding := snapshot.StageBinding{QueueID: fixture.QueueID, QueueRaw: queue, HeadRaw: d.raw["head.json"]}
		if o.rc != nil {
			name, _ := snapshot.ReceiptName(o.rc.Seq.Uint64())
			binding.ReceiptRaw = d.raw["receipts/"+name]
			if o.rc.RequestID != nil {
				rp, _ := snapshot.RequestPath(*o.rc.RequestID)
				binding.RequestRaw = o.posts[rp]
			}
		}
		if err = shared.Bind(binding); err != nil {
			return nil, err
		}
	}
	if !recovery {
		inv, e := d.inventory()
		if e != nil {
			return nil, e
		}
		_, err = transaction.ClassifyStage(o.stageObservation(inv), nil)
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

func (h *ftHarness) validateReceipt(rc *snapshot.Receipt, posts, prior map[string][]byte) error {
	if rc.Seq != "1" && rc.RequestID == nil {
		return fixtureRefused
	}
	if rc.ActorRole != "OWNER" && rc.ActorRole != "OPERATOR" {
		return fixtureRefused
	}
	if rc.Kind != "MUTATION" && rc.Kind != "RECONCILE" && (rc.TicketID != nil || rc.ExpectedRevision != nil) {
		return fixtureRefused
	}
	allowed := map[string]bool{}
	if rc.RequestID != nil {
		p, err := snapshot.RequestPath(*rc.RequestID)
		if err != nil {
			return err
		}
		allowed[p] = true
	}
	switch rc.Kind {
	case "INIT":
		for _, p := range []string{"VERSION", "intent/queue.json", "intent/policy.json", "reservations.json"} {
			allowed[p] = true
		}
		pins := 0
		for p := range posts {
			if strings.HasPrefix(p, "pinned/") {
				allowed[p] = true
				pins++
			}
		}
		if pins != 1 {
			return fixtureRefused
		}
		for _, pre := range rc.Pre {
			if pre.Sha256 != nil {
				return fixtureRefused
			}
		}
	case "MUTATION":
		if rc.Seq != "2" || rc.RequestID == nil || *rc.RequestID != "seed" {
			return fixtureRefused
		}
		allowed[ftTicket] = true
	case "PAUSE", "UNPAUSE":
		allowed["barrier.json"] = true
	case "RECONCILE":
		allowed[ftTicket] = true
		for p := range posts {
			if strings.HasPrefix(p, "evidence/") {
				allowed[p] = true
			}
		}
	default:
		return fixtureRefused
	}
	if len(posts) != len(allowed) {
		return fixtureRefused
	}
	for p, raw := range posts {
		if !allowed[p] {
			return fixtureRefused
		}
		if p != "VERSION" && !strings.HasPrefix(p, "evidence/") && raw != nil {
			if err := ftCanonical(raw); err != nil {
				return err
			}
		}
		switch {
		case p == "VERSION":
			if string(raw) != snapshot.VersionBytes {
				return fixtureRefused
			}
		case p == "intent/queue.json":
			q, err := intent.DecodeQueue(raw)
			if err != nil {
				return err
			}
			if q.QueueID.Raw != fixture.QueueID || !q.Fixture || q.ExecutionCutover != nil || q.ImportMapSha256 != nil || q.IntentBranch != "main" {
				return fixtureRefused
			}
		case p == "intent/policy.json":
			q, err := intent.DecodePolicy(raw)
			if err != nil {
				return err
			}
			if len(q.Runtimes) != 0 {
				return fixtureRefused
			}
		case p == "reservations.json":
			want := wire.EncodeFile(ftObject("profile", wire.String("taskman-reservation-set/0"), "queueId", wire.String(fixture.QueueID), "entries", wire.Array()))
			if !bytes.Equal(raw, want) {
				return fixtureRefused
			}
		case p == "barrier.json" && raw != nil:
			b, err := snapshot.DecodeBarrier(raw)
			if err != nil {
				return err
			}
			if b.QueueID.Raw != fixture.QueueID || b.SinceSeq != rc.Seq || b.Scope != "ADMISSION" || b.Reason != "OPERATOR" || b.Actor != rc.ActorID || b.Since != rc.RecordedAt {
				return fixtureRefused
			}
		case p == ftTicket:
			c, err := ticket.Decode(raw)
			if err != nil {
				return err
			}
			if rc.TicketID == nil || rc.TicketID.Raw != fixture.TicketID("AT-01") || c.TicketID.Raw != rc.TicketID.Raw {
				return fixtureRefused
			}
			if rc.Kind == "MUTATION" {
				if c.Revision != "1" || rc.ExpectedRevision != nil {
					return fixtureRefused
				}
			} else {
				old, err := ticket.Decode(prior[p])
				if err != nil {
					return err
				}
				if rc.ExpectedRevision == nil || *rc.ExpectedRevision != old.Revision {
					return fixtureRefused
				}
				if !bytes.Equal(raw, prior[p]) && (c.Revision.Int() != old.Revision.Int()+1 || c.PreviousRecordSha256 == nil || *c.PreviousRecordSha256 != old.FileDigest()) {
					return fixtureRefused
				}
			}
		case strings.HasPrefix(p, "requests/"):
			r, err := snapshot.DecodeRequest(raw)
			if err != nil {
				return err
			}
			if prior[p] != nil || rc.RequestID == nil || r.Entry.RequestID != *rc.RequestID || r.Seq != rc.Seq || r.Entry.Outcome.Outcome != rc.Outcome || len(r.Entry.Outcome.Codes) != 0 {
				return fixtureRefused
			}
			if rc.TicketID == nil {
				if r.Entry.Outcome.ResultingRevision != nil || r.Entry.Outcome.ResultingAcceptanceRevision != nil {
					return fixtureRefused
				}
			} else {
				c, err := ticket.Decode(posts[ftTicket])
				if err != nil {
					return err
				}
				out := r.Entry.Outcome
				if out.ResultingRevision == nil || out.ResultingAcceptanceRevision == nil || *out.ResultingRevision != c.Revision || *out.ResultingAcceptanceRevision != c.AcceptanceRevision {
					return fixtureRefused
				}
			}
		}
	}
	if rc.Kind == "RECONCILE" {
		discards := 0
		for p, raw := range posts {
			if !strings.HasPrefix(p, "evidence/") {
				continue
			}
			discards++
			if !bytes.Equal(posts[ftTicket], prior[ftTicket]) {
				return fixtureRefused
			}
			for _, pre := range rc.Pre {
				if pre.Path == ftTicket && !ftEqualDigest(pre.Sha256, ftDigest(raw)) {
					return fixtureRefused
				}
			}
		}
		if discards > 1 {
			return fixtureRefused
		}
	}
	for _, pre := range rc.Pre {
		if pre.Path == ftTicket && rc.Kind == "RECONCILE" {
			continue
		} // physical D, not canonical C
		if strings.HasPrefix(pre.Path, "evidence/") {
			continue
		} // retained orphan reuse
		if !ftEqualDigest(pre.Sha256, ftDigest(prior[pre.Path])) {
			return fixtureRefused
		}
	}
	return nil
}

func (o *ftObservation) stageObservation(inv *transaction.Inventory) transaction.StageObservation {
	obs := transaction.StageObservation{Inventory: inv, BaseState: "UNCHANGED", ReceiptInventory: "COMPLETE_NO_NEXT", Premise: transaction.FixtureNoRuntime, CurrentHead: o.disk.raw["head.json"]}
	for name, f := range o.disk.stage {
		obs.Files = append(obs.Files, transaction.StageFile{Name: name, Type: "REGULAR", Data: f.raw})
	}
	return obs
}

func (o *ftObservation) bindDescriptor() error {
	d, rc := o.desc, o.rc
	if rc == nil || rc.RequestID == nil || d.RequestID != *rc.RequestID || d.RecordedAt != rc.RecordedAt {
		return fixtureRefused
	}
	if d.Base == nil {
		if rc.Seq != "1" {
			return fixtureRefused
		}
	} else if d.Base.LastSeq.Uint64()+1 != rc.Seq.Uint64() || rc.Prev == nil || *rc.Prev != d.Base.LastReceiptSha256 {
		return fixtureRefused
	}
	kind := d.Operation
	if kind == transaction.KeepJournal || kind == transaction.AdoptFile {
		kind = "RECONCILE"
	}
	if rc.Kind != kind {
		return fixtureRefused
	}
	rp, _ := snapshot.RequestPath(d.RequestID)
	req, err := snapshot.DecodeRequest(o.posts[rp])
	if err != nil {
		return err
	}
	if req.Entry.MutationSha256 != d.RequestSha256 {
		return fixtureRefused
	}
	seen := map[string]bool{}
	for _, a := range d.Artifacts {
		b, err := o.artifactBytes(a)
		if err != nil {
			return err
		}
		if uint64(len(b)) != a.Bytes.Uint64() || wire.Sum(b) != a.Sha256 {
			return fixtureRefused
		}
		seen[a.Target] = true
	}
	for p, b := range o.posts {
		if b != nil && !seen[p] {
			return fixtureRefused
		}
	}
	if d.Operation == transaction.KeepJournal {
		pre := o.pre[ftTicket]
		if pre == nil {
			return fixtureRefused
		}
		discard, ok := o.posts["evidence/"+string(*pre)]
		if !ok || wire.Sum(discard) != *pre {
			return fixtureRefused
		}
	}
	return nil
}

func (o *ftObservation) artifactBytes(a snapshot.StageDescription) ([]byte, error) {
	switch a.Role {
	case "HEAD":
		return o.head, nil
	case "RECEIPT":
		b, ok := o.disk.raw[a.Target]
		if !ok {
			return nil, fixtureRefused
		}
		return b, nil
	case "POST":
		b, ok := o.posts[a.Target]
		if !ok || b == nil {
			return nil, fixtureRefused
		}
		return b, nil
	case "EVIDENCE":
		bound := false
		for _, p := range o.rc.Post {
			if p.BlobSha256 != nil && a.Target == "evidence/"+string(*p.BlobSha256) {
				bound = true
			}
		}
		if !bound {
			return nil, fixtureRefused
		}
		b, ok := o.disk.raw[a.Target]
		if !ok {
			return nil, fixtureRefused
		}
		return b, nil
	}
	return nil, fixtureRefused
}

// Tokens come from this fresh session's retained no-follow observations. The
// boolean map is publication capability; partial/temp observations never get it.
func (h *ftHarness) reopen(o *ftObservation, cleanupOnly bool) (map[string]*fixtureStage, error) {
	shared, err := o.disk.sharedStage()
	if err != nil {
		return nil, err
	}
	// Recheck shared structure at reopen; it is not publication capability.
	if (shared.Descriptor == nil) != (o.desc == nil) {
		return nil, fixtureRefused
	}
	tokens := map[string]*fixtureStage{}
	declared := map[string]snapshot.StageDescription{}
	if o.desc != nil {
		for _, a := range o.desc.Artifacts {
			declared[a.Slot] = a
		}
	}
	err = h.s.operation(func() error {
		qualified := map[*fixtureStage]bool{}
		for name, f := range o.disk.stage {
			role, publish := fixtureDescriptor, false
			if name != "active.json" && name != "active.json.tmp" {
				a, ok := declared[name]
				if !ok {
					return fixtureRefused
				}
				if uint64(len(f.raw)) > a.Bytes.Uint64() {
					return fixtureRefused
				}
				publish = uint64(len(f.raw)) == a.Bytes.Uint64()
				if publish && wire.Sum(f.raw) != a.Sha256 {
					return fixtureRefused
				}
				if !publish && !cleanupOnly {
					return fixtureRefused
				}
				target, err := ftTarget(a.Target)
				if err != nil {
					return err
				}
				role = target.role
			} else if name == "active.json.tmp" && o.desc != nil {
				active := o.disk.stage["active.json"].raw
				if len(f.raw) > len(active) || (len(f.raw) == len(active) && !bytes.Equal(f.raw, active)) {
					return fixtureRefused
				}
			}
			checked, err := h.s.checkedFile("staging", name, f.info, int64(len(f.raw)), wire.Sum(f.raw))
			if err != nil {
				return err
			}
			if err = h.s.closeFile(checked); err != nil {
				return err
			}
			token := &fixtureStage{owner: h.s, slot: fixtureSlot(name), role: role, info: f.info, digest: wire.Sum(f.raw), size: int64(len(f.raw))}
			tokens[name] = token
			qualified[token] = publish && !cleanupOnly
		}
		h.s.stages = qualified
		return nil
	})
	return tokens, err
}

func (h *ftHarness) cleanup(abort bool) error {
	o, err := h.observe(!abort)
	if err != nil {
		return err
	}
	if !abort && (o.pending || !bytes.Equal(o.head, o.disk.raw["head.json"])) {
		return fixtureRefused
	}
	if !abort {
		// Equal bytes after a failed rename/link sync still require durability
		// confirmation before any cleanup of the committed proof.
		for p, raw := range o.posts {
			if raw != nil {
				if err = h.confirm(p, raw); err != nil {
					return err
				}
			}
		}
		if err = h.confirm("head.json", o.head); err != nil {
			return err
		}
		name, _ := snapshot.ReceiptName(o.rc.Seq.Uint64())
		if err = h.confirm("receipts/"+name, o.disk.raw["receipts/"+name]); err != nil {
			return err
		}
	}
	if _, err = h.reopen(o, true); err != nil {
		return err
	}
	var obs transaction.StageObservation
	state := transaction.CleanupState{}
	if abort {
		inv, e := o.disk.inventory()
		if e != nil {
			return e
		}
		obs = o.stageObservation(inv)
		if _, e = transaction.Cleanup(obs, state); e != nil {
			return e
		}
	}
	names := []string{}
	for name := range o.disk.stage {
		if name != "active.json" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if err = h.s.removeStage(fixtureSlot(name)); err != nil {
			return err
		}
		state.Removed = append(state.Removed, name)
		if abort {
			if _, err = transaction.Cleanup(obs, state); err != nil {
				return err
			}
		}
	}
	if err = h.s.operation(func() error { return h.s.syncParent("staging") }); err != nil {
		return err
	}
	state.PayloadSynced = true
	if abort {
		if _, err = transaction.Cleanup(obs, state); err != nil {
			return err
		}
	}
	if _, ok := o.disk.stage["active.json"]; ok {
		if err = h.s.removeStage("active.json"); err != nil {
			return err
		}
		state.DescriptorUnlinked, state.DescriptorSynced = true, true
	}
	if abort {
		result, e := transaction.Cleanup(obs, state)
		if e != nil {
			return e
		}
		if result.FreshDescriptor != "MODEL_READY" {
			return fixtureRefused
		}
	}
	h.era = ""
	return nil
}

// NO Plan, Model, original invocation, or cached expected posts enter recovery.
// Missing bytes are derived from committed receipt/blob/genesis proof only.
func (h *ftHarness) recover() error {
	o, err := h.observe(true)
	if err != nil {
		return err
	}
	if o.desc == nil {
		if o.pending {
			return fixtureRefused
		}
		// A returned failure after active unlink leaves a directory-sync debt.
		// No bytes are reconstructed and no new era starts before confirming it.
		if h.era != "" && h.s.parents["staging"] != nil {
			if !bytes.Equal(o.head, o.disk.raw["head.json"]) {
				return fixtureRefused
			}
			if err = h.s.operation(func() error { return h.s.syncParent("staging") }); err != nil {
				return err
			}
			h.era = ""
		}
		return nil
	}
	tokens, err := h.reopen(o, false)
	if err != nil {
		return err
	}
	if !o.pending {
		return h.cleanup(false)
	}
	name, _ := snapshot.ReceiptName(o.rc.Seq.Uint64())
	if err = h.confirm("receipts/"+name, o.disk.raw["receipts/"+name]); err != nil {
		return err
	}
	for _, p := range o.rc.Post {
		if p.BlobSha256 != nil {
			bp := "evidence/" + string(*p.BlobSha256)
			if err = h.confirm(bp, o.disk.raw[bp]); err != nil {
				return err
			}
		}
	}
	ordered := []snapshot.StageDescription{}
	for _, a := range o.desc.Artifacts {
		if a.Role == "POST" && !strings.HasPrefix(a.Target, "evidence/") {
			ordered = append(ordered, a)
		}
	}
	// With missing old bytes we cannot refresh a sizing model. Actual POST-sized
	// growth first, then already-applied posts; deleted barrier remains last.
	sort.Slice(ordered, func(i, j int) bool {
		gi := ordered[i].Bytes.Uint64() >= uint64(len(o.disk.raw[ordered[i].Target]))
		gj := ordered[j].Bytes.Uint64() >= uint64(len(o.disk.raw[ordered[j].Target]))
		if gi != gj {
			return gi
		}
		return ordered[i].Target < ordered[j].Target
	})
	apply := func(a snapshot.StageDescription, pre *wire.Digest) error {
		raw, e := o.artifactBytes(a)
		if e != nil {
			return e
		}
		if bytes.Equal(o.disk.raw[a.Target], raw) && o.disk.raw[a.Target] != nil {
			return h.confirm(a.Target, raw)
		}
		stage := tokens[a.Slot]
		if stage == nil {
			target, e := ftTarget(a.Target)
			if e != nil {
				return e
			}
			stage, e = h.s.prepare(fixtureSlot(a.Slot), target.role, raw)
			if e != nil {
				return e
			}
		}
		return h.apply(transaction.Artifact{Description: a, Data: raw}, stage, pre, o.desc.Operation)
	}
	for _, a := range ordered {
		if err = apply(a, o.pre[a.Target]); err != nil {
			return err
		}
	}
	for _, p := range o.rc.Post {
		if p.Sha256 != nil {
			continue
		}
		if _, err = journal.DeleteRedo(*o.pre[p.Path], ftDigest(o.disk.raw[p.Path])); err != nil {
			return err
		}
		if err = h.s.removeBarrier(fixtureTarget{fixtureBarrier, "barrier.json"}, *o.pre[p.Path]); err != nil {
			return err
		}
	}
	for _, a := range o.desc.Artifacts {
		if a.Role == "HEAD" {
			if err = apply(a, ftDigest(o.disk.raw["head.json"])); err != nil {
				return err
			}
		}
	}
	return h.cleanup(false)
}

func TestTMV0009_AS11_FixtureFoundationFreshRecovery(t *testing.T) {
	for _, op := range []string{transaction.Init, transaction.Pause, transaction.Unpause, transaction.KeepJournal, transaction.AdoptFile} {
		for _, stop := range []string{"durable-receipt-link", "first-post", "durable-head"} {
			t.Run(op+"/"+stop, func(t *testing.T) {
				h := ftNew(t, op != transaction.Init)
				if op == transaction.Unpause {
					_, e := h.publish(h.plan(h.request(transaction.Pause, "base-pause")), nil)
					fixtureMust(t, e)
				}
				if op == transaction.KeepJournal {
					h.edit([]byte{})
				}
				if op == transaction.AdoptFile {
					c := fixture.Ticket("AT-01")
					c.Title = "adopted"
					h.edit(c.Encode())
				}
				p := h.plan(h.request(op, "recover"))
				fault := errors.New("finite prefix stop")
				committed, err := h.publish(p, func(step string) error {
					if step == stop || (stop == "first-post" && strings.HasPrefix(step, "durable-post:")) {
						return fault
					}
					return nil
				})
				if !committed || !errors.Is(err, fault) {
					t.Fatalf("prefix: committed=%v err=%v", committed, err)
				}
				h.restart()
				done := ftRecoveryCostWitness(t, h, p)
				fixtureMust(t, h.recover())
				done()
				ftFinal(t, h, p)
				o, err := h.observe(false)
				fixtureMust(t, err)
				if !bytes.Equal(o.disk.raw["head.json"], p.Head()) || len(o.disk.stage) != 0 {
					t.Fatal("recovered head/cleanup")
				}
			})
		}
	}
}

func ftFinalCost(t *testing.T, p *transaction.Plan) transaction.Cost {
	t.Helper()
	c, err := transaction.CheckCapacity(p)
	fixtureMust(t, err)
	for _, p := range c.Prefixes {
		if p.Step == "durable-staging-cleanup" {
			return p.Cost
		}
	}
	t.Fatal("missing final cost")
	return transaction.Cost{}
}
