package roadmap

import (
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/specindex"
)

// Requirement is one docs/specs/REQUIREMENTS.tsv row.
type Requirement struct {
	ID    string
	File  string
	Line  string
	Title string
}

// LoadRequirements parses docs/specs/REQUIREMENTS.tsv, in either the
// committed layout (no line column, SRG-V0-015) or a line-bearing one, into an
// id-keyed lookup. It is strictly read-only: this package never writes to that
// file. The table is opened without blocking and read only as a regular file of
// at most maxInputFileBytes; a FIFO, device, or oversize table is an error, and
// so is a first line that is not an index header. A malformed row is skipped
// rather than causing the whole load to fail, since a malformed row must not
// turn every requirement reference on the roadmap into a load error.
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
	rows, err := specindex.ParseIndex(string(data))
	if err != nil {
		return nil, err
	}
	requirements := make(map[string]Requirement, len(rows))
	for id, row := range rows {
		requirement := Requirement{ID: id, File: row.File, Title: row.Title}
		if row.Line > 0 {
			requirement.Line = strconv.Itoa(row.Line)
		}
		requirements[id] = requirement
	}
	return requirements, nil
}

// specLines locates the definition line of a resolved link whose index row
// carried none (SRG-V0-017), reading each spec under root at most once. A spec
// that cannot be read, or that no longer defines the id, leaves the line empty:
// the link still names its file and title, never an invented line.
type specLines struct {
	root  string
	texts map[string]string
}

func newSpecLines(root string) *specLines {
	return &specLines{root: root, texts: map[string]string{}}
}

func (s *specLines) fill(link EvidenceLink) EvidenceLink {
	if !link.Resolved || link.Line != "" || s.root == "" {
		return link
	}
	clean := path.Clean(link.File)
	if clean != link.File || path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return link
	}
	text, ok := s.texts[clean]
	if !ok {
		if file, err := os.OpenFile(filepath.Join(s.root, filepath.FromSlash(clean)), inputOpenFlags, 0); err == nil {
			data, err := readRegularBounded(file)
			file.Close()
			if err == nil {
				text = string(data)
			}
		}
		s.texts[clean] = text
	}
	if number := specindex.DefinitionLine(text, link.RequirementID); number > 0 {
		link.Line = strconv.Itoa(number)
	}
	return link
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
