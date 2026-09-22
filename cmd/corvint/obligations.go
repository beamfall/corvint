package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// obligationsInputCeiling bounds each sidecar input read: the CEM raw ceiling
// is 4 MiB and an impact receipt never exceeds it (EFO-V0-001).
const obligationsInputCeiling = 4 << 20

// obligationsOptions is the whole argument surface of `corvint obligations`.
// The verb reads two files and no repository, so it takes no --root.
type obligationsOptions struct {
	cem, impact string
	limit       int
}

// parseObligationsInvocation reports whether argv is an obligations run and
// its parsed options (EFO-V0-001).
func parseObligationsInvocation(arguments []string) (obligationsOptions, bool, error) {
	if len(arguments) == 0 || arguments[0] != "obligations" {
		return obligationsOptions{}, false, nil
	}
	options := obligationsOptions{limit: 64}
	values := map[string]*string{"--cem": &options.cem, "--impact": &options.impact}
	limit := ""
	values["--limit"] = &limit
	rest := arguments[1:]
	for len(rest) > 0 {
		flag, inline, hasInline := strings.Cut(rest[0], "=")
		target, known := values[flag]
		if !known {
			return options, true, argumentError(fmt.Sprintf("unrecognized arguments: %s", rest[0]))
		}
		if *target != "" {
			return options, true, argumentError(fmt.Sprintf("argument %s: given twice", flag))
		}
		if !hasInline {
			if len(rest) < 2 || rest[1] == "" {
				return options, true, argumentError(fmt.Sprintf("argument %s: requires exactly one value", flag))
			}
			inline, rest = rest[1], rest[1:]
		}
		if inline == "" {
			return options, true, argumentError(fmt.Sprintf("argument %s: requires exactly one value", flag))
		}
		*target, rest = inline, rest[1:]
	}
	if options.cem == "" || options.impact == "" {
		return options, true, argumentError("obligations requires --cem FILE and --impact FILE")
	}
	if limit != "" {
		parsed, err := strconv.Atoi(limit)
		if err != nil || parsed < 1 {
			return options, true, argumentError("argument --limit: must be a positive integer")
		}
		options.limit = parsed
	}
	return options, true, nil
}

// runObligations composes and writes the sidecar. It reads exactly the two
// named files and touches nothing else (EFO-V0-001, EFO-V0-007).
func runObligations(options obligationsOptions, stdout, stderr io.Writer) int {
	cem, err := readObligationsInput("cem", options.cem)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	impact, err := readObligationsInput("impact", options.impact)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	document, err := extevidence.Obligations(cem, impact, options.limit)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	encoded, err := gokernel.CanonicalJSON(document)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write obligations sidecar"})
		return 2
	}
	return 0
}

func readObligationsInput(name, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &gokernel.Error{Code: "invalid-obligations-input", Message: fmt.Sprintf("cannot read %s input", name)}
	}
	if len(data) > obligationsInputCeiling {
		return nil, &gokernel.Error{Code: "invalid-obligations-input", Message: fmt.Sprintf("%s input exceeds %d bytes", name, obligationsInputCeiling)}
	}
	return data, nil
}
