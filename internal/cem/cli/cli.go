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

var readArguments = []string{"--map", "--patch", "--target", "--expected-base", "--max-unknown", "--max-mechanical"}

var cemActions = map[string]cemAction{
	"begin": {
		arguments: []string{"--patch", "--base", "--output"},
		required:  []string{"--patch", "--output"},
	},
	"prepare": {
		arguments: []string{"--base", "--target", "--map", "--patch"},
		required:  []string{"--base", "--target"},
		booleans:  []string{"--replace"},
		paths:     []string{"--map", "--patch"},
	},
	"cite": {
		arguments: []string{"--map", "--hunk", "--evidence-path", "--bytes", "--lines", "--relation", "--output"},
		required:  []string{"--map", "--hunk", "--evidence-path", "--relation"},
		paths:     []string{"--evidence-path"},
		spans:     []string{"--bytes", "--lines"},
		choices: map[string][]string{"--relation": {
			"call-site", "decision", "dependency", "implementation", "incident", "specification", "test-claim",
		}},
		mutex: &mutexGroup{names: []string{"--bytes", "--lines"}, required: true},
	},
	"mark": {
		arguments: []string{"--map", "--hunk", "--disposition", "--reason", "--output"},
		required:  []string{"--map", "--hunk", "--disposition", "--reason"},
		choices: map[string][]string{
			"--disposition": {"unknown", "mechanical"},
			"--reason": {
				"conflicting-evidence", "formatter-only", "import-reorder", "insufficient-evidence",
				"line-ending-only", "move", "no-evidence", "rename", "whitespace-only",
			},
		},
	},
	"verify": {arguments: readArguments, required: []string{"--map"}},
	"status": {arguments: readArguments, required: []string{"--map"}},
	"report": {
		arguments: []string{"--map", "--patch", "--output", "--target", "--expected-base", "--max-unknown", "--max-mechanical"},
		required:  []string{"--map"},
	},
	"cover": {
		arguments: []string{"--map", "--coverprofile", "--test-run", "--output"},
		required:  []string{"--map", "--coverprofile", "--test-run"},
	},
	"anchor": {
		arguments: []string{"--map", "--commit"},
		required:  []string{"--map"},
		paths:     []string{"--map"},
	},
	"provenance": {arguments: []string{"--commit"}, required: []string{"--commit"}},
}

// cemActionOrder is the order the oracle declares its subparsers in, which is
// the order its invalid-choice message lists them in; cover (TCQ-V0-051) has
// no oracle counterpart and is listed last; anchor and provenance
// (FPK-V0-037..040) follow it.
var cemActionOrder = []string{"begin", "prepare", "cite", "mark", "verify", "status", "report", "cover", "anchor", "provenance"}

// GitNotes serves anchor and provenance (internal/gitnotes). The binary
// installs it, so the CEM seams' dependency closure stays the standard library
// and internal/cem; an uninstalled handler is an invalid choice.
var GitNotes func(ctx context.Context, root, action string, values map[string]string) (map[string]any, error)

func quotedChoices(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, "'"+value+"'")
	}
	return strings.Join(quoted, ", ")
}

func invalidChoice(argument, value string, choices []string) error {
	return cemArgumentError(fmt.Sprintf("argument %s: invalid choice: '%s' (choose from %s)",
		argument, value, quotedChoices(choices)))
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
	envelope, err := dispatchCEM(ctx, root, arguments)
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
	switch action {
	case "begin":
		return session.Begin(ctx, workflow.BeginOptions{
			PatchPath: flags.values["--patch"], Base: flags.values["--base"],
			Output: flags.values["--output"],
		})
	case "prepare":
		return session.Prepare(ctx, workflow.PrepareOptions{
			Base: flags.values["--base"], Target: flags.values["--target"],
			MapPath: flags.values["--map"], Cache: flags.values["--patch"],
			Replace: flags.booleans["--replace"],
		})
	case "cite":
		return session.Cite(ctx, workflow.CiteOptions{
			MapPath: flags.values["--map"], Hunk: flags.values["--hunk"],
			EvidencePath: flags.values["--evidence-path"], Bytes: flags.values["--bytes"],
			Lines: flags.values["--lines"], Relation: flags.values["--relation"],
			Output: flags.values["--output"],
		})
	case "mark":
		return session.Mark(ctx, workflow.MarkOptions{
			MapPath: flags.values["--map"], Hunk: flags.values["--hunk"],
			Disposition: flags.values["--disposition"], Reason: flags.values["--reason"],
			Output: flags.values["--output"],
		})
	case "cover":
		return session.Cover(ctx, workflow.CoverOptions{
			MapPath: flags.values["--map"], Coverprofile: flags.values["--coverprofile"],
			TestRun: flags.values["--test-run"], Output: flags.values["--output"],
		})
	case "anchor", "provenance":
		if GitNotes != nil {
			return GitNotes(ctx, root, action, flags.values)
		}
	case "status", "verify", "report":
		maxUnknown, err := flags.limit("--max-unknown")
		if err != nil {
			return nil, err
		}
		maxMechanical, err := flags.limit("--max-mechanical")
		if err != nil {
			return nil, err
		}
		return session.Read(ctx, action, workflow.ReadOptions{
			MapPath: flags.values["--map"], PatchPath: flags.values["--patch"],
			PatchGiven: flags.present["--patch"], ExpectedBase: flags.values["--expected-base"],
			Target: flags.values["--target"], Output: flags.values["--output"],
			Limits: workflow.PolicyLimits{MaxUnknown: maxUnknown, MaxMechanical: maxMechanical},
		})
	}
	return nil, invalidChoice("cem_command", action, cemActionOrder)
}

// oracleReadFailures maps the candidate's bounded-input failures onto the
// oracle's CLI-level read errors. The oracle reads the patch and the map in its
// argument layer rather than in its CEM module, so those two failures carry no
// code at all and one fixed message; the candidate reads them inside the
// session and had invented a code and a path-bearing message for each.
//
// A failure that carries a bounded recovery line keeps that line after the
// fixed text: collapsing an inherited-map rejection to "cannot read CEM map"
// alone forced the caller to read the implementation to find --replace.
var oracleReadFailures = map[string]string{
	cemcode.PatchUnavailable: "cannot read patch",
	cemcode.MapUnavailable:   "cannot read CEM map",
}

func emitCEMError(stderr io.Writer, err error) {
	code := cemcode.CodeOf(err)
	if untyped, ok := oracleReadFailures[code]; ok {
		if guidance := cemcode.GuidanceOf(err); guidance != "" {
			untyped += ": " + guidance
		}
		_, _ = fmt.Fprintf(stderr, "{\"error\": %s, \"ok\": false}\n", wire.CanonicalString(untyped))
		return
	}
	if code == "" {
		code = "internal-error"
	}
	_, _ = fmt.Fprintf(stderr, "{\"code\": %s, \"error\": %s, \"ok\": false}\n",
		wire.CanonicalString(code), wire.CanonicalString(cemcode.MessageOf(err)))
}
