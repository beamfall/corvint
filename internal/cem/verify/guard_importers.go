package verify

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxGuardImporterDirectories bounds the direct importers whose tests a hunk
// with no own-directory candidate searches, in directory order (TCQ-V0-761).
const MaxGuardImporterDirectories = 16

// importIndex maps each import path of the copy's root module to the sorted
// directories whose Go files, tests included, import it.
type importIndex struct {
	module    string
	importers map[string][]string
}

// indexImporters reads the import clauses of every Go file of the copy's root
// module once: hidden, testdata and vendor directories and nested modules are
// skipped, and an unreadable file or a copy with no module line imports
// nothing.
func indexImporters(tree string) *importIndex {
	index := &importIndex{importers: map[string][]string{}}
	data, err := os.ReadFile(filepath.Join(tree, "go.mod"))
	if err != nil {
		return index
	}
	for _, line := range strings.Split(string(data), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "module" {
			index.module = strings.Trim(fields[1], `"`)
		}
	}
	if index.module == "" {
		return index
	}
	seen := map[string]map[string]bool{}
	_ = filepath.WalkDir(tree, func(full string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relative, _ := filepath.Rel(tree, full)
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			name := entry.Name()
			if relative != "." && (strings.HasPrefix(name, ".") || name == "testdata" || name == "vendor") {
				return filepath.SkipDir
			}
			if _, err := os.Lstat(filepath.Join(full, "go.mod")); relative != "." && err == nil {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || !strings.HasSuffix(relative, ".go") {
			return nil
		}
		source, _, err := readCopyFile(tree, full)
		if err != nil {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), "", source, parser.ImportsOnly)
		if err != nil {
			return nil
		}
		dir := path.Dir(relative)
		for _, spec := range parsed.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			if seen[imported] == nil {
				seen[imported] = map[string]bool{}
			}
			seen[imported][dir] = true
		}
		return nil
	})
	for imported, dirs := range seen {
		for dir := range dirs {
			index.importers[imported] = append(index.importers[imported], dir)
		}
		sort.Strings(index.importers[imported])
	}
	return index
}

// of returns at most MaxGuardImporterDirectories directories, other than dir
// itself, whose files import the package in dir.
func (index *importIndex) of(dir string) []string {
	if index.module == "" {
		return nil
	}
	imported := index.module
	if dir != "." {
		imported += "/" + dir
	}
	var dirs []string
	for _, importer := range index.importers[imported] {
		if importer != dir && len(dirs) < MaxGuardImporterDirectories {
			dirs = append(dirs, importer)
		}
	}
	return dirs
}

// exportedNames keeps the exported names of reached: only they are visible to
// an importing package.
func exportedNames(reached map[string]bool) map[string]bool {
	exported := map[string]bool{}
	for name := range reached {
		if first, _ := utf8.DecodeRuneInString(name); unicode.IsUpper(first) {
			exported[name] = true
		}
	}
	return exported
}
