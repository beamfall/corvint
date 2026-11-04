package main

import (
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"os"

	"github.com/Beamfall/corvint/internal/analyzernativebridge"
)

func main() {
	streams := &clierror.Streams{Binary: "corvint-analyzer-objective-c", Protocol: true, Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-analyzer-objective-c", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}

	if analyzernativebridge.Run(os.Args[1:], "corvint-analyzer-objective-c/v0", os.Stdin, streams.Stdout(), analyzernativebridge.AnalyzeObjectiveC) != nil {
		os.Exit(streams.Status(1))
	}
}
