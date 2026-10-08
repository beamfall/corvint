package testrunner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	cw "github.com/Beamfall/corvint/internal/cem/wire"
)

// PlanDocument and ReceiptDocument preserve the runner companion's exact JSON
// field order: plan identity is the digest of this native serialization.
type PlanDocument struct {
	Profile    string     `json:"profile"`
	Request    Request    `json:"request"`
	Invocation Invocation `json:"invocation"`
}
type ReceiptDocument struct {
	Profile     string      `json:"profile"`
	PlanSha256  string      `json:"planSha256"`
	Execution   Execution   `json:"execution"`
	Observation Observation `json:"observation"`
	Error       string      `json:"error"`
}

// DecodeDocument rejects duplicate, trailing and unknown fields, including in
// nested tool declarations. Documents remain operator declarations, not proof.
func DecodeDocument(b []byte, out any) error {
	if len(b) > MaxReportBytes {
		return fmt.Errorf("document bound")
	}
	v, e := cw.Parse(b)
	if e != nil {
		return e
	}
	t := reflect.TypeOf(out)
	if t == nil || t.Kind() != reflect.Pointer {
		return fmt.Errorf("document destination")
	}
	if e = closedFields(v, t.Elem()); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(out)
}
func closedFields(v cw.Value, t reflect.Type) error {
	switch t.Kind() {
	case reflect.Struct:
		if v.Kind != cw.KindObject {
			return fmt.Errorf("expected object")
		}
		fields := map[string]reflect.Type{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name != "-" {
				fields[name] = f.Type
			}
		}
		for _, k := range v.Obj.Keys {
			ft, ok := fields[k]
			if !ok {
				return fmt.Errorf("unknown field %q", k)
			}
			if e := closedFields(v.Obj.Values[k], ft); e != nil {
				return e
			}
		}
	case reflect.Pointer:
		if v.Kind != cw.KindNull {
			return closedFields(v, t.Elem())
		}
	case reflect.Map:
		if v.Kind == cw.KindObject {
			for _, k := range v.Obj.Keys {
				if e := closedFields(v.Obj.Values[k], t.Elem()); e != nil {
					return e
				}
			}
		}
	case reflect.Slice:
		// []byte is encoded as a base64 string by encoding/json.
		if t.Elem().Kind() != reflect.Uint8 && v.Kind == cw.KindArray {
			for _, x := range v.Arr {
				if e := closedFields(x, t.Elem()); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
