package pathbrief

import (
	"encoding/json"
	"errors"
	"slices"
)

// LinksPath is the repository path of the requirement-link file (TBC-V1-012).
const LinksPath = ".corvint/requirement-links.json"

// LinksSchema names the layout of the requirement-link file.
const LinksSchema = "corvint-requirement-links/1"

// LinkFile is the requirement-link file. One entry of TBC-V1-012 is one evidence index of one
// record of one path: the path, the evidence span it points at and the record commit. Commits and
// evidence spans are tables so that the 14,000 entries of this repository do not repeat them.
type LinkFile struct {
	Schema      string         `json:"schema"`
	InputCommit string         `json:"inputCommit"`
	Commits     []LinkCommit   `json:"commits"`
	Evidence    []LinkEvidence `json:"evidence"`
	Paths       []LinkPath     `json:"paths"`

	byPath map[string]int
}

// LinkCommit is one record commit: the SHA in a sealed record's file name and its committer time
// in Unix seconds, kept so that a reader needs no history to pick the newest entry.
type LinkCommit struct {
	SHA           string `json:"sha"`
	CommitterTime int64  `json:"committerTime"`
}

// LinkEvidence is one pinned span of a specification or decision file, with every requirement ID
// whose definition begins inside the span at that blob. Requirements may be empty.
type LinkEvidence struct {
	File         string   `json:"file"`
	Start        int      `json:"start"`
	End          int      `json:"end"`
	BlobOid      string   `json:"blobOid"`
	Requirements []string `json:"requirements"`
}

// LinkPath holds the records that tie one path to evidence spans.
type LinkPath struct {
	Path    string       `json:"path"`
	Records []LinkRecord `json:"records"`
}

// LinkRecord is the links one sealed record holds for a path. Commit and Evidence index the
// tables. Blob is the path's blob at the record commit, or "" when the path is absent there; it
// lets a reader state whether the file changed since without reading history.
type LinkRecord struct {
	Commit   int    `json:"commit"`
	Blob     string `json:"blob"`
	Evidence []int  `json:"evidence"`
}

// Link is one entry of the file, joined to its tables.
type Link struct {
	Path          string
	Evidence      LinkEvidence
	Commit        string
	CommitterTime int64
	PathBlob      string
}

// ParseLinks reads a requirement-link file and refuses one whose schema, table rows or table
// indexes are not valid, so a damaged file is a missing source and never a partial answer.
func ParseLinks(data []byte) (*LinkFile, error) {
	file := &LinkFile{}
	if err := json.Unmarshal(data, file); err != nil {
		return nil, errors.New("requirement-link file is not JSON")
	}
	if file.Schema != LinksSchema {
		return nil, errors.New("requirement-link file has an unknown schema")
	}
	for _, commit := range file.Commits {
		if !validOID(commit.SHA) || commit.CommitterTime < 0 {
			return nil, errors.New("requirement-link file has a malformed commit row")
		}
	}
	for _, evidence := range file.Evidence {
		if evidence.File == "" || evidence.Start < 0 || evidence.End < evidence.Start ||
			!validOID(evidence.BlobOid) || slices.Contains(evidence.Requirements, "") {
			return nil, errors.New("requirement-link file has a malformed evidence row")
		}
	}
	file.byPath = make(map[string]int, len(file.Paths))
	for index, entry := range file.Paths {
		if _, seen := file.byPath[entry.Path]; seen || entry.Path == "" {
			return nil, errors.New("requirement-link file repeats or omits a path")
		}
		file.byPath[entry.Path] = index
		for _, record := range entry.Records {
			if record.Commit < 0 || record.Commit >= len(file.Commits) {
				return nil, errors.New("requirement-link file names a commit outside its table")
			}
			if record.Blob != "" && !validOID(record.Blob) {
				return nil, errors.New("requirement-link file has a malformed path blob")
			}
			for _, evidence := range record.Evidence {
				if evidence < 0 || evidence >= len(file.Evidence) {
					return nil, errors.New("requirement-link file names evidence outside its table")
				}
			}
		}
	}
	return file, nil
}

// Entries is the number of entries the file holds.
func (f *LinkFile) Entries() int {
	total := 0
	for _, entry := range f.Paths {
		for _, record := range entry.Records {
			total += len(record.Evidence)
		}
	}
	return total
}

// ForPath returns the entries of one path, in file order. A path is matched exactly: a renamed
// file has no entries under its new name (TBC-V1-012).
func (f *LinkFile) ForPath(path string) []Link {
	index, ok := f.byPath[path]
	if !ok {
		return nil
	}
	var links []Link
	for _, record := range f.Paths[index].Records {
		commit := f.Commits[record.Commit]
		for _, evidence := range record.Evidence {
			links = append(links, Link{Path: path, Evidence: f.Evidence[evidence], Commit: commit.SHA,
				CommitterTime: commit.CommitterTime, PathBlob: record.Blob})
		}
	}
	return links
}
