package ticket

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Hold condition kinds (SCC-V0-010): a hold may carry one machine-checkable
// condition beside its free-text reason. The condition never releases the
// hold by itself; status and doctor re-evaluate it at each read.
const (
	HoldConditionCommitContained = "COMMIT_CONTAINED"
	HoldConditionPathExists      = "PATH_EXISTS"
	HoldConditionSymbolDefined   = "SYMBOL_DEFINED"
)

// HoldConditionKinds is the closed kind set.
var HoldConditionKinds = []string{HoldConditionCommitContained, HoldConditionPathExists, HoldConditionSymbolDefined}

// HoldConditionMaxSymbolBytes bounds a SYMBOL_DEFINED symbol.
const HoldConditionMaxSymbolBytes = 128

// HoldCondition is a hold's optional `condition` object: COMMIT_CONTAINED
// {kind, ref, commit}, PATH_EXISTS {kind, ref, path} or SYMBOL_DEFINED
// {kind, ref, path, symbol}. Ref is a full refname under refs/.
type HoldCondition struct {
	Kind   string
	Ref    string
	Commit string
	Path   string
	Symbol string
}

// ReadHoldCondition decodes a condition object with the key set its kind
// requires; any other key is MALFORMED.
func ReadHoldCondition(r *wire.Reader) *HoldCondition {
	c := &HoldCondition{}
	kind := ""
	if v := r.Value(); v.Kind == wire.KindObject && v.Obj != nil {
		if k, ok := v.Obj.Get("kind"); ok {
			kind = k.Str
		}
	}
	switch kind {
	case HoldConditionCommitContained:
		r.Closed("kind", "ref", "commit")
	case HoldConditionPathExists:
		r.Closed("kind", "ref", "path")
	case HoldConditionSymbolDefined:
		r.Closed("kind", "ref", "path", "symbol")
	}
	c.Kind = r.Field("kind").Enum(HoldConditionKinds...)
	c.Ref = holdConditionRef(r.Field("ref"))
	switch c.Kind {
	case HoldConditionCommitContained:
		c.Commit = r.Field("commit").OID()
	case HoldConditionPathExists:
		c.Path = r.Field("path").Path()
	case HoldConditionSymbolDefined:
		c.Path = r.Field("path").Path()
		c.Symbol = holdConditionSymbol(r.Field("symbol"))
		if strings.HasSuffix(c.Path, "/") {
			r.Field("path").Fail(wire.CodeMalformed, "SYMBOL_DEFINED path names a file, not a directory prefix")
		}
	}
	return c
}

// Value renders the condition with exactly its kind's keys.
func (c *HoldCondition) Value() wire.Value {
	o := wire.NewObject()
	o.Set("kind", wire.String(c.Kind))
	o.Set("ref", wire.String(c.Ref))
	switch c.Kind {
	case HoldConditionCommitContained:
		o.Set("commit", wire.String(c.Commit))
	case HoldConditionPathExists:
		o.Set("path", wire.String(c.Path))
	case HoldConditionSymbolDefined:
		o.Set("path", wire.String(c.Path))
		o.Set("symbol", wire.String(c.Symbol))
	}
	return wire.ObjectValue(o)
}

// holdConditionRef accepts a full refname under refs/ that Git's refname
// rules admit, so evaluation never resolves an abbreviation or a revision
// expression.
func holdConditionRef(r *wire.Reader) string {
	s := r.String()
	if r.Err() != nil {
		return ""
	}
	if !HoldConditionRefValid(s) {
		r.Fail(wire.CodeMalformed, "ref %q must be a full refname under refs/", s)
		return ""
	}
	return s
}

// HoldConditionRefValid reports whether s is a full refname under refs/ of
// at most 256 bytes that Git's refname rules admit.
func HoldConditionRefValid(s string) bool {
	bad := !strings.HasPrefix(s, "refs/") || len(s) > 256 || strings.HasSuffix(s, ".") ||
		strings.Contains(s, "..") || strings.Contains(s, "@{")
	// No component is empty, starts with '.' or ends with ".lock".
	for _, comp := range strings.Split(s, "/") {
		bad = bad || comp == "" || strings.HasPrefix(comp, ".") || strings.HasSuffix(comp, ".lock")
	}
	for i := 0; i < len(s) && !bad; i++ {
		c := s[i]
		bad = c <= ' ' || c >= 0x7F || strings.IndexByte("~^:?*[\\", c) >= 0
	}
	return !bad
}

// holdConditionSymbol accepts an identifier-shaped symbol: a letter or '_'
// followed by letters, digits or '_'.
func holdConditionSymbol(r *wire.Reader) string {
	s := r.String()
	if r.Err() != nil {
		return ""
	}
	ok := s != "" && len(s) <= HoldConditionMaxSymbolBytes
	for i := 0; i < len(s) && ok; i++ {
		c := s[i]
		ok = c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (i > 0 && c >= '0' && c <= '9')
	}
	if !ok {
		r.Fail(wire.CodeMalformed, "symbol %q must be an identifier of at most %d bytes", s, HoldConditionMaxSymbolBytes)
		return ""
	}
	return s
}
