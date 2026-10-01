// Package testacceptance composes actual, operator-approved experimental
// browser observations. Unknown validity remains unknown.
package testacceptance

import (
	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

const Schema = "corvint-new-e2e-assessment/0"
const RequestSchema = "corvint-new-e2e-request/0"
const InputLimit = 4 << 20
const ReportLimit = 16 << 20

type Repository struct {
	Root   string `json:"root"`
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
}
type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Command struct {
	Argv             []string `json:"argv"`
	EntrypointSHA256 string   `json:"entrypoint_sha256,omitempty"`
	ExecutableSHA256 string   `json:"executable_sha256"`
}
type Test struct {
	ID             string                        `json:"id"`
	File           string                        `json:"file"`
	Line           int                           `json:"line"`
	Title          string                        `json:"title"`
	Project        string                        `json:"project"`
	ControlCommand *Command                      `json:"control_command,omitempty"`
	Control        *behaviorfalsify.Plan         `json:"control,omitempty"`
	Tool           *behaviorfalsify.ToolIdentity `json:"tool,omitempty"`
}
type Request struct {
	Schema         string     `json:"schema"`
	Product        Repository `json:"product"`
	TestRepository Repository `json:"test_repository"`
	Inputs         []File     `json:"inputs"`
	Tests          []Test     `json:"tests"`
	Config         string     `json:"config"`
	Package        string     `json:"package"`
	Lockfile       string     `json:"lockfile"`
	Runner         Command    `json:"runner"`
	Server         Command    `json:"server"`
	ReadyURL       string     `json:"ready_url"`
	AppBuildDir    string     `json:"app_build_dir"`
	RunnerVersion  string     `json:"runner_version"`
	Environment    string     `json:"environment"`
	Repeat         int        `json:"repeat"`
	TimeoutSeconds int        `json:"timeout_seconds"`
}
type Row struct {
	ID              string                        `json:"id"`
	ObservedID      string                        `json:"observed_id,omitempty"`
	State           jstestprovider.ExecutionState `json:"state"`
	DurationMS      float64                       `json:"duration_ms"`
	Retries         int                           `json:"retries"`
	Attempts        int                           `json:"attempts"`
	Validity        testvalidity.Projection       `json:"validity"`
	IdentityUnknown []string                      `json:"identity_unknown"`
}
type Cleanup struct {
	OwnedGroup    bool                             `json:"owned_group"`
	Status        string                           `json:"status"`
	Qualification string                           `json:"qualification"`
	Descendants   *procgroup.DescendantObservation `json:"descendants,omitempty"`
	Cancelled     bool                             `json:"cancelled"`
	TimedOut      bool                             `json:"timed_out"`
}
type Run struct {
	Kind             string                            `json:"kind"`
	Ordinal          int                               `json:"ordinal"`
	TestID           string                            `json:"test_id,omitempty"`
	RequestedFiles   []string                          `json:"requested_files"`
	ObservedSchedule *jstestprovider.ExecutionSchedule `json:"observed_schedule,omitempty"`
	ReceiptSHA256    string                            `json:"receipt_sha256,omitempty"`
	Rows             []Row                             `json:"rows"`
	Reasons          []string                          `json:"reasons"`
	Cleanup          Cleanup                           `json:"cleanup"`
	NodeVersion      string                            `json:"node_version,omitempty"`
}
type Control struct {
	ID            string   `json:"id"`
	PlanDigest    string   `json:"plan_digest,omitempty"`
	ReceiptDigest string   `json:"receipt_digest,omitempty"`
	Status        string   `json:"status"`
	Unsupported   []string `json:"unsupported"`
	Cleanup       Cleanup  `json:"cleanup"`
}
type Assessment struct {
	ID       string                  `json:"id"`
	Verdict  string                  `json:"verdict"`
	Reasons  []string                `json:"reasons"`
	Validity testvalidity.Projection `json:"validity"`
	Repeats  RepeatSummary           `json:"repeats"`
	Cleanup  string                  `json:"cleanup"`
	Order    OrderEvidence           `json:"order"`
}

// RepeatSummary counts actual zero-retry repeat rows for one declared test.
// Durations are present only when at least one row was observed.
type RepeatSummary struct {
	Passed        int      `json:"passed"`
	Failed        int      `json:"failed"`
	Other         int      `json:"other"`
	MinDurationMS *float64 `json:"min_duration_ms,omitempty"`
	MaxDurationMS *float64 `json:"max_duration_ms,omitempty"`
}

// OrderEvidence attaches the order-dependence probes to the test they concern.
// Requested file filters and isolation runs are requests, not an observed schedule.
type OrderEvidence struct {
	Status             string   `json:"status"`
	RequestedFileOrder string   `json:"requested_file_order"`
	OriginalState      string   `json:"original_state"`
	ReversedState      string   `json:"reversed_state"`
	IsolatedStates     []string `json:"isolated_states"`
}
type Report struct {
	Verdict          string       `json:"verdict"`
	Reasons          []string     `json:"reasons"`
	Schema           string       `json:"schema"`
	RequestDigest    string       `json:"request_digest"`
	Product          Repository   `json:"product"`
	TestRepository   Repository   `json:"test_repository"`
	Environment      string       `json:"environment"`
	Build            string       `json:"build"`
	ExecutableSHA256 string       `json:"executable_sha256"`
	Runs             []Run        `json:"runs"`
	Controls         []Control    `json:"controls"`
	Assessments      []Assessment `json:"assessments"`
	Unknowns         []string     `json:"unknowns"`
	Body             string       `json:"body"`
}

// Job is private to the self-worker protocol; it cannot supply observations.
type Job struct {
	Approved  string   `json:"approved"`
	Request   Request  `json:"request"`
	Kind      string   `json:"kind"`
	Files     []string `json:"files,omitempty"`
	TestIndex int      `json:"test_index"`
}
type WorkerResult struct {
	Run     *Run     `json:"run,omitempty"`
	Control *Control `json:"control,omitempty"`
	Error   string   `json:"error,omitempty"`
}
