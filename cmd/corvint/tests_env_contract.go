package main

import (
	"io"

	"github.com/Beamfall/corvint/internal/testsource"
)

// runTestsRoleLocators checks notification role locators against mounted component markup
// (TSA-V0-206). Exit 1 when a finding is reported.
func runTestsRoleLocators(repo testsource.Repo, args []string, out, diagnostic io.Writer) int {
	f, once := testsFlags("tests role-locators")
	options := testsource.RoleLocatorOptions{Revision: "HEAD"}
	once("rev", &options.Revision)
	if err := f.Parse(args); err != nil || f.NArg() != 0 {
		return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests role-locators [--rev REV]")
	}
	report, err := testsource.RoleLocators(repo, options)
	if err != nil {
		return testsFailure(diagnostic, err)
	}
	if code := testsWrite(out, diagnostic, report); code != 0 {
		return code
	}
	if len(report.Findings) > 0 {
		return 1
	}
	return 0
}

// runTestsEnv reads one environment profile's contract and, with --base, its impact
// (TSA-V0-210..216). Warnings are advisory and never change the exit status.
func runTestsEnv(repo testsource.Repo, args []string, out, diagnostic io.Writer) int {
	f, once := testsFlags("tests env")
	options := testsource.EnvContractOptions{Revision: "HEAD"}
	once("profile", &options.Profile)
	once("rev", &options.Revision)
	once("base", &options.Base)
	if err := f.Parse(args); err != nil || f.NArg() != 0 || !testsource.ValidProfileName(options.Profile) {
		return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests env --profile NAME [--rev REV] [--base REV]")
	}
	report, err := testsource.EnvContract(repo, options)
	if err != nil {
		return testsFailure(diagnostic, err)
	}
	return testsWrite(out, diagnostic, report)
}
