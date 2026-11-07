package lrfrepo

import (
	"bytes"
	"math"
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
	// IntentFormRequirementsSubset reads the OCM-V0-001 Requirements section and
	// keeps only the requirement IDs the map lists in intentScope.requirements,
	// so a change can cite part of a spec over the obligation bound (OIF-V0-670
	// to OIF-V0-673, V1-2275).
	IntentFormRequirementsSubset = "requirements-subset"
	maxTicketIDBytes             = 64
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
	// nativeNames maps each OpenSpec obligation ID to its requirement name
	// (STA-V0-004); only the OpenSpec forms set it.
	nativeNames map[string]string
	// excludedRequirements lists the toolchain requirements that are not
	// obligations, in document order (STA-V0-003, STA-V0-005).
	excludedRequirements []map[string]string
	// nativeStatus is the Spec Kit `**Status**:` header text, nil when absent.
	nativeStatus *string
}

var intentFormParsers = map[string]func(path, oid string, data []byte) (intentDerivation, error){
	"":                      requirementsDerivation,
	IntentFormADR:           adrDecisionsDerivation,
	IntentFormRoadmap:       roadmapAcceptanceDerivation,
	IntentFormOpenSpecSpec:  openSpecSpecDerivation,
	IntentFormOpenSpecDelta: openSpecDeltaDerivation,
	IntentFormSpecKit:       specKitSpecDerivation,
	// The subset form derives only through deriveRecordedIntent, which carries
	// the listed IDs; this entry makes the name known and refuses a bare call.
	IntentFormRequirementsSubset: unlistedSubsetDerivation,
}

var intentFormIDs = map[string]*regexp.Regexp{
	"":                           requirementID,
	IntentFormADR:                adrDecisionID,
	IntentFormRoadmap:            roadmapTicketID,
	IntentFormOpenSpecSpec:       openSpecObligationID,
	IntentFormOpenSpecDelta:      openSpecObligationID,
	IntentFormSpecKit:            specKitRequirementID,
	IntentFormRequirementsSubset: requirementID,
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

// deriveRecordedIntent derives an intent with its recorded form and, for the
// requirements-subset form only, its listed requirement IDs (OIF-V0-671).
func deriveRecordedIntent(form string, subset []string, path, oid string, data []byte) (intentDerivation, error) {
	if form != IntentFormRequirementsSubset {
		if len(subset) != 0 {
			return intentDerivation{}, fail("invalid-intent-scope", "only the requirements-subset form lists requirement IDs")
		}
		return deriveIntent(form, path, oid, data)
	}
	if !utf8.Valid(data) {
		return intentDerivation{}, fail("invalid-intent", "intent scope must be UTF-8 Markdown")
	}
	return requirementsSubsetDerivation(path, oid, data, subset)
}

func unlistedSubsetDerivation(string, string, []byte) (intentDerivation, error) {
	return intentDerivation{}, fail("missing-requirements", "requirements-subset lists no requirement IDs")
}

// validRequirementSubset applies OIF-V0-670: 1 to 256 requirement IDs in
// strictly ascending byte order, so each subset has one encoding.
func validRequirementSubset(subset []string) error {
	if len(subset) == 0 {
		return fail("missing-requirements", "requirements-subset lists no requirement IDs")
	}
	if len(subset) > maxObligations {
		return fail("too-many-obligations", "requirements-subset lists more requirement IDs than the obligation limit")
	}
	for index, identity := range subset {
		if !requirementID.MatchString(identity) {
			return fail("invalid-intent-scope", "requirements-subset lists an invalid requirement ID")
		}
		if index > 0 && subset[index-1] >= identity {
			return fail("invalid-intent-scope", "requirements-subset IDs must be unique and in ascending order")
		}
	}
	return nil
}

// requirementsSubsetDerivation reads the whole Requirements section without the
// obligation bound, then keeps the listed IDs in document order. The span is
// the whole section, so its digest still binds every requirement line.
func requirementsSubsetDerivation(path, oid string, data []byte, subset []string) (intentDerivation, error) {
	if err := validRequirementSubset(subset); err != nil {
		return intentDerivation{}, err
	}
	intent, all, scope, err := scanRequirements(path, oid, data, math.MaxInt)
	if err != nil {
		return intentDerivation{}, err
	}
	listed := make(map[string]bool, len(subset))
	for _, identity := range subset {
		listed[identity] = true
	}
	requirements := make([]string, 0, len(subset))
	for _, identity := range all {
		if listed[identity] {
			requirements = append(requirements, identity)
		}
	}
	if len(requirements) != len(subset) {
		return intentDerivation{}, fail("unknown-subset-requirement", "requirements-subset lists an ID that is not in the Requirements section")
	}
	intent.form = IntentFormRequirementsSubset
	intent.subset = strings.Join(subset, ",")
	return intentDerivation{intent: intent, requirements: requirements, scope: scope}, nil
}

// intentSubset returns the recorded requirements-subset IDs; nil for any
// other form.
func intentSubset(intent ocmIntent) []string {
	if intent.subset == "" {
		return nil
	}
	return strings.Split(intent.subset, ",")
}

// parseIntentSubset reads the intentScope.requirements wire member, present
// exactly when the form is requirements-subset (OIF-V0-671).
func parseIntentSubset(object *wire.Object, form string) (string, error) {
	value, declared := object.Get("requirements")
	if declared != (form == IntentFormRequirementsSubset) {
		return "", fail("invalid-intent-scope", "intentScope requirements is present exactly for the requirements-subset form")
	}
	if !declared {
		return "", nil
	}
	if value.Kind != wire.KindArray || len(value.Arr) > maxObligations {
		if value.Kind == wire.KindArray {
			return "", fail("too-many-obligations", "requirements-subset lists more requirement IDs than the obligation limit")
		}
		return "", fail("invalid-intent-scope", "intentScope requirements must be an array")
	}
	subset := make([]string, 0, len(value.Arr))
	for _, item := range value.Arr {
		if item.Kind != wire.KindString {
			return "", fail("invalid-intent-scope", "intentScope requirements must be strings")
		}
		subset = append(subset, item.Str)
	}
	if err := validRequirementSubset(subset); err != nil {
		return "", err
	}
	return strings.Join(subset, ","), nil
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
