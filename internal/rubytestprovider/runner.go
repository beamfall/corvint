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
	// MaxSpecFileBytes bounds one snapshot file; it equals the bound discovery
	// reads a retained spec or helper file under, so a file the run bound is
	// always one discovery can compare (LPCV-V0-092, LPCV-V0-094).
	MaxSpecFileBytes     = 4 << 20
	maxReportBytes       = 32 << 20
	maxRunnerOutputBytes = 1 << 20
	maxRSpecIDBytes      = 512
	reportDirectory      = ".corvint/rspec"
	// evidenceDirectory is testvaliditydoc.EvidenceDirectory, which imports
	// this package.
	evidenceDirectory = ".corvint/test-evidence"
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

// maxSnapshotEntries bounds every directory entry a spec-tree walk visits,
// whatever its type; tests may lower it.
var maxSnapshotEntries = 16384

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
	if secretscreen.MatchString(project) {
		return Document{}, errors.New("--dir is secret-shaped")
	}
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
	before := takeSnapshot(ctx, root, document.ObjectFormat)

	observation := procgroup.Run(ctx, procgroup.Spec{Argv: resolveArgv(argv), Dir: dir, Env: os.Environ(), Timeout: timeout, OutputLimit: maxRunnerOutputBytes, OverflowPolicy: procgroup.OverflowTruncate})
	report, reportErr := readBounded(root, jsonReport, maxReportBytes)
	var junit []byte
	var junitErr error
	if config.JUnit {
		junit, junitErr = readBounded(root, junitReport, maxReportBytes)
	}
	_ = root.Remove(jsonReport)
	_ = root.Remove(junitReport)

	after := takeSnapshot(ctx, root, document.ObjectFormat)
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
		if len(document.Examples) == MaxExamples {
			document.ExamplesOmitted += len(parsed.Examples) - index
			break
		}
		example, state := observeExample(raw, project, document.HelperSet, before, after)
		switch state {
		case exampleInvalid:
			document.Examples = []Example{}
			document.ExamplesOmitted = 0
			document.Infrastructure = &Infrastructure{Reason: "report-invalid", Detail: "an example has no project-relative file path or positive line"}
			return document, nil
		case exampleWithheld:
			// A secret-shaped file path is never retained; the example is
			// counted as omitted and the summary still carries its outcome.
			document.ExamplesOmitted++
			continue
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
	case observation.Signal != "":
		return &Infrastructure{Reason: "runner-signalled", Detail: "rspec was terminated by a signal; a report it wrote is not a complete run"}
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

const (
	exampleRetained = iota
	exampleInvalid
	exampleWithheld
)

func observeExample(raw rspecExample, project, helperSet string, before, after snapshot) (Example, int) {
	file := path.Clean(strings.TrimPrefix(raw.FilePath, "./"))
	if raw.LineNumber < 1 || !fs.ValidPath(file) || file == "." {
		return Example{}, exampleInvalid
	}
	worktreeFile := path.Join(project, file)
	if secretscreen.MatchString(worktreeFile) {
		return Example{}, exampleWithheld
	}
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
	return example, exampleRetained
}

func closedStatus(status string) string {
	switch status {
	case statusPassed, statusFailed, statusPending:
		return status
	}
	return "unknown"
}

// isHelperPath reports whether a project-relative name is, or lies under, a
// conventional helper path: spec/spec_helper.rb, spec/rails_helper.rb or
// spec/support.
func isHelperPath(name string) bool {
	return name == "spec/spec_helper.rb" || name == "spec/rails_helper.rb" || name == "spec/support" || strings.HasPrefix(name, "spec/support/")
}

// bindHelpers records the conventional helper files present before the run:
// spec/spec_helper.rb, spec/rails_helper.rb and spec/support/**/*.rb. A helper
// path that is not a regular file (a symlink, say) or is secret-shaped leaves
// the helper set explicitly unbound rather than silently smaller.
func bindHelpers(document *Document, before, after snapshot) {
	switch {
	case before.unbound != "":
		document.HelpersUnbound = before.unbound
	case after.unbound != "":
		document.HelpersUnbound = after.unbound
	}
	for _, name := range append(append([]string{}, before.irregular...), after.irregular...) {
		if isHelperPath(name) && document.HelpersUnbound == "" {
			document.HelpersUnbound = "helper-not-regular-file"
		}
	}
	for _, name := range before.sorted() {
		if !isHelperPath(name) {
			continue
		}
		blob := before.files[name]
		worktreeName := path.Join(document.ProjectDirectory, name)
		switch {
		case secretscreen.MatchString(worktreeName):
			document.HelpersUnbound = "helper-path-secret-shaped"
			continue
		case blob == "":
			document.HelpersUnbound = "helper-over-bound"
			continue
		}
		if after.files[name] != blob && document.HelpersUnbound == "" {
			document.HelpersUnbound = "helper-changed-during-run"
		}
		document.Helpers = append(document.Helpers, BoundFile{Path: worktreeName, Blob: blob})
	}
	if document.HelpersUnbound != "" {
		document.Helpers = []BoundFile{}
	}
	document.HelperSet = HelperSetDigest(document.Helpers)
}

// snapshot maps each regular spec/**/*.rb file, project-relative, to its blob
// ID ("" when over the per-file bound). irregular lists the entries that are
// neither directories nor regular files, never followed. unbound names why no
// snapshot exists.
type snapshot struct {
	files     map[string]string
	irregular []string
	unbound   string
}

func (s snapshot) sorted() []string {
	names := make([]string, 0, len(s.files))
	for name := range s.files {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}

func takeSnapshot(ctx context.Context, root *os.Root, format string) snapshot {
	result := snapshot{files: map[string]string{}}
	err := walkTree(ctx, root, "spec", func(name string, mode fs.FileMode) error {
		if !mode.IsRegular() {
			result.irregular = append(result.irregular, name)
			return nil
		}
		if !strings.HasSuffix(name, ".rb") {
			return nil
		}
		if len(result.files) == MaxSnapshotFiles {
			return errSnapshotOverBound
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		data, err := readBounded(root, name, MaxSpecFileBytes)
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

// CurrentHelpers lists, sorted and worktree-relative, the conventional helper
// files of projectDirectory as they are now, under the same paths and entry
// bound as a run. A helper path that is not a regular file, or a tree over the
// bound, is an error: the current set cannot be compared (LPCV-V0-094).
func CurrentHelpers(root *os.Root, projectDirectory string) ([]string, error) {
	prefix := ""
	if projectDirectory != "." {
		prefix = projectDirectory + "/"
	}
	info, err := root.Lstat(filepath.FromSlash(prefix + "spec"))
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errNotDirectory
	}
	helpers := []string{}
	visit := func(name string, mode fs.FileMode) error {
		if !isHelperPath(strings.TrimPrefix(name, prefix)) {
			return nil
		}
		if !mode.IsRegular() {
			return errors.New("a helper path is not a regular file")
		}
		if strings.HasSuffix(name, ".rb") {
			helpers = append(helpers, name)
		}
		return nil
	}
	for _, name := range []string{prefix + "spec/spec_helper.rb", prefix + "spec/rails_helper.rb"} {
		info, err := root.Lstat(filepath.FromSlash(name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := visit(name, info.Mode().Type()); err != nil {
			return nil, err
		}
	}
	if err := walkTree(context.Background(), root, prefix+"spec/support", visit); err != nil {
		return nil, err
	}
	sortStrings(helpers)
	return helpers, nil
}

// walkTree visits every non-directory entry under start without following a
// symlink, counting every entry against maxSnapshotEntries. A missing start is
// empty; a start that is not a real directory is errNotDirectory.
func walkTree(ctx context.Context, root *os.Root, start string, visit func(name string, mode fs.FileMode) error) error {
	info, err := root.Lstat(filepath.FromSlash(start))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errNotDirectory
	}
	pending, entries := []string{start}, 0
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		children, err := listDirectory(root, name, &entries)
		if err != nil {
			return err
		}
		for _, child := range children {
			childName := name + "/" + child.Name()
			if child.IsDir() {
				pending = append(pending, childName)
				continue
			}
			if err := visit(childName, child.Type()); err != nil {
				return err
			}
		}
	}
	return nil
}

// listDirectory reads one directory in chunks, refusing it once the walk's
// entry count passes maxSnapshotEntries, and only when the opened directory is
// the one a no-follow Lstat admitted.
func listDirectory(root *os.Root, name string, entries *int) ([]fs.DirEntry, error) {
	admitted, err := root.Lstat(filepath.FromSlash(name))
	if err != nil {
		return nil, err
	}
	if !admitted.IsDir() {
		return nil, errNotDirectory
	}
	directory, err := root.Open(filepath.FromSlash(name))
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	opened, err := directory.Stat()
	if err != nil || !os.SameFile(admitted, opened) {
		return nil, errors.New("directory changed while it was opened")
	}
	var children []fs.DirEntry
	for {
		batch, err := directory.ReadDir(256)
		*entries += len(batch)
		if *entries > maxSnapshotEntries {
			return nil, errSnapshotOverBound
		}
		children = append(children, batch...)
		if errors.Is(err, io.EOF) || err == nil && len(batch) == 0 {
			return children, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

var (
	errOverBound         = errors.New("file is over its bound")
	errSnapshotOverBound = errors.New("spec snapshot is over its bound")
	errNotDirectory      = errors.New("not a real directory")
)

// readBounded reads one regular file under root without following a final
// symlink or blocking on a special file, refusing content over limit and a
// file that is not the one the Lstat admitted.
func readBounded(root *os.Root, name string, limit int64) ([]byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	file, err := openNoFollow(root, name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, errors.New("file changed while it was opened")
	}
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
			if err := ensureIgnored(root, name); err != nil {
				return err
			}
		}
	}
	// A pre-existing .corvint/.gitignore may not ignore rspec/, so the report
	// directory carries its own (LPCV-V0-095).
	return ensureIgnored(root, reportDirectory)
}

// ensureIgnored writes directory/.gitignore containing "*" when absent and
// never overwrites an existing one (LPCV-V0-095).
func ensureIgnored(root *os.Root, directory string) error {
	file, err := root.OpenFile(directory+"/.gitignore", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
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

// IgnoreEvidence ensures the worktree's .corvint/ and its test-evidence
// directory are ignored before a retained document is written there, whatever
// an existing .corvint/.gitignore says (LPCV-V0-095).
func IgnoreEvidence(worktree string) error {
	root, err := os.OpenRoot(worktree)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, name := range []string{".corvint", evidenceDirectory} {
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
		if err := ensureIgnored(root, name); err != nil {
			return err
		}
	}
	return nil
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
