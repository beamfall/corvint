// Package testplan is the read-only test consolidation planner
// (docs/specs/test-consolidation-planner-v0.md, TCN-V0): it turns a set of flow
// variations with declared anchors into fewer focused tests with an
// independent step order, existing passing witnesses, duplicates and
// abstentions, printed as a checkable table or a closed JSON plan.
package testplan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/gokernel"
)

const (
	// InputSchema names the planner input document (TCN-V0-002).
	InputSchema = "test-consolidation-input/0"
	// MaxInputBytes bounds the input document and a checked plan file.
	MaxInputBytes = 8 << 20
	// MaxVariations is the AFU-V1-052 row bound.
	MaxVariations = 8192
	maxList       = 64
	maxWitnesses  = 16
	maxFactKey    = 128
	maxIdentifier = 256
	maxSpecBytes  = 4096
	maxDepth      = 8
)

// Variation members in the order a missing-anchor abstention names them (TCN-V0-003).
var anchorMembers = []string{"spec", "app", "screen", "setup", "user", "org", "action", "assertion", "requires", "changes", "destructive"}

var variationMembers = map[string]bool{"variation_id": true, "flow_id": true, "route": true, "witnesses": true}

func init() {
	for _, m := range anchorMembers {
		variationMembers[m] = true
	}
}

// Input is one decoded test-consolidation-input/0 document, variations sorted by ID.
type Input struct {
	Revision   string
	Variations []Variation
	digest     string
}

// Digest is "sha256:" over the canonical input (TCN-V0-009).
func (in *Input) Digest() string { return in.digest }

// Variation is one input row. A nil pointer or slice is a missing anchor; Route and FlowID
// are optional and empty when absent.
type Variation struct {
	ID          string
	FlowID      string
	Spec        *string
	App         *string
	Route       string
	Screen      *string
	Setup       *string
	User        *string
	Org         *string
	Action      []string
	Assertion   []string // sorted set
	Requires    []string // sorted facts
	Changes     []string // sorted facts
	Destructive *bool
	Witnesses   []Witness // sorted by test_id, project; unique
	// Missing lists the absent anchors in anchorMembers order.
	Missing []string
}

// Witness names an existing test the variation declares as its own (TCN-V0-002, TCN-V0-004).
// Project is empty when the declaration names no project.
type Witness struct {
	TestID  string
	Project string
	Basis   string
}

func invalidInput(format string, args ...any) error {
	return &gokernel.Error{Code: "test-plan-invalid-input", Message: fmt.Sprintf(format, args...)}
}

// DecodeInput strictly decodes one closed test-consolidation-input/0 document (TCN-V0-002).
func DecodeInput(data []byte) (*Input, error) {
	if len(data) > MaxInputBytes {
		return nil, invalidInput("input exceeds %d bytes", MaxInputBytes)
	}
	if !utf8.Valid(data) {
		return nil, invalidInput("input is not valid UTF-8")
	}
	if err := checkStructure(data); err != nil {
		return nil, invalidInput("input is not one closed JSON document: %v", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil || top == nil {
		return nil, invalidInput("input must be a JSON object")
	}
	for key := range top {
		if key != "schema" && key != "revision" && key != "variations" {
			return nil, invalidInput("unknown input member %q", key)
		}
	}
	var schema string
	if raw, ok := top["schema"]; !ok || json.Unmarshal(raw, &schema) != nil || schema != InputSchema {
		return nil, invalidInput("schema must be %s", InputSchema)
	}
	in := &Input{}
	if raw, ok := top["revision"]; ok {
		if json.Unmarshal(raw, &in.Revision) != nil || !objectID(in.Revision) {
			return nil, invalidInput("revision must be a full lowercase hexadecimal object ID")
		}
	}
	var rows []json.RawMessage
	if raw, ok := top["variations"]; !ok || isNull(raw) || json.Unmarshal(raw, &rows) != nil {
		return nil, invalidInput("variations must be an array")
	}
	if len(rows) < 1 || len(rows) > MaxVariations {
		return nil, invalidInput("variations must hold 1..%d entries", MaxVariations)
	}
	seen := map[string]bool{}
	for i, raw := range rows {
		v, err := decodeVariation(raw)
		if err != nil {
			return nil, invalidInput("variations[%d]: %v", i, err)
		}
		if seen[v.ID] {
			return nil, invalidInput("variation_id %s is repeated", v.ID)
		}
		seen[v.ID] = true
		in.Variations = append(in.Variations, v)
	}
	sort.Slice(in.Variations, func(i, j int) bool { return in.Variations[i].ID < in.Variations[j].ID })
	canonical, err := json.Marshal(canonicalInput(in))
	if err != nil {
		return nil, invalidInput("input cannot be canonicalized")
	}
	in.digest = sha(canonical)
	return in, nil
}

func decodeVariation(raw json.RawMessage) (Variation, error) {
	var v Variation
	var m map[string]json.RawMessage
	if isNull(raw) || json.Unmarshal(raw, &m) != nil {
		return v, errors.New("a variation must be an object")
	}
	for key := range m {
		if !variationMembers[key] {
			return v, fmt.Errorf("unknown member %q", key)
		}
	}
	if raw, ok := m["variation_id"]; !ok || json.Unmarshal(raw, &v.ID) != nil || !identifier(v.ID) {
		return v, errors.New("variation_id must be an identifier")
	}
	optional := func(name string, target *string) error {
		raw, ok := m[name]
		if !ok || isNull(raw) {
			return nil
		}
		if json.Unmarshal(raw, target) != nil || !identifier(*target) {
			return fmt.Errorf("%s must be an identifier", name)
		}
		return nil
	}
	if err := optional("flow_id", &v.FlowID); err != nil {
		return v, err
	}
	if err := optional("route", &v.Route); err != nil {
		return v, err
	}
	present := func(name string) (json.RawMessage, bool) {
		raw, ok := m[name]
		if !ok || isNull(raw) {
			v.Missing = append(v.Missing, name)
			return nil, false
		}
		return raw, true
	}
	for _, name := range anchorMembers {
		raw, ok := present(name)
		if !ok {
			continue
		}
		var err error
		switch name {
		case "spec":
			v.Spec, err = stringMember(raw, name, specPath)
		case "app":
			v.App, err = stringMember(raw, name, identifier)
		case "screen":
			v.Screen, err = stringMember(raw, name, identifier)
		case "setup":
			v.Setup, err = stringMember(raw, name, identifier)
		case "user":
			v.User, err = stringMember(raw, name, identifier)
		case "org":
			v.Org, err = stringMember(raw, name, func(s string) bool { return s == "" || identifier(s) })
		case "action":
			v.Action, err = listMember(raw, name, 1, false)
		case "assertion":
			v.Assertion, err = listMember(raw, name, 1, true)
		case "requires":
			v.Requires, err = factMember(raw, name)
		case "changes":
			v.Changes, err = factMember(raw, name)
		case "destructive":
			var b bool
			if json.Unmarshal(raw, &b) != nil {
				err = errors.New("destructive must be a boolean")
			}
			v.Destructive = &b
		}
		if err != nil {
			return v, err
		}
	}
	if raw, ok := m["witnesses"]; ok {
		witnesses, err := witnessMember(raw)
		if err != nil {
			return v, err
		}
		v.Witnesses = witnesses
	}
	return v, nil
}

func stringMember(raw json.RawMessage, name string, valid func(string) bool) (*string, error) {
	var s string
	if json.Unmarshal(raw, &s) != nil || !valid(s) {
		return nil, fmt.Errorf("%s is not a valid value", name)
	}
	return &s, nil
}

// listMember decodes 1..64 identifiers; a set refuses a repeated member and is returned sorted.
func listMember(raw json.RawMessage, name string, minimum int, set bool) ([]string, error) {
	var items []string
	if json.Unmarshal(raw, &items) != nil || len(items) < minimum || len(items) > maxList {
		return nil, fmt.Errorf("%s must be an array of %d..%d identifiers", name, minimum, maxList)
	}
	for _, item := range items {
		if !identifier(item) {
			return nil, fmt.Errorf("%s holds an invalid identifier", name)
		}
	}
	if set {
		items = sortedCopy(items)
		for i := 1; i < len(items); i++ {
			if items[i] == items[i-1] {
				return nil, fmt.Errorf("%s repeats %s", name, items[i])
			}
		}
	}
	return items, nil
}

// factMember decodes 0..64 key=value facts; one set naming a key twice is contradictory.
func factMember(raw json.RawMessage, name string) ([]string, error) {
	var items []string
	if json.Unmarshal(raw, &items) != nil || items == nil || len(items) > maxList {
		return nil, fmt.Errorf("%s must be an array of at most %d facts", name, maxList)
	}
	keys := map[string]bool{}
	for _, item := range items {
		key, _, ok := strings.Cut(item, "=")
		if !ok || key == "" || len(key) > maxFactKey {
			return nil, fmt.Errorf("%s holds a malformed fact", name)
		}
		if keys[key] {
			return nil, fmt.Errorf("%s names key %q twice", name, key)
		}
		keys[key] = true
	}
	return sortedCopy(items), nil
}

func witnessMember(raw json.RawMessage) ([]Witness, error) {
	var items []map[string]json.RawMessage
	if isNull(raw) || json.Unmarshal(raw, &items) != nil || len(items) > maxWitnesses {
		return nil, fmt.Errorf("witnesses must be an array of at most %d objects", maxWitnesses)
	}
	unique := map[[2]string]Witness{}
	for _, item := range items {
		if item == nil {
			return nil, errors.New("a witness must be an object")
		}
		var w Witness
		for key, value := range item {
			var s string
			if json.Unmarshal(value, &s) != nil {
				return nil, errors.New("witness members must be strings")
			}
			switch key {
			case "test_id":
				w.TestID = s
			case "project":
				if !identifier(s) {
					return nil, errors.New("witness project must be an identifier")
				}
				w.Project = s
			case "basis":
				w.Basis = s
			default:
				return nil, fmt.Errorf("unknown witness member %q", key)
			}
		}
		if !identifier(w.TestID) {
			return nil, errors.New("witness test_id must be an identifier")
		}
		if w.Basis != "declared" && w.Basis != "reviewed" {
			return nil, errors.New("witness basis must be declared or reviewed")
		}
		key := [2]string{w.TestID, w.Project}
		if prior, ok := unique[key]; !ok || w.Basis < prior.Basis {
			unique[key] = w
		}
	}
	out := make([]Witness, 0, len(unique))
	for _, w := range unique {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TestID != out[j].TestID {
			return out[i].TestID < out[j].TestID
		}
		return out[i].Project < out[j].Project
	})
	return out, nil
}

// checkStructure walks every token: no duplicate object member, no trailing data, bounded depth.
func checkStructure(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	if err := walkValue(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing data after the document")
	}
	return nil
}

func walkValue(d *json.Decoder, depth int) error {
	if depth > maxDepth {
		return errors.New("nesting exceeds the input shape")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		keys := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, _ := key.(string)
			if keys[name] {
				return fmt.Errorf("member %q is repeated", name)
			}
			keys[name] = true
			if err := walkValue(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := walkValue(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected delimiter")
	}
	_, err = d.Token()
	return err
}

func isNull(raw json.RawMessage) bool { return string(bytes.TrimSpace(raw)) == "null" }

// identifier is the TCN-V0-002 charset [A-Za-z0-9._:/@{}=+-]{1,256}.
func identifier(s string) bool {
	if len(s) < 1 || len(s) > maxIdentifier {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("._:/@{}=+-", c) >= 0) {
			return false
		}
	}
	return true
}

// specPath is a canonical repository-relative path with no empty, "." or ".." segment and no
// whitespace, control character or "|".
func specPath(s string) bool {
	if s == "" || len(s) > maxSpecBytes {
		return false
	}
	for _, r := range s {
		if r == '|' || r == '\\' || unicode.IsSpace(r) || unicode.IsControl(r) || r == utf8.RuneError {
			return false
		}
	}
	for _, segment := range strings.Split(s, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func objectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !(s[i] >= '0' && s[i] <= '9' || s[i] >= 'a' && s[i] <= 'f') {
			return false
		}
	}
	return true
}

func sortedCopy(items []string) []string {
	out := make([]string, len(items))
	copy(out, items)
	sort.Strings(out)
	return out
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// canonical* fix the member order of the canonical input: variations sorted by ID, set members
// sorted, absent and null members omitted, compact JSON (TCN-V0-009).
type canonicalDocument struct {
	Schema     string               `json:"schema"`
	Revision   string               `json:"revision,omitempty"`
	Variations []canonicalVariation `json:"variations"`
}

type canonicalVariation struct {
	VariationID string             `json:"variation_id"`
	FlowID      string             `json:"flow_id,omitempty"`
	Spec        *string            `json:"spec,omitempty"`
	App         *string            `json:"app,omitempty"`
	Route       string             `json:"route,omitempty"`
	Screen      *string            `json:"screen,omitempty"`
	Setup       *string            `json:"setup,omitempty"`
	User        *string            `json:"user,omitempty"`
	Org         *string            `json:"org,omitempty"`
	Action      []string           `json:"action,omitempty"`
	Assertion   []string           `json:"assertion,omitempty"`
	Requires    *[]string          `json:"requires,omitempty"`
	Changes     *[]string          `json:"changes,omitempty"`
	Destructive *bool              `json:"destructive,omitempty"`
	Witnesses   []canonicalWitness `json:"witnesses,omitempty"`
}

type canonicalWitness struct {
	TestID  string `json:"test_id"`
	Project string `json:"project,omitempty"`
	Basis   string `json:"basis"`
}

func canonicalInput(in *Input) canonicalDocument {
	doc := canonicalDocument{Schema: InputSchema, Revision: in.Revision, Variations: []canonicalVariation{}}
	for i := range in.Variations {
		v := &in.Variations[i]
		c := canonicalVariation{VariationID: v.ID, FlowID: v.FlowID, Spec: v.Spec, App: v.App, Route: v.Route, Screen: v.Screen,
			Setup: v.Setup, User: v.User, Org: v.Org, Action: v.Action, Assertion: v.Assertion, Destructive: v.Destructive}
		if v.Requires != nil {
			c.Requires = &v.Requires
		}
		if v.Changes != nil {
			c.Changes = &v.Changes
		}
		for _, w := range v.Witnesses {
			c.Witnesses = append(c.Witnesses, canonicalWitness{TestID: w.TestID, Project: w.Project, Basis: w.Basis})
		}
		doc.Variations = append(doc.Variations, c)
	}
	return doc
}
