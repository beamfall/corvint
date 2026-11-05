// SPDX-License-Identifier: Apache-2.0

package shell

// repositoryRootCommand recognizes only the literal top-level query, without
// a different Git directory or working directory. Treating its value as a
// root-relative path adds edges for root wrappers such as lane-host-heavy.
// AZU-V0-004, AZU-V0-8901.
func repositoryRootCommand(text string) bool {
	words := plainWords(fieldsTop(redirection.ReplaceAllString(text, "")))
	if len(words) >= 3 && words[0] == "env" && words[1] == "-u" {
		words = words[3:]
	}
	return len(words) == 3 && words[0] == "git" && words[1] == "rev-parse" && words[2] == "--show-toplevel"
}
