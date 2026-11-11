package rubytestprovider

import (
	"encoding/json"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/rubytestreceipt"
)

// SelectorPath is rubytestreceipt.SelectorPath, kept under its old name.
const SelectorPath = rubytestreceipt.SelectorPath

// SelectorLine is rubytestreceipt.SelectorLine, kept under its old name.
const SelectorLine = rubytestreceipt.SelectorLine

// SelectorID is rubytestreceipt.SelectorID, kept under its old name.
const SelectorID = rubytestreceipt.SelectorID

// SelectorMatched is rubytestreceipt.SelectorMatched, kept under its old name.
const SelectorMatched = rubytestreceipt.SelectorMatched

// SelectorNoExample is rubytestreceipt.SelectorNoExample, kept under its old name.
const SelectorNoExample = rubytestreceipt.SelectorNoExample

// SelectorUnattributed is rubytestreceipt.SelectorUnattributed, kept under its old name.
const SelectorUnattributed = rubytestreceipt.SelectorUnattributed

// SelectorNotEvaluated is rubytestreceipt.SelectorNotEvaluated, kept under its old name.
const SelectorNotEvaluated = rubytestreceipt.SelectorNotEvaluated

// MaxSelectors is rubytestreceipt.MaxSelectors, kept under its old name.
const MaxSelectors = rubytestreceipt.MaxSelectors

// reportedLocation is what the report states about one example's location:
// the rerun path and scoped id from its RSpec id, and its declaration file
// and line.
type reportedLocation struct {
	rerunPath, scopedID, file string
	line                      int
	// inclusions are the project-relative shared-group inclusion lines the
	// report states, empty when it states none (LPCV-V0-137).
	inclusions []reportedInclusion
}

// reportedInclusion is one project-relative shared-group inclusion file:line.
type reportedInclusion struct {
	file string
	line int
}

func locate(raw rspecExample) reportedLocation {
	location := reportedLocation{file: path.Clean(strings.TrimPrefix(raw.FilePath, "./")), line: raw.LineNumber}
	location.inclusions, _ = inclusionFrames(raw.SharedGroupInclusionBacktrace)
	if open := strings.LastIndexByte(raw.ID, '['); open > 0 && strings.HasSuffix(raw.ID, "]") {
		location.rerunPath = path.Clean(strings.TrimPrefix(raw.ID[:open], "./"))
		location.scopedID = raw.ID[open+1 : len(raw.ID)-1]
	}
	return location
}

func under(name, prefix string) bool {
	return name != "" && (prefix == "." || name == prefix || strings.HasPrefix(name, prefix+"/"))
}

// selectionOutcomes attributes the reported examples to each selector
// (LPCV-V0-132). examples is nil when the run produced no readable report.
func selectionOutcomes(specs []string, examples []rspecExample) []SelectorOutcome {
	outcomes := make([]SelectorOutcome, 0, len(specs))
	locations := make([]reportedLocation, 0, len(examples))
	for _, raw := range examples {
		locations = append(locations, locate(raw))
	}
	for _, value := range specs {
		parsed, _ := rubytestreceipt.ParseSelector(value)
		outcome := SelectorOutcome{Selector: value, Kind: parsed.Kind, Outcome: SelectorNotEvaluated}
		if examples != nil {
			matched, fromFile := 0, false
			for _, location := range locations {
				switch parsed.Kind {
				case SelectorPath:
					if under(location.rerunPath, parsed.Path) || under(location.file, parsed.Path) {
						matched++
					}
				case SelectorLine:
					fromFile = fromFile || location.file == parsed.Path || location.rerunPath == parsed.Path
					if lineSelected(parsed, location) {
						matched++
					}
				case SelectorID:
					for _, id := range parsed.Ids {
						if location.rerunPath == parsed.Path && (location.scopedID == id || strings.HasPrefix(location.scopedID, id+":")) {
							matched++
							break
						}
					}
				}
			}
			switch {
			case matched > 0:
				outcome.Outcome, outcome.Matched = SelectorMatched, matched
			case fromFile:
				outcome.Outcome = SelectorUnattributed
			default:
				outcome.Outcome = SelectorNoExample
			}
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes
}

// lineSelected reports whether a path:line selector names the example's
// declaration line or, when the report states the example's shared-group
// inclusions, the line that included its shared group (LPCV-V0-137).
func lineSelected(parsed rubytestreceipt.Selector, location reportedLocation) bool {
	for _, line := range parsed.Lines {
		if location.file == parsed.Path && location.line == line {
			return true
		}
		for _, inclusion := range location.inclusions {
			if inclusion.file == parsed.Path && inclusion.line == line {
				return true
			}
		}
	}
	return false
}

// observeShared records an example's shared-example origin from the report's
// shared_group_inclusion_backtrace, which the stock RSpec JSON formatter does
// not emit and the Corvint formatter does (LPCV-V0-134, LPCV-V0-136). origin
// is the example's own worktree file:line.
func observeShared(backtrace json.RawMessage, project, origin string) (*SharedExample, string) {
	frames, absent := inclusionFrames(backtrace)
	if absent != "" {
		return nil, absent
	}
	shared := &SharedExample{Origin: origin, IncludedAt: []string{}}
	for _, frame := range frames {
		location := path.Join(project, frame.file) + ":" + strconv.Itoa(frame.line)
		if !rubytestreceipt.InclusionLocation(location) {
			return nil, rubytestreceipt.SharedInclusionUnbnd
		}
		shared.IncludedAt = append(shared.IncludedAt, location)
	}
	return shared, ""
}

// inclusionFrames reads the project-relative inclusion file:line of every
// frame of a shared_group_inclusion_backtrace, or the closed reason there is
// none: not-reported, not-shared or inclusion-unbound.
func inclusionFrames(backtrace json.RawMessage) ([]reportedInclusion, string) {
	if len(backtrace) == 0 || string(backtrace) == "null" {
		return nil, rubytestreceipt.SharedNotReported
	}
	var frames []struct {
		InclusionLocation *string `json:"inclusion_location"`
	}
	if err := json.Unmarshal(backtrace, &frames); err != nil {
		return nil, rubytestreceipt.SharedInclusionUnbnd
	}
	if len(frames) == 0 {
		return nil, rubytestreceipt.SharedNotShared
	}
	if len(frames) > rubytestreceipt.MaxInclusions {
		return nil, rubytestreceipt.SharedInclusionUnbnd
	}
	inclusions := make([]reportedInclusion, 0, len(frames))
	for _, frame := range frames {
		if frame.InclusionLocation == nil {
			return nil, rubytestreceipt.SharedInclusionUnbnd
		}
		raw := *frame.InclusionLocation
		colon := strings.LastIndexByte(raw, ':')
		if colon < 0 {
			return nil, rubytestreceipt.SharedInclusionUnbnd
		}
		line, ok := rubytestreceipt.LocationLine(raw[colon+1:])
		file := path.Clean(strings.TrimPrefix(raw[:colon], "./"))
		if !ok || !fs.ValidPath(file) || file == "." {
			return nil, rubytestreceipt.SharedInclusionUnbnd
		}
		inclusions = append(inclusions, reportedInclusion{file: file, line: line})
	}
	return inclusions, ""
}
