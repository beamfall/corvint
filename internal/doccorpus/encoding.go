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
	"sync"
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
// canonical comparison encodes a, so neither waits for the other (DCP-V1-414).
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
// canonical encoding of a, without encoding a again (DCP-V1-414). a is only read.
func canonicalArtifactDigest(a *Artifact, data []byte) (string, error) {
	tail := hashArtifactTail(data)
	if err := tail.suffix(a, data); err != nil {
		return "", err
	}
	return tail.digest, nil
}

// artifactTail locates the sha256 member that ends canonical artifact bytes and hashes the
// bytes before it followed by an empty sha256 member: exactly the empty-field encoding hashValue
// hashes, because sha256 is the last member of the canonical object.
type artifactTail struct {
	at     int
	digest string
}

func hashArtifactTail(data []byte) artifactTail {
	at := bytes.LastIndex(data, []byte(`,"sha256":`))
	if at < 0 {
		return artifactTail{at: -1}
	}
	h := sha256.New()
	h.Write(data[:at])
	h.Write([]byte(`,"sha256":""}` + "\n"))
	return artifactTail{at, hex.EncodeToString(h.Sum(nil))}
}

// suffix refuses data whose bytes after the located member are not a's sha256 member and the
// closing brace; such data is not canonical.
func (t artifactTail) suffix(a *Artifact, data []byte) error {
	value, err := json.Marshal(a.SHA256)
	if err != nil {
		return err
	}
	if t.at < 0 || !bytes.Equal(data[t.at:], append(append([]byte(`,"sha256":`), value...), "}\n"...)) {
		return fail("noncanonical artifact")
	}
	return nil
}

// verify refuses a mismatched recorded digest or an unsupported schema.
func (t artifactTail) verify(a *Artifact, data []byte) error {
	if err := t.suffix(a, data); err != nil {
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
