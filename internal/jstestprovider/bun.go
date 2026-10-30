package jstestprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// Bun runner profile (LPCV-V0-254..262, V1-2191, GitHub #1405): `bun test` with the JUnit
// reporter, bound to the observed bun version, the bun.lock digest, the preload scripts and the
// exact command, under the same procgroup containment and receipt as the Vitest unit adapter.

// BunRefusalSchema names the typed pre-run bun refusal document (LPCV-V0-258).
const BunRefusalSchema = "corvint-bun-test-refusal/0"

// Bun refusal codes (LPCV-V0-258). Each is decided before `bun test` starts.
const (
	BunVersionUnqualified  = "bun-version-unqualified"
	BunLockfileUnqualified = "bun-lockfile-unqualified"
	BunIsolationUndeclared = "bun-isolation-concurrency-undeclared"
)

// BunIsolationSharedService is the declared-isolation profile of a suite that runs against
// caller-started services (LPCV-V0-259).
const BunIsolationSharedService = "declared-shared-service"

// qualifiedBunVersions are the exact bun releases a live run of this profile observed
// (LPCV-V0-257). A version is admitted one exact release at a time, never as a range.
var qualifiedBunVersions = []string{"1.3.11"}

// qualifiedBunLockfileVersions are the text bun.lock lockfileVersion values the profile reads.
var qualifiedBunLockfileVersions = []int{1}

// QualifiedBunVersions returns a copy of the admitted bun releases.
func QualifiedBunVersions() []string { return slices.Clone(qualifiedBunVersions) }

const (
	bunVersionTimeout   = 10 * time.Second
	bunReportLimit      = defaultOutputLimit
	bunMaxPreloads      = 16
	bunMaxServices      = 16
	bunLockfileHeadSize = 4096
)

// BunConfig configures one `bun test` run. Config.TestFiles are both the bound files and the
// positional path selectors; Config.Lockfile must be a text bun.lock.
type BunConfig struct {
	Config
	// Executable is the bun binary; "" resolves "bun" on PATH.
	Executable string
	// Preload scripts are passed as --preload in order and bound by digest.
	Preload []string
	// MaxConcurrency, when positive, is passed as --max-concurrency.
	MaxConcurrency int
	// TestNamePattern, when not empty, is passed as --test-name-pattern.
	TestNamePattern string
	// Services are caller-started services the suite depends on, as NAME=IDENTITY declarations.
	Services []string
	// ReportFile is where --reporter-outfile writes, removed before the run; a temp file if empty.
	ReportFile string
}

// BunRefusal is a pre-run bun refusal with one closed code.
type BunRefusal struct {
	Code   string
	Detail string
}

func (e *BunRefusal) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + ": " + e.Detail
}

type bunRefusalDocument struct {
	Schema                    string   `json:"schema"`
	Code                      string   `json:"code"`
	Message                   string   `json:"message"`
	QualifiedBunVersions      []string `json:"qualifiedBunVersions"`
	QualifiedLockfileVersions []int    `json:"qualifiedLockfileVersions"`
}

// EncodeBunRefusal returns the canonical refusal document plus LF.
func EncodeBunRefusal(r *BunRefusal) []byte {
	data, _ := json.Marshal(bunRefusalDocument{Schema: BunRefusalSchema, Code: r.Code, Message: r.Error(),
		QualifiedBunVersions: QualifiedBunVersions(), QualifiedLockfileVersions: slices.Clone(qualifiedBunLockfileVersions)})
	return append(data, '\n')
}

// ProducerBuild is the provider binary's own build identity (LPCV-V0-256).
type ProducerBuild struct {
	Name      string `json:"name"`
	Module    string `json:"module"`
	Revision  string `json:"revision"`
	Modified  bool   `json:"modified"`
	GoVersion string `json:"goVersion"`
}

// BunRun is the bun profile's receipt member (LPCV-V0-256): what ran, against which bun and
// lockfile, with which preloads, concurrency and declared services.
type BunRun struct {
	Producer            ProducerBuild     `json:"producer"`
	Executable          string            `json:"executable"`
	ExecutableDigest    string            `json:"executableDigest"`
	Version             string            `json:"version"`
	Lockfile            string            `json:"lockfile"`
	LockfileDigest      string            `json:"lockfileDigest"`
	LockfileVersion     int               `json:"lockfileVersion"`
	Preload             []string          `json:"preload"`
	PreloadDigests      map[string]string `json:"preloadDigests"`
	MaxConcurrency      int               `json:"maxConcurrency"`
	TestNamePattern     string            `json:"testNamePattern,omitempty"`
	Isolation           string            `json:"isolation,omitempty"`
	Services            []string          `json:"services,omitempty"`
	ServiceBoundary     string            `json:"serviceBoundary,omitempty"`
	ExitObserved        bool              `json:"exitObserved"`
	ExitStatus          int               `json:"exitStatus"`
	RepositoryUnchanged *bool             `json:"repositoryUnchanged,omitempty"`
}

var bunLockfileVersion = regexp.MustCompile(`"lockfileVersion"\s*:\s*(\d+)`)

// bunMemberShapeError refuses a bun member on any receipt but an unprofiled unit one
// (LPCV-V0-256); otherwise it returns next.
func bunMemberShapeError(r Receipt, next error) error {
	if r.Bun != nil && (r.Profile != "" || r.Kind != "unit") {
		return errors.New("bun-member-requires-unit-default-profile")
	}
	return next
}

// AdmitBun decides every pre-run refusal and returns the resolved executable, observed version
// and lockfile version (LPCV-V0-257..259). It starts no test.
func AdmitBun(ctx context.Context, cfg BunConfig) (string, string, int, error) {
	if len(cfg.Services) > 0 && cfg.MaxConcurrency != 1 {
		return "", "", 0, &BunRefusal{Code: BunIsolationUndeclared, Detail: "a suite declared against a caller-started service must run with --max-concurrency=1"}
	}
	lockVersion, err := bunLockfile(cfg.Lockfile)
	if err != nil {
		return "", "", 0, err
	}
	executable := cfg.Executable
	if executable == "" {
		executable = "bun"
	}
	resolved, err := exec.LookPath(executable)
	if err != nil {
		return "", "", 0, &BunRefusal{Code: BunVersionUnqualified, Detail: "bun executable not found: " + err.Error()}
	}
	if abs, err := filepath.Abs(resolved); err == nil {
		resolved = abs
	}
	version, err := bunVersion(ctx, resolved)
	if err != nil {
		return "", "", 0, &BunRefusal{Code: BunVersionUnqualified, Detail: "bun version unobserved: " + err.Error()}
	}
	if !slices.Contains(qualifiedBunVersions, version) {
		return "", "", 0, &BunRefusal{Code: BunVersionUnqualified, Detail: fmt.Sprintf("observed bun %s; qualified bun versions: %s", version, strings.Join(qualifiedBunVersions, ", "))}
	}
	if cfg.RunnerVersion != "" && cfg.RunnerVersion != version {
		return "", "", 0, &BunRefusal{Code: BunVersionUnqualified, Detail: fmt.Sprintf("pinned bun %s but observed %s", cfg.RunnerVersion, version)}
	}
	return resolved, version, lockVersion, nil
}

// bunLockfile reads the lockfileVersion of a text bun.lock. A missing, binary (bun.lockb),
// unreadable or other-version lockfile is refused (LPCV-V0-258).
func bunLockfile(path string) (int, error) {
	refuse := func(detail string) (int, error) {
		return 0, &BunRefusal{Code: BunLockfileUnqualified, Detail: detail}
	}
	if path == "" {
		return refuse("no --lockfile; the profile binds a text bun.lock")
	}
	if filepath.Base(path) != "bun.lock" {
		return refuse(fmt.Sprintf("%s is not a text bun.lock", filepath.Base(path)))
	}
	f, err := os.Open(path)
	if err != nil {
		return refuse(err.Error())
	}
	defer f.Close()
	head := make([]byte, bunLockfileHeadSize)
	n, _ := f.Read(head)
	m := bunLockfileVersion.FindSubmatch(head[:n])
	if m == nil {
		return refuse("bun.lock names no lockfileVersion")
	}
	version, _ := strconv.Atoi(string(m[1]))
	if !slices.Contains(qualifiedBunLockfileVersions, version) {
		return refuse(fmt.Sprintf("bun.lock lockfileVersion %d is not qualified", version))
	}
	return version, nil
}

func bunVersion(ctx context.Context, executable string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, bunVersionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	version := strings.TrimSpace(out.String())
	if version == "" || len(version) > 64 || strings.ContainsAny(version, " \n\t") {
		return "", errors.New("bun --version printed no single version")
	}
	return version, nil
}

// currentProducer is this binary's build identity from its embedded build information.
func currentProducer() ProducerBuild {
	p := ProducerBuild{Name: "corvint-js-test-provider", Revision: "unrecorded", GoVersion: runtime.Version()}
	if info, ok := debug.ReadBuildInfo(); ok {
		p.Module = info.Main.Path
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				p.Revision = s.Value
			case "vcs.modified":
				p.Modified = s.Value == "true"
			}
		}
	}
	return p
}

// BunArgv is the exact `bun test` command line (LPCV-V0-255).
func BunArgv(executable, report string, cfg BunConfig) []string {
	argv := []string{executable, "test", "--reporter=junit", "--reporter-outfile=" + report}
	for _, p := range cfg.Preload {
		argv = append(argv, "--preload", p)
	}
	if cfg.MaxConcurrency > 0 {
		argv = append(argv, "--max-concurrency="+strconv.Itoa(cfg.MaxConcurrency))
	}
	if cfg.TestNamePattern != "" {
		argv = append(argv, "--test-name-pattern="+cfg.TestNamePattern)
	}
	return append(argv, cfg.TestFiles...)
}

// RunBun admits, runs `bun test` under procgroup containment and returns the bound receipt. A
// typed *BunRefusal is returned before anything runs (LPCV-V0-258); a failed test is an
// observation, and a nonzero exit the report does not explain is an infrastructure failure, as
// for Vitest.
func RunBun(ctx context.Context, cfg BunConfig) (Receipt, error) {
	if len(cfg.TestFiles) == 0 {
		return Receipt{}, errors.New("the bun profile needs at least one --test-file")
	}
	if len(cfg.Preload) > bunMaxPreloads || len(cfg.Services) > bunMaxServices {
		return Receipt{}, fmt.Errorf("at most %d preloads and %d services", bunMaxPreloads, bunMaxServices)
	}
	executable, version, lockVersion, err := AdmitBun(ctx, cfg)
	if err != nil {
		return Receipt{}, err
	}
	report := cfg.ReportFile
	if report == "" {
		f, err := os.CreateTemp("", "corvint-bun-*.xml")
		if err != nil {
			return Receipt{}, err
		}
		report = f.Name()
		f.Close()
		defer os.Remove(report)
	}
	if err := os.Remove(report); err != nil && !os.IsNotExist(err) {
		return Receipt{}, err
	}
	argv := BunArgv(executable, report, cfg)
	cfg.RunnerName, cfg.RunnerVersion = "bun", version
	identity, err := cfg.identity(ctx, argv)
	if err != nil {
		return Receipt{}, err
	}
	identity.NodeVersion = ""
	lockDigest, err := digestFile(cfg.Lockfile)
	if err != nil {
		return Receipt{}, err
	}
	executableDigest, err := digestFile(executable)
	if err != nil {
		return Receipt{}, err
	}
	preloads := make(map[string]string, len(cfg.Preload))
	for _, p := range cfg.Preload {
		path := p
		if !filepath.IsAbs(path) {
			path = filepath.Join(cfg.Dir, p)
		}
		if preloads[p], err = digestFile(path); err != nil {
			return Receipt{}, err
		}
	}
	run := &BunRun{Producer: currentProducer(), Executable: executable, ExecutableDigest: executableDigest, Version: version,
		Lockfile: cfg.Lockfile, LockfileDigest: lockDigest, LockfileVersion: lockVersion, Preload: append([]string{}, cfg.Preload...),
		PreloadDigests: preloads, MaxConcurrency: cfg.MaxConcurrency, TestNamePattern: cfg.TestNamePattern}
	if len(cfg.Services) > 0 {
		run.Isolation, run.Services = BunIsolationSharedService, append([]string{}, cfg.Services...)
		run.ServiceBoundary = "caller-started services are declared, not observed; the run proves no service state"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	receipt := Receipt{Kind: "unit", Identity: identity, Bun: run,
		AppBuildAtStart: AppBuildIdentity{Unknown: true, Reason: "bun test runs against source directly, not a served build"}}
	receipt.AppBuildAtPublish = receipt.AppBuildAtStart
	receipt.TestRepositoryAtStart, _ = observeTestRepository(ctx, cfg.Dir)
	obs := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: cfg.Dir, Env: os.Environ(), Timeout: timeout, OutputLimit: outputLimitOrDefault(cfg.OutputLimit)})
	run.ExitObserved, run.ExitStatus = obs.ExitObserved, obs.ExitStatus
	receipt.TestRepositoryAtPublish, _ = observeTestRepository(context.WithoutCancel(ctx), cfg.Dir)
	if receipt.TestRepositoryAtStart != nil && receipt.TestRepositoryAtPublish != nil {
		same := *receipt.TestRepositoryAtStart == *receipt.TestRepositoryAtPublish
		run.RepositoryUnchanged = &same
	}
	if boundary := unitBoundaryFailure(obs, "bun", timeout); boundary != nil {
		receipt.Infrastructure, receipt.Cancelled = boundary, obs.Cancelled
		return receipt, nil
	}
	data, err := readBoundedReport(report, bunReportLimit)
	if err != nil {
		receipt.Infrastructure = &InfrastructureFailure{Reason: "report-not-written", Detail: err.Error()}
		return receipt, nil
	}
	tests, infra, err := ParseBunJUnit(data)
	if err != nil {
		receipt.Infrastructure = &InfrastructureFailure{Reason: "report-unparseable", Detail: err.Error()}
		return receipt, nil
	}
	receipt.Tests, receipt.Infrastructure = tests, infra
	if receipt.Infrastructure == nil && unexplainedNonzeroExit(obs, tests) {
		receipt.Infrastructure = &InfrastructureFailure{Reason: "exit-status-unexplained",
			Detail: fmt.Sprintf("bun exited %d but the JUnit report has no failed or infrastructure outcome to account for it", obs.ExitStatus)}
	}
	return receipt, nil
}

type bunCase struct {
	Name     string     `xml:"name,attr"`
	File     string     `xml:"file,attr"`
	Line     string     `xml:"line,attr"`
	Time     string     `xml:"time,attr"`
	Failures []bunIssue `xml:"failure"`
	Errors   []bunIssue `xml:"error"`
	Skipped  *bunIssue  `xml:"skipped"`
}

type bunIssue struct {
	Type    string `xml:"type,attr"`
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

type bunSuite struct {
	Name   string     `xml:"name,attr"`
	File   string     `xml:"file,attr"`
	Tests  *int       `xml:"tests,attr"`
	Cases  []bunCase  `xml:"testcase"`
	Suites []bunSuite `xml:"testsuite"`
}

// ParseBunJUnit parses one `bun test --reporter=junit` report into outcomes (LPCV-V0-260). Each
// test case carries its file and declaring line as its anchor, and its full name is the describe
// chain and test name joined by " > ". Bun reports a failing beforeAll/afterAll hook as an
// unnamed case with no line; it surfaces as one infrastructure outcome, never a test result. A
// declared count that disagrees with the cases, or a report with no case, is a run-level failure.
func ParseBunJUnit(data []byte) ([]TestOutcome, *InfrastructureFailure, error) {
	var root struct {
		XMLName xml.Name
		bunSuite
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	if err := decoder.Decode(&root); err != nil {
		return nil, nil, err
	}
	if root.XMLName.Local != "testsuites" && root.XMLName.Local != "testsuite" {
		return nil, nil, errors.New("not a bun JUnit report")
	}
	var out []TestOutcome
	mismatch := ""
	var walk func(s bunSuite, chain []string) int
	walk = func(s bunSuite, chain []string) int {
		n := len(s.Cases)
		for _, c := range s.Cases {
			out = append(out, bunOutcome(c, chain))
		}
		for _, child := range s.Suites {
			next := chain
			if child.File == "" || child.Name != child.File {
				next = append(slices.Clone(chain), child.Name)
			}
			n += walk(child, next)
		}
		if s.Tests != nil && *s.Tests != n && mismatch == "" {
			mismatch = s.Name
		}
		return n
	}
	walk(root.bunSuite, nil)
	if mismatch != "" {
		return out, &InfrastructureFailure{Reason: "report-count-mismatch", Detail: fmt.Sprintf("suite %q declares a test count other than its cases", mismatch)}, nil
	}
	if len(out) == 0 {
		return nil, &InfrastructureFailure{Reason: "no-suites-collected", Detail: "the bun JUnit report lists no test case"}, nil
	}
	return out, nil, nil
}

func bunOutcome(c bunCase, chain []string) TestOutcome {
	full := strings.Join(append(slices.Clone(chain), c.Name), " > ")
	outcome := TestOutcome{Name: c.Name, FullName: full, State: StatePassed}
	if seconds, err := strconv.ParseFloat(c.Time, 64); err == nil {
		outcome.DurationMS = seconds * 1000
	}
	if line, err := strconv.Atoi(c.Line); err == nil && line > 0 && c.File != "" {
		outcome.Anchor = &Anchor{File: c.File, Line: line}
	}
	switch {
	case c.Name == "(unnamed)" && c.Line == "" && (len(c.Failures) > 0 || len(c.Errors) > 0):
		outcome.State, outcome.FullName = StateInfrastructure, strings.Join(append(slices.Clone(chain), "(hook)"), " > ")
		outcome.FailureMessage = "a beforeAll or afterAll hook failed in " + c.File
	case len(c.Errors) > 0:
		outcome.State, outcome.FailureMessage = StateInfrastructure, issueText(c.Errors[0])
	case len(c.Failures) > 0:
		outcome.State, outcome.FailureMessage = StateFailed, issueText(c.Failures[0])
	case c.Skipped != nil:
		outcome.State = StateSkipped
	}
	return outcome
}

func issueText(i bunIssue) string {
	for _, s := range []string{i.Message, strings.TrimSpace(i.Text), i.Type} {
		if s != "" {
			return s
		}
	}
	return "failed"
}
