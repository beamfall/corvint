//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/postmergemetrics"
)

func cliGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	argv := append([]string{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)
	cmd := exec.Command("git", argv...)
	cmd.Dir = root
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + root, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("%v %s", e, b)
	}
	return strings.TrimSpace(string(b))
}
func inputFile(t *testing.T, path string, value any) {
	t.Helper()
	b, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestMetricsCommand(t *testing.T) {
	t.Run("PMM-V0-007 actual-git-report", func(t *testing.T) {
		root := t.TempDir()
		root, e := filepath.EvalSymlinks(root)
		if e != nil {
			t.Fatal(e)
		}
		cliGit(t, root, "init", "-q")
		file := filepath.Join(root, "content.txt")
		if e = os.WriteFile(file, []byte("bot\n"), 0600); e != nil {
			t.Fatal(e)
		}
		cliGit(t, root, "add", ".")
		cliGit(t, root, "commit", "-qm", "bot")
		bot := cliGit(t, root, "rev-parse", "HEAD")
		if e = os.WriteFile(file, []byte("human\naddition\n"), 0600); e != nil {
			t.Fatal(e)
		}
		cliGit(t, root, "commit", "-qam", "approve")
		approved := cliGit(t, root, "rev-parse", "HEAD")
		zero := int64(0)
		p := postmergemetrics.Policy{Profile: "postmerge-policy/0", Classes: []postmergemetrics.Class{{ID: "prose", MinSamples: 1}}}
		h := postmergemetrics.History{Profile: "postmerge-history/0", Classes: []postmergemetrics.Inventory{{Class: "prose", LastSequence: 1, RunsComplete: true, EventsComplete: true, RunsThrough: "2026-10-01T00:00:00Z", EventsThrough: "2026-10-01T00:00:00Z"}}, Runs: []postmergemetrics.Run{{ID: "one", Class: "prose", Sequence: 1, At: "2026-09-01T00:00:00Z", Outcome: "generated", Stages: []postmergemetrics.Stage{{Name: "draft", Outcome: "passed", DurationMS: &zero}}, Followup: postmergemetrics.Followup{Status: "created", DurationMS: &zero}, Git: &postmergemetrics.GitPair{Root: root, Bot: bot, Approved: approved}}}, Events: []postmergemetrics.Event{}}
		dir := t.TempDir()
		policy, records := filepath.Join(dir, "policy.json"), filepath.Join(dir, "records.json")
		inputFile(t, policy, p)
		inputFile(t, records, h)
		args := []string{"report", "--policy", policy, "--records", records, "--from", "2026-09-01T00:00:00Z", "--until", "2026-10-01T00:00:00Z"}
		before := cliGit(t, root, "status", "--porcelain=v1")
		var out, errOut bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if run(ctx, args, &out, &errOut) != 0 {
			t.Fatal(errOut.String())
		}
		var got postmergemetrics.Report
		if e = json.Unmarshal(out.Bytes(), &got); e != nil {
			t.Fatal(e)
		}
		m := got.Runs[0].Measurement
		if m == nil || m.Files != 1 || m.TextAdded != 2 || m.TextDeleted != 1 || got.Authority != "none" {
			t.Fatalf("%+v", got)
		}
		var again bytes.Buffer
		if run(ctx, args, &again, &errOut) != 0 || !bytes.Equal(out.Bytes(), again.Bytes()) {
			t.Fatal("report differs", errOut.String())
		}
		if after := cliGit(t, root, "status", "--porcelain=v1"); after != before {
			t.Fatal("report modified repo")
		}
		h.Events = []postmergemetrics.Event{{ID: "revert", RunID: "one", At: "2026-09-02T00:00:00Z", Kind: "revert"}}
		p.Classes[0].DemoteRuns = 2
		inputFile(t, policy, p)
		inputFile(t, records, h)
		out.Reset()
		if run(ctx, args, &out, &errOut) != 0 {
			t.Fatal(errOut.String())
		}
		if e = json.Unmarshal(out.Bytes(), &got); e != nil || got.Classes[0].Recommendation != "demoted" {
			t.Fatal(e, got)
		}

		cancelled, stop := context.WithCancel(context.Background())
		stop()
		out.Reset()
		errOut.Reset()
		if run(cancelled, args, &out, &errOut) == 0 || out.Len() != 0 || errOut.String() != "{\"error\":\"cancelled\"}\n" {
			t.Fatalf("cancelled CLI: %s %s", out.String(), errOut.String())
		}
	})
}
func TestMetricsCommandRefusals(t *testing.T) {
	t.Run("PMM-V0-007 closed-flags-and-inputs", func(t *testing.T) {
		for _, args := range [][]string{nil, {"report"}, {"report", "--policy", "one", "--policy", "two", "--from", "x", "--until", "y"}, {"report", "--policy", "one", "--records", "two", "--from", "x", "--unknown", "y"}} {
			var out, errOut bytes.Buffer
			if run(context.Background(), args, &out, &errOut) == 0 || out.Len() != 0 || errOut.String() != "{\"error\":\"invalid-input\"}\n" {
				t.Fatalf("%v %s %s", args, out.String(), errOut.String())
			}
		}
		if _, e := read(t.TempDir(), 100); e != postmergemetrics.ErrInput {
			t.Fatal(e)
		}
		f := filepath.Join(t.TempDir(), "fifo")
		if e := syscall.Mkfifo(f, 0600); e != nil {
			t.Fatal(e)
		}
		done := make(chan error, 1)
		go func() { _, e := read(f, 100); done <- e }()
		select {
		case e := <-done:
			if e != postmergemetrics.ErrInput {
				t.Fatal(e)
			}
		case <-time.After(time.Second):
			t.Fatal("FIFO input blocked")
		}
	})
}
