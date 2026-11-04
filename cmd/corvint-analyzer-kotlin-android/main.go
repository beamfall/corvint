package main

import (
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/analyzerkotlinandroid"
)

func main() {
	streams := &clierror.Streams{Binary: "corvint-analyzer-kotlin-android", Protocol: true, Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-analyzer-kotlin-android", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}

	if run(os.Args[1:], os.Stdin, streams.Stdout()) != 0 {
		os.Exit(streams.Status(2))
	}
}

// An unexpected argv rejects like a malformed request instead of silently
// analyzing stdin, matching the sibling candidates in the same
// corvint-analyzer-candidate/experimental family (ACP-011).
func run(args []string, reader io.Reader, writer io.Writer) int {
	if len(args) != 0 {
		output, err := analyzerkotlinandroid.Process(nil)
		if err != nil {
			return 2
		}
		if err = writeAll(writer, output); err != nil {
			return 2
		}
		return 0
	}
	if input, ok := reader.(*os.File); ok {
		return runFile(input, writer)
	}
	output, err := analyzerkotlinandroid.ProcessReader(reader)
	if err != nil {
		return 2
	}
	if err = writeAll(writer, output); err != nil {
		return 2
	}
	return 0
}

func runFile(input *os.File, writer io.Writer) int {
	output, err := analyzerkotlinandroid.ProcessFile(input)
	if err != nil {
		return 2
	}
	if err = writeAll(writer, output); err != nil {
		return 2
	}
	return 0
}

func writeAll(writer io.Writer, output []byte) error {
	for len(output) > 0 {
		n, err := writer.Write(output)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(output) {
			return io.ErrShortWrite
		}
		output = output[n:]
	}
	return nil
}
