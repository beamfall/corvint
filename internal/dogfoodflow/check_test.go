package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
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
