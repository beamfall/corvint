package appflows

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

var identifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
var selector = regexp.MustCompile(`^#[A-Za-z][A-Za-z0-9_-]{0,63}$`)

func Digest(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Decode's refusals are distinct values so a caller can name the refusal class without echoing the
// input (AFU-V1-127); their messages are unchanged.
var (
	errDecodeBytes   = errors.New("flow input exceeds byte limit")
	errSecretShaped  = errors.New("flow input contains secret-shaped data")
	errInvalidJSON   = errors.New("invalid flow JSON")
	errInvalidSchema = errors.New("invalid flow schema")
	errTrailingInput = errors.New("trailing flow input")
)

// Decode rejects duplicate keys, excessive depth, unknown fields and secret-shaped content.
func Decode(data []byte, out any) error {
	return decode(data, out, nil)
}

// decode is Decode with an optional exemption: when the whole-text screen hits, exempt returns the
// text with identifier-only spans neutralized, and that text is screened instead (AFU-V1-126).
func decode(data []byte, out any, exempt func([]byte) string) error {
	if len(data) > MaxBytes {
		return errDecodeBytes
	}
	if screenSecrets(string(data)) && (exempt == nil || screenSecrets(exempt(data))) {
		return errSecretShaped
	}
	if _, err := wire.Parse(data); err != nil {
		return errInvalidJSON
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return errInvalidSchema
	}
	if d.Decode(new(any)) != io.EOF {
		return errTrailingInput
	}
	return nil
}

// flowTierKeyPlaceholder replaces an exempted flow_tiers member name before the second screen.
const flowTierKeyPlaceholder = `"flow-id"`

// maskFlowTierKeys returns data with each member name of the top-level flow_tiers object that is
// literally a quoted identifier (the flow ID shape, no escapes) replaced by a neutral name, so the
// secret screen reads the flow ID as the identifier it is rather than as a credential name. Every
// value, every other member name, and any flow_tiers name written with an escape or outside the
// identifier shape stay screened exactly as written (AFU-V1-126). Malformed JSON stops the walk;
// the parse that follows refuses it.
func maskFlowTierKeys(data []byte) string {
	type frame struct{ object, key, tiers bool }
	var stack []frame
	var masked strings.Builder
	last, topKey := 0, ""
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		end := int(d.InputOffset())
		// In an object awaiting a member name, a string token is that name; `}` is not.
		if name, ok := tok.(string); ok && len(stack) > 0 && stack[len(stack)-1].object && stack[len(stack)-1].key {
			n := len(stack)
			stack[n-1].key = false
			if n == 1 {
				topKey = name
			}
			if quoted := `"` + name + `"`; stack[n-1].tiers && identifier.MatchString(name) && end >= len(quoted) && string(data[end-len(quoted):end]) == quoted {
				masked.Write(data[last : end-len(quoted)])
				masked.WriteString(flowTierKeyPlaceholder)
				last = end
			}
			continue
		}
		switch tok {
		case json.Delim('{'):
			stack = append(stack, frame{object: true, key: true, tiers: len(stack) == 1 && topKey == "flow_tiers"})
			continue
		case json.Delim('['):
			stack = append(stack, frame{})
			continue
		case json.Delim('}'), json.Delim(']'):
			stack = stack[:len(stack)-1]
		}
		if n := len(stack); n > 0 && stack[n-1].object {
			stack[n-1].key = true
		}
	}
	masked.Write(data[last:])
	return masked.String()
}

// openInputFile is a test seam for a path swap between Lstat and open.
var openInputFile = openInput

// errInputBytes is ReadFile's byte-bound refusal, which a run-report ingest reports as its bound code.
var errInputBytes = errors.New("flow input exceeds byte limit")

// ReadFile reads a flow input only when Lstat shows a regular file before it is opened (AFU-V1-036).
func ReadFile(filename string) ([]byte, error) {
	before, err := os.Lstat(filename)
	if err != nil {
		return nil, errors.New("cannot open flow input")
	}
	if !before.Mode().IsRegular() {
		return nil, errors.New("flow input must be a regular file")
	}
	f, err := openInputFile(filename)
	if err != nil {
		return nil, errors.New("cannot open flow input")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.New("flow input changed while being read")
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if len(b) > MaxBytes {
		return nil, errInputBytes
	}
	return b, err
}

// WriteConfined exclusively creates filename under root and writes data, following no symlink
// inside or out of the root (AFU-V1-036).
func WriteConfined(root, filename string, data []byte) error {
	rel, err := confinedName(root, filename)
	if err != nil {
		return err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return errors.New("flow root unavailable")
	}
	defer r.Close()
	if err = realParents(r, rel); err != nil {
		return err
	}
	f, err := r.OpenFile(rel, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("cannot exclusively create flow output")
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = r.Remove(rel)
	}
	return err
}

func confinedName(root, filename string) (string, error) {
	abs, err := filepath.Abs(filename)
	if err != nil {
		return "", errors.New("invalid flow output path")
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", errors.New("flow output must be under the repository root")
	}
	rel = filepath.ToSlash(rel)
	if gitPath(rel) {
		return "", errors.New(gitPathRefused)
	}
	if !safePath(rel) {
		return "", errors.New("flow output must be under the repository root")
	}
	return rel, nil
}

func realParents(r *os.Root, rel string) error {
	for dir := path.Dir(rel); dir != "."; dir = path.Dir(dir) {
		st, err := r.Lstat(dir)
		if err != nil || !st.IsDir() {
			return errors.New("flow output parent must be a real directory")
		}
	}
	return nil
}

// readSource reads a root-confined source only when Lstat shows a regular file before a non-blocking
// open of the same file, so a FIFO or a swapped path is refused without blocking (AFU-V1-036).
func readSource(root, relative string) ([]byte, error) {
	if !safePath(relative) {
		return nil, errors.New("invalid source path")
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	before, err := r.Lstat(relative)
	if err != nil {
		return nil, errors.New("source unavailable")
	}
	if !before.Mode().IsRegular() {
		return nil, errors.New("source must be regular")
	}
	f, err := openRootInput(r, relative)
	if err != nil {
		return nil, errors.New("source unavailable")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.New("source changed while being read")
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if len(b) > MaxBytes {
		return nil, errors.New("source exceeds limit")
	}
	return b, err
}

func safePath(s string) bool {
	if s == "" {
		return false
	}
	if strings.ContainsAny(s, "\\\x00\n\r:") {
		return false
	}
	if strings.HasPrefix(s, "/") {
		return false
	}
	if path.Clean(s) != s {
		return false
	}
	if s == ".." {
		return false
	}
	if strings.HasPrefix(s, "../") {
		return false
	}
	return !gitPath(s)
}

// gitPathRefused is the named refusal for a path whose first component is .git, case-folded:
// repository metadata is never a valid flow write target (AFU-V1-036).
const gitPathRefused = "git-path-refused"

// gitPath reports whether s's first repository-relative component is .git, case-folded.
func gitPath(s string) bool {
	first, _, _ := strings.Cut(s, "/")
	return strings.EqualFold(first, ".git")
}

func localPath(p string) bool {
	if !strings.HasPrefix(p, "/") {
		return false
	}
	if strings.HasPrefix(p, "//") {
		return false
	}
	u, err := url.Parse(p)
	if err != nil {
		return false
	}
	if len(p) > 256 {
		return false
	}
	if strings.ContainsAny(p, "\\\r\n") {
		return false
	}
	return u.Host == "" && u.Scheme == "" && u.User == nil && u.Fragment == ""
}

func ValidateManifest(m Manifest) error {
	bad := errors.New("invalid or unsupported flow manifest")
	if m.Profile != IntentProfile {
		return bad
	}
	if !identifier.MatchString(m.Application) {
		return bad
	}
	u, err := url.Parse(m.Origin)
	if err != nil {
		return bad
	}
	if u.Scheme != "http" {
		return bad
	}
	if u.Hostname() != "127.0.0.1" {
		return bad
	}
	if u.Port() == "" {
		return bad
	}
	if u.String() != "http://"+u.Host {
		return bad
	}
	if len(m.Sources) < 1 || len(m.Sources)+len(m.Tests) > MaxSources {
		return bad
	}
	if !slices.Contains(m.Sources, m.BackendSource) {
		return bad
	}
	if !identifier.MatchString(m.Fixture) {
		return bad
	}
	if !localPath(m.IdentityPath) || !localPath(m.ResetPath) {
		return bad
	}
	if len(m.Server) < 1 || len(m.Server) > 16 {
		return bad
	}
	for _, arg := range m.Server {
		if len(arg) > 1024 || strings.ContainsRune(arg, 0) {
			return bad
		}
	}
	seen := map[string]bool{}
	for _, p := range append(slices.Clone(m.Sources), m.Tests...) {
		if !safePath(p) || seen[p] {
			return bad
		}
		seen[p] = true
	}
	if len(m.Scenarios) < 1 || len(m.Scenarios) > 25 {
		return bad
	}
	seen = map[string]bool{}
	roles := map[string]string{}
	actions := 0
	for _, s := range m.Scenarios {
		if role, ok := roles[s.Path]; ok && role != s.Role {
			return errors.New("conflicting roles for one route are unsupported")
		}
		roles[s.Path] = s.Role
		if seen[s.ID] {
			return bad
		}
		seen[s.ID] = true
		if err := validateScenario(s); err != nil {
			return err
		}
		actions += len(s.Actions)
	}
	if actions > 200 {
		return bad
	}
	return nil
}

func validateScenario(s Scenario) error {
	bad := errors.New("invalid or unsupported scenario")
	if !identifier.MatchString(s.ID) || !identifier.MatchString(s.Role) {
		return bad
	}
	if !localPath(s.Path) {
		return bad
	}
	if s.Basis != "declared" && s.Basis != "inferred" {
		return bad
	}
	if len(s.Actions) > 50 || len(s.Checks) < 1 || len(s.Checks) > 25 {
		return bad
	}
	for _, a := range s.Actions {
		if err := validateAction(a); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	for _, c := range s.Checks {
		if seen[c.ID] {
			return bad
		}
		seen[c.ID] = true
		if err := validateCheck(c); err != nil {
			return err
		}
	}
	return nil
}

func validateAction(a Action) error {
	bad := errors.New("unsupported flow action")
	if a.Kind == "reload" {
		if a.Selector != "" || a.Value != "" {
			return bad
		}
		return nil
	}
	if !selector.MatchString(a.Selector) {
		return bad
	}
	if a.Kind == "click" && a.Value == "" {
		return nil
	}
	if a.Kind == "fill" && len(a.Value) <= 128 {
		return nil
	}
	return bad
}

func validateCheck(c Check) error {
	bad := errors.New("unsupported flow check")
	if !identifier.MatchString(c.ID) {
		return bad
	}
	var v any
	if json.Unmarshal(c.Want, &v) != nil {
		return bad
	}
	if c.Kind == "json" {
		if !localPath(c.Path) || c.Selector != "" {
			return bad
		}
		if !strings.HasPrefix(c.Pointer, "/") || len(c.Pointer) > 128 {
			return bad
		}
		return scalar(v)
	}
	if c.Path != "" || c.Pointer != "" || !selector.MatchString(c.Selector) {
		return bad
	}
	switch c.Kind {
	case "text", "contains":
		s, ok := v.(string)
		if !ok || len(s) > 128 {
			return bad
		}
	case "count":
		n, ok := v.(float64)
		if !ok || n < 0 || n > 10000 || n != float64(int(n)) {
			return bad
		}
	case "visible":
		if v != true {
			return bad
		}
	default:
		return bad
	}
	return nil
}

func scalar(v any) error {
	switch n := v.(type) {
	case string:
		if len(n) <= 128 {
			return nil
		}
	case bool:
		return nil
	case float64:
		if n >= -10000 && n <= 10000 && n == float64(int(n)) {
			return nil
		}
	}
	return errors.New("only bounded scalar expectations are supported")
}

// runGit runs one Git read; tests replace it to force a process outcome.
var runGit = procgroup.Run

// errPlatformUnsupported reports a Git read refused by the platform, not by its input (PRS-V1-021,
// V1-2487); the message ends with the procgroup refusal so the platform-refusal list names it.
var errPlatformUnsupported = &gokernel.Error{Code: "process-platform-unsupported",
	Message: "flow Git source unavailable: process-platform-unsupported"}

func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	// The process-wide pinned Git: an MCP server pins it at start, so no call repeats a PATH lookup.
	gitPath := gitstatus.Executable()
	if !filepath.IsAbs(gitPath) {
		return nil, errors.New("Git unavailable")
	}
	// The gokernel environment and credential prefix keep a read from lazily fetching into a partial
	// clone's object store, so a flows read never writes the repository (invariant 4, MCPV0-017).
	argv := append([]string{gitPath, "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "credential.helper=", "-C", root}, args...)
	o := runGit(ctx, procgroup.Spec{Argv: argv, Dir: root, Env: gokernel.SanitizedGitEnvironment(), Timeout: 10 * time.Second, OutputLimit: MaxBytes})
	if errors.Is(o.Err, procgroup.ErrPlatformUnsupported) {
		return nil, errPlatformUnsupported
	}
	if o.Err != nil || o.ExitStatus != 0 {
		return nil, errors.New("flow Git source unavailable")
	}
	return o.Stdout, nil
}

// Capture rejects uncommitted declared inputs; unrelated working-tree edits confer no evidence.
func Capture(ctx context.Context, root, manifestPath string) (Input, error) {
	in := Input{Root: root}
	raw, err := readSource(root, manifestPath)
	if err != nil {
		return in, err
	}
	if err = Decode(raw, &in.Manifest); err != nil {
		return in, err
	}
	if err = ValidateManifest(in.Manifest); err != nil {
		return in, err
	}
	tree, err := git(ctx, root, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return in, err
	}
	in.Binding = Binding{Tree: strings.TrimSpace(string(tree)), ManifestDigest: Digest(raw), Sources: map[string]string{}, Fixture: in.Manifest.Fixture}
	paths := append(slices.Clone(in.Manifest.Sources), in.Manifest.Tests...)
	paths = append(paths, manifestPath)
	slices.Sort(paths)
	paths = slices.Compact(paths)
	total := 0
	for _, p := range paths {
		b, readErr := readSource(root, p)
		if readErr != nil {
			return in, readErr
		}
		pinned, gitErr := git(ctx, root, "show", in.Binding.Tree+":"+p)
		if gitErr != nil {
			return in, gitErr
		}
		if !bytes.Equal(b, pinned) {
			return in, errors.New("declared flow source is dirty")
		}
		total += len(b)
		if total > MaxBytes {
			return in, errors.New("source budget exceeded")
		}
		if secretscreen.MatchString(string(b)) {
			return in, errors.New("declared source contains secret-shaped data")
		}
		in.Binding.Sources[p] = Digest(b)
		if slices.Contains(in.Manifest.Tests, p) {
			in.Sources = append(in.Sources, Source{Path: p, Text: string(b)})
		}
	}
	in.Binding.FrontendDigest = buildDigest(in.Manifest.Sources, in.Binding.Sources)
	in.Binding.BackendDigest = in.Binding.Sources[in.Manifest.BackendSource]
	return in, nil
}

func buildDigest(paths []string, hashes map[string]string) string {
	paths = slices.Clone(paths)
	slices.Sort(paths)
	var b strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&b, "%s\x00%s\n", p, hashes[p])
	}
	return Digest([]byte(b.String()))
}

func ValueDigest(v json.RawMessage) string {
	var value any
	if json.Unmarshal(v, &value) != nil {
		return ""
	}
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(value)
	return Digest(bytes.TrimSuffix(b.Bytes(), []byte{'\n'}))
}

// screenSecrets is Decode's secret screen; a test counts its calls (V1-2164).
var screenSecrets = secretscreen.MatchString

// decodeElement strictly decodes one element of a document that Decode has already bounded,
// secret-screened and parsed, so a custom UnmarshalJSON does not repeat those whole-input checks
// for every element (V1-2164). It is valid only inside a Decode call.
func decodeElement(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return errors.New("invalid flow schema")
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing flow input")
	}
	return nil
}
