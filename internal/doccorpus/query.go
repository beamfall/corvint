package doccorpus

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"

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
	drift       *anchorDrift
	observation string
	// root is the primary checkout the overlay read, where a caller revision's
	// ancestry is observed (DCP-V1-332).
	root string
	// inputs is each declared compiled input's live as-of state (DCP-V1-576).
	inputs []InputFreshness
	// repositories is each declared checkout's own state, fresh or stale, by root-commit
	// identity (DCP-V1-701).
	repositories map[string]string
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

// CorpusObservation observes every checkout the artifact's live freshness reads, the primary
// root and each declared additional repository, concurrently (DCP-V1-280). It equals
// ObservationKey of the primary observation for a single-repository corpus, and an overlay's
// Observation is reusable exactly while CorpusObservation still returns it.
func CorpusObservation(ctx context.Context, root string, a *Artifact) (string, error) {
	dirs := []string{root}
	for _, r := range a.Manifest.AdditionalRepositories {
		dirs = append(dirs, a.Manifest.RevisionRoot(root, r.Revision))
	}
	keys, errs := make([]string, len(dirs)), make([]error, len(dirs))
	var wg sync.WaitGroup
	for i, dir := range dirs {
		wg.Go(func() {
			observed, err := contextindex.Observe(ctx, dir)
			keys[i], errs[i] = ObservationKey(observed), err
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return "", err
		}
	}
	return withProbeKey(checkoutsKey(a, keys), probeObservationKey(root, a)), nil
}

func indexObservationKey(live *contextindex.TreeObservation) string {
	return ObservationKey(live.Observation)
}

// checkoutsKey joins the primary key and each additional repository's identity and key,
// length-prefixed so no path byte can make two checkout sets collide.
func checkoutsKey(a *Artifact, keys []string) string {
	if len(keys) == 1 {
		return keys[0]
	}
	var b strings.Builder
	for i, key := range keys {
		if i > 0 {
			key = a.Manifest.AdditionalRepositories[i-1].ID + "\x00" + key
		}
		fmt.Fprintf(&b, "%d:%s;", len(key), key)
	}
	return b.String()
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
	// The probe key is read before any probe, so a change during the check only
	// forces a later rebuild (DCP-V1-577).
	probes := probeObservationKey(root, a)
	// Every declared checkout is observed in parallel; each additional repository's inputs are
	// checked against its own declared checkout, never the primary tree (DCP-V1-178), and that
	// checkout's own observation joins the overlay's reuse key (DCP-V1-280, DCR-V1-411).
	type checkout struct {
		dir, prefix string
		member      func(string) bool
		live        *contextindex.TreeObservation
		stale       bool
		limits      []string
		drift       *anchorDrift
		err         error
	}
	checkouts := []*checkout{{dir: root, member: func(revision string) bool { _, ok := a.Manifest.additionalRepository(revision); return !ok }}}
	for _, r := range a.Manifest.AdditionalRepositories {
		checkouts = append(checkouts, &checkout{dir: a.Manifest.RevisionRoot(root, r.Revision), prefix: "additional repository " + r.ID + ": ", member: func(revision string) bool { return revision == r.Revision }})
	}
	var wg sync.WaitGroup
	for _, c := range checkouts {
		c.drift = &anchorDrift{pinned: map[string]string{}, changed: map[string]inputDrift{}}
		wg.Go(func() { c.live, c.stale, c.limits, c.err = liveRepository(ctx, c.dir, a, c.member, c.prefix, c.drift) })
	}
	wg.Wait()
	drift := &anchorDrift{pinned: map[string]string{}, changed: map[string]inputDrift{}}
	stale := false
	limits := []string{}
	keys := []string{}
	repositories := map[string]string{}
	for i, c := range checkouts {
		if c.err != nil {
			return unknown, c.err
		}
		stale = stale || c.stale
		id := a.Manifest.Repository.ID
		if i > 0 {
			id = a.Manifest.AdditionalRepositories[i-1].ID
		}
		repositories[id] = "fresh"
		if c.stale {
			repositories[id] = "stale"
		}
		limits = append(limits, c.limits...)
		if i == 0 {
			limits = append(limits, "structural source binding does not prove semantic truth, test adequacy or accepted intent")
			if !attestedBuilder(a.Builder) {
				limits = append(limits, "builder source revision is unrecorded, dirty or not a Git object ID; no immutable build attestation")
			}
		}
		maps.Copy(drift.pinned, c.drift.pinned)
		maps.Copy(drift.changed, c.drift.changed)
		keys = append(keys, indexObservationKey(c.live))
	}
	live := checkouts[0].live
	observation := withProbeKey(checkoutsKey(a, keys), probes)
	state := "fresh"
	if stale {
		state = "stale"
	}
	inputs, err := probeInputFreshness(ctx, root, a)
	if err != nil {
		return unknown, err
	}
	state = foldInputFreshness(state, inputs)
	limits = append(limits, inputAsOfLimitations(inputs)...)
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
	overlay := FreshnessOverlay{State: state, Limitations: limits, drift: drift, observation: observation, root: root, inputs: inputs, repositories: repositories}
	if symbols {
		overlay.symbols = &symbolOverlay{root: root, live: live, observation: keys[0], repository: a.Manifest.Repository.ID, files: &sharedSymbolFiles{files: map[string]sharedSymbolFile{}}}
	}
	return overlay, nil
}

// liveRepository compares the scopes and inputs whose revision member admits against the live
// checkout at dir, returning whether any is stale and each reason under prefix. It records each
// admitted input's pinned blob, and its live drift when it differs, in drift (DCP-V1-282). The
// live checkout is observed from Git's identity, its stat-cached status and one tree listing;
// no tracked file is read or indexed (DCR-V1-411).
func liveRepository(ctx context.Context, dir string, a *Artifact, member func(string) bool, prefix string, drift *anchorDrift) (*contextindex.TreeObservation, bool, []string, error) {
	live, err := observeTree(ctx, dir)
	if err != nil {
		return nil, false, nil, err
	}
	auth, err := gitauth.Open(dir, newGitBudget(ctx))
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
	// An over-bound input is declared too; its change is noted, though no anchor cites it
	// (DCI-V1-024).
	overBound := []OverBoundInput{}
	for _, in := range a.Manifest.OverBoundInputs {
		if member(in.Revision) {
			declared[inputKey(in.Revision, in.Path)] = true
			overBound = append(overBound, in)
		}
	}
	// Each live path is noted once per admitted scope holding it, as a scope-by-path pass did;
	// the reasons are sorted by the caller, so the visiting order is not observable (DCR-V1-450).
	scopes := newScopeSet(a.Manifest.Scopes, member)
	for rev := range scopes {
		for p := range live.Tracked {
			if scopes.holds(rev, p) && !declared[inputKey(rev, p)] {
				scopes.each(rev, p, func(string) { note(livePath("new input in declared scope: ", p)) })
			}
		}
		for p := range live.Skipped {
			scopes.each(rev, p, func(string) { note(livePath("unsupported live scope entry: ", p)) })
		}
		for _, p := range live.DirtyPaths {
			if scopes.holds(rev, p) && !declared[inputKey(rev, p)] {
				scopes.each(rev, p, func(string) { note(livePath("new working-tree input in declared scope: ", p)) })
			}
		}
	}
	dirty := map[string]bool{}
	for _, p := range live.DirtyPaths {
		dirty[p] = true
	}
	// A path the live tree listing holds needs no further tree walk (DCP-V1-144,
	// DCR-V1-411); every other path resolves in one verified walk whose Git cost
	// is independent of the input count (DCP-V1-155).
	lookups := []string{}
	for _, in := range inputs {
		if _, ok := live.Blobs[in.Path]; !ok {
			lookups = append(lookups, in.Path)
		}
	}
	for _, in := range overBound {
		if _, ok := live.Blobs[in.Path]; !ok {
			lookups = append(lookups, in.Path)
		}
	}
	resolved := map[string]gitauth.TreeEntry{}
	if len(lookups) > 0 {
		if resolved, err = resolvePathsPerPathBound(ctx, auth, live.CommitRevision, lookups); err != nil {
			return nil, false, nil, err
		}
	}
	for _, in := range inputs {
		// Historical provider records are independent of source freshness. A live
		// replacement of one is still visible; no HEAD equality heuristic is used.
		entry, exists := resolved[in.Path]
		if blob, ok := live.Blobs[in.Path]; ok {
			entry, exists = gitauth.TreeEntry{Type: "blob", Mode: blob.Mode, OID: blob.OID}, true
		}
		regular := exists && entry.Type == "blob" && (entry.Mode == "100644" || entry.Mode == "100755")
		if !regular || entry.OID != in.Blob {
			note("input changed or absent: " + in.Path)
		}
		if dirty[in.Path] {
			note("working-tree input differs: " + in.Path)
		}
		key := inputKey(in.Revision, in.Path)
		drift.pinned[key] = in.Blob
		if !regular || entry.OID != in.Blob || dirty[in.Path] {
			drift.changed[key] = inputDrift{dir: dir, oid: entry.OID, objectFormat: live.ObjectFormat, dirty: dirty[in.Path], exists: regular}
		}
	}
	for _, in := range overBound {
		entry, exists := resolved[in.Path]
		if blob, ok := live.Blobs[in.Path]; ok {
			entry, exists = gitauth.TreeEntry{Type: "blob", Mode: blob.Mode, OID: blob.OID}, true
		}
		if !exists || entry.OID != in.Blob || dirty[in.Path] {
			note("over-bound input changed or absent: " + in.Path)
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
	return queryContext(ctx, a, request, freshness, limitations, nil, nil, nil)
}

// QueryLive answers request under a live freshness overlay. Besides the
// overlay's state and limitations, the receipt reports the re-resolution of
// every symbol anchor it cites (DCP-V1-141). Only the anchors the receipt
// cites are re-resolved, on demand and under one budget per call, so the
// receipt is the one a fresh reader returns (DCP-V1-144).
func QueryLive(ctx context.Context, a *Artifact, request Request, overlay FreshnessOverlay) (Receipt, error) {
	checker := &symbolChecker{root: overlay.root, overlay: overlay.symbols, checks: map[string]SymbolAnchorCheck{}, anchors: &anchorStates{drift: overlay.drift, states: map[string]string{}, files: map[string][]byte{}}}
	defer checker.close()
	return queryContext(ctx, a, request, overlay.State, overlay.Limitations, checker, overlay.inputs, overlay.repositories)
}

// queryContext answers request. inputs is the live as-of state of each declared
// compiled input; a reader without it reports each one unknown, with its
// limitation, and never fresh (DCP-V1-576). repositories is the live state of each declared
// checkout; without it each covered repository is listed unknown (DCP-V1-701).
func queryContext(ctx context.Context, a *Artifact, request Request, freshness string, limitations []string, symbols *symbolChecker, inputs []InputFreshness, repositories map[string]string) (Receipt, error) {
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	if len(inputs) == 0 {
		if inputs = staticInputFreshness(a); len(inputs) > 0 {
			freshness = foldInputFreshness(freshness, inputs)
			limitations = append(append([]string{}, limitations...), inputAsOfLimitations(inputs)...)
		}
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
	// DCP-V1-328: search admits exactly include:["claims"]; flow keeps its filters.
	if (request.Sections != nil || request.Include != nil) && !SearchIncludesClaims(request) && (request.Operation != "flow" || !ValidFlowFilter(request.Sections, request.Include)) {
		return Receipt{}, fail("flow section and include filters are bounded and apply only to flow")
	}
	// DCP-V1-650: locate by query, without a path, ranks module and symbol
	// subjects only; a kind filter there narrows to one of them. A path keeps
	// the path form, as permissive pure callers expect.
	locateQuery := request.Operation == "locate" && request.Query != "" && request.Path == ""
	if locateQuery {
		if request.Repository != "" {
			return Receipt{}, fail("a locate query takes no repository qualifier")
		}
		if request.Kinds == nil {
			request.Kinds = append([]string{}, LocateQueryKinds...)
		}
		for _, k := range request.Kinds {
			if !slices.Contains(LocateQueryKinds, k) {
				return Receipt{}, fail("a locate query kind filter names only module or symbol")
			}
		}
	}
	// DCP-V1-331: search also admits a subject kind filter.
	// DCS-V1-592: claims admits a claim kind filter and exclusion, which
	// never name one kind twice.
	if request.Operation == "claims" {
		if !ValidClaimKinds(request.Kinds) || !ValidClaimKinds(request.ExcludeKinds) || slices.ContainsFunc(request.Kinds, func(k string) bool { return slices.Contains(request.ExcludeKinds, k) }) {
			return Receipt{}, fail("claim kind filters name distinct known claim kinds")
		}
	} else if request.ExcludeKinds != nil {
		return Receipt{}, fail("a kind exclusion applies only to claims")
	} else if request.Kinds != nil && (request.Operation != "gaps" && request.Operation != "search" && !locateQuery || !ValidGapKinds(request.Kinds)) {
		return Receipt{}, fail("kind filter is bounded and applies only to gaps and search")
	}
	// DCS-V1-596: navigation takes an id or a query, not both.
	if request.Operation == "navigation" && request.ID != "" && request.Query != "" {
		return Receipt{}, fail("navigation takes an id or a query, not both")
	}
	if (request.State != "" || request.Qualifier != "" || request.Run != "") && (request.Operation != "claims" || request.Query != "" || request.Path != "" || !ValidStatusFilter(request.State, request.Qualifier, request.Run)) {
		return Receipt{}, fail("status filter is bounded and applies only to claims without a path or query")
	}
	if request.Offset < 0 || request.Offset > MaxCorpusRecords*4 || a.Schema != SchemaV2 && (request.Offset != 0 || request.Operation == "inventory") {
		return Receipt{}, fail("pagination requires corpus /2 and bounded offset")
	}
	if !ValidCallerRevision(request.CallerRevision) {
		return Receipt{}, fail("caller revision must be a full lowercase hex Git object ID")
	}
	capability := CapabilityFor(request.Operation)
	if capability == "" {
		return Receipt{}, fail("unknown corpus operation")
	}
	// DCP-V1-602: a declared alias names its repository's root-commit identity.
	request.Repository = a.Manifest.repositoryOfAlias(request.Repository)
	pathForm := (request.Operation == "trace" || request.Operation == "gaps") && request.Path != ""
	if request.Repository != "" && (request.Operation != "locate" && request.Operation != "recommend-tests" && !pathForm || !a.Manifest.declaresRepository(request.Repository)) {
		return Receipt{}, fail("repository qualifier applies only to locate, recommend-tests and a trace or gaps path and must name a declared repository")
	}
	var resolution *PathResolution
	if request.Path != "" && (request.Operation == "trace" || request.Operation == "gaps") {
		if request.ID != "" {
			return Receipt{}, fail("trace and gaps take an id or a path, not both")
		}
		id, err := a.Manifest.nativeFileID(request.Repository, request.Path)
		if err != nil {
			return Receipt{}, err
		}
		repository := request.Repository
		if repository == "" {
			repository = a.Manifest.Repository.ID
		}
		resolution = &PathResolution{Repository: repository, Path: request.Path, ID: id}
		request.ID = id
	}
	// DCP-V1-332: only a live reader's checkout can observe the caller revision's ancestry.
	if request.CallerRevision != "" {
		root := ""
		if symbols != nil {
			root = symbols.root
		}
		if envelope.CallerRevision, err = relateCallerRevision(ctx, root, a.Manifest.Repository.Revision, request.CallerRevision); err != nil {
			return Receipt{}, err
		}
	}
	r := Receipt{Envelope: envelope, Schema: ReceiptSchema, Operation: request.Operation, ArtifactSHA256: a.SHA256, Repository: a.Manifest.Repository, Tree: a.Tree, Trust: "generated", Freshness: freshness, State: "ready", Results: []any{}, Citations: []Anchor{}, Capabilities: a.Capabilities, Limitations: append([]string{}, limitations...)}
	r.PathResolution = resolution
	if a.Schema == SchemaV2 {
		r.Capabilities = boundedCapabilities(a.Capabilities)
	}
	// DCP-V1-650: a locate query ranks subjects, which test knowledge cannot answer.
	if !a.HasCapability(capability) && testKnowledgeServes(a, request.Operation) && !locateQuery {
		r.Limitations = append(r.Limitations, capability+" capability absent; answered from test-knowledge records only")
	} else if !a.HasCapability(capability) {
		r.State = "unavailable"
		r.Miss = "capability-absent"
		for _, cap := range a.Capabilities {
			if cap.Name == capability {
				r.Limitations = append(r.Limitations, cap.Reason)
			}
		}
		finalizeEnvelope(&r, symbols)
		return r, nil
	}
	if freshness == "stale" {
		r.Limitations = append(r.Limitations, "evidence-stale; results describe the pinned revision only")
	}
	// DCP-V1-330: an id that names no record but exactly one subject's
	// alternate id resolves to that subject, and the receipt records it. A
	// record get finds by the id, including a join, stability or declared
	// legacy-case record, always wins over an alias.
	if request.ID != "" && (OperationInput(request.Operation) == "id" || request.Operation == "gaps") {
		if id, ok := resolveAlternateID(a, request.ID); ok && !getFindsRecord(a, request.ID) {
			r.AliasResolution = &AliasResolution{AlternateID: request.ID, ID: id}
			request.ID = id
		}
	}
	switch request.Operation {
	case "info", "validate":
		info := map[string]any{"builder": a.Builder, "manifest_sha256": a.ManifestSHA256, "profile_sha256": a.ProfileSHA256, "profile": a.Manifest.Profile, "built_at": a.Manifest.BuiltAt, "validation": validationOf(a)}
		// DCP-V1-578: each declared compiled input's as-of point and live lag.
		if len(inputs) > 0 {
			info["inputs_as_of"] = inputs
		}
		// DCP-V1-701: every covered repository, when the corpus spans or inventories repositories.
		if len(a.Manifest.AdditionalRepositories) > 0 || a.Manifest.RepositoryInventory != nil {
			info["repositories"] = repositoryListing(a, repositories)
		}
		r.Results = append(r.Results, info)
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
		for _, x := range a.ClaimAudits {
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
		if locateQuery {
			if e := search(ctx, a, request, &r); e != nil {
				return Receipt{}, e
			}
			break
		}
		// DCP-V1-310: a repository qualifier keeps only records anchored at the path there.
		for _, j := range testKnowledgeOnPath(a, request.Path) {
			if pathInRepository(recordAnchors(j), request.Path, request.Repository) {
				addRecord(&r, j)
			}
		}
		anchored := []string{}
		if a.RuntimeIndex != nil {
			for _, ref := range a.RuntimeIndex.Paths[request.Path] {
				if ref.Kind == "subject" && pathInRepository(a.Subjects[ref.Offset].Evidence.Anchors, request.Path, request.Repository) {
					addRecord(&r, a.indexedRecord(ref))
					anchored = append(anchored, a.Subjects[ref.Offset].ID)
				}
			}
		} else {
			for _, s := range a.Subjects {
				if subjectPath(s, request.Path) && pathInRepository(s.Evidence.Anchors, request.Path, request.Repository) {
					r.Results = append(r.Results, s)
					r.Citations = append(r.Citations, s.Evidence.Anchors...)
					anchored = append(anchored, s.ID)
				}
			}
		}
		// DCS-V1-594: and the subjects documenting them, one hop, each with
		// its documents relation.
		have := map[string]bool{}
		for _, v := range r.Results {
			have[recordID(v)] = true
		}
		if _, e := addDocumentingOwners(ctx, a, anchored, have, &r); e != nil {
			return Receipt{}, e
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
		// DCP-V1-284: a kind filter keeps only the named kinds and totals each, before paging.
		kind := func(k string) bool {
			if request.Kinds == nil {
				return true
			}
			if _, ok := r.KindTotals[k]; !ok {
				return false
			}
			r.KindTotals[k]++
			return true
		}
		if request.Kinds != nil {
			r.KindTotals = map[string]int{}
			for _, k := range request.Kinds {
				r.KindTotals[k] = 0
			}
		}
		for _, g := range append(append([]Gap{}, a.Gaps...), generated...) {
			if err := ctx.Err(); err != nil {
				return Receipt{}, err
			}
			if (request.ID == "" || g.Subject == request.ID) && kind(g.Kind) {
				r.Results = append(r.Results, g)
			}
		}
		// DCP-V1-721, DCP-V1-722: module ownership gaps follow, under a declared production
		// scope only; an undeclared scope is reported, never counted as everything (DCP-V1-720).
		if a.Manifest.ProductionScope != nil {
			ownership, _, _, _, e := moduleOwnership(ctx, a)
			if e != nil {
				return Receipt{}, e
			}
			for _, g := range ownership {
				if (request.ID == "" || g.Subject == request.ID) && kind(g.Kind) {
					r.Results = append(r.Results, g)
				}
			}
		} else if hasFlowSubjects(a) || slices.Contains(request.Kinds, ModuleWithoutFlow) || slices.Contains(request.Kinds, ModuleClaimsWithoutFlow) {
			r.Limitations = append(r.Limitations, productionScopeUndeclared)
		}
		// DCP-V1-186: supplied gaps follow the computed ones, labelled by origin.
		// DCP-V1-304: a gap that targets a claim is also selected by the
		// claim's subject.
		targets := suppliedGapClaimSubjects(a, request.ID)
		for _, g := range a.SuppliedGaps {
			if (request.ID == "" || g.Subject == request.ID || g.ID == request.ID || targets[g.Subject]) && kind(g.Kind) {
				r.Results = append(r.Results, g)
				r.Citations = append(r.Citations, g.Evidence.Anchors...)
			}
		}
		// DCP-V1-327: supplied coverage-ledger, flow-completeness and
		// unresolved-concept claims follow, selected by subject and by their
		// detail kind; they are claims, not gaps.
		for _, c := range a.Claims {
			if len(a.Details) == 0 {
				break
			}
			k := claimParityKind(a.Details[c.ID])
			if k != "coverage_ledger" && k != "flow_completeness" && k != "unresolved_concept" {
				continue
			}
			if (request.ID == "" || c.Subject == request.ID || c.ID == request.ID) && kind(k) {
				r.Results = append(r.Results, c)
				r.Citations = append(r.Citations, c.Evidence.Anchors...)
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
		// DCP-V1-723: module-to-flow ownership under the declared production scope.
		if row, e := moduleOwnershipMetric(ctx, a); e != nil {
			return Receipt{}, e
		} else if row != nil {
			r.Results = append(r.Results, row)
		}
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
	attachClaimAudits(a, &r)
	annotateVisibleResults(a, request, &r)
	attachClaimMentions(a, &r)
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
	nameResultRepositories(a, request, &r)
	finalizeEnvelope(&r, symbols)
	if _, err := Encode(r); err != nil {
		return Receipt{}, err
	}
	return r, nil
}

// declaresRepository reports whether id is the primary or a declared additional repository.
func (m Manifest) declaresRepository(id string) bool {
	if id == m.Repository.ID {
		return true
	}
	for _, r := range m.AdditionalRepositories {
		if r.ID == id {
			return true
		}
	}
	return false
}

// repositoryOfAlias returns the root-commit identity of the additional repository declared under
// alias, or value unchanged when no repository has that alias (DCP-V1-602).
func (m Manifest) repositoryOfAlias(value string) string {
	for _, r := range m.AdditionalRepositories {
		if r.Alias != "" && r.Alias == value {
			return r.ID
		}
	}
	return value
}

// nativeFileID is the ID of the native file subject for path in repository (the primary when
// empty): the native provider's ID, ":file:", the repository's alias qualifier and the path
// (DCP-V1-601, DCP-V1-602). An additional repository without an alias holds no native inputs.
func (m Manifest) nativeFileID(repository, p string) (string, error) {
	native := ""
	for _, provider := range m.Providers {
		if provider.Kind == "native" {
			native = provider.ID
			break
		}
	}
	if native == "" {
		return "", fail("a trace or gaps path needs a native provider")
	}
	if repository == "" || repository == m.Repository.ID {
		return native + ":file:" + p, nil
	}
	for _, r := range m.AdditionalRepositories {
		if r.ID == repository && r.Alias != "" {
			return native + ":file:" + r.Alias + ":" + p, nil
		}
	}
	return "", fail("a trace or gaps path in an additional repository needs its workspace alias")
}

// pathInRepository reports whether an anchor at path (or symbol) belongs to repository; an empty
// repository admits every record (DCP-V1-310).
func pathInRepository(anchors []Anchor, path, repository string) bool {
	if repository == "" {
		return true
	}
	for _, a := range anchors {
		if (a.Path == path || a.Symbol == path) && a.Repository == repository {
			return true
		}
	}
	return false
}

// resultRepository is the repository of the record's anchor at path, else of its first anchor,
// else the primary repository.
func resultRepository(a *Artifact, anchors []Anchor, path string) string {
	for _, x := range anchors {
		if path != "" && (x.Path == path || x.Symbol == path) {
			return x.Repository
		}
	}
	if len(anchors) > 0 {
		return anchors[0].Repository
	}
	return a.Manifest.Repository.ID
}

// nameResultRepositories names each visible locate or recommend-tests result's repository when
// the artifact spans more than one repository (DCP-V1-311); single-repository receipts are
// unchanged.
func nameResultRepositories(a *Artifact, request Request, r *Receipt) {
	if len(a.Manifest.AdditionalRepositories) == 0 || request.Operation != "locate" && request.Operation != "recommend-tests" {
		return
	}
	for _, v := range r.Results {
		id := recordID(v)
		if id == "" {
			continue
		}
		if r.ResultRepositories == nil {
			r.ResultRepositories = map[string]string{}
		}
		r.ResultRepositories[id] = resultRepository(a, recordAnchors(v), request.Path)
	}
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

// getFindsRecord reports whether get resolves id to a corpus record; the
// legacy-case-unlinked gap get reports for an undeclared case is not one.
func getFindsRecord(a *Artifact, id string) bool {
	probe := Receipt{Operation: "get"}
	get(a, id, &probe)
	for _, v := range probe.Results {
		if g, ok := v.(Gap); !ok || g.Kind != "legacy-case-unlinked" {
			return true
		}
	}
	return false
}

func get(a *Artifact, id string, r *Receipt) {
	before := len(r.Results)
	getRecords(a, id, r)
	if len(r.Results) == before {
		getLegacyCase(a, id, r)
		return
	}
	getMigrationChain(a, id, r)
}

// getMigrationChain appends, in artifact order, every migration_mapping
// relation reachable from an inventory unit through migration mappings in
// either direction (DCP-V1-307): the unit's own mapping and the chain across
// earlier and later migrations. Relations the receipt already holds are not
// repeated. A corpus without migration mappings reads nothing more.
func getMigrationChain(a *Artifact, id string, r *Receipt) {
	var edges func(node string) []int
	if x := a.RuntimeIndex; x != nil {
		if ref, ok := x.IDs[id]; !ok || ref.Kind != "subject" || a.Subjects[ref.Offset].Kind != "inventory" {
			return
		}
		edges = func(node string) []int { return x.Edges[node] }
	} else {
		adjacent := map[string][]int{}
		for i, rel := range a.Relations {
			if rel.Type == "migration_mapping" {
				adjacent[rel.From] = append(adjacent[rel.From], i)
				if rel.To != rel.From {
					adjacent[rel.To] = append(adjacent[rel.To], i)
				}
			}
		}
		if len(adjacent[id]) == 0 {
			return
		}
		edges = func(node string) []int { return adjacent[node] }
	}
	held := map[string]bool{}
	for _, v := range r.Results {
		if rel, ok := v.(Relation); ok {
			held[rel.ID] = true
		}
	}
	seen := map[string]bool{id: true}
	chain := map[int]bool{}
	for queue := []string{id}; len(queue) > 0; queue = queue[1:] {
		for _, i := range edges(queue[0]) {
			rel := a.Relations[i]
			if rel.Type != "migration_mapping" || chain[i] {
				continue
			}
			chain[i] = true
			for _, next := range []string{rel.From, rel.To} {
				if !seen[next] {
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}
	}
	offsets := make([]int, 0, len(chain))
	for i := range chain {
		offsets = append(offsets, i)
	}
	sort.Ints(offsets)
	for _, i := range offsets {
		if !held[a.Relations[i].ID] {
			addRecord(r, a.Relations[i])
		}
	}
}

// getLegacyCase resolves a test-management case ID that names no corpus
// record to the manual_case subject declaring it and that subject's
// manual_case_for links (DCP-V1-213). When no subject declares the case, the
// receipt carries a legacy-case-unlinked gap naming why: the external export
// is never corpus evidence, so only a declared link joins it to a subject.
func getLegacyCase(a *Artifact, id string, r *Receipt) {
	if canonical, ok := legacyCaseID(id); !ok || canonical != id {
		return
	}
	for _, s := range a.Subjects {
		if d := a.Details[s.ID].LegacyCase; s.Kind != "manual_case" || d == nil || d.ID != id {
			continue
		}
		addRecord(r, s)
		for _, rel := range a.Relations {
			if rel.Type == "manual_case_for" && rel.From == s.ID {
				addRecord(r, rel)
			}
		}
		return
	}
	r.Results = append(r.Results, Gap{Subject: id, Kind: "legacy-case-unlinked", Reason: "no manual_case subject in this corpus declares legacy case " + id + " in a legacy_case detail; the external case export is not corpus evidence, so the case links to no corpus subject"})
}

func getRecords(a *Artifact, id string, r *Receipt) {
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
	for _, j := range a.ClaimAudits {
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
	// DCP-V1-214: a supplied gap is retrievable by its ID, as through the index.
	for _, g := range a.SuppliedGaps {
		if g.ID == id {
			addRecord(r, g)
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
		// A supplied test-run receipt (DCP-V1-211) or coverage detail
		// (DCP-V1-327) changes no other coverage metric.
		if d := a.Details[claim.ID]; d.TestRun == nil && d.CoverageLedger == nil && d.FlowCompleteness == nil && d.UnresolvedConcept == nil {
			claimed[claim.Subject] = true
		}
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
	rows = append(rows, suppliedDetailCoverage(a)...)
	rows = append(rows, testKnowledgeCoverage(a)...)
	rows = append(rows, claimMentionCoverage(a)...)
	return append(rows, behaviorCoverage(a)...)
}

// suppliedGapClaimSubjects returns the claims on subject id that a supplied
// gap targets (DCP-V1-304). It reads nothing when no supplied gap exists, so
// a corpus without supplied gaps does no extra work.
func suppliedGapClaimSubjects(a *Artifact, id string) map[string]bool {
	if id == "" || len(a.SuppliedGaps) == 0 {
		return nil
	}
	targeted := map[string]bool{}
	for _, g := range a.SuppliedGaps {
		targeted[g.Subject] = true
	}
	claims := map[string]bool{}
	if x := a.RuntimeIndex; x != nil {
		for _, i := range x.Claims[id] {
			if c := a.Claims[i]; c.Subject == id && targeted[c.ID] {
				claims[c.ID] = true
			}
		}
		return claims
	}
	for _, c := range a.Claims {
		if c.Subject == id && targeted[c.ID] {
			claims[c.ID] = true
		}
	}
	return claims
}

// suppliedStatusCoverage counts producer-supplied statuses per state
// (DCP-V1-188) and supplied test-run receipts per outcome (DCP-V1-211). A
// qualified state counts under STATE:QUALIFIER, never under the plain state
// (DCP-V1-301), and test-run receipts that carry a flake verdict or cleanup
// outcome are counted per value over the receipts that carry it (DCP-V1-305).
// Each family yields no row when nothing was supplied, so existing coverage
// receipts keep their bytes.
func suppliedStatusCoverage(a *Artifact) []any {
	rows := suppliedClaimCoverage(a, "supplied_status:", "supplied status claims in this state / all supplied status claims",
		"producer-supplied status; not derived or verified by the compiler and grants no trust",
		func(d RecordDetails) string {
			if d.Status == nil {
				return ""
			}
			if d.Status.Qualifier != "" {
				return d.Status.State + ":" + d.Status.Qualifier
			}
			return d.Status.State
		})
	rows = append(rows, suppliedClaimCoverage(a, "supplied_test_run:", "supplied test-run receipts with this outcome / all supplied test-run receipts",
		"producer-supplied test-run receipt; not executed, derived or verified by the compiler and grants no trust",
		func(d RecordDetails) string {
			if d.TestRun != nil {
				return d.TestRun.Outcome
			}
			return ""
		})...)
	rows = append(rows, suppliedClaimCoverage(a, "supplied_test_run_flake:", "supplied test-run receipts with this flake verdict / supplied test-run receipts that carry a flake verdict",
		"producer-supplied flake verdict; not re-run, derived or verified by the compiler and grants no trust",
		func(d RecordDetails) string {
			if d.TestRun != nil {
				return d.TestRun.FlakeVerdict
			}
			return ""
		})...)
	return append(rows, suppliedClaimCoverage(a, "supplied_test_run_cleanup:", "supplied test-run receipts with this cleanup outcome / supplied test-run receipts that carry a cleanup outcome",
		"producer-supplied cleanup outcome; not observed, derived or verified by the compiler and grants no trust",
		func(d RecordDetails) string {
			if d.TestRun != nil {
				return d.TestRun.Cleanup
			}
			return ""
		})...)
}

// suppliedDetailCoverage counts the supplied coverage-ledger statuses and
// flow completeness flags (DCP-V1-327). Each row is supplied, never derived,
// and grants no trust; a true flag is not complete coverage.
func suppliedDetailCoverage(a *Artifact) []any {
	flag := func(f CompletenessFlag) string {
		if f.Value != nil && *f.Value {
			return "true"
		}
		return "false"
	}
	limitation := "producer-supplied coverage detail; not derived or verified by the compiler and grants no trust"
	rows := suppliedClaimCoverage(a, "supplied_coverage_ledger:", "supplied coverage-ledger claims with this status / all supplied coverage-ledger claims", limitation,
		func(d RecordDetails) string {
			if d.CoverageLedger != nil {
				return d.CoverageLedger.LedgerStatus
			}
			return ""
		})
	rows = append(rows, suppliedClaimCoverage(a, "supplied_flow_coverage_complete:", "supplied flow completeness claims with this coverage flag / all supplied flow completeness claims", limitation,
		func(d RecordDetails) string {
			if d.FlowCompleteness != nil {
				return flag(d.FlowCompleteness.CoverageComplete)
			}
			return ""
		})...)
	rows = append(rows, suppliedClaimCoverage(a, "supplied_flow_audit_complete:", "supplied flow completeness claims with this audit flag / all supplied flow completeness claims", limitation,
		func(d RecordDetails) string {
			if d.FlowCompleteness != nil {
				return flag(d.FlowCompleteness.AuditComplete)
			}
			return ""
		})...)
	return append(rows, suppliedClaimCoverage(a, "supplied_unresolved_concepts:", "supplied unresolved concepts / all supplied unresolved concepts", limitation,
		func(d RecordDetails) string {
			if d.UnresolvedConcept != nil {
				return "unplaced"
			}
			return ""
		})...)
}

func suppliedClaimCoverage(a *Artifact, prefix, definition, limitation string, value func(RecordDetails) string) []any {
	counts := map[string]int{}
	total := 0
	for _, claim := range a.Claims {
		if v := value(a.Details[claim.ID]); v != "" {
			counts[v]++
			total++
		}
	}
	if total == 0 {
		return nil
	}
	values := make([]string, 0, len(counts))
	for v := range counts {
		values = append(values, v)
	}
	sort.Strings(values)
	rows := []any{}
	for _, v := range values {
		rows = append(rows, map[string]any{"metric": prefix + v, "value": counts[v], "denominator": total, "defined": true, "definition": definition, "origin": "supplied", "revision": a.Manifest.Repository.Revision, "limitations": []string{limitation}})
	}
	return rows
}
