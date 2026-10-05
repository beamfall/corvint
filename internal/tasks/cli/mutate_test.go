package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// createPayloadJSON is the closed §3.3 CREATE payload as canonical JSON:
// keys sorted, no insignificant whitespace, exactly as the wire codec admits.
const createPayloadJSON = `{"acceptanceCriteria":["it exists"],"body":null,"capabilities":[],` +
	`"dependencies":[],"dueDate":null,"effects":{"coverage":"QUALIFIED","externalUnbounded":false,` +
	`"resources":[],"touchPaths":[]},"estimateMinutes":null,"executionClass":"AUTONOMOUS",` +
	`"kind":"FEATURE","labels":[],"milestone":null,"order":"0","owner":null,"priority":"P2",` +
	`"requiredGates":[],"requirementRefs":[],"source":{"kind":"NATIVE","sourceItemId":null,` +
	`"sourceQueueId":"queue:acme:main","sourceRevisionSha256":null},"supersededBy":null,` +
	`"supersedes":null,"title":"Console ticket"}`

// TestTMV0008_AS02_TicketCreateIsVisibleToTheReadVerbs is TCP-02b end to
// end: a ticket the CLI creates is committed to the journal and then listed
// by the ordinary read path, which is what a board needs in order to have
// anything to show.
func TestTMV0008_AS02_TicketCreateIsVisibleToTheReadVerbs(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}

	empty := atm(t, r.Root, nil, "ticket", "list")
	if len(empty.res.Items) != 0 {
		t.Fatalf("a fresh queue already lists %d tickets", len(empty.res.Items))
	}

	created := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-1", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %+v", created.res)
	}
	id := field(created.res.Items[0], "ticketId").Str
	if !strings.HasPrefix(id, "ticket:acme:main:") {
		t.Fatalf("ticketId = %q, so the caller cannot learn its allocated id", id)
	}
	if got := field(created.res.Items[0], "receipt").Str; got != "000000000002.json" {
		t.Errorf("receipt = %q, want the second receipt", got)
	}

	listed := atm(t, r.Root, nil, "ticket", "list")
	if listed.res.Outcome != wire.OutcomeOK || len(listed.res.Items) != 1 {
		t.Fatalf("ticket list after create: %+v", listed.res)
	}
	if got := field(listed.res.Items[0], "ticketId").Str; got != id {
		t.Errorf("listed %q, created %q", got, id)
	}
}

// TestTMV0006_AS03_TicketCreateRetryReplays checks the idempotency key at
// the CLI boundary: re-running the identical command replays instead of
// creating a second ticket.
func TestTMV0006_AS03_TicketCreateRetryReplays(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	atm(t, r.Root, nil, "init")

	args := []string{"ticket", "create", "--request-id", "req-1",
		"--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON}
	first := atm(t, r.Root, nil, args...)
	if first.res.Outcome != wire.OutcomeOK {
		t.Fatalf("first create: %+v", first.res)
	}
	again := atm(t, r.Root, nil, args...)
	if again.res.Outcome != wire.OutcomeOK {
		t.Fatalf("retry: %+v", again.res)
	}
	if field(first.res.Items[0], "ticketId").Str != field(again.res.Items[0], "ticketId").Str {
		t.Fatal("replay lost allocated ticket identity")
	}
	if !field(again.res.Items[0], "replayed").Bool {
		t.Error("an identical retry was not replayed")
	}
	if got := field(again.res.Items[0], "receipt").Str; got != "" {
		t.Errorf("a replay reported receipt %q, so it wrote a second one", got)
	}
	listed := atm(t, r.Root, nil, "ticket", "list")
	if len(listed.res.Items) != 1 {
		t.Errorf("the queue holds %d tickets after a replayed retry, want 1", len(listed.res.Items))
	}
}

// TestTMV0008_AS07_MutationsRefuseOnAnUninitializedStore keeps the
// uninitialized refusal honest: a mutation verb must report that there is no
// store, not create one implicitly.
func TestTMV0008_AS07_MutationsRefuseOnAnUninitializedStore(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())

	x := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-1", "--payload", createPayloadJSON)
	if x.res.Outcome == wire.OutcomeOK {
		t.Fatal("a mutation succeeded against an uninitialized store")
	}
	if len(x.res.Codes) == 0 || x.res.Codes[0] != wire.CodeUninitialized {
		t.Errorf("codes = %v, want %s", x.res.Codes, wire.CodeUninitialized)
	}
}

// TestMutationTargetTakesTheLocalIDAndHelpNamesThePayload is V1-0329: a
// mutation resolves the local ticket ID that `ticket show` accepts, and a
// verb's --help names the payload keys its refusal would otherwise demand.
func TestMutationTargetTakesTheLocalIDAndHelpNamesThePayload(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	atm(t, r.Root, nil, "init")
	created := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-1", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %+v", created.res)
	}
	id := field(created.res.Items[0], "ticketId").Str
	local := id[strings.LastIndex(id, ":")+1:]
	revision := field(created.res.Items[0], "resultingRevision").Str

	moved := atm(t, r.Root, nil, "ticket", "prioritize", "--request-id", "req-2",
		"--target", local, "--expected-revision", revision, "--payload", `{"order":"1","priority":"P1"}`)
	if moved.res.Outcome != wire.OutcomeOK {
		t.Fatalf("prioritize --target %s: %+v", local, moved.res)
	}
	if got := field(moved.res.Items[0], "ticketId").Str; got != id {
		t.Errorf("prioritize --target %s changed %q, want %q", local, got, id)
	}

	help := atm(t, r.Root, nil, "ticket", "prioritize", "--help")
	if help.res.Outcome != wire.OutcomeOK || len(help.res.Items) != 1 {
		t.Fatalf("ticket prioritize --help: %+v", help.res)
	}
	keys := []string{}
	for _, k := range field(help.res.Items[0], "payloadKeys").Arr {
		keys = append(keys, k.Str)
	}
	if strings.Join(keys, ",") != "order,priority" {
		t.Errorf("payloadKeys = %v, want [order priority]", keys)
	}
}

func TestCALV0043_CLIRecoveryAndPreview(t *testing.T) {
	t.Run("CAL-V0-043 operator recovery CLI", func(t *testing.T) {
		root, old := expiredCLIStore(t, 1)
		ok := func(args ...string) wire.Value {
			t.Helper()
			r := atm(t, root, nil, args...)
			if r.res.Outcome != wire.OutcomeOK {
				t.Fatalf("%v: %+v", args, r.res)
			}
			if len(r.res.Items) == 0 {
				return wire.Null()
			}
			return r.res.Items[0]
		}
		ok("reap", "--request-id", "reap-for-recovery")
		id := old[0].Ticket
		for i := 1; i <= 3; i++ {
			a := ok("claim", id, "--holder", "recovery-agent", "--request-id", fmt.Sprintf("retry-%d", i), "--scope", "src")
			ok("release", "--attempt", field(a, "attemptId").Str, "--generation", field(a, "generation").Str, "--request-id", fmt.Sprintf("cancel-%d", i))
		}
		preview := ok("plan", "preview")
		if !strings.Contains(string(wire.Encode(preview)), wire.CodeRetryExhausted) {
			t.Fatalf("missing preview exhaustion: %s", wire.Encode(preview))
		}
		refusal := func(args ...string) {
			t.Helper()
			r := atm(t, root, nil, args...)
			if r.res.Outcome == wire.OutcomeOK {
				t.Fatalf("accepted %v", args)
			}
		}
		common := []string{"ticket", "reopen", "--target", id, "--expected-revision", "1", "--payload", `{"reason":"owner permits another attempt"}`, "--issued-at", "2026-09-29T00:00:00Z"}
		refusal(append(append([]string{}, common...), "--request-id", "operator", "--role", "OPERATOR")...)
		refusal("ticket", "reopen", "--target", id, "--request-id", "missing-revision", "--payload", `{"reason":"recover"}`)
		refusal("ticket", "reopen", "--target", id, "--expected-revision", "1", "--request-id", "missing-reason", "--payload", `{}`)
		for i, reason := range []string{"", "   "} {
			refusal("ticket", "reopen", "--target", id, "--expected-revision", "1", "--request-id", fmt.Sprintf("blank-%d", i), "--payload", string(wire.Encode(wire.ObjectValue(wire.NewObject().Set("reason", wire.String(reason))))))
		}
		first := ok(append(append([]string{}, common...), "--request-id", "owner-recovery", "--role", "OWNER")...)
		if field(first, "resultingAcceptanceRevision").Str != "2" {
			t.Fatalf("no fresh acceptance: %s", wire.Encode(first))
		}
		again := ok(append(append([]string{}, common...), "--request-id", "owner-recovery", "--role", "OWNER")...)
		if !field(again, "replayed").Bool {
			t.Fatal("not replayed")
		}
		preview = ok("plan", "preview")
		if strings.Contains(string(wire.Encode(preview)), wire.CodeRetryExhausted) {
			t.Fatalf("exhaustion remains: %s", wire.Encode(preview))
		}
		a := ok("claim", id, "--holder", "fresh-agent", "--request-id", "fresh", "--scope", "src")
		if field(a, "attemptId").Str == old[0].AttemptID {
			t.Fatal("old attempt reused")
		}
		ok("receipt", "audit")
	})
}

// v10750Repo is an initialized fixture store for the V1-0750 cases.
func v10750Repo(t *testing.T) string {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	return r.Root
}

// reverseKeys renders a canonical JSON document compactly with every
// object's keys in reverse byte order, string bytes untouched.
func reverseKeys(t *testing.T, canonical string) string {
	t.Helper()
	v, err := wire.Parse([]byte(canonical + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	var render func(v wire.Value) string
	render = func(v wire.Value) string {
		switch v.Kind {
		case wire.KindObject:
			keys := v.Obj.SortedKeys()
			parts := make([]string, 0, len(keys))
			for i := len(keys) - 1; i >= 0; i-- {
				parts = append(parts, string(wire.Encode(wire.String(keys[i])))+":"+render(v.Obj.Vals[keys[i]]))
			}
			return "{" + strings.Join(parts, ",") + "}"
		case wire.KindArray:
			parts := make([]string, len(v.Arr))
			for i, e := range v.Arr {
				parts[i] = render(e)
			}
			return "[" + strings.Join(parts, ",") + "]"
		}
		return string(wire.Encode(v))
	}
	return render(v)
}

func indentJSON(t *testing.T, canonical, sep string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(canonical), "", " "); err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(buf.String(), "\n", sep)
}

// TestV10750_MutationPayloadsAreCanonicalizedBeforeTheDigest is V1-0750:
// ticket create and ticket refine accept a semantically valid payload with
// whitespace, unsorted object keys or unsorted set arrays, and record the
// same request digest as the canonical form. The proof is the idempotency
// key: a retry with the canonical payload under the same request ID and
// issuedAt replays, which it does only when the envelope digests are equal.
func TestV10750_MutationPayloadsAreCanonicalizedBeforeTheDigest(t *testing.T) {
	sets := strings.NewReplacer(`"labels":[]`, `"labels":["alpha","zeta"]`,
		`"requirementRefs":[]`, `"requirementRefs":["REQ-1","REQ-2"]`,
		`"touchPaths":[]`, `"touchPaths":["a/y.go","b/x.go"]`)
	canonicalSets := sets.Replace(createPayloadJSON)
	unsortedSets := strings.NewReplacer(`["alpha","zeta"]`, `["zeta","alpha"]`,
		`["REQ-1","REQ-2"]`, `["REQ-2","REQ-1"]`, `["a/y.go","b/x.go"]`, `["b/x.go","a/y.go"]`).Replace(canonicalSets)
	cases := []struct {
		name, variant, canonical string
	}{
		{"pretty-printed with newlines", indentJSON(t, createPayloadJSON, "\n") + "\n", createPayloadJSON},
		{"pretty-printed folded to spaces", indentJSON(t, createPayloadJSON, " "), createPayloadJSON},
		{"default separators", strings.NewReplacer(`,"`, `, "`, `":`, `": `).Replace(createPayloadJSON), createPayloadJSON},
		{"reversed object keys", reverseKeys(t, createPayloadJSON), createPayloadJSON},
		{"unsorted set arrays", unsortedSets, canonicalSets},
		{"all at once", indentJSON(t, reverseKeys(t, unsortedSets), " "), canonicalSets},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.variant == c.canonical {
				t.Fatal("the variant is already canonical")
			}
			root := v10750Repo(t)
			args := []string{"ticket", "create", "--request-id", "req-1", "--issued-at", "2026-09-07T12:00:00Z"}
			first := atm(t, root, []byte(c.variant), append(args, "--payload-stdin")...)
			if first.res.Outcome != wire.OutcomeOK || field(first.res.Items[0], "replayed").Bool {
				t.Fatalf("variant create: %+v", first.res)
			}
			again := atm(t, root, nil, append(args, "--payload", c.canonical)...)
			if again.res.Outcome != wire.OutcomeOK || !field(again.res.Items[0], "replayed").Bool {
				t.Fatalf("the canonical retry did not replay, so the digests differ: %+v", again.res)
			}
		})
	}

	t.Run("refine", func(t *testing.T) {
		root := v10750Repo(t)
		created := atm(t, root, nil, "ticket", "create", "--request-id", "req-1",
			"--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
		if created.res.Outcome != wire.OutcomeOK {
			t.Fatalf("create: %+v", created.res)
		}
		id := field(created.res.Items[0], "ticketId").Str
		args := []string{"ticket", "refine", "--request-id", "req-2", "--target", id,
			"--expected-revision", field(created.res.Items[0], "resultingRevision").Str,
			"--issued-at", "2026-09-07T12:01:00Z"}
		variant := "{\n \"title\": \"Refined\",\n \"requirementRefs\": [\"REQ-9\", \"REQ-1\"],\n \"labels\": [\"zeta\", \"alpha\"]\n}\n"
		first := atm(t, root, []byte(variant), append(args, "--payload-stdin")...)
		if first.res.Outcome != wire.OutcomeOK || field(first.res.Items[0], "replayed").Bool {
			t.Fatalf("variant refine: %+v", first.res)
		}
		canonical := `{"labels":["alpha","zeta"],"requirementRefs":["REQ-1","REQ-9"],"title":"Refined"}`
		again := atm(t, root, nil, append(args, "--payload", canonical)...)
		if again.res.Outcome != wire.OutcomeOK || !field(again.res.Items[0], "replayed").Bool {
			t.Fatalf("the canonical refine retry did not replay: %+v", again.res)
		}
	})
}

// TestV10750_OrderedArraysKeepTheirOrderAndSetDuplicatesRefuse: only arrays
// the closed schema reads as sets are sorted. acceptanceCriteria is ordered,
// so its given order is recorded and a retry that sorts it is a different
// request; a duplicate set element is refused with its path.
func TestV10750_OrderedArraysKeepTheirOrderAndSetDuplicatesRefuse(t *testing.T) {
	root := v10750Repo(t)
	ordered := strings.Replace(createPayloadJSON, `"acceptanceCriteria":["it exists"]`,
		`"acceptanceCriteria":["second step","first step"]`, 1)
	args := []string{"ticket", "create", "--request-id", "req-1", "--issued-at", "2026-09-07T12:00:00Z"}
	created := atm(t, root, nil, append(args, "--payload", indentJSON(t, ordered, " "))...)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %+v", created.res)
	}
	shown := atm(t, root, nil, "ticket", "show", field(created.res.Items[0], "ticketId").Str)
	if shown.res.Outcome != wire.OutcomeOK {
		t.Fatalf("show: %+v", shown.res)
	}
	criteria := field(field(shown.res.Items[0], "record"), "acceptanceCriteria")
	if got := string(wire.Encode(criteria)); got != `["second step","first step"]` {
		t.Errorf("acceptanceCriteria recorded as %s, want the given order", got)
	}
	sorted := strings.Replace(ordered, `["second step","first step"]`, `["first step","second step"]`, 1)
	if again := atm(t, root, nil, append(args, "--payload", sorted)...); again.res.Outcome == wire.OutcomeOK && field(again.res.Items[0], "replayed").Bool {
		t.Error("sorting an ordered array replayed the original request, so the order was not significant")
	}

	dup := strings.Replace(createPayloadJSON, `"labels":[]`, `"labels":["b","a","b"]`, 1)
	x := atm(t, root, nil, "ticket", "create", "--request-id", "req-dup", "--payload", dup)
	if x.res.Outcome == wire.OutcomeOK {
		t.Fatal("a duplicate set element was accepted")
	}
	if msg := strings.Join(x.res.Warnings, " ") + fmt.Sprint(x.res.Items); !strings.Contains(msg, "/payload/labels") {
		t.Errorf("duplicate refusal does not name /payload/labels: %+v", x.res)
	}
}

// TestV10750_ReleasePayloadsCanonicalizeFramingButKeepSetOrderStrict is
// V1-0750 for the release verbs: a release payload with unsorted object keys
// and non-canonical string escapes is accepted and records the canonical
// request digest (the canonical retry replays), while a release set array
// given out of order is refused with a message naming its path and the fix.
// Release payloads do not sort sets for the caller.
func TestV10750_ReleasePayloadsCanonicalizeFramingButKeepSetOrderStrict(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), releaseQueueBytes(true))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	t1 := createTicket(t, r.Root, "release-ticket-one")
	t2 := createTicket(t, r.Root, "release-ticket-two")

	canonical := releaseCreatePayload("v0-9", "Version 0.9/A", t1, "")
	loose := fmt.Sprintf(`{"version":"v0-9","title":"Version 0.9\/A","ticketIds":[%q],"requiredGates":[],"predecessorReleaseIds":[],"acceptanceCriteria":["release criterion"]}`, t1)
	args := func(payload string) []string {
		return []string{"release", "create", "--request-id", "release-canonical", "--target", "v0-9", "--issued-at", "2026-09-20T12:01:00Z", "--payload", payload}
	}
	if x := atm(t, r.Root, nil, args(loose)...); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "replayed").Bool {
		t.Fatalf("release payload with unsorted keys and escapes: %+v", x.res)
	}
	if x := atm(t, r.Root, nil, args(canonical)...); x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "replayed").Bool {
		t.Fatalf("canonical retry did not replay, so the digests differ: %+v", x.res)
	}
	if x := atm(t, r.Root, nil, args(releaseCreatePayload("v0-9", "Version 0.9/B", t1, ""))...); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("a different payload under the same request replayed, so the replay proves nothing: %+v", x.res)
	}

	lo, hi := t1, t2
	if hi < lo {
		lo, hi = hi, lo
	}
	unsorted := strings.Replace(releaseCreatePayload("v1-0", "Version 1.0", t1, ""), fmt.Sprintf(`"ticketIds":[%q]`, t1), fmt.Sprintf(`"ticketIds":[%q,%q]`, hi, lo), 1)
	x := atm(t, r.Root, nil, "release", "create", "--request-id", "release-unsorted-set", "--target", "v1-0", "--issued-at", "2026-09-20T12:02:00Z", "--payload", unsorted)
	if x.res.Outcome == wire.OutcomeOK {
		t.Fatal("a release payload with an unsorted set array was accepted")
	}
	msg := strings.Join(x.res.Warnings, " ") + fmt.Sprint(x.res.Items)
	if !strings.Contains(msg, "/payload/ticketIds") || !strings.Contains(msg, "sort its elements") {
		t.Errorf("unsorted set refusal does not name /payload/ticketIds and the fix: %+v", x.res)
	}
}
