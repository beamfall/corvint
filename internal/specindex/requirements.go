// Package specindex reads the generated requirement index, docs/specs/REQUIREMENTS.tsv, and
// locates a requirement's definition line in its spec (SRG-V0-015, SRG-V0-017).
package specindex

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// Row is one requirement of the index. Line is the definition's one-based line number when the
// text carried a line column (the `--lines` view, or an index committed before SRG-V0-015), and
// 0 otherwise; DefinitionLine resolves it from the spec text.
type Row struct {
	ID         string
	File       string
	Line       int
	Title      string
	Acceptance string
	AcceptedBy string
}

// ErrHeader reports index text whose first line is not a requirement index header.
var ErrHeader = errors.New("docs/specs/REQUIREMENTS.tsv header is not id, file[, line], title")

// ParseIndex reads index text in either layout: the committed `id file title acceptance
// acceptedBy` (SRG-V0-015) or the line-bearing `id file line title ...`. Columns after title are
// optional. A row without an id, a file or a title, or whose line column is not a positive
// integer, is skipped rather than failing the whole index; a later row for an id replaces an
// earlier one.
func ParseIndex(text string) (map[string]Row, error) {
	lines := strings.Split(text, "\n")
	header := strings.Split(strings.TrimSuffix(lines[0], "\r"), "\t")
	if len(header) < 3 || header[0] != "id" || header[1] != "file" {
		return nil, ErrHeader
	}
	line, title := -1, 2
	if header[2] == "line" {
		line, title = 2, 3
	}
	if len(header) <= title || header[title] != "title" {
		return nil, ErrHeader
	}
	column := func(fields []string, name string) string {
		for i, value := range header {
			if value == name && i < len(fields) {
				return fields[i]
			}
		}
		return ""
	}
	rows := make(map[string]Row)
	for _, text := range lines[1:] {
		fields := strings.Split(strings.TrimSuffix(text, "\r"), "\t")
		if len(fields) <= title || fields[0] == "" || fields[1] == "" {
			continue
		}
		row := Row{ID: fields[0], File: fields[1], Title: fields[title],
			Acceptance: column(fields, "acceptance"), AcceptedBy: column(fields, "acceptedBy")}
		if line >= 0 {
			number, err := strconv.Atoi(fields[line])
			if err != nil || number < 1 {
				continue
			}
			row.Line = number
		}
		rows[row.ID] = row
	}
	return rows, nil
}

// blank is the ASCII whitespace Perl's \s matches in the generator's byte strings.
const blank = "\t\n\v\f\r "

var marked = regexp.MustCompile("^[\t\n\v\f\r ]*(?:[-*][\t\n\v\f\r ]+|\\*\\*|`)")

// DefinitionLine returns the one-based line of spec text that defines id under the generator's
// ranks (SRG-V0-003, SRG-V0-004): the first definition line, else the first table row that opens
// with the id, else 0. It is how a reader of the committed index, which carries no line number,
// finds the clause (SRG-V0-017).
func DefinitionLine(text, id string) int {
	quoted := regexp.QuoteMeta(id)
	const lead = "^[\t\n\v\f\r ]*(?:[-*][\t\n\v\f\r ]+)?(?:\\*\\*)?`?"
	period := regexp.MustCompile(lead + quoted + "`?\\.(?:\\*\\*)?[\t\n\v\f\r ]")
	colon := regexp.MustCompile(lead + quoted + "`?(?:\\*\\*)?[\t\n\v\f\r ]*:")
	table := regexp.MustCompile("^[\t\n\v\f\r ]*\\|[\t\n\v\f\r ]*`?" + quoted + "`?(?:[\t\n\v\f\r |.])")
	firstTable := 0
	previousBlank := true
	lines := strings.SplitAfter(text, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		boundary := previousBlank || marked.MatchString(line)
		previousBlank = strings.Trim(line, blank) == ""
		if !strings.Contains(line, id) {
			continue
		}
		if boundary && (period.MatchString(line) || colon.MatchString(line)) {
			return i + 1
		}
		if firstTable == 0 && table.MatchString(line) {
			firstTable = i + 1
		}
	}
	return firstTable
}
