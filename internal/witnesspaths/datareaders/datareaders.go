// Package datareaders finds the readers of a data path a witness path policy
// declares (AGW-V0-044) in a head-revision index. It is apart from
// witnesspaths so that a package the index's own tests import, such as
// internal/cem/workflow, can decode the policy without importing the index.
package datareaders

import (
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/witnesspaths"
	"github.com/Beamfall/corvint/internal/contextindex"
)

const maxFoundReaders = 16

// Find lists the declared readers, each marked untracked when
// the head tree lacks it, self when it is the data path, no-extractor when no
// extractor reads its suffix, and unusable with the refusal when it is not a
// usable source (readerRefusal), then the readers found by quoted path. Only
// declared and found readers count (AGW-V0-044).
func Find(dataPath string, declared []string, index *contextindex.Index, unparsed map[string]string) []witnesspaths.Reader {
	result := []witnesspaths.Reader{}
	seen := map[string]bool{}
	for _, name := range declared {
		if seen[name] {
			continue
		}
		seen[name] = true
		reader := witnesspaths.Reader{Path: name, Origin: "declared"}
		_, tracked := index.Tracked[name]
		_, gitlink := index.Skipped[name]
		switch tier, _ := witnesspaths.LanguageCoverage(name); {
		case !tracked || gitlink:
			reader.Origin = "untracked"
		case name == dataPath:
			reader.Origin = "self"
		case tier == "NONE":
			reader.Origin = "no-extractor"
		default:
			if refusal := readerRefusal(name, index, unparsed); refusal != "" {
				reader.Origin, reader.Reason = "unusable", refusal
			}
		}
		result = append(result, reader)
	}
	for _, name := range findReaders(dataPath, index, unparsed) {
		if !seen[name] {
			result = append(result, witnesspaths.Reader{Path: name, Origin: "found"})
		}
	}
	return result
}

// readerRefusal is why a tracked path with an extractor is no usable reader,
// or "": the index did not admit it as a source (an excluded, generated or
// oversized file), it is not a regular file, or its extraction was refused.
// A reader whose facts are absent is no evidence that it reads anything, so it
// must not admit a data path (AGW-V0-044).
func readerRefusal(name string, index *contextindex.Index, unparsed map[string]string) string {
	source, admitted := index.Sources[name]
	if !admitted {
		for _, exclusion := range index.Exclusions {
			if exclusion.Path == name {
				return "NOT_ADMITTED: " + exclusion.Reason
			}
		}
		return "NOT_ADMITTED: the index holds no source for this path"
	}
	switch source.Mode {
	case "100644", "100755":
	case "120000":
		return "SYMLINK_OUT_OF_PROFILE"
	default:
		return "FILE_MODE_OUT_OF_PROFILE: " + source.Mode
	}
	if reason, dropped := unparsed[name]; dropped {
		return reason
	}
	return ""
}

// findReaders names, in path order and at most maxFoundReaders, the usable
// head-tree sources with an extractor whose text quotes dataPath,
// repository-relative or relative to the source's own directory (AGW-V0-044).
func findReaders(dataPath string, index *contextindex.Index, unparsed map[string]string) []string {
	names := make([]string, 0, len(index.Sources))
	for name := range index.Sources {
		if name != dataPath {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	found := []string{}
	for _, name := range names {
		if tier, _ := witnesspaths.LanguageCoverage(name); tier == "NONE" || readerRefusal(name, index, unparsed) != "" {
			continue
		}
		text, valid, loaded := index.Sources[name].Text()
		if !valid || !loaded {
			continue
		}
		forms := []string{dataPath}
		if dir := path.Dir(name); dir != "." && strings.HasPrefix(dataPath, dir+"/") {
			forms = append(forms, strings.TrimPrefix(dataPath, dir+"/"))
		}
		if quotes(text, forms) {
			found = append(found, name)
			if len(found) == maxFoundReaders {
				break
			}
		}
	}
	return found
}

func quotes(text string, forms []string) bool {
	for _, form := range forms {
		for _, quote := range []string{`"`, `'`, "`"} {
			if strings.Contains(text, quote+form+quote) {
				return true
			}
		}
	}
	return false
}
