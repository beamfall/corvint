package rubytestprovider

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

const (
	maxSnapshotFileBytes = 8 << 20
	maxReportBytes       = 32 << 20
	maxRunnerOutputBytes = 1 << 20
	maxRSpecIDBytes      = 512
	reportDirectory      = ".corvint/rspec"
	// DefaultTimeout bounds one RSpec run.
	DefaultTimeout = 30 * time.Minute
)

// Config is one RSpec run. Dir is the absolute project directory RSpec runs
// in; ExecPrefix, when present, runs RSpec through a wrapper such as
// `docker compose run --rm app` whose working directory must be the same
// project directory. Git is always observed on the host, never through the
// prefix (LPCV-V0-091).
type Config struct {
	Dir        string
	ExecPrefix []string
	RSpec      []string
	Specs      []string
	JUnit      bool
	Timeout    time.Duration
}

// observeRepository names the host test repository; tests may replace it.
var observeRepository = jstestprovider.ObserveTestRepositoryAtStart

// EnclosingWorktree is the nearest symlink-resolved ancestor of dir (dir
// included) holding a .git entry, the root corvint test-validity --discover is
// pointed at.
func EnclosingWorktree(dir string) (string, error) {
	current, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("no Git worktree encloses --dir")
		}
		current = parent
	}
}

// Run runs RSpec once and returns the observation document. An error is a
// refusal before any run; every failure after the run starts is recorded as
// Infrastructure in the returned document instead.
func Run(ctx context.Context, config Config) (Document, error) {
	if !filepath.IsAbs(config.Dir) || filepath.Clean(config.Dir) != config.Dir {
		return Document{}, errors.New("--dir must be an absolute clean path")
	}
	dir, err := filepath.EvalSymlinks(config.Dir)
	if err != nil {
		return Document{}, err
	}
	worktree, err := EnclosingWorktree(dir)
	if err != nil {
		return Document{}, err
	}
	relative, err := filepath.Rel(worktree, dir)
	if err != nil || !filepath.IsLocal(relative) {
		return Document{}, errors.New("--dir is not inside its worktree")
	}
	project := filepath.ToSlash(relative)
	for _, spec := range config.Specs {
		if !filepath.IsLocal(spec) {
			return Document{}, errors.New("--spec must be a project-relative path")
		}
	}
	rspec := config.RSpec
	if len(rspec) == 0 {
		rspec = []string{"bundle", "exec", "rspec"}
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return Document{}, err
	}
	defer root.Close()
	if err := ensureReportDirectory(root); err != nil {
		return Document{}, err
	}
	nonce, err := randomHex()
	if err != nil {
		return Document{}, err
	}
	jsonReport := reportDirectory + "/" + nonce + ".json"
	junitReport := reportDirectory + "/" + nonce + ".xml"
	argv := append(append([]string{}, config.ExecPrefix...), rspec...)
	argv = append(argv, "--format", "json", "--out", jsonReport)
	if config.JUnit {
		argv = append(argv, "--format", "RspecJunitFormatter", "--out", junitReport)
	}
	argv = append(argv, config.Specs...)

	document := Document{Profile: Profile, Runner: "rspec", ProjectDirectory: project, ObjectFormat: "sha1", HelperBasis: HelperBasis, Examples: []Example{}, Helpers: []BoundFile{}, JUnit: JUnit{State: "NOT_REQUESTED"}}
	if secretscreen.MatchArgv(argv) {
		document.CommandWithheld = "secret-shaped"
	} else {
		document.Command = argv
	}
	document.RepositoryBefore = observeRepository(ctx, dir)
	if document.RepositoryBefore != nil && len(document.RepositoryBefore.Revision) == 64 {
		document.ObjectFormat = "sha256"
	}
	before := takeSnapshot(root, document.ObjectFormat)

	observation := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(argv), Dir: dir, Env: os.Environ(), Timeout: timeout, OutputLimit: maxRunnerOutputBytes, OverflowPolicy: procgroup.OverflowTruncate})
	report, reportErr := readBounded(root, jsonReport, maxReportBytes)
	var junit []byte
	var junitErr error
	if config.JUnit {
		junit, junitErr = readBounded(root, junitReport, maxReportBytes)
	}
	_ = root.Remove(jsonReport)
	_ = root.Remove(junitReport)

	after := takeSnapshot(root, document.ObjectFormat)
	document.RepositoryAfter = observeRepository(ctx, dir)
	if config.JUnit {
		if junitErr != nil {
			document.JUnit = JUnit{State: "NOT_PRODUCED", Reason: "junit-report-unavailable"}
		} else {
			sum := sha256.Sum256(junit)
			document.JUnit = JUnit{State: "PRODUCED", Digest: "sha256:" + hex.EncodeToString(sum[:])}
		}
	}
	if observation.ExitObserved {
		status := observation.ExitStatus
		document.ExitStatus = &status
	}
	bindHelpers(&document, before, after)
	if failure := runFailure(observation); failure != nil {
		document.Infrastructure = failure
		return document, nil
	}
	if reportErr != nil {
		document.Infrastructure = &Infrastructure{Reason: "report-missing", Detail: "rspec did not write a readable JSON report; the project needs the json formatter and a working directory equal to --dir"}
		return document, nil
	}
	parsed, err := parseReport(report)
	if err != nil {
		document.Infrastructure = &Infrastructure{Reason: "report-invalid", Detail: err.Error()}
		return document, nil
	}
	document.RunnerVersion = parsed.version()
	document.Summary = &Summary{Examples: parsed.Summary.ExampleCount, Failures: parsed.Summary.FailureCount, Pending: parsed.Summary.PendingCount, ErrorsOutsideOfExample: parsed.Summary.ErrorsOutsideOfExamplesCount}
	for index, raw := range parsed.Examples {
		if index == MaxExamples {
			document.ExamplesOmitted = len(parsed.Examples) - index
			break
		}
		example, ok := observeExample(raw, project, document.HelperSet, before, after)
		if !ok {
			document.Examples = []Example{}
			document.ExamplesOmitted = 0
			document.Infrastructure = &Infrastructure{Reason: "report-invalid", Detail: "an example has no project-relative file path or positive line"}
			return document, nil
		}
		document.Examples = append(document.Examples, example)
	}
	switch {
	case parsed.Summary.ExampleCount != len(parsed.Examples):
		document.Infrastructure = &Infrastructure{Reason: "report-count-mismatch", Detail: "summary example_count differs from the reported examples"}
	case parsed.Summary.ErrorsOutsideOfExamplesCount > 0:
		document.Infrastructure = &Infrastructure{Reason: "rspec-errors-outside-examples", Detail: strconv.Itoa(parsed.Summary.ErrorsOutsideOfExamplesCount) + " error(s) occurred outside of examples"}
	case observation.ExitStatus != 0 && parsed.Summary.FailureCount == 0:
		document.Infrastructure = &Infrastructure{Reason: "runner-exit-unexplained", Detail: "rspec exited " + strconv.Itoa(observation.ExitStatus) + " with no failed example"}
	}
	return document, nil
}

func runFailure(observation procgroup.Observation) *Infrastructure {
	switch {
	case !observation.Started:
		return &Infrastructure{Reason: "start-failed", Detail: "the rspec command could not be started"}
	case observation.TimedOut:
		return &Infrastructure{Reason: "timeout", Detail: "rspec did not complete within the --timeout bound"}
	case observation.Cancelled:
		return &Infrastructure{Reason: "cancelled", Detail: "the run was cancelled"}
	case !observation.WaitCompleted || !observation.ExitObserved:
		return &Infrastructure{Reason: "runner-exit-unobserved", Detail: "the rspec exit status was not observed"}
	}
	return nil
}

type rspecReport struct {
	Version  *string        `json:"version"`
	Examples []rspecExample `json:"examples"`
	Summary  struct {
		ExampleCount                 int `json:"example_count"`
		FailureCount                 int `json:"failure_count"`
		PendingCount                 int `json:"pending_count"`
		ErrorsOutsideOfExamplesCount int `json:"errors_outside_of_examples_count"`
	} `json:"summary"`
}

type rspecExample struct {
	ID              string `json:"id"`
	FullDescription string `json:"full_description"`
	Status          string `json:"status"`
	FilePath        string `json:"file_path"`
	LineNumber      int    `json:"line_number"`
}

func parseReport(data []byte) (rspecReport, error) {
	var report rspecReport
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&report); err != nil {
		return rspecReport{}, errors.New("rspec JSON report does not decode")
	}
	if report.Examples == nil {
		return rspecReport{}, errors.New("rspec JSON report has no examples member")
	}
	return report, nil
}

func (report rspecReport) version() string {
	if report.Version == nil || len(*report.Version) > 64 {
		return ""
	}
	for _, c := range *report.Version {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '.' || c == '+' || c == '-') {
			return ""
		}
	}
	return *report.Version
}

func observeExample(raw rspecExample, project, helperSet string, before, after snapshot) (Example, bool) {
	file := path.Clean(strings.TrimPrefix(raw.FilePath, "./"))
	if raw.LineNumber < 1 || !fs.ValidPath(file) || file == "." {
		return Example{}, false
	}
	worktreeFile := path.Join(project, file)
	example := Example{ID: worktreeFile + ":" + strconv.Itoa(raw.LineNumber), File: worktreeFile, Line: raw.LineNumber, Status: closedStatus(raw.Status), Helpers: helperSet}
	if len(raw.ID) <= maxRSpecIDBytes && !secretscreen.MatchString(raw.ID) {
		example.RSpecID = raw.ID
	}
	switch {
	case len(raw.FullDescription) > MaxDescriptionBytes:
		example.DescriptionWithheld = "over-bound"
	case secretscreen.MatchString(raw.FullDescription):
		example.DescriptionWithheld = "secret-shaped"
	default:
		example.Description = raw.FullDescription
	}
	blob, inBefore := before.files[file]
	switch {
	case before.unbound != "":
		example.SpecBlobUnbound = before.unbound
	case !inBefore:
		example.SpecBlobUnbound = "spec-file-outside-snapshot"
	case blob == "":
		example.SpecBlobUnbound = "spec-file-over-bound"
	case after.unbound != "" || after.files[file] != blob:
		example.SpecBlobUnbound = "spec-file-changed-during-run"
	default:
		example.SpecBlob = blob
	}
	return example, true
}

func closedStatus(status string) string {
	switch status {
	case statusPassed, statusFailed, statusPending:
		return status
	}
	return "unknown"
}

// bindHelpers records the conventional helper files present before the run:
// spec/spec_helper.rb, spec/rails_helper.rb and spec/support/**/*.rb.
func bindHelpers(document *Document, before, after snapshot) {
	switch {
	case before.unbound != "":
		document.HelpersUnbound = before.unbound
	case after.unbound != "":
		document.HelpersUnbound = after.unbound
	}
	for _, name := range before.sorted() {
		if name != "spec/spec_helper.rb" && name != "spec/rails_helper.rb" && !strings.HasPrefix(name, "spec/support/") {
			continue
		}
		blob := before.files[name]
		if blob == "" {
			document.HelpersUnbound = "helper-over-bound"
			continue
		}
		if after.files[name] != blob && document.HelpersUnbound == "" {
			document.HelpersUnbound = "helper-changed-during-run"
		}
		document.Helpers = append(document.Helpers, BoundFile{Path: path.Join(document.ProjectDirectory, name), Blob: blob})
	}
	if document.HelpersUnbound != "" {
		document.Helpers = []BoundFile{}
	}
	document.HelperSet = HelperSetDigest(document.Helpers)
}

// snapshot maps each regular spec/**/*.rb file, project-relative, to its blob
// ID ("" when over the per-file bound). unbound names why no snapshot exists.
type snapshot struct {
	files   map[string]string
	unbound string
}

func (s snapshot) sorted() []string {
	names := make([]string, 0, len(s.files))
	for name := range s.files {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}

func takeSnapshot(root *os.Root, format string) snapshot {
	result := snapshot{files: map[string]string{}}
	err := fs.WalkDir(root.FS(), "spec", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			if name == "spec" && errors.Is(err, fs.ErrNotExist) {
				return fs.SkipAll
			}
			return err
		}
		if !entry.Type().IsRegular() || !strings.HasSuffix(name, ".rb") {
			return nil
		}
		if len(result.files) == MaxSnapshotFiles {
			return errSnapshotOverBound
		}
		data, err := readBounded(root, name, maxSnapshotFileBytes)
		if errors.Is(err, errOverBound) {
			result.files[name] = ""
			return nil
		}
		if err != nil {
			return err
		}
		result.files[name] = BlobID(format, data)
		return nil
	})
	switch {
	case errors.Is(err, errSnapshotOverBound):
		return snapshot{files: map[string]string{}, unbound: "spec-snapshot-over-bound"}
	case err != nil:
		return snapshot{files: map[string]string{}, unbound: "spec-snapshot-unavailable"}
	}
	return result
}

var (
	errOverBound         = errors.New("file is over its bound")
	errSnapshotOverBound = errors.New("spec snapshot is over its bound")
)

// readBounded reads one regular file under root without following a final
// symlink, refusing content over limit.
func readBounded(root *os.Root, name string, limit int64) ([]byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errOverBound
	}
	return data, nil
}

// ensureReportDirectory creates .corvint/rspec under the project directory,
// refusing a symlinked component, and ignores .corvint so the run's own
// artifacts never make the repository dirty (LPCV-V0-095).
func ensureReportDirectory(root *os.Root) error {
	for _, name := range []string{".corvint", reportDirectory} {
		if err := root.Mkdir(name, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", name)
		}
		if name == ".corvint" {
			if err := ensureIgnored(root); err != nil {
				return err
			}
		}
	}
	return nil
}

// ensureIgnored writes .corvint/.gitignore containing "*" when absent and
// never overwrites an existing one (LPCV-V0-095).
func ensureIgnored(root *os.Root) error {
	file, err := root.OpenFile(".corvint/.gitignore", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := file.Write([]byte("*\n")); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// IgnoreEvidence ensures the worktree's .corvint/ is ignored before a
// retained document is written there (LPCV-V0-095).
func IgnoreEvidence(worktree string) error {
	root, err := os.OpenRoot(worktree)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Mkdir(".corvint", 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	info, err := root.Lstat(".corvint")
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New(".corvint is not a directory")
	}
	return ensureIgnored(root)
}

func randomHex() (string, error) {
	var buffer [8]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer[:]), nil
}

func resolveArgv(argv []string) []string {
	if len(argv) == 0 {
		return argv
	}
	resolved, err := exec.LookPath(argv[0])
	if err != nil {
		return argv
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		return argv
	}
	return append([]string{absolute}, argv[1:]...)
}
