package testrunner

import "path/filepath"

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
}
