package rbmutate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// The containerised runner (FPK-V0-078 to FPK-V0-081). A declared profile
// runs the baseline and every mutant through the corvint-test-runner `rspec`
// request with its exec prefix and path mapping (TRE-V0-081 to TRE-V0-086):
// each run gets a fresh directory under the declared workspace holding a
// disposable copy of the export with that run's source written into it, the
// prefix mounts only that directory, and the run's runner receipt is
// admitted against its plan before its report is read. The export itself is
// never written.

// ContainerProfileName is the one admitted declaration profile.
const ContainerProfileName = "corvint-rbmutate-container/0"

const (
	// RunPlaceholder is replaced, in every exec prefix word, by the host
	// path of the run's disposable directory.
	RunPlaceholder = "{run}"
	// The run directory's layout: the copied tree and the fresh report
	// directory the runner creates.
	runTree   = "tree"
	runReport = "report"
	// containerReport is the stock RSpec JSON report the profile writes.
	containerReport = "rspec.json"
	// The runner documents' profiles, as corvint-test-runner writes them.
	runnerPlanProfile    = "corvint-test-runner-plan/0"
	runnerReceiptProfile = "corvint-test-runner-receipt/0"
	// Bounds of one declaration and of one disposable copy.
	maxDeclarationBytes = 64 << 10
	maxPrefixWords      = 64
	maxCopyFiles        = 20000
	maxCopyBytes        = 256 << 20
	maxRunnerTimeout    = 1800
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// errCopyBound is an export too large to copy once per run.
var errCopyBound = errors.New("export exceeds the disposable copy bound")

// ContainerProfile declares the containerised RSpec runner (FPK-V0-078).
// Executable and ExecutableSha256 are the independently pinned identity of
// the container CLI; ExecPrefix follows it in argv and must name
// RunPlaceholder, which each run replaces with its own directory; Container
// is where the prefix mounts that directory; Workspace is the existing host
// directory runs are created in, one the container runtime can mount.
type ContainerProfile struct {
	Profile          string   `json:"profile"`
	Executable       string   `json:"executable"`
	ExecutableSha256 string   `json:"executableSha256"`
	ExecPrefix       []string `json:"execPrefix"`
	Container        string   `json:"container"`
	Workspace        string   `json:"workspace"`
}

// RunnerReceipt names one containerised run's admitted corvint-test-runner
// receipt (FPK-V0-080). The plan binds the run's disposable paths and the
// receipt its output digests, so both identify that run and differ between
// invocations; they are not replay inputs.
type RunnerReceipt struct {
	Profile       string `json:"profile"`
	PlanSHA256    string `json:"plan_sha256"`
	ReceiptSHA256 string `json:"receipt_sha256"`
	Complete      bool   `json:"complete"`
	// ReportError is why the runner did not admit the run's report
	// (FPK-V0-080): ReportUnparsed or ObservationIncomplete. A run with
	// one is never killed or survived.
	ReportError string `json:"report_error,omitempty"`
}

// Why the runner did not admit a containerised run's report (FPK-V0-080).
const (
	// ReportUnparsed is a report the runner's RSpec parser rejects.
	ReportUnparsed = "report-unparsed"
	// ObservationIncomplete is a parsed report whose observation is
	// incomplete: its counts disagree with its examples, or its exit
	// status with its outcomes.
	ObservationIncomplete = "observation-incomplete"
)

// ParseContainerProfile decodes one declaration strictly: bounded, one
// object, no unknown or duplicate-shaped trailing content, and valid.
func ParseContainerProfile(data []byte) (*ContainerProfile, error) {
	if len(data) > maxDeclarationBytes {
		return nil, fmt.Errorf("rbmutate: container profile exceeds %d bytes", maxDeclarationBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var profile ContainerProfile
	if err := decoder.Decode(&profile); err != nil {
		return nil, fmt.Errorf("rbmutate: container profile is not one %s object: %w", ContainerProfileName, err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("rbmutate: container profile has trailing content")
	}
	if err := profile.validate(); err != nil {
		return nil, err
	}
	return &profile, nil
}

// validate checks what can be checked without the host; the rspec profile
// re-admits the prefix and mapping on every run (TRE-V0-081, TRE-V0-082).
func (profile *ContainerProfile) validate() error {
	switch {
	case profile.Profile != ContainerProfileName:
		return fmt.Errorf("rbmutate: container profile is not %s", ContainerProfileName)
	case !filepath.IsAbs(profile.Executable) || filepath.Clean(profile.Executable) != profile.Executable:
		return fmt.Errorf("rbmutate: container executable must be an absolute clean path")
	case !sha256Pattern.MatchString(profile.ExecutableSha256):
		return fmt.Errorf("rbmutate: container executableSha256 must be 64 lowercase hex digits")
	case len(profile.ExecPrefix) == 0 || len(profile.ExecPrefix) > maxPrefixWords:
		return fmt.Errorf("rbmutate: container execPrefix must hold 1 to %d words", maxPrefixWords)
	case !path.IsAbs(profile.Container) || path.Clean(profile.Container) != profile.Container || profile.Container == "/":
		return fmt.Errorf("rbmutate: container path must be absolute, clean and not /")
	case !filepath.IsAbs(profile.Workspace) || filepath.Clean(profile.Workspace) != profile.Workspace || profile.Workspace == string(filepath.Separator):
		return fmt.Errorf("rbmutate: container workspace must be an absolute clean path and not the root")
	}
	named := false
	for _, word := range profile.ExecPrefix {
		named = named || strings.Contains(word, RunPlaceholder)
	}
	if !named {
		return fmt.Errorf("rbmutate: container execPrefix must name %s, the run directory it mounts", RunPlaceholder)
	}
	return nil
}

// specRunner is one way of running the cited spec file against a source.
type specRunner interface {
	name() string
	runSpec(ctx context.Context, source []byte) specRun
}

// findSpecRunner resolves the declared container runner, else the host one.
func findSpecRunner(ctx context.Context, copy Copy, configuration settings) (specRunner, string) {
	if configuration.container != nil {
		return findContainerRunner(copy, configuration)
	}
	return findRunner(ctx, copy, configuration)
}

// containerRunner is one admitted container profile over one export.
type containerRunner struct {
	profile       *ContainerProfile
	copy          Copy
	configuration settings
	// workspace is the declared workspace with symbolic links resolved, so
	// every mapped host path is the one the container runtime mounts.
	workspace string
}

// name is the CLI's base name and the prefix, absolute words cut to base
// names, with the placeholder left in place: no host path.
func (runner containerRunner) name() string {
	return rspecRunner{argv: append([]string{runner.profile.Executable}, runner.profile.ExecPrefix...)}.name()
}

// findContainerRunner checks the host side of the profile before any run:
// the workspace, the pinned CLI, the copy bound, and the rspec profile's
// admission of the prefix and mapping. Each lack is a fixed detail.
func findContainerRunner(copy Copy, configuration settings) (specRunner, string) {
	runner := containerRunner{profile: configuration.container, copy: copy, configuration: configuration}
	workspace, err := filepath.EvalSymlinks(runner.profile.Workspace)
	if err != nil {
		return runner, "the container workspace is not a directory"
	}
	if information, err := os.Stat(workspace); err != nil || !information.IsDir() {
		return runner, "the container workspace is not a directory"
	}
	if overlapsRepository(workspace, configuration.root) {
		return runner, "the container workspace is inside or contains the repository"
	}
	runner.workspace = workspace
	// The runner opens the CLI without following a link, so a declared
	// link is refused here rather than on every run.
	information, err := os.Lstat(runner.profile.Executable)
	if err != nil || !information.Mode().IsRegular() {
		return runner, "the container executable is not a regular file"
	}
	sum, err := boundedDigest(runner.profile.Executable, tr.MaxPinnedArtifactBytes)
	if errors.Is(err, errExecutableBound) {
		return runner, "the container executable exceeds the runner's 256 MiB bound"
	}
	if err != nil || sum != runner.profile.ExecutableSha256 {
		return runner, "the container executable does not match its declared sha256"
	}
	if err := walkExport(copy.Dir(), nil); err != nil {
		if errors.Is(err, errCopyBound) {
			return runner, errCopyBound.Error()
		}
		return runner, "the export cannot be copied for a run"
	}
	probe := filepath.Join(workspace, "corvint-rbmutate-probe")
	request := runner.request(probe, map[string]string{configuration.changedPath: tr.Digest(nil)}, "")
	if _, err := registry.Build(request); err != nil {
		return runner, "the rspec runner profile refuses the declared container profile"
	}
	return runner, ""
}

// errExecutableBound is a container CLI larger than the test runner's
// pinned-executable bound, which the runner would refuse on every run.
var errExecutableBound = errors.New("container executable exceeds the pinned-executable bound")

// boundedDigest hashes the file at name with a streaming read of at most
// bound bytes; a larger file is errExecutableBound and is never read whole.
func boundedDigest(name string, bound int64) (string, error) {
	file, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if information, err := file.Stat(); err != nil || !information.Mode().IsRegular() {
		return "", errors.New("not a regular file")
	} else if information.Size() > bound {
		return "", errExecutableBound
	}
	hash := sha256.New()
	read, err := io.Copy(hash, io.LimitReader(file, bound+1))
	if err != nil {
		return "", err
	}
	if read > bound {
		return "", errExecutableBound
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// overlapsRepository reports whether the resolved workspace is the
// repository root, lies under it, or contains it. Runs create directories
// in the workspace, so either would write into, or expose, the checkout.
// Directories are compared by identity, so a case-insensitive file system
// or a second path to the same directory cannot hide an overlap. An unset
// root (a caller judging a bare copy) has no repository to protect; a root
// that does not resolve is treated as overlapping.
func overlapsRepository(workspace, root string) bool {
	if root == "" {
		return false
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return true
	}
	rootInformation, err := os.Stat(resolved)
	if err != nil {
		return true
	}
	workspaceInformation, err := os.Stat(workspace)
	if err != nil {
		return true
	}
	return hasAncestor(workspace, rootInformation) || hasAncestor(resolved, workspaceInformation)
}

// hasAncestor reports whether dir, or any directory above it, is target.
func hasAncestor(dir string, target os.FileInfo) bool {
	for {
		if information, err := os.Stat(dir); err == nil && os.SameFile(information, target) {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// request is the corvint-test-runner rspec request of one run directory.
func (runner containerRunner) request(runDir string, inputs map[string]string, optionsDigest string) tr.Request {
	prefix := make([]string, len(runner.profile.ExecPrefix))
	for index, word := range runner.profile.ExecPrefix {
		prefix[index] = strings.ReplaceAll(word, RunPlaceholder, runDir)
	}
	request := tr.Request{
		Runner: "rspec", Root: filepath.Join(runDir, runTree), ReportDir: filepath.Join(runDir, runReport),
		TimeoutSeconds: runnerTimeout(runner.configuration), InputFiles: inputs,
		Selectors:  []string{runner.configuration.target},
		Executable: runner.profile.Executable, ExecutableSha256: runner.profile.ExecutableSha256,
		ExecPrefix: prefix, PathMap: []tr.PathMapping{{Host: runDir, Container: runner.profile.Container}},
	}
	if optionsDigest != "" {
		request.Config, request.ConfigSha256 = optionsName, optionsDigest
	}
	return request
}

// runnerTimeout is the per-run timeout in whole seconds, inside the
// runner's 1 to 1800 bound.
func runnerTimeout(configuration settings) int {
	seconds := int(math.Ceil(configuration.perRun.Seconds()))
	return max(1, min(seconds, maxRunnerTimeout))
}

// runSpec runs the cited spec file against source in a fresh disposable
// copy, which is removed afterwards. A run that could not be set up or
// whose receipt is not admitted has no verdict.
func (runner containerRunner) runSpec(ctx context.Context, source []byte) specRun {
	run, err := runner.execute(ctx, source)
	if err != nil {
		return specRun{outcome: runBroken, output: err.Error(), receipt: Receipt{ExitStatus: -1, SourceSHA256: digest(source)}}
	}
	return run
}

func (runner containerRunner) execute(ctx context.Context, source []byte) (specRun, error) {
	runDir, err := os.MkdirTemp(runner.workspace, "corvint-rbmutate-")
	if err != nil {
		return specRun{}, fmt.Errorf("rbmutate: create run directory: %w", err)
	}
	// A container may leave files the host user cannot remove; what
	// remains is outside the export and every later run.
	defer func() { _ = os.RemoveAll(runDir) }()
	tree := filepath.Join(runDir, runTree)
	if err := copyExport(runner.copy.Dir(), tree); err != nil {
		return specRun{}, fmt.Errorf("rbmutate: copy export: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tree, filepath.FromSlash(runner.configuration.changedPath)), source, 0o644); err != nil {
		return specRun{}, fmt.Errorf("rbmutate: write run source: %w", err)
	}
	optionsPath, err := writeOptions(tree)
	if err != nil {
		return specRun{}, fmt.Errorf("rbmutate: write options: %w", err)
	}
	options, err := os.ReadFile(optionsPath)
	if err != nil {
		return specRun{}, err
	}
	spec, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(runner.configuration.testPath)))
	if err != nil {
		return specRun{}, err
	}
	inputs := map[string]string{runner.configuration.changedPath: tr.Digest(source), runner.configuration.testPath: tr.Digest(spec)}
	request := runner.request(runDir, inputs, tr.Digest(options))
	invocation, err := registry.Build(request)
	if err != nil {
		return specRun{}, fmt.Errorf("rbmutate: plan the run: %w", err)
	}
	plan := tr.PlanDocument{Profile: runnerPlanProfile, Request: request, Invocation: invocation}
	execution, runErr := tr.Execute(ctx, request, invocation)
	observation, parseErr := registry.Parse(execution.Input)
	receipt := tr.ReceiptDocument{Profile: runnerReceiptProfile, PlanSha256: tr.Identity(plan), Execution: execution, Observation: observation, Error: joinErrors(runErr, parseErr)}
	if err := admitRunnerReceipt(plan, receipt); err != nil {
		return specRun{}, err
	}
	input := execution.Input
	content, reported := input.Reports[containerReport]
	run := classifyReport(input.ExitCode, input.ExitCode != tr.KilledExitCode || input.TimedOut, input.TimedOut,
		string(input.Stdout)+string(input.Stderr), content, reported, digest(source))
	if runErr != nil && (run.outcome == runPassed || run.outcome == runFailed) {
		// The runner refused the run after it exited: a pinned input,
		// the options file or a template changed under it. Such a run is
		// never credited either way.
		run.outcome = runUnloadable
	}
	reportError := ""
	switch {
	case parseErr != nil:
		reportError = ReportUnparsed
	case !observation.Complete:
		reportError = ObservationIncomplete
	}
	if reportError != "" && (run.outcome == runPassed || run.outcome == runFailed) {
		// The receipt's own parser did not admit the report this
		// classification read, so neither verdict is earned from it.
		run.outcome = runUnadmitted
	}
	run.receipt.Runner = &RunnerReceipt{Profile: runnerReceiptProfile, PlanSHA256: receipt.PlanSha256, ReceiptSHA256: tr.Identity(receipt), Complete: observation.Complete, ReportError: reportError}
	return run, nil
}

// admitRunnerReceipt applies the binding checks corvint-test-runner `run`
// and plain CEM admission apply (TRE-V0-083, CEM-V1-281, CEM-V1-282): the
// plan rebuilds from its own request, the receipt binds that plan, its
// inputs and invocation, and the receipt is a closed document. The inputs
// are this run's bytes, never a revision's blobs, so no target check
// applies.
func admitRunnerReceipt(plan tr.PlanDocument, receipt tr.ReceiptDocument) error {
	rebuilt, err := registry.Build(plan.Request)
	if err != nil || tr.Identity(rebuilt) != tr.Identity(plan.Invocation) {
		return fmt.Errorf("rbmutate: runner plan does not match the rspec runner profile")
	}
	execution := receipt.Execution
	if receipt.Profile != runnerReceiptProfile || receipt.PlanSha256 != tr.Identity(plan) || execution.Runner != plan.Request.Runner ||
		execution.InputSha256 != tr.Identity(plan.Request.InputFiles) || execution.InvocationSha256 != tr.Identity(plan.Invocation) {
		return fmt.Errorf("rbmutate: runner receipt does not bind its plan")
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	var decoded tr.ReceiptDocument
	if err := tr.DecodeDocument(encoded, &decoded); err != nil {
		return fmt.Errorf("rbmutate: runner receipt is not admitted: %w", err)
	}
	return nil
}

func joinErrors(errs ...error) string {
	words := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			words = append(words, err.Error())
		}
	}
	return strings.Join(words, "; ")
}

// copyExport copies the export into target, which must not exist: regular
// files with their permission bits, directories, and symbolic links as
// links. Anything else is left out.
func copyExport(source, target string) error {
	return walkExport(source, func(relative string, entry fs.DirEntry) error {
		from, to := filepath.Join(source, relative), filepath.Join(target, relative)
		switch {
		case entry.IsDir() && relative == ".":
			return os.Mkdir(to, 0o755)
		case entry.IsDir():
			return os.Mkdir(to, 0o755)
		case entry.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(from)
			if err != nil {
				return err
			}
			return os.Symlink(link, to)
		}
		information, err := entry.Info()
		if err != nil {
			return err
		}
		content, err := os.ReadFile(from)
		if err != nil {
			return err
		}
		return os.WriteFile(to, content, information.Mode().Perm())
	})
}

// walkExport visits every directory, regular file and symbolic link of the
// export, refusing more than maxCopyFiles files or maxCopyBytes bytes. A nil
// visit only measures.
func walkExport(source string, visit func(relative string, entry fs.DirEntry) error) error {
	files, size := 0, int64(0)
	return filepath.WalkDir(source, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, name)
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
			if !entry.Type().IsRegular() {
				return nil
			}
			information, err := entry.Info()
			if err != nil {
				return err
			}
			files, size = files+1, size+information.Size()
		}
		if files > maxCopyFiles || size > maxCopyBytes {
			return errCopyBound
		}
		if visit == nil {
			return nil
		}
		return visit(relative, entry)
	})
}
