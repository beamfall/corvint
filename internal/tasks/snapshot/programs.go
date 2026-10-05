package snapshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"io"
)

const MaxProgramsBytes = 1 << 20
const SupervisedProfile = "taskman-codex-supervisor/0"

type WorktreeRecord struct {
	Path            string `json:"path"`
	Identity        string `json:"identity"`
	PrivateIdentity string `json:"privateIdentity"`
	Commit          string `json:"commit"`
	Removed         bool   `json:"removed"`
	// Repository names the extra repository this worktree belongs to
	// (CAL-V0-071); empty is the queue's own repository.
	Repository string `json:"repository,omitempty"`
}
type Program struct {
	CurrentAttempt    string `json:"currentAttempt"`
	CurrentGeneration string `json:"currentGeneration"`
	Assignment        uint64 `json:"assignment,string"`
	OwnerReleased     bool   `json:"ownerReleased"`

	Worktrees []WorktreeRecord `json:"worktrees"`

	Control         string `json:"control"`
	EffectDirectory string `json:"effectDirectory"`
	Group           string `json:"group"`
	StartedAt       string `json:"startedAt"`
	Turns           uint64 `json:"turns,string"`
	InputTokens     uint64 `json:"inputTokens,string"`
	OutputTokens    uint64 `json:"outputTokens,string"`
	UsageKnown      bool   `json:"usageKnown"`

	WorktreeCommit      string `json:"worktreeCommit"`
	WorktreeIdentity    string `json:"worktreeIdentity"`
	WorktreeGitDir      string `json:"worktreeGitDir"`
	CommonIdentity      string `json:"commonIdentity"`
	IntegrationIdentity string `json:"integrationIdentity"`
	IntegrationGitDir   string `json:"integrationGitDir"`
	IntegrationBranch   string `json:"integrationBranch"`
	CandidateCommit     string `json:"candidateCommit"`
	ResultClass         string `json:"resultClass"`
	ID                  string `json:"id"`
	Profile             string `json:"profile"`
	OwnerPID            int    `json:"ownerPid,string"`
	OwnerStarted        string `json:"ownerStarted"`
	Epoch               uint64 `json:"epoch,string"`
	ConfigSHA256        string `json:"configSha256"`
	Phase               string `json:"phase"`
	Effect              string `json:"effect"`
	Worktree            string `json:"worktree"`
	Base                string `json:"base"`
	LeaderPID           int    `json:"leaderPid,string"`
	LeaderStarted       string `json:"leaderStarted"`
	SessionID           string `json:"sessionId"`
	ResultSHA256        string `json:"resultSha256"`
	Quiescence          string `json:"quiescence"`

	// Repositories are the extra repositories of a multi-repository program
	// (CAL-V0-071), sorted by name; absent keeps single-repository bytes.
	Repositories []RepositoryRecord `json:"repositories,omitempty"`
}
type Programs struct {
	Profile string    `json:"profile"`
	QueueID string    `json:"queueId"`
	Entries []Program `json:"entries"`
}

func (p Programs) Encode() ([]byte, error) {
	raw, e := EncodeProgramJSON(p)
	if e != nil {
		return nil, e
	}
	if _, e = DecodePrograms(raw); e != nil {
		return nil, e
	}
	return raw, nil
}
func DecodePrograms(raw []byte) (*Programs, error) {
	if len(raw) > MaxProgramsBytes {
		return nil, fmt.Errorf("program inventory byte limit")
	}
	if _, e := wire.Parse(raw); e != nil {
		return nil, e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var p Programs
	if e := d.Decode(&p); e != nil {
		return nil, e
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return nil, fmt.Errorf("trailing program input")
	}
	if p.Profile != "taskman-programs/0" || len(p.Entries) > 64 {
		return nil, fmt.Errorf("program profile/count")
	}
	if _, e := wire.ParseQueueID("programs", p.QueueID); e != nil {
		return nil, e
	}
	last := ""
	for _, x := range p.Entries {
		if _, e := wire.ParseIdentifier("program id", x.ID); e != nil {
			return nil, e
		}
		if x.ID <= last || x.Profile != SupervisedProfile || x.OwnerPID <= 0 || x.OwnerStarted == "" || x.Epoch == 0 {
			return nil, fmt.Errorf("program identity/order")
		}
		last = x.ID
		if _, e := wire.ParseDigest("config", x.ConfigSHA256); e != nil {
			return nil, e
		}
		switch x.Phase {
		case "ADMITTED", "WORKTREE_ADD", "WORKTREE_REMOVE", "READY", "SPAWNING", "RUNNING", "STOPPING", "FINISHED", "BLOCKED_RECOVERY":
		default:
			return nil, fmt.Errorf("program phase")
		}
		if e := checkRepositoryRecords(x); e != nil {
			return nil, e
		}
	}
	return &p, nil
}

// EncodeProgramJSON bridges the typed optional record to canonical Tasks JSON.
func EncodeProgramJSON(value any) ([]byte, error) {
	raw, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	var v any
	if e = json.Unmarshal(raw, &v); e != nil {
		return nil, e
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if e = enc.Encode(v); e != nil {
		return nil, e
	}
	parsed, e := wire.Parse(b.Bytes())
	if e != nil {
		return nil, e
	}
	return wire.EncodeFile(parsed), nil
}

// RepositoryRecord binds one extra repository of a multi-repository program
// (CAL-V0-071, CAL-V0-072): its policy name, checkout, shared Git identity,
// assignment base commit and the newest preserved candidate commit, which is
// the base itself when the stage left the repository unchanged. An
// integration designation (CAL-V0-087) names the branch the checkout must
// have checked out and the checkout's directory identity at admission; both
// are absent for a repository that cannot integrate a changed candidate.
type RepositoryRecord struct {
	Name                string `json:"name"`
	Checkout            string `json:"checkout"`
	CommonIdentity      string `json:"commonIdentity"`
	Base                string `json:"base"`
	Candidate           string `json:"candidate,omitempty"`
	IntegrationBranch   string `json:"integrationBranch,omitempty"`
	IntegrationIdentity string `json:"integrationIdentity,omitempty"`
}

func checkRepositoryRecords(x Program) error {
	if len(x.Repositories) > intent.MaxSupervisedRepositories {
		return fmt.Errorf("program repository count")
	}
	names := map[string]bool{}
	for i, r := range x.Repositories {
		if !intent.ValidRepositoryName(r.Name) || (i > 0 && x.Repositories[i-1].Name >= r.Name) || r.Checkout == "" || r.CommonIdentity == "" || r.Base == "" {
			return fmt.Errorf("program repository record")
		}
		if (r.IntegrationBranch == "") != (r.IntegrationIdentity == "") {
			return fmt.Errorf("program repository integration designation")
		}
		if r.IntegrationBranch != "" {
			if _, e := wire.ParseLabel("integrationBranch", r.IntegrationBranch); e != nil {
				return e
			}
		}
		names[r.Name] = true
	}
	for _, w := range x.Worktrees {
		if w.Repository != "" && !names[w.Repository] {
			return fmt.Errorf("program worktree names an undeclared repository")
		}
	}
	return nil
}
