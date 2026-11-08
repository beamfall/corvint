// SPDX-License-Identifier: Apache-2.0
// merge-drivers is repository development tooling, not a release executable.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/Beamfall/corvint/internal/mergedrivers"
)

const usage = `usage: merge-drivers append-union|json-union BASE OURS THEIRS [MARKER_SIZE]
       merge-drivers keep-ours BASE OURS THEIRS [MARKER_SIZE]

Git merge drivers (DWC-V0-7700..7703). Required inputs are Git's %O %A %B;
MARKER_SIZE defaults to 7. Writes only OURS. Exit 0: resolved; 1: conflict;
2: invalid invocation or I/O failure. keep-ours defers generated projections
to make spec-requirements; spec-requirements-check still rejects stale output.

Example: go run ./tools/merge-drivers append-union base.md ours.md theirs.md 7
Install locally: make merge-drivers
`

func main() { os.Exit(run(os.Args[1:], os.Stderr)) }

func run(args []string, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprint(stderr, usage)
		return 0
	}
	if len(args) < 4 || len(args) > 5 || (args[0] != "append-union" && args[0] != "json-union" && args[0] != "keep-ours") {
		fmt.Fprint(stderr, usage)
		return 2
	}
	size := 7
	if len(args) == 5 {
		var err error
		size, err = strconv.Atoi(args[4])
		if err != nil || size < 1 || size > 1024 {
			return 2
		}
	}
	if args[0] == "keep-ours" {
		return 0
	}
	var data [3][]byte
	for i, path := range args[1:4] {
		var err error
		data[i], err = os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	var result []byte
	clean := true
	var err error
	if args[0] == "append-union" {
		result, clean, err = mergedrivers.Append(context.Background(), "", data[0], data[1], data[2], size)
	} else {
		result, err = mergedrivers.JSON(data[0], data[1], data[2])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	if err == nil {
		err = os.WriteFile(args[2], result, 0o600)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if !clean {
		return 1
	}
	return 0
}
