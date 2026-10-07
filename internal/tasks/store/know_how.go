package store

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Know-how freshness states (KHN-V0-005). They are computed at read time
// against the reader's committed HEAD and never stored.
const (
	KnowHowCurrent = "CURRENT"
	KnowHowStale   = "STALE"
	KnowHowUnknown = "UNKNOWN"
)

// KnowHowDeliveryMaxBytes caps the canonical encoding of the notes a claim
// delivers (KHN-V0-006).
const KnowHowDeliveryMaxBytes = 2048

// KnowHowNote is one active ADD entry of a home ticket with its read-time
// freshness: Anchors holds one state per entry anchor, in anchor order, and
// Freshness the note state. The text is agent-authored, untrusted data.
type KnowHowNote struct {
	TicketID  string
	Entry     ticket.KnowHowEntry
	Anchors   []string
	Freshness string
}

// KnowHowPins resolves rev's commit and the blob each path names in that
// commit with one `git cat-file --batch-check` in root (KHN-V0-001).
// A path that is missing or is not a regular blob is refused: a note is never
// pinned to something the writer's commit does not hold.
func KnowHowPins(root, rev string, paths []string) (string, []string, error) {
	commit, objs, err := catFileAtCommit(root, rev, paths)
	if err != nil {
		return "", nil, err
	}
	if commit == "" {
		return "", nil, wire.Errorf(wire.CodeMalformed, "/payload/commit", "%s does not name a commit in this checkout", rev)
	}
	blobs := make([]string, len(paths))
	for i, p := range paths {
		if objs[i].kind != "blob" {
			return "", nil, wire.Errorf(wire.CodeMalformed, "/payload/anchors", "anchor %q is not a file in commit %s", p, commit)
		}
		blobs[i] = objs[i].oid
	}
	return commit, blobs, nil
}

// SelectKnowHow returns the active notes of every ticket in inv, or of home
// alone when home is set, whose anchors intersect paths (all notes when paths
// is empty). A path ending in "/" matches every anchor below it; any other
// path matches one anchor exactly (KHN-V0-006).
func SelectKnowHow(inv *ticket.Inventory, paths []string, home string) []KnowHowNote {
	var out []KnowHowNote
	for _, id := range inv.IDs() {
		if home != "" && id != home {
			continue
		}
		rec, ok := inv.Get(id)
		if !ok || len(rec.KnowHow) == 0 {
			continue
		}
		for _, k := range ticket.ActiveKnowHow(rec.KnowHow) {
			if len(paths) == 0 || knowHowIntersects(k.Anchors, paths) {
				out = append(out, KnowHowNote{TicketID: id, Entry: k})
			}
		}
	}
	return out
}

func knowHowIntersects(anchors []ticket.KnowHowAnchor, paths []string) bool {
	for _, a := range anchors {
		for _, t := range paths {
			if a.Path == t || (strings.HasSuffix(t, "/") && strings.HasPrefix(a.Path, t)) {
				return true
			}
		}
	}
	return false
}

// ResolveKnowHowFreshness sets every note's anchor and note freshness from
// root's committed HEAD with one batched Git call and returns that HEAD
// commit, or "" when it is unavailable (KHN-V0-005). An anchor whose blob
// equals its pin is CURRENT and one that differs is STALE; a missing path, a
// non-blob or any Git failure is UNKNOWN, never CURRENT. A note is STALE when
// any anchor is, else UNKNOWN when any anchor is, else CURRENT. It writes
// nothing: the Git environment disables optional locks.
func ResolveKnowHowFreshness(root string, notes []KnowHowNote) string {
	index := map[string]int{}
	var paths []string
	for _, n := range notes {
		for _, a := range n.Entry.Anchors {
			if _, ok := index[a.Path]; !ok {
				index[a.Path] = len(paths)
				paths = append(paths, a.Path)
			}
		}
	}
	head, objs, err := catFileAtCommit(root, "HEAD", paths)
	if err != nil {
		head = ""
	}
	for i := range notes {
		n := &notes[i]
		n.Anchors = make([]string, len(n.Entry.Anchors))
		stale, unknown := false, false
		for j, a := range n.Entry.Anchors {
			state := KnowHowUnknown
			if head != "" {
				if o := objs[index[a.Path]]; o.kind == "blob" {
					state = KnowHowStale
					if o.oid == a.Blob {
						state = KnowHowCurrent
					}
				}
			}
			n.Anchors[j] = state
			stale = stale || state == KnowHowStale
			unknown = unknown || state == KnowHowUnknown
		}
		n.Freshness = KnowHowCurrent
		if stale {
			n.Freshness = KnowHowStale
		} else if unknown {
			n.Freshness = KnowHowUnknown
		}
	}
	return head
}

// SortKnowHow orders notes CURRENT, then UNKNOWN, then STALE, newest first
// within a state, then by ticket id and note seq for a deterministic order.
func SortKnowHow(notes []KnowHowNote) {
	rank := map[string]int{KnowHowCurrent: 0, KnowHowUnknown: 1, KnowHowStale: 2}
	sort.SliceStable(notes, func(i, j int) bool {
		a, b := notes[i], notes[j]
		if rank[a.Freshness] != rank[b.Freshness] {
			return rank[a.Freshness] < rank[b.Freshness]
		}
		if a.Entry.RecordedAt != b.Entry.RecordedAt {
			return a.Entry.RecordedAt > b.Entry.RecordedAt
		}
		if a.TicketID != b.TicketID {
			return a.TicketID < b.TicketID
		}
		return a.Entry.Seq.Int() > b.Entry.Seq.Int()
	})
}

// KnowHowNoteValue encodes one note. The compact form a claim delivers keeps
// the text, anchor paths with their states, routes and time; the full form
// adds the pins, provenance and supersession a list reader audits.
func KnowHowNoteValue(n KnowHowNote, compact bool) wire.Value {
	k := n.Entry
	anchors := make([]wire.Value, 0, len(k.Anchors))
	for j, a := range k.Anchors {
		o := wire.NewObject().Set("path", wire.String(a.Path)).Set("freshness", wire.String(n.Anchors[j]))
		if !compact {
			o.Set("blob", wire.String(a.Blob))
		}
		anchors = append(anchors, wire.ObjectValue(o))
	}
	o := wire.NewObject().Set("ticketId", wire.String(n.TicketID)).Set("note", wire.String(string(k.Seq)))
	o.Set("freshness", wire.String(n.Freshness)).Set("text", wire.String(k.Text))
	o.Set("anchors", wire.Array(anchors...)).Set("routes", wire.Strings(append([]string{}, k.Routes...)))
	o.Set("recordedAt", wire.String(string(k.RecordedAt)))
	if !compact {
		o.Set("commit", wire.String(k.Commit))
		o.Set("supersedes", countOrNullValue(k.Supersedes)).Set("reason", strOrNull(k.Reason))
		o.Set("attempt", strOrNull(k.Attempt)).Set("evidencePath", strOrNull(k.EvidencePath))
		gen := wire.Null()
		if k.Generation != nil {
			gen = wire.String(string(*k.Generation))
		}
		o.Set("generation", gen)
		o.Set("actor", wire.ObjectValue(wire.NewObject().Set("id", wire.String(k.ActorID)).Set("role", wire.String(k.ActorRole))))
	}
	return wire.ObjectValue(o)
}

func countOrNullValue(c *wire.Count) wire.Value {
	if c == nil {
		return wire.Null()
	}
	return wire.String(string(*c))
}

func strOrNull(s *string) wire.Value {
	if s == nil {
		return wire.Null()
	}
	return wire.String(*s)
}

// ProjectKnowHow keeps the longest prefix of the ordered notes for which
// envelope(omitted) plus the canonical encoding of the notes array fits
// maxBytes, and returns it with the number left out (KHN-V0-006). envelope
// is the size the enclosing member adds around the array for a given omitted
// count; nil means a bare array. Every prefix is a candidate, so the order the
// reader sees is never reshuffled to fill the cap.
func ProjectKnowHow(notes []KnowHowNote, compact bool, maxBytes int, envelope func(omitted int) int) ([]wire.Value, int) {
	if envelope == nil {
		envelope = func(int) int { return 0 }
	}
	items := []wire.Value{}
	best := 0
	array := len("[]")
	for i, n := range notes {
		v := KnowHowNoteValue(n, compact)
		array += len(wire.Encode(v))
		if i > 0 {
			array++ // the separating comma
		}
		if array > maxBytes {
			break // no longer prefix fits either
		}
		items = append(items, v)
		if array+envelope(len(notes)-len(items)) <= maxBytes {
			best = len(items)
		}
	}
	return items[:best], len(notes) - best
}

// ClaimedKnowHow is what a claim or claim-next delivers from the know-how
// ledgers (KHN-V0-006): the active notes whose anchors intersect the claimed
// ticket's touchPaths, with freshness against the claimant's HEAD. It is
// computed when the response is built, never pinned at admission, so a replay
// shows current freshness. Err keeps an unreadable inventory visible; it never
// fails the committed claim.
type ClaimedKnowHow struct {
	Head  string
	Notes []KnowHowNote
	Err   error
}

func claimKnowHow(repo *intent.Repository, root, ticketID string) ClaimedKnowHow {
	st, err := intent.Load(repo.IntentRoot())
	if err != nil {
		return ClaimedKnowHow{Err: err}
	}
	rec, ok := st.Inventory.Get(ticketID)
	if !ok {
		return ClaimedKnowHow{Err: wire.Errorf(wire.CodeMissingEvidence, "claim", "claimed ticket %s is not in the inventory", ticketID)}
	}
	var c ClaimedKnowHow
	if len(rec.Effects.TouchPaths) == 0 {
		return c
	}
	c.Notes = SelectKnowHow(st.Inventory, rec.Effects.TouchPaths, "")
	if len(c.Notes) > 0 {
		c.Head = ResolveKnowHowFreshness(root, c.Notes)
		SortKnowHow(c.Notes)
	}
	return c
}

type catFileObject struct{ oid, kind string }

// catFileAtCommit answers rev's commit and the object each path names in
// that commit with one `git cat-file --batch-check` in root. It resolves
// rev^{commit} first and asks every path as <commit oid>:<path>, so a
// concurrent commit cannot mix two commits into one answer (KHN-V0-001,
// KHN-V0-005). The commit is "" when rev names no commit; a path Git cannot
// resolve has kind "", so callers read it as absent; a Git failure or a
// short answer is an error.
func catFileAtCommit(root, rev string, paths []string) (string, []catFileObject, error) {
	if root == "" {
		return "", nil, wire.Errorf(wire.CodeUnsupported, "git", "no checkout to observe")
	}
	inR, inW, err := os.Pipe()
	if err != nil {
		return "", nil, gitObservationFailed(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		_, _ = inR.Close(), inW.Close()
		return "", nil, gitObservationFailed(err)
	}
	defer outR.Close()
	c := exec.Command("git", "-c", "credential.helper=", "cat-file", "--batch-check=%(objectname) %(objecttype)")
	c.Dir = root
	c.Env = gitEnvironment()
	c.Stdin, c.Stdout = inR, outW
	err = c.Start()
	// The child holds its own copies; closing ours lets EOF reach both sides.
	_, _ = inR.Close(), outW.Close()
	if err != nil {
		_ = inW.Close()
		return "", nil, gitObservationFailed(err)
	}
	out := bufio.NewReader(outR)
	commit, objs, err := askAtCommit(inW, out, rev, paths)
	_ = inW.Close()
	_, _ = io.Copy(io.Discard, out)
	if werr := c.Wait(); err == nil && werr != nil {
		err = gitObservationFailed(werr)
	}
	if err != nil {
		return "", nil, err
	}
	return commit, objs, nil
}

func askAtCommit(stdin io.WriteCloser, out *bufio.Reader, rev string, paths []string) (string, []catFileObject, error) {
	if _, err := io.WriteString(stdin, rev+"^{commit}\n"); err != nil {
		return "", nil, gitObservationFailed(err)
	}
	first, err := readCatFileObject(out)
	if err != nil || first.kind != "commit" {
		return "", nil, err
	}
	// Git flushes each answer, so the paths are written from a goroutine
	// while the answers are read: neither pipe can fill and block the other.
	written := make(chan error, 1)
	go func() {
		var b strings.Builder
		for _, p := range paths {
			b.WriteString(first.oid + ":" + p + "\n")
		}
		_, err := io.WriteString(stdin, b.String())
		written <- errors.Join(err, stdin.Close())
	}()
	objs := make([]catFileObject, len(paths))
	for i := range paths {
		if objs[i], err = readCatFileObject(out); err != nil {
			_ = stdin.Close()
			<-written
			return "", nil, err
		}
	}
	if err := <-written; err != nil {
		return "", nil, gitObservationFailed(err)
	}
	return first.oid, objs, nil
}

// readCatFileObject reads one answer line. A line that is not "<oid> <type>"
// (for example "<name> missing") is an absent object, never an error.
func readCatFileObject(out *bufio.Reader) (catFileObject, error) {
	line, err := out.ReadString('\n')
	if err != nil {
		return catFileObject{}, wire.Errorf(wire.CodeUnsupported, "git", "git answered short: %v", err)
	}
	oid, kind, ok := strings.Cut(strings.TrimSuffix(line, "\n"), " ")
	if !ok || strings.Contains(kind, " ") {
		return catFileObject{}, nil
	}
	if _, err := wire.ParseOID("oid", oid); err != nil {
		return catFileObject{}, nil
	}
	return catFileObject{oid: oid, kind: kind}, nil
}

func gitObservationFailed(err error) error {
	return wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", err)
}
