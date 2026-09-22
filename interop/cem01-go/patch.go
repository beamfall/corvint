package main

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var hunkHeaderRE = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@.*$`)

type bodyLine struct {
	prefix  byte
	payload []byte
}

type patchHunk struct {
	ID       string
	OldPath  *string
	NewPath  *string
	OldRange lineRange
	NewRange lineRange
	Body     []bodyLine
	RawBody  []byte
}

type filePatch struct {
	oldPath *string
	newPath *string
	mode    string
	hunks   []*patchHunk
}

type parsedPatch struct {
	files []*filePatch
	hunks []*patchHunk
}

type patchLines struct {
	lines [][]byte
	pos   int
}

func tokenizeLF(b []byte) ([][]byte, *cemError) {
	if len(b) == 0 {
		return nil, invalid("empty-input")
	}
	records := bytes.Count(b, []byte{'\n'})
	if b[len(b)-1] != '\n' {
		records++
	}
	if records > maxRecords {
		return nil, invalid("too-many-lines")
	}
	lines := make([][]byte, 0, records)
	for len(b) > 0 {
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			lines = append(lines, b)
			break
		}
		lines = append(lines, b[:i+1])
		b = b[i+1:]
	}
	return lines, nil
}

func parsePatch(raw []byte) (*parsedPatch, *cemError) {
	if len(raw) == 0 || len(raw) > maxPatchBytes || bytes.IndexByte(raw, 0) >= 0 {
		return nil, invalid("patch")
	}
	lines, err := tokenizeLF(raw)
	if err != nil {
		return nil, err
	}
	for _, line := range lines {
		// A binary marker is a whole record; hunk body records always begin with a prefix byte.
		if hasPrefixLine(line, "GIT binary patch") || hasPrefixLine(line, "Binary files ") {
			return nil, invalid("patch")
		}
	}
	p := &parsedPatch{}
	in := &patchLines{lines: lines}
	seenPairs, oldUsed, newUsed, hunkIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for in.pos < len(in.lines) {
		var f *filePatch
		if hasPrefixLine(in.peek(), "diff --git ") {
			f, err = parseGitFile(in)
		} else if hasPrefixLine(in.peek(), "--- ") {
			f, err = parseBareFile(in)
		} else {
			return nil, invalid("patch-top-level")
		}
		if err != nil {
			return nil, err
		}
		pair := ptrVal(f.oldPath) + "\x00" + ptrVal(f.newPath)
		if seenPairs[pair] {
			return nil, invalid("duplicate-file-group")
		}
		seenPairs[pair] = true
		if f.oldPath != nil {
			if oldUsed[*f.oldPath] {
				return nil, invalid("duplicate-old-path")
			}
			oldUsed[*f.oldPath] = true
		}
		if f.newPath != nil {
			if newUsed[*f.newPath] {
				return nil, invalid("duplicate-new-path")
			}
			newUsed[*f.newPath] = true
		}
		for _, h := range f.hunks {
			if hunkIDs[h.ID] {
				return nil, invalid("duplicate-hunk-id")
			}
			hunkIDs[h.ID] = true
			p.hunks = append(p.hunks, h)
		}
		p.files = append(p.files, f)
	}
	if len(p.hunks) == 0 || len(p.hunks) > maxHunks {
		return nil, invalid("patch-hunk-count")
	}
	return p, nil
}

func (p *patchLines) peek() []byte { return p.lines[p.pos] }
func (p *patchLines) take() []byte { b := p.lines[p.pos]; p.pos++; return b }

func trimLF(line []byte) ([]byte, bool) {
	if len(line) > 0 && line[len(line)-1] == '\n' {
		return line[:len(line)-1], true
	}
	return line, false
}

func exactText(line []byte, prefix string) (string, bool) {
	b, lf := trimLF(line)
	if !lf || !bytes.HasPrefix(b, []byte(prefix)) || !utf8Bytes(b[len(prefix):]) {
		return "", false
	}
	return string(b[len(prefix):]), true
}

func utf8Bytes(b []byte) bool                       { return bytes.Equal([]byte(string(b)), b) }
func hasPrefixLine(line []byte, prefix string) bool { return bytes.HasPrefix(line, []byte(prefix)) }

func parseBareFile(in *patchLines) (*filePatch, *cemError) {
	oldText, ok := exactText(in.take(), "--- ")
	if !ok || in.pos >= len(in.lines) {
		return nil, invalid("file-header")
	}
	newText, ok := exactText(in.take(), "+++ ")
	if !ok {
		return nil, invalid("file-header")
	}
	oldPath, ok1 := headerPath(oldText, "a/", false)
	newPath, ok2 := headerPath(newText, "b/", false)
	if !ok1 || !ok2 || oldPath == nil || newPath == nil || *oldPath != *newPath {
		return nil, invalid("diff-metadata-mismatch")
	}
	f := &filePatch{oldPath: oldPath, newPath: newPath}
	if err := parseHunks(in, f); err != nil {
		return nil, err
	}
	return f, nil
}

func parseGitFile(in *patchLines) (*filePatch, *cemError) {
	diffHeader, ok := exactText(in.take(), "diff --git ")
	if !ok {
		return nil, invalid("diff-header")
	}

	var indexMode, createMode, deleteMode, renameFrom, renameTo *string
	var similaritySeen bool
	for in.pos < len(in.lines) && !hasPrefixLine(in.peek(), "--- ") {
		lineBytes := in.take()
		line, lf := trimLF(lineBytes)
		if !lf {
			return nil, invalid("metadata")
		}
		s := string(line)
		switch {
		case strings.HasPrefix(s, "index "):
			if indexMode != nil {
				return nil, invalid("metadata-repeat")
			}
			mode, err := parseIndex(s)
			if err != nil {
				return nil, err
			}
			indexMode = &mode
		case strings.HasPrefix(s, "new file mode "):
			if createMode != nil || deleteMode != nil {
				return nil, invalid("metadata-repeat")
			}
			mode := strings.TrimPrefix(s, "new file mode ")
			if !validMode(mode) {
				return nil, invalid("unsupported-tree-mode")
			}
			createMode = &mode
		case strings.HasPrefix(s, "deleted file mode "):
			if deleteMode != nil || createMode != nil {
				return nil, invalid("metadata-repeat")
			}
			mode := strings.TrimPrefix(s, "deleted file mode ")
			if !validMode(mode) {
				return nil, invalid("unsupported-tree-mode")
			}
			deleteMode = &mode
		case strings.HasPrefix(s, "rename from "):
			if renameFrom != nil {
				return nil, invalid("metadata-repeat")
			}
			path := strings.TrimPrefix(s, "rename from ")
			if !validPath(path) {
				return nil, invalid("path")
			}
			renameFrom = &path
		case strings.HasPrefix(s, "rename to "):
			if renameTo != nil {
				return nil, invalid("metadata-repeat")
			}
			path := strings.TrimPrefix(s, "rename to ")
			if !validPath(path) {
				return nil, invalid("path")
			}
			renameTo = &path
		case strings.HasPrefix(s, "similarity index ") || strings.HasPrefix(s, "dissimilarity index "):
			if similaritySeen {
				return nil, invalid("metadata-repeat")
			}
			similaritySeen = true
			value := strings.TrimPrefix(strings.TrimPrefix(s, "dis"), "similarity index ")
			if !strings.HasSuffix(value, "%") {
				return nil, invalid("metadata")
			}
			n, e := strconv.Atoi(strings.TrimSuffix(value, "%"))
			if e != nil || n < 0 || n > 100 || strconv.Itoa(n)+"%" != value {
				return nil, invalid("metadata")
			}
		case strings.HasPrefix(s, "old mode ") || strings.HasPrefix(s, "new mode "):
			return nil, invalid("unsupported-tree-mode")
		default:
			return nil, invalid("metadata")
		}
	}
	if in.pos+1 >= len(in.lines) {
		return nil, invalid("file-header")
	}
	oldText, ok := exactText(in.take(), "--- ")
	if !ok {
		return nil, invalid("file-header")
	}
	newText, ok := exactText(in.take(), "+++ ")
	if !ok {
		return nil, invalid("file-header")
	}
	oldPath, ok1 := headerPath(oldText, "a/", true)
	newPath, ok2 := headerPath(newText, "b/", true)
	if !ok1 || !ok2 || oldPath == nil && newPath == nil {
		return nil, invalid("file-header")
	}
	diffOld, diffNew := oldPath, newPath
	if diffOld == nil {
		diffOld = newPath
	}
	if diffNew == nil {
		diffNew = oldPath
	}
	if diffOld == nil || diffNew == nil || diffHeader != "a/"+*diffOld+" b/"+*diffNew {
		return nil, invalid("diff-metadata-mismatch")
	}
	if oldPath == nil {
		if createMode == nil || deleteMode != nil || newPath == nil {
			return nil, invalid("diff-metadata-mismatch")
		}
		if indexMode != nil && *indexMode != "" && *indexMode != *createMode {
			return nil, invalid("diff-metadata-mismatch")
		}
	} else if newPath == nil {
		if deleteMode == nil || createMode != nil {
			return nil, invalid("diff-metadata-mismatch")
		}
		if indexMode != nil && *indexMode != "" && *indexMode != *deleteMode {
			return nil, invalid("diff-metadata-mismatch")
		}
	} else {
		if createMode != nil || deleteMode != nil {
			return nil, invalid("diff-metadata-mismatch")
		}
	}
	if (renameFrom == nil) != (renameTo == nil) {
		return nil, invalid("diff-metadata-mismatch")
	}
	if renameFrom != nil && (*renameFrom != *diffOld || *renameTo != *diffNew || oldPath == nil || newPath == nil || *oldPath == *newPath) {
		return nil, invalid("diff-metadata-mismatch")
	}
	if oldPath != nil && newPath != nil && *oldPath != *newPath && renameFrom == nil {
		return nil, invalid("diff-metadata-mismatch")
	}
	f := &filePatch{oldPath: oldPath, newPath: newPath}
	if createMode != nil {
		f.mode = *createMode
	}
	if deleteMode != nil {
		f.mode = *deleteMode
	}
	if err := parseHunks(in, f); err != nil {
		return nil, err
	}
	return f, nil
}

func parseIndex(s string) (string, *cemError) {
	fields := strings.Split(strings.TrimPrefix(s, "index "), " ")
	if len(fields) < 1 || len(fields) > 2 {
		return "", invalid("metadata")
	}
	oids := strings.Split(fields[0], "..")
	if len(oids) != 2 || !abbrevOID(oids[0]) || !abbrevOID(oids[1]) {
		return "", invalid("metadata")
	}
	if len(fields) == 2 {
		if !validMode(fields[1]) {
			return "", invalid("unsupported-tree-mode")
		}
		return fields[1], nil
	}
	return "", nil
}

func abbrevOID(s string) bool { return len(s) >= 4 && len(s) <= 64 && lowerHex(s) }
func validMode(s string) bool { return s == "100644" || s == "100755" }

func headerPath(s, prefix string, nullOK bool) (*string, bool) {
	if s == "/dev/null" {
		return nil, nullOK
	}
	if !strings.HasPrefix(s, prefix) {
		return nil, false
	}
	p := strings.TrimPrefix(s, prefix)
	if !validPath(p) {
		return nil, false
	}
	return &p, true
}

func parseHunks(in *patchLines, f *filePatch) *cemError {
	var lastOldEnd uint64
	var delta int64
	for in.pos < len(in.lines) && hasPrefixLine(in.peek(), "@@ ") {
		h, err := parseHunk(in, f.oldPath, f.newPath)
		if err != nil {
			return err
		}
		oldPos := h.OldRange.Start
		if h.OldRange.Count > 0 {
			oldPos--
		}
		if len(f.hunks) > 0 && oldPos < lastOldEnd {
			return invalid("hunk-order")
		}
		// ALGORITHMS.md parser rule: reject inconsistent new positions (decision 0192).
		expectedNew := int64(oldPos) + delta
		if h.NewRange.Count > 0 {
			expectedNew++
		}
		if expectedNew != int64(h.NewRange.Start) {
			return invalid("new-cursor")
		}
		lastOldEnd = oldPos + h.OldRange.Count
		delta += int64(h.NewRange.Count) - int64(h.OldRange.Count)
		f.hunks = append(f.hunks, h)
	}
	if len(f.hunks) == 0 {
		return invalid("zero-hunks")
	}
	return nil
}

func parseHunk(in *patchLines, oldPath, newPath *string) (*patchHunk, *cemError) {
	header := in.take()
	h, lf := trimLF(header)
	if !lf {
		return nil, invalid("hunk-header")
	}
	if len(h) > 0 && h[len(h)-1] == '\r' {
		h = h[:len(h)-1]
	}
	m := hunkHeaderRE.FindSubmatch(h)
	if m == nil {
		return nil, invalid("hunk-header")
	}
	oldRange, ok := parseHeaderRange(m[1], m[2])
	if !ok {
		return nil, invalid("hunk-range")
	}
	newRange, ok := parseHeaderRange(m[3], m[4])
	if !ok {
		return nil, invalid("hunk-range")
	}
	ph := &patchHunk{OldPath: oldPath, NewPath: newPath, OldRange: oldRange, NewRange: newRange}
	var oldCount, newCount uint64
	changed := false
	for oldCount < oldRange.Count || newCount < newRange.Count {
		if in.pos >= len(in.lines) {
			return nil, invalid("short-hunk")
		}
		line := in.take()
		if len(line) == 0 || line[0] != ' ' && line[0] != '-' && line[0] != '+' {
			return nil, invalid("hunk-payload")
		}
		if line[len(line)-1] != '\n' {
			return nil, invalid("hunk-payload")
		}
		payload := append([]byte(nil), line[1:]...)
		bl := bodyLine{prefix: line[0], payload: payload}
		switch line[0] {
		case ' ':
			oldCount++
			newCount++
		case '-':
			oldCount++
			changed = true
		case '+':
			newCount++
			changed = true
		}
		if oldCount > oldRange.Count || newCount > newRange.Count {
			return nil, invalid("surplus-hunk-payload")
		}
		ph.Body = append(ph.Body, bl)
		ph.RawBody = append(ph.RawBody, line...)
		if in.pos < len(in.lines) && bytes.Equal(in.peek(), []byte("\\ No newline at end of file\n")) {
			if len(payload) == 0 || payload[len(payload)-1] != '\n' {
				return nil, invalid("newline-marker")
			}
			ph.Body[len(ph.Body)-1].payload = ph.Body[len(ph.Body)-1].payload[:len(payload)-1]
			ph.RawBody = append(ph.RawBody, in.take()...)
		}
	}
	if !changed {
		return nil, invalid("unchanged-hunk")
	}
	if in.pos < len(in.lines) {
		next := in.peek()
		if len(next) > 0 && (next[0] == ' ' || next[0] == '+' || next[0] == '-' && !bytes.HasPrefix(next, []byte("--- ")) || bytes.HasPrefix(next, []byte("\\ No newline"))) {
			return nil, invalid("surplus-hunk-payload")
		}
	}
	ph.ID = hunkID(ph)
	return ph, nil
}

func parseHeaderRange(startBytes, countBytes []byte) (lineRange, bool) {
	start, err := strconv.ParseUint(string(startBytes), 10, 64)
	if err != nil || strconv.FormatUint(start, 10) != string(startBytes) || start > maxWireInteger {
		return lineRange{}, false
	}
	count := uint64(1)
	if countBytes != nil {
		count, err = strconv.ParseUint(string(countBytes), 10, 64)
		if err != nil || strconv.FormatUint(count, 10) != string(countBytes) || count > maxWireInteger {
			return lineRange{}, false
		}
	}
	r := lineRange{Start: start, Count: count}
	return r, validRange(r)
}

func hunkID(h *patchHunk) string {
	old := "null"
	if h.OldPath != nil {
		old = canonicalString(*h.OldPath)
	}
	newp := "null"
	if h.NewPath != nil {
		newp = canonicalString(*h.NewPath)
	}
	s := `{"contentSha256":` + canonicalString(shaHex(h.RawBody)) + `,"newPath":` + newp +
		`,"newRange":{"count":` + strconv.FormatUint(h.NewRange.Count, 10) + `,"start":` + strconv.FormatUint(h.NewRange.Start, 10) +
		`},"oldPath":` + old + `,"oldRange":{"count":` + strconv.FormatUint(h.OldRange.Count, 10) + `,"start":` + strconv.FormatUint(h.OldRange.Start, 10) + `}}`
	return "hunk:sha256:" + shaHex([]byte(s))
}

func ptrVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (v *verifier) verifyPatchSimulation(commit string, p *parsedPatch) *cemError {
	paths := []string{}
	for _, f := range p.files {
		if f.oldPath != nil {
			paths = append(paths, *f.oldPath)
		}
		if f.newPath != nil {
			paths = append(paths, *f.newPath)
		}
	}
	if err := v.preloadTrees(commit, paths); err != nil {
		return err
	}
	oids := []string{}
	for _, f := range p.files {
		if f.oldPath != nil {
			entry, _ := v.treeEntry(commit, *f.oldPath)
			if entry != nil && entry.typ == "blob" {
				oids = append(oids, entry.oid)
			}
		}
	}
	if err := v.preloadBlobs(oids); err != nil {
		return err
	}
	for _, f := range p.files {
		var base []byte
		if f.oldPath == nil {
			entry, err := v.treeEntry(commit, *f.newPath)
			if err != nil {
				return err
			}
			if entry != nil {
				return invalid("create-destination-exists")
			}
		} else {
			entry, err := v.treeEntry(commit, *f.oldPath)
			if err != nil {
				return err
			}
			if entry == nil || entry.typ != "blob" || entry.mode != "100644" && entry.mode != "100755" {
				return invalid("patch-base")
			}
			base, err = v.blob(entry.oid)
			if err != nil {
				return err
			}
			if len(base) > 0 {
				if _, err := tokenizeLF(base); err != nil {
					return err
				}
			}
			if f.mode != "" && entry.mode != f.mode && f.newPath == nil {
				return invalid("diff-metadata-mismatch")
			}
			if f.newPath != nil && *f.newPath != *f.oldPath {
				dest, err := v.treeEntry(commit, *f.newPath)
				if err != nil {
					return err
				}
				if dest != nil {
					return invalid("rename-destination-exists")
				}
			}
		}
		out, err := simulate(v.ctx, base, f.hunks)
		if err != nil {
			return err
		}
		if f.newPath == nil && len(out) != 0 {
			return invalid("delete-not-empty")
		}
	}
	return nil
}

func simulate(ctx context.Context, base []byte, hunks []*patchHunk) ([]byte, *cemError) {
	lines := [][]byte{}
	if len(base) > 0 {
		var err *cemError
		lines, err = tokenizeLF(base)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, operational("verification-timeout")
		}
	}
	var out []byte
	oldCursor := uint64(0)
	for _, h := range hunks {
		if err := ctx.Err(); err != nil {
			return nil, operational("verification-timeout")
		}
		targetOld := h.OldRange.Start
		if h.OldRange.Count > 0 {
			targetOld--
		}
		if targetOld < oldCursor || targetOld > uint64(len(lines)) {
			return nil, invalid("hunk-range")
		}
		for oldCursor < targetOld {
			if oldCursor%1024 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, operational("verification-timeout")
				}
			}
			out = append(out, lines[oldCursor]...)
			oldCursor++
		}
		expectedNew := uint64(bytes.Count(out, []byte{'\n'}))
		if len(out) > 0 && out[len(out)-1] != '\n' {
			expectedNew++
		}
		if h.NewRange.Count > 0 {
			expectedNew++
		}
		if expectedNew != h.NewRange.Start {
			return nil, invalid("new-cursor")
		}
		for _, bl := range h.Body {
			if err := ctx.Err(); err != nil {
				return nil, operational("verification-timeout")
			}
			switch bl.prefix {
			case ' ', '-':
				if oldCursor >= uint64(len(lines)) || !bytes.Equal(lines[oldCursor], bl.payload) {
					return nil, invalid("patch-context")
				}
				oldCursor++
			}
			if bl.prefix == ' ' || bl.prefix == '+' {
				out = append(out, bl.payload...)
			}
		}
	}
	for oldCursor < uint64(len(lines)) {
		if oldCursor%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, operational("verification-timeout")
			}
		}
		out = append(out, lines[oldCursor]...)
		oldCursor++
	}
	if err := ctx.Err(); err != nil {
		return nil, operational("verification-timeout")
	}
	return out, nil
}

func verifyHunkMap(m *cemMap, p *parsedPatch) *cemError {
	parsed := make(map[string]*patchHunk, len(p.hunks))
	for _, h := range p.hunks {
		parsed[h.ID] = h
	}
	if len(m.Hunks) != len(p.hunks) {
		return invalid("hunk-count")
	}
	evidenceByID := map[string]bool{}
	for _, e := range m.Evidence {
		evidenceByID[e.ID] = true
	}
	cited, mapIDs, pairs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	validRelations := map[string]bool{"specification": true, "decision": true, "test-claim": true, "implementation": true, "call-site": true, "dependency": true, "incident": true}
	for _, mh := range m.Hunks {
		ph := parsed[mh.ID]
		if ph == nil || mapIDs[mh.ID] {
			return invalid("hunk-id")
		}
		mapIDs[mh.ID] = true
		path := ph.OldPath
		if ph.NewPath != nil {
			path = ph.NewPath
		}
		if path == nil || mh.Path != *path || mh.OldRange != ph.OldRange || mh.NewRange != ph.NewRange {
			return invalid("hunk-range")
		}
		switch mh.Disposition {
		case "supported":
			if mh.Reason != "evidence-backed" || len(mh.Basis) < 1 || len(mh.Basis) > maxBases {
				return invalid("basis")
			}
			for _, b := range mh.Basis {
				if !evidenceByID[b.EvidenceID] || !validRelations[b.Relation] {
					return invalid("basis")
				}
				pair := b.EvidenceID + "\x00" + b.Relation
				if pairs[pair] {
					return invalid("duplicate-basis")
				}
				pairs[pair], cited[b.EvidenceID] = true, true
			}
		case "unknown":
			if len(mh.Basis) != 0 || mh.Reason != "no-evidence" && mh.Reason != "insufficient-evidence" && mh.Reason != "conflicting-evidence" {
				return invalid("basis")
			}
		case "mechanical":
			if len(mh.Basis) != 0 || !proveMechanical(ph, mh.Reason) {
				return invalid("unproven-mechanical")
			}
		default:
			return invalid("disposition")
		}
	}
	for _, e := range m.Evidence {
		if !cited[e.ID] {
			return invalid("orphan-evidence")
		}
	}
	return nil
}

func proveMechanical(h *patchHunk, reason string) bool {
	var removed, added []byte
	for _, bl := range h.Body {
		if bl.prefix == '-' {
			removed = append(removed, bl.payload...)
		}
		if bl.prefix == '+' {
			added = append(added, bl.payload...)
		}
	}
	if len(removed) == 0 || len(added) == 0 || bytes.Equal(removed, added) {
		return false
	}
	switch reason {
	case "line-ending-only":
		return bytes.Equal(bytes.ReplaceAll(removed, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(added, []byte("\r\n"), []byte("\n")))
	case "whitespace-only":
		if bytes.Count(removed, []byte{'\n'}) != bytes.Count(added, []byte{'\n'}) {
			return false
		}
		rr, er := tokenizeLF(removed)
		aa, ea := tokenizeLF(added)
		if er != nil || ea != nil || len(rr) != len(aa) {
			return false
		}
		for i := range rr {
			if !bytes.Equal(stripHorizontal(rr[i]), stripHorizontal(aa[i])) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func stripHorizontal(b []byte) []byte {
	o := make([]byte, 0, len(b))
	for _, c := range b {
		if c != '\t' && c != '\r' && c != '\f' && c != '\v' && c != ' ' {
			o = append(o, c)
		}
	}
	return o
}

var _ = fmt.Sprintf
