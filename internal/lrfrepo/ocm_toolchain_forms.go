package lrfrepo

import (
	"bytes"
	"regexp"
	"strings"
)

// EXPERIMENTAL (STA-V0, docs/specs/spec-toolchain-adapters-v0.md): declared
// OCM intent forms for OpenSpec and Spec Kit specs. Like every OIF-V0 form they
// are selected only by the caller's --intent-form, never inferred from a path
// or content, and LRF refuses them (OIF-V0-010).

const (
	// IntentFormOpenSpecSpec reads the `### Requirement:` blocks of an
	// OpenSpec main spec's `## Requirements` section (STA-V0-002).
	IntentFormOpenSpecSpec = "openspec-spec"
	// IntentFormOpenSpecDelta reads the ADDED and MODIFIED requirement blocks
	// of one OpenSpec change delta file (STA-V0-003).
	IntentFormOpenSpecDelta = "openspec-delta"
	// IntentFormSpecKit reads the FR and SC lines of a Spec Kit spec.md
	// (STA-V0-005).
	IntentFormSpecKit = "speckit-spec"
)

var (
	openSpecRequirementHeader = regexp.MustCompile(`^###\s*(?i:requirement):\s*(.+?)\s*$`)
	openSpecClosingRun        = regexp.MustCompile(`[ \t]+#+[ \t]*$`)
	openSpecObligationID      = regexp.MustCompile(`^OSR-[0-9A-F]{12}$`)
	openSpecDeltaHeading      = regexp.MustCompile(`^## ((?i:added|modified|removed|renamed)) (?i:requirements)[ \t]*$`)
	openSpecRenameFrom        = regexp.MustCompile("^\\s*[-*+]?\\s*FROM:\\s*`?###\\s*Requirement:\\s*(.+?)`?\\s*$")
	openSpecRenameMarker      = regexp.MustCompile(`^\s*[-*+]?\s*(FROM|TO):`)
	openSpecRenameTo          = regexp.MustCompile("^\\s*[-*+]?\\s*TO:\\s*`?###\\s*Requirement:\\s*(.+?)`?\\s*$")
	// openSpecRemovedBullet is upstream's bullet form of a REMOVED entry,
	// "- `### Requirement: Name`" with optional backticks (V1-1707).
	openSpecRemovedBullet  = regexp.MustCompile("^\\s*-\\s*`?###\\s*Requirement:\\s*(.+?)`?\\s*$")
	specKitRequirementID   = regexp.MustCompile(`^(?:FR|SC)-[0-9]{3,}$`)
	specKitRequirementLine = regexp.MustCompile(`^- \*\*((?:FR|SC)-[0-9]{3,})\*\*: `)
	specKitRequirements    = regexp.MustCompile(`^## Requirements(?: \*\([^)\n]*\)\*)?[ \t]*$`)
	specKitSuccessCriteria = regexp.MustCompile(`^## Success Criteria(?: \*\([^)\n]*\)\*)?[ \t]*$`)
)

// ToolchainPrepareMembers are the PrepareResult members only the STA-V0 forms
// set; the CLI emits them as prepare envelope members.
type ToolchainPrepareMembers struct {
	// NativeNames maps each OpenSpec obligation ID to its requirement name
	// (STA-V0-004).
	NativeNames map[string]string
	// ExcludedRequirements lists OpenSpec REMOVED/RENAMED entries and Spec Kit
	// clarification-marked requirements, in document order (STA-V0-003,
	// STA-V0-005).
	ExcludedRequirements []map[string]string
	// NativeStatus is the Spec Kit `**Status**:` text, nil when absent.
	NativeStatus *string
}

func (derived intentDerivation) toolchainMembers() ToolchainPrepareMembers {
	return ToolchainPrepareMembers{
		NativeNames: derived.nativeNames, ExcludedRequirements: derived.excludedRequirements, NativeStatus: derived.nativeStatus,
	}
}

var (
	specKitStatusMarker        = []byte("**Status**:")
	specKitClarificationMarker = []byte("[NEEDS CLARIFICATION")
)

// openSpecRequirementName reports whether line is a `### Requirement:` header
// and returns its native name. Like upstream normalizeRequirementName, a
// closing `#` run counts only when a space or tab precedes it, so `C#` keeps
// its `#`; the name is otherwise byte-exact and case-sensitive.
func openSpecRequirementName(line []byte) (string, bool) {
	match := openSpecRequirementHeader.FindSubmatch(line)
	if match == nil {
		return "", false
	}
	return strings.TrimSpace(openSpecClosingRun.ReplaceAllString(string(match[1]), "")), true
}

// openSpecObligation derives the STA-V0-002 obligation ID from a native name.
func openSpecObligation(name string) string {
	return "OSR-" + strings.ToUpper(sha256Hex([]byte(name))[:12])
}

func markdownLevel2Line(line []byte) bool {
	return bytes.Equal(line, []byte("##")) || bytes.HasPrefix(line, []byte("## "))
}

func openSpecLevel3Line(line []byte) bool {
	_, header := openSpecRequirementName(line)
	return header || markdownLevel3.Match(line)
}

type openSpecBlock struct {
	name       string
	start, end int
}

// openSpecBlocks reads every requirement block between the heading at start and
// end. Any other unfenced level-3 heading is refused, never skipped, and a
// block runs to the next unfenced level-3 or level-2 heading.
func openSpecBlocks(data []byte, lines []int, fenced []bool, start, end int) ([]openSpecBlock, error) {
	blocks := make([]openSpecBlock, 0)
	for number, offset := range lines {
		line := lineWithoutEnding(data, offset)
		if offset <= start || offset >= end || fenced[number] || !openSpecLevel3Line(line) {
			continue
		}
		name, header := openSpecRequirementName(line)
		if !header || name == "" {
			return nil, fail("invalid-requirement-item", "requirements heading is not a named ### Requirement: block")
		}
		blocks = append(blocks, openSpecBlock{name: name, start: offset, end: openSpecBlockEnd(data, lines, fenced, number, end)})
	}
	return blocks, nil
}

func openSpecBlockEnd(data []byte, lines []int, fenced []bool, header, end int) int {
	for number := header + 1; number < len(lines) && lines[number] < end; number++ {
		line := lineWithoutEnding(data, lines[number])
		if !fenced[number] && (openSpecLevel3Line(line) || markdownLevel2Line(line)) {
			return lines[number]
		}
	}
	return end
}

func addOpenSpecRequirements(derived *intentDerivation, data []byte, blocks []openSpecBlock) error {
	for _, block := range blocks {
		identity := openSpecObligation(block.name)
		if _, repeated := derived.statements[identity]; repeated {
			return fail("duplicate-requirement", "OpenSpec intent repeats a requirement name")
		}
		derived.requirements = append(derived.requirements, identity)
		derived.statements[identity] = data[block.start:block.end]
		derived.nativeNames[identity] = block.name
	}
	return nil
}

func openSpecSpecDerivation(path, oid string, data []byte) (intentDerivation, error) {
	lines := lineOffsets(data)
	fenced := fencedLines(data, lines)
	headings := headingStarts(data, lines, fenced, []byte("## Requirements"))
	if len(headings) != 1 {
		return intentDerivation{}, fail("invalid-requirements-section", "OpenSpec spec must contain exactly one ## Requirements heading")
	}
	start := headings[0]
	end := sectionEnd(data, lines, fenced, start)
	blocks, err := openSpecBlocks(data, lines, fenced, start, end)
	if err != nil {
		return intentDerivation{}, err
	}
	derived := intentDerivation{statements: map[string][]byte{}, nativeNames: map[string]string{}}
	if err := addOpenSpecRequirements(&derived, data, blocks); err != nil {
		return intentDerivation{}, err
	}
	return finishDerivation(derived, path, oid, data, start, end)
}

type openSpecDeltaSection struct {
	operation  string
	start, end int
}

func openSpecDeltaDerivation(path, oid string, data []byte) (intentDerivation, error) {
	lines := lineOffsets(data)
	fenced := fencedLines(data, lines)
	sections := openSpecDeltaSections(data, lines, fenced)
	if len(sections) == 0 {
		return intentDerivation{}, fail("missing-requirements", "OpenSpec delta has no delta section")
	}
	if err := refuseStrayRequirementBlocks(data, lines, fenced, sections); err != nil {
		return intentDerivation{}, err
	}
	derived := intentDerivation{statements: map[string][]byte{}, nativeNames: map[string]string{}, excludedRequirements: []map[string]string{}}
	for _, section := range sections {
		var err error
		switch section.operation {
		case "ADDED", "MODIFIED":
			var blocks []openSpecBlock
			if blocks, err = openSpecBlocks(data, lines, fenced, section.start, section.end); err == nil {
				err = addOpenSpecRequirements(&derived, data, blocks)
			}
		case "REMOVED":
			derived.excludedRequirements = append(derived.excludedRequirements, openSpecRemoved(data, lines, fenced, section)...)
		default:
			var renamed []map[string]string
			if renamed, err = openSpecRenamed(data, lines, fenced, section); err == nil {
				derived.excludedRequirements = append(derived.excludedRequirements, renamed...)
			}
		}
		if err != nil {
			return intentDerivation{}, err
		}
	}
	return finishDerivation(derived, path, oid, data, 0, len(data))
}

// openSpecDeltaSections returns every unfenced delta heading (repeatable,
// case-insensitive) with its operation and section bounds, in document order.
func openSpecDeltaSections(data []byte, lines []int, fenced []bool) []openSpecDeltaSection {
	sections := make([]openSpecDeltaSection, 0)
	for number, offset := range lines {
		if fenced[number] {
			continue
		}
		if match := openSpecDeltaHeading.FindSubmatch(lineWithoutEnding(data, offset)); match != nil {
			sections = append(sections, openSpecDeltaSection{
				operation: strings.ToUpper(string(match[1])), start: offset, end: sectionEnd(data, lines, fenced, offset),
			})
		}
	}
	return sections
}

// refuseStrayRequirementBlocks fails a requirement header outside the four
// delta sections, or inside RENAMED, whose entries are FROM:/TO: lines.
func refuseStrayRequirementBlocks(data []byte, lines []int, fenced []bool, sections []openSpecDeltaSection) error {
	for number, offset := range lines {
		if fenced[number] {
			continue
		}
		if _, header := openSpecRequirementName(lineWithoutEnding(data, offset)); !header {
			continue
		}
		inside := false
		for _, section := range sections {
			if offset > section.start && offset < section.end {
				inside = section.operation != "RENAMED"
			}
		}
		if !inside {
			return fail("invalid-delta-section", "OpenSpec requirement block is outside the ADDED, MODIFIED and REMOVED sections")
		}
	}
	return nil
}

func openSpecRemoved(data []byte, lines []int, fenced []bool, section openSpecDeltaSection) []map[string]string {
	removed := make([]map[string]string, 0)
	for number, offset := range lines {
		if offset <= section.start || offset >= section.end || fenced[number] {
			continue
		}
		line := lineWithoutEnding(data, offset)
		if name, header := openSpecRequirementName(line); header {
			removed = append(removed, map[string]string{"operation": "REMOVED", "name": name})
		} else if match := openSpecRemovedBullet.FindSubmatch(line); match != nil && openSpecRenameName(match[1]) != "" {
			removed = append(removed, map[string]string{"operation": "REMOVED", "name": openSpecRenameName(match[1])})
		}
	}
	return removed
}

// openSpecRenamed pairs each FROM: line with the next TO: line. A FROM: displaced
// by another FROM:, a TO: with no pending FROM:, or a FROM: pending at the
// section end fails, as upstream reports each as an unpaired rename. So does a
// FROM:/TO: line that names no requirement.
func openSpecRenamed(data []byte, lines []int, fenced []bool, section openSpecDeltaSection) ([]map[string]string, error) {
	renamed := make([]map[string]string, 0)
	pending := ""
	pendingSet := false
	unpaired := fail("invalid-delta-section", "OpenSpec RENAMED entry is not a FROM:/TO: pair")
	for number, offset := range lines {
		if offset <= section.start || offset >= section.end || fenced[number] {
			continue
		}
		line := lineWithoutEnding(data, offset)
		if match := openSpecRenameFrom.FindSubmatch(line); match != nil {
			if pendingSet || openSpecRenameName(match[1]) == "" {
				return nil, unpaired
			}
			pending, pendingSet = openSpecRenameName(match[1]), true
		} else if match := openSpecRenameTo.FindSubmatch(line); match != nil {
			if !pendingSet || openSpecRenameName(match[1]) == "" {
				return nil, unpaired
			}
			renamed = append(renamed, map[string]string{"operation": "RENAMED", "from": pending, "to": openSpecRenameName(match[1])})
			pendingSet = false
		} else if openSpecRenameMarker.Match(line) {
			return nil, unpaired
		}
	}
	if pendingSet {
		return nil, unpaired
	}
	return renamed, nil
}

func openSpecRenameName(raw []byte) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(openSpecClosingRun.ReplaceAllString(string(raw), "")), "`"))
}

func specKitSpecDerivation(path, oid string, data []byte) (intentDerivation, error) {
	lines := lineOffsets(data)
	fenced := fencedLines(data, lines)
	requirements := specKitSections(data, lines, fenced, specKitRequirements)
	criteria := specKitSections(data, lines, fenced, specKitSuccessCriteria)
	if len(requirements) != 1 || len(criteria) > 1 {
		return intentDerivation{}, fail("invalid-requirements-section", "Spec Kit spec must contain exactly one ## Requirements and at most one ## Success Criteria section")
	}
	derived := intentDerivation{statements: map[string][]byte{}, excludedRequirements: []map[string]string{}, nativeStatus: specKitStatus(data, lines, fenced)}
	seen := map[string]bool{}
	for number, offset := range lines {
		if fenced[number] {
			continue
		}
		match := specKitRequirementLine.FindSubmatch(lineWithoutEnding(data, offset))
		if match == nil || !specKitLineInSection(string(match[1]), offset, requirements, criteria) {
			continue
		}
		identity := string(match[1])
		if !validObligationID(IntentFormSpecKit, identity) {
			return intentDerivation{}, fail("invalid-obligation-id", "Spec Kit requirement ID exceeds the obligation ID limit")
		}
		if seen[identity] {
			return intentDerivation{}, fail("duplicate-requirement", "Spec Kit spec repeats a requirement ID")
		}
		seen[identity] = true
		statement := specKitStatement(data, lines, number)
		if bytes.Contains(statement, specKitClarificationMarker) {
			derived.excludedRequirements = append(derived.excludedRequirements, map[string]string{"id": identity, "reason": "needs-clarification"})
			continue
		}
		derived.requirements = append(derived.requirements, identity)
		derived.statements[identity] = statement
	}
	return finishDerivation(derived, path, oid, data, 0, len(data))
}

// specKitSections returns the [start, end) bounds of every unfenced level-2
// heading matching heading. The `*(mandatory)*` annotation the upstream spec
// template carries is accepted; the section includes its ### subsections.
func specKitSections(data []byte, lines []int, fenced []bool, heading *regexp.Regexp) [][2]int {
	sections := make([][2]int, 0, 1)
	for number, offset := range lines {
		if !fenced[number] && heading.Match(lineWithoutEnding(data, offset)) {
			sections = append(sections, [2]int{offset, sectionEnd(data, lines, fenced, offset)})
		}
	}
	return sections
}

// specKitLineInSection keeps FR lines to the Requirements section and SC lines
// to the Success Criteria section.
func specKitLineInSection(identity string, offset int, requirements, criteria [][2]int) bool {
	sections := requirements
	if strings.HasPrefix(identity, "SC-") {
		sections = criteria
	}
	for _, section := range sections {
		if offset > section[0] && offset < section[1] {
			return true
		}
	}
	return false
}

// specKitStatement is the requirement line plus the following lines indented
// at least two spaces.
func specKitStatement(data []byte, lines []int, number int) []byte {
	statement := append([]byte(nil), lineWithEnding(data, lines, number)...)
	for next := number + 1; next < len(lines); next++ {
		if !bytes.HasPrefix(lineWithoutEnding(data, lines[next]), []byte("  ")) {
			break
		}
		statement = append(statement, lineWithEnding(data, lines, next)...)
	}
	return statement
}

// specKitStatus returns the first unfenced `**Status**:` line's text in the
// header (before the first level-2 heading), trimmed of surrounding spaces and
// tabs, or nil when the header has none.
func specKitStatus(data []byte, lines []int, fenced []bool) *string {
	for number, offset := range lines {
		line := lineWithoutEnding(data, offset)
		if fenced[number] {
			continue
		}
		if markdownLevel2Line(line) {
			return nil
		}
		if rest, found := bytes.CutPrefix(line, specKitStatusMarker); found {
			status := string(bytes.Trim(rest, " \t"))
			return &status
		}
	}
	return nil
}
