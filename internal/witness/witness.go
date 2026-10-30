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
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/witnesspaths"
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
	// Test names the caller-reported test an --evidence witness came from
	// (AGW-V0-008); it is absent on every Change Evidence Map citation.
	Test string `json:"test,omitempty"`
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
	// Declaration is the base tree's path-policy rule that classified this
	// path; absent for every path no rule applies to (AGW-V0-042).
	Declaration *Declaration `json:"declaration,omitempty"`
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
	// Stable is present only for an exact cem/1.0 map, so a 0.x report keeps
	// its bytes (CEM-V1-070).
	Stable *StableBinding `json:"stable,omitempty"`
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
	// CandidateWitnessed counts obligations that stay open but carry at least
	// one witness; Unwitnessed counts open obligations with none. Together they
	// are Opened-Closed, so "a candidate exists" never reads as "nothing" and
	// "nothing" never reads as "a candidate exists" (AGW-V0-008).
	CandidateWitnessed int `json:"candidateWitnessed"`
	Unwitnessed        int `json:"unwitnessed"`
	// Exempt counts the obligations a declared inert path holds: determined,
	// never closed, and neither candidate-witnessed nor unwitnessed
	// (AGW-V0-043). It is omitted when zero.
	Exempt int `json:"exempt,omitempty"`
}

// Range identifies the immutable interval the report covers.
type Range struct {
	Base         string `json:"base"`
	BaseTree     string `json:"baseTree"`
	Head         string `json:"head"`
	HeadTree     string `json:"headTree"`
	ChangedPaths int    `json:"changedPaths"`
	// Admission is the committed-range engine's verdict on which changed paths
	// it will analyse at all (internal/contextindex.PerPathRangeImpact):
	// COMPUTED, PARTIAL when it refused some paths each on their own
	// (AGW-V0-004), or REFUSED when it refused the whole range.
	Admission       string `json:"admission"`
	AdmissionDetail string `json:"admissionDetail"`
	// Closure is the reverse-dependency engine's verdict over the admitted
	// paths (internal/contextindex.Impact).
	Closure       string `json:"closure"`
	ClosureDetail string `json:"closureDetail"`
	// HeadBinding is COMMIT-PINNED when the head is a historical commit read
	// only from the object store (AGW-V0-007); it is absent for the checked-out
	// HEAD, whose report bytes are unchanged.
	HeadBinding string `json:"headBinding,omitempty"`
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
	// PacketCoverage names the cost of every packet the report compiled, in
	// compile order. A stage that compiled no packet contributes no entry.
	PacketCoverage []PacketCoverage `json:"packetCoverage"`
	// PathPolicy is the base tree's declared path policy, present only when
	// the base tree holds one (AGW-V0-045).
	PathPolicy *PathPolicy `json:"pathPolicy,omitempty"`
}

// PacketCoverage names one compiled packet (AGW-V0-003). A PRODUCED entry
// carries the packet's coverage fields; a NOT_PRODUCED entry carries a reason
// and no numbers, because its receipt had no readable coverage block.
type PacketCoverage struct {
	Stage  string `json:"stage"`
	Path   string `json:"path,omitempty"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	*PacketCounts
}

// PacketCounts copies a packet's coverage fields under the packet's own names.
type PacketCounts struct {
	PacketBytes     int  `json:"packet_bytes"`
	BudgetBytes     *int `json:"budget_bytes"`
	WithinBudget    bool `json:"within_budget"`
	IncludedResults int  `json:"included_results"`
	OmittedResults  int  `json:"omitted_results"`
}

// change is one raw entry of the admitted universe.
type change struct {
	status  string
	path    string
	newMode string
	// oldMode is the base tree's mode, which a deletion keeps (AGW-V0-042).
	oldMode string
}

// Options configure one report.
type Options struct {
	// Base is the full immutable commit object ID the range starts at.
	Base string
	// CEMPath overrides the in-repository Change Evidence Map location.
	CEMPath string
	// StableArtifacts is the absolute artifact root the stable verifier reads
	// for an exact cem/1.0 map. Without it a stable map is UNSUPPORTED and no
	// citation is applied (CEM-V1-071).
	StableArtifacts string
	// HeadPinned marks an index built by contextindex.BuildRevisionContext at
	// a commit that need not be checked out. Admission then reads only the Git
	// object store (contextindex.CommitPinnedPerPathRangeImpact) and the range
	// is reported COMMIT-PINNED (AGW-V0-007).
	HeadPinned bool
	// Evidence lists clean absolute paths of caller-supplied test-run
	// documents. Each bound one attaches non-closing CALLER_REPORTED witnesses;
	// an unreadable or undecodable one refuses the report (AGW-V0-008).
	Evidence []string
	// RepositoryEvidence lists caller-supplied test-run documents by
	// repository-relative path. Each is admitted only inside the checkout,
	// with symbolic links resolved and refused when they leave it
	// (AGW-V0-011); it then binds exactly as an Evidence document does.
	RepositoryEvidence []string
}

// Compile builds the report for base..index.CommitRevision. It only reads: no
// file is written and no worktree state is mutated.
func Compile(ctx context.Context, index *contextindex.Index, options Options) (*Report, error) {
	report, _, err := compile(ctx, index, options)
	return report, err
}

// compile is Compile that also returns each bound Playwright receipt, in
// option order, for paired-range citation (AGW-V0-014).
func compile(ctx context.Context, index *contextindex.Index, options Options) (*Report, []receiptBinding, error) {
	budget := gitrun.NewDefaultBudget()
	baseTree, err := resolveBaseTree(ctx, budget, index, options.Base)
	if err != nil {
		return nil, nil, err
	}
	changes, err := readChanges(ctx, budget, index, baseTree)
	if err != nil {
		return nil, nil, err
	}
	admission := admit(ctx, index, options.Base, options.HeadPinned)
	closure := reverseClosure(index, admission.paths)
	// A path whose facts the index dropped is not shallowly covered -- it is
	// not covered at all, and the receipt has to say which of the two it was.
	unparsed := index.UnparsedPaths()
	obligations := changeObligations(changes, admission, closure, unparsed)
	pathPolicy, rules := readPathPolicy(ctx, budget, index.Root, options.Base)
	readers := applyPathPolicy(obligations, rules, deletedFiles(changes), index, unparsed)
	impacts := impactObligations(closure, changes, unparsed)
	obligations = append(obligations, impacts...)
	obligations = append(obligations, newReaders(readers, impacts)...)
	source, document := readCEMSource(ctx, budget, index, options)
	source.Unattached = attachWitnesses(obligations, source, document)
	sources := []Source{source}
	if count := len(options.Evidence) + len(options.RepositoryEvidence); count > MaxEvidenceFiles {
		return nil, nil, fmt.Errorf("--evidence and --evidence-in-repo name %d documents; at most %d are read", count, MaxEvidenceFiles)
	}
	var receipts []receiptBinding
	add := func(evidence Source, tests []evidenceTest, bound *receiptBinding) {
		evidence.Basis, evidence.Unattached = attachEvidence(obligations, tests)
		sources = append(sources, evidence)
		if bound != nil {
			receipts = append(receipts, *bound)
		}
	}
	for _, file := range options.Evidence {
		evidence, tests, bound, err := readEvidence(ctx, budget, index, file)
		if err != nil {
			return nil, nil, err
		}
		add(evidence, tests, bound)
	}
	for _, name := range options.RepositoryEvidence {
		evidence, tests, bound, err := readRepositoryEvidence(ctx, budget, index, name)
		if err != nil {
			return nil, nil, err
		}
		add(evidence, tests, bound)
	}
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
		Authorities:    authorities,
		Sources:        sources,
		Obligations:    obligations,
		PacketCoverage: append(admission.packets, closure.packets...),
		PathPolicy:     pathPolicy,
	}
	if options.HeadPinned {
		report.Range.HeadBinding = "COMMIT-PINNED"
	}
	if source.Stable != nil {
		report.Profile = StableProfile
	}
	report.Summary = summarize(obligations)
	report.Preconditions = preconditions(report)
	return report, receipts, nil
}

// ResolveCommit resolves any revision Git accepts to the full commit object
// ID it names, read-only and under the bounded, graft-isolated Git runner
// (AGW-V0-005). An option-like revision, a revision that names no commit, and
// output that is not one full hex object ID refuse.
func ResolveCommit(ctx context.Context, root, revision string) (string, error) {
	if revision == "" || strings.HasPrefix(revision, "-") || strings.ContainsAny(revision, "\x00\n") {
		return "", fmt.Errorf("revision %q is not a Git revision", revision)
	}
	output, err := run(ctx, gitrun.NewDefaultBudget(), root, 128, "rev-parse", "--verify", "--quiet", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("revision %q does not resolve to a commit", revision)
	}
	commit := strings.TrimSuffix(string(output), "\n")
	if (len(commit) != 40 && len(commit) != 64) || strings.TrimLeft(commit, "0123456789abcdef") != "" {
		return "", fmt.Errorf("revision %q resolved to a malformed object ID", revision)
	}
	return commit, nil
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
		result = append(result, change{status: metadata[4], path: fields[position+1], newMode: metadata[1], oldMode: metadata[0][1:]})
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
	packets   []PacketCoverage
	refused   map[string]string
	truncated []string
	// bounded names the admitted paths the engine's ranked packet dropped at
	// its result ceiling (AGW-V0-038); each stays NOT_RUN under its own reason.
	bounded map[string]bool
}

// admit asks the committed-range engine which changed paths it will analyse.
// It is the sole authority on admission; this package adds no second one. A
// path the engine refuses on its own is named in refused with its rule, and
// the rest of the range is still analysed (AGW-V0-004).
func admit(ctx context.Context, index *contextindex.Index, base string, pinned bool) stage {
	engine := contextindex.PerPathRangeImpact
	if pinned {
		engine = contextindex.CommitPinnedPerPathRangeImpact
	}
	receipt, err := engine(ctx, index, base, impactLimit)
	if err != nil {
		return stage{status: "REFUSED", detail: err.Error(), packets: []PacketCoverage{}, refused: map[string]string{}}
	}
	refused := map[string]string{}
	named := make([]string, 0)
	if rows, ok := receipt["refusedPaths"].([]any); ok {
		for _, row := range rows {
			fields, _ := row.(map[string]any)
			refusedPath, rule := text(fields["path"]), text(fields["rule"])
			if refusedPath == "" || rule == "" {
				return stage{status: "REFUSED", detail: "the committed-range engine returned a malformed per-path refusal", packets: []PacketCoverage{}, refused: map[string]string{}}
			}
			refused[refusedPath] = rule
			if len(named) < maxNamedRefusals {
				named = append(named, refusedPath+" ("+rule+"): "+text(fields["detail"]))
			}
		}
	}
	admitted := make([]string, 0)
	for _, item := range results(receipt) {
		if text(item["kind"]) == "path" {
			admitted = append(admitted, text(item["id"]))
		}
	}
	sort.Strings(admitted)
	packets := []PacketCoverage{packetCoverage("admission", "", receipt)}
	result := stage{status: "COMPUTED", paths: admitted, receipts: []map[string]any{receipt}, packets: packets, refused: refused}
	details := make([]string, 0, 2)
	if len(refused) != 0 {
		detail := fmt.Sprintf("%d changed paths were refused per path and each is a NOT_RUN obligation: %s", len(refused), strings.Join(named, "; "))
		if len(refused) > len(named) {
			detail += fmt.Sprintf("; and %d more", len(refused)-len(named))
		}
		details = append(details, detail)
	}
	result.bounded = packetBounded(receipt, admitted)
	if len(result.bounded) != 0 {
		omitted := make([]string, 0, len(result.bounded))
		for item := range result.bounded {
			omitted = append(omitted, item)
		}
		sort.Strings(omitted)
		// Every omitted path is named: the path bound caps the list at 100.
		details = append(details, fmt.Sprintf("%d admitted paths fell outside the %d-result admission packet bound (ADMISSION_PACKET_BOUND) and each is a NOT_RUN obligation: %s",
			len(omitted), impactLimit, strings.Join(omitted, ", ")))
	}
	if len(details) != 0 {
		result.status = "PARTIAL"
		result.detail = strings.Join(details, "; ")
	}
	return result
}

// packetBounded names each changed path the engine admitted but ranked out of
// its result packet. The receipt's coverage.critical_missing lists exactly
// those selectors; a path is admitted when the engine neither refused it nor
// returned it, so reading the list keeps the packet ceiling from silently
// widening NOT_RUN under a COMPUTED admission (AGW-V0-038).
func packetBounded(receipt map[string]any, admitted []string) map[string]bool {
	coverageMap, _ := receipt["coverage"].(map[string]any)
	missing, _ := coverageMap["critical_missing"].([]any)
	included := map[string]bool{}
	for _, item := range admitted {
		included[item] = true
	}
	result := map[string]bool{}
	for _, selector := range missing {
		changedPath, ok := strings.CutPrefix(text(selector), "path:")
		if ok && changedPath != "" && !included[changedPath] {
			result[changedPath] = true
		}
	}
	return result
}

// maxNamedRefusals bounds how many per-path refusals the admission detail
// spells out; every refused path keeps its own obligation regardless.
const maxNamedRefusals = 8

// packetCoverage projects one compiled receipt's coverage block. The engine
// builds that block with native Go values; an absent block or a value of any
// other type is reported unreadable rather than read as zero.
func packetCoverage(stageName, changedPath string, receipt map[string]any) PacketCoverage {
	coverageMap, _ := receipt["coverage"].(map[string]any)
	packetBytes, bytesTyped := coverageMap["packet_bytes"].(int)
	within, withinTyped := coverageMap["within_budget"].(bool)
	included, includedTyped := coverageMap["included_results"].(int)
	omitted, omittedTyped := coverageMap["omitted_results"].(int)
	budget, budgetTyped := budgetOf(coverageMap["budget_bytes"])
	packet := PacketCoverage{Stage: stageName, Path: changedPath, Status: "NOT_PRODUCED", Reason: "packet-coverage-unreadable"}
	if !(bytesTyped && withinTyped && includedTyped && omittedTyped && budgetTyped) {
		return packet
	}
	packet.Status, packet.Reason = "PRODUCED", ""
	packet.PacketCounts = &PacketCounts{PacketBytes: packetBytes, BudgetBytes: budget,
		WithinBudget: within, IncludedResults: included, OmittedResults: omitted}
	return packet
}

// budgetOf reads budget_bytes, which is null for an unbounded packet.
func budgetOf(value any) (*int, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, true
	case int:
		return &typed, true
	}
	return nil, false
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
	result := stage{status: "COMPUTED", paths: admitted, packets: []PacketCoverage{}, refused: map[string]string{}, truncated: []string{}}
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
		result.packets = append(result.packets, packetCoverage("closure", changedPath, receipt))
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
		rule, refusedAlone := admission.refused[item.path]
		switch {
		case admitted[item.path]:
			if _, refused := closure.refused[item.path]; refused {
				obligation.Verdict, obligation.Reason = NotRun, "CLOSURE_REFUSED"
			}
		case admission.status == "REFUSED":
			obligation.Verdict, obligation.Reason = NotRun, "ADMISSION_REFUSED"
		case refusedAlone:
			obligation.Verdict, obligation.Reason = NotRun, "ADMISSION_REFUSED_"+rule
		case admission.bounded[item.path]:
			obligation.Verdict, obligation.Reason = NotRun, "ADMISSION_PACKET_BOUND"
		case tier != "DEEP":
			obligation.Verdict, obligation.Reason = NotRun, tierReason
		default:
			obligation.Verdict, obligation.Reason = NotRun, "ADMITTED_PATH_NOT_RANKED"
		}
		result = append(result, obligation)
	}
	return result
}

// newReaders drops each data-reader obligation whose path the reverse-dependency
// closure already opened, so one path is never two impact obligations.
func newReaders(readers, impacts []Obligation) []Obligation {
	opened := map[string]bool{}
	for _, item := range impacts {
		opened[item.Path] = true
	}
	result := make([]Obligation, 0, len(readers))
	for _, item := range readers {
		if !opened[item.Path] {
			result = append(result, item)
		}
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
	return witnesspaths.LanguageCoverage(item.path)
}

// readCEMSource loads the cem/1.0 Change Evidence Map from the head tree. The
// map is the repository's existing binding of claims to patch hunks; this
// package reuses it rather than inventing a parallel notion of evidence.
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
	if !wire.DeclaresStable(raw) {
		// A retired cem/0.x map is refused with its closed code and fix
		// line; its citations are never applied (CEM-V1-210).
		if _, err := wire.ParseStable(raw); err != nil {
			source.Status, source.Detail = "UNSUPPORTED", err.Error()
			if cemcode.CodeOf(err) != cemcode.UnsupportedSpec {
				source.Status = "INVALID"
			}
			return source, nil
		}
	}
	return readStableSource(ctx, source, raw, index, options)
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
		if obligations[position].Verdict == NotRun || obligations[position].Verdict == Exempt {
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
			if items[left].Source != items[right].Source {
				return items[left].Source < items[right].Source
			}
			if items[left].HunkID != items[right].HunkID {
				return items[left].HunkID < items[right].HunkID
			}
			if items[left].Relation != items[right].Relation {
				return items[left].Relation < items[right].Relation
			}
			if items[left].EvidencePath != items[right].EvidencePath {
				return items[left].EvidencePath < items[right].EvidencePath
			}
			return items[left].Test < items[right].Test
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
		case Exempt:
			result.Exempt++
		}
		if item.Verdict != Closed && item.Verdict != Exempt {
			if len(item.Witnesses) > 0 {
				result.CandidateWitnessed++
			} else {
				result.Unwitnessed++
			}
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
	if decidable := result.Analysed - result.Exempt; decidable > 0 {
		result.ProvenPerMille = result.Closed * 1000 / decidable
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
	if report.Range.Admission == "REFUSED" {
		result = append(result, Precondition{
			ID:        "WITNESS-P4-NO-BLAST-RADIUS",
			Statement: "the committed-range engine admitted no part of this range, so every obligation is NOT_RUN and no reverse-dependency surface was opened: " + report.Range.AdmissionDetail,
		})
	}
	bounded := 0
	for _, obligation := range report.Obligations {
		if obligation.Reason == "ADMISSION_PACKET_BOUND" {
			bounded++
		}
	}
	if report.Range.Admission == "PARTIAL" && bounded != 0 {
		result = append(result, Precondition{
			ID: "WITNESS-P8-ADMISSION-PACKET-BOUND",
			Statement: fmt.Sprintf("the committed-range engine's admission packet holds at most %d ranked results, so %d admitted changed paths beyond it are NOT_RUN with reason ADMISSION_PACKET_BOUND and no reverse-dependency surface was opened for them; split the range to analyse them: %s",
				impactLimit, bounded, report.Range.AdmissionDetail),
		})
	}
	if report.Range.Admission == "PARTIAL" && strings.Contains(report.Range.AdmissionDetail, "refused per path") {
		result = append(result, Precondition{
			ID:        "WITNESS-P7-PATHS-REFUSED-PER-PATH",
			Statement: "the committed-range engine refused some changed paths on their own, so each is NOT_RUN and no reverse-dependency surface was opened for it: " + report.Range.AdmissionDetail,
		})
	}
	if report.Range.Closure != "COMPUTED" && report.Range.Admission != "REFUSED" {
		result = append(result, Precondition{
			ID:        "WITNESS-P6-INCOMPLETE-REVERSE-DEPENDENCY-CLOSURE",
			Statement: "the reverse-dependency surface is not fully enumerated, so obligations this range may have broken without touching are missing from the denominator: " + report.Range.ClosureDetail,
		})
	}
	if report.PathPolicy != nil && report.PathPolicy.Status != "BOUND" {
		result = append(result, Precondition{
			ID:        "WITNESS-P9-PATH-POLICY-INVALID",
			Statement: "the base tree's declared path policy " + report.PathPolicy.Path + " is " + report.PathPolicy.Status + ", so no path was classified by it: " + report.PathPolicy.Detail,
		})
	}
	if report.Summary.NotRun > 0 && report.Range.Admission != "REFUSED" {
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
