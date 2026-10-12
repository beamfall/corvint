package cli_test

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sourceReviewPolicy opts into obligations.requireSourceReview.
func sourceReviewPolicy(p wire.Value) {
	p.Obj.Set("obligations", wire.ObjectValue(wire.NewObject().Set("workerWitness", wire.Bool(false)).Set("requireSourceReview", wire.Bool(true))))
}

func setSources(t *testing.T, root, id, req, changes string) {
	t.Helper()
	x := atm(t, root, nil, "ticket", "obligations", "set", "--target", id, "--expected-revision", obligationRevision(t, root, id),
		"--request-id", req, "--issued-at", "2026-10-11T10:00:00Z", "--payload", `{"changes":`+changes+`}`)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("set %s: %s", req, x.stdout)
	}
}

// sourceFindings renders preflight's source-review findings as kind:id.
func sourceFindings(t *testing.T, v wire.Value) string {
	t.Helper()
	var out []string
	for _, f := range field(v, "findings").Arr {
		if k := field(f, "kind").Str; strings.HasPrefix(k, "SOURCE_REVIEW_") {
			if field(f, "remedy").Str == "" || !strings.Contains(field(f, "detail").Str, field(f, "id").Str) {
				t.Fatalf("finding without a remedy or the obligation: %s", wire.Encode(f))
			}
			out = append(out, k+":"+field(f, "id").Str)
		}
	}
	return strings.Join(out, ",")
}

var reviewChanges = `[{"id":"AC-1","state":null,"core":null,"reason":"reviewed C7","sources":[{"kind":"legacy-case","ref":"C7","digest":"` + digestA + `"}]},` +
	`{"id":"AC-2","state":null,"core":null,"reason":"searched","sources":[{"kind":"none","query":"soft step","searched":["testrail-json:sha256:` + digestA + `"],"at":"2026-10-11T09:00:00Z"}]},` +
	`{"id":"AC-3","state":null,"core":null,"reason":"no query","sources":[{"kind":"none","query":" \u00a0\u2003","searched":["testrail-json:sha256:` + digestA + `"],"at":"2026-10-11T09:00:00Z"}]},` +
	`{"id":"AC-4","state":null,"core":null,"reason":"nothing searched","sources":[{"kind":"none","query":"conflicting","searched":[],"at":"2026-10-11T09:00:00Z"}]},` +
	`{"id":"AC-9","state":"DEFERRED","core":null,"reason":"later"}]`

var digestA = strings.Repeat("a", 64)

// TestTOLV0040_PreflightRequiresSourceReview: under requireSourceReview,
// preflight refuses each OPEN obligation without sources
// (SOURCE_REVIEW_MISSING) and each whose none source has a blank query or an
// empty searched set (SOURCE_REVIEW_NONE_UNSEARCHED), naming the id, with
// Unicode whitespace counted as blank; a DEFERRED obligation is not checked;
// a secret in a source ref, query or searched id refuses SECRET_DETECTED. Without the policy the same ledger
// has no source-review finding.
func TestTOLV0040_PreflightRequiresSourceReview(t *testing.T) {
	r, id, _ := obligationRepoWith(t, sourceReviewPolicy)
	seedObligations(t, r.Root, id)
	setSources(t, r.Root, id, "set-sources", reviewChanges)
	state := fixture.TreeSnapshot(t, r.StateDir)
	x := atm(t, r.Root, nil, "preflight", id)
	if x.res.Outcome == wire.OutcomeOK || x.code == 0 || field(x.res.Items[0], "status").Str != "PREFLIGHT_FAILED" {
		t.Fatalf("preflight passed: %s", x.stdout)
	}
	want := "SOURCE_REVIEW_MISSING:AC-5,SOURCE_REVIEW_MISSING:AC-6,SOURCE_REVIEW_MISSING:AC-7,SOURCE_REVIEW_MISSING:AC-8," +
		"SOURCE_REVIEW_NONE_UNSEARCHED:AC-3,SOURCE_REVIEW_NONE_UNSEARCHED:AC-4"
	if got := sourceFindings(t, x.res.Items[0]); got != want {
		t.Fatalf("findings\n got %s\nwant %s", got, want)
	}
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("preflight wrote state")
	}

	for name, src := range map[string]string{
		"ref":      `{"kind":"legacy-case","ref":"C7 AKIAABCDEFGHIJKLMNOP","digest":"` + digestA + `"}`,
		"query":    `{"kind":"none","query":"AKIAABCDEFGHIJKLMNOP","searched":["s"],"at":"2026-10-11T09:00:00Z"}`,
		"searched": `{"kind":"none","query":"q","searched":["AKIAABCDEFGHIJKLMNOP"],"at":"2026-10-11T09:00:00Z"}`,
	} {
		x := atm(t, r.Root, nil, "ticket", "obligations", "set", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
			"--request-id", "set-secret-"+name, "--payload", `{"changes":[{"id":"AC-5","state":null,"core":null,"reason":"r","sources":[`+src+`]}]}`)
		if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), wire.CodeSecretDetected) || strings.Contains(string(x.stdout), "AKIAABCDEFGHIJKLMNOP") {
			t.Fatalf("secret %s: %s", name, x.stdout)
		}
	}

	off, offID, _ := obligationRepo(t)
	seedObligations(t, off.Root, offID)
	setSources(t, off.Root, offID, "set-sources", reviewChanges)
	if got := sourceFindings(t, atm(t, off.Root, nil, "preflight", offID).res.Items[0]); got != "" {
		t.Fatalf("policy off still reviewed sources: %s", got)
	}
}

// TestTOLV0039_PolicyLeavesLedgerBytes: the same writes without sources
// fold to the same entry bytes with the policy on and off, show and plan
// add no sourceReview member, and a policy without the key decodes as off.
func TestTOLV0039_PolicyLeavesLedgerBytes(t *testing.T) {
	entries := func(edit func(wire.Value)) (string, wire.Value) {
		r, id, _ := obligationRepoWith(t, edit)
		seedObligations(t, r.Root, id)
		setSources(t, r.Root, id, "set-1", `[{"id":"AC-2","state":"DEFECT","core":null,"reason":"bug"}]`)
		v := obligationsShow(t, r.Root, id)
		return string(wire.Encode(field(v, "entries"))), v
	}
	plain, v := entries(nil)
	on, _ := entries(sourceReviewPolicy)
	if plain != on || strings.Contains(plain, "sources") {
		t.Fatalf("policy changed ledger bytes\noff %s\non  %s", plain, on)
	}
	if _, ok := v.Obj.Get("sourceReview"); ok {
		t.Fatal("show added sourceReview to a ledger without sources")
	}
	decode := func(o wire.Value) (*intent.Policy, error) {
		p := fixture.PolicyValue()
		p.Obj.Set("obligations", o)
		return intent.DecodePolicy(wire.EncodeFile(p))
	}
	if pol, err := decode(wire.ObjectValue(wire.NewObject().Set("workerWitness", wire.Bool(true)))); err != nil || pol.ObligationSourceReviewRequired() {
		t.Fatalf("a policy without requireSourceReview: %v", err)
	}
	p := fixture.PolicyValue()
	sourceReviewPolicy(p)
	if pol, err := intent.DecodePolicy(wire.EncodeFile(p)); err != nil || !pol.ObligationSourceReviewRequired() || pol.WorkerObligationWitness() {
		t.Fatalf("requireSourceReview: %v", err)
	}
	if _, err := decode(wire.ObjectValue(wire.NewObject().Set("workerWitness", wire.Bool(false)).Set("requireSourceReview", wire.String("yes")))); err == nil {
		t.Fatal("a string requireSourceReview decoded")
	}
}

// TestTOLV0042_StaleSourcesListed: show and plan list a legacy-case source
// whose digest in the --legacy-cases record differs from the reviewed one,
// and a legacy-spec source whose file at HEAD changed, as stale; a WITNESSED
// obligation stays WITNESSED; an unreadable source is unresolved with the
// reason; plan's ok is unchanged; neither read writes state.
func TestTOLV0042_StaleSourcesListed(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	sum := sha256.Sum256([]byte(obligationSpec))
	specDigest := hex.EncodeToString(sum[:])
	setSources(t, r.Root, id, "set-sources",
		`[{"id":"AC-1","state":null,"core":null,"reason":"reviewed","sources":[{"kind":"legacy-case","ref":"C7","digest":"`+digestA+`"}]},`+
			`{"id":"AC-2","state":null,"core":null,"reason":"reviewed","sources":[{"kind":"legacy-spec","ref":"e2e/login.spec.ts","digest":"`+specDigest+`"},{"kind":"manual-case","ref":"C99","digest":"`+digestA+`"}]}]`)
	if x := atm(t, r.Root, nil, declareArgs(id, "w-1", obligationRevision(t, r.Root, id), head, "AC-1")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness: %s", x.stdout)
	}
	cases := filepath.Join(t.TempDir(), "cases.json")
	fixture.Write(t, cases, []byte(`{"schema":"corvint-legacy-cases/1","content":"ids-paths-titles","cases":[`+
		`{"legacy":{"id":"C7","state":"manual"},"path":["Billing"],"digest":"`+strings.Repeat("b", 64)+`"}]}`))
	review := func(args ...string) (wire.Value, wire.Value) {
		t.Helper()
		x := atm(t, r.Root, nil, append([]string{"ticket", "obligations", "show", "--target", id}, args...)...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("show: %s", x.stdout)
		}
		sr, ok := x.res.Items[0].Obj.Get("sourceReview")
		if !ok {
			t.Fatalf("no sourceReview: %s", x.stdout)
		}
		return sr, x.res.Items[0]
	}
	list := func(v wire.Value, key string) string {
		var out []string
		for _, it := range field(v, key).Arr {
			out = append(out, field(it, "id").Str+":"+field(it, "state").Str+":"+field(it, "kind").Str+":"+field(it, "ref").Str)
		}
		return strings.Join(out, ",")
	}
	state := fixture.TreeSnapshot(t, r.StateDir)
	sr, item := review("--legacy-cases", cases)
	if list(sr, "stale") != "AC-1:WITNESSED:legacy-case:C7" || list(sr, "unresolved") != "AC-2:OPEN:manual-case:C99" {
		t.Fatalf("source review: %s", wire.Encode(sr))
	}
	if c := field(sr, "stale").Arr[0]; field(c, "reviewed").Str != digestA || field(c, "current").Str != strings.Repeat("b", 64) {
		t.Fatalf("stale digests: %s", wire.Encode(c))
	}
	if entryStates(item)["AC-1"] != "WITNESSED" {
		t.Fatal("a stale source demoted WITNESSED")
	}
	if sr, _ := review(); list(sr, "stale") != "" || list(sr, "unresolved") != "AC-1:WITNESSED:legacy-case:C7,AC-2:OPEN:manual-case:C99" {
		t.Fatalf("without --legacy-cases: %s", wire.Encode(sr))
	}
	fixture.Write(t, filepath.Join(r.Root, "e2e", "login.spec.ts"), []byte(obligationSpec+"// edited\n"))
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-am", "edit spec")
	sr, _ = review("--legacy-cases", cases)
	if list(sr, "stale") != "AC-1:WITNESSED:legacy-case:C7,AC-2:OPEN:legacy-spec:e2e/login.spec.ts" || field(sr, "specCommit").Str != headOID(t, r.Root) {
		t.Fatalf("legacy spec change: %s", wire.Encode(sr))
	}
	plan := filepath.Join(t.TempDir(), "plan.json")
	fixture.Write(t, plan, []byte(`{"profile":"taskman-obligation-plan/0","ticketId":"`+id+`","tests":[{"test":"a","project":"chromium","obligations":["AC-2","AC-3","AC-4","AC-5","AC-6","AC-7","AC-8","AC-9"]}]}`))
	p := atm(t, r.Root, nil, "ticket", "obligations", "plan", "--target", id, "--plan", plan, "--legacy-cases", cases)
	if p.res.Outcome != wire.OutcomeOK || !field(p.res.Items[0], "ok").Bool || list(field(p.res.Items[0], "sourceReview"), "stale") == "" {
		t.Fatalf("plan: %s", p.stdout)
	}
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("show or plan wrote state")
	}
	// Another schema, trailing data after the record and a repeated member
	// are each refused rather than read one way (TOL-V0-043).
	b64 := strings.Repeat("b", 64)
	for name, raw := range map[string]string{
		"another schema":  `{"schema":"corvint-legacy-cases/0","cases":[]}`,
		"trailing data":   `{"schema":"corvint-legacy-cases/1","cases":[]} {"schema":"x"}`,
		"repeated digest": `{"schema":"corvint-legacy-cases/1","cases":[{"legacy":{"id":"C7"},"digest":"` + b64 + `","digest":"` + digestA + `"}]}`,
		"repeated schema": `{"schema":"corvint-legacy-cases/1","schema":"corvint-legacy-cases/1","cases":[]}`,
	} {
		bad := filepath.Join(t.TempDir(), "bad.json")
		fixture.Write(t, bad, []byte(raw))
		if x := atm(t, r.Root, nil, "ticket", "obligations", "show", "--target", id, "--legacy-cases", bad); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("%s accepted: %s", name, x.stdout)
		}
	}
}
