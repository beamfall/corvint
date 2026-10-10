package cli

import (
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Preflight statuses (TOL-V0-025).
const (
	preflightPassed = "PASSED"
	preflightFailed = "PREFLIGHT_FAILED"
)

// Remedies of the plan and deep findings.
var planRemedies = map[string]string{
	ticket.PlanUnassigned:        "assign the obligation to exactly one planned test",
	ticket.PlanSplit:             "assign the obligation to exactly one planned test",
	ticket.PlanUnknownObligation: "remove the id from the plan, or seed it on the ledger",
	ticket.PlanAlreadyClosed:     "remove the closed obligation from the plan",
}

const (
	planMissingRemedy = "commit the plan document at that path, or drop --plan"
	deepCheckRemedy   = "fix what the gate reports, commit, and rerun preflight --deep"

	unreadableSpecPathRemedy = "rename the spec file so its path holds no line break"
)

// preflight is the TOL-V0-025..027 read: it checks a ticket's open
// obligations against the spec files at a commit, read from Git objects, so
// no checkout of the commit is needed; --plan adds the TOL-V0-020 plan
// check of a plan document at the commit, and --deep runs named policy
// COMMAND gates in a temporary clean worktree of the commit. It writes no
// queue state, and refuses PREFLIGHT_FAILED with every finding and its
// remedy.
func preflight(env Env, cmd []string, args []string) *wire.Result {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return usage(cmd, "the first argument is the ticket id or local token")
	}
	target := args[0]
	commit, planPath := "HEAD", ""
	var gates, prefixes []string
	deep := false
	if res := pairFlags(cmd, args[1:], map[string]*string{"--commit": &commit, "--plan": &planPath},
		map[string]*[]string{"--gate": &gates, "--path": &prefixes}, map[string]*bool{"--deep": &deep}); res != nil {
		return res
	}
	if deep != (len(gates) > 0) {
		return usage(cmd, "--deep runs the named gates: give --deep with one or more --gate GATE")
	}
	for _, p := range append(append([]string{}, prefixes...), planPath) {
		if p != "" && (path.IsAbs(p) || strings.HasPrefix(path.Clean(p), "..") || strings.ContainsAny(p, "\\\n\r")) {
			return usage(cmd, "--path and --plan take repository-relative paths without line breaks")
		}
	}
	var rec *ticket.Record
	var ledger *ticket.ObligationLedger
	var defs []*intent.GateDefinition
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		r, l, ferr := obligationTarget(rc, target)
		if ferr != nil {
			return ferr
		}
		if l == nil {
			return wire.Errorf(wire.CodeMalformed, "--target", "%s ticket %s has no obligation ledger; seed it first", ticket.ObligationUnknownDetail, r.TicketID.Raw)
		}
		rec, ledger = r, l
		if deep {
			d, err := store.PreflightGateDefinitions(rc.store.Policy, gates)
			if err != nil {
				return err
			}
			defs = d
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	root, err := store.WorktreeRoot(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	resolved, _, _, err := store.FilesAtCommit(root, commit, nil)
	if err != nil {
		return errorResult(cmd, err)
	}
	if resolved == "" {
		return errorResult(cmd, wire.Errorf(wire.CodeMissingEvidence, "--commit", "the repository holds no commit %s", prose(commit)))
	}
	paths, err := store.TreePathsAtCommit(root, resolved, prefixes, func(p string) bool {
		return obligation.SpecFile(p) && underAny(p, prefixes)
	})
	if err != nil {
		return errorResult(cmd, err)
	}
	// Git's object queries are line-framed, so a path with a line break is
	// never asked: it is a finding of its own and cannot shift the answers
	// for the other paths.
	var findings []obligation.Finding
	readable := paths[:0:0]
	for _, p := range paths {
		if strings.ContainsAny(p, "\n\r") {
			findings = append(findings, obligation.Finding{Kind: obligation.FindingUnreadableSpecPath, Path: p,
				Detail: "the spec file's path holds a line break, so preflight cannot read it", Remedy: unreadableSpecPathRemedy})
			continue
		}
		readable = append(readable, p)
	}
	_, present, content, err := store.FilesAtCommit(root, resolved, readable)
	if err != nil {
		return errorResult(cmd, err)
	}
	var warnings []string
	files := map[string][]byte{}
	for _, p := range readable {
		if b, ok := content[p]; ok {
			files[p] = b
		} else if present[p] {
			warnings = append(warnings, prose("spec file "+p+" exceeds the preflight read bound and was not scanned"))
		}
	}
	findings = append(findings, obligation.CheckSources(ledger, files)...)
	if planPath != "" {
		pf, err := preflightPlan(root, resolved, planPath, rec, ledger)
		if err != nil {
			return errorResult(cmd, err)
		}
		findings = append(findings, pf...)
	}
	if deep {
		// An interrupt kills the running gate's process group and removes
		// the worktree before preflight refuses.
		ctx, stop := signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM)
		runs, err := store.PreflightGates(ctx, root, resolved, defs)
		interrupted := ctx.Err() != nil
		stop()
		if interrupted {
			return failure(cmd, rc, wire.Errorf(wire.CodeUnsupported, "--deep", "preflight was interrupted; the running gate was stopped and the temporary worktree removed"))
		}
		if err != nil {
			return errorResult(cmd, err)
		}
		for _, r := range runs {
			if r.Passed {
				continue
			}
			how := "ended " + r.Class
			if r.Class == "EXIT" {
				how = "exited " + r.ExitCode
			}
			detail := "gate " + r.GateID + " " + how
			if r.Changed != "" {
				detail += " but changed the worktree (" + r.Changed + "); later gates did not run"
			}
			if line := obligation.FirstActionableLine(string(r.Output)); line != "" {
				detail += ": " + line
			}
			findings = append(findings, obligation.Finding{Kind: obligation.FindingDeepCheckFailed, ID: r.GateID, Detail: detail, Remedy: deepCheckRemedy})
		}
	}
	obligation.SortFindings(findings)
	items := make([]wire.Value, 0, len(findings))
	for _, f := range findings {
		items = append(items, findingValue(f))
	}
	ok := len(findings) == 0
	status := preflightPassed
	if !ok {
		status = preflightFailed
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{wire.ObjectValue(wire.NewObject().Set("ok", wire.Bool(ok)).Set("status", wire.String(status)).
		Set("ticketId", wire.String(rec.TicketID.Raw)).Set("commit", wire.String(resolved)).
		Set("specFileCount", wire.String(string(wire.CountOf(int64(len(files)))))).Set("findings", wire.Array(items...)))}
	res.Untrusted = true
	res.Warnings = append(res.Warnings, warnings...)
	if !ok {
		res.Outcome = wire.OutcomeRefused
		res.Warnings = append(res.Warnings, preflightFailed+": the ticket's obligations are not ready to run at this commit; each finding names its remedy")
	}
	return res
}

// preflightPlan runs the TOL-V0-020 plan check on the plan document at
// commit (TOL-V0-026). A path that is not a file there is a PLAN_MISSING
// finding; an unreadable, oversized or mismatched plan refuses.
func preflightPlan(root, commit, planPath string, rec *ticket.Record, l *ticket.ObligationLedger) ([]obligation.Finding, error) {
	_, present, content, err := store.FilesAtCommit(root, commit, []string{planPath})
	if err != nil {
		return nil, err
	}
	if !present[planPath] {
		return []obligation.Finding{{Kind: obligation.FindingPlanMissing, Path: planPath,
			Detail: "the commit holds no file " + planPath, Remedy: planMissingRemedy}}, nil
	}
	raw, ok := content[planPath]
	if !ok {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "--plan", "the plan at the commit exceeds %d bytes", store.PreflightBlobBytes)
	}
	plan, err := ticket.DecodeObligationPlan(raw)
	if err != nil {
		return nil, err
	}
	if plan.TicketID.Raw != rec.TicketID.Raw {
		return nil, wire.Errorf(wire.CodeMalformed, "/ticketId", "the plan names %s, not %s", plan.TicketID.Raw, rec.TicketID.Raw)
	}
	found, _ := ticket.CheckObligationPlan(l, plan)
	out := make([]obligation.Finding, 0, len(found))
	for _, f := range found {
		detail := "planned tests: none"
		if len(f.Tests) > 0 {
			detail = "planned tests: " + strings.Join(f.Tests, ", ")
		}
		out = append(out, obligation.Finding{Kind: f.Kind, ID: f.ID, Path: planPath, Detail: detail, Remedy: planRemedies[f.Kind]})
	}
	return out, nil
}

// underAny reports whether p lies under one of prefixes, or prefixes is
// empty.
func underAny(p string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, pre := range prefixes {
		pre = strings.TrimSuffix(path.Clean(pre), "/")
		if pre == "." || p == pre || strings.HasPrefix(p, pre+"/") {
			return true
		}
	}
	return false
}

// findingValue renders one finding; an absent id, path or line is null.
func findingValue(f obligation.Finding) wire.Value {
	opt := func(s string) wire.Value {
		if s == "" {
			return wire.Null()
		}
		return wire.String(prose(s))
	}
	line := wire.Null()
	if f.Line > 0 {
		line = wire.String(string(wire.CountOf(int64(f.Line))))
	}
	return wire.ObjectValue(wire.NewObject().Set("kind", wire.String(f.Kind)).Set("id", opt(f.ID)).Set("path", opt(f.Path)).
		Set("line", line).Set("detail", wire.String(prose(f.Detail))).Set("remedy", wire.String(f.Remedy)))
}
