package obscorpus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Corpus verdicts (OCA-V0-005).
const (
	VerdictPass        = "PASS"
	VerdictFail        = "FAIL"
	VerdictNotRun      = "NOT_RUN"
	VerdictNotObserved = "NOT_OBSERVED"
)

// Closed verdict reasons.
const (
	ReasonResultMissing   = "result-missing"
	ReasonLabelMissing    = "label-missing"
	ReasonVerifierMissing = "verifier-result-missing"
	ReasonTelemetryAbsent = "telemetry-not-observed"
)

// Non-authority markers every replay report carries (OCA-V0-004).
const (
	EvidenceOwnerLabelled = "owner-labelled"
	AuthorityNone         = "none"
	ScoringExactLabel     = "exact-label-match"
	MaxSubjects           = 1024
	corpusSchema          = "oca-corpus/1"
	reportSchema          = "oca-replay/1"
)

// Refusals.
var (
	ErrInvalidCorpus  = errors.New("invalid corpus")
	ErrChangedCorpus  = errors.New("changed corpus inputs are a new corpus")
	ErrSealMismatch   = errors.New("result is not bound to this sealed corpus")
	ErrInvalidResults = errors.New("invalid replay results")
)

// Subject is one corpus subject pinned to an immutable source revision.
type Subject struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
}

// Corpus is the owner's unsealed input. A subject without a label is allowed
// and can never score PASS.
type Corpus struct {
	Subjects          []Subject         `json:"subjects"`
	Labels            map[string]string `json:"labels"`
	EvaluatorRevision string            `json:"evaluatorRevision"`
	ScoringRule       string            `json:"scoringRule"`
}

// Sealed is an immutable corpus identity; only Seal constructs one, so results
// can be scored only after the inputs are sealed.
type Sealed struct {
	id        string
	canonical sealedForm
}

type sealedForm struct {
	Schema            string     `json:"schema"`
	Subjects          []Subject  `json:"subjects"`
	Labels            [][]string `json:"labels"`
	EvaluatorRevision string     `json:"evaluatorRevision"`
	ScoringRule       string     `json:"scoringRule"`
}

// Result is one revealed verifier outcome for a sealed subject.
type Result struct {
	SealID    string      `json:"sealId"`
	SubjectID string      `json:"subjectId"`
	Outcome   string      `json:"outcome"`
	Telemetry Observation `json:"telemetry"`
}

// Verdict carries no outcome or label text, only the closed status and reason.
type Verdict struct {
	SubjectID string `json:"subjectId"`
	Verdict   string `json:"verdict"`
	Reason    string `json:"reason,omitempty"`
}

// Report is owner-labelled evidence, never authority. Denominator is every
// sealed subject and only PASS counts as success.
type Report struct {
	Schema                string    `json:"schema"`
	SealID                string    `json:"sealId"`
	Evidence              string    `json:"evidence"`
	Authority             string    `json:"authority"`
	IndependentValidation bool      `json:"independentValidation"`
	Overall               string    `json:"overall"`
	Pass                  int       `json:"pass"`
	Fail                  int       `json:"fail"`
	NotRun                int       `json:"notRun"`
	NotObserved           int       `json:"notObserved"`
	Denominator           int       `json:"denominator"`
	Verdicts              []Verdict `json:"verdicts"`
}

// ID is the SHA-256 of the canonical sealed subject/label/evaluator/scoring form.
func (s Sealed) ID() string { return s.id }

// Seal validates and seals a corpus independently of subject and label order.
func Seal(corpus Corpus) (Sealed, error) {
	if err := validateCorpus(corpus); err != nil {
		return Sealed{}, err
	}
	subjects := append([]Subject(nil), corpus.Subjects...)
	sort.Slice(subjects, func(i, j int) bool { return subjects[i].ID < subjects[j].ID })
	labels := make([][]string, 0, len(corpus.Labels))
	for _, subject := range subjects {
		if label, ok := corpus.Labels[subject.ID]; ok {
			labels = append(labels, []string{subject.ID, label})
		}
	}
	form := sealedForm{Schema: corpusSchema, Subjects: subjects, Labels: labels, EvaluatorRevision: corpus.EvaluatorRevision, ScoringRule: corpus.ScoringRule}
	encoded, err := json.Marshal(form)
	if err != nil {
		return Sealed{}, fmt.Errorf("%w: %v", ErrInvalidCorpus, err)
	}
	digest := sha256.Sum256(encoded)
	return Sealed{id: hex.EncodeToString(digest[:]), canonical: form}, nil
}

// Reuse admits a corpus only when its sealed identity is identical.
func (s Sealed) Reuse(corpus Corpus) error {
	candidate, err := Seal(corpus)
	if err != nil {
		return err
	}
	if candidate.id != s.id {
		return fmt.Errorf("%w: sealed %s, candidate %s", ErrChangedCorpus, s.id, candidate.id)
	}
	return nil
}

// Replay scores revealed results against the sealed corpus deterministically.
func (s Sealed) Replay(results []Result) (Report, error) {
	bySubject, err := s.indexResults(results)
	if err != nil {
		return Report{}, err
	}
	labels := make(map[string]string, len(s.canonical.Labels))
	for _, pair := range s.canonical.Labels {
		labels[pair[0]] = pair[1]
	}
	report := Report{Schema: reportSchema, SealID: s.id, Evidence: EvidenceOwnerLabelled, Authority: AuthorityNone, Denominator: len(s.canonical.Subjects)}
	for _, subject := range s.canonical.Subjects {
		result, ran := bySubject[subject.ID]
		label, labelled := labels[subject.ID]
		verdict := score(subject.ID, result, ran, label, labelled)
		report.Verdicts = append(report.Verdicts, verdict)
	}
	tally(&report)
	return report, nil
}

func (s Sealed) indexResults(results []Result) (map[string]Result, error) {
	sealed := make(map[string]bool, len(s.canonical.Subjects))
	for _, subject := range s.canonical.Subjects {
		sealed[subject.ID] = true
	}
	bySubject := make(map[string]Result, len(results))
	for _, result := range results {
		if result.SealID != s.id {
			return nil, ErrSealMismatch
		}
		if !sealed[result.SubjectID] {
			return nil, fmt.Errorf("%w: unsealed subject", ErrInvalidResults)
		}
		if _, duplicate := bySubject[result.SubjectID]; duplicate {
			return nil, fmt.Errorf("%w: duplicate subject result", ErrInvalidResults)
		}
		bySubject[result.SubjectID] = result
	}
	return bySubject, nil
}

// score lets a missing result, label, verifier outcome, or telemetry field
// block success; only an observed mismatch is FAIL.
func score(subjectID string, result Result, ran bool, label string, labelled bool) Verdict {
	if !ran {
		return Verdict{SubjectID: subjectID, Verdict: VerdictNotRun, Reason: ReasonResultMissing}
	}
	if !labelled {
		return Verdict{SubjectID: subjectID, Verdict: VerdictNotObserved, Reason: ReasonLabelMissing}
	}
	outcome := boundText(result.Outcome, identityPattern, ReasonUnboundedText)
	if outcome.Status != StatusObserved {
		return Verdict{SubjectID: subjectID, Verdict: VerdictNotObserved, Reason: outcomeReason(outcome)}
	}
	if outcome.Value != label {
		return Verdict{SubjectID: subjectID, Verdict: VerdictFail}
	}
	if !telemetryObserved(result.Telemetry) {
		return Verdict{SubjectID: subjectID, Verdict: VerdictNotObserved, Reason: ReasonTelemetryAbsent}
	}
	return Verdict{SubjectID: subjectID, Verdict: VerdictPass}
}

// telemetryObserved re-bounds a decoded observation's values through Capture, so
// a result file that writes OBSERVED statuses over missing, unbounded, or
// mutable values cannot count as observed telemetry.
func telemetryObserved(o Observation) bool {
	raw := RawTelemetry{
		Identity:      o.Identity.Value,
		Revision:      o.Revision.Value,
		InputTokens:   o.InputTokens.Value,
		OutputTokens:  o.OutputTokens.Value,
		LatencyMillis: o.LatencyMillis.Value,
	}
	return o.Complete() && Capture(raw).Complete()
}

func outcomeReason(outcome Text) string {
	if outcome.Reason == ReasonNotReported {
		return ReasonVerifierMissing
	}
	return outcome.Reason
}

func tally(report *Report) {
	counters := map[string]*int{VerdictPass: &report.Pass, VerdictFail: &report.Fail, VerdictNotRun: &report.NotRun, VerdictNotObserved: &report.NotObserved}
	for _, verdict := range report.Verdicts {
		*counters[verdict.Verdict]++
	}
	report.Overall = overall(*report)
}

func overall(report Report) string {
	if report.Fail > 0 {
		return VerdictFail
	}
	if report.NotObserved > 0 {
		return VerdictNotObserved
	}
	if report.NotRun > 0 || report.Denominator == 0 {
		return VerdictNotRun
	}
	return VerdictPass
}

func validateCorpus(corpus Corpus) error {
	if len(corpus.Subjects) == 0 || len(corpus.Subjects) > MaxSubjects {
		return fmt.Errorf("%w: subject count %d outside 1..%d", ErrInvalidCorpus, len(corpus.Subjects), MaxSubjects)
	}
	if corpus.ScoringRule != ScoringExactLabel {
		return fmt.Errorf("%w: unknown scoring rule", ErrInvalidCorpus)
	}
	if boundText(corpus.EvaluatorRevision, revisionPattern, ReasonNotImmutable).Status != StatusObserved {
		return fmt.Errorf("%w: evaluator revision is not an immutable revision", ErrInvalidCorpus)
	}
	seen := make(map[string]bool, len(corpus.Subjects))
	for _, subject := range corpus.Subjects {
		if err := validateSubject(subject, seen); err != nil {
			return err
		}
		seen[subject.ID] = true
	}
	for subjectID, label := range corpus.Labels {
		if err := validateLabel(subjectID, label, seen); err != nil {
			return err
		}
	}
	return nil
}

func validateSubject(subject Subject, seen map[string]bool) error {
	if boundText(subject.ID, identityPattern, ReasonUnboundedText).Status != StatusObserved {
		return fmt.Errorf("%w: subject identity is not bounded", ErrInvalidCorpus)
	}
	if seen[subject.ID] {
		return fmt.Errorf("%w: duplicate subject %s", ErrInvalidCorpus, subject.ID)
	}
	if boundText(subject.Revision, revisionPattern, ReasonNotImmutable).Status != StatusObserved {
		return fmt.Errorf("%w: subject %s revision is not an immutable revision", ErrInvalidCorpus, subject.ID)
	}
	return nil
}

func validateLabel(subjectID, label string, seen map[string]bool) error {
	if !seen[subjectID] {
		return fmt.Errorf("%w: label for unsealed subject", ErrInvalidCorpus)
	}
	if secretscreen.MatchString(label) {
		return fmt.Errorf("%w: label for %s is secret-shaped", ErrInvalidCorpus, subjectID)
	}
	if boundText(label, identityPattern, ReasonUnboundedText).Status != StatusObserved {
		return fmt.Errorf("%w: label for %s is not bounded", ErrInvalidCorpus, subjectID)
	}
	return nil
}
