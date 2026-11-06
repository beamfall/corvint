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
// leaves pending after a failed later read. On merge_group it re-evaluates the queued PR named by
// the queue ref and posts the result on the merge-group commit.
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
	const groupRef = "refs/heads/gh-readonly-queue/main/pr-112-" + old
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
		groupRef                           string // non-empty: a merge_group event
	}
	// run returns the status POSTs of one step run and whether the step exited zero. An empty
	// field makes that read fail.
	run := func(in input) ([]string, bool) {
		dir := t.TempDir()
		bin := filepath.Join(dir, "bin")
		if err := os.Mkdir(bin, 0o700); err != nil {
			t.Fatal(err)
		}
		files := map[string]string{
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
  *" graphql "*|graphql\ *) f=threads.json ;;
  *"repos/$REPO/pulls/$PR "*) f=pull.json ;;
  *"repos/$REPO/commits/"*"/statuses?per_page=100 "*) f=statuses.json ;;
  *" --paginate repos/$REPO/issues/$PR/comments?per_page=100 "*) f=comments.json ;;
  *) exit 22 ;;
esac
[ -f "$STUB_DIR/$f" ] || exit 22
exec jq -r "$last" "$STUB_DIR/$f"
`
		if err := execfile.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o700); err != nil {
			t.Fatal(err)
		}
		event, pr, sha := "pull_request_target", "112", ""
		if in.groupRef != "" {
			event, pr, sha = "merge_group", "", group
		}
		cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", script)
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"STUB_DIR="+dir, "REPO=beamfall/corvint", "EVENT="+event, "PR="+pr,
			"GROUP_REF="+in.groupRef, "GROUP_SHA="+sha,
			"RUN_URL=https://github.com/beamfall/corvint/actions/runs/1", "GH_TOKEN=stub")
		err := cmd.Run()
		posts, _ := os.ReadFile(filepath.Join(dir, "posts.log"))
		return strings.Split(strings.TrimSuffix(string(posts), "\n"), "\n")[:strings.Count(string(posts), "\n")], err == nil
	}
	userSuccess := `[{"state":"success","context":"greptile-gate","creator":{"type":"User"}},{"state":"failure","context":"greptile-gate","creator":{"type":"Bot"}}]`
	clean := threads(2, thread(true, "greptile-apps"), thread(false, "Russe11"))
	pass := "[" + summary("5/5", head) + "," + other + "]"
	for name, tc := range map[string]struct {
		in     input
		state  string // final status; "" = no post
		target string // commit the statuses land on
		ok     bool
	}{
		"5/5 on head, threads resolved": {input{"open", "[]", pass, clean, ""}, "success", head, true},
		"5/5 on an older head":          {input{"open", "[]", "[" + summary("5/5", old) + "]", clean, ""}, "pending", head, true},
		"no Greptile summary":           {input{"open", "[]", "[" + other + "]", clean, ""}, "pending", head, true},
		"4/5 on head":                   {input{"open", "[]", "[" + summary("4/5", head) + "]", clean, ""}, "failure", head, true},
		"unresolved Greptile thread":    {input{"open", "[]", pass, threads(1, thread(false, "greptile-apps")), ""}, "failure", head, true},
		"over 100 threads":              {input{"open", "[]", pass, threads(101), ""}, "failure", head, true},
		"newer summary wins":            {input{"open", "[]", "[" + strings.Replace(summary("5/5", head), "10:00", "09:00", 1) + "," + summary("3/5", head) + "]", clean, ""}, "failure", head, true},
		"bot success is re-evaluated":   {input{"open", `[{"state":"success","context":"greptile-gate","creator":{"type":"Bot"}}]`, "[" + summary("4/5", head) + "]", clean, ""}, "failure", head, true},
		"user success kept":             {input{"open", userSuccess, "[" + summary("4/5", head) + "]", clean, ""}, "", head, true},
		"closed PR":                     {input{"closed", "[]", pass, clean, ""}, "", head, true},
		"unreadable PR":                 {input{"", "[]", pass, clean, ""}, "", head, false},
		"unreadable statuses":           {input{"open", "", pass, clean, ""}, "pending", head, false},
		"unreadable comments":           {input{"open", "[]", "", clean, ""}, "pending", head, false},
		"unreadable threads":            {input{"open", "[]", pass, "", ""}, "pending", head, false},
		"queue: 5/5 on PR head":         {input{"open", "[]", pass, clean, groupRef}, "success", group, true},
		"queue: thread reopened":        {input{"open", "[]", pass, threads(1, thread(false, "greptile-apps")), groupRef}, "failure", group, true},
		"queue: user waiver":            {input{"open", userSuccess, "[" + summary("4/5", head) + "]", clean, groupRef}, "success", group, true},
		"queue: unreadable threads":     {input{"open", "[]", pass, "", groupRef}, "pending", group, false},
		"queue: bad ref":                {input{"open", "[]", pass, clean, "refs/heads/main"}, "", group, false},
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
		for _, p := range posts {
			if !strings.Contains(p, "repos/beamfall/corvint/statuses/"+tc.target+" ") || !strings.Contains(p, "context=greptile-gate") {
				t.Errorf("%s: posted %q, want greptile-gate statuses on %s", name, p, tc.target)
			}
		}
		// A refresh withdraws any earlier success first, so only the last post may be success.
		for i, p := range posts {
			if i < len(posts)-1 && !strings.Contains(p, "state=pending ") {
				t.Errorf("%s: post %d is %q, want pending before the final status", name, i, p)
			}
		}
		if len(posts) == 0 || !strings.Contains(posts[len(posts)-1], "state="+tc.state+" ") {
			t.Errorf("%s: posted %q, want a final %s status", name, posts, tc.state)
		}
	}
}
