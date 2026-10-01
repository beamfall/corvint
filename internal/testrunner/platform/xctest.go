package platform

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

var xctestSelector = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)+/[A-Za-z_][A-Za-z0-9_]*$`)
var xctestSuite = regexp.MustCompile(`^Test Suite '([^']+)' (started|passed|failed) at [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9:.]+\.$`)
var xctestStart = regexp.MustCompile(`^Test Case '-\[([^\]']+)\]' started\.$`)
var xctestEnd = regexp.MustCompile(`^Test Case '-\[([^\]']+)\]' (passed|failed|skipped) \([0-9.]+ seconds\)\.$`)
var xctestIssue = regexp.MustCompile(`^(.+):([0-9]+): (error: )?-\[([^\]]+)\] : (.*)$`)
var xctestSummary = regexp.MustCompile(`^\s*Executed ([0-9]+) tests?, with (?:([0-9]+) tests? skipped and )?([0-9]+) failures? \(([0-9]+) unexpected\) in [0-9.]+ \([0-9.]+\) seconds$`)

type xctestFrame struct {
	name   string
	first  int
	result string
}
type xctestFact struct {
	failures, unexpected int
	skipped              bool
}

func buildXCTest(r tr.Request, i *tr.Invocation) error {
	pkg := r.Root
	if r.Project != "" {
		if !relative(r.Project) {
			return fmt.Errorf("SwiftPM package path must be literal")
		}
		pkg = filepath.Join(pkg, r.Project)
	}
	if r.Config != filepath.Join(pkg, "Package.swift") || !digest.MatchString(r.ConfigSha256) {
		return fmt.Errorf("SwiftPM XCTest requires the selected package's pinned Package.swift")
	}
	i.Argv = []string{"test", "--package-path", pkg, "--enable-xctest", "--disable-swift-testing", "--no-parallel", "--disable-automatic-resolution", "--skip-update"}
	for _, s := range r.Selectors {
		if !xctestSelector.MatchString(s) {
			return fmt.Errorf("XCTest selector requires exact Module.Class/testMethod")
		}
		i.Argv = append(i.Argv, "--filter", "^"+regexp.QuoteMeta(s)+"$")
	}
	return nil
}

// This profile is SwiftPM6.4/macOS serial XCTest's native protocol. Unlike its
// parallel xUnit writer, the native terminal explicitly distinguishes a skip.
// Suite summaries count failure events, not failing test methods.
func parseXCTest(in tr.Input, o *tr.Observation) error {
	if len(in.Reports) != 0 || !utf8.Valid(in.Stdout) || len(in.Stdout) == 0 || in.Stdout[len(in.Stdout)-1] != '\n' {
		return fmt.Errorf("complete native XCTest stdout required")
	}
	var stack []xctestFrame
	var facts []xctestFact
	active, firstFile, message := "", "", ""
	issueCount, unexpectedCount := 0, 0
	skipped, unknownIssue := false, false
	started, finished, note := false, false, false
	failedModules := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(string(in.Stdout), "\n"), "\n") {
		if line == "" {
			continue
		}
		if len(stack) > 0 && stack[len(stack)-1].result != "" {
			m := xctestSummary.FindStringSubmatch(line)
			if m == nil {
				return fmt.Errorf("XCTest suite missing native aggregate summary")
			}
			counts := []int{0, 0, 0, 0}
			for j := 1; j <= 4; j++ {
				if m[j] != "" {
					v, e := strconv.Atoi(m[j])
					if e != nil || v < 0 || v > tr.MaxTests*32 {
						return fmt.Errorf("XCTest count bound")
					}
					counts[j-1] = v
				}
			}
			frame := stack[len(stack)-1]
			actual := []int{len(facts) - frame.first, 0, 0, 0}
			for _, f := range facts[frame.first:] {
				if f.skipped {
					actual[1]++
				}
				actual[2] += f.failures
				actual[3] += f.unexpected
			}
			for j := range counts {
				if counts[j] != actual[j] {
					return fmt.Errorf("XCTest native suite count contradiction")
				}
			}
			if (frame.result == "failed") != (actual[2] > 0) {
				return fmt.Errorf("XCTest suite outcome contradicts failure events")
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				finished = true
			}
			continue
		}
		if m := xctestSuite.FindStringSubmatch(line); m != nil {
			if active != "" || finished {
				return fmt.Errorf("XCTest suite transition outside lifecycle")
			}
			if m[2] == "started" {
				if len(stack) == 0 {
					if started || (m[1] != "All tests" && m[1] != "Selected tests") {
						return fmt.Errorf("XCTest native root missing/duplicated")
					}
					started = true
				}
				if len(stack) >= 32 {
					return fmt.Errorf("XCTest nesting bound")
				}
				stack = append(stack, xctestFrame{name: m[1], first: len(facts)})
			} else {
				if len(stack) == 0 || stack[len(stack)-1].name != m[1] {
					return fmt.Errorf("XCTest suite end without matching start")
				}
				stack[len(stack)-1].result = m[2]
			}
			continue
		}
		if m := xctestStart.FindStringSubmatch(line); m != nil {
			if finished || len(stack) == 0 || active != "" {
				return fmt.Errorf("XCTest overlapping/unframed test start")
			}
			class, method, ok := strings.Cut(m[1], " ")
			if !ok || !xctestSelector.MatchString(class+"/"+method) {
				return fmt.Errorf("unqualified XCTest native identity")
			}
			active = m[1]
			firstFile = ""
			message = ""
			issueCount = 0
			unexpectedCount = 0
			skipped = false
			unknownIssue = false
			continue
		}
		if m := xctestEnd.FindStringSubmatch(line); m != nil {
			if active == "" || active != m[1] {
				return fmt.Errorf("XCTest terminal without matching start")
			}
			state := map[string]string{"passed": tr.Passed, "failed": tr.Failed, "skipped": tr.Skipped}[m[2]]
			if (state == tr.Failed) != (issueCount > 0) || (state == tr.Skipped && !skipped) || (state == tr.Passed && skipped) {
				return fmt.Errorf("XCTest terminal contradicts native issue/skip observations")
			}
			class, method, _ := strings.Cut(active, " ")
			kind := ""
			if state == tr.Failed {
				kind = tr.Assertion
				if unexpectedCount > 0 {
					kind = tr.Infrastructure
				} else if unknownIssue {
					kind = tr.Unknown
				}
				module, _, _ := strings.Cut(class, ".")
				failedModules[module] = true
			}
			names := []string{}
			for _, s := range stack {
				names = append(names, s.name)
			}
			o.Tests = append(o.Tests, test(class+"/"+method, method, firstFile, strings.Join(names, "/"), state, kind, strings.TrimSpace(message)))
			facts = append(facts, xctestFact{issueCount, unexpectedCount, skipped})
			active = ""
			if len(o.Tests) > tr.MaxTests {
				return fmt.Errorf("XCTest inventory bound")
			}
			continue
		}
		if m := xctestIssue.FindStringSubmatch(line); m != nil {
			if active == "" || m[4] != active {
				return fmt.Errorf("XCTest issue outside its active test")
			}
			if firstFile == "" {
				firstFile = m[1]
			}
			message += line + "\n"
			if m[3] == "error: " {
				issueCount++
				if strings.HasPrefix(m[5], "failed: caught error:") {
					unexpectedCount++
				} else if !strings.HasPrefix(m[5], "XCTAssert") && !strings.HasPrefix(m[5], "XCTFail") && !strings.HasPrefix(m[5], "failed -") {
					unknownIssue = true
				}
			} else if strings.HasPrefix(m[5], "Test skipped -") || strings.HasPrefix(m[5], "Test skipped: threw error ") {
				if skipped {
					return fmt.Errorf("duplicate XCTest skip")
				}
				skipped = true
			} else {
				return fmt.Errorf("unrecognized XCTest native diagnostic")
			}
			continue
		}
		if finished && line == "Note: Some test targets reported failures:" {
			if note || len(failedModules) == 0 {
				return fmt.Errorf("contradictory XCTest failure note")
			}
			note = true
			continue
		}
		if finished && note && strings.HasPrefix(line, "  - ") && strings.HasSuffix(line, " (XCTest)") {
			name := strings.TrimSuffix(strings.TrimPrefix(line, "  - "), " (XCTest)")
			if !failedModules[name] {
				return fmt.Errorf("XCTest failure note references an unobserved target")
			}
			continue
		}
		if strings.HasPrefix(line, "Test Case ") || strings.HasPrefix(line, "Test Suite ") || strings.HasPrefix(strings.TrimSpace(line), "Executed ") {
			return fmt.Errorf("unrecognized XCTest protocol frame")
		}
		if active != "" {
			message += line + "\n"
		} else {
			return fmt.Errorf("unframed XCTest output")
		}
	}
	if !started || !finished || len(stack) != 0 || active != "" {
		return fmt.Errorf("incomplete XCTest native lifecycle")
	}
	failed := len(failedModules) > 0
	if (failed && in.ExitCode != 1) || (!failed && in.ExitCode != 0) {
		problem(o, "EXIT_CONTRADICTION", "XCTest native outcome and process exit disagree")
	}
	o.Complete = len(o.Problems) == 0
	return nil
}
