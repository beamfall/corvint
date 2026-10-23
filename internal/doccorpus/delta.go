package doccorpus

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

// Cross-revision delta worklist (DCP-V1-360..369): the previous corpus's
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

// Delta read bounds (DCP-V1-367): per blob, admitted bytes in total, bytes
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
	Anchor           string      `json:"anchor"`
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

// DeltaReceipt is the paged `docs corpus delta` worklist (DCP-V1-360).
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
}

// deltaRecord is one classified previous record with its re-pinned evidence.
type deltaRecord struct {
	entry    DeltaEntry
	kind     string
	subject  Subject
	claim    Claim
	relation Relation
	anchors  []Anchor
}

type deltaRun struct {
	root          string
	previous      *Artifact
	current       *Artifact
	auth          *gitauth.Repository
	pre           *pathPrefetch
	newInputs     map[string]Input
	oldInputs     map[string]Input
	blobs         map[string][]byte
	blobReasons   map[string]string
	read, hashed  int
	hashLimit     int // deltaHashMaxBytes; zero means the default
	readLimit     int // deltaReadMaxBytes; zero means the default
	index         *contextindex.Index
	resolver      *SymbolResolver
	records       []*deltaRecord
	regions       []DeltaEntry
	limitations   []string
	skippedNative int
}

// Delta classifies previous against current (DCP-V1-360..365) and returns one
// page of the worklist. current is the opened or freshly built current corpus;
// source is "artifact" or "manifest". It reads Git objects only.
func Delta(ctx context.Context, root string, previous, current *Artifact, source string, offset, limit int) (*DeltaReceipt, error) {
	if limit == 0 {
		limit = DeltaDefaultLimit
	}
	if limit < 1 || limit > MaxResults || offset < 0 {
		return nil, fail("delta limit must be 1..256 and offset non-negative")
	}
	run, err := classifyDelta(ctx, root, previous, current)
	if err != nil {
		return nil, err
	}
	entries := []DeltaEntry{}
	for _, r := range run.records {
		entries = append(entries, r.entry)
	}
	entries = append(entries, run.regions...)
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if deltaClassOrder[a.Class] != deltaClassOrder[b.Class] {
			return deltaClassOrder[a.Class] < deltaClassOrder[b.Class]
		}
		if deltaKindOrder[a.RecordKind] != deltaKindOrder[b.RecordKind] {
			return deltaKindOrder[a.RecordKind] < deltaKindOrder[b.RecordKind]
		}
		return a.ID < b.ID
	})
	counts := map[string]int{DeltaCarryForward: 0, DeltaMoved: 0, DeltaChanged: 0, DeltaRemoved: 0, DeltaUnresolved: 0, DeltaUncovered: 0}
	for _, e := range entries {
		counts[e.Class]++
	}
	receipt := &DeltaReceipt{Schema: DeltaSchema, Previous: deltaSide("artifact", previous), Current: deltaSide(source, current), Counts: counts, Total: len(entries), Offset: offset, Limit: limit, Entries: []DeltaEntry{}, Limitations: run.limitations}
	if offset < len(entries) {
		end := min(len(entries), offset+limit)
		receipt.Entries = entries[offset:end]
		if end < len(entries) {
			receipt.NextOffset = end
		}
	}
	return receipt, nil
}

// CarryForward returns a corvint-corpus-provider/1 record holding provider's
// previous records that the delta classes carry-forward or moved and that the
// current corpus does not already hold, each anchor re-pinned to the current
// revision with its span hash unchanged (DCP-V1-366). The record is meant to be
// committed and declared as a records provider of the next build.
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
	for _, r := range run.records {
		if r.entry.Provider != provider || r.entry.PresentInCurrent || r.entry.Class != DeltaCarryForward && r.entry.Class != DeltaMoved {
			continue
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
	if err := carriedClosure(run, record, provider); err != nil {
		return nil, err
	}
	if len(record.Subjects) > MaxRecords || len(record.Claims) > MaxRecords || len(record.Relations) > MaxRecords {
		return nil, capacityExceeded("carried records of one kind", max(len(record.Subjects), len(record.Claims), len(record.Relations)), MaxRecords)
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
// demoted records whose dependency is carried by no provider (DCP-V1-365), so
// what remains is a dependency another provider carries; the refusal names it
// so the caller can carry and build that provider first (DCP-V1-366).
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
		if r.entry.Class == DeltaCarryForward || r.entry.Class == DeltaMoved {
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

func classifyDelta(ctx context.Context, root string, previous, current *Artifact) (_ *deltaRun, err error) {
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
	d := &deltaRun{root: root, previous: previous, current: current, auth: auth, pre: newPathPrefetch(current.Manifest.Repository.Revision), hashLimit: deltaHashMaxBytes, readLimit: deltaReadMaxBytes, newInputs: map[string]Input{}, oldInputs: map[string]Input{}, blobs: map[string][]byte{}, blobReasons: map[string]string{}}
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
	if err := d.prefetch(ctx); err != nil {
		return nil, err
	}
	if err := d.classifyRecords(ctx); err != nil {
		return nil, err
	}
	if err := d.uncovered(ctx); err != nil {
		return nil, err
	}
	d.limitations = append(d.limitations,
		"journeys, observations and supplied gaps of the previous corpus are not classified",
		"anchors in additional repositories are reported unresolved and never carried",
		"symbol re-resolution across paths covers the languages the symbol resolver normalizes; other symbol anchors are placed by span hash in their own path only")
	if d.skippedNative > 0 {
		d.limitations = append(d.limitations, fmt.Sprintf("%d native or compiler-derived previous records are rebuilt by the compiler and not classified", d.skippedNative))
	}
	return d, nil
}

// prefetch batches the tree lookups and blob reads the classification needs,
// so the Git operation count does not grow with the record count.
func (d *deltaRun) prefetch(ctx context.Context) error {
	oids, paths := []string{}, []string{}
	for _, list := range [][]Evidence{subjectEvidence(d.previous.Subjects), claimEvidence(d.previous.Claims), relationEvidence(d.previous.Relations)} {
		for _, e := range list {
			for _, a := range e.Anchors {
				if a.Repository != d.previous.Manifest.Repository.ID {
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
	return d.pre.read(ctx, d.auth, oids, deltaFileMaxBytes, d.readLimit)
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
	if data, ok := d.blobs[oid]; ok {
		return data, "", nil
	}
	if reason, ok := d.blobReasons[oid]; ok {
		return nil, reason, nil
	}
	budget := d.readLimit
	if budget == 0 {
		budget = deltaReadMaxBytes
	}
	limit := min(deltaFileMaxBytes, budget-d.read)
	if limit < 1 {
		reason := "the 64 MiB delta read bound was reached before blob " + oid
		d.blobReasons[oid] = reason
		return nil, reason, nil
	}
	// The limit is checked against the object header before any body is read.
	data, over, err := d.pre.blobWithin(ctx, d.auth, oid, limit)
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
		d.blobReasons[oid] = reason
		return nil, reason, nil
	}
	d.read += len(data)
	d.blobs[oid] = data
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
	add := func(kind, id, provider string, e Evidence, kindOK bool, r *deltaRecord) error {
		if !recordsProviders[provider] {
			d.skippedNative++
			return nil
		}
		r.kind = kind
		r.entry = DeltaEntry{RecordKind: kind, ID: id, Provider: provider, PresentInCurrent: present[id]}
		if err := d.classifyEvidence(ctx, r, e); err != nil {
			return err
		}
		if !kindOK && (r.entry.Class == DeltaCarryForward || r.entry.Class == DeltaMoved) {
			r.entry.Class, r.entry.Reason = DeltaUnresolved, "record kind is adoption-only; carry-forward emits generic provider records only"
		}
		d.records = append(d.records, r)
		return nil
	}
	for _, s := range d.previous.Subjects {
		if err := add("subject", s.ID, s.Provider, s.Evidence, subjectKinds[s.Kind], &deltaRecord{subject: s}); err != nil {
			return err
		}
	}
	for _, c := range d.previous.Claims {
		if err := add("claim", c.ID, c.Provider, c.Evidence, true, &deltaRecord{claim: c}); err != nil {
			return err
		}
	}
	for _, r := range d.previous.Relations {
		if err := add("relation", r.ID, r.Provider, r.Evidence, relationKinds[r.Type], &deltaRecord{relation: r}); err != nil {
			return err
		}
	}
	d.demoteDependencies(present)
	return nil
}

// demoteDependencies keeps the carried set closed: a carried claim needs its
// subject, and a carried relation its endpoints, in the current corpus or
// carried with it (DCP-V1-365).
func (d *deltaRun) demoteDependencies(present map[string]bool) {
	carried := func(kind string) map[string]bool {
		out := map[string]bool{}
		for _, r := range d.records {
			if r.kind == kind && (r.entry.Class == DeltaCarryForward || r.entry.Class == DeltaMoved) {
				out[r.entry.ID] = true
			}
		}
		return out
	}
	subjects := carried("subject")
	for _, r := range d.records {
		if r.kind == "claim" && (r.entry.Class == DeltaCarryForward || r.entry.Class == DeltaMoved) && !present[r.claim.Subject] && !subjects[r.claim.Subject] {
			r.entry.Class, r.entry.Reason = DeltaUnresolved, "claim subject "+r.claim.Subject+" is neither in the current corpus nor carried forward"
		}
	}
	claims := carried("claim")
	for _, r := range d.records {
		if r.kind != "relation" || r.entry.Class != DeltaCarryForward && r.entry.Class != DeltaMoved {
			continue
		}
		for _, end := range []string{r.relation.From, r.relation.To} {
			if !present[end] && !subjects[end] && !claims[end] {
				r.entry.Class, r.entry.Reason = DeltaUnresolved, "relation endpoint "+end+" is neither in the current corpus nor carried forward"
				break
			}
		}
	}
}

// classifyEvidence classifies every anchor and gives the record the most
// severe anchor state: removed, changed, unresolved, moved, carry-forward.
func (d *deltaRun) classifyEvidence(ctx context.Context, r *deltaRecord, e Evidence) error {
	if len(e.Anchors) == 0 {
		r.entry.Class, r.entry.Reason = DeltaUnresolved, "record has no anchored span whose hash could be confirmed"
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
	switch {
	case a.Repository != d.previous.Manifest.Repository.ID:
		return state(DeltaUnresolved, "anchor names an additional repository; the delta re-validates the primary repository only")
	case a.Authority != "external-provider":
		return state(DeltaUnresolved, "anchor authority is "+a.Authority+"; only imported external-provider evidence is carried")
	case a.Start < 1 || a.End < a.Start || a.SpanSHA256 == "":
		return state(DeltaUnresolved, "anchor has no valid span hash to confirm")
	}
	in, declared := d.newInputs[a.Path]
	if !declared {
		entry, exists, err := d.pre.lookup(ctx, d.auth, d.current.Manifest.Repository.Revision, a.Path)
		if err != nil {
			return da, Anchor{}, err
		}
		if moved, ok, err := d.symbolMove(ctx, a, &da); err != nil || ok {
			return da, moved, err
		}
		switch {
		case da.State == DeltaChanged:
			return d.changedLines(ctx, a, da)
		case da.State != "":
			return da, Anchor{}, nil
		case exists && regularBlob(entry):
			return state(DeltaUnresolved, "anchored path is not a declared input of the current manifest")
		case exists:
			return state(DeltaRemoved, "anchored path is no longer a regular file at the current revision")
		}
		return state(DeltaRemoved, "anchored path is absent at the current revision")
	}
	data, reason, err := d.blob(ctx, in.Blob, in.SHA256)
	if err != nil {
		return da, Anchor{}, err
	}
	if reason != "" {
		return state(DeltaUnresolved, "current "+reason)
	}
	pin := func(s, e int) Anchor {
		p := a
		p.Revision, p.Blob, p.SHA256, p.Start, p.End = d.current.Manifest.Repository.Revision, in.Blob, in.SHA256, s, e
		da.Current = &DeltaSpan{Path: a.Path, Blob: in.Blob, SHA256: in.SHA256, Start: s, End: e, SpanSHA256: a.SpanSHA256}
		return p
	}
	if in.Blob == a.Blob {
		digest, ok := d.spanHash(spanLines(data), a.Start, a.End)
		if !ok {
			return state(DeltaUnresolved, deltaHashBoundReason)
		}
		if digest == a.SpanSHA256 {
			da.State, da.Reason = DeltaCarryForward, "anchored blob is unchanged at the current revision"
			return da, pin(a.Start, a.End), nil
		}
	}
	oldData, oldReason, err := d.blob(ctx, a.Blob, a.SHA256)
	if err != nil {
		return da, Anchor{}, err
	}
	var oldSpan []byte
	if oldReason == "" {
		lines := spanLines(oldData)
		digest, ok := d.spanHash(lines, a.Start, a.End)
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
		if err := d.confirmSymbol(ctx, a, a.Path, a.Start, a.End); err != "" {
			return state(DeltaUnresolved, err)
		}
		da.State, da.Reason = DeltaCarryForward, "span hash confirmed at its pinned lines in the current blob"
		return da, pin(a.Start, a.End), nil
	case len(others) == 1:
		end := others[0] + a.End - a.Start
		if err := d.confirmSymbol(ctx, a, a.Path, others[0], end); err != "" {
			return state(DeltaUnresolved, err)
		}
		da.State, da.Reason = DeltaMoved, "span hash confirmed at new lines in the same path"
		return da, pin(others[0], end), nil
	case len(others) > 1:
		return state(DeltaUnresolved, "span hash matches more than one window in the current blob")
	}
	if moved, ok, err := d.symbolMove(ctx, a, &da); err != nil || ok {
		return da, moved, err
	}
	switch da.State {
	case DeltaChanged:
		return d.changedLines(ctx, a, da)
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
func (d *deltaRun) changedLines(ctx context.Context, a Anchor, da DeltaAnchor) (DeltaAnchor, Anchor, error) {
	path := a.Path
	if da.Current != nil {
		path = da.Current.Path
	}
	in, declared := d.newInputs[path]
	if !declared {
		da.Reason += "; changed lines not computed: " + path + " is not a declared input of the current manifest"
		return da, Anchor{}, nil
	}
	data, reason, err := d.blob(ctx, in.Blob, in.SHA256)
	if err == nil && reason == "" {
		var oldData []byte
		oldData, reason, err = d.blob(ctx, a.Blob, a.SHA256)
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

// confirmSymbol returns a reason when a symbol anchor's symbol is not declared
// inside its re-pinned span at the current revision, the condition the next
// build's anchor check enforces.
func (d *deltaRun) confirmSymbol(ctx context.Context, a Anchor, path string, start, end int) string {
	if a.Symbol == "" {
		return ""
	}
	index, err := d.liveIndex(ctx)
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
func (d *deltaRun) symbolMove(ctx context.Context, a Anchor, da *DeltaAnchor) (Anchor, bool, error) {
	if a.Symbol == "" || contextindex.SymbolRegionLanguage(a.Path) == "" {
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
	if reason := d.confirmSymbol(ctx, a, l.Path, l.Start, l.End); reason != "" {
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
// (DCP-V1-364). Compiler-derived native claims exist for every declaration, so
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
			if a.Current != nil {
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
// provider, names as its subject (DCP-V1-364). The list is bounded by the
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
