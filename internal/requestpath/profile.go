package requestpath

import (
	"bufio"
	"fmt"
	"sort"
	"strings"
)

// Pre-run verdicts, modes and batch decisions (AMAP-V0-135, AMAP-V0-136).
const (
	VerdictReady      = "READY"
	VerdictBlocked    = "BLOCKED"
	VerdictIncomplete = "INCOMPLETE"

	ModeFailClosed = "FAIL_CLOSED"
	ModeWarn       = "WARN"

	BatchSchedule             = "SCHEDULE"
	BatchDoNotSchedule        = "DO_NOT_SCHEDULE"
	BatchScheduleWithWarnings = "SCHEDULE_WITH_WARNINGS"
)

// Profile is a target environment profile reduced to whether each key holds a non-empty value. It
// never keeps a value (AMAP-V0-135).
type Profile struct {
	Name     string
	nonEmpty map[string]bool
}

// MaxProfileBytes bounds a profile file.
const MaxProfileBytes = 1 << 20

// ParseProfile reads a dotenv-format profile: `KEY=VALUE` lines, an optional `export ` prefix,
// quoted values, blank lines and `#` comments. A malformed line refuses naming its line number
// only, never its content.
func ParseProfile(name, text string) (*Profile, error) {
	if len(text) > MaxProfileBytes {
		return nil, refuse("request-path-profile-invalid", "the profile exceeds %d bytes", MaxProfileBytes)
	}
	p := &Profile{Name: name, nonEmpty: map[string]bool{}}
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 64*1024), MaxProfileBytes)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !validKey(key) {
			return nil, refuse("request-path-profile-invalid", "profile line %d is not KEY=VALUE", n)
		}
		p.nonEmpty[key] = unquote(strings.TrimSpace(value)) != ""
	}
	if sc.Err() != nil {
		return nil, refuse("request-path-profile-invalid", "the profile is unreadable after line %d", n)
	}
	return p, nil
}

func validKey(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && (r >= '0' && r <= '9' || r == '.' || r == '-')) {
			return false
		}
	}
	return true
}

func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') {
		if i := strings.IndexByte(v[1:], v[0]); i >= 0 {
			return v[1 : i+1] // `KEY="" # note` is empty
		}
	}
	if strings.HasPrefix(v, "#") {
		return ""
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

// PreRun is the pre-run check of a request path against a profile. It names keys only.
type PreRun struct {
	Profile               string         `json:"profile"`
	Mode                  string         `json:"mode"`
	Verdict               string         `json:"verdict"`
	Batch                 string         `json:"batch"`
	Unset                 []string       `json:"unset"`
	Empty                 []string       `json:"empty"`
	DefaultedUnsetOrEmpty []string       `json:"defaultedUnsetOrEmpty"`
	ConfigKeysNotChecked  []string       `json:"configKeysNotChecked"`
	Unresolved            map[string]int `json:"unresolved"`
	Message               string         `json:"message"`
}

// Check decides whether a batch on this request path may be scheduled against p (AMAP-V0-135,
// AMAP-V0-136). A required key unset or empty blocks; an unresolved key read, call, middleware,
// mount, source or bound leaves the check INCOMPLETE, because a key could be read there. In
// FAIL_CLOSED mode neither is scheduled; WARN schedules with the same findings.
func Check(r *Report, p *Profile, warn bool) *PreRun {
	out := &PreRun{Profile: p.Name, Mode: ModeFailClosed, Unset: []string{}, Empty: []string{}, DefaultedUnsetOrEmpty: []string{},
		ConfigKeysNotChecked: []string{}, Unresolved: map[string]int{}}
	if warn {
		out.Mode = ModeWarn
	}
	for _, k := range r.EnvKeys {
		nonEmpty, set := p.nonEmpty[k.Name]
		switch {
		case set && nonEmpty:
		case k.Defaulted:
			out.DefaultedUnsetOrEmpty = append(out.DefaultedUnsetOrEmpty, k.Name)
		case !set:
			out.Unset = append(out.Unset, k.Name)
		default:
			out.Empty = append(out.Empty, k.Name)
		}
	}
	for _, k := range r.ConfigKeys {
		out.ConfigKeysNotChecked = append(out.ConfigKeysNotChecked, k.Name)
	}
	blocking := 0
	for _, u := range r.Unknowns {
		out.Unresolved[u.Kind]++
		if u.Kind != UnknownConfigKey {
			blocking++
		}
	}
	switch {
	case len(out.Unset)+len(out.Empty) > 0:
		out.Verdict = VerdictBlocked
		out.Message = fmt.Sprintf("required environment key(s) unset or empty in profile %s: %s", p.Name, strings.Join(append(append([]string{}, out.Unset...), out.Empty...), ", "))
	case blocking > 0:
		out.Verdict = VerdictIncomplete
		kinds := make([]string, 0, len(out.Unresolved))
		for k := range out.Unresolved {
			if k != UnknownConfigKey {
				kinds = append(kinds, k)
			}
		}
		sort.Strings(kinds)
		out.Message = "the request path has unresolved " + strings.Join(kinds, ", ") + " entries; a key read there is not checked"
	default:
		out.Verdict = VerdictReady
		out.Message = "every environment key the request path reads is set and non-empty in profile " + p.Name
	}
	switch {
	case out.Verdict == VerdictReady:
		out.Batch = BatchSchedule
	case warn:
		out.Batch = BatchScheduleWithWarnings
	default:
		out.Batch = BatchDoNotSchedule
	}
	return out
}
