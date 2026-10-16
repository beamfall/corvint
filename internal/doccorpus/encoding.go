package doccorpus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"io"
	"os"
	"path"
	"runtime"
	"runtime/debug"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

func Digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func Encode(value any) ([]byte, error) {
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
	if len(data) > limit {
		return fail("input bound exceeded")
	}
	if err := json.Unmarshal(data, target, json.RejectUnknownMembers(true)); err != nil {
		return fail("invalid closed JSON input")
	}
	return nil
}
func hashValue(value any) (string, error) {
	h := sha256.New()
	n, err := EncodeTo(h, value)
	if err != nil {
		return "", err
	}
	if n+1 > int64(encodingLimit(value)) {
		return "", fail("output bound exceeded")
	}
	h.Write([]byte{'\n'})
	return hex.EncodeToString(h.Sum(nil)), nil
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
	return Builder{Version: Schema, Revision: revision, Engine: "native-contextindex/" + Schema + "/" + runtime.Version()}
}
func ReadFile(root, path string) ([]byte, error) { return readFileBounded(root, path, MaxBytes) }

// ReadPlaywrightFile admits a caller-run Playwright listing or JSON report, which a multi-project
// suite can grow past MaxBytes; the producers still bound every record they emit.
func ReadPlaywrightFile(root, path string) ([]byte, error) {
	return readFileBounded(root, path, PlaywrightFileMaxBytes)
}

// ReadCorpusFile admits a bounded /2 manifest or artifact; its decoder still
// enforces the original limit for /1 and for independent receipt profiles.
func ReadCorpusFile(root, path string) ([]byte, error) {
	return readFileBounded(root, path, MaxCorpusBytes)
}
func readFileBounded(root, path string, limit int64) ([]byte, error) {
	if rule := LocalInputPathRule(path); rule != "" {
		return nil, fail("invalid local input path: input paths must be canonical and relative to the input root (" + rule + ")")
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
	err := decodeBounded(data, &m, MaxCorpusBytes)
	if err == nil && len(data) > encodingLimit(m) {
		err = fail("manifest input bound exceeded")
	}
	return m, err
}
func ParseArtifact(data []byte) (*Artifact, error) {
	var a Artifact
	if err := decodeBounded(data, &a, MaxCorpusBytes); err != nil {
		return nil, err
	}
	c := encodingComparer{want: data}
	if _, err := EncodeTo(&c, a); err != nil {
		return nil, err
	}
	if c.n+1 > int64(encodingLimit(a)) {
		return nil, fail("output bound exceeded")
	}
	if !c.equal() {
		return nil, fail("noncanonical artifact")
	}
	digest := a.SHA256
	a.SHA256 = ""
	expected, err := hashValue(a)
	if err != nil || digest != expected {
		return nil, fail("artifact digest mismatch")
	}
	a.SHA256 = digest
	if a.Schema != Schema && a.Schema != SchemaV2 {
		return nil, fail("unsupported artifact schema")
	}
	return &a, nil
}

// EncodeTo streams the Encode bytes of value, without the trailing newline, to w and returns
// how many it wrote. It applies no output bound. json.Marshal draws its output buffer from a
// sync.Pool, so its growth cost depended on whether a GC cycle or a P migration had emptied the
// pool (V1-1380); the streaming encoder only keeps a bounded 4 KiB working buffer. Wrapping w
// also hides a *bytes.Buffer from jsontext, whose bytes.Buffer path regrows the buffer whenever
// less than a quarter of its length is free and would defeat a pre-sized Grow.
func EncodeTo(w io.Writer, value any) (int64, error) {
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
