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
	// leader's group (TRE-V0-025); omitted when false, so older plans keep
	// their bytes.
	RetireDetachedDescendants bool `json:"retireDetachedDescendants,omitempty"`
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
