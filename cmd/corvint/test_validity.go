package main

import (
	"cmp"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/testsource"
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
	receipt    string
	discover   bool
	routeStubs bool
	// static and paths select the static source projection (LPCV-V0-150).
	static   bool
	paths    []string
	producer bool
}

// testValidityWithProducer is the opt-in --producer receipt: the unchanged
// document members followed by one additive producer member (PUB-V0-027).
type testValidityWithProducer struct {
	testvaliditydoc.Document
	Producer producerIdentity `json:"producer"`
}

func parseTestValidityOptions(arguments []string) (testValidityOptions, error) {
	options := testValidityOptions{}
	for index := 0; index < len(arguments); index++ {
		if !strings.HasPrefix(arguments[index], "-") {
			options.paths = append(options.paths, arguments[index])
			continue
		}
		name, value, inline := splitWitnessFlag(arguments[index])
		if name == "--static" {
			if inline || options.static {
				return testValidityOptions{}, argumentError("--static takes no value and may be given once")
			}
			options.static = true
			continue
		}
		if name == "--discover" {
			if inline || options.discover {
				return testValidityOptions{}, argumentError("--discover takes no value and may be given once")
			}
			options.discover = true
			continue
		}
		if name == "--route-stubs" {
			if inline || options.routeStubs {
				return testValidityOptions{}, argumentError("--route-stubs takes no value and may be given once")
			}
			options.routeStubs = true
			continue
		}
		if name == "--producer" {
			if inline || options.producer {
				return testValidityOptions{}, argumentError("--producer takes no value and may be given once")
			}
			options.producer = true
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
	if options.routeStubs && options.receipt == "" {
		return testValidityOptions{}, argumentError("--route-stubs requires --receipt")
	}
	if options.static && (options.discover || options.receipt != "") {
		return testValidityOptions{}, argumentError("--static excludes --receipt and --discover")
	}
	if len(options.paths) > 0 && !options.static {
		return testValidityOptions{}, argumentError("a PATH operand requires --static")
	}
	return options, nil
}

// runTestValidity prints one corvint-test-validity/0 document. Without
// --receipt or --discover there is no test-level evidence, so the run
// projection states every axis UNSUPPORTED (LPCV-V0-049) instead of implying
// a pass.
func runTestValidity(root string, arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) > 0 && arguments[0] == "negate" {
		return runTestValidityNegate(root, arguments[1:], stdout, stderr)
	}
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
		project := testvaliditydoc.Project
		if options.routeStubs {
			project = testvaliditydoc.ProjectRouteStubs
		}
		document = testvaliditydoc.JoinStepNegation(project(receipt), receipt, root)
		// Each declaring anchor's file is made relative to the root or working directory (LPCV-V0-145).
		document = testvaliditydoc.Locate(document, receipt, cmp.Or(root, "."))
	}
	if options.static {
		static, staticErr := staticTestValidity(root, options.paths)
		if staticErr != nil {
			emitError(stderr, staticErr)
			return 2
		}
		document = static
	}
	if options.discover {
		discovered, discoverErr := discoverTestValidity(root)
		if discoverErr != nil {
			emitError(stderr, discoverErr)
			return 2
		}
		document = discovered
	}
	var output any = document
	if options.producer {
		output = testValidityWithProducer{Document: document, Producer: currentProducer()}
	}
	if err := emit(stdout, output); err != nil {
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
	document, err := testvaliditydoc.DiscoverWithReaders(absolute, testsource.ExampleSources, testvaliditydoc.RubyReaders{Examples: testsource.RSpecExampleSourcesWithSupport, Inclusion: testsource.RSpecInclusionSource})
	var refusal *testvaliditydoc.DiscoveryError
	if errors.As(err, &refusal) {
		return testValidityDocument{}, &gokernel.Error{Code: refusal.Code, Message: refusal.Message}
	}
	return document, err
}

// staticTestValidity projects the committed HEAD test source under paths (default: the whole
// tree) without a retained run (LPCV-V0-150..152). It reads Git objects only and writes nothing.
func staticTestValidity(root string, paths []string) (testValidityDocument, error) {
	resolved, err := resolveExplicitRoot(cmp.Or(root, "."))
	if err != nil {
		return testValidityDocument{}, err
	}
	revision, read, err := testsource.StaticExamples(testsource.OpenRepo(context.Background(), resolved), "HEAD", paths)
	var refusal *testsource.Error
	if errors.As(err, &refusal) {
		return testValidityDocument{}, &gokernel.Error{Code: refusal.Code, Message: refusal.Message}
	}
	if err != nil {
		return testValidityDocument{}, err
	}
	files := make([]testvaliditydoc.StaticFile, 0, len(read))
	for _, file := range read {
		files = append(files, testvaliditydoc.StaticFile{Path: file.Path, Examples: file.Examples, Reason: file.Reason})
	}
	return testvaliditydoc.ProjectStatic(revision, files), nil
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
	data, err := readReceiptFile(parent, filepath.Base(resolved))
	if errors.Is(err, testvaliditydoc.ErrPlatformUnsupported) {
		return testvaliditydoc.Input{}, receiptPlatformRefusal()
	}
	if err != nil {
		return testvaliditydoc.Input{}, invalidReceipt("cannot read --receipt: " + err.Error())
	}
	receipt, err := testvaliditydoc.Decode(data)
	if err != nil {
		return testvaliditydoc.Input{}, invalidReceipt("--receipt " + err.Error())
	}
	return receipt, nil
}

// readReceiptFile is testvaliditydoc.ReadFile; tests force its platform
// refusal (PRS-V1-920).
var readReceiptFile = testvaliditydoc.ReadFile

// receiptPlatformRefusal is the typed receipt platform refusal (PRS-V1-920).
func receiptPlatformRefusal() *gokernel.Error {
	return &gokernel.Error{Code: testvaliditydoc.PlatformUnsupportedCode, Message: testvaliditydoc.ErrPlatformUnsupported.Error()}
}

func invalidReceipt(message string) error {
	return &gokernel.Error{Code: "invalid-test-validity-receipt", Message: message}
}
