package affected

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ExistenceReadersPath is the repository's declaration of the Go packages
// whose tests test the existence of paths named inside manifest files
// (AFP-V0-980).
const ExistenceReadersPath = ".corvint/existence-readers.json"

// ExistenceReadersProfile is the declaration's closed profile name.
const ExistenceReadersProfile = "corvint-existence-readers/0"

// WitnessDeclaredExistenceReader names a declared existence reader one of
// whose manifests names a dirty path absent from the checkout (AFP-V0-980).
const WitnessDeclaredExistenceReader = "DECLARED_EXISTENCE_READER"

const (
	maxExistenceReadersBytes   = 64 << 10
	maxExistenceReadersEntries = 64
	maxExistenceManifests      = 8
	maxExistenceManifestBytes  = 4 << 20
	maxExistenceNames          = 1 << 16
	maxExistenceReaderReason   = 512
)

// ExistenceReader is one declared entry with the names its manifests hold.
type ExistenceReader struct {
	Package   string   `json:"package"`
	Manifests []string `json:"manifests"`
	Reason    string   `json:"reason"`
	// Names are the repository paths the manifests name, sorted; they are
	// derived when the graph is built, never declared.
	Names []string `json:"names,omitempty"`
}

type existenceReadersFile struct {
	Profile string            `json:"profile"`
	Entries []ExistenceReader `json:"entries"`
}

// readExistenceReaders returns the declared entries with their manifest names:
// nil with no declaration, an error for one that is unreadable, oversized or
// invalid in any member, or whose manifest is unreadable, oversized or names
// more than the bound (AFP-V0-980).
func readExistenceReaders(root *Source) ([]ExistenceReader, error) {
	if root == nil {
		return nil, nil
	}
	body, err := root.ReadBounded(ExistenceReadersPath, maxExistenceReadersBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ExistenceReadersPath, err)
	}
	var declared existenceReadersFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("%s: %w", ExistenceReadersPath, err)
	}
	if declared.Profile != ExistenceReadersProfile || declared.Entries == nil || len(declared.Entries) > maxExistenceReadersEntries {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d entries", ExistenceReadersPath, ExistenceReadersProfile, maxExistenceReadersEntries)
	}
	previous := ""
	for index := range declared.Entries {
		entry := &declared.Entries[index]
		if !cleanRelative(entry.Package) || entry.Package <= previous || entry.Names != nil {
			return nil, fmt.Errorf("%s: package %q", ExistenceReadersPath, entry.Package)
		}
		previous = entry.Package
		if entry.Reason == "" || len(entry.Reason) > maxExistenceReaderReason || !utf8.ValidString(entry.Reason) || strings.ContainsFunc(entry.Reason, unicode.IsControl) {
			return nil, fmt.Errorf("%s: reason of %q", ExistenceReadersPath, entry.Package)
		}
		if len(entry.Manifests) == 0 || len(entry.Manifests) > maxExistenceManifests {
			return nil, fmt.Errorf("%s: %q must name 1 to %d manifests", ExistenceReadersPath, entry.Package, maxExistenceManifests)
		}
		names := map[string]bool{}
		for position, manifest := range entry.Manifests {
			if !cleanRelative(manifest) || slices.Contains(entry.Manifests[:position], manifest) {
				return nil, fmt.Errorf("%s: manifest %q of %q", ExistenceReadersPath, manifest, entry.Package)
			}
			content, err := root.ReadBounded(manifest, maxExistenceManifestBytes)
			if err != nil {
				return nil, fmt.Errorf("%s: manifest %q of %q: %w", ExistenceReadersPath, manifest, entry.Package, err)
			}
			manifestNames(content, path.Dir(manifest), names)
			if len(names) > maxExistenceNames {
				return nil, fmt.Errorf("%s: the manifests of %q name more than %d paths", ExistenceReadersPath, entry.Package, maxExistenceNames)
			}
		}
		entry.Names = sortedKeys(names)
	}
	return declared.Entries, nil
}

// cleanRelative reports a clean, slash-separated repository-relative path that
// does not climb out of the repository.
func cleanRelative(value string) bool {
	return value != "." && fs.ValidPath(value)
}

// manifestNames adds to names every maximal run of path bytes in content that
// is a valid repository path as written or joined to directory, the
// manifest's own directory, against which a Markdown link resolves.
func manifestNames(content []byte, directory string, names map[string]bool) {
	pathByte := func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._/@+-", r))
	}
	for _, field := range bytes.FieldsFunc(content, pathByte) {
		value := string(field)
		if strings.HasPrefix(value, "/") {
			continue
		}
		for _, candidate := range []string{path.Clean(value), path.Join(directory, value)} {
			if cleanRelative(candidate) {
				names[candidate] = true
			}
		}
	}
}

// existenceReaderUnits maps each declared entry to the Go unit of its package
// directory, or returns an error naming an entry the Go plugin does not
// observe. A graph without the Go plugin declares none of them.
func (graph *Graph) existenceReaderUnits() (map[string]string, error) {
	if len(graph.existenceReaders) == 0 || !slices.Contains(graph.languages, "go") {
		return nil, nil
	}
	byDirectory := map[string]string{}
	for _, id := range graph.order {
		unit := graph.units[id]
		files := append(append([]string(nil), unit.Sources...), unit.Tests...)
		if strings.HasPrefix(id, "go:") && len(files) != 0 {
			byDirectory[relativeDirectory(files[0])] = id
		}
	}
	units := make(map[string]string, len(graph.existenceReaders))
	for _, entry := range graph.existenceReaders {
		id, ok := byDirectory[entry.Package]
		if !ok {
			return nil, fmt.Errorf("%s: package %q is no Go package the graph observes", ExistenceReadersPath, entry.Package)
		}
		units[entry.Package] = id
	}
	return units, nil
}

// existenceReadersOf selects each declared entry's unit not already reached
// when a manifest names a dirty path absent from the checkout, or the graph
// has no disk root, or the declaration itself is dirty (AFP-V0-980). Like a
// declared read scope it is a terminal witness and adds no unknown entry.
func (graph *Graph) existenceReadersOf(reached map[string]Witness, dirty []string) {
	declarationDirty := slices.Contains(dirty, ExistenceReadersPath)
	for _, entry := range graph.existenceReaders {
		id, ok := graph.existenceUnits[entry.Package]
		if !ok {
			continue
		}
		if _, seen := reached[id]; seen {
			continue
		}
		for _, dirtyPath := range dirty {
			_, named := slices.BinarySearch(entry.Names, dirtyPath)
			if declarationDirty || named && graph.absent(dirtyPath) {
				reached[id] = Witness{Kind: WitnessDeclaredExistenceReader, DirtyPath: dirtyPath, Via: []string{id}}
				break
			}
		}
	}
}

// absent reports a path missing from the graph's disk checkout; a graph with
// no disk root cannot tell, so every path counts as absent.
func (graph *Graph) absent(relative string) bool {
	if graph.root == "" {
		return true
	}
	_, err := os.Lstat(filepath.Join(graph.root, filepath.FromSlash(relative)))
	return errors.Is(err, fs.ErrNotExist)
}

// ExistenceReaderPackages lists the package directories the repository
// declares as existence readers (AFP-V0-980), sorted.
func (graph *Graph) ExistenceReaderPackages() []string {
	packages := make([]string, 0, len(graph.existenceReaders))
	for _, entry := range graph.existenceReaders {
		packages = append(packages, entry.Package)
	}
	return packages
}
