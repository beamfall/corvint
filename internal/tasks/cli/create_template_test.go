package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func strs(v wire.Value) []string {
	out := []string{}
	for _, e := range v.Arr {
		out = append(out, e.Str)
	}
	return out
}

// templateRun runs `ticket create --template` with a stdin that panics when
// read, so the test also proves the command never reads stdin.
func templateRun(t *testing.T, root string, extra ...string) *wire.Result {
	t.Helper()
	var out, errb bytes.Buffer
	args := append([]string{"ticket", "create", "--template"}, extra...)
	code := cli.Run(cli.Env{Cwd: root, Args: args, Stdin: unreadHelpInput{}, Stdout: &out, Stderr: &errb})
	res, err := wire.DecodeResult(out.Bytes())
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out.Bytes())
	}
	if (code == 0) != (res.Outcome == wire.OutcomeOK) || errb.Len() != 0 {
		t.Fatalf("%v: exit %d outcome %s stderr %s", args, code, res.Outcome, errb.Bytes())
	}
	return res
}

// TestCALV0069_CreateTemplateIsReadOnlyAndAccepted is V1-0751 acceptance
// criteria 1 and 3: the template writes nothing and takes no lock, and once
// title, body and acceptanceCriteria are filled in, ticket create accepts it.
func TestCALV0069_CreateTemplateIsReadOnlyAndAccepted(t *testing.T) {
	r := receiptFixture(t) // initialized store with one ticket; lock removed
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	res := templateRun(t, r.Root)
	fixture.AssertUntouched(t, r, state, intents, "ticket create --template")
	if _, err := os.Lstat(filepath.Join(r.CommonDir, "taskman.lock")); !os.IsNotExist(err) {
		t.Fatalf("template created the lock: %v", err)
	}
	if res.Snapshot != nil || res.Mutation != nil || len(res.Items) != 1 {
		t.Fatalf("unexpected envelope claims: %+v", res)
	}
	item := res.Items[0]
	payload := field(item, "payload")
	if got := field(item, "payloadCanonical").Str; got != string(wire.Encode(payload)) {
		t.Fatalf("payloadCanonical is not the canonical payload: %s", got)
	}
	// The printed bytes must already be canonical: they parse back unchanged.
	if v, err := wire.Parse([]byte(field(item, "payloadCanonical").Str + "\n")); err != nil || string(wire.Encode(v)) != field(item, "payloadCanonical").Str {
		t.Fatalf("payload is not byte-canonical: %v", err)
	}
	if !slices.Equal(strs(field(item, "fill")), []string{"acceptanceCriteria", "body", "title"}) {
		t.Errorf("fill = %v", strs(field(item, "fill")))
	}

	unedited := atm(t, r.Root, nil, "ticket", "create", "--request-id", "tpl-unedited",
		"--issued-at", "2026-10-04T12:00:00Z", "--payload", field(item, "payloadCanonical").Str)
	if unedited.res.Outcome == wire.OutcomeOK {
		t.Fatal("an unedited template was accepted; its empty title must refuse")
	}

	payload.Obj.Set("title", wire.String("Template ticket"))
	payload.Obj.Set("body", wire.String("Filled from ticket create --template."))
	payload.Obj.Set("acceptanceCriteria", wire.Strings([]string{"it round-trips"}))
	created := atm(t, r.Root, []byte(string(wire.Encode(payload))), "ticket", "create",
		"--request-id", "tpl-filled", "--issued-at", "2026-10-04T12:00:00Z", "--payload-stdin")
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("filled template refused: %+v", created.res)
	}
	shown := atm(t, r.Root, nil, "ticket", "show", field(created.res.Items[0], "ticketId").Str)
	if shown.res.Outcome != wire.OutcomeOK || field(shown.res.Items[0], "status").Str != ticket.StatusOpen {
		t.Fatalf("created ticket is not OPEN: %+v", shown.res)
	}
}

// TestCALV0069_CreateTemplateNamesEnumsAndNullableKeys is acceptance
// criterion 2: every CREATE key, nested effects and source member is
// documented, enum values come from the decoder's vocabularies, and the
// nullable keys are named. It also works before init: it reads queue.json.
func TestCALV0069_CreateTemplateNamesEnumsAndNullableKeys(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	res := templateRun(t, r.Root)
	if res.Outcome != wire.OutcomeOK {
		t.Fatalf("template on an uninitialized store: %+v", res)
	}
	item := res.Items[0]
	fields := field(item, "fields")
	queueID := field(item, "queueId").Str
	if queueID != "queue:acme:main" {
		t.Fatalf("queueId = %q", queueID)
	}
	keys := append(append([]string{}, mutation.PayloadKeys[mutation.OpCreate]...), strs(field(item, "optionalKeys"))...)
	keys = append(keys, "effects.coverage", "effects.externalUnbounded", "effects.resources", "effects.resources[].class",
		"effects.resources[].key", "effects.touchPaths", "source.kind", "source.sourceItemId", "source.sourceQueueId",
		"source.sourceRevisionSha256", "dependencies[].gateId", "dependencies[].obligation", "dependencies[].ticketId")
	for _, k := range keys {
		if _, ok := fields.Obj.Get(k); !ok {
			t.Errorf("fields omits %s", k)
		}
	}
	if len(fields.Obj.SortedKeys()) != len(keys) {
		t.Errorf("fields has %d entries, want %d", len(fields.Obj.SortedKeys()), len(keys))
	}
	for k, want := range map[string][]string{
		"kind": ticket.Kinds, "priority": ticket.Priorities, "executionClass": ticket.ExecutionClasses,
		"effects.coverage": ticket.Coverages, "effects.resources[].class": ticket.ResourceClasses,
		"source.kind": ticket.SourceKinds, "dependencies[].obligation": ticket.Obligations,
		"requiredRoles": ticket.StageRoles, "source.sourceQueueId": {queueID}, "requiredGates": {"verify"},
	} {
		if got := strs(field(field(fields, k), "values")); !slices.Equal(got, want) {
			t.Errorf("%s values = %v, want %v", k, got, want)
		}
	}
	wantNull := []string{"body", "dependencies[].gateId", "dueDate", "estimateMinutes", "milestone", "owner",
		"source.sourceItemId", "source.sourceRevisionSha256", "supersededBy", "supersedes"}
	if got := strs(field(item, "nullableKeys")); !slices.Equal(got, wantNull) {
		t.Errorf("nullableKeys = %v, want %v", got, wantNull)
	}
	src := field(field(item, "payload"), "source")
	if field(src, "sourceQueueId").Str != queueID || field(src, "kind").Str != "NATIVE" {
		t.Errorf("source = %s", wire.Encode(src))
	}
}

// TestCALV0069_TemplateRefusesOtherVerbsAndFlags keeps --template a pure
// read: no other verb takes it and it composes with no mutation flag.
func TestCALV0069_TemplateRefusesOtherVerbsAndFlags(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "ticket", "refine", "--template"); x.res.Outcome == wire.OutcomeOK {
		t.Error("refine accepted --template")
	}
	if res := templateRun(t, r.Root, "--request-id", "x"); res.Outcome == wire.OutcomeOK {
		t.Error("--template accepted --request-id")
	}
	help := atm(t, r.Root, nil, "ticket", "create", "--help")
	if !slices.Contains(strs(field(help.res.Items[0], "flags")), "--template") || field(help.res.Items[0], "template").Str == "" {
		t.Errorf("create help does not name --template: %s", help.stdout)
	}
}
