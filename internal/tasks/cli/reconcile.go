package cli

import (
	"context"
	"io"
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func reconcileInspect(env Env, args []string) *wire.Result {
	cmd := []string{"reconcile", "inspect"}
	if len(args) != 1 {
		return usage(cmd, "reconcile inspect needs one ticket ID")
	}
	target, ticketErr := wire.ParseTicketID("targetId", args[0])
	releaseID := ""
	if ticketErr != nil {
		if _, err := wire.ParseLabel("releaseId", args[0]); err != nil {
			return errorResult(cmd, ticketErr)
		}
		releaseID = args[0]
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	var queue wire.QueueID
	if releaseID == "" {
		queue, _ = wire.ParseQueueID("queueId", target.QueueID())
	} else {
		raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "head.json"), wire.MaxJournalHeadBytes)
		if err != nil {
			return errorResult(cmd, err)
		}
		head, err := snapshot.DecodeHead(raw)
		if err != nil {
			return errorResult(cmd, err)
		}
		queue = head.QueueID
	}
	reader := journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: queue, PrimaryWorktree: repo.PrimaryWorktree}
	var proof *journal.Result
	if releaseID == "" {
		proof, err = reader.Reconciliation(target.Raw, "barrier.json")
	} else {
		proof, err = reader.ReconciliationRelease(releaseID, "barrier.json")
	}
	result := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK}
	if proof != nil && proof.Head != nil {
		observed := snapshot.Snapshot{Head: proof.Head, IntentTree: proof.Identity.IntentTreeSha256}
		result.Snapshot = observed.EnvelopeSnapshot(repo.PrimaryWorktreeSha256(), proof.Pending)
	}
	if err != nil {
		result.Outcome = outcomeFor(wire.CodeOf(err))
		result.Codes = []string{wire.CodeOf(err)}
		result.Warnings = []string{prose(err.Error())}
		return result
	}
	if barrier := proof.Records["barrier.json"]; barrier.Sha256 != nil {
		decoded, err := snapshot.DecodeBarrier(barrier.Raw)
		if err != nil {
			return errorResult(cmd, err)
		}
		result.Snapshot.Barrier = &wire.BarrierRef{Scope: decoded.Scope, Reason: decoded.Reason}
	}
	path := "intent/tickets/" + target.Local + ".json"
	if releaseID != "" {
		path = "intent/releases/" + releaseID + ".json"
	}
	record := proof.Records[path]
	value, err := wire.Parse(record.Raw)
	if err != nil {
		return errorResult(cmd, err)
	}
	item := wire.NewObject()
	if releaseID == "" {
		item.Set("ticketId", wire.String(target.Raw))
	} else {
		item.Set("releaseId", wire.String(releaseID))
	}
	item.Set("canonicalRecordSha256", wire.String(string(*record.Sha256)))
	item.Set("canonicalRecord", value)
	item.Set("projectionAgreement", wire.String(proof.ProjectionAgreement))
	item.Set("historicalAcceptance", wire.String(proof.HistoricalAcceptance))
	item.Set("actorAuthentication", wire.String(proof.ActorAuthentication))
	item.Set("liveness", wire.String(proof.Liveness))
	item.Set("runtimeQualification", wire.String(proof.RuntimeQualification))
	result.Items = []wire.Value{wire.ObjectValue(item)}
	result.Untrusted = true
	return result
}

type reconcileFlags struct{ choice, request, target, file, digest, role string }

func parseReconcileFlags(args []string) (reconcileFlags, error) {
	flags := reconcileFlags{role: "OWNER"}
	values := map[string]*string{"--request-id": &flags.request, "--target": &flags.target, "--file": &flags.file, "--canonical-sha256": &flags.digest, "--role": &flags.role}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if seen[arg] {
			return flags, wire.Errorf(wire.CodeMalformed, "arguments", "duplicate flag %s", arg)
		}
		seen[arg] = true
		if arg == "--keep-journal" || arg == "--adopt-file" {
			if flags.choice != "" {
				return flags, wire.Errorf(wire.CodeMalformed, "choice", "choose exactly one reconciliation operation")
			}
			flags.choice = transaction.KeepJournal
			if arg == "--adopt-file" {
				flags.choice = transaction.AdoptFile
			}
			continue
		}
		destination, ok := values[arg]
		if !ok {
			return flags, wire.Errorf(wire.CodeMalformed, "arguments", "unknown flag %s", arg)
		}
		if i+1 >= len(args) {
			return flags, wire.Errorf(wire.CodeMalformed, "arguments", "%s needs a value", arg)
		}
		i++
		*destination = args[i]
	}
	if flags.choice == "" || flags.file == "" {
		return flags, wire.Errorf(wire.CodeMalformed, "arguments", "choose --keep-journal or --adopt-file and provide --file with preserved original bytes")
	}
	if _, err := mutation.ParseRequestID("requestId", flags.request); err != nil {
		return flags, err
	}
	if _, err := wire.ParseTicketID("targetId", flags.target); err != nil {
		if _, labelErr := wire.ParseLabel("releaseId", flags.target); labelErr != nil {
			return flags, err
		}
		if flags.choice == transaction.AdoptFile {
			return flags, wire.Errorf(wire.CodeUnsupported, "choice", "release ADOPT_FILE is NOT_RUN")
		}
	}
	if flags.role != "OWNER" && flags.role != "OPERATOR" {
		return flags, wire.Errorf(wire.CodeMalformed, "role", "reconciliation requires OWNER or OPERATOR")
	}
	if flags.choice == transaction.KeepJournal {
		if _, err := wire.ParseDigest("canonicalSha256", flags.digest); err != nil {
			return flags, err
		}
	} else if seen["--canonical-sha256"] {
		return flags, wire.Errorf(wire.CodeMalformed, "canonicalSha256", "canonical digest applies only to KEEP_JOURNAL")
	}
	return flags, nil
}

func reconcileIntent(env Env, args []string) *wire.Result {
	cmd := []string{"reconcile", "intent"}
	flags, err := parseReconcileFlags(args)
	if err != nil {
		return errorResult(cmd, err)
	}
	actor, err := initActor(flags.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	var raw []byte
	if flags.file == "-" {
		limit := wire.MaxTicketFileBytes
		if _, e := wire.ParseTicketID("targetId", flags.target); e != nil {
			limit = wire.MaxReleaseFileBytes
		}
		raw, err = io.ReadAll(io.LimitReader(env.Stdin, int64(limit)+1))
	} else {
		path := flags.file
		if !filepath.IsAbs(path) {
			path = filepath.Join(env.Cwd, path)
		}
		limit := wire.MaxTicketFileBytes
		if _, e := wire.ParseTicketID("targetId", flags.target); e != nil {
			limit = wire.MaxReleaseFileBytes
		}
		raw, err = intent.ReadFile(path, limit)
	}
	if err != nil {
		return errorResult(cmd, err)
	}
	limit := wire.MaxTicketFileBytes
	if _, e := wire.ParseTicketID("targetId", flags.target); e != nil {
		limit = wire.MaxReleaseFileBytes
	}
	if len(raw) > limit {
		return errorResult(cmd, wire.Errorf(wire.CodeLimitExceeded, "file", "original file exceeds record bound"))
	}
	if raw == nil {
		raw = []byte{}
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	request := store.ReconcileRequest{RequestID: flags.request, TargetID: flags.target, Choice: flags.choice, File: raw, CanonicalSha256: wire.Digest(flags.digest)}
	report, err := store.Reconcile(context.Background(), repo, actor, request, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	if report.Release == "" {
		return mutateResult(cmd, report)
	}
	item := wire.NewObject().Set("releaseId", wire.String(report.Release)).Set("receipt", wire.String(report.Receipt)).Set("replayed", wire.Bool(report.Outcome.Replayed)).Set("resultingRevision", nullableCount(report.Outcome.ResultingReleaseRevision))
	result := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(item)}}
	if report.Outcome.Outcome != mutation.OutcomeCompleted {
		result.Outcome = wire.OutcomeRefused
		result.Codes = report.Outcome.Codes
	}
	return result
}
