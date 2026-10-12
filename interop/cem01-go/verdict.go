package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// verdictEnvironment names the stable result members the verdict digest
// excludes, so re-verification on another admitted Go tuple yields the same
// digest (CEM-V1-066).
var verdictEnvironment = []string{"runtime", "repositoryEnvelope", "verdictSha256"}

// MarshalJSON emits the stable result with verdictSha256 computed from the
// other members, on every outcome (CEM-V1-066).
func (r stableResult) MarshalJSON() ([]byte, error) {
	type plain stableResult
	p := plain(r)
	p.VerdictSHA256 = ""
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	if p.VerdictSHA256, err = verdictDigest(raw); err != nil {
		return nil, err
	}
	return json.Marshal(p)
}

// verdictDigest is the SHA-256 of the canonical JSON value of an encoded
// stable result with the environment members removed: sorted keys, minimal
// separators, decimal integers and the canonical string escapes.
func verdictDigest(raw []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return "", err
	}
	object, ok := decoded.(map[string]any)
	if !ok {
		return "", errors.New("stable result is not a JSON object")
	}
	for _, key := range verdictEnvironment {
		delete(object, key)
	}
	var b strings.Builder
	if err := writeCanonicalJSON(&b, object); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:]), nil
}

func writeCanonicalJSON(b *strings.Builder, v any) error {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		b.WriteString(canonicalString(x))
	case json.Number:
		n, err := x.Int64()
		if err != nil {
			return fmt.Errorf("stable result number %s is not an integer", x)
		}
		fmt.Fprintf(b, "%d", n)
	case []any:
		b.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeCanonicalJSON(b, item); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for key := range x {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(canonicalString(key))
			b.WriteByte(':')
			if err := writeCanonicalJSON(b, x[key]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return fmt.Errorf("stable result value of type %T", v)
	}
	return nil
}
