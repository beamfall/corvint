package journal

import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ProfileCheckpoint names the derived audit checkpoint (CAL-V0-059). It is
// not journal state: no receipt posts it, no archive carries it and deleting
// it only costs the next read one complete audit.
const ProfileCheckpoint = "taskman-audit-checkpoint/0"

// MaxCheckpointBytes bounds the checkpoint file a read is willing to consume.
const MaxCheckpointBytes = 16 * wire.MiB

// Audit modes reported by Result.Mode.
const (
	ModeFull       = "FULL"
	ModeCheckpoint = "CHECKPOINT_PLUS_TAIL"
)

// CheckpointPath is a sibling of the state directory, so state scans, archive
// export and older runtimes never see the file.
func CheckpointPath(stateDir string) string { return stateDir + ".checkpoint.json" }

// CheckpointEntry is the latest canonical afterimage of one non-request path
// at the checkpoint sequence. A nil digest is a retained deletion.
type CheckpointEntry struct {
	Path   string
	Seq    wire.Size
	Sha256 *wire.Digest
}

// Checkpoint records what one complete settled audit established through Seq.
// It claims nothing a reader does not rebind: the named receipt must still
// hash to ReceiptSha256 and every entry must still match its projection.
type Checkpoint struct {
	QueueID          wire.QueueID
	PrimaryWorktree  string
	Seq              wire.Size
	ReceiptSha256    wire.Digest
	Generation       wire.Size
	InitSha256       wire.Digest
	SemanticCoverage string
	Entries          []CheckpointEntry
}

func maxCheckpointEntries() int {
	return wire.MaxArchiveScanEntries + intent.MaxIntentRootEntries + wire.MaxTicketsPerQueue + wire.MaxReleasesPerQueue
}

// Checkpoint derives the checkpoint a complete settled audit supports. It
// returns nil for every other observation: a checkpoint is never extended
// from another checkpoint and never taken over a pending receipt.
func (res *Result) Checkpoint() *Checkpoint {
	if res == nil || res.chain == nil || res.Mode != ModeFull || res.Pending || res.StagingPresent || res.Head == nil || res.StructuralConsistency != "CONSISTENT" || res.LastSeq != res.Head.LastSeq {
		return nil
	}
	cp := &Checkpoint{QueueID: res.Head.QueueID, PrimaryWorktree: res.Head.PrimaryWorktree, Seq: res.LastSeq, ReceiptSha256: res.LastReceiptSha256, Generation: res.Head.Generation, InitSha256: res.Head.InitSha256, SemanticCoverage: res.SemanticCoverage}
	for p, l := range res.chain.canonical {
		if strings.HasPrefix(p, "requests/") {
			continue
		}
		cp.Entries = append(cp.Entries, CheckpointEntry{Path: p, Seq: l.seq, Sha256: l.digest})
	}
	sort.Slice(cp.Entries, func(i, j int) bool { return cp.Entries[i].Path < cp.Entries[j].Path })
	return cp
}

// Encode renders the canonical file bytes.
func (c *Checkpoint) Encode() []byte {
	entries := make([]wire.Value, 0, len(c.Entries))
	for _, e := range c.Entries {
		o := wire.NewObject()
		o.Set("path", wire.String(e.Path))
		o.Set("seq", wire.String(string(e.Seq)))
		if e.Sha256 == nil {
			o.Set("sha256", wire.Null())
		} else {
			o.Set("sha256", wire.String(string(*e.Sha256)))
		}
		entries = append(entries, wire.ObjectValue(o))
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(ProfileCheckpoint))
	o.Set("queueId", wire.String(c.QueueID.Raw))
	o.Set("primaryWorktree", wire.String(c.PrimaryWorktree))
	o.Set("seq", wire.String(string(c.Seq)))
	o.Set("receiptSha256", wire.String(string(c.ReceiptSha256)))
	o.Set("generation", wire.String(string(c.Generation)))
	o.Set("initSha256", wire.String(string(c.InitSha256)))
	o.Set("semanticCoverage", wire.String(c.SemanticCoverage))
	o.Set("entries", wire.Array(entries...))
	return wire.EncodeFile(wire.ObjectValue(o))
}

// DecodeCheckpoint parses checkpoint bytes. Any error means only that the
// checkpoint is unusable; callers fall back to the complete audit.
func DecodeCheckpoint(raw []byte) (*Checkpoint, error) {
	if len(raw) > MaxCheckpointBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "checkpoint larger than %d bytes", MaxCheckpointBytes)
	}
	max := maxCheckpointEntries()
	v, err := wire.ParseWith(raw, wire.ParseOptions{WideArrayKey: "entries", WideArrayMax: max, MaxNodes: 8*max + 64})
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	if err := wire.ProfileVersion("/profile", v, ProfileCheckpoint); err != nil {
		return nil, err
	}
	r.Closed("profile", "queueId", "primaryWorktree", "seq", "receiptSha256", "generation", "initSha256", "semanticCoverage", "entries")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileCheckpoint); err != nil {
		return nil, err
	}
	c := &Checkpoint{}
	c.QueueID = r.Field("queueId").QueueID()
	c.PrimaryWorktree = r.Field("primaryWorktree").PathText()
	c.Seq = r.Field("seq").Size()
	c.ReceiptSha256 = r.Field("receiptSha256").Digest()
	c.Generation = r.Field("generation").Size()
	c.InitSha256 = r.Field("initSha256").Digest()
	c.SemanticCoverage = r.Field("semanticCoverage").Enum("KNOWN_CODECS", "UNKNOWN")
	entries := r.Field("entries").Array(max, true)
	if err := r.Err(); err != nil {
		return nil, err
	}
	if c.Seq.Uint64() < 1 {
		return nil, wire.Errorf(wire.CodeMalformed, "/seq", "checkpoint must name a receipt")
	}
	c.Entries = make([]CheckpointEntry, 0, len(entries))
	for i, e := range entries {
		e.Closed("path", "seq", "sha256")
		entry := CheckpointEntry{Path: e.Field("path").String(), Seq: e.Field("seq").Size(), Sha256: e.Field("sha256").DigestOrNull()}
		if err := r.Err(); err != nil {
			return nil, err
		}
		if _, err := snapshot.PostBound(entry.Path); err != nil {
			return nil, err
		}
		if strings.HasPrefix(entry.Path, "requests/") || entry.Seq.Uint64() < 1 || entry.Seq.Uint64() > c.Seq.Uint64() {
			return nil, wire.Errorf(wire.CodeMalformed, e.Where(), "entry outside the checkpoint prefix")
		}
		if i > 0 && c.Entries[i-1].Path >= entry.Path {
			return nil, wire.Errorf(wire.CodeMalformed, e.Where(), "entries are not strictly path-ordered")
		}
		c.Entries = append(c.Entries, entry)
	}
	return c, nil
}
