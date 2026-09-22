package main

import (
	"bytes"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/procgroup"
)

const inputLimit = 64 << 10
const outputLimit = 1 << 20
const fixtureEntryLimit = 100
const fixtureByteLimit = 10 << 20

type manifest struct {
	Partition           string `json:"partition"`
	Source              string `json:"source"`
	ComparisonPolicyRef string `json:"comparison_policy_ref"`
	ResourceProfileRef  string `json:"resource_profile_ref"`
	BaselineRef         string `json:"baseline_ref"`
	PreregistrationRef  string `json:"preregistration_ref"`
	Tasks               []task `json:"tasks"`
}
type task struct {
	ID                       string    `json:"id"`
	Kind                     string    `json:"kind"`
	Repo                     string    `json:"repo"`
	BaseCommit               string    `json:"base_commit"`
	Snapshot                 string    `json:"snapshot"`
	SnapshotSHA256           string    `json:"snapshot_sha256"`
	Source                   string    `json:"source"`
	Old                      version   `json:"old"`
	New                      version   `json:"new"`
	Argv                     []string  `json:"argv"`
	Stdin                    reference `json:"stdin"`
	Cwd                      string    `json:"cwd"`
	Env                      []string  `json:"env"`
	AcceptedIntentRef        string    `json:"accepted_intent_ref"`
	HistoricalObservationRef string    `json:"historical_observation_ref"`
	ExpectedUnknowns         []string  `json:"expected_unknowns"`
	ExpectedEffects          []effect  `json:"expected_effects"`
	ForbiddenClaims          []string  `json:"forbidden_claims"`
	GoldRef                  string    `json:"gold_ref"`
}
type reference struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type version struct {
	Commit           string        `json:"commit"`
	Executable       string        `json:"executable"`
	ExecutableSHA256 string        `json:"executable_sha256"`
	BuildIdentity    buildIdentity `json:"build_identity"`
}
type buildIdentity struct {
	GoVersion  string   `json:"go_version"`
	GOOS       string   `json:"GOOS"`
	GOARCH     string   `json:"GOARCH"`
	CGOEnabled string   `json:"CGO_ENABLED"`
	BuildFlags []string `json:"build_flags"`
}
type effect struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}
type resourceProfile struct {
	HostProfile          string                     `json:"host_profile"`
	Repetitions          int                        `json:"repetitions"`
	InputBytes           int                        `json:"input_bytes"`
	OutputBytesPerStream int                        `json:"output_bytes_per_stream"`
	FixtureEntries       int                        `json:"fixture_entries"`
	FixtureBytes         int                        `json:"fixture_bytes"`
	ProcessTimeout       string                     `json:"process_timeout"`
	RequestTimeout       string                     `json:"request_timeout"`
	BuildBounds          map[string]json.RawMessage `json:"build_bounds"`
	AllowedEffects       []effect                   `json:"allowed_effects"`
}
type gold struct {
	Label      string `json:"label"`
	Critical   bool   `json:"critical"`
	Provenance string `json:"provenance"`
}
type fixtureRegistryEntry struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}
type fixtureRegistry []fixtureRegistryEntry

type preparedTask struct {
	Task     task
	Input    []byte
	Snapshot []byte
	Old      []byte
	New      []byte
	OldMode  os.FileMode
	NewMode  os.FileMode
	Refusal  procgroup.PrelaunchRefusal
	Detail   string
}
type preparedManifest struct {
	Manifest manifest
	Profile  resourceProfile
	Tasks    []preparedTask
	SHA256   string
}

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// Decode the token tree first: encoding/json alone accepts duplicate keys and
// silently zeroes omitted fields. All struct keys, including nested ones, are exact.
func strictDecode(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	value, err := jsonValue(dec)
	if err != nil {
		return err
	}
	if _, err = dec.Token(); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	if err = exactShape(value, reflect.TypeOf(dst).Elem()); err != nil {
		return err
	}
	dec = json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
func jsonValue(dec *json.Decoder) (any, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		values := map[string]any{}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return nil, err
			}
			name := key.(string)
			if _, ok := values[name]; ok {
				return nil, fmt.Errorf("duplicate field %q", name)
			}
			v, err := jsonValue(dec)
			if err != nil {
				return nil, err
			}
			values[name] = v
		}
		_, err = dec.Token()
		return values, err
	case json.Delim('['):
		values := []any{}
		for dec.More() {
			v, err := jsonValue(dec)
			if err != nil {
				return nil, err
			}
			values = append(values, v)
		}
		_, err = dec.Token()
		return values, err
	default:
		return token, nil
	}
}
func exactShape(v any, t reflect.Type) error {
	if v == nil {
		return errors.New("null is not a declared field value")
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return errors.New("expected object")
		}
		if len(m) != t.NumField() {
			return fmt.Errorf("wrong fields for %s", t.Name())
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := f.Tag.Get("json")
			x, ok := m[name]
			if !ok {
				return fmt.Errorf("missing %s", name)
			}
			if err := exactShape(x, f.Type); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	case reflect.Slice:
		a, ok := v.([]any)
		if !ok {
			return errors.New("expected array")
		}
		for _, x := range a {
			if err := exactShape(x, t.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}
func cleanRelative(p string, allowDot bool) bool {
	if p == "" || strings.ContainsAny(p, "\x00\\") || strings.Contains(p, ":") || filepath.IsAbs(p) {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return false
		}
	}
	if p == "." {
		return allowDot
	}
	return filepath.ToSlash(filepath.Clean(p)) == p
}
func readReference(base string, ref reference, limit int) ([]byte, error) {
	if ref.Path == "" || !digestPattern.MatchString(ref.SHA256) {
		return nil, errors.New("invalid reference")
	}
	p := ref.Path
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	info, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("reference must be a regular file")
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > limit {
		return nil, errResourceBound
	}
	if digest(b) != ref.SHA256 {
		return nil, errDigestMismatch
	}
	return b, nil
}

var errDigestMismatch = errors.New("digest mismatch")
var errResourceBound = errors.New("resource bound exceeded")

func (registry fixtureRegistry) contains(sha string) bool {
	for _, r := range registry {
		if r.SHA256 == sha {
			return true
		}
	}
	return false
}
func loadManifest(path string, registry fixtureRegistry) (preparedManifest, error) {
	var p preparedManifest
	data, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	p.SHA256 = digest(data)
	if err = strictDecode(data, &p.Manifest); err != nil {
		return p, err
	}
	m := p.Manifest
	if m.Source == "" || len(m.Tasks) == 0 {
		return p, errors.New("source and tasks required")
	}
	if m.Partition != "pilot" && m.Partition != "development" && m.Partition != "heldout" {
		return p, errors.New("invalid partition")
	}
	base := filepath.Dir(path)
	profileData, err := os.ReadFile(resolveRef(base, m.ResourceProfileRef))
	if err != nil {
		return p, err
	}
	if err = strictDecode(profileData, &p.Profile); err != nil {
		return p, err
	}
	r := p.Profile
	if r.HostProfile != "owned-synthetic-process-group" {
		return p, errors.New("unqualified host profile")
	}
	if r.Repetitions != 3 || r.InputBytes != inputLimit || r.OutputBytesPerStream != outputLimit || r.FixtureEntries != fixtureEntryLimit || r.FixtureBytes != fixtureByteLimit || r.ProcessTimeout != "10s" || r.RequestTimeout != "120s" {
		return p, errors.New("resource profile must carry the frozen limits")
	}
	for _, e := range r.AllowedEffects {
		if !validEffect(e) {
			return p, errors.New("invalid allowed effect")
		}
	}
	ids := map[string]bool{}
	for _, t := range m.Tasks {
		if ids[t.ID] {
			return p, errors.New("duplicate task ID")
		}
		ids[t.ID] = true
		pt := prepareTask(base, t, r, registry)
		p.Tasks = append(p.Tasks, pt)
	}
	return p, nil
}
func resolveRef(base, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}
func validEffect(e effect) bool {
	return cleanRelative(e.Path, false) && (e.Kind == "create" || e.Kind == "write" || e.Kind == "delete" || e.Kind == "mode")
}
func prepareTask(base string, t task, r resourceProfile, registry fixtureRegistry) (p preparedTask) {
	p.Task = t
	p.Refusal = procgroup.RefusalDescriptorInvalid
	fail := func(err error) preparedTask {
		p.Detail = err.Error()
		if errors.Is(err, errDigestMismatch) {
			p.Refusal = procgroup.RefusalDigestMismatch
		}
		if errors.Is(err, errResourceBound) {
			p.Refusal = procgroup.RefusalResourceBound
		}
		return p
	}
	if t.ID == "" || t.Source == "" || !cleanRelative(t.Cwd, true) {
		return fail(errors.New("id, source and scratch-relative cwd required"))
	}
	if !reflect.DeepEqual(t.Old.BuildIdentity, t.New.BuildIdentity) {
		return fail(errors.New("unequal build identities"))
	}
	b := t.Old.BuildIdentity
	if b.GoVersion != "go1.27.1" || b.GOOS == "" || b.GOARCH == "" || (b.CGOEnabled != "0" && b.CGOEnabled != "1") || !contains(b.BuildFlags, "-trimpath") {
		return fail(errors.New("invalid build identity"))
	}
	if !contains(t.Env, "TZ=UTC") || !contains(t.Env, "LC_ALL=C") {
		return fail(errors.New("determinism environment missing"))
	}
	names := map[string]bool{}
	for _, entry := range t.Env {
		k, _, ok := strings.Cut(entry, "=")
		if !ok || k == "" || names[k] || strings.ContainsRune(entry, 0) {
			return fail(errors.New("invalid environment"))
		}
		names[k] = true
	}
	for _, e := range t.ExpectedEffects {
		if !validEffect(e) || !containsEffect(r.AllowedEffects, e) {
			return fail(errors.New("invalid or unpermitted effect"))
		}
	}
	if err := validateFixtureArgs(t.Argv); err != nil {
		return fail(err)
	}
	var g gold
	raw, err := os.ReadFile(resolveRef(base, t.GoldRef))
	if err != nil {
		return fail(err)
	}
	if err = strictDecode(raw, &g); err != nil {
		return fail(err)
	}
	// A lexically malformed digest is descriptor-invalid, never fixture-not-registered:
	// CRR-V0-001(b)'s syntax check runs before (g)'s registry lookup, which runs before
	// (c)'s content check.
	registered := []string{t.Old.ExecutableSHA256, t.New.ExecutableSHA256, t.Stdin.SHA256}
	for _, sha := range registered {
		if !digestPattern.MatchString(sha) {
			return fail(errors.New("malformed digest"))
		}
	}
	for _, sha := range registered {
		if !registry.contains(sha) {
			p.Refusal = procgroup.RefusalFixtureNotRegistered
			p.Detail = "fixture not registered"
			return p
		}
	}
	p.Input, err = readReference(base, t.Stdin, inputLimit)
	if err != nil {
		return fail(err)
	}
	p.Old, err = readReference(base, reference{t.Old.Executable, t.Old.ExecutableSHA256}, 128<<20)
	if err != nil {
		return fail(err)
	}
	p.New, err = readReference(base, reference{t.New.Executable, t.New.ExecutableSHA256}, 128<<20)
	if err != nil {
		return fail(err)
	}
	for _, v := range []struct {
		version version
		binary  []byte
	}{{t.Old, p.Old}, {t.New, p.New}} {
		actual, err := nativeBuildIdentity(v.binary)
		if err != nil {
			return fail(err)
		}
		declared := v.version.BuildIdentity
		if actual.GoVersion != declared.GoVersion || actual.GOOS != declared.GOOS || actual.GOARCH != declared.GOARCH || actual.CGOEnabled != declared.CGOEnabled {
			return fail(errors.New("binary build identity mismatch"))
		}
	}
	oldInfo, err := os.Stat(resolveRef(base, t.Old.Executable))
	if err != nil {
		return fail(err)
	}
	newInfo, err := os.Stat(resolveRef(base, t.New.Executable))
	if err != nil {
		return fail(err)
	}
	p.OldMode = oldInfo.Mode().Perm()
	p.NewMode = newInfo.Mode().Perm()
	p.Snapshot, err = readReference(base, reference{t.Snapshot, t.SnapshotSHA256}, fixtureByteLimit+1<<20)
	if err != nil {
		return fail(err)
	}
	if _, err = readSnapshot(p.Snapshot); err != nil {
		return fail(err)
	}
	p.Refusal = procgroup.RefusalNone
	return p
}
func contains(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
func containsEffect(a []effect, v effect) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}

func nativeBuildIdentity(binary []byte) (buildIdentity, error) {
	info, err := buildinfo.Read(bytes.NewReader(binary))
	if err != nil {
		return buildIdentity{}, err
	}
	b := buildIdentity{GoVersion: info.GoVersion, BuildFlags: []string{}}
	for _, s := range info.Settings {
		switch s.Key {
		case "GOOS":
			b.GOOS = s.Value
		case "GOARCH":
			b.GOARCH = s.Value
		case "CGO_ENABLED":
			b.CGOEnabled = s.Value
		case "-trimpath":
			if s.Value == "true" {
				b.BuildFlags = append(b.BuildFlags, "-trimpath")
			}
		}
	}
	if !contains(b.BuildFlags, "-trimpath") {
		return b, errors.New("native fixture was not built with -trimpath")
	}
	return b, nil
}
