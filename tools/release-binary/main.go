// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
)

const usage = `usage: release-binary <command> [flags]

Build, checksum and record one Corvint product binary at a time (decision 0594 item 8).
Every command works offline on a local output directory; none tags, publishes or opens a network connection.

commands:
  check     [--root DIR] [--binaries FILE]
            Validate the catalog and require it to classify every cmd/ executable.
  select    --tag TAG [--binaries FILE]
            Print the JSON selection for a release tag: vX.Y.Z (every product binary),
            SET/vX.Y.Z (one version set) or NAME/vX.Y.Z (one binary).
  build     --binary NAME --version X.Y.Z --output DIR [--root DIR] [--binaries FILE] [--host-only]
            Build NAME for every catalog target from a clean HEAD, write NAME_<goos>_<goarch>
            archives, NAME.SHA256SUMS and NAME.release-entry.json into DIR. Nothing else is built.
  manifest  --output DIR [--base FILE] [--declare-skew SET=REASON]... [--root DIR] [--binaries FILE]
            Merge DIR/*.release-entry.json over the previous release-manifest.json in FILE,
            refuse undeclared version skew, write DIR/release-manifest.json and its .sha256.
  release   --tag TAG --output DIR [--base FILE] [--declare-skew SET=REASON]... [--root DIR] [--binaries FILE]
            Run one tag end to end: select its binaries, build each of them (and nothing else)
            into DIR, merge the release manifest over FILE and recheck DIR; print the
            corvint.release-run/1 summary. DIR must be absent or empty; a failure leaves it as found.
  verify    --manifest FILE [--output DIR] [--binaries FILE]
            Recheck a manifest's shape and skew rule and, with DIR, its digest file, every
            archive and NAME.SHA256SUMS of each recorded binary whose archives are present.

DIR must lie outside the repository; no output ever replaces an existing file.

example:
  go run ./tools/release-binary build --binary corvint-docs --version 1.0.1 --output /tmp/rel
  go run ./tools/release-binary manifest --output /tmp/rel --base previous/release-manifest.json
  go run ./tools/release-binary release --tag corvint-docs/v1.0.1 --output /tmp/rel-docs --base previous/release-manifest.json

exit status: 0 success; 2 refusal or usage error, with the reason on stderr.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

type skewFlags []SkewDeclaration

func (s *skewFlags) String() string { return fmt.Sprint(*s) }

func (s *skewFlags) Set(value string) error {
	set, reason, ok := strings.Cut(value, "=")
	if !ok || set == "" || strings.TrimSpace(reason) == "" {
		return fmt.Errorf("--declare-skew wants SET=REASON, got %q", value)
	}
	*s = append(*s, SkewDeclaration{VersionSet: set, Reason: reason})
	return nil
}

func run(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) == 0 || arguments[0] == "--help" || arguments[0] == "-h" || arguments[0] == "help" {
		fmt.Fprint(stdout, usage)
		if len(arguments) == 0 {
			return 2
		}
		return 0
	}
	command, rest := arguments[0], arguments[1:]
	flags := flag.NewFlagSet("release-binary "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "repository root (default: the working directory)")
	binaries := flags.String("binaries", "", "catalog file (default: the embedded tools/release-binary/binaries.json)")
	tag := flags.String("tag", "", "release tag to select for")
	binary := flags.String("binary", "", "product binary to build")
	version := flags.String("version", "", "release version MAJOR.MINOR.PATCH[-pre]")
	output := flags.String("output", "", "output directory")
	hostOnly := flags.Bool("host-only", false, "build only the host target (local dry run)")
	base := flags.String("base", "", "previous release-manifest.json to carry forward")
	manifestPath := flags.String("manifest", "", "release-manifest.json to verify")
	var declarations skewFlags
	flags.Var(&declarations, "declare-skew", "SET=REASON: record that this release knowingly skews SET")
	if err := flags.Parse(rest); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "release-binary %s: unexpected arguments %q\n", command, flags.Args())
		return 2
	}
	catalog, err := loadCatalog(*binaries)
	if err != nil {
		return refuse(stderr, command, err)
	}
	switch command {
	case "check":
		repository, err := resolveRoot(*root)
		if err != nil {
			return refuse(stderr, command, err)
		}
		if err := catalog.checkAgainstTree(repository); err != nil {
			return refuse(stderr, command, err)
		}
		fmt.Fprintf(stdout, "release-binary check: PASS %d binaries, %d product, %d version sets\n", len(catalog.Binaries), len(catalog.products("")), len(catalog.VersionSets))
		return 0
	case "select":
		if *tag == "" {
			return refuse(stderr, command, errors.New("--tag is required"))
		}
		selection, err := selectTag(catalog, *tag)
		if err != nil {
			return refuse(stderr, command, err)
		}
		return printJSON(stdout, stderr, command, selection)
	case "build":
		if *binary == "" || *version == "" || *output == "" {
			return refuse(stderr, command, errors.New("--binary, --version and --output are required"))
		}
		repository, err := resolveRoot(*root)
		if err != nil {
			return refuse(stderr, command, err)
		}
		entry, err := runBuild(ctx, BuildOptions{Root: repository, Catalog: catalog, Binary: *binary, Version: *version, Output: *output, HostOnly: *hostOnly})
		if err != nil {
			return refuse(stderr, command, err)
		}
		return printJSON(stdout, stderr, command, entry)
	case "manifest":
		if *output == "" {
			return refuse(stderr, command, errors.New("--output is required"))
		}
		repository, err := resolveRoot(*root)
		if err != nil {
			return refuse(stderr, command, err)
		}
		manifest, err := runManifest(catalog, repository, *output, *base, declarations)
		if err != nil {
			return refuse(stderr, command, err)
		}
		return printJSON(stdout, stderr, command, manifest)
	case "release":
		if *tag == "" || *output == "" {
			return refuse(stderr, command, errors.New("--tag and --output are required"))
		}
		if *hostOnly {
			return refuse(stderr, command, errors.New("--host-only is not a release: a release run builds every catalog target"))
		}
		repository, err := resolveRoot(*root)
		if err != nil {
			return refuse(stderr, command, err)
		}
		run, err := runRelease(ctx, ReleaseOptions{Root: repository, Catalog: catalog, Tag: *tag, Output: *output, Base: *base, Declarations: declarations})
		if err != nil {
			return refuse(stderr, command, err)
		}
		return printJSON(stdout, stderr, command, run)
	case "verify":
		if *manifestPath == "" {
			return refuse(stderr, command, errors.New("--manifest is required"))
		}
		raw, err := readBoundedJSONBytes(*manifestPath)
		if err != nil {
			return refuse(stderr, command, err)
		}
		var manifest Manifest
		if err := decodeStrict(raw, &manifest); err != nil {
			return refuse(stderr, command, fmt.Errorf("%s: %w", *manifestPath, err))
		}
		if err := manifest.validate(catalog); err != nil {
			return refuse(stderr, command, err)
		}
		if *output != "" {
			rechecked, err := verifyOutput(manifest, raw, *output)
			if err != nil {
				return refuse(stderr, command, err)
			}
			fmt.Fprintf(stdout, "release-binary verify: PASS %d binaries recorded, %d archive sets rechecked in %s: %s\n", len(manifest.Binaries), len(rechecked), *output, strings.Join(rechecked, " "))
			return 0
		}
		fmt.Fprintf(stdout, "release-binary verify: PASS %d binaries recorded\n", len(manifest.Binaries))
		return 0
	default:
		fmt.Fprintf(stderr, "release-binary: unknown command %q\n%s", command, usage)
		return 2
	}
}

// runManifest merges the output directory's entries over the base manifest, validates the
// result including the skew rule, and retains it with its digest outside the repository
// (ARTIFACT-BIN-V0-003, -004, -007).
func runManifest(catalog Catalog, root, output, base string, declarations []SkewDeclaration) (merged Manifest, err error) {
	if isRepository(root) {
		if err := refuseOutputInsideRepository(root, output); err != nil {
			return Manifest{}, err
		}
	}
	previous := Manifest{Schema: manifestSchema, Binaries: map[string]Record{}, DeclaredSkew: []SkewDeclaration{}}
	if base != "" {
		if err := readBoundedJSON(base, &previous); err != nil {
			return Manifest{}, err
		}
		if previous.Schema != manifestSchema || previous.Binaries == nil {
			return Manifest{}, fmt.Errorf("base %s: not a %s document", base, manifestSchema)
		}
	}
	entries, err := loadEntries(output, catalog)
	if err != nil {
		return Manifest{}, err
	}
	if len(entries) == 0 {
		return Manifest{}, fmt.Errorf("no-entries: %s holds no *%s", output, entrySuffix)
	}
	merged = mergeManifest(catalog, previous, entries, declarations)
	if err := merged.validate(catalog); err != nil {
		return Manifest{}, err
	}
	raw, err := canonicalJSON(merged)
	if err != nil {
		return Manifest{}, err
	}
	files := &outputs{directory: output}
	if err := files.reserve(manifestName, manifestName+".sha256"); err != nil {
		return Manifest{}, err
	}
	defer func() {
		if err != nil {
			files.discard()
		}
	}()
	if err = files.write(manifestName, raw, 0o644); err != nil {
		return Manifest{}, err
	}
	if err = files.write(manifestName+".sha256", []byte(digestOf(raw)+"  "+manifestName+"\n"), 0o644); err != nil {
		return Manifest{}, err
	}
	return merged, nil
}

// isRepository reports whether root holds a .git entry (a checkout or a linked worktree).
func isRepository(root string) bool {
	_, err := os.Lstat(filepath.Join(root, ".git"))
	return err == nil
}

// resolveRoot defaults to the working directory: the Makefile targets and the workflow run the
// tool from the repository root, and a caller elsewhere names the root explicitly.
func resolveRoot(root string) (string, error) {
	if root == "" {
		root = "."
	}
	return filepath.Abs(root)
}

func refuse(stderr io.Writer, command string, err error) int {
	fmt.Fprintf(stderr, "release-binary %s: REFUSE %v\n", command, err)
	return 2
}

func printJSON(stdout, stderr io.Writer, command string, value any) int {
	raw, err := canonicalJSON(value)
	if err != nil {
		return refuse(stderr, command, err)
	}
	if _, err := stdout.Write(raw); err != nil {
		return 1
	}
	return 0
}
