package affected

import (
	"strings"

	"github.com/Beamfall/corvint/internal/taskprojection"
)

// Non-source path classes (AFP-V0-088). A dirty path of one of these classes
// is task state, not source: it names no unit and widens no plan unless a
// unit names it.
const (
	// ClassTaskStoreProjection is a file of a Corvint Tasks store projection
	// (TaskStoreProjectionDir): the policy, the queue, or one ticket or
	// release record that corvint-tasks writes.
	ClassTaskStoreProjection = taskprojection.Class
)

// TaskStoreProjectionDir is the repository-relative directory a Corvint Tasks
// store projects its intent into (AFP-V0-088).
const TaskStoreProjectionDir = taskprojection.Dir

// taskStoreProjectionReason is the stated reason of every
// ClassTaskStoreProjection entry.
const taskStoreProjectionReason = "Corvint Tasks store projection written by corvint-tasks; no unit's path literal or declared read scope names it, so it is task state, not a build or test input"

// NonSource is one class of dirty paths classified as task state rather than
// source (AFP-V0-088): its paths add no unknown entry, and Reason says why.
// The reason is stated once per class, not per path, so a projection of
// hundreds of tickets stays a compact member.
type NonSource struct {
	Class  string   `json:"class"`
	Reason string   `json:"reason"`
	Paths  []string `json:"paths"`
}

// IsTaskStoreProjectionPath reports whether a canonical repository-relative
// path is a file of the recognised Tasks store projection layout:
// `.taskman/policy.json`, `.taskman/queue.json`, or one
// `.taskman/tickets/<name>.json` or `.taskman/releases/<name>.json` record.
// Any other path under `.taskman/` is not recognised and stays unowned.
func IsTaskStoreProjectionPath(path string) bool {
	return taskprojection.IsPath(path)
}

// taskState removes, from the unknown entries seed raised, each
// UNOWNED_DIRTY_PATH entry for a recognised Tasks store projection path that
// no unit names, and returns those paths, in dirty-path order, as one
// ClassTaskStoreProjection group, or no group when there are none
// (AFP-V0-088). A projection path that some unit's path
// literal resolves to (AFP-V0-021, AFP-V0-072) or whose declared read scope
// holds it (AFP-V0-023) keeps its unknown entry: a test reads it, and the
// reader selection only adds. An unbounded reader (rule (d)) is selected on
// every dirty path and does not count as naming one.
func (graph *Graph) taskState(unknown []Unknown) ([]NonSource, []Unknown) {
	var tokens [][]pathToken
	paths := make([]string, 0)
	kept := make([]Unknown, 0, len(unknown))
	for _, entry := range unknown {
		if entry.Reason != UnknownUnownedDirtyPath || !IsTaskStoreProjectionPath(entry.Detail) {
			kept = append(kept, entry)
			continue
		}
		if tokens == nil {
			tokens = graph.pathTokens()
		}
		if graph.named(entry.Detail, tokens) {
			kept = append(kept, entry)
			continue
		}
		paths = append(paths, entry.Detail)
	}
	if len(paths) == 0 {
		return nil, kept
	}
	return []NonSource{{Class: ClassTaskStoreProjection, Reason: taskStoreProjectionReason, Paths: paths}}, kept
}

// named reports whether any unit's path literal resolves to path or any
// unit's declared read scope holds it, regardless of what else reached it.
// A literal names a projection file only through its whole file name
// (namesFile, AFP-V0-088 (2)): a directory fragment such as `/tickets/` or
// `.taskman/`, or a file-name fragment such as the `.taskman/tickets/V1-` of
// a formatted literal, names a family of records, not this one.
func (graph *Graph) named(path string, tokens [][]pathToken) bool {
	parts := strings.Split(path, "/")
	for i, id := range graph.order {
		if namesFileAny(tokens[i], parts) && graph.resolves(id, path) {
			return true
		}
		unit := graph.units[id]
		files := append(append([]string(nil), unit.Sources...), unit.Tests...)
		if unit.ReadScoped && len(files) != 0 && inDeclaredScope(relativeDirectory(files[0]), unit.ReadScope, path) {
			return true
		}
	}
	return false
}

func namesFileAny(tokens []pathToken, parts []string) bool {
	for _, token := range tokens {
		if token.namesFile(parts) {
			return true
		}
	}
	return false
}

// namesFile reports whether one of the token's component runs matches the
// split path's last components and ends at its whole file name: a run whose
// last component is only a prefix of the file name (a literal concatenated
// with a variable or a format verb) does not name the file (AFP-V0-088 (2)).
func (token pathToken) namesFile(parts []string) bool {
	for _, candidate := range token.runs {
		offset := len(parts) - len(candidate.components)
		if offset < 0 || candidate.components[len(candidate.components)-1] != parts[len(parts)-1] {
			continue
		}
		if candidate.matchesAt(parts[offset:]) {
			return true
		}
	}
	return false
}
