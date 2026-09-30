package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/breakagemap"
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

type breakageBindings map[string]string

func (b breakageBindings) String() string { return "explicit ID=DIR bindings" }
func (b breakageBindings) Set(value string) error {
	id, dir, ok := strings.Cut(value, "=")
	if !ok || id == "" || dir == "" || b[id] != "" || len(b) >= 8 {
		return fmt.Errorf("invalid or repeated repository binding")
	}
	b[id] = dir
	return nil
}

// runBreakage is an additive experimental command. The root integration owns
// dispatch and help registration; this function is also exercised directly.
func runBreakage(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("breakage", flag.ContinueOnError)
	f.SetOutput(diagnostic)
	manifest := f.String("manifest", "", "explicit local manifest")
	api := f.String("api", "", "REPOSITORY:PATH:SYMBOL")
	base := f.String("base", "", "API repository immutable base commit")
	bindings := breakageBindings{}
	f.Var(bindings, "repository", "ID=DIR")
	if err := f.Parse(args); err != nil {
		return 2
	}
	if f.NArg() != 0 || *manifest == "" || *api == "" {
		fmt.Fprintln(diagnostic, "breakage requires --manifest FILE --api REPOSITORY:PATH:SYMBOL and explicit --repository ID=DIR bindings")
		return 2
	}
	manifestPath := *manifest
	if !filepath.IsAbs(manifestPath) {
		manifestPath = filepath.Join(root, manifestPath)
	}
	for id, dir := range bindings {
		if !filepath.IsAbs(dir) {
			bindings[id] = filepath.Join(root, dir)
		}
	}
	raw, err := appflows.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintln(diagnostic, "breakage: manifest unavailable")
		return 2
	}
	report, err := breakagemap.Compile(ctx, raw, bindings, *api, *base)
	if err != nil {
		fmt.Fprintln(diagnostic, "breakage:", err)
		return 2
	}
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(diagnostic, "breakage: output unavailable")
		return 2
	}
	return 0
}
