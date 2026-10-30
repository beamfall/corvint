package companionrelease

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	maxPackageFiles = 512
	maxPackageBytes = 8 << 20
)

func exportedPackageTree(export Export, prefix, sourcePath, sourceDigest string) ([]ArchiveEntry, ArtifactManifest, error) {
	name := strings.TrimSuffix(strings.TrimPrefix(prefix, "integrations/"), "/")
	var files []ArchiveEntry
	h := sha256.New()
	total := 0
	for _, source := range export.Files {
		if !strings.HasPrefix(source.Path, prefix) {
			continue
		}
		rel := strings.TrimPrefix(source.Path, prefix)
		if rel == "" {
			return nil, ArtifactManifest{}, fmt.Errorf("host package %s has an empty member", name)
		}
		mode := int64(0o644)
		if source.Mode == "100755" {
			mode = 0o755
		}
		path := "plugins/" + name + "/" + rel
		if err := refuseUnsafePath(path); err != nil {
			return nil, ArtifactManifest{}, fmt.Errorf("host package %s: %w", name, err)
		}
		files = append(files, ArchiveEntry{Path: path, Mode: mode, Data: source.Data})
		total += len(source.Data)
		fmt.Fprintf(h, "%s\x00%06o\x00%d\x00", path, mode, len(source.Data))
		_, _ = h.Write(source.Data)
		if len(files) > maxPackageFiles || total > maxPackageBytes {
			return nil, ArtifactManifest{}, fmt.Errorf("host package %s exceeds %d files or %d bytes", name, maxPackageFiles, maxPackageBytes)
		}
	}
	if len(files) == 0 {
		return nil, ArtifactManifest{}, fmt.Errorf("host package %s is absent from exported source", name)
	}
	names := make([]string, len(files))
	for i := range files {
		names[i] = files[i].Path
	}
	if err := refuseCaseFoldCollisions(names); err != nil {
		return nil, ArtifactManifest{}, fmt.Errorf("host package %s: %w", name, err)
	}
	return files, ArtifactManifest{
		Name: name, Kind: "host-package-tree", Path: "plugins/" + name + "/",
		SHA256: hex.EncodeToString(h.Sum(nil)), SizeBytes: int64(total), Commit: export.HeadCommit,
		Tree: export.HeadTree, SourceArchivePath: sourcePath, SourceArchiveSHA256: sourceDigest, Support: "FALLBACK",
	}, nil
}
