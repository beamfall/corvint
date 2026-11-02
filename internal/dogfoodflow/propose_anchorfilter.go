package dogfoodflow

import (
	"path"
	"sort"
	"strings"
)

// testSourcePath reports a test source file by its name: a _test or test_
// stem, or a .test or .spec infix, such as p_test.go, test_p.py or p.test.mjs.
func testSourcePath(name string) bool {
	base := path.Base(name)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return ext != "" && stem != "" && (strings.HasSuffix(stem, "_test") || strings.HasPrefix(stem, "test_") ||
		strings.HasSuffix(stem, ".test") || strings.HasSuffix(stem, ".spec"))
}

// fixtureTextPath reports a file whose text names requirement IDs as fixture
// data rather than as a contract: a test source file or a data file
// (DCW-V0-847).
func fixtureTextPath(name string) bool { return testSourcePath(name) || dataPath(name) }

// ownRequirementIDs splits the requirement IDs a hunk adds into those it
// implements and those it names only as fixture text (DCW-V0-847). A hunk of
// a test or data file implements an ID only when the change's other edits,
// outside test and data files, name it too, or the BASE file the test tests
// does; any other ID it names is fixture text and anchors nothing.
func (p *proposal) ownRequirementIDs(name string, ids []string) (own, fixture []string) {
	if len(ids) == 0 || !fixtureTextPath(name) {
		return ids, nil
	}
	named := map[string]bool{}
	note := func(text []byte) {
		for _, id := range requirementToken.FindAll(text, -1) {
			named[string(id)] = true
		}
	}
	for key, other := range p.derived {
		if key != other.ID || fixtureTextPath(other.DisplayPath) {
			continue
		}
		edits, _ := hunkText(other)
		note(edits)
	}
	for _, tested := range testedFiles(name) {
		if blob, ok := p.blob(p.base, tested); ok {
			note(blob)
		}
	}
	for _, id := range ids {
		if named[id] {
			own = append(own, id)
		} else {
			fixture = append(fixture, id)
		}
	}
	return own, fixture
}

// testedFiles lists the files a test source file tests by its name.
func testedFiles(name string) []string {
	dir, base := path.Split(name)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	found := []string{}
	for _, cut := range []func(string) (string, bool){
		func(s string) (string, bool) { return strings.CutSuffix(s, "_test") },
		func(s string) (string, bool) { return strings.CutPrefix(s, "test_") },
		func(s string) (string, bool) { return strings.CutSuffix(s, ".test") },
		func(s string) (string, bool) { return strings.CutSuffix(s, ".spec") },
	} {
		if tested, ok := cut(stem); ok && tested != "" {
			found = append(found, dir+tested+ext)
		}
	}
	return found
}

// foreignRequirementLine names, for a BASE line naming a hunk's path, the
// requirement IDs that make it another requirement's line rather than this
// hunk's anchor, or "" when it may anchor (DCW-V0-847): a line carrying
// requirement IDs must carry one the hunk names when it names any, so a
// neighbouring traceability row is never drafted for it.
func foreignRequirementLine(own []string, text []byte) string {
	if len(own) == 0 {
		return ""
	}
	tokens := map[string]bool{}
	for _, id := range requirementToken.FindAll(text, -1) {
		tokens[string(id)] = true
	}
	if len(tokens) == 0 {
		return ""
	}
	for _, id := range own {
		if tokens[id] {
			return ""
		}
	}
	names := make([]string, 0, len(tokens))
	for id := range tokens {
		names = append(names, id)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
