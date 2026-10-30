package ruby

import (
	json "encoding/json/v2"
	"errors"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

const (
	// FrontierDeclarationPath is the repository's declaration of the located
	// frontier sites it has resolved (RUBY-AFFECTED-V0-048).
	FrontierDeclarationPath = ".corvint/ruby-frontiers.json"
	// FrontierDeclarationSchema is the only schema the declaration takes.
	FrontierDeclarationSchema = "ruby-frontiers/0"
	// FrontierDeclarationInvalid reports a present declaration that cannot
	// be read, so it resolves nothing.
	FrontierDeclarationInvalid = "ruby:frontier-declaration-invalid"
	// maxFrontierDeclarationBytes bounds the declaration read.
	maxFrontierDeclarationBytes = 1 << 20
	// maxSitesPerReason bounds how many sites of one reason a frontier list
	// names; the rest are counted in one "<reason>:+<n>" entry.
	maxSitesPerReason = 16
)

// declarableReasons are the reasons a declaration can resolve. A file that
// cannot be read, Cucumber ownership and an invalid declaration are never
// resolved by assertion (RUBY-AFFECTED-V0-048).
var declarableReasons = map[string]bool{
	FrontierDynamicLoad:            true,
	FrontierConditionalLoad:        true,
	FrontierDynamicTestDefinition:  true,
	FrontierAmbiguousFramework:     true,
	FrontierRelativeLoad:           true,
	FrontierImplicitTestDependency: true,
	FrontierRailsAutoload:          true,
}

type frontierDeclaration struct {
	Schema   string   `json:"schema"`
	Resolved []string `json:"resolved"`
}

// readFrontierDeclaration returns the located sites the repository declares
// resolved. An absent declaration resolves nothing and is valid; a present
// one that is unreadable, not strict JSON, of another schema, or names an
// entry that is not one declarable located site is invalid and resolves
// nothing (RUBY-AFFECTED-V0-048).
func readFrontierDeclaration(root *affected.Source) (map[string]bool, bool) {
	body, err := root.Read(FrontierDeclarationPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, true
	}
	if err != nil || len(body) > maxFrontierDeclarationBytes {
		return nil, false
	}
	var decoded frontierDeclaration
	if json.Unmarshal(body, &decoded, json.RejectUnknownMembers(true)) != nil || decoded.Schema != FrontierDeclarationSchema || decoded.Resolved == nil {
		return nil, false
	}
	resolved := make(map[string]bool, len(decoded.Resolved))
	for _, entry := range decoded.Resolved {
		reason, _, _, ok := splitSite(entry)
		if !ok || !declarableReasons[reason] || resolved[entry] {
			return nil, false
		}
		resolved[entry] = true
	}
	return resolved, true
}

// splitSite parses "<reason>:<path>[:<line>]", where reason is
// "ruby:<name>".
func splitSite(entry string) (string, string, int, bool) {
	parts := strings.SplitN(entry, ":", 3)
	if len(parts) != 3 || parts[0] != "ruby" || parts[1] == "" {
		return "", "", 0, false
	}
	reason, rest := parts[0]+":"+parts[1], parts[2]
	line := 0
	if cut := strings.LastIndex(rest, ":"); cut >= 0 {
		if number, err := strconv.Atoi(rest[cut+1:]); err == nil && number > 0 && strconv.Itoa(number) == rest[cut+1:] {
			rest, line = rest[:cut], number
		}
	}
	if !affected.ValidRelativePath(rest) {
		return "", "", 0, false
	}
	return reason, rest, line, true
}

// undeclared returns, sorted and bounded per reason, the sites no declared
// entry resolves: an entry naming the site exactly, or naming its reason and
// path without a line, which resolves every site of that reason in the file.
func undeclared(sites map[string]bool, resolved map[string]bool) []string {
	kept := make(map[string]bool, len(sites))
	for site := range sites {
		if resolved[site] {
			continue
		}
		if reason, relative, line, ok := splitSite(site); ok && line > 0 && resolved[reason+":"+relative] {
			continue
		}
		kept[site] = true
	}
	return boundSites(kept)
}

// boundSites names at most maxSitesPerReason sites of each reason, in order,
// and counts the rest in one "<reason>:+<n>" entry (RUBY-AFFECTED-V0-046).
func boundSites(sites map[string]bool) []string {
	ordered := sortedKeys(sites)
	if len(ordered) == 0 {
		return nil
	}
	counts := make(map[string]int)
	extra := make(map[string]int)
	bounded := make([]string, 0, len(ordered))
	for _, site := range ordered {
		reason := siteReason(site)
		counts[reason]++
		if counts[reason] > maxSitesPerReason {
			extra[reason]++
			continue
		}
		bounded = append(bounded, site)
	}
	for reason, count := range extra {
		bounded = append(bounded, reason+":+"+strconv.Itoa(count))
	}
	sort.Strings(bounded)
	return bounded
}

func siteReason(site string) string {
	parts := strings.SplitN(site, ":", 3)
	if len(parts) < 2 {
		return site
	}
	return parts[0] + ":" + parts[1]
}

// dynamicDefinitionLine is the 1-based line of the first construct
// hasDynamicTestDefinition matched, or 0 when only the whole file shows it.
func dynamicDefinitionLine(text string) int {
	masked := strings.Split(maskQuotedLiteralsAndInlineComments(maskHeredocBodies(text)), "\n")
	for _, pattern := range []func(string) bool{
		dynamicDefinition.MatchString,
		func(line string) bool {
			return strings.Contains(line, "shared_examples") || strings.Contains(line, "shared_context")
		},
		dynamicLoop.MatchString,
	} {
		for index, line := range masked {
			if pattern(line) {
				return index + 1
			}
		}
	}
	return 0
}
