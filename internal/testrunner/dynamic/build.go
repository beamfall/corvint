// Package dynamic implements explicit experimental dynamic-language runner profiles.
// A supported native report is not evidence of runtime or device qualification.
package dynamic

import (
	"embed"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

//go:embed reporters/*
var reporters embed.FS

func Runners() []string {
	return []string{"mocha", "node-test", "jest", "vitest", "ava", "bun-test", "deno-test", "playwright", "cypress", "webdriverio", "testcafe", "nightwatch", "detox", "storybook-test-runner", "storybook-vitest", "pytest", "unittest", "rspec", "minitest", "test-unit", "rails-test"}
}

func known(r string) bool {
	for _, x := range Runners() {
		if x == r {
			return true
		}
	}
	return false
}

// Build never discovers tests, installs packages, or executes configuration.
// Executable and configuration admission belongs to the common executor.
func Build(r tr.Request) (tr.Invocation, error) {
	if !known(r.Runner) {
		return tr.Invocation{}, fmt.Errorf("unknown dynamic runner %q", r.Runner)
	}
	if !filepath.IsAbs(r.Executable) || !filepath.IsAbs(r.ReportDir) {
		return tr.Invocation{}, fmt.Errorf("absolute executable and fresh report directory required")
	}
	for _, s := range r.Selectors {
		if s == "" || strings.HasPrefix(s, "-") || strings.ContainsAny(s, "\x00\r\n*?[]{}!,") || s == ".." || strings.HasPrefix(s, "../") {
			return tr.Invocation{}, fmt.Errorf("unsafe or ambiguous selector %q", s)
		}
	}
	for _, s := range []string{r.Config, r.Project} {
		if strings.ContainsAny(s, "\x00\r\n") || strings.HasPrefix(s, "-") {
			return tr.Invocation{}, fmt.Errorf("invalid configuration/project")
		}
	}
	v := tr.Invocation{SuccessExitCodes: []int{0}, FailureExitCodes: []int{1}, Environment: map[string]string{}, Format: r.Runner, Files: map[string][]byte{}}
	add := func(args ...string) { v.Argv = append(v.Argv, args...) }
	report := func(name string) string {
		v.ReportPaths = append(v.ReportPaths, name)
		return filepath.Join(r.ReportDir, name)
	}
	plugin := func(name string) string {
		b, _ := reporters.ReadFile("reporters/" + name)
		v.Files[name] = b
		return filepath.Join(r.ReportDir, name)
	}
	config := func(flag string) {
		if r.Config != "" {
			add(flag, r.Config)
		}
	}
	switch r.Runner {
	case "node-test":
		add("--test", "--test-reporter="+plugin("node.mjs"), "--test-reporter-destination="+report("node.jsonl"))
		add(r.Selectors...)
	case "jest":
		add("--json", "--outputFile="+report("jest.json"), "--runInBand")
		config("--config")
		if len(r.Selectors) > 0 {
			add("--runTestsByPath")
			add(r.Selectors...)
		}
	case "vitest", "storybook-vitest":
		if len(r.Selectors) > 0 {
			return tr.Invocation{}, fmt.Errorf("Vitest substring filters do not guarantee exact file selection; use pinned config")
		}
		add("run", "--reporter=json", "--outputFile="+report("vitest.json"))
		config("--config")
		add(r.Selectors...)
	case "ava":
		add("--tap")
		config("--config")
		add(r.Selectors...)
	case "bun-test":
		r.Selectors = append([]string{}, r.Selectors...)
		for i, sel := range r.Selectors {
			if !filepath.IsAbs(sel) {
				r.Selectors[i] = "./" + sel
			}
		}
		add("test", "--reporter=junit", "--reporter-outfile="+report("bun.xml"))
		add(r.Selectors...)
	case "deno-test":
		add("test", "--junit-path="+report("deno.xml"))
		config("--config")
		add(r.Selectors...)
	case "playwright":
		v.GracefulInterrupt = true
		add("test", "--reporter=json")
		v.Environment["PLAYWRIGHT_JSON_OUTPUT_FILE"] = report("playwright.json")
		config("--config")
		if r.Project != "" {
			add("--project=" + r.Project)
		}
		for _, sel := range r.Selectors {
			if !filepath.IsAbs(sel) {
				sel = filepath.Join(r.Root, sel)
			}
			add("^" + regexp.QuoteMeta(sel) + "$")
		}
	case "pytest":
		add("-m", "pytest", "--junitxml="+report("pytest.xml"))
		config("-c")
		add(r.Selectors...)
	case "unittest":
		add(plugin("unittest_reporter.py"), report("unittest.json"))
		add(r.Selectors...)
	case "rspec":
		add("--format", "json", "--out", report("rspec.json"))
		if r.Config != "" {
			add("--options", r.Config)
		}
		add(r.Selectors...)
	case "minitest", "rails-test":
		if len(r.Selectors) == 0 {
			return tr.Invocation{}, fmt.Errorf("explicit test files required for %s; no implicit Ruby loader", r.Runner)
		}
		add(plugin("minitest_reporter.rb"), report("minitest.json"))
		if r.Config != "" {
			add("--require", r.Config)
		}
		add("--")
		add(r.Selectors...)
	case "test-unit":
		if len(r.Selectors) == 0 {
			return tr.Invocation{}, fmt.Errorf("explicit Test::Unit test files required")
		}
		add(plugin("testunit_reporter.rb"), report("testunit.json"))
		add(r.Selectors...)
	case "testcafe":
		v.FailureExitCodes = nil
		for n := 1; n < 256; n++ {
			v.FailureExitCodes = append(v.FailureExitCodes, n)
		}
		if r.Project == "" {
			return tr.Invocation{}, fmt.Errorf("explicit TestCafe browser required in Project")
		}
		add(r.Project)
		add(r.Selectors...)
		add("--reporter", "json:"+report("testcafe.json"))
		config("--config-file")
	case "mocha", "cypress":
		if strings.ContainsAny(r.ReportDir, ",=") {
			return tr.Invocation{}, fmt.Errorf("reporter output directory cannot contain comma or equals")
		}
		if r.Runner == "mocha" {
			add("--posix-exit-codes", "--reporter", plugin("mocha.cjs"), "--reporter-options", "outputDir="+r.ReportDir)
			config("--config")
			add(r.Selectors...)
			v.ReportPatterns = []string{"mocha-*.json"}
			break
		}
		add("run", "--posix-exit-codes", "--reporter", plugin("cypress.cjs"), "--reporter-options", "outputDir="+r.ReportDir)
		if len(r.Selectors) > 0 {
			add("--spec", strings.Join(r.Selectors, ","))
		}
		config("--config-file")
		if r.Project != "" {
			add("--browser", r.Project)
		}
		v.ReportPatterns = []string{"cypress-*.json"}
	case "webdriverio":
		if r.Config == "" || len(r.ReportFiles) == 0 {
			return tr.Invocation{}, fmt.Errorf("WebdriverIO needs pinned JSON reporter configuration and explicit report shard manifest")
		}
		add("run", r.Config)
		for _, s := range r.Selectors {
			add("--spec", s)
		}
		v.ReportPaths = append(v.ReportPaths, r.ReportFiles...)
	case "nightwatch":
		v.FailureExitCodes = []int{5}
		add("--reporter=json", "--output", r.ReportDir)
		config("--config")
		if r.Project != "" {
			add("--env", r.Project)
		}
		add(r.Selectors...)
		if len(r.ReportFiles) == 0 {
			return tr.Invocation{}, fmt.Errorf("Nightwatch requires explicit native report shard manifest")
		}
		v.ReportPaths = append(v.ReportPaths, r.ReportFiles...)
	case "detox":
		if r.Project == "" {
			return tr.Invocation{}, fmt.Errorf("explicit Detox device configuration required")
		}
		add("test", "--configuration", r.Project, "--retries", "0", "--", "--json", "--outputFile="+report("detox.json"))
		add(r.Selectors...)
	case "storybook-test-runner":
		if len(r.Selectors) > 0 {
			return tr.Invocation{}, fmt.Errorf("legacy Storybook story selection is not a file selector; use explicit full story-set invocation")
		}
		for _, arg := range []string{r.ReportDir, r.Project, r.Config} {
			if !regexp.MustCompile(`^[A-Za-z0-9_./:-]*$`).MatchString(arg) {
				return tr.Invocation{}, fmt.Errorf("legacy Storybook native shell forwarding requires simple paths/URL")
			}
		}
		add("--json", "--outputFile", report("storybook.json"))
		if r.Config != "" {
			return tr.Invocation{}, fmt.Errorf("legacy Storybook discovers configuration; pin config via InputFiles")
		}
		if r.Project != "" {
			add("--url", r.Project)
		}
	}
	for _, p := range v.ReportPaths {
		if filepath.IsAbs(p) || filepath.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") {
			return tr.Invocation{}, fmt.Errorf("unsafe report manifest path %q", p)
		}
	}
	return v, nil
}
