// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

func validTranscriptContext() transcriptContext {
	return transcriptContext{
		ActualEnvironmentSHA256:  zeroDigest,
		CapabilityID:             "go-live-capability:sha256:" + zeroDigest,
		DiscoveredPackagePaths:   []string{"example.test/a"},
		DiscoveryID:              "go-live-discovery:sha256:" + oneDigest,
		ListedPackages:           []string{"go-package:sha256:" + zeroDigest},
		Nonce:                    "0123456789abcdef0123456789abcdef",
		PlanID:                   "go-live-plan:sha256:" + zeroDigest,
		RequestedPackagePatterns: []string{"./..."},
		RequestedRunnerPackages:  []string{"example.test/a"},
		ToolchainID:              "go-toolchain:sha256:" + oneDigest,
	}
}

func emptyTestOutput() map[string]any {
	return map[string]any{"byteCount": "0", "outputType": nil, "sha256": emptyDigest, "text": nil, "truncated": false}
}

func testFact(action string, test *string, duration any) map[string]any {
	return testFactForPackage("example.test/a", action, test, duration)
}

func testFactForPackage(packageName, action string, test *string, duration any) map[string]any {
	return map[string]any{
		"artifactPathSha256": nil, "attribute": nil, "durationNanoseconds": duration,
		"factKind": "TEST", "failedBuild": nil, "output": emptyTestOutput(),
		"package": packageName, "parentTestId": nil, "rawAction": action,
		"sourceAnchor": nil, "test": optional(test), "timeRaw": "2026-08-23T12:00:00-04:00",
		"timeUtc": "2026-08-23T16:00:00Z",
	}
}

func validTranscriptObjects(t *testing.T) ([]map[string]any, map[string]any, transcriptContext) {
	t.Helper()
	context := validTranscriptContext()
	testName := "TestAlpha/sub"
	facts := []map[string]any{
		testFact("start", nil, nil), testFact("run", &testName, nil),
		testFact("pass", &testName, "1000"), testFact("pass", nil, "2000"),
	}
	observed := &eventObservation{observed: map[string]bool{testIdentity("example.test/a", testName): true}}
	wei, err := computeWEI(context, observed)
	if err != nil {
		t.Fatal(err)
	}
	runID := attemptID(context.Nonce, context.PlanID)
	events := make([]map[string]any, len(facts))
	for index, fact := range facts {
		event := map[string]any{
			"fact": fact, "profile": "go-live-event/0", "runId": runID,
			"sequence": strconv.Itoa(index), "wei": wei,
		}
		body, _ := canonical(event)
		event["id"] = domainID("go-live-event", "go-live-event/0", body)
		events[index] = event
	}
	testID := testIdentity("example.test/a", testName)
	run := map[string]any{
		"coverage":          map[string]any{"artifactSha256": nil, "completeness": "ABSENT", "mode": "NONE", "packages": []any{}, "rawRootSha256": nil},
		"ephemeralDeletion": "COMPLETE", "eventCount": "4", "eventRootSha256": eventRoot(events),
		"execution": map[string]any{
			"cancelled": false, "containment": map[string]any{"mechanism": "PROCESS_GROUP_BEST_EFFORT", "qualified": false},
			"durationNanoseconds": "3000", "exitCode": "0", "failureClass": nil, "limit": nil,
			"resources": map[string]any{"cpuMilliseconds": nil, "memoryPeakBytes": nil, "openFilesPeak": nil, "processesPeak": nil},
			"signal":    nil, "status": "PASSED", "timedOut": false,
		},
		"io": map[string]any{
			"decoder": "COMPLETE", "stderrBytes": "0", "stderrDrained": true, "stderrRawSha256": emptyDigest,
			"stdoutBytes": "0", "stdoutDrained": true, "stdoutRawSha256": emptyDigest,
		},
		"persistence": "NONE", "planId": context.PlanID, "profile": "go-live-run/0",
		"qualification": "EXPERIMENTAL_TRANSCRIPT", "runId": runID,
		"scope": map[string]any{
			"buildTerminals": []any{}, "conclusion": "UNKNOWN", "excluded": []any{},
			"executedTests": []any{testID}, "listedPackages": stringArray(context.ListedPackages),
			"observedTests":            []any{testID},
			"packageTerminals":         []any{map[string]any{"package": "example.test/a", "status": "PASSED"}},
			"requestedPackagePatterns": stringArray(context.RequestedPackagePatterns), "skippedTests": []any{},
			"testTerminals":  []any{map[string]any{"status": "PASSED", "test": testID}},
			"unknownReasons": stringArray(mandatoryUnknownReasons),
		},
		"sourceCurrency": "CURRENT", "sourcePostSha256": zeroDigest, "sourcePreSha256": zeroDigest,
		"toolchainCurrency": "CURRENT", "toolchainPostSha256": oneDigest, "toolchainPreSha256": oneDigest,
		"wei": wei,
	}
	rehashRun(run)
	return events, run, context
}

func multiPackageTranscriptObjects(t *testing.T) ([]map[string]any, map[string]any, transcriptContext) {
	t.Helper()
	events, run, context := validTranscriptObjects(t)
	context.DiscoveredPackagePaths = []string{"example.test/0", "example.test/a"}
	context.RequestedRunnerPackages = []string{"example.test/0", "example.test/a"}
	context.ListedPackages = []string{"go-package:sha256:" + zeroDigest, "go-package:sha256:" + oneDigest}
	alpha := "TestAlpha"
	zulu := "TestZulu"
	facts := []map[string]any{
		testFactForPackage("example.test/0", "start", nil, nil),
		testFactForPackage("example.test/0", "run", &zulu, nil),
		testFactForPackage("example.test/0", "pass", &zulu, "1000"),
		testFactForPackage("example.test/0", "run", &alpha, nil),
		testFactForPackage("example.test/0", "skip", &alpha, "500"),
		testFactForPackage("example.test/0", "pass", nil, "2500"),
	}
	for _, fact := range facts {
		events = append(events, map[string]any{
			"fact": fact, "profile": "go-live-event/0", "runId": events[0]["runId"],
			"sequence": strconv.Itoa(len(events)), "wei": events[0]["wei"],
		})
	}
	for _, importPath := range []string{"example.test/z-build", "example.test/a-build"} {
		fact := map[string]any{
			"factKind": "BUILD", "importPath": importPath,
			"output":    map[string]any{"byteCount": "0", "sha256": emptyDigest, "text": nil, "truncated": false},
			"rawAction": "build-fail",
		}
		events = append(events, map[string]any{
			"fact": fact, "profile": "go-live-event/0", "runId": events[0]["runId"],
			"sequence": strconv.Itoa(len(events)), "wei": events[0]["wei"],
		})
	}
	testIDs := []string{
		testIdentity("example.test/a", "TestAlpha/sub"),
		testIdentity("example.test/0", alpha),
		testIdentity("example.test/0", zulu),
	}
	observed := &eventObservation{observed: map[string]bool{testIDs[0]: true, testIDs[1]: true, testIDs[2]: true}}
	wei, err := computeWEI(context, observed)
	if err != nil {
		t.Fatal(err)
	}
	for index, event := range events {
		event["sequence"] = strconv.Itoa(index)
		event["wei"] = wei
		rehashEvent(event)
	}
	sort.Strings(testIDs)
	passed := []string{testIdentity("example.test/a", "TestAlpha/sub"), testIdentity("example.test/0", zulu)}
	sort.Strings(passed)
	scope := run["scope"].(map[string]any)
	scope["buildTerminals"] = []any{
		map[string]any{"importPath": "example.test/a-build", "status": "FAILED"},
		map[string]any{"importPath": "example.test/z-build", "status": "FAILED"},
	}
	scope["executedTests"] = []any{testIDs[0], testIDs[1], testIDs[2]}
	scope["listedPackages"] = stringArray(context.ListedPackages)
	scope["observedTests"] = []any{testIDs[0], testIDs[1], testIDs[2]}
	scope["packageTerminals"] = []any{
		map[string]any{"package": "example.test/0", "status": "PASSED"},
		map[string]any{"package": "example.test/a", "status": "PASSED"},
	}
	scope["skippedTests"] = []any{testIdentity("example.test/0", alpha)}
	scope["testTerminals"] = []any{
		map[string]any{"status": "PASSED", "test": passed[0]},
		map[string]any{"status": "PASSED", "test": passed[1]},
		map[string]any{"status": "SKIPPED", "test": testIdentity("example.test/0", alpha)},
	}
	run["eventCount"] = strconv.Itoa(len(events))
	run["eventRootSha256"] = eventRoot(events)
	run["wei"] = wei
	execution := run["execution"].(map[string]any)
	execution["status"], execution["failureClass"] = "INCOMPLETE", "INFRASTRUCTURE"
	rehashRun(run)
	return events, run, context
}

func rehashEvent(event map[string]any) {
	body, _ := canonical(without(event, "id"))
	event["id"] = domainID("go-live-event", "go-live-event/0", body)
}

func rehashRun(run map[string]any) {
	body, _ := canonical(without(run, "id"))
	run["id"] = domainID("go-live-run", "go-live-run/0", body)
}

func transcriptBytes(t *testing.T, events []map[string]any, run map[string]any) []byte {
	t.Helper()
	var result bytes.Buffer
	for _, object := range append(events, run) {
		raw, err := canonical(object)
		if err != nil {
			t.Fatal(err)
		}
		result.Write(raw)
		result.WriteByte('\n')
	}
	return result.Bytes()
}

func TestEventWEIRunTranscript(t *testing.T) {
	contextRaw, err := os.ReadFile(filepath.Join(sourceDir(t), "testdata", "producer-context.json"))
	if err != nil {
		t.Fatal(err)
	}
	if digest := sha256.Sum256(contextRaw); hex.EncodeToString(digest[:]) != "317ccb8ea3b0884264fb409a49aab512dc54b8ea2985b557c04824495d8af6f5" {
		t.Fatal("producer context bytes changed")
	}
	var context transcriptContext
	if err := json.Unmarshal(contextRaw, &context); err != nil {
		t.Fatal(err)
	}
	discoveryRaw, err := os.ReadFile(filepath.Join(sourceDir(t), "testdata", "receipt-producer-discovery.json"))
	if err != nil {
		t.Fatal(err)
	}
	if digest := sha256.Sum256(discoveryRaw); hex.EncodeToString(digest[:]) != "e735f855264dc76602ec1bd3cf7f08792f47cbf0ee4b2909211d67b44094d2a2" {
		t.Fatal("receipt producer discovery bytes changed")
	}
	discovery, err := VerifyDiscovery(discoveryRaw)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(discovery.Packages))
	ids := make([]string, len(discovery.Packages))
	for index, pack := range discovery.Packages {
		paths[index], ids[index] = pack.ImportPath, pack.ID
	}
	sort.Strings(paths)
	sort.Strings(ids)
	if context.ActualEnvironmentSHA256 != discovery.EnvironmentSHA256 || context.DiscoveryID != discovery.ID ||
		context.ToolchainID != discovery.ToolchainID || !equalStrings(context.DiscoveredPackagePaths, paths) ||
		!equalStrings(context.ListedPackages, ids) || !equalStrings(context.RequestedPackagePatterns, discovery.Argv[5:]) ||
		!equalStrings(context.RequestedRunnerPackages, discovery.RequestedRunnerPackages) {
		t.Fatal("producer receipt context is not bound to producer discovery")
	}
	golden, err := os.ReadFile(filepath.Join(sourceDir(t), "testdata", "valid-transcript.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyTranscript(golden, context); err != nil {
		t.Fatal(err)
	}
	if digest := sha256.Sum256(golden); hex.EncodeToString(digest[:]) != "62b1946e676c242c03697e33cf89988db19f3f9e5287a0160904b211f2bfbf11" {
		t.Fatal("producer transcript bytes changed")
	}
	if got := attemptID(context.Nonce, context.PlanID); got != "go-live-attempt:sha256:e084e7a42e3b38d008e11a3db8046da1568a0ae003715e78949f5beb500ddea6" {
		t.Fatalf("fixed attempt id changed: %s", got)
	}
	lines := bytes.Split(bytes.TrimSuffix(golden, []byte{'\n'}), []byte{'\n'})
	run, err := parseCanonical(append(append([]byte(nil), lines[len(lines)-1]...), '\n'))
	if err != nil {
		t.Fatal(err)
	}
	if run["eventRootSha256"] != "a227968dbd3ad132d4a29fd967ff844f7e373376a0762d84c30bb2fc1ece6f39" || run["id"] != "go-live-run:sha256:50800b62465b6ea18c3c3569c091a95dcfb80c6f000a2d3afd167aee3a4d973f" {
		t.Fatalf("fixed terminal identities changed: %v %v", run["eventRootSha256"], run["id"])
	}
}

func TestEventTranscriptAdversaries(t *testing.T) {
	cases := []struct {
		name string
		want rejectCode
		edit func([]map[string]any, map[string]any, *transcriptContext)
	}{
		{"sequence-gap", identityMismatch, func(events []map[string]any, _ map[string]any, _ *transcriptContext) {
			events[1]["sequence"] = "2"
			rehashEvent(events[1])
		}},
		{"wrong-wei", identityMismatch, func(events []map[string]any, _ map[string]any, _ *transcriptContext) {
			events[0]["wei"] = "workspace-execution:sha256:" + zeroDigest
			rehashEvent(events[0])
		}},
		{"pause-without-run", runnerTestState, func(events []map[string]any, _ map[string]any, _ *transcriptContext) {
			events[1]["fact"].(map[string]any)["rawAction"] = "pause"
			rehashEvent(events[1])
		}},
		{"duration-overflow", runnerFieldMatrix, func(events []map[string]any, _ map[string]any, _ *transcriptContext) {
			events[2]["fact"].(map[string]any)["durationNanoseconds"] = "18446744073709551616"
			rehashEvent(events[2])
		}},
		{"event-root", identityMismatch, func(_ []map[string]any, run map[string]any, _ *transcriptContext) {
			run["eventRootSha256"] = zeroDigest
			rehashRun(run)
		}},
		{"scope-forgery", runnerFieldMatrix, func(_ []map[string]any, run map[string]any, _ *transcriptContext) {
			run["scope"].(map[string]any)["executedTests"] = []any{}
			rehashRun(run)
		}},
		{"run-id", identityMismatch, func(_ []map[string]any, run map[string]any, _ *transcriptContext) {
			run["id"] = "go-live-run:sha256:" + zeroDigest
		}},
		{"listed-package", runnerFieldMatrix, func(_ []map[string]any, run map[string]any, _ *transcriptContext) {
			run["scope"].(map[string]any)["listedPackages"] = []any{}
			rehashRun(run)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			events, run, context := validTranscriptObjects(t)
			test.edit(events, run, &context)
			err := VerifyTranscript(transcriptBytes(t, events, run), context)
			if codeOf(err) != test.want {
				t.Fatalf("error = %v, want %s", err, test.want)
			}
		})
	}
}

func TestEventCountMustMatchObservedEvents(t *testing.T) {
	events, run, context := validTranscriptObjects(t)
	run["eventCount"] = "3"
	rehashRun(run)
	if err := VerifyTranscript(transcriptBytes(t, events, run), context); codeOf(err) != identityMismatch {
		t.Fatalf("event-count-only forgery error = %v, want %s", err, identityMismatch)
	}
}

func TestVerifierEnforcesCanonicalMultiPackageScopeOrdering(t *testing.T) {
	events, run, context := multiPackageTranscriptObjects(t)
	if err := VerifyTranscript(transcriptBytes(t, events, run), context); err != nil {
		t.Fatalf("multi-package baseline rejected: %v", err)
	}
	scope := run["scope"].(map[string]any)
	wantPackages := []any{
		map[string]any{"package": "example.test/0", "status": "PASSED"},
		map[string]any{"package": "example.test/a", "status": "PASSED"},
	}
	if !equalCanonicalArray(scope["packageTerminals"], wantPackages) {
		t.Fatalf("package terminals = %v", scope["packageTerminals"])
	}
	wantBuilds := []any{
		map[string]any{"importPath": "example.test/a-build", "status": "FAILED"},
		map[string]any{"importPath": "example.test/z-build", "status": "FAILED"},
	}
	if !equalCanonicalArray(scope["buildTerminals"], wantBuilds) {
		t.Fatalf("build terminals = %v", scope["buildTerminals"])
	}
	wantTests := []string{
		testIdentity("example.test/a", "TestAlpha/sub"),
		testIdentity("example.test/0", "TestAlpha"),
		testIdentity("example.test/0", "TestZulu"),
	}
	sort.Strings(wantTests)
	if !equalAnyStrings(scope["observedTests"], wantTests) || !equalAnyStrings(scope["executedTests"], wantTests) {
		t.Fatalf("observed/executed tests = %v / %v", scope["observedTests"], scope["executedTests"])
	}
	for _, field := range []string{"buildTerminals", "packageTerminals", "observedTests", "executedTests", "testTerminals"} {
		t.Run(field, func(t *testing.T) {
			events, run, context := multiPackageTranscriptObjects(t)
			values := run["scope"].(map[string]any)[field].([]any)
			values[0], values[1] = values[1], values[0]
			rehashRun(run)
			if err := VerifyTranscript(transcriptBytes(t, events, run), context); codeOf(err) != runnerFieldMatrix {
				t.Fatalf("out-of-order %s error = %v, want %s", field, err, runnerFieldMatrix)
			}
		})
	}
}

func TestTranscriptCanonicalAndTerminalBoundaries(t *testing.T) {
	events, run, context := validTranscriptObjects(t)
	raw := transcriptBytes(t, events, run)
	duplicate := bytes.Replace(raw, []byte(`{"fact":`), []byte(`{"fact":null,"fact":`), 1)
	if err := VerifyTranscript(duplicate, context); codeOf(err) != canonicalDuplicateKey {
		t.Fatalf("duplicate = %v", err)
	}
	postTerminal := append(append([]byte(nil), raw...), []byte("{}\n")...)
	if err := VerifyTranscript(postTerminal, context); err == nil {
		t.Fatal("accepted bytes after terminal")
	}
	if err := VerifyTranscript(bytes.TrimSuffix(raw, []byte{'\n'}), context); err == nil {
		t.Fatal("accepted transcript without LF")
	}
	oversizedEvent := append(bytes.Repeat([]byte{'x'}, maxDocumentBytes), '\n')
	oversizedEvent = append(oversizedEvent, []byte("{}\n")...)
	if err := VerifyTranscript(oversizedEvent, context); codeOf(err) != limitExceeded {
		t.Fatalf("oversized event = %v", err)
	}
}

func TestIncompleteLimitCausality(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
		edit  func(map[string]any)
	}{
		{"run-time-timeout", true, func(run map[string]any) {
			execution := run["execution"].(map[string]any)
			execution["status"], execution["exitCode"], execution["failureClass"] = "INCOMPLETE", nil, "TIMEOUT"
			execution["timedOut"], execution["limit"] = true, "RUN_TIME"
		}},
		{"output-infrastructure", true, func(run map[string]any) {
			execution := run["execution"].(map[string]any)
			execution["status"], execution["exitCode"], execution["failureClass"] = "INCOMPLETE", nil, "INFRASTRUCTURE"
			execution["limit"] = "OUTPUT_BYTES"
			run["io"].(map[string]any)["decoder"] = "TRUNCATED"
		}},
		{"provider-failure-without-wire-field", true, func(run map[string]any) {
			execution := run["execution"].(map[string]any)
			execution["status"], execution["failureClass"] = "INCOMPLETE", "INFRASTRUCTURE"
		}},
		{"cancelled-unknown-still-rejected", false, func(run map[string]any) {
			execution := run["execution"].(map[string]any)
			execution["status"], execution["exitCode"], execution["failureClass"] = "INCOMPLETE", nil, "UNKNOWN"
			execution["cancelled"] = true
			run["io"].(map[string]any)["decoder"] = "REJECTED"
		}},
		{"timeout-wrong-limit", false, func(run map[string]any) {
			execution := run["execution"].(map[string]any)
			execution["status"], execution["exitCode"], execution["failureClass"] = "INCOMPLETE", nil, "TIMEOUT"
			execution["timedOut"], execution["limit"] = true, "OUTPUT_BYTES"
		}},
		{"product-class-incomplete", false, func(run map[string]any) {
			execution := run["execution"].(map[string]any)
			execution["status"], execution["exitCode"], execution["failureClass"] = "INCOMPLETE", "1", "ASSERTION_OR_TEST"
			run["ephemeralDeletion"] = "INCOMPLETE"
		}},
		{"stale-dominates-other-causes", true, func(run map[string]any) {
			execution := run["execution"].(map[string]any)
			execution["status"], execution["exitCode"], execution["failureClass"] = "INCOMPLETE", nil, "STALE"
			execution["cancelled"], execution["timedOut"], execution["limit"] = false, true, "RUN_TIME"
			run["sourceCurrency"], run["sourcePostSha256"] = "STALE", oneDigest
		}},
		{"stale-cannot-be-cancelled-and-timed-out", false, func(run map[string]any) {
			execution := run["execution"].(map[string]any)
			execution["status"], execution["exitCode"], execution["failureClass"] = "INCOMPLETE", nil, "STALE"
			execution["cancelled"], execution["timedOut"], execution["limit"] = true, true, "RUN_TIME"
			run["sourceCurrency"], run["sourcePostSha256"] = "STALE", oneDigest
		}},
		{"stale-wrong-class", false, func(run map[string]any) {
			execution := run["execution"].(map[string]any)
			execution["status"], execution["exitCode"], execution["failureClass"] = "INCOMPLETE", nil, "TIMEOUT"
			execution["timedOut"], execution["limit"] = true, "RUN_TIME"
			run["sourceCurrency"], run["sourcePostSha256"] = "STALE", oneDigest
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events, run, context := validTranscriptObjects(t)
			test.edit(run)
			rehashRun(run)
			err := VerifyTranscript(transcriptBytes(t, events, run), context)
			if test.valid && err != nil {
				t.Fatalf("valid incomplete receipt rejected: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("invalid incomplete receipt accepted")
			}
		})
	}
}

func TestProviderCancellationDifferentialsAllowOnlyObservedTerminals(t *testing.T) {
	t.Run("zero-events", func(t *testing.T) {
		_, run, context := validTranscriptObjects(t)
		observation := &eventObservation{observed: map[string]bool{}}
		wei, err := computeWEI(context, observation)
		if err != nil {
			t.Fatal(err)
		}
		run["eventCount"], run["eventRootSha256"], run["wei"] = "0", eventRoot(nil), wei
		execution := run["execution"].(map[string]any)
		execution["cancelled"], execution["exitCode"], execution["failureClass"], execution["status"] = true, nil, "CANCELLATION", "INCOMPLETE"
		run["io"].(map[string]any)["decoder"] = "REJECTED"
		scope := run["scope"].(map[string]any)
		for _, key := range []string{"executedTests", "observedTests", "packageTerminals", "skippedTests", "testTerminals"} {
			scope[key] = []any{}
		}
		rehashRun(run)
		if err := VerifyTranscript(transcriptBytes(t, nil, run), context); err != nil {
			t.Fatalf("zero-event cancellation rejected: %v", err)
		}
	})

	t.Run("partial-open-package-and-test", func(t *testing.T) {
		events, run, context := validTranscriptObjects(t)
		events = events[:2]
		run["eventCount"], run["eventRootSha256"] = "2", eventRoot(events)
		execution := run["execution"].(map[string]any)
		execution["cancelled"], execution["exitCode"], execution["failureClass"], execution["status"] = true, nil, "CANCELLATION", "INCOMPLETE"
		run["io"].(map[string]any)["decoder"] = "REJECTED"
		scope := run["scope"].(map[string]any)
		scope["packageTerminals"], scope["testTerminals"] = []any{}, []any{}
		rehashRun(run)
		if err := VerifyTranscript(transcriptBytes(t, events, run), context); err != nil {
			t.Fatalf("partial cancellation rejected: %v", err)
		}

		execution["cancelled"], execution["exitCode"], execution["failureClass"], execution["status"] = false, "0", nil, "PASSED"
		run["io"].(map[string]any)["decoder"] = "COMPLETE"
		rehashRun(run)
		if err := VerifyTranscript(transcriptBytes(t, events, run), context); err == nil {
			t.Fatal("open package/test state accepted as PASSED")
		}
	})
}

func TestFailedBuildClassPrecedence(t *testing.T) {
	_, run, context := validTranscriptObjects(t)
	execution := run["execution"].(map[string]any)
	execution["status"], execution["exitCode"], execution["failureClass"] = "FAILED", "1", "ASSERTION_OR_TEST"
	observation := &eventObservation{
		buildEventFailed: map[string]bool{"example.test/dependency": true},
		packageState:     map[string]string{"example.test/a": "FAILED"},
		testTerminal:     map[string]string{"go-test:sha256:" + zeroDigest: "FAILED"},
	}
	ioObject := run["io"].(map[string]any)
	if err := verifyExecution(execution, observation, context.RequestedRunnerPackages, true, true, true, ioObject); err == nil {
		t.Fatal("accepted assertion class in the presence of a build terminal")
	}
	execution["failureClass"] = "BUILD"
	if err := verifyExecution(execution, observation, context.RequestedRunnerPackages, true, true, true, ioObject); err != nil {
		t.Fatalf("build-class failure rejected: %v", err)
	}
}

func TestUnreferencedBuildFailureClosesIncomplete(t *testing.T) {
	events, run, context := validTranscriptObjects(t)
	buildFact := map[string]any{
		"factKind": "BUILD", "importPath": "example.test/dependency",
		"output":    map[string]any{"byteCount": "0", "sha256": emptyDigest, "text": nil, "truncated": false},
		"rawAction": "build-fail",
	}
	buildEvent := map[string]any{
		"fact": buildFact, "profile": "go-live-event/0", "runId": events[0]["runId"],
		"sequence": "0", "wei": events[0]["wei"],
	}
	rehashEvent(buildEvent)
	events = append([]map[string]any{buildEvent}, events...)
	for index := 1; index < len(events); index++ {
		events[index]["sequence"] = strconv.Itoa(index)
		rehashEvent(events[index])
	}
	run["eventCount"] = strconv.Itoa(len(events))
	run["eventRootSha256"] = eventRoot(events)
	run["scope"].(map[string]any)["buildTerminals"] = []any{map[string]any{"importPath": "example.test/dependency", "status": "FAILED"}}
	execution := run["execution"].(map[string]any)
	execution["status"], execution["exitCode"], execution["failureClass"] = "INCOMPLETE", "1", "INFRASTRUCTURE"
	rehashRun(run)
	if err := VerifyTranscript(transcriptBytes(t, events, run), context); err != nil {
		t.Fatalf("handled build mismatch receipt rejected: %v", err)
	}
	execution["status"], execution["failureClass"] = "FAILED", "BUILD"
	rehashRun(run)
	if err := VerifyTranscript(transcriptBytes(t, events, run), context); err == nil {
		t.Fatal("build mismatch accepted as FAILED")
	}
}

func FuzzVerifyTranscript(f *testing.F) {
	f.Add([]byte("{}\n"))
	context := validTranscriptContext()
	f.Fuzz(func(t *testing.T, raw []byte) {
		_ = VerifyTranscript(raw, context)
	})
}
