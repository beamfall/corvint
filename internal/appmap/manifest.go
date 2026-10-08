package appmap

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/rootalias"
)

var appPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

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
	if len(m.Routers) == 0 || len(m.Routers) > maxRouterFiles {
		return m, invalidManifest("manifest routers must list 1 to %d files", maxRouterFiles)
	}
	seen := map[string]bool{}
	for _, r := range m.Routers {
		if !safeRelative(r.Path) || seen[r.Path] {
			return m, invalidManifest("router path %q is not a unique repository-relative path", r.Path)
		}
		seen[r.Path] = true
		if r.Dialect != DialectUIRouter {
			return m, invalidManifest("router %s: dialect must be %s", r.Path, DialectUIRouter)
		}
	}
	if m.Flows != "" && !safeRelative(m.Flows) {
		return m, invalidManifest("manifest flows must be a repository-relative directory")
	}
	t := m.Tests
	if t.Repo != "" && !rootalias.Valid(t.Repo) {
		return m, invalidManifest("tests.repo must be a root alias matching ^[a-z][a-z0-9-]{0,31}$")
	}
	if !safeRelative(t.Root) {
		return m, invalidManifest("tests.root must be a repository-relative directory")
	}
	if len(t.Specs) == 0 || len(t.PageObjects) == 0 {
		return m, invalidManifest("tests.specs and tests.page_objects must each name a directory")
	}
	dirs := map[string]bool{}
	for _, group := range [][]string{t.Specs, t.PageObjects, t.Workflows, t.Scenarios} {
		for _, d := range group {
			if !safeRelative(d) || !under(d, t.Root) || dirs[d] {
				return m, invalidManifest("test directory %q must be unique and inside tests.root", d)
			}
			dirs[d] = true
		}
	}
	bound := map[string]bool{}
	for _, p := range m.PageObjectScreens {
		if !safeRelative(p.Path) || !under(p.Path, t.Root) || bound[p.Path] || strings.TrimSpace(p.State) == "" {
			return m, invalidManifest("page_object_screens entry %q is invalid or repeated", p.Path)
		}
		bound[p.Path] = true
	}
	return m, nil
}

// under reports whether p is dir or inside it.
func under(p, dir string) bool { return p == dir || strings.HasPrefix(p, dir+"/") }
