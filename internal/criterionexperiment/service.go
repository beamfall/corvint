package criterionexperiment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	tw "github.com/Beamfall/corvint/internal/tasks/wire"
)

func newOutput(dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("output must be absolute new disposable directory")
	}
	if e := os.Mkdir(dir, 0700); e != nil {
		return fmt.Errorf("output must not exist: %w", e)
	}
	return os.WriteFile(filepath.Join(dir, "EXPERIMENT-DISPOSABLE"), []byte(Profile+"\n"), 0600)
}
func save(dir, name string, b []byte) error { return os.WriteFile(filepath.Join(dir, name), b, 0600) }
func PlanExperiment(ctx context.Context, repoPath string, r Request, out string, tasks TasksVerifierConfig) (Plan, error) {
	var p Plan
	if e := r.validate(); e != nil {
		return p, e
	}
	repo, e := gitauth.Open(repoPath, gitrun.NewDefaultBudget())
	if e != nil {
		return p, e
	}
	defer repo.BeginObjectSession()()
	cem, e := Read(r.CEM)
	if e != nil {
		return p, e
	}
	if e = validateCEM(ctx, repo, r, cem); e != nil {
		return p, e
	}
	captures, e := capture(ctx, repoPath, r.Ticket, r.Attempt, tasks)
	if e != nil {
		return p, e
	}
	tree, e := repo.CommitTree(ctx, r.Target)
	if e != nil {
		return p, e
	}
	binding, hashes, a, e := authorityBinding(captures, tree)
	if e != nil {
		return p, e
	}
	if a.Phase != "RUNNING" || a.BaseCommit != r.Base || a.AttemptID != r.Attempt || binding.Ticket != r.Ticket {
		return p, fmt.Errorf("planning requires canonical ticket and RUNNING attempt at base")
	}
	if len(hashes) != len(r.Criteria) {
		return p, fmt.Errorf("complete native acceptance list required")
	}
	for i, c := range r.Criteria {
		if hashes[i] != c.AcceptanceSha256 {
			return p, fmt.Errorf("acceptance index/hash mismatch")
		}
		if _, _, _, e = inventory(ctx, repo, r, c, r.Target); e != nil {
			return p, e
		}
	}
	if e = checkAnchors(ctx, repo, r, captures); e != nil {
		return p, e
	}
	p = Plan{Profile, r, binding, Digest(cem), Digest(tw.EncodeFile(captures.Capture.Value())), captures.Verification.Verifier}
	if e = newOutput(out); e != nil {
		return p, e
	}
	for name, b := range map[string][]byte{"plan.json": Encode(p), "cem.json": cem, "captures.json": tw.EncodeFile(captures.Capture.Value())} {
		if e = save(out, name, b); e != nil {
			return p, e
		}
	}
	return p, nil
}
func loadPlan(ctx context.Context, repo *gitauth.Repository, planPath string, tasks TasksVerifierConfig) (Plan, error) {
	var p Plan
	b, e := Read(planPath)
	if e != nil {
		return p, e
	}
	if e = Decode(b, &p); e != nil {
		return p, e
	}
	if p.Schema != Profile {
		return p, fmt.Errorf("unsupported plan")
	}
	if e = p.Request.validate(); e != nil {
		return p, e
	}
	dir := filepath.Dir(planPath)
	cem, e := Read(filepath.Join(dir, "cem.json"))
	if e != nil {
		return p, e
	}
	if Digest(cem) != p.CEMSha256 {
		return p, fmt.Errorf("CEM digest changed")
	}
	if e = validateCEM(ctx, repo, p.Request, cem); e != nil {
		return p, e
	}
	b, e = Read(filepath.Join(dir, "captures.json"))
	if e != nil {
		return p, e
	}
	c, e := verifyCapture(ctx, b, tasks)
	if e != nil {
		return p, e
	}
	if Digest(b) != p.CapturesSha256 || c.Capture.Producer != p.TasksVerifier || c.Verification.Verifier != p.TasksVerifier {
		return p, fmt.Errorf("capture digest changed")
	}
	tree, e := repo.CommitTree(ctx, p.Request.Target)
	if e != nil {
		return p, e
	}
	binding, hashes, a, e := authorityBinding(c, tree)
	if e != nil {
		return p, e
	}
	if binding != p.Binding || a.Phase != "RUNNING" || a.BaseCommit != p.Request.Base || p.Request.Ticket != binding.Ticket || p.Request.Attempt != binding.Attempt || len(hashes) != len(p.Request.Criteria) {
		return p, fmt.Errorf("historical binding mismatch")
	}
	for i, c := range p.Request.Criteria {
		if c.AcceptanceSha256 != hashes[i] {
			return p, fmt.Errorf("acceptance mismatch")
		}
	}
	if e = checkAnchors(ctx, repo, p.Request, c); e != nil {
		return p, e
	}
	return p, nil
}
func scenarios(p Plan) []Scenario {
	rows := []Scenario{}
	for i, c := range p.Request.Criteria {
		rows = append(rows, Scenario{Criterion: i, Role: "candidate", Commit: p.Request.Target})
		if c.Relation != "new-behavior" {
			rows = append(rows, Scenario{Criterion: i, Role: "base", Commit: p.Request.Base})
		}
		for _, commit := range c.Controls {
			rows = append(rows, Scenario{Criterion: i, Role: "control", Commit: commit})
		}
	}
	return rows
}
func Run(ctx context.Context, repoPath, planPath, approval, out string, experimental, trusted bool, tasks TasksVerifierConfig) (Receipt, error) {
	var receipt Receipt
	if !experimental || !trusted {
		return receipt, fmt.Errorf("explicit experimental and trusted-local admission required")
	}
	repo, e := gitauth.Open(repoPath, gitrun.NewDefaultBudget())
	if e != nil {
		return receipt, e
	}
	defer repo.BeginObjectSession()()
	p, e := loadPlan(ctx, repo, planPath, tasks)
	if e != nil {
		return receipt, e
	}
	digest := CanonicalDigest(p)
	if approval != digest {
		return receipt, fmt.Errorf("exact plan digest approval required")
	}
	if e = newOutput(out); e != nil {
		return receipt, e
	}
	receipt = Receipt{Profile, digest, "CALLER_REPORTED", "REVIEWER_ATTESTED", scenarios(p)}
	for i := range receipt.Scenarios {
		row := &receipt.Scenarios[i]
		c := p.Request.Criteria[row.Criterion]
		files, modes, snap, e := inventory(ctx, repo, p.Request, c, row.Commit)
		row.SnapshotSha256 = snap
		stdout, stderr := []byte{}, []byte{}
		row.ExitCode = 255
		if e == nil && ctx.Err() == nil {
			dir := filepath.Join(out, fmt.Sprintf("scenario-%03d", i))
			e = os.Mkdir(dir, 0700)
			if e == nil {
				stdout, stderr, row.ExitCode, row.Complete, e = runScenario(ctx, p.Request, c, files, modes, dir)
			}
		}
		row.Classification = Classify(stdout, row.ExitCode, row.Complete, c.Test, c.Assertion, packageName(files, c))
		if snap == "" {
			row.Classification = "unsupported"
		}
		if ctx.Err() != nil && snap == "" {
			row.Classification = "not-run"
		}
		row.StdoutSha256 = Digest(stdout)
		row.StderrSha256 = Digest(stderr)
		if e = save(out, fmt.Sprintf("stdout-%03d.jsonl", i), stdout); e != nil {
			return receipt, e
		}
		if e = save(out, fmt.Sprintf("stderr-%03d.txt", i), stderr); e != nil {
			return receipt, e
		}
		// Checkpoint all requested rows, including those not reached on interruption.
		if e = save(out, "receipt.json", Encode(receipt)); e != nil {
			return receipt, e
		}
	}
	return receipt, nil
}
func Verify(ctx context.Context, repoPath, planPath, receiptPath string, live bool, tasks TasksVerifierConfig) (Summary, error) {
	var summary Summary
	repo, e := gitauth.Open(repoPath, gitrun.NewDefaultBudget())
	if e != nil {
		return summary, e
	}
	defer repo.BeginObjectSession()()
	p, e := loadPlan(ctx, repo, planPath, tasks)
	if e != nil {
		return summary, e
	}
	var before Captures
	if live {
		if before, e = checkLive(ctx, repoPath, repo, p, tasks); e != nil {
			return summary, e
		}
	}
	b, e := Read(receiptPath)
	if e != nil {
		return summary, e
	}
	var receipt Receipt
	if e = Decode(b, &receipt); e != nil {
		return summary, e
	}
	if receipt.Schema != Profile || receipt.PlanSha256 != CanonicalDigest(p) || receipt.Assurance != "CALLER_REPORTED" || receipt.OracleAssurance != "REVIEWER_ATTESTED" {
		return summary, fmt.Errorf("receipt binding mismatch")
	}
	expected := scenarios(p)
	if len(expected) != len(receipt.Scenarios) {
		return summary, fmt.Errorf("scenario denominator mismatch")
	}
	summary = Summary{Schema: Profile, State: "VERIFIED_UNRESOLVED", PlanSha256: receipt.PlanSha256, ReceiptSha256: CanonicalDigest(receipt), CEMSha256: p.CEMSha256, AcceptanceSha256: p.Binding.AcceptanceSha256, Binding: p.Binding, Criteria: len(p.Request.Criteria), Unknowns: []string{"execution is CALLER_REPORTED; oracle relevance is REVIEWER_ATTESTED; adequacy and hostile detached descendants are not established"}}
	closed := true
	manifest := []string{}
	for _, c := range p.Request.Criteria {
		summary.RegisteredControls += len(c.Controls)
		if len(c.Controls) == 0 {
			closed = false
		}
	}
	for i, row := range receipt.Scenarios {
		want := expected[i]
		if row.Criterion != want.Criterion || row.Role != want.Role || row.Commit != want.Commit {
			return summary, fmt.Errorf("scenario order/identity mismatch")
		}
		c := p.Request.Criteria[row.Criterion]
		files, _, snap, sourceErr := inventory(ctx, repo, p.Request, c, row.Commit)
		if row.Classification != "not-run" && row.SnapshotSha256 != snap {
			return summary, fmt.Errorf("execution snapshot changed")
		}
		stdout, e := Read(filepath.Join(filepath.Dir(receiptPath), fmt.Sprintf("stdout-%03d.jsonl", i)))
		if e != nil {
			return summary, e
		}
		stderr, e := Read(filepath.Join(filepath.Dir(receiptPath), fmt.Sprintf("stderr-%03d.txt", i)))
		if e != nil {
			return summary, e
		}
		if len(stdout) > 1<<20 || len(stderr) > 1<<20 || Digest(stdout) != row.StdoutSha256 || Digest(stderr) != row.StderrSha256 {
			return summary, fmt.Errorf("raw artifact changed")
		}
		manifest = append(manifest, row.StdoutSha256, row.StderrSha256)
		classification := Classify(stdout, row.ExitCode, row.Complete, c.Test, c.Assertion, packageName(files, c))
		if sourceErr != nil {
			classification = "unsupported"
			if row.Complete {
				return summary, fmt.Errorf("unsupported source claimed complete")
			}
		}
		if row.Classification == "not-run" {
			if row.SnapshotSha256 != "" || row.Complete || row.ExitCode != 255 || len(stdout) != 0 || len(stderr) != 0 {
				return summary, fmt.Errorf("invalid not-run row")
			}
			classification = "not-run"
		}
		if classification != row.Classification {
			return summary, fmt.Errorf("classification mismatch")
		}
		if row.Role == "control" {
			if sourceErr == nil {
				summary.ApplicableControls++
			}
			if row.Complete {
				summary.ExecutedControls++
			}
			if classification == "expected-failure" {
				summary.KilledControls++
			} else {
				closed = false
			}
		} else if row.Role == "base" && c.Relation == "repair" {
			if classification != "expected-failure" {
				closed = false
			}
		} else if classification != "pass" {
			closed = false
		}
	}
	summary.ArtifactManifestSha256 = CanonicalDigest(manifest)
	if closed {
		summary.State = "VERIFIED_SATISFIED"
	}
	if live {
		after, err := checkLive(ctx, repoPath, repo, p, tasks)
		if err != nil {
			return summary, err
		}
		if before.Verification.Binding.Snapshot != after.Verification.Binding.Snapshot {
			return summary, fmt.Errorf("live native snapshot moved during verification")
		}
		if !closed {
			return summary, fmt.Errorf("registered experiment policy unresolved")
		}
		summary.State = "GATE_SATISFIED"
	}
	return summary, nil
}
