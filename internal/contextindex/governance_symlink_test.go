// SPDX-License-Identifier: Apache-2.0

package contextindex

var symlinkGovernanceFiles = map[string]string{
	"go.mod":        "module example.test/links\n\ngo 1.27.0\n",
	"CLAUDE.md":     "# Instructions\n\n## Verify\n\nRun the focused tests.\n",
	"docs/guide.md": "# Guide\n",
	"pkg/link.go":   "package link\n\nfunc Link() {}\n",
}
