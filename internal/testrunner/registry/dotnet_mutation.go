// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"fmt"
	"regexp"
	"strings"
)

// DotnetMutant is a deterministic body-edit candidate for the core harness.
// It is not a trial, a kill, or a qualification record. The caller must bind the
// source to its corpus, apply the exact edit and run FULL before counting it.
type DotnetMutant struct {
	Line          int
	Before, After string
}

var dotnetScalarReturn = regexp.MustCompile(`^([ \t]*return[ \t]+)(true|false|0|1)([ \t]*;[ \t]*)$`)

// DotnetBodyMutants offers the bounded C# scalar-return operator. Operators for
// data, directory and process classes belong to the core harness. Unsupported
// dialect bodies cannot silently shrink a claimed mutation population.
func DotnetBodyMutants(data []byte) ([]DotnetMutant, error) {
	// A raw scalar-looking line inside a multiline literal or comment is not
	// a return statement. This initial operator refuses those sources rather
	// than counting text replacement as a semantic C# mutation population.
	if strings.ContainsAny(string(data), "\"'/") {
		return nil, fmt.Errorf("dotnet-mutation-literals-or-comments-unsupported")
	}
	bodies, ok := csharpBodies(data)
	if !ok {
		return nil, fmt.Errorf("dotnet-mutation-regions-unsupported")
	}
	text := string(data)
	// Match CSharpBodies' classic-Mac normalization before using its line numbers.
	if strings.Contains(text, "\r") && !strings.Contains(text, "\n") {
		text = strings.ReplaceAll(text, "\r", "\n")
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var out []DotnetMutant
	for _, body := range bodies {
		for line := body.Start; line < body.End; line++ {
			if line < 1 || line > len(lines) {
				return nil, fmt.Errorf("dotnet-mutation-regions-unsupported")
			}
			before := lines[line-1]
			parts := dotnetScalarReturn.FindStringSubmatch(before)
			if parts == nil {
				continue
			}
			replacement := map[string]string{"true": "false", "false": "true", "0": "1", "1": "0"}[parts[2]]
			out = append(out, DotnetMutant{Line: line, Before: before, After: parts[1] + replacement + parts[3]})
		}
	}
	return out, nil
}
