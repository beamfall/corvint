package dogfoodflow

import "strings"

// RecordPath reports a repository-relative path under docs/build-log/ or
// .agent-evidence/ with a non-empty remainder (DCW-V0-057). A path Git quotes
// never matches.
func RecordPath(path string) bool {
	for _, prefix := range []string{"docs/build-log/", ".agent-evidence/"} {
		if rest, found := strings.CutPrefix(path, prefix); found && rest != "" {
			return true
		}
	}
	return false
}

// RecordChange classifies the paths a BASE..HEAD diff touches (git diff
// --no-renames --name-only). A change is a record when it touches at least one
// RecordPath and every other path is one of the CEM sidecar paths a chore may
// touch: .corvint/change.cem.json or .corvint/changes/<NAME>.cem.json
// (DCW-V0-057). A change that also touches any other path is a change, and a
// change that touches only sidecar paths is a chore, never a record.
// internal/cem/workflow.StableCitationClass applies the same rule to the hunk
// paths of a committed map (CEM-V1-170); keep the two in step.
func RecordChange(paths []string) bool {
	record := false
	for _, path := range paths {
		switch {
		case RecordPath(path):
			record = true
		case path == ".corvint/change.cem.json", choreChangesPath(path):
		default:
			return false
		}
	}
	return record
}

// recordChange classifies this run's BASE..HEAD diff (DCW-V0-057).
func (c *change) recordChange() bool {
	return RecordChange(nonEmpty(strings.Split(c.gitValue("diff", "--no-renames", "--name-only", c.base, c.target), "\n")))
}

// recordIntentPath reports a path a record change may name as its intent: a
// build-log entry or a decision file (DCW-V0-058).
func recordIntentPath(path string) bool {
	rest, found := strings.CutPrefix(path, "docs/build-log/")
	if !found {
		rest, found = strings.CutPrefix(path, "docs/decisions/")
	}
	return found && rest != "" && !strings.Contains(rest, "/") && strings.HasSuffix(rest, ".md")
}
