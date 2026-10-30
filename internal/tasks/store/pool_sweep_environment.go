package store

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

type sweepEnvironment struct {
	values map[string]string
	file   wire.Digest
}

type sweepParentIdentity struct {
	path string
	info os.FileInfo
}

func sweepParents(path string) ([]sweepParentIdentity, error) {
	parents := []sweepParentIdentity{}
	for p := filepath.Dir(path); ; p = filepath.Dir(p) {
		st, e := os.Lstat(p)
		if e != nil {
			return nil, e
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, "env file", "non-directory or symlink parent")
		}
		parents = append(parents, sweepParentIdentity{p, st})
		if p == filepath.Dir(p) {
			break
		}
	}
	return parents, nil
}
func loadSweepEnvironment(root string, keys []string, file string) (sweepEnvironment, error) {
	return loadSweepEnvironmentAt(root, keys, file, nil)
}

// The optional hook is used only by same-package race fixtures.
func loadSweepEnvironmentAt(root string, keys []string, file string, reached func(string, string)) (sweepEnvironment, error) {
	env := sweepEnvironment{values: map[string]string{}, file: wire.Sum(nil)}
	allowed := map[string]bool{}
	for _, key := range keys {
		allowed[key] = true
		if value, ok := os.LookupEnv(key); ok {
			env.values[key] = value
		}
	}
	if file == "" {
		return env, nil
	}
	for _, component := range strings.Split(filepath.ToSlash(file), "/") {
		if component == ".." || component == "." {
			return env, wire.Errorf(wire.CodeMalformed, "env file", "path traversal")
		}
	}
	path := file
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	parents, e := sweepParents(path)
	if e != nil {
		return env, e
	}
	before, e := os.Lstat(path)
	if e != nil {
		return env, e
	}
	if !before.Mode().IsRegular() || before.Mode().Perm()&0077 != 0 {
		return env, wire.Errorf(wire.CodeUnsupportedFilesystem, "env file", "private regular file required")
	}
	if reached != nil {
		reached("beforeRead", path)
	}
	raw, e := intent.ReadFile(path, 65536)
	if reached != nil {
		reached("afterRead", path)
	}
	if e != nil {
		return env, e
	}
	after, e := os.Lstat(path)
	if e != nil {
		return env, e
	}
	if !os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() || before.Mode() != after.Mode() {
		return env, wire.Errorf(wire.CodeSnapshotMoved, "env file", "changed during read")
	}
	afterParents, e := sweepParents(path)
	if e != nil {
		return env, e
	}
	if len(parents) != len(afterParents) {
		return env, wire.Errorf(wire.CodeSnapshotMoved, "env file", "parent chain changed")
	}
	for i, before := range parents {
		after := afterParents[i]
		if before.path != after.path || !os.SameFile(before.info, after.info) {
			return env, wire.Errorf(wire.CodeSnapshotMoved, "env file", "parent identity changed during read")
		}
	}
	if !utf8.Valid(raw) || strings.IndexByte(string(raw), 0) >= 0 {
		return env, wire.Errorf(wire.CodeMalformed, "env file", "UTF8 without NUL required")
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(raw) == 0 {
		lines = nil
	}
	if len(lines) > 64 {
		return env, wire.Errorf(wire.CodeLimitExceeded, "env file", "at most64 lines")
	}
	seen := map[string]bool{}
	for _, line := range lines {
		key, value, ok := strings.Cut(line, "=")
		if !ok || !allowed[key] || seen[key] {
			return env, wire.Errorf(wire.CodeMalformed, "env file", "duplicate, undeclared or malformed key")
		}
		seen[key] = true
		env.values[key] = value
	}
	env.file = wire.Sum(raw)
	return env, nil
}
func (s sweepEnvironment) phase(keys []string) ([]string, wire.Digest) {
	out := []string{}
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out = append(out, key+"="+value)
		}
	}
	sort.Strings(out)
	return out, wire.Sum([]byte(strings.Join(out, "\x00")))
}
