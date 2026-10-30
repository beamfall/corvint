package jstestprovider

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// RouteStubsAnnotation is the one per-attempt annotation the route-stubs.cjs producer publishes
// (LPCV-V0-072). Its description is the closed JSON RouteStubRecord.
const RouteStubsAnnotation = "corvint-route-stubs"

// Route-stub record bounds. A record outside them is not a record.
const (
	MaxRouteStubs           = 64
	maxRouteStubTextBytes   = 1024
	maxRouteStubLine        = 10_000_000
	maxRouteStubHits        = 1<<31 - 1
	maxRouteStubDescription = 128 << 10
)

// RouteStubRecord is one attempt's producer record of every request stub registered through
// page.route, context.route, routeWebSocket or routeFromHAR on the contexts the attempt observed
// (LPCV-V0-072, LPCV-V0-076). A present record with
// no stubs means the producer ran and saw none; an absent record means nothing was produced and is
// never read as zero hits.
type RouteStubRecord struct {
	Stubs []RouteStub `json:"stubs"`
}

// RouteStub is one registered route pattern, how often its handler ran in the attempt and where
// it was registered. API is omitted for a plain route registration. File and Line are both omitted
// when the producer could not resolve the call. Unrouted is set when the test removed the stub with
// unroute or unrouteAll, and is otherwise omitted (LPCV-V0-214).
type RouteStub struct {
	Target   string `json:"target"`
	API      string `json:"api,omitempty"`
	Kind     string `json:"kind"`
	Pattern  string `json:"pattern"`
	Hits     int    `json:"hits"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Unrouted bool   `json:"unrouted,omitempty"`
}

var (
	routeStubTargets = map[string]bool{"page": true, "context": true}
	routeStubKinds   = map[string]bool{"glob": true, "regexp": true, "predicate": true, "other": true}
	// routeStubAPIs are the registration APIs a stub may name besides the default route
	// (LPCV-V0-076).
	routeStubAPIs = map[string]bool{"": true, "routeWebSocket": true, "routeFromHAR": true}
)

// ValidRouteStubRecord reports whether a record is inside the closed vocabulary and bounds.
func ValidRouteStubRecord(record RouteStubRecord) bool {
	if record.Stubs == nil || len(record.Stubs) > MaxRouteStubs {
		return false
	}
	for _, stub := range record.Stubs {
		if !routeStubTargets[stub.Target] || !routeStubAPIs[stub.API] || !routeStubKinds[stub.Kind] || !routeStubText(stub.Pattern) || stub.Hits < 0 || stub.Hits > maxRouteStubHits {
			return false
		}
		if stub.File == "" && stub.Line == 0 {
			continue
		}
		if !routeStubText(stub.File) || stub.Line < 1 || stub.Line > maxRouteStubLine {
			return false
		}
	}
	return true
}

func routeStubText(value string) bool {
	return value != "" && len(value) <= maxRouteStubTextBytes && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

// ScreenRouteStubRecord applies the product secret screen to every pattern and registration path.
// A pattern is also screened with its regular-expression escapes removed, so a credential written
// as `https:\/\/user:pass@host` is caught; a hit there replaces the pattern with that screened form.
func ScreenRouteStubRecord(record RouteStubRecord) RouteStubRecord {
	stubs := make([]RouteStub, len(record.Stubs))
	for i, stub := range record.Stubs {
		stub.Pattern = screenRouteStubPattern(stub.Pattern)
		stub.File, _ = secretscreen.Screen(stub.File)
		stubs[i] = stub
	}
	return RouteStubRecord{Stubs: stubs}
}

func screenRouteStubPattern(pattern string) string {
	screened, _ := secretscreen.Screen(pattern)
	if unescaped, hit := secretscreen.Screen(regexpEscapes.Replace(pattern)); hit {
		return unescaped
	}
	return screened
}

var regexpEscapes = strings.NewReplacer(`\/`, "/", `\.`, ".", `\:`, ":", `\@`, "@", `\?`, "?", `\=`, "=", `\&`, "&", `\-`, "-")

// UnmarshalJSON decodes a record closed: unknown and duplicate members are refused at every level,
// and every stub must state its hit count, so a missing or null count is never read as zero hits.
// It serves both the producer annotation and a retained receipt's routeStubs member.
func (r *RouteStubRecord) UnmarshalJSON(data []byte) error {
	if irregularJSONMember(data) {
		return errors.New("route-stub record has a duplicate or inexact member")
	}
	var wire struct {
		Stubs []struct {
			Target   string          `json:"target"`
			API      json.RawMessage `json:"api"`
			Kind     string          `json:"kind"`
			Pattern  string          `json:"pattern"`
			Hits     *int            `json:"hits"`
			File     string          `json:"file"`
			Line     int             `json:"line"`
			Unrouted json.RawMessage `json:"unrouted"`
		} `json:"stubs"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	record := RouteStubRecord{}
	if wire.Stubs != nil {
		record.Stubs = make([]RouteStub, len(wire.Stubs))
	}
	for i, stub := range wire.Stubs {
		if stub.Hits == nil {
			return errors.New("route stub has no hit count")
		}
		api := ""
		if stub.API != nil {
			// A present api member must name an API: null decodes to "" and is refused with empty,
			// never read as route.
			if json.Unmarshal(stub.API, &api) != nil || api == "" {
				return errors.New("route stub names no api")
			}
		}
		// A present unrouted member is exactly true: false, null or any other value is refused, so
		// the member has one spelling (LPCV-V0-214).
		if stub.Unrouted != nil && string(stub.Unrouted) != "true" {
			return errors.New("route stub unrouted member is not true")
		}
		record.Stubs[i] = RouteStub{Target: stub.Target, API: api, Kind: stub.Kind, Pattern: stub.Pattern, Hits: *stub.Hits, File: stub.File, Line: stub.Line, Unrouted: stub.Unrouted != nil}
	}
	*r = record
	return nil
}

// routeStubMembers are the exact member names of a record. encoding/json matches names without
// regard to case, so a case variant such as "API" would otherwise overwrite a refused "api".
var routeStubMembers = map[string]bool{"stubs": true, "target": true, "api": true, "kind": true, "pattern": true, "hits": true, "file": true, "line": true, "unrouted": true}

// irregularJSONMember reports whether any object in data names a member twice or by a name that is
// not exactly a route-stub record member, or data is not one well-formed JSON value.
func irregularJSONMember(data []byte) bool {
	return irregularJSONMemberOf(data, routeStubMembers)
}

// irregularJSONMemberOf is irregularJSONMember over the exact member names of one closed record.
func irregularJSONMemberOf(data []byte, members map[string]bool) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var walk func() bool
	walk = func() bool {
		token, err := decoder.Token()
		if err != nil {
			return true
		}
		switch token {
		case json.Delim('{'):
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				name, ok := key.(string)
				if err != nil || !ok || seen[name] || !members[name] {
					return true
				}
				seen[name] = true
				if walk() {
					return true
				}
			}
			_, err = decoder.Token()
			return err != nil
		case json.Delim('['):
			for decoder.More() {
				if walk() {
					return true
				}
			}
			_, err = decoder.Token()
			return err != nil
		}
		return false
	}
	return walk()
}

// routeStubRecord reads the attempt's corvint-route-stubs annotation. Only exactly one closed,
// in-bounds annotation is a record; none, a duplicate or a malformed one yields nil, so the
// consumer reports NOT_PRODUCED rather than zero hits.
func routeStubRecord(annotations []playwrightAnnotation) *RouteStubRecord {
	var found *playwrightAnnotation
	for i := range annotations {
		if annotations[i].Type != RouteStubsAnnotation {
			continue
		}
		if found != nil {
			return nil
		}
		found = &annotations[i]
	}
	if found == nil || found.malformed || len(found.Description) > maxRouteStubDescription {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(found.Description)))
	var record RouteStubRecord
	if decoder.Decode(&record) != nil {
		return nil
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil
	}
	if !ValidRouteStubRecord(record) {
		return nil
	}
	screened := ScreenRouteStubRecord(record)
	return &screened
}

// hasRouteStubs reports the unprofiled-only routeStubs member: an external profile gains a wire
// field only through another profile revision.
func hasRouteStubs(r Receipt) bool {
	for _, test := range r.Tests {
		for _, detail := range test.AttemptDetails {
			if detail.RouteStubs != nil {
				return true
			}
		}
	}
	return false
}
