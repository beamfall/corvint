package contextindex

// This file exposes the index's existing JavaScript/TypeScript import relation to consumers
// outside the package (the application map, AMAP-V0-005) without a second resolver: both functions
// delegate to the code the index itself uses, so a change to web specifier resolution reaches every
// consumer at once.

// WebImportBinding is one local name a static named import binds.
type WebImportBinding struct {
	Local, Module, Exported string
	Line                    int
}

// WebImportBindings returns the value names a JavaScript or TypeScript source binds through static
// named imports, ordered by local name. Type-only items bind nothing and are absent.
func WebImportBindings(text string) []WebImportBinding {
	bindings := webImportBindings(text)
	out := make([]WebImportBinding, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, WebImportBinding{Local: b.local, Module: b.module, Exported: b.exported, Line: b.line})
	}
	return out
}

// ResolveWebImport resolves one specifier written in importerPath to the single indexed source it
// names, or to the empty string when it names none or several.
func ResolveWebImport(index *Index, importerPath, specifier string) string {
	return resolvedWebSource(index, importerPath, specifier)
}
