package cemcandidate

import (
	"context"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
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
	cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", dir}, args...)...)
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

// TestAssembleRefusesRetiredAndStableSources: both assemblies converted only a
// cem/0.x source, which is retired; a 0.x source refuses with the retired-spec
// fix and a cem/1.0 source refuses as already stable, and neither writes output
// (CEM-V1-210).
func TestAssembleRefusesRetiredAndStableSources(t *testing.T) {
	stableRaw, e := os.ReadFile(filepath.Join("..", "..", "protocol", "cem-1.0", "stable", "repository-envelope-packet", "inherited", "fixtures", "maps", "sha1-positive-sealed.json"))
	if e != nil {
		t.Fatal(e)
	}
	r, out := fixture(t, "sha1")
	s, stableOut := stableFixture(t, "sha1")
	for _, c := range []struct {
		name, want string
		source     []byte
	}{{"retired", cw.RetiredSpecFix, nil}, {"stable", "needs no assembly", stableRaw}} {
		if c.source != nil {
			r.SourceMap = save(t, r.SourceMap.Path, c.source)
			s.SourceMap = r.SourceMap
		}
		_, e := Assemble(t.Context(), r, out)
		_, se := AssembleStable(t.Context(), s, stableOut)
		for _, got := range []error{e, se} {
			if cemcode.CodeOf(got) != cemcode.UnsupportedSpec || !strings.Contains(got.Error(), c.want) {
				t.Fatalf("%s source: %v", c.name, got)
			}
		}
		for _, p := range []string{out, stableOut} {
			if _, e := os.Lstat(p); !os.IsNotExist(e) {
				t.Fatalf("%s source wrote output %s: %v", c.name, p, e)
			}
		}
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
