package skillexport

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/trace"
)

const testRevision = "0123456789abcdef0123456789abcdef01234567"

func newRecord(t *testing.T, task, outcome string) trace.Record {
	t.Helper()
	record, err := trace.NewRecord(trace.Input{
		Revision:     testRevision,
		Task:         task,
		OpenedPaths:  []string{"cache/demux.go"},
		ChangedPaths: []string{"cache/reader.go"},
		Verification: []string{"go test ./..."},
		Outcome:      outcome,
	}, []string{"cache/demux.go", "cache/reader.go"})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

// LTA-V0-006: the same admitted rows export to identical bytes on every run,
// in name order, and rows that are not `passed` are not rules.
func TestExportProducesDeterministicBytes_LTA006(t *testing.T) {
	t.Parallel()
	records := []trace.Record{newRecord(t, "Split demux key 2", "passed"), newRecord(t, "Split demux key 1", "failed"), newRecord(t, "Split demux key 0", "passed")}
	first, err := Export(records)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Export([]trace.Record{records[2], records[0], records[1]})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("exported %d and %d skills, want 2 passed rules", len(first), len(second))
	}
	for offset := range first {
		if first[offset].Name != second[offset].Name || first[offset].Name >= firstAfter(first, offset) {
			t.Fatalf("skill order differs or is unsorted: %q vs %q", first[offset].Name, second[offset].Name)
		}
		for _, file := range []string{SkillFile, ReferenceFile} {
			if !bytes.Equal(first[offset].Files[file], second[offset].Files[file]) {
				t.Fatalf("%s/%s bytes differ between runs", first[offset].Name, file)
			}
		}
	}
}

func firstAfter(skills []Skill, offset int) string {
	if offset+1 < len(skills) {
		return skills[offset+1].Name
	}
	return "\xff"
}

// LTA-V0-007: every exported skill names the admission evidence digest (the
// sha256 of the stored row bytes) and the evaluation result verbatim; a row
// that is not an admitted rule, or whose digest does not re-validate, is
// refused.
func TestExportNamesAdmissionDigestAndEvaluation_LTA007(t *testing.T) {
	t.Parallel()
	record := newRecord(t, "Split demux key", "passed")
	row, err := trace.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(row)
	want := "sha256:" + hex.EncodeToString(sum[:])
	skill, err := ExportRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Digest != want {
		t.Fatalf("digest = %s, want %s", skill.Digest, want)
	}
	for _, file := range []string{SkillFile, ReferenceFile} {
		text := string(skill.Files[file])
		if !strings.Contains(text, "- Admission evidence digest: "+want+"\n") || !strings.Contains(text, "- Evaluation: NOT_RECORDED\n") {
			t.Fatalf("%s lacks the digest or evaluation line:\n%s", file, text)
		}
	}
	for _, outcome := range []string{"failed", "blocked"} {
		if _, err := ExportRecord(newRecord(t, "Split demux key", outcome)); err == nil || !strings.Contains(err.Error(), "not an admitted rule") {
			t.Fatalf("outcome %s: err = %v, want refusal", outcome, err)
		}
	}
	tampered := record
	tampered.Task = "Different task"
	if _, err := ExportRecord(tampered); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("tampered row: err = %v, want digest refusal", err)
	}
}

var hostNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// hostLoad parses SKILL.md the way an Agent Skills loader (Claude Code, Codex)
// does: a leading `---` block of `key: value` lines, then a Markdown body.
func hostLoad(t *testing.T, document []byte) (map[string]string, string) {
	t.Helper()
	text := string(document)
	if !strings.HasPrefix(text, "---\n") {
		t.Fatalf("no frontmatter: %q", text)
	}
	rest := text[len("---\n"):]
	frontmatter, body, found := strings.Cut(rest, "\n---\n")
	if !found {
		t.Fatalf("unterminated frontmatter: %q", text)
	}
	fields := map[string]string{}
	for _, line := range strings.Split(frontmatter, "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			t.Fatalf("frontmatter line is not key: value: %q", line)
		}
		if strings.HasPrefix(value, "\"") {
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				t.Fatalf("frontmatter %s is not a quoted scalar: %v", key, err)
			}
			value = unquoted
		}
		fields[key] = value
	}
	return fields, body
}

// LTA-V0-008: a host loader parses the emitted frontmatter, finds a bounded
// `name` equal to the skill directory and a bounded non-empty `description`,
// and reloads the admission evidence digest from the body. No real host ran;
// this fixture reproduces the loader's frontmatter contract.
func TestHostRoundTripLoadsExportedSkill_LTA008(t *testing.T) {
	t.Parallel()
	task := "Quote \"demux\" key\n\ttab and \x01 control " + strings.Repeat("x", 400)
	skill, err := ExportRecord(newRecord(t, task, "passed"))
	if err != nil {
		t.Fatal(err)
	}
	fields, body := hostLoad(t, skill.Files[SkillFile])
	name, description := fields["name"], fields["description"]
	if name != skill.Name || len(name) == 0 || len(name) > 64 || !hostNamePattern.MatchString(name) {
		t.Fatalf("name %q is not a loadable skill name", name)
	}
	if len(description) == 0 || len(description) > 1024 || strings.ContainsAny(description, "\n\t\x01") || !strings.Contains(description, `Quote "demux" key tab and`) {
		t.Fatalf("description %q is not a loadable one-line description", description)
	}
	_, digestLine, found := strings.Cut(body, "- Admission evidence digest: ")
	if !found {
		t.Fatalf("body lacks the digest line:\n%s", body)
	}
	reloaded, _, _ := strings.Cut(digestLine, "\n")
	if reloaded != skill.Digest || !strings.HasPrefix(reloaded, "sha256:") || len(reloaded) != len("sha256:")+64 {
		t.Fatalf("reloaded digest %q, want %q", reloaded, skill.Digest)
	}
	if !strings.Contains(body, "read `"+ReferenceFile+"`") || !strings.Contains(string(skill.Files[ReferenceFile]), "- cache/reader.go\n") {
		t.Fatalf("progressive disclosure broken: body=%q reference=%q", body, skill.Files[ReferenceFile])
	}
}
