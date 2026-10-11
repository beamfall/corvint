package revieworder

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/gitnotes"
)

type attribution struct {
	path       string
	start, end int64
	value      Value
}

// readNote keeps foreign ranges as intervals: a huge endpoint cannot cause a
// per-line allocation. A malformed record makes the whole note unknown rather
// than silently dropping a potentially conflicting attestation.
func readNote(note *Note, target string) ([]attribution, string) {
	if note == nil {
		return nil, "git-ai-note-absent-or-unavailable"
	}
	if !commitID(target) || note.Commit != target || !commitID(note.BlobOID) {
		return nil, "git-ai-note-binding-mismatch"
	}
	if len(note.Body) > gitnotes.MaxNoteBytes {
		return nil, "git-ai-note-over-limit"
	}
	log, raw, ok := strings.Cut("\n"+string(note.Body), "\n---\n")
	var metadata struct {
		Schema     string                     `json:"schema_version"`
		BaseCommit string                     `json:"base_commit_sha"`
		Prompts    map[string]json.RawMessage `json:"prompts"`
		Sessions   map[string]json.RawMessage `json:"sessions"`
		Humans     map[string]json.RawMessage `json:"humans"`
	}
	if !ok || json.Unmarshal([]byte(raw), &metadata) != nil || metadata.Schema != "authorship/3.0.0" ||
		!commitID(metadata.BaseCommit) || metadata.Prompts == nil {
		return nil, "git-ai-note-malformed-or-unsupported"
	}
	records := []attribution{}
	path := ""
	// Every file needs an attestation; otherwise a split, unquoted newline
	// could silently replace its path before the following human record.
	pathNeedsAttestation := false
	for _, line := range strings.Split(log, "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			if pathNeedsAttestation {
				return nil, "git-ai-note-malformed-path"
			}
			path = line
			if strings.HasPrefix(path, "\"") {
				var err error
				path, err = strconv.Unquote(path)
				if err != nil {
					return nil, "git-ai-note-malformed-path"
				}
			} else if strings.TrimLeftFunc(path, unicode.IsSpace) != path || strings.ContainsAny(path, " \t\r\"") {
				return nil, "git-ai-note-malformed-path"
			}
			if path == "" {
				return nil, "git-ai-note-malformed-path"
			}
			pathNeedsAttestation = true
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			return nil, "git-ai-note-malformed-attestation"
		}
		id, ranges, separated := strings.Cut(line[2:], " ")
		if path == "" || !separated || id == "" || ranges == "" || strings.ContainsAny(id+ranges, " \t\r") {
			return nil, "git-ai-note-malformed-attestation"
		}
		value := Unknown
		if strings.HasPrefix(id, "h_") && len(id) == 16 && hexID(id[2:]) && humanRecord(metadata.Humans[id]) {
			value = No
		}
		if (len(id) == 16 || len(id) == 7) && hexID(id) && aiRecord(metadata.Prompts[id]) {
			value = Yes
		}
		session, task, found := strings.Cut(id, "::t_")
		if strings.HasPrefix(session, "s_") && len(session) == 16 && hexID(session[2:]) &&
			found && len(task) == 14 && hexID(task) && aiRecord(metadata.Sessions[session]) {
			value = Yes
		}
		for _, span := range strings.Split(ranges, ",") {
			first, last, ranged := strings.Cut(span, "-")
			if strings.Trim(first, "0123456789") != "" || (ranged && strings.Trim(last, "0123456789") != "") {
				return nil, "git-ai-note-malformed-range"
			}
			start, err := strconv.ParseInt(first, 10, 64)
			if err != nil || start < 1 {
				return nil, "git-ai-note-malformed-range"
			}
			end := start
			if ranged {
				end, err = strconv.ParseInt(last, 10, 64)
			}
			if err != nil || end < start {
				return nil, "git-ai-note-malformed-range"
			}
			records = append(records, attribution{path, start, end, value})
		}
		pathNeedsAttestation = false
	}
	if pathNeedsAttestation {
		return nil, "git-ai-note-malformed-path"
	}
	return records, ""
}

func aiRecord(raw json.RawMessage) bool {
	var record struct {
		AgentID struct {
			Tool string `json:"tool"`
		} `json:"agent_id"`
	}
	return json.Unmarshal(raw, &record) == nil && record.AgentID.Tool != ""
}

func humanRecord(raw json.RawMessage) bool {
	var record struct {
		Author string `json:"author"`
	}
	return json.Unmarshal(raw, &record) == nil && record.Author != ""
}

func lineAuthor(records []attribution, path string, line int64) Value {
	value := Unknown
	found := false
	for _, r := range records {
		if r.path != path || line < r.start || line > r.end {
			continue
		}
		if r.value == Unknown || (found && value != r.value) {
			return Unknown
		}
		value, found = r.value, true
	}
	return value
}
