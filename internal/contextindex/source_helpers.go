package contextindex

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
)

const (
	contextMaxDefiners = 50
	contextMaxBytes    = 1 << 20
)

var (
	contextToken = regexp.MustCompile(`[A-Za-z0-9]+`)
	contextCamel = regexp.MustCompile(`([a-z])([A-Z])`)
	contextFence = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
)

func appendRelation(relations []string, kind string) []string {
	if slices.Contains(relations, kind) {
		return relations
	}
	return append(relations, kind)
}

// contextIsTest extends impact's test rule with the Java and JVM convention
// of a `Test`/`Tests` suffix on the class name.
func contextIsTest(value string) bool {
	if isTestPath(value) {
		return true
	}
	base := strings.TrimSuffix(path.Base(value), path.Ext(value))
	return strings.HasSuffix(base, "Test") || strings.HasSuffix(base, "Tests")
}

// readCoChangeHistory reads the recent non-merge commits behind the revision
// with their paths: the bounded, local history the co-change slot counts over.
func readCoChangeHistory(ctx context.Context, index *Index) ([]historyEntry, error) {
	if history, ok := packHistory(index); ok {
		return history, nil
	}
	// `%D` limited to no refs prints only `grafted`, on a shallow boundary or a
	// grafts entry, whose path list is a diff against a parent Git does not have.
	raw, err := git(ctx, index.Root, maxHistoryBytes, nil, "log", "-z", fmt.Sprintf("-%d", maxHistoryCommits), "--no-merges", "--no-renames", "--decorate-refs=refs/nothing", "--format=%x1e%D%x1d%H%x1f%T%x1f%s", "--name-only", "HEAD")
	if err != nil {
		return nil, err
	}
	entries, _, err := parseHistory(dropGraftedCommits(raw), index.ObjectFormat, true)
	return entries, err
}

// dropGraftedCommits removes each record decorated `grafted` and strips the
// decoration field from the rest, leaving the format parseHistory reads.
func dropGraftedCommits(raw []byte) []byte {
	kept := make([]byte, 0, len(raw))
	for _, record := range bytes.Split(raw, []byte{0x1e}) {
		// A record without the field reaches parseHistory whole, which refuses it.
		decoration, rest, found := bytes.Cut(record, []byte{0x1d})
		if !found {
			decoration, rest = nil, record
		}
		if slices.Contains(strings.Split(string(bytes.TrimSpace(decoration)), ", "), "grafted") {
			continue
		}
		kept = append(append(kept, 0x1e), rest...)
	}
	return kept
}

// nextFence returns the fence open after line, "" when none is: a fence line
// opens one when none is open, and closes the open one only with the same
// character, at least as long, and no info string, so a fence of the other
// kind or a shorter one inside a block is literal text.
func nextFence(open, line string) string {
	match := contextFence.FindStringSubmatch(line)
	if match == nil {
		return open
	}
	if open == "" {
		return match[1]
	}
	closes := match[1][0] == open[0] && len(match[1]) >= len(open) && strings.TrimSpace(match[2]) == ""
	if !closes {
		return open
	}
	return ""
}

func sourceTextBounded(source Source) (string, bool) {
	if max(len(source.Data), int(source.body.length)) > contextMaxBytes {
		return "", false
	}
	text, valid, loaded := source.Text()
	return text, loaded && valid
}

func lexicalTerms(text string) []string {
	spaced := contextCamel.ReplaceAllString(text, "$1 $2")
	seen := map[string]struct{}{}
	result := make([]string, 0)
	for _, token := range contextToken.FindAllString(spaced, -1) {
		token = strings.ToLower(token)
		if len(token) < 2 {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	sort.Strings(result)
	return result
}

func stringSetOf(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

// taskStoreProjectionReason marks a Tasks store projection file, which is task
// state rather than evidence (DIRTY-CACHE-017).
const taskStoreProjectionReason = "task store projection"

// sealedChangeEvidenceReason marks a sealed CEM under `.corvint/changes/`,
// change evidence derived from its bind commit rather than source
// (DIRTY-CACHE-019).
const sealedChangeEvidenceReason = "sealed change evidence"

// trackedPath admits any path in the tree, indexed or not: an unread subject
// still has counterparts, siblings, and importers even without indexed text.
func trackedPath(index *Index, value string) bool {
	if _, ok := index.Sources[value]; ok {
		return true
	}
	_, ok := index.Tracked[value]
	return ok
}

// The `test` relation (TCP-V0-015): the test counterpart of a code file the
// packet admits, and the source an admitted test exercises, from four
// deterministic signals read at query time from the existing tables: the
// mirrored path/stem (`pairRelation`), an import edge from the test to the
// source (`Imports`), a whole-word mention in the test of a name the source
// declares (`Words`, rarity-weighted as `referenceRows` weights), and a test
// name (`Test<Name>`, `test_<name>`, `describe('<name>')`) whose camel-split
// tokens start with a declared name. One slot is reserved for the best row.

type testName struct {
	path, name string
	tokens     []string
}

// testLinkMaps are the symbol-derived tables of a testLinker: a function of
// Index.Symbols alone, built here on every packet unless the snapshot carries
// them as a testLinkTable (IDX-SNAP-V0-809).
type testLinkMaps struct {
	// declared maps a declared name (at least four bytes, at most
	// contextMaxDefiners definers) to its definers; declaredBy is the inverse;
	// declaredByToken indexes the names by their first camel-split token.
	declared, declaredBy, declaredByToken map[string][]string
	// declaredTokens is nameTokens of each retained declared name, split once.
	declaredTokens map[string][]string
	// testNames indexes the test-function names of the test paths by their
	// first camel-split token after the `Test`/`test_` prefix.
	testNames map[string][]testName
}

func buildTestLinkMaps(symbols []Symbol) testLinkMaps {
	maps := testLinkMaps{
		declared: map[string][]string{}, declaredBy: map[string][]string{},
		declaredByToken: map[string][]string{}, declaredTokens: map[string][]string{},
		testNames: map[string][]testName{},
	}
	for _, symbol := range symbols {
		if len(symbol.Name) >= 4 {
			maps.declared[symbol.Name] = appendRelation(maps.declared[symbol.Name], symbol.Path)
		}
		remainder, isTestName := testNameRemainder(symbol.Name)
		if isTestName && contextIsTest(symbol.Path) {
			maps.addTestName(symbol.Path, symbol.Name, remainder)
		}
	}
	for name, definers := range maps.declared {
		if len(definers) > contextMaxDefiners {
			delete(maps.declared, name)
			continue
		}
		for _, definer := range definers {
			maps.declaredBy[definer] = append(maps.declaredBy[definer], name)
		}
		tokens := nameTokens(name)
		maps.declaredTokens[name] = tokens
		if len(tokens) > 0 {
			maps.declaredByToken[tokens[0]] = append(maps.declaredByToken[tokens[0]], name)
		}
	}
	for _, names := range maps.declaredBy {
		sort.Strings(names)
	}
	return maps
}

func (maps *testLinkMaps) addTestName(path, name, remainder string) {
	tokens := nameTokens(remainder)
	if len(tokens) == 0 {
		return
	}
	maps.testNames[tokens[0]] = append(maps.testNames[tokens[0]], testName{path, name, tokens})
}

// The lookups below read the snapshot's testLinkTable when the linker has one
// and the maps otherwise; both answer every lookup alike (IDX-SNAP-V0-809).

// testNameRemainder strips the `Test`/`test_` prefix of a test-function name.
func testNameRemainder(name string) (string, bool) {
	switch {
	case strings.HasPrefix(name, "test_"):
		return name[5:], len(name) > 5
	case strings.HasPrefix(name, "Test") && len(name) > 4 && name[4] >= 'A' && name[4] <= 'Z':
		return name[4:], true
	}
	return "", false
}

// nameTokens is the lowercase camel-split token list of a name: each token a
// maximal run of ASCII letters and digits, split further where a lowercase
// letter meets an uppercase one; every other byte, ASCII or not, separates.
// A name with no token yields nil. This is the exact ASCII semantics of the
// regex form (contextCamel `$1 $2`, then contextToken) it replaced.
func nameTokens(name string) []string {
	var tokens []string
	start := -1
	for position := 0; position < len(name); position++ {
		if !isASCIIAlnum(name[position]) {
			tokens, start = appendNameToken(tokens, name, start, position), -1
			continue
		}
		if start >= 0 && isASCIILower(name[position-1]) && isASCIIUpper(name[position]) {
			tokens, start = appendNameToken(tokens, name, start, position), position
			continue
		}
		if start < 0 {
			start = position
		}
	}
	return appendNameToken(tokens, name, start, len(name))
}

func appendNameToken(tokens []string, name string, start, end int) []string {
	if start < 0 {
		return tokens
	}
	return append(tokens, strings.ToLower(name[start:end]))
}
