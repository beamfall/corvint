// SPDX-License-Identifier: Apache-2.0
package python

import (
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// importRootsNarrowing is the enable switch of SLF-V1-001. It stays false until
// the rule's FIH-V1 report passes (SLF-V1-006): resolving an import through a
// configured or layout root discharges the SLF-V1-002 frontier that import
// would otherwise raise, which narrows the plan. While it is false the plugin
// reads no configuration and resolves against the fixed roots alone.
var importRootsNarrowing = false

// pytestConfigNames is pytest's per-directory configuration search order, the
// one `corvint affected`'s advisory pytest check reads.
var pytestConfigNames = []string{"pytest.ini", ".pytest.ini", "pyproject.toml", "tox.ini", "setup.cfg"}

// pytestSections names, per configuration file, the section that holds pytest's
// options. pytest.ini and .pytest.ini are pytest's even without one.
var pytestSections = map[string]string{
	"pytest.ini":     "pytest",
	".pytest.ini":    "pytest",
	"pyproject.toml": "tool.pytest.ini_options",
	"tox.ini":        "pytest",
	"setup.cfg":      "tool:pytest",
}

var (
	configSection = regexp.MustCompile(`^\[\s*([^\]]+?)\s*\]`)
	configKey     = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*)[ \t]*[=:][ \t]*(.*)$`)
	configString  = regexp.MustCompile(`"([^"]*)"|'([^']*)'`)
)

// importRoots lists, sorted and unique, the repository-relative import roots of
// SLF-V1-001 other than the repository root and the fixed SourceRoots: the
// directory of each pytest configuration file (its rootdir), each configured
// `pythonpath` entry below it, the nearest non-package ancestor of each
// `testpaths` entry and of each Python file. A configuration file that cannot
// be read adds nothing; the layout roots still apply.
func importRoots(root *affected.Source, files []string) []string {
	packages := make(map[string]bool, len(files)/4)
	for _, relative := range files {
		if path.Base(relative) == "__init__.py" {
			packages[path.Dir(relative)] = true
		}
	}
	roots := make(map[string]bool)
	add := func(directory string) {
		if directory != "." && directory != "" && affected.ValidRelativePath(directory) {
			roots[directory] = true
		}
	}
	for _, relative := range files {
		add(nearestNonPackage(path.Dir(relative), packages))
	}
	configs, err := root.Files(func(_, name string) bool { return slices.Contains(pytestConfigNames, name) })
	if err != nil {
		return sortedKeys(roots)
	}
	for _, config := range configs {
		body, readErr := root.Read(config)
		if readErr != nil {
			continue
		}
		values, declared := pytestOptions(path.Base(config), string(body))
		if !declared {
			continue
		}
		directory := path.Dir(config)
		add(directory)
		for _, entry := range values["pythonpath"] {
			if joined, ok := below(directory, entry); ok {
				add(joined)
			}
		}
		for _, entry := range values["testpaths"] {
			if joined, ok := below(directory, entry); ok {
				add(nearestNonPackage(joined, packages))
			}
		}
	}
	return sortedKeys(roots)
}

// nearestNonPackage climbs from directory while it is a package (holds an
// `__init__.py`): the first directory that is not one is where pytest's
// prepend import mode puts a module's root on the path.
func nearestNonPackage(directory string, packages map[string]bool) string {
	for directory != "." && packages[directory] {
		directory = path.Dir(directory)
	}
	return directory
}

// below joins a configured entry to its configuration's directory, refusing an
// absolute entry or one that climbs out of the repository.
func below(directory, entry string) (string, bool) {
	entry = strings.TrimSpace(entry)
	if entry == "" || strings.HasPrefix(entry, "/") || strings.Contains(entry, "\\") {
		return "", false
	}
	joined := path.Join(directory, entry)
	if joined == ".." || strings.HasPrefix(joined, "../") {
		return "", false
	}
	return joined, true
}

// pytestOptions reads the `pythonpath` and `testpaths` entries of the pytest
// section of one configuration file, and whether the file is pytest's
// configuration (it holds the section, or it is a pytest.ini). An INI value is
// whitespace separated and continues on indented lines; a TOML value is a
// string or an array of strings, possibly over several lines.
func pytestOptions(name, body string) (map[string][]string, bool) {
	want := pytestSections[name]
	toml := strings.HasSuffix(name, ".toml")
	declared := name == "pytest.ini" || name == ".pytest.ini"
	values := map[string][]string{}
	section, key := "", ""
	var pending strings.Builder
	flush := func() {
		if key == "" {
			return
		}
		text := pending.String()
		if toml {
			for _, match := range configString.FindAllStringSubmatch(text, -1) {
				values[key] = append(values[key], match[1]+match[2])
			}
		} else {
			values[key] = append(values[key], strings.Fields(text)...)
		}
		key = ""
		pending.Reset()
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if key != "" && toml && strings.Contains(pending.String(), "[") && !strings.Contains(pending.String(), "]") {
			pending.WriteString(" " + trimmed)
			continue
		}
		if key != "" && !toml && trimmed != "" && line != strings.TrimLeft(line, " \t") {
			pending.WriteString(" " + trimmed)
			continue
		}
		flush()
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if match := configSection.FindStringSubmatch(trimmed); match != nil {
			section = match[1]
			if section == want {
				declared = true
			}
			continue
		}
		if section != want {
			continue
		}
		if match := configKey.FindStringSubmatch(trimmed); match != nil && (match[1] == "pythonpath" || match[1] == "testpaths") {
			key = match[1]
			value := match[2]
			if cut := strings.Index(value, " #"); cut >= 0 && toml {
				value = value[:cut]
			}
			pending.WriteString(value)
		}
	}
	flush()
	return values, declared
}

// rootAliases lists the dotted names relative names under each of roots, the
// SLF-V1-001 aliases a file adds beside moduleAliases. An empty roots, the
// held rule, adds none.
func rootAliases(relative string, roots []string) []string {
	if len(roots) == 0 {
		return nil
	}
	var aliases []string
	for directory := path.Dir(relative); directory != "."; directory = path.Dir(directory) {
		if _, found := slices.BinarySearch(roots, directory); !found {
			continue
		}
		rest := strings.TrimPrefix(relative, directory+"/")
		if rest == "__init__.py" {
			continue
		}
		aliases = append(aliases, primaryModuleName(rest))
	}
	return aliases
}
