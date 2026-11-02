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

// AFP-V0-171 (V1-1847): when CI reruns on a pull-request head that changes .github/, the
// control-plane job keeps a ci-control-plane success a user (the admin review) posted on that
// head: it posts nothing. A bot-posted success, a newer failure, another context, or no status
// at all (a new push) still gets failure, and an unreadable status listing posts nothing.
func TestAFPV0171ControlPlaneKeepsAUserPostedSuccess(t *testing.T) {
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
	status := func(state, creator, context string) string {
		return `{"state":"` + state + `","context":"` + context + `","creator":{"login":"x","type":"` + creator + `"}}`
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
  *"repos/$REPO/commits/$HEAD/statuses?per_page=100 --jq "*)
    [ -f "$STUB_DIR/statuses.json" ] || exit 22
    exec jq -r "$3" "$STUB_DIR/statuses.json" ;;
  *) exit 22 ;;
esac
`
		if err := execfile.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o700); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", script)
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"STUB_DIR="+dir, "REPO=beamfall/corvint", "HEAD="+head,
			"RUN_URL=https://github.com/beamfall/corvint/actions/runs/1", "GH_TOKEN=stub")
		err := cmd.Run()
		posts, _ := os.ReadFile(filepath.Join(dir, "posts.log"))
		return string(posts), err == nil
	}
	if posts, ok := run("[" + status("success", "User", "ci-control-plane") + "," + status("failure", "Bot", "ci-control-plane") + "]"); !ok || posts != "" {
		t.Errorf("a rerun after the admin-posted success: exit ok %v, posted %q, want nothing", ok, posts)
	}
	for name, statuses := range map[string]string{
		"new push, no status":     "[]",
		"bot success":             "[" + status("success", "Bot", "ci-control-plane") + "]",
		"newer failure":           "[" + status("failure", "Bot", "ci-control-plane") + "," + status("success", "User", "ci-control-plane") + "]",
		"user success, other ctx": "[" + status("success", "User", "go-product") + "," + status("failure", "Bot", "ci-control-plane") + "]",
		"user pending":            "[" + status("pending", "User", "ci-control-plane") + "]",
	} {
		posts, ok := run(statuses)
		if !ok || strings.Count(posts, "\n") != 1 || !strings.Contains(posts, "repos/beamfall/corvint/statuses/"+head) ||
			!strings.Contains(posts, "state=failure") || !strings.Contains(posts, "context=ci-control-plane") {
			t.Errorf("%s: exit ok %v, posted %q, want one ci-control-plane failure", name, ok, posts)
		}
	}
	if posts, ok := run(""); ok || posts != "" {
		t.Errorf("an unreadable status listing: exit ok %v, posted %q, want a failed step that posts nothing", ok, posts)
	}
}
