// Package postmergemetrics produces caller-owned, recommendation-only evidence.
package postmergemetrics

import "context"

const (
	PolicyLimit  = 1 << 20
	HistoryLimit = 16 << 20
	maxDuration  = int64(31536000000)
)

type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrInput     Error = "invalid-input"
	ErrPolicy    Error = "invalid-policy"
	ErrHistory   Error = "invalid-history"
	ErrWindow    Error = "invalid-window"
	ErrGit       Error = "git-evidence-unavailable"
	ErrCancelled Error = "deadline-or-cancelled"
)

type Policy struct {
	Profile string  `json:"profile"`
	Classes []Class `json:"classes"`
}
type Class struct {
	ID              string `json:"id"`
	MaxCorrectionBP int64  `json:"maxCorrectionBP"`
	MinSamples      int    `json:"minSamples"`
	DemoteRuns      int    `json:"demoteRuns"`
}
type History struct {
	Profile string      `json:"profile"`
	Classes []Inventory `json:"classes"`
	Runs    []Run       `json:"runs"`
	Events  []Event     `json:"events"`
}
type Inventory struct {
	Class          string `json:"class"`
	LastSequence   int    `json:"lastSequence"`
	RunsComplete   bool   `json:"runsComplete"`
	EventsComplete bool   `json:"eventsComplete"`
	RunsThrough    string `json:"runsThrough"`
	EventsThrough  string `json:"eventsThrough"`
}
type Run struct {
	ID       string   `json:"id"`
	Class    string   `json:"class"`
	Sequence int      `json:"sequence"`
	At       string   `json:"at"`
	Outcome  string   `json:"outcome"`
	Stages   []Stage  `json:"stages"`
	Followup Followup `json:"followup"`
	Git      *GitPair `json:"git"`
}
type Stage struct {
	Name       string `json:"name"`
	Outcome    string `json:"outcome"`
	DurationMS *int64 `json:"durationMs"`
}
type Followup struct {
	Status     string `json:"status"`
	DurationMS *int64 `json:"durationMs"`
}
type GitPair struct {
	Root     string `json:"root"`
	Bot      string `json:"bot"`
	Approved string `json:"approved"`
}
type Event struct {
	ID    string `json:"id"`
	RunID string `json:"runID"`
	At    string `json:"at"`
	Kind  string `json:"kind"`
}

type Measurement struct {
	Bot          string `json:"bot"`
	Approved     string `json:"approved"`
	BotTree      string `json:"botTree"`
	ApprovedTree string `json:"approvedTree"`
	GitVersion   string `json:"gitVersion"`
	Convention   string `json:"convention"`
	Files        int64  `json:"files"`
	BinaryFiles  int64  `json:"binaryFiles"`
	GitlinkFiles int64  `json:"gitlinkFiles"`
	TextAdded    int64  `json:"textAdded"`
	TextDeleted  int64  `json:"textDeleted"`
	Added        *int64 `json:"added"`
	Deleted      *int64 `json:"deleted"`
}
type Measurer interface {
	Measure(context.Context, GitPair) (Measurement, error)
}
type RunResult struct {
	Run         Run          `json:"run"`
	Measurement *Measurement `json:"measurement"`
	Corrected   bool         `json:"corrected"`
	Reverted    bool         `json:"reverted"`
	Demoted     bool         `json:"demoted"`
}
type ClassResult struct {
	Class                string     `json:"class"`
	Policy               Class      `json:"policy"`
	Inventory            *Inventory `json:"inventory"`
	HistoryComplete      bool       `json:"historyComplete"`
	Total                int64      `json:"total"`
	Generated            int64      `json:"generated"`
	NoChange             int64      `json:"noChange"`
	Failed               int64      `json:"failed"`
	Unknown              int64      `json:"unknown"`
	Corrected            int64      `json:"corrected"`
	Reverted             int64      `json:"reverted"`
	Demoted              int64      `json:"demoted"`
	CorrectionEvents     int64      `json:"correctionEvents"`
	RevertEvents         int64      `json:"revertEvents"`
	InvalidFindingEvents int64      `json:"invalidFindingEvents"`
	ContainmentEvents    int64      `json:"containmentEvents"`
	EditedFiles          int64      `json:"editedFiles"`
	TextAdded            int64      `json:"textAdded"`
	TextDeleted          int64      `json:"textDeleted"`
	UnknownEditRuns      int64      `json:"unknownEditRuns"`
	RemainingDemotion    int        `json:"remainingDemotion"`
	Recommendation       string     `json:"recommendation"`
	Reasons              []string   `json:"reasons"`
}
type Report struct {
	Profile       string        `json:"profile"`
	Authority     string        `json:"authority"`
	Provenance    string        `json:"provenance"`
	From          string        `json:"from"`
	Until         string        `json:"until"`
	PolicySHA256  string        `json:"policySHA256"`
	HistorySHA256 string        `json:"historySHA256"`
	Classes       []ClassResult `json:"classes"`
	Runs          []RunResult   `json:"runs"`
}
