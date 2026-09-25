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
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// stateFiles are the fixed state-dir files a transaction inventory carries.
// `barrier.json` is optional; the rest exist in any initialized store.
var stateFiles = []string{"VERSION", "head.json", "reservations.json", "barrier.json"}

// scanDirectories are the state-dir children scanned for retained files.
// `staging` is deliberately absent: §5.6 staging is transient, excluded from
// the retained inventory, and re-added by the capacity model itself.
var scanDirectories = []string{"receipts", "requests", "evidence", "pinned"}

// inventory builds the complete transaction inventory of an initialized store:
// every retained state-dir file, every Git-tracked intent file, and the state
// directories that exist. It reports what is on disk and interprets nothing;
// a file outside the §3.4 layout is refused rather than skipped, so an
// unrecognized path can never be silently dropped from a capacity or
// projection check.
func inventory(repo *intent.Repository) (*transaction.Inventory, error) {
	files, dirs, err := scan(repo)
	if err != nil {
		return nil, err
	}
	return transaction.NewInventory(files, dirs)
}

func scan(repo *intent.Repository) ([]archive.FileEntry, []string, error) {
	files := []archive.FileEntry{}
	dirs := []string{}
	for _, name := range stateFiles {
		entry, ok, err := fileEntry(filepath.Join(repo.StateDir, name), name)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			files = append(files, entry)
		}
	}
	for _, dir := range scanDirectories {
		found, children, err := scanDir(repo.StateDir, dir)
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
	intentFiles, err := scanIntent(repo)
	if err != nil {
		return nil, nil, err
	}
	files = append(files, intentFiles...)
	sort.Strings(dirs)
	archive.SortFiles(files)
	return files, dirs, nil
}

type children struct {
	files []archive.FileEntry
	dirs  []string
}

// scanDir walks one state-dir child. `requests` is the only sharded one; a
// directory anywhere else, and any nested directory under a shard, is outside
// the §3.4 layout and refused.
func scanDir(stateDir, dir string) (bool, children, error) {
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
			shard, err := scanShard(stateDir, rel)
			if err != nil {
				return false, out, err
			}
			out.dirs = append(out.dirs, rel)
			out.files = append(out.files, shard...)
			continue
		}
		entry, ok, err := fileEntry(filepath.Join(root, e.Name()), rel)
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
		entry, ok, err := fileEntry(filepath.Join(root, e.Name()), rel)
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
	root := filepath.Join(repo.PrimaryWorktree, intent.Dir)
	files := []archive.FileEntry{}
	for _, name := range []string{"queue.json", "policy.json"} {
		entry, ok, err := fileEntry(filepath.Join(root, name), "intent/"+name)
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
			entry, ok, err := fileEntry(filepath.Join(root, dir, e.Name()), rel)
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
	bound, ok := intent.BoundFor(rel)
	if !ok {
		bound = wire.MaxEvidenceBlobBytes
	}
	raw, err := intent.ReadFile(full, bound)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return archive.FileEntry{}, false, nil
		}
		return archive.FileEntry{}, false, err
	}
	return archive.FileEntry{Path: rel, Sha256: wire.Sum(raw), Bytes: wire.SizeOf(uint64(len(raw)))}, true, nil
}
