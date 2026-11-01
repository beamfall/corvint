package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/dogfoodoperation"
	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// ChangeOptions are the inputs of `corvint dogfood change`; the text fields
// carry the DOGFOOD_* values the former script read from its environment.
type ChangeOptions struct {
	CaptureLegacyRecord func(LegacyRecordAttempt) error
	Root                string
	Base                string
	Steps               Runner
	Task                string
	Verify              string
	VerifyFile          string
	Outcome             string
	IntentsFile         string
	Citations           string
	// CitationReplace is DOGFOOD_CITATION_REPLACE: HUNK_ID=DIGEST entries
	// naming the cited hunks whose earlier citations the plan replaces
	// (DCW-V0-390).
	CitationReplace string
	OCMLinks        string
	// AggregateOverLimit lets a change whose admitted paths exceed the 200-path
	// learning bound record the complete nonlearning aggregate receipt instead of
	// refusing admitted-path-limit (DCW-V0-374). Local completion leaves it off:
	// its enrolled aggregate consumes that refusal (ALO-V0).
	AggregateOverLimit bool
}

type step struct{ name, status, reason string }

type change struct {
	flow
	options            ChangeOptions
	evidence           string
	runTmp             string
	rows               []step
	citationStage      string
	cemPrepared        bool
	citationStageOwned bool
	citationCount      int
	citationFailedRow  int
	// citationFailedValues names the refused row's RELATION and START:END
	// fields, quoted, so an invalid-arguments hint names the rejected value
	// (V1-1147).
	citationFailedValues string
	citeSpanUnaligned    bool // no hunk edits the refused span (DCW-V0-063)
	// mergedSeals are the sealed CEM paths BASE..HEAD adds that re-verified
	// (DCW-V0-042); an aggregate bound refusal names them (DCW-V0-113).
	mergedSeals []string
	// citationRefusals is every refused cite row of the plan, found in one
	// pass after the first refusal (DCW-V0-114).
	citationRefusals     []string
	linkObservedRows     int
	citedOver            bool
	guardHunks           int // hunks the guarding-test search would select (DCW-V0-069)
	intentPublishTmp     string
	mapKeepTmp           string
	localOutcomeSHA      string
	contextAbstentionSHA string
	queryAbstentionSHA   string
	bootstrapUnknown     int
	reasonedUnknown      int
	unknownNotes         []byte
	manifestValid        bool
	manifestReason       string
	manifestIssue        string
	noIntent             bool
	intentSnapshot       []byte
	intentSource         []byte
	intents              []string
	linkPlan             []byte
	linksReady           bool
	linkPlanSHA          string
	linkPlanRows         int
	uniformHunks         int
	uniformSpan          string
	// recitedHunk names the cited hunk a plan row would change (DCW-V0-117),
	// and recitedDigest its current expected-old digest (DCW-V0-390).
	recitedHunk   string
	recitedDigest string
	// replaceDigests and replaceSelectors are the admitted
	// DOGFOOD_CITATION_REPLACE entries, by hunk ID and by plan selector;
	// replaceApplied and replaceReplayed name the hunks this run replaced or
	// found already replaced (DCW-V0-390, DCW-V0-392).
	replaceDigests   map[string]string
	replaceSelectors map[string]string
	replaceApplied   []string
	replaceReplayed  []string
	// staleBinding names the row whose hunk moved since the citation-plan
	// binding was recorded against an earlier map (DCW-V0-066).
	staleBinding string
	// remappedPlan is the plan with its ordinal rows moved through the
	// recorded binding when only inserted admitted sealed-CEM hunks shifted
	// them (DCW-V0-340); the plan file itself is never rewritten.
	remappedPlan []byte
	// sealCiteRows cite each unknown admitted sealed-CEM hunk the plan does
	// not name against the DCW-V0-042 admission span at BASE (DCW-V0-341).
	sealCiteRows []string
	// mismatchDetail names the plan row or owed hunk that made
	// citationPlanMismatch refuse, for the inputs precheck (DCW-V0-379).
	mismatchDetail string
	// relationDetail names the first plan row whose RELATION is not a
	// registered basis relation (DCW-V0-402).
	relationDetail string
	// deferrals are the plan's deferred rows, which run cem defer instead of
	// citing a hunk (DCW-V0-370).
	deferrals []deferral
	// citationFailedMap is the map the failing cite command read: the staged
	// citation map after the first row (DCW-V0-068).
	citationFailedMap string
	// nested marks a finish-lane pass, whose run already checked the
	// worktree for an earlier lane's state (DCW-V0-400).
	nested bool
	// scope narrows a finish-lane pass to the steps its decision reads
	// (DCW-V0-115); a standalone change run is always passFull.
	scope passScope
}

// Change runs the pre-change context, binding and status steps and writes
// .corvint/dogfood-report.json, exactly as script/dogfood-change.sh did.
func Change(ctx context.Context, options ChangeOptions, stderr io.Writer) (int, error) {
	run := &change{flow: flow{ctx: ctx, prefix: "dogfood-change", root: options.Root, stderr: stderr}, options: options}
	defer run.releaseOperation()
	return boundary(ctx, run.run, run.cleanup)
}

func (c *change) cleanup() {
	if c.intentPublishTmp != "" {
		_ = removeFile(c.intentPublishTmp)
	}
	if c.mapKeepTmp != "" {
		_ = removeFile(c.mapKeepTmp)
	}
	_ = c.cleanupCitationStage()
	if c.runTmp != "" {
		_ = os.RemoveAll(c.runTmp)
	}
}

func (c *change) path(name string) string {
	return c.root + "/" + name
}

func (c *change) run() int {
	c.requireRoot()
	c.requireTrustedAncestry()
	c.gitDir = c.gitValue("rev-parse", "--absolute-git-dir")
	c.acquireOperation()
	c.target = c.gitValue("rev-parse", "HEAD^{commit}")
	c.requireNoAggregateMarker()
	c.base = c.gitValue("rev-parse", c.options.Base+"^{commit}")
	// finish-lane checks once before its first pass; its passes skip it.
	if !c.nested {
		c.requireFreshWorktree(c.base, c.gitDir, true)
	}
	task := c.options.Task
	if task == "" {
		task = "Dogfood change from " + c.base[:12] + " to HEAD"
	}
	// The sealed-CEM admissions refuse before the previous run's evidence is
	// cleared, so a refusal keeps it (DCW-V0-042, DCW-V0-043).
	inherited := c.inheritedCEM()
	c.admitSealedCEMs(inherited)
	c.requireArchivedBaseCEM(inherited)
	c.precheckIntentObligations()
	c.evidence = c.gitDir + "/corvint"
	_ = os.MkdirAll(c.evidence, 0o777)
	_ = os.MkdirAll(c.path(".corvint"), 0o777)
	c.clearStepEnvelopes()
	runTmp, err := os.MkdirTemp(c.evidence, "dogfood-change.")
	if err != nil {
		c.say("%s: %v\n", c.prefix, err)
		c.exit(2)
	}
	c.runTmp = runTmp
	// One private store per run lets every OCM prepare, status and the
	// aggregate reuse the first accepted stable verdict of this exact CEM; it
	// is removed with runTmp (OCM-V0-027).
	if reuseDir := filepath.Join(runTmp, "stable-reuse"); os.Mkdir(reuseDir, 0o700) == nil {
		c.ctx = withStableReuse(c.ctx, reuseDir)
	}
	c.citationStage = ".corvint/.cem-citations." + filepath.Base(runTmp) + ".json"
	c.resolveAnchor()
	c.noteMergedForward()
	c.noteUntrackedBuildOutputs()
	// Query and impact run after the change, so they are coordination-time
	// receipts; the agent's prechange-*.json receipts are never written here (DCW-V0-026).
	if c.scope == passFull {
		c.coordinationQuery(task)
		c.prechangeImpact()
		if c.noDiffYet() {
			return c.reportNoDiffYet()
		}
	}
	c.cemPrepared = c.prepare("cem-prepare", c.evidence+"/cem-prepare.json", func(status int, stderr []byte, reason string) bool {
		return status == 2 && outdatedCEMMaps[chomp(string(stderr))]
	}, "cem", "prepare", "--base", c.base, "--target", c.target) == 0
	c.markStableChoreMap()
	if c.scope == passPrepare {
		if c.cemPrepared {
			if code := c.precheckInputs(); code != 0 {
				return code
			}
			return c.precheckCitationPlan()
		}
		c.renderReport()
		return c.reportFailures()
	}
	// The manifest is frozen before citation so the plan check knows which
	// base-absent intent hunks an author may deliberately leave uncited.
	c.manifestReason = c.validateIntentManifest()
	c.manifestValid = c.manifestReason == ""
	c.deferIntents()
	c.citeStep()
	c.guardingTestsStep()
	_ = os.WriteFile(c.path(".corvint/change.ocm-status.json"), nil, 0o666)
	switch {
	case c.noIntent:
		c.declareNoIntent()
	case !c.manifestValid:
		c.addStep("ocm-aggregate", "NOT_PRODUCED", c.manifestReason)
	case !c.runOCMScopes():
		c.addStep("ocm-aggregate", "NOT_PRODUCED", "intent-scope-drift")
	case c.finishOCMAggregate():
		c.countBootstrapUnknowns()
	}
	// An unknown hunk the plan kept with a recorded reason is admitted beside
	// the bootstrap unknowns; a prepared no-evidence hunk never is (DCW-V0-019).
	_, c.reasonedUnknown = c.flow.reasonedUnknowns(c.path(".corvint/change.cem.json"), c.intents, c.bootstrapUnknown > 0)
	c.runStatusStep(c.evidence+"/cem-status.json",
		statusArguments(c.root, c.base, c.target, c.bootstrapUnknown+c.reasonedUnknown, len(c.deferrals))...)
	// The recorder refuses a dirty tree, so it runs last and sees the sidecar
	// this pass prepared and cited: an untracked or modified sidecar reports
	// record-index-failed and the pass is never complete (DCW-V0-015).
	if c.scope != passBind {
		c.rows = append(c.rows, c.localOutcome(task))
	}
	c.renderReport()
	return c.reportFailures()
}

// noDiffYetReason marks the steps a change-start run cannot produce because
// HEAD has no committed difference from BASE (DCW-V0-383).
const noDiffYetReason = "no-diff-yet"

// noDiffYetSteps are the steps a no-diff-yet run reports NOT_PRODUCED.
var noDiffYetSteps = []string{"cem-prepare", "cem-cite", "ocm-aggregate", "cem-status", "local-outcome"}

// noDiffYet reports a change-start run (DCW-V0-383, V1-3121): no citation plan
// or outcome was supplied, and HEAD's tree is BASE's tree, so there is no
// change to prepare, cite or bind yet. A run with either input keeps every step.
func (c *change) noDiffYet() bool {
	if c.options.Citations != "" || c.options.Outcome != "" {
		return false
	}
	return c.gitValue("rev-parse", c.base+"^{tree}") == c.gitValue("rev-parse", c.target+"^{tree}")
}

// reportNoDiffYet writes the report with the pre-change rows and every later
// step NOT_PRODUCED no-diff-yet, so the report is never complete and
// dogfood-check still refuses it, and exits 0 naming the inputs still owed. A
// failing pre-change row still fails the run.
func (c *change) reportNoDiffYet() int {
	failed := false
	for _, row := range c.rows {
		failed = failed || c.rowFailing(row)
	}
	for _, name := range noDiffYetSteps {
		c.addStep(name, "NOT_PRODUCED", noDiffYetReason)
	}
	c.renderReport()
	if failed {
		return c.reportFailures()
	}
	c.say("dogfood-change: PRECHANGE no-diff-yet base=%s target=%s report=%s\n", c.base, c.target, c.path(".corvint/dogfood-report.json"))
	c.say("  pre-change context recorded; %s NOT_PRODUCED %s until a commit differs from BASE\n", strings.Join(noDiffYetSteps, ", "), noDiffYetReason)
	c.say("  inputs still to supply after the last commit: CITATIONS (corvint dogfood propose-citations %s > plan.tsv), INTENTS, VERIFY, OUTCOME; check them with make dogfood-inputs-check BASE=%s\n", c.base, c.base)
	return 0
}

// admitSealedCEMs admits the sealed CEMs BASE..HEAD adds only as unmodified,
// verifiable sealed changes the branch merged or stacked on (DCW-V0-042):
// each is re-verified against its own base and bind commit exactly as
// batch-check verifies a constituent. Any other added, modified or deleted
// file under .corvint/changes/ refuses sealed-cem-in-change before a step runs.
// The archive of BASE's own inherited CEM is admitted without re-verification:
// it keeps bytes BASE already tracks (DCW-V0-043). A sealed CEM a migration
// commit rewrote is admitted as the seal baseline, and HEAD must keep the
// migration's bytes (DCW-V0-050).
func (c *change) admitSealedCEMs(inherited inheritedCEM) {
	changed := nonEmpty(strings.Split(c.gitValue("diff", "--no-renames", "--name-status", c.base, c.target, "--", ".corvint/changes"), "\n"))
	// A migration commit's rewrites are the seal baseline, not constituents,
	// and do not count toward the limit; HEAD must keep them even where the
	// net diff shows no change (DCW-V0-050).
	migrated, reverted := c.migratedSeals(c.base, c.target)
	if len(changed) == 0 && len(reverted) == 0 {
		return
	}
	kept := changed[:0]
	for _, line := range changed {
		if state, path, _ := strings.Cut(line, "\t"); state != "M" || !migrated[path] {
			kept = append(kept, line)
		}
	}
	admitted := len(changed) - len(kept)
	changed = kept
	refusals := []string{}
	for _, path := range reverted {
		refusals = append(refusals, path+": sealed-cem-migration-reverted")
	}
	if len(changed) > batchConstituentLimit {
		refusals = append(refusals, fmt.Sprintf("sealed CEMs=%d exceed %d", len(changed), batchConstituentLimit))
		changed = nil
	}
	notes := []string{}
	if admitted > 0 {
		notes = append(notes, fmt.Sprintf("sealed-cem-migrated maps=%d commit=%s", admitted, c.migrationNote(c.base, c.target)))
	}
	for _, line := range changed {
		state, path, _ := strings.Cut(line, "\t")
		match := sealedCEMPath.FindStringSubmatch(path)
		switch {
		case match == nil:
			refusals = append(refusals, path+": unrecognized-sealed-cem")
		case state != "A":
			refusals = append(refusals, path+": sealed-cem-modified status="+state)
		case path == inherited.path && c.archives(inherited, c.target):
			notes = append(notes, "inherited-cem-archived NOT_REVERIFIED path="+path)
		default:
			result := c.verifySealedCEM(c.options.Steps, c.target, match[1])
			if result.failure == retiredSeal {
				// The change binds the whole range, so its own cem/1.0 map
				// covers the retired seal's hunks and commits (CEM-V1-203).
				notes = append(notes, "sealed-cem-retired "+result.detail+" NOT_REVERIFIED bound-by-this-change")
				continue
			}
			if result.failure != "" {
				refusals = append(refusals, path+": "+result.failure+" "+result.detail)
				continue
			}
			c.mergedSeals = append(c.mergedSeals, path)
			notes = append(notes, fmt.Sprintf("sealed-cem-merged bind=%s base=%s bootstrapUnknown=%d", match[1], result.base, len(result.bootstrap)))
		}
	}
	if len(refusals) > 0 {
		c.say("dogfood-change: REFUSE sealed-cem-in-change\n")
		for _, refusal := range refusals {
			c.say("  %s\n", refusal)
		}
		c.say("  BASE..HEAD adds a sealed CEM that is not an unmodified, verifiable sealed change; restore it from its bind commit, or revert or drop the seal commit, then rebind\n")
		c.say("  fix: for a merge batch of sealed changes, run corvint dogfood batch-check %s instead of rebinding it (DCW-V0-039)\n", c.base)
		c.exit(2)
	}
	for _, note := range notes {
		c.say("dogfood-change: NOTE %s\n", note)
	}
}

// requireArchivedBaseCEM refuses, before any step runs, a change in a
// repository that seals while BASE still tracks an unsealed CEM that HEAD
// keeps under no .corvint/changes/ file: the change replaces that CEM, and
// dogfood seal would refuse unarchived-base-cem only after the whole loop
// (DCW-V0-027, DCW-V0-043). A repository that never sealed is unaffected.
func (c *change) requireArchivedBaseCEM(inherited inheritedCEM) {
	if inherited.blob == "" || c.gitValue("ls-tree", c.base, "--", ".corvint/changes") == "" {
		return
	}
	archived := c.gitValue("ls-tree", "-r", c.target, "--", ".corvint/changes")
	if strings.Contains(archived, " "+inherited.blob+"\t") {
		return
	}
	c.say("dogfood-change: REFUSE unarchived-base-cem\n")
	c.say("  BASE tracks an unsealed .corvint/change.cem.json (last written by %s) that no .corvint/changes/ file at HEAD keeps; this change replaces it, so dogfood-seal would refuse\n", inherited.writer)
	if inherited.path != "" {
		c.say("  fix: %s\n", inherited.archiveRecovery())
	} else {
		c.say("  fix: archive BASE's .corvint/change.cem.json (blob %s) under .corvint/changes/<the commit that bound it>.cem.json in its own commit\n", inherited.blob)
	}
	c.say("  then rerun dogfood-change with the same BASE; the plan cites the archived file's hunk (DCW-V0-043)\n")
	c.exit(2)
}

// clearStepEnvelopes removes the previous run's "<step>.stderr" files, so a step
// that does not run this time cannot leave evidence for a reason it never emitted.
func (c *change) clearStepEnvelopes() {
	entries, _ := os.ReadDir(c.evidence)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".stderr") && !strings.HasPrefix(entry.Name(), ".") {
			_ = removeFile(c.evidence + "/" + entry.Name())
		}
	}
}

func (c *change) addStep(name, status, reason string) {
	c.rows = append(c.rows, step{name, status, reason})
}

// exec runs one Corvint command with stdout and stderr redirected to files and
// returns its exit status; a redirection that cannot open fails like Bash, with 1.
func (c *change) exec(args []string, output, errorFile string) int {
	stdout, err := os.Create(output)
	if err != nil {
		return 1
	}
	defer stdout.Close()
	stderr, err := os.Create(errorFile)
	if err != nil {
		return 1
	}
	defer stderr.Close()
	status := c.options.Steps.Run(c.ctx, c.root, args, stdout, stderr)
	c.checkpoint()
	return status
}

// runStep reports one command as a PRODUCED row, or NOT_PRODUCED with the code
// its stderr names.
func (c *change) runStep(name, output string, args ...string) int {
	errorFile := c.evidence + "/" + name + ".stderr"
	status := c.exec(args, output, errorFile)
	if status == 0 {
		c.addStep(name, "PRODUCED", "none")
		return 0
	}
	c.addStep(name, "NOT_PRODUCED", failureReason(readFile(errorFile), status))
	return status
}

// runStatusStep is runStep for `cem status`: a stable verifier that refuses
// the protocol's 1,024-operation budget with unsupported-resource-limit is
// rerun once with the opt-in scaled bound, and that result stands (DCW-V0-373).
func (c *change) runStatusStep(output string, args ...string) int {
	errorFile := c.evidence + "/cem-status.stderr"
	status := c.exec(args, output, errorFile)
	if status != 0 && resourceLimited(readFile(output)) {
		status = c.exec(withScaledOperations(args), output, errorFile)
	}
	if status == 0 {
		c.addStep("cem-status", "PRODUCED", "none")
		return 0
	}
	c.addStep("cem-status", "NOT_PRODUCED", failureReason(readFile(errorFile), status))
	return status
}

// outdatedCEMMaps are the exact outdated-map refusals: the coded form a current
// binary prints (CCF-V1-004, proposed under decision 0398) and the codeless form
// an N-1 --corvint-bin still prints.
//
// A cem/1.0 prepare also names a 0.x map at the frozen path as outdated by its
// spec, so the first stable pass over a branch bound with cem/0.x regenerates
// it (CEM-V1-147); an N-1 binary never prints that form.
var outdatedCEMMaps = map[string]bool{
	`{"code": "map-unavailable", "error": "cannot read CEM map: the existing map records a different base or patch; pass --replace to regenerate", "ok": false}`: true,
	`{"error": "cannot read CEM map: the existing map records a different base or patch; pass --replace to regenerate", "ok": false}`:                            true,
	`{"code": "map-unavailable", "error": "cannot read CEM map: the existing map records a different spec; pass --replace to regenerate", "ok": false}`:          true,
}

// prepare derives a CEM or OCM map and regenerates it with --replace only after
// the exact outdated-map refusal: every rebind after a sidecar commit finds an
// outdated map at the frozen path, and any other failure must never rewrite a
// committed cited map.
func (c *change) prepare(name, output string, outdated func(int, []byte, string) bool, args ...string) int {
	errorFile := c.evidence + "/" + name + ".stderr"
	status := c.exec(args, output, errorFile)
	if status == 0 {
		c.addStep(name, "PRODUCED", "none")
		return 0
	}
	stderr := readFile(errorFile)
	reason := failureReason(stderr, status)
	if !outdated(status, stderr, reason) {
		c.addStep(name, "NOT_PRODUCED", reason)
		return status
	}
	return c.runStep(name, output, append(args, "--replace")...)
}

// prechangeImpact runs impact and keeps a typed, digest-bound abstention when
// it refuses with exactly one envelope whose code is an impact abstention.
func (c *change) prechangeImpact() {
	output := c.evidence + "/coordination-time-impact.json"
	errorFile := c.evidence + "/coordination-time-impact.stderr"
	argvFile := c.evidence + "/coordination-time-impact.argv"
	artifact := c.evidence + "/coordination-time-impact-abstention.json"
	argv := []string{c.options.Steps.Path, "--root", c.root, "impact", "--base", c.base, "--range-profile", "expanded-256", "--limit", "20"}
	failed := func() { c.addStep("coordination-time-impact", "NOT_PRODUCED", "context-abstention-evidence-failed") }
	if removeFile(artifact) != nil || writePrivate(argvFile, []byte(strings.Join(argv, "\x00")+"\x00")) != nil {
		failed()
		return
	}
	status := c.exec(argv[3:], output, errorFile)
	// DCW-V0-075: a Git deadline overrun under host load gets one bounded
	// retry; a second overrun is a visible, non-blocking abstention.
	if impactDeadlineOverrun(status, output, errorFile) {
		c.say("dogfood-change: NOTE coordination-time-impact deadline overrun; retrying once\n")
		status = c.exec(argv[3:], output, errorFile)
		if impactDeadlineOverrun(status, output, errorFile) {
			c.addStep("coordination-time-impact", "NOT_PRODUCED", impactDeadlineReason)
			return
		}
	}
	if status == 0 {
		c.addStep("coordination-time-impact", "PRODUCED", "none")
		return
	}
	stderr := readFile(errorFile)
	reason := failureReason(stderr, status)
	if reason == impactDeadlineReason {
		// Only the two-overrun path above may record the non-blocking reason.
		reason = "exit-" + strconv.Itoa(status)
	}
	code, _ := firstCapture(impactRefusal, stderr)
	if status != 2 || !impactAbstentions[reason] || code != reason || hasContent(output) || !impactAbstentionStderr(stderr) {
		if impactAbstentions[reason] {
			reason = "context-abstention-invalid"
		}
		c.addStep("coordination-time-impact", "NOT_PRODUCED", reason)
		return
	}
	stdoutSHA, stdoutErr := fileSHA256(output)
	stderrSHA, stderrErr := fileSHA256(errorFile)
	argvSHA, argvErr := fileSHA256(argvFile)
	if stdoutErr != nil || stderrErr != nil || argvErr != nil {
		failed()
		return
	}
	record := []byte(abstentionArtifact(argvSHA, c.base, reason, stderrSHA, stdoutSHA, c.target) + "\n")
	if writePrivate(artifact, record) != nil {
		failed()
		return
	}
	c.contextAbstentionSHA = sha256Hex(record)
	c.addStep("coordination-time-impact", "NOT_PRODUCED", reason)
}

func abstentionArtifact(argvSHA, base, reason, stderrSHA, stdoutSHA, target string) string {
	return `{"argvSha256":"sha256:` + argvSHA + `","base":"` + base + `","exitStatus":"2","profile":"corvint-dogfood-context-abstention/0","reason":"` + reason + `","status":"NOT_PRODUCED","stderrSha256":"sha256:` + stderrSHA + `","stdoutSha256":"sha256:` + stdoutSHA + `","step":"coordination-time-impact","target":"` + target + `"}`
}

// localOutcome records the author's verification outcome, or names why no
// outcome input was provided, and returns the local-outcome row.
func (c *change) localOutcome(task string) step {
	verify, reason := verifyInput(c.options)
	if reason != "" {
		return step{"local-outcome", "NOT_PRODUCED", reason}
	}
	if c.options.Outcome == "" || len(verify) == 0 {
		return step{"local-outcome", "NOT_PRODUCED", "outcome-input-not-provided"}
	}
	output := c.evidence + "/local-outcome.json"
	errorFile := c.evidence + "/local-outcome.stderr"
	if err := validateVerify(verify); err != nil {
		if writePrivate(errorFile, []byte(err.Error()+"\n")) != nil {
			return step{"local-outcome", "NOT_PRODUCED", "aggregate-publication-failed"}
		}
		fmt.Fprintln(c.stderr, err)
		for _, suggestion := range quotedRunSuggestions(c.options) {
			fmt.Fprintf(c.stderr, "quote the -run alternation, a shell would split it at |: %s\n", suggestion)
		}
		return step{"local-outcome", "NOT_PRODUCED", verifyFailureReason(err)}
	}
	args := append([]string{"dogfood-record", "--base", c.base, "--target", c.target, "--task", task}, verify...)
	if c.options.AggregateOverLimit {
		args = append(args, "--aggregate-over-limit")
	}
	status, captureErr := c.recordWithClosedStages(append(args, "--outcome", c.options.Outcome), output, errorFile)
	if captureErr != nil {
		return step{"local-outcome", "NOT_PRODUCED", "aggregate-publication-failed"}
	}
	if status != 0 {
		reason := failureReason(readFile(errorFile), status)
		// Keep the legacy aggregate fallback classification separate from the public CLI code.
		if reason == "trace-path-limit-exceeded" {
			reason = "admitted-path-limit"
		}
		return step{"local-outcome", "NOT_PRODUCED", reason}
	}
	c.localOutcomeSHA, _ = fileSHA256(output)
	switch allCaptures(recordState, readFile(output)) {
	case "recorded":
		return step{"local-outcome", "PRODUCED", "none"}
	case "aggregate-recorded":
		if !c.options.AggregateOverLimit {
			return step{"local-outcome", "NOT_PRODUCED", "invalid-record-admission-output"}
		}
		if _, err := tracerecordrepo.ParseAggregateOutcome(readFile(output)); err != nil {
			return step{"local-outcome", "NOT_PRODUCED", "invalid-record-admission-output"}
		}
		return step{"local-outcome", "PRODUCED", "none"}
	case "no-source-paths":
		return step{"local-outcome", "NOT_PRODUCED", "no-source-paths"}
	}
	return step{"local-outcome", "NOT_PRODUCED", "invalid-record-admission-output"}
}

// verifyInput reads the verification commands as --verify argument pairs: the
// verify file's lines, else DOGFOOD_VERIFY. A non-empty reason is the
// local-outcome reason an unreadable or over-bound verify file records.
func verifyInput(options ChangeOptions) ([]string, string) {
	switch {
	case options.VerifyFile != "" && !isRegular(options.VerifyFile):
		return nil, "verify-file-unavailable"
	case options.VerifyFile != "":
		data, err := readPrefix(options.VerifyFile, maxVerifyFileBytes+1)
		if err != nil {
			return nil, "verify-file-unavailable"
		}
		if len(data) > maxVerifyFileBytes {
			return nil, "verify-file-over-bound"
		}
		return verifyArguments(data), ""
	case options.Verify != "":
		return verifyArguments([]byte(options.Verify + "\n")), ""
	}
	return nil, ""
}

// verifyFailureReason is the local-outcome reason of a validateVerify error.
func verifyFailureReason(err error) string {
	if reason := trace.VerificationFailureReason(err); reason != "" {
		return reason
	}
	return "record-failed"
}

// maxVerifyFileBytes bounds DOGFOOD_VERIFY_FILE well above the recorder's own
// 50 commands of 512 characters, leaving room for blank lines (DCW-V0-032).
const maxVerifyFileBytes = 65536

var recordState = regexp.MustCompile(`^.*"state":"([a-z-]*)".*$`)

// verifyArguments passes one recorder argument pair per line that is not
// blank (verifyLineArguments); the recorder and localOutcome share command
// validation before recorder invocation.
func verifyArguments(data []byte) []string {
	args := []string{}
	for _, line := range textLines(data) {
		if strings.TrimLeft(line, " \t\n\v\f\r") != "" {
			args = append(args, verifyLineArguments(line)...)
		}
	}
	return args
}

// noIntentManifest is the whole DOGFOOD_INTENTS_FILE content that declares a
// change with no requirements spec (DCW-V0-024). A "#" line is never an intent
// path, so it cannot collide with a manifest, and an unset variable or an
// empty file still refuses missing-intent-scope.
var noIntentManifest = []byte("#no-intent-declared\n")

// maxIntentManifestBytes is 16 paths of at most 512 bytes, each with its LF.
const maxIntentManifestBytes = 16 * 513

// validateIntentManifest freezes DOGFOOD_INTENTS_FILE: 1-16 sorted,
// LF-terminated, canonical repository-relative paths, or the no-intent
// declaration. It reads at most the bound plus one sentinel byte and returns
// the ocm-aggregate refusal code, or "" when the manifest is valid.
func (c *change) validateIntentManifest() string {
	data, paths, reason, issue := readIntentManifest(c.options.IntentsFile)
	c.manifestIssue = issue
	if reason != "" {
		return reason
	}
	c.intentSource = data
	if paths == nil {
		c.intentSnapshot, c.noIntent = data, true
		return ""
	}
	if len(paths) > maxAggregateScopes {
		// A well-formed list over the scope bound is not a missing scope
		// (DCW-V0-376); the byte bound above keeps the loop bounded.
		return "intent-scope-limit"
	}
	if c.recordIntent(paths) {
		// A record change names its build-log entry or decision as its intent;
		// it has no requirements section to assess, so the run publishes the
		// no-intent declaration the check accepts (DCW-V0-058, DCW-V0-024).
		c.intentSnapshot, c.noIntent = noIntentManifest, true
		return ""
	}
	c.intentSnapshot, c.intents = data, paths
	return ""
}

// readIntentManifest is the grammar check of DOGFOOD_INTENTS_FILE, which needs
// no Git read: the bytes and paths of a well-formed manifest (nil paths for the
// no-intent declaration), or the refusal code and the first fault it names
// (DCW-V0-387). finish-lane runs it before any step (DCW-V0-882).
func readIntentManifest(source string) (data []byte, paths []string, reason, issue string) {
	if source == "" || !isRegular(source) || isSymlink(source) {
		return nil, nil, "missing-intent-scope", ""
	}
	data, err := readPrefix(source, maxIntentManifestBytes+1)
	if err == nil && len(data) > maxIntentManifestBytes {
		return nil, nil, "intent-manifest-over-bound", ""
	}
	if err != nil || len(data) == 0 || data[len(data)-1] != '\n' {
		switch {
		case err == nil && len(data) == 0:
			issue = "the file is empty"
		case err == nil:
			issue = "the last line does not end with LF"
		}
		return nil, nil, "missing-intent-scope", issue
	}
	if bytes.Equal(data, noIntentManifest) {
		return data, nil, "", ""
	}
	paths = readLines(data)
	for number, path := range paths {
		if !canonicalIntentPath(path) {
			return nil, nil, "missing-intent-scope", fmt.Sprintf("line %d %q is not a canonical repository-relative path", number+1, path)
		}
		if number > 0 && paths[number-1] >= path {
			// DCW-V0-387: name the first line out of byte order and the rule.
			return nil, nil, "missing-intent-scope", fmt.Sprintf("line %d %q does not sort after line %d %q; the lines must be in strictly increasing byte order with no duplicates (LC_ALL=C sort -u)", number+1, path, number, paths[number-1])
		}
	}
	return data, paths, "", ""
}

// recordIntent reports a record change whose manifest names only build-log
// entries the change itself touches and decision files present at the target
// (DCW-V0-058).
func (c *change) recordIntent(paths []string) bool {
	changed := nonEmpty(strings.Split(c.gitValue("diff", "--no-renames", "--name-only", c.base, c.target), "\n"))
	if !RecordChange(changed) {
		return false
	}
	for _, path := range paths {
		if !recordIntentPath(path) {
			return false
		}
		if strings.HasPrefix(path, "docs/build-log/") && !slices.Contains(changed, path) {
			return false
		}
		if kind, status := c.git(true, "cat-file", "-t", c.target+":"+path); status != 0 || kind != "blob\n" {
			return false
		}
	}
	return true
}

func canonicalIntentPath(path string) bool {
	if path == "" || len(path) > 512 || path == "." || path == ".." {
		return false
	}
	for _, prefix := range []string{"#", "/", "./", "../"} {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}
	for _, part := range []string{`\`, "//", "/./", "/../"} {
		if strings.Contains(path, part) {
			return false
		}
	}
	for _, suffix := range []string{"/", "/.", "/.."} {
		if strings.HasSuffix(path, suffix) {
			return false
		}
	}
	return true
}

// citeStep applies the author's citation plan to the prepared map, staging
// intermediate maps so only the last cite publishes the tracked map. A map left
// by an earlier run is never cited when this run prepared none.
func (c *change) citeStep() {
	switch {
	case !c.cemPrepared || !isRegular(c.path(".corvint/change.cem.json")):
		c.addStep("cem-cite", "NOT_PRODUCED", "cem-map-not-produced")
		return
	case c.options.Citations == "":
		c.addStep("cem-cite", "NOT_PRODUCED", "citation-plan-not-provided")
		return
	case !isRegular(c.options.Citations):
		c.addStep("cem-cite", "NOT_PRODUCED", "citation-plan-unavailable")
		return
	}
	citeOutput := c.evidence + "/cem-cite.jsonl"
	_ = writePrivate(citeOutput, nil)
	_ = os.Chmod(citeOutput, 0o600)
	status, reason := "PRODUCED", "none"
	plan, shape := c.planShape()
	replaceRefusal := ""
	if shape == "" {
		replaceRefusal = c.resolveCitationReplace(plan)
	}
	switch {
	case shape != "":
		status, reason = "NOT_PRODUCED", shape
	case replaceRefusal != "":
		status, reason = "NOT_PRODUCED", replaceRefusal
	case c.recitesCitedHunk(plan):
		status, reason = "NOT_PRODUCED", "citation-plan-recites-cited-hunk"
	case c.citationCount > 1 && exists(c.path(c.citationStage)):
		status, reason = "NOT_PRODUCED", "citation-stage-exists"
	case c.prepareUnknownReasons(plan) != nil:
		status, reason = "NOT_PRODUCED", "unknown-reasons-not-recorded"
	default:
		// A remapped plan keeps the binding it was remapped through, so a
		// rerun of the same bytes remaps the same way (DCW-V0-340).
		if c.remappedPlan == nil {
			c.recordCitationBinding(plan)
		}
		cited := citedHunks(c.path(".corvint/change.cem.json"))
		before, _ := readPrefix(c.path(".corvint/change.cem.json"), maxPlanBytes)
		status, reason = c.cite(withRows(plan, c.sealCiteRows), citeOutput)
		c.citedOver = cited > 0 && status == "PRODUCED" && len(c.replaceApplied) == 0
		if status == "PRODUCED" {
			for _, id := range c.replaceApplied {
				c.say("dogfood-change: NOTE cem-cite replaced hunk=%s expected-digest=%s: only this hunk's earlier citations were replaced by the plan's rows for it; every other hunk kept its citations (DCW-V0-390)\n", id, c.replaceDigests[id])
			}
		}
		for _, id := range c.replaceReplayed {
			c.say("dogfood-change: NOTE cem-cite replace-already-applied hunk=%s: it already holds exactly the plan's rows for it, so the plan replays (DCW-V0-392)\n", id)
		}
		c.keepMapEncoding(before)
		if status == "PRODUCED" && c.publishUnknownReasons() != nil {
			status, reason = "NOT_PRODUCED", "unknown-reasons-not-recorded"
		}
	}
	if c.cleanupCitationStage() != nil {
		status, reason = "NOT_PRODUCED", "citation-stage-cleanup-failed"
	}
	c.addStep("cem-cite", status, reason)
}

// guardingTestsStep records the guarding-test search (TCQ-V0-062) as
// NOT_PRODUCED when the map holds a hunk that search would select. The search
// runs the target's tests unconfined for up to 1200 seconds, so dogfood change
// never runs it; the row and its note keep the step visible without blocking
// completion (DCW-V0-069).
func (c *change) guardingTestsStep() {
	data, err := readPrefix(c.path(".corvint/change.cem.json"), maxPlanBytes)
	if err != nil {
		return
	}
	if c.guardHunks = guardingTestHunks(data); c.guardHunks > 0 {
		c.addStep(guardingTestsStepName, "NOT_PRODUCED", guardingTestsNotRun)
	}
}

// GuardingTestHunksFile counts, with the same read bound and rule as the
// dogfood change row, the hunks of the map at path that the guarding-test
// search would select, so the staged bind path reports the same NOTE
// (DOGFOOD-BIND-008). A map that cannot be decoded counts 0.
func GuardingTestHunksFile(path string) (int, error) {
	data, err := readPrefix(path, maxPlanBytes)
	if err != nil {
		return 0, err
	}
	return guardingTestHunks(data), nil
}

const (
	guardingTestsStepName = "guarding-tests"
	guardingTestsNotRun   = "guarding-tests-not-run"
)

// shellWord quotes value as one POSIX shell word, so a NOTE command stays
// copyable when the repository path holds spaces or quotes.
func shellWord(value string) string {
	if value != "" && strings.Trim(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_./-:@+=,") == "" {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// guardingTestHunks counts the hunks cem verify --guarding-tests searches: not
// mechanical, in non-test Go source, and citing no _test.go span as test-claim.
func guardingTestHunks(data []byte) int {
	var decoded struct {
		Evidence []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		} `json:"evidence"`
		Hunks []struct {
			Disposition string `json:"disposition"`
			Path        string `json:"path"`
			Basis       []struct {
				EvidenceID string `json:"evidenceId"`
				Relation   string `json:"relation"`
			} `json:"basis"`
		} `json:"hunks"`
	}
	if json.Unmarshal(data, &decoded) != nil {
		return 0
	}
	paths := map[string]string{}
	for _, record := range decoded.Evidence {
		paths[record.ID] = record.Path
	}
	count := 0
	for _, hunk := range decoded.Hunks {
		if hunk.Disposition == "mechanical" || !strings.HasSuffix(hunk.Path, ".go") || strings.HasSuffix(hunk.Path, "_test.go") {
			continue
		}
		cites := false
		for _, basis := range hunk.Basis {
			cites = cites || basis.Relation == "test-claim" && strings.HasSuffix(paths[basis.EvidenceID], "_test.go")
		}
		if !cites {
			count++
		}
	}
	return count
}

// planShape runs, in the bind's order, the plan checks that read only the
// plan and the prepared map: row shape and bound, the map binding, and
// uniformity. It returns the plan to cite, remapped when the binding moved,
// and "" or the cem-cite refusal code. The bind's citeStep and the prepare
// pass's precheck share it, so a pre-flight refuses exactly what the bind
// would (DCW-V0-379).
func (c *change) planShape() ([]byte, string) {
	plan, valid := c.validateCitationPlan()
	mismatch := ""
	if valid {
		mismatch = c.citationPlanMismatch(plan)
		if mismatch == "" && c.remappedPlan != nil {
			plan = c.remappedPlan
		}
		c.citationCount += len(c.sealCiteRows)
	}
	switch {
	case !valid && c.citationCount > 256:
		return plan, "citation-plan-row-limit-exceeded"
	case !valid:
		return plan, "invalid-citation-plan"
	case c.unknownRelationRow(plan):
		return plan, "citation-plan-relation-unknown"
	case mismatch != "":
		return plan, mismatch
	// Appended sealed-hunk rows count toward uniformity, so they cannot
	// complete a plan whose own rows all cite their span (DCW-V0-341).
	case c.uniformPlan(plan) || len(plan) > 0 && len(c.sealCiteRows) > 0 && c.uniformPlan(withRows(plan, c.sealCiteRows)):
		return plan, "citation-plan-uniform"
	}
	return plan, ""
}

// uniformPlan reports, and keeps for the fix hint, a plan that cites one span
// for every hunk of a change with more than one hunk; it is refused before any
// cite runs (DCW-V0-048). A chore or record change is exempt (DCW-V0-058).
func (c *change) uniformPlan(plan []byte) bool {
	data, err := readPrefix(c.path(".corvint/change.cem.json"), maxPlanBytes)
	if err != nil {
		return false
	}
	hunks, span, uniform := uniformCitation(plan, mapHunks(data))
	if !uniform {
		hunks, span, uniform = c.carriedUniform(plan)
	}
	if !uniform || c.choreRole() != "" || c.recordChange() {
		return false
	}
	c.uniformHunks, c.uniformSpan = hunks, span
	return true
}

// carriedUniform reports a staged plan (DCW-V0-037) that completes a uniform
// citation: its rows all cite one span, and every hunk it does not name
// already cites only that span in the map, so the stages together cite one
// span for every hunk (DCW-V0-883).
func (c *change) carriedUniform(plan []byte) (int, string, bool) {
	m, ok := c.readCitationMap()
	if !ok || len(m.Hunks) < 2 {
		return 0, "", false
	}
	named, span := map[string]bool{}, ""
	for _, line := range textLines(plan) {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 || unknownRow(fields) {
			return 0, "", false
		}
		rowSpan, ok := canonicalSpan(fields[1], fields[2])
		if !ok || span != "" && rowSpan != span {
			return 0, "", false
		}
		span, named[fields[0]] = rowSpan, true
	}
	if span == "" {
		return 0, "", false
	}
	spans := map[string]string{}
	for _, evidence := range m.Evidence {
		spans[evidence.ID], _ = canonicalSpan(evidence.Path, strconv.FormatInt(evidence.Span.Start, 10)+":"+strconv.FormatInt(evidence.Span.End, 10))
	}
	carried := 0
	for index, hunk := range m.Hunks {
		if named[strconv.Itoa(index+1)] || named[hunk.ID] {
			continue
		}
		if len(hunk.Basis) == 0 {
			return 0, "", false
		}
		for _, basis := range hunk.Basis {
			if spans[basis.EvidenceID] != span {
				return 0, "", false
			}
		}
		carried++
	}
	if carried == 0 {
		return 0, "", false
	}
	return len(m.Hunks), span, true
}

// recitesCitedHunk reports, before any cite or mark runs, a plan row that
// would change a hunk the map already cites: a citation the hunk's basis does
// not already hold, or an unknown row for a cited hunk. cem cite only adds
// evidence, so such a row would join an earlier plan's citations instead of
// replacing them; a rerun of the same plan, and a staged plan naming only
// hunks with an empty basis (unknown or mechanical), still apply
// (DCW-V0-117). The row and hunk are kept for the fix hint. A row whose span
// this check cannot resolve is left to cem cite.
func (c *change) recitesCitedHunk(plan []byte) bool {
	m, ok := c.readCitationMap()
	if !ok {
		return false
	}
	evidence, blobs := m.evidenceKeys(), map[string][]byte{}
	for index, line := range readLines(plan) {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 {
			continue
		}
		hunk := m.hunkIndex(fields[0])
		// A hunk with no basis carries no earlier citation to join, and a
		// hunk DOGFOOD_CITATION_REPLACE names is replaced (DCW-V0-390).
		if hunk < 0 || len(m.Hunks[hunk].Basis) == 0 {
			continue
		}
		if _, replaced := c.replaceDigests[m.Hunks[hunk].ID]; replaced {
			continue
		}
		held, resolved := c.rowHeld(m, evidence, blobs, hunk, fields)
		if unknownRow(fields) {
			held = false
		}
		if resolved && !held {
			c.citationFailedRow, c.recitedHunk, c.recitedDigest = index+1, m.Hunks[hunk].ID, m.hunkDigest(hunk)
			return true
		}
	}
	return false
}

// baseLineSpan is the byte span START:END that cem cite derives from a
// one-based, inclusive line span of path at BASE, or false when the blob or
// span does not resolve (CEM evidenceSpan).
func (c *change) baseLineSpan(blobs map[string][]byte, path, lines string) (string, bool) {
	left, right, found := strings.Cut(lines, ":")
	first, firstErr := strconv.Atoi(left)
	last, lastErr := strconv.Atoi(right)
	if !found || firstErr != nil || lastErr != nil || first < 1 || last < first {
		return "", false
	}
	blob, cached := blobs[path]
	if !cached {
		output, status, bounded := c.boundedGit("cat-file", "blob", c.base+":"+path)
		if status != 0 || !bounded {
			return "", false
		}
		blob = []byte(output)
		blobs[path] = blob
	}
	records := bytes.SplitAfter(blob, []byte("\n"))
	if len(records) > 0 && len(records[len(records)-1]) == 0 {
		records = records[:len(records)-1]
	}
	if last > len(records) {
		return "", false
	}
	start := 0
	for _, record := range records[:first-1] {
		start += len(record)
	}
	end := start
	for _, record := range records[first-1 : last] {
		end += len(record)
	}
	return fmt.Sprintf("%d:%d", start, end), true
}

// declaresStableMap reports a regular map, read at most the native map bound,
// that declares cem/1.0, the only map the bind loop marks with a chore role.
func declaresStableMap(mapPath string) bool {
	if isSymlink(mapPath) {
		return false
	}
	data, err := readPrefix(mapPath, maxPlanBytes)
	return err == nil && wire.DeclaresStable(data)
}

// stableStatusArguments appends the explicit artifact root a cem/1.0 status
// requires: the absolute repository root, which a producer map without
// artifacts never opens (CEM-V1-146, CEM-V1-147). Every map is read as
// cem/1.0; a retired cem/0.x map is refused by the status reader with
// unsupported-spec (CEM-V1-224).
func stableStatusArguments(root string, args ...string) []string {
	return append(args, "--artifacts", stableArtifactRoot(root))
}

// stableArtifactRoot is the absolute spelling of the repository root, which
// the stable verifier requires of --artifacts.
func stableArtifactRoot(root string) string {
	if absolute, err := filepath.Abs(root); err == nil {
		return absolute
	}
	return root
}

// citedHunks counts the hunks of a map that no longer carry the prepared
// "unknown" disposition, which cem prepare keeps when it resumes the map.
func citedHunks(mapPath string) int {
	data, err := readPrefix(mapPath, maxPlanBytes)
	if err != nil {
		return 0
	}
	cited := 0
	for _, hunk := range mapHunks(data) {
		if hunk["disposition"] != "unknown" {
			cited++
		}
	}
	return cited
}

func (c *change) cite(plan []byte, citeOutput string) (string, string) {
	citeMap := ".corvint/change.cem.json"
	c.citationStageOwned = c.citationCount > 1
	replaced := map[string]bool{}
	for index, line := range readLines(plan) {
		fields := strings.SplitN(line, "\t", 4)
		args := []string{"cem", "cite", "--map", citeMap, "--hunk", fields[0], "--evidence-path", fields[1], "--lines", fields[2], "--relation", fields[3]}
		if unknownRow(fields) {
			args = []string{"cem", "mark", "--map", citeMap, "--hunk", fields[0], "--disposition", "unknown", "--reason", fields[2]}
		}
		args = append(args, c.replaceArgs(fields, replaced)...)
		if c.citationCount > 1 {
			destination := c.citationStage
			if index+1 == c.citationCount {
				destination = ".corvint/change.cem.json"
			}
			args = append(args, "--output", destination)
		}
		part := c.runTmp + "/cem-cite.json"
		errorFile := c.evidence + "/cem-cite.stderr"
		if status := c.exec(args, part, errorFile); status != 0 {
			c.citationFailedRow = index + 1
			c.citationFailedMap = citeMap
			if len(fields) == 4 && !unknownRow(fields) {
				c.citationFailedValues = "RELATION " + strconv.Quote(fields[3]) + " and START:END " + strconv.Quote(fields[2])
			}
			stderr := readFile(errorFile)
			c.citeSpanUnaligned = bytes.Contains(stderr, []byte(citeSpanUnedited))
			reason := failureReason(stderr, status)
			if planRowCodes[reason] && !unknownRow(fields) {
				c.probeCitationRows(readLines(plan), index, citeMap, reason)
			}
			return "NOT_PRODUCED", reason
		}
		// Intermediate receipts truthfully name the stage; only the last cite
		// publishes the original map. Keep their raw stdout as private evidence.
		appendFile(citeOutput, readFile(part))
		citeMap = c.citationStage
	}
	return "PRODUCED", "none"
}

// planRowCodes are the cem cite refusals a plan row itself causes, which
// another row of the same plan can repeat (DCW-V0-114).
var planRowCodes = map[string]bool{"cite-span-not-stable": true, "cite-self-modified-authority": true, "missing-evidence": true, "invalid-arguments": true}

// probeCitationRows reports every refused cite row in one pass: after the
// first refusal at row failed, each later cite row is cited against the same
// unchanged map into a private probe output that is removed at once, so no
// staged or tracked map changes. The checks a row can fail read only its own
// evidence and the map's patch, never another row's citation, so the probe
// sees what a corrected rerun would. Unknown rows are not probed (DCW-V0-114).
func (c *change) probeCitationRows(lines []string, failed int, citeMap, reason string) {
	c.citationRefusals = []string{citationRefusal(failed, lines[failed], reason)}
	probe := ".corvint/.cem-citations-probe." + filepath.Base(c.runTmp) + ".json"
	defer func() { _ = removeFile(c.path(probe)) }()
	for index := failed + 1; index < len(lines); index++ {
		fields := strings.SplitN(lines[index], "\t", 4)
		if len(fields) != 4 || unknownRow(fields) {
			continue
		}
		errorFile := c.runTmp + "/cem-cite-probe.stderr"
		status := c.exec([]string{"cem", "cite", "--map", citeMap, "--hunk", fields[0], "--evidence-path", fields[1], "--lines", fields[2], "--relation", fields[3], "--output", probe},
			c.runTmp+"/cem-cite-probe.json", errorFile)
		_ = removeFile(c.path(probe))
		if status != 0 {
			c.citationRefusals = append(c.citationRefusals, citationRefusal(index, lines[index], failureReason(readFile(errorFile), status)))
		}
	}
}

// citePrecheckStep is the finish-lane prepare row for a plan whose cite rows
// cem cite would refuse (DCW-V0-335).
const citePrecheckStep = "cem-cite-precheck"

// precheckCitationPlan probes every cite row of the plan against the map the
// prepare pass derived, before finish-lane spends its bind pass. Each row is
// cited into a private probe output that is removed at once, as
// probeCitationRows does, so the prepared map never changes. Only refusals a
// row itself causes are reported, each with cem cite's own detail, which for
// cite-span-not-stable names the duplicate copies and a wider unique span;
// plan shape and binding refusals stay the bind's to report (DCW-V0-335).
func (c *change) precheckCitationPlan() int {
	if c.options.Citations == "" || !isRegular(c.options.Citations) {
		return 0
	}
	plan, valid := c.validateCitationPlan()
	if !valid {
		return 0
	}
	probe := ".corvint/.cem-citations-probe." + filepath.Base(c.runTmp) + ".json"
	defer func() { _ = removeFile(c.path(probe)) }()
	reason := ""
	for index, line := range readLines(plan) {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 || unknownRow(fields) {
			continue
		}
		errorFile := c.runTmp + "/cem-cite-precheck.stderr"
		status := c.exec([]string{"cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", fields[0], "--evidence-path", fields[1], "--lines", fields[2], "--relation", fields[3], "--output", probe},
			c.runTmp+"/cem-cite-precheck.json", errorFile)
		_ = removeFile(c.path(probe))
		if status == 0 {
			continue
		}
		stderr := readFile(errorFile)
		code := failureReason(stderr, status)
		if !planRowCodes[code] {
			continue
		}
		if reason == "" {
			reason = code
		}
		refusal := citationRefusal(index, line, code)
		if detail := refusalDetail(stderr); detail != "" {
			refusal += ": " + detail
		}
		c.citationRefusals = append(c.citationRefusals, refusal)
	}
	if reason == "" {
		return 0
	}
	c.addStep(citePrecheckStep, "NOT_PRODUCED", reason)
	c.say("dogfood-change: FAIL %s refused plan rows=%d (cem cite against the prepared map, before any bind; fix every row, then rerun, DCW-V0-335):\n", citePrecheckStep, len(c.citationRefusals))
	for _, refusal := range c.citationRefusals[:min(len(c.citationRefusals), maxNamedRefusals)] {
		c.say("  %s\n", refusal)
	}
	if len(c.citationRefusals) > maxNamedRefusals {
		c.say("  (+%d more)\n", len(c.citationRefusals)-maxNamedRefusals)
	}
	return 1
}

// inputsPrecheckStep is the finish-lane prepare row for an input the bind
// would refuse before any cite runs (DCW-V0-379).
const inputsPrecheckStep = "dogfood-inputs-precheck"

// precheckInputs refuses, in the prepare pass and before any bind, an intent
// manifest or citation plan the bind pass would refuse with no step run: the
// manifest's ocm-aggregate reason when DOGFOOD_INTENTS_FILE is supplied, then
// the plan shape, map binding and uniformity checks citeStep runs first,
// through the same validators and with the bind's reason codes and fix hints.
// The checks that need the bind's own staging or notes state stay the bind's
// (DCW-V0-379).
func (c *change) precheckInputs() int {
	c.manifestReason = c.validateIntentManifest()
	c.manifestValid = c.manifestReason == ""
	if c.options.IntentsFile != "" && !c.manifestValid {
		return c.refuseInput(step{"ocm-aggregate", "NOT_PRODUCED", c.manifestReason}, "--intents "+c.options.IntentsFile)
	}
	if c.options.Citations == "" || !isRegular(c.options.Citations) {
		return 0
	}
	plan, reason := c.planShape()
	if reason != "" {
		input := "--citations " + c.options.Citations
		switch {
		case reason == "citation-plan-uniform":
			input += fmt.Sprintf(" rows=all hunks=%d span=%s", c.uniformHunks, c.uniformSpan)
		case c.mismatchDetail != "" && (reason == "citation-plan-map-mismatch" || reason == "empty-citation-plan"):
			input += " " + c.mismatchDetail
		case reason == "citation-plan-relation-unknown":
			input += " " + c.relationDetail
		}
		return c.refuseInput(step{"cem-cite", "NOT_PRODUCED", reason}, input)
	}
	return c.precheckBootstrapIntentRows(plan)
}

// refuseInput records the inputs precheck row and prints the bind step's
// reason, the offending input and that step's own fix hint.
func (c *change) refuseInput(row step, input string) int {
	c.addStep(inputsPrecheckStep, "NOT_PRODUCED", row.reason)
	c.say("dogfood-change: FAIL %s %s %s input=%s (the %s step would refuse it; nothing was bound, DCW-V0-379)\n", inputsPrecheckStep, row.name, row.reason, input, row.name)
	if hint := c.fixHint(row); hint != "" {
		c.say("  fix: %s\n", hint)
	}
	return 1
}

// refusalDetail is the error text of a cem cite refusal envelope on one line.
func refusalDetail(stderr []byte) string {
	var envelope struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(firstLine(stderr), &envelope) != nil {
		return ""
	}
	return oneLine(envelope.Error)
}

// citationRefusal names one refused plan row by number, hunk, path and span.
func citationRefusal(index int, line, reason string) string {
	fields := strings.SplitN(line, "\t", 4)
	for len(fields) < 3 {
		fields = append(fields, "")
	}
	return fmt.Sprintf("row %d hunk=%s path=%s lines=%s %s", index+1, fields[0], strconv.Quote(fields[1]), fields[2], reason)
}

// maxNamedRefusals bounds how many refused plan rows the report names.
const maxNamedRefusals = 32

// keepMapEncoding restores the map bytes read before a cite pass that changed
// only their encoding: the CEM contract fixes strict JSON, not a byte layout,
// so rerunning a plan the committed map already carries leaves the worktree
// clean (DCW-V0-019, V1-0386).
func (c *change) keepMapEncoding(before []byte) {
	mapPath := c.path(".corvint/change.cem.json")
	after, err := readPrefix(mapPath, maxPlanBytes)
	if err != nil || bytes.Equal(before, after) || !sameJSON(before, after) {
		return
	}
	info, err := os.Stat(mapPath)
	if err != nil {
		return
	}
	c.mapKeepTmp = c.path(".corvint/.change.cem.json." + strconv.Itoa(os.Getpid()))
	if writePrivate(c.mapKeepTmp, before) != nil || os.Chmod(c.mapKeepTmp, info.Mode().Perm()) != nil ||
		os.Rename(c.mapKeepTmp, mapPath) != nil {
		_ = removeFile(c.mapKeepTmp)
	}
	c.mapKeepTmp = ""
}

// sameJSON reports two documents that each hold one JSON value and hold the
// same one; numbers compare by their literal text.
func sameJSON(left, right []byte) bool {
	leftValue, leftOK := decodeJSON(left)
	rightValue, rightOK := decodeJSON(right)
	return leftOK && rightOK && reflect.DeepEqual(leftValue, rightValue)
}

func decodeJSON(data []byte) (any, bool) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil, false
	}
	_, err := decoder.Token()
	return value, err == io.EOF
}

func appendFile(name string, data []byte) {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o666)
	if err != nil {
		return
	}
	_, _ = file.Write(data)
	_ = file.Close()
}

func (c *change) cleanupCitationStage() error {
	if !c.citationStageOwned {
		return nil
	}
	if err := removeFile(c.path(c.citationStage)); err != nil {
		return err
	}
	c.citationStageOwned = false
	return nil
}

// validateCitationPlan freezes at most the byte ceiling plus one sentinel byte
// of DOGFOOD_CITATIONS and admits LF-terminated four-column rows only.
func (c *change) validateCitationPlan() (plan []byte, valid bool) {
	plan, err := readPrefix(c.options.Citations, maxPlanBytes+1)
	if err != nil || len(plan) > maxPlanBytes {
		return nil, false
	}
	c.citationCount = bytes.Count(plan, []byte("\n"))
	if c.citationCount > 256 {
		return nil, false
	}
	if len(plan) == 0 {
		return plan, true
	}
	if plan[len(plan)-1] != '\n' || hasForbiddenControl(plan) {
		return nil, false
	}
	// Deferred rows leave the plan before its cite rows are read: they are
	// not citations, so the cite count excludes them (DCW-V0-370).
	plan, c.deferrals, valid = splitDeferredRows(plan)
	if !valid {
		return nil, false
	}
	c.citationCount = bytes.Count(plan, []byte("\n"))
	for _, line := range textLines(plan) {
		if !nonEmptyFields(line, 4) {
			return nil, false
		}
		if fields := strings.Split(line, "\t"); unknownRow(fields) && !reasonedUnknownRow(fields) {
			return nil, false
		}
	}
	return plan, true
}

// unknownRelationRow reports, and keeps for the refusal, the first cite row
// whose RELATION is not one of the frozen basis relations cem cite admits, so
// the whole plan is refused before any cite with the full allowed list rather
// than one cite step's argument error (DCW-V0-402). Unknown rows carry a reason
// in that field and are not checked here.
func (c *change) unknownRelationRow(plan []byte) bool {
	for index, line := range textLines(plan) {
		fields := strings.Split(line, "\t")
		if len(fields) == 4 && !unknownRow(fields) && !wire.Relations[fields[3]] {
			c.relationDetail = fmt.Sprintf("row %d relation=%s", index+1, strconv.Quote(fields[3]))
			return true
		}
	}
	return false
}

// RelationChoices lists the registered basis relations in byte order: the one
// list the citation-plan refusal and docs/DOGFOOD.md name (DCW-V0-402).
func RelationChoices() []string {
	return slices.Sorted(maps.Keys(wire.Relations))
}

// unknownRow reports a plan row that keeps its hunk unknown instead of citing
// it: HUNK<TAB>unknown<TAB>REASON<TAB>NOTE (DCW-V0-019). A cite row's third
// field is a START:END span, so a field without ':' is never a cite.
func unknownRow(fields []string) bool {
	return len(fields) == 4 && fields[1] == "unknown" && !strings.Contains(fields[2], ":")
}

// recordedUnknownReasons are the map reasons an author records for a hunk kept
// unknown; the prepared no-evidence reason is not one, so the sealed map
// tells a reasoned unknown from an unreasoned one (DCW-V0-019).
var recordedUnknownReasons = map[string]bool{"insufficient-evidence": true, "conflicting-evidence": true}

// reasonedUnknownRow admits an unknown row with a recorded reason and a
// one-line note that names something: at least one letter or digit.
func reasonedUnknownRow(fields []string) bool {
	return recordedUnknownReasons[fields[2]] && strings.IndexFunc(fields[3], func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	}) >= 0
}

// prepareUnknownReasons builds, before anything is cited, the private notes
// file: one HUNK_ID<TAB>REASON<TAB>NOTE line per hunk kept unknown with a
// reason, in map order. It starts from the earlier lines of hunks the prepared
// map still records as unknown with the same reason, so a staged plan keeps
// the notes of earlier stages, then applies the accepted plan's rows in order:
// an unknown row replaces a hunk's note and a citation drops it. The map itself records only the reason. Only a regular
// file is read; an earlier file over the bound, a result over the bound or a
// directory at the path refuses, since the notes could not be retained whole.
func (c *change) prepareUnknownReasons(plan []byte) error {
	c.unknownNotes = nil
	path := c.evidence + unknownReasons
	info, err := os.Lstat(path)
	switch {
	case err == nil && info.IsDir():
		return errors.New("unknown reasons path is a directory")
	case err != nil && !os.IsNotExist(err):
		return err
	}
	notes := map[string][]string{}
	if err == nil && info.Mode().IsRegular() {
		previous, err := readPrefix(path, maxUnknownReasonBytes+1)
		if err != nil {
			return err
		}
		if len(previous) > maxUnknownReasonBytes {
			return errors.New("unknown reasons exceed the bound")
		}
		for _, line := range textLines(previous) {
			if fields := strings.SplitN(line, "\t", 3); len(fields) == 3 {
				notes[fields[0]] = fields
			}
		}
	}
	data, _ := readPrefix(c.path(".corvint/change.cem.json"), maxPlanBytes)
	hunks := mapHunks(data)
	kept := map[string]string{}
	for _, hunk := range hunks {
		if note, ok := notes[hunk["id"]]; ok && hunk["disposition"] == "unknown" && recordedUnknownReasons[hunk["reason"]] && note[1] == hunk["reason"] {
			kept[hunk["id"]] = strings.Join(note, "\t")
		}
	}
	for _, line := range textLines(plan) {
		fields := strings.Split(line, "\t")
		id := fields[0]
		if index, err := strconv.Atoi(id); err == nil && index >= 1 && index <= len(hunks) {
			id = hunks[index-1]["id"]
		}
		if unknownRow(fields) {
			kept[id] = id + "\t" + fields[2] + "\t" + fields[3]
		} else {
			delete(kept, id) // a citation supersedes an earlier note
		}
	}
	var notesFile strings.Builder
	for _, hunk := range hunks {
		if line, ok := kept[hunk["id"]]; ok {
			notesFile.WriteString(line + "\n")
		}
	}
	if notesFile.Len() > maxUnknownReasonBytes {
		return errors.New("unknown reasons exceed the bound")
	}
	if notesFile.Len() > 0 {
		c.unknownNotes = []byte(notesFile.String())
	}
	return nil
}

// publishUnknownReasons replaces the notes file only after the plan is cited,
// so a refused or failed stage leaves the earlier notes as they were. The new
// file is created owner-only beside the path and renamed over it; with no
// notes the path is removed.
func (c *change) publishUnknownReasons() error {
	path := c.evidence + unknownReasons
	if c.unknownNotes == nil {
		return removeFile(path)
	}
	next := path + ".next"
	if err := removeFile(next); err != nil {
		return err
	}
	if err := writePrivate(next, c.unknownNotes); err != nil {
		return errors.Join(err, removeFile(next))
	}
	if err := os.Rename(next, path); err != nil {
		return errors.Join(err, removeFile(next))
	}
	return nil
}

const unknownReasons = "/cem-unknown-reasons.tsv"

// reasonedUnknowns counts the map's unknown hunks whose reason the author
// recorded (recordedUnknownReasons): all of them, as reported, and those the
// allowance adds. With bootstrap set, a hunk on an intent path absent at BASE
// is already in the bootstrap allowance, so it is not added twice. Only a
// regular map is read, at most the native map bound.
func (f *flow) reasonedUnknowns(mapPath string, intents []string, bootstrap bool) (all, allowance int) {
	if !isRegular(mapPath) || isSymlink(mapPath) {
		return 0, 0
	}
	data, err := readPrefix(mapPath, maxPlanBytes)
	if err != nil {
		return 0, 0
	}
	absent := map[string]bool{}
	if bootstrap {
		for _, path := range intents {
			if !f.gitSucceeds("cat-file", "-e", f.base+":"+path) {
				absent[path] = true
			}
		}
	}
	for _, hunk := range mapHunks(data) {
		if hunk["disposition"] == "unknown" && recordedUnknownReasons[hunk["reason"]] {
			all++
			if !absent[hunk["path"]] {
				allowance++
			}
		}
	}
	return all, allowance
}

const maxPlanBytes = 4194304

// maxUnknownReasonBytes bounds the private notes file of DCW-V0-046, well
// above one maximum-size plan; notes over it refuse rather than truncate.
const maxUnknownReasonBytes = 4 * maxPlanBytes

func nonEmptyFields(line string, count int) bool {
	fields := strings.Split(line, "\t")
	if len(fields) != count {
		return false
	}
	for _, field := range fields {
		if field == "" {
			return false
		}
	}
	return true
}

var ordinal = regexp.MustCompile(`^[1-9][0-9]*$`)

// citationPlanMismatch binds a plan to the map prepared in this run
// (DCW-V0-019) and returns "" when it matches: no ordinal may exceed the map's
// hunk count, a numeric selector must be canonical, and every hunk the map
// records as unknown must be named by ordinal or full ID unless its path is an
// intent absent at BASE or more such hunks remain than one 256-row plan can
// name. An empty plan matches only a map that owes no hunk at all, and is
// otherwise refused as empty-citation-plan (DCW-V0-032). Only a regular map is
// read, at most the native 4 MiB map bound.
func (c *change) citationPlanMismatch(plan []byte) string {
	const mismatch = "citation-plan-map-mismatch"
	mapPath := c.path(".corvint/change.cem.json")
	if !isRegular(mapPath) || isSymlink(mapPath) {
		return ""
	}
	bootstrap := map[string]bool{}
	if c.manifestValid {
		for _, path := range c.intents {
			if !c.gitSucceeds("cat-file", "-e", c.base+":"+path) {
				bootstrap[path] = true
			}
		}
	}
	data, err := readPrefix(mapPath, maxPlanBytes)
	if err != nil {
		return mismatch
	}
	hunks := mapHunks(data)
	c.remappedPlan, c.sealCiteRows = nil, nil
	sealed := c.admittedSealHunks(hunks)
	// The deferral record the bind commit adds is a hunk the binding's map
	// lacked, so ordinal rows shifted only by it are remapped like ones
	// shifted by admitted sealed-CEM hunks (DCW-V0-372).
	records := deferralRecordHunks(hunks)
	inserted := maps.Clone(sealed)
	maps.Copy(inserted, records)
	if c.ordinalsMoved(plan, hunks, inserted) {
		return mismatch
	}
	if c.remappedPlan != nil {
		plan = c.remappedPlan
	}
	named := map[string]bool{}
	for _, line := range textLines(plan) {
		selector, _, _ := strings.Cut(line, "\t")
		named[selector] = true
	}
	mapIDs := map[string]bool{}
	for _, hunk := range hunks {
		mapIDs[hunk["id"]] = true
	}
	for index, line := range textLines(plan) {
		selector, _, _ := strings.Cut(line, "\t")
		// A hunk ID a later edit or amend re-hunked is refused here, naming
		// the current hunk, before cem cite rejects it (DCW-V0-441).
		if strings.HasPrefix(selector, wire.HunkPrefix) && !mapIDs[selector] {
			c.mismatchDetail = staleHunkRowDetail(index+1, selector, hunks, named)
			return mismatch
		}
		numeric := ordinal.MatchString(selector)
		if strings.ContainsAny(selector[:1], "+0123456789") && !numeric {
			c.mismatchDetail = fmt.Sprintf("row %d selector %s is not a canonical hunk ordinal", index+1, strconv.Quote(selector))
			return mismatch
		}
		if value, err := strconv.Atoi(selector); numeric && (err != nil || value > len(hunks)) {
			c.mismatchDetail = fmt.Sprintf("row %d ordinal %s is past the prepared map's %d hunks", index+1, selector, len(hunks))
			return mismatch
		}
	}
	// An unknown admitted sealed-CEM hunk the plan does not name counts as
	// cited by its verified admission while the admission span resolves at
	// BASE and the plan has room for its row (DCW-V0-341).
	citation := ""
	if len(sealed) > 0 {
		citation = c.sealCitation()
	}
	// A deferral record hunk the plan does not name is cited against the
	// DCW-V0-372 bullet at BASE the same way (DCW-V0-372).
	recordCitation := ""
	if len(records) > 0 {
		recordCitation = c.authorityCitation(deferralAuthorityAnchor)
	}
	// A hunk on a path whose whole BASE..HEAD change is one commit that
	// replays as regenerated is cited against the DCW-V0-401 bullet at BASE
	// (DCW-V0-398, V1-3214).
	regenCitation := c.authorityCitation(regenerationAuthorityAnchor)
	seals := []string{}
	for index, hunk := range hunks {
		if hunk["disposition"] != "unknown" || recordedUnknownReasons[hunk["reason"]] || named[strconv.Itoa(index+1)] || named[hunk["id"]] {
			continue
		}
		switch {
		case citation != "" && sealed[hunk["id"]]:
			seals = append(seals, hunk["id"]+"\t"+citation)
		case recordCitation != "" && records[hunk["id"]]:
			seals = append(seals, hunk["id"]+"\t"+recordCitation)
		case regenCitation != "" && c.regeneratedPath(hunk["path"]):
			seals = append(seals, hunk["id"]+"\t"+regenCitation)
		}
	}
	if len(textLines(plan))+len(seals) <= 256 {
		c.sealCiteRows = seals
	}
	selfCited := map[string]bool{}
	for _, row := range c.sealCiteRows {
		id, _, _ := strings.Cut(row, "\t")
		selfCited[id] = true
	}
	// A hunk an earlier plan kept unknown with a recorded reason is marked,
	// so, like a cited one, it needs no row (DCW-V0-019).
	owed, unmarked := []int{}, 0
	for index, hunk := range hunks {
		if hunk["disposition"] == "unknown" && !bootstrap[hunk["path"]] && !recordedUnknownReasons[hunk["reason"]] {
			unmarked++
			if !selfCited[hunk["id"]] {
				owed = append(owed, index+1)
			}
		}
	}
	// More owed hunks than one plan has rows: split plans stay admissible.
	// Appended sealed-hunk rows share the 256-row budget, so they count.
	if unmarked > 256 && len(plan) > 0 {
		return ""
	}
	for _, index := range owed {
		if !named[strconv.Itoa(index)] && !named[hunks[index-1]["id"]] {
			c.mismatchDetail = fmt.Sprintf("hunk %d path=%s is owed a row the plan does not name", index, strconv.Quote(hunks[index-1]["path"]))
			if len(plan) == 0 {
				return "empty-citation-plan"
			}
			return mismatch
		}
	}
	return ""
}

// citationBinding is the private record of the hunk IDs, in map order, that the
// last accepted plan was cited against: its first line is the plan's digest.
const citationBinding = "/citation-plan-binding"

func (c *change) recordCitationBinding(plan []byte) {
	lines := []string{sha256Hex(plan)}
	for _, hunk := range mapHunks(readFile(c.path(".corvint/change.cem.json"))) {
		lines = append(lines, hunk["id"])
	}
	_ = writePrivate(c.evidence+citationBinding, []byte(strings.Join(lines, "\n")+"\n"))
}

// ordinalsMoved reports a plan whose ordinal row named a hunk that the map now
// holds at another ordinal, so the row would cite the wrong hunk (V1-0239).
// When every such shift comes only from admitted sealed-CEM hunks the
// recorded map lacked, and every ordinal row's recorded hunk is still in the
// map, it remaps those rows through the recorded hunk IDs into remappedPlan
// and prints one NOTE instead (DCW-V0-340); any other shift still refuses.
func (c *change) ordinalsMoved(plan []byte, hunks []map[string]string, sealed map[string]bool) bool {
	recorded := readLines(readFile(c.evidence + citationBinding))
	if len(recorded) == 0 || recorded[0] != sha256Hex(plan) {
		return false
	}
	known := map[string]bool{}
	for _, id := range recorded[1:] {
		known[id] = true
	}
	current, withoutSeals := map[string]int{}, map[string]int{}
	inserted := 0
	for index, hunk := range hunks {
		current[hunk["id"]] = index + 1
		if sealed[hunk["id"]] && !known[hunk["id"]] {
			inserted++
			continue
		}
		withoutSeals[hunk["id"]] = index + 1 - inserted
	}
	lines := textLines(plan)
	remapped := append([]string(nil), lines...)
	moved, absent := 0, ""
	for index, line := range lines {
		selector, rest, _ := strings.Cut(line, "\t")
		value, err := strconv.Atoi(selector)
		if !ordinal.MatchString(selector) || err != nil || value >= len(recorded) {
			continue
		}
		now, found := current[recorded[value]]
		if !found {
			if absent == "" {
				absent = fmt.Sprintf("plan row %d (ordinal %d) named hunk %s, which the current map no longer holds, so its ordinals cannot be remapped past the admitted sealed-CEM hunks", index+1, value, recorded[value])
			}
			continue
		}
		if now == value {
			continue
		}
		if withoutSeals[recorded[value]] != value {
			c.staleBinding = fmt.Sprintf("plan row %d (ordinal %d) named hunk %s, which the current map holds at ordinal %d", index+1, value, recorded[value], now)
			return true
		}
		moved++
		remapped[index] = strconv.Itoa(now) + "\t" + rest
	}
	if moved == 0 {
		return false
	}
	if absent != "" {
		c.staleBinding = absent
		return true
	}
	c.remappedPlan = []byte(strings.Join(remapped, "\n") + "\n")
	c.say("%s: NOTE citation-plan-remapped rows=%d sealedHunks=%d binding=%s/citation-plan-binding (DCW-V0-340)\n", c.prefix, moved, inserted, c.evidence)
	return false
}

// admittedSealHunks is the set of the map's hunk IDs on a sealed CEM path that
// dogfood change admitted as an unmodified, re-verified merged seal
// (DCW-V0-042); a retired or migrated seal is not in it.
func (c *change) admittedSealHunks(hunks []map[string]string) map[string]bool {
	paths := map[string]bool{}
	for _, path := range c.mergedSeals {
		paths[path] = true
	}
	sealed := map[string]bool{}
	for _, hunk := range hunks {
		if paths[hunk["path"]] {
			sealed[hunk["id"]] = true
		}
	}
	return sealed
}

// sealAuthority and sealAuthorityAnchor locate, at BASE, the DCW-V0-042
// requirement that admits a merged sealed CEM: the pinned span an admitted
// sealed-CEM hunk is cited against (DCW-V0-341). deferralAuthorityAnchor is
// the DCW-V0-372 requirement a deferral record hunk is cited against.
const (
	sealAuthority           = "docs/specs/daily-change-evidence-workflow-v0.md"
	sealAuthorityAnchor     = "- `DCW-V0-042`:"
	deferralAuthorityAnchor = "- `DCW-V0-372`:"
	// regenerationAuthorityAnchor is the DCW-V0-401 requirement a regenerated
	// hunk is cited against (DCW-V0-398).
	regenerationAuthorityAnchor = "- `DCW-V0-401`:"
)

// regeneratedPath reports whether path's whole BASE..HEAD change is one
// non-merge commit that replays as regenerated from its parent (DCW-V0-398):
// then BASE holds that parent's bytes of path and HEAD that commit's, so every
// hunk on path is generated. A path any other commit also touches, or whose
// one commit has a hand edit anywhere, still owes its citation rows.
func (c *change) regeneratedPath(path string) bool {
	listing, status, bounded := c.boundedGit("rev-list", "--full-history", c.base+"..HEAD", "--", path)
	commits := nonEmpty(strings.Split(listing, "\n"))
	if status != 0 || !bounded || len(commits) != 1 {
		return false
	}
	// Path limiting rewrites --parents, so the real parents are read apart.
	parents := nonEmpty(strings.Split(c.gitValue("rev-parse", commits[0]+"^@"), "\n"))
	return len(parents) == 1 && c.regenerated(parents[0], commits[0]).ok
}

// sealCitation cites the DCW-V0-042 bullet (DCW-V0-341).
func (c *change) sealCitation() string {
	return c.authorityCitation(sealAuthorityAnchor)
}

// authorityCitation returns the EVIDENCE<TAB>START:END<TAB>specification
// fields citing the workflow spec bullet that starts with anchor as BASE holds
// it: its anchor line through its last two-space continuation line. A BASE
// without that spec, without the anchor, or whose spec read exceeds the flow's
// Git bound returns "", and the hunks stay owed by the plan.
func (c *change) authorityCitation(anchor string) string {
	text, status, bounded := c.boundedGit("cat-file", "blob", c.base+":"+sealAuthority)
	if status != 0 || !bounded {
		return ""
	}
	lines := textLines([]byte(text))
	for start, line := range lines {
		if !strings.HasPrefix(line, anchor) {
			continue
		}
		end := start + 1
		for end < len(lines) && strings.HasPrefix(lines[end], "  ") {
			end++
		}
		return fmt.Sprintf("%s\t%d:%d\tspecification", sealAuthority, start+1, end)
	}
	return ""
}

// withRows is plan with rows appended, each LF-terminated.
func withRows(plan []byte, rows []string) []byte {
	if len(rows) == 0 {
		return plan
	}
	return append(append([]byte(nil), plan...), []byte(strings.Join(rows, "\n")+"\n")...)
}

// mapHunks decodes the disposition, id and path of each hunk of a map in map
// order, whatever its JSON layout, so a compact map reads like the indent-2 one
// cem prepare writes. A cem/1.0 map's hunks carry the same members, so the
// bind loop cites, counts and binds a plan to them alike (CEM-V1-147). A map
// that does not decode has no hunks.
func mapHunks(data []byte) []map[string]string {
	hunks := []map[string]string{}
	var decoded struct {
		Hunks []struct {
			Disposition string `json:"disposition"`
			ID          string `json:"id"`
			Path        string `json:"path"`
			Reason      string `json:"reason"`
		} `json:"hunks"`
	}
	if json.Unmarshal(data, &decoded) != nil {
		return hunks
	}
	for _, hunk := range decoded.Hunks {
		hunks = append(hunks, map[string]string{"disposition": hunk.Disposition, "id": hunk.ID, "path": hunk.Path, "reason": hunk.Reason})
	}
	return hunks
}

// runOCMScopes prepares, links and checks one OCM map per frozen intent.
func (c *change) runOCMScopes() bool {
	c.loadOCMLinkPlan()
	failed := false
	for index, path := range c.intents {
		number := fmt.Sprintf("%03d", index+1)
		mapPath := ".corvint/change.ocm." + number + ".json"
		prepared := c.prepare("ocm-prepare-"+number, c.evidence+"/ocm-prepare-"+number+".json", func(_ int, _ []byte, reason string) bool {
			return reason == "map-outdated"
		}, "ocm", "prepare", "--map", mapPath, "--cem", ".corvint/change.cem.json", "--intent", path, "--expected-base", c.base, "--target", c.target)
		if prepared != 0 {
			failed = true
			c.skipOCMLinks(path)
		} else if c.linksReady {
			c.runOCMLinks(mapPath, path)
		}
		if c.runStep("ocm-status-"+number, c.evidence+"/ocm-status-"+number+".json", "ocm", "status", "--map", mapPath,
			"--cem", ".corvint/change.cem.json", "--expected-base", c.base, "--target", c.target) != 0 {
			failed = true
		}
	}
	return !failed
}

// loadOCMLinkPlan freezes the optional DOGFOOD_OCM_LINKS plan and records its
// digest and row count (DCW-V0-018). Without it no obligation is linked and no
// row is reported.
func (c *change) loadOCMLinkPlan() {
	source := c.options.OCMLinks
	if source == "" {
		return
	}
	if !isRegular(source) {
		c.addStep("ocm-links", "NOT_PRODUCED", "ocm-link-plan-unavailable")
		return
	}
	plan, err := readPrefix(source, maxPlanBytes+1)
	if err != nil {
		c.addStep("ocm-links", "NOT_PRODUCED", "invalid-ocm-link-plan")
		return
	}
	if len(plan) == 0 {
		c.addStep("ocm-links", "NOT_PRODUCED", "empty-ocm-link-plan")
		return
	}
	rows := bytes.Count(plan, []byte("\n"))
	c.linkObservedRows = rows
	if len(plan) <= maxPlanBytes && rows > 256 {
		c.addStep("ocm-links", "NOT_PRODUCED", "ocm-link-plan-row-limit-exceeded")
		return
	}
	if len(plan) > maxPlanBytes || plan[len(plan)-1] != '\n' || hasForbiddenControl(plan) || !c.linkRowsValid(plan) {
		c.addStep("ocm-links", "NOT_PRODUCED", "invalid-ocm-link-plan")
		return
	}
	c.linkPlan, c.linkPlanSHA, c.linkPlanRows, c.linksReady = plan, sha256Hex(plan), rows, true
	c.addStep("ocm-links", "PRODUCED", "none")
}

var emptyListItem = regexp.MustCompile(`(^|,)(,|$)`)

func (c *change) linkRowsValid(plan []byte) bool {
	scope := map[string]bool{}
	for _, path := range textLines(c.intentSnapshot) {
		scope[path] = true
	}
	for _, line := range textLines(plan) {
		fields := strings.Split(line, "\t")
		if len(fields) != 5 || fields[1] == "" || fields[3] == "" || emptyListItem.MatchString(fields[2]) || emptyListItem.MatchString(fields[4]) || !scope[fields[0]] {
			return false
		}
	}
	return true
}

// runOCMLinks applies each author row naming this scope through the verified
// `ocm link`; a link is never inferred, and each row reports ocm-link-<plan row>.
func (c *change) runOCMLinks(mapPath, path string) {
	for index, line := range readLines(c.linkPlan) {
		fields := strings.SplitN(line, "\t", 5)
		if fields[0] != path {
			continue
		}
		name := fmt.Sprintf("ocm-link-%03d", index+1)
		args := []string{"ocm", "link", "--map", mapPath, "--cem", ".corvint/change.cem.json", "--obligation", fields[1],
			"--test-path", fields[3], "--expected-base", c.base, "--target", c.target}
		for _, hunk := range strings.Split(fields[2], ",") {
			args = append(args, "--hunk", hunk)
		}
		for _, claim := range strings.Split(fields[4], ",") {
			args = append(args, "--claim", claim)
		}
		c.runStep(name, c.evidence+"/"+name+".json", args...)
	}
}

// skipOCMLinks reports each plan row naming an intent whose map did not
// prepare, so no row is dropped without a reason (V1-0227).
func (c *change) skipOCMLinks(path string) {
	for index, line := range readLines(c.linkPlan) {
		if strings.SplitN(line, "\t", 2)[0] == path {
			c.addStep(fmt.Sprintf("ocm-link-%03d", index+1), "NOT_PRODUCED", "ocm-map-not-prepared")
		}
	}
}

// declareNoIntent reports each OCM step as not assessed and publishes the
// declaration the check reads (DCW-V0-024). No intent can own a link row, so a
// supplied link plan still refuses.
func (c *change) declareNoIntent() {
	if c.options.OCMLinks != "" {
		c.addStep("ocm-links", "NOT_PRODUCED", "invalid-ocm-link-plan")
	}
	c.addStep("ocm-prepare", "NOT_PRODUCED", "no-intent-declared")
	c.addStep("ocm-status", "NOT_PRODUCED", "no-intent-declared")
	if c.publishIntentManifest() {
		c.addStep("ocm-aggregate", "NOT_PRODUCED", "no-intent-declared")
	}
}

// finishOCMAggregate publishes the frozen manifest and verifies the ordered
// map set against it.
func (c *change) finishOCMAggregate() bool {
	return c.publishIntentManifest() &&
		c.runStep("ocm-aggregate", c.path(".corvint/change.ocm-status.json"), "dogfood-ocm", "status", "--expected-base", c.base, "--target", c.target) == 0
}

// publishIntentManifest publishes the frozen manifest, unless its source
// changed during the run.
func (c *change) publishIntentManifest() bool {
	current, err := os.ReadFile(c.options.IntentsFile)
	if err != nil || !bytes.Equal(current, c.intentSource) {
		c.addStep("ocm-aggregate", "NOT_PRODUCED", "intent-scope-drift")
		return false
	}
	c.intentPublishTmp = c.path(".corvint/.change.ocm-intents." + strconv.Itoa(os.Getpid()))
	if writePrivate(c.intentPublishTmp, c.intentSnapshot) != nil || os.Chmod(c.intentPublishTmp, 0o600) != nil ||
		os.Rename(c.intentPublishTmp, c.path(".corvint/change.ocm-intents")) != nil {
		c.addStep("ocm-aggregate", "NOT_PRODUCED", "intent-scope-drift")
		return false
	}
	c.intentPublishTmp = ""
	return true
}

func (c *change) countBootstrapUnknowns() {
	c.bootstrapUnknown = bootstrapUnknowns(&c.flow, c.intents)
}

// bootstrapUnknowns counts the intents absent at BASE.
func bootstrapUnknowns(f *flow, intents []string) int {
	count := 0
	for _, path := range intents {
		if !f.gitSucceeds("cat-file", "-e", f.base+":"+path) {
			count++
		}
	}
	return count
}

// failing reports a row that keeps the report incomplete; the typed
// abstentions do not.
func failing(row step) bool {
	switch {
	case row.status == "PRODUCED":
		return false
	case row.name == "local-outcome" && row.status == "NOT_PRODUCED" && row.reason == "no-source-paths":
		return false
	case row.name == "coordination-time-impact" && row.status == "NOT_PRODUCED" && impactAbstentions[row.reason]:
		return false
	case row == step{"coordination-time-impact", "NOT_PRODUCED", impactDeadlineReason}:
		return false
	case row.reason == "no-intent-declared":
		return false
	case row == step{guardingTestsStepName, "NOT_PRODUCED", guardingTestsNotRun}:
		return false
	}
	return true
}

func (c *change) complete() bool {
	queryRows := 0
	for _, row := range c.rows {
		if row.name == queryStep {
			queryRows++
		}
	}
	if queryRows > 1 {
		return false
	}
	for _, row := range c.rows {
		if c.rowFailing(row) {
			return false
		}
	}
	return true
}

// renderReport writes the report bytes of profile corvint-dogfood-change/0 and
// reports each row as a self-observation.
func (c *change) renderReport() {
	var report bytes.Buffer
	fmt.Fprintf(&report, "{\n  \"profile\": \"corvint-dogfood-change/0\",\n  \"base\": \"%s\",\n  \"target\": \"%s\",\n  \"complete\": %t,\n  \"steps\": [\n", c.base, c.target, c.complete())
	for index, row := range c.rows {
		if index > 0 {
			report.WriteString(",\n")
		}
		fmt.Fprintf(&report, "    {\"name\": \"%s\", \"status\": \"%s\", \"reason\": \"%s\"}", row.name, row.status, row.reason)
	}
	report.WriteString("\n  ],\n")
	aggregate := readFile(c.path(".corvint/change.ocm-status.json"))
	switch {
	case c.noIntent:
		report.WriteString(noIntentOCMStatus + "\n")
	case len(aggregate) > 0:
		report.WriteString("  \"ocmStatus\": ")
		report.Write(firstLine(aggregate))
	default:
		report.WriteString("  \"ocmStatus\": null\n")
	}
	report.WriteString(`  ,"localOutcomeEvidenceSha256": ` + digestOrNull(c.localOutcomeSHA) + "\n")
	report.WriteString(`  ,"contextAbstentionEvidenceSha256": ` + digestOrNull(c.contextAbstentionSHA) + "\n")
	report.WriteString(`  ,"queryAbstentionEvidenceSha256": ` + digestOrNull(c.queryAbstentionSHA) + "\n")
	if c.anchorObserved {
		fmt.Fprintf(&report, "  ,\"anchor\": {\"state\": \"OBSERVED\", \"mergeBase\": \"%s\"}\n", c.anchorMergeBase)
	} else {
		report.WriteString("  ,\"anchor\": {\"state\": \"NOT_OBSERVED\", \"mergeBase\": null}\n")
	}
	if c.linkPlanSHA != "" {
		fmt.Fprintf(&report, "  ,\"ocmLinkPlan\": {\"sha256\": \"sha256:%s\", \"rows\": %d}\n", c.linkPlanSHA, c.linkPlanRows)
	} else {
		report.WriteString("  ,\"ocmLinkPlan\": null\n")
	}
	fmt.Fprintf(&report, "  ,\"dogfoodPolicy\": {\"bootstrapUnknown\": %d, \"maximumUnknownAfterBootstrap\": 0}\n", c.bootstrapUnknown)
	fmt.Fprintf(&report, "  ,\"packetCoverage\": [%s, %s]\n", c.packetCoverage("coordination-time-query"), c.packetCoverage("coordination-time-impact"))
	report.WriteString("  ,\"dogfoodCheck\": null\n}\n")
	_ = os.WriteFile(c.path(".corvint/dogfood-report.json"), report.Bytes(), 0o666)
	for _, row := range c.rows {
		c.options.Steps.Run(c.ctx, c.root, []string{"dogfood-observe", "--step", row.name, "--status", row.status, "--reason", row.reason}, io.Discard, io.Discard)
		c.checkpoint()
	}
}

// noIntentOCMStatus is the report's ocmStatus line when the change declared no
// intent: intent linkage was not assessed, never covered (DCW-V0-024).
const noIntentOCMStatus = `  "ocmStatus": {"state": "NOT_ASSESSED", "reason": "no-intent-declared"}`

func digestOrNull(digest string) string {
	if digest == "" {
		return "null"
	}
	return `"sha256:` + digest + `"`
}

var packetFields = []struct{ key, pattern string }{
	{"packet_bytes", `0|[1-9][0-9]*`},
	{"budget_bytes", `null|0|[1-9][0-9]*`},
	{"within_budget", `true|false`},
	{"included_results", `0|[1-9][0-9]*`},
	{"omitted_results", `0|[1-9][0-9]*`},
}

// packetCoverage copies one step's packet coverage fields under the packet's
// own names (DCW-V0-016). A step that compiled no packet, or whose output does
// not carry exactly one well-formed occurrence of each field, is reported
// NOT_PRODUCED rather than given numbers it did not produce.
func (c *change) packetCoverage(name string) string {
	status := ""
	for _, row := range c.rows {
		if row.name == name {
			status = row.status
			break
		}
	}
	if status != "PRODUCED" {
		return `{"step": "` + name + `", "status": "NOT_PRODUCED", "reason": "packet-not-compiled"}`
	}
	output, err := os.ReadFile(c.evidence + "/" + name + ".json")
	fields := ""
	for _, field := range packetFields {
		occurrences := regexp.MustCompile(`[{,]"`+field.key+`":`).FindAll(output, -1)
		matches := regexp.MustCompile(`[{,]"`+field.key+`":(`+field.pattern+`)[,}]`).FindAllSubmatch(output, -1)
		if err != nil || len(occurrences) != 1 || len(matches) != 1 {
			return `{"step": "` + name + `", "status": "NOT_PRODUCED", "reason": "packet-coverage-unreadable"}`
		}
		fields += `, "` + field.key + `": ` + string(matches[0][1])
	}
	return `{"step": "` + name + `", "status": "PRODUCED"` + fields + `}`
}

// fixHints names the fix for each refusal a first-time adopter hits
// (DCW-V0-014, docs/DOGFOOD.md "Daily adopter path"); the first matching
// STEP:REASON pattern wins and "*" matches any text.
var fixHints = []struct{ pattern, hint string }{
	{"*:git-budget-exceeded", "the Git operation bound already scales with the map (1,024 plus 4 per hunk and evidence record; cem status reruns with --scaled-operations, DCW-V0-373), so a pass needed more operations than its records explain; rerun the same corvint dogfood change {base} with the --corvint-bin that wrote the map, and if it repeats keep {evidence}/{step}.stderr and report it; meanwhile the change can be bound and sealed as smaller changes and checked with corvint dogfood batch-check {base} (DCW-V0-039, DCW-V0-040)"},
	{"cem-*:map-unavailable", "the CEM map {map} could not be read ({evidence}/{step}.stderr says cannot read CEM map): it is missing, not a regular file or not a valid map, often because it was removed or edited while the run was in progress; restore a committed map with git restore --source=HEAD -- .corvint/change.cem.json, or rerun corvint dogfood change {base} to prepare it again (DCW-V0-068)"},
	{"cem-cite:citation-plan-not-provided", "set DOGFOOD_CITATIONS to the path of a TSV plan with one row per hunk of .corvint/change.cem.json"},
	{"cem-cite:citation-plan-unavailable", "DOGFOOD_CITATIONS must be the path of a TSV file of ORDINAL<TAB>PATH<TAB>START:END<TAB>RELATION rows, not the rows themselves"},
	{"cem-cite:empty-citation-plan", "DOGFOOD_CITATIONS names an empty file but .corvint/change.cem.json still has unknown hunks; write one row per unknown hunk (corvint dogfood propose-citations {base} prints a per-hunk draft to review and write there), or unset DOGFOOD_CITATIONS to prepare the map without citing (DCW-V0-150)"},
	{"cem-cite:citation-plan-row-limit-exceeded", "DOGFOOD_CITATIONS row limit=256 observed={citationRows}; use explicit staged plans of at most 256 rows on the same CEM (docs/DOGFOOD.md Staged binding of one oversized commit)"},
	{"cem-cite:invalid-citation-plan", "each row is ORDINAL<TAB>PATH<TAB>START:END<TAB>RELATION in worklist order, LF-terminated, at most 256 rows; a hunk kept unknown takes ORDINAL<TAB>unknown<TAB>REASON<TAB>NOTE, REASON being insufficient-evidence or conflicting-evidence and NOTE a one-line reason (DCW-V0-019)"},
	{"cem-cite:citation-plan-relation-unknown", "RELATION, the fourth field of a cite row, is one of {relations}; a test file or test case cited as evidence takes test-claim (tests is not a relation); fix that row and every other, then rerun (DCW-V0-402)"},
	{"cem-cite:citation-plan-map-mismatch", "the plan does not match the map prepared for HEAD: a row names an ordinal past its hunks or is not a canonical ordinal, or an unknown hunk is unnamed (often because a later commit re-prepared the map); or a row's ordinal now names another hunk than when this plan was first cited (a later commit added or removed a hunk before it); rewrite DOGFOOD_CITATIONS from the current .corvint/change.cem.json, naming every unknown hunk except the hunk of an intent spec absent at BASE, by a citation or by an ORDINAL<TAB>unknown<TAB>REASON<TAB>NOTE row"},
	{"cem-cite:citation-plan-uniform", "the plan cites one span for every hunk: hunks={uniformHunks} span={uniformSpan}; cite, per hunk, the BASE lines that hunk depends on or implements (its caller, contract, test or spec section), or keep a hunk unknown with ORDINAL<TAB>unknown<TAB>REASON<TAB>NOTE; one span may serve several hunks, never every hunk of a multi-hunk non-chore change; for example 1<TAB>docs/specs/x-v0.md<TAB>40:44<TAB>specification for a spec hunk and 2<TAB>internal/x/x.go<TAB>120:126<TAB>dependency for the code hunk that implements it; corvint dogfood propose-citations {base} prints a per-hunk draft to review and write to the plan (DCW-V0-048, DCW-V0-150)"},
	{"cem-cite:citation-plan-recites-cited-hunk", "plan row {row} would change hunk {recitedHunk}, which .corvint/change.cem.json already cites; cem cite only adds evidence, so nothing was cited or marked; to replace only that hunk's earlier citations with this plan's rows for it, keeping every other hunk's citations, rerun DOGFOOD_CITATION_REPLACE={recitedHunk}={recitedDigest} corvint dogfood change {base} with the same plan (DCW-V0-390); a staged plan otherwise names only hunks still unknown (DCW-V0-117)"},
	{"cem-cite:citation-replace-digest-mismatch", "DOGFOOD_CITATION_REPLACE expects an earlier state of hunk {recitedHunk}: the map changed since the digest was read, so nothing was cited or marked; reread the map and, if the replacement is still wanted, rerun DOGFOOD_CITATION_REPLACE={recitedHunk}={recitedDigest} corvint dogfood change {base} (DCW-V0-391)"},
	{"cem-cite:citation-replace-invalid", "DOGFOOD_CITATION_REPLACE takes at most 16 comma- or space-separated HUNK_ID=DIGEST entries, each naming by full hunk ID, once, a hunk of .corvint/change.cem.json that the plan names, with its 64-hex digest from the citation-plan-recites-cited-hunk fix line; nothing was cited or marked (DCW-V0-391)"},
	{"cem-cite:unknown-reasons-not-recorded", "the unknown rows of DOGFOOD_CITATIONS could not be kept in {evidence}/cem-unknown-reasons.tsv (or its .next file): before citing (an occupied path, or notes over 16 MiB) nothing was cited; after citing the map has changed and earlier notes are kept; remove whatever occupies the path and rerun the same plan, which replays idempotently (DCW-V0-046)"},
	{"cem-cite:invalid-arguments", "plan row {row}{rowValues}: RELATION must be one of call-site, decision, dependency, implementation, incident, specification or test-claim, and START:END a line span of the cited file; any file present at BASE may be cited, code and tests as well as specs (DCW-V0-041)"},
	{"cem-cite:missing-evidence", "plan row {row} cites a path that is not a regular file at BASE; cite a source, test or spec file that exists at BASE, since a file this change adds cannot be evidence for itself (DCW-V0-041)"},
	{"cem-cite:cite-span-not-stable", "plan row {row} cites BASE lines that this change edits or deletes; plan spans are BASE line numbers, counted in the file as it is at BASE {base}, not at HEAD; cite a START:END span the change leaves unchanged (DCW-V0-845)"},
	{"cem-cite:cite-self-modified-authority", "plan row {row} cites a governing file (spec, decision or other authority) that this change itself modifies, which cannot govern it; cite a governing source the change leaves unmodified, or keep the hunk unknown with ORDINAL<TAB>unknown<TAB>REASON<TAB>NOTE (CEM-V1-114)"},
	{"ocm-aggregate:missing-intent-scope", intentScopeHint},
	{"ocm-aggregate:intent-manifest-over-bound", "DOGFOOD_INTENTS_FILE is larger than 16 paths of 512 bytes can be; list 1-16 repository-relative spec paths, or the one line #no-intent-declared"},
	{"ocm-aggregate:intent-scope-limit", "DOGFOOD_INTENTS_FILE lists more than 16 intent scopes, the bound of the aggregate OCM map (OCM-V0-013); list the 1-16 specs that govern the change, record any other scope as a deferred intent (DCW-V0-371), or write the one line #no-intent-declared; the change itself still binds at any size (DCW-V0-376)"},
	{"ocm-prepare-*:invalid-requirements-section", requirementsSectionHint},
	{"ocm-prepare-*:excluded-artifact-mismatch", uncommittedHint},
	{"ocm-prepare-*:" + bootstrapIntentReason, bootstrapIntentHint},
	{"ocm-prepare-*:too-many-obligations", "the intent on line N of DOGFOOD_INTENTS_FILE, {step} being ocm-prepare-N, derives more obligations than one OCM map admits (256 requirements, OCM-V0-023); {evidence}/{step}.stderr names the count. Remove that intent from DOGFOOD_INTENTS_FILE and cite its BASE requirement rows or decisions in DOGFOOD_CITATIONS instead, or name a smaller spec that holds this change's requirements, then rerun corvint dogfood change {base} (DCW-V0-806)"},
	{"ocm-prepare-*:verification-timeout", "OCM prepare for the intent on line N of DOGFOOD_INTENTS_FILE, {step} being ocm-prepare-N, stopped at its verification deadline (often host load); {evidence}/{step}.stderr keeps its refusal, the maps of the intents before it are kept and nothing was committed; rerun the same corvint dogfood change {base} (or finish-lane) command, which prepares that intent again (DCW-V0-116)"},
	{"coordination-time-impact:unsupported-impact-worktree", uncommittedHint},
	{"local-outcome:record-index-failed", uncommittedHint},
	{"ocm-status-*", "fix the ocm-prepare row with the same number first; if it was produced, the worktree has uncommitted changes (often the prepared sidecar); commit them, then rerun corvint dogfood change {base}"},
	{"cem-status:not-ready", "read verification.issues and policyIssues in {evidence}/cem-status.json: excluded-artifact-mismatch means the sidecar is uncommitted, max-unknown-exceeded means DOGFOOD_CITATIONS does not cite, or keep unknown with a recorded reason, every hunk"},
	{"ocm-links:ocm-link-plan-unavailable", "DOGFOOD_OCM_LINKS must be the path of a TSV file of INTENT<TAB>REQUIREMENT<TAB>HUNKS<TAB>TEST_PATH<TAB>CLAIMS rows, not the rows themselves"},
	{"ocm-links:empty-ocm-link-plan", "DOGFOOD_OCM_LINKS names an empty file; add at least one row, or unset DOGFOOD_OCM_LINKS so every requirement stays unassessed"},
	{"ocm-links:ocm-link-plan-row-limit-exceeded", "DOGFOOD_OCM_LINKS row limit=256 observed={linkRows}; combine cited hunk selectors for the same requirement in one row, or split the change into separately bound commits"},
	{"ocm-links:invalid-ocm-link-plan", "each DOGFOOD_OCM_LINKS row is INTENT<TAB>REQUIREMENT<TAB>HUNK[,HUNK...]<TAB>TEST_PATH<TAB>CLAIM[,CLAIM...], LF-terminated, at most 256 rows, and INTENT is listed in DOGFOOD_INTENTS_FILE"},
	{"ocm-link-*:ocm-map-not-prepared", "the map for this row's intent did not prepare, so the row was not linked; fix that intent's ocm-prepare row above, then rerun corvint dogfood change {base}"},
	{"ocm-link-*", "read {evidence}/{step}.stderr: each linked hunk must be cited in the committed sidecar, and each claim a test case or t.Run name at HEAD containing the exact requirement ID; otherwise delete the DOGFOOD_OCM_LINKS row so the requirement stays unassessed"},
	{"ocm-aggregate:intent-scope-drift", "fix the ocm-prepare or ocm-status row above; otherwise the intents file changed during the run"},
	{"*:unsupported-object-alternates", "the clone borrows objects through .git/objects/info/alternates (git clone --reference or --shared); run git repack -a -d, delete .git/objects/info/alternates and .git/objects/info/commit-graphs, run git commit-graph write --reachable, then rerun corvint dogfood change {base}"},
	{"local-outcome:admitted-path-limit", "a learning trace admits at most 200 changed paths, and this recorder refused instead of writing the complete nonlearning aggregate receipt that corvint dogfood change records over that bound (DCW-V0-374); pass a --corvint-bin whose dogfood-record accepts --aggregate-over-limit and rerun corvint dogfood change {base}"},
	{"local-outcome:aggregate-candidate-limit", "the aggregate local outcome holds at most 4,096 changed paths (DCW-V0-374); bind and seal the change as smaller changes, merge them, and check the merge batch with corvint dogfood batch-check {base} (DCW-V0-039, DCW-V0-040)"},
	{"local-outcome:aggregate-receipt-byte-limit", "the aggregate local outcome receipt is bounded at 4 MiB of paths (DCW-V0-374); bind and seal the change as smaller changes, merge them, and check the merge batch with corvint dogfood batch-check {base} (DCW-V0-039, DCW-V0-040)"},
	{"local-outcome:outcome-input-not-provided", "set DOGFOOD_OUTCOME (passed, failed or blocked) and DOGFOOD_VERIFY_FILE (one verification command per line)"},
	{"local-outcome:verify-file-unavailable", "DOGFOOD_VERIFY_FILE must be the path of a regular file holding one verification command per line, not the commands themselves"},
	{"local-outcome:verify-file-over-bound", "DOGFOOD_VERIFY_FILE is larger than 64 KiB; keep at most 50 verification commands of at most 512 characters, one per line"},
	{"local-outcome:unsupported-verify-syntax", "each DOGFOOD_VERIFY_FILE line is one command of ASCII letters, digits and _./:@=+, - only, with no quotes, ^, $, |, parentheses or other shell syntax, except one quoted -run alternation of test names such as -run 'TestA|TestB', -run '^(TestA|TestB)$' or -run '^TestA$|^TestB$' (DCW-V0-384, DCW-V0-801), and leading NAME='value' or NAME=\"value\" assignments such as GOFLAGS='-count=1 -v' go test ./pkg (DCW-V0-804); write -run TestName instead of -run '^TestName$'; a command that needs quotes or regular-expression syntax, such as a split -run '^Test[A-C]' group, goes in a script committed with the change and is listed as sh path/to/script.sh (DCW-V0-065)"},
	{"local-outcome:record-failed", "read {evidence}/local-outcome.stderr for the cause; DOGFOOD_VERIFY_FILE holds at most 50 commands of at most 512 characters each, so split a longer command into several lines"},
}

// intentScopeHint is the missing-intent-scope fix; a manifest that was read
// is prefixed with the first line it refused (DCW-V0-387).
const intentScopeHint = "DOGFOOD_INTENTS_FILE must be the path of a sorted, LF-terminated file listing 1-16 repository-relative spec paths, or of a file holding the one line #no-intent-declared when no requirements spec governs the change"

const uncommittedHint = "the worktree has uncommitted changes (often the prepared sidecar); commit them, then rerun corvint dogfood change {base}"

// untrackedHint names the untracked paths the recorder counts as dirty
// (DCW-V0-091). The recorder ignores core.excludesFile, so git status can
// hide a path a personal excludes file covers.
const untrackedHint = "the recorder requires a clean tree and counts these untracked paths as dirty: {paths}; it ignores core.excludesFile, so git status may not show them; move them out of the worktree or list them in the repository .gitignore, then rerun corvint dogfood change {base} (DCW-V0-091)"

// maxNamedUntracked bounds how many untracked paths untrackedHint names.
const maxNamedUntracked = 8

// untrackedPaths is the sorted untracked paths, outside .corvint/ and the
// recorder's private artifacts, that the recorder's clean-tree check sees.
// A failed or over-bound read names none, so the generic hint applies.
func (c *change) untrackedPaths() []string {
	output, status, bounded := c.boundedGit("-c", "core.excludesFile=", "ls-files", "--others", "--exclude-standard", "-z")
	if status != 0 || !bounded {
		return nil
	}
	paths := []string{}
	for _, path := range strings.Split(output, "\x00") {
		if path != "" && !strings.HasPrefix(path, ".corvint/") && !trace.IsRecordArtifactPath(path, len(c.base)) {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	return paths
}

// noteUntrackedBuildOutputs names, before anything is prepared, the untracked
// top-level files a `go build ./cmd/NAME` left in the worktree: NAME or
// NAME.exe beside a cmd/NAME directory. The recorder counts them as dirty, so
// the run would end in record-index-failed (DCW-V0-386).
func (c *change) noteUntrackedBuildOutputs() {
	outputs := []string{}
	for _, path := range c.untrackedPaths() {
		name := strings.TrimSuffix(path, ".exe")
		if name == "" || strings.Contains(name, "/") {
			continue
		}
		if info, err := os.Stat(c.path("cmd/" + name)); err == nil && info.IsDir() {
			outputs = append(outputs, path)
		}
	}
	if len(outputs) == 0 {
		return
	}
	named := strings.Join(outputs[:min(len(outputs), maxNamedUntracked)], ", ")
	if len(outputs) > maxNamedUntracked {
		named += fmt.Sprintf(" (+%d more)", len(outputs)-maxNamedUntracked)
	}
	c.say("dogfood-change: NOTE untracked-build-output %s\n  go build ./cmd/NAME writes NAME into the worktree and the recorder counts an untracked file as dirty; delete it and build into scratch instead: go build -o \"$TMPDIR/NAME\" ./cmd/NAME (DCW-V0-386)\n", named)
}

// aggregateBoundReasons are the multi-intent aggregate bound refusals
// (OCM-V0-013, OCM-V0-021).
var aggregateBoundReasons = map[string]bool{"map-too-large": true, "too-many-claims": true, "too-many-obligations": true}

// maxNamedSeals bounds how many merged sealed changes the stacked hint names.
const maxNamedSeals = 4

// stackedPredecessorHint names the stacked-predecessor cause of an aggregate
// bound refusal: BASE..HEAD also carries merged sealed changes, whose hunks
// and governing specs count toward this change's obligations. The route is to
// bind against the sealed predecessor's seal commit, so only this lane's hunks
// bind and batch-check verifies both (DCW-V0-113).
func (c *change) stackedPredecessorHint() string {
	named := []string{}
	for _, path := range c.mergedSeals[:min(len(c.mergedSeals), maxNamedSeals)] {
		bind := sealedCEMPath.FindStringSubmatch(path)[1]
		seal := c.gitValue("log", "-1", "--format=%H", "--diff-filter=A", c.target, "--", path)
		if seal == "" {
			seal = "UNKNOWN"
		}
		named = append(named, "bind="+bind+" seal="+seal)
	}
	if len(c.mergedSeals) > maxNamedSeals {
		named = append(named, fmt.Sprintf("(+%d more)", len(c.mergedSeals)-maxNamedSeals))
	}
	return fmt.Sprintf("stacked-predecessor: BASE %s..HEAD also merges %d sealed change(s) (%s), whose hunks and specs count toward this change's bound; "+
		"a lane stacked on one sealed predecessor binds against that predecessor's seal commit instead: rerun dogfood-change, dogfood-check and dogfood-seal with BASE=<seal>, list only this lane's specs in DOGFOOD_INTENTS_FILE, "+
		"and check the merge batch with corvint dogfood batch-check, which verifies each sealed change against its own base; otherwise narrow the intents (docs/DOGFOOD.md \"Stacking on a sealed predecessor\", DCW-V0-113)",
		c.base, len(c.mergedSeals), strings.Join(named, ", "))
}

// aggregateBoundPattern reads the observed total, the bound and the per-scope
// shares from an OCM aggregate bound refusal (OCM-V0-021).
var aggregateBoundPattern = regexp.MustCompile(`: observed ([0-9]+), bound ([0-9]+); per scope((?:,? [0-9]+=[0-9]+)+)`)

// maxAggregateDetail bounds the inline refusal detail.
const maxAggregateDetail = 1024

// aggregateBoundShares reads the ocm-aggregate refusal's message, bounded, and
// the bound and per-scope shares it names when they cover every declared
// intent in manifest order; shares is nil otherwise.
func (c *change) aggregateBoundShares() (detail string, bound int, shares []int) {
	var refusal struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(bytes.TrimSpace(readFile(c.evidence+"/ocm-aggregate.stderr")), &refusal) != nil || refusal.Error == "" {
		return "", 0, nil
	}
	detail = refusal.Error
	if len(detail) > maxAggregateDetail {
		detail = detail[:maxAggregateDetail] + "..."
	}
	match := aggregateBoundPattern.FindStringSubmatch(refusal.Error)
	if match == nil {
		return detail, 0, nil
	}
	bound, _ = strconv.Atoi(match[2])
	for index, pair := range strings.Split(strings.TrimPrefix(match[3], " "), ", ") {
		line, count, _ := strings.Cut(pair, "=")
		value, err := strconv.Atoi(count)
		if line != strconv.Itoa(index+1) || err != nil {
			return detail, 0, nil
		}
		shares = append(shares, value)
	}
	if len(shares) != len(c.intents) || len(shares) > maxAggregateScopes {
		return detail, 0, nil
	}
	return detail, bound, shares
}

// maxAggregateScopes is the manifest's scope bound (OCM-V0-013), which keeps
// the exhaustive subset search at 2^16 sets.
const maxAggregateScopes = 16

// aggregateFitHint names the intent subset whose shares fit the bound with the
// largest total, and the scopes to bind as a separate staged change
// (DCW-V0-346). It searches every subset of the at most 16 scopes. The omitted
// scopes stay named, and nothing is dropped from this run: the refusal stands.
func (c *change) aggregateFitHint() string {
	_, bound, shares := c.aggregateBoundShares()
	if shares == nil {
		return "the per-scope shares are NOT_OBSERVED (see " + c.evidence + "/ocm-aggregate.stderr); list fewer intent scopes in DOGFOOD_INTENTS_FILE and bind the rest as a separate staged change (OCM-V0-024)"
	}
	return fitHint(c.intents, bound, shares)
}

// fitHint names, for intents in manifest order with their shares of one
// aggregate bound, the subset that fits with the largest total and the rest.
func fitHint(intents []string, bound int, shares []int) string {
	best, bestTotal := 0, -1
	for set := 1; set < 1<<len(shares); set++ {
		total := 0
		for index, share := range shares {
			if set>>index%2 == 1 {
				total += share
			}
		}
		if total <= bound && total > bestTotal {
			best, bestTotal = set, total
		}
	}
	named := func(keep bool) string {
		parts := []string{}
		for index, path := range intents {
			if (best>>index%2 == 1) == keep {
				parts = append(parts, fmt.Sprintf("%s=%d", path, shares[index]))
			}
		}
		return strings.Join(parts, " ")
	}
	if bestTotal < 0 {
		return fmt.Sprintf("no intent scope fits the bound %d alone (%s); split the governing spec, putting new requirements in a companion spec, and list that one instead (OCM-V0-024, DCW-V0-346)", bound, named(false))
	}
	return fmt.Sprintf("keep in DOGFOOD_INTENTS_FILE: %s (total %d of bound %d); bind separately as a staged change: %s (OCM-V0-024, DCW-V0-346)", named(true), bestTotal, bound, named(false))
}

// citeSpanUnedited is the cem cite refusal phrase for a span whose base
// lines no hunk edits but whose text recurs in the changed file.
const citeSpanUnedited = "are edited by no hunk"

// citeSpanUnalignedHint is the DCW-V0-063 fix line for that refusal.
const citeSpanUnalignedHint = "plan row {row} cites BASE lines that no hunk edits (at most hunk context), which git's line alignment leaves unchanged, but their text occurs more than once in the changed file (another copy exists or the change adds one), and the stability check matches text, not line positions; cite a span whose text occurs once in the changed file, for example by widening it to a neighbouring distinct line (DCW-V0-063); plan spans are BASE line numbers, counted in the file as it is at BASE {base}, not at HEAD (DCW-V0-845)"

// staleBindingHint replaces the citation-plan-map-mismatch fix line when the
// refusal came from a binding recorded against an earlier map (DCW-V0-066).
const staleBindingHint = "stale binding: {evidence}/citation-plan-binding was recorded when these same plan bytes were cited against an earlier map, and {staleBinding}; if the plan was kept from that map, rewrite it from the current .corvint/change.cem.json; if it was rewritten for the current map and only repeats the earlier bytes, remove {evidence}/citation-plan-binding and rerun (DCW-V0-066)"

// failedMap is the map a failing CEM step read: the cite command's own map,
// else the change's sidecar.
func (c *change) failedMap() string {
	if c.citationFailedMap != "" {
		return c.citationFailedMap
	}
	return ".corvint/change.cem.json"
}

// rowValues renders the refused row's quoted fields for a hint, or nothing
// when the row was not a citation row.
func (c *change) rowValues() string {
	if c.citationFailedValues == "" {
		return ""
	}
	return " (" + c.citationFailedValues + ")"
}

func (c *change) fixHint(row step) string {
	value := row.name + ":" + row.reason
	if value == "cem-cite:citation-plan-map-mismatch" && strings.Contains(c.mismatchDetail, staleHunkIDMarker) {
		return staleHunkIDHint(c.base, c.options.Citations)
	}
	if value == "cem-cite:citation-plan-map-mismatch" && c.staleBinding != "" {
		return strings.NewReplacer("{evidence}", c.evidence, "{staleBinding}", c.staleBinding).Replace(staleBindingHint)
	}
	if value == "local-outcome:record-index-failed" {
		if paths := c.untrackedPaths(); len(paths) > 0 {
			named := strings.Join(paths[:min(len(paths), maxNamedUntracked)], ", ")
			if len(paths) > maxNamedUntracked {
				named += fmt.Sprintf(" (+%d more)", len(paths)-maxNamedUntracked)
			}
			return strings.NewReplacer("{paths}", named, "{base}", c.base).Replace(untrackedHint)
		}
	}
	if row.name == "ocm-aggregate" && aggregateBoundReasons[row.reason] {
		if len(c.mergedSeals) > 0 {
			return c.stackedPredecessorHint()
		}
		return c.aggregateFitHint()
	}
	if value == "ocm-aggregate:missing-intent-scope" && c.manifestIssue != "" {
		return "DOGFOOD_INTENTS_FILE " + c.options.IntentsFile + ": " + c.manifestIssue + "; " + intentScopeHint
	}
	if value == "local-outcome:unsupported-verify-syntax" {
		if fix := quotedRunFix(c.options.VerifyFile); fix != "" {
			return "DOGFOOD_VERIFY_FILE " + fix
		}
	}
	if value == "ocm-aggregate:intent-scope-drift" {
		// A refused prepare stops the aggregate before it runs, so the
		// refusal is the cause and nothing drifted (DCW-V0-806).
		for _, prepared := range c.rows {
			if strings.HasPrefix(prepared.name, "ocm-prepare-") && c.rowFailing(prepared) {
				return fmt.Sprintf("%s refused %s first, so the aggregate did not run and no intent drifted; fix that row (its fix line is above), then rerun corvint dogfood change %s", prepared.name, prepared.reason, c.base)
			}
		}
	}
	if strings.HasPrefix(row.name, "ocm-prepare-") && row.reason == "invalid-requirements-section" {
		return c.requirementsSectionHint(row.name)
	}
	if row.reason == "repository-object-unavailable" {
		return objectUnavailableHint(row.name, c.evidence, c.base)
	}
	if value == "local-outcome:aggregate-schema-invalid" {
		return c.aggregateSchemaHint()
	}
	if value == "cem-cite:cite-span-not-stable" && c.citeSpanUnaligned {
		return strings.NewReplacer("{row}", strconv.Itoa(c.citationFailedRow), "{base}", c.base).Replace(citeSpanUnalignedHint)
	}
	for _, entry := range fixHints {
		prefix, suffix, star := strings.Cut(entry.pattern, "*")
		matched := value == entry.pattern
		if star {
			matched = len(value) >= len(prefix)+len(suffix) && strings.HasPrefix(value, prefix) && strings.HasSuffix(value, suffix)
		}
		if matched {
			return strings.NewReplacer("{evidence}", c.evidence, "{base}", c.base, "{step}", row.name, "{map}", c.path(c.failedMap()), "{row}", strconv.Itoa(c.citationFailedRow), "{rowValues}", c.rowValues(), "{citationRows}", strconv.Itoa(c.citationCount), "{linkRows}", strconv.Itoa(c.linkObservedRows), "{uniformHunks}", strconv.Itoa(c.uniformHunks), "{uniformSpan}", c.uniformSpan, "{recitedHunk}", c.recitedHunk, "{recitedDigest}", c.recitedDigest, "{relations}", strings.Join(RelationChoices(), ", ")).Replace(entry.hint)
		}
	}
	return ""
}

// maxQuotedRefusal bounds the refusal message objectUnavailableHint quotes.
const maxQuotedRefusal = 240

// objectUnavailableHint is the repository-object-unavailable fix (DCW-V0-940,
// V1-2445): the step's own refusal message from evidence/step.stderr, which
// names the object, then the fetch remedy. The message is the stderr's JSON
// error, else its first line, bounded and stripped of control characters.
func objectUnavailableHint(step, evidence, base string) string {
	stderr := bytes.TrimSpace(readFile(evidence + "/" + step + ".stderr"))
	var refusal struct {
		Error string `json:"error"`
	}
	message := ""
	if json.Unmarshal(stderr, &refusal) == nil {
		message = refusal.Error
	} else {
		line, _, _ := bytes.Cut(stderr, []byte("\n"))
		message = string(line)
	}
	message = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(message))
	if len(message) > maxQuotedRefusal {
		message = strings.ToValidUTF8(message[:maxQuotedRefusal], "") + "..."
	}
	named := "its refusal in " + evidence + "/" + step + ".stderr names the missing object"
	if message != "" {
		named = strconv.Quote(message) + " (" + evidence + "/" + step + ".stderr)"
	}
	return step + " could not read a Git object it needs: " + named + "; fetch the missing history into this clone (git fetch origin, and git fetch origin " + base + " when BASE is not local; git fetch --unshallow for a shallow clone; a plain clone of a lane checkout also needs that checkout's origin fetched), then rerun corvint dogfood change " + base + " (DCW-V0-940)"
}

// requirementsSectionHint is the ocm-prepare invalid-requirements-section fix
// for an intent present at BASE; requirementsSectionBootstrapHint is the fix for
// a bootstrap intent this change adds (ASD-V0-092, V1-3254; DCW-V0-449).
const (
	requirementsSectionHint          = `the intent on line N of DOGFOOD_INTENTS_FILE, {step} being ocm-prepare-N, must be a spec Markdown file with exactly one "## Requirements" heading outside code fences; give it one, or name instead the spec that holds this change's requirements, then rerun corvint dogfood change {base}`
	requirementsSectionBootstrapHint = `%s on line %s of DOGFOOD_INTENTS_FILE is absent at BASE, a bootstrap intent this change adds (such as a new companion spec, DCW-V0-449); it is read at the change's commit and must hold exactly one "## Requirements" heading outside code fences: add the heading to that new spec and commit it, keep each of its hunks as no row or an unknown row in DOGFOOD_CITATIONS, then rerun corvint dogfood change {base}`
)

// requirementsSectionHint names the bootstrap route when the intent of the
// failed ocm-prepare-N row is absent at BASE, and the BASE fix otherwise.
func (c *change) requirementsSectionHint(name string) string {
	number := strings.TrimPrefix(name, "ocm-prepare-")
	index, err := strconv.Atoi(number)
	if err != nil || index < 1 || index > len(c.intents) || c.gitSucceeds("cat-file", "-e", c.base+":"+c.intents[index-1]) {
		return strings.NewReplacer("{step}", name, "{base}", c.base).Replace(requirementsSectionHint)
	}
	return strings.ReplaceAll(fmt.Sprintf(requirementsSectionBootstrapHint, strconv.Quote(c.intents[index-1]), number), "{base}", c.base)
}

// aggregateSchemaHint names the field and recovery the recorder wrote for an
// aggregate-schema-invalid local outcome (ASD-V0-089), from its stderr.
func (c *change) aggregateSchemaHint() string {
	var refusal struct {
		Error string `json:"error"`
	}
	detail := ""
	if json.Unmarshal(bytes.TrimSpace([]byte(readFile(c.evidence+"/local-outcome.stderr"))), &refusal) == nil {
		_, detail, _ = strings.Cut(refusal.Error, "aggregate-schema-invalid ")
	}
	if detail == "" {
		detail = "field: not named by this recorder; fix: read " + c.evidence + "/local-outcome.stderr, remove leading or trailing whitespace from each DOGFOOD_VERIFY_FILE line and rerun corvint dogfood change BASE with the lane-built --corvint-bin, which names the field"
	}
	return strings.ReplaceAll(detail, "BASE", c.base)
}

// noteAgentReceipts reports, without blocking, an agent pre-change receipt that
// is absent, over the 4 MiB bound, malformed, from another tool, or was not
// written against the base tree, and how a base-tree receipt matches its
// dogfood prechange binding (DCW-V0-031, DCW-V0-032, DCW-V0-881).
func (c *change) noteAgentReceipts() {
	baseTree := c.gitValue("rev-parse", c.base+"^{tree}")
	for _, name := range []string{"prechange-query", "prechange-impact"} {
		c.noteAgentReceipt(name, baseTree)
	}
	// A context receipt is orientation and never stands in for the query
	// receipt (DCW-V0-881).
	if !isRegular(c.evidence+"/prechange-query.json") && isRegular(c.evidence+"/prechange-context.json") {
		c.say("dogfood-change: NOTE prechange-context orientation-only satisfies-no-slot\n")
	}
}

// receiptTools names the tool each agent receipt slot must come from (DCW-V0-881).
var receiptTools = map[string]string{"prechange-query": "query", "prechange-impact": "impact"}

func (c *change) noteAgentReceipt(name, baseTree string) {
	path := c.evidence + "/" + name + ".json"
	if !isRegular(path) {
		c.say("dogfood-change: NOTE %s NOT_OBSERVED agent-receipt-absent\n", name)
		return
	}
	var receipt struct {
		Tool    *string `json:"tool"`
		Context struct {
			Revision string `json:"revision"`
		} `json:"context"`
	}
	data, err := readPrefix(path, maxPlanBytes+1)
	switch {
	case err == nil && len(data) > maxPlanBytes:
		c.say("dogfood-change: NOTE %s NOT_OBSERVED agent-receipt-over-bound\n", name)
		return
	case err != nil:
		c.say("dogfood-change: NOTE %s NOT_OBSERVED agent-receipt-unreadable\n", name)
		return
	case json.Unmarshal(data, &receipt) != nil:
		c.say("dogfood-change: NOTE %s NOT_OBSERVED agent-receipt-malformed\n", name)
		return
	}
	if receipt.Tool != nil && *receipt.Tool != receiptTools[name] {
		c.say("dogfood-change: NOTE %s NOT_OBSERVED agent-receipt-unsupported tool=%s\n", name, strconv.Quote(*receipt.Tool))
		return
	}
	switch tree := receipt.Context.Revision; tree {
	case baseTree:
		c.noteBoundReceipt(name, path)
	case "":
		c.say("dogfood-change: NOTE %s NOT_OBSERVED agent-receipt-tree-unknown\n", name)
	default:
		c.say("dogfood-change: NOTE %s STALE agent-receipt-not-base-tree tree=%s base-tree=%s\n", name, tree, baseTree)
	}
}

// reportFailures notes an accepted impact abstention, then lists each failing
// row with its fix and exits 1, or exits 0 when the report is complete.
func (c *change) reportFailures() int {
	for _, row := range c.rows {
		if row.name == "coordination-time-impact" && (impactAbstentions[row.reason] || row.reason == impactDeadlineReason) {
			c.say("dogfood-change: NOTE coordination-time-impact NOT_PRODUCED %s\n", row.reason)
		}
	}
	if c.queryAbstentionSHA != "" {
		c.say("dogfood-change: NOTE %s NOT_PRODUCED %s\n", queryStep, queryAbstentionReason)
	}
	if c.guardHunks > 0 {
		c.say("dogfood-change: NOTE %s NOT_PRODUCED %s hunks=%d: to cite an observed guarding test, run corvint cem verify --map .corvint/change.cem.json --expected-base %s --target %s --artifacts %s --guarding-tests and add its guardingTests.plan rows to the citation plan; it runs the change's tests unconfined for up to 1200 seconds (docs/DOGFOOD.md section 4)\n",
			guardingTestsStepName, guardingTestsNotRun, c.guardHunks, c.base, c.target, shellWord(stableArtifactRoot(c.root)))
	}
	c.noteAgentReceipts()
	if c.complete() {
		// A complete bind pass closes with one PASS line (DCW-V0-342), so a
		// silent exit is never mistaken for an interrupted run.
		c.say("dogfood-change: PASS complete base=%s target=%s report=%s\n", c.base, c.target, c.path(".corvint/dogfood-report.json"))
		return 0
	}
	c.say("dogfood-change: FAIL not-complete\n")
	for _, row := range c.rows {
		if !c.rowFailing(row) {
			continue
		}
		c.say("  %s: %s\n", row.name, row.reason)
		if row.name == "ocm-aggregate" && aggregateBoundReasons[row.reason] {
			if detail, _, _ := c.aggregateBoundShares(); detail != "" {
				c.say("    detail: %s\n", detail)
			}
		}
		if hint := c.fixHint(row); hint != "" {
			c.say("    fix: %s\n", hint)
		}
		if row.name == "cem-cite" && len(c.citationRefusals) > 0 {
			c.say("    refused plan rows=%d (every cite row checked in this pass; fix them all before rerunning, DCW-V0-114):\n", len(c.citationRefusals))
			for _, refusal := range c.citationRefusals[:min(len(c.citationRefusals), maxNamedRefusals)] {
				c.say("      %s\n", refusal)
			}
			if len(c.citationRefusals) > maxNamedRefusals {
				c.say("      (+%d more)\n", len(c.citationRefusals)-maxNamedRefusals)
			}
		}
	}
	// cem cite only adds evidence and prepare resumes a matching map, so a
	// corrected plan joins the earlier plan's citations (DCW-V0-019).
	if c.citedOver {
		c.say("  cem-cite: the plan was added to citations the map already carried and never replaces them; to correct one hunk's earlier citations, rerun with the DOGFOOD_CITATION_REPLACE entry the refusal names, or delete .corvint/change.cem.json and rerun corvint dogfood change %s (docs/DOGFOOD.md step 4)\n", c.base)
	}
	query := readFile(c.evidence + "/coordination-time-query.stderr")
	// The authority-start refusal is selected by the task wording, not by the
	// change; a malformed store refuses any wording and names no profile.
	for _, line := range textLines(query) {
		if strings.HasPrefix(line, `{"code": "unsupported-query-trace-state", "error": "native Go authority-start query `) {
			c.say("  coordination-time-query: DOGFOOD_TASK wording selected the authority-start profile, which refuses a present local trace store; keep this receipt and the task (docs/DOGFOOD.md section 1)\n")
			break
		}
	}
	// Amending or rebasing after a recorded pass strands that trace, as does
	// another lane's run in this worktree; query and the recorder then refuse
	// every later run with this message.
	stranded := []byte(`"error": "local trace store contains unreachable revision: `)
	if bytes.Contains(query, stranded) || bytes.Contains(readFile(c.evidence+"/local-outcome.stderr"), stranded) {
		c.say("  local trace store: a recorded trace names a commit no longer reachable from HEAD; restore that commit as an ancestor, or retire the trace with corvint migrate-traces --dry-run then --apply --plan-digest DIGEST (LTPM-V0-012); add new commits instead of amending or rebasing (docs/DOGFOOD.md \"Daily adopter path\"); the store belongs to this worktree, so run each lane in its own worktree (git worktree add) or clone rather than switching branches here (DCW-V0-044)\n")
	}
	c.say("  full report: %s\n", c.path(".corvint/dogfood-report.json"))
	return 1
}

const AggregateReportProfile = "corvint-dogfood-change/1"

type AggregateEnrollment struct {
	Session       string `json:"session"`
	Generation    string `json:"generation"`
	PlanDigest    string `json:"planDigest"`
	BindingSHA256 string `json:"bindingSha256"`
}
type AggregateReportStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}
type AggregateReportAnchor struct {
	State     string  `json:"state"`
	MergeBase *string `json:"mergeBase"`
}
type AggregateReportPolicy struct {
	BootstrapUnknown             int `json:"bootstrapUnknown"`
	MaximumUnknownAfterBootstrap int `json:"maximumUnknownAfterBootstrap"`
}
type AggregateCheckResult struct {
	BaseVerifierSHA256           string  `json:"baseVerifierSha256"`
	TreeVerifierSHA256           string  `json:"treeVerifierSha256"`
	OverrideVerifierSHA256       *string `json:"overrideVerifierSha256"`
	BootstrapUnknown             int     `json:"bootstrapUnknown"`
	MaximumUnknownAfterBootstrap int     `json:"maximumUnknownAfterBootstrap"`
	OutputsAgree                 bool    `json:"outputsAgree"`
}

// AggregateReport intentionally has no Complete member. The native /1 reader
// checks a closed structural representation, including the real enrollment.
type AggregateReport struct {
	Profile                         string                `json:"profile"`
	Base                            string                `json:"base"`
	Target                          string                `json:"target"`
	CompletionState                 string                `json:"completionState"`
	Steps                           []AggregateReportStep `json:"steps"`
	OCMStatus                       json.RawMessage       `json:"ocmStatus"`
	LocalOutcomeEvidenceSHA256      string                `json:"localOutcomeEvidenceSha256"`
	ContextAbstentionEvidenceSHA256 *string               `json:"contextAbstentionEvidenceSha256"`
	QueryAbstentionEvidenceSHA256   *string               `json:"queryAbstentionEvidenceSha256"`
	Anchor                          AggregateReportAnchor `json:"anchor"`
	OCMLinkPlan                     json.RawMessage       `json:"ocmLinkPlan"`
	DogfoodPolicy                   AggregateReportPolicy `json:"dogfoodPolicy"`
	PacketCoverage                  []json.RawMessage     `json:"packetCoverage"`
	DogfoodCheck                    *AggregateCheckResult `json:"dogfoodCheck"`
	Enrollment                      AggregateEnrollment   `json:"enrollment"`
	LocalOutcomeProfile             string                `json:"localOutcomeProfile"`
}

var aggregateDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var aggregateKeyPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func ParseAggregateReport(raw []byte) (AggregateReport, error) {
	var value AggregateReport
	// ASD-V0-089: each refusal names the report member it found malformed.
	bad := tracerecordrepo.AggregateSchemaReportError
	if len(raw) == 0 || len(raw) > 4<<20 {
		return value, bad("size")
	}
	parsed, err := wire.Parse(raw)
	if err != nil {
		return value, bad("json")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&value); err != nil {
		return value, bad("members")
	}
	// Canonical comparison requires every member and rejects null scalar values;
	// incoming whitespace and object key order are not semantic constraints.
	encoded, err := aggregateWorkerJSON(value)
	if err != nil || !bytes.Equal(wire.CanonicalValue(parsed), bytes.TrimSuffix(encoded, []byte{'\n'})) {
		return value, bad("canonical form")
	}
	if value.Profile != AggregateReportProfile || value.LocalOutcomeProfile != tracerecordrepo.AggregateOutcomeProfile || !fullRevision.MatchString(value.Base) || !fullRevision.MatchString(value.Target) || !aggregateDigestPattern.MatchString(value.LocalOutcomeEvidenceSHA256) || !aggregateKeyPattern.MatchString(value.Enrollment.Session) || !aggregateKeyPattern.MatchString(value.Enrollment.PlanDigest) || !aggregateDigestPattern.MatchString(value.Enrollment.BindingSHA256) || len(value.Enrollment.Generation) != 68 || !strings.HasPrefix(value.Enrollment.Generation, value.Enrollment.PlanDigest+"-") {
		return value, bad("profile, base, target, localOutcomeEvidenceSha256 or enrollment")
	}
	if value.CompletionState != "complete" && value.CompletionState != "incomplete" {
		return value, bad("completionState")
	}
	if len(value.Steps) < 1 || len(value.Steps) > 256 || value.DogfoodPolicy.BootstrapUnknown < 0 || value.DogfoodPolicy.MaximumUnknownAfterBootstrap != 0 {
		return value, bad("steps or dogfoodPolicy")
	}
	outcomeRows := 0
	rows := &change{}
	for _, row := range value.Steps {
		if row.Status != "PRODUCED" && row.Status != "NOT_PRODUCED" {
			return value, bad("steps status")
		}
		if row.Name == "local-outcome" {
			outcomeRows++
			if row.Status != "PRODUCED" || row.Reason != "none" {
				return value, bad("local-outcome step")
			}
		}
		rows.rows = append(rows.rows, step{row.Name, row.Status, row.Reason})
	}
	if value.QueryAbstentionEvidenceSHA256 != nil {
		if !aggregateDigestPattern.MatchString(*value.QueryAbstentionEvidenceSHA256) {
			return value, bad("queryAbstentionEvidenceSha256")
		}
		rows.queryAbstentionSHA = *value.QueryAbstentionEvidenceSHA256
	}
	if value.ContextAbstentionEvidenceSHA256 != nil && !aggregateDigestPattern.MatchString(*value.ContextAbstentionEvidenceSHA256) {
		return value, bad("contextAbstentionEvidenceSha256")
	}
	if outcomeRows != 1 || (value.CompletionState == "complete" && !rows.complete()) {
		return value, bad("steps completion")
	}
	if value.Anchor.State == "OBSERVED" {
		if value.Anchor.MergeBase == nil || !fullRevision.MatchString(*value.Anchor.MergeBase) {
			return value, bad("anchor.mergeBase")
		}
	} else if value.Anchor.State != "NOT_OBSERVED" || value.Anchor.MergeBase != nil {
		return value, bad("anchor.state")
	}
	if value.DogfoodCheck != nil {
		check := value.DogfoodCheck
		if !aggregateDigestPattern.MatchString(check.BaseVerifierSHA256) || !aggregateDigestPattern.MatchString(check.TreeVerifierSHA256) || (check.OverrideVerifierSHA256 != nil && !aggregateDigestPattern.MatchString(*check.OverrideVerifierSHA256)) || check.BootstrapUnknown < 0 || check.MaximumUnknownAfterBootstrap != 0 {
			return value, bad("dogfoodCheck")
		}
	}
	return value, nil
}

// PrepareAggregateReport retains the original governed fields and changes only
// the witnessed local-outcome row and the explicit /1 enrollment discriminator.
func PrepareAggregateReport(old []byte, enrollment AggregateEnrollment, receipt []byte) ([]byte, error) {
	outcome, err := tracerecordrepo.ParseAggregateOutcome(receipt)
	if err != nil {
		return nil, err
	}
	binding, err := tracerecordrepo.AggregateBinding(outcome)
	if err != nil || binding != enrollment.BindingSHA256 || outcome.Task != "Local completion "+enrollment.PlanDigest || outcome.Outcome != "passed" {
		return nil, errors.New("aggregate-binding-drift")
	}
	if len(old) > 4<<20 {
		return nil, errors.New("aggregate-schema-invalid")
	}
	if _, err := wire.Parse(old); err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(old, &object); err != nil {
		return nil, err
	}
	var base, target string
	if json.Unmarshal(object["base"], &base) != nil || json.Unmarshal(object["target"], &target) != nil || base != outcome.Base || target != outcome.Target {
		return nil, errors.New("aggregate-binding-drift")
	}
	var profile string
	if json.Unmarshal(object["profile"], &profile) != nil || profile != "corvint-dogfood-change/0" {
		return nil, errors.New("aggregate-schema-invalid")
	}
	if _, ok := object["complete"]; !ok {
		return nil, errors.New("aggregate-schema-invalid")
	}
	if _, ok := object["completionState"]; ok {
		return nil, errors.New("aggregate-schema-invalid")
	}
	if _, ok := object["enrollment"]; ok {
		return nil, errors.New("aggregate-schema-invalid")
	}
	if _, ok := object["localOutcomeProfile"]; ok {
		return nil, errors.New("aggregate-schema-invalid")
	}
	put := func(key string, value any) { object[key], _ = json.Marshal(value) }
	delete(object, "complete")
	put("profile", AggregateReportProfile)
	put("enrollment", enrollment)
	put("localOutcomeProfile", tracerecordrepo.AggregateOutcomeProfile)
	put("localOutcomeEvidenceSha256", "sha256:"+sha256Hex(receipt))
	put("dogfoodCheck", nil)
	var rows []AggregateReportStep
	if err := json.Unmarshal(object["steps"], &rows); err != nil {
		return nil, err
	}
	changed := 0
	for i, row := range rows {
		if row.Name == "local-outcome" {
			if row.Status != "NOT_PRODUCED" || row.Reason != "admitted-path-limit" {
				return nil, errors.New("aggregate-legacy-failure-unverified")
			}
			rows[i] = AggregateReportStep{"local-outcome", "PRODUCED", "none"}
			changed++
		}
	}
	if changed != 1 {
		return nil, errors.New("aggregate-legacy-failure-unverified")
	}
	put("steps", rows)
	put("completionState", "incomplete")
	raw, err := aggregateWorkerJSON(object)
	if err != nil {
		return nil, err
	}
	value, err := ParseAggregateReport(raw)
	if err != nil {
		return nil, err
	}
	completion := &change{}
	for _, row := range value.Steps {
		completion.rows = append(completion.rows, step{row.Name, row.Status, row.Reason})
	}
	if value.QueryAbstentionEvidenceSHA256 != nil {
		completion.queryAbstentionSHA = *value.QueryAbstentionEvidenceSHA256
	}
	if completion.complete() {
		value.CompletionState = "complete"
	}
	return EncodeAggregateReport(value)
}
func EncodeAggregateReport(value AggregateReport) ([]byte, error) {
	raw, err := aggregateWorkerJSON(value)
	if err != nil {
		return nil, err
	}
	if _, err = ParseAggregateReport(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func aggregateReadFile(name string, bound int) ([]byte, error) {
	if err := dogfoodoperation.CheckParents(name); err != nil {
		return nil, err
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > int64(bound) {
		return nil, errors.New("aggregate-prior-evidence-drift")
	}
	raw, err := io.ReadAll(io.LimitReader(file, int64(bound)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > bound {
		return nil, errors.New("aggregate-prior-evidence-drift")
	}
	return raw, nil
}

// AggregateMarkerPresent is a read-only downgrade guard, not an authority
// validator. Positive Check/Seal authority still comes from the loaded owner.
func AggregateMarkerPresent(root, gitDir string) (bool, error) {
	name, err := aggregateMarker(root, gitDir, "")
	return name != "", err
}

// aggregateMarker returns the file that makes the aggregate marker present, or
// "" when none does. With a target, a parseable aggregate receipt bound to
// another target is the stale evidence of an earlier commit's run, not this
// change's enrollment, so it is skipped (DCW-V0-448).
func aggregateMarker(root, gitDir, target string) (string, error) {
	// Repository spelling aliases (such as /tmp and /private/tmp) are already
	// supported. Resolve the root, then reject symlinks below that boundary.
	canonicalRoot, rootErr := filepath.EvalSymlinks(root)
	if rootErr != nil {
		return "", rootErr
	}
	root = canonicalRoot
	owner, err := aggregateReadFile(filepath.Join(gitDir, "corvint", "local-completion", "owner"), 65)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err == nil {
		key := strings.TrimSuffix(string(owner), "\n")
		if !aggregateKeyPattern.MatchString(key) {
			return "", errors.New("aggregate-enrollment-required")
		}
		stateFile := filepath.Join(gitDir, "corvint", "local-completion", key, "state.json")
		raw, readErr := aggregateReadFile(stateFile, 256<<10)
		if readErr != nil {
			return "", readErr
		}
		if _, err := wire.Parse(raw); err != nil {
			return "", err
		}
		var state map[string]json.RawMessage
		if err := json.Unmarshal(raw, &state); err != nil {
			return "", err
		}
		if _, ok := state["aggregateOutcome"]; ok {
			return stateFile, nil
		}
	}
	// An unenrolled change over the learning bound records the same aggregate
	// receipt (DCW-V0-374); the plain change report that binds its digest makes
	// it that run's evidence, not an enrollment marker, so the change can rerun.
	changeReportBinds := ""
	for i, name := range []string{filepath.Join(root, ".corvint", "dogfood-report.json"), filepath.Join(gitDir, "corvint", "local-outcome.json")} {
		raw, err := aggregateReadFile(name, 4<<20)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(raw, &object) != nil {
			continue
		} // Legacy failed output is not an aggregate marker.
		var profile string
		_ = json.Unmarshal(object["profile"], &profile)
		if i == 0 && profile == "corvint-dogfood-change/0" {
			_ = json.Unmarshal(object["localOutcomeEvidenceSha256"], &changeReportBinds)
		}
		if profile == tracerecordrepo.AggregateOutcomeProfile && changeReportBinds != "" && changeReportBinds == "sha256:"+sha256Hex(raw) {
			continue
		}
		if i == 1 && target != "" && profile == tracerecordrepo.AggregateOutcomeProfile {
			if receipt, err := tracerecordrepo.ParseAggregateOutcome(raw); err == nil && receipt.Target != target {
				continue
			}
		}
		if profile == AggregateReportProfile || profile == tracerecordrepo.AggregateOutcomeProfile {
			return name, nil
		}
		if _, ok := object["localOutcomeProfile"]; ok {
			return name, nil
		}
	}
	return "", nil
}

// requireNoAggregateMarker refuses, before any step runs, a change whose
// checkout holds an aggregate marker for HEAD, naming the marker file and its
// recovery (DCW-V0-448); a receipt bound to another target is skipped.
func (c *change) requireNoAggregateMarker() {
	marker, err := aggregateMarker(c.root, c.gitDir, c.target)
	if err == nil && marker == "" {
		return
	}
	c.say("dogfood-change: REFUSE aggregate-enrollment-required\n")
	if err != nil {
		c.say("  the aggregate marker under %s cannot be read: %v\n", shellWord(filepath.Join(c.gitDir, "corvint")), err)
		c.say("  fix: restore or remove the unreadable file named above, then rerun the same command (DCW-V0-448)\n")
		c.exit(2)
	}
	c.say("  marker: %s\n", shellWord(marker))
	if filepath.Base(marker) == "state.json" {
		// The enrollment owner names this state; moving it aside would leave
		// the owner pointing at missing state, so the recovery is the
		// enrollment's own keyed finish (DCW-V0-448).
		key := filepath.Base(filepath.Dir(marker))
		c.say("  this checkout has an enrolled aggregate change whose state records an aggregate outcome; do not move its state aside, which leaves the enrollment owner naming missing state\n")
		c.say("  fix: resume it with corvint dogfood status --session-key %s and finish it with corvint dogfood finish --session-key %s --aggregate-outcome-profile %s, then rerun the same command (DCW-V0-448)\n", key, key, tracerecordrepo.AggregateOutcomeProfile)
		c.exit(2)
	}
	c.say("  this checkout holds aggregate local-outcome evidence for HEAD that no plain change report binds; finish an enrolled aggregate change with corvint dogfood check %s, or, when the file is left over from an earlier run, move it aside (reversible)\n", c.options.Base)
	c.say("  fix: mv %s %s, then rerun the same command (DCW-V0-448)\n", shellWord(marker), shellWord(marker+".stale"))
	c.exit(2)
}

// LegacyRecordAttempt is an actual recorder invocation, captured only after
// bounded stdout/stderr stages close successfully. It is not inferred from a
// report reason or a stderr substring.
// The legacy diff admits 64 MiB of names. JSON escaping is at most six bytes
// per input byte; the extra allowance covers bounded admitted paths/trace fields.
const legacyRecorderOutputLimit = 6*(64<<20) + (4 << 20)

type LegacyRecordAttempt struct {
	Exit           int
	Stdout, Stderr []byte
}
type recorderStage struct {
	file      *os.File
	remaining int
	overflow  bool
}

func (s *recorderStage) Write(raw []byte) (int, error) {
	n := min(len(raw), s.remaining)
	written, err := s.file.Write(raw[:n])
	s.remaining -= written
	if err != nil {
		return written, err
	}
	if n < len(raw) {
		s.overflow = true
		return written, io.ErrShortWrite
	}
	return written, nil
}
func (c *change) recordWithClosedStages(args []string, output, errorFile string) (int, error) {
	stdout, err := os.CreateTemp(c.evidence, ".local-outcome-*.stdout")
	if err != nil {
		return 1, err
	}
	stderr, err := os.CreateTemp(c.evidence, ".local-outcome-*.stderr")
	if err != nil {
		_ = stdout.Close()
		return 1, err
	}
	outStage := &recorderStage{file: stdout, remaining: legacyRecorderOutputLimit}
	errStage := &recorderStage{file: stderr, remaining: 64 << 10}
	status := c.options.Steps.Run(c.ctx, c.root, args, outStage, errStage)
	outClose, errClose := stdout.Close(), stderr.Close()
	if outClose != nil {
		return 1, outClose
	}
	if errClose != nil {
		return 1, errClose
	}
	if outStage.overflow || errStage.overflow {
		return 1, errors.New("aggregate-publication-failed")
	}
	outRaw, err := aggregateReadFile(stdout.Name(), legacyRecorderOutputLimit)
	if err != nil {
		return 1, err
	}
	errRaw, err := aggregateReadFile(stderr.Name(), 64<<10)
	if err != nil {
		return 1, err
	}
	if c.options.CaptureLegacyRecord != nil {
		if err = c.options.CaptureLegacyRecord(LegacyRecordAttempt{status, outRaw, errRaw}); err != nil {
			return 1, err
		}
	}
	if err = os.Rename(stdout.Name(), output); err != nil {
		return 1, err
	}
	if err = os.Rename(stderr.Name(), errorFile); err != nil {
		return 1, err
	}
	c.checkpoint()
	return status, nil
}

// passScope names the steps one change pass runs. finish-lane's prepare pass
// reads only the prepared map and its bind pass only the cite, guarding-test,
// OCM and status rows; the rebind is the full pass whose report is checked, so
// the narrowed passes never decide the delivered evidence (DCW-V0-115).
type passScope int

const (
	passFull passScope = iota
	passPrepare
	passBind
)
