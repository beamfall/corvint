// SPDX-License-Identifier: Apache-2.0

package contextindex

// taskScanText is the task text the lexical identifier channel tokenizes
// (GPK-V0-1950). Reuse JSON value extraction so decoding requires a complete,
// valid serialization boundary, excludes envelope keys and handles unicode.
// Ordinary paths, regex and quoted code remain verbatim; a count of backslash
// escapes is not an encoding signal. Extract once, without decoding the values
// again: a JSON value can itself contain literal backslashes or quoted code.
func taskScanText(task string) string {
	return contextQueryValues(task)
}
