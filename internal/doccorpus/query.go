package doccorpus

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/contextindex"
)

// Freshness is a live overlay; it never rewrites the immutable artifact.
func Freshness(ctx context.Context, root string, a *Artifact) (string, []string, error) {
	overlay, err := liveFreshness(ctx, root, a, false)
	return overlay.State, overlay.Limitations, err
}

// FreshnessOverlay is a corpus reader's live overlay: the artifact-wide state
// and limitations Freshness reports, and the live tree that re-resolves the
// symbol anchors a receipt cites (DCP-V1-144).
type FreshnessOverlay struct {
	State       string
	Limitations []string
	symbols     *symbolOverlay
	observation string
}

// Observation is the ObservationKey of the repository state the overlay was
// built from, so a reader can reuse it only while that state holds
// (DCP-V1-144).
func (o FreshnessOverlay) Observation() string { return o.observation }

// ObservationKey identifies a repository state by the fields
// contextindex.Observe reports.
func ObservationKey(o contextindex.Observation) string {
	dirty := append([]string{}, o.DirtyPaths...)
	sort.Strings(dirty)
	return strings.Join(append([]string{o.ObjectFormat, o.CommitRevision, o.Revision, o.StatusSHA256}, dirty...), "\x00")
}

// LiveFreshness is Freshness plus symbol-anchor re-resolution (DCP-V1-141).
// The overall state is unchanged by the symbol checks.
func LiveFreshness(ctx context.Context, root string, a *Artifact) (FreshnessOverlay, error) {
	return liveFreshness(ctx, root, a, true)
}

func liveFreshness(ctx context.Context, root string, a *Artifact, symbols bool) (_ FreshnessOverlay, err error) {
	defer func() {
		err = gitCapacity(err, fmt.Sprintf("while checking freshness of %d inputs", len(a.Manifest.Inputs)))
	}()
	unknown := FreshnessOverlay{State: "unknown"}
	primary := func(revision string) bool { _, ok := a.Manifest.additionalRepository(revision); return !ok }
	live, stale, limits, err := liveRepository(ctx, root, a, primary, "")
	if err != nil {
		return unknown, err
	}
	limits = append(limits, "structural source binding does not prove semantic truth, test adequacy or accepted intent")
	if a.Builder.Revision == "unrecorded" || strings.HasSuffix(a.Builder.Revision, "+dirty") {
		limits = append(limits, "builder source revision is unrecorded or dirty; no immutable build attestation")
	}
	observation := ObservationKey(contextindex.Observation{
		ObjectFormat: live.ObjectFormat, CommitRevision: live.CommitRevision, Revision: live.Revision, StatusSHA256: live.StatusSHA256, DirtyPaths: live.DirtyPaths})
	// Each additional repository's inputs are checked against its own declared checkout, never
	// the primary tree (DCP-V1-178). The primary observation alone then no longer identifies the
	// state, so the overlay carries no reusable observation key.
	for _, r := range a.Manifest.AdditionalRepositories {
		member := func(revision string) bool { return revision == r.Revision }
		_, changed, reasons, err := liveRepository(ctx, a.Manifest.RevisionRoot(root, r.Revision), a, member, "additional repository "+r.ID+": ")
		if err != nil {
			return unknown, err
		}
		stale = stale || changed
		limits = append(limits, reasons...)
		observation = ""
	}
	state := "fresh"
	if stale {
		state = "stale"
	}
	sort.Strings(limits)
	if a.Schema == SchemaV2 {
		unique := []string{}
		for _, x := range limits {
			if len(unique) == 0 || unique[len(unique)-1] != x {
				unique = append(unique, x)
			}
		}
		limits = unique
		if len(limits) > MaxResults {
			omitted := len(limits) - MaxResults
			limits = append(limits[:MaxResults], fmt.Sprintf("freshness diagnostic bound withheld %d reasons", omitted))
		}
	}
	overlay := FreshnessOverlay{State: state, Limitations: limits, observation: observation}
	if symbols {
		overlay.symbols = &symbolOverlay{root: root, live: live, repository: a.Manifest.Repository.ID, files: &sharedSymbolFiles{files: map[string]sharedSymbolFile{}}}
	}
	return overlay, nil
}

// liveRepository compares the scopes and inputs whose revision member admits against the live
// checkout at dir, returning whether any is stale and each reason under prefix.
func liveRepository(ctx context.Context, dir string, a *Artifact, member func(string) bool, prefix string) (*contextindex.Index, bool, []string, error) {
	live, err := contextindex.BuildContext(ctx, dir, "")
	if err != nil {
		return nil, false, nil, err
	}
	auth, err := gitauth.Open(dir, newGitBudget())
	if err != nil {
		return nil, false, nil, err
	}
	release := auth.BeginObjectSession()
	defer release()
	stale := false
	limits := []string{}
	note := func(reason string) {
		stale = true
		limits = append(limits, prefix+reason)
	}
	livePath := func(reason, path string) string {
		if a.Schema == SchemaV2 {
			return reason + "[outside pinned inventory]"
		}
		return reason + path
	}
	declared := map[string]bool{}
	inputs := []Input{}
	for _, in := range a.Manifest.Inputs {
		if member(in.Revision) {
			declared[inputKey(in.Revision, in.Path)] = true
			inputs = append(inputs, in)
		}
	}
	for _, scope := range a.Manifest.Scopes {
		if !member(scope.Revision) {
			continue
		}
		for p := range live.Tracked {
			if inScope(p, scope.Path) && !declared[inputKey(scope.Revision, p)] {
				note(livePath("new input in declared scope: ", p))
			}
		}
		for p := range live.Skipped {
			if inScope(p, scope.Path) {
				note(livePath("unsupported live scope entry: ", p))
			}
		}
		for _, p := range live.DirtyPaths {
			if inScope(p, scope.Path) && !declared[inputKey(scope.Revision, p)] {
				note(livePath("new working-tree input in declared scope: ", p))
			}
		}
	}
	dirty := map[string]bool{}
	for _, p := range live.DirtyPaths {
		dirty[p] = true
	}
	// A clean path the live index already pinned at CommitRevision needs no
	// further tree walk (DCP-V1-144); every other path resolves in one verified
	// walk whose Git cost is independent of the input count (DCP-V1-155).
	lookups := []string{}
	for _, in := range inputs {
		if _, ok := live.Sources[in.Path]; !ok || dirty[in.Path] {
			lookups = append(lookups, in.Path)
		}
	}
	resolved := map[string]gitauth.TreeEntry{}
	if len(lookups) > 0 {
		if resolved, err = auth.ResolvePaths(ctx, live.CommitRevision, lookups); err != nil {
			return nil, false, nil, err
		}
	}
	for _, in := range inputs {
		// Historical provider records are independent of source freshness. A live
		// replacement of one is still visible; no HEAD equality heuristic is used.
		entry, exists := resolved[in.Path]
		if source, ok := live.Sources[in.Path]; ok && !dirty[in.Path] {
			entry, exists = gitauth.TreeEntry{Type: "blob", Mode: source.Mode, OID: source.BlobHash}, true
		}
		if !exists || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") || entry.OID != in.Blob {
			note("input changed or absent: " + in.Path)
		}
		if dirty[in.Path] {
			note("working-tree input differs: " + in.Path)
		}
	}
	return live, stale, limits, nil
}
func CapabilityFor(operation string) string {
	if cap := typedCapability(operation); cap != "" {
		return cap
	}
	if operation == "inventory" {
		return "info"
	}
	for _, name := range capabilityNames {
		for _, op := range capabilityTools(name) {
			if op == operation {
				return name
			}
		}
	}
	return ""
}
func (a *Artifact) HasCapability(name string) bool {
	for _, c := range a.Capabilities {
		if c.Name == name {
			return c.State == "present"
		}
	}
	return false
}

// Query consumes an already validated artifact. Entry points must call Open;
// pure callers can use the artifact returned by Build in the same operation.
func Query(a *Artifact, request Request, freshness string, limitations []string) (Receipt, error) {
	return QueryContext(context.Background(), a, request, freshness, limitations)
}

func QueryContext(ctx context.Context, a *Artifact, request Request, freshness string, limitations []string) (Receipt, error) {
	return queryContext(ctx, a, request, freshness, limitations, nil)
}

// QueryLive answers request under a live freshness overlay. Besides the
// overlay's state and limitations, the receipt reports the re-resolution of
// every symbol anchor it cites (DCP-V1-141). Only the anchors the receipt
// cites are re-resolved, on demand and under one budget per call, so the
// receipt is the one a fresh reader returns (DCP-V1-144).
func QueryLive(ctx context.Context, a *Artifact, request Request, overlay FreshnessOverlay) (Receipt, error) {
	checker := &symbolChecker{overlay: overlay.symbols, checks: map[string]SymbolAnchorCheck{}}
	defer checker.close()
	return queryContext(ctx, a, request, overlay.State, overlay.Limitations, checker)
}

func queryContext(ctx context.Context, a *Artifact, request Request, freshness string, limitations []string, symbols *symbolChecker) (Receipt, error) {
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	envelope, err := trustEnvelope(a, freshness, request.Retirement)
	if err != nil {
		return Receipt{}, err
	}
	if request.Limit == 0 {
		request.Limit = 20
	}
	if request.Limit < 1 || request.Limit > MaxResults || len(request.Query) > 1024 || len(request.ID) > 1024 || len(request.Path) > 1024 {
		return Receipt{}, fail("invalid query bounds")
	}
	if request.Depth < 0 || request.Depth > MaxFlowDepth || request.Depth != 0 && request.Operation != "trace" {
		return Receipt{}, fail("flow trace depth is bounded and applies only to trace")
	}
	if (request.Sections != nil || request.Include != nil) && (request.Operation != "flow" || !ValidFlowFilter(request.Sections, request.Include)) {
		return Receipt{}, fail("flow section and include filters are bounded and apply only to flow")
	}
	if request.Offset < 0 || request.Offset > MaxCorpusRecords*4 || a.Schema != SchemaV2 && (request.Offset != 0 || request.Operation == "inventory") {
		return Receipt{}, fail("pagination requires corpus /2 and bounded offset")
	}
	capability := CapabilityFor(request.Operation)
	if capability == "" {
		return Receipt{}, fail("unknown corpus operation")
	}
	r := Receipt{Envelope: envelope, Schema: ReceiptSchema, Operation: request.Operation, ArtifactSHA256: a.SHA256, Repository: a.Manifest.Repository, Tree: a.Tree, Trust: "generated", Freshness: freshness, State: "ready", Results: []any{}, Citations: []Anchor{}, Capabilities: a.Capabilities, Limitations: append([]string{}, limitations...)}
	if a.Schema == SchemaV2 {
		r.Capabilities = boundedCapabilities(a.Capabilities)
	}
	if !a.HasCapability(capability) && testKnowledgeServes(a, request.Operation) {
		r.Limitations = append(r.Limitations, capability+" capability absent; answered from test-knowledge records only")
	} else if !a.HasCapability(capability) {
		r.State = "unavailable"
		r.Miss = "capability-absent"
		for _, cap := range a.Capabilities {
			if cap.Name == capability {
				r.Limitations = append(r.Limitations, cap.Reason)
			}
		}
		finalizeEnvelope(&r)
		return r, nil
	}
	if freshness == "stale" {
		r.Limitations = append(r.Limitations, "evidence-stale; results describe the pinned revision only")
	}
	switch request.Operation {
	case "info", "validate":
		r.Results = append(r.Results, map[string]any{"builder": a.Builder, "manifest_sha256": a.ManifestSHA256, "profile_sha256": a.ProfileSHA256, "profile": a.Manifest.Profile, "built_at": a.Manifest.BuiltAt, "validation": "source-rederived"})
		if a.ImportParity != nil {
			r.Results = append(r.Results, map[string]any{"import_parity": a.ImportParity, "restricted_summaries": a.RestrictedSummaries, "scope": "supplied adoption records only; source completeness is not inferred"})
		}
	case "inventory":
		for _, x := range a.Subjects {
			r.Results = append(r.Results, x)
		}
		for _, x := range a.Claims {
			r.Results = append(r.Results, x)
		}
		for _, x := range a.Relations {
			r.Results = append(r.Results, x)
		}
		for _, x := range a.Journeys {
			r.Results = append(r.Results, x)
		}
		for _, x := range a.Observations {
			r.Results = append(r.Results, x)
		}
		for _, x := range a.TestKnowledge {
			r.Results = append(r.Results, x)
		}
		for _, x := range a.FlowRelations {
			r.Results = append(r.Results, x)
		}
		sort.Slice(r.Results, func(i, j int) bool { return recordID(r.Results[i]) < recordID(r.Results[j]) })
	case "search":
		if e := search(ctx, a, request, &r); e != nil {
			return Receipt{}, e
		}
	case "get", "trace":
		get(a, request.ID, &r)
		if request.Operation == "trace" && len(a.FlowRelations) > 0 && isFlowSubject(a, request.ID) {
			// DCP-V1-098: trace walks the same declared flow relations as impact/affected.
			r.Results = append(r.Results, TraceFlows(a, []string{request.ID}, request.Depth))
		}
	case "locate":
		for _, j := range testKnowledgeOnPath(a, request.Path) {
			addRecord(&r, j)
		}
		if a.RuntimeIndex != nil {
			for _, ref := range a.RuntimeIndex.Paths[request.Path] {
				if ref.Kind == "subject" {
					addRecord(&r, a.indexedRecord(ref))
				}
			}
			break
		}
		for _, s := range a.Subjects {
			if subjectPath(s, request.Path) {
				r.Results = append(r.Results, s)
				r.Citations = append(r.Citations, s.Evidence.Anchors...)
			}
		}
	case "related":
		for _, rel := range a.Relations {
			if rel.From == request.ID || rel.To == request.ID {
				r.Results = append(r.Results, rel)
				r.Citations = append(r.Citations, rel.Evidence.Anchors...)
			}
		}
	case "journey":
		for _, j := range a.Journeys {
			if j.ID == request.ID || j.Subject == request.ID {
				r.Results = append(r.Results, j)
				r.Citations = append(r.Citations, j.Evidence.Anchors...)
			}
		}
	case "stability":
		for _, stability := range a.StabilityEvidence {
			if stability.ID == request.ID || stability.TestID == request.ID || stability.ContractID == request.ID {
				r.Results = append(r.Results, stability)
			}
		}
	case "gaps":
		if a.RuntimeIndex == nil {
			copy := *a
			index, e := BuildQueryIndex(ctx, a)
			if e != nil {
				return Receipt{}, e
			}
			copy.RuntimeIndex = index
			a = &copy
		}
		generated, e := typedGaps(ctx, a, freshness)
		if e != nil {
			return Receipt{}, e
		}
		for _, g := range append(append([]Gap{}, a.Gaps...), generated...) {
			if err := ctx.Err(); err != nil {
				return Receipt{}, err
			}
			if request.ID == "" || g.Subject == request.ID {
				r.Results = append(r.Results, g)
			}
		}
		// DCP-V1-186: supplied gaps follow the computed ones, labelled by origin.
		for _, g := range a.SuppliedGaps {
			if request.ID == "" || g.Subject == request.ID || g.ID == request.ID {
				r.Results = append(r.Results, g)
				r.Citations = append(r.Citations, g.Evidence.Anchors...)
			}
		}
	case "concept", "claims", "flow", "dependencies", "recommend-tests", "navigation", "vocabulary", "intent":
		if a.RuntimeIndex == nil {
			copy := *a
			index, e := BuildQueryIndex(ctx, a)
			if e != nil {
				return Receipt{}, e
			}
			copy.RuntimeIndex = index
			a = &copy
		}
		if e := typedQuery(ctx, a, request, &r); e != nil {
			return Receipt{}, e
		}
	case "coverage":
		r.Results = append(r.Results, coverageMetrics(a)...)
		if a.ImportParity != nil {
			r.Results = append(r.Results, map[string]any{"import_parity": a.ImportParity, "restricted_summaries": a.RestrictedSummaries})
		}
		for _, cap := range a.Capabilities {
			r.Results = append(r.Results, map[string]any{"capability": cap.Name, "value": cap.Count, "denominator": cap.Denominator, "definition": "emitted records per declared input; not a behavioral coverage percentage", "rule": cap.Rule, "revision": a.Manifest.Repository.Revision, "defined": cap.Denominator > 0, "limitations": []string{cap.Reason}})
		}
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	if a.Schema == SchemaV2 {
		return pageReceipt(ctx, a, r, request, symbols)
	}
	if len(r.Results) == 0 && r.Miss == "" {
		r.State = "empty"
		r.Miss = "no-match"
		if request.Operation == "get" || request.Operation == "journey" || request.Operation == "stability" || request.Operation == "trace" {
			r.Miss = "evidence-not-found"
		}
	}
	if len(r.Results) > request.Limit {
		r.Omitted = len(r.Results) - request.Limit
		r.Results = r.Results[:request.Limit]
		r.Limitations = append(r.Limitations, "result limit withheld records")
	}
	for _, result := range r.Results {
		id := ""
		switch value := result.(type) {
		case Subject:
			id = value.ID
		case Claim:
			id = value.ID
		case Relation:
			id = value.ID
		}
		if detail, ok := a.Details[id]; ok {
			if r.Details == nil {
				r.Details = map[string]RecordDetails{}
			}
			r.Details[id] = detail
		}
	}
	// Citations are deduplicated and independently bounded; truncation is visible.
	sort.Slice(r.Citations, func(i, j int) bool { return anchorKey(r.Citations[i]) < anchorKey(r.Citations[j]) })
	citations := []Anchor{}
	last := ""
	for _, a := range r.Citations {
		k := anchorKey(a)
		if k != last {
			citations = append(citations, a)
			last = k
		}
	}
	if len(citations) > MaxResults {
		r.Limitations = append(r.Limitations, "citation limit withheld anchors; full record retains evidence")
		citations = citations[:MaxResults]
	}
	r.Citations = citations
	if err := attachSymbolAnchors(ctx, &r, symbols); err != nil {
		return Receipt{}, err
	}
	finalizeEnvelope(&r)
	if _, err := Encode(r); err != nil {
		return Receipt{}, err
	}
	return r, nil
}
func anchorKey(a Anchor) string { return a.Revision + ":" + a.Path + ":" + a.SpanSHA256 }
func subjectPath(s Subject, p string) bool {
	for _, a := range s.Evidence.Anchors {
		if a.Path == p || a.Symbol == p {
			return true
		}
	}
	return false
}
func get(a *Artifact, id string, r *Receipt) {
	for _, j := range a.TestKnowledge {
		if j.ID == id {
			addRecord(r, j)
		}
	}
	for _, j := range a.FlowRelations {
		if j.ID == id {
			addRecord(r, j)
		}
	}
	if x := a.RuntimeIndex; x != nil {
		for _, s := range a.StabilityEvidence {
			if s.ID == id || s.TestID == id || s.ContractID == id {
				r.Results = append(r.Results, s)
			}
		}
		if ref, ok := x.IDs[id]; ok {
			addRecord(r, a.indexedRecord(ref))
		}
		for _, i := range x.Claims[id] {
			addRecord(r, a.Claims[i])
		}
		if r.Operation == "trace" {
			for _, i := range x.Edges[id] {
				if a.Relations[i].ID != id {
					addRecord(r, a.Relations[i])
				}
			}
			for _, o := range a.Observations {
				if o.Link.Subject == id {
					addRecord(r, o)
				}
			}
		}
		return
	}
	for _, stability := range a.StabilityEvidence {
		if stability.ID == id || stability.TestID == id || stability.ContractID == id {
			r.Results = append(r.Results, stability)
		}
	}
	for _, s := range a.Subjects {
		if s.ID == id {
			r.Results = append(r.Results, s)
			r.Citations = append(r.Citations, s.Evidence.Anchors...)
		}
	}
	for _, c := range a.Claims {
		if c.ID == id || c.Subject == id {
			r.Results = append(r.Results, c)
			r.Citations = append(r.Citations, c.Evidence.Anchors...)
		}
	}
	if a.Schema == SchemaV2 {
		for _, rel := range a.Relations {
			if rel.ID == id {
				r.Results = append(r.Results, rel)
				r.Citations = append(r.Citations, rel.Evidence.Anchors...)
			}
		}
		for _, j := range a.Journeys {
			if j.ID == id {
				r.Results = append(r.Results, j)
			}
		}
		for _, o := range a.Observations {
			if o.Link.ID == id {
				r.Results = append(r.Results, o)
			}
		}
	}
	if r.Operation != "trace" {
		return
	}
	for _, rel := range a.Relations {
		if (rel.From == id || rel.To == id) && (a.Schema != SchemaV2 || rel.ID != id) {
			r.Results = append(r.Results, rel)
			r.Citations = append(r.Citations, rel.Evidence.Anchors...)
		}
	}
	for _, o := range a.Observations {
		if o.Link.Subject == id {
			r.Results = append(r.Results, o)
			r.Citations = append(r.Citations, o.Link.StepEvidence...)
		}
	}
}
func search(ctx context.Context, a *Artifact, request Request, r *Receipt) error {
	terms := contextindex.EvidenceTerms(request.Query)
	if x := a.RuntimeIndex; x != nil {
		scores := map[int]int{}
		for term := range terms {
			if err := ctx.Err(); err != nil {
				return err
			}
			for _, i := range x.Terms[term] {
				scores[i]++
			}
		}
		for i, s := range a.Subjects {
			if err := ctx.Err(); err != nil {
				return err
			}
			if strings.EqualFold(request.Query, s.Name) {
				scores[i] += len(terms) + 1
			}
		}
		ids := make([]int, 0, len(scores))
		for i := range scores {
			ids = append(ids, i)
		}
		sort.Slice(ids, func(i, j int) bool {
			if scores[ids[i]] != scores[ids[j]] {
				return scores[ids[i]] > scores[ids[j]]
			}
			return a.Subjects[ids[i]].ID < a.Subjects[ids[j]].ID
		})
		for _, i := range ids {
			addRecord(r, a.Subjects[i])
		}
		return ctx.Err()
	}
	type ranked struct {
		s     Subject
		score int
	}
	rankedSubjects := []ranked{}
	claims := map[string][]string{}
	for _, c := range a.Claims {
		if err := ctx.Err(); err != nil {
			return err
		}
		claims[c.Subject] = append(claims[c.Subject], c.Text)
	}
	for _, s := range a.Subjects {
		if err := ctx.Err(); err != nil {
			return err
		}
		text := s.Name + " " + s.ID
		text += " " + strings.Join(claims[s.ID], " ")
		words := contextindex.EvidenceTerms(text)
		score := 0
		for term := range terms {
			if _, ok := words[term]; ok {
				score++
			}
		}
		if strings.EqualFold(request.Query, s.Name) {
			score += len(terms) + 1
		}
		if score > 0 {
			rankedSubjects = append(rankedSubjects, ranked{s, score})
		}
	}
	sort.Slice(rankedSubjects, func(i, j int) bool {
		if rankedSubjects[i].score != rankedSubjects[j].score {
			return rankedSubjects[i].score > rankedSubjects[j].score
		}
		return rankedSubjects[i].s.ID < rankedSubjects[j].s.ID
	})
	for _, item := range rankedSubjects {
		r.Results = append(r.Results, item.s)
		r.Citations = append(r.Citations, item.s.Evidence.Anchors...)
	}
	return ctx.Err()
}

func ReadQuery(ctx context.Context, root string, data []byte, request Request) (Receipt, error) {
	a, err := Open(ctx, root, data)
	if err != nil {
		return Receipt{}, err
	}
	overlay, err := LiveFreshness(ctx, root, a)
	if err != nil {
		return Receipt{}, err
	}
	return QueryLive(ctx, a, request, overlay)
}

func coverageMetrics(a *Artifact) []any {
	claimed, observed := map[string]bool{}, map[string]bool{}
	for _, claim := range a.Claims {
		claimed[claim.Subject] = true
	}
	for _, o := range a.Observations {
		observed[o.Link.Subject] = true
	}
	withClaims, tests, tested, verified := 0, 0, 0, 0
	for _, s := range a.Subjects {
		if claimed[s.ID] {
			withClaims++
		}
		if s.Kind == "test" {
			tests++
			if observed[s.ID] {
				tested++
			}
		}
	}
	for _, j := range a.Journeys {
		if j.Status == "verified" {
			verified++
		}
	}
	rows := []any{}
	for _, metric := range []struct {
		name, definition   string
		value, denominator int
	}{
		{"subjects_with_claims", "subjects with at least one explicit claim / all subjects", withClaims, len(a.Subjects)},
		{"tests_with_observations", "test subjects with exact retained receipt joins / all declared test subjects", tested, tests},
		{"journeys_with_recorded_verification", "journeys with matching ordered retained steps / all declared journey dispositions", verified, len(a.Journeys)},
	} {
		rows = append(rows, map[string]any{"metric": metric.name, "value": metric.value, "denominator": metric.denominator, "defined": metric.denominator > 0, "definition": metric.definition, "revision": a.Manifest.Repository.Revision, "limitations": []string{"scoped declaration coverage only; provider honesty, semantic truth and adequacy unknown"}})
	}
	for _, s := range a.Subjects {
		if d := a.Details[s.ID].Coverage; d != nil {
			rows = append(rows, map[string]any{"metric": s.ID, "value": len(d.Numerator), "denominator": len(d.Denominator), "defined": len(d.Denominator) > 0, "definition": d.Definition, "rule": d.Rule, "revision": a.Manifest.Repository.Revision, "limitations": []string{"supplied membership only; original intent and semantic adequacy unassessed"}})
		}
	}
	rows = append(rows, suppliedStatusCoverage(a)...)
	rows = append(rows, testKnowledgeCoverage(a)...)
	return append(rows, behaviorCoverage(a)...)
}

// suppliedStatusCoverage counts producer-supplied statuses per state
// (DCP-V1-188). It yields no row when no status was supplied, so existing
// coverage receipts keep their bytes.
func suppliedStatusCoverage(a *Artifact) []any {
	counts := map[string]int{}
	total := 0
	for _, claim := range a.Claims {
		if d := a.Details[claim.ID].Status; d != nil {
			counts[d.State]++
			total++
		}
	}
	if total == 0 {
		return nil
	}
	states := make([]string, 0, len(counts))
	for state := range counts {
		states = append(states, state)
	}
	sort.Strings(states)
	rows := []any{}
	for _, state := range states {
		rows = append(rows, map[string]any{"metric": "supplied_status:" + state, "value": counts[state], "denominator": total, "defined": true, "definition": "supplied status claims in this state / all supplied status claims", "origin": "supplied", "revision": a.Manifest.Repository.Revision, "limitations": []string{"producer-supplied status; not derived or verified by the compiler and grants no trust"}})
	}
	return rows
}
