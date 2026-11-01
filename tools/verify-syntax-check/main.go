// SPDX-License-Identifier: Apache-2.0
// verify-syntax-check refuses a dogfood VERIFY file finish-lane would refuse,
// with finish-lane's own lines, before script/dogfood-finish.sh builds or reads
// the tree (DCW-V0-753):
//
//	go run ./tools/verify-syntax-check --prefix dogfood-inputs-check --base SHA FILE
//
// Exit 0 admits the file, exit 2 refuses it.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Beamfall/corvint/internal/dogfoodflow"
)

func main() {
	flags := flag.NewFlagSet("verify-syntax-check", flag.ContinueOnError)
	prefix := flags.String("prefix", "dogfood-finish-lane", "the refusal line prefix")
	base := flags.String("base", "", "the BASE the fix line names")
	if flags.Parse(os.Args[1:]) != nil || flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: verify-syntax-check [--prefix NAME] [--base SHA] VERIFY_FILE")
		os.Exit(2)
	}
	code, err := dogfoodflow.CheckVerifySyntax(context.Background(), flags.Arg(0), *base, *prefix, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "verify-syntax-check:", err)
		os.Exit(2)
	}
	os.Exit(code)
}
