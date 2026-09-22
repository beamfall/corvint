package contextindex

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strconv"
)

// CanonicalJSON reproduces Python json.dumps(value, sort_keys=True,
// separators=(",", ":")) with its default ensure_ascii=True behavior.
func CanonicalJSON(value any) ([]byte, error) {
	var output bytes.Buffer
	if err := appendJSON(&output, reflect.ValueOf(value)); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func appendJSON(output *bytes.Buffer, value reflect.Value) error {
	if !value.IsValid() || (value.Kind() == reflect.Interface && value.IsNil()) {
		output.WriteString("null")
		return nil
	}
	if value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			output.WriteString("null")
			return nil
		}
		return appendJSON(output, value.Elem())
	}
	if value.CanInterface() {
		if integer, ok := value.Interface().(pythonInteger); ok {
			output.WriteString(string(integer))
			return nil
		}
	}
	switch value.Kind() {
	case reflect.Bool:
		if value.Bool() {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case reflect.String:
		appendJSONString(output, value.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		output.WriteString(strconv.FormatInt(value.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		output.WriteString(strconv.FormatUint(value.Uint(), 10))
	case reflect.Slice, reflect.Array:
		output.WriteByte('[')
		for index := 0; index < value.Len(); index++ {
			if index != 0 {
				output.WriteByte(',')
			}
			if err := appendJSON(output, value.Index(index)); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("canonical JSON map key must be a string")
		}
		keys := value.MapKeys()
		sort.Slice(keys, func(left, right int) bool { return keys[left].String() < keys[right].String() })
		output.WriteByte('{')
		for index, key := range keys {
			if index != 0 {
				output.WriteByte(',')
			}
			appendJSONString(output, key.String())
			output.WriteByte(':')
			if err := appendJSON(output, value.MapIndex(key)); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("unsupported canonical JSON type %s", value.Type())
	}
	return nil
}

func appendJSONString(output *bytes.Buffer, value string) {
	output.WriteByte('"')
	for _, character := range value {
		switch character {
		case '"':
			output.WriteString(`\"`)
		case '\\':
			output.WriteString(`\\`)
		case '\b':
			output.WriteString(`\b`)
		case '\f':
			output.WriteString(`\f`)
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			switch {
			case character >= 0x20 && character <= 0x7e:
				output.WriteRune(character)
			case character <= 0xffff:
				fmt.Fprintf(output, `\u%04x`, character)
			default:
				character -= 0x10000
				fmt.Fprintf(output, `\u%04x\u%04x`, 0xd800+(character>>10), 0xdc00+(character&0x3ff))
			}
		}
	}
	output.WriteByte('"')
}
