package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type obligationReportKey struct{}

// WithObligationReport carries the CLI's own recomputation of a Playwright
// report into Mutate for an OBLIGATIONS_WITNESS (TOL-V0-013). The report
// itself never reaches the store (TOL-V0-009).
func WithObligationReport(ctx context.Context, check *mutation.ObligationReportCheck) context.Context {
	return context.WithValue(ctx, obligationReportKey{}, check)
}

func obligationReport(ctx context.Context) *mutation.ObligationReportCheck {
	check, _ := ctx.Value(obligationReportKey{}).(*mutation.ObligationReportCheck)
	return check
}

// ObligationEventReader reads one ledger event from the evidence store. An
// absent event is (nil, nil), so the fold reports MISSING_EVIDENCE; every
// read is bounded by the event slot and re-hashed by the fold.
func ObligationEventReader(repo *intent.Repository) func(wire.Digest) ([]byte, error) {
	return func(d wire.Digest) ([]byte, error) {
		raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(d)), ticket.MaxObligationEventBytes)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return raw, err
	}
}

// obligationInputs reads the target's ledger chain for an OBLIGATIONS_*
// mutation; every other operation reads nothing.
func obligationInputs(repo *intent.Repository, env *mutation.Envelope, records map[string]journal.Record) (map[wire.Digest][]byte, error) {
	if !ticket.IsObligationOperation(env.Operation) || env.TargetID == nil {
		return nil, nil
	}
	record, ok := records["intent/tickets/"+env.TargetID.Local+".json"]
	if !ok {
		return nil, nil
	}
	rec, err := ticket.Decode(record.Raw)
	if err != nil {
		return nil, err
	}
	return ticket.ObligationChainEvents(ObligationEventReader(repo), rec.ObligationsRef)
}

// WorktreeRoot returns the resolved top level of the checkout holding dir:
// the root a Playwright report's spec files are mapped against before the
// TOL-V0-012 source presence check.
func WorktreeRoot(dir string) (string, error) {
	out, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	top := strings.TrimSpace(string(out))
	if resolved, err := filepath.EvalSymlinks(top); err == nil {
		top = resolved
	}
	return top, nil
}

// ExactWorktreeRoot resolves a policy-declared test repository directory and
// requires it to be the top level of its own Git worktree (TOL-V0-064): Git
// discovers upward, so a declared checkout that lost its .git would otherwise
// resolve to an enclosing repository. Witness and receipt audit both use it.
func ExactWorktreeRoot(dir string) (string, error) {
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	root, err := WorktreeRoot(want)
	if err != nil {
		return "", err
	}
	if filepath.Clean(root) != filepath.Clean(want) {
		return "", fmt.Errorf("%s is not a worktree root; Git resolved it to the enclosing worktree %s", dir, root)
	}
	return root, nil
}
