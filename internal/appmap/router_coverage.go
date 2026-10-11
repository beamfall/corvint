package appmap

import (
	"path"
	"strings"
)

// AppCoverage describes only the manifest's declared discovery scope (AMAP-V0-045).
type AppCoverage struct {
	Status     string   `json:"status"`
	Patterns   []string `json:"patterns"`
	Matched    []string `json:"matched"`
	Declared   []string `json:"declared"`
	Undeclared []string `json:"undeclared"`
}

func validateRouterGlobs(globs []string) error {
	if globs == nil {
		return nil
	}
	if len(globs) == 0 || len(globs) > maxRouterFiles {
		return invalidManifest("router_globs must list 1 to 64 patterns")
	}
	seen := map[string]bool{}
	for _, g := range globs {
		if !safeRelative(g) || seen[g] || strings.Contains(g, "\\") {
			return invalidManifest("invalid or repeated router glob %q", g)
		}
		seen[g] = true
		for _, segment := range strings.Split(g, "/") {
			if strings.Contains(segment, "**") && segment != "**" {
				return invalidManifest("** must occupy a whole glob segment")
			}
			if _, err := path.Match(segment, ""); err != nil {
				return invalidManifest("invalid router glob %q", g)
			}
		}
	}
	return nil
}

// globRoute uses dynamic programming so several ** segments cannot cause exponential matching.
func globRoute(pattern, name string) bool {
	p, n := strings.Split(pattern, "/"), strings.Split(name, "/")
	row := make([]bool, len(n)+1)
	row[0] = true
	for _, segment := range p {
		next := make([]bool, len(n)+1)
		for j := 0; j <= len(n); j++ {
			if segment == "**" {
				next[j] = row[j] || (j > 0 && next[j-1])
			} else if j > 0 {
				match, _ := path.Match(segment, n[j-1])
				next[j] = row[j-1] && match
			}
		}
		row = next
	}
	return row[len(n)]
}

func (b *builder) routerCoverage(r repo) error {
	if len(b.m.RouterGlobs) == 0 {
		return nil
	}
	entries, err := r.tree(b.rev, []string{"."}, maxTestFiles)
	if err != nil {
		return err
	}
	c := &AppCoverage{Status: StatusResolved, Patterns: uniqueSorted(b.m.RouterGlobs), Matched: []string{}, Declared: []string{}, Undeclared: []string{}}
	declared := map[string]bool{}
	for _, rf := range b.m.Routers {
		declared[rf.Path] = true
		c.Declared = append(c.Declared, rf.Path)
	}
	for _, e := range entries {
		for _, g := range c.Patterns {
			if !globRoute(g, e.path) {
				continue
			}
			c.Matched = append(c.Matched, e.path)
			if !declared[e.path] {
				c.Status = StatusUnknown
				c.Undeclared = append(c.Undeclared, e.path)
				b.unknown(Unknown{Kind: "router", Ref: e.path, Path: e.path, Reason: "router-not-declared"})
			}
			break
		}
	}
	c.Matched, c.Declared, c.Undeclared = uniqueSorted(c.Matched), uniqueSorted(c.Declared), uniqueSorted(c.Undeclared)
	b.out.AppCoverage = c
	return nil
}
