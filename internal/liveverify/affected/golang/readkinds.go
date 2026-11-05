// SPDX-License-Identifier: Apache-2.0
package golang

import (
	"go/parser"
	"go/token"
	"strings"
)

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
