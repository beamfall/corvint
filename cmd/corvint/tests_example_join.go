package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/testsource"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

const exampleJoinUsage = "usage: corvint tests example-join --receipt FILE [--rev REV] [--select SELECTOR=REQ[,REQ...] ...] [--requirements FILE]"

// runTestsExampleJoin joins each example of one Ruby or JS provider receipt to its static
// test-validity verdict at the revision the receipt ran on, and echoes the caller-asserted
// requirement IDs on the selected examples (TSA-V0-410..419).
func runTestsExampleJoin(repo testsource.Repo, args []string, out, diagnostic io.Writer) int {
	f, once := testsFlags("tests example-join")
	options := testsource.ExampleJoinOptions{Revision: "HEAD"}
	var receipt, mapping string
	var selectErr error
	f.Func("select", "SELECTOR=REQ[,REQ...]", func(value string) error {
		selection, err := testsource.ParseRequirementSelection(value)
		if err != nil {
			if selectErr == nil {
				selectErr = err
			}
			return nil
		}
		options.Selections = append(options.Selections, selection)
		return nil
	})
	once("receipt", &receipt)
	once("rev", &options.Revision)
	once("requirements", &mapping)
	if err := f.Parse(args); err != nil || f.NArg() != 0 || receipt == "" {
		return testsRefuse(diagnostic, "tests-invalid-arguments", exampleJoinUsage)
	}
	if selectErr != nil {
		return testsFailure(diagnostic, selectErr)
	}
	if mapping != "" {
		text, err := readRequirementMap(mapping)
		if err != nil {
			return testsRefuse(diagnostic, "tests-source-unavailable", "cannot read --requirements: "+err.Error())
		}
		selections, err := testsource.ParseRequirementMap(text)
		if err != nil {
			return testsFailure(diagnostic, err)
		}
		options.Selections = append(options.Selections, selections...)
	}
	input, err := readProviderReceipt(receipt)
	if err != nil {
		return testsRefuse(diagnostic, "tests-source-unavailable", err.Error())
	}
	options.Ruby, _ = input.RubyDocument()
	options.JS, _ = input.JSReceipt()
	if options.Ruby == nil && options.JS == nil {
		return testsRefuse(diagnostic, "tests-invalid-arguments", "--receipt is not a Ruby or JS provider receipt")
	}
	report, err := testsource.ExampleJoin(repo, options)
	if err != nil {
		return testsFailure(diagnostic, err)
	}
	return testsWrite(out, diagnostic, report)
}

// readRequirementMap reads the bounded mapping file the caller named.
func readRequirementMap(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	parent, err := os.OpenRoot(filepath.Dir(resolved))
	if err != nil {
		return "", err
	}
	defer parent.Close()
	data, err := testvaliditydoc.ReadFile(parent, filepath.Base(resolved))
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errors.New("the file is empty")
	}
	return string(data), nil
}
