// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	q "github.com/Beamfall/corvint/internal/runnerqualification"
	s "github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
)

func pinnedFixture(t *testing.T) (string, q.Replay) {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", root}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_COMMITTER_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %v %s", args, e, b)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-b", "main")
	git("-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "base")
	base := git("rev-parse", "HEAD")
	git("checkout", "-b", "topic")
	git("-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "head")
	git("checkout", "main")
	git("-c", "commit.gpgsign=false", "merge", "--no-ff", "-m", "merge", "topic")
	merge := git("rev-parse", "HEAD")
	h := strings.Repeat("a", 64)
	d := q.Declaration{Runner: "fixture", Corpus: q.Corpus{Repository: "fixture", Commit: merge}, Model: "traced", Classes: []s.Class{s.DataEdit}, Identity: q.Identity{Runner: testevidence.RunnerIdentity{Plugin: "fixture", PluginVersion: "1", ExecutableDigest: h, RunnerVersion: "1", ConfigurationDigest: h}, Execution: testevidence.ExecutionIdentity{OS: "linux", Architecture: "amd64", TracingBackend: "fixture", TracingVersion: "1", EnvironmentDigest: h}, SelectorVersion: "1", RecordFormat: testevidence.Format, NonSemanticDigest: h, VerifierRevision: base}}
	return root, q.Replay{Request: q.Request{Declaration: d, Checkout: root, Merges: []q.Merge{{Commit: merge, Base: base, Head: merge, Classes: []s.Class{s.DataEdit}}}, Samples: 300, Budget: time.Minute}}
}

func TestRSPV0034OperatorReplayWritesUnadmittedFailure(t *testing.T) {
	root, replay := pinnedFixture(t)
	input, out := filepath.Join(t.TempDir(), "input.json"), filepath.Join(t.TempDir(), "candidate.json")
	b, e := json.Marshal(replay)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(input, b, 0600); e != nil {
		t.Fatal(e)
	}
	var stdout, stderr bytes.Buffer
	rc := run(context.Background(), []string{"replay", "-input", input, "-checkout", root, "-out", out, "-budget", "1m"}, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("rc=%d out=%s err=%s", rc, &stdout, &stderr)
	}
	b, e = os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var record q.Record
	if e = json.Unmarshal(b, &record); e != nil {
		t.Fatal(e)
	}
	if record.Complete || !strings.Contains(strings.Join(record.Reasons, ","), "retained-capture-missing") {
		t.Fatalf("missing evidence qualified: %+v", record)
	}
	if rc = run(context.Background(), []string{"replay", "-input", input, "-checkout", root, "-out", out, "-budget", "1m"}, &stdout, &stderr); rc != 2 {
		t.Fatal("overwrote record")
	}
}
func TestRSPV0034PinnedMergeVerifier(t *testing.T) {
	root, replay := pinnedFixture(t)
	if e := verifyPins(context.Background(), replay.Request.Declaration, root, replay.Request.Merges); e != nil {
		t.Fatal(e)
	}
	replay.Request.Merges[0].Base = replay.Request.Merges[0].Head
	if e := verifyPins(context.Background(), replay.Request.Declaration, root, replay.Request.Merges); e == nil {
		t.Fatal("wrong merge pins accepted")
	}
}
func TestRSPV0033OperatorHelpAndStrictInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if rc := run(context.Background(), []string{"--help"}, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "Example:") || !strings.Contains(stdout.String(), "UNADMITTED") {
		t.Fatal("missing usage or authority limit")
	}
	stdout.Reset()
	if rc := run(context.Background(), []string{"help", "--json"}, &stdout, &stderr); rc != 0 {
		t.Fatal(rc)
	}
	b, e := os.ReadFile("command-tree.json")
	if e != nil {
		t.Fatal(e)
	}
	var got, want any
	if json.Unmarshal(stdout.Bytes(), &got) != nil || json.Unmarshal(b, &want) != nil {
		t.Fatal("invalid command tree")
	}
	gb, _ := json.Marshal(got)
	wb, _ := json.Marshal(want)
	if !bytes.Equal(gb, wb) {
		t.Fatal("command tree stale")
	}
	for _, bad := range []string{`{"unknown":true}`, `{"Request":{},"Request":{}}`, `{"Request":`} {
		input := filepath.Join(t.TempDir(), "input")
		if e = os.WriteFile(input, []byte(bad), 0600); e != nil {
			t.Fatal(e)
		}
		out := filepath.Join(t.TempDir(), "output")
		if rc := run(context.Background(), []string{"replay", "-input", input, "-checkout", t.TempDir(), "-out", out, "-budget", "1m"}, &stdout, &stderr); rc != 2 {
			t.Fatalf("invalid input accepted: %s rc %d", bad, rc)
		}
		if _, e = os.Stat(out); !os.IsNotExist(e) {
			t.Fatal("invalid input wrote output")
		}
	}
}
