package platform

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// The script is fixed code. Paths and selectors arrive as JVM properties/argv,
// not interpolated Groovy source. Project configuration remains trusted code.
const gradleReportScript = `gradle.projectsEvaluated {
 def selected = gradle.rootProject.tasks.findByPath(System.getProperty('corvint.testTask'))
 if (!(selected instanceof org.gradle.api.tasks.testing.Test)) { throw new GradleException('selected task is not a native Test task') }
 selected.reports.junitXml.required = true
 selected.reports.junitXml.outputLocation.fileValue(new File(System.getProperty('corvint.reportsDir')))
 selected.reports.junitXml.outputPerTestCase = true
 selected.reports.junitXml.mergeReruns = false
 selected.ignoreFailures = false
 selected.filter.failOnNoMatchingTests = true
 selected.outputs.upToDateWhen { false }
}
`

// Gradle treats patterns beginning with an uppercase letter as simple-class
// selectors. Require a package prefix and a non-uppercase leading character.
var gradleSelector = regexp.MustCompile(`^[a-z_$][A-Za-z0-9_$]*(?:\.[A-Za-z_$][A-Za-z0-9_$]*)+#[A-Za-z_$][A-Za-z0-9_$]*$`)

var gradleTask = regexp.MustCompile(`^(:[A-Za-z_][A-Za-z0-9_-]*)+$`)

func buildJVMProject(r tr.Request, i *tr.Invocation) error {
	if !relative(r.Project) || !filepath.IsAbs(r.Config) || !digest.MatchString(r.ConfigSha256) {
		return fmt.Errorf("JVM project runner requires a literal project directory and pinned native configuration")
	}
	java, ok := r.Tools["java"]
	if !ok || !filepath.IsAbs(java.Executable) || !digest.MatchString(java.Sha256) || filepath.Base(java.Executable) != "java" {
		return fmt.Errorf("JVM build launchers require an independently pinned Java runtime")
	}
	i.Environment["JAVA_HOME"] = filepath.Dir(filepath.Dir(java.Executable))
	project := filepath.Join(r.Root, r.Project)
	switch r.Runner {
	case "gradle-junit":
		if r.Config != filepath.Join(project, "build.gradle") && r.Config != filepath.Join(project, "build.gradle.kts") {
			return fmt.Errorf("Gradle configuration must be the selected project's build file")
		}
		if !gradleTask.MatchString(r.Target) {
			return fmt.Errorf("Gradle requires one exact task path")
		}
		i.Files["corvint.init.gradle"] = []byte(gradleReportScript)
		i.Argv = []string{"--offline", "--no-daemon", "--console=plain", "--project-dir", project, "--init-script", filepath.Join(r.ReportDir, "corvint.init.gradle"), "-Dcorvint.testTask=" + r.Target, "-Dcorvint.reportsDir=" + r.ReportDir, r.Target}
		for _, s := range r.Selectors {
			if !gradleSelector.MatchString(s) {
				return fmt.Errorf("Gradle selectors require a fully package-qualified class#method with non-uppercase package prefix")
			}
			i.Argv = append(i.Argv, "--tests", strings.Replace(s, "#", ".", 1))
		}
	case "maven-surefire":
		if r.Config != filepath.Join(project, "pom.xml") || r.Target != "" {
			return fmt.Errorf("Maven requires the selected project's pom.xml and fixed test lifecycle")
		}
		// Surefire has no reportsDirectory CLI property. The pinned POM must wire
		// its reportsDirectory to ${corvint.reportsDirectory}; missing wiring yields
		// missing fresh reports and abstention, never import of old target reports.
		i.Argv = []string{"--offline", "--batch-mode", "--no-transfer-progress", "--file", r.Config, "-Dcorvint.reportsDirectory=" + r.ReportDir, "-DfailIfNoTests=true", "-Dsurefire.rerunFailingTestsCount=0", "test"}
		if r.Reporter != "" {
			if !filepath.IsAbs(r.Reporter) || !digest.MatchString(r.ReporterSha256) {
				return fmt.Errorf("Maven settings file must be independently pinned")
			}
			i.Argv = append(i.Argv, "--settings", r.Reporter)
		}
		var selectors []string
		for _, s := range r.Selectors {
			if !javaSelector.MatchString(s) {
				return fmt.Errorf("Surefire selectors require exact class#method")
			}
			selectors = append(selectors, s)
		}
		if len(selectors) > 0 {
			i.Argv = append(i.Argv, "-Dtest="+strings.Join(selectors, ","))
		}
	}
	i.ReportPatterns = []string{"TEST-*.xml"}
	return nil
}

// Gradle and Surefire reports are qualified separately; their failure taxonomy
// differs (Gradle emits exceptions as <failure>, Surefire as <error>).
func parseJVMProject(in tr.Input, o *tr.Observation) error {
	if len(in.Reports) == 0 {
		return fmt.Errorf("native JVM XML reports missing")
	}
	failed := false
	for name, b := range in.Reports {
		if filepath.Base(name) != name || !strings.HasPrefix(name, "TEST-") || !strings.HasSuffix(name, ".xml") {
			return fmt.Errorf("unexpected native JVM report path")
		}
		n, err := readXMLWithSchema(b, in.Runner == "maven-surefire")
		if err != nil {
			return err
		}
		if n.name != "testsuite" || n.attr["name"] == "" {
			return fmt.Errorf("invalid native JVM suite")
		}
		if in.Runner == "maven-surefire" && n.attr["version"] != "3.0.2" {
			return fmt.Errorf("unqualified Surefire report version")
		}
		wanted := map[string]int{}
		for _, key := range []string{"tests", "failures", "errors", "skipped"} {
			wanted[key], err = count(n, key)
			if err != nil {
				return err
			}
		}
		if flakes, ok := n.attr["flakes"]; ok && flakes != "0" {
			return fmt.Errorf("Surefire retry history requires a qualified retry profile")
		}
		actual := map[string]int{}
		for _, c := range n.children {
			switch c.name {
			case "properties", "system-out", "system-err":
				continue
			case "testcase":
			default:
				return fmt.Errorf("unknown JVM suite event")
			}
			if c.attr["status"] != "" || c.attr["result"] != "" {
				return fmt.Errorf("unqualified JVM testcase status attribute")
			}
			if c.attr["name"] == "" || c.attr["classname"] == "" {
				return fmt.Errorf("native JVM identity missing")
			}
			state, kind, message, terminal := tr.Passed, "", "", 0
			for _, x := range c.children {
				switch x.name {
				case "failure", "error", "skipped":
					terminal++
					message = x.attr["message"] + "\n" + strings.TrimSpace(x.text)
					switch x.name {
					case "skipped":
						state = tr.Skipped
						actual["skipped"]++
					case "error":
						state, kind = tr.Failed, tr.Infrastructure
						actual["errors"]++
						failed = true
					case "failure":
						state, kind = tr.Failed, tr.Unknown
						actual["failures"]++
						failed = true
						switch x.attr["type"] {
						case "java.lang.AssertionError", "junit.framework.AssertionFailedError", "org.opentest4j.AssertionFailedError", "org.opentest4j.MultipleFailuresError":
							kind = tr.Assertion
						}
					}
				case "system-out", "system-err":
				default:
					return fmt.Errorf("unknown JVM outcome/retry event %q", x.name)
				}
			}
			if terminal > 1 {
				return fmt.Errorf("contradictory native JVM outcome")
			}
			actual["tests"]++
			o.Tests = append(o.Tests, test(c.attr["classname"]+"#"+c.attr["name"], c.attr["name"], "", n.attr["name"], state, kind, strings.TrimSpace(message)))
		}
		for key, want := range wanted {
			if actual[key] != want {
				return fmt.Errorf("native JVM %s count mismatch", key)
			}
		}
	}
	if (failed && in.ExitCode != 1) || (!failed && in.ExitCode != 0) {
		problem(o, "EXIT_CONTRADICTION", "native JVM reports disagree with process exit")
	}
	o.Complete = len(o.Problems) == 0
	return nil
}
