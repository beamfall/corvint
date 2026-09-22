package main

import (
	"os"

	"github.com/Beamfall/corvint/internal/analyzernativebridge"
)

func main() {
	if analyzernativebridge.Run(os.Args[1:], "corvint-analyzer-objective-c/v0", os.Stdin, os.Stdout, analyzernativebridge.AnalyzeObjectiveC) != nil {
		os.Exit(1)
	}
}
