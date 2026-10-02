// Package platform reads runner-specific JVM, Apple and shell observations.
// Native report integrity is not authenticated execution or assertion adequacy.
package platform

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// Runners includes explicit unavailable obligations; Build never silently
// substitutes a generic XML runner for a platform that has not been qualified.
func Runners() []string {
	return []string{"swift-testing", "swift-xctest", "xcode-xctest", "junit-platform", "junit4", "testng", "gradle-junit", "maven-surefire", "kotest", "kotlin-test", "android-junit", "espresso", "uiautomator", "robolectric", "appium", "bats", "shellspec", "pgtap", "sqllogictest", "shader-behavior", "html-behavior", "structured-data-behavior"}
}

var digest = regexp.MustCompile(`^[0-9a-f]{64}$`)
var javaSelector = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$.]*#[A-Za-z_$][A-Za-z0-9_$]*$`)

func relative(p string) bool {
	return p != "" && !filepath.IsAbs(p) && filepath.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../") && !strings.ContainsAny(p, "\x00\r\n:*?[]\\") && !strings.HasPrefix(p, "-")
}

func Build(r tr.Request) (tr.Invocation, error) {
	i := tr.Invocation{Format: r.Runner, SuccessExitCodes: []int{0}, FailureExitCodes: []int{1}, Environment: map[string]string{}, Files: map[string][]byte{}}
	if !filepath.IsAbs(r.Executable) || !digest.MatchString(r.ExecutableSha256) || !filepath.IsAbs(r.Root) || !filepath.IsAbs(r.ReportDir) || len(r.Selectors) > tr.MaxTests {
		return i, fmt.Errorf("platform runner requires pinned absolute tool, root and artifact directory")
	}
	seen := map[string]bool{}
	for _, s := range r.Selectors {
		if s == "" || len(s) > 4096 || strings.ContainsAny(s, "\x00\r\n") || seen[s] {
			return i, fmt.Errorf("invalid or duplicate selector")
		}
		seen[s] = true
	}
	switch r.Runner {
	case "swift-xctest":
		if err := buildXCTest(r, &i); err != nil {
			return i, err
		}
	case "gradle-junit", "maven-surefire":
		if err := buildJVMProject(r, &i); err != nil {
			return i, err
		}
	case "android-junit", "espresso", "uiautomator":
		if err := buildAndroid(r, &i); err != nil {
			return i, err
		}
	case "xcode-xctest":
		i.FailureExitCodes = []int{65}
		if !relative(r.Project) || !strings.HasSuffix(r.Project, ".xcodeproj") || !filepath.IsAbs(r.Config) || !strings.HasSuffix(r.Config, ".xcscheme") || !digest.MatchString(r.ConfigSha256) {
			return i, fmt.Errorf("Xcode requires literal project and pinned shared scheme file")
		}
		if _, ok := r.Tools["xcresulttool"]; !ok {
			return i, fmt.Errorf("Xcode requires independently pinned xcresulttool")
		}
		scheme := filepath.Join(r.Root, r.Project, "xcshareddata", "xcschemes", filepath.Base(r.Config))
		if r.Config != scheme {
			return i, fmt.Errorf("Xcode config must be the selected project shared scheme")
		}
		bundle := filepath.Join(r.ReportDir, "result.xcresult")
		args := []string{"test", "-project", filepath.Join(r.Root, r.Project), "-scheme", strings.TrimSuffix(filepath.Base(r.Config), ".xcscheme"), "-destination", "platform=macOS", "-resultBundlePath", bundle, "-derivedDataPath", filepath.Join(r.ReportDir, "DerivedData"), "CODE_SIGNING_ALLOWED=NO"}
		for _, s := range r.Selectors {
			if !relative(s) || strings.Count(s, "/") != 2 {
				return i, fmt.Errorf("Xcode selector requires exact target/class/method")
			}
			args = append(args, "-only-testing:"+s)
		}
		i.Phases = []tr.Phase{{Kind: "TEST", Tool: "primary", Argv: args}, {Kind: "DECODE", Tool: "xcresulttool", Argv: []string{"get", "test-results", "tests", "--path", bundle, "--compact", "--schema-version", "0.4.0"}, StdoutReport: "xcode-tests.json"}, {Kind: "DECODE", Tool: "xcresulttool", Argv: []string{"get", "test-results", "summary", "--path", bundle, "--compact", "--schema-version", "0.4.0"}, StdoutReport: "xcode-summary.json"}}
		i.ReportPaths = []string{"xcode-tests.json", "xcode-summary.json"}
	case "swift-testing":
		if r.Project != "" && !relative(r.Project) {
			return i, fmt.Errorf("Swift package path must be literal repository-relative")
		}
		pkg := r.Root
		if r.Project != "" {
			pkg = filepath.Join(pkg, r.Project)
		}
		i.Argv = []string{"test", "--package-path", pkg, "--disable-xctest", "--enable-swift-testing", "--no-parallel", "--event-stream-version", "0", "--event-stream-output-path", filepath.Join(r.ReportDir, "swift-events.jsonl")}
		for _, s := range r.Selectors {
			// SwiftPM filters the discovery identifier, not the event's source suffix.
			if strings.Contains(s, "/") || !strings.Contains(s, ".") {
				return i, fmt.Errorf("Swift selector must be the exact module-qualified discovery name")
			}
			i.Argv = append(i.Argv, "--filter", "^"+regexp.QuoteMeta(s)+"$")
		}
		i.ReportPaths = []string{"swift-events.jsonl"}
	case "junit-platform", "junit4", "kotlin-test", "kotest", "robolectric":
		if !filepath.IsAbs(r.Reporter) || !digest.MatchString(r.ReporterSha256) || !relative(r.Project) {
			return i, fmt.Errorf("JUnit requires a pinned console standalone jar and repository-relative classes directory")
		}
		engine := "junit-jupiter"
		if r.Runner == "junit4" || r.Runner == "robolectric" {
			engine = "junit-vintage"
		} else if r.Runner == "kotest" {
			engine = "kotest"
		}
		i.Argv = []string{"-jar", r.Reporter, "execute", "--class-path", filepath.Join(r.Root, r.Project), "--include-engine", engine, "--reports-dir", r.ReportDir, "--disable-banner", "--disable-ansi-colors", "--fail-if-no-tests"}
		if r.Runner == "kotlin-test" || r.Runner == "kotest" || r.Runner == "robolectric" {
			cp := r.Reporter + string(filepath.ListSeparator) + filepath.Join(r.Root, r.Project) + string(filepath.ListSeparator) + filepath.Join(r.Root, r.Project, "*")
			i.Argv = []string{"-cp", cp, "org.junit.platform.console.ConsoleLauncher", "execute", "--class-path", filepath.Join(r.Root, r.Project), "--include-engine", engine, "--reports-dir", r.ReportDir, "--disable-banner", "--disable-ansi-colors", "--fail-if-no-tests", "--include-classname", ".+"}
		}
		if r.Runner == "robolectric" {
			if r.Target != "35" || !filepath.IsAbs(r.Config) || !digest.MatchString(r.ConfigSha256) || filepath.Base(r.Config) != "android-all-instrumented-15-robolectric-13954326-i7.jar" {
				return i, fmt.Errorf("Robolectric4.16.1 profile requires SDK35 and its pinned exact instrumented SDK artifact")
			}
			i.Argv = append([]string{"-Drobolectric.offline=true", "-Drobolectric.enabledSdks=35", "-Drobolectric.dependency.dir=" + filepath.Dir(r.Config)}, i.Argv...)
		}
		if len(r.Selectors) == 0 {
			i.Argv = append(i.Argv, "--scan-class-path="+filepath.Join(r.Root, r.Project))
		}
		for _, s := range r.Selectors {
			if javaSelector.MatchString(s) && r.Runner != "kotest" {
				i.Argv = append(i.Argv, "--select-method", s)
			} else if strings.HasPrefix(s, "[engine:"+engine+"]/") {
				i.Argv = append(i.Argv, "--select-unique-id", s)
			} else {
				return i, fmt.Errorf("JUnit requires an exact class#method or native unique ID")
			}
		}
		i.ReportPaths = []string{"TEST-" + engine + ".xml"}
	case "testng":
		i.SuccessExitCodes = []int{0, 2}
		i.FailureExitCodes = []int{1, 3}
		if !relative(r.Project) || !filepath.IsAbs(r.Config) || !digest.MatchString(r.ConfigSha256) || len(r.Selectors) != 0 {
			return i, fmt.Errorf("TestNG requires pinned suite XML, repository-relative classpath directory, and suite-owned selectors")
		}
		cp := filepath.Join(r.Root, r.Project)
		i.Argv = []string{"-cp", cp + string(filepath.ListSeparator) + filepath.Join(cp, "*"), "org.testng.TestNG", "-usedefaultlisteners", "false", "-reporter", "org.testng.reporters.XMLReporter", "-d", r.ReportDir, r.Config}
		i.ReportPaths = []string{"testng-results.xml"}
	case "bats":
		if !relative(r.Project) || !strings.HasSuffix(r.Project, ".bats") || len(r.Selectors) > 1 {
			return i, fmt.Errorf("Bats requires one literal .bats file and at most one exact test-name selector")
		}
		i.Argv = []string{"--formatter", "tap"}
		if len(r.Selectors) == 1 {
			i.Argv = append(i.Argv, "--filter", "^"+regexp.QuoteMeta(r.Selectors[0])+"$")
		}
		i.Argv = append(i.Argv, filepath.Join(r.Root, r.Project))
	case "shellspec":
		i.FailureExitCodes = []int{101}
		if !relative(r.Project) || !strings.HasSuffix(r.Project, ".sh") || len(r.Selectors) != 0 {
			return i, fmt.Errorf("ShellSpec requires one literal spec file; name-pattern filtering is not an exact selector")
		}
		if _, ok := r.Tools["shell"]; !ok {
			return i, fmt.Errorf("ShellSpec requires independently pinned shell tool")
		}
		i.Argv = []string{"--shell", r.Tools["shell"].Executable, "--format", "junit", "--no-color", "--no-banner", r.Project}
	default:
		return i, fmt.Errorf("runner %q capability unavailable: native runtime/profile qualification is an open obligation", r.Runner)
	}
	return i, nil
}

func Parse(in tr.Input) (o tr.Observation, err error) {
	o = tr.Observation{Runner: in.Runner, Tests: []tr.Test{}, Problems: []tr.Problem{}, RetryInformation: tr.NotReported}
	defer func() {
		state, code := "", ""
		if err != nil {
			state, code = tr.Unknown, "INVALID_NATIVE_REPORT"
		}
		if in.Overflow {
			state, code = tr.Unknown, "OUTPUT_OVERFLOW"
		}
		if in.Interrupted {
			state, code = tr.Interrupted, "PROCESS_INTERRUPTED"
		}
		if in.TimedOut {
			state, code = tr.TimedOut, "PROCESS_TIMEOUT"
		}
		if state != "" {
			for j := range o.Tests {
				o.Tests[j].State = state
				o.Tests[j].FailureKind = tr.Infrastructure
				for k := range o.Tests[j].Attempts {
					o.Tests[j].Attempts[k].State = state
					o.Tests[j].Attempts[k].FailureKind = tr.Infrastructure
				}
			}
			problem(&o, code, "the report or process boundary prevents a complete test observation")
		}
	}()
	if len(in.Reports) > tr.MaxReports || len(in.Stdout) > tr.MaxReportBytes || len(in.Stderr) > tr.MaxReportBytes {
		return o, fmt.Errorf("platform report bound exceeded")
	}
	for _, b := range in.Reports {
		if len(b) > tr.MaxReportBytes {
			return o, fmt.Errorf("platform report bound exceeded")
		}
	}
	switch in.Runner {
	case "swift-xctest":
		err = parseXCTest(in, &o)
	case "gradle-junit", "maven-surefire":
		err = parseJVMProject(in, &o)
	case "android-junit", "espresso", "uiautomator":
		err = parseAndroid(in, &o)
	case "xcode-xctest":
		err = parseXcode(in, &o)
	case "swift-testing":
		err = parseSwift(in, &o)
	case "junit-platform", "junit4", "kotlin-test", "kotest", "robolectric":
		err = parseJUnit(in, &o)
	case "testng":
		err = parseTestNG(in, &o)
	case "bats":
		err = parseBats(in, &o)
	case "shellspec":
		err = parseShellSpec(in, &o)
	default:
		return o, fmt.Errorf("runner %q native parser unavailable", in.Runner)
	}
	if err != nil {
		o.Complete = false
		return o, err
	}
	if len(o.Tests) == 0 {
		problem(&o, "NO_TESTS", "the runner did not report an executed or skipped test")
	}
	if len(o.Tests) > tr.MaxTests {
		return o, fmt.Errorf("test inventory exceeds bound")
	}
	seen := map[string]bool{}
	for _, t := range o.Tests {
		if t.ID == "" || seen[t.ID] || len(t.Attempts) > tr.MaxAttempts {
			return o, fmt.Errorf("ambiguous or oversized test identity")
		}
		seen[t.ID] = true
	}
	if len(o.Problems) != 0 {
		o.Complete = false
	}
	sort.Slice(o.Tests, func(i, j int) bool { return o.Tests[i].ID < o.Tests[j].ID })
	return o, nil
}

func problem(o *tr.Observation, code, detail string) {
	o.Problems = append(o.Problems, tr.Problem{Code: code, Detail: detail})
	o.Complete = false
}

func test(id, name, file, suite, state, kind, message string) tr.Test {
	return tr.Test{ID: id, Name: name, File: file, Suite: suite, State: state, FailureKind: kind, Message: message}
}
