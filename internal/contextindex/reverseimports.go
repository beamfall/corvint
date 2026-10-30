package contextindex

import (
	"encoding/json"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/projectprofile"
)

// This file holds the reverse-import resolution rules `GPK-V0-027` names, and
// nothing else. The clause admits an impact path only where this specification
// names a rule for its suffix, and names exactly three:
//
//	(a) `.go`  -- the containing Go package's import path.
//	(b) `.py`  -- the dotted-module spellings the path can be imported by.
//	(c) `.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, `.cjs` -- relative specifiers,
//	    and the alias prefix the *project profile* configures.
//
// Each is a port of one arm of the frozen oracle's `_reverse_importers`
// (`src/context_corvint.py:714-749`), which is the sole authority for what these
// rules mean: `GPK-V0-033` forbids a rule whose only authority is candidate
// behaviour. Where an arm's behaviour looks like a defect it is reproduced
// anyway and the reproduction is commented, because the corpus expectations are
// the oracle's bytes.
//
// A suffix with no rule resolves nothing. `.rs`, `.cs`, `.swift`, `.kt`,
// `.kts`, `.rb`, `.sql` and `.m` are indexed and searchable but have no oracle
// arm, so their impact packets disclose that missing reverse-import dimension.

// The suffixes of rules (a) and (b). The web set of rule (c) is `webSuffixes`,
// which webimports.go already owns.
const (
	goImpactSuffix     = ".go"
	pythonImpactSuffix = ".py"
)

// ImpactRuleNamed reports whether `GPK-V0-027` names a reverse-import
// resolution rule for this path's suffix. It is the single predicate behind
// the receipt's coverage disclosure. Index admission is a separate predicate:
// an admitted suffix can lack a reverse-import rule without being refused.
func ImpactRuleNamed(changedPath string) bool {
	if strings.HasSuffix(changedPath, goImpactSuffix) || strings.HasSuffix(changedPath, pythonImpactSuffix) {
		return true
	}
	return webSuffixes[strings.ToLower(pythonPathSuffix(changedPath))]
}

// reverseImporters resolves the sources that import changedPath through the
// rule named for its suffix. The dispatch order and each arm's suffix test are
// the oracle's: the Go and Python arms test the raw suffix exactly, and only
// the web arm case-folds, because only the web arm asks `PurePosixPath.suffix`
// for a lowercased answer.
func reverseImporters(index *Index, changedPath string) []importer {
	switch {
	case strings.HasSuffix(changedPath, goImpactSuffix):
		return goReverseImporters(index, changedPath)
	case strings.HasSuffix(changedPath, pythonImpactSuffix):
		return pythonReverseImporters(index, changedPath)
	case webSuffixes[strings.ToLower(pythonPathSuffix(changedPath))]:
		return webReverseImporters(index, changedPath)
	}
	return nil
}

// goReverseImporters is rule (a): the changed file's directory is its package,
// and an importer names that package by the module-qualified import path.
func goReverseImporters(index *Index, changedPath string) []importer {
	target := goPackageImportPath(index.Module, changedPath)
	found := make([]importer, 0)
	for importerPath, imports := range index.Imports {
		if importerPath != changedPath && contains(imports, target) {
			found = append(found, importer{importerPath, target})
		}
	}
	return sortImporters(found)
}

// goPackageImportPath is the import path of the package holding changedPath:
// the module path for a root-package file (go.dev/ref/mod: the module path is
// the import path of the module's root directory), else the module path plus
// the file's directory. Without a module the directory alone is the path, as
// the oracle spells it. Decision 0023 admits the root package; the oracle
// resolves it to `module/.`, which nothing imports (DR-0017).
func goPackageImportPath(module, changedPath string) string {
	directory := path.Dir(changedPath)
	if module == "" {
		return directory
	}
	if directory == "." {
		return module
	}
	return module + "/" + directory
}

// pythonReverseImporters is rule (b). A Python module is imported by a dotted
// name, never by a path, and the same file is reachable under several such
// names depending on whether the importer runs with the package root or the
// source root on `sys.path`. `PythonImportCandidates` enumerates the spellings
// the oracle admits; this walk finds the importers that used one.
//
// Every importer is searched, not only the Python ones, exactly as the oracle
// does: `corvint.imports` is one table and the arm iterates all of it.
func pythonReverseImporters(index *Index, changedPath string) []importer {
	candidates := PythonImportCandidates(changedPath)
	found := make([]importer, 0)
	for importerPath, imports := range index.Imports {
		if importerPath == changedPath {
			continue
		}
		for imported := range imports {
			if _, admitted := candidates[imported]; admitted {
				found = append(found, importer{importerPath, imported})
			}
		}
	}
	return sortImporters(found)
}

// webReverseImporters is rule (c). A web specifier names a file, not a package,
// so resolution runs forwards -- resolve every relative or aliased specifier in
// the repository and keep the ones that land on the changed file -- rather than
// deriving the names the changed file could be imported by.
//
// Two properties are the oracle's and look like defects:
//
//   - There is no `importerPath != changedPath` guard, where the Go and Python
//     arms both have one. A web source that imports its own stem therefore
//     reports itself as its own reverse-importer. Reproduced, not repaired:
//     the corpus expectation is the oracle's bytes (`GPK-V0-033`).
//   - The extensionless target is compared both directly and against an
//     `/index` child, which is how a directory specifier resolves. The oracle
//     compares `resolved + "/index"` after stripping trailing slashes and does
//     not check that the directory form is actually a directory.
func webReverseImporters(index *Index, changedPath string) []importer {
	profile := projectprofile.ByID(index.ProfileID)
	target := pythonWithoutSuffix(changedPath)
	found := make([]importer, 0)
	for importerPath, imports := range index.Imports {
		for imported := range imports {
			if !webSpecifierAdmitted(imported, profile) {
				continue
			}
			resolved := webImportTarget(importerPath, imported, profile)
			if resolved == target || strings.TrimRight(resolved, "/")+"/index" == target {
				found = append(found, importer{importerPath, imported})
			}
		}
	}
	return sortImporters(found)
}

// webSpecifierAdmitted keeps the two specifier shapes that name a file inside
// this repository. A bare specifier names a package from the dependency tree,
// which no reverse-import search over repository sources can resolve.
//
// The alias is the profile's, and a profile that declares none admits no
// aliased specifier. That is the `GPK-V0-027` requirement doing real work: the
// oracle hardcodes one project's source root and would resolve `@/x` into it
// from any repository at all.
func webSpecifierAdmitted(imported string, profile projectprofile.Profile) bool {
	if strings.HasPrefix(imported, ".") {
		return true
	}
	return profile.WebAliasPrefix != "" && strings.HasPrefix(imported, profile.WebAliasPrefix)
}

// webPackageNameImported reports whether some source may reach changedPath
// through the name of the nested package that holds it, which rule (c) cannot
// resolve: a bare specifier such as `@scope/contracts` names a workspace
// package, and the package's entry point and barrel re-exports decide which of
// its files it reaches (GPK-V0-069, proposed). The holding package is the
// nearest directory below the root with an indexed `package.json`; the root
// package is excluded. A manifest whose name cannot be read is reported as
// reachable, because nothing then shows it is not.
func webPackageNameImported(index *Index, changedPath string) bool {
	manifest, found := enclosingPackageManifest(index, changedPath)
	if !found {
		return false
	}
	name, readable := packageManifestName(manifest)
	if !readable {
		return true
	}
	if name == "" {
		return false
	}
	for _, imports := range index.Imports {
		for imported := range imports {
			if imported == name || strings.HasPrefix(imported, name+"/") {
				return true
			}
		}
	}
	return false
}

// enclosingPackageManifest finds the nearest indexed `package.json` in a
// directory that holds changedPath, stopping before the repository root.
func enclosingPackageManifest(index *Index, changedPath string) (Source, bool) {
	for directory := path.Dir(changedPath); directory != "."; directory = path.Dir(directory) {
		if manifest, indexed := index.Sources[directory+"/package.json"]; indexed {
			return manifest, true
		}
	}
	return Source{}, false
}

// packageManifestName reads the `name` field of a `package.json`. It is
// unreadable when the bytes are not loaded, not text, or not a JSON object.
func packageManifestName(manifest Source) (string, bool) {
	text, valid, loaded := manifest.Text()
	if !valid || !loaded {
		return "", false
	}
	var fields struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(text), &fields) != nil {
		return "", false
	}
	return fields.Name, true
}

// webImportTarget resolves one specifier to a repository path with its web
// extension removed, porting `_web_import_target`
// (`src/context_corvint_index.py:1518-1524`). The one deliberate difference is
// the alias root, which comes from the profile instead of the literal
// `internal/web/app/src/` the oracle carries.
//
// The extension is removed only when it is a web extension, so `./theme.css`
// stays `theme.css` and cannot collide with a `theme.ts` beside it.
func webImportTarget(importerPath, imported string, profile projectprofile.Profile) string {
	target := path.Dir(importerPath) + "/" + imported
	if profile.WebAliasPrefix != "" && strings.HasPrefix(imported, profile.WebAliasPrefix) {
		target = profile.WebAliasRoot + imported[len(profile.WebAliasPrefix):]
	}
	target = path.Clean(target)
	if webSuffixes[strings.ToLower(pythonPathSuffix(target))] {
		return pythonWithoutSuffix(target)
	}
	return target
}

// pythonPathSuffix is `PurePosixPath(value).suffix`, which `path.Ext` is not:
// a name whose only dot is its first character is a hidden file with no
// suffix, so `.ts` is a name and not an extension. Both resolution sites here
// ask the oracle's question, so both must get the oracle's answer.
func pythonPathSuffix(value string) string {
	name := path.Base(value)
	dot := strings.LastIndexByte(name, '.')
	if dot <= 0 || dot == len(name)-1 {
		return ""
	}
	return name[dot:]
}

// pythonWithoutSuffix is `str(PurePosixPath(value).with_suffix(""))` for a
// value already in normal form, which every caller's is.
func pythonWithoutSuffix(value string) string {
	return strings.TrimSuffix(value, pythonPathSuffix(value))
}

// sortImporters puts the pairs in the oracle's `sorted(set(reverse))` order.
// The set is what makes the order total: a pair can only be produced once per
// arm, so sorting on the pair alone is a total order over the result.
func sortImporters(found []importer) []importer {
	sort.Slice(found, func(left, right int) bool {
		if found[left].path != found[right].path {
			return found[left].path < found[right].path
		}
		return found[left].imported < found[right].imported
	})
	return found
}
