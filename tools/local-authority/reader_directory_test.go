package main

import (
	"encoding/json"
	"github.com/Beamfall/corvint/internal/localauthority"
	"os"
	"strings"
	"testing"
)

func testReaderAudit() readerAudit {
	return readerAudit{Profile: "corvint-reader-admission-audit/0", RootID: "operator-owned", Epoch: "1", Generation: "2", AuthorityUID: "450", AuthorityGID: "450", ReaderUID: "501", ReaderName: "tester", AuthorityUUID: "11111111-1111-1111-1111-111111111111", ReaderUUID: "22222222-2222-2222-2222-222222222222", GroupUUID: "33333333-3333-3333-3333-333333333333", PrivilegeAuditSHA256: strings.Repeat("a", 64), LocalDirectoryOnly: true, NoOtherGroupPrivileges: true}
}
func testReaderGroup(a readerAudit) map[string][]string {
	return map[string][]string{"dsAttrTypeStandard:PrimaryGroupID": {a.AuthorityGID}, "dsAttrTypeStandard:GeneratedUID": {a.GroupUUID}, "dsAttrTypeStandard:RecordName": {"_corvintauthority"}}
}
func TestReaderGroupAdmissionAndPartialWithdrawal(t *testing.T) {
	a := testReaderAudit()
	if e := validateReaderAudit(a); e != nil {
		t.Fatal(e)
	}
	for _, names := range [][]string{nil, {a.ReaderName}} {
		for _, ids := range [][]string{nil, {a.ReaderUUID}} {
			g := testReaderGroup(a)
			g["dsAttrTypeStandard:GroupMembership"] = names
			g["dsAttrTypeStandard:GroupMembers"] = ids
			if e := validateReaderGroup(a, g, []string{"_corvintauthority"}, "rollback"); e != nil {
				t.Fatal(e)
			}
			if got := validateReaderGroup(a, g, []string{"_corvintauthority"}, "before") == nil; got != (len(names) == 0 && len(ids) == 0) {
				t.Fatal("before membership")
			}
			if got := validateReaderGroup(a, g, []string{"_corvintauthority"}, "after") == nil; got != (len(names) == 1 && len(ids) == 1) {
				t.Fatal("after membership")
			}
		}
	}
	for _, key := range []string{"GroupMembership", "GroupMembers", "NestedGroups", "PrimaryGroupID", "GeneratedUID", "RecordName"} {
		t.Run(key, func(t *testing.T) {
			g := testReaderGroup(a)
			g["dsAttrTypeStandard:"+key] = []string{"unexpected"}
			if validateReaderGroup(a, g, []string{"_corvintauthority"}, "rollback") == nil {
				t.Fatal("accepted drift")
			}
		})
	}
	if validateReaderGroup(a, testReaderGroup(a), []string{"_corvintauthority", "other"}, "before") == nil {
		t.Fatal("other primary member")
	}
}
func TestReaderPlistClosedShape(t *testing.T) {
	good := `<?xml version="1.0"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>dsAttrTypeStandard:GroupMembers</key><array><string>one</string></array></dict></plist>`
	r, e := directoryPlist([]byte(good))
	if e != nil || len(attribute(r, "GroupMembers")) != 1 {
		t.Fatalf("%v %v", r, e)
	}
	for _, bad := range []string{strings.Replace(good, "<array>", "<dict>", 1), strings.Replace(good, "<string>one</string>", "<integer>1</integer>", 1), strings.Replace(good, "</dict>", "<key>dsAttrTypeStandard:GroupMembers</key><array/></dict>", 1), good + good, strings.Repeat("x", 65537)} {
		if _, e := directoryPlist([]byte(bad)); e == nil {
			t.Fatal("accepted malformed plist")
		}
	}
}
func TestReaderAuditRejectsAmbiguousIdentity(t *testing.T) {
	for _, change := range []func(*readerAudit){func(a *readerAudit) { a.ReaderUID = "0" }, func(a *readerAudit) { a.ReaderUID = a.AuthorityUID }, func(a *readerAudit) { a.ReaderUID = "0501" }, func(a *readerAudit) { a.ReaderName = "../user" }, func(a *readerAudit) { a.ReaderUUID = "invalid" }, func(a *readerAudit) { a.NoOtherGroupPrivileges = false }, func(a *readerAudit) { a.Generation = "01" }, func(a *readerAudit) { a.PrivilegeAuditSHA256 = "" }} {
		a := testReaderAudit()
		change(&a)
		if validateReaderAudit(a) == nil {
			t.Fatal("accepted bad audit")
		}
	}
}

func TestReaderTemplatesRemainInert(t *testing.T) {
	for _, name := range []string{"reader-admission-audit.template.json", "execution-root.template.json", "native-bootstrap.template.json"} {
		raw, e := os.ReadFile("../../conformance/local-authority-v0/templates/" + name)
		if e != nil {
			t.Fatal(e)
		}
		var value map[string]any
		if e = json.Unmarshal(raw, &value); e != nil {
			t.Fatal(e)
		}
		switch name {
		case "reader-admission-audit.template.json":
			var a readerAudit
			if localauthority.Decode(raw, &a) == nil && validateReaderAudit(a) == nil {
				t.Fatal("template grants reader")
			}
		case "execution-root.template.json":
			if value["profile"] != "corvint-protected-root/1" || value["revoked"] != true || value["hostQualification"] != nil || value["readerUid"] != nil {
				t.Fatal("active root template")
			}
		case "native-bootstrap.template.json":
			phases := value["phases"].([]any)
			if len(phases) != 2 {
				t.Fatal("finite phases")
			}
			for _, phase := range phases {
				p := phase.(map[string]any)
				if p["enrollmentHandle"] != nil || p["issuedAt"] != nil || p["expiresAt"] != nil {
					t.Fatal("invented campaign grant")
				}
			}
		}
	}
}

func TestReaderDirectoryIDsRejectAliases(t *testing.T) {
	for _, v := range []string{"450", "501", "-2", "4294967294"} {
		if !canonicalDirectoryID(v) {
			t.Fatal(v)
		}
	}
	for _, v := range []string{"0450", "+450", " 450", "450.0", "4.5e2", "-02", "4294967296"} {
		if canonicalDirectoryID(v) {
			t.Fatal(v)
		}
	}
}

func TestReaderWithdrawalRejectsOwnerDrift(t *testing.T) {
	for _, owner := range []uint32{0, 450} {
		if e := validateRestrictedReaderOwner(owner, "450"); e != nil {
			t.Fatal(e)
		}
	}
	for _, owner := range []uint32{501, 451} {
		if validateRestrictedReaderOwner(owner, "450") == nil {
			t.Fatal("0700 owner retains read access", owner)
		}
	}
}
