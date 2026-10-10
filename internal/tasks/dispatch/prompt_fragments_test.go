//go:build darwin || linux

package dispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// fragmentRaw builds configuration JSON from testConfig with members
// overridden: prompts (nil omits it) and per-role prompt JSON.
func fragmentRaw(t *testing.T, c *Config, prompts string, rolePrompts ...string) []byte {
	t.Helper()
	var top map[string]json.RawMessage
	raw, _ := json.Marshal(c)
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	if prompts != "" {
		top["prompts"] = json.RawMessage(prompts)
	}
	var roles []map[string]json.RawMessage
	if err := json.Unmarshal(top["roles"], &roles); err != nil {
		t.Fatal(err)
	}
	for i, p := range rolePrompts {
		roles[i]["prompt"] = json.RawMessage(p)
	}
	top["roles"], _ = json.Marshal(roles)
	out, err := json.Marshal(top)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func quote(s string) string { b, _ := json.Marshal(s); return string(b) }

// CAL-V0-175, CAL-V0-176: a config whose roles reference a shared fragment
// decodes to exactly the Config of the inline equivalent, so the rendered
// prompt is byte-identical; 21 roles repeating a 12.5 KiB rules block exceed
// MaxConfig inline but fit once the block is shared.
func TestCALV0175_FragmentPromptEqualsInline(t *testing.T) {
	rules := strings.Repeat("Rule: keep evidence, never guess {ticketLocal}.\n", 260) // ~12.5 KiB
	base := testConfig(t, "exit 0")
	base.Hosts["sh"] = Host{Argv: []string{base.Hosts["sh"].Argv[0], "{prompt}"}}
	base.GlobalCap = 64
	inline, fragment := *base, *base
	inline.Roles, fragment.Roles = nil, nil
	var refs []string
	for i := 0; i < 21; i++ {
		r := base.Roles[0]
		r.Name = fmt.Sprintf("r%d", i)
		own := fmt.Sprintf("Role %d works on {ticketLocal} as {holder}.\n", i)
		r.Prompt = own + rules + "End."
		inline.Roles = append(inline.Roles, r)
		r.Prompt = "placeholder"
		fragment.Roles = append(fragment.Roles, r)
		refs = append(refs, `[`+quote(own)+`,{"fragment":"rules"},"End."]`)
	}
	inlineRaw, _ := json.Marshal(&inline)
	if len(inlineRaw) <= MaxConfig {
		t.Fatalf("inline fixture is only %d bytes", len(inlineRaw))
	}
	if _, err := DecodeConfig(inlineRaw); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized inline config: %v", err)
	}
	fragRaw := fragmentRaw(t, &fragment, `{"rules":`+quote(rules)+`}`, refs...)
	if len(fragRaw) > MaxConfig/4 {
		t.Fatalf("fragment config is %d bytes", len(fragRaw))
	}
	got, err := DecodeConfig(fragRaw)
	if err != nil {
		t.Fatalf("fragment config refused: %v", err)
	}
	// The same expanded text under the limit, decoded inline, is the oracle.
	inline.Roles = inline.Roles[:2]
	got.Roles = got.Roles[:2]
	smallInline, _ := json.Marshal(&inline)
	want, err := DecodeConfig(smallInline)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expanded config differs from inline:\n%+v\n%+v", got.Roles[0], want.Roles[0])
	}
	values := map[string]string{"{ticketLocal}": "T-1", "{holder}": "h"}
	if a, b := Render(got.Roles[1].Prompt, values), Render(want.Roles[1].Prompt, values); a != b || !strings.HasPrefix(a, "Role 1 works on T-1 as h.\nRule:") {
		t.Fatalf("rendered prompts differ or are wrong: %q", a[:40])
	}
	again, _ := json.Marshal(got)
	if _, err := DecodeConfig(again); err != nil || bytes.Contains(again, []byte(`"prompts"`)) {
		t.Fatalf("decoded config does not round-trip as inline: %v", err)
	}
}

// CAL-V0-176: a config without fragments decodes exactly as before, and
// string prompts that merely look like references stay literal.
func TestCALV0176_ConfigWithoutFragmentsUnchanged(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].Prompt = `@rules {"fragment":"rules"} work on {ticketLocal}`
	raw, _ := json.Marshal(c)
	got, err := DecodeConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, c) {
		t.Fatalf("decoded %+v, want %+v", got.Roles[0], c.Roles[0])
	}
	for name, p := range map[string]string{"number": `7`, "object": `{"fragment":"x"}`, "empty": `""`, "null": `null`} {
		if _, err := DecodeConfig(fragmentRaw(t, c, "", p)); err == nil {
			t.Errorf("%s prompt accepted", name)
		}
	}
	// A repeated prompt member decodes per occurrence, as the string field
	// did: null keeps the earlier text, and an earlier bad type still fails.
	work := `"prompt":` + quote(c.Roles[0].Prompt)
	if got, err := DecodeConfig(bytes.Replace(raw, []byte(work), []byte(work+`,"prompt":null`), 1)); err != nil || got.Roles[0].Prompt != c.Roles[0].Prompt {
		t.Fatalf("prompt then null: %v", err)
	}
	if _, err := DecodeConfig(bytes.Replace(raw, []byte(work), []byte(`"prompt":123,`+work), 1)); err == nil {
		t.Fatal("number then prompt accepted")
	}
	// A repeated roles member decodes into the same elements, as before: a
	// second role object that omits budget keeps the first one's.
	c.Roles[0].Budget = &Budget{SessionsPerDay: 3}
	withBudget, _ := json.Marshal(c)
	second := `,"roles":[{"name":"impl","host":"sh","cap":2,"match":{},"idleSeconds":30,"wallSeconds":60}]}`
	twice := append(bytes.TrimSuffix(withBudget, []byte("}")), second...)
	if got, err := DecodeConfig(twice); err != nil || got.Roles[0].Budget == nil || got.Roles[0].Prompt != c.Roles[0].Prompt {
		t.Fatalf("repeated roles member: %v %+v", err, got)
	}
	c.Roles[0].Budget = nil
	unknown := bytes.Replace(raw, []byte(`"name":"impl"`), []byte(`"name":"impl","extra":1`), 1)
	if _, err := DecodeConfig(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown role member: %v", err)
	}
}

// CAL-V0-177: malformed, unknown, empty, oversized, nested, repeated and
// unreferenced fragments refuse at load, naming the role and fragment.
func TestCALV0177_FragmentRefusals(t *testing.T) {
	c := testConfig(t, "exit 0")
	ok := `{"rules":"Rules.\n"}`
	big := quote(strings.Repeat("x", 64<<10+1))
	half := quote(strings.Repeat("x", 40<<10))
	if _, err := DecodeConfig(fragmentRaw(t, c, ok, `[{"fragment":"rules"},"work {ticketLocal}"]`)); err != nil {
		t.Fatalf("valid reference refused: %v", err)
	}
	for name, tc := range map[string]struct{ prompts, prompt, want string }{
		"unknown":       {ok, `[{"fragment":"rule"}]`, `role impl prompt references unknown fragment "rule"`},
		"empty name":    {ok, `[{"fragment":""},{"fragment":"rules"}]`, `role impl prompt references unknown fragment ""`},
		"empty text":    {`{"rules":""}`, `[{"fragment":"rules"}]`, `prompt fragment rules must be 1..65536 bytes`},
		"oversized":     {`{"rules":` + big + `}`, `[{"fragment":"rules"}]`, `prompt fragment rules must be 1..65536 bytes`},
		"expanded size": {`{"a":` + half + `,"b":` + half + `}`, `[{"fragment":"a"},{"fragment":"b"}]`, `role impl prompt must be 1..65536 bytes after fragment expansion; fragment "b" passes the limit`},
		"nested":        {`{"rules":{"fragment":"other"}}`, `[{"fragment":"rules"}]`, `prompt fragment "rules" must be a string (fragments do not nest)`},
		"list value":    {`{"rules":["a"]}`, `[{"fragment":"rules"}]`, `prompt fragment "rules" must be a string`},
		"repeated":      {`{"rules":"a","rules":"b"}`, `[{"fragment":"rules"}]`, `prompt fragment "rules" is repeated`},
		"unreferenced":  {`{"rules":"a","spare":"b"}`, `[{"fragment":"rules"}]`, `prompt fragment spare is referenced by no role`},
		"bad name":      {`{"Rules":"a"}`, `[{"fragment":"Rules"}]`, `prompt fragment name "Rules" must match`},
		"not object":    {`["a"]`, `[{"fragment":"rules"}]`, `prompts must be an object`},
		"null prompts":  {`null`, `"x"`, `prompts must be an object`},
		"empty array":   {ok, `[]`, `role impl prompt: array needs 1..64 parts`},
		"empty part":    {ok, `["",{"fragment":"rules"}]`, `role impl prompt: part 0 must be`},
		"extra member":  {ok, `[{"fragment":"rules","x":1}]`, `role impl prompt: part 0 must be`},
		"folded member": {ok, `[{"Fragment":"rules"}]`, `role impl prompt: part 0 must be`},
		"repeat member": {ok, `[{"fragment":"x","fragment":"rules"}]`, `role impl prompt: part 0 must be`},
		"number part":   {ok, `[{"fragment":"rules"},3]`, `role impl prompt: part 1 must be`},
		"too many":      {ok, `[` + strings.Repeat(`"a",`, 64) + `{"fragment":"rules"}]`, `array needs 1..64 parts`},
	} {
		_, err := DecodeConfig(fragmentRaw(t, c, tc.prompts, tc.prompt))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", name, err, tc.want)
		}
	}
	roles := testConfig(t, "exit 0")
	for i := 1; i < 33; i++ {
		r := roles.Roles[0]
		r.Name = fmt.Sprintf("r%d", i)
		roles.Roles = append(roles.Roles, r)
	}
	refs := make([]string, 33)
	for i := range refs {
		refs[i] = `[{"fragment":"missing"}]` // the role bound refuses before expansion runs
	}
	if _, err := DecodeConfig(fragmentRaw(t, roles, ok, refs...)); err == nil || !strings.Contains(err.Error(), "roles needs 1..32 entries") {
		t.Errorf("33 roles: %v", err)
	}
	many := map[string]string{}
	for i := 0; i < 33; i++ {
		many[fmt.Sprintf("f%d", i)] = "x"
	}
	raw, _ := json.Marshal(many)
	if _, err := DecodeConfig(fragmentRaw(t, c, string(raw), `[{"fragment":"f0"}]`)); err == nil || !strings.Contains(err.Error(), "at most 32 fragments") {
		t.Errorf("33 fragments: %v", err)
	}
}

// CAL-V0-178: placeholder rules apply to the expanded prompt, so a fragment
// cannot carry {prompt}, an unknown placeholder, an {operatorNote} for an
// unsafe host, or a {model}/{effort} the role cannot deliver.
func TestCALV0178_FragmentCannotSmugglePlaceholders(t *testing.T) {
	for name, tc := range map[string]struct {
		text string
		host []string
	}{
		"prompt":           {"{prompt}", nil},
		"unknown":          {"{secret}", nil},
		"model":            {"{model}", nil},
		"effort":           {"{effort}", nil},
		"note unsafe host": {"{operatorNote}", []string{"{ticket}", "{prompt}"}},
	} {
		c := testConfig(t, "exit 0")
		if tc.host != nil {
			c.Hosts["sh"] = Host{Argv: append([]string{c.Hosts["sh"].Argv[0]}, tc.host...)}
		}
		inline := *c
		inline.Roles = []Role{c.Roles[0]}
		inline.Roles[0].Prompt = "work " + tc.text
		rawInline, _ := json.Marshal(&inline)
		_, inlineErr := DecodeConfig(rawInline)
		_, fragErr := DecodeConfig(fragmentRaw(t, c, `{"x":`+quote(tc.text)+`}`, `["work ",{"fragment":"x"}]`))
		if inlineErr == nil || fragErr == nil || inlineErr.Error() != fragErr.Error() {
			t.Errorf("%s: inline %v, fragment %v", name, inlineErr, fragErr)
		}
	}
	c := testConfig(t, "exit 0")
	if _, err := DecodeConfig(fragmentRaw(t, c, `{"note":"{operatorNote}"}`, `["work {ticketLocal}\n",{"fragment":"note"}]`)); err != nil {
		t.Fatalf("{operatorNote} fragment for a safe host refused: %v", err)
	}
}

// CAL-V0-176: an idle-reload of a fragment config applies the expanded
// prompt, and a broken reference is refused with the role and fragment
// recorded, the applied configuration kept.
func TestCALV0176_ReloadExpandsFragments(t *testing.T) {
	c := testConfig(t, "exit 0")
	f := newReloadFixture(t, c, &fakeQueue{})
	f.writeRaw(fragmentRaw(t, c, `{"rules":"Rules.\n"}`, `[{"fragment":"rules"},"changed {ticketLocal}"]`))
	f.tick()
	if got := f.d.Config.Roles[0].Prompt; got != "Rules.\nchanged {ticketLocal}" {
		t.Fatalf("reloaded prompt %q", got)
	}
	f.writeRaw(fragmentRaw(t, c, `{"rules":"Rules.\n"}`, `[{"fragment":"rulez"}]`))
	f.tick()
	r := f.d.ledger.Config.Refused
	if r == nil || !strings.Contains(r.Reason, `role impl prompt references unknown fragment "rulez"`) || f.d.Config.Roles[0].Prompt != "Rules.\nchanged {ticketLocal}" {
		t.Fatalf("refusal %+v, applied %q", r, f.d.Config.Roles[0].Prompt)
	}
}
