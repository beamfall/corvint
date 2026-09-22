// Package witness reports the unwitnessed surface of one committed change
// range: of the obligations that range opens, how many were closed by evidence
// from a party other than the change's author, how many stand unproven, and how
// many could not be determined at all.
//
// The denominator is the universe admitted, never the result set produced.
// Every path the range touches enters the report; a path no analyzer admits is
// named and counted as NOT_RUN rather than dropped, so an empty closed set can
// never read as "nothing outstanding".
package witness

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
)

// Profile is the frozen report identity.
const Profile = "corvint-witness/0"

// impactLimit is the ranked-result ceiling contextindex admits. Results ranked
// out by it lose their identity, so the surface they represent is counted as
// undetermined rather than silently absent.
const impactLimit = 50

const maxDiffBytes = 8 << 20

// Verdict is one obligation's closure state. NOT_RUN is never a weak form of
// CLOSED: it records an obligation admitted to the denominator and left
// undecided.
type Verdict string

const (
	Closed   Verdict = "CLOSED"
	Unproven Verdict = "UNPROVEN"
	NotRun   Verdict = "NOT_RUN"
)

// Kind separates the surface the range wrote from the surface it may have
// broken without touching.
type Kind string

const (
	KindChange Kind = "CHANGE"
	KindImpact Kind = "IMPACT"
)

// Authority is one witness-authority class and the determination of whether
// evidence carrying it closes an obligation. Closing is table data rather than
// a branch, so admitting a new authority is a row, not a rewrite of the
// verdict.
type Authority struct {
	Class    string `json:"class"`
	Closing  bool   `json:"closing"`
	Citation string `json:"citation"`
}

// authorities is the complete V0 authority table. No row closes: CF-V0-014
// denies caller-reported evidence any closing power, CF-V0-012 rules a
// producer-declared citation a retrieval aid rather than a witness, and
// CF-V0-031 rules a document the same change set authors caller-controlled,
// so evidence this range writes is authored by the party under audit.
var authorities = []Authority{
	{
		Class:    "CALLER_REPORTED",
		Closing:  false,
		Citation: "CF-V0-014 (docs/specs/change-frontier-v0.md): signatures over caller-controlled bytes, clean-target statements, passing rows, and exit zero cannot upgrade this authority",
	},
	{
		Class:    "PRODUCER_DECLARED",
		Closing:  false,
		Citation: "CF-V0-012 (docs/specs/change-frontier-v0.md): a producer-declared citation is a retrieval aid, never a witness",
	},
	{
		Class:    "SELF_AUTHORED",
		Closing:  false,
		Citation: "CF-V0-014 (docs/specs/change-frontier-v0.md): caller-reported evidence is non-closing so that an actor cannot certify its own work, and an evidence blob this range writes is authored by the party under audit; CF-V0-031 (accepted 2026-08-29) restates that invariant over authority resolution, where a document introduced or modified by the same declared change set is caller-controlled",
	},
}

func closes(class string) bool {
	for _, authority := range authorities {
		if authority.Class == class {
			return authority.Closing
		}
	}
	return false
}

// Witness is one candidate closer bound to one obligation.
type Witness struct {
	Source       string `json:"source"`
	HunkID       string `json:"hunkId"`
	Relation     string `json:"relation"`
	Authority    string `json:"authority"`
	EvidencePath string `json:"evidencePath"`
	Closing      bool   `json:"closing"`
}

// Obligation is one unit of the range's blast radius that requires witnessing.
type Obligation struct {
	ID   string `json:"id"`
	Kind Kind   `json:"kind"`
	Path string `json:"path"`
	// Relation is how the obligation entered the universe: the changed path
	// itself, or the ranked relation the closure engine reported.
	Relation string `json:"relation"`
	// Coverage is how deeply Corvint can analyse this path's language today.
	Coverage  string    `json:"coverage"`
	Verdict   Verdict   `json:"verdict"`
	Reason    string    `json:"reason"`
	Witnesses []Witness `json:"witnesses"`
}

// Source records one witness source and whether it was consulted at all.
type Source struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Path       string `json:"path"`
	Detail     string `json:"detail"`
	Hunks      int    `json:"hunks"`
	Basis      int    `json:"basis"`
	Unattached int    `json:"unattached"`
}

// Precondition names something that must exist before the report could claim
// more than it does. It is emitted instead of a fabricated trust level.
type Precondition struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
}

// Summary is the headline count. Ratios are per-mille integers so identical
// inputs produce identical bytes.
type Summary struct {
	Opened               int `json:"opened"`
	Analysed             int `json:"analysed"`
	Closed               int `json:"closed"`
	Unproven             int `json:"unproven"`
	NotRun               int `json:"notRun"`
	WitnessesExamined    int `json:"witnessesExamined"`
	WitnessesClosing     int `json:"witnessesClosing"`
	DeterminablePerMille int `json:"determinablePerMille"`
	ProvenPerMille       int `json:"provenPerMille"`
}

// Range identifies the immutable interval the report covers.
type Range struct {
	Base         string `json:"base"`
	BaseTree     string `json:"baseTree"`
	Head         string `json:"head"`
	HeadTree     string `json:"headTree"`
	ChangedPaths int    `json:"changedPaths"`
	// Admission is the committed-range engine's verdict on which changed paths
	// it will analyse at all (internal/contextindex.RangeImpact).
	Admission       string `json:"admission"`
	AdmissionDetail string `json:"admissionDetail"`
	// Closure is the reverse-dependency engine's verdict over the admitted
	// paths (internal/contextindex.Impact).
	Closure       string `json:"closure"`
	ClosureDetail string `json:"closureDetail"`
}

// Report is the complete deterministic result.
type Report struct {
	Profile       string         `json:"profile"`
	Range         Range          `json:"range"`
	Summary       Summary        `json:"summary"`
	Authorities   []Authority    `json:"authorities"`
	Sources       []Source       `json:"sources"`
	Preconditions []Precondition `json:"preconditions"`
	Obligations   []Obligation   `json:"obligations"`
}

// change is one raw entry of the admitted universe.
type change struct {
	status  string
	path    string
	newMode string
}

// Options configure one report.
type Options struct {
	// Base is the full immutable commit object ID the range starts at.
	Base string
	// CEMPath overrides the in-repository Change Evidence Map location.
	CEMPath string
}

// Compile builds the report for base..index.CommitRevision. It only reads: no
// file is written and no worktree state is mutated.
func Compile(ctx context.Context, index *contextindex.Index, options Options) (*Report, error) {
	budget := gitrun.NewDefaultBudget()
	baseTree, err := resolveBaseTree(ctx, budget, index, options.Base)
	if err != nil {
		return nil, err
	}
	changes, err := readChanges(ctx, budget, index, baseTree)
	if err != nil {
		return nil, err
	}
	admission := admit(ctx, index, options.Base)
	closure := reverseClosure(index, admission.paths)
	// A path whose facts the index dropped is not shallowly covered -- it is
	// not covered at all, and the receipt has to say which of the two it was.
	unparsed := index.UnparsedPaths()
	obligations := changeObligations(changes, admission, closure, unparsed)
	obligations = append(obligations, impactObligations(closure, changes, unparsed)...)
	source, document := readCEMSource(ctx, budget, index, options)
	source.Unattached = attachWitnesses(obligations, source, document)
	decide(obligations)
	sortObligations(obligations)

	report := &Report{
		Profile: Profile,
		Range: Range{
			Base: options.Base, BaseTree: baseTree,
			Head: index.CommitRevision, HeadTree: index.Revision,
			ChangedPaths:    len(changes),
			Admission:       admission.status,
			AdmissionDetail: admission.detail,
			Closure:         closure.status,
			ClosureDetail:   closure.detail,
		},
		Authorities: authorities,
		Sources:     []Source{source},
		Obligations: obligations,
	}
	report.Summary = summarize(obligations)
	report.Preconditions = preconditions(report)
	return report, nil
}

func resolveBaseTree(ctx context.Context, budget *gitrun.Budget, index *contextindex.Index, base string) (string, error) {
	if len(base) != len(index.Revision) || strings.TrimLeft(base, "0123456789abcdef") != "" {
		return "", fmt.Errorf("--base must be a full lowercase-hex commit object ID")
	}
	kind, err := run(ctx, budget, index.Root, 64, "cat-file", "-t", base)
	if err != nil || string(kind) != "commit\n" {
		return "", fmt.Errorf("--base must identify an available commit object")
	}
	if _, err := run(ctx, budget, index.Root, 64, "merge-base", "--is-ancestor", base, index.CommitRevision); err != nil {
		return "", fmt.Errorf("--base must be an ancestor of the captured HEAD commit")
	}
	tree, err := run(ctx, budget, index.Root, 128, "rev-parse", base+"^{tree}")
	if err != nil {
		return "", fmt.Errorf("cannot resolve the base commit tree")
	}
	return strings.TrimSuffix(string(tree), "\n"), nil
}

// readChanges enumerates the admitted universe. Renames are decomposed into a
// deletion and an addition so every raw record names exactly one path and no
// path can be lost to a rename heuristic.
func readChanges(ctx context.Context, budget *gitrun.Budget, index *contextindex.Index, baseTree string) ([]change, error) {
	raw, err := run(ctx, budget, index.Root, maxDiffBytes,
		"diff-tree", "--no-commit-id", "--raw", "-r", "-z", "--no-renames", baseTree, index.Revision)
	if err != nil {
		return nil, fmt.Errorf("cannot read the change range")
	}
	fields := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	result := make([]change, 0, len(fields)/2)
	for position := 0; position+1 < len(fields); position += 2 {
		metadata := strings.Fields(fields[position])
		if len(metadata) != 5 || len(metadata[0]) != 7 || metadata[0][0] != ':' {
			return nil, fmt.Errorf("Git raw range status is malformed")
		}
		result = append(result, change{status: metadata[4], path: fields[position+1], newMode: metadata[1]})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].path < result[right].path })
	return result, nil
}

// stage is one existing engine's verdict over this range: the paths it
// admitted, the ranked receipts it produced, and — where it refused or hit its
// own ceiling — exactly which paths that happened for.
type stage struct {
	status    string
	detail    string
	paths     []string
	receipts  []map[string]any
	refused   map[string]string
	truncated []string
}

// admit asks the committed-range engine which changed paths it will analyse.
// It is the sole authority on admission; this package adds no second one.
func admit(ctx context.Context, index *contextindex.Index, base string) stage {
	receipt, err := contextindex.RangeImpact(ctx, index, base, impactLimit)
	if err != nil {
		return stage{status: "REFUSED", detail: err.Error(), refused: map[string]string{}}
	}
	admitted := make([]string, 0)
	for _, item := range results(receipt) {
		if text(item["kind"]) == "path" {
			admitted = append(admitted, text(item["id"]))
		}
	}
	sort.Strings(admitted)
	return stage{status: "COMPUTED", paths: admitted, receipts: []map[string]any{receipt}, refused: map[string]string{}}
}

// reverseClosure runs the existing reverse-dependency engine over the admitted
// paths. RangeImpact qualifies the committed range but emits no reverse-import
// closure; Impact is where that closure lives, so the two are chained rather
// than reimplemented.
//
// It calls Impact once per admitted path rather than once for the whole set,
// because the engine truncates its ranked results to a fixed ceiling and its
// receipt counts that truncation as zero omissions. One call per path keeps a
// crowded path from silently discarding another path's dependents, and names
// every path that reached the ceiling on its own.
func reverseClosure(index *contextindex.Index, admitted []string) stage {
	result := stage{status: "COMPUTED", paths: admitted, refused: map[string]string{}, truncated: []string{}}
	if len(admitted) == 0 {
		result.status = "EMPTY"
		result.detail = "the admission stage admitted no path, so no reverse-dependency closure was opened"
		return result
	}
	for _, changedPath := range admitted {
		receipt, err := contextindex.Impact(index, []string{changedPath}, impactLimit)
		if err != nil {
			result.refused[changedPath] = err.Error()
			continue
		}
		if atCeiling(receipt) {
			result.truncated = append(result.truncated, changedPath)
		}
		result.receipts = append(result.receipts, receipt)
	}
	switch {
	case len(result.refused) == len(admitted):
		result.status = "REFUSED"
		result.detail = "the reverse-dependency engine refused every admitted path"
	case len(result.refused) != 0:
		result.status = "PARTIAL"
		result.detail = fmt.Sprintf("%d of %d admitted paths were refused by the reverse-dependency engine", len(result.refused), len(admitted))
	case len(result.truncated) != 0:
		result.status = "TRUNCATED"
		result.detail = fmt.Sprintf("%d of %d admitted paths reached the %d-result ranking ceiling, so their dependents beyond it are unenumerated",
			len(result.truncated), len(admitted), impactLimit)
	}
	return result
}

// atCeiling reports whether a receipt filled its ranked-result limit. The
// engine's own omitted_results cannot answer this: contextindex truncates the
// result slice before counting it, so the field reads zero however much was
// discarded.
func atCeiling(receipt map[string]any) bool {
	coverageMap, ok := receipt["coverage"].(map[string]any)
	if !ok {
		return false
	}
	included, ok := coverageMap["included_results"].(int)
	return ok && included >= impactLimit
}

// changeObligations opens one obligation per changed path and marks the subset
// the admission engine accepted. Every other path keeps a named reason, so no
// input is dropped from the denominator.
func changeObligations(changes []change, admission, closure stage, unparsed map[string]string) []Obligation {
	admitted := map[string]bool{}
	for _, item := range admission.paths {
		admitted[item] = true
	}
	result := make([]Obligation, 0, len(changes))
	for _, item := range changes {
		tier, tierReason := coverage(item, unparsed)
		obligation := Obligation{
			ID: "change:" + item.path, Kind: KindChange, Path: item.path,
			Relation: "changed-path", Coverage: tier, Witnesses: []Witness{},
		}
		switch {
		case admitted[item.path]:
			if _, refused := closure.refused[item.path]; refused {
				obligation.Verdict, obligation.Reason = NotRun, "CLOSURE_REFUSED"
			}
		case admission.status != "COMPUTED":
			obligation.Verdict, obligation.Reason = NotRun, "ADMISSION_REFUSED"
		case tier != "DEEP":
			obligation.Verdict, obligation.Reason = NotRun, tierReason
		default:
			obligation.Verdict, obligation.Reason = NotRun, "ADMITTED_PATH_NOT_RANKED"
		}
		result = append(result, obligation)
	}
	return result
}

// impactObligations opens one obligation per reverse-dependency the range did
// not itself touch, unioned across every per-path closure receipt.
func impactObligations(closure stage, changes []change, unparsed map[string]string) []Obligation {
	changed := map[string]bool{}
	for _, item := range changes {
		changed[item.path] = true
	}
	seen := map[string]bool{}
	result := make([]Obligation, 0)
	for _, receipt := range closure.receipts {
		for _, item := range results(receipt) {
			kind, identifier := text(item["kind"]), text(item["id"])
			if kind == "path" || identifier == "" || changed[identifier] || seen[kind+":"+identifier] {
				continue
			}
			seen[kind+":"+identifier] = true
			tier, _ := coverage(change{status: "M", path: identifier}, unparsed)
			result = append(result, Obligation{
				ID: "impact:" + kind + ":" + identifier, Kind: KindImpact, Path: identifier,
				Relation: kind, Coverage: tier, Witnesses: []Witness{},
			})
		}
	}
	return result
}

// coverage names how deeply Corvint can analyse one changed path today. Language
// coverage is uneven, and a weakly covered language widens NOT_RUN rather than
// shrinking it.
func coverage(item change, unparsed map[string]string) (string, string) {
	// An extractor that ran and dropped the file outranks the language's
	// nominal tier: the index holds no fact from this path, so reporting the
	// language's usual depth would overstate what was actually analysed.
	if reason, dropped := unparsed[item.path]; dropped && item.status != "D" {
		return "NONE", reason
	}
	switch {
	case item.status == "D":
		return "NONE", "PATH_DELETED_OUT_OF_PROFILE"
	case item.newMode == "120000":
		return "NONE", "SYMLINK_OUT_OF_PROFILE"
	case item.newMode == "160000":
		return "NONE", "GITLINK_OUT_OF_PROFILE"
	}
	switch strings.ToLower(path.Ext(item.path)) {
	case ".go":
		if path.Dir(item.path) == "." {
			return "DEEP", "ROOT_PACKAGE_OUT_OF_PROFILE"
		}
		return "DEEP", ""
	case ".py":
		return "SHALLOW", "LANGUAGE_COVERAGE_SHALLOW_PYTHON"
	// The lexically-extracted languages report GENERIC rather than SHALLOW on
	// purpose. SHALLOW is Python's tier, and Python is analysed by a validated
	// grammar that yields both definitions and imports. These languages are
	// read by a line scanner that yields definitions only: no import graph, no
	// syntax validation, and a declaration whose keyword the scanner does not
	// know is simply missed. That is more than the GENERIC languages get and
	// less than Python gets, and given the choice between the two existing
	// tiers the honest direction to round is down. The reason code carries the
	// detail the tier cannot.
	case ".rs":
		return "GENERIC", "LANGUAGE_COVERAGE_LEXICAL_SYMBOLS_RUST"
	case ".cs":
		return "GENERIC", "LANGUAGE_COVERAGE_LEXICAL_SYMBOLS_CSHARP"
	case ".swift":
		return "GENERIC", "LANGUAGE_COVERAGE_LEXICAL_SYMBOLS_SWIFT"
	case ".kt", ".kts":
		return "GENERIC", "LANGUAGE_COVERAGE_LEXICAL_SYMBOLS_KOTLIN"
	case ".rb":
		return "GENERIC", "LANGUAGE_COVERAGE_LEXICAL_SYMBOLS_RUBY"
	// Admitted as searchable text with no symbol extraction at all. `.m` is
	// shared by Objective-C, MATLAB and Mathematica, so it is deliberately left
	// text-only rather than analysed as whichever language was guessed.
	case ".sql", ".m":
		return "GENERIC", "LANGUAGE_COVERAGE_TEXT_ONLY"
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".sh":
		return "GENERIC", "LANGUAGE_COVERAGE_GENERIC_EXTRACTOR"
	default:
		return "NONE", "LANGUAGE_COVERAGE_NONE"
	}
}

// readCEMSource loads the Change Evidence Map from the head tree. The map is
// the repository's existing binding of claims to patch hunks; this package
// reuses it rather than inventing a parallel notion of evidence.
func readCEMSource(ctx context.Context, budget *gitrun.Budget, index *contextindex.Index, options Options) (Source, *wire.Map) {
	location := options.CEMPath
	if location == "" {
		location = wire.ExcludedCEMPath
	}
	source := Source{Name: "cem", Path: location}
	raw, err := run(ctx, budget, index.Root, wire.MaxMapBytes, "cat-file", "blob", index.Revision+":"+location)
	if err != nil {
		if cemcode.CodeOf(err) == cemcode.GitExitFailure {
			source.Status, source.Detail = committedAtPath(ctx, budget, index, location)
		} else {
			source.Status, source.Detail = "INVALID", err.Error()
		}
		return source, nil
	}
	document, err := wire.ParseMap(raw)
	if err != nil {
		source.Status, source.Detail = "INVALID", err.Error()
		return source, nil
	}
	source.Hunks = len(document.Hunks)
	for _, hunk := range document.Hunks {
		source.Basis += len(hunk.Basis)
	}
	if document.BaseRevision != options.Base {
		source.Status = "UNBOUND"
		source.Detail = "the map is bound to base " + document.BaseRevision + ", not the requested base; its citations are not applied"
		return source, nil
	}
	source.Status = "BOUND"
	return source, document
}

// committedAtPath settles a failed blob read. `git cat-file blob` exits
// non-zero both when nothing is committed at the path and when a tree or
// gitlink is, so only an empty literal ls-tree listing earns ABSENT.
func committedAtPath(ctx context.Context, budget *gitrun.Budget, index *contextindex.Index, location string) (string, string) {
	listing, err := run(ctx, budget, index.Root, 4096, "--literal-pathspecs", "ls-tree", "-z", "--full-tree", index.Revision, "--", location)
	if err != nil {
		return "INVALID", err.Error()
	}
	if len(listing) == 0 {
		return "ABSENT", "no Change Evidence Map is committed at this path in the head tree"
	}
	return "INVALID", "a non-blob object is committed at this path in the head tree"
}

// attachWitnesses binds each map citation to the obligation for the hunk's
// path and returns the number of citations whose hunk path lies outside the
// admitted universe, so a map that does not describe this range cannot appear
// to have covered it.
func attachWitnesses(obligations []Obligation, source Source, document *wire.Map) int {
	if source.Status != "BOUND" || document == nil {
		return 0
	}
	byPath := map[string]int{}
	for position := range obligations {
		if obligations[position].Kind == KindChange {
			byPath[obligations[position].Path] = position
		}
	}
	evidencePaths := map[string]string{}
	for _, record := range document.Evidence {
		evidencePaths[record.ID] = record.Path
	}
	changedPaths := map[string]bool{}
	for position := range obligations {
		if obligations[position].Kind == KindChange {
			changedPaths[obligations[position].Path] = true
		}
	}
	unattached := 0
	for _, hunk := range document.Hunks {
		position, ok := byPath[hunk.Path]
		if !ok {
			unattached += len(hunk.Basis)
			continue
		}
		for _, basis := range hunk.Basis {
			evidencePath := evidencePaths[basis.EvidenceID]
			class := classify(basis.Relation, evidencePath, changedPaths)
			obligations[position].Witnesses = append(obligations[position].Witnesses, Witness{
				Source: "cem", HunkID: hunk.ID, Relation: basis.Relation,
				Authority: class, EvidencePath: evidencePath, Closing: closes(class),
			})
		}
	}
	return unattached
}

// classify derives one citation's authority from repository facts rather than
// from a label the citation carries. A test claim is caller-reported by
// CF-V0-014; evidence the range itself writes is self-authored; everything else
// is a producer declaration.
func classify(relation, evidencePath string, changedPaths map[string]bool) string {
	if relation == "test-claim" {
		return "CALLER_REPORTED"
	}
	if evidencePath != "" && changedPaths[evidencePath] {
		return "SELF_AUTHORED"
	}
	return "PRODUCER_DECLARED"
}

// decide resolves each obligation. NOT_RUN is sticky: an obligation whose blast
// radius is unknown is never upgraded by a witness, because the extent of what
// the witness would have to cover is itself unknown.
func decide(obligations []Obligation) {
	for position := range obligations {
		if obligations[position].Verdict == NotRun {
			continue
		}
		obligations[position].Verdict, obligations[position].Reason = Unproven, "NO_CLOSING_AUTHORITY"
		for _, item := range obligations[position].Witnesses {
			if item.Closing {
				obligations[position].Verdict, obligations[position].Reason = Closed, ""
				break
			}
		}
	}
}

var kindRank = map[Kind]int{KindChange: 0, KindImpact: 1}

func sortObligations(obligations []Obligation) {
	sort.SliceStable(obligations, func(left, right int) bool {
		if kindRank[obligations[left].Kind] != kindRank[obligations[right].Kind] {
			return kindRank[obligations[left].Kind] < kindRank[obligations[right].Kind]
		}
		if obligations[left].Path != obligations[right].Path {
			return obligations[left].Path < obligations[right].Path
		}
		return obligations[left].ID < obligations[right].ID
	})
	for position := range obligations {
		sort.SliceStable(obligations[position].Witnesses, func(left, right int) bool {
			items := obligations[position].Witnesses
			if items[left].HunkID != items[right].HunkID {
				return items[left].HunkID < items[right].HunkID
			}
			if items[left].Relation != items[right].Relation {
				return items[left].Relation < items[right].Relation
			}
			return items[left].EvidencePath < items[right].EvidencePath
		})
	}
}

func summarize(obligations []Obligation) Summary {
	result := Summary{}
	for _, item := range obligations {
		result.Opened++
		switch item.Verdict {
		case Closed:
			result.Closed++
		case Unproven:
			result.Unproven++
		case NotRun:
			result.NotRun++
		}
		for _, candidate := range item.Witnesses {
			result.WitnessesExamined++
			if candidate.Closing {
				result.WitnessesClosing++
			}
		}
	}
	result.Analysed = result.Opened - result.NotRun
	if result.Opened > 0 {
		result.DeterminablePerMille = result.Analysed * 1000 / result.Opened
	}
	if result.Analysed > 0 {
		result.ProvenPerMille = result.Closed * 1000 / result.Analysed
	}
	return result
}

// preconditions names what would have to exist before the report could report a
// higher closed count, so a zero numerator is read as a missing capability
// rather than as a clean result.
func preconditions(report *Report) []Precondition {
	result := make([]Precondition, 0, 4)
	if !anyClosingAuthority() {
		result = append(result, Precondition{
			ID:        "WITNESS-P1-NO-CLOSING-AUTHORITY",
			Statement: "no authority class admitted in V0 can close an obligation, so the closed count is structurally zero; tcq/0 and frontier/0 are implemented in this repository and still close nothing, because CF-V0-014 freezes the TCQ relation as non-closing and CF-V0-012 makes a lexical candidate a retrieval aid rather than a witness; closing requires a NEW profile identifier under CF-V0-029, not a reinterpretation of V0 bytes",
		})
	}
	result = append(result, Precondition{
		ID:        "WITNESS-P2-AUTHORITY-LABELS-ARE-LITERALS",
		Statement: "the repository's existing authority labels are hardcoded constants, not computed verdicts (internal/lrf/types.go:12 assigns every CEM basis row \"producer-declared\"; internal/worktreeimpact/compiler.go:445 emits confidence \"authoritative\"); this report therefore derives each citation's authority from repository facts and reports no trust level built on those literals",
	})
	if report.Sources[0].Status != "BOUND" {
		result = append(result, Precondition{
			ID:        "WITNESS-P3-NO-BOUND-EVIDENCE-MAP",
			Statement: "no Change Evidence Map is bound to this base, so no citation was examined for any obligation: " + report.Sources[0].Detail,
		})
	}
	if report.Range.Admission != "COMPUTED" {
		result = append(result, Precondition{
			ID:        "WITNESS-P4-NO-BLAST-RADIUS",
			Statement: "the committed-range engine admitted no part of this range, so every obligation is NOT_RUN and no reverse-dependency surface was opened: " + report.Range.AdmissionDetail,
		})
	}
	if report.Range.Closure != "COMPUTED" && report.Range.Admission == "COMPUTED" {
		result = append(result, Precondition{
			ID:        "WITNESS-P6-INCOMPLETE-REVERSE-DEPENDENCY-CLOSURE",
			Statement: "the reverse-dependency surface is not fully enumerated, so obligations this range may have broken without touching are missing from the denominator: " + report.Range.ClosureDetail,
		})
	}
	if report.Summary.NotRun > 0 && report.Range.Admission == "COMPUTED" {
		result = append(result, Precondition{
			ID:        "WITNESS-P5-PARTIAL-LANGUAGE-COVERAGE",
			Statement: fmt.Sprintf("%d of %d obligations are undetermined; Go is the only language the range impact engine admits, and Python contributes no import edges (internal/contextindex/parse.go:496)", report.Summary.NotRun, report.Summary.Opened),
		})
	}
	return result
}

func anyClosingAuthority() bool {
	for _, item := range authorities {
		if item.Closing {
			return true
		}
	}
	return false
}

func run(ctx context.Context, budget *gitrun.Budget, root string, limit int, args ...string) ([]byte, error) {
	return gitrun.Run(ctx, budget, gitrun.Options{Dir: root, Env: gitEnvironment(), StdoutLimit: limit}, append([]string{"-c", "advice.graftFileDeprecated=false"}, args...)...)
}

// gitEnvironment builds the same scrubbed child environment its peers
// (internal/tracerecordrepo, internal/tracemigraterepo, internal/genesis)
// pass to gitrun.Run: an explicit allowlist of the host variables Git needs,
// plus fixed determinism and non-interactivity settings. An ambient
// GIT_DIR or GIT_WORK_TREE in this process's own environment is not on the
// allowlist, so it never reaches the child.
func gitEnvironment() []string {
	environment := make([]string, 0, 16)
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment,
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0",
		"GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE="+os.DevNull, "GCM_INTERACTIVE=never", "GIT_ASKPASS=",
	)
}

func results(impact map[string]any) []map[string]any {
	rows, ok := impact["results"].([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if typed, ok := row.(map[string]any); ok {
			result = append(result, typed)
		}
	}
	return result
}

func text(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}
