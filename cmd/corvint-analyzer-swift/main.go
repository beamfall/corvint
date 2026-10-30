// corvint-analyzer-swift is an unregistered experimental fact extractor.
package main

import (
	"github.com/Beamfall/corvint/internal/analyzerswift"
	"os"
)

func main() { os.Exit(analyzerswift.Run(os.Args[1:], os.Stdout)) }
