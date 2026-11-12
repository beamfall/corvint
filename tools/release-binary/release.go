// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const releaseRunSchema = "corvint.release-run/1"

// ReleaseRun is what `release` prints: the tag it ran, the binaries it built and the verified
// manifest it left in the output directory (ARTIFACT-BIN-V0-523).
type ReleaseRun struct {
	Schema         string   `json:"schema"`
	Tag            string   `json:"tag"`
	Scope          string   `json:"scope"`
	Version        string   `json:"version"`
	Binaries       []string `json:"binaries"`
	ManifestSHA256 string   `json:"manifestSha256"`
	Rechecked      []string `json:"rechecked"`
}

// ReleaseOptions are the inputs of one tag's release run.
type ReleaseOptions struct {
	Root         string
	Catalog      Catalog
	Tag          string
	Output       string
	Base         string
	Declarations []SkewDeclaration
}

// runRelease turns one tag into a verified output directory: it selects the tag's binaries,
// builds each of them and nothing else, merges their entries into the release manifest and
// rechecks the directory. DIR must be absent or empty, so the manifest merges only this run's
// entries, and any failure leaves DIR as the run found it (ARTIFACT-BIN-V0-523, -524).
func runRelease(ctx context.Context, options ReleaseOptions) (run ReleaseRun, err error) {
	selection, err := selectTag(options.Catalog, options.Tag)
	if err != nil {
		return ReleaseRun{}, err
	}
	if err := refuseOutputInsideRepository(options.Root, options.Output); err != nil {
		return ReleaseRun{}, err
	}
	created, err := claimEmptyOutput(options.Output)
	if err != nil {
		return ReleaseRun{}, err
	}
	// written names every file a completed step left; a failing step removes its own partial
	// output (ARTIFACT-BIN-V0-007), so this is all the run has to undo.
	var written []string
	defer func() {
		if err != nil {
			rollBackOutput(options.Output, written, created)
		}
	}()
	for _, name := range selection.Binaries {
		if err = ctx.Err(); err != nil {
			return ReleaseRun{}, err
		}
		entry, buildErr := runBuild(ctx, BuildOptions{Root: options.Root, Catalog: options.Catalog, Binary: name, Version: selection.Version, Output: options.Output})
		if buildErr != nil {
			err = fmt.Errorf("%s: %w", name, buildErr)
			return ReleaseRun{}, err
		}
		written = append(written, name+".SHA256SUMS", name+entrySuffix)
		for _, archive := range entry.Archives {
			written = append(written, archive.Name)
		}
	}
	manifest, err := runManifest(options.Catalog, options.Root, options.Output, options.Base, options.Declarations)
	if err != nil {
		return ReleaseRun{}, err
	}
	written = append(written, manifestName, manifestName+".sha256")
	raw, err := canonicalJSON(manifest)
	if err != nil {
		return ReleaseRun{}, err
	}
	rechecked, err := verifyOutput(manifest, raw, options.Output)
	if err != nil {
		return ReleaseRun{}, err
	}
	return ReleaseRun{Schema: releaseRunSchema, Tag: selection.Tag, Scope: selection.Scope, Version: selection.Version, Binaries: selection.Binaries, ManifestSHA256: digestOf(raw), Rechecked: rechecked}, nil
}

// claimEmptyOutput requires DIR to be absent or an empty directory and reports whether this run
// created it. It refuses `output-not-empty` before anything is built.
func claimEmptyOutput(directory string) (created bool, err error) {
	entries, err := os.ReadDir(directory)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return false, err
		}
		return true, nil
	case err != nil:
		return false, err
	case len(entries) != 0:
		return false, fmt.Errorf("output-not-empty: %s holds %d entries; a release run writes into an absent or empty directory so no other run's entry reaches its manifest", directory, len(entries))
	}
	return false, nil
}

// rollBackOutput removes the files a failed run wrote into the directory it claimed empty, and
// the directory itself when the run created it and nothing else has appeared there since.
func rollBackOutput(directory string, written []string, created bool) {
	for index := len(written) - 1; index >= 0; index-- {
		os.Remove(filepath.Join(directory, written[index]))
	}
	if created {
		os.Remove(directory)
	}
}
