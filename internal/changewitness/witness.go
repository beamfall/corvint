// Package changewitness is the experimental pure evaluator for the
// `ocm-change-witnessed-v0` relation (docs/specs/change-witness-relation-v0.md).
//
// It decides, for one OCM obligation, whether a referenced hunk changes lines
// inside the unique Go definition its requirement span names. It reads no
// path, index, network, or worktree: every input is bytes the caller took from
// verified Git objects. No Frontier profile consumes it yet; `frontier/0`
// neither emits nor recognises the relation (CWR-V0-001).
package changewitness

import (
	"bytes"
	"sort"
)

// Relation is the CWR-V0-001 relation string; AuthorityClass is CWR-V0-003's.
const (
	Relation       = "ocm-change-witnessed-v0"
	AuthorityClass = "VERIFIER_DERIVED"
)

// Outcomes (CWR-V0-015).
const (
	Witnessed = "witnessed"
	Withheld  = "withheld"
	Abstained = "abstained"
)

// Reasons and diagnostic codes (CWR-V0-015). Only SelfAuthoredObligation is a
// Frontier reason; an abstained obligation keeps its frontier/0 reason.
const (
	ReasonNotLinked          = "OBLIGATION_NOT_LINKED"
	SelfAuthoredObligation   = "SELF_AUTHORED_OBLIGATION"
	ReasonSourceUnparsed     = "RESOLVER_SOURCE_UNPARSED"
	ReasonNoNamedIdentifier  = "NO_NAMED_IDENTIFIER"
	ReasonNoIntersection     = "NO_MATERIAL_INTERSECTION"
	ReasonIneligibleHunk     = "INELIGIBLE_WITNESS_HUNK"
	IdentifierUnresolved     = "IDENTIFIER_UNRESOLVED"
	IdentifierAmbiguous      = "IDENTIFIER_AMBIGUOUS"
	LanguageUnsupported      = "RESOLVER_LANGUAGE_UNSUPPORTED"
	linkedDisposition        = "linked"
	supportedHunkDisposition = "supported"
)

// Input is one obligation's verified relation inputs.
type Input struct {
	Disposition string   // OCM obligation disposition
	IntentPath  string   // OCM intent path
	TargetSpan  []byte   // requirement span bytes at the target revision
	BasePresent bool     // intent path and obligation ID both exist at the expected base
	BaseSpan    []byte   // requirement span bytes at the expected base
	Sources     []Source // every blob of the target tree; only `.go` paths are read
	Hunks       []Hunk   // hunks the obligation structurally references
}

// Source is one target-revision blob read from Git objects.
type Source struct {
	Path, BlobOID string
	Bytes         []byte
}

// Hunk is one canonical CEM hunk. Changed holds its target-side changed line
// ranges; WhitespaceOnly marks a hunk the verifier reverified as
// whitespace-only or line-ending-only (CF-V0-008).
type Hunk struct {
	ID, Path, Disposition string
	WhitespaceOnly        bool
	Changed               []LineRange
}

// LineRange is a 1-based start and a line count; a zero count is empty.
type LineRange struct{ Start, Count int }

// Result is one evaluation. Witnesses is set only when Outcome is Witnessed;
// HunkIDs only when Withheld (CWR-V0-009 keeps the referenced hunk IDs).
type Result struct {
	Outcome, Reason string
	HunkIDs         []string
	Witnesses       []Witness
	Diagnostics     []Diagnostic
}

// Witness is one qualifying identifier/definition/hunk triple.
type Witness struct {
	Identifier, Path, BlobOID string
	StartLine, EndLine        int
	HunkID                    string
}

// Diagnostic is a value-free count (CWR-V0-011).
type Diagnostic struct {
	Code  string
	Count int
}

// Evaluate applies CWR-V0-004's conditions in order and reports the first
// that fails, or every qualifying witness.
func Evaluate(in Input) Result {
	if in.Disposition != linkedDisposition {
		return Result{Outcome: Abstained, Reason: ReasonNotLinked}
	}
	if !baseStable(in) {
		return Result{Outcome: Withheld, Reason: SelfAuthoredObligation, HunkIDs: hunkIDs(in.Hunks)}
	}
	definitions, unparsed := goDefinitions(in.Sources)
	if unparsed {
		return Result{Outcome: Abstained, Reason: ReasonSourceUnparsed}
	}
	named, counts := nameIdentifiers(identifierTokens(in.TargetSpan), definitions)
	counts[LanguageUnsupported] = unsupportedHunks(in.Hunks)
	diagnostics := diagnosticList(counts)
	if len(named) == 0 {
		return Result{Outcome: Abstained, Reason: ReasonNoNamedIdentifier, Diagnostics: diagnostics}
	}
	intersecting := intersections(named, in.Hunks)
	if len(intersecting) == 0 {
		return Result{Outcome: Abstained, Reason: ReasonNoIntersection, Diagnostics: diagnostics}
	}
	witnesses := eligible(intersecting, in.IntentPath)
	if len(witnesses) == 0 {
		return Result{Outcome: Abstained, Reason: ReasonIneligibleHunk, Diagnostics: diagnostics}
	}
	return Result{Outcome: Witnessed, Witnesses: witnesses, Diagnostics: diagnostics}
}

// baseStable is CWR-V0-005: the span exists at base with identical bytes.
func baseStable(in Input) bool {
	if !in.BasePresent {
		return false
	}
	return bytes.Equal(in.BaseSpan, in.TargetSpan)
}

func hunkIDs(hunks []Hunk) []string {
	seen := map[string]bool{}
	for _, hunk := range hunks {
		seen[hunk.ID] = true
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// nameIdentifiers keeps tokens with exactly one definition (CWR-V0-006) and
// counts the rest by diagnostic code.
func nameIdentifiers(tokens []string, definitions map[string][]definition) ([]definition, map[string]int) {
	counts := map[string]int{}
	named := []definition{}
	for _, token := range tokens {
		found := definitions[token]
		switch len(found) {
		case 0:
			counts[IdentifierUnresolved]++
		case 1:
			named = append(named, found[0])
		default:
			counts[IdentifierAmbiguous]++
		}
	}
	return named, counts
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func unsupportedHunks(hunks []Hunk) int {
	count := 0
	for _, hunk := range hunks {
		count += boolCount(!isGoPath(hunk.Path))
	}
	return count
}

func diagnosticList(counts map[string]int) []Diagnostic {
	list := []Diagnostic{}
	for code, count := range counts {
		if count == 0 {
			continue
		}
		list = append(list, Diagnostic{Code: code, Count: count})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Code < list[j].Code })
	return list
}

// candidate is one material intersection before the CWR-V0-008 filter.
type candidate struct {
	witness Witness
	hunk    Hunk
}

// intersections is CWR-V0-007: a non-empty overlap of a changed target range
// with a definition span in the same path; whitespace-only hunks never count.
func intersections(named []definition, hunks []Hunk) []candidate {
	found := []candidate{}
	for _, hunk := range hunks {
		found = append(found, hunkIntersections(named, hunk)...)
	}
	return found
}

func hunkIntersections(named []definition, hunk Hunk) []candidate {
	found := []candidate{}
	if hunk.WhitespaceOnly {
		return found
	}
	for _, def := range named {
		if !touches(def, hunk) {
			continue
		}
		found = append(found, candidate{Witness{def.name, def.path, def.blobOID, def.start, def.end, hunk.ID}, hunk})
	}
	return found
}

func touches(def definition, hunk Hunk) bool {
	if def.path != hunk.Path {
		return false
	}
	for _, changed := range hunk.Changed {
		if overlaps(changed, def.start, def.end) {
			return true
		}
	}
	return false
}

func overlaps(changed LineRange, start, end int) bool {
	if changed.Count <= 0 {
		return false
	}
	last := changed.Start + changed.Count - 1
	return changed.Start <= end && last >= start
}

// eligible is CWR-V0-004 condition 5 and CWR-V0-008.
func eligible(found []candidate, intentPath string) []Witness {
	witnesses := []Witness{}
	for _, item := range found {
		if item.hunk.Disposition != supportedHunkDisposition {
			continue
		}
		if item.hunk.Path == intentPath {
			continue
		}
		witnesses = append(witnesses, item.witness)
	}
	sort.Slice(witnesses, func(i, j int) bool { return witnessLess(witnesses[i], witnesses[j]) })
	return witnesses
}

func witnessLess(a, b Witness) bool {
	if a.Identifier != b.Identifier {
		return a.Identifier < b.Identifier
	}
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	return a.HunkID < b.HunkID
}
