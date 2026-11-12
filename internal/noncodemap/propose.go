// SPDX-License-Identifier: Apache-2.0

package noncodemap

import (
	"bufio"
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"io"
	"slices"
	"strings"
)

// MaxRecordedBytes bounds a recorded-reads file.
const MaxRecordedBytes = 8 << 20

// recordedRead is one line of a recorded-reads file: a test and a repository
// path it read.
type recordedRead struct {
	Test string `json:"test"`
	Path string `json:"path"`
}

// Propose reads recorded test reads, one JSON object `{"test": ..., "path":
// ...}` per line, and returns a `corvint-data-map/0` document with one
// proposed entry per read path (NCM-V1-003): the reading tests sorted, the
// evidence naming source, and no acceptance member or proof, so Evaluate
// reads every entry as proposed and none narrows. A path under `.corvint/`
// or on the never-narrow list is left out, since no entry may narrow it.
// Propose writes nothing; the caller prints the document.
func Propose(recorded io.Reader, source string) ([]byte, error) {
	if !printable(source) || len(source) > maxField/2 {
		source = "the recorded-reads file"
	}
	body, err := io.ReadAll(io.LimitReader(recorded, MaxRecordedBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > MaxRecordedBytes {
		return nil, fmt.Errorf("recorded reads exceed %d bytes", MaxRecordedBytes)
	}
	readers := map[string]map[string]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64<<10), MaxRecordedBytes)
	number := 0
	for scanner.Scan() {
		number++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var read recordedRead
		if err := json.Unmarshal(line, &read, json.RejectUnknownMembers(true)); err != nil {
			return nil, fmt.Errorf("recorded reads line %d: %v", number, err)
		}
		if !CleanPath(read.Path) || !printable(read.Test) || strings.ContainsRune(read.Test, ' ') {
			return nil, fmt.Errorf("recorded reads line %d: needs a printable test without spaces and a clean repository-relative path", number)
		}
		if strings.HasPrefix(read.Path, corvintDirectory) {
			continue
		}
		if _, never := NeverNarrowed(read.Path); never {
			continue
		}
		if readers[read.Path] == nil {
			readers[read.Path] = map[string]bool{}
		}
		readers[read.Path][read.Test] = true
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	proposal := Map{Profile: Profile, Entries: []Entry{}}
	for _, p := range sortedKeys(readers) {
		tests := sortedKeys(readers[p])
		if len(tests) > maxTests {
			tests = tests[:maxTests]
		}
		proposal.Entries = append(proposal.Entries, Entry{
			Path:     p,
			Tests:    tests,
			Evidence: fmt.Sprintf("proposed from recorded reads in %s: %d test(s) read this path", source, len(readers[p])),
		})
	}
	out, err := json.Marshal(proposal, jsontext.WithIndent("  "), json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
