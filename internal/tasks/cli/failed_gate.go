package cli

import (
	"path/filepath"
	"regexp"
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// failedGate is the newest FAILED or BLOCKED gate result of one attempt.
type failedGate struct {
	attemptID string
	digest    wire.Digest
	result    *snapshot.GateResult
}

// lastFailedGate returns the gate result of a that ran and did not pass
// (state FAILED or BLOCKED) and ended last, ties broken by gate ID. An
// attempt keeps one result per gate, ordered by digest rather than time, so
// every record is read; one that is absent or differs from its digest leaves
// the answer unknown, so ok is false (CAL-V0-322). found is false when a has
// no such result.
func lastFailedGate(repo *intent.Repository, a *snapshot.Attempt) (g failedGate, found, ok bool) {
	for _, raw := range a.GateResults {
		d := wire.Digest(raw)
		rec, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(d)), snapshot.MaxGateRecordBytes)
		if err != nil || wire.Sum(rec) != d {
			return failedGate{}, false, false
		}
		r, err := snapshot.DecodeGateResult(rec)
		if err != nil {
			return failedGate{}, false, false
		}
		if r.State != "FAILED" && r.State != "BLOCKED" {
			continue
		}
		if !found || r.EndedAt > g.result.EndedAt || (r.EndedAt == g.result.EndedAt && r.GateID > g.result.GateID) {
			g, found = failedGate{attemptID: a.AttemptID, digest: d, result: r}, true
		}
	}
	return g, found, true
}

// ticketLastFailedGate is lastFailedGate over every attempt of ticket id:
// the result that ended last, ties broken by the later attempt ID.
func ticketLastFailedGate(repo *intent.Repository, pol *intent.Policy, attempts map[string]*snapshot.Attempt, id string) wire.Value {
	ids := []string{}
	for aid, a := range attempts {
		if a.TicketID.Raw == id {
			ids = append(ids, aid)
		}
	}
	sort.Strings(ids)
	var best *failedGate
	for _, aid := range ids {
		g, found, ok := lastFailedGate(repo, attempts[aid])
		if !ok {
			return wire.String("NOT_OBSERVED")
		}
		if found && (best == nil || g.result.EndedAt >= best.result.EndedAt) {
			best = &g
		}
	}
	if best == nil {
		return wire.Null()
	}
	return failedGateValue(repo, pol, *best)
}

// attemptLastFailedGate is `lastFailedGate` of `attempt show`.
func attemptLastFailedGate(repo *intent.Repository, pol *intent.Policy, a *snapshot.Attempt) wire.Value {
	g, found, ok := lastFailedGate(repo, a)
	switch {
	case !ok:
		return wire.String("NOT_OBSERVED")
	case !found:
		return wire.Null()
	}
	return failedGateValue(repo, pol, g)
}

// failedGateValue renders a failed gate result with the TCD-V0-165 tail of
// its retained output: the last gateTailLines non-blank lines, cleaned,
// bounded and secret-screened, and the failing line. The output is read
// from its content-addressed evidence blob and used only when it still has
// the recorded digest and size; otherwise tail and failingLine are null and
// output.observed is NOT_OBSERVED, never a guess (CAL-V0-322).
func failedGateValue(repo *intent.Repository, pol *intent.Policy, g failedGate) wire.Value {
	r := g.result
	o := wire.NewObject().
		Set("attemptId", wire.String(g.attemptID)).
		Set("gateId", wire.String(r.GateID)).
		Set("resultSha256", wire.String(string(g.digest))).
		Set("state", wire.String(r.State)).
		Set("outcomeClass", wire.String(r.OutcomeClass)).
		Set("exitCode", nullableCount(r.ExitCode)).
		Set("signal", nullableString(r.Signal)).
		Set("endedAt", wire.String(string(r.EndedAt)))
	var out *snapshot.GateEvidence
	for i := range r.Evidence {
		if r.Evidence[i].Label == "output" {
			out = &r.Evidence[i]
			break
		}
	}
	if out == nil {
		return wire.ObjectValue(o.Set("output", wire.Null()).Set("failingLine", wire.Null()).Set("tail", wire.Null()))
	}
	path := "evidence/" + string(out.Sha256)
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(out.Sha256)), wire.MaxGateOutputBytes)
	observed := err == nil && wire.Sum(raw) == out.Sha256 && uint64(len(raw)) == out.Bytes.Uint64()
	output := wire.NewObject().
		Set("sha256", wire.String(string(out.Sha256))).
		Set("bytes", wire.String(string(out.Bytes))).
		Set("path", wire.String(path))
	if !observed {
		output.Set("observed", wire.String("NOT_OBSERVED"))
		return wire.ObjectValue(o.Set("output", wire.ObjectValue(output)).Set("failingLine", wire.Null()).Set("tail", wire.Null()))
	}
	output.Set("observed", wire.String("OBSERVED"))
	var pattern *regexp.Regexp
	if pol != nil {
		if def := pol.Gate(r.GateID); def != nil {
			pattern = def.FailurePattern
		}
	}
	tail, failing := obligation.FailureTailMatching(string(raw), gateTailLines, pattern)
	return wire.ObjectValue(o.Set("output", wire.ObjectValue(output)).Set("failingLine", stringOrNull(failing)).Set("tail", wire.Strings(tail)))
}
