package testrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Digest identifies retained bytes, not their authority or causal provenance.
func Digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func Identity(v any) string  { b, _ := json.Marshal(v); return Digest(b) }

// Normalize enforces the shared boundary after runner-specific parsing.
func Normalize(in Input, o Observation) Observation {
	o.Runner = in.Runner
	o.Problems = append(o.Problems, in.ExecutionProblems...)
	problem := func(code, detail string) { o.Problems = append(o.Problems, Problem{code, detail}); o.Complete = false }
	if in.TimedOut {
		problem("timeout", "runner did not finish within its admitted duration")
	}
	if in.Interrupted {
		problem("interrupted", "runner was cancelled")
	}
	if in.Overflow {
		problem("output-overflow", "retained output exceeded its bound")
	}
	if len(in.Stdout) > MaxReportBytes || len(in.Stderr) > MaxReportBytes || len(in.Reports) > MaxReports {
		problem("input-bound", "input exceeds the closed observation bounds")
	}
	for _, b := range in.Reports {
		if len(b) > MaxReportBytes {
			problem("report-bound", "report exceeds byte bound")
		}
	}
	if len(o.Tests) == 0 {
		problem("no-tests", "no observed test inventory")
	}
	if len(o.Tests) > MaxTests {
		problem("test-bound", "observed test inventory exceeds bound")
	}
	seen := map[string]bool{}
	failed := false
	for _, t := range o.Tests {
		if t.Granularity != "" && t.Granularity != "CASE" && t.Granularity != "SUITE_ONLY" {
			problem("unknown-granularity", "unrecognized native inventory granularity")
		}
		for _, n := range []*int{t.ExecutedCount, t.SkippedCount} {
			if n != nil && (*n < 0 || *n > MaxTests) {
				problem("aggregate-count-bound", "native aggregate count exceeds bound")
			}
		}
		if t.Granularity == "SUITE_ONLY" && (t.ExecutedCount == nil || t.SkippedCount == nil) {
			problem("aggregate-count-missing", "suite-only observation requires native counts")
		}
		if t.Granularity == "SUITE_ONLY" && t.ExecutedCount != nil && t.SkippedCount != nil {
			if *t.ExecutedCount+*t.SkippedCount > MaxTests {
				problem("aggregate-count-bound", "native aggregate total exceeds bound")
			}
			if (t.State == Passed && *t.ExecutedCount == 0) || (t.State == Skipped && *t.ExecutedCount != 0) {
				problem("aggregate-state-conflict", "native aggregate state contradicts executed/skipped counts")
			}
		}
		if t.Granularity == "SUITE_ONLY" && t.ExecutedCount != nil && t.SkippedCount != nil && *t.ExecutedCount+*t.SkippedCount == 0 {
			problem("no-executed-inventory", "empty native suite cannot establish observed test inventory")
		}
		if t.ID == "" || seen[t.ID] {
			problem("ambiguous-test-identity", "empty or duplicate native test identity")
		}
		seen[t.ID] = true
		if !state(t.State) {
			problem("unknown-test-state", "unrecognized native state")
		}
		if t.State == Failed {
			failed = true
		}
		if t.State == Unknown || t.State == Interrupted || t.State == TimedOut {
			problem("unresolved-test-state", "native test did not retain a resolved terminal state")
		}
		if len(t.Attempts) > MaxAttempts {
			problem("attempt-bound", "reported attempt count exceeds bound")
		}
		for _, a := range t.Attempts {
			if !state(a.State) {
				problem("unknown-attempt-state", "unrecognized attempt state")
			}
		}
	}
	expected := map[string]bool{}
	for _, id := range in.Expected {
		if id == "" || expected[id] {
			problem("ambiguous-expected-selector", "empty or duplicate expected selector")
		}
		expected[id] = true
		if !seen[id] {
			problem("missing-selected-test", "expected native test identity absent")
		}
	}
	if err := exitProfile(in.SuccessExitCodes, in.FailureExitCodes, in.OutcomeNeutralExitCodes); err != nil {
		problem("invalid-exit-profile", err.Error())
	}
	success, failure := in.SuccessExitCodes, in.FailureExitCodes
	if len(success) == 0 && len(in.OutcomeNeutralExitCodes) == 0 {
		success = []int{0}
	}
	if len(failure) == 0 && len(in.OutcomeNeutralExitCodes) == 0 {
		failure = []int{1}
	}
	contains := func(xs []int, n int) bool {
		for _, x := range xs {
			if x == n {
				return true
			}
		}
		return false
	}
	if in.ExitCode < 0 || (!contains(success, in.ExitCode) && !contains(failure, in.ExitCode) && !contains(in.OutcomeNeutralExitCodes, in.ExitCode)) {
		problem("runner-exit", "runner exit is outside its admitted native status profile")
	}
	if contains(success, in.ExitCode) && failed {
		problem("contradictory-exit", "reported test failure with successful process exit")
	}
	if contains(failure, in.ExitCode) && !failed {
		problem("unclassified-exit", "failed process has no ordinary observed test failure")
	}
	if o.RetryInformation != Retained && o.RetryInformation != NotReported && o.RetryInformation != NotApplicable {
		problem("unknown-retry-information", "retry observability was not declared")
	}
	if len(o.Problems) > 0 {
		o.Complete = false
		// Keep identities and native diagnostics, but do not expose a resolved
		// result after execution, inventory or report validity has failed.
		for i := range o.Tests {
			o.Tests[i].State = Unknown
			for j := range o.Tests[i].Attempts {
				o.Tests[i].Attempts[j].State = Unknown
			}
		}
	}
	return o
}
func state(s string) bool {
	switch s {
	case Passed, Failed, Skipped, Flaky, TimedOut, Interrupted, Unknown:
		return true
	}
	return false
}
func relative(name string) error {
	if name == "" || len(name) > 512 || strings.ContainsAny(name, "\\\x00\r\n") || strings.HasPrefix(name, "/") {
		return fmt.Errorf("invalid relative artifact path")
	}
	for _, p := range strings.Split(name, "/") {
		if p == "" || p == "." || p == ".." || strings.EqualFold(p, ".git") {
			return fmt.Errorf("invalid relative artifact segment")
		}
	}
	return nil
}

func exitProfile(success, failure, neutral []int) error {
	if len(success) == 0 && len(neutral) == 0 {
		success = []int{0}
	}
	if len(failure) == 0 && len(neutral) == 0 {
		failure = []int{1}
	}
	seen := map[int]bool{}
	for _, list := range [][]int{success, failure, neutral} {
		if len(list) > 256 {
			return fmt.Errorf("exit profile exceeds bound")
		}
		for _, code := range list {
			if code < 0 || code > 255 || seen[code] {
				return fmt.Errorf("exit profile must contain disjoint bounded native statuses")
			}
			seen[code] = true
		}
	}
	return nil
}
