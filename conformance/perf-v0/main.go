package main

import (
	"context"
	"errors"
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, runCLI(context.Background(), os.Args[1:]))
	os.Exit(2)
}

// Historical registrations remain evidence; none can execute a retired oracle.
func runCLI(context.Context, []string) error {
	return errors.New("retired-python-protocol: decision 0088 cancelled the paired performance runner; historical receipts are unchanged")
}
