package affected

import (
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// GraphCacheMaxEntries caps the entries one graph cache directory holds
// (AFP-V0-970): a removed worktree's entry is evicted once this many newer
// entries are written.
const GraphCacheMaxEntries = 32

// graphCacheListBound is the most names eviction reads, the AFP-V0-439
// listing bound.
const graphCacheListBound = 4096

// evictGraphCache removes the oldest entries of directory, by modification
// time and then name, until at most GraphCacheMaxEntries remain, never kept,
// the entry just written (AFP-V0-970). It lists and removes only through the
// handle, and only regular files named like an entry, so nothing outside the
// cache directory and no other file in it is touched. A listing or removal
// failure leaves the entries in place: eviction never fails a write.
func evictGraphCache(directory *os.Root, kept string) {
	listing, err := directory.Open(".")
	if err != nil {
		return
	}
	defer listing.Close()
	type entry struct {
		name     string
		modified time.Time
	}
	entries := []entry{}
	read := 0
	for read < graphCacheListBound {
		batch, err := listing.ReadDir(256)
		for _, item := range batch {
			read++
			name := item.Name()
			if !strings.HasPrefix(name, "affected-graph-") || !strings.HasSuffix(name, ".json") {
				continue
			}
			info, statErr := directory.Lstat(name)
			if statErr != nil || !info.Mode().IsRegular() {
				continue
			}
			entries = append(entries, entry{name, info.ModTime()})
		}
		if err == io.EOF || (err == nil && len(batch) == 0) {
			break
		}
		if err != nil {
			return
		}
	}
	if len(entries) <= GraphCacheMaxEntries {
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].modified.Equal(entries[j].modified) {
			return entries[i].modified.Before(entries[j].modified)
		}
		return entries[i].name < entries[j].name
	})
	excess := len(entries) - GraphCacheMaxEntries
	for _, candidate := range entries {
		if excess == 0 {
			return
		}
		if candidate.name == kept {
			continue
		}
		if directory.Remove(candidate.name) == nil {
			excess--
		}
	}
}
