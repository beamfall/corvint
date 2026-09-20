package typescript

import (
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// A global hook affects every configured test, including when only a helper
// imported by that hook changed. Use the config unit as the shared dependency.
func bindPlaywrightGlobalHooks(configPath, source string, result *affected.Result) []PlaywrightUnknown {
	clean, err := stripComments(source, false)
	if err != nil {
		return nil
	}
	object, ok := playwrightConfigObject(clean)
	if !ok {
		return nil
	}
	properties, ok := playwrightObjectProperties(object)
	if !ok {
		return nil
	}
	paths := map[string]string{}
	configIndex := -1
	for index, unit := range result.Units {
		for _, name := range append(append([]string{}, unit.Sources...), unit.Tests...) {
			paths[name] = unit.ID
			if name == configPath {
				configIndex = index
			}
		}
	}
	var unknown []PlaywrightUnknown
	for _, key := range []string{"globalSetup", "globalTeardown"} {
		raw, exists := properties[key]
		if !exists {
			continue
		}
		values, ok := playwrightStringArray(raw)
		if value, literal := playwrightString(raw); literal {
			values, ok = []string{value}, true
		}
		if !ok || len(values) == 0 || configIndex < 0 {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownConfigSyntax, key+" is not a static source path"))
			continue
		}
		for _, value := range values {
			if !playwrightRelativeDirectory(value) || value == "." {
				unknown = append(unknown, selectionUnknown(PlaywrightUnknownConfigSyntax, key+" path is unsupported"))
				continue
			}
			reference := value
			if !strings.HasPrefix(reference, ".") {
				reference = "./" + reference
			}
			target, _, unresolved := resolveImport(configPath, reference, paths)
			if unresolved || target == "" || path.Clean(path.Join(path.Dir(configPath), value)) == configPath {
				unknown = append(unknown, selectionUnknown(PlaywrightUnknownConfigSyntax, key+" source is unresolved"))
				continue
			}
			result.Units[configIndex].Imports = sortedUnique(append(result.Units[configIndex].Imports, target))
		}
	}
	return unknown
}
