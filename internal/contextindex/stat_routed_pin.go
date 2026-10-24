package contextindex

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// statRoutedPinKey marks a build whose worktree pin pass routes the entries
// Git's index vouches for to the committed blob (DCR-V1-452). Only
// BuildRevisionContext sets it: the corpus pins an explicit revision, without
// a status read, for every revision it names and again after compiling, so
// each of those passes opened every tracked file. Build, BuildContext and
// BuildEval keep their clean-repository-reads-no-blob construction.
type statRoutedPinKey struct{}

func withStatRoutedPins(ctx context.Context) context.Context {
	return context.WithValue(ctx, statRoutedPinKey{}, true)
}

func statRoutedPins(ctx context.Context) bool {
	routed, _ := ctx.Value(statRoutedPinKey{}).(bool)
	return routed
}

// gitIndexStat is the stat data Git's index records for a path, each field
// truncated to 32 bits as the index stores it.
type gitIndexStat struct {
	ctimeSec, ctimeNsec, mtimeSec, mtimeNsec uint32
	ino, mode, uid, gid, size                uint32
}

// gitIndex is the stage-0 stat data of one index file and the index file's
// own modification time, the racy-clean boundary.
type gitIndex struct {
	entries             map[string]gitIndexStat
	mtimeSec, mtimeNsec uint32
}

// maxGitIndexBytes bounds the index read; a larger index vouches for nothing.
const maxGitIndexBytes = 256 << 20

// maxGitIndexPathBytes bounds one decoded index pathname (PATH_MAX), and
// maxGitIndexNameExpansion bounds all decoded pathnames together as a multiple
// of the index size, so a version 4 index's prefix compression cannot expand
// into unbounded memory. An index past either bound vouches for nothing.
const (
	maxGitIndexPathBytes     = 4096
	maxGitIndexNameExpansion = 8
)

// gitIndexExtendedFlags are the extended entry flags Git defines
// (skip-worktree and intent-to-add); Git refuses an index setting any other.
const gitIndexExtendedFlags = 0x6000

// pinStatVouched pins every eligible entry Git's index vouches for from the
// committed blob, in one batched object read, and marks it in done; it opens
// no worktree file. An entry is vouched when its index record is stage 0,
// carries no assume-valid, skip-worktree or intent-to-add flag, and its
// worktree file's lstat matches the record's size, modification and change
// times, inode, owner and executable bit, with the modification time in a
// whole second strictly before the index file's own (Git's racy-clean rule,
// with a one-second window). Such a file holds
// exactly the blob the index names, so opening it could at best repeat a
// committed object, and the blob read here is the tree entry's own: it is
// immutable content, which is why nothing about the worktree is trusted.
// Vouching therefore selects only the source; an eligible entry's pinned
// bytes are its tree blob's whichever source supplies them. An
// unreadable, unsupported or corrupt index, a stat mismatch, a racy entry or a
// failed batch leaves the entry to the hashed worktree pass (DCR-V1-452,
// DCR-V1-453).
func pinStatVouched(ctx context.Context, root string, identity repositoryIdentity, entries []treeEntry, dirty map[string]struct{}, pinned []pinnedEntry, done []bool) {
	vouched := statVouchedEntries(ctx, root, identity, entries, dirty)
	needed := make([]treeEntry, 0, len(entries))
	for position, entry := range entries {
		if vouched[position] {
			needed = append(needed, entry)
		}
	}
	if len(needed) == 0 {
		return
	}
	batch := fetchVouchedBlobs(ctx, root, entries, needed)
	if batch.err != nil {
		return
	}
	for position, entry := range entries {
		if !vouched[position] {
			continue
		}
		blob, ok := batch.blobs[entry.oid]
		if !ok {
			continue
		}
		pinned[position] = pinnedFrom(entry, blob)
		done[position] = true
	}
}

// fetchVouchedBlobs is the vouched entries' batched blob read; tests replace
// it to fail the batch.
var fetchVouchedBlobs = fetchResidualBlobs

// statVouchedEntries reports which eligible entries Git's index vouches for
// (see pinStatVouched). The lstat runs on the plain path: a symlinked parent
// can only make an entry vouched, and a vouched entry is pinned from the
// object store, never from the file the path reaches.
func statVouchedEntries(ctx context.Context, root string, identity repositoryIdentity, entries []treeEntry, dirty map[string]struct{}) []bool {
	vouched := make([]bool, len(entries))
	index, ok := readGitIndexFor(ctx, root, identity.objectFormat)
	if !ok {
		return vouched
	}
	for position, entry := range entries {
		if !eligibleForWorktreePin(entry, dirty) {
			continue
		}
		recorded, ok := index.entries[entry.path]
		if !ok {
			continue
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(entry.path)))
		if err != nil {
			continue
		}
		vouched[position] = index.vouches(recorded, info)
	}
	return vouched
}

// vouches applies the stat comparison and the racy-clean rule to one entry.
func (index *gitIndex) vouches(recorded gitIndexStat, info os.FileInfo) bool {
	if !info.Mode().IsRegular() {
		return false
	}
	live, ok := fileIndexStat(info)
	if !ok {
		return false
	}
	executable := info.Mode().Perm()&0o100 != 0
	if executable != (recorded.mode == 0o100755) || recorded.mode != 0o100644 && recorded.mode != 0o100755 {
		return false
	}
	if uint32(info.Size()) != recorded.size || live.ctimeSec != recorded.ctimeSec || live.ctimeNsec != recorded.ctimeNsec ||
		live.mtimeSec != recorded.mtimeSec || live.mtimeNsec != recorded.mtimeNsec || live.ino != recorded.ino ||
		live.uid != recorded.uid || live.gid != recorded.gid {
		return false
	}
	// The racy window is a whole second: a filesystem or Git build that
	// records only seconds cannot order two writes within the same second.
	return recorded.mtimeSec < index.mtimeSec
}

// readGitIndexFor locates the checkout's index through Git, which honours
// linked worktrees and GIT_INDEX_FILE, and reads it.
func readGitIndexFor(ctx context.Context, root, objectFormat string) (*gitIndex, bool) {
	raw, err := git(ctx, root, maxIdentityBytes, nil, "rev-parse", "--git-path", "index")
	if err != nil {
		return nil, false
	}
	path := strings.TrimSuffix(string(raw), "\n")
	if path == "" || strings.ContainsAny(path, "\n\x00") {
		return nil, false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return readGitIndex(path, objectFormat)
}

// readGitIndex opens the index without following a symlink or blocking, and
// parses it within maxGitIndexBytes.
func readGitIndex(path, objectFormat string) (*gitIndex, bool) {
	file, err := openGitIndex(path)
	if err != nil {
		return nil, false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxGitIndexBytes {
		return nil, false
	}
	live, ok := fileIndexStat(info)
	if !ok {
		return nil, false
	}
	data := make([]byte, info.Size()+1)
	count, err := io.ReadFull(file, data)
	if err != io.ErrUnexpectedEOF || int64(count) != info.Size() {
		return nil, false
	}
	index, ok := parseGitIndex(data[:count], objectFormat)
	if !ok {
		return nil, false
	}
	index.mtimeSec, index.mtimeNsec = live.mtimeSec, live.mtimeNsec
	return index, true
}

// parseGitIndex reads the stage-0 entries of a version 2, 3 or 4 index whose
// trailing checksum verifies. Any other version, a malformed entry, an unknown
// extended flag, a pathname past maxGitIndexPathBytes, decoded pathnames past
// maxGitIndexNameExpansion times the index size, a split index (`link`) or an
// unknown required extension refuses the whole file. A
// path with a conflict stage or a flag that stops Git checking its stat is
// left out, so it is never vouched.
func parseGitIndex(data []byte, objectFormat string) (*gitIndex, bool) {
	hashSize := 20
	if objectFormat == "sha256" {
		hashSize = 32
	} else if objectFormat != "sha1" {
		return nil, false
	}
	if len(data) < 12+hashSize || string(data[:4]) != "DIRC" {
		return nil, false
	}
	body, trailer := data[:len(data)-hashSize], data[len(data)-hashSize:]
	// An all-zero trailer is Git's index.skipHash mode. Vouching only selects
	// where a pinned blob is read from, never what it is, so such an index is
	// parsed with the same bounds instead of being refused.
	if !bytes.Equal(trailer, make([]byte, hashSize)) && !indexChecksumMatches(body, trailer, objectFormat) {
		return nil, false
	}
	version := binary.BigEndian.Uint32(data[4:8])
	if version < 2 || version > 4 {
		return nil, false
	}
	count := binary.BigEndian.Uint32(data[8:12])
	fixed := 40 + hashSize + 2
	if uint64(count) > uint64(len(body))/uint64(fixed) {
		return nil, false
	}
	entries := make(map[string]gitIndexStat, count)
	excluded := map[string]struct{}{}
	offset := 12
	previous := []byte{}
	nameBudget := uint64(len(data)) * maxGitIndexNameExpansion
	for range count {
		if offset+fixed > len(body) {
			return nil, false
		}
		entry := body[offset:]
		field := func(at int) uint32 { return binary.BigEndian.Uint32(entry[at : at+4]) }
		stat := gitIndexStat{
			ctimeSec: field(0), ctimeNsec: field(4), mtimeSec: field(8), mtimeNsec: field(12),
			ino: field(20), mode: field(24), uid: field(28), gid: field(32), size: field(36),
		}
		flags := binary.BigEndian.Uint16(entry[40+hashSize:])
		cursor := fixed
		trusted := flags&0x8000 == 0 && flags&0x3000 == 0
		if flags&0x4000 != 0 {
			if version < 3 || offset+cursor+2 > len(body) {
				return nil, false
			}
			extended := binary.BigEndian.Uint16(entry[cursor:])
			if extended&^gitIndexExtendedFlags != 0 {
				return nil, false
			}
			trusted = trusted && extended == 0
			cursor += 2
		}
		var name []byte
		if version == 4 {
			strip, width, ok := indexVarint(entry[cursor:])
			if !ok || strip > uint64(len(previous)) {
				return nil, false
			}
			cursor += width
			end := bytes.IndexByte(entry[cursor:], 0)
			if end < 0 {
				return nil, false
			}
			length := len(previous) - int(strip) + end
			if length > maxGitIndexPathBytes || uint64(length) > nameBudget {
				return nil, false
			}
			nameBudget -= uint64(length)
			name = append(append(make([]byte, 0, length), previous[:len(previous)-int(strip)]...), entry[cursor:cursor+end]...)
			cursor += end + 1
		} else {
			end := bytes.IndexByte(entry[cursor:], 0)
			if end < 0 {
				return nil, false
			}
			if length := int(flags & 0x0fff); length < 0x0fff && length != end || end > maxGitIndexPathBytes {
				return nil, false
			}
			name = entry[cursor : cursor+end]
			cursor = (cursor + end + 8) &^ 7
		}
		if len(name) == 0 || offset+cursor > len(body) {
			return nil, false
		}
		path := string(name)
		if !trusted {
			excluded[path] = struct{}{}
		} else {
			entries[path] = stat
		}
		previous = name
		offset += cursor
	}
	for offset < len(body) {
		if offset+8 > len(body) {
			return nil, false
		}
		signature := body[offset : offset+4]
		size := binary.BigEndian.Uint32(body[offset+4 : offset+8])
		if string(signature) == "link" || (signature[0] < 'A' || signature[0] > 'Z') && string(signature) != "sdir" {
			return nil, false
		}
		if uint64(size) > uint64(len(body)-offset-8) {
			return nil, false
		}
		offset += 8 + int(size)
	}
	for path := range excluded {
		delete(entries, path)
	}
	return &gitIndex{entries: entries}, true
}

// indexVarint decodes the offset varint of an index v4 path prefix.
func indexVarint(data []byte) (uint64, int, bool) {
	if len(data) == 0 {
		return 0, 0, false
	}
	value := uint64(data[0] & 0x7f)
	width := 1
	for data[width-1]&0x80 != 0 {
		if width >= len(data) || width >= 9 {
			return 0, 0, false
		}
		value = (value+1)<<7 | uint64(data[width]&0x7f)
		width++
	}
	return value, width, true
}

func indexChecksumMatches(body, trailer []byte, objectFormat string) bool {
	if objectFormat == "sha256" {
		sum := sha256.Sum256(body)
		return bytes.Equal(sum[:], trailer)
	}
	sum := sha1.Sum(body)
	return bytes.Equal(sum[:], trailer)
}
