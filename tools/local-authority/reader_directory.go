package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var directoryName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,63}$`)
var directoryUUID = regexp.MustCompile(`^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$`)

// Directory Services plist output is data, not executable or inherited config.
// Accept only a dictionary of unique keys and bounded string arrays.
func directoryPlist(raw []byte) (map[string][]string, error) {
	if len(raw) > 64<<10 {
		return nil, errors.New("directory record bound")
	}
	d := xml.NewDecoder(bytes.NewReader(raw))
	out := map[string][]string{}
	stack := []string{}
	key := ""
	values := []string{}
	seenDict := false
	for {
		token, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		switch t := token.(type) {
		case xml.StartElement:
			parent := ""
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			switch t.Name.Local {
			case "plist":
				if len(stack) != 0 {
					return nil, errors.New("plist nesting")
				}
			case "dict":
				if parent != "plist" || seenDict {
					return nil, errors.New("dictionary shape")
				}
				seenDict = true
			case "key":
				if parent != "dict" || key != "" {
					return nil, errors.New("dictionary key")
				}
				if e = d.DecodeElement(&key, &t); e != nil {
					return nil, e
				}
				if key == "" {
					return nil, errors.New("empty key")
				}
				if _, ok := out[key]; ok {
					return nil, errors.New("duplicate key")
				}
				continue
			case "array":
				if parent != "dict" || key == "" {
					return nil, errors.New("dictionary array")
				}
				values = nil
			case "string":
				if parent != "array" {
					return nil, errors.New("dictionary value")
				}
				var value string
				if e = d.DecodeElement(&value, &t); e != nil {
					return nil, e
				}
				if len(value) > 4096 || len(values) >= 1024 {
					return nil, errors.New("directory values bound")
				}
				values = append(values, value)
				continue
			default:
				return nil, errors.New("unsupported directory value")
			}
			stack = append(stack, t.Name.Local)
		case xml.EndElement:
			if len(stack) == 0 || stack[len(stack)-1] != t.Name.Local {
				return nil, errors.New("directory nesting")
			}
			if t.Name.Local == "array" {
				out[key] = values
				key = ""
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return nil, errors.New("directory text")
			}
		}
	}
	if !seenDict || len(stack) != 0 || key != "" {
		return nil, errors.New("incomplete directory record")
	}
	return out, nil
}
func attribute(record map[string][]string, key string) []string {
	return record["dsAttrTypeStandard:"+key]
}
func exactAttribute(record map[string][]string, key, value string) bool {
	v := attribute(record, key)
	return len(v) == 1 && v[0] == value
}

// Human readers may have macOS account aliases. Bound and validate every name
// before resolving any alias; service identities remain singleton records.
var readerAliasName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.@-]{0,255}$`)

func validateReaderUser(record map[string][]string, name, uid, uuid string, human bool, resolve func(string, string) (map[string][]string, error)) error {
	if !exactAttribute(record, "UniqueID", uid) || !exactAttribute(record, "GeneratedUID", uuid) {
		return errors.New("reader user identity drift")
	}
	names := attribute(record, "RecordName")
	if !human {
		if !exactAttribute(record, "RecordName", name) {
			return errors.New("service record name drift")
		}
		return nil
	}
	if len(names) == 0 || len(names) > 16 {
		return errors.New("reader alias count")
	}
	seen := map[string]bool{}
	canonical := false
	for _, alias := range names {
		folded := strings.ToLower(alias)
		if len(alias) > 256 || !readerAliasName.MatchString(alias) || seen[folded] {
			return errors.New("reader alias shape or collision")
		}
		seen[folded] = true
		canonical = canonical || alias == name
	}
	if !canonical {
		return errors.New("reader canonical name missing")
	}
	for _, alias := range names {
		for _, node := range []string{".", "/Search"} {
			resolved, err := resolve(node, "/Users/"+alias)
			if err != nil {
				return err
			}
			if !exactAttribute(resolved, "UniqueID", uid) || !exactAttribute(resolved, "GeneratedUID", uuid) {
				return errors.New("reader alias identity disagrees")
			}
		}
	}
	return nil
}

func readerDecimal(s string) bool {
	n, e := strconv.ParseUint(s, 10, 16)
	return e == nil && n > 0 && strconv.FormatUint(n, 10) == s
}
func uniqueSubset(actual, allowed []string) bool {
	seen := map[string]bool{}
	for _, s := range actual {
		if seen[s] {
			return false
		}
		seen[s] = true
		ok := false
		for _, a := range allowed {
			ok = ok || s == a
		}
		if !ok {
			return false
		}
	}
	return true
}

type readerAudit struct {
	Profile                string `json:"profile"`
	RootID                 string `json:"rootId"`
	Epoch                  string `json:"epoch"`
	Generation             string `json:"generation"`
	AuthorityUID           string `json:"authorityUid"`
	AuthorityGID           string `json:"authorityGid"`
	ReaderUID              string `json:"readerUid"`
	ReaderName             string `json:"readerName"`
	AuthorityUUID          string `json:"authorityUUID"`
	ReaderUUID             string `json:"readerUUID"`
	GroupUUID              string `json:"groupUUID"`
	PrivilegeAuditSHA256   string `json:"privilegeAuditSHA256"`
	LocalDirectoryOnly     bool   `json:"localDirectoryOnly"`
	NoOtherGroupPrivileges bool   `json:"noOtherGroupPrivileges"`
}

func validateReaderAudit(a readerAudit) error {
	if a.Profile != "corvint-reader-admission-audit/0" || !a.LocalDirectoryOnly || !a.NoOtherGroupPrivileges || !directoryName.MatchString(a.ReaderName) || a.ReaderName == "_corvintauthority" || a.ReaderName == "_corvintcheck" || !directoryUUID.MatchString(a.AuthorityUUID) || !directoryUUID.MatchString(a.ReaderUUID) || !directoryUUID.MatchString(a.GroupUUID) || a.AuthorityUUID == a.ReaderUUID {
		return errors.New("reader audit identity")
	}
	for _, s := range []string{a.AuthorityUID, a.AuthorityGID, a.ReaderUID} {
		if !readerDecimal(s) {
			return errors.New("reader numeric identity")
		}
	}
	if a.ReaderUID == a.AuthorityUID {
		return errors.New("reader is authority")
	}
	if a.RootID == "" || len(a.RootID) > 256 || strings.Contains(strings.ToLower(a.RootID), "fixture") {
		return errors.New("reader root identity")
	}
	for _, s := range []string{a.PrivilegeAuditSHA256} {
		if _, e := decodeHex(s, 32); e != nil {
			return errors.New("reader audit digest")
		}
	}
	for _, s := range []string{a.Epoch, a.Generation} {
		n, e := strconv.ParseUint(s, 10, 64)
		if e != nil || strconv.FormatUint(n, 10) != s {
			return errors.New("reader audit policy generation")
		}
	}
	return nil
}

// Before admission membership must be empty. Rollback allows only exact subsets
// of the two independently written DS attributes, so partial writes are owned.
func validateReaderGroup(a readerAudit, group map[string][]string, primaryUsers []string, phase string) error {
	if !exactAttribute(group, "PrimaryGroupID", a.AuthorityGID) || !exactAttribute(group, "GeneratedUID", a.GroupUUID) || !exactAttribute(group, "RecordName", "_corvintauthority") || len(attribute(group, "NestedGroups")) != 0 {
		return errors.New("authority group identity or nesting")
	}
	if len(primaryUsers) != 1 || primaryUsers[0] != "_corvintauthority" {
		return errors.New("unexpected primary group member")
	}
	names, ids := attribute(group, "GroupMembership"), attribute(group, "GroupMembers")
	if !uniqueSubset(names, []string{a.ReaderName}) || !uniqueSubset(ids, []string{a.ReaderUUID}) {
		return errors.New("unexpected supplementary member")
	}
	switch phase {
	case "before":
		if len(names) != 0 || len(ids) != 0 {
			return errors.New("preexisting reader membership")
		}
	case "after":
		if len(names) != 1 || len(ids) != 1 {
			return errors.New("reader membership incomplete")
		}
	case "rollback":
	default:
		return fmt.Errorf("unknown member phase %q", phase)
	}
	return nil
}

// DS may represent unrelated system accounts with canonical negative IDs.
// Numeric aliases such as 0450 are rejected before any membership comparison.
func canonicalDirectoryID(s string) bool {
	n, e := strconv.ParseInt(s, 10, 64)
	return e == nil && n >= -2147483648 && n <= 4294967295 && strconv.FormatInt(n, 10) == s
}

func validateRestrictedReaderOwner(observed uint32, authority string) error {
	owner, e := strconv.ParseUint(authority, 10, 32)
	if e != nil || observed != 0 && observed != uint32(owner) {
		return errors.New("evidence mode restricted but owner drift prevents complete withdrawal; ledger retained")
	}
	return nil
}

// Operator records are ordinary JSON, unlike signed canonical wire envelopes.
// Reject duplicate keys before typed strict decoding without imposing key order.
func decodeReaderJSON(raw []byte, value any) error {
	if len(raw) > 32<<10 {
		return errors.New("reader document bound")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var visit func(int) error
	visit = func(depth int) error {
		if depth > 16 {
			return errors.New("reader document depth")
		}
		token, e := d.Token()
		if e != nil {
			return e
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			keys := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return e
				}
				s, ok := key.(string)
				if !ok || keys[s] {
					return errors.New("duplicate reader document key")
				}
				keys[s] = true
				if e = visit(depth + 1); e != nil {
					return e
				}
			}
		case '[':
			for d.More() {
				if e = visit(depth + 1); e != nil {
					return e
				}
			}
		default:
			return errors.New("reader document delimiter")
		}
		_, e = d.Token()
		return e
	}
	if e := visit(0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("trailing reader document")
	}
	if e := strictDecode(raw, value); e != nil {
		return e
	}
	var original, typed any
	if e := json.Unmarshal(raw, &original); e != nil {
		return e
	}
	encoded, e := json.Marshal(value)
	if e != nil {
		return e
	}
	if e = json.Unmarshal(encoded, &typed); e != nil {
		return e
	}
	a, e := json.Marshal(original)
	if e != nil {
		return e
	}
	b, e := json.Marshal(typed)
	if e != nil {
		return e
	}
	if !bytes.Equal(a, b) {
		return errors.New("reader document exact fields required")
	}
	return nil
}

// Inventory names include unrelated local service records, not just audited short names.
func validateReaderUserInventory(raw []byte, key string, a readerAudit) ([]string, error) {
	if len(raw) > 1<<20 {
		return nil, errors.New("directory member inventory unavailable")
	}
	identities := map[string][]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 2 || !readerAliasName.MatchString(f[0]) || !canonicalDirectoryID(f[1]) {
			return nil, errors.New("ambiguous directory inventory")
		}
		identities[f[1]] = append(identities[f[1]], f[0])
	}
	if key == "PrimaryGroupID" {
		return identities[a.AuthorityGID], nil
	}
	if key != "UniqueID" {
		return nil, errors.New("unsupported directory inventory")
	}
	for _, pair := range []struct{ id, name string }{{a.AuthorityUID, "_corvintauthority"}, {a.ReaderUID, a.ReaderName}} {
		v := identities[pair.id]
		if len(v) != 1 || v[0] != pair.name {
			return nil, errors.New("duplicate directory UID")
		}
	}
	return nil, nil
}
