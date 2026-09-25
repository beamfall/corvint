package lrfrepo

import (
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// EXPERIMENTAL (OIF-V0, docs/specs/ocm-intent-forms-v0.md, intent proposed):
// declared OCM intent forms. The caller names the form; nothing here guesses it
// from content. The default form ("" on the wire and in memory) is the
// OCM-V0-001 `## Requirements` section and stays byte-identical.

const (
	// IntentFormADR reads each numbered `### N.` item under the one Decisions
	// heading from adrDecisionsHeadings.
	IntentFormADR = "adr-decisions"
	// IntentFormRoadmap reads each roadmap ticket that carries an Acceptance line.
	IntentFormRoadmap = "roadmap-acceptance"
	maxTicketIDBytes  = 64
)

var (
	adrNumberLine      = regexp.MustCompile(`^adr: ([0-9]{4})$`)
	adrDecisionHeading = regexp.MustCompile(`^### ([1-9][0-9]{0,2}[a-z]?)\. \S`)
	markdownLevel3     = regexp.MustCompile(`^ {0,3}###(?:[ \t]|$)`)
	adrDecisionID      = regexp.MustCompile(`^ADR-[0-9]{4}-D[1-9][0-9]{0,2}[a-z]?$`)
	roadmapTaskItem    = regexp.MustCompile(`^- \[[^\]]\] `)
	roadmapTicketHead  = regexp.MustCompile(`^- \[[^\]]\] \*\*(.+?)\*\*`)
	roadmapTicketID    = regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:-[A-Za-z0-9]+)+$`)
)

var roadmapAcceptanceMarker = []byte("  - **Acceptance:**")

// adrDecisionsHeadings is the closed OIF-V0-005 set of Decisions section headings.
var adrDecisionsHeadings = map[string]bool{
	"## Decisions":    true,
	"## Decision":     true,
	"## 2. Decisions": true,
	"## 2. Decision":  true,
}

// intentDerivation is everything one intent form derives from a pinned blob.
// statements is nil for the default form, whose statements stay the
// requirement lines requirementStatement finds in scope.
type intentDerivation struct {
	intent       ocmIntent
	requirements []string
	scope        []byte
	statements   map[string][]byte
	excluded     []string
}

var intentFormParsers = map[string]func(path, oid string, data []byte) (intentDerivation, error){
	"":                requirementsDerivation,
	IntentFormADR:     adrDecisionsDerivation,
	IntentFormRoadmap: roadmapAcceptanceDerivation,
}

var intentFormIDs = map[string]*regexp.Regexp{
	"":                requirementID,
	IntentFormADR:     adrDecisionID,
	IntentFormRoadmap: roadmapTicketID,
}

func deriveIntent(form, path, oid string, data []byte) (intentDerivation, error) {
	parse, known := intentFormParsers[form]
	if !known {
		return intentDerivation{}, fail("invalid-intent-form", "intent form is not a declared OCM intent form")
	}
	if !utf8.Valid(data) {
		return intentDerivation{}, fail("invalid-intent", "intent scope must be UTF-8 Markdown")
	}
	derived, err := parse(path, oid, data)
	derived.intent.form = form
	return derived, err
}

func validObligationID(form, identity string) bool {
	return len(identity) <= maxTicketIDBytes && intentFormIDs[form].MatchString(identity)
}

func declaresIntentForm(object *wire.Object) bool {
	_, declared := object.Get("form")
	return declared
}

// parseIntentForm reads the optional wire member. Absence is the default form;
// an explicit default or unknown name is refused so each form has one encoding.
func parseIntentForm(object *wire.Object) (string, error) {
	if !declaresIntentForm(object) {
		return "", nil
	}
	form, err := stringMember(object, "form")
	if _, known := intentFormParsers[form]; err != nil || form == "" || !known {
		return "", fail("invalid-intent-form", "intent form is not a declared OCM intent form")
	}
	return form, nil
}

// refuseDeclaredIntentForm keeps LRF-feeding surfaces on the default form: the
// LRF evaluator reads only OCM-V0-001 requirement lines.
func refuseDeclaredIntentForm(document *ocmDocument) error {
	if document.intent.form != "" {
		return fail("unsupported-intent-form", "LRF accepts only the default OCM intent form")
	}
	return nil
}

func obligationStatement(statements map[string][]byte, scope []byte, identity string) ([]byte, error) {
	if statements == nil {
		return requirementStatement(scope, identity)
	}
	return append([]byte(nil), statements[identity]...), nil
}

func requirementsDerivation(path, oid string, data []byte) (intentDerivation, error) {
	intent, requirements, scope, err := requirementsFromBlob(path, oid, data)
	return intentDerivation{intent: intent, requirements: requirements, scope: scope}, err
}

func adrDecisionsDerivation(path, oid string, data []byte) (intentDerivation, error) {
	lines := lineOffsets(data)
	fenced := fencedLines(data, lines)
	number, err := adrNumber(data, lines)
	if err != nil {
		return intentDerivation{}, err
	}
	headings := adrDecisionsHeadingStarts(data, lines, fenced)
	if len(headings) != 1 {
		return intentDerivation{}, fail("invalid-decisions-section", "ADR intent must contain exactly one Decisions heading from the OIF-V0-005 set")
	}
	start := headings[0]
	end := sectionEnd(data, lines, fenced, start)
	items, err := adrDecisionItems(data, lines, fenced, start, end)
	if err != nil {
		return intentDerivation{}, err
	}
	derived := intentDerivation{statements: map[string][]byte{}}
	for index, item := range items {
		identity := "ADR-" + number + "-D" + item.label
		if _, repeated := derived.statements[identity]; repeated {
			return intentDerivation{}, fail("duplicate-requirement", "Decisions section repeats a decision number")
		}
		derived.requirements = append(derived.requirements, identity)
		derived.statements[identity] = data[item.start:adrItemEnd(items, index, end)]
	}
	return finishDerivation(derived, path, oid, data, start, end)
}

// adrDecisionsHeadingStarts returns every unfenced line in adrDecisionsHeadings.
func adrDecisionsHeadingStarts(data []byte, lines []int, fenced []bool) []int {
	headings := make([]int, 0, 1)
	for number, start := range lines {
		if !fenced[number] && adrDecisionsHeadings[string(lineWithoutEnding(data, start))] {
			headings = append(headings, start)
		}
	}
	return headings
}

// adrNumber reads the one `adr: NNNN` line of the leading `---` front matter.
func adrNumber(data []byte, lines []int) (string, error) {
	if !bytes.Equal(lineWithoutEnding(data, 0), []byte("---")) {
		return "", fail("invalid-adr-number", "ADR intent must open with front matter")
	}
	numbers := make([]string, 0, 1)
	for _, offset := range lines[1:] {
		line := lineWithoutEnding(data, offset)
		if bytes.Equal(line, []byte("---")) {
			return oneADRNumber(numbers)
		}
		if match := adrNumberLine.FindSubmatch(line); match != nil {
			numbers = append(numbers, string(match[1]))
		}
	}
	return "", fail("invalid-adr-number", "ADR front matter is not closed")
}

func oneADRNumber(numbers []string) (string, error) {
	if len(numbers) != 1 {
		return "", fail("invalid-adr-number", "ADR front matter must carry exactly one adr: NNNN line")
	}
	return numbers[0], nil
}

type adrDecision struct {
	label string
	start int
}

// adrDecisionItems requires every unfenced level-3 heading in the section to be
// a numbered decision; any other level-3 heading is refused, never skipped.
func adrDecisionItems(data []byte, lines []int, fenced []bool, start, end int) ([]adrDecision, error) {
	items := make([]adrDecision, 0)
	for number, offset := range lines {
		line := lineWithoutEnding(data, offset)
		if offset <= start || offset >= end || fenced[number] || !markdownLevel3.Match(line) {
			continue
		}
		match := adrDecisionHeading.FindSubmatch(line)
		if match == nil {
			return nil, fail("invalid-decision-item", "Decisions heading is not a numbered ### N. item")
		}
		items = append(items, adrDecision{label: string(match[1]), start: offset})
	}
	return items, nil
}

func adrItemEnd(items []adrDecision, index, end int) int {
	if index+1 < len(items) {
		return items[index+1].start
	}
	return end
}

func roadmapAcceptanceDerivation(path, oid string, data []byte) (intentDerivation, error) {
	lines := lineOffsets(data)
	fenced := fencedLines(data, lines)
	derived := intentDerivation{statements: map[string][]byte{}}
	seen := map[string]bool{}
	for number, offset := range lines {
		line := lineWithoutEnding(data, offset)
		if fenced[number] || !roadmapTaskItem.Match(line) {
			continue
		}
		identity, err := roadmapTicketIdentity(line)
		if err != nil {
			return intentDerivation{}, err
		}
		if seen[identity] {
			return intentDerivation{}, fail("duplicate-requirement", "roadmap intent repeats a ticket ID")
		}
		seen[identity] = true
		acceptance := roadmapAcceptance(data, lines, fenced, number+1)
		if len(acceptance) == 0 {
			derived.excluded = append(derived.excluded, identity)
			continue
		}
		derived.requirements = append(derived.requirements, identity)
		derived.statements[identity] = acceptance
	}
	return finishDerivation(derived, path, oid, data, 0, len(data))
}

func roadmapTicketIdentity(line []byte) (string, error) {
	match := roadmapTicketHead.FindSubmatch(line)
	if match == nil {
		return "", fail("invalid-ticket-item", "roadmap task item is not a closed **ID — title** ticket")
	}
	identity, title, found := strings.Cut(string(match[1]), " — ")
	if !found || strings.TrimSpace(title) == "" || !validObligationID(IntentFormRoadmap, identity) {
		return "", fail("invalid-ticket-item", "roadmap ticket ID or title is invalid")
	}
	return identity, nil
}

// roadmapAcceptance returns the ticket's `  - **Acceptance:**` sub-bullets
// (inline text or a nested list) and their deeper-indented continuation lines.
// The ticket body ends at the first empty or unindented line.
func roadmapAcceptance(data []byte, lines []int, fenced []bool, from int) []byte {
	var acceptance []byte
	inAcceptance := false
	for number := from; number < len(lines); number++ {
		line := lineWithoutEnding(data, lines[number])
		if len(line) == 0 || line[0] != ' ' {
			break
		}
		opens := !fenced[number] && roadmapAcceptanceOpens(line)
		inAcceptance = opens || inAcceptance && bytes.HasPrefix(line, []byte("    "))
		if inAcceptance {
			acceptance = append(acceptance, lineWithEnding(data, lines, number)...)
		}
	}
	return acceptance
}

func roadmapAcceptanceOpens(line []byte) bool {
	rest, found := bytes.CutPrefix(line, roadmapAcceptanceMarker)
	return found && (len(rest) == 0 || rest[0] == ' ')
}

func lineWithEnding(data []byte, lines []int, number int) []byte {
	if number+1 < len(lines) {
		return data[lines[number]:lines[number+1]]
	}
	return data[lines[number]:]
}

func finishDerivation(derived intentDerivation, path, oid string, data []byte, start, end int) (intentDerivation, error) {
	if len(derived.requirements) == 0 {
		return intentDerivation{}, fail("missing-requirements", "intent form yields no requirements")
	}
	if len(derived.requirements) > maxObligations {
		return intentDerivation{}, fail("too-many-obligations", "intent form exceeds the obligation limit")
	}
	derived.scope = data[start:end]
	derived.intent = ocmIntent{path: path, blobOID: oid, start: int64(start), end: int64(end), spanSHA256: sha256Hex(derived.scope)}
	return derived, nil
}
