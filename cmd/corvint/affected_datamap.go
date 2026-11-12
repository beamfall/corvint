// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/noncodemap"
)

// parseAffectedProposeDataMapOptions accepts `--propose-data-map RECORDED`
// (or `--propose-data-map=RECORDED`) alone (NCM-V1-003).
func parseAffectedProposeDataMapOptions(rest []string) (affectedInvocation, error) {
	if len(rest) == 0 {
		return affectedInvocation{}, argumentError("--propose-data-map requires exactly one value")
	}
	_, value, inline := strings.Cut(rest[0], "=")
	rest = rest[1:]
	if !inline {
		if len(rest) == 0 {
			return affectedInvocation{}, argumentError("--propose-data-map requires exactly one value")
		}
		value, rest = rest[0], rest[1:]
	}
	if value == "" {
		return affectedInvocation{}, argumentError("--propose-data-map requires a recorded-reads file")
	}
	if len(rest) != 0 {
		return affectedInvocation{}, argumentError("affected --propose-data-map takes no other argument: " + rest[0])
	}
	return affectedInvocation{ProposeDataMap: value, NoCache: true}, nil
}

// runAffectedProposeDataMap writes to stdout a corvint-data-map/0 document
// proposing one entry per path the recorded reads name (NCM-V1-003). It first
// reads the repository's map, and refuses an invalid one with
// data-map-invalid and exit 2 (NCM-V1-007); it writes no file.
func runAffectedProposeDataMap(invocation affectedInvocation, stdout, stderr io.Writer) int {
	if _, err := noncodemap.Read(invocation.Root); err != nil {
		emitError(stderr, &gokernel.Error{Code: noncodemap.CodeInvalid, Message: err.Error()})
		return 2
	}
	name := invocation.ProposeDataMap
	if !filepath.IsAbs(name) {
		name = filepath.Join(invocation.Root, name)
	}
	file, err := os.Open(name)
	if err != nil {
		emitError(stderr, argumentError(fmt.Sprintf("--propose-data-map: cannot read %s: %v", invocation.ProposeDataMap, err)))
		return 2
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		emitError(stderr, argumentError("--propose-data-map: "+invocation.ProposeDataMap+" is not a regular file"))
		return 2
	}
	proposal, err := noncodemap.Propose(file, filepath.ToSlash(invocation.ProposeDataMap))
	if err != nil {
		emitError(stderr, argumentError("--propose-data-map: "+err.Error()))
		return 2
	}
	if _, err := stdout.Write(proposal); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write the data-map proposal"})
		return 2
	}
	return 0
}
