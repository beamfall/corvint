package main

import (
	"errors"
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"io"
	"os"

	"github.com/Beamfall/corvint/experimental/analyzers/sqlnative"
)

func main() {
	streams := &clierror.Streams{Binary: "corvint-analyzer-sql-native", Protocol: true, Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-analyzer-sql-native", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}
	os.Exit(streams.Status(run(os.Args[1:], os.Stdin, streams.Stdout())))
}

// An unexpected argv rejects like a malformed request instead of silently
// analyzing stdin, matching the sibling candidates in the same
// corvint-analyzer-candidate/experimental family (ACP-011).
func run(args []string, stdin io.Reader, stdout io.Writer) int {
	var input []byte
	if len(args) == 0 {
		// ACP-012: a read failure analyzes no bytes, which is the
		// NONCANONICAL_REQUEST sentinel with exit 0.
		read, err := io.ReadAll(io.LimitReader(stdin, 1500001))
		if err == nil {
			input = read
		}
	}
	if err := writeAll(stdout, sqlnative.AnalyzeFrame(input)); err != nil {
		return 2
	}
	return 0
}

func writeAll(w io.Writer, output []byte) error {
	for len(output) != 0 {
		n, err := w.Write(output)
		if n < 0 || n > len(output) {
			return errors.New("invalid output write")
		}
		if n != 0 {
			output = output[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
