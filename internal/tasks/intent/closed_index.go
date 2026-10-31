// SPDX-License-Identifier: Apache-2.0

package intent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The closed-ticket index (TCX-V0-980..985) is derived, rebuildable state
// beside the journal state directory. It lets an OPEN-path read take the
// digest and summary of a COMPLETED or ARCHIVED ticket from the file's stat
// key instead of reading and decoding it. It is never authority: an entry
// is trusted only while the file's size and modification time match it and
// that time predates the index capture by closedIndexRacyWindow, and any
// header, checksum or decode mismatch discards the whole index, so the read
// falls back to the full scan.
const (
	closedIndexMagic      = "corvint-tasks-closed-index/2"
	closedIndexSuffix     = ".closed-index"
	closedIndexRacyWindow = 2 * time.Second
	closedIndexMaxBytes   = wire.MaxIntentTreeBytes
)

// ClosedIndexPath is the index file of a state directory: a sibling, like
// the journal checkpoint, so it is never journal state.
func ClosedIndexPath(stateDir string) string { return stateDir + closedIndexSuffix }

type statKey struct{ size, mtime int64 }

func statKeyOf(info os.FileInfo) statKey {
	return statKey{size: info.Size(), mtime: info.ModTime().UnixNano()}
}

// ClosedEntry is one closed ticket file of the index.
type ClosedEntry struct {
	Size    int64
	mtime   int64
	Sha     wire.Digest
	Summary *ticket.Summary
}

// ClosedIndex is a decoded closed-ticket index.
type ClosedIndex struct {
	queueID   string
	policySha wire.Digest
	captured  int64
	entries   map[string]*ClosedEntry // tickets/<local>.json
}

// trusted returns the entry for rel when its stat key still matches and its
// modification time is outside the racy window of the capture.
func (idx *ClosedIndex) trusted(rel string, key statKey) *ClosedEntry {
	if idx == nil {
		return nil
	}
	e, ok := idx.entries[rel]
	if !ok || e.Size != key.size || e.mtime != key.mtime || key.mtime >= idx.captured-int64(closedIndexRacyWindow) {
		return nil
	}
	return e
}

// governs reports whether idx was captured for tree's queue and policy; an
// index of another queue or policy vouches for no entry (TCX-V0-983). A nil
// idx governs every tree.
func (idx *ClosedIndex) governs(tree Tree) bool {
	if idx == nil {
		return true
	}
	for _, f := range tree.Files {
		switch f.Path {
		case PolicyFile:
			if f.Sha256 != idx.policySha {
				return false
			}
		case QueueFile:
			q, err := DecodeQueue(f.Raw)
			if err != nil || q.QueueID.Raw != idx.queueID {
				return false
			}
		}
	}
	return true
}

// ReadClosedIndex reads the index of stateDir. Any absence, bound, header,
// checksum or decode failure returns nil, and the reader scans in full. It
// never writes.
func ReadClosedIndex(stateDir string) *ClosedIndex {
	path := ClosedIndexPath(stateDir)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > closedIndexMaxBytes {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	idx, err := decodeClosedIndex(raw)
	if err != nil {
		return nil
	}
	return idx
}

const (
	sepField = "\t"
	sepList  = "\x1f"
	sepPart  = "\x1e"
)

func safeField(s string) bool { return !strings.ContainsAny(s, "\t\n\x1e\x1f") }

func optField(p *string) string {
	if p == nil {
		return ""
	}
	return "=" + *p
}

func parseOpt(s string) *string {
	if s == "" {
		return nil
	}
	v := s[1:]
	return &v
}

// encodeEntry is one index line, or false when a field cannot be encoded.
func encodeEntry(rel string, e *ClosedEntry) (string, bool) {
	s := e.Summary
	strs := []string{rel, s.TicketID.Raw, s.Status, s.Priority, s.Kind, string(s.UpdatedAt)}
	for _, p := range []*string{s.ArchivedFrom, s.Owner, s.Milestone} {
		if p != nil {
			strs = append(strs, *p)
		}
	}
	strs = append(strs, s.Labels...)
	strs = append(strs, s.RequiredGates...)
	deps := make([]string, 0, len(s.Dependencies))
	for _, d := range s.Dependencies {
		strs = append(strs, d.TicketID.Raw, d.Obligation)
		gate := optField(d.GateID)
		if d.GateID != nil {
			strs = append(strs, *d.GateID)
		}
		deps = append(deps, d.TicketID.Raw+sepPart+d.Obligation+sepPart+gate)
	}
	for _, v := range strs {
		if !safeField(v) {
			return "", false
		}
	}
	for _, l := range append(append([]string(nil), s.Labels...), s.RequiredGates...) {
		if l == "" {
			return "", false
		}
	}
	completed := ""
	if s.CompletedAt != nil {
		completed = "=" + string(*s.CompletedAt)
	}
	return strings.Join([]string{rel, strconv.FormatInt(e.Size, 10), strconv.FormatInt(e.mtime, 10), string(e.Sha),
		s.TicketID.Raw, s.Status, optField(s.ArchivedFrom), s.Priority, string(s.Order), string(s.AcceptanceRevision), s.Kind,
		optField(s.Owner), optField(s.Milestone), strings.Join(s.Labels, sepList), strings.Join(deps, sepList),
		string(s.UpdatedAt), completed, strconv.Quote(s.Title), strings.Join(s.RequiredGates, sepList)}, sepField), true
}

const closedEntryFields = 19

func encodeClosedIndex(idx *ClosedIndex) []byte {
	rels := make([]string, 0, len(idx.entries))
	for rel := range idx.entries {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	var body bytes.Buffer
	for _, rel := range rels {
		line, ok := encodeEntry(rel, idx.entries[rel])
		if !ok {
			continue
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	sum := sha256.Sum256(body.Bytes())
	var out bytes.Buffer
	out.WriteString(strings.Join([]string{closedIndexMagic, idx.queueID, string(idx.policySha),
		strconv.FormatInt(idx.captured, 10), hex.EncodeToString(sum[:])}, sepField))
	out.WriteByte('\n')
	out.Write(body.Bytes())
	return out.Bytes()
}

var errClosedIndex = wire.Errorf(wire.CodeMalformed, "closed-index", "closed-ticket index does not decode")

func decodeClosedIndex(raw []byte) (*ClosedIndex, error) {
	nl := bytes.IndexByte(raw, '\n')
	if nl < 0 {
		return nil, errClosedIndex
	}
	head := strings.Split(string(raw[:nl]), sepField)
	body := raw[nl+1:]
	if len(head) != 5 || head[0] != closedIndexMagic {
		return nil, errClosedIndex
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != head[4] {
		return nil, errClosedIndex
	}
	captured, err := strconv.ParseInt(head[3], 10, 64)
	if err != nil {
		return nil, errClosedIndex
	}
	idx := &ClosedIndex{queueID: head[1], policySha: wire.Digest(head[2]), captured: captured, entries: map[string]*ClosedEntry{}}
	for len(body) > 0 {
		nl := bytes.IndexByte(body, '\n')
		if nl < 0 {
			return nil, errClosedIndex
		}
		f := strings.Split(string(body[:nl]), sepField)
		body = body[nl+1:]
		e, err := decodeEntry(f)
		if err != nil {
			return nil, err
		}
		if _, dup := idx.entries[f[0]]; dup {
			return nil, errClosedIndex
		}
		idx.entries[f[0]] = e
	}
	return idx, nil
}

func decodeEntry(f []string) (*ClosedEntry, error) {
	if len(f) != closedEntryFields {
		return nil, errClosedIndex
	}
	size, err1 := strconv.ParseInt(f[1], 10, 64)
	mtime, err2 := strconv.ParseInt(f[2], 10, 64)
	id, err3 := wire.ParseTicketID("", f[4])
	if err1 != nil || err2 != nil || err3 != nil || f[0] != TicketsDir+"/"+id.Local+".json" || !ticket.Closed(f[5]) {
		return nil, errClosedIndex
	}
	s := &ticket.Summary{TicketID: id, Status: f[5], ArchivedFrom: parseOpt(f[6]), Priority: f[7], Order: wire.Count(f[8]),
		AcceptanceRevision: wire.Count(f[9]), Kind: f[10], Owner: parseOpt(f[11]), Milestone: parseOpt(f[12]), UpdatedAt: wire.Timestamp(f[15])}
	if f[13] != "" {
		s.Labels = strings.Split(f[13], sepList)
	}
	if f[14] != "" {
		for _, d := range strings.Split(f[14], sepList) {
			parts := strings.Split(d, sepPart)
			if len(parts) != 3 {
				return nil, errClosedIndex
			}
			dep, err := wire.ParseTicketID("", parts[0])
			if err != nil {
				return nil, errClosedIndex
			}
			s.Dependencies = append(s.Dependencies, ticket.Dependency{TicketID: dep, Obligation: parts[1], GateID: parseOpt(parts[2])})
		}
	}
	if f[16] != "" {
		at := wire.Timestamp(f[16][1:])
		s.CompletedAt = &at
	}
	title, err := strconv.Unquote(f[17])
	if err != nil {
		return nil, errClosedIndex
	}
	s.Title = title
	if f[18] != "" {
		s.RequiredGates = strings.Split(f[18], sepList)
	}
	return &ClosedEntry{Size: size, mtime: mtime, Sha: wire.Digest(f[3]), Summary: s}, nil
}

// LoadTreeIndexed is LoadTree over a tree TreeDigestIndexed captured with
// idx: every file it read is decoded and checked as LoadTree does, and every
// closed ticket it took from idx is held as a summary whose record is read,
// re-hashed against the indexed digest and decoded on first Get
// (TCX-V0-982). An index written for another queue or policy is not
// trusted: those files are read and decoded here, as in the full scan.
// Store.Tickets then holds only the records decoded here.
func LoadTreeIndexed(primaryWorktree string, tree Tree, want wire.Digest, idx *ClosedIndex) (*Store, error) {
	if want != "" && tree.Sha256 != want {
		return nil, wire.Errorf(wire.CodeSnapshotMoved, filepath.Join(primaryWorktree, Dir), "intent tree digest %s differs from the pinned snapshot %s", tree.Sha256, want)
	}
	return decodeTreeIndexed(filepath.Join(primaryWorktree, Dir), tree, idx)
}

// readIndexed reads one indexed file and proves it still has the digest the
// tree recorded for it.
func readIndexed(root, rel string, sha wire.Digest) ([]byte, error) {
	full := filepath.Join(root, filepath.FromSlash(rel))
	r, err := safeopen.Root(root)
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, root, "cannot open the intent store: %v", err)
	}
	defer r.Close()
	raw, err := readInRoot(r, full, rel, wire.MaxTicketFileBytes)
	if err != nil {
		return nil, err
	}
	if wire.Sum(raw) != sha {
		return nil, wire.Errorf(wire.CodeSnapshotMoved, full, "closed ticket bytes differ from the captured tree digest")
	}
	return raw, nil
}

// closedLoader decodes a lazily held record with the checks decodeTree
// applies to every ticket.
func closedLoader(root string, shas map[string]wire.Digest) ticket.Loader {
	return func(s *ticket.Summary) (*ticket.Record, error) {
		rel := TicketsDir + "/" + s.TicketID.Local + ".json"
		raw, err := readIndexed(root, rel, shas[rel])
		if err != nil {
			return nil, err
		}
		rec, err := ticket.Decode(raw)
		if err != nil {
			return nil, prefix(err, rel)
		}
		if rec.TicketID.Raw != s.TicketID.Raw || rec.Status != s.Status {
			return nil, wire.Errorf(wire.CodeSnapshotMoved, filepath.Join(root, rel), "closed ticket differs from its index summary")
		}
		return rec, nil
	}
}

// WriteClosedIndex refreshes the closed-ticket index of a repository after
// a write (TCX-V0-984): entries whose stat key still matches are kept, and
// every other closed ticket is read, decoded and validated as a full load
// does. The file is replaced by rename. A store that does not load, or an
// index that cannot be written, leaves the readers' full scan in place.
func WriteClosedIndex(primaryWorktree, stateDir string) error {
	old := ReadClosedIndex(stateDir)
	captured := time.Now().UnixNano()
	keys := map[string]statKey{}
	tree, err := treeDigest(primaryWorktree, old, keys)
	if err == nil && !old.governs(tree) {
		old = nil
		tree, err = treeDigest(primaryWorktree, nil, keys)
	}
	if err != nil {
		return err
	}
	st, err := LoadTreeIndexed(primaryWorktree, tree, "", old)
	if err != nil {
		return err
	}
	idx := &ClosedIndex{queueID: st.Queue.QueueID.Raw, policySha: st.Digests[PolicyFile], captured: captured, entries: map[string]*ClosedEntry{}}
	for _, id := range st.Inventory.IDs() {
		s, _ := st.Inventory.Summary(id)
		if !ticket.Closed(s.Status) {
			continue
		}
		rel := TicketsDir + "/" + s.TicketID.Local + ".json"
		key, ok := keys[rel]
		if !ok || key.mtime >= captured-int64(closedIndexRacyWindow) {
			continue
		}
		idx.entries[rel] = &ClosedEntry{Size: key.size, mtime: key.mtime, Sha: st.Digests[rel], Summary: s}
	}
	return writeFileAtomic(ClosedIndexPath(stateDir), encodeClosedIndex(idx))
}

func writeFileAtomic(path string, raw []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
