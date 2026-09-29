package flowdocs

import (
	"github.com/Beamfall/corvint/internal/doccorpus"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// openDirectory rejects symlink ancestors before acquiring a confined root.
// All later file operations are relative to that root, including race handling.
func openDirectory(dir string) (*os.Root, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fail("invalid directory")
	}
	current, err := os.OpenRoot(string(filepath.Separator))
	if err != nil {
		return nil, fail("cannot open filesystem root")
	}
	for _, part := range strings.Split(strings.TrimPrefix(abs, string(filepath.Separator)), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		next, e := openComponent(current, part)
		if e != nil {
			current.Close()
			return nil, e
		}
		current.Close()
		current = next
	}
	return current, nil
}

func openComponent(parent *os.Root, name string) (*os.Root, error) {
	before, err := parent.Lstat(name)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fail("directory must be a real nonsymlink component")
	}
	next, err := parent.OpenRoot(name)
	if err != nil {
		return nil, fail("cannot open directory component")
	}
	opened, err := next.Stat(".")
	after, afterErr := parent.Lstat(name)
	if err != nil || afterErr != nil || !after.IsDir() || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, opened) || !os.SameFile(before, after) {
		next.Close()
		return nil, fail("directory changed during confined open")
	}
	return next, nil
}

func ReadFile(name string) ([]byte, error) {
	r, err := openDirectory(filepath.Dir(name))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return readRoot(r, filepath.Base(name), MaxManifestBytes)
}
func ReadCorpusFile(name string) ([]byte, error) {
	r, err := openDirectory(filepath.Dir(name))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return readRoot(r, filepath.Base(name), doccorpus.MaxCorpusBytes)
}

func readRoot(r *os.Root, name string, limit int) ([]byte, error) {
	s, err := r.Lstat(name)
	if err != nil || !s.Mode().IsRegular() || s.Size() > int64(limit) {
		return nil, fail("input must be a bounded regular file")
	}
	f, err := openRootFile(r, name)
	if err != nil {
		return nil, fail("cannot open input")
	}
	defer f.Close()
	actual, err := f.Stat()
	if err != nil || !os.SameFile(s, actual) {
		return nil, fail("input changed during read")
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(b) > limit {
		return nil, fail("input read bound exceeded")
	}
	return b, nil
}
func Materialize(destination string, r *Result) error {
	return materialize(destination, r.Files, MaxFlows*8+2)
}

// MaterializeCoverage shares the confined writer with the bounded coverage descendant.
func MaterializeCoverage(destination string, files map[string][]byte) error {
	return materialize(destination, files, MaxFlows*8+4)
}
func materialize(destination string, files map[string][]byte, count int) error {
	if len(files) > count {
		return fail("output file count exceeded")
	}
	parent, err := openDirectory(filepath.Dir(destination))
	if err != nil {
		return err
	}
	defer parent.Close()
	base := filepath.Base(destination)
	if base == "." || base == ".." {
		return fail("fresh output directory required")
	}
	if err = parent.Mkdir(base, 0700); err != nil {
		return fail("output directory must be fresh; refusing overwrite")
	}
	dir, err := openComponent(parent, base)
	if err != nil {
		return fail("cannot open fresh output directory")
	}
	defer dir.Close()
	names := []string{}
	total := 0
	for name, b := range files {
		if !fs.ValidPath(name) || strings.Contains(name, "\\") {
			return fail("invalid output name")
		}
		total += len(b)
		names = append(names, name)
	}
	if total > MaxOutputBytes {
		return fail("aggregate output byte bound exceeded")
	}
	sort.Strings(names)
	subdirs := map[string]*os.Root{}
	defer func() {
		for _, sub := range subdirs {
			sub.Close()
		}
	}()
	for _, name := range names {
		target, filename := dir, name
		if slash := strings.IndexByte(name, '/'); slash >= 0 {
			subname := name[:slash]
			filename = name[slash+1:]
			target = subdirs[subname]
			if target == nil {
				if err = dir.Mkdir(subname, 0700); err != nil {
					return fail("cannot exclusively create flow directory")
				}
				target, err = openComponent(dir, subname)
				if err != nil {
					return err
				}
				subdirs[subname] = target
			}
		}
		file, err := target.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fail("cannot exclusively create output")
		}
		_, err = file.Write(files[name])
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			return fail("output write failed")
		}
	}
	return nil
}
func CompareOutput(destination string, files map[string][]byte) ([]string, error) {
	root, err := openDirectory(destination)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	diff := []string{}
	seen := map[string]bool{}
	count := 0
	err = fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, e error) error {
		if e != nil {
			return fail("output inventory failed")
		}
		if name == "." {
			return nil
		}
		count++
		if count > MaxFlows*9+4 {
			return fail("output inventory exceeds bound")
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fail("output contains symlink")
		}
		if d.IsDir() {
			return nil
		}
		expected, ok := files[name]
		if !ok {
			diff = append(diff, name)
			return nil
		}
		seen[name] = true
		limit := MaxBytes
		if name == "generation.json" || name == "source-generation.json" {
			limit = MaxManifestBytes
		}
		b, e := readRoot(root, name, limit)
		if e != nil {
			return e
		}
		if string(b) != string(expected) {
			diff = append(diff, name)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for name := range files {
		if !seen[name] {
			diff = append(diff, name)
		}
	}
	sort.Strings(diff)
	return diff, nil
}
