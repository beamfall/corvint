//go:build darwin || linux

package cli_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// SCC-V0-013: queue status re-evaluates a typed hold condition at every
// read: BLOCKED while the deploy ref lacks the commit, UNBLOCKED (as a
// patch-equivalent commit) at the first read after the ref moves, UNKNOWN
// for an unresolvable ref; the ticket stays HELD and the doctor names the
// met and unknown conditions.
func TestSCCV0013_StatusReevaluatesHoldConditions(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "held", "P1", `["src/"]`)
	if _, ok := sccStatusConditions(t, root); ok {
		t.Fatal("holdConditions present without any typed hold")
	}
	base := sccCLIGit(t, root, nil, "rev-parse", "HEAD")
	x := sccCLICommit(t, root, base, "fixture.txt", "fixture\n", "add fixture")
	sccCLIGit(t, root, nil, "update-ref", "refs/heads/scc-deploy", base)
	sccHold(t, root, id, `{"holdId":"h1","reason":"needs the fixture commit deployed","condition":{"kind":"COMMIT_CONTAINED","ref":"refs/heads/scc-deploy","commit":"`+x+`"}}`)
	sccHold(t, root, id, `{"holdId":"h2","reason":"needs the observer","condition":{"kind":"PATH_EXISTS","ref":"refs/heads/scc-missing","path":"observer/"}}`)
	got, _ := sccStatusConditions(t, root)
	if field(got["h1"], "state").Str != "BLOCKED" || field(got["h2"], "state").Str != "UNKNOWN" {
		t.Fatalf("before deploy: %v", got)
	}
	if d := field(got["h2"], "detail").Str; !strings.Contains(d, "does not resolve") {
		t.Fatalf("UNKNOWN must name its reason: %q", d)
	}
	deployed := sccCLICommit(t, root, sccCLICommit(t, root, base, "other.txt", "o\n", "other"), "fixture.txt", "fixture\n", "add fixture")
	sccCLIGit(t, root, nil, "update-ref", "refs/heads/scc-deploy", deployed)
	got, _ = sccStatusConditions(t, root)
	h1 := got["h1"]
	if field(h1, "state").Str != "UNBLOCKED" || field(h1, "refCommit").Str != deployed ||
		field(field(h1, "containment"), "state").Str != "EQUIVALENT" || !strings.HasPrefix(field(h1, "detail").Str, "condition met at refs/heads/scc-deploy") {
		t.Fatalf("after deploy: %s", wire.Encode(h1))
	}
	if field(got["h2"], "state").Str != "UNKNOWN" {
		t.Fatalf("h2 after deploy: %s", wire.Encode(got["h2"]))
	}
	show := atm(t, root, nil, "ticket", "show", id)
	if field(field(show.res.Items[0], "record"), "status").Str != "HELD" {
		t.Fatalf("a met condition must not release the hold: %s", show.stdout)
	}
	_, item := doctorOK(t, root)
	met, unknown := findingsOf(item, "HOLD_CONDITION_MET"), findingsOf(item, "HOLD_CONDITION_UNKNOWN")
	if len(met) != 1 || field(met[0], "who").Str != id+"/h1" || len(unknown) != 1 || field(unknown[0], "who").Str != id+"/h2" {
		t.Fatalf("doctor hold findings: met=%v unknown=%v", met, unknown)
	}
}

// SCC-V0-014: the doctor names a free-text hold whose reason names a commit
// origin's default branch (refs/remotes/origin/HEAD) contains, or a path
// that exists at it; an uncontained commit reports nothing, and an
// unresolvable deploy ref reports nothing with a warning.
func TestSCCV0014_DoctorFreeTextHolds(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	base := sccCLIGit(t, root, nil, "rev-parse", "HEAD")
	x := sccCLICommit(t, root, base, "fixture.txt", "fixture\n", "add fixture")
	y := sccCLICommit(t, root, base, "never.txt", "never\n", "never deployed")
	sha, path, unmet := planTicket(t, root, "sha", "P1", `["src/"]`), planTicket(t, root, "path", "P1", `["src/"]`), planTicket(t, root, "unmet", "P1", `["src/"]`)
	sccHold(t, root, sha, `{"holdId":"h1","reason":"do not queue until `+x[:12]+` is deployed."}`)
	sccHold(t, root, path, `{"holdId":"h1","reason":"no mock yet (internal/mock/service.go); see https://example.invalid/a/b"}`)
	sccHold(t, root, unmet, `{"holdId":"h1","reason":"waiting on `+y+`"}`)
	x0 := atm(t, root, nil, "doctor")
	if len(findingsOf(x0.res.Items[0], "FREE_TEXT_HOLD_MET")) != 0 || !strings.Contains(strings.Join(x0.res.Warnings, "\n"), "refs/remotes/origin/HEAD does not resolve") {
		t.Fatalf("no origin default branch: %s", x0.stdout)
	}
	sccCLIGit(t, root, nil, "update-ref", "refs/remotes/origin/scc-deploy", base)
	sccCLIGit(t, root, nil, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/scc-deploy")
	if x1 := atm(t, root, nil, "doctor"); len(findingsOf(x1.res.Items[0], "FREE_TEXT_HOLD_MET")) != 0 || len(x1.res.Warnings) != 0 {
		t.Fatalf("nothing is deployed yet: %s", x1.stdout)
	}
	deployed := sccCLICommit(t, root, x, "internal/mock/service.go", "package mock\n", "add mock")
	sccCLIGit(t, root, nil, "update-ref", "refs/remotes/origin/scc-deploy", deployed)
	_, item := doctorOK(t, root)
	who := map[string]bool{}
	for _, f := range findingsOf(item, "FREE_TEXT_HOLD_MET") {
		who[field(f, "who").Str] = true
		if !strings.Contains(field(f, "detail").Str, "condition met at refs/remotes/origin/HEAD "+deployed) {
			t.Fatalf("finding detail: %s", wire.Encode(f))
		}
	}
	if !reflect.DeepEqual(who, map[string]bool{sha + "/h1": true, path + "/h1": true}) {
		t.Fatalf("free-text findings: %v", who)
	}
}
