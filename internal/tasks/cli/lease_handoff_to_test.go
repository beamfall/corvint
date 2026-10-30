package cli_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-082..084: the release flags record an advisory hand-off on the
// terminal generation; ticket show and plan preview derive nextStage from
// it, a retry carries it into the prior-generation history, and claims for
// another stage are not refused.
func TestCALV0084_CLIHandoffTargetNextStage(t *testing.T) {
	t.Run("CAL-V0-084 CLIHandoffTargetNextStage", func(t *testing.T) {
		root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		id := planTicket(t, root, "handoff-to", "P1", `["src/"]`)
		treeRaw, err := exec.Command("git", "-C", root, "rev-parse", "HEAD^{tree}").Output()
		if err != nil {
			t.Fatal(err)
		}
		tree := strings.TrimSpace(string(treeRaw))
		runOK := func(args ...string) run {
			t.Helper()
			r := handoffCLI(t, root, args...)
			if r.res.Outcome != wire.OutcomeOK {
				t.Fatalf("%v: %s", args, r.stdout)
			}
			return r
		}
		refused := func(args ...string) {
			t.Helper()
			r := handoffCLI(t, root, args...)
			if !hasCode(r.res, wire.CodeMalformed) {
				t.Fatalf("%v: expected MALFORMED: %s", args, r.stdout)
			}
		}
		nextStage := func(want string) {
			t.Helper()
			show := runOK("ticket", "show", id)
			got, ok := show.res.Items[0].Obj.Get("nextStage")
			if !ok || (want == "" && got.Kind != wire.KindNull) || (want != "" && got.Str != want) {
				t.Fatalf("ticket show nextStage want %q: %s", want, show.stdout)
			}
			plan := runOK("plan", "preview")
			entries := field(plan.res.Items[0], "entries").Arr
			if len(entries) != 1 {
				t.Fatalf("plan: %s", plan.stdout)
			}
			got, ok = entries[0].Obj.Get("nextStage")
			if !ok || (want == "" && got.Kind != wire.KindNull) || (want != "" && got.Str != want) {
				t.Fatalf("plan preview nextStage want %q: %s", want, plan.stdout)
			}
			obs, err := cli.ObserveDispatch(root)
			if err != nil || len(obs.Tickets) != 1 {
				t.Fatalf("dispatch observation: %v", err)
			}
			if dispatched := obs.Tickets[0].NextStage; dispatched != want && (want != "" || dispatched != "NONE") {
				t.Fatalf("dispatcher nextStage %q want %q", dispatched, want)
			}
		}
		nextStage("")
		refused("claim", id, "--holder", "w", "--stage", "implement", "--handoff-to", "review", "--request-id", "claim-flag")
		c := runOK("claim", id, "--holder", "w0", "--stage", "implement", "--request-id", "claim-0")
		a, g := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
		nextStage("")
		runOK("submit", "--attempt", a, "--generation", g, "--tree", tree, "--request-id", "submit-0")
		release := []string{"release", "--attempt", a, "--generation", g}
		refused(append(release, "--reason", wire.CodeContaminated, "--handoff-to", "review", "--request-id", "r-dirty")...)
		refused(append(release, "--handoff-to", "review", "--request-id", "r-none")...)
		refused(append(release, "--reason", wire.CodeHandoff, "--handoff-to", "deploy", "--request-id", "r-stage")...)
		refused(append(release, "--reason", wire.CodeHandoff, "--handoff-reason", "STAGE_COMPLETE", "--request-id", "r-reason")...)
		refused(append(release, "--reason", wire.CodeHandoff, "--handoff-to", "implement", "--handoff-reason", "CHANGES_REQUESTED", "--request-id", "r-changes")...)
		refused(append(release, "--reason", wire.CodeHandoff, "--handoff-to", "", "--request-id", "r-empty")...)
		if field(runOK("attempt", "show", a).res.Items[0], "phase").Str == "CANCELLED" {
			t.Fatal("a refused hand-off ended the generation")
		}
		runOK(append(release, "--reason", wire.CodeHandoff, "--handoff-to", "review", "--handoff-reason", "STAGE_COMPLETE", "--request-id", "release-0")...)
		ended := runOK("attempt", "show", a).res.Items[0]
		if field(ended, "handoffTo").Str != "review" || field(ended, "handoffReason").Str != "STAGE_COMPLETE" {
			t.Fatalf("recorded hand-off: %s", wire.Encode(ended))
		}
		nextStage("review")

		// Advisory: an implement claim after a review hand-off is admitted.
		c = runOK("claim", id, "--holder", "w1", "--stage", "implement", "--request-id", "claim-1")
		a, g = field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
		show := runOK("attempt", "show", a).res.Items[0]
		prior := field(show, "priorGenerations").Arr
		if len(prior) != 1 || field(prior[0], "handoffTo").Str != "review" || field(prior[0], "handoffReason").Str != "STAGE_COMPLETE" || field(show, "retryCount").Str != "0" {
			t.Fatalf("retry history: %s", wire.Encode(show))
		}
		nextStage("")
		runOK("submit", "--attempt", a, "--generation", g, "--tree", tree, "--request-id", "submit-1")
		runOK("release", "--attempt", a, "--generation", g, "--reason", wire.CodeHandoff, "--request-id", "release-1")
		nextStage("")

		c = runOK("claim", id, "--holder", "w2", "--stage", "review", "--request-id", "claim-2")
		a, g = field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
		runOK("submit", "--attempt", a, "--generation", g, "--tree", tree, "--request-id", "submit-2")
		refused("release", "--attempt", a, "--generation", g, "--reason", wire.CodeReviewReturned, "--handoff-to", "review", "--request-id", "r-review")
		runOK("release", "--attempt", a, "--generation", g, "--reason", wire.CodeReviewReturned, "--request-id", "release-2")
		if _, ok := runOK("attempt", "show", a).res.Items[0].Obj.Get("handoffTo"); ok {
			t.Fatal("the REVIEW_RETURNED default was written")
		}
		nextStage("implement")
		if field(runOK("attempt", "show", a).res.Items[0], "retryCount").Str != "0" {
			t.Fatal("clean hand-offs charged a retry")
		}
		runOK("receipt", "audit")
	})
}

// CAL-V0-083: release help lists the closed hand-off targets and reasons and
// the flags that carry them.
func TestCALV0083_ReleaseHelpListsHandoffCodes(t *testing.T) {
	t.Run("CAL-V0-083 ReleaseHelpListsHandoffCodes", func(t *testing.T) {
		var out bytes.Buffer
		code := cli.Run(cli.Env{Cwd: t.TempDir(), Args: []string{"release", "--help", "--verbose"}, Stdin: unreadHelpInput{}, Stdout: &out})
		r, e := wire.DecodeResult(out.Bytes())
		if e != nil || code != 0 {
			t.Fatalf("release help: %s (%v)", out.Bytes(), e)
		}
		join := func(key string) string {
			var s []string
			for _, v := range field(r.Items[0], key).Arr {
				s = append(s, v.Str)
			}
			return strings.Join(s, ",")
		}
		if join("handoffTargets") != "implement,review,integrate" || join("handoffReasonCodes") != "CHANGES_REQUESTED,STAGE_COMPLETE,STAGE_INCOMPLETE" {
			t.Fatalf("hand-off codes: %s", out.Bytes())
		}
		if flags := join("flags"); !strings.Contains(flags, "--handoff-to") || !strings.Contains(flags, "--handoff-reason") {
			t.Fatalf("flags: %s", flags)
		}
	})
}
