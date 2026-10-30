package tcq

import (
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// preflightJSON enforces the raw byte, depth, and aggregate member ceilings with
// a bounded string-aware scan. TCQ-V0-041 requires this to run *before* semantic
// JSON parsing so a hostile document is rejected before any unbounded tree is
// allocated.
func preflightJSON(raw []byte, bounds jsonBounds) error {
	if len(raw) > bounds.bytes {
		return fail(CodeResourceExhausted)
	}
	scan := preflightScan{bounds: bounds}
	for _, current := range raw {
		if err := scan.step(current); err != nil {
			return err
		}
	}
	return nil
}

// frame tracks one open aggregate: objects only count `:`, arrays count the
// first byte of each item.
type frame struct {
	isArray    bool
	expectItem bool
}

type preflightScan struct {
	bounds      jsonBounds
	stack       []frame
	members     int
	inString    bool
	escaped     bool
	numberBytes int
}

func (scan *preflightScan) step(current byte) error {
	if scan.inString {
		scan.stepString(current)
		return nil
	}
	if isJSONSpace(current) {
		scan.numberBytes = 0
		return nil
	}
	if err := scan.stepNumber(current); err != nil {
		return err
	}
	if err := scan.countArrayItem(current); err != nil {
		return err
	}
	return scan.stepStructure(current)
}

func (scan *preflightScan) stepString(current byte) {
	switch {
	case scan.escaped:
		scan.escaped = false
	case current == '\\':
		scan.escaped = true
	case current == '"':
		scan.inString = false
	}
}

func (scan *preflightScan) stepNumber(current byte) error {
	continues := scan.numberBytes > 0 && isNumberByte(current)
	starts := scan.numberBytes == 0 && (current == '-' || (current >= '0' && current <= '9'))
	if !continues && !starts {
		scan.numberBytes = 0
		return nil
	}
	scan.numberBytes++
	if scan.numberBytes > maxJSONNumberBytes {
		return fail(CodeResourceExhausted)
	}
	return nil
}

func (scan *preflightScan) countArrayItem(current byte) error {
	top := len(scan.stack) - 1
	if top < 0 || !scan.stack[top].isArray || !scan.stack[top].expectItem {
		return nil
	}
	if current == ',' || current == ']' {
		return nil
	}
	scan.stack[top].expectItem = false
	return scan.count()
}

func (scan *preflightScan) stepStructure(current byte) error {
	switch current {
	case '"':
		scan.inString = true
	case '{':
		return scan.push(frame{})
	case '[':
		return scan.push(frame{isArray: true, expectItem: true})
	case '}', ']':
		if len(scan.stack) > 0 {
			scan.stack = scan.stack[:len(scan.stack)-1]
		}
	case ':':
		return scan.count()
	case ',':
		if top := len(scan.stack) - 1; top >= 0 && scan.stack[top].isArray {
			scan.stack[top].expectItem = true
		}
	}
	return nil
}

func (scan *preflightScan) push(next frame) error {
	scan.stack = append(scan.stack, next)
	if len(scan.stack) > scan.bounds.depth {
		return fail(CodeResourceExhausted)
	}
	return nil
}

func (scan *preflightScan) count() error {
	scan.members++
	if scan.members > scan.bounds.members {
		return fail(CodeResourceExhausted)
	}
	return nil
}

func isJSONSpace(current byte) bool {
	return current == ' ' || current == '\t' || current == '\r' || current == '\n'
}

func isNumberByte(current byte) bool {
	switch {
	case current >= '0' && current <= '9':
		return true
	case current == '.' || current == 'e' || current == 'E' || current == '+' || current == '-':
		return true
	}
	return false
}

// parseCanonical is the TCQ-V0-042 step-3 gate for one artifact: bounded
// preflight, then strict JSON (UTF-8, no duplicate keys, integers only), then a
// byte-equality re-encode. A document that parses but is not in the frozen
// encoding is `noncanonical-*`, never silently normalized.
func parseCanonical(raw []byte, bounds jsonBounds, invalidCode, noncanonicalCode string) (wire.Value, error) {
	if err := preflightJSON(raw, bounds); err != nil {
		return wire.Value{}, err
	}
	value, err := wire.Parse(raw)
	if err != nil {
		return wire.Value{}, fail(invalidCode)
	}
	if value.Kind != wire.KindObject {
		return wire.Value{}, fail(invalidCode)
	}
	if string(canonicalJSON(value)) != string(raw) {
		return wire.Value{}, fail(noncanonicalCode)
	}
	return value, nil
}

// exactObject enforces a closed schema: exactly these keys, no more and no fewer.
func exactObject(value wire.Value, fields []string, code string) (*wire.Object, error) {
	if value.Kind != wire.KindObject || len(value.Obj.Keys) != len(fields) {
		return nil, fail(code)
	}
	for _, field := range fields {
		if _, ok := value.Obj.Get(field); !ok {
			return nil, fail(code)
		}
	}
	return value.Obj, nil
}

func stringField(object *wire.Object, name, code string) (string, error) {
	value, ok := object.Get(name)
	if !ok || value.Kind != wire.KindString {
		return "", fail(code)
	}
	return value.Str, nil
}

func intField(object *wire.Object, name string, low, high int64, code string) (int64, error) {
	value, ok := object.Get(name)
	if !ok || value.Kind != wire.KindInt || value.Int < low || value.Int > high {
		return 0, fail(code)
	}
	return value.Int, nil
}
