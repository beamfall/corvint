package verify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// verdictEnvironment names the result members the verdict digest excludes:
// the consumer runtime, the repository-envelope label and the digest itself,
// so re-verification on another admitted Go tuple yields the same digest
// (CEM-V1-066).
var verdictEnvironment = []string{"runtime", "repositoryEnvelope", "verdictSha256"}

// MarshalJSON emits the result with verdictSha256 computed from the other
// members, on every outcome and every construction path (CEM-V1-066).
func (r StableResult) MarshalJSON() ([]byte, error) {
	type plain StableResult
	p := plain(r)
	p.VerdictSha256 = ""
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	if p.VerdictSha256, err = verdictDigest(raw); err != nil {
		return nil, err
	}
	return json.Marshal(p)
}

// VerdictDigest returns the verdictSha256 the result encodes with: the SHA-256
// of the canonical JSON value of the result minus runtime, repositoryEnvelope
// and verdictSha256.
func (r StableResult) VerdictDigest() (string, error) {
	type plain StableResult
	raw, err := json.Marshal(plain(r))
	if err != nil {
		return "", err
	}
	return verdictDigest(raw)
}

// verdictDigest hashes the canonical-json-value encoding of the encoded
// result object with the environment members removed.
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
	value, err := verdictValue(object)
	if err != nil {
		return "", err
	}
	return stableDigest(wire.CanonicalValue(value)), nil
}

// verdictValue converts a UseNumber-decoded JSON value to a wire value. The
// stable result carries only integers, some of them negative, so any other
// number is an error rather than a silently rounded digest input.
func verdictValue(v any) (wire.Value, error) {
	switch x := v.(type) {
	case nil:
		return wire.Value{Kind: wire.KindNull}, nil
	case bool:
		return wire.Value{Kind: wire.KindBool, Bool: x}, nil
	case string:
		return wire.Value{Kind: wire.KindString, Str: x}, nil
	case json.Number:
		n, err := x.Int64()
		if err != nil {
			return wire.Value{}, fmt.Errorf("stable result number %s is not an integer", x)
		}
		return wire.Value{Kind: wire.KindInt, Int: n}, nil
	case []any:
		items := make([]wire.Value, len(x))
		for i, item := range x {
			value, err := verdictValue(item)
			if err != nil {
				return wire.Value{}, err
			}
			items[i] = value
		}
		return wire.Value{Kind: wire.KindArray, Arr: items}, nil
	case map[string]any:
		object := &wire.Object{Values: map[string]wire.Value{}}
		for key, item := range x {
			value, err := verdictValue(item)
			if err != nil {
				return wire.Value{}, err
			}
			object.Keys = append(object.Keys, key)
			object.Values[key] = value
		}
		return wire.Value{Kind: wire.KindObject, Obj: object}, nil
	}
	return wire.Value{}, fmt.Errorf("stable result value of type %T", v)
}
