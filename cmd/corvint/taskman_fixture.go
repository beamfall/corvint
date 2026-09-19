package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Beamfall/corvint/internal/taskman"
)

func runTaskmanFixture(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	flags := map[string]string{}
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) || (args[i] != "--executor" && args[i] != "--observations") || args[i+1] == "" {
			fmt.Fprintln(stderr, "taskman fixture: expected --executor ABSOLUTE_FILE --observations FILE")
			return 2
		}
		if _, exists := flags[args[i]]; exists {
			fmt.Fprintln(stderr, "taskman fixture: duplicate argument")
			return 2
		}
		flags[args[i]] = args[i+1]
	}
	if len(flags) != 2 {
		fmt.Fprintln(stderr, "taskman fixture: executor and observations are required")
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	raw, e := taskman.Preview(ctx, root, flags["--executor"], flags["--observations"])
	if e != nil {
		fmt.Fprintln(stderr, "taskman fixture:", e)
		return 2
	}
	_, e = stdout.Write(raw)
	if e != nil {
		return 2
	}
	return 0
}
