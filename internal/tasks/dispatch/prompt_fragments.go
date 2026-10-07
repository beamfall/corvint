package dispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Shared prompt fragments (CAL-V0-175..178, proposed): a top-level
// `prompts` map holds text once, and a role `prompt` may be an array of
// literal strings and {"fragment": NAME} references. DecodeConfig joins the
// parts with no separator before any role check, so the role prompt the
// dispatcher validates and renders is byte-identical to the inline form.

const (
	maxFragments     = 32
	maxFragmentBytes = 64 << 10
	maxPromptParts   = 64
	maxRolePrompt    = 64 << 10
)

// PromptFragments is the closed top-level fragment map. Its decoder refuses
// a repeated name and a value that is not a JSON string, naming the fragment,
// so a fragment can never hold a reference to another.
type PromptFragments map[string]string

func (p *PromptFragments) UnmarshalJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	if t, err := d.Token(); err != nil || t != json.Delim('{') {
		return fmt.Errorf("prompts must be an object of fragment name to text")
	}
	out := PromptFragments{}
	for d.More() {
		t, err := d.Token()
		if err != nil {
			return fmt.Errorf("prompts: %w", err)
		}
		name, _ := t.(string)
		if _, dup := out[name]; dup {
			return fmt.Errorf("prompt fragment %q is repeated", name)
		}
		var v json.RawMessage
		if err := d.Decode(&v); err != nil {
			return fmt.Errorf("prompts: %w", err)
		}
		var s string
		if len(v) == 0 || v[0] != '"' || json.Unmarshal(v, &s) != nil {
			return fmt.Errorf("prompt fragment %q must be a string (fragments do not nest)", name)
		}
		out[name] = s
	}
	*p = out
	return nil
}

// promptPart is one element of an array role prompt: literal text or a
// fragment reference.
type promptPart struct {
	text     string
	fragment string
}

// UnmarshalJSON keeps the closed Role member set and admits a prompt that is
// either a string (unchanged behaviour) or an array of parts.
func (r *Role) UnmarshalJSON(raw []byte) error {
	type plain Role
	var v struct {
		plain
		Prompt json.RawMessage `json:"prompt"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return err
	}
	out := Role(v.plain)
	out.Prompt, out.promptParts = "", nil
	p := bytes.TrimSpace(v.Prompt)
	switch {
	case len(p) == 0 || string(p) == "null":
	case p[0] == '"':
		if err := json.Unmarshal(p, &out.Prompt); err != nil {
			return err
		}
	case p[0] == '[':
		parts, err := decodePromptParts(p)
		if err != nil {
			return fmt.Errorf("role %s prompt: %v", out.Name, err)
		}
		out.promptParts = parts
	default:
		return fmt.Errorf("role %s prompt must be a string or an array of parts", out.Name)
	}
	*r = out
	return nil
}

func decodePromptParts(raw []byte) ([]promptPart, error) {
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return nil, err
	}
	if len(elems) == 0 || len(elems) > maxPromptParts {
		return nil, fmt.Errorf("array needs 1..%d parts", maxPromptParts)
	}
	parts := make([]promptPart, 0, len(elems))
	for i, e := range elems {
		var part promptPart
		if e[0] == '"' {
			if json.Unmarshal(e, &part.text) != nil || part.text == "" {
				return nil, fmt.Errorf("part %d must be a non-empty string or {\"fragment\": NAME}", i)
			}
		} else {
			var ref struct {
				Fragment *string `json:"fragment"`
			}
			rd := json.NewDecoder(bytes.NewReader(e))
			rd.DisallowUnknownFields()
			if e[0] != '{' || rd.Decode(&ref) != nil || ref.Fragment == nil || !exactFragmentMember(e) {
				return nil, fmt.Errorf("part %d must be a non-empty string or {\"fragment\": NAME}", i)
			}
			part.fragment = *ref.Fragment
		}
		parts = append(parts, part)
	}
	return parts, nil
}

// exactFragmentMember requires the reference object to spell "fragment"
// exactly once; encoding/json would match it case-insensitively and take
// the last repeat.
func exactFragmentMember(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	if t, err := d.Token(); err != nil || t != json.Delim('{') {
		return false
	}
	n := 0
	for d.More() {
		t, err := d.Token()
		if err != nil || t != "fragment" {
			return false
		}
		n++
		var skip json.RawMessage
		if d.Decode(&skip) != nil {
			return false
		}
	}
	return n == 1
}

// expandPrompts resolves every array role prompt against c.Prompts, then
// drops the map and the parts, so the decoded Config is the inline
// equivalent. A fragment no role references is refused (CAL-V0-177).
func (c *Config) expandPrompts() error {
	fail := func(f string, a ...any) error { return fmt.Errorf("dispatch config: "+f, a...) }
	if len(c.Prompts) > maxFragments {
		return fail("prompts holds at most %d fragments", maxFragments)
	}
	names := make([]string, 0, len(c.Prompts))
	for name, text := range c.Prompts {
		names = append(names, name)
		if !ValidName(name) {
			return fail("prompt fragment name %q must match [a-z][a-z0-9-]{0,23}", name)
		}
		if text == "" || len(text) > maxFragmentBytes {
			return fail("prompt fragment %s must be 1..%d bytes", name, maxFragmentBytes)
		}
	}
	sort.Strings(names)
	used := map[string]bool{}
	for i := range c.Roles {
		r := &c.Roles[i]
		if r.promptParts == nil {
			continue
		}
		var b strings.Builder
		for _, p := range r.promptParts {
			text := p.text
			if p.fragment != "" || p.text == "" {
				var ok bool
				if text, ok = c.Prompts[p.fragment]; !ok {
					return fail("role %s prompt references unknown fragment %q", r.Name, p.fragment)
				}
				used[p.fragment] = true
			}
			if b.Len()+len(text) > maxRolePrompt {
				return fail("role %s prompt must be 1..%d bytes after fragment expansion", r.Name, maxRolePrompt)
			}
			b.WriteString(text)
		}
		r.Prompt, r.promptParts = b.String(), nil
	}
	for _, name := range names {
		if !used[name] {
			return fail("prompt fragment %s is referenced by no role", name)
		}
	}
	c.Prompts = nil
	return nil
}
