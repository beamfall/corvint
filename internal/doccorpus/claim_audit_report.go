package doccorpus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	// AuditSampleSchema and AuditReportSchema are the claim-audit sample and report
	// documents (DCP-V1-265..268).
	AuditSampleSchema = "corvint-corpus-audit-sample/1"
	AuditReportSchema = "corvint-corpus-audit-report/1"
	// MaxAuditSample bounds a requested sample size.
	MaxAuditSample = 1024
	// MaxAuditSeed keeps the seed exact in every JSON reader.
	MaxAuditSeed = 1<<53 - 1
)

// AuditStratifications are the admitted --stratify values; "" is an unstratified sample.
var AuditStratifications = []string{"provider", "kind", "flow", "trust", "patterns"}

const auditSampleMethod = "per-stratum rank by sha256(seed, claim id); proportional allocation by largest remainder, ties by stratum name"

// AuditPatternWeight is the over-sampling weight of a flagged pattern stratum relative to the
// unflagged stratum under --stratify patterns (DCX-V1-350).
const AuditPatternWeight = 4

const auditPatternSampleMethod = "per-stratum rank by sha256(seed, claim id); stratum is the first flagged pattern in report order, else unflagged; weighted largest-remainder allocation with each flagged stratum weighted 4 per claim and unflagged 1, capped at stratum population, ties by stratum name"

type AuditSampleOptions struct {
	Seed     uint64
	Size     int
	Stratify string
	Scope    string
}

// AuditCorpus is the artifact identity a sample or report is tied to.
type AuditCorpus struct {
	ArtifactSHA256  string     `json:"artifact_sha256"`
	Repository      Repository `json:"repository"`
	Tree            string     `json:"tree"`
	Profile         string     `json:"profile"`
	ProfileRevision string     `json:"profile_revision"`
	ManifestSHA256  string     `json:"manifest_sha256"`
}
type AuditDesign struct {
	Seed       uint64 `json:"seed"`
	Size       int    `json:"size"`
	Stratify   string `json:"stratify"`
	Scope      string `json:"scope"`
	Population int    `json:"population"`
	Sampled    int    `json:"sampled"`
	Method     string `json:"method"`
}
type AuditStratum struct {
	Name       string `json:"name"`
	Population int    `json:"population"`
	Sampled    int    `json:"sampled"`
}
type AuditAnchorRef struct {
	Path       string `json:"path"`
	Start      int    `json:"start_line"`
	End        int    `json:"end_line"`
	SpanSHA256 string `json:"span_sha256"`
}
type AuditSampleClaim struct {
	ClaimID    string           `json:"claim_id"`
	Stratum    string           `json:"stratum"`
	Provider   string           `json:"provider"`
	Derivation string           `json:"derivation"`
	Trust      string           `json:"trust"`
	SpanSHA256 string           `json:"span_sha256"`
	Anchors    []AuditAnchorRef `json:"anchors"`
}
type AuditSample struct {
	Schema      string             `json:"schema"`
	Corpus      AuditCorpus        `json:"corpus"`
	Design      AuditDesign        `json:"design"`
	Strata      []AuditStratum     `json:"strata"`
	Claims      []AuditSampleClaim `json:"claims"`
	Limitations []string           `json:"limitations"`
}

func auditCorpus(a *Artifact) AuditCorpus {
	return AuditCorpus{a.SHA256, a.Manifest.Repository, a.Tree, a.Manifest.Profile.ID, a.Manifest.Profile.Revision, a.ManifestSHA256}
}

// ValidAuditStratify reports whether value is an admitted --stratify value.
func ValidAuditStratify(value string) bool {
	return value == "" || strings.Contains(" "+strings.Join(AuditStratifications, " ")+" ", " "+value+" ")
}

func auditStratum(a *Artifact, subjects map[string]Subject, claim Claim, by string) string {
	switch by {
	case "provider":
		return claim.Provider
	case "kind":
		if kind := a.Details[claim.ID].ClaimKind; kind != "" {
			return kind
		}
		return "unspecified"
	case "flow":
		if s, ok := subjects[claim.Subject]; ok && (s.Kind == "flow" || s.Kind == "variation") {
			return s.ID
		}
		return "no-flow"
	case "trust":
		return claim.Evidence.Trust
	}
	return "all"
}

// auditPatternStrata maps each flagged claim to its first flagged pattern (DCX-V1-350).
func auditPatternStrata(a *Artifact) map[string]string {
	flags, _, _ := corpusPatternFlags(a)
	out := map[string]string{}
	for i := len(CorpusPatternNames) - 1; i >= 0; i-- {
		for id := range flags[CorpusPatternNames[i]] {
			out[id] = CorpusPatternNames[i]
		}
	}
	return out
}

// weightedAllocation allocates size across strata in proportion to population times weight,
// capping each at its population and redistributing the rest, then by largest remainder with
// ties by stratum name (DCX-V1-350).
func weightedAllocation(names []string, population map[string]int, weight func(string) int, size int) map[string]int {
	alloc := map[string]int{}
	open := append([]string{}, names...)
	remaining := size
	for {
		total := 0
		for _, name := range open {
			total += population[name] * weight(name)
		}
		next := open[:0:0]
		capped := false
		for _, name := range open {
			if total > 0 && remaining*weight(name) >= total {
				alloc[name] = population[name]
				remaining -= population[name]
				capped = true
				continue
			}
			next = append(next, name)
		}
		open = next
		if !capped || len(open) == 0 {
			break
		}
	}
	total := 0
	for _, name := range open {
		total += population[name] * weight(name)
	}
	if total == 0 {
		return alloc
	}
	type rem struct {
		name string
		num  int
	}
	rems, given := []rem{}, 0
	for _, name := range open {
		n := remaining * population[name] * weight(name)
		alloc[name] = n / total
		given += alloc[name]
		rems = append(rems, rem{name, n % total})
	}
	sort.SliceStable(rems, func(i, k int) bool { return rems[i].num > rems[k].num })
	for i := 0; given < remaining && i < len(rems); i++ {
		if alloc[rems[i].name] < population[rems[i].name] {
			alloc[rems[i].name]++
			given++
		}
	}
	return alloc
}

func auditInScope(claim Claim, scope string) bool {
	if scope == "" {
		return true
	}
	for _, an := range claim.Evidence.Anchors {
		if an.Path == scope || strings.HasPrefix(an.Path, strings.TrimSuffix(scope, "/")+"/") {
			return true
		}
	}
	return false
}

func auditRank(seed uint64, id string) string {
	sum := sha256.Sum256([]byte(strconv.FormatUint(seed, 10) + "\x00" + id))
	return hex.EncodeToString(sum[:])
}

// SampleClaims draws a reproducible claim sample: the same artifact, seed and options always
// give the same sample (DCP-V1-265).
func SampleClaims(a *Artifact, o AuditSampleOptions) (*AuditSample, error) {
	if o.Seed > MaxAuditSeed || o.Size < 1 || o.Size > MaxAuditSample || !ValidAuditStratify(o.Stratify) || o.Scope != "" && !validPath(strings.TrimSuffix(o.Scope, "/")) {
		return nil, fail(fmt.Sprintf("audit sample: seed is 0..%d, size is 1..%d, stratify is one of %s, scope is a canonical relative path", MaxAuditSeed, MaxAuditSample, strings.Join(AuditStratifications, "|")))
	}
	subjects := map[string]Subject{}
	for _, s := range a.Subjects {
		subjects[s.ID] = s
	}
	var patterns map[string]string
	if o.Stratify == "patterns" {
		patterns = auditPatternStrata(a)
	}
	strata := map[string][]Claim{}
	population := 0
	for _, claim := range a.Claims {
		if !auditInScope(claim, o.Scope) {
			continue
		}
		name := auditStratum(a, subjects, claim, o.Stratify)
		if patterns != nil {
			name = patterns[claim.ID]
			if name == "" {
				name = "unflagged"
			}
		}
		strata[name] = append(strata[name], claim)
		population++
	}
	names := make([]string, 0, len(strata))
	for name := range strata {
		names = append(names, name)
	}
	sort.Strings(names)
	alloc := map[string]int{}
	if o.Size >= population {
		for _, name := range names {
			alloc[name] = len(strata[name])
		}
	} else if o.Stratify == "patterns" {
		counts := map[string]int{}
		for _, name := range names {
			counts[name] = len(strata[name])
		}
		alloc = weightedAllocation(names, counts, func(name string) int {
			if name == "unflagged" {
				return 1
			}
			return AuditPatternWeight
		}, o.Size)
	} else {
		type rem struct {
			name string
			num  int
		}
		rems, given := []rem{}, 0
		for _, name := range names {
			n := len(strata[name]) * o.Size
			alloc[name] = n / population
			given += alloc[name]
			rems = append(rems, rem{name, n % population})
		}
		sort.SliceStable(rems, func(i, k int) bool { return rems[i].num > rems[k].num })
		for i := 0; given < o.Size; i++ {
			if alloc[rems[i].name] < len(strata[rems[i].name]) {
				alloc[rems[i].name]++
				given++
			}
		}
	}
	stratify := o.Stratify
	if stratify == "" {
		stratify = "none"
	}
	out := &AuditSample{Schema: AuditSampleSchema, Corpus: auditCorpus(a), Design: AuditDesign{Seed: o.Seed, Size: o.Size, Stratify: stratify, Scope: o.Scope, Population: population, Method: auditSampleMethod}, Strata: []AuditStratum{}, Claims: []AuditSampleClaim{},
		Limitations: []string{"a sample measures only the sampled claims; strata are proportionally allocated and the pooled figure is not population-weighted", "claim identity and span hashes come from this artifact; a rebuilt corpus can stale a verdict"}}
	if patterns != nil {
		out.Design.Method = auditPatternSampleMethod
		out.Limitations[0] = "a sample measures only the sampled claims; flagged pattern strata are over-sampled, so only the report's population-weighted figure estimates the population"
	}
	for _, name := range names {
		claims := append([]Claim{}, strata[name]...)
		sort.Slice(claims, func(i, k int) bool {
			ri, rk := auditRank(o.Seed, claims[i].ID), auditRank(o.Seed, claims[k].ID)
			return ri < rk || ri == rk && claims[i].ID < claims[k].ID
		})
		out.Strata = append(out.Strata, AuditStratum{name, len(claims), alloc[name]})
		for _, claim := range claims[:alloc[name]] {
			refs := []AuditAnchorRef{}
			for _, an := range claim.Evidence.Anchors {
				refs = append(refs, AuditAnchorRef{an.Path, an.Start, an.End, an.SpanSHA256})
			}
			out.Claims = append(out.Claims, AuditSampleClaim{claim.ID, name, claim.Provider, claim.Evidence.Derivation, claim.Evidence.Trust, ClaimAuditSpan(claim), refs})
		}
	}
	sort.Slice(out.Claims, func(i, k int) bool { return out.Claims[i].ClaimID < out.Claims[k].ClaimID })
	out.Design.Sampled = len(out.Claims)
	if population == 0 {
		out.Limitations = append(out.Limitations, "no claim is in scope; the sample is empty and any report abstains")
	}
	return out, nil
}

// ParseAuditSample strictly decodes a sample document.
func ParseAuditSample(raw []byte) (*AuditSample, error) {
	var s AuditSample
	if err := decode(raw, &s); err != nil || s.Schema != AuditSampleSchema || s.Claims == nil || s.Strata == nil || len(s.Claims) > MaxAuditSample || s.Design.Sampled != len(s.Claims) {
		return nil, fail("audit report: invalid " + AuditSampleSchema + " sample")
	}
	for _, st := range s.Strata {
		if st.Sampled < 0 || st.Population < st.Sampled || st.Population > MaxCorpusRecords*4 {
			return nil, fail("audit report: sample strata need 0 <= sampled <= population <= " + strconv.Itoa(MaxCorpusRecords*4))
		}
	}
	seen := map[string]bool{}
	for _, c := range s.Claims {
		if seen[c.ClaimID] || c.ClaimID == "" {
			return nil, fail("audit report: sample claim ids must be unique and non-empty")
		}
		seen[c.ClaimID] = true
	}
	// Strata reconcile with the claims and totals (DCX-V1-351): unique names, each sampled claim in
	// exactly one listed stratum, per-stratum sampled counts equal to its claims, and population and
	// sampled totals equal to the design's.
	sampled := map[string]int{}
	population, total := 0, 0
	for _, st := range s.Strata {
		if _, dup := sampled[st.Name]; dup {
			return nil, fail("audit report: sample strata names must be unique")
		}
		sampled[st.Name] = 0
		population += st.Population
		total += st.Sampled
	}
	for _, c := range s.Claims {
		if _, ok := sampled[c.Stratum]; !ok {
			return nil, fail("audit report: sample claim " + c.ClaimID + " names unlisted stratum " + strconv.Quote(c.Stratum))
		}
		sampled[c.Stratum]++
	}
	for _, st := range s.Strata {
		if sampled[st.Name] != st.Sampled {
			return nil, fail(fmt.Sprintf("audit report: stratum %q lists %d sampled claims but the sample has %d", st.Name, st.Sampled, sampled[st.Name]))
		}
	}
	if population != s.Design.Population || total != s.Design.Sampled {
		return nil, fail(fmt.Sprintf("audit report: strata total population %d and sampled %d differ from the design's %d and %d", population, total, s.Design.Population, s.Design.Sampled))
	}
	return &s, nil
}

// AuditVerdictCounts counts verdicts or claim outcomes by verdict.
type AuditVerdictCounts struct {
	Correct      int `json:"correct"`
	Incorrect    int `json:"incorrect"`
	Unsupported  int `json:"unsupported_by_citation"`
	Unverifiable int `json:"unverifiable"`
}

func (c *AuditVerdictCounts) add(verdict string) {
	switch verdict {
	case "correct":
		c.Correct++
	case "incorrect":
		c.Incorrect++
	case "unsupported-by-citation":
		c.Unsupported++
	case "unverifiable":
		c.Unverifiable++
	}
}

// AuditRatio is accuracy = correct / (correct + incorrect + unsupported) with its 95% Wilson
// interval; an empty denominator is undefined and the figure abstains (DCP-V1-266).
type AuditRatio struct {
	Numerator   int       `json:"numerator"`
	Denominator int       `json:"denominator"`
	Defined     bool      `json:"defined"`
	Value       *float64  `json:"value"`
	Interval95  []float64 `json:"interval_95"`
	Definition  string    `json:"definition"`
}

const auditAccuracyDefinition = "correct / (correct + incorrect + unsupported-by-citation); unverifiable is kept separate; 95% Wilson score interval"

func auditRatio(c AuditVerdictCounts) AuditRatio {
	n := c.Correct + c.Incorrect + c.Unsupported
	r := AuditRatio{Numerator: c.Correct, Denominator: n, Defined: n > 0, Interval95: []float64{}, Definition: auditAccuracyDefinition}
	if n == 0 {
		return r
	}
	round := func(v float64) float64 { return math.Round(v*1e6) / 1e6 }
	p, z := float64(c.Correct)/float64(n), 1.959963984540054
	den := 1 + z*z/float64(n)
	mid := (p + z*z/(2*float64(n))) / den
	half := z * math.Sqrt(p*(1-p)/float64(n)+z*z/(4*float64(n)*float64(n))) / den
	value := round(p)
	r.Value = &value
	r.Interval95 = []float64{round(math.Max(0, mid-half)), round(math.Min(1, mid+half))}
	return r
}

// AuditExcluded holds current verdicts that never enter the headline: non-independent
// reviewers and the generator's own method (DCP-V1-267).
type AuditExcluded struct {
	NonIndependent  int                `json:"non_independent"`
	GeneratorMethod int                `json:"generator_method"`
	Verdicts        AuditVerdictCounts `json:"verdicts"`
	Accuracy        AuditRatio         `json:"accuracy"`
}

// AuditGroup is one stratum, or the overall sample, of a report. Claim outcomes count
// claims; StaleVerdicts and Excluded count verdicts.
type AuditGroup struct {
	Name          string             `json:"name"`
	Sampled       int                `json:"sampled"`
	Audited       int                `json:"audited"`
	Outcomes      AuditVerdictCounts `json:"outcomes"`
	Unaudited     int                `json:"unaudited"`
	Conflicted    int                `json:"conflicted"`
	Missing       int                `json:"missing"`
	StaleVerdicts int                `json:"stale_verdicts"`
	Accuracy      AuditRatio         `json:"accuracy"`
	Excluded      AuditExcluded      `json:"excluded"`
	Claims        []string           `json:"claims"`
}

// AuditMethodGroup counts headline verdicts by method, so model and test verdicts stay
// labelled beside human ones.
type AuditMethodGroup struct {
	Method   string             `json:"method"`
	Verdicts AuditVerdictCounts `json:"verdicts"`
	Accuracy AuditRatio         `json:"accuracy"`
}

type AuditClaimOutcome struct {
	ClaimID  string   `json:"claim_id"`
	Stratum  string   `json:"stratum"`
	Outcome  string   `json:"outcome"`
	Headline []string `json:"headline"`
	Excluded []string `json:"excluded"`
	Stale    []string `json:"stale"`
}

type AuditSampleBinding struct {
	Path   string      `json:"path"`
	SHA256 string      `json:"sha256"`
	Corpus AuditCorpus `json:"corpus"`
	Design AuditDesign `json:"design"`
}

// AuditWeighted is the population-weighted accuracy sum over strata of (N_h / N) * p_h, where
// N_h is a stratum's sampled-from population and p_h its accuracy; it abstains unless every
// populated stratum's accuracy is defined (DCX-V1-351).
type AuditWeighted struct {
	Population      int      `json:"population"`
	Defined         bool     `json:"defined"`
	Value           *float64 `json:"value"`
	UndefinedStrata []string `json:"undefined_strata"`
	Definition      string   `json:"definition"`
}

const auditWeightedDefinition = "sum over populated strata of (stratum population / population) * stratum accuracy; no interval; abstains when any populated stratum's accuracy is undefined"

type AuditReport struct {
	Schema  string             `json:"schema"`
	Sample  AuditSampleBinding `json:"sample"`
	Corpus  AuditCorpus        `json:"corpus"`
	Overall AuditGroup         `json:"overall"`
	// Weighted is present only for a stratified sample (DCX-V1-351).
	Weighted    *AuditWeighted      `json:"weighted,omitempty"`
	ByStratum   []AuditGroup        `json:"by_stratum"`
	ByMethod    []AuditMethodGroup  `json:"by_method"`
	Claims      []AuditClaimOutcome `json:"claims"`
	Limitations []string            `json:"limitations"`
}

func newAuditGroup(name string) AuditGroup {
	return AuditGroup{Name: name, Claims: []string{}}
}

// ReportAudit measures sampled-claim accuracy from the verdicts joined in a. Only current,
// independent, non-generator-method verdicts enter the headline; a sampled claim with none is
// unaudited and the figure abstains rather than guess (DCP-V1-266..268).
func ReportAudit(a *Artifact, sample *AuditSample, samplePath string, sampleRaw []byte) (*AuditReport, error) {
	if sample.Corpus.Repository.ID != a.Manifest.Repository.ID || sample.Corpus.Profile != a.Manifest.Profile.ID {
		return nil, fail("audit report: sample corpus identity differs (repository or profile)")
	}
	claims := map[string]Claim{}
	for _, c := range a.Claims {
		claims[c.ID] = c
	}
	verdicts := map[string][]ClaimAuditJoin{}
	for _, j := range a.ClaimAudits {
		verdicts[j.ClaimID] = append(verdicts[j.ClaimID], j)
	}
	r := &AuditReport{Schema: AuditReportSchema, Sample: AuditSampleBinding{samplePath, Digest(sampleRaw), sample.Corpus, sample.Design}, Corpus: auditCorpus(a), ByStratum: []AuditGroup{}, ByMethod: []AuditMethodGroup{}, Claims: []AuditClaimOutcome{},
		Limitations: []string{claimAuditLimitation, "the headline counts only current verdicts from reviewers independent of the generator, excluding a model verdict on a generated claim", "independence is provider-reported apart from the reviewer and audit provider differing from the claim's provider", "the pooled overall figure is not population-weighted across strata"}}
	overall := newAuditGroup("overall")
	strata := map[string]*AuditGroup{}
	methods := map[string]*AuditVerdictCounts{}
	excludedMethods := map[string]bool{}
	for _, sc := range sample.Claims {
		g, ok := strata[sc.Stratum]
		if !ok {
			ng := newAuditGroup(sc.Stratum)
			g = &ng
			strata[sc.Stratum] = g
		}
		o := AuditClaimOutcome{ClaimID: sc.ClaimID, Stratum: sc.Stratum, Headline: []string{}, Excluded: []string{}, Stale: []string{}}
		headline := map[string]bool{}
		excluded := AuditExcluded{}
		for _, j := range verdicts[sc.ClaimID] {
			switch {
			case j.Status == "stale":
				o.Stale = append(o.Stale, j.ID)
			case j.Headline:
				o.Headline = append(o.Headline, j.ID)
				headline[j.Verdict] = true
				if methods[j.Method] == nil {
					methods[j.Method] = &AuditVerdictCounts{}
				}
				methods[j.Method].add(j.Verdict)
			case j.Status == "current":
				o.Excluded = append(o.Excluded, j.ID)
				if !j.Independent {
					excluded.NonIndependent++
				}
				if j.GeneratorMethod {
					excluded.GeneratorMethod++
				}
				excluded.Verdicts.add(j.Verdict)
				excludedMethods[j.Method] = true
			}
		}
		_, present := claims[sc.ClaimID]
		switch {
		case !present:
			o.Outcome = "missing"
		case len(headline) == 0:
			o.Outcome = "unaudited"
		case len(headline) > 1:
			o.Outcome = "conflicted"
		default:
			for v := range headline {
				o.Outcome = v
			}
		}
		for _, grp := range []*AuditGroup{&overall, g} {
			grp.Sampled++
			grp.Claims = append(grp.Claims, sc.ClaimID)
			grp.StaleVerdicts += len(o.Stale)
			grp.Excluded.NonIndependent += excluded.NonIndependent
			grp.Excluded.GeneratorMethod += excluded.GeneratorMethod
			grp.Excluded.Verdicts.Correct += excluded.Verdicts.Correct
			grp.Excluded.Verdicts.Incorrect += excluded.Verdicts.Incorrect
			grp.Excluded.Verdicts.Unsupported += excluded.Verdicts.Unsupported
			grp.Excluded.Verdicts.Unverifiable += excluded.Verdicts.Unverifiable
			switch o.Outcome {
			case "missing":
				grp.Missing++
			case "unaudited":
				grp.Unaudited++
			case "conflicted":
				grp.Conflicted++
			default:
				grp.Audited++
				grp.Outcomes.add(o.Outcome)
			}
		}
		r.Claims = append(r.Claims, o)
	}
	finish := func(g *AuditGroup) {
		g.Accuracy = auditRatio(g.Outcomes)
		g.Excluded.Accuracy = auditRatio(g.Excluded.Verdicts)
	}
	finish(&overall)
	r.Overall = overall
	names := make([]string, 0, len(strata))
	for name := range strata {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		finish(strata[name])
		r.ByStratum = append(r.ByStratum, *strata[name])
	}
	if sample.Design.Stratify != "" && sample.Design.Stratify != "none" {
		r.Weighted = auditWeighted(sample.Strata, strata)
		if !r.Weighted.Defined {
			r.Limitations = append(r.Limitations, "the population-weighted figure abstained: a populated stratum has no defined accuracy")
		}
	}
	for _, m := range []string{"human", "model", "test"} {
		if c := methods[m]; c != nil {
			r.ByMethod = append(r.ByMethod, AuditMethodGroup{m, *c, auditRatio(*c)})
		}
	}
	if !overall.Accuracy.Defined {
		r.Limitations = append(r.Limitations, "abstained: no sampled claim has a current headline verdict that is correct, incorrect or unsupported")
	}
	if overall.Unaudited+overall.Conflicted+overall.Missing > 0 {
		r.Limitations = append(r.Limitations, "unaudited, conflicted and missing sampled claims are outside the accuracy denominator and reported beside it")
	}
	if sample.Corpus.ArtifactSHA256 != a.SHA256 {
		r.Limitations = append(r.Limitations, "the sample was drawn from another build of this repository and profile; verdicts are matched by claim id and span hash")
	}
	return r, nil
}

// RenderAuditTable prints a text view of the same report.
func RenderAuditTable(r *AuditReport) []byte {
	var b bytes.Buffer
	ratio := func(x AuditRatio) string {
		if !x.Defined {
			return "abstain (0/0)"
		}
		return fmt.Sprintf("%.4f (%d/%d) 95%% [%.4f, %.4f]", *x.Value, x.Numerator, x.Denominator, x.Interval95[0], x.Interval95[1])
	}
	fmt.Fprintf(&b, "claim audit report  artifact %s  sample seed %d stratify %s size %d/%d\n", r.Corpus.ArtifactSHA256, r.Sample.Design.Seed, r.Sample.Design.Stratify, r.Sample.Design.Sampled, r.Sample.Design.Population)
	fmt.Fprintf(&b, "%-32s %7s %7s %9s %9s %7s %6s  %s\n", "group", "sampled", "audited", "unaudited", "conflicted", "missing", "stale", "accuracy")
	for _, g := range append([]AuditGroup{r.Overall}, r.ByStratum...) {
		fmt.Fprintf(&b, "%-32s %7d %7d %9d %9d %7d %6d  %s\n", g.Name, g.Sampled, g.Audited, g.Unaudited, g.Conflicted, g.Missing, g.StaleVerdicts, ratio(g.Accuracy))
		fmt.Fprintf(&b, "%-32s excluded: %d non-independent, %d generator-method, accuracy %s\n", "", g.Excluded.NonIndependent, g.Excluded.GeneratorMethod, ratio(g.Excluded.Accuracy))
	}
	if w := r.Weighted; w != nil {
		value := "abstain (undefined strata: " + strings.Join(w.UndefinedStrata, ", ") + ")"
		if w.Defined {
			value = fmt.Sprintf("%.4f over population %d", *w.Value, w.Population)
		}
		fmt.Fprintf(&b, "%-32s %s\n", "population-weighted", value)
	}
	for _, m := range r.ByMethod {
		fmt.Fprintf(&b, "method %-25s accuracy %s\n", m.Method, ratio(m.Accuracy))
	}
	for _, l := range r.Limitations {
		fmt.Fprintf(&b, "limitation: %s\n", l)
	}
	return b.Bytes()
}

func auditWeighted(sampled []AuditStratum, groups map[string]*AuditGroup) *AuditWeighted {
	w := &AuditWeighted{UndefinedStrata: []string{}, Definition: auditWeightedDefinition}
	for _, st := range sampled {
		w.Population += st.Population
	}
	sum := 0.0
	for _, st := range sampled {
		if st.Population == 0 {
			continue
		}
		g := groups[st.Name]
		if g == nil || !g.Accuracy.Defined {
			w.UndefinedStrata = append(w.UndefinedStrata, st.Name)
			continue
		}
		sum += float64(st.Population) / float64(w.Population) * float64(g.Accuracy.Numerator) / float64(g.Accuracy.Denominator)
	}
	sort.Strings(w.UndefinedStrata)
	if w.Population > 0 && len(w.UndefinedStrata) == 0 {
		v := math.Round(sum*1e6) / 1e6
		w.Defined, w.Value = true, &v
	}
	return w
}
