package sql

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

var tapPlan = regexp.MustCompile(`^1\.\.(0|[1-9][0-9]*)$`)
var tapResult = regexp.MustCompile(`^(ok|not ok) ([1-9][0-9]*)(?: (.*))?$`)
var tapSkip = regexp.MustCompile(`(?i)^# SKIP(?: (.*))?$`)

func parseTAP(in tr.Input, o *tr.Observation) error {
	if in.ExitCode != 0 {
		problem(o, "SQL_PROCESS_ERROR", fmt.Sprintf("psql exit %d: %s", in.ExitCode, string(in.Stderr)))
		return nil
	}
	if len(in.Stderr) > 0 {
		problem(o, "SQL_STDERR", "unclassified native SQL diagnostic: "+string(in.Stderr))
	}
	if len(in.Stdout) == 0 {
		problem(o, "NO_TESTS", "no native TAP test inventory")
		return nil
	}
	if in.Stdout[len(in.Stdout)-1] != '\n' {
		return fmt.Errorf("incomplete TAP line")
	}
	plan := -1
	trailingPlan := false
	for _, line := range strings.Split(strings.TrimSuffix(string(in.Stdout), "\n"), "\n") {
		if strings.HasPrefix(line, "Bail out!") {
			problem(o, "TAP_BAILOUT", line)
			return nil
		}
		if strings.HasPrefix(line, "#") {
			if len(o.Tests) > 0 {
				o.Tests[len(o.Tests)-1].Message += line + "\n"
			}
			continue
		}
		if m := tapPlan.FindStringSubmatch(line); m != nil {
			if plan >= 0 {
				return fmt.Errorf("duplicate TAP plan")
			}
			n, e := strconv.Atoi(m[1])
			if e != nil || n > tr.MaxTests {
				return fmt.Errorf("TAP plan bound")
			}
			plan = n
			trailingPlan = len(o.Tests) > 0
			continue
		}
		m := tapResult.FindStringSubmatch(line)
		if m == nil || trailingPlan {
			return fmt.Errorf("unknown TAP record or case after terminal plan")
		}
		n, e := strconv.Atoi(m[2])
		if e != nil || n != len(o.Tests)+1 || n > tr.MaxTests {
			return fmt.Errorf("TAP ordinal missing, duplicate or out of bounds")
		}
		tail := m[3]
		description := ""
		directive := ""
		if strings.HasPrefix(tail, "#") {
			directive = tail
		} else if strings.HasPrefix(tail, "- ") {
			description = strings.TrimPrefix(tail, "- ")
			if i := strings.Index(description, " # "); i >= 0 {
				directive = description[i+1:]
				description = description[:i]
			}
		} else if tail != "" {
			return fmt.Errorf("unqualified pgTAP description syntax")
		}
		state, kind, message := tr.Passed, "", ""
		if m[1] == "not ok" {
			state, kind = tr.Failed, tr.Unknown
		}
		if directive != "" {
			skip := tapSkip.FindStringSubmatch(directive)
			if skip == nil || state == tr.Failed {
				return fmt.Errorf("unqualified or contradictory TAP directive")
			}
			state = tr.Skipped
			message = skip[1]
		}
		id := in.SourceFile + "::" + m[2]
		if description != "" {
			id += "::" + description
		}
		o.Tests = append(o.Tests, tr.Test{ID: id, Name: description, File: in.SourceFile, Suite: in.SourceFile, Granularity: "CASE", State: state, FailureKind: kind, Message: message})
	}
	if plan < 0 {
		return fmt.Errorf("missing TAP plan")
	}
	if plan != len(o.Tests) {
		return fmt.Errorf("native TAP plan/result count mismatch")
	}
	if plan == 0 {
		problem(o, "NO_TESTS", "native TAP reported no tests")
	}
	return nil
}
