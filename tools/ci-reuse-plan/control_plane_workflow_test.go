package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testplatform"
)

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
  *"repos/$REPO/pulls/1144/files?per_page=100"*) printf 'README.md\n.github/workflows/ci.yml\n' ;;
  *"repos/$REPO/pulls/1144 "*) echo 2 ;;
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
