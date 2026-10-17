package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/defects"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// policyCLIRepo is exclusionCLIRepo with edit applied to the policy before
// init, plus one committed source file.
func policyCLIRepo(t *testing.T, edit func(p wire.Value)) *fixture.Repo {
	t.Helper()
	r := exclusionCLIRepo(t)
	path := filepath.Join(r.IntentDir, "policy.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pv, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	edit(pv)
	fixture.Write(t, path, wire.EncodeFile(pv))
	fixture.Write(t, filepath.Join(r.Root, "src", "a.go"), []byte("package a\n"))
	git(t, r.Root, "add", "src")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "files")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %s", x.stdout)
	}
	return r
}

func objectOf(t *testing.T, raw string) wire.Value {
	t.Helper()
	v, err := wire.ParseInput([]byte(raw))
	if err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	return v
}

// TestKHNV0030_KHNV0031_ValueScreenRefusesWithoutEcho: with knowHow.screen
// declaring a values file and the EMAIL class, a declared value or an email
// address in a ticket note, a know-how note or obligation text refuses
// SECRET_DETECTED naming the field and class, never the value, and writes
// nothing; PHONE, not opted in, is accepted; an unreadable values file
// refuses the screened write.
func TestKHNV0030_KHNV0031_ValueScreenRefusesWithoutEcho(t *testing.T) {
	const deny = "tok-7f3a9c2e1b"
	r := policyCLIRepo(t, func(p wire.Value) {
		p.Obj.Set("knowHow", objectOf(t, `{"workerAdd":false,"screen":{"denyValuesFile":"secrets/deny.env","classes":["EMAIL"]}}`))
	})
	fixture.Write(t, filepath.Join(r.Root, "secrets", "deny.env"), []byte("API_TOKEN="+deny+"\n"))
	t.Setenv("CORVINT_TASKS_ACTOR", "owner")
	id := planTicket(t, r.Root, "screened", "P2", `["src/a.go"]`)
	issued := "2026-10-17T12:00:00Z"
	note := func(req, text string) run {
		return atm(t, r.Root, nil, "ticket", "note", "set", id, "--request-id", req, "--issued-at", issued, "--text", text, "--supersedes", "0")
	}
	knowHow := func(req, text string) run {
		return atm(t, r.Root, nil, "ticket", "know-how", "add", id, "--request-id", req, "--expected-revision", "1", "--issued-at", issued, "--text", text, "--anchor", "src/a.go")
	}
	seed := func(req, title string) run {
		return atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
			"--request-id", req, "--issued-at", issued, "--payload", `{"prefix":"AC","obligations":[{"id":"AC-1","title":"`+title+`","core":true}]}`)
	}
	before := fixture.TreeSnapshot(t, r.StateDir)
	for name, x := range map[string]run{
		"note value":       note("n-1", "export API_TOKEN="+deny+" first"),
		"note email":       note("n-2", "ask ops@example.com"),
		"know-how value":   knowHow("k-1", "set "+deny+" before go test"),
		"obligation value": seed("o-1", "login with "+deny),
		"obligation email": seed("o-2", "mail ops@example.com"),
	} {
		out := string(x.stdout)
		if x.res.Outcome == wire.OutcomeOK || !strings.Contains(out, `"`+wire.CodeSecretDetected+`"`) || strings.Contains(out, deny) || strings.Contains(out, "ops@example") ||
			!strings.Contains(out, "VALUE_SCREEN_MATCHED") {
			t.Fatalf("%s: %s", name, out)
		}
		if strings.HasSuffix(name, "email") && !strings.Contains(out, "EMAIL") {
			t.Fatalf("%s does not name the class: %s", name, out)
		}
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("a screened refusal wrote state")
	}
	if x := note("n-3", "call +44 20 7946 0958 for access"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("PHONE is not opted in: %s", x.stdout)
	}
	if err := os.Remove(filepath.Join(r.Root, "secrets", "deny.env")); err != nil {
		t.Fatal(err)
	}
	if x := knowHow("k-2", "plain text"); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "SCREEN_VALUES_UNAVAILABLE") {
		t.Fatalf("missing values file: %s", x.stdout)
	}
}

// TestTWTV0007_WorktreeEnsureThroughTheCLI: without a worktrees section the
// verb refuses PREREQUISITE_UNSATISFIED; with one it creates the ticket
// worktree once, returns the same path and branch again, reports a dirty
// worktree, and writes no Tasks state.
func TestTWTV0007_WorktreeEnsureThroughTheCLI(t *testing.T) {
	plain := policyCLIRepo(t, func(wire.Value) {})
	pid := planTicket(t, plain.Root, "plain", "P2", `["src/a.go"]`)
	if x := atm(t, plain.Root, nil, "worktree", "ensure", pid); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodePrerequisiteUnsatisfied) {
		t.Fatalf("undeclared: %s", x.stdout)
	}
	r := policyCLIRepo(t, func(p wire.Value) {
		p.Obj.Set("worktrees", objectOf(t, `{"repository":".","base":"main","branchPattern":"ticket/{ticket}"}`))
	})
	id := planTicket(t, r.Root, "wt", "P2", `["src/a.go"]`)
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	first := atm(t, r.Root, nil, "worktree", "ensure", id)
	if first.res.Outcome != wire.OutcomeOK || len(first.res.Items) != 1 {
		t.Fatalf("ensure: %s", first.stdout)
	}
	item := first.res.Items[0]
	local := strings.TrimPrefix(id, "")
	if i := strings.LastIndex(id, ":"); i >= 0 {
		local = id[i+1:]
	}
	if field(item, "branch").Str != "ticket/"+local || !field(item, "created").Bool || field(item, "dirty").Bool ||
		field(field(item, "repository"), "sha256").Str == "" || field(item, "head").Str != field(item, "baseCommit").Str {
		t.Fatalf("first: %s", first.stdout)
	}
	path := field(item, "path").Str
	fixture.Write(t, filepath.Join(path, "wip.txt"), []byte("wip\n"))
	again := atm(t, r.Root, nil, "worktree", "ensure", local)
	if again.res.Outcome != wire.OutcomeOK || field(again.res.Items[0], "path").Str != path || field(again.res.Items[0], "branch").Str != field(item, "branch").Str ||
		field(again.res.Items[0], "created").Bool || !field(again.res.Items[0], "dirty").Bool || len(again.res.Warnings) == 0 {
		t.Fatalf("again: %s", again.stdout)
	}
	if raw, err := os.ReadFile(filepath.Join(path, "wip.txt")); err != nil || string(raw) != "wip\n" {
		t.Fatalf("dirty file changed: %v", err)
	}
	if x := atm(t, r.Root, nil, "worktree", "ensure", id, "--path", "../outside"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeOutOfScope) {
		t.Fatalf("outside root: %s", x.stdout)
	}
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("worktree ensure wrote Tasks state")
	}
}

// TestTDRV0010_DefectsVerbsThroughTheCLI: add assigns ids in the private
// register, update appends, list filters and writes nothing, and none of
// them writes the Tasks journal or intent.
func TestTDRV0010_DefectsVerbsThroughTheCLI(t *testing.T) {
	r := policyCLIRepo(t, func(wire.Value) {})
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	if x := atm(t, r.Root, nil, "defects", "list"); x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 0 || len(x.res.Warnings) == 0 {
		t.Fatalf("empty list: %s", x.stdout)
	}
	reg := filepath.Join(r.Root, ".git", defects.PrivateDir, defects.RegisterFile)
	if _, err := os.Stat(reg); !os.IsNotExist(err) {
		t.Fatal("list created the register")
	}
	add := atm(t, r.Root, nil, "defects", "add", "--title", "proxy resets", "--location", "e2e/a.spec.ts:3", "--evidence", "ci run 7", "--category", "ENVIRONMENT")
	if add.res.Outcome != wire.OutcomeOK || field(add.res.Items[0], "id").Str != "DEF-1" || field(add.res.Items[0], "status").Str != "SUSPECTED" || !add.res.Untrusted {
		t.Fatalf("add: %s", add.stdout)
	}
	if x := atm(t, r.Root, nil, "defects", "add", "--title", "total wrong", "--location", "src/a.go", "--evidence", "AC-1 fails"); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "id").Str != "DEF-2" {
		t.Fatalf("second add: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "defects", "update", "DEF-2", "--status", "CONFIRMED", "--severity", "HIGH"); x.res.Outcome != wire.OutcomeOK ||
		field(x.res.Items[0], "revision").Str != "2" || field(x.res.Items[0], "title").Str != "total wrong" {
		t.Fatalf("update: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "defects", "update", "DEF-9", "--status", "CONFIRMED"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMissingEvidence) {
		t.Fatalf("unknown update: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "defects", "add", "--title", "t", "--location", "l", "--evidence", "AKIAABCDEFGHIJKLMNOP"); x.res.Outcome == wire.OutcomeOK ||
		!hasCode(x.res, wire.CodeSecretDetected) || strings.Contains(string(x.stdout), "AKIAABCDEFGHIJKLMNOP") {
		t.Fatalf("secret: %s", x.stdout)
	}
	regBefore, err := os.ReadFile(reg)
	if err != nil || strings.Count(string(regBefore), "\n") != 3 {
		t.Fatalf("register: %v\n%s", err, regBefore)
	}
	list := atm(t, r.Root, nil, "defects", "list", "--category", "PRODUCT")
	if list.res.Outcome != wire.OutcomeOK || len(list.res.Items) != 1 || field(list.res.Items[0], "status").Str != "CONFIRMED" {
		t.Fatalf("list: %s", list.stdout)
	}
	if regAfter, _ := os.ReadFile(reg); string(regAfter) != string(regBefore) {
		t.Fatal("list wrote the register")
	}
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("defects verbs wrote Tasks state")
	}
}

// TestTDRV0007_TDRV0008_PreflightDefectFindings: preflight checks register
// ids cited by obligations and expected-fail lines: an unknown id and a
// NOT_A_BUG id are findings, and a DEFECT obligation resting only on an
// ENVIRONMENT defect is DEFECT_ENVIRONMENT, while a PRODUCT defect satisfies.
func TestTDRV0007_TDRV0008_PreflightDefectFindings(t *testing.T) {
	r, id, _ := preflightRepo(t, strings.Replace(cleanSpec, "(BUG-17)", "(DEF-9)", 1), cleanSeed)
	for _, args := range [][]string{
		{"defects", "add", "--title", "proxy", "--location", "ci", "--evidence", "run 1", "--category", "ENVIRONMENT"},
		{"defects", "add", "--title", "discount", "--location", "src", "--evidence", "run 2"},
		{"defects", "add", "--title", "flake", "--location", "src", "--evidence", "run 3"},
		{"defects", "update", "DEF-3", "--status", "NOT_A_BUG"},
	} {
		if x := atm(t, r.Root, nil, args...); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	setSources(t, r.Root, id, "set-defects", `[{"id":"AC-1","state":"DEFECT","core":null,"reason":"blocked by DEF-1"},`+
		`{"id":"AC-2","state":"DEFECT","core":null,"reason":"blocked by DEF-2"},{"id":"AC-5","state":"DEFECT","core":null,"reason":"see DEF-3"}]`)
	x := atm(t, r.Root, nil, "preflight", id)
	if len(x.res.Items) != 1 {
		t.Fatalf("preflight: %s", x.stdout)
	}
	var got []string
	for _, f := range strings.Split(findingList(t, x.res.Items[0]), ",") {
		if strings.HasPrefix(f, "DEFECT_") {
			got = append(got, f)
		}
	}
	want := "DEFECT_ENVIRONMENT:AC-1::,DEFECT_NOT_A_BUG:DEF-3::,DEFECT_UNKNOWN:DEF-9:e2e/checkout.spec.ts:8"
	if strings.Join(got, ",") != want || x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("findings\n got %s\nwant %s\n%s", strings.Join(got, ","), want, x.stdout)
	}
}
