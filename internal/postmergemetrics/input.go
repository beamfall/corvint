package postmergemetrics

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var oid = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)

func ParsePolicy(data []byte) (Policy, error) {
	var p Policy
	if decode(data, PolicyLimit, &p) != nil {
		return p, ErrInput
	}
	return p, validatePolicy(p)
}
func ParseHistory(data []byte) (History, error) {
	var h History
	if decode(data, HistoryLimit, &h) != nil {
		return h, ErrInput
	}
	return h, nil
}

// The token pass rejects duplicate keys before ordinary struct decoding could overwrite them.
func decode(data []byte, limit int, out any) error {
	if len(data) > limit || !utf8.Valid(data) {
		return ErrInput
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if scan(d, 0) != nil {
		return ErrInput
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrInput
	}
	if required(data, reflect.TypeOf(out).Elem()) != nil {
		return ErrInput
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return ErrInput
	}
	return nil
}
func scan(d *json.Decoder, depth int) error {
	if depth > 32 {
		return ErrInput
	}
	t, e := d.Token()
	if e != nil {
		return ErrInput
	}
	v, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch v {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			s, ok := k.(string)
			if e != nil || !ok || seen[s] {
				return ErrInput
			}
			seen[s] = true
			if scan(d, depth+1) != nil {
				return ErrInput
			}
		}
		t, e = d.Token()
		if e != nil || t != json.Delim('}') {
			return ErrInput
		}
	case '[':
		for d.More() {
			if scan(d, depth+1) != nil {
				return ErrInput
			}
		}
		t, e = d.Token()
		if e != nil || t != json.Delim(']') {
			return ErrInput
		}
	default:
		return ErrInput
	}
	return nil
}
func required(raw json.RawMessage, typ reflect.Type) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if typ.Kind() == reflect.Pointer {
			return nil
		}
		return ErrInput
	}
	if typ.Kind() == reflect.Pointer {
		return required(raw, typ.Elem())
	}
	switch typ.Kind() {
	case reflect.Struct:
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil {
			return ErrInput
		}
		if len(obj) != typ.NumField() {
			return ErrInput
		}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			v, ok := obj[f.Tag.Get("json")]
			if !ok || required(v, f.Type) != nil {
				return ErrInput
			}
		}
	case reflect.Slice:
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) != nil {
			return ErrInput
		}
		for _, v := range arr {
			if required(v, typ.Elem()) != nil {
				return ErrInput
			}
		}
	}
	return nil
}
func timestamp(s string) (time.Time, error) {
	if len(s) > 40 || !strings.HasSuffix(s, "Z") {
		return time.Time{}, ErrInput
	}
	t, e := time.Parse(time.RFC3339Nano, s)
	if e != nil || t.Year() < 1 {
		return time.Time{}, ErrInput
	}
	return t, nil
}
func validatePolicy(p Policy) error {
	if p.Profile != "postmerge-policy/0" || len(p.Classes) < 1 || len(p.Classes) > 64 {
		return ErrPolicy
	}
	seen := map[string]bool{}
	for _, c := range p.Classes {
		if !identifier.MatchString(c.ID) || seen[c.ID] || c.MaxCorrectionBP < 0 || c.MaxCorrectionBP > 10000 || c.MinSamples < 1 || c.MinSamples > 10000 || c.DemoteRuns < 0 || c.DemoteRuns > 10000 {
			return ErrPolicy
		}
		seen[c.ID] = true
	}
	return nil
}
func duration(n *int64, unknown bool) bool {
	if unknown {
		return n == nil
	}
	return n != nil && *n >= 0 && *n <= maxDuration
}
func validateHistory(p Policy, h History) error {
	if h.Profile != "postmerge-history/0" || len(h.Classes) > 64 || len(h.Runs) > 10000 || len(h.Events) > 50000 {
		return ErrHistory
	}
	classes := map[string]bool{}
	for _, c := range p.Classes {
		classes[c.ID] = true
	}
	meta := map[string]bool{}
	for _, m := range h.Classes {
		_, a := timestamp(m.RunsThrough)
		_, b := timestamp(m.EventsThrough)
		if !classes[m.Class] || meta[m.Class] || m.LastSequence < 0 || m.LastSequence > 10000 || a != nil || b != nil {
			return ErrHistory
		}
		meta[m.Class] = true
	}
	runs := map[string]Run{}
	seq := map[string]map[int]bool{}
	order := map[string][]Run{}
	for _, r := range h.Runs {
		_, e := timestamp(r.At)
		_, exists := runs[r.ID]
		if !identifier.MatchString(r.ID) || !classes[r.Class] || exists || r.Sequence < 1 || r.Sequence > 10000 || e != nil || len(r.Stages) > 32 {
			return ErrHistory
		}
		switch r.Outcome {
		case "generated", "no-change", "failed", "unknown":
		default:
			return ErrHistory
		}
		if seq[r.Class] == nil {
			seq[r.Class] = map[int]bool{}
		}
		if seq[r.Class][r.Sequence] {
			return ErrHistory
		}
		seq[r.Class][r.Sequence] = true
		order[r.Class] = append(order[r.Class], r)
		stages := map[string]bool{}
		for _, s := range r.Stages {
			if !identifier.MatchString(s.Name) || stages[s.Name] || !duration(s.DurationMS, s.Outcome == "unknown") {
				return ErrHistory
			}
			stages[s.Name] = true
			switch s.Outcome {
			case "passed", "failed", "skipped", "unknown":
			default:
				return ErrHistory
			}
		}
		if !duration(r.Followup.DurationMS, r.Followup.Status == "unknown") {
			return ErrHistory
		}
		switch r.Followup.Status {
		case "created", "unknown":
		case "no-op":
			if *r.Followup.DurationMS != 0 {
				return ErrHistory
			}
		default:
			return ErrHistory
		}
		if r.Git != nil {
			g := r.Git
			if r.Outcome != "generated" || !filepath.IsAbs(g.Root) || len(g.Root) > 4096 || strings.ContainsRune(g.Root, 0) || !oid.MatchString(g.Bot) || !oid.MatchString(g.Approved) || len(g.Bot) != len(g.Approved) {
				return ErrHistory
			}
		}
		runs[r.ID] = r
	}
	for _, rows := range order {
		sort.Slice(rows, func(i, j int) bool { return timeIDLess(rows[i].At, rows[i].ID, rows[j].At, rows[j].ID) })
		for i := 1; i < len(rows); i++ {
			if rows[i].Sequence <= rows[i-1].Sequence {
				return ErrHistory
			}
		}
	}
	events := map[string]bool{}
	for _, ev := range h.Events {
		r, ok := runs[ev.RunID]
		at, e := timestamp(ev.At)
		rt, _ := timestamp(r.At)
		if !identifier.MatchString(ev.ID) || events[ev.ID] || !ok || e != nil || at.Before(rt) {
			return ErrHistory
		}
		events[ev.ID] = true
		switch ev.Kind {
		case "revert", "correction":
			if r.Outcome != "generated" {
				return ErrHistory
			}
		case "invalid-finding", "containment":
		default:
			return ErrHistory
		}
	}
	return nil
}
func timeIDLess(a, ai, b, bi string) bool {
	at, _ := timestamp(a)
	bt, _ := timestamp(b)
	if at.Equal(bt) {
		return ai < bi
	}
	return at.Before(bt)
}
