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
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

var identifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
var selector = regexp.MustCompile(`^#[A-Za-z][A-Za-z0-9_-]{0,63}$`)

func Digest(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Decode rejects duplicate keys, excessive depth, unknown fields and secret-shaped content.
func Decode(data []byte, out any) error {
	if len(data) > MaxBytes {
		return errors.New("flow input exceeds byte limit")
	}
	if secretscreen.MatchString(string(data)) {
		return errors.New("flow input contains secret-shaped data")
	}
	if _, err := wire.Parse(data); err != nil {
		return errors.New("invalid flow JSON")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return errors.New("invalid flow schema")
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing flow input")
	}
	return nil
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
	if err != nil || !safePath(filepath.ToSlash(rel)) {
		return "", errors.New("flow output must be under the repository root")
	}
	return filepath.ToSlash(rel), nil
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
	return !strings.HasPrefix(s, "../")
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

func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, errors.New("Git unavailable")
	}
	argv := append([]string{gitPath, "--no-optional-locks", "-c", "core.fsmonitor=false", "-C", root}, args...)
	o := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: root, Timeout: 10 * time.Second, OutputLimit: MaxBytes})
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
