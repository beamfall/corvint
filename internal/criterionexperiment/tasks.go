package criterionexperiment

import (
	"context"
	"fmt"
	tw "github.com/Beamfall/corvint/internal/tasks/wire"
	"io"
	"os"
	"path/filepath"
)

// TasksVerifierConfig comes from the operator, never from a saved plan.
type TasksVerifierConfig struct{ Executable, SHA256 string }

func (c TasksVerifierConfig) check() error {
	_, digestErr := tw.ParseDigest("tasks executable", c.SHA256)
	if !filepath.IsAbs(c.Executable) || digestErr != nil {
		return fmt.Errorf("independently configured Tasks executable and SHA256 required")
	}
	st, e := os.Lstat(c.Executable)
	if e != nil || !st.Mode().IsRegular() || st.Size() > 256<<20 || st.Mode()&0111 == 0 {
		return fmt.Errorf("Tasks executable must be a bounded regular executable")
	}
	f, e := os.Open(c.Executable)
	if e != nil {
		return e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, (256<<20)+1))
	if e != nil || len(b) > 256<<20 || Digest(b) != c.SHA256 {
		return fmt.Errorf("Tasks executable binding changed")
	}
	return nil
}
func tasksResult(ctx context.Context, dir string, cfg TasksVerifierConfig, verb string, input []byte, args ...string) (*tw.Result, error) {
	if e := cfg.check(); e != nil {
		return nil, e
	}
	raw, e := runTasks(ctx, dir, cfg.Executable, append([]string{"criterion-binding", verb}, args...), input)
	post := cfg.check()
	if post != nil {
		return nil, post
	}
	if e != nil {
		return nil, e
	}
	r, e := tw.DecodeResult(raw)
	if e != nil {
		return nil, fmt.Errorf("unsupported Tasks criterion-binding response: %w", e)
	}
	if r.Outcome != tw.OutcomeOK || len(r.Command) != 2 || r.Command[0] != "criterion-binding" || r.Command[1] != verb || len(r.Items) != 1 || len(r.Codes) != 0 || r.Mutation != nil || r.Page != nil {
		return nil, fmt.Errorf("Tasks criterion-binding unavailable or refused")
	}
	if verb == "verify" && r.Snapshot != nil {
		return nil, fmt.Errorf("historical verification returned live snapshot")
	}
	return r, nil
}
func capture(ctx context.Context, repo, ticket, attempt string, cfg TasksVerifierConfig) (Captures, error) {
	r, e := tasksResult(ctx, repo, cfg, "capture", nil, "--ticket", ticket, "--attempt", attempt)
	if e != nil {
		return Captures{}, e
	}
	c, v, e := tw.ReadCriterionCaptureResult(r.Items[0])
	if e != nil {
		return Captures{}, e
	}
	if e = checkVerification(c, v, cfg); e != nil {
		return Captures{}, e
	}
	if r.Snapshot == nil || r.Snapshot.PendingRedo || r.Snapshot.Barrier != nil || r.Snapshot.HeadSeq == nil || *r.Snapshot.HeadSeq != v.Binding.Snapshot.HeadSeq || r.Snapshot.HeadReceiptSha256 == nil || *r.Snapshot.HeadReceiptSha256 != v.Binding.Snapshot.HeadReceiptSHA256 || r.Snapshot.IntentTreeSha256 == nil || *r.Snapshot.IntentTreeSha256 != v.Binding.Snapshot.IntentTreeSHA256 || r.Snapshot.PrimaryWorktreeSha256 == nil || *r.Snapshot.PrimaryWorktreeSha256 != v.Binding.Snapshot.PrimaryWorktreeSHA256 {
		return Captures{}, fmt.Errorf("capture outer snapshot mismatch")
	}
	if c.Producer != v.Verifier || v.Binding.TicketID.Raw != ticket || v.Binding.AttemptID != attempt {
		return Captures{}, fmt.Errorf("capture identity mismatch")
	}
	return Captures{c, v}, nil
}
func verifyCapture(ctx context.Context, raw []byte, cfg TasksVerifierConfig) (Captures, error) {
	c, e := tw.DecodeCriterionCapture(raw)
	if e != nil {
		return Captures{}, e
	}
	r, e := tasksResult(ctx, "", cfg, "verify", raw)
	if e != nil {
		return Captures{}, e
	}
	v, e := tw.ReadCriterionVerification(r.Items[0])
	if e != nil {
		return Captures{}, e
	}
	if e = checkVerification(c, v, cfg); e != nil {
		return Captures{}, e
	}
	return Captures{c, v}, nil
}
func checkVerification(c tw.CriterionCapture, v tw.CriterionVerification, cfg TasksVerifierConfig) error {
	if v.Producer != c.Producer || string(v.Verifier.ExecutableSHA256) != cfg.SHA256 || v.CaptureSHA256 != tw.Sum(tw.EncodeFile(c.Value())) {
		return fmt.Errorf("Tasks verification identity/digest mismatch")
	}
	return nil
}
