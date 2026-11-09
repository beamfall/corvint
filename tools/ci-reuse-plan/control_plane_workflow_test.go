package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testplatform"
)

// AFP-V0-5102: the trusted metadata job starts on PR activity without a CI dependency.
func TestV15102ControlPlaneStartsOnPRActivity(t *testing.T) {
	w := readWorkflow(t, "ci-control-plane.yml")
	for _, line := range []string{
		"  pull_request_target:",
		"    branches: [main]",
		"    types: [opened, synchronize, reopened, ready_for_review]",
		"  workflow_run:",
		"    workflows: [CI]",
		"    types: [completed]",
	} {
		wantLine(t, "triggers", w, line)
	}
	b := jobBlock(t, w, "control-plane")
	for _, job := range []string{"control-plane", "merge-group", "main-miss"} {
		if got := statusJobRuns(t, w, job, map[string]any{"event_name": "pull_request_target"}); got != (job == "control-plane") {
			t.Errorf("PR activity starts %s = %v", job, got)
		}
	}
	for _, forbidden := range []string{"actions/checkout", " uses:", "needs:", "write-all"} {
		if strings.Contains(b, forbidden) {
			t.Errorf("metadata job contains %q", forbidden)
		}
	}
	if !strings.Contains(b, "if: github.event_name == 'pull_request_target' || (") {
		t.Error("metadata job does not start independently on pull_request_target")
	}
	if !strings.Contains(b, "    permissions:\n      contents: read\n      pull-requests: read\n      statuses: write\n    steps:") {
		t.Error("metadata job must declare exactly its three minimal permissions")
	}
	if strings.Contains(runScript(t, b), "${{") {
		t.Error("GitHub expressions must enter the shell through env, never run text")
	}
	for _, line := range []string{
		"          HEAD: ${{ github.event.pull_request.head.sha || github.event.workflow_run.head_sha }}",
		"          PR_NUMBER: ${{ github.event.pull_request.number }}",
	} {
		wantLine(t, "metadata inputs", b, line)
	}
}

// Execute the actual trusted script with JSON API responses. Each negative case first has a
// passing complete fixture; the call trace proves the intended failure/revalidation was reached.
func TestV15102ControlPlaneMetadataBoundToHead(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the control-plane step runs under bash with a gh spy")
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatal(err)
		}
	}
	pr := func(head, base, state string, count int) string {
		return fmt.Sprintf(`{"state":%q,"head":{"sha":%q},"base":{"sha":%q},"changed_files":%d}`, state, head, base, count)
	}
	good := pr(runHead, "base", "open", 2)
	clean := `[[{"filename":"README.md"}],[{"filename":"docs/x.md"}]]`
	workflow := `[[{"filename":"README.md"},{"filename":".github/workflows/ci.yml"}]]`
	waiver := `[{"context":"ci-control-plane-waiver","state":"success","creator":{"login":"Russe11"}}]`
	var cappedFiles []map[string]string
	for i := 0; i < 3000; i++ {
		cappedFiles = append(cappedFiles, map[string]string{"filename": fmt.Sprintf("docs/%d.md", i)})
	}
	cappedJSON, err := json.Marshal([]any{cappedFiles})
	if err != nil {
		t.Fatal(err)
	}
	type fixture struct {
		before, after, files, statuses, fail string
		fallback                             bool
		want, last                           string
	}
	baseline := fixture{before: good, after: good, files: clean, statuses: "[]", want: "success", last: "post"}
	run := func(t *testing.T, f fixture) {
		t.Helper()
		dir := t.TempDir()
		for name, data := range map[string]string{"before.json": f.before, "after.json": f.after, "files.json": f.files, "statuses.json": f.statuses} {
			if !json.Valid([]byte(data)) {
				t.Fatalf("invalid fixture %s", name)
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		stub := `#!/bin/sh
[ "$1" = api ] || exit 2
shift
call() {
  echo "$1" >>"$STUB_DIR/calls"
  [ "$FAIL_AT" != "$1" ] || exit 22
}
case " $* " in
  *" -X POST "*)
    call post
    echo "$*" >>"$STUB_DIR/posts" ;;
  *"repos/$REPO/pulls?state=open&per_page=100"*)
    [ "$FALLBACK" = yes ] || exit 23
    call lookup
    echo 1144 ;;
  *" --paginate --slurp repos/$REPO/pulls/1144/files?per_page=100 "*)
    call files
    cat "$STUB_DIR/files.json" ;;
  *"repos/$REPO/pulls/1144 "*)
    if [ -f "$STUB_DIR/read" ]; then
      call after
      cat "$STUB_DIR/after.json"
    else
      call before
      touch "$STUB_DIR/read"
      cat "$STUB_DIR/before.json"
    fi ;;
  *" --paginate repos/$REPO/commits/$HEAD/statuses?per_page=100 --jq "*)
    call statuses
    for a; do last=$a; done
    jq -r "$last" "$STUB_DIR/statuses.json" ;;
  *) exit 24 ;;
esac
`
		if err := execfile.WriteFile(filepath.Join(dir, "gh"), []byte(stub), 0o700); err != nil {
			t.Fatal(err)
		}
		script := filepath.Join(dir, "step.sh")
		if err := os.WriteFile(script, []byte(runScript(t, jobBlock(t, readWorkflow(t, "ci-control-plane.yml"), "control-plane"))), 0o600); err != nil {
			t.Fatal(err)
		}
		fallback, number := "no", "1144"
		if f.fallback {
			fallback, number = "yes", ""
		}
		cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", script)
		cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
			"STUB_DIR="+dir, "REPO=beamfall/corvint", "HEAD="+runHead, "PR_NUMBER="+number,
			"FALLBACK="+fallback, "FAIL_AT="+f.fail, "WAIVER_APPROVERS=Russe11", "GH_TOKEN=stub",
			"RUN_URL=https://github.com/beamfall/corvint/actions/runs/1")
		out, err := cmd.CombinedOutput()
		posts, _ := os.ReadFile(filepath.Join(dir, "posts"))
		calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
		if !strings.HasSuffix(string(calls), f.last+"\n") {
			t.Errorf("calls %q: want last %s; output %s", calls, f.last, out)
		}
		if f.want == "" {
			if len(posts) != 0 {
				t.Errorf("unsafe status posted: %s", posts)
			}
			return
		}
		if err != nil || strings.Count(string(posts), "\n") != 1 ||
			!strings.Contains(string(posts), "repos/beamfall/corvint/statuses/"+runHead+" ") ||
			!strings.Contains(string(posts), "state="+f.want+" ") || !strings.Contains(string(posts), "context=ci-control-plane ") {
			t.Fatalf("err %v, posts %q, want %s; output %s", err, posts, f.want, out)
		}
		if !strings.HasSuffix(string(calls), "after\npost\n") {
			t.Errorf("no revalidation immediately before post: %s", calls)
		}
	}
	// Prove the fixture/encoding works before any injected failure.
	t.Run("complete clean listing", func(t *testing.T) { run(t, baseline) })
	for _, tc := range []struct {
		name   string
		change func(*fixture)
	}{
		{"workflow path", func(f *fixture) { f.files, f.want = workflow, "failure" }},
		{"renamed away", func(f *fixture) {
			f.files = `[[{"filename":"README.md"},{"filename":"docs/x.md","previous_filename":".github/x"}]]`
			f.want = "failure"
		}},
		{"owner waiver", func(f *fixture) { f.files, f.statuses = workflow, waiver }},
		{"workflow_run waiver rerun", func(f *fixture) { f.files, f.statuses, f.fallback = workflow, waiver, true }},
		{"old event head", func(f *fixture) { f.before = pr("new-head", "base", "open", 2); f.want, f.last = "", "before" }},
		{"head changed while listing", func(f *fixture) { f.after = pr("new-head", "base", "open", 2); f.want, f.last = "", "after" }},
		{"head changed after waiver", func(f *fixture) {
			f.files, f.statuses = workflow, waiver
			f.after = pr("new-head", "base", "open", 2)
			f.want, f.last = "", "after"
		}},
		{"base changed", func(f *fixture) { f.after = pr(runHead, "new-base", "open", 2); f.want, f.last = "", "after" }},
		{"PR closed", func(f *fixture) { f.after = pr(runHead, "base", "closed", 2); f.want, f.last = "", "after" }},
		{"file count changed", func(f *fixture) { f.after = pr(runHead, "base", "open", 3); f.want, f.last = "", "after" }},
		{"3000 files even with waiver", func(f *fixture) {
			f.before = pr(runHead, "base", "open", 3000)
			f.statuses = waiver
			f.want, f.last = "", "before"
		}},
		{"over 3000 files", func(f *fixture) { f.before = pr(runHead, "base", "open", 3001); f.want, f.last = "", "before" }},
		{"incomplete pages", func(f *fixture) { f.files = `[[{"filename":"README.md"}]]`; f.want, f.last = "", "files" }},
		{"listing reaches API cap", func(f *fixture) { f.files = string(cappedJSON); f.want, f.last = "", "files" }},
		{"missing file name", func(f *fixture) { f.files = `[[{"filename":"README.md"},{}]]`; f.want, f.last = "", "files" }},
		{"files call fails", func(f *fixture) { f.fail, f.want, f.last = "files", "", "files" }},
		{"first read fails", func(f *fixture) { f.fail, f.want, f.last = "before", "", "before" }},
		{"revalidation fails", func(f *fixture) { f.fail, f.want, f.last = "after", "", "after" }},
		{"waiver call fails", func(f *fixture) { f.files = workflow; f.fail, f.want, f.last = "statuses", "", "statuses" }},
	} {
		t.Run(tc.name, func(t *testing.T) { f := baseline; tc.change(&f); run(t, f) })
	}
}

// V1-5096 (replaces the AFP-V0-171 user-success keep): when CI reruns on a pull-request head
// that changes .github/, the control-plane job posts ci-control-plane success only when the newest
// ci-control-plane-waiver status on that head was posted by a login in WAIVER_APPROVERS (matched
// case-insensitively), found past the first page of statuses. A hand-posted ci-control-plane
// success, a waiver by any other writer, a withdrawn waiver, or no status at all gets failure, and
// an unreadable status listing posts nothing.
func TestV15096ControlPlaneWaiverNeedsAnApprover(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the control-plane step runs under bash with a #!/bin/sh gh spy")
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("the control-plane step needs %s on PATH: %v", tool, err)
		}
	}
	const head = "59cc560b9d98f746bba0282d18c0442e65b85a98"
	script := filepath.Join(t.TempDir(), "step.sh")
	if err := os.WriteFile(script, []byte(runScript(t, jobBlock(t, readWorkflow(t, "ci-control-plane.yml"), "control-plane"))), 0o600); err != nil {
		t.Fatal(err)
	}
	status := func(state, login, context string) string {
		return `{"state":"` + state + `","context":"` + context + `","creator":{"login":"` + login + `","type":"User"}}`
	}
	// run returns the status POSTs of one step run with STATUSES as the newest-first listing of
	// statuses on the head ("" makes the listing fail), and whether the step exited zero.
	run := func(statuses string) (string, bool) {
		dir := t.TempDir()
		bin := filepath.Join(dir, "bin")
		if err := os.Mkdir(bin, 0o700); err != nil {
			t.Fatal(err)
		}
		if statuses != "" {
			if err := os.WriteFile(filepath.Join(dir, "statuses.json"), []byte(statuses), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		// A stub gh: one open PR (#1144) at HEAD that changes a workflow; the status listing
		// is filtered with the step's own --jq through jq; a status POST is appended to posts.log.
		stub := `#!/bin/sh
[ "$1" = api ] || exit 2
shift
case " $* " in
  *" -X POST "*) echo "$*" >>"$STUB_DIR/posts.log"; exit 0 ;;
  *"repos/$REPO/pulls?state=open&per_page=100"*) echo 1144 ;;
  *"repos/$REPO/pulls/1144/files?per_page=100"*) printf '[[{"filename":"README.md"},{"filename":".github/workflows/ci.yml"}]]\n' ;;
  *"repos/$REPO/pulls/1144 "*) printf '{"state":"open","head":{"sha":"%s"},"base":{"sha":"base"},"changed_files":2}\n' "$HEAD" ;;
  *" --paginate repos/$REPO/commits/$HEAD/statuses?per_page=100 --jq "*)
    # One JSON document per page; --paginate applies the --jq filter to every page.
    [ -f "$STUB_DIR/statuses.json" ] || exit 22
    for a; do last=$a; done
    exec jq -r "$last" "$STUB_DIR/statuses.json" ;;
  *) exit 22 ;;
esac
`
		if err := execfile.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o700); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", script)
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"STUB_DIR="+dir, "REPO=beamfall/corvint", "HEAD="+head,
			"RUN_URL=https://github.com/beamfall/corvint/actions/runs/1", "GH_TOKEN=stub", "WAIVER_APPROVERS=Russe11")
		err := cmd.Run()
		posts, _ := os.ReadFile(filepath.Join(dir, "posts.log"))
		return string(posts), err == nil
	}
	waiver := status("success", "Russe11", "ci-control-plane-waiver")
	var page1 []string
	for i := 0; i < 100; i++ {
		page1 = append(page1, status("success", "github-actions[bot]", "go-product"))
	}
	for name, tc := range map[string]struct{ statuses, state string }{
		"approver waiver":            {"[" + waiver + "," + status("failure", "github-actions[bot]", "ci-control-plane") + "]", "success"},
		"approver login, other case": {"[" + status("success", "russe11", "ci-control-plane-waiver") + "]", "success"},
		"waiver past 100 statuses":   {"[" + strings.Join(page1, ",") + "]\n[" + waiver + "]", "success"},
		"new push, no status":        {"[]", "failure"},
		"hand-posted gate success":   {"[" + status("success", "Russe11", "ci-control-plane") + "]", "failure"},
		"waiver by another writer":   {"[" + status("success", "lane-bot", "ci-control-plane-waiver") + "]", "failure"},
		"waiver by a login prefix":   {"[" + status("success", "Russe1", "ci-control-plane-waiver") + "]", "failure"},
		"approver withdrew":          {"[" + status("failure", "Russe11", "ci-control-plane-waiver") + "," + waiver + "]", "failure"},
		"other writer after waiver":  {"[" + status("failure", "lane-bot", "ci-control-plane-waiver") + "," + waiver + "]", "success"},
	} {
		posts, ok := run(tc.statuses)
		if !ok || strings.Count(posts, "\n") != 1 || !strings.Contains(posts, "repos/beamfall/corvint/statuses/"+head) ||
			!strings.Contains(posts, "state="+tc.state+" ") || !strings.Contains(posts, "context=ci-control-plane ") {
			t.Errorf("%s: exit ok %v, posted %q, want one ci-control-plane %s", name, ok, posts, tc.state)
		}
		if tc.state == "success" && !strings.Contains(posts, "owner waiver by ") {
			t.Errorf("%s: posted %q, want the approver named in the description", name, posts)
		}
	}
	if posts, ok := run(""); ok || posts != "" {
		t.Errorf("an unreadable status listing: exit ok %v, posted %q, want a failed step that posts nothing", ok, posts)
	}
}
