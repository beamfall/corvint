package main

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

type corpusOptions struct {
	root, op, manifest, artifact, input, previous, revision, scope, timestamp, query, id, path, page, cem, retirement, runner, schema string
	// cases and baseline are the eval case file and ratchet receipt (DCP-V1-250, DCP-V1-254).
	cases, baseline string
	// repository is the locate and recommend-tests qualifier: a declared repository's root-commit
	// identity (DCP-V1-310, DCP-V1-312).
	repository string
	// state, qualifier and run select claims' supplied status claims (DCP-V1-318).
	state, qualifier, run string
	// provider is the carry-forward records provider ID (DCX-V1-366).
	provider string
	// uncovered and anchorless select the delta worklist view; paged reports whether --limit or
	// --offset was given (DCX-V1-421..423).
	uncovered, anchorless string
	paged                 bool
	// callerRevision is the caller's checkout commit compared with the corpus source revision
	// (DCP-V1-332).
	callerRevision string
	// scopes holds every repeated manifest --scope in argument order (DCP-V1-161).
	scopes []string
	// maxMentionRate and maxPatternRate are the opt-in build thresholds (DCX-V1-346, DCX-V1-352).
	maxMentionRate, maxPatternRate string
	// given records every option present on the command line, so an empty value is refused
	// rather than read as absent.
	given map[string]bool
	// section and include are the flow filters as comma-separated lists (DCP-V1-170, DCP-V1-171).
	section, include string
	// kind is the docs gaps kind filter as a comma-separated list (DCP-V1-321).
	kind string
	// excludeKind is the corpus claims kind exclusion as a comma-separated list (DCS-V1-592).
	excludeKind string
	// stable is the cem-stable verifier binding; its artifact root is not a --root-relative input.
	stable       doccorpus.StableCEMInputs
	playwright   playwrightCorpusOptions
	legacyCases  legacyCasesCorpusOptions
	audit        corpusAuditOptions
	apply, check bool
	// verifyRederive reads with the complete DCP-V1-011 rederivation and timings writes the read
	// phase summary to stderr (DCR-V1-413, DCR-V1-415).
	verifyRederive, timings bool
	limit, offset           int
	// readTimings collects the phases when timings is set.
	readTimings *doccorpus.ReadTimings
	// profile is the extract operation's repository path of its extractor profile at --revision
	// (DCX-V1-399); it is read from Git, not from --root. With profileSHA256 it is instead the
	// absolute path of a caller-owned profile outside the repository (DCX-V1-478).
	profile, profileSHA256 string
	// workspaceFile, noWorkspace and repos are the shared workspace declaration and its --repo
	// flag form (DCP-V1-604).
	workspaceFile string
	noWorkspace   bool
	repos         []string
}

// stringFlags maps each value-taking corpus option to its field.
func (o *corpusOptions) stringFlags() map[string]*string {
	flags := map[string]*string{"--retirement": &o.retirement, "--manifest": &o.manifest, "--artifact": &o.artifact, "--input": &o.input, "--previous": &o.previous, "--revision": &o.revision, "--scope": &o.scope, "--timestamp": &o.timestamp, "--query": &o.query, "--id": &o.id, "--path": &o.path, "--page": &o.page, "--cem": &o.cem, "--runner": &o.runner, "--schema": &o.schema, "--section": &o.section, "--include": &o.include, "--cases": &o.cases, "--baseline": &o.baseline, "--repository": &o.repository, "--state": &o.state, "--qualifier": &o.qualifier, "--run": &o.run, "--provider": &o.provider, "--caller-revision": &o.callerRevision, "--kind": &o.kind, "--exclude-kind": &o.excludeKind, "--uncovered": &o.uncovered, "--anchorless": &o.anchorless, "--workspace": &o.workspaceFile, "--repo": new(string),
		"--max-unresolved-mention-rate": &o.maxMentionRate, "--max-pattern-rate": &o.maxPatternRate,
		"--expected-base": &o.stable.ExpectedBase, "--target": &o.stable.Target, "--stable-artifact-root": &o.stable.ArtifactRoot}
	o.playwright.addFlags(flags)
	o.legacyCases.addFlags(flags)
	o.audit.addFlags(flags)
	flags["--profile"] = &o.profile
	flags["--profile-sha256"] = &o.profileSHA256
	return flags
}

func parseCorpusInvocation(args []string) (corpusOptions, bool, error) {
	o := corpusOptions{root: ".", limit: 20}
	pos := commandPositionAfterRoots(args)
	if pos < 0 || pos+1 >= len(args) || args[pos] != "docs" || args[pos+1] != "corpus" {
		return o, false, nil
	}
	for i := 0; i < pos; i++ {
		if args[i] == "--root" {
			o.root = args[i+1]
			i++
		} else {
			o.root = strings.TrimPrefix(args[i], "--root=")
		}
	}
	if pos+2 >= len(args) {
		return o, true, argumentError("docs corpus requires an operation")
	}
	o.op = args[pos+2]
	if op, ok := corpusAuditOperation(args[pos+2:]); ok {
		o.op = op
		pos++
	}
	flags := o.stringFlags()
	seen := map[string]bool{}
	for i := pos + 3; i < len(args); i++ {
		flag, value, inline := strings.Cut(args[i], "=")
		if seen[flag] && flag != "--scope" && flag != "--repo" {
			return o, true, argumentError("duplicate corpus option")
		}
		seen[flag] = true
		if flag == "--apply" || flag == "--check" || flag == "--verify-rederive" || flag == "--timings" || flag == "--no-workspace" {
			if inline {
				return o, true, argumentError(flag + " takes no value")
			}
			o.noWorkspace = o.noWorkspace || flag == "--no-workspace"
			o.apply = o.apply || flag == "--apply"
			o.check = o.check || flag == "--check"
			o.verifyRederive = o.verifyRederive || flag == "--verify-rederive"
			o.timings = o.timings || flag == "--timings"
			continue
		}
		if flag == "--limit" || flag == "--offset" {
			if !inline {
				if i+1 >= len(args) {
					return o, true, argumentError("missing corpus limit")
				}
				i++
				value = args[i]
			}
			n, err := strconv.Atoi(value)
			if err != nil || flag == "--limit" && (n < 1 || n > doccorpus.MaxResults) || flag == "--offset" && (n < 0 || n > doccorpus.MaxCorpusRecords*4) {
				return o, true, argumentError("invalid corpus limit")
			}
			if flag == "--limit" {
				o.limit = n
			} else {
				o.offset = n
			}
			continue
		}
		target, ok := flags[flag]
		if !ok {
			return o, true, argumentError("unsupported corpus option")
		}
		if !inline {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return o, true, argumentError("missing corpus option value")
			}
			i++
			value = args[i]
		}
		*target = value
		if flag == "--scope" {
			o.scopes = append(o.scopes, value)
		}
		if flag == "--repo" {
			o.repos = append(o.repos, value)
		}
	}
	o.given = seen
	o.paged = seen["--limit"] || seen["--offset"]
	operation, ok := lookupCorpusOperation(o.op)
	if !ok {
		return o, true, argumentError("unsupported corpus operation")
	}
	for flag := range seen {
		if !operation.admits(flag) {
			return o, true, argumentError("option does not apply to corpus operation")
		}
	}
	for _, flag := range operation.required {
		if strings.TrimSpace(*flags[flag]) == "" {
			return o, true, argumentError("missing required corpus option")
		}
	}
	if seen["--repository"] && o.repository == "" {
		return o, true, argumentError("--repository must name a repository")
	}
	// DCP-V1-318, DCP-V1-319, DCX-V1-506: a present status filter, --profile-sha256 or intent --id is non-empty.
	for _, flag := range []string{"--state", "--qualifier", "--run", "--profile-sha256"} {
		if seen[flag] && strings.TrimSpace(*flags[flag]) == "" {
			return o, true, argumentError(flag + " must not be empty")
		}
	}
	if o.op == "intent" && seen["--id"] && strings.TrimSpace(o.id) == "" {
		return o, true, argumentError("--id must not be empty")
	}
	if seen["--caller-revision"] && (o.callerRevision == "" || !doccorpus.ValidCallerRevision(o.callerRevision)) {
		return o, true, argumentError("--caller-revision must be a full lowercase hex Git object ID")
	}
	if seen["--schema"] && o.schema != "1" && o.schema != "2" {
		return o, true, argumentError("--schema must be 1 or 2")
	}
	if !doccorpus.ValidFlowFilter(corpusList(o.section), corpusList(o.include)) {
		return o, true, argumentError("invalid flow section or include filter")
	}
	if o.op == "search" && seen["--include"] && o.include != "claims" {
		return o, true, argumentError("search --include must be claims")
	}
	if !doccorpus.ValidGapKinds(corpusList(o.kind)) {
		return o, true, argumentError("invalid gap kind filter")
	}
	var err error
	o.root, err = resolveExplicitRoot(o.root)
	return o, true, err
}

// corpusList splits a comma-separated option value; an absent option is nil.
func corpusList(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

// errCorpusCheckRefused marks a behavior-adapter check report, or a legacy inventory whose
// crosswalk refused a reviewed mapping, that lists refusals; the report is still the command output.
var errCorpusCheckRefused = errors.New("behavior adapter check refused")

func runCorpus(ctx context.Context, o corpusOptions, stdout, stderr io.Writer) int {
	start := time.Now()
	if o.timings {
		o.readTimings = &doccorpus.ReadTimings{}
		// The summary is diagnostics on stderr after the result; stdout is unchanged (DCR-V1-415).
		defer func() { o.readTimings.Write(stderr, time.Since(start)) }()
	}
	data, err := compileCorpus(ctx, o)
	if errors.Is(err, errCorpusCheckRefused) || errors.Is(err, errCorpusEvalRegressed) {
		// DCP-V1-044: a refused check still prints its full report; so does a failed eval
		// ratchet (DCP-V1-254).
		if _, err := stdout.Write(data); err != nil {
			emitCorpusError(stderr, err)
			return 2
		}
		return 1
	}
	if err != nil {
		emitCorpusError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(data); err != nil {
		emitCorpusError(stderr, err)
		return 2
	}
	return 0
}
func emitCorpusError(w io.Writer, err error) {
	err = doccorpus.Typed(err) // a Git capacity failure is typed, never internal-error (DCP-V1-158)
	var e *doccorpus.Error
	if errors.As(err, &e) {
		emitError(w, &gokernel.Error{Code: e.Code, Message: e.Message})
		return
	}
	emitError(w, err)
}

// corpusInputPathError names the first local-input flag whose path the bounded reader would refuse
// and the rule it breaks (DCP-V1-054). The code stays corpus-refused; what the reader admits is unchanged.
func corpusInputPathError(o corpusOptions) error {
	flags := o.stringFlags()
	for _, flag := range corpusLocalInputFlags {
		value := *flags[flag]
		if value == "" {
			continue
		}
		if rule := doccorpus.LocalInputPathRule(value); rule != "" {
			return &doccorpus.Error{Code: "corpus-refused", Message: flag + ": local input paths must be canonical and relative to --root (" + rule + ")"}
		}
	}
	// --page names the page maintain rewrites, so its refusal names the flag and the broken rule
	// too, before any read; the code is unchanged (DCP-V1-241).
	if o.op == "maintain" && o.page != "" {
		if rule := doccorpus.CorpusPageRule(o.page); rule != "" {
			return &doccorpus.Error{Code: "corpus-refused", Message: "--page: maintenance requires a Markdown page relative to --root outside Git metadata (" + rule + ")"}
		}
	}
	return nil
}

// corpusBuild builds what docs corpus build prints: --manifest itself, or the manifest a
// --artifact embeds, which rebuilds that artifact and reuses its records when its derivation
// is this builder's (DCX-V1-495, DCX-V1-532). Only build writes.
func corpusBuild(ctx context.Context, o corpusOptions) (*doccorpus.Artifact, error) {
	// Presence, not value, decides: an empty --manifest= beside --artifact is still both.
	artifact := o.given["--artifact"]
	if artifact == o.given["--manifest"] || artifact && o.artifact == "" || !artifact && o.manifest == "" {
		return nil, argumentError("docs corpus build requires exactly one of --manifest or --artifact")
	}
	if artifact {
		raw, err := doccorpus.ReadCorpusFile(o.root, o.artifact)
		if err != nil {
			return nil, err
		}
		m, err := doccorpus.ManifestOfArtifact(raw)
		if err != nil {
			return nil, err
		}
		if _, err := corpusBuildThresholds(o); err != nil {
			return nil, err
		}
		a, _, err := doccorpus.RebuildArtifact(ctx, o.root, m, raw)
		return a, err
	}
	raw, err := doccorpus.ReadCorpusFile(o.root, o.manifest)
	if err != nil {
		return nil, err
	}
	m, err := doccorpus.ParseManifest(raw)
	if err != nil {
		return nil, err
	}
	if _, err := corpusBuildThresholds(o); err != nil {
		return nil, err
	}
	return doccorpus.BuildAdmitted(ctx, o.root, m)
}

func compileCorpus(ctx context.Context, o corpusOptions) ([]byte, error) {
	if err := corpusInputPathError(o); err != nil {
		return nil, err
	}
	// DCP-V1-602: trace takes exactly one of --id or --path, gaps at most one; --repository
	// names the repository of the path. Checked after parsing, before anything is read.
	if (o.op == "trace" || o.op == "gaps") && (o.id != "" && o.path != "" || o.op == "trace" && o.id == "" && o.path == "" || o.repository != "" && o.path == "") {
		return nil, argumentError("corpus " + o.op + " takes --id or --path, and --repository only with --path")
	}
	ws, err := bindCorpusWorkspace(o)
	if err != nil {
		return nil, err
	}
	if o.op == "manifest" {
		schema := doccorpus.ManifestSchema
		if o.schema == "2" {
			schema = doccorpus.ManifestSchemaV2
		}
		if ws != nil {
			m, err := corpusWorkspaceManifest(ctx, o, ws, schema)
			if err != nil {
				return nil, err
			}
			return doccorpus.Encode(m)
		}
		m, err := doccorpus.ManifestInventory(ctx, o.root, o.revision, o.scopes, schema, o.timestamp)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(m)
	}
	if o.op == "build" {
		a, err := corpusBuild(ctx, o)
		if err != nil {
			return nil, err
		}
		if err := checkCorpusWorkspace(o.root, ws, a.Manifest); err != nil {
			return nil, err
		}
		thresholds, err := corpusBuildThresholds(o)
		if err != nil {
			return nil, err
		}
		if err := doccorpus.CheckBuildThresholds(a, thresholds); err != nil {
			return nil, err
		}
		return doccorpus.Encode(a)
	}
	if o.op == "discover-playwright" || o.op == "witness-playwright" {
		return compilePlaywrightCorpus(ctx, o)
	}
	if o.op == "discover-legacy" {
		return compileLegacyCorpus(ctx, o)
	}
	if o.op == "discover-tests" {
		return compileTestDiscovery(ctx, o)
	}
	if o.op == "extract" {
		root, err := corpusExtractRoot(o, ws)
		if err != nil {
			return nil, err
		}
		return doccorpus.BuildExtractorInventory(ctx, doccorpus.ExtractorInput{Root: root, Revision: o.revision, Profile: o.profile, ProfileSHA256: o.profileSHA256, Schema: o.schema})
	}
	if o.op == "import-cases" || o.op == "search-cases" {
		return compileLegacyCasesCorpus(o)
	}
	if o.op == "behavior-provider" {
		raw, err := doccorpus.ReadFile(o.root, o.input)
		if err != nil {
			return nil, err
		}
		provider, err := doccorpus.BuildBehaviorProviderV2(raw)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(provider)
	}
	if o.op == "estate" {
		raw, err := doccorpus.ReadFile(o.root, o.input)
		if err != nil {
			return nil, err
		}
		in, err := doccorpus.ParseEstate(raw)
		if err != nil {
			return nil, err
		}
		receipt, err := doccorpus.Estate(ctx, o.root, in)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(receipt)
	}
	if o.op == "compact-provider" {
		raw, err := doccorpus.ReadProviderFile(o.root, o.input)
		if err != nil {
			return nil, err
		}
		return doccorpus.CompactProvider(o.input, raw)
	}
	if o.op == "behavior-adapter" {
		request, err := doccorpus.ReadFile(o.root, o.input)
		if err != nil {
			return nil, err
		}
		var previous []byte
		if o.previous != "" {
			previous, err = doccorpus.ReadFile(o.root, o.previous)
			if err != nil {
				return nil, err
			}
		}
		if o.check {
			report := doccorpus.CheckBehaviorAdapter(request, previous)
			data, err := doccorpus.Encode(report)
			if err == nil && !report.Accepted {
				err = errCorpusCheckRefused
			}
			return data, err
		}
		result, err := doccorpus.BuildBehaviorAdapter(request, previous)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(result)
	}
	if o.op == "eval" {
		return compileCorpusEval(ctx, o)
	}
	if o.op == "delta" || o.op == "carry-forward" {
		return compileCorpusDelta(ctx, o)
	}
	t := o.readTimings
	var step time.Time
	// Reads admit the verified artifact without rederiving it, streaming the file rather than
	// holding it whole; validate and --verify-rederive run the complete rederivation
	// (DCR-V1-410, DCR-V1-413, DCR-V1-431). A query's live freshness is observed while the
	// records are decoded and verified and the query index is derived, and used only once they
	// are (DCR-V1-411, DCR-V1-419).
	// Every exit, a refusal included, cancels and joins a started observation, so none outlives
	// the read or writes its timing after the summary is printed.
	liveCtx, cancelLive := context.WithCancel(ctx)
	live := &corpusLiveRead{root: o.root, timings: t}
	joinPrimary := func() {}
	defer func() {
		cancelLive()
		live.join()
		joinPrimary()
	}()
	options := doccorpus.ReadOptions{Rederive: o.verifyRederive || o.op == "validate", Timings: t, Index: doccorpus.IndexedOperation(o.op)}
	if corpusQueryOperation(o.op) {
		// The primary checkout is observed from the read's start, before the header arrives
		// (DCR-V1-465); the deferred cancel and join above end it on every exit (DCR-V1-467).
		liveCtx, joinPrimary = doccorpus.WithPrimaryObservation(liveCtx, o.root)
		options.Header = func(h *doccorpus.Artifact) { live.start(liveCtx, h) }
	}
	a, err := doccorpus.OpenReadFile(ctx, o.root, o.artifact, options)
	if err != nil {
		return nil, err
	}
	if err := checkCorpusWorkspace(o.root, ws, a.Manifest); err != nil {
		return nil, err
	}
	if o.op == "audit-sample" || o.op == "audit-report" {
		return compileCorpusAudit(a, o)
	}
	if o.op == "patterns" {
		return compileCorpusPatterns(a, o)
	}
	if o.op == "render" {
		output, err := doccorpus.Render(a)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(output)
	}
	if o.op == "maintain" {
		state, _, err := doccorpus.Freshness(ctx, o.root, a)
		if err != nil {
			return nil, err
		}
		if state != "fresh" {
			return nil, argumentError("maintenance requires fresh corpus evidence")
		}
		output, err := doccorpus.Maintain(o.root, o.page, a, o.apply)
		if err != nil {
			return nil, err
		}
		return doccorpus.Encode(output)
	}
	if o.apply {
		return nil, argumentError("--apply requires corpus maintain")
	}
	if o.op == "cem-stable" {
		return compileStableCorpusCEM(ctx, o, a)
	}
	if o.op == "cem" {
		data, err := doccorpus.ReadFile(o.root, o.cem)
		if err != nil {
			return nil, err
		}
		return nil, doccorpus.RefuseCEMProjection(data)
	}
	// DCP-V1-313, DCP-V1-318: corpus claims takes exactly one of --path, --query or --state;
	// --qualifier and --run refine --state.
	if o.op == "claims" {
		forms := 0
		for _, v := range []string{o.path, o.query, o.state} {
			if strings.TrimSpace(v) != "" {
				forms++
			}
		}
		if forms != 1 {
			return nil, argumentError("corpus claims takes exactly one of --path, --query or --state")
		}
		if o.state == "" && (o.qualifier != "" || o.run != "") || o.state != "" && !doccorpus.ValidStatusFilter(o.state, o.qualifier, o.run) {
			return nil, argumentError("invalid corpus claims status filter")
		}
		// DCS-V1-592: a claims kind filter and exclusion name distinct known claim kinds; a
		// given but empty --kind or --exclude-kind is an empty list, refused as MCP refuses it.
		kinds, excluded := corpusList(o.kind), corpusList(o.excludeKind)
		if o.given["--kind"] && kinds == nil || o.given["--exclude-kind"] && excluded == nil || !doccorpus.ValidClaimKinds(kinds) || !doccorpus.ValidClaimKinds(excluded) || slices.ContainsFunc(kinds, func(k string) bool { return slices.Contains(excluded, k) }) {
			return nil, argumentError("invalid claim kind filter")
		}
	}
	// DCS-V1-597: corpus navigation takes exactly one of --id or --query, and the one given is
	// not empty; presence, not value, decides which was given.
	if o.op == "navigation" && (o.given["--id"] == o.given["--query"] || o.given["--id"] && strings.TrimSpace(o.id) == "" || o.given["--query"] && strings.TrimSpace(o.query) == "") {
		return nil, argumentError("corpus navigation takes exactly one non-empty --id or --query")
	}
	// An indexed read answers from, or is refused by, its query index before any freshness
	// failure, as the corpus MCP server does (DCP-V1-195).
	if doccorpus.IndexedOperation(o.op) {
		step = time.Now()
		index, err := doccorpus.BuildQueryIndex(ctx, a)
		if t != nil {
			t.Index += time.Since(step)
		}
		if err != nil {
			return nil, err
		}
		a.RuntimeIndex = index
	}
	live.wait(liveCtx, a)
	if live.err != nil {
		return nil, live.err
	}
	overlay := live.overlay
	var retirement *doccorpus.RetirementPolicy
	if o.retirement != "" {
		raw, e := doccorpus.ReadFile(o.root, o.retirement)
		if e != nil {
			return nil, e
		}
		retirement, e = doccorpus.ParseRetirement(raw)
		if e != nil {
			return nil, e
		}
	}
	step = time.Now()
	receipt, err := doccorpus.QueryLive(ctx, a, doccorpus.Request{Operation: o.op, Retirement: retirement, Query: o.query, ID: o.id, Path: o.path, Limit: o.limit, Offset: o.offset, Sections: corpusList(o.section), Include: corpusList(o.include), Kinds: corpusList(o.kind), ExcludeKinds: corpusList(o.excludeKind), Repository: o.repository, State: o.state, Qualifier: o.qualifier, Run: o.run, CallerRevision: o.callerRevision}, overlay)
	if t != nil {
		t.Query += time.Since(step)
	}
	if err != nil {
		return nil, err
	}
	step = time.Now()
	data, err := doccorpus.Encode(receipt)
	if t != nil {
		t.Encode += time.Since(step)
	}
	return data, err
}

// corpusQueryOperation reports whether op answers a query receipt under live freshness, rather
// than auditing, rendering, maintaining or projecting the artifact.
func corpusQueryOperation(op string) bool {
	switch op {
	case "audit-sample", "audit-report", "render", "maintain", "cem-stable", "cem":
		return false
	}
	return true
}

// corpusLiveFreshness is the live observation a query read starts; tests replace it.
var corpusLiveFreshness = doccorpus.LiveFreshness

// corpusLiveRead observes a query's live freshness overlay.
type corpusLiveRead struct {
	root    string
	timings *doccorpus.ReadTimings
	done    chan struct{}
	overlay doccorpus.FreshnessOverlay
	err     error
}

// start observes on its own goroutine. It reads only the artifact's manifest, schema and
// builder, so it can start on the header.
func (l *corpusLiveRead) start(ctx context.Context, a *doccorpus.Artifact) {
	l.done = make(chan struct{})
	go func() {
		defer close(l.done)
		step := time.Now()
		l.overlay, l.err = corpusLiveFreshness(ctx, l.root, a)
		if l.timings != nil {
			l.timings.Freshness += time.Since(step)
		}
	}()
}

// wait returns once the observation started on the header, or a new one of a, has finished.
func (l *corpusLiveRead) wait(ctx context.Context, a *doccorpus.Artifact) {
	if l.done == nil {
		l.start(ctx, a)
	}
	<-l.done
}

// join waits for a started observation, if any, to finish.
func (l *corpusLiveRead) join() {
	if l.done != nil {
		<-l.done
	}
}

// compileStableCorpusCEM is the explicit cem/1.0 documentation projection
// (CEM-V1-162, CEM-V1-163). The repository is --root; the map is admitted only
// on a stable verifier ACCEPT, and a refusal exits 2 with no stdout, so the
// 0.x `cem` operation is never relabelled stable.
func compileStableCorpusCEM(ctx context.Context, o corpusOptions, a *doccorpus.Artifact) ([]byte, error) {
	data, err := doccorpus.ReadFile(o.root, o.cem)
	if err != nil {
		return nil, err
	}
	inputs := o.stable
	inputs.Repository = o.root
	if inputs.ArtifactRoot, err = filepath.Abs(inputs.ArtifactRoot); err != nil {
		return nil, argumentError("cannot resolve --stable-artifact-root")
	}
	receipt, err := doccorpus.StableCEMProjection(ctx, a, data, o.id, inputs)
	if err != nil {
		return nil, err
	}
	return doccorpus.Encode(receipt)
}
