package testrunner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// KilledExitCode is the phase exit code of a process that reported no normal
// exit status: it was signal-terminated (runner timeout, interruption, output
// overflow or an external signal) or never started. It is the only negative
// integer a plan or receipt document carries, and only as an `exitCode` member.
const KilledExitCode = -1

var (
	// ErrNegativeInteger names a document negative integer other than an
	// `exitCode` member equal to KilledExitCode.
	ErrNegativeInteger = errors.New("negative-document-integer")
	// ErrKilledRunComplete names a receipt that claims a complete observation
	// although a phase was killed, timed out, interrupted or overflowed.
	ErrKilledRunComplete = errors.New("killed-run-complete")
)

// DecodeDocument rejects duplicate, trailing and unknown fields, including in
// nested tool declarations. Documents remain operator declarations, not proof.
// A receipt additionally passes CheckReceipt.
func DecodeDocument(b []byte, out any) error {
	if len(b) > MaxReportBytes {
		return fmt.Errorf("document bound")
	}
	structural, e := StructuralBytes(b)
	if e != nil {
		return e
	}
	v, e := cw.Parse(structural)
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
	if e = d.Decode(out); e != nil {
		return e
	}
	if r, ok := out.(*ReceiptDocument); ok {
		return CheckReceipt(*r)
	}
	return nil
}

// StructuralBytes returns b with the sign of each `"exitCode": -1` object
// member blanked, so the strict non-negative wire parser can check duplicate
// keys, closed fields and encoding while the typed decode keeps the original
// bytes. Any other negative number is refused with ErrNegativeInteger.
func StructuralBytes(b []byte) ([]byte, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	type frame struct{ object, key bool }
	var stack []frame
	key, out, cloned := "", b, false
	for {
		tok, e := d.Token()
		if e == io.EOF {
			return out, nil
		}
		if e != nil {
			return nil, e
		}
		if len(stack) == 0 && tok != json.Delim('{') && tok != json.Delim('[') {
			// A scalar document is refused by the typed decoders, not here.
			continue
		}
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{':
				stack = append(stack, frame{object: true, key: true})
			case '[':
				stack = append(stack, frame{})
			default:
				stack = stack[:len(stack)-1]
				if len(stack) > 0 && stack[len(stack)-1].object {
					stack[len(stack)-1].key = true
				}
			}
			continue
		}
		top := &stack[len(stack)-1]
		if top.object && top.key {
			key, top.key = tok.(string), false
			continue
		}
		if n, ok := tok.(json.Number); ok && strings.HasPrefix(string(n), "-") {
			if !top.object || key != "exitCode" || n != "-1" {
				return nil, fmt.Errorf("%w: %s", ErrNegativeInteger, n)
			}
			end := int(d.InputOffset())
			if out[end-2] != '-' {
				return nil, fmt.Errorf("%w: offset", ErrNegativeInteger)
			}
			if !cloned {
				out, cloned = bytes.Clone(b), true
			}
			out[end-2] = ' '
		}
		if top.object {
			top.key = true
		}
	}
}

// CheckReceipt refuses a receipt whose observation is complete although a
// phase did not exit normally within its bounds.
func CheckReceipt(r ReceiptDocument) error {
	for _, p := range r.Execution.Phases {
		if (p.ExitCode == KilledExitCode || p.TimedOut || p.Interrupted || p.Overflow) && r.Observation.Complete {
			return fmt.Errorf("%w: %s phase", ErrKilledRunComplete, p.Kind)
		}
	}
	return nil
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
