package rubytestprovider

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/secretscreen"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

const cannedReport = `{"version":"3.13.2","examples":[
{"id":"./spec/models/user_spec.rb[1:1]","description":"saves","full_description":"User saves","status":"passed","file_path":"./spec/models/user_spec.rb","line_number":2},
{"id":"./spec/models/user_spec.rb[1:2]","description":"rejects","full_description":"User rejects a blank name","status":"failed","file_path":"./spec/models/user_spec.rb","line_number":4},
{"id":"./spec/models/user_spec.rb[1:3]","description":"later","full_description":"User later","status":"pending","file_path":"./spec/models/user_spec.rb","line_number":6}],
"summary":{"example_count":3,"failure_count":1,"pending_count":1,"errors_outside_of_examples_count":0}}`

// requireHost skips where the shell fakes or host Git are unavailable; Ruby
// itself is never required.
func requireHost(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fakes need a POSIX shell")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir, "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	command.Env = append(cleanGitEnvironment(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func cleanGitEnvironment() []string {
	var environment []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			environment = append(environment, entry)
		}
	}
	return environment
}

func write(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

// repository is a committed Rails-shaped project with conventional helpers.
func repository(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "config", "user.email", "test@example.invalid")
	git(t, root, "config", "user.name", "test")
	write(t, filepath.Join(root, "spec", "spec_helper.rb"), "RSpec.configure { }\n", 0o644)
	write(t, filepath.Join(root, "spec", "rails_helper.rb"), "require 'spec_helper'\n", 0o644)
	write(t, filepath.Join(root, "spec", "support", "factories.rb"), "# factories\n", 0o644)
	write(t, filepath.Join(root, "spec", "models", "user_spec.rb"), "RSpec.describe User do\n  it 'saves'\n  it 'rejects'\n  it 'later'\nend\n", 0o644)
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "fixture")
	return root
}

// fakeRSpec writes canned JSON to the json (or Corvint formatter) --out
// path, exits 97 when a --require file is absent, and otherwise exits with code.
func fakeRSpec(t *testing.T, report string, code int) string {
	t.Helper()
	bin := t.TempDir()
	canned := filepath.Join(bin, "report.json")
	write(t, canned, report, 0o644)
	script := "#!/bin/sh\nformat=\nprevious=\nfor argument in \"$@\"; do\n" +
		"  case \"$previous\" in\n" +
		"    --format) format=\"$argument\" ;;\n" +
		"    --require) [ -f \"$argument\" ] || exit 97 ;;\n" +
		"    --out) if { [ \"$format\" = json ] || [ \"$format\" = CorvintRSpecJsonFormatter ]; } && [ -s '" + canned + "' ]; then cat '" + canned + "' > \"$argument\"; fi\n" +
		"           if [ \"$format\" = RspecJunitFormatter ]; then echo '<testsuite/>' > \"$argument\"; fi ;;\n" +
		"  esac\n  previous=\"$argument\"\ndone\nexit " + strconv.Itoa(code) + "\n"
	path := filepath.Join(bin, "rspec")
	write(t, path, script, 0o755)
	return path
}

func run(t *testing.T, config Config) Document {
	t.Helper()
	if config.Timeout == 0 {
		config.Timeout = time.Minute
	}
	document, err := Run(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

// LPCV-V0-090: one run records the RSpec JSON report, the command and the
// host repository identity before and after; the run's own report directory
// leaves the worktree clean.
func TestLPCVV0090RunBindsRepositoryBeforeAndAfter(t *testing.T) {
	requireHost(t)
	root := repository(t)
	document := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, cannedReport, 1)}, JUnit: true})
	if err := Validate(document); err != nil {
		t.Fatal(err)
	}
	head := git(t, root, "rev-parse", "HEAD")
	if document.Infrastructure != nil || document.RunnerVersion != "3.13.2" || document.ProjectDirectory != "." || document.ObjectFormat != "sha1" {
		t.Fatalf("document=%+v", document)
	}
	if document.RepositoryBefore == nil || document.RepositoryBefore.Revision != head || document.RepositoryBefore.DirtyState != "clean" || *document.RepositoryAfter != *document.RepositoryBefore {
		t.Fatalf("identity before=%+v after=%+v", document.RepositoryBefore, document.RepositoryAfter)
	}
	if !slices.Contains(document.Command, "json") || document.ExitStatus == nil || *document.ExitStatus != 1 || document.Summary.Failures != 1 {
		t.Fatalf("command=%v exit=%v summary=%+v", document.Command, document.ExitStatus, document.Summary)
	}
	if document.JUnit.State != "PRODUCED" || !strings.HasPrefix(document.JUnit.Digest, "sha256:") {
		t.Fatalf("junit=%+v", document.JUnit)
	}
	if status := git(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("run left the worktree dirty: %q", status)
	}
	// The run created .corvint, so .corvint/.gitignore alone ignores it and
	// rspec/ holds nothing once the reports are removed (LPCV-V0-084).
	if entries, err := os.ReadDir(filepath.Join(root, ".corvint", "rspec")); err != nil || len(entries) != 0 {
		t.Fatalf("reports not removed: %v %v", entries, err)
	}
}

// LPCV-V0-090: failures that prevent a complete observation are named
// infrastructure, never a pass.
func TestLPCVV0090InfrastructureReasons(t *testing.T) {
	requireHost(t)
	root := repository(t)
	for name, test := range map[string]struct {
		report string
		code   int
		reason string
	}{
		"missing report":      {"", 1, "report-missing"},
		"invalid report":      {"{", 0, "report-invalid"},
		"count mismatch":      {strings.Replace(cannedReport, `"example_count":3`, `"example_count":4`, 1), 1, "report-count-mismatch"},
		"errors outside":      {strings.Replace(cannedReport, `"errors_outside_of_examples_count":0`, `"errors_outside_of_examples_count":1`, 1), 1, "rspec-errors-outside-examples"},
		"unexplained exit":    {strings.Replace(strings.Replace(cannedReport, `"status":"failed"`, `"status":"passed"`, 1), `"failure_count":1`, `"failure_count":0`, 1), 2, "runner-exit-unexplained"},
		"start failure":       {cannedReport, -1, "start-failed"},
		"example has no line": {strings.Replace(cannedReport, `"line_number":2`, `"line_number":0`, 1), 1, "report-invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			rspec := []string{filepath.Join(t.TempDir(), "absent-rspec")}
			if test.code >= 0 {
				rspec = []string{fakeRSpec(t, test.report, test.code)}
			}
			document := run(t, Config{Dir: root, RSpec: rspec})
			if document.Infrastructure == nil || document.Infrastructure.Reason != test.reason {
				t.Fatalf("infrastructure=%+v", document.Infrastructure)
			}
			if err := Validate(document); err != nil {
				t.Fatal(err)
			}
			if run := ProjectRun(document); run.Execution.State != testvalidity.ExecutionInfrastructure {
				t.Fatalf("run=%+v", run)
			}
		})
	}
}

// LPCV-V0-091: under an exec prefix (a container whose .git names a host
// path), Git is observed on the host, never through the prefix.
func TestLPCVV0091ExecPrefixObservesGitOnHost(t *testing.T) {
	requireHost(t)
	main := repository(t)
	worktree := filepath.Join(filepath.Dir(main), filepath.Base(main)+"-linked")
	git(t, main, "worktree", "add", "-q", worktree)
	t.Cleanup(func() { _ = os.RemoveAll(worktree) })
	if info, err := os.Lstat(filepath.Join(worktree, ".git")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf(".git is not a host-path file: %v %v", info, err)
	}
	marker := filepath.Join(t.TempDir(), "prefix-used")
	prefix := filepath.Join(t.TempDir(), "container")
	// The fake container refuses Git and breaks GIT_DIR for what it runs.
	write(t, prefix, "#!/bin/sh\nif [ \"$1\" = git ]; then exit 97; fi\necho used > '"+marker+"'\nGIT_DIR=/nonexistent exec \"$@\"\n", 0o755)
	document := run(t, Config{Dir: worktree, ExecPrefix: []string{prefix}, RSpec: []string{fakeRSpec(t, cannedReport, 1)}})
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("rspec did not run through the prefix: %v", err)
	}
	if document.Command[0] != prefix || document.Infrastructure != nil {
		t.Fatalf("document=%+v", document)
	}
	head := git(t, worktree, "rev-parse", "HEAD")
	if document.RepositoryBefore == nil || document.RepositoryBefore.Revision != head || document.RepositoryAfter == nil || *document.RepositoryAfter != *document.RepositoryBefore {
		t.Fatalf("host identity before=%+v after=%+v", document.RepositoryBefore, document.RepositoryAfter)
	}
}

// LPCV-V0-092: each example carries its worktree file:line identity, full
// description, closed status and the Git blob IDs of its spec file and of the
// conventional helper files.
func TestLPCVV0092ExampleIdentityAndBlobs(t *testing.T) {
	requireHost(t)
	root := repository(t)
	project := filepath.Join(root, "app")
	write(t, filepath.Join(project, "spec", "spec_helper.rb"), "RSpec.configure { }\n", 0o644)
	write(t, filepath.Join(project, "spec", "support", "nested", "auth.rb"), "# auth\n", 0o644)
	write(t, filepath.Join(project, "spec", "models", "user_spec.rb"), "RSpec.describe User do\nend\n", 0o644)
	write(t, filepath.Join(project, "spec", "models", "unrelated.txt"), "not ruby\n", 0o644)
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "app")
	document := run(t, Config{Dir: project, RSpec: []string{fakeRSpec(t, cannedReport, 1)}})
	if document.ProjectDirectory != "app" || len(document.Examples) != 3 {
		t.Fatalf("document=%+v", document)
	}
	example := document.Examples[1]
	specBlob := git(t, root, "hash-object", "app/spec/models/user_spec.rb")
	if example.ID != "app/spec/models/user_spec.rb:4" || example.RSpecID != "./spec/models/user_spec.rb[1:2]" || example.Description != "User rejects a blank name" || example.Status != "failed" || example.SpecBlob != specBlob || example.Helpers != document.HelperSet {
		t.Fatalf("example=%+v want blob %s", example, specBlob)
	}
	var paths []string
	for _, helper := range document.Helpers {
		if helper.Blob != git(t, root, "hash-object", helper.Path) {
			t.Fatalf("helper blob %+v", helper)
		}
		paths = append(paths, helper.Path)
	}
	if !slices.Equal(paths, []string{"app/spec/spec_helper.rb", "app/spec/support/nested/auth.rb"}) || document.HelperBasis != HelperBasis {
		t.Fatalf("helpers=%v", paths)
	}

	secret := strings.Replace(cannedReport, "User saves", "User saves ghp_"+strings.Repeat("A", 36), 1)
	long := strings.Replace(cannedReport, "User saves", strings.Repeat("x", MaxDescriptionBytes+1), 1)
	for want, report := range map[string]string{"secret-shaped": secret, "over-bound": long} {
		withheld := run(t, Config{Dir: project, RSpec: []string{fakeRSpec(t, report, 1)}})
		if withheld.Examples[0].Description != "" || withheld.Examples[0].DescriptionWithheld != want {
			t.Fatalf("%s: example=%+v", want, withheld.Examples[0])
		}
	}
}

// LPCV-V0-094: freshness is STALE for a changed or removed bound blob and
// UNKNOWN when the run's identity cannot support CURRENT.
func TestLPCVV0094FreshnessFromBlobs(t *testing.T) {
	requireHost(t)
	root := repository(t)
	document := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, cannedReport, 1)}})
	worktree, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer worktree.Close()
	read := func(name string) ([]byte, error) { return worktree.ReadFile(name) }
	helpers := func(project string) ([]string, error) { return CurrentHelpers(worktree, project) }
	if axis := Freshness(document, read, helpers); axis.State != testvalidity.FreshnessCurrent {
		t.Fatalf("current=%+v", axis)
	}
	write(t, filepath.Join(root, "spec", "support", "factories.rb"), "# changed\n", 0o644)
	if axis := Freshness(document, read, helpers); axis.State != testvalidity.FreshnessStale || axis.Reason != "retained-digest-mismatch" {
		t.Fatalf("stale=%+v", axis)
	}
	unreadable := func(string) ([]byte, error) { return nil, errors.New("denied") }
	if axis := Freshness(document, unreadable, helpers); axis.State != testvalidity.FreshnessUnknown || axis.Reason != "retained-bound-source-unreadable" {
		t.Fatalf("unreadable=%+v", axis)
	}
	removed := func(string) ([]byte, error) { return nil, fs.ErrNotExist }
	if axis := Freshness(document, removed, helpers); axis.State != testvalidity.FreshnessStale {
		t.Fatalf("removed=%+v", axis)
	}
	unbound := document
	unbound.RepositoryAfter = nil
	if axis := Freshness(unbound, func(name string) ([]byte, error) { return worktree.ReadFile(name) }, helpers); axis.State == testvalidity.FreshnessCurrent {
		t.Fatalf("unbound=%+v", axis)
	}
}

func openRoot(t *testing.T, dir string) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root
}

func freshness(t *testing.T, root *os.Root, document Document) testvalidity.Axis {
	t.Helper()
	return Freshness(document, func(name string) ([]byte, error) { return root.ReadFile(name) }, func(project string) ([]string, error) { return CurrentHelpers(root, project) })
}

// LPCV-V0-093: a failure past the retained-example bound still fails the run
// from the summary count, and the omitted examples leave freshness UNKNOWN.
func TestLPCVV0093TruncatedFailureFailsRun(t *testing.T) {
	requireHost(t)
	root := repository(t)
	var examples []string
	for index := 0; index <= MaxExamples; index++ {
		status := "passed"
		if index == MaxExamples {
			status = "failed"
		}
		examples = append(examples, `{"id":"./spec/models/user_spec.rb[1:`+strconv.Itoa(index+1)+`]","full_description":"User","status":"`+status+`","file_path":"./spec/models/user_spec.rb","line_number":2}`)
	}
	report := `{"version":"3.13.2","examples":[` + strings.Join(examples, ",") + `],"summary":{"example_count":` + strconv.Itoa(MaxExamples+1) + `,"failure_count":1,"pending_count":0,"errors_outside_of_examples_count":0}}`
	document := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, report, 1)}})
	if document.Infrastructure != nil || document.ExamplesOmitted != 1 || len(document.Examples) != MaxExamples || anyFailed(document.Examples) {
		t.Fatalf("infrastructure=%+v omitted=%d examples=%d", document.Infrastructure, document.ExamplesOmitted, len(document.Examples))
	}
	if projection := ProjectRun(document); projection.Execution.State != testvalidity.ExecutionFailed {
		t.Fatalf("run=%+v", projection.Execution)
	}
	if axis := freshness(t, openRoot(t, root), document); axis.State != testvalidity.FreshnessUnknown || axis.Reason != "retained-examples-omitted" {
		t.Fatalf("freshness=%+v", axis)
	}
}

// LPCV-V0-092: a helper path that is a symlink leaves the helper set
// explicitly unbound instead of silently dropping it, and the current helper
// listing refuses it.
func TestLPCVV0092SymlinkedHelperIsUnbound(t *testing.T) {
	requireHost(t)
	root := repository(t)
	write(t, filepath.Join(root, "lib", "real.rb"), "# real\n", 0o644)
	if err := os.Symlink(filepath.Join("..", "..", "lib", "real.rb"), filepath.Join(root, "spec", "support", "linked.rb")); err != nil {
		t.Fatal(err)
	}
	document := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, cannedReport, 1)}})
	if document.HelpersUnbound != "helper-not-regular-file" || len(document.Helpers) != 0 {
		t.Fatalf("helpersUnbound=%q helpers=%+v", document.HelpersUnbound, document.Helpers)
	}
	worktree := openRoot(t, root)
	if axis := freshness(t, worktree, document); axis.State != testvalidity.FreshnessUnknown || axis.Reason != "retained-helper-set-unbound" {
		t.Fatalf("freshness=%+v", axis)
	}
	if _, err := CurrentHelpers(worktree, "."); err == nil {
		t.Fatal("CurrentHelpers admitted a symlinked helper")
	}
}

// LPCV-V0-094: a helper added after the run makes retained evidence STALE.
func TestLPCVV0094AddedHelperIsStale(t *testing.T) {
	requireHost(t)
	root := repository(t)
	document := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, cannedReport, 1)}})
	worktree := openRoot(t, root)
	if axis := freshness(t, worktree, document); axis.State != testvalidity.FreshnessCurrent {
		t.Fatalf("current=%+v", axis)
	}
	write(t, filepath.Join(root, "spec", "support", "added.rb"), "# added\n", 0o644)
	if axis := freshness(t, worktree, document); axis.State != testvalidity.FreshnessStale || axis.Reason != "retained-helper-set-changed" {
		t.Fatalf("added=%+v", axis)
	}
	failing := func(string) ([]string, error) { return nil, errors.New("denied") }
	if axis := Freshness(document, func(name string) ([]byte, error) { return worktree.ReadFile(name) }, failing); axis.State != testvalidity.FreshnessUnknown || axis.Reason != "retained-helper-set-unreadable" {
		t.Fatalf("unreadable=%+v", axis)
	}
}

// LPCV-V0-092: a secret-shaped spec or helper path is never retained: the
// example is counted as omitted, the helper set is unbound, and Validate
// refuses a document that carries one.
func TestLPCVV0092SecretShapedPathsWithheld(t *testing.T) {
	requireHost(t)
	root := repository(t)
	secretDirectory := "ghp_" + strings.Repeat("A", 36)
	secretSpec := "spec/" + secretDirectory + "/user_spec.rb"
	if !secretscreen.MatchString(secretSpec) {
		t.Fatalf("fixture path %q is not secret-shaped", secretSpec)
	}
	report := strings.Replace(cannedReport, `"file_path":"./spec/models/user_spec.rb","line_number":4`, `"file_path":"./`+secretSpec+`","line_number":4`, 1)
	document := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, report, 1)}})
	if document.ExamplesOmitted != 1 || len(document.Examples) != 2 || document.Infrastructure != nil {
		t.Fatalf("omitted=%d examples=%+v infrastructure=%+v", document.ExamplesOmitted, document.Examples, document.Infrastructure)
	}
	if encoded, err := json.Marshal(document); err != nil || strings.Contains(string(encoded), secretDirectory) {
		t.Fatalf("secret-shaped path retained: %v", err)
	}
	if projection := ProjectRun(document); projection.Execution.State != testvalidity.ExecutionFailed {
		t.Fatalf("run=%+v", projection.Execution)
	}
	forged := document
	forged.Examples = append([]Example{}, document.Examples...)
	forged.Examples[0].File = secretSpec
	forged.Examples[0].ID = secretSpec + ":2"
	if err := Validate(forged); err == nil {
		t.Fatal("Validate admitted a secret-shaped example file")
	}

	write(t, filepath.Join(root, "spec", "support", secretDirectory+".rb"), "# helper\n", 0o644)
	helpers := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, cannedReport, 1)}})
	if helpers.HelpersUnbound != "helper-path-secret-shaped" || len(helpers.Helpers) != 0 {
		t.Fatalf("helpersUnbound=%q helpers=%+v", helpers.HelpersUnbound, helpers.Helpers)
	}
}

// LPCV-V0-092: the spec walk counts every entry, not only Ruby files, and a
// cancelled context stops it.
func TestLPCVV0092SnapshotWalkIsBounded(t *testing.T) {
	requireHost(t)
	root := repository(t)
	for index := 0; index < 10; index++ {
		write(t, filepath.Join(root, "spec", "fixtures", strconv.Itoa(index)+".json"), "{}\n", 0o644)
	}
	previous := maxSnapshotEntries
	maxSnapshotEntries = 8
	t.Cleanup(func() { maxSnapshotEntries = previous })
	worktree := openRoot(t, root)
	if bounded := takeSnapshot(context.Background(), worktree, "sha1"); bounded.unbound != "spec-snapshot-over-bound" {
		t.Fatalf("bounded=%+v", bounded)
	}
	maxSnapshotEntries = previous
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if stopped := takeSnapshot(cancelled, worktree, "sha1"); stopped.unbound != "spec-snapshot-unavailable" {
		t.Fatalf("cancelled=%+v", stopped)
	}
}

// LPCV-V0-094: a repository identity that is present but not fully observed
// never supports CURRENT.
func TestLPCVV0094EmptyRepositoryIdentityIsUnknown(t *testing.T) {
	requireHost(t)
	root := repository(t)
	document := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, cannedReport, 1)}})
	empty := &jstestprovider.ApplicationRepositoryIdentity{}
	document.RepositoryBefore, document.RepositoryAfter = empty, empty
	if axis := freshness(t, openRoot(t, root), document); axis.State != testvalidity.FreshnessUnknown || axis.Reason != "retained-repository-identity-unavailable" {
		t.Fatalf("freshness=%+v", axis)
	}
}

// LPCV-V0-090: a runner killed by a signal is infrastructure even when it
// wrote a failure report first.
func TestLPCVV0090SignalledRunnerIsInfrastructure(t *testing.T) {
	requireHost(t)
	root := repository(t)
	rspec := fakeRSpec(t, cannedReport, 1)
	script, err := os.ReadFile(rspec)
	if err != nil {
		t.Fatal(err)
	}
	write(t, rspec, strings.Replace(string(script), "exit 1\n", "kill -9 $$\n", 1), 0o755)
	document := run(t, Config{Dir: root, RSpec: []string{rspec}})
	if document.Infrastructure == nil || document.Infrastructure.Reason != "runner-signalled" {
		t.Fatalf("infrastructure=%+v exit=%v", document.Infrastructure, document.ExitStatus)
	}
	if projection := ProjectRun(document); projection.Execution.State != testvalidity.ExecutionInfrastructure {
		t.Fatalf("run=%+v", projection.Execution)
	}
}

// LPCV-V0-095: the provider ignores .corvint/ when it creates it and never
// overwrites an existing .corvint/.gitignore; both go through the shared
// testevidence.IgnoreRetained helper (LPCV-V0-084, V1-1811).
func TestLPCVV0095GitignoreCreatedOnceNeverOverwritten(t *testing.T) {
	requireHost(t)
	root := repository(t)
	write(t, filepath.Join(root, ".corvint", ".gitignore"), "rspec/\n", 0o644)
	run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, cannedReport, 1)}})
	if data, err := os.ReadFile(filepath.Join(root, ".corvint", ".gitignore")); err != nil || string(data) != "rspec/\n" {
		t.Fatalf("existing .gitignore changed: %q %v", data, err)
	}
	fresh := repository(t)
	run(t, Config{Dir: fresh, RSpec: []string{fakeRSpec(t, cannedReport, 1)}})
	if data, err := os.ReadFile(filepath.Join(fresh, ".corvint", ".gitignore")); err != nil || string(data) != "*\n" {
		t.Fatalf("created .gitignore: %q %v", data, err)
	}
}

// LPCV-V0-095: a tracked .corvint/.gitignore that ignores neither rspec/ nor
// test-evidence/ still leaves the worktree clean after a run and retention.
func TestLPCVV0095ExistingGitignoreStillClean(t *testing.T) {
	requireHost(t)
	root := repository(t)
	write(t, filepath.Join(root, ".corvint", ".gitignore"), "cache/\n", 0o644)
	git(t, root, "add", ".corvint/.gitignore")
	git(t, root, "commit", "-q", "-m", "tracked corvint ignore")
	run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, cannedReport, 1)}})
	if _, err := testevidence.Retain(root, Producer, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if status := git(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("worktree dirty: %q", status)
	}
	if data, err := os.ReadFile(filepath.Join(root, ".corvint", ".gitignore")); err != nil || string(data) != "cache/\n" {
		t.Fatalf("existing .gitignore changed: %q %v", data, err)
	}
}
