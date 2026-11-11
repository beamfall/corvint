// Command requirement-links-export is the one-time export of TBC-V1-012: it reads the sealed
// change records at one commit and writes the requirement-link file.
//
//	requirement-links-export --root ABSOLUTE_ROOT --commit REVISION --out FILE
//
// It prints the run's record (input commit, output hash and counts) as JSON on standard output
// and exits 0, or 2 when the export cannot run. It is not part of the corvint binary.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/pathbrief"
)

func main() {
	root := flag.String("root", "", "absolute path of the repository top")
	commit := flag.String("commit", "", "the input commit")
	out := flag.String("out", "", "the file to write")
	flag.Parse()
	if *root == "" || *commit == "" || *out == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: requirement-links-export --root ABSOLUTE_ROOT --commit REVISION --out FILE")
		os.Exit(2)
	}
	data, stats, err := pathbrief.Export(context.Background(), *root, *commit)
	if err == nil {
		err = os.WriteFile(*out, data, 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "requirement-links-export:", err)
		os.Exit(2)
	}
	if err := gokernel.WriteCanonicalJSON(os.Stdout, stats); err != nil {
		os.Exit(2)
	}
}
