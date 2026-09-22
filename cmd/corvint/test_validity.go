package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// testValidityDocument is the corvint-test-validity/0 document (LPCV-V0-051),
// built by internal/testvaliditydoc so the MCP test-validity profile emits
// the same bytes.
type testValidityDocument = testvaliditydoc.Document

// parseTestValidityInvocation intercepts `test-validity` after any --root
// prefix. The root is read only by --discover (LPCV-V0-053).
func parseTestValidityInvocation(arguments []string) (string, []string, bool) {
	index, root := 0, ""
	for index < len(arguments) && (arguments[index] == "--root" && rootPreambleValue(arguments, index+1) || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			root, index = arguments[index+1], index+2
			continue
		}
		root, index = strings.TrimPrefix(arguments[index], "--root="), index+1
	}
	if index >= len(arguments) || arguments[index] != "test-validity" {
		return "", nil, false
	}
	return root, arguments[index+1:], true
}

type testValidityOptions struct {
	receipt  string
	discover bool
}

func parseTestValidityOptions(arguments []string) (testValidityOptions, error) {
	options := testValidityOptions{}
	for index := 0; index < len(arguments); index++ {
		name, value, inline := splitWitnessFlag(arguments[index])
		if name == "--discover" {
			if inline || options.discover {
				return testValidityOptions{}, argumentError("--discover takes no value and may be given once")
			}
			options.discover = true
			continue
		}
		if name != "--receipt" {
			return testValidityOptions{}, argumentError("unrecognized test-validity option: " + name)
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return testValidityOptions{}, argumentError("missing value for --receipt")
			}
			index++
			value = arguments[index]
		}
		if value == "" {
			return testValidityOptions{}, argumentError("--receipt requires a non-empty FILE")
		}
		if options.receipt != "" {
			return testValidityOptions{}, argumentError("--receipt may be given once")
		}
		options.receipt = value
	}
	if options.discover && options.receipt != "" {
		return testValidityOptions{}, argumentError("--discover and --receipt are mutually exclusive")
	}
	return options, nil
}

// runTestValidity prints one corvint-test-validity/0 document. Without
// --receipt or --discover there is no test-level evidence, so the run
// projection states every axis UNSUPPORTED (LPCV-V0-049) instead of implying
// a pass.
func runTestValidity(root string, arguments []string, stdout, stderr io.Writer) int {
	options, err := parseTestValidityOptions(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	document := testvaliditydoc.Unsupported()
	if options.receipt != "" {
		receipt, readErr := readProviderReceipt(options.receipt)
		if readErr != nil {
			emitError(stderr, readErr)
			return 2
		}
		document = testvaliditydoc.Project(receipt)
	}
	if options.discover {
		discovered, discoverErr := discoverTestValidity(root)
		if discoverErr != nil {
			emitError(stderr, discoverErr)
			return 2
		}
		document = discovered
	}
	if err := emit(stdout, document); err != nil {
		emitError(stderr, err)
		return 2
	}
	return 0
}

// discoverTestValidity resolves the worktree root (default: the working
// directory) and runs the shared retained-evidence discovery (LPCV-V0-053).
func discoverTestValidity(root string) (testValidityDocument, error) {
	if root == "" {
		root = "."
	}
	absolute, err := filepath.Abs(root)
	if err == nil {
		absolute, err = filepath.EvalSymlinks(absolute)
	}
	if err != nil {
		return testValidityDocument{}, &gokernel.Error{Code: "invalid-test-evidence-location", Message: "worktree root cannot be resolved"}
	}
	document, err := testvaliditydoc.Discover(absolute)
	var refusal *testvaliditydoc.DiscoveryError
	if errors.As(err, &refusal) {
		return testValidityDocument{}, &gokernel.Error{Code: refusal.Code, Message: refusal.Message}
	}
	return document, err
}

// readProviderReceipt decodes one bounded, closed provider document. Any
// unreadable, oversized, unknown-field, trailing-data, or receipt-less input
// is refused, never projected.
func readProviderReceipt(path string) (testvaliditydoc.Input, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return testvaliditydoc.Input{}, invalidReceipt("cannot resolve --receipt: " + err.Error())
	}
	parent, err := os.OpenRoot(filepath.Dir(resolved))
	if err != nil {
		return testvaliditydoc.Input{}, invalidReceipt("cannot open --receipt parent: " + err.Error())
	}
	defer parent.Close()
	data, err := testvaliditydoc.ReadFile(parent, filepath.Base(resolved))
	if err != nil {
		return testvaliditydoc.Input{}, invalidReceipt("cannot read --receipt: " + err.Error())
	}
	receipt, err := testvaliditydoc.Decode(data)
	if err != nil {
		return testvaliditydoc.Input{}, invalidReceipt("--receipt " + err.Error())
	}
	return receipt, nil
}

func invalidReceipt(message string) error {
	return &gokernel.Error{Code: "invalid-test-validity-receipt", Message: message}
}
