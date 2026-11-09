package docmaintain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Blinded maintainer task (SDD-V0-015). A task set declares questions a maintainer answers from a
// generated reference alone; the answer key is a separate file whose SHA-256 the task set commits
// to before any response exists, so a key cannot be fitted to responses. The scorer is mechanical
// and records the result whether it passes or fails.

const (
	BlindedTaskSetProfile   = "corvint-docs-blinded-task-set/0"
	BlindedKeyProfile       = "corvint-docs-blinded-answer-key/0"
	BlindedResponsesProfile = "corvint-docs-blinded-responses/0"
	BlindedResultProfile    = "corvint-docs-blinded-result/0"
)

// BlindedTaskSet is what the answerer sees: the reference to generate and the questions. It never
// carries answers.
type BlindedTaskSet struct {
	Profile string `json:"profile"`
	// Reference names the generated reference: the docs draft selector at an immutable revision.
	Reference struct {
		Source   string `json:"source"`
		Package  string `json:"package"`
		Revision string `json:"revision"`
	} `json:"reference"`
	AnswerKeySHA256 string            `json:"answer_key_sha256"`
	PassNumerator   int               `json:"pass_numerator"`
	PassDenominator int               `json:"pass_denominator"`
	Questions       []BlindedQuestion `json:"questions"`
}

// BlindedQuestion is one declared question.
type BlindedQuestion struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// BlindedKey holds, per question, the facts a correct answer must state.
type BlindedKey struct {
	Profile string              `json:"profile"`
	Answers map[string][]string `json:"answers"`
}

// BlindedResponses is one answerer's run: who or what answered, under which condition, and the
// answers by question ID.
type BlindedResponses struct {
	Profile     string            `json:"profile"`
	Participant string            `json:"participant"`
	Condition   string            `json:"condition"`
	Answers     map[string]string `json:"answers"`
}

// BlindedScore is one question's mechanical score.
type BlindedScore struct {
	ID      string   `json:"id"`
	Correct bool     `json:"correct"`
	Missing []string `json:"missing"`
}

// BlindedResult is the retained record of one scored run.
type BlindedResult struct {
	Profile         string         `json:"profile"`
	TaskSetSHA256   string         `json:"task_set_sha256"`
	AnswerKeySHA256 string         `json:"answer_key_sha256"`
	ResponsesSHA256 string         `json:"responses_sha256"`
	Participant     string         `json:"participant"`
	Condition       string         `json:"condition"`
	Correct         int            `json:"correct"`
	Total           int            `json:"total"`
	Verdict         string         `json:"verdict"`
	Scores          []BlindedScore `json:"scores"`
}

// ScoreBlinded applies the scoring rule: a question is correct when its answer states every
// required fact of the key (case-insensitive, whitespace-collapsed substring); a missing answer
// is incorrect. The run passes when correct*PassDenominator >= total*PassNumerator. It refuses a
// key whose SHA-256 differs from the one the task set committed to, a key or response of the
// wrong profile, and a key that does not cover exactly the declared questions.
func ScoreBlinded(taskSet, key, responses []byte) (BlindedResult, error) {
	var ts BlindedTaskSet
	var k BlindedKey
	var r BlindedResponses
	if err := json.Unmarshal(taskSet, &ts); err != nil || ts.Profile != BlindedTaskSetProfile {
		return BlindedResult{}, fmt.Errorf("task set: not %s", BlindedTaskSetProfile)
	}
	if digestHex(key) != ts.AnswerKeySHA256 {
		return BlindedResult{}, fmt.Errorf("answer key sha256 %s differs from the task set's committed %s", digestHex(key), ts.AnswerKeySHA256)
	}
	if err := json.Unmarshal(key, &k); err != nil || k.Profile != BlindedKeyProfile {
		return BlindedResult{}, fmt.Errorf("answer key: not %s", BlindedKeyProfile)
	}
	if err := json.Unmarshal(responses, &r); err != nil || r.Profile != BlindedResponsesProfile {
		return BlindedResult{}, fmt.Errorf("responses: not %s", BlindedResponsesProfile)
	}
	if ts.PassDenominator <= 0 || ts.PassNumerator <= 0 || ts.PassNumerator > ts.PassDenominator || len(ts.Questions) == 0 {
		return BlindedResult{}, fmt.Errorf("task set: needs questions and a pass ratio in (0, 1]")
	}
	if len(k.Answers) != len(ts.Questions) {
		return BlindedResult{}, fmt.Errorf("answer key covers %d questions, task set declares %d", len(k.Answers), len(ts.Questions))
	}
	res := BlindedResult{Profile: BlindedResultProfile, TaskSetSHA256: digestHex(taskSet), AnswerKeySHA256: ts.AnswerKeySHA256,
		ResponsesSHA256: digestHex(responses), Participant: r.Participant, Condition: r.Condition, Total: len(ts.Questions), Scores: []BlindedScore{}}
	for _, q := range ts.Questions {
		facts, ok := k.Answers[q.ID]
		if !ok || len(facts) == 0 {
			return BlindedResult{}, fmt.Errorf("answer key has no facts for question %q", q.ID)
		}
		answer := normalize(r.Answers[q.ID])
		s := BlindedScore{ID: q.ID, Missing: []string{}}
		for _, f := range facts {
			if answer == "" || !strings.Contains(answer, normalize(f)) {
				s.Missing = append(s.Missing, f)
			}
		}
		s.Correct = len(s.Missing) == 0
		if s.Correct {
			res.Correct++
		}
		res.Scores = append(res.Scores, s)
	}
	res.Verdict = "FAIL"
	if res.Correct*ts.PassDenominator >= res.Total*ts.PassNumerator {
		res.Verdict = "PASS"
	}
	return res, nil
}

func normalize(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }
