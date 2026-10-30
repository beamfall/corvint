package ticket

// Obligation-ledger codecs (TOL-V0-001..008). A NATIVE record carries only the
// bounded reference; the ledger itself is the fold of an immutable chain of
// taskman-obligation-event/0 events, each retaining the whole mutation
// envelope that wrote it, so the fold and the audit never depend on a
// discarded envelope. The word "obligation" here never means the dependency
// obligation COMPLETED|GATE_PASSED (Obligations above).
import (
	"bytes"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const (
	ObligationEventProfile = "taskman-obligation-event/0"
	ObligationPlanProfile  = "taskman-obligation-plan/0"
	// MaxObligationEventBytes is the derived-event slot bound
	// (snapshot.MaxDerivedEventBytes, TOL-V0-002).
	MaxObligationEventBytes    = 65536
	MaxObligationTitleBytes    = 256
	MaxObligationReasonBytes   = 512
	MaxObligationChanges       = 64
	MaxObligationMatches       = 16
	MaxObligationPathElems     = 16
	MaxObligationPathElemBytes = 512
	MaxObligationTestIDBytes   = 256
	MaxObligationPlanTests     = 4096

	OpObligationsSeed    = "OBLIGATIONS_SEED"
	OpObligationsSet     = "OBLIGATIONS_SET"
	OpObligationsWitness = "OBLIGATIONS_WITNESS"

	ObligationOpen      = "OPEN"
	ObligationWitnessed = "WITNESSED"
	ObligationDefect    = "DEFECT"
	ObligationBlocked   = "BLOCKED"
	ObligationDeferred  = "DEFERRED"
	// ObligationPartial is the optional TOL-V0-058 state of a row some of
	// whose steps are witnessed while a later step is pending. It is set
	// only through OBLIGATIONS_SET and never counts as witnessed.
	ObligationPartial = "PARTIAL"

	ObligationSourceReport   = "PLAYWRIGHT_REPORT"
	ObligationSourceDeclared = "DECLARED"

	// Review-source kinds of an entry's optional sources list (TOL-V0-037).
	// They name what a reviewer read, not witness evidence.
	ObligationReviewLegacyCase = "legacy-case"
	ObligationReviewLegacySpec = "legacy-spec"
	ObligationReviewManualCase = "manual-case"
	ObligationReviewNone       = "none"

	MaxObligationSources          = 16
	MaxObligationSourceRefBytes   = 256
	MaxObligationSourceQueryBytes = 1024
	MaxObligationSourceSearched   = 32

	// Stable refusal detail prefixes (TOL-V0-002, 007, 009, 013).
	ObligationUnknownDetail        = "OBLIGATION_UNKNOWN:"
	ObligationCreditMismatchDetail = "OBLIGATION_CREDIT_MISMATCH:"
	ObligationEventTooLargeDetail  = "OBLIGATION_EVENT_TOO_LARGE:"
	ObligationReportDetail         = "OBLIGATION_REPORT:"
	ObligationVersionDetail        = "OBLIGATION_REPORT_VERSION_UNQUALIFIED:"
	ObligationChainDetail          = "OBLIGATION_CHAIN:"
	// TOL-V0-024: a report witness refused because its run's post-check
	// failed.
	ObligationPostCheckDetail = "OBLIGATION_POST_CHECK_FAILED:"
	// TOL-V0-035: a seeded alias that repeats another alias, or that holds
	// a native id of the ledger prefix as a whole token.
	ObligationAliasDuplicateDetail = "OBLIGATION_ALIAS_DUPLICATE:"
	ObligationAliasCollisionDetail = "OBLIGATION_ALIAS_COLLISION:"
	// TOL-V0-061: a qualification report that does not classify the
	// embedded fixture as expected.
	ObligationQualificationDetail = "OBLIGATION_QUALIFICATION_FAILED:"
	// TOL-V0-065: a witness naming a test repository the policy does not
	// declare.
	ObligationTestRepositoryDetail = "OBLIGATION_TEST_REPOSITORY_UNDECLARED:"
)

// ObligationStates is the closed entry state set (TOL-V0-003).
var ObligationStates = []string{ObligationOpen, ObligationWitnessed, ObligationDefect, ObligationBlocked, ObligationDeferred, ObligationPartial}

// ObligationPending reports whether state is one a witness can still credit
// and the source, review and batch checks still examine: OPEN, PARTIAL
// (TOL-V0-058), DEFECT or BLOCKED.
func ObligationPending(state string) bool {
	return state == ObligationOpen || state == ObligationPartial || state == ObligationDefect || state == ObligationBlocked
}

// ObligationOperations are the three ledger write operations.
var ObligationOperations = []string{OpObligationsSeed, OpObligationsSet, OpObligationsWitness}

// IsObligationOperation reports one of the three ledger write operations.
func IsObligationOperation(op string) bool {
	return op == OpObligationsSeed || op == OpObligationsSet || op == OpObligationsWitness
}

// ObligationCounts is the reference count object (TOL-V0-001). Total excludes
// DEFERRED entries; CoreTotal counts core entries that are not DEFERRED.
type ObligationCounts struct {
	Witnessed, Total, Deferred, CoreWitnessed, CoreTotal int64
}

func (c ObligationCounts) Value() wire.Value {
	return noteObject("witnessed", countValue(c.Witnessed), "total", countValue(c.Total), "deferred", countValue(c.Deferred),
		"coreWitnessed", countValue(c.CoreWitnessed), "coreTotal", countValue(c.CoreTotal))
}

func readObligationCounts(r *wire.Reader) ObligationCounts {
	r.Closed("witnessed", "total", "deferred", "coreWitnessed", "coreTotal")
	return ObligationCounts{
		Witnessed: r.Field("witnessed").Count().Int(), Total: r.Field("total").Count().Int(), Deferred: r.Field("deferred").Count().Int(),
		CoreWitnessed: r.Field("coreWitnessed").Count().Int(), CoreTotal: r.Field("coreTotal").Count().Int(),
	}
}

func (c ObligationCounts) valid() bool {
	return c.Witnessed <= c.Total && c.CoreWitnessed <= c.CoreTotal && c.CoreTotal <= c.Total &&
		c.CoreWitnessed <= c.Witnessed && c.Total+c.Deferred <= wire.ObligationsMaxEntries
}

// ObligationHighWater is the monotone witnessed mark of one acceptance
// revision (TOL-V0-016).
type ObligationHighWater struct {
	AcceptanceRevision wire.Count
	Witnessed          int64
}

// ObligationRaise names the WORKER attempt generation whose witness last
// raised the high-water mark (TOL-V0-018).
type ObligationRaise struct {
	Attempt            string
	Generation         wire.Size
	AcceptanceRevision wire.Count
}

// ObligationsReference is the optional record member (TOL-V0-001).
type ObligationsReference struct {
	Prefix    string
	Revision  wire.Count
	Head      wire.Digest
	Counts    ObligationCounts
	HighWater ObligationHighWater
	LastRaise *ObligationRaise
}

func (o ObligationsReference) Value() wire.Value {
	raise := wire.Null()
	if o.LastRaise != nil {
		raise = noteObject("attempt", wire.String(o.LastRaise.Attempt), "generation", wire.String(string(o.LastRaise.Generation)),
			"acceptanceRevision", wire.String(string(o.LastRaise.AcceptanceRevision)))
	}
	return noteObject("prefix", wire.String(o.Prefix), "revision", wire.String(string(o.Revision)), "head", wire.String(string(o.Head)),
		"counts", o.Counts.Value(),
		"highWater", noteObject("acceptanceRevision", wire.String(string(o.HighWater.AcceptanceRevision)), "witnessed", countValue(o.HighWater.Witnessed)),
		"lastRaise", raise)
}

// ReadObligationsReference reads and checks the closed reference shape.
func ReadObligationsReference(r *wire.Reader) *ObligationsReference {
	r.Closed("prefix", "revision", "head", "counts", "highWater", "lastRaise")
	o := &ObligationsReference{Revision: r.Field("revision").Count(), Head: r.Field("head").Digest(), Counts: readObligationCounts(r.Field("counts"))}
	o.Prefix = readObligationPrefix(r.Field("prefix"))
	hw := r.Field("highWater")
	hw.Closed("acceptanceRevision", "witnessed")
	o.HighWater = ObligationHighWater{AcceptanceRevision: hw.Field("acceptanceRevision").Count(), Witnessed: hw.Field("witnessed").Count().Int()}
	if lr := r.Field("lastRaise"); !lr.IsNull() {
		lr.Closed("attempt", "generation", "acceptanceRevision")
		o.LastRaise = &ObligationRaise{Attempt: lr.Field("attempt").Identifier(), Generation: lr.Field("generation").Size(), AcceptanceRevision: lr.Field("acceptanceRevision").Count()}
	}
	if r.Err() != nil {
		return nil
	}
	switch {
	case o.Revision.Int() < 1 || o.Revision.Int() > wire.ObligationsMaxEvents:
		r.Fail(wire.CodeLimitExceeded, "obligations revision must be 1..%d", wire.ObligationsMaxEvents)
	case !o.Counts.valid():
		r.Field("counts").Fail(wire.CodeMalformed, "obligation counts are inconsistent")
	case o.HighWater.AcceptanceRevision.Int() < 1 || o.HighWater.Witnessed > wire.ObligationsMaxEntries:
		r.Field("highWater").Fail(wire.CodeMalformed, "obligation high water is out of range")
	case o.LastRaise != nil && o.LastRaise.AcceptanceRevision.Int() < 1:
		r.Field("lastRaise").Fail(wire.CodeMalformed, "lastRaise acceptance revision must be at least 1")
	}
	if r.Err() != nil {
		return nil
	}
	return o
}

// validateObligations binds the reference to its record: NATIVE only, and a
// high-water mark of the current acceptance revision is never below the
// current witnessed count (TOL-V0-016). Older marks are stale, not wrong.
func (rec *Record) validateObligations() error {
	o := rec.ObligationsRef
	if o == nil {
		return nil
	}
	if rec.Source.Kind != "NATIVE" || rec.ShadowOverlay {
		return wire.Errorf(wire.CodeMalformed, "/obligations", "only a NATIVE record carries an obligation ledger")
	}
	acc := rec.AcceptanceRevision.Int()
	if o.HighWater.AcceptanceRevision.Int() > acc || (o.LastRaise != nil && o.LastRaise.AcceptanceRevision.Int() > acc) {
		return wire.Errorf(wire.CodeMalformed, "/obligations/highWater", "obligation marks cannot postdate the record's acceptance revision")
	}
	if o.HighWater.AcceptanceRevision.Int() == acc && o.HighWater.Witnessed < o.Counts.Witnessed {
		return wire.Errorf(wire.CodeMalformed, "/obligations/highWater", "high water is below the current witnessed count")
	}
	return nil
}

func countValue(n int64) wire.Value { return wire.String(string(wire.CountOf(n))) }

func readObligationPrefix(r *wire.Reader) string {
	s := r.String()
	if r.Err() != nil {
		return ""
	}
	if _, err := wire.ParseObligationPrefix(r.Where(), s); err != nil {
		r.Fail(wire.CodeOf(err), "%v", err)
		return ""
	}
	return s
}

// ReadObligationID reads one `<prefix>-<n>` id (TOL-V0-003).
func ReadObligationID(r *wire.Reader) string {
	s := r.String()
	if r.Err() != nil {
		return ""
	}
	if _, err := wire.ParseObligationID(r.Where(), s); err != nil {
		r.Fail(wire.CodeOf(err), "%v", err)
		return ""
	}
	return s
}

// readObligationAlias reads one adopter alias (TOL-V0-034).
func readObligationAlias(r *wire.Reader) string {
	s := r.String()
	if r.Err() != nil {
		return ""
	}
	if _, err := wire.ParseObligationAlias(r.Where(), s); err != nil {
		r.Fail(wire.CodeOf(err), "%v", err)
		return ""
	}
	return s
}

// ObligationIDPrefix is the prefix of a valid id.
func ObligationIDPrefix(id string) string {
	return id[:strings.LastIndexByte(id, '-')]
}

func readNonblank(r *wire.Reader, max int) string {
	s := r.Prose(1, max)
	if r.Err() == nil && strings.TrimSpace(s) == "" {
		r.Fail(wire.CodeMalformed, "prose must contain non-whitespace")
		return ""
	}
	return s
}

// readIDKeyed reads a semantic array whose objects are ordered by strictly
// ascending byte order of their `id` member.
func readIDKeyed(r *wire.Reader, max int, what string, each func(*wire.Reader) string) {
	items := r.Array(max, true)
	if r.Err() != nil {
		return
	}
	if len(items) == 0 {
		r.Fail(wire.CodeMalformed, "%s needs at least one entry", what)
		return
	}
	prev := ""
	for i, it := range items {
		id := each(it)
		if r.Err() != nil {
			return
		}
		if i > 0 && id <= prev {
			if id == prev {
				it.Fail(wire.CodeDuplicateID, "duplicate obligation id %q", id)
			} else {
				it.Fail(wire.CodeMalformed, "%s are sorted by id", what)
			}
			return
		}
		prev = id
	}
}

// ObligationMatch is one passing, source-bound match of an id (TOL-V0-004).
// StepPath runs from the outermost step to the matched step, so its last
// element is StepTitle; both are empty for a test-level match.
type ObligationMatch struct {
	Path      string
	TestID    string
	TitlePath []string
	StepTitle *string
	StepPath  []string
}

func (m ObligationMatch) Value() wire.Value {
	return noteObject("path", wire.String(m.Path), "testId", wire.String(m.TestID), "titlePath", wire.Strings(m.TitlePath),
		"stepTitle", wire.StringOrNull(m.StepTitle), "stepPath", wire.Strings(m.StepPath))
}

func readTitleElems(r *wire.Reader, min int) []string {
	out := r.Strings(MaxObligationPathElems, true, func(x *wire.Reader) string { return x.Prose(1, MaxObligationPathElemBytes) })
	if r.Err() == nil && len(out) < min {
		r.Fail(wire.CodeMalformed, "title path needs at least %d element(s)", min)
	}
	if out == nil {
		out = []string{}
	}
	return out
}

func readObligationMatch(r *wire.Reader) ObligationMatch {
	r.Closed("path", "testId", "titlePath", "stepTitle", "stepPath")
	m := ObligationMatch{Path: ReadFilePath(r.Field("path")), TestID: r.Field("testId").Prose(1, MaxObligationTestIDBytes)}
	m.TitlePath = readTitleElems(r.Field("titlePath"), 1)
	m.StepTitle = r.Field("stepTitle").ProseOrNull(MaxObligationPathElemBytes)
	m.StepPath = readTitleElems(r.Field("stepPath"), 0)
	if r.Err() != nil {
		return m
	}
	if m.StepTitle == nil && len(m.StepPath) != 0 || m.StepTitle != nil && (len(m.StepPath) == 0 || m.StepPath[len(m.StepPath)-1] != *m.StepTitle) {
		r.Fail(wire.CodeMalformed, "stepPath must end at stepTitle and be empty for a test-level match")
	}
	return m
}

func readObligationMatches(r *wire.Reader) []ObligationMatch {
	items := r.Array(MaxObligationMatches, false)
	if r.Err() == nil && len(items) == 0 {
		r.Fail(wire.CodeMalformed, "a report credit needs at least one match")
	}
	out := make([]ObligationMatch, 0, len(items))
	for _, it := range items {
		out = append(out, readObligationMatch(it))
	}
	return out
}

// MatchesValue encodes matches in canonical-byte set order.
func MatchesValue(ms []ObligationMatch) wire.Value {
	vs := make([]wire.Value, 0, len(ms))
	for _, m := range ms {
		vs = append(vs, m.Value())
	}
	sorted, err := wire.SortedSet("/matches", vs)
	if err != nil {
		return wire.Array(vs...)
	}
	return sorted
}

// ObligationDeclaration is the caller-vouched DECLARED witness (TOL-V0-004).
type ObligationDeclaration struct {
	ManifestSha256 wire.Digest
	TestID         string
	Reason         string
}

func (d ObligationDeclaration) Value() wire.Value {
	return noteObject("manifestSha256", wire.String(string(d.ManifestSha256)), "testId", wire.String(d.TestID), "reason", wire.String(d.Reason))
}

func readObligationDeclaration(r *wire.Reader) *ObligationDeclaration {
	if r.IsNull() {
		return nil
	}
	r.Closed("manifestSha256", "testId", "reason")
	return &ObligationDeclaration{ManifestSha256: r.Field("manifestSha256").Digest(), TestID: r.Field("testId").Prose(1, MaxObligationTestIDBytes), Reason: readNonblank(r.Field("reason"), MaxObligationReasonBytes)}
}

// MaxObligationEvidenceRefBytes bounds a provenance evidence reference
// (TOL-V0-059).
const MaxObligationEvidenceRefBytes = 256

// ObligationProvenance is the optional, caller-supplied provenance of one
// witness (TOL-V0-059): the commit of the test source, the tree of the test
// environment, a reference to retained evidence and the witnessing test's
// title. Each member is optional and at least one is named. It is reported,
// never checked against Git or the report.
type ObligationProvenance struct {
	TestCommit      *string
	EnvironmentTree *string
	EvidenceRef     *string
	TestTitle       *string
}

func (p ObligationProvenance) Value() wire.Value {
	o := wire.NewObject()
	for _, kv := range []struct {
		k string
		v *string
	}{{"environmentTree", p.EnvironmentTree}, {"evidenceRef", p.EvidenceRef}, {"testCommit", p.TestCommit}, {"testTitle", p.TestTitle}} {
		if kv.v != nil {
			o.Set(kv.k, wire.String(*kv.v))
		}
	}
	return wire.ObjectValue(o)
}

// Strings lists the named prose members, for secret screening.
func (p *ObligationProvenance) Strings() []string {
	var out []string
	if p == nil {
		return nil
	}
	for _, s := range []*string{p.EvidenceRef, p.TestTitle} {
		if s != nil {
			out = append(out, *s)
		}
	}
	return out
}

// readObligationProvenance reads the closed provenance object, every member
// optional and at least one present.
func readObligationProvenance(r *wire.Reader) *ObligationProvenance {
	r.Closed(wire.OptionalKeys(r.Value(), nil, "testCommit", "environmentTree", "evidenceRef", "testTitle")...)
	if r.Err() != nil {
		return nil
	}
	p := &ObligationProvenance{}
	opt := func(key string, read func(*wire.Reader) string) *string {
		if !wire.Has(r.Value(), key) {
			return nil
		}
		s := read(r.Field(key))
		return &s
	}
	p.TestCommit = opt("testCommit", (*wire.Reader).OID)
	p.EnvironmentTree = opt("environmentTree", (*wire.Reader).OID)
	p.EvidenceRef = opt("evidenceRef", func(f *wire.Reader) string { return readNonblank(f, MaxObligationEvidenceRefBytes) })
	p.TestTitle = opt("testTitle", func(f *wire.Reader) string { return readNonblank(f, MaxObligationTestIDBytes) })
	if r.Err() != nil {
		return nil
	}
	if p.TestCommit == nil && p.EnvironmentTree == nil && p.EvidenceRef == nil && p.TestTitle == nil {
		r.Fail(wire.CodeMalformed, "provenance names at least one member")
		return nil
	}
	return p
}

// ObligationEvidence is the WITNESSED evidence of one entry (TOL-V0-004).
// Provenance is nil when the witness named none (TOL-V0-059).
type ObligationEvidence struct {
	Source            string
	EventSha256       wire.Digest
	Commit            string
	ReportSha256      *wire.Digest
	PlaywrightVersion *string
	Matches           []ObligationMatch
	Declaration       *ObligationDeclaration
	Provenance        *ObligationProvenance
	// TestRepository is the policy-declared test repository the witness
	// resolved its commit and sources in (TOL-V0-065); nil when none.
	TestRepository *string
}

func (e ObligationEvidence) Value() wire.Value {
	report, matches, decl := wire.Null(), wire.Null(), wire.Null()
	if e.ReportSha256 != nil {
		report = wire.String(string(*e.ReportSha256))
	}
	if e.Matches != nil {
		matches = MatchesValue(e.Matches)
	}
	if e.Declaration != nil {
		decl = e.Declaration.Value()
	}
	v := noteObject("source", wire.String(e.Source), "eventSha256", wire.String(string(e.EventSha256)), "commit", wire.String(e.Commit),
		"reportSha256", report, "playwrightVersion", wire.StringOrNull(e.PlaywrightVersion), "matches", matches, "declaration", decl)
	if e.Provenance != nil {
		v.Obj.Set("provenance", e.Provenance.Value())
	}
	if e.TestRepository != nil {
		v.Obj.Set("testRepository", wire.String(*e.TestRepository))
	}
	return v
}

// ReadObligationEvidence reads and checks the source-specific shape.
func ReadObligationEvidence(r *wire.Reader) *ObligationEvidence {
	r.Closed(wire.OptionalKeys(r.Value(), []string{"source", "eventSha256", "commit", "reportSha256", "playwrightVersion", "matches", "declaration"}, "provenance", "testRepository")...)
	e := &ObligationEvidence{Source: r.Field("source").Enum(ObligationSourceReport, ObligationSourceDeclared, ObligationSourceReviewed), EventSha256: r.Field("eventSha256").Digest(), Commit: r.Field("commit").OID()}
	e.ReportSha256 = r.Field("reportSha256").DigestOrNull()
	e.PlaywrightVersion = r.Field("playwrightVersion").LabelOrNull()
	if m := r.Field("matches"); !m.IsNull() {
		e.Matches = readObligationMatches(m)
	}
	e.Declaration = readObligationDeclaration(r.Field("declaration"))
	if wire.Has(r.Value(), "provenance") {
		e.Provenance = readObligationProvenance(r.Field("provenance"))
	}
	if wire.Has(r.Value(), "testRepository") {
		name := r.Field("testRepository").Label()
		e.TestRepository = &name
	}
	if r.Err() != nil {
		return nil
	}
	report, declared := e.Source == ObligationSourceReport, e.Source == ObligationSourceDeclared
	if report != (e.ReportSha256 != nil) || report != (e.PlaywrightVersion != nil) || report != (e.Matches != nil) || declared != (e.Declaration != nil) {
		r.Fail(wire.CodeMalformed, "evidence members do not match source %s", e.Source)
		return nil
	}
	return e
}

// ObligationReviewSource is one element of an entry's optional review
// sources (TOL-V0-037): a reviewed legacy case, legacy spec or manual case
// {kind, ref, digest}, or a recorded empty search {kind: none, query,
// searched, at}. It records what a reviewer read; it is never witness
// evidence and never changes an entry's state.
type ObligationReviewSource struct {
	Kind     string
	Ref      string
	Digest   wire.Digest
	Query    string
	Searched []string
	At       wire.Timestamp
}

func (s ObligationReviewSource) Value() wire.Value {
	if s.Kind == ObligationReviewNone {
		searched := make([]wire.Value, 0, len(s.Searched))
		for _, id := range s.Searched {
			searched = append(searched, wire.String(id))
		}
		return noteObject("kind", wire.String(s.Kind), "query", wire.String(s.Query), "searched", wire.Array(searched...), "at", wire.String(string(s.At)))
	}
	return noteObject("kind", wire.String(s.Kind), "ref", wire.String(s.Ref), "digest", wire.String(string(s.Digest)))
}

func obligationSourcesValue(sources []ObligationReviewSource) wire.Value {
	vs := make([]wire.Value, 0, len(sources))
	for _, s := range sources {
		vs = append(vs, s.Value())
	}
	return wire.Array(vs...)
}

// readObligationSources reads 1..16 closed review sources in order. The
// codec admits an empty query or searched set; requireSourceReview is
// enforced by preflight (TOL-V0-040), so existing ledgers stay readable.
func readObligationSources(r *wire.Reader) []ObligationReviewSource {
	items := r.Array(MaxObligationSources, true)
	if r.Err() != nil {
		return nil
	}
	if len(items) == 0 {
		r.Fail(wire.CodeMalformed, "sources needs at least one entry")
		return nil
	}
	out := make([]ObligationReviewSource, 0, len(items))
	for _, it := range items {
		var s ObligationReviewSource
		if it.Value().Kind == wire.KindObject && wire.Has(it.Value(), "kind") {
			s.Kind = it.Field("kind").Enum(ObligationReviewLegacyCase, ObligationReviewLegacySpec, ObligationReviewManualCase, ObligationReviewNone)
		} else {
			it.Closed("kind")
		}
		if it.Err() != nil {
			return nil
		}
		if s.Kind == ObligationReviewNone {
			it.Closed("kind", "query", "searched", "at")
			s.Query = it.Field("query").Prose(0, MaxObligationSourceQueryBytes)
			s.Searched = it.Field("searched").Strings(MaxObligationSourceSearched, false, func(x *wire.Reader) string { return readNonblank(x, MaxObligationSourceRefBytes) })
			s.At = it.Field("at").Timestamp()
		} else {
			it.Closed("kind", "ref", "digest")
			s.Ref = readNonblank(it.Field("ref"), MaxObligationSourceRefBytes)
			s.Digest = it.Field("digest").Digest()
		}
		if it.Err() != nil {
			return nil
		}
		out = append(out, s)
	}
	return out
}

// ObligationEntry is one folded ledger entry (TOL-V0-003). Alias is the
// optional adopter alias fixed at seed (TOL-V0-034), "" when absent.
// Sources is the optional review-source list (TOL-V0-037), nil when absent.
type ObligationEntry struct {
	ID       string
	Alias    string
	Title    string
	Core     bool
	State    string
	Reason   *string
	Evidence *ObligationEvidence
	Sources  []ObligationReviewSource
	// Review is the optional latest reviewed witness (TOL-V0-075), nil when
	// absent.
	Review *ObligationReviewRecord
	// Cites, Scope and Candidate are the optional TOL-V0-079, 081 and 084
	// members, nil when absent.
	Cites     []ObligationCite
	Scope     *ObligationScope
	Candidate *ObligationCandidate
	// SatisfiedBy is the optional TOL-V0-120 mirror owner, nil when absent.
	SatisfiedBy   *ObligationMirrorRef
	UpdatedAt     wire.Timestamp
	UpdatedByID   string
	UpdatedByRole string
}

func (e ObligationEntry) Value() wire.Value {
	ev := wire.Null()
	if e.Evidence != nil {
		ev = e.Evidence.Value()
	}
	v := noteObject("id", wire.String(e.ID), "title", wire.String(e.Title), "core", wire.Bool(e.Core), "state", wire.String(e.State),
		"reason", wire.StringOrNull(e.Reason), "evidence", ev, "updatedAt", wire.String(string(e.UpdatedAt)),
		"updatedBy", noteObject("id", wire.String(e.UpdatedByID), "role", wire.String(e.UpdatedByRole)))
	if e.Alias != "" {
		v.Obj.Set("alias", wire.String(e.Alias))
	}
	if e.Sources != nil {
		v.Obj.Set("sources", obligationSourcesValue(e.Sources))
	}
	if e.Review != nil {
		v.Obj.Set("review", e.Review.Value())
	}
	if e.Cites != nil {
		v.Obj.Set("cites", obligationCitesValue(e.Cites))
	}
	if e.Scope != nil {
		v.Obj.Set("scope", e.Scope.Value())
	}
	if e.Candidate != nil {
		v.Obj.Set("candidate", e.Candidate.Value())
	}
	if e.SatisfiedBy != nil {
		v.Obj.Set("satisfiedBy", e.SatisfiedBy.Value())
	}
	return v
}

// ReadObligationEntry reads one closed entry and checks that evidence is
// present exactly when the state is WITNESSED.
func ReadObligationEntry(r *wire.Reader) *ObligationEntry {
	r.Closed(wire.OptionalKeys(r.Value(), []string{"id", "title", "core", "state", "reason", "evidence", "updatedAt", "updatedBy"}, "alias", "sources", "review", "cites", "scope", "candidate", "satisfiedBy")...)
	e := &ObligationEntry{ID: ReadObligationID(r.Field("id")), Title: readNonblank(r.Field("title"), MaxObligationTitleBytes), Core: r.Field("core").Bool(),
		State: r.Field("state").Enum(ObligationStates...), UpdatedAt: r.Field("updatedAt").Timestamp()}
	if wire.Has(r.Value(), "alias") {
		e.Alias = readObligationAlias(r.Field("alias"))
	}
	if wire.Has(r.Value(), "sources") {
		e.Sources = readObligationSources(r.Field("sources"))
	}
	if wire.Has(r.Value(), "review") {
		e.Review = readObligationReviewRecord(r.Field("review"))
	}
	if wire.Has(r.Value(), "cites") {
		e.Cites = readObligationCites(r.Field("cites"))
	}
	if wire.Has(r.Value(), "scope") {
		e.Scope = readObligationScope(r.Field("scope"))
	}
	if wire.Has(r.Value(), "candidate") {
		e.Candidate = readObligationCandidate(r.Field("candidate"), true)
	}
	if wire.Has(r.Value(), "satisfiedBy") {
		e.SatisfiedBy = readObligationMirrorRef(r.Field("satisfiedBy"))
	}
	e.Reason = r.Field("reason").StringOrNull(func(x *wire.Reader) string { return readNonblank(x, MaxObligationReasonBytes) })
	if ev := r.Field("evidence"); !ev.IsNull() {
		e.Evidence = ReadObligationEvidence(ev)
	}
	by := r.Field("updatedBy")
	by.Closed("id", "role")
	e.UpdatedByID = by.Field("id").Label()
	e.UpdatedByRole = by.Field("role").Enum("OWNER", "OPERATOR", "WORKER")
	if r.Err() != nil {
		return nil
	}
	if (e.State == ObligationWitnessed) != (e.Evidence != nil) {
		r.Fail(wire.CodeMalformed, "evidence is present exactly when state is WITNESSED")
		return nil
	}
	return e
}

// ObligationSeedItem is one seeded obligation {id, alias?, title, core}.
// Alias is the optional adopter alias (TOL-V0-034), "" when absent.
type ObligationSeedItem struct {
	ID    string
	Alias string
	Title string
	Core  bool
}

// ObligationSeedPayload is the OBLIGATIONS_SEED payload (TOL-V0-005).
type ObligationSeedPayload struct {
	Prefix      string
	Obligations []ObligationSeedItem
}

func (p ObligationSeedPayload) Value() wire.Value {
	vs := make([]wire.Value, 0, len(p.Obligations))
	for _, o := range p.Obligations {
		v := noteObject("id", wire.String(o.ID), "title", wire.String(o.Title), "core", wire.Bool(o.Core))
		if o.Alias != "" {
			v.Obj.Set("alias", wire.String(o.Alias))
		}
		vs = append(vs, v)
	}
	return noteObject("prefix", wire.String(p.Prefix), "obligations", wire.Array(vs...))
}

// ReadObligationSeedPayload reads {prefix, obligations}: 1..256 sorted
// unique entries whose ids carry the payload prefix.
func ReadObligationSeedPayload(r *wire.Reader) *ObligationSeedPayload {
	r.Closed("prefix", "obligations")
	p := &ObligationSeedPayload{Prefix: readObligationPrefix(r.Field("prefix"))}
	readIDKeyed(r.Field("obligations"), wire.ObligationsMaxEntries, "obligations", func(it *wire.Reader) string {
		it.Closed(wire.OptionalKeys(it.Value(), []string{"id", "title", "core"}, "alias")...)
		o := ObligationSeedItem{ID: ReadObligationID(it.Field("id")), Title: readNonblank(it.Field("title"), MaxObligationTitleBytes), Core: it.Field("core").Bool()}
		if wire.Has(it.Value(), "alias") {
			o.Alias = readObligationAlias(it.Field("alias"))
		}
		if it.Err() == nil && ObligationIDPrefix(o.ID) != p.Prefix {
			it.Field("id").Fail(wire.CodeMalformed, "obligation id %q does not carry prefix %s", o.ID, p.Prefix)
		}
		p.Obligations = append(p.Obligations, o)
		return o.ID
	})
	return p
}

// ObligationChange is one OBLIGATIONS_SET change (TOL-V0-007). Sources,
// when non-nil, replaces the entry's review sources (TOL-V0-038); Cites
// replaces its cited source locations (TOL-V0-079); Scope proposes or
// accepts a scope decision (TOL-V0-080); Candidate records a witness
// candidate awaiting review (TOL-V0-084).
type ObligationChange struct {
	ID        string
	State     *string
	Core      *bool
	Reason    string
	Sources   []ObligationReviewSource
	Cites     []ObligationCite
	Scope     *ObligationScopeChange
	Candidate *ObligationCandidate
	// Mirror is the optional satisfiedBy member (TOL-V0-120): set or clear
	// the entry's mirror owner.
	Mirror *ObligationMirrorChange
}

// ObligationSetPayload is the OBLIGATIONS_SET payload.
type ObligationSetPayload struct{ Changes []ObligationChange }

func (p ObligationSetPayload) Value() wire.Value {
	vs := make([]wire.Value, 0, len(p.Changes))
	for _, c := range p.Changes {
		core := wire.Null()
		if c.Core != nil {
			core = wire.Bool(*c.Core)
		}
		v := noteObject("id", wire.String(c.ID), "state", wire.StringOrNull(c.State), "core", core, "reason", wire.String(c.Reason))
		if c.Sources != nil {
			v.Obj.Set("sources", obligationSourcesValue(c.Sources))
		}
		if c.Cites != nil {
			v.Obj.Set("cites", obligationCitesValue(c.Cites))
		}
		if c.Scope != nil {
			v.Obj.Set("scope", c.Scope.Value())
		}
		if c.Candidate != nil {
			v.Obj.Set("candidate", c.Candidate.Value())
		}
		if c.Mirror != nil {
			v.Obj.Set("satisfiedBy", c.Mirror.Value())
		}
		vs = append(vs, v)
	}
	return noteObject("changes", wire.Array(vs...))
}

// ReadObligationSetPayload reads {changes}: 1..64 sorted unique changes,
// each naming a state, a core flag or review sources, and a required
// reason. WITNESSED is not a settable state.
func ReadObligationSetPayload(r *wire.Reader) *ObligationSetPayload {
	r.Closed("changes")
	p := &ObligationSetPayload{}
	readIDKeyed(r.Field("changes"), MaxObligationChanges, "changes", func(it *wire.Reader) string {
		it.Closed(wire.OptionalKeys(it.Value(), []string{"id", "state", "core", "reason"}, "sources", "cites", "scope", "candidate", "satisfiedBy")...)
		c := ObligationChange{ID: ReadObligationID(it.Field("id")), Reason: readNonblank(it.Field("reason"), MaxObligationReasonBytes)}
		c.State = it.Field("state").StringOrNull(func(x *wire.Reader) string {
			return x.Enum(ObligationOpen, ObligationDefect, ObligationBlocked, ObligationDeferred, ObligationPartial)
		})
		if core := it.Field("core"); !core.IsNull() {
			b := core.Bool()
			c.Core = &b
		}
		if wire.Has(it.Value(), "sources") {
			c.Sources = readObligationSources(it.Field("sources"))
		}
		if wire.Has(it.Value(), "cites") {
			c.Cites = readObligationCites(it.Field("cites"))
		}
		if wire.Has(it.Value(), "scope") {
			c.Scope = readObligationScopeChange(it.Field("scope"))
		}
		if wire.Has(it.Value(), "candidate") {
			c.Candidate = readObligationCandidate(it.Field("candidate"), false)
		}
		if wire.Has(it.Value(), "satisfiedBy") {
			c.Mirror = readObligationMirrorChange(it.Field("satisfiedBy"))
		}
		if it.Err() == nil && c.State == nil && c.Core == nil && c.Sources == nil && c.Cites == nil && c.Scope == nil && c.Candidate == nil && c.Mirror == nil {
			it.Fail(wire.CodeMalformed, "a change names a state, a core flag, review sources, cites, a scope decision, a candidate, a mirror or a combination")
		}
		if it.Err() == nil && c.Scope != nil && c.Core != nil {
			it.Fail(wire.CodeMalformed, "a scope decision sets core itself; core must be null beside scope")
		}
		p.Changes = append(p.Changes, c)
		return c.ID
	})
	return p
}

// ObligationCredit is one witness credit: {id, matches} for a report, {id}
// for a declaration.
type ObligationCredit struct {
	ID      string
	Matches []ObligationMatch
}

// ObligationWitnessPayload is the OBLIGATIONS_WITNESS payload (TOL-V0-008).
type ObligationWitnessPayload struct {
	Source            string
	Commit            string
	ReportSha256      *wire.Digest
	PlaywrightVersion *string
	Credits           []ObligationCredit
	Declaration       *ObligationDeclaration
	Attempt           *string
	Generation        *wire.Size
	// Provenance is the optional witness provenance (TOL-V0-059); nil keeps
	// the exact pre-TOL-V0-059 payload bytes.
	Provenance *ObligationProvenance
	// TestRepository is the optional policy-declared test repository name
	// (TOL-V0-065); nil keeps the exact earlier payload bytes.
	TestRepository *string
	// Review is the REVIEWED source's verdicts (TOL-V0-073); nil for every
	// other source, keeping the exact earlier payload bytes.
	Review *ObligationReview
}

func (p ObligationWitnessPayload) Value() wire.Value {
	report, decl, gen := wire.Null(), wire.Null(), wire.Null()
	if p.ReportSha256 != nil {
		report = wire.String(string(*p.ReportSha256))
	}
	if p.Declaration != nil {
		decl = p.Declaration.Value()
	}
	if p.Generation != nil {
		gen = wire.String(string(*p.Generation))
	}
	vs := make([]wire.Value, 0, len(p.Credits))
	for _, c := range p.Credits {
		if p.Source == ObligationSourceReport {
			vs = append(vs, noteObject("id", wire.String(c.ID), "matches", MatchesValue(c.Matches)))
		} else {
			vs = append(vs, noteObject("id", wire.String(c.ID)))
		}
	}
	v := noteObject("source", wire.String(p.Source), "commit", wire.String(p.Commit), "reportSha256", report,
		"playwrightVersion", wire.StringOrNull(p.PlaywrightVersion), "credits", wire.Array(vs...), "declaration", decl,
		"attempt", wire.StringOrNull(p.Attempt), "generation", gen)
	if p.Provenance != nil {
		v.Obj.Set("provenance", p.Provenance.Value())
	}
	if p.TestRepository != nil {
		v.Obj.Set("testRepository", wire.String(*p.TestRepository))
	}
	if p.Review != nil {
		v.Obj.Set("review", p.Review.Value())
	}
	return v
}

// ReadObligationWitnessPayload reads the closed witness payload and checks
// its source-specific members.
func ReadObligationWitnessPayload(r *wire.Reader) *ObligationWitnessPayload {
	r.Closed(wire.OptionalKeys(r.Value(), []string{"source", "commit", "reportSha256", "playwrightVersion", "credits", "declaration", "attempt", "generation"}, "provenance", "testRepository", "review")...)
	p := &ObligationWitnessPayload{Source: r.Field("source").Enum(ObligationSourceReport, ObligationSourceDeclared, ObligationSourceReviewed), Commit: r.Field("commit").OID()}
	p.ReportSha256 = r.Field("reportSha256").DigestOrNull()
	p.PlaywrightVersion = r.Field("playwrightVersion").LabelOrNull()
	p.Declaration = readObligationDeclaration(r.Field("declaration"))
	p.Attempt = r.Field("attempt").StringOrNull((*wire.Reader).Identifier)
	p.Generation = r.Field("generation").SizeOrNull()
	if wire.Has(r.Value(), "provenance") {
		p.Provenance = readObligationProvenance(r.Field("provenance"))
	}
	if wire.Has(r.Value(), "testRepository") {
		name := r.Field("testRepository").Label()
		p.TestRepository = &name
	}
	if wire.Has(r.Value(), "review") {
		p.Review = ReadObligationReview(r.Field("review"))
	}
	if r.Err() != nil {
		return p
	}
	report, declared, reviewed := p.Source == ObligationSourceReport, p.Source == ObligationSourceDeclared, p.Source == ObligationSourceReviewed
	if report != (p.ReportSha256 != nil) || report != (p.PlaywrightVersion != nil) || declared != (p.Declaration != nil) || reviewed != (p.Review != nil) {
		r.Fail(wire.CodeMalformed, "witness members do not match source %s", p.Source)
		return p
	}
	if (p.Attempt == nil) != (p.Generation == nil) {
		r.Fail(wire.CodeMalformed, "attempt and generation are named together or not at all")
		return p
	}
	readIDKeyed(r.Field("credits"), wire.ObligationsMaxEntries, "credits", func(it *wire.Reader) string {
		var c ObligationCredit
		if report {
			it.Closed("id", "matches")
			c.ID = ReadObligationID(it.Field("id"))
			c.Matches = readObligationMatches(it.Field("matches"))
		} else {
			it.Closed("id")
			c.ID = ReadObligationID(it.Field("id"))
		}
		p.Credits = append(p.Credits, c)
		return c.ID
	})
	if reviewed && r.Err() == nil && len(p.Credits) != 1 {
		r.Field("credits").Fail(wire.CodeMalformed, "a REVIEWED witness credits exactly one obligation")
	}
	return p
}

// SortObligationPayload sorts, in place, the id-keyed arrays of an
// obligation payload by id, so a CLI caller need not pre-sort. It is the
// input boundary only; the wire reader still requires sorted bytes.
func SortObligationPayload(op string, v wire.Value) {
	key := map[string]string{OpObligationsSeed: "obligations", OpObligationsSet: "changes", OpObligationsWitness: "credits"}[op]
	if v.Kind != wire.KindObject || v.Obj == nil || key == "" {
		return
	}
	arr, ok := v.Obj.Get(key)
	if !ok || arr.Kind != wire.KindArray {
		return
	}
	idOf := func(x wire.Value) string {
		if x.Kind == wire.KindObject && x.Obj != nil {
			if s, ok := x.Obj.Get("id"); ok && s.Kind == wire.KindString {
				return s.Str
			}
		}
		return ""
	}
	sort.SliceStable(arr.Arr, func(i, j int) bool { return idOf(arr.Arr[i]) < idOf(arr.Arr[j]) })
}

// ObligationRequest is the closed view of the mutation envelope one ledger
// event retains (TOL-V0-002). Actor fields are claims; authority is the
// mutation layer's (TOL-V0-015) and the audit's.
type ObligationRequest struct {
	RequestID          string
	QueueID            wire.QueueID
	TicketID           wire.TicketID
	ExpectedRevision   wire.Count
	ActorID, ActorRole string
	Operation          string
	IssuedAt           wire.Timestamp
	Seed               *ObligationSeedPayload
	Set                *ObligationSetPayload
	Witness            *ObligationWitnessPayload
	Raw                []byte
}

// DecodeObligationRequest decodes a retained taskman-mutation/0 envelope of
// one OBLIGATIONS_* operation through the same payload readers as the
// mutation decoder.
func DecodeObligationRequest(raw []byte) (*ObligationRequest, error) {
	if len(raw) > MaxObligationEventBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/request", "obligation request too large")
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	if err := wire.ProfileVersion("/request/profile", v, "taskman-mutation/0"); err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/request")
	r.Closed("profile", "requestId", "actor", "queueId", "targetId", "expectedRevision", "operation", "payload", "issuedAt")
	if err = r.Err(); err != nil {
		return nil, err
	}
	if err = wire.CheckProfile("/request/profile", r.Field("profile").String(), "taskman-mutation/0"); err != nil {
		return nil, err
	}
	q := &ObligationRequest{RequestID: r.Field("requestId").Identifier(), QueueID: r.Field("queueId").QueueID(), TicketID: r.Field("targetId").TicketID(),
		ExpectedRevision: r.Field("expectedRevision").Count(), Operation: r.Field("operation").Enum(ObligationOperations...), IssuedAt: r.Field("issuedAt").Timestamp()}
	a := r.Field("actor")
	a.Closed("id", "role")
	q.ActorID = a.Field("id").Label()
	q.ActorRole = a.Field("role").Enum("OWNER", "OPERATOR", "WORKER")
	if err = r.Err(); err != nil {
		return nil, err
	}
	p := r.Field("payload")
	switch q.Operation {
	case OpObligationsSeed:
		q.Seed = ReadObligationSeedPayload(p)
	case OpObligationsSet:
		q.Set = ReadObligationSetPayload(p)
	default:
		q.Witness = ReadObligationWitnessPayload(p)
	}
	if err = r.Err(); err != nil {
		return nil, err
	}
	if len(q.RequestID) > wire.MaxRequestIDBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/request/requestId", "request ID too large")
	}
	if q.TicketID.QueueID() != q.QueueID.Raw {
		return nil, wire.Errorf(wire.CodeMalformed, "/request/targetId", "ticket outside request queue")
	}
	q.Raw = append([]byte(nil), raw...)
	return q, nil
}

// ObligationEvent is one immutable ledger event (TOL-V0-002).
type ObligationEvent struct {
	TicketID      wire.TicketID
	Revision      wire.Count
	Previous      *wire.Digest
	RequestSha256 wire.Digest
	Request       []byte
	Decoded       *ObligationRequest
}

// Encode validates the proposed event through the closed decoder.
func (e ObligationEvent) Encode() ([]byte, error) {
	q, err := wire.Parse(e.Request)
	if err != nil {
		return nil, err
	}
	previous := wire.Null()
	if e.Previous != nil {
		previous = wire.String(string(*e.Previous))
	}
	raw := wire.EncodeFile(noteObject("profile", wire.String(ObligationEventProfile), "ticketId", wire.String(e.TicketID.Raw),
		"revision", wire.String(string(e.Revision)), "previous", previous, "requestSha256", wire.String(string(e.RequestSha256)), "request", q))
	if _, err = DecodeObligationEvent(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// DecodeObligationEvent reads one event, its retained request and the
// request digest binding.
func DecodeObligationEvent(raw []byte) (*ObligationEvent, error) {
	if len(raw) > MaxObligationEventBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "%s obligation event exceeds %d bytes", ObligationEventTooLargeDetail, MaxObligationEventBytes)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	if err := wire.ProfileVersion("/profile", v, ObligationEventProfile); err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "ticketId", "revision", "previous", "requestSha256", "request")
	if err = r.Err(); err != nil {
		return nil, err
	}
	if err = wire.CheckProfile("/profile", r.Field("profile").String(), ObligationEventProfile); err != nil {
		return nil, err
	}
	e := &ObligationEvent{TicketID: r.Field("ticketId").TicketID(), Revision: r.Field("revision").Count(), Previous: r.Field("previous").DigestOrNull(), RequestSha256: r.Field("requestSha256").Digest()}
	if err = r.Err(); err != nil {
		return nil, err
	}
	request, _ := v.Obj.Get("request")
	e.Request = wire.EncodeFile(request)
	if e.Decoded, err = DecodeObligationRequest(e.Request); err != nil {
		return nil, err
	}
	if e.Revision.Int() < 1 || e.Revision.Int() > wire.ObligationsMaxEvents {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/revision", "ledger revision must be 1..%d", wire.ObligationsMaxEvents)
	}
	if (e.Revision == "1") != (e.Previous == nil) {
		return nil, wire.Errorf(wire.CodeMalformed, "/previous", "only the first event has a null previous")
	}
	if wire.Sum(e.Request) != e.RequestSha256 {
		return nil, wire.Errorf(wire.CodeMalformed, "/requestSha256", "retained request digest mismatch")
	}
	if e.Decoded.TicketID.Raw != e.TicketID.Raw {
		return nil, wire.Errorf(wire.CodeMalformed, "/request/targetId", "event/request ticket mismatch")
	}
	return e, nil
}

// ObligationLedger is the folded ledger: entries in ascending id order.
type ObligationLedger struct {
	Prefix   string
	Revision int64
	Head     wire.Digest
	Entries  []ObligationEntry
	// Events is the event chain from the first event to Head, kept so a
	// reader can show each event's receipt binding.
	Events []wire.Digest
	// Unblocks counts, across the chain, entries that left BLOCKED
	// (TOL-V0-124); it is derived by the fold and never encoded.
	Unblocks int64
}

// Entry returns the entry with id, or nil.
func (l *ObligationLedger) Entry(id string) *ObligationEntry {
	if l == nil {
		return nil
	}
	i := sort.Search(len(l.Entries), func(i int) bool { return l.Entries[i].ID >= id })
	if i < len(l.Entries) && l.Entries[i].ID == id {
		return &l.Entries[i]
	}
	return nil
}

// Counts derives the reference counts (TOL-V0-001).
func (l *ObligationLedger) Counts() ObligationCounts {
	var c ObligationCounts
	if l == nil {
		return c
	}
	for _, e := range l.Entries {
		if e.State == ObligationDeferred {
			c.Deferred++
			continue
		}
		c.Total++
		if e.Core {
			c.CoreTotal++
		}
		if e.State == ObligationWitnessed {
			c.Witnessed++
			if e.Core {
				c.CoreWitnessed++
			}
		}
	}
	return c
}

// admitAlias refuses an alias that another entry already carries, or that
// holds a native id of the ledger prefix as a whole token (TOL-V0-035).
func (l *ObligationLedger) admitAlias(alias string) error {
	for _, e := range l.Entries {
		if e.Alias == alias {
			return wire.Errorf(wire.CodeDuplicateID, "/request/payload/obligations", "%s alias %s is already carried by %s", ObligationAliasDuplicateDetail, alias, e.ID)
		}
	}
	if nativeIDIn(alias, l.Prefix) {
		return wire.Errorf(wire.CodeDuplicateID, "/request/payload/obligations", "%s alias %s holds a native %s id", ObligationAliasCollisionDetail, alias, l.Prefix)
	}
	return nil
}

// nativeIDIn reports whether s holds a valid id of prefix as a whole token
// bounded by a byte outside [A-Za-z0-9-] (the TOL-V0-010 rule).
func nativeIDIn(s, prefix string) bool {
	idByte := func(c byte) bool {
		return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-'
	}
	for i := 0; i < len(s); {
		if !idByte(s[i]) {
			i++
			continue
		}
		j := i
		for j < len(s) && idByte(s[j]) {
			j++
		}
		if p, err := wire.ParseObligationID("id", s[i:j]); err == nil && p == prefix {
			return true
		}
		i = j
	}
	return false
}

// Aliases maps each alias of the ledger to its native id (TOL-V0-034).
func (l *ObligationLedger) Aliases() map[string]string {
	out := map[string]string{}
	if l != nil {
		for _, e := range l.Entries {
			if e.Alias != "" {
				out[e.Alias] = e.ID
			}
		}
	}
	return out
}

// EntriesValue encodes the folded entries.
func (l *ObligationLedger) EntriesValue() wire.Value {
	vs := []wire.Value{}
	if l != nil {
		for _, e := range l.Entries {
			vs = append(vs, e.Value())
		}
	}
	return wire.Array(vs...)
}

func (l *ObligationLedger) clone() *ObligationLedger {
	out := &ObligationLedger{}
	if l != nil {
		out.Prefix, out.Revision, out.Head, out.Unblocks = l.Prefix, l.Revision, l.Head, l.Unblocks
		out.Entries = append([]ObligationEntry{}, l.Entries...)
		out.Events = append([]wire.Digest{}, l.Events...)
	}
	return out
}

// ApplyObligationEvent folds one event whose digest is eventSha256 onto l (nil
// before the first event). It is the single transition used by the writer
// and by every fold and audit, so they cannot disagree. Authority is not
// checked here; refusal codes follow TOL-V0-003..008.
func ApplyObligationEvent(l *ObligationLedger, ev *ObligationEvent, eventSha256 wire.Digest) (*ObligationLedger, error) {
	q := ev.Decoded
	if ev.Revision.Int() != revisionOf(l)+1 {
		return nil, wire.Errorf(wire.CodeMalformed, "/revision", "%s event revision %s does not follow ledger revision %d", ObligationChainDetail, ev.Revision, revisionOf(l))
	}
	if (l == nil) != (ev.Previous == nil) || (l != nil && *ev.Previous != l.Head) {
		return nil, wire.Errorf(wire.CodeMalformed, "/previous", "%s event does not link to the ledger head", ObligationChainDetail)
	}
	out := l.clone()
	stamp := func(e *ObligationEntry) {
		e.UpdatedAt, e.UpdatedByID, e.UpdatedByRole = q.IssuedAt, q.ActorID, q.ActorRole
	}
	switch q.Operation {
	case OpObligationsSeed:
		if out.Prefix != "" && out.Prefix != q.Seed.Prefix {
			return nil, wire.Errorf(wire.CodeMalformed, "/request/payload/prefix", "a later seed must repeat ledger prefix %s", out.Prefix)
		}
		out.Prefix = q.Seed.Prefix
		for _, o := range q.Seed.Obligations {
			if out.Entry(o.ID) != nil {
				return nil, wire.Errorf(wire.CodeDuplicateID, "/request/payload/obligations", "obligation %s is already in the ledger", o.ID)
			}
			if o.Alias != "" {
				if err := out.admitAlias(o.Alias); err != nil {
					return nil, err
				}
			}
			e := ObligationEntry{ID: o.ID, Alias: o.Alias, Title: o.Title, Core: o.Core, State: ObligationOpen}
			stamp(&e)
			out.Entries = append(out.Entries, e)
		}
		if len(out.Entries) > wire.ObligationsMaxEntries {
			return nil, wire.Errorf(wire.CodeLimitExceeded, "/request/payload/obligations", "a ledger holds at most %d obligations", wire.ObligationsMaxEntries)
		}
		sort.Slice(out.Entries, func(i, j int) bool { return out.Entries[i].ID < out.Entries[j].ID })
	case OpObligationsSet:
		for _, c := range q.Set.Changes {
			e := out.Entry(c.ID)
			if e == nil {
				return nil, wire.Errorf(wire.CodeMalformed, "/request/payload/changes", "%s obligation %s is not in the ledger", ObligationUnknownDetail, c.ID)
			}
			if c.State != nil {
				if e.State == ObligationWitnessed && *c.State != ObligationWitnessed {
					e.Evidence = nil
				}
				e.State = *c.State
			}
			if c.Core != nil {
				e.Core = *c.Core
			}
			if c.Sources != nil {
				e.Sources = c.Sources
			}
			if c.Cites != nil {
				e.Cites = c.Cites
			}
			if c.Candidate != nil {
				cand := *c.Candidate
				cand.At = q.IssuedAt
				e.Candidate = &cand
			}
			if c.Scope != nil {
				if err := applyScopeChange(e, c.Scope, c.Reason, q); err != nil {
					return nil, err
				}
			}
			if c.Mirror != nil {
				if err := applyMirrorChange(e, c.Mirror, q); err != nil {
					return nil, err
				}
			}
			reason := c.Reason
			e.Reason = &reason
			stamp(e)
		}
	case OpObligationsWitness:
		w := q.Witness
		for _, c := range w.Credits {
			e := out.Entry(c.ID)
			if e == nil {
				return nil, wire.Errorf(wire.CodeMalformed, "/request/payload/credits", "%s obligation %s is not in the ledger", ObligationUnknownDetail, c.ID)
			}
			if w.Source == ObligationSourceReviewed {
				if err := applyReviewedWitness(e, w, eventSha256); err != nil {
					return nil, err
				}
				e.Candidate = nil
				stamp(e)
				continue
			}
			if e.State == ObligationWitnessed || e.State == ObligationDeferred {
				return nil, wire.Errorf(wire.CodeMalformed, "/request/payload/credits", "%s obligation %s is %s and is not credited", ObligationCreditMismatchDetail, c.ID, e.State)
			}
			e.State = ObligationWitnessed
			e.Reason = nil
			e.Candidate = nil
			e.Evidence = &ObligationEvidence{Source: w.Source, EventSha256: eventSha256, Commit: w.Commit, ReportSha256: w.ReportSha256,
				PlaywrightVersion: w.PlaywrightVersion, Matches: c.Matches, Declaration: w.Declaration, Provenance: w.Provenance, TestRepository: w.TestRepository}
			stamp(e)
		}
	}
	out.Unblocks += countUnblocks(l, out)
	out.Revision = ev.Revision.Int()
	out.Head = eventSha256
	out.Events = append(out.Events, eventSha256)
	return out, nil
}

func revisionOf(l *ObligationLedger) int64 {
	if l == nil {
		return 0
	}
	return l.Revision
}

// ChainError is a typed fold failure: the ledger is UNKNOWN, never empty or
// partial (TOL-V0-006).
type ChainError struct {
	Code   string // MISSING_EVIDENCE or MALFORMED
	Detail string
}

func (e *ChainError) Error() string { return e.Code + ": " + ObligationChainDetail + " " + e.Detail }

// FoldObligationChain walks the chain back from ref.Head through read, then
// folds it forward and checks the result against the reference: revision,
// prefix and counts. read returns (nil, nil) for an absent event.
func FoldObligationChain(id wire.TicketID, ref ObligationsReference, read func(wire.Digest) ([]byte, error)) (*ObligationLedger, error) {
	var chain []*ObligationEvent
	var digests []wire.Digest
	head := &ref.Head
	for head != nil {
		if len(chain) >= wire.ObligationsMaxEvents {
			return nil, &ChainError{wire.CodeMalformed, "chain is longer than the event bound"}
		}
		raw, err := read(*head)
		if err != nil {
			return nil, &ChainError{wire.CodeMissingEvidence, "event " + string(*head) + " is unreadable: " + err.Error()}
		}
		if raw == nil {
			return nil, &ChainError{wire.CodeMissingEvidence, "event " + string(*head) + " is missing from the evidence store"}
		}
		if wire.Sum(raw) != *head {
			return nil, &ChainError{wire.CodeMalformed, "event " + string(*head) + " fails its digest"}
		}
		ev, err := DecodeObligationEvent(raw)
		if err != nil {
			return nil, &ChainError{wire.CodeMalformed, "event " + string(*head) + " does not decode: " + err.Error()}
		}
		if ev.TicketID.Raw != id.Raw {
			return nil, &ChainError{wire.CodeMalformed, "event " + string(*head) + " belongs to another ticket"}
		}
		chain = append(chain, ev)
		digests = append(digests, *head)
		head = ev.Previous
	}
	var l *ObligationLedger
	for i := len(chain) - 1; i >= 0; i-- {
		next, err := ApplyObligationEvent(l, chain[i], digests[i])
		if err != nil {
			return nil, &ChainError{wire.CodeMalformed, "event " + string(digests[i]) + " does not fold: " + err.Error()}
		}
		l = next
	}
	if l == nil || l.Revision != ref.Revision.Int() || l.Prefix != ref.Prefix || l.Counts() != ref.Counts {
		return nil, &ChainError{wire.CodeMalformed, "the folded ledger does not match the record reference"}
	}
	return l, nil
}

// ObligationLedgerBeforeRequest folds the chain and, when one of its events
// retains the request requestID, returns the ledger as it stood before that
// event (nil before the first event) with found true. A writer that derives a
// request from ledger state uses it so a retry rebuilds the same bytes and
// reaches the TM-V0-006 request-id replay (TOL-V0-014).
func ObligationLedgerBeforeRequest(id wire.TicketID, ref ObligationsReference, read func(wire.Digest) ([]byte, error), requestID string) (*ObligationLedger, bool, error) {
	l, err := FoldObligationChain(id, ref, read)
	if err != nil {
		return nil, false, err
	}
	var before *ObligationLedger
	for _, d := range l.Events {
		raw, err := read(d)
		if err != nil {
			return nil, false, err
		}
		ev, err := DecodeObligationEvent(raw)
		if err != nil {
			return nil, false, &ChainError{wire.CodeMalformed, "event " + string(d) + " does not decode: " + err.Error()}
		}
		if ev.Decoded.RequestID == requestID {
			return before, true, nil
		}
		if before, err = ApplyObligationEvent(before, ev, d); err != nil {
			return nil, false, &ChainError{wire.CodeMalformed, "event " + string(d) + " does not fold: " + err.Error()}
		}
	}
	return nil, false, nil
}

// ObligationEventOf returns the decoded event when raw is a ledger event, so
// a journal reader can recognize the content-addressed blob kind.
func ObligationEventOf(raw []byte) (*ObligationEvent, bool) {
	if !bytes.Contains(raw, []byte(ObligationEventProfile)) {
		return nil, false
	}
	ev, err := DecodeObligationEvent(raw)
	return ev, err == nil
}

// ObligationChainEvents collects the ledger chain from ref.Head back to the
// first event, at most the event bound of reads. It stops at the first
// absent (nil), undecodable or already-visited event, so a corrupt chain
// that points back into itself cannot loop; FoldObligationChain then refuses
// with the typed chain error. A nil ref collects nothing.
func ObligationChainEvents(read func(wire.Digest) ([]byte, error), ref *ObligationsReference) (map[wire.Digest][]byte, error) {
	out := map[wire.Digest][]byte{}
	if ref == nil {
		return out, nil
	}
	head := &ref.Head
	for steps := 0; head != nil && steps < wire.ObligationsMaxEvents; steps++ {
		if _, seen := out[*head]; seen {
			return out, nil
		}
		raw, err := read(*head)
		if err != nil || raw == nil {
			return out, err
		}
		out[*head] = raw
		ev, err := DecodeObligationEvent(raw)
		if err != nil {
			return out, nil
		}
		head = ev.Previous
	}
	return out, nil
}
