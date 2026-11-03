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
	"sync"
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
//
// Format 3 (TCX-V0-845) holds every ticket file, open or closed, each
// admitted only after its record re-encoded to exactly its bytes. Reads still
// trust only closed entries; the native writer and the CLI preload use an
// entry only for bytes whose digest and length equal it (TCX-V0-846, 847).
const (
	closedIndexMagic      = "corvint-tasks-closed-index/3"
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
	// line is the entry's encoded index line, kept from the decode so a
	// rewrite that keeps the entry writes it again byte for byte
	// (TCX-V0-849).
	line string
}

// ClosedIndex is a decoded closed-ticket index.
type ClosedIndex struct {
	queueID   string
	policySha wire.Digest
	captured  int64
	entries   map[string]*ClosedEntry // tickets/<local>.json
	// body is the decoded file's entry lines, so a rewrite whose lines are
	// unchanged leaves the file as it is (TCX-V0-849).
	body []byte
}

// trusted returns the entry for rel when its stat key still matches and its
// modification time is outside the racy window of the capture. A read trusts
// only a closed entry; the index rewrite, which never serves a record, also
// keeps an open one (TCX-V0-845).
func (idx *ClosedIndex) trusted(rel string, key statKey, open bool) *ClosedEntry {
	if idx == nil {
		return nil
	}
	e, ok := idx.entries[rel]
	if !ok || e.Size != key.size || e.mtime != key.mtime || key.mtime >= idx.captured-int64(closedIndexRacyWindow) {
		return nil
	}
	if !open && !ticket.Closed(e.Summary.Status) {
		return nil
	}
	return e
}

// WriterSummaries returns the entries by digest when idx governs queueID
// and policySha, and nil otherwise. A caller may hold a ticket as its entry's
// summary only for bytes whose SHA-256 and length equal the entry's
// (TCX-V0-846).
func (idx *ClosedIndex) WriterSummaries(queueID string, policySha wire.Digest) map[wire.Digest]*ClosedEntry {
	if idx == nil || idx.queueID != queueID || idx.policySha != policySha {
		return nil
	}
	out := make(map[wire.Digest]*ClosedEntry, len(idx.entries))
	for _, e := range idx.entries {
		out[e.Sha] = e
	}
	return out
}

// LoadSummarized loads the store from a full capture, holding a ticket as
// its idx entry's summary when its bytes' SHA-256 and length equal the
// entry's and decoding it from those bytes on first Get (TCX-V0-847). A nil
// idx, or one that does not govern the tree, is the full load.
func LoadSummarized(primaryWorktree string, idx *ClosedIndex) (*Store, error) {
	tree, err := TreeDigest(primaryWorktree)
	if err != nil {
		return nil, err
	}
	return LoadSummarizedTree(primaryWorktree, tree, idx)
}

// LoadSummarizedTree is LoadSummarized over a tree the caller captured with
// TreeDigest (every file's bytes in hand), so a caller that already holds
// the capture (the claim's know-how read, TCX-V0-975) does not read the
// store again.
func LoadSummarizedTree(primaryWorktree string, tree Tree, idx *ClosedIndex) (*Store, error) {
	if idx == nil || !idx.governs(tree) {
		return LoadTree(primaryWorktree, tree, "")
	}
	files := append([]File(nil), tree.Files...)
	held := map[string][]byte{}
	for i, f := range files {
		e, ok := idx.entries[f.Path]
		if !ok || !strings.HasPrefix(f.Path, TicketsDir+"/") || e.Sha != f.Sha256 || e.Size != int64(len(f.Raw)) {
			continue
		}
		held[f.Path] = f.Raw
		files[i].Raw = nil
	}
	summarized := tree
	summarized.Files = files
	st, err := decodeTreeHeld(filepath.Join(primaryWorktree, Dir), summarized, idx, held)
	if err != nil {
		return nil, err
	}
	st.Tree = tree
	return st, nil
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
	// A parse is reused only for byte-identical file content, whatever its
	// timestamps say (TCX-V0-849).
	sum := sha256.Sum256(raw)
	if idx := parsedIndex(path, sum); idx != nil {
		return idx
	}
	idx, err := decodeClosedIndex(raw)
	if err != nil {
		return nil
	}
	rememberIndex(path, sum, idx)
	return idx
}

// parsedIndexes holds, in process memory, the last parse of each index path
// with the SHA-256 of the bytes it parsed (TCX-V0-849). A parsed index is
// never changed after its decode, so callers share it.
var (
	parsedMu      sync.Mutex
	parsedIndexes = map[string]parsedIndexEntry{}
)

type parsedIndexEntry struct {
	sum [sha256.Size]byte
	idx *ClosedIndex
}

func parsedIndex(path string, sum [sha256.Size]byte) *ClosedIndex {
	parsedMu.Lock()
	defer parsedMu.Unlock()
	if e, ok := parsedIndexes[path]; ok && e.sum == sum {
		return e.idx
	}
	return nil
}

func rememberIndex(path string, sum [sha256.Size]byte, idx *ClosedIndex) {
	parsedMu.Lock()
	defer parsedMu.Unlock()
	if len(parsedIndexes) >= 16 {
		clear(parsedIndexes)
	}
	parsedIndexes[path] = parsedIndexEntry{sum: sum, idx: idx}
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
		line := idx.entries[rel].line
		if line == "" {
			var ok bool
			if line, ok = encodeEntry(rel, idx.entries[rel]); !ok {
				continue
			}
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	idx.body = body.Bytes()
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
	idx := &ClosedIndex{queueID: head[1], policySha: wire.Digest(head[2]), captured: captured, entries: map[string]*ClosedEntry{}, body: body}
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
		e.line = strings.Join(f, sepField)
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
	if err1 != nil || err2 != nil || err3 != nil || f[0] != TicketsDir+"/"+id.Local+".json" || f[5] == "" {
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
// applies to every ticket, from its held bytes when it has them and
// otherwise from the file, re-hashed against the captured digest.
func closedLoader(root string, shas map[string]wire.Digest, held map[string][]byte) ticket.Loader {
	return func(s *ticket.Summary) (*ticket.Record, error) {
		rel := TicketsDir + "/" + s.TicketID.Local + ".json"
		raw, ok := held[rel]
		if !ok {
			var err error
			if raw, err = readIndexed(root, rel, shas[rel]); err != nil {
				return nil, err
			}
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

// WriteClosedIndex refreshes the ticket index of a repository after a write
// (TCX-V0-984, TCX-V0-845): entries whose stat key still matches are kept,
// and every other ticket is read, decoded and validated as a full load does,
// and admitted only when its record re-encodes to exactly its bytes. The
// file is replaced by rename. A store that does not load, or an index that
// cannot be written, leaves the readers' full scan in place.
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
	// Only the files read here are decoded and checked, as a full load checks
	// them; a kept entry is not decoded, and no inventory is built
	// (TCX-V0-849).
	byPath := map[string]File{}
	for _, f := range tree.Files {
		byPath[f.Path] = f
	}
	queueFile, policyFile := byPath[QueueFile], byPath[PolicyFile]
	if queueFile.Raw == nil || policyFile.Raw == nil {
		return wire.Errorf(wire.CodeUninitialized, filepath.Join(primaryWorktree, Dir), "queue.json or policy.json is absent")
	}
	q, err := DecodeQueue(queueFile.Raw)
	if err != nil {
		return prefix(err, QueueFile)
	}
	p, err := DecodePolicy(policyFile.Raw)
	if err != nil {
		return prefix(err, PolicyFile)
	}
	gateIDs := p.GateIDs()
	idx := &ClosedIndex{queueID: q.QueueID.Raw, policySha: policyFile.Sha256, captured: captured, entries: map[string]*ClosedEntry{}}
	for _, f := range tree.Files {
		if !strings.HasPrefix(f.Path, TicketsDir+"/") {
			continue
		}
		key, ok := keys[f.Path]
		if !ok || key.mtime >= captured-int64(closedIndexRacyWindow) {
			continue
		}
		if f.Raw == nil {
			// Taken from the old index by its stat key: kept with its line.
			if old == nil {
				continue
			}
			if kept, ok := old.entries[f.Path]; ok && kept.Size == key.size && kept.mtime == key.mtime && kept.Sha == f.Sha256 {
				idx.entries[f.Path] = kept
			}
			continue
		}
		if wire.Sum(f.Raw) != f.Sha256 {
			return wire.Errorf(wire.CodeSnapshotMoved, filepath.Join(primaryWorktree, Dir, f.Path), "captured bytes do not match the tree digest")
		}
		rec, err := decodeTicketFile(f, q, gateIDs)
		if err != nil {
			return err
		}
		// A read file joins only when its record re-encodes to its bytes.
		if !bytes.Equal(reencode(rec), f.Raw) {
			continue
		}
		idx.entries[f.Path] = &ClosedEntry{Size: key.size, mtime: key.mtime, Sha: f.Sha256, Summary: ticket.SummaryOf(rec)}
	}
	encoded := encodeClosedIndex(idx)
	// Unchanged lines for the same queue and policy leave the file as it is,
	// with the capture time its entries were admitted under (TCX-V0-849).
	if old != nil && old.queueID == idx.queueID && old.policySha == idx.policySha && bytes.Equal(old.body, idx.body) {
		return nil
	}
	return writeFileAtomic(ClosedIndexPath(stateDir), encoded)
}

// reencode is the re-encoding an index entry's bytes must equal; tests
// replace it to show a non-canonical record is left out (TCX-V0-845).
var reencode = (*ticket.Record).Encode

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
