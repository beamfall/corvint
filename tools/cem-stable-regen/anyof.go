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
	anyOfStatus = "SCHEMA_ANYOF_BRANCH_ADDED"
	// anyOfPointer marks the one README paragraph the first recorded anyOf
	// branch adds.
	anyOfPointer = "Recorded schema anyOf branches:"
)

// anyOfRecord is the --any-of --record file: the reason, the JSON pointer of
// one object-schema node of RESULT.schema.json, the sha256 of the compact JSON
// of that node, and the JSON object schema it must also admit (CEM-V1-880).
type anyOfRecord struct {
	Reason            string          `json:"reason"`
	Pointer           string          `json:"pointer"`
	PredecessorSHA256 string          `json:"predecessorSha256"`
	Branch            json.RawMessage `json:"branch"`
}

func loadAnyOfRecord(file string) (*anyOfRecord, []byte, *jnode, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, nil, err
	}
	if _, err := parseJSON(src); err != nil {
		return nil, nil, nil, fmt.Errorf("--record: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(src))
	dec.DisallowUnknownFields()
	var r anyOfRecord
	if err := dec.Decode(&r); err != nil {
		return nil, nil, nil, fmt.Errorf("--record: %w", err)
	}
	switch {
	case strings.TrimSpace(r.Reason) == "":
		return nil, nil, nil, errors.New("--record has no reason")
	case !strings.HasPrefix(r.Pointer, "/"):
		return nil, nil, nil, fmt.Errorf("--record pointer %q is not a non-root JSON pointer", r.Pointer)
	}
	branch, err := parseJSON(r.Branch)
	if err != nil || branch.kind != '{' || len(branch.keys) == 0 {
		return nil, nil, nil, errors.New("--record branch is not a non-empty JSON object schema")
	}
	return &r, src, branch, nil
}

// alternatives returns the existing anyOf alternatives of n, or n itself when
// n is not exactly {"anyOf": [...]}. An anyOf beside any other keyword is
// refused, so a branch never changes what a sibling keyword constrains.
func alternatives(n *jnode) (alts []*jnode, wrap bool, err error) {
	if n == nil || n.kind != '{' {
		return nil, false, errors.New("the node is not an object schema")
	}
	i := n.index("anyOf")
	if i < 0 {
		return []*jnode{n}, true, nil
	}
	if len(n.keys) != 1 {
		return nil, false, errors.New("the node has anyOf beside another keyword")
	}
	if a := n.elems[i]; a.kind == '[' && len(a.elems) > 0 {
		return a.elems, false, nil
	}
	return nil, false, errors.New("the node's anyOf is not a non-empty array")
}

// anyOf adds one recorded alternative to one object-schema node of both
// RESULT.schema.json copies (CEM-V1-880): the old node's bytes stay, only
// re-indented when the node is first wrapped, and the same chain is re-bound
// as for the other modes.
func anyOf(t *tree, o options) (string, error) {
	if problems := check(t); len(problems) > 0 {
		return "", fmt.Errorf("the packet is not self-consistent before adding a branch: %s", strings.Join(problems, "; "))
	}
	rec, recSrc, branch, err := loadAnyOfRecord(o.record)
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
	alts, wrap, err := alternatives(node)
	if err != nil {
		return "", fmt.Errorf("pointer %q: %w", rec.Pointer, err)
	}
	if got := valueDigest(node.raw(schema.src)); got != rec.PredecessorSHA256 {
		return "", fmt.Errorf("pointer %q: the record names predecessor %s, the schema holds %s", rec.Pointer, rec.PredecessorSHA256, got)
	}
	branchDigest := valueDigest(rec.Branch)
	for _, a := range alts {
		if valueDigest(a.raw(schema.src)) == branchDigest {
			return "", fmt.Errorf("pointer %q already admits the recorded branch", rec.Pointer)
		}
	}
	indent := lineIndent(schema.src, parent.kspan[i][0])
	inner := indent + "    "
	var text strings.Builder
	text.WriteString("{\n" + indent + "  \"anyOf\": [\n")
	for _, a := range alts {
		raw := string(a.raw(schema.src))
		if wrap {
			raw = strings.ReplaceAll(raw, "\n", "\n    ")
		}
		text.WriteString(inner + raw + ",\n")
	}
	text.WriteString(inner + render(rec.Branch, branch, inner) + "\n")
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
	if !bytes.Contains(readmeOld, []byte(anyOfPointer)) {
		readmeNew := append(append([]byte(nil), readmeOld...), []byte("\n"+anyOfPointer+" a revision may also add one alternative to an object\nschema node of RESULT.schema.json with `make cem-stable-regen-any-of`\n(CEM-V1-880), which keeps the old node and every case and cell, records the\npredecessor digest of the node and the reason, and appends a section to\nCHANGES.md.\n")...)
		t.set(packetDir+"/"+readmeName, readmeNew)
		packetChanged[readmeName] = readmeNew
	}

	cases, err := t.read(packetDir + "/" + casesName)
	if err != nil {
		return "", err
	}
	packetManifestOld, _ := t.read(packetDir + "/manifest.json")
	oldSum, newSum, err := sealChain(t, packetManifestOld, packetChanged, cases, cases, nil, anyOfStatus, func(changesOld []byte, outside map[string][]byte) []byte {
		var b strings.Builder
		fmt.Fprintf(&b, "\n# Recorded schema anyOf branch at `%s` (%s)\n\n", rec.Pointer, o.date)
		fmt.Fprintf(&b, "Base: the public packet with manifest sha256\n%s.\n", digest(packetManifestOld))
		fmt.Fprintf(&b, "Reason: %s\n", rec.Reason)
		fmt.Fprintf(&b, "Record file sha256: %s.\n", digest(recSrc))
		b.WriteString("Produced by `make cem-stable-regen-any-of` (tools/cem-stable-regen, CEM-V1-880).\n\n")
		fmt.Fprintf(&b, "- RESULT.schema.json `%s` had %d alternative(s); it now has %d, the last\n  being the recorded branch.\n", rec.Pointer, len(alts), len(alts)+1)
		fmt.Fprintf(&b, "- Node sha256 (compact JSON): %s -> %s; branch sha256 %s.\n", rec.PredecessorSHA256, newDigest, branchDigest)
		b.WriteString("- No member, case, cell, exit or other schema byte changed; every frozen\n  expectation stays valid because the schema only admits more.\n\n")
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
		return "", fmt.Errorf("the packet with the added branch fails its own check: %s", strings.Join(problems, "; "))
	}
	var report strings.Builder
	fmt.Fprintf(&report, "cem-stable-regen: added an anyOf branch at %s\n", rec.Pointer)
	fmt.Fprintf(&report, "packet manifest %s -> %s\n", oldSum, newSum)
	for _, name := range sortedKeys(t.next) {
		fmt.Fprintln(&report, "wrote "+name)
	}
	return report.String(), nil
}
