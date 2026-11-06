package main

import (
	"context"
	"io"
	"os"
	"strings"

	breakagecmd "github.com/Beamfall/corvint/internal/command/breakage"
)

func parseBreakageInvocation(arguments []string) (string, []string, bool, error) {
	index, root := 0, ""
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", nil, false, nil
			}
			root = arguments[index+1]
			index += 2
		} else {
			root = strings.TrimPrefix(arguments[index], "--root=")
			index++
		}
	}
	if index >= len(arguments) || arguments[index] != "breakage" {
		return "", nil, false, nil
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", nil, true, argumentError("cannot resolve current directory")
		}
	} else {
		var err error
		root, err = normalizeRoot(root)
		if err != nil {
			return "", nil, true, err
		}
	}
	return root, arguments[index+1:], true, nil
}

// runBreakage keeps the root dispatch seam; the command handlers live in
// internal/command/breakage.
func runBreakage(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	return breakagecmd.Run(ctx, root, args, out, diagnostic)
}
