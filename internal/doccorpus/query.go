package doccorpus

import (
	"context"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
)

// Freshness is a live overlay; it never rewrites the immutable artifact.
func Freshness(ctx context.Context, root string, a *Artifact) (string, []string, error) {
	live, err := contextindex.BuildContext(ctx, root, "")
	if err != nil {
		return "unknown", nil, err
	}
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return "unknown", nil, err
	}
	release := auth.BeginObjectSession()
	defer release()
	state := "fresh"
	limits := []string{"structural source binding does not prove semantic truth, test adequacy or accepted intent"}
	if a.Builder.Revision == "unrecorded" || strings.HasSuffix(a.Builder.Revision, "+dirty") {
		limits = append(limits, "builder source revision is unrecorded or dirty; no immutable build attestation")
	}
	declared := map[string]bool{}
	for _, in := range a.Manifest.Inputs {
		declared[inputKey(in.Revision, in.Path)] = true
	}
	for _, scope := range a.Manifest.Scopes {
		for p := range live.Tracked {
			if inScope(p, scope.Path) && !declared[inputKey(scope.Revision, p)] {
				state = "stale"
				limits = append(limits, "new input in declared scope: "+p)
			}
		}
		for p := range live.Skipped {
			if inScope(p, scope.Path) {
				state = "stale"
				limits = append(limits, "unsupported live scope entry: "+p)
			}
		}
		for _, p := range live.DirtyPaths {
			if inScope(p, scope.Path) && !declared[inputKey(scope.Revision, p)] {
				state = "stale"
				limits = append(limits, "new working-tree input in declared scope: "+p)
			}
		}
	}
	dirty := map[string]bool{}
	for _, p := range live.DirtyPaths {
		dirty[p] = true
	}
	for _, in := range a.Manifest.Inputs {
		// Historical provider records are independent of source freshness. A live
		// replacement of one is still visible; no HEAD equality heuristic is used.
		entry, exists, err := auth.LookupTreeEntry(ctx, live.CommitRevision, in.Path)
		if err != nil {
			return "unknown", nil, err
		}
		if !exists || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") || entry.OID != in.Blob {
			state = "stale"
			limits = append(limits, "input changed or absent: "+in.Path)
		}
		if dirty[in.Path] {
			state = "stale"
			limits = append(limits, "working-tree input differs: "+in.Path)
		}
	}
	sort.Strings(limits)
	return state, limits, nil
}
func CapabilityFor(operation string) string {
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
	if request.Limit == 0 {
		request.Limit = 20
	}
	if request.Limit < 1 || request.Limit > MaxResults || len(request.Query) > 1024 || len(request.ID) > 1024 || len(request.Path) > 1024 {
		return Receipt{}, fail("invalid query bounds")
	}
	capability := CapabilityFor(request.Operation)
	if capability == "" {
		return Receipt{}, fail("unknown corpus operation")
	}
	r := Receipt{Schema: ReceiptSchema, Operation: request.Operation, ArtifactSHA256: a.SHA256, Repository: a.Manifest.Repository, Tree: a.Tree, Trust: "generated", Freshness: freshness, State: "ready", Results: []any{}, Citations: []Anchor{}, Capabilities: a.Capabilities, Limitations: append([]string{}, limitations...)}
	if !a.HasCapability(capability) {
		r.State = "unavailable"
		r.Miss = "capability-absent"
		for _, cap := range a.Capabilities {
			if cap.Name == capability {
				r.Limitations = append(r.Limitations, cap.Reason)
			}
		}
		return r, nil
	}
	if freshness == "stale" {
		r.Limitations = append(r.Limitations, "evidence-stale; results describe the pinned revision only")
	}
	switch request.Operation {
	case "info", "validate":
		r.Results = append(r.Results, map[string]any{"builder": a.Builder, "manifest_sha256": a.ManifestSHA256, "profile_sha256": a.ProfileSHA256, "profile": a.Manifest.Profile, "built_at": a.Manifest.BuiltAt, "validation": "source-rederived"})
	case "search":
		search(a, request, &r)
	case "get", "trace":
		get(a, request.ID, &r)
	case "locate":
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
	case "gaps":
		for _, g := range a.Gaps {
			if request.ID == "" || g.Subject == request.ID {
				r.Results = append(r.Results, g)
			}
		}
	case "coverage":
		r.Results = append(r.Results, coverageMetrics(a)...)
		for _, cap := range a.Capabilities {
			r.Results = append(r.Results, map[string]any{"capability": cap.Name, "value": cap.Count, "denominator": cap.Denominator, "definition": "emitted records per declared input; not a behavioral coverage percentage", "rule": cap.Rule, "revision": a.Manifest.Repository.Revision, "defined": cap.Denominator > 0, "limitations": []string{cap.Reason}})
		}
	}
	if len(r.Results) == 0 {
		r.State = "empty"
		r.Miss = "no-match"
		if request.Operation == "get" || request.Operation == "journey" || request.Operation == "trace" {
			r.Miss = "evidence-not-found"
		}
	}
	if len(r.Results) > request.Limit {
		r.Omitted = len(r.Results) - request.Limit
		r.Results = r.Results[:request.Limit]
		r.Limitations = append(r.Limitations, "result limit withheld records")
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
	if r.Operation != "trace" {
		return
	}
	for _, rel := range a.Relations {
		if rel.From == id || rel.To == id {
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
func search(a *Artifact, request Request, r *Receipt) {
	terms := contextindex.EvidenceTerms(request.Query)
	type ranked struct {
		s     Subject
		score int
	}
	rankedSubjects := []ranked{}
	claims := map[string][]string{}
	for _, c := range a.Claims {
		claims[c.Subject] = append(claims[c.Subject], c.Text)
	}
	for _, s := range a.Subjects {
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
}

func ReadQuery(ctx context.Context, root string, data []byte, request Request) (Receipt, error) {
	a, err := Open(ctx, root, data)
	if err != nil {
		return Receipt{}, err
	}
	state, limits, err := Freshness(ctx, root, a)
	if err != nil {
		return Receipt{}, err
	}
	return Query(a, request, state, limits)
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
	return append(rows, behaviorCoverage(a)...)
}
