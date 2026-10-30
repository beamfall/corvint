package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

func TestAggregateStrictQueryAbstentionNativeReplay(t *testing.T) {
	binary := os.Getenv("CORVINT_AGGREGATE_QUERY_TEST_BINARY")
	if binary == "" {
		t.Skip("requires retained native candidate binary")
	}
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".context-corvint/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Authority\nUse the repository work queue.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "query authority")
	base := testGit(t, root, "rev-parse", "HEAD")
	trace := filepath.Join(root, ".context-corvint/traces/preserved.jsonl")
	if err := os.MkdirAll(filepath.Dir(trace), 0700); err != nil {
		t.Fatal(err)
	}
	before := []byte("preserve existing trace bytes exactly\n")
	if err := os.WriteFile(trace, before, 0600); err != nil {
		t.Fatal(err)
	}
	task := "Identify the active work queue, required workflow gates, and minimum context needed to safely take the next roadmap ticket"
	args := []string{"query", "--task", task, "--limit", "1"}
	runner := Runner{Path: binary}
	observed, err := runAggregateVerifier(context.Background(), runner, root, args)
	if err != nil || observed.status != 2 || len(observed.stdout) != 0 || string(observed.stderr) != queryAbstentionEnvelope {
		t.Fatalf("actual query: %+v %v", observed, err)
	}
	argv := []byte(strings.Join(append([]string{binary, "--root", root}, args...), "\x00") + "\x00")
	artifact := queryAbstentionArtifact(argv, task, base, base, observed.stdout, observed.stderr)
	evidence := filepath.Join(root, ".git/corvint")
	if err = os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	for suffix, raw := range map[string][]byte{".argv": argv, ".json": observed.stdout, ".stderr": observed.stderr, "-abstention.json": artifact} {
		if err = os.WriteFile(filepath.Join(evidence, queryStep+suffix), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	digest := "sha256:" + sha256Hex(artifact)
	plan := strings.Repeat("a", 64)
	raw, err := EncodeAggregateReport(AggregateReport{Profile: AggregateReportProfile, Base: base, Target: base, CompletionState: "complete", Steps: []AggregateReportStep{{"local-outcome", "PRODUCED", "none"}, {queryStep, "NOT_PRODUCED", queryAbstentionReason}}, OCMStatus: json.RawMessage(`{}`), LocalOutcomeEvidenceSHA256: "sha256:" + plan, QueryAbstentionEvidenceSHA256: &digest, Anchor: AggregateReportAnchor{State: "NOT_OBSERVED"}, OCMLinkPlan: json.RawMessage(`null`), PacketCoverage: []json.RawMessage{}, Enrollment: AggregateEnrollment{strings.Repeat("b", 64), plan + "-001", plan, "sha256:" + plan}, LocalOutcomeProfile: tracerecordrepo.AggregateOutcomeProfile})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseAggregateReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	c := newCheck(context.Background(), "dogfood-check", CheckOptions{Root: root, BaseVerifier: runner, TreeVerifier: runner}, &out, &stderr)
	c.aggregate = &parsed
	c.base, c.target, c.evidence = base, base, evidence
	code, err := boundary(c.ctx, func() int {
		retained, claimed := c.checkQueryAbstention(raw)
		if !claimed || retained != task {
			t.Fatal("bound QAT lost")
		}
		c.resolveVerifiers()
		c.verifyQueryAbstention(retained)
		return 0
	}, func() {})
	if err != nil || code != 0 {
		t.Fatalf("strict replay: %d %v %s", code, err, stderr.String())
	}
	after, err := os.ReadFile(trace)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("query changed trace", err)
	}
}

// TestSealRefusesToDropAnUnarchivedBaseCEM reproduces V1-0137: a change that
// replaced BASE's tracked CEM must not seal while no .corvint/changes/ file
// keeps BASE's copy, and seals once one does.
func TestSealRefusesToDropAnUnarchivedBaseCEM(t *testing.T) {
	root := t.TempDir()
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, root, "init", "-q")
	write(".corvint/change.cem.json", "earlier change\n")
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-q", "-m", "earlier bind")
	earlier := testGit(t, root, "rev-parse", "HEAD")
	write("a.txt", "change\n")
	write(".corvint/change.cem.json", "this change\n")
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-q", "-m", "bind")
	seal := func() (int, string) {
		var stdout, stderr bytes.Buffer
		run := newCheck(context.Background(), "dogfood-seal", CheckOptions{Root: root}, &stdout, &stderr)
		run.base = earlier
		code, err := boundary(context.Background(), run.seal, func() {})
		if err != nil {
			t.Fatal(err)
		}
		return code, stderr.String()
	}
	code, stderr := seal()
	want := "dogfood-seal: REFUSE unarchived-base-cem\n  BASE tracks .corvint/change.cem.json (bound at " + earlier + ")"
	if code != 2 || !strings.HasPrefix(stderr, want) {
		t.Fatalf("seal exit=%d stderr=%q; want exit 2 and %q", code, stderr, want)
	}
	// DCW-V0-043: the refusal names the exact archive command.
	blob, archive := testGit(t, root, "rev-parse", earlier+":.corvint/change.cem.json"), ".corvint/changes/"+earlier+".cem.json"
	fix := "  fix: mkdir -p .corvint/changes && git cat-file blob " + blob + " > " + archive + " && git add " + archive +
		" && git commit -q -m \"chore: archive inherited change evidence\" -- " + archive + "\n" +
		"  then rerun dogfood-change, bind, dogfood-check and dogfood-seal with the same BASE (DCW-V0-043)\n"
	if !strings.HasSuffix(stderr, fix) {
		t.Fatalf("seal stderr=%q; want suffix %q", stderr, fix)
	}
	if head := testGit(t, root, "log", "-1", "--format=%s"); head != "bind" {
		t.Fatalf("refused seal committed %q", head)
	}
	write(".corvint/changes/"+earlier+".cem.json", "earlier change\n")
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-q", "-m", "bind with the earlier CEM archived")
	if code, stderr = seal(); code != 0 {
		t.Fatalf("archived seal exit=%d stderr=%q", code, stderr)
	}
}

// TestFlowGitIgnoresAmbientConfigurationAndReplaceRefs reproduces V1-0362: the
// flow's Git reads run under Core's sanitized environment, so a global
// fsmonitor hook never runs and a replace ref never rewrites a read commit.
func TestFlowGitIgnoresAmbientConfigurationAndReplaceRefs(t *testing.T) {
	root, scratch := t.TempDir(), t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "commit", "-q", "--allow-empty", "-m", "first")
	first := testGit(t, root, "rev-parse", "HEAD")
	testGit(t, root, "commit", "-q", "--allow-empty", "-m", "second")
	testGit(t, root, "replace", first, testGit(t, root, "rev-parse", "HEAD"))
	marker, hook, global := filepath.Join(scratch, "hook-ran"), filepath.Join(scratch, "fsmonitor"), filepath.Join(scratch, "gitconfig")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\ntouch '"+marker+"'\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(global, []byte("[core]\n\tfsmonitor = "+hook+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	run := &flow{ctx: context.Background(), root: root, stderr: io.Discard}
	if subject := run.gitValue("log", "-1", "--format=%s", first); subject != "first" {
		t.Fatalf("replace ref rewrote the read commit: subject %q", subject)
	}
	run.gitValue("status", "--porcelain", "--untracked-files=all")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("global fsmonitor hook ran: %v", err)
	}
}

func TestAggregateReportAllMemberShapes(t *testing.T) {
	// Nullable and opaque fields are explicit controls, not invented refusal
	// requirements. Their missing member still violates the closed schema.
	value := AggregateReport{Profile: AggregateReportProfile, Base: strings.Repeat("1", 40), Target: strings.Repeat("2", 40), CompletionState: "incomplete", Steps: []AggregateReportStep{{"local-outcome", "PRODUCED", "none"}}, OCMStatus: json.RawMessage("null"), LocalOutcomeEvidenceSHA256: "sha256:" + strings.Repeat("a", 64), Anchor: AggregateReportAnchor{State: "NOT_OBSERVED"}, OCMLinkPlan: json.RawMessage("null"), PacketCoverage: []json.RawMessage{}, Enrollment: AggregateEnrollment{strings.Repeat("b", 64), strings.Repeat("c", 64) + "-001", strings.Repeat("c", 64), "sha256:" + strings.Repeat("d", 64)}, LocalOutcomeProfile: tracerecordrepo.AggregateOutcomeProfile}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParseAggregateReport(raw); err != nil {
		t.Fatalf("control: %v", err)
	}
	members := []string{"profile", "base", "target", "completionState", "steps", "ocmStatus", "localOutcomeEvidenceSha256", "contextAbstentionEvidenceSha256", "queryAbstentionEvidenceSha256", "anchor", "ocmLinkPlan", "dogfoodPolicy", "packetCoverage", "dogfoodCheck", "enrollment", "localOutcomeProfile"}
	nullable := map[string]bool{"ocmStatus": true, "contextAbstentionEvidenceSha256": true, "queryAbstentionEvidenceSha256": true, "ocmLinkPlan": true, "packetCoverage": true, "dogfoodCheck": true}
	for _, member := range members {
		for _, shape := range []string{"missing", "null", "wrong-type"} {
			if nullable[member] && shape == "null" || (member == "ocmStatus" || member == "ocmLinkPlan") && shape == "wrong-type" {
				continue
			}
			if member == "completionState" && (shape == "missing" || shape == "null") {
				continue
			} // accepted existing cells.
			t.Run(member+"/"+shape, func(t *testing.T) {
				var m map[string]any
				if err := json.Unmarshal(raw, &m); err != nil {
					t.Fatal(err)
				}
				switch shape {
				case "missing":
					delete(m, member)
				case "null":
					m[member] = nil
				case "wrong-type":
					m[member] = true
				}
				mutant, err := json.Marshal(m)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = ParseAggregateReport(mutant); err == nil {
					t.Fatal("closed report mutation accepted")
				}
			})
		}
	}
	for group, members := range map[string][]string{"enrollment": {"session", "generation", "planDigest", "bindingSha256"}, "dogfoodPolicy": {"bootstrapUnknown", "maximumUnknownAfterBootstrap"}, "anchor": {"state", "mergeBase"}} {
		for _, member := range members {
			for _, shape := range []string{"missing", "null", "wrong-type"} {
				if member == "mergeBase" && shape == "null" {
					continue
				}
				t.Run(group+"/"+member+"/"+shape, func(t *testing.T) {
					var m map[string]any
					if err := json.Unmarshal(raw, &m); err != nil {
						t.Fatal(err)
					}
					nested := m[group].(map[string]any)
					switch shape {
					case "missing":
						delete(nested, member)
					case "null":
						nested[member] = nil
					case "wrong-type":
						nested[member] = true
					}
					mutant, err := json.Marshal(m)
					if err != nil {
						t.Fatal(err)
					}
					if _, err = ParseAggregateReport(mutant); err == nil {
						t.Fatal("closed nested report mutation accepted")
					}
				})
			}
		}
	}
}

// DCW-V0-033: the PASS summary names states and counts from the status JSON and
// prints NOT_OBSERVED for anything absent or not a plain token.
func TestDCWV0033CheckSummaryAbstainsOnUnobservedFields(t *testing.T) {
	t.Parallel()
	cem := []byte(`{"state":"ready-for-ci","counts":{"total":3,"supported":2,"unknown":1,"mechanical":0},"tool":"cem-status"}` + "\n")
	// OCM-V0-019: the requirement members come from the change's own
	// coverage, never the aggregate's whole intent scope (total 56 here).
	ocm := []byte(`{"aggregate":{"state":"ready-for-review","coverage":{"total":56,"linked":0,"unknown":56}},"tool":"dogfood-ocm-status"}` + "\n")
	unobserved := " requirements=NOT_OBSERVED linked=NOT_OBSERVED unlinked=NOT_OBSERVED"
	cases := []struct {
		name     string
		cem, ocm []byte
		noIntent bool
		coverage *changeCoverage
		want     string
	}{
		{"complete", cem, ocm, false, &changeCoverage{requirements: 2, linked: 1}, "cem=ready-for-ci hunks=3 supported=2 unknown=1 mechanical=0 unknownShare=1/3 reasonedUnknown=0 ocm=ready-for-review requirements=2 linked=1 unlinked=1"},
		{"cites none", cem, ocm, false, &changeCoverage{}, "cem=ready-for-ci hunks=3 supported=2 unknown=1 mechanical=0 unknownShare=1/3 reasonedUnknown=0 ocm=ready-for-review requirements=0 linked=0 unlinked=0"},
		{"coverage unread", cem, ocm, false, nil, "cem=ready-for-ci hunks=3 supported=2 unknown=1 mechanical=0 unknownShare=1/3 reasonedUnknown=0 ocm=ready-for-review" + unobserved},
		{"no intent", cem, []byte("dogfood-check: NOTE intent-linkage NOT_ASSESSED no-intent-declared\n"), true, nil, "cem=ready-for-ci hunks=3 supported=2 unknown=1 mechanical=0 unknownShare=1/3 reasonedUnknown=0 ocm=NOT_ASSESSED"},
		{"malformed", []byte("not json\n"), []byte("{}\n"), false, nil, "cem=NOT_OBSERVED hunks=NOT_OBSERVED unknownShare=NOT_OBSERVED reasonedUnknown=0 ocm=NOT_OBSERVED"},
		{"missing counts", []byte(`{"state":"ready-for-ci","counts":{"total":1}}`), []byte(`{"aggregate":{}}`), false, nil, "cem=ready-for-ci hunks=1 supported=NOT_OBSERVED unknown=NOT_OBSERVED mechanical=NOT_OBSERVED unknownShare=NOT_OBSERVED reasonedUnknown=0 ocm=NOT_OBSERVED" + unobserved},
		{"forged token", []byte(`{"state":"ok\ndogfood-check: PASS","counts":{}}`), []byte(`{"aggregate":{"state":"a b"}}`), false, nil, "cem=NOT_OBSERVED hunks=NOT_OBSERVED supported=NOT_OBSERVED unknown=NOT_OBSERVED mechanical=NOT_OBSERVED unknownShare=NOT_OBSERVED reasonedUnknown=0 ocm=NOT_OBSERVED" + unobserved},
	}
	for _, tc := range cases {
		want := "dogfood-check: SUMMARY " + tc.want + " report=.corvint/dogfood-report.json detail=/g/corvint/dogfood-check.stdout"
		if got := checkSummary(tc.cem, tc.ocm, tc.noIntent, 0, tc.coverage, "", "/g/corvint/dogfood-check.stdout"); got != want {
			t.Errorf("%s\n got %s\nwant %s", tc.name, got, want)
		}
	} // DCW-V0-046: the share is unknown over material hunks, mechanical ones
	// excluded, and the reasoned count is the check's own allowance.
	mixed := []byte(`{"state":"ready-for-ci","counts":{"total":4,"supported":1,"unknown":2,"mechanical":1}}` + "\n")
	want := "dogfood-check: SUMMARY cem=ready-for-ci hunks=4 supported=1 unknown=2 mechanical=1 unknownShare=2/3 reasonedUnknown=1 ocm=NOT_ASSESSED report=.corvint/dogfood-report.json detail=/d"
	if got := checkSummary(mixed, nil, true, 1, nil, "", "/d"); got != want {
		t.Errorf("mixed\n got %s\nwant %s", got, want)
	}
	inconsistent := []byte(`{"state":"ready-for-ci","counts":{"total":1,"supported":0,"unknown":1,"mechanical":1}}` + "\n")
	if got := checkSummary(inconsistent, nil, true, 0, nil, "", "/d"); !strings.Contains(got, " unknownShare=NOT_OBSERVED ") {
		t.Errorf("inconsistent counts: %s", got)
	}
}

// TestSidecarBaseRevisionReadsAnyLayout pins V1-1326: the previous binding's
// base is read from the committed sidecar's top-level member whatever its JSON
// layout, so the compact cem/1.0 map resolves as the indented one did, and a
// sidecar with no single top-level base has none.
func TestSidecarBaseRevisionReadsAnyLayout(t *testing.T) {
	base := strings.Repeat("ab", 20)
	for _, tc := range []struct {
		name, sidecar string
		want          bool
	}{
		{"compact cem/1.0", `{"baseRevision":"` + base + `","hunks":[{"id":"h","baseRevision":"x"}],"spec":"cem/1.0"}` + "\n", true},
		{"indented", "{\n  \"baseRevision\": \"" + base + "\",\n  \"spec\": \"cem/1.0\"\n}\n", true},
		{"member last", `{"spec":"cem/1.0","verification":{"baseRevision":"x"},"baseRevision":"` + base + `"}`, true},
		{"nested only", `{"spec":"cem/1.0","verification":{"baseRevision":"` + base + `"}}`, false},
		{"repeated", `{"baseRevision":"` + base + `","baseRevision":"` + base + `"}`, false},
		{"not a string", `{"baseRevision":40}`, false},
		{"trailing bytes", `{"baseRevision":"` + base + `"} {}`, false},
		{"not an object", `["baseRevision","` + base + `"]`, false},
		{"truncated", `{"baseRevision":"` + base + `"`, false},
	} {
		got, ok := sidecarBaseRevision([]byte(tc.sidecar))
		if ok != tc.want || (ok && got != base) {
			t.Errorf("%s: got %q %v, want ok=%v", tc.name, got, ok, tc.want)
		}
	}
}

// TestOCMV0019SummaryCountsRequirementsTheChangeCitesOrTraces pins OCM-V0-019
// and OCM-V0-020: the SUMMARY's requirements are the declared requirements a
// bound map cites or the diff edits, linked those a test, traceability row or
// linked obligation names as an exact token, and a map that cites none counts
// zero, not the intent's whole requirements section.
func TestOCMV0019SummaryCountsRequirementsTheChangeCitesOrTraces(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	// A configured inter-hunk context must not merge the code-only commit's
	// two spec edits into one hunk over the requirements between them.
	testGit(t, root, "config", "diff.interHunkContext", "20")
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(message string) string {
		t.Helper()
		testGit(t, root, "add", "-A", ".")
		testGit(t, root, "commit", "-q", "-m", message)
		return testGit(t, root, "rev-parse", "HEAD")
	}
	spec := "# Spec\n\n## Requirements\n\n- `FIX-V0-001`: one.\n  continued.\n- `FIX-V0-002`: two.\n- `FIX-V0-003`: three.\n" +
		"```\n- `FIX-V0-009`: fenced, still enumerated as OCM-V0-001 does.\n```\n\n## Traceability\n\n| FIX-V0-001 | a_test.go |\n"
	write("spec.md", spec)
	write("a_test.go", "package a\n// FIX-V0-002 case, and FIX-V0-003-extra is a longer token.\n")
	write("code.go", "package a\n")
	base := commit("base")
	write("code.go", "package a\n\nconst answer = 2\n")
	write("spec.md", strings.Replace(strings.Replace(spec, "# Spec\n", "# Spec, revised\n", 1), "## Traceability\n\n", "## Traceability\n\nRows:\n", 1))
	codeOnly := commit("code only")
	write("spec.md", strings.Replace(spec, "- `FIX-V0-003`: three.\n", "- `FIX-V0-003`: three.\n- `FIX-V0-004`: four.\n- `FIX-V0-005`: five.\n", 1)+"| FIX-V0-004 | b_test.go |\n")
	write("b_test.sh", "# FIX-V0-005 shell case\n")
	addsTwo := commit("adds two traced requirements")
	blob := testGit(t, root, "rev-parse", base+":spec.md")
	span := func(from, to string) string {
		start, end := strings.Index(spec, from), strings.Index(spec, to)+len(to)
		return `{"blobOid":"` + blob + `","path":"spec.md","span":{"end":` + strconv.Itoa(end) + `,"start":` + strconv.Itoa(start) + `}}`
	}
	cases := []struct {
		name, target, evidence, ocm string
		want                        *changeCoverage
	}{
		{"cites two traced IDs", codeOnly, span("continued.", "two."), "", &changeCoverage{requirements: 2, linked: 2}},
		{"cites an unresolved ID", codeOnly, span("- `FIX-V0-003`", "three."), "", &changeCoverage{requirements: 1, unlinked: "FIX-V0-003"}},
		{"names each unresolved ID", codeOnly, span("- `FIX-V0-003`", "three.") + "," + span("- `FIX-V0-001`", "continued."), "", &changeCoverage{requirements: 2, linked: 1, unlinked: "FIX-V0-003"}},
		{"a linked obligation traces it", codeOnly, span("- `FIX-V0-003`", "three."), `{"obligations":[{"disposition":"linked","id":"FIX-V0-003"}]}`, &changeCoverage{requirements: 1, linked: 1}},
		{"cites none", codeOnly, "", "", &changeCoverage{}},
		{"cites outside the section", codeOnly, span("# Spec", "# Spec") + "," + span("| FIX-V0-001", "a_test.go |"), "", &changeCoverage{}},
		{"adds two traced IDs", addsTwo, "", "", &changeCoverage{requirements: 2, linked: 2}},
	}
	for _, tc := range cases {
		write(".corvint/change.cem.json", `{"evidence":[`+tc.evidence+`],"hunks":[]}`)
		ocm := tc.ocm
		if ocm == "" {
			ocm = `{"obligations":[]}`
		}
		write(".corvint/change.ocm.001.json", ocm)
		run := &flow{ctx: context.Background(), root: root, stderr: io.Discard, base: base, target: tc.target}
		got := run.changeCoverage(filepath.Join(root, ".corvint/change.cem.json"), []string{"spec.md"})
		if got == nil || *got != *tc.want {
			t.Errorf("%s: coverage %+v, want %+v", tc.name, got, tc.want)
		}
	}
	// OCM-V0-001 enumeration: a backtick run whose info string holds a backtick
	// opens no fence, and a requirement line inside a real fence still counts.
	for blob, want := range map[string]string{
		"## Requirements\n\n- `ABC-V0-001`: a.\n```x`y\n## Next\n- `ABC-V0-002`: b.\n":                 "ABC-V0-001",
		"## Requirements\n\n```\n- `ABC-V0-001`: fenced.\n## Not a heading\n```\n- `ABC-V0-002`: b.\n": "ABC-V0-001 ABC-V0-002",
	} {
		var ids []string
		for _, item := range parseRequirements([]byte(blob)).items {
			ids = append(ids, item.id)
		}
		if got := strings.Join(ids, " "); got != want {
			t.Errorf("parseRequirements(%q) = %q, want %q", blob, got, want)
		}
	}
	// A map without an evidence member, or an intent absent at the target,
	// is not read as zero: the SUMMARY prints NOT_OBSERVED.
	run := &flow{ctx: context.Background(), root: root, stderr: io.Discard, base: base, target: codeOnly}
	write(".corvint/change.cem.json", `{"hunks":[]}`)
	if got := run.changeCoverage(filepath.Join(root, ".corvint/change.cem.json"), []string{"spec.md"}); got != nil {
		t.Errorf("map without evidence: %+v", got)
	}
	write(".corvint/change.cem.json", `{"evidence":[]}`)
	if got := run.changeCoverage(filepath.Join(root, ".corvint/change.cem.json"), []string{"absent.md"}); got != nil {
		t.Errorf("absent intent: %+v", got)
	}
}

// OCM-V0-025: the check names each unlinked requirement ID on stderr, bounded,
// and prints nothing when every referenced requirement is linked or the
// coverage could not be read.
func TestOCMV0025CheckNamesUnlinkedRequirementIDs(t *testing.T) {
	if got := (&changeCoverage{requirements: 3, linked: 1, unlinked: "FIX-V0-002,FIX-V0-003"}).unlinkedNote(); got != "dogfood-check: NOTE unlinked-requirements count=2 ids=FIX-V0-002,FIX-V0-003" {
		t.Fatalf("note %q", got)
	}
	for _, coverage := range []*changeCoverage{nil, {}, {requirements: 2, linked: 2}} {
		if got := coverage.unlinkedNote(); got != "" {
			t.Fatalf("%+v note %q", coverage, got)
		}
	}
	ids := []string{}
	for i := 1; i <= maxNamedUnlinked+2; i++ {
		ids = append(ids, fmt.Sprintf("FIX-V0-%03d", i))
	}
	got := (&changeCoverage{requirements: len(ids), unlinked: strings.Join(ids, ",")}).unlinkedNote()
	if want := fmt.Sprintf("count=%d ids=%s (+2 more)", len(ids), strings.Join(ids[:maxNamedUnlinked], ",")); !strings.HasSuffix(got, want) {
		t.Fatalf("bounded note %q, want suffix %q", got, want)
	}
}

// TestSealDCWV0166CommitsUnderTheEnvironmentIdentity reproduces V1-2404: the
// seal commit takes its author and committer from GIT_AUTHOR_* and
// GIT_COMMITTER_*, not from a global configuration identity.
func TestSealDCWV0166CommitsUnderTheEnvironmentIdentity(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "commit", "-q", "--allow-empty", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", "change.cem.json"), []byte("{}\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-q", "-m", "bind")
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[user]\n\tname = global\n\temail = global@example.invalid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_AUTHOR_NAME", "Seal Author")
	t.Setenv("GIT_AUTHOR_EMAIL", "author@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Seal Committer")
	t.Setenv("GIT_COMMITTER_EMAIL", "committer@example.invalid")
	var stdout, stderr bytes.Buffer
	run := newCheck(context.Background(), "dogfood-seal", CheckOptions{Root: root}, &stdout, &stderr)
	run.base = base
	code, err := boundary(context.Background(), run.seal, func() {})
	if err != nil || code != 0 {
		t.Fatalf("seal exit=%d err=%v stderr=%q", code, err, stderr.String())
	}
	got := testGit(t, root, "log", "-1", "--format=%s|%an|%ae|%cn|%ce")
	if want := "chore: seal change evidence|Seal Author|author@example.invalid|Seal Committer|committer@example.invalid"; got != want {
		t.Fatalf("seal commit identity %q; want %q", got, want)
	}
}
