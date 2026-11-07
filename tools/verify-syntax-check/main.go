// SPDX-License-Identifier: Apache-2.0
// verify-syntax-check refuses a dogfood VERIFY file finish-lane would refuse,
// with finish-lane's own lines, before script/dogfood-finish.sh builds or reads
// the tree (DWC-V0-753):
//
//	go run ./tools/verify-syntax-check --prefix dogfood-inputs-check --base SHA [--entry TARGET] [--citations F] [--intents F] [--outcome O] FILE
//
// It takes the wrapper's inputs-check flags, so its rerun hints name the same
// command (DCW-V0-730); --verify and --sealed are accepted and unused, since
// FILE is the verify input. Exit 0 admits the file, exit 2 refuses it.
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
	entry := flags.String("entry", "", "the make target the wrapper ran")
	citations := flags.String("citations", "", "the citation plan the fix line names")
	intents := flags.String("intents", "", "the intent manifest the fix line names")
	outcome := flags.String("outcome", "", "the outcome the fix line names")
	flags.String("verify", "", "accepted and unused: VERIFY_FILE is the verify input")
	flags.String("sealed", "", "accepted and unused")
	if flags.Parse(os.Args[1:]) != nil || flags.NArg() != 1 || (*entry != "" && !dogfoodflow.ValidFinishLaneEntry(*entry)) {
		fmt.Fprintln(os.Stderr, "usage: verify-syntax-check [--prefix NAME] [--base SHA] [--entry TARGET] [--citations F] [--intents F] [--outcome O] VERIFY_FILE")
		os.Exit(2)
	}
	options := dogfoodflow.FinishLaneOptions{Entry: *entry, Change: dogfoodflow.ChangeOptions{
		Base: *base, Citations: *citations, IntentsFile: *intents, VerifyFile: flags.Arg(0), Outcome: *outcome,
	}}
	code, err := dogfoodflow.CheckVerifySyntax(context.Background(), options, *prefix, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "verify-syntax-check:", err)
		os.Exit(2)
	}
	os.Exit(code)
}
