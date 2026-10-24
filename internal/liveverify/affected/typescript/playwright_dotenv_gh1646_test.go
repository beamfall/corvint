package typescript

import (
	"slices"
	"strings"
	"testing"
)

func gh1646Resolve(t *testing.T, root, setup string, inputs ...PlaywrightEnvironmentInput) ([]PlaywrightProject, []PlaywrightEnvironmentInput, []PlaywrightEnvironmentFile) {
	t.Helper()
	config := gh1640Config(setup, "process.env.CHROME_PATH")
	projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", config)
	if len(unknown) != 0 || len(projects) != 2 {
		t.Fatalf("projects=%+v unknown=%+v", projects, unknown)
	}
	resolved, files := resolvePlaywrightExecutables(root, projects, inputs, "playwright.config.ts", config)
	return projects, resolved, files
}

// AFP-V0-405..407: a bare require('dotenv').config() with no .env leaves the recorded UNSET value
// standing and records the absence; the GitHub #1646 config no longer blocks browser identity.
func TestPlaywrightDotenvAbsentFileKeepsRecordedValue_GH1646(t *testing.T) {
	unset := NewPlaywrightEnvironmentInput("CHROME_PATH", PlaywrightEnvironmentInvocation, "", false)
	for _, setup := range []string{
		"require('dotenv').config();",
		"'use strict';\nimport dotenv from 'dotenv';\ndotenv.config();",
		"import * as dotenv from 'dotenv';\ndotenv.config({ quiet: true });",
		"const dotenv = require('dotenv');\ndotenv.config({ path: '.env', override: false });",
	} {
		t.Run(setup, func(t *testing.T) {
			projects, inputs, files := gh1646Resolve(t, t.TempDir(), setup, unset)
			for _, project := range projects {
				if len(project.execution) != 0 || project.Executable == nil || !project.Executable.Bundled {
					t.Fatalf("%s: execution=%+v executable=%+v", project.Name, project.execution, project.Executable)
				}
			}
			if len(files) != 1 || files[0].Path != ".env" || files[0].State != PlaywrightEnvironmentFileAbsent || files[0].SHA256 != "" || !strings.HasPrefix(files[0].Site, "playwright.config.ts:") {
				t.Fatalf("files=%+v", files)
			}
			if len(inputs) != 1 || inputs[0].Resolved == nil || inputs[0].Resolved.State != PlaywrightEnvironmentUnset || inputs[0].Resolved.Source != PlaywrightEnvironmentInvocation {
				t.Fatalf("inputs=%+v", inputs)
			}
		})
	}
}

// AFP-V0-406, AFP-V0-407: a present file is recorded with its digest; an UNSET variable takes the
// file's value (source DOTENV), and a variable already set is never overridden.
func TestPlaywrightDotenvPresentFileResolves_GH1646(t *testing.T) {
	root := t.TempDir()
	content := "# local browser\nCHROME_PATH=/opt/chrome/chrome\r\nOTHER='x'\n"
	write(t, root, ".env.local", content)
	setup := "require('dotenv').config({ path: './.env.local' });"
	unset := NewPlaywrightEnvironmentInput("CHROME_PATH", PlaywrightEnvironmentInvocation, "", false)
	projects, inputs, files := gh1646Resolve(t, root, setup, unset)
	if len(files) != 1 || files[0] != (PlaywrightEnvironmentFile{Path: ".env.local", SHA256: gh1640Digest(content), Site: "playwright.config.ts:2", State: PlaywrightEnvironmentFilePresent}) {
		t.Fatalf("files=%+v", files)
	}
	want := PlaywrightEnvironmentResolution{File: ".env.local", SHA256: gh1640Digest("/opt/chrome/chrome"), Source: PlaywrightEnvironmentDotenv, State: PlaywrightEnvironmentSet, Value: "/opt/chrome/chrome"}
	if inputs[0].Resolved == nil || *inputs[0].Resolved != want || inputs[0].State != PlaywrightEnvironmentUnset {
		t.Fatalf("inputs=%+v", inputs)
	}
	for _, project := range projects {
		if len(project.execution) != 0 || project.Executable == nil || project.Executable.Path != "/opt/chrome/chrome" {
			t.Fatalf("%s: execution=%+v executable=%+v", project.Name, project.execution, project.Executable)
		}
	}
	set := NewPlaywrightEnvironmentInput("CHROME_PATH", PlaywrightEnvironmentArgument, "/usr/bin/chromium", true)
	projects, inputs, _ = gh1646Resolve(t, root, setup, set)
	if inputs[0].Resolved == nil || inputs[0].Resolved.Source != PlaywrightEnvironmentArgument || inputs[0].Resolved.Value != "/usr/bin/chromium" {
		t.Fatalf("override: inputs=%+v", inputs)
	}
	if projects[0].Executable == nil || projects[0].Executable.Path != "/usr/bin/chromium" {
		t.Fatalf("override: executable=%+v", projects[0].Executable)
	}
}

// AFP-V0-408: a computed or overriding argument, a load after other code, a .vault file, an
// unmodelled line and dotenv/config stay a frontier whose detail names the call site or file.
func TestPlaywrightDotenvFrontierNamesSite_GH1646(t *testing.T) {
	unset := NewPlaywrightEnvironmentInput("CHROME_PATH", PlaywrightEnvironmentInvocation, "", false)
	for _, row := range []struct {
		label, setup, file, content, detail string
	}{
		{"computed path", "require('dotenv').config({ path: process.env.ENV_FILE });", "", "", "dotenv config() call at playwright.config.ts:2 has a computed or unmodelled argument"},
		{"override true", "require('dotenv').config({ override: true });", "", "", "playwright.config.ts:2 has a computed or unmodelled argument"},
		{"parent path", "require('dotenv').config({ path: '../.env' });", "", "", "playwright.config.ts:2 has a computed or unmodelled argument"},
		{"trailing slash path", "require('dotenv').config({ path: '.env/' });", ".env", "CHROME_PATH=/opt/chrome\n", "playwright.config.ts:2 has a computed or unmodelled argument"},
		{"uncleaned path", "require('dotenv').config({ path: 'missing/../.env' });", ".env", "CHROME_PATH=/opt/chrome\n", "playwright.config.ts:2 has a computed or unmodelled argument"},
		{"backslash value", "require('dotenv').config();", ".env", "CHROME_PATH=C:\\chrome\\chrome.exe\n", "reads .env, whose line 1 is not a NAME=VALUE form"},
		{"quoted backtick", "require('dotenv').config();", ".env", "CHROME_PATH='/opt/`x`'\n", "reads .env, whose line 1 is not a NAME=VALUE form"},
		{"after other code", "const mode = 'ci';\nrequire('dotenv').config();", "", "", "uses dotenv at playwright.config.ts:3 outside a top-level config() call"},
		{"named import", "import { config } from 'dotenv';\nconfig();", "", "", "imports dotenv at playwright.config.ts:2"},
		{"vault file", "require('dotenv').config();", ".env.vault", "DOTENV_VAULT=x\n", ".env.vault exists"},
		{"export line", "require('dotenv').config();", ".env", "A=1\nexport CHROME_PATH=/x\n", "reads .env, whose line 2 is not a NAME=VALUE form"},
		{"inline comment", "require('dotenv').config();", ".env", "CHROME_PATH=/x # local\n", "whose line 1"},
		{"dotenv/config", "import 'dotenv/config';", "", "", `the config loads "dotenv/config"`},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := t.TempDir()
			if row.file != "" {
				write(t, root, row.file, row.content)
			}
			projects, _, files := gh1646Resolve(t, root, row.setup, unset)
			if files != nil {
				t.Fatalf("frontier recorded files: %+v", files)
			}
			for _, project := range projects {
				if project.Executable != nil || len(project.execution) != 1 || project.execution[0].Reason != PlaywrightUnknownBrowserExecutable || !strings.Contains(project.execution[0].Detail, row.detail) {
					t.Fatalf("%s: execution=%+v executable=%+v", project.Name, project.execution, project.Executable)
				}
			}
		})
	}
}

// AFP-V0-406: the modelled line grammar is the one every dotenv release parses alike.
func TestPlaywrightDotenvLineGrammar_GH1646(t *testing.T) {
	values, line := parsePlaywrightDotenv("A=1\n\n  # note\nB = 'two words'\r\nC=\"q\"\rD=\nE= spaced value  \nA=last\n")
	want := map[string]string{"A": "last", "B": "two words", "C": "q", "D": "", "E": "spaced value"}
	if line != 0 || len(values) != len(want) {
		t.Fatalf("values=%+v line=%d", values, line)
	}
	for name, value := range want {
		if values[name] != value {
			t.Fatalf("%s=%q, want %q", name, values[name], value)
		}
	}
	for _, bad := range []string{"export A=1", "A: 1", "A='x' # c", "A='x' ", `A="a\nb"`, "A=`x`", "A='open", "A=x y", "A", `A=C:\chrome\chrome.exe`, "A=a`b", "A='a`b'", "A=\"a`b\""} {
		if _, line := parsePlaywrightDotenv(bad + "\n"); line != 1 {
			t.Fatalf("%q admitted", bad)
		}
	}
}

// AFP-V0-406: the static plan records the observed file beside the environment inputs.
func TestPlaywrightStaticPlanRecordsEnvironmentFile_GH1646(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0","dotenv":"16"}}`)
	write(t, root, "playwright.config.ts", gh1640Config("require('dotenv').config();", "process.env.CHROME_PATH"))
	write(t, root, "tests/a.spec.ts", "import { test } from '@playwright/test';\ntest('a', async () => {});\n")
	inputs := []PlaywrightEnvironmentInput{NewPlaywrightEnvironmentInput("CHROME_PATH", PlaywrightEnvironmentInvocation, "", false)}
	plan, _, err := selectPlaywrightStaticDraft(root, "playwright.config.ts", []string{"tests/a.spec.ts"}, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.EnvironmentFiles, []PlaywrightEnvironmentFile{{Path: ".env", Site: "playwright.config.ts:2", State: PlaywrightEnvironmentFileAbsent}}) {
		t.Fatalf("environmentFiles=%+v", plan.EnvironmentFiles)
	}
	if slices.ContainsFunc(plan.Unknown, func(unknown PlaywrightUnknown) bool { return unknown.Reason == PlaywrightUnknownBrowserExecutable }) {
		t.Fatalf("unknown=%+v", plan.Unknown)
	}
}
