package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0743_CreateRecordsSourceRefForAbsentPaths pins V1-3696: a create
// whose touch path is absent from origin/main is refused with nothing
// written unless the body records the branch and commit that hold it or
// --creates-paths declares that the ticket creates it. A recorded commit in
// this clone is checked; one outside it is recorded with NOT_OBSERVED; a
// replay skips the check; a repository with no origin ref warns.
func TestTCLV0743_CreateRecordsSourceRefForAbsentPaths(t *testing.T) {
	r := fixture.TempRepo(t)
	git(t, r.Root, "init", "-q", "-b", "main")
	commit := func(msg string) string {
		git(t, r.Root, "add", "-A", "--", "present")
		git(t, r.Root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", msg)
		return strings.TrimSpace(gitOutput(t, r.Root, "rev-parse", "HEAD"))
	}
	fixture.Write(t, filepath.Join(r.Root, "present", "a.go"), []byte("package present\n"))
	git(t, r.Root, "update-ref", "refs/remotes/origin/main", commit("base"))
	fixture.Write(t, filepath.Join(r.Root, "lane", "x.go"), []byte("package lane\n"))
	git(t, r.Root, "add", "--", "lane")
	laneSha := commit("lane-only code")
	git(t, r.Root, "branch", "lane-x", laneSha)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %s", x.stdout)
	}
	n := 0
	create := func(body string, paths string, extra ...string) run {
		n++
		p := strings.NewReplacer(`"title":"Console ticket"`, `"title":"subject `+string(rune('a'+n))+`"`, `"body":null`, `"body":`+body, `"touchPaths":[]`, `"touchPaths":[`+paths+`]`).Replace(createPayloadJSON)
		args := append([]string{"ticket", "create", "--request-id", "src-" + string(rune('a'+n)), "--issued-at", "2026-11-01T12:00:00Z", "--payload", p}, extra...)
		return atm(t, r.Root, nil, args...)
	}
	warned := func(x run, s string) bool { return strings.Contains(strings.Join(x.res.Warnings, " "), s) }
	count := func() int { return len(atm(t, r.Root, nil, "ticket", "list").res.Items) }

	if x := create(`null`, `"present/"`); x.res.Outcome != wire.OutcomeOK || warned(x, "TCL-V0-743") {
		t.Fatalf("a present path was not accepted silently: %s", x.stdout)
	}
	before := count()
	x := create(`"lane code"`, `"lane/x.go","present/"`)
	if x.res.Outcome == wire.OutcomeOK || len(x.res.Codes) == 0 || x.res.Codes[0] != wire.CodeMalformed || !warned(x, "lane/x.go are absent from the base origin/main") ||
		warned(x, "present/ ") || !warned(x, "Source ref: BRANCH@FULL_SHA") || !warned(x, "--creates-paths") || count() != before {
		t.Fatalf("an absent path without a source ref was not refused with nothing written: %s", x.stdout)
	}
	x = create(`"lane code\nSource ref: lane-x@`+laneSha+`"`, `"lane/x.go"`)
	if x.res.Outcome != wire.OutcomeOK || !warned(x, "present at the recorded source ref lane-x@"+laneSha) {
		t.Fatalf("a recorded source ref holding the path was not accepted: %s", x.stdout)
	}
	if replay := atm(t, r.Root, nil, "ticket", "create", "--request-id", "src-"+string(rune('a'+n)), "--issued-at", "2026-11-01T12:00:00Z", "--payload",
		strings.NewReplacer(`"title":"Console ticket"`, `"title":"subject `+string(rune('a'+n))+`"`, `"body":null`, `"body":"lane code\nSource ref: lane-x@`+laneSha+`"`, `"touchPaths":[]`, `"touchPaths":["lane/x.go"]`).Replace(createPayloadJSON)); replay.res.Outcome != wire.OutcomeOK || warned(replay, "TCL-V0-743") {
		t.Fatalf("a replay ran the subject check: %s", replay.stdout)
	}
	base := strings.TrimSpace(gitOutput(t, r.Root, "rev-parse", "refs/remotes/origin/main"))
	if x := create(`"Source ref: main@`+base+`"`, `"lane/y.go"`); x.res.Outcome == wire.OutcomeOK || !warned(x, "absent from both the base") {
		t.Fatalf("a source ref lacking the path was accepted: %s", x.stdout)
	}
	if x := create(`"Source ref: mac-only@`+strings.Repeat("ab", 20)+`"`, `"lane/z.go"`); x.res.Outcome != wire.OutcomeOK || !warned(x, "is not in this clone") || warned(x, "present at the recorded source ref") {
		t.Fatalf("a source ref outside this clone was not recorded with NOT_OBSERVED: %s", x.stdout)
	}
	if x := create(`null`, `"new/pkg/"`, "--creates-paths"); x.res.Outcome != wire.OutcomeOK || warned(x, "TCL-V0-743") {
		t.Fatalf("--creates-paths did not skip the check: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "ticket", "refine", "--creates-paths"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("refine accepted --creates-paths: %s", x.stdout)
	}
	git(t, r.Root, "update-ref", "-d", "refs/remotes/origin/main")
	if x := create(`null`, `"lane/w.go"`); x.res.Outcome != wire.OutcomeOK || !warned(x, "NOT_OBSERVED (TCL-V0-743)") {
		t.Fatalf("a repository without an origin ref did not warn: %s", x.stdout)
	}
}
