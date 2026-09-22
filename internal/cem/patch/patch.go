package patch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// GroupKind classifies one contiguous file group.
type GroupKind int

const (
	KindModify GroupKind = iota
	KindCreate
	KindDelete
	KindRename
)

// BodyLine is one hunk body line. Payload excludes the prefix byte and retains
// its trailing LF; StripLFOld/StripLFNew record a following
// no-final-newline marker for the respective reconstructed side.
type BodyLine struct {
	Prefix     byte
	Payload    []byte
	StripLFOld bool
	StripLFNew bool
}

// Hunk is one parsed textual hunk.
type Hunk struct {
	OldPath       *string
	NewPath       *string
	DisplayPath   string
	OldRange      wire.Range
	NewRange      wire.Range
	Body          []BodyLine
	RawBody       []byte
	ContentSha256 string
	ID            string
}

// Group is one contiguous file group.
type Group struct {
	Kind    GroupKind
	OldPath *string
	NewPath *string
	Mode    string // bound tree mode for creates and deletes
	Hunks   []*Hunk
}

// Patch is a fully parsed CEM 0.1 conformant patch.
type Patch struct {
	Groups []*Group
	Hunks  []*Hunk // every hunk in patch order
}

func malformed(format string, args ...any) *cemcode.Error {
	return cemcode.New(cemcode.MalformedPatch, format, args...)
}

func metadataMismatch(format string, args ...any) *cemcode.Error {
	return cemcode.New(cemcode.DiffMetadataMismatch, format, args...)
}

type metadata struct {
	indexSeen   bool
	indexMode   string
	newFileMode string
	deleteMode  string
	renameFrom  string
	renameTo    string
	similarity  bool
}

type parser struct {
	lines [][]byte
	pos   int
}

// Parse reads one bounded CEM 0.1 patch.
func Parse(raw []byte) (*Patch, error) {
	if len(raw) == 0 {
		return nil, cemcode.New(cemcode.BinaryPatch, "patch is empty")
	}
	if len(raw) > MaxPatchBytes {
		return nil, cemcode.New(cemcode.BinaryPatch, "patch exceeds %d bytes", MaxPatchBytes)
	}
	if bytes.IndexByte(raw, 0) >= 0 {
		return nil, cemcode.New(cemcode.BinaryPatch, "patch contains a NUL byte")
	}
	lines, err := SplitLF(raw)
	if err != nil {
		return nil, err
	}
	p := &parser{lines: lines}
	result := &Patch{}
	usedPairs := map[string]bool{}
	usedOld := map[string]bool{}
	usedNew := map[string]bool{}
	seenIDs := map[string]bool{}
	for p.pos < len(p.lines) {
		group, err := p.parseGroup()
		if err != nil {
			return nil, err
		}
		pairKey := optionalKey(group.OldPath) + "\x00" + optionalKey(group.NewPath)
		if usedPairs[pairKey] {
			return nil, malformed("file path pair occurs in more than one group")
		}
		usedPairs[pairKey] = true
		if group.OldPath != nil {
			if usedOld[*group.OldPath] {
				return nil, malformed("old path %q consumed more than once", *group.OldPath)
			}
			usedOld[*group.OldPath] = true
		}
		if group.NewPath != nil {
			if usedNew[*group.NewPath] {
				return nil, malformed("destination path %q consumed more than once", *group.NewPath)
			}
			usedNew[*group.NewPath] = true
		}
		for _, hunk := range group.Hunks {
			if seenIDs[hunk.ID] {
				return nil, malformed("duplicate derived hunk ID %s", hunk.ID)
			}
			seenIDs[hunk.ID] = true
			result.Hunks = append(result.Hunks, hunk)
		}
		result.Groups = append(result.Groups, group)
	}
	if len(result.Hunks) == 0 {
		return nil, cemcode.New(cemcode.BinaryPatch, "patch contains no textual hunks")
	}
	if len(result.Hunks) > wire.MaxHunks {
		return nil, malformed("patch exceeds %d hunks", wire.MaxHunks)
	}
	return result, nil
}

func optionalKey(path *string) string {
	if path == nil {
		return "\x00null"
	}
	return *path
}

func (p *parser) peek() []byte {
	if p.pos >= len(p.lines) {
		return nil
	}
	return p.lines[p.pos]
}

// lineText returns the current line without its trailing LF; CR is retained as
// payload except where a header grammar explicitly permits CRLF.
func lineText(line []byte) string {
	return string(bytes.TrimSuffix(line, []byte{'\n'}))
}

func hasPrefixLine(line []byte, prefix string) bool {
	return line != nil && strings.HasPrefix(lineText(line), prefix)
}

func (p *parser) parseGroup() (*Group, error) {
	line := p.peek()
	if hasPrefixLine(line, "GIT binary patch") || hasPrefixLine(line, "Binary files ") {
		return nil, cemcode.New(cemcode.BinaryPatch, "binary patches are not cem/0.1 conformant")
	}
	switch {
	case hasPrefixLine(line, "diff --git "):
		return p.parseStateMachineGroup()
	case hasPrefixLine(line, "--- "):
		return p.parseBareGroup()
	default:
		return nil, malformed("expected a diff --git or file header line")
	}
}

// parseBareGroup accepts the bare `--- a/PATH` / `+++ b/PATH` form, permitted
// only for an ordinary modification whose old and new paths are both non-null.
func (p *parser) parseBareGroup() (*Group, error) {
	oldPath, newPath, err := p.parseFileHeaders()
	if err != nil {
		return nil, err
	}
	if oldPath == nil || newPath == nil {
		return nil, metadataMismatch("creates and deletes must use the diff --git state machine")
	}
	if *oldPath != *newPath {
		return nil, malformed("bare file headers cannot declare a rename")
	}
	group := &Group{Kind: KindModify, OldPath: oldPath, NewPath: newPath}
	return group, p.parseHunks(group)
}

func (p *parser) parseStateMachineGroup() (*Group, error) {
	diffLine := lineText(p.peek())
	p.pos++
	meta, err := p.parseMetadata()
	if err != nil {
		return nil, err
	}
	if p.peek() == nil {
		return nil, malformed("diff --git group is missing file headers")
	}
	if hasPrefixLine(p.peek(), "GIT binary patch") || hasPrefixLine(p.peek(), "Binary files ") {
		return nil, cemcode.New(cemcode.BinaryPatch, "binary patches are not cem/0.1 conformant")
	}
	oldPath, newPath, err := p.parseFileHeaders()
	if err != nil {
		return nil, err
	}
	group, err := classifyGroup(diffLine, meta, oldPath, newPath)
	if err != nil {
		return nil, err
	}
	return group, p.parseHunks(group)
}

func (p *parser) parseMetadata() (*metadata, error) {
	meta := &metadata{}
	for {
		line := p.peek()
		if line == nil {
			return nil, malformed("diff --git group is missing file headers")
		}
		text := lineText(line)
		switch {
		case strings.HasPrefix(text, "index "):
			if meta.indexSeen {
				return nil, malformed("repeated index line")
			}
			mode, err := parseIndexLine(text)
			if err != nil {
				return nil, err
			}
			meta.indexSeen, meta.indexMode = true, mode
		case strings.HasPrefix(text, "new file mode "):
			if meta.newFileMode != "" || meta.deleteMode != "" {
				return nil, malformed("repeated or contradictory file mode metadata")
			}
			mode, err := requireTreeMode(strings.TrimPrefix(text, "new file mode "))
			if err != nil {
				return nil, err
			}
			meta.newFileMode = mode
		case strings.HasPrefix(text, "deleted file mode "):
			if meta.newFileMode != "" || meta.deleteMode != "" {
				return nil, malformed("repeated or contradictory file mode metadata")
			}
			mode, err := requireTreeMode(strings.TrimPrefix(text, "deleted file mode "))
			if err != nil {
				return nil, err
			}
			meta.deleteMode = mode
		case strings.HasPrefix(text, "old mode ") || strings.HasPrefix(text, "new mode "):
			return nil, malformed("old mode/new mode metadata is unsupported in cem/0.1")
		case strings.HasPrefix(text, "rename from "):
			if meta.renameFrom != "" {
				return nil, malformed("repeated rename from line")
			}
			meta.renameFrom = strings.TrimPrefix(text, "rename from ")
		case strings.HasPrefix(text, "rename to "):
			if meta.renameTo != "" {
				return nil, malformed("repeated rename to line")
			}
			meta.renameTo = strings.TrimPrefix(text, "rename to ")
		case strings.HasPrefix(text, "similarity index ") || strings.HasPrefix(text, "dissimilarity index "):
			if meta.similarity {
				return nil, malformed("repeated similarity metadata")
			}
			if err := validateSimilarity(text); err != nil {
				return nil, err
			}
			meta.similarity = true
		default:
			return meta, nil
		}
		p.pos++
	}
}

func parseIndexLine(text string) (string, error) {
	rest := strings.TrimPrefix(text, "index ")
	body, mode, hasMode := strings.Cut(rest, " ")
	oids := strings.SplitN(body, "..", 2)
	if len(oids) != 2 {
		return "", malformed("index line must carry OLD..NEW object IDs")
	}
	for _, oid := range oids {
		if len(oid) < 4 || len(oid) > 64 || !isLowerHex(oid) {
			return "", malformed("index object IDs must be 4..64 lowercase hex bytes")
		}
	}
	if !hasMode {
		return "", nil
	}
	return requireTreeMode(mode)
}

func requireTreeMode(mode string) (string, error) {
	if mode != "100644" && mode != "100755" {
		return "", cemcode.New(cemcode.UnsupportedTreeMode, "tree mode %q is not 100644 or 100755", mode)
	}
	return mode, nil
}

func validateSimilarity(text string) error {
	_, rest, _ := strings.Cut(text, "index ")
	percent, hasSuffix := strings.CutSuffix(rest, "%")
	if !hasSuffix || len(percent) == 0 || len(percent) > 3 {
		return malformed("similarity index must be 0..100 percent")
	}
	if len(percent) > 1 && percent[0] == '0' { // canonical decimal, decision 0192

		return malformed("similarity index must be 0..100 percent")
	}
	value := 0
	for _, c := range []byte(percent) {
		if c < '0' || c > '9' {
			return malformed("similarity index must be 0..100 percent")
		}
		value = value*10 + int(c-'0')
	}
	if value > 100 {
		return malformed("similarity index must be 0..100 percent")
	}
	return nil
}

// parseFileHeaders consumes `--- X` and `+++ Y`, returning nil for /dev/null.
func (p *parser) parseFileHeaders() (*string, *string, error) {
	oldLine := p.peek()
	if !hasPrefixLine(oldLine, "--- ") {
		return nil, nil, malformed("expected --- file header")
	}
	p.pos++
	newLine := p.peek()
	if !hasPrefixLine(newLine, "+++ ") {
		return nil, nil, malformed("expected +++ file header")
	}
	p.pos++
	oldPath, err := parseHeaderPath(lineText(oldLine)[4:], "a/")
	if err != nil {
		return nil, nil, err
	}
	newPath, err := parseHeaderPath(lineText(newLine)[4:], "b/")
	if err != nil {
		return nil, nil, err
	}
	if oldPath == nil && newPath == nil {
		return nil, nil, malformed("both file headers cannot be /dev/null")
	}
	return oldPath, newPath, nil
}

func parseHeaderPath(text, prefix string) (*string, error) {
	if text == "/dev/null" {
		return nil, nil
	}
	if strings.HasPrefix(text, `"`) {
		return nil, malformed("quoted or escaped Git paths are not in cem/0.1")
	}
	path, hasPrefix := strings.CutPrefix(text, prefix)
	if !hasPrefix {
		return nil, malformed("file header path must begin with %q", prefix)
	}
	if err := wire.ValidatePath(path); err != nil {
		return nil, err
	}
	return &path, nil
}

func classifyGroup(diffLine string, meta *metadata, oldPath, newPath *string) (*Group, error) {
	switch {
	case oldPath == nil: // create
		if meta.newFileMode == "" {
			return nil, metadataMismatch("a create must carry new file mode")
		}
		if meta.deleteMode != "" || meta.renameFrom != "" || meta.renameTo != "" {
			return nil, metadataMismatch("create metadata contradicts its file headers")
		}
		if meta.indexSeen && meta.indexMode != "" && meta.indexMode != meta.newFileMode {
			return nil, metadataMismatch("index mode disagrees with new file mode")
		}
		if diffLine != "diff --git a/"+*newPath+" b/"+*newPath {
			return nil, metadataMismatch("diff --git paths disagree with the create file headers")
		}
		return &Group{Kind: KindCreate, NewPath: newPath, Mode: meta.newFileMode}, nil
	case newPath == nil: // delete
		if meta.deleteMode == "" {
			return nil, metadataMismatch("a delete must carry deleted file mode")
		}
		if meta.newFileMode != "" || meta.renameFrom != "" || meta.renameTo != "" {
			return nil, metadataMismatch("delete metadata contradicts its file headers")
		}
		if meta.indexSeen && meta.indexMode != "" && meta.indexMode != meta.deleteMode {
			return nil, metadataMismatch("index mode disagrees with deleted file mode")
		}
		if diffLine != "diff --git a/"+*oldPath+" b/"+*oldPath {
			return nil, metadataMismatch("diff --git paths disagree with the delete file headers")
		}
		return &Group{Kind: KindDelete, OldPath: oldPath, Mode: meta.deleteMode}, nil
	case *oldPath != *newPath: // modified rename
		if meta.renameFrom == "" || meta.renameTo == "" {
			return nil, metadataMismatch("a rename must carry paired rename from/to metadata")
		}
		if meta.renameFrom != *oldPath || meta.renameTo != *newPath {
			return nil, metadataMismatch("rename metadata disagrees with the file headers")
		}
		if meta.newFileMode != "" || meta.deleteMode != "" {
			return nil, metadataMismatch("rename metadata contradicts its file headers")
		}
		if diffLine != "diff --git a/"+*oldPath+" b/"+*newPath {
			return nil, metadataMismatch("diff --git paths disagree with the rename file headers")
		}
		return &Group{Kind: KindRename, OldPath: oldPath, NewPath: newPath}, nil
	default: // ordinary modification
		if meta.newFileMode != "" || meta.deleteMode != "" || meta.renameFrom != "" || meta.renameTo != "" {
			return nil, metadataMismatch("modification metadata contradicts its file headers")
		}
		if diffLine != "diff --git a/"+*oldPath+" b/"+*newPath {
			return nil, metadataMismatch("diff --git paths disagree with the file headers")
		}
		return &Group{Kind: KindModify, OldPath: oldPath, NewPath: newPath}, nil
	}
}

func (p *parser) parseHunks(group *Group) error {
	if !hasPrefixLine(p.peek(), "@@ -") {
		return malformed("file group requires at least one hunk")
	}
	oldCursor := int64(0)
	delta := int64(0)
	for hasPrefixLine(p.peek(), "@@ -") {
		hunk, err := p.parseHunk(group)
		if err != nil {
			return err
		}
		if err := checkHunkPlacement(hunk, &oldCursor, &delta); err != nil {
			return err
		}
		if err := checkGroupShape(group, hunk); err != nil {
			return err
		}
		group.Hunks = append(group.Hunks, hunk)
	}
	if surplus := p.peek(); surplus != nil {
		text := lineText(surplus)
		nextHeader := strings.HasPrefix(text, "--- ") || strings.HasPrefix(text, "diff --git ")
		bodyShaped := len(text) > 0 && (text[0] == '+' || text[0] == '-' || text[0] == ' ' || text[0] == '\\')
		if bodyShaped && !nextHeader {
			return cemcode.New(cemcode.ExtraHunkPayload, "surplus payload after the declared hunk ranges")
		}
	}
	return nil
}

// checkHunkPlacement rejects overlapping or out-of-order old ranges and
// inconsistent new positions within one file group.
func checkHunkPlacement(hunk *Hunk, oldCursor, delta *int64) error {
	oldStart, oldCount := hunk.OldRange.Start, hunk.OldRange.Count
	newStart, newCount := hunk.NewRange.Start, hunk.NewRange.Count
	effectiveStart := oldStart
	if oldCount == 0 {
		effectiveStart = oldStart + 1 // insertion point sits after line oldStart
	}
	if effectiveStart <= *oldCursor {
		return malformed("hunk old ranges overlap or are out of order")
	}
	expectedNew := oldStart + *delta
	switch {
	case oldCount == 0:
		expectedNew = oldStart + *delta + 1
	case newCount == 0:
		expectedNew = oldStart + *delta - 1
	}
	if newStart != expectedNew {
		return malformed("hunk new position is inconsistent with prior hunks")
	}
	*oldCursor = effectiveStart + oldCount - 1
	*delta += newCount - oldCount
	return nil
}

func checkGroupShape(group *Group, hunk *Hunk) error {
	switch group.Kind {
	case KindCreate:
		if hunk.OldRange.Start != 0 || hunk.OldRange.Count != 0 {
			return malformed("every create hunk carries an empty old range")
		}
	case KindDelete:
		if hunk.NewRange.Start != 0 || hunk.NewRange.Count != 0 {
			return malformed("every delete hunk carries an empty new range")
		}
	}
	return nil
}

func (p *parser) parseHunk(group *Group) (*Hunk, error) {
	header := lineText(p.peek())
	oldRange, newRange, err := parseHunkHeader(header)
	if err != nil {
		return nil, err
	}
	p.pos++
	hunk := &Hunk{OldPath: group.OldPath, NewPath: group.NewPath, OldRange: oldRange, NewRange: newRange}
	hunk.DisplayPath = displayPath(group)
	if err := p.parseHunkBody(hunk); err != nil {
		return nil, err
	}
	digest := sha256.Sum256(hunk.RawBody)
	hunk.ContentSha256 = hex.EncodeToString(digest[:])
	hunk.ID = wire.HunkIdentity(hunk.ContentSha256, hunk.OldPath, hunk.NewPath,
		hunk.OldRange, hunk.NewRange)
	return hunk, nil
}

func displayPath(group *Group) string {
	if group.Kind == KindDelete {
		return *group.OldPath
	}
	return *group.NewPath
}

// parseHunkHeader reads `@@ -START[,COUNT] +START[,COUNT] @@` with an optional
// uninterpreted suffix and optional CR before LF.
func parseHunkHeader(text string) (wire.Range, wire.Range, error) {
	fail := func() (wire.Range, wire.Range, error) {
		return wire.Range{}, wire.Range{}, malformed("invalid hunk header")
	}
	rest, hasPrefix := strings.CutPrefix(text, "@@ -")
	if !hasPrefix {
		return fail()
	}
	oldPart, rest, found := strings.Cut(rest, " +")
	if !found {
		return fail()
	}
	newPart, _, found := strings.Cut(rest, " @@") // the suffix is uninterpreted
	if !found {
		return fail()
	}
	oldRange, err := parseRangePart(oldPart)
	if err != nil {
		return fail()
	}
	newRange, err := parseRangePart(newPart)
	if err != nil {
		return fail()
	}
	return oldRange, newRange, nil
}

func parseRangePart(text string) (wire.Range, error) {
	startText, countText, hasCount := strings.Cut(text, ",")
	start, ok := parseWireInteger(startText)
	if !ok {
		return wire.Range{}, malformed("invalid hunk range")
	}
	count := int64(1)
	if hasCount {
		if count, ok = parseWireInteger(countText); !ok {
			return wire.Range{}, malformed("invalid hunk range")
		}
	}
	if count == 0 {
		if start > wire.MaxWireInteger {
			return wire.Range{}, malformed("invalid hunk range")
		}
	} else if start < 1 {
		return wire.Range{}, malformed("nonzero hunk counts require a one-based start")
	}
	if start+count > wire.MaxWireInteger {
		return wire.Range{}, malformed("hunk range exceeds the wire integer bound")
	}
	return wire.Range{Start: start, Count: count}, nil
}

func parseWireInteger(text string) (int64, bool) {
	if len(text) == 0 || len(text) > 16 {
		return 0, false
	}
	if len(text) > 1 && text[0] == '0' {
		return 0, false
	}
	value := int64(0)
	for _, c := range []byte(text) {
		if c < '0' || c > '9' {
			return 0, false
		}
		value = value*10 + int64(c-'0')
	}
	if value > wire.MaxWireInteger {
		return 0, false
	}
	return value, true
}

const noNewlineMarker = `\ No newline at end of file`

func (p *parser) parseHunkBody(hunk *Hunk) error {
	oldRemaining := hunk.OldRange.Count
	newRemaining := hunk.NewRange.Count
	changed := false
	var raw bytes.Buffer
	for oldRemaining > 0 || newRemaining > 0 {
		line := p.peek()
		if line == nil {
			return malformed("hunk body is shorter than its declared ranges")
		}
		if len(line) == 0 {
			return malformed("empty hunk body record")
		}
		if line[len(line)-1] != '\n' { // only the marker removes an LF, decision 0192
			return malformed("hunk body record must end in LF")
		}
		prefix := line[0]
		payload := line[1:]
		switch prefix {
		case ' ':
			if oldRemaining <= 0 || newRemaining <= 0 {
				return malformed("hunk body is longer than its declared ranges")
			}
			oldRemaining--
			newRemaining--
		case '-':
			if oldRemaining <= 0 {
				return malformed("hunk body is longer than its declared ranges")
			}
			oldRemaining--
		case '+':
			if newRemaining <= 0 {
				return malformed("hunk body is longer than its declared ranges")
			}
			newRemaining--
		default:
			return malformed("invalid hunk body prefix")
		}
		raw.Write(line)
		hunk.Body = append(hunk.Body, BodyLine{Prefix: prefix, Payload: payload})
		changed = changed || prefix != ' '
		p.pos++
		if err := p.consumeNoNewlineMarker(hunk, &raw); err != nil {
			return err
		}
	}
	if !changed {
		return malformed("a hunk requires at least one changed line")
	}
	hunk.RawBody = raw.Bytes()
	return nil
}

func (p *parser) consumeNoNewlineMarker(hunk *Hunk, raw *bytes.Buffer) error {
	line := p.peek()
	if string(line) != noNewlineMarker+"\n" {
		return nil
	}
	last := &hunk.Body[len(hunk.Body)-1]
	if len(last.Payload) == 0 || last.Payload[len(last.Payload)-1] != '\n' {
		return malformed("no-newline marker must follow a body line ending in LF")
	}
	if last.StripLFOld || last.StripLFNew {
		return malformed("repeated no-newline marker")
	}
	switch last.Prefix {
	case '-':
		last.StripLFOld = true
	case '+':
		last.StripLFNew = true
	default:
		last.StripLFOld, last.StripLFNew = true, true
	}
	raw.Write(line)
	p.pos++
	return nil
}

// OldPayload returns one body line's old-side reconstructed bytes.
func (line BodyLine) OldPayload() []byte { return line.sidePayload(line.StripLFOld) }

// NewPayload returns one body line's new-side reconstructed bytes.
func (line BodyLine) NewPayload() []byte { return line.sidePayload(line.StripLFNew) }

func (line BodyLine) sidePayload(strip bool) []byte {
	if strip {
		return bytes.TrimSuffix(line.Payload, []byte{'\n'})
	}
	return line.Payload
}

func isLowerHex(text string) bool {
	if len(text) == 0 {
		return false
	}
	for _, c := range []byte(text) {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
