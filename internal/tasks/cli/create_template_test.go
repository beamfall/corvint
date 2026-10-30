package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// TestCALV0073_CreateTemplateIsReadOnlyAndAccepted is V1-0751 acceptance
// criteria 1 and 3: the template writes nothing and takes no lock, and once
// title, body and acceptanceCriteria are filled in, ticket create accepts it.
func TestCALV0073_CreateTemplateIsReadOnlyAndAccepted(t *testing.T) {
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

// TestCALV0073_CreateTemplateNamesEnumsAndNullableKeys is acceptance
// criterion 2: every CREATE key, nested effects and source member is
// documented, enum values come from the decoder's vocabularies, and the
// nullable keys are named. It also works before init: it reads queue.json.
func TestCALV0073_CreateTemplateNamesEnumsAndNullableKeys(t *testing.T) {
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
	} {
		if got := strs(field(field(fields, k), "values")); !slices.Equal(got, want) {
			t.Errorf("%s values = %v, want %v", k, got, want)
		}
	}
	// values is reserved for a key's own closed enum: array members use
	// elementValues and the open queue identifier uses current.
	for _, c := range []struct {
		key, name string
		want      []string
	}{
		{"requiredRoles", "elementValues", ticket.StageRoles},
		{"requiredGates", "elementValues", []string{"verify"}},
		{"source.sourceQueueId", "current", []string{queueID}},
	} {
		entry := field(fields, c.key)
		if got := strs(field(entry, c.name)); !slices.Equal(got, c.want) {
			t.Errorf("%s %s = %v, want %v", c.key, c.name, got, c.want)
		}
		if _, ok := entry.Obj.Get("values"); ok {
			t.Errorf("%s carries values though it is not a closed scalar enum", c.key)
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

// TestCALV0073_TemplateRefusesOtherVerbsAndFlags keeps --template a pure
// read: no other verb takes it and it composes with no mutation flag.
func TestCALV0073_TemplateRefusesOtherVerbsAndFlags(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "ticket", "refine", "--template"); x.res.Outcome == wire.OutcomeOK {
		t.Error("refine accepted --template")
	}
	// Refusal is by flag presence, so a flag set to its default value or to
	// the empty string is refused too.
	for _, extra := range [][]string{{"--request-id", "x"}, {"--role", "OWNER"}, {"--payload", ""}, {"--payload-stdin"}} {
		if res := templateRun(t, r.Root, extra...); res.Outcome == wire.OutcomeOK {
			t.Errorf("--template accepted %v", extra)
		}
	}
	help := atm(t, r.Root, nil, "ticket", "create", "--help", "--verbose")
	if !slices.Contains(strs(field(help.res.Items[0], "flags")), "--template") || field(help.res.Items[0], "template").Str == "" {
		t.Errorf("create help does not name --template: %s", help.stdout)
	}
}

// payloadPaths lists every dotted key path of a payload object; an array
// of objects contributes NAME[].MEMBER paths from its first element.
func payloadPaths(prefix string, v wire.Value) []string {
	out := []string{}
	for _, k := range v.Obj.SortedKeys() {
		c, _ := v.Obj.Get(k)
		p := prefix + k
		out = append(out, p)
		switch {
		case c.Kind == wire.KindObject:
			out = append(out, payloadPaths(p+".", c)...)
		case c.Kind == wire.KindArray && len(c.Arr) > 0 && c.Arr[0].Kind == wire.KindObject:
			out = append(out, payloadPaths(p+"[].", c.Arr[0])...)
		}
	}
	return out
}

// nullAt returns a copy of the canonical payload with the member at path
// set to null; a NAME[] segment descends into the first element.
func nullAt(t *testing.T, canonical, path string) []byte {
	t.Helper()
	v, err := wire.Parse([]byte(canonical + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	cur := v
	segs := strings.Split(path, ".")
	for _, s := range segs[:len(segs)-1] {
		next, _ := cur.Obj.Get(strings.TrimSuffix(s, "[]"))
		if strings.HasSuffix(s, "[]") {
			next = next.Arr[0]
		}
		cur = next
	}
	cur.Obj.Set(segs[len(segs)-1], wire.Null())
	return wire.Encode(v)
}

// TestCALV0073_TemplateFieldsMatchPayloadNullability is the drift guard:
// every key path of the emitted payload (with one dependency and one
// resource element populated) has a fields entry, fields has no other
// entries than those paths and optionalKeys, and each path's nullable flag
// matches what ticket create actually accepts on the fixture.
func TestCALV0073_TemplateFieldsMatchPayloadNullability(t *testing.T) {
	r := receiptFixture(t)
	item := templateRun(t, r.Root).Items[0]
	fields := field(item, "fields")
	payload := field(item, "payload")
	payload.Obj.Set("title", wire.String("Drift probe"))
	payload.Obj.Set("body", wire.String("probe"))
	payload.Obj.Set("acceptanceCriteria", wire.Strings([]string{"probe"}))
	base := atm(t, r.Root, []byte(string(wire.Encode(payload))), "ticket", "create",
		"--request-id", "drift-dep-target", "--issued-at", "2026-10-04T12:00:00Z", "--payload-stdin")
	if base.res.Outcome != wire.OutcomeOK {
		t.Fatalf("dependency target refused: %+v", base.res)
	}
	dep := wire.NewObject()
	dep.Set("gateId", wire.String("verify"))
	dep.Set("obligation", wire.String("GATE_PASSED"))
	dep.Set("ticketId", wire.String(field(base.res.Items[0], "ticketId").Str))
	payload.Obj.Set("dependencies", wire.Array(wire.ObjectValue(dep)))
	res := wire.NewObject()
	res.Set("class", wire.String("PATH"))
	res.Set("key", wire.String("docs"))
	effects, _ := payload.Obj.Get("effects")
	effects.Obj.Set("resources", wire.Array(wire.ObjectValue(res)))
	canonical := string(wire.Encode(payload))
	if x := atm(t, r.Root, []byte(canonical), "ticket", "create", "--request-id", "drift-base",
		"--issued-at", "2026-10-04T12:00:00Z", "--payload-stdin"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("populated probe refused: %+v", x.res)
	}

	paths := payloadPaths("", payload)
	want := append(slices.Clone(paths), strs(field(item, "optionalKeys"))...)
	slices.Sort(want)
	if got := fields.Obj.SortedKeys(); !slices.Equal(got, want) {
		t.Fatalf("fields keys = %v\nwant payload paths plus optionalKeys = %v", got, want)
	}
	// dependencies[].gateId is conditionally nullable: null with the probe's
	// GATE_PASSED obligation must refuse, and the nullability probe below
	// pairs the null with COMPLETED, as its fields note says.
	if x := atm(t, r.Root, nullAt(t, canonical, "dependencies[].gateId"), "ticket", "create", "--request-id", "drift-gate-passed-null",
		"--issued-at", "2026-10-04T12:00:00Z", "--payload-stdin"); x.res.Outcome == wire.OutcomeOK {
		t.Error("a null gateId with obligation GATE_PASSED was accepted")
	}
	completed := strings.Replace(canonical, `"obligation":"GATE_PASSED"`, `"obligation":"COMPLETED"`, 1)
	for i, p := range paths {
		nullable := field(field(fields, p), "nullable").Bool
		src := canonical
		if p == "dependencies[].gateId" {
			src = completed
		}
		x := atm(t, r.Root, nullAt(t, src, p), "ticket", "create", "--request-id", fmt.Sprintf("drift-null-%d", i),
			"--issued-at", "2026-10-04T12:00:00Z", "--payload-stdin")
		if accepted := x.res.Outcome == wire.OutcomeOK; accepted != nullable {
			t.Errorf("%s: fields nullable=%v but null accepted=%v (%s)", p, nullable, accepted, x.stdout)
		}
	}
}
