package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/worklistadapter"
	"github.com/Beamfall/corvint/internal/workqueue"
	"github.com/Beamfall/corvint/internal/worksource"
)

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
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture"}, strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("init exit=%d stderr=%s", exit, &stderr)
	}
	if got := stdout.String(); got != ".corvint/work-queue-policy.json\n.corvint/worklist.json\n.corvint/work-queue-adapter\n" {
		t.Fatalf("init output: %q", got)
	}
	before := materializationManifest(t, root)
	if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture"}, strings.NewReader(""), &stdout, &stderr); exit != 2 {
		t.Fatalf("second init exit=%d", exit)
	}
	if !reflect.DeepEqual(before, materializationManifest(t, root)) {
		t.Fatal("refused init changed files")
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	shim := "#!/bin/bash\nexec '" + binary + "' -test.run='^TestWorkAdapterProcess$' work adapter \"$@\"\n"
	worklist := `{"profile":"corvint-worklist/0","tickets":[
{"body":"Run the parser suite batch.","id":"suite-parser","title":"Suite batch: parser","touchPaths":["internal/parser"]},
{"body":"Classify and repair the flaky index failure.","id":"repair-index","title":"Failure-classification repair: index","touchPaths":["internal/index"]},
{"body":"Retain the test-validity receipt for the CLI suite.","id":"receipt-cli","title":"Test-validity receipt: cli","touchPaths":["cmd/cli"]},
{"body":"Clean up and retry the parser fixtures.","id":"retry-parser","title":"Cleanup and retry: parser fixtures","touchPaths":["internal/parser"]}]}
`
	writeFixtureFiles(t, root, map[string]string{".corvint/work-queue-adapter": shim, ".corvint/worklist.json": worklist})
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
	if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture"}, strings.NewReader(""), &stdout, &stderr); exit != 2 {
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
		if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture"}, strings.NewReader(""), &stdout, &stderr); exit != 2 {
			t.Fatalf("root %s: init exit=%d stdout=%s stderr=%s", root, exit, &stdout, &stderr)
		}
		if _, err := os.Lstat(filepath.Join(root, ".corvint")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("root %s: initialization wrote outside a repository root: %v", root, err)
		}
	}
}

func TestWorkInitUsesPortableAdapterShell(t *testing.T) {
	t.Parallel()
	if !strings.HasPrefix(workAdapterScript, "#!/bin/sh\n") {
		t.Fatalf("generated adapter has non-portable shebang: %q", strings.SplitN(workAdapterScript, "\n", 2)[0])
	}
}

func TestWorkMissingAdoptionWorklistIsSourceUnqualified(t *testing.T) {
	t.Parallel()
	root := materializationFixture(t)
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture"}, strings.NewReader(""), &stdout, &stderr); exit != 0 {
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

// WQO-V0-046: store scope is complete only for exact repository-worklist-v0 output.
func TestWorkMappingReproduced(t *testing.T) {
	t.Parallel()
	root := materializationFixture(t)
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"--root", root, "work", "init", "--repository", "fixture"}, strings.NewReader(""), &stdout, &stderr); exit != 0 {
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
	if exit := run([]string{"--root", t.TempDir(), "work", "init", "--repository", "bad name"}, strings.NewReader(""), &stdout, &stderr); exit != 2 {
		t.Fatal("invalid repository token accepted")
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
