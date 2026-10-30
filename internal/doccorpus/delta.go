package doccorpus

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

// Cross-revision delta worklist (DCX-V1-360..369): the previous corpus's
// provider records classified against the current revision's blobs, plus the
// changed source lines no claim covers, and a carry-forward provider record
// that feeds the confirmed records into the next build unchanged, keyed by
// span hash.
const (
	DeltaSchema = "corvint-corpus-delta/1"

	DeltaCarryForward = "carry-forward"
	DeltaMoved        = "moved"
	DeltaChanged      = "changed"
	DeltaRemoved      = "removed"
	DeltaUnresolved   = "unresolved"
	DeltaUncovered    = "uncovered"

	// DeltaDefaultLimit is the page size when the request names none.
	DeltaDefaultLimit = 64
)

// Delta read bounds (DCX-V1-367): per blob, admitted bytes in total, bytes
// hashed to place spans, changed-line hunks per anchor and subjects named per
// uncovered region.
const (
	deltaFileMaxBytes   = 8 << 20
	deltaReadMaxBytes   = 64 << 20
	deltaHashMaxBytes   = 64 << 20
	deltaHunksPerAnchor = 16
	deltaRegionSubjects = 8
	deltaLimitedPaths   = 16
)

// compilerReviewLimitation is the limitation checkEvidence appends when it
// demotes provider-reported review; carry-forward removes it again so the next
// build re-adds it byte for byte.
const compilerReviewLimitation = "provider-reported review is unauthenticated and grants no Core authority"

var deltaClassOrder = map[string]int{DeltaRemoved: 0, DeltaChanged: 1, DeltaUnresolved: 2, DeltaUncovered: 3, DeltaMoved: 4, DeltaCarryForward: 5}

// Regions occur only among uncovered entries, where changed lines lead the subjects no claim cites.
var deltaKindOrder = map[string]int{"region": 0, "subject": 1, "claim": 2, "relation": 3}

// DeltaSpan is one side of an anchored span or an uncovered region.
type DeltaSpan struct {
	Path       string `json:"path"`
	Blob       string `json:"blob,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	Start      int    `json:"start_line"`
	End        int    `json:"end_line"`
	SpanSHA256 string `json:"span_sha256,omitempty"`
	Estimated  bool   `json:"estimated,omitempty"`
}

// DeltaAnchor is one previous anchor re-validated at the current revision.
type DeltaAnchor struct {
	Anchor string `json:"anchor"`
	// Repository names a declared additional repository the anchor belongs to (DCX-V1-420);
	// it is omitted for the primary repository, whose receipts keep their bytes.
	Repository       string      `json:"repository,omitempty"`
	State            string      `json:"state"`
	Reason           string      `json:"reason"`
	Previous         DeltaSpan   `json:"previous"`
	Current          *DeltaSpan  `json:"current,omitempty"`
	ChangedLines     []DeltaHunk `json:"changed_lines,omitempty"`
	ChangedTruncated bool        `json:"changed_lines_truncated,omitempty"`
}

// DeltaEntry is one worklist item: a previous provider record, or a changed
// source region of the current revision that no claim covers.
type DeltaEntry struct {
	Class            string        `json:"class"`
	RecordKind       string        `json:"record_kind"`
	ID               string        `json:"id"`
	Provider         string        `json:"provider,omitempty"`
	Reason           string        `json:"reason"`
	PresentInCurrent bool          `json:"present_in_current,omitempty"`
	Anchors          []DeltaAnchor `json:"anchors,omitempty"`
	Region           *DeltaSpan    `json:"region,omitempty"`
	Subjects         []string      `json:"subjects,omitempty"`
	SubjectsOmitted  int           `json:"subjects_omitted,omitempty"`
}

// DeltaSide names one compared corpus.
type DeltaSide struct {
	Source         string     `json:"source"`
	SHA256         string     `json:"sha256"`
	ManifestSHA256 string     `json:"manifest_sha256"`
	Repository     Repository `json:"repository"`
	Tree           string     `json:"tree"`
	Builder        Builder    `json:"builder"`
}

// DeltaReceipt is the paged `docs corpus delta` worklist (DCX-V1-360).
type DeltaReceipt struct {
	Schema      string         `json:"schema"`
	Previous    DeltaSide      `json:"previous"`
	Current     DeltaSide      `json:"current"`
	Counts      map[string]int `json:"counts"`
	Total       int            `json:"total"`
	Offset      int            `json:"offset"`
	Limit       int            `json:"limit"`
	NextOffset  int            `json:"next_offset,omitempty"`
	Entries     []DeltaEntry   `json:"entries"`
	Limitations []string       `json:"limitations"`
	// Anchorless summarizes the records with no anchor by provider and record kind instead of
	// listing each as an unresolved entry (DCX-V1-422).
	Anchorless []DeltaAnchorless `json:"anchorless,omitempty"`
	// UncoveredSubjectsOmitted counts the subject-level uncovered entries that the `changed`
	// uncovered mode leaves out of the worklist (DCX-V1-421).
	UncoveredSubjectsOmitted int `json:"uncovered_subjects_omitted,omitempty"`
}

// DeltaAnchorless is one provider and record kind's count of previous records that have no
// anchor, so no span hash can be confirmed (DCX-V1-422).
type DeltaAnchorless struct {
	Provider   string `json:"provider"`
	RecordKind string `json:"record_kind"`
	Count      int    `json:"count"`
	Reason     string `json:"reason"`
}

// DeltaStreamSchema is the header line of the whole-worklist `--format jsonl` stream
// (DCX-V1-423); every later line is one DeltaEntry.
const DeltaStreamSchema = "corvint-corpus-delta-stream/1"

// DeltaStreamHeader is the first line of the jsonl stream: the receipt without paging.
type DeltaStreamHeader struct {
	Schema                   string            `json:"schema"`
	Previous                 DeltaSide         `json:"previous"`
	Current                  DeltaSide         `json:"current"`
	Counts                   map[string]int    `json:"counts"`
	Total                    int               `json:"total"`
	Limitations              []string          `json:"limitations"`
	Anchorless               []DeltaAnchorless `json:"anchorless,omitempty"`
	UncoveredSubjectsOmitted int               `json:"uncovered_subjects_omitted,omitempty"`
}

// Delta worklist modes (DCX-V1-421, DCX-V1-422).
const (
	DeltaUncoveredAll      = "all"
	DeltaUncoveredChanged  = "changed"
	DeltaAnchorlessSummary = "summary"
	DeltaAnchorlessList    = "list"

	deltaAnchorlessReason = "record has no anchored span whose hash could be confirmed"
)

// DeltaOptions selects the worklist view. Empty modes take their defaults: uncovered `all`
// and anchorless `summary`.
type DeltaOptions struct {
	Offset, Limit int
	Uncovered     string
	Anchorless    string
	// Timings, when set, receives the comparison phase and, for the stream, the encode phase
	// (DCR-V1-462). It never changes the result.
	Timings *ReadTimings
}

// deltaRepo is one repository whose anchors the delta re-validates: the primary, or a declared
// additional repository read from its own checkout at one of its pinned revisions (DCX-V1-420,
// DCX-V1-424).
type deltaRepo struct {
	id, revision, dir string
	primary           bool
	auth              *gitauth.Repository
	pre               *pathPrefetch
	inputs            map[string]Input
	index             *contextindex.Index
	unavailable       string
	// pins is, for a declared additional repository, one view per pinned revision in the
	// manifest's order: its current revision first, then its further pinned revisions.
	pins     map[string]*deltaRepo
	pinOrder []string
}

// deltaRecord is one classified previous record with its re-pinned evidence.
type deltaRecord struct {
	entry    DeltaEntry
	kind     string
	subject  Subject
	claim    Claim
	relation Relation
	anchors  []Anchor
	// details are the record's typed adoption details in the previous corpus, if any; a carried
	// record re-applies them unchanged (DCX-V1-443).
	details *RecordDetails
	// adoption reports an adoption-only subject kind or relation type (DCX-V1-440).
	adoption bool
}

func (r *deltaRecord) carried() bool {
	return r.entry.Class == DeltaCarryForward || r.entry.Class == DeltaMoved
}

type deltaRun struct {
	root         string
	previous     *Artifact
	current      *Artifact
	auth         *gitauth.Repository
	pre          *pathPrefetch
	newInputs    map[string]Input
	oldInputs    map[string]Input
	blobs        map[string][]byte
	blobReasons  map[string]string
	read, hashed int
	// verified caches span digests by deltaSpanKey so a repeated span is
	// charged to the hashing bound once (DCX-V1-444).
	verified      map[deltaSpanKey]string
	hashLimit     int // deltaHashMaxBytes; zero means the default
	readLimit     int // deltaReadMaxBytes; zero means the default
	index         *contextindex.Index
	resolver      *SymbolResolver
	records       []*deltaRecord
	regions       []DeltaEntry
	limitations   []string
	skippedNative int
	// extra holds the declared additional repositories previous anchors name (DCX-V1-420).
	extra map[string]*deltaRepo
}

// Delta classifies previous against current (DCX-V1-360..365) and returns one
// page of the worklist. current is the opened or freshly built current corpus;
// source is "artifact" or "manifest". It reads Git objects only.
func Delta(ctx context.Context, root string, previous, current *Artifact, source string, offset, limit int) (*DeltaReceipt, error) {
	return DeltaWith(ctx, root, previous, current, source, DeltaOptions{Offset: offset, Limit: limit})
}

// DeltaWith is Delta with the uncovered and anchorless modes (DCX-V1-421, DCX-V1-422).
func DeltaWith(ctx context.Context, root string, previous, current *Artifact, source string, o DeltaOptions) (*DeltaReceipt, error) {
	if o.Limit == 0 {
		o.Limit = DeltaDefaultLimit
	}
	if o.Limit < 1 || o.Limit > MaxResults || o.Offset < 0 {
		return nil, fail("delta limit must be 1..256 and offset non-negative")
	}
	w, err := timedDeltaWorklist(ctx, root, previous, current, o)
	if err != nil {
		return nil, err
	}
	receipt := &DeltaReceipt{Schema: DeltaSchema, Previous: deltaSide("artifact", previous), Current: deltaSide(source, current), Counts: w.counts, Total: len(w.entries), Offset: o.Offset, Limit: o.Limit, Entries: []DeltaEntry{}, Limitations: w.limitations, Anchorless: w.anchorless, UncoveredSubjectsOmitted: w.subjectsOmitted}
	if o.Offset < len(w.entries) {
		end := min(len(w.entries), o.Offset+o.Limit)
		receipt.Entries = w.entries[o.Offset:end]
		if end < len(w.entries) {
			receipt.NextOffset = end
		}
	}
	return receipt, nil
}

// DeltaStream classifies once and returns the whole worklist as JSON lines: one
// DeltaStreamHeader line, then every entry in worklist order (DCX-V1-423). Paging options are
// ignored. The stream is bounded by the 128 MiB corpus byte bound.
func DeltaStream(ctx context.Context, root string, previous, current *Artifact, source string, o DeltaOptions) ([]byte, error) {
	w, err := timedDeltaWorklist(ctx, root, previous, current, o)
	if err != nil {
		return nil, err
	}
	if o.Timings != nil {
		defer func(start time.Time) { o.Timings.Encode += time.Since(start) }(time.Now())
	}
	header, err := Encode(DeltaStreamHeader{Schema: DeltaStreamSchema, Previous: deltaSide("artifact", previous), Current: deltaSide(source, current), Counts: w.counts, Total: len(w.entries), Limitations: w.limitations, Anchorless: w.anchorless, UncoveredSubjectsOmitted: w.subjectsOmitted})
	if err != nil {
		return nil, err
	}
	out := header
	for _, e := range w.entries {
		line, err := Encode(e)
		if err != nil {
			return nil, err
		}
		if len(out)+len(line) > MaxCorpusBytes {
			return nil, capacityExceeded("delta stream bytes", len(out)+len(line), MaxCorpusBytes)
		}
		out = append(out, line...)
	}
	return out, nil
}

type deltaList struct {
	entries         []DeltaEntry
	counts          map[string]int
	limitations     []string
	anchorless      []DeltaAnchorless
	subjectsOmitted int
}

// timedDeltaWorklist is deltaWorklist, its duration added to o.Timings.Compare when set.
func timedDeltaWorklist(ctx context.Context, root string, previous, current *Artifact, o DeltaOptions) (*deltaList, error) {
	if o.Timings != nil {
		defer func(start time.Time) { o.Timings.Compare += time.Since(start) }(time.Now())
	}
	return deltaWorklist(ctx, root, previous, current, o)
}

// deltaWorklist classifies once and orders the entries of the selected view.
func deltaWorklist(ctx context.Context, root string, previous, current *Artifact, o DeltaOptions) (*deltaList, error) {
	switch o.Uncovered {
	case "":
		o.Uncovered = DeltaUncoveredAll
	case DeltaUncoveredAll, DeltaUncoveredChanged:
	default:
		return nil, fail("delta uncovered mode must be changed or all")
	}
	switch o.Anchorless {
	case "":
		o.Anchorless = DeltaAnchorlessSummary
	case DeltaAnchorlessSummary, DeltaAnchorlessList:
	default:
		return nil, fail("delta anchorless mode must be summary or list")
	}
	run, err := classifyDelta(ctx, root, previous, current)
	if err != nil {
		return nil, err
	}
	w := &deltaList{entries: []DeltaEntry{}, limitations: run.limitations}
	anchorless := map[[2]string]int{}
	for _, r := range run.records {
		if o.Anchorless == DeltaAnchorlessSummary && len(r.entry.Anchors) == 0 && r.entry.Class == DeltaUnresolved && r.entry.Reason == deltaAnchorlessReason {
			anchorless[[2]string{r.entry.Provider, r.kind}]++
			continue
		}
		w.entries = append(w.entries, r.entry)
	}
	for _, e := range run.regions {
		if o.Uncovered == DeltaUncoveredChanged && e.RecordKind == "subject" {
			w.subjectsOmitted++
			continue
		}
		w.entries = append(w.entries, e)
	}
	keys := make([][2]string, 0, len(anchorless))
	for k := range anchorless {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return deltaKindOrder[keys[i][1]] < deltaKindOrder[keys[j][1]]
	})
	for _, k := range keys {
		w.anchorless = append(w.anchorless, DeltaAnchorless{Provider: k[0], RecordKind: k[1], Count: anchorless[k], Reason: deltaAnchorlessReason})
	}
	sort.SliceStable(w.entries, func(i, j int) bool {
		a, b := w.entries[i], w.entries[j]
		if deltaClassOrder[a.Class] != deltaClassOrder[b.Class] {
			return deltaClassOrder[a.Class] < deltaClassOrder[b.Class]
		}
		if deltaKindOrder[a.RecordKind] != deltaKindOrder[b.RecordKind] {
			return deltaKindOrder[a.RecordKind] < deltaKindOrder[b.RecordKind]
		}
		return a.ID < b.ID
	})
	w.counts = map[string]int{DeltaCarryForward: 0, DeltaMoved: 0, DeltaChanged: 0, DeltaRemoved: 0, DeltaUnresolved: 0, DeltaUncovered: 0}
	for _, e := range w.entries {
		w.counts[e.Class]++
	}
	return w, nil
}

// CarryForward returns a corvint-corpus-provider/1 record holding provider's
// previous records that the delta classes carry-forward or moved and that the
// current corpus does not already hold, each anchor re-pinned to the current
// revision with its span hash unchanged (DCX-V1-366). The record is meant to be
// committed and declared as a records provider of the next build. A record that
// carries adoption kinds or typed details is a corvint-corpus-adoption-provider/1
// record holding those details unchanged (DCX-V1-443).
func CarryForward(ctx context.Context, root string, previous, current *Artifact, provider string) (*ProviderRecord, error) {
	declared, ok := Provider{}, false
	for _, p := range previous.Manifest.Providers {
		if p.ID == provider && p.Kind == "records" {
			declared, ok = p, true
		}
	}
	if !ok {
		return nil, fail("carry-forward provider is not a records provider of the previous corpus")
	}
	run, err := classifyDelta(ctx, root, previous, current)
	if err != nil {
		return nil, err
	}
	record := &ProviderRecord{Schema: ProviderSchema, ID: provider, Version: declared.Version, Source: current.Manifest.Repository, Subjects: []Subject{}, Claims: []Claim{}, Relations: []Relation{}, Journeys: []Journey{}, Observations: []ObservationLink{}, Capabilities: []CapabilityDeclaration{}}
	// An adoption provider's records go back as the adoption record they came from, under its
	// record bound; every adoption import leaves a typed_detail parity row (DCX-V1-443).
	adoption, details := slices.ContainsFunc(previous.ImportParity, func(p ImportParity) bool {
		return p.Provider == provider && p.SourceKind == "typed_detail"
	}), map[string]RecordDetails{}
	for _, r := range run.records {
		if r.entry.Provider != provider || r.entry.PresentInCurrent || !r.carried() {
			continue
		}
		adoption = adoption || r.adoption
		if r.details != nil {
			details[r.entry.ID] = *r.details
		}
		switch r.kind {
		case "subject":
			s := r.subject
			s.Evidence = carriedEvidence(s.Evidence, r.anchors)
			record.Subjects = append(record.Subjects, s)
		case "claim":
			c := r.claim
			c.Evidence = carriedEvidence(c.Evidence, r.anchors)
			record.Claims = append(record.Claims, c)
		case "relation":
			rel := r.relation
			rel.Evidence = carriedEvidence(rel.Evidence, r.anchors)
			record.Relations = append(record.Relations, rel)
		}
	}
	// Adoption kinds and typed details re-apply only through the adoption schema, which only a /2
	// manifest admits (DCX-V1-443).
	limit := MaxRecords
	if adoption || len(details) > 0 {
		if current.Manifest.Schema != ManifestSchemaV2 {
			return nil, fail("carry-forward of adoption provider " + provider + " emits a " + AdoptionProviderSchema + " record, which only a " + ManifestSchemaV2 + " current manifest admits")
		}
		record.Schema, record.Details, limit = AdoptionProviderSchema, details, MaxCorpusRecords
	}
	if err := carriedClosure(run, record, provider); err != nil {
		return nil, err
	}
	if len(record.Subjects) > limit || len(record.Claims) > limit || len(record.Relations) > limit {
		return nil, capacityExceeded("carried records of one kind", max(len(record.Subjects), len(record.Claims), len(record.Relations)), limit)
	}
	reason := "carried forward from corpus " + previous.SHA256 + " with every span hash confirmed at the current revision"
	for _, kind := range []struct {
		name  string
		count int
	}{{"subjects", len(record.Subjects)}, {"claims", len(record.Claims)}, {"relations", len(record.Relations)}} {
		if kind.count > 0 {
			record.Capabilities = append(record.Capabilities, CapabilityDeclaration{Name: kind.name, State: "present", Reason: reason})
		}
	}
	if _, err := Encode(record); err != nil {
		return nil, err
	}
	return record, nil
}

// carriedClosure refuses a carry-forward record that would not import on its
// own: a carried claim's subject, or a carried relation's endpoint, that is
// neither in the current corpus nor in the record. Classification already
// demoted records whose dependency is carried by no provider (DCX-V1-365), so
// what remains is a dependency another provider carries; the refusal names it
// so the caller can carry and build that provider first (DCX-V1-366).
func carriedClosure(run *deltaRun, record *ProviderRecord, provider string) error {
	present := map[string]bool{}
	for _, s := range run.current.Subjects {
		present[s.ID] = true
	}
	for _, c := range run.current.Claims {
		present[c.ID] = true
	}
	for _, s := range record.Subjects {
		present[s.ID] = true
	}
	for _, c := range record.Claims {
		present[c.ID] = true
	}
	carriedBy := map[string]string{}
	for _, r := range run.records {
		if r.carried() {
			carriedBy[r.entry.ID] = r.entry.Provider
		}
	}
	missing := []string{}
	need := func(id, dep string) {
		if !present[dep] {
			missing = append(missing, id+" needs "+dep+" (carried by provider "+carriedBy[dep]+")")
		}
	}
	for _, c := range record.Claims {
		need(c.ID, c.Subject)
	}
	for _, r := range record.Relations {
		need(r.ID, r.From)
		need(r.ID, r.To)
	}
	// A typed detail may also join to a relation (DCX-V1-442).
	for _, r := range run.current.Relations {
		present[r.ID] = true
	}
	for _, r := range record.Relations {
		present[r.ID] = true
	}
	for _, id := range sortedKeys(record.Details) {
		for _, ref := range detailReferences(record.Details[id]) {
			need(id, ref)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	shown := missing[:min(len(missing), deltaLimitedPaths)]
	return fail(fmt.Sprintf("carry-forward of provider %s depends on %d records outside it and the current corpus: %s; carry, declare and build those providers first, then carry %s against that build", provider, len(missing), strings.Join(shown, "; "), provider))
}

// carriedEvidence re-pins anchors and undoes the compiler's review demotion so
// the next build's import reproduces the previous evidence exactly.
func carriedEvidence(e Evidence, anchors []Anchor) Evidence {
	e.Anchors = slices.Clone(anchors)
	e.Limitations = slices.Clone(e.Limitations)
	if e.ReportedTrust == "reviewed" {
		e.Trust, e.ReportedTrust = "reviewed", ""
		if n := len(e.Limitations); n > 0 && e.Limitations[n-1] == compilerReviewLimitation {
			e.Limitations = e.Limitations[:n-1]
		}
	}
	return e
}

func deltaSide(source string, a *Artifact) DeltaSide {
	return DeltaSide{Source: source, SHA256: a.SHA256, ManifestSHA256: a.ManifestSHA256, Repository: a.Manifest.Repository, Tree: a.Tree, Builder: a.Builder}
}

func classifyDelta(ctx context.Context, root string, previous, current *Artifact) (*deltaRun, error) {
	return classifyDeltaWithin(ctx, root, previous, current, deltaReadMaxBytes)
}

// classifyDeltaWithin is classifyDelta under readLimit bytes read from every repository's object
// store together, prefetch included (DCX-V1-367, DCX-V1-420).
func classifyDeltaWithin(ctx context.Context, root string, previous, current *Artifact, readLimit int) (_ *deltaRun, err error) {
	defer func() { err = gitCapacity(err, "while classifying the corpus delta") }()
	if previous.Manifest.Repository.ID != current.Manifest.Repository.ID {
		return nil, fail("previous and current corpora name different repositories")
	}
	ctx = gitstatus.WithIsolation(ctx)
	auth, err := gitauth.Open(root, newGitBudget(ctx))
	if err != nil {
		return nil, err
	}
	release := auth.BeginObjectSession()
	defer release()
	d := &deltaRun{root: root, previous: previous, current: current, auth: auth, pre: newPathPrefetch(current.Manifest.Repository.Revision), hashLimit: deltaHashMaxBytes, readLimit: readLimit, newInputs: map[string]Input{}, oldInputs: map[string]Input{}, blobs: map[string][]byte{}, blobReasons: map[string]string{}}
	for _, in := range current.Manifest.Inputs {
		if in.Revision == current.Manifest.Repository.Revision {
			d.newInputs[in.Path] = in
		}
	}
	for _, in := range previous.Manifest.Inputs {
		if in.Revision == previous.Manifest.Repository.Revision {
			d.oldInputs[in.Path] = in
		}
	}
	releaseExtra := d.openAdditional(ctx)
	defer releaseExtra()
	if err := d.prefetch(ctx); err != nil {
		return nil, err
	}
	if err := d.classifyRecords(ctx); err != nil {
		return nil, err
	}
	if err := d.uncovered(ctx); err != nil {
		return nil, err
	}
	d.limitations = append(d.limitations, current.deltaUnverified...)
	d.limitations = append(d.limitations,
		"journeys, observations and supplied gaps of the previous corpus are not classified",
		"uncovered regions cover the primary repository's changed source inputs only; additional repository anchors are re-validated but their changed lines are not listed",
		"symbol re-resolution across paths covers the languages the symbol resolver normalizes; other symbol anchors are placed by span hash in their own path only")
	if d.skippedNative > 0 {
		d.limitations = append(d.limitations, fmt.Sprintf("%d native or compiler-derived previous records are rebuilt by the compiler and not classified", d.skippedNative))
	}
	return d, nil
}

// openAdditional opens, read-only, the checkout of every declared additional repository that a
// previous anchor names, exactly as the build reads it (DCP-V1-176, DCX-V1-420): the current
// manifest must declare the repository, its checkout must open, and its root commit must equal
// the declared identity. A repository that fails any of these keeps the reason, and its anchors
// are unresolved with it. The returned function releases every object session.
func (d *deltaRun) openAdditional(ctx context.Context) func() {
	d.extra = map[string]*deltaRepo{}
	releases := []func(){}
	named := map[string]bool{}
	for _, list := range [][]Evidence{subjectEvidence(d.previous.Subjects), claimEvidence(d.previous.Claims), relationEvidence(d.previous.Relations)} {
		for _, e := range list {
			for _, a := range e.Anchors {
				if a.Repository != d.previous.Manifest.Repository.ID {
					named[a.Repository] = true
				}
			}
		}
	}
	for _, id := range sortedKeys(named) {
		repo := &deltaRepo{id: id, inputs: map[string]Input{}}
		d.extra[id] = repo
		var declared *AdditionalRepository
		for i, r := range d.current.Manifest.AdditionalRepositories {
			if r.ID == id {
				declared = &d.current.Manifest.AdditionalRepositories[i]
			}
		}
		if declared == nil {
			repo.unavailable = "anchor names repository " + id + ", which the current manifest does not declare"
			continue
		}
		repo.revision = declared.Revision
		dir := d.current.Manifest.RevisionRoot(d.root, declared.Revision)
		if got, err := contextindex.CorpusRepositoryID(ctx, dir, declared.Revision); err != nil || got != id {
			repo.unavailable = "additional repository " + id + " checkout is unavailable or does not hold its declared identity and revision"
			continue
		}
		auth, err := gitauth.Open(dir, newGitBudget(ctx))
		if err != nil {
			repo.unavailable = checkoutUnavailable("additional repository checkout unavailable: "+id, err).Message
			continue
		}
		releases = append(releases, auth.BeginObjectSession())
		repo.auth, repo.dir, repo.pins, repo.pinOrder = auth, dir, map[string]*deltaRepo{}, declared.pinned()
		// Each pinned revision keeps its own inputs, so an anchor is resolved at the revision it
		// was pinned to while the current manifest still pins it (DCX-V1-424).
		for _, revision := range repo.pinOrder {
			pin := &deltaRepo{id: id, revision: revision, dir: dir, auth: auth, pre: newPathPrefetch(revision), inputs: map[string]Input{}}
			if revision != declared.Revision {
				if got, err := contextindex.CorpusRepositoryID(ctx, dir, revision); err != nil || got != id {
					pin.unavailable = "additional repository " + id + " checkout does not hold its pinned revision " + revision
				}
			}
			for _, in := range d.current.Manifest.Inputs {
				if in.Revision == revision {
					pin.inputs[in.Path] = in
				}
			}
			repo.pins[revision] = pin
		}
	}
	return func() {
		for _, release := range releases {
			release()
		}
	}
}

// repoOf is the repository a previous anchor belongs to: the primary, or a declared additional
// repository opened by openAdditional, at the anchor's own revision while the current manifest
// still pins it and otherwise at the repository's current revision (DCX-V1-424).
func (d *deltaRun) repoOf(a Anchor) *deltaRepo {
	if a.Repository == d.previous.Manifest.Repository.ID {
		return &deltaRepo{id: a.Repository, revision: d.current.Manifest.Repository.Revision, primary: true, auth: d.auth, pre: d.pre, inputs: d.newInputs}
	}
	if repo, ok := d.extra[a.Repository]; ok {
		if repo.unavailable != "" {
			return repo
		}
		if pin, ok := repo.pins[a.Revision]; ok {
			return pin
		}
		return repo.pins[repo.revision]
	}
	return &deltaRepo{id: a.Repository, unavailable: "anchor names repository " + a.Repository + ", which the current manifest does not declare"}
}

// prefetch batches the tree lookups and blob reads the classification needs,
// so the Git operation count does not grow with the record count. Every batch draws on the one
// delta read bound shared by all repositories, so prefetch never reads past it (DCX-V1-367,
// DCX-V1-420).
func (d *deltaRun) prefetch(ctx context.Context) error {
	oids, paths := []string{}, []string{}
	extraOIDs, extraPaths := map[*deltaRepo][]string{}, map[*deltaRepo][]string{}
	for _, list := range [][]Evidence{subjectEvidence(d.previous.Subjects), claimEvidence(d.previous.Claims), relationEvidence(d.previous.Relations)} {
		for _, e := range list {
			for _, a := range e.Anchors {
				if a.Repository != d.previous.Manifest.Repository.ID {
					if repo := d.repoOf(a); repo.unavailable == "" {
						extraOIDs[repo] = append(extraOIDs[repo], a.Blob)
						if in, ok := repo.inputs[a.Path]; ok {
							extraOIDs[repo] = append(extraOIDs[repo], in.Blob)
						} else {
							extraPaths[repo] = append(extraPaths[repo], a.Path)
						}
					}
					continue
				}
				oids = append(oids, a.Blob)
				if in, ok := d.newInputs[a.Path]; ok {
					oids = append(oids, in.Blob)
				} else {
					paths = append(paths, a.Path)
				}
			}
		}
	}
	for _, p := range sortedKeys(d.newInputs) {
		in := d.newInputs[p]
		if in.Purpose != "source" {
			continue
		}
		if old, ok := d.oldInputs[p]; !ok || old.Blob != in.Blob {
			oids = append(oids, in.Blob)
			if ok {
				oids = append(oids, old.Blob)
			}
		}
	}
	if err := d.pre.resolve(ctx, d.auth, paths); err != nil {
		return err
	}
	if err := d.prefetchBodies(ctx, d.pre, d.auth, oids); err != nil {
		return err
	}
	for _, id := range sortedKeys(d.extra) {
		repo := d.extra[id]
		if repo.unavailable != "" {
			continue
		}
		for _, revision := range repo.pinOrder {
			pin := repo.pins[revision]
			if pin.unavailable != "" {
				continue
			}
			if err := pin.pre.resolve(ctx, pin.auth, extraPaths[pin]); err != nil {
				return err
			}
			if err := d.prefetchBodies(ctx, pin.pre, pin.auth, extraOIDs[pin]); err != nil {
				return err
			}
		}
	}
	return nil
}

// prefetchBodies batch-reads oids within what is left of the shared read bound and charges the
// bodies read to it; blobIn serves them without charging again.
func (d *deltaRun) prefetchBodies(ctx context.Context, pre *pathPrefetch, auth *gitauth.Repository, oids []string) error {
	left := d.readBudget() - d.read
	if left < 1 || len(oids) == 0 {
		return nil
	}
	before := pre.heldBytes()
	if err := pre.read(ctx, auth, oids, deltaFileMaxBytes, left); err != nil {
		return err
	}
	d.read += pre.heldBytes() - before
	return nil
}

func (d *deltaRun) readBudget() int {
	if d.readLimit == 0 {
		return deltaReadMaxBytes
	}
	return d.readLimit
}

func subjectEvidence(list []Subject) []Evidence {
	out := make([]Evidence, len(list))
	for i := range list {
		out[i] = list[i].Evidence
	}
	return out
}
func claimEvidence(list []Claim) []Evidence {
	out := make([]Evidence, len(list))
	for i := range list {
		out[i] = list[i].Evidence
	}
	return out
}
func relationEvidence(list []Relation) []Evidence {
	out := make([]Evidence, len(list))
	for i := range list {
		out[i] = list[i].Evidence
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// blob returns a bounded, digest-verified blob body, or the reason it is
// unavailable. Only a cancelled context or a Git capacity failure is an error.
func (d *deltaRun) blob(ctx context.Context, oid, sha string) ([]byte, string, error) {
	return d.blobIn(ctx, &deltaRepo{primary: true, auth: d.auth, pre: d.pre}, oid, sha)
}

// blobIn is blob read from one repository's object store. Every repository shares the read
// bound, which prefetched bodies were already charged to; an additional repository's bodies and
// reasons are cached under its identity.
func (d *deltaRun) blobIn(ctx context.Context, repo *deltaRepo, oid, sha string) ([]byte, string, error) {
	key := oid
	if !repo.primary {
		key = repo.id + "\x00" + oid
	}
	if data, ok := d.blobs[key]; ok {
		return data, "", nil
	}
	if reason, ok := d.blobReasons[key]; ok {
		return nil, reason, nil
	}
	limit := min(deltaFileMaxBytes, d.readBudget()-d.read)
	data, held := repo.pre.held(oid)
	if !held && limit < 1 {
		reason := "the 64 MiB delta read bound was reached before blob " + oid
		d.blobReasons[key] = reason
		return nil, reason, nil
	}
	var over bool
	var err error
	if !held {
		// The limit is checked against the object header before any body is read.
		data, over, err = repo.pre.blobWithin(ctx, repo.auth, oid, limit)
	}
	reason := ""
	switch {
	case err != nil:
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		var coded *Error
		if typed := gitCapacity(err, ""); errors.As(typed, &coded) {
			return nil, "", typed
		}
		reason = "blob " + oid + " is not readable from this repository"
	case over && limit == deltaFileMaxBytes:
		reason = "blob " + oid + " exceeds the 8 MiB per-file delta bound"
	case over:
		reason = fmt.Sprintf("blob %s exceeds the %d bytes left under the 64 MiB delta read bound", oid, limit)
	case sha != "" && Digest(data) != sha:
		reason = "blob " + oid + " does not hash to its recorded sha256"
	}
	if reason != "" {
		d.blobReasons[key] = reason
		return nil, reason, nil
	}
	if !held {
		d.read += len(data)
	}
	d.blobs[key] = data
	return data, "", nil
}

func (d *deltaRun) classifyRecords(ctx context.Context) error {
	recordsProviders := map[string]bool{}
	for _, p := range d.previous.Manifest.Providers {
		if p.Kind == "records" {
			recordsProviders[p.ID] = true
		}
	}
	present := map[string]bool{}
	for _, s := range d.current.Subjects {
		present[s.ID] = true
	}
	for _, c := range d.current.Claims {
		present[c.ID] = true
	}
	for _, r := range d.current.Relations {
		present[r.ID] = true
	}
	// Adoption-only kinds are classified like generic records (DCX-V1-440); one whose kind needs
	// typed details the previous corpus does not hold stays unresolved (DCX-V1-441).
	add := func(kind, id, provider string, e Evidence, known bool, typed string, needsDetails bool, r *deltaRecord) error {
		if !recordsProviders[provider] {
			d.skippedNative++
			return nil
		}
		r.kind, r.adoption = kind, typed != ""
		r.entry = DeltaEntry{RecordKind: kind, ID: id, Provider: provider, PresentInCurrent: present[id]}
		if details, ok := d.previous.Details[id]; ok {
			r.details = &details
		}
		if err := d.classifyEvidence(ctx, r, e); err != nil {
			return err
		}
		switch {
		case !known && r.carried():
			r.entry.Class, r.entry.Reason = DeltaUnresolved, "record kind is in neither the generic nor the adoption vocabulary"
		case needsDetails && r.details == nil && r.carried():
			r.entry.Class, r.entry.Reason = DeltaUnresolved, "adoption record kind "+typed+" requires typed details that the previous corpus does not hold"
		}
		d.records = append(d.records, r)
		return nil
	}
	adoptionKind := func(generic, adoption map[string]bool, kind string) string {
		if !generic[kind] && adoption[kind] {
			return kind
		}
		return ""
	}
	for _, s := range d.previous.Subjects {
		if err := add("subject", s.ID, s.Provider, s.Evidence, subjectKinds[s.Kind] || adoptionSubjectKinds[s.Kind], adoptionKind(subjectKinds, adoptionSubjectKinds, s.Kind), typedSubjectNeedsDetails(s.Kind), &deltaRecord{subject: s}); err != nil {
			return err
		}
	}
	for _, c := range d.previous.Claims {
		if err := add("claim", c.ID, c.Provider, c.Evidence, true, "", false, &deltaRecord{claim: c}); err != nil {
			return err
		}
	}
	for _, r := range d.previous.Relations {
		if err := add("relation", r.ID, r.Provider, r.Evidence, relationKinds[r.Type] || adoptionRelationKinds[r.Type], adoptionKind(relationKinds, adoptionRelationKinds, r.Type), typedRelationNeedsDetails(r.Type), &deltaRecord{relation: r}); err != nil {
			return err
		}
	}
	d.demoteDependencies(present)
	return nil
}

// deltaDependency is one ID a carried record needs in the current corpus or carried with it,
// carried under one of kinds (any record kind when kinds is nil).
type deltaDependency struct {
	id, what string
	kinds    []string
}

// dependencies are a claim's subject, a relation's endpoints, and every ID the record's typed
// details reference (DCX-V1-365, DCX-V1-442).
func (r *deltaRecord) dependencies() []deltaDependency {
	deps := []deltaDependency{}
	switch r.kind {
	case "claim":
		deps = append(deps, deltaDependency{r.claim.Subject, "claim subject", []string{"subject"}})
	case "relation":
		for _, end := range []string{r.relation.From, r.relation.To} {
			deps = append(deps, deltaDependency{end, "relation endpoint", []string{"subject", "claim"}})
		}
	}
	if r.details != nil {
		for _, id := range detailReferences(*r.details) {
			deps = append(deps, deltaDependency{id, "typed detail reference", nil})
		}
	}
	return deps
}

// detailReferences lists the record IDs typed details join to, as validateAdoptionJoins checks
// them; a test link's target is its relation's endpoint.
func detailReferences(d RecordDetails) []string {
	refs := []string{}
	if f := d.Flow; f != nil {
		for _, s := range f.Sections {
			refs = append(refs, s.Paragraphs...)
		}
		refs = append(append(refs, f.Variations...), f.Entrypoints...)
	}
	if p := d.Paragraph; p != nil {
		refs = append(refs, p.Flow)
		if p.RetiredTo != "" {
			refs = append(refs, p.RetiredTo)
		}
	}
	if c := d.Coverage; c != nil {
		refs = append(refs, c.Denominator...)
	}
	if x := d.IntentComparison; x != nil {
		refs = append(append(append(append(refs, x.ReportedIntent...), x.ObservedClaims...), x.Missing...), x.Conflicts...)
	}
	if s := d.Status; s != nil {
		refs = append(refs, s.Supporting...)
	}
	return refs
}

// demoteDependencies keeps the carried set closed: every dependency of a carried record is in
// the current corpus or carried with it (DCX-V1-365, DCX-V1-442). A demotion re-checks the records
// that depend on the demoted ID, so the set is stable when the queue drains.
func (d *deltaRun) demoteDependencies(present map[string]bool) {
	carried := map[string]int{}
	dependents := map[string][]int{}
	deps := make([][]deltaDependency, len(d.records))
	for i, r := range d.records {
		if !r.carried() {
			continue
		}
		carried[r.kind+"\x00"+r.entry.ID]++
		deps[i] = r.dependencies()
		for _, dep := range deps[i] {
			dependents[dep.id] = append(dependents[dep.id], i)
		}
	}
	have := func(dep deltaDependency) bool {
		if present[dep.id] {
			return true
		}
		kinds := dep.kinds
		if kinds == nil {
			kinds = []string{"subject", "claim", "relation"}
		}
		for _, kind := range kinds {
			if carried[kind+"\x00"+dep.id] > 0 {
				return true
			}
		}
		return false
	}
	// An exact test join needs its file among the anchors its target test has in the next build:
	// the current subject's, or the carried subject's re-pinned anchors (DCX-V1-442).
	testPaths := map[string][]string{}
	for _, s := range d.current.Subjects {
		for _, a := range s.Evidence.Anchors {
			testPaths[s.ID] = append(testPaths[s.ID], a.Path)
		}
	}
	for _, r := range d.records {
		if r.kind == "subject" && !present[r.entry.ID] {
			for _, a := range r.anchors {
				testPaths[r.entry.ID] = append(testPaths[r.entry.ID], a.Path)
			}
		}
	}
	demote := func(r *deltaRecord, reason string) {
		r.entry.Class, r.entry.Reason = DeltaUnresolved, reason
		carried[r.kind+"\x00"+r.entry.ID]--
	}
	queue := make([]int, len(d.records))
	for i := range queue {
		queue[i] = i
	}
	for next := 0; next < len(queue); next++ {
		r := d.records[queue[next]]
		if !r.carried() {
			continue
		}
		for _, dep := range deps[queue[next]] {
			if !have(dep) {
				demote(r, dep.what+" "+dep.id+" is neither in the current corpus nor carried forward")
				break
			}
		}
		if x := r.details; r.carried() && !r.entry.PresentInCurrent && r.kind == "relation" && x != nil && x.TestLink != nil && x.TestLink.JoinConfidence == "exact" && !slices.Contains(testPaths[r.relation.To], x.TestLink.File) {
			demote(r, "exact test join file "+x.TestLink.File+" is not anchored by test "+r.relation.To+" at the current revision")
		}
		// A record the current corpus holds still satisfies its dependents.
		if !r.carried() && !present[r.entry.ID] {
			queue = append(queue, dependents[r.entry.ID]...)
		}
	}
}

// classifyEvidence classifies every anchor and gives the record the most
// severe anchor state: removed, changed, unresolved, moved, carry-forward.
func (d *deltaRun) classifyEvidence(ctx context.Context, r *deltaRecord, e Evidence) error {
	if len(e.Anchors) == 0 {
		r.entry.Class, r.entry.Reason = DeltaUnresolved, deltaAnchorlessReason
		return nil
	}
	rank := map[string]int{DeltaRemoved: 0, DeltaChanged: 1, DeltaUnresolved: 2, DeltaMoved: 3, DeltaCarryForward: 4}
	r.entry.Class = DeltaCarryForward
	worst := 5
	for _, a := range e.Anchors {
		da, pinned, err := d.classifyAnchor(ctx, a)
		if err != nil {
			return err
		}
		r.entry.Anchors = append(r.entry.Anchors, da)
		r.anchors = append(r.anchors, pinned)
		if rank[da.State] < worst {
			worst = rank[da.State]
			r.entry.Class, r.entry.Reason = da.State, da.Reason
		}
	}
	return nil
}

func formatDeltaAnchor(a Anchor) string {
	if a.Symbol != "" {
		return FormatSymbolAnchor(a)
	}
	return fmt.Sprintf("%s:%d-%d@%s", a.Path, a.Start, a.End, a.Revision)
}

// classifyAnchor re-validates one previous anchor at the current revision and
// returns the anchor re-pinned there when its exact span hash is confirmed.
func (d *deltaRun) classifyAnchor(ctx context.Context, a Anchor) (DeltaAnchor, Anchor, error) {
	da := DeltaAnchor{Anchor: formatDeltaAnchor(a), Previous: DeltaSpan{Path: a.Path, Blob: a.Blob, SHA256: a.SHA256, Start: a.Start, End: a.End, SpanSHA256: a.SpanSHA256}}
	state := func(s, reason string) (DeltaAnchor, Anchor, error) {
		da.State, da.Reason = s, reason
		return da, Anchor{}, nil
	}
	repo := d.repoOf(a)
	if !repo.primary {
		da.Repository = repo.id
	}
	switch {
	case repo.unavailable != "":
		return state(DeltaUnresolved, repo.unavailable)
	case a.Authority != "external-provider":
		return state(DeltaUnresolved, "anchor authority is "+a.Authority+"; only imported external-provider evidence is carried")
	case a.Start < 1 || a.End < a.Start || a.SpanSHA256 == "":
		return state(DeltaUnresolved, "anchor has no valid span hash to confirm")
	}
	in, declared := repo.inputs[a.Path]
	if !declared {
		entry, exists, err := repo.pre.lookup(ctx, repo.auth, repo.revision, a.Path)
		if err != nil {
			return da, Anchor{}, err
		}
		if moved, ok, err := d.symbolMove(ctx, repo, a, &da); err != nil || ok {
			return da, moved, err
		}
		switch {
		case da.State == DeltaChanged:
			return d.changedLines(ctx, repo, a, da)
		case da.State != "":
			return da, Anchor{}, nil
		case exists && regularBlob(entry):
			return state(DeltaUnresolved, "anchored path is not a declared input of the current manifest")
		case exists:
			return state(DeltaRemoved, "anchored path is no longer a regular file at the current revision")
		}
		return state(DeltaRemoved, "anchored path is absent at the current revision")
	}
	data, reason, err := d.blobIn(ctx, repo, in.Blob, in.SHA256)
	if err != nil {
		return da, Anchor{}, err
	}
	if reason != "" {
		return state(DeltaUnresolved, "current "+reason)
	}
	pin := func(s, e int) Anchor {
		p := a
		p.Revision, p.Blob, p.SHA256, p.Start, p.End = repo.revision, in.Blob, in.SHA256, s, e
		da.Current = &DeltaSpan{Path: a.Path, Blob: in.Blob, SHA256: in.SHA256, Start: s, End: e, SpanSHA256: a.SpanSHA256}
		return p
	}
	if in.Blob == a.Blob {
		digest, ok := d.cachedSpanHash(deltaSpanKey{repo.id, a.Path, in.Blob, a.Start, a.End}, data)
		if !ok {
			return state(DeltaUnresolved, deltaHashBoundReason)
		}
		if digest == a.SpanSHA256 {
			da.State, da.Reason = DeltaCarryForward, "anchored blob is unchanged at the current revision"
			return da, pin(a.Start, a.End), nil
		}
	}
	oldData, oldReason, err := d.blobIn(ctx, repo, a.Blob, a.SHA256)
	if err != nil {
		return da, Anchor{}, err
	}
	var oldSpan []byte
	if oldReason == "" {
		lines := spanLines(oldData)
		digest, ok := d.cachedSpanHash(deltaSpanKey{repo.id, a.Path, a.Blob, a.Start, a.End}, oldData)
		if !ok {
			return state(DeltaUnresolved, deltaHashBoundReason)
		}
		if digest == a.SpanSHA256 {
			oldSpan = []byte(strings.Join(lines[a.Start-1:a.End], ""))
		}
	}
	same, others, bounded := d.place(data, a, oldSpan)
	switch {
	case bounded:
		return state(DeltaUnresolved, deltaHashBoundReason)
	case same:
		if err := d.confirmSymbol(ctx, repo, a, a.Path, a.Start, a.End); err != "" {
			return state(DeltaUnresolved, err)
		}
		da.State, da.Reason = DeltaCarryForward, "span hash confirmed at its pinned lines in the current blob"
		return da, pin(a.Start, a.End), nil
	case len(others) == 1:
		end := others[0] + a.End - a.Start
		if err := d.confirmSymbol(ctx, repo, a, a.Path, others[0], end); err != "" {
			return state(DeltaUnresolved, err)
		}
		da.State, da.Reason = DeltaMoved, "span hash confirmed at new lines in the same path"
		return da, pin(others[0], end), nil
	case len(others) > 1:
		return state(DeltaUnresolved, "span hash matches more than one window in the current blob")
	}
	if moved, ok, err := d.symbolMove(ctx, repo, a, &da); err != nil || ok {
		return da, moved, err
	}
	switch da.State {
	case DeltaChanged:
		return d.changedLines(ctx, repo, a, da)
	case DeltaRemoved, DeltaUnresolved:
		return da, Anchor{}, nil
	}
	da.State, da.Reason = DeltaChanged, "span hash is absent from the current blob"
	if oldReason != "" {
		da.Reason += "; changed lines not computed: previous " + oldReason
		return da, Anchor{}, nil
	}
	newLines := splitLines(data)
	hunks := lineDiff(splitLines(oldData), newLines, deltaMaxEdits)
	da.ChangedLines, da.ChangedTruncated = overlappingHunks(hunks, a.Start, a.End, deltaHunksPerAnchor)
	start, okStart := mapLine(hunks, a.Start, false)
	end, okEnd := mapLine(hunks, a.End, true)
	start, end = max(start, 1), min(end, len(newLines))
	if (okStart || okEnd) && start <= end {
		digest, ok := d.spanHash(spanLines(data), start, end)
		if !ok {
			return state(DeltaUnresolved, deltaHashBoundReason)
		}
		da.Current = &DeltaSpan{Path: a.Path, Blob: in.Blob, SHA256: in.SHA256, Start: start, End: end, SpanSHA256: digest, Estimated: true}
	} else {
		da.Reason = "anchored lines were deleted from the current blob"
	}
	return da, Anchor{}, nil
}

// changedLines fills the changed-line hunks of an anchor the symbol resolver
// reported changed, diffing the previous blob against the current blob of the
// re-resolved path while keeping the resolver's estimated current span.
func (d *deltaRun) changedLines(ctx context.Context, repo *deltaRepo, a Anchor, da DeltaAnchor) (DeltaAnchor, Anchor, error) {
	path := a.Path
	if da.Current != nil {
		path = da.Current.Path
	}
	in, declared := repo.inputs[path]
	if !declared {
		da.Reason += "; changed lines not computed: " + path + " is not a declared input of the current manifest"
		return da, Anchor{}, nil
	}
	data, reason, err := d.blobIn(ctx, repo, in.Blob, in.SHA256)
	if err == nil && reason == "" {
		var oldData []byte
		oldData, reason, err = d.blobIn(ctx, repo, a.Blob, a.SHA256)
		if err == nil && reason == "" {
			hunks := lineDiff(splitLines(oldData), splitLines(data), deltaMaxEdits)
			da.ChangedLines, da.ChangedTruncated = overlappingHunks(hunks, a.Start, a.End, deltaHunksPerAnchor)
			return da, Anchor{}, nil
		}
	}
	if err != nil {
		return da, Anchor{}, err
	}
	da.Reason += "; changed lines not computed: " + reason
	return da, Anchor{}, nil
}

// place reports whether the span hash holds at the pinned lines and the other
// line windows where it holds (at most two). A known previous span narrows the
// candidates to windows whose first line matches before any hashing.
func (d *deltaRun) place(data []byte, a Anchor, oldSpan []byte) (same bool, others []int, bounded bool) {
	lines := spanLines(data)
	width := a.End - a.Start + 1
	if a.End <= len(lines) {
		digest, ok := d.spanHash(lines, a.Start, a.End)
		if !ok {
			return false, nil, true
		}
		if digest == a.SpanSHA256 {
			return true, nil, false
		}
	}
	firstLine := ""
	if oldSpan != nil {
		firstLine = spanLines(oldSpan)[0]
	}
	for first := 0; first+width <= len(lines); first++ {
		if first == a.Start-1 || oldSpan != nil && lines[first] != firstLine {
			continue
		}
		digest, ok := d.spanHash(lines, first+1, first+width)
		if !ok {
			return false, nil, true
		}
		if digest == a.SpanSHA256 {
			others = append(others, first+1)
			if len(others) == 2 {
				break
			}
		}
	}
	return false, others, false
}

const deltaHashBoundReason = "the 64 MiB delta hashing bound was reached before the span could be confirmed"

// spanLines numbers lines exactly as spanDigest and the compiler's span do: a
// trailing newline leaves one final empty line, so an anchor may end there.
func spanLines(data []byte) []string {
	return strings.SplitAfter(string(data), "\n")
}

// spanHash digests lines start..end of lines, charging their bytes to the
// hashing bound before hashing. ok is false, with nothing charged or hashed,
// when the bytes would pass the bound; an out-of-range span digests to "".
func (d *deltaRun) spanHash(lines []string, start, end int) (string, bool) {
	if start < 1 || end > len(lines) || end < start {
		return "", true
	}
	joined := strings.Join(lines[start-1:end], "")
	limit := d.hashLimit
	if limit == 0 {
		limit = deltaHashMaxBytes
	}
	if d.hashed+len(joined) > limit {
		return "", false
	}
	d.hashed += len(joined)
	return Digest([]byte(joined)), true
}

// deltaSpanCacheMax bounds the verified-span cache; spans past it are hashed
// and charged each time.
const deltaSpanCacheMax = 1 << 16

// deltaSpanKey names one span of one blob read for one path and repository.
type deltaSpanKey struct {
	repo, path, blob string
	start, end       int
}

// cachedSpanHash is spanHash over data's lines, reusing the digest of a span
// already hashed under key. Only a digest actually computed is cached, so a
// span never hashed is never confirmed; callers still compare the digest.
func (d *deltaRun) cachedSpanHash(key deltaSpanKey, data []byte) (string, bool) {
	if digest, ok := d.verified[key]; ok {
		return digest, true
	}
	digest, ok := d.spanHash(spanLines(data), key.start, key.end)
	if ok && digest != "" && len(d.verified) < deltaSpanCacheMax {
		if d.verified == nil {
			d.verified = map[deltaSpanKey]string{}
		}
		d.verified[key] = digest
	}
	return digest, ok
}

// liveIndex is the current revision's context index, built once on demand.
func (d *deltaRun) liveIndex(ctx context.Context) (*contextindex.Index, error) {
	if d.index == nil {
		index, err := contextindex.BuildRevisionContext(ctx, d.root, d.current.Manifest.Repository.Revision)
		if err != nil {
			return nil, err
		}
		d.index = index
	}
	return d.index, nil
}

// repoIndex is the context index of repo's current pinned revision: the primary's live index,
// or an additional repository's index built once from its own checkout, as the build does.
func (d *deltaRun) repoIndex(ctx context.Context, repo *deltaRepo) (*contextindex.Index, error) {
	if repo.primary {
		return d.liveIndex(ctx)
	}
	if repo.index == nil {
		index, err := contextindex.BuildRevisionContext(ctx, repo.dir, repo.revision)
		if err != nil {
			return nil, err
		}
		repo.index = index
	}
	return repo.index, nil
}

// confirmSymbol returns a reason when a symbol anchor's symbol is not declared
// inside its re-pinned span at the current revision, the condition the next
// build's anchor check enforces.
func (d *deltaRun) confirmSymbol(ctx context.Context, repo *deltaRepo, a Anchor, path string, start, end int) string {
	if a.Symbol == "" {
		return ""
	}
	index, err := d.repoIndex(ctx, repo)
	if err != nil {
		return "current revision symbol index unavailable: " + err.Error()
	}
	for _, s := range index.Symbols {
		if s.Path == path && s.Name == a.Symbol && s.Line >= start && s.Line <= end {
			return ""
		}
	}
	return "symbol " + a.Symbol + " is not declared inside the re-pinned span at the current revision"
}

// symbolMove re-resolves a symbol anchor through the symbol resolver
// (DCP-V1-140..142) when its span hash is absent from its own path. ok reports a
// moved anchor with a confirmed exact span hash; otherwise da may carry a
// removed, changed or unresolved state, or stay empty when the resolver does
// not apply.
func (d *deltaRun) symbolMove(ctx context.Context, repo *deltaRepo, a Anchor, da *DeltaAnchor) (Anchor, bool, error) {
	if !repo.primary || a.Symbol == "" || contextindex.SymbolRegionLanguage(a.Path) == "" {
		return Anchor{}, false, nil
	}
	if d.resolver == nil {
		index, err := d.liveIndex(ctx)
		if err != nil {
			da.State, da.Reason = DeltaUnresolved, "current revision symbol index unavailable: "+err.Error()
			return Anchor{}, false, nil
		}
		d.resolver = NewSymbolResolver(d.auth, index, d.current.Manifest.Repository, nil)
		d.resolver.prefetched = d.pre
	}
	check, err := d.resolver.Resolve(ctx, a)
	if err != nil {
		return Anchor{}, false, err
	}
	switch check.State {
	case SymbolAnchorGone:
		da.State, da.Reason = DeltaRemoved, "symbol re-resolution: "+check.Reason
		return Anchor{}, false, nil
	case SymbolAnchorChanged:
		da.State, da.Reason = DeltaChanged, "symbol re-resolution: "+check.Reason
		if l := check.Live; l != nil {
			da.Current = &DeltaSpan{Path: l.Path, Blob: l.Blob, SHA256: l.SHA256, Start: l.Start, End: l.End, SpanSHA256: l.SpanSHA256, Estimated: true}
		}
		return Anchor{}, false, nil
	case SymbolAnchorLive, SymbolAnchorMoved:
	default:
		da.State, da.Reason = DeltaUnresolved, "symbol re-resolution: "+check.Reason
		return Anchor{}, false, nil
	}
	l := check.Live
	in, declared := d.newInputs[l.Path]
	if !declared || in.Blob != l.Blob {
		da.State, da.Reason = DeltaUnresolved, "symbol re-resolved to "+l.Path+", which is not a declared input of the current manifest"
		return Anchor{}, false, nil
	}
	data, reason, err := d.blob(ctx, in.Blob, in.SHA256)
	if err != nil {
		return Anchor{}, false, err
	}
	if reason != "" {
		da.State, da.Reason = DeltaUnresolved, "current "+reason
		return Anchor{}, false, nil
	}
	digest, ok := d.spanHash(spanLines(data), l.Start, l.End)
	if !ok {
		da.State, da.Reason = DeltaUnresolved, deltaHashBoundReason
		return Anchor{}, false, nil
	}
	if digest != a.SpanSHA256 {
		da.State, da.Reason = DeltaChanged, "symbol region re-resolved to "+l.Path+" but its exact span hash differs"
		da.Current = &DeltaSpan{Path: l.Path, Blob: l.Blob, SHA256: l.SHA256, Start: l.Start, End: l.End, SpanSHA256: digest, Estimated: true}
		return Anchor{}, false, nil
	}
	if reason := d.confirmSymbol(ctx, repo, a, l.Path, l.Start, l.End); reason != "" {
		da.State, da.Reason = DeltaUnresolved, reason
		return Anchor{}, false, nil
	}
	p := a
	p.Revision, p.Path, p.Blob, p.SHA256, p.Start, p.End = d.current.Manifest.Repository.Revision, l.Path, in.Blob, in.SHA256, l.Start, l.End
	da.State, da.Reason = DeltaMoved, "symbol re-resolved with its span hash confirmed at "+l.Path
	da.Current = &DeltaSpan{Path: l.Path, Blob: in.Blob, SHA256: in.SHA256, Start: l.Start, End: l.End, SpanSHA256: a.SpanSHA256}
	return p, true, nil
}

// uncovered lists the source lines of the current revision that changed since
// the previous revision and that no records-provider claim covers
// (DCX-V1-364). Compiler-derived native claims exist for every declaration, so
// they do not count as coverage.
func (d *deltaRun) uncovered(ctx context.Context) error {
	revision := d.current.Manifest.Repository.Revision
	covers := map[string][][2]int{}
	cover := func(path string, s, e int) { covers[path] = append(covers[path], [2]int{s, e}) }
	native := map[string]bool{}
	for _, p := range d.current.Manifest.Providers {
		if p.Kind != "records" {
			native[p.ID] = true
		}
	}
	for _, c := range d.current.Claims {
		if native[c.Provider] {
			continue
		}
		for _, a := range c.Evidence.Anchors {
			if a.Revision == revision {
				cover(a.Path, a.Start, a.End)
			}
		}
	}
	for _, r := range d.records {
		if r.kind != "claim" {
			continue
		}
		for _, a := range r.entry.Anchors {
			if a.Current != nil && a.Repository == "" {
				cover(a.Current.Path, a.Current.Start, a.Current.End)
			}
		}
	}
	subjects := map[string][]deltaSubjectSpan{}
	for _, s := range d.current.Subjects {
		for _, a := range s.Evidence.Anchors {
			if a.Revision == revision {
				subjects[a.Path] = append(subjects[a.Path], deltaSubjectSpan{s.ID, a.Start, a.End})
			}
		}
	}
	unread := []string{}
	for _, p := range sortedKeys(d.newInputs) {
		in := d.newInputs[p]
		if in.Purpose != "source" {
			continue
		}
		old, existed := d.oldInputs[p]
		if existed && old.Blob == in.Blob {
			continue
		}
		data, reason, err := d.blob(ctx, in.Blob, in.SHA256)
		if err != nil {
			return err
		}
		if reason != "" {
			unread = append(unread, p+" ("+reason+")")
			continue
		}
		lines := splitLines(data)
		if len(lines) == 0 {
			continue
		}
		regions := []DeltaHunk{}
		why := "lines changed since the previous revision"
		if !existed {
			regions, why = append(regions, DeltaHunk{NewStart: 1, NewCount: len(lines)}), "path is new at the current revision"
		} else {
			oldData, oldReason, err := d.blob(ctx, old.Blob, old.SHA256)
			if err != nil {
				return err
			}
			if oldReason != "" {
				regions, why = append(regions, DeltaHunk{NewStart: 1, NewCount: len(lines), Coarse: true}), "whole file treated as changed: previous "+oldReason
			} else {
				regions = lineDiff(splitLines(oldData), lines, deltaMaxEdits)
				if len(regions) == 1 && regions[0].Coarse {
					why = "coarse region: the line diff exceeded its edit bound"
				}
			}
		}
		for _, h := range regions {
			if h.NewCount == 0 {
				continue
			}
			for _, r := range uncoveredRanges(h.NewStart, h.NewStart+h.NewCount-1, covers[p]) {
				d.regions = append(d.regions, regionEntry(p, in, data, r[0], r[1], why, subjects[p]))
			}
		}
	}
	if len(unread) > 0 {
		shown := unread[:min(len(unread), deltaLimitedPaths)]
		d.limitations = append(d.limitations, fmt.Sprintf("%d changed source inputs were not read, so their uncovered lines are unknown: %s", len(unread), strings.Join(shown, "; ")))
	}
	d.uncitedSubjects()
	return nil
}

type deltaSubjectSpan struct {
	id         string
	start, end int
}

// uncoveredRanges returns the sub-ranges of lines s..e that no covering span
// touches, in line order.
func uncoveredRanges(s, e int, covers [][2]int) [][2]int {
	sorted := append([][2]int(nil), covers...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i][0] < sorted[j][0] })
	out := [][2]int{}
	next := s
	for _, c := range sorted {
		if c[1] < next || c[0] > e {
			continue
		}
		if c[0] > next {
			out = append(out, [2]int{next, c[0] - 1})
		}
		next = max(next, c[1]+1)
		if next > e {
			return out
		}
	}
	return append(out, [2]int{next, e})
}

// regionEntry is one uncovered changed region with the current subjects
// anchored inside it (at most deltaRegionSubjects). Regions are disjoint
// sub-ranges of read blobs, so their digests stay inside the read bound.
func regionEntry(p string, in Input, data []byte, s, e int, why string, subjects []deltaSubjectSpan) DeltaEntry {
	entry := DeltaEntry{Class: DeltaUncovered, RecordKind: "region", ID: fmt.Sprintf("%s:%d-%d", p, s, e), Reason: why, Region: &DeltaSpan{Path: p, Blob: in.Blob, SHA256: in.SHA256, Start: s, End: e, SpanSHA256: spanDigest(data, s, e)}}
	named := map[string]bool{}
	for _, sub := range subjects {
		if sub.start <= e && sub.end >= s {
			named[sub.id] = true
		}
	}
	ids := sortedKeys(named)
	if len(ids) > deltaRegionSubjects {
		entry.SubjectsOmitted = len(ids) - deltaRegionSubjects
		ids = ids[:deltaRegionSubjects]
	}
	entry.Subjects = ids
	return entry
}

// uncitedSubjects lists every current subject that no current claim, of any
// provider, names as its subject (DCX-V1-364). The list is bounded by the
// corpus capacity and paged with the rest of the worklist.
func (d *deltaRun) uncitedSubjects() {
	cited := map[string]bool{}
	for _, c := range d.current.Claims {
		cited[c.Subject] = true
	}
	for _, s := range d.current.Subjects {
		if cited[s.ID] {
			continue
		}
		d.regions = append(d.regions, DeltaEntry{Class: DeltaUncovered, RecordKind: "subject", ID: s.ID, Provider: s.Provider, Reason: "no claim in the current corpus names this subject"})
	}
}
