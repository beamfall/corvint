package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestReaderLedgerRealWriterRoundTrip(t *testing.T) {
	for _, state := range []string{"INTENT", "DIRECTORY_BOUND", "MEMBERSHIP_VERIFIED", "ACTIVE"} {
		l := readerLedger{Profile: "corvint-installed-reader/0", Audit: testReaderAudit(), AuditSHA256: strings.Repeat("a", 64), State: state}
		if state != "INTENT" {
			l.Device = "1"
			l.Inode = "2"
		}
		raw := mustJSONLine(l)
		var got readerLedger
		if e := decodeReaderJSON(raw, &got); e != nil {
			t.Fatal(state, e)
		}
		if !reflect.DeepEqual(l, got) {
			t.Fatal("writer/reader mismatch")
		}
		alias := strings.Replace(string(raw), `"readerUid":`, `"ReaderUid":`, 1)
		if decodeReaderJSON([]byte(alias), &got) == nil {
			t.Fatal("case alias reader identity")
		}
		duplicate := strings.Replace(string(raw), `"state":`, `"state":"INTENT","state":`, 1)
		if decodeReaderJSON([]byte(duplicate), &got) == nil {
			t.Fatal("duplicate ledger state")
		}
	}
}
func TestReaderAdmissionFailureBoundaries(t *testing.T) {
	t.Run("PLE-V0-011 reader admission preserves fail closed ordering", func(t *testing.T) {
		want := []string{"intent", "create", "persist:DIRECTORY_BOUND", "own", "add:GroupMembership", "add:GroupMembers", "verify", "persist:MEMBERSHIP_VERIFIED", "expose", "persist:ACTIVE"}
		for fail := -1; fail < len(want); fail++ {
			t.Run(string(rune('A'+fail+1)), func(t *testing.T) {
				events := []string{}
				closed := false
				durable := readerLedger{}
				persisted := false
				step := func(name string) error {
					events = append(events, name)
					if len(events)-1 == fail {
						return errors.New("injected interruption")
					}
					return nil
				}
				save := func(l readerLedger) error {
					if e := step("persist:" + l.State); e != nil {
						return e
					}
					raw := mustJSONLine(l)
					if e := decodeReaderJSON(raw, &durable); e != nil {
						t.Fatal(e)
					}
					persisted = true
					return nil
				}
				l := readerLedger{Profile: "corvint-installed-reader/0", Audit: testReaderAudit(), State: "INTENT"}
				e := runReaderAdmission(l, readerAdmissionOps{
					intent: func(l readerLedger) error {
						if e := step("intent"); e != nil {
							return e
						}
						return decodeReaderJSON(mustJSONLine(l), &durable)
					},
					create: func(l *readerLedger) (int, error) {
						if e := step("create"); e != nil {
							return -1, e
						}
						l.Device = "1"
						l.Inode = "2"
						l.State = "DIRECTORY_BOUND"
						return 7, nil
					}, close: func(fd int) { closed = fd == 7 }, persist: save,
					own: func(int, readerAudit) error {
						if !persisted || durable.Inode != "2" {
							t.Fatal("ownership before durable binding")
						}
						return step("own")
					},
					add: func(key, value string) error {
						if durable.State != "DIRECTORY_BOUND" {
							t.Fatal("membership before binding")
						}
						return step("add:" + key)
					},
					verify: func() error { return step("verify") }, expose: func(int, readerLedger) error {
						if durable.State != "MEMBERSHIP_VERIFIED" {
							t.Fatal("exposure before verified durable membership")
						}
						return step("expose")
					},
				})
				count := len(want)
				if fail >= 0 {
					count = fail + 1
				}
				if !reflect.DeepEqual(events, want[:count]) {
					t.Fatal(events)
				}
				if (e != nil) != (fail >= 0) {
					t.Fatal(e)
				}
				if closed != (fail < 0 || fail >= 2) {
					t.Fatal("descriptor cleanup", fail, closed)
				}
			})
		}

	})
}
func TestReaderWithdrawalPartialMembershipAndDSFailure(t *testing.T) {
	a := testReaderAudit()
	l := readerLedger{Audit: a}
	for _, names := range [][]string{nil, {a.ReaderName}} {
		for _, ids := range [][]string{nil, {a.ReaderUUID}} {
			events := []string{}
			g := testReaderGroup(a)
			g["dsAttrTypeStandard:GroupMembership"] = names
			g["dsAttrTypeStandard:GroupMembers"] = ids
			e := runReaderWithdrawal(l, readerWithdrawalOps{restrict: func(readerLedger) error { events = append(events, "restrict"); return nil }, check: func() (map[string][]string, error) { events = append(events, "check"); return g, nil }, remove: func(key, value string) error {
				events = append(events, key)
				if key == "GroupMembership" && value != a.ReaderName || key == "GroupMembers" && value != a.ReaderUUID {
					t.Fatal("unowned removal")
				}
				return nil
			}, verifyEmpty: func() error { events = append(events, "empty"); return nil }, retire: func() error { events = append(events, "retire"); return nil }})
			want := []string{"restrict", "check"}
			if len(names) != 0 {
				want = append(want, "GroupMembership")
			}
			if len(ids) != 0 {
				want = append(want, "GroupMembers")
			}
			want = append(want, "empty", "retire")
			if e != nil || !reflect.DeepEqual(events, want) {
				t.Fatal(events, e)
			}
		}
	}
	restricted := false
	retired := false
	e := runReaderWithdrawal(l, readerWithdrawalOps{restrict: func(readerLedger) error { restricted = true; return nil }, check: func() (map[string][]string, error) {
		if !restricted {
			t.Fatal("DS precedes withdrawal")
		}
		return nil, errors.New("DS offline")
	}, retire: func() error { retired = true; return nil }})
	if e == nil || !restricted || retired {
		t.Fatal("DS failure lost ledger")
	}
}
func TestReaderRestrictionDriftAndRetirementFailure(t *testing.T) {
	for _, fault := range []string{"binding", "owner", "acl", "none"} {
		modeRestricted := false
		calls := 0
		e := runReaderRestriction("450", readerRestrictionOps{binding: func() error {
			calls++
			if fault == "binding" {
				return errors.New("inode drift")
			}
			return nil
		}, chmod: func() error { modeRestricted = true; return nil }, owner: func() (uint32, error) {
			if fault == "owner" {
				return 501, nil
			}
			return 450, nil
		}, acl: func() error {
			if fault == "acl" {
				return errors.New("ACL grants")
			}
			return nil
		}, sync: func() error { return nil }})
		if (e == nil) != (fault == "none") || modeRestricted != (fault != "binding") {
			t.Fatal(fault, e)
		}
		if fault == "none" && calls != 2 {
			t.Fatal("missing final binding")
		}
	}
	principalsChanged := false
	attempt := 0
	evidence := func() error {
		attempt++
		if attempt == 1 {
			return errors.New("evidence unavailable")
		}
		return nil
	}
	principals := func() error { principalsChanged = true; return nil }
	if runRetirementArchives(evidence, principals) == nil || principalsChanged {
		t.Fatal("principal mutation preceded evidence preflight")
	}
	if e := runRetirementArchives(evidence, principals); e != nil || !principalsChanged {
		t.Fatal("retry", e)
	}
}
