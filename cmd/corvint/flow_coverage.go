package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Beamfall/corvint/internal/flowcoverage"
	"io"
)

func runFlowCoverage(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("flows coverage", flag.ContinueOnError)
	f.SetOutput(diagnostic)
	d := f.String("denominator", "", "committed denominator path")
	runs := f.String("receipts", "", "committed run inventory path")
	rev := f.String("revision", "", "immutable target revision (default HEAD)")
	write := f.String("write-back", "", "fresh documentation directory")
	check := f.String("check", "", "read-only documentation check")
	offset := f.Int("offset", 0, "full-report row offset")
	limit := f.Int("limit", 20, "page rows (1..100)")
	if f.Parse(args) != nil || f.NArg() != 0 || *d == "" || *runs == "" || (*write != "" && *check != "") {
		fmt.Fprintln(diagnostic, "coverage requires --denominator FILE --receipts FILE; --write-back and --check are exclusive")
		return 2
	}
	r, e := flowcoverage.Compile(ctx, root, flowcoverage.Options{Denominator: *d, Receipts: *runs, Revision: *rev})
	if e != nil {
		fmt.Fprintln(diagnostic, e)
		return 2
	}
	b, e := r.Page(*offset, *limit)
	if e != nil {
		fmt.Fprintln(diagnostic, e)
		return 2
	}
	if *write != "" {
		e = r.Write(*write)
	}
	changed := false
	if *check != "" {
		var diff []string
		diff, e = r.Check(*check)
		changed = len(diff) > 0
		for _, p := range diff {
			fmt.Fprintln(diagnostic, "changed:", p)
		}
	}
	if e != nil {
		fmt.Fprintln(diagnostic, e)
		return 2
	}
	fmt.Fprintln(out, string(b))
	if !r.Report.Passed || changed {
		return 1
	}
	return 0
}
