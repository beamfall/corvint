package contextindex

import (
	"fmt"
	"path"
	"strings"
)

// goCaller is one non-test Go reverse importer that names an exported
// declaration of a changed file through its local name for the package: the
// first such code line found, for the first changed path (in sorted order)
// that it calls.
type goCaller struct {
	changedPath, qualified string
	line                   int
}

// recordGoCaller adds importer to callers when it is a direct Go caller of
// changedPath (GPK-V0-075). It changes no score: the caller only
// feeds the omission disclosure. A root-package changed path is out of scope,
// because DR-0017 and the broad-module-root reservation govern its importers.
func recordGoCaller(callers map[string]goCaller, index *Index, changedPath string, importer importer, importLine int) {
	_, known := callers[importer.path]
	if known || isTestPath(importer.path) || !strings.HasSuffix(importer.path, goImpactSuffix) ||
		!strings.HasSuffix(changedPath, goImpactSuffix) || path.Dir(changedPath) == "." {
		return
	}
	text, valid, loaded := index.Sources[importer.path].Text()
	if !loaded || !valid {
		return
	}
	qualifier := goImportQualifier(strings.Split(text, "\n")[importLine-1], importer.imported, goPackageName(index, changedPath))
	if qualifier == "" {
		return
	}
	names := exportedGoNames(index, changedPath)
	for lineIndex, line := range goCodeLines(text) {
		for _, name := range names {
			if lineIndex+1 != importLine && containsPythonWord(line, qualifier+"."+name) {
				callers[importer.path] = goCaller{changedPath, qualifier + "." + name, lineIndex + 1}
				return
			}
		}
	}
}

// goPackageName is the name in changedPath's package clause, or "" when the
// source is unreadable or declares none.
func goPackageName(index *Index, changedPath string) string {
	text, valid, loaded := index.Sources[changedPath].Text()
	if !loaded || !valid {
		return ""
	}
	for _, line := range goCodeLines(text) {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "package" {
			return fields[1]
		}
	}
	return ""
}

// goImportQualifier is the local name an import line binds for imported: the
// alias written before the quoted path, else the package's declared name.
// A blank or dot import binds no qualifier.
func goImportQualifier(line, imported, packageName string) string {
	before, _, found := strings.Cut(line, "\""+imported+"\"")
	if !found {
		return ""
	}
	fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(before), "import"))
	if len(fields) == 0 || fields[len(fields)-1] == "(" {
		return packageName
	}
	alias := fields[len(fields)-1]
	if alias == "_" || alias == "." {
		return ""
	}
	return alias
}

// omittedCallerDisclosures names each direct Go caller whose reverse-import
// row the result limit dropped, with its rank, so an agent reading a packet
// that omits a caller learns which caller, why, and what limit reaches it.
// At most maxEvidence callers are named; the remainder is counted.
func omittedCallerDisclosures(results []map[string]any, callers map[string]goCaller, limit int) []string {
	lines := make([]string, 0)
	unnamed := 0
	for position, result := range results {
		caller, ok := callers[stringValue(result["id"])]
		if position < limit || result["kind"] != "reverse-import" || !ok {
			continue
		}
		if len(lines) == maxEvidence {
			unnamed++
			continue
		}
		lines = append(lines, fmt.Sprintf(
			"direct Go caller %s (line %d names %s declared by %s) ranked %d as a %v-score reverse-import row and was omitted by result limit %d",
			result["id"], caller.line, caller.qualified, caller.changedPath, position+1, result["score"], limit))
	}
	if unnamed != 0 {
		lines = append(lines, fmt.Sprintf("%d more direct Go callers omitted by result limit %d", unnamed, limit))
	}
	return lines
}
