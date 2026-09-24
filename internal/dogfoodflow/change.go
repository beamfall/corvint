package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ChangeOptions are the inputs of `corvint dogfood change`; the text fields
// carry the DOGFOOD_* values the former script read from its environment.
type ChangeOptions struct {
	Root        string
	Base        string
	Steps       Runner
	Task        string
	Verify      string
	VerifyFile  string
	Outcome     string
	IntentsFile string
	Citations   string
	OCMLinks    string
}

type step struct{ name, status, reason string }

type change struct {
	flow
	options              ChangeOptions
	evidence             string
	runTmp               string
	rows                 []step
	citationStage        string
	citationStageOwned   bool
	citationCount        int
	citedOver            bool
	intentPublishTmp     string
	localOutcomeSHA      string
	contextAbstentionSHA string
	bootstrapUnknown     int
	manifestValid        bool
	intentSnapshot       []byte
	intents              []string
	linkPlan             []byte
	linksReady           bool
	linkPlanSHA          string
	linkPlanRows         int
}

// Change runs the pre-change context, binding and status steps and writes
// .corvint/dogfood-report.json, exactly as script/dogfood-change.sh did.
func Change(ctx context.Context, options ChangeOptions, stderr io.Writer) (int, error) {
	run := &change{flow: flow{ctx: ctx, prefix: "dogfood-change", root: options.Root, stderr: stderr}, options: options}
	return boundary(ctx, run.run, run.cleanup)
}

func (c *change) cleanup() {
	if c.intentPublishTmp != "" {
		_ = removeFile(c.intentPublishTmp)
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
	c.base = c.gitValue("rev-parse", c.options.Base+"^{commit}")
	c.target = c.gitValue("rev-parse", "HEAD^{commit}")
	task := c.options.Task
	if task == "" {
		task = "Dogfood change from " + c.base[:12] + " to HEAD"
	}
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
	if c.gitValue("diff", "--name-only", c.base, c.target, "--", ".corvint/changes") != "" {
		c.say("dogfood-change: REFUSE sealed-cem-in-change\n")
		c.say("  BASE..HEAD adds a sealed CEM; revert or drop the seal commit, then rebind\n")
		c.exit(2)
	}
	c.runStep("prechange-query", c.evidence+"/prechange-query.json", "query", "--task", task, "--limit", "1")
	c.prechangeImpact()
	c.prepare("cem-prepare", c.evidence+"/cem-prepare.json", func(status int, stderr []byte, reason string) bool {
		return status == 2 && chomp(string(stderr)) == outdatedCEMMap
	}, "cem", "prepare", "--base", c.base, "--target", c.target)
	// The manifest is frozen before citation so the plan check knows which
	// base-absent intent hunks an author may deliberately leave uncited.
	c.manifestValid = c.validateIntentManifest()
	c.citeStep()
	_ = os.WriteFile(c.path(".corvint/change.ocm-status.json"), nil, 0o666)
	switch {
	case !c.manifestValid:
		c.addStep("ocm-aggregate", "NOT_PRODUCED", "missing-intent-scope")
	case !c.runOCMScopes():
		c.addStep("ocm-aggregate", "NOT_PRODUCED", "intent-scope-drift")
	case c.finishOCMAggregate():
		c.countBootstrapUnknowns()
	}
	c.runStep("cem-status", c.evidence+"/cem-status.json", "cem", "status", "--map", ".corvint/change.cem.json",
		"--expected-base", c.base, "--target", c.target, "--max-unknown", strconv.Itoa(c.bootstrapUnknown), "--max-mechanical", "0")
	// The recorder refuses a dirty tree, so it runs last and sees the sidecar
	// this pass prepared and cited: an untracked or modified sidecar reports
	// record-index-failed and the pass is never complete (DCW-V0-015).
	c.rows = append(c.rows, c.localOutcome(task))
	c.renderReport()
	return c.reportFailures()
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

const outdatedCEMMap = `{"error": "cannot read CEM map: the existing map records a different base or patch; pass --replace to regenerate", "ok": false}`

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
// it refuses the range with exactly one unsupported-impact-range envelope.
func (c *change) prechangeImpact() {
	output := c.evidence + "/prechange-impact.json"
	errorFile := c.evidence + "/prechange-impact.stderr"
	argvFile := c.evidence + "/prechange-impact.argv"
	artifact := c.evidence + "/prechange-impact-abstention.json"
	argv := []string{c.options.Steps.Path, "--root", c.root, "impact", "--base", c.base, "--range-profile", "expanded-256", "--limit", "20"}
	failed := func() { c.addStep("prechange-impact", "NOT_PRODUCED", "context-abstention-evidence-failed") }
	if removeFile(artifact) != nil || writePrivate(argvFile, []byte(strings.Join(argv, "\x00")+"\x00")) != nil {
		failed()
		return
	}
	status := c.exec(argv[3:], output, errorFile)
	if status == 0 {
		c.addStep("prechange-impact", "PRODUCED", "none")
		return
	}
	stderr := readFile(errorFile)
	reason := failureReason(stderr, status)
	if status != 2 || reason != "unsupported-impact-range" || hasContent(output) || bytes.Count(stderr, []byte("\n")) != 1 || !anyLine(impactRefusal, stderr) {
		if reason == "unsupported-impact-range" {
			reason = "context-abstention-invalid"
		}
		c.addStep("prechange-impact", "NOT_PRODUCED", reason)
		return
	}
	stdoutSHA, stdoutErr := fileSHA256(output)
	stderrSHA, stderrErr := fileSHA256(errorFile)
	argvSHA, argvErr := fileSHA256(argvFile)
	if stdoutErr != nil || stderrErr != nil || argvErr != nil {
		failed()
		return
	}
	record := []byte(abstentionArtifact(argvSHA, c.base, stderrSHA, stdoutSHA, c.target) + "\n")
	if writePrivate(artifact, record) != nil {
		failed()
		return
	}
	c.contextAbstentionSHA = sha256Hex(record)
	c.addStep("prechange-impact", "NOT_PRODUCED", reason)
}

func abstentionArtifact(argvSHA, base, stderrSHA, stdoutSHA, target string) string {
	return `{"argvSha256":"sha256:` + argvSHA + `","base":"` + base + `","exitStatus":"2","profile":"corvint-dogfood-context-abstention/0","reason":"unsupported-impact-range","status":"NOT_PRODUCED","stderrSha256":"sha256:` + stderrSHA + `","stdoutSha256":"sha256:` + stdoutSHA + `","step":"prechange-impact","target":"` + target + `"}`
}

// localOutcome records the author's verification outcome, or names why no
// outcome input was provided, and returns the local-outcome row.
func (c *change) localOutcome(task string) step {
	verify := []string{}
	switch {
	case c.options.VerifyFile != "" && !isRegular(c.options.VerifyFile):
		return step{"local-outcome", "NOT_PRODUCED", "verify-file-unavailable"}
	case c.options.VerifyFile != "":
		verify = verifyArguments(readFile(c.options.VerifyFile))
	case c.options.Verify != "":
		verify = verifyArguments([]byte(c.options.Verify + "\n"))
	}
	if c.options.Outcome == "" || len(verify) == 0 {
		return step{"local-outcome", "NOT_PRODUCED", "outcome-input-not-provided"}
	}
	output := c.evidence + "/local-outcome.json"
	errorFile := c.evidence + "/local-outcome.stderr"
	args := append([]string{"dogfood-record", "--base", c.base, "--target", c.target, "--task", task}, verify...)
	status := c.exec(append(args, "--outcome", c.options.Outcome), output, errorFile)
	if status != 0 {
		return step{"local-outcome", "NOT_PRODUCED", failureReason(readFile(errorFile), status)}
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

var recordState = regexp.MustCompile(`^.*"state":"([a-z-]*)".*$`)

// verifyArguments passes one --verify per line that is not blank; the recorder
// applies its own bounds to each command.
func verifyArguments(data []byte) []string {
	args := []string{}
	for _, line := range textLines(data) {
		if strings.TrimLeft(line, " \t\n\v\f\r") != "" {
			args = append(args, "--verify", line)
		}
	}
	return args
}

// validateIntentManifest freezes DOGFOOD_INTENTS_FILE: 1-16 sorted,
// LF-terminated, canonical repository-relative paths.
func (c *change) validateIntentManifest() bool {
	source := c.options.IntentsFile
	if source == "" || !isRegular(source) || isSymlink(source) {
		return false
	}
	data, err := os.ReadFile(source)
	if err != nil || len(data) == 0 || len(data) > 16*513 || data[len(data)-1] != '\n' {
		return false
	}
	paths := readLines(data)
	if len(paths) > 16 {
		return false
	}
	previous := ""
	for _, path := range paths {
		if !canonicalIntentPath(path) || (previous != "" && previous >= path) {
			return false
		}
		previous = path
	}
	c.intentSnapshot, c.intents = data, paths
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
// intermediate maps so only the last cite publishes the tracked map.
func (c *change) citeStep() {
	switch {
	case !isRegular(c.path(".corvint/change.cem.json")):
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
	switch {
	case !valid:
		status, reason = "NOT_PRODUCED", "invalid-citation-plan"
	case !c.citationPlanMatchesMap(plan):
		status, reason = "NOT_PRODUCED", "citation-plan-map-mismatch"
	case c.citationCount > 1 && exists(c.path(c.citationStage)):
		status, reason = "NOT_PRODUCED", "citation-stage-exists"
	default:
		cited := citedHunks(c.path(".corvint/change.cem.json"))
		status, reason = c.cite(plan, citeOutput)
		c.citedOver = cited > 0 && status == "PRODUCED"
	}
	if c.cleanupCitationStage() != nil {
		status, reason = "NOT_PRODUCED", "citation-stage-cleanup-failed"
	}
	c.addStep("cem-cite", status, reason)
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
			return "NOT_PRODUCED", failureReason(readFile(errorFile), status)
		}
		// Intermediate receipts truthfully name the stage; only the last cite
		// publishes the original map. Keep their raw stdout as private evidence.
		appendFile(citeOutput, readFile(part))
		citeMap = c.citationStage
	}
	return "PRODUCED", "none"
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
	}
	return plan, true
}

const maxPlanBytes = 4194304

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

var (
	hunkField = regexp.MustCompile(`^      "(disposition|id|path)": "(.*)$`)
	ordinal   = regexp.MustCompile(`^[1-9][0-9]*$`)
	valueEnd  = regexp.MustCompile(`",?$`)
)

// citationPlanMatchesMap binds a plan to the map prepared in this run
// (DCW-V0-019): no ordinal may exceed the map's hunk count, a numeric selector
// must be canonical, and every hunk the map records as unknown must be named by
// ordinal or full ID unless its path is an intent absent at BASE or more such
// hunks remain than one 256-row plan can name. An empty plan stays a no-op, and
// only a regular map is read, at most the native 4 MiB map bound.
func (c *change) citationPlanMatchesMap(plan []byte) bool {
	mapPath := c.path(".corvint/change.cem.json")
	if len(plan) == 0 || !isRegular(mapPath) || isSymlink(mapPath) {
		return true
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
		return false
	}
	hunks := mapHunks(data)
	named := map[string]bool{}
	for _, line := range textLines(plan) {
		selector, _, _ := strings.Cut(line, "\t")
		named[selector] = true
	}
	for selector := range named {
		numeric := ordinal.MatchString(selector)
		if strings.ContainsAny(selector[:1], "+0123456789") && !numeric {
			return false
		}
		if value, err := strconv.Atoi(selector); numeric && (err != nil || value > len(hunks)) {
			return false
		}
	}
	owed := []int{}
	for index, hunk := range hunks {
		if hunk["disposition"] == "unknown" && !bootstrap[hunk["path"]] {
			owed = append(owed, index+1)
		}
	}
	// More owed hunks than one plan has rows: split plans stay admissible.
	if len(owed) > 256 {
		return true
	}
	for _, index := range owed {
		if !named[strconv.Itoa(index)] && !named[hunks[index-1]["id"]] {
			return false
		}
	}
	return true
}

// mapHunks reads the scalar disposition, id and path of each hunk from the
// canonical indent-2 map encoding: each hunk opens on a four-space "{" line
// inside "hunks" and its scalar keys sit at six spaces.
func mapHunks(data []byte) []map[string]string {
	hunks := []map[string]string{}
	preamble := map[string]string{}
	inside := false
	for _, line := range textLines(data) {
		if line == `  "hunks": [` {
			inside = true
			continue
		}
		if strings.HasPrefix(line, "  ]") {
			inside = false
		}
		if !inside {
			continue
		}
		if line == "    {" {
			hunks = append(hunks, map[string]string{})
			continue
		}
		match := hunkField.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		value := valueEnd.ReplaceAllString(match[2], "")
		current := preamble
		if len(hunks) > 0 {
			current = hunks[len(hunks)-1]
		}
		current[match[1]] = value
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
	if len(plan) > maxPlanBytes || rows > 256 || plan[len(plan)-1] != '\n' || hasForbiddenControl(plan) || !c.linkRowsValid(plan) {
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

// finishOCMAggregate publishes the frozen manifest, unless its source changed
// during the run, and verifies the ordered map set against it.
func (c *change) finishOCMAggregate() bool {
	current, err := os.ReadFile(c.options.IntentsFile)
	if err != nil || !bytes.Equal(current, c.intentSnapshot) {
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
	return c.runStep("ocm-aggregate", c.path(".corvint/change.ocm-status.json"), "dogfood-ocm", "status", "--expected-base", c.base, "--target", c.target) == 0
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

// failing reports a row that keeps the report incomplete; the two typed
// abstentions do not.
func failing(row step) bool {
	switch {
	case row.status == "PRODUCED":
		return false
	case row.name == "local-outcome" && row.status == "NOT_PRODUCED" && row.reason == "no-source-paths":
		return false
	case row.name == "prechange-impact" && row.status == "NOT_PRODUCED" && row.reason == "unsupported-impact-range":
		return false
	}
	return true
}

func (c *change) complete() bool {
	for _, row := range c.rows {
		if failing(row) {
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
	if aggregate := readFile(c.path(".corvint/change.ocm-status.json")); len(aggregate) > 0 {
		report.WriteString("  \"ocmStatus\": ")
		report.Write(firstLine(aggregate))
	} else {
		report.WriteString("  \"ocmStatus\": null\n")
	}
	report.WriteString(`  ,"localOutcomeEvidenceSha256": ` + digestOrNull(c.localOutcomeSHA) + "\n")
	report.WriteString(`  ,"contextAbstentionEvidenceSha256": ` + digestOrNull(c.contextAbstentionSHA) + "\n")
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
	fmt.Fprintf(&report, "  ,\"packetCoverage\": [%s, %s]\n", c.packetCoverage("prechange-query"), c.packetCoverage("prechange-impact"))
	report.WriteString("  ,\"dogfoodCheck\": null\n}\n")
	_ = os.WriteFile(c.path(".corvint/dogfood-report.json"), report.Bytes(), 0o666)
	for _, row := range c.rows {
		c.options.Steps.Run(c.ctx, c.root, []string{"dogfood-observe", "--step", row.name, "--status", row.status, "--reason", row.reason}, io.Discard, io.Discard)
		c.checkpoint()
	}
}

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
	{"cem-cite:citation-plan-not-provided", "set DOGFOOD_CITATIONS to the path of a TSV plan with one row per hunk of .corvint/change.cem.json"},
	{"cem-cite:citation-plan-unavailable", "DOGFOOD_CITATIONS must be the path of a TSV file of ORDINAL<TAB>PATH<TAB>START:END<TAB>RELATION rows, not the rows themselves"},
	{"cem-cite:invalid-citation-plan", "each row is ORDINAL<TAB>PATH<TAB>START:END<TAB>RELATION in worklist order, LF-terminated, at most 256 rows"},
	{"cem-cite:citation-plan-map-mismatch", "the plan does not match the map prepared for HEAD: a row names an ordinal past its hunks or is not a canonical ordinal, or an unknown hunk is unnamed (often because a later commit re-prepared the map); rewrite DOGFOOD_CITATIONS from the current .corvint/change.cem.json, naming every unknown hunk except the hunk of an intent spec absent at BASE"},
	{"ocm-aggregate:missing-intent-scope", "DOGFOOD_INTENTS_FILE must be the path of a sorted, LF-terminated file listing 1-16 repository-relative spec paths"},
	{"ocm-prepare-*:invalid-requirements-section", `intent must be a spec that exists at BASE and contains exactly one "## Requirements" heading`},
	{"ocm-prepare-*:excluded-artifact-mismatch", uncommittedHint},
	{"prechange-impact:unsupported-impact-worktree", uncommittedHint},
	{"local-outcome:record-index-failed", uncommittedHint},
	{"ocm-status-*", "fix the ocm-prepare row with the same number first; if it was produced, the worktree has uncommitted changes (often the prepared sidecar); commit them, then rerun make dogfood-change"},
	{"cem-status:not-ready", "read verification.issues and policyIssues in {evidence}/cem-status.json: excluded-artifact-mismatch means the sidecar is uncommitted, max-unknown-exceeded means DOGFOOD_CITATIONS does not cite every hunk"},
	{"ocm-links:ocm-link-plan-unavailable", "DOGFOOD_OCM_LINKS must be the path of a TSV file of INTENT<TAB>REQUIREMENT<TAB>HUNKS<TAB>TEST_PATH<TAB>CLAIMS rows, not the rows themselves"},
	{"ocm-links:empty-ocm-link-plan", "DOGFOOD_OCM_LINKS names an empty file; add at least one row, or unset DOGFOOD_OCM_LINKS so every requirement stays unassessed"},
	{"ocm-links:invalid-ocm-link-plan", "each DOGFOOD_OCM_LINKS row is INTENT<TAB>REQUIREMENT<TAB>HUNK[,HUNK...]<TAB>TEST_PATH<TAB>CLAIM[,CLAIM...], LF-terminated, at most 256 rows, and INTENT is listed in DOGFOOD_INTENTS_FILE"},
	{"ocm-link-*", "read {evidence}/{step}.stderr: each linked hunk must be cited in the committed sidecar, and each claim a test case or t.Run name at HEAD containing the exact requirement ID; otherwise delete the DOGFOOD_OCM_LINKS row so the requirement stays unassessed"},
	{"ocm-aggregate:intent-scope-drift", "fix the ocm-prepare or ocm-status row above; otherwise the intents file changed during the run"},
	{"local-outcome:outcome-input-not-provided", "set DOGFOOD_OUTCOME (passed, failed or blocked) and DOGFOOD_VERIFY_FILE (one verification command per line)"},
}

const uncommittedHint = "the worktree has uncommitted changes (often the prepared sidecar); commit them, then rerun make dogfood-change"

func (c *change) fixHint(row step) string {
	value := row.name + ":" + row.reason
	for _, entry := range fixHints {
		prefix, suffix, star := strings.Cut(entry.pattern, "*")
		matched := value == entry.pattern
		if star {
			matched = len(value) >= len(prefix)+len(suffix) && strings.HasPrefix(value, prefix) && strings.HasSuffix(value, suffix)
		}
		if matched {
			return strings.NewReplacer("{evidence}", c.evidence, "{step}", row.name).Replace(entry.hint)
		}
	}
	return ""
}

// reportFailures lists each failing row with its fix and exits 1, or exits 0
// silently when the report is complete.
func (c *change) reportFailures() int {
	if c.complete() {
		return 0
	}
	c.say("dogfood-change: FAIL not-complete\n")
	for _, row := range c.rows {
		if !failing(row) {
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
		c.say("  cem-cite: the plan was added to citations the map already carried and never replaces them; to correct an earlier plan, delete .corvint/change.cem.json and rerun make dogfood-change (docs/DOGFOOD.md step 4)\n")
	}
	query := readFile(c.evidence + "/prechange-query.stderr")
	// The authority-start refusal is selected by the task wording, not by the
	// change; a malformed store refuses any wording and names no profile.
	for _, line := range textLines(query) {
		if strings.HasPrefix(line, `{"code": "unsupported-query-trace-state", "error": "native Go authority-start query `) {
			c.say("  prechange-query: DOGFOOD_TASK wording selected the authority-start profile, which refuses a present local trace store; keep this receipt and the task (docs/DOGFOOD.md section 1)\n")
			break
		}
	}
	// Amending or rebasing after a recorded pass strands that trace; query and
	// the recorder then refuse every later run with this message.
	stranded := []byte(`"error": "local trace store contains unreachable revision: `)
	if bytes.Contains(query, stranded) || bytes.Contains(readFile(c.evidence+"/local-outcome.stderr"), stranded) {
		c.say("  local trace store: a recorded trace names a commit no longer reachable from HEAD; restore that commit as an ancestor and add new commits instead of amending or rebasing (docs/DOGFOOD.md \"Daily adopter path\")\n")
	}
	c.say("  full report: %s\n", c.path(".corvint/dogfood-report.json"))
	return 1
}
