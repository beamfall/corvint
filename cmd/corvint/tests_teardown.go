package main

import (
	"errors"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/testsource"
)

// runTestsTeardown prints the teardown dependents of the named committed test files under a
// committed side-effect declaration (TSA-V0-110..119). It exits 1 when a declared finding exists.
func runTestsTeardown(repo testsource.Repo, args []string, out, diagnostic io.Writer) int {
	f, once := testsFlags("tests teardown")
	options := testsource.TeardownOptions{Revision: "HEAD"}
	receipts := []string{}
	f.Func("files", "test file paths", func(value string) error {
		for _, name := range strings.Split(value, ",") {
			if strings.TrimSpace(name) != "" {
				options.Files = append(options.Files, name)
			}
		}
		return nil
	})
	f.Func("receipt", "provider receipt", func(value string) error {
		if value == "" {
			return errors.New("empty receipt")
		}
		receipts = append(receipts, value)
		return nil
	})
	once("declaration", &options.Declaration)
	once("rev", &options.Revision)
	if err := f.Parse(args); err != nil || f.NArg() != 0 || len(options.Files) == 0 || options.Declaration == "" {
		return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests teardown --files PATH[,PATH...] --declaration PATH [--receipt FILE ...] [--rev REV]")
	}
	if len(receipts) > testsource.MaxTeardownReceipts {
		return testsRefuse(diagnostic, "tests-bound-exceeded", "more than 16 receipts")
	}
	for _, path := range receipts {
		input, err := readProviderReceipt(path)
		if err != nil {
			return testsRefuse(diagnostic, "tests-source-unavailable", err.Error())
		}
		js, _ := input.JSReceipt()
		options.Receipts = append(options.Receipts, testsource.TeardownReceipt{Name: path, JS: js})
	}
	report, err := testsource.TeardownDependents(repo, options)
	if err != nil {
		return testsFailure(diagnostic, err)
	}
	if code := testsWrite(out, diagnostic, report); code != 0 {
		return code
	}
	if report.Declared() > 0 {
		return 1
	}
	return 0
}
