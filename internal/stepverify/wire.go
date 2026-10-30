// Package stepverify observes a quiescent, host-confined authoring step. Local
// digests and host assertions confer no execution authority or authentication.
package stepverify

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

const MaxRecordBytes = 32 << 20
const MaxEntries = 20_000
const MaxFileBytes = 32 << 20
const MaxTotalBytes = 128 << 20
const MaxDepth = 64

var ErrInput = errors.New("STEP_INPUT")
var ErrUnsupported = errors.New("STEP_UNSUPPORTED")
var ErrDrift = errors.New("STEP_DRIFT")
var idPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
var keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)
var shaPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Checkout struct {
	ID   string `json:"id"`
	Root string `json:"root"`
}
type EnvironmentKey struct {
	Key   string `json:"key"`
	Class string `json:"class"`
}
type Declaration struct {
	Profile      string           `json:"profile"`
	SessionID    string           `json:"session_id"`
	CapabilityID string           `json:"capability_id"`
	Author       Checkout         `json:"author"`
	ReadOnly     []Checkout       `json:"read_only"`
	WritePaths   []string         `json:"write_paths"`
	GuardPaths   []string         `json:"guard_paths"`
	Environment  []EnvironmentKey `json:"environment"`
}
type Host struct {
	Profile            string            `json:"profile"`
	ObserverID         string            `json:"observer_id"`
	SessionID          string            `json:"session_id"`
	CapabilityID       string            `json:"capability_id"`
	DeclarationDigest  string            `json:"declaration_sha256"`
	MetadataProtected  bool              `json:"metadata_protected"`
	FilesystemConfined bool              `json:"filesystem_confined"`
	GitBinary          string            `json:"git_binary"`
	GitBinaryDigest    string            `json:"git_binary_sha256"`
	Environment        *[]EnvironmentKey `json:"environment"`
}
type Entry struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Mode     uint32 `json:"mode"`
	Size     int64  `json:"size"`
	Digest   string `json:"sha256"`
	Identity string `json:"identity_sha256"`
}
type CheckoutState struct {
	ID              string   `json:"id"`
	Root            string   `json:"root"`
	GitDir          string   `json:"git_dir"`
	CommonDir       string   `json:"common_dir"`
	Commit          string   `json:"commit"`
	Tree            string   `json:"tree"`
	IndexDigest     string   `json:"index_sha256"`
	ContentDigest   string   `json:"content_sha256"`
	AdminDigest     string   `json:"admin_sha256"`
	ContentComplete bool     `json:"content_complete"`
	Complete        bool     `json:"complete"`
	Entries         []Entry  `json:"entries"`
	AdminEntries    []Entry  `json:"admin_entries"`
	Unknowns        []string `json:"unknowns"`
}
type State struct {
	Profile           string          `json:"profile"`
	DeclarationDigest string          `json:"declaration_sha256"`
	HostDigest        string          `json:"host_sha256"`
	SessionID         string          `json:"session_id"`
	CapabilityID      string          `json:"capability_id"`
	Checkouts         []CheckoutState `json:"checkouts"`
	Complete          bool            `json:"complete"`
	Digest            string          `json:"sha256"`
}
type Finding struct {
	Code     string `json:"code"`
	Checkout string `json:"checkout"`
	Path     string `json:"path"`
}
type Receipt struct {
	Profile            string          `json:"profile"`
	DeclarationDigest  string          `json:"declaration_sha256"`
	HostDigest         string          `json:"host_sha256"`
	BeforeDigest       string          `json:"before_state_sha256"`
	AfterDigest        *string         `json:"post_state_sha256"`
	Before             []CheckoutState `json:"before"`
	After              []CheckoutState `json:"after"`
	WriteScopeVerdict  string          `json:"write_scope_verdict"`
	EnvironmentVerdict string          `json:"environment_verdict"`
	Findings           []Finding       `json:"findings"`
	Unknowns           []string        `json:"unknowns"`
	Digest             string          `json:"sha256"`
}

func Encode(value any) []byte { data, _ := json.Marshal(value); return append(data, '\n') }
func hash(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func digest(value any) string {
	parsed, err := wire.Parse(Encode(value))
	if err != nil {
		panic("invalid internal step record")
	}
	return hash(wire.CanonicalValue(parsed))
}

// Strict wire parsing rejects duplicates, coercion and malformed Unicode. The
// typed round trip also rejects missing fields and JSON's case-folded members.
func decode(data []byte, dst any) error {
	if len(data) > MaxRecordBytes {
		return ErrInput
	}
	parsed, err := wire.Parse(data)
	if err != nil {
		return ErrInput
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(dst) != nil {
		return ErrInput
	}
	round, err := wire.Parse(Encode(dst))
	if err != nil {
		return ErrInput
	}
	a := wire.CanonicalValue(parsed)
	b := wire.CanonicalValue(round)
	if !bytes.Equal(a, b) {
		return ErrInput
	}
	return nil
}
func textSafe(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsFunc(s, unicode.IsControl)
}
func absolute(s string) bool {
	return len(s) <= 4096 && textSafe(s) && filepath.IsAbs(s) && filepath.Clean(s) == s
}
func literal(s string) bool { return len(s) <= 1024 && textSafe(s) && wire.ValidatePath(s) == nil }
func scope(s string) bool {
	s = strings.TrimSuffix(s, "/")
	if !literal(s) || strings.ContainsAny(s, "*?[]\\:") {
		return false
	}
	for _, part := range strings.Split(s, "/") {
		if strings.EqualFold(part, ".git") {
			return false
		}
	}
	return true
}
func pathsValid(paths []string) bool {
	if paths == nil || len(paths) > 1024 {
		return false
	}
	seen := map[string]bool{}
	for _, p := range paths {
		if !scope(p) || seen[p] {
			return false
		}
		seen[p] = true
	}
	return true
}
func envValid(keys []EnvironmentKey, declaration bool) bool {
	if keys == nil || len(keys) > 1024 {
		return false
	}
	seen := map[string]bool{}
	for _, e := range keys {
		if !keyPattern.MatchString(e.Key) || seen[e.Key] {
			return false
		}
		seen[e.Key] = true
		if e.Class != "NONE" && e.Class != "READ_ONLY" && (declaration || e.Class != "OUTWARD_WRITE" && e.Class != "UNKNOWN") {
			return false
		}
	}
	return true
}
func DecodeDeclaration(data []byte) (Declaration, error) {
	var d Declaration
	if decode(data, &d) != nil || d.Profile != "corvint-step-declaration/0" || !idPattern.MatchString(d.SessionID) || !idPattern.MatchString(d.CapabilityID) || d.ReadOnly == nil || len(d.ReadOnly) > 7 || !pathsValid(d.WritePaths) || !pathsValid(d.GuardPaths) || !envValid(d.Environment, true) {
		return d, ErrInput
	}
	seen := map[string]bool{}
	roots := []string{}
	for _, c := range append([]Checkout{d.Author}, d.ReadOnly...) {
		if !idPattern.MatchString(c.ID) || seen[c.ID] || !absolute(c.Root) {
			return d, ErrInput
		}
		seen[c.ID] = true
		for _, root := range roots {
			if c.Root == root || strings.HasPrefix(c.Root, root+"/") || strings.HasPrefix(root, c.Root+"/") {
				return d, ErrInput
			}
		}
		roots = append(roots, c.Root)
	}
	sort.Strings(d.WritePaths)
	sort.Strings(d.GuardPaths)
	sort.Slice(d.ReadOnly, func(i, j int) bool { return d.ReadOnly[i].ID < d.ReadOnly[j].ID })
	sort.Slice(d.Environment, func(i, j int) bool { return d.Environment[i].Key < d.Environment[j].Key })
	return d, nil
}
func DeclarationDigest(d Declaration) string { return digest(d) }
func DecodeHost(data []byte, d Declaration) (Host, error) {
	var h Host
	if decode(data, &h) != nil || h.Profile != "corvint-step-host/0" || !idPattern.MatchString(h.ObserverID) || h.SessionID != d.SessionID || h.CapabilityID != d.CapabilityID || h.DeclarationDigest != DeclarationDigest(d) || !absolute(h.GitBinary) || !shaPattern.MatchString(h.GitBinaryDigest) {
		return h, ErrInput
	}
	if !h.MetadataProtected || !h.FilesystemConfined {
		return h, ErrUnsupported
	}
	if h.Environment != nil {
		if !envValid(*h.Environment, false) {
			return h, ErrInput
		}
		sort.Slice(*h.Environment, func(i, j int) bool { return (*h.Environment)[i].Key < (*h.Environment)[j].Key })
	}
	return h, nil
}
func Environment(d Declaration, h Host) (string, []Finding) {
	findings := []Finding{}
	var err error
	d, h, err = inputs(d, h)
	if err != nil {
		return "UNSUPPORTED", findings
	}
	if h.Environment == nil {
		return "NOT_OBSERVED", findings
	}
	allowed := map[string]string{}
	for _, e := range d.Environment {
		allowed[e.Key] = e.Class
	}
	for _, e := range *h.Environment {
		code := ""
		if e.Class == "OUTWARD_WRITE" || e.Class == "UNKNOWN" {
			code = "ENVIRONMENT_PROHIBITED"
		} else if c, ok := allowed[e.Key]; !ok || c != e.Class {
			code = "ENVIRONMENT_UNDECLARED"
		}
		if code != "" {
			findings = append(findings, Finding{code, "", e.Key})
		}
	}
	if len(findings) > 0 {
		return "FAIL", findings
	}
	return "PASS", findings
}
func stateDigest(s State) string { s.Digest = ""; return digest(s) }
func DecodeState(data []byte, d Declaration, h Host) (State, error) {
	var s State
	if decode(data, &s) != nil || s.Profile != "corvint-step-state/0" || s.DeclarationDigest != DeclarationDigest(d) || s.HostDigest != digest(h) || s.SessionID != d.SessionID || s.CapabilityID != d.CapabilityID || !s.Complete || s.Digest != stateDigest(s) {
		return s, ErrInput
	}
	want := append([]Checkout{d.Author}, d.ReadOnly...)
	if len(s.Checkouts) != len(want) {
		return s, ErrInput
	}
	count := 0
	for i, c := range s.Checkouts {
		if c.ID != want[i].ID || c.Root != want[i].Root || !c.Complete || !c.ContentComplete || !absolute(c.GitDir) || !absolute(c.CommonDir) || !wire.IsGitOid(c.Commit) || !wire.IsGitOid(c.Tree) || c.Entries == nil || c.AdminEntries == nil || len(c.Unknowns) != 0 || c.ContentDigest != digest(c.Entries) || c.AdminDigest != digest(c.AdminEntries) {
			return s, ErrInput
		}
		if len(c.Entries) == 0 || c.Entries[0].Path != "." || c.Entries[0].Kind != "DIRECTORY" || c.Unknowns == nil {
			return s, ErrInput
		}
		for group, entries := range [][]Entry{c.Entries, c.AdminEntries} {
			last := ""
			for j, e := range entries {
				count++
				if count > MaxEntries || j > 0 && e.Path <= last || e.Path != "." && !literal(e.Path) || !shaPattern.MatchString(e.Digest) || !shaPattern.MatchString(e.Identity) || e.Size < 0 || e.Size > MaxFileBytes || e.Kind != "FILE" && e.Kind != "DIRECTORY" && e.Kind != "SYMLINK" && (group != 1 || e.Kind != "ABSENT") {
					return s, ErrInput
				}
				mode := os.FileMode(e.Mode)
				if e.Kind == "FILE" && !mode.IsRegular() || e.Kind == "DIRECTORY" && (!mode.IsDir() || e.Size != 0 || e.Digest != hash(nil)) || e.Kind == "SYMLINK" && (group == 1 || mode&os.ModeSymlink == 0 || e.Size > 1024) {
					return s, ErrInput
				}
				if e.Kind == "ABSENT" && (e.Mode != 0 || e.Size != 0 || e.Digest != hash(nil) || e.Identity != hash(nil)) {
					return s, ErrInput
				}
				last = e.Path
			}
		}
		adminRoots := map[string]bool{}
		for _, entry := range c.AdminEntries {
			if (entry.Path == "git" || entry.Path == "common") && entry.Kind == "DIRECTORY" {
				adminRoots[entry.Path] = true
			}
		}
		if !adminRoots["git"] || !adminRoots["common"] {
			return s, ErrInput
		}
		if c.IndexDigest != indexDigest(c.AdminEntries) {
			return s, ErrInput
		}
	}
	return s, nil
}

// EnvironmentReceipt records host-supplied key classifications, never values.
type EnvironmentReceipt struct {
	Profile           string    `json:"profile"`
	DeclarationDigest string    `json:"declaration_sha256"`
	HostDigest        string    `json:"host_sha256"`
	Verdict           string    `json:"environment_verdict"`
	Findings          []Finding `json:"findings"`
	Unknowns          []string  `json:"unknowns"`
	Digest            string    `json:"sha256"`
}

func Preflight(d Declaration, h Host) (EnvironmentReceipt, int, error) {
	d, h, err := inputs(d, h)
	if err != nil {
		return EnvironmentReceipt{}, 2, err
	}
	verdict, findings := Environment(d, h)
	r := EnvironmentReceipt{Profile: "corvint-step-environment/0", DeclarationDigest: DeclarationDigest(d), HostDigest: digest(h), Verdict: verdict, Findings: findings, Unknowns: []string{"HOST_ASSERTIONS_UNAUTHENTICATED", "ENVIRONMENT_VALUES_NOT_READ"}}
	code := 0
	if verdict == "FAIL" {
		code = 1
	}
	if verdict == "NOT_OBSERVED" {
		code = 2
		r.Unknowns = append(r.Unknowns, "ENVIRONMENT_NOT_OBSERVED")
	}
	sort.Strings(r.Unknowns)
	r.Digest = digest(r)
	return r, code, nil
}

// BoundedEncode refuses oversized records rather than silently dropping evidence.
func BoundedEncode(value any) ([]byte, error) {
	data := Encode(value)
	if len(data) > MaxRecordBytes {
		return nil, ErrUnsupported
	}
	return data, nil
}
