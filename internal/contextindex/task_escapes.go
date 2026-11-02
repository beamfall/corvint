// SPDX-License-Identifier: Apache-2.0

package contextindex

import "strings"

// taskScanText is the task text the lexical identifier channel tokenizes
// (GPK-V0-1950). A task pasted as one escaped string literal, such as a
// JSON-encoded issue body, carries its line breaks as the two characters `\`
// `n`. Tokenized as written, the `n` joins the next word: "\n\nThe" becomes
// the identifier `nThe` and "\nDirectedGraph" becomes `nDirectedGraph`, so the
// channel searches for, and its receipt discloses, names the task never wrote,
// and loses the name it did write.
//
// The text is decoded only when it has that signature: no raw line break at
// all and at least two `\n` escapes. A task that writes real line breaks keeps
// every backslash, so an escape it quotes from code ("printf(\"a\nb\")") is
// read as written. Decoding is one left-to-right pass over `\\`, `\"`, `\/`,
// `\n`, `\r` and `\t`; every other backslash sequence is kept verbatim. The
// receipt still echoes the task as given, and the confident tier and the
// task-context ranking read the task as written: only the channel's terms and
// identifiers read this text.
func taskScanText(task string) string {
	if strings.ContainsAny(task, "\n\r") || strings.Count(task, `\n`) < 2 {
		return task
	}
	var decoded strings.Builder
	decoded.Grow(len(task))
	for index := 0; index < len(task); index++ {
		if task[index] != '\\' || index+1 == len(task) {
			decoded.WriteByte(task[index])
			continue
		}
		switch next := task[index+1]; next {
		case 'n':
			decoded.WriteByte('\n')
		case 'r':
			decoded.WriteByte('\r')
		case 't':
			decoded.WriteByte('\t')
		case '\\', '"', '/':
			decoded.WriteByte(next)
		default:
			decoded.WriteByte('\\')
			continue
		}
		index++
	}
	return decoded.String()
}
