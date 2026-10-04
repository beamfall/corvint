package platform

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

var tapPlan = regexp.MustCompile(`^1\.\.([0-9]+)$`)
var tapCase = regexp.MustCompile(`^(ok|not ok) ([1-9][0-9]*) (.+)$`)

func parseBats(in tr.Input, o *tr.Observation) error {
	if len(in.Reports) != 0 || !utf8.Valid(in.Stdout) || len(in.Stdout) == 0 || in.Stdout[len(in.Stdout)-1] != '\n' {
		return fmt.Errorf("Bats requires complete native TAP stdout")
	}
	lines := strings.Split(strings.TrimSuffix(string(in.Stdout), "\n"), "\n")
	plan := tapPlan.FindStringSubmatch(lines[0])
	if len(plan) != 2 {
		return fmt.Errorf("Bats plan must precede result records")
	}
	want, err := strconv.Atoi(plan[1])
	if err != nil || want > tr.MaxTests {
		return fmt.Errorf("Bats plan bound")
	}
	failed := false
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "# ") {
			if len(o.Tests) > 0 && o.Tests[len(o.Tests)-1].State == tr.Failed {
				a := &o.Tests[len(o.Tests)-1]
				a.Message += strings.TrimPrefix(line, "# ") + "\n"
			}
			continue
		}
		m := tapCase.FindStringSubmatch(line)
		if len(m) != 4 {
			return fmt.Errorf("unsupported Bats output record")
		}
		ordinal, _ := strconv.Atoi(m[2])
		if ordinal != len(o.Tests)+1 {
			return fmt.Errorf("Bats ordinal missing or repeated")
		}
		name, state, kind, message := m[3], tr.Passed, "", ""
		if p := strings.Index(name, " # skip"); p >= 0 {
			if m[1] != "ok" {
				return fmt.Errorf("contradictory Bats skip")
			}
			message = strings.TrimSpace(name[p+7:])
			name, state = name[:p], tr.Skipped
		} else if strings.Contains(name, " # ") {
			return fmt.Errorf("Bats extended timing/retry/timeout directive not qualified")
		} else if m[1] == "not ok" {
			// A failing shell command can be an assertion or infrastructure loss.
			// TAP alone does not establish that distinction.
			state, kind, failed = tr.Failed, tr.Unknown, true
		}
		if name == "" {
			return fmt.Errorf("empty Bats identity")
		}
		o.Tests = append(o.Tests, test(name, name, "", "", state, kind, message))
	}
	if len(o.Tests) != want {
		return fmt.Errorf("Bats final count mismatch")
	}
	if (failed && in.ExitCode != 1) || (!failed && in.ExitCode != 0) {
		problem(o, "EXIT_CONTRADICTION", "Bats TAP and exit disagree")
	}
	o.Complete = len(o.Problems) == 0
	return nil
}

func parseShellSpec(in tr.Input, o *tr.Observation) error {
	if len(in.Reports) != 0 {
		return fmt.Errorf("ShellSpec profile reads native JUnit formatter stdout")
	}
	n, err := readXML(in.Stdout)
	if err != nil {
		return err
	}
	if n.name != "testsuites" || n.attr["name"] == "" {
		return fmt.Errorf("not a ShellSpec formatter report")
	}
	rootCounts := map[string]int{}
	for _, k := range []string{"tests", "errors", "failures"} {
		rootCounts[k], err = count(n, k)
		if err != nil {
			return err
		}
	}
	all := map[string]int{}
	for _, suite := range n.children {
		if suite.name != "testsuite" || suite.attr["name"] == "" {
			return fmt.Errorf("invalid ShellSpec suite")
		}
		counts := map[string]int{}
		for _, k := range []string{"tests", "errors", "failures", "skipped"} {
			counts[k], err = count(suite, k)
			if err != nil {
				return err
			}
		}
		actual := map[string]int{}
		for _, c := range suite.children {
			if c.name != "testcase" || c.attr["classname"] != suite.attr["name"] || c.attr["name"] == "" {
				return fmt.Errorf("invalid ShellSpec example identity")
			}
			state, kind, message, terminal := tr.Passed, "", "", 0
			for _, x := range c.children {
				switch x.name {
				case "failure", "error", "skip":
					terminal++
					message = x.attr["message"]
					switch x.name {
					case "failure":
						state, kind = tr.Failed, tr.Assertion
						actual["failures"]++
					case "error":
						state, kind = tr.Failed, tr.Infrastructure
						actual["errors"]++
					case "skip":
						state = tr.Skipped
						actual["skipped"]++
					}
				case "system-out", "system-err":
				default:
					return fmt.Errorf("unsupported ShellSpec example outcome %q", x.name)
				}
			}
			if terminal > 1 {
				return fmt.Errorf("contradictory ShellSpec outcomes")
			}
			id := c.attr["classname"] + "::" + c.attr["name"]
			o.Tests = append(o.Tests, test(id, c.attr["name"], suite.attr["name"], suite.attr["name"], state, kind, message))
			actual["tests"]++
		}
		for k, v := range counts {
			if actual[k] != v {
				return fmt.Errorf("ShellSpec suite %s count mismatch", k)
			}
			all[k] += v
		}
	}
	for k, v := range rootCounts {
		if all[k] != v {
			return fmt.Errorf("ShellSpec aggregate %s count mismatch", k)
		}
	}
	// ShellSpec's failed-example exit is 101, not conventional exit 1.
	failed := all["failures"]+all["errors"] > 0
	if (failed && in.ExitCode != 101) || (!failed && in.ExitCode != 0) {
		problem(o, "EXIT_CONTRADICTION", "ShellSpec formatter and actual exit disagree")
	}
	o.Complete = len(o.Problems) == 0
	return nil
}
