// SPDX-License-Identifier: Apache-2.0

// The record APIs define the derived, language-neutral records of RSP-V0-006.
// These APIs perform no repository, plugin, trace or store I/O. Admission and
// selection belong to their consumers; well-formedness is not authority.
package testevidence

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode/utf8"
)

const Format = "corvint-test-evidence/1"
const MaxRecordBytes = 8 << 20

type State string

const (
	Present    State = "present"
	Absent     State = "absent"
	Incomplete State = "incomplete"
)

// Component distinguishes a complete empty observation from missing evidence.
// Absent and incomplete always carry a reason; incomplete may retain known edges.
type Component[T any] struct {
	State  State  `json:"state"`
	Reason string `json:"reason,omitempty"`
	Items  []T    `json:"items"`
}

func Missing[T any](reason string) Component[T] { return Component[T]{State: Absent, Reason: reason} }

type RunnerIdentity struct {
	Plugin              string `json:"plugin"`
	PluginVersion       string `json:"plugin_version"`
	ExecutableDigest    string `json:"executable_digest"`
	RunnerVersion       string `json:"runner_version"`
	ConfigurationDigest string `json:"configuration_digest"`
}

type ExecutionIdentity struct {
	OS                string `json:"os"`
	Architecture      string `json:"architecture"`
	TracingBackend    string `json:"tracing_backend"`
	TracingVersion    string `json:"tracing_version"`
	EnvironmentDigest string `json:"environment_digest"`
}

// Producer carries bindings for later admission, never an "admitted" boolean.
type Producer struct {
	CoreDigest          string `json:"core_digest"`
	VerifierRevision    string `json:"verifier_revision"`
	ConfigurationDigest string `json:"configuration_digest"`
	Ref                 string `json:"ref"`
	Run                 string `json:"run"`
	ContentDigest       string `json:"content_digest"`
}

// Path keeps both names of a link. Repository disambiguates sibling paths
// (RSP-V0-007, RSP-V0-049). Names are slash-separated, repository-relative;
// root directories are ".". Case folding is done at capture, on both names.
type Path struct {
	Repository string `json:"repository"`
	Lexical    string `json:"lexical"`
	Resolved   string `json:"resolved"`
}

type Blob struct {
	Path Path   `json:"path"`
	OID  string `json:"oid"`
	// LexicalOID pins the link itself when the names differ.
	LexicalOID string `json:"lexical_oid,omitempty"`
}

type Range struct {
	Start       int32 `json:"start"`
	StartColumn int32 `json:"start_column"`
	End         int32 `json:"end"`
	EndColumn   int32 `json:"end_column"`
}

type ExecutedFile struct {
	Blob       Blob    `json:"blob"`
	Executed   []Range `json:"executed"`
	Unexecuted []Range `json:"unexecuted"`
}

type Body struct {
	ID     string `json:"id"`
	Parent string `json:"parent,omitempty"`
	Range  Range  `json:"range"`
}

type RegionsFile struct {
	Blob   Blob   `json:"blob"`
	Bodies []Body `json:"bodies"`
}

type WriteKind string

const (
	Create    WriteKind = "create"
	Replace   WriteKind = "replace"
	Append    WriteKind = "append"
	Overwrite WriteKind = "overwrite"
	Truncate  WriteKind = "truncate"
)

type Write struct {
	Path Path      `json:"path"`
	Kind WriteKind `json:"kind"`
}

type Executable struct {
	Blob         Blob   `json:"blob"`
	Digest       string `json:"digest"`
	Instrumented bool   `json:"instrumented"`
}

type Contact struct {
	Kind string `json:"kind"` // network, loopback, unix-socket, named-pipe, ipc
}

type OutsideCounts struct {
	Toolchain uint64 `json:"toolchain"`
	Cache     uint64 `json:"cache"`
	Temporary uint64 `json:"temporary"`
	Other     uint64 `json:"other"`
}

// Record contains one test's evidence, or its conservative union. Sources retain
// each distinct collection's provenance; their count cannot grow by replaying a
// collection (RSP-V0-055). No environment value or outside path is retained.
type Record struct {
	Format           string                  `json:"format"`
	Repository       string                  `json:"repository"`
	Test             string                  `json:"test"`
	Commit           string                  `json:"commit"`
	Runner           RunnerIdentity          `json:"runner"`
	Execution        ExecutionIdentity       `json:"execution"`
	Sources          []Producer              `json:"sources"`
	CollectionCount  uint64                  `json:"collection_count"`
	Nondeterministic bool                    `json:"nondeterministic"`
	Executed         Component[ExecutedFile] `json:"executed"`
	Regions          Component[RegionsFile]  `json:"regions"`
	Loaded           Component[Blob]         `json:"loaded"`
	Reads            Component[Blob]         `json:"reads"`
	Listings         Component[Path]         `json:"listings"`
	Lookups          Component[Path]         `json:"lookups"`
	Writes           Component[Write]        `json:"writes"`
	Execs            Component[Executable]   `json:"execs"`
	External         Component[Contact]      `json:"external"`
	Outside          OutsideCounts           `json:"outside"`
}

// New leaves every component explicitly absent. Identity must be supplied by the
// core before storage; a legacy record can expose its edges without inventing it.
func New(repository, test, commit string) Record {
	return Record{Format: Format, Repository: repository, Test: test, Commit: commit,
		Executed: Missing[ExecutedFile]("not-collected"), Regions: Missing[RegionsFile]("not-collected"),
		Loaded: Missing[Blob]("not-collected"), Reads: Missing[Blob]("not-collected"),
		Listings: Missing[Path]("not-collected"), Lookups: Missing[Path]("not-collected"),
		Writes: Missing[Write]("not-collected"), Execs: Missing[Executable]("not-collected"), External: Missing[Contact]("not-collected")}
}

// TargetKey prevents reuse across target inventories (RSP-V0-053). Cross-target
// qualification is a consumer admission decision, never inferred here.
type TargetKey struct{ Repository, Test, OS, Architecture string }

func (r Record) TargetKey() (TargetKey, error) {
	k := TargetKey{r.Repository, r.Test, r.Execution.OS, r.Execution.Architecture}
	if k.Repository == "" || k.Test == "" || k.OS == "" || k.Architecture == "" || !validText(k.Repository) || !validText(k.Test) || !validText(k.OS) || !validText(k.Architecture) {
		return TargetKey{}, errors.New("target-identity-missing")
	}
	return k, nil
}

func validHex(s string, sizes ...int) bool {
	for _, size := range sizes {
		if len(s) == size {
			_, err := hex.DecodeString(s)
			return err == nil && s == strings.ToLower(s)
		}
	}
	return false
}
func validText(s string) bool { return utf8.ValidString(s) && !strings.ContainsRune(s, 0) }

func validPath(p Path) bool {
	valid := func(s string) bool {
		return s != "" && validText(s) && !strings.ContainsAny(s, "\\\x00:") && !strings.HasPrefix(s, "/") && s != ".." && !strings.HasPrefix(s, "../") && path.Clean(s) == s
	}
	return p.Repository != "" && validText(p.Repository) && valid(p.Lexical) && valid(p.Resolved)
}
func validBlob(b Blob) bool {
	return validPath(b.Path) && b.Path.Lexical != "." && b.Path.Resolved != "." && validHex(b.OID, 40, 64) && (b.Path.Lexical == b.Path.Resolved && (b.LexicalOID == "" || b.LexicalOID == b.OID) || b.Path.Lexical != b.Path.Resolved && validHex(b.LexicalOID, 40, 64))
}
func validRange(r Range) bool {
	return r.Start > 0 && r.StartColumn > 0 && r.End >= r.Start && r.EndColumn > 0 && (r.End > r.Start || r.EndColumn > r.StartColumn)
}
func checkComponent[T any](name string, c Component[T], valid func(T) bool) error {
	if !validText(c.Reason) {
		return fmt.Errorf("%s:malformed-reason", name)
	}
	switch c.State {
	case Present:
		if c.Reason != "" {
			return fmt.Errorf("%s:present-with-reason", name)
		}
	case Absent:
		if len(c.Items) != 0 || c.Reason == "" {
			return fmt.Errorf("%s:invalid-absence", name)
		}
	case Incomplete:
		if c.Reason == "" {
			return fmt.Errorf("%s:missing-reason", name)
		}
	default:
		return fmt.Errorf("%s:unknown-state", name)
	}
	for _, item := range c.Items {
		if !valid(item) {
			return fmt.Errorf("%s:malformed-item", name)
		}
	}
	return nil
}

// Validate refuses unknown or malformed inputs. Callers must retain the error as
// an incomplete-record reason, never substitute a present empty component.
func (r Record) Validate() error {
	if r.Format != Format {
		return errors.New("unsupported-record-format")
	}
	if _, err := r.TargetKey(); err != nil {
		return err
	}
	if !validHex(r.Commit, 40, 64) {
		return errors.New("commit-missing-or-malformed")
	}
	if !validText(r.Runner.Plugin) || !validText(r.Runner.PluginVersion) || !validText(r.Runner.RunnerVersion) || r.Runner.Plugin == "" || r.Runner.PluginVersion == "" || r.Runner.RunnerVersion == "" || !validHex(r.Runner.ExecutableDigest, 64) || !validHex(r.Runner.ConfigurationDigest, 64) {
		return errors.New("runner-identity-incomplete")
	}
	if !validText(r.Execution.TracingBackend) || !validText(r.Execution.TracingVersion) || r.Execution.TracingBackend == "" || r.Execution.TracingVersion == "" || !validHex(r.Execution.EnvironmentDigest, 64) {
		return errors.New("execution-identity-incomplete")
	}
	if r.CollectionCount == 0 || r.CollectionCount != uint64(len(r.Sources)) {
		return errors.New("collection-count-invalid")
	}
	seen := map[string]bool{}
	for _, p := range r.Sources {
		if !validHex(p.CoreDigest, 64) || !validHex(p.VerifierRevision, 40, 64) || !validHex(p.ConfigurationDigest, 64) || !validHex(p.ContentDigest, 64) || !validText(p.Ref) || !validText(p.Run) || p.Ref == "" || p.Run == "" || seen[p.Run] {
			return errors.New("producer-provenance-incomplete-or-duplicate")
		}
		seen[p.Run] = true
	}
	return r.ValidateComponents()
}

// ValidateComponents checks payloads without claiming identity or provenance.
func (r Record) ValidateComponents() error {
	checks := []error{
		checkComponent("executed", r.Executed, func(f ExecutedFile) bool {
			if !validBlob(f.Blob) {
				return false
			}
			for _, rs := range [][]Range{f.Executed, f.Unexecuted} {
				for _, x := range rs {
					if !validRange(x) {
						return false
					}
				}
			}
			return true
		}),
		checkComponent("regions", r.Regions, validRegions),
		checkComponent("loaded", r.Loaded, validBlob), checkComponent("reads", r.Reads, validBlob),
		checkComponent("listings", r.Listings, validPath), checkComponent("lookups", r.Lookups, validPath),
		checkComponent("writes", r.Writes, func(w Write) bool {
			return validPath(w.Path) && (w.Kind == Create || w.Kind == Replace || w.Kind == Append || w.Kind == Overwrite || w.Kind == Truncate)
		}),
		checkComponent("execs", r.Execs, func(e Executable) bool { return validBlob(e.Blob) && validHex(e.Digest, 64) }),
		checkComponent("external", r.External, func(c Contact) bool {
			switch c.Kind {
			case "network", "loopback", "unix-socket", "named-pipe", "ipc":
				return true
			}
			return false
		}),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}
func validRegions(f RegionsFile) bool {
	if !validBlob(f.Blob) {
		return false
	}
	bodies := map[string]Body{}
	for _, b := range f.Bodies {
		if b.ID == "" || !validText(b.ID) || !validText(b.Parent) || !validRange(b.Range) {
			return false
		}
		if _, dup := bodies[b.ID]; dup {
			return false
		}
		bodies[b.ID] = b
	}
	for _, b := range f.Bodies {
		seen := map[string]bool{b.ID: true}
		for b.Parent != "" {
			p, ok := bodies[b.Parent]
			if !ok || seen[p.ID] || !contains(p.Range, b.Range) {
				return false
			}
			seen[p.ID] = true
			b = p
		}
	}
	return true
}
func contains(a, b Range) bool {
	return (a.Start < b.Start || a.Start == b.Start && a.StartColumn <= b.StartColumn) && (a.End > b.End || a.End == b.End && a.EndColumn >= b.EndColumn)
}

// Encode and Decode bound the new record wire, leaving existing linecov wires
// and collectors unchanged. Decode also rejects duplicate and unknown members.
func Encode(r Record) ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(r)
	if len(b) > MaxRecordBytes {
		return nil, errors.New("record-size-cap")
	}
	return b, err
}
func Decode(data []byte) (Record, error) {
	var r Record
	if !utf8.Valid(data) {
		return r, errors.New("record-invalid-utf8")
	}
	if len(data) > MaxRecordBytes {
		return r, errors.New("record-size-cap")
	}
	if err := uniqueJSON(json.NewDecoder(bytes.NewReader(data))); err != nil {
		return r, err
	}
	if err := jsonv2.Unmarshal(data, &r, jsonv2.RejectUnknownMembers(true)); err != nil {
		return Record{}, fmt.Errorf("malformed-record: %w", err)
	}
	if err := r.Validate(); err != nil {
		return Record{}, err
	}
	return r, nil
}
func uniqueJSON(d *json.Decoder) error {
	var value func(int) error
	value = func(depth int) error {
		if depth > 64 {
			return errors.New("record-depth-cap")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					t, err := d.Token()
					if err != nil {
						return err
					}
					k, ok := t.(string)
					if !ok || seen[k] {
						return errors.New("duplicate-record-member")
					}
					seen[k] = true
					if err := value(depth + 1); err != nil {
						return err
					}
				}
			case '[':
				for d.More() {
					if err := value(depth + 1); err != nil {
						return err
					}
				}
			default:
				return errors.New("malformed-record")
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing-record-data")
	}
	return nil
}
