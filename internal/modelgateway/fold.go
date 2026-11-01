// Package modelgateway is the experimental loopback model gateway described by
// docs/specs/local-model-gateway-v0.md: a passthrough proxy for the Anthropic
// Messages API that records per-request token usage and folds oversized shell
// output deterministically.
package modelgateway

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// FoldOptions bound one Folder. Zero values take the LMG-V0-006/007 defaults.
type FoldOptions struct {
	Threshold int    // fold text longer than this many bytes
	Head      int    // bytes kept from the start once noise is dropped
	Tail      int    // bytes kept from the end
	Archive   string // directory holding each folded original as <sha256>.txt
}

// Folder rewrites oversized Bash tool results in a Messages API request.
type Folder struct {
	opts FoldOptions
	// write stores one archive file; tests replace it to simulate failure.
	write func(path string, data []byte) error
}

// NewFolder returns a Folder with defaults applied.
func NewFolder(opts FoldOptions) *Folder {
	if opts.Threshold <= 0 {
		opts.Threshold = 8192
	}
	if opts.Head <= 0 {
		opts.Head = 4096
	}
	if opts.Tail <= 0 {
		opts.Tail = 2048
	}
	return &Folder{opts: opts, write: writeArchive}
}

// FoldRequest returns the body to forward and how many results it folded. A
// body it cannot read, or one with nothing to fold, comes back unchanged
// (LMG-V0-009), so the forwarded bytes equal the received bytes. A fold
// replaces only the bytes of each folded JSON string; every other byte,
// including key order and number spelling, is forwarded as received.
func (f *Folder) FoldRequest(body []byte) ([]byte, int) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	root, err := parseNode(decoder, body, 0)
	if err != nil {
		return body, 0
	}
	if _, err := decoder.Token(); err != io.EOF {
		return body, 0
	}
	bash := map[string]bool{}
	var splices []splice
	for _, message := range root.field("messages").items() {
		for _, block := range message.field("content").items() {
			switch block.field("type").text() {
			case "tool_use":
				if id := block.field("id"); id.isString() && block.field("name").text() == "Bash" {
					bash[id.str] = true
				}
			case "tool_result":
				if !bash[block.field("tool_use_id").text()] || block.field("is_error").value() == true {
					continue
				}
				splices = f.foldContent(block.field("content"), splices)
			}
		}
	}
	if len(splices) == 0 {
		return body, 0
	}
	out := make([]byte, 0, len(body))
	at := int64(0)
	for _, s := range splices {
		out = append(append(out, body[at:s.start]...), s.text...)
		at = s.end
	}
	return append(out, body[at:]...), len(splices)
}

// foldContent folds a tool_result's string content or each of its text blocks.
func (f *Folder) foldContent(content *node, splices []splice) []splice {
	if content.isString() {
		return f.foldString(content, splices)
	}
	for _, part := range content.items() {
		if part.field("type").text() == "text" {
			splices = f.foldString(part.field("text"), splices)
		}
	}
	return splices
}

func (f *Folder) foldString(n *node, splices []splice) []splice {
	if !n.isString() {
		return splices
	}
	folded, ok := f.Fold(n.str)
	if !ok {
		return splices
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if encoder.Encode(folded) != nil {
		return splices
	}
	return append(splices, splice{start: n.start, end: n.end, text: bytes.TrimSuffix(encoded.Bytes(), []byte("\n"))})
}

// splice replaces body[start:end] with text.
type splice struct {
	start, end int64
	text       []byte
}

// maxDepth matches encoding/json's nesting limit.
const maxDepth = 10000

// node is one JSON value; a string also carries its byte span in the body.
type node struct {
	kind       json.Delim // '{', '[', or 0 for a scalar
	scalar     any
	str        string
	start, end int64
	fields     map[string]*node
	elems      []*node
}

func parseNode(d *json.Decoder, body []byte, depth int) (*node, error) {
	if depth > maxDepth {
		return nil, errors.New("nesting too deep")
	}
	before := d.InputOffset()
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	n := &node{scalar: token}
	switch token := token.(type) {
	case string:
		n.str, n.end = token, d.InputOffset()
		n.start = before + int64(bytes.IndexByte(body[before:n.end], '"'))
	case json.Delim:
		n.kind, n.scalar = token, nil
		if token == '{' {
			n.fields = map[string]*node{}
		}
		for d.More() {
			key := ""
			if token == '{' {
				k, err := d.Token()
				if err != nil {
					return nil, err
				}
				key, _ = k.(string)
			}
			child, err := parseNode(d, body, depth+1)
			if err != nil {
				return nil, err
			}
			if token == '{' {
				n.fields[key] = child
			} else {
				n.elems = append(n.elems, child)
			}
		}
		if _, err := d.Token(); err != nil {
			return nil, err
		}
	}
	return n, nil
}

func (n *node) field(key string) *node {
	if n == nil {
		return nil
	}
	return n.fields[key]
}

func (n *node) items() []*node {
	if n == nil {
		return nil
	}
	return n.elems
}

func (n *node) isString() bool {
	_, ok := n.value().(string)
	return ok
}

func (n *node) text() string {
	if n.isString() {
		return n.str
	}
	return ""
}

func (n *node) value() any {
	if n == nil {
		return nil
	}
	return n.scalar
}

// Fold returns the folded form of one text and true, or false when the text is
// within the threshold or its archive copy could not be written (LMG-V0-008).
// The result depends only on text and the options (LMG-V0-005).
func (f *Folder) Fold(text string) (string, bool) {
	if len(text) <= f.opts.Threshold {
		return "", false
	}
	sum := sha256.Sum256([]byte(text))
	path := filepath.Join(f.opts.Archive, hex.EncodeToString(sum[:])+".txt")
	if f.opts.Archive == "" || f.write(path, []byte(text)) != nil {
		return "", false
	}
	var kept strings.Builder
	lines, dropped := 0, 0
	for rest := text; rest != ""; {
		line := rest
		if i := strings.IndexByte(rest, '\n'); i >= 0 {
			line, rest = rest[:i+1], rest[i+1:]
		} else {
			rest = ""
		}
		lines++
		if goTestNoise(line) {
			dropped++
		} else {
			kept.WriteString(line)
		}
	}
	body := kept.String()
	omittedLines, omittedBytes := 0, 0
	if len(body) > f.opts.Threshold {
		head := cutAfterLine(body, f.opts.Head)
		tail := cutBeforeLine(body, f.opts.Tail)
		if len(head)+len(tail) < len(body) {
			middle := body[len(head) : len(body)-len(tail)]
			omittedLines, omittedBytes = strings.Count(middle, "\n"), len(middle)
			body = head + fmt.Sprintf("[corvint-gateway: … %d lines omitted …]\n", omittedLines) + tail
		}
	}
	marker := fmt.Sprintf("[corvint-gateway: folded %d bytes; dropped %d passing-test lines; omitted %d lines (%d bytes); full output (%d lines): %s]\n",
		len(text), dropped, omittedLines, omittedBytes, lines, path)
	return marker + body, true
}

// goTestNoise reports a line that only says a Go test or package passed.
func goTestNoise(line string) bool {
	trimmed := strings.TrimRight(line, "\r\n")
	body := strings.TrimLeft(trimmed, " \t")
	switch {
	case trimmed == "PASS":
		return true
	case strings.HasPrefix(trimmed, "=== RUN ") || strings.HasPrefix(trimmed, "=== PAUSE ") || strings.HasPrefix(trimmed, "=== CONT "):
		return true
	case strings.HasPrefix(body, "--- PASS: "):
		return true
	case strings.HasPrefix(trimmed, "ok  \t") || strings.HasPrefix(trimmed, "ok   "):
		return true
	case strings.HasPrefix(trimmed, "?   \t") && strings.HasSuffix(trimmed, "[no test files]"):
		return true
	}
	return false
}

// cutAfterLine returns the longest prefix of whole lines within limit bytes,
// or the longest whole-rune prefix within limit when the first line alone is
// longer.
func cutAfterLine(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	if i := strings.LastIndexByte(text[:limit], '\n'); i >= 0 {
		return text[:i+1]
	}
	for limit > 0 && !utf8.RuneStart(text[limit]) {
		limit--
	}
	return text[:limit]
}

// cutBeforeLine returns the longest suffix of whole lines within limit bytes,
// or the longest whole-rune suffix within limit when the last line alone is
// longer.
func cutBeforeLine(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	start := len(text) - limit
	if i := strings.IndexByte(text[start:], '\n'); i >= 0 && start+i+1 < len(text) {
		return text[start+i+1:]
	}
	for start < len(text) && !utf8.RuneStart(text[start]) {
		start++
	}
	return text[start:]
}

// writeArchive stores data at path once: an existing private regular file with
// exactly this content is left alone; anything else at path is replaced through
// a temporary file and rename, and a path that cannot be replaced is an error.
func writeArchive(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() && info.Mode().Perm() == 0o600 && info.Size() == int64(len(data)) {
		if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
			return nil
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".fold-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
