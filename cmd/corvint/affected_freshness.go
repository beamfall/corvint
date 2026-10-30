package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// affectedDirtyPathsShown bounds the dirty paths a MIXED warning or refusal names.
const affectedDirtyPathsShown = 8

// affectedDirtyDetail names, in sorted order, the first affectedDirtyPathsShown
// dirty paths and the count of the rest, and states that Corvint runs Git
// without global configuration (decision 0158), so a path ignored only by a
// global excludes file is dirty here, with the fix (AFP-V0-356, V1-2053).
func affectedDirtyDetail(paths []string) string {
	sorted := slices.Sorted(slices.Values(paths))
	shown := make([]string, 0, affectedDirtyPathsShown)
	for _, path := range sorted[:min(len(sorted), affectedDirtyPathsShown)] {
		shown = append(shown, strconv.Quote(path))
	}
	detail := "dirty: " + strings.Join(shown, ", ")
	if len(sorted) > len(shown) {
		detail += fmt.Sprintf(" (+%d more)", len(sorted)-len(shown))
	}
	return detail + "; Corvint runs Git without global configuration, so a path ignored only by a global excludes file (core.excludesFile or ~/.config/git/ignore) is dirty here: ignore it in a tracked .gitignore or in .git/info/exclude"
}

// affectedMixedWarning is the AFP-V0-044 warning: a --base range path outside
// the worktree dirty set is planned together with uncommitted worktree content
// that the receipt's revision does not pin. It is empty, and the member is
// omitted, for a clean worktree, the worktree-only form, or a range inside the
// dirty set.
func affectedMixedWarning(revision string, dirty, committed []string) string {
	if len(dirty) == 0 || len(committed) == 0 {
		return ""
	}
	dirtySet := make(map[string]struct{}, len(dirty))
	for _, path := range dirty {
		dirtySet[path] = struct{}{}
	}
	for _, path := range committed {
		if _, ok := dirtySet[path]; !ok {
			return fmt.Sprintf("MIXED worktree: the --base range is planned at commit %s together with %d dirty worktree paths whose mutable content no commit pins; %s.", revision, len(dirty), affectedDirtyDetail(dirty))
		}
	}
	return ""
}

// affectedMixedWorktreeRefusal is the AFP-V0-045 --require-clean refusal,
// returned before the range diff, the graph walk or any provider runs. It
// names the bounded dirty set and the global-excludes cause (AFP-V0-356).
func affectedMixedWorktreeRefusal(revision string, dirty []string) error {
	return &gokernel.Error{Code: "unsupported-affected-mixed-worktree", Message: fmt.Sprintf("MIXED worktree: --require-clean refuses to plan; HEAD is commit %s with %d dirty worktree paths; %s", revision, len(dirty), affectedDirtyDetail(dirty))}
}

// affectedPathListMixedRefusal is the AFP-V0-094 refusal: `--path` plans at
// HEAD, and a worktree with any status entry would let uncommitted bytes into
// a graph the receipt binds to that commit. It names the bounded dirty set and
// the global-excludes cause (AFP-V0-356).
func affectedPathListMixedRefusal(revision string, dirty []string) error {
	return &gokernel.Error{Code: "unsupported-affected-mixed-worktree", Message: fmt.Sprintf("MIXED worktree: --path plans at HEAD and refuses a dirty worktree; HEAD is commit %s with %d dirty worktree paths; %s. %s", revision, len(dirty), affectedDirtyDetail(dirty), affectedPathListRecovery(revision))}
}

// affectedPathListRecovery is the AFP-V0-507 recovery a --path refusal names:
// the exact commands that plan the same change without mixing HEAD with
// uncommitted bytes, so the caller never has to under-select by hand.
func affectedPathListRecovery(revision string) string {
	return fmt.Sprintf("Recovery: run `corvint affected` without --path to plan the dirty worktree set now, or commit the dirty paths and run `corvint affected --base %s`, which plans every path committed after this HEAD together with any later worktree change", revision)
}

// affectedPathListIgnoredRefusal names an ignored file HEAD does not track that
// a --path plan would read from disk (AFP-V0-094).
func affectedPathListIgnoredRefusal(revision, path string) error {
	return &gokernel.Error{Code: "unsupported-affected-mixed-worktree", Message: fmt.Sprintf("MIXED worktree: --path plans at HEAD and refuses ignored file %s, which HEAD does not track and the graph reads; HEAD is commit %s", path, revision)}
}
