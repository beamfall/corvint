package appmap

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/rootalias"
)

var appPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// basePathPattern is a literal router basename: one or more unreserved path segments (AMAP-V0-191).
var basePathPattern = regexp.MustCompile(`^(/[A-Za-z0-9._~-]+)+$`)

// validateMounts checks each React mount names another declared React router and a literal
// terminal-splat path, and that mounts form no cycle (AMAP-V0-192). Whether that router declares
// the path is checked when the routers are read.
func validateMounts(routers []RouterFile) error {
	dialect, parent := map[string]string{}, map[string]string{}
	for _, r := range routers {
		dialect[r.Path] = r.Dialect
	}
	for _, r := range routers {
		if r.Mount == nil {
			continue
		}
		mt := r.Mount
		if r.Dialect != DialectReact || dialect[mt.Router] != DialectReact || mt.Router == r.Path {
			return invalidManifest("router %s: mount must name another declared react-router/0 router", r.Path)
		}
		if !strings.HasSuffix(mt.Path, "/*") || !strings.HasPrefix(mt.Path, "/") || !literalRoutePath(mt.Path) {
			return invalidManifest("router %s: mount path must be a literal absolute path ending in /*", r.Path)
		}
		parent[r.Path] = mt.Router
	}
	for p := range parent {
		seen := map[string]bool{}
		for cur := p; cur != ""; cur = parent[cur] {
			if seen[cur] {
				return invalidManifest("router %s: mounts form a cycle", p)
			}
			seen[cur] = true
		}
	}
	return nil
}

func invalidManifest(format string, args ...any) error {
	return &gokernel.Error{Code: "appmap-invalid-manifest", Message: fmt.Sprintf(format, args...)}
}

// decodeManifest reads the closed application-map-manifest/0 document (AMAP-V0-001): unknown or
// duplicate members, secret-shaped content and every path outside the repository are refused.
func decodeManifest(raw []byte) (Manifest, error) {
	var m Manifest
	if len(raw) > maxManifestBytes {
		return m, bound(fmt.Sprintf("manifest exceeds %d bytes", maxManifestBytes))
	}
	if err := appflows.Decode(raw, &m); err != nil {
		return m, invalidManifest("manifest: %v", err)
	}
	if m.Schema != ManifestSchema {
		return m, invalidManifest("manifest schema must be %s", ManifestSchema)
	}
	if !appPattern.MatchString(m.App) {
		return m, invalidManifest("manifest app must match %s", appPattern)
	}
	if m.HashPrefix != "" && m.HashPrefix != "#" && m.HashPrefix != "#!" {
		return m, invalidManifest("manifest hash_prefix must be empty, # or #!")
	}
	if m.BasePath != "" && !basePathPattern.MatchString(m.BasePath) {
		return m, invalidManifest("manifest base_path must be literal /segments with no trailing slash")
	}
	if len(m.Routers) == 0 || len(m.Routers) > maxRouterFiles {
		return m, invalidManifest("manifest routers must list 1 to %d files", maxRouterFiles)
	}
	seen := map[string]bool{}
	for _, r := range m.Routers {
		if !safeRelative(r.Path) || seen[r.Path] {
			return m, invalidManifest("router path %q is not a unique repository-relative path", r.Path)
		}
		seen[r.Path] = true
		if r.Dialect != DialectUIRouter && r.Dialect != DialectRails && r.Dialect != DialectReact {
			return m, invalidManifest("router %s: unsupported dialect %s", r.Path, r.Dialect)
		}
	}
	if err := validateMounts(m.Routers); err != nil {
		return m, err
	}
	if err := validateRouterGlobs(m.RouterGlobs); err != nil {
		return m, err
	}
	if err := contextindex.ValidateRegistryDeclarations(m.Registries); err != nil {
		return m, invalidManifest("manifest %v", err)
	}
	if m.Flows != "" && !safeRelative(m.Flows) {
		return m, invalidManifest("manifest flows must be a repository-relative directory")
	}
	t := m.Tests
	if t.Repo != "" && !rootalias.Valid(t.Repo) {
		return m, invalidManifest("tests.repo must be a root alias matching ^[a-z][a-z0-9-]{0,31}$")
	}
	if t.Root != "." && !safeRelative(t.Root) {
		return m, invalidManifest("tests.root must be a repository-relative directory")
	}
	if len(t.Specs) == 0 {
		return m, invalidManifest("tests.specs must name a directory")
	}
	// page_objects may be empty, and may repeat a specs directory: a flat layout keeps its
	// helpers beside the specs (AMAP-V0-780). Every other repetition is refused.
	dirs, specs := map[string]bool{}, map[string]bool{}
	for g, group := range [][]string{t.Specs, t.PageObjects, t.Workflows, t.Scenarios} {
		inGroup := map[string]bool{}
		for _, d := range group {
			flat := g == 1 && specs[d]
			if !safeRelative(d) || !under(d, t.Root) || inGroup[d] || (dirs[d] && !flat) {
				return m, invalidManifest("test directory %q must be unique and inside tests.root", d)
			}
			dirs[d], inGroup[d] = true, true
			if g == 0 {
				specs[d] = true
			}
		}
	}
	if t.WebRoot != "" && !safeRelative(t.WebRoot) {
		return m, invalidManifest("tests.web_root must be a repository-relative directory")
	}
	bound := map[string]bool{}
	for _, p := range m.PageObjectScreens {
		if !safeRelative(p.Path) || !under(p.Path, t.Root) || bound[p.Path] || strings.TrimSpace(p.State) == "" {
			return m, invalidManifest("page_object_screens entry %q is invalid or repeated", p.Path)
		}
		bound[p.Path] = true
	}
	if m.DIConstants != nil && (len(m.DIConstants) == 0 || len(m.DIConstants) > maxDIScopePaths) {
		return m, invalidManifest("manifest di_constants must list 1 to %d paths", maxDIScopePaths)
	}
	if m.DIClosureLimit != 0 && (m.DIClosureLimit < maxImportClosure || m.DIClosureLimit > maxReachFiles || m.DIConstants == nil) {
		return m, invalidManifest("manifest di_closure_limit must be %d to %d and needs di_constants", maxImportClosure, maxReachFiles)
	}
	scoped := map[string]bool{}
	for _, p := range m.DIConstants {
		if !safeRelative(p) || scoped[p] {
			return m, invalidManifest("di_constants path %q is not a unique repository-relative path", p)
		}
		scoped[p] = true
	}
	return m, nil
}

// under reports whether p is dir or inside it.
// under reports whether p is dir or inside it; dir "." is the whole repository (AMAP-V0-075).
func under(p, dir string) bool { return dir == "." || p == dir || strings.HasPrefix(p, dir+"/") }
