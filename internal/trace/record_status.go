package trace

import (
	"fmt"
	"os"
	"strings"
)

const recordStatusPrefix = ".context-corvint/traces/"

// IsRecordArtifactPath recognizes only the recorder's exact artifact names. It
// establishes no ownership, safety, or content validity by itself.
func IsRecordArtifactPath(path string, objectIDLength int) bool {
	name, ok := strings.CutPrefix(path, recordStatusPrefix)
	if !ok || strings.Contains(name, "/") {
		return false
	}
	if name == ".trace-operation.lock" {
		return true
	}
	if revision, ok := strings.CutSuffix(name, ".jsonl"); ok {
		return exactObjectID(revision, objectIDLength)
	}
	if parts := stagedRecordPattern.FindStringSubmatch(name); parts != nil {
		return exactObjectID(parts[1], objectIDLength)
	}
	if parts := stagedEvictionPattern.FindStringSubmatch(name); parts != nil {
		return exactObjectID(parts[1], objectIDLength) && exactObjectID(parts[3], objectIDLength)
	}
	return false
}

// ValidateRecordStatusPaths proves the read-only path prerequisites for ignoring
// untracked recorder artifacts in source-status admission. Canonical content is
// still validated by the store. ownedStage is empty except for Append's exact,
// descriptor-pinned temporary at its final prepublication stability check.
func ValidateRecordStatusPaths(root string, paths []string, objectIDLength int, ownedStage string) error {
	if len(paths) == 0 {
		return nil
	}
	if len(paths) > MaxTraceFiles+2 {
		return fmt.Errorf("local trace status exceeds the entry bound")
	}
	for _, path := range paths {
		if !IsRecordArtifactPath(path, objectIDLength) {
			return fmt.Errorf("untracked path is not a recorder artifact: %s", path)
		}
		name := strings.TrimPrefix(path, recordStatusPrefix)
		if (isStagedRecordName(name) || isStagedEvictionName(name)) && path != ownedStage {
			return fmt.Errorf("unignored pre-existing trace staging residue: %s", path)
		}
	}
	directory, err := openTraceDirectory(root, false)
	if err != nil {
		return err
	}
	defer directory.close()
	privateDirectories := func() error {
		for _, file := range []*os.File{directory.corvintFile, directory.traceFile} {
			info, err := file.Stat()
			if err != nil || info.Mode().Perm() != 0o700 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
				return fmt.Errorf("unsafe local trace directory mode")
			}
		}
		return directory.confirm()
	}
	if err := privateDirectories(); err != nil {
		return err
	}
	for _, path := range paths {
		name := strings.TrimPrefix(path, recordStatusPrefix)
		file, err := directory.openRegular(name, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		validate := func() error {
			info, err := file.Stat()
			if err != nil || info.Mode().Perm() != 0o600 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || info.Size() > MaxTraceStoreBytes {
				return fmt.Errorf("unsafe local trace file mode or size: %s", name)
			}
			if name == ".trace-operation.lock" && info.Size() != 0 {
				return fmt.Errorf("unsafe nonempty local trace operation lock")
			}
			return directory.confirmMember(name, file)
		}
		err = validate()
		if err == nil {
			err = privateDirectories()
		}
		if err == nil {
			err = validate()
		}
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return privateDirectories()
}
