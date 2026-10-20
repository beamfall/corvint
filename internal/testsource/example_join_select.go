package testsource

import (
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
)

// Caller-asserted requirement IDs on selected examples (TSA-V0-417..419).

const (
	// RequirementBasisCallerAsserted marks requirement IDs the caller supplied; Corvint never
	// infers them.
	RequirementBasisCallerAsserted = "caller-asserted"
	// MaxRequirementSelections bounds the selectors one join takes.
	MaxRequirementSelections = 256
	// MaxRequirementIDs bounds the requirement IDs one selector names.
	MaxRequirementIDs = 32
)

// RequirementSelection is one `SELECTOR=REQ[,REQ...]` pair as the caller gave it.
type RequirementSelection struct {
	Selector string
	IDs      []string
}

// ExampleRequirements is the caller-asserted echo on one record.
type ExampleRequirements struct {
	Basis     string   `json:"basis"`
	IDs       []string `json:"ids"`
	Selectors []string `json:"selectors"`
}

var (
	requirementID     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)
	selectorScopedID  = regexp.MustCompile(`^([^\[\]]+)\[[1-9][0-9]{0,5}(?::[1-9][0-9]{0,5}){0,31}\]$`)
	selectorLocation  = regexp.MustCompile(`^(.+):([1-9][0-9]{0,8})$`)
	selectorForbidden = "=,[] \t\x00\n\r\\"
)

func malformed(format string, args ...any) error {
	return refuse("tests-join-selector-malformed", format, args...)
}

// ParseRequirementSelection reads one `SELECTOR=REQ[,REQ...]` pair, refusing
// tests-join-selector-malformed when it does not parse (TSA-V0-417, TSA-V0-418).
func ParseRequirementSelection(pair string) (RequirementSelection, error) {
	selector, list, ok := strings.Cut(pair, "=")
	if !ok || !validSelector(selector) {
		return RequirementSelection{}, malformed("%q is not SELECTOR=REQ[,REQ...] with a PATH, PATH:LINE or PATH[ID] selector", pair)
	}
	ids := strings.Split(list, ",")
	if len(ids) > MaxRequirementIDs {
		return RequirementSelection{}, malformed("%q names more than %d requirement IDs", selector, MaxRequirementIDs)
	}
	for _, id := range ids {
		if !requirementID.MatchString(id) {
			return RequirementSelection{}, malformed("%q is not a requirement ID", id)
		}
	}
	return RequirementSelection{Selector: selector, IDs: ids}, nil
}

// ParseRequirementMap reads a mapping file: one pair per line, blank and `#` lines skipped.
func ParseRequirementMap(text string) ([]RequirementSelection, error) {
	out := []RequirementSelection{}
	for number, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		selection, err := ParseRequirementSelection(line)
		if err != nil {
			return nil, malformed("line %d: %s", number+1, err.(*Error).Message)
		}
		out = append(out, selection)
	}
	return out, nil
}

// validSelector accepts PATH, PATH:LINE or PATH[ID] over a repository-relative path.
func validSelector(selector string) bool {
	file := selector
	if m := selectorScopedID.FindStringSubmatch(selector); m != nil {
		file = m[1]
	} else if m := selectorLocation.FindStringSubmatch(selector); m != nil {
		file = m[1]
	}
	return file != "" && fs.ValidPath(file) && file != "." && file != ".git" && !strings.HasPrefix(file, ".git/") && !strings.ContainsAny(file, selectorForbidden)
}

// matches reports whether a selector names a row: PATH its identity file, PATH:LINE its receipt
// location or a Ruby inclusion site, PATH[ID] its RSpec identity.
func (row joined) matches(selector string) bool {
	if selectorScopedID.MatchString(selector) {
		return row.record.IdentityKind == ExampleIdentityRSpec && row.record.Example == selector
	}
	if selectorLocation.MatchString(selector) {
		return fmt.Sprintf("%s:%d", row.record.File, row.record.Line) == selector || slices.Contains(row.included, selector)
	}
	return row.path != "" && row.path == selector
}

// applySelections echoes each selection's IDs on the rows it matches. A repeated selector or a
// bound overrun refuses tests-join-selector-malformed and a selector matching no row refuses
// tests-join-selector-unknown (TSA-V0-418, TSA-V0-419).
func applySelections(rows []joined, selections []RequirementSelection) error {
	if len(selections) > MaxRequirementSelections {
		return malformed("more than %d selectors", MaxRequirementSelections)
	}
	seen := map[string]bool{}
	for _, selection := range selections {
		if !validSelector(selection.Selector) || len(selection.IDs) == 0 || len(selection.IDs) > MaxRequirementIDs {
			return malformed("%q is not a selector with 1 to %d requirement IDs", selection.Selector, MaxRequirementIDs)
		}
		if seen[selection.Selector] {
			return malformed("selector %q is repeated", selection.Selector)
		}
		seen[selection.Selector] = true
		for _, id := range selection.IDs {
			if !requirementID.MatchString(id) {
				return malformed("%q is not a requirement ID", id)
			}
		}
	}
	for _, selection := range selections {
		matched := false
		for index := range rows {
			if !rows[index].matches(selection.Selector) {
				continue
			}
			matched = true
			echo := rows[index].record.Requirements
			if echo == nil {
				echo = &ExampleRequirements{Basis: RequirementBasisCallerAsserted, IDs: []string{}, Selectors: []string{}}
				rows[index].record.Requirements = echo
			}
			echo.IDs = append(echo.IDs, selection.IDs...)
			echo.Selectors = append(echo.Selectors, selection.Selector)
		}
		if !matched {
			return refuse("tests-join-selector-unknown", "selector %q matches no example in the receipt", selection.Selector)
		}
	}
	for index := range rows {
		if echo := rows[index].record.Requirements; echo != nil {
			slices.Sort(echo.IDs)
			echo.IDs = slices.Compact(echo.IDs)
		}
	}
	return nil
}
