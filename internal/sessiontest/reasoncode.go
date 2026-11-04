package sessiontest

import (
	"regexp"
	"strings"
)

// Reason codes are the stable machine form of a Go unit's prose Reason
// (STR-V0-548, V1-3478): a tool reads Unit.ReasonCode instead of matching
// the prose, which may be reworded. The enum is closed; a reason no rule
// names is ReasonOther, so a new prose shape is visible rather than
// misfiled.
const (
	ReasonSelectedChangedTests   = "selected-changed-tests"
	ReasonSelectedReachingChange = "selected-reaching-change"
	ReasonSelectedBuildOnly      = "selected-build-only"
	ReasonAttributed             = "whole-attributed"
	ReasonAlsoSelected           = "whole-also-selected"
	ReasonNotSeparable           = "whole-not-separable"
	ReasonTestSideEffect         = "whole-test-side-effect"
	ReasonUnreadable             = "whole-unreadable"
	ReasonSharedDeclaration      = "whole-shared-declaration"
	ReasonTestImportsChanged     = "whole-test-imports-changed"
	ReasonNoTestFunction         = "whole-no-test-function"
	ReasonPatternBound           = "whole-pattern-bound"
	ReasonNotInGraph             = "whole-not-in-graph"
	ReasonNoChangedTestFile      = "whole-no-changed-test-file"
	ReasonForeignTestFile        = "whole-foreign-test-file"
	ReasonNestedModule           = "whole-nested-module"
	ReasonDirectSource           = "direct-source-change"
	ReasonNonFunctionEdit        = "direct-source-non-function-edit"
	ReasonReferenceSignature     = "direct-source-reference-signature"
	ReasonEveryTestReaches       = "direct-source-every-test-reaches"
	ReasonSourceSliceBound       = "direct-source-slice-bound"
	ReasonReachedPrefix          = "reached-via-" // + the witness kind in kebab case
	ReasonOther                  = "other"
)

// witnessReason matches the "whole: KIND via PATH" form of a whole unit
// kept by its plan witness.
var witnessReason = regexp.MustCompile(`^whole: ([A-Z][A-Z_]*) via `)

// referenceReason matches the detail of a changed function whose reference
// other than a call keeps the package whole.
var referenceReason = regexp.MustCompile(` in \S+ has `)

// ReasonCode returns the stable code of a Go unit's prose reason, "" for no
// reason. Rules are ordered: the first that matches decides.
func ReasonCode(reason string) string {
	switch {
	case reason == "":
		return ""
	case reason == moduleReason:
		return ReasonNestedModule
	case strings.HasPrefix(reason, "whole: MUST_RUN ("):
		// A declared must-run unit (AFP-V0-2485); its prose carries the
		// project's free-text reason, so no substring rule below may read it.
		return ReasonReachedPrefix + "must-run"
	case strings.HasPrefix(reason, "selected: the tests of "):
		return ReasonSelectedChangedTests
	case strings.HasPrefix(reason, "selected: the tests reaching "):
		return ReasonSelectedReachingChange
	case strings.HasPrefix(reason, "selected: no test reaches "):
		return ReasonSelectedBuildOnly
	case strings.HasPrefix(reason, "whole: attributed: "):
		return ReasonAttributed
	case strings.HasPrefix(reason, "whole: also selected by "):
		return ReasonAlsoSelected
	case strings.Contains(reason, " is not separable from "):
		return ReasonNotSeparable
	}
	if match := witnessReason.FindStringSubmatch(reason); match != nil {
		if match[1] != "DIRECT_SOURCE_CHANGE" {
			return ReasonReachedPrefix + strings.ReplaceAll(strings.ToLower(match[1]), "_", "-")
		}
		return directSourceCode(reason)
	}
	switch {
	case strings.HasSuffix(reason, "the -run pattern exceeds the bound"):
		return ReasonPatternBound
	case strings.Contains(reason, " declares init") || strings.Contains(reason, " declares TestMain") ||
		strings.Contains(reason, " for its side effects") || strings.Contains(reason, " dot-imports ") ||
		(strings.Contains(reason, " initializes ") && strings.HasSuffix(reason, " with a call")):
		return ReasonTestSideEffect
	case strings.HasSuffix(reason, " cannot be read") || strings.HasSuffix(reason, " does not parse") ||
		strings.HasSuffix(reason, " cannot be read at base") || strings.HasSuffix(reason, " is not a file at base") ||
		strings.HasSuffix(reason, " cannot be listed at base"):
		return ReasonUnreadable
	case strings.Contains(reason, " declared in "):
		return ReasonSharedDeclaration
	case strings.Contains(reason, "which no test file of the package"):
		return ReasonTestImportsChanged
	case reason == "whole: the changed test files declare no test function":
		return ReasonNoTestFunction
	case reason == "whole: the plan's unit is not in the graph":
		return ReasonNotInGraph
	case reason == "whole: no dirty path is one of the unit's test files":
		return ReasonNoChangedTestFile
	case reason == "whole: a changed test file is not a test file of the unit":
		return ReasonForeignTestFile
	}
	return ReasonOther
}

// directSourceCode classifies a DIRECT_SOURCE_CHANGE unit kept whole by the
// detail its source slicing appended in parentheses (TSP-V0-006).
func directSourceCode(reason string) string {
	switch {
	case strings.Contains(reason, " changes more than plain functions)"):
		return ReasonNonFunctionEdit
	case strings.Contains(reason, "(every test function of the package reaches "):
		return ReasonEveryTestReaches
	case strings.Contains(reason, "(source slicing plans at most "):
		return ReasonSourceSliceBound
	case strings.Contains(reason, "(also selected by "):
		return ReasonAlsoSelected
	case strings.HasSuffix(reason, "(the -run pattern exceeds the bound)"):
		return ReasonPatternBound
	case referenceReason.MatchString(reason):
		return ReasonReferenceSignature
	}
	return ReasonDirectSource
}
