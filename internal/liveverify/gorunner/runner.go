// Package gorunner executes one bounded, explicit Go test plan.
package gorunner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxOutputBytes   = int64(8 << 20)
	MaxCoverageBytes = int64(256 << 20)
	// MaxPackages leaves room for the executable and four frozen go arguments
	// within the 4,096-element argv bound.
	MaxPackages   = 4_091
	MaxArgvBytes  = 4 << 20
	MaxEnvEntries = 256
	MaxEnvBytes   = 1 << 20
	MaxRunTime    = 30 * time.Minute
	MaxPipeWait   = 2 * time.Second
)

var (
	ErrInvalidPlan         = errors.New("gorunner: invalid plan")
	ErrUnsupportedPlatform = errors.New("gorunner: unsupported platform")
	ErrStart               = errors.New("gorunner: start failed")
	ErrWait                = errors.New("gorunner: wait failed")
	ErrPipe                = errors.New("gorunner: pipe drain failed")
	ErrPipeWait            = errors.New("gorunner: pipe drain timed out")
	ErrContainment         = errors.New("gorunner: process-group cleanup failed")
)

type Containment string

const (
	ContainmentProcessGroupBestEffort Containment = "PROCESS_GROUP_BEST_EFFORT"
	ContainmentUnsupported            Containment = "UNSUPPORTED"
)

type EnvironmentVariable struct {
	Name  string
	Value string
}

type Plan struct {
	// GoExecutable and WorkingDirectory must be clean absolute paths whose final
	// components are not symlinks. Mandatory parent-component, source, toolchain,
	// and environment identity checks belong to the coordinator immediately
	// before and after Run; Run exposes no potentially unbounded callbacks.
	GoExecutable     string
	WorkingDirectory string
	Environment      []EnvironmentVariable
	Packages         []string
	OutputLimitBytes int64
	// RetainOutput keeps at most OutputLimitBytes across both Result Data fields.
	// Data is transient caller-owned output and is nil when RetainOutput is false.
	// Bounded retention deliberately replaces arbitrary streaming callbacks: an
	// uncooperative callback could defeat the hard return bound.
	RetainOutput bool
	Timeout      time.Duration
	// Coverage is an opt-in combined-run coverage request. Nil preserves the
	// exact non-coverage invocation and result.
	Coverage *CoverageRequest
}

type StreamResult struct {
	Bytes uint64
	// Data is a transient bounded copy and is nil unless Plan.RetainOutput is true.
	Data      []byte
	Drained   bool
	RawSHA256 string
}

type Result struct {
	Argv                []string
	Cancelled           bool
	Containment         Containment
	Duration            time.Duration
	ExitCode            int
	Exited              bool
	OutputLimitExceeded bool
	PipeWaitExpired     bool
	ProcessCleanupDone  bool
	Started             bool
	Stderr              StreamResult
	Stdout              StreamResult
	TimedOut            bool
	Coverage            CoverageObservation
}

func Run(ctx context.Context, plan Plan) (result Result, runErr error) {
	plan = clonePlan(plan)
	containment, supported := platformContainment()
	result = Result{Containment: containment, ExitCode: -1, Coverage: absentCoverageObservation()}
	if plan.Coverage != nil {
		result.Coverage = rejectedCoverageObservation(plan.Coverage.Mode)
	}
	argv, environment, err := validatePlan(plan)
	if err != nil {
		return result, err
	}
	result.Argv = append([]string{plan.GoExecutable}, argv...)
	coverage, err := prepareCoverage(plan.Coverage)
	if err != nil {
		return result, err
	}
	if coverage != nil {
		defer func() {
			result.Coverage = coverage.finish(result, runErr)
		}()
	}
	if !supported {
		return result, ErrUnsupportedPlatform
	}
	if err := ctx.Err(); err != nil {
		result.Cancelled = true
		return result, err
	}
	budget := newOutputBudget(uint64(plan.OutputLimitBytes))
	stdout := newCapture(budget, plan.RetainOutput)
	stderr := newCapture(budget, plan.RetainOutput)
	runContext, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	command := exec.CommandContext(runContext, plan.GoExecutable, argv...)
	command.Dir = plan.WorkingDirectory
	command.Env = environment
	command.Stdin = bytes.NewReader(nil)
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return result, fmt.Errorf("%w: stdout: %w", ErrPipe, err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return result, fmt.Errorf("%w: stderr: %w", ErrPipe, err)
	}
	command.Stdout = stdoutWriter
	command.Stderr = stderrWriter
	command.WaitDelay = MaxPipeWait
	configureCommand(command)
	var terminationMu sync.Mutex
	var terminationErr error
	cancelFired := false
	command.Cancel = func() error {
		err := terminateCommand(command)
		terminationMu.Lock()
		cancelFired = true
		terminationErr = errors.Join(terminationErr, err)
		terminationMu.Unlock()
		return err
	}

	startedAt := time.Now()
	if err := command.Start(); err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
		result.Duration = time.Since(startedAt)
		result.Stdout = stdout.result(false)
		result.Stderr = stderr.result(false)
		return result, fmt.Errorf("%w: %w", ErrStart, err)
	}
	result.Started = true
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	drains := make(chan error, 2)
	go drainPipe(stdoutReader, stdout, drains)
	go drainPipe(stderrReader, stderr, drains)
	waitDone := make(chan error, 1)
	go func() { waitDone <- command.Wait() }()
	timer := time.NewTimer(plan.Timeout)
	waitErr := error(nil)
	shutdownDeadline := time.Time{}
	select {
	case waitErr = <-waitDone:
		stopTimer(timer)
		// Only the caller's context can have fired Cancel here: the other
		// cases are the ones that call cancelRun. Wait returns only after the
		// cancellation goroutine has reported, so the record is final.
		terminationMu.Lock()
		result.Cancelled = cancelFired
		terminationMu.Unlock()
	case <-ctx.Done():
		result.Cancelled = true
		stopTimer(timer)
		shutdownDeadline = time.Now().Add(MaxPipeWait)
		cancelRun()
		waitErr = <-waitDone
	case <-timer.C:
		result.TimedOut = true
		shutdownDeadline = time.Now().Add(MaxPipeWait)
		cancelRun()
		waitErr = <-waitDone
	case <-budget.exceeded:
		result.OutputLimitExceeded = true
		stopTimer(timer)
		shutdownDeadline = time.Now().Add(MaxPipeWait)
		cancelRun()
		waitErr = <-waitDone
	}
	if shutdownDeadline.IsZero() {
		shutdownDeadline = time.Now().Add(MaxPipeWait)
	}
	cleanupErr := cleanupCommand(command, shutdownDeadline)
	terminationMu.Lock()
	terminationFailure := terminationErr
	terminationMu.Unlock()
	result.ProcessCleanupDone = cleanupErr == nil && terminationFailure == nil
	pipeWait := time.Until(shutdownDeadline)
	pipeErr, pipeWaitExpired := waitForDrains(stdoutReader, stderrReader, drains, pipeWait)
	result.Duration = time.Since(startedAt)
	result.PipeWaitExpired = pipeWaitExpired
	pipesDrained := !pipeWaitExpired && pipeErr == nil
	result.Stdout = stdout.result(pipesDrained)
	result.Stderr = stderr.result(pipesDrained)
	result.OutputLimitExceeded = result.OutputLimitExceeded || budget.wasExceeded()
	if command.ProcessState != nil {
		result.Exited = command.ProcessState.Exited()
		result.ExitCode = command.ProcessState.ExitCode()
	}
	if waitErr != nil {
		var exitError *exec.ExitError
		if !errors.As(waitErr, &exitError) {
			runErr = fmt.Errorf("%w: %w", ErrWait, waitErr)
		}
	}
	if result.PipeWaitExpired {
		runErr = errors.Join(runErr, ErrPipeWait)
	} else if pipeErr != nil {
		runErr = errors.Join(runErr, fmt.Errorf("%w: %w", ErrPipe, pipeErr))
	}
	if terminationFailure != nil || cleanupErr != nil {
		runErr = errors.Join(runErr, fmt.Errorf("%w: %w", ErrContainment, errors.Join(terminationFailure, cleanupErr)))
	}
	return result, runErr
}

func drainPipe(reader *os.File, capture *capture, done chan<- error) {
	_, err := io.Copy(capture, reader)
	_ = reader.Close()
	done <- err
}

func waitForDrains(stdout, stderr *os.File, drains <-chan error, timeout time.Duration) (error, bool) {
	if timeout <= 0 {
		_ = stdout.Close()
		_ = stderr.Close()
		return collectDrains(drains, 2), true
	}
	timer := time.NewTimer(timeout)
	defer stopTimer(timer)
	var result error
	for remaining := 2; remaining > 0; remaining-- {
		select {
		case err := <-drains:
			result = errors.Join(result, err)
		case <-timer.C:
			_ = stdout.Close()
			_ = stderr.Close()
			result = errors.Join(result, collectDrains(drains, remaining))
			return result, true
		}
	}
	return result, false
}

func collectDrains(drains <-chan error, count int) error {
	var result error
	for range count {
		result = errors.Join(result, <-drains)
	}
	return result
}

func clonePlan(plan Plan) Plan {
	plan.Environment = append([]EnvironmentVariable(nil), plan.Environment...)
	plan.Packages = append([]string(nil), plan.Packages...)
	if plan.Coverage != nil {
		coverage := *plan.Coverage
		coverage.Packages = append([]CoveragePackage(nil), coverage.Packages...)
		for index := range coverage.Packages {
			coverage.Packages[index].Files = append([]CoverageSourceFile(nil), coverage.Packages[index].Files...)
		}
		plan.Coverage = &coverage
	}
	return plan
}

func stopTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}

func validatePlan(plan Plan) ([]string, []string, error) {
	if plan.GoExecutable == "" || !filepath.IsAbs(plan.GoExecutable) || filepath.Clean(plan.GoExecutable) != plan.GoExecutable {
		return nil, nil, invalid("GoExecutable must be a clean absolute path")
	}
	info, err := os.Lstat(plan.GoExecutable)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, nil, invalid("GoExecutable must name a non-symlink regular file")
	}
	if plan.WorkingDirectory == "" || !filepath.IsAbs(plan.WorkingDirectory) || filepath.Clean(plan.WorkingDirectory) != plan.WorkingDirectory {
		return nil, nil, invalid("WorkingDirectory must be a clean absolute path")
	}
	info, err = os.Lstat(plan.WorkingDirectory)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, nil, invalid("WorkingDirectory must name a non-symlink directory")
	}
	if plan.Timeout <= 0 || plan.Timeout > MaxRunTime {
		return nil, nil, invalid("Timeout is outside the admitted range")
	}
	if plan.OutputLimitBytes <= 0 || plan.OutputLimitBytes > MaxOutputBytes {
		return nil, nil, invalid("OutputLimitBytes is outside the admitted range")
	}
	if err := validatePackages(plan.Packages); err != nil {
		return nil, nil, err
	}
	environment, err := validateEnvironment(plan.Environment)
	if err != nil {
		return nil, nil, err
	}
	argv := []string{"test", "-json", "-count=1", "-vet=off"}
	if plan.Coverage != nil {
		if err := validateCoverageRequest(*plan.Coverage); err != nil {
			return nil, nil, err
		}
		argv = append(argv, "-covermode="+string(plan.Coverage.Mode), "-coverprofile="+plan.Coverage.ProfilePath)
	}
	argv = append(argv, plan.Packages...)
	if len(argv)+1 > 4_096 {
		return nil, nil, invalid("argv exceeds the admitted element bound")
	}
	total := len(plan.GoExecutable)
	for _, argument := range argv {
		total += len(argument) + 1
	}
	if total > MaxArgvBytes {
		return nil, nil, invalid("argv exceeds the admitted byte bound")
	}
	return argv, environment, nil
}

func validatePackages(packages []string) error {
	if len(packages) == 0 || len(packages) > MaxPackages {
		return invalid("Packages must be non-empty and bounded")
	}
	if len(packages) == 1 && packages[0] == "./..." {
		return nil
	}
	if !sort.StringsAreSorted(packages) {
		return invalid("Packages must be sorted")
	}
	previous := ""
	for _, name := range packages {
		if name == previous {
			return invalid("Packages must not contain duplicates")
		}
		previous = name
		if !validImportPath(name) {
			return invalid("Packages contains a non-admitted import path")
		}
	}
	return nil
}

func validImportPath(name string) bool {
	if name == "" || len(name) > 4_096 || !utf8.ValidString(name) || strings.HasPrefix(name, "-") || strings.ContainsAny(name, "@,\\*?[") {
		return false
	}
	for _, character := range name {
		if unicode.IsSpace(character) || unicode.IsControl(character) {
			return false
		}
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || segment == "." || segment == ".." || segment == "..." {
			return false
		}
	}
	return true
}

func validateEnvironment(values []EnvironmentVariable) ([]string, error) {
	if len(values) > MaxEnvEntries {
		return nil, invalid("Environment exceeds the admitted entry bound")
	}
	result := make([]string, 0, len(values))
	previous := ""
	total := 0
	for _, value := range values {
		if value.Name <= previous {
			return nil, invalid("Environment names must be sorted and unique")
		}
		previous = value.Name
		if !validEnvironmentName(value.Name) || strings.IndexByte(value.Value, 0) >= 0 {
			return nil, invalid("Environment contains an invalid entry")
		}
		entry := value.Name + "=" + value.Value
		total += len(entry) + 1
		if total > MaxEnvBytes {
			return nil, invalid("Environment exceeds the admitted byte bound")
		}
		result = append(result, entry)
	}
	return result, nil
}

func validEnvironmentName(name string) bool {
	if name == "" || name[0] < 'A' || name[0] > 'Z' {
		return false
	}
	for _, character := range name {
		if (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' {
			return false
		}
	}
	return true
}

func invalid(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidPlan, message)
}

type outputBudget struct {
	mu       sync.Mutex
	limit    uint64
	observed uint64
	stored   uint64
	overflow bool
	exceeded chan struct{}
}

func newOutputBudget(limit uint64) *outputBudget {
	return &outputBudget{limit: limit, exceeded: make(chan struct{}, 1)}
}

func (budget *outputBudget) admit(length int) int {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	amount := uint64(length)
	if math.MaxUint64-budget.observed < amount {
		budget.observed = math.MaxUint64
	} else {
		budget.observed += amount
	}
	remaining := uint64(0)
	if budget.stored < budget.limit {
		remaining = budget.limit - budget.stored
	}
	keep := amount
	if keep > remaining {
		keep = remaining
	}
	budget.stored += keep
	if budget.observed > budget.limit && !budget.overflow {
		budget.overflow = true
		budget.exceeded <- struct{}{}
	}
	return int(keep)
}

func (budget *outputBudget) wasExceeded() bool {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	return budget.overflow
}

type capture struct {
	budget *outputBudget
	buffer bytes.Buffer
	bytes  uint64
	digest hash.Hash
	retain bool
}

func newCapture(budget *outputBudget, retain bool) *capture {
	return &capture{
		budget: budget,
		digest: sha256.New(),
		retain: retain,
	}
}

func (capture *capture) Write(value []byte) (int, error) {
	_, _ = capture.digest.Write(value)
	if math.MaxUint64-capture.bytes < uint64(len(value)) {
		capture.bytes = math.MaxUint64
	} else {
		capture.bytes += uint64(len(value))
	}
	keep := capture.budget.admit(len(value))
	if keep > 0 && capture.retain {
		_, _ = capture.buffer.Write(value[:keep])
	}
	return len(value), nil
}

func (capture *capture) result(drained bool) StreamResult {
	return StreamResult{
		Bytes:     capture.bytes,
		Data:      append([]byte(nil), capture.buffer.Bytes()...),
		Drained:   drained,
		RawSHA256: hex.EncodeToString(capture.digest.Sum(nil)),
	}
}
