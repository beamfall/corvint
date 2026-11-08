// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AZU-V0-005 / AZU-V0-9900: the Go plugin's hidden-directory walk must not
// refuse a name no unit can claim (retained metadata below a dot directory),
// while an unrepresentable Go source there still refuses rather than
// silently narrowing the unit set.
func TestHiddenUnrepresentableNames_AZU_V0_005(t *testing.T) {
	base := fstest.MapFS{
		"go.mod":                   {Data: []byte("module example.test/m\n\ngo 1.27.3\n")},
		".github/helper/helper.go": {Data: []byte("package helper\n")},
		"embedder/embed.go":        {Data: []byte("package embedder\nimport _ \"embed\"\n//go:embed payload.txt\nvar payload string\n")},
		"embedder/payload.txt":     {Data: []byte("valid payload\n")},
	}
	unclaimed := fstest.MapFS{".metadata/name\xff.txt": {Data: []byte("x\n")}}
	for name, file := range base {
		unclaimed[name] = file
	}
	graph, err := affected.BuildFS(unclaimed, golang.New())
	if err != nil {
		t.Fatalf("unclaimed hidden name refused: %v", err)
	}
	if _, ok := graph.Unit("go:example.test/m/.github/helper"); !ok {
		t.Fatalf("hidden helper not indexed: %v", graph.UnitIDs())
	}
	if unit, ok := graph.Unit("go:example.test/m/embedder"); !ok || !slices.Equal(unit.EmbedFiles, []string{"embedder/payload.txt"}) {
		t.Fatalf("valid embed lost: %+v", unit)
	}
	// Disk and immutable walks must both ignore unrelated metadata.
	files := make(map[string]string, len(unclaimed))
	for name, file := range unclaimed {
		files[name] = string(file.Data)
	}
	if invalidNamesOnDisk() {
		root, _ := selwideFiles(t, files)
		disk, err := affected.Build(context.Background(), root, golang.New())
		if err != nil || disk.Digest() != graph.Digest() {
			t.Fatalf("disk embed graph differs: %v", err)
		}
	}
	// AFP-V0-9012: a declared output's Go suffix must not cause filename
	// validation before the output exclusion, on disk or immutable sources.
	unclaimed[affected.NonSourcePathsPath] = &fstest.MapFile{Data: []byte(`{"profile":"corvint-non-source-paths/0","entries":[{"path":".evidence/","reason":"retained output"}]}`)}
	unclaimed[".evidence/name\xff.go"] = &fstest.MapFile{Data: []byte("not Go source")}
	declaredGraph, err := affected.BuildFS(unclaimed, golang.New())
	if err != nil {
		t.Fatalf("declared hidden output refused: %v", err)
	}
	if invalidNamesOnDisk() {
		files := map[string]string{}
		for name, file := range unclaimed {
			files[name] = string(file.Data)
		}
		root, _ := selwideFiles(t, files)
		disk, err := affected.Build(context.Background(), root, golang.New())
		if err != nil || disk.Digest() != declaredGraph.Digest() {
			t.Fatalf("declared-output disk graph differs: %v", err)
		}
	}
	// Only the closed declaration profile can exempt source candidates. An
	// unknown member must not hide an otherwise unrepresentable Go filename.
	t.Run("unknown declaration form retains source claim", func(t *testing.T) {
		unknown := fstest.MapFS{}
		files := map[string]string{}
		for name, file := range unclaimed {
			unknown[name] = file
			files[name] = string(file.Data)
		}
		body := `{"profile":"corvint-non-source-paths/0","entries":[{"path":".evidence/","reason":"retained output","future":true}]}`
		unknown[affected.NonSourcePathsPath] = &fstest.MapFile{Data: []byte(body)}
		files[affected.NonSourcePathsPath] = body
		if affected.DeclaredNonSourceHolds(affected.FSSource(unknown))(".evidence/name\xff.go") {
			t.Fatal("unknown declaration exempted source")
		}
		if _, err := golang.New().UnitsSource(affected.FSSource(unknown)); !errors.Is(err, affected.ErrWalkUnrepresentable) {
			t.Fatalf("unknown declaration lost immutable source claim: %v", err)
		}
		if invalidNamesOnDisk() {
			root, _ := selwideFiles(t, files)
			if _, err := golang.New().Units(context.Background(), root); !errors.Is(err, affected.ErrWalkUnrepresentable) {
				t.Fatalf("unknown declaration lost disk source claim: %v", err)
			}
		}
	})
	claimed := fstest.MapFS{".metadata/name\xff.go": {Data: []byte("package metadata\n")}}
	for name, file := range base {
		claimed[name] = file
	}
	if _, err := affected.BuildFS(claimed, golang.New()); !errors.Is(err, affected.ErrWalkUnrepresentable) {
		t.Fatalf("claimed unrepresentable Go source: error %v, want ErrWalkUnrepresentable", err)
	}
	// An actual embed target with an invalid name still refuses, including
	// directory expansion and all: hidden-directory matches.
	for _, pattern := range []string{".assets/*", ".assets", "all:.metadata"} {
		t.Run(pattern, func(t *testing.T) {
			targets := fstest.MapFS{}
			for name, file := range base {
				targets[name] = file
			}
			targets["embedder/embed.go"] = &fstest.MapFile{Data: []byte("package embedder\nimport _ \"embed\"\n//go:embed " + pattern + "\nvar payload string\n")}
			targets["embedder/.assets/name\xff.txt"] = &fstest.MapFile{Data: []byte("x")}
			targets["embedder/.metadata/name\xff.txt"] = &fstest.MapFile{Data: []byte("x")}
			if _, err := affected.BuildFS(targets, golang.New()); !errors.Is(err, affected.ErrWalkUnrepresentable) {
				t.Fatalf("claimed unrepresentable embed: error %v, want ErrWalkUnrepresentable", err)
			}
		})
	}
}

// invalidNamesOnDisk reports a host filesystem that can create an invalid
// UTF-8 filename: Windows and macOS (APFS) refuse one.
func invalidNamesOnDisk() bool {
	return runtime.GOOS != "windows" && runtime.GOOS != "darwin"
}
