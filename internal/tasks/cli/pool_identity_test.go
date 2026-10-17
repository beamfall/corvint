package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const (
	identityDefault = "1111111111111111111111111111111111111111:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	identityOther   = "2222222222222222222222222222222222222222:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	identityForeign = "3333333333333333333333333333333333333333:cccccccccccccccccccccccccccccccccccccccc"
)

func identitySourceValue(pair string) wire.Value {
	rev, tree, _ := strings.Cut(pair, ":")
	return wire.ObjectValue(wire.NewObject().Set("revision", wire.String(rev)).Set("tree", wire.String(tree)))
}

func identityJSON(pair string) string {
	rev, tree, _ := strings.Cut(pair, ":")
	return `{"revision":"` + rev + `","tree":"` + tree + `"}` + "\n"
}

// identityPolicy is the exclusion fixture's policy with pool db declaring
// allowedSources and, per member, an identity command that prints the file
// dir/<member>.
func identityPolicy(dir string, allowed ...string) wire.Value {
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
	cfg := wire.NewObject()
	for _, m := range []string{"a", "b"} {
		command := wire.ObjectValue(wire.NewObject().Set("argv", wire.Strings([]string{"/bin/cat", filepath.Join(dir, m)})).Set("cwd", wire.String("REPOSITORY")).Set("env", wire.Array()).Set("timeoutSeconds", wire.String("5")))
		cfg.Set(m, wire.ObjectValue(wire.NewObject().Set("identity", command)))
	}
	list := []wire.Value{}
	for _, x := range allowed {
		list = append(list, identitySourceValue(x))
	}
	pool := wire.NewObject().Set("id", wire.String("db")).Set("members", wire.Strings([]string{"a", "b"})).Set("memberConfig", wire.ObjectValue(cfg)).Set("allowedSources", wire.Array(list...)).Set("defaultSource", identitySourceValue(identityDefault))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(pool)))
	return v
}

func identityCLIRepo(t *testing.T) (*fixture.Repo, string) {
	t.Helper()
	r := exclusionCLIRepo(t)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(identityPolicy(dir, identityDefault, identityOther)))
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init %s", x.stdout)
	}
	return r, dir
}

func identityPools(t *testing.T, r *fixture.Repo) *snapshot.PoolState {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(r.StateDir, "pools.json"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := snapshot.DecodePools(raw)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func identityClaim(t *testing.T, r *fixture.Repo, name string) (string, string) {
	t.Helper()
	id := planTicket(t, r.Root, name, "P2", `["`+name+`"]`)
	x := atm(t, r.Root, nil, "claim", id, "--holder", "builder", "--request-id", "claim-"+name, "--stage", "implement")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim %s", x.stdout)
	}
	return field(x.res.Items[0], "attemptId").Str, field(x.res.Items[0], "generation").Str
}

func identityStatusMember(t *testing.T, r *fixture.Repo, member string) wire.Value {
	t.Helper()
	x := atm(t, r.Root, nil, "pool", "status", "--member", member)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("status %s", x.stdout)
	}
	return field(field(x.res.Items[0], "pools").Arr[0], "members").Arr[0]
}

// PSR-V0-081..085: acquire refuses a source outside the allow-list, a guarded
// member is FREE only after its identity reports the default pair, every
// failed, malformed or drifted observation keeps it QUARANTINED with the
// reason and both pairs, status shows the last observation, and removing an
// allow-list pair strands no live allocation.
func TestPSRV0081_SourceIdentityGuard(t *testing.T) {
	r, dir := identityCLIRepo(t)
	a, g := identityClaim(t, r, "one")
	acquire := func(request, source string) run {
		return atm(t, r.Root, nil, "pool", "acquire", "--attempt", a, "--generation", g, "--pool", "db", "--request-id", request, "--source", source)
	}
	// PSR-V0-081: outside the allow-list is BLOCKED OUT_OF_SCOPE, typed
	// SOURCE_NOT_ALLOWED, and writes nothing beyond its receipt.
	before := fixture.TreeSnapshot(t, filepath.Join(r.StateDir, "pools.json"))
	x := acquire("foreign", identityForeign)
	if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeOutOfScope) || !bytes.Contains(x.stdout, []byte(transaction.ReasonSourceNotAllowed)) {
		t.Fatalf("foreign source admitted: %s", x.stdout)
	}
	if _, err := os.Stat(filepath.Join(r.StateDir, "pools.json")); err == nil && !fixture.SameTree(before, fixture.TreeSnapshot(t, filepath.Join(r.StateDir, "pools.json"))) {
		t.Fatal("refused acquire changed pools.json")
	}
	if x := acquire("malformed", "not-a-pair"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, "MALFORMED") {
		t.Fatalf("malformed source admitted: %s", x.stdout)
	}
	x = acquire("allowed", identityOther)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("allowed source refused: %s", x.stdout)
	}
	alloc := field(field(x.res.Items[0], "poolAllocation"), "allocationId").Str
	member := field(field(x.res.Items[0], "poolAllocation"), "memberId").Str
	state := identityPools(t, r)
	if len(state.Entries) != 1 || state.Entries[0].Source == nil || state.Entries[0].Source.String() != identityOther {
		t.Fatalf("allocation did not record its source: %+v", state.Entries)
	}
	if s := identityStatusMember(t, r, member); !bytes.Equal(wire.Encode(field(s, "source")), wire.Encode(identitySourceValue(identityOther))) || field(s, "identity").Str != "NOT_OBSERVED" {
		t.Fatalf("status source: %s", wire.Encode(s))
	}

	// PSR-V0-085: a policy update removing the allocation's pair leaves the
	// live allocation in place; new acquires of that pair are refused.
	v := identityPolicy(dir, identityDefault)
	v.Obj.Set("policyVersion", wire.String("2"))
	path := filepath.Join(dir, "policy-2.json")
	fixture.Write(t, path, wire.EncodeFile(v))
	if x := atm(t, r.Root, nil, "policy", "update", "--request-id", "narrow", "--expected-policy-version", "1", "--file", path); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("policy update with a live allocation: %s", x.stdout)
	}
	if st := identityPools(t, r); len(st.Entries) != 1 || st.Entries[0].State != "ALLOCATED" || string(st.Entries[0].AllocationID) != alloc {
		t.Fatalf("narrowing stranded the allocation: %+v", st.Entries)
	}
	a2, g2 := identityClaim(t, r, "two")
	if x := atm(t, r.Root, nil, "pool", "acquire", "--attempt", a2, "--generation", g2, "--pool", "db", "--request-id", "removed", "--source", identityOther); x.res.Outcome == wire.OutcomeOK || !bytes.Contains(x.stdout, []byte(transaction.ReasonSourceNotAllowed)) {
		t.Fatalf("removed pair admitted: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "pool", "release", "--attempt", a, "--generation", g, "--allocation", alloc, "--request-id", "early"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release %s", x.stdout)
	}

	confirm := func(request string) run {
		return atm(t, r.Root, nil, "pool", "confirm-safe", "--member", member, "--allocation", alloc, "--evidence", "local:fixture", "--reason", "reset", "--request-id", request)
	}
	quarantined := func(want string) {
		t.Helper()
		st := identityPools(t, r)
		if len(st.Entries) != 1 || st.Entries[0].State != "QUARANTINED" || !strings.HasPrefix(st.Entries[0].Reason, want) {
			t.Fatalf("member not quarantined with %s: %+v", want, st.Entries)
		}
	}
	// PSR-V0-084: drift. The member still reports the removed pair; the guard
	// applies at release and names both pairs.
	fixture.Write(t, filepath.Join(dir, member), []byte(identityJSON(identityOther)))
	x = confirm("drift")
	if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeQuiescenceUnproved) {
		t.Fatalf("drifted member freed: %s", x.stdout)
	}
	quarantined(transaction.ReasonSourceDrift)
	if reason := identityPools(t, r).Entries[0].Reason; !strings.Contains(reason, identityOther) || !strings.Contains(reason, identityDefault) {
		t.Fatalf("drift reason lacks both pairs: %s", reason)
	}
	s := identityStatusMember(t, r, member)
	idv := field(s, "identity")
	if field(idv, "class").Str != snapshot.PoolIdentityReported || !bytes.Equal(wire.Encode(field(idv, "source")), wire.Encode(identitySourceValue(identityOther))) || field(idv, "observedAt").Str == "" || field(idv, "observedAt").Str == "NOT_OBSERVED" || field(idv, "observedSeq").Str == "0" {
		t.Fatalf("status identity: %s", wire.Encode(s))
	}
	if !strings.HasPrefix(field(field(s, "quarantine"), "reason").Str, transaction.ReasonSourceDrift) {
		t.Fatalf("status quarantine: %s", wire.Encode(s))
	}
	// The same request id replays the refusal and does not free the member.
	if x := confirm("drift"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("replayed drift freed: %s", x.stdout)
	}
	// A failing and a malformed identity command never free the member.
	for _, c := range []struct{ request, content, class string }{
		{"failing", "", "EXIT_NONZERO"},
		{"malformed-output", `{"revision":"1111111111111111111111111111111111111111"}`, "MALFORMED_OUTPUT"},
		{"trailing-output", identityJSON(identityDefault) + "extra", "MALFORMED_OUTPUT"},
		// A stray closing delimiter is a syntax error, not end of input.
		{"trailing-brace", identityJSON(identityDefault) + "}", "MALFORMED_OUTPUT"},
		{"trailing-bracket", identityJSON(identityDefault) + "]", "MALFORMED_OUTPUT"},
	} {
		_ = os.Remove(filepath.Join(dir, member))
		if c.content != "" {
			fixture.Write(t, filepath.Join(dir, member), []byte(c.content))
		}
		if x := confirm(c.request); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("%s freed the member: %s", c.request, x.stdout)
		}
		quarantined(transaction.ReasonIdentityUnproved + ": identity command " + c.class)
		if got := field(field(identityStatusMember(t, r, member), "identity"), "class").Str; got != c.class {
			t.Fatalf("%s class %s", c.request, got)
		}
	}
	// The default pair frees it and keeps the last observation.
	fixture.Write(t, filepath.Join(dir, member), []byte(identityJSON(identityDefault)))
	if x := confirm("matched"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("matching identity refused: %s", x.stdout)
	}
	s = identityStatusMember(t, r, member)
	if field(s, "state").Str != "FREE" || field(field(s, "identity"), "class").Str != snapshot.PoolIdentityReported || field(s, "source").Kind != wire.KindNull {
		t.Fatalf("freed status: %s", wire.Encode(s))
	}
	if x := atm(t, r.Root, nil, "receipt", "audit"); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "structuralConsistency").Str != "CONSISTENT" {
		t.Fatalf("audit %s", x.stdout)
	}
}

// PSR-V0-082: policy declarations are closed and consistent.
func TestPSRV0082_IdentityPolicyRules(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]func(v wire.Value){
		"default not allowed": func(v wire.Value) { pool(v).Obj.Set("defaultSource", identitySourceValue(identityForeign)) },
		"allowed without default": func(v wire.Value) {
			dropKey(pool(v), "defaultSource")
		},
		"duplicate pair": func(v wire.Value) {
			pool(v).Obj.Set("allowedSources", wire.Array(identitySourceValue(identityDefault), identitySourceValue(identityDefault)))
		},
		"member without identity": func(v wire.Value) {
			cfg, _ := pool(v).Obj.Get("memberConfig")
			cfg.Obj.Set("b", wire.ObjectValue(wire.NewObject()))
		},
		"identity without allowedSources": func(v wire.Value) {
			dropKey(pool(v), "allowedSources")
			dropKey(pool(v), "defaultSource")
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := exclusionCLIRepo(t)
			v := identityPolicy(dir, identityDefault)
			mutate(v)
			fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
			if x := atm(t, r.Root, nil, "init"); x.res.Outcome == wire.OutcomeOK {
				t.Fatalf("policy accepted: %s", wire.EncodeFile(v))
			}
		})
	}
}

func dropKey(v wire.Value, key string) {
	delete(v.Obj.Vals, key)
	keys := []string{}
	for _, k := range v.Obj.Keys {
		if k != key {
			keys = append(keys, k)
		}
	}
	v.Obj.Keys = keys
}

func pool(v wire.Value) wire.Value {
	pools, _ := v.Obj.Get("pools")
	return pools.Arr[0]
}

// PSR-V0-086: a pool without allowedSources is unchanged: no source or
// identity keys in pools.json or pool status, and --source is refused.
func TestPSRV0086_PoolsWithoutSourcesUnchanged(t *testing.T) {
	r := exclusionCLIRepo(t)
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	a, g := identityClaim(t, r, "one")
	if x := atm(t, r.Root, nil, "pool", "acquire", "--attempt", a, "--generation", g, "--pool", "db", "--request-id", "src", "--source", identityDefault); x.res.Outcome == wire.OutcomeOK || !bytes.Contains(x.stdout, []byte(transaction.ReasonSourceNotAllowed)) {
		t.Fatalf("--source admitted on a pool without allowedSources: %s", x.stdout)
	}
	x := atm(t, r.Root, nil, "pool", "acquire", "--attempt", a, "--generation", g, "--pool", "db", "--request-id", "plain")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("plain acquire %s", x.stdout)
	}
	raw, err := os.ReadFile(filepath.Join(r.StateDir, "pools.json"))
	if err != nil || bytes.Contains(raw, []byte(`"source"`)) || bytes.Contains(raw, []byte(`"identities"`)) {
		t.Fatalf("legacy pools.json gained keys: %v %s", err, raw)
	}
	s := atm(t, r.Root, nil, "pool", "status")
	if bytes.Contains(s.stdout, []byte(`"defaultSource"`)) || bytes.Contains(s.stdout, []byte(`"identity"`)) || bytes.Contains(s.stdout, []byte(`"source"`)) {
		t.Fatalf("legacy pool status gained keys: %s", s.stdout)
	}
}

// PSR-V0-085: a lane-untouched release of a source-guarded member does not
// skip the guard; the member is quarantined until its identity matches.
func TestPSRV0085_LaneUntouchedKeepsIdentityGuard(t *testing.T) {
	r := exclusionCLIRepo(t)
	q := fixture.QueueValue()
	q.Obj.Set("fixture", wire.Bool(false))
	q.Obj.Set("canonicalWriter", wire.String("NATIVE"))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), wire.EncodeFile(q))
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(identityPolicy(dir, identityDefault)))
	ok := func(args ...string) run {
		t.Helper()
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v %s", args, x.stdout)
		}
		return x
	}
	ok("init")
	var qualification strings.Builder
	qualification.WriteString(`{"Action":"start","Package":"` + transaction.QualificationPackage + `"}` + "\n")
	for _, name := range transaction.QualificationSuite {
		for _, action := range []string{"run", "pass"} {
			qualification.WriteString(`{"Action":"` + action + `","Package":"` + transaction.QualificationPackage + `","Test":"` + name + `"}` + "\n")
		}
	}
	qualification.WriteString(`{"Action":"pass","Package":"` + transaction.QualificationPackage + `"}` + "\n")
	qp := filepath.Join(dir, "synthetic-cutover.jsonl")
	fixture.Write(t, qp, []byte(qualification.String()))
	ok("cutover", "--execution", "--decision", "test-only-identity", "--qualification", qp)
	id := field(ok("ticket", "create", "--request-id", "ticket", "--payload", createPayloadJSON).res.Items[0], "ticketId").Str
	item := ok("claim", id, "--holder", "builder", "--request-id", "claim", "--scope", "src", "--pool", "db", "--stage", "implement").res.Items[0]
	alloc := field(item, "poolAllocation")
	ok("release", "--attempt", field(item, "attemptId").Str, "--generation", field(item, "generation").Str, "--request-id", "untouched", "--lane-untouched", "--evidence", "local:unused")
	st := identityPools(t, r)
	if len(st.Entries) != 1 || st.Entries[0].State != "QUARANTINED" || string(st.Entries[0].AllocationID) != field(alloc, "allocationId").Str {
		t.Fatalf("lane-untouched release freed a guarded member: %+v", st.Entries)
	}
	member := field(alloc, "memberId").Str
	fixture.Write(t, filepath.Join(dir, member), []byte(identityJSON(identityDefault)))
	ok("pool", "confirm-safe", "--member", member, "--allocation", field(alloc, "allocationId").Str, "--evidence", "local:fixture", "--reason", "reset", "--request-id", "safe")
	if st := identityPools(t, r); len(st.Entries) != 0 || len(st.Identities) != 1 {
		t.Fatalf("matching identity did not free the member: %+v", st)
	}
}
