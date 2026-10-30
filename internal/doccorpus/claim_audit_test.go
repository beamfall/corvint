package doccorpus

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// claimAuditFixture builds the adoption corpus, lets records choose verdicts against its claims,
// then commits those verdicts as one claim-audit provider and returns the audited manifest.
func claimAuditFixture(t *testing.T, edit func(*ProviderRecord), records func(base *Artifact, ev Anchor) []ClaimAuditRecord, raw func([]byte) []byte) (string, Manifest) {
	t.Helper()
	root, m := adoptionFixture(t, edit)
	base, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	ev := tkAnchor(t, root, m, m.Repository.Revision, "src/value.go", 4, 4)
	record := ProviderRecord{Schema: ClaimAuditSchema, ID: "audit", Version: "1", Source: m.Repository, ClaimAudits: &ClaimAuditSet{Records: records(base, ev)}}
	data, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if raw != nil {
		data = raw(data)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence/audit.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "evidence/audit.json")
	git(t, root, "commit", "-qm", "claim audit")
	rev := git(t, root, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, Provider{ID: "audit", Kind: "records", Version: "1", Revision: rev, Record: "evidence/audit.json"})
	m.Scopes = append(m.Scopes, Scope{"evidence/audit.json", rev})
	m.Inputs = append(m.Inputs, Input{Path: "evidence/audit.json", Revision: rev, Blob: git(t, root, "rev-parse", rev+":evidence/audit.json"), SHA256: Digest(data), Provider: "audit", Purpose: "provider"})
	return root, m
}

func auditClaim(t *testing.T, a *Artifact, id string) Claim {
	t.Helper()
	for _, c := range a.Claims {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no claim %s", id)
	return Claim{}
}

// auditVerdict is an independent human verdict pinned to the claim's current span.
func auditVerdict(t *testing.T, base *Artifact, id, claimID, verdict string, ev Anchor) ClaimAuditRecord {
	return ClaimAuditRecord{ID: "audit:" + id, ClaimID: claimID, SpanSHA256: ClaimAuditSpan(auditClaim(t, base, claimID)), Verdict: verdict, Reviewer: "reviewer-b", Independent: true, Method: "human", Evidence: []Anchor{ev}, Note: "checked against src/value.go"}
}

// auditGaps lists the gap kinds the gaps operation reports for one subject.
func auditGaps(t *testing.T, a *Artifact, id string) string {
	t.Helper()
	r, err := Query(a, Request{Operation: "gaps", ID: id, Limit: MaxResults}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	kinds := []string{}
	for _, x := range r.Results {
		if g, ok := x.(Gap); ok {
			kinds = append(kinds, g.Kind)
		}
	}
	return strings.Join(kinds, ",")
}

func auditBuild(t *testing.T, edit func(*ProviderRecord), records func(*Artifact, Anchor) []ClaimAuditRecord, raw func([]byte) []byte) (string, *Artifact, error) {
	t.Helper()
	root, m := claimAuditFixture(t, edit, records, raw)
	a, err := Build(context.Background(), root, m)
	return root, a, err
}

func one(t *testing.T, edit func(*testing.T, *Artifact, Anchor, *ClaimAuditRecord)) func(*Artifact, Anchor) []ClaimAuditRecord {
	return func(base *Artifact, ev Anchor) []ClaimAuditRecord {
		r := auditVerdict(t, base, "v1", "adapter:claim", "correct", ev)
		if edit != nil {
			edit(t, base, ev, &r)
		}
		return []ClaimAuditRecord{r}
	}
}

// DCP-V1-260: the claim-audit provider record is closed and bounded; an invalid verdict refuses
// the build rather than entering any figure.
func TestDCPV1260ClaimAuditSchemaIsClosed(t *testing.T) {
	t.Parallel()
	root, a, err := auditBuild(t, nil, one(t, nil), nil)
	if err != nil || len(a.ClaimAudits) != 1 || a.ClaimAudits[0].Status != "current" || !a.ClaimAudits[0].Headline {
		t.Fatalf("valid verdict not joined as headline: %+v %v", a, err)
	}
	raw, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), root, raw); err != nil {
		t.Fatalf("audited artifact does not rederive: %v", err)
	}
	for name, raw := range map[string]func([]byte) []byte{
		"unknown record member": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"verdict":"correct"`), []byte(`"verdict":"correct","trusted":true`), 1)
		},
		"unknown set member": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"claim_audits":{`), []byte(`"claim_audits":{"accuracy":1,`), 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := auditBuild(t, nil, one(t, nil), raw); err == nil || !strings.Contains(err.Error(), "corpus-refused") {
				t.Fatalf("unknown member admitted: %v", err)
			}
		})
	}
	for name, c := range map[string]struct {
		edit func(*ClaimAuditRecord)
		want string
	}{
		"verdict":     {func(r *ClaimAuditRecord) { r.Verdict = "mostly-correct" }, "unknown claim-audit verdict or method"},
		"method":      {func(r *ClaimAuditRecord) { r.Method = "vibes" }, "unknown claim-audit verdict or method"},
		"unqualified": {func(r *ClaimAuditRecord) { r.ID = "v1" }, "provider-qualified"},
		"span":        {func(r *ClaimAuditRecord) { r.SpanSHA256 = "abc" }, "claim_id and span_sha256"},
		"claim":       {func(r *ClaimAuditRecord) { r.ClaimID = "" }, "claim_id and span_sha256"},
		"reviewer":    {func(r *ClaimAuditRecord) { r.Reviewer = "" }, "reviewer or note"},
		"evidence": {func(r *ClaimAuditRecord) {
			for len(r.Evidence) <= 64 {
				r.Evidence = append(r.Evidence, r.Evidence[0])
			}
		}, "bounded list"},
		"anchor pin":     {func(r *ClaimAuditRecord) { r.Evidence[0].Start = 0 }, "evidence anchor"},
		"forged binding": {func(r *ClaimAuditRecord) { r.Evidence[0].SpanSHA256 = Digest([]byte("other")) }, "evidence anchor binding mismatch"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := auditBuild(t, nil, one(t, func(_ *testing.T, _ *Artifact, _ Anchor, r *ClaimAuditRecord) { c.edit(r) }), nil)
			tkRefused(t, err, c.want)
		})
	}
	t.Run("duplicate id", func(t *testing.T) {
		_, _, err := auditBuild(t, nil, func(base *Artifact, ev Anchor) []ClaimAuditRecord {
			r := auditVerdict(t, base, "v1", "adapter:claim", "correct", ev)
			return []ClaimAuditRecord{r, r}
		}, nil)
		tkRefused(t, err, "unique")
	})
	t.Run("empty set", func(t *testing.T) {
		_, _, err := auditBuild(t, nil, func(*Artifact, Anchor) []ClaimAuditRecord { return []ClaimAuditRecord{} }, nil)
		tkRefused(t, err, "record bound")
	})
}

// DCP-V1-261, DCP-V1-264: a verdict whose span hash no longer matches the claim is stale,
// reported as a gap, counted as stale, and never enters the headline; with no other verdict the
// report abstains instead of guessing.
func TestDCPV1261StaleClaimAuditVerdictIsExcluded(t *testing.T) {
	t.Parallel()
	_, a, err := auditBuild(t, nil, func(base *Artifact, ev Anchor) []ClaimAuditRecord {
		old := auditClaim(t, base, "adapter:claim")
		old.Text = "Count was a provider-declared capability"
		stale := auditVerdict(t, base, "stale", "adapter:claim", "correct", ev)
		stale.SpanSHA256 = ClaimAuditSpan(old)
		missing := auditVerdict(t, base, "missing", "adapter:claim", "correct", ev)
		missing.ClaimID = "adapter:retired-claim"
		return []ClaimAuditRecord{stale, missing}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	status := map[string]string{}
	for _, j := range a.ClaimAudits {
		status[j.ID] = j.Status
		if j.Headline {
			t.Fatalf("%s is in the headline: %+v", j.ID, j)
		}
	}
	if status["audit:stale"] != "stale" || status["audit:missing"] != "unresolved" {
		t.Fatalf("status = %v", status)
	}
	if got := auditGaps(t, a, "audit:stale"); got != "stale-claim-audit" {
		t.Fatalf("stale gap = %q", got)
	}
	if got := auditGaps(t, a, "audit:missing"); got != "unresolved-claim-audit" {
		t.Fatalf("unresolved gap = %q", got)
	}
	sample, err := SampleClaims(a, AuditSampleOptions{Seed: 7, Size: MaxAuditSample})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := Encode(sample)
	report, err := ReportAudit(a, sample, "sample.json", raw)
	if err != nil {
		t.Fatal(err)
	}
	if report.Overall.StaleVerdicts != 1 || report.Overall.Audited != 0 || report.Overall.Accuracy.Defined || report.Overall.Accuracy.Value != nil || report.Overall.Unaudited != report.Overall.Sampled {
		t.Fatalf("stale verdict leaked into the report: %+v", report.Overall)
	}
	if !strings.Contains(strings.Join(report.Limitations, "\n"), "abstained") {
		t.Fatalf("no abstention: %v", report.Limitations)
	}
}

// DCP-V1-262, DCP-V1-267: non-independent verdicts and the generator's own method never enter
// the headline accuracy; they are counted beside it with their own ratio.
func TestDCPV1262NonIndependentAndGeneratorVerdictsStayOutOfHeadline(t *testing.T) {
	t.Parallel()
	generated := func(p *ProviderRecord) { p.Claims[0].Evidence.Derivation = "generated" }
	_, a, err := auditBuild(t, generated, func(base *Artifact, ev Anchor) []ClaimAuditRecord {
		self := auditVerdict(t, base, "self", "adapter:claim", "correct", ev)
		self.Independent = false
		same := auditVerdict(t, base, "same-reviewer", "adapter:claim", "correct", ev)
		same.Reviewer = "adapter"
		model := auditVerdict(t, base, "model", "adapter:claim", "correct", ev)
		model.Method = "model"
		human := auditVerdict(t, base, "human", "adapter:claim", "incorrect", ev)
		return []ClaimAuditRecord{self, same, model, human}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	headline := map[string]bool{}
	for _, j := range a.ClaimAudits {
		headline[j.ID] = j.Headline
	}
	if headline["audit:self"] || headline["audit:same-reviewer"] || headline["audit:model"] || !headline["audit:human"] {
		t.Fatalf("headline = %v", headline)
	}
	sample, err := SampleClaims(a, AuditSampleOptions{Seed: 1, Size: MaxAuditSample})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := Encode(sample)
	report, err := ReportAudit(a, sample, "sample.json", raw)
	if err != nil {
		t.Fatal(err)
	}
	o := report.Overall
	if o.Audited != 1 || o.Outcomes.Incorrect != 1 || o.Outcomes.Correct != 0 || o.Accuracy.Numerator != 0 || o.Accuracy.Denominator != 1 || o.Accuracy.Value == nil || *o.Accuracy.Value != 0 {
		t.Fatalf("headline counted an excluded verdict: %+v", o)
	}
	if o.Excluded.NonIndependent != 2 || o.Excluded.GeneratorMethod != 1 || o.Excluded.Verdicts.Correct != 3 || o.Excluded.Accuracy.Denominator != 3 {
		t.Fatalf("excluded verdicts not reported beside the headline: %+v", o.Excluded)
	}
	if len(report.ByMethod) != 1 || report.ByMethod[0].Method != "human" {
		t.Fatalf("by_method counts excluded verdicts: %+v", report.ByMethod)
	}
}

// DCP-V1-265, DCP-V1-266: the same artifact, seed and options give the same sample; the report
// states its denominator and a 95% interval tied to the artifact identity.
func TestDCPV1265AuditSampleIsDeterministic(t *testing.T) {
	t.Parallel()
	_, a, err := auditBuild(t, nil, one(t, nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	population := len(a.Claims)
	if population < 3 {
		t.Fatalf("fixture has %d claims", population)
	}
	draw := func(o AuditSampleOptions) []byte {
		t.Helper()
		s, err := SampleClaims(a, o)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := Encode(s)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	options := AuditSampleOptions{Seed: 42, Size: 2, Stratify: "provider"}
	first := draw(options)
	if !bytes.Equal(first, draw(options)) {
		t.Fatal("same seed, artifact and options gave a different sample")
	}
	sample, err := ParseAuditSample(first)
	if err != nil {
		t.Fatal(err)
	}
	if sample.Design.Population != population || sample.Design.Sampled != 2 || sample.Corpus.ArtifactSHA256 != a.SHA256 || len(sample.Strata) == 0 {
		t.Fatalf("design = %+v", sample.Design)
	}
	sampled := 0
	for _, s := range sample.Strata {
		sampled += s.Sampled
	}
	if sampled != 2 {
		t.Fatalf("strata allocate %d", sampled)
	}
	differs := false
	for seed := uint64(0); seed < 16 && !differs; seed++ {
		differs = !bytes.Equal(draw(AuditSampleOptions{Seed: seed, Size: 2}), draw(AuditSampleOptions{Seed: 42, Size: 2}))
	}
	if !differs {
		t.Fatal("the seed does not affect the sample")
	}
	scoped, err := SampleClaims(a, AuditSampleOptions{Seed: 1, Size: MaxAuditSample, Scope: "no/such/dir"})
	if err != nil || scoped.Design.Population != 0 || len(scoped.Claims) != 0 {
		t.Fatalf("scope ignored: %+v %v", scoped, err)
	}
	for _, bad := range []AuditSampleOptions{{Size: 0}, {Size: MaxAuditSample + 1}, {Size: 1, Seed: MaxAuditSeed + 1}, {Size: 1, Stratify: "author"}, {Size: 1, Scope: "../x"}} {
		if _, err := SampleClaims(a, bad); err == nil {
			t.Fatalf("%+v admitted", bad)
		}
	}
	all, err := SampleClaims(a, AuditSampleOptions{Seed: 3, Size: MaxAuditSample, Stratify: "kind"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := Encode(all)
	report, err := ReportAudit(a, all, "sample.json", raw)
	if err != nil {
		t.Fatal(err)
	}
	acc := report.Overall.Accuracy
	if report.Overall.Sampled != population || acc.Denominator != 1 || acc.Numerator != 1 || len(acc.Interval95) != 2 || acc.Interval95[1] != 1 || acc.Interval95[0] < 0.2 || acc.Interval95[0] > 0.21 {
		t.Fatalf("accuracy = %+v", acc)
	}
	if report.Sample.SHA256 != Digest(raw) || report.Corpus.ArtifactSHA256 != a.SHA256 || report.Overall.Unaudited != population-1 {
		t.Fatalf("report binding = %+v", report.Sample)
	}
	other := *all
	other.Corpus.Profile = "another-profile"
	if _, err := ReportAudit(a, &other, "sample.json", raw); err == nil {
		t.Fatal("a sample from another corpus was reported")
	}
	if _, err := ParseAuditSample(bytes.Replace(raw, []byte(`"schema":"`+AuditSampleSchema+`"`), []byte(`"schema":"`+AuditSampleSchema+`","weight":1`), 1)); err == nil {
		t.Fatal("unknown sample member admitted")
	}
	if !strings.Contains(string(RenderAuditTable(report)), "1.0000 (1/1)") {
		t.Fatalf("table:\n%s", RenderAuditTable(report))
	}
}

// DCP-V1-263, DCP-V1-264: claim views show the current verdict without changing the claim or
// its trust label, and a current incorrect verdict is a gap on the claim and its subject.
func TestDCPV1263ClaimAuditViewsKeepTrust(t *testing.T) {
	t.Parallel()
	_, a, err := auditBuild(t, nil, one(t, func(_ *testing.T, _ *Artifact, _ Anchor, r *ClaimAuditRecord) { r.Verdict = "incorrect" }), nil)
	if err != nil {
		t.Fatal(err)
	}
	root, m := adoptionFixture(t, nil)
	plain, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	want := auditClaim(t, plain, "adapter:claim")
	for _, op := range []Request{{Operation: "get", ID: "adapter:claim"}, {Operation: "trace", ID: "adapter:claim"}, {Operation: "claims", Path: want.Evidence.Anchors[0].Path}} {
		r, err := Query(a, op, "fresh", nil)
		if err != nil {
			t.Fatal(err)
		}
		views := r.ClaimAudits["adapter:claim"]
		if len(views) != 1 || views[0].Verdict != "incorrect" || views[0].Status != "current" || !views[0].Headline {
			t.Fatalf("%s: claim audit views = %+v", op.Operation, r.ClaimAudits)
		}
		for _, x := range r.Results {
			if c, ok := x.(Claim); ok && c.ID == want.ID && (c.Evidence.Trust != want.Evidence.Trust || c.Text != want.Text || c.Evidence.State != want.Evidence.State) {
				t.Fatalf("%s: verdict changed the claim: %+v", op.Operation, c)
			}
		}
		if r.Trust != "generated" {
			t.Fatalf("%s: receipt trust = %s", op.Operation, r.Trust)
		}
	}
	for _, id := range []string{"adapter:claim", want.Subject} {
		if got := auditGaps(t, a, id); !strings.Contains(got, "claim-audit-incorrect") {
			t.Fatalf("%s gaps = %q", id, got)
		}
	}
}
