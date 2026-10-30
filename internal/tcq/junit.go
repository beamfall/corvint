package tcq

import (
	"bytes"
	"encoding/xml"
	"io"
	"sort"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// junitRow is one projected testcase. TCQ-V0-030 stores exactly the execution
// key, the row ID, and the status: raw names, classnames, messages, stdout, and
// stderr never reach identity or output.
type junitRow struct {
	executionKey string
	id           string
	status       string
	keyed        bool
}

type junitReport struct {
	sha256        string
	byteLength    int
	keyed         []junitRow
	unkeyedCount  int
	hasNonPassing bool
}

// xmlDeclarations are the only leading declarations TCQ-V0-026 may remove. Any
// other `<?` sequence anywhere in the payload invalidates the report.
var xmlDeclarations = [][]byte{
	[]byte(`<?xml version="1.0"?>`),
	[]byte(`<?xml version='1.0'?>`),
	[]byte(`<?xml version="1.0" encoding="UTF-8"?>`),
	[]byte(`<?xml version="1.0" encoding="utf-8"?>`),
	[]byte(`<?xml version='1.0' encoding='UTF-8'?>`),
	[]byte(`<?xml version='1.0' encoding='utf-8'?>`),
}

// junitPayload is the TCQ-V0-026 byte preflight. It runs on the private report
// copy BEFORE any parser is constructed, so DTD, entity, and processing
// instruction surfaces are refused as bytes rather than disabled after the fact.
func junitPayload(raw []byte) ([]byte, error) {
	if len(raw) > maxReportBytes {
		return nil, fail(CodeResourceExhausted)
	}
	payload := bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	for _, declaration := range xmlDeclarations {
		if bytes.HasPrefix(payload, declaration) {
			payload = payload[len(declaration):]
			break
		}
	}
	forbidden := [][]byte{[]byte("<?"), []byte("<!DOCTYPE"), []byte("<!ENTITY")}
	for _, marker := range forbidden {
		if bytes.Contains(payload, marker) {
			return nil, fail(CodeInvalidJUnit)
		}
	}
	if !utf8.Valid(payload) {
		return nil, fail(CodeInvalidJUnit)
	}
	if !attributeCountsBounded(payload) {
		return nil, fail(CodeResourceExhausted)
	}
	return payload, nil
}

// unparsedSections are the markup forms whose bytes are not start-tag syntax.
var unparsedSections = [][2][]byte{
	{[]byte("!--"), []byte("-->")},
	{[]byte("![CDATA["), []byte("]]>")},
}

// attributeCountsBounded bounds the TCQ-V0-041 attribute count on the bytes.
// Go's decoder allocates every attribute of a start tag before returning it, so
// counting afterwards would let one 4 MiB tag allocate hundreds of MiB. Each
// `=` outside a quoted value inside a tag introduces one attribute. Malformed
// markup is left for the strict decoder to refuse.
func attributeCountsBounded(payload []byte) bool {
	rest := payload
	for {
		open := bytes.IndexByte(rest, '<')
		if open < 0 {
			return true
		}
		rest = rest[open+1:]
		if tail, skipped := skipUnparsedSection(rest); skipped {
			rest = tail
			continue
		}
		count, tail := tagAttributeCount(rest)
		if count > maxXMLAttributes {
			return false
		}
		rest = tail
	}
}

func skipUnparsedSection(rest []byte) ([]byte, bool) {
	for _, section := range unparsedSections {
		if !bytes.HasPrefix(rest, section[0]) {
			continue
		}
		end := bytes.Index(rest, section[1])
		if end < 0 {
			return nil, true
		}
		return rest[end+len(section[1]):], true
	}
	return rest, false
}

func tagAttributeCount(rest []byte) (int, []byte) {
	count := 0
	var quote byte
	for index, current := range rest {
		switch {
		case quote != 0:
			if current == quote {
				quote = 0
			}
		case current == '"' || current == '\'':
			quote = current
		case current == '=':
			count++
		case current == '>':
			return count, rest[index+1:]
		}
	}
	return count, nil
}

// junitChildren is the complete closed element grammar of TCQ-V0-027. Any
// unknown element or descendant — including XInclude or a nested `testcase` —
// invalidates the report.
var junitChildren = map[string]map[string]bool{
	"testsuites": {"testsuite": true},
	"testsuite":  {"properties": true, "testsuite": true, "testcase": true, "system-out": true, "system-err": true},
	"testcase":   {"properties": true, "failure": true, "error": true, "skipped": true, "system-out": true, "system-err": true},
	"properties": {"property": true},
	"property":   {},
	"failure":    {},
	"error":      {},
	"skipped":    {},
	"system-out": {},
	"system-err": {},
}

var junitRoots = map[string]bool{"testsuite": true, "testsuites": true}

// parseJUnit performs the single strict traversal of TCQ-V0-027: depth,
// element, attribute, and testcase counters are enforced during the walk, never
// after building an unbounded tree.
func parseJUnit(repository Repository, target string, raw []byte) (junitReport, error) {
	payload, err := junitPayload(raw)
	if err != nil {
		return junitReport{}, err
	}
	walk := &junitWalk{repository: repository, target: target, reportSHA: sha256Hex(raw), pathKeyed: map[string]bool{}}
	if err := walk.run(payload); err != nil {
		return junitReport{}, err
	}
	return walk.report(raw), nil
}

type openCase struct {
	ordinal   int
	attrs     map[string]string
	terminals []string
}

type junitWalk struct {
	repository   Repository
	target       string
	reportSHA    string
	stack        []string
	cases        []openCase
	rows         []junitRow
	elementCount int
	caseCount    int
	pathKeyed    map[string]bool
}

func (walk *junitWalk) run(payload []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	decoder.Strict = true
	// Go's decoder resolves no external entity, DTD, or XInclude, and the byte
	// preflight already refused every declaration form, so the parser has no
	// capability left to disable (TCQ-V0-026).
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fail(CodeInvalidJUnit)
		}
		if err := walk.token(token); err != nil {
			return err
		}
	}
	if len(walk.stack) > 0 || walk.elementCount == 0 {
		return fail(CodeInvalidJUnit)
	}
	return nil
}

func (walk *junitWalk) token(token xml.Token) error {
	switch typed := token.(type) {
	case xml.StartElement:
		return walk.start(typed)
	case xml.EndElement:
		return walk.end(typed)
	case xml.ProcInst, xml.Directive:
		return fail(CodeInvalidJUnit)
	case xml.CharData:
		return walk.text(typed)
	}
	return nil
}

// text refuses character data outside the root: Go's decoder admits it, but a
// well-formed document has only whitespace around its one root (TCQ-V0-027).
func (walk *junitWalk) text(data xml.CharData) error {
	if len(walk.stack) > 0 {
		return nil
	}
	if len(bytes.Trim(data, " \t\r\n")) > 0 {
		return fail(CodeInvalidJUnit)
	}
	return nil
}

func (walk *junitWalk) start(element xml.StartElement) error {
	name := element.Name.Local
	// TCQ-V0-027: raw JUnit has no namespace.
	if element.Name.Space != "" {
		return fail(CodeInvalidJUnit)
	}
	if _, known := junitChildren[name]; !known {
		return fail(CodeInvalidJUnit)
	}
	if err := walk.countElement(element); err != nil {
		return err
	}
	if len(walk.stack) == 0 {
		// Go's decoder admits a second top-level element; XML allows one root.
		if walk.elementCount > 1 {
			return fail(CodeInvalidJUnit)
		}
		if !junitRoots[name] {
			return fail(CodeInvalidJUnit)
		}
	} else if !junitChildren[walk.stack[len(walk.stack)-1]][name] {
		return fail(CodeInvalidJUnit)
	}
	walk.stack = append(walk.stack, name)
	return walk.openTerminalOrCase(name, element)
}

func (walk *junitWalk) countElement(element xml.StartElement) error {
	walk.elementCount++
	if walk.elementCount > maxXMLElements || len(walk.stack)+1 > maxXMLDepth {
		return fail(CodeResourceExhausted)
	}
	if len(element.Attr) > maxXMLAttributes {
		return fail(CodeResourceExhausted)
	}
	for _, attribute := range element.Attr {
		if len(attribute.Name.Local) > maxXMLAttributeBytes || len(attribute.Value) > maxXMLAttributeBytes {
			return fail(CodeResourceExhausted)
		}
	}
	// Go's decoder admits a repeated attribute, which XML forbids; a strict
	// traversal refuses it rather than let the last spelling pick the status.
	seen := make(map[xml.Name]bool, len(element.Attr))
	for _, attribute := range element.Attr {
		if seen[attribute.Name] {
			return fail(CodeInvalidJUnit)
		}
		seen[attribute.Name] = true
	}
	return nil
}

func (walk *junitWalk) openTerminalOrCase(name string, element xml.StartElement) error {
	if name == "testcase" {
		walk.caseCount++
		if walk.caseCount > maxTestcases {
			return fail(CodeResourceExhausted)
		}
		walk.cases = append(walk.cases, openCase{ordinal: walk.caseCount - 1, attrs: attributeMap(element)})
		return nil
	}
	terminal := name == "failure" || name == "error" || name == "skipped"
	inCase := len(walk.stack) >= 2 && walk.stack[len(walk.stack)-2] == "testcase"
	if terminal && inCase && len(walk.cases) > 0 {
		last := len(walk.cases) - 1
		walk.cases[last].terminals = append(walk.cases[last].terminals, name)
	}
	return nil
}

func (walk *junitWalk) end(element xml.EndElement) error {
	name := element.Name.Local
	if len(walk.stack) == 0 || walk.stack[len(walk.stack)-1] != name {
		return fail(CodeInvalidJUnit)
	}
	walk.stack = walk.stack[:len(walk.stack)-1]
	if name != "testcase" {
		return nil
	}
	closed := walk.cases[len(walk.cases)-1]
	walk.cases = walk.cases[:len(walk.cases)-1]
	status, err := testcaseStatus(closed.attrs, closed.terminals)
	if err != nil {
		return err
	}
	return walk.project(closed, status)
}

// project derives the row identity of one closed testcase (TCQ-V0-028/030). A
// row is keyed only when its `file` resolves to a tracked target-tree blob in a
// supported mode; everything else retains only a count.
func (walk *junitWalk) project(closed openCase, status string) error {
	path, name, keyable := rowIdentityInputs(closed.attrs)
	keyed := false
	if keyable {
		tracked, err := walk.trackedBlob(path)
		if err != nil {
			return err
		}
		keyed = tracked
	}
	if !keyed {
		walk.rows = append(walk.rows, junitRow{status: status})
		return nil
	}
	key := executionKey(path, name)
	preimage := jsonObject(
		member{"executionKeySha256", jsonString(key)},
		member{"ordinal", jsonInt(int64(closed.ordinal))},
		member{"reportSha256", jsonString(walk.reportSHA)},
		member{"status", jsonString(status)},
	)
	walk.rows = append(walk.rows, junitRow{
		executionKey: key,
		id:           rowPrefix + domainHash(domainRow, canonicalValue(preimage)),
		status:       status,
		keyed:        true,
	})
	return nil
}

func rowIdentityInputs(attrs map[string]string) (string, string, bool) {
	name, hasName := attrs["name"]
	if !hasName || name == "" || len(name) > 1024 || hasControl(name) || !utf8.ValidString(name) {
		return "", "", false
	}
	// TCQ-V0-028 bounds `file` at 1,024 UTF-8 bytes, like `name`.
	path, ok := normalPath(attrs["file"], 1024)
	if !ok {
		return "", "", false
	}
	return path, name, true
}

func (walk *junitWalk) trackedBlob(path string) (bool, error) {
	if cached, ok := walk.pathKeyed[path]; ok {
		return cached, nil
	}
	entry, err := walk.repository.TreeEntry(walk.target, path)
	if err != nil {
		return false, err // a Git lookup failure is not an absent path (TCQ-V0-028)
	}
	tracked := entry.Found && entry.Type == "blob" && blobModes[entry.Mode]
	walk.pathKeyed[path] = tracked
	return tracked, nil
}

func (walk *junitWalk) report(raw []byte) junitReport {
	report := junitReport{sha256: walk.reportSHA, byteLength: len(raw)}
	for _, row := range walk.rows {
		if row.keyed {
			report.keyed = append(report.keyed, row)
		} else {
			report.unkeyedCount++
		}
		if row.status == "FAILED" || row.status == "ERROR" {
			report.hasNonPassing = true
		}
	}
	return report
}

// attributeMap keeps only unprefixed attributes: raw JUnit has no namespace, so
// a prefixed `x:status` is an unknown attribute, never the raw `status`.
func attributeMap(element xml.StartElement) map[string]string {
	attrs := make(map[string]string, len(element.Attr))
	for _, attribute := range element.Attr {
		if attribute.Name.Space == "" {
			attrs[attribute.Name.Local] = attribute.Value
		}
	}
	return attrs
}

// testcaseStatus is the complete, case-sensitive table of TCQ-V0-029. Any other
// combination, a repeated terminal child, or multiple terminal kinds invalidates
// the report — a malformed status is never coerced to a passing row.
func testcaseStatus(attrs map[string]string, terminals []string) (string, error) {
	if len(terminals) > 1 {
		return "", fail(CodeInvalidJUnit)
	}
	status, hasStatus := attrs["status"]
	disabled, hasDisabled := attrs["disabled"]
	statusIn := func(values ...string) bool { return optionalIn(status, hasStatus, values) }
	disabledIn := func(values ...string) bool { return optionalIn(disabled, hasDisabled, values) }
	if len(terminals) == 0 {
		switch {
		case statusIn("", "run", "passed") && disabledIn("", "false"):
			return "PASSED", nil
		case statusIn("", "notrun", "disabled", "skipped") && disabled == "true":
			return "SKIPPED", nil
		case statusIn("notrun", "disabled", "skipped") && disabledIn("", "false"):
			return "SKIPPED", nil
		}
		return "", fail(CodeInvalidJUnit)
	}
	switch terminals[0] {
	case "failure":
		if statusIn("", "failed") && disabledIn("", "false") {
			return "FAILED", nil
		}
	case "error":
		if statusIn("", "error") && disabledIn("", "false") {
			return "ERROR", nil
		}
	case "skipped":
		if statusIn("", "notrun", "disabled", "skipped") && disabledIn("", "false", "true") {
			return "SKIPPED", nil
		}
	}
	return "", fail(CodeInvalidJUnit)
}

// optionalIn treats "" in the allowed set as "attribute absent", which is how
// the TCQ-V0-029 table distinguishes an absent attribute from an empty one.
func optionalIn(value string, present bool, allowed []string) bool {
	for _, candidate := range allowed {
		if candidate == "" && !present {
			return true
		}
		if candidate != "" && present && candidate == value {
			return true
		}
	}
	return false
}

// observationRows is the TCQ-V0-031 row projection: sorted by
// (executionKeySha256, id) and strictly unique.
func observationRows(report junitReport) []wire.Value {
	rows := append([]junitRow(nil), report.keyed...)
	sort.Slice(rows, func(left, right int) bool {
		if rows[left].executionKey != rows[right].executionKey {
			return rows[left].executionKey < rows[right].executionKey
		}
		return rows[left].id < rows[right].id
	})
	values := make([]wire.Value, 0, len(rows))
	for _, row := range rows {
		values = append(values, jsonObject(
			member{"executionKeySha256", jsonString(row.executionKey)},
			member{"id", jsonString(row.id)},
			member{"status", jsonString(row.status)},
		))
	}
	return values
}

// matchingRows returns the keyed rows whose execution key equals key.
func matchingRows(report junitReport, key string) []junitRow {
	var matches []junitRow
	for _, row := range report.keyed {
		if row.executionKey == key {
			matches = append(matches, row)
		}
	}
	return matches
}
