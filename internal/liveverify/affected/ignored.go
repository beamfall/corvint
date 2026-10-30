package affected

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

// ErrIgnoredHoldConflict refuses a second exclusion hold on one root whose
// entries differ from the hold already in force.
var ErrIgnoredHoldConflict = errors.New("affected: a different ignored-path exclusion already holds this root")

// ignoredHolds names, per absolute root, the ignored paths HEAD does not track
// that every disk walk of that root skips while a caller holds them
// (TJAA-V0-100).
var ignoredHolds = struct {
	sync.Mutex
	held map[string]*ignoredHold
}{held: map[string]*ignoredHold{}}

type ignoredHold struct {
	count   int
	entries []string
}

// HoldIgnoredExclusion makes every disk walk of root skip ignored, the
// IgnoredPaths list of the same worktree, until the returned release is
// called (TJAA-V0-100). Each entry names one ignored file, or, with a
// trailing slash, a wholly ignored directory that holds no tracked file; Git
// lists neither a tracked file nor a directory that holds one, so a tracked
// but ignored file stays a graph input. Such a path is not a dirty path and
// its bytes are not HEAD's, so it is no graph input either, as the --path
// refusal of an ignored file HEAD does not track already holds
// (AFP-V0-094). A hold changes only what a walk lists: an explicit read still
// reads, and an import that resolves only to an excluded file is unresolved.
// A second hold of the same root must name the same entries. An entry that
// may stand for a different name on disk (one holding U+FFFD, which Git's
// lossy decoding writes for invalid bytes, or one that is no valid relative
// path) is dropped, so that path stays a graph input.
func HoldIgnoredExclusion(root string, ignored []string) (func(), error) {
	entries := exactIgnored(NormalizeIgnored(ignored))
	ignoredHolds.Lock()
	defer ignoredHolds.Unlock()
	if hold := ignoredHolds.held[root]; hold != nil {
		if !slices.Equal(hold.entries, entries) {
			return nil, ErrIgnoredHoldConflict
		}
		hold.count++
	} else {
		ignoredHolds.held[root] = &ignoredHold{count: 1, entries: entries}
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			ignoredHolds.Lock()
			defer ignoredHolds.Unlock()
			if hold := ignoredHolds.held[root]; hold != nil {
				if hold.count--; hold.count <= 0 {
					delete(ignoredHolds.held, root)
				}
			}
		})
	}, nil
}

// NormalizeIgnored returns the sorted, duplicate-free ignored entries.
func NormalizeIgnored(ignored []string) []string {
	entries := slices.Clone(ignored)
	slices.Sort(entries)
	return slices.Compact(entries)
}

func exactIgnored(entries []string) []string {
	exact := entries[:0:0]
	for _, entry := range entries {
		if !strings.ContainsRune(entry, utf8.RuneError) && ValidRelativePath(strings.TrimSuffix(entry, "/")) {
			exact = append(exact, entry)
		}
	}
	return exact
}

// HeldIgnoredMatcher reports, for a repository-relative path, whether the
// exclusion held for root covers it, or returns nil when no hold is in force.
// A resolver that tries several candidate files for one import MUST treat a
// covered candidate as unresolved rather than fall through to a later one,
// because the excluded file may be the one the runtime loads (TJAA-V0-100).
func HeldIgnoredMatcher(root string) func(string) bool {
	ignored := heldIgnored(root)
	if len(ignored) == 0 {
		return nil
	}
	return func(relative string) bool { return IgnoredMatch(ignored, relative) }
}

// heldIgnored returns the entries a hold on root excludes, or nil.
func heldIgnored(root string) []string {
	ignoredHolds.Lock()
	defer ignoredHolds.Unlock()
	if hold := ignoredHolds.held[root]; hold != nil {
		return hold.entries
	}
	return nil
}

// IgnoredMatch reports whether relative, a repository-relative file path or
// directory path without a trailing slash, is an entry or lies under a
// directory entry of the sorted ignored list.
func IgnoredMatch(ignored []string, relative string) bool {
	for _, entry := range ignored {
		if directory, ok := strings.CutSuffix(entry, "/"); ok {
			if relative == directory || strings.HasPrefix(relative, entry) {
				return true
			}
		} else if relative == entry {
			return true
		}
	}
	return false
}
