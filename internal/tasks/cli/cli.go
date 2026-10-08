// Package cli routes the `corvint-tasks` verbs: pure reads and archive
// export/verification under TM-V0-008, plus init and fourteen ticket
// mutations through the §5.2 journal writer. Unbuilt administrative and
// runtime verbs answer NOT_RUN. Output is always one taskman-command-result/0
// envelope (§3.3) on stdout, except `archive export`, whose stdout is the
// archive stream and whose envelope goes to stderr. Attempt-mode `run` keeps
// the envelope on stdout and sends the command's own output to stderr.
package cli

import (
	"io"
	"os"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/scopes"
	"github.com/Beamfall/corvint/internal/tasks/service"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Version is the binary's version label. It is a delivery label, not a
// claim that any gate has run.
const Version = "0.0.0-tcp01-unverified"

// Build is the first-parent commit count of the Corvint commit this binary
// was built from. cmd/corvint-tasks sets it from its -ldflags
// "-X main.build=N" stamp (decision 0397); an unstamped build reports 0.
var Build = "0"

// Env is the process environment a command runs in.
type Env struct {
	afterRead func() // deterministic snapshot-move tests only
	Cwd       string
	Args      []string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	// ScopeDeriver derives a claim scope for a ticket that declares none
	// (CAL-V0-022); nil uses the explicitly enabled pack deriver.
	ScopeDeriver store.ScopeDeriver
}

// ReadVerbs are the verb paths this binary implements. The reads are pure
// TM-V0-008 reads over the intent store and the state-dir snapshot; the
// inventory-only reads (`ticket search|export`, `roadmap`, `gate list|show`)
// report every journal-dependent fact as NOT_OBSERVED rather than omitting
// the verb (§3.3 "Read verb inputs and items"). `init` and the fifteen
// `ticket` mutations write: they commit through the §5.2 journal writer.
var ReadVerbs = []string{
	"criterion-binding capture", "criterion-binding verify",
	"help", "version", "ticket list", "ticket search", "ticket show", "ticket blockers", "ticket export",
	"queue status", "roadmap", "critical-path", "gate list", "gate show", "archive export", "archive verify", "receipt audit", "reconcile inspect", "reconcile intent",
	"init", "pause", "unpause", "policy show", "policy update", "import", "cutover",
	"ticket create", "ticket refine", "ticket prioritize", "ticket set-dependencies",
	"ticket set-gates", "ticket set-effects", "ticket hold", "ticket release-hold", "ticket reopen",
	"ticket archive", "ticket restore", "ticket complete-manual", "ticket grant-approval",
	"ticket revoke-approval", "ticket attach-evidence",
	"release create", "release update", "release candidate", "release record-gate", "release promote", "release list", "release show", "release readiness",
	"claim", "renew", "release", "reap", "widen", "attempt show", "attempt heartbeat", "plan preview",
	"lane-leader", "run", "admit", "cancel", "retry", "resume", "drain", "answer", "pending", "program show",
	"dispatch", "dispatch status", "dispatch unpark",
	"submit", "gate run", "complete", "health", "pool status", "pool sweep", "pool cleanup", "pool recover", "pool confirm-safe",
	"ticket note set", "ticket note clear", "ticket note show", "ticket note history",
	"ticket know-how add", "ticket know-how retract", "ticket know-how reconfirm", "ticket know-how list",
	"ticket obligations seed", "ticket obligations set", "ticket obligations witness", "ticket obligations show", "ticket obligations plan",
	"gate record", "gate resubmit", "gate history",
	"service install", "service status", "service uninstall", "service stop", "service resume", "service run", "service run-helper",
	"ticket escalate", "ticket answer", "ticket escalation list", "ticket escalation show", "ticket escalation history",
}

// OmittedVerbs are the verb paths the SPEC names that this binary does not
// implement; each answers NOT_RUN. The remaining administrative verbs arrive
// with the rest of TCP-02;
// `config show`, `plan record` and `receipt show|replay` remain
// unimplemented; receipt audit exposes the native journal reader.
var OmittedVerbs = []string{
	"config", "plan record", "receipt show", "receipt replay",
	"archive restore",
}

// Run executes one command and returns the process exit code: 0 iff the
// envelope outcome is OK.
func Run(env Env) int {
	if env.ScopeDeriver == nil {
		env.ScopeDeriver = scopes.Derive
	}
	if env.Stdout == nil {
		env.Stdout = io.Discard
	}
	if env.Stderr == nil {
		env.Stderr = io.Discard
	}
	if env.Stdin == nil {
		env.Stdin = strings.NewReader("")
	}
	args := env.Args
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		return emit(env.Stdout, helpResult())
	}
	if result := commandHelp(args); result != nil {
		return emit(env.Stdout, result)
	}
	switch args[0] {
	case "criterion-binding":
		return emit(env.Stdout, criterionBindingCommand(env, args[1:]))
	case "admit", "resume", "retry", "cancel", "drain", "answer":
		return emit(env.Stdout, programCommand(env, args[0], args[1:]))
	case "pending":
		return emit(env.Stdout, programRead(env, true, args[1:]))
	case "program":
		if len(args) > 1 && args[1] == "show" {
			return emit(env.Stdout, programRead(env, false, args[2:]))
		}
		return emit(env.Stdout, usage([]string{"program"}, "expected show"))
	case "run":
		if attemptMode(args[1:]) {
			return attemptRun(env, args[1:])
		}
		return emit(env.Stdout, programRun(env, args[1:]))
	case "dispatch":
		return emit(env.Stdout, dispatchCommand(env, args[1:]))
	case "service":
		return emit(env.Stdout, serviceCommand(env, args[1:]))
	case "version", "--version":
		return emit(env.Stdout, versionResult())
	case "ticket":
		if len(args) < 2 {
			return emit(env.Stdout, usage([]string{"ticket"}, "ticket needs a verb: list, search, show <id>, blockers <id>, export, or a mutation (create, refine, ...)"))
		}
		switch args[1] {
		case "list":
			return emit(env.Stdout, compactRead([]string{"ticket", "list"}, args[2:], compactSpec{summary: listSummary, values: listValueFlags}, func(a []string) *wire.Result { return ticketList(env, a) }))
		case "search":
			return emit(env.Stdout, compactRead([]string{"ticket", "search"}, args[2:], compactSpec{summary: listSummary, values: searchValueFlags}, func(a []string) *wire.Result { return ticketSearch(env, a) }))
		case "show":
			return emit(env.Stdout, compactRead([]string{"ticket", "show"}, args[2:], compactSpec{summary: ticketSummary}, func(a []string) *wire.Result { return ticketShow(env, a, true) }))
		case "blockers":
			return emit(env.Stdout, ticketShow(env, args[2:], false))
		case "export":
			return emit(env.Stdout, ticketExport(env, args[2:]))
		}
		if args[1] == "note" {
			return emit(env.Stdout, noteCommand(env, args[2:]))
		}
		if args[1] == "know-how" {
			return emit(env.Stdout, knowHowCommand(env, args[2:]))
		}
		if args[1] == "obligations" {
			return emit(env.Stdout, obligationsCommand(env, args[2:]))
		}
		switch args[1] {
		case "escalate":
			return emit(env.Stdout, escalateCommand(env, args[2:]))
		case "answer":
			return emit(env.Stdout, answerCommand(env, args[2:]))
		case "escalation":
			return emit(env.Stdout, escalationReadCommand(env, args[2:]))
		}
		if _, ok := mutationVerbs[args[1]]; ok {
			return emit(env.Stdout, mutateCommand(env, args[1], args[2:]))
		}
		if isOmitted("ticket " + args[1]) {
			return emit(env.Stdout, notRun([]string{"ticket", args[1]}))
		}
		return emit(env.Stdout, usage([]string{"ticket"}, "unknown ticket verb"))
	case "release":
		if len(args) < 2 {
			return emit(env.Stdout, usage([]string{"release"}, "release needs a verb"))
		}
		if strings.HasPrefix(args[1], "--") {
			return emit(env.Stdout, leaseCommand(env, "release", args[1:]))
		}
		return emit(env.Stdout, releaseCommand(env, args[1], args[2:]))
	case "health", "claim", "renew", "reap", "widen", "submit", "complete":
		return emit(env.Stdout, leaseCommand(env, args[0], args[1:]))
	case "pool":
		if len(args) > 1 && args[1] == "sweep" {
			return emit(env.Stdout, poolSweepCommand(env, args[2:]))
		}
		if len(args) > 1 && args[1] == "status" {
			return emit(env.Stdout, poolStatus(env, args[2:]))
		}
		if len(args) == 2 && args[1] == "--help" {
			return emit(env.Stdout, usage([]string{"pool"}, "pool confirm-safe --member MEMBER --allocation SHA256 --evidence LOCAL_REF --reason REASON"))
		}
		if len(args) > 1 && (args[1] == "confirm-safe" || args[1] == "cleanup" || args[1] == "recover") {
			return emit(env.Stdout, leaseCommand(env, "pool "+args[1], args[2:]))
		}
		return emit(env.Stdout, usage([]string{"pool"}, "unknown pool verb"))
	case "attempt":
		if len(args) > 1 && args[1] == "show" {
			return emit(env.Stdout, compactRead([]string{"attempt", "show"}, args[2:], compactSpec{summary: attemptSummary}, func(a []string) *wire.Result { return attemptCommand(env, append([]string{"show"}, a...)) }))
		}
		return emit(env.Stdout, attemptCommand(env, args[1:]))
	case "pause", "unpause":
		return emit(env.Stdout, barrierCommand(env, args[0], args[1:]))
	case "policy":
		return emit(env.Stdout, policyCommand(env, args[1:]))
	case "import":
		return emit(env.Stdout, importCommand(env, args[1:]))
	case "cutover":
		return emit(env.Stdout, cutoverCommand(env, args[1:]))
	case "init":
		return emit(env.Stdout, initCommand(env, args[1:]))
	case "queue":
		if len(args) >= 2 && args[1] == "status" {
			return emit(env.Stdout, compactRead([]string{"queue", "status"}, args[2:], compactSpec{summary: queueStatusSummary, extra: "--retries"}, func(a []string) *wire.Result { return queueStatus(env, a) }))
		}
		return emit(env.Stdout, usage([]string{"queue"}, "queue needs the verb status"))
	case "roadmap":
		return emit(env.Stdout, compactRead([]string{"roadmap"}, args[1:], compactSpec{summary: roadmapSummary, values: pageValueFlags}, func(a []string) *wire.Result { return roadmap(env, a) }))
	case "critical-path":
		return emit(env.Stdout, criticalPathCommand(env, args[1:]))
	case "plan":
		if len(args) > 1 && args[1] == "preview" {
			return emit(env.Stdout, compactRead([]string{"plan", "preview"}, args[2:], compactSpec{summary: planSummary, exclusive: "--selected-only", values: []string{"--pool", "--stage", "--exclude-member"}}, func(a []string) *wire.Result { return planPreview(env, a) }))
		}
		return emit(env.Stdout, planCommand(env, args[1:]))
	case "gate":
		if len(args) < 2 {
			return emit(env.Stdout, usage([]string{"gate"}, "gate needs a verb: list, show <gateId>, run, record, resubmit, history, state"))
		}
		switch args[1] {
		case "list":
			return emit(env.Stdout, gateList(env, args[2:]))
		case "show":
			return emit(env.Stdout, gateShow(env, args[2:]))
		case "run":
			return emit(env.Stdout, leaseCommand(env, "gate run", args[2:]))
		case "record", "resubmit":
			return emit(env.Stdout, gateReviewCommand(env, args[1], args[2:]))
		case "history":
			return emit(env.Stdout, gateHistory(env, args[2:]))
		case "state":
			return emit(env.Stdout, gateState(env, args[2:]))
		}
		return emit(env.Stdout, usage([]string{"gate"}, "unknown gate verb"))

	case "reconcile":
		if len(args) < 2 {
			return emit(env.Stdout, usage([]string{"reconcile"}, "reconcile needs inspect <ticketId> or intent with an explicit choice"))
		}
		switch args[1] {
		case "inspect":
			return emit(env.Stdout, reconcileInspect(env, args[2:]))
		case "intent":
			return emit(env.Stdout, reconcileIntent(env, args[2:]))
		}
		return emit(env.Stdout, usage([]string{"reconcile"}, "unknown reconcile verb"))
	case "receipt":
		if len(args) < 2 {
			return emit(env.Stdout, usage([]string{"receipt"}, "receipt needs a verb: audit, show <seq>, replay <seq>"))
		}
		switch args[1] {
		case "audit":
			return emit(env.Stdout, receiptAudit(env, args[2:]))
		case "show", "replay":
			return emit(env.Stdout, notRun([]string{"receipt", args[1]}))
		}
		return emit(env.Stdout, usage([]string{"receipt"}, "unknown receipt verb"))
	case "archive":
		if len(args) < 2 {
			return emit(env.Stdout, usage([]string{"archive"}, "archive needs a verb: export [--staging DIR], verify [FILE]"))
		}
		switch args[1] {
		case "export":
			// Stream on stdout, envelope on stderr (§3.3 exception).
			return emit(env.Stderr, archiveExport(env, args[2:]))
		case "verify":
			return emit(env.Stdout, archiveVerify(env, args[2:]))
		case "restore":
			return emit(env.Stdout, notRun([]string{"archive", "restore"}))
		}
		return emit(env.Stdout, usage([]string{"archive"}, "unknown archive verb"))
	}
	if isOmitted(args[0]) {
		return emit(env.Stdout, notRun([]string{args[0]}))
	}
	return emit(env.Stdout, usage([]string{"help"}, "unknown verb; run `corvint-tasks help`"))
}

func isOmitted(verb string) bool {
	for _, v := range OmittedVerbs {
		if v == verb {
			return true
		}
	}
	return false
}

// prose returns s when it is valid envelope prose, else a fixed message, so
// an error text can never make the envelope itself unencodable.
func prose(s string) string {
	if _, err := wire.ParseProse("", s, 1, wire.MaxProseBytes); err == nil {
		return s
	}
	return "message withheld: not valid prose"
}

// emit encodes the envelope and returns the exit code. An envelope that
// fails its own validation is replaced by a minimal ERROR envelope.
func emit(w io.Writer, res *wire.Result) int {
	data, err := res.Encode()
	if err != nil {
		fallback := &wire.Result{Command: res.Command, Outcome: wire.OutcomeError, Codes: []string{wire.CodeOf(err)}, Warnings: []string{prose(err.Error())}}
		data, err = fallback.Encode()
		if err != nil {
			data = []byte(`{"codes":["MALFORMED"],"command":["help"],"items":[],"mutation":null,"outcome":"ERROR","page":null,"profile":"taskman-command-result/0","retryable":false,"snapshot":null,"untrusted":[],"warnings":["envelope could not be encoded"]}` + "\n")
		}
		res = fallback
	}
	_, _ = w.Write(data)
	if res.Outcome == wire.OutcomeOK {
		return 0
	}
	return 1
}

func usage(cmd []string, msg string) *wire.Result {
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeError, Warnings: []string{prose(msg)}}
}

func notRun(cmd []string) *wire.Result {
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeNotRun, Warnings: []string{
		"verb " + strings.Join(cmd, " ") + " is not implemented yet; it arrives with a later TCP-02 slice. Nothing was written, locked or created.",
	}}
}

func helpResult() *wire.Result {
	o := wire.NewObject()
	o.Set("implemented", wire.Strings(ReadVerbs))
	o.Set("omitted", wire.Strings(OmittedVerbs))
	// The §3.1 status and §3.2 eligibility vocabularies, so a consumer
	// renders the columns this tool actually reports instead of carrying its
	// own list that silently drops an unrecognized value.
	o.Set("statuses", wire.Strings(ticket.Statuses))
	o.Set("eligibility", wire.Strings([]string{ticket.EligibilityBlocked, ticket.EligibilityUnknown}))
	o.Set("releaseReasonCodes", wire.Strings(wire.Codes))
	o.Set("supervisionLimits", wire.Strings([]string{"Optional policy profile for one policy-selected host, Codex, Claude Code or OpenCode; pinned executable and Core CLI required", "Token usage is observed, not hard-enforced; absent counters remain unknown", "Shared observed cutoffs permit one already-admitted turn per active lane of overshoot", "Explicit clean integration checkout and exact candidate/base grant required; no publication"}))
	o.Set("usage", wire.Strings([]string{
		"corvint-tasks run --program ID --config FILE --role implementer|reviewer|integrator --count N --host codex|claude-code|opencode",
		"corvint-tasks run --attempt ID --generation G --timeout SECONDS [--lease-minutes N] [--role ROLE] [--detach] -- COMMAND...   (command output on stderr; exit status is the command's; --detach keeps it running in its own session)",
		"corvint-tasks run --attach --attempt ID [--run RUN] [--wait SECONDS]   (reads a detached run; exit 75 while it runs)",
		"corvint-tasks admit|resume|retry|drain|cancel --program ID --config FILE",
		"corvint-tasks answer --program ID --config FILE --question SHA256 --revision N --answer TEXT",
		"corvint-tasks pending; corvint-tasks program show",
		"corvint-tasks ticket list [--status S[,S...]] [--offset N] [--limit N] [--summary | --fields KEY[.SUB],...]",
		"corvint-tasks ticket search [--status S] [--kind K] [--priority P] [--owner L] [--milestone L] [--label L] [--text T] [--offset N] [--limit N] [--summary | --fields KEY[.SUB],...]",
		"corvint-tasks ticket show <ticketId|local> [--summary | --fields KEY[.SUB],...]",
		"corvint-tasks ticket blockers <ticketId|local>",
		"corvint-tasks ticket export [--offset N] [--limit N]",
		"corvint-tasks queue status [--retries] [--summary | --fields KEY[.SUB],...]",
		"corvint-tasks roadmap [--offset N] [--limit N] [--summary | --fields KEY[.SUB],...]",
		"corvint-tasks gate list",
		"corvint-tasks gate show <gateId>",
		"corvint-tasks archive export [--staging DIR]   (stream on stdout, envelope on stderr)",
		"corvint-tasks archive verify [FILE|-]           (stdin when absent)",
		"corvint-tasks init [--role ROLE] [--request-id ID]",
		"corvint-tasks policy update --request-id ID --expected-policy-version N --file PATH [--role OWNER|OPERATOR]",
		"corvint-tasks ticket <mutation> --request-id ID (--payload JSON | --payload-stdin) [--target TICKET|LOCAL --expected-revision N] [--issued-at TS] [--role ROLE]",
		"corvint-tasks ticket <mutation> --help   (its payload keys)",
		"corvint-tasks <command> --help   (usage and flags; add --verbose for notes, reason codes and preconditions)",
		"corvint-tasks ticket create --template   (canonical CREATE payload with field types, enums and nullability; read-only)",
		"corvint-tasks release create|update|candidate|record-gate|promote --request-id ID --target RELEASE [--expected-revision N] [--payload JSON] [--role ROLE]",
		"corvint-tasks release list|show RELEASE|readiness RELEASE",
		"corvint-tasks claim <ticketId|local> --holder LABEL --request-id ID [--lease-minutes N] [--branch LABEL] [--base OID] [--scope PATH...] [--pool ID] [--stage implement|review|integrate]",
		"corvint-tasks health --member ID [--stage STAGE] --request-id ID",
		"corvint-tasks pool status [--pool ID] [--member ID]",
		"corvint-tasks pool sweep --request-id ID --timeout-seconds N [--member ID] [--role ROLE]",
		"corvint-tasks pool cleanup --member ID --allocation SHA256 --request-id ID",
		"corvint-tasks pool recover --member ID --allocation SHA256 --reason TEXT --request-id ID",
		"corvint-tasks pool confirm-safe --member ID --allocation SHA256 --evidence REF --reason TEXT --request-id ID",
		"corvint-tasks renew --attempt ID --generation G --request-id ID [--lease-minutes N]",
		"corvint-tasks release --attempt ID --generation G --request-id ID [--reason CODE] [--handoff-to STAGE [--handoff-reason CODE]]",
		"corvint-tasks reap --request-id ID [--attempt ID --generation G [--lease-expires-at T]]",
		"corvint-tasks widen --attempt ID --generation G --request-id ID (--scope PATH... | --whole-repository)",
		"corvint-tasks attempt show <attemptId> [--summary | --fields KEY[.SUB],...]",
		"corvint-tasks plan preview [--pool ID] [--stage implement|review|integrate] [--selected-only] [--summary | --fields KEY[.SUB],...]",
		"corvint-tasks claim --next --holder LABEL --request-id ID [--lease-minutes N] [--branch LABEL] [--base OID] [--scope PATH...] [--pool ID] [--stage implement|review|integrate]",
		"corvint-tasks cutover --execution --decision REF --qualification FILE",
		"corvint-tasks submit --attempt ID --generation G --request-id ID --tree OID",
		"corvint-tasks gate run --attempt ID --generation G --request-id ID --gate GATE [--worktree DIR]",
		"corvint-tasks gate record|resubmit <ticketId|local> --gate GATE --subject-receipt SEQ --expected-generation N --expected-revision N --request-id ID [...]",
		"corvint-tasks gate history <ticketId|local> --gate GATE [--cursor SHA256] [--limit N]",
		"corvint-tasks complete --attempt ID --generation G --request-id ID --commit OID",
		"corvint-tasks service install|status|uninstall|stop|resume --program ID [...]   (per-user launchd/systemd --user dispatcher service)",
		"corvint-tasks version",
	}))
	o.Set("note", wire.String("every read takes no lock and writes nothing, and reports journal facts it cannot observe as NOT_OBSERVED; `init`, `policy update` and the fourteen `ticket` mutations commit through the §5.2 writer (TCP-02/TCP-02b); new external-agent queue setup: docs/TASKS-EXTERNAL-AGENTS.md; ticket blockers reports static intent checks, while plan preview reports claim selection; releaseReasonCodes lists every accepted --reason value"))
	return &wire.Result{Command: []string{"help"}, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
}

// LiveFormats is the CAL-V0-131 sorted set of every format this build
// persists and decodes again: the store VERSION and each profile of the
// journal, intent, lease, run, release, review, pool, dispatcher and user
// service records, plus the command-result envelope a supervisor reads back.
// Output-only profiles are left out (TestCALV0131_LiveFormatsCoverEveryDecodedProfile
// keeps the two sets apart). Two builds whose sets are equal may replace each
// other in place (CAL-V0-130); any other pair needs a drain.
func LiveFormats() []string {
	f := []string{
		strings.TrimSpace(snapshot.VersionBytes), wire.ProfileCommandResult,
		wire.CriterionCaptureProfile, wire.CriterionVerificationProfile, wire.CriterionCaptureResultProfile,
		archive.Profile, journal.ProfileCheckpoint, journal.ProfileWriterCheckpoint, mutation.Profile, mutation.OutcomeProfile,
		intent.ProfileImportMap, intent.ProfileQueue, intent.ProfilePolicy,
		snapshot.ProfileHead, snapshot.ProfileBarrier, snapshot.ProfileReceipt, snapshot.ProfileInit,
		snapshot.ProfileAttempt, snapshot.ProfileReservations, snapshot.ProfileRetryAccounting,
		snapshot.ProfileGateResult, snapshot.ProfileManifest, snapshot.SupervisedProfile,
		snapshot.ProfileExternalReviewRequest, snapshot.ProfileExternalReviewEvent,
		snapshot.ProfilePools, snapshot.ProfileDirectPoolAdmission, snapshot.ProfileLaneUntouched,
		"taskman-pool-observation/0", "taskman-pool-sweep-observation/0", "taskman-pool-sweep-result/0",
		"taskman-programs/0", "taskman-stage/0", "taskman-operator-note-cursor/0",
		ticket.Profile, ticket.OperatorNoteProfile, ticket.EscalationRequestProfile, ticket.EscalationEventProfile,
		ticket.ObligationEventProfile, ticket.ObligationPlanProfile,
		release.Profile, release.AttestationProfile, release.MutationProfile,
		transaction.RunOutcomeProfile, runRecordProfile,
		dispatch.ConfigProfile, dispatch.StateProfile, dispatch.EventProfile, "taskman-dispatch-reader-lifecycle/0", "taskman-dispatch-detached-run/0",
		service.ProfileName, service.ManifestName, service.ControlName, service.PulseName, service.OperationName,
		service.RequestsName, service.HelperRecordName, service.ResumeOperationName,
	}
	sort.Strings(f)
	return f
}

func versionResult() *wire.Result {
	o := wire.NewObject()
	o.Set("version", wire.String(Version+"+build."+Build))
	o.Set("goVersion", wire.String(runtime.Version()))
	o.Set("slice", wire.String("TCP-01"))
	o.Set("verification", wire.String("NOT_RUN"))
	o.Set("formats", wire.Strings(LiveFormats()))
	return &wire.Result{Command: []string{"version"}, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
}

// outcomeFor maps a §11 code to the envelope outcome of a failed read.
func outcomeFor(code string) string {
	switch code {
	case wire.CodeSnapshotMoved, wire.CodeRedoPending:
		return wire.OutcomeNotRun
	case wire.CodeMalformed, wire.CodeLimitExceeded, wire.CodeUnsupportedVersion, wire.CodeGateUnknown,
		wire.CodeDuplicateID, wire.CodeCycle, wire.CodeDependencyMissing, wire.CodeInvalidPriority:
		return wire.OutcomeError
	}
	return wire.OutcomeRefused
}

// readCtx is what a read verb sees inside the snapshot.
type readCtx struct {
	repo          *intent.Repository
	snap          *snapshot.Snapshot
	store         *intent.Store
	journalAbsent bool
	// proof is the one journal audit a read command shares (CAL-V0-061). It
	// is bound to snap and dropped whenever the snapshot is re-read.
	proof *journal.Result
	// tree is the intent tree the snapshot's first probe hashed, set only
	// while body runs: the second probe hashes the tree again after body, so
	// a journal audit inside body shares both hashes instead of reading the
	// tree twice more (CAL-V0-140).
	tree *intent.Tree
}

// reuseProbedTree exists so the parity test can compare decoding the probed
// tree, and sharing it with the journal audit, with the reads they replace.
var reuseProbedTree = true

// withStore resolves the repository, runs the TM-V0-008 protocol and loads
// the intent store pinned to the probed tree digest, then runs body.
func withStore(env Env, body func(rc *readCtx) error) (*readCtx, error) {
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return nil, err
	}
	rc := &readCtx{repo: repo}
	// probed is the tree the latest probe hashed. Reader.Read runs probe,
	// body, probe in turn, so inside body it is the tree that produced
	// s.IntentTree; body decodes those bytes instead of reading the tree a
	// third time, and the second probe still reads it fresh (TM-V0-008).
	var probed *intent.Tree
	rd := snapshot.Reader{StateDir: repo.StateDir, IntentTree: func() (wire.Digest, error) {
		probed = nil
		t, err := intent.TreeDigest(repo.IntentRoot())
		if err != nil {
			return "", err
		}
		probed = &t
		return t.Sha256, nil
	}}
	snap, err := rd.Read(func(s *snapshot.Snapshot) error {
		if s.Head.PrimaryWorktree != repo.PrimaryWorktree {
			return wire.Errorf(wire.CodeUnsupportedFilesystem, repo.StateDir, "head.primaryWorktree %q differs from the resolved primary worktree %q (repository relocation is unsupported in this preview)", s.Head.PrimaryWorktree, repo.PrimaryWorktree)
		}
		var st *intent.Store
		var err error
		if probed != nil && probed.Sha256 == s.IntentTree && reuseProbedTree {
			st, err = intent.LoadTree(repo.IntentRoot(), *probed, s.IntentTree)
		} else {
			st, err = intent.LoadExpecting(repo.IntentRoot(), s.IntentTree)
		}
		if err != nil {
			return err
		}
		if st.Queue.QueueID.Raw != s.Head.QueueID.Raw {
			return wire.Errorf(wire.CodeMalformed, repo.StateDir+"/head.json/queueId", "head queue %s differs from queue.json %s", s.Head.QueueID.Raw, st.Queue.QueueID.Raw)
		}
		rc.snap = s
		rc.store = st
		rc.proof = nil
		rc.tree = nil
		if probed != nil && probed.Sha256 == s.IntentTree && reuseProbedTree {
			rc.tree = probed
		}
		err = body(rc)
		rc.tree = nil
		if env.afterRead != nil {
			env.afterRead()
		}
		return err
	})
	rc.snap = snap
	return rc, err
}

func failure(cmd []string, rc *readCtx, err error) *wire.Result {
	if rc != nil && rc.repo != nil {
		// A read refused for intent location names its repair (CTW-V0-007).
		err = rc.repo.WithIntentRepair(err)
	}
	code := wire.CodeOf(err)
	res := &wire.Result{Command: cmd, Outcome: outcomeFor(code), Codes: []string{code}, Warnings: []string{prose(err.Error())}}
	if rc != nil && rc.snap != nil && rc.snap.Head != nil {
		res.Snapshot = rc.snap.EnvelopeSnapshot(rc.repo.PrimaryWorktreeSha256(), code == wire.CodeRedoPending)
	}
	return res
}

func success(cmd []string, rc *readCtx) *wire.Result {
	if rc.journalAbsent {
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Warnings: []string{"journal-absent; unaudited current worktree intent projection; journal history and liveness NOT_OBSERVED"}}
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Snapshot: rc.snap.EnvelopeSnapshot(rc.repo.PrimaryWorktreeSha256(), false)}
}

// flags parses `--name value` / `--name=value` pairs; positional arguments
// are returned in order. Unknown flags are reported.
func flags(args []string, known ...string) (map[string]string, []string, error) {
	out := map[string]string{}
	var pos []string
	isKnown := func(n string) bool {
		for _, k := range known {
			if k == n {
				return true
			}
		}
		return false
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			pos = append(pos, a)
			continue
		}
		name, val, hasVal := strings.Cut(a[2:], "=")
		if !isKnown(name) {
			return nil, nil, wire.Errorf(wire.CodeMalformed, "argv", "unknown flag --%s", prose(name))
		}
		if !hasVal {
			if i+1 >= len(args) {
				return nil, nil, wire.Errorf(wire.CodeMalformed, "argv", "flag --%s needs a value", name)
			}
			i++
			val = args[i]
		}
		out[name] = val
	}
	return out, pos, nil
}

// pageValueFlags and searchValueFlags are the value-taking flags of the paged
// reads, so `--fields` and `--summary` extraction leaves their values alone.
var (
	pageValueFlags   = []string{"--offset", "--limit"}
	listValueFlags   = []string{"--offset", "--limit", "--status"}
	searchValueFlags = []string{"--offset", "--limit", "--status", "--kind", "--priority", "--owner", "--milestone", "--label", "--text"}
)

// page is the parsed `--offset` / `--limit` pair (§3.3): offset ≥ 0, limit
// 1..PageMax, default PageDefault. Both are checked before any read.
type page struct {
	offset wire.Count
	limit  wire.Count
}

func parsePage(fl map[string]string) (page, error) {
	p := page{offset: wire.Count("0"), limit: wire.CountOf(wire.PageDefault)}
	var err error
	if v, ok := fl["offset"]; ok {
		if p.offset, err = wire.ParseCount("--offset", v); err != nil {
			return p, err
		}
	}
	if v, ok := fl["limit"]; ok {
		if p.limit, err = wire.ParseCount("--limit", v); err != nil {
			return p, err
		}
		if p.limit.Int() < 1 || p.limit.Int() > wire.PageMax {
			return p, wire.Errorf(wire.CodeLimitExceeded, "--limit", "limit must be 1..%d", wire.PageMax)
		}
	}
	return p, nil
}

// window clips [offset, offset+limit) to a total.
func (p page) window(total int) (int, int) {
	start := p.offset.Int()
	if start > int64(total) {
		start = int64(total)
	}
	end := start + p.limit.Int()
	if end > int64(total) {
		end = int64(total)
	}
	return int(start), int(end)
}

// result renders the page object; truncated is true iff rows remain after
// the ones returned (the next offset is offset + len(items)).
func (p page) result(total, returned int) *wire.Page {
	tot := wire.CountOf(int64(total))
	return &wire.Page{Offset: p.offset, Limit: p.limit, Total: &tot, Truncated: p.offset.Int()+int64(returned) < int64(total)}
}

// pagedFlags parses a verb that takes only paging flags and no positional
// argument, plus the named extra flags.
func pagedFlags(verb string, args []string, extra ...string) (map[string]string, page, error) {
	fl, pos, err := flags(args, append([]string{"offset", "limit"}, extra...)...)
	if err != nil {
		return nil, page{}, err
	}
	if len(pos) > 0 {
		return nil, page{}, wire.Errorf(wire.CodeMalformed, "argv", "%s takes no positional argument", verb)
	}
	p, err := parsePage(fl)
	return fl, p, err
}

// listViews renders one page of compact ticket views over ids (already in
// §4.3 order) and returns the page result.
// listViews renders one page of list items (CAL-V0-173): no record member,
// no blockers or unknowns on a terminal (COMPLETED or ARCHIVED) ticket, and
// the reader-wide attempt-liveness unknown reported once as a returned
// warning instead of on every item.
//
// Every item also carries the record-derived completedAt and statusChangedAt
// (CAL-V0-182); ended, when non-nil, supplies lastAttemptEndedAt for the
// page's tickets (CAL-V0-183, `ticket list` only; a missing entry is null).
func listViews(rc *readCtx, ids []string, p page, ended map[string]wire.Value) ([]wire.Value, *wire.Page, []string) {
	ctx := rc.store.Context()
	start, end := p.window(len(ids))
	items := make([]wire.Value, 0, end-start)
	var hoisted []string
	for _, id := range ids[start:end] {
		v, _ := rc.store.Inventory.View(id, ctx)
		terminal := v.Record.Status == ticket.StatusCompleted || v.Record.Status == ticket.StatusArchived
		kept := v.Unknowns[:0:0]
		for _, u := range v.Unknowns {
			if u.Code == wire.CodeAttemptLive && u.TicketID == "" {
				if hoisted == nil {
					hoisted = []string{"store-wide unknown " + u.Code + " for every listed ticket: " + u.Detail + "; list items do not repeat it"}
				}
				continue
			}
			kept = append(kept, u)
		}
		v.Unknowns = kept
		item := v.Value(false)
		drop := []string{"record"}
		if terminal {
			drop = append(drop, "blockers", "unknowns")
		}
		item = withoutKeys(item, drop...)
		addTransitionTimes(item, v.Record)
		items = append(items, item)
	}
	if ended != nil {
		for i, id := range ids[start:end] {
			v, ok := ended[id]
			if !ok {
				v = wire.Null()
			}
			items[i].Obj.Set("lastAttemptEndedAt", v)
		}
	}
	return items, p.result(len(ids), len(items)), hoisted
}

// addTransitionTimes sets completedAt and statusChangedAt (CAL-V0-182) from
// the record alone. completedAt is the completion time, null when the record
// carries none. statusChangedAt is exact only where the record proves its
// last status transition: an unmutated DRAFT or OPEN record (createdAt), a COMPLETED
// record whose last write was the completion, or a HELD record whose last
// write placed its only hold. Every other record reports UNKNOWN rather
// than a guess; no field is added to the record itself.
func addTransitionTimes(item wire.Value, r *ticket.Record) {
	if item.Obj == nil || r == nil {
		return
	}
	completed := wire.Null()
	if r.Completion != nil {
		completed = wire.String(string(r.Completion.RecordedAt))
	}
	changed := wire.String("UNKNOWN")
	switch {
	case r.Status == ticket.StatusCompleted:
		if r.Completion != nil && r.Completion.RecordedAt == r.UpdatedAt {
			changed = wire.String(string(r.UpdatedAt)) // the last write completed it
		}
	case r.Status == ticket.StatusHeld:
		if len(r.Holds) == 1 && r.Holds[0].PlacedAt == r.UpdatedAt {
			changed = wire.String(string(r.UpdatedAt)) // the last write placed the only hold
		}
	case r.Revision == "1" && (r.Status == ticket.StatusDraft || r.Status == ticket.StatusOpen):
		changed = wire.String(string(r.CreatedAt)) // never written since its creation
	}
	item.Obj.Set("completedAt", completed)
	item.Obj.Set("statusChangedAt", changed)
}

// lastAttemptEnded returns, per listed ticket, the time its latest terminal
// attempt ended (CAL-V0-183): the recordedAt of the receipt that wrote the
// attempt's terminal phase. The receipt is the audited latest afterimage of
// the attempt record, read by name and checked against the audited digest,
// so only one bounded receipt read happens per listed ticket. A ticket with
// no terminal attempt is null; one whose receipt cannot be proven (pruned,
// rewritten, mismatched) is UNKNOWN; an absent journal is NOT_OBSERVED.
func lastAttemptEnded(rc *readCtx, ids []string) (map[string]wire.Value, error) {
	out := map[string]wire.Value{}
	if rc.journalAbsent {
		for _, id := range ids {
			out[id] = wire.String(string(ticket.NotObserved))
		}
		return out, nil
	}
	in, _, err := planInput(rc)
	if err != nil || len(in.Attempts) == 0 {
		return out, err
	}
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	latest := map[string]*snapshot.Attempt{}
	for _, a := range in.Attempts {
		id := a.TicketID.Raw
		if !want[id] || a.Live() {
			continue
		}
		if b := latest[id]; b == nil || a.PhaseSinceSeq.Uint64() > b.PhaseSinceSeq.Uint64() || (a.PhaseSinceSeq == b.PhaseSinceSeq && a.AttemptID > b.AttemptID) {
			latest[id] = a
		}
	}
	src := journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.IntentRoot()}
	for id, a := range latest {
		out[id] = wire.String("UNKNOWN")
		path := "attempts/" + a.AttemptID + ".json"
		record, ok := rc.proof.Records[path]
		if !ok || record.Sha256 == nil || record.Seq != a.PhaseSinceSeq {
			continue
		}
		name, err := snapshot.ReceiptName(record.Seq.Uint64())
		if err != nil {
			continue
		}
		raw, err := src.Read("receipts/"+name, wire.MaxReceiptFileBytes)
		if err != nil {
			continue
		}
		receipt, err := snapshot.DecodeReceipt(raw)
		if err != nil || receipt.Seq != record.Seq {
			continue
		}
		for _, post := range receipt.Post {
			if post.Path == path && post.Sha256 != nil && *post.Sha256 == *record.Sha256 {
				out[id] = wire.String(string(receipt.RecordedAt))
			}
		}
	}
	return out, nil
}

// listStatuses parses the `ticket list --status S[,S...]` filter
// (CAL-V0-181): a non-empty comma-separated set of distinct §3.2 statuses,
// given at most once. Any other form refuses MALFORMED on --status before
// the store is read.
func listStatuses(args []string, fl map[string]string) (map[string]bool, error) {
	seen := 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--status" || strings.HasPrefix(a, "--status=") {
			seen++
		}
		if a == "--status" || a == "--offset" || a == "--limit" {
			i++
		}
	}
	if seen > 1 {
		return nil, wire.Errorf(wire.CodeMalformed, "--status", "--status given more than once; pass one comma-separated list")
	}
	v, ok := fl["status"]
	if !ok {
		return nil, nil
	}
	set := map[string]bool{}
	for _, s := range strings.Split(v, ",") {
		known := false
		for _, a := range ticket.Statuses {
			known = known || s == a
		}
		if !known {
			return nil, wire.Errorf(wire.CodeMalformed, "--status", "value %q not in {%s}", prose(s), strings.Join(ticket.Statuses, "|"))
		}
		if set[s] {
			return nil, wire.Errorf(wire.CodeMalformed, "--status", "status %s listed more than once", s)
		}
		set[s] = true
	}
	return set, nil
}

func ticketList(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "list"}
	fl, p, err := pagedFlags("ticket list", args, "status")
	if err != nil {
		return failure(cmd, nil, err)
	}
	statuses, err := listStatuses(args, fl)
	if err != nil {
		return failure(cmd, nil, err)
	}
	var items []wire.Value
	var pg *wire.Page
	var hoisted []string
	rc, err := withStore(env, func(rc *readCtx) error {
		ids := rc.store.Inventory.Sorted()
		if statuses != nil {
			kept := make([]string, 0, len(ids))
			for _, id := range ids {
				if rec, ok := rc.store.Inventory.Get(id); ok && statuses[rec.Status] {
					kept = append(kept, id)
				}
			}
			ids = kept
		}
		start, end := p.window(len(ids))
		ended, err := lastAttemptEnded(rc, ids[start:end])
		if err != nil {
			return err
		}
		items, pg, hoisted = listViews(rc, ids, p, ended)
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = items
	res.Page = pg
	res.Untrusted = len(items) > 0
	res.Warnings = append(res.Warnings, hoisted...)
	return res
}

// searchFilter is the closed `ticket search` input (§3.3): every given
// filter must match (conjunction); at least one is required.
type searchFilter struct {
	status    *string
	kind      *string
	priority  *string
	owner     *string
	milestone *string
	label     *string
	text      *string // lower-cased substring of title or body
}

func enumFlag(fl map[string]string, name string, allowed []string) (*string, error) {
	v, ok := fl[name]
	if !ok {
		return nil, nil
	}
	for _, a := range allowed {
		if v == a {
			return &v, nil
		}
	}
	return nil, wire.Errorf(wire.CodeMalformed, "--"+name, "value %q not in {%s}", prose(v), strings.Join(allowed, "|"))
}

func labelFlag(fl map[string]string, name string) (*string, error) {
	v, ok := fl[name]
	if !ok {
		return nil, nil
	}
	l, err := wire.ParseLabel("--"+name, v)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func parseSearchFilter(fl map[string]string) (searchFilter, error) {
	var f searchFilter
	var err error
	if f.status, err = enumFlag(fl, "status", ticket.Statuses); err != nil {
		return f, err
	}
	if f.kind, err = enumFlag(fl, "kind", ticket.Kinds); err != nil {
		return f, err
	}
	if f.priority, err = enumFlag(fl, "priority", ticket.Priorities); err != nil {
		return f, err
	}
	if f.owner, err = labelFlag(fl, "owner"); err != nil {
		return f, err
	}
	if f.milestone, err = labelFlag(fl, "milestone"); err != nil {
		return f, err
	}
	if f.label, err = labelFlag(fl, "label"); err != nil {
		return f, err
	}
	if v, ok := fl["text"]; ok {
		t, err := wire.ParseProse("--text", v, 1, wire.MaxTitleBytes)
		if err != nil {
			return f, err
		}
		if strings.ContainsAny(t, "\t\n\r") {
			return f, wire.Errorf(wire.CodeMalformed, "--text", "search text must not contain TAB, LF or CR")
		}
		low := strings.ToLower(t)
		f.text = &low
	}
	if f.status == nil && f.kind == nil && f.priority == nil && f.owner == nil && f.milestone == nil && f.label == nil && f.text == nil {
		return f, wire.Errorf(wire.CodeMalformed, "argv", "ticket search needs at least one filter (--status, --kind, --priority, --owner, --milestone, --label, --text)")
	}
	return f, nil
}

func (f searchFilter) matches(rec *ticket.Record) bool {
	if f.status != nil && rec.Status != *f.status {
		return false
	}
	if f.kind != nil && rec.Kind != *f.kind {
		return false
	}
	if f.priority != nil && rec.Priority != *f.priority {
		return false
	}
	if f.owner != nil && (rec.Owner == nil || *rec.Owner != *f.owner) {
		return false
	}
	if f.milestone != nil && (rec.Milestone == nil || *rec.Milestone != *f.milestone) {
		return false
	}
	if f.label != nil {
		found := false
		for _, l := range rec.Labels {
			if l == *f.label {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if f.text != nil {
		if !strings.Contains(strings.ToLower(rec.Title), *f.text) && (rec.Body == nil || !strings.Contains(strings.ToLower(*rec.Body), *f.text)) {
			return false
		}
	}
	return true
}

// ticketSearch is `ticket list` restricted by the closed filter set; items,
// order, paging and the untrusted label are exactly those of `ticket list`.
func ticketSearch(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "search"}
	fl, p, err := pagedFlags("ticket search", args, "status", "kind", "priority", "owner", "milestone", "label", "text")
	if err != nil {
		return failure(cmd, nil, err)
	}
	f, err := parseSearchFilter(fl)
	if err != nil {
		return failure(cmd, nil, err)
	}
	var items []wire.Value
	var pg *wire.Page
	var hoisted []string
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		var ids []string
		for _, id := range rc.store.Inventory.Sorted() {
			rec, _ := rc.store.Inventory.Get(id)
			if f.matches(rec) {
				ids = append(ids, id)
			}
		}
		items, pg, hoisted = listViews(rc, ids, p, nil)
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = items
	res.Page = pg
	res.Untrusted = len(items) > 0
	res.Warnings = append(res.Warnings, hoisted...)
	return res
}

// ticketExport emits the full canonical records of one page of tickets in
// §4.3 order, each with its intent path and file digest, so the bytes can
// be reproduced exactly (`record` re-encodes to the file bytes). A page
// holds at most `limit` records and at most MaxListResultBytes of encoded
// items; the byte bound stops the page early with `truncated:true`, and
// the next offset is offset + len(items). No record is ever cut.
func ticketExport(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "export"}
	_, p, err := pagedFlags("ticket export", args)
	if err != nil {
		return failure(cmd, nil, err)
	}
	var items []wire.Value
	var pg *wire.Page
	rc, err := withStore(env, func(rc *readCtx) error {
		ids := rc.store.Inventory.Sorted()
		start, end := p.window(len(ids))
		items = items[:0]
		bytes := 0
		for _, id := range ids[start:end] {
			rec, _ := rc.store.Inventory.Get(id)
			path := intent.TicketsDir + "/" + rec.TicketID.Local + ".json"
			o := wire.NewObject()
			o.Set("ticketId", wire.String(rec.TicketID.Raw))
			o.Set("path", wire.String(path))
			o.Set("sha256", wire.String(string(rc.store.Digests[path])))
			o.Set("bytes", wire.String(string(wire.SizeOf(uint64(len(rec.Encode()))))))
			o.Set("record", rec.Value())
			v := wire.ObjectValue(o)
			// Same accounting as wire.Result.Encode: encoded item plus one.
			n := len(wire.Encode(v)) + 1
			if bytes+n > wire.MaxListResultBytes {
				break
			}
			bytes += n
			items = append(items, v)
		}
		pg = p.result(len(ids), len(items))
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = items
	res.Page = pg
	res.Untrusted = len(items) > 0
	return res
}

// roadmap renders one row per ticket grouped by milestone (milestones in
// byte order, tickets without one last, §4.3 order inside a group). Gate
// state is per-ticket evidence this reader cannot observe and is reported
// NOT_OBSERVED (invariant 5), never omitted or assumed.
func roadmap(env Env, args []string) *wire.Result {
	cmd := []string{"roadmap"}
	_, p, err := pagedFlags("roadmap", args)
	if err != nil {
		return failure(cmd, nil, err)
	}
	var items []wire.Value
	var pg *wire.Page
	var unmilestoned int64
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		unmilestoned = openWithoutMilestone(rc.store.Inventory)
		ids := rc.store.Inventory.Sorted()
		milestone := func(id string) (string, bool) {
			rec, _ := rc.store.Inventory.Get(id)
			if rec.Milestone == nil {
				return "", false
			}
			return *rec.Milestone, true
		}
		sort.SliceStable(ids, func(i, j int) bool {
			a, aok := milestone(ids[i])
			b, bok := milestone(ids[j])
			if aok != bok {
				return aok // a milestone sorts before none
			}
			return a < b
		})
		start, end := p.window(len(ids))
		ctx := rc.store.Context()
		items = items[:0]
		for _, id := range ids[start:end] {
			v, _ := rc.store.Inventory.View(id, ctx)
			rec := v.Record
			o := wire.NewObject()
			o.Set("milestone", wire.StringOrNull(rec.Milestone))
			o.Set("ticketId", wire.String(rec.TicketID.Raw))
			o.Set("title", wire.String(rec.Title))
			o.Set("status", wire.String(rec.Status))
			o.Set("kind", wire.String(rec.Kind))
			o.Set("priority", wire.String(rec.Priority))
			o.Set("order", wire.String(string(rec.Order)))
			o.Set("owner", wire.StringOrNull(rec.Owner))
			o.Set("requiredGates", wire.Strings(rec.RequiredGates))
			o.Set("gateResults", wire.String(string(v.GateResults)))
			o.Set("eligibility", wire.String(v.Eligibility))
			o.Set("nextAction", wire.String(v.NextAction))
			items = append(items, wire.ObjectValue(o))
		}
		pg = p.result(len(ids), len(items))
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = items
	res.Page = pg
	res.Untrusted = len(items) > 0
	if unmilestoned > 0 {
		res.Warnings = append(res.Warnings, strconv.FormatInt(unmilestoned, 10)+" OPEN ticket(s) have no milestone")
	}
	return res
}

// openWithoutMilestone counts OPEN tickets whose milestone is null
// (CAL-V0-196), over the whole inventory, independent of paging.
func openWithoutMilestone(inv *ticket.Inventory) int64 {
	n := int64(0)
	for _, id := range inv.IDs() {
		if rec, ok := inv.Get(id); ok && rec.Status == ticket.StatusOpen && rec.Milestone == nil {
			n++
		}
	}
	return n
}

// gateValue renders one policy gate definition with the §3.1 field names.
func gateValue(g intent.GateDefinition) wire.Value {
	o := wire.NewObject()
	o.Set("gateId", wire.String(g.GateID))
	o.Set("kind", wire.String(g.Kind))
	o.Set("argv", wire.Strings(g.Argv))
	o.Set("cwd", wire.String(g.Cwd))
	o.Set("env", wire.Strings(g.Env))
	o.Set("timeoutSeconds", wire.String(string(g.TimeoutSeconds)))
	ex := wire.NewObject()
	if g.ExpectedExit == nil {
		ex.Set("exitCode", wire.Null())
	} else {
		ex.Set("exitCode", wire.String(string(*g.ExpectedExit)))
	}
	ex.Set("reducer", wire.StringOrNull(g.Reducer))
	o.Set("expected", wire.ObjectValue(ex))
	o.Set("evidence", wire.Strings(g.Evidence))
	o.Set("inputs", wire.Strings(g.Inputs))
	if g.SharedResource == nil {
		o.Set("sharedResource", wire.Null())
	} else {
		sr := wire.NewObject()
		sr.Set("class", wire.String(g.SharedResource.Class))
		sr.Set("key", wire.String(g.SharedResource.Key))
		o.Set("sharedResource", wire.ObjectValue(sr))
	}
	o.Set("reusable", wire.Bool(g.Reusable))
	o.Set("required", wire.Bool(g.Required))
	return wire.ObjectValue(o)
}

// gateList lists every policy gate definition in policy order (≤ the
// policy array bound, never paginated). Definitions are validated policy
// fields (labels, identifiers, counts), not prose; `untrusted` is empty
// as for `queue status`.
func gateList(env Env, args []string) *wire.Result {
	cmd := []string{"gate", "list"}
	if len(args) != 0 {
		return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "argv", "gate list takes no argument"))
	}
	var items []wire.Value
	rc, err := withStore(env, func(rc *readCtx) error {
		items = items[:0]
		for _, g := range rc.store.Policy.Gates {
			items = append(items, gateValue(g))
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = items
	return res
}

// gateShow renders one gate's definition, the tickets whose requiredGates
// name it (sorted ticket IDs), and `results:"NOT_OBSERVED"`: gate results
// live in the journal and no TCP-01 reader can see them. An unknown gate is
// REFUSED with GATE_UNKNOWN.
func gateShow(env Env, args []string) *wire.Result {
	cmd := []string{"gate", "show"}
	if len(args) != 1 || strings.HasPrefix(args[0], "--") {
		return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "argv", "gate show takes exactly one gateId"))
	}
	gateID, err := wire.ParseLabel("argv", args[0])
	if err != nil {
		return failure(cmd, nil, err)
	}
	var item *wire.Value
	rc, err := withStore(env, func(rc *readCtx) error {
		item = nil
		for _, g := range rc.store.Policy.Gates {
			if g.GateID != gateID {
				continue
			}
			var requiredBy []string
			for _, id := range rc.store.Inventory.IDs() {
				rec, _ := rc.store.Inventory.Get(id)
				for _, rg := range rec.RequiredGates {
					if rg == gateID {
						requiredBy = append(requiredBy, id)
						break
					}
				}
			}
			sort.Strings(requiredBy)
			o := gateValue(g).Obj
			o.Set("requiredBy", wire.Strings(requiredBy))
			o.Set("results", wire.String(string(ticket.NotObserved)))
			v := wire.ObjectValue(o)
			item = &v
			return nil
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	if item == nil {
		res.Outcome = wire.OutcomeRefused
		res.Codes = []string{wire.CodeGateUnknown}
		res.Warnings = []string{"gate " + gateID + " is not defined in policy"}
		return res
	}
	res.Items = []wire.Value{*item}
	return res
}

// qualifyTicket expands a local ticket token to its full ID in the queue; a
// full ID passes through. Reads and mutations share it, so both accept the
// same forms (V1-0329).
func qualifyTicket(queueID, arg string) string {
	if strings.HasPrefix(arg, "ticket:") {
		return arg
	}
	return "ticket:" + strings.TrimPrefix(queueID, "queue:") + ":" + arg
}

func resolveTicketArg(rc *readCtx, arg string) (string, error) {
	raw := qualifyTicket(rc.store.Queue.QueueID.Raw, arg)
	id, err := wire.ParseTicketID("argv", raw)
	if err != nil {
		return "", err
	}
	if id.QueueID() != rc.store.Queue.QueueID.Raw {
		return "", wire.Errorf(wire.CodeMalformed, "argv", "ticket %s is outside queue %s", id.Raw, rc.store.Queue.QueueID.Raw)
	}
	return id.Raw, nil
}

func ticketShow(env Env, args []string, includeRecord bool) *wire.Result {
	verb := "blockers"
	if includeRecord {
		verb = "show"
	}
	cmd := []string{"ticket", verb}
	if len(args) != 1 || strings.HasPrefix(args[0], "--") {
		return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "argv", "ticket %s takes exactly one ticket id or local token", verb))
	}
	var item *wire.Value
	notFound := ""
	read := withStore
	if includeRecord {
		read = withInventoryStore
	}
	rc, err := read(env, func(rc *readCtx) error {
		item = nil
		notFound = ""
		id, err := resolveTicketArg(rc, args[0])
		if err != nil {
			return err
		}
		if _, ok := rc.store.Inventory.Get(id); !ok {
			notFound = id
			return nil
		}
		ctx, err := ticketContext(rc)
		if err != nil {
			return err
		}
		attempts := map[string]*snapshot.Attempt{}
		var in transaction.PlanInput
		if !rc.journalAbsent {
			var e error
			in, _, e = planInput(rc)
			if e != nil {
				return e
			}
			attempts = in.Attempts
			// CAL-V0-102: eligibility shows the derived loop hold.
			rec, _ := rc.store.Inventory.Get(id)
			ctx.Loop = transaction.LoopHoldOf(attempts, rec, in.Policy)
		}
		v, _ := rc.store.Inventory.View(id, ctx)
		// ERG-V0-011: a read-only complete-manual offer when every required
		// external review gate is a CURRENT PASS; never a write.
		offers := completionOffers{rc: rc, in: in}
		offers.offer(&v)
		val := v.Value(includeRecord)
		if includeRecord {
			val.Obj.Set("operatorNote", operatorNoteShowValue(rc, v.Record))
		}
		val.Obj.Set("retries", retryObservation(rc, attempts, v.Record))
		// CAL-V0-084: null without a journal, since nothing is observed.
		val.Obj.Set("nextStage", transaction.NextStage(attempts, v.Record))
		val.Obj.Set("claimabilityScope", wire.String("RECORDED_DEFAULT_EXTERNAL_AGENT_PLAN"))
		if rc.journalAbsent {
			val.Obj.Set("claimable", wire.Null()).Set("claimabilityReason", wire.String("NOT_OBSERVED"))
		} else {
			claimable, reason := transaction.RecordedClaimability(in, v.Record)
			val.Obj.Set("claimable", claimable).Set("claimabilityReason", wire.String(reason))
		}
		item = &val
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	if notFound != "" {
		res.Outcome = wire.OutcomeRefused
		res.Warnings = append(res.Warnings, "ticket "+notFound+" does not exist in this queue")
		return res
	}
	res.Items = []wire.Value{*item}
	res.Untrusted = true
	return res
}

func queueStatus(env Env, args []string) *wire.Result {
	cmd := []string{"queue", "status"}
	withRetries := len(args) == 1 && args[0] == "--retries"
	if len(args) != 0 && !withRetries {
		return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "argv", "queue status takes only --retries, --fields or --summary"))
	}
	var item wire.Value
	observedAt := time.Now().UTC()
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		st := rc.store
		by := map[string]int{}
		unknown, blocked := 0, 0
		ctx := st.Context()
		for _, id := range st.Inventory.IDs() {
			v, _ := st.Inventory.View(id, ctx)
			by[v.Record.Status]++
			if v.Eligibility == ticket.EligibilityUnknown {
				unknown++
			} else {
				blocked++
			}
		}
		bo := wire.NewObject()
		for _, s := range ticket.Statuses {
			bo.Set(s, wire.String(string(wire.CountOf(int64(by[s])))))
		}
		o := wire.NewObject()
		o.Set("queueId", wire.String(st.Queue.QueueID.Raw))
		o.Set("canonicalWriter", wire.String(st.Queue.CanonicalWriter))
		o.Set("fixture", wire.Bool(st.Queue.Fixture))
		o.Set("intentBranch", wire.String(st.Queue.IntentBranch))
		o.Set("executionCutover", wire.Bool(st.Queue.ExecutionCutover != nil))
		o.Set("writeBarrier", wire.String(st.Queue.WriteBarrier.Reason))
		o.Set("policySha256", wire.String(string(st.Policy.PolicySha256())))
		o.Set("tickets", wire.String(string(wire.CountOf(int64(st.Inventory.Len())))))
		o.Set("byStatus", wire.ObjectValue(bo))
		o.Set("intentChecksPassed", wire.String(string(wire.CountOf(int64(unknown)))))
		o.Set("blocked", wire.String(string(wire.CountOf(int64(blocked)))))
		o.Set("openWithoutMilestone", wire.String(string(wire.CountOf(openWithoutMilestone(st.Inventory)))))
		last, completions := completionSummary(rc, observedAt)
		o.Set("lastCompletion", last)
		o.Set("completions", completions)
		if rc.journalAbsent {
			for _, key := range []string{"headSeq", "generation", "attempts"} {
				o.Set(key, wire.String("NOT_OBSERVED"))
			}
			o.Set("barrier", wire.Null())
			o.Set("liveAttempts", wire.Null())
		} else {
			o.Set("headSeq", wire.String(string(rc.snap.Head.LastSeq)))
			o.Set("generation", wire.String(string(rc.snap.Head.Generation)))
			if rc.snap.Barrier == nil {
				o.Set("barrier", wire.Null())
			} else {
				b := wire.NewObject()
				b.Set("scope", wire.String(rc.snap.Barrier.Scope))
				b.Set("reason", wire.String(rc.snap.Barrier.Reason))
				o.Set("barrier", wire.ObjectValue(b))
			}
			live, err := liveAttempts(rc)
			if err != nil {
				return err
			}
			o.Set("attempts", wire.String(string(wire.CountOf(int64(len(live))))))
			values := liveAttemptsValue(live)
			for i, a := range live {
				addHolderObservation(values.Arr[i].Obj, a, observedAt, st.Policy.HeartbeatTTLSeconds())
			}
			o.Set("liveAttempts", values)
		}
		if len(st.Policy.Pools) > 0 && !rc.journalAbsent {
			occupancy, e := poolOccupancy(rc)
			if e != nil {
				return e
			}
			o.Set("pools", occupancy)
		}
		attempts := map[string]*snapshot.Attempt{}
		fallback := wire.Null()
		if !rc.journalAbsent {
			in, _, e := planInput(rc)
			if e != nil {
				return e
			}
			attempts = in.Attempts
			fallback = serialFallbackDeferredValue(in)
		}
		o.Set("serialFallbackDeferred", fallback)
		if v, ok := obligationsSummary(st.Inventory); ok {
			o.Set("obligations", v)
		}
		// The per-ticket retry map dominates the bytes on a large queue, so
		// it is opt-in (CAL-V0-169); the attempt audit above still runs.
		if withRetries {
			retries := []wire.Value{}
			for _, id := range st.Inventory.IDs() {
				rec, _ := st.Inventory.Get(id)
				if rec.Status != "OPEN" && rec.Status != "HELD" {
					continue
				}
				retries = append(retries, wire.ObjectValue(wire.NewObject().Set("ticketId", wire.String(id)).Set("ticketRevision", wire.String(string(rec.AcceptanceRevision))).Set("retries", retryObservation(rc, attempts, rec))))
			}
			o.Set("retries", wire.Array(retries...))
		}
		o.Set("journalAudit", wire.String(auditMode(rc)))
		o.Set("publication", wire.String(string(ticket.NotObserved)))
		item = wire.ObjectValue(o)
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	// A separate racy observation outside the store snapshot (CAL-V0-095).
	item.Obj.Set("preparationAdmission", preparationAdmission(rc.repo))
	res := success(cmd, rc)
	res.Items = []wire.Value{item}
	return res
}

// obligationsSummary is the TOL-V0-019 queue status sum over the OPEN and
// HELD native tickets that carry an obligation ledger, read from the loaded
// records only. ok is false when no such ticket exists, so the key is
// absent and legacy output is byte-identical.
func obligationsSummary(inv *ticket.Inventory) (wire.Value, bool) {
	var n, witnessed, total, deferred, coreWitnessed, coreTotal int64
	for _, id := range inv.IDs() {
		rec, _ := inv.Get(id)
		if rec == nil || rec.ObligationsRef == nil || rec.Source.Kind != "NATIVE" || rec.Status != ticket.StatusOpen && rec.Status != ticket.StatusHeld {
			continue
		}
		c := rec.ObligationsRef.Counts
		n++
		witnessed, total, deferred = witnessed+c.Witnessed, total+c.Total, deferred+c.Deferred
		coreWitnessed, coreTotal = coreWitnessed+c.CoreWitnessed, coreTotal+c.CoreTotal
	}
	if n == 0 {
		return wire.Value{}, false
	}
	count := func(v int64) wire.Value { return wire.String(string(wire.CountOf(v))) }
	core := wire.NewObject().Set("witnessed", count(coreWitnessed)).Set("total", count(coreTotal))
	return wire.ObjectValue(wire.NewObject().Set("tickets", count(n)).Set("witnessed", count(witnessed)).Set("total", count(total)).
		Set("deferred", count(deferred)).Set("core", wire.ObjectValue(core))), true
}

// serialFallbackDeferredValue is the CAL-V0-193 queue-status list of the
// tickets the default plan defers only by the WHOLE_REPOSITORY serial
// fallback, in plan order. It plans only when some OPEN or HELD ticket has
// unbounded effects, so a queue whose tickets all declare a scope reads no
// plan and reports an empty list.
func serialFallbackDeferredValue(in transaction.PlanInput) wire.Value {
	for _, id := range in.Tickets.IDs() {
		rec, _ := in.Tickets.Get(id)
		if (rec.Status == ticket.StatusOpen || rec.Status == ticket.StatusHeld) && transaction.UnboundedEffects(rec.Effects) {
			return wire.Strings(transaction.PriorityFirst(in).SerialFallbackDeferred())
		}
	}
	return wire.Strings([]string{})
}

// completionSummary is the CAL-V0-184 queue-status throughput view, derived
// from the current records the store already holds and no receipt scan.
// lastCompletion is the latest completion.recordedAt (ties by ticket ID),
// null when no record carries a completion. Its receipt is the sequence of
// the audited latest afterimage of that ticket when the completion was the
// record's last write; an archived record, a later write or an audit this
// read cannot complete makes it UNKNOWN and an absent journal NOT_OBSERVED.
// A non-acceptance edit in the same second as the completion is
// indistinguishable by timestamp and is a recorded limit. The selecting audit runs first so the attempt reads below it
// reuse the same proof (CAL-V0-061). completions counts current-record
// completions in (observedAt-window, observedAt]; a completion a later
// REOPEN removed from its record is not counted.
func completionSummary(rc *readCtx, observedAt time.Time) (wire.Value, wire.Value) {
	observedAt = observedAt.UTC().Truncate(time.Second)
	var last *ticket.Record
	hour, day := 0, 0
	for _, id := range rc.store.Inventory.IDs() {
		rec, ok := rc.store.Inventory.Get(id)
		if !ok || rec.Completion == nil {
			continue
		}
		c := rec.Completion.RecordedAt
		if last == nil || c > last.Completion.RecordedAt || (c == last.Completion.RecordedAt && rec.TicketID.Raw > last.TicketID.Raw) {
			last = rec
		}
		at, err := time.Parse("2006-01-02T15:04:05Z", string(c))
		if err != nil || at.After(observedAt) {
			continue
		}
		age := observedAt.Sub(at)
		if age < time.Hour {
			hour++
		}
		if age < 24*time.Hour {
			day++
		}
	}
	counts := wire.NewObject()
	counts.Set("observedAt", wire.String(observedAt.Format("2006-01-02T15:04:05Z")))
	counts.Set("lastHour", wire.String(string(wire.CountOf(int64(hour)))))
	counts.Set("last24Hours", wire.String(string(wire.CountOf(int64(day)))))
	if last == nil {
		return wire.Null(), wire.ObjectValue(counts)
	}
	receipt := wire.String(string(ticket.NotObserved))
	if !rc.journalAbsent {
		path := "intent/tickets/" + last.TicketID.Local + ".json"
		// An audit this read cannot complete leaves the receipt UNKNOWN; the
		// state reads that follow audit again and fail as they always did.
		receipt = wire.String("UNKNOWN")
		if proof, err := auditState(rc, "reservations.json", path); err == nil {
			if r, ok := proof.Records[path]; ok && r.Sha256 != nil && last.Status == ticket.StatusCompleted && last.UpdatedAt == last.Completion.RecordedAt {
				receipt = wire.String(string(r.Seq))
			}
		}
	}
	lc := wire.NewObject()
	lc.Set("ticketId", wire.String(last.TicketID.Raw))
	lc.Set("at", wire.String(string(last.Completion.RecordedAt)))
	lc.Set("receipt", receipt)
	return wire.ObjectValue(lc), wire.ObjectValue(counts)
}

func archiveExport(env Env, args []string) *wire.Result {
	cmd := []string{"archive", "export"}
	fl, pos, err := flags(args, "staging")
	if err != nil || len(pos) > 0 {
		if err == nil {
			err = wire.Errorf(wire.CodeMalformed, "argv", "archive export takes no positional argument")
		}
		return failure(cmd, nil, err)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return failure(cmd, nil, err)
	}
	rc := &readCtx{repo: repo}
	out, err := archive.Export(archive.ExportOptions{Repo: repo, Staging: fl["staging"], Stdout: env.Stdout})
	if err != nil {
		rc.snap = archive.SnapshotOnFailure
		return failure(cmd, rc, err)
	}
	rc.snap = out.Snapshot
	res := success(cmd, rc)
	o := wire.NewObject()
	o.Set("manifestSha256", wire.String(string(wire.Sum(out.Manifest.Encode()))))
	o.Set("bytes", wire.String(string(wire.SizeOf(out.Bytes))))
	o.Set("files", wire.String(string(wire.CountOf(int64(len(out.Manifest.Files))))))
	o.Set("exportedAtSeq", wire.String(string(out.Manifest.ExportedAtSeq)))
	o.Set("intentTreeSha256", wire.String(string(out.Manifest.IntentTreeSha256)))
	res.Items = []wire.Value{wire.ObjectValue(o)}
	return res
}

func archiveVerify(env Env, args []string) *wire.Result {
	cmd := []string{"archive", "verify"}
	if len(args) > 1 || (len(args) == 1 && strings.HasPrefix(args[0], "--")) {
		return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "argv", "archive verify takes at most one file argument (or - for stdin)"))
	}
	var r io.Reader = env.Stdin
	if len(args) == 1 && args[0] != "-" {
		f, err := os.Open(args[0])
		if err != nil {
			return failure(cmd, nil, wire.Errorf(wire.CodeUnsupportedFilesystem, "argv", "cannot open archive: %v", err))
		}
		defer f.Close()
		r = f
	}
	vr, err := archive.Verify(r)
	if err != nil {
		return failure(cmd, nil, err)
	}
	o := wire.NewObject()
	o.Set("queueId", wire.String(vr.Manifest.QueueID.Raw))
	o.Set("exportedAtSeq", wire.String(string(vr.Manifest.ExportedAtSeq)))
	o.Set("headGeneration", wire.String(string(vr.Manifest.HeadGeneration)))
	o.Set("lastReceiptSha256", wire.String(string(vr.Manifest.LastReceiptSha256)))
	o.Set("intentTreeSha256", wire.String(string(vr.Manifest.IntentTreeSha256)))
	o.Set("files", wire.String(string(wire.CountOf(int64(vr.Files)))))
	o.Set("bytes", wire.String(string(wire.SizeOf(vr.Bytes))))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
}

// withoutKeys copies an object item without the named keys.
func withoutKeys(v wire.Value, keys ...string) wire.Value {
	out := wire.NewObject()
	for _, k := range v.Obj.Keys {
		if !slices.Contains(keys, k) {
			out.Set(k, v.Obj.Vals[k])
		}
	}
	return wire.ObjectValue(out)
}
