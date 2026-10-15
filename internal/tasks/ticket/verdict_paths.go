package ticket

import "github.com/Beamfall/corvint/internal/tasks/wire"

// Named refusal reasons for effects.verdictPaths (TCB-V0-036). Each prefixes
// its refusal message, so a caller can match it without parsing the prose.
const (
	// VerdictPathsEmpty refuses MALFORMED: an empty list would govern no
	// path, so a ticket that wants the touchPaths fallback omits the key.
	VerdictPathsEmpty = "VERDICT_PATHS_EMPTY"
	// VerdictPathOutsideTouchPaths refuses OUT_OF_SCOPE: the verdict
	// predicate domain may only narrow the claim scope, never widen it.
	VerdictPathOutsideTouchPaths = "VERDICT_PATH_OUTSIDE_TOUCH_PATHS"
)

// ReadVerdictPaths reads the optional verdictPaths member of the effects
// object ef, whose touchPaths are touch (TCB-V0-035, TCB-V0-036). It is nil
// when the member is absent. A present list has the touchPaths shape (a
// sorted, duplicate-free set of repository Paths, at most wire.MaxTouchPaths)
// and must be nonempty and covered, path by path, by touch under CAL-V0-021.
func ReadVerdictPaths(ef *wire.Reader, touch []string) []string {
	if ef.Err() != nil || !wire.Has(ef.Value(), "verdictPaths") {
		return nil
	}
	f := ef.Field("verdictPaths")
	paths := f.Strings(wire.MaxTouchPaths, false, (*wire.Reader).Path)
	if f.Err() != nil {
		return nil
	}
	if len(paths) == 0 {
		f.Fail(wire.CodeMalformed, "%s: effects.verdictPaths must name at least one path; omit it to govern effects.touchPaths", VerdictPathsEmpty)
		return nil
	}
	if i := wire.UncoveredVerdictPath(touch, paths); i >= 0 {
		f.Fail(wire.CodeOutOfScope, "%s: effects.verdictPaths/%d %q is not covered by effects.touchPaths (a directory key ends in \"/\")", VerdictPathOutsideTouchPaths, i, paths[i])
		return nil
	}
	return paths
}

// SetVerdictPaths adds verdictPaths to the effects object eo only when
// paths is non-nil, so a record or payload that omits it keeps its bytes.
func SetVerdictPaths(eo *wire.Object, paths []string) {
	if paths != nil {
		eo.Set("verdictPaths", wire.Strings(paths))
	}
}
