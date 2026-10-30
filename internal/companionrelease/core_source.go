package companionrelease

import (
	"context"
	"fmt"
)

// CoreSource is the hash-verified Corvint source archive of one clean
// checkout's HEAD, with the Git identity that produced it.
type CoreSource struct {
	Commit     string
	Tree       string
	GitVersion string
	Archive    []byte // the exact source/corvint-src.tar.gz bytes the companion bundle carries
}

// CoreSourceArchive exports root's HEAD with the same verified reader and
// deterministic tar.gz the companion bundle uses for source/corvint-src.tar.gz.
// It takes no Corvint Tasks input, so a Core-only candidate can carry its own
// source archive (PRS-V1-005). scratch must be an existing private directory.
func CoreSourceArchive(ctx context.Context, root, scratch string) (CoreSource, error) {
	toolchain, err := resolveToolchain(ctx, scratch)
	if err != nil {
		return CoreSource{}, err
	}
	if err := requireCleanTree(ctx, toolchain.GitPath, root, scratch); err != nil {
		return CoreSource{}, fmt.Errorf("corvint root: %w", err)
	}
	export, err := exportSource(ctx, toolchain.GitPath, root, scratch)
	if err != nil {
		return CoreSource{}, fmt.Errorf("export corvint source: %w", err)
	}
	entries := func() ([]ArchiveEntry, error) { return sourceArchiveEntries(export), nil }
	archive, err := buildTarGzTwice(entries)
	if err != nil {
		return CoreSource{}, fmt.Errorf("corvint source archive: %w", err)
	}
	if err := verifyTarGz(archive, sourceArchiveEntries(export)); err != nil {
		return CoreSource{}, fmt.Errorf("corvint source archive verification: %w", err)
	}
	return CoreSource{Commit: export.HeadCommit, Tree: export.HeadTree, GitVersion: toolchain.GitVersion, Archive: archive}, nil
}
