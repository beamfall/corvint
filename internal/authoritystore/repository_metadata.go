package authoritystore

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
)

type repositoryMetadata struct {
	path    string
	present bool
	info    os.FileInfo
	raw     []byte
}

func readRepositoryMetadata(path string) (repositoryMetadata, error) {
	out := repositoryMetadata{path: path}
	before, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > 64<<10 {
		return out, errUnavailable
	}
	file, err := openBindingMetadata(path)
	if err != nil {
		return out, errUnavailable
	}
	defer file.Close()
	held, err := file.Stat()
	if err != nil || !sameRepositoryMetadata(before, held) {
		return out, errUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err != nil || int64(len(raw)) != before.Size() {
		return out, errUnavailable
	}
	after, err := file.Stat()
	named, e := os.Lstat(path)
	if err != nil || e != nil || !sameRepositoryMetadata(before, after) || !sameRepositoryMetadata(after, named) {
		return out, errUnavailable
	}
	out.present = true
	out.info = after
	out.raw = raw
	return out, nil
}
func sameRepositoryMetadata(a, b os.FileInfo) bool {
	if a == nil || b == nil || !os.SameFile(a, b) || a.Mode() != b.Mode() || a.Size() != b.Size() || a.ModTime() != b.ModTime() {
		return false
	}
	// Native Unix change time is required; a platform without it cannot provide
	// the protected metadata witness. Access times intentionally do not compare.
	av, bv := reflect.ValueOf(a.Sys()), reflect.ValueOf(b.Sys())
	if av.Kind() != reflect.Pointer || bv.Kind() != reflect.Pointer || av.IsNil() || bv.IsNil() {
		return false
	}
	av, bv = av.Elem(), bv.Elem()
	for _, field := range []string{"Ctimespec", "Ctim"} {
		ac, bc := av.FieldByName(field), bv.FieldByName(field)
		if ac.IsValid() && bc.IsValid() && ac.CanInterface() && bc.CanInterface() {
			return reflect.DeepEqual(ac.Interface(), bc.Interface())
		}
	}
	return false
}
func (b *repositoryBinding) captureConfiguration() error {
	seen := map[string]bool{}
	for _, dir := range []string{b.repo.GitDir, b.repo.CommonDir} {
		for _, name := range []string{"config", "config.worktree"} {
			path := filepath.Join(dir, name)
			if seen[path] {
				continue
			}
			seen[path] = true
			entry, err := readRepositoryMetadata(path)
			if err != nil {
				return err
			}
			b.configuration = append(b.configuration, entry)
		}
	}
	return nil
}
func (b *repositoryBinding) configurationUnchanged() error {
	for _, before := range b.configuration {
		after, err := readRepositoryMetadata(before.path)
		if err != nil || before.present != after.present || !bytes.Equal(before.raw, after.raw) {
			return errUnavailable
		}
		if before.present && !sameRepositoryMetadata(before.info, after.info) {
			return errUnavailable
		}
	}
	return nil
}
