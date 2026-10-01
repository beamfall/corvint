package postmergeworkflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	connector "github.com/Beamfall/corvint/internal/postmergeconnector"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var hash = regexp.MustCompile(`^[0-9a-f]{64}$`)
var relativePath = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,512}$`)

func SHA256(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func valueHash(v any) string { b, _ := json.Marshal(v); return SHA256(b) }

// Decode rejects duplicate keys as well as unknown fields. Duplicate keys are
// otherwise silently overwritten by encoding/json, including policy bindings.
func Decode(b []byte, v any) error {
	if len(b) == 0 || len(b) > MaxBytes {
		return fmt.Errorf("input-size-invalid")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if err := uniqueJSON(d, 0, reflect.TypeOf(v)); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing-json")
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("schema-invalid")
	}
	return nil
}
func schemaFields(typ reflect.Type) map[string]reflect.Type {
	fields := map[string]reflect.Type{}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if field.Anonymous && name == "" {
			embedded := field.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				for key, value := range schemaFields(embedded) {
					fields[key] = value
				}
				continue
			}
		}
		if name == "" {
			name = field.Name
		}
		fields[name] = field.Type
	}
	return fields
}
func uniqueJSON(d *json.Decoder, depth int, typ reflect.Type) error {
	if depth > 64 {
		return fmt.Errorf("json-depth-exceeded")
	}
	t, err := d.Token()
	if err != nil {
		return fmt.Errorf("json-invalid")
	}
	switch t {
	case json.Delim('{'):
		for typ != nil && typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		fields := map[string]reflect.Type{}
		if typ != nil && typ.Kind() == reflect.Struct {
			fields = schemaFields(typ)
		}
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return fmt.Errorf("json-invalid")
			}
			key, ok := k.(string)
			if !ok || seen[key] {
				return fmt.Errorf("duplicate-json-key")
			}
			seen[key] = true
			var child reflect.Type
			if typ != nil && typ.Kind() == reflect.Struct {
				var ok bool
				child, ok = fields[key]
				if !ok {
					return fmt.Errorf("schema-key-invalid")
				}
			} else if typ != nil && typ.Kind() == reflect.Map {
				child = typ.Elem()
			}
			if err := uniqueJSON(d, depth+1, child); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return fmt.Errorf("json-invalid")
		}
	case json.Delim('['):
		for typ != nil && typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		var child reflect.Type
		if typ != nil && (typ.Kind() == reflect.Array || typ.Kind() == reflect.Slice) {
			child = typ.Elem()
		}
		for d.More() {
			if err := uniqueJSON(d, depth+1, child); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return fmt.Errorf("json-invalid")
		}
	}
	return nil
}
func validPath(s string) bool {
	return relativePath.MatchString(s) && !strings.HasPrefix(s, "/") && path.Clean(s) == s && s != "." && s != ".." && !strings.HasPrefix(s, "../")
}
func stringsValid(ss []string, paths bool) bool {
	if len(ss) > 256 {
		return false
	}
	seen := map[string]bool{}
	for _, s := range ss {
		if seen[s] || (!paths && !identifier.MatchString(s)) || (paths && !validPath(s)) {
			return false
		}
		seen[s] = true
	}
	return true
}
func findingsValid(fs []connector.Finding) bool {
	if len(fs) > 64 {
		return false
	}
	seen := map[string]bool{}
	for _, f := range fs {
		if !validPath(f.Path) || f.Line < 1 || !slices.Contains([]string{"ordinary", "security"}, f.Classification) || !slices.Contains([]string{"defect", "scope-mismatch", "coverage-gap", "security"}, f.Class) || seen[valueHash(f)] {
			return false
		}
		seen[valueHash(f)] = true
	}
	return true
}
func expectedValues(e Expected) map[string]any {
	return map[string]any{"affected_flows": e.AffectedFlows, "followup": e.Followup, "test_gaps": e.TestGaps, "defects": e.Defects}
}
func validateInputs(f Fixture, p Policy, registry Registry) error {
	if f.Profile != Profile || p.Profile != Profile || f.Connector.Forge.Change.Binding != p.Connector.Expected || !identifier.MatchString(f.SourceItem) {
		return fmt.Errorf("fixture-policy-binding-invalid")
	}
	if err := connector.ValidatePolicy(p.Connector); err != nil {
		return err
	}
	if !filepath.IsAbs(p.Executable) || filepath.Clean(p.Executable) != p.Executable || !hash.MatchString(p.ExecutableSHA256) || !hash.MatchString(p.RuntimeSHA256) || len(p.Args) > 32 {
		return fmt.Errorf("driver-policy-invalid")
	}
	for _, arg := range p.Args {
		if len(arg) > 2048 || strings.ContainsAny(arg, "\x00\r\n") {
			return fmt.Errorf("driver-argument-invalid")
		}
	}
	if !stringsValid(f.Expected.AffectedFlows, false) || !stringsValid(f.Expected.TestGaps, false) || !findingsValid(f.Expected.Defects) || len(f.Expected.Labels) != 4 {
		return fmt.Errorf("expectations-invalid")
	}
	if p.Registry != "" && (!filepath.IsAbs(p.Registry) || !hash.MatchString(p.RegistrySHA256) || registry.Profile != Profile || len(registry.Approvals) > 1024) {
		return fmt.Errorf("registry-invalid")
	}
	if p.Registry == "" && p.RegistrySHA256 != "" {
		return fmt.Errorf("registry-invalid")
	}
	for field, value := range expectedValues(f.Expected) {
		label, ok := f.Expected.Labels[field]
		if !ok || !hash.MatchString(label.EvidenceSHA256) {
			return fmt.Errorf("label-invalid")
		}
		switch label.Basis {
		case "generated":
			if label.Human != "" || label.Approval != "" {
				return fmt.Errorf("generated-label-promoted")
			}
		case "human-verified":
			if p.Registry == "" || !identifier.MatchString(label.Human) || !identifier.MatchString(label.Approval) {
				return fmt.Errorf("human-label-unverified")
			}
			matches := 0
			for _, a := range registry.Approvals {
				if a.Binding == p.Connector.Expected && a.Field == field && a.ValueSHA256 == valueHash(value) && a.Label == label {
					matches++
				}
			}
			if matches != 1 {
				return fmt.Errorf("human-label-unverified")
			}
		default:
			return fmt.Errorf("label-basis-invalid")
		}
	}
	return nil
}

// Applicability belongs to the harness contract, never to adapter assertions.
func validateResult(r Result, request Request, source string) error {
	if r.Request != request || r.Input.Binding != request.Binding || r.Input.SourceItem != source {
		return fmt.Errorf("driver-binding-mismatch")
	}
	if !stringsValid(r.AffectedFlows, false) || !stringsValid(r.DocumentationTargets, true) || !stringsValid(r.TestGaps, false) || !findingsValid(r.Defects) {
		return fmt.Errorf("driver-outcomes-invalid")
	}
	if r.Input.Counts.Docs != len(r.DocumentationTargets) || r.Input.Counts.Tests != len(r.TestGaps) || r.Input.Counts.Corpus != 0 || valueHash(sortedFindings(r.Input.Findings)) != valueHash(sortedFindings(r.Defects)) {
		return fmt.Errorf("driver-outcomes-inconsistent")
	}
	if len(r.Input.Drafts) > 0 {
		return fmt.Errorf("draft-scope-validation-unavailable")
	}
	required := map[string]bool{"trigger": true, "intake": true, "delta": true, "followup": true, "findings": true, "metrics": true,
		"docs-author": len(r.DocumentationTargets) > 0, "docs-scope": len(r.DocumentationTargets) > 0, "docs-validation": len(r.DocumentationTargets) > 0,
		"tests-author": len(r.TestGaps) > 0, "tests-scope": len(r.TestGaps) > 0, "tests-validation": len(r.TestGaps) > 0, "draft-requests": false}
	if len(r.Stages) != len(required) {
		return fmt.Errorf("required-stage-missing")
	}
	seen := map[string]bool{}
	for _, stage := range r.Stages {
		needed, ok := required[stage.Name]
		if !ok || seen[stage.Name] {
			return fmt.Errorf("stage-invalid")
		}
		seen[stage.Name] = true
		if needed {
			if stage.Status != "observed" || !hash.MatchString(stage.ArtifactSHA256) {
				return fmt.Errorf("required-stage-blocked")
			}
		} else if stage.Status != "not-applicable" || stage.ArtifactSHA256 != "" {
			return fmt.Errorf("stage-applicability-invalid")
		}
	}
	// Author/scope/validation adapters are not integrated yet. A driver cannot
	// self-certify their positive result with a digest or omit the expected draft.
	if len(r.DocumentationTargets) > 0 || len(r.TestGaps) > 0 {
		return fmt.Errorf("draft-scope-validation-unavailable")
	}
	return nil
}
func sortedFindings(in []connector.Finding) []connector.Finding {
	out := slices.Clone(in)
	if out == nil {
		out = []connector.Finding{}
	}
	slices.SortFunc(out, func(a, b connector.Finding) int { return strings.Compare(valueHash(a), valueHash(b)) })
	return out
}
func normalized(in []string) []string {
	out := slices.Clone(in)
	if out == nil {
		out = []string{}
	}
	slices.Sort(out)
	return out
}
func normalize(r Result) Result {
	r.AffectedFlows = normalized(r.AffectedFlows)
	r.DocumentationTargets = normalized(r.DocumentationTargets)
	r.TestGaps = normalized(r.TestGaps)
	r.Defects = sortedFindings(r.Defects)
	r.Input.Findings = sortedFindings(r.Input.Findings)
	slices.SortFunc(r.Stages, func(a, b Stage) int { return strings.Compare(a.Name, b.Name) })
	return r
}
