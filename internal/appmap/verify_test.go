package appmap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/parser"
	gotoken "go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	js "github.com/Beamfall/corvint/internal/jstestprovider"
)

// receiptTestID recomputes the PWP qualified test identity for a synthetic receipt.
func receiptTestID(r js.Receipt, t js.TestOutcome) string {
	b, _ := json.Marshal(struct {
		Identity js.Identity
		Project  *js.ProjectIdentity
		Anchor   *js.Anchor
		FullName string
	}{r.Identity, t.Project, t.Anchor, t.FullName})
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// pwpReceipt builds a qualified corvint-playwright-external/3 receipt whose application identity
// is appRev, with one outcome per state named t0, t1, ...
func pwpReceipt(t *testing.T, appRev string, states ...js.ExecutionState) js.Receipt {
	t.Helper()
	config := sum([]byte("module.exports={};\n"))
	build := sum([]byte("app.js"))
	r := js.Receipt{Profile: js.AttemptExternalProfile, Kind: "e2e",
		Identity: js.Identity{ConfigFile: "/fixture/config.cjs", ConfigDigest: config, ConfigInputDigests: map[string]string{"/fixture/config.cjs": config},
			TestFileDigests: map[string]string{"/fixture/test.cjs": sum([]byte("test"))}, PackageDigest: sum([]byte("package")), NodeVersion: "v22.23.2",
			RunnerName: "playwright", RunnerVersion: "1.63.0", Environment: map[string]string{}, Argv: []string{"node", "playwright"}},
		External: &js.ExternalLifecycle{ReadyURL: "http://127.0.0.1:8000", DeclaredAppIdentity: appRev, Ownership: "external", CleanupResponsibility: "external",
			ServerDescendants: "unknown", ReadyAtStart: true, ReadyAtPublish: true, RunnerDescendantsGone: true, InputsUnchanged: true, ConfigOverride: "synthetic fixture"},
		AppBuildAtStart: js.AppBuildIdentity{Digest: build}, AppBuildAtPublish: js.AppBuildIdentity{Digest: build}, Schedule: &js.ExecutionSchedule{Workers: 1}}
	use := json.RawMessage(`{"browserName":"chromium","channel":"","headless":true,"launchOptions":{},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v22.23.2","browserType":"chromium","browserVersion":"Google Chrome for Testing 153.0.8010.12","channel":"","executableSource":"playwright-bundled","executableName":"chromium-headless-shell","executablePath":"/portable/cache/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell","executableSha256":"a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282","browserRevision":"1243","manifestBrowserVersion":"153.0.8010.12","headlessShellAvailable":true}}`)
	for i, state := range states {
		kind := "none"
		if state == js.StateFailed {
			kind = "assertion-or-test"
		}
		name := "t" + strconv.Itoa(i)
		a := &js.Anchor{File: "/fixture/test.cjs", Line: i + 1}
		o := js.TestOutcome{Name: name, FullName: name, State: state, Anchor: a, Project: &js.ProjectIdentity{Browser: "chromium", Device: "unknown", Use: use, ConfigDigest: config},
			Attempts: []js.Attempt{{State: state, FailureKind: kind}}, AttemptDetails: []js.AttemptDetail{{State: state, Anchor: a}}}
		o.ID = receiptTestID(r, o)
		r.Tests = append(r.Tests, o)
		r.Schedule.Starts = append(r.Schedule.Starts, js.ExecutionStart{FullName: name, File: a.File, Line: a.Line})
	}
	return r
}

func writeReceipt(t *testing.T, r js.Receipt) string {
	t.Helper()
	for i := range r.Tests {
		r.Tests[i].ID = receiptTestID(r, r.Tests[i])
	}
	raw, err := js.EncodeQualified(r)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(name, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

const slotStep = "step:book-tee-time/select-slot"

// linkStep adds a declared test link from one book-tee-time step to key and commits it.
func linkStep(t *testing.T, root, step, basis, key string) string {
	t.Helper()
	p := "flows/book-tee-time.json"
	data := readText(t, root, p)
	link := `{"from":"` + step + `","basis":"` + basis + `","target":{"type":"test","test_key":"` + key + `"}}`
	writeFile(t, root, p, strings.Replace(data, `"links": []`, `"links": [`+link+`]`, 1))
	return commitAll(t, root, "link "+step)
}

func verification(t *testing.T, m *Map, receipts []string, binds ...string) *Verification {
	t.Helper()
	v, err := LoadVerification(m, receipts, binds)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// stepVerification returns the printed verification of one step in a flow projection.
func stepVerification(t *testing.T, m *Map, root, step string, v *Verification) map[string]any {
	t.Helper()
	raw, err := ProjectFlow(context.Background(), m, "book-tee-time", Options{Root: root, Verification: v})
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > DefaultFlowBudget {
		t.Fatalf("flow projection %d bytes exceeds its cap", len(raw))
	}
	for _, it := range decode(t, raw)["steps"].([]any) {
		s := it.(map[string]any)
		if s["id"] == step {
			got, _ := s["verification"].(map[string]any)
			return got
		}
	}
	t.Fatalf("step %s not printed", step)
	return nil
}

// RVN-V0-002, RVN-V0-005, RVN-V0-006: a passing qualified outcome bound by a declared intent link
// verifies the step at the receipt's application revision and cites the receipt.
func TestRVNV0002PassingReceiptVerifiesStep(t *testing.T) {
	root, rev := fixtureRepo(t)
	r := pwpReceipt(t, rev, js.StatePassed)
	name := writeReceipt(t, r)
	head := linkStep(t, root, "select-slot", "declared", r.Tests[0].ID)
	m := build(t, root, head)
	if _, st := m.step(slotStep); st == nil || len(st.Tests) != 1 || st.Tests[0] != r.Tests[0].ID {
		t.Fatalf("declared link not compiled onto the step: %+v", st)
	}
	got := stepVerification(t, m, root, slotStep, verification(t, m, []string{name}))
	raw, _ := os.ReadFile(name)
	if got["status"] != Verified || got["revision"] != rev || got["authority"] != AuthorityLearned || got["app_identity"] != "declared" || got["selector_evidence"] != "run-verified" {
		t.Fatalf("verification %v", got)
	}
	ref := got["receipt"].(map[string]any)
	if ref["sha256"] != sum(raw) || ref["test_key"] != r.Tests[0].ID || ref["profile"] != js.AttemptExternalProfile {
		t.Fatalf("receipt reference %v", ref)
	}
	if other := stepVerification(t, m, root, "step:book-tee-time/book", verification(t, m, []string{name})); other["status"] != Unverified || other["reason"] != "no-binding" {
		t.Fatalf("unbound step %v", other)
	}
	// The screen projection carries the same status on the same step.
	doc := screenDoc(t, m, "app.clubs.teesheets", Options{Root: root, Verification: verification(t, m, []string{name})})
	found := false
	for _, it := range doc["steps"].([]any) {
		s := it.(map[string]any)
		if s["id"] == slotStep {
			found = s["verification"].(map[string]any)["status"] == Verified
		}
	}
	if !found {
		t.Fatal("screen projection lost the step verification")
	}
	if vs := VerifySteps(context.Background(), m, "book-tee-time", Options{Root: root, Verification: verification(t, m, []string{name})}); vs[slotStep].Status != Verified {
		t.Fatalf("VerifySteps %v", vs)
	}
}

// RVN-V0-002: an agent-declared binding verifies like a compiled one; one whose test key is absent
// from every receipt, one naming no step, and a malformed or receipt-less one refuse.
func TestRVNV0002DeclaredBindings(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	r := pwpReceipt(t, rev, js.StatePassed)
	name := writeReceipt(t, r)
	got := stepVerification(t, m, root, slotStep, verification(t, m, []string{name}, slotStep+"="+r.Tests[0].ID))
	if got["status"] != Verified {
		t.Fatalf("declared binding %v", got)
	}
	absent := strings.Repeat("a", 64)
	for want, binds := range map[string][]string{
		"appmap-verify-test-absent":  {slotStep + "=" + absent},
		"appmap-verify-unknown-step": {"step:book-tee-time/none=" + r.Tests[0].ID},
		"appmap-invalid-query":       {"select-slot=" + r.Tests[0].ID},
	} {
		if _, err := LoadVerification(m, []string{name}, binds); codeOf(err) != want {
			t.Errorf("%v: %v, want %s", binds, err, want)
		}
	}
	if _, err := LoadVerification(m, nil, []string{slotStep + "=" + r.Tests[0].ID}); codeOf(err) != "appmap-invalid-query" {
		t.Errorf("bind without receipt: %v", err)
	}
	notPWP := filepath.Join(t.TempDir(), "bad.json")
	writeFile(t, filepath.Dir(notPWP), "bad.json", `{"receipt":{"kind":"e2e","tests":[]}}`)
	link := filepath.Join(t.TempDir(), "link.json")
	if err := os.Symlink(name, link); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{notPWP, link, filepath.Join(root, "missing.json")} {
		if _, err := LoadVerification(m, []string{bad}, nil); codeOf(err) != "appmap-verify-invalid-receipt" {
			t.Errorf("%s: %v", bad, err)
		}
	}
	many := make([]string, MaxReceipts+1)
	for i := range many {
		many[i] = name
	}
	if _, err := LoadVerification(m, many, nil); codeOf(err) != "appmap-invalid-query" {
		t.Errorf("receipt bound: %v", err)
	}
}

// RVN-V0-002: an inferred link never binds a step.
func TestRVNV0002InferredLinkDoesNotBind(t *testing.T) {
	root, rev := fixtureRepo(t)
	r := pwpReceipt(t, rev, js.StatePassed)
	name := writeReceipt(t, r)
	m := build(t, root, linkStep(t, root, "select-slot", "inferred", r.Tests[0].ID))
	if got := stepVerification(t, m, root, slotStep, verification(t, m, []string{name})); got["status"] != Unverified || got["reason"] != "no-binding" {
		t.Fatalf("inferred link %v", got)
	}
}

// RVN-V0-004, RVN-V0-005: a change to the step's router state after the receipt's revision makes a
// verified step UNVERIFIED_AT_HEAD; evaluated at the receipt's revision it is VERIFIED again.
func TestRVNV0004SourceChangeUnverifiesAtHead(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	r := pwpReceipt(t, rev, js.StatePassed)
	v := verification(t, m, []string{writeReceipt(t, r)}, slotStep+"="+r.Tests[0].ID)
	routes := readText(t, root, "app/routes.js")
	writeFile(t, root, "app/routes.js", strings.Replace(routes, "flags: ['new_teesheet']", "flags: ['new_teesheet', 'beta']", 1))
	commitAll(t, root, "change teesheet state")
	got := stepVerification(t, m, root, slotStep, v)
	if got["status"] != UnverifiedAtHead || got["revision"] != rev || got["receipt"] == nil {
		t.Fatalf("changed source %v", got)
	}
	raw, err := ProjectFlow(context.Background(), m, "book-tee-time", Options{Root: root, Revision: rev, Verification: v})
	if err != nil || !bytes.Contains(raw, []byte(`"status":"VERIFIED"`)) {
		t.Fatalf("at the receipt revision: %v %s", err, raw)
	}
	// A map rebuilt at HEAD pins the new source, which the receipt never ran against.
	m2 := build(t, root, "HEAD")
	if got := stepVerification(t, m2, root, slotStep, v); got["status"] != Unverified || got["reason"] != "anchor-differs-at-app-revision" {
		t.Fatalf("rebuilt map %v", got)
	}
}

// RVN-V0-003, RVN-V0-005: a failing outcome contradicts the step, even beside a passing one.
func TestRVNV0003FailingReceiptContradicts(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	r := pwpReceipt(t, rev, js.StatePassed, js.StateFailed)
	name := writeReceipt(t, r)
	got := stepVerification(t, m, root, slotStep, verification(t, m, []string{name}, slotStep+"="+r.Tests[1].ID))
	if got["status"] != Contradicted || got["revision"] != rev || got["selector_evidence"] != "run-contradicted" {
		t.Fatalf("failing outcome %v", got)
	}
	got = stepVerification(t, m, root, slotStep, verification(t, m, []string{name}, slotStep+"="+r.Tests[0].ID, slotStep+"="+r.Tests[1].ID))
	if got["status"] != Contradicted || got["outcomes"] != float64(2) {
		t.Fatalf("pass and fail %v", got)
	}
	flaky := pwpReceipt(t, rev, js.StateSkipped)
	if got := stepVerification(t, m, root, slotStep, verification(t, m, []string{writeReceipt(t, flaky)}, slotStep+"="+flaky.Tests[0].ID)); got["status"] != Unverified || got["reason"] != "inconclusive-outcome" {
		t.Fatalf("skipped outcome %v", got)
	}
	unqualified := pwpReceipt(t, rev, js.StatePassed)
	unqualified.External.RunnerDescendantsGone = false
	if got := stepVerification(t, m, root, slotStep, verification(t, m, []string{writeReceipt(t, unqualified)}, slotStep+"="+unqualified.Tests[0].ID)); got["status"] != Unverified || got["reason"] != "inconclusive-outcome" {
		t.Fatalf("unqualified outcome %v", got)
	}
}

// RVN-V0-003: an unknown, unresolvable or non-commit application revision never reads VERIFIED or
// CONTRADICTED, and a failure that cannot be placed keeps a passing outcome from verifying.
func TestRVNV0003UnresolvedRevisionNeverVerifies(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	tree := gitTest(t, root, "rev-parse", rev+"^{tree}")
	for _, identity := range []string{"fixture-app", strings.Repeat("0", 40), rev[:12], tree, strings.ToUpper(rev)} {
		r := pwpReceipt(t, identity, js.StatePassed, js.StateFailed)
		v := verification(t, m, []string{writeReceipt(t, r)}, slotStep+"="+r.Tests[0].ID)
		if got := stepVerification(t, m, root, slotStep, v); got["status"] != Unverified || got["reason"] != "app-revision-unresolved" {
			t.Errorf("%s pass: %v", identity, got)
		}
		v = verification(t, m, []string{writeReceipt(t, r)}, slotStep+"="+r.Tests[1].ID)
		if got := stepVerification(t, m, root, slotStep, v); got["status"] != Unverified {
			t.Errorf("%s fail: %v", identity, got)
		}
	}
	pass := pwpReceipt(t, rev, js.StatePassed)
	fail := pwpReceipt(t, "fixture-app", js.StateFailed)
	fail.Tests[0].Name, fail.Tests[0].FullName = "other", "other"
	v := verification(t, m, []string{writeReceipt(t, pass), writeReceipt(t, fail)}, slotStep+"="+pass.Tests[0].ID, slotStep+"="+receiptTestID(fail, fail.Tests[0]))
	if got := stepVerification(t, m, root, slotStep, v); got["status"] != Unverified || got["reason"] != "unplaced-failure" {
		t.Fatalf("unplaced failure %v", got)
	}
	r := pwpReceipt(t, rev, js.StatePassed)
	raw, err := ProjectFlow(context.Background(), m, "book-tee-time", Options{Root: root, Revision: "no-such-rev", Verification: verification(t, m, []string{writeReceipt(t, r)}, slotStep+"="+r.Tests[0].ID)})
	if err != nil || bytes.Contains(raw, []byte(`"VERIFIED"`)) || !bytes.Contains(raw, []byte(`"freshness-unknown"`)) {
		t.Fatalf("unknown evaluated revision: %v %s", err, raw)
	}
}

// RVN-V0-006, RVN-V0-008: verification fits every budget the projections already enforce, adds no
// field when no receipt is supplied, and reads never change the worktree.
func TestRVNV0006BudgetsAndReadOnly(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	r := pwpReceipt(t, rev, js.StatePassed)
	v := verification(t, m, []string{writeReceipt(t, r)}, slotStep+"="+r.Tests[0].ID, "step:book-tee-time/book="+r.Tests[0].ID)
	before := gitTest(t, root, "status", "--porcelain", "--ignored")
	ctx := context.Background()
	for budget := MinBudget; budget <= DefaultFlowBudget; budget += 197 {
		for _, c := range []struct {
			project func(context.Context, *Map, string, Options) ([]byte, error)
			query   string
		}{{ProjectFlow, "book-tee-time"}, {ProjectScreen, "app.clubs.teesheets"}} {
			raw, err := c.project(ctx, m, c.query, Options{Root: root, Budget: budget, Verification: v})
			if err != nil && codeOf(err) != "appmap-budget-too-small" {
				t.Fatalf("%s budget %d: %v", c.query, budget, err)
			}
			if len(raw) > budget {
				t.Fatalf("%s budget %d: %d bytes", c.query, budget, len(raw))
			}
		}
	}
	for _, c := range []struct {
		project func(context.Context, *Map, string, Options) ([]byte, error)
		query   string
	}{{ProjectFlow, "book-tee-time"}, {ProjectScreen, "app.clubs.teesheets"}} {
		if raw, err := c.project(ctx, m, c.query, Options{Root: root, Verification: v}); err != nil || !bytes.Contains(raw, []byte(`"VERIFIED"`)) {
			t.Fatalf("%s at its default budget: %v %s", c.query, err, raw)
		}
	}
	plain, err := ProjectFlow(ctx, m, "book-tee-time", Options{Root: root})
	if err != nil || bytes.Contains(plain, []byte(`"verification"`)) {
		t.Fatalf("verification printed without receipts: %v", err)
	}
	if after := gitTest(t, root, "status", "--porcelain", "--ignored"); after != before {
		t.Fatalf("worktree changed:\n%s", after)
	}
}

// RVN-V0-007: verification never changes any other field of a projection, and the ranking and
// authority packages do not import the map.
func TestRVNV0007AuthorityLimit(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	r := pwpReceipt(t, rev, js.StatePassed)
	v := verification(t, m, []string{writeReceipt(t, r)}, slotStep+"="+r.Tests[0].ID)
	ctx := context.Background()
	for _, c := range []struct {
		project func(context.Context, *Map, string, Options) ([]byte, error)
		query   string
	}{{ProjectFlow, "book-tee-time"}, {ProjectScreen, "app.clubs.teesheets"}} {
		plain, err := c.project(ctx, m, c.query, Options{Root: root, Full: true})
		if err != nil {
			t.Fatal(err)
		}
		verified, err := c.project(ctx, m, c.query, Options{Root: root, Full: true, Verification: v})
		if err != nil || !bytes.Contains(verified, []byte(`"VERIFIED"`)) {
			t.Fatalf("%s: %v", c.query, err)
		}
		doc := decode(t, verified)
		for _, it := range doc["steps"].([]any) {
			delete(it.(map[string]any), "verification")
		}
		if fmtJSON(doc) != fmtJSON(decode(t, plain)) {
			t.Fatalf("%s: verification changed other fields", c.query)
		}
	}
	for _, dir := range []string{"../contextindex", "../slotlearn", "../localauthority", "../authoritystore", "../authorityevent"} {
		pkgs, err := parser.ParseDir(gotoken.NewFileSet(), dir, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, pkg := range pkgs {
			for name, f := range pkg.Files {
				for _, imp := range f.Imports {
					if strings.Contains(imp.Path.Value, "/internal/appmap") {
						t.Errorf("%s imports the application map", name)
					}
				}
			}
		}
	}
}

// RVN-V0-006: a step's status depends only on its own app-source anchors, so a projection whose
// unrelated anchor cannot be read (here a missing changed blob of the flow intent, as in a partial
// clone) prints the same status as VerifySteps.
func TestRVNV0006StatusIgnoresUnrelatedAnchors(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	r := pwpReceipt(t, rev, js.StatePassed)
	v := verification(t, m, []string{writeReceipt(t, r)}, slotStep+"="+r.Tests[0].ID)
	writeFile(t, root, "flows/book-tee-time.json", readText(t, root, "flows/book-tee-time.json")+"\n")
	commitAll(t, root, "touch intent")
	blob := gitTest(t, root, "rev-parse", "HEAD:flows/book-tee-time.json")
	if err := os.Remove(filepath.Join(root, ".git", "objects", blob[:2], blob[2:])); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	raw, err := ProjectFlow(ctx, m, "book-tee-time", Options{Root: root, Verification: v})
	if err != nil || !bytes.Contains(raw, []byte(`"freshness":"UNKNOWN"`)) {
		t.Fatalf("unrelated anchor did not read UNKNOWN: %v %s", err, raw)
	}
	got := stepVerification(t, m, root, slotStep, v)
	direct := VerifySteps(ctx, m, "book-tee-time", Options{Root: root, Verification: v})[slotStep]
	if got["status"] != Verified || direct.Status != Verified {
		t.Fatalf("projection %v, VerifySteps %+v", got, direct)
	}
}

// RVN-V0-001: a receipt rewritten in place between two observations is detected by size or
// modification time even though its identity (inode) is unchanged.
func TestRVNV0001ReceiptRewriteDetected(t *testing.T) {
	name := filepath.Join(t.TempDir(), "receipt.json")
	writeFile(t, filepath.Dir(name), "receipt.json", "{}")
	before, err := os.Lstat(name)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := os.Lstat(name); !unchanged(before, again) {
		t.Fatal("an untouched file reads changed")
	}
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"x":1}`)
	f.Close()
	after, err := os.Lstat(name)
	if err != nil || !os.SameFile(before, after) || unchanged(before, after) {
		t.Fatalf("in-place rewrite not detected: %v", err)
	}
}
