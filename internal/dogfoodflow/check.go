package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// CheckOptions are the inputs of `corvint dogfood check` and `dogfood seal`.
// The base and tree verifiers must be distinct identities only when the caller
// has them; a portable check may pass the running binary for both.
type CheckOptions struct {
	Root         string
	Base         string
	BaseVerifier Runner
	TreeVerifier Runner
	Override     *Runner
	Exception    string
}

type check struct {
	flow
	options     CheckOptions
	stdout      io.Writer
	report      string
	evidence    string
	baseSHA     string
	treeSHA     string
	overrideSHA string
}

type verification struct {
	status         int
	stdout, stderr []byte
}

// Check verifies the bound change against the report, as script/dogfood-check.sh did.
func Check(ctx context.Context, options CheckOptions, stdout, stderr io.Writer) (int, error) {
	run := newCheck(ctx, "dogfood-check", options, stdout, stderr)
	return boundary(ctx, run.run, func() {})
}

// Seal checks the bound change, then moves its CEM out of the one shared
// tracked path in a rename-only commit (docs/DOGFOOD.md §4, DOGFOOD-013).
func Seal(ctx context.Context, options CheckOptions, stdout, stderr io.Writer) (int, error) {
	run := newCheck(ctx, "dogfood-check", options, stdout, stderr)
	return boundary(ctx, func() int {
		// The check's own exits, including its no-change PASS, return here as the
		// former `dogfood-check.sh || exit` did.
		code, err := boundary(ctx, run.run, func() {})
		if err != nil {
			run.exit(-1)
		}
		if code != 0 {
			return code
		}
		run.prefix = "dogfood-seal"
		return run.seal()
	}, func() {})
}

func newCheck(ctx context.Context, prefix string, options CheckOptions, stdout, stderr io.Writer) *check {
	return &check{flow: flow{ctx: ctx, prefix: prefix, root: options.Root, stderr: stderr}, options: options, stdout: stdout}
}

func (c *check) seal() int {
	bind := c.gitValue("rev-parse", "HEAD^{commit}")
	if !c.gitSucceeds("cat-file", "-e", bind+":.corvint/change.cem.json") {
		c.refuse("nothing-to-seal")
	}
	sealed := ".corvint/changes/" + bind + ".cem.json"
	_ = os.MkdirAll(c.root+"/.corvint/changes", 0o777)
	if c.gitPassthrough("mv", ".corvint/change.cem.json", sealed) != 0 {
		c.exit(2)
	}
	if c.gitPassthrough("commit", "-q", "-m", "chore: seal change evidence") != 0 {
		c.exit(2)
	}
	fmt.Fprintf(c.stdout, "dogfood-seal: PASS sealed=%s\n", sealed)
	return 0
}

// gitPassthrough runs one Git command whose output reaches the caller.
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
	c.exit(1)
}

func (c *check) run() int {
	c.requireRoot()
	c.base = c.gitValue("rev-parse", c.options.Base+"^{commit}")
	c.target = c.gitValue("rev-parse", "HEAD^{commit}")
	c.gitDir = c.gitValue("rev-parse", "--absolute-git-dir")
	c.report = c.root + "/.corvint/dogfood-report.json"
	c.evidence = c.gitDir + "/corvint"
	c.resolveAnchor()
	if c.isSealCommit(c.target) {
		c.say("dogfood-check: REFUSE sealed-head\n")
		c.say("  HEAD only seals the bound CEM; check its parent: git checkout --detach HEAD^\n")
		c.exit(2)
	}
	c.requireCleanChange()
	c.reportUnboundCommits()
	if c.options.Exception != "" {
		c.say("dogfood-check: FAIL exception-contract-undefined\n")
		c.exit(2)
	}
	report := c.checkReport()
	abstention := c.checkContextAbstention(report)
	c.resolveVerifiers()
	if abstention {
		c.verifyAbstention()
	}
	return c.verifyBinding(report)
}

// requireCleanChange refuses an uncommitted worktree and passes a clean one with
// no change between BASE and HEAD.
func (c *check) requireCleanChange() {
	_, diffStatus := c.git(false, "diff", "--quiet", c.base, c.target, "--", ".")
	if diffStatus > 1 {
		c.exit(2)
	}
	worktree := c.gitValue("status", "--porcelain", "--untracked-files=all")
	if worktree != "" {
		refusal := "dirty-worktree"
		if diffStatus == 0 {
			refusal = "uncommitted-change-not-in-base-target"
		}
		c.say("dogfood-check: REFUSE %s\n", refusal)
		c.say("  required order: commit the change; corvint dogfood change <sha>; commit .corvint/change.cem.json; corvint dogfood change <sha>; corvint dogfood check <sha>\n")
		c.exit(2)
	}
	if diffStatus == 0 {
		fmt.Fprintf(c.stdout, "dogfood-check: PASS no-change-clean-worktree\n")
		c.exit(0)
	}
}

var (
	reportBase        = regexp.MustCompile(`^  "base": "([0-9a-f]*)",$`)
	reportTarget      = regexp.MustCompile(`^  "target": "([0-9a-f]*)",$`)
	outcomeDigest     = regexp.MustCompile(`^  ,"localOutcomeEvidenceSha256": "sha256:([0-9a-f]{64})"$`)
	abstentionDigest  = regexp.MustCompile(`^  ,"contextAbstentionEvidenceSha256": "sha256:([0-9a-f]{64})"$`)
	missingRow        = regexp.MustCompile(`^.*"name": "([^"]*)", "status": "NOT_PRODUCED", "reason": "([^"]*)".*$`)
	abstentionRow     = regexp.MustCompile(`"name": "prechange-impact", "status": "NOT_PRODUCED", "reason": "([^"]*)"`)
	cemBaseRevisionRE = regexp.MustCompile(`^  "baseRevision": "([0-9a-f]{40})",$`)
	fullRevision      = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// checkReport requires a complete report bound to this BASE, HEAD, anchor and
// local outcome, and returns its bytes.
func (c *check) checkReport() []byte {
	if !isRegular(c.report) {
		lines := []string{"  fix: run corvint dogfood change " + c.base + " on this HEAD until it reports complete"}
		// A reviewer's clone of a bind commit never has the author's private report (DCW-V0-017).
		if c.gitSucceeds("cat-file", "-e", c.target+":.corvint/change.cem.json") {
			lines = append(lines, "  review: a reviewer without the author report: verifier agreement is author-only evidence (docs/DOGFOOD.md step 11); verify the bound CEM instead: corvint cem verify --map .corvint/change.cem.json --expected-base "+c.base+" --target "+c.target)
		}
		c.fail("dogfood-report-missing", lines...)
	}
	report := readFile(c.report)
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
	return report
}

// checkContextAbstention verifies the digest-bound impact abstention the report
// claims, or its absence, and reports whether one is claimed.
func (c *check) checkContextAbstention(report []byte) bool {
	digest := allCaptures(abstentionDigest, report)
	artifact := c.evidence + "/prechange-impact-abstention.json"
	reason, _ := firstCapture(abstentionRow, report)
	if !impactAbstentions[reason] {
		if _, err := os.Stat(artifact); digest != "" || err == nil {
			c.fail("context-abstention-evidence-drift")
		}
		if !hasLine(report, `  ,"contextAbstentionEvidenceSha256": null`) {
			c.fail("dogfood-report-drift")
		}
		return false
	}
	if actual, err := fileSHA256(artifact); digest == "" || !isRegular(artifact) || err != nil || actual != digest {
		c.fail("context-abstention-evidence-drift")
	}
	argvFile := c.evidence + "/prechange-impact.argv"
	output := c.evidence + "/prechange-impact.json"
	errorFile := c.evidence + "/prechange-impact.stderr"
	argvSHA, _ := fileSHA256(argvFile)
	stdoutSHA, _ := fileSHA256(output)
	stderrSHA, _ := fileSHA256(errorFile)
	stderr := readFile(errorFile)
	code, _ := firstCapture(impactRefusal, stderr)
	if !isRegular(argvFile) || !isRegular(output) || !isRegular(errorFile) || !c.argvRecorded(readFile(argvFile)) ||
		bytes.Count(stderr, []byte("\n")) != 1 || hasContent(output) || code != reason ||
		chomp(string(readFile(artifact))) != abstentionArtifact(argvSHA, c.base, reason, stderrSHA, stdoutSHA, c.target) {
		c.fail("context-abstention-evidence-drift")
	}
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
	var stdout, stderr bytes.Buffer
	status := runner.Run(c.ctx, c.root, args, &stdout, &stderr)
	c.checkpoint()
	return verification{status, stdout.Bytes(), stderr.Bytes()}
}

func agree(left, right verification) bool {
	return left.status == right.status && bytes.Equal(left.stdout, right.stdout) && bytes.Equal(left.stderr, right.stderr)
}

// verifyAll runs args with the base and tree verifiers and the optional
// override, and reports the tree result and whether all of them agree.
func (c *check) verifyAll(args ...string) (verification, bool) {
	base := c.verify(c.options.BaseVerifier, args...)
	tree := c.verify(c.options.TreeVerifier, args...)
	if !agree(base, tree) {
		return tree, false
	}
	if c.options.Override == nil {
		return tree, true
	}
	return tree, agree(tree, c.verify(*c.options.Override, args...))
}

func (c *check) verifyAbstention() {
	args := []string{"impact", "--base", c.base, "--range-profile", "expanded-256", "--limit", "20"}
	base := c.verify(c.options.BaseVerifier, args...)
	tree := c.verify(c.options.TreeVerifier, args...)
	if !agree(base, tree) || tree.status != 2 || len(tree.stdout) > 0 || !bytes.Equal(tree.stderr, readFile(c.evidence+"/prechange-impact.stderr")) {
		c.fail("verifier-disagreement")
	}
	if c.options.Override != nil && !agree(tree, c.verify(*c.options.Override, args...)) {
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

// verifyBinding requires the verifiers to agree on the OCM aggregate and the
// CEM policy, records their identities in the report and prints the results.
func (c *check) verifyBinding(report []byte) int {
	if !isRegular(c.root + "/.corvint/change.cem.json") {
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
	if noIntent != hasLine(report, noIntentOCMStatus) {
		c.fail("dogfood-report-drift")
	}
	ocmLine := []byte("dogfood-check: NOTE intent-linkage NOT_ASSESSED no-intent-declared\n")
	bootstrap := 0
	if !noIntent {
		ocmLine = c.verifyOCM(aggregate)
		bootstrap = bootstrapUnknowns(&c.flow, readLines(readFile(intentSnapshot)))
	}
	if !bytes.Contains(report, []byte(`  ,"dogfoodPolicy": {"bootstrapUnknown": `+strconv.Itoa(bootstrap)+`, "maximumUnknownAfterBootstrap": 0}`)) {
		_ = c.record(true, bootstrap)
		c.fail("dogfood-report-drift")
	}
	cem, agreed := c.verifyAll("cem", "status", "--map", ".corvint/change.cem.json", "--expected-base", c.base, "--target", c.target,
		"--max-unknown", strconv.Itoa(bootstrap), "--max-mechanical", "0")
	if !agreed {
		_ = c.record(false, bootstrap)
		c.fail("verifier-disagreement")
	}
	if c.record(true, bootstrap) != nil {
		c.fail("dogfood-report-write")
	}
	_, _ = c.stdout.Write(firstLine(cem.stdout))
	if cem.status != 0 {
		c.fail("cem-policy", missingLines(readFile(c.report))...)
	}
	_, _ = c.stdout.Write(ocmLine)
	fmt.Fprintf(c.stdout, "dogfood-check: PASS\n")
	return 0
}

// verifyOCM requires the verifiers to agree on the OCM aggregate the change
// published and returns its first line.
func (c *check) verifyOCM(aggregate string) []byte {
	ocm, agreed := c.verifyAll("dogfood-ocm", "status", "--expected-base", c.base, "--target", c.target)
	if !agreed {
		_ = c.record(false, 0)
		c.fail("verifier-disagreement")
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
	override := "null"
	if c.overrideSHA != "" {
		override = `"sha256:` + c.overrideSHA + `"`
	}
	replacement := fmt.Sprintf(`  ,"dogfoodCheck": {"baseVerifierSha256": "sha256:%s", "treeVerifierSha256": "sha256:%s", "overrideVerifierSha256": %s, "bootstrapUnknown": %d, "maximumUnknownAfterBootstrap": 0, "outputsAgree": %t}`,
		c.baseSHA, c.treeSHA, override, bootstrap, agreed)
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
	value := allCaptures(cemBaseRevisionRE, []byte(sidecar))
	if status != 0 || !fullRevision.MatchString(value) {
		return "", false
	}
	_, status = c.git(false, "merge-base", "--is-ancestor", value, commit)
	return value, status == 0
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
func (c *check) reportUnboundCommits() {
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
	if len(unbound) > 0 {
		c.say("dogfood-check: NOTE unbound-commits count=%d window=%s..%s\n", len(unbound), previousBase, c.base)
		for _, commit := range unbound {
			c.say("  unbound %s\n", commit)
		}
	}
	if len(retroactive) > 0 {
		c.say("dogfood-check: NOTE retroactive-bound-commits count=%d window=%s..%s\n", len(retroactive), previousBase, c.base)
		for _, commit := range retroactive {
			c.say("  retroactive %s\n", commit)
		}
	}
}
