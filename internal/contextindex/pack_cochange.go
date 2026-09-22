package contextindex

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
)

// The cochange section carries the recent non-merge history the co-change
// slot counts over (readCoChangeHistory), so a subject hit does not spawn
// `git log`. In a complete repository it is a pure function of the HEAD
// commit, so the pack names that commit in its header and a reader whose HEAD differs (the same tree after
// an amend, say) ignores the section and spawns as before; every other
// section is still served. Layout: [u32 commits][u32 total paths][u32 commit,
// tree, subject string ids x n each][u32 row offsets x n+1][u32 path ids x
// total], a CSR over the string table.
func cochangeSection(history []historyEntry, table *packStrings) sectionWriter {
	n := len(history)
	commits, trees, subjects := make([]uint32, n), make([]uint32, n), make([]uint32, n)
	rows := make([]uint32, 0, n+1)
	paths := make([]uint32, 0, 8*n)
	for index, entry := range history {
		commits[index], trees[index], subjects[index] = table.id(entry.commit), table.id(entry.tree), table.id(entry.subject)
		rows = append(rows, uint32(len(paths)))
		paths = append(paths, stringIDs(table, entry.paths)...)
	}
	rows = append(rows, uint32(len(paths)))
	return u32Section(packSectionCochange, []uint32{uint32(n), uint32(len(paths))}, commits, trees, subjects, rows, paths)
}

func decodeCochangeSection(data []byte, strings packStringTable) ([]historyEntry, error) {
	if len(data) < 8 {
		return nil, errors.New("pack cochange header is truncated")
	}
	n, total := uint64(binary.LittleEndian.Uint32(data)), uint64(binary.LittleEndian.Uint32(data[4:]))
	offset := uint64(8)
	var columns [4][]uint32
	for index, count := range [4]uint64{n, n, n, n + 1} {
		array, next, err := u32Array(data, offset, count)
		if err != nil {
			return nil, err
		}
		columns[index], offset = array, next
	}
	paths, offset, err := u32Array(data, offset, total)
	if err != nil {
		return nil, err
	}
	if offset != uint64(len(data)) {
		return nil, errors.New("pack cochange section has trailing bytes")
	}
	rows := columns[3]
	entries := make([]historyEntry, 0, n)
	for index := uint64(0); index < n; index++ {
		low, high := rows[index], rows[index+1]
		if low > high || uint64(high) > total {
			return nil, errors.New("pack cochange rows are not ascending within the paths")
		}
		entry := historyEntry{paths: make([]string, 0, high-low)}
		for _, id := range paths[low:high] {
			path, err := strings.get(id)
			if err != nil {
				return nil, err
			}
			entry.paths = append(entry.paths, path)
		}
		if entry.commit, err = strings.get(columns[0][index]); err != nil {
			return nil, err
		}
		if entry.tree, err = strings.get(columns[1][index]); err != nil {
			return nil, err
		}
		if entry.subject, err = strings.get(columns[2][index]); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// cochange decodes the section once per retained mapping: a later read of
// the same pack registers the entries this one decoded. The strings they
// hold alias the mapping, which outlives every index built over it.
func (f *packFile) cochange(data []byte, strings packStringTable) ([]historyEntry, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if f.history != nil {
		return f.history, nil
	}
	history, err := decodeCochangeSection(data, strings)
	if err != nil {
		return nil, err
	}
	f.history = history
	return history, nil
}

// packHistoryCapacity bounds the registrations one process retains. A
// registration pins its index, and a reader holds one or two loaded indexes
// at a time, so a small ring covers the live ones.
const packHistoryCapacity = 8

// packHistories maps a pack-loaded Index to the history its cochange
// section held for the reader's HEAD commit. readCoChangeHistory consults
// it before spawning; an index absent here (a gob hit, a build, a pack whose
// commit differed, or a registration the ring has since evicted) spawns
// exactly as before.
var packHistories = packHistoryRegistry{entries: map[*Index][]historyEntry{}}

// packHistoryRegistry retains at most packHistoryCapacity registrations and
// drops the oldest first.
type packHistoryRegistry struct {
	mutex   sync.Mutex
	entries map[*Index][]historyEntry
	ring    [packHistoryCapacity]*Index
	next    int
}

func registerPackHistory(index *Index, history []historyEntry) {
	packHistories.mutex.Lock()
	defer packHistories.mutex.Unlock()
	if _, ok := packHistories.entries[index]; ok {
		return
	}
	delete(packHistories.entries, packHistories.ring[packHistories.next])
	packHistories.entries[index] = history
	packHistories.ring[packHistories.next] = index
	packHistories.next = (packHistories.next + 1) % packHistoryCapacity
}

// forgetPackHistory drops index's registration and its ring slot, so a
// losing first read's index is not kept reachable until the slot is reused.
func forgetPackHistory(index *Index) {
	packHistories.mutex.Lock()
	defer packHistories.mutex.Unlock()
	delete(packHistories.entries, index)
	for slot, held := range packHistories.ring {
		if held == index {
			packHistories.ring[slot] = nil
		}
	}
}

func packHistory(index *Index) ([]historyEntry, bool) {
	packHistories.mutex.Lock()
	defer packHistories.mutex.Unlock()
	history, ok := packHistories.entries[index]
	return history, ok
}

// packHistoryForWrite reads the history the pack will carry and the commit
// it is keyed by. The identity is read on both sides of the log, so a HEAD
// that moved or a history cut that appeared or vanished during the read
// yields no section rather than one keyed to the wrong walk. A cut history (a
// shallow repository or a grafts file) yields no section: removing the cut
// keeps HEAD but changes what `git log HEAD` returns, and the section would
// then serve the old walk. A reader under a cut ignores the section for the
// same reason.
func packHistoryForWrite(ctx context.Context, index *Index) ([]historyEntry, string) {
	before, err := readIdentity(ctx, index.Root)
	if err != nil || before.historyCut || before.commitRevision != index.CommitRevision || before.treeRevision != index.Revision {
		return nil, ""
	}
	history, err := readCoChangeHistory(ctx, index)
	if err != nil {
		return nil, ""
	}
	after, err := readIdentity(ctx, index.Root)
	if err != nil || after != before {
		return nil, ""
	}
	return history, index.CommitRevision
}
