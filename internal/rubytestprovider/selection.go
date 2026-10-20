package rubytestprovider

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Selector kinds and per-selector outcomes (LPCV-V0-131, LPCV-V0-132).
const (
	SelectorPath = "path"
	SelectorLine = "line"
	SelectorID   = "id"

	// SelectorMatched: the report attributes at least one example to it.
	SelectorMatched = "MATCHED"
	// SelectorNoExample: the selector matched no example.
	SelectorNoExample = "NO_EXAMPLE"
	// SelectorUnattributed: examples ran from the selector's file but none is
	// declared on its lines, so the report cannot attribute them (a line
	// naming a group, a line inside an example, or a shared-group inclusion
	// line when the report does not state inclusions, LPCV-V0-137).
	SelectorUnattributed = "UNATTRIBUTED"
	// SelectorNotEvaluated: the run produced no readable report.
	SelectorNotEvaluated = "NOT_EVALUATED"

	// MaxSelectors bounds the --spec selectors of one run.
	MaxSelectors     = 256
	maxSelectorBytes = 1024
	maxLocationLine  = 10_000_000
	// maxInclusions bounds a retained shared-group inclusion backtrace.
	maxInclusions        = 4
	maxInclusionLocation = 512

	sharedNotShared      = "not-shared"
	sharedNotReported    = "not-reported"
	sharedInclusionUnbnd = "inclusion-unbound"
)

// selector is one parsed --spec value: a project-relative file or directory,
// path:line[:line...], or path[scoped-id(,scoped-id)...].
type selector struct {
	kind  string
	path  string
	lines []int
	ids   []string
}

// parseSelector validates one --spec value. A value whose suffix is not a
// well-formed line list or id list is a plain path, as RSpec reads it.
func parseSelector(value string) (selector, error) {
	if value == "" || len(value) > maxSelectorBytes || strings.ContainsRune(value, 0) {
		return selector{}, errors.New("--spec selector is empty or over its bound")
	}
	parsed := selector{kind: SelectorPath}
	name := value
	if open := strings.LastIndexByte(value, '['); open > 0 && strings.HasSuffix(value, "]") {
		if ids, ok := scopedIDs(value[open+1 : len(value)-1]); ok {
			parsed.kind, parsed.ids, name = SelectorID, ids, value[:open]
		}
	}
	if parsed.kind == SelectorPath {
		rest := name
		var lines []int
		for {
			colon := strings.LastIndexByte(rest, ':')
			if colon < 0 {
				break
			}
			line, ok := locationLine(rest[colon+1:])
			if !ok {
				break
			}
			lines = append([]int{line}, lines...)
			rest = rest[:colon]
		}
		if len(lines) > 0 {
			parsed.kind, parsed.lines, name = SelectorLine, lines, rest
		}
	}
	if !filepath.IsLocal(name) {
		return selector{}, errors.New("--spec must be a project-relative path")
	}
	parsed.path = path.Clean(strings.TrimPrefix(filepath.ToSlash(name), "./"))
	if !fs.ValidPath(parsed.path) || parsed.kind != SelectorPath && parsed.path == "." {
		return selector{}, errors.New("--spec must be a project-relative path")
	}
	return parsed, nil
}

func scopedIDs(list string) ([]string, bool) {
	var ids []string
	for _, id := range strings.Split(list, ",") {
		for _, component := range strings.Split(id, ":") {
			if component == "" || len(component) > 9 || strings.Trim(component, "0123456789") != "" {
				return nil, false
			}
		}
		ids = append(ids, id)
	}
	return ids, true
}

func locationLine(text string) (int, bool) {
	if text == "" || len(text) > 8 || strings.Trim(text, "0123456789") != "" {
		return 0, false
	}
	line, err := strconv.Atoi(text)
	return line, err == nil && line >= 1 && line <= maxLocationLine
}

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
		parsed, _ := parseSelector(value)
		outcome := SelectorOutcome{Selector: value, Kind: parsed.kind, Outcome: SelectorNotEvaluated}
		if examples != nil {
			matched, fromFile := 0, false
			for _, location := range locations {
				switch parsed.kind {
				case SelectorPath:
					if under(location.rerunPath, parsed.path) || under(location.file, parsed.path) {
						matched++
					}
				case SelectorLine:
					fromFile = fromFile || location.file == parsed.path || location.rerunPath == parsed.path
					if lineSelected(parsed, location) {
						matched++
					}
				case SelectorID:
					for _, id := range parsed.ids {
						if location.rerunPath == parsed.path && (location.scopedID == id || strings.HasPrefix(location.scopedID, id+":")) {
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
func lineSelected(parsed selector, location reportedLocation) bool {
	for _, line := range parsed.lines {
		if location.file == parsed.path && location.line == line {
			return true
		}
		for _, inclusion := range location.inclusions {
			if inclusion.file == parsed.path && inclusion.line == line {
				return true
			}
		}
	}
	return false
}

func validateSelection(document Document) error {
	if document.SelectionWithheld != "" && (document.SelectionWithheld != "secret-shaped" || document.Selection != nil) {
		return errors.New("Ruby provider selection is withheld with an unknown reason or also carried")
	}
	if len(document.Selection) > MaxSelectors {
		return errors.New("Ruby provider selection is over its bound")
	}
	for _, outcome := range document.Selection {
		parsed, err := parseSelector(outcome.Selector)
		if err != nil || parsed.kind != outcome.Kind || secretscreen.MatchString(outcome.Selector) {
			return errors.New("Ruby provider selector is malformed")
		}
		switch outcome.Outcome {
		case SelectorMatched:
			if outcome.Matched < 1 {
				return errors.New("Ruby provider matched selector has no example count")
			}
		case SelectorNoExample, SelectorUnattributed, SelectorNotEvaluated:
			if outcome.Matched != 0 {
				return errors.New("Ruby provider unmatched selector carries an example count")
			}
		default:
			return errors.New("Ruby provider selector outcome is outside the closed set")
		}
	}
	return nil
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
		if !inclusionLocation(location) {
			return nil, sharedInclusionUnbnd
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
		return nil, sharedNotReported
	}
	var frames []struct {
		InclusionLocation *string `json:"inclusion_location"`
	}
	if err := json.Unmarshal(backtrace, &frames); err != nil {
		return nil, sharedInclusionUnbnd
	}
	if len(frames) == 0 {
		return nil, sharedNotShared
	}
	if len(frames) > maxInclusions {
		return nil, sharedInclusionUnbnd
	}
	inclusions := make([]reportedInclusion, 0, len(frames))
	for _, frame := range frames {
		if frame.InclusionLocation == nil {
			return nil, sharedInclusionUnbnd
		}
		raw := *frame.InclusionLocation
		colon := strings.LastIndexByte(raw, ':')
		if colon < 0 {
			return nil, sharedInclusionUnbnd
		}
		line, ok := locationLine(raw[colon+1:])
		file := path.Clean(strings.TrimPrefix(raw[:colon], "./"))
		if !ok || !fs.ValidPath(file) || file == "." {
			return nil, sharedInclusionUnbnd
		}
		inclusions = append(inclusions, reportedInclusion{file: file, line: line})
	}
	return inclusions, ""
}

func inclusionLocation(location string) bool {
	colon := strings.LastIndexByte(location, ':')
	if colon < 0 || len(location) > maxInclusionLocation || secretscreen.MatchString(location) {
		return false
	}
	// fs.ValidPath admits NUL and other control characters; a retained
	// inclusion never carries one.
	if strings.ContainsFunc(location, func(c rune) bool { return c < 0x20 || c == 0x7f }) {
		return false
	}
	_, ok := locationLine(location[colon+1:])
	return ok && localSlashPath(location[:colon])
}

func validateShared(example Example) error {
	switch {
	case example.SharedExample != nil && example.SharedExampleAbsent != "":
		return errors.New("Ruby provider example carries both a shared-example origin and its absence")
	case example.SharedExample != nil:
		shared := example.SharedExample
		if shared.Origin != example.ID || len(shared.IncludedAt) < 1 || len(shared.IncludedAt) > maxInclusions {
			return errors.New("Ruby provider shared-example origin is malformed")
		}
		for _, location := range shared.IncludedAt {
			if !inclusionLocation(location) {
				return errors.New("Ruby provider shared-example inclusion is malformed")
			}
		}
	case example.SharedExampleAbsent != "" && example.SharedExampleAbsent != sharedNotShared && example.SharedExampleAbsent != sharedNotReported && example.SharedExampleAbsent != sharedInclusionUnbnd:
		return errors.New("Ruby provider shared-example absence is outside the closed set")
	}
	return nil
}
