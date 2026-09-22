package roadmap

import (
	"bufio"
	"bytes"
	"os"
	"strings"
)

// Requirement is one docs/specs/REQUIREMENTS.tsv row.
type Requirement struct {
	ID    string
	File  string
	Line  string
	Title string
}

// LoadRequirements parses docs/specs/REQUIREMENTS.tsv (an "id\tfile\tline\t
// title" header followed by one requirement per row) into an id-keyed
// lookup. It is strictly read-only: this package never writes to that file.
// The table is opened without blocking and read only as a regular file of
// at most maxInputFileBytes; a FIFO, device, or oversize table is an error.
// A row with fewer than four tab-separated fields is skipped rather than
// causing the whole load to fail, since a malformed row must not turn every
// requirement reference on the roadmap into a load error.
func LoadRequirements(path string) (map[string]Requirement, error) {
	file, err := os.OpenFile(path, inputOpenFlags, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := readRegularBounded(file)
	if err != nil {
		return nil, err
	}

	requirements := make(map[string]Requirement)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 4<<20)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 4 || fields[0] == "" {
			continue
		}
		requirements[fields[0]] = Requirement{ID: fields[0], File: fields[1], Line: fields[2], Title: fields[3]}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return requirements, nil
}

// ResolveRequirement looks up id in requirements, reporting an EvidenceLink
// that states "unresolved" (Resolved: false, no file/line/title) rather than
// inventing a location when the id has no row in this snapshot of the file.
func ResolveRequirement(requirements map[string]Requirement, id string) EvidenceLink {
	if requirement, ok := requirements[id]; ok {
		return EvidenceLink{RequirementID: id, File: requirement.File, Line: requirement.Line, Title: requirement.Title, Resolved: true}
	}
	return EvidenceLink{RequirementID: id, Resolved: false}
}
