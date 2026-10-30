// SPDX-License-Identifier: Apache-2.0
// dogfood-guard-hunks prints the number of hunks in a CEM map that the
// guarding-test search (TCQ-V0-062) would select, by the same rule as the
// dogfood change row (DCW-V0-069). script/dogfood-bind-range.sh runs it so a
// staged bind prints the same guarding-tests NOTE (DOGFOOD-BIND-008).
package main

import (
	"fmt"
	"os"

	"github.com/Beamfall/corvint/internal/dogfoodflow"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: dogfood-guard-hunks MAP")
		os.Exit(2)
	}
	count, err := dogfoodflow.GuardingTestHunksFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "dogfood-guard-hunks: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(count)
}
