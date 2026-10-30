package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// packetCommit writes files into root and commits them, returning the
// commit and its tree.
func packetCommit(t *testing.T, root, msg string, files map[string]string) (string, string) {
	t.Helper()
	for p, body := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		fixture.Write(t, full, []byte(body))
		git(t, root, "add", "--", p)
	}
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", msg)
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD", "HEAD^{tree}").Output()
	if err != nil {
		t.Fatal(err)
	}
	f := strings.Fields(string(out))
	return f[0], f[1]
}

func numbered(prefix string, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(prefix + " line " + strings.Repeat("x", i%7) + "\n")
	}
	return b.String()
}

func packetOf(t *testing.T, root, id string, args ...string) (wire.Value, run) {
	t.Helper()
	x := atm(t, root, nil, append([]string{"gate", "packet", id}, args...)...)
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 || !x.res.Untrusted {
		t.Fatalf("gate packet %v: %s", args, x.stdout)
	}
	return x.res.Items[0], x
}

func packetFiles(v wire.Value) []string {
	var out []string
	for _, f := range field(field(v, "diff"), "files").Arr {
		out = append(out, field(f, "path").Str)
	}
	return out
}

// reviewRounds drives a ticket through RETURN, resubmit and PASS on commit
// c1, then a second candidate c2 RETURNed with one item that names a changed
// test file and one that does not; c3 changes the tests again.
func reviewRounds(t *testing.T) (root, id, c1, c3 string) {
	t.Helper()
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, _ = ergStore(t)
	id = planTicket(t, root, "reviewed", "P1", `["src/"]`)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	c1, tree1 := packetCommit(t, root, "c1", map[string]string{"src/login.go": "package src\n", "src/login_test.go": numbered("base", 20), "src/fixtures/user.json": "{}\n"})
	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
	attempt, generation := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", tree1, "--request-id", "submit-a")
	subject := field(runOK("receipt", "audit").res.Items[0], "headSeq").Str
	runOK("gate", "record", id, "--gate", ergGate, "--verdict", "RETURN", "--subject-receipt", subject, "--expected-generation", "0", "--expected-revision", "0",
		"--request-id", "review-1", "--reason", "TESTS:first round")
	runOK("gate", "resubmit", id, "--gate", ergGate, "--author-attempt", attempt, "--subject-receipt", subject,
		"--expected-generation", "1", "--expected-revision", "1", "--reason", "FIXED:added the case", "--request-id", "resubmit-1")
	runOK("gate", "record", id, "--gate", ergGate, "--verdict", "PASS", "--subject-receipt", subject, "--expected-generation", "2", "--expected-revision", "2", "--request-id", "review-2")
	runOK("release", "--attempt", attempt, "--generation", generation, "--reason", wire.CodeHandoff, "--request-id", "release-a")

	_, tree2 := packetCommit(t, root, "c2", map[string]string{"src/login.go": "package src\n\nfunc Login() {}\n", "src/login_test.go": numbered("base", 20) + "lockout\n"})
	c = runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-b")
	attempt, generation = field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", tree2, "--request-id", "submit-b")
	subject = field(runOK("receipt", "audit").res.Items[0], "headSeq").Str
	runOK("gate", "record", id, "--gate", ergGate, "--verdict", "RETURN", "--subject-receipt", subject, "--expected-generation", "2", "--expected-revision", "3",
		"--request-id", "review-3", "--reason", "STYLE:session.go naming", "--reason", "TESTS:login_test.go lockout asserts nothing")
	c3, _ = packetCommit(t, root, "c3", map[string]string{"src/login_test.go": numbered("base", 20) + "lockout\nassert locked\n", "src/other_test.go": "package src\n"})
	return root, id, c1, c3
}

// TestERGV0017_ReviewPacketAcrossRounds: after several review rounds the
// packet's delta base is the newest ancestor that passed review, its diff is
// limited to test and fixture paths, it names every returned item of the
// last RETURN with its touched status, fits the cap with nothing omitted,
// and writes nothing (ERG-V0-017, 018, 020, 021).
func TestERGV0017_ReviewPacketAcrossRounds(t *testing.T) {
	root, id, c1, c3 := reviewRounds(t)
	before := fixture.TreeSnapshot(t, filepath.Join(root, ".git"))
	v, x := packetOf(t, root, id, "--commit", "HEAD")
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, filepath.Join(root, ".git"))) {
		t.Fatal("gate packet wrote state")
	}
	base := field(v, "deltaBase")
	if field(v, "profile").Str != "taskman-review-packet/0" || field(v, "commit").Str != c3 || field(base, "commit").Str != c1 || field(base, "kind").Str != "PASSED_REVIEW" ||
		field(base, "gate").Str != ergGate || field(field(v, "diff"), "scope").Str != "TEST_FIXTURE_PATHS" {
		t.Fatalf("packet base: %s", x.stdout)
	}
	if got := strings.Join(packetFiles(v), ","); got != "src/login_test.go,src/other_test.go" {
		t.Fatalf("diff files %s: %s", got, x.stdout)
	}
	items := field(field(v, "returned"), "items").Arr
	if len(items) != 2 || field(items[0], "code").Str != "STYLE" || field(items[0], "status").Str != "UNTOUCHED" || field(items[1], "code").Str != "TESTS" ||
		field(items[1], "status").Str != "TOUCHED" || strList(items[1], "paths") != "src/login_test.go" || field(items[1], "text").Str != "login_test.go lockout asserts nothing" {
		t.Fatalf("returned items: %s", wire.Encode(field(v, "returned")))
	}
	if n := len(wire.Encode(v)); n > 32768 || len(field(v, "omitted").Arr) != 0 || field(field(v, "obligations"), "state").Str != "NONE" {
		t.Fatalf("packet size %d omitted %s", n, wire.Encode(field(v, "omitted")))
	}
}

// TestERGV0019_CapNamesOmittedRanges: a packet over the cap keeps every
// item code and status and lists each omitted hunk by its line range; a cap
// outside 4096..262144 is a usage error (ERG-V0-019).
func TestERGV0019_CapNamesOmittedRanges(t *testing.T) {
	root, id, _, _ := reviewRounds(t)
	big := map[string]string{}
	for _, name := range []string{"a", "b", "c"} {
		big["src/testdata/"+name+".txt"] = numbered(strings.Repeat(name, 40), 120)
	}
	packetCommit(t, root, "big", big)
	v, x := packetOf(t, root, id, "--commit", "HEAD", "--max-bytes", "4096")
	if n := len(wire.Encode(v)); n > 4096 {
		t.Fatalf("packet %d bytes over the cap: %s", n, x.stdout)
	}
	if items := field(field(v, "returned"), "items").Arr; len(items) != 2 || field(items[1], "status").Str != "TOUCHED" {
		t.Fatalf("items under the cap: %s", x.stdout)
	}
	ranges := map[string]bool{}
	for _, o := range field(v, "omitted").Arr {
		if field(o, "section").Str == "diff" {
			ranges[field(o, "path").Str+" "+field(o, "range").Str] = true
		}
	}
	if !ranges["src/testdata/c.txt lines 1-120"] {
		t.Fatalf("omitted ranges %v: %s", ranges, wire.Encode(field(v, "omitted")))
	}
	for _, bad := range []string{"4095", "262145", "x"} {
		if x := atm(t, root, nil, "gate", "packet", id, "--max-bytes", bad); x.res.Outcome != wire.OutcomeError || !strings.Contains(string(x.stdout), "--max-bytes is 4096..262144") {
			t.Fatalf("--max-bytes %s: %s", bad, x.stdout)
		}
	}
}

// TestERGV0017_ReviewClaimNamesClearedCandidate: before a review-stage
// claim the packet without --commit is the submitted candidate's (what the
// dispatcher now reads before its pre-launch claim); once the claim opens a
// generation without the candidate, it refuses with the named reason
// instead of a bare missing branch (Codex #1189 finding 2).
func TestERGV0017_ReviewClaimNamesClearedCandidate(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, _ := ergStore(t)
	id := planTicket(t, root, "reviewed", "P1", `["src/"]`)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	c1, tree := packetCommit(t, root, "c1", map[string]string{"src/login_test.go": "package src\n"})
	git(t, root, "branch", id[strings.LastIndex(id, ":")+1:], c1) // the worker's candidate branch
	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
	attempt, generation := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", tree, "--request-id", "submit-a")
	runOK("release", "--attempt", attempt, "--generation", generation, "--reason", wire.CodeHandoff, "--request-id", "release-a")
	if v, x := packetOf(t, root, id); field(v, "commit").Str != c1 {
		t.Fatalf("packet before the review claim: %s", x.stdout)
	}
	runOK("claim", id, "--holder", "tester", "--stage", "review", "--request-id", "claim-review")
	x := atm(t, root, nil, "gate", "packet", id)
	if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "CANDIDATE_CLEARED_BY_CLAIM:") || !strings.Contains(string(x.stdout), "pass --commit") {
		t.Fatalf("packet after a review claim: %s", x.stdout)
	}
}

// TestERGV0018_NoPassedAncestorFallsBack: a ticket with no PASS gets the
// full diff from its fallback base, every path, and the packet says so
// (ERG-V0-018).
func TestERGV0018_NoPassedAncestorFallsBack(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := ergStore(t)
	id := planTicket(t, root, "unreviewed", "P1", `["src/"]`)
	first, _ := packetCommit(t, root, "c1", map[string]string{"src/a.go": "package src\n"})
	packetCommit(t, root, "c2", map[string]string{"src/a.go": "package src\n\nvar A = 1\n", "src/a_test.go": "package src\n"})
	v, x := packetOf(t, root, id, "--commit", "HEAD")
	base := field(v, "deltaBase")
	if field(base, "kind").Str != "NO_PASSED_ANCESTOR" || field(base, "source").Str != "FIRST_PARENT" || field(base, "commit").Str != first ||
		!strings.Contains(field(base, "note").Str, "no ancestor commit passed review") || field(field(v, "diff"), "scope").Str != "ALL_PATHS" ||
		strings.Join(packetFiles(v), ",") != "src/a.go,src/a_test.go" || field(v, "returned").Kind != wire.KindNull {
		t.Fatalf("fallback packet: %s", x.stdout)
	}
}

// TestERGV0027_HistoryBoundNamesNextCursor: a gate with more events than the
// packet reads is named in omitted and in readOnDemand.history as the exact
// `gate history --cursor` command, and that command returns the events the
// packet did not read (ERG-V0-027).
func TestERGV0027_HistoryBoundNamesNextCursor(t *testing.T) {
	root, id, c1, _ := reviewRounds(t)
	cli.ShrinkReviewPacketHistory(t, 2, 1)
	v, x := packetOf(t, root, id, "--commit", "HEAD")
	if base := field(v, "deltaBase"); field(base, "commit").Str != c1 || field(base, "kind").Str != "PASSED_REVIEW" {
		t.Fatalf("packet base under the history bound: %s", x.stdout)
	}
	more := field(field(v, "readOnDemand"), "history").Arr
	if len(more) != 1 {
		t.Fatalf("readOnDemand.history: %s", x.stdout)
	}
	cmd := strings.Fields(more[0].Str)
	if len(cmd) != 7 || strings.Join(cmd[:5], " ") != "gate history "+id+" --gate "+ergGate || cmd[5] != "--cursor" {
		t.Fatalf("history command %q", more[0].Str)
	}
	var named bool
	for _, o := range field(v, "omitted").Arr {
		named = named || field(o, "section").Str == "history" && field(o, "path").Str == ergGate && strings.Contains(field(o, "range").Str, cmd[6])
	}
	if !named {
		t.Fatalf("omitted does not name the history cursor: %s", wire.Encode(field(v, "omitted")))
	}
	all := atm(t, root, nil, "gate", "history", id, "--gate", ergGate)
	rest := atm(t, root, nil, cmd...)
	if all.res.Outcome != wire.OutcomeOK || rest.res.Outcome != wire.OutcomeOK || len(all.res.Items) != 4 || len(rest.res.Items) != 2 {
		t.Fatalf("history all %s rest %s", all.stdout, rest.stdout)
	}
	for i, it := range rest.res.Items {
		if field(it, "sha256").Str != field(all.res.Items[2+i], "sha256").Str {
			t.Fatalf("cursor page event %d is not the unread event: %s", i, rest.stdout)
		}
	}
}

// TestERGV0026_CheckpointCommitBinding: a hand-off note naming a commit the
// packet's repository holds binds the checkpoint OBSERVED with its relation
// to the reviewed commit; a commit the repository lacks stays NOT_OBSERVED
// (ERG-V0-026).
func TestERGV0026_CheckpointCommitBinding(t *testing.T) {
	root, id, c1, c3 := reviewRounds(t)
	note := func(name, commit string) string {
		raw, err := json.Marshal(&dispatch.HandoffNote{Profile: dispatch.HandoffNoteProfile, Worker: "w-1", Ticket: id, Role: "implement",
			At: "2026-10-21T00:00:00Z", ContextTokens: "0", Messages: []string{"checkpoint"}, Files: []dispatch.NoteFile{}, Commit: commit})
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(t.TempDir(), name)
		fixture.Write(t, p, append(raw, '\n'))
		return p
	}
	for _, tc := range []struct{ name, commit, binding, relation string }{
		{"reviewed", c3, "OBSERVED", cli.CheckpointAtReviewed},
		{"ancestor", c1, "OBSERVED", cli.CheckpointAncestor},
		{"absent", strings.Repeat("ab", 20), "NOT_OBSERVED", ""},
		{"none", "", "NOT_OBSERVED", ""},
	} {
		v, x := packetOf(t, root, id, "--commit", "HEAD", "--checkpoint", note(tc.name+".json", tc.commit))
		cp := field(v, "checkpoint")
		if field(cp, "commitBinding").Str != tc.binding || field(cp, "commitRelation").Str != tc.relation || field(cp, "commit").Str != tc.commit {
			t.Fatalf("%s checkpoint: %s", tc.name, x.stdout)
		}
	}
}
