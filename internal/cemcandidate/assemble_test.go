package cemcandidate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	cw "github.com/Beamfall/corvint/internal/cem/wire"
	tw "github.com/Beamfall/corvint/internal/tasks/wire"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func realTemp(t *testing.T) string {
	t.Helper()
	p, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Candidate Proof", "GIT_AUTHOR_EMAIL=proof@example.invalid", "GIT_COMMITTER_NAME=Candidate Proof", "GIT_COMMITTER_EMAIL=proof@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	b, e := cmd.Output()
	if e != nil {
		t.Fatalf("git %v: %v", args, e)
	}
	return strings.TrimSpace(string(b))
}
func save(t *testing.T, p string, raw []byte) FileRef {
	t.Helper()
	if e := os.WriteFile(p, raw, 0600); e != nil {
		t.Fatal(e)
	}
	return FileRef{p, tr.Digest(raw)}
}
func encode(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

// Native artifacts here are manufactured wire fixtures, not Tasks semantic or
// runner execution evidence. A separate live proof exercises actual artifacts.
func fixture(t *testing.T, format string) (Request, string) {
	t.Helper()
	kit := filepath.Join("..", "..", "protocol", "cem-1.0")
	repo := realTemp(t)
	git(t, repo, "init", "-q", "--object-format="+format)
	for _, name := range []string{"app.txt", "rule.txt"} {
		b, e := os.ReadFile(filepath.Join(kit, "repository", "base", name))
		if e != nil {
			t.Fatal(e)
		}
		save(t, filepath.Join(repo, name), b)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-qm", "candidate base")
	base := git(t, repo, "rev-parse", "HEAD")
	save(t, filepath.Join(repo, "app.txt"), []byte("after\n"))
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-qm", "candidate content")
	target := git(t, repo, "rev-parse", "HEAD")
	raw, e := os.ReadFile(filepath.Join(kit, "maps", format+"-valid.json"))
	if e != nil {
		t.Fatal(e)
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	for _, key := range []string{"criterionBindings", "criterionLinks", "runnerReceipts", "artifacts"} {
		delete(m, key)
	}
	m["spec"] = json.RawMessage(`"cem/0.2"`)
	var original map[string]any
	_ = json.Unmarshal(raw, &original)
	hunk := original["hunks"].([]any)[0].(map[string]any)["id"].(string)
	evidence := original["evidence"].([]any)[0].(map[string]any)["id"].(string)
	dir := realTemp(t)
	ticket, e := tw.ParseTicketID("test", "ticket:example:main:TEST-1")
	if e != nil {
		t.Fatal(e)
	}
	id := tw.CriterionIdentity{ExecutableSHA256: tw.Digest(strings.Repeat("a", 64)), Version: "test", Build: "0"}
	c := tw.CriterionCapture{Producer: id, Policy: "opaque manufactured policy", Ticket: "opaque manufactured ticket", Queue: "opaque manufactured queue", Attempt: "opaque manufactured attempt", ClaimedTicket: "opaque manufactured claimed ticket"}
	capture := tw.EncodeFile(c.Value())
	digest := tw.Digest(strings.Repeat("b", 64))
	binding := tw.CriterionBinding{Snapshot: tw.CriterionSnapshot{HeadSeq: "1", HeadReceiptSHA256: digest, IntentTreeSHA256: digest, PrimaryWorktreeSHA256: digest}, TicketID: ticket, AcceptanceRevision: "1", AcceptanceCriteria: []string{"observed behavior"}, AttemptID: "attempt:example:main:0123456789abcdef0123456789abcdef", Generation: "1", Phase: "build", BaseCommit: base, PolicySHA256: digest, PolicyContentIDSHA256: digest, ConfigSHA256: digest, RuntimeID: "external-agent", LeaseExpiresAt: "2026-01-01T01:00:00Z"}
	v := tw.CriterionVerification{Producer: id, Verifier: id, CaptureSHA256: tw.Sum(capture), Binding: binding}
	plan := tr.PlanDocument{Profile: "corvint-test-runner-plan/0", Request: tr.Request{Runner: "go-test", Root: repo, InputFiles: map[string]string{"app.txt": tr.Digest([]byte("after\n"))}}, Invocation: tr.Invocation{Argv: []string{"not-executed"}}}
	receipt := tr.ReceiptDocument{Profile: "corvint-test-runner-receipt/0", PlanSha256: tr.Identity(plan), Execution: tr.Execution{Profile: "corvint-test-runner-execution/0", Runner: plan.Request.Runner, InputSha256: tr.Identity(plan.Request.InputFiles), InvocationSha256: tr.Identity(plan.Invocation), ExecutionAuthority: "CALLER_OBSERVED", DependencyClosure: "NOT_OBSERVED"}, Observation: tr.Observation{Runner: plan.Request.Runner, Complete: false, Tests: []tr.Test{{ID: "fixture", State: tr.Unknown}}}, Error: "manufactured observation"}
	r := Request{Profile: Profile, Repository: repo, ExpectedBase: base, Target: target, TicketID: ticket.Raw, AttemptID: binding.AttemptID, SourceMap: save(t, filepath.Join(dir, "source.json"), encode(t, m)), Capture: save(t, filepath.Join(dir, "capture.json"), capture), Verification: save(t, filepath.Join(dir, "verification.json"), tw.EncodeFile(v.Value())), RunnerPlan: save(t, filepath.Join(dir, "plan.json"), encode(t, plan)), RunnerReceipt: save(t, filepath.Join(dir, "receipt.json"), encode(t, receipt)), Links: []Link{{CriterionIndex: 0, HunkIDs: []string{hunk}, EvidenceIDs: []string{evidence}}}}
	return r, filepath.Join(realTemp(t), "bundle")
}
func TestAssembleReferenceIntegrityAndDeclaredSource(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			r, out := fixture(t, format)
			result, e := Assemble(t.Context(), r, out)
			if e != nil {
				t.Fatal(e)
			}
			if result.Verification.Integrity != "VERIFIED" || result.DeclaredInputGitBinding != "DECLARED_INPUT_BYTES_MATCH_TARGET" || result.ExecutionAtCommit != "NOT_OBSERVED" || result.NativeCaptureSemanticVerification != "NOT_OBSERVED" {
				t.Fatalf("overstated result %+v", result)
			}
			for _, v := range result.Verification.Limits {
				if v != "NOT_OBSERVED" {
					t.Fatal("promoted authority")
				}
			}
			raw, e := Read(result.CandidatePath)
			if e != nil {
				t.Fatal(e)
			}
			c, e := cw.ParseCandidate(raw)
			if e != nil {
				t.Fatal(e)
			}
			if len(c.Criteria) != 1 || c.Criteria[0].TicketID != r.TicketID {
				t.Fatal("wrong criterion identity")
			}
			for name, pin := range map[string]FileRef{"runner-plan": r.RunnerPlan, "runner-receipt": r.RunnerReceipt, "tasks-capture": r.Capture, "tasks-verification": r.Verification} {
				b, e := Read(filepath.Join(out, "artifacts", name+".json"))
				if e != nil || tr.Digest(b) != pin.Sha256 {
					t.Fatalf("original bytes lost: %s %v", name, e)
				}
			}
			// Only app.txt was declared, despite rule.txt also existing. The successful
			// result must retain dependency/source execution unknowns for that subset.
			if result.Verification.Limits["sourceGitBinding"] != "NOT_OBSERVED" {
				t.Fatal("subset became execution source closure")
			}
		})
	}
}
func TestAssembleRetainsEveryNativeOutcome(t *testing.T) {
	r, baseOut := fixture(t, "sha1")
	raw, e := Read(r.RunnerReceipt.Path)
	if e != nil {
		t.Fatal(e)
	}
	var receipt tr.ReceiptDocument
	if e := Decode(raw, &receipt); e != nil {
		t.Fatal(e)
	}
	states := []string{tr.Passed, tr.Failed, tr.Skipped, tr.Flaky, tr.TimedOut, tr.Interrupted, tr.Unknown}
	kinds := []string{"", tr.Assertion, tr.BuildFailure, tr.CollectionFailure, tr.Infrastructure}
	for _, state := range states {
		for _, kind := range kinds {
			receipt.Observation.Tests = []tr.Test{{ID: "native", State: state, FailureKind: kind, Attempts: []tr.Attempt{{State: state, FailureKind: kind, Message: "original retry"}}}}
			receipt.Observation.Complete = state == tr.Passed
			encoded := encode(t, receipt)
			r.RunnerReceipt = save(t, r.RunnerReceipt.Path, encoded)
			out := baseOut + "-" + state + "-" + kind
			result, e := Assemble(t.Context(), r, out)
			if e != nil {
				t.Fatal(e)
			}
			got, e := Read(filepath.Join(out, "artifacts/runner-receipt.json"))
			if e != nil || !bytes.Equal(encoded, got) {
				t.Fatal("native outcome rewritten")
			}
			if result.Verification.Limits["criterionDiscrimination"] != "NOT_OBSERVED" {
				t.Fatal("failure became discrimination")
			}
		}
	}
}
func TestAssembleRefusesMismatchedReferences(t *testing.T) {
	for _, kind := range []string{"ticket", "attempt", "base", "tree", "criterion", "hunk", "evidence", "plan-hash", "input-hash", "invocation-hash", "source-bytes", "git-path", "changed-artifact", "cem03", "old-sidecar", "inside-repo", "overwrite"} {
		t.Run(kind, func(t *testing.T) {
			r, out := fixture(t, "sha1")
			mutateReceipt := func(f func(*tr.ReceiptDocument)) {
				raw, _ := Read(r.RunnerReceipt.Path)
				var x tr.ReceiptDocument
				if e := Decode(raw, &x); e != nil {
					t.Fatal(e)
				}
				f(&x)
				r.RunnerReceipt = save(t, r.RunnerReceipt.Path, encode(t, x))
			}
			switch kind {
			case "ticket":
				r.TicketID = "ticket:example:main:OTHER"
			case "attempt":
				r.AttemptID = "other-attempt"
			case "base":
				r.ExpectedBase = r.Target
			case "tree":
				raw, _ := Read(r.Verification.Path)
				value, e := tw.Parse(raw)
				if e != nil {
					t.Fatal(e)
				}
				v, e := tw.ReadCriterionVerification(value)
				if e != nil {
					t.Fatal(e)
				}
				wrong := r.ExpectedBase
				v.Binding.CandidateTreeOID = &wrong
				r.Verification = save(t, r.Verification.Path, tw.EncodeFile(v.Value()))
			case "criterion":
				r.Links[0].CriterionIndex = 1
			case "hunk":
				r.Links[0].HunkIDs[0] = "hunk:sha256:" + strings.Repeat("0", 64)
			case "evidence":
				r.Links[0].EvidenceIDs[0] = "evidence:sha256:" + strings.Repeat("0", 64)
			case "plan-hash":
				mutateReceipt(func(x *tr.ReceiptDocument) { x.PlanSha256 = r.RunnerPlan.Sha256 + "x" })
			case "input-hash":
				mutateReceipt(func(x *tr.ReceiptDocument) { x.Execution.InputSha256 = strings.Repeat("0", 64) })
			case "invocation-hash":
				mutateReceipt(func(x *tr.ReceiptDocument) { x.Execution.InvocationSha256 = strings.Repeat("0", 64) })
			case "source-bytes", "git-path":
				raw, _ := Read(r.RunnerPlan.Path)
				var p tr.PlanDocument
				if e := Decode(raw, &p); e != nil {
					t.Fatal(e)
				}
				if kind == "source-bytes" {
					p.Request.InputFiles["app.txt"] = tr.Digest([]byte("different"))
				} else {
					p.Request.InputFiles = map[string]string{".GiT/config": strings.Repeat("0", 64)}
				}
				r.RunnerPlan = save(t, r.RunnerPlan.Path, encode(t, p))
				mutateReceipt(func(x *tr.ReceiptDocument) {
					x.PlanSha256 = tr.Identity(p)
					x.Execution.InputSha256 = tr.Identity(p.Request.InputFiles)
				})
			case "changed-artifact":
				save(t, r.RunnerReceipt.Path, []byte("changed"))
			case "cem03":
				raw, _ := Read(r.SourceMap.Path)
				r.SourceMap = save(t, r.SourceMap.Path, bytes.Replace(raw, []byte("cem/0.2"), []byte("cem/0.3"), 1))
			case "old-sidecar":
				if e := os.Mkdir(filepath.Join(r.Repository, ".corvint"), 0700); e != nil {
					t.Fatal(e)
				}
				raw, _ := Read(r.SourceMap.Path)
				save(t, filepath.Join(r.Repository, cw.ExcludedCEMPath), raw)
				git(t, r.Repository, "add", ".")
				git(t, r.Repository, "commit", "-qm", "sidecar")
				r.Target = git(t, r.Repository, "rev-parse", "HEAD")
			case "inside-repo":
				out = filepath.Join(r.Repository, "new-bundle")
			case "overwrite":
				if e := os.Mkdir(out, 0700); e != nil {
					t.Fatal(e)
				}
			}
			if _, e := Assemble(t.Context(), r, out); e == nil {
				t.Fatal("unsafe assembly admitted")
			}
			if _, e := os.Stat(filepath.Join(out, "candidate.json")); !os.IsNotExist(e) {
				t.Fatal("failed assembly published candidate")
			}
		})
	}
}
func TestNativePlanIdentityIsNotRawArtifactDigest(t *testing.T) {
	r, out := fixture(t, "sha1")
	raw, e := Read(r.RunnerPlan.Path)
	if e != nil {
		t.Fatal(e)
	}
	var p tr.PlanDocument
	if e := Decode(raw, &p); e != nil {
		t.Fatal(e)
	}
	formatted, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	formatted = append(formatted, '\n')
	r.RunnerPlan = save(t, r.RunnerPlan.Path, formatted)
	if r.RunnerPlan.Sha256 == tr.Identity(p) {
		t.Fatal("test requires distinct identities")
	}
	result, e := Assemble(t.Context(), r, out)
	if e != nil {
		t.Fatal(e)
	}
	candidate, e := Read(result.CandidatePath)
	if e != nil {
		t.Fatal(e)
	}
	c, e := cw.ParseCandidate(candidate)
	if e != nil {
		t.Fatal(e)
	}
	if c.Receipts[0].PlanSha256 != r.RunnerPlan.Sha256 {
		t.Fatal("candidate used native identity as raw artifact digest")
	}
}
func TestAssemblyCancellationAndInputSymlinks(t *testing.T) {
	r, out := fixture(t, "sha1")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, e := Assemble(ctx, r, out); e == nil {
		t.Fatal("cancel ignored")
	}
	link := filepath.Join(filepath.Dir(r.RunnerPlan.Path), "linked-plan.json")
	if e := os.Symlink(r.RunnerPlan.Path, link); e != nil {
		t.Fatal(e)
	}
	r.RunnerPlan.Path = link
	if _, e := Assemble(context.Background(), r, out); e == nil {
		t.Fatal("symlink accepted")
	}
}

// TestRunnerBindingAdmitsKilledRunReceipt covers V1-1025: a runner-killed
// phase (exitCode -1) binds, but never with a complete observation.
func TestRunnerBindingAdmitsKilledRunReceipt(t *testing.T) {
	plan := tr.PlanDocument{Profile: "corvint-test-runner-plan/0", Request: tr.Request{Runner: "go-test", InputFiles: map[string]string{"app.txt": tr.Digest([]byte("after\n"))}}, Invocation: tr.Invocation{Argv: []string{"not-executed"}}}
	phase := tr.PhaseResult{Kind: "TEST", ExitCode: tr.KilledExitCode, TimedOut: true}
	receipt := tr.ReceiptDocument{Profile: "corvint-test-runner-receipt/0", PlanSha256: tr.Identity(plan), Execution: tr.Execution{Profile: "corvint-test-runner-execution/0", Runner: plan.Request.Runner, InputSha256: tr.Identity(plan.Request.InputFiles), InvocationSha256: tr.Identity(plan.Invocation), Phases: []tr.PhaseResult{phase}, ExecutionAuthority: "CALLER_OBSERVED", DependencyClosure: "NOT_OBSERVED"}, Observation: tr.Observation{Runner: plan.Request.Runner, Tests: []tr.Test{{ID: "fixture", State: tr.Unknown}}, Problems: []tr.Problem{{Code: "timeout", Detail: "runner did not finish within its admitted duration"}}}}
	raw := encode(t, receipt)
	if !bytes.Contains(raw, []byte(`"exitCode":-1`)) {
		t.Fatal("fixture lacks killed-run exit code")
	}
	if _, e := runnerBinding(encode(t, plan), raw); e != nil {
		t.Fatalf("killed-run receipt refused: %v", e)
	}
	receipt.Observation.Complete = true
	if _, e := runnerBinding(encode(t, plan), encode(t, receipt)); !errors.Is(e, tr.ErrKilledRunComplete) {
		t.Fatalf("complete killed run admitted: %v", e)
	}
	receipt.Observation.Complete = false
	receipt.Execution.Phases[0].ExitCode = -9
	if _, e := runnerBinding(encode(t, plan), encode(t, receipt)); !errors.Is(e, tr.ErrNegativeInteger) {
		t.Fatalf("unadmitted negative exit code: %v", e)
	}
}
