package main

import (
	"path/filepath"

	"github.com/Beamfall/corvint/internal/liveverify/jsresolve"
)

// jsSuffixes is the web set `reference-resolves` judges under FPK-V0-018,
// the same six the index's web import lexer owns. A `.d.ts` path ends in
// `.ts` and is judged like any other TypeScript blob.
var jsSuffixes = map[string]bool{".js": true, ".mjs": true, ".cjs": true, ".jsx": true, ".ts": true, ".tsx": true}

func jsPath(path string) bool {
	return jsSuffixes[filepath.Ext(path)]
}

// referenceResolves dispatches the reference claim on the citing file's
// language: the identifier must sit on the cited line of the citing blob and
// the declaring blob must declare it at top level.
func referenceResolves(path string, citing, declaring []byte, name string, line int) bool {
	if jsPath(path) {
		return jsresolve.IdentifierAtLine(citing, name, line) && jsresolve.DeclaresAtTopLevel(declaring, name)
	}
	return goIdentifierAtLine(citing, name, line) && goDeclaresAtTopLevel(declaring, name)
}

// jsImportsAtLine accepts the specifier as the index recorded it: an ESM
// import, export-from, dynamic import, or require whose literal is exactly
// `imported` and whose statement spans the cited line.
func jsImportsAtLine(content []byte, imported string, line int) bool {
	return jsresolve.ImportsAtLine(content, imported, line)
}
