package evalrepo

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

func canonicalJSON(value any) ([]byte, error) {
	return encodeJSON(value, false)
}

func weightedJSON(value any) ([]byte, error) {
	return encodeJSON(value, true)
}

func encodeJSON(value any, spaced bool) ([]byte, error) {
	var output bytes.Buffer
	if err := appendJSON(&output, reflect.ValueOf(value), spaced); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func appendJSON(output *bytes.Buffer, value reflect.Value, spaced bool) error {
	if !value.IsValid() || (value.Kind() == reflect.Interface && value.IsNil()) {
		output.WriteString("null")
		return nil
	}
	if value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			output.WriteString("null")
			return nil
		}
		return appendJSON(output, value.Elem(), spaced)
	}
	switch value.Kind() {
	case reflect.Bool:
		output.WriteString(strconv.FormatBool(value.Bool()))
	case reflect.String:
		appendJSONString(output, value.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		output.WriteString(strconv.FormatInt(value.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		output.WriteString(strconv.FormatUint(value.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		floating := value.Float()
		if math.IsNaN(floating) || math.IsInf(floating, 0) {
			return fmt.Errorf("non-finite JSON number")
		}
		encoded := strconv.FormatFloat(floating, 'f', -1, value.Type().Bits())
		if !strings.Contains(encoded, ".") {
			encoded += ".0"
		}
		output.WriteString(encoded)
	case reflect.Slice, reflect.Array:
		output.WriteByte('[')
		for index := 0; index < value.Len(); index++ {
			if index != 0 {
				output.WriteByte(',')
				if spaced {
					output.WriteByte(' ')
				}
			}
			if err := appendJSON(output, value.Index(index), spaced); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("JSON map key must be a string")
		}
		keys := value.MapKeys()
		sort.Slice(keys, func(left, right int) bool { return keys[left].String() < keys[right].String() })
		output.WriteByte('{')
		for index, key := range keys {
			if index != 0 {
				output.WriteByte(',')
				if spaced {
					output.WriteByte(' ')
				}
			}
			appendJSONString(output, key.String())
			output.WriteByte(':')
			if spaced {
				output.WriteByte(' ')
			}
			if err := appendJSON(output, value.MapIndex(key), spaced); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("unsupported JSON type %s", value.Type())
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
