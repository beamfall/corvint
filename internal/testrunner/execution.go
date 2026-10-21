package testrunner

import (
	"path/filepath"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// ExecutionTempDir is the private TMPDIR the executor gives every phase of a
// request; profiles whose native tools derive paths from TMPDIR bound it here.
func ExecutionTempDir(reportDir string) string { return filepath.Join(reportDir, ".tmp") }

type PhaseResult struct {
	Kind         string `json:"kind"`
	ToolSha256   string `json:"toolSha256"`
	ArgvSha256   string `json:"argvSha256"`
	StdoutSha256 string `json:"stdoutSha256"`
	StderrSha256 string `json:"stderrSha256"`
	ExitCode     int    `json:"exitCode"`
	TimedOut     bool   `json:"timedOut"`
	Interrupted  bool   `json:"interrupted"`
	Overflow     bool   `json:"overflow"`
}
type Execution struct {
	Profile            string            `json:"profile"`
	Runner             string            `json:"runner"`
	InputSha256        string            `json:"inputSha256"`
	InvocationSha256   string            `json:"invocationSha256"`
	Phases             []PhaseResult     `json:"phases"`
	ReportSha256       map[string]string `json:"reportSha256"`
	Input              Input             `json:"-"`
	ExecutionAuthority string            `json:"executionAuthority"`
	DependencyClosure  string            `json:"dependencyClosure"`
	// Retirement is present only for plans that request detached retirement.
	Retirement *groupreap.Retirement `json:"retirement,omitempty"`
	// DeviceCleanup is present only for plans that declare device cleanup.
	DeviceCleanup *DeviceCleanup `json:"deviceCleanup,omitempty"`
}

// Device cleanup outcomes (TRE-V0-087). Only a run that timed out or was
// interrupted needs cleanup; it is clean only when every step succeeded.
const (
	CleanupNotRequired = "not-required"
	CleanupClean       = "clean"
	CleanupIncomplete  = "incomplete"
)

// DeviceCleanup records the device-side stop of a timed-out or interrupted run.
type DeviceCleanup struct {
	Outcome string              `json:"outcome"`
	Steps   []CleanupStepResult `json:"steps"`
}

// CleanupStepResult is one bounded cleanup step. Status is ok, failed,
// timed-out or unobserved; ExitCode is -1 for a step that ran no process.
type CleanupStepResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	ExitCode int    `json:"exitCode"`
	Detail   string `json:"detail,omitempty"`
}
