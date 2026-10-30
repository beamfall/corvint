package contextindex

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const instructionRuleURL = "https://github.com/openai/codex/blob/rust-v0.153.2/codex-rs/core/src/agents_md.rs"
const instructionRuleSHA256 = "8bbaf068c099fdeeaf4fe49076d398da7671f20c9a962fde5c4eb7653008fed4"
const instructionDefaultBudget = 32768
const instructionDirectoryBound = 64

// InstructionLoadOptions selects a conditional project-only prediction. These
// values do not attest the running host's trust, settings or environment state.
type InstructionLoadOptions struct {
	Host, Version, CWD, Profile string
}

type instructionWarning struct {
	Detail     string `json:"detail,omitempty"`
	Code       string `json:"code"`
	Line       int    `json:"line,omitempty"`
	ByteOffset int    `json:"byte_offset"`
	CodePoint  string `json:"code_point,omitempty"`
}

type instructionLoadRow struct {
	Path             string               `json:"path"`
	BlobHash         string               `json:"blob_hash,omitempty"`
	State            string               `json:"state"`
	Reason           string               `json:"reason"`
	SourceBytes      int                  `json:"source_bytes,omitempty"`
	RetainedRawBytes int                  `json:"retained_raw_bytes,omitempty"`
	DecodedBytes     int                  `json:"decoded_bytes,omitempty"`
	Warnings         []instructionWarning `json:"warnings"`
}

// InstructionLoadSet uses only pinned index bytes. Host loading syntax never
// changes the context packet's project-owned authority or ranking.
func InstructionLoadSet(index *Index, subject string, options InstructionLoadOptions) (map[string]any, error) {
	if options.Host == "" || options.Version == "" {
		return nil, &Error{Message: "instruction prediction requires explicit host and version"}
	}
	if options.Profile != "default" {
		return nil, &Error{Message: "instruction profile must be default (an explicit conditional assumption)"}
	}
	cwd := options.CWD
	if cwd != "." {
		clean, err := cleanImpactPath(cwd)
		if err != nil || clean != cwd {
			return nil, &Error{Message: "instruction cwd must be a normalized repository-relative directory"}
		}
	}
	cleanSubject, err := cleanImpactPath(subject)
	if err != nil || subject == "" || !trackedPath(index, cleanSubject) {
		return nil, &Error{Message: "instruction prediction requires a tracked subject"}
	}
	if cwd != "." && !strings.HasPrefix(cleanSubject, cwd+"/") {
		return nil, &Error{Message: "instruction cwd must contain the subject"}
	}
	directories := []string{cwd}
	for current := cwd; current != "."; {
		current = path.Dir(current)
		directories = append(directories, current)
		if len(directories) > instructionDirectoryBound {
			return nil, &Error{Message: "instruction directory depth exceeds 64"}
		}
	}
	slices.Reverse(directories)
	rows := make([]instructionLoadRow, 0)
	receipt := map[string]any{
		"profile": "corvint-instruction-load-set/0", "host": options.Host, "host_version": options.Version,
		"assumed_profile": options.Profile, "cwd": cwd, "subject": cleanSubject,
		"revision": index.Revision, "commit_revision": index.CommitRevision,
		"qualification": "CONDITIONAL_SOURCE_RULE_PREDICTION", "actual_session_load_set": "UNKNOWN",
		"project_authority": "UNCHANGED", "state": "PREDICTED",
		"assumptions":  []string{"trusted-project", "one-turn-environment", "default-root-markers-.git", "git-root-is-host-project-root", "no-fallback-filenames", "project-budget-32768", "readable-regular-files", "pinned-tree-matches-host-instruction-inventory"},
		"unknowns":     []string{"actual-trust", "actual-config", "actual-root-marker-resolution", "actual-environment-count-and-order", "actual-file-access", "global-and-user-instructions", "other-injected-instructions", "session-state", "untracked-host-instructions", "diff-under-review-not-supplied"},
		"rules":        map[string]any{"source": instructionRuleURL, "sha256": instructionRuleSHA256, "discovery_lines": "184-280", "loading_lines": "60-174"},
		"warnings_are": "inspection signals, not proof of malicious intent; authority rows remain unchanged",
	}
	if options.Host != "codex" || options.Version != "0.153.2" {
		receipt["state"], receipt["qualification"] = "UNKNOWN", "UNCONFIRMED_HOST_VERSION_RULES"
		receipt["rules"] = nil
		receipt["rows"] = rows
		return receipt, nil
	}
	remaining, decodedTotal := instructionDefaultBudget, 0
	uncertain := false
	for _, directory := range directories {
		selected, selectionUnknown := false, false
		for _, name := range []string{"AGENTS.override.md", "AGENTS.md", "CLAUDE.md"} {
			candidate := path.Join(directory, name)
			if !trackedPath(index, candidate) {
				continue
			}
			source, present := index.Sources[candidate]
			row := instructionLoadRow{Path: candidate, BlobHash: source.BlobHash, Warnings: []instructionWarning{}}
			if slices.Contains(index.DirtyPaths, candidate) {
				row.Warnings = append(row.Warnings, instructionWarning{Code: "SOURCE_MODIFIED_WORKTREE", Detail: "prediction uses committed blob"})
			}
			data, available := instructionSourceBytes(source, present)
			if available {
				row.SourceBytes = len(data)
				row.Warnings = append(row.Warnings, instructionControls(data)...)
			}
			switch {
			case name == "CLAUDE.md":
				row.State, row.Reason = "IGNORED", "not a default Codex filename; host syntax does not negate project authority"
			case selected:
				row.State, row.Reason = "SHADOWED", "earlier filename selected in this directory; host syntax does not negate project authority"
				if selectionUnknown {
					row.State, row.Reason = "UNKNOWN", "earlier regular-file selection is unresolved"
				}
			default:
				selected = true
				switch {
				case !available || (source.Mode != "100644" && source.Mode != "100755"):
					row.State, row.Reason = "UNKNOWN", "selected source bytes or regular-file identity unavailable; no fallback or downstream budget claim"
					uncertain = true
					selectionUnknown = source.Mode != "100644" && source.Mode != "100755"
				case uncertain:
					row.State, row.Reason = "UNKNOWN", "earlier selection or budget is unresolved"
				case remaining == 0:
					row.State, row.Reason = "BUDGET_EXHAUSTED", "earlier entries exhausted the project byte budget"
				default:
					row.RetainedRawBytes = min(len(data), remaining)
					decoded := instructionLossyUTF8(data[:row.RetainedRawBytes])
					row.DecodedBytes = len(decoded)
					row.State, row.Reason = "LOADED", "conditional default filename selection, root to cwd"
					if strings.TrimSpace(decoded) == "" {
						row.State, row.Reason = "EMPTY_SELECTED", "selected whitespace-only source shadows alternatives and consumes no inner budget"
					} else {
						remaining -= row.RetainedRawBytes
						decodedTotal += row.DecodedBytes
						if row.RetainedRawBytes < len(data) {
							row.State = "TRUNCATED"
						}
					}
				}
			}
			rows = append(rows, row)
		}
	}
	if uncertain {
		receipt["state"] = "UNKNOWN"
	}
	receipt["rows"] = rows
	if !uncertain {
		receipt["remaining_inner_raw_budget"] = remaining
		receipt["remaining_outer_decoded_budget"] = max(0, instructionDefaultBudget-decodedTotal)
	}
	return receipt, nil
}

func instructionSourceBytes(source Source, present bool) ([]byte, bool) {
	if !present || max(len(source.Data), int(source.body.length)) > contextMaxBytes {
		return nil, false
	}
	if source.Data != nil {
		return source.Data, true
	}
	text, valid, loaded := source.Text()
	if !valid || !loaded {
		return nil, false
	}
	return []byte(text), true
}

// Rust from_utf8_lossy replaces a valid prefix of an incomplete/invalid
// sequence once; Go's ToValidUTF8 groups adjacent invalid bytes differently.
func instructionLossyUTF8(raw []byte) string {
	var out strings.Builder
	for len(raw) > 0 {
		r, size := utf8.DecodeRune(raw)
		if r != utf8.RuneError || size > 1 {
			out.Write(raw[:size])
			raw = raw[size:]
			continue
		}
		length := 1
		if raw[0] >= 0xc2 && raw[0] <= 0xdf {
			length = 2
		}
		if raw[0] >= 0xe0 && raw[0] <= 0xef {
			length = 3
		}
		if raw[0] >= 0xf0 && raw[0] <= 0xf4 {
			length = 4
		}
		consumed := 1
		for consumed < length && consumed < len(raw) {
			b := raw[consumed]
			if b < 0x80 || b > 0xbf {
				break
			}
			if consumed == 1 && ((raw[0] == 0xe0 && b < 0xa0) || (raw[0] == 0xed && b > 0x9f) || (raw[0] == 0xf0 && b < 0x90) || (raw[0] == 0xf4 && b > 0x8f)) {
				break
			}
			consumed++
		}
		out.WriteRune(utf8.RuneError)
		raw = raw[consumed:]
	}
	return out.String()
}

func instructionControls(data []byte) []instructionWarning {
	warnings := []instructionWarning{}
	line := 1
	for offset, r := range string(data) {
		if r == '\n' {
			line++
		}
		if unicode.Is(unicode.Cf, r) || r == 0x034f {
			if len(warnings) == 32 {
				warnings = append(warnings, instructionWarning{Code: "SUSPICIOUS_CONTROL_WARNINGS_CAPPED"})
				break
			}
			warnings = append(warnings, instructionWarning{Code: "SUSPICIOUS_UNICODE_CONTROL", Line: line, ByteOffset: offset, CodePoint: fmt.Sprintf("U+%04X", r)})
		}
	}
	return warnings
}
