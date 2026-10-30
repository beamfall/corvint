package main

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type fileEntry struct {
	Path      string
	Mode      int64
	Data      []byte
	Directory bool
}

// The fixture encoding is sorted USTAR, zero metadata, regular files/directories
// only. It is derived owned-test data, not a general repository archive extractor.
func canonicalSnapshot(entries []fileEntry) ([]byte, error) {
	entries = append([]fileEntry(nil), entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	for _, e := range entries {
		h := &tar.Header{Name: e.Path, Mode: e.Mode, Size: int64(len(e.Data)), ModTime: time.Unix(0, 0), Format: tar.FormatUSTAR, Typeflag: tar.TypeReg}
		if e.Directory {
			h.Typeflag = tar.TypeDir
			h.Size = 0
		}
		if err := w.WriteHeader(h); err != nil {
			return nil, err
		}
		if !e.Directory {
			if _, err := w.Write(e.Data); err != nil {
				return nil, err
			}
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func readSnapshot(data []byte) ([]fileEntry, error) {
	r := tar.NewReader(bytes.NewReader(data))
	entries := []fileEntry{}
	seen := map[string]bool{}
	total := int64(0)
	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if !cleanRelative(h.Name, false) || seen[h.Name] || h.Mode < 0 || h.Mode > 0777 {
			return nil, errors.New("invalid snapshot entry")
		}
		seen[h.Name] = true
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir {
			return nil, errors.New("owned snapshots support only regular files and directories")
		}
		total += h.Size
		if len(entries) >= fixtureEntryLimit || total > fixtureByteLimit {
			return nil, errResourceBound
		}
		b, err := io.ReadAll(io.LimitReader(r, fixtureByteLimit+1))
		if err != nil {
			return nil, err
		}
		entries = append(entries, fileEntry{h.Name, h.Mode, b, h.Typeflag == tar.TypeDir})
	}
	canonical, err := canonicalSnapshot(entries)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(canonical, data) {
		return nil, errors.New("snapshot is not canonical tar")
	}
	return entries, nil
}
func materialize(data []byte, root string) error {
	entries, err := readSnapshot(data)
	if err != nil {
		return err
	}
	for _, e := range entries {
		p := filepath.Join(root, e.Path)
		if e.Directory {
			if err = os.MkdirAll(p, 0700); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return err
		}
		if err = os.WriteFile(p, e.Data, 0600); err != nil {
			return err
		}
		if err = os.Chmod(p, os.FileMode(e.Mode)); err != nil {
			return err
		}
	}
	// Directory permissions are applied after children, allowing read-only fixtures.
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.Directory {
			if err = os.Chmod(filepath.Join(root, e.Path), os.FileMode(e.Mode)); err != nil {
				return err
			}
		}
	}
	return nil
}

type fileState struct {
	Digest string
	Mode   fs.FileMode
	Size   int64
	Link   string
}

func treeState(root string) (map[string]fileState, error) {
	result := map[string]fileState{}
	total := int64(0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if len(result) >= fixtureEntryLimit {
			return errResourceBound
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		s := fileState{Mode: info.Mode(), Size: info.Size()}
		switch {
		case info.Mode().IsRegular():
			total += info.Size()
			if total > fixtureByteLimit {
				return errResourceBound
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			b, err := io.ReadAll(io.LimitReader(f, fixtureByteLimit+1))
			f.Close()
			if err != nil {
				return err
			}
			if len(b) > fixtureByteLimit {
				return errResourceBound
			}
			s.Digest = digest(b)
		case info.Mode()&os.ModeSymlink != 0:
			s.Link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		case info.IsDir():
		default:
			return errors.New("unobservable special file in scratch")
		}
		result[filepath.ToSlash(rel)] = s
		return nil
	})
	return result, err
}

type observedEffect struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Basis  string `json:"basis"`
}

func diffEffects(before, after map[string]fileState, declared []effect) []observedEffect {
	actual := map[effect]bool{}
	for p, b := range before {
		a, ok := after[p]
		if !ok {
			actual[effect{p, "delete"}] = true
			continue
		}
		if b.Mode != a.Mode {
			actual[effect{p, "mode"}] = true
		}
		if b.Digest != a.Digest || b.Link != a.Link {
			actual[effect{p, "write"}] = true
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			actual[effect{p, "create"}] = true
		}
	}
	out := []observedEffect{}
	for e := range actual {
		basis := "undeclared-observed"
		if containsEffect(declared, e) {
			basis = "declared-observed"
		}
		out = append(out, observedEffect{e.Path, e.Kind, "PRODUCED", basis})
	}
	for _, e := range declared {
		if !actual[e] {
			out = append(out, observedEffect{e.Path, e.Kind, "NOT_PRODUCED", "declared-absent"})
		}
	}
	return unionEffects(out)
}
func unionEffects(effects []observedEffect) []observedEffect {
	m := map[effect]observedEffect{}
	for _, e := range effects {
		k := effect{e.Path, e.Kind}
		old, ok := m[k]
		if !ok || old.Status != "PRODUCED" {
			m[k] = e
		}
	}
	out := make([]observedEffect, 0, len(m))
	for _, e := range m {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func removeScratch(root string) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.Chmod(path, 0700)
		}
		return nil
	})
	_ = os.RemoveAll(root)
}
