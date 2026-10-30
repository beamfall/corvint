package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/rbmutate"
	"github.com/Beamfall/corvint/internal/liveverify/rbmutate/minirspec"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// proveFakeContainer stands in for a container CLI: it accepts
// `run --rm -v RUN:/work -w /work/tree fake-image rspec ARGS...`, logs RUN,
// maps /work paths back to RUN and runs the minirspec double in the run's
// tree with the test's PATH, as an image brings its own environment.
const proveFakeContainer = `#!/bin/sh
[ "$1" = run ] || exit 97
shift
host=""
dir=""
while [ $# -gt 0 ]; do
  case "$1" in
    --rm) shift ;;
    -v) host="${2%%:/work}"; shift 2 ;;
    -w) dir="$2"; shift 2 ;;
    fake-image) shift; break ;;
    *) exit 98 ;;
  esac
done
[ "$1" = rspec ] || exit 99
shift
echo "$host" >> "@LOG@"
cd "$host${dir#/work}" || exit 96
for word; do
  shift
  case "$word" in
    /work/*) set -- "$@" "$host${word#/work}" ;;
    *) set -- "$@" "$word" ;;
  esac
done
PATH="@PATH@" exec "@RUBY@" "@SHIM@" "$@"
`

// writeFakeContainerDeclaration installs the fake CLI and writes its
// corvint-rbmutate-container/0 declaration; it returns the declaration's
// path and the run log.
func writeFakeContainerDeclaration(t *testing.T) (string, string) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the test runner executes on darwin and linux only")
	}
	ruby, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby is not available")
	}
	shim, err := minirspec.Install(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bin, workspace, logs := t.TempDir(), t.TempDir(), t.TempDir()
	log := filepath.Join(logs, "runs.log")
	script := strings.NewReplacer("@LOG@", log, "@RUBY@", ruby, "@SHIM@", shim, "@PATH@", os.Getenv("PATH")).Replace(proveFakeContainer)
	executable := filepath.Join(bin, "docker")
	if err := os.WriteFile(executable, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	declaration, err := json.Marshal(rbmutate.ContainerProfile{
		Profile: rbmutate.ContainerProfileName, Executable: executable, ExecutableSha256: tr.Digest([]byte(script)),
		ExecPrefix: []string{"run", "--rm", "-v", rbmutate.RunPlaceholder + ":/work", "-w", "/work/tree", "fake-image", "rspec"},
		Container:  "/work", Workspace: workspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(logs, "runner.json")
	if err := os.WriteFile(path, declaration, 0o644); err != nil {
		t.Fatal(err)
	}
	return path, log
}

// TestFPKV0078MutateRunnerFlagParses pins the flag grammar and its
// refusals (FPK-V0-078).
func TestFPKV0078MutateRunnerFlagParses(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	declaration, _ := writeFakeContainerDeclaration(t)
	for _, form := range [][]string{{"--mutate-runner", declaration}, {"--mutate-runner=" + declaration}} {
		parsed, handled, err := parseProveInvocation(append([]string{"--root", root, "prove", "--mutate"}, append(form, "lib/a.rb")...))
		if !handled || err != nil || parsed.prove.rubyRunner == nil || parsed.prove.rubyRunner.Container != "/work" {
			t.Fatalf("%v: handled %v err %v runner %+v", form, handled, err, parsed.prove.rubyRunner)
		}
	}
	parsed, _, err := parseProveInvocation([]string{"--root", root, "prove", "--mutate", "lib/a.rb"})
	if err != nil || parsed.prove.rubyRunner != nil {
		t.Fatalf("no declaration must keep the host runner: %+v %v", parsed.prove.rubyRunner, err)
	}
	invalid := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"profile":"corvint-rbmutate-container/0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"prove", "--mutate-runner", declaration, "lib/a.rb"},
		{"prove", "--mutate", "--mutate-runner", declaration, "--mutate-runner", declaration, "lib/a.rb"},
		{"prove", "--mutate", "--mutate-runner"},
		{"prove", "--mutate", "--mutate-runner", filepath.Join(t.TempDir(), "absent.json"), "lib/a.rb"},
		{"prove", "--mutate", "--mutate-runner", invalid, "lib/a.rb"},
	} {
		if _, _, err := parseProveInvocation(append([]string{"--root", root}, args...)); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

// TestFPKV0079ProveRunsRubyRowsThroughTheDeclaredContainer: a named-source
// row under `--mutate-runner` runs its baseline and every mutant through the
// declared container, each in its own removed run directory, and every
// receipt in the row's mutation record carries its runner receipt; the
// repository is unchanged.
func TestFPKV0079ProveRunsRubyRowsThroughTheDeclaredContainer(t *testing.T) {
	t.Parallel()
	requireNestableSandbox(t)
	declaration, log := writeFakeContainerDeclaration(t)
	root := proveContainerRepository(t)
	before := treeDigest(t, root)
	// No rspec on PATH: only the declared container can run the rows.
	environment := []string{"PATH=/usr/bin:/bin"}
	receipt, _, stderr, code := runProveCLIEnvironment(t, environment, root, "--mutate", "--mutate-runner", declaration,
		"--mutate-source", "app/models/scoring.rb:3-6", "--mutate-example", "spec/models/scoring_spec.rb:4:7:10")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var row map[string]any
	for _, candidate := range proofRows(t, receipt) {
		if stringAt(candidate, "kind") == kindNamedSource {
			row = candidate
		}
	}
	mutation, _ := row["mutation"].(map[string]any)
	mutants, _ := mutation["mutants"].([]any)
	if mutation["stage"] != "mutants" || mutation["runner"] != "docker run --rm -v {run}:/work -w tree fake-image rspec" || len(mutants) == 0 {
		t.Fatalf("mutation record: %v", row)
	}
	receipts := []any{mutation["baseline"]}
	for _, entry := range mutants {
		mutant, _ := entry.(map[string]any)
		receipts = append(receipts, mutant["receipt"])
	}
	for _, entry := range receipts {
		receiptOf, _ := entry.(map[string]any)
		runner, _ := receiptOf["runner_receipt"].(map[string]any)
		if runner["profile"] != "corvint-test-runner-receipt/0" || len(stringAt(runner, "plan_sha256")) != 64 || len(stringAt(runner, "receipt_sha256")) != 64 ||
			runner["complete"] != true || runner["report_error"] != nil {
			t.Fatalf("receipt without its runner receipt: %v", receiptOf)
		}
	}
	content, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	runs := strings.Fields(string(content))
	if len(runs) != len(receipts) {
		t.Fatalf("want one run per receipt, got %d runs for %d receipts", len(runs), len(receipts))
	}
	for _, run := range runs {
		if _, err := os.Stat(run); !os.IsNotExist(err) {
			t.Fatalf("run directory %s survived: %v", run, err)
		}
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("prove --mutate-runner wrote the repository")
	}
}

// proveContainerRepository commits the weak-spec Ruby fixture to a fresh
// repository and returns its root.
func proveContainerRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{"app/models/scoring.rb": proveScoringBase, "spec/models/scoring_spec.rb": proveScoringWeakSpec} {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"}, {"add", "."}, {"commit", "-qm", "base"},
	} {
		affectedGit(t, root, arguments...)
	}
	return root
}

// TestFPKV0078ProveRefusesAWorkspaceInsideTheRepository: prove passes the
// checkout root to the engine, so a declared workspace under it leaves the
// row NOT_RUN with the fixed detail and no run is started.
func TestFPKV0078ProveRefusesAWorkspaceInsideTheRepository(t *testing.T) {
	t.Parallel()
	requireNestableSandbox(t)
	declaration, log := writeFakeContainerDeclaration(t)
	root := proveContainerRepository(t)
	content, err := os.ReadFile(declaration)
	if err != nil {
		t.Fatal(err)
	}
	var profile rbmutate.ContainerProfile
	if err := json.Unmarshal(content, &profile); err != nil {
		t.Fatal(err)
	}
	// An empty directory leaves the worktree clean.
	profile.Workspace = filepath.Join(root, "tmp")
	if err := os.Mkdir(profile.Workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if content, err = json.Marshal(profile); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(declaration, content, 0o644); err != nil {
		t.Fatal(err)
	}
	receipt, _, stderr, code := runProveCLIEnvironment(t, []string{"PATH=/usr/bin:/bin"}, root, "--mutate", "--mutate-runner", declaration,
		"--mutate-source", "app/models/scoring.rb:3-6", "--mutate-example", "spec/models/scoring_spec.rb:4:7:10")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var row map[string]any
	for _, candidate := range proofRows(t, receipt) {
		if stringAt(candidate, "kind") == kindNamedSource {
			row = candidate
		}
	}
	if row["falsified"] != falsifiedNotRun || stringAt(row, "detail") != "the container workspace is inside or contains the repository" {
		t.Fatalf("row: %v", row)
	}
	if runs, err := os.ReadFile(log); err == nil && len(runs) != 0 {
		t.Fatalf("a run started in a workspace inside the repository: %s", runs)
	}
	if entries, err := os.ReadDir(profile.Workspace); err != nil || len(entries) != 0 {
		t.Fatalf("workspace inside the repository was written: %v %v", entries, err)
	}
}
