package main

import (
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"os"

	"github.com/Beamfall/corvint/internal/analyzernativebridge"
)

func main() {
	streams := &clierror.Streams{Binary: "corvint-analyzer-java", Protocol: true, Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-analyzer-java", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}

	if analyzernativebridge.Run(os.Args[1:], "corvint-analyzer-java/v0", os.Stdin, streams.Stdout(), analyzernativebridge.AnalyzeJava) != nil {
		os.Exit(streams.Status(1))
	}
}
