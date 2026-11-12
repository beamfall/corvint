package jstestprovider

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/Beamfall/corvint/internal/jstestreceipt"
)

// RouteStubsAnnotation is the one per-attempt annotation the route-stubs.cjs producer publishes
// (LPCV-V0-072). Its description is the closed JSON RouteStubRecord.
const RouteStubsAnnotation = "corvint-route-stubs"

// MaxRouteStubs is jstestreceipt.MaxRouteStubs, kept under its old name.
const MaxRouteStubs = jstestreceipt.MaxRouteStubs

// RouteStubRecord is jstestreceipt.RouteStubRecord, kept under its old name.
type RouteStubRecord = jstestreceipt.RouteStubRecord

// RouteStub is jstestreceipt.RouteStub, kept under its old name.
type RouteStub = jstestreceipt.RouteStub

// ValidRouteStubRecord is jstestreceipt.ValidRouteStubRecord, kept under its old name.
func ValidRouteStubRecord(record RouteStubRecord) bool {
	return jstestreceipt.ValidRouteStubRecord(record)
}

// ScreenRouteStubRecord is jstestreceipt.ScreenRouteStubRecord, kept under its old name.
func ScreenRouteStubRecord(record RouteStubRecord) RouteStubRecord {
	return jstestreceipt.ScreenRouteStubRecord(record)
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
	if found == nil || found.malformed || len(found.Description) > jstestreceipt.MaxRouteStubDescription {
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
