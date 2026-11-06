// Package breakage owns the experimental `corvint breakage` command handlers
// (BKM-V0). The root CLI keeps the --root preamble, dispatch and help; this
// package receives the resolved root and the arguments after "breakage".
package breakage

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/breakagemap"
)

type bindings map[string]string

func (b bindings) String() string { return "explicit ID=DIR bindings" }
func (b bindings) Set(value string) error {
	id, dir, ok := strings.Cut(value, "=")
	if !ok || id == "" || dir == "" || b[id] != "" || len(b) >= 8 {
		return fmt.Errorf("invalid or repeated repository binding")
	}
	b[id] = dir
	return nil
}

// Run executes `breakage` or `breakage manifest` with the already resolved
// root and returns the process exit code.
func Run(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	if len(args) > 0 && args[0] == "manifest" {
		return runManifest(ctx, root, args[1:], out, diagnostic)
	}
	f := flag.NewFlagSet("breakage", flag.ContinueOnError)
	f.SetOutput(diagnostic)
	manifest := f.String("manifest", "", "explicit local manifest")
	api := f.String("api", "", "REPOSITORY:PATH:SYMBOL")
	base := f.String("base", "", "API repository immutable base commit")
	repositories := bindings{}
	f.Var(repositories, "repository", "ID=DIR")
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
	for id, dir := range repositories {
		if !filepath.IsAbs(dir) {
			repositories[id] = filepath.Join(root, dir)
		}
	}
	raw, err := appflows.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintln(diagnostic, "breakage: manifest unavailable")
		return 2
	}
	report, err := breakagemap.Compile(ctx, raw, repositories, *api, *base)
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

type sourceList []string

func (s *sourceList) String() string { return "explicit REPOSITORY:PATH pins" }
func (s *sourceList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// runManifest pins explicitly named sources from explicitly bound checkouts
// into a corvint-breakage-manifest/0 document (BKM-V0-010).
func runManifest(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("breakage manifest", flag.ContinueOnError)
	f.SetOutput(diagnostic)
	repositories := bindings{}
	var sources sourceList
	f.Var(repositories, "repository", "ID=DIR")
	f.Var(&sources, "source", "REPOSITORY:PATH[:START-END]")
	skeleton := f.Bool("provider-skeleton", false, "emit an empty external-evidence-provider/3 record whose repositories match the manifest")
	if err := f.Parse(args); err != nil {
		return 2
	}
	if f.NArg() != 0 || len(repositories) == 0 || len(sources) == 0 {
		fmt.Fprintln(diagnostic, "breakage manifest requires --repository ID=DIR and --source REPOSITORY:PATH")
		return 2
	}
	for id, dir := range repositories {
		if !filepath.IsAbs(dir) {
			repositories[id] = filepath.Join(root, dir)
		}
	}
	manifest, err := breakagemap.BuildManifestWith(ctx, repositories, sources, *skeleton)
	if err != nil {
		fmt.Fprintln(diagnostic, "breakage manifest:", err)
		return 2
	}
	if err := json.NewEncoder(out).Encode(manifest); err != nil {
		fmt.Fprintln(diagnostic, "breakage: output unavailable")
		return 2
	}
	return 0
}
