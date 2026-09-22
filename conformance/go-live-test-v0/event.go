// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	runnerBuildState   rejectCode = "RUNNER_BUILD_STATE"
	runnerFieldMatrix  rejectCode = "RUNNER_FIELD_MATRIX"
	runnerPackageState rejectCode = "RUNNER_PACKAGE_STATE"
	runnerTestState    rejectCode = "RUNNER_TEST_STATE"
	maxTranscriptBytes            = 2*maxDocumentBytes + (maxArrayEntries+1)*512
)

type transcriptContext struct {
	ActualEnvironmentSHA256  string
	CapabilityID             string
	CapabilityPreimageBase64 string
	DiscoveredPackagePaths   []string
	DiscoveryID              string
	GOARCH                   string
	GOOS                     string
	ListedPackages           []string
	Nonce                    string
	PlanID                   string
	PlanPreimageBase64       string
	RequestedPackagePatterns []string
	RequestedRunnerPackages  []string
	ToolchainID              string
	VerifierExecutableSHA256 string
}

type eventObservation struct {
	buildEventFailed    map[string]bool
	buildMismatch       bool
	buildOutputObserved map[string]bool
	executed            map[string]bool
	failedBuildReported map[string]bool
	openState           bool
	observed            map[string]bool
	packageState        map[string]string
	skipped             map[string]bool
	testState           map[string]string
	testTerminal        map[string]string
}

var mandatoryUnknownReasons = []string{
	"BUILD_CONSTRAINT_VARIANTS",
	"CROSS_PLATFORM_VARIANTS",
	"DISCOVERY_INCOMPLETE",
	"EXTERNAL_MODULE_FRONTIER",
	"FUZZ_BENCHMARK_FRONTIER",
	"MANDATORY_GATE_OUTSIDE_RUN",
	"NESTED_MODULE_FRONTIER",
	"NETWORK_STATE_UNKNOWN",
	"NON_GO_TEST_FRONTIER",
	"NO_AFFECTED_SELECTION_PROOF",
	"PACKAGE_PATTERN_SEMANTICS",
	"PARENT_TEST_UNAVAILABLE",
	"SOURCE_ANCHOR_UNAVAILABLE",
	"UNOBSERVED_DYNAMIC_SUBTESTS",
}

func VerifyTranscript(raw []byte, context transcriptContext) error {
	if len(raw) == 0 || len(raw) > maxTranscriptBytes || raw[len(raw)-1] != '\n' {
		return reject(limitExceeded)
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) < 1 || len(lines) > maxArrayEntries+1 {
		return reject(limitExceeded)
	}
	if err := validateTranscriptContext(context); err != nil {
		return err
	}
	observation := &eventObservation{
		buildEventFailed: map[string]bool{}, buildOutputObserved: map[string]bool{},
		executed: map[string]bool{}, failedBuildReported: map[string]bool{}, observed: map[string]bool{},
		packageState: map[string]string{}, skipped: map[string]bool{}, testState: map[string]string{},
		testTerminal: map[string]string{},
	}
	expectedRunID := attemptID(context.Nonce, context.PlanID)
	events := make([]map[string]any, 0, len(lines)-1)
	factBytes := uint64(0)
	for index, line := range lines[:len(lines)-1] {
		if len(line) == 0 || len(line)+1 > maxDocumentBytes {
			return reject(limitExceeded)
		}
		object, err := parseCanonical(append(append([]byte(nil), line...), '\n'))
		if err != nil {
			return err
		}
		if err := verifyEvent(object, uint64(index), expectedRunID, context.DiscoveredPackagePaths, observation); err != nil {
			return err
		}
		factBody, err := canonical(object["fact"])
		if err != nil || uint64(len(factBody)) > uint64(maxDocumentBytes)-factBytes {
			return reject(limitExceeded)
		}
		factBytes += uint64(len(factBody))
		events = append(events, object)
	}
	if err := closeStateMachines(observation, context.RequestedRunnerPackages); err != nil {
		return err
	}
	wei, err := computeWEI(context, observation)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event["wei"] != wei {
			return reject(identityMismatch)
		}
		body, _ := canonical(without(event, "id"))
		if event["id"] != domainID("go-live-event", "go-live-event/0", body) {
			return reject(identityMismatch)
		}
	}
	if len(lines[len(lines)-1])+1 > maxDocumentBytes {
		return reject(limitExceeded)
	}
	run, err := parseCanonical(append(append([]byte(nil), lines[len(lines)-1]...), '\n'))
	if err != nil {
		return err
	}
	return verifyRun(run, events, observation, context, wei, expectedRunID)
}

func validateTranscriptContext(context transcriptContext) error {
	if !isDigest(context.ActualEnvironmentSHA256) || !isID(context.CapabilityID, "go-live-capability") || !isID(context.DiscoveryID, "go-live-discovery") || !isID(context.PlanID, "go-live-plan") || !isID(context.ToolchainID, "go-toolchain") || len(context.Nonce) != 32 || strings.ToLower(context.Nonce) != context.Nonce || !hexString(context.Nonce) || len(context.RequestedRunnerPackages) == 0 || len(context.DiscoveredPackagePaths) == 0 || !sortedUnique(context.RequestedRunnerPackages) || !sortedUnique(context.DiscoveredPackagePaths) || !sortedUnique(context.ListedPackages) || !validPatterns(context.RequestedPackagePatterns) {
		return reject(discoveryInput)
	}
	for _, pkg := range context.DiscoveredPackagePaths {
		if pkg == "" || len(pkg) > maxPatternBytes || !validText(pkg) {
			return reject(discoveryInput)
		}
	}
	for _, pkg := range context.RequestedRunnerPackages {
		if !containsString(context.DiscoveredPackagePaths, pkg) || !validExactImportPath(pkg) {
			return reject(discoveryInput)
		}
	}
	for _, id := range context.ListedPackages {
		if !isID(id, "go-package") {
			return reject(discoveryInput)
		}
	}
	return nil
}

func verifyEvent(event map[string]any, sequence uint64, expectedRunID string, discoveredPackages []string, observation *eventObservation) error {
	if err := exactKeys(event, "fact", "id", "profile", "runId", "sequence", "wei"); err != nil {
		return err
	}
	profile, err := stringField(event, "profile")
	if err != nil || profile != "go-live-event/0" {
		return reject(runnerFieldMatrix)
	}
	if !isIDValue(event["id"], "go-live-event") || event["runId"] != expectedRunID || !isIDValue(event["wei"], "workspace-execution") || event["sequence"] != strconv.FormatUint(sequence, 10) {
		return reject(identityMismatch)
	}
	fact, ok := event["fact"].(map[string]any)
	if !ok {
		return reject(runnerFieldMatrix)
	}
	kind, _ := fact["factKind"].(string)
	switch kind {
	case "TEST":
		return verifyTestFact(fact, discoveredPackages, observation)
	case "BUILD":
		return verifyBuildFact(fact, observation)
	default:
		return reject(runnerFieldMatrix)
	}
}

func verifyTestFact(fact map[string]any, discoveredPackages []string, observation *eventObservation) error {
	if err := exactKeys(fact, "artifactPathSha256", "attribute", "durationNanoseconds", "factKind", "failedBuild", "output", "package", "parentTestId", "rawAction", "sourceAnchor", "test", "timeRaw", "timeUtc"); err != nil {
		return err
	}
	packageName, err := stringField(fact, "package")
	if err != nil || !containsString(discoveredPackages, packageName) || fact["factKind"] != "TEST" || fact["parentTestId"] != nil || fact["sourceAnchor"] != nil {
		return reject(runnerFieldMatrix)
	}
	action, err := stringField(fact, "rawAction")
	if err != nil || !oneOf(action, "start", "run", "pause", "cont", "output", "pass", "bench", "fail", "skip", "attr", "artifacts") {
		return reject(runnerFieldMatrix)
	}
	if err := verifyTimes(fact); err != nil {
		return err
	}
	testName, hasTest, err := optionalNonemptyString(fact["test"])
	if err != nil {
		return err
	}
	duration, hasDuration, err := optionalDecimal(fact["durationNanoseconds"])
	_ = duration
	if err != nil {
		return err
	}
	if oneOf(action, "start", "run", "pause", "cont", "output", "attr", "artifacts") && hasDuration || oneOf(action, "pass", "fail") && !hasDuration {
		return reject(runnerFieldMatrix)
	}
	if oneOf(action, "run", "pause", "cont", "bench", "attr", "artifacts") && !hasTest {
		return reject(runnerFieldMatrix)
	}
	if action == "start" && hasTest {
		return reject(runnerFieldMatrix)
	}
	if action == "fail" {
		failedBuild, hasFailedBuild, err := optionalNonemptyString(fact["failedBuild"])
		if err != nil || hasFailedBuild && hasTest {
			return reject(runnerFieldMatrix)
		}
		if hasFailedBuild {
			observation.failedBuildReported[failedBuild] = true
		}
	} else if fact["failedBuild"] != nil {
		return reject(runnerFieldMatrix)
	}
	if err := verifyFactExtras(fact, action); err != nil {
		return err
	}
	if hasTest {
		testID := testIdentity(packageName, testName)
		observation.observed[testID] = true
		return advanceTest(observation, testID, action)
	}
	return advancePackage(observation, packageName, action)
}

func verifyBuildFact(fact map[string]any, observation *eventObservation) error {
	if err := exactKeys(fact, "factKind", "importPath", "output", "rawAction"); err != nil {
		return err
	}
	path, err := stringField(fact, "importPath")
	if err != nil || fact["factKind"] != "BUILD" {
		return reject(runnerFieldMatrix)
	}
	action, err := stringField(fact, "rawAction")
	if err != nil || !oneOf(action, "build-output", "build-fail") {
		return reject(runnerFieldMatrix)
	}
	if err := verifyOutput(fact["output"], true, action == "build-output"); err != nil {
		return err
	}
	if observation.buildEventFailed[path] {
		return reject(runnerBuildState)
	}
	if action == "build-output" {
		observation.buildOutputObserved[path] = true
		return nil
	}
	if action == "build-fail" {
		observation.buildEventFailed[path] = true
	}
	return nil
}

func verifyTimes(fact map[string]any) error {
	raw, err := stringField(fact, "timeRaw")
	if err != nil {
		return reject(runnerFieldMatrix)
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil || !strings.HasSuffix(raw, "Z") && !hasNumericOffset(raw) {
		return reject(runnerFieldMatrix)
	}
	utc, err := stringField(fact, "timeUtc")
	if err != nil || parsed.UTC().Format(time.RFC3339Nano) != utc {
		return reject(runnerFieldMatrix)
	}
	return nil
}

func hasNumericOffset(value string) bool {
	if len(value) < 6 {
		return false
	}
	suffix := value[len(value)-6:]
	return (suffix[0] == '+' || suffix[0] == '-') && suffix[3] == ':'
}

func verifyFactExtras(fact map[string]any, action string) error {
	if err := verifyOutput(fact["output"], false, action == "output"); err != nil {
		return err
	}
	if action == "attr" {
		attribute, ok := fact["attribute"].(map[string]any)
		if !ok || exactKeys(attribute, "keySha256", "valueSha256") != nil || !isDigestValue(attribute["keySha256"]) || !isDigestValue(attribute["valueSha256"]) {
			return reject(runnerFieldMatrix)
		}
	} else if fact["attribute"] != nil {
		return reject(runnerFieldMatrix)
	}
	if action == "artifacts" {
		if !isDigestValue(fact["artifactPathSha256"]) {
			return reject(runnerFieldMatrix)
		}
	} else if fact["artifactPathSha256"] != nil {
		return reject(runnerFieldMatrix)
	}
	return nil
}

func verifyOutput(value any, build bool, permitsOutput bool) error {
	output, ok := value.(map[string]any)
	if !ok {
		return reject(runnerFieldMatrix)
	}
	keys := []string{"byteCount", "sha256", "text", "truncated"}
	if !build {
		keys = []string{"byteCount", "outputType", "sha256", "text", "truncated"}
	}
	if err := exactKeys(output, keys...); err != nil || !isDigestValue(output["sha256"]) || output["text"] != nil || output["truncated"] != false {
		return reject(runnerFieldMatrix)
	}
	count, ok := output["byteCount"].(string)
	if !ok || !canonicalDecimal(count) {
		return reject(runnerFieldMatrix)
	}
	if !build {
		if output["outputType"] != nil && !oneOfValue(output["outputType"], "frame", "error", "error-continue") {
			return reject(runnerFieldMatrix)
		}
	}
	if !permitsOutput {
		empty := sha256.Sum256(nil)
		if count != "0" || output["sha256"] != hex.EncodeToString(empty[:]) || !build && output["outputType"] != nil {
			return reject(runnerFieldMatrix)
		}
	}
	return nil
}

func advancePackage(observation *eventObservation, packageName, action string) error {
	state := observation.packageState[packageName]
	switch action {
	case "start":
		if state != "" {
			return reject(runnerPackageState)
		}
		observation.packageState[packageName] = "STARTED"
	case "pass", "fail", "skip":
		if state != "STARTED" {
			return reject(runnerPackageState)
		}
		terminal := map[string]string{"pass": "PASSED", "fail": "FAILED", "skip": "SKIPPED"}[action]
		observation.packageState[packageName] = terminal
	case "output":
		if state == "" || isTerminal(state) {
			return reject(runnerPackageState)
		}
	default:
		return reject(runnerFieldMatrix)
	}
	return nil
}

func advanceTest(observation *eventObservation, testID, action string) error {
	state := observation.testState[testID]
	switch action {
	case "run":
		if state != "" {
			return reject(runnerTestState)
		}
		observation.testState[testID] = "RUNNING"
		observation.executed[testID] = true
	case "pause":
		if state != "RUNNING" {
			return reject(runnerTestState)
		}
		observation.testState[testID] = "PAUSED"
	case "cont":
		if state != "PAUSED" {
			return reject(runnerTestState)
		}
		observation.testState[testID] = "RUNNING"
	case "pass", "fail", "skip", "bench":
		if state != "RUNNING" && state != "PAUSED" {
			return reject(runnerTestState)
		}
		terminal := strings.ToUpper(action)
		if terminal == "PASS" {
			terminal = "PASSED"
		} else if terminal == "FAIL" {
			terminal = "FAILED"
		} else if terminal == "SKIP" {
			terminal = "SKIPPED"
		}
		observation.testState[testID] = terminal
		observation.testTerminal[testID] = terminal
		if terminal == "SKIPPED" {
			observation.skipped[testID] = true
		}
	case "output", "attr", "artifacts":
		if state != "RUNNING" && state != "PAUSED" {
			return reject(runnerTestState)
		}
	default:
		return reject(runnerFieldMatrix)
	}
	return nil
}

func closeStateMachines(observation *eventObservation, requested []string) error {
	for _, pkg := range requested {
		if !isTerminal(observation.packageState[pkg]) {
			observation.openState = true
		}
	}
	for _, state := range observation.testState {
		if !isTerminal(state) && state != "BENCH" {
			observation.openState = true
		}
	}
	observation.buildMismatch = !equalStrings(sortedMapKeys(observation.buildEventFailed), sortedMapKeys(observation.failedBuildReported))
	return nil
}

func computeWEI(context transcriptContext, observation *eventObservation) (string, error) {
	tests := sortedMapKeys(observation.observed)
	discoveredBody, _ := canonical(map[string]any{
		"completeness": "INCOMPLETE", "packages": stringArray(context.RequestedRunnerPackages), "tests": stringArray(tests),
	})
	discoveredDigest := bareDomainDigest("go-discovered-tests", "go-discovered-tests/0", discoveredBody)
	body, _ := canonical(map[string]any{
		"actualEnvironmentSha256": context.ActualEnvironmentSHA256, "capabilityId": context.CapabilityID,
		"discoveredTestSetSha256": discoveredDigest, "discoveryId": context.DiscoveryID,
		"planId": context.PlanID, "toolchainId": context.ToolchainID,
	})
	return domainID("workspace-execution", "go-live-wei/0", body), nil
}

func verifyRun(run map[string]any, events []map[string]any, observation *eventObservation, context transcriptContext, wei, expectedRunID string) error {
	if err := exactKeys(run, "coverage", "ephemeralDeletion", "eventCount", "eventRootSha256", "execution", "id", "io", "persistence", "planId", "profile", "qualification", "runId", "scope", "sourceCurrency", "sourcePostSha256", "sourcePreSha256", "toolchainCurrency", "toolchainPostSha256", "toolchainPreSha256", "wei"); err != nil {
		return err
	}
	if run["profile"] != "go-live-run/0" || run["persistence"] != "NONE" || run["qualification"] != "EXPERIMENTAL_TRANSCRIPT" || run["planId"] != context.PlanID || run["wei"] != wei || run["runId"] != expectedRunID || run["eventCount"] != strconv.Itoa(len(events)) || !oneOfValue(run["ephemeralDeletion"], "COMPLETE", "INCOMPLETE") {
		return reject(identityMismatch)
	}
	for _, event := range events {
		if event["runId"] != run["runId"] {
			return reject(identityMismatch)
		}
	}
	if run["eventRootSha256"] != eventRoot(events) {
		return reject(identityMismatch)
	}
	if err := verifyCoverageNone(run["coverage"]); err != nil {
		return err
	}
	ioObject, err := verifyRunIO(run["io"])
	if err != nil {
		return err
	}
	if err := verifyRunScope(run["scope"], observation, context); err != nil {
		return err
	}
	if err := verifyCurrency(run); err != nil {
		return err
	}
	execution, ok := run["execution"].(map[string]any)
	if !ok || verifyExecution(execution, observation, context.RequestedRunnerPackages, run["ephemeralDeletion"] == "COMPLETE", run["sourceCurrency"] == "CURRENT", run["toolchainCurrency"] == "CURRENT", ioObject) != nil {
		return reject(runnerFieldMatrix)
	}
	body, _ := canonical(without(run, "id"))
	if run["id"] != domainID("go-live-run", "go-live-run/0", body) {
		return reject(identityMismatch)
	}
	return nil
}

func attemptID(nonce, planID string) string {
	body, _ := canonical(map[string]any{"nonce": nonce, "planId": planID})
	return domainID("go-live-attempt", "go-live-attempt/0", body)
}

func verifyCoverageNone(value any) error {
	object, ok := value.(map[string]any)
	if !ok || exactKeys(object, "artifactSha256", "completeness", "mode", "packages", "rawRootSha256") != nil || object["artifactSha256"] != nil || object["completeness"] != "ABSENT" || object["mode"] != "NONE" || object["rawRootSha256"] != nil {
		return reject(runnerFieldMatrix)
	}
	packages, ok := object["packages"].([]any)
	if !ok || len(packages) != 0 {
		return reject(runnerFieldMatrix)
	}
	return nil
}

func verifyRunIO(value any) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok || exactKeys(object, "decoder", "stderrBytes", "stderrDrained", "stderrRawSha256", "stdoutBytes", "stdoutDrained", "stdoutRawSha256") != nil || !oneOfValue(object["decoder"], "COMPLETE", "REJECTED", "TRUNCATED") || !decimalValue(object["stderrBytes"]) || !decimalValue(object["stdoutBytes"]) || !isDigestValue(object["stderrRawSha256"]) || !isDigestValue(object["stdoutRawSha256"]) {
		return nil, reject(runnerFieldMatrix)
	}
	if _, ok := object["stderrDrained"].(bool); !ok {
		return nil, reject(runnerFieldMatrix)
	}
	if _, ok := object["stdoutDrained"].(bool); !ok {
		return nil, reject(runnerFieldMatrix)
	}
	return object, nil
}

func verifyRunScope(value any, observation *eventObservation, context transcriptContext) error {
	scope, ok := value.(map[string]any)
	if !ok || exactKeys(scope, "buildTerminals", "conclusion", "excluded", "executedTests", "listedPackages", "observedTests", "packageTerminals", "requestedPackagePatterns", "skippedTests", "testTerminals", "unknownReasons") != nil || scope["conclusion"] != "UNKNOWN" {
		return reject(runnerFieldMatrix)
	}
	if !emptyArray(scope["excluded"]) || !equalAnyStrings(scope["executedTests"], sortedMapKeys(observation.executed)) || !equalAnyStrings(scope["listedPackages"], context.ListedPackages) || !equalAnyStrings(scope["observedTests"], sortedMapKeys(observation.observed)) || !equalAnyStrings(scope["requestedPackagePatterns"], context.RequestedPackagePatterns) || !equalAnyStrings(scope["skippedTests"], sortedMapKeys(observation.skipped)) {
		return reject(runnerFieldMatrix)
	}
	if !equalCanonicalArray(scope["packageTerminals"], packageTerminalObjects(observation, context.RequestedRunnerPackages)) || !equalCanonicalArray(scope["testTerminals"], testTerminalObjects(observation)) || !equalCanonicalArray(scope["buildTerminals"], buildTerminalObjects(observation)) {
		return reject(runnerFieldMatrix)
	}
	reasons, err := anyStrings(scope["unknownReasons"])
	if err != nil || !equalStrings(reasons, mandatoryUnknownReasons) {
		return reject(runnerFieldMatrix)
	}
	return nil
}

func verifyCurrency(run map[string]any) error {
	for _, prefix := range []string{"source", "toolchain"} {
		currency, ok := run[prefix+"Currency"].(string)
		pre, preOK := run[prefix+"PreSha256"].(string)
		post, postPresent, err := optionalNonemptyString(run[prefix+"PostSha256"])
		if !ok || !preOK || !isDigest(pre) || err != nil || postPresent && !isDigest(post) {
			return reject(runnerFieldMatrix)
		}
		expected := "UNKNOWN"
		if postPresent && post == pre {
			expected = "CURRENT"
		} else if postPresent {
			expected = "STALE"
		}
		if currency != expected {
			return reject(runnerFieldMatrix)
		}
	}
	return nil
}

func verifyExecution(execution map[string]any, observation *eventObservation, requested []string, cleanup, sourceCurrent, toolchainCurrent bool, ioObject map[string]any) error {
	if exactKeys(execution, "cancelled", "containment", "durationNanoseconds", "exitCode", "failureClass", "limit", "resources", "signal", "status", "timedOut") != nil || !decimalValue(execution["durationNanoseconds"]) || !oneOfValue(execution["status"], "PASSED", "FAILED", "INCOMPLETE") {
		return reject(runnerFieldMatrix)
	}
	if _, ok := execution["cancelled"].(bool); !ok {
		return reject(runnerFieldMatrix)
	}
	if _, ok := execution["timedOut"].(bool); !ok {
		return reject(runnerFieldMatrix)
	}
	if execution["exitCode"] != nil {
		code, ok := execution["exitCode"].(string)
		if !ok || !canonicalDecimal(code) {
			return reject(runnerFieldMatrix)
		}
		if _, err := strconv.ParseUint(code, 10, 32); err != nil {
			return reject(runnerFieldMatrix)
		}
	}
	if execution["failureClass"] != nil && !oneOfValue(execution["failureClass"], "ASSERTION_OR_TEST", "BUILD", "DISCOVERY", "INFRASTRUCTURE", "TIMEOUT", "CANCELLATION", "CRASH", "STALE", "UNKNOWN") || execution["signal"] != nil && !oneOfValue(execution["signal"], "SIGINT", "SIGTERM", "SIGKILL", "OTHER") || execution["limit"] != nil && !oneOfValue(execution["limit"], "COVERAGE_BYTES", "COVERAGE_FILES", "CPU", "EVENT_BYTES", "EVENTS", "LINE_BYTES", "MEMORY", "OPEN_FILES", "OUTPUT_BYTES", "PACKAGES", "PROCESSES", "RUN_TIME", "TESTS") {
		return reject(runnerFieldMatrix)
	}
	containment, ok := execution["containment"].(map[string]any)
	if !ok || exactKeys(containment, "mechanism", "qualified") != nil || containment["mechanism"] != "PROCESS_GROUP_BEST_EFFORT" || containment["qualified"] != false {
		return reject(runnerFieldMatrix)
	}
	resources, ok := execution["resources"].(map[string]any)
	if !ok || exactKeys(resources, "cpuMilliseconds", "memoryPeakBytes", "openFilesPeak", "processesPeak") != nil {
		return reject(runnerFieldMatrix)
	}
	for _, key := range []string{"cpuMilliseconds", "memoryPeakBytes", "openFilesPeak", "processesPeak"} {
		if resources[key] != nil && !decimalValue(resources[key]) {
			return reject(runnerFieldMatrix)
		}
	}
	status := execution["status"].(string)
	allDrained := ioObject["stdoutDrained"] == true && ioObject["stderrDrained"] == true
	decoderComplete := ioObject["decoder"] == "COMPLETE"
	if status == "PASSED" {
		if execution["exitCode"] != "0" || execution["failureClass"] != nil || execution["cancelled"] != false || execution["timedOut"] != false || execution["limit"] != nil || execution["signal"] != nil || !cleanup || !sourceCurrent || !toolchainCurrent || !allDrained || !decoderComplete || len(observation.buildEventFailed) != 0 || observation.buildMismatch || observation.openState {
			return reject(runnerFieldMatrix)
		}
		for _, pkg := range requested {
			if observation.packageState[pkg] != "PASSED" {
				return reject(runnerPackageState)
			}
		}
		for _, terminal := range observation.testTerminal {
			if terminal != "PASSED" && terminal != "SKIPPED" {
				return reject(runnerTestState)
			}
		}
		return nil
	}
	if status == "FAILED" {
		if execution["exitCode"] == nil || execution["exitCode"] == "0" || execution["failureClass"] != "ASSERTION_OR_TEST" && execution["failureClass"] != "BUILD" || execution["cancelled"] != false || execution["timedOut"] != false || execution["limit"] != nil || !cleanup || !sourceCurrent || !toolchainCurrent || !allDrained || !decoderComplete || observation.buildMismatch || observation.openState {
			return reject(runnerFieldMatrix)
		}
		hasTestFailure := false
		for _, terminal := range observation.testTerminal {
			if terminal == "FAILED" {
				hasTestFailure = true
			}
		}
		hasPackageFailure := false
		for _, pkg := range requested {
			if observation.packageState[pkg] == "FAILED" {
				hasPackageFailure = true
			}
		}
		if len(observation.buildEventFailed) != 0 && execution["failureClass"] != "BUILD" || len(observation.buildEventFailed) == 0 && (execution["failureClass"] != "ASSERTION_OR_TEST" || !hasTestFailure && !hasPackageFailure) {
			return reject(runnerFieldMatrix)
		}
		return nil
	}
	failureClass, ok := execution["failureClass"].(string)
	if !ok || failureClass == "ASSERTION_OR_TEST" || failureClass == "BUILD" || failureClass == "DISCOVERY" {
		return reject(runnerFieldMatrix)
	}
	if execution["cancelled"] == true && execution["timedOut"] == true {
		return reject(runnerFieldMatrix)
	}
	if !sourceCurrent || !toolchainCurrent {
		if failureClass != "STALE" {
			return reject(runnerFieldMatrix)
		}
		return nil
	}
	if cleanup && allDrained && decoderComplete && execution["cancelled"] == false && execution["timedOut"] == false && execution["limit"] == nil && execution["signal"] == nil && failureClass != "CRASH" && failureClass != "INFRASTRUCTURE" && !observation.buildMismatch && !observation.openState {
		return reject(runnerFieldMatrix)
	}
	if execution["cancelled"] == true && failureClass != "CANCELLATION" && failureClass != "INFRASTRUCTURE" || execution["cancelled"] == false && failureClass == "CANCELLATION" || execution["timedOut"] == true && (failureClass != "TIMEOUT" || execution["limit"] != "RUN_TIME") || execution["timedOut"] == false && (failureClass == "TIMEOUT" || execution["limit"] == "RUN_TIME") {
		return reject(runnerFieldMatrix)
	}
	if execution["limit"] != nil && execution["limit"] != "RUN_TIME" && failureClass != "INFRASTRUCTURE" && failureClass != "UNKNOWN" {
		return reject(runnerFieldMatrix)
	}
	if (!cleanup || !allDrained || !decoderComplete) && failureClass != "INFRASTRUCTURE" && failureClass != "UNKNOWN" && failureClass != "STALE" && failureClass != "CANCELLATION" && failureClass != "TIMEOUT" {
		return reject(runnerFieldMatrix)
	}
	if observation.buildMismatch && failureClass != "INFRASTRUCTURE" && failureClass != "UNKNOWN" {
		return reject(runnerFieldMatrix)
	}
	return nil
}

func hexString(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}

func eventRoot(events []map[string]any) string {
	entries := make([]any, len(events))
	for index, event := range events {
		entries[index] = map[string]any{
			"digest":   strings.TrimPrefix(event["id"].(string), "go-live-event:sha256:"),
			"sequence": strconv.Itoa(index),
		}
	}
	body, _ := canonical(map[string]any{"events": entries})
	return bareDomainDigest("go-event-root", "go-event-root/0", body)
}

func testIdentity(packageName, testName string) string {
	body, _ := canonical(map[string]any{"name": testName, "package": packageName})
	return domainID("go-test", "go-test/0", body)
}

func packageTerminalObjects(observation *eventObservation, requested []string) []any {
	result := make([]any, 0, len(requested))
	for _, pkg := range requested {
		if status := observation.packageState[pkg]; isTerminal(status) {
			result = append(result, map[string]any{"package": pkg, "status": status})
		}
	}
	sortCanonicalValues(result)
	return result
}

func testTerminalObjects(observation *eventObservation) []any {
	result := make([]any, 0, len(observation.testTerminal))
	for test, status := range observation.testTerminal {
		result = append(result, map[string]any{"status": status, "test": test})
	}
	sortCanonicalValues(result)
	return result
}

func buildTerminalObjects(observation *eventObservation) []any {
	result := make([]any, 0, len(observation.buildEventFailed))
	for path := range observation.buildEventFailed {
		result = append(result, map[string]any{"importPath": path, "status": "FAILED"})
	}
	sortCanonicalValues(result)
	return result
}

func sortCanonicalValues(values []any) {
	sort.Slice(values, func(left, right int) bool {
		leftRaw, _ := canonical(values[left])
		rightRaw, _ := canonical(values[right])
		return bytes.Compare(leftRaw, rightRaw) < 0
	})
}

func equalCanonicalArray(value any, expected []any) bool {
	actual, ok := value.([]any)
	if !ok {
		return false
	}
	actualRaw, _ := canonical(actual)
	expectedRaw, _ := canonical(expected)
	return bytes.Equal(actualRaw, expectedRaw)
}

func sortedMapKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func anyStrings(value any) ([]string, error) {
	array, ok := value.([]any)
	if !ok {
		return nil, reject(runnerFieldMatrix)
	}
	result := make([]string, len(array))
	for index, value := range array {
		text, ok := value.(string)
		if !ok {
			return nil, reject(runnerFieldMatrix)
		}
		result[index] = text
	}
	return result, nil
}

func equalAnyStrings(value any, expected []string) bool {
	actual, err := anyStrings(value)
	return err == nil && equalStrings(actual, expected)
}

func emptyArray(value any) bool {
	array, ok := value.([]any)
	return ok && len(array) == 0
}

func optionalNonemptyString(value any) (string, bool, error) {
	if value == nil {
		return "", false, nil
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return "", false, reject(runnerFieldMatrix)
	}
	return text, true, nil
}

func optionalDecimal(value any) (uint64, bool, error) {
	if value == nil {
		return 0, false, nil
	}
	text, ok := value.(string)
	if !ok || !canonicalDecimal(text) {
		return 0, false, reject(runnerFieldMatrix)
	}
	parsed, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, false, reject(runnerFieldMatrix)
	}
	return parsed, true, nil
}

func canonicalDecimal(value string) bool {
	if value == "0" {
		return true
	}
	if value == "" || value[0] < '1' || value[0] > '9' {
		return false
	}
	for _, digit := range value[1:] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}

func decimalValue(value any) bool {
	text, ok := value.(string)
	return ok && canonicalDecimal(text)
}

func isDigestValue(value any) bool {
	text, ok := value.(string)
	return ok && isDigest(text)
}

func isIDValue(value any, kind string) bool {
	text, ok := value.(string)
	return ok && isID(text, kind)
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func oneOfValue(value any, allowed ...string) bool {
	text, ok := value.(string)
	return ok && oneOf(text, allowed...)
}

func isTerminal(state string) bool {
	return oneOf(state, "PASSED", "FAILED", "SKIPPED")
}
