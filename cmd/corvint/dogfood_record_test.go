package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

// LTPM-V0-011.
func TestDogfoodRecordMixedChangeRecordsExactSourceSubset(t *testing.T) {
	t.Parallel()
	root := newRecordFixtureAt(t, filepath.Join(t.TempDir(), "repository"))
	base := recordRevision(t, root)
	writeDogfoodChange(t, root, "internal/example/value.go", "package example\n\nconst Changed = true\n")
	writeDogfoodChange(t, root, ".gitignore", ".context-corvint/\nlocal-only\n")
	commitDogfoodChange(t, root, "mixed")
	target := recordRevision(t, root)

	result := execute(t, candidateCommand(dogfoodRecordArguments(root, base, target)...))
	if result.exit != 0 || len(result.stderr) != 0 {
		t.Fatalf("result=%#v", result)
	}
	var payload struct {
		State      string   `json:"state"`
		Candidates []string `json:"candidates"`
		Admitted   []string `json:"admitted"`
		Trace      struct {
			Changed []string `json:"changed_paths"`
		} `json:"trace"`
	}
	if err := json.Unmarshal(result.stdout, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.State != "recorded" || !reflect.DeepEqual(payload.Candidates, []string{".gitignore", "internal/example/value.go"}) ||
		!reflect.DeepEqual(payload.Admitted, []string{".gitignore", "internal/example/value.go"}) || !reflect.DeepEqual(payload.Trace.Changed, payload.Admitted) {
		t.Fatalf("payload=%+v", payload)
	}
	traceFiles := 0
	for path := range traceTreeSnapshot(t, root) {
		if filepath.Ext(path) == ".jsonl" {
			traceFiles++
		}
	}
	if traceFiles != 1 {
		t.Fatalf("trace file count=%d", traceFiles)
	}
}

// LTPM-V0-011.
func TestDogfoodRecordReceiptDisclosesTruncatedAncestry(t *testing.T) {
	t.Parallel()
	payload := dogfoodRecordPayload(tracerecordrepo.DogfoodResult{
		State:    "recorded",
		Recorded: &tracerecordrepo.Result{Store: "store", TruncatedAncestry: 3},
	})
	if payload["truncated_ancestry"] != 3 {
		t.Fatalf("receipt truncated_ancestry=%v, want 3", payload["truncated_ancestry"])
	}
}

// GPK-V0-050.
func TestDogfoodRecordGitignoreOnlyRecordsTrace(t *testing.T) {
	t.Parallel()
	root := newRecordFixtureAt(t, filepath.Join(t.TempDir(), "repository"))
	base := recordRevision(t, root)
	writeDogfoodChange(t, root, ".gitignore", ".context-corvint/\nlocal-only\n")
	commitDogfoodChange(t, root, "non-source")
	target := recordRevision(t, root)

	result := execute(t, candidateCommand(dogfoodRecordArguments(root, base, target)...))
	if result.exit != 0 || len(result.stderr) != 0 {
		t.Fatalf("result=%#v", result)
	}
	var payload struct {
		State      string   `json:"state"`
		Candidates []string `json:"candidates"`
		Admitted   []string `json:"admitted"`
		Trace      struct {
			Changed []string `json:"changed_paths"`
		} `json:"trace"`
	}
	if err := json.Unmarshal(result.stdout, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.State != "recorded" || !reflect.DeepEqual(payload.Candidates, []string{".gitignore"}) ||
		!reflect.DeepEqual(payload.Admitted, []string{".gitignore"}) || !reflect.DeepEqual(payload.Trace.Changed, payload.Admitted) {
		t.Fatalf("payload=%+v", payload)
	}
	traceFiles := 0
	for path := range traceTreeSnapshot(t, root) {
		if filepath.Ext(path) == ".jsonl" {
			traceFiles++
		}
	}
	if traceFiles != 1 {
		t.Fatalf("trace file count=%d", traceFiles)
	}
}

// GPK-V0-050.
func TestDogfoodRecordUnrelatedNonSourceWritesNoTrace(t *testing.T) {
	t.Parallel()
	root := newRecordFixtureAt(t, filepath.Join(t.TempDir(), "repository"))
	base := recordRevision(t, root)
	writeDogfoodChange(t, root, "README.csv", "non-source\n")
	commitDogfoodChange(t, root, "non-source")
	target := recordRevision(t, root)
	before := traceTreeSnapshot(t, root)

	result := execute(t, candidateCommand(dogfoodRecordArguments(root, base, target)...))
	if result.exit != 0 || len(result.stderr) != 0 {
		t.Fatalf("result=%#v", result)
	}
	var payload struct {
		State      string   `json:"state"`
		Candidates []string `json:"candidates"`
		Admitted   []string `json:"admitted"`
	}
	if err := json.Unmarshal(result.stdout, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.State != "no-source-paths" || !reflect.DeepEqual(payload.Candidates, []string{"README.csv"}) || len(payload.Admitted) != 0 {
		t.Fatalf("payload=%+v", payload)
	}
	if after := traceTreeSnapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("no-source state mutated trace store\nbefore=%#v\nafter=%#v", before, after)
	}
}

// LTPM-V0-011.
func TestDogfoodRecordNULSafeChangedPathAcquisition(t *testing.T) {
	t.Parallel()
	root := newRecordFixtureAt(t, filepath.Join(t.TempDir(), "repository"))
	base := recordRevision(t, root)
	path := "notes\nfile.csv"
	writeDogfoodChange(t, root, path, "changed\n")
	commitDogfoodChange(t, root, "line-feed path")
	target := recordRevision(t, root)

	result := execute(t, candidateCommand(dogfoodRecordArguments(root, base, target)...))
	if result.exit != 0 || len(result.stderr) != 0 {
		t.Fatalf("result=%#v", result)
	}
	var payload struct {
		State      string   `json:"state"`
		Candidates []string `json:"candidates"`
	}
	if err := json.Unmarshal(result.stdout, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.State != "no-source-paths" || !reflect.DeepEqual(payload.Candidates, []string{path}) {
		t.Fatalf("payload=%+v", payload)
	}
}

// LTPM-V0-011.
func TestDogfoodRecordTargetDriftWritesNoTrace(t *testing.T) {
	t.Parallel()
	root := newRecordFixtureAt(t, filepath.Join(t.TempDir(), "repository"))
	base := recordRevision(t, root)
	writeDogfoodChange(t, root, "internal/example/value.go", "package example\n\nconst First = true\n")
	commitDogfoodChange(t, root, "target")
	target := recordRevision(t, root)
	writeDogfoodChange(t, root, "internal/example/value.go", "package example\n\nconst Second = true\n")
	commitDogfoodChange(t, root, "drift")
	before := traceTreeSnapshot(t, root)

	result := execute(t, candidateCommand(dogfoodRecordArguments(root, base, target)...))
	if result.exit != 2 || len(result.stdout) != 0 || !bytes.Contains(result.stderr, []byte(`"code": "repository-identity-changed"`)) ||
		!bytes.Contains(result.stderr, []byte("repository identity changed")) {
		t.Fatalf("result=%#v", result)
	}
	if after := traceTreeSnapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("target drift mutated trace store\nbefore=%#v\nafter=%#v", before, after)
	}
}

// LTPM-V0-011.
func TestDogfoodRecordAdmissionFailureRetainsReasonAndWritesNoTrace(t *testing.T) {
	t.Parallel()
	root := newRecordFixtureAt(t, filepath.Join(t.TempDir(), "repository"))
	base := recordRevision(t, root)
	writeDogfoodChange(t, root, "to"+"ken=fixture.txt", "secret-shaped path\n")
	commitDogfoodChange(t, root, "admission failure")
	target := recordRevision(t, root)
	before := traceTreeSnapshot(t, root)

	result := execute(t, candidateCommand(dogfoodRecordArguments(root, base, target)...))
	if result.exit != 2 || len(result.stdout) != 0 || !bytes.Contains(result.stderr, []byte(`"code": "secret-shaped-path"`)) {
		t.Fatalf("result=%#v", result)
	}
	if after := traceTreeSnapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("admission failure mutated trace store\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestDogfoodRecordRemainsOutsidePublicHelp(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"help"}, bytes.NewReader(nil), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, &stdout, &stderr)
	}
	if bytes.Contains(stdout.Bytes(), []byte("dogfood-record")) {
		t.Fatalf("hidden coordination command entered public help: %s", &stdout)
	}
}

func dogfoodRecordArguments(root, base, target string) []string {
	return []string{
		"--root", root, "dogfood-record", "--base", base, "--target", target,
		"--task", "dogfood task", "--verify", "go test ./...", "--outcome", "passed",
	}
}

func writeDogfoodChange(t *testing.T, root, path, data string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitDogfoodChange(t *testing.T, root, message string) {
	t.Helper()
	for _, arguments := range [][]string{{"add", "-A"}, {"commit", "-qm", message}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
}
