// SPDX-License-Identifier: Apache-2.0

// Package runnerprotocol hosts the optional, shadow-only corvint-runner/1
// profile. It never executes runner commands or admits selection authority.
package runnerprotocol

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"reflect"
	"strings"

	"github.com/Beamfall/corvint/internal/testevidence"
)

const Protocol = "corvint-runner/1"
const DiscoverySchema = "corvint-runner-plugins/1"

// Schema is the closed JSON Schema subset used by the conformance kit and host.
// The same definitions validate both directions and are exportable for other
// implementations; no permissive second decoder exists in the host.
type Schema struct {
	Type       string             `json:"type,omitempty"`
	AnyOf      []*Schema          `json:"anyOf,omitempty"`
	Properties map[string]*Schema `json:"properties,omitempty"`
	Required   []string           `json:"required,omitempty"`
	Additional any                `json:"additionalProperties,omitempty"`
	Items      *Schema            `json:"items,omitempty"`
	Enum       []string           `json:"enum,omitempty"`
}

func obj(fields map[string]*Schema, optional ...string) *Schema {
	s := &Schema{Type: "object", Properties: fields, Additional: false}
	for k := range fields {
		if !contains(optional, k) {
			s.Required = append(s.Required, k)
		}
	}
	sortStrings(s.Required)
	return s
}
func str() *Schema                    { return &Schema{Type: "string"} }
func arr(s *Schema) *Schema           { return &Schema{Type: "array", Items: s} }
func dict(s *Schema) *Schema          { return &Schema{Type: "object", Additional: s} }
func enumeration(v ...string) *Schema { return &Schema{Type: "string", Enum: v} }
func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
func reflected(t reflect.Type) *Schema {
	switch t.Kind() {
	case reflect.Struct:
		fields := map[string]*Schema{}
		var optional []string
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")
			fields[tag[0]] = reflected(f.Type)
			if f.Type.Kind() == reflect.Slice {
				fields[tag[0]] = &Schema{AnyOf: []*Schema{fields[tag[0]], {Type: "null"}}}
			}
			if len(tag) > 1 && tag[1] == "omitempty" {
				optional = append(optional, tag[0])
			}
		}
		return obj(fields, optional...)
	case reflect.Slice:
		return arr(reflected(t.Elem()))
	case reflect.String:
		return str()
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	default:
		return &Schema{Type: "integer"}
	}
}
func nativeSchema() *Schema {
	return obj(map[string]*Schema{
		"test":     str(),
		"executed": reflected(reflect.TypeFor[testevidence.Component[testevidence.ExecutedFile]]()),
		"regions":  reflected(reflect.TypeFor[testevidence.Component[testevidence.RegionsFile]]()),
		"loaded":   reflected(reflect.TypeFor[testevidence.Component[testevidence.Blob]]()),
	})
}

// Schemas returns independent request and result schemas for every method.
func Schemas() (map[string]*Schema, map[string]*Schema) {
	requests := map[string]*Schema{
		"hello":     obj(map[string]*Schema{"protocols": arr(str()), "core": str()}),
		"command":   obj(map[string]*Schema{"runner": str(), "purpose": enumeration("inventory", "run", "collect"), "tests": arr(str()), "run_dir": str()}, "tests"),
		"inventory": obj(map[string]*Schema{"runner": str(), "output": str()}),
		"parse":     obj(map[string]*Schema{"runner": str(), "report": str()}),
		"evidence":  obj(map[string]*Schema{"runner": str(), "artifacts": arr(str())}),
		"shutdown":  obj(map[string]*Schema{}),
	}
	results := map[string]*Schema{
		"hello":     obj(map[string]*Schema{"protocol": str(), "plugin": str(), "version": str(), "runners": arr(str()), "pieces": dict(arr(enumeration("inventory", "test-evidence", "qualification"))), "features": arr(str())}),
		"command":   obj(map[string]*Schema{"argv": arr(str()), "cwd": str(), "env": dict(str()), "artifacts": arr(str()), "superset": {Type: "boolean"}}),
		"inventory": obj(map[string]*Schema{"config_digest": str(), "tests": arr(obj(map[string]*Schema{"id": str(), "path": str()}))}),
		"parse":     obj(map[string]*Schema{"results": arr(obj(map[string]*Schema{"id": str(), "outcome": enumeration("passed", "failed", "skipped"), "reason": str()}, "reason"))}),
		"evidence":  obj(map[string]*Schema{"records": arr(nativeSchema())}),
		"shutdown":  obj(map[string]*Schema{}),
	}
	return requests, results
}
func (s *Schema) validate(v any) error {
	if len(s.AnyOf) > 0 {
		for _, choice := range s.AnyOf {
			if choice.validate(v) == nil {
				return nil
			}
		}
		return fmt.Errorf("schema-violation:union")
	}
	bad := func() error { return fmt.Errorf("schema-violation:%s", s.Type) }
	switch s.Type {
	case "object":
		m, ok := v.(map[string]any)
		if !ok {
			return bad()
		}
		for _, k := range s.Required {
			if _, ok := m[k]; !ok {
				return fmt.Errorf("schema-violation:missing:%s", k)
			}
		}
		for k, x := range m {
			rule, ok := s.Properties[k]
			if !ok {
				rule, ok = s.Additional.(*Schema)
			}
			if !ok {
				return fmt.Errorf("schema-violation:unknown:%s", k)
			}
			if err := rule.validate(x); err != nil {
				return err
			}
		}
	case "array":
		a, ok := v.([]any)
		if !ok {
			return bad()
		}
		for _, x := range a {
			if err := s.Items.validate(x); err != nil {
				return err
			}
		}
	case "string":
		x, ok := v.(string)
		if !ok || strings.ContainsRune(x, 0) {
			return bad()
		}
		if len(s.Enum) > 0 && !contains(s.Enum, x) {
			return bad()
		}
	case "null":
		if v != nil {
			return bad()
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return bad()
		}
	case "integer":
		x, ok := v.(float64)
		if !ok || x != float64(int64(x)) {
			return bad()
		}
	default:
		return bad()
	}
	return nil
}
func decode(data []byte, s *Schema, out any) error {
	var value any
	// v2 rejects duplicate members and invalid UTF-8, including nested objects.
	if err := jsonv2.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("protocol-error:%w", err)
	}
	if err := s.validate(value); err != nil {
		return err
	}
	if err := jsonv2.Unmarshal(data, out, jsonv2.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("schema-violation:%w", err)
	}
	return nil
}
func validatePayload(method string, data []byte, result bool, out any) error {
	requests, results := Schemas()
	schema := requests[method]
	if result {
		schema = results[method]
	}
	if schema == nil {
		return fmt.Errorf("protocol-error:unknown-method")
	}
	return decode(data, schema, out)
}

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}
type response struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *remoteError    `json:"error,omitempty"`
}
type remoteError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func decodeRequest(data []byte) (request, error) {
	var r request
	// Decode the envelope separately: params are validated by the selected method.
	var fields map[string]json.RawMessage
	if err := jsonv2.Unmarshal(data, &fields); err != nil {
		return r, fmt.Errorf("protocol-error:%w", err)
	}
	if len(fields) != 3 || fields["id"] == nil || fields["method"] == nil || fields["params"] == nil {
		return r, fmt.Errorf("protocol-error:request-envelope")
	}
	if err := jsonv2.Unmarshal(data, &r, jsonv2.RejectUnknownMembers(true)); err != nil {
		return r, fmt.Errorf("protocol-error:%w", err)
	}
	if r.ID == "" {
		return r, fmt.Errorf("protocol-error:empty-id")
	}
	var v any
	err := validatePayload(r.Method, r.Params, false, &v)
	return r, err
}
func decodeResponse(data []byte, id, method string, out any) error {
	var fields map[string]json.RawMessage
	if err := jsonv2.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("protocol-error:%w", err)
	}
	if len(fields) != 2 || fields["id"] == nil || (fields["result"] == nil) == (fields["error"] == nil) {
		return fmt.Errorf("protocol-error:response-envelope")
	}
	var r response
	if err := jsonv2.Unmarshal(data, &r, jsonv2.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("protocol-error:%w", err)
	}
	if r.ID != id || id == "" {
		return fmt.Errorf("protocol-error:response-id")
	}
	if fields["error"] != nil {
		var e remoteError
		if err := decode(fields["error"], obj(map[string]*Schema{"code": str(), "message": str()}), &e); err != nil {
			return err
		}
		return fmt.Errorf("error-response:%s:%s", e.Code, e.Message)
	}
	return validatePayload(method, r.Result, true, out)
}
