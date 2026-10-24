package doccorpus

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Corpus pattern report (DCX-V1-348..352): bounded, deterministic signals of generated-text
// failure modes across the claims the mention check examines. A flag is a sampling signal, not a
// verdict; it never changes a claim, its trust or a gap.

const (
	// CorpusPatternsSchema is the `docs corpus patterns` report document.
	CorpusPatternsSchema = "corvint-corpus-patterns/1"
	// MaxPatternClaims bounds the claim IDs listed per pattern; the rest are counted as omitted.
	MaxPatternClaims = 256

	patternTemplateMinSubjects = 3
	patternTemplateMinWords    = 3
	patternClusterMinClaims    = 5
	patternBroadMaxLines       = 200
	patternWholeFileMinLines   = 20
	patternShapeMinFamily      = 10
	patternShapeMinShare       = 0.8
)

// CorpusPatternNames are the patterns in report order; the first flagged name is a claim's
// audit stratum under --stratify patterns.
var CorpusPatternNames = []string{"token-disjoint", "templated-text", "anchor-cluster", "broad-anchor", "uniform-shape"}

var corpusPatternRules = map[string]string{
	"token-disjoint": "the claim has at least one code-like mention (DCP-V1-340) and none resolves in a cited span",
	"templated-text": "the claim's normalized template (lowercase; mentions, the subject name and digit runs replaced; whitespace collapsed) has at least min_words words and is shared by claims on at least min_subjects distinct subjects",
	"anchor-cluster": "at least min_claims claims cite the identical anchor (repository, revision, path, start, end, span hash)",
	"broad-anchor":   "a cited anchor spans at least max_lines lines, or is a whole file (span hash equals file hash) of at least whole_file_min_lines lines",
	"uniform-shape":  "within a subject kind of at least min_family claims, the most common shape (first three template words and word-count bucket of five) covers at least min_share of the family",
}

var corpusPatternThresholds = map[string]map[string]float64{
	"token-disjoint": {"min_mentions": 1},
	"templated-text": {"min_subjects": patternTemplateMinSubjects, "min_words": patternTemplateMinWords},
	"anchor-cluster": {"min_claims": patternClusterMinClaims},
	"broad-anchor":   {"max_lines": patternBroadMaxLines, "whole_file_min_lines": patternWholeFileMinLines},
	"uniform-shape":  {"min_family": patternShapeMinFamily, "min_share": patternShapeMinShare},
}

// CorpusPattern is one pattern's result over the examined claims.
type CorpusPattern struct {
	Name        string             `json:"name"`
	Rule        string             `json:"rule"`
	Thresholds  map[string]float64 `json:"thresholds"`
	Count       int                `json:"count"`
	Denominator int                `json:"denominator"`
	Rate        *float64           `json:"rate"`
	Groups      int                `json:"groups"`
	Claims      []string           `json:"claims"`
	Omitted     int                `json:"omitted"`
}

// CorpusPatternReport is the `docs corpus patterns` document.
type CorpusPatternReport struct {
	Schema      string          `json:"schema"`
	Corpus      AuditCorpus     `json:"corpus"`
	Population  int             `json:"population"`
	Flagged     int             `json:"flagged"`
	Patterns    []CorpusPattern `json:"patterns"`
	Limitations []string        `json:"limitations"`
}

func roundRate(n, d int) *float64 {
	if d == 0 {
		return nil
	}
	v := math.Round(float64(n)/float64(d)*1e6) / 1e6
	return &v
}

// claimTemplate normalizes text for the templated-text and uniform-shape patterns.
func claimTemplate(text, subjectName string) string {
	tokens, _ := ExtractCodeTokens(text)
	sort.SliceStable(tokens, func(i, j int) bool { return len(tokens[i].Token) > len(tokens[j].Token) })
	for _, t := range tokens {
		text = strings.ReplaceAll(text, "`"+t.Token+"`", " <code> ")
		text = strings.ReplaceAll(text, t.Token, " <code> ")
	}
	text = strings.ToLower(text)
	if name := strings.ToLower(strings.TrimSpace(subjectName)); len(name) > 1 {
		text = strings.ReplaceAll(text, name, " <subject> ")
	}
	var b strings.Builder
	digit := false
	for i := 0; i < len(text); i++ {
		if text[i] >= '0' && text[i] <= '9' {
			if !digit {
				b.WriteByte('0')
			}
			digit = true
			continue
		}
		digit = false
		b.WriteByte(text[i])
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func claimShape(template string) string {
	words := strings.Fields(template)
	head := words
	if len(head) > 3 {
		head = head[:3]
	}
	bucket := len(words) / 5 * 5
	return strings.Join(head, " ") + " | " + strconv.Itoa(bucket) + "-" + strconv.Itoa(bucket+4)
}

// corpusPatternFlags returns, per pattern name, the flagged claim IDs and the number of flagged
// groups, plus the examined population.
func corpusPatternFlags(a *Artifact) (map[string]map[string]bool, map[string]int, int) {
	native := map[string]bool{}
	for _, p := range a.Manifest.Providers {
		native[p.ID] = p.Kind == "native"
	}
	subjects := map[string]Subject{}
	for _, s := range a.Subjects {
		subjects[s.ID] = s
	}
	flags := map[string]map[string]bool{}
	groups := map[string]int{}
	for _, name := range CorpusPatternNames {
		flags[name] = map[string]bool{}
	}
	examined := []Claim{}
	for _, claim := range a.Claims {
		if mentionChecked(a, native, claim) {
			examined = append(examined, claim)
		}
	}
	for _, check := range a.ClaimMentions {
		if len(check.Mentions) > 0 && check.InSpan == 0 {
			flags["token-disjoint"][check.ClaimID] = true
			groups["token-disjoint"]++
		}
	}
	templates := map[string][]Claim{}
	clusters := map[string]map[string]bool{}
	families := map[string][]string{}
	shapes := map[string]string{}
	for _, claim := range examined {
		subject := subjects[claim.Subject]
		template := claimTemplate(claim.Text, subject.Name)
		if len(strings.Fields(template)) >= patternTemplateMinWords {
			templates[template] = append(templates[template], claim)
		}
		kind := subject.Kind
		if kind == "" {
			kind = "unknown"
		}
		families[kind] = append(families[kind], claim.ID)
		shapes[claim.ID] = claimShape(template)
		broad := false
		for _, an := range claim.Evidence.Anchors {
			key := fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d\x00%s", an.Repository, an.Revision, an.Path, an.Start, an.End, an.SpanSHA256)
			if clusters[key] == nil {
				clusters[key] = map[string]bool{}
			}
			clusters[key][claim.ID] = true
			lines := an.End - an.Start + 1
			if lines >= patternBroadMaxLines || an.SpanSHA256 == an.SHA256 && an.SpanSHA256 != "" && lines >= patternWholeFileMinLines {
				broad = true
			}
		}
		if broad {
			flags["broad-anchor"][claim.ID] = true
			groups["broad-anchor"]++
		}
	}
	for _, claims := range templates {
		distinct := map[string]bool{}
		for _, c := range claims {
			distinct[c.Subject] = true
		}
		if len(distinct) >= patternTemplateMinSubjects {
			groups["templated-text"]++
			for _, c := range claims {
				flags["templated-text"][c.ID] = true
			}
		}
	}
	for _, ids := range clusters {
		if len(ids) >= patternClusterMinClaims {
			groups["anchor-cluster"]++
			for id := range ids {
				flags["anchor-cluster"][id] = true
			}
		}
	}
	for _, ids := range families {
		if len(ids) < patternShapeMinFamily {
			continue
		}
		counts := map[string]int{}
		for _, id := range ids {
			counts[shapes[id]]++
		}
		top, best := "", 0
		for shape, n := range counts {
			if n > best || n == best && shape < top {
				top, best = shape, n
			}
		}
		if float64(best) >= patternShapeMinShare*float64(len(ids)) {
			groups["uniform-shape"]++
			for _, id := range ids {
				if shapes[id] == top {
					flags["uniform-shape"][id] = true
				}
			}
		}
	}
	return flags, groups, len(examined)
}

// PatternReport reports every corpus pattern over the examined claims (DCX-V1-348).
func PatternReport(a *Artifact) *CorpusPatternReport {
	flags, groups, population := corpusPatternFlags(a)
	r := &CorpusPatternReport{Schema: CorpusPatternsSchema, Corpus: auditCorpus(a), Population: population, Patterns: []CorpusPattern{},
		Limitations: []string{"examines non-native claims except supplied test-run receipts", "a flagged pattern is a sampling signal, not a verdict; it changes no claim, trust or gap", "thresholds are fixed by this contract version"}}
	flagged := map[string]bool{}
	for _, name := range CorpusPatternNames {
		ids := make([]string, 0, len(flags[name]))
		for id := range flags[name] {
			ids = append(ids, id)
			flagged[id] = true
		}
		sort.Strings(ids)
		p := CorpusPattern{Name: name, Rule: corpusPatternRules[name], Thresholds: corpusPatternThresholds[name], Count: len(ids), Denominator: population, Rate: roundRate(len(ids), population), Groups: groups[name], Claims: ids}
		if len(ids) > MaxPatternClaims {
			p.Claims, p.Omitted = ids[:MaxPatternClaims], len(ids)-MaxPatternClaims
		}
		r.Patterns = append(r.Patterns, p)
	}
	r.Flagged = len(flagged)
	if population == 0 {
		r.Limitations = append(r.Limitations, "no claim is examined; every rate is undefined")
	}
	return r
}

// RenderPatternTable renders the report as a fixed-width table.
func RenderPatternTable(r *CorpusPatternReport) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "corpus %s  examined %d  flagged %d\n", r.Corpus.ArtifactSHA256, r.Population, r.Flagged)
	fmt.Fprintf(&b, "%-16s %8s %8s %10s %8s\n", "pattern", "count", "groups", "rate", "omitted")
	for _, p := range r.Patterns {
		rate := "undefined"
		if p.Rate != nil {
			rate = strconv.FormatFloat(*p.Rate, 'f', 6, 64)
		}
		fmt.Fprintf(&b, "%-16s %8d %8d %10s %8d\n", p.Name, p.Count, p.Groups, rate, p.Omitted)
	}
	return []byte(b.String())
}

// BuildThresholds are the opt-in build refusals (DCX-V1-346, DCX-V1-352). A nil rate is unset.
type BuildThresholds struct {
	MaxUnresolvedMentionRate *float64
	MaxPatternRates          map[string]float64
}

// ParsePatternRates parses `R` (every pattern) or `NAME=R[,NAME=R]` with each R in [0, 1].
func ParsePatternRates(value string) (map[string]float64, error) {
	out := map[string]float64{}
	refuse := fail("--max-pattern-rate must be R or NAME=R[,NAME=R] with R in [0, 1] and NAME one of " + strings.Join(CorpusPatternNames, "|"))
	if r, err := ParseRate(value); err == nil {
		for _, name := range CorpusPatternNames {
			out[name] = r
		}
		return out, nil
	}
	for _, part := range strings.Split(value, ",") {
		name, raw, ok := strings.Cut(part, "=")
		r, err := ParseRate(raw)
		if !ok || err != nil || corpusPatternRules[name] == "" {
			return nil, refuse
		}
		if _, dup := out[name]; dup {
			return nil, refuse
		}
		out[name] = r
	}
	return out, nil
}

// ParseRate parses one decimal rate in [0, 1].
func ParseRate(value string) (float64, error) {
	r, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(r) || r < 0 || r > 1 || strings.ContainsAny(value, "eExXpP_+") {
		return 0, fail("rate must be a decimal in [0, 1]")
	}
	return r, nil
}

// UnresolvedMentionRate is checked claims with an unresolved mention / checked claims that have a
// code-like mention; nil when no checked claim has one.
func UnresolvedMentionRate(a *Artifact) (int, int, *float64) {
	n := 0
	for _, check := range a.ClaimMentions {
		if check.Unresolved > 0 {
			n++
		}
	}
	return n, len(a.ClaimMentions), roundRate(n, len(a.ClaimMentions))
}

// CheckBuildThresholds refuses a built corpus whose stated rates exceed the opt-in thresholds,
// naming every exceeded threshold; with no threshold set it accepts.
func CheckBuildThresholds(a *Artifact, t BuildThresholds) error {
	exceeded := []string{}
	if t.MaxUnresolvedMentionRate != nil {
		if n, d, rate := UnresolvedMentionRate(a); rate != nil && float64(n)/float64(d) > *t.MaxUnresolvedMentionRate {
			exceeded = append(exceeded, fmt.Sprintf("unresolved-mention rate %d/%d = %s exceeds %s", n, d, strconv.FormatFloat(*rate, 'f', -1, 64), strconv.FormatFloat(*t.MaxUnresolvedMentionRate, 'f', -1, 64)))
		}
	}
	if len(t.MaxPatternRates) > 0 {
		for _, p := range PatternReport(a).Patterns {
			limit, ok := t.MaxPatternRates[p.Name]
			if ok && p.Rate != nil && float64(p.Count)/float64(p.Denominator) > limit {
				exceeded = append(exceeded, fmt.Sprintf("%s rate %d/%d = %s exceeds %s", p.Name, p.Count, p.Denominator, strconv.FormatFloat(*p.Rate, 'f', -1, 64), strconv.FormatFloat(limit, 'f', -1, 64)))
			}
		}
	}
	if len(exceeded) > 0 {
		return fail("corpus build threshold exceeded: " + strings.Join(exceeded, "; "))
	}
	return nil
}
