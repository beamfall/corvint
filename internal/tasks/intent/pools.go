package intent

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"sort"
)

const MaxPoolMembers = 256

var StageRoles = []string{"implement", "review", "integrate"}

type ConfigRef struct{ Revision, Path, Blob string }
type PoolCommand struct {
	Argv, Env      []string
	Cwd            string
	TimeoutSeconds wire.Count
}
type MemberConfig struct {
	ConfigRef       *ConfigRef
	Health, Cleanup *PoolCommand
	SafeReuse       *SafeReuse
}
type Pool struct {
	ID           string
	Members      []string
	ReservedFor  map[string]string
	MemberConfig map[string]MemberConfig
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
func readPoolCommand(r *wire.Reader) *PoolCommand {
	r.Closed("argv", "cwd", "env", "timeoutSeconds")
	c := &PoolCommand{Cwd: r.Field("cwd").Enum("REPOSITORY"), Env: r.Field("env").Strings(64, false, (*wire.Reader).Label), TimeoutSeconds: boundCount(r.Field("timeoutSeconds"), 1, 300)}
	for _, arg := range r.Field("argv").Array(128, true) {
		c.Argv = append(c.Argv, arg.Prose(1, 4096))
	}
	if len(c.Argv) == 0 {
		r.Fail(wire.CodeMalformed, "empty pool command")
	}
	return c
}
func readPools(r *wire.Reader, env []string) []Pool {
	out := []Pool{}
	ids, members, refs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, x := range r.Array(64, false) {
		x.Closed(wire.OptionalKeys(x.Value(), []string{"id", "members"}, "reservedFor", "memberConfig")...)
		p := Pool{ID: x.Field("id").Label(), Members: x.Field("members").Strings(MaxPoolMembers, false, (*wire.Reader).Label), ReservedFor: map[string]string{}, MemberConfig: map[string]MemberConfig{}}
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
				cr.Closed(wire.OptionalKeys(cr.Value(), nil, "configRef", "health", "cleanup", "safeReuse")...)
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
				if wire.Has(cr.Value(), "safeReuse") {
					c.SafeReuse = readSafeReuse(cr.Field("safeReuse"), env)
				}
				p.MemberConfig[name] = c
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
