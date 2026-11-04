// corvint-analyzer-swift is an unregistered experimental fact extractor.
package main

import (
	"github.com/Beamfall/corvint/internal/analyzerswift"
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"os"
)

func main() {
	streams := &clierror.Streams{Binary: "corvint-analyzer-swift", Protocol: true, Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-analyzer-swift", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}
	os.Exit(streams.Status(analyzerswift.Run(os.Args[1:], streams.Stdout())))
}
