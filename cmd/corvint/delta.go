package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Beamfall/corvint/internal/delta"
	"github.com/Beamfall/corvint/internal/extevidence"
	"io"
	"os"
	"strings"
)

// parseDeltaInvocation keeps public root/value handling outside the immutable compiler.
func parseDeltaInvocation(arguments []string) (string, []string, bool, error) {
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
	if index >= len(arguments) || arguments[index] != "delta" {
		return "", nil, false, nil
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", nil, true, argumentError("cannot resolve current directory")
		}
	}
	root, err := normalizeRoot(root)
	if err != nil {
		return "", nil, true, err
	}
	rest := arguments[index+1:]
	for i := 0; i < len(rest); i++ {
		name, _, inline := strings.Cut(rest[i], "=")
		switch strings.TrimPrefix(strings.TrimPrefix(name, "-"), "-") {
		case "base", "head", "previous-generation", "work-key-pattern", "provider", "checkout":
			if !inline {
				if i+1 >= len(rest) || argparseOptionLike(rest[i+1]) {
					return "", nil, true, argumentError("missing delta option value")
				}
				i++
			}
		}
	}
	return root, rest, true, nil
}

// runDelta emits only the compiler's closed record or a bounded refusal code.
func runDelta(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("delta", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	o := delta.Options{Build: build}
	f.StringVar(&o.Base, "base", "", "full base commit")
	f.StringVar(&o.Head, "head", "", "full head commit")
	f.StringVar(&o.PreviousGeneration, "previous-generation", "", "prior flow generation")
	f.StringVar(&o.WorkKeyPattern, "work-key-pattern", "", "opaque metadata key pattern")
	f.Func("provider", "local provider record", func(s string) error { o.Providers = append(o.Providers, s); return nil })
	f.Func("checkout", "repository-id=local-path", func(s string) error {
		id, path, ok := strings.Cut(s, "=")
		if !ok || id == "" || path == "" {
			return fmt.Errorf("invalid checkout")
		}
		o.Checkouts = append(o.Checkouts, extevidence.Checkout{ID: id, Source: path})
		return nil
	})
	if err := f.Parse(args); err != nil || f.NArg() != 0 {
		fmt.Fprintln(diagnostic, "delta-invalid-arguments")
		return 2
	}
	result, err := delta.Compile(ctx, root, o)
	if err != nil {
		fmt.Fprintln(diagnostic, "delta-unavailable")
		return 2
	}
	raw, err := result.Canonical()
	if err != nil {
		fmt.Fprintln(diagnostic, "delta-record-invalid")
		return 2
	}
	if _, err = out.Write(append(raw, '\n')); err != nil {
		return 1
	}
	return 0
}
