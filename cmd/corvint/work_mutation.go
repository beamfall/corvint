package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// complete describes the accepted store scope, not merely the directories we
// could read. WQO's closed policy declares no external store roots, so production
// detect-only observations cannot establish complete mutation coverage.
type workManifest struct {
	status, index, corvint, digest string
	complete, monitoredComplete    bool
	changed, qualificationFailed   bool
	rows                           map[string]map[string]workMutationFact
	rootComplete                   map[string]bool
}

type workMutationFact struct {
	mode     fs.FileMode
	digest   string
	readable bool
	exists   bool
}

const workManifestBytes = 512 << 20
const workManifestEntries = 200000

// workMutationManifest brackets the caller tree, resolved Git/common metadata,
// and isolated target. Private HOME/TMPDIR are disposable execution storage.
func workMutationManifest(ctx context.Context, roots []string) workManifest {
	unique := map[string]bool{}
	for _, root := range roots {
		if root != "" {
			unique[filepath.Clean(root)] = true
		}
	}
	paths := make([]string, 0, len(unique))
	for root := range unique {
		paths = append(paths, root)
	}
	sort.Strings(paths)
	hash := sha256.New()
	result := workManifest{monitoredComplete: true, rows: map[string]map[string]workMutationFact{}, rootComplete: map[string]bool{}}
	budget, entries := int64(workManifestBytes), 0
	for _, root := range paths {
		rows := map[string]workMutationFact{}
		result.rows[root] = rows
		digest, err := workManifestRootRows(ctx, root, &budget, &entries, rows)
		result.rootComplete[root] = err == nil
		if err != nil {
			result.monitoredComplete = false
			continue
		}
		fmt.Fprintf(hash, "%d:%s:%s\n", len(root), root, digest)
	}
	if result.monitoredComplete {
		result.digest = hex.EncodeToString(hash.Sum(nil))
	}
	return result
}

func workManifestRoot(ctx context.Context, path string, budget *int64, entries *int) (string, error) {
	return workManifestRootRows(ctx, path, budget, entries, nil)
}

func workManifestRootRows(ctx context.Context, path string, budget *int64, entries *int, rows map[string]workMutationFact) (string, error) {
	before, err := os.Lstat(path)
	if err != nil {
		if rows != nil && errors.Is(err, os.ErrNotExist) {
			rows["\x00root"] = workMutationFact{}
		}
		return "", err
	}
	if rows != nil {
		rows["\x00root"] = workMutationFact{exists: true, mode: before.Mode()}
	}
	if !before.IsDir() {
		return "", errors.New("manifest root is not directory")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return "", err
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(before, opened) {
		return "", errors.New("manifest root changed")
	}
	hash := sha256.New()
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		*entries++
		if *entries > workManifestEntries {
			return errors.New("manifest entry limit")
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if rows != nil {
			rows[name] = workMutationFact{exists: true, mode: info.Mode()}
		}
		digest, err := workManifestEntry(root, name, info, budget)
		if err != nil {
			return err
		}
		row := fmt.Sprintf("%o:%s", info.Mode(), digest)
		if rows != nil {
			rows[name] = workMutationFact{exists: true, mode: info.Mode(), readable: true, digest: digest}
		}
		fmt.Fprintf(hash, "%d:%s:%s\n", len(name), name, row)
		return nil
	})
	if err != nil {
		return "", err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		return "", errors.New("manifest root changed")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func workManifestEntry(root *os.Root, name string, before fs.FileInfo, budget *int64) (string, error) {
	if before.IsDir() {
		return "directory", nil
	}
	if before.Mode()&os.ModeSymlink != 0 {
		link, err := root.Readlink(name)
		if err != nil {
			return "", err
		}
		after, err := root.Lstat(name)
		if err != nil || !workManifestSame(before, after) {
			return "", errors.New("manifest link changed")
		}
		digest := sha256.Sum256([]byte(link))
		return hex.EncodeToString(digest[:]), nil
	}
	if !before.Mode().IsRegular() {
		return "", errors.New("unsupported manifest entry")
	}
	// Reject symlink components before opening. os.Root additionally prevents
	// escape if a component changes between this check and the open.
	if err := workManifestParents(root, name); err != nil {
		return "", err
	}
	file, err := workOpenManifest(root, name)
	if err != nil {
		return "", err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !workManifestSame(before, opened) {
		return "", errors.New("manifest file changed")
	}
	hash := sha256.New()
	count, err := io.Copy(hash, io.LimitReader(file, *budget+1))
	*budget -= count
	if err != nil || *budget < 0 {
		return "", errors.New("manifest byte limit or read failure")
	}
	after, err := file.Stat()
	if err != nil || !workManifestSame(before, after) || count != before.Size() {
		return "", errors.New("manifest read changed")
	}
	current, err := root.Lstat(name)
	if err != nil || !workManifestSame(before, current) {
		return "", errors.New("manifest path changed")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func workManifestParents(root *os.Root, name string) error {
	parts := strings.Split(filepath.ToSlash(name), "/")
	for i := 1; i < len(parts); i++ {
		info, err := root.Lstat(strings.Join(parts[:i], "/"))
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return errors.New("manifest parent changed")
		}
	}
	return nil
}

func workManifestSame(a, b fs.FileInfo) bool {
	return os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime() == b.ModTime()
}

func workCloseManifest(opening workManifest, closing workManifest) workManifest {
	closing.changed = opening.changed || closing.changed
	if opening.monitoredComplete && closing.monitoredComplete && opening.digest != closing.digest {
		closing.changed = true
	}
	for root, before := range opening.rows {
		after, observed := closing.rows[root]
		if !observed {
			continue
		}
		for name, old := range before {
			value, exists := after[name]
			if exists && (old.exists != value.exists || old.mode != value.mode || (old.readable && value.readable && old.digest != value.digest)) {
				closing.changed = true
			}
			if !exists && closing.rootComplete[root] {
				closing.changed = true
			}
		}
		for name := range after {
			if _, exists := before[name]; !exists && opening.rootComplete[root] {
				closing.changed = true
			}
		}
	}
	return closing
}
