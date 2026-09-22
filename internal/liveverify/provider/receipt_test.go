package provider

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/godiscovery"
	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
	"github.com/Beamfall/corvint/internal/liveverify/gotest"
)

func TestComposeReceiptExactVector(t *testing.T) {
	input := validReceiptInput(t)
	receipt, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyReceipt(input, receipt); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(receipt.Transcript, receipt.Run) || bytes.Count(receipt.Transcript, []byte{'\n'}) != len(input.Events)+1 {
		t.Fatal("terminal framing is not exact")
	}
	for index, event := range receipt.Events {
		if !bytes.HasSuffix(event, []byte{'\n'}) || !bytes.Contains(event, []byte(`"sequence":"`+string(rune('0'+index))+`"`)) {
			t.Fatalf("event %d framing/sequence is not exact: %s", index, event)
		}
	}
	if !bytes.Contains(receipt.Run, []byte(`"qualification":"EXPERIMENTAL_TRANSCRIPT"`)) ||
		!bytes.Contains(receipt.Run, []byte(`"conclusion":"UNKNOWN"`)) ||
		!bytes.Contains(receipt.Run, []byte(`"mode":"NONE"`)) ||
		bytes.Contains(receipt.Run, []byte(`"qualified":true`)) {
		t.Fatalf("run overclaims qualification, scope, coverage, or containment: %s", receipt.Run)
	}
	if receipt.RunID != "go-live-attempt:sha256:e084e7a42e3b38d008e11a3db8046da1568a0ae003715e78949f5beb500ddea6" ||
		receipt.WEI != "workspace-execution:sha256:8e773615dbb9e914229879301cc89a1382ee86668113f64df83e1a7bbd0cb195" {
		t.Fatalf("fixed identity vector changed: run=%s wei=%s", receipt.RunID, receipt.WEI)
	}
	for index, want := range []string{
		"go-live-event:sha256:7875857baa8d0389ba81c40268c8ecbc6803adc869ca816aa1a633ad84b49980",
		"go-live-event:sha256:b8c3525407cb644f69cfcbd3b43a4f6f370ce98c5e4aea98718f1532f70743d7",
		"go-live-event:sha256:11bd552be42f1dacfad9a25b1124b5e79eb0d46a0fe414a10f73b51363e88ecf",
		"go-live-event:sha256:2bc77b883478e8a9baebec6772aa60121570428064b5cfb4753c1b04b9f4ec39",
	} {
		if !bytes.Contains(receipt.Events[index], []byte(`"id":"`+want+`"`)) {
			t.Fatalf("fixed event %d identity changed: %s", index, receipt.Events[index])
		}
	}
	if !bytes.Contains(receipt.Run, []byte(`"eventRootSha256":"a227968dbd3ad132d4a29fd967ff844f7e373376a0762d84c30bb2fc1ece6f39"`)) ||
		!bytes.Contains(receipt.Run, []byte(`"id":"go-live-run:sha256:50800b62465b6ea18c3c3569c091a95dcfb80c6f000a2d3afd167aee3a4d973f"`)) {
		t.Fatalf("fixed terminal vector changed: %s", receipt.Run)
	}
}

func TestReceiptDeterministicCanonicalOrdering(t *testing.T) {
	input := validReceiptInput(t)
	first, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ComposeReceipt(cloneReceiptInput(input))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Transcript, second.Transcript) {
		t.Fatal("semantically unordered discovery packages changed receipt bytes")
	}
}

func TestReceiptMultiPackageScopeOrderingAndObservedTestCount(t *testing.T) {
	input := multiPackageReceiptInput(t)
	receipt, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	var run map[string]any
	if err := json.Unmarshal(receipt.Run, &run); err != nil {
		t.Fatal(err)
	}
	scope := run["scope"].(map[string]any)
	wantPackages := []any{
		map[string]any{"package": "example.test/0", "status": "PASSED"},
		map[string]any{"package": "example.test/a", "status": "PASSED"},
	}
	if !reflect.DeepEqual(scope["packageTerminals"], wantPackages) {
		t.Fatalf("package terminals = %v", scope["packageTerminals"])
	}
	wantBuilds := []any{
		map[string]any{"importPath": "example.test/a-build", "status": "FAILED"},
		map[string]any{"importPath": "example.test/z-build", "status": "FAILED"},
	}
	if !reflect.DeepEqual(scope["buildTerminals"], wantBuilds) {
		t.Fatalf("build terminals = %v", scope["buildTerminals"])
	}
	alpha := testID("example.test/0", "TestAlpha")
	zeroZulu := testID("example.test/0", "TestZulu")
	aZulu := testID("example.test/a", "TestZulu")
	wantTests := []string{alpha, zeroZulu, aZulu}
	sort.Strings(wantTests)
	if !reflect.DeepEqual(scope["observedTests"], []any{wantTests[0], wantTests[1], wantTests[2]}) ||
		!reflect.DeepEqual(scope["executedTests"], []any{wantTests[0], wantTests[1], wantTests[2]}) {
		t.Fatalf("observed/executed tests = %v / %v", scope["observedTests"], scope["executedTests"])
	}
	passed := []string{zeroZulu, aZulu}
	sort.Strings(passed)
	wantTerminals := []any{
		map[string]any{"status": "PASSED", "test": passed[0]},
		map[string]any{"status": "PASSED", "test": passed[1]},
		map[string]any{"status": "SKIPPED", "test": alpha},
	}
	if !reflect.DeepEqual(scope["testTerminals"], wantTerminals) {
		t.Fatalf("test terminals = %v", scope["testTerminals"])
	}
	wrongCount := cloneReceiptInput(input)
	wrongCount.Observation.TestCount = uint64(len(input.Discovery.RequestedRunnerPackages))
	if _, err := ComposeReceipt(wrongCount); !errors.Is(err, ErrReceiptInvalid) {
		t.Fatalf("requested-package count accepted as observed-test count: %v", err)
	}
}

func TestReceiptCoverageIsDerivedWithoutChangingAbsentBytes(t *testing.T) {
	input := validReceiptInput(t)
	absent, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	wantAbsent := `"coverage":{"artifactSha256":null,"completeness":"ABSENT","mode":"NONE","packages":[],"rawRootSha256":null}`
	if !bytes.Contains(absent.Run, []byte(wantAbsent)) {
		t.Fatalf("absent coverage bytes changed: %s", absent.Run)
	}

	input.Runner.Coverage = gorunner.CoverageObservation{
		ArtifactSHA256: digestForReceipt("canonical profile"), Completeness: gorunner.CoverageComplete,
		Mode: "UNIT_COVERPROFILE_SET", Packages: []string{"example.test/a"}, RawRootSHA256: digestForReceipt("raw profile"),
	}
	covered, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	wantCovered := `"coverage":{"artifactSha256":"` + digestForReceipt("canonical profile") +
		`","completeness":"COMPLETE","mode":"UNIT_COVERPROFILE_SET","packages":["example.test/a"],"rawRootSha256":"` +
		digestForReceipt("raw profile") + `"}`
	if !bytes.Contains(covered.Run, []byte(wantCovered)) {
		t.Fatalf("covered bytes are not canonical: %s", covered.Run)
	}
	if absent.RunID != covered.RunID || absent.WEI != covered.WEI || !bytes.Equal(absent.Events[0], covered.Events[0]) {
		t.Fatal("coverage changed attempt, WEI, or event identity")
	}
	if bytes.Equal(absent.Run, covered.Run) {
		t.Fatal("coverage did not change the terminal document")
	}
}

func TestReceiptZeroObservedCoverageIsNotAbsent(t *testing.T) {
	input := validReceiptInput(t)
	input.Runner.Coverage = gorunner.CoverageObservation{
		ArtifactSHA256: digestForReceipt("mode: set\n"), Completeness: gorunner.CoverageComplete,
		Mode: "UNIT_COVERPROFILE_SET", Packages: []string{}, RawRootSHA256: digestForReceipt("mode: set\n"),
	}
	receipt, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(receipt.Run, []byte(`"completeness":"COMPLETE","mode":"UNIT_COVERPROFILE_SET","packages":[]`)) ||
		bytes.Contains(receipt.Run, []byte(`"completeness":"ABSENT"`)) {
		t.Fatalf("zero observed coverage collapsed to absent: %s", receipt.Run)
	}
}

func TestReceiptCoverageCompletenessFailsClosedAfterRun(t *testing.T) {
	tests := []struct {
		name string
		edit func(*ReceiptInput)
	}{
		{"nonzero", func(input *ReceiptInput) { input.Runner.ExitCode = 1 }},
		{"timeout", func(input *ReceiptInput) { input.Runner.TimedOut = true }},
		{"cancellation", func(input *ReceiptInput) { input.Runner.Cancelled = true }},
		{"truncation", func(input *ReceiptInput) { input.Decoder = DecoderTruncated }},
		{"source drift", func(input *ReceiptInput) { input.SourcePostSHA256 = digestForReceipt("changed source") }},
		{"toolchain drift", func(input *ReceiptInput) { input.ToolchainPostSHA256 = digestForReceipt("changed toolchain") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validReceiptInput(t)
			input.Runner.Coverage = gorunner.CoverageObservation{
				ArtifactSHA256: digestForReceipt("canonical"), Completeness: gorunner.CoverageComplete,
				Mode: "UNIT_COVERPROFILE_SET", RawRootSHA256: digestForReceipt("raw"),
			}
			test.edit(&input)
			receipt, err := ComposeReceipt(input)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(receipt.Run, []byte(`"coverage":{"artifactSha256":"`+digestForReceipt("canonical")+`","completeness":"PARTIAL"`)) {
				t.Fatalf("coverage was not demoted: %s", receipt.Run)
			}
		})
	}
}

func TestReceiptRejectsMutatedDiscoveryWithRetainedID(t *testing.T) {
	input := validReceiptInput(t)
	input.Discovery.Packages[0].ImportPath = "example.test/forged"
	if _, err := ComposeReceipt(input); err == nil {
		t.Fatal("accepted mutated discovery under retained identity")
	}
}

func TestWriteProducerFixture(t *testing.T) {
	directory := os.Getenv("CORVINT_GO_LIVE_PRODUCER_FIXTURE")
	if directory == "" {
		t.Skip("explicit fixture export not requested")
	}
	input := validReceiptInput(t)
	receipt, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "transcript.jsonl"), receipt.Transcript, 0o600); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(input.Discovery.Packages))
	listed := make([]string, len(input.Discovery.Packages))
	for index, pack := range input.Discovery.Packages {
		paths[index], listed[index] = pack.ImportPath, pack.ID
	}
	context, err := json.Marshal(map[string]any{
		"actualEnvironmentSha256": input.ActualEnvironmentSHA256, "capabilityId": input.CapabilityID,
		"discoveredPackagePaths": paths, "discoveryId": input.Discovery.ID, "listedPackages": listed,
		"nonce": input.Nonce, "planId": input.PlanID, "requestedPackagePatterns": input.Discovery.Argv[5:],
		"requestedRunnerPackages": input.Discovery.RequestedRunnerPackages, "toolchainId": input.Discovery.ToolchainID,
	})
	if err != nil {
		t.Fatal(err)
	}
	context = append(context, '\n')
	if err := os.WriteFile(filepath.Join(directory, "context.json"), context, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "receipt-discovery.json"), input.DiscoveryCanonical, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReceiptRejectsIncompleteObservationAsPass(t *testing.T) {
	input := validReceiptInput(t)
	input.Observation.IncompleteReasons = []string{"BENCHMARK_ACTION"}
	receipt, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(receipt.Run, []byte(`"status":"INCOMPLETE"`)) || !bytes.Contains(receipt.Run, []byte(`"failureClass":"INFRASTRUCTURE"`)) {
		t.Fatalf("incomplete observation was not closed as incomplete: %s", receipt.Run)
	}
}

func TestReceiptProviderFailureCannotPass(t *testing.T) {
	input := validReceiptInput(t)
	input.ProviderFailure = true
	receipt, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"failureClass":"INFRASTRUCTURE"`, `"sourceCurrency":"CURRENT"`, `"status":"INCOMPLETE"`, `"toolchainCurrency":"CURRENT"`} {
		if !bytes.Contains(receipt.Run, []byte(want)) {
			t.Fatalf("missing %s in %s", want, receipt.Run)
		}
	}
	if bytes.Contains(receipt.Run, []byte(`,"status":"PASSED","timedOut":`)) {
		t.Fatalf("provider failure produced a pass: %s", receipt.Run)
	}
}

func TestReceiptZeroEventCancellation(t *testing.T) {
	input := validReceiptInput(t)
	input.Events = nil
	input.Observation = gotest.Observation{}
	input.Decoder = DecoderRejected
	input.Runner.Exited = false
	input.Runner.ExitCode = -1
	input.Runner.Cancelled = true
	receipt, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipt.Events) != 0 || !bytes.Equal(receipt.Transcript, receipt.Run) ||
		!bytes.Contains(receipt.Run, []byte(`"eventCount":"0"`)) ||
		!bytes.Contains(receipt.Run, []byte(`"failureClass":"CANCELLATION"`)) {
		t.Fatalf("zero-event cancellation was not closed canonically: %s", receipt.Transcript)
	}
}

func TestReceiptPartialOpenCancellationRetainsObservedEvents(t *testing.T) {
	input := validReceiptInput(t)
	input.Events = append([]gotest.Event(nil), input.Events[:2]...)
	var eventBytes uint64
	for _, event := range input.Events {
		fact, err := event.CanonicalFactBody()
		if err != nil {
			t.Fatal(err)
		}
		eventBytes += uint64(len(fact))
	}
	input.Observation = gotest.Observation{
		Events: input.Events, EventCount: 2, EventBytes: eventBytes, TestCount: 1,
		Packages: []gotest.PackageState{{Name: "example.test/a", Status: "running", Tests: []gotest.TestState{{Name: "TestAlpha/sub", Status: "running"}}}},
	}
	input.Decoder = DecoderRejected
	input.Runner.Exited = false
	input.Runner.ExitCode = -1
	input.Runner.Cancelled = true
	receipt, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipt.Events) != 2 || bytes.Contains(receipt.Run, []byte(`"packageTerminals":[{`)) ||
		bytes.Contains(receipt.Run, []byte(`"testTerminals":[{`)) ||
		!bytes.Contains(receipt.Run, []byte(`"failureClass":"CANCELLATION"`)) {
		t.Fatalf("partial-open cancellation did not preserve observed-only truth: %s", receipt.Transcript)
	}
	if err := VerifyReceipt(input, receipt); err != nil {
		t.Fatal(err)
	}
}

func TestReceiptForgeryRejections(t *testing.T) {
	base := validReceiptInput(t)
	valid, err := ComposeReceipt(base)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		edit func(*ReceiptInput, *Receipt)
	}{
		{"sequence gap", func(input *ReceiptInput, _ *Receipt) { input.Events[1].Sequence = 2 }},
		{"environment", func(input *ReceiptInput, _ *Receipt) { input.ActualEnvironmentSHA256 = digestForReceipt("other") }},
		{"source stale", func(input *ReceiptInput, _ *Receipt) { input.SourcePostSHA256 = digestForReceipt("stale") }},
		{"duplicate package", func(input *ReceiptInput, _ *Receipt) {
			input.Discovery.Packages[1].ImportPath = input.Discovery.Packages[0].ImportPath
		}},
		{"event id", func(_ *ReceiptInput, receipt *Receipt) { receipt.Events[0][20] ^= 1 }},
		{"terminal bytes", func(_ *ReceiptInput, receipt *Receipt) {
			receipt.Transcript = append(receipt.Transcript, []byte("{}\n")...)
		}},
		{"run id", func(_ *ReceiptInput, receipt *Receipt) {
			receipt.RunID = "go-live-attempt:sha256:" + digestForReceipt("forged")
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := cloneReceiptInput(base)
			receipt := cloneReceipt(valid)
			test.edit(&input, &receipt)
			if test.name == "source stale" {
				stale, composeErr := ComposeReceipt(input)
				if composeErr != nil || !bytes.Contains(stale.Run, []byte(`"failureClass":"STALE"`)) || !bytes.Contains(stale.Run, []byte(`"status":"INCOMPLETE"`)) {
					t.Fatalf("stale identity not closed safely: %v %s", composeErr, stale.Run)
				}
				return
			}
			if test.name == "environment" {
				if err := VerifyReceipt(input, receipt); err == nil {
					t.Fatal("forged attachment verified")
				}
				return
			}
			if test.name == "event id" || test.name == "terminal bytes" || test.name == "run id" {
				if err := VerifyReceipt(input, receipt); err == nil {
					t.Fatal("forged receipt verified")
				}
				return
			}
			if _, err := ComposeReceipt(input); err == nil {
				t.Fatal("forged input composed")
			}
		})
	}
}

func TestReceiptCancellationAndUnknownPostIdentity(t *testing.T) {
	input := validReceiptInput(t)
	input.Runner.Cancelled = true
	input.Runner.Exited = false
	input.Runner.ExitCode = -1
	input.SourcePostSHA256 = ""
	input.ToolchainPostSHA256 = ""
	input.Decoder = DecoderRejected
	current := input
	current.SourcePostSHA256 = current.SourcePreSHA256
	current.ToolchainPostSHA256 = current.ToolchainPreSHA256
	currentReceipt, err := ComposeReceipt(current)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(currentReceipt.Run, []byte(`"failureClass":"CANCELLATION"`)) {
		t.Fatalf("pure cancellation misclassified: %s", currentReceipt.Run)
	}
	writeOptionalReceiptFixture(t, "current-cancellation", currentReceipt.Transcript)
	receipt, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"failureClass":"STALE"`, `"status":"INCOMPLETE"`, `"sourceCurrency":"UNKNOWN"`, `"toolchainCurrency":"UNKNOWN"`} {
		if !bytes.Contains(receipt.Run, []byte(want)) {
			t.Fatalf("missing %s in %s", want, receipt.Run)
		}
	}
	writeOptionalReceiptFixture(t, "unknown-identity-cancellation", receipt.Transcript)
}

func TestReceiptStaleDominatesCancellationAndLimit(t *testing.T) {
	input := validReceiptInput(t)
	input.SourcePostSHA256 = digestForReceipt("stale")
	input.Runner.Cancelled = true
	input.Runner.OutputLimitExceeded = true
	input.Decoder = DecoderTruncated
	receipt, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"cancelled":true`, `"failureClass":"STALE"`, `"limit":"OUTPUT_BYTES"`, `"status":"INCOMPLETE"`} {
		if !bytes.Contains(receipt.Run, []byte(want)) {
			t.Fatalf("missing %s in %s", want, receipt.Run)
		}
	}
	writeOptionalReceiptFixture(t, "stale-cancellation-output-limit", receipt.Transcript)
}

func writeOptionalReceiptFixture(t *testing.T, name string, transcript []byte) {
	t.Helper()
	if directory := os.Getenv("CORVINT_RECEIPT_DOMINANCE_FIXTURES"); directory != "" {
		if err := os.WriteFile(filepath.Join(directory, name+".jsonl"), transcript, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func validReceiptInput(t *testing.T) ReceiptInput {
	t.Helper()
	discovery, discoveryCanonical := receiptDiscovery(t)
	empty := digestForReceipt("")
	when := time.Date(2026, 8, 23, 16, 0, 0, 0, time.UTC)
	events := []gotest.Event{
		{Sequence: 0, Kind: gotest.TestEvent, Action: "start", Package: "example.test/a", RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
		{Sequence: 1, Kind: gotest.TestEvent, Action: "run", Package: "example.test/a", Test: "TestAlpha/sub", RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
		{Sequence: 2, Kind: gotest.TestEvent, Action: "pass", Package: "example.test/a", Test: "TestAlpha/sub", ElapsedNS: 1000, HasElapsed: true, RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
		{Sequence: 3, Kind: gotest.TestEvent, Action: "pass", Package: "example.test/a", ElapsedNS: 2000, HasElapsed: true, RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
	}
	var eventBytes uint64
	for _, event := range events {
		body, err := event.CanonicalFactBody()
		if err != nil {
			t.Fatal(err)
		}
		eventBytes += uint64(len(body))
	}
	return ReceiptInput{
		ActualEnvironmentSHA256: digestForReceipt("environment"),
		CapabilityID:            "go-live-capability:sha256:" + digestForReceipt("capability"),
		PlanID:                  "go-live-plan:sha256:" + digestForReceipt("plan"),
		Nonce:                   "0123456789abcdef0123456789abcdef",
		Discovery:               discovery,
		DiscoveryCanonical:      discoveryCanonical,
		Events:                  events,
		Observation: gotest.Observation{
			Events: events, EventCount: 4, EventBytes: eventBytes, TestCount: 1,
			Packages: []gotest.PackageState{{Name: "example.test/a", Status: "pass", ElapsedNS: 2000, HasElapsed: true, Tests: []gotest.TestState{{Name: "TestAlpha/sub", Status: "pass", ElapsedNS: 1000, HasElapsed: true}}}},
		},
		Runner: gorunner.Result{
			Started: true, Exited: true, ExitCode: 0, ProcessCleanupDone: true,
			Containment: gorunner.ContainmentProcessGroupBestEffort, Duration: 3 * time.Microsecond,
			Stdout: gorunner.StreamResult{Drained: true, RawSHA256: empty},
			Stderr: gorunner.StreamResult{Drained: true, RawSHA256: empty},
		},
		Decoder:         DecoderComplete,
		SourcePreSHA256: digestForReceipt("source"), SourcePostSHA256: digestForReceipt("source"),
		ToolchainPreSHA256: digestForReceipt("toolchain-observation"), ToolchainPostSHA256: digestForReceipt("toolchain-observation"),
		EphemeralDeletionComplete: true,
	}
}

func multiPackageReceiptInput(t *testing.T) ReceiptInput {
	t.Helper()
	input := validReceiptInput(t)
	discovery, discoveryCanonical := multiPackageReceiptDiscovery(t)
	when := time.Date(2026, 8, 23, 16, 0, 0, 0, time.UTC)
	events := []gotest.Event{
		{Sequence: 0, Kind: gotest.TestEvent, Action: "start", Package: "example.test/a", RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
		{Sequence: 1, Kind: gotest.TestEvent, Action: "run", Package: "example.test/a", Test: "TestZulu", RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
		{Sequence: 2, Kind: gotest.TestEvent, Action: "pass", Package: "example.test/a", Test: "TestZulu", ElapsedNS: 1000, HasElapsed: true, RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
		{Sequence: 3, Kind: gotest.TestEvent, Action: "pass", Package: "example.test/a", ElapsedNS: 2000, HasElapsed: true, RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
		{Sequence: 4, Kind: gotest.TestEvent, Action: "start", Package: "example.test/0", RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
		{Sequence: 5, Kind: gotest.TestEvent, Action: "run", Package: "example.test/0", Test: "TestZulu", RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
		{Sequence: 6, Kind: gotest.TestEvent, Action: "pass", Package: "example.test/0", Test: "TestZulu", ElapsedNS: 1000, HasElapsed: true, RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
		{Sequence: 7, Kind: gotest.TestEvent, Action: "run", Package: "example.test/0", Test: "TestAlpha", RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
		{Sequence: 8, Kind: gotest.TestEvent, Action: "skip", Package: "example.test/0", Test: "TestAlpha", ElapsedNS: 500, HasElapsed: true, RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
		{Sequence: 9, Kind: gotest.TestEvent, Action: "pass", Package: "example.test/0", ElapsedNS: 2500, HasElapsed: true, RawTime: "2026-08-23T12:00:00-04:00", TimeUTC: when, HasTime: true},
		{Sequence: 10, Kind: gotest.BuildEvent, Action: "build-fail", ImportPath: "example.test/z-build"},
		{Sequence: 11, Kind: gotest.BuildEvent, Action: "build-fail", ImportPath: "example.test/a-build"},
	}
	var eventBytes uint64
	for _, event := range events {
		body, err := event.CanonicalFactBody()
		if err != nil {
			t.Fatal(err)
		}
		eventBytes += uint64(len(body))
	}
	input.Discovery = discovery
	input.DiscoveryCanonical = discoveryCanonical
	input.Events = events
	input.Observation = gotest.Observation{
		Events: events, EventCount: uint64(len(events)), EventBytes: eventBytes, TestCount: 3,
		Builds: []gotest.BuildState{{ImportPath: "example.test/z-build", Status: "fail"}, {ImportPath: "example.test/a-build", Status: "fail"}},
		Packages: []gotest.PackageState{
			{Name: "example.test/a", Status: "pass", ElapsedNS: 2000, HasElapsed: true, Tests: []gotest.TestState{{Name: "TestZulu", Status: "pass", ElapsedNS: 1000, HasElapsed: true}}},
			{Name: "example.test/0", Status: "pass", ElapsedNS: 2500, HasElapsed: true, Tests: []gotest.TestState{{Name: "TestZulu", Status: "pass", ElapsedNS: 1000, HasElapsed: true}, {Name: "TestAlpha", Status: "skip", ElapsedNS: 500, HasElapsed: true}}},
		},
	}
	return input
}

func receiptDiscovery(t *testing.T) (godiscovery.Document, []byte) {
	t.Helper()
	raw := `{"Dir":"/workspace/a","ImportPath":"example.test/a","Name":"a","Match":["./..."]}` +
		`{"Dir":"/workspace/z","ImportPath":"example.test/z","Name":"z","DepOnly":true}`
	materials := []godiscovery.PackageMaterial{
		{ImportPath: "example.test/a", RawDir: "/workspace/a", DirNamespace: godiscovery.NamespaceSource, DirLogicalPath: "a"},
		{ImportPath: "example.test/z", RawDir: "/workspace/z", DirNamespace: godiscovery.NamespaceSource, DirLogicalPath: "z"},
	}
	return composeReceiptDiscovery(t, raw, materials)
}

func multiPackageReceiptDiscovery(t *testing.T) (godiscovery.Document, []byte) {
	t.Helper()
	raw := `{"Dir":"/workspace/a","ImportPath":"example.test/a","Name":"a","Match":["./..."]}` +
		`{"Dir":"/workspace/0","ImportPath":"example.test/0","Name":"zero","Match":["./..."]}` +
		`{"Dir":"/workspace/z","ImportPath":"example.test/z","Name":"z","DepOnly":true}`
	materials := []godiscovery.PackageMaterial{
		{ImportPath: "example.test/a", RawDir: "/workspace/a", DirNamespace: godiscovery.NamespaceSource, DirLogicalPath: "a"},
		{ImportPath: "example.test/0", RawDir: "/workspace/0", DirNamespace: godiscovery.NamespaceSource, DirLogicalPath: "0"},
		{ImportPath: "example.test/z", RawDir: "/workspace/z", DirNamespace: godiscovery.NamespaceSource, DirLogicalPath: "z"},
	}
	return composeReceiptDiscovery(t, raw, materials)
}

func composeReceiptDiscovery(t *testing.T, raw string, materials []godiscovery.PackageMaterial) (godiscovery.Document, []byte) {
	t.Helper()
	pathDigest := func(logical string) string {
		value, err := godiscovery.Hash("go-logical-path", "go-logical-path/0", []byte(`{"namespace":"SOURCE","path":"`+logical+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	for index := range materials {
		materials[index].DirPathSHA256 = pathDigest(materials[index].DirLogicalPath)
	}
	manifest, err := godiscovery.Decode(strings.NewReader(raw), godiscovery.Commitments{
		DependencyMaterializationSHA256: digestForReceipt("dependency-materialization"),
		ModuleMode:                      godiscovery.ModuleModeModule,
		Packages:                        materials,
		SourceWSI:                       "workspace-source:sha256:" + digestForReceipt("source"),
	})
	if err != nil {
		t.Fatal(err)
	}
	empty := sha256.Sum256(nil)
	document, canonical, err := godiscovery.Compose(manifest, godiscovery.DocumentInput{
		EnvironmentSHA256: digestForReceipt("environment"), PackagePatterns: []string{"./..."},
		RawStderrSHA256: hex.EncodeToString(empty[:]), ToolchainID: "go-toolchain:sha256:" + digestForReceipt("toolchain"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := godiscovery.VerifyDocument(document, canonical); err != nil {
		t.Fatal(err)
	}
	return document, canonical
}

func cloneReceiptInput(input ReceiptInput) ReceiptInput {
	input.Events = append([]gotest.Event(nil), input.Events...)
	input.DiscoveryCanonical = append([]byte(nil), input.DiscoveryCanonical...)
	input.Discovery.Packages = append([]godiscovery.Package(nil), input.Discovery.Packages...)
	input.Discovery.RequestedRunnerPackages = append([]string(nil), input.Discovery.RequestedRunnerPackages...)
	input.Observation.Events = append([]gotest.Event(nil), input.Observation.Events...)
	input.Observation.IncompleteReasons = append([]string(nil), input.Observation.IncompleteReasons...)
	input.Runner.Coverage.Packages = append([]string(nil), input.Runner.Coverage.Packages...)
	return input
}

func cloneReceipt(receipt Receipt) Receipt {
	result := receipt
	result.Run = append([]byte(nil), receipt.Run...)
	result.Transcript = append([]byte(nil), receipt.Transcript...)
	result.Events = make([][]byte, len(receipt.Events))
	for index := range receipt.Events {
		result.Events[index] = append([]byte(nil), receipt.Events[index]...)
	}
	return result
}

func digestForReceipt(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
