package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	specVersion        = "cem/0.1"
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

type cliArgs struct {
	repository string
	mapFile    string
	patchFile  string
	target     string
	targetSet  bool
}

type cemError struct {
	code        string
	operational bool
}

func (e *cemError) Error() string { return e.code }

func invalid(code string) *cemError     { return &cemError{code: code} }
func operational(code string) *cemError { return &cemError{code: code, operational: true} }

func parseArgs(argv []string) (cliArgs, *cemError) {
	var a cliArgs
	if len(argv) == 0 || argv[0] != "verify" {
		return a, operational("invocation")
	}
	seen := map[string]bool{}
	for i := 1; i < len(argv); i += 2 {
		if i+1 >= len(argv) || seen[argv[i]] {
			return a, operational("invocation")
		}
		seen[argv[i]] = true
		switch argv[i] {
		case "--repository":
			a.repository = argv[i+1]
		case "--map":
			a.mapFile = argv[i+1]
		case "--patch":
			a.patchFile = argv[i+1]
		case "--target":
			a.target = argv[i+1]
			a.targetSet = true
		default:
			return a, operational("invocation")
		}
	}
	if a.repository == "" || a.mapFile == "" || a.patchFile == "" {
		return a, operational("invocation")
	}
	if a.targetSet && !validOID(a.target) {
		return a, operational("invocation")
	}
	return a, nil
}

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

func verify(a cliArgs) ([]driftItem, *cemError) {
	ctx, cancel := context.WithTimeout(context.Background(), verificationBudget)
	defer cancel()

	mapBytes, err := readBounded(ctx, a.mapFile, maxJSONBytes)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, operational("verification-timeout")
		}
		if errors.Is(err, errSizeLimit) {
			return nil, invalid("resource")
		}
		return nil, operational("repository-io")
	}
	var m cemMap
	if err := strictDecode(mapBytes, &m); err != nil {
		return nil, invalid("invalid-json")
	}
	if err := validateRequiredShape(mapBytes); err != nil {
		return nil, invalid("version-shape")
	}
	if err := validateMapShape(&m); err != nil {
		return nil, err
	}
	patch, err := readBounded(ctx, a.patchFile, maxPatchBytes)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, operational("verification-timeout")
		}
		if errors.Is(err, errSizeLimit) {
			return nil, invalid("resource")
		}
		return nil, operational("repository-io")
	}
	if shaHex(patch) != m.PatchSHA256 {
		return nil, invalid("patch-digest")
	}
	parsed, parseErr := parsePatch(patch)
	if parseErr != nil {
		return nil, parseErr
	}
	v := &verifier{ctx: ctx, repo: a.repository, blobs: make(map[string][]byte), trees: make(map[string]*treeEntry)}
	if err := v.rejectAlternates(); err != nil {
		return nil, err
	}
	baseOID, gitErr := v.resolveCommit(m.BaseRevision)
	if gitErr != nil {
		return nil, gitErr
	}
	if baseOID != m.BaseRevision {
		return nil, invalid("base-revision")
	}
	if err := v.verifyPatchSimulation(baseOID, parsed); err != nil {
		return nil, err
	}
	if err := v.verifyEvidence(baseOID, &m); err != nil {
		return nil, err
	}
	if err := verifyHunkMap(&m, parsed); err != nil {
		return nil, err
	}
	if !a.targetSet {
		return []driftItem{}, nil
	}
	targetOID, gitErr := v.resolveCommit(a.target)
	if gitErr != nil {
		return nil, gitErr
	}
	if targetOID != a.target {
		return nil, invalid("target-revision")
	}
	return v.computeDrift(targetOID, m.Evidence)
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

func validateRequiredShape(raw []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return err
	}
	if !hasKeys(root, "spec", "baseRevision", "patchSha256", "evidence", "hunks") {
		return errors.New("missing top-level field")
	}
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

func strictDecode(raw []byte, dst any) error {
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
	dec = json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if dec.Decode(new(any)) != io.EOF {
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

func validateMapShape(m *cemMap) *cemError {
	if m.Spec != specVersion || !validOID(m.BaseRevision) || !validDigest(m.PatchSHA256) {
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
	if v.gitOps >= maxGitOps {
		return nil, invalid("resource")
	}
	v.gitOps++
	remaining := 10 * time.Second
	if deadline, ok := v.ctx.Deadline(); ok && time.Until(deadline) < remaining {
		remaining = time.Until(deadline)
	}
	if remaining <= 0 {
		return nil, operational("verification-timeout")
	}
	ctx, cancel := context.WithTimeout(v.ctx, remaining)
	defer cancel()
	base := []string{"--no-pager", "-c", "core.hooksPath=/dev/null", "-c", "core.attributesFile=/dev/null", "-c", "filter.lfs.smudge=", "-c", "filter.lfs.required=false", "-c", "protocol.allow=never", "-c", "credential.helper=", "-C", v.repo}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = append(cleanGitEnvironment(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, operational("git-timeout")
		}
		return nil, operational("repository-io")
	}
	return stdout.Bytes(), nil
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

func (v *verifier) resolveCommit(rev string) (string, *cemError) {
	if !validOID(rev) {
		return "", invalid("revision-shape")
	}
	b, err := v.git("rev-parse", "--verify", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	oid := strings.TrimSuffix(string(b), "\n")
	if !validOID(oid) || len(oid) != len(rev) {
		return "", operational("object-format")
	}
	return oid, nil
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
		e.data = append([]byte(nil), b[e.Span.Start:e.Span.End]...)
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
