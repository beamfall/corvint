package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestReaderUserInventoryPreservesIdentityChecks(t *testing.T) {
	a := readerAudit{AuthorityUID: "450", AuthorityGID: "450", ReaderUID: "501", ReaderName: "tester"}
	const users = "_corvintauthority 450\ntester 501\n"
	for _, tt := range []struct {
		name, key, raw string
		want           []string
		bad            bool
	}{
		{name: "PLE-V0-011 dotted service UID", key: "UniqueID", raw: users + "com.malwarebytes.mbam.nobody 1000\n"},
		{name: "dotted service primary GID", key: "PrimaryGroupID", raw: "_corvintauthority 450\ntester 20\ncom.malwarebytes.mbam.nobody -2\n", want: []string{"_corvintauthority"}},
		{name: "implicit dotted member retained", key: "PrimaryGroupID", raw: "_corvintauthority 450\ncom.vendor.service 450\n", want: []string{"_corvintauthority", "com.vendor.service"}},
		{name: "dotted authority collision", key: "UniqueID", raw: users + "com.vendor.service 450\n", bad: true},
		{name: "dotted reader collision", key: "UniqueID", raw: users + "com.vendor.service 501\n", bad: true},
		{name: "duplicate exact row", key: "UniqueID", raw: users + "tester 501\n", bad: true},
		{name: "canonical name replaced", key: "UniqueID", raw: "_corvintauthority 450\ntester.alias 501\n", bad: true},
		{name: "missing reader", key: "UniqueID", raw: "_corvintauthority 450\n", bad: true},
		{name: "noncanonical UID", key: "UniqueID", raw: users + "com.vendor.service 01000\n", bad: true},
		{name: "noncanonical GID", key: "PrimaryGroupID", raw: "com.vendor.service -02\n", bad: true},
		{name: "extra field", key: "UniqueID", raw: users + "com.vendor.service 1000 extra\n", bad: true},
		{name: "missing ID", key: "UniqueID", raw: users + "com.vendor.service\n", bad: true},
		{name: "unsafe name", key: "UniqueID", raw: users + "com/vendor/service 1000\n", bad: true},
		{name: "non ASCII name", key: "UniqueID", raw: users + "cöm.vendor.service 1000\n", bad: true},
		{name: "long name", key: "UniqueID", raw: users + strings.Repeat("a", 257) + " 1000\n", bad: true},
		{name: "name bound", key: "UniqueID", raw: users + strings.Repeat("a", 256) + " 1000\n"},
		{name: "output bound", key: "UniqueID", raw: strings.Repeat(" ", 1<<20+1), bad: true},
		{name: "unsupported key", key: "GeneratedUID", raw: users, bad: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateReaderUserInventory([]byte(tt.raw), tt.key, a)
			if (err != nil) != tt.bad {
				t.Fatalf("error=%v bad=%v", err, tt.bad)
			}
			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("members=%v want=%v", got, tt.want)
			}
		})
	}
}
