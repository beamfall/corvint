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
	OCMLinks            string
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
	// staleBinding names the row whose hunk moved since the citation-plan
	// binding was recorded against an earlier map (DCW-V0-066).
	staleBinding string
	// citationFailedMap is the map the failing cite command read: the staged
	// citation map after the first row (DCW-V0-068).
	citationFailedMap string
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
	c.gitDir = c.gitValue("rev-parse", "--absolute-git-dir")
	c.acquireOperation()
	if present, err := AggregateMarkerPresent(c.root, c.gitDir); err != nil || present {
		c.refuse("aggregate-enrollment-required")
	}
	c.base = c.gitValue("rev-parse", c.options.Base+"^{commit}")
	c.target = c.gitValue("rev-parse", "HEAD^{commit}")
	task := c.options.Task
	if task == "" {
		task = "Dogfood change from " + c.base[:12] + " to HEAD"
	}
	// The sealed-CEM admissions refuse before the previous run's evidence is
	// cleared, so a refusal keeps it (DCW-V0-042, DCW-V0-043).
	inherited := c.inheritedCEM()
	c.admitSealedCEMs(inherited)
	c.requireArchivedBaseCEM(inherited)
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
	c.citationStage = ".corvint/.cem-citations." + filepath.Base(runTmp) + ".json"
	c.resolveAnchor()
	// Query and impact run after the change, so they are coordination-time
	// receipts; the agent's prechange-*.json receipts are never written here (DCW-V0-026).
	if c.scope == passFull {
		c.coordinationQuery(task)
		c.prechangeImpact()
	}
	c.cemPrepared = c.prepare("cem-prepare", c.evidence+"/cem-prepare.json", func(status int, stderr []byte, reason string) bool {
		return status == 2 && outdatedCEMMaps[chomp(string(stderr))]
	}, "cem", "prepare", "--base", c.base, "--target", c.target) == 0
	c.markStableChoreMap()
	if c.scope == passPrepare {
		if c.cemPrepared {
			return 0
		}
		c.renderReport()
		return c.reportFailures()
	}
	// The manifest is frozen before citation so the plan check knows which
	// base-absent intent hunks an author may deliberately leave uncited.
	c.manifestReason = c.validateIntentManifest()
	c.manifestValid = c.manifestReason == ""
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
	c.runStep("cem-status", c.evidence+"/cem-status.json", stableStatusArguments(c.root,
		"cem", "status", "--map", ".corvint/change.cem.json",
		"--expected-base", c.base, "--target", c.target, "--max-unknown", strconv.Itoa(c.bootstrapUnknown+c.reasonedUnknown), "--max-mechanical", "0")...)
	// The recorder refuses a dirty tree, so it runs last and sees the sidecar
	// this pass prepared and cited: an untracked or modified sidecar reports
	// record-index-failed and the pass is never complete (DCW-V0-015).
	if c.scope != passBind {
		c.rows = append(c.rows, c.localOutcome(task))
	}
	c.renderReport()
	return c.reportFailures()
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
	if status != 2 || !impactAbstentions[reason] || code != reason || hasContent(output) || bytes.Count(stderr, []byte("\n")) != 1 {
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
		return step{"local-outcome", "NOT_PRODUCED", verifyFailureReason(err)}
	}
	args := append([]string{"dogfood-record", "--base", c.base, "--target", c.target, "--task", task}, verify...)
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

// validateVerify checks the commands of verifyInput's argument pairs against
// the recorder's admitted command syntax and bounds.
func validateVerify(verify []string) error {
	commands := make([]string, 0, len(verify)/2)
	for i := 1; i < len(verify); i += 2 {
		commands = append(commands, verify[i])
	}
	return trace.ValidateVerificationCommands(commands)
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

// verifyArguments passes one --verify per line that is not blank; the recorder
// and localOutcome share command validation before recorder invocation.
func verifyArguments(data []byte) []string {
	args := []string{}
	for _, line := range textLines(data) {
		if strings.TrimLeft(line, " \t\n\v\f\r") != "" {
			args = append(args, "--verify", line)
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
	source := c.options.IntentsFile
	if source == "" || !isRegular(source) || isSymlink(source) {
		return "missing-intent-scope"
	}
	data, err := readPrefix(source, maxIntentManifestBytes+1)
	if err == nil && len(data) > maxIntentManifestBytes {
		return "intent-manifest-over-bound"
	}
	if err != nil || len(data) == 0 || data[len(data)-1] != '\n' {
		return "missing-intent-scope"
	}
	c.intentSource = data
	if bytes.Equal(data, noIntentManifest) {
		c.intentSnapshot, c.noIntent = data, true
		return ""
	}
	paths := readLines(data)
	if len(paths) > 16 {
		return "missing-intent-scope"
	}
	previous := ""
	for _, path := range paths {
		if !canonicalIntentPath(path) || (previous != "" && previous >= path) {
			return "missing-intent-scope"
		}
		previous = path
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
	plan, valid := c.validateCitationPlan()
	mismatch := ""
	if valid {
		mismatch = c.citationPlanMismatch(plan)
	}
	switch {
	case !valid && c.citationCount > 256:
		status, reason = "NOT_PRODUCED", "citation-plan-row-limit-exceeded"
	case !valid:
		status, reason = "NOT_PRODUCED", "invalid-citation-plan"
	case mismatch != "":
		status, reason = "NOT_PRODUCED", mismatch
	case c.uniformPlan(plan):
		status, reason = "NOT_PRODUCED", "citation-plan-uniform"
	case c.citationCount > 1 && exists(c.path(c.citationStage)):
		status, reason = "NOT_PRODUCED", "citation-stage-exists"
	case c.prepareUnknownReasons(plan) != nil:
		status, reason = "NOT_PRODUCED", "unknown-reasons-not-recorded"
	default:
		c.recordCitationBinding(plan)
		cited := citedHunks(c.path(".corvint/change.cem.json"))
		before, _ := readPrefix(c.path(".corvint/change.cem.json"), maxPlanBytes)
		status, reason = c.cite(plan, citeOutput)
		c.citedOver = cited > 0 && status == "PRODUCED"
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

// uniformPlan reports, and keeps for the fix hint, a plan that cites one span
// for every hunk of a change with more than one hunk; it is refused before any
// cite runs (DCW-V0-048). A chore or record change is exempt (DCW-V0-058).
func (c *change) uniformPlan(plan []byte) bool {
	data, err := readPrefix(c.path(".corvint/change.cem.json"), maxPlanBytes)
	if err != nil {
		return false
	}
	hunks, span, uniform := uniformCitation(plan, mapHunks(data))
	if !uniform || c.choreRole() != "" || c.recordChange() {
		return false
	}
	c.uniformHunks, c.uniformSpan = hunks, span
	return true
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
	for index, line := range readLines(plan) {
		fields := strings.SplitN(line, "\t", 4)
		args := []string{"cem", "cite", "--map", citeMap, "--hunk", fields[0], "--evidence-path", fields[1], "--lines", fields[2], "--relation", fields[3]}
		if unknownRow(fields) {
			args = []string{"cem", "mark", "--map", citeMap, "--hunk", fields[0], "--disposition", "unknown", "--reason", fields[2]}
		}
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
			return "NOT_PRODUCED", failureReason(stderr, status)
		}
		// Intermediate receipts truthfully name the stage; only the last cite
		// publishes the original map. Keep their raw stdout as private evidence.
		appendFile(citeOutput, readFile(part))
		citeMap = c.citationStage
	}
	return "PRODUCED", "none"
}

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
func (c *change) validateCitationPlan() ([]byte, bool) {
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
	if c.ordinalsMoved(plan, hunks) {
		return mismatch
	}
	named := map[string]bool{}
	for _, line := range textLines(plan) {
		selector, _, _ := strings.Cut(line, "\t")
		named[selector] = true
	}
	for selector := range named {
		numeric := ordinal.MatchString(selector)
		if strings.ContainsAny(selector[:1], "+0123456789") && !numeric {
			return mismatch
		}
		if value, err := strconv.Atoi(selector); numeric && (err != nil || value > len(hunks)) {
			return mismatch
		}
	}
	// A hunk an earlier plan kept unknown with a recorded reason is marked,
	// so, like a cited one, it needs no row (DCW-V0-019).
	owed := []int{}
	for index, hunk := range hunks {
		if hunk["disposition"] == "unknown" && !bootstrap[hunk["path"]] && !recordedUnknownReasons[hunk["reason"]] {
			owed = append(owed, index+1)
		}
	}
	// More owed hunks than one plan has rows: split plans stay admissible.
	if len(owed) > 256 && len(plan) > 0 {
		return ""
	}
	for _, index := range owed {
		if !named[strconv.Itoa(index)] && !named[hunks[index-1]["id"]] {
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
func (c *change) ordinalsMoved(plan []byte, hunks []map[string]string) bool {
	recorded := readLines(readFile(c.evidence + citationBinding))
	if len(recorded) == 0 || recorded[0] != sha256Hex(plan) {
		return false
	}
	current := map[string]int{}
	for index, hunk := range hunks {
		current[hunk["id"]] = index + 1
	}
	for index, line := range textLines(plan) {
		selector, _, _ := strings.Cut(line, "\t")
		value, err := strconv.Atoi(selector)
		if !ordinal.MatchString(selector) || err != nil || value >= len(recorded) {
			continue
		}
		if now, found := current[recorded[value]]; found && now != value {
			c.staleBinding = fmt.Sprintf("plan row %d (ordinal %d) named hunk %s, which the current map holds at ordinal %d", index+1, value, recorded[value], now)
			return true
		}
	}
	return false
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
	{"*:git-budget-exceeded", "the change is larger than one 1,024-Git-operation verifier pass; bind and seal it as smaller changes on their own branches, merge them, and check the merge batch with corvint dogfood batch-check {base} instead of rebinding it (DCW-V0-039, DCW-V0-040)"},
	{"cem-*:map-unavailable", "the CEM map {map} could not be read ({evidence}/{step}.stderr says cannot read CEM map): it is missing, not a regular file or not a valid map, often because it was removed or edited while the run was in progress; restore a committed map with git restore --source=HEAD -- .corvint/change.cem.json, or rerun corvint dogfood change {base} to prepare it again (DCW-V0-068)"},
	{"cem-cite:citation-plan-not-provided", "set DOGFOOD_CITATIONS to the path of a TSV plan with one row per hunk of .corvint/change.cem.json"},
	{"cem-cite:citation-plan-unavailable", "DOGFOOD_CITATIONS must be the path of a TSV file of ORDINAL<TAB>PATH<TAB>START:END<TAB>RELATION rows, not the rows themselves"},
	{"cem-cite:empty-citation-plan", "DOGFOOD_CITATIONS names an empty file but .corvint/change.cem.json still has unknown hunks; write one row per unknown hunk, or unset DOGFOOD_CITATIONS to prepare the map without citing"},
	{"cem-cite:citation-plan-row-limit-exceeded", "DOGFOOD_CITATIONS row limit=256 observed={citationRows}; use explicit staged plans of at most 256 rows on the same CEM (docs/DOGFOOD.md Staged binding of one oversized commit)"},
	{"cem-cite:invalid-citation-plan", "each row is ORDINAL<TAB>PATH<TAB>START:END<TAB>RELATION in worklist order, LF-terminated, at most 256 rows; a hunk kept unknown takes ORDINAL<TAB>unknown<TAB>REASON<TAB>NOTE, REASON being insufficient-evidence or conflicting-evidence and NOTE a one-line reason (DCW-V0-019)"},
	{"cem-cite:citation-plan-map-mismatch", "the plan does not match the map prepared for HEAD: a row names an ordinal past its hunks or is not a canonical ordinal, or an unknown hunk is unnamed (often because a later commit re-prepared the map); or a row's ordinal now names another hunk than when this plan was first cited (a later commit added or removed a hunk before it); rewrite DOGFOOD_CITATIONS from the current .corvint/change.cem.json, naming every unknown hunk except the hunk of an intent spec absent at BASE, by a citation or by an ORDINAL<TAB>unknown<TAB>REASON<TAB>NOTE row"},
	{"cem-cite:citation-plan-uniform", "the plan cites one span for every hunk: hunks={uniformHunks} span={uniformSpan}; cite, per hunk, the BASE lines that hunk depends on or implements (its caller, contract, test or spec section), or keep a hunk unknown with ORDINAL<TAB>unknown<TAB>REASON<TAB>NOTE; one span may serve several hunks, never every hunk of a multi-hunk non-chore change (DCW-V0-048)"},
	{"cem-cite:unknown-reasons-not-recorded", "the unknown rows of DOGFOOD_CITATIONS could not be kept in {evidence}/cem-unknown-reasons.tsv (or its .next file): before citing (an occupied path, or notes over 16 MiB) nothing was cited; after citing the map has changed and earlier notes are kept; remove whatever occupies the path and rerun the same plan, which replays idempotently (DCW-V0-046)"},
	{"cem-cite:invalid-arguments", "plan row {row}{rowValues}: RELATION must be one of call-site, decision, dependency, implementation, incident, specification or test-claim, and START:END a line span of the cited file; any file present at BASE may be cited, code and tests as well as specs (DCW-V0-041)"},
	{"cem-cite:missing-evidence", "plan row {row} cites a path that is not a regular file at BASE; cite a source, test or spec file that exists at BASE, since a file this change adds cannot be evidence for itself (DCW-V0-041)"},
	{"cem-cite:cite-span-not-stable", "plan row {row} cites BASE lines that this change edits or deletes; cite a START:END span the change leaves unchanged"},
	{"cem-cite:cite-self-modified-authority", "plan row {row} cites a governing file (spec, decision or other authority) that this change itself modifies, which cannot govern it; cite a governing source the change leaves unmodified, or keep the hunk unknown with ORDINAL<TAB>unknown<TAB>REASON<TAB>NOTE (CEM-V1-114)"},
	{"ocm-aggregate:missing-intent-scope", "DOGFOOD_INTENTS_FILE must be the path of a sorted, LF-terminated file listing 1-16 repository-relative spec paths, or of a file holding the one line #no-intent-declared when no requirements spec governs the change"},
	{"ocm-aggregate:intent-manifest-over-bound", "DOGFOOD_INTENTS_FILE is larger than 16 paths of 512 bytes can be; list 1-16 repository-relative spec paths, or the one line #no-intent-declared"},
	{"ocm-prepare-*:invalid-requirements-section", `intent must be a spec that exists at BASE and contains exactly one "## Requirements" heading`},
	{"ocm-prepare-*:excluded-artifact-mismatch", uncommittedHint},
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
	{"local-outcome:admitted-path-limit", "a local outcome admits at most 200 changed paths; bind and seal the change as smaller changes, merge them, and check the merge batch with corvint dogfood batch-check {base} instead of rebinding it (DCW-V0-039, DCW-V0-040)"},
	{"local-outcome:outcome-input-not-provided", "set DOGFOOD_OUTCOME (passed, failed or blocked) and DOGFOOD_VERIFY_FILE (one verification command per line)"},
	{"local-outcome:verify-file-unavailable", "DOGFOOD_VERIFY_FILE must be the path of a regular file holding one verification command per line, not the commands themselves"},
	{"local-outcome:verify-file-over-bound", "DOGFOOD_VERIFY_FILE is larger than 64 KiB; keep at most 50 verification commands of at most 512 characters, one per line"},
	{"local-outcome:unsupported-verify-syntax", "each DOGFOOD_VERIFY_FILE line is one command of ASCII letters, digits and _./:@=+, - only, with no quotes, ^, $, |, parentheses or other shell syntax; write -run TestName instead of -run '^TestName$'; a command that needs quotes or regular-expression syntax, such as a split -run '^Test[A-C]' group, goes in a script committed with the change and is listed as sh path/to/script.sh (DCW-V0-065)"},
	{"local-outcome:record-failed", "read {evidence}/local-outcome.stderr for the cause; DOGFOOD_VERIFY_FILE holds at most 50 commands of at most 512 characters each, so split a longer command into several lines"},
}

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

// citeSpanUnedited is the cem cite refusal phrase for a span whose base
// lines no hunk edits but whose text recurs in the changed file.
const citeSpanUnedited = "are edited by no hunk"

// citeSpanUnalignedHint is the DCW-V0-063 fix line for that refusal.
const citeSpanUnalignedHint = "plan row {row} cites BASE lines that no hunk edits (at most hunk context), which git's line alignment leaves unchanged, but their text occurs more than once in the changed file (another copy exists or the change adds one), and the stability check matches text, not line positions; cite a span whose text occurs once in the changed file, for example by widening it to a neighbouring distinct line (DCW-V0-063)"

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
	if value == "cem-cite:cite-span-not-stable" && c.citeSpanUnaligned {
		return strings.ReplaceAll(citeSpanUnalignedHint, "{row}", strconv.Itoa(c.citationFailedRow))
	}
	for _, entry := range fixHints {
		prefix, suffix, star := strings.Cut(entry.pattern, "*")
		matched := value == entry.pattern
		if star {
			matched = len(value) >= len(prefix)+len(suffix) && strings.HasPrefix(value, prefix) && strings.HasSuffix(value, suffix)
		}
		if matched {
			return strings.NewReplacer("{evidence}", c.evidence, "{base}", c.base, "{step}", row.name, "{map}", c.path(c.failedMap()), "{row}", strconv.Itoa(c.citationFailedRow), "{rowValues}", c.rowValues(), "{citationRows}", strconv.Itoa(c.citationCount), "{linkRows}", strconv.Itoa(c.linkObservedRows), "{uniformHunks}", strconv.Itoa(c.uniformHunks), "{uniformSpan}", c.uniformSpan).Replace(entry.hint)
		}
	}
	return ""
}

// noteAgentReceipts reports, without blocking, an agent pre-change receipt that
// is absent, over the 4 MiB bound, malformed, or was not written against the
// base tree (DCW-V0-031, DCW-V0-032).
func (c *change) noteAgentReceipts() {
	baseTree := c.gitValue("rev-parse", c.base+"^{tree}")
	for _, name := range []string{"prechange-query", "prechange-impact"} {
		c.noteAgentReceipt(name, baseTree)
	}
}

func (c *change) noteAgentReceipt(name, baseTree string) {
	path := c.evidence + "/" + name + ".json"
	if !isRegular(path) {
		c.say("dogfood-change: NOTE %s NOT_OBSERVED agent-receipt-absent\n", name)
		return
	}
	var receipt struct {
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
	switch tree := receipt.Context.Revision; tree {
	case baseTree:
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
		return 0
	}
	c.say("dogfood-change: FAIL not-complete\n")
	for _, row := range c.rows {
		if !c.rowFailing(row) {
			continue
		}
		c.say("  %s: %s\n", row.name, row.reason)
		if hint := c.fixHint(row); hint != "" {
			c.say("    fix: %s\n", hint)
		}
	}
	// cem cite only adds evidence and prepare resumes a matching map, so a
	// corrected plan joins the earlier plan's citations (DCW-V0-019).
	if c.citedOver {
		c.say("  cem-cite: the plan was added to citations the map already carried and never replaces them; to correct an earlier plan, delete .corvint/change.cem.json and rerun corvint dogfood change %s (docs/DOGFOOD.md step 4)\n", c.base)
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
	bad := errors.New("aggregate-schema-invalid")
	if len(raw) == 0 || len(raw) > 4<<20 {
		return value, bad
	}
	parsed, err := wire.Parse(raw)
	if err != nil {
		return value, bad
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&value); err != nil {
		return value, bad
	}
	// Canonical comparison requires every member and rejects null scalar values;
	// incoming whitespace and object key order are not semantic constraints.
	encoded, err := aggregateWorkerJSON(value)
	if err != nil || !bytes.Equal(wire.CanonicalValue(parsed), bytes.TrimSuffix(encoded, []byte{'\n'})) {
		return value, bad
	}
	if value.Profile != AggregateReportProfile || value.LocalOutcomeProfile != tracerecordrepo.AggregateOutcomeProfile || !fullRevision.MatchString(value.Base) || !fullRevision.MatchString(value.Target) || !aggregateDigestPattern.MatchString(value.LocalOutcomeEvidenceSHA256) || !aggregateKeyPattern.MatchString(value.Enrollment.Session) || !aggregateKeyPattern.MatchString(value.Enrollment.PlanDigest) || !aggregateDigestPattern.MatchString(value.Enrollment.BindingSHA256) || len(value.Enrollment.Generation) != 68 || !strings.HasPrefix(value.Enrollment.Generation, value.Enrollment.PlanDigest+"-") {
		return value, bad
	}
	if value.CompletionState != "complete" && value.CompletionState != "incomplete" {
		return value, bad
	}
	if len(value.Steps) < 1 || len(value.Steps) > 256 || value.DogfoodPolicy.BootstrapUnknown < 0 || value.DogfoodPolicy.MaximumUnknownAfterBootstrap != 0 {
		return value, bad
	}
	outcomeRows := 0
	rows := &change{}
	for _, row := range value.Steps {
		if row.Status != "PRODUCED" && row.Status != "NOT_PRODUCED" {
			return value, bad
		}
		if row.Name == "local-outcome" {
			outcomeRows++
			if row.Status != "PRODUCED" || row.Reason != "none" {
				return value, bad
			}
		}
		rows.rows = append(rows.rows, step{row.Name, row.Status, row.Reason})
	}
	if value.QueryAbstentionEvidenceSHA256 != nil {
		if !aggregateDigestPattern.MatchString(*value.QueryAbstentionEvidenceSHA256) {
			return value, bad
		}
		rows.queryAbstentionSHA = *value.QueryAbstentionEvidenceSHA256
	}
	if value.ContextAbstentionEvidenceSHA256 != nil && !aggregateDigestPattern.MatchString(*value.ContextAbstentionEvidenceSHA256) {
		return value, bad
	}
	if outcomeRows != 1 || (value.CompletionState == "complete" && !rows.complete()) {
		return value, bad
	}
	if value.Anchor.State == "OBSERVED" {
		if value.Anchor.MergeBase == nil || !fullRevision.MatchString(*value.Anchor.MergeBase) {
			return value, bad
		}
	} else if value.Anchor.State != "NOT_OBSERVED" || value.Anchor.MergeBase != nil {
		return value, bad
	}
	if value.DogfoodCheck != nil {
		check := value.DogfoodCheck
		if !aggregateDigestPattern.MatchString(check.BaseVerifierSHA256) || !aggregateDigestPattern.MatchString(check.TreeVerifierSHA256) || (check.OverrideVerifierSHA256 != nil && !aggregateDigestPattern.MatchString(*check.OverrideVerifierSHA256)) || check.BootstrapUnknown < 0 || check.MaximumUnknownAfterBootstrap != 0 {
			return value, bad
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
	// Repository spelling aliases (such as /tmp and /private/tmp) are already
	// supported. Resolve the root, then reject symlinks below that boundary.
	canonicalRoot, rootErr := filepath.EvalSymlinks(root)
	if rootErr != nil {
		return false, rootErr
	}
	root = canonicalRoot
	owner, err := aggregateReadFile(filepath.Join(gitDir, "corvint", "local-completion", "owner"), 65)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err == nil {
		key := strings.TrimSuffix(string(owner), "\n")
		if !aggregateKeyPattern.MatchString(key) {
			return false, errors.New("aggregate-enrollment-required")
		}
		raw, readErr := aggregateReadFile(filepath.Join(gitDir, "corvint", "local-completion", key, "state.json"), 256<<10)
		if readErr != nil {
			return false, readErr
		}
		if _, err := wire.Parse(raw); err != nil {
			return false, err
		}
		var state map[string]json.RawMessage
		if err := json.Unmarshal(raw, &state); err != nil {
			return false, err
		}
		if _, ok := state["aggregateOutcome"]; ok {
			return true, nil
		}
	}
	for _, name := range []string{filepath.Join(root, ".corvint", "dogfood-report.json"), filepath.Join(gitDir, "corvint", "local-outcome.json")} {
		raw, err := aggregateReadFile(name, 4<<20)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(raw, &object) != nil {
			continue
		} // Legacy failed output is not an aggregate marker.
		var profile string
		_ = json.Unmarshal(object["profile"], &profile)
		if profile == AggregateReportProfile || profile == tracerecordrepo.AggregateOutcomeProfile {
			return true, nil
		}
		if _, ok := object["localOutcomeProfile"]; ok {
			return true, nil
		}
	}
	return false, nil
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
