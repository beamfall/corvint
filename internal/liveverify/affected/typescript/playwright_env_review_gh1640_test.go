package typescript

import (
	"strings"
	"testing"
)

// AFP-V0-384: a key or fallback literal with an escape sequence is not modelled, because Go and
// JavaScript decode escapes differently (`'\U00000041'` is "U00000041" in JavaScript and `'\a'` is
// "a"), so the EXECUTION unknown stays. The pure scope already refuses the first and third forms
// as configs; the parser itself must not decode them either.
func TestPlaywrightExecutableEscapedLiteralKeepsUnknown_GH1640(t *testing.T) {
	for _, expression := range []string{`process.env['\U00000041']`, `process.env["\u0041"]`, `process.env.N\u0041ME`, `process.env.CHROME ?? '/opt/\a/chrome'`} {
		if name, _, fallback, ok := playwrightEnvExecutable(expression); ok {
			t.Fatalf("%s is modelled as name=%q fallback=%q", expression, name, fallback)
		}
	}
	input := NewPlaywrightEnvironmentInput("A", PlaywrightEnvironmentArgument, "/opt/a", true)
	for _, expression := range []string{`process.env['\U00000041']`, `process.env["\u0041"]`} {
		config := gh1640Config("", expression)
		projects, _, _ := parsePlaywrightConfig("playwright.config.ts", config)
		resolvePlaywrightExecutables(t.TempDir(), projects, []PlaywrightEnvironmentInput{input}, "playwright.config.ts", config)
		for _, project := range projects {
			if project.Executable != nil {
				t.Fatalf("%s: %s resolved to %+v", expression, project.Name, project.Executable)
			}
		}
	}
}

// AFP-V0-386: a Node module loader, however it is reached, and process.loadEnvFile can set the
// variable before the config reads it, so the EXECUTION unknown stays and names the writer.
func TestPlaywrightExecutableLoaderWriterKeepsUnknown_GH1640(t *testing.T) {
	unset := NewPlaywrightEnvironmentInput("CHROME", PlaywrightEnvironmentInvocation, "", false)
	for _, row := range []struct{ label, setup, detail string }{
		{"createRequire alias", "import { createRequire } from 'node:module';\nconst load = createRequire(import.meta.url);\nload('dotenv/config');", "node:module"},
		{"bare module import", "import { createRequire } from 'module';\ncreateRequire(import.meta.url)('dotenv/config');", `"module"`},
		{"aliased require", "const r = require;\nr('dotenv/config');", "require"},
		{"module.require", "module.require('dotenv/config');", "require"},
		{"loadEnvFile", "process.loadEnvFile();", "loadEnvFile"},
	} {
		t.Run(row.label, func(t *testing.T) {
			config := gh1640Config(row.setup, "process.env.CHROME")
			projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", config)
			if len(unknown) != 0 {
				t.Logf("config refused before resolution: %+v", unknown)
				return
			}
			resolvePlaywrightExecutables(t.TempDir(), projects, []PlaywrightEnvironmentInput{unset}, "playwright.config.ts", config)
			for _, project := range projects {
				if project.Executable != nil || len(project.execution) != 1 || !strings.Contains(project.execution[0].Detail, row.detail) {
					t.Fatalf("%s: execution=%+v executable=%+v want detail %q", project.Name, project.execution, project.Executable, row.detail)
				}
			}
		})
	}
	neutral := gh1640Config("import path from 'node:path';\nimport os from 'os';", "process.env.CHROME")
	for _, project := range gh1640Resolve(t, neutral, unset) {
		if project.Executable == nil || !project.Executable.Bundled {
			t.Fatalf("%s: neutral built-ins must still resolve: execution=%+v executable=%+v", project.Name, project.execution, project.Executable)
		}
	}
}

// AFP-V0-385: undefined launches the bundled browser only when no channel is set; with a channel
// the installed browser decides the binary, so the EXECUTION unknown stays and says why.
func TestPlaywrightExecutableUndefinedWithChannelKeepsUnknown_GH1640(t *testing.T) {
	unset := NewPlaywrightEnvironmentInput("CHROME", PlaywrightEnvironmentInvocation, "", false)
	config := "import { defineConfig } from '@playwright/test';\nexport default defineConfig({\n  use: { launchOptions: { executablePath: process.env.CHROME } },\n  projects: [\n    { name: 'chrome', use: { channel: 'chrome' } },\n    { name: 'chromium', use: { browserName: 'chromium' } },\n  ],\n});\n"
	for _, project := range gh1640Resolve(t, config, unset) {
		if project.Executable != nil || len(project.execution) != 1 || !strings.Contains(project.execution[0].Detail, "channel decides the browser binary") {
			t.Fatalf("%s: execution=%+v executable=%+v", project.Name, project.execution, project.Executable)
		}
	}
	set := NewPlaywrightEnvironmentInput("CHROME", PlaywrightEnvironmentArgument, "/opt/chrome", true)
	for _, project := range gh1640Resolve(t, config, set) {
		if project.Executable == nil || project.Executable.Path != "/opt/chrome" {
			t.Fatalf("%s: a defined path overrides the channel: execution=%+v executable=%+v", project.Name, project.execution, project.Executable)
		}
	}
}
