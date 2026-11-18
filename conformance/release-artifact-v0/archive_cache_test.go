// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// GAG-V0-9963, ARTIFACT-GO-V0-005: a raw build continues its own initially
// cold loose cache, never the other side's or another target's. The real Go
// builds must still produce the same bytes and clean committed identity.
func TestArchiveCacheContinuationPreservesIndependentBuilds(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/cachefixture\n\ngo 1.27.4\n")
	mustWrite(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	runGitTest(t, root, "init", "-q")
	runGitTest(t, root, "add", ".")
	runGitTest(t, root, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-qm", "fixture")
	commit, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	gitDirectory, err := gitOutput(ctx, root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := loadManifest("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest.Profile.Package = "."
	manifest.Profile.ModulePath = "example.com/cachefixture"
	target := Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	scratch := t.TempDir()
	options := Options{Root: root, Output: filepath.Join(scratch, "existing-loose-gate")}
	var looseDigest string
	seen := map[string]bool{}
	for _, side := range []string{"a", "b"} {
		cache := archiveTargetCache(scratch, target, side)
		if seen[cache] {
			t.Fatal("A and B share a cache")
		}
		seen[cache] = true
		if _, err := os.Stat(cache); !os.IsNotExist(err) {
			t.Fatalf("cache was not initially cold: %v", err)
		}
		loose := filepath.Join(scratch, "loose-"+side)
		// Use the loose builder's actual layout, independently of the archive helper.
		looseCache := filepath.Join(options.Output, "cache-"+side+"-"+target.GOOS+"-"+target.GOARCH)
		if cache != looseCache {
			t.Fatalf("raw cache %s does not continue loose cache %s", cache, looseCache)
		}
		if err := buildOnce(ctx, root, manifest, target, "1", commit, loose, looseCache, filepath.Join(scratch, "loose-tmp-"+side)); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(cache)
		if err != nil || len(entries) == 0 {
			t.Fatalf("loose build did not prime cache: %v", err)
		}
		digest, _, err := fileDigest(loose)
		if err != nil || (looseDigest != "" && digest != looseDigest) {
			t.Fatalf("independent loose builds differ: %v", err)
		}
		looseDigest = digest
		source := filepath.Join(scratch, "source-"+side)
		if err := exportRawCommit(ctx, root, scratch, commit, source); err != nil {
			t.Fatal(err)
		}
		raw := filepath.Join(scratch, "raw-"+side)
		if err := archiveBuildOnce(ctx, source, strings.TrimSpace(gitDirectory), commit, manifest, target, raw, cache, filepath.Join(scratch, "raw-tmp-"+side)); err != nil {
			t.Fatal(err)
		}
		rawDigest, _, err := fileDigest(raw)
		if err != nil || rawDigest != looseDigest {
			t.Fatalf("raw build differs from verified loose build: %v", err)
		}
		info, err := readBuildInfo(raw)
		if err != nil || info["vcs.revision"] != commit || info["vcs.modified"] != "false" {
			t.Fatalf("raw provenance changed: %v %v", info, err)
		}
	}
	for _, other := range manifest.Targets {
		if other.GOOS == target.GOOS && other.GOARCH == target.GOARCH {
			continue
		}
		for _, side := range []string{"a", "b"} {
			if seen[archiveTargetCache(scratch, other, side)] {
				t.Fatalf("target %s shares a cache", other.id())
			}
		}
	}
}
