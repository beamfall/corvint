// Package cli is the thin CEM command dispatch. It lives under internal/cem
// so the dependency-closure regression covers it: the CEM seams and this
// dispatch depend only on the Go standard library and the local Git
// executable, and never route through internal/gokernel or any
// runtime/provider surface.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

type cemFlags struct {
	values   map[string]string
	present  map[string]bool
	booleans map[string]bool
}

// --- argparse-faithful argument validation ---
//
// The oracle's CEM surface is argparse, and its messages and their ORDER are
// part of the observable result. Every check below mirrors one argparse stage,
// and semantic work only begins once they all pass: the candidate used to
// report "hunk selector does not identify a map hunk" for an invocation that
// argparse rejects before the handler ever runs.

type mutexGroup struct {
	names    []string
	required bool
}

// cemAction mirrors one subparser declaration, including argument ORDER, which
// is the order argparse lists missing required arguments in.
type cemAction struct {
	arguments []string // declaration order, required and optional alike
	required  []string // declaration order
	booleans  []string
	// paths are the arguments the oracle declares with type=_path, which
	// validates and normalizes at PARSE time. Arguments declared type=Path get
	// no such check, so this set is not "every path-shaped option".
	paths []string
	// spans are the arguments the oracle declares with type=_span, which like
	// _path converts at PARSE time: a malformed, non-integer, negative, or
	// descending span fails before the command reads anything.
	spans   []string
	choices map[string][]string
	mutex   *mutexGroup
}

// readArguments are shared by status and verify. --artifacts is the explicit
// absolute artifact root only a cem/1.0 map accepts (CEM-V1-017, CEM-V1-018).
var readArguments = []string{"--map", "--patch", "--target", "--expected-base", "--max-unknown", "--max-mechanical", "--artifacts"}

var nonRegularChoice = map[string][]string{"--non-regular": {workflow.NonRegularUnknown}}

// pathPolicyChoices adds --path-policy base, the additive cem-path-policy/1
// profile of prepare and status (CAX-V0-060).
var pathPolicyChoices = map[string][]string{"--non-regular": {workflow.NonRegularUnknown}, "--path-policy": {workflow.PathPolicyBase}}

var cemActions = map[string]cemAction{
	// --spec admits only cem/1.0, the one producer profile; omitted, begin and
	// prepare write cem/1.0 too (CEM-V1-140). The producer, not an argparse
	// choice, judges the value, so a retired label refuses unsupported-spec with
	// the recovery line and any other value invalid-arguments (CEM-V1-216).
	"begin": {
		arguments: []string{"--patch", "--base", "--output", "--spec"},
		required:  []string{"--patch", "--output"},
	},
	// --non-regular unknown selects the additive cem-non-regular/1 profile
	// on prepare, verify and status (CAX-V0-020 to CAX-V0-022).
	"prepare": {
		arguments: []string{"--base", "--target", "--map", "--patch", "--spec", "--non-regular", "--path-policy"},
		required:  []string{"--base", "--target"},
		booleans:  []string{"--replace"},
		paths:     []string{"--map", "--patch"},
		choices:   pathPolicyChoices,
	},
	// --paired-output, --paired-range and --paired-root together cite into the
	// additive cem-paired-evidence/1 document instead of the map (CAX-V0-032).
	// --replace with --expected-hunk-digest replaces one hunk's citations
	// (CEM-V1-694).
	"cite": {
		arguments: []string{"--map", "--hunk", "--evidence-path", "--bytes", "--lines", "--relation", "--output", "--paired-output", "--paired-range", "--paired-root", "--expected-hunk-digest"},
		required:  []string{"--map", "--hunk", "--evidence-path", "--relation"},
		booleans:  []string{"--replace"},
		paths:     []string{"--evidence-path", "--paired-output"},
		spans:     []string{"--bytes", "--lines"},
		choices: map[string][]string{"--relation": {
			"call-site", "decision", "dependency", "implementation", "incident", "specification", "test-claim",
		}},
		mutex: &mutexGroup{names: []string{"--bytes", "--lines"}, required: true},
	},
	"mark": {
		arguments: []string{"--map", "--hunk", "--disposition", "--reason", "--output", "--expected-hunk-digest"},
		required:  []string{"--map", "--hunk", "--disposition", "--reason"},
		choices: map[string][]string{
			"--disposition": {"unknown", "mechanical"},
			"--reason": {
				"conflicting-evidence", "formatter-only", "import-reorder", "insufficient-evidence",
				"line-ending-only", "move", "no-evidence", "rename", "whitespace-only",
			},
		},
	},
	"verify": {arguments: append(slices.Clone(readArguments), "--non-regular"), required: []string{"--map"}, booleans: []string{"--citation", "--hunk-revert", "--guarding-tests"}, choices: nonRegularChoice},
	// --max-deferred-intents bounds the change's recorded intent-scope
	// deferral; verify refuses it (CEM-V1-645). --scaled-operations opts a
	// cem/1.0 status into the map-scaled Git operation bound (DCW-V0-373).
	// --paired with --paired-root re-verifies a cem-paired-evidence/1 document
	// instead of reading the map's status (CAX-V0-033).
	"status": {arguments: append(slices.Clone(readArguments), "--max-deferred-intents", "--non-regular", "--path-policy", "--paired", "--paired-root"), required: []string{"--map"}, booleans: []string{"--citation", "--scaled-operations"}, paths: []string{"--paired"}, choices: pathPolicyChoices},
	"report": {
		arguments: []string{"--map", "--patch", "--output", "--target", "--expected-base", "--max-unknown", "--max-mechanical", "--format", "--ocm", "--artifacts", "--max-deferred-intents"},
		required:  []string{"--map"},
		choices:   map[string][]string{"--format": {"markdown", "json"}},
		booleans:  []string{"--preview"},
	},
	"cover": {
		arguments: []string{"--map", "--coverprofile", "--test-run", "--output", "--states-output"},
		required:  []string{"--map", "--coverprofile", "--test-run"},
	},
	"discriminate": {
		arguments: []string{"--map", "--target", "--max-hunks", "--max-mutants", "--wall-time", "--same-change-test", "--output"},
		required:  []string{"--map", "--target"},
	},
	"anchor": {
		arguments: []string{"--map", "--commit", "--artifacts"},
		required:  []string{"--map"},
		paths:     []string{"--map"},
	},
	"provenance": {arguments: []string{"--commit", "--notes-tip", "--verdicts-tip"}, required: []string{"--commit"}, booleans: []string{"--verdicts"}},
	"export": {
		arguments: []string{"--map", "--expected-base", "--target", "--output", "--witness", "--artifacts"},
		required:  []string{"--map", "--expected-base", "--target", "--output"},
		paths:     []string{"--map"},
	},
	// defer records one intent-scope deferral as a tracked content-addressed
	// file the change creates, never a map member (CEM-V1-644; experimental).
	"defer": {
		arguments: []string{"--map", "--intent", "--reason"},
		required:  []string{"--map", "--intent", "--reason"},
		paths:     []string{"--map", "--intent"},
	},
}

// cemActionOrder is the order the oracle declares its subparsers in, which is
// the order its invalid-choice message lists them in; cover (TCQ-V0-051),
// discriminate (TCQ-V0-055), anchor and provenance (FPK-V0-037..040), and
// export (RCB-V0-001) and defer (CEM-V1-644) have no oracle counterpart and
// are listed last.
var cemActionOrder = []string{"begin", "prepare", "cite", "mark", "verify", "status", "report", "cover", "discriminate", "anchor", "provenance", "export", "defer"}

// GitNotes serves anchor and provenance (internal/gitnotes). The binary
// installs it, so the CEM seams' dependency closure stays the standard library
// and internal/cem; an uninstalled handler is an invalid choice. values holds
// only the valued arguments given; booleans the boolean flags given.
var GitNotes func(ctx context.Context, root, action string, values map[string]string, booleans map[string]bool) (map[string]any, error)

// Export serves export (internal/receiptbundle), installed by the binary for
// the same reason as GitNotes; cem/1.0 admits callerRoot (CEM-V1-060).
var Export func(ctx context.Context, root, callerRoot string, values map[string]string) (map[string]any, error)

// StableOCM verifies the OCM a cem/1.0 report supplies through the stable
// universe admission (internal/lrfrepo); the binary installs it for the same
// reason as GitNotes. Nil leaves stable obligations NOT_PRODUCED (V1-1305).
var StableOCM workflow.StableOCMReader

// IntentObligations counts one intent document's obligations for a recorded
// intent-scope deferral (CEM-V1-644, CEM-V1-645). The binary installs it;
// nil leaves `cem defer` refused and each checked count NOT_CHECKED.
var IntentObligations workflow.IntentObligationCounter

// DataReaders finds a declared data path's readers in the target index for
// the cem-path-policy/1 profile (CAX-V0-064). The binary installs it; nil
// leaves each data row data-reader-not-produced.
var DataReaders workflow.DataReaderFinder

func quotedChoices(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, pythonRepr(value))
	}
	return strings.Join(quoted, ", ")
}

// invalidChoice renders argparse's invalid-choice message. Like argparse's %r,
// the value is a Python repr, so a quote or control character in it is escaped
// rather than ending the quoted span.
func invalidChoice(argument, value string, choices []string) error {
	return cemArgumentError(fmt.Sprintf("argument %s: invalid choice: %s (choose from %s)",
		argument, pythonRepr(value), quotedChoices(choices)))
}

// pythonRepr quotes a string the way Python's repr() does: single quotes unless
// the string holds one and no double quote, with backslashes, the chosen quote
// and non-printable characters escaped.
func pythonRepr(value string) string {
	quote := '\''
	if strings.ContainsRune(value, '\'') && !strings.ContainsRune(value, '"') {
		quote = '"'
	}
	var output strings.Builder
	output.WriteRune(quote)
	for _, character := range value {
		switch {
		case character == '\\':
			output.WriteString(`\\`)
		case character == '\n':
			output.WriteString(`\n`)
		case character == '\r':
			output.WriteString(`\r`)
		case character == '\t':
			output.WriteString(`\t`)
		case character == quote:
			output.WriteByte('\\')
			output.WriteRune(character)
		case character >= 0x20 && character <= 0x7e, unicode.IsLetter(character), unicode.IsMark(character),
			unicode.IsNumber(character), unicode.IsPunct(character), unicode.IsSymbol(character):
			output.WriteRune(character)
		case character <= 0xff:
			fmt.Fprintf(&output, `\x%02x`, character)
		case character <= 0xffff:
			fmt.Fprintf(&output, `\u%04x`, character)
		default:
			fmt.Fprintf(&output, `\U%08x`, character)
		}
	}
	output.WriteRune(quote)
	return output.String()
}

// parseAction validates the subcommand before anything else, exactly as
// argparse resolves a required subparser first.
func parseAction(arguments []string) (string, cemAction, error) {
	if len(arguments) == 0 {
		return "", cemAction{}, cemArgumentError("the following arguments are required: cem_command")
	}
	spec, known := cemActions[arguments[0]]
	if !known {
		return "", cemAction{}, invalidChoice("cem_command", arguments[0], cemActionOrder)
	}
	return arguments[0], spec, nil
}

// parseCEMFlags walks the tokens once. Parse-time failures - a missing value, an
// invalid choice, a mutually exclusive pair - are reported where they occur, so
// the first one in token order wins, as argparse does. A repeated flag is NOT an
// error: argparse keeps the last value.
func parseCEMFlags(spec cemAction, arguments []string) (*cemFlags, []string, error) {
	flags := &cemFlags{values: map[string]string{}, present: map[string]bool{}, booleans: map[string]bool{}}
	known := map[string]bool{}
	for _, name := range spec.arguments {
		known[name] = true
	}
	booleans := map[string]bool{}
	for _, name := range spec.booleans {
		booleans[name] = true
	}
	var unrecognized []string
	for index := 0; index < len(arguments); {
		name := arguments[index]
		switch {
		case booleans[name]:
			flags.booleans[name] = true
			index++
		case known[name]:
			if index+1 >= len(arguments) {
				return nil, nil, cemArgumentError(fmt.Sprintf("argument %s: expected one argument", name))
			}
			value := arguments[index+1]
			if choices, limited := spec.choices[name]; limited && !slices.Contains(choices, value) {
				return nil, nil, invalidChoice(name, value, choices)
			}
			if slices.Contains(spec.paths, name) {
				normalized, err := repositoryRelative(name, value)
				if err != nil {
					return nil, nil, err
				}
				value = normalized
			}
			if slices.Contains(spec.spans, name) {
				if err := parseTimeSpan(name, value); err != nil {
					return nil, nil, err
				}
			}
			if err := checkMutex(spec, flags, name); err != nil {
				return nil, nil, err
			}
			flags.present[name] = true
			flags.values[name] = value
			index += 2
		default:
			unrecognized = append(unrecognized, name)
			index++
		}
	}
	return flags, unrecognized, nil
}

func checkMutex(spec cemAction, flags *cemFlags, name string) error {
	if spec.mutex == nil || !slices.Contains(spec.mutex.names, name) {
		return nil
	}
	for _, other := range spec.mutex.names {
		if other != name && flags.present[other] {
			return cemArgumentError(fmt.Sprintf("argument %s: not allowed with argument %s", name, other))
		}
	}
	return nil
}

// validateParsed runs the post-parse stages in argparse's order: required
// arguments, then a required mutually exclusive group, then unrecognized
// tokens last.
func validateParsed(spec cemAction, flags *cemFlags, unrecognized []string) error {
	if err := refusePairedChange(unrecognized); err != nil {
		return err
	}
	var missing []string
	for _, name := range spec.required {
		if !flags.present[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		return cemArgumentError("the following arguments are required: " + strings.Join(missing, ", "))
	}
	if spec.mutex != nil && spec.mutex.required {
		chosen := false
		for _, name := range spec.mutex.names {
			chosen = chosen || flags.present[name]
		}
		if !chosen {
			return cemArgumentError("one of the arguments " + strings.Join(spec.mutex.names, " ") + " is required")
		}
	}
	if len(unrecognized) != 0 {
		return cemArgumentError("unrecognized arguments: " + strings.Join(unrecognized, " "))
	}
	return nil
}

// refusePairedChange refuses the witness's paired-change flags on every cem
// action before any other stage, with the reason instead of the generic
// unrecognized-arguments line (CEM-V1-648): a cem/1.0 map binds one
// repository's base and target, so a change split across an app range and a
// tests range has no cem verb yet, and no flag guesses one.
func refusePairedChange(unrecognized []string) error {
	if message, ok := PairedChangeRefusal(unrecognized); ok {
		return cemArgumentError(message)
	}
	return nil
}

// PairedChangeRefusal returns the CEM-V1-648 refusal for the first of tokens
// that is a paired-change flag, spaced or in = form, and false when none is.
// `prove --cem` shares it so every surface that reads a cem/1.0 map refuses a
// paired change with the same reason.
func PairedChangeRefusal(tokens []string) (string, bool) {
	for _, token := range tokens {
		name, _, _ := strings.Cut(token, "=")
		if name == "--range" || name == "--range-root" || name == "--paired-from" {
			return fmt.Sprintf("argument %s: cem takes no paired change: a cem/1.0 map binds one repository's base and target, so pair an app range and a tests range with corvint witness and map each repository's change on its own (CEM-V1-648)", name), true
		}
	}
	return "", false
}

// repositoryRelative mirrors the oracle's _path argparse type: it rejects an
// absolute, blank, or bare-"." value and any value with a dot-dot component,
// then returns the POSIX normalization. Rejecting here rather than at a later
// containment check is what keeps the ERROR CODE identical: argparse fails a
// type conversion as invalid-arguments, long before the command sees the value.
func repositoryRelative(name, value string) (string, error) {
	invalid := cemArgumentError(fmt.Sprintf("argument %s: path must be repository-relative", name))
	if strings.TrimSpace(value) == "" || strings.HasPrefix(value, "/") {
		return "", invalid
	}
	parts := make([]string, 0, strings.Count(value, "/")+1)
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return "", invalid
		}
		if part == "" || part == "." {
			continue
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return "", invalid
	}
	return strings.Join(parts, "/"), nil
}

// parseTimeSpan mirrors the oracle's _span argparse type, which composes
// _non_negative. Each rejection carries its own message, because argparse names
// the specific conversion that failed; collapsing them into one message loses
// the distinction the oracle draws. Bounds against the evidence blob are NOT
// checked here: argparse cannot see the blob, so those stay semantic codes.
func parseTimeSpan(name, value string) error {
	fail := func(detail string) error {
		return cemArgumentError(fmt.Sprintf("argument %s: %s", name, detail))
	}
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return fail("span must be START:END")
	}
	bounds := make([]int64, 0, 2)
	for _, part := range parts {
		parsed, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return fail("value must be an integer")
		}
		if parsed < 0 {
			return fail("value must be non-negative")
		}
		bounds = append(bounds, parsed)
	}
	if bounds[1] < bounds[0] {
		return fail("span end must not precede its start")
	}
	return nil
}

func cemArgumentError(message string) error {
	return cemcode.New(cemcode.InvalidArguments, "%s", message)
}

func (f *cemFlags) limit(name string) (*int, error) {
	if !f.present[name] {
		return nil, nil
	}
	value, err := strconv.Atoi(f.values[name])
	if err != nil || value < 0 {
		return nil, cemArgumentError(name + " must be a non-negative integer")
	}
	return &value, nil
}

// Run executes one `corvint cem ACTION` invocation.
func Run(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	return RunWithCallerRoot(ctx, root, root, arguments, stdout, stderr)
}

// RunWithCallerRoot is Run for an entry point that normalized the root it was
// given. Every producer command reads root; a cem/1.0 status or verify passes
// callerRoot, the root byte for byte as the caller named it, to stable
// admission, which judges it exactly as `cem verify-stable --repository` judges
// the same string (CEM-V1-017).
func RunWithCallerRoot(ctx context.Context, root, callerRoot string, arguments []string, stdout, stderr io.Writer) int {
	envelope, err := dispatchCEMWithCallerRoot(ctx, root, callerRoot, arguments)
	if err != nil {
		emitCEMError(stderr, err)
		return 2
	}
	encoded, marshalErr := json.Marshal(envelope)
	if marshalErr != nil {
		emitCEMError(stderr, cemcode.New("output-failed", "cannot encode CEM output"))
		return 2
	}
	encoded = escapeASCIIJSON(encoded)
	if _, writeErr := fmt.Fprintf(stdout, "%s\n", encoded); writeErr != nil {
		emitCEMError(stderr, cemcode.New("output-failed", "cannot write CEM output"))
		return 2
	}
	// CEM-V1-017: a stable read exits with the verifier's own code (0 accept,
	// 1 reject, 2 refusal), never one re-derived from the envelope; an accepted
	// status whose policy ceilings are exceeded exits 1 (CEM-V1-145).
	if stable, isStable := envelope["stableVerification"].(workflow.StableVerification); isStable {
		if stable.Exit == 0 && envelope["ok"] != true {
			return 1
		}
		return stable.Exit
	}
	if envelope["ok"] != true {
		return 1
	}
	return 0
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

func dispatchCEM(ctx context.Context, root string, arguments []string) (map[string]any, error) {
	return dispatchCEMWithCallerRoot(ctx, root, root, arguments)
}

func dispatchCEMWithCallerRoot(ctx context.Context, root, callerRoot string, arguments []string) (map[string]any, error) {
	action, spec, err := parseAction(arguments)
	if err != nil {
		return nil, err
	}
	flags, unrecognized, err := parseCEMFlags(spec, arguments[1:])
	if err != nil {
		return nil, err
	}
	if err := validateParsed(spec, flags, unrecognized); err != nil {
		return nil, err
	}
	session, err := workflow.Open(root)
	if err != nil {
		return nil, err
	}
	session.SetStableRepository(callerRoot)
	session.SetStableOCMReader(StableOCM)
	session.SetIntentObligationCounter(IntentObligations)
	session.SetDataReaderFinder(DataReaders)
	switch action {
	case "begin":
		return session.Begin(ctx, workflow.BeginOptions{
			PatchPath: flags.values["--patch"], Base: flags.values["--base"],
			Output: flags.values["--output"], Spec: flags.values["--spec"],
		})
	case "prepare":
		return session.Prepare(ctx, workflow.PrepareOptions{
			Base: flags.values["--base"], Target: flags.values["--target"],
			MapPath: flags.values["--map"], Cache: flags.values["--patch"],
			Replace: flags.booleans["--replace"], Spec: flags.values["--spec"],
			NonRegular: flags.values["--non-regular"], PathPolicy: flags.values["--path-policy"],
		})
	case "cite":
		if paired := pairedFlagsGiven(flags, "--paired-output", "--paired-range", "--paired-root"); paired != nil {
			if err := paired(); err != nil {
				return nil, err
			}
			if flags.present["--output"] {
				return nil, cemcode.New(cemcode.InvalidArguments, "argument --output: not allowed with --paired-output: a paired citation never writes the map")
			}
			if flags.booleans["--replace"] || flags.present["--expected-hunk-digest"] {
				return nil, cemcode.New(cemcode.InvalidArguments, "argument --replace: not allowed with --paired-output: a paired citation never writes the map")
			}
			return session.CitePaired(ctx, workflow.PairedCiteOptions{
				MapPath: flags.values["--map"], Output: flags.values["--paired-output"],
				Range: flags.values["--paired-range"], Root: flags.values["--paired-root"],
				Hunk: flags.values["--hunk"], EvidencePath: flags.values["--evidence-path"],
				Bytes: flags.values["--bytes"], Lines: flags.values["--lines"], Relation: flags.values["--relation"],
			})
		}
		// A replacement is guarded: --replace and --expected-hunk-digest are
		// given together or not at all (CEM-V1-694).
		if flags.booleans["--replace"] != flags.present["--expected-hunk-digest"] {
			return nil, cemcode.New(cemcode.InvalidArguments, "argument --replace: requires --expected-hunk-digest, and --expected-hunk-digest requires --replace")
		}
		if flags.booleans["--replace"] && !hunkDigestShape(flags.values["--expected-hunk-digest"]) {
			return nil, cemcode.New(cemcode.InvalidArguments, "argument --expected-hunk-digest: must be 64 lowercase hexadecimal digits")
		}
		return session.Cite(ctx, workflow.CiteOptions{
			MapPath: flags.values["--map"], Hunk: flags.values["--hunk"],
			EvidencePath: flags.values["--evidence-path"], Bytes: flags.values["--bytes"],
			Lines: flags.values["--lines"], Relation: flags.values["--relation"],
			Output:  flags.values["--output"],
			Replace: flags.booleans["--replace"], ExpectedHunkDigest: flags.values["--expected-hunk-digest"],
		})
	case "mark":
		if digest, given := flags.values["--expected-hunk-digest"], flags.present["--expected-hunk-digest"]; given && !hunkDigestShape(digest) {
			return nil, cemcode.New(cemcode.InvalidArguments, "argument --expected-hunk-digest: must be 64 lowercase hexadecimal digits")
		}
		return session.Mark(ctx, workflow.MarkOptions{
			MapPath: flags.values["--map"], Hunk: flags.values["--hunk"],
			Disposition: flags.values["--disposition"], Reason: flags.values["--reason"],
			Output: flags.values["--output"], ExpectedHunkDigest: flags.values["--expected-hunk-digest"],
		})
	case "cover":
		return session.Cover(ctx, workflow.CoverOptions{
			MapPath: flags.values["--map"], Coverprofile: flags.values["--coverprofile"],
			TestRun: flags.values["--test-run"], Output: flags.values["--output"],
			StatesOutput: flags.values["--states-output"],
		})
	case "discriminate":
		return session.Discriminate(ctx, workflow.DiscriminateOptions{
			MapPath: flags.values["--map"], Target: flags.values["--target"],
			MaxHunks: flags.values["--max-hunks"], MaxMutants: flags.values["--max-mutants"],
			WallTime: flags.values["--wall-time"], SameChangeTests: flags.values["--same-change-test"],
			Output: flags.values["--output"],
		})
	case "anchor", "provenance":
		if GitNotes != nil {
			return GitNotes(ctx, root, action, flags.values, flags.booleans)
		}
	case "export":
		if Export != nil {
			return Export(ctx, root, callerRoot, flags.values)
		}
	case "defer":
		return session.Defer(ctx, workflow.DeferOptions{
			MapPath: flags.values["--map"], Intent: flags.values["--intent"], Reason: flags.values["--reason"],
		})
	case "status", "verify", "report":
		if paired := pairedFlagsGiven(flags, "--paired", "--paired-root"); paired != nil {
			if err := paired(); err != nil {
				return nil, err
			}
			for _, name := range spec.arguments {
				if flags.present[name] && name != "--map" && name != "--paired" && name != "--paired-root" {
					return nil, cemcode.New(cemcode.InvalidArguments, "argument %s: not allowed with --paired", name)
				}
			}
			for _, name := range spec.booleans {
				if flags.booleans[name] {
					return nil, cemcode.New(cemcode.InvalidArguments, "argument %s: not allowed with --paired", name)
				}
			}
			return session.PairedStatus(ctx, workflow.PairedStatusOptions{
				MapPath: flags.values["--map"], Paired: flags.values["--paired"], Root: flags.values["--paired-root"],
			})
		}
		maxUnknown, err := flags.limit("--max-unknown")
		if err != nil {
			return nil, err
		}
		maxMechanical, err := flags.limit("--max-mechanical")
		if err != nil {
			return nil, err
		}
		maxDeferred, err := flags.limit("--max-deferred-intents")
		if err != nil {
			return nil, err
		}
		// --preview renders the Markdown report into the envelope and writes
		// nothing, so the report is read-only on request (DCW-V0-345).
		if flags.booleans["--preview"] {
			if flags.present["--output"] || flags.values["--format"] == "json" {
				return nil, cemcode.New(cemcode.InvalidArguments, "--preview writes nothing and takes neither --output nor --format json")
			}
			action = workflow.ActionReportPreview
		}
		return session.Read(ctx, action, workflow.ReadOptions{
			MapPath: flags.values["--map"], PatchGiven: flags.present["--patch"], ExpectedBase: flags.values["--expected-base"],
			Target: flags.values["--target"], Output: flags.values["--output"],
			Format: flags.values["--format"], OCMPath: flags.values["--ocm"],
			OCMGiven:     flags.present["--ocm"],
			Limits:       workflow.PolicyLimits{MaxUnknown: maxUnknown, MaxMechanical: maxMechanical},
			ArtifactRoot: flags.values["--artifacts"], ArtifactsGiven: flags.present["--artifacts"],
			Citation: flags.booleans["--citation"], HunkRevert: flags.booleans["--hunk-revert"],
			GuardingTests: flags.booleans["--guarding-tests"], MaxDeferredIntents: maxDeferred,
			ScaledOperations: flags.booleans["--scaled-operations"],
			NonRegular:       flags.values["--non-regular"] == workflow.NonRegularUnknown,
			PathPolicy:       flags.values["--path-policy"] == workflow.PathPolicyBase,
		})
	}
	return nil, invalidChoice("cem_command", action, cemActionOrder)
}

// oracleReadFailures keeps the oracle's fixed CLI-level read messages for the
// patch and the map. The oracle reads both in its argument layer and emitted no
// code; the candidate keeps the fixed text and, since decision 0398, also emits
// the code (CCF-V1-004), so a caller can branch on it.
//
// A failure that carries a bounded recovery line keeps that line after the
// fixed text: collapsing an inherited-map rejection to "cannot read CEM map"
// alone forced the caller to read the implementation to find --replace.
var oracleReadFailures = map[string]string{
	cemcode.PatchUnavailable: "cannot read patch",
	cemcode.MapUnavailable:   "cannot read CEM map",
}

// hunkDigestShape admits a wire.HunkDigest value: 64 lowercase hex digits.
func hunkDigestShape(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}

func emitCEMError(stderr io.Writer, err error) {
	code := cemcode.CodeOf(err)
	message := cemcode.MessageOf(err)
	if fixed, ok := oracleReadFailures[code]; ok {
		message = fixed
		if guidance := cemcode.GuidanceOf(err); guidance != "" {
			message += ": " + guidance
		}
	}
	if code == "" {
		code = "internal-error"
	}
	_, _ = fmt.Fprintf(stderr, "{\"code\": %s, \"error\": %s, \"ok\": false}\n",
		wire.CanonicalString(code), wire.CanonicalString(message))
}

// pairedFlagsGiven returns nil when none of the paired-profile flags names
// was given, and otherwise a check that refuses unless all of them were: the
// paired profile is used only whole (CAX-V0-032, CAX-V0-033).
func pairedFlagsGiven(flags *cemFlags, names ...string) func() error {
	var given, missing []string
	for _, name := range names {
		if flags.present[name] {
			given = append(given, name)
		} else {
			missing = append(missing, name)
		}
	}
	if len(given) == 0 {
		return nil
	}
	return func() error {
		if len(missing) != 0 {
			return cemcode.New(cemcode.InvalidArguments, "argument %s: requires %s", given[0], strings.Join(missing, " and "))
		}
		return nil
	}
}
