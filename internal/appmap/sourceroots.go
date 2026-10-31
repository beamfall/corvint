// SPDX-License-Identifier: Apache-2.0

package appmap

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/rootalias"
)

// splitSource reads one --source value: ALIAS:DIR names a directory of the root the map pinned
// under ALIAS; any other value is a directory of --root (SKH-V0-189).
func splitSource(s string) (alias, dir string) {
	if a, d, ok := strings.Cut(s, ":"); ok && rootalias.Valid(a) {
		return a, d
	}
	return "", s
}

// mapPin returns the map's pin of alias, or false when the map did not read that root.
func (m *Map) mapPin(alias string) (RootPin, bool) {
	for _, p := range m.Roots {
		if p.Repo == alias {
			return p, true
		}
	}
	return RootPin{}, false
}

// mapRoot opens the repository the map read under alias at the commit the map recorded for it:
// --root at the map revision for "", otherwise the operator-bound o.Repos[alias] at the map's
// RootPin revision, read from committed objects only (SKH-V0-189).
func mapRoot(ctx context.Context, m *Map, alias string, o Options) (repo, string, error) {
	if alias == "" {
		r := repo{ctx: ctx, root: o.Root}
		rev, err := r.resolve(m.Revision)
		if err != nil || rev != m.Revision {
			return repo{}, "", orPlatform(err, &gokernel.Error{Code: "appmap-invalid-revision", Message: "the map revision is not a commit in --root"})
		}
		return r, rev, nil
	}
	pin, ok := m.mapPin(alias)
	if !ok {
		return repo{}, "", rootUnavailable(alias, "is not a root the map read")
	}
	r, rev, err := openRoot(ctx, o.Root, alias, o.Repos[alias], pin.Revision)
	if err != nil {
		return repo{}, "", err
	}
	if rev != pin.Revision {
		return repo{}, "", rootUnavailable(alias, "revision "+pin.Revision+" the map recorded is not a commit of the root")
	}
	return r, rev, nil
}

// sourceRoot is one repository a source verb reads: the root, the map's revision of it and the
// declared directories under it.
type sourceRoot struct {
	alias string
	r     repo
	rev   string
	dirs  []string
}

// openSources validates the --source values and opens each root they name at the map's revision of
// it, checking every directory is a tree there. Roots are returned with --root first, then by alias.
func openSources(ctx context.Context, m *Map, sources []string, o Options) ([]*sourceRoot, error) {
	query := func(msg string) error { return &gokernel.Error{Code: "appmap-invalid-query", Message: msg} }
	seen, byAlias := map[string]bool{}, map[string]*sourceRoot{}
	for _, s := range sources {
		alias, dir := splitSource(s)
		key := alias + "\x00" + dir
		if !safeRelative(dir) || seen[key] {
			return nil, query(fmt.Sprintf("--source %q is not a unique repository-relative directory", s))
		}
		seen[key] = true
		if byAlias[alias] == nil {
			byAlias[alias] = &sourceRoot{alias: alias}
		}
		byAlias[alias].dirs = append(byAlias[alias].dirs, dir)
	}
	aliases := make([]string, 0, len(byAlias))
	for a := range byAlias {
		aliases = append(aliases, a)
	}
	sort.Strings(aliases)
	out := make([]*sourceRoot, 0, len(aliases))
	for _, a := range aliases {
		sr := byAlias[a]
		r, rev, err := mapRoot(ctx, m, a, o)
		if err != nil {
			return nil, err
		}
		sr.r, sr.rev = r, rev
		missing, err := r.missingDirs(rev, sr.dirs)
		if err != nil {
			return nil, err
		}
		if len(missing) > 0 {
			if a != "" {
				return nil, query(fmt.Sprintf("--source %q is not a directory at root %s's map revision", a+":"+missing[0], a))
			}
			msg := fmt.Sprintf("--source %q is not a directory at the map revision", missing[0])
			if _, ok := m.mapPin(missing[0]); ok {
				msg += fmt.Sprintf("; %s is a root the map read: pass --source %s:DIR with --repo %s=ABSOLUTE_ROOT", missing[0], missing[0], missing[0])
			}
			return nil, query(msg)
		}
		out = append(out, sr)
	}
	return out, nil
}

// sourceTree lists the regular files under every opened root's directories. One path read from two
// roots refuses: site-knowledge records are keyed by repository-relative path (SKH-V0-189).
func sourceTree(roots []*sourceRoot, limit int) ([]blobEntry, error) {
	all, err := sourceEntries(roots, limit)
	if err != nil {
		return nil, err
	}
	owner := map[string]string{}
	for _, e := range all {
		if prev, ok := owner[e.path]; ok && prev != e.repo {
			return nil, &gokernel.Error{Code: "appmap-invalid-query", Message: fmt.Sprintf("%s is read from two roots (%s and %s)", e.path, rootName(prev), rootName(e.repo))}
		}
		owner[e.path] = e.repo
	}
	return all, nil
}

// sourceEntries lists the regular files under every opened root's directories within one shared
// limit; each entry carries its root, so a reader that keys records by root and path (the endpoint
// join, AMAP-V0-222) may read one path from two roots.
func sourceEntries(roots []*sourceRoot, limit int) ([]blobEntry, error) {
	all := []blobEntry{}
	for _, sr := range roots {
		entries, err := sr.r.tree(sr.rev, sr.dirs, limit-len(all))
		if err != nil {
			return nil, err
		}
		all = append(all, entries...)
	}
	return all, nil
}

func rootName(alias string) string {
	if alias == "" {
		return "--root"
	}
	return alias
}
