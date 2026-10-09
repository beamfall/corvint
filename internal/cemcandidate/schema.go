// Package cemcandidate joins explicit reference evidence without promoting
// native Tasks authority, historical execution or criterion adequacy.
package cemcandidate

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	cw "github.com/Beamfall/corvint/internal/cem/wire"
)

const Profile = "cem-candidate-assembly/0"
const MaxDocument = 4 << 20

type FileRef struct {
	Path   string `json:"path"`
	Sha256 string `json:"sha256"`
}
type Link struct {
	CriterionIndex int      `json:"criterionIndex"`
	HunkIDs        []string `json:"hunkIds"`
	EvidenceIDs    []string `json:"evidenceIds"`
}
type Request struct {
	Profile       string  `json:"profile"`
	Repository    string  `json:"repository"`
	ExpectedBase  string  `json:"expectedBase"`
	Target        string  `json:"target"`
	TicketID      string  `json:"ticketId"`
	AttemptID     string  `json:"attemptId"`
	SourcePrefix  string  `json:"sourcePrefix"`
	SourceMap     FileRef `json:"sourceMap"`
	Capture       FileRef `json:"capture"`
	Verification  FileRef `json:"verification"`
	RunnerPlan    FileRef `json:"runnerPlan"`
	RunnerReceipt FileRef `json:"runnerReceipt"`
	Links         []Link  `json:"links"`
}

// Decode requires all non-optional fields and exact scalar kinds before Go's
// decoder can coerce null into a zero-valued string or integer. Native nullable
// slices/maps and optional pointers remain nullable, preserving their ABI.
func Decode(raw []byte, out any) error {
	return decode(raw, raw, out)
}

// decode checks structure on structural bytes, which differ from raw only in
// signs a profile owner admitted (testrunner.StructuralBytes), and decodes raw.
func decode(structural, raw []byte, out any) error {
	if len(raw) > MaxDocument {
		return fmt.Errorf("document exceeds bound")
	}
	value, e := cw.Parse(structural)
	if e != nil {
		return e
	}
	t := reflect.TypeOf(out)
	if t == nil || t.Kind() != reflect.Pointer {
		return fmt.Errorf("pointer destination required")
	}
	if e := typed(value, t.Elem()); e != nil {
		return e
	}
	return json.Unmarshal(raw, out)
}
func typed(v cw.Value, t reflect.Type) error {
	switch t.Kind() {
	case reflect.Pointer:
		if v.Kind == cw.KindNull {
			return nil
		}
		return typed(v, t.Elem())
	case reflect.Struct:
		if v.Kind != cw.KindObject {
			return fmt.Errorf("object required")
		}
		fields := map[string]reflect.Type{}
		required := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			parts := strings.Split(f.Tag.Get("json"), ",")
			if parts[0] == "-" {
				continue
			}
			if parts[0] == "" {
				return fmt.Errorf("untagged document field")
			}
			fields[parts[0]] = f.Type
			required[parts[0]] = len(parts) == 1 || parts[1] != "omitempty"
		}
		for _, key := range v.Obj.Keys {
			field, ok := fields[key]
			if !ok {
				return fmt.Errorf("unknown field %q", key)
			}
			if e := typed(v.Obj.Values[key], field); e != nil {
				return fmt.Errorf("%s: %w", key, e)
			}
			delete(required, key)
		}
		for key, needed := range required {
			if needed {
				return fmt.Errorf("missing field %q", key)
			}
		}
	case reflect.Map:
		if v.Kind == cw.KindNull {
			return nil
		}
		if v.Kind != cw.KindObject {
			return fmt.Errorf("map required")
		}
		for _, key := range v.Obj.Keys {
			if e := typed(v.Obj.Values[key], t.Elem()); e != nil {
				return e
			}
		}
	case reflect.Slice:
		if v.Kind == cw.KindNull {
			return nil
		}
		if t.Elem().Kind() == reflect.Uint8 {
			if v.Kind != cw.KindString {
				return fmt.Errorf("base64 string required")
			}
			return nil
		}
		if v.Kind != cw.KindArray {
			return fmt.Errorf("array required")
		}
		for _, item := range v.Arr {
			if e := typed(item, t.Elem()); e != nil {
				return e
			}
		}
	case reflect.String:
		if v.Kind != cw.KindString {
			return fmt.Errorf("string required")
		}
	case reflect.Bool:
		if v.Kind != cw.KindBool {
			return fmt.Errorf("boolean required")
		}
	case reflect.Int, reflect.Int64:
		if v.Kind != cw.KindInt {
			return fmt.Errorf("integer required")
		}
	default:
		return fmt.Errorf("unsupported document type")
	}
	return nil
}
func literal(p string) bool {
	if cw.ValidatePath(p) != nil {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if strings.EqualFold(s, ".git") {
			return false
		}
	}
	return true
}
func (r Request) validate() error {
	if r.Profile != Profile || !filepath.IsAbs(r.Repository) || !cw.IsGitOid(r.ExpectedBase) || !cw.IsGitOid(r.Target) || len(r.ExpectedBase) != len(r.Target) || r.TicketID == "" || r.AttemptID == "" || len(r.Links) == 0 || len(r.Links) > 256 {
		return fmt.Errorf("incomplete assembly request")
	}
	if r.SourcePrefix != "" && !literal(r.SourcePrefix) {
		return fmt.Errorf("invalid source prefix")
	}
	for _, pin := range r.inputs() {
		if !filepath.IsAbs(pin.Path) || filepath.Clean(pin.Path) != pin.Path || !cw.IsSha256(pin.Sha256) {
			return fmt.Errorf("absolute pinned input required")
		}
	}
	for _, l := range r.Links {
		if l.CriterionIndex < 0 || l.CriterionIndex >= 256 || len(l.HunkIDs) == 0 || len(l.HunkIDs) > 32 || len(l.EvidenceIDs) == 0 || len(l.EvidenceIDs) > 32 {
			return fmt.Errorf("invalid explicit criterion link")
		}
	}
	return nil
}
func (r Request) inputs() []FileRef {
	return []FileRef{r.SourceMap, r.Capture, r.Verification, r.RunnerPlan, r.RunnerReceipt}
}
