package gitauth

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
)

// firstFewBytes is the prefix Git scans for a NUL to classify a blob as binary.
const firstFewBytes = 8000

// patchSection is one "diff --git" section the verified change set predicts.
// A typechange between kinds is two sections, a deletion then a creation.
type patchSection struct {
	path     string
	old, new TreeEntry
}

// requirePatchProvenance proves that the patch Git emitted is the canonical
// derivation of the verified change set: sections match the set one-to-one
// in order, every header line names the verified modes and OIDs, binary
// sections are pinned by OID, and every textual section replays the verified
// old bytes into the verified new bytes. Any other patch refuses.
func (r *Repository) requirePatchProvenance(ctx context.Context, out []byte, changed []changedEntry) error {
	sections := patchSections(changed)
	blobs, err := r.verifiedBlobs(ctx, sections)
	if err != nil {
		return err
	}
	cursor := patchCursor{rest: out}
	for _, section := range sections {
		if err := proveSection(&cursor, section, blobs); err != nil {
			return err
		}
	}
	if len(cursor.rest) != 0 {
		return unavailable("canonical patch carries a section the verified trees do not")
	}
	return nil
}

// verifiedBlobs reads and self-hashes every blob a section's content derives
// from, in one batch bounded by MaxTotalBlobBytes. Gitlink sides derive their
// content from the OID alone, and a mode-only section contributes no content
// to the patch.
func (r *Repository) verifiedBlobs(ctx context.Context, sections []patchSection) (map[string][]byte, error) {
	var oids []string
	blobs := map[string][]byte{}
	for _, section := range sections {
		if section.old.OID == section.new.OID {
			continue
		}
		for _, side := range []TreeEntry{section.old, section.new} {
			if side.OID == "" || side.Type != "blob" || blobs[side.OID] != nil {
				continue
			}
			blobs[side.OID] = []byte{}
			oids = append(oids, side.OID)
		}
	}
	if len(oids) == 0 {
		return blobs, nil
	}
	stdin := []byte(strings.Join(oids, "\x00") + "\x00")
	out, err := r.gitStdin(ctx, MaxTotalBlobBytes, stdin, "cat-file", "--batch", "-z")
	if err != nil {
		return nil, err
	}
	records := batchRecords{rest: out}
	for _, oid := range oids {
		record, err := records.next()
		if err != nil {
			return nil, err
		}
		if record.missing || record.oid != oid || record.kind != "blob" {
			return nil, unavailable("patch input %s does not resolve locally", oid)
		}
		if err := requireObjectIdentity(ctx, "blob", oid, record.body); err != nil {
			return nil, err
		}
		blobs[oid] = record.body
	}
	return blobs, nil
}

// patchSections predicts the sections Git emits: one per changed entry,
// except that a change between kinds (regular, symlink, gitlink) is a
// deletion section followed by a creation section.
func patchSections(changed []changedEntry) []patchSection {
	sections := make([]patchSection, 0, len(changed))
	for _, entry := range changed {
		if entry.old.OID != "" && entry.new.OID != "" && entryKind(entry.old.Mode) != entryKind(entry.new.Mode) {
			sections = append(sections, patchSection{path: entry.path, old: entry.old})
			sections = append(sections, patchSection{path: entry.path, new: entry.new})
			continue
		}
		sections = append(sections, patchSection{path: entry.path, old: entry.old, new: entry.new})
	}
	return sections
}

// entryKind is the S_IFMT part of a six-digit octal mode.
func entryKind(mode string) string {
	return mode[:min(len(mode), 2)]
}

func proveSection(cursor *patchCursor, section patchSection, blobs map[string][]byte) error {
	oldLabel, newLabel := sideLabel("a/", section.path, section.old), sideLabel("b/", section.path, section.new)
	if err := cursor.expect("diff --git " + quoteLabel("a/", section.path) + " " + quoteLabel("b/", section.path)); err != nil {
		return err
	}
	for _, line := range modeLines(section) {
		if err := cursor.expect(line); err != nil {
			return err
		}
	}
	if section.old.OID == section.new.OID {
		return nil
	}
	if err := cursor.expect(indexLine(section)); err != nil {
		return err
	}
	oldBytes, newBytes := sideContent(section.old, blobs), sideContent(section.new, blobs)
	if bytes.Equal(oldBytes, newBytes) {
		return nil
	}
	if isBinary(oldBytes) || isBinary(newBytes) {
		return cursor.expect("Binary files " + oldLabel + " and " + newLabel + " differ")
	}
	if err := cursor.expect("--- " + filePairLabel(oldLabel)); err != nil {
		return err
	}
	if err := cursor.expect("+++ " + filePairLabel(newLabel)); err != nil {
		return err
	}
	return replayHunks(cursor, oldBytes, newBytes)
}

func modeLines(section patchSection) []string {
	switch {
	case section.old.OID == "":
		return []string{"new file mode " + section.new.Mode}
	case section.new.OID == "":
		return []string{"deleted file mode " + section.old.Mode}
	case section.old.Mode != section.new.Mode:
		return []string{"old mode " + section.old.Mode, "new mode " + section.new.Mode}
	}
	return nil
}

func indexLine(section patchSection) string {
	width := max(len(section.old.OID), len(section.new.OID))
	null := strings.Repeat("0", width)
	oldOID, newOID := section.old.OID, section.new.OID
	if oldOID == "" {
		oldOID = null
	}
	if newOID == "" {
		newOID = null
	}
	line := "index " + oldOID + ".." + newOID
	if section.old.Mode == section.new.Mode {
		line += " " + section.old.Mode
	}
	return line
}

// sideContent is the bytes Git diffs for one side: nothing for an absent
// side, the commit line for a gitlink, and the verified blob otherwise.
func sideContent(side TreeEntry, blobs map[string][]byte) []byte {
	switch {
	case side.OID == "":
		return nil
	case side.Type == "commit":
		return []byte("Subproject commit " + side.OID + "\n")
	}
	return blobs[side.OID]
}

func isBinary(content []byte) bool {
	return bytes.IndexByte(content[:min(len(content), firstFewBytes)], 0) >= 0
}

func sideLabel(prefix, path string, side TreeEntry) string {
	if side.OID == "" {
		return "/dev/null"
	}
	return quoteLabel(prefix, path)
}

// filePairLabel is the "---"/"+++" label: Git appends a tab when it holds a
// space so the name survives tools that split on whitespace.
func filePairLabel(label string) string {
	if strings.Contains(label, " ") {
		return label + "\t"
	}
	return label
}

// quoteLabel renders prefix+path as Git's quote_c_style does under the pinned
// core.quotePath=false: control bytes, DEL, double quote, and backslash force
// a double-quoted form; every other byte, including non-ASCII, is literal.
func quoteLabel(prefix, path string) string {
	label := prefix + path
	quoted := false
	var sb strings.Builder
	sb.WriteByte('"')
	for _, c := range []byte(label) {
		quoted = quoted || c < 0x20 || c == 0x7f || c == '"' || c == '\\'
		sb.WriteString(quoteByte(c))
	}
	sb.WriteByte('"')
	if !quoted {
		return label
	}
	return sb.String()
}

var quoteEscapes = map[byte]string{
	'\a': `\a`, '\b': `\b`, '\t': `\t`, '\n': `\n`, '\v': `\v`, '\f': `\f`, '\r': `\r`, '"': `\"`, '\\': `\\`,
}

func quoteByte(c byte) string {
	if escape, ok := quoteEscapes[c]; ok {
		return escape
	}
	if c < 0x20 || c == 0x7f {
		return fmt.Sprintf("\\%03o", c)
	}
	return string([]byte{c})
}

// patchCursor reads one LF-terminated line at a time from the patch bytes.
type patchCursor struct {
	rest []byte
}

func (c *patchCursor) line() ([]byte, error) {
	line, rest, found := bytes.Cut(c.rest, []byte{'\n'})
	if !found {
		return nil, unavailable("canonical patch is truncated")
	}
	c.rest = rest
	return line, nil
}

func (c *patchCursor) expect(want string) error {
	line, err := c.line()
	if err != nil {
		return err
	}
	if string(line) != want {
		return unavailable("canonical patch does not match the verified trees")
	}
	return nil
}

// replayHunks applies the section's hunks to the verified old bytes and
// requires the result to equal the verified new bytes. Context and removed
// lines must equal the old line they stand for, so every hunk byte is
// accounted for by a verified object.
func replayHunks(cursor *patchCursor, oldBytes, newBytes []byte) error {
	old := splitRecords(oldBytes)
	var out []byte
	consumed, emitted := 0, 0
	for bytes.HasPrefix(cursor.rest, []byte("@@ -")) {
		header, err := cursor.line()
		if err != nil {
			return err
		}
		oldStart, oldCount, newStart, newCount, ok := parseHunkHeader(string(header))
		if !ok || oldStart < consumed || oldStart > len(old) || newStart != emitted+(oldStart-consumed) {
			return unavailable("canonical patch hunk does not match the verified trees")
		}
		for ; consumed < oldStart; consumed++ {
			out = append(out, old[consumed]...)
		}
		emitted = newStart
		for oldCount > 0 || newCount > 0 {
			prefix, record, err := cursor.bodyLine()
			if err != nil {
				return err
			}
			if prefix != '+' && (oldCount == 0 || consumed >= len(old) || !bytes.Equal(old[consumed], record)) {
				return unavailable("canonical patch hunk does not match the verified trees")
			}
			if prefix != '+' {
				consumed++
				oldCount--
			}
			if prefix == '-' {
				continue
			}
			if newCount == 0 {
				return unavailable("canonical patch hunk does not match the verified trees")
			}
			out = append(out, record...)
			emitted++
			newCount--
		}
	}
	for ; consumed < len(old); consumed++ {
		out = append(out, old[consumed]...)
	}
	if !bytes.Equal(out, newBytes) {
		return unavailable("canonical patch does not replay the verified trees")
	}
	return nil
}

// bodyLine returns one hunk body line's prefix and the record it stands for,
// folding a following "\ No newline at end of file" marker into the record.
func (c *patchCursor) bodyLine() (byte, []byte, error) {
	line, err := c.line()
	if err != nil {
		return 0, nil, err
	}
	if len(line) == 0 || !strings.ContainsRune(" -+", rune(line[0])) {
		return 0, nil, unavailable("canonical patch hunk does not match the verified trees")
	}
	record := append(bytes.Clone(line[1:]), '\n')
	if bytes.HasPrefix(c.rest, []byte("\\ ")) {
		if _, err := c.line(); err != nil {
			return 0, nil, err
		}
		record = record[:len(record)-1]
	}
	return line[0], record, nil
}

// splitRecords splits content the way xdiff does: each record keeps its LF,
// and a final record without one is kept as is.
func splitRecords(content []byte) [][]byte {
	var records [][]byte
	for len(content) > 0 {
		n := bytes.IndexByte(content, '\n') + 1
		if n == 0 {
			n = len(content)
		}
		records = append(records, content[:n])
		content = content[n:]
	}
	return records
}

// parseHunkHeader reads "@@ -a[,b] +c[,d] @@..." into zero-based starts and
// counts; a range with count zero names the line before the insertion point.
func parseHunkHeader(header string) (oldStart, oldCount, newStart, newCount int, ok bool) {
	rest, found := strings.CutPrefix(header, "@@ -")
	if !found {
		return 0, 0, 0, 0, false
	}
	oldRange, rest, found := strings.Cut(rest, " +")
	if !found {
		return 0, 0, 0, 0, false
	}
	newRange, _, found := strings.Cut(rest, " @@")
	if !found {
		return 0, 0, 0, 0, false
	}
	oldStart, oldCount, ok = parseRange(oldRange)
	if !ok {
		return 0, 0, 0, 0, false
	}
	newStart, newCount, ok = parseRange(newRange)
	return oldStart, oldCount, newStart, newCount, ok
}

func parseRange(text string) (start, count int, ok bool) {
	startText, countText, found := strings.Cut(text, ",")
	count = 1
	if found {
		n, err := strconv.Atoi(countText)
		if err != nil || n < 0 || countText != strconv.Itoa(n) {
			return 0, 0, false
		}
		count = n
	}
	start, err := strconv.Atoi(startText)
	if err != nil || start < 0 || startText != strconv.Itoa(start) {
		return 0, 0, false
	}
	if count > 0 {
		start--
	}
	return start, count, start >= 0
}
