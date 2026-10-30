package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gateagreement"
	"github.com/Beamfall/corvint/internal/migrationratchet"
)

// readAgreementLog decodes every record of an agreement log.
func readAgreementLog(t *testing.T, path string) []gateagreement.Record {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	records := []gateagreement.Record{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var r gateagreement.Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		records = append(records, r)
	}
	return records
}

// TestGVAV0001AdvisoryCompareCLI drives --advisory, --compare and --log on tests lint and tests
// duplicates and the agreement summary: the advisory exit is 0 where the gate is 1, an error
// still exits 2 and appends nothing, a log inside the repository and --compare without --log
// refuse, and the summary reports per-rule rates (GVA-V0-001, GVA-V0-004, GVA-V0-005,
// GVA-V0-007).
func TestGVAV0001AdvisoryCompareCLI(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	base := testsCommit(t, root, map[string]string{"e2e/cart.spec.ts": testsCartSpec})
	head := testsCommit(t, root, map[string]string{
		"e2e/basket.spec.ts": strings.Replace(testsCartSpec, "adds an item", "basket gains", 1) +
			"test('waits', async ({ page }) => {\n  await page.waitForTimeout(100);\n});\n",
	})
	dir := t.TempDir()
	log := filepath.Join(dir, "agreement.jsonl")
	verdicts := filepath.Join(dir, "verdicts.json")
	if err := os.WriteFile(verdicts, []byte(`{"schema":"corvint-gate-verdicts/0","verdicts":[
 {"rule":"fixed-wait","path":"e2e/basket.spec.ts","line":7},
 {"rule":"brittle-selector","path":"e2e/basket.spec.ts","line":2},
 {"rule":"duplicate-pair","test":"e2e/cart.spec.ts :: adds an item","other":"e2e/basket.spec.ts :: basket gains"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	lint := []string{"--root", root, "tests", "lint", "--base", base, "--all-lines"}
	if code, _, _ := runCLI(t, lint...); code != 1 {
		t.Fatalf("lint gate: code=%d", code)
	}
	code, out, errb := runCLI(t, append(lint, "--advisory", "--compare", verdicts, "--log", log)...)
	if code != 0 || !strings.Contains(out, "fixed-wait") {
		t.Fatalf("advisory lint: code=%d out=%s err=%s", code, out, errb)
	}
	records := readAgreementLog(t, log)
	r := records[0]
	if len(records) != 1 || r.Command != "tests lint" || r.ExitCode != 1 || r.AdvisoryExitCode != 0 || r.Revision != head ||
		r.Comparison != gateagreement.StatusCompared || r.Rules["fixed-wait"].Agreements != 1 || r.Rules["no-assertion"].CorvintOnly != 1 ||
		r.Rules["brittle-selector"].CallerOnly != 1 || strings.Join(r.Arguments, " ") != strings.Join(lint[4:], " ") ||
		len(r.IgnoredVerdicts) != 1 || r.Rules["duplicate-pair"] != (gateagreement.RuleCounts{}) {
		t.Fatalf("lint record = %+v", r)
	}
	if build, _ := json.Marshal(r.Build); !strings.Contains(string(build), `"version"`) {
		t.Fatalf("build identity = %s", build)
	}
	dup := []string{"--root", root, "tests", "duplicates", "--max-line-tests", "2", "--base", base, "--gate", "V1-2841"}
	if code, _, errb := runCLI(t, append(dup, "--advisory", "--compare", verdicts, "--log", log)...); code != 0 {
		t.Fatalf("advisory duplicates: code=%d err=%s", code, errb)
	}
	if records = readAgreementLog(t, log); len(records) != 2 || records[1].Rules["duplicate-pair"].Agreements != 1 {
		t.Fatalf("duplicates record = %+v", records[1])
	}
	if code, _, errb := runCLI(t, "--root", root, "tests", "lint", "--base", "no-such-rev", "--advisory", "--log", log); code != 2 || !strings.Contains(errb, `"ok":false`) {
		t.Fatalf("advisory error: code=%d err=%s", code, errb)
	}
	if len(readAgreementLog(t, log)) != 2 {
		t.Fatal("a refused run appended a record")
	}
	for _, c := range []struct {
		args []string
		code string
	}{
		{append(lint, "--compare", verdicts), "tests-invalid-arguments"},
		{append(lint, "--log", filepath.Join(root, "inside.jsonl")), "agreement-log-inside-repository"},
		{append(lint, "--compare", log, "--log", log), "agreement-verdicts-invalid"},
		{append(lint, "--advisory", "--advisory"), "tests-invalid-arguments"},
	} {
		if code, _, errb := runCLI(t, c.args...); code != 2 || !strings.Contains(errb, c.code) {
			t.Fatalf("%v: code=%d err=%s", c.args, code, errb)
		}
	}
	code, out, errb = runCLI(t, "tests", "agreement", "--log", log, "--command", "tests lint")
	var summary gateagreement.Summary
	if code != 0 || json.Unmarshal([]byte(out), &summary) != nil || summary.Runs != 1 || summary.ComparedRuns != 1 {
		t.Fatalf("summary: code=%d out=%s err=%s", code, out, errb)
	}
	for _, rule := range summary.Rules {
		if rule.Rule == "fixed-wait" && (rule.AgreementRate == nil || *rule.AgreementRate != 1) {
			t.Fatalf("fixed-wait rate = %+v", rule)
		}
	}
	if code, _, _ := runCLI(t, "tests", "agreement"); code != 2 {
		t.Fatalf("agreement without --log: code=%d", code)
	}
}

// TestGVAV0003MigrationRatchetAdvisory proves the migration ratchet's advisory mode exits 0 on a
// failing receipt and compares its policy failures (GVA-V0-001, GVA-V0-003).
func TestGVAV0003MigrationRatchetAdvisory(t *testing.T) {
	profile := migrationRatchetProfile()
	profile.Candidate.Records = append(profile.Candidate.Records, migrationRatchetRecord("test-execution", "test:orphan", "reviewed"))
	profile.Candidate.ArtifactSHA256 = migrationratchet.SnapshotDigest(profile.Candidate)
	path := writeMigrationRatchetProfile(t, profile)
	dir := t.TempDir()
	log := filepath.Join(dir, "ratchet.jsonl")
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"migration-ratchet", "--profile", path, "--advisory", "--log", log}, strings.NewReader(""), &stdout, &stderr); exit != 0 ||
		!strings.Contains(stdout.String(), `"verdict":"fail"`) {
		t.Fatalf("advisory ratchet: exit=%d stderr=%s", exit, stderr.String())
	}
	first := readAgreementLog(t, log)[0]
	if first.ExitCode != 1 || first.Comparison != gateagreement.StatusNotProduced || len(first.CorvintOnly) == 0 {
		t.Fatalf("ratchet record = %+v", first)
	}
	verdicts := filepath.Join(dir, "v.json")
	encoded, _ := json.Marshal(map[string]any{"schema": gateagreement.VerdictSchema, "verdicts": first.CorvintOnly})
	if err := os.WriteFile(verdicts, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	if exit := run([]string{"migration-ratchet", "--profile", path, "--compare", verdicts, "--log", log}, strings.NewReader(""), &bytes.Buffer{}, &stderr); exit != 1 {
		t.Fatalf("compared ratchet exit=%d", exit)
	}
	second := readAgreementLog(t, log)[1]
	if second.Comparison != gateagreement.StatusCompared || len(second.Agreements) != len(first.CorvintOnly) || len(second.CallerOnly)+len(second.CorvintOnly) != 0 {
		t.Fatalf("compared ratchet record = %+v", second)
	}
	stderr.Reset()
	if exit := run([]string{"migration-ratchet", "--profile", path, "--compare", verdicts}, strings.NewReader(""), &bytes.Buffer{}, &stderr); exit != 2 {
		t.Fatalf("compare without log exit=%d", exit)
	}
}

// TestGVAV0088RatchetIdentityDeltaKeys proves removals, state regressions and broken reverse links
// become findings and that the record names the baseline revision and the caller label
// (GVA-V0-088, GVA-V0-089).
func TestGVAV0088RatchetIdentityDeltaKeys(t *testing.T) {
	receipt := migrationratchet.Receipt{
		Removals:           []migrationratchet.Delta{{Kind: "contract", BaselineID: "c:1"}, {Kind: "contract", BaselineID: "c:2", ExceptedBy: "x"}},
		StateChanges:       []migrationratchet.StateDelta{{Kind: "test", CandidateID: "t:1", Direction: "regression"}, {Kind: "test", CandidateID: "t:2", Direction: "progress"}},
		BrokenReverseLinks: []migrationratchet.Delta{{Kind: "test", BaselineID: "t:3"}},
	}
	observed := ratchetObserved(receipt)
	got := []string{}
	for _, f := range observed.Findings {
		got = append(got, f.Rule+"|"+f.Test)
	}
	want := []string{"removal:contract|c:1", "state-regression:test|t:1", "broken-reverse-link:test|t:3"}
	if strings.Join(got, ",") != strings.Join(want, ",") || len(observed.Abstentions) != 1 || observed.Abstentions[0].Reason != "excepted" {
		t.Fatalf("findings=%v abstentions=%+v", got, observed.Abstentions)
	}
	path := writeMigrationRatchetProfile(t, migrationRatchetProfile())
	log := filepath.Join(t.TempDir(), "r.jsonl")
	var stderr bytes.Buffer
	if exit := run([]string{"migration-ratchet", "--profile", path, "--label", "batch-7", "--log", log}, strings.NewReader(""), &bytes.Buffer{}, &stderr); exit != 0 {
		t.Fatalf("exit=%d %s", exit, stderr.String())
	}
	record := readAgreementLog(t, log)[0]
	if record.Label != "batch-7" || record.BaselineRevision == "" {
		t.Fatalf("record label=%q baseline=%q", record.Label, record.BaselineRevision)
	}
}

// TestGVAV0094ValidityAdvisoryCompareCLI drives --advisory, --compare and --log on tests
// validity: the advisory exit is 0 where the refusal of a test as proof is 1, stdout is unchanged,
// a finding is keyed at its test's path and line, a refused run appends nothing, and --compare
// without --log or a log inside the repository refuse (GVA-V0-094).
func TestGVAV0094ValidityAdvisoryCompareCLI(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	head := testsCommit(t, root, map[string]string{"e2e/alpha.spec.ts": testsSerialSpec, "spec/widget_spec.rb": testsMemoizedSpec})
	dir := t.TempDir()
	log := filepath.Join(dir, "validity.jsonl")
	verdicts := filepath.Join(dir, "verdicts.json")
	if err := os.WriteFile(verdicts, []byte(`{"schema":"corvint-gate-verdicts/0","verdicts":[
 {"rule":"MEMOIZED-BLOCK-EXPECTATION","path":"spec/widget_spec.rb","line":4},
 {"rule":"LEGACY-ONLY-RULE","path":"spec/widget_spec.rb"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	validity := []string{"--root", root, "tests", "validity", "--files", "e2e/alpha.spec.ts,spec/widget_spec.rb"}
	gateCode, gateOut, _ := runCLI(t, validity...)
	if gateCode != 1 {
		t.Fatalf("validity gate: code=%d", gateCode)
	}
	code, out, errb := runCLI(t, append(validity, "--advisory", "--compare", verdicts, "--log", log, "--label", "shadow")...)
	if code != 0 || out != gateOut {
		t.Fatalf("advisory validity: code=%d err=%s same-stdout=%v", code, errb, out == gateOut)
	}
	records := readAgreementLog(t, log)
	r := records[0]
	if len(records) != 1 || r.Command != "tests validity" || r.ExitCode != 1 || r.AdvisoryExitCode != 0 || r.Revision != head || r.Label != "shadow" ||
		r.Comparison != gateagreement.StatusCompared || r.Rules["MEMOIZED-BLOCK-EXPECTATION"].Agreements != 1 ||
		r.Rules["ISOLATED-IN-SERIAL"].CorvintOnly != 1 || r.Rules["LEGACY-ONLY-RULE"].CallerOnly != 1 ||
		strings.Join(r.Arguments, " ") != strings.Join(validity[4:], " ") {
		t.Fatalf("validity record = %+v", r)
	}
	if code, _, errb := runCLI(t, "--root", root, "tests", "validity", "--files", "spec/absent_spec.rb", "--advisory", "--log", log); code != 2 || !strings.Contains(errb, `"ok":false`) {
		t.Fatalf("advisory error: code=%d err=%s", code, errb)
	}
	if len(readAgreementLog(t, log)) != 1 {
		t.Fatal("a refused run appended a record")
	}
	for _, c := range []struct {
		args []string
		code string
	}{
		{append(validity, "--compare", verdicts), "tests-invalid-arguments"},
		{append(validity, "--log", filepath.Join(root, "inside.jsonl")), "agreement-log-inside-repository"},
	} {
		if code, _, errb := runCLI(t, c.args...); code != 2 || !strings.Contains(errb, c.code) {
			t.Fatalf("%v: code=%d err=%s", c.args, code, errb)
		}
	}
}

// TestGVAV0095ValidityCompareKeepsParityFindings proves `tests validity --legacy ... --compare`
// keys each seed-parity and assertion-parity finding like a file finding, so a caller verdict on
// the same rule at the new test agrees instead of reading as callerOnly, and a link that was not
// analysed abstains on its family's rules (GVA-V0-095).
func TestGVAV0095ValidityCompareKeepsParityFindings(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	testsCommit(t, root, map[string]string{
		"spec/legacy_spec.rb": "RSpec.describe A do\n  it 'a' do\n    create_list(:item, 2)\n    expect(Item.count).to eq(2)\n  end\nend\n",
		"spec/new_spec.rb":    "RSpec.describe A do\n  it 'a' do\n    create(:item)\n    expect(Item.count).to eq(1)\n  end\nend\n",
		"spec/skip_spec.rb":   "RSpec.describe A do\n  xit 'a' do\n    create_list(:item, 2)\n  end\nend\n",
	})
	dir := t.TempDir()
	log, verdicts := filepath.Join(dir, "validity.jsonl"), filepath.Join(dir, "verdicts.json")
	if err := os.WriteFile(verdicts, []byte(`{"schema":"corvint-gate-verdicts/0","verdicts":[
 {"rule":"SEED-PARITY-DIVERGES","path":"spec/new_spec.rb","line":2},
 {"rule":"ASSERTION-PARITY-UNMATCHED-LEGACY","path":"spec/new_spec.rb","line":2},
 {"rule":"SEED-PARITY-DIVERGES","path":"spec/skip_spec.rb","line":2}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errb := runCLI(t, "--root", root, "tests", "validity", "--legacy", "spec/new_spec.rb:2=spec/legacy_spec.rb:2",
		"--legacy", "spec/skip_spec.rb:2=spec/legacy_spec.rb:2", "--advisory", "--compare", verdicts, "--log", log)
	if code != 0 {
		t.Fatalf("advisory validity: code=%d err=%s", code, errb)
	}
	records := readAgreementLog(t, log)
	if len(records) != 1 {
		t.Fatalf("records = %+v", records)
	}
	for _, rule := range []string{"SEED-PARITY-DIVERGES", "ASSERTION-PARITY-UNMATCHED-LEGACY"} {
		if got := records[0].Rules[rule]; got.Agreements != 1 || got.CallerOnly != 0 {
			t.Errorf("%s = %+v, want one agreement; record %+v", rule, got, records[0])
		}
	}
	if got := records[0].Rules["SEED-PARITY-DIVERGES"]; got.Abstentions != 1 {
		t.Errorf("not-analysed link: %+v, want one abstention; record %+v", got, records[0])
	}
}
