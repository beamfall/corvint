// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Fixtures are the verification half of this suite. Each fixture declares the
// seed universe to build (universe.go) and one or more cases; each case
// perturbs the map the real producers wrote for that universe in a closed
// vocabulary, offers it to the `ocm status` reader, and expects one exact
// outcome. The genuine, unperturbed map is the `valid` case of every fixture
// that declares one.

// Fixture is one directory under fixtures/.
type Fixture struct {
	// Dir is the fixture directory, filled in by LoadFixtures.
	Dir string `json:"-"`

	ID          string               `json:"id"`
	Note        string               `json:"note"`
	Obligations []DeclaredObligation `json:"obligations"`
	Cases       []Case               `json:"cases"`
}

// Case is one perturbation of the produced map and its expected verdict.
type Case struct {
	ID     string `json:"id"`
	Note   string `json:"note"`
	Ops    []Op   `json:"ops"`
	Expect Expect `json:"expect"`
}

// Op is one perturbation. Exactly one shape applies per op:
//
//	{"set": {"obligation": ID, "field": F, "value": V}}   set a scalar field of one row
//	{"set": {"obligation": ID, "field": F, "array": [..]}} set an array field of one row
//	{"swap": [ID, ID]}                                    exchange two rows' positions
//	{"drop": ID}                                          remove one row
//	{"append": {row object}}                              append a whole row
//	{"top": {"field": F, "value": V}}                     set a top-level string member
//	{"cem": {"field": F, "value": V}}                     set a member of the cem binding
type Op struct {
	Set    *SetOp         `json:"set,omitempty"`
	Swap   []string       `json:"swap,omitempty"`
	Drop   string         `json:"drop,omitempty"`
	Append map[string]any `json:"append,omitempty"`
	Top    *FieldOp       `json:"top,omitempty"`
	CEM    *FieldOp       `json:"cem,omitempty"`
}

// SetOp names one row field.
type SetOp struct {
	Obligation string   `json:"obligation"`
	Field      string   `json:"field"`
	Value      *string  `json:"value,omitempty"`
	Array      []string `json:"array,omitempty"`
}

// FieldOp names one member of an object.
type FieldOp struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// Expect is the exact verdict.
type Expect struct {
	// Target selects the caller target: "" or "target" for the produced
	// target, "successor" for the equal-tree successor commit.
	Target string `json:"target,omitempty"`
	// Refusal is the operational refusal code when the reader returns no
	// verdict at all.
	Refusal string `json:"refusal,omitempty"`
	State   string `json:"state,omitempty"`
	Code    string `json:"code,omitempty"`
	Linked  *int   `json:"linked,omitempty"`
	Unknown *int   `json:"unknown,omitempty"`
}

// LoadFixtures reads every fixtures/<id>/case.json under dir, sorted by ID.
func LoadFixtures(dir string) ([]Fixture, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "fixtures"))
	if err != nil {
		return nil, err
	}
	var fixtures []Fixture
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fixtureDir := filepath.Join(dir, "fixtures", entry.Name())
		var f Fixture
		if err := loadJSON(filepath.Join(fixtureDir, "case.json"), &f); err != nil {
			return nil, err
		}
		if f.ID != entry.Name() {
			return nil, fmt.Errorf("%s: id %q does not match directory", fixtureDir, f.ID)
		}
		f.Dir = fixtureDir
		fixtures = append(fixtures, f)
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].ID < fixtures[j].ID })
	return fixtures, nil
}

// Validate checks the fixture's data invariants.
func (f Fixture) Validate() error {
	if f.Note == "" || len(f.Obligations) == 0 || len(f.Cases) == 0 {
		return fmt.Errorf("fixture %q: note, obligations, and cases are required", f.ID)
	}
	seen := map[string]bool{}
	for _, c := range f.Cases {
		if c.ID == "" || c.Note == "" {
			return fmt.Errorf("fixture %q: every case needs an id and a note", f.ID)
		}
		if seen[c.ID] {
			return fmt.Errorf("fixture %q: duplicate case %q", f.ID, c.ID)
		}
		seen[c.ID] = true
		if err := c.Expect.validate(f.ID, c.ID); err != nil {
			return err
		}
		for _, op := range c.Ops {
			if err := op.validate(f.ID, c.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e Expect) validate(fixture, id string) error {
	if e.Refusal != "" && (e.State != "" || e.Code != "") {
		return fmt.Errorf("fixture %q case %q: refusal excludes state and code", fixture, id)
	}
	if e.Refusal == "" && e.State == "" {
		return fmt.Errorf("fixture %q case %q: expect a state or a refusal", fixture, id)
	}
	if e.State == "invalid" && e.Code == "" {
		return fmt.Errorf("fixture %q case %q: an invalid verdict names its code", fixture, id)
	}
	return nil
}

func (op Op) validate(fixture, id string) error {
	shapes := 0
	for _, present := range []bool{op.Set != nil, len(op.Swap) > 0, op.Drop != "", op.Append != nil, op.Top != nil, op.CEM != nil} {
		if present {
			shapes++
		}
	}
	if shapes != 1 {
		return fmt.Errorf("fixture %q case %q: every op has exactly one shape", fixture, id)
	}
	if len(op.Swap) != 0 && len(op.Swap) != 2 {
		return fmt.Errorf("fixture %q case %q: swap names two rows", fixture, id)
	}
	return nil
}

// Apply perturbs the produced map bytes and returns canonical bytes.
func (c Case) Apply(produced []byte) ([]byte, error) {
	root, err := wire.Parse(produced)
	if err != nil {
		return nil, err
	}
	for _, op := range c.Ops {
		if err := op.apply(root); err != nil {
			return nil, fmt.Errorf("case %q: %w", c.ID, err)
		}
	}
	return append(wire.CanonicalValue(root), '\n'), nil
}

func (op Op) apply(root wire.Value) error {
	switch {
	case op.Set != nil:
		return op.Set.apply(root)
	case len(op.Swap) == 2:
		return swapRows(root, op.Swap[0], op.Swap[1])
	case op.Drop != "":
		return dropRow(root, op.Drop)
	case op.Append != nil:
		return appendRow(root, op.Append)
	case op.Top != nil:
		setMember(root.Obj, op.Top.Field, stringValue(op.Top.Value))
	case op.CEM != nil:
		setMember(root.Obj.Values["cem"].Obj, op.CEM.Field, stringValue(op.CEM.Value))
	}
	return nil
}

func (s SetOp) apply(root wire.Value) error {
	row, err := findRow(root, s.Obligation)
	if err != nil {
		return err
	}
	if s.Value != nil {
		setMember(row.Obj, s.Field, stringValue(*s.Value))
		return nil
	}
	items := make([]wire.Value, 0, len(s.Array))
	for _, item := range s.Array {
		items = append(items, stringValue(item))
	}
	setMember(row.Obj, s.Field, wire.Value{Kind: wire.KindArray, Arr: items})
	return nil
}

func rows(root wire.Value) []wire.Value {
	return root.Obj.Values["obligations"].Arr
}

func setRows(root wire.Value, items []wire.Value) {
	setMember(root.Obj, "obligations", wire.Value{Kind: wire.KindArray, Arr: items})
}

// setMember sets one member, registering a new key so the canonical encoder
// emits it.
func setMember(object *wire.Object, key string, value wire.Value) {
	if _, present := object.Values[key]; !present {
		object.Keys = append(object.Keys, key)
	}
	object.Values[key] = value
}

func findRow(root wire.Value, id string) (wire.Value, error) {
	for _, row := range rows(root) {
		if row.Obj.Values["id"].Str == id {
			return row, nil
		}
	}
	return wire.Value{}, fmt.Errorf("no obligation %q in the produced map", id)
}

func rowIndex(root wire.Value, id string) (int, error) {
	for index, row := range rows(root) {
		if row.Obj.Values["id"].Str == id {
			return index, nil
		}
	}
	return 0, fmt.Errorf("no obligation %q in the produced map", id)
}

func swapRows(root wire.Value, a, b string) error {
	i, err := rowIndex(root, a)
	if err != nil {
		return err
	}
	j, err := rowIndex(root, b)
	if err != nil {
		return err
	}
	items := rows(root)
	items[i], items[j] = items[j], items[i]
	return nil
}

func dropRow(root wire.Value, id string) error {
	i, err := rowIndex(root, id)
	if err != nil {
		return err
	}
	items := rows(root)
	setRows(root, append(items[:i:i], items[i+1:]...))
	return nil
}

func appendRow(root wire.Value, fields map[string]any) error {
	row := wire.Value{Kind: wire.KindObject, Obj: &wire.Object{Values: map[string]wire.Value{}}}
	for key, value := range fields {
		switch typed := value.(type) {
		case string:
			setMember(row.Obj, key, stringValue(typed))
		case []any:
			items := make([]wire.Value, 0, len(typed))
			for _, item := range typed {
				text, ok := item.(string)
				if !ok {
					return fmt.Errorf("append %q: array items must be strings", key)
				}
				items = append(items, stringValue(text))
			}
			setMember(row.Obj, key, wire.Value{Kind: wire.KindArray, Arr: items})
		default:
			return fmt.Errorf("append %q: unsupported value", key)
		}
	}
	setRows(root, append(rows(root), row))
	return nil
}

func stringValue(text string) wire.Value {
	return wire.Value{Kind: wire.KindString, Str: text}
}
