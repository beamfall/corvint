// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/procgroup"
)

func contextFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{"go.mod": "module example.com/editorcontext\n\ngo 1.27.1\n", "AGENTS.md": "Use docs/specs/widget-v0.md for widget Value changes.\n", "internal/AGENTS.md": "Widget changes require TestValue.\n", "docs/specs/widget-v0.md": "# Widget\n\nPROOF-001: Value must remain observable.\n", "internal/widget/widget.go": "package widget\nfunc Value() int {return 42}\n", "internal/widget/widget_test.go": "package widget\nimport \"testing\"\nfunc TestValue(t *testing.T){if Value()!=42 {t.Fatal(\"bad\")}}\n"}
	for path, body := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "Fixture"}} {
		fixtureGit(t, root, args...)
	}
	return root
}
func fixtureGit(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git fixture: %s %v", b, e)
	}
	return b
}
func contextBinary(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("owned worker profile supports Darwin/Linux")
	}
	binary := filepath.Join(t.TempDir(), "corvint-lsp")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/corvint-lsp")
	c.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off")
	if b, e := c.CombinedOutput(); e != nil {
		t.Fatalf("native build: %s %v", b, e)
	}
	return binary
}
func TestNativeContextWorker(t *testing.T) {
	executable := contextBinary(t)
	root := contextFixture(t)
	input := contextWorkerInput{Subject: "internal/widget/widget.go", Task: "change widget Value PROOF-001", Limit: 6}
	body, _ := json.Marshal(input)
	registry, e := bridge.NewTaskReview(root)
	if e != nil {
		t.Fatal(e)
	}
	expected, failure := registry.Call(context.Background(), bridge.ToolContext, body)
	if failure != nil {
		t.Fatal(failure)
	}
	expectedObject, failure := expected.Object()
	if failure != nil {
		t.Fatal(failure)
	}
	expectedBytes, _ := json.Marshal(expectedObject)
	before := fixtureGit(t, root, "status", "--porcelain")
	beforeFiles := fixtureContents(t, root)
	subject := expected.Receipt["subject"].(map[string]any)
	blob := strings.TrimSpace(string(fixtureGit(t, root, "rev-parse", "HEAD:internal/widget/widget.go")))
	if subject["blob_hash"] != blob || expected.Repository.CommitRevision != strings.TrimSpace(string(fixtureGit(t, root, "rev-parse", "HEAD"))) || expected.Repository.TreeRevision != strings.TrimSpace(string(fixtureGit(t, root, "rev-parse", "HEAD^{tree}"))) {
		t.Fatal("Core fixture evidence did not resolve to committed objects")
	}

	for attempt := 0; attempt < 2; attempt++ {
		o := procgroup.Run(context.Background(), contextWorkerSpec(executable, root, body))
		if o.Err != nil || o.ExitStatus != 0 || !contextWorkerCleanup(o) {
			t.Fatalf("native worker not admissible: %+v", o)
		}
		if string(o.Stdout) != string(expectedBytes) {
			t.Fatal("worker changed exact frozen bridge object")
		}
		for _, want := range []string{"AGENTS.md", "PROOF-001", "widget_test.go"} {
			if !strings.Contains(string(o.Stdout), want) {
				t.Errorf("missing fixture evidence %s", want)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		o = procgroup.Run(ctx, contextWorkerSpec(executable, root, body))
		cancel()
		if !o.Cancelled || !contextWorkerCleanup(o) {
			t.Fatalf("deadline cleanup: %+v", o)
		}
	}
	input.Subject = "internal/widget/missing.go"
	body, _ = json.Marshal(input)
	abstained, failure := registry.Call(context.Background(), bridge.ToolContext, body)
	if failure != nil || abstained.State != "ABSTAINED" {
		t.Fatal("fixture did not produce a real abstention", failure, abstained.State)
	}
	object, failure := abstained.Object()
	if failure != nil {
		t.Fatal(failure)
	}
	bytes, _ := json.Marshal(object)
	observation := procgroup.Run(context.Background(), contextWorkerSpec(executable, root, body))
	if observation.Err != nil || !contextWorkerCleanup(observation) || string(observation.Stdout) != string(bytes) {
		t.Fatal("real Core abstention was not preserved", observation.Err)
	}
	after := fixtureGit(t, root, "status", "--porcelain")
	if string(before) != string(after) || !reflect.DeepEqual(beforeFiles, fixtureContents(t, root)) {
		t.Fatal("fixture worktree changed")
	}
	// Fixed mode never launches gopls; its only result is the task-review object.
	if expectedObject["tool"] != bridge.ToolContext {
		t.Fatal("wrong Core operation")
	}
}
func TestWorkerIdentityDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker")
	os.WriteFile(path, []byte("first"), 0600)
	digest, e := workerDigest(path)
	if e != nil {
		t.Fatal(e)
	}
	identity := workerIdentity{path, digest}
	if !identity.current() {
		t.Fatal("stable identity rejected")
	}
	os.WriteFile(path, []byte("second"), 0600)
	if identity.current() {
		t.Fatal("replaced worker accepted")
	}
}

func TestWorkerCleanupUnknownRefuses(t *testing.T) {
	base := procgroup.Observation{WaitCompleted: true, PipesDrained: true, OwnedProcessGroupCleanup: true, DescendantCleanupStatus: "owned-process-group", DescendantObservation: &procgroup.DescendantObservation{Scope: "observed-pid-start-identities", IntervalMS: 20, Absent: true}}
	if !contextWorkerCleanup(base) {
		t.Fatal("documented normal runner qualification rejected")
	}
	cases := []procgroup.Observation{base, base, base, base, base, base}
	cases[0].WaitCompleted = false
	cases[1].PipesDrained = false
	cases[2].OwnedProcessGroupCleanup = false
	cases[3].DescendantCleanupQualification = "PARTIAL"
	cases[4].DescendantObservation = nil
	cases[5].DescendantCleanupStatus = ""
	for _, o := range cases {
		if contextWorkerCleanup(o) {
			t.Fatal("unknown/incomplete cleanup accepted")
		}
	}
}

func fixtureContents(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	files := map[string][32]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[relative] = sha256.Sum256(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
