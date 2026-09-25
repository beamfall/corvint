package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/worklistadapter"
	"github.com/Beamfall/corvint/internal/workqueue"
	"github.com/Beamfall/corvint/internal/worksource"
)

var workBoundBuild struct {
	once sync.Once
	root string
	path string
	err  error
}

func workBoundCorvint(t *testing.T) string {
	t.Helper()
	workBoundBuild.once.Do(func() {
		workBoundBuild.root, workBoundBuild.err = os.MkdirTemp("", "corvint-work-bound-")
		if workBoundBuild.err != nil {
			return
		}
		workBoundBuild.root, workBoundBuild.err = filepath.EvalSymlinks(workBoundBuild.root)
		if workBoundBuild.err != nil {
			return
		}
		workBoundBuild.path = filepath.Join(workBoundBuild.root, "home", ".local", "bin", "corvint")
		workBoundBuild.err = workBuildCorvint(workBoundBuild.path, "fixture-1")
	})
	if workBoundBuild.err != nil {
		t.Fatal(workBoundBuild.err)
	}
	return workBoundBuild.path
}

func workBuildCorvint(path, build string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	command := exec.Command("go", "build", "-trimpath", "-ldflags", "-X main.build="+build, "-o", path, ".")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOCACHE="+filepath.Join(os.TempDir(), "corvint-go-build-cache"))
	if output, err := command.CombinedOutput(); err != nil {
		return errors.New(err.Error() + ": " + string(output))
	}
	return nil
}

func workBuildCorvintWithoutVCS(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", path, ".")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOCACHE="+filepath.Join(os.TempDir(), "corvint-go-build-cache"))
	if output, err := command.CombinedOutput(); err != nil {
		return errors.New(err.Error() + ": " + string(output))
	}
	return nil
}

func workCopyExecutable(t *testing.T, source, destination string) {
	t.Helper()
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, raw, 0755); err != nil {
		t.Fatal(err)
	}
}

// TestWorkAdapterProcess is the adapter child the adoption fixture's committed
// script re-executes; the sanitized observer environment carries only argv.
func TestWorkAdapterProcess(t *testing.T) {
	arguments := flag.Args()
	if len(arguments) == 0 || arguments[0] != "work" {
		t.Skip("adapter child only")
	}
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	os.Exit(run(append([]string{"--root", directory}, arguments...), strings.NewReader(""), os.Stdout, os.Stderr))
}

// WQO-V0-047/048: an adopted repository worklist observes and proposes a wave
// with no repository mutation; verification tasks are tickets with touchPaths.
func TestWorkAdoptedRepositoryWorklist(t *testing.T) {
	t.Parallel()
	workCaptureSlot(t)
	root := materializationFixture(t)
	binary := workBoundCorvint(t)
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture", "--corvint-executable", binary}, strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("init exit=%d stderr=%s", exit, &stderr)
	}
	if got := stdout.String(); got != ".corvint/work-queue-policy.json\n.corvint/worklist.json\n.corvint/work-queue-adapter\n" {
		t.Fatalf("init output: %q", got)
	}
	before := materializationManifest(t, root)
	if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture", "--corvint-executable", binary}, strings.NewReader(""), &stdout, &stderr); exit != 2 {
		t.Fatalf("second init exit=%d", exit)
	}
	if !reflect.DeepEqual(before, materializationManifest(t, root)) {
		t.Fatal("refused init changed files")
	}
	worklist := `{"profile":"corvint-worklist/0","tickets":[
{"body":"Run the parser suite batch.","id":"suite-parser","title":"Suite batch: parser","touchPaths":["internal/parser"]},
{"body":"Classify and repair the flaky index failure.","id":"repair-index","title":"Failure-classification repair: index","touchPaths":["internal/index"]},
{"body":"Retain the test-validity receipt for the CLI suite.","id":"receipt-cli","title":"Test-validity receipt: cli","touchPaths":["cmd/cli"]},
{"body":"Clean up and retry the parser fixtures.","id":"retry-parser","title":"Cleanup and retry: parser fixtures","touchPaths":["internal/parser"]}]}
`
	writeFixtureFiles(t, root, map[string]string{".corvint/worklist.json": worklist})
	materializationGit(t, root, "add", ".")
	materializationGit(t, root, "commit", "-qm", "adopt work queue")
	committed := materializationManifest(t, root)

	observation := workAdoptedRun(t, root, "work", "observe").Observation
	if observation == nil || observation.State != workqueue.StateValidated || observation.MutationState != "UNCHANGED_OBSERVED" {
		t.Fatalf("observation: %+v", observation)
	}
	unknowns := strings.Join(observation.Unknowns, " ")
	if strings.Contains(unknowns, workqueue.UnknownSourceUnqualified) || !strings.Contains(unknowns, workqueue.UnknownExecutableIdentityUnqualified) {
		t.Fatalf("adopted observation unknowns: %v", observation.Unknowns)
	}
	envelope := &workqueue.CapacityEnvelope{
		Available:    []workqueue.CapacityClass{{AvailableUnits: 4, ID: "capacity:fixture:worklist:agent"}},
		Capabilities: []string{"capability:fixture:worklist:agent"}, Profile: workqueue.EnvelopeProfile, RepositoryAuthorityID: "repo:fixture",
	}
	workqueue.RefreshEnvelope(envelope)
	envelopePath := filepath.Join(t.TempDir(), "capacity.json")
	if err := os.WriteFile(envelopePath, envelope.Canonical(), 0600); err != nil {
		t.Fatal(err)
	}
	proposal := workAdoptedRun(t, root, "work", "propose-wave", "--envelope", envelopePath, "--limit", "4").Proposal
	if proposal == nil || proposal.MutationAuthority || proposal.State != "ELIGIBLE_AT" {
		t.Fatalf("proposal: %+v", proposal)
	}
	selected := []string{}
	for _, entry := range proposal.Entries {
		if entry.State == "SELECTED" {
			selected = append(selected, entry.TicketID)
		}
	}
	want := []string{"ticket:fixture:worklist:suite-parser", "ticket:fixture:worklist:repair-index", "ticket:fixture:worklist:receipt-cli"}
	if !reflect.DeepEqual(selected, want) {
		t.Fatalf("selected %v, want %v; proposal %+v", selected, want, proposal)
	}
	excluded := proposal.Entries[len(proposal.Entries)-1]
	if excluded.TicketID != "ticket:fixture:worklist:retry-parser" || excluded.State != "EXCLUDED" || len(excluded.CollisionGroupIDs) == 0 {
		t.Fatalf("conflicting ticket exclusion not exposed: %+v", proposal.Entries)
	}
	if !reflect.DeepEqual(committed, materializationManifest(t, root)) {
		t.Fatal("observe or propose-wave mutated the repository")
	}
}

func TestWorkInitRejectsSymlinkedDirectory(t *testing.T) {
	t.Parallel()
	root := materializationFixture(t)
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(root, ".corvint")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture", "--corvint-executable", workBoundCorvint(t)}, strings.NewReader(""), &stdout, &stderr); exit != 2 {
		t.Fatalf("init exit=%d stdout=%s stderr=%s", exit, &stdout, &stderr)
	}
	entries, err := os.ReadDir(external)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 || stdout.Len() != 0 {
		t.Fatalf("symlink escape wrote outside repository: entries=%v stdout=%q", entries, &stdout)
	}
}

func TestWorkInitRequiresRepositoryRoot(t *testing.T) {
	t.Parallel()
	repository := materializationFixture(t)
	subdirectory := filepath.Join(repository, "subdirectory")
	if err := os.Mkdir(subdirectory, 0755); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{t.TempDir(), subdirectory} {
		var stdout, stderr bytes.Buffer
		if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture", "--corvint-executable", workBoundCorvint(t)}, strings.NewReader(""), &stdout, &stderr); exit != 2 {
			t.Fatalf("root %s: init exit=%d stdout=%s stderr=%s", root, exit, &stdout, &stderr)
		}
		if _, err := os.Lstat(filepath.Join(root, ".corvint")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("root %s: initialization wrote outside a repository root: %v", root, err)
		}
	}
}

func TestWorkInitUsesPortableAdapterShell(t *testing.T) {
	t.Parallel()
	script := string(workBoundAdapterScript(workCorvintExecutableBinding{}))
	if !strings.HasPrefix(script, "#!/bin/sh\n") {
		t.Fatalf("generated adapter has non-portable shebang: %q", strings.SplitN(script, "\n", 2)[0])
	}
}

// WQO-V0-049: every documented install location produces one reviewed,
// descriptor-executed binding with no ambient PATH lookup.
func TestWorkInitBindsExplicitExecutableWQOV0049(t *testing.T) {
	t.Run("WQO-V0-049 explicit executable binding", testWorkInitBindsExplicitExecutable)
}

func testWorkInitBindsExplicitExecutable(t *testing.T) {
	binary := workBoundCorvint(t)
	fixtureRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		path string
	}{
		{"home-local-bin", filepath.Join(fixtureRoot, "home", ".local", "bin", "corvint")},
		{"opt-homebrew-bin", filepath.Join(fixtureRoot, "opt", "homebrew", "bin", "corvint")},
		{"usr-local-bin", filepath.Join(fixtureRoot, "usr", "local", "bin", "corvint")},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			workCopyExecutable(t, binary, test.path)
			root := materializationFixture(t)
			var stdout, stderr bytes.Buffer
			exit := run([]string{"--root", root, "work", "init", "--repository", "fixture", "--corvint-executable", test.path}, strings.NewReader(""), &stdout, &stderr)
			if exit != 0 {
				t.Fatalf("init exit=%d stderr=%s", exit, &stderr)
			}
			raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(workAdapterPath)))
			if err != nil {
				t.Fatal(err)
			}
			binding, err := workParseBoundAdapter(raw)
			if err != nil {
				t.Fatal(err)
			}
			if binding.Path != test.path || binding.SHA256 == "" || binding.Version == "" || binding.Source.Package != workCorvintPackage {
				t.Fatalf("binding: %+v", binding)
			}
			if bytes.Contains(raw, []byte("exec corvint")) || !bytes.Contains(raw, []byte("exec \"$corvint_executable\" work adapter")) {
				t.Fatalf("adapter searches PATH or omits descriptor execution: %s", raw)
			}
		})
	}
	t.Run("companion-buildvcs-false", func(t *testing.T) {
		path := filepath.Join(fixtureRoot, "bundle", "bin", "corvint")
		if err := workBuildCorvintWithoutVCS(path); err != nil {
			t.Fatal(err)
		}
		root := materializationFixture(t)
		var stdout, stderr bytes.Buffer
		if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture", "--corvint-executable", path}, strings.NewReader(""), &stdout, &stderr); exit != 0 {
			t.Fatalf("init exit=%d stderr=%s", exit, &stderr)
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(workAdapterPath)))
		if err != nil {
			t.Fatal(err)
		}
		binding, err := workParseBoundAdapter(raw)
		if err != nil {
			t.Fatal(err)
		}
		if binding.Source.ModuleVersion != "(devel)" || binding.Source.VCS != "" || binding.Source.Revision != "" || binding.Source.Modified {
			t.Fatalf("buildvcs=false source identity: %+v", binding.Source)
		}
	})
}

// WQO-V0-049: relative, missing, unsafe-parent and repository-owned executables,
// reached directly or through a symlink, are refused before init writes anything.
func TestWorkInitRejectsUnqualifiedExecutableWQOV0049(t *testing.T) {
	binary := workBoundCorvint(t)
	fixtureRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	unsafe := filepath.Join(fixtureRoot, "unsafe", "corvint")
	workCopyExecutable(t, binary, unsafe)
	if err := os.Chmod(filepath.Dir(unsafe), 0777); err != nil {
		t.Fatal(err)
	}
	link := func(target, path string) string {
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		return path
	}
	repositoryLocal := func(root string) string {
		path := filepath.Join(root, "bin", "corvint")
		workCopyExecutable(t, binary, path)
		return path
	}
	for _, test := range []struct {
		name, reason string
		path         func(string) string
	}{
		{"relative", "path must be canonical and absolute", func(string) string { return "corvint" }},
		{"missing", "path cannot be resolved", func(string) string { return filepath.Join(fixtureRoot, "missing") }},
		{"symlink-to-unsafe-parent", "path has an unsafe parent component", func(string) string {
			return link(unsafe, filepath.Join(fixtureRoot, "linked-unsafe"))
		}},
		{"symlink-to-missing", "path cannot be resolved", func(string) string {
			return link(filepath.Join(fixtureRoot, "missing"), filepath.Join(fixtureRoot, "linked-missing"))
		}},
		{"symlink-to-repository-local", "path is repository-controlled", func(root string) string {
			return link(repositoryLocal(root), filepath.Join(t.TempDir(), "corvint"))
		}},
		{"repository-link-to-external", "path is repository-controlled", func(root string) string {
			if err := os.MkdirAll(filepath.Join(root, "tools"), 0o755); err != nil {
				t.Fatal(err)
			}
			return link(binary, filepath.Join(root, "tools", "corvint"))
		}},
		{"unsafe-parent", "path has an unsafe parent component", func(string) string { return unsafe }},
		{"repository-local", "path is repository-controlled", repositoryLocal},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := materializationFixture(t)
			var stdout, stderr bytes.Buffer
			exit := run([]string{"--root", root, "work", "init", "--repository", "fixture", "--corvint-executable", test.path(root)}, strings.NewReader(""), &stdout, &stderr)
			if exit != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "executable is unqualified: "+test.reason) {
				t.Fatalf("exit=%d stdout=%q stderr=%q, want reason %q", exit, &stdout, &stderr, test.reason)
			}
			if _, err := os.Lstat(filepath.Join(root, ".corvint")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("refused init wrote .corvint: %v", err)
			}
		})
	}
}

// WQO-V0-049: a symlinked --corvint-executable (an installer link in ~/.local/bin)
// binds its resolved target, says so, and observation then qualifies.
func TestWorkInitBindsResolvedSymlinkTargetWQOV0049(t *testing.T) {
	binary := workBoundCorvint(t)
	linkDirectory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(linkDirectory, "corvint")
	if err := os.Symlink(binary, linked); err != nil {
		t.Fatal(err)
	}
	root := materializationFixture(t)
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture", "--corvint-executable", linked}, strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("init exit=%d stderr=%s", exit, &stderr)
	}
	if want := "corvint work init: " + linked + " resolves through a symlink; bound its target " + binary; !strings.HasPrefix(stderr.String(), want) {
		t.Fatalf("init stderr %q lacks %q", &stderr, want)
	}
	adapter, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(workAdapterPath)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(adapter, []byte(strconv.Quote(binary))) || bytes.Contains(adapter, []byte(linked)) {
		t.Fatalf("adapter does not bind only the resolved target:\n%s", adapter)
	}
	materializationGit(t, root, "add", ".corvint")
	materializationGit(t, root, "commit", "-qm", "adopt work queue")
	stdout.Reset()
	stderr.Reset()
	if exit := run([]string{"--root", root, "work", "observe"}, strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("observe exit=%d stderr=%q", exit, &stderr)
	}
	upgraded := filepath.Join(linkDirectory, "upgraded", "corvint")
	workCopyExecutable(t, binary, upgraded)
	if err := os.Remove(linked); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(upgraded, linked); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if exit := run([]string{"--root", root, "work", "observe"}, strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("observe after retargeting the link exit=%d stderr=%q", exit, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	if exit := run([]string{"--root", root, "work", "rebind", "--corvint-executable", linked}, strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("rebind exit=%d stderr=%q", exit, &stderr)
	}
	if want := "corvint work rebind: " + linked + " resolves through a symlink; bound its target " + upgraded; !strings.HasPrefix(stderr.String(), want) {
		t.Fatalf("rebind stderr %q lacks %q", &stderr, want)
	}
}

// WQO-V0-049/050: byte replacement is stale/unqualified until the operator
// explicitly regenerates the reviewed adapter binding.
func TestWorkExecutableChangeRequiresReviewedRebindWQOV0050(t *testing.T) {
	t.Run("WQO-V0-050 reviewed executable rebind", testWorkExecutableChangeRequiresReviewedRebind)
}

func testWorkExecutableChangeRequiresReviewedRebind(t *testing.T) {
	fixtureRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bound := filepath.Join(fixtureRoot, "home", ".local", "bin", "corvint")
	workCopyExecutable(t, workBoundCorvint(t), bound)
	root := workInitializedRepository(t, bound)
	replacement := filepath.Join(fixtureRoot, "replacement")
	if err := workBuildCorvint(replacement, "fixture-2"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, bound); err != nil {
		t.Fatal(err)
	}
	workAssertFinalError(t, workCommandBytes(t, root, "work", "observe"), 2, "SOURCE_UNQUALIFIED")
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"--root", root, "work", "rebind", "--corvint-executable", bound}, strings.NewReader(""), &stdout, &stderr); exit != 0 || stdout.String() != workAdapterPath+"\n" {
		t.Fatalf("rebind exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
	}
	materializationGit(t, root, "add", workAdapterPath)
	materializationGit(t, root, "commit", "-qm", "review executable rebind")
	result := workAdoptedRun(t, root, "work", "observe")
	if result.Observation == nil || result.Observation.State != workqueue.StateValidated {
		t.Fatalf("rebound observation: %+v", result.Observation)
	}
	for _, receipt := range result.Observation.AdapterReceipts {
		if receipt.ExecutableQualification != "UNQUALIFIED" {
			t.Fatalf("unexpected executable qualification: %+v", receipt)
		}
	}
}

// WQO-V0-049: path removal and symlink replacement both fail before adapter
// execution, while the retained descriptor prevents byte substitution races.
func TestWorkExecutableMissingAndSymlinkSwapWQOV0049(t *testing.T) {
	for _, test := range []struct {
		name string
		swap func(*testing.T, string)
	}{
		{"missing", func(t *testing.T, path string) {
			t.Helper()
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink-swap", func(t *testing.T, path string) {
			t.Helper()
			backup := path + ".replacement"
			workCopyExecutable(t, workBoundCorvint(t), backup)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(backup, path); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixtureRoot, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			bound := filepath.Join(fixtureRoot, "usr", "local", "bin", "corvint")
			workCopyExecutable(t, workBoundCorvint(t), bound)
			root := workInitializedRepository(t, bound)
			test.swap(t, bound)
			workAssertFinalError(t, workCommandBytes(t, root, "work", "observe"), 2, "SOURCE_UNQUALIFIED")
		})
	}
}

func workInitializedRepository(t *testing.T, executable string) string {
	t.Helper()
	root := materializationFixture(t)
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture", "--corvint-executable", executable}, strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("init exit=%d stderr=%s", exit, &stderr)
	}
	materializationGit(t, root, "add", ".")
	materializationGit(t, root, "commit", "-qm", "adopt work queue")
	return root
}

func workCommandBytes(t *testing.T, root string, arguments ...string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exit := run(append([]string{"--root", root}, arguments...), strings.NewReader(""), &stdout, &stderr)
	if exit != 2 {
		t.Fatalf("%v exit=%d stdout=%s stderr=%s", arguments, exit, &stdout, &stderr)
	}
	return stdout.Bytes()
}

func TestWorkMissingAdoptionWorklistIsSourceUnqualified(t *testing.T) {
	t.Parallel()
	root := materializationFixture(t)
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture", "--corvint-executable", workBoundCorvint(t)}, strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("init exit=%d stderr=%s", exit, &stderr)
	}
	worklistPath, _ := worklistadapter.WorklistPath(worklistadapter.RepositoryMapping)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(worklistPath))); err != nil {
		t.Fatal(err)
	}
	materializationGit(t, root, "add", ".corvint")
	materializationGit(t, root, "commit", "-qm", "partial adoption")
	stdout.Reset()
	stderr.Reset()
	exit := run([]string{"--root", root, "work", "observe"}, strings.NewReader(""), &stdout, &stderr)
	workAssertFinalError(t, stdout.Bytes(), exit, "SOURCE_UNQUALIFIED")
}

// WQO-V0-051: every SOURCE_UNQUALIFIED refusal names its reason on one stderr
// line while stdout keeps the closed canonical result; init says to commit.
func TestWorkSourceUnqualifiedNamesReasonWQOV0051(t *testing.T) {
	t.Run("WQO-V0-051 source refusal reason", testWorkSourceUnqualifiedNamesReason)
}

func testWorkSourceUnqualifiedNamesReason(t *testing.T) {
	t.Parallel()
	workCaptureSlot(t)
	root := materializationFixture(t)
	observe := func(want string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		exit := run([]string{"--root", root, "work", "observe"}, strings.NewReader(""), &stdout, &stderr)
		workAssertFinalError(t, stdout.Bytes(), exit, "SOURCE_UNQUALIFIED")
		line := stderr.String()
		if strings.Count(line, "\n") != 1 || !strings.HasPrefix(line, "corvint work: SOURCE_UNQUALIFIED: ") || !strings.Contains(line, want) || strings.ContainsRune(line, '\u009b') {
			t.Fatalf("stderr %q does not name %q on one plain line", line, want)
		}
	}
	observe(".corvint/work-queue-policy.json is not committed at HEAD")
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture", "--corvint-executable", workBoundCorvint(t)}, strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("init exit=%d stderr=%s", exit, &stderr)
	}
	if got := stdout.String(); got != ".corvint/work-queue-policy.json\n.corvint/worklist.json\n.corvint/work-queue-adapter\n" {
		t.Fatalf("init stdout changed: %q", got)
	}
	if got := stderr.String(); got != "corvint work init: review and commit these three files; work observe and propose-wave return SOURCE_UNQUALIFIED until they are committed\n" {
		t.Fatalf("init stderr: %q", got)
	}
	observe("the worktree is dirty")
	materializationGit(t, root, "add", ".corvint")
	materializationGit(t, root, "commit", "-qm", "adopt work queue")
	stdout.Reset()
	stderr.Reset()
	if exit := run([]string{"--root", root, "work", "observe"}, strings.NewReader(""), &stdout, &stderr); exit != 0 || stderr.Len() != 0 {
		t.Fatalf("committed observe exit=%d stderr=%q", exit, &stderr)
	}
	materializationGit(t, root, "config", "include.path", "unused")
	observe("the repository is unsupported: repository config uses an include directive")
	materializationGit(t, root, "config", "--unset", "include.path")
	worktreeConfig := filepath.Join(root, ".git", "config.worktree")
	for driver, want := range map[string]string{"lfs": "filter.lfs.clean", "leak-\u009b31m": "filter.*.clean"} {
		if err := os.WriteFile(worktreeConfig, []byte("[filter \""+driver+"\"]\n\tclean = x\n"), 0600); err != nil {
			t.Fatal(err)
		}
		observe("Git status refused the repository: repository config.worktree sets " + want)
	}
}

func TestWorkRebindUnqualifiedAdoptionOmitsGitStderr(t *testing.T) {
	t.Parallel()
	root := materializationFixture(t)
	payload := "\x1b[31m\u009b" + strings.Repeat("A", 20000)
	config, err := os.OpenFile(filepath.Join(root, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = config.WriteString("[core]\n\tbare = " + payload + "\n")
	if closeErr := config.Close(); err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	var stdout, stderr bytes.Buffer
	exit := run([]string{"--root", root, "work", "rebind", "--corvint-executable", workBoundCorvint(t)}, strings.NewReader(""), &stdout, &stderr)
	line := stderr.String()
	if exit != 2 || stdout.Len() != 0 || !strings.HasPrefix(line, "corvint work rebind: existing adoption is unqualified: ") || strings.Count(line, "\n") != 1 {
		t.Fatalf("rebind exit=%d stdout=%q stderr=%q", exit, &stdout, line)
	}
	if strings.ContainsAny(line, "\x1b\u009b") || strings.Contains(line, "AAAA") {
		t.Fatalf("rebind refusal echoed Git stderr: %q", line)
	}
}

func TestWorkInitRollsBackCreatedFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "second"), []byte("racing writer\n"), 0600); err != nil {
		t.Fatal(err)
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	files := []workAdoptionFile{
		{path: ".corvint/first", raw: []byte("created\n"), mode: 0644},
		{path: ".corvint/second", raw: []byte("must not replace\n"), mode: 0644},
	}
	created, err := writeWorkAdoptionFiles(directory, files)
	if err == nil {
		t.Fatal("racing target did not refuse initialization")
	}
	if len(created) != 0 {
		t.Fatalf("failed initialization reported created files: %v", created)
	}
	if _, err := os.Lstat(filepath.Join(root, "first")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("created file survived rollback: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "second"))
	if err != nil || string(raw) != "racing writer\n" {
		t.Fatalf("racing file changed: %q, %v", raw, err)
	}
}

type workAdoptedResult struct {
	Observation *workqueue.Observation
	Proposal    *struct {
		Entries []struct {
			CollisionGroupIDs []string
			State, TicketID   string
		}
		MutationAuthority bool
		State             string
	}
}

func workAdoptedRun(t *testing.T, root string, arguments ...string) workAdoptedResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exit := run(append([]string{"--root", root}, arguments...), strings.NewReader(""), &stdout, &stderr)
	var result workAdoptedResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || exit != 0 {
		t.Fatalf("%v exit=%d err=%v stdout=%s stderr=%s", arguments, exit, err, &stdout, &stderr)
	}
	return result
}

// WQO-V0-046: store scope is complete only for exact repository-worklist-v0 output
// (decision-0046-v0's self-dogfood counterpart is covered by TestWorkMappingReproducedSelfDogfood).
func TestWorkMappingReproduced(t *testing.T) {
	t.Parallel()
	root := materializationFixture(t)
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture", "--corvint-executable", workBoundCorvint(t)}, strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("init: %s", &stderr)
	}
	materializationGit(t, root, "add", ".")
	materializationGit(t, root, "commit", "-qm", "adopt")
	source, err := worksource.Acquire(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	policy, err := workqueue.ParsePolicy(workSourceTestFile(t, source, worklistadapter.PolicyPath))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, details, checkpoint, err := worklistadapter.DocumentsFromSource(source, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !workMappingReproduced(source, policy, snapshot.Canonical(), details.Canonical(), checkpoint.Canonical()) {
		t.Fatal("exact mapping output not reproduced")
	}
	tampered := append(append([]byte(nil), checkpoint.Canonical()...), ' ')
	if workMappingReproduced(source, policy, snapshot.Canonical(), details.Canonical(), tampered) {
		t.Fatal("differing adapter output qualified store scope")
	}
	unmapped := *policy
	unmapped.MappingVersion = "owner-defined-v0"
	if workMappingReproduced(source, &unmapped, snapshot.Canonical(), details.Canonical(), checkpoint.Canonical()) {
		t.Fatal("unknown mapping qualified store scope")
	}
	if exit := run([]string{"--root", t.TempDir(), "work", "init", "--repository", "bad name", "--corvint-executable", workBoundCorvint(t)}, strings.NewReader(""), &stdout, &stderr); exit != 2 {
		t.Fatal("invalid repository token accepted")
	}
}

// WQO-V0-046: decision-0046-v0 (Corvint's own self-dogfood mapping, docs/worklist.json)
// qualifies store scope by the same exact byte-reproduction argument as repository-worklist-v0.
func TestWorkMappingReproducedSelfDogfood(t *testing.T) {
	t.Parallel()
	selfRoot := workProductionFixture(t)
	selfSource, err := worksource.Acquire(context.Background(), selfRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer selfSource.Close()
	selfPolicy, err := workqueue.ParsePolicy(workSourceTestFile(t, selfSource, worklistadapter.PolicyPath))
	if err != nil {
		t.Fatal(err)
	}
	if selfPolicy.MappingVersion != workSelfDogfoodMapping {
		t.Fatalf("fixture policy mapping = %q, want %q", selfPolicy.MappingVersion, workSelfDogfoodMapping)
	}
	selfSnapshot, selfDetails, selfCheckpoint, err := worklistadapter.DocumentsFromSource(selfSource, selfPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if !workMappingReproduced(selfSource, selfPolicy, selfSnapshot.Canonical(), selfDetails.Canonical(), selfCheckpoint.Canonical()) {
		t.Fatal("decision-0046-v0 exact mapping output not reproduced")
	}
	tamperedSelf := append(append([]byte(nil), selfCheckpoint.Canonical()...), ' ')
	if workMappingReproduced(selfSource, selfPolicy, selfSnapshot.Canonical(), selfDetails.Canonical(), tamperedSelf) {
		t.Fatal("differing decision-0046-v0 adapter output qualified store scope")
	}
}

func workSourceTestFile(t *testing.T, source *worksource.Source, path string) []byte {
	t.Helper()
	raw, err := worklistadapter.SourceFile(source, path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
