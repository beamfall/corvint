package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const auditJUnitTemplate = `<?xml version="1.0"?>
<testsuites>
  <testsuite name="example.com/fixture/core"><testcase classname="example.com/fixture/core" name="TestCore">%CORE%</testcase></testsuite>
  <testsuite name="example.com/fixture/solo"><testcase classname="example.com/fixture/solo" name="TestSolo">%SOLO%</testcase></testsuite>
</testsuites>
`

func writeAuditJUnit(t *testing.T, name, core, solo string) {
	t.Helper()
	body := strings.NewReplacer("%CORE%", core, "%SOLO%", solo).Replace(auditJUnitTemplate)
	if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// auditFixture commits an edit to core/core.go and writes a manifest naming
// that change, with JUnit outcomes outside the repository.
func auditFixture(t *testing.T, changedCore, changedSolo string) (root, manifest string) {
	t.Helper()
	root, base, head, outcomes := auditFixtureChange(t)
	writeAuditJUnit(t, filepath.Join(outcomes, "base.xml"), "", "")
	writeAuditJUnit(t, filepath.Join(outcomes, "changed.xml"), changedCore, changedSolo)
	manifest = writeAuditManifest(t, outcomes, `{"id":"edit-core","base":"`+base+`","head":"`+head+`","format":"junit","baseline":"base.xml","changed":"changed.xml"}`)
	return root, manifest
}

// auditFixtureChange commits an edit to core/core.go, checks base out again,
// and returns an empty directory outside the repository for outcomes.
func auditFixtureChange(t *testing.T) (root, base, head, outcomes string) {
	t.Helper()
	root = affectedFixtureRepository(t)
	base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	corePath := filepath.Join(root, "core", "core.go")
	body, err := os.ReadFile(corePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corePath, append(body, []byte("\n// audited edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "edit core")
	head = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	affectedGit(t, root, "checkout", "-q", base)
	return root, base, head, t.TempDir()
}

func writeAuditManifest(t *testing.T, directory string, changes ...string) string {
	t.Helper()
	manifest := filepath.Join(directory, "changes.json")
	document := `{"profile":"corvint-affected-audit-input/0","changes":[` + strings.Join(changes, ",") + `]}`
	if err := os.WriteFile(manifest, []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func runAuditCLI(t *testing.T, root, manifest string) (map[string]any, int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "affected", "audit", "--changes", manifest}, strings.NewReader(""), &stdout, &stderr)
	var report map[string]any
	if code != 2 {
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("exit %d stdout is not JSON: %v\n%s\n%s", code, err, stdout.String(), stderr.String())
		}
	}
	return report, code, stderr.String()
}

// auditSelector returns one change's scored selector by name.
func auditSelector(t *testing.T, report map[string]any, change int, name string) map[string]any {
	t.Helper()
	for _, item := range report["changes"].([]any)[change].(map[string]any)["selectors"].([]any) {
		if selector := item.(map[string]any); selector["name"] == name {
			return selector
		}
	}
	t.Fatalf("no selector %s in %v", name, report)
	return nil
}

func auditAggregate(t *testing.T, report map[string]any, name string) map[string]any {
	t.Helper()
	for _, item := range report["aggregate"].([]any) {
		if aggregate := item.(map[string]any); aggregate["name"] == name {
			return aggregate
		}
	}
	t.Fatalf("no aggregate %s in %v", name, report)
	return nil
}

func TestAffectedAuditReplaysChangesAgainstOutcomes_AFPV0445(t *testing.T) {
	t.Parallel()
	t.Run("a selected failure is no miss", func(t *testing.T) {
		root, manifest := auditFixture(t, `<failure message="boom"/>`, "")
		before := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		status := affectedGit(t, root, "status", "--porcelain", "--ignored")
		refs := affectedGit(t, root, "for-each-ref")
		report, code, stderr := runAuditCLI(t, root, manifest)
		if code != 0 || report["ok"] != true || report["profile"] != "corvint-affected-audit/1" {
			t.Fatalf("exit %d: %s %v", code, stderr, report)
		}
		change := report["changes"].([]any)[0].(map[string]any)
		observation := change["observation"].(map[string]any)
		if change["kind"] != "replay" || observation["state"] != "OBSERVED" || len(observation["must_rerun"].([]any)) != 1 || observation["observed_tests"].(float64) != 2 {
			t.Fatalf("change=%v", change)
		}
		selector := auditSelector(t, report, 0, "affected")
		if selector["scope"] != "BOUNDED" || selector["fallback"] != false || selector["verdict"] != "SAFE" || len(selector["misses"].([]any)) != 0 ||
			selector["selected_tests"].(float64) != 1 || selector["true_positives"].(float64) != 1 || selector["recall"] != "1.000000" || selector["precision"] != "1.000000" {
			t.Fatalf("selector=%v", selector)
		}
		aggregate := auditAggregate(t, report, "affected")
		if aggregate["changes_with_miss"].(float64) != 0 || aggregate["observed_changes"].(float64) != 1 || aggregate["rule_of_three"] != "1.000000" ||
			aggregate["miss_rate_upper_bound"] != "0.950000" || aggregate["recall"] != "1.000000" {
			t.Fatalf("aggregate=%v", aggregate)
		}
		for _, name := range []string{"appmap-impact"} {
			if scored := auditSelector(t, report, 0, name); scored["verdict"] != "NOT_RUN" || scored["recall"] != "NOT_RUN" {
				t.Fatalf("%s=%v", name, scored)
			}
			if aggregate := auditAggregate(t, report, name); aggregate["changes"].(float64) != 0 || aggregate["miss_rate_upper_bound"] != "NOT_RUN" {
				t.Fatalf("%s aggregate=%v", name, aggregate)
			}
		}
		if names := report["selectors"].([]any); len(names) != 2 || names[0].(map[string]any)["name"] != "affected" || names[1].(map[string]any)["name"] != "appmap-impact" {
			t.Fatalf("selectors=%v", names)
		}
		for _, item := range report["selectors"].([]any) {
			selector := item.(map[string]any)
			want := "NOT_RUN"
			if selector["name"] == "affected" {
				want = "RUN"
			}
			if selector["state"] != want {
				t.Fatalf("selectors=%v", report["selectors"])
			}
		}
		if after := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD")); after != before {
			t.Fatalf("HEAD moved %s -> %s", before, after)
		}
		if after := affectedGit(t, root, "status", "--porcelain", "--ignored"); after != status {
			t.Fatalf("the audited worktree changed:\n%s\n%s", status, after)
		}
		if after := affectedGit(t, root, "for-each-ref"); after != refs {
			t.Fatalf("the audited refs changed:\n%s\n%s", refs, after)
		}
	})
	t.Run("an unselected failure is a miss and exits 1", func(t *testing.T) {
		root, manifest := auditFixture(t, "", `<failure message="boom"/>`)
		report, code, stderr := runAuditCLI(t, root, manifest)
		if code != 1 || report["ok"] != false {
			t.Fatalf("exit %d: %s %v", code, stderr, report)
		}
		selector := auditSelector(t, report, 0, "affected")
		misses := selector["misses"].([]any)
		if len(misses) != 1 || misses[0].(map[string]any)["classname"] != "example.com/fixture/solo" || selector["verdict"] != "MISS" || selector["recall"] != "0.000000" || selector["precision"] != "0.000000" {
			t.Fatalf("selector=%v", selector)
		}
		if auditAggregate(t, report, "affected")["changes_with_miss"].(float64) != 1 {
			t.Fatalf("aggregate=%v", report["aggregate"])
		}
	})
	t.Run("an invalid manifest exits 2", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		manifest := filepath.Join(t.TempDir(), "changes.json")
		if err := os.WriteFile(manifest, []byte(`{"profile":"corvint-affected-audit-input/0","changes":[{"id":"x","base":"main","head":"main","format":"junit","baseline":"a","changed":"b"}]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, code, stderr := runAuditCLI(t, root, manifest); code != 2 || !strings.Contains(stderr, "unsupported-affected-audit") {
			t.Fatalf("exit %d: %s", code, stderr)
		}
	})
}

// AFP-V0-447: missing observation is UNKNOWN, never safe, and exits 3.
func TestAffectedAuditMissingObservationIsUnknown_AFPV0447(t *testing.T) {
	t.Parallel()
	t.Run("no outcome files", func(t *testing.T) {
		root, base, head, outcomes := auditFixtureChange(t)
		manifest := writeAuditManifest(t, outcomes, `{"id":"unobserved","base":"`+base+`","head":"`+head+`"}`)
		report, code, stderr := runAuditCLI(t, root, manifest)
		if code != 3 || report["ok"] != false {
			t.Fatalf("exit %d: %s %v", code, stderr, report)
		}
		change := report["changes"].([]any)[0].(map[string]any)
		if change["observation"].(map[string]any)["state"] != "UNKNOWN" {
			t.Fatalf("change=%v", change)
		}
		selector := auditSelector(t, report, 0, "affected")
		if selector["verdict"] != "UNKNOWN" || selector["recall"] != "UNKNOWN" || selector["precision"] != "UNKNOWN" || selector["scope"] != "BOUNDED" {
			t.Fatalf("selector=%v", selector)
		}
		aggregate := auditAggregate(t, report, "affected")
		if aggregate["unknown_changes"].(float64) != 1 || aggregate["observed_changes"].(float64) != 0 || aggregate["recall"] != "UNKNOWN" ||
			aggregate["miss_rate_upper_bound"] != "UNKNOWN" || aggregate["rule_of_three"] != nil {
			t.Fatalf("aggregate=%v", aggregate)
		}
	})
	t.Run("a test the changed run dropped is unobserved", func(t *testing.T) {
		root, base, head, outcomes := auditFixtureChange(t)
		writeAuditJUnit(t, filepath.Join(outcomes, "base.xml"), "", "")
		changed := `<?xml version="1.0"?><testsuites><testsuite name="example.com/fixture/core"><testcase classname="example.com/fixture/core" name="TestCore"/></testsuite></testsuites>`
		if err := os.WriteFile(filepath.Join(outcomes, "changed.xml"), []byte(changed), 0o644); err != nil {
			t.Fatal(err)
		}
		manifest := writeAuditManifest(t, outcomes, `{"id":"dropped","base":"`+base+`","head":"`+head+`","format":"junit","baseline":"base.xml","changed":"changed.xml"}`)
		report, code, stderr := runAuditCLI(t, root, manifest)
		if code != 3 {
			t.Fatalf("exit %d: %s %v", code, stderr, report)
		}
		observation := report["changes"].([]any)[0].(map[string]any)["observation"].(map[string]any)
		if len(observation["unobserved"].([]any)) != 1 || len(observation["removed"].([]any)) != 0 {
			t.Fatalf("observation=%v", observation)
		}
		selector := auditSelector(t, report, 0, "affected")
		if selector["verdict"] != "UNKNOWN" || selector["recall"] != "UNKNOWN" || len(selector["unobserved_unselected"].([]any)) != 1 {
			t.Fatalf("selector=%v", selector)
		}
	})
	t.Run("a selector error is UNKNOWN", func(t *testing.T) {
		root, manifest := auditFixture(t, `<failure message="boom"/>`, "")
		body, err := os.ReadFile(manifest)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.Replace(body, []byte(`"changed":"changed.xml"`), []byte(`"changed":"changed.xml","appmap":{"map":"absent.json"}`), 1)
		if err := os.WriteFile(manifest, body, 0o644); err != nil {
			t.Fatal(err)
		}
		report, code, stderr := runAuditCLI(t, root, manifest)
		if code != 3 {
			t.Fatalf("exit %d: %s %v", code, stderr, report)
		}
		for _, name := range []string{"appmap-impact"} {
			selector := auditSelector(t, report, 0, name)
			if selector["scope"] != "ERROR" || selector["verdict"] != "UNKNOWN" || selector["error"] == nil || selector["recall"] != "UNKNOWN" {
				t.Fatalf("%s=%v", name, selector)
			}
			if aggregate := auditAggregate(t, report, name); aggregate["unknown_changes"].(float64) != 1 || aggregate["miss_rate_upper_bound"] != "UNKNOWN" {
				t.Fatalf("%s aggregate=%v", name, aggregate)
			}
		}
		if auditSelector(t, report, 0, "affected")["verdict"] != "SAFE" {
			t.Fatalf("report=%v", report)
		}
	})
}

// AFP-V0-447: revert and mutant changes are built in the private clone.
func TestAffectedAuditRevertAndMutantChanges_AFPV0447(t *testing.T) {
	t.Parallel()
	root, base, head, outcomes := auditFixtureChange(t)
	writeAuditJUnit(t, filepath.Join(outcomes, "base.xml"), "", "")
	writeAuditJUnit(t, filepath.Join(outcomes, "changed.xml"), `<failure message="boom"/>`, "")
	patch := affectedGit(t, root, "diff", base, head)
	if err := os.WriteFile(filepath.Join(outcomes, "mutant.diff"), []byte(patch), 0o644); err != nil {
		t.Fatal(err)
	}
	refs := affectedGit(t, root, "for-each-ref")
	objects := affectedGit(t, root, "count-objects", "-v")
	manifest := writeAuditManifest(t, outcomes,
		`{"id":"revert-edit","kind":"revert","base":"`+head+`","revert":"`+head+`","format":"junit","baseline":"base.xml","changed":"changed.xml"}`,
		`{"id":"mutant-edit","kind":"mutant","base":"`+base+`","diff":"mutant.diff","format":"junit","baseline":"base.xml","changed":"changed.xml"}`)
	report, code, stderr := runAuditCLI(t, root, manifest)
	if code != 0 {
		t.Fatalf("exit %d: %s %v", code, stderr, report)
	}
	again, _, _ := runAuditCLI(t, root, manifest)
	for index, kind := range []string{"revert", "mutant"} {
		change := report["changes"].([]any)[index].(map[string]any)
		if change["kind"] != kind || change["head"] == change["base"] || !validGitObjectID(change["head"].(string)) ||
			change["head"] != again["changes"].([]any)[index].(map[string]any)["head"] {
			t.Fatalf("%s change=%v", kind, change)
		}
		if selector := auditSelector(t, report, index, "affected"); selector["scope"] != "BOUNDED" || selector["verdict"] != "SAFE" || selector["true_positives"].(float64) != 1 {
			t.Fatalf("%s selector=%v", kind, selector)
		}
	}
	if auditAggregate(t, report, "affected")["observed_changes"].(float64) != 2 {
		t.Fatalf("aggregate=%v", report["aggregate"])
	}
	if after := affectedGit(t, root, "for-each-ref"); after != refs {
		t.Fatalf("refs changed:\n%s\n%s", refs, after)
	}
	if after := affectedGit(t, root, "count-objects", "-v"); after != objects {
		t.Fatalf("objects changed:\n%s\n%s", objects, after)
	}
	bad := writeAuditManifest(t, t.TempDir(), `{"id":"x","kind":"mutant","base":"`+base+`","head":"`+head+`","diff":"d"}`)
	if _, code, stderr := runAuditCLI(t, root, bad); code != 2 || !strings.Contains(stderr, "unsupported-affected-audit") {
		t.Fatalf("a mutant with a head: exit %d: %s", code, stderr)
	}
}

// AFP-V0-447: the appmap-impact selector is run at the change head and scored.
func TestAffectedAuditAppmapImpactSelector_AFPV0447(t *testing.T) {
	t.Parallel()
	root := appmapCLIRepo(t)
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatal(diag)
	}
	outcomes := t.TempDir()
	if err := os.WriteFile(filepath.Join(outcomes, "map.json"), []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "club.html"), []byte("<p></p>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", "-A")
	affectedGit(t, root, "-c", "user.name=F", "-c", "user.email=f@example.invalid", "commit", "-qm", "template")
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	playwright := func(status string) string {
		return `{"suites":[{"title":"e2e/specs/login.spec.ts","file":"e2e/specs/login.spec.ts","specs":[{"title":"logs in","tests":[{"projectName":"chromium","status":"` + status + `"}]}]}]}`
	}
	for name, status := range map[string]string{"base.json": "expected", "changed.json": "unexpected"} {
		if err := os.WriteFile(filepath.Join(outcomes, name), []byte(playwright(status)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := writeAuditManifest(t, outcomes, `{"id":"template","base":"`+base+`","head":"`+head+`","format":"playwright-json","baseline":"base.json","changed":"changed.json","appmap":{"map":"map.json"}}`)
	report, _, stderr := runAuditCLI(t, root, manifest)
	if report == nil {
		t.Fatal(stderr)
	}
	selector := auditSelector(t, report, 0, "appmap-impact")
	// An unmapped template makes the selector require the full suite, and a fallback selects the failing test.
	if selector["selector_state"] != "full-suite-required" || selector["scope"] != "FALLBACK" || selector["fallback"] != true || selector["verdict"] != "SAFE" ||
		selector["recall"] != "1.000000" {
		t.Fatalf("selector=%v", selector)
	}
	if aggregate := auditAggregate(t, report, "appmap-impact"); aggregate["changes"].(float64) != 1 || aggregate["fallback_changes"].(float64) != 1 {
		t.Fatalf("aggregate=%v", aggregate)
	}
}

func TestAffectedAuditSelectionMatching_AFPV0447(t *testing.T) {
	t.Parallel()
	pick := affectedAuditPick{files: map[string]bool{"a.spec.ts": true}, units: map[string]bool{"go:example.com/p": true}, keys: map[string]bool{"b.spec.ts > suite > case": true}}
	for _, tc := range []struct {
		id   affectedAuditID
		want bool
	}{
		{affectedAuditID{File: "a.spec.ts", Name: "x"}, true},
		{affectedAuditID{Classname: "example.com/p", Name: "TestX"}, true},
		{affectedAuditID{File: "b.spec.ts", Name: "[chromium] suite > case"}, true},
		{affectedAuditID{File: "b.spec.ts", Name: "suite > other"}, false},
		{affectedAuditID{File: "c.spec.ts", Name: "x"}, false},
	} {
		if got := pick.selects(tc.id); got != tc.want {
			t.Fatalf("selects(%v)=%v", tc.id, got)
		}
	}
	if !(affectedAuditPick{fallback: true}).selects(affectedAuditID{Name: "anything"}) {
		t.Fatal("a fallback selects every test")
	}
}

func TestAffectedAuditPlaywrightAndBounds_AFPV0445(t *testing.T) {
	t.Parallel()
	ids, err := parseAffectedAuditPlaywright([]byte(`{"suites":[{"title":"shop.spec.ts","file":"shop.spec.ts","specs":[],"suites":[{"title":"cart","file":"shop.spec.ts","specs":[{"title":"adds","file":"shop.spec.ts","tests":[{"projectName":"chromium","status":"unexpected"},{"projectName":"firefox","status":"flaky"}]}]}]}]}`))
	if err != nil || len(ids) != 2 || ids[0].Name != "[chromium] cart > adds" || ids[0].Status != "failed" || ids[1].Status != "flaky" || ids[0].File != "shop.spec.ts" {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
	if _, err := parseAffectedAuditPlaywright([]byte(`{"suites":[{"file":"a.ts","specs":[{"title":"t","tests":[{"status":"interrupted"}]}]}]}`)); err == nil {
		t.Fatal("an unknown Playwright status must be refused")
	}
	for _, tc := range []struct {
		k, n int
		want string
	}{{0, 10, "0.258866"}, {0, 1, "0.950000"}, {1, 10, "0.394163"}, {10, 10, "1.000000"}} {
		if got := affectedAuditDecimal(clopperPearsonUpper(tc.k, tc.n, 0.05)); got != tc.want {
			t.Fatalf("clopper-pearson(%d/%d)=%s want %s", tc.k, tc.n, got, tc.want)
		}
	}
}

// AFP-V0-447 (4), (5): an unobserved test, even a selected one, keeps recall
// UNKNOWN, and an UNKNOWN per-change ratio keeps the aggregate ratio UNKNOWN.
func TestAffectedAuditUnknownRatiosPropagate_AFPV0447(t *testing.T) {
	t.Parallel()
	a, b, c := affectedAuditID{File: "a"}, affectedAuditID{File: "b"}, affectedAuditID{File: "c"}
	pick := affectedAuditPick{files: map[string]bool{"a": true, "b": true}, size: 2, state: "BOUNDED"}
	partial := scoreAffectedAuditSelector("affected", pick, "", nil, 0,
		affectedAuditObservation{State: "OBSERVED", MustRerun: []affectedAuditID{a, c}, Unobserved: []affectedAuditID{b}},
		[]affectedAuditOutcome{{id: a}, {id: c}})
	if partial.Verdict != auditVerdictMiss || partial.Recall != auditUnknown || partial.Precision != auditUnknown {
		t.Fatalf("partial=%+v", partial)
	}
	clean := scoreAffectedAuditSelector("affected", pick, "", nil, 0,
		affectedAuditObservation{State: "OBSERVED", MustRerun: []affectedAuditID{a}},
		[]affectedAuditOutcome{{id: a}})
	if clean.Verdict != auditVerdictSafe || clean.Recall != "1.000000" || clean.Precision != "1.000000" {
		t.Fatalf("clean=%+v", clean)
	}
	change := func(scored affectedAuditSelection, observation affectedAuditObservation) affectedAuditChange {
		selectors := make([]affectedAuditSelection, len(affectedAuditSelectorNames))
		for index, name := range affectedAuditSelectorNames {
			selectors[index] = affectedAuditSelection{Name: name, Verdict: auditVerdictNotRun}
		}
		selectors[0] = scored
		return affectedAuditChange{Observation: observation, Selectors: selectors}
	}
	aggregate := aggregateAffectedAudit([]affectedAuditChange{
		change(partial, affectedAuditObservation{State: "OBSERVED", MustRerun: []affectedAuditID{a, c}}),
		change(clean, affectedAuditObservation{State: "OBSERVED", MustRerun: []affectedAuditID{a}}),
	})[0]
	if aggregate.UnknownChanges != 0 || aggregate.ChangesWithMiss != 1 || aggregate.Recall != auditUnknown || aggregate.Precision != auditUnknown {
		t.Fatalf("aggregate=%+v", aggregate)
	}
}

// AFP-V0-447, AFP-V0-490: the e2e-compile selector was removed with the E2E proof lab, so a
// change carrying e2e is an input error (exit 2), never silently ignored.
func TestAffectedAuditE2EFieldRefused_AFPV0447(t *testing.T) {
	root := appmapCLIRepo(t)
	commit := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	document := `{"id":"c","base":"` + commit + `","head":"` + commit + `","e2e":{"manifest":"appmap.json"}}`
	report, code, stderr := runAuditCLI(t, root, writeAuditManifest(t, t.TempDir(), document))
	var refusal struct{ Code, Error string }
	if code != 2 || json.Unmarshal([]byte(stderr), &refusal) != nil || refusal.Code != "unsupported-affected-audit" ||
		!strings.Contains(refusal.Error, "--changes:") || !strings.Contains(refusal.Error, `"e2e"`) {
		t.Fatalf("exit %d, stderr %q (%v)", code, stderr, report)
	}
}
