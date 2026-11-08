// SPDX-License-Identifier: Apache-2.0

package testsupport

import "strings"

// WithoutGitIdentity keeps the caller environment except for Git's author and
// committer overrides. Fixtures can append their own identity and pinned dates
// afterwards, or let their existing repository/command configuration supply them.
func WithoutGitIdentity(environment []string) []string {
	kept := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(name) {
		case "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_AUTHOR_DATE",
			"GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "GIT_COMMITTER_DATE":
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}
