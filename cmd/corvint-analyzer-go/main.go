// corvint-analyzer-go is an unregistered experimental fact extractor.
package main

import (
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/analyzergo"
)

func main() {
	streams := &clierror.Streams{Binary: "corvint-analyzer-go", Protocol: true, Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-analyzer-go", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}

	if run(os.Args[1:], os.Stdin, streams.Stdout()) != 0 {
		os.Exit(streams.Status(2))
	}
}

// ACP-011: unexpected argv is the NONCANONICAL_REQUEST sentinel; stdin is unread.
func run(args []string, reader io.Reader, writer io.Writer) int {
	var raw []byte
	if len(args) == 0 {
		read, err := io.ReadAll(io.LimitReader(reader, analyzergo.MaxRequestBytes+1))
		if err == nil {
			raw = read
		}
	}
	response, err := analyzergo.Process(raw)
	if err != nil {
		return 2
	}
	for len(response) > 0 {
		n, err := writer.Write(response)
		if err != nil || n <= 0 {
			return 2
		}
		response = response[n:]
	}
	return 0
}
