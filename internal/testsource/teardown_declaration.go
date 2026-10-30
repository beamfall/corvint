package testsource

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
)

// The side-effect declaration of `corvint tests teardown` (TSA-V0-111): a closed, versioned,
// revision-pinned, project-owned list of resource kinds, the calls that create and delete them,
// and the actions that produce dependents.

const (
	// SideEffectSchema names the declaration document.
	SideEffectSchema   = "corvint-side-effect-declaration/0"
	maxDeclaredKinds   = 256
	maxDeclaredActions = 1024
	maxDeclaredNames   = 64
)

var (
	declaredKind     = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	declaredCall     = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]{0,127}[?!]?$`)
	declaredRevision = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
)

// SideEffectDeclaration is the corvint-side-effect-declaration/0 document.
type SideEffectDeclaration struct {
	Schema    string             `json:"schema"`
	Version   int                `json:"version"`
	Revision  string             `json:"revision"`
	Resources []DeclaredResource `json:"resources"`
	Actions   []DeclaredAction   `json:"actions"`
}

// DeclaredResource is one resource kind and the calls that create and delete it.
type DeclaredResource struct {
	Kind   string   `json:"kind"`
	Create []string `json:"create"`
	Delete []string `json:"delete"`
}

// DeclaredAction is one call on a parent resource: it produces a dependent of the named kind, which
// the parent's delete removes when Cascade is set. An action without a parent is inert.
type DeclaredAction struct {
	Call      string `json:"call"`
	Parent    string `json:"parent,omitempty"`
	Dependent string `json:"dependent,omitempty"`
	Cascade   bool   `json:"cascade,omitempty"`
}

// sideEffects is the validated declaration indexed by call name.
type sideEffects struct {
	creates map[string]string // create name -> kind
	deletes map[string]string // delete name -> kind
	actions map[string]DeclaredAction
}

func (e sideEffects) declared(call string) bool {
	_, create := e.creates[call]
	_, remove := e.deletes[call]
	_, action := e.actions[call]
	return create || remove || action
}

// decodeSideEffects decodes and validates a declaration; every defect refuses
// tests-declaration-invalid.
func decodeSideEffects(name string, data []byte) (SideEffectDeclaration, sideEffects, error) {
	invalid := func(why string) (SideEffectDeclaration, sideEffects, error) {
		return SideEffectDeclaration{}, sideEffects{}, refuse("tests-declaration-invalid", "%s: %s", name, why)
	}
	var d SideEffectDeclaration
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&d); err != nil {
		return invalid("not a closed " + SideEffectSchema + " document")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return invalid("trailing data after the document")
	}
	switch {
	case d.Schema != SideEffectSchema:
		return invalid("schema must be " + SideEffectSchema)
	case d.Version < 1:
		return invalid("version must be a positive integer")
	case !declaredRevision.MatchString(d.Revision):
		return invalid("revision must be a 40- or 64-hex commit")
	case len(d.Resources) == 0 || len(d.Resources) > maxDeclaredKinds:
		return invalid("resources must hold 1 to 256 kinds")
	case len(d.Actions) > maxDeclaredActions:
		return invalid("actions must hold at most 1024 entries")
	}
	e := sideEffects{creates: map[string]string{}, deletes: map[string]string{}, actions: map[string]DeclaredAction{}}
	kinds, names := map[string]bool{}, map[string]bool{}
	claim := func(call string) bool {
		if !declaredCall.MatchString(call) || names[call] {
			return false
		}
		names[call] = true
		return true
	}
	for _, r := range d.Resources {
		if !declaredKind.MatchString(r.Kind) || kinds[r.Kind] {
			return invalid("resource kinds must be unique names matching [a-z][a-z0-9_-]{0,63}")
		}
		kinds[r.Kind] = true
		if len(r.Create) == 0 || len(r.Delete) == 0 || len(r.Create) > maxDeclaredNames || len(r.Delete) > maxDeclaredNames {
			return invalid("resource " + r.Kind + " must have 1 to 64 create and delete names")
		}
		for _, call := range r.Create {
			if !claim(call) {
				return invalid("call names must be unique identifiers")
			}
			e.creates[call] = r.Kind
		}
		for _, call := range r.Delete {
			if !claim(call) {
				return invalid("call names must be unique identifiers")
			}
			e.deletes[call] = r.Kind
		}
	}
	for _, a := range d.Actions {
		switch {
		case !claim(a.Call):
			return invalid("call names must be unique identifiers")
		case a.Parent != "" && !kinds[a.Parent]:
			return invalid("action " + a.Call + " names an undeclared parent kind")
		case a.Dependent != "" && (a.Parent == "" || !kinds[a.Dependent]):
			return invalid("action " + a.Call + " needs a parent and a declared dependent kind")
		case a.Cascade && a.Dependent == "":
			return invalid("action " + a.Call + " declares cascade without a dependent")
		}
		e.actions[a.Call] = a
	}
	return d, e, nil
}
