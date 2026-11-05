// SPDX-License-Identifier: Apache-2.0
package golang

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// readKindAliases shares one cache-stamped traversal among declared scanners.
// Scanners open .go aliases by name, regardless of the target's extension or
// module. If any target or listing cannot be bounded, keep their lexical reads.
type readKindAliases struct {
	once     sync.Once
	targets  []string
	complete bool
}

func (a *readKindAliases) read(root *affected.Source) ([]string, bool) {
	a.once.Do(func() {
		a.complete = true
		pending := []string{"."}
		var links []string
		quote := strings.NewReplacer("\\", "\\\\", "*", "\\*", "?", "\\?", "[", "\\[", "]", "\\]")
		entries := 0
		for len(pending) > 0 && a.complete {
			directory := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			children, err := root.ReadDir(directory)
			if err != nil {
				a.complete = false
				break
			}
			for _, child := range children {
				entries++
				if entries > affected.MaxWalkEntries {
					a.complete = false
					break
				}
				name := child.Name()
				relative := path.Join(directory, name)
				if child.IsDir() {
					if name != ".git" {
						pending = append(pending, relative)
					}
					continue
				}
				if child.Type()&fs.ModeSymlink == 0 {
					continue
				}
				if len(a.targets)+len(links) >= affected.MaxPathsPerUnit {
					a.complete = false
					break
				}
				// A non-Go alias may be an intermediate hop: retargeting it
				// can change a scanner's input without editing the final file.
				links = append(links, "/"+quote.Replace(relative))
				if !strings.HasSuffix(name, ".go") && name != "go.mod" {
					continue
				}
				target, ok := root.LinkTarget(relative)
				if !ok {
					a.complete = false
					break
				}
				// Existing reader globs are root-relative. Quote metacharacters
				// so a target's filename remains a literal dependency.
				a.targets = append(a.targets, "/"+quote.Replace(target))
			}
		}
		if len(a.targets) > 0 {
			a.targets = append(a.targets, links...)
		}
	})
	return a.targets, a.complete
}

// testReadKinds reads the project-owned, file-local bound of AFP-V0-9000.
// It is deliberately a closed declaration: no inferred read domain and no
// arbitrary pattern that could accidentally exclude a repository reader.
// Parse failures and misplaced/duplicate declarations retain rule (d).
func testReadKinds(relative string, body []byte) []string {
	const prefix = "//corvint:read-kinds"
	if !strings.HasSuffix(relative, "_test.go") || !strings.Contains(string(body), prefix) {
		return nil
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, relative, body, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	count := 0
	var kinds []string
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, prefix) {
				count++
				if comment.Pos() < file.Package && startsLine(body, fileSet.Position(comment.Pos()).Offset) {
					switch comment.Text {
					case prefix + " .go go.mod":
						kinds = []string{"*.go", "go.mod"}
					case prefix + " fixtures":
						kinds = []string{}
					}
				}
			}
		}
	}
	if count != 1 {
		return nil
	}
	return kinds
}
