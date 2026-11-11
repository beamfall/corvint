package rubytestreceipt

import (
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
	MaxSelectorBytes = 1024
	maxLocationLine  = 10_000_000
	// MaxInclusions bounds a retained shared-group inclusion backtrace.
	MaxInclusions        = 4
	MaxInclusionLocation = 512

	SharedNotShared      = "not-shared"
	SharedNotReported    = "not-reported"
	SharedInclusionUnbnd = "inclusion-unbound"
)

// Selector is one parsed --spec value: a project-relative file or directory,
// path:line[:line...], or path[scoped-id(,scoped-id)...].
type Selector struct {
	Kind  string
	Path  string
	Lines []int
	Ids   []string
}

// ParseSelector validates one --spec value. A value whose suffix is not a
// well-formed line list or id list is a plain path, as RSpec reads it.
func ParseSelector(value string) (Selector, error) {
	if value == "" || len(value) > MaxSelectorBytes || strings.ContainsRune(value, 0) {
		return Selector{}, errors.New("--spec selector is empty or over its bound")
	}
	parsed := Selector{Kind: SelectorPath}
	name := value
	if open := strings.LastIndexByte(value, '['); open > 0 && strings.HasSuffix(value, "]") {
		if ids, ok := scopedIDs(value[open+1 : len(value)-1]); ok {
			parsed.Kind, parsed.Ids, name = SelectorID, ids, value[:open]
		}
	}
	if parsed.Kind == SelectorPath {
		rest := name
		var lines []int
		for {
			colon := strings.LastIndexByte(rest, ':')
			if colon < 0 {
				break
			}
			line, ok := LocationLine(rest[colon+1:])
			if !ok {
				break
			}
			lines = append([]int{line}, lines...)
			rest = rest[:colon]
		}
		if len(lines) > 0 {
			parsed.Kind, parsed.Lines, name = SelectorLine, lines, rest
		}
	}
	if !filepath.IsLocal(name) {
		return Selector{}, errors.New("--spec must be a project-relative path")
	}
	parsed.Path = path.Clean(strings.TrimPrefix(filepath.ToSlash(name), "./"))
	if !fs.ValidPath(parsed.Path) || parsed.Kind != SelectorPath && parsed.Path == "." {
		return Selector{}, errors.New("--spec must be a project-relative path")
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

func LocationLine(text string) (int, bool) {
	if text == "" || len(text) > 8 || strings.Trim(text, "0123456789") != "" {
		return 0, false
	}
	line, err := strconv.Atoi(text)
	return line, err == nil && line >= 1 && line <= maxLocationLine
}

func validateSelection(document Document) error {
	if document.SelectionWithheld != "" && (document.SelectionWithheld != "secret-shaped" || document.Selection != nil) {
		return errors.New("Ruby provider selection is withheld with an unknown reason or also carried")
	}
	if len(document.Selection) > MaxSelectors {
		return errors.New("Ruby provider selection is over its bound")
	}
	for _, outcome := range document.Selection {
		parsed, err := ParseSelector(outcome.Selector)
		if err != nil || parsed.Kind != outcome.Kind || secretscreen.MatchString(outcome.Selector) {
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

func InclusionLocation(location string) bool {
	colon := strings.LastIndexByte(location, ':')
	if colon < 0 || len(location) > MaxInclusionLocation || secretscreen.MatchString(location) {
		return false
	}
	// fs.ValidPath admits NUL and other control characters; a retained
	// inclusion never carries one.
	if strings.ContainsFunc(location, func(c rune) bool { return c < 0x20 || c == 0x7f }) {
		return false
	}
	_, ok := LocationLine(location[colon+1:])
	return ok && localSlashPath(location[:colon])
}

func ValidateShared(example Example) error {
	switch {
	case example.SharedExample != nil && example.SharedExampleAbsent != "":
		return errors.New("Ruby provider example carries both a shared-example origin and its absence")
	case example.SharedExample != nil:
		shared := example.SharedExample
		if shared.Origin != example.ID || len(shared.IncludedAt) < 1 || len(shared.IncludedAt) > MaxInclusions {
			return errors.New("Ruby provider shared-example origin is malformed")
		}
		for _, location := range shared.IncludedAt {
			if !InclusionLocation(location) {
				return errors.New("Ruby provider shared-example inclusion is malformed")
			}
		}
	case example.SharedExampleAbsent != "" && example.SharedExampleAbsent != SharedNotShared && example.SharedExampleAbsent != SharedNotReported && example.SharedExampleAbsent != SharedInclusionUnbnd:
		return errors.New("Ruby provider shared-example absence is outside the closed set")
	}
	return nil
}
