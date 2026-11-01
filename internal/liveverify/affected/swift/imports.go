package swift

import "strings"

// ImportedModule is one module a Swift import declaration names, with the
// 1-based line of the declaration.
type ImportedModule struct {
	Module string
	Line   int
}

// ImportedModules lists the modules the import declarations of one Swift
// source name, in source order. It reads the pattern target inspection uses
// for a target's raw imports, over the comment- and string-masked code, so a
// quoted or commented `import` is not read as a dependency (NGI-V0-980).
func ImportedModules(text string) []ImportedModule {
	code := swiftCodeMask(text)
	matches := importPattern.FindAllStringSubmatchIndex(code, -1)
	imported := make([]ImportedModule, 0, len(matches))
	for _, match := range matches {
		imported = append(imported, ImportedModule{
			Module: code[match[2]:match[3]],
			Line:   strings.Count(code[:match[2]], "\n") + 1,
		})
	}
	return imported
}

// TargetModule is the module name of a SwiftPM unit ID the plugin emits
// (`swift:spm:<package directory>/<target>`), and false for any other unit.
func TargetModule(unitID string) (string, bool) {
	rest, ok := strings.CutPrefix(unitID, "swift:spm:")
	if !ok {
		return "", false
	}
	cut := strings.LastIndexByte(rest, '/')
	if cut < 0 || cut == len(rest)-1 {
		return "", false
	}
	return rest[cut+1:], true
}
