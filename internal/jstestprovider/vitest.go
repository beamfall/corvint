package jstestprovider

import (
	"encoding/json"
	"regexp"
	"strconv"
)

// Vitest's JSON reporter (Jest-`--json`-compatible; see
// evidence/ipr-08-runner-selection.md section 3). As of vitest@5.0.0 it does
// not emit assertionResults[].location (confirmed empirically - see
// testdata/vitest-mixed.json and vitest_test.go); this adapter recovers a
// file:line anchor from the assertion's own failure stack trace instead.
type vitestReport struct {
	NumTotalTests int                `json:"numTotalTests"`
	Success       bool               `json:"success"`
	TestResults   []vitestFileResult `json:"testResults"`
}

type vitestFileResult struct {
	Name             string            `json:"name"`
	Status           string            `json:"status"`
	Message          string            `json:"message"`
	AssertionResults []vitestAssertion `json:"assertionResults"`
}

type vitestAssertion struct {
	FullName        string   `json:"fullName"`
	Title           string   `json:"title"`
	Status          string   `json:"status"`
	Duration        float64  `json:"duration"`
	FailureMessages []string `json:"failureMessages"`
}

// vitestStackAnchor matches the first "at <file>:<line>:<col>" frame in a
// Vitest failure message, e.g.:
//
//	"AssertionError: expected -1 to be 999 // Object.is equality\n    at /path/math.test.js:12:24\n    at file:///..."
var vitestStackAnchor = regexp.MustCompile(`at (\S+\.test\.[jt]sx?):(\d+):(\d+)`)

// ParseVitestJSON parses one Vitest `--reporter=json` output file into
// TestOutcomes. A file-level entry with status "failed" and no failed
// assertion (a collection error that prevented any test from running, or a
// file-level hook such as afterAll that threw after its tests passed)
// surfaces as one StateInfrastructure outcome rather than being silently
// dropped, per AGENTS.md invariant 2 (missing evidence must never collapse to
// certainty). A report with no outcome at all (Vitest found no test files)
// returns the run-level "no-suites-collected" failure, as ParsePlaywrightJSON
// does.
func ParseVitestJSON(data []byte) ([]TestOutcome, *InfrastructureFailure, error) {
	var report vitestReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, nil, err
	}
	var out []TestOutcome
	for _, file := range report.TestResults {
		assertionFailed := false
		for _, a := range file.AssertionResults {
			assertionFailed = assertionFailed || a.Status == "failed"
			outcome := TestOutcome{
				Name:       a.Title,
				FullName:   a.FullName,
				State:      vitestState(a.Status),
				DurationMS: a.Duration,
			}
			if len(a.FailureMessages) > 0 {
				outcome.FailureMessage = a.FailureMessages[0]
				outcome.Anchor = vitestAnchorFrom(a.FailureMessages[0])
			}
			out = append(out, outcome)
		}
		if file.Status == "failed" && !assertionFailed {
			out = append(out, TestOutcome{
				Name:           file.Name,
				FullName:       file.Name,
				State:          StateInfrastructure,
				FailureMessage: file.Message,
			})
		}
	}
	if len(out) == 0 {
		return nil, &InfrastructureFailure{Reason: "no-suites-collected", Detail: "the Vitest report lists no test result"}, nil
	}
	return out, nil, nil
}

func vitestState(status string) ExecutionState {
	switch status {
	case "passed":
		return StatePassed
	case "failed":
		return StateFailed
	case "pending", "skipped", "todo":
		return StateSkipped
	default:
		return StateInfrastructure
	}
}

func vitestAnchorFrom(message string) *Anchor {
	m := vitestStackAnchor.FindStringSubmatch(message)
	if m == nil {
		return nil
	}
	line, _ := strconv.Atoi(m[2])
	return &Anchor{File: m[1], Line: line}
}
