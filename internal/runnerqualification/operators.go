// SPDX-License-Identifier: Apache-2.0

package runnerqualification

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/runnerselection"
)

// FileTarget is a corpus-declared opportunity. Existing files have non-nil
// Bytes, including empty files. Destinations must be proven absent at the pin.
// The caller obtains these facts from core records and the immutable tree.
type FileTarget struct {
	Path, Destination string
	Bytes             []byte
	Class             runnerselection.Class
	Kind              string // data, listed-add, lookup-add, delete, rename, exec, configuration
	DestinationAbsent bool
}

const MaxMutationBytes = 32 << 20

func validPath(p string) bool {
	return validText(p) && p != "." && p != ".." && path.Clean(p) == p && !path.IsAbs(p) && !strings.HasPrefix(p, "../") && !strings.ContainsAny(p, "\\:")
}

// FilePopulation enumerates every operator opportunity in its supplied corpus
// targets before sampling. It refuses the entire population at the cap; a
// prefix of a population cannot justify a statistical claim about the whole.
func FilePopulation(targets []FileTarget) Population {
	out := Population{Complete: true}
	fail := func(reason string) Population { return Population{Reason: reason} }
	if len(targets) == 0 || len(targets) > MaxPopulation {
		return fail("file-targets-empty-or-capped")
	}
	add := func(t FileTarget, operator string, offset int, edits ...Edit) {
		out.Mutants = append(out.Mutants, Mutant{ID: fmt.Sprintf("%s:%s:%d", t.Path, operator, offset), Piece: "core-file", Operator: operator, Class: t.Class, Edits: edits})
	}
	seen := map[string]bool{}
	retainedBytes := 0
	for _, t := range targets {
		if !validPath(t.Path) || seen[t.Path+":"+t.Kind] {
			return fail("file-target-invalid-or-duplicate")
		}
		seen[t.Path+":"+t.Kind] = true
		remaining := MaxPopulation - len(out.Mutants)
		if remaining == 0 {
			return fail("population-capped")
		}
		// Bound rendered bytes before cloning. Enumerating O(n) edits of an
		// n-byte file otherwise allocates O(n²) bytes before noticing a cap.
		if len(t.Bytes) > MaxMutationBytes {
			return fail("population-bytes-capped")
		}
		count := 1
		if t.Kind == "data" {
			count = 2 * len(t.Bytes)
		} else if t.Kind == "exec" || t.Kind == "configuration" {
			count = len(t.Bytes)
		}
		if count > 0 && len(t.Bytes) > (MaxMutationBytes-retainedBytes)/(2*count) {
			return fail("population-bytes-capped")
		}
		retainedBytes += 2 * count * len(t.Bytes)
		switch t.Kind {
		case "data":
			if t.Class != runnerselection.DataEdit || t.Bytes == nil {
				return fail("data-target-invalid")
			}
			if len(t.Bytes) > remaining/2 {
				return fail("population-capped")
			}
			for i := range t.Bytes {
				b := bytes.Clone(t.Bytes)
				b[i] ^= 1
				add(t, "byte", i, Edit{t.Path, bytes.Clone(t.Bytes), b})
				add(t, "truncate", i, Edit{t.Path, bytes.Clone(t.Bytes), append([]byte{}, t.Bytes[:i]...)})
			}
		case "listed-add", "lookup-add":
			if t.Class != runnerselection.Add || t.Bytes != nil || !t.DestinationAbsent {
				return fail("add-target-not-absent")
			}
			add(t, t.Kind, 0, Edit{t.Path, nil, []byte("qualification mutant\n")})
		case "delete", "rename":
			if t.Class != runnerselection.DeleteRename || t.Bytes == nil {
				return fail("delete-target-invalid")
			}
			edits := []Edit{{t.Path, bytes.Clone(t.Bytes), nil}}
			if t.Kind == "rename" {
				if !validPath(t.Destination) || t.Destination == t.Path || !t.DestinationAbsent {
					return fail("rename-destination-not-absent")
				}
				edits = append(edits, Edit{t.Destination, nil, bytes.Clone(t.Bytes)})
			}
			add(t, t.Kind, 0, edits...)
		case "exec", "configuration":
			want := runnerselection.ExecTarget
			if t.Kind == "configuration" {
				want = runnerselection.RunnerConfig
			}
			if t.Class != want || t.Bytes == nil {
				return fail("file-target-class-invalid")
			}
			if len(t.Bytes) > remaining {
				return fail("population-capped")
			}
			for i := range t.Bytes {
				b := bytes.Clone(t.Bytes)
				b[i] ^= 1
				add(t, t.Kind, i, Edit{t.Path, bytes.Clone(t.Bytes), b})
			}
		default:
			return fail("file-operator-unsupported")
		}
	}
	if len(out.Mutants) == 0 {
		return fail("population-empty")
	}
	sort.Slice(out.Mutants, func(i, j int) bool { return out.Mutants[i].ID < out.Mutants[j].ID })
	return out
}

// ValidatePopulation ties plugin operators to the T3 language manifest mutation
// piece. Core operators cannot be relabelled as body/declaration/server mutants.
func ValidatePopulation(d Declaration, class runnerselection.Class, p Population) error {
	if !p.Complete || p.Reason != "" {
		return fmt.Errorf("population-incomplete:%s", p.Reason)
	}
	if len(p.Mutants) == 0 || len(p.Mutants) > MaxPopulation {
		return fmt.Errorf("population-empty-or-capped")
	}
	pieces := map[string]bool{}
	for _, piece := range d.Mutation {
		pieces[piece.Name] = true
	}
	seen, operators := map[string]bool{}, map[string]bool{}
	retainedBytes := 0
	for _, m := range p.Mutants {
		if !validText(m.ID) || seen[m.ID] || m.Class != class || !validText(m.Operator) || len(m.Edits) == 0 || len(m.Edits) > 2 {
			return fmt.Errorf("mutant-invalid")
		}
		seen[m.ID], operators[m.Operator] = true, true
		switch class {
		case runnerselection.DataEdit, runnerselection.Add, runnerselection.DeleteRename, runnerselection.ExecTarget, runnerselection.RunnerConfig:
			if m.Piece != "core-file" || !coreEditShape(m) {
				return fmt.Errorf("core-operator-class-invalid")
			}
		case runnerselection.BodyEdit, runnerselection.DeclarationEdit, runnerselection.E2EServer:
			if !pieces[m.Piece] || m.Piece == "core-file" {
				return fmt.Errorf("mutation-piece-undeclared:%s", m.Piece)
			}
		default:
			return fmt.Errorf("core-operator-class-invalid")
		}
		paths := map[string]bool{}
		for _, e := range m.Edits {
			if len(e.Before) > MaxMutationBytes-retainedBytes {
				return fmt.Errorf("population-bytes-capped")
			}
			retainedBytes += len(e.Before)
			if len(e.After) > MaxMutationBytes-retainedBytes {
				return fmt.Errorf("population-bytes-capped")
			}
			retainedBytes += len(e.After)
			if !validPath(e.Path) || paths[e.Path] || (e.Before == nil && e.After == nil) || (e.Before != nil && e.After != nil && bytes.Equal(e.Before, e.After)) {
				return fmt.Errorf("mutant-edit-invalid")
			}
			paths[e.Path] = true
		}
	}
	// These classes explicitly require both operators, not a favourable subset.
	for _, op := range map[runnerselection.Class][]string{runnerselection.DataEdit: {"byte", "truncate"}, runnerselection.Add: {"listed-add", "lookup-add"}, runnerselection.DeleteRename: {"delete", "rename"}}[class] {
		if !operators[op] {
			return fmt.Errorf("required-operator-missing:%s", op)
		}
	}
	return nil
}

// coreEditShape admits only the forms generated by FilePopulation (RSP-V0-033/034).
// Names and manifest membership alone are not proof of a file operation.
func coreEditShape(m Mutant) bool {
	if len(m.Edits) == 0 {
		return false
	}
	e := m.Edits[0]
	switch m.Class {
	case runnerselection.Add:
		return (m.Operator == "listed-add" || m.Operator == "lookup-add") && len(m.Edits) == 1 && e.Before == nil && e.After != nil
	case runnerselection.DeleteRename:
		if e.Before == nil || e.After != nil {
			return false
		}
		switch m.Operator {
		case "delete":
			return len(m.Edits) == 1
		case "rename":
			return len(m.Edits) == 2 && m.Edits[1].Before == nil && m.Edits[1].After != nil && bytes.Equal(e.Before, m.Edits[1].After)
		}
	case runnerselection.DataEdit, runnerselection.ExecTarget, runnerselection.RunnerConfig:
		if len(m.Edits) != 1 || e.Before == nil || e.After == nil {
			return false
		}
		if m.Class == runnerselection.DataEdit && m.Operator == "truncate" {
			return len(e.After) < len(e.Before) && bytes.Equal(e.After, e.Before[:len(e.After)])
		}
		want := map[runnerselection.Class]string{runnerselection.DataEdit: "byte", runnerselection.ExecTarget: "exec", runnerselection.RunnerConfig: "configuration"}[m.Class]
		if m.Operator != want || len(e.Before) != len(e.After) {
			return false
		}
		changed := 0
		for i, b := range e.Before {
			if b != e.After[i] {
				if b^e.After[i] != 1 {
					return false
				}
				changed++
			}
		}
		return changed == 1
	}
	return false
}
