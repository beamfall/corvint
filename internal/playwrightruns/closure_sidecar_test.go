package playwrightruns

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var (
	closureX = "sha256:" + strings.Repeat("a", 64)
	closureY = "sha256:" + strings.Repeat("b", 64)
	blobsB   = map[string]string{"cart.spec.ts": "2222222222222222222222222222222222222222"}
)

// closed is a declaration with a DECLARED closure for cart.spec.ts.
func closed(app, config string, blobs map[string]string, digest string) map[string]any {
	d := identity(app, config, blobs)
	d["testClosureRule"] = ClosureRuleDeclared
	d["testClosureDigests"] = map[string]any{"cart.spec.ts": digest}
	return d
}

func sidecarFor(t *testing.T, run *Run, declared any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"profile": SidecarProfile, "reportSha256": run.Sha256, "corvintRunIdentity": declared})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// PWR-V0-064: a sidecar binds to exactly one report's bytes and is read strictly and bounded.
func TestPWRV0064_SidecarBindsOneReport(t *testing.T) {
	bare := func() *Run { return mustParse(t, reportJSON(t, "1.63.0", nil, cartTest(false, pass()))) }
	run := bare()
	if err := ApplySidecar(run, sidecarFor(t, run, identity("app-1", "cfg-1", blobsA))); err != nil {
		t.Fatal(err)
	}
	if run.IdentitySource != IdentitySidecar || !run.InputsDeclared || run.Inputs.ApplicationRevision != "app-1" || run.Inputs.TestFileBlobs["cart.spec.ts"] != blobsA["cart.spec.ts"] {
		t.Fatalf("sidecar run = %+v", run)
	}
	other := mustParse(t, reportJSON(t, "1.63.0", nil, cartTest(false, pass()), fxTest{file: "b.spec.ts", title: "b", status: "expected", results: []fxResult{pass()}}))
	if err := ApplySidecar(bare(), sidecarFor(t, other, identity("app-1", "cfg-1", blobsA))); !errors.Is(err, ErrSidecarMismatch) {
		t.Fatalf("other report's sidecar: %v", err)
	}
	good := string(sidecarFor(t, run, identity("app-1", "cfg-1", blobsA)))
	invalid := map[string]string{
		"profile":        strings.Replace(good, SidecarProfile, "corvint-run-identity-sidecar/9", 1),
		"unknown member": strings.Replace(good, `"profile"`, `"extra":1,"profile"`, 1),
		"trailing":       good + `{}`,
		"no identity":    `{"profile":"` + SidecarProfile + `","reportSha256":"` + run.Sha256 + `"}`,
		"array identity": `{"profile":"` + SidecarProfile + `","reportSha256":"` + run.Sha256 + `","corvintRunIdentity":[]}`,
		"short sha":      strings.Replace(good, run.Sha256, run.Sha256[:10], 1),
		"not json":       `nope`,
	}
	for name, body := range invalid {
		if err := ApplySidecar(bare(), []byte(body)); !errors.Is(err, ErrSidecarInvalid) {
			t.Errorf("%s: err = %v, want ErrSidecarInvalid", name, err)
		}
	}
	dir := t.TempDir()
	big := filepath.Join(dir, "big.identity.json")
	if err := os.WriteFile(big, make([]byte, MaxSidecarBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSidecarFile(big); !errors.Is(err, ErrSidecarTooLarge) {
		t.Fatalf("oversized sidecar: %v", err)
	}
	if err := ApplySidecar(bare(), make([]byte, MaxSidecarBytes+1)); !errors.Is(err, ErrSidecarTooLarge) {
		t.Fatalf("oversized bytes: %v", err)
	}
	for _, name := range []string{dir, filepath.Join(dir, "absent.json")} {
		if _, err := ReadSidecarFile(name); !errors.Is(err, ErrSidecarUnreadable) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	small := filepath.Join(dir, "ok.identity.json")
	if err := os.WriteFile(small, []byte(good), 0o600); err != nil {
		t.Fatal(err)
	}
	if raw, err := ReadSidecarFile(small); err != nil || string(raw) != good {
		t.Fatalf("regular sidecar: %q %v", raw, err)
	}
}

// PWR-V0-066: a sidecar never overrides an embedded declaration.
func TestPWRV0066_SidecarNeverOverridesEmbedded(t *testing.T) {
	embedded := func() *Run {
		return mustParse(t, reportJSON(t, "1.63.0", identity("app-1", "cfg-1", blobsA), cartTest(false, pass())))
	}
	run := embedded()
	if err := ApplySidecar(run, sidecarFor(t, run, identity("app-1", "cfg-1", blobsA))); err != nil || run.IdentitySource != IdentityEmbedded {
		t.Fatalf("equal sidecar: %v %q", err, run.IdentitySource)
	}
	for name, declared := range map[string]map[string]any{
		"app":     identity("app-2", "cfg-1", blobsA),
		"config":  identity("app-1", "cfg-2", blobsA),
		"blob":    identity("app-1", "cfg-1", blobsB),
		"closure": closed("app-1", "cfg-1", blobsA, closureX),
		"revision": func() map[string]any {
			d := identity("app-1", "cfg-1", blobsA)
			d["testRevision"] = strings.Repeat("c", 40)
			return d
		}(),
	} {
		run := embedded()
		if err := ApplySidecar(run, sidecarFor(t, run, declared)); !errors.Is(err, ErrSidecarConflict) {
			t.Errorf("%s: err = %v, want ErrSidecarConflict", name, err)
		}
		if run.Inputs.ApplicationRevision != "app-1" || run.Inputs.ConfigDigest != "cfg-1" {
			t.Errorf("%s: embedded inputs replaced: %+v", name, run.Inputs)
		}
	}
	// A malformed embedded member still counts as embedded: a sidecar may not fill it in.
	malformed := mustParse(t, reportJSON(t, "1.63.0", map[string]any{"applicationRevision": 7}, cartTest(false, pass())))
	if err := ApplySidecar(malformed, sidecarFor(t, malformed, identity("app-1", "cfg-1", blobsA))); !errors.Is(err, ErrSidecarConflict) {
		t.Fatalf("malformed embedded: %v", err)
	}
}

// PWR-V0-067: the identity source is stated only on request, survives shard merging and is
// validated when a baseline is read back.
func TestPWRV0067_IdentitySource(t *testing.T) {
	declared := mustParse(t, reportJSON(t, "1.63.0", identity("app-1", "cfg-1", blobsA), cartTest(false, pass())))
	bare := mustParse(t, reportJSON(t, "1.63.0", nil, cartTest(false, pass())))
	plain, err := json.Marshal(StabilityLast([]*Run{declared, bare}, 2, 0))
	if err != nil || strings.Contains(string(plain), "identitySource") {
		t.Fatalf("unrequested identitySource: %s %v", plain, err)
	}
	MarkIdentitySource(declared)
	MarkIdentitySource(bare)
	r := StabilityLast([]*Run{declared, bare}, 2, 0)
	eq(t, "sources", []string{r.Runs[0].IdentitySource, r.Runs[1].IdentitySource}, []string{IdentityEmbedded, IdentityNotDeclared})

	a, b := *declared, *declared
	a.Shard, b.Shard = &Shard{Current: 1, Total: 2}, &Shard{Current: 2, Total: 2}
	b.IdentitySource = IdentitySidecar
	if merged := mergeShards([]*Run{&a, &b}); merged.IdentitySource != IdentitySidecar {
		t.Fatalf("merged source = %q", merged.IdentitySource)
	}

	dir := t.TempDir()
	full := named(t, dir, "full.json", bt("a.spec.ts", "both", "unexpected"))
	iso := named(t, dir, "iso.json", bt("a.spec.ts", "both", "unexpected"))
	body := func(source string) string {
		return `{"profile":"corvint-run-baseline/0","revision":"` + baselineRev + `","known":[{"project":"chromium","file":"a.spec.ts","titlePath":["both"],` +
			`"full":{"path":` + jsonString(full.Path) + `,"sha256":"` + full.Run.Sha256 + `","identitySource":"` + source + `"},"isolated":{"path":` + jsonString(iso.Path) + `,"sha256":"` + iso.Run.Sha256 + `"}}],"flaky":[]}`
	}
	p := filepath.Join(dir, "b.json")
	for source, ok := range map[string]bool{IdentitySidecar: true, IdentityEmbedded: true, IdentityNotDeclared: true, "GUESSED": false} {
		if err := os.WriteFile(p, []byte(body(source)), 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, err := ReadBaseline(p)
		if (err == nil) != ok || (!ok && !errors.Is(err, ErrBaselineShape)) {
			t.Errorf("identitySource %q: err = %v", source, err)
		}
	}
}

// PWR-V0-068: a closure declaration is read on its own and dropped when malformed.
func TestPWRV0068_ClosureDeclaration(t *testing.T) {
	read := func(declared map[string]any) Inputs {
		run := mustParse(t, reportJSON(t, "1.63.0", declared, cartTest(false, pass())))
		if !run.InputsDeclared || run.Inputs.ApplicationRevision != "app-1" || run.Inputs.ConfigDigest != "cfg-1" {
			t.Fatalf("closure member hid the other inputs: %+v", run.Inputs)
		}
		return run.Inputs
	}
	in := read(closed("app-1", "cfg-1", blobsA, closureX))
	if in.TestClosureRule != ClosureRuleDeclared || in.TestClosureDigests["cart.spec.ts"] != closureX {
		t.Fatalf("declared closure = %+v", in)
	}
	with := func(rule any, digests any) map[string]any {
		d := identity("app-1", "cfg-1", blobsA)
		d["testClosureRule"], d["testClosureDigests"] = rule, digests
		return d
	}
	cases := map[string]struct {
		decl     map[string]any
		rule     string
		digested bool
	}{
		"computed rule admitted": {with("corvint-test-closure/0", map[string]any{"cart.spec.ts": closureX}), ClosureRuleComputed, true},
		"unknown rule":           {with("corvint-test-closure/1", map[string]any{"cart.spec.ts": closureX}), "", true},
		"rule not a string":      {with(1, map[string]any{"cart.spec.ts": closureX}), "", true},
		"bad digest":             {with(ClosureRuleDeclared, map[string]any{"cart.spec.ts": "sha256:abc"}), ClosureRuleDeclared, false},
		"upper hex":              {with(ClosureRuleDeclared, map[string]any{"cart.spec.ts": "sha256:" + strings.Repeat("A", 64)}), ClosureRuleDeclared, false},
		"digests not an object":  {with(ClosureRuleDeclared, "x"), ClosureRuleDeclared, false},
		"conflicting spellings":  {with(ClosureRuleDeclared, map[string]any{"cart.spec.ts": closureX, "./cart.spec.ts": closureY}), ClosureRuleDeclared, false},
	}
	for name, c := range cases {
		in := read(c.decl)
		if in.TestClosureRule != c.rule || (in.TestClosureDigests["cart.spec.ts"] != "") != c.digested {
			t.Errorf("%s: closure = %q %v", name, in.TestClosureRule, in.TestClosureDigests)
		}
	}
	// A closure member joins the shard-set key only when declared.
	plain := mustParse(t, reportJSON(t, "1.63.0", identity("app-1", "cfg-1", blobsA), cartTest(false, pass())))
	withClosure := mustParse(t, reportJSON(t, "1.63.0", closed("app-1", "cfg-1", blobsA, closureX), cartTest(false, pass())))
	plain.Shard, withClosure.Shard = &Shard{Current: 1, Total: 2}, &Shard{Current: 1, Total: 2}
	if shardGroupKey(plain) == shardGroupKey(withClosure) {
		t.Fatal("closure did not join the shard key")
	}
}

// PWR-V0-069: --pool-by closure pools across config and unrelated changes on the declared closure.
func TestPWRV0069_PoolByClosure(t *testing.T) {
	one := reportJSON(t, "1.63.0", closed("app-1", "cfg-1", blobsA, closureX), cartTest(false, pass()))
	two := reportJSON(t, "1.63.0", closed("app-1", "cfg-2", blobsA, closureX), cartTest(false, pass()))
	byInputs := StabilityPooled(runsOf(t, one, two), 2, 0, PoolByInputs)
	if len(byInputs.Tests) != 2 || byInputs.PoolBy != "" {
		t.Fatalf("inputs pooling = %+v", byInputs)
	}
	byClosure := StabilityPooled(runsOf(t, one, two), 2, 0, PoolByClosure)
	e := onlyEntries(t, byClosure, 1)[0]
	if e.Verdict != Deterministic || e.Inputs.TestClosureDigest != closureX || e.Inputs.TestClosureRule != ClosureRuleDeclared ||
		e.Inputs.ConfigDigest != "" || e.Inputs.TestFileBlob != "" || byClosure.PoolBy != PoolByClosure {
		t.Fatalf("closure pooling = %+v", byClosure)
	}
	eq(t, "closure sources", []string{byClosure.Runs[0].ClosureSource, byClosure.Runs[1].ClosureSource}, []string{ClosureRuleDeclared, ClosureRuleDeclared})
	raw, _ := json.Marshal(e.Inputs)
	if strings.Contains(string(raw), "configDigest") || strings.Contains(string(raw), "testFileBlob\"") {
		t.Fatalf("closure inputs carry the replaced members: %s", raw)
	}
	// Another closure digest splits; a report without a closure abstains and is counted.
	changed := reportJSON(t, "1.63.0", closed("app-1", "cfg-1", blobsA, closureY), cartTest(false, pass()))
	none := reportJSON(t, "1.63.0", identity("app-1", "cfg-1", blobsA), cartTest(false, pass()))
	r := StabilityPooled(runsOf(t, one, changed, none), 2, 0, PoolByClosure)
	if len(r.Tests) != 3 || r.Summary.ClosureNotDeclaredRuns != 1 || r.Runs[2].ClosureSource != IdentityNotDeclared {
		t.Fatalf("report = %+v", r)
	}
	var missing *StabilityEntry
	for i := range r.Tests {
		if r.Tests[i].Reason == ReasonInputsMissing {
			missing = &r.Tests[i]
		}
	}
	if missing == nil || !slices.Equal(missing.MissingInputs, []string{"testClosureDigests[cart.spec.ts]", "testClosureRule"}) || !slices.Equal(missing.Runs, []int{3}) {
		t.Fatalf("missing closure entry = %+v", missing)
	}
	// The default key is unchanged: StabilityLast equals the inputs mode byte for byte.
	a, _ := json.Marshal(StabilityLast(runsOf(t, one, two, none), 2, 0))
	b, _ := json.Marshal(StabilityPooled(runsOf(t, one, two, none), 2, 0, PoolByInputs))
	if string(a) != string(b) || strings.Contains(string(a), "poolBy") || strings.Contains(string(a), "closure") {
		t.Fatalf("default output changed: %s", a)
	}
}

// PWR-V0-070: one closure digest over two test-file blobs is a contradiction.
func TestPWRV0070_ClosureContradiction(t *testing.T) {
	one := reportJSON(t, "1.63.0", closed("app-1", "cfg-1", blobsA, closureX), cartTest(false, pass()))
	same := reportJSON(t, "1.63.0", closed("app-2", "cfg-9", blobsA, closureX), cartTest(false, pass()))
	moved := reportJSON(t, "1.63.0", closed("app-1", "cfg-1", blobsB, closureX), cartTest(false, pass()))
	rehashed := reportJSON(t, "1.63.0", closed("app-1", "cfg-1", blobsB, closureY), cartTest(false, pass()))
	if _, _, ok := ClosureContradiction(runsOf(t, one, same, rehashed)); ok {
		t.Fatal("consistent closures reported as contradicted")
	}
	run, earlier, ok := ClosureContradiction(runsOf(t, one, same, moved))
	if !ok || run != 3 || earlier != 1 {
		t.Fatalf("contradiction = %d %d %v", run, earlier, ok)
	}
}

// PWR-V0-074: --pool-by closure pools computed closures as COMPUTED and never pools a computed
// digest with a declared one.
func TestPWRV0074_PoolByComputedClosure(t *testing.T) {
	computed := func(config string) map[string]any {
		d := closed("app-1", config, blobsA, closureX)
		d["testClosureRule"] = ClosureRuleComputed
		return d
	}
	one := reportJSON(t, "1.63.0", computed("cfg-1"), cartTest(false, pass()))
	two := reportJSON(t, "1.63.0", computed("cfg-2"), cartTest(false, pass()))
	r := StabilityPooled(runsOf(t, one, two), 2, 0, PoolByClosure)
	e := onlyEntries(t, r, 1)[0]
	if e.Verdict != Deterministic || e.Inputs.TestClosureRule != ClosureRuleComputed || e.Inputs.TestClosureDigest != closureX || r.Summary.ClosureNotDeclaredRuns != 0 {
		t.Fatalf("computed pooling = %+v", r)
	}
	eq(t, "closure sources", []string{r.Runs[0].ClosureSource, r.Runs[1].ClosureSource}, []string{"COMPUTED", "COMPUTED"})
	declared := reportJSON(t, "1.63.0", closed("app-1", "cfg-1", blobsA, closureX), cartTest(false, pass()))
	mixed := StabilityPooled(runsOf(t, one, declared), 2, 0, PoolByClosure)
	if len(mixed.Tests) != 2 || mixed.Runs[0].ClosureSource != "COMPUTED" || mixed.Runs[1].ClosureSource != ClosureRuleDeclared {
		t.Fatalf("declared and computed pooled together: %+v", mixed)
	}
}
