package platform

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

var emulatorSerial = regexp.MustCompile(`^emulator-[0-9]{4,5}$`)
var androidComponent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*/androidx\.test\.runner\.AndroidJUnitRunner$`)
var androidOK = regexp.MustCompile(`^OK \(([0-9]+) tests?\)$`)
var androidFailed = regexp.MustCompile(`^Tests run: ([0-9]+),\s+Failures: ([0-9]+)$`)

func buildAndroid(r tr.Request, i *tr.Invocation) error {
	if !emulatorSerial.MatchString(r.Target) || !androidComponent.MatchString(r.Project) {
		return fmt.Errorf("Android profile requires an explicit local emulator serial and AndroidJUnitRunner component")
	}
	// APK installation and device state are independent admission obligations;
	// this profile never installs, grants permissions, or clears data. The adb server is an external runtime dependency.
	i.Argv = []string{"-s", r.Target, "shell", "am", "instrument", "-w", "-r"}
	if len(r.Selectors) > 0 {
		for _, s := range r.Selectors {
			if !javaSelector.MatchString(s) {
				return fmt.Errorf("Android selectors require exact class#method")
			}
		}
		i.Argv = append(i.Argv, "-e", "class", strings.Join(r.Selectors, ","))
	}
	i.Argv = append(i.Argv, r.Project)
	// adb's zero means the shell command completed, even if native tests failed.
	i.SuccessExitCodes = nil
	i.FailureExitCodes = nil
	i.OutcomeNeutralExitCodes = []int{0}
	return nil
}

func parseAndroid(in tr.Input, o *tr.Observation) error {
	if len(in.Reports) != 0 || !utf8.Valid(in.Stdout) || len(in.Stdout) == 0 || in.Stdout[len(in.Stdout)-1] != '\n' {
		return fmt.Errorf("Android requires complete native instrumentation stdout")
	}
	status := map[string]string{}
	last := ""
	resultStream := ""
	resultStarted, finished := false, false
	current, total, ignored, failures := 0, -1, 0, 0
	active := ""
	number := func(s string) (int, error) {
		n, e := strconv.Atoi(s)
		if e != nil || n < 0 || n > tr.MaxTests || strconv.Itoa(n) != s {
			return 0, fmt.Errorf("invalid Android count")
		}
		return n, nil
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(in.Stdout), "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if finished {
			if line != "" {
				return fmt.Errorf("content after Android completion")
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "INSTRUMENTATION_STATUS: "):
			if resultStarted {
				return fmt.Errorf("Android status after result")
			}
			key, value, ok := strings.Cut(strings.TrimPrefix(line, "INSTRUMENTATION_STATUS: "), "=")
			if !ok {
				return fmt.Errorf("malformed Android status")
			}
			switch key {
			case "class", "test", "current", "numtests", "id", "stack", "stream":
			default:
				return fmt.Errorf("unrecognized Android status field %q", key)
			}
			if _, ok = status[key]; ok {
				return fmt.Errorf("duplicate Android status field")
			}
			status[key] = value
			last = key
		case strings.HasPrefix(line, "INSTRUMENTATION_STATUS_CODE: "):
			if resultStarted {
				return fmt.Errorf("Android status code after result")
			}
			code, e := strconv.Atoi(strings.TrimPrefix(line, "INSTRUMENTATION_STATUS_CODE: "))
			if e != nil {
				return e
			}
			if status["id"] != "AndroidJUnitRunner" || status["class"] == "" || status["test"] == "" || len(status["class"])+len(status["test"]) > 4096 {
				return fmt.Errorf("Android runner/test identity missing")
			}
			n, e := number(status["numtests"])
			if e != nil {
				return e
			}
			ordinal, e := number(status["current"])
			if e != nil {
				return e
			}
			if total < 0 {
				total = n
			}
			if total != n || ordinal < 1 || ordinal > total {
				return fmt.Errorf("Android inventory mismatch")
			}
			id := status["class"] + "#" + status["test"]
			if code == 1 {
				if active != "" || ordinal != current+1 {
					return fmt.Errorf("duplicate/out-of-order Android start")
				}
				active = id
				current = ordinal
			} else {
				if active != id || ordinal != current {
					return fmt.Errorf("Android terminal without matching start")
				}
				state, kind := tr.Passed, ""
				switch code {
				case 0:
					if status["stack"] != "" {
						return fmt.Errorf("Android pass contains failure stack")
					}
				case -1:
					state, kind = tr.Failed, tr.Infrastructure
					failures++
				case -2:
					state, kind = tr.Failed, tr.Unknown
					failures++
					for _, prefix := range []string{"java.lang.AssertionError", "junit.framework.AssertionFailedError", "org.opentest4j.AssertionFailedError"} {
						if strings.HasPrefix(status["stack"], prefix+":") || strings.HasPrefix(status["stack"], prefix+"\n") || status["stack"] == prefix {
							kind = tr.Assertion
						}
					}
				case -3:
					state = tr.Skipped
					ignored++
				case -4:
					state = tr.Skipped
				default:
					return fmt.Errorf("unknown Android outcome code")
				}
				if state == tr.Failed && status["stack"] == "" {
					return fmt.Errorf("Android failure missing observation")
				}
				o.Tests = append(o.Tests, test(id, status["test"], "", status["class"], state, kind, strings.TrimSpace(status["stack"])))
				active = ""
			}
			status = map[string]string{}
			last = ""
		case strings.HasPrefix(line, "INSTRUMENTATION_RESULT: "):
			if resultStarted || active != "" || len(status) != 0 {
				return fmt.Errorf("Android result before complete test inventory")
			}
			key, value, ok := strings.Cut(strings.TrimPrefix(line, "INSTRUMENTATION_RESULT: "), "=")
			if !ok || key != "stream" {
				return fmt.Errorf("Android process/build result is not native test completion")
			}
			resultStarted = true
			resultStream = value + "\n"
			last = ""
		case strings.HasPrefix(line, "INSTRUMENTATION_CODE: "):
			if !resultStarted || strings.TrimPrefix(line, "INSTRUMENTATION_CODE: ") != "-1" {
				return fmt.Errorf("Android instrumentation did not complete")
			}
			finished = true
		default:
			if strings.HasPrefix(line, "INSTRUMENTATION_") {
				return fmt.Errorf("unrecognized Android instrumentation record")
			}
			if resultStarted {
				resultStream += line + "\n"
			} else if last == "stack" || last == "stream" {
				status[last] += "\n" + line
			} else if line != "" {
				return fmt.Errorf("unframed Android output")
			}
		}
	}
	if !finished || active != "" || len(status) != 0 {
		return fmt.Errorf("incomplete Android instrumentation stream")
	}
	if total < 0 {
		total = 0
	}
	if len(o.Tests) != total || current != total {
		return fmt.Errorf("Android observed inventory count mismatch")
	}
	summaries := 0
	for _, line := range strings.Split(resultStream, "\n") {
		if m := androidOK.FindStringSubmatch(line); m != nil {
			summaries++
			n, e := number(m[1])
			if e != nil || n != total-ignored || failures != 0 {
				return fmt.Errorf("Android success summary contradiction")
			}
		} else if m := androidFailed.FindStringSubmatch(line); m != nil {
			summaries++
			n, e := number(m[1])
			f, e2 := number(m[2])
			if e != nil || e2 != nil || n != total-ignored || f != failures || failures == 0 {
				return fmt.Errorf("Android failure summary contradiction")
			}
		}
	}
	if summaries != 1 {
		return fmt.Errorf("Android native aggregate summary missing/ambiguous")
	}
	if in.ExitCode != 0 {
		problem(o, "ADB_FAILURE", "adb transport/shell did not exit successfully")
	}
	o.Complete = len(o.Problems) == 0
	return nil
}
