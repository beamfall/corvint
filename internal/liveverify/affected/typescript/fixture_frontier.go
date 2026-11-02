// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// fixtureDirectory returns the outermost `testdata` directory a repository-relative path lies
// below, such as `internal/appmap/testdata` for `internal/appmap/testdata/e2e/a.spec.ts`, or ""
// when the path has no `testdata` component (AFP-V0-446).
func fixtureDirectory(relative string) string {
	components := strings.Split(relative, "/")
	for index, component := range components[:max(len(components)-1, 0)] {
		if component == "testdata" {
			return strings.Join(components[:index+1], "/")
		}
	}
	return ""
}

// localizeFixtureFrontier moves every located, unpinned FrontierUnresolvedImport site whose file a
// unit under a `testdata` directory owns, and whose relative specifier names a path inside that
// `testdata` directory, from the plugin-wide set onto that unit's own Frontier (AFP-V0-446). Such
// a site names a file that does not exist inside the fixture, so no dirty path outside the fixture
// can be what it loads. Every other site stays plugin-wide: a specifier that leaves the directory,
// a bare or aliased one, a computed import, whose target nothing bounds, and every other reason. A
// fixture unit also stays plugin-wide, with every site in it, when an import edge crosses its
// `testdata` directory's boundary in either direction anywhere in the units it is connected to by
// imports: a unit outside the directory that imports one inside, or one inside that imports one
// outside, such as a server stub the fixture suite drives. A site no unit owns, such as a file of
// an unidentified runner, and a pinned site stay plugin-wide. A reason whose every site moved
// leaves the plugin-wide set; a reason raised without a site is unchanged.
//
// A located, unpinned FrontierE2ERuntimeDependency site of such a unit moves the same way, unless
// an end-to-end runner config whose directory encloses the site's file lies outside the unit's
// `testdata` directory, among e2eConfigs, the directories of every end-to-end runner config read
// (AFP-V0-725): such a config could collect the file into a suite the repository runs itself. The
// site stands for a server or binary the end-to-end run drives; below `testdata` that run is one
// the enclosing package's tests start, so what it drives is the fixture's own bytes or a binary
// those tests build or execute, edges the plugin of those tests models. The graph raises the
// moved site back to the plugin-wide frontier where no Go unit hosts the directory
// (promoteUnhostedFixtureFrontier), as for every localized site.
func localizeFixtureFrontier(result *affected.Result, frontier frontierSet, e2eConfigs []string) {
	owner := make(map[string]int, len(result.Units))
	byID := make(map[string]int, len(result.Units))
	for index, unit := range result.Units {
		byID[unit.ID] = index
		for _, relative := range append(append([]string(nil), unit.Sources...), unit.Tests...) {
			owner[relative] = index
		}
	}
	fixtures := make([]string, len(result.Units))
	for index, unit := range result.Units {
		if len(unit.Sources) != 0 || len(unit.Tests) != 0 {
			fixtures[index] = fixtureDirectory(firstPath(unit))
		}
	}
	for index, unit := range result.Units {
		// A workspace package unit names no file; it lies where its members
		// lie, and outside every fixture when they do not share one.
		if len(unit.Sources) != 0 || len(unit.Tests) != 0 {
			continue
		}
		for _, target := range unit.Imports {
			member, known := byID[target]
			if !known || len(result.Units[member].Sources) == 0 && len(result.Units[member].Tests) == 0 {
				continue
			}
			if fixtures[index] == "" {
				fixtures[index] = fixtures[member]
			} else if fixtures[index] != fixtures[member] {
				fixtures[index] = ""
				break
			}
		}
	}
	escaped := escapedFixtureUnits(result.Units, byID, fixtures)
	move := func(reason string, eligible func(file, token, fixture string) bool) {
		sites := frontier[reason]
		for site, token := range sites {
			if strings.HasPrefix(token, pinnedSite) {
				continue
			}
			file := site
			index, owned := owner[site]
			if !owned {
				file = siteFile(site)
				index, owned = owner[file]
			}
			if !owned || fixtures[index] == "" || escaped[index] || !eligible(file, token, fixtures[index]) {
				continue
			}
			unit := &result.Units[index]
			unit.Frontier = append(unit.Frontier, reason+":"+siteWithToken(site, token))
			delete(sites, site)
		}
		if sites != nil && len(sites) == 0 {
			delete(frontier, reason)
		}
	}
	move(FrontierUnresolvedImport, insideFixture)
	move(FrontierE2ERuntimeDependency, func(file, _, fixture string) bool {
		return !collectedOutside(file, fixture, e2eConfigs)
	})
	for index := range result.Units {
		sort.Strings(result.Units[index].Frontier)
	}
}

// collectedOutside reports whether a config directory among directories encloses file but lies
// outside fixture, so a runner started there could collect file (AFP-V0-725).
func collectedOutside(file, fixture string, directories []string) bool {
	for _, directory := range directories {
		if inside(file, directory) && directory != fixture && !strings.HasPrefix(directory, fixture+"/") {
			return true
		}
	}
	return false
}

// insideFixture reports whether the specifier spelled at a site of file names a path inside
// fixture: a `./` or `../` specifier resolved against the file's directory, with a query or
// fragment dropped as resolveImport drops it. A bare or aliased specifier, or one whose path
// leaves the directory, does not.
func insideFixture(file, specifier, fixture string) bool {
	clean := strings.SplitN(strings.SplitN(specifier, "?", 2)[0], "#", 2)[0]
	if !strings.HasPrefix(clean, "./") && !strings.HasPrefix(clean, "../") {
		return false
	}
	return strings.HasPrefix(path.Join(path.Dir(file), clean), fixture+"/")
}

// escapedFixtureUnits marks each fixture unit connected by import edges, in either direction, to
// an import edge whose two ends lie in different fixture directories or one of them outside every
// fixture: both ends of such an edge escape, and escape spreads along the edges that stay inside
// one fixture directory until no unit changes.
func escapedFixtureUnits(units []affected.Unit, byID map[string]int, fixtures []string) []bool {
	escaped := make([]bool, len(units))
	for index, unit := range units {
		for _, target := range unit.Imports {
			if to, known := byID[target]; known && fixtures[to] != fixtures[index] {
				escaped[index], escaped[to] = true, true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for index, unit := range units {
			if fixtures[index] == "" {
				continue
			}
			for _, target := range unit.Imports {
				if to, known := byID[target]; known && fixtures[to] == fixtures[index] && escaped[to] != escaped[index] {
					escaped[index], escaped[to], changed = true, true, true
				}
			}
		}
	}
	return escaped
}

// siteFile strips the `:line` suffix of a located site, leaving its file.
func siteFile(site string) string {
	index := strings.LastIndex(site, ":")
	if index < 0 || index == len(site)-1 || strings.Trim(site[index+1:], "0123456789") != "" {
		return site
	}
	return site[:index]
}
