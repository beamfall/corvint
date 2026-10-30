//go:build unix

package main

import (
	"fmt"
	wq "github.com/Beamfall/corvint/conformance/work-queue-v0"
	"os"
)

func main() {
	if err := wq.ProseMain(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
