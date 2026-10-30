package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/toolchainreceipt"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(arguments []string, output, errors io.Writer) int {
	if len(arguments) != 1 {
		fmt.Fprintln(errors, "usage: corvint-go-toolchain-receipt GOROOT")
		return 2
	}
	result, err := toolchainreceipt.Tree(arguments[0])
	if err != nil {
		fmt.Fprintln(errors, err)
		return 1
	}
	if err := json.NewEncoder(output).Encode(result); err != nil {
		fmt.Fprintln(errors, err)
		return 1
	}
	return 0
}
