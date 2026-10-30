package doccorpus

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// ClaimAuditSchema is the closed provider record for independent claim-audit verdicts
// (DCP-V1-260..262). A verdict measures correctness; it never promotes a trust label.
const ClaimAuditSchema = "corvint-corpus-claim-audit/1"

// MaxClaimAudits bounds claim-audit verdicts per corpus (DCP-V1-260).
const MaxClaimAudits = 4096

// maxClaimAuditViews bounds the verdict views a receipt attaches to one claim (DCP-V1-263).
const maxClaimAuditViews = 32

const claimAuditLimitation = "provider-reported verdict; the corpus does not authenticate reviewers, and a verdict never changes a trust label"

var (
	claimAuditVerdicts = words("correct incorrect unsupported-by-citation unverifiable")
	claimAuditMethods  = words("human model test")
)

type ClaimAuditSet struct {
	Records []ClaimAuditRecord `json:"records"`
}

// ClaimAuditRecord is one verdict on one claim. SpanSHA256 is the claim's audit span hash as
// `audit sample` emits it (ClaimAuditSpan); a later mismatch makes the verdict stale.
type ClaimAuditRecord struct {
	ID          string   `json:"id"`
	ClaimID     string   `json:"claim_id"`
	SpanSHA256  string   `json:"span_sha256"`
	Verdict     string   `json:"verdict"`
	Reviewer    string   `json:"reviewer"`
	Independent bool     `json:"reviewer_independent_of_generator"`
	Method      string   `json:"method"`
	Evidence    []Anchor `json:"evidence"`
	Note        string   `json:"note,omitempty"`
}

// ClaimAuditJoin is the build-time join of one verdict, kept as a separate artifact member:
// the audited claim is never rewritten (DCP-V1-261).
type ClaimAuditJoin struct {
	ID                string           `json:"id"`
	Provider          string           `json:"provider"`
	ClaimID           string           `json:"claim_id"`
	Status            string           `json:"status"`
	Verdict           string           `json:"verdict"`
	Method            string           `json:"method"`
	Independent       bool             `json:"independent"`
	GeneratorMethod   bool             `json:"generator_method"`
	Headline          bool             `json:"headline"`
	CurrentSpanSHA256 string           `json:"current_span_sha256"`
	Reasons           []string         `json:"reasons"`
	Limitations       []string         `json:"limitations"`
	Record            ClaimAuditRecord `json:"record"`
}

// ClaimAuditView is the compact verdict a receipt shows beside a claim (DCP-V1-263).
type ClaimAuditView struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	Verdict     string   `json:"verdict"`
	Method      string   `json:"method"`
	Reviewer    string   `json:"reviewer"`
	Independent bool     `json:"independent"`
	Headline    bool     `json:"headline"`
	Reasons     []string `json:"reasons"`
}

type providerClaimAudits struct {
	provider string
	set      *ClaimAuditSet
}

// ClaimAuditSpan is the claim's audit span hash: SHA-256 over the claim text and, in order,
// each evidence anchor's path, line range and span hash. A changed claim text or cited span
// changes it (DCP-V1-261).
func ClaimAuditSpan(claim Claim) string {
	var b strings.Builder
	b.WriteString(ClaimAuditSchema + "\n" + claim.ID + "\n" + claim.Text + "\n")
	for _, a := range claim.Evidence.Anchors {
		b.WriteString(a.Repository + "\x00" + a.Path + "\x00" + strconv.Itoa(a.Start) + "\x00" + strconv.Itoa(a.End) + "\x00" + a.SpanSHA256 + "\n")
	}
	return Digest([]byte(b.String()))
}

// admitClaimAudits applies the schema-only admission rules (DCP-V1-260).
func (c *compiler) admitClaimAudits(p Provider, record ProviderRecord) error {
	if c.manifest.Schema != ManifestSchemaV2 {
		return fail("claim-audit provider requires manifest /2")
	}
	set := record.ClaimAudits
	if set == nil || record.TestKnowledge != nil || record.FlowRelations != nil || record.BehaviorContracts != nil || record.Details != nil || record.RestrictedFindings != nil || record.Gaps != nil || len(record.Subjects)+len(record.Claims)+len(record.Relations)+len(record.Journeys)+len(record.Observations)+len(record.Capabilities) != 0 {
		return fail("claim-audit provider carries only its claim_audits member")
	}
	if len(set.Records) == 0 || len(set.Records) > MaxClaimAudits {
		return fail("invalid claim-audit record bound")
	}
	seen := map[string]bool{}
	for _, r := range set.Records {
		if !strings.HasPrefix(r.ID, p.ID+":") || !safeText(r.ID, 1024) || seen[r.ID] {
			return fail("claim-audit id must be unique and provider-qualified")
		}
		seen[r.ID] = true
		if r.ClaimID == "" || !safeText(r.ClaimID, 1024) || !wire.IsSha256(r.SpanSHA256) {
			return fail("claim-audit claim_id and span_sha256 are required")
		}
		if !claimAuditVerdicts[r.Verdict] || !claimAuditMethods[r.Method] {
			return fail("unknown claim-audit verdict or method")
		}
		if r.Reviewer == "" || !safeText(r.Reviewer, 256) || r.Note != "" && !safeText(r.Note, 16<<10) {
			return fail("invalid or secret-shaped claim-audit reviewer or note")
		}
		if r.Evidence == nil || len(r.Evidence) > 64 {
			return fail("claim-audit evidence is a bounded list")
		}
		for _, a := range r.Evidence {
			if err := c.checkTestKnowledgeAnchor(a); err != nil {
				return fail("invalid claim-audit evidence anchor")
			}
		}
	}
	return nil
}

// joinClaimAudits runs after every provider is imported (DCP-V1-261, DCP-V1-262).
func (c *compiler) joinClaimAudits(sets []providerClaimAudits) error {
	claims := map[string]Claim{}
	for _, claim := range c.artifact.Claims {
		claims[claim.ID] = claim
	}
	seen := map[string]bool{}
	total := 0
	for _, pa := range sets {
		total += len(pa.set.Records)
		if total > MaxClaimAudits {
			return fail("aggregate claim-audit bound exceeded")
		}
		for _, r := range pa.set.Records {
			if seen[r.ID] {
				return fail("duplicate claim-audit id")
			}
			seen[r.ID] = true
			if r.Evidence == nil {
				r.Evidence = []Anchor{}
			}
			j := ClaimAuditJoin{ID: r.ID, Provider: pa.provider, ClaimID: r.ClaimID, Status: "current", Verdict: r.Verdict, Method: r.Method, Reasons: []string{}, Limitations: []string{claimAuditLimitation}, Record: r}
			// Evidence anchors bind to declared inputs; a forged binding refuses the build, and an
			// undeclared path stays visible as unanchored evidence.
			for _, a := range r.Evidence {
				state, err := c.resolveTestKnowledgeAnchor(a, a.Revision)
				if err != nil {
					return fail("claim-audit evidence anchor binding mismatch")
				}
				if state == "unanchored" {
					j.Limitations = append(j.Limitations, "evidence anchor "+a.Path+" is not a declared input; it is unanchored")
				}
			}
			claim, ok := claims[r.ClaimID]
			if !ok {
				j.Status = "unresolved"
				j.Reasons = append(j.Reasons, "unresolved: claim is not in this corpus")
			} else {
				j.CurrentSpanSHA256 = ClaimAuditSpan(claim)
				if j.CurrentSpanSHA256 != r.SpanSHA256 {
					j.Status = "stale"
					j.Reasons = append(j.Reasons, "stale: claim text or cited span changed since the verdict")
				}
				j.Independent = r.Independent && r.Reviewer != claim.Provider && pa.provider != claim.Provider
				if !r.Independent {
					j.Reasons = append(j.Reasons, "not independent: reviewer_independent_of_generator is false")
				}
				if r.Reviewer == claim.Provider || pa.provider == claim.Provider {
					j.Reasons = append(j.Reasons, "not independent: reviewer or audit provider is the claim's provider")
				}
				j.GeneratorMethod = r.Method == "model" && claim.Evidence.Derivation == "generated"
				if j.GeneratorMethod {
					j.Reasons = append(j.Reasons, "generator method: model verdict on a generated claim")
				}
			}
			j.Headline = j.Status == "current" && j.Independent && !j.GeneratorMethod
			sort.Strings(j.Reasons)
			c.artifact.ClaimAudits = append(c.artifact.ClaimAudits, j)
		}
	}
	sort.Slice(c.artifact.ClaimAudits, func(i, k int) bool { return c.artifact.ClaimAudits[i].ID < c.artifact.ClaimAudits[k].ID })
	return nil
}

func claimAuditAnchors(j ClaimAuditJoin) []Anchor { return j.Record.Evidence }

// claimAuditViews returns the verdicts on claim id, current first, bounded (DCP-V1-263).
func claimAuditViews(a *Artifact, id string) ([]ClaimAuditView, bool) {
	out := []ClaimAuditView{}
	for _, j := range a.ClaimAudits {
		if j.ClaimID == id {
			out = append(out, ClaimAuditView{ID: j.ID, Status: j.Status, Verdict: j.Verdict, Method: j.Method, Reviewer: j.Record.Reviewer, Independent: j.Independent, Headline: j.Headline, Reasons: j.Reasons})
		}
	}
	sort.SliceStable(out, func(i, k int) bool { return (out[i].Status == "current") && out[k].Status != "current" })
	if len(out) > maxClaimAuditViews {
		return out[:maxClaimAuditViews], true
	}
	return out, false
}

// attachClaimAudits shows each visible claim's verdicts beside it without changing the claim
// or its trust label (DCP-V1-263).
func attachClaimAudits(a *Artifact, r *Receipt) {
	if len(a.ClaimAudits) == 0 {
		return
	}
	r.ClaimAudits = nil
	truncated := false
	for _, value := range r.Results {
		claim, ok := value.(Claim)
		if !ok {
			continue
		}
		views, cut := claimAuditViews(a, claim.ID)
		if len(views) == 0 {
			continue
		}
		truncated = truncated || cut
		if r.ClaimAudits == nil {
			r.ClaimAudits = map[string][]ClaimAuditView{}
		}
		r.ClaimAudits[claim.ID] = views
	}
	if truncated {
		r.Limitations = append(r.Limitations, "claim audit views withheld; get the audit records by id")
	}
}

// claimAuditGaps reports unjoined verdicts and every current incorrect verdict as a defect on
// the claim and on the subject (flow) it describes (DCP-V1-264).
func claimAuditGaps(a *Artifact) []Gap {
	out := []Gap{}
	// Only the claims a current incorrect verdict judges are looked up, so only theirs are
	// mapped; the last claim with an ID still names its subject (DCR-V1-557).
	var subjects map[string]string
	for _, j := range a.ClaimAudits {
		if j.Status == "current" && j.Verdict == "incorrect" {
			if subjects == nil {
				subjects = map[string]string{}
			}
			subjects[j.ClaimID] = ""
		}
	}
	if subjects != nil {
		for _, claim := range a.Claims {
			if _, judged := subjects[claim.ID]; judged {
				subjects[claim.ID] = claim.Subject
			}
		}
	}
	for _, j := range a.ClaimAudits {
		switch j.Status {
		case "stale":
			out = append(out, Gap{j.ID, "stale-claim-audit", strings.Join(j.Reasons, "; ")})
		case "unresolved":
			out = append(out, Gap{j.ID, "unresolved-claim-audit", strings.Join(j.Reasons, "; ")})
		case "current":
			if j.Verdict != "incorrect" {
				continue
			}
			label := "independent"
			if !j.Headline {
				label = "non-headline"
			}
			reason := "current " + label + " " + j.Method + " verdict " + j.ID + " judges claim " + j.ClaimID + " incorrect"
			out = append(out, Gap{j.ClaimID, "claim-audit-incorrect", reason})
			if s := subjects[j.ClaimID]; s != "" && s != j.ClaimID {
				out = append(out, Gap{s, "claim-audit-incorrect", reason})
			}
		}
	}
	return out
}
