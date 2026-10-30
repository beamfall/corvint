package snapshot

import (
	"encoding/json"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type Supervision struct {
	WorkerHolder        string     `json:"workerHolder"`
	LeaderStarted       string     `json:"leaderStarted"`
	ProgramID           string     `json:"programId"`
	OwnerStarted        string     `json:"ownerStarted"`
	Worker              bool       `json:"worker"`
	SessionID           string     `json:"sessionId"`
	AuthorHolder        string     `json:"authorHolder"`
	AuthorSession       string     `json:"authorSession"`
	ReviewerHolder      string     `json:"reviewerHolder"`
	ReviewerSession     string     `json:"reviewerSession"`
	ReviewTree          string     `json:"reviewTree"`
	ReviewDigest        string     `json:"reviewDigest"`
	HandoffDigest       string     `json:"handoffDigest"`
	QuestionID          string     `json:"questionId"`
	QuestionRevision    wire.Count `json:"questionRevision"`
	Question            string     `json:"question"`
	Answer              string     `json:"answer"`
	ResumePhase         string     `json:"resumePhase"`
	IntegrationExpected string     `json:"integrationExpected"`
	IntegrationCommit   string     `json:"integrationCommit"`
	IntegrationGrant    string     `json:"integrationGrant"`
	Turns               wire.Count `json:"turns"`
}

func readSupervision(r *wire.Reader) *Supervision {
	r.Closed("integrationExpected", "questionId", "questionRevision", "workerHolder", "leaderStarted", "programId", "ownerStarted", "worker", "sessionId", "authorHolder", "authorSession", "reviewerHolder", "reviewerSession", "reviewTree", "reviewDigest", "handoffDigest", "question", "answer", "resumePhase", "integrationCommit", "integrationGrant", "turns")
	return &Supervision{IntegrationExpected: r.Field("integrationExpected").String(), QuestionID: r.Field("questionId").String(), QuestionRevision: r.Field("questionRevision").Count(), WorkerHolder: r.Field("workerHolder").String(), LeaderStarted: r.Field("leaderStarted").String(), ProgramID: r.Field("programId").Identifier(), OwnerStarted: r.Field("ownerStarted").Identifier(), Worker: r.Field("worker").Bool(), SessionID: r.Field("sessionId").String(), AuthorHolder: r.Field("authorHolder").String(), AuthorSession: r.Field("authorSession").String(), ReviewerHolder: r.Field("reviewerHolder").String(), ReviewerSession: r.Field("reviewerSession").String(), ReviewTree: r.Field("reviewTree").String(), ReviewDigest: r.Field("reviewDigest").String(), HandoffDigest: r.Field("handoffDigest").String(), Question: r.Field("question").String(), Answer: r.Field("answer").String(), ResumePhase: r.Field("resumePhase").String(), IntegrationCommit: r.Field("integrationCommit").String(), IntegrationGrant: r.Field("integrationGrant").String(), Turns: r.Field("turns").Count()}
}
func supervisionValue(s *Supervision) wire.Value {
	raw, _ := EncodeProgramJSON(s)
	v, _ := wire.Parse(raw)
	return v
}
func CloneSupervision(s *Supervision) *Supervision {
	if s == nil {
		return nil
	}
	b, _ := json.Marshal(s)
	var out Supervision
	_ = json.Unmarshal(b, &out)
	return &out
}
