// Package testrunner defines experimental, runner-neutral execution observations.
// Its parsers describe native reports; no state proves semantic test adequacy.
package testrunner

const (
	Passed                 = "PASSED"
	Failed                 = "FAILED"
	Skipped                = "SKIPPED"
	Flaky                  = "FLAKY"
	TimedOut               = "TIMED_OUT"
	Interrupted            = "INTERRUPTED"
	Unknown                = "UNKNOWN"
	Assertion              = "ASSERTION"
	BuildFailure           = "BUILD"
	CollectionFailure      = "COLLECTION"
	Infrastructure         = "INFRASTRUCTURE"
	Retained               = "RETAINED"
	NotReported            = "NOT_REPORTED"
	NotApplicable          = "NOT_APPLICABLE"
	MaxPinnedArtifactBytes = 256 << 20
	MaxReportBytes         = 4 << 20
	MaxTests               = 4096
	MaxAttempts            = 32
	MaxReports             = 32
)

type Tool struct {
	Executable string `json:"executable"`
	Sha256     string `json:"sha256"`
}
type Request struct {
	ExpectedTests    []string          `json:"expectedTests"`
	Target           string            `json:"target"`
	InputFiles       map[string]string `json:"inputFiles"`
	Runner           string            `json:"runner"`
	Root             string            `json:"root"`
	Executable       string            `json:"executable"`
	ExecutableSha256 string            `json:"executableSha256"`
	Selectors        []string          `json:"selectors"`
	Project          string            `json:"project"`
	Config           string            `json:"config"`
	ConfigSha256     string            `json:"configSha256"`
	Reporter         string            `json:"reporter"`
	ReporterSha256   string            `json:"reporterSha256"`
	ReportFiles      []string          `json:"reportFiles"`
	Tools            map[string]Tool   `json:"tools"`
	ReportDir        string            `json:"reportDir"`
	TimeoutSeconds   int               `json:"timeoutSeconds"`
	// ExpectedSelection is omitted when absent so historical plan bytes and
	// identities are unchanged.
	ExpectedSelection *Selection `json:"expectedSelection,omitempty"`
	// ExecPrefix, PathMap and Formatter are admitted only for the rspec
	// profile (TRE-V0-081..085). Each is omitted when absent, so every other
	// plan keeps its historical bytes and identity (TRE-V0-086).
	ExecPrefix []string      `json:"execPrefix,omitempty"`
	PathMap    []PathMapping `json:"pathMap,omitempty"`
	Formatter  string        `json:"formatter,omitempty"`
}

// HasRSpecOnlyMembers reports whether any member admitted only for the rspec
// profile is present; every other profile refuses such a request (TRE-V0-081).
func (r Request) HasRSpecOnlyMembers() bool {
	return len(r.ExecPrefix) > 0 || len(r.PathMap) > 0 || r.Formatter != ""
}

// PathMapping names one host directory and the path at which the exec
// prefix's runtime sees it (TRE-V0-082).
type PathMapping struct {
	Host      string `json:"host"`
	Container string `json:"container"`
}
type Phase struct {
	StdoutReport string            `json:"stdoutReport"`
	Kind         string            `json:"kind"`
	Tool         string            `json:"tool"`
	Argv         []string          `json:"argv"`
	Environment  map[string]string `json:"environment"`
}
type Invocation struct {
	OutcomeNeutralExitCodes []int             `json:"outcomeNeutralExitCodes"`
	SuccessExitCodes        []int             `json:"successExitCodes"`
	FailureExitCodes        []int             `json:"failureExitCodes"`
	Argv                    []string          `json:"argv"`
	Phases                  []Phase           `json:"phases"`
	Files                   map[string][]byte `json:"files"`
	ReportPaths             []string          `json:"reportPaths"`
	ReportPatterns          []string          `json:"reportPatterns"`
	Environment             map[string]string `json:"environment"`
	Format                  string            `json:"format"`
	GracefulInterrupt       bool              `json:"gracefulInterrupt,omitempty"`
	// RetireDetachedDescendants proves and retires processes that left the
	// leader's group (TRE-V0-030); omitted when false, so older plans keep
	// their bytes.
	RetireDetachedDescendants bool `json:"retireDetachedDescendants,omitempty"`
	// DeviceCleanup is the bounded device-side stop a device-bound profile
	// declares for a timed-out or interrupted run (TRE-V0-087); omitted when
	// empty, so older plans keep their bytes.
	DeviceCleanup []CleanupStep `json:"deviceCleanup,omitempty"`
}

// CleanupStep is one device-cleanup action (TRE-V0-087). A tool step runs
// Argv with the declared Tool. A WebDriver step deletes the session whose
// identity the test phase wrote to SessionFile in the report directory, on
// the canonical loopback endpoint WebDriver.
type CleanupStep struct {
	Name string   `json:"name"`
	Tool string   `json:"tool,omitempty"`
	Argv []string `json:"argv,omitempty"`
	// Expect is the exact last stdout line a tool step must print to count
	// as done, so a transport that drops the remote exit status cannot pass.
	Expect      string `json:"expect,omitempty"`
	WebDriver   string `json:"webDriver,omitempty"`
	SessionFile string `json:"sessionFile,omitempty"`
}
type Input struct {
	Target                  string            `json:"target"`
	SourceRoot              string            `json:"sourceRoot"`
	Selectors               []string          `json:"selectors"`
	SourceFile              string            `json:"sourceFile"`
	ExecutionProblems       []Problem         `json:"executionProblems"`
	OutcomeNeutralExitCodes []int             `json:"outcomeNeutralExitCodes"`
	SuccessExitCodes        []int             `json:"successExitCodes"`
	FailureExitCodes        []int             `json:"failureExitCodes"`
	Runner                  string            `json:"runner"`
	Reports                 map[string][]byte `json:"reports"`
	Stdout                  []byte            `json:"stdout"`
	Stderr                  []byte            `json:"stderr"`
	ExitCode                int               `json:"exitCode"`
	TimedOut                bool              `json:"timedOut"`
	Interrupted             bool              `json:"interrupted"`
	Overflow                bool              `json:"overflow"`
	Expected                []string          `json:"expected"`
	ExpectedSelection       *Selection        `json:"expectedSelection,omitempty"`
}
type Attempt struct {
	State       string `json:"state"`
	FailureKind string `json:"failureKind"`
	Message     string `json:"message"`
}
type Test struct {
	Granularity   string    `json:"granularity,omitempty"`
	ExecutedCount *int      `json:"executedCount,omitempty"`
	SkippedCount  *int      `json:"skippedCount,omitempty"`
	FailureKind   string    `json:"failureKind"`
	Message       string    `json:"message"`
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	File          string    `json:"file"`
	Suite         string    `json:"suite"`
	State         string    `json:"state"`
	Attempts      []Attempt `json:"attempts"`
	// InclusionLocations is the native shared-group inclusion backtrace of an
	// RSpec example under the corvint-rspec-json/0 formatter (TRE-V0-085);
	// omitted when the example was not included through a shared group.
	InclusionLocations []string `json:"inclusionLocations,omitempty"`
}
type Problem struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}
type Observation struct {
	Runner           string    `json:"runner"`
	Tests            []Test    `json:"tests"`
	Problems         []Problem `json:"problems"`
	Complete         bool      `json:"complete"`
	RetryInformation string    `json:"retryInformation"`
}
