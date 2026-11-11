package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/cliapp"
	"io"
	"math/big"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/observations"
)

// PUB-V0-001 keeps this literal synchronized with VERSION.
const version = "1.0.0-rc.3"
const maximumImpactLimit = 50
const defaultHarnessBudgetBytes = 8_000

type options struct {
	command         string
	helpTopic       string
	root            string
	impactPaths     []string
	impactProviders []string
	impactCheckouts []extevidence.Checkout
	impactLimit     int
	impactBudget    *int
	requireClean    bool
	// impactCapabilities and impactProducer are the GPK-V0-084 listing and
	// the opt-in PUB-V0-027 producer member.
	impactCapabilities bool
	impactProducer     bool
	impactBase         string
	impactBaseSet      bool
	impactProfile      string
	impactLanguage     string
	featureID          string
	featureIDSet       bool
	featureLimit       int
	featureBudget      *int
	version            string
}

type readResult struct {
	value []byte
	err   error
}

func argumentError(message string) error {
	return &gokernel.Error{Code: "invalid-arguments", Message: message}
}

func parse(arguments []string) (options, error) {
	result := options{impactLimit: defaultImpactLimit, featureLimit: 10}
	if topic, requested, err := parseHelpInvocation(arguments); requested || err != nil {
		result.helpTopic = topic
		return result, err
	}
	if isVersionRequest(arguments) {
		result.version = arguments[len(arguments)-1]
		return result, nil
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return result, argumentError("cannot resolve current directory")
	}
	result.root = workingDirectory
	index := 0
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		value := ""
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return result, argumentError("missing value for --root")
			}
			value = arguments[index+1]
			index += 2
		} else {
			value = strings.TrimPrefix(arguments[index], "--root=")
			index++
		}
		root, err := resolveExplicitRoot(value)
		if err != nil {
			return result, err
		}
		result.root = root
	}
	if isVersionRequest(arguments[index:]) {
		result.version = arguments[len(arguments)-1]
		return result, nil
	}
	if index < len(arguments) && arguments[index] == "impact" {
		result.command = "impact"
		// The capabilities listing reads no repository, so it needs no Git
		// root (V1-2067); every other impact keeps the root check first.
		parsed, parseErr := parseImpactArguments(result, arguments[index+1:])
		if parseErr == nil && parsed.impactCapabilities {
			return parsed, nil
		}
		if err := requireRepositoryRoot(result.root); err != nil {
			return result, err
		}
		return parsed, parseErr
	}
	if index < len(arguments) && arguments[index] == "feature" {
		result.command = "feature"
		return parseFeatureArguments(result, arguments[index+1:])
	}
	if index >= len(arguments) {
		return result, argumentError("the following arguments are required: command")
	}
	if arguments[index] == "version" {
		// PRS-V1-910: agents type `corvint version` by analogy with corvint-tasks.
		return result, argumentError(invalidChoice("command", arguments[index], topLevelCommands).Error() + "; to print the version, run corvint --version (or corvint --version --json)")
	}
	return result, argumentError(invalidChoice("command", arguments[index], topLevelCommands).Error() + didYouMean(arguments[index], topLevelCommands))
}

func parseImpactArguments(result options, arguments []string) (options, error) {
	return parseImpactArgumentsWithLayout(result, arguments, runtime.GOOS)
}

func parseImpactArgumentsForPlatform(result options, arguments []string, platform string) (options, error) {
	if !nativePlatformQualified(platform) {
		return result, impactPlatformRefusal(platform)
	}
	positionalOnly := false
	for index := 0; index < len(arguments); {
		argument := arguments[index]
		if !positionalOnly && argument == "--" {
			positionalOnly = true
			index++
			continue
		}
		name, value, inline := strings.Cut(argument, "=")
		if !positionalOnly && name == "--budget-bytes" {
			// MCPV0-032 (proposed): path impact honours a byte budget.
			if result.impactBudget != nil {
				return result, argumentError("argument --budget-bytes: may not be repeated")
			}
			if !inline {
				if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
					return result, argumentError("argument --budget-bytes: expected one argument")
				}
				value = arguments[index+1]
				index += 2
			} else {
				index++
			}
			budget, ok := pythonBoundedInteger(value, contextindex.MaxPacketBytes)
			if !ok {
				return result, argumentError("argument --budget-bytes: invalid literal for int() with base 10: " + pythonRepr(value))
			}
			if budget < contextindex.MinPacketBytes || budget > contextindex.MaxPacketBytes {
				return result, argumentError(fmt.Sprintf(
					"argument --budget-bytes: budget_bytes must be between %d and %d",
					contextindex.MinPacketBytes, contextindex.MaxPacketBytes))
			}
			result.impactBudget = &budget
			continue
		}
		if !positionalOnly && name == "--capabilities" {
			// GPK-V0-084: the listing is a separate mode that reads no repository.
			if inline || len(arguments) != 1 {
				return result, argumentError("argument --capabilities: accepts no value and no other impact argument")
			}
			result.impactCapabilities = true
			index++
			continue
		}
		if !positionalOnly && name == "--producer" {
			if inline || result.impactProducer {
				return result, argumentError("argument --producer: does not accept a value or repetition")
			}
			result.impactProducer = true
			index++
			continue
		}
		if !positionalOnly && name == "--require-clean" {
			if inline || result.requireClean {
				return result, argumentError("argument --require-clean: does not accept a value or repetition")
			}
			result.requireClean = true
			index++
			continue
		}
		if !positionalOnly && name == "--base" {
			if result.impactBaseSet {
				return result, argumentError("argument --base: may not be repeated")
			}
			if !inline {
				if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
					return result, argumentError("argument --base: expected one argument")
				}
				value = arguments[index+1]
				index += 2
			} else {
				index++
			}
			result.impactBase, result.impactBaseSet = value, true
			continue
		}
		if !positionalOnly && name == "--language-profile" {
			if result.impactLanguage != "" {
				return result, argumentError("argument --language-profile: may not be repeated")
			}
			if !inline {
				if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
					return result, argumentError("argument --language-profile: expected one argument")
				}
				value = arguments[index+1]
				index += 2
			} else {
				index++
			}
			if value != "non-go-syntax-v0" {
				return result, argumentError("argument --language-profile: expected non-go-syntax-v0")
			}
			result.impactLanguage = value
			continue
		}
		if !positionalOnly && name == "--range-profile" {
			if result.impactProfile != "" {
				return result, argumentError("argument --range-profile: may not be repeated")
			}
			if !inline {
				if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
					return result, argumentError("argument --range-profile: expected one argument")
				}
				value = arguments[index+1]
				index += 2
			} else {
				index++
			}
			if value != "expanded-256" {
				return result, argumentError("argument --range-profile: expected expanded-256")
			}
			result.impactProfile = value
			continue
		}
		if !positionalOnly && name == "--limit" {
			if !inline {
				if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
					return result, argumentError("argument --limit: expected one argument")
				}
				value = arguments[index+1]
				index += 2
			} else {
				index++
			}
			limit, ok := pythonInteger(value)
			if !ok {
				return result, argumentError("argument --limit: invalid int value: " + pythonRepr(value))
			}
			result.impactLimit = limit
			continue
		}
		if !positionalOnly && (name == "--provider" || name == "--provider-command" || name == "--provider-mcp") {
			if !inline {
				if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
					return result, argumentError("argument " + name + ": expected one argument")
				}
				value = arguments[index+1]
				index += 2
			} else {
				index++
			}
			// EEP-TR-001: only this explicit option reaches the command
			// transport; EEP-TR-002: an argv defect is refused before launch.
			source, err := providerSource(name, value, len(result.impactProviders))
			if err != nil {
				return result, err
			}
			result.impactProviders = append(result.impactProviders, source)
			continue
		}
		if !positionalOnly && name == "--repository" {
			if !inline {
				if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
					return result, argumentError("argument --repository: expected one argument")
				}
				value = arguments[index+1]
				index += 2
			} else {
				index++
			}
			checkout, err := extevidence.ParseCheckout(value)
			if err != nil {
				return result, argumentError("argument --repository: " + err.Error())
			}
			if len(result.impactCheckouts) == extevidence.MaxCheckouts {
				return result, argumentError(fmt.Sprintf("argument --repository: at most %d checkouts", extevidence.MaxCheckouts))
			}
			for _, earlier := range result.impactCheckouts {
				if earlier.ID == checkout.ID {
					return result, argumentError("argument --repository: repository id " + pythonRepr(checkout.ID) + " is bound twice")
				}
			}
			result.impactCheckouts = append(result.impactCheckouts, checkout)
			continue
		}
		if !positionalOnly && strings.HasPrefix(argument, "-") {
			return result, argumentError("unrecognized arguments: " + argument)
		}
		normalized, err := normalizeImpactPath(argument)
		if err != nil {
			return result, err
		}
		if err := validateImpactPathAdmission(normalized); err != nil {
			return result, err
		}
		result.impactPaths = append(result.impactPaths, normalized)
		index++
	}
	if result.impactCapabilities {
		return result, nil
	}
	if len(result.impactCheckouts) != 0 && len(result.impactProviders) == 0 {
		return result, argumentError("--repository requires --provider")
	}
	if result.requireClean && result.impactBaseSet {
		return result, argumentError("--require-clean is available only for path impact, not --base")
	}
	if result.impactBudget != nil && result.impactBaseSet {
		return result, impactBudgetOptionRefusal()
	}
	if result.impactBudget != nil && len(result.impactProviders) != 0 {
		return result, argumentError("--budget-bytes cannot bound the --provider external section")
	}
	if len(result.impactProviders) != 0 && result.impactBaseSet {
		return result, argumentError("--provider is available only for the default path profile, not --base")
	}
	if result.impactBaseSet {
		if result.impactLanguage != "" {
			return result, argumentError("--language-profile is for path impact; range impact names its language profile automatically")
		}
		if len(result.impactPaths) != 0 {
			return result, argumentError("--base is mutually exclusive with paths")
		}
		return result, nil
	}
	if result.impactProfile != "" {
		return result, argumentError("--range-profile requires --base")
	}
	if len(result.impactPaths) == 0 {
		return result, argumentError("the following arguments are required: paths")
	}
	return result, nil
}

func parseFeatureArguments(result options, arguments []string) (options, error) {
	return parseFeatureArgumentsForPlatform(result, arguments, runtime.GOOS)
}

func parseFeatureArgumentsForPlatform(result options, arguments []string, platform string) (options, error) {
	if !nativePlatformQualified(platform) {
		return result, featurePlatformRefusal(platform)
	}
	positionalOnly := false
	for index := 0; index < len(arguments); {
		argument := arguments[index]
		if !positionalOnly && argument == "--" {
			positionalOnly = true
			index++
			continue
		}
		name, value, inline := strings.Cut(argument, "=")
		if !positionalOnly && (name == "--limit" || name == "--budget-bytes") {
			if !inline {
				if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
					return result, argumentError("argument " + name + ": expected one argument")
				}
				value = arguments[index+1]
				index += 2
			} else {
				index++
			}
			if name == "--limit" {
				limit, ok := pythonInteger(value)
				if !ok {
					return result, argumentError("argument --limit: invalid int value: " + pythonRepr(value))
				}
				result.featureLimit = limit
				continue
			}
			budget, ok := pythonBoundedInteger(value, contextindex.MaxPacketBytes)
			if !ok {
				return result, argumentError("argument --budget-bytes: invalid literal for int() with base 10: " + pythonRepr(value))
			}
			if budget < contextindex.MinPacketBytes || budget > contextindex.MaxPacketBytes {
				return result, argumentError(fmt.Sprintf(
					"argument --budget-bytes: budget_bytes must be between %d and %d",
					contextindex.MinPacketBytes, contextindex.MaxPacketBytes,
				))
			}
			result.featureBudget = &budget
			continue
		}
		if !positionalOnly && strings.HasPrefix(argument, "-") {
			return result, argumentError("unrecognized arguments: " + argument)
		}
		if result.featureIDSet {
			return result, argumentError("unrecognized arguments: " + argument)
		}
		result.featureID, result.featureIDSet = argument, true
		index++
	}
	if !result.featureIDSet {
		return result, argumentError("the following arguments are required: feature_id")
	}
	return result, contextindex.ValidateFeature(result.featureID, result.featureLimit)
}

func nativePlatformQualified(platform string) bool {
	return platform == "darwin" || platform == "linux"
}

func pythonInteger(value string) (int, bool) {
	return pythonBoundedInteger(value, maximumImpactLimit)
}

func pythonBoundedInteger(value string, maximum int) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	sign := 1
	if value[0] == '+' || value[0] == '-' {
		if value[0] == '-' {
			sign = -1
		}
		value = value[1:]
	}
	runes := []rune(value)
	if len(runes) == 0 {
		return 0, false
	}
	decimal := new(big.Int)
	ten := big.NewInt(10)
	for index, character := range runes {
		if character == '_' {
			if index == 0 || index+1 == len(runes) {
				return 0, false
			}
			if _, ok := decimalDigit(runes[index-1]); !ok {
				return 0, false
			}
			if _, ok := decimalDigit(runes[index+1]); !ok {
				return 0, false
			}
			continue
		}
		digit, ok := decimalDigit(character)
		if !ok {
			return 0, false
		}
		decimal.Mul(decimal, ten)
		decimal.Add(decimal, big.NewInt(int64(digit)))
	}
	if !decimal.IsInt64() {
		return sign * (maximum + 1), true
	}
	parsed := decimal.Int64()
	if parsed > int64(maximum) {
		parsed = int64(maximum + 1)
	}
	return sign * int(parsed), true
}

func decimalDigit(character rune) (int, bool) {
	if !unicode.IsDigit(character) {
		return 0, false
	}
	for value := 0; value <= 9; value++ {
		zero := character - rune(value)
		if unicode.IsDigit(zero) && (value == 9 || !unicode.IsDigit(zero-1)) {
			return value, true
		}
	}
	return 0, false
}

func pythonRepr(value string) string {
	quote := '\''
	if strings.ContainsRune(value, '\'') && !strings.ContainsRune(value, '"') {
		quote = '"'
	}
	var output strings.Builder
	output.WriteRune(quote)
	for _, character := range value {
		switch character {
		case '\\':
			output.WriteString(`\\`)
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			if character == quote {
				output.WriteByte('\\')
				output.WriteRune(character)
			} else if pythonPrintable(character) {
				output.WriteRune(character)
			} else if character <= 0xff {
				fmt.Fprintf(&output, `\x%02x`, character)
			} else if character <= 0xffff {
				fmt.Fprintf(&output, `\u%04x`, character)
			} else {
				fmt.Fprintf(&output, `\U%08x`, character)
			}
		}
	}
	output.WriteRune(quote)
	return output.String()
}

func pythonPrintable(character rune) bool {
	if character >= 0x20 && character <= 0x7e {
		return true
	}
	return unicode.IsLetter(character) || unicode.IsMark(character) || unicode.IsNumber(character) ||
		unicode.IsPunct(character) || unicode.IsSymbol(character)
}

func normalizeImpactPath(value string) (string, error) {
	if strings.TrimSpace(value) == "" || filepath.IsAbs(value) {
		return "", argumentError("argument paths: path must be repository-relative")
	}
	for _, part := range strings.Split(filepath.ToSlash(value), "/") {
		if part == ".." {
			return "", argumentError("argument paths: path must be repository-relative")
		}
	}
	normalized := filepath.ToSlash(filepath.Clean(value))
	if normalized == "." {
		return "", argumentError("argument paths: path must be repository-relative")
	}
	return normalized, nil
}

// requireRepositoryRoot refuses a working directory that holds no .git entry exactly as
// resolveExplicitRoot refuses an explicit --root, so an omitted --root is classified alike
// (CCF-V1-004). An explicit root already passed this check.
func requireRepositoryRoot(root string) error {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return notRepositoryRootRefusal(".", root)
	}
	return nil
}

func resolveExplicitRoot(value string) (string, error) {
	root, err := normalizeRoot(value)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return "", notRepositoryRootRefusal(value, root)
	}
	return root, nil
}

// normalizeQueryRoot is the query adapter's lexical phase. Repository access
// and symlink resolution happen only after every query option and intent has
// been validated, preserving Python's argument-before-repository precedence.
func normalizeQueryRoot(value string) (string, error) {
	if value == "~" || strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", argumentError("cannot expand --root")
		}
		value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
	}
	root, err := filepath.Abs(value)
	if err != nil {
		return "", argumentError("invalid --root")
	}
	return root, nil
}

func resolveQueryRoot(root string) (string, error) {
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return "", notQueryRepositoryRootRefusal(root)
	}
	return root, nil
}

// normalizeRoot performs only syntactic --root handling: tilde expansion,
// absolutization, and best-effort symlink resolution. It never inspects the
// repository, so CEM invocations keep the frozen validation precedence — a
// map or argument defect is judged before any repository check.
func normalizeRoot(value string) (string, error) {
	if value == "~" || strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", argumentError("cannot expand --root")
		}
		value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
	}
	root, err := filepath.Abs(value)
	if err != nil {
		return "", argumentError("invalid --root")
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	return root, nil
}

// invalidChoice renders argparse's invalid-choice message. The choice order is
// the parser's declaration order, which argparse preserves. Like argparse's
// %r, the value is a Python repr, so a quote or control character in it is
// escaped rather than ending the quoted span.
func invalidChoice(argument, value string, choices []string) error {
	quoted := make([]string, 0, len(choices))
	for _, choice := range choices {
		quoted = append(quoted, pythonRepr(choice))
	}
	return argumentError("argument " + argument + ": invalid choice: " + pythonRepr(value) +
		" (choose from " + strings.Join(quoted, ", ") + ")")
}

var (
	// topLevelCommands must name every top-level verb runContext dispatches, so
	// the invalid-choice message never understates the binary. The leading run
	// keeps the Python oracle's argparse declaration order; verbs this binary
	// adds are appended in the order rootHelp documents them.
	topLevelCommands = []string{"init", "adopt", "feature", "impact", "cem", "context",
		"affected", "index", "docs",
		"test-evidence", "test-validity", "test-plan", "features", "overview", "review", "flows", "runs", "test", "tests", "watch", "mcp", "how"}
)

func emit(stream io.Writer, value any) error {
	encoded, err := gokernel.CanonicalJSON(value)
	if err != nil {
		return err
	}
	encoded = escapeASCIIJSON(encoded)
	_, err = fmt.Fprintf(stream, "%s\n", encoded)
	return err
}

func escapeASCIIJSON(encoded []byte) []byte {
	output := make([]byte, 0, len(encoded))
	for len(encoded) > 0 {
		character, width := utf8.DecodeRune(encoded)
		if character <= 0x7e {
			output = append(output, byte(character))
			encoded = encoded[width:]
			continue
		}
		if character <= 0xffff {
			output = appendUnicodeEscape(output, character)
			encoded = encoded[width:]
			continue
		}
		character -= 0x10000
		output = appendUnicodeEscape(output, 0xd800+(character>>10))
		output = appendUnicodeEscape(output, 0xdc00+(character&0x3ff))
		encoded = encoded[width:]
	}
	return output
}

func appendUnicodeEscape(output []byte, character rune) []byte {
	const hexadecimal = "0123456789abcdef"
	return append(output, '\\', 'u',
		hexadecimal[(character>>12)&0xf], hexadecimal[(character>>8)&0xf],
		hexadecimal[(character>>4)&0xf], hexadecimal[character&0xf])
}

func run(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runContext(context.Background(), arguments, stdin, stdout, stderr)
}

func runContext(ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// mcp runs first: every argument after the verb is the server's, and no other parser may read it (AHI-373).
	if len(arguments) > 0 && arguments[0] == "mcp" {
		return runMCP(ctx, arguments[1:], stdin, stdout, stderr)
	}
	if mcpAfterGlobalRoot(arguments) {
		emitError(stderr, argumentError("argument command: mcp takes its own --root after the verb: corvint mcp --root ABSOLUTE_ROOT"))
		return 2
	}
	if code, handled := runCEMInvocation(ctx, arguments, stdout, stderr); handled {
		return code
	}
	// The cem actions after the closed verifier argv read one repository (WSD-V0-146).
	if root, _, rest, isCEM, err := parseCEMInvocation(arguments); isCEM && err == nil {
		if _, help, _ := parseHelpInvocation(arguments); !help {
			guarded, err := guardWorkspace("cem", root, rest, true)
			if err != nil {
				emitError(stderr, err)
				return 2
			}
			arguments = append(arguments[:len(arguments)-len(rest):len(arguments)-len(rest)], guarded...)
		}
	}
	if exit, handled := runCorpusIntegration(ctx, arguments, stdin, stdout, stderr); handled {
		return exit
	}
	if _, requested, _ := parseHelpInvocation(arguments); !requested {
		if code, handled := runDocsShim(ctx, arguments, stdin, stdout, stderr); handled {
			return code
		}
		if code, handled := runFlowsShim(ctx, arguments, stdin, stdout, stderr); handled {
			return code
		}
		if root, isIndex, indexErr := parseIndexInvocation(arguments); isIndex {
			if indexErr != nil {
				emitError(stderr, indexErr)
				return 2
			}
			return runIndex(ctx, root, stdout, recordingStderr(root.root, stderr))
		}
		if options, isTaskContext, contextErr := parseTaskContextInvocation(arguments); isTaskContext {
			if contextErr != nil {
				emitError(stderr, contextErr)
				return 2
			}
			return runTaskContext(ctx, options, stdout, stderr)
		}
		if options, requested, err := parseGuidanceInvocation(arguments); requested {
			if err != nil {
				emitError(stderr, err)
				return 2
			}
			return runGuidance(ctx, options, stdout, stderr)
		}
		if root, isAffected, affectedErr := parseAffectedInvocation(arguments); isAffected {
			if affectedErr != nil {
				emitError(stderr, affectedErr)
				return 2
			}
			return runAffected(ctx, root, stdout, stderr)
		}
		if root, rest, requested := runsInvocation(arguments); requested {
			return runRuns(ctx, root, rest, stdout, stderr)
		}
		if root, rest, requested := testInvocation(arguments); requested {
			return runTest(ctx, root, rest, stdout, stderr)
		}
		if root, rest, requested := testsInvocation(arguments); requested {
			return runTests(ctx, root, rest, stdout, stderr)
		}
		if root, rest, requested := watchInvocation(arguments); requested {
			return runWatch(ctx, root, rest, stdin, stdout, stderr)
		}
		if root, activation, rest, isActivation, activationErr := parseActivationInvocation(arguments); isActivation {
			if activationErr != nil {
				emitError(stderr, activationErr)
				return 2
			}
			return runActivation(ctx, root, activation, rest, stdout, stderr)
		}
		if code, isTestCommand := runTestCommand(ctx, arguments, stdout, stderr); isTestCommand {
			return code
		}
		if code, handled := runTwoRootImpactInvocation(ctx, arguments, stdout, stderr); handled {
			return code
		}
		if root, callerRoot, rest, isCEM, cemErr := parseCEMInvocation(arguments); isCEM {
			if cemErr != nil {
				emitError(stderr, cemErr)
				return 2
			}
			return cemcli.RunWithCallerRoot(ctx, root, callerRoot, rest, stdout, recordingStderr(root, stderr))
		}
	}
	options, err := parse(arguments)
	if err != nil {
		observeUnsupported(options.root, err, "")
		emitError(stderr, err)
		return 2
	}
	if options.helpTopic != "" {
		if _, err := io.WriteString(stdout, helpText(options.helpTopic)); err != nil {
			emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write help output"})
			return 2
		}
		return 0
	}
	if options.version != "" {
		// The last argument: --version for text, --json for corvint-version/1 (PRS-V1-020).
		return emitVersion(stdout, stderr, options.version == "--json")
	}
	if options.command == "impact" && options.impactCapabilities {
		return emitImpactCapabilities(stdout, stderr)
	}
	if options.command == "impact" || options.command == "feature" {
		buildFailed := false
		build := func() (*contextindex.Index, error) {
			started := time.Now()
			index, err := contextindex.Build(ctx, options.root)
			if err == nil {
				noteSnapshotMissBuild(stderr, options.command, options.root, time.Since(started))
				return index, nil
			}
			buildFailed = true
			observeUnsupported(options.root, err, "")
			var contextError *contextindex.Error
			if errors.As(err, &contextError) && contextError.Code == "unsupported-impact-repository" {
				switch options.command {
				case "feature":
					err = &contextindex.Error{Code: "unsupported-feature-repository", Message: strings.Replace(contextError.Message, "repository index", "feature index", 1)}
				}
			}
			return nil, err
		}
		compute := func(index *contextindex.Index) (map[string]any, error) {
			if options.command == "feature" {
				return contextindex.FeatureBudget(index, options.featureID, options.featureLimit, options.featureBudget)
			}
			if options.requireClean && index.SourceDirty() {
				return contextindex.ImpactCleanAbstention(index, options.impactPaths, options.impactLimit, impactBudget(options.impactPaths, options.impactBudget))
			}
			if options.impactBaseSet {
				if err := impactRangeAncestryRefusal(ctx, index.Root); err != nil {
					return nil, err
				}
			}
			if options.impactBaseSet && options.impactProfile == "expanded-256" {
				return contextindex.ExpandedPerPathRangeImpact(ctx, index, options.impactBase, options.impactLimit)
			}
			if options.impactBaseSet {
				return contextindex.PerPathRangeImpact(ctx, index, options.impactBase, options.impactLimit)
			}
			var contextReceipt map[string]any
			var err error
			if options.impactLanguage != "" {
				contextReceipt, err = contextindex.PathImpactSyntax(index, options.impactPaths, options.impactLimit, options.impactBudget)
			} else {
				contextReceipt, err = standaloneImpactContext(index, options.impactPaths, options.impactLimit, options.impactBudget)
			}
			if err != nil || len(options.impactProviders) == 0 {
				return contextReceipt, err
			}
			// EEP-V0-003: provider output lives only under `external`; every
			// other member is exactly what the run without --provider produced.
			contextReceipt["external"] = extevidence.Section(ctx, index, options.impactProviders, options.impactCheckouts, options.impactPaths, options.impactLimit)
			return contextReceipt, nil
		}
		// Path impact and feature read the snapshot on the terms of
		// IDX-SNAP-V0-019. The working-tree and range profiles read Source.Data
		// directly, which a deferred pack body leaves nil, so they take only the
		// whole-file read (IDX-SNAP-V0-021). A miss builds exactly as before.
		// ERI-V0-261 (V1-1881): a range whose over-bound refusal is proved
		// without the index refuses before the snapshot is loaded, with the
		// bytes the loaded path returns.
		if options.command == "impact" && options.impactBaseSet {
			if err := contextindex.PerPathRangeBoundRefusalBeforeLoad(ctx, options.root, options.impactBase, options.impactLimit, options.impactProfile == "expanded-256"); err != nil {
				observeUnsupported(options.root, err, "")
				emitError(stderr, err)
				return 2
			}
		}
		eager := func() *contextindex.Index { return snapshotIndex(ctx, options.root) }
		deferred := func() *contextindex.Index { return deferredSnapshotIndex(ctx, options.root) }
		if options.impactBaseSet {
			deferred = eager
		}
		contextReceipt, err := overSnapshot(deferred, eager, build, compute)
		if buildFailed {
			emitError(stderr, err)
			return 2
		}
		if err != nil {
			observeUnsupported(options.root, err, "")
			emitError(stderr, err)
			return 2
		}
		if options.impactProducer {
			return emitContextPayloadWithProducer(stdout, stderr, options.command, contextReceipt, currentProducer())
		}
		return emitContextPayload(stdout, stderr, options.command, contextReceipt)
	}
	return 0
}

func validateImpactPathAdmission(value string) error {
	if contextindex.ImpactPathAdmitted(value) {
		return nil
	}
	return impactPathSuffixRefusal(value)
}

// standaloneImpactContext compiles path impact. The syntax profile, explicit or
// selected by a Ruby path (NGI-V0-033), is the corvint.impact receipt itself
// (NGI-V0-068): only a caller budget (MCPV0-032) compiles it. The historical
// default profile keeps the harness default budget it has always had for a path
// no reverse-import rule names.
func standaloneImpactContext(index *contextindex.Index, paths []string, limit int, budget *int) (map[string]any, error) {
	if contextindex.SyntaxProfileSelected(paths) {
		return contextindex.PathImpactSyntax(index, paths, limit, budget)
	}
	if budget = impactBudget(paths, budget); budget != nil {
		return contextindex.EvalImpact(index, paths, limit, budget)
	}
	return contextindex.Impact(index, paths, limit)
}

func impactBudget(paths []string, budget *int) *int {
	if budget != nil || contextindex.SyntaxProfileSelected(paths) {
		return budget
	}
	for _, value := range paths {
		if !contextindex.ImpactRuleNamed(value) {
			fallback := defaultHarnessBudgetBytes - gokernel.OutputOverheadBytes
			return &fallback
		}
	}
	return nil
}

func emitContextPayload(stdout, stderr io.Writer, tool string, contextReceipt map[string]any) int {
	return emitPayload(stdout, stderr, tool, map[string]any{"ok": true, "mutates": false, "tool": tool, "context": contextReceipt})
}

// emitContextPayloadWithProducer adds the opt-in top-level producer member
// (PUB-V0-027); every other member is the default envelope's.
func emitContextPayloadWithProducer(stdout, stderr io.Writer, tool string, contextReceipt map[string]any, producer producerIdentity) int {
	return emitPayload(stdout, stderr, tool, map[string]any{"ok": true, "mutates": false, "tool": tool, "context": contextReceipt, "producer": producer.members()})
}

// emitImpactCapabilities prints the GPK-V0-084 admission listing. Beyond the
// impact root check it reads no repository content, index or stdin.
func emitImpactCapabilities(stdout, stderr io.Writer) int {
	payload := contextindex.ImpactCapabilities(defaultImpactLimit)
	payload["ok"], payload["mutates"], payload["tool"] = true, false, "impact-capabilities"
	payload["profile"] = contextindex.ImpactCapabilitiesProfile
	payload["producer"] = currentProducer().members()
	return emitPayload(stdout, stderr, "impact capabilities", payload)
}

func emitPayload(stdout, stderr io.Writer, tool string, payload map[string]any) int {
	outputFailure := "cannot write " + tool + " output"
	encoded, err := contextindex.CanonicalJSON(payload)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: outputFailure})
		return 2
	}
	if _, err := fmt.Fprintf(stdout, "%s\n", encoded); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: outputFailure})
		return 2
	}
	return 0
}

// runCEMInvocation runs the cem command that owns its whole argv before the
// general router: the stable verifier.
func runCEMInvocation(ctx context.Context, args []string, stdout, stderr io.Writer) (int, bool) {
	// A help request for the action prints its help before the closed argv
	// parser would refuse it (V1-2109).
	if topic, requested, err := parseHelpInvocation(args); requested && err == nil && topic == cemActionHelpPrefix+"verify-stable" {
		if _, err := io.WriteString(stdout, helpText(topic)); err != nil {
			return 1, true
		}
		return 0, true
	}
	return cemcli.RunStableInvocation(ctx, args, stdout, stderr)
}

// parseCEMInvocation detects `[--root PATH] cem ...`. root is normalized and
// symlink-resolved for 0.x reads; an empty --root is the working directory.
// callerRoot is the --root value byte for byte, even empty, or the working
// directory when --root is omitted, for stable admission (CEM-V1-017).
func parseCEMInvocation(arguments []string) (root, callerRoot string, rest []string, isCEM bool, err error) {
	index := 0
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		value := ""
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", "", nil, false, nil
			}
			value = arguments[index+1]
			index += 2
		} else {
			value = strings.TrimPrefix(arguments[index], "--root=")
			index++
		}
		root = value
	}
	if index >= len(arguments) || arguments[index] != "cem" {
		return "", "", nil, false, nil
	}
	callerRoot = root
	if root == "" {
		if root, err = os.Getwd(); err != nil {
			return "", "", nil, true, argumentError("cannot resolve current directory")
		}
		if index == 0 { // only an omitted --root names the working directory
			callerRoot = root
		}
	} else if root, err = normalizeRoot(root); err != nil {
		return "", "", nil, true, err
	}
	return root, callerRoot, arguments[index+1:], true, nil
}

func readInputBounded(ctx context.Context, stdin io.Reader, limit int64) ([]byte, error) {
	result := make(chan readResult, 1)
	go func() {
		value, err := io.ReadAll(io.LimitReader(stdin, limit+1))
		result <- readResult{value: value, err: err}
	}()
	select {
	case completed := <-result:
		return completed.value, completed.err
	case <-ctx.Done():
		if closer, ok := stdin.(io.Closer); ok {
			_ = closer.Close()
		}
		return nil, ctx.Err()
	}
}

// dubiousOwnershipMarker is the text Git prints, before the repository path in
// single quotes, when it refuses a repository owned by another user.
const dubiousOwnershipMarker = "detected dubious ownership in repository at '"

// dubiousOwnershipRefusal classifies Git's uncoded dubious-ownership exit as
// one coded refusal, the same from every verb (GPK-V0-071). Every Git
// subprocess runs with global and system configuration nulled (GPK-V0-009), so
// the safe.directory exception Git suggests can never take effect; the message
// names the repository and the remedies that do work. It returns nil for any
// other error.
func dubiousOwnershipRefusal(err error) error {
	message := err.Error()
	start := strings.Index(message, dubiousOwnershipMarker)
	if start < 0 {
		return nil
	}
	repository, _, quoted := strings.Cut(message[start+len(dubiousOwnershipMarker):], "'")
	if !quoted || repository == "" {
		return nil
	}
	return &gokernel.Error{Code: "repository-ownership-refused", Message: "Git refuses the repository at " + pythonRepr(repository) + " because another user owns it (detected dubious ownership); Corvint runs Git without global or system configuration, so a safe.directory exception cannot take effect: run Corvint as the user who owns the repository, or give the current user ownership of it"}
}

func emitError(stderr io.Writer, err error) {
	if refusal := dubiousOwnershipRefusal(err); refusal != nil {
		err = refusal
	}
	code := "internal-error"
	var kernelError *gokernel.Error
	if errors.As(err, &kernelError) {
		code = kernelError.Code
	}
	var contextError *contextindex.Error
	if errors.As(err, &contextError) && contextError.Code != "" {
		beforeOK, afterOK := diagnosticMembers(err)
		_, _ = fmt.Fprintf(
			stderr, "{\"code\": %s, \"error\": %s, %s\"ok\": false%s}\n",
			pythonJSONString(contextError.Code), pythonJSONString(err.Error()), beforeOK, afterOK,
		)
		return
	}
	if errors.As(err, &contextError) {
		if class := contextindex.GitStartErrorClass(err); class != "" {
			// DRC-V0-910: the start class is additive; the error bytes are unchanged.
			_, _ = fmt.Fprintf(stderr, "{\"error\": %s, \"git_start_error\": %s, \"ok\": false}\n", pythonJSONString(err.Error()), pythonJSONString(class))
			return
		}
		_, _ = fmt.Fprintf(stderr, "{\"error\": %s, \"ok\": false}\n", pythonJSONString(err.Error()))
		return
	}
	beforeOK, afterOK := diagnosticMembers(err)
	_, _ = fmt.Fprintf(
		stderr, "{\"code\": %s, \"error\": %s, %s\"ok\": false%s}\n",
		pythonJSONString(code), pythonJSONString(err.Error()), beforeOK, afterOK,
	)
}

func observeUnsupported(root string, err error, task string) {
	code := ""
	var kernelError *gokernel.Error
	if errors.As(err, &kernelError) {
		code = kernelError.Code
	}
	var contextError *contextindex.Error
	if errors.As(err, &contextError) {
		code = contextError.Code
	}
	recordUnsupported(root, code, task)
}

// recordUnsupported appends one SOL-V0-007 row; like every observation it is
// fail-open and never alters the refusal the caller already emitted.
func recordUnsupported(root, code, task string) {
	if root == "" || !strings.HasPrefix(code, "unsupported-") {
		return
	}
	intent := "unknown"
	if task != "" {
		intent = contextindex.QueryIntent(task)
	}
	event := observations.Event{Kind: "unsupported", Code: code, QueryIntent: intent}
	if task != "" {
		event.TaskSHA256 = observations.TaskHash(task)
	}
	_ = observations.Append(root, event)
}

// unsupportedRecorder is the SOL-V0-007 command boundary for a command whose
// every refusal leaves as one JSON stderr envelope per write: it records the
// envelope's unsupported-* code, so no call site inside the command does.
type unsupportedRecorder struct {
	root   string
	stderr io.Writer
}

// recordingStderr puts a command's stderr behind the SOL-V0-007 boundary. An
// omitted --root is the working directory, the root parse() gives impact.
func recordingStderr(root string, stderr io.Writer) io.Writer {
	if root == "" {
		root = "."
	}
	return unsupportedRecorder{root: root, stderr: stderr}
}

func (recorder unsupportedRecorder) Write(envelope []byte) (int, error) {
	written, err := recorder.stderr.Write(envelope)
	var refusal struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(envelope, &refusal) == nil {
		recordUnsupported(recorder.root, refusal.Code, "")
	}
	return written, err
}

func pythonJSONString(value string) string {
	var output strings.Builder
	output.WriteByte('"')
	for _, character := range value {
		switch character {
		case '"':
			output.WriteString(`\"`)
		case '\\':
			output.WriteString(`\\`)
		case '\b':
			output.WriteString(`\b`)
		case '\f':
			output.WriteString(`\f`)
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			switch {
			case character >= 0x20 && character <= 0x7e:
				output.WriteRune(character)
			case character <= 0xffff:
				fmt.Fprintf(&output, `\u%04x`, character)
			default:
				value := character - 0x10000
				fmt.Fprintf(&output, `\u%04x\u%04x`, 0xd800+(value>>10), 0xdc00+(value&0x3ff))
			}
		}
	}
	output.WriteByte('"')
	return output.String()
}

func main() {
	if code, handled := cliapp.Answer("corvint", os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}

	signalled, cancel := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer cancel()
	exitProcess(runProfiledContext(signalled, signalled, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.LookupEnv))
}

// providerSource turns one `--provider` or `--provider-command` value into a
// provider source under the shared provider bound.
func providerSource(name, value string, selected int) (string, error) {
	if value == "" {
		return "", argumentError("argument " + name + ": expected one argument")
	}
	if selected == extevidence.MaxProviders {
		return "", argumentError(fmt.Sprintf("argument %s: at most %d providers", name, extevidence.MaxProviders))
	}
	if name == "--provider" {
		return value, nil
	}
	source, err := extevidence.ParseCommand(value)
	if name == "--provider-mcp" {
		source, err = extevidence.ParseMCP(value)
	}
	if err != nil {
		return "", argumentError("argument " + name + ": " + err.Error())
	}
	return source, nil
}

// defaultImpactLimit is impact's --limit default, which the GPK-V0-084
// listing also reports.
const defaultImpactLimit = 10

// build is the first-parent commit count of the built commit, stamped with
// -ldflags "-X main.build=N" (PUB-V0-021). An unstamped build reports 0.
var build = "0"
