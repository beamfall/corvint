package intent

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"path"
	"slices"
	"sort"
	"strings"
)

const MaxPoolMembers = 256

// MaxPoolCommandSeconds bounds one memberConfig health, cleanup or prepare command
// (CAL-V0-033, PSR-V0-011, PSR-V0-024). Inside pool sweep the member's shared safeReuse
// deadline and the sweep's total budget still bound it.
const MaxPoolCommandSeconds = 3600

// CwdRepository and CwdPinned are the pool command working-directory kinds.
const (
	CwdRepository = "REPOSITORY"
	CwdPinned     = "PINNED_REPOSITORY"
)

var StageRoles = []string{"implement", "review", "integrate"}

type ConfigRef struct{ Revision, Path, Blob string }

// PinnedCwd names an external Git worktree (PSR-V0-012). A command declaring
// it runs there only while that worktree's HEAD is Revision and its inputs are
// clean; otherwise it refuses and never runs elsewhere.
type PinnedCwd struct{ Path, Revision string }
type PoolCommand struct {
	Argv, Env      []string
	Cwd            string
	Pinned         *PinnedCwd
	TimeoutSeconds wire.Count
}
type MemberConfig struct {
	ConfigRef       *ConfigRef
	Health, Cleanup *PoolCommand
	// Prepare is the optional sweep step that runs after a passed verify and
	// before delegated confirmation, while the member stays owned and
	// quarantined (PSR-V0-024..026). It requires SafeReuse.
	Prepare   *PoolCommand
	SafeReuse *SafeReuse
	// Identity is the read-only command that reports the member's current
	// source pair as {"revision","tree"} JSON (PSR-V0-082). It is declared
	// exactly when the pool declares allowedSources.
	Identity *PoolCommand
	// Operations holds the member's declared typed lifecycle commands, keyed
	// by a PoolOperations name (PSR-V0-160). An undeclared one is refused.
	Operations map[string]*PoolCommand
}

// PoolOperations is the closed set of typed member lifecycle operations
// (PSR-V0-160).
var PoolOperations = []string{"provision", "refresh", "reset", "retarget", "teardown"}

// MaxPoolSources bounds a pool's approved-source allow-list (PSR-V0-081).
const MaxPoolSources = 16

// PoolSource is one approved revision/tree pair of a pool member's source
// (PSR-V0-081).
type PoolSource struct{ Revision, Tree string }

func (s PoolSource) String() string { return s.Revision + ":" + s.Tree }

// ParsePoolSource reads a REVISION:TREE pair of full Git object ids.
func ParsePoolSource(where, s string) (PoolSource, error) {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return PoolSource{}, wire.Errorf(wire.CodeMalformed, where, "source must be REVISION:TREE")
	}
	rev, e := wire.ParseOID(where, s[:i])
	if e != nil {
		return PoolSource{}, e
	}
	tree, e := wire.ParseOID(where, s[i+1:])
	if e != nil {
		return PoolSource{}, e
	}
	return PoolSource{rev, tree}, nil
}

func readPoolSource(r *wire.Reader) PoolSource {
	r.Closed("revision", "tree")
	return PoolSource{r.Field("revision").OID(), r.Field("tree").OID()}
}

// Allows reports whether the pool's allow-list holds the pair.
func (p *Pool) Allows(s PoolSource) bool { return slices.Contains(p.AllowedSources, s) }

// MaxPoolWaitSeconds bounds a pool's maxWaitSeconds and
// handoffGraceSeconds (PSR-V0-130, PSR-V0-136).
const MaxPoolWaitSeconds = 600

type Pool struct {
	ID           string
	Members      []string
	ReservedFor  map[string]string
	MemberConfig map[string]MemberConfig
	// PriorityAdmission opts the pool into CAL-V0-101 priority-yield
	// admission. Omission, like an explicit false, keeps the old admission.
	PriorityAdmission bool
	// MaxWaitSeconds is the longest `pool wait` registration the pool
	// accepts (PSR-V0-130); zero means the pool keeps no waitlist.
	MaxWaitSeconds int64
	// HandoffGraceSeconds keeps capacity for a clean HANDOFF's ticket for
	// at most this long (PSR-V0-136); zero, the default, keeps none.
	HandoffGraceSeconds int64
	// AllowedSources and DefaultSource are the PSR-V0-081 approved-source
	// allow-list and the pair a member must report before it is FREE; both
	// are absent or both present.
	AllowedSources []PoolSource
	DefaultSource  *PoolSource
}

func ReadConfigRef(r *wire.Reader) *ConfigRef {
	r.Closed("revision", "path", "blob")
	return &ConfigRef{r.Field("revision").OID(), r.Field("path").Identifier(), r.Field("blob").OID()}
}
func ConfigRefValue(c *ConfigRef) wire.Value {
	if c == nil {
		return wire.Null()
	}
	return wire.ObjectValue(wire.NewObject().Set("revision", wire.String(c.Revision)).Set("path", wire.String(c.Path)).Set("blob", wire.String(c.Blob)))
}

// readPoolCwd accepts "REPOSITORY" or a closed pinned external worktree
// {kind:"PINNED_REPOSITORY",path,revision}: a clean absolute path and a full
// Git object id. Validation is host-independent; execution verifies the pin.
func readPoolCwd(r *wire.Reader) (string, *PinnedCwd) {
	if r.Value().Kind != wire.KindObject {
		return r.Enum(CwdRepository), nil
	}
	r.Closed("kind", "path", "revision")
	r.Field("kind").Exact(CwdPinned)
	p := &PinnedCwd{Path: r.Field("path").PathText(), Revision: r.Field("revision").OID()}
	if r.Err() == nil && (path.Clean(p.Path) != p.Path || p.Path == "/") {
		r.Fail(wire.CodeMalformed, "pinned cwd path must be a clean absolute directory")
	}
	return CwdPinned, p
}
func readPoolCommand(r *wire.Reader) *PoolCommand {
	r.Closed("argv", "cwd", "env", "timeoutSeconds")
	c := &PoolCommand{Env: r.Field("env").Strings(64, false, (*wire.Reader).Label), TimeoutSeconds: boundCount(r.Field("timeoutSeconds"), 1, MaxPoolCommandSeconds)}
	c.Cwd, c.Pinned = readPoolCwd(r.Field("cwd"))
	for _, arg := range r.Field("argv").Array(128, true) {
		c.Argv = append(c.Argv, arg.Prose(1, 4096))
	}
	if len(c.Argv) == 0 {
		r.Fail(wire.CodeMalformed, "empty pool command")
	}
	return c
}

// readPoolOperations reads a member's non-empty, closed operations object
// (PSR-V0-160); every command's environment is a subset of the policy's.
func readPoolOperations(cr *wire.Reader, env []string) map[string]*PoolCommand {
	or := cr.Field("operations")
	or.Closed(wire.OptionalKeys(or.Value(), nil, PoolOperations...)...)
	out := map[string]*PoolCommand{}
	for _, op := range PoolOperations {
		if wire.Has(or.Value(), op) {
			out[op] = readPoolCommand(or.Field(op))
			subsetOf(cr, out[op].Env, env, "pool operation environment")
		}
	}
	if len(out) == 0 {
		or.Fail(wire.CodeMalformed, "pool operations declare no operation")
	}
	return out
}

func readPools(r *wire.Reader, env []string) []Pool {
	out := []Pool{}
	ids, members, refs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, x := range r.Array(64, false) {
		x.Closed(wire.OptionalKeys(x.Value(), []string{"id", "members"}, "reservedFor", "memberConfig", "priorityAdmission", "allowedSources", "defaultSource", "maxWaitSeconds", "handoffGraceSeconds")...)
		p := Pool{ID: x.Field("id").Label(), Members: x.Field("members").Strings(MaxPoolMembers, false, (*wire.Reader).Label), ReservedFor: map[string]string{}, MemberConfig: map[string]MemberConfig{}}
		if wire.Has(x.Value(), "priorityAdmission") {
			p.PriorityAdmission = x.Field("priorityAdmission").Bool()
		}
		// PSR-V0-130, PSR-V0-136: both waitlist bounds are optional and
		// at most MaxPoolWaitSeconds.
		if wire.Has(x.Value(), "maxWaitSeconds") {
			p.MaxWaitSeconds = boundCount(x.Field("maxWaitSeconds"), 1, MaxPoolWaitSeconds).Int()
		}
		if wire.Has(x.Value(), "handoffGraceSeconds") {
			p.HandoffGraceSeconds = boundCount(x.Field("handoffGraceSeconds"), 0, MaxPoolWaitSeconds).Int()
		}
		readPoolSources(x, &p)
		if ids[p.ID] || len(p.Members) == 0 {
			x.Fail(wire.CodeMalformed, "duplicate or empty pool")
		}
		ids[p.ID] = true
		local := map[string]bool{}
		for _, m := range p.Members {
			if members[m] {
				x.Fail(wire.CodeDuplicateID, "pool member is not queue-unique")
			}
			local[m] = true
			members[m] = true
		}
		sort.Strings(p.Members)
		for _, field := range []string{"reservedFor", "memberConfig"} {
			if !wire.Has(x.Value(), field) {
				continue
			}
			mr := x.Field(field)
			if mr.Value().Kind != wire.KindObject {
				mr.Fail(wire.CodeMalformed, "pool member map required")
				continue
			}
			for _, name := range mr.Value().Obj.Keys {
				if !local[name] {
					mr.Fail(wire.CodeMalformed, "unknown pool member")
				}
				cr := mr.Field(name)
				if field == "reservedFor" {
					p.ReservedFor[name] = cr.Enum(StageRoles...)
					continue
				}
				cr.Closed(wire.OptionalKeys(cr.Value(), nil, "configRef", "health", "cleanup", "prepare", "safeReuse", "identity", "operations")...)
				c := MemberConfig{}
				if wire.Has(cr.Value(), "configRef") {
					c.ConfigRef = ReadConfigRef(cr.Field("configRef"))
					key := c.ConfigRef.Revision + ":" + c.ConfigRef.Path
					if refs[key] {
						cr.Fail(wire.CodeDuplicateID, "aliased member configuration")
					}
					refs[key] = true
				}
				if wire.Has(cr.Value(), "health") {
					c.Health = readPoolCommand(cr.Field("health"))
					subsetOf(cr, c.Health.Env, env, "pool health environment")
				}
				if wire.Has(cr.Value(), "cleanup") {
					c.Cleanup = readPoolCommand(cr.Field("cleanup"))
					subsetOf(cr, c.Cleanup.Env, env, "pool cleanup environment")
				}
				if wire.Has(cr.Value(), "prepare") {
					c.Prepare = readPoolCommand(cr.Field("prepare"))
					subsetOf(cr, c.Prepare.Env, env, "pool prepare environment")
				}
				if wire.Has(cr.Value(), "safeReuse") {
					c.SafeReuse = readSafeReuse(cr.Field("safeReuse"), env)
				}
				// PSR-V0-024: only pool sweep runs prepare, so it needs safeReuse.
				if c.Prepare != nil && c.SafeReuse == nil {
					cr.Fail(wire.CodeMalformed, "pool prepare requires safeReuse")
				}
				if wire.Has(cr.Value(), "identity") {
					c.Identity = readPoolCommand(cr.Field("identity"))
					subsetOf(cr, c.Identity.Env, env, "pool identity environment")
					// PSR-V0-082: pool sweep's delegated confirmation runs no
					// identity probe in this slice, so the two never combine.
					if c.SafeReuse != nil {
						cr.Fail(wire.CodeMalformed, "pool identity does not combine with safeReuse")
					}
				}
				if wire.Has(cr.Value(), "operations") {
					c.Operations = readPoolOperations(cr, env)
				}
				p.MemberConfig[name] = c
			}
		}
		// PSR-V0-082: every member of a pool with allowedSources declares an
		// identity command, and only such a pool's members declare one.
		for _, m := range p.Members {
			if (p.MemberConfig[m].Identity != nil) != (p.DefaultSource != nil) {
				x.Fail(wire.CodeMalformed, "pool identity is declared for every member exactly when allowedSources is")
				break
			}
		}
		out = append(out, p)
	}
	if len(members) > MaxPoolMembers {
		r.Fail(wire.CodeLimitExceeded, "pool member bound")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// readPoolSources reads the PSR-V0-081 allowedSources and defaultSource:
// both or neither, a bounded duplicate-free list, and a default in it.
func readPoolSources(x *wire.Reader, p *Pool) {
	has, hasDefault := wire.Has(x.Value(), "allowedSources"), wire.Has(x.Value(), "defaultSource")
	if has != hasDefault {
		x.Fail(wire.CodeMalformed, "allowedSources and defaultSource are declared together")
		return
	}
	if !has {
		return
	}
	for _, sr := range x.Field("allowedSources").Array(MaxPoolSources, true) {
		s := readPoolSource(sr)
		if slices.Contains(p.AllowedSources, s) {
			sr.Fail(wire.CodeDuplicateID, "duplicate allowed source")
		}
		p.AllowedSources = append(p.AllowedSources, s)
	}
	if len(p.AllowedSources) == 0 {
		x.Fail(wire.CodeMalformed, "allowedSources must be non-empty")
	}
	def := readPoolSource(x.Field("defaultSource"))
	if x.Err() == nil && !slices.Contains(p.AllowedSources, def) {
		x.Fail(wire.CodeMalformed, "defaultSource must be one of allowedSources")
	}
	p.DefaultSource = &def
}

func (p *Policy) Pool(id string) *Pool {
	for i := range p.Pools {
		if p.Pools[i].ID == id {
			return &p.Pools[i]
		}
	}
	return nil
}
func (p *Policy) MemberDefinition(id, member string) wire.Digest {
	poolDef := p.Pool(id)
	if poolDef == nil {
		return ""
	}
	exists := false
	for _, m := range poolDef.Members {
		exists = exists || m == member
	}
	if !exists {
		return ""
	}
	v, e := wire.Parse(p.Raw)
	if e != nil {
		return ""
	}
	list, ok := v.Obj.Get("pools")
	if !ok {
		return ""
	}
	for _, pool := range list.Arr {
		x, _ := pool.Obj.Get("id")
		if x.Str != id {
			continue
		}
		o := wire.NewObject().Set("pool", wire.String(id)).Set("member", wire.String(member))
		for _, field := range []string{"reservedFor", "memberConfig"} {
			value, _ := pool.Obj.Get(field)
			entry := wire.Null()
			if value.Kind == wire.KindObject {
				if got, ok := value.Obj.Get(member); ok {
					entry = got
				}
			}
			o.Set(field, entry)
		}
		return wire.Sum(wire.EncodeFile(wire.ObjectValue(o)))
	}
	return ""
}
