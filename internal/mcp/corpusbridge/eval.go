package corpusbridge

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// Labelled corpus evaluation (DCP-V1-250..257). Every case is answered by CallTimed, the
// code path that serves the corpus MCP tools, so a passing case is a passing served answer.
const (
	EvalCasesSchema   = "corvint-corpus-eval-cases/1"
	EvalReceiptSchema = "corvint-corpus-eval/1"
	// MaxEvalCases bounds one case file (DCP-V1-250).
	MaxEvalCases = 1024
	maxEvalID    = 256
)

// evalOrigins are the closed case origins; only "labelled" cases are human-labelled.
var evalOrigins = map[string]bool{"labelled": true, "synthetic": true, "generated": true}

// EvalExpect is a case's expectation: an answer (subjects, claims or paths) or an abstention.
type EvalExpect struct {
	Subjects []string `json:"subjects,omitempty"`
	Claims   []string `json:"claims,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Abstain  bool     `json:"abstain,omitempty"`
	MustNot  []string `json:"must_not,omitempty"`
}

// EvalCase is one caller-owned labelled case; arguments are the MCP tool arguments verbatim.
type EvalCase struct {
	ID        string         `json:"id"`
	Operation string         `json:"operation"`
	Arguments jsontext.Value `json:"arguments,omitempty"`
	Expect    EvalExpect     `json:"expect"`
	Tags      []string       `json:"tags,omitempty"`
	Origin    string         `json:"origin,omitempty"`
}

// EvalCases is the closed case file.
type EvalCases struct {
	Schema string     `json:"schema"`
	Origin string     `json:"origin"`
	Cases  []EvalCase `json:"cases"`
}

// EvalRatio is a measured proportion with its explicit numerator and denominator; Value is
// null when the denominator is zero.
type EvalRatio struct {
	N     int      `json:"n"`
	D     int      `json:"d"`
	Value *float64 `json:"value"`
}

func ratio(n, d int) EvalRatio {
	r := EvalRatio{N: n, D: d}
	if d > 0 {
		v := float64(n) / float64(d)
		r.Value = &v
	}
	return r
}

// EvalUnresolved names an expected or must_not item absent from the artifact.
type EvalUnresolved struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// EvalResult is one case's measured outcome.
type EvalResult struct {
	ID                string           `json:"id"`
	Operation         string           `json:"operation"`
	Tool              string           `json:"tool"`
	Origin            string           `json:"origin"`
	Tags              []string         `json:"tags"`
	Status            string           `json:"status"`
	ErrorCode         string           `json:"error_code,omitempty"`
	ExpectAbstain     bool             `json:"expect_abstain"`
	Hit               bool             `json:"hit"`
	Rank              *int             `json:"rank"`
	K                 int              `json:"k"`
	Returned          int              `json:"returned"`
	PrecisionAtK      EvalRatio        `json:"precision_at_k"`
	RecallAtK         EvalRatio        `json:"recall_at_k"`
	Abstained         bool             `json:"abstained"`
	Abstention        string           `json:"abstention"`
	MustNotViolations []string         `json:"must_not_violations"`
	Unresolved        []EvalUnresolved `json:"unresolved"`
	LatencyMS         int64            `json:"latency_ms"`
	ReceiptBytes      int              `json:"receipt_bytes"`
}

// EvalGroup summarizes the cases of one operation, tag or origin, naming them.
type EvalGroup struct {
	CaseIDs           []string  `json:"case_ids"`
	Cases             int       `json:"cases"`
	Passed            int       `json:"passed"`
	Failed            int       `json:"failed"`
	Errors            int       `json:"errors"`
	Hit               EvalRatio `json:"hit"`
	Abstention        EvalRatio `json:"abstention"`
	PrecisionAtK      EvalRatio `json:"precision_at_k"`
	RecallAtK         EvalRatio `json:"recall_at_k"`
	MustNotViolations int       `json:"must_not_violations"`
	Unresolved        int       `json:"unresolved"`
}

// EvalSummary groups the results overall and per operation, tag and origin.
type EvalSummary struct {
	Overall     EvalGroup            `json:"overall"`
	ByOperation map[string]EvalGroup `json:"by_operation"`
	ByTag       map[string]EvalGroup `json:"by_tag"`
	ByOrigin    map[string]EvalGroup `json:"by_origin"`
}

// EvalCorpus is the evaluated artifact's identity.
type EvalCorpus struct {
	ArtifactSHA256 string            `json:"artifact_sha256"`
	RepositoryID   string            `json:"repository_id"`
	SourceRevision string            `json:"source_revision"`
	Tree           string            `json:"tree"`
	Profile        doccorpus.Profile `json:"profile"`
	ProfileSHA256  string            `json:"profile_sha256"`
	ManifestSHA256 string            `json:"manifest_sha256"`
	Builder        doccorpus.Builder `json:"builder"`
}

// EvalIdentity is what two receipts must share to be compared (DCP-V1-255): the exact case
// file and the corpus identity (repository, profile and served receipt schema). A rebuilt
// artifact of the same corpus stays comparable; that is the point of the ratchet.
type EvalIdentity struct {
	CasesSHA256   string `json:"cases_sha256"`
	RepositoryID  string `json:"repository_id"`
	ProfileID     string `json:"profile_id"`
	ReceiptSchema string `json:"receipt_schema"`
}

// EvalCaseFile names the case file that was scored.
type EvalCaseFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Origin string `json:"origin"`
	Cases  int    `json:"cases"`
}

// EvalChange is one case whose outcome moved against the baseline.
type EvalChange struct {
	ID      string   `json:"id"`
	Reasons []string `json:"reasons"`
}

// EvalComparison is the --baseline ratchet outcome.
type EvalComparison struct {
	Path          string       `json:"path"`
	SHA256        string       `json:"sha256"`
	Comparable    bool         `json:"comparable"`
	NotComparable []string     `json:"not_comparable"`
	Regressed     []EvalChange `json:"regressed"`
	Improved      []EvalChange `json:"improved"`
	Unchanged     int          `json:"unchanged"`
}

// EvalReport is the corvint-corpus-eval/1 receipt.
type EvalReport struct {
	Schema      string          `json:"schema"`
	CaseFile    EvalCaseFile    `json:"case_file"`
	Corpus      EvalCorpus      `json:"corpus"`
	Identity    EvalIdentity    `json:"comparison_identity"`
	Cases       []EvalResult    `json:"cases"`
	Summary     EvalSummary     `json:"summary"`
	Baseline    *EvalComparison `json:"baseline,omitempty"`
	Limitations []string        `json:"limitations"`
}

// Regressed reports whether the ratchet must fail: a regression or a refused comparison.
func (r *EvalReport) Regressed() bool {
	return r.Baseline != nil && (!r.Baseline.Comparable || len(r.Baseline.Regressed) > 0)
}

func evalRefused(message string) error {
	return &doccorpus.Error{Code: "corpus-refused", Message: "eval: " + message}
}

// ToolFor names the MCP tool that serves a corpus operation.
func ToolFor(op string) (string, bool) {
	for name, known := range tools {
		if known == op {
			return name, true
		}
	}
	return "", false
}

// ParseEvalCases admits a closed case file (DCP-V1-250). Any malformed case refuses the file:
// an evaluation never silently drops a case.
func ParseEvalCases(raw []byte) (EvalCases, error) {
	var cases EvalCases
	if err := json.Unmarshal(raw, &cases, json.RejectUnknownMembers(true)); err != nil {
		return cases, evalRefused("case file is not closed " + EvalCasesSchema + " JSON")
	}
	if cases.Schema != EvalCasesSchema {
		return cases, evalRefused("case file schema must be " + EvalCasesSchema)
	}
	if !evalOrigins[cases.Origin] {
		return cases, evalRefused("case file origin must be labelled, synthetic or generated")
	}
	if len(cases.Cases) == 0 || len(cases.Cases) > MaxEvalCases {
		return cases, evalRefused(fmt.Sprintf("case file needs 1..%d cases", MaxEvalCases))
	}
	seen := map[string]bool{}
	for _, c := range cases.Cases {
		if c.ID == "" || len(c.ID) > maxEvalID || seen[c.ID] {
			return cases, evalRefused("case IDs must be unique, non-empty and at most 256 bytes")
		}
		seen[c.ID] = true
		where := "case " + c.ID + ": "
		if _, ok := ToolFor(c.Operation); !ok {
			return cases, evalRefused(where + "unsupported corpus operation " + c.Operation)
		}
		if len(c.Arguments) > 0 && c.Arguments.Kind() != '{' {
			return cases, evalRefused(where + "arguments must be a JSON object")
		}
		if c.Origin != "" && (!evalOrigins[c.Origin] || c.Origin == "labelled" && cases.Origin != "labelled") {
			return cases, evalRefused(where + "origin must be synthetic or generated, or labelled in a labelled file")
		}
		e := c.Expect
		answers := len(e.Subjects) + len(e.Claims) + len(e.Paths)
		if e.Abstain == (answers > 0) {
			return cases, evalRefused(where + "expect needs either abstain or expected subjects, claims or paths")
		}
		for _, list := range [][]string{e.Subjects, e.Claims, e.Paths, e.MustNot, c.Tags} {
			if !distinctNonEmpty(list) {
				return cases, evalRefused(where + "expected IDs, paths and tags must be distinct and non-empty")
			}
		}
	}
	return cases, nil
}

func distinctNonEmpty(list []string) bool {
	seen := map[string]bool{}
	for _, item := range list {
		if item == "" || len(item) > 1024 || seen[item] {
			return false
		}
		seen[item] = true
	}
	return true
}

// loadedArtifact returns the memoized, rederived artifact the tools answer from.
func (r *Registry) loadedArtifact(ctx context.Context) (*doccorpus.Artifact, error) {
	raw, err := doccorpus.ReadCorpusFile(r.root, r.path)
	if err != nil {
		return nil, err
	}
	if doccorpus.Digest(raw) != r.digest {
		return nil, evalRefused("configured artifact changed")
	}
	return r.artifactFor(ctx, raw, &Timing{})
}

// Evaluate scores every case through CallTimed (DCP-V1-251..253).
func (r *Registry) Evaluate(ctx context.Context, cases EvalCases, casesPath string, casesRaw []byte) (*EvalReport, error) {
	a, err := r.loadedArtifact(ctx)
	if err != nil {
		return nil, err
	}
	known := evalUniverse(a)
	report := &EvalReport{
		Schema:   EvalReceiptSchema,
		CaseFile: EvalCaseFile{Path: casesPath, SHA256: doccorpus.Digest(casesRaw), Origin: cases.Origin, Cases: len(cases.Cases)},
		Corpus: EvalCorpus{ArtifactSHA256: a.SHA256, RepositoryID: a.Manifest.Repository.ID, SourceRevision: a.Manifest.Repository.Revision, Tree: a.Tree,
			Profile: a.Manifest.Profile, ProfileSHA256: a.ProfileSHA256, ManifestSHA256: a.ManifestSHA256, Builder: a.Builder},
		Limitations: []string{"cases are caller-owned; a score measures only these cases, never corpus completeness or external utility",
			"latency_ms is wall time on this host and is never compared by the ratchet"},
	}
	report.Identity = EvalIdentity{CasesSHA256: report.CaseFile.SHA256, RepositoryID: a.Manifest.Repository.ID, ProfileID: a.Manifest.Profile.ID, ReceiptSchema: doccorpus.ReceiptSchema}
	for _, c := range cases.Cases {
		result, err := r.evaluateCase(ctx, c, cases.Origin, known)
		if err != nil {
			return nil, err
		}
		report.Cases = append(report.Cases, result)
	}
	report.Summary = summarize(report.Cases)
	return report, nil
}

type evalKnown struct{ subjects, claims, records, paths map[string]bool }

func evalUniverse(a *doccorpus.Artifact) evalKnown {
	k := evalKnown{subjects: map[string]bool{}, claims: map[string]bool{}, records: map[string]bool{}, paths: map[string]bool{}}
	anchors := func(e doccorpus.Evidence) {
		for _, anchor := range e.Anchors {
			k.paths[anchor.Path] = true
		}
	}
	for _, in := range a.Manifest.Inputs {
		k.paths[in.Path] = true
	}
	for _, s := range a.Subjects {
		k.subjects[s.ID], k.records[s.ID] = true, true
		anchors(s.Evidence)
	}
	for _, c := range a.Claims {
		k.claims[c.ID], k.records[c.ID] = true, true
		anchors(c.Evidence)
	}
	for _, rel := range a.Relations {
		k.records[rel.ID] = true
		anchors(rel.Evidence)
	}
	return k
}

func (r *Registry) evaluateCase(ctx context.Context, c EvalCase, fileOrigin string, known evalKnown) (EvalResult, error) {
	tool, _ := ToolFor(c.Operation)
	origin := c.Origin
	if origin == "" {
		origin = fileOrigin
	}
	result := EvalResult{ID: c.ID, Operation: c.Operation, Tool: tool, Origin: origin, Tags: append([]string{}, c.Tags...), ExpectAbstain: c.Expect.Abstain,
		MustNotViolations: []string{}, Unresolved: []EvalUnresolved{}, K: 20}
	sort.Strings(result.Tags)
	for _, group := range []struct {
		kind  string
		ids   []string
		known map[string]bool
	}{{"subject", c.Expect.Subjects, known.subjects}, {"claim", c.Expect.Claims, known.claims}, {"path", c.Expect.Paths, known.paths}, {"must_not", c.Expect.MustNot, known.records}} {
		for _, id := range group.ids {
			if !group.known[id] {
				result.Unresolved = append(result.Unresolved, EvalUnresolved{Kind: group.kind, ID: id})
			}
		}
	}
	arguments := []byte(c.Arguments)
	if len(arguments) == 0 {
		arguments = []byte("{}")
	}
	var limit struct {
		Limit *int `json:"limit"`
	}
	if json.Unmarshal(arguments, &limit) == nil && limit.Limit != nil {
		result.K = *limit.Limit
	}
	expected := len(c.Expect.Subjects) + len(c.Expect.Claims) + len(c.Expect.Paths)
	result.RecallAtK = ratio(0, expected)
	structured, text, failure, transport, timing := r.CallTimed(ctx, tool, arguments)
	result.LatencyMS, result.ReceiptBytes = timing.TotalMS, len(text)
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if transport != nil || failure != nil {
		result.Status = "error"
		if transport != nil {
			result.ErrorCode = transport.Code
		} else {
			result.ErrorCode = failure.Code
		}
		result.Abstention = "not-run"
		return result, nil
	}
	rows, _ := structured["results"].([]any)
	if len(rows) > result.K {
		rows = rows[:result.K]
	}
	result.Returned = len(rows)
	result.Abstained = len(rows) == 0
	wantSubjects, wantClaims, wantPaths, mustNot := set(c.Expect.Subjects), set(c.Expect.Claims), set(c.Expect.Paths), set(c.Expect.MustNot)
	found, violated := map[string]bool{}, map[string]bool{}
	relevant := 0
	for index, row := range rows {
		ids, paths := rowIdentity(row)
		hit := false
		for _, id := range ids {
			if wantSubjects[id] {
				found["subject\x00"+id], hit = true, true
			}
			if wantClaims[id] {
				found["claim\x00"+id], hit = true, true
			}
			if mustNot[id] {
				violated[id] = true
			}
		}
		for _, path := range paths {
			if wantPaths[path] {
				found["path\x00"+path] = true
				hit = true
			}
		}
		if hit {
			relevant++
			if result.Rank == nil {
				rank := index + 1
				result.Rank = &rank
			}
		}
	}
	result.PrecisionAtK = ratio(relevant, len(rows))
	result.RecallAtK = ratio(len(found), expected)
	result.Hit = result.Rank != nil
	for id := range violated {
		result.MustNotViolations = append(result.MustNotViolations, id)
	}
	sort.Strings(result.MustNotViolations)
	switch {
	case c.Expect.Abstain && result.Abstained:
		result.Abstention = "correct"
	case c.Expect.Abstain:
		result.Abstention = "false-answer"
	case result.Abstained:
		result.Abstention = "missed-answer"
	default:
		result.Abstention = "answered"
	}
	ok := len(result.MustNotViolations) == 0 && len(result.Unresolved) == 0
	if c.Expect.Abstain {
		ok = ok && result.Abstained
	} else {
		ok = ok && result.Hit
	}
	result.Status = "fail"
	if ok {
		result.Status = "pass"
	}
	return result, nil
}

func set(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item] = true
	}
	return out
}

// rowIdentity reads one served result row's record IDs (its id and subject, and those of a
// nested relation) and the paths it cites (its path and evidence anchors).
func rowIdentity(row any) (ids, paths []string) {
	object, _ := row.(map[string]any)
	collect := func(o map[string]any) {
		for _, key := range []string{"id", "subject"} {
			if value, ok := o[key].(string); ok && value != "" {
				ids = append(ids, value)
			}
		}
		if value, ok := o["path"].(string); ok && value != "" {
			paths = append(paths, value)
		}
		if evidence, ok := o["evidence"].(map[string]any); ok {
			anchors, _ := evidence["anchors"].([]any)
			for _, raw := range anchors {
				if anchor, ok := raw.(map[string]any); ok {
					if value, ok := anchor["path"].(string); ok && value != "" {
						paths = append(paths, value)
					}
				}
			}
		}
	}
	if object == nil {
		return nil, nil
	}
	collect(object)
	if relation, ok := object["relation"].(map[string]any); ok {
		collect(relation)
	}
	return ids, paths
}

func summarize(results []EvalResult) EvalSummary {
	s := EvalSummary{ByOperation: map[string]EvalGroup{}, ByTag: map[string]EvalGroup{}, ByOrigin: map[string]EvalGroup{}}
	type tally struct {
		group                                            EvalGroup
		hitN, hitD, absN, absD, precN, precD, recN, recD int
	}
	all := &tally{}
	ops, tags, origins := map[string]*tally{}, map[string]*tally{}, map[string]*tally{}
	get := func(m map[string]*tally, key string) *tally {
		if m[key] == nil {
			m[key] = &tally{}
		}
		return m[key]
	}
	for _, r := range results {
		targets := []*tally{all, get(ops, r.Operation), get(origins, r.Origin)}
		for _, tag := range r.Tags {
			targets = append(targets, get(tags, tag))
		}
		for _, t := range targets {
			t.group.CaseIDs = append(t.group.CaseIDs, r.ID)
			t.group.Cases++
			switch r.Status {
			case "pass":
				t.group.Passed++
			case "fail":
				t.group.Failed++
			default:
				t.group.Errors++
			}
			// An errored case stays in every denominator as a miss (DCP-V1-253).
			if r.ExpectAbstain {
				t.absD++
				if r.Abstention == "correct" {
					t.absN++
				}
			} else {
				t.hitD++
				if r.Hit {
					t.hitN++
				}
			}
			t.precN, t.precD = t.precN+r.PrecisionAtK.N, t.precD+r.PrecisionAtK.D
			t.recN, t.recD = t.recN+r.RecallAtK.N, t.recD+r.RecallAtK.D
			t.group.MustNotViolations += len(r.MustNotViolations)
			t.group.Unresolved += len(r.Unresolved)
		}
	}
	finish := func(t *tally) EvalGroup {
		g := t.group
		sort.Strings(g.CaseIDs)
		g.Hit, g.Abstention = ratio(t.hitN, t.hitD), ratio(t.absN, t.absD)
		g.PrecisionAtK, g.RecallAtK = ratio(t.precN, t.precD), ratio(t.recN, t.recD)
		return g
	}
	s.Overall = finish(all)
	for key, t := range ops {
		s.ByOperation[key] = finish(t)
	}
	for key, t := range tags {
		s.ByTag[key] = finish(t)
	}
	for key, t := range origins {
		s.ByOrigin[key] = finish(t)
	}
	return s
}

// CompareEval applies the --baseline ratchet (DCP-V1-254, DCP-V1-255): a receipt whose case
// file or corpus identity differs is not comparable; otherwise every regressed case is listed,
// and improvements never offset a regression.
func CompareEval(current *EvalReport, baselinePath string, baselineRaw []byte) error {
	var baseline EvalReport
	if err := json.Unmarshal(baselineRaw, &baseline, json.RejectUnknownMembers(true)); err != nil || baseline.Schema != EvalReceiptSchema {
		return evalRefused("baseline is not a " + EvalReceiptSchema + " receipt")
	}
	c := &EvalComparison{Path: baselinePath, SHA256: doccorpus.Digest(baselineRaw), NotComparable: []string{}, Regressed: []EvalChange{}, Improved: []EvalChange{}}
	current.Baseline = c
	if baseline.Identity.CasesSHA256 != current.Identity.CasesSHA256 {
		c.NotComparable = append(c.NotComparable, "case file changed")
	}
	if baseline.Identity.RepositoryID != current.Identity.RepositoryID || baseline.Identity.ProfileID != current.Identity.ProfileID {
		c.NotComparable = append(c.NotComparable, "corpus identity changed")
	}
	if baseline.Identity.ReceiptSchema != current.Identity.ReceiptSchema {
		c.NotComparable = append(c.NotComparable, "served receipt schema changed")
	}
	before := map[string]EvalResult{}
	for _, r := range baseline.Cases {
		before[r.ID] = r
	}
	if len(c.NotComparable) == 0 && len(before) != len(current.Cases) {
		c.NotComparable = append(c.NotComparable, "baseline cases do not match its case file identity")
	}
	if len(c.NotComparable) > 0 {
		return nil
	}
	for _, now := range current.Cases {
		then, ok := before[now.ID]
		if !ok {
			c.NotComparable = append(c.NotComparable, "baseline cases do not match its case file identity")
			c.Regressed, c.Improved, c.Unchanged = []EvalChange{}, []EvalChange{}, 0
			return nil
		}
		worse, better := caseDelta(then, now)
		switch {
		case len(worse) > 0:
			c.Regressed = append(c.Regressed, EvalChange{ID: now.ID, Reasons: worse})
		case len(better) > 0:
			c.Improved = append(c.Improved, EvalChange{ID: now.ID, Reasons: better})
		default:
			c.Unchanged++
		}
	}
	c.Comparable = true
	return nil
}

// caseDelta compares one case's quality fields; latency and bytes are never ratcheted.
func caseDelta(then, now EvalResult) (worse, better []string) {
	compare := func(name string, before, after int, higherIsBetter bool) {
		if before == after {
			return
		}
		if (after > before) == higherIsBetter {
			better = append(better, name)
		} else {
			worse = append(worse, name)
		}
	}
	statusRank := map[string]int{"pass": 2, "fail": 1, "error": 0}
	compare("status "+then.Status+" -> "+now.Status, statusRank[then.Status], statusRank[now.Status], true)
	rank := func(r *int) int {
		if r == nil {
			return 1 << 30
		}
		return *r
	}
	compare("rank", rank(then.Rank), rank(now.Rank), false)
	compare("recall_at_k", then.RecallAtK.N, now.RecallAtK.N, true)
	compare("precision_at_k", then.PrecisionAtK.N*max(now.PrecisionAtK.D, 1), now.PrecisionAtK.N*max(then.PrecisionAtK.D, 1), true)
	compare("must_not_violations", len(then.MustNotViolations), len(now.MustNotViolations), false)
	correct := func(r EvalResult) int {
		if r.Abstention == "correct" || r.Abstention == "answered" {
			return 1
		}
		return 0
	}
	compare("abstention "+then.Abstention+" -> "+now.Abstention, correct(then), correct(now), true)
	return worse, better
}

// RenderEvalTable is the --format table view of a receipt.
func RenderEvalTable(r *EvalReport) []byte {
	var b strings.Builder
	value := func(x EvalRatio) string {
		if x.Value == nil {
			return fmt.Sprintf("-(%d/%d)", x.N, x.D)
		}
		return fmt.Sprintf("%.3f(%d/%d)", *x.Value, x.N, x.D)
	}
	fmt.Fprintf(&b, "corpus %s profile %s artifact %s cases %s (%s)\n", r.Corpus.RepositoryID, r.Corpus.Profile.ID, r.Corpus.ArtifactSHA256, r.CaseFile.SHA256, r.CaseFile.Origin)
	fmt.Fprintf(&b, "%-32s %-10s %-9s %-6s %-5s %-14s %-14s %-13s %s\n", "CASE", "OPERATION", "ORIGIN", "STATUS", "RANK", "P@K", "R@K", "ABSTENTION", "NOTES")
	for _, c := range r.Cases {
		rank := "-"
		if c.Rank != nil {
			rank = fmt.Sprint(*c.Rank)
		}
		notes := []string{}
		if c.ErrorCode != "" {
			notes = append(notes, "error="+c.ErrorCode)
		}
		if len(c.MustNotViolations) > 0 {
			notes = append(notes, "must_not="+strings.Join(c.MustNotViolations, ","))
		}
		for _, u := range c.Unresolved {
			notes = append(notes, "unresolved "+u.Kind+"="+u.ID)
		}
		fmt.Fprintf(&b, "%-32s %-10s %-9s %-6s %-5s %-14s %-14s %-13s %s\n", c.ID, c.Operation, c.Origin, c.Status, rank, value(c.PrecisionAtK), value(c.RecallAtK), c.Abstention, strings.Join(notes, " "))
	}
	group := func(label string, g EvalGroup) {
		fmt.Fprintf(&b, "%-24s cases=%d pass=%d fail=%d error=%d hit=%s abstention=%s p@k=%s r@k=%s must_not=%d unresolved=%d\n", label, g.Cases, g.Passed, g.Failed, g.Errors, value(g.Hit), value(g.Abstention), value(g.PrecisionAtK), value(g.RecallAtK), g.MustNotViolations, g.Unresolved)
	}
	group("overall", r.Summary.Overall)
	for _, section := range []struct {
		prefix string
		groups map[string]EvalGroup
	}{{"operation", r.Summary.ByOperation}, {"tag", r.Summary.ByTag}, {"origin", r.Summary.ByOrigin}} {
		keys := make([]string, 0, len(section.groups))
		for key := range section.groups {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			group(section.prefix+" "+key, section.groups[key])
		}
	}
	if c := r.Baseline; c != nil {
		if !c.Comparable {
			fmt.Fprintf(&b, "baseline %s NOT COMPARABLE: %s\n", c.Path, strings.Join(c.NotComparable, "; "))
		} else {
			fmt.Fprintf(&b, "baseline %s regressed=%d improved=%d unchanged=%d\n", c.Path, len(c.Regressed), len(c.Improved), c.Unchanged)
		}
		for _, change := range c.Regressed {
			fmt.Fprintf(&b, "REGRESSED %s: %s\n", change.ID, strings.Join(change.Reasons, "; "))
		}
		for _, change := range c.Improved {
			fmt.Fprintf(&b, "improved %s: %s\n", change.ID, strings.Join(change.Reasons, "; "))
		}
	}
	return []byte(b.String())
}
