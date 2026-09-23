// Package skillexport renders admitted learned traces as Agent Skills
// documents: one directory per admitted rule holding a short SKILL.md with
// YAML frontmatter and a references/trace.md that carries the full detail
// (LTA-V0-006 to LTA-V0-008). It is a pure projection over trace rows the
// reader already re-validated; it never reads or writes repository state.
package skillexport

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/trace"
)

const (
	// Evaluation is the only evaluation result V0 can name: the learned-path
	// gate (LTA-V0-001) admits the mechanism, never an individual row, so no
	// per-trace evaluation result is recorded anywhere.
	Evaluation = "NOT_RECORDED"

	// namePrefix plus 16 trace-id hex digits stays inside the 64-character
	// Agent Skills name bound and the [a-z0-9-] alphabet.
	namePrefix          = "corvint-learned-"
	nameDigits          = 16
	descriptionTaskRune = 200
	descriptionPrefix   = "Learned from an admitted passed Corvint trace: "
	descriptionSuffix   = " Use when a task matches it."

	SkillFile     = "SKILL.md"
	ReferenceFile = "references/trace.md"
)

// Skill is one exported admitted rule: its directory name and file bytes.
type Skill struct {
	Name   string
	Digest string
	Files  map[string][]byte
}

// Export projects every admitted rule among records, sorted by name. Rows whose
// outcome is not `passed` are not rules and are skipped; a row that cannot be
// re-encoded (a tampered digest) fails the whole export.
func Export(records []trace.Record) ([]Skill, error) {
	skills := make([]Skill, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if record.Outcome != "passed" {
			continue
		}
		skill, err := ExportRecord(record)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[skill.Name]; duplicate {
			return nil, fmt.Errorf("exported skill name collides: %s", skill.Name)
		}
		seen[skill.Name] = struct{}{}
		skills = append(skills, skill)
	}
	sort.Slice(skills, func(left, right int) bool { return skills[left].Name < skills[right].Name })
	return skills, nil
}

// ExportRecord renders one admitted rule. Only a `passed` row is a rule; the
// admission evidence digest is the sha256 of the row bytes the store holds.
func ExportRecord(record trace.Record) (Skill, error) {
	if record.Outcome != "passed" {
		return Skill{}, fmt.Errorf("trace %s is not an admitted rule: outcome %q", record.TraceID, record.Outcome)
	}
	row, err := trace.Encode(record)
	if err != nil {
		return Skill{}, fmt.Errorf("trace %s is not exportable: %w", record.TraceID, err)
	}
	if len(record.TraceID) < nameDigits {
		return Skill{}, fmt.Errorf("trace id too short: %q", record.TraceID)
	}
	sum := sha256.Sum256(row)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	name := namePrefix + record.TraceID[:nameDigits]
	return Skill{
		Name:   name,
		Digest: digest,
		Files: map[string][]byte{
			SkillFile:     []byte(skillDocument(name, digest, record)),
			ReferenceFile: []byte(referenceDocument(digest, record)),
		},
	}, nil
}

func skillDocument(name, digest string, record trace.Record) string {
	var out strings.Builder
	out.WriteString("---\nname: " + name + "\n")
	out.WriteString("description: " + yamlQuote(description(record.Task)) + "\n---\n\n")
	out.WriteString("# " + name + "\n\n")
	out.WriteString("One admitted Corvint learned trace with outcome `passed`, exported by `corvint skill-export`.\n\n")
	out.WriteString("## Task\n\n" + record.Task + "\n\n")
	out.WriteString("## Provenance\n\n")
	out.WriteString("- Admission evidence digest: " + digest + "\n")
	out.WriteString("- Evaluation: " + Evaluation + "\n")
	out.WriteString("- Trace: " + record.TraceID + " at revision " + record.Revision + "\n\n")
	fmt.Fprintf(&out, "The trace opened %d paths, changed %d paths and ran %d verification commands; ", len(record.OpenedPaths), len(record.ChangedPaths), len(record.Verification))
	out.WriteString("read `" + ReferenceFile + "` for the full lists.\n")
	return out.String()
}

func referenceDocument(digest string, record trace.Record) string {
	var out strings.Builder
	out.WriteString("# Trace " + record.TraceID + "\n\n")
	out.WriteString("- Revision: " + record.Revision + "\n")
	out.WriteString("- Admission evidence digest: " + digest + "\n")
	out.WriteString("- Evaluation: " + Evaluation + "\n")
	out.WriteString("- Outcome: " + record.Outcome + "\n")
	writeList(&out, "Opened paths", record.OpenedPaths)
	writeList(&out, "Changed paths", record.ChangedPaths)
	writeList(&out, "Verification", record.Verification)
	return out.String()
}

func writeList(out *strings.Builder, heading string, values []string) {
	out.WriteString("\n## " + heading + "\n\n")
	if len(values) == 0 {
		out.WriteString("(none)\n")
		return
	}
	for _, value := range values {
		out.WriteString("- " + value + "\n")
	}
}

// description folds the task onto one printable line and bounds it well
// inside the 1024-character Agent Skills limit.
func description(task string) string {
	printable := strings.Map(printableRune, task)
	folded := strings.Join(strings.Fields(printable), " ")
	if utf8.RuneCountInString(folded) > descriptionTaskRune {
		runes := []rune(folded)
		folded = string(runes[:descriptionTaskRune]) + "..."
	}
	return descriptionPrefix + folded + descriptionSuffix
}

func printableRune(r rune) rune {
	if unicode.IsControl(r) {
		return ' '
	}
	return r
}

// yamlQuote writes a YAML double-quoted scalar so any task text stays one
// frontmatter value.
func yamlQuote(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
	return `"` + escaped + `"`
}
