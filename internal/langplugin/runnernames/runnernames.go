// SPDX-License-Identifier: Apache-2.0
// Package runnernames holds the runner and plugin names core selection code
// compares against. It is a leaf so plugin directories that reach it do not
// reach the core langplugin package (LGP-V0-009, LGP-V0-020).
package runnernames

// Runner and plugin names that core runner-selection plugins compare against.
// Core packages read them here instead of spelling the literal (LGP-V0-009).
const (
	RunnerBats               = "bats"
	RunnerCargoTest          = "cargo-test"
	RunnerDotnetMtpMstest    = "dotnet-mtp-mstest"
	RunnerDotnetMtpNunit     = "dotnet-mtp-nunit"
	RunnerDotnetMtpXunit     = "dotnet-mtp-xunit"
	RunnerDotnetVstestMstest = "dotnet-vstest-mstest"
	RunnerDotnetVstestNunit  = "dotnet-vstest-nunit"
	RunnerDotnetVstestXunit  = "dotnet-vstest-xunit"
	RunnerGradleJunit        = "gradle-junit"
	LanguageJava             = "java"
	RunnerJest               = "jest"
	RunnerJunitPlatform      = "junit-platform"
	RunnerMavenSurefire      = "maven-surefire"
	RunnerMinitest           = "minitest"
	RunnerNextest            = "nextest"
	RunnerNodeTest           = "node-test"
	RunnerPlaywright         = "playwright"
	RunnerPytest             = "pytest"
	RunnerRailsTest          = "rails-test"
	RunnerRspec              = "rspec"
	RunnerShellspec          = "shellspec"
	RunnerTestUnit           = "test-unit"
	RunnerUnittest           = "unittest"
	RunnerVitest             = "vitest"
)
