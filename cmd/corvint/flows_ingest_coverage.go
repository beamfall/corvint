package main

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/appflows"
)

// runFlowsIngestCoverage writes the corvint-coverage-ingest/1 report for one per-test coverage
// file bound to run evidence at a revision (AFU-V1-112..119). It reads only and writes stdout.
func runFlowsIngestCoverage(ctx context.Context, root string, args []string, out, diag io.Writer) error {
	f := flag.NewFlagSet("flows ingest-coverage", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	in := appflows.CoverageIngestInput{Root: root}
	f.StringVar(&in.Format, "format", "", "v8, istanbul, rspec or lcov")
	f.StringVar(&in.From, "from", "", "coverage file")
	f.Func("evidence", "test-run-evidence/0 JSONL file (repeatable)", func(s string) error {
		in.Evidence = append(in.Evidence, s)
		return nil
	})
	f.StringVar(&in.Revision, "revision", "", "full commit the coverage must have run at")
	f.StringVar(&in.TestKey, "test-key", "", "test key of a single-test coverage file")
	f.StringVar(&in.Links, "links", "", "corvint-coverage-links/1 declared or supplied links")
	f.StringVar(&in.StripPrefix, "strip-prefix", "", "path prefix to remove from covered paths")
	if f.Parse(args) != nil || in.Format == "" || in.From == "" || len(in.Evidence) == 0 || in.Revision == "" || f.NArg() != 0 {
		return errors.New("flows ingest-coverage requires --format v8|istanbul|rspec|lcov --from FILE --evidence FILE --revision FULL_COMMIT")
	}
	data, err := appflows.IngestCoverage(ctx, in)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
