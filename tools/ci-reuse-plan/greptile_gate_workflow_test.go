package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testplatform"
)

// GRG-V0-001: the gate job of .github/workflows/greptile-gate.yml ends on `greptile-gate` success
// for an open PR head only when Greptile's newest summary reviewed that exact head at 5/5 and no
// Greptile review thread is unresolved; it keeps a user-posted success, ends pending while the
// head is unreviewed and failure otherwise, posts nothing for a closed or unreadable PR, and
// leaves pending after a failed later read. On merge_group it re-evaluates the queued PR and posts
// the result on the PR head and the merge-group commit; any other run also posts on the PR's
// active merge-group commit, and an owner waiver is found past the first page of statuses.
func TestGRGV0001GreptileGatePostsSuccessOnlyFor5Of5OnHead(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the gate step runs under bash with a #!/bin/sh gh spy")
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("the gate step needs %s on PATH: %v", tool, err)
		}
	}
	const head = "0c28ca2a4b1e2f3a4b5c6d7e8f90a1b2c3d4e5f6"
	const old = "8f05365360a1b2c3d4e5f60718293a4b5c6d7e8f"
	const group = "6d1e0a7b2c3d4e5f60718293a4b5c6d7e8f90a1b"
	script := filepath.Join(t.TempDir(), "step.sh")
	if err := os.WriteFile(script, []byte(runScript(t, jobBlock(t, readWorkflow(t, "greptile-gate.yml"), "gate"))), 0o600); err != nil {
		t.Fatal(err)
	}
	summary := func(score, sha string) string {
		body := "<h3>Greptile Summary</h3>\\n\\n<h3>Confidence Score: " + score + "</h3>\\n\\nReviews (1) · Last reviewed commit: [\\\"chore: seal (x)\\\"](https://github.com/beamfall/corvint/commit/" + sha + ") · Reviewed by Greptile"
		return `{"user":{"login":"greptile-apps[bot]"},"updated_at":"2026-11-06T10:00:00Z","body":"` + body + `"}`
	}
	other := `{"user":{"login":"Russe11"},"updated_at":"2026-11-06T11:00:00Z","body":"Confidence Score: 5/5 /commit/` + head + `"}`
	thread := func(resolved bool, author string) string {
		r := "false"
		if resolved {
			r = "true"
		}
		return `{"isResolved":` + r + `,"comments":{"nodes":[{"author":{"login":"` + author + `"}}]}}`
	}
	threads := func(total int, nodes ...string) string {
		return `{"data":{"repository":{"pullRequest":{"reviewThreads":{"totalCount":` + strconv.Itoa(total) + `,"nodes":[` + strings.Join(nodes, ",") + `]}}}}}`
	}
	type input struct {
		state, statuses, comments, threads string
		mergeGroup                         bool   // a merge_group event
		queue                              string // merge-queue entry read; "" = not queued, "fail" = read fails
	}
	// run returns the status POSTs of one step run and whether the step exited zero. An empty
	// field makes that read fail.
	run := func(in input) ([]string, bool) {
		dir := t.TempDir()
		bin := filepath.Join(dir, "bin")
		if err := os.Mkdir(bin, 0o700); err != nil {
			t.Fatal(err)
		}
		queue := `{"data":{"repository":{"pullRequest":{"mergeQueueEntry":null}}}}`
		switch in.queue {
		case "":
		case "fail":
			queue = ""
		default:
			queue = `{"data":{"repository":{"pullRequest":{"mergeQueueEntry":{"headCommit":{"oid":"` + in.queue + `"}}}}}}`
		}
		files := map[string]string{
			"queue.json":    queue,
			"pull.json":     `{"state":"` + in.state + `","head":{"sha":"` + head + `"}}`,
			"statuses.json": in.statuses,
			"comments.json": in.comments,
			"threads.json":  in.threads,
		}
		for name, body := range files {
			if body == "" || (name == "pull.json" && in.state == "") {
				continue
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		// A stub gh: each read applies the step's own --jq (the last argument) to its fixture.
		stub := `#!/bin/sh
[ "$1" = api ] || exit 2
shift
for a; do last=$a; done
case " $* " in
  *" -X POST "*) echo "$*" >>"$STUB_DIR/posts.log"; exit 0 ;;
  *mergeQueueEntry*) f=queue.json ;;
  *" graphql "*|graphql\ *) f=threads.json ;;
  *"repos/$REPO/pulls/$PR "*) f=pull.json ;;
  *"repos/$REPO/commits/"*"/statuses?per_page=100 "*) f=statuses.json ;;
  *" --paginate repos/$REPO/issues/$PR/comments?per_page=100 "*) f=comments.json ;;
  *) exit 22 ;;
esac
[ -f "$STUB_DIR/$f" ] || exit 22
# A fixture holds one JSON document per page; without --paginate gh returns only the first.
case " $* " in
  *" --paginate "*) exec jq -r "$last" "$STUB_DIR/$f" ;;
  *) jq -cn input "$STUB_DIR/$f" | jq -r "$last" ;;
esac
`
		if err := execfile.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o700); err != nil {
			t.Fatal(err)
		}
		event, sha := "pull_request_target", ""
		if in.mergeGroup {
			event, sha = "merge_group", group
		}
		cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", script)
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"STUB_DIR="+dir, "REPO=beamfall/corvint", "EVENT="+event, "PR=112", "GROUP_SHA="+sha,
			"RUN_URL=https://github.com/beamfall/corvint/actions/runs/1", "GH_TOKEN=stub")
		err := cmd.Run()
		posts, _ := os.ReadFile(filepath.Join(dir, "posts.log"))
		return strings.Split(strings.TrimSuffix(string(posts), "\n"), "\n")[:strings.Count(string(posts), "\n")], err == nil
	}
	userSuccess := `[{"state":"success","context":"greptile-gate","creator":{"type":"User"}},{"state":"failure","context":"greptile-gate","creator":{"type":"Bot"}}]`
	// A waiver behind more than 100 newer statuses of other contexts, on the second page.
	var page1 []string
	for i := 0; i < 100; i++ {
		page1 = append(page1, `{"state":"success","context":"go-product","creator":{"type":"Bot"}}`)
	}
	deepWaiver := "[" + strings.Join(page1, ",") + "]\n" + userSuccess
	clean := threads(2, thread(true, "greptile-apps"), thread(false, "Russe11"))
	pass := "[" + summary("5/5", head) + "," + other + "]"
	for name, tc := range map[string]struct {
		in     input
		state  string   // final status; "" = no post
		target []string // commits the statuses land on, each ending on state
		ok     bool
	}{
		"5/5 on head, threads resolved": {input{"open", "[]", pass, clean, false, ""}, "success", []string{head}, true},
		"5/5 on an older head":          {input{"open", "[]", "[" + summary("5/5", old) + "]", clean, false, ""}, "pending", []string{head}, true},
		"no Greptile summary":           {input{"open", "[]", "[" + other + "]", clean, false, ""}, "pending", []string{head}, true},
		"4/5 on head":                   {input{"open", "[]", "[" + summary("4/5", head) + "]", clean, false, ""}, "failure", []string{head}, true},
		"unresolved Greptile thread":    {input{"open", "[]", pass, threads(1, thread(false, "greptile-apps")), false, ""}, "failure", []string{head}, true},
		"over 100 threads":              {input{"open", "[]", pass, threads(101), false, ""}, "failure", []string{head}, true},
		"newer summary wins":            {input{"open", "[]", "[" + strings.Replace(summary("5/5", head), "10:00", "09:00", 1) + "," + summary("3/5", head) + "]", clean, false, ""}, "failure", []string{head}, true},
		"bot success is re-evaluated":   {input{"open", `[{"state":"success","context":"greptile-gate","creator":{"type":"Bot"}}]`, "[" + summary("4/5", head) + "]", clean, false, ""}, "failure", []string{head}, true},
		"user success kept":             {input{"open", userSuccess, "[" + summary("4/5", head) + "]", clean, false, ""}, "", []string{head}, true},
		"closed PR":                     {input{"closed", "[]", pass, clean, false, ""}, "", []string{head}, true},
		"unreadable PR":                 {input{"", "[]", pass, clean, false, ""}, "", []string{head}, false},
		"unreadable statuses":           {input{"open", "", pass, clean, false, ""}, "pending", []string{head}, false},
		"unreadable comments":           {input{"open", "[]", "", clean, false, ""}, "pending", []string{head}, false},
		"unreadable threads":            {input{"open", "[]", pass, "", false, ""}, "pending", []string{head}, false},
		"queue: 5/5 on PR head":         {input{"open", "[]", pass, clean, true, ""}, "success", []string{head, group}, true},
		"queue: thread reopened":        {input{"open", "[]", pass, threads(1, thread(false, "greptile-apps")), true, ""}, "failure", []string{head, group}, true},
		"queue: user waiver":            {input{"open", userSuccess, "[" + summary("4/5", head) + "]", clean, true, ""}, "success", []string{group}, true},
		"queue: unreadable threads":     {input{"open", "[]", pass, "", true, ""}, "pending", []string{head, group}, false},
		"queued: lowered score":         {input{"open", "[]", "[" + summary("4/5", head) + "]", clean, false, group}, "failure", []string{head, group}, true},
		"queued: 5/5 refreshes both":    {input{"open", "[]", pass, clean, false, group}, "success", []string{head, group}, true},
		"queued: waiver on queue":       {input{"open", userSuccess, "[" + summary("4/5", head) + "]", clean, false, group}, "success", []string{group}, true},
		"unreadable queue entry":        {input{"open", "[]", pass, clean, false, "fail"}, "pending", []string{head}, false},
		"waiver past 100 statuses":      {input{"open", deepWaiver, "[" + summary("4/5", head) + "]", clean, false, ""}, "", []string{head}, true},
	} {
		posts, ok := run(tc.in)
		if ok != tc.ok {
			t.Errorf("%s: exit ok %v, want %v (posts %q)", name, ok, tc.ok, posts)
		}
		if tc.state == "" {
			if len(posts) != 0 {
				t.Errorf("%s: posted %q, want nothing", name, posts)
			}
			continue
		}
		byTarget := map[string][]string{}
		for _, p := range posts {
			matched := false
			for _, target := range tc.target {
				if strings.Contains(p, "repos/beamfall/corvint/statuses/"+target+" ") && strings.Contains(p, "context=greptile-gate") {
					byTarget[target] = append(byTarget[target], p)
					matched = true
				}
			}
			if !matched {
				t.Errorf("%s: posted %q, want greptile-gate statuses on %v", name, p, tc.target)
			}
		}
		// A refresh withdraws any earlier success first, so only each target's last post may be success.
		for _, target := range tc.target {
			got := byTarget[target]
			for i, p := range got {
				if i < len(got)-1 && !strings.Contains(p, "state=pending ") {
					t.Errorf("%s: post %d on %s is %q, want pending before the final status", name, i, target, p)
				}
			}
			if len(got) == 0 || !strings.Contains(got[len(got)-1], "state="+tc.state+" ") {
				t.Errorf("%s: posted %q on %s, want a final %s status", name, got, target, tc.state)
			}
		}
	}
}

// GRG-V0-001: every gate run for one PR shares one concurrency group keyed by the PR number the
// resolve job names (from the queue ref on merge_group) and never cancels a running job, so an
// older run cannot post a stale pass after a newer run's result.
func TestGRGV0001GreptileGateSerializesRunsPerPR(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the resolve step runs under bash")
	w := readWorkflow(t, "greptile-gate.yml")
	gate := jobBlock(t, w, "gate")
	for _, line := range []string{
		"    needs: resolve",
		"      group: greptile-gate-pr-${{ needs.resolve.outputs.pr }}",
		"      cancel-in-progress: false",
		"          PR: ${{ needs.resolve.outputs.pr }}",
	} {
		wantLine(t, "gate job", gate, line)
	}
	if strings.Count(w, "concurrency:") != 1 {
		t.Errorf("want one concurrency block, on the gate job")
	}
	script := filepath.Join(t.TempDir(), "resolve.sh")
	if err := os.WriteFile(script, []byte(runScript(t, jobBlock(t, w, "resolve"))), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		event, pr, ref, want string // want "" = the step fails
	}{
		"pull request":           {"pull_request_target", "112", "", "112"},
		"queue ref":              {"merge_group", "", "refs/heads/gh-readonly-queue/main/pr-112-8f05365360a1b2c3d4e5f60718293a4b5c6d7e8f", "112"},
		"queue bad ref":          {"merge_group", "", "refs/heads/main", ""},
		"queue ignores a PR env": {"merge_group", "7", "refs/heads/main", ""},
		"missing number":         {"workflow_dispatch", "", "", ""},
		"non-numeric":            {"workflow_dispatch", "12; x", "", ""},
	} {
		out := filepath.Join(t.TempDir(), "out")
		cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", script)
		cmd.Env = append(os.Environ(), "EVENT="+tc.event, "PR="+tc.pr, "GROUP_REF="+tc.ref, "GITHUB_OUTPUT="+out)
		err := cmd.Run()
		got, _ := os.ReadFile(out)
		if tc.want == "" {
			if err == nil || len(got) != 0 {
				t.Errorf("%s: exit err %v, output %q; want a failure with no output", name, err, got)
			}
			continue
		}
		if err != nil || string(got) != "pr="+tc.want+"\n" {
			t.Errorf("%s: exit err %v, output %q; want pr=%s", name, err, got, tc.want)
		}
	}
}
