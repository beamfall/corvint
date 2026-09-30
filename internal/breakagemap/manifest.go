// Package breakagemap composes explicitly scoped immutable witnesses. A map is
// advisory evidence of relationships, never proof that an API change breaks them.
package breakagemap

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

const Schema = "corvint-breakage-map/0"
const MaxBytes = 1 << 20
const MaxSources = 256
const MaxEdges = 2000

var oid = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var identifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

type Manifest struct {
	Schema       string            `json:"schema"`
	Repositories []Repository      `json:"repositories"`
	Sources      []Source          `json:"sources"`
	Providers    []json.RawMessage `json:"providers"`
}
type Repository struct {
	ID     string `json:"id"`
	Origin string `json:"origin"`
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
}
type Source struct {
	Repository string `json:"repository"`
	Path       string `json:"path"`
	Blob       string `json:"blob"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
}

func sourceKey(repository, p string) string { return repository + ":" + p }
func safePath(p string) bool {
	return p != "" && len(p) <= 1024 && strings.Count(p, "/") < 32 && p != "." && path.Clean(p) == p && !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "../") && !strings.ContainsAny(p, "\\:\x00\r\n\t") && !strings.ContainsAny(p, "*?[")
}

// Decode accepts one strict document. The shared wire reader rejects duplicate
// members and excessive nesting before the typed decoder can lose information.
func Decode(data []byte) (Manifest, error) {
	var m Manifest
	if len(data) > MaxBytes || !utf8.Valid(data) {
		return m, errors.New("manifest bytes outside bounds")
	}
	if secretscreen.MatchString(string(data)) {
		return m, errors.New("manifest contains secret-shaped content")
	}
	if _, err := wire.Parse(data); err != nil {
		return m, errors.New("invalid manifest JSON")
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil || !lowerKeys(raw) {
		return m, errors.New("noncanonical manifest member")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return m, errors.New("invalid manifest fields")
	}
	if d.Decode(new(any)) != io.EOF {
		return m, errors.New("trailing manifest data")
	}
	if m.Schema != "corvint-breakage-manifest/0" || len(m.Repositories) < 1 || len(m.Repositories) > 8 || len(m.Sources) < 1 || len(m.Sources) > MaxSources || len(m.Providers) > 4 {
		return m, errors.New("manifest profile or count outside bounds")
	}
	repos := map[string]bool{}
	origins := map[string]bool{}
	for _, r := range m.Repositories {
		if !identifier.MatchString(r.ID) || !oid.MatchString(r.Origin) || !oid.MatchString(r.Commit) || !oid.MatchString(r.Tree) || repos[r.ID] || origins[r.Origin] {
			return m, errors.New("invalid or ambiguous repository identity")
		}
		repos[r.ID] = true
		origins[r.Origin] = true
	}
	seen := map[string]bool{}
	for _, s := range m.Sources {
		k := sourceKey(s.Repository, s.Path)
		if !repos[s.Repository] || !safePath(s.Path) || !oid.MatchString(s.Blob) || s.Start < 1 || s.End < s.Start || s.End > 200000 || seen[k] {
			return m, errors.New("invalid or duplicate source pin")
		}
		seen[k] = true
	}
	for _, p := range m.Providers {
		if _, err := extevidence.Decode1(p); err != nil {
			return m, errors.New("invalid EEP V1/V2 record")
		}
	}
	return m, nil
}

// encoding/json otherwise accepts case-insensitive struct field aliases.
func lowerKeys(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if k != strings.ToLower(k) || !lowerKeys(child) {
				return false
			}
		}
	case []any:
		for _, child := range x {
			if !lowerKeys(child) {
				return false
			}
		}
	}
	return true
}
