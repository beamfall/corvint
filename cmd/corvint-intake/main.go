// corvint-intake validates reader output without fetching or exposing raw context.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/intake"
)

func run(args []string, in io.Reader, out, stderr io.Writer) int {
	f := flag.NewFlagSet("corvint-intake", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	if len(args) == 1 && args[0] == "--help" {
		fmt.Fprintln(out, "Experimental local intake: validate --root PATH --base FULL_OID --head FULL_OID < candidate.json; reader-check --root PATH --raw FILE --output FILE. Output never grants authority. Host confinement/reader extraction NOT_OBSERVED.")
		return 0
	}
	if len(args) < 1 {
		fmt.Fprintln(stderr, "INTAKE_ARGUMENTS")
		return 2
	}
	root := f.String("root", "", "authoring repository")
	base := f.String("base", "", "trusted base pin")
	head := f.String("head", "", "trusted head pin")
	raw := f.String("raw", "", "external raw file")
	output := f.String("output", "", "external fresh intake file")
	if f.Parse(args[1:]) != nil || f.NArg() != 0 || *root == "" {
		fmt.Fprintln(stderr, "INTAKE_ARGUMENTS")
		return 2
	}
	if args[0] == "reader-check" {
		if *raw == "" || *output == "" || *base != "" || *head != "" {
			fmt.Fprintln(stderr, "INTAKE_ARGUMENTS")
			return 2
		}
		if err := intake.PreflightReader(context.Background(), intake.ReaderBoundary{AuthorRoot: *root, RawInput: *raw, IntakeOutput: *output}); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if json.NewEncoder(out).Encode(map[string]string{"profile": "corvint-intake-reader-preflight/0", "boundary": "OBSERVED", "hostConfinement": "NOT_OBSERVED", "readerExtraction": "NOT_OBSERVED"}) != nil {
			return 2
		}
		return 0
	}
	if args[0] != "validate" || *base == "" || *head == "" || *raw != "" || *output != "" {
		fmt.Fprintln(stderr, "INTAKE_ARGUMENTS")
		return 2
	}
	data, err := io.ReadAll(io.LimitReader(in, intake.MaxBytes+1))
	if err != nil {
		fmt.Fprintln(stderr, "INTAKE_INPUT")
		return 2
	}
	admitted, err := intake.BuildAuthorInput(context.Background(), *root, *base, *head, data)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if _, err = out.Write(admitted); err != nil {
		return 2
	}
	return 0
}
func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
