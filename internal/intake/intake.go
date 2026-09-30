// Package intake admits closed-vocabulary reader claims before an author sees them.
// It does not interpret raw prose or grant authority to a reader claim.
package intake

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

const (
	Profile  = "corvint-intake/0"
	MaxBytes = 1 << 20
	MaxItems = 1024
)

// Errors crossing the author boundary carry fixed codes, never rejected values.
var (
	ErrSchema     = errors.New("INTAKE_SCHEMA")
	ErrRepository = errors.New("INTAKE_REPOSITORY")
	ErrPath       = errors.New("INTAKE_PATH")
	ErrBoundary   = errors.New("INTAKE_READER_BOUNDARY")
)

type Behaviour struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}
type Criteria struct {
	Met     int64 `json:"met"`
	Unmet   int64 `json:"unmet"`
	Unknown int64 `json:"unknown"`
}
type Parent struct {
	ID         string   `json:"id"`
	Alignment  string   `json:"alignment"`
	Acceptance Criteria `json:"acceptance_criteria"`
	Concerns   []string `json:"concerns"`
}
type WorkItem struct {
	ID         string   `json:"id"`
	Alignment  string   `json:"alignment"`
	Acceptance Criteria `json:"acceptance_criteria"`
	Concerns   []string `json:"concerns"`
	Parent     *Parent  `json:"parent_review"`
}
type Record struct {
	Profile    string      `json:"profile"`
	Base       string      `json:"base"`
	Head       string      `json:"head"`
	Intent     string      `json:"intent"`
	Behaviours []Behaviour `json:"claimed_behaviours"`
	Flags      []string    `json:"flags"`
	Migrations int64       `json:"migrations_claimed"`
	Tests      []string    `json:"tests_claimed"`
	Comparison string      `json:"description_vs_diff"`
	Concerns   []string    `json:"concerns"`
	WorkItems  []WorkItem  `json:"work_items"`
}

var identifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
var concerns = []string{"INSTRUCTION_ATTEMPT", "OUTWARD_ACTION", "SECRET_REFERENCE", "MISSING_EVIDENCE", "CONFLICTED_CLAIM"}
var alignments = []string{"ALIGNED", "PARTIAL", "CONFLICTED", "UNKNOWN"}

func object(v wire.Value, keys ...string) bool {
	if v.Kind != wire.KindObject || len(v.Obj.Keys) != len(keys) {
		return false
	}
	for _, k := range keys {
		if _, ok := v.Obj.Values[k]; !ok {
			return false
		}
	}
	return true
}
func member(v wire.Value, key string) wire.Value { return v.Obj.Values[key] }
func enum(v wire.Value, choices ...string) bool {
	return v.Kind == wire.KindString && slices.Contains(choices, v.Str)
}
func count(v wire.Value) bool { return v.Kind == wire.KindInt && v.Int >= 0 && v.Int <= 1000000 }
func id(v wire.Value) bool    { return v.Kind == wire.KindString && identifier.MatchString(v.Str) }
func literalPath(v wire.Value) bool {
	if v.Kind != wire.KindString || len(v.Str) > 4096 || wire.ValidatePath(v.Str) != nil || strings.ContainsAny(v.Str, "*?[]\\:") {
		return false
	}
	for _, part := range strings.Split(v.Str, "/") {
		if strings.EqualFold(part, ".git") {
			return false
		}
	}
	for _, r := range v.Str {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
func set(v wire.Value, bound int, valid func(wire.Value) bool) bool {
	if v.Kind != wire.KindArray || len(v.Arr) > bound {
		return false
	}
	seen := map[string]bool{}
	for _, e := range v.Arr {
		key := string(wire.CanonicalValue(e))
		if !valid(e) || seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}
func concernSet(v wire.Value) bool {
	return set(v, len(concerns), func(e wire.Value) bool { return enum(e, concerns...) })
}
func criteria(v wire.Value) bool {
	return object(v, "met", "unmet", "unknown") && count(member(v, "met")) && count(member(v, "unmet")) && count(member(v, "unknown"))
}
func parent(v wire.Value) bool {
	return object(v, "id", "alignment", "acceptance_criteria", "concerns") && id(member(v, "id")) && enum(member(v, "alignment"), alignments...) && criteria(member(v, "acceptance_criteria")) && concernSet(member(v, "concerns"))
}
func workItem(v wire.Value) bool {
	if !object(v, "id", "alignment", "acceptance_criteria", "concerns", "parent_review") {
		return false
	}
	p := member(v, "parent_review")
	return id(member(v, "id")) && enum(member(v, "alignment"), alignments...) && criteria(member(v, "acceptance_criteria")) && concernSet(member(v, "concerns")) && (p.Kind == wire.KindNull || parent(p))
}

// Decode admits the exact field grammar. Missing fields and explicit null arrays fail.
func Decode(raw []byte) (Record, error) {
	var result Record
	if len(raw) > MaxBytes {
		return result, ErrSchema
	}
	v, err := wire.Parse(raw)
	if err != nil || !object(v, "profile", "base", "head", "intent", "claimed_behaviours", "flags", "migrations_claimed", "tests_claimed", "description_vs_diff", "concerns", "work_items") {
		return result, ErrSchema
	}
	for _, k := range []string{"base", "head"} {
		e := member(v, k)
		if e.Kind != wire.KindString || !wire.IsGitOid(e.Str) {
			return result, ErrSchema
		}
	}
	if !enum(member(v, "profile"), Profile) || !enum(member(v, "intent"), "FIX", "FEATURE", "REFACTOR", "DOCS", "TEST", "MAINTENANCE", "UNKNOWN") || !enum(member(v, "description_vs_diff"), alignments...) || !count(member(v, "migrations_claimed")) || !concernSet(member(v, "concerns")) {
		return result, ErrSchema
	}
	if !set(member(v, "flags"), MaxItems, id) || !set(member(v, "tests_claimed"), MaxItems, literalPath) || !set(member(v, "claimed_behaviours"), MaxItems, func(e wire.Value) bool {
		return object(e, "kind", "path") && enum(member(e, "kind"), "ADD", "CHANGE", "REMOVE", "UNKNOWN") && literalPath(member(e, "path"))
	}) || !set(member(v, "work_items"), 256, workItem) {
		return result, ErrSchema
	}
	ids := map[string]bool{}
	for _, item := range member(v, "work_items").Arr {
		key := member(item, "id").Str
		if ids[key] {
			return result, ErrSchema
		}
		ids[key] = true
	}
	if json.Unmarshal(raw, &result) != nil {
		return Record{}, ErrSchema
	}
	return result, nil
}

// BuildAuthorInput is the sole production author-input builder. Repository identity
// is supplied by the trusted host, not taken from the reader's claimed pins.
func BuildAuthorInput(ctx context.Context, root, base, head string, candidate []byte) ([]byte, error) {
	r, err := Decode(candidate)
	if err != nil {
		return nil, err
	}
	if !wire.IsGitOid(base) || !wire.IsGitOid(head) || r.Base != base || r.Head != head {
		return nil, ErrRepository
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	repo, err := gitauth.Open(root, gitrun.NewBudget(1024, 30*time.Second))
	if err != nil {
		return nil, ErrRepository
	}
	for _, revision := range []string{base, head} {
		resolved, e := repo.Resolve(ctx, revision)
		if e != nil || resolved != revision {
			return nil, ErrRepository
		}
		if _, e := repo.CommitTree(ctx, revision); e != nil {
			return nil, ErrRepository
		}
	}
	// Memoize repeated path claims without weakening either pin's object checks.
	checked := map[string]bool{}
	exists := func(path string, test, baseOnly bool) error {
		key := path
		if test {
			key = "test:" + path
		}
		if baseOnly {
			key = "base:" + key
		}
		if checked[key] {
			return nil
		}
		found := false
		revisions := []string{base, head}
		if baseOnly {
			revisions = []string{base}
		}
		for _, revision := range revisions {
			e, present, lookupErr := repo.LookupTreeEntry(ctx, revision, path)
			if lookupErr != nil {
				return ErrRepository
			}
			if present && ((e.Type == "blob" && (e.Mode == "100644" || e.Mode == "100755")) || (!test && ((e.Type == "blob" && e.Mode == "120000") || e.Type == "tree"))) {
				found = true
			}
		}
		if !found {
			return ErrPath
		}
		checked[key] = true
		return nil
	}
	for _, b := range r.Behaviours {
		if err := exists(b.Path, false, b.Kind == "REMOVE"); err != nil {
			return nil, err
		}
	}
	for _, p := range r.Tests {
		if err := exists(p, true, false); err != nil {
			return nil, err
		}
	}
	slices.SortFunc(r.Behaviours, func(a, b Behaviour) int {
		if c := strings.Compare(a.Kind, b.Kind); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	slices.Sort(r.Flags)
	slices.Sort(r.Tests)
	slices.Sort(r.Concerns)
	slices.SortFunc(r.WorkItems, func(a, b WorkItem) int { return strings.Compare(a.ID, b.ID) })
	for i := range r.WorkItems {
		slices.Sort(r.WorkItems[i].Concerns)
		if r.WorkItems[i].Parent != nil {
			slices.Sort(r.WorkItems[i].Parent.Concerns)
		}
	}
	raw, _ := json.Marshal(r)
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, ErrSchema
	}
	return append(wire.CanonicalValue(v), '\n'), nil
}
