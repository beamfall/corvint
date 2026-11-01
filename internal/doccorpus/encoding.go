package doccorpus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"reflect"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

func Digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func Encode(value any) ([]byte, error) {
	countArtifactEncode(value)
	data, err := json.Marshal(value, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	if len(data)+1 > encodingLimit(value) {
		return nil, fail("output bound exceeded")
	}
	return append(data, '\n'), nil
}
func decode(data []byte, target any) error {
	return decodeBounded(data, target, MaxBytes)
}
func decodeBounded(data []byte, target any, limit int) error {
	return decodeNamed("", data, target, limit)
}

// decodeNamed is decodeBounded for a caller-supplied input: a refusal keeps the "invalid closed
// JSON input" prefix and, when name is set, appends the input name, the JSON pointer of the first
// invalid value, the enclosing record id when one is readable, the reason and the byte offset
// (DCX-V1-470). An artifact keeps the bare message, which its streamed decoder must match.
func decodeNamed(name string, data []byte, target any, limit int, opts ...json.Options) error {
	if len(data) > limit {
		return fail("input bound exceeded")
	}
	if err := json.Unmarshal(data, target, append([]json.Options{json.RejectUnknownMembers(true)}, opts...)...); err != nil {
		if name == "" {
			return fail("invalid closed JSON input")
		}
		return fail("invalid closed JSON input: " + locateDecodeError(name, data, err))
	}
	return nil
}

// locateDecodeError renders where and why a closed decode failed. Every caller-derived part is
// bounded, so the message stays small whatever the input holds.
func locateDecodeError(name string, data []byte, err error) string {
	pointer, reason, offset := "", "invalid JSON", int64(-1)
	var semantic *json.SemanticError
	var syntactic *jsontext.SyntacticError
	var compact compactAnchorError
	switch {
	case errors.As(err, &semantic):
		pointer, offset = string(semantic.JSONPointer), semantic.ByteOffset
		switch {
		case errors.As(semantic.Err, &compact):
			reason = string(compact)
		case errors.Is(semantic.Err, json.ErrUnknownName):
			reason = "unknown member"
		case semantic.GoType != nil:
			reason = "expected " + jsonKindOf(semantic.GoType) + ", got " + semantic.JSONKind.String()
		}
	case errors.As(err, &syntactic):
		pointer, offset = string(syntactic.JSONPointer), syntactic.ByteOffset
		if syntactic.Err != nil {
			reason = syntactic.Err.Error()
		}
	}
	if pointer == "" {
		pointer = "/"
	}
	parts := []string{boundedText(name, 1024), boundedText(pointer, 512)}
	if semantic != nil {
		if id := recordIDAt(data, pointer); id != "" {
			parts = append(parts, "record "+id)
		}
	}
	located := strings.Join(parts, ": ") + ": " + boundedText(reason, 256)
	if offset >= 0 {
		located += " (offset " + strconv.FormatInt(offset, 10) + ")"
	}
	return located
}

// jsonKindOf names the JSON value a Go decode target expects.
func jsonKindOf(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct, reflect.Map:
		return "object"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return "number"
	}
	return "value"
}

// recordIDAt reads the string id of the record a JSON pointer of the form /member/index/... falls
// in, best effort: a member or element that is not there, or an id that is not a short printable
// string, yields "".
func recordIDAt(data []byte, pointer string) string {
	tokens := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	if len(tokens) < 2 {
		return ""
	}
	index, err := strconv.Atoi(tokens[1])
	if err != nil || index < 0 {
		return ""
	}
	var top map[string]jsontext.Value
	if json.Unmarshal(data, &top) != nil {
		return ""
	}
	member := strings.NewReplacer("~1", "/", "~0", "~").Replace(tokens[0])
	var records []jsontext.Value
	if json.Unmarshal(top[member], &records) != nil || index >= len(records) {
		return ""
	}
	var record struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(records[index], &record) != nil || record.ID == "" || len(record.ID) > 256 ||
		!utf8.ValidString(record.ID) || strings.IndexFunc(record.ID, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
		return ""
	}
	return record.ID
}

// boundedText cuts caller-derived text to at most limit bytes on a rune boundary.
func boundedText(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut] + "..."
}
func hashValue(value any) (string, error) {
	digest, _, err := hashSized(value)
	return digest, err
}

// hashSized is hashValue with the encoded size, newline included; on an output bound refusal
// it still returns the size, so a caller can name how far the bound was exceeded (DCS-V1-569).
func hashSized(value any) (string, int64, error) {
	h := sha256.New()
	n, err := EncodeTo(h, value)
	if err != nil {
		return "", 0, err
	}
	if n+1 > int64(encodingLimit(value)) {
		return "", n + 1, fail("output bound exceeded")
	}
	h.Write([]byte{'\n'})
	return hex.EncodeToString(h.Sum(nil)), n + 1, nil
}
func validPath(value string) bool { return LocalInputPathRule(value) == "" }

// LocalInputPathRule names the rule a local input path breaks, or returns "" for a canonical path
// relative to the input root (DCP-V1-054). The reader never admits absolute or parent paths.
func LocalInputPathRule(value string) string {
	switch {
	case value == "":
		return "got an empty path"
	case len(value) > 1024:
		return "got a path over 1024 bytes"
	case !utf8.ValidString(value):
		return "got a path that is not UTF-8"
	case strings.ContainsAny(value, "\\\x00\r\n\t"):
		return "got a backslash, NUL, tab, CR or LF"
	case strings.HasPrefix(value, "/"):
		return "got an absolute path"
	case value == ".." || strings.HasPrefix(value, "../"):
		return "got a parent traversal"
	case value == ".":
		return "got the root itself"
	case path.Clean(value) != value:
		return "got a non-canonical path"
	}
	return ""
}

func textOK(value string) bool {
	return len(value) > 0 && len(value) <= 64<<10 && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}
func currentBuilder() Builder {
	revision := "unrecorded"
	dirty := false
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.modified" {
				dirty = s.Value == "true"
			}
			if s.Key == "vcs.revision" {
				revision = s.Value
			}
		}
	}
	if dirty {
		revision += "+dirty"
	}
	return Builder{Version: Schema, Revision: revision, Engine: "native-contextindex/" + Schema + "/derivation-" + DerivationVersion + "/" + runtime.Version()}
}

// DerivationVersion names the derivation a build runs: every rule that turns the same pinned
// inputs into the artifact's records. A change that can alter the records built from the same
// inputs increments it; one that changes only how a read ranks, renders or bounds them does not.
// The verified read admits an artifact from another attested builder revision only while the
// version, the builder schema and the engine are equal (DCX-V1-471).
const DerivationVersion = "11"

// derivationOf reads the derivation version a builder's engine label records, or "" for an
// engine from before the version was recorded.
func derivationOf(b Builder) string {
	_, rest, ok := strings.Cut(b.Engine, "/derivation-")
	if !ok {
		return ""
	}
	version, _, _ := strings.Cut(rest, "/")
	return version
}

// sameDerivation admits a recorded builder for the verified read: the running builder itself, or
// another attested revision with the same builder schema, derivation version and engine. The
// refusal names what differs (DCX-V1-471).
func sameDerivation(recorded, running Builder) error {
	if recorded == running {
		return nil
	}
	named := func(v string) string {
		if v == "" {
			return "unrecorded"
		}
		return boundedText(v, 128)
	}
	var reason string
	switch {
	case derivationOf(recorded) != derivationOf(running):
		reason = "derivation version differs (artifact " + named(derivationOf(recorded)) + ", running builder " + named(derivationOf(running)) + ")"
	case recorded.Version != running.Version:
		reason = "builder schema differs (artifact " + named(recorded.Version) + ", running builder " + named(running.Version) + ")"
	case recorded.Engine != running.Engine:
		reason = "engine differs (artifact " + named(recorded.Engine) + ", running builder " + named(running.Engine) + ")"
	case !attestedBuilder(recorded):
		reason = "the artifact's builder revision is unrecorded, dirty or not a Git object ID"
	default:
		return nil
	}
	return errNotRederivable("builder differs: " + reason + "; " + RebuildHint)
}

// RebuildHint names the explicit write that refreshes an artifact the verified read refuses as
// built by another builder; the read itself never rebuilds (DCX-V1-497).
const RebuildHint = "rebuild the corpus from its embedded manifest with corvint docs corpus build --artifact <artifact> into a new file"

func ReadFile(root, path string) ([]byte, error) { return readFileBounded(root, path, MaxBytes) }

// ReadPlaywrightFile admits a caller-run Playwright listing or JSON report, which a multi-project
// suite can grow past MaxBytes; the producers still bound every record they emit.
func ReadPlaywrightFile(root, path string) ([]byte, error) {
	return readFileBounded(root, path, PlaywrightFileMaxBytes)
}

// ReadProviderFile admits one provider record or shard within its per-shard bound (DCX-V1-531).
func ReadProviderFile(root, path string) ([]byte, error) {
	return readFileBounded(root, path, MaxProviderBytes)
}

// ReadCorpusFile admits a bounded /2 manifest or artifact; its decoder still
// enforces the original limit for /1 and for independent receipt profiles.
func ReadCorpusFile(root, path string) ([]byte, error) {
	return readFileBounded(root, path, MaxCorpusBytes)
}

// StatCorpusFile is ReadCorpusFile without the read: the path rule and the
// no-follow walk of every directory component are the read's, and the leaf
// must be a regular file. It returns the leaf's lstat record, so a caller can
// tell an unchanged file without reading it (DCR-V1-417).
func StatCorpusFile(root, path string) (fs.FileInfo, error) {
	if rule := LocalInputPathRule(path); rule != "" {
		return nil, fail("invalid local input path: input paths must be canonical and relative to the input root (" + rule + ")")
	}
	if err := platformRefusal(); err != nil {
		return nil, err
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, fail("input root unavailable")
	}
	defer dir.Close()
	info, err := testvaliditydoc.StatFile(dir, path)
	if err != nil {
		return nil, &Error{Code: "corpus-input-unavailable", Message: "input missing, unsafe or over bound"}
	}
	return info, nil
}

// streamCorpusFile is ReadCorpusFile with read consuming the file instead of
// a copy of it (DCR-V1-431). A failure to open or read the file is the read's
// refusal; read's own refusal is returned as it is.
func streamCorpusFile(root, path string, read func(io.Reader) error) error {
	if rule := LocalInputPathRule(path); rule != "" {
		return fail("invalid local input path: input paths must be canonical and relative to the input root (" + rule + ")")
	}
	if err := platformRefusal(); err != nil {
		return err
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return fail("input root unavailable")
	}
	defer dir.Close()
	var refused error
	err = testvaliditydoc.StreamFileBounded(dir, path, MaxCorpusBytes, func(r io.Reader) error {
		reader := &failingReader{r: r}
		refused = read(reader)
		return reader.err
	})
	if err != nil {
		return &Error{Code: "corpus-input-unavailable", Message: "input missing, unsafe or over bound"}
	}
	return refused
}

// failingReader records the first read error other than the end of the input.
type failingReader struct {
	r   io.Reader
	err error
}

func (f *failingReader) Read(p []byte) (int, error) {
	n, err := f.r.Read(p)
	if err != nil && err != io.EOF && f.err == nil {
		f.err = err
	}
	return n, err
}

// PlatformUnsupportedCode is the typed refusal of a docs corpus input read on a
// platform whose receipt reader is not qualified: windows/amd64 and every
// platform other than darwin and linux (PRS-V1-022, decision 0559).
const PlatformUnsupportedCode = "corpus-platform-unsupported"

// receiptPlatformRefusal is testvaliditydoc.PlatformRefusal; tests force it.
var receiptPlatformRefusal = testvaliditydoc.PlatformRefusal

// platformRefusal refuses before any input is opened, so an unsupported
// platform never reports a missing or unsafe input in place of its real cause.
func platformRefusal() error {
	if receiptPlatformRefusal() == nil {
		return nil
	}
	return &Error{Code: PlatformUnsupportedCode, Message: "docs corpus reads are unsupported on this platform"}
}

func readFileBounded(root, path string, limit int64) ([]byte, error) {
	if rule := LocalInputPathRule(path); rule != "" {
		return nil, fail("invalid local input path: input paths must be canonical and relative to the input root (" + rule + ")")
	}
	if err := platformRefusal(); err != nil {
		return nil, err
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, fail("input root unavailable")
	}
	defer dir.Close()
	data, err := testvaliditydoc.ReadFileBounded(dir, path, limit)
	if err != nil {
		return nil, &Error{Code: "corpus-input-unavailable", Message: "input missing, unsafe or over bound"}
	}
	return data, nil
}
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	err := decodeNamed("manifest", data, &m, MaxCorpusBytes)
	if err == nil && len(data) > encodingLimit(m) {
		err = fail("manifest input bound exceeded")
	}
	return m, err
}
func ParseArtifact(data []byte) (*Artifact, error) {
	a, err := decodeArtifact(data)
	if err != nil {
		return nil, err
	}
	if err := verifyCanonical(a, data); err != nil {
		return nil, err
	}
	return a, nil
}

// decodeArtifact is ParseArtifact's bounded closed decode alone.
func decodeArtifact(data []byte) (*Artifact, error) {
	var a Artifact
	if err := decodeBounded(data, &a, MaxCorpusBytes); err != nil {
		return nil, err
	}
	return &a, nil
}

// verifyCanonical refuses data that is not exactly the canonical encoding of a, whose recorded
// digest differs, or whose schema is unsupported. The digest is hashed from data while the
// canonical comparison encodes a, so neither waits for the other (DCR-V1-414).
func verifyCanonical(a *Artifact, data []byte) error {
	var digest string
	var digestErr error
	var wg sync.WaitGroup
	wg.Go(func() { digest, digestErr = canonicalArtifactDigest(a, data) })
	c := encodingComparer{want: data}
	_, err := EncodeTo(&c, *a)
	wg.Wait()
	if err != nil {
		return err
	}
	if c.n+1 > int64(encodingLimit(*a)) {
		return fail("output bound exceeded")
	}
	if !c.equal() {
		return fail("noncanonical artifact")
	}
	if digestErr != nil || a.SHA256 != digest {
		return fail("artifact digest mismatch")
	}
	if a.Schema != Schema && a.Schema != SchemaV2 {
		return fail("unsupported artifact schema")
	}
	return nil
}

// canonicalArtifactDigest is hashValue of a with its sha256 field empty, computed from data, the
// canonical encoding of a, without encoding a again (DCR-V1-414). a is only read.
func canonicalArtifactDigest(a *Artifact, data []byte) (string, error) {
	tail := hashArtifactTail(data)
	if err := tail.suffix(a); err != nil {
		return "", err
	}
	return tail.digest, nil
}

// suffix refuses data whose bytes after the located member are not a's sha256 member and the
// closing brace; such data is not canonical.
func (t artifactTail) suffix(a *Artifact) error {
	value, err := json.Marshal(a.SHA256)
	if err != nil {
		return err
	}
	if t.tail == nil || !bytes.Equal(t.tail, append(append([]byte(`,"sha256":`), value...), "}\n"...)) {
		return fail("noncanonical artifact")
	}
	return nil
}

// verify refuses a mismatched recorded digest or an unsupported schema.
func (t artifactTail) verify(a *Artifact) error {
	if err := t.suffix(a); err != nil {
		return err
	}
	if a.SHA256 != t.digest {
		return fail("artifact digest mismatch")
	}
	if a.Schema != Schema && a.Schema != SchemaV2 {
		return fail("unsupported artifact schema")
	}
	return nil
}

// EncodeTo streams the Encode bytes of value, without the trailing newline, to w and returns
// how many it wrote. It applies no output bound. json.Marshal draws its output buffer from a
// sync.Pool, so its growth cost depended on whether a GC cycle or a P migration had emptied the
// pool (V1-1380). The streaming encoder never buffers the whole output: its working buffer
// starts at 4 KiB and grows only to hold the largest single token, such as one long string, so
// what still depends on the pool is bounded by that token, not by the output. Wrapping w
// also hides a *bytes.Buffer from jsontext, whose bytes.Buffer path regrows the buffer whenever
// less than a quarter of its length is free and would defeat a pre-sized Grow.
func EncodeTo(w io.Writer, value any) (int64, error) {
	countArtifactEncode(value)
	c := countingWriter{w: w}
	err := json.MarshalWrite(&c, value, json.Deterministic(true))
	return c.n, err
}

// EncodeSized returns exactly the Encode result, built in a buffer pre-sized to sizeHint bytes so
// that its allocation follows the output size instead of encoder pool state (V1-1380).
func EncodeSized(value any, sizeHint int) ([]byte, error) {
	var out bytes.Buffer
	out.Grow(max(sizeHint, 0))
	n, err := EncodeTo(&out, value)
	if err != nil {
		return nil, err
	}
	if n+1 > int64(encodingLimit(value)) {
		return nil, fail("output bound exceeded")
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// EncodedEqual reports whether value encodes, with the trailing newline, to exactly want,
// comparing while the encoder writes instead of building a second copy (V1-1380). It applies no
// output bound; a match is exactly len(want) bytes long.
func EncodedEqual(value any, want []byte) bool {
	c := encodingComparer{want: want}
	_, err := EncodeTo(&c, value)
	return err == nil && c.equal()
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// encodingComparer keeps counting after a mismatch so that callers can still apply the output
// bound to the full encoded length, as Encode did.
type encodingComparer struct {
	want   []byte
	n      int64
	differ bool
}

func (c *encodingComparer) Write(p []byte) (int, error) {
	if !c.differ && (c.n+int64(len(p)) > int64(len(c.want)) || !bytes.Equal(p, c.want[c.n:c.n+int64(len(p))])) {
		c.differ = true
	}
	c.n += int64(len(p))
	return len(p), nil
}
func (c *encodingComparer) equal() bool {
	return !c.differ && c.n+1 == int64(len(c.want)) && c.want[c.n] == '\n'
}
