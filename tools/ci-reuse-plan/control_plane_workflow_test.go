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

// CCEP-V0-001: the trusted metadata job starts on PR activity without a CI dependency.
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
	// The main-miss job moved to main-miss.yml (CIN-V0-7922), which runs on workflow_run alone;
	// TestCINV07922MainMissWorkflowBinding_CINV07922 refuses any pull_request trigger there.
	for _, job := range []string{"control-plane", "merge-group"} {
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
	// The job inherits exactly the three workflow-level permissions that the
	// least-privilege exception row declares (ARTIFACT-V0-008).
	if strings.Contains(b, "    permissions:") || !strings.Contains(w, "\npermissions:\n  contents: read\n  pull-requests: read\n  statuses: write\n\n") {
		t.Error("metadata job must inherit exactly the three minimal workflow-level permissions")
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

// CCEP-V0-001: execute the actual trusted script with JSON API responses. Each negative case first has a
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
	clean := `[[{"filename":"README.md"},{"filename":"docs/x.md"}]]`
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
	type fixture = controlPlaneFixture
	baseline := fixture{before: good, after: good, files: clean, statuses: "[]", want: "success", last: "post"}
	run := func(t *testing.T, f fixture) {
		t.Helper()
		runControlPlaneFixture(t, f)
	}
	multiJSON, err := json.Marshal([]any{cappedFiles[:100], cappedFiles[100:101]})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("complete multiple pages", func(t *testing.T) {
		f := baseline
		f.before, f.after, f.files = pr(runHead, "base", "open", 101), pr(runHead, "base", "open", 101), string(multiJSON)
		run(t, f)
	})
	shortJSON, err := json.Marshal([]any{cappedFiles[:99], cappedFiles[99:101]})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("short nonfinal page with matching total and page count", func(t *testing.T) {
		f := baseline
		f.before, f.after, f.files = pr(runHead, "base", "open", 101), pr(runHead, "base", "open", 101), string(shortJSON)
		f.want, f.last = "", "files"
		run(t, f)
	})
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
		// V1-5263: at the ceiling no file is listed; without a base ref name the tree comparison
		// cannot start, and a waiver cannot turn that into a status.
		{"3000 files and no base ref even with waiver", func(f *fixture) {
			f.before = pr(runHead, "base", "open", 3000)
			f.statuses = waiver
			f.want, f.last = "", "before"
		}},
		{"over 3000 files and no base ref", func(f *fixture) {
			f.before = pr(runHead, "base", "open", 3001)
			f.want, f.last = "", "before"
		}},
		{"short nonfinal page", func(f *fixture) {
			f.files = `[[{"filename":"README.md"}],[{"filename":"docs/x.md"}]]`
			f.want, f.last = "", "files"
		}},
		{"empty extra page", func(f *fixture) {
			f.files = `[[{"filename":"README.md"},{"filename":"docs/x.md"}],[]]`
			f.want, f.last = "", "files"
		}},
		{"duplicate masks omitted file", func(f *fixture) {
			f.files = `[[{"filename":"README.md"},{"filename":"README.md"}]]`
			f.want, f.last = "", "files"
		}},
		{"empty file name", func(f *fixture) {
			f.files = `[[{"filename":"README.md"},{"filename":""}]]`
			f.want, f.last = "", "files"
		}},
		{"missing metadata", func(f *fixture) { f.before = `{}`; f.want, f.last = "", "before" }},
		{"missing count", func(f *fixture) {
			f.before = strings.ReplaceAll(good, `,"changed_files":2`, "")
			f.want, f.last = "", "before"
		}},
		{"missing base", func(f *fixture) { f.before = pr(runHead, "", "open", 2); f.want, f.last = "", "before" }},
		{"lookup fails", func(f *fixture) { f.fallback = true; f.fail, f.want, f.last = "lookup", "", "lookup" }},
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
	for _, key := range []string{"HEAD", "REPO", "PR_NUMBER", "GH_TOKEN", "RUN_URL", "EVENT_NAME"} {
		t.Run("missing input "+key, func(t *testing.T) {
			f := baseline
			f.env, f.want, f.last = map[string]string{key: ""}, "", ""
			run(t, f)
		})
	}
	for key, value := range map[string]string{"HEAD": `bad"head`, "REPO": "../other/repo", "PR_NUMBER": "../1", "EVENT_NAME": "push"} {
		t.Run("malformed input "+key, func(t *testing.T) {
			f := baseline
			f.env, f.want, f.last = map[string]string{key: value}, "", ""
			run(t, f)
		})
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
			"STUB_DIR="+dir, "REPO=beamfall/corvint", "HEAD="+head, "EVENT_NAME=workflow_run",
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

// controlPlaneFixture is one run of the control-plane step against a gh spy. reads maps the exact
// URL of any other GET (the V1-5263 tree comparison) to its response body; such a call is traced
// by its URL, and a GET with no fixture is traced as "unexpected ..." and fails.
type controlPlaneFixture struct {
	before, after, files, statuses, fail string
	// failAfter names a read that prints its fixture body and then exits non-zero.
	failAfter  string
	fallback   bool
	env        map[string]string
	reads      map[string]string
	want, last string
}

// controlPlaneRun is what one fixture run did: the gh call trace, the status POSTs, the step's
// output and whether the step exited non-zero.
type controlPlaneRun struct {
	calls, posts, out string
	failed            bool
}

// runControlPlaneFixture executes the workflow's real control-plane script and checks the last
// call, the single status f.want (none when empty) and the revalidation right before a POST.
func runControlPlaneFixture(t *testing.T, f controlPlaneFixture) controlPlaneRun {
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
	for url, data := range f.reads {
		name := strings.Map(func(r rune) rune {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				return r
			}
			return '_'
		}, url)
		if err := os.WriteFile(filepath.Join(dir, "read_"+name), []byte(data), 0o600); err != nil {
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
  *)
    # Any other GET is a bare URL served from the fixture named for exactly that URL.
    name=$(printf '%s' "$1" | tr -c 'A-Za-z0-9' '_')
    if [ $# -ne 1 ] || [ ! -f "$STUB_DIR/read_$name" ]; then
      echo "unexpected $*" >>"$STUB_DIR/calls"
      exit 24
    fi
    call "$1"
    cat "$STUB_DIR/read_$name"
    [ "$FAIL_AFTER" != "$1" ] || exit 25 ;;
esac
`
	if err := execfile.WriteFile(filepath.Join(dir, "gh"), []byte(stub), 0o700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "step.sh")
	if err := os.WriteFile(script, []byte(runScript(t, jobBlock(t, readWorkflow(t, "ci-control-plane.yml"), "control-plane"))), 0o600); err != nil {
		t.Fatal(err)
	}
	fallback, number, event := "no", "1144", "pull_request_target"
	if f.fallback {
		fallback, number, event = "yes", "", "workflow_run"
	}
	cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", script)
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"STUB_DIR="+dir, "REPO=beamfall/corvint", "HEAD="+runHead, "PR_NUMBER="+number,
		"EVENT_NAME="+event, "FALLBACK="+fallback, "FAIL_AT="+f.fail, "FAIL_AFTER="+f.failAfter, "WAIVER_APPROVERS=Russe11", "GH_TOKEN=stub",
		"RUN_URL=https://github.com/beamfall/corvint/actions/runs/1")
	for key, value := range f.env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	out, err := cmd.CombinedOutput()
	posts, _ := os.ReadFile(filepath.Join(dir, "posts"))
	calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
	got := controlPlaneRun{calls: string(calls), posts: string(posts), out: string(out), failed: err != nil}
	if (f.last == "" && len(calls) != 0) || (f.last != "" && !strings.HasSuffix(string(calls), f.last+"\n")) {
		t.Errorf("calls %q: want last %s; output %s", calls, f.last, out)
	}
	if f.want == "" {
		if len(posts) != 0 {
			t.Errorf("unsafe status posted: %s", posts)
		}
		return got
	}
	if err != nil || strings.Count(string(posts), "\n") != 1 ||
		!strings.Contains(string(posts), "repos/beamfall/corvint/statuses/"+runHead+" ") ||
		!strings.Contains(string(posts), "state="+f.want+" ") || !strings.Contains(string(posts), "context=ci-control-plane ") {
		t.Fatalf("err %v, posts %q, want %s; output %s", err, posts, f.want, out)
	}
	if !strings.HasSuffix(string(calls), "after\npost\n") {
		t.Errorf("no revalidation immediately before post: %s", calls)
	}
	return got
}

// CCEP-V0-001 (V1-5263): at 3000 or more changed files the step lists no file. It reads the base
// branch's current tip, the merge base of that tip and the head, and the root tree of the tip, the
// merge base and the head, and posts success only when the `.github` entry is the same tree at all
// three or absent at all three. A difference takes the listing path's failure and waiver lookup;
// any failed, malformed or truncated read posts nothing and fails the step, waiver or not. Below
// 3000 files the call trace is the listing path's, unchanged.
func TestV15263ControlPlaneLargePRComparesGitHubTrees(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the control-plane step runs under bash with a gh spy")
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatal(err)
		}
	}
	const api = "repos/beamfall/corvint/"
	sha := func(c string) string { return strings.Repeat(c, 40) }
	// staleBase is the PR's recorded base.sha; tip is where the base branch is now.
	staleBase, tip, mergeBase := sha("a"), sha("b"), sha("c")
	baseTree, headTree, staleTree, tipTree := sha("d"), sha("e"), sha("f"), sha("6")
	ci, ci2 := sha("1"), sha("2")
	pr := func(head, base, ref, state string, count int) string {
		return fmt.Sprintf(`{"state":%q,"head":{"sha":%q},"base":{"sha":%q,"ref":%s},"changed_files":%d}`, state, head, base, ref, count)
	}
	big := pr(runHead, staleBase, `"main"`, "open", 4523)
	refURL := api + "git/ref/heads/main"
	compareURL := func(base string) string { return api + "compare/" + base + "..." + runHead + "?per_page=1" }
	commitURL := func(commit string) string { return api + "git/commits/" + commit }
	treeURL := func(tree string) string { return api + "git/trees/" + tree }
	ref := func(name, kind, commit string) string {
		return fmt.Sprintf(`{"ref":%q,"object":{"type":%q,"sha":%q}}`, name, kind, commit)
	}
	compare := func(base, merge string) string {
		return fmt.Sprintf(`{"base_commit":{"sha":%q},"merge_base_commit":{"sha":%q},"files":[{"filename":"docs/0.md"}]}`, base, merge)
	}
	commit := func(commit, tree string) string { return fmt.Sprintf(`{"sha":%q,"tree":{"sha":%q}}`, commit, tree) }
	entry := func(path, kind, object string) string {
		return fmt.Sprintf(`{"path":%q,"mode":"040000","type":%q,"sha":%q}`, path, kind, object)
	}
	tree := func(tree, truncated string, entries ...string) string {
		return fmt.Sprintf(`{"sha":%q,"truncated":%s,"tree":[%s]}`, tree, truncated, strings.Join(entries, ","))
	}
	// Lookalike entries must not count as `.github`.
	readme, nested := entry("README.md", "blob", sha("9")), entry(".github-old", "tree", sha("8"))
	dotGitHub := func(object string) string { return entry(".github", "tree", object) }
	waiver := `[{"context":"ci-control-plane-waiver","state":"success","creator":{"login":"Russe11"}}]`
	// complete is a 4,523-file PR whose `.github` tree is the same at the tip, the merge base and
	// the head.
	complete := func() controlPlaneFixture {
		return controlPlaneFixture{before: big, after: big, files: "[]", statuses: "[]", want: "success", last: "post",
			reads: map[string]string{
				refURL:               ref("refs/heads/main", "commit", tip),
				compareURL(tip):      compare(tip, mergeBase),
				commitURL(tip):       commit(tip, tipTree),
				treeURL(tipTree):     tree(tipTree, "false", dotGitHub(ci), readme),
				commitURL(mergeBase): commit(mergeBase, baseTree),
				treeURL(baseTree):    tree(baseTree, "false", readme, dotGitHub(ci), nested),
				commitURL(runHead):   commit(runHead, headTree),
				treeURL(headTree):    tree(headTree, "false", dotGitHub(ci), readme),
			}}
	}
	reads := []string{refURL, compareURL(tip), commitURL(tip), treeURL(tipTree), commitURL(mergeBase), treeURL(baseTree),
		commitURL(runHead), treeURL(headTree)}
	trace := func(steps ...string) string { return strings.Join(steps, "\n") + "\n" }
	compared := func(rest ...string) string { return trace(append(append([]string{"before"}, reads...), rest...)...) }
	// check runs f and requires the exact call trace; no trace may name the PR's stale base.sha.
	check := func(t *testing.T, f controlPlaneFixture, want string) controlPlaneRun {
		t.Helper()
		got := runControlPlaneFixture(t, f)
		if got.calls != want {
			t.Errorf("call trace\n%swant\n%soutput %s", got.calls, want, got.out)
		}
		if strings.Contains(got.calls, staleBase) {
			t.Errorf("a read used the PR's recorded base.sha: %s", got.calls)
		}
		return got
	}

	// Prove the complete fixture before changing or breaking it.
	for name, count := range map[string]int{"4523 files": 4523, "exactly 3000 files": 3000, "3001 files": 3001} {
		t.Run("equal trees, "+name, func(t *testing.T) {
			f := complete()
			f.before, f.after = pr(runHead, staleBase, `"main"`, "open", count), pr(runHead, staleBase, `"main"`, "open", count)
			got := check(t, f, compared("after", "post"))
			if !strings.Contains(got.posts, "description=PR changes nothing under .github/ ") {
				t.Errorf("posts %q: want the clean description", got.posts)
			}
		})
	}
	t.Run("equal trees on the completed-CI fallback", func(t *testing.T) {
		f := complete()
		f.fallback = true
		check(t, f, "lookup\n"+compared("after", "post"))
	})
	t.Run("all three entries equal, and the log names them", func(t *testing.T) {
		got := check(t, complete(), compared("after", "post"))
		want := "ci-control-plane: #1144 has 4523 changed files; .github is tree:" + ci + " at base tip " + tip +
			", tree:" + ci + " at merge base " + mergeBase + " and tree:" + ci + " at " + runHead + "\n"
		if !strings.Contains(got.out, want) {
			t.Errorf("output %s: want %q", got.out, want)
		}
	})
	t.Run("no .github at any of the three", func(t *testing.T) {
		f := complete()
		f.reads[treeURL(tipTree)] = tree(tipTree, "false", readme)
		f.reads[treeURL(baseTree)] = tree(baseTree, "false", readme, nested)
		f.reads[treeURL(headTree)] = tree(headTree, "false", readme)
		check(t, f, compared("after", "post"))
	})
	t.Run("base ref with a slash", func(t *testing.T) {
		f := complete()
		f.before = pr(runHead, staleBase, `"release/1.0"`, "open", 4523)
		f.after = f.before
		delete(f.reads, refURL)
		f.reads[api+"git/ref/heads/release/1.0"] = ref("refs/heads/release/1.0", "commit", tip)
		got := runControlPlaneFixture(t, f)
		if !strings.HasPrefix(got.calls, "before\n"+api+"git/ref/heads/release/1.0\n"+compareURL(tip)+"\n") {
			t.Errorf("call trace %s", got.calls)
		}
	})

	// A difference is the listing path's failure: the waiver lookup runs and decides.
	differing := map[string]func(f *controlPlaneFixture){
		"different tree SHA": func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(headTree, "false", dotGitHub(ci2), readme)
		},
		"added at the head": func(f *controlPlaneFixture) {
			f.reads[treeURL(tipTree)] = tree(tipTree, "false", readme)
			f.reads[treeURL(baseTree)] = tree(baseTree, "false", readme)
		},
		"removed at the head": func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(headTree, "false", readme)
		},
		"head entry is not a tree": func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(headTree, "false", entry(".github", "blob", ci), readme)
		},
		"merge-base entry is not a tree": func(f *controlPlaneFixture) {
			f.reads[treeURL(baseTree)] = tree(baseTree, "false", entry(".github", "commit", ci), readme)
		},
		"same non-tree entry at all three": func(f *controlPlaneFixture) {
			f.reads[treeURL(tipTree)] = tree(tipTree, "false", entry(".github", "blob", ci))
			f.reads[treeURL(baseTree)] = tree(baseTree, "false", entry(".github", "blob", ci))
			f.reads[treeURL(headTree)] = tree(headTree, "false", entry(".github", "blob", ci))
		},
		"absent at the merge base, not a tree at the head": func(f *controlPlaneFixture) {
			f.reads[treeURL(tipTree)] = tree(tipTree, "false", readme)
			f.reads[treeURL(baseTree)] = tree(baseTree, "false", readme)
			f.reads[treeURL(headTree)] = tree(headTree, "false", entry(".github", "blob", ci))
		},
		"not a tree at the merge base, absent at the head": func(f *controlPlaneFixture) {
			f.reads[treeURL(tipTree)] = tree(tipTree, "false", entry(".github", "blob", ci))
			f.reads[treeURL(baseTree)] = tree(baseTree, "false", entry(".github", "blob", ci))
			f.reads[treeURL(headTree)] = tree(headTree, "false", readme)
		},
		// The base branch moved `.github` after the merge base; the head did not take it.
		"tip differs from the merge base, head equals the merge base": func(f *controlPlaneFixture) {
			f.reads[treeURL(tipTree)] = tree(tipTree, "false", dotGitHub(ci2), readme)
		},
		"head equals the tip, both differ from the merge base": func(f *controlPlaneFixture) {
			f.reads[treeURL(tipTree)] = tree(tipTree, "false", dotGitHub(ci2), readme)
			f.reads[treeURL(headTree)] = tree(headTree, "false", dotGitHub(ci2), readme)
		},
		"absent at the tip only": func(f *controlPlaneFixture) {
			f.reads[treeURL(tipTree)] = tree(tipTree, "false", readme)
		},
	}
	for name, change := range differing {
		for _, tc := range []struct {
			name, statuses, want string
			fallback             bool
		}{
			{"no waiver", "[]", "failure", false},
			{"approver waiver", waiver, "success", false},
			{"waiver by another writer", strings.ReplaceAll(waiver, "Russe11", "lane-bot"), "failure", false},
			{"approver withdrew", strings.ReplaceAll(waiver, `"success"`, `"failure"`), "failure", false},
			{"no waiver on the completed-CI fallback", "[]", "failure", true},
			{"approver waiver on the completed-CI fallback", waiver, "success", true},
		} {
			t.Run(name+", "+tc.name, func(t *testing.T) {
				f := complete()
				change(&f)
				f.statuses, f.want, f.fallback = tc.statuses, tc.want, tc.fallback
				want := compared("statuses", "after", "post")
				if tc.fallback {
					want = "lookup\n" + want
				}
				got := check(t, f, want)
				wantText := "description=PR changes .github/; merge needs an approver ci-control-plane-waiver status on this SHA "
				if tc.statuses == waiver {
					wantText = "description=owner waiver by Russe11 on this head; PR changes .github/ "
				}
				if !strings.Contains(got.posts, wantText) {
					t.Errorf("posts %q: want %q", got.posts, wantText)
				}
			})
		}
	}

	// The comparison point is the merge base with the branch's current tip. Here the PR's recorded
	// base.sha is a stale ancestor whose `.github` equals the head's, so comparing against it
	// would report clean; main changed `.github` afterwards and the head reverted it.
	t.Run("stale base.sha agrees with the head but the tip's merge base does not", func(t *testing.T) {
		f := complete()
		f.reads[compareURL(staleBase)] = compare(staleBase, staleBase)
		f.reads[commitURL(staleBase)] = commit(staleBase, staleTree)
		f.reads[treeURL(staleTree)] = tree(staleTree, "false", dotGitHub(ci))
		f.reads[treeURL(tipTree)] = tree(tipTree, "false", dotGitHub(ci2), readme)
		f.reads[treeURL(baseTree)] = tree(baseTree, "false", dotGitHub(ci2), readme)
		f.want = "failure"
		check(t, f, compared("statuses", "after", "post"))
	})

	// Every refusal starts from the complete fixture plus an approver waiver, posts nothing, fails
	// the step, and stops at the named read.
	refusals := []struct {
		name, last string
		change     func(f *controlPlaneFixture)
	}{
		{"ref read fails", refURL, func(f *controlPlaneFixture) { f.fail = refURL }},
		{"ref answer names another ref", refURL, func(f *controlPlaneFixture) {
			f.reads[refURL] = ref("refs/heads/main-old", "commit", tip)
		}},
		{"ref answer is a prefix listing", refURL, func(f *controlPlaneFixture) {
			f.reads[refURL] = "[" + ref("refs/heads/main", "commit", tip) + "]"
		}},
		{"ref points at a tag object", refURL, func(f *controlPlaneFixture) {
			f.reads[refURL] = ref("refs/heads/main", "tag", tip)
		}},
		{"tip SHA is short", refURL, func(f *controlPlaneFixture) {
			f.reads[refURL] = ref("refs/heads/main", "commit", tip[:39])
		}},
		{"tip SHA is not lower-case hex", refURL, func(f *controlPlaneFixture) {
			f.reads[refURL] = ref("refs/heads/main", "commit", strings.ToUpper(tip))
		}},
		{"tip SHA carries a path", refURL, func(f *controlPlaneFixture) {
			f.reads[refURL] = ref("refs/heads/main", "commit", tip+"/../../pulls/1")
		}},
		{"tip SHA is missing", refURL, func(f *controlPlaneFixture) {
			f.reads[refURL] = `{"ref":"refs/heads/main","object":{"type":"commit"}}`
		}},
		{"compare read fails", compareURL(tip), func(f *controlPlaneFixture) { f.fail = compareURL(tip) }},
		{"merge base is missing", compareURL(tip), func(f *controlPlaneFixture) {
			f.reads[compareURL(tip)] = fmt.Sprintf(`{"base_commit":{"sha":%q}}`, tip)
		}},
		{"merge base is null", compareURL(tip), func(f *controlPlaneFixture) {
			f.reads[compareURL(tip)] = fmt.Sprintf(`{"base_commit":{"sha":%q},"merge_base_commit":null}`, tip)
		}},
		{"merge base SHA is short", compareURL(tip), func(f *controlPlaneFixture) {
			f.reads[compareURL(tip)] = compare(tip, mergeBase[:7])
		}},
		{"compare answers for another base", compareURL(tip), func(f *controlPlaneFixture) {
			f.reads[compareURL(tip)] = compare(staleBase, mergeBase)
		}},
		{"tip commit read fails", commitURL(tip), func(f *controlPlaneFixture) { f.fail = commitURL(tip) }},
		{"tip commit answers for another commit", commitURL(tip), func(f *controlPlaneFixture) {
			f.reads[commitURL(tip)] = commit(mergeBase, tipTree)
		}},
		{"tip tree read fails", treeURL(tipTree), func(f *controlPlaneFixture) { f.fail = treeURL(tipTree) }},
		{"tip tree is truncated", treeURL(tipTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(tipTree)] = tree(tipTree, "true", dotGitHub(ci), readme)
		}},
		{"tip tree has a null entry", treeURL(tipTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(tipTree)] = tree(tipTree, "false", dotGitHub(ci), "null")
		}},
		{"merge-base commit read fails", commitURL(mergeBase), func(f *controlPlaneFixture) { f.fail = commitURL(mergeBase) }},
		{"merge-base commit answers for another commit", commitURL(mergeBase), func(f *controlPlaneFixture) {
			f.reads[commitURL(mergeBase)] = commit(tip, baseTree)
		}},
		{"merge-base root tree SHA is not hex", commitURL(mergeBase), func(f *controlPlaneFixture) {
			f.reads[commitURL(mergeBase)] = commit(mergeBase, "main")
		}},
		{"merge-base tree read fails", treeURL(baseTree), func(f *controlPlaneFixture) { f.fail = treeURL(baseTree) }},
		{"merge-base tree is truncated", treeURL(baseTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(baseTree)] = tree(baseTree, "true", readme, dotGitHub(ci))
		}},
		{"head commit read fails", commitURL(runHead), func(f *controlPlaneFixture) { f.fail = commitURL(runHead) }},
		{"head commit has no tree", commitURL(runHead), func(f *controlPlaneFixture) {
			f.reads[commitURL(runHead)] = fmt.Sprintf(`{"sha":%q}`, runHead)
		}},
		{"head tree read fails", treeURL(headTree), func(f *controlPlaneFixture) { f.fail = treeURL(headTree) }},
		{"head tree is truncated", treeURL(headTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(headTree, "true", dotGitHub(ci), readme)
		}},
		{"head tree does not say whether it is truncated", treeURL(headTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(headTree, "null", dotGitHub(ci), readme)
		}},
		{"head tree answers for another tree", treeURL(headTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(baseTree, "false", dotGitHub(ci), readme)
		}},
		{"head tree has no entry list", treeURL(headTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = fmt.Sprintf(`{"sha":%q,"truncated":false,"tree":{}}`, headTree)
		}},
		{"head tree lists .github twice", treeURL(headTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(headTree, "false", dotGitHub(ci), dotGitHub(ci))
		}},
		{"head .github tree SHA is not hex", treeURL(headTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(headTree, "false", dotGitHub("absent"), readme)
		}},
		{"head .github tree SHA is missing", treeURL(headTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(headTree, "false", `{"path":".github","type":"tree"}`)
		}},
		{"head tree entry is not an object", treeURL(headTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(headTree, "false", `".github"`)
		}},
		{"head tree has a null entry", treeURL(headTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(headTree, "false", dotGitHub(ci), "null", readme)
		}},
		{"head tree has an entry without a path", treeURL(headTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(headTree, "false", dotGitHub(ci), `{"mode":"040000","type":"tree","sha":"`+ci2+`"}`)
		}},
		{"head tree has an entry whose path is not a string", treeURL(headTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(headTree, "false", dotGitHub(ci), `{"path":7,"type":"blob","sha":"`+ci2+`"}`)
		}},
		{"head tree is not JSON", treeURL(headTree), func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = "<html>bad gateway</html>"
		}},
		{"waiver read fails after a difference", "statuses", func(f *controlPlaneFixture) {
			f.reads[treeURL(headTree)] = tree(headTree, "false", dotGitHub(ci2))
			f.fail = "statuses"
		}},
		{"revalidation read fails", "after", func(f *controlPlaneFixture) { f.fail = "after" }},
	}
	// Each read also prints a valid body and then fails; pipefail must stop the step there.
	for _, url := range reads {
		refusals = append(refusals, struct {
			name, last string
			change     func(f *controlPlaneFixture)
		}{"valid body, then the read fails: " + url[len(api):], url, func(f *controlPlaneFixture) { f.failAfter = url }})
	}
	for _, tc := range refusals {
		for trigger, fallback := range map[string]bool{"": false, " on the completed-CI fallback": true} {
			t.Run("refused: "+tc.name+trigger, func(t *testing.T) {
				f := complete()
				f.statuses, f.want, f.last, f.fallback = waiver, "", tc.last, fallback
				tc.change(&f)
				if got := runControlPlaneFixture(t, f); !got.failed || strings.Contains(got.calls, "unexpected") {
					t.Errorf("step exit ok %v, calls %q; want a failed step stopped at %s; output %s", !got.failed, got.calls, tc.last, got.out)
				}
			})
		}
	}
	// A base ref name outside the allow-list never reaches a URL: the first PR read is the only call.
	for _, name := range []string{
		`""`, `"../../pulls/1"`, `"main/../x"`, `"main..x"`, `"main?per_page=1"`, `"main#x"`, `"main x"`, `"main%2Fx"`,
		`"-main"`, `".main"`, `"main/"`, `"/main"`, `"a//b"`, `"main\nx"`, `"$(id)"`, `"` + strings.Repeat("a", 256) + `"`,
		`null`, `7`, `["main"]`,
	} {
		t.Run("refused: base ref "+name[:min(len(name), 24)], func(t *testing.T) {
			f := complete()
			f.before = pr(runHead, staleBase, name, "open", 4523)
			f.statuses, f.want, f.last = waiver, "", "before"
			if got := runControlPlaneFixture(t, f); !got.failed || got.calls != "before\n" {
				t.Errorf("step exit ok %v, calls %q; want a failed step after the first read; output %s", !got.failed, got.calls, got.out)
			}
		})
	}
	// A PR that moved posts nothing: the event head is checked before any tree read, and the PR
	// is re-read right before the POST on the clean and on the waived path alike.
	t.Run("old event head", func(t *testing.T) {
		f := complete()
		f.before = pr(sha("7"), staleBase, `"main"`, "open", 4523)
		f.want, f.last = "", "before"
		check(t, f, "before\n")
	})
	for name, after := range map[string]string{
		"head moved":         pr(sha("7"), staleBase, `"main"`, "open", 4523),
		"base.sha moved":     pr(runHead, tip, `"main"`, "open", 4523),
		"file count changed": pr(runHead, staleBase, `"main"`, "open", 4524),
		"PR closed":          pr(runHead, staleBase, `"main"`, "closed", 4523),
	} {
		t.Run("stale at revalidation, clean: "+name, func(t *testing.T) {
			f := complete()
			f.after, f.want, f.last = after, "", "after"
			runControlPlaneFixture(t, f)
		})
		t.Run("stale at revalidation, waived: "+name, func(t *testing.T) {
			f := complete()
			f.reads[treeURL(headTree)] = tree(headTree, "false", dotGitHub(ci2))
			f.statuses, f.after, f.want, f.last = waiver, after, "", "after"
			runControlPlaneFixture(t, f)
		})
	}

	// Below the ceiling the listing path is untouched: the same calls in the same order, and no
	// tree read even though every tree fixture is on offer.
	small := func(count int, files string) controlPlaneFixture {
		f := complete()
		f.before, f.after, f.files = pr(runHead, staleBase, `"main"`, "open", count), pr(runHead, staleBase, `"main"`, "open", count), files
		return f
	}
	var pages [][]map[string]string
	for i := 0; i < 2999; i += 100 {
		var page []map[string]string
		for j := i; j < min(i+100, 2999); j++ {
			page = append(page, map[string]string{"filename": fmt.Sprintf("docs/%d.md", j)})
		}
		pages = append(pages, page)
	}
	under, err := json.Marshal(pages)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("2999 files are listed", func(t *testing.T) {
		check(t, small(2999, string(under)), trace("before", "files", "after", "post"))
	})
	t.Run("small clean PR is listed", func(t *testing.T) {
		got := check(t, small(2, `[[{"filename":"README.md"},{"filename":"docs/x.md"}]]`), trace("before", "files", "after", "post"))
		want := "-X POST repos/beamfall/corvint/statuses/" + runHead + " -f state=success -f context=ci-control-plane " +
			"-f description=PR changes nothing under .github/ -f target_url=https://github.com/beamfall/corvint/actions/runs/1\n"
		if got.posts != want {
			t.Errorf("posts %q, want %q", got.posts, want)
		}
	})
	t.Run("small .github PR is listed and waived", func(t *testing.T) {
		f := small(2, `[[{"filename":"README.md"},{"filename":".github/workflows/ci.yml"}]]`)
		f.statuses = waiver
		got := check(t, f, trace("before", "files", "statuses", "after", "post"))
		want := "-X POST repos/beamfall/corvint/statuses/" + runHead + " -f state=success -f context=ci-control-plane " +
			"-f description=owner waiver by Russe11 on this head; PR changes .github/ -f target_url=https://github.com/beamfall/corvint/actions/runs/1\n"
		if got.posts != want {
			t.Errorf("posts %q, want %q", got.posts, want)
		}
	})
	t.Run("small .github PR is listed and fails", func(t *testing.T) {
		f := small(2, `[[{"filename":"README.md"},{"filename":".github/workflows/ci.yml"}]]`)
		f.want = "failure"
		got := check(t, f, trace("before", "files", "statuses", "after", "post"))
		want := "-X POST repos/beamfall/corvint/statuses/" + runHead + " -f state=failure -f context=ci-control-plane " +
			"-f description=PR changes .github/; merge needs an approver ci-control-plane-waiver status on this SHA " +
			"-f target_url=https://github.com/beamfall/corvint/actions/runs/1\n"
		if got.posts != want {
			t.Errorf("posts %q, want %q", got.posts, want)
		}
	})
	t.Run("small PR on the completed-CI fallback is listed", func(t *testing.T) {
		f := small(2, `[[{"filename":"README.md"},{"filename":"docs/x.md"}]]`)
		f.fallback = true
		check(t, f, trace("lookup", "before", "files", "after", "post"))
	})
}
