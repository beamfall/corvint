package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/authoritystore"
	"github.com/Beamfall/corvint/internal/localauthority"
)

const readerLedgerName = "installed-reader.json"

type readerLedger struct {
	Profile     string      `json:"profile"`
	Audit       readerAudit `json:"audit"`
	AuditSHA256 string      `json:"auditSHA256"`
	State       string      `json:"state"`
	Device      string      `json:"device"`
	Inode       string      `json:"inode"`
}

func readerPolicyInactive() error {
	raw, e := readRootFile(filepath.Join(authoritystore.RootPath, "accepted-root.json"), 128<<10)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	var root authoritystore.RootDocument
	if localauthority.Decode(raw, &root) != nil || root.Profile != authoritystore.RootProfile || !root.Revoked {
		return errors.New("independently revoke accepted root before reader mutation")
	}
	return nil
}
func readReaderRecord(path string) (readerLedger, error) {
	var l readerLedger
	raw, e := readRootFile(path, 32<<10)
	if e != nil {
		return l, e
	}
	if decodeReaderJSON(raw, &l) != nil || l.Profile != "corvint-installed-reader/0" || validateReaderAudit(l.Audit) != nil {
		return l, errors.New("reader ledger")
	}
	if _, e = decodeHex(l.AuditSHA256, 32); e != nil {
		return l, e
	}
	if l.State != "INTENT" && l.State != "DIRECTORY_BOUND" && l.State != "MEMBERSHIP_VERIFIED" && l.State != "ACTIVE" {
		return l, errors.New("reader ledger state")
	}
	if (l.Device == "") != (l.Inode == "") || l.State != "INTENT" && l.Inode == "" {
		return l, errors.New("reader ledger binding")
	}
	for _, s := range []string{l.Device, l.Inode} {
		if s != "" {
			n, e := strconv.ParseUint(s, 10, 64)
			if e != nil || strconv.FormatUint(n, 10) != s {
				return l, errors.New("reader ledger identity")
			}
		}
	}
	return l, nil
}
func readerDSCL(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/dscl", append([]string{"."}, args...)...)
	cmd.Env = []string{}
	var out limitedBuffer
	out.max = 1 << 20
	cmd.Stdout = &out
	var diagnostic limitedBuffer
	diagnostic.max = 4096
	cmd.Stderr = &diagnostic
	if e := cmd.Run(); e != nil {
		return nil, e
	}
	if out.overflow || diagnostic.overflow {
		return nil, errors.New("directory output bound")
	}
	return out.Bytes(), nil
}
func directoryRecord(path string) (map[string][]string, error) { return directoryNodeRecord(".", path) }
func directoryNodeRecord(node, path string) (map[string][]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	args := []string{"-plist", node, "-read", path}
	if strings.HasPrefix(path, "/Users/") {
		args = append(args, "RecordName", "UniqueID", "GeneratedUID")
	}
	cmd := exec.CommandContext(ctx, "/usr/bin/dscl", args...)
	cmd.Env = []string{}
	var out limitedBuffer
	out.max = 64 << 10
	cmd.Stdout = &out
	var diagnostic limitedBuffer
	diagnostic.max = 4096
	cmd.Stderr = &diagnostic
	if e := cmd.Run(); e != nil {
		return nil, fmt.Errorf("directory read failed: %w", e)
	}
	if out.overflow || diagnostic.overflow {
		return nil, errors.New("directory output bound")
	}
	return directoryPlist(out.Bytes())
}
func checkReaderDirectory(a readerAudit, phase string) (map[string][]string, error) {
	raw, e := readRootFile(filepath.Join(authoritystore.RootPath, "installed-accounts.json"), 4096)
	if e != nil {
		return nil, e
	}
	var accounts accountRecord
	if strictDecode(raw, &accounts) != nil || !validAccountRecord(accounts) || accounts.Accounts[0].ID != a.AuthorityUID || accounts.Accounts[0].ID != a.AuthorityGID || accounts.Accounts[1].ID == a.ReaderUID {
		return nil, errors.New("reader account ledger disagreement")
	}
	for _, user := range []struct{ name, uid, uuid string }{{"_corvintauthority", a.AuthorityUID, a.AuthorityUUID}, {a.ReaderName, a.ReaderUID, a.ReaderUUID}} {
		r, e := directoryRecord("/Users/" + user.name)
		if e != nil {
			return nil, e
		}
		resolved, err := directoryNodeRecord("/Search", "/Users/"+user.name)
		if err != nil {
			return nil, err
		}
		if !exactAttribute(resolved, "UniqueID", user.uid) || !exactAttribute(resolved, "GeneratedUID", user.uuid) {
			return nil, errors.New("search identity disagrees")
		}
		if e := validateReaderUser(r, user.name, user.uid, user.uuid, user.name == a.ReaderName, directoryNodeRecord); e != nil {
			return nil, e
		}
	}
	for _, pair := range []struct{ name, uid string }{{"_corvintauthority", a.AuthorityUID}, {a.ReaderName, a.ReaderUID}} {
		u, e := user.Lookup(pair.name)
		if e != nil || u.Uid != pair.uid {
			return nil, errors.New("system user identity")
		}
		u, e = user.LookupId(pair.uid)
		if e != nil || u.Username != pair.name {
			return nil, errors.New("system reverse user identity")
		}
	}
	resolvedGroup, e := user.LookupGroup("_corvintauthority")
	if e != nil || resolvedGroup.Gid != a.AuthorityGID {
		return nil, errors.New("system group identity")
	}
	resolvedGroup, e = user.LookupGroupId(a.AuthorityGID)
	if e != nil || resolvedGroup.Name != "_corvintauthority" {
		return nil, errors.New("system reverse group identity")
	}
	searchGroup, e := directoryNodeRecord("/Search", "/Groups/_corvintauthority")
	if e != nil || !exactAttribute(searchGroup, "GeneratedUID", a.GroupUUID) || !exactAttribute(searchGroup, "PrimaryGroupID", a.AuthorityGID) {
		return nil, errors.New("search group identity")
	}
	// Full UID and primary-GID lists close duplicate identities and implicit members.
	primary := []string{}
	for _, key := range []string{"UniqueID", "PrimaryGroupID"} {
		raw, e := readerDSCL("-list", "/Users", key)
		if e != nil || len(raw) > 1<<20 {
			return nil, errors.New("directory member inventory unavailable")
		}
		members, err := validateReaderUserInventory(raw, key, a)
		if err != nil {
			return nil, err
		}
		if key == "PrimaryGroupID" {
			primary = members
		}
	}
	raw, e = readerDSCL("-list", "/Groups", "PrimaryGroupID")
	if e != nil || len(raw) > 1<<20 {
		return nil, errors.New("group identity inventory unavailable")
	}
	count := 0
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 2 || !canonicalDirectoryID(f[1]) {
			return nil, errors.New("ambiguous group inventory")
		}
		if f[1] == a.AuthorityGID {
			if f[0] != "_corvintauthority" {
				return nil, errors.New("duplicate authority GID")
			}
			count++
		}
	}
	if count != 1 {
		return nil, errors.New("authority GID identity")
	}
	group, e := directoryRecord("/Groups/_corvintauthority")
	if e != nil {
		return nil, e
	}
	return group, validateReaderGroup(a, group, primary, phase)
}
func admitReader(auditPath string) error {
	if os.Geteuid() != 0 {
		return errors.New("operator root required")
	}
	unlock, e := operatorLock()
	if e != nil {
		return e
	}
	defer unlock()
	if e = readerPolicyInactive(); e != nil {
		return e
	}
	raw, e := readRootFile(auditPath, 32<<10)
	if e != nil {
		return e
	}
	var a readerAudit
	if e = decodeReaderJSON(raw, &a); e != nil {
		return e
	}
	if e = validateReaderAudit(a); e != nil {
		return e
	}
	if _, e = checkReaderDirectory(a, "before"); e != nil {
		return e
	}
	l := readerLedger{Profile: "corvint-installed-reader/0", Audit: a, AuditSHA256: digest(raw), State: "INTENT"}
	return runReaderAdmission(l, readerAdmissionOps{
		intent: func(l readerLedger) error {
			return exclusiveFile(filepath.Join(authoritystore.RootPath, readerLedgerName), mustJSONLine(l), 0600)
		},
		create: createReaderEvidence, close: closeReaderEvidence, persist: writeReaderLedger, own: ownReaderEvidence,
		add: func(key, value string) error {
			_, e := readerDSCL("-append", "/Groups/_corvintauthority", key, value)
			return e
		},
		verify: func() error { _, e := checkReaderDirectory(a, "after"); return e }, expose: exposeReaderEvidence,
	})
}
func withdrawReader() error {
	if os.Geteuid() != 0 {
		return errors.New("operator root required")
	}
	unlock, e := operatorLock()
	if e != nil {
		return e
	}
	defer unlock()
	if e = readerPolicyInactive(); e != nil {
		return e
	}
	l, e := readReaderRecord(filepath.Join(authoritystore.RootPath, readerLedgerName))
	if e != nil {
		return e
	}
	return runReaderWithdrawal(l, readerWithdrawalOps{
		restrict: restrictReaderEvidence,
		check:    func() (map[string][]string, error) { return checkReaderDirectory(l.Audit, "rollback") },
		remove: func(key, value string) error {
			_, e := readerDSCL("-delete", "/Groups/_corvintauthority", key, value)
			return e
		},
		verifyEmpty: func() error { _, e := checkReaderDirectory(l.Audit, "before"); return e }, retire: retireReaderLedger,
	})
}
