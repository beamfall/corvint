package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	maxWireInteger     = uint64(9007199254740991)
	maxJSONBytes       = 4 << 20
	maxPatchBytes      = 8 << 20
	maxBlobBytes       = 64 << 20
	maxTotalBlobs      = 128 << 20
	maxRecords         = 262144
	maxEvidence        = 4096
	maxHunks           = 2048
	maxBases           = 32
	maxGitOps          = 1024
	verificationBudget = 30 * time.Minute
)

type cemError struct {
	code, detail, stableCode string // detail and stableCode: see metadataOnlyRefusal
	operational              bool
}

func (e *cemError) Error() string { return e.code }

func invalid(code string) *cemError     { return &cemError{code: code} }
func operational(code string) *cemError { return &cemError{code: code, operational: true} }

type span struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
}

func (s *span) UnmarshalJSON(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || !hasKeys(fields, "start", "end") {
		return errors.New("span shape")
	}
	start, err := parseWireUint(fields["start"])
	if err != nil {
		return err
	}
	end, err := parseWireUint(fields["end"])
	if err != nil {
		return err
	}
	s.Start, s.End = start, end
	return nil
}

type lineRange struct {
	Start uint64 `json:"start"`
	Count uint64 `json:"count"`
}

func (r *lineRange) UnmarshalJSON(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || !hasKeys(fields, "start", "count") {
		return errors.New("range shape")
	}
	start, err := parseWireUint(fields["start"])
	if err != nil {
		return err
	}
	count, err := parseWireUint(fields["count"])
	if err != nil {
		return err
	}
	r.Start, r.Count = start, count
	return nil
}

type evidence struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	BlobOID    string `json:"blobOid"`
	Span       span   `json:"span"`
	SpanSHA256 string `json:"spanSha256"`
	data       []byte
}

type basis struct {
	EvidenceID string `json:"evidenceId"`
	Relation   string `json:"relation"`
}

type mappedHunk struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	OldRange    lineRange `json:"oldRange"`
	NewRange    lineRange `json:"newRange"`
	Disposition string    `json:"disposition"`
	Reason      string    `json:"reason"`
	Basis       []basis   `json:"basis"`
}

type cemMap struct {
	Spec         string       `json:"spec"`
	BaseRevision string       `json:"baseRevision"`
	PatchSHA256  string       `json:"patchSha256"`
	Evidence     []evidence   `json:"evidence"`
	Hunks        []mappedHunk `json:"hunks"`
}

type nullableSpan struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
}

type driftItem struct {
	EvidenceID    string        `json:"evidenceId"`
	Path          string        `json:"path"`
	Status        string        `json:"status"`
	TargetBlobOID *string       `json:"targetBlobOid"`
	TargetSpan    *nullableSpan `json:"targetSpan"`
}

type verifier struct {
	ctx       context.Context
	repo      string
	gitOps    int
	blobBytes int64
	blobs     map[string][]byte
	trees     map[string]*treeEntry
}

var errSizeLimit = errors.New("size limit")

var errNotRegular = errors.New("not a stable regular file")

func readBounded(ctx context.Context, path string, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, errNotRegular
	}
	f, err := openInputNonblocking(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return nil, errNotRegular
	}
	if after.Size() > limit {
		return nil, errSizeLimit
	}
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		b, readErr := io.ReadAll(io.LimitReader(f, limit+1))
		if readErr == nil && int64(len(b)) > limit {
			readErr = errSizeLimit
		}
		done <- result{data: b, err: readErr}
	}()
	select {
	case got := <-done:
		return got.data, got.err
	case <-ctx.Done():
		_ = f.Close()
		return nil, ctx.Err()
	}
}

func validateEvidenceHunkShape(root map[string]json.RawMessage) error {
	var evidenceItems []map[string]json.RawMessage
	if err := json.Unmarshal(root["evidence"], &evidenceItems); err != nil || evidenceItems == nil {
		return errors.New("evidence shape")
	}
	for _, item := range evidenceItems {
		if !hasKeys(item, "id", "path", "blobOid", "span", "spanSha256") {
			return errors.New("missing evidence field")
		}
		var s map[string]json.RawMessage
		if json.Unmarshal(item["span"], &s) != nil || !hasKeys(s, "start", "end") {
			return errors.New("missing span field")
		}
	}
	var hunks []map[string]json.RawMessage
	if err := json.Unmarshal(root["hunks"], &hunks); err != nil || hunks == nil {
		return errors.New("hunks shape")
	}
	for _, item := range hunks {
		if !hasKeys(item, "id", "path", "oldRange", "newRange", "disposition", "reason", "basis") {
			return errors.New("missing hunk field")
		}
		for _, key := range []string{"oldRange", "newRange"} {
			var r map[string]json.RawMessage
			if json.Unmarshal(item[key], &r) != nil || !hasKeys(r, "start", "count") {
				return errors.New("missing range field")
			}
		}
		var bases []map[string]json.RawMessage
		if err := json.Unmarshal(item["basis"], &bases); err != nil || bases == nil {
			return errors.New("basis shape")
		}
		for _, b := range bases {
			if !hasKeys(b, "evidenceId", "relation") {
				return errors.New("missing basis field")
			}
		}
	}
	return nil
}

func hasKeys(m map[string]json.RawMessage, keys ...string) bool {
	if m == nil || len(m) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := m[key]; !ok {
			return false
		}
	}
	return true
}

// strictJSON admits exactly one well-formed JSON value in valid UTF-8 with no duplicate member.
func strictJSON(raw []byte) error {
	if !utf8.Valid(raw) || hasUnpairedJSONSurrogate(raw) {
		return errors.New("utf8")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := scanJSONValue(dec); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing")
	}
	return nil
}

func scanJSONValue(dec *json.Decoder) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	switch x := t.(type) {
	case json.Delim:
		switch x {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				k, err := dec.Token()
				if err != nil {
					return err
				}
				ks, ok := k.(string)
				if !ok || seen[ks] {
					return errors.New("duplicate")
				}
				seen[ks] = true
				if err := scanJSONValue(dec); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim('}') {
				return errors.New("object")
			}
		case '[':
			for dec.More() {
				if err := scanJSONValue(dec); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim(']') {
				return errors.New("array")
			}
		default:
			return errors.New("delimiter")
		}
	case json.Number:
		_ = x
	}
	return nil
}

func parseWireUint(raw []byte) (uint64, error) {
	s := string(raw)
	if s == "" || s == "null" || s == "true" || s == "false" {
		return 0, errors.New("integer")
	}
	neg := false
	if s[0] == '-' {
		neg, s = true, s[1:]
	}
	exponent := 0
	if at := strings.IndexAny(s, "eE"); at >= 0 {
		expText := s[at+1:]
		s = s[:at]
		expNeg := false
		if len(expText) > 0 && (expText[0] == '+' || expText[0] == '-') {
			expNeg = expText[0] == '-'
			expText = expText[1:]
		}
		if expText == "" {
			return 0, errors.New("integer")
		}
		expText = strings.TrimLeft(expText, "0")
		n := 0
		if expText != "" {
			if len(expText) > 7 {
				n = maxJSONBytes + 1
			} else {
				var err error
				n, err = strconv.Atoi(expText)
				if err != nil {
					return 0, errors.New("integer")
				}
				if n > maxJSONBytes+1 {
					n = maxJSONBytes + 1
				}
			}
		}
		if expNeg {
			n = -n
		}
		exponent = n
	}
	fraction := 0
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		fraction = len(s) - dot - 1
		s = s[:dot] + s[dot+1:]
	}
	if s == "" {
		return 0, errors.New("integer")
	}
	allZero := true
	for _, c := range []byte(s) {
		if c < '0' || c > '9' {
			return 0, errors.New("integer")
		}
		if c != '0' {
			allZero = false
		}
	}
	if neg && !allZero {
		return 0, errors.New("integer")
	}
	if allZero {
		return 0, nil
	}
	s = strings.TrimLeft(s, "0")
	scale := exponent - fraction
	if scale < 0 {
		cut := -scale
		if cut > len(s) {
			return 0, errors.New("integer")
		}
		for _, c := range []byte(s[len(s)-cut:]) {
			if c != '0' {
				return 0, errors.New("integer")
			}
		}
		s = s[:len(s)-cut]
		if s == "" {
			return 0, nil
		}
	} else if scale > 0 {
		if scale > 16 || len(s)+scale > 16 {
			return 0, errors.New("integer")
		}
		s += strings.Repeat("0", scale)
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || n > maxWireInteger {
		return 0, errors.New("integer")
	}
	return n, nil
}

func hasUnpairedJSONSurrogate(raw []byte) bool {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '"' {
			continue
		}
		for i++; i < len(raw); i++ {
			if raw[i] == '"' {
				break
			}
			if raw[i] != '\\' || i+1 >= len(raw) {
				continue
			}
			if raw[i+1] != 'u' {
				i++
				continue
			}
			if i+5 >= len(raw) {
				continue
			}
			u, ok := hex4(raw[i+2 : i+6])
			if !ok {
				continue
			}
			if u >= 0xd800 && u <= 0xdbff {
				if i+11 >= len(raw) || raw[i+6] != '\\' || raw[i+7] != 'u' {
					return true
				}
				v, ok := hex4(raw[i+8 : i+12])
				if !ok || v < 0xdc00 || v > 0xdfff || !utf16.IsSurrogate(rune(u)) {
					return true
				}
				i += 11
			} else if u >= 0xdc00 && u <= 0xdfff {
				return true
			} else {
				i += 5
			}
		}
	}
	return false
}

func hex4(b []byte) (uint16, bool) {
	if len(b) != 4 {
		return 0, false
	}
	n, err := strconv.ParseUint(string(b), 16, 16)
	return uint16(n), err == nil
}

func validateChangeFields(m *cemMap) *cemError {
	if !validOID(m.BaseRevision) || !validDigest(m.PatchSHA256) {
		return invalid("version-shape")
	}
	if len(m.Evidence) > maxEvidence || len(m.Hunks) > maxHunks {
		return invalid("resource")
	}
	for i := range m.Evidence {
		e := &m.Evidence[i]
		if !validPath(e.Path) || !validOID(e.BlobOID) || !validDigest(e.SpanSHA256) ||
			e.Span.Start >= e.Span.End || e.Span.End > maxWireInteger ||
			!strings.HasPrefix(e.ID, "evidence:sha256:") || !validDigest(strings.TrimPrefix(e.ID, "evidence:sha256:")) {
			return invalid("evidence-shape")
		}
	}
	for i := range m.Hunks {
		h := &m.Hunks[i]
		if !validPath(h.Path) ||
			!validRange(h.OldRange) || !validRange(h.NewRange) || len([]byte(h.Reason)) > 64 ||
			!strings.HasPrefix(h.ID, "hunk:sha256:") || !validDigest(strings.TrimPrefix(h.ID, "hunk:sha256:")) || h.Basis == nil {
			return invalid("hunk-shape")
		}
	}
	return nil
}

func validRange(r lineRange) bool {
	if r.Start > maxWireInteger || r.Count > maxWireInteger || r.Start > maxWireInteger-r.Count {
		return false
	}
	return (r.Count == 0 && r.Start <= maxWireInteger) || (r.Count != 0 && r.Start >= 1)
}

func validOID(s string) bool {
	return (len(s) == 40 || len(s) == 64) && lowerHex(s)
}

func validDigest(s string) bool { return len(s) == 64 && lowerHex(s) }

func lowerHex(s string) bool {
	for _, c := range []byte(s) {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func validNullablePath(p *string) bool { return p == nil || validPath(*p) }

func validPath(p string) bool {
	if p == "" || len([]byte(p)) > 512 || !utf8.ValidString(p) || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") {
		return false
	}
	for _, r := range p {
		if r == 0 || r < 0x20 || r == 0x7f {
			return false
		}
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func shaHex(b []byte) string {
	d := sha256.Sum256(b)
	return hex.EncodeToString(d[:])
}

func shaHexContext(ctx context.Context, b []byte) (string, *cemError) {
	h := sha256.New()
	for len(b) > 0 {
		if err := ctx.Err(); err != nil {
			return "", operational("verification-timeout")
		}
		n := 1 << 20
		if n > len(b) {
			n = len(b)
		}
		_, _ = h.Write(b[:n])
		b = b[n:]
	}
	if err := ctx.Err(); err != nil {
		return "", operational("verification-timeout")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func canonicalString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func canonicalEvidence(e evidence) []byte {
	s := `{"blobOid":` + canonicalString(e.BlobOID) + `,"path":` + canonicalString(e.Path) +
		`,"span":{"end":` + strconv.FormatUint(e.Span.End, 10) + `,"start":` + strconv.FormatUint(e.Span.Start, 10) +
		`},"spanSha256":` + canonicalString(e.SpanSHA256) + `}`
	return []byte(s)
}

func (v *verifier) git(args ...string) ([]byte, *cemError) {
	return v.gitInput(nil, args...)
}

func (v *verifier) gitInput(input []byte, args ...string) ([]byte, *cemError) {
	var stdout bytes.Buffer
	if err := v.gitTo(&stdout, input, args...); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

// gitTo runs one hardened Git command and writes its standard output to stdout.
func (v *verifier) gitTo(stdout io.Writer, input []byte, args ...string) *cemError {
	if v.gitOps >= maxGitOps {
		return invalid("resource")
	}
	v.gitOps++
	remaining := 10 * time.Second
	if deadline, ok := v.ctx.Deadline(); ok && time.Until(deadline) < remaining {
		remaining = time.Until(deadline)
	}
	if remaining <= 0 {
		return operational("verification-timeout")
	}
	ctx, cancel := context.WithTimeout(v.ctx, remaining)
	defer cancel()
	base := []string{"--no-pager", "-c", "core.hooksPath=/dev/null", "-c", "core.attributesFile=/dev/null", "-c", "filter.lfs.smudge=", "-c", "filter.lfs.required=false", "-c", "protocol.allow=never", "-c", "credential.helper=", "-C", v.repo}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = append(cleanGitEnvironment(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1")
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return operational("git-timeout")
		}
		return operational("repository-io")
	}
	return nil
}

func cleanGitEnvironment() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, item := range env {
		if !strings.HasPrefix(item, "GIT_") {
			out = append(out, item)
		}
	}
	return out
}

// rejectAlternates refuses, before any object read, a repository whose object database would
// borrow objects from another store through objects/info/alternates (CEM-GO-005).
func (v *verifier) rejectAlternates() *cemError {
	b, err := v.git("rev-parse", "--git-path", "objects/info/alternates")
	if err != nil {
		return err
	}
	path := strings.TrimSuffix(string(b), "\n")
	if !filepath.IsAbs(path) {
		path = filepath.Join(v.repo, path)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		return operational("object-alternates")
	}
	return nil
}

type treeEntry struct {
	mode string
	typ  string
	oid  string
}

func (v *verifier) treeEntry(commit, path string) (*treeEntry, *cemError) {
	key := commit + "\x00" + path
	if entry, ok := v.trees[key]; ok {
		return entry, nil
	}
	if err := v.preloadTrees(commit, []string{path}); err != nil {
		return nil, err
	}
	return v.trees[key], nil
}

func (v *verifier) preloadTrees(commit string, paths []string) *cemError {
	if v.trees == nil {
		v.trees = make(map[string]*treeEntry)
	}
	unique := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		key := commit + "\x00" + path
		if _, ok := v.trees[key]; !ok && !seen[path] {
			unique = append(unique, path)
			seen[path] = true
		}
	}
	for start := 0; start < len(unique); start += 512 {
		end := start + 512
		if end > len(unique) {
			end = len(unique)
		}
		args := []string{"ls-tree", "-z", commit, "--"}
		for _, path := range unique[start:end] {
			args = append(args, ":(literal)"+path)
			v.trees[commit+"\x00"+path] = nil
		}
		b, err := v.git(args...)
		if err != nil {
			return err
		}
		if len(b) > 0 && b[len(b)-1] != 0 {
			return invalid("tree-entry")
		}
		for _, raw := range bytes.Split(bytes.TrimSuffix(b, []byte{0}), []byte{0}) {
			if len(raw) == 0 {
				continue
			}
			head, gotPath, ok := strings.Cut(string(raw), "\t")
			parts := strings.Split(head, " ")
			if !ok || !seen[gotPath] || len(parts) != 3 || !validOID(parts[2]) {
				return invalid("tree-entry")
			}
			v.trees[commit+"\x00"+gotPath] = &treeEntry{mode: parts[0], typ: parts[1], oid: parts[2]}
		}
	}
	return nil
}

func (v *verifier) blob(oid string) ([]byte, *cemError) {
	if b, ok := v.blobs[oid]; ok {
		return b, nil
	}
	if err := v.preloadBlobs([]string{oid}); err != nil {
		return nil, err
	}
	return v.blobs[oid], nil
}

func (v *verifier) preloadBlobs(oids []string) *cemError {
	unique := []string{}
	seen := map[string]bool{}
	for _, oid := range oids {
		if _, ok := v.blobs[oid]; !ok && !seen[oid] {
			unique = append(unique, oid)
			seen[oid] = true
		}
	}
	if len(unique) == 0 {
		return nil
	}
	input := []byte(strings.Join(unique, "\n") + "\n")
	check, err := v.gitInput(input, "cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize)")
	if err != nil {
		return err
	}
	sizes := make([]int, len(unique))
	lines := strings.Split(strings.TrimSuffix(string(check), "\n"), "\n")
	if len(lines) != len(unique) {
		return invalid("blob-size")
	}
	newTotal := v.blobBytes
	for i, line := range lines {
		parts := strings.Split(line, " ")
		if len(parts) == 2 && parts[0] == unique[i] && parts[1] == "missing" {
			return operational("repository-io")
		}
		if len(parts) != 3 || parts[0] != unique[i] || parts[1] != "blob" {
			return operational("repository-io")
		}
		size, e := strconv.Atoi(parts[2])
		if e != nil || size < 0 {
			return operational("repository-io")
		}
		if size > maxBlobBytes {
			return invalid("resource")
		}
		newTotal += int64(size)
		if newTotal > maxTotalBlobs {
			return invalid("resource")
		}
		sizes[i] = size
	}
	content, err := v.gitInput(input, "cat-file", "--batch")
	if err != nil {
		return err
	}
	for i, oid := range unique {
		at := bytes.IndexByte(content, '\n')
		if at < 0 {
			return operational("repository-io")
		}
		parts := strings.Split(string(content[:at]), " ")
		content = content[at+1:]
		if len(parts) == 2 && parts[0] == oid && parts[1] == "missing" {
			return operational("repository-io")
		}
		if len(parts) != 3 || parts[0] != oid || parts[1] != "blob" || parts[2] != strconv.Itoa(sizes[i]) || len(content) < sizes[i]+1 || content[sizes[i]] != '\n' {
			return operational("repository-io")
		}
		v.blobs[oid] = append([]byte(nil), content[:sizes[i]]...)
		content = content[sizes[i]+1:]
	}
	if len(content) != 0 {
		return operational("repository-io")
	}
	v.blobBytes = newTotal
	return nil
}

func (v *verifier) verifyEvidence(commit string, m *cemMap) *cemError {
	paths := make([]string, len(m.Evidence))
	for i := range m.Evidence {
		paths[i] = m.Evidence[i].Path
	}
	if err := v.preloadTrees(commit, paths); err != nil {
		return err
	}
	oids := []string{}
	for _, e := range m.Evidence {
		if entry, _ := v.treeEntry(commit, e.Path); entry != nil && entry.typ == "blob" {
			oids = append(oids, entry.oid)
		}
	}
	if err := v.preloadBlobs(oids); err != nil {
		return err
	}
	seen := map[string]bool{}
	for i := range m.Evidence {
		if err := v.ctx.Err(); err != nil {
			return operational("verification-timeout")
		}
		e := &m.Evidence[i]
		if seen[e.ID] {
			return invalid("duplicate-evidence")
		}
		seen[e.ID] = true
		entry, err := v.treeEntry(commit, e.Path)
		if err != nil {
			return err
		}
		if entry == nil || entry.typ != "blob" || entry.mode != "100644" && entry.mode != "100755" || entry.oid != e.BlobOID {
			return invalid("evidence-blob")
		}
		b, err := v.blob(entry.oid)
		if err != nil {
			return err
		}
		if e.Span.End > uint64(len(b)) {
			return invalid("evidence-span")
		}
		// A window into the cached blob (CEM-V1-690): span storage is bounded
		// by the distinct blob bytes, not the summed span lengths.
		e.data = b[e.Span.Start:e.Span.End:e.Span.End]
		spanDigest, hashErr := shaHexContext(v.ctx, e.data)
		if hashErr != nil {
			return hashErr
		}
		if spanDigest != e.SpanSHA256 || "evidence:sha256:"+shaHex(canonicalEvidence(*e)) != e.ID {
			return invalid("evidence-id")
		}
	}
	return nil
}

func (v *verifier) computeDrift(commit string, evidence []evidence) ([]driftItem, *cemError) {
	paths := make([]string, len(evidence))
	for i := range evidence {
		paths[i] = evidence[i].Path
	}
	if err := v.preloadTrees(commit, paths); err != nil {
		return nil, err
	}
	oids := []string{}
	for _, e := range evidence {
		if entry, _ := v.treeEntry(commit, e.Path); entry != nil && regularFile(entry) && entry.oid != e.BlobOID {
			oids = append(oids, entry.oid)
		}
	}
	if err := v.preloadBlobs(oids); err != nil {
		return nil, err
	}
	out := make([]driftItem, 0, len(evidence))
	for _, e := range evidence {
		item := driftItem{EvidenceID: e.ID, Path: e.Path, Status: "deleted"}
		entry, err := v.treeEntry(commit, e.Path)
		if err != nil {
			return nil, err
		}
		if entry == nil || entry.typ != "blob" {
			out = append(out, item)
			continue
		}
		item.TargetBlobOID = &entry.oid
		if entry.oid == e.BlobOID {
			item.Status = "stable"
			item.TargetSpan = &nullableSpan{Start: e.Span.Start, End: e.Span.End}
			out = append(out, item)
			continue
		}
		if !regularFile(entry) {
			item.TargetBlobOID = nil
			out = append(out, item)
			continue
		}
		b, err := v.blob(entry.oid)
		if err != nil {
			return nil, err
		}
		first, matches, matchErr := firstTwoMatches(v.ctx, b, e.data)
		if matchErr != nil {
			return nil, operational("verification-timeout")
		}
		switch matches {
		case 0:
			item.Status = "stale"
		case 1:
			item.Status = "relocated"
			item.TargetSpan = &nullableSpan{Start: uint64(first), End: uint64(first + len(e.data))}
		default:
			item.Status = "ambiguous"
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EvidenceID < out[j].EvidenceID })
	return out, nil
}

// regularFile reports whether a tree entry has file content that same-path drift may search.
func regularFile(entry *treeEntry) bool {
	return entry.typ == "blob" && (entry.mode == "100644" || entry.mode == "100755")
}

func firstTwoMatches(ctx context.Context, haystack, needle []byte) (int, int, error) {
	first, count := -1, 0
	for pos := 0; pos <= len(haystack)-len(needle); {
		if err := ctx.Err(); err != nil {
			return -1, count, err
		}
		i := bytes.Index(haystack[pos:], needle)
		if i < 0 {
			break
		}
		at := pos + i
		if count == 0 {
			first = at
		}
		count++
		if count == 2 {
			return first, count, nil
		}
		pos = at + 1
	}
	return first, count, ctx.Err()
}

// Experimental candidate support is additive. These mechanics are derived from
// protocol/cem-1.0/ALGORITHMS.md; no native Corvint package is linked here.
const candidateSpec = "cem/1.0-experimental.1"
const candidateSidecar = ".corvint/change.cem.json"

type candidateMap struct {
	cemMap
	Artifacts []candidateArtifact
}
type candidateArtifact struct{ Kind, Path, SHA256 string }
type candidateResult struct {
	Profile            string            `json:"profile"`
	Spec               string            `json:"spec"`
	Integrity          string            `json:"integrity"`
	BaseRevision       string            `json:"baseRevision"`
	TargetRevision     string            `json:"targetRevision"`
	ReferenceIntegrity string            `json:"referenceIntegrity"`
	Limits             map[string]string `json:"limits"`
	Code               string            `json:"code"`
}

func newCandidateResult(base, target string) candidateResult {
	r := candidateResult{Profile: "cem-candidate-verification/1", Spec: candidateSpec, Integrity: "NOT_VERIFIED", BaseRevision: base, TargetRevision: target, ReferenceIntegrity: "REFERENCE_INTEGRITY_ONLY", Limits: map[string]string{}}
	for _, key := range []string{"nativeAuthority", "historicalValidity", "currentApplicability", "sourceGitBinding", "authentication", "dependencyClosure", "criterionDiscrimination", "externalInteroperability"} {
		r.Limits[key] = "NOT_OBSERVED"
	}
	return r
}

// Candidate numbers deliberately have a narrower lexical domain than historical 0.1.
func candidateTokens(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	depth := 0
	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch v := t.(type) {
		case json.Delim:
			if v == '{' || v == '[' {
				depth++
				if depth > 64 {
					return errors.New("depth")
				}
			} else {
				depth--
			}
		case json.Number:
			s := string(v)
			if s == "" || len(s) > 16 || len(s) > 1 && s[0] == '0' {
				return errors.New("integer")
			}
			for _, c := range s {
				if c < '0' || c > '9' {
					return errors.New("integer")
				}
			}
			n, e := strconv.ParseUint(s, 10, 64)
			if e != nil || n > maxWireInteger {
				return errors.New("integer")
			}
		}
	}
	return strictJSON(raw)
}

func candidateRecords(raw json.RawMessage, bound int, keys ...string) ([]map[string]json.RawMessage, error) {
	var rows []map[string]json.RawMessage
	if json.Unmarshal(raw, &rows) != nil || len(rows) == 0 || len(rows) > bound {
		return nil, errors.New("records")
	}
	for _, row := range rows {
		if !hasKeys(row, keys...) {
			return nil, errors.New("record shape")
		}
	}
	return rows, nil
}
func candidateText(row map[string]json.RawMessage, key string) string {
	var s string
	_ = json.Unmarshal(row[key], &s)
	return s
}
func candidatePath(p string) bool {
	if !validPath(p) {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if strings.EqualFold(s, ".git") {
			return false
		}
	}
	return true
}
func candidateTicket(s string) bool {
	p := strings.Split(s, ":")
	if len(s) > 128 || len(p) != 4 || p[0] != "ticket" || len(p[3]) > 64 {
		return false
	}
	for _, token := range p[1:] {
		if token == "" {
			return false
		}
		for i, c := range []byte(token) {
			alpha := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
			if !alpha && (i == 0 || c != '.' && c != '_' && c != '-') {
				return false
			}
		}
	}
	return true
}

// Scalar kinds are checked before Go decoding: encoding/json otherwise maps
// null to the zero value of string and integer destinations without an error.
func candidateWireString(raw json.RawMessage) (string, bool) {
	raw = bytes.TrimSpace(raw)
	var value string
	if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	return value, true
}
func candidateWireInteger(raw json.RawMessage) (uint64, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || len(raw) > 16 || len(raw) > 1 && raw[0] == '0' {
		return 0, false
	}
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, e := strconv.ParseUint(string(raw), 10, 64)
	return n, e == nil && n <= maxWireInteger
}
func candidateScalarTypes(row map[string]json.RawMessage) bool {
	for key, raw := range row {
		switch key {
		case "start", "end", "count", "criterionIndex":
			if _, ok := candidateWireInteger(raw); !ok {
				return false
			}
		case "span", "oldRange", "newRange":
			var child map[string]json.RawMessage
			if json.Unmarshal(raw, &child) != nil || child == nil || !candidateScalarTypes(child) {
				return false
			}
		case "evidence", "hunks", "basis", "criterionBindings", "runnerReceipts", "criterionLinks", "artifacts":
			var children []map[string]json.RawMessage
			if json.Unmarshal(raw, &children) != nil || children == nil {
				return false
			}
			for _, child := range children {
				if child == nil || !candidateScalarTypes(child) {
					return false
				}
			}
		case "hunkIds", "evidenceIds", "runnerReceiptSha256s":
			var children []json.RawMessage
			if json.Unmarshal(raw, &children) != nil || children == nil {
				return false
			}
			for _, child := range children {
				if _, ok := candidateWireString(child); !ok {
					return false
				}
			}
		default:
			if _, ok := candidateWireString(raw); !ok {
				return false
			}
		}
	}
	return true
}

func candidateCanonicalRecord(row map[string]json.RawMessage) ([]byte, error) {
	keys := []string{}
	for key := range row {
		if key != "id" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var out strings.Builder
	out.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString(canonicalString(key))
		out.WriteByte(':')
		if key == "externalSpan" {
			// CEM-V1-332: the external span member is the closed {end,start}
			// object, written in canonical key order.
			var sp map[string]json.RawMessage
			if json.Unmarshal(row[key], &sp) != nil || len(sp) != 2 {
				return nil, errors.New("external span type")
			}
			start, okStart := candidateWireInteger(sp["start"])
			end, okEnd := candidateWireInteger(sp["end"])
			if !okStart || !okEnd {
				return nil, errors.New("external span type")
			}
			out.WriteString(`{"end":` + strconv.FormatUint(end, 10) + `,"start":` + strconv.FormatUint(start, 10) + `}`)
		} else if key == "criterionIndex" {
			n, ok := candidateWireInteger(row[key])
			if !ok {
				return nil, errors.New("criterion index type")
			}
			out.WriteString(strconv.FormatUint(n, 10))
		} else {
			s, ok := candidateWireString(row[key])
			if !ok {
				return nil, errors.New("criterion string type")
			}
			out.WriteString(canonicalString(s))
		}
	}
	out.WriteByte('}')
	return []byte(out.String()), nil
}
func decodeCandidate(raw []byte) (candidateMap, *cemError) {
	var m candidateMap
	if len(raw) > maxJSONBytes || candidateTokens(raw) != nil {
		return m, invalid("invalid-json")
	}
	var root map[string]json.RawMessage
	_ = json.Unmarshal(raw, &root)
	if !hasKeys(root, "spec", "baseRevision", "patchSha256", "excludedPath", "evidence", "hunks", "criterionBindings", "runnerReceipts", "criterionLinks", "artifacts") || candidateText(root, "spec") != candidateSpec || candidateText(root, "excludedPath") != candidateSidecar || validateEvidenceHunkShape(root) != nil || !candidateScalarTypes(root) {
		return m, invalid("version-shape")
	}
	m.Spec = candidateSpec
	m.BaseRevision = candidateText(root, "baseRevision")
	m.PatchSHA256 = candidateText(root, "patchSha256")
	if json.Unmarshal(root["evidence"], &m.Evidence) != nil || json.Unmarshal(root["hunks"], &m.Hunks) != nil {
		return m, invalid("version-shape")
	}
	if err := validateChangeFields(&m.cemMap); err != nil {
		return m, err
	}
	criteria, e := candidateRecords(root["criterionBindings"], 256, "id", "ticketId", "acceptanceRevision", "acceptanceSha256", "criterionIndex", "criterionSha256", "captureSha256", "verificationSha256", "claimTicketSha256", "snapshotHeadReceiptSha256")
	if e != nil {
		return m, invalid("criterion-shape")
	}
	receipts, e := candidateRecords(root["runnerReceipts"], 64, "sha256", "profile", "planSha256", "sourceGitBinding", "executionAuthority", "dependencyClosure", "authentication")
	if e != nil {
		return m, invalid("receipt-shape")
	}
	links, e := candidateRecords(root["criterionLinks"], 1024, "criterionId", "hunkIds", "evidenceIds", "runnerReceiptSha256s")
	if e != nil {
		return m, invalid("link-shape")
	}
	artifacts, e := candidateRecords(root["artifacts"], 1024, "kind", "path", "sha256")
	if e != nil {
		return m, invalid("artifact-shape")
	}
	roles, paths, used := map[string]string{}, map[string]bool{}, map[string]bool{}
	for _, a := range artifacts {
		kind, path, digest := candidateText(a, "kind"), candidateText(a, "path"), candidateText(a, "sha256")
		if !candidatePath(path) || !validDigest(digest) || paths[path] || roles[digest] != "" {
			return m, invalid("artifact-shape")
		}
		switch kind {
		case "tasks-capture", "tasks-verification", "tasks-claimed-ticket", "runner-plan", "runner-receipt":
		default:
			return m, invalid("artifact-kind")
		}
		roles[digest] = kind
		paths[path] = true
		m.Artifacts = append(m.Artifacts, candidateArtifact{kind, path, digest})
	}
	use := func(digest, role string) bool {
		if roles[digest] != role {
			return false
		}
		used[digest] = true
		return true
	}
	criterionIDs, tuples, groups := map[string]bool{}, map[string]bool{}, map[string]string{}
	for _, c := range criteria {
		id, ticket, rev := candidateText(c, "id"), candidateText(c, "ticketId"), candidateText(c, "acceptanceRevision")
		n, err := strconv.ParseUint(rev, 10, 64)
		if err != nil || n == 0 || n > maxWireInteger || strconv.FormatUint(n, 10) != rev || !candidateTicket(ticket) {
			return m, invalid("criterion-shape")
		}
		for _, key := range []string{"acceptanceSha256", "criterionSha256", "captureSha256", "verificationSha256", "claimTicketSha256", "snapshotHeadReceiptSha256"} {
			if !validDigest(candidateText(c, key)) {
				return m, invalid("criterion-shape")
			}
		}
		index, integerOK := candidateWireInteger(c["criterionIndex"])
		if !integerOK || index >= 256 {
			return m, invalid("criterion-index")
		}
		canonical, err := candidateCanonicalRecord(c)
		if err != nil || id != "criterion:sha256:"+shaHex(canonical) || criterionIDs[id] {
			return m, invalid("criterion-id")
		}
		criterionIDs[id] = true
		group := ticket + ":" + rev
		tuple := group + ":" + strconv.FormatUint(index, 10)
		if tuples[tuple] {
			return m, invalid("criterion-duplicate")
		}
		tuples[tuple] = true
		coherence := ""
		for _, key := range []string{"acceptanceSha256", "captureSha256", "verificationSha256", "claimTicketSha256", "snapshotHeadReceiptSha256"} {
			coherence += candidateText(c, key)
		}
		if prior := groups[group]; prior != "" && prior != coherence {
			return m, invalid("criterion-coherence")
		}
		groups[group] = coherence
		if !use(candidateText(c, "captureSha256"), "tasks-capture") || !use(candidateText(c, "verificationSha256"), "tasks-verification") || !use(candidateText(c, "claimTicketSha256"), "tasks-claimed-ticket") {
			return m, invalid("artifact-role")
		}
	}
	receiptIDs := map[string]bool{}
	for _, r := range receipts {
		id := candidateText(r, "sha256")
		if !validDigest(id) || receiptIDs[id] || candidateText(r, "profile") != "corvint-test-runner-receipt/0" || candidateText(r, "sourceGitBinding") != "NOT_OBSERVED" || candidateText(r, "executionAuthority") != "CALLER_OBSERVED" || candidateText(r, "dependencyClosure") != "NOT_OBSERVED" || candidateText(r, "authentication") != "NOT_OBSERVED" || !use(id, "runner-receipt") || !use(candidateText(r, "planSha256"), "runner-plan") {
			return m, invalid("receipt-shape")
		}
		receiptIDs[id] = true
	}
	if len(used) != len(roles) {
		return m, invalid("unused-artifact")
	}
	hunks := map[string]mappedHunk{}
	for _, h := range m.Hunks {
		hunks[h.ID] = h
	}
	evidenceIDs := map[string]bool{}
	for _, e := range m.Evidence {
		evidenceIDs[e.ID] = true
	}
	linkedCriteria, linkedReceipts := map[string]bool{}, map[string]bool{}
	for _, l := range links {
		id := candidateText(l, "criterionId")
		if !criterionIDs[id] || linkedCriteria[id] {
			return m, invalid("link-criterion")
		}
		linkedCriteria[id] = true
		arrays := map[string][]string{}
		for _, key := range []string{"hunkIds", "evidenceIds", "runnerReceiptSha256s"} {
			var ids []string
			if json.Unmarshal(l[key], &ids) != nil || len(ids) == 0 || len(ids) > 32 {
				return m, invalid("link-shape")
			}
			seen := map[string]bool{}
			for _, v := range ids {
				if seen[v] {
					return m, invalid("link-duplicate")
				}
				seen[v] = true
			}
			arrays[key] = ids
		}
		linkedBasis := map[string]bool{}
		for _, hid := range arrays["hunkIds"] {
			h, ok := hunks[hid]
			if !ok {
				return m, invalid("link-hunk")
			}
			for _, b := range h.Basis {
				linkedBasis[b.EvidenceID] = true
			}
		}
		for _, eid := range arrays["evidenceIds"] {
			if !evidenceIDs[eid] || !linkedBasis[eid] {
				return m, invalid("link-evidence")
			}
		}
		for _, rid := range arrays["runnerReceiptSha256s"] {
			if !receiptIDs[rid] {
				return m, invalid("link-receipt")
			}
			linkedReceipts[rid] = true
		}
	}
	if len(linkedCriteria) != len(criterionIDs) || len(linkedReceipts) != len(receiptIDs) {
		return m, invalid("unused-reference")
	}
	return m, nil
}

func candidateRegular(root *os.Root, path string) error {
	parts := strings.Split(path, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return err
		}
		if i < len(parts)-1 {
			if !info.IsDir() {
				return errNotRegular
			}
		} else if !info.Mode().IsRegular() {
			return errNotRegular
		}
	}
	return nil
}
func candidateArtifacts(ctx context.Context, root *os.Root, artifacts []candidateArtifact) *cemError {
	total := 0
	for _, a := range artifacts {
		if ctx.Err() != nil {
			return operational("verification-timeout")
		}
		if candidateRegular(root, a.Path) != nil {
			return invalid("artifact-path")
		}
		f, e := root.Open(a.Path)
		if e != nil {
			return invalid("artifact-read")
		}
		b, e := io.ReadAll(io.LimitReader(f, maxJSONBytes+1))
		_ = f.Close()
		total += len(b)
		if e != nil || len(b) > maxJSONBytes || total > 16<<20 || shaHex(b) != a.SHA256 {
			return invalid("artifact-integrity")
		}
	}
	return nil
}

// Keep the buffer named: promoted ReadFrom and WriteString bypass the cap.
type candidateBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (b *candidateBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.buffer.Len() {
		b.exceeded = true
		return 0, errSizeLimit
	}
	return b.buffer.Write(p)
}
func (b *candidateBuffer) Bytes() []byte { return b.buffer.Bytes() }
func (v *verifier) candidateGit(limit int, args ...string) ([]byte, *cemError) {
	if v.gitOps >= maxGitOps {
		return nil, invalid("resource")
	}
	v.gitOps++
	ctx, cancel := context.WithTimeout(v.ctx, 10*time.Second)
	defer cancel()
	base := []string{"--no-pager", "--literal-pathspecs", "-c", "core.hooksPath=/dev/null", "-c", "core.attributesFile=/dev/null", "-c", "protocol.allow=never", "-c", "credential.helper=", "-C", v.repo}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
	cmd.Env = append(cleanGitEnvironment(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1", "GIT_ATTR_NOSYSTEM=1")
	out, stderr := &candidateBuffer{limit: limit}, &candidateBuffer{limit: 64 << 10}
	cmd.Stdout = out
	cmd.Stderr = stderr
	err := cmd.Run()
	// A child exit error can mask a pipe-copy error in Cmd.Run.
	if out.exceeded || stderr.exceeded {
		return nil, operational("unsupported-resource-limit")
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, operational("git-timeout")
		}
		return nil, operational("repository-io")
	}
	return out.Bytes(), nil
}
func candidateObjectHash(kind string, b []byte, oidLength int) string {
	header := []byte(fmt.Sprintf("%s %d\x00", kind, len(b)))
	if oidLength == 40 {
		h := sha1.New()
		_, _ = h.Write(header)
		_, _ = h.Write(b)
		return hex.EncodeToString(h.Sum(nil))
	}
	h := sha256.New()
	_, _ = h.Write(header)
	_, _ = h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
func (v *verifier) candidateObject(kind, oid string, limit int) ([]byte, *cemError) {
	b, e := v.candidateGit(limit, "cat-file", kind, oid)
	if e != nil {
		return nil, e
	}
	if candidateObjectHash(kind, b, len(oid)) != oid {
		return nil, invalid("object-integrity")
	}
	return b, nil
}

// Every traversed object is self-hashed before its names or bytes enter caches.
func (v *verifier) candidateSnapshot(commit string) (map[string]*treeEntry, *cemError) {
	b, e := v.candidateObject("commit", commit, 1<<20)
	if e != nil {
		return nil, e
	}
	line, _, ok := strings.Cut(string(b), "\n")
	if !ok || !strings.HasPrefix(line, "tree ") {
		return nil, invalid("commit-shape")
	}
	tree := strings.TrimPrefix(line, "tree ")
	if !validOID(tree) || len(tree) != len(commit) {
		return nil, invalid("commit-shape")
	}
	files := map[string]*treeEntry{}
	var walk func(string, string, int) *cemError
	walk = func(oid, prefix string, depth int) *cemError {
		if depth > 32 {
			return invalid("resource")
		}
		raw, err := v.candidateObject("tree", oid, 4<<20)
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		for len(raw) > 0 {
			sep := bytes.IndexByte(raw, ' ')
			nul := bytes.IndexByte(raw, 0)
			width := len(commit) / 2
			if sep < 1 || nul <= sep+1 || len(raw) < nul+1+width {
				return invalid("tree-shape")
			}
			mode, name := string(raw[:sep]), string(raw[sep+1:nul])
			child := hex.EncodeToString(raw[nul+1 : nul+1+width])
			raw = raw[nul+1+width:]
			path := prefix + name
			if strings.Contains(name, "/") || !candidatePath(path) || seen[name] {
				return invalid("tree-path")
			}
			seen[name] = true
			if mode == "40000" {
				if err := walk(child, path+"/", depth+1); err != nil {
					return err
				}
				continue
			}
			if mode != "100644" && mode != "100755" {
				return operational("unsupported-tree-mode")
			}
			if len(files) >= 4096 {
				return invalid("resource")
			}
			entry := &treeEntry{mode: mode, typ: "blob", oid: child}
			files[path] = entry
			v.trees[commit+"\x00"+path] = entry
		}
		return nil
	}
	if err := walk(tree, "", 0); err != nil {
		return nil, err
	}
	return files, nil
}
func candidateRepository(repo string) *cemError {
	root, e := os.OpenRoot(repo)
	if e != nil {
		return operational("repository-io")
	}
	defer root.Close()
	for _, path := range []string{".git", ".git/objects", ".git/objects/info"} {
		i, e := root.Lstat(path)
		if e != nil || !i.IsDir() {
			return operational("unsupported-repository")
		}
	}
	for _, path := range []string{".git/objects/info/alternates", ".git/objects/info/http-alternates", ".git/info/grafts", ".git/shallow", ".git/info/attributes"} {
		if _, e := root.Lstat(path); !errors.Is(e, os.ErrNotExist) {
			return operational("unsupported-repository")
		}
	}
	if candidateRegular(root, ".git/config") != nil {
		return operational("unsupported-config")
	}
	f, e := root.Open(".git/config")
	if e != nil {
		return operational("unsupported-config")
	}
	b, e := io.ReadAll(io.LimitReader(f, 65537))
	_ = f.Close()
	if e != nil || len(b) > 65536 {
		return operational("unsupported-config")
	}
	// The deliberately narrow reference profile refuses includes, filters, promisor
	// stores and custom extensions rather than trying to sanitize arbitrary config.
	section := ""
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if line != "[core]" && line != "[extensions]" {
				return operational("unsupported-config")
			}
			section = line
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			return operational("unsupported-config")
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if section == "[core]" {
			switch key {
			case "repositoryformatversion", "filemode", "bare", "logallrefupdates", "ignorecase", "precomposeunicode":
			default:
				return operational("unsupported-config")
			}
		} else if section != "[extensions]" || key != "objectformat" {
			return operational("unsupported-config")
		}
	}
	return nil
}
func verifyCandidate(ctx context.Context, repo string, raw []byte, base, target, artifactDir string) *cemError {
	m, e := decodeCandidate(raw)
	if e != nil {
		return e
	}
	if !validOID(base) || !validOID(target) || len(base) != len(target) || m.BaseRevision != base {
		return invalid("base-target-binding")
	}
	if !filepath.IsAbs(artifactDir) {
		return operational("artifact-root")
	}
	root, err := os.OpenRoot(artifactDir)
	if err != nil {
		return operational("artifact-root")
	}
	defer root.Close()
	if e := candidateArtifacts(ctx, root, m.Artifacts); e != nil {
		return e
	}
	if e := candidateRepository(repo); e != nil {
		return e
	}
	v := &verifier{ctx: ctx, repo: repo, blobs: map[string][]byte{}, trees: map[string]*treeEntry{}}
	before, e := v.candidateSnapshot(base)
	if e != nil {
		return e
	}
	after, e := v.candidateSnapshot(target)
	if e != nil {
		return e
	}
	changed := map[string]bool{}
	all := map[string]bool{}
	for p := range before {
		all[p] = true
	}
	for p := range after {
		all[p] = true
	}
	needed := map[string]bool{candidateSidecar: true}
	for p := range all {
		a, b := before[p], after[p]
		if (a == nil) != (b == nil) || a != nil && b != nil && (a.mode != b.mode || a.oid != b.oid) {
			if p != candidateSidecar {
				changed[p] = true
			}
			needed[p] = true
		}
	}
	for _, e := range m.Evidence {
		needed[e.Path] = true
	}
	for p := range needed {
		for _, item := range []struct {
			commit string
			files  map[string]*treeEntry
		}{{base, before}, {target, after}} {
			entry := item.files[p]
			v.trees[item.commit+"\x00"+p] = entry
			if entry != nil {
				if _, ok := v.blobs[entry.oid]; !ok {
					b, e := v.candidateObject("blob", entry.oid, maxBlobBytes)
					if e != nil {
						return e
					}
					v.blobBytes += int64(len(b))
					if v.blobBytes > maxTotalBlobs {
						return invalid("resource")
					}
					v.blobs[entry.oid] = b
				}
			}
		}
	}
	for _, entry := range []*treeEntry{before[candidateSidecar], after[candidateSidecar]} {
		if entry != nil && entry.mode != "100644" {
			return invalid("sidecar-mode")
		}
	}
	if entry := after[candidateSidecar]; entry != nil && !bytes.Equal(v.blobs[entry.oid], raw) {
		return invalid("sidecar-bytes")
	}
	emptyTree := candidateObjectHash("tree", nil, len(base))
	var patch []byte
	paths := []string{}
	for p := range changed {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	args := []string{"--attr-source=" + emptyTree, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--diff-algorithm=myers", "--no-indent-heuristic", "--unified=3", "--inter-hunk-context=0", "--full-index", "--src-prefix=a/", "--dst-prefix=b/", "--no-color", base, target, "--"}
	if len(paths) > 0 {
		args = append(args, paths...)
		patch, e = v.candidateGit(maxPatchBytes, args...)
		if e != nil {
			return e
		}
	} else {
		patch = nil
	}
	if shaHex(patch) != m.PatchSHA256 {
		return invalid("patch-digest")
	}
	parsed, e := parsePatch(patch)
	if e != nil {
		return e
	}
	if parsed.binary {
		// The candidate route stays text-only (CEM-V1-934).
		return invalid("patch")
	}
	seen := map[string]bool{}
	for _, f := range parsed.files {
		path := ""
		if f.oldPath != nil {
			path = *f.oldPath
		}
		if f.newPath != nil {
			if path != "" && path != *f.newPath {
				return invalid("patch-path")
			}
			path = *f.newPath
		}
		if !changed[path] || seen[path] {
			return invalid("patch-inventory")
		}
		seen[path] = true
		a, b := before[path], after[path]
		if (f.oldPath == nil) != (a == nil) || (f.newPath == nil) != (b == nil) {
			return invalid("patch-inventory")
		}
		var input, want []byte
		if a != nil {
			input = v.blobs[a.oid]
		}
		if b != nil {
			want = v.blobs[b.oid]
		}
		got, e := simulate(ctx, input, f.hunks)
		if e != nil {
			return e
		}
		if !bytes.Equal(got, want) {
			return invalid("patch-target")
		}
	}
	if len(seen) != len(changed) {
		return invalid("patch-inventory")
	}
	if e := v.verifyPatchSimulation(base, parsed); e != nil {
		return e
	}
	if e := v.verifyEvidence(base, &m.cemMap); e != nil {
		return e
	}
	if e := verifyHunkMap(&m.cemMap, parsed); e != nil {
		return e
	}
	drift, e := v.computeDrift(target, m.Evidence)
	if e != nil {
		return e
	}
	for _, d := range drift {
		if d.Status != "stable" && d.Status != "relocated" {
			return invalid("evidence-drift")
		}
	}
	return candidateArtifacts(ctx, root, m.Artifacts)
}

// CEM 0.3: independently authored from the public algorithm contract.
const sidecar03 = ".corvint/change.cem.json"

type coverage03 struct {
	ProfileSHA256 string      `json:"profileSha256"`
	TestRun       string      `json:"testRun"`
	Mode          string      `json:"mode"`
	State         string      `json:"state"`
	Covered       []lineRange `json:"covered"`
}
type survivor03 struct {
	Operator    string `json:"operator"`
	Line        uint64 `json:"line"`
	Description string `json:"description"`
}
type bounds03 struct {
	MaxHunks        uint64 `json:"maxHunks"`
	MaxMutants      uint64 `json:"maxMutants"`
	WallTimeSeconds uint64 `json:"wallTimeSeconds"`
}
type discrimination03 struct {
	TreeRevision    string       `json:"treeRevision"`
	SelectionSHA256 string       `json:"selectionSha256"`
	Mutants         uint64       `json:"mutants"`
	Killed          uint64       `json:"killed"`
	Survived        uint64       `json:"survived"`
	Survivors       []survivor03 `json:"survivors"`
	Bounds          bounds03     `json:"bounds"`
	State           string       `json:"state"`
	Detail          string       `json:"detail"`
}
type hunk03 struct {
	mappedHunk
	Coverage      *coverage03       `json:"coverage,omitempty"`
	Discriminates *discrimination03 `json:"discriminates,omitempty"`
}

// The old reader intentionally accepts some integral decimal spellings. This
// profile's scanner is separate so its stricter lexemes cannot change that ABI.
func strictJSON03(raw []byte) error {
	if !utf8.Valid(raw) || hasUnpairedJSONSurrogate(raw) {
		return errors.New("utf8")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		t, e := dec.Token()
		if e != nil {
			return e
		}
		if n, ok := t.(json.Number); ok {
			s := string(n)
			if len(s) == 0 {
				return errors.New("number")
			}
			for _, b := range []byte(s) {
				if b < '0' || b > '9' {
					return errors.New("number")
				}
			}
			u, e := strconv.ParseUint(s, 10, 64)
			if e != nil || u > maxWireInteger {
				return errors.New("number")
			}
			return nil
		}
		d, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		if depth >= 64 {
			return errors.New("depth")
		}
		switch d {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				k, e := dec.Token()
				if e != nil {
					return e
				}
				s, ok := k.(string)
				if !ok || seen[s] {
					return errors.New("key")
				}
				seen[s] = true
				if e = value(depth + 1); e != nil {
					return e
				}
			}
			t, e = dec.Token()
			if e != nil || t != json.Delim('}') {
				return errors.New("object")
			}
		case '[':
			for dec.More() {
				if e = value(depth + 1); e != nil {
					return e
				}
			}
			t, e = dec.Token()
			if e != nil || t != json.Delim(']') {
				return errors.New("array")
			}
		default:
			return errors.New("delimiter")
		}
		return nil
	}
	if e := value(0); e != nil {
		return e
	}
	if _, e := dec.Token(); e != io.EOF {
		return errors.New("trailing")
	}
	return nil
}
func object03(raw json.RawMessage, required, optional []string) (map[string]json.RawMessage, *cemError) {
	var obj map[string]json.RawMessage
	if len(raw) == 0 || bytes.TrimSpace(raw)[0] != '{' || json.Unmarshal(raw, &obj) != nil || obj == nil {
		return nil, invalid("invalid-field")
	}
	allowed := map[string]bool{}
	for _, k := range required {
		allowed[k] = true
		if _, ok := obj[k]; !ok {
			return nil, invalid("missing-field")
		}
	}
	for _, k := range optional {
		allowed[k] = true
	}
	for k := range obj {
		if !allowed[k] {
			return nil, invalid("unknown-field")
		}
	}
	return obj, nil
}
func list03(raw json.RawMessage, limit int) ([]json.RawMessage, *cemError) {
	var a []json.RawMessage
	if len(raw) == 0 || bytes.TrimSpace(raw)[0] != '[' || json.Unmarshal(raw, &a) != nil || a == nil || len(a) > limit {
		return nil, invalid("invalid-field")
	}
	return a, nil
}
func text03(raw json.RawMessage) (string, bool) {
	var s string
	if len(raw) == 0 || bytes.TrimSpace(raw)[0] != '"' || json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}
func uint03(raw json.RawMessage) (uint64, bool) {
	var n uint64
	if len(raw) == 0 || raw[0] < '0' || raw[0] > '9' || json.Unmarshal(raw, &n) != nil || n > maxWireInteger {
		return 0, false
	}
	return n, true
}
func textBound03(s string, min, max int) bool {
	if len(s) < min || len(s) > max {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
func range03(raw json.RawMessage) (lineRange, *cemError) {
	var r lineRange
	o, e := object03(raw, []string{"start", "count"}, nil)
	if e != nil {
		return r, e
	}
	var ok bool
	r.Start, ok = uint03(o["start"])
	if !ok {
		return r, invalid("invalid-field")
	}
	r.Count, ok = uint03(o["count"])
	if !ok || r.Count > 0 && r.Start == 0 {
		return r, invalid("invalid-field")
	}
	return r, nil
}

// hunkRange03 admits any wire-integer start/count pair for a hunk's old and new
// range; a zero start with a nonzero count is a hunk-mapping issue, not wire.
func hunkRange03(raw json.RawMessage) (lineRange, *cemError) {
	var r lineRange
	o, e := object03(raw, []string{"start", "count"}, nil)
	if e != nil {
		return r, e
	}
	var ok bool
	if r.Start, ok = uint03(o["start"]); !ok {
		return r, invalid("invalid-field")
	}
	if r.Count, ok = uint03(o["count"]); !ok {
		return r, invalid("invalid-field")
	}
	return r, nil
}
func coverageDecode03(raw json.RawMessage, hr lineRange) (*coverage03, *cemError) {
	o, e := object03(raw, []string{"profileSha256", "testRun", "mode", "state", "covered"}, nil)
	if e != nil {
		return nil, e
	}
	c := &coverage03{}
	var ok bool
	if c.ProfileSHA256, ok = text03(o["profileSha256"]); !ok || !validDigest(c.ProfileSHA256) {
		return nil, invalid("invalid-field")
	}
	if c.TestRun, ok = text03(o["testRun"]); !ok || !textBound03(c.TestRun, 1, 256) {
		return nil, invalid("invalid-field")
	}
	if c.Mode, ok = text03(o["mode"]); !ok || c.Mode != "set" && c.Mode != "count" && c.Mode != "atomic" {
		return nil, invalid("invalid-field")
	}
	if c.State, ok = text03(o["state"]); !ok || c.State != "covered" && c.State != "uncovered" {
		return nil, invalid("invalid-field")
	}
	a, e := list03(o["covered"], maxJSONBytes)
	if e != nil {
		return nil, e
	}
	c.Covered = []lineRange{}
	var end uint64
	for i, raw := range a {
		r, e := range03(raw)
		if e != nil {
			return nil, e
		}
		if r.Count == 0 || hr.Count == 0 || r.Start < hr.Start || r.Start-hr.Start >= hr.Count || r.Count > hr.Count-(r.Start-hr.Start) || i > 0 && r.Start <= end {
			return nil, invalid("invalid-field")
		}
		end = r.Start + r.Count
		c.Covered = append(c.Covered, r)
	}
	if (c.State == "covered") != (len(c.Covered) > 0) {
		return nil, invalid("invalid-field")
	}
	return c, nil
}
func relation03(s string) bool {
	switch s {
	case "specification", "decision", "test-claim", "implementation", "call-site", "dependency", "incident":
		return true
	}
	return false
}

// structuralTuples are the exact Go releases admitted for structural
// mechanical proof: go1.27.1, the S0E prototype tuple, and go1.27.2 and
// go1.27.3, each admitted by its sealed
// protocol/cem-1.0/structural-tuples/<version> packet (decisions 0522, 0547).
var structuralTuples = map[string]bool{"go1.27.1": true, "go1.27.2": true, "go1.27.3": true}

func structuralRuntimeAdmitted(version string) bool { return structuralTuples[version] }

func structuralReason03(s string) bool {
	switch s {
	case "rename", "move", "import-reorder", "formatter-only":
		return true
	}
	return false
}

// CEM 0.3: independently authored from the public algorithm contract.
type token03 struct {
	Kind    token.Token
	Literal string
	Offset  int
}
type syntax03 struct {
	File   *ast.File
	Set    *token.FileSet
	Tokens []token03
}

func parseGo03(raw []byte) (syntax03, bool) {
	s := syntax03{Set: token.NewFileSet()}
	var err error
	s.File, err = parser.ParseFile(s.Set, "proof.go", raw, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return s, false
	}
	fs := token.NewFileSet()
	f := fs.AddFile("proof.go", -1, len(raw))
	var scan scanner.Scanner
	bad := false
	scan.Init(f, raw, func(token.Position, string) { bad = true }, scanner.ScanComments)
	for {
		p, k, v := scan.Scan()
		if k == token.EOF {
			break
		}
		s.Tokens = append(s.Tokens, token03{k, v, f.Offset(p)})
	}
	return s, !bad
}
func tokensEqual03(a, b []token03) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || a[i].Literal != b[i].Literal {
			return false
		}
	}
	return true
}
func tokensKey03(ts []token03) string {
	var b strings.Builder
	for _, t := range ts {
		b.WriteString(t.Kind.String())
		b.WriteByte(0)
		b.WriteString(t.Literal)
		b.WriteByte(0)
	}
	return b.String()
}
func wholeWord03(text, from, to string) string {
	var b strings.Builder
	for len(text) > 0 {
		r, n := utf8.DecodeRuneInString(text)
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			end := n
			for end < len(text) {
				r, n = utf8.DecodeRuneInString(text[end:])
				if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					break
				}
				end += n
			}
			word := text[:end]
			if word == from {
				word = to
			}
			b.WriteString(word)
			text = text[end:]
		} else {
			b.WriteString(text[:n])
			text = text[n:]
		}
	}
	return b.String()
}
func rename03(a, b syntax03) bool {
	if len(a.Tokens) != len(b.Tokens) {
		return false
	}
	from, to := "", ""
	for i, x := range a.Tokens {
		y := b.Tokens[i]
		if x.Kind != y.Kind {
			return false
		}
		if x.Literal == y.Literal || x.Kind == token.COMMENT {
			continue
		}
		if x.Kind != token.IDENT {
			return false
		}
		if from == "" {
			from, to = x.Literal, y.Literal
		}
		if x.Literal != from || y.Literal != to {
			return false
		}
	}
	if from == "" || from == to || ast.IsExported(from) || ast.IsExported(to) {
		return false
	}
	for _, t := range a.Tokens {
		if t.Kind == token.IDENT && t.Literal == to {
			return false
		}
	}
	declared, forbidden := false, false
	if a.File.Name.Name == from {
		forbidden = true
	}
	ast.Inspect(a.File, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ValueSpec:
			for _, id := range x.Names {
				declared = declared || id.Name == from
			}
		case *ast.TypeSpec:
			declared = declared || x.Name.Name == from
		case *ast.FuncDecl:
			if x.Recv != nil {
				for _, field := range x.Recv.List {
					for _, id := range field.Names {
						declared = declared || id.Name == from
					}
				}
			}
			if x.Name.Name == from {
				if x.Recv != nil {
					forbidden = true
				} else {
					declared = true
				}
			}
		case *ast.LabeledStmt:
			declared = declared || x.Label.Name == from
		case *ast.ImportSpec:
			declared = declared || x.Name != nil && x.Name.Name == from
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE {
				for _, v := range x.Lhs {
					if id, ok := v.(*ast.Ident); ok && id.Name == from {
						declared = true
					}
				}
			}
		case *ast.RangeStmt:
			if x.Tok == token.DEFINE {
				for _, v := range []ast.Expr{x.Key, x.Value} {
					if id, ok := v.(*ast.Ident); ok && id.Name == from {
						declared = true
					}
				}
			}
		case *ast.FuncType:
			for _, fields := range []*ast.FieldList{x.TypeParams, x.Params, x.Results} {
				if fields != nil {
					for _, f := range fields.List {
						for _, id := range f.Names {
							declared = declared || id.Name == from
						}
					}
				}
			}
		case *ast.SelectorExpr:
			forbidden = forbidden || x.Sel.Name == from
		case *ast.KeyValueExpr:
			if id, ok := x.Key.(*ast.Ident); ok && id.Name == from {
				forbidden = true
			}
		case *ast.StructType:
			for _, f := range x.Fields.List {
				for _, id := range f.Names {
					forbidden = forbidden || id.Name == from
				}
			}
		case *ast.InterfaceType:
			for _, f := range x.Methods.List {
				for _, id := range f.Names {
					forbidden = forbidden || id.Name == from
				}
			}
		}
		return true
	})
	if !declared || forbidden {
		return false
	}
	for i, x := range a.Tokens {
		y := b.Tokens[i]
		if x.Kind == token.IDENT {
			want := x.Literal
			if want == from {
				want = to
			}
			if y.Literal != want {
				return false
			}
		} else if x.Kind == token.COMMENT {
			directive := strings.HasPrefix(x.Literal, "//go:") || strings.HasPrefix(x.Literal, "//line ") || strings.HasPrefix(x.Literal, "//export ")
			if directive {
				if x.Literal != y.Literal {
					return false
				}
			} else if wholeWord03(x.Literal, from, to) != y.Literal {
				return false
			}
		} else if x.Literal != y.Literal {
			return false
		}
	}
	return true
}

type interval03 struct{ Start, End int }

func nodeInterval03(s syntax03, n ast.Node) interval03 {
	return interval03{s.Set.Position(n.Pos()).Offset, s.Set.Position(n.End()).Offset}
}
func tokensWithin03(ts []token03, ranges []interval03) (inside, outside []token03) {
	for _, t := range ts {
		in := false
		for _, r := range ranges {
			if t.Offset >= r.Start && t.Offset < r.End {
				in = true
				break
			}
		}
		if in {
			inside = append(inside, t)
		} else {
			outside = append(outside, t)
		}
	}
	return
}
func importReorder03(a, b syntax03) bool {
	parts := func(s syntax03) (ordered []string, comments, outside []token03) {
		ranges := []interval03{}
		for _, d := range s.File.Decls {
			if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
				ranges = append(ranges, nodeInterval03(s, g))
			}
		}
		for _, i := range s.File.Imports {
			name := ""
			if i.Name != nil {
				name = i.Name.Name
			}
			ordered = append(ordered, name+"\x00"+i.Path.Value)
		}
		_, outside = tokensWithin03(s.Tokens, ranges)
		for _, t := range s.Tokens {
			if t.Kind == token.COMMENT {
				comments = append(comments, t)
			}
		}
		return
	}
	ao, ac, ax := parts(a)
	bo, bc, bx := parts(b)
	if len(ao) != len(bo) || strings.Join(ao, "\x01") == strings.Join(bo, "\x01") || !tokensEqual03(ac, bc) || !tokensEqual03(ax, bx) {
		return false
	}
	sort.Strings(ao)
	sort.Strings(bo)
	return strings.Join(ao, "\x01") == strings.Join(bo, "\x01")
}
func move03(a, b syntax03) bool {
	parts := func(s syntax03) (all, orderedSensitive []string, outside []token03) {
		ranges := []interval03{}
		for _, d := range s.File.Decls {
			r := nodeInterval03(s, d)
			switch x := d.(type) {
			case *ast.GenDecl:
				if x.Doc != nil {
					r.Start = s.Set.Position(x.Doc.Pos()).Offset
				}
			case *ast.FuncDecl:
				if x.Doc != nil {
					r.Start = s.Set.Position(x.Doc.Pos()).Offset
				}
			}
			ranges = append(ranges, r)
			inside, _ := tokensWithin03(s.Tokens, []interval03{r})
			key := tokensKey03(inside)
			all = append(all, key)
			sensitive := false
			switch x := d.(type) {
			case *ast.GenDecl:
				sensitive = x.Tok == token.VAR
			case *ast.FuncDecl:
				sensitive = x.Recv == nil && x.Name.Name == "init"
			}
			if sensitive {
				orderedSensitive = append(orderedSensitive, key)
			}
		}
		_, outside = tokensWithin03(s.Tokens, ranges)
		return
	}
	aa, as, ax := parts(a)
	ba, bs, bx := parts(b)
	if strings.Join(aa, "\x02") == strings.Join(ba, "\x02") || strings.Join(as, "\x02") != strings.Join(bs, "\x02") || !tokensEqual03(ax, bx) {
		return false
	}
	sort.Strings(aa)
	sort.Strings(ba)
	return strings.Join(aa, "\x02") == strings.Join(ba, "\x02")
}
func structuralProof03(ctx context.Context, _ string, base []byte, f *filePatch, h *patchHunk, reason string) bool {
	if f.oldPath == nil || f.newPath == nil {
		return false
	}
	hunks := f.hunks
	if reason != "move" {
		copy := *h
		copy.NewRange.Start = copy.OldRange.Start
		if copy.OldRange.Count == 0 && copy.NewRange.Count > 0 {
			copy.NewRange.Start++
		}
		if copy.OldRange.Count > 0 && copy.NewRange.Count == 0 {
			copy.NewRange.Start--
		}
		hunks = []*patchHunk{&copy}
	}
	image, e := simulate(ctx, base, hunks)
	if e != nil || bytes.Equal(base, image) {
		return false
	}
	return structuralPredicate03(reason, base, image)
}

// structuralPredicate03 judges one derived image against its base file; the
// structural corpus (decision 0551) drives it directly.
func structuralPredicate03(reason string, base, image []byte) bool {
	a, ok := parseGo03(base)
	if !ok {
		return false
	}
	b, ok := parseGo03(image)
	if !ok {
		return false
	}
	switch reason {
	case "rename":
		return rename03(a, b)
	case "move":
		return move03(a, b)
	case "import-reorder":
		return importReorder03(a, b)
	case "formatter-only":
		x, e := format.Source(base)
		if e != nil {
			return false
		}
		y, e := format.Source(image)
		return e == nil && bytes.Equal(x, y)
	}
	return false
}

// CEM 0.3: independently authored from the public algorithm contract.
// The proposal deliberately admits only the existing portable primary/config
// envelope. Exceeding this envelope is operational unsupported, never invalid.
func repository03(repo string) *cemError {
	if !filepath.IsAbs(repo) {
		return operational("unsupported-repository-envelope")
	}
	if os.Getenv("GIT_ALTERNATE_OBJECT_DIRECTORIES") != "" {
		return operational("unsupported-object-alternates")
	}
	for _, name := range []string{"alternates", "http-alternates"} {
		if _, e := os.Lstat(filepath.Join(repo, ".git", "objects", "info", name)); !errors.Is(e, os.ErrNotExist) {
			return operational("unsupported-object-alternates")
		}
	}
	if e := candidateRepository(repo); e != nil {
		return operational("unsupported-repository-envelope")
	}
	return nil
}

// buffer03 intentionally does not embed bytes.Buffer: a promoted ReadFrom would
// let io.Copy bypass Write and its resource bound.
type buffer03 struct {
	buffer bytes.Buffer
	limit  int
}

func (b *buffer03) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.buffer.Len() {
		return 0, errSizeLimit
	}
	return b.buffer.Write(p)
}
func (b *buffer03) Bytes() []byte { return b.buffer.Bytes() }

func (v *verifier) git03(limit int, args ...string) ([]byte, *cemError) {
	if v.gitOps >= maxGitOps {
		return nil, operational("unsupported-resource-limit")
	}
	v.gitOps++
	ctx, cancel := context.WithTimeout(v.ctx, 10*time.Second)
	defer cancel()
	base := []string{"--no-pager", "--no-optional-locks", "--git-dir=" + filepath.Join(v.repo, ".git"), "-c", "core.hooksPath=/dev/null", "-c", "protocol.allow=never", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "core.attributesFile=/dev/null", "-c", "credential.helper=", "-c", "core.quotePath=false", "-c", "diff.suppressBlankEmpty=false", "-c", "diff.orderFile=/dev/null", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false", "-c", "diff.renames=false", "-c", "diff.algorithm=myers", "-c", "diff.wsErrorHighlight=none", "-c", "diff.srcPrefix=a/", "-c", "diff.dstPrefix=b/", "-c", "diff.external=", "-c", "diff.ignoreSubmodules=none", "-c", "advice.graftFileDeprecated=false"}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
	cmd.Dir = v.repo
	cmd.Env = append(cleanGitEnvironment(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE=/dev/null", "GIT_ASKPASS=", "GIT_ATTR_NOSYSTEM=1")
	stdout, stderr := &buffer03{limit: limit}, &buffer03{limit: 64 << 10}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	// The output copy may finish well after a successful Git exits on a loaded host; 250 ms
	// turned valid reads into git-read-failed (CEM-V1-820).
	cmd.WaitDelay = 5 * time.Second
	// CommandContext owns cancellation of this exact child until Wait reaps it.
	// No group ID is retained or signalled after reap. Descendant retirement is
	// not qualified by this narrowly admitted, sanitized Git-only backend.
	err := cmd.Run()
	if err != nil {
		if ctx.Err() != nil {
			return nil, operational("git-timeout")
		}
		if errors.Is(err, errSizeLimit) {
			return nil, operational("unsupported-resource-limit")
		}
		return nil, operational("git-read-failed")
	}
	return append([]byte(nil), stdout.Bytes()...), nil
}
func (v *verifier) object03(kind, oid string, limit int) ([]byte, *cemError) {
	b, e := v.git03(limit, "cat-file", kind, oid)
	if e != nil {
		return nil, e
	}
	if candidateObjectHash(kind, b, len(oid)) != oid {
		return nil, operational("repository-object-unavailable")
	}
	return b, nil
}

// commitTree03 reads one commit's root tree id, as snapshot03 does.
func (v *verifier) commitTree03(commit string) (string, *cemError) {
	raw, e := v.object03("commit", commit, 1<<20)
	if e != nil {
		return "", e
	}
	first, _, ok := strings.Cut(string(raw), "\n")
	root := strings.TrimPrefix(first, "tree ")
	if !ok || !strings.HasPrefix(first, "tree ") || !validOID(root) || len(root) != len(commit) {
		return "", operational("repository-object-unavailable")
	}
	return root, nil
}
func (v *verifier) snapshot03(commit string) (map[string]*treeEntry, *cemError) {
	// One MiB commits and 4096 entries are a declared narrower operational envelope.
	raw, e := v.object03("commit", commit, 1<<20)
	if e != nil {
		return nil, e
	}
	first, _, ok := strings.Cut(string(raw), "\n")
	if !ok || !strings.HasPrefix(first, "tree ") {
		return nil, operational("repository-object-unavailable")
	}
	root := strings.TrimPrefix(first, "tree ")
	if !validOID(root) || len(root) != len(commit) {
		return nil, operational("repository-object-unavailable")
	}
	entries := map[string]*treeEntry{}
	treeBytes := 0
	var walk func(string, string, int) *cemError
	walk = func(oid, prefix string, depth int) *cemError {
		if depth > 128 {
			return operational("unsupported-resource-limit")
		}
		body, e := v.object03("tree", oid, 4<<20)
		if e != nil {
			return e
		}
		treeBytes += len(body)
		if treeBytes > 4<<20 {
			return operational("unsupported-resource-limit")
		}
		seen := map[string]bool{}
		type childTree struct{ oid, path string }
		children := []childTree{}
		for len(body) > 0 {
			space, nul := bytes.IndexByte(body, ' '), bytes.IndexByte(body, 0)
			width := len(commit) / 2
			if space <= 0 || nul <= space+1 || len(body) < nul+1+width {
				return operational("repository-object-unavailable")
			}
			mode, name := string(body[:space]), string(body[space+1:nul])
			child := hex.EncodeToString(body[nul+1 : nul+1+width])
			body = body[nul+1+width:]
			path := prefix + name
			if strings.Contains(name, "/") || !validPath(path) || seen[name] {
				return operational("unsupported-repository-envelope")
			}
			seen[name] = true
			if len(entries) >= 4096 {
				return operational("unsupported-resource-limit")
			}
			typ := ""
			switch mode {
			case "40000":
				typ = "tree"
			case "120000":
				typ = "blob"
			case "160000":
				typ = "commit"
			default:
				if len(mode) != 6 || !strings.HasPrefix(mode, "100") {
					return operational("unsupported-repository-envelope")
				}
				n, err := strconv.ParseUint(mode, 8, 32)
				if err != nil {
					return operational("unsupported-repository-envelope")
				}
				mode = "100644"
				if n&0100 != 0 {
					mode = "100755"
				}
				typ = "blob"
			}
			entries[path] = &treeEntry{mode: mode, typ: typ, oid: child}
			if typ == "tree" {
				children = append(children, childTree{child, path + "/"})
			}
		}
		// Parse every trailing entry before granting any descendant authority.
		for _, c := range children {
			if e := walk(c.oid, c.path, depth+1); e != nil {
				return e
			}
		}
		return nil
	}
	if e := walk(root, "", 0); e != nil {
		return nil, e
	}
	return entries, nil
}
func (v *verifier) blob03(oid string) ([]byte, *cemError) {
	if b, ok := v.blobs[oid]; ok {
		return b, nil
	}
	b, e := v.object03("blob", oid, maxBlobBytes)
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > maxTotalBlobs-v.blobBytes {
		return nil, operational("unsupported-resource-limit")
	}
	v.blobBytes += int64(len(b))
	v.blobs[oid] = b
	return b, nil
}
func (v *verifier) patch03(base, target string) ([]byte, *cemError) {
	emptyTree := candidateObjectHash("tree", nil, len(base))
	patch, e := v.git03(maxPatchBytes, "--attr-source="+emptyTree, "diff", "--full-index", "--no-color", "--no-ext-diff", "--no-textconv", "--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers", "--unified=3", "--inter-hunk-context=0", "--src-prefix=a/", "--dst-prefix=b/", "--ignore-submodules=none", "--end-of-options", base, target, "--", ".", ":(exclude).corvint/change.cem.json")
	// Canonical diff derivation has its own public failure class. Keep resource
	// and cancellation errors, and errors from ordinary object reads, distinct.
	if e != nil && e.code == "git-read-failed" {
		return nil, operational("git-diff-failed")
	}
	return patch, e
}

// CEM 0.3: independently authored from the public algorithm contract.
type drift03 struct {
	EvidenceID    string        `json:"evidenceId"`
	Path          string        `json:"path"`
	BaseBlobOID   string        `json:"baseBlobOid"`
	Status        string        `json:"status"`
	TargetBlobOID *string       `json:"targetBlobOid"`
	TargetSpan    *nullableSpan `json:"targetSpan"`
}
type witness03 struct {
	ID            string            `json:"id"`
	Coverage      *coverage03       `json:"coverage,omitempty"`
	Discriminates *discrimination03 `json:"discriminates,omitempty"`
}
type runtime03 struct {
	GoVersion               string `json:"goVersion"`
	StructuralQualification string `json:"structuralQualification"`
}
type result03 struct {
	Profile            string         `json:"profile"`
	Spec               string         `json:"spec"`
	Outcome            string         `json:"outcome"`
	Accept             *bool          `json:"accept"`
	Stage              string         `json:"stage"`
	Code               *string        `json:"code"`
	IssueCodes         []string       `json:"issueCodes"`
	Assurance          *string        `json:"assurance"`
	ExpectedBase       *string        `json:"expectedBase"`
	TargetRevision     *string        `json:"targetRevision"`
	MapSHA256          *string        `json:"mapSha256"`
	PatchSHA256        *string        `json:"patchSha256"`
	Drift              []drift03      `json:"drift"`
	Hunks              []witness03    `json:"hunks"`
	Counts             map[string]int `json:"counts"`
	Runtime            runtime03      `json:"runtime"`
	RepositoryEnvelope string         `json:"repositoryEnvelope"`
}

func string03(s string) *string { return &s }
func (r *result03) fail(stage string, e *cemError, verification bool) int {
	r.Stage = stage
	if e.operational {
		r.Assurance = nil
		r.Outcome = "UNSUPPORTED"
		r.Accept = nil
		r.Code = string03(e.code)
		return 2
	}
	no := false
	r.Accept = &no
	r.Outcome = "REJECT"
	if verification {
		r.IssueCodes = append(r.IssueCodes, e.code)
		return 1
	}
	r.Code = string03(e.code)
	return 2
}

// Stable CEM is a separate closed wire and verification operation. These types
// retain the full document; no 0.x profile substitution or lossy writer is used.
const stableSpec = "cem/1.0"

// stableCriterion has three closed forms: the plain Git blob span form {id,
// criterionIndex, criterionSha256, evidenceId} (CEM-V1-083), the optional Tasks
// extension with all eight Tasks members and no evidenceId, or the external
// form {id, criterionIndex, criterionSha256, externalRepository,
// externalBlobOid, externalSpan} (CEM-V1-330). Empty members are omitted, so
// the plain and Tasks forms keep their identity and encoding.
type stableCriterion struct {
	ID                         string `json:"id"`
	TicketID                   string `json:"ticketId,omitempty"`
	AcceptanceRevision         string `json:"acceptanceRevision,omitempty"`
	CriterionIndex             uint64 `json:"criterionIndex"`
	AcceptanceSHA256           string `json:"acceptanceSha256,omitempty"`
	CriterionSHA256            string `json:"criterionSha256"`
	CaptureSHA256              string `json:"captureSha256,omitempty"`
	VerificationSHA256         string `json:"verificationSha256,omitempty"`
	ClaimTicketSHA256          string `json:"claimTicketSha256,omitempty"`
	SnapshotHeadReceiptSHA256  string `json:"snapshotHeadReceiptSha256,omitempty"`
	SnapshotHeadArtifactSHA256 string `json:"snapshotHeadArtifactSha256,omitempty"`
	EvidenceID                 string `json:"evidenceId,omitempty"`
	ExternalRepository         string `json:"externalRepository,omitempty"`
	ExternalBlobOID            string `json:"externalBlobOid,omitempty"`
	ExternalSpan               *span  `json:"externalSpan,omitempty"`
}

var stableExternalCriterionKeys = []string{"externalRepository", "externalBlobOid", "externalSpan"}

// stableExternalRepositoryName is the closed external repository name
// (CEM-V1-330): ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$.
func stableExternalRepositoryName(s string) bool {
	if len(s) < 1 || len(s) > 64 {
		return false
	}
	for i, r := range s {
		alnum := r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if !alnum && (i == 0 || r != '.' && r != '_' && r != '-') {
			return false
		}
	}
	return true
}

var stableTasksCriterionKeys = []string{"ticketId", "acceptanceRevision", "acceptanceSha256", "captureSha256", "verificationSha256", "claimTicketSha256", "snapshotHeadReceiptSha256", "snapshotHeadArtifactSha256"}

// stableCriterionKeys selects the closed set: any Tasks member requires the
// whole Tasks form; otherwise any external member requires the whole external
// form (CEM-V1-331); otherwise the plain form applies.
func stableCriterionKeys(raw json.RawMessage) []string {
	var obj map[string]json.RawMessage
	_ = json.Unmarshal(raw, &obj)
	for _, k := range stableTasksCriterionKeys {
		if _, ok := obj[k]; ok {
			return []string{"id", "ticketId", "acceptanceRevision", "criterionIndex", "acceptanceSha256", "criterionSha256", "captureSha256", "verificationSha256", "claimTicketSha256", "snapshotHeadReceiptSha256", "snapshotHeadArtifactSha256"}
		}
	}
	for _, k := range stableExternalCriterionKeys {
		if _, ok := obj[k]; ok {
			return []string{"id", "criterionIndex", "criterionSha256", "externalRepository", "externalBlobOid", "externalSpan"}
		}
	}
	return []string{"id", "criterionIndex", "criterionSha256", "evidenceId"}
}

type stableRunner struct {
	SHA256             string               `json:"sha256"`
	Profile            string               `json:"profile"`
	PlanSHA256         string               `json:"planSha256"`
	SourceGitBinding   string               `json:"sourceGitBinding"`
	ExecutionAuthority string               `json:"executionAuthority"`
	DependencyClosure  string               `json:"dependencyClosure"`
	Authentication     stableAuthentication `json:"authentication"`
}
type stableLink struct {
	CriterionID          string   `json:"criterionId"`
	HunkIDs              []string `json:"hunkIds"`
	EvidenceIDs          []string `json:"evidenceIds"`
	RunnerReceiptSHA256s []string `json:"runnerReceiptSha256s"`
}
type stableArtifact struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type stableReferences struct {
	CriterionBindings []stableCriterion `json:"criterionBindings"`
	RunnerReceipts    []stableRunner    `json:"runnerReceipts"`
	CriterionLinks    []stableLink      `json:"criterionLinks"`
	Artifacts         []stableArtifact  `json:"artifacts"`
}
type stableMap struct {
	Spec         string     `json:"spec"`
	BaseRevision string     `json:"baseRevision"`
	PatchSHA256  string     `json:"patchSha256"`
	ExcludedPath string     `json:"excludedPath"`
	Evidence     []evidence `json:"evidence"`
	Hunks        []hunk03   `json:"hunks"`
	stableReferences
	// Optional CEM-V1-093/095 members; never authority, never echoed.
	Provenance *stableProvenance `json:"provenance,omitempty"`
	Role       string            `json:"role,omitempty"`
}
type stableArtifactCheck struct {
	Kind       string `json:"kind"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	ByteLength int    `json:"byteLength"`
}
type stableResult struct {
	Profile            string                `json:"profile"`
	Spec               string                `json:"spec"`
	VerificationMode   string                `json:"verificationMode"`
	Outcome            string                `json:"outcome"`
	Accept             *bool                 `json:"accept"`
	Stage              string                `json:"stage"`
	Code               *string               `json:"code"`
	IssueCodes         []string              `json:"issueCodes"`
	Assurance          *string               `json:"assurance"`
	ExpectedBase       *string               `json:"expectedBase"`
	TargetRevision     *string               `json:"targetRevision"`
	MapSHA256          *string               `json:"mapSha256"`
	PatchSHA256        *string               `json:"patchSha256"`
	Sidecar            string                `json:"sidecar"`
	Drift              []drift03             `json:"drift"`
	Hunks              resultHunks           `json:"hunks"`
	References         resultReferences      `json:"references"`
	ArtifactChecks     []stableArtifactCheck `json:"artifactChecks"`
	Citation           *citationBlock        `json:"citation"`
	Axes               map[string]string     `json:"axes"`
	Runtime            runtime03             `json:"runtime"`
	RepositoryEnvelope string                `json:"repositoryEnvelope"`
	// VerdictSHA256 is recomputed by MarshalJSON on every encoding (CEM-V1-066).
	VerdictSHA256 string `json:"verdictSha256"`
}

func emptyStableReferences() stableReferences {
	return stableReferences{[]stableCriterion{}, []stableRunner{}, []stableLink{}, []stableArtifact{}}
}

// stableLegacyEnvelope is the repository envelope of the legacy portable
// verify-stable (R1) path: the independently reviewed
// primary-clean-config-bounded/1 envelope that R1-EXPECTED.json pins. The
// canonical path uses stableCanonicalEnvelope. newStableResult takes the
// envelope from its caller, so no result carries a defaulted label (V1-2142).
const stableLegacyEnvelope = "primary-clean-config-bounded/1"

func newStableResult(base, target, envelope string) stableResult {
	axes := map[string]string{}
	for _, key := range []string{"changeIntegrity", "referenceIntegrity", "structuralProof", "coverageValidation", "discriminationValidation"} {
		axes[key] = "NOT_CHECKED"
	}
	for _, key := range []string{"nativeAuthority", "historicalValidity", "currentApplicability", "sourceGitBinding", "sourceInventoryCompleteness", "executionAtCommit", "runnerReceiptSemantics", "runnerExecution", "authentication", "dependencyClosure", "criterionAdequacy", "criterionDiscrimination", "externalInteroperability"} {
		axes[key] = "NOT_OBSERVED"
	}
	r := stableResult{Profile: "cem-stable-verification/1", Spec: stableSpec, VerificationMode: "canonical-and-reference-integrity", Outcome: "UNSUPPORTED", Stage: "arguments", IssueCodes: []string{}, Sidecar: "NOT_CHECKED", Drift: []drift03{}, Hunks: resultHunks{}, References: resultReferences(emptyStableReferences()), ArtifactChecks: []stableArtifactCheck{}, Axes: axes, Runtime: runtime03{runtime.Version(), "NOT_REQUIRED"}, RepositoryEnvelope: envelope}
	if base != "" {
		r.ExpectedBase = string03(base)
	}
	if target != "" {
		r.TargetRevision = string03(target)
	}
	return r
}

// failStable records a refusal or rejection. The citation block is carried only
// by an accepted result (CEM-V1-092), so every failure clears it.
func failStable(r *stableResult, stage string, e *cemError, verification bool) (stableResult, int) {
	r.Citation = nil
	r.Stage = stage
	if e.operational {
		r.Assurance = nil
		r.Outcome = "UNSUPPORTED"
		r.Accept = nil
		r.Code = string03(e.code)
		return *r, 2
	}
	no := false
	r.Accept = &no
	r.Outcome = "REJECT"
	switch stage {
	case "references":
		r.Axes["referenceIntegrity"] = "FAILED"
	case "binding", "verification", "drift":
		r.Axes["changeIntegrity"] = "FAILED"
	case "artifacts":
		r.Axes["referenceIntegrity"] = "FAILED"
	}
	if stage == "verification" && e.code == "unproven-mechanical" {
		r.Axes["structuralProof"] = "FAILED"
	}
	if verification {
		r.IssueCodes = append(r.IssueCodes, e.code)
		return *r, 1
	}
	r.Code = string03(e.code)
	return *r, 2
}
func prefixedStable(s, prefix string) bool {
	return strings.HasPrefix(s, prefix) && validDigest(strings.TrimPrefix(s, prefix))
}
func stableString(o map[string]json.RawMessage, key string, valid func(string) bool) (string, *cemError) {
	s, ok := text03(o[key])
	if !ok || valid != nil && !valid(s) {
		return "", invalid("invalid-field")
	}
	return s, nil
}
func stableIDs(raw json.RawMessage, prefix string) ([]string, *cemError) {
	a, e := list03(raw, 32)
	if e != nil {
		return nil, e
	}
	if len(a) == 0 {
		return nil, invalid("invalid-field")
	}
	out := []string{}
	for _, v := range a {
		s, ok := text03(v)
		if !ok || !prefixedStable(s, prefix) {
			return nil, invalid("invalid-field")
		}
		out = append(out, s)
	}
	return out, nil
}
func stableDiscrimination(raw json.RawMessage, hr lineRange) (*discrimination03, *cemError) {
	o, e := object03(raw, []string{"treeRevision", "selectionSha256", "mutants", "killed", "survived", "survivors", "bounds", "state", "detail"}, nil)
	if e != nil {
		return nil, e
	}
	d := &discrimination03{}
	var ok bool
	if d.TreeRevision, e = stableString(o, "treeRevision", validOID); e != nil {
		return nil, e
	}
	if d.SelectionSHA256, e = stableString(o, "selectionSha256", validDigest); e != nil {
		return nil, e
	}
	for _, v := range []struct {
		key  string
		dest *uint64
	}{{"mutants", &d.Mutants}, {"killed", &d.Killed}, {"survived", &d.Survived}} {
		if *v.dest, ok = uint03(o[v.key]); !ok {
			return nil, invalid("invalid-field")
		}
	}
	a, e := list03(o["survivors"], maxJSONBytes)
	if e != nil {
		return nil, e
	}
	d.Survivors = []survivor03{}
	for _, raw := range a {
		o, e := object03(raw, []string{"operator", "line", "description"}, nil)
		if e != nil {
			return nil, e
		}
		v := survivor03{}
		if v.Operator, e = stableString(o, "operator", func(s string) bool { return textBound03(s, 1, 64) }); e != nil {
			return nil, e
		}
		if v.Line, ok = uint03(o["line"]); !ok || hr.Count == 0 || v.Line < hr.Start || v.Line-hr.Start >= hr.Count {
			return nil, invalid("invalid-field")
		}
		if v.Description, e = stableString(o, "description", func(s string) bool { return textBound03(s, 1, 512) }); e != nil {
			return nil, e
		}
		d.Survivors = append(d.Survivors, v)
	}
	b, e := object03(o["bounds"], []string{"maxHunks", "maxMutants", "wallTimeSeconds"}, nil)
	if e != nil {
		return nil, e
	}
	for _, v := range []struct {
		key  string
		dest *uint64
	}{{"maxHunks", &d.Bounds.MaxHunks}, {"maxMutants", &d.Bounds.MaxMutants}, {"wallTimeSeconds", &d.Bounds.WallTimeSeconds}} {
		if *v.dest, ok = uint03(b[v.key]); !ok || *v.dest == 0 {
			return nil, invalid("invalid-field")
		}
	}
	if d.State, e = stableString(o, "state", nil); e != nil {
		return nil, e
	}
	if d.Detail, e = stableString(o, "detail", func(s string) bool { return textBound03(s, 0, 512) }); e != nil {
		return nil, e
	}
	// CEM-V1-090: mutants never exceeds the declared bounds.maxMutants.
	if d.Mutants > d.Bounds.MaxMutants || d.Killed > d.Mutants || d.Survived > d.Mutants-d.Killed || uint64(len(d.Survivors)) != d.Survived {
		return nil, invalid("invalid-field")
	}
	switch d.State {
	case "discriminates":
		if d.Killed == 0 || d.Survived != 0 {
			return nil, invalid("invalid-field")
		}
	case "survived":
		if d.Survived == 0 {
			return nil, invalid("invalid-field")
		}
	case "not-run":
		if d.Mutants != 0 {
			return nil, invalid("invalid-field")
		}
	default:
		return nil, invalid("invalid-field")
	}
	return d, nil
}
func decodeStable(raw []byte) (stableMap, *cemError) {
	m := stableMap{Evidence: []evidence{}, Hunks: []hunk03{}, stableReferences: emptyStableReferences()}
	if len(raw) > maxJSONBytes {
		return m, operational("map-unavailable")
	}
	if strictJSON03(raw) != nil {
		return m, invalid("invalid-json")
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil || root == nil {
		return m, invalid("invalid-field")
	}
	if v, exists := root["spec"]; exists {
		s, ok := text03(v)
		if !ok || s != stableSpec {
			return m, invalid("unsupported-spec")
		}
	}
	root, e := object03(raw, []string{"spec", "baseRevision", "patchSha256", "excludedPath", "evidence", "hunks", "criterionBindings", "runnerReceipts", "criterionLinks", "artifacts"}, []string{"provenance", "role"})
	if e != nil {
		return m, e
	}
	m.Spec = stableSpec
	if m.BaseRevision, e = stableString(root, "baseRevision", validOID); e != nil {
		return m, e
	}
	if m.PatchSHA256, e = stableString(root, "patchSha256", validDigest); e != nil {
		return m, e
	}
	if m.ExcludedPath, e = stableString(root, "excludedPath", func(s string) bool { return s == sidecar03 }); e != nil {
		return m, invalid("invalid-excluded-path")
	}
	es, e := list03(root["evidence"], maxEvidence)
	if e != nil {
		return m, e
	}
	for _, raw := range es {
		o, e := object03(raw, []string{"id", "blobOid", "path", "span", "spanSha256"}, nil)
		if e != nil {
			return m, e
		}
		v := evidence{}
		if v.ID, e = stableString(o, "id", func(s string) bool { return prefixedStable(s, "evidence:sha256:") }); e != nil {
			return m, e
		}
		if v.BlobOID, e = stableString(o, "blobOid", validOID); e != nil {
			return m, e
		}
		if v.Path, e = stableString(o, "path", validPath); e != nil {
			return m, e
		}
		sp, e := object03(o["span"], []string{"start", "end"}, nil)
		if e != nil {
			return m, e
		}
		var ok bool
		if v.Span.Start, ok = uint03(sp["start"]); !ok {
			return m, invalid("invalid-field")
		}
		if v.Span.End, ok = uint03(sp["end"]); !ok {
			return m, invalid("invalid-field")
		}
		if v.SpanSHA256, e = stableString(o, "spanSha256", validDigest); e != nil {
			return m, e
		}
		m.Evidence = append(m.Evidence, v)
	}
	hs, e := list03(root["hunks"], maxHunks)
	if e != nil {
		return m, e
	}
	if len(hs) == 0 {
		return m, invalid("invalid-field")
	}
	for _, raw := range hs {
		o, e := object03(raw, []string{"id", "path", "oldRange", "newRange", "disposition", "reason", "basis"}, []string{"coverage", "discriminates"})
		if e != nil {
			return m, e
		}
		v := hunk03{}
		if v.ID, e = stableString(o, "id", func(s string) bool { return prefixedStable(s, "hunk:sha256:") }); e != nil {
			return m, e
		}
		if v.Path, e = stableString(o, "path", validPath); e != nil {
			return m, e
		}
		if v.OldRange, e = range03(o["oldRange"]); e != nil {
			return m, e
		}
		if v.NewRange, e = range03(o["newRange"]); e != nil {
			return m, e
		}
		if v.Disposition, e = stableString(o, "disposition", func(s string) bool { return s == "supported" || s == "unknown" || s == "mechanical" }); e != nil {
			return m, e
		}
		if v.Reason, e = stableString(o, "reason", func(s string) bool {
			return s == "evidence-backed" || s == "no-evidence" || s == "insufficient-evidence" || s == "conflicting-evidence" || s == "whitespace-only" || s == "line-ending-only" || structuralReason03(s)
		}); e != nil {
			return m, e
		}
		bs, e := list03(o["basis"], maxBases)
		if e != nil {
			return m, e
		}
		v.Basis = []basis{}
		for _, raw := range bs {
			o, e := object03(raw, []string{"evidenceId", "relation"}, nil)
			if e != nil {
				return m, e
			}
			b := basis{}
			if b.EvidenceID, e = stableString(o, "evidenceId", func(s string) bool { return prefixedStable(s, "evidence:sha256:") }); e != nil {
				return m, e
			}
			if b.Relation, e = stableString(o, "relation", relation03); e != nil {
				return m, e
			}
			v.Basis = append(v.Basis, b)
		}
		switch v.Disposition {
		case "supported":
			if v.Reason != "evidence-backed" || len(v.Basis) == 0 {
				return m, invalid("invalid-field")
			}
		case "unknown":
			if len(v.Basis) != 0 || v.Reason != "no-evidence" && v.Reason != "insufficient-evidence" && v.Reason != "conflicting-evidence" {
				return m, invalid("invalid-field")
			}
		case "mechanical":
			if len(v.Basis) != 0 || !structuralReason03(v.Reason) && v.Reason != "whitespace-only" && v.Reason != "line-ending-only" {
				return m, invalid("invalid-field")
			}
		}
		if b, exists := o["coverage"]; exists {
			v.Coverage, e = coverageDecode03(b, v.NewRange)
			if e != nil {
				return m, e
			}
		}
		if b, exists := o["discriminates"]; exists {
			v.Discriminates, e = stableDiscrimination(b, v.NewRange)
			if e != nil {
				return m, e
			}
		}
		m.Hunks = append(m.Hunks, v)
	}
	if e = stableEvidenceReferences(m); e != nil {
		return m, e
	}
	cs, e := list03(root["criterionBindings"], 256)
	if e != nil {
		return m, e
	}
	for _, raw := range cs {
		keys := stableCriterionKeys(raw)
		tasks := keys[1] == "ticketId"
		external := keys[3] == "externalRepository"
		o, e := object03(raw, keys, nil)
		if e != nil {
			return m, e
		}
		c := stableCriterion{}
		if c.ID, e = stableString(o, "id", func(s string) bool { return prefixedStable(s, "criterion:sha256:") }); e != nil {
			return m, e
		}
		if tasks {
			if c.TicketID, e = stableString(o, "ticketId", candidateTicket); e != nil {
				return m, e
			}
			if c.AcceptanceRevision, e = stableString(o, "acceptanceRevision", func(s string) bool {
				n, e := strconv.ParseUint(s, 10, 64)
				return len(s) <= 16 && e == nil && n > 0 && n <= maxWireInteger && strconv.FormatUint(n, 10) == s
			}); e != nil {
				return m, e
			}
		}
		var ok bool
		if c.CriterionIndex, ok = uint03(o["criterionIndex"]); !ok || c.CriterionIndex > 255 {
			return m, invalid("invalid-field")
		}
		digests := []struct {
			key  string
			dest *string
		}{{"criterionSha256", &c.CriterionSHA256}}
		if tasks {
			digests = []struct {
				key  string
				dest *string
			}{{"acceptanceSha256", &c.AcceptanceSHA256}, {"criterionSha256", &c.CriterionSHA256}, {"captureSha256", &c.CaptureSHA256}, {"verificationSha256", &c.VerificationSHA256}, {"claimTicketSha256", &c.ClaimTicketSHA256}, {"snapshotHeadReceiptSha256", &c.SnapshotHeadReceiptSHA256}, {"snapshotHeadArtifactSha256", &c.SnapshotHeadArtifactSHA256}}
		}
		for _, v := range digests {
			if *v.dest, e = stableString(o, v.key, validDigest); e != nil {
				return m, e
			}
		}
		if external {
			if c.ExternalRepository, e = stableString(o, "externalRepository", stableExternalRepositoryName); e != nil {
				return m, e
			}
			if c.ExternalBlobOID, e = stableString(o, "externalBlobOid", validOID); e != nil {
				return m, e
			}
			so, e := object03(o["externalSpan"], []string{"start", "end"}, nil)
			if e != nil {
				return m, e
			}
			start, okStart := uint03(so["start"])
			end, okEnd := uint03(so["end"])
			if !okStart || !okEnd || start >= end {
				return m, invalid("invalid-field")
			}
			c.ExternalSpan = &span{Start: start, End: end}
		} else if !tasks {
			if c.EvidenceID, e = stableString(o, "evidenceId", func(s string) bool { return prefixedStable(s, "evidence:sha256:") }); e != nil {
				return m, e
			}
		}
		m.CriterionBindings = append(m.CriterionBindings, c)
	}
	rs, e := list03(root["runnerReceipts"], 64)
	if e != nil {
		return m, e
	}
	for _, raw := range rs {
		o, e := object03(raw, []string{"sha256", "profile", "planSha256", "sourceGitBinding", "executionAuthority", "dependencyClosure", "authentication"}, nil)
		if e != nil {
			return m, e
		}
		v := stableRunner{}
		if v.SHA256, e = stableString(o, "sha256", validDigest); e != nil {
			return m, e
		}
		if v.Profile, e = stableString(o, "profile", func(s string) bool { return s == "corvint-test-runner-receipt/0" || s == "opaque/0" }); e != nil {
			return m, e
		}
		if v.PlanSHA256, e = stableString(o, "planSha256", validDigest); e != nil {
			return m, e
		}
		for _, f := range []struct {
			key, want string
			dest      *string
		}{{"sourceGitBinding", "NOT_OBSERVED", &v.SourceGitBinding}, {"executionAuthority", "CALLER_OBSERVED", &v.ExecutionAuthority}, {"dependencyClosure", "NOT_OBSERVED", &v.DependencyClosure}} {
			if *f.dest, e = stableString(o, f.key, func(s string) bool { return s == f.want }); e != nil {
				return m, e
			}
		}
		if v.Authentication, e = decodeStableAuthentication(o["authentication"]); e != nil {
			return m, e
		}
		m.RunnerReceipts = append(m.RunnerReceipts, v)
	}
	ls, e := list03(root["criterionLinks"], 1024)
	if e != nil {
		return m, e
	}
	for _, raw := range ls {
		o, e := object03(raw, []string{"criterionId", "hunkIds", "evidenceIds", "runnerReceiptSha256s"}, nil)
		if e != nil {
			return m, e
		}
		v := stableLink{}
		if v.CriterionID, e = stableString(o, "criterionId", func(s string) bool { return prefixedStable(s, "criterion:sha256:") }); e != nil {
			return m, e
		}
		if v.HunkIDs, e = stableIDs(o["hunkIds"], "hunk:sha256:"); e != nil {
			return m, e
		}
		if v.EvidenceIDs, e = stableIDs(o["evidenceIds"], "evidence:sha256:"); e != nil {
			return m, e
		}
		if v.RunnerReceiptSHA256s, e = stableIDs(o["runnerReceiptSha256s"], ""); e != nil {
			return m, e
		}
		m.CriterionLinks = append(m.CriterionLinks, v)
	}
	as, e := list03(root["artifacts"], 1024)
	if e != nil {
		return m, e
	}
	for _, raw := range as {
		o, e := object03(raw, []string{"kind", "path", "sha256"}, nil)
		if e != nil {
			return m, e
		}
		v := stableArtifact{}
		if v.Kind, e = stableString(o, "kind", func(s string) bool {
			switch s {
			case "tasks-capture", "tasks-verification", "tasks-claimed-ticket", "tasks-snapshot-head-receipt", "runner-receipt", "runner-plan", stableAttestationKind:
				return true
			}
			return false
		}); e != nil {
			return m, e
		}
		if v.Path, e = stableString(o, "path", func(s string) bool { return candidatePath(s) && s != sidecar03 }); e != nil {
			return m, e
		}
		if v.SHA256, e = stableString(o, "sha256", validDigest); e != nil {
			return m, e
		}
		m.Artifacts = append(m.Artifacts, v)
	}
	if m.Provenance, m.Role, e = decodeStableProvenanceRole(root); e != nil {
		return m, e
	}
	return m, nil
}

// stableEvidenceReferences applies the map-local wire checks that the inherited
// cem/0.3 algorithms keep for cem/1.0 (CEM-V1-228): a duplicate evidence ID, a
// basis naming no evidence record, a duplicate basis pair within one hunk and an
// uncited evidence record are all `invalid-field` at the wire stage, before any
// repository read.
func stableEvidenceReferences(m stableMap) *cemError {
	ids, cited := map[string]bool{}, map[string]bool{}
	for _, ev := range m.Evidence {
		if ids[ev.ID] {
			return invalid("invalid-field")
		}
		ids[ev.ID] = true
	}
	for _, h := range m.Hunks {
		pairs := map[string]bool{}
		for _, b := range h.Basis {
			key := b.EvidenceID + "\x00" + b.Relation
			if !ids[b.EvidenceID] || pairs[key] {
				return invalid("invalid-field")
			}
			pairs[key] = true
			cited[b.EvidenceID] = true
		}
	}
	if len(cited) != len(ids) {
		return invalid("invalid-field")
	}
	return nil
}

func validateStableReferences(m *stableMap) *cemError {
	criteria, tuples, groups, spans := map[string]bool{}, map[string]bool{}, map[string]string{}, map[string]bool{}
	for _, c := range m.CriterionBindings {
		raw, _ := json.Marshal(c)
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(raw, &fields)
		canonical, e := candidateCanonicalRecord(fields)
		if e != nil || c.ID != "criterion:sha256:"+shaHex(canonical) {
			return invalid("invalid-criterion-identity")
		}
		// Plain criteria form one unnamed group that no Tasks ticket can occupy.
		tuple := c.TicketID + ":" + c.AcceptanceRevision + ":" + strconv.FormatUint(c.CriterionIndex, 10)
		plain := c.TicketID == ""
		key := c.EvidenceID
		if c.ExternalSpan != nil {
			key = "external\x00" + c.ExternalRepository + "\x00" + c.ExternalBlobOID + "\x00" + strconv.FormatUint(c.ExternalSpan.Start, 10) + "\x00" + strconv.FormatUint(c.ExternalSpan.End, 10)
		}
		if criteria[c.ID] || tuples[tuple] || plain && spans[key] {
			return invalid("duplicate-reference")
		}
		spans[key] = plain
		criteria[c.ID] = true
		tuples[tuple] = true
		group := c.TicketID + ":" + c.AcceptanceRevision
		coherence := c.AcceptanceSHA256 + c.CaptureSHA256 + c.VerificationSHA256 + c.ClaimTicketSHA256 + c.SnapshotHeadReceiptSHA256 + c.SnapshotHeadArtifactSHA256
		if prior, exists := groups[group]; exists && prior != coherence {
			return invalid("reference-coherence")
		}
		groups[group] = coherence
	}
	runners := map[string]bool{}
	for _, r := range m.RunnerReceipts {
		if runners[r.SHA256] {
			return invalid("duplicate-reference")
		}
		runners[r.SHA256] = true
	}
	hunks := map[string]hunk03{}
	evidenceIDs := map[string]bool{}
	for _, h := range m.Hunks {
		if _, ok := hunks[h.ID]; ok {
			return invalid("duplicate-reference")
		}
		hunks[h.ID] = h
	}
	spanDigests := map[string]string{}
	for _, ev := range m.Evidence {
		evidenceIDs[ev.ID] = true
		spanDigests[ev.ID] = ev.SpanSHA256
	}
	// A plain criterion is its cited base span; canonical evidence checks bind
	// that span digest to the Git blob bytes.
	for _, c := range m.CriterionBindings {
		if c.EvidenceID == "" {
			continue
		}
		if !evidenceIDs[c.EvidenceID] {
			return invalid("unresolved-reference")
		}
		if spanDigests[c.EvidenceID] != c.CriterionSHA256 {
			return invalid("reference-coherence")
		}
	}
	linkedCriteria, linkedRunners := map[string]bool{}, map[string]bool{}
	for _, l := range m.CriterionLinks {
		if linkedCriteria[l.CriterionID] {
			return invalid("duplicate-reference")
		}
		if !criteria[l.CriterionID] {
			return invalid("unresolved-reference")
		}
		linkedCriteria[l.CriterionID] = true
		for _, a := range [][]string{l.HunkIDs, l.EvidenceIDs, l.RunnerReceiptSHA256s} {
			seen := map[string]bool{}
			for _, id := range a {
				if seen[id] {
					return invalid("duplicate-reference")
				}
				seen[id] = true
			}
		}
		for _, id := range l.HunkIDs {
			if _, exists := hunks[id]; !exists {
				return invalid("unresolved-reference")
			}
		}
		for _, id := range l.EvidenceIDs {
			if !evidenceIDs[id] {
				return invalid("unresolved-reference")
			}
		}
		for _, id := range l.RunnerReceiptSHA256s {
			if !runners[id] {
				return invalid("unresolved-reference")
			}
			linkedRunners[id] = true
		}
	}
	for _, l := range m.CriterionLinks {
		basisIDs := map[string]bool{}
		for _, id := range l.HunkIDs {
			for _, b := range hunks[id].Basis {
				basisIDs[b.EvidenceID] = true
			}
		}
		for _, id := range l.EvidenceIDs {
			if !basisIDs[id] {
				return invalid("reference-coherence")
			}
		}
	}
	for _, h := range m.Hunks {
		for _, b := range h.Basis {
			if !evidenceIDs[b.EvidenceID] {
				return invalid("unresolved-reference")
			}
		}
	}
	// CEM-V1-176: an unused criterion or runner fails coherence after
	// link/basis membership and before artifact role closure.
	if len(linkedCriteria) != len(criteria) || len(linkedRunners) != len(runners) {
		return invalid("reference-coherence")
	}
	required := map[string]string{}
	want := func(digest, kind string) bool {
		if prior, exists := required[digest]; exists && prior != kind {
			return false
		}
		required[digest] = kind
		return true
	}
	for _, c := range m.CriterionBindings {
		// Plain and external criteria (CEM-V1-332) carry no Tasks artifact role.
		if c.TicketID == "" {
			continue
		}
		if !want(c.CaptureSHA256, "tasks-capture") || !want(c.VerificationSHA256, "tasks-verification") || !want(c.ClaimTicketSHA256, "tasks-claimed-ticket") || !want(c.SnapshotHeadArtifactSHA256, "tasks-snapshot-head-receipt") {
			return invalid("artifact-role-closure")
		}
	}
	envelopes := map[string]bool{}
	for _, r := range m.RunnerReceipts {
		if !want(r.SHA256, "runner-receipt") || !want(r.PlanSHA256, "runner-plan") {
			return invalid("artifact-role-closure")
		}
		// CEM-V1-084: each signed receipt names its own receipt-attestation
		// artifact; an attestation no receipt names fails the closure below.
		if r.Authentication.dsse() {
			if envelopes[r.Authentication.EnvelopeSHA256] {
				return invalid("duplicate-reference")
			}
			envelopes[r.Authentication.EnvelopeSHA256] = true
			if !want(r.Authentication.EnvelopeSHA256, stableAttestationKind) {
				return invalid("artifact-role-closure")
			}
		}
	}
	paths, digests := map[string]bool{}, map[string]bool{}
	for _, a := range m.Artifacts {
		// CEM-V1-176: a repeated path or digest precedes the record's kind.
		if paths[a.Path] || digests[a.SHA256] {
			return invalid("duplicate-reference")
		}
		if required[a.SHA256] != a.Kind {
			return invalid("artifact-role-closure")
		}
		paths[a.Path] = true
		digests[a.SHA256] = true
	}
	if len(digests) != len(required) {
		return invalid("artifact-role-closure")
	}
	if len(m.CriterionBindings) == 0 && (len(m.RunnerReceipts) != 0 || len(m.CriterionLinks) != 0 || len(m.Artifacts) != 0) || len(m.CriterionBindings) > 0 && (len(m.RunnerReceipts) == 0 || len(m.CriterionLinks) == 0 || len(m.Artifacts) == 0) {
		return invalid("reference-coherence")
	}
	return nil
}

// Admit each directory through an O_NOFOLLOW descriptor first. A Root reopened
// for that component must identify the same directory before it can anchor the
// next read. The final leaf is also opened O_NOFOLLOW/nonblocking. This retains
// bounded rooted reads without claiming immunity to hostile same-user mutation.
func openStableArtifact(root, path string) (*os.File, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return nil, errNotRegular
	}
	if !filepath.IsAbs(root) || !candidatePath(path) || path == sidecar03 {
		return nil, errNotRegular
	}
	parts := append(strings.Split(strings.TrimPrefix(filepath.Clean(root), "/"), "/"), strings.Split(path, "/")...)
	parent, e := openStableTop(parts[0])
	if e != nil {
		return nil, e
	}
	for i, part := range parts {
		if i == 0 {
			continue
		}
		if part == "" || part == "." || part == ".." {
			parent.Close()
			return nil, errNotRegular
		}
		directory := i < len(parts)-1
		// Root may resolve a symlink internally even with O_NOFOLLOW. Admit
		// only an observed regular leaf/directory and bind both observations
		// to the opened descriptor; hostile same-user races are not qualified.
		prior, e := parent.Lstat(part)
		if e != nil || prior.Mode()&os.ModeSymlink != 0 || directory && !prior.IsDir() || !directory && !prior.Mode().IsRegular() {
			parent.Close()
			return nil, errNotRegular
		}
		f, e := openStableNoFollow(parent, part, directory)
		if e != nil {
			parent.Close()
			return nil, e
		}
		info, e := f.Stat()
		if e != nil {
			f.Close()
			parent.Close()
			return nil, e
		}
		if !directory {
			observed, oe := parent.Lstat(part)
			parent.Close()
			if oe != nil || !info.Mode().IsRegular() || !observed.Mode().IsRegular() || !os.SameFile(prior, info) || !os.SameFile(info, observed) {
				f.Close()
				return nil, errNotRegular
			}
			return f, nil
		}
		child, e := parent.OpenRoot(part)
		if e != nil {
			f.Close()
			parent.Close()
			return nil, e
		}
		opened, e1 := child.Stat(".")
		observed, e2 := parent.Lstat(part)
		if e1 != nil || e2 != nil || !info.IsDir() || !observed.IsDir() || !os.SameFile(prior, info) || !os.SameFile(info, opened) || !os.SameFile(info, observed) {
			child.Close()
			f.Close()
			parent.Close()
			return nil, errNotRegular
		}
		f.Close()
		parent.Close()
		parent = child
	}
	parent.Close()
	return nil, errNotRegular
}

// openStableTop opens the first component below / by its absolute path, with
// the same Lstat, open and SameFile admission every later directory gets, so /
// is opened for reading only when root is / itself: a sandbox such as Landlock
// cannot grant that read (V1-2365, AFP-V0-361).
func openStableTop(part string) (*os.Root, error) {
	if part == "" {
		return openStableRoot("/")
	}
	if part == "." || part == ".." {
		return nil, errNotRegular
	}
	name := stableTopPath(part)
	prior, e := os.Lstat(name)
	if e != nil || prior.Mode()&os.ModeSymlink != 0 || !prior.IsDir() {
		return nil, errNotRegular
	}
	top, e := openStableRoot(name)
	if e != nil {
		return nil, e
	}
	opened, e1 := top.Stat(".")
	observed, e2 := os.Lstat(name)
	if e1 != nil || e2 != nil || observed.Mode()&os.ModeSymlink != 0 || !observed.IsDir() || !os.SameFile(prior, opened) || !os.SameFile(prior, observed) {
		top.Close()
		return nil, errNotRegular
	}
	return top, nil
}

// openStableRoot is os.OpenRoot; a test observes the absolute names it opens.
var openStableRoot = os.OpenRoot

// stableTopPath is the absolute path of a first component; a test points it
// at a symlink or directory it owns.
var stableTopPath = func(part string) string { return "/" + part }

func readStableArtifact(ctx context.Context, root string, a stableArtifact) ([]byte, *cemError) {
	if ctx.Err() != nil {
		return nil, operational("verification-timeout")
	}
	f, e := openStableArtifact(root, a.Path)
	if e != nil {
		return nil, operational("artifact-unavailable")
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || info.Size() > maxJSONBytes {
		return nil, operational("artifact-resource-limit")
	}
	b, e := io.ReadAll(io.LimitReader(f, maxJSONBytes+1))
	if ctx.Err() != nil {
		return nil, operational("verification-timeout")
	}
	if e != nil {
		return nil, operational("artifact-unavailable")
	}
	if len(b) > maxJSONBytes {
		return nil, operational("artifact-resource-limit")
	}
	return b, nil
}
func checkStableArtifacts(ctx context.Context, root string, as []stableArtifact) ([]stableArtifactCheck, *cemError) {
	return checkStableArtifactReads(ctx, root, as, readStableArtifact)
}
func checkStableArtifactReads(ctx context.Context, root string, as []stableArtifact, read func(context.Context, string, stableArtifact) ([]byte, *cemError)) ([]stableArtifactCheck, *cemError) {
	checks, _, e := checkStableArtifactContents(ctx, root, as, read)
	return checks, e
}

// checkStableArtifactContents also returns the verified bytes by digest, which
// receipt authentication reads (CEM-V1-084).
func checkStableArtifactContents(ctx context.Context, root string, as []stableArtifact, read func(context.Context, string, stableArtifact) ([]byte, *cemError)) ([]stableArtifactCheck, map[string][]byte, *cemError) {
	checks := []stableArtifactCheck{}
	contents := map[string][]byte{}
	first := make([][]byte, len(as))
	total := 0
	for i, a := range as {
		b, e := read(ctx, root, a)
		if e != nil {
			return nil, nil, e
		}
		total += len(b)
		if total > 16<<20 {
			return nil, nil, operational("artifact-resource-limit")
		}
		if shaHex(b) != a.SHA256 {
			return nil, nil, invalid("artifact-digest-mismatch")
		}
		first[i] = b
	}
	total = 0
	for i, a := range as {
		b, e := read(ctx, root, a)
		if e != nil {
			return nil, nil, e
		}
		total += len(b)
		if total > 16<<20 {
			return nil, nil, operational("artifact-resource-limit")
		}
		if !bytes.Equal(b, first[i]) || shaHex(b) != a.SHA256 {
			return nil, nil, invalid("artifact-changed-during-verification")
		}
		checks = append(checks, stableArtifactCheck{a.Kind, a.Path, a.SHA256, len(b)})
		contents[a.SHA256] = b
	}
	return checks, contents, nil
}
func runStable(ctx context.Context, repo string, raw []byte, base, target, artifacts string) (stableResult, int) {
	return runStableTrusted(ctx, repo, raw, base, target, artifacts, nil)
}

// runStableTrusted is runStable with the consumer --trusted-keys set; nil
// means no file was supplied (CEM-V1-084).
func runStableTrusted(ctx context.Context, repo string, raw []byte, base, target, artifacts string, trusted map[string]ed25519.PublicKey) (stableResult, int) {
	return runStableExternal(ctx, repo, raw, base, target, artifacts, trusted, nil)
}

// runStableExternal is runStableTrusted with the consumer
// --external-repository roots by name (CEM-V1-333); nil supplies none.
func runStableExternal(ctx context.Context, repo string, raw []byte, base, target, artifacts string, trusted map[string]ed25519.PublicKey, externals map[string]string) (stableResult, int) {
	r := newStableResult(base, target, stableLegacyEnvelope)
	r.MapSHA256 = string03(shaHex(raw))
	m, e := decodeStable(raw)
	if e != nil {
		return failStable(&r, "wire", e, false)
	}
	r.Axes["coverageValidation"] = "ABSENT"
	r.Axes["discriminationValidation"] = "ABSENT"
	for _, h := range m.Hunks {
		if h.Coverage != nil {
			r.Axes["coverageValidation"] = "WIRE_VALIDATED_ONLY"
		}
		if h.Discriminates != nil {
			r.Axes["discriminationValidation"] = "WIRE_VALIDATED_ONLY"
		}
	}
	if e := validateStableReferences(&m); e != nil {
		return failStable(&r, "references", e, false)
	}
	r.Hunks = m.Hunks
	r.References = resultReferences(m.stableReferences)
	if base == "" {
		return failStable(&r, "authority-arguments", operational("expected-base-required"), false)
	}
	if target == "" {
		return failStable(&r, "authority-arguments", operational("target-required"), false)
	}
	if artifacts == "" {
		return failStable(&r, "authority-arguments", operational("artifacts-required"), false)
	}
	if !filepath.IsAbs(artifacts) {
		return failStable(&r, "authority-arguments", operational("invalid-arguments"), false)
	}
	if !validOID(base) || !validOID(target) || len(base) != len(target) {
		return failStable(&r, "authority-arguments", operational("invalid-arguments"), false)
	}
	for _, h := range m.Hunks {
		if structuralReason03(h.Reason) {
			r.Runtime.StructuralQualification = "QUALIFIED_EXACT_TUPLE"
			if !structuralRuntimeAdmitted(runtime.Version()) {
				r.Runtime.StructuralQualification = "UNSUPPORTED"
				r.Axes["structuralProof"] = "UNSUPPORTED_RUNTIME"
				return failStable(&r, "runtime", operational("unsupported-structural-runtime"), false)
			}
		}
	}
	if runtime.GOOS == "windows" {
		return failStable(&r, "runtime", operational("unsupported-process-containment"), false)
	}
	if e := repository03(repo); e != nil {
		return failStable(&r, "repository", e, false)
	}
	v := &verifier{ctx: ctx, repo: repo, blobs: map[string][]byte{}, trees: map[string]*treeEntry{}}
	before, e := v.snapshot03(base)
	if e != nil {
		return failStable(&r, "repository", e, false)
	}
	r.ExpectedBase = string03(base)
	r.Assurance = string03("structural-only")
	if m.BaseRevision != base {
		return failStable(&r, "binding", invalid("base-revision-mismatch"), true)
	}
	after, e := v.snapshot03(target)
	if e != nil {
		return failStable(&r, "repository", e, false)
	}
	r.TargetRevision = string03(target)
	if en := before[sidecar03]; en != nil && (en.typ != "blob" || en.mode != "100644") {
		return failStable(&r, "binding", invalid("excluded-path-not-file"), true)
	}
	r.Sidecar = "ABSENT"
	if en := after[sidecar03]; en != nil {
		r.Sidecar = "NOT_CHECKED"
		if en.typ != "blob" || en.mode != "100644" {
			r.Sidecar = "UNSUPPORTED_KIND"
			return failStable(&r, "binding", invalid("excluded-artifact-mismatch"), true)
		}
		b, e := v.blob03(en.oid)
		if e != nil {
			return failStable(&r, "repository", e, false)
		}
		if !bytes.Equal(b, raw) {
			r.Sidecar = "MISMATCH"
			return failStable(&r, "binding", invalid("excluded-artifact-mismatch"), true)
		}
		r.Sidecar = "EXACT"
	}
	patch, e := v.patch03(base, target)
	if e != nil {
		return failStable(&r, "repository", e, false)
	}
	r.PatchSHA256 = string03(shaHex(patch))
	r.Assurance = string03("canonical")
	if *r.PatchSHA256 != m.PatchSHA256 {
		return failStable(&r, "verification", invalid("patch-digest-mismatch"), true)
	}
	parsed, e := parsePatch(patch)
	if e != nil {
		return failStable(&r, "verification", e.stable(), true)
	}
	ph := map[string]*patchHunk{}
	groups := map[string]*filePatch{}
	for _, f := range parsed.files {
		for _, h := range f.hunks {
			ph[h.ID] = h
			groups[h.ID] = f
		}
	}
	if len(m.Hunks) != len(ph) {
		return failStable(&r, "verification", invalid("patch-hunk-count"), true)
	}
	seen := map[string]bool{}
	for i, h := range m.Hunks {
		p := ph[h.ID]
		// CEM-V1-750: the map lists the patch hunks in patch order.
		if p == nil || seen[h.ID] || h.ID != parsed.hunks[i].ID {
			return failStable(&r, "verification", invalid("invalid-field"), true)
		}
		seen[h.ID] = true
		path := p.OldPath
		if p.NewPath != nil {
			path = p.NewPath
		}
		if path == nil || h.Path != *path || h.OldRange != p.OldRange || h.NewRange != p.NewRange {
			return failStable(&r, "verification", invalid("invalid-field"), true)
		}
	}
	// Prove the Git change inventory independently, including changes which the
	// textual diff might omit. The sidecar is its one fixed recursion exclusion.
	changed := map[string]bool{}
	all := map[string]bool{}
	for p := range before {
		all[p] = true
	}
	for p := range after {
		all[p] = true
	}
	for p := range all {
		if p == sidecar03 {
			continue
		}
		a, b := before[p], after[p]
		if a != nil && a.typ == "tree" || b != nil && b.typ == "tree" {
			continue
		}
		if (a == nil) != (b == nil) || a != nil && b != nil && (a.oid != b.oid || a.mode != b.mode) {
			changed[p] = true
		}
	}
	mapped := map[string]bool{}
	for _, f := range parsed.files {
		path := ""
		if f.oldPath != nil {
			path = *f.oldPath
		}
		if f.newPath != nil {
			if path != "" && path != *f.newPath {
				return failStable(&r, "verification", invalid("invalid-field"), true)
			}
			path = *f.newPath
		}
		a, b := before[path], after[path]
		if !changed[path] || mapped[path] || (f.oldPath == nil) != (a == nil) || (f.newPath == nil) != (b == nil) {
			return failStable(&r, "verification", invalid("invalid-field"), true)
		}
		mapped[path] = true
		if f.binary {
			if e := binaryEntriesMatch(f, a, b); e != nil {
				return failStable(&r, stableStage(e), e, !e.operational)
			}
			continue
		}
		var old, new []byte
		for _, x := range []struct {
			entry *treeEntry
			dest  *[]byte
		}{{a, &old}, {b, &new}} {
			if x.entry == nil {
				continue
			}
			if !regularFile(x.entry) {
				return failStable(&r, "repository", operational("unsupported-tree-mode"), false)
			}
			data, e := v.blob03(x.entry.oid)
			if e != nil {
				return failStable(&r, "repository", e, false)
			}
			*x.dest = data
		}
		if f.oldPath == nil && b != nil && f.mode != b.mode || f.newPath == nil && a != nil && f.mode != a.mode {
			return failStable(&r, "verification", invalid("diff-metadata-mismatch"), true)
		}
		got, e := simulate(ctx, old, f.hunks)
		if e != nil {
			return failStable(&r, "verification", e, true)
		}
		if !bytes.Equal(got, new) {
			return failStable(&r, "verification", invalid("invalid-field"), true)
		}
	}
	if len(mapped) != len(changed) {
		return failStable(&r, "repository", operational("unsupported-patch-inventory"), false)
	}
	evidenceIDs := map[string]bool{}
	for i := range m.Evidence {
		item := &m.Evidence[i]
		if evidenceIDs[item.ID] {
			return failStable(&r, "verification", invalid("invalid-field"), true)
		}
		evidenceIDs[item.ID] = true
		en := before[item.Path]
		if en == nil || !regularFile(en) || en.oid != item.BlobOID {
			return failStable(&r, "verification", invalid("evidence-unavailable"), true)
		}
		data, e := v.blob03(en.oid)
		if e != nil {
			return failStable(&r, "repository", e, false)
		}
		if item.Span.End <= item.Span.Start || item.Span.End > uint64(len(data)) {
			return failStable(&r, "verification", invalid("evidence-unavailable"), true)
		}
		// A window into the cached blob (CEM-V1-690).
		item.data = data[item.Span.Start:item.Span.End:item.Span.End]
		if shaHex(item.data) != item.SpanSHA256 || "evidence:sha256:"+shaHex(canonicalEvidence(*item)) != item.ID {
			return failStable(&r, "verification", invalid("fabricated-evidence-id"), true)
		}
	}
	cited := map[string]bool{}
	for _, h := range m.Hunks {
		pairs := map[string]bool{}
		for _, b := range h.Basis {
			key := b.EvidenceID + "\x00" + b.Relation
			if !evidenceIDs[b.EvidenceID] || pairs[key] {
				return failStable(&r, "verification", invalid("invalid-field"), true)
			}
			pairs[key] = true
			cited[b.EvidenceID] = true
		}
	}
	for id := range evidenceIDs {
		if !cited[id] {
			return failStable(&r, "verification", invalid("invalid-field"), true)
		}
	}
	mechanical := false
	for _, h := range m.Hunks {
		if h.Disposition != "mechanical" {
			continue
		}
		mechanical = true
		p := ph[h.ID]
		proven := false
		if structuralReason03(h.Reason) {
			f := groups[h.ID]
			if f.oldPath != nil {
				en := before[*f.oldPath]
				if en != nil && regularFile(en) {
					proven = structuralProof03(ctx, h.Path, v.blobs[en.oid], f, p, h.Reason)
				}
			}
		} else {
			proven = proveMechanical(p, h.Reason)
		}
		if ctx.Err() != nil {
			return failStable(&r, "verification", operational("verification-timeout"), false)
		}
		if !proven {
			return failStable(&r, "verification", invalid("unproven-mechanical"), true)
		}
	}
	r.Axes["structuralProof"] = "NOT_REQUIRED"
	if mechanical {
		r.Axes["structuralProof"] = "PROVEN_DECLARED_FILE_PREDICATES"
	}
	unsafe := false
	for _, ev := range m.Evidence {
		d := drift03{EvidenceID: ev.ID, Path: ev.Path, BaseBlobOID: ev.BlobOID, Status: "deleted"}
		en := after[ev.Path]
		if en != nil && en.typ == "blob" {
			if en.oid == ev.BlobOID {
				d.Status = "stable"
				d.TargetBlobOID = string03(en.oid)
				d.TargetSpan = &nullableSpan{ev.Span.Start, ev.Span.End}
			} else if regularFile(en) {
				data, e := v.blob03(en.oid)
				if e != nil {
					return failStable(&r, "repository", e, false)
				}
				d.TargetBlobOID = string03(en.oid)
				first, matches, err := firstTwoMatches(ctx, data, ev.data)
				if err != nil {
					return failStable(&r, "repository", operational("verification-timeout"), false)
				}
				switch matches {
				case 0:
					d.Status = "stale"
				case 1:
					d.Status = "relocated"
					d.TargetSpan = &nullableSpan{uint64(first), uint64(first + len(ev.data))}
				default:
					d.Status = "ambiguous"
				}
			}
		}
		if d.Status != "stable" && d.Status != "relocated" {
			unsafe = true
		}
		r.Drift = append(r.Drift, d)
	}
	sort.Slice(r.Drift, func(i, j int) bool { return r.Drift[i].EvidenceID < r.Drift[j].EvidenceID })
	if unsafe {
		return failStable(&r, "drift", invalid("evidence-drift"), true)
	}
	r.Axes["changeIntegrity"] = "VERIFIED"
	checks, contents, e := checkStableArtifactContents(ctx, artifacts, m.Artifacts, readStableArtifact)
	if e != nil {
		return failStable(&r, "artifacts", e, true)
	}
	if stableHasSignedReceipt(&m) {
		tree, e := v.commitTree03(target)
		if e != nil {
			return failStable(&r, "repository", e, false)
		}
		axis, e := stableAuthenticate(&m, contents, tree, trusted)
		if e != nil {
			return failStableAuthentication(&r, e)
		}
		r.Axes["authentication"] = axis
	}
	if e := checkStableExternal(ctx, v, m.CriterionBindings, externals); e != nil {
		return failStable(&r, "artifacts", e, true)
	}
	r.ArtifactChecks = checks
	r.Axes["referenceIntegrity"] = "REFERENCE_INTEGRITY_ONLY"
	if len(m.Artifacts) == 0 {
		r.Axes["referenceIntegrity"] = "EMPTY_REFERENCE_SET"
	}
	r.Citation = computeCitation(citationHunks(m.Hunks), ph, m.Evidence)
	yes := true
	r.Accept = &yes
	r.Outcome = "ACCEPT"
	r.Stage = "complete"
	return r, 0
}

func executeStableCLI(ctx context.Context, argv []string) (stableResult, int) {
	r, status, _, _ := executeStableCLIExternal(ctx, argv)
	return r, status
}

// executeStableCLIExternal also returns the map bytes read and the supplied
// external repository roots, for the CEM-V1-336 stderr fix line.
func executeStableCLIExternal(ctx context.Context, argv []string) (stableResult, int, []byte, map[string]string) {
	flags := map[string]string{}
	externals := map[string]string{}
	allowed := map[string]bool{"--repository": true, "--map": true, "--expected-base": true, "--target": true, "--artifacts": true, "--trusted-keys": true}
	bad := len(argv)%2 != 0
	if !bad {
		for i := 0; i < len(argv); i += 2 {
			if argv[i] == "--external-repository" {
				name, root, ok := strings.Cut(argv[i+1], "=")
				if _, dup := externals[name]; !ok || dup || !stableExternalRepositoryName(name) || !filepath.IsAbs(root) || filepath.Clean(root) != root {
					bad = true
					break
				}
				externals[name] = root
				continue
			}
			if !allowed[argv[i]] || flags[argv[i]] != "" || argv[i+1] == "" {
				bad = true
				break
			}
			flags[argv[i]] = argv[i+1]
		}
	}
	r := newStableResult(flags["--expected-base"], flags["--target"], stableLegacyEnvelope)
	if bad || flags["--repository"] == "" || flags["--map"] == "" || !filepath.IsAbs(flags["--repository"]) || !filepath.IsAbs(flags["--map"]) {
		res, status := failStable(&r, "arguments", operational("invalid-arguments"), false)
		return res, status, nil, externals
	}
	trusted, ok := readStableTrustedKeys(ctx, flags["--trusted-keys"])
	if !ok {
		res, status := failStable(&r, "arguments", operational("invalid-arguments"), false)
		return res, status, nil, externals
	}
	raw, e := readBounded(ctx, flags["--map"], maxJSONBytes)
	if e != nil {
		res, status := failStable(&r, "input", operational("map-unavailable"), false)
		return res, status, nil, externals
	}
	res, status := runStableExternal(ctx, flags["--repository"], raw, flags["--expected-base"], flags["--target"], flags["--artifacts"], trusted, externals)
	return res, status, raw, externals
}
func runStableCLI(argv []string) {
	ctx, cancel := context.WithTimeout(context.Background(), verificationBudget)
	defer cancel()
	r, status, raw, externals := executeStableCLIExternal(ctx, argv)
	_ = json.NewEncoder(os.Stdout).Encode(r)
	if fix := stableRelativePathFix(r, argv); fix != "" {
		fmt.Fprintln(os.Stderr, "cem01-go verify-stable: "+fix)
	}
	if fix := stableExternalFix(r, raw, externals); fix != "" {
		fmt.Fprintln(os.Stderr, "cem01-go verify-stable: "+fix)
	}
	os.Exit(status)
}

// stableRelativePathFix names the recovery for an invalid-arguments refusal
// whose cause is a relative path option, in the native consumer's words
// (CEM-V1-117). It goes to stderr, never into the closed result; a relative
// path is refused, never resolved.
func stableRelativePathFix(r stableResult, argv []string) string {
	if r.Code == nil || *r.Code != "invalid-arguments" {
		return ""
	}
	var relative []string
	for _, name := range []string{"--repository", "--map", "--artifacts", "--trusted-keys", "--external-repository"} {
		for i := 0; i+1 < len(argv); i += 2 {
			value := argv[i+1]
			if name == "--external-repository" {
				_, value, _ = strings.Cut(value, "=")
			}
			if argv[i] == name && value != "" && !filepath.IsAbs(value) {
				relative = append(relative, name)
				break
			}
		}
	}
	if len(relative) == 0 {
		return ""
	}
	return "invalid-arguments: relative path option " + strings.Join(relative, ", ") + "; fix: pass each as an absolute path such as \"$PWD/<path>\"; a relative path is refused, never resolved"
}

// checkStableExternal observes each external criterion span (CEM-V1-334) in
// map order. An unsupplied, inadmissible or unreadable external repository is
// artifact-unavailable, so the verification abstains and never accepts; only
// observed bytes that differ from criterionSha256 reject. Every external read
// charges the primary verifier's Git-operation and distinct-blob budgets
// (CEM-V1-335).
func checkStableExternal(ctx context.Context, primary *verifier, criteria []stableCriterion, externals map[string]string) *cemError {
	for _, c := range criteria {
		if c.ExternalSpan == nil {
			continue
		}
		root, ok := externals[c.ExternalRepository]
		if !ok || repository03(root) != nil {
			return operational("artifact-unavailable")
		}
		v := &verifier{ctx: ctx, repo: root, gitOps: primary.gitOps, blobBytes: primary.blobBytes, blobs: map[string][]byte{}, trees: map[string]*treeEntry{}}
		b, e := v.blob03(c.ExternalBlobOID)
		primary.gitOps, primary.blobBytes = v.gitOps, v.blobBytes
		if ctx.Err() != nil {
			return operational("verification-timeout")
		}
		if e != nil {
			return operational("artifact-unavailable")
		}
		if c.ExternalSpan.End > uint64(len(b)) || shaHex(b[c.ExternalSpan.Start:c.ExternalSpan.End]) != c.CriterionSHA256 {
			return invalid("artifact-digest-mismatch")
		}
	}
	return nil
}

// stableExternalFix names every external repository the map binds that the
// consumer did not supply, after an artifact-unavailable abstention
// (CEM-V1-336). It goes to stderr, never into the closed result.
func stableExternalFix(r stableResult, raw []byte, externals map[string]string) string {
	if r.Code == nil || *r.Code != "artifact-unavailable" || r.Stage != "artifacts" || raw == nil {
		return ""
	}
	m, e := decodeStable(raw)
	if e != nil {
		return ""
	}
	missing := map[string]bool{}
	for _, c := range m.CriterionBindings {
		if _, ok := externals[c.ExternalRepository]; c.ExternalSpan != nil && !ok {
			missing[c.ExternalRepository] = true
		}
	}
	if len(missing) == 0 {
		return ""
	}
	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, name)
	}
	sort.Strings(names)
	return "artifact-unavailable: external repository " + strings.Join(names, ", ") + " not supplied; external criteria stay NOT_OBSERVED; fix: pass --external-repository NAME=/absolute/path for each"
}
