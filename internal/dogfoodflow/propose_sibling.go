package dogfoodflow

import (
	"fmt"
	"path"
	"strings"
)

// siblingProposal drafts a new file's row from a BASE sibling that follows
// the same pattern (DCW-V0-841): the file a new test or script tests in the
// same directory, then a file of the same base name in a sibling directory
// under the same parent. The row cites the sibling's first stable heading or
// declaration, else its first stable line, as a marked proposal. It returns
// "" for a file present at BASE, a data file, or when no sibling has a
// stable line no other row cites.
func (p *proposal) siblingProposal(hunk proposedHunk, ordinal string, edited map[string][][2]int, used map[string]bool) (string, string) {
	if dataPath(hunk.path) {
		return "", ""
	}
	if _, atBase := p.blob(p.base, hunk.path); atBase {
		return "", ""
	}
	for _, sibling := range p.siblings(hunk.path) {
		lines := p.baseLines(sibling.path)
		if len(lines) < 2 {
			continue
		}
		preferred := preferredAnchor(sibling.path)
		candidates := []int{}
		for index, text := range lines {
			if preferred(string(text)) {
				candidates = append(candidates, index+1)
			}
		}
		for index := range lines {
			candidates = append(candidates, index+1)
		}
		for _, line := range candidates {
			cite := fmt.Sprintf("%s:%d:%d", sibling.path, line, line)
			if used[cite] || !p.stable(sibling.path, line, edited) {
				continue
			}
			p.proposals[ordinal] = true
			p.reasons[ordinal] = hunk.path + " is new at BASE and follows the pattern of its BASE sibling " + sibling.path + " (" + sibling.pattern +
				"); its first stable heading or declaration is drafted, so confirm the new file implements that contract, or cite the one it does (DCW-V0-841)"
			return fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, sibling.path, line, line, proposalRelation(sibling.path)), cite
		}
	}
	return "", ""
}

type siblingFile struct{ path, pattern string }

// maxSiblingDirs bounds the sibling directories one parent lends.
const maxSiblingDirs = 64

// siblings lists the BASE files a new file name follows, in proposal order.
func (p *proposal) siblings(name string) []siblingFile {
	dir, base := path.Split(name)
	found := []siblingFile{}
	seen := map[string]bool{}
	add := func(candidate, pattern string) {
		if candidate == name || seen[candidate] {
			return
		}
		seen[candidate] = true
		// A draft never cites a path the change modifies (DCW-V0-393).
		if _, ok := p.blob(p.base, candidate); ok && !p.changed[candidate] {
			found = append(found, siblingFile{candidate, pattern})
		}
	}
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if strings.HasSuffix(stem, "_test") && ext != "" {
		add(dir+strings.TrimSuffix(stem, "_test")+ext, "the file this test tests")
	}
	if strings.HasPrefix(stem, "test_") && ext != "" {
		add(dir+strings.TrimPrefix(stem, "test_")+ext, "the file this test tests")
	}
	parent := path.Dir(strings.TrimSuffix(dir, "/"))
	if dir == "" {
		return found
	}
	if p.siblingDirs == nil {
		p.siblingDirs = map[string][]string{}
	}
	dirs, cached := p.siblingDirs[parent]
	if !cached {
		prefix := parent + "/"
		if parent == "." {
			prefix = ""
		}
		args := []string{"ls-tree", "-d", "--name-only", p.base}
		if prefix != "" {
			args = append(args, "--", prefix)
		}
		listed, status := p.git(true, args...)
		if status == 0 {
			for _, entry := range strings.Split(chomp(listed), "\n") {
				if entry != "" {
					dirs = append(dirs, entry)
				}
			}
		}
		// A parent with very many directories lends only the first of them.
		if len(dirs) > maxSiblingDirs {
			dirs = dirs[:maxSiblingDirs]
		}
		p.siblingDirs[parent] = dirs
	}
	for _, other := range dirs {
		add(other+"/"+base, "the same file name in the sibling directory "+other+"/")
	}
	return found
}
