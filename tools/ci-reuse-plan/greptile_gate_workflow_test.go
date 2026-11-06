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

// GRG-V0-001: the gate job of .github/workflows/greptile-gate.yml posts `greptile-gate` success
// on an open PR head only when Greptile's newest summary reviewed that exact head at 5/5 and no
// Greptile review thread is unresolved; it keeps a user-posted success, posts pending while the
// head is unreviewed, failure otherwise, and posts nothing for a closed PR or a failed read.
func TestGRGV0001GreptileGatePostsSuccessOnlyFor5Of5OnHead(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the gate step runs under bash with a #!/bin/sh gh spy")
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("the gate step needs %s on PATH: %v", tool, err)
		}
	}
	const head = "0c28ca2a4b1e2f3a4b5c6d7e8f90a1b2c3d4e5f6"
	const old = "8f05365360a1b2c3d4e5f60718293a4b5c6d7e8f"
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
	}
	// run returns the status POSTs of one step run and whether the step exited zero. An empty
	// field makes that read fail.
	run := func(in input) (string, bool) {
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
		cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", script)
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"STUB_DIR="+dir, "REPO=beamfall/corvint", "PR=112",
			"RUN_URL=https://github.com/beamfall/corvint/actions/runs/1", "GH_TOKEN=stub")
		err := cmd.Run()
		posts, _ := os.ReadFile(filepath.Join(dir, "posts.log"))
		return string(posts), err == nil
	}
	userSuccess := `[{"state":"success","context":"greptile-gate","creator":{"type":"User"}},{"state":"failure","context":"greptile-gate","creator":{"type":"Bot"}}]`
	clean := threads(2, thread(true, "greptile-apps"), thread(false, "Russe11"))
	pass := "[" + summary("5/5", head) + "," + other + "]"
	for name, tc := range map[string]struct {
		in    input
		state string // "" = no post
		ok    bool
	}{
		"5/5 on head, threads resolved": {input{"open", "[]", pass, clean}, "success", true},
		"5/5 on an older head":          {input{"open", "[]", "[" + summary("5/5", old) + "]", clean}, "pending", true},
		"no Greptile summary":           {input{"open", "[]", "[" + other + "]", clean}, "pending", true},
		"4/5 on head":                   {input{"open", "[]", "[" + summary("4/5", head) + "]", clean}, "failure", true},
		"unresolved Greptile thread":    {input{"open", "[]", pass, threads(1, thread(false, "greptile-apps"))}, "failure", true},
		"over 100 threads":              {input{"open", "[]", pass, threads(101)}, "failure", true},
		"newer summary wins":            {input{"open", "[]", "[" + strings.Replace(summary("5/5", head), "10:00", "09:00", 1) + "," + summary("3/5", head) + "]", clean}, "failure", true},
		"bot success is re-evaluated":   {input{"open", `[{"state":"success","context":"greptile-gate","creator":{"type":"Bot"}}]`, "[" + summary("4/5", head) + "]", clean}, "failure", true},
		"user success kept":             {input{"open", userSuccess, "[" + summary("4/5", head) + "]", clean}, "", true},
		"closed PR":                     {input{"closed", "[]", pass, clean}, "", true},
		"unreadable PR":                 {input{"", "[]", pass, clean}, "", false},
		"unreadable statuses":           {input{"open", "", pass, clean}, "", false},
		"unreadable comments":           {input{"open", "[]", "", clean}, "", false},
		"unreadable threads":            {input{"open", "[]", pass, ""}, "", false},
	} {
		posts, ok := run(tc.in)
		if ok != tc.ok {
			t.Errorf("%s: exit ok %v, want %v (posts %q)", name, ok, tc.ok, posts)
		}
		if tc.state == "" {
			if posts != "" {
				t.Errorf("%s: posted %q, want nothing", name, posts)
			}
			continue
		}
		if strings.Count(posts, "\n") != 1 || !strings.Contains(posts, "repos/beamfall/corvint/statuses/"+head+" ") ||
			!strings.Contains(posts, "state="+tc.state+" ") || !strings.Contains(posts, "context=greptile-gate") {
			t.Errorf("%s: posted %q, want one %s greptile-gate status on %s", name, posts, tc.state, head)
		}
	}
}
