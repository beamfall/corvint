package jstestprovider

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// digestFile hashes one file's content. An empty path digests as "" so a
// caller can distinguish "not configured" from "hashed"; a named file that
// cannot be read is an error.
func digestFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// digestFiles hashes each path independently, keyed by the path given.
func digestFiles(paths []string) (map[string]string, error) {
	out := make(map[string]string, len(paths))
	for _, p := range paths {
		d, err := digestFile(p)
		if err != nil {
			return nil, err
		}
		out[p] = d
	}
	return out, nil
}

// digestCombined hashes the concatenation of several files' own digests, in
// a stable order, so the result changes if any one of them changes. Used to
// bind package.json + lockfile as a single package identity.
func digestCombined(paths ...string) (string, error) {
	h := sha256.New()
	for _, p := range paths {
		d, err := digestFile(p)
		if err != nil {
			return "", err
		}
		io.WriteString(h, p)
		io.WriteString(h, "=")
		io.WriteString(h, d)
		io.WriteString(h, "\n")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DigestAppBuildDir hashes every regular file under dir (relative path plus
// content), sorted for determinism, into one app-build identity digest. A
// missing or empty directory returns an explicit AppBuildIdentity rather
// than a digest of nothing, so "not built yet" is never mistaken for "built
// and empty." An entry that does not resolve to a regular file (a symlink to
// a directory, a dangling symlink) makes the identity unknown as well.
func DigestAppBuildDir(dir string) (AppBuildIdentity, error) {
	if dir == "" {
		return AppBuildIdentity{Unknown: true, Reason: "no app build directory configured"}, nil
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return AppBuildIdentity{Unknown: true, Reason: "app build directory not found: " + dir}, nil
	}
	var rels []string
	var unbindable string
	files := map[string]string{}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			unbindable = rel
			return filepath.SkipAll
		}
		digest, err := digestFile(path)
		if err != nil {
			return err
		}
		rels = append(rels, rel)
		files[rel] = digest
		return nil
	})
	if err != nil {
		return AppBuildIdentity{}, err
	}
	if unbindable != "" {
		return AppBuildIdentity{Unknown: true, Reason: "app build entry is not a regular file: " + unbindable}, nil
	}
	if len(rels) == 0 {
		return AppBuildIdentity{Unknown: true, Reason: "app build directory is empty: " + dir}, nil
	}
	sort.Strings(rels)
	h := sha256.New()
	for _, rel := range rels {
		io.WriteString(h, rel)
		io.WriteString(h, "=")
		io.WriteString(h, files[rel])
		io.WriteString(h, "\n")
	}
	return AppBuildIdentity{Digest: hex.EncodeToString(h.Sum(nil))}, nil
}
