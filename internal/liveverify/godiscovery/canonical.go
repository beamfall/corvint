package godiscovery

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
	"sort"
	"unicode/utf8"
)

func Hash(kind, profile string, body []byte) (string, error) {
	if kind == "" || profile == "" || !validText(kind) || !validText(profile) {
		return "", errors.New("invalid hash domain")
	}
	value := sha256.New()
	writeLength32(value, len(kind))
	_, _ = value.Write([]byte(kind))
	writeLength32(value, len(profile))
	_, _ = value.Write([]byte(profile))
	writeLength64(value, len(body))
	_, _ = value.Write(body)
	return hex.EncodeToString(value.Sum(nil)), nil
}

func prefixedID(kind, profile string, body map[string]any) (string, []byte, error) {
	canonical, err := canonicalJSON(body)
	if err != nil {
		return "", nil, err
	}
	digest, err := Hash(kind, profile, canonical)
	if err != nil {
		return "", nil, err
	}
	return kind + ":sha256:" + digest, canonical, nil
}

func bareDigest(kind, profile string, body any) (string, error) {
	canonical, err := canonicalJSON(body)
	if err != nil {
		return "", err
	}
	return Hash(kind, profile, canonical)
}

func writeLength32(value hash.Hash, length int) {
	var buffer [4]byte
	binary.BigEndian.PutUint32(buffer[:], uint32(length))
	_, _ = value.Write(buffer[:])
}

func writeLength64(value hash.Hash, length int) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(length))
	_, _ = value.Write(buffer[:])
}

func canonicalJSON(value any) ([]byte, error) {
	var output bytes.Buffer
	if err := appendCanonical(&output, value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func appendCanonical(output *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case nil:
		output.WriteString("null")
	case bool:
		if typed {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case string:
		if !validText(typed) {
			return errors.New("invalid canonical string")
		}
		appendJSONString(output, typed)
	case []string:
		output.WriteByte('[')
		for index, item := range typed {
			if index != 0 {
				output.WriteByte(',')
			}
			if !validText(item) {
				return errors.New("invalid canonical string")
			}
			appendJSONString(output, item)
		}
		output.WriteByte(']')
	case []any:
		output.WriteByte('[')
		for index, item := range typed {
			if index != 0 {
				output.WriteByte(',')
			}
			if err := appendCanonical(output, item); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			if !validText(key) {
				return errors.New("invalid canonical key")
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		output.WriteByte('{')
		for index, key := range keys {
			if index != 0 {
				output.WriteByte(',')
			}
			appendJSONString(output, key)
			output.WriteByte(':')
			if err := appendCanonical(output, typed[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return errors.New("unsupported canonical value")
	}
	return nil
}

func appendJSONString(output *bytes.Buffer, value string) {
	const hexadecimal = "0123456789abcdef"
	output.WriteByte('"')
	for _, character := range []byte(value) {
		switch character {
		case '"', '\\':
			output.WriteByte('\\')
			output.WriteByte(character)
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
			if character < 0x20 {
				output.WriteString(`\u00`)
				output.WriteByte(hexadecimal[character>>4])
				output.WriteByte(hexadecimal[character&0x0f])
			} else {
				output.WriteByte(character)
			}
		}
	}
	output.WriteByte('"')
}

func validText(value string) bool {
	if len(value) > MaxStringBytes || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character >= 0xfdd0 && character <= 0xfdef {
			return false
		}
		if character&0xffff == 0xfffe || character&0xffff == 0xffff {
			return false
		}
	}
	return true
}
