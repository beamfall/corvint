package main

import (
	"os"

	"github.com/Beamfall/corvint/internal/analyzernativebridge"
)

func main() {
	if analyzernativebridge.Run(os.Args[1:], "corvint-analyzer-java/v0", os.Stdin, os.Stdout, analyzernativebridge.AnalyzeJava) != nil {
		os.Exit(1)
	}
}
