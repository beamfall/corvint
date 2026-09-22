package provider

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json/jsontext"
	"errors"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/godiscovery"
	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
	"github.com/Beamfall/corvint/internal/liveverify/gotest"
)

const (
	EventProfile = "go-live-event/0"
	RunProfile   = "go-live-run/0"
)

var ErrReceiptInvalid = errors.New("go-live-receipt: invalid input")

type DecoderStatus string

const (
	DecoderComplete  DecoderStatus = "COMPLETE"
	DecoderRejected  DecoderStatus = "REJECTED"
	DecoderTruncated DecoderStatus = "TRUNCATED"
)

// ReceiptInput contains only facts available after execution, post-identity
// observation, and provider-owned state deletion. It contains no raw output or
// environment values. Empty post identities mean that axis is UNKNOWN.
type ReceiptInput struct {
	ActualEnvironmentSHA256   string
	CapabilityID              string
	PlanID                    string
	Nonce                     string
	Discovery                 godiscovery.Document
	DiscoveryCanonical        []byte
	Events                    []gotest.Event
	Observation               gotest.Observation
	Runner                    gorunner.Result
	Decoder                   DecoderStatus
	SourcePreSHA256           string
	SourcePostSHA256          string
	ToolchainPreSHA256        string
	ToolchainPostSHA256       string
	EphemeralDeletionComplete bool
	ProviderFailure           bool
}

// Receipt is a caller-owned, non-persistent canonical transcript. Event and
// Run members include their terminal LF; Transcript is their exact concatenation.
type Receipt struct {
	RunID      string
	WEI        string
	Events     [][]byte
	Run        []byte
	Transcript []byte
}

type receiptObservation struct {
	buildFailed     map[string]bool
	executedTests   map[string]bool
	observedTests   map[string]bool
	skippedTests    map[string]bool
	testTerminals   map[string]string
	packageTerminal map[string]string
	benchmark       bool
	buildMismatch   bool
}

// ComposeReceipt binds normalized Go facts to the final WEI, emits consecutive
// evidence events, and closes them with exactly one terminal run document.
func ComposeReceipt(input ReceiptInput) (Receipt, error) {
	observation, err := validateReceiptInput(input)
	if err != nil {
		return Receipt{}, ErrReceiptInvalid
	}
	runID := prefixedID("go-live-attempt", "go-live-attempt/0", appendAttemptBody(nil, input.Nonce, input.PlanID))
	testIDs := sortedKeys(observation.observedTests)
	discoveredTestsBody := appendDiscoveredTestsBody(nil, input.Discovery.RequestedRunnerPackages, testIDs)
	discoveredTestsSHA256 := bareID("go-discovered-tests", "go-discovered-tests/0", discoveredTestsBody)
	weiBody := appendWEIBody(nil, input, discoveredTestsSHA256)
	wei := prefixedID("workspace-execution", "go-live-wei/0", weiBody)

	receipt := Receipt{RunID: runID, WEI: wei, Events: make([][]byte, len(input.Events))}
	eventDigests := make([]string, len(input.Events))
	for index, event := range input.Events {
		fact, factErr := event.CanonicalFactBody()
		if factErr != nil {
			return Receipt{}, ErrReceiptInvalid
		}
		body := appendEventBody(nil, fact, runID, uint64(index), wei, "")
		id := prefixedID("go-live-event", EventProfile, body)
		encoded := appendEventBody(nil, fact, runID, uint64(index), wei, id)
		encoded = append(encoded, '\n')
		receipt.Events[index] = encoded
		eventDigests[index] = strings.TrimPrefix(id, "go-live-event:sha256:")
		receipt.Transcript = append(receipt.Transcript, encoded...)
	}
	eventRoot := bareID("go-event-root", "go-event-root/0", appendEventRootBody(nil, eventDigests))
	runWithoutID := appendRun(nil, input, observation, runID, wei, eventRoot, "")
	runIDDocument := prefixedID("go-live-run", RunProfile, runWithoutID)
	receipt.Run = appendRun(nil, input, observation, runID, wei, eventRoot, runIDDocument)
	receipt.Run = append(receipt.Run, '\n')
	receipt.Transcript = append(receipt.Transcript, receipt.Run...)
	return receipt, nil
}

// VerifyReceipt recomputes every identity and byte from the independently
// supplied post-run facts. It rejects gaps, duplicate terminals, extra bytes,
// stale attachment, and any non-canonical mutation.
func VerifyReceipt(input ReceiptInput, receipt Receipt) error {
	want, err := ComposeReceipt(input)
	if err != nil || receipt.RunID != want.RunID || receipt.WEI != want.WEI ||
		!bytes.Equal(receipt.Run, want.Run) || !bytes.Equal(receipt.Transcript, want.Transcript) ||
		len(receipt.Events) != len(want.Events) {
		return ErrReceiptInvalid
	}
	for index := range want.Events {
		if !bytes.Equal(receipt.Events[index], want.Events[index]) {
			return ErrReceiptInvalid
		}
	}
	return nil
}

func validateReceiptInput(input ReceiptInput) (receiptObservation, error) {
	invalid := func() (receiptObservation, error) { return receiptObservation{}, ErrReceiptInvalid }
	if godiscovery.VerifyDocument(input.Discovery, input.DiscoveryCanonical) != nil {
		return invalid()
	}
	if !validDigest(input.ActualEnvironmentSHA256) || !validID(input.CapabilityID, "go-live-capability") ||
		!validID(input.PlanID, "go-live-plan") || !validID(input.Discovery.ID, "go-live-discovery") ||
		!validID(input.Discovery.ToolchainID, "go-toolchain") || len(input.Nonce) != 32 ||
		input.Nonce != strings.ToLower(input.Nonce) || !validLowerHex(input.Nonce) ||
		!validDigest(input.SourcePreSHA256) || !validDigest(input.ToolchainPreSHA256) ||
		(input.SourcePostSHA256 != "" && !validDigest(input.SourcePostSHA256)) ||
		(input.ToolchainPostSHA256 != "" && !validDigest(input.ToolchainPostSHA256)) ||
		(input.Decoder != DecoderComplete && input.Decoder != DecoderRejected && input.Decoder != DecoderTruncated) ||
		!input.Runner.Started || len(input.Discovery.RequestedRunnerPackages) == 0 ||
		!sortedUnique(input.Discovery.RequestedRunnerPackages) || len(input.Discovery.Packages) == 0 ||
		len(input.Discovery.Packages) > godiscovery.MaxPackages || len(input.Events) > gotest.MaxEvents ||
		input.Observation.EventCount != uint64(len(input.Events)) || len(input.Observation.Events) != len(input.Events) || len(input.Discovery.Argv) < 6 ||
		input.ActualEnvironmentSHA256 != input.Discovery.EnvironmentSHA256 ||
		input.Discovery.SourceWSI != "workspace-source:sha256:"+input.SourcePreSHA256 ||
		!validPackagePatterns(input.Discovery.Argv[5:]) || !matchesDiscoveryPatterns(input.Discovery) ||
		!validOptionalReasons(input.Observation.IncompleteReasons) || !validCoverageObservation(input.Runner.Coverage) {
		return invalid()
	}
	if input.Discovery.Argv[0] != "@PINNED_GO@" || input.Discovery.Argv[1] != "list" ||
		input.Discovery.Argv[2] != "-deps" || input.Discovery.Argv[3] != "-test" ||
		input.Discovery.Argv[4] != "-json="+godiscovery.ClosedFields {
		return invalid()
	}
	if input.Runner.Duration < 0 || input.Runner.Stdout.Bytes > uint64(gorunner.MaxOutputBytes) ||
		input.Runner.Stderr.Bytes > uint64(gorunner.MaxOutputBytes) ||
		input.Runner.Stdout.Bytes > ^uint64(0)-input.Runner.Stderr.Bytes ||
		input.Runner.Stdout.Bytes+input.Runner.Stderr.Bytes > uint64(gorunner.MaxOutputBytes) ||
		!validDigest(input.Runner.Stdout.RawSHA256) || !validDigest(input.Runner.Stderr.RawSHA256) ||
		input.Runner.Containment != gorunner.ContainmentProcessGroupBestEffort ||
		input.Runner.Duration > gorunner.MaxRunTime+gorunner.MaxPipeWait ||
		(input.Runner.Exited && (input.Runner.ExitCode < 0 || uint64(input.Runner.ExitCode) > uint64(^uint32(0)))) ||
		(!input.Runner.Exited && input.Runner.ExitCode != -1) {
		return invalid()
	}
	discovered := make(map[string]bool, len(input.Discovery.Packages))
	listed := make(map[string]bool, len(input.Discovery.Packages))
	for _, pack := range input.Discovery.Packages {
		if pack.ImportPath == "" || !utf8.ValidString(pack.ImportPath) || !validID(pack.ID, "go-package") || discovered[pack.ImportPath] || listed[pack.ID] {
			return invalid()
		}
		discovered[pack.ImportPath], listed[pack.ID] = true, true
	}
	for _, requested := range input.Discovery.RequestedRunnerPackages {
		if !discovered[requested] {
			return invalid()
		}
	}
	result := receiptObservation{
		buildFailed: make(map[string]bool), executedTests: make(map[string]bool), observedTests: make(map[string]bool),
		skippedTests: make(map[string]bool), testTerminals: make(map[string]string), packageTerminal: make(map[string]string),
	}
	var eventBytes, outputBytes uint64
	packageStates := make(map[string]string)
	testStates := make(map[string]string)
	testPackages := make(map[string]string)
	buildStates := make(map[string]string)
	failedBuildRefs := make(map[string]uint64)
	for index, event := range input.Events {
		if event.Sequence != uint64(index) {
			return invalid()
		}
		fact, err := event.CanonicalFactBody()
		observedFact, observedErr := input.Observation.Events[index].CanonicalFactBody()
		if err != nil || uint64(len(fact)) > ^uint64(0)-eventBytes {
			return invalid()
		}
		if observedErr != nil || input.Observation.Events[index].Sequence != uint64(index) || !bytes.Equal(fact, observedFact) ||
			!advanceReceiptState(event, packageStates, testStates, testPackages, buildStates) {
			return invalid()
		}
		eventBytes += uint64(len(fact))
		if eventBytes > uint64(gotest.MaxEventBytes) {
			return invalid()
		}
		if event.OutputBytes > ^uint64(0)-outputBytes {
			return invalid()
		}
		outputBytes += event.OutputBytes
		if event.Kind == gotest.TestEvent {
			if !discovered[event.Package] {
				return invalid()
			}
			if event.Test != "" {
				testID := testID(event.Package, event.Test)
				result.observedTests[testID] = true
				switch event.Action {
				case "run":
					result.executedTests[testID] = true
				case "pass":
					if result.testTerminals[testID] != "" {
						return invalid()
					}
					result.testTerminals[testID] = "PASSED"
				case "fail":
					if result.testTerminals[testID] != "" {
						return invalid()
					}
					result.testTerminals[testID] = "FAILED"
				case "skip":
					if result.testTerminals[testID] != "" {
						return invalid()
					}
					result.testTerminals[testID], result.skippedTests[testID] = "SKIPPED", true
				case "bench":
					if result.testTerminals[testID] != "" {
						return invalid()
					}
					result.testTerminals[testID], result.benchmark = "BENCH", true
				}
			} else {
				if event.FailedBuild != "" {
					failedBuildRefs[event.FailedBuild]++
				}
				switch event.Action {
				case "pass":
					if result.packageTerminal[event.Package] != "" {
						return invalid()
					}
					result.packageTerminal[event.Package] = "PASSED"
				case "fail":
					if result.packageTerminal[event.Package] != "" {
						return invalid()
					}
					result.packageTerminal[event.Package] = "FAILED"
				case "skip":
					if result.packageTerminal[event.Package] != "" {
						return invalid()
					}
					result.packageTerminal[event.Package] = "SKIPPED"
				}
			}
		} else if event.Kind == gotest.BuildEvent && event.Action == "build-fail" {
			if result.buildFailed[event.ImportPath] {
				return invalid()
			}
			result.buildFailed[event.ImportPath] = true
		}
	}
	for path := range failedBuildRefs {
		if !result.buildFailed[path] {
			return invalid()
		}
	}
	for path := range result.buildFailed {
		if failedBuildRefs[path] == 0 {
			result.buildMismatch = true
		}
	}
	if eventBytes != input.Observation.EventBytes || outputBytes != input.Observation.OutputBytes ||
		uint64(len(result.observedTests)) != input.Observation.TestCount || !sameObservation(input.Observation, result) {
		return invalid()
	}
	if input.Decoder == DecoderComplete {
		for _, requested := range input.Discovery.RequestedRunnerPackages {
			if result.packageTerminal[requested] == "" {
				return invalid()
			}
		}
		for test := range result.executedTests {
			if result.testTerminals[test] == "" {
				return invalid()
			}
		}
		for _, state := range testStates {
			if !receiptTerminal(state) {
				return invalid()
			}
		}
	}
	return result, nil
}

func advanceReceiptState(event gotest.Event, packages, tests, testPackages, builds map[string]string) bool {
	if event.Kind == gotest.BuildEvent {
		state := builds[event.ImportPath]
		switch event.Action {
		case "build-output":
			if state == "fail" {
				return false
			}
			builds[event.ImportPath] = "output"
			return true
		case "build-fail":
			if state == "fail" {
				return false
			}
			builds[event.ImportPath] = "fail"
			return true
		default:
			return false
		}
	}
	if event.Kind != gotest.TestEvent {
		return false
	}
	if event.Action == "start" {
		if event.Test != "" || packages[event.Package] != "" {
			return false
		}
		packages[event.Package] = "running"
		return true
	}
	if packages[event.Package] != "running" {
		return false
	}
	if event.Test == "" {
		switch event.Action {
		case "output":
			return true
		case "pass", "fail", "skip":
			for test, state := range tests {
				if testPackages[test] == event.Package && !receiptTerminal(state) {
					return false
				}
			}
			packages[event.Package] = event.Action
			return true
		default:
			return false
		}
	}
	id := testID(event.Package, event.Test)
	state := tests[id]
	switch event.Action {
	case "run":
		if state != "" {
			return false
		}
		tests[id], testPackages[id] = "running", event.Package
	case "output":
		if state != "running" && state != "paused" {
			return false
		}
	case "attr", "artifacts":
		if state != "running" && state != "paused" {
			return false
		}
	case "pause":
		if state != "running" {
			return false
		}
		tests[id] = "paused"
	case "cont":
		if state != "paused" {
			return false
		}
		tests[id] = "running"
	case "pass", "skip":
		if state != "running" && state != "paused" {
			return false
		}
		tests[id] = event.Action
	case "fail", "bench":
		if state != "running" && state != "paused" {
			return false
		}
		tests[id] = event.Action
	default:
		return false
	}
	return true
}

func receiptTerminal(state string) bool {
	return state == "pass" || state == "fail" || state == "skip" || state == "bench"
}

func sameObservation(observation gotest.Observation, derived receiptObservation) bool {
	packages := make(map[string]gotest.PackageState, len(observation.Packages))
	for _, pack := range observation.Packages {
		if _, exists := packages[pack.Name]; exists {
			return false
		}
		packages[pack.Name] = pack
		if status := terminalStatus(pack.Status); status != "" && derived.packageTerminal[pack.Name] != status {
			return false
		}
		for _, test := range pack.Tests {
			id := testID(pack.Name, test.Name)
			if status := terminalStatus(test.Status); status != "" && derived.testTerminals[id] != status {
				return false
			}
		}
	}
	for name, status := range derived.packageTerminal {
		pack, ok := packages[name]
		if !ok || terminalStatus(pack.Status) != status {
			return false
		}
	}
	builds := make(map[string]string, len(observation.Builds))
	for _, build := range observation.Builds {
		if _, exists := builds[build.ImportPath]; exists {
			return false
		}
		builds[build.ImportPath] = build.Status
	}
	for path := range derived.buildFailed {
		if builds[path] != "fail" {
			return false
		}
	}
	for path, status := range builds {
		if status == "fail" && !derived.buildFailed[path] {
			return false
		}
	}
	return true
}

func terminalStatus(status string) string {
	switch status {
	case "pass":
		return "PASSED"
	case "fail":
		return "FAILED"
	case "skip":
		return "SKIPPED"
	case "bench":
		return "BENCH"
	default:
		return ""
	}
}

func appendAttemptBody(dst []byte, nonce, planID string) []byte {
	dst = append(dst, `{"nonce":`...)
	dst = appendQuote(dst, nonce)
	dst = append(dst, `,"planId":`...)
	dst = appendQuote(dst, planID)
	return append(dst, '}')
}

func appendDiscoveredTestsBody(dst []byte, packages, tests []string) []byte {
	dst = append(dst, `{"completeness":"INCOMPLETE","packages":`...)
	dst = appendStringArray(dst, packages)
	dst = append(dst, `,"tests":`...)
	dst = appendStringArray(dst, tests)
	return append(dst, '}')
}

func appendWEIBody(dst []byte, input ReceiptInput, discovered string) []byte {
	dst = append(dst, `{"actualEnvironmentSha256":`...)
	dst = appendQuote(dst, input.ActualEnvironmentSHA256)
	dst = append(dst, `,"capabilityId":`...)
	dst = appendQuote(dst, input.CapabilityID)
	dst = append(dst, `,"discoveredTestSetSha256":`...)
	dst = appendQuote(dst, discovered)
	dst = append(dst, `,"discoveryId":`...)
	dst = appendQuote(dst, input.Discovery.ID)
	dst = append(dst, `,"planId":`...)
	dst = appendQuote(dst, input.PlanID)
	dst = append(dst, `,"toolchainId":`...)
	dst = appendQuote(dst, input.Discovery.ToolchainID)
	return append(dst, '}')
}

func appendEventBody(dst, fact []byte, runID string, sequence uint64, wei, id string) []byte {
	dst = append(dst, `{"fact":`...)
	dst = append(dst, fact...)
	if id != "" {
		dst = append(dst, `,"id":`...)
		dst = appendQuote(dst, id)
	}
	dst = append(dst, `,"profile":"go-live-event/0","runId":`...)
	dst = appendQuote(dst, runID)
	dst = append(dst, `,"sequence":`...)
	dst = appendQuote(dst, strconv.FormatUint(sequence, 10))
	dst = append(dst, `,"wei":`...)
	dst = appendQuote(dst, wei)
	return append(dst, '}')
}

func appendEventRootBody(dst []byte, digests []string) []byte {
	dst = append(dst, `{"events":[`...)
	for index, digest := range digests {
		if index != 0 {
			dst = append(dst, ',')
		}
		dst = append(dst, `{"digest":`...)
		dst = appendQuote(dst, digest)
		dst = append(dst, `,"sequence":`...)
		dst = appendQuote(dst, strconv.Itoa(index))
		dst = append(dst, '}')
	}
	return append(dst, ']', '}')
}

func appendRun(dst []byte, input ReceiptInput, observation receiptObservation, runID, wei, eventRoot, id string) []byte {
	status, failureClass, limit := executionOutcome(input, observation)
	dst = append(dst, `{"coverage":`...)
	dst = appendCoverage(dst, effectiveCoverage(input))
	dst = append(dst, `,"ephemeralDeletion":`...)
	if input.EphemeralDeletionComplete {
		dst = appendQuote(dst, "COMPLETE")
	} else {
		dst = appendQuote(dst, "INCOMPLETE")
	}
	dst = append(dst, `,"eventCount":`...)
	dst = appendQuote(dst, strconv.Itoa(len(input.Events)))
	dst = append(dst, `,"eventRootSha256":`...)
	dst = appendQuote(dst, eventRoot)
	dst = append(dst, `,"execution":{"cancelled":`...)
	// GLTP-V0-039: an elapsed run deadline outranks cancellation, and the
	// independent verifier rejects a receipt carrying both bits true.
	dst = strconv.AppendBool(dst, input.Runner.Cancelled && !input.Runner.TimedOut)
	dst = append(dst, `,"containment":{"mechanism":`...)
	dst = appendQuote(dst, string(input.Runner.Containment))
	dst = append(dst, `,"qualified":false}`...)
	dst = append(dst, `,"durationNanoseconds":`...)
	dst = appendQuote(dst, strconv.FormatInt(input.Runner.Duration.Nanoseconds(), 10))
	dst = append(dst, `,"exitCode":`...)
	if input.Runner.Exited && input.Runner.ExitCode >= 0 {
		dst = appendQuote(dst, strconv.Itoa(input.Runner.ExitCode))
	} else {
		dst = append(dst, "null"...)
	}
	dst = append(dst, `,"failureClass":`...)
	dst = appendNullable(dst, failureClass)
	dst = append(dst, `,"limit":`...)
	dst = appendNullable(dst, limit)
	dst = append(dst, `,"resources":{"cpuMilliseconds":null,"memoryPeakBytes":null,"openFilesPeak":null,"processesPeak":null},"signal":null,"status":`...)
	dst = appendQuote(dst, status)
	dst = append(dst, `,"timedOut":`...)
	dst = strconv.AppendBool(dst, input.Runner.TimedOut)
	dst = append(dst, '}')
	if id != "" {
		dst = append(dst, `,"id":`...)
		dst = appendQuote(dst, id)
	}
	dst = append(dst, `,"io":{"decoder":`...)
	dst = appendQuote(dst, string(input.Decoder))
	dst = append(dst, `,"stderrBytes":`...)
	dst = appendQuote(dst, strconv.FormatUint(input.Runner.Stderr.Bytes, 10))
	dst = append(dst, `,"stderrDrained":`...)
	dst = strconv.AppendBool(dst, input.Runner.Stderr.Drained)
	dst = append(dst, `,"stderrRawSha256":`...)
	dst = appendQuote(dst, input.Runner.Stderr.RawSHA256)
	dst = append(dst, `,"stdoutBytes":`...)
	dst = appendQuote(dst, strconv.FormatUint(input.Runner.Stdout.Bytes, 10))
	dst = append(dst, `,"stdoutDrained":`...)
	dst = strconv.AppendBool(dst, input.Runner.Stdout.Drained)
	dst = append(dst, `,"stdoutRawSha256":`...)
	dst = appendQuote(dst, input.Runner.Stdout.RawSHA256)
	dst = append(dst, '}')
	dst = append(dst, `,"persistence":"NONE","planId":`...)
	dst = appendQuote(dst, input.PlanID)
	dst = append(dst, `,"profile":"go-live-run/0","qualification":"EXPERIMENTAL_TRANSCRIPT","runId":`...)
	dst = appendQuote(dst, runID)
	dst = append(dst, `,"scope":`...)
	dst = appendScope(dst, input, observation)
	sourceCurrency := currency(input.SourcePreSHA256, input.SourcePostSHA256)
	toolchainCurrency := currency(input.ToolchainPreSHA256, input.ToolchainPostSHA256)
	dst = append(dst, `,"sourceCurrency":`...)
	dst = appendQuote(dst, sourceCurrency)
	dst = append(dst, `,"sourcePostSha256":`...)
	dst = appendNullable(dst, input.SourcePostSHA256)
	dst = append(dst, `,"sourcePreSha256":`...)
	dst = appendQuote(dst, input.SourcePreSHA256)
	dst = append(dst, `,"toolchainCurrency":`...)
	dst = appendQuote(dst, toolchainCurrency)
	dst = append(dst, `,"toolchainPostSha256":`...)
	dst = appendNullable(dst, input.ToolchainPostSHA256)
	dst = append(dst, `,"toolchainPreSha256":`...)
	dst = appendQuote(dst, input.ToolchainPreSHA256)
	dst = append(dst, `,"wei":`...)
	dst = appendQuote(dst, wei)
	return append(dst, '}')
}

func appendCoverage(dst []byte, coverage gorunner.CoverageObservation) []byte {
	dst = append(dst, `{"artifactSha256":`...)
	dst = appendNullable(dst, coverage.ArtifactSHA256)
	dst = append(dst, `,"completeness":`...)
	dst = appendQuote(dst, string(coverage.Completeness))
	dst = append(dst, `,"mode":`...)
	dst = appendQuote(dst, coverage.Mode)
	dst = append(dst, `,"packages":`...)
	dst = appendStringArray(dst, coverage.Packages)
	dst = append(dst, `,"rawRootSha256":`...)
	dst = appendNullable(dst, coverage.RawRootSHA256)
	return append(dst, '}')
}

func effectiveCoverage(input ReceiptInput) gorunner.CoverageObservation {
	coverage := input.Runner.Coverage
	if coverage.Completeness == "" {
		return gorunner.CoverageObservation{
			Completeness: gorunner.CoverageAbsent,
			Mode:         "NONE",
			Packages:     []string{},
		}
	}
	if coverage.Completeness != gorunner.CoverageComplete {
		return coverage
	}
	if input.Decoder == DecoderComplete && input.Runner.Started && input.Runner.Exited && input.Runner.ExitCode == 0 &&
		!input.Runner.Cancelled && !input.Runner.TimedOut && !input.Runner.OutputLimitExceeded && !input.Runner.PipeWaitExpired &&
		input.Runner.ProcessCleanupDone && input.Runner.Stdout.Drained && input.Runner.Stderr.Drained &&
		input.SourcePostSHA256 == input.SourcePreSHA256 && input.ToolchainPostSHA256 == input.ToolchainPreSHA256 && !input.ProviderFailure {
		return coverage
	}
	coverage.Completeness = gorunner.CoveragePartial
	return coverage
}

func validCoverageObservation(coverage gorunner.CoverageObservation) bool {
	if coverage.Completeness == "" {
		return coverage.ArtifactSHA256 == "" && coverage.Mode == "" && len(coverage.Packages) == 0 && coverage.RawRootSHA256 == ""
	}
	if coverage.Completeness == gorunner.CoverageAbsent {
		return coverage.ArtifactSHA256 == "" && coverage.Mode == "NONE" && len(coverage.Packages) == 0 && coverage.RawRootSHA256 == ""
	}
	if coverage.Mode != "UNIT_COVERPROFILE_SET" && coverage.Mode != "UNIT_COVERPROFILE_COUNT" && coverage.Mode != "UNIT_COVERPROFILE_ATOMIC" {
		return false
	}
	if coverage.Completeness == gorunner.CoverageRejected {
		return coverage.ArtifactSHA256 == "" && len(coverage.Packages) == 0 && coverage.RawRootSHA256 == ""
	}
	if coverage.Completeness != gorunner.CoverageComplete && coverage.Completeness != gorunner.CoveragePartial {
		return false
	}
	return validDigest(coverage.ArtifactSHA256) && validDigest(coverage.RawRootSHA256) && sortedUniqueOrEmpty(coverage.Packages)
}

func sortedUniqueOrEmpty(values []string) bool {
	previous := ""
	for _, value := range values {
		if value == "" || value <= previous || !utf8.ValidString(value) {
			return false
		}
		previous = value
	}
	return true
}

var allUnknownReasons = []string{
	"BUILD_CONSTRAINT_VARIANTS", "CROSS_PLATFORM_VARIANTS", "DISCOVERY_INCOMPLETE", "EXTERNAL_MODULE_FRONTIER",
	"FUZZ_BENCHMARK_FRONTIER", "MANDATORY_GATE_OUTSIDE_RUN", "NESTED_MODULE_FRONTIER", "NETWORK_STATE_UNKNOWN",
	"NON_GO_TEST_FRONTIER", "NO_AFFECTED_SELECTION_PROOF", "PACKAGE_PATTERN_SEMANTICS", "PARENT_TEST_UNAVAILABLE",
	"SOURCE_ANCHOR_UNAVAILABLE", "UNOBSERVED_DYNAMIC_SUBTESTS",
}

func appendScope(dst []byte, input ReceiptInput, observation receiptObservation) []byte {
	builds := make([]string, 0, len(observation.buildFailed))
	for value := range observation.buildFailed {
		builds = append(builds, value)
	}
	sort.Strings(builds)
	listed := make([]string, 0, len(input.Discovery.Packages))
	for _, pack := range input.Discovery.Packages {
		listed = append(listed, pack.ID)
	}
	sort.Strings(listed)
	patterns := append([]string(nil), input.Discovery.Argv[5:]...)
	sort.Strings(patterns)
	dst = append(dst, `{"buildTerminals":[`...)
	for index, value := range builds {
		if index != 0 {
			dst = append(dst, ',')
		}
		dst = append(dst, `{"importPath":`...)
		dst = appendQuote(dst, value)
		dst = append(dst, `,"status":"FAILED"}`...)
	}
	dst = append(dst, `],"conclusion":"UNKNOWN","excluded":[],"executedTests":`...)
	dst = appendStringArray(dst, sortedKeys(observation.executedTests))
	dst = append(dst, `,"listedPackages":`...)
	dst = appendStringArray(dst, listed)
	dst = append(dst, `,"observedTests":`...)
	dst = appendStringArray(dst, sortedKeys(observation.observedTests))
	dst = append(dst, `,"packageTerminals":[`...)
	packageBodies := make([][]byte, 0, len(input.Discovery.RequestedRunnerPackages))
	for _, pack := range input.Discovery.RequestedRunnerPackages {
		if status := observation.packageTerminal[pack]; status != "" {
			body := []byte(`{"package":`)
			body = appendQuote(body, pack)
			body = append(body, `,"status":`...)
			body = appendQuote(body, status)
			body = append(body, '}')
			packageBodies = append(packageBodies, body)
		}
	}
	sortBytes(packageBodies)
	dst = appendBodies(dst, packageBodies)
	dst = append(dst, ']')
	dst = append(dst, `,"requestedPackagePatterns":`...)
	dst = appendStringArray(dst, patterns)
	dst = append(dst, `,"skippedTests":`...)
	dst = appendStringArray(dst, sortedKeys(observation.skippedTests))
	dst = append(dst, `,"testTerminals":[`...)
	testBodies := make([][]byte, 0, len(observation.testTerminals))
	for test, status := range observation.testTerminals {
		body := []byte(`{"status":`)
		body = appendQuote(body, status)
		body = append(body, `,"test":`...)
		body = appendQuote(body, test)
		body = append(body, '}')
		testBodies = append(testBodies, body)
	}
	sortBytes(testBodies)
	dst = appendBodies(dst, testBodies)
	dst = append(dst, ']')
	dst = append(dst, `,"unknownReasons":`...)
	dst = appendStringArray(dst, allUnknownReasons)
	return append(dst, '}')
}

func executionOutcome(input ReceiptInput, observation receiptObservation) (string, string, string) {
	current := currency(input.SourcePreSHA256, input.SourcePostSHA256) == "CURRENT" && currency(input.ToolchainPreSHA256, input.ToolchainPostSHA256) == "CURRENT"
	limit := ""
	if input.Runner.OutputLimitExceeded {
		limit = "OUTPUT_BYTES"
	}
	if input.Runner.TimedOut {
		limit = "RUN_TIME"
	}
	complete := input.Decoder == DecoderComplete && len(input.Observation.IncompleteReasons) == 0 && !observation.benchmark && !observation.buildMismatch && !input.ProviderFailure && input.EphemeralDeletionComplete && current && input.Runner.Exited && input.Runner.ProcessCleanupDone && input.Runner.Stdout.Drained && input.Runner.Stderr.Drained && !input.Runner.OutputLimitExceeded && !input.Runner.PipeWaitExpired && !input.Runner.Cancelled && !input.Runner.TimedOut
	hasFailure := len(observation.buildFailed) != 0
	allRequestedPassed := true
	for _, requested := range input.Discovery.RequestedRunnerPackages {
		status := observation.packageTerminal[requested]
		if status != "PASSED" {
			allRequestedPassed = false
		}
		if status == "FAILED" {
			hasFailure = true
		}
	}
	for _, status := range observation.testTerminals {
		if status == "FAILED" {
			hasFailure = true
		}
	}
	if complete && input.Runner.ExitCode == 0 && !hasFailure && allRequestedPassed {
		return "PASSED", "", limit
	}
	if complete && input.Runner.ExitCode != 0 && hasFailure {
		if len(observation.buildFailed) != 0 {
			return "FAILED", "BUILD", limit
		}
		return "FAILED", "ASSERTION_OR_TEST", limit
	}
	if !current {
		return "INCOMPLETE", "STALE", limit
	}
	if input.Runner.TimedOut {
		return "INCOMPLETE", "TIMEOUT", limit
	}
	if input.ProviderFailure {
		return "INCOMPLETE", "INFRASTRUCTURE", limit
	}
	if input.Runner.Cancelled {
		return "INCOMPLETE", "CANCELLATION", ""
	}
	if input.Runner.OutputLimitExceeded {
		return "INCOMPLETE", "INFRASTRUCTURE", limit
	}
	return "INCOMPLETE", "INFRASTRUCTURE", ""
}

func currency(pre, post string) string {
	if post == "" {
		return "UNKNOWN"
	}
	if pre == post {
		return "CURRENT"
	}
	return "STALE"
}

func testID(packageName, name string) string {
	body := []byte(`{"name":`)
	body = appendQuote(body, name)
	body = append(body, `,"package":`...)
	body = appendQuote(body, packageName)
	body = append(body, '}')
	return prefixedID("go-test", "go-test/0", body)
}
func bareID(kind, profile string, body []byte) string {
	sum := framedDigest(kind, profile, body)
	return hex.EncodeToString(sum[:])
}
func prefixedID(kind, profile string, body []byte) string {
	return kind + ":sha256:" + bareID(kind, profile, body)
}
func framedDigest(kind, profile string, body []byte) [32]byte {
	h := sha256.New()
	var b4 [4]byte
	var b8 [8]byte
	binary.BigEndian.PutUint32(b4[:], uint32(len(kind)))
	h.Write(b4[:])
	h.Write([]byte(kind))
	binary.BigEndian.PutUint32(b4[:], uint32(len(profile)))
	h.Write(b4[:])
	h.Write([]byte(profile))
	binary.BigEndian.PutUint64(b8[:], uint64(len(body)))
	h.Write(b8[:])
	h.Write(body)
	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}
func appendQuote(dst []byte, value string) []byte {
	result, err := jsontext.AppendQuote(dst, value)
	if err != nil {
		panic("invalid UTF-8 passed receipt validation")
	}
	return result
}
func appendNullable(dst []byte, value string) []byte {
	if value == "" {
		return append(dst, "null"...)
	}
	return appendQuote(dst, value)
}
func appendStringArray(dst []byte, values []string) []byte {
	dst = append(dst, '[')
	for index, value := range values {
		if index != 0 {
			dst = append(dst, ',')
		}
		dst = appendQuote(dst, value)
	}
	return append(dst, ']')
}
func appendBodies(dst []byte, values [][]byte) []byte {
	for index, value := range values {
		if index != 0 {
			dst = append(dst, ',')
		}
		dst = append(dst, value...)
	}
	return dst
}
func sortBytes(values [][]byte) {
	sort.Slice(values, func(i, j int) bool { return bytes.Compare(values[i], values[j]) < 0 })
}
func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
func sortedUnique(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for index, value := range values {
		if value == "" || !utf8.ValidString(value) || index != 0 && values[index-1] >= value {
			return false
		}
	}
	return true
}
func validOptionalReasons(values []string) bool {
	for index, value := range values {
		if value == "" || !utf8.ValidString(value) || index != 0 && values[index-1] >= value {
			return false
		}
	}
	return true
}
func matchesDiscoveryPatterns(document godiscovery.Document) bool {
	set := make(map[string]bool)
	for _, pack := range document.Packages {
		for _, match := range pack.Match {
			set[match] = true
		}
	}
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return strings.Join(values, "\x00") == strings.Join(document.Argv[5:], "\x00")
}
func validDigest(value string) bool { return len(value) == 64 && validLowerHex(value) }
func validLowerHex(value string) bool {
	for _, c := range value {
		if c < '0' || c > '9' {
			if c < 'a' || c > 'f' {
				return false
			}
		}
	}
	return true
}
func validID(value, kind string) bool {
	return strings.HasPrefix(value, kind+":sha256:") && validDigest(strings.TrimPrefix(value, kind+":sha256:"))
}
