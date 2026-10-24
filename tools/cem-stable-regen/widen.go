package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	widenStatus = "SCHEMA_ENUM_WIDENED"
	// widenPointer marks the one README paragraph the first recorded widening
	// adds.
	widenPointer = "Recorded schema enum widenings:"
)

// widenRecord is the --widen --record file: the reason, the JSON pointer of one
// string-valued `const` or `enum` node in RESULT.schema.json, the sha256 of the
// compact JSON of that node, and the values it must also admit (CEM-V1-620).
type widenRecord struct {
	Reason            string   `json:"reason"`
	Pointer           string   `json:"pointer"`
	PredecessorSHA256 string   `json:"predecessorSha256"`
	Add               []string `json:"add"`
}

func loadWidenRecord(file string) (*widenRecord, []byte, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, err
	}
	if _, err := parseJSON(src); err != nil {
		return nil, nil, fmt.Errorf("--record: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(src))
	dec.DisallowUnknownFields()
	var r widenRecord
	if err := dec.Decode(&r); err != nil {
		return nil, nil, fmt.Errorf("--record: %w", err)
	}
	switch {
	case strings.TrimSpace(r.Reason) == "":
		return nil, nil, errors.New("--record has no reason")
	case !strings.HasPrefix(r.Pointer, "/"):
		return nil, nil, fmt.Errorf("--record pointer %q is not a non-root JSON pointer", r.Pointer)
	case len(r.Add) == 0:
		return nil, nil, errors.New("--record adds no value")
	}
	seen := map[string]bool{}
	for _, v := range r.Add {
		if v == "" || seen[v] {
			return nil, nil, fmt.Errorf("--record add value %q is empty or repeated", v)
		}
		seen[v] = true
	}
	return &r, src, nil
}

// resolveMember follows an RFC 6901 pointer through object members only and
// returns the named node and the object that holds it.
func resolveMember(root *jnode, src []byte, pointer string) (parent *jnode, index int, err error) {
	n := root
	tokens := strings.Split(pointer[1:], "/")
	for i, raw := range tokens {
		if n == nil || n.kind != '{' {
			return nil, 0, fmt.Errorf("pointer %q does not name an object member at %q", pointer, raw)
		}
		key := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")
		j := n.index(key)
		if j < 0 {
			return nil, 0, fmt.Errorf("pointer %q names no member %q", pointer, key)
		}
		if i == len(tokens)-1 {
			return n, j, nil
		}
		n = n.elems[j]
	}
	return nil, 0, fmt.Errorf("pointer %q is empty", pointer)
}

// enumValues returns the admitted values of a node that is exactly
// {"const": STRING} or {"enum": [STRING, ...]}; any other node is refused, so a
// widening never edits a keyword it does not understand.
func enumValues(src []byte, n *jnode) ([]string, error) {
	if n == nil || n.kind != '{' || len(n.keys) != 1 {
		return nil, errors.New("the node is not exactly a string const or enum")
	}
	v := n.elems[0]
	switch {
	case n.keys[0] == "const" && v.kind == '"':
		return []string{v.text(src)}, nil
	case n.keys[0] == "enum" && v.kind == '[' && len(v.elems) > 0:
		var out []string
		for _, e := range v.elems {
			if e.kind != '"' {
				return nil, errors.New("the enum holds a value that is not a string")
			}
			out = append(out, e.text(src))
		}
		return out, nil
	}
	return nil, errors.New("the node is not exactly a string const or enum")
}

// widen adds recorded values to one string const or enum of both
// RESULT.schema.json copies (CEM-V1-620): every existing value, every other
// schema byte, every case and every cell stay identical, and the same chain is
// re-bound as for the other modes.
func widen(t *tree, o options) (string, error) {
	if problems := check(t); len(problems) > 0 {
		return "", fmt.Errorf("the packet is not self-consistent before widening: %s", strings.Join(problems, "; "))
	}
	rec, recSrc, err := loadWidenRecord(o.record)
	if err != nil {
		return "", err
	}
	schema, err := t.parse(stableDir + "/" + schemaName)
	if err != nil {
		return "", err
	}
	parent, i, err := resolveMember(schema.root, schema.src, rec.Pointer)
	if err != nil {
		return "", err
	}
	node := parent.elems[i]
	old, err := enumValues(schema.src, node)
	if err != nil {
		return "", fmt.Errorf("pointer %q: %w", rec.Pointer, err)
	}
	if got := valueDigest(node.raw(schema.src)); got != rec.PredecessorSHA256 {
		return "", fmt.Errorf("pointer %q: the record names predecessor %s, the schema holds %s", rec.Pointer, rec.PredecessorSHA256, got)
	}
	values := append([]string(nil), old...)
	for _, v := range rec.Add {
		for _, existing := range old {
			if v == existing {
				return "", fmt.Errorf("pointer %q already admits %q", rec.Pointer, v)
			}
		}
		values = append(values, v)
	}
	indent := lineIndent(schema.src, parent.kspan[i][0])
	var text strings.Builder
	text.WriteString("{\n" + indent + "  \"enum\": [\n")
	for k, v := range values {
		text.WriteString(indent + "    " + quote(v))
		if k < len(values)-1 {
			text.WriteByte(',')
		}
		text.WriteByte('\n')
	}
	text.WriteString(indent + "  ]\n" + indent + "}")
	schemaOld := schema.src
	schemaNew, err := applySplices(schemaOld, []splice{replaceValue(node, text.String())})
	if err != nil {
		return "", err
	}
	newDigest := valueDigest([]byte(text.String()))
	t.set(stableDir+"/"+schemaName, schemaNew)
	t.set(packetDir+"/"+schemaName, schemaNew)
	packetChanged := map[string][]byte{schemaName: schemaNew}

	readmeOld, err := t.read(packetDir + "/" + readmeName)
	if err != nil {
		return "", err
	}
	if !bytes.Contains(readmeOld, []byte(widenPointer)) {
		readmeNew := append(append([]byte(nil), readmeOld...), []byte("\n"+widenPointer+" a revision may also widen one string `const` or\n`enum` of RESULT.schema.json with `make cem-stable-regen-widen` (CEM-V1-620),\nwhich keeps every existing value, case and cell, records the predecessor digest\nof the widened node and the reason, and appends a section to CHANGES.md.\n")...)
		t.set(packetDir+"/"+readmeName, readmeNew)
		packetChanged[readmeName] = readmeNew
	}

	cases, err := t.read(packetDir + "/" + casesName)
	if err != nil {
		return "", err
	}
	packetManifestOld, _ := t.read(packetDir + "/manifest.json")
	oldSum, newSum, err := sealChain(t, packetManifestOld, packetChanged, cases, cases, nil, widenStatus, func(changesOld []byte, outside map[string][]byte) []byte {
		var b strings.Builder
		fmt.Fprintf(&b, "\n# Recorded schema enum widening of `%s` (%s)\n\n", rec.Pointer, o.date)
		fmt.Fprintf(&b, "Base: the public packet with manifest sha256\n%s.\n", digest(packetManifestOld))
		fmt.Fprintf(&b, "Reason: %s\n", rec.Reason)
		fmt.Fprintf(&b, "Record file sha256: %s.\n", digest(recSrc))
		b.WriteString("Produced by `make cem-stable-regen-widen` (tools/cem-stable-regen, CEM-V1-620).\n\n")
		fmt.Fprintf(&b, "- RESULT.schema.json `%s` admitted `%s`; it now admits `%s`.\n", rec.Pointer, strings.Join(old, "`, `"), strings.Join(values, "`, `"))
		fmt.Fprintf(&b, "- Widened node sha256 (compact JSON): %s -> %s.\n", rec.PredecessorSHA256, newDigest)
		b.WriteString("- No member, case, cell, exit or other schema byte changed; every frozen\n  expectation stays valid because no value was removed.\n\n")
		writeFileTables(&b, packetManifestOld, outside, map[string][2][]byte{
			schemaName:  {schemaOld, schemaNew},
			readmeName:  {readmeOld, packetChanged[readmeName]},
			changesName: {changesOld, nil},
		})
		fmt.Fprintf(&b, "| %s/%s | identical to %s/%s |\n", stableDir, schemaName, packetDir, schemaName)
		return []byte(b.String())
	})
	if err != nil {
		return "", err
	}
	if problems := check(t); len(problems) > 0 {
		return "", fmt.Errorf("widened packet fails its own check: %s", strings.Join(problems, "; "))
	}
	var report strings.Builder
	fmt.Fprintf(&report, "cem-stable-regen: widened %s by %s\n", rec.Pointer, strings.Join(rec.Add, ", "))
	fmt.Fprintf(&report, "packet manifest %s -> %s\n", oldSum, newSum)
	for _, name := range sortedKeys(t.next) {
		fmt.Fprintln(&report, "wrote "+name)
	}
	return report.String(), nil
}
