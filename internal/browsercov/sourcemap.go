// SPDX-License-Identifier: Apache-2.0

package browsercov

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// sourceMap is a decoded Source Map v3: for each generated line, the
// segments that map a generated column to an original position.
type sourceMap struct {
	Sources        []string
	SourcesContent []*string
	SourceRoot     string
	Lines          [][]segment // by 0-based generated line, sorted by Col
}

type segment struct {
	Col           int // generated column, UTF-16 units
	Src, OrigLine int // -1 Src for a segment that maps nowhere
}

type rawMap struct {
	Version        int               `json:"version"`
	Sources        []string          `json:"sources"`
	SourcesContent []*string         `json:"sourcesContent"`
	SourceRoot     string            `json:"sourceRoot"`
	Mappings       string            `json:"mappings"`
	Sections       []json.RawMessage `json:"sections"`
}

func parseSourceMap(text []byte) (*sourceMap, error) {
	var r rawMap
	if err := json.Unmarshal(text, &r); err != nil {
		return nil, fmt.Errorf("source map: %v", err)
	}
	if r.Version != 3 {
		return nil, fmt.Errorf("source map version %d", r.Version)
	}
	if len(r.Sections) > 0 {
		return nil, errors.New("indexed source map (sections) is not supported")
	}
	m := &sourceMap{Sources: r.Sources, SourcesContent: r.SourcesContent, SourceRoot: r.SourceRoot}
	var src, oline, ocol, name int
	for _, line := range strings.Split(r.Mappings, ";") {
		var segs []segment
		col := 0
		for _, field := range strings.Split(line, ",") {
			if field == "" {
				continue
			}
			v, err := decodeVLQ(field)
			if err != nil {
				return nil, err
			}
			col += v[0]
			s := segment{Col: col, Src: -1}
			switch len(v) {
			case 1:
			case 4, 5:
				src += v[1]
				oline += v[2]
				ocol += v[3]
				if len(v) == 5 {
					name += v[4]
				}
				if src < 0 || src >= len(r.Sources) || oline < 0 {
					return nil, fmt.Errorf("source map segment %q out of range", field)
				}
				s.Src, s.OrigLine = src, oline
			default:
				return nil, fmt.Errorf("source map segment %q has %d fields", field, len(v))
			}
			segs = append(segs, s)
		}
		m.Lines = append(m.Lines, segs)
	}
	return m, nil
}

const b64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func decodeVLQ(s string) ([]int, error) {
	var out []int
	v, shift := 0, 0
	for i := 0; i < len(s); i++ {
		d := strings.IndexByte(b64, s[i])
		if d < 0 {
			return nil, fmt.Errorf("source map: bad VLQ %q", s)
		}
		v += (d & 31) << shift
		if d&32 != 0 {
			shift += 5
			if shift > 30 {
				return nil, fmt.Errorf("source map: VLQ overflow %q", s)
			}
			continue
		}
		n := v >> 1
		if v&1 != 0 {
			n = -n
		}
		out = append(out, n)
		v, shift = 0, 0
	}
	if shift != 0 {
		return nil, fmt.Errorf("source map: truncated VLQ %q", s)
	}
	return out, nil
}

// inlineMap returns the data: source map a script names in its last
// sourceMappingURL comment, or ok=false when it names none or an external one.
func inlineMap(source string) (text []byte, ok bool, err error) {
	ref := mapURL(source)
	if !strings.HasPrefix(ref, "data:") {
		return nil, false, nil
	}
	meta, data, found := strings.Cut(strings.TrimPrefix(ref, "data:"), ",")
	if !found {
		return nil, true, errors.New("malformed data: source map URL")
	}
	if strings.HasSuffix(meta, ";base64") {
		b, err := base64.StdEncoding.DecodeString(data)
		return b, true, err
	}
	s, err := url.PathUnescape(data)
	return []byte(s), true, err
}

func mapURL(source string) string {
	i := max(strings.LastIndex(source, "//# sourceMappingURL="), strings.LastIndex(source, "//@ sourceMappingURL="))
	if i < 0 {
		return ""
	}
	ref := source[i+len("//# sourceMappingURL="):]
	if j := strings.IndexAny(ref, " \t\r\n'\""); j >= 0 {
		ref = ref[:j]
	}
	return ref
}
