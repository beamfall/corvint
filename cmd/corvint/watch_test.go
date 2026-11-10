// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/linecov/watch"
)

func watchGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := fixtureGitCommand(args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// watchModule commits a one-package module whose F body is a known block and
// writes an index in which TestPass reached it and TestFail did not.
func watchModule(t *testing.T) (root, index string) {
	t.Helper()
	root = t.TempDir()
	for rel, body := range map[string]string{
		"go.mod":      "module example.com/m\n\ngo 1.22\n",
		"p/p.go":      "package p\nfunc F() int {\n return 1\n}\n",
		"p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n\nfunc TestFail(t *testing.T) { t.Fail() }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	watchGit(t, root, "init", "-q")
	watchGit(t, root, "add", ".")
	watchGit(t, root, "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "m")
	base := watchGit(t, root, "rev-parse", "HEAD")
	ctx := context.Background()
	oids, err := linecov.OIDs(ctx, root, base)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := linecov.RunnerID(ctx, root, nil, linecov.DefaultTestTimeout, "")
	if err != nil {
		t.Fatal(err)
	}
	ix, err := linecov.Build("example.com/m", base, id, oids, []linecov.Record{
		{Package: "p", Name: "TestPass", EnvObserved: true, Profile: []byte("mode: atomic\nexample.com/m/p/p.go:2.14,4.2 1 1\n")},
		{Package: "p", Name: "TestFail", EnvObserved: true, Profile: []byte("mode: atomic\nexample.com/m/p/p.go:2.14,4.2 1 0\n")},
		{Package: "p", Name: linecov.InOrder, Union: true, EnvObserved: true, Profile: []byte("mode: atomic\nexample.com/m/p/p.go:2.14,4.2 1 0\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	index = filepath.Join(t.TempDir(), "linecov.idx")
	f, err := os.Create(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := ix.Encode(f); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return root, index
}

func watchCLI(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func watchEvents(t *testing.T, stdout string) []watch.Event {
	t.Helper()
	var out []watch.Event
	for _, l := range strings.Split(strings.TrimSpace(stdout), "\n") {
		var e watch.Event
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			t.Fatalf("%q: %v", l, err)
		}
		if e.Scope != "user" || e.GateEligible {
			t.Fatalf("event not user-scoped: %+v", e)
		}
		out = append(out, e)
	}
	return out
}

// LPCV-V0-1085: the verb is listed, has help with usage and an example, and
// refuses a missing index, conflicting sources, a non-repository root and an
// unreadable index with exit 2 and a typed diagnostic on stderr only.
func TestWatchHelpAndRefusals(t *testing.T) {
	t.Parallel()
	code, stdout, _ := watchCLI(t, "", "watch", "--help")
	if code != 0 || !strings.Contains(stdout, "Usage:\n  corvint [--root PATH] watch --index FILE") || !strings.Contains(stdout, "Example:\n  corvint watch --index linecov.idx --once --dry-run") {
		t.Fatalf("help: exit %d\n%s", code, stdout)
	}
	if code, stdout, _ := watchCLI(t, "", "help", "watch"); code != 0 || !strings.HasPrefix(stdout, watchHelp) {
		t.Fatalf("help watch: exit %d", code)
	}
	root, index := watchModule(t)
	for _, c := range []struct {
		name, code string
		args       []string
	}{
		{"missing index", "watch-invalid-arguments", []string{"--root", root, "watch", "--once"}},
		{"both sources", "watch-invalid-arguments", []string{"--root", root, "watch", "--index", index, "--once", "--events-stdin"}},
		{"positional", "watch-invalid-arguments", []string{"--root", root, "watch", "--index", index, "p"}},
		{"unreadable index", "watch-index-unavailable", []string{"--root", root, "watch", "--index", filepath.Join(root, "go.mod"), "--once"}},
		{"not a repository", "not a Git repository", []string{"--root", t.TempDir(), "watch", "--index", index, "--once"}},
	} {
		code, stdout, stderr := watchCLI(t, "", c.args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, c.code) {
			t.Errorf("%s: exit %d stdout %q stderr %q", c.name, code, stdout, stderr)
		}
	}
}

// LPCV-V0-1085, LPCV-V0-1087: --once --dry-run answers which tests a
// working-tree edit can affect, with its replan latency, without running
// one; --once runs them and exits 1 on a failure; stdin notifications drive
// the loop and a closed stdin ends it with status 0.
func TestWatchOnceDryRunCLI(t *testing.T) {
	root, index := watchModule(t)
	if err := os.WriteFile(filepath.Join(root, "p", "p.go"), []byte("package p\nfunc F() int {\n return 2\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := watchCLI(t, "", "--root", root, "watch", "--index", index, "--once", "--dry-run", "--packages", "p")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q stdout %q", code, stderr, stdout)
	}
	got := watchEvents(t, stdout)
	if len(got) != 1 || got[0].Kind != "plan" || got[0].Plan == nil || len(got[0].Plan.Run) != 1 || got[0].Plan.Run[0].Run != "^(TestPass)$" || got[0].ReplanMillis <= 0 {
		t.Fatalf("events %+v", got)
	}
	// Two stdin saves: the first run fails (TestFail is reached once F's body
	// changes shape so both tests select), the second passes after a repair.
	if err := os.WriteFile(filepath.Join(root, "p", "p_test.go"), []byte("package p\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n\nfunc TestFail(t *testing.T) {\n\tif F() != 3 {\n\t\tt.Fatal(F())\n\t}\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = watchCLI(t, "", "--root", root, "watch", "--index", index, "--once", "--packages", "p")
	if code != 1 {
		t.Fatalf("a failing once run exited %d: %s", code, stdout)
	}
	got = watchEvents(t, stdout)
	if len(got) != 2 || got[1].Kind != "result" || got[1].ExitCode != 1 {
		t.Fatalf("events %+v", got)
	}
	// A loop that ends by a closed stdin exits 0 though its run failed: only
	// --once reports the run in the exit status.
	code, stdout, _ = watchCLI(t, "save\n", "--root", root, "watch", "--index", index, "--events-stdin", "--packages", "p")
	if got = watchEvents(t, stdout); code != 0 || len(got) != 2 || got[1].Kind != "result" || got[1].ExitCode != 1 {
		t.Fatalf("a failing stdin-driven run exited %d: %+v", code, got)
	}
	if err := os.WriteFile(filepath.Join(root, "p", "p.go"), []byte("package p\nfunc F() int {\n return 3\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = watchCLI(t, "save\n", "--root", root, "watch", "--index", index, "--events-stdin", "--packages", "p")
	if code != 0 {
		t.Fatalf("a repaired stdin-driven run exited %d: %s", code, stdout)
	}
	if got = watchEvents(t, stdout); len(got) != 2 || got[1].Kind != "result" || got[1].ExitCode != 0 {
		t.Fatalf("events %+v", got)
	}
}
