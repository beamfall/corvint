package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// papercutRepo is an initialized queue with one created ticket; it returns
// the repo, the ticket's local token and its revision.
func papercutRepo(t *testing.T) (*fixture.Repo, string, string) {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	atm(t, r.Root, nil, "init")
	created := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-c", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %+v", created.res)
	}
	id := field(created.res.Items[0], "ticketId").Str
	return r, id[strings.LastIndex(id, ":")+1:], field(created.res.Items[0], "resultingRevision").Str
}

// TestTCLV0860_PositionalAndCurrentRevision: a positional ticket id or an
// unknown flag refuses MALFORMED with the code in codes and, for the
// positional, names --target and --expected-revision; --expected-revision
// current submits the read revision for one target and is refused with
// several.
func TestTCLV0860_PositionalAndCurrentRevision(t *testing.T) {
	r, local, revision := papercutRepo(t)
	pos := atm(t, r.Root, nil, "ticket", "prioritize", local, "--request-id", "p1", "--payload", `{"order":"1","priority":"P1"}`)
	if pos.code == 0 || pos.res.Outcome != wire.OutcomeError || !hasCode(pos.res, wire.CodeMalformed) {
		t.Fatalf("positional: code %d %+v", pos.code, pos.res)
	}
	if w := strings.Join(pos.res.Warnings, " "); !strings.Contains(w, "--target TICKET") || !strings.Contains(w, "--expected-revision N|current") {
		t.Errorf("positional refusal does not name the flags: %q", w)
	}
	unknown := atm(t, r.Root, nil, "ticket", "prioritize", "--bogus", "x", "--request-id", "p2")
	if unknown.res.Outcome != wire.OutcomeError || !hasCode(unknown.res, wire.CodeMalformed) {
		t.Errorf("unknown flag: %+v", unknown.res)
	}
	moved := atm(t, r.Root, nil, "ticket", "prioritize", "--request-id", "p3", "--target", local,
		"--expected-revision", "current", "--payload", `{"order":"1","priority":"P1"}`)
	if moved.res.Outcome != wire.OutcomeOK {
		t.Fatalf("--expected-revision current: %+v", moved.res)
	}
	if got := field(moved.res.Items[0], "resultingRevision").Str; got == revision || got == "" {
		t.Errorf("resultingRevision %q after revision %q", got, revision)
	}
	several := atm(t, r.Root, nil, "ticket", "hold", "--request-id", "p4", "--target", local, "--target", "L999",
		"--expected-revision", "current", "--payload", `{"reason":"x"}`)
	if several.res.Outcome == wire.OutcomeOK {
		t.Errorf("current with several targets was accepted: %+v", several.res)
	}
	create := atm(t, r.Root, nil, "ticket", "create", "--request-id", "p5", "--expected-revision", "current", "--payload", createPayloadJSON)
	if create.res.Outcome == wire.OutcomeOK {
		t.Errorf("CREATE with --expected-revision current was accepted: %+v", create.res)
	}
}

// examples collects every help-tree example that runs `corvint-tasks ticket VERB`.
func examples(v any, out map[string]string) {
	switch v := v.(type) {
	case map[string]any:
		if ex, ok := v["example"].(string); ok && strings.HasPrefix(ex, "corvint-tasks ticket ") {
			out[strings.Fields(ex)[2]] = ex
		}
		for _, x := range v {
			examples(x, out)
		}
	case []any:
		for _, x := range v {
			examples(x, out)
		}
	}
}

// TestTCLV0861_MutationExamplesRun: every single-target mutation's derived
// example names --target and --expected-revision, and prioritize's example
// with its placeholders filled runs.
func TestTCLV0861_MutationExamplesRun(t *testing.T) {
	r, local, revision := papercutRepo(t)
	var out bytes.Buffer
	if code := cli.Run(cli.Env{Cwd: r.Root, Args: []string{"help", "--json"}, Stdout: &out, Stderr: &out}); code != 0 {
		t.Fatalf("help --json: %d %s", code, out.Bytes())
	}
	var tree any
	if err := json.Unmarshal(out.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	examples(tree, got)
	for _, verb := range []string{"refine", "prioritize", "set-dependencies", "set-gates", "set-effects", "release-hold",
		"reopen", "archive", "restore", "complete-manual", "grant-approval", "revoke-approval", "attach-evidence", "progress"} {
		ex, ok := got[verb]
		if !ok {
			t.Errorf("no help example for ticket %s", verb)
			continue
		}
		if !strings.Contains(ex, " --target ") || !strings.Contains(ex, " --expected-revision ") {
			t.Errorf("ticket %s example omits a required flag: %q", verb, ex)
		}
	}
	argv := strings.Fields(got["prioritize"])[1:]
	fill := map[string]string{"ID": "ex-1", "JSON": `{"order":"1","priority":"P1"}`, "TICKET": local, "N": revision}
	for i, a := range argv {
		if v, ok := fill[a]; ok {
			argv[i] = v
		}
	}
	if x := atm(t, r.Root, nil, argv...); x.res.Outcome != wire.OutcomeOK {
		t.Errorf("filled example %v: %+v", argv, x.res)
	}
}

func papercutStore(t *testing.T) *fixture.Repo {
	t.Helper()
	r := fixture.TempRepo(t)
	a := fixture.Ticket("L001")
	a.Labels = []string{"alpha"}
	b := fixture.Ticket("L002")
	b.Status = ticket.StatusDraft
	cpWrite(t, r, true, a, b)
	return r
}

// TestTCLV0862_ListFieldsAcceptShowKeys: list and search --fields take show
// keys, record.* included, and stay CAL-V0-173 list items without one.
func TestTCLV0862_ListFieldsAcceptShowKeys(t *testing.T) {
	r := papercutStore(t)
	for _, argv := range [][]string{
		{"ticket", "list", "--fields", "ticketId,record.labels,claimable"},
		{"ticket", "search", "--status", "OPEN", "--fields", "ticketId,record.labels,effectiveRequiredGates"},
	} {
		x := atm(t, r.Root, nil, argv...)
		if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) == 0 {
			t.Fatalf("%v: %+v", argv, x.res)
		}
		item := x.res.Items[0]
		rec := field(item, "record")
		if rec.Kind != wire.KindObject {
			t.Fatalf("%v: no record member: %s", argv, wire.Encode(item))
		}
		labels := field(rec, "labels")
		if len(labels.Arr) != 1 || labels.Arr[0].Str != "alpha" {
			t.Errorf("%v: record.labels = %s", argv, wire.Encode(labels))
		}
		if keys := item.Obj.SortedKeys(); len(keys) != 3 {
			t.Errorf("%v: projected keys %v", argv, keys)
		}
	}
	for _, argv := range [][]string{{"ticket", "list"}, {"ticket", "list", "--fields", "ticketId,title"}} {
		x := atm(t, r.Root, nil, argv...)
		for _, item := range x.res.Items {
			if _, ok := item.Obj.Get("record"); ok {
				t.Errorf("%v: list item carries record", argv)
			}
		}
	}
	if x := atm(t, r.Root, nil, "ticket", "list", "--fields", "noSuchKey"); x.res.Outcome == wire.OutcomeOK {
		t.Errorf("unknown key accepted: %+v", x.res)
	}
}

// TestTCLV0863_SearchQueryNamesText: --query refuses MALFORMED naming --text.
func TestTCLV0863_SearchQueryNamesText(t *testing.T) {
	r := papercutStore(t)
	for _, argv := range [][]string{{"ticket", "search", "--query", "x"}, {"ticket", "search", "--query=x"}} {
		x := atm(t, r.Root, nil, argv...)
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMalformed) || !strings.Contains(strings.Join(x.res.Warnings, " "), "--text T") {
			t.Errorf("%v: %+v", argv, x.res)
		}
	}
	if x := atm(t, r.Root, nil, "ticket", "search", "--text", "--query"); x.res.Outcome != wire.OutcomeOK {
		t.Errorf("--query as the --text value refused: %+v", x.res)
	}
}

// TestTCLV0864_RoadmapStatusFilter: roadmap --status keeps only listed
// statuses, counts the kept rows and uses the list grammar.
func TestTCLV0864_RoadmapStatusFilter(t *testing.T) {
	r := papercutStore(t)
	x := atm(t, r.Root, nil, "roadmap", "--status", "DRAFT")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 || !strings.HasSuffix(field(x.res.Items[0], "ticketId").Str, "L002") {
		t.Fatalf("roadmap --status DRAFT: %+v", x.res)
	}
	if x := atm(t, r.Root, nil, "roadmap", "--status", "OPEN,DRAFT"); len(x.res.Items) != 2 {
		t.Errorf("roadmap --status OPEN,DRAFT: %+v", x.res)
	}
	if x := atm(t, r.Root, nil, "roadmap", "--status", "OPEN", "--count"); x.res.Outcome != wire.OutcomeOK || !strings.Contains(string(x.stdout), `"total":"1"`) {
		t.Errorf("roadmap --status OPEN --count: %s", x.stdout)
	}
	for _, argv := range [][]string{{"roadmap", "--status", "BOGUS"}, {"roadmap", "--status", "OPEN,OPEN"}} {
		if x := atm(t, r.Root, nil, argv...); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMalformed) {
			t.Errorf("%v: %+v", argv, x.res)
		}
	}
}

// TestTCLV0865_ShowNamesFailingIntentChecks: show names each failing intent
// check, and a manually completed ticket's status blocker says why it fails.
func TestTCLV0865_ShowNamesFailingIntentChecks(t *testing.T) {
	r, local, revision := papercutRepo(t)
	open := atm(t, r.Root, nil, "ticket", "show", local)
	if open.res.Outcome != wire.OutcomeOK {
		t.Fatalf("show: %+v", open.res)
	}
	item := open.res.Items[0]
	failed := field(item, "intentChecksFailed")
	if failed.Kind != wire.KindArray || (field(item, "intentChecks").Str == "PASSED") != (len(failed.Arr) == 0) {
		t.Fatalf("intentChecks %q with intentChecksFailed %s", field(item, "intentChecks").Str, wire.Encode(failed))
	}
	done := atm(t, r.Root, nil, "ticket", "complete-manual", "--request-id", "cm", "--target", local,
		"--expected-revision", revision, "--payload", `{"evidence":[],"reason":"migrated manual completion"}`)
	if done.res.Outcome != wire.OutcomeOK {
		t.Fatalf("complete-manual: %+v", done.res)
	}
	item = atm(t, r.Root, nil, "ticket", "show", local).res.Items[0]
	if field(item, "intentChecks").Str != "FAILED" || string(wire.Encode(field(item, "intentChecksFailed"))) != `["TICKET_STATE"]` {
		t.Fatalf("completed: intentChecks %q intentChecksFailed %s", field(item, "intentChecks").Str, wire.Encode(field(item, "intentChecksFailed")))
	}
	detail := field(field(item, "blockers").Arr[0], "detail").Str
	if !strings.Contains(detail, "admission checks") || !strings.Contains(detail, "completion itself is not in question") {
		t.Errorf("TICKET_STATE detail does not explain the closed status: %q", detail)
	}
}

// TestTMV0008_AS08_ExportOverLimitIsAnError (V1-3533): an over-limit export
// is an ERROR with LIMIT_EXCEEDED in codes and a nonzero exit, and it names
// the allowed range; it never looks like an empty successful page.
func TestTMV0008_AS08_ExportOverLimitIsAnError(t *testing.T) {
	r := papercutStore(t)
	x := atm(t, r.Root, nil, "ticket", "export", "--limit", "2000")
	if x.code == 0 || x.res.Outcome != wire.OutcomeError || !hasCode(x.res, wire.CodeLimitExceeded) || len(x.res.Items) != 0 {
		t.Fatalf("export --limit 2000: code %d %+v", x.code, x.res)
	}
	if w := strings.Join(x.res.Warnings, " "); !strings.Contains(w, "1..1000") {
		t.Errorf("refusal does not name the maximum: %q", w)
	}
}

// TestCAPV0060_CompleteManualHintNamesTicketVerb (V1-3828): the top-level
// `complete-manual` is an unknown verb whose hint names `ticket
// complete-manual`, and help lists that form.
func TestCAPV0060_CompleteManualHintNamesTicketVerb(t *testing.T) {
	r := fixture.TempRepo(t)
	x := atm(t, r.Root, nil, "complete-manual")
	if x.code == 0 || x.res.Outcome == wire.OutcomeOK || !strings.Contains(strings.Join(x.res.Warnings, " "), "corvint-tasks ticket complete-manual") {
		t.Fatalf("complete-manual: code %d %+v", x.code, x.res)
	}
	help := atm(t, r.Root, nil, "help")
	if !strings.Contains(string(help.stdout), "|complete-manual|") {
		t.Errorf("help does not list ticket complete-manual")
	}
}

// TestTCLV0866_ClaimRefusesWholeRepository pins V1-3597: claim
// --whole-repository is a parse-time MALFORMED refusal naming widen, not a
// transaction misfit, and the claim usage does not list the flag.
func TestTCLV0866_ClaimRefusesWholeRepository(t *testing.T) {
	r, local, _ := papercutRepo(t)
	for _, args := range [][]string{
		{"claim", local, "--holder", "h", "--request-id", "r866", "--whole-repository"},
		{"claim", "--next", "--holder", "h", "--request-id", "r866n", "--whole-repository"},
	} {
		got := atm(t, r.Root, nil, args...)
		if got.code == 0 || !hasCode(got.res, wire.CodeMalformed) {
			t.Fatalf("%v: code=%d stdout=%s", args, got.code, got.stdout)
		}
		if !strings.Contains(string(got.stdout), "widen --attempt A --generation G --whole-repository") || !strings.Contains(string(got.stdout), "TCL-V0-866") || strings.Contains(string(got.stdout), "lease arguments do not fit") {
			t.Fatalf("%v: refusal does not name widen: %s", args, got.stdout)
		}
	}
	help := atm(t, r.Root, nil, "claim", "--help")
	if strings.Contains(string(help.stdout), "--whole-repository") {
		t.Fatalf("claim usage lists --whole-repository: %s", help.stdout)
	}
}
