package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/liveverify/rbmutate"
)

// Named-source claims (FPK-V0-074 to FPK-V0-077): `--mutate-source` names a
// Ruby source, changed or not, with optional line spans, and each
// `--mutate-example SPEC:LINE[:LINE...]` cites RSpec examples that claim to
// cover it. Every cited spec file becomes one row of this kind.
const (
	kindNamedSource        = "named-source-test"
	namedSourceClaimPrefix = "named source test for "
	// The bounds of one selector: spec files (one row and one mutation run
	// each), caller spans, and example lines per spec file.
	maxNamedSpecFiles    = 4
	maxNamedSpans        = 16
	maxNamedExampleLines = 16
)

var (
	namedSourcePattern  = regexp.MustCompile(`^(.+\.rb)(?::([0-9][0-9,-]*))?$`)
	namedExamplePattern = regexp.MustCompile(`^(.+\.rb)((?::[0-9]+)+)$`)
)

// proveNamedSource is a parsed named-source selector.
type proveNamedSource struct {
	source string
	spans  []mutate.LineSpan
	// examples maps each cited spec file to its sorted example lines.
	examples map[string][]int
}

// lineBasis says how the mutated lines were chosen (FPK-V0-076): the
// caller's spans, else the whole file under the mutant cap. The lines the
// cited examples execute are not cheaply knowable without instrumenting
// RSpec, so that basis is never claimed.
func (named *proveNamedSource) lineBasis() string {
	if len(named.spans) != 0 {
		return rbmutate.LineBasisCallerSpans
	}
	return rbmutate.LineBasisWholeFile
}

// withoutNamedSourceFlags lifts `--mutate-source` and every `--mutate-example`
// (each `--flag VALUE` or `--flag=VALUE`) out of the arguments after the
// command word, before the wrapped parser sees them. A token after `--` is
// positional and is never lifted.
func withoutNamedSourceFlags(arguments []string) ([]string, *proveNamedSource, error) {
	kept := make([]string, 0, len(arguments))
	var sources, examples []string
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--" {
			kept = append(kept, arguments[index:]...)
			break
		}
		name, value, inline := strings.Cut(argument, "=")
		if name != "--mutate-source" && name != "--mutate-example" {
			kept = append(kept, argument)
			continue
		}
		if !inline {
			if index+1 >= len(arguments) {
				return nil, nil, argumentError("argument " + name + ": expected one argument")
			}
			index++
			value = arguments[index]
		}
		if name == "--mutate-source" {
			sources = append(sources, value)
		} else {
			examples = append(examples, value)
		}
	}
	if len(sources) == 0 && len(examples) == 0 {
		return kept, nil, nil
	}
	named, err := parseNamedSource(sources, examples)
	return kept, named, err
}

func parseNamedSource(sources, examples []string) (*proveNamedSource, error) {
	if len(sources) != 1 {
		if len(sources) == 0 {
			return nil, argumentError("argument --mutate-example: requires --mutate-source")
		}
		return nil, argumentError("argument --mutate-source: may not be repeated")
	}
	if len(examples) == 0 {
		return nil, argumentError("argument --mutate-source: requires at least one --mutate-example SPEC:LINE")
	}
	match := namedSourcePattern.FindStringSubmatch(sources[0])
	if match == nil {
		return nil, argumentError("argument --mutate-source: expected PATH.rb[:START-END[,START-END...]]")
	}
	source, err := normalizeImpactPath(match[1])
	if err != nil {
		return nil, argumentError("argument --mutate-source: path must be repository-relative")
	}
	if mutationTestPath(source) {
		return nil, argumentError("argument --mutate-source: " + source + " is a test file")
	}
	named := &proveNamedSource{source: source, examples: map[string][]int{}}
	if match[2] != "" {
		if named.spans, err = parseNamedSpans(match[2]); err != nil {
			return nil, err
		}
	}
	for _, example := range examples {
		if err := named.addExample(example); err != nil {
			return nil, err
		}
	}
	return named, nil
}

// parseNamedSpans reads `START-END` or `LINE` items, comma-separated, each
// 1-based and inclusive.
func parseNamedSpans(text string) ([]mutate.LineSpan, error) {
	items := strings.Split(text, ",")
	if len(items) > maxNamedSpans {
		return nil, argumentError(fmt.Sprintf("argument --mutate-source: at most %d line spans", maxNamedSpans))
	}
	spans := make([]mutate.LineSpan, 0, len(items))
	for _, item := range items {
		first, last, ranged := strings.Cut(item, "-")
		if !ranged {
			last = first
		}
		start, startErr := strconv.Atoi(first)
		end, endErr := strconv.Atoi(last)
		if startErr != nil || endErr != nil || start < 1 || end < start {
			return nil, argumentError("argument --mutate-source: line span " + item + " is not START-END with 1 <= START <= END")
		}
		spans = append(spans, mutate.LineSpan{Start: start, End: end})
	}
	return spans, nil
}

func (named *proveNamedSource) addExample(example string) error {
	match := namedExamplePattern.FindStringSubmatch(example)
	if match == nil {
		return argumentError("argument --mutate-example: expected SPEC_spec.rb:LINE[:LINE...], got " + example)
	}
	spec, err := normalizeImpactPath(match[1])
	if err != nil || !rbmutate.TestFile(spec) {
		return argumentError("argument --mutate-example: " + match[1] + " is not a repository-relative *_spec.rb file")
	}
	if _, known := named.examples[spec]; !known && len(named.examples) == maxNamedSpecFiles {
		return argumentError(fmt.Sprintf("argument --mutate-example: at most %d spec files", maxNamedSpecFiles))
	}
	lines := named.examples[spec]
	for _, word := range strings.Split(strings.TrimPrefix(match[2], ":"), ":") {
		line, err := strconv.Atoi(word)
		if err != nil || line < 1 {
			return argumentError("argument --mutate-example: line " + word + " of " + spec + " is not 1-based")
		}
		if !containsInt(lines, line) {
			lines = append(lines, line)
		}
	}
	sort.Ints(lines)
	if len(lines) > maxNamedExampleLines {
		return argumentError(fmt.Sprintf("argument --mutate-example: at most %d example lines per spec file", maxNamedExampleLines))
	}
	named.examples[spec] = lines
	return nil
}

func containsInt(values []int, value int) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// checkNamedSource refuses a selector outside --mutate's impact and change
// modes, and with --attack-tests, which is Go-only.
func checkNamedSource(parsed options) error {
	if parsed.prove.named == nil {
		return nil
	}
	if !parsed.proveMutate {
		return argumentError("argument --mutate-source: requires --mutate")
	}
	if parsed.proveAttackTests {
		return argumentError("argument --mutate-source: --attack-tests judges Go rows only")
	}
	return nil
}

// withNamedSourcePath adds the named source to path impact's positional
// paths when it is not one of them, so `prove --mutate --mutate-source S
// --mutate-example E` needs no path of its own.
func withNamedSourcePath(rewritten []string, named *proveNamedSource) []string {
	for _, argument := range rewritten {
		if argument == "--" {
			return append(append([]string{}, rewritten...), named.source)
		}
	}
	return append(append([]string{}, rewritten...), "--", named.source)
}

// namedSourceRows is one row per cited spec file, in path order, claiming
// that its cited examples cover the named source. The row is bound to the
// spec file's blob at the packet revision by citeAffectedBlobs.
func namedSourceRows(named *proveNamedSource) []proveRow {
	if named == nil {
		return nil
	}
	specs := make([]string, 0, len(named.examples))
	for spec := range named.examples {
		specs = append(specs, spec)
	}
	sort.Strings(specs)
	rows := make([]proveRow, 0, len(specs))
	for _, spec := range specs {
		lines := named.examples[spec]
		row := proveRow{
			Authority: "test-convention", Kind: kindNamedSource, Line: lines[0], Path: spec,
			Result: namedTarget(spec, lines), reason: namedSourceClaimPrefix + named.source,
			examples: lines, namedLines: named.spans, lineBasis: named.lineBasis(),
		}
		classifyTrust(&row)
		rows = append(rows, row)
	}
	return rows
}

func namedTarget(spec string, lines []int) string {
	target := spec
	for _, line := range lines {
		target += ":" + strconv.Itoa(line)
	}
	return target
}
