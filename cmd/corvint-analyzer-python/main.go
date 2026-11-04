package main

import (
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/analyzerpython"
)

func main() {
	streams := &clierror.Streams{Binary: "corvint-analyzer-python", Protocol: true, Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-analyzer-python", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}

	if run(os.Args[1:], os.Stdin, streams.Stdout()) != 0 {
		os.Exit(streams.Status(2))
	}
}

// run deliberately owns only inherited standard streams. It opens no caller
// path and the analyzer itself has no ambient filesystem, process, or network
// dependency. An unexpected argv rejects like a malformed request instead of
// silently analyzing stdin, matching the sibling candidates in the same
// corvint-analyzer-candidate/experimental family (ACP-011).
func run(args []string, reader io.Reader, writer io.Writer) int {
	if len(args) != 0 {
		if err := writeAll(writer, analyzerpython.Process(nil)); err != nil {
			return 2
		}
		return 0
	}
	frame, err := io.ReadAll(io.LimitReader(reader, analyzerpython.MaxRequestBytes+1))
	if err != nil {
		frame = nil
	}
	if err := writeAll(writer, analyzerpython.Process(frame)); err != nil {
		return 2
	}
	return 0
}

func writeAll(writer io.Writer, value []byte) error {
	for len(value) > 0 {
		written, err := writer.Write(value)
		if err != nil {
			return err
		}
		if written <= 0 || written > len(value) {
			return io.ErrShortWrite
		}
		value = value[written:]
	}
	return nil
}
