package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// stateFiles are the fixed state-dir files a transaction inventory carries.
// `barrier.json` is optional; the rest exist in any initialized store.
var stateFiles = []string{"VERSION", "head.json", "reservations.json", "barrier.json", "pools.json", "programs.json"}

// scanDirectories are the state-dir children scanned for retained files.
// `staging` is deliberately absent: §5.6 staging is transient, excluded from
// the retained inventory, and re-added by the capacity model itself.
var scanDirectories = []string{"receipts", "requests", "evidence", "pinned", "attempts"}

// inventory builds the complete transaction inventory of an initialized store:
// every retained state-dir file, every Git-tracked intent file, and the state
// directories that exist. It reports what is on disk and interprets nothing;
// a file outside the §3.4 layout is refused rather than skipped, so an
// unrecognized path can never be silently dropped from a capacity or
// projection check.
func inventory(repo *intent.Repository) (*transaction.Inventory, error) {
	files, dirs, err := scanWithReader(repo, intent.ReadFile)
	if err != nil {
		return nil, err
	}
	return retainedInventory(repo, files, dirs)
}

type inventoryReader func(string, int) ([]byte, error)

func scan(repo *intent.Repository) ([]archive.FileEntry, []string, error) {
	return scanWithReader(repo, intent.ReadFile)
}

func scanWithReader(repo *intent.Repository, read inventoryReader, observed ...map[string]journal.PhysicalFile) ([]archive.FileEntry, []string, error) {
	files := []archive.FileEntry{}
	dirs := []string{}
	for _, name := range stateFiles {
		entry, ok, err := fileEntryWithReader(filepath.Join(repo.StateDir, name), name, read, observed...)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			files = append(files, entry)
		}
	}
	for _, dir := range scanDirectories {
		found, children, err := scanDirWithReader(repo.StateDir, dir, read, observed...)
		if err != nil {
			return nil, nil, err
		}
		if !found {
			continue
		}
		dirs = append(dirs, dir)
		dirs = append(dirs, children.dirs...)
		files = append(files, children.files...)
	}
	sealed, err := sealedEntries(repo, read)
	if err != nil {
		return nil, nil, err
	}
	files = withSealed(files, sealed)
	intentFiles, err := scanIntentWithReader(repo, read, observed...)
	if err != nil {
		return nil, nil, err
	}
	files = append(files, intentFiles...)
	sort.Strings(dirs)
	archive.SortFiles(files)
	return files, dirs, nil
}

// sealedEntries lists the receipts and request afterimages the segments of a
// taskman-journal-head/1 hold, under their logical receipts/ and requests/
// paths (JSG-V0-012), so a store's inventory is the same before and after a
// seal. Each segment is read through read and checked against the manifest
// the head binds; a manifest or segment a concurrent seal replaced is a
// moved observation. A head that does not decode adds nothing: the journal
// audit owns that refusal. Without segments/ the head is not read again: a
// seal creates segments/ before it publishes a head that binds it, and only
// an unseal removes it, after it published a head that does not.
func sealedEntries(repo *intent.Repository, read inventoryReader) ([]archive.FileEntry, error) {
	if _, err := os.Lstat(segmentPath(repo, snapshot.SegmentsDir)); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	raw, err := read(filepath.Join(repo.StateDir, "head.json"), wire.MaxJournalHeadBytes)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	head, err := snapshot.DecodeHead(raw)
	if err != nil || head.Segments == nil {
		return nil, nil
	}
	moved := func(p, why string) error { return wire.Errorf(wire.CodeSnapshotMoved, p, "%s", why) }
	name := snapshot.SegmentManifestName(head.Segments.Through.Uint64())
	m, err := read(filepath.Join(repo.StateDir, filepath.FromSlash(name)), snapshot.MaxSegmentManifestBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, moved(name, "a later seal replaced the manifest the head named")
	}
	if err != nil {
		return nil, err
	}
	if wire.Sum(m) != head.Segments.ManifestSha256 {
		return nil, moved(name, "segment manifest differs from the digest the head binds")
	}
	manifest, err := snapshot.DecodeSegmentManifest(m, 1)
	if err != nil {
		return nil, err
	}
	var out []archive.FileEntry
	for _, e := range manifest.Segments {
		seg := snapshot.SegmentName(e.First, e.Last)
		raw, err := read(filepath.Join(repo.StateDir, filepath.FromSlash(seg)), snapshot.MaxSegmentBytes)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, moved(seg, "bound segment absent")
		}
		if err != nil {
			return nil, err
		}
		if uint64(len(raw)) != e.Bytes || wire.Sum(raw) != e.Sha256 {
			return nil, moved(seg, "segment differs from the digest the manifest binds")
		}
		s, err := snapshot.DecodeSegment(raw, e.First, e.Last)
		if err != nil {
			return nil, err
		}
		for _, list := range [][]snapshot.SegmentFile{s.Receipts, s.Requests} {
			for _, f := range list {
				out = append(out, archive.FileEntry{Path: f.Path, Sha256: wire.Sum(f.Raw), Bytes: wire.SizeOf(uint64(len(f.Raw)))})
			}
		}
	}
	return out, nil
}

// withSealed adds the sealed entries to files. A physical file still linked
// at a sealed path is the same file (a seal checks every digest before it
// drops one), so the sealed entry replaces it.
func withSealed(files, sealed []archive.FileEntry) []archive.FileEntry {
	if len(sealed) == 0 {
		return files
	}
	paths := make(map[string]struct{}, len(sealed))
	for _, f := range sealed {
		paths[f.Path] = struct{}{}
	}
	kept := files[:0]
	for _, f := range files {
		if _, dup := paths[f.Path]; !dup {
			kept = append(kept, f)
		}
	}
	return append(kept, sealed...)
}

type children struct {
	files []archive.FileEntry
	dirs  []string
}

// scanDir walks one state-dir child. `requests` is the only sharded one; a
// directory anywhere else, and any nested directory under a shard, is outside
// the §3.4 layout and refused.
func scanDir(stateDir, dir string) (bool, children, error) {
	return scanDirWithReader(stateDir, dir, intent.ReadFile)
}

func scanDirWithReader(stateDir, dir string, read inventoryReader, observed ...map[string]journal.PhysicalFile) (bool, children, error) {
	var out children
	root := filepath.Join(stateDir, dir)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, out, nil
		}
		return false, out, err
	}
	for _, e := range entries {
		rel := dir + "/" + e.Name()
		if e.IsDir() {
			if dir != "requests" {
				return false, out, wire.Errorf(wire.CodeMalformed, rel, "unexpected directory in %s", dir)
			}
			shard, err := scanShardWithReader(stateDir, rel, read, observed...)
			if err != nil {
				return false, out, err
			}
			out.dirs = append(out.dirs, rel)
			out.files = append(out.files, shard...)
			continue
		}
		entry, ok, err := fileEntryWithReader(filepath.Join(root, e.Name()), rel, read, observed...)
		if err != nil {
			return false, out, err
		}
		if ok {
			out.files = append(out.files, entry)
		}
	}
	return true, out, nil
}

func scanShard(stateDir, shard string) ([]archive.FileEntry, error) {
	return scanShardWithReader(stateDir, shard, intent.ReadFile)
}

func scanShardWithReader(stateDir, shard string, read inventoryReader, observed ...map[string]journal.PhysicalFile) ([]archive.FileEntry, error) {
	root := filepath.Join(stateDir, shard)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	files := []archive.FileEntry{}
	for _, e := range entries {
		rel := shard + "/" + e.Name()
		if e.IsDir() {
			return nil, wire.Errorf(wire.CodeMalformed, rel, "unexpected directory in a request shard")
		}
		entry, ok, err := fileEntryWithReader(filepath.Join(root, e.Name()), rel, read, observed...)
		if err != nil {
			return nil, err
		}
		if ok {
			files = append(files, entry)
		}
	}
	return files, nil
}

// scanIntent lists the Git-tracked intent files under their archive-namespace
// paths. The inventory names them `intent/...` because that is how a plan's
// post paths name them (§3.4).
func scanIntent(repo *intent.Repository) ([]archive.FileEntry, error) {
	return scanIntentWithReader(repo, intent.ReadFile)
}

func scanIntentWithReader(repo *intent.Repository, read inventoryReader, observed ...map[string]journal.PhysicalFile) ([]archive.FileEntry, error) {
	root := filepath.Join(repo.IntentRoot(), intent.Dir)
	files := []archive.FileEntry{}
	// The import map is listed like the intent tree lists it (CTS-V0-014), so
	// the inventory and the journal's intent tree digest name the same files.
	for _, name := range []string{intent.QueueFile, intent.PolicyFile, intent.ImportMapFile} {
		entry, ok, err := fileEntryWithReader(filepath.Join(root, name), "intent/"+name, read, observed...)
		if err != nil {
			return nil, err
		}
		if ok {
			files = append(files, entry)
		}
	}
	for _, dir := range []string{intent.TicketsDir, intent.ReleasesDir} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			rel := "intent/" + dir + "/" + e.Name()
			entry, ok, err := fileEntryWithReader(filepath.Join(root, dir, e.Name()), rel, read, observed...)
			if err != nil {
				return nil, err
			}
			if ok {
				files = append(files, entry)
			}
		}
	}
	return files, nil
}

// fileEntry digests one file. An absent file is reported as absent rather
// than as an error: the caller decides whether its absence is legal.
func fileEntry(full, rel string) (archive.FileEntry, bool, error) {
	return fileEntryWithReader(full, rel, intent.ReadFile)
}

func fileEntryWithReader(full, rel string, read inventoryReader, observed ...map[string]journal.PhysicalFile) (archive.FileEntry, bool, error) {
	// Archive identities keep intent/, while BoundFor takes intent-relative names.
	bound, ok := intent.BoundFor(strings.TrimPrefix(rel, "intent/"))
	if !ok {
		bound = wire.MaxEvidenceBlobBytes
	}
	if len(observed) > 0 {
		if file, present := observed[0][rel]; present {
			if file.Bytes < 0 || file.Bytes > bound {
				return archive.FileEntry{}, false, wire.Errorf(wire.CodeLimitExceeded, rel, "observed inventory bytes exceed bound")
			}
			return archive.FileEntry{Path: rel, Sha256: file.Sha256, Bytes: wire.SizeOf(uint64(file.Bytes))}, true, nil
		}
	}
	raw, err := read(full, bound)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return archive.FileEntry{}, false, nil
		}
		return archive.FileEntry{}, false, err
	}
	return archive.FileEntry{Path: rel, Sha256: wire.Sum(raw), Bytes: wire.SizeOf(uint64(len(raw)))}, true, nil
}
