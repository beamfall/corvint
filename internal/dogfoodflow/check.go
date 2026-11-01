package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/dogfoodocm"
	"github.com/Beamfall/corvint/internal/dogfoodoperation"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

// CheckOptions are the inputs of `corvint dogfood check` and `dogfood seal`.
// The base and tree verifiers must be distinct identities only when the caller
// has them; a portable check may pass the running binary for both.
type CheckOptions struct {
	AggregateBegin    func(context.Context, string, []byte) (*AggregateCheckStage, error)
	AggregateValidate func(context.Context, string, []byte) error
	AggregatePublish  func(context.Context, string, AggregateCheckCapture) error
	Root              string
	Base              string
	BaseVerifier      Runner
	TreeVerifier      Runner
	Override          *Runner
	// ConcurrentVerifiers runs the base and tree verifiers' cem status
	// commands at once (DCW-V0-907); the dogfood commands set it.
	ConcurrentVerifiers bool
	Exception           string
	// Verbose prints the full CEM and OCM status JSON on PASS, the output
	// DCW-V0-020 fixed before DCW-V0-033 (DOGFOOD_VERBOSE=1).
	Verbose bool
	// OutputChange is DOGFOOD_OUTPUT_CHANGE: the requirement ID of a change
	// that alters the canonical `cem status` output by construction, so the
	// base verifier cannot agree with the tree verifier (DCW-V0-047).
	OutputChange string
	// ReusePass replays the PASS a former check left in its receipt when that
	// receipt binds the same BASE, HEAD and inputs digest instead of rerunning
	// the verifiers (DCW-V0-347). Seal sets it; `dogfood check` takes it from
	// DOGFOOD_CHECK_REUSE=1.
	ReusePass bool
}

// AggregateCheckStage is an already admitted private observation. Finish only
// closes and retains its bytes; it confers no successful-check authority.
type AggregateCheckStage struct {
	Stdout, Stderr io.Writer
	Finish         func(int, error) (AggregateCheckCapture, error)
	RetainFailure  func(error) error
}

type AggregateCheckCapture struct {
	OriginalReport []byte
	ProposedReport []byte
	Exit           int
	Stdout         []byte
	Stderr         []byte
	Failed         bool
}

type check struct {
	sealed                                     bool
	sealedTarget                               string
	cemPath                                    string
	aggregateCapture                           *AggregateCheckStage
	aggregateSavedStdout, aggregateSavedStderr io.Writer
	aggregate                                  *AggregateReport
	aggregateOriginal                          []byte
	aggregatePrepared                          []byte
	flow
	options     CheckOptions
	stdout      io.Writer
	report      string
	evidence    string
	baseSHA     string
	treeSHA     string
	overrideSHA string
	// outputChange is the admitted DOGFOOD_OUTPUT_CHANGE requirement ID, and
	// baseStatusSHA and treeStatusSHA the digests of the base and tree
	// verifiers' `cem status` stdout it records (DCW-V0-047), and
	// baseRefusal the base verifier's refusal code when it refused the read
	// instead, its digest then empty; all are empty without a declaration.
	outputChange, baseStatusSHA, treeStatusSHA, baseRefusal string
	// clock times the run's phases (DCW-V0-942).
	clock *phaseClock
}

type verification struct {
	status         int
	stdout, stderr []byte
}

// Check verifies the bound change against the report, as script/dogfood-check.sh did.
func Check(ctx context.Context, options CheckOptions, stdout, stderr io.Writer) (int, error) {
	run := newCheck(ctx, "dogfood-check", options, stdout, stderr)
	defer run.releaseOperation()
	defer func() { _, _, _ = run.writeTimings() }()
	code, err := boundary(ctx, run.run, func() {})
	if publishErr := run.finishAggregateCapture(code, err); publishErr != nil {
		return 2, publishErr
	}
	return code, err
}

// Seal checks the bound change, then moves its CEM out of the one shared
// tracked path in a rename-only commit (docs/DOGFOOD.md §4, DOGFOOD-013).
func Seal(ctx context.Context, options CheckOptions, stdout, stderr io.Writer) (int, error) {
	options.ReusePass = true
	run := newCheck(ctx, "dogfood-check", options, stdout, stderr)
	defer run.releaseOperation()
	defer func() { _, _, _ = run.writeTimings() }()
	return boundary(ctx, func() int {
		// The check's own exits, including its no-change PASS, return here as the
		// former `dogfood-check.sh || exit` did.
		code, err := boundary(ctx, run.run, func() {})
		publishErr := run.finishAggregateCapture(code, err)
		if err != nil {
			run.exit(-1)
		}
		if publishErr != nil {
			run.refuse(publishErr.Error())
		}
		if code != 0 {
			return code
		}
		run.prefix = "dogfood-seal"
		return run.seal()
	}, func() {})
}

func newCheck(ctx context.Context, prefix string, options CheckOptions, stdout, stderr io.Writer) *check {
	return &check{flow: flow{ctx: ctx, prefix: prefix, root: options.Root, stderr: stderr}, options: options, stdout: stdout, cemPath: ".corvint/change.cem.json", clock: newPhaseClock(time.Now())}
}

func (c *check) seal() int {
	c.clock.to("seal")
	bind := c.gitValue("rev-parse", "HEAD^{commit}")
	if !c.gitSucceeds("cat-file", "-e", bind+":.corvint/change.cem.json") {
		c.refuse("nothing-to-seal")
	}
	if replaced := c.unarchivedBaseCEM(bind); replaced != "" {
		c.say("%s: REFUSE unarchived-base-cem\n", c.prefix)
		c.say("  BASE tracks .corvint/change.cem.json (bound at %s) that this change replaced and no .corvint/changes/ file keeps\n", replaced)
		if inherited := c.inheritedCEM(); inherited.path != "" {
			c.say("  fix: %s\n", inherited.archiveRecovery())
			c.say("  then rerun dogfood-change, bind, dogfood-check and dogfood-seal with the same BASE (DCW-V0-043)\n")
		}
		c.exit(2)
	}
	sealed := ".corvint/changes/" + bind + ".cem.json"
	_ = os.MkdirAll(c.root+"/.corvint/changes", 0o777)
	if c.gitPassthrough("mv", ".corvint/change.cem.json", sealed) != 0 {
		c.exit(2)
	}
	if c.gitPassthrough("commit", "-q", "-m", "chore: seal change evidence") != 0 {
		c.exit(2)
	}
	if c.baseIndependence() != "" {
		// The seal keeps the declared output change beside its PASS (DCW-V0-047).
		fmt.Fprintf(c.stdout, "dogfood-seal: PASS sealed=%s %s\n", sealed, c.baseIndependence())
		return 0
	}
	fmt.Fprintf(c.stdout, "dogfood-seal: PASS sealed=%s\n", sealed)
	return 0
}

// unarchivedBaseCEM names the commit that bound BASE's tracked CEM when this
// change replaced it and the bind tree keeps no copy under .corvint/changes/,
// so sealing would drop it from the tree (V1-0137); otherwise it is empty.
func (c *check) unarchivedBaseCEM(bind string) string {
	baseCEM, status := c.git(true, "rev-parse", "--verify", "-q", c.base+":.corvint/change.cem.json")
	if status != 0 {
		return ""
	}
	baseCEM = chomp(baseCEM)
	if baseCEM == c.gitValue("rev-parse", bind+":.corvint/change.cem.json") {
		return ""
	}
	archived, _ := c.git(false, "ls-tree", "-r", bind, "--", ".corvint/changes")
	if strings.Contains(archived, " "+baseCEM+"\t") {
		return ""
	}
	return c.gitValue("log", "-1", "--format=%H", c.base, "--", ".corvint/change.cem.json")
}

// gitPassthrough runs one Git command whose output reaches the caller. It is
// the seal's git mv and git commit, so it keeps the caller's environment: the
// commit takes its author identity, signing and hooks from the user's
// configuration, and a sanitized environment would reach those hooks.
func (c *check) gitPassthrough(args ...string) int {
	command := exec.CommandContext(c.ctx, "git", append([]string{"-C", c.root}, args...)...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	command.Stdout, command.Stderr = c.stdout, c.stderr
	status := exitStatus(command.Run())
	c.checkpoint()
	return status
}

func (c *check) fail(reason string, lines ...string) {
	c.say("dogfood-check: FAIL %s\n", reason)
	for _, line := range lines {
		c.say("%s\n", line)
	}
	if recovery := recoveryLine(reason, c.base, lines); recovery != "" {
		c.say("%s\n", recovery)
	}
	c.exit(1)
}

func (c *check) run() int {
	c.requireRoot()
	c.requireTrustedAncestry()
	c.base = c.gitValue("rev-parse", c.options.Base+"^{commit}")
	c.target = c.gitValue("rev-parse", "HEAD^{commit}")
	c.cemPath = ".corvint/change.cem.json"
	if c.sealed {
		binding, err := ResolveSealBinding(c.ctx, c.root, c.target)
		if err != nil {
			c.refuse("sealed-binding-required")
		}
		c.sealedTarget = c.target
		c.target, c.cemPath = binding.EvidenceTarget, binding.MapPath
	}
	c.gitDir = c.gitValue("rev-parse", "--absolute-git-dir")
	c.evidence = c.gitDir + "/corvint"
	c.adoptBuildTimings()
	c.clock.to("lock-wait")
	c.acquireOperation()
	c.clock.to("precheck")
	c.enableAggregateCapture()
	c.report = c.root + "/.corvint/dogfood-report.json"
	c.resolveAnchor()
	if c.isSealCommit(c.target) {
		c.say("dogfood-check: REFUSE sealed-head\n")
		c.say("  HEAD only seals the bound CEM; check its parent: git checkout --detach HEAD^\n")
		c.exit(2)
	}
	var report []byte
	marker, markerErr := AggregateMarkerPresent(c.root, c.gitDir)
	if markerErr != nil {
		c.refuse("aggregate-enrollment-required")
	}
	if marker && c.sealed {
		c.refuse("sealed-aggregate-unsupported")
	}
	if marker {
		report = c.checkReport()
	}
	c.requireCleanChange()
	c.reportUnboundCommits()
	if c.options.Exception != "" {
		c.say("dogfood-check: FAIL exception-contract-undefined\n")
		c.exit(2)
	}
	if report == nil {
		report = c.checkReport()
	}
	if c.sealed {
		var shape struct {
			Steps []struct{ Name, Status, Reason string } `json:"steps"`
		}
		if json.Unmarshal(report, &shape) != nil {
			c.refuse("sealed-context-replay-unsupported")
		}
		for _, name := range []string{"coordination-time-impact", "coordination-time-query"} {
			count := 0
			for _, step := range shape.Steps {
				if step.Name == name {
					count++
					if step.Status != "PRODUCED" || step.Reason != "none" {
						c.refuse("sealed-context-replay-unsupported")
					}
				}
			}
			if count != 1 {
				c.refuse("sealed-context-replay-unsupported")
			}
		}
	}
	c.admitOutputChange()
	abstention := c.checkContextAbstention(report)
	queryTask, queryAbstention := c.checkQueryAbstention(report)
	if c.sealed && (abstention || queryAbstention) {
		c.refuse("sealed-context-replay-unsupported")
	}
	c.resolveVerifiers()
	// A pass bound to this BASE, HEAD and every input stands; anything else
	// reruns the verifiers (DCW-V0-347).
	c.clock.to("reuse")
	if !c.sealed && c.reusePass() {
		return 0
	}
	c.clock.to("binding")
	passed := func() {}
	if !c.sealed {
		passed = c.capturePass()
	}
	if abstention {
		c.verifyAbstention()
	}
	if queryAbstention {
		c.verifyQueryAbstention(queryTask)
	}
	code := c.verifyBinding(report)
	if code == 0 {
		if c.sealed {
			current := c.gitValue("rev-parse", "HEAD^{commit}")
			binding, err := ResolveSealBinding(c.ctx, c.root, current)
			if err != nil || current != c.sealedTarget || binding.EvidenceTarget != c.target || binding.MapPath != c.cemPath {
				c.refuse("sealed-binding-drift")
			}
			c.requireCleanChange()
		}
		passed()
	}
	return code
}

// requireCleanChange refuses an uncommitted worktree and passes a clean one with
// no change between BASE and HEAD.
func (c *check) requireCleanChange() {
	_, diffStatus := c.git(false, "diff", "--quiet", c.base, c.target, "--", ".")
	if diffStatus > 1 {
		c.exit(2)
	}
	// Core's hardened probe reads status against private metadata and refuses
	// a configuration that could run a repository-owned filter or hook, which
	// a plain git status would run (DCW-V0-045).
	repository, err := gokernel.ProbeRepositoryContext(c.ctx, c.root)
	c.checkpoint()
	if err != nil {
		c.say("dogfood-check: REFUSE worktree-status-unavailable\n  %s\n", oneLine(err.Error()))
		c.say("  fix: remove the repository configuration the reason names, or run dogfood check in a plain clone (git clone --no-hardlinks)\n")
		c.exit(2)
	}
	if repository.DirtyPathCount != 0 {
		refusal := "dirty-worktree"
		if diffStatus == 0 {
			refusal = "uncommitted-change-not-in-base-target"
		}
		c.say("dogfood-check: REFUSE %s\n", refusal)
		c.say("  required order: commit the change; corvint dogfood change <sha>; commit .corvint/change.cem.json; corvint dogfood change <sha>; corvint dogfood check <sha>\n")
		c.exit(2)
	}
	if diffStatus == 0 {
		// A declaration is never admitted by an empty range: an unknown ID
		// refuses output-change-requirement-unknown and a known one
		// output-change-not-in-diff, before the empty-range PASS (DCW-V0-047).
		c.admitOutputChange()
		fmt.Fprintf(c.stdout, "dogfood-check: PASS no-change-clean-worktree\n")
		c.exit(0)
	}
}

var (
	reportBase       = regexp.MustCompile(`^  "base": "([0-9a-f]*)",$`)
	reportTarget     = regexp.MustCompile(`^  "target": "([0-9a-f]*)",$`)
	outcomeDigest    = regexp.MustCompile(`^  ,"localOutcomeEvidenceSha256": "sha256:([0-9a-f]{64})"$`)
	abstentionDigest = regexp.MustCompile(`^  ,"contextAbstentionEvidenceSha256": "sha256:([0-9a-f]{64})"$`)
	missingRow       = regexp.MustCompile(`^.*"name": "([^"]*)", "status": "NOT_PRODUCED", "reason": "([^"]*)".*$`)
	abstentionRow    = regexp.MustCompile(`"name": "coordination-time-impact", "status": "NOT_PRODUCED", "reason": "([^"]*)"`)
	fullRevision     = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// checkReport requires a complete report bound to this BASE, HEAD, anchor and
// local outcome, and returns its bytes.
func (c *check) checkReport() []byte {
	defer c.clock.enter("report", "-")()
	if !isRegular(c.report) {
		lines := []string{"  fix: run corvint dogfood change " + c.base + " on this HEAD until it reports complete"}
		// A reviewer's clone of a bind commit never has the author's private report (DCW-V0-017).
		if c.gitSucceeds("cat-file", "-e", c.target+":.corvint/change.cem.json") {
			lines = append(lines, "  review: a reviewer without the author report: verifier agreement is author-only evidence (docs/DOGFOOD.md step 11); verify the bound CEM instead: corvint cem verify --map .corvint/change.cem.json --expected-base "+c.base+" --target "+c.target)
		}
		c.fail("dogfood-report-missing", lines...)
	}
	report := readFile(c.report)
	var profile struct {
		Profile string `json:"profile"`
	}
	_ = json.Unmarshal(report, &profile)
	marker, markerErr := AggregateMarkerPresent(c.root, c.gitDir)
	if markerErr != nil {
		c.refuse("aggregate-enrollment-required")
	}
	if marker || profile.Profile == AggregateReportProfile {
		canonicalRoot, rootErr := filepath.EvalSymlinks(c.root)
		if rootErr != nil {
			c.fail("aggregate-prior-evidence-drift")
		}
		bounded, err := aggregateReadFile(filepath.Join(canonicalRoot, ".corvint", "dogfood-report.json"), 4<<20)
		if err != nil {
			c.fail("aggregate-prior-evidence-drift")
		}
		value, err := ParseAggregateReport(bounded)
		if err != nil {
			c.fail("aggregate-schema-invalid", "  "+strings.ReplaceAll(tracerecordrepo.AggregateSchemaFix(err), "BASE", c.base))
		}
		if value.Base != c.base || value.Target != c.target || value.CompletionState != "complete" {
			c.fail("aggregate-binding-drift")
		}
		if value.Anchor.State == "OBSERVED" {
			if !c.anchorObserved || value.Anchor.MergeBase == nil || *value.Anchor.MergeBase != c.anchorMergeBase {
				c.fail("aggregate-binding-drift")
			}
		} else if c.anchorObserved {
			c.fail("aggregate-binding-drift")
		}
		if c.options.AggregateValidate == nil || c.options.AggregatePublish == nil {
			c.refuse("aggregate-enrollment-required")
		}
		if err = c.options.AggregateValidate(c.ctx, c.root, bounded); err != nil {
			c.fail(err.Error())
		}
		c.aggregate = &value
		c.aggregateOriginal = bounded
		return bounded
	}
	if allCaptures(reportBase, report) != c.base || allCaptures(reportTarget, report) != c.target {
		c.fail("dogfood-report-drift", "  fix: the report binds another BASE or HEAD; rerun corvint dogfood change "+c.base+" on this HEAD")
	}
	if !bytes.Contains(report, []byte(`"complete": true`)) {
		c.fail("dogfood-report-drift", "  fix: the report is not complete; resolve the rows corvint dogfood change "+c.base+" lists, then rerun it")
	}
	anchor := `  ,"anchor": {"state": "NOT_OBSERVED", "mergeBase": null}`
	if c.anchorObserved {
		anchor = `  ,"anchor": {"state": "OBSERVED", "mergeBase": "` + c.anchorMergeBase + `"}`
	}
	if !hasLine(report, anchor) {
		c.fail("dogfood-report-drift")
	}
	outcome := allCaptures(outcomeDigest, report)
	evidence := c.evidence + "/local-outcome.json"
	if actual, err := fileSHA256(evidence); outcome == "" || !isRegular(evidence) || err != nil || actual != outcome {
		c.fail("local-outcome-evidence-drift")
	}
	c.verifyAggregateOutcome(evidence)
	return report
}

// verifyAggregateOutcome reacquires an over-limit change's nonlearning
// aggregate receipt from this BASE and HEAD and requires identical bytes, so a
// self-consistent receipt for other paths cannot pass (DCW-V0-375). A legacy
// trace receipt keeps the digest binding alone.
func (c *check) verifyAggregateOutcome(evidence string) {
	defer c.clock.enter("outcome-replay", "-")()
	raw := readFile(evidence)
	if allCaptures(recordState, raw) != "aggregate-recorded" {
		return
	}
	value, err := tracerecordrepo.ParseAggregateOutcome(raw)
	if err != nil || value.Base != c.base || value.Target != c.target {
		c.fail("local-outcome-evidence-drift")
		return
	}
	expected := value.AggregateExpected
	expected.Tree = c.gitValue("rev-parse", c.target+"^{tree}")
	if _, err = tracerecordrepo.VerifyAggregateOutcome(c.ctx, c.root, expected, raw); err != nil {
		c.fail("local-outcome-evidence-drift", "  fix: the aggregate local outcome does not reproduce from "+c.base+".."+c.target+" ("+err.Error()+"); rerun corvint dogfood change "+c.base+" on this HEAD")
	}
}

// checkContextAbstention verifies the digest-bound impact abstention the report
// claims, or its absence, and reports whether one is claimed.
func (c *check) checkContextAbstention(report []byte) bool {
	digest := allCaptures(abstentionDigest, report)
	if c.aggregate != nil && c.aggregate.ContextAbstentionEvidenceSHA256 != nil {
		digest = strings.TrimPrefix(*c.aggregate.ContextAbstentionEvidenceSHA256, "sha256:")
	}
	artifact := c.evidence + "/coordination-time-impact-abstention.json"
	reason, _ := firstCapture(abstentionRow, report)
	if c.aggregate != nil {
		for _, row := range c.aggregate.Steps {
			if row.Name == "coordination-time-impact" && row.Status == "NOT_PRODUCED" {
				reason = row.Reason
			}
		}
	}
	if !impactAbstentions[reason] {
		if _, err := os.Stat(artifact); digest != "" || err == nil {
			c.fail("context-abstention-evidence-drift")
		}
		// DCW-V0-075: an admitted deadline overrun keeps the exact envelope
		// and empty stdout of its last run; it is not rerun.
		if reason == impactDeadlineReason {
			if !impactDeadlineOverrun(2, c.evidence+"/coordination-time-impact.json", c.evidence+"/coordination-time-impact.stderr") {
				c.fail("context-abstention-evidence-drift")
			}
			c.say("dogfood-check: NOTE coordination-time-impact NOT_PRODUCED %s\n", reason)
		}
		if c.aggregate == nil && !hasLine(report, `  ,"contextAbstentionEvidenceSha256": null`) {
			c.fail("dogfood-report-drift")
		}
		return false
	}
	if actual, err := fileSHA256(artifact); digest == "" || !isRegular(artifact) || err != nil || actual != digest {
		c.fail("context-abstention-evidence-drift")
	}
	argvFile := c.evidence + "/coordination-time-impact.argv"
	output := c.evidence + "/coordination-time-impact.json"
	errorFile := c.evidence + "/coordination-time-impact.stderr"
	argvSHA, _ := fileSHA256(argvFile)
	stdoutSHA, _ := fileSHA256(output)
	stderrSHA, _ := fileSHA256(errorFile)
	stderr := readFile(errorFile)
	code, _ := firstCapture(impactRefusal, stderr)
	if !isRegular(argvFile) || !isRegular(output) || !isRegular(errorFile) || !c.argvRecorded(readFile(argvFile)) ||
		!impactAbstentionStderr(stderr) || hasContent(output) || code != reason ||
		chomp(string(readFile(artifact))) != abstentionArtifact(argvSHA, c.base, reason, stderrSHA, stdoutSHA, c.target) {
		c.fail("context-abstention-evidence-drift")
	}
	c.say("dogfood-check: NOTE coordination-time-impact NOT_PRODUCED %s\n", reason)
	return true
}

// argvRecorded requires exactly the NUL-terminated argv the change recorded for
// its impact step; any executable path is admitted as argv[0].
func (c *check) argvRecorded(data []byte) bool {
	if len(data) == 0 || data[len(data)-1] != 0 {
		return false
	}
	argv := strings.Split(string(data[:len(data)-1]), "\x00")
	expected := []string{"--root", c.root, "impact", "--base", c.base, "--range-profile", "expanded-256", "--limit", "20"}
	if len(argv) != 10 || argv[0] == "" || !sameDirectory(argv[2], c.root) {
		return false
	}
	argv[2] = c.root
	return strings.Join(argv[1:], "\x00") == strings.Join(expected, "\x00")
}

// sameDirectory admits two spellings of one directory, such as /tmp and
// /private/tmp, as the scripts' `cd && pwd` normalization did.
func sameDirectory(left, right string) bool {
	if left == right {
		return true
	}
	leftResolved, leftErr := filepath.EvalSymlinks(left)
	rightResolved, rightErr := filepath.EvalSymlinks(right)
	return leftErr == nil && rightErr == nil && leftResolved == rightResolved
}

// resolveVerifiers records each verifier executable's identity.
func (c *check) resolveVerifiers() {
	var baseErr, treeErr, overrideErr error
	c.baseSHA, baseErr = fileSHA256(c.options.BaseVerifier.Path)
	c.treeSHA, treeErr = fileSHA256(c.options.TreeVerifier.Path)
	if c.options.Override != nil {
		c.overrideSHA, overrideErr = fileSHA256(c.options.Override.Path)
	}
	if baseErr != nil || treeErr != nil || overrideErr != nil {
		c.refuse("verifier-unavailable")
	}
}

func (c *check) verify(runner Runner, args ...string) verification {
	defer c.clock.enter(c.verifierPhase(runner), commandLabel(args))()
	if c.aggregate != nil {
		before, err := fileSHA256(runner.Path)
		if err != nil {
			c.refuse("verifier-unavailable")
		}
		for _, identity := range []struct{ path, sha string }{{c.options.BaseVerifier.Path, c.baseSHA}, {c.options.TreeVerifier.Path, c.treeSHA}} {
			if identity.path == runner.Path && identity.sha != before {
				c.refuse("verifier-disagreement")
			}
		}
		if c.options.Override != nil && c.options.Override.Path == runner.Path && c.overrideSHA != before {
			c.refuse("verifier-disagreement")
		}
		result, err := runAggregateVerifier(c.ctx, runner, c.root, args)
		if err != nil {
			c.refuse(err.Error())
		}
		after, err := fileSHA256(runner.Path)
		if err != nil || before != after {
			c.refuse("verifier-disagreement")
		}
		c.checkpoint()
		return result
	}
	result := runPlain(c.ctx, c.root, runner, args)
	c.checkpoint()
	return result
}

// runPlain runs one verifier command into its own buffers.
func runPlain(ctx context.Context, root string, runner Runner, args []string) verification {
	var stdout, stderr bytes.Buffer
	status := runner.Run(ctx, root, args, &stdout, &stderr)
	return verification{status, stdout.Bytes(), stderr.Bytes()}
}

// verifyPair runs args with the base and the tree verifier. For a cem status
// pair (status set) with ConcurrentVerifiers set and no aggregate enrollment
// the two run at once, each into its own buffers, and one checkpoint follows
// both; the results are compared exactly as when they run one after the other
// (DCW-V0-907). Every other pair, query abstention among them, stays
// sequential, and impact never comes here: it takes the non-blocking
// contextindex snapshot-refresh lock.
func (c *check) verifyPair(status bool, args ...string) (verification, verification) {
	if !status || !c.options.ConcurrentVerifiers || c.aggregate != nil {
		return c.verify(c.options.BaseVerifier, args...), c.verify(c.options.TreeVerifier, args...)
	}
	var base verification
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		base = runPlain(c.ctx, c.root, c.options.BaseVerifier, args)
	}()
	tree := runPlain(c.ctx, c.root, c.options.TreeVerifier, args)
	wait.Wait()
	c.checkpoint()
	return base, tree
}

func agree(left, right verification) bool {
	return left.status == right.status && bytes.Equal(left.stdout, right.stdout) && bytes.Equal(left.stderr, right.stderr)
}

// agreeImpact is agree for an impact rerun: the stderr of each is compared
// without its diagnostic snapshot-miss notice, whose duration differs on every
// run and which a verifier with a matching snapshot never writes (DCW-V0-377).
func agreeImpact(left, right verification) bool {
	return left.status == right.status && bytes.Equal(left.stdout, right.stdout) &&
		bytes.Equal(withoutImpactSnapshotNotice(left.stderr), withoutImpactSnapshotNotice(right.stderr))
}

// verifyAll runs args with the base and tree verifiers and the optional
// override, and reports the tree result and whether all of them agree. Only a
// cem status command (status set) may run its pair concurrently (DCW-V0-907).
func (c *check) verifyAll(status bool, args ...string) (verification, bool) {
	base, tree := c.verifyPair(status, args...)
	if !agree(base, tree) {
		return tree, false
	}
	if c.options.Override == nil {
		return tree, true
	}
	return tree, agree(tree, c.verify(*c.options.Override, args...))
}

// requirementIDPattern bounds a declared requirement ID before it is looked up,
// so no tab, newline or path text reaches the comparison.
var (
	requirementIDPattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*(-[A-Za-z0-9]+)+$`)
	requirementSpecPath  = regexp.MustCompile(`^docs/specs/[A-Za-z0-9][A-Za-z0-9._-]*\.md$`)
)

// maxRequirementsBytes bounds the REQUIREMENTS.tsv read; a larger index is
// treated as unreadable rather than read whole.
const maxRequirementsBytes = 16 << 20

// admitOutputChange admits DOGFOOD_OUTPUT_CHANGE only when HEAD's
// docs/specs/REQUIREMENTS.tsv maps the requirement ID to a spec that
// BASE..HEAD modifies; anything else refuses before a verifier runs
// (DCW-V0-047). The aggregate report's closed schema has no member for the
// declaration, so an enrolled aggregate check refuses it.
func (c *check) admitOutputChange() {
	id := c.options.OutputChange
	if id == "" {
		return
	}
	if c.aggregate != nil {
		c.say("dogfood-check: REFUSE output-change-aggregate-unsupported\n")
		c.say("  fix: unset DOGFOOD_OUTPUT_CHANGE; the enrolled aggregate report cannot record base-independence=NOT_PROVED (DCW-V0-047)\n")
		c.exit(2)
	}
	spec := c.requirementSpec(id)
	if spec == "" {
		c.say("dogfood-check: REFUSE output-change-requirement-unknown\n")
		c.say("  fix: set DOGFOOD_OUTPUT_CHANGE to a requirement ID that docs/specs/REQUIREMENTS.tsv at HEAD maps to its spec (DCW-V0-047)\n")
		c.exit(2)
	}
	if _, status := c.git(true, "diff", "--quiet", "--no-ext-diff", "--no-textconv", c.base, c.target, "--", spec); status != 1 {
		c.say("dogfood-check: REFUSE output-change-not-in-diff\n")
		c.say("  %s maps to %s, which %s..%s does not modify; the requirement that changes the output must change in the same change (DCW-V0-047)\n", id, spec, c.base, c.target)
		c.exit(2)
	}
	c.outputChange = id
}

// requirementSpec is the spec path HEAD's REQUIREMENTS.tsv maps id to, or
// empty when the ID is malformed, absent, or the index is unreadable.
func (c *check) requirementSpec(id string) string {
	if !requirementIDPattern.MatchString(id) || len(id) > 64 {
		return ""
	}
	object := c.target + ":docs/specs/REQUIREMENTS.tsv"
	size, status := c.git(true, "cat-file", "-s", object)
	if length, err := strconv.Atoi(chomp(size)); status != 0 || err != nil || length > maxRequirementsBytes {
		return ""
	}
	index, status := c.git(true, "cat-file", "blob", object)
	if status != 0 {
		return ""
	}
	for _, line := range strings.Split(index, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) >= 2 && fields[0] == id {
			if requirementSpecPath.MatchString(fields[1]) {
				return fields[1]
			}
			return ""
		}
	}
	return ""
}

// verifyStatus is verifyAll for `cem status`. Under an admitted output-change
// declaration the tree verifier fills both roles, the base verifier still
// runs, and the digests of both stdouts are kept: the change proves its own
// verifier deterministic, not independent of its base (DCW-V0-047). A base
// verifier that refuses the read (an exit status other than 0 or 1) produced
// no status, so its refusal code is kept instead of a digest of its absent
// output (V1-1307, decision 0535).
//
// Verifiers that agree on the protocol budget's unsupported-resource-limit
// refusal are rerun once with the opt-in scaled bound, and both must agree on
// that result too: a base verifier without the flag refuses it, so the check
// fails closed as a disagreement (DCW-V0-373). The enrolled aggregate worker
// admits only the protocol argv, so it keeps the refusal.
func (c *check) verifyStatus(args ...string) (verification, bool) {
	result, agreed := c.verifyStatusOnce(args...)
	if agreed && c.aggregate == nil && resourceLimited(result.stdout) {
		return c.verifyStatusOnce(withScaledOperations(args)...)
	}
	return result, agreed
}

func (c *check) verifyStatusOnce(args ...string) (verification, bool) {
	if c.outputChange == "" {
		return c.verifyAll(true, args...)
	}
	base, tree, agreed := c.verifyDeclared(args...)
	c.baseStatusSHA, c.baseRefusal, c.treeStatusSHA = sha256Hex(base.stdout), "", sha256Hex(tree.stdout)
	if base.status != 0 && base.status != 1 {
		c.baseStatusSHA, c.baseRefusal = "", failureReason(base.stderr, base.status)
	}
	return tree, agreed
}

// verifyDeclared is verifyAll under an admitted output-change declaration: the
// base verifier still runs, the tree verifier fills both roles, and it must
// agree with itself and with any override (DCW-V0-047).
func (c *check) verifyDeclared(args ...string) (verification, verification, bool) {
	base, tree := c.verifyPair(true, args...)
	if !agree(tree, c.verify(c.options.TreeVerifier, args...)) {
		return base, tree, false
	}
	if c.options.Override == nil {
		return base, tree, true
	}
	return base, tree, agree(tree, c.verify(*c.options.Override, args...))
}

// baseIndependence is the fields an admitted output change records on the
// SUMMARY, NOTE and seal PASS lines, or empty before its `cem status` ran.
func (c *check) baseIndependence() string {
	if c.outputChange == "" || c.treeStatusSHA == "" {
		return ""
	}
	base := "base=sha256:" + c.baseStatusSHA
	if c.baseRefusal != "" {
		base = "base=NOT_PRODUCED base-refusal=" + c.baseRefusal
	}
	return "base-independence=NOT_PROVED " + base + " tree=sha256:" + c.treeStatusSHA + " declared=" + c.outputChange
}

func (c *check) verifyAbstention() {
	args := []string{"impact", "--base", c.base, "--range-profile", "expanded-256", "--limit", "20"}
	base := c.verify(c.options.BaseVerifier, args...)
	tree := c.verify(c.options.TreeVerifier, args...)
	recorded := withoutImpactSnapshotNotice(readFile(c.evidence + "/coordination-time-impact.stderr"))
	if !agreeImpact(base, tree) || tree.status != 2 || len(tree.stdout) > 0 || !bytes.Equal(withoutImpactSnapshotNotice(tree.stderr), recorded) {
		c.fail("verifier-disagreement")
	}
	if c.options.Override != nil && !agreeImpact(tree, c.verify(*c.options.Override, args...)) {
		c.fail("verifier-disagreement")
	}
}

func missingLines(report []byte) []string {
	lines := []string{}
	for _, line := range textLines(report) {
		if match := missingRow.FindStringSubmatch(line); match != nil {
			lines = append(lines, "  missing "+match[1]+": "+match[2])
		}
	}
	return lines
}

// verifyBinding verifies the OCM aggregate (DCW-V0-160), requires the verifiers to agree on the
// CEM policy, records their identities in the report and prints the results.
func (c *check) verifyBinding(report []byte) int {
	if !isRegular(c.root + "/" + c.cemPath) {
		c.fail("cem-map-missing", missingLines(report)...)
	}
	intentSnapshot := c.root + "/.corvint/change.ocm-intents"
	aggregate := c.root + "/.corvint/change.ocm-status.json"
	if !isRegular(intentSnapshot) || !isRegular(aggregate) {
		c.fail("missing-intent-scope")
	}
	// The declaration and the report's ocmStatus must agree, so a swapped
	// snapshot cannot skip OCM verification (DCW-V0-024).
	noIntent := bytes.Equal(readFile(intentSnapshot), noIntentManifest)
	reportedNoIntent := hasLine(report, noIntentOCMStatus)
	if c.aggregate != nil {
		var status struct {
			State  string `json:"state"`
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(c.aggregate.OCMStatus, &status)
		reportedNoIntent = status.State == "NOT_ASSESSED" && status.Reason == "no-intent-declared"
	}
	if noIntent != reportedNoIntent {
		c.fail("dogfood-report-drift")
	}
	ocmLine := []byte("dogfood-check: NOTE intent-linkage NOT_ASSESSED no-intent-declared\n")
	bootstrap := 0
	intents := []string{}
	if !noIntent {
		ocmLine = c.verifyOCM(aggregate)
		intents = readLines(readFile(intentSnapshot))
		bootstrap = bootstrapUnknowns(&c.flow, intents)
	}
	// The committed map carries each reason an author recorded for a hunk kept
	// unknown, so the check admits them as the change did (DCW-V0-019).
	reasoned, allowance := c.flow.reasonedUnknowns(c.root+"/"+c.cemPath, intents, bootstrap > 0)
	policyMatches := bytes.Contains(report, []byte(`  ,"dogfoodPolicy": {"bootstrapUnknown": `+strconv.Itoa(bootstrap)+`, "maximumUnknownAfterBootstrap": 0}`))
	if c.aggregate != nil {
		policyMatches = c.aggregate.DogfoodPolicy.BootstrapUnknown == bootstrap && c.aggregate.DogfoodPolicy.MaximumUnknownAfterBootstrap == 0
	}
	if !policyMatches {
		_ = c.record(true, bootstrap)
		c.fail("dogfood-report-drift")
	}
	// The check bounds the deferrals by the record the range adds, and refuses
	// one that also names a kept intent, as the change does (DCW-V0-371).
	deferrals, deferred := c.flow.recordedDeferrals(c.base, c.target)
	if kept := slices.DeleteFunc(slices.Clone(deferred), func(path string) bool { return !slices.Contains(intents, path) }); len(kept) > 0 {
		c.fail("deferred-intent-kept", "  paths="+strings.Join(kept, ","),
			"  fix: a scope is either kept in the intent manifest or recorded as deferred; rebind without one of them (DCW-V0-371)")
	}
	args := statusArguments(c.root, c.base, c.target, bootstrap+allowance, deferrals)
	args[3] = c.cemPath
	cem, agreed := c.verifyStatus(args...)
	if !agreed {
		_ = c.record(false, bootstrap)
		c.fail("verifier-disagreement")
	}
	if c.record(true, bootstrap) != nil {
		c.fail("dogfood-report-write")
	}
	cemLine := firstLine(cem.stdout)
	if cem.status != 0 {
		// A failing policy keeps its whole status JSON: it is the repair detail.
		_, _ = c.stdout.Write(cemLine)
		c.fail("cem-policy", missingLines(readFile(c.report))...)
	}
	detail := c.evidence + "/dogfood-check.stdout"
	full := append(append([]byte{}, cemLine...), ocmLine...)
	if noIntent {
		full = cemLine
	}
	if c.options.Verbose || os.MkdirAll(c.evidence, 0o700) != nil || removeFile(detail) != nil || writePrivate(detail, full) != nil {
		// Verbose, or with no detail file to point at, the former full output stands.
		_, _ = c.stdout.Write(cemLine)
		_, _ = c.stdout.Write(ocmLine)
		if c.baseIndependence() != "" {
			fmt.Fprintf(c.stdout, "dogfood-check: NOTE %s\n", c.baseIndependence())
		}
		writeLines(c.stderr, c.tierWarnings(cemLine))
		fmt.Fprintf(c.stdout, "dogfood-check: PASS\n")
		return 0
	}
	var coverage *changeCoverage
	if !noIntent {
		coverage = c.changeCoverage(c.root+"/"+c.cemPath, intents)
	}
	fmt.Fprintf(c.stdout, "%s\n", checkSummary(cemLine, ocmLine, noIntent, reasoned, coverage, c.baseIndependence(), detail))
	if note := coverage.unlinkedNote(); note != "" {
		c.say("%s\n", note)
	}
	if note := coverage.outsideNote(); note != "" {
		c.say("%s\n", note)
	}
	if noIntent {
		_, _ = c.stdout.Write(ocmLine)
	}
	writeLines(c.stderr, c.tierWarnings(cemLine))
	fmt.Fprintf(c.stdout, "dogfood-check: PASS\n")
	return 0
}

// checkSummary is the one PASS line that stands in for the CEM and OCM status
// JSON (DCW-V0-033): their states and counts, the report, and the detail file
// that holds the full JSON. A field the JSON does not carry prints NOT_OBSERVED.
// unknownShare is unknown over material (non-mechanical) hunks, the measure of
// the CEM kill condition, and reasonedUnknown the unknown hunks kept with a
// recorded reason (DCW-V0-019). independence, when not empty, is the
// base-independence=NOT_PROVED fields of an admitted output change (DCW-V0-047).
// coverage fills the requirements, linked and unlinked members: the declared
// requirements the change cites or edits, not the aggregate's whole intent
// scope (OCM-V0-019).
func checkSummary(cemLine, ocmLine []byte, noIntent bool, reasoned int, coverage *changeCoverage, independence, detail string) string {
	var cem struct {
		State  *string `json:"state"`
		Counts *struct {
			Total      *int `json:"total"`
			Supported  *int `json:"supported"`
			Unknown    *int `json:"unknown"`
			Mechanical *int `json:"mechanical"`
		} `json:"counts"`
	}
	_ = json.Unmarshal(cemLine, &cem)
	fields := []string{"cem=" + summaryString(cem.State)}
	if counts := cem.Counts; counts != nil {
		fields = append(fields, "hunks="+summaryInt(counts.Total), "supported="+summaryInt(counts.Supported), "unknown="+summaryInt(counts.Unknown), "mechanical="+summaryInt(counts.Mechanical),
			"unknownShare="+unknownShare(counts.Total, counts.Unknown, counts.Mechanical))
	} else {
		fields = append(fields, "hunks=NOT_OBSERVED", "unknownShare=NOT_OBSERVED")
	}
	fields = append(fields, "reasonedUnknown="+strconv.Itoa(reasoned))
	if noIntent {
		fields = append(fields, "ocm=NOT_ASSESSED")
	} else {
		var ocm struct {
			Aggregate *struct {
				State *string `json:"state"`
			} `json:"aggregate"`
		}
		_ = json.Unmarshal(ocmLine, &ocm)
		if ocm.Aggregate == nil {
			fields = append(fields, "ocm=NOT_OBSERVED")
		} else {
			fields = append(fields, "ocm="+summaryString(ocm.Aggregate.State))
			fields = append(fields, coverage.summaryFields()...)
		}
	}
	if independence != "" {
		fields = append(fields, independence)
	}
	fields = append(fields, "report=.corvint/dogfood-report.json", "detail="+detail)
	return "dogfood-check: SUMMARY " + strings.Join(fields, " ")
}

// summaryString admits one state token of letters, digits, '-' and '_';
// anything else is NOT_OBSERVED, so the line cannot be forged or split.
func summaryString(value *string) string {
	if value == nil || *value == "" || len(*value) > 64 || strings.TrimFunc(*value, func(r rune) bool {
		return r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
	}) != "" {
		return "NOT_OBSERVED"
	}
	return *value
}

// unknownShare is UNKNOWN/MATERIAL, material hunks being the total less the
// mechanical ones; it is NOT_OBSERVED when a count is absent or inconsistent.
func unknownShare(total, unknown, mechanical *int) string {
	if total == nil || unknown == nil || mechanical == nil || *unknown < 0 || *mechanical < 0 || *unknown+*mechanical > *total {
		return "NOT_OBSERVED"
	}
	return strconv.Itoa(*unknown) + "/" + strconv.Itoa(*total-*mechanical)
}

func summaryInt(value *int) string {
	if value == nil {
		return "NOT_OBSERVED"
	}
	return strconv.Itoa(*value)
}

// verifyOCM verifies the OCM aggregate the change published and returns its
// first line. The aggregate is a function of the target tree and its frozen
// scope set, so it is computed once, by the tree verifier, and that one result
// stands for every verifier role, declared output change or not (DCW-V0-160,
// V1-2274; it was run by each verifier, as for `cem status`). Its byte
// comparison with the aggregate `dogfood change` published, which the change's
// own executable computed, stays the second computation of it.
func (c *check) verifyOCM(aggregate string) []byte {
	var ocm verification
	if c.sealed {
		result, err := dogfoodocm.Status(c.ctx, dogfoodocm.Options{Root: c.root, CEMPath: c.cemPath, ExpectedBase: c.base, Target: c.target})
		if err != nil {
			c.fail("ocm-policy")
		}
		raw, err := json.Marshal(result)
		if err != nil {
			c.fail("ocm-policy")
		}
		ocm = verification{stdout: append(raw, '\n')}
	} else {
		ocm = c.verify(c.options.TreeVerifier, "dogfood-ocm", "status", "--expected-base", c.base, "--target", c.target)
	}
	if ocm.status != 0 {
		reason, _ := firstCapture(codePattern, ocm.stderr)
		if reason == "" {
			reason = "ocm-policy"
		}
		_ = c.record(true, 0)
		c.fail(reason)
	}
	if !bytes.Equal(ocm.stdout, readFile(aggregate)) {
		_ = c.record(true, 0)
		c.fail("intent-scope-drift", "  fix: the OCM maps changed after corvint dogfood change; rerun corvint dogfood change "+c.base)
	}
	return firstLine(ocm.stdout)
}

// record replaces the report's single dogfoodCheck line with the verifier
// identities and their agreement.
func (c *check) record(agreed bool, bootstrap int) error {
	if c.sealed {
		// The parent report remains immutable provenance. A continuation records
		// this actual verifier comparison separately at the current seal.
		raw, err := json.Marshal(struct {
			Profile        string `json:"profile"`
			Target         string `json:"target"`
			EvidenceTarget string `json:"evidenceTarget"`
			Map            string `json:"map"`
			BaseVerifier   string `json:"baseVerifierSha256"`
			TreeVerifier   string `json:"treeVerifierSha256"`
			OutputsAgree   bool   `json:"outputsAgree"`
		}{"corvint-sealed-check/0", c.sealedTarget, c.target, c.cemPath, c.baseSHA, c.treeSHA, agreed})
		if err != nil {
			return err
		}
		return writePrivate(c.evidence+"/dogfood-sealed-check.json", append(raw, '\n'))
	}

	if c.aggregate != nil {
		value := *c.aggregate
		result := &AggregateCheckResult{BaseVerifierSHA256: "sha256:" + c.baseSHA, TreeVerifierSHA256: "sha256:" + c.treeSHA, BootstrapUnknown: bootstrap, OutputsAgree: agreed}
		if c.overrideSHA != "" {
			identity := "sha256:" + c.overrideSHA
			result.OverrideVerifierSHA256 = &identity
		}
		value.DogfoodCheck = result
		raw, err := EncodeAggregateReport(value)
		if err != nil {
			return err
		}
		c.aggregatePrepared = raw
		return nil
	}
	override := "null"
	if c.overrideSHA != "" {
		override = `"sha256:` + c.overrideSHA + `"`
	}
	replacement := fmt.Sprintf(`  ,"dogfoodCheck": {"baseVerifierSha256": "sha256:%s", "treeVerifierSha256": "sha256:%s", "overrideVerifierSha256": %s, "bootstrapUnknown": %d, "maximumUnknownAfterBootstrap": 0, "outputsAgree": %t}`,
		c.baseSHA, c.treeSHA, override, bootstrap, agreed)
	if c.baseIndependence() != "" {
		// outputsAgree then means the tree verifier agreed with itself and any
		// override, as in LCP-V0-014; the member says what stays unproved (DCW-V0-047).
		base := fmt.Sprintf(`"baseStatusStdoutSha256": "sha256:%s"`, c.baseStatusSHA)
		if c.baseRefusal != "" {
			base = fmt.Sprintf(`"baseStatus": "NOT_PRODUCED", "baseRefusal": "%s"`, c.baseRefusal)
		}
		replacement = strings.TrimSuffix(replacement, "}") + fmt.Sprintf(`, "baseIndependence": {"state": "NOT_PROVED", "declared": "%s", %s, "treeStatusStdoutSha256": "sha256:%s"}}`,
			c.outputChange, base, c.treeStatusSHA)
	}
	data, err := os.ReadFile(c.report)
	if err != nil {
		return err
	}
	lines := textLines(data)
	replaced := 0
	for index, line := range lines {
		if strings.HasPrefix(line, `  ,"dogfoodCheck": `) {
			lines[index] = replacement
			replaced++
		}
	}
	if replaced != 1 {
		return fmt.Errorf("report has %d dogfoodCheck lines", replaced)
	}
	// Stage in the Git directory and rename, so an interrupted write never
	// leaves a truncated report or an unignored file in the worktree.
	staged := filepath.Join(c.gitDir, "corvint-dogfood-report.json.tmp")
	if err := os.WriteFile(staged, []byte(strings.Join(lines, "\n")+"\n"), 0o666); err != nil {
		return err
	}
	return os.Rename(staged, c.report)
}

func (c *check) unboundNotObserved(reason string) {
	c.say("dogfood-check: NOTE unbound-commits NOT_OBSERVED %s\n", reason)
}

// previousBinding is the CEM committed at BASE, else the bind commit under the
// newest seal reachable from BASE (DOGFOOD-014).
func (c *check) previousBinding() (string, bool) {
	if c.gitSucceeds("cat-file", "-e", c.base+":.corvint/change.cem.json") {
		return c.base, true
	}
	seals, _ := c.git(false, "rev-list", "--no-merges", "--max-count=256", c.base, "--", ".corvint/changes")
	for _, seal := range readLines([]byte(seals)) {
		if c.isSealCommit(seal) {
			parent, status := c.git(false, "rev-parse", seal+"^1")
			return chomp(parent), status == 0
		}
	}
	return "", false
}

func (c *check) cemBaseRevision(commit string) (string, bool) {
	sidecar, status := c.git(true, "show", commit+":.corvint/change.cem.json")
	if status != 0 {
		return "", false
	}
	value, ok := sidecarBaseRevision([]byte(sidecar))
	if !ok || !fullRevision.MatchString(value) {
		return "", false
	}
	_, status = c.git(false, "merge-base", "--is-ancestor", value, commit)
	return value, status == 0
}

// sidecarBaseRevision reads the top-level baseRevision of a committed CEM
// sidecar whatever its JSON layout, so the compact cem/1.0 map and an older
// indented one both resolve (V1-1326). A document that is not one JSON object,
// or that repeats or lacks the member, has no base.
func sidecarBaseRevision(data []byte) (string, bool) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return "", false
	}
	value, found := "", false
	for decoder.More() {
		token, err := decoder.Token()
		key, isKey := token.(string)
		if err != nil || !isKey {
			return "", false
		}
		if key != "baseRevision" {
			var skipped json.RawMessage
			if decoder.Decode(&skipped) != nil {
				return "", false
			}
			continue
		}
		if found || decoder.Decode(&value) != nil {
			return "", false
		}
		found = true
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return "", false
	}
	if _, err := decoder.Token(); err != io.EOF {
		return "", false
	}
	return value, found
}

func nonEmpty(lines []string) []string {
	kept := []string{}
	for _, line := range lines {
		if line != "" {
			kept = append(kept, line)
		}
	}
	return kept
}

// reportUnboundCommits notes, without failing, non-merge commits after the
// base's committed CEM base that no CEM committed in that window binds, and
// separately those bound only by a retroactive binding commit (docs/DOGFOOD.md §4).
// windowNote prints one NOTE line for the commits of the previous-binding
// window, which all lie before BASE and so outside this change (DCW-V0-343).
// The line carries the count, the newest and oldest commit and the private file
// that lists every commit, newest first, one "<label> SHA..." line each. When
// that file cannot be written the commits are listed inline instead, so the
// list is never dropped. window-start names the previous binding whose CEM
// base starts the window, so a window that starts before a rebased lane's
// BASE is explained (DCW-V0-903).
func (c *check) windowNote(note, label string, commits []string, previous, previousBase string) {
	if len(commits) == 0 {
		return
	}
	list := c.evidence + "/dogfood-check." + note
	var body strings.Builder
	for _, commit := range commits {
		body.WriteString(label + " " + commit + "\n")
	}
	newest := strings.Fields(commits[0])[0]
	oldest := strings.Fields(commits[len(commits)-1])[0]
	if os.MkdirAll(c.evidence, 0o700) == nil && removeFile(list) == nil && writePrivate(list, []byte(body.String())) == nil {
		c.say("dogfood-check: NOTE %s count=%d window=%s..%s window-start=cem-base-of:%s newest=%s oldest=%s before-base list=%s\n", note, len(commits), previousBase, c.base, previous, newest, oldest, list)
		return
	}
	c.say("dogfood-check: NOTE %s count=%d window=%s..%s window-start=cem-base-of:%s\n", note, len(commits), previousBase, c.base, previous)
	for _, commit := range commits {
		c.say("  %s %s\n", label, commit)
	}
}

func (c *check) reportUnboundCommits() {
	// A former run's commit lists must not outlive the NOTE that named them.
	for _, note := range []string{"unbound-commits", "retroactive-bound-commits"} {
		_ = removeFile(c.evidence + "/dogfood-check." + note)
	}
	previous, ok := c.previousBinding()
	if !ok {
		c.unboundNotObserved("previous-cem-absent")
		return
	}
	previousBase, ok := c.cemBaseRevision(previous)
	if !ok {
		c.unboundNotObserved("previous-cem-base-unavailable")
		return
	}
	window, status := c.git(false, "rev-list", "--no-merges", "--max-count=257", c.base, "^"+previousBase)
	if status != 0 {
		c.unboundNotObserved("window-unavailable")
		return
	}
	commits := nonEmpty(textLines([]byte(chomp(window))))
	if len(commits) > 256 {
		c.unboundNotObserved("window-exceeds-256-commits")
		return
	}
	covered := map[string]bool{}
	retroactiveBinding := map[string]string{}
	sidecars, _ := c.git(false, "rev-list", "--full-history", "--no-merges", c.base, "^"+previousBase, "--", ".corvint/change.cem.json")
	for _, sidecar := range readLines([]byte(sidecars)) {
		if c.isSealCommit(sidecar) {
			covered[sidecar] = true
			continue
		}
		sidecarBase, ok := c.cemBaseRevision(sidecar)
		if !ok {
			c.unboundNotObserved("window-cem-base-unavailable")
			return
		}
		bound, _ := c.git(false, "rev-list", sidecar, "^"+sidecarBase, "^"+previousBase)
		trailer, _ := c.git(false, "log", "-1", "--format=%(trailers:key=Corvint-Dogfood-Binding,valueonly)", sidecar)
		retroactive := chomp(trailer) == "retroactive"
		for _, commit := range strings.Split(chomp(bound), "\n") {
			if !retroactive {
				covered[commit] = true
				continue
			}
			// The newest retroactive binding of a commit names it; an empty bound
			// keys the sidecar itself with no binding, as the former awk did.
			key, binding := commit, sidecar
			if commit == "" {
				key, binding = sidecar, ""
			}
			if _, seen := retroactiveBinding[key]; !seen {
				retroactiveBinding[key] = binding
			}
		}
	}
	unbound, retroactive := []string{}, []string{}
	for _, commit := range commits {
		if covered[commit] {
			continue
		}
		if binding, found := retroactiveBinding[commit]; found {
			retroactive = append(retroactive, commit+" binding="+binding)
		} else {
			unbound = append(unbound, commit)
		}
	}
	c.windowNote("unbound-commits", "unbound", unbound, previous, previousBase)
	c.windowNote("retroactive-bound-commits", "retroactive", retroactive, previous, previousBase)
}

func (c *check) enableAggregateCapture() {
	if c.options.AggregatePublish == nil {
		return
	}
	raw, err := aggregateReadFile(filepath.Join(c.root, ".corvint", "dogfood-report.json"), 4<<20)
	if err != nil {
		return
	}
	if _, err = ParseAggregateReport(raw); err != nil {
		return
	}
	if c.options.AggregateBegin == nil {
		c.refuse("aggregate-enrollment-required")
	}
	stage, err := c.options.AggregateBegin(c.ctx, c.root, raw)
	if err != nil {
		c.refuse(err.Error())
	}
	if stage == nil || stage.Stdout == nil || stage.Stderr == nil || stage.Finish == nil {
		c.refuse("aggregate-publication-failed")
	}
	c.aggregateCapture = stage
	c.aggregateOriginal = raw
	if c.stdout == nil {
		c.stdout = io.Discard
	}
	if c.stderr == nil {
		c.stderr = io.Discard
	}
	c.aggregateSavedStdout, c.aggregateSavedStderr = c.stdout, c.stderr
	c.stdout = io.MultiWriter(stage.Stdout, c.stdout)
	c.stderr = io.MultiWriter(stage.Stderr, c.stderr)
}
func (c *check) finishAggregateCapture(code int, runErr error) error {
	if c.aggregateCapture == nil {
		return dogfoodoperation.Check(c.ctx)
	}
	// Closing an admitted observation is independent of current source validity,
	// cancellation and HOLD. It starts no process and publishes no success.
	stage := c.aggregateCapture
	capture, closeErr := stage.Finish(code, runErr)
	c.stdout, c.stderr = c.aggregateSavedStdout, c.aggregateSavedStderr
	c.aggregateCapture = nil
	if err := dogfoodoperation.Check(c.ctx); err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if runErr != nil {
		return runErr
	}
	if err := c.ctx.Err(); err != nil {
		return err
	}
	if code != 0 || capture.Failed {
		return nil
	}
	if c.aggregate == nil || c.options.AggregatePublish == nil {
		return fmt.Errorf("aggregate-publication-failed")
	}
	capture.OriginalReport, capture.ProposedReport = c.aggregateOriginal, c.aggregatePrepared
	if err := c.options.AggregatePublish(c.ctx, c.root, capture); err != nil {
		if stage.RetainFailure != nil {
			return errors.Join(err, stage.RetainFailure(err))
		}
		return err
	}
	return nil
}
