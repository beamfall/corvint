package releasecandidate

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

const (
	maxCandidateFiles       = 15
	maxCandidateDirectories = 5 // Includes the candidate root.
	maxCandidateBytes       = 1 << 30
	maxInstallSiblings      = 4096
)

func probeCoreVersion(ctx context.Context, binary, directory, expected string) error {
	observation := procgroup.Run(ctx, procgroup.Spec{
		Argv: []string{binary, "--version"}, Dir: directory,
		Env:     []string{"PATH=/usr/bin:/bin", "HOME=" + directory, "LANG=C", "LC_ALL=C"},
		Timeout: 30 * time.Second, ShutdownTimeout: time.Second,
		OutputLimit: 4096, StderrLimit: 4096,
	})
	if observation.Err != nil || !observation.ExitObserved || !observation.WaitCompleted || observation.ExitStatus != 0 || observation.Signal != "" || !observation.OwnedProcessGroupCleanup {
		return fmt.Errorf("version probe failed or cleanup was not observed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(string(observation.Stdout)) != expected {
		return fmt.Errorf("version identity mismatch")
	}
	return nil
}

// The closed candidate is small. ReadDir with a positive bound avoids allocating
// an unbounded hostile directory listing before the inventory limit can fire.
func readCandidateFiles(ctx context.Context, directory string) (map[string][]byte, map[string]bool, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	files := map[string][]byte{}
	directories := map[string]bool{".": true}
	pending := []string{"."}
	remaining := int64(maxCandidateBytes)
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		name := pending[0]
		pending = pending[1:]
		handle, err := root.Open(name)
		if err != nil {
			return nil, nil, err
		}
		entries, readErr := handle.ReadDir(maxCandidateFiles + maxCandidateDirectories)
		closeErr := handle.Close()
		if readErr != nil && readErr != io.EOF {
			return nil, nil, readErr
		}
		if closeErr != nil {
			return nil, nil, closeErr
		}
		if len(entries) >= maxCandidateFiles+maxCandidateDirectories {
			return nil, nil, fmt.Errorf("candidate entry count exceeds bound")
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
			relative := filepath.Join(name, entry.Name())
			info, err := entry.Info()
			if err != nil {
				return nil, nil, err
			}
			if info.IsDir() {
				if name != "." || len(directories) >= maxCandidateDirectories {
					return nil, nil, fmt.Errorf("candidate directory inventory exceeds bound")
				}
				directories[filepath.ToSlash(relative)] = true
				pending = append(pending, relative)
				continue
			}
			if !info.Mode().IsRegular() || len(files) >= maxCandidateFiles {
				return nil, nil, fmt.Errorf("candidate file inventory is invalid")
			}
			raw, err := readCandidateFile(root, relative, info, min(int64(maxInputBytes), remaining))
			if err != nil {
				return nil, nil, err
			}
			remaining -= int64(len(raw))
			files[filepath.ToSlash(relative)] = raw
		}
	}
	return files, directories, nil
}

func readCandidateFile(root *os.Root, name string, expected os.FileInfo, limit int64) ([]byte, error) {
	if expected.Size() < 0 || expected.Size() > limit {
		return nil, fmt.Errorf("candidate file exceeds byte bound")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	actual, err := file.Stat()
	if err != nil || !actual.Mode().IsRegular() || !os.SameFile(expected, actual) {
		return nil, fmt.Errorf("candidate file changed during admission")
	}
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(raw)) > limit || int64(len(raw)) != expected.Size() {
		return nil, fmt.Errorf("candidate file changed or exceeded byte bound")
	}
	return raw, nil
}

// Installation assumes an exclusively owned local store. These checks refuse
// static aliases; they do not promise a transaction against concurrent renames.
func validateInstallStore(ctx context.Context, candidate, store, target string) error {
	resolved, err := resolveProspective(target)
	if err != nil {
		return err
	}
	if resolved != target {
		return fmt.Errorf("install store must be canonical and contain no symlink components")
	}
	input, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return err
	}
	if pathOverlap(strings.ToLower(input), strings.ToLower(store)) {
		return fmt.Errorf("install store overlaps retained candidate")
	}
	for current := target; ; current = filepath.Dir(current) {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && !info.IsDir() {
			return fmt.Errorf("install store component is not a real directory")
		}
		if err := refuseInstallAlias(filepath.Dir(current), filepath.Base(current)); err != nil {
			return err
		}
		if current == store {
			break
		}
	}
	return nil
}

func refuseInstallAlias(parent, name string) error {
	directory, err := os.Open(parent)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer directory.Close()
	entries, err := directory.ReadDir(maxInstallSiblings + 1)
	if err != nil && err != io.EOF {
		return err
	}
	if len(entries) > maxInstallSiblings {
		return fmt.Errorf("install parent entry count exceeds bound")
	}
	for _, entry := range entries {
		if entry.Name() != name && strings.EqualFold(entry.Name(), name) {
			return fmt.Errorf("install path has a case-fold alias")
		}
	}
	return nil
}
