package main

import (
	"os"

	"github.com/Beamfall/corvint/internal/analyzernativebridge"
)

func main() {
	if analyzernativebridge.Run(os.Args[1:], "corvint-analyzer-c-jni/v0", os.Stdin, os.Stdout, analyzernativebridge.AnalyzeC) != nil {
		os.Exit(1)
	}
}
