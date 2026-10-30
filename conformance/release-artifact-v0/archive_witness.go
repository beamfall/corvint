package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func writeArchiveWitnessBestEffort(root string, witness ArchiveWitness) error {
	if witness.Revision == "" || witness.Tree == "" {
		return errors.New("verdict did not resolve an immutable revision")
	}
	content, err := canonicalJSON(witness)
	if err != nil || len(content) > maxWitnessBytes {
		return errors.New("witness exceeds its canonical bound")
	}
	home, err := os.MkdirTemp("", "corvint-release-witness-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(home)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	gitDirectoryBytes, err := closedGit(ctx, root, home, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return err
	}
	privateDirectory := filepath.Join(strings.TrimSpace(string(gitDirectoryBytes)), "corvint")
	if err := ensurePrivateDirectory(privateDirectory); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(privateDirectory, ".release-go-archive-report-")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	_, writeErr := temporary.Write(content)
	syncErr := temporary.Sync()
	closeErr := temporary.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	destination := filepath.Join(privateDirectory, "release-go-archive-report.json")
	if err := os.Rename(temporaryName, destination); err != nil {
		return err
	}
	return syncDirectory(privateDirectory)
}

func ensurePrivateDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(directory)
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("private witness parent is not a real directory")
	}
	if err := invokingUserOwns(info); err != nil {
		return err
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(directory, 0o700); err != nil {
			return err
		}
	}
	return nil
}
