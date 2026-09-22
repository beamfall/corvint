package main

import (
	"context"
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/authoritystore"
	"github.com/Beamfall/corvint/internal/localauthority"
)

func retireAccounts() error {
	if os.Geteuid() != 0 {
		return errors.New("operator root required")
	}
	if e := ensureRootDirectory(authoritystore.RootPath); e != nil {
		return e
	}
	unlock, lockErr := operatorLock()
	if lockErr != nil {
		return lockErr
	}
	defer unlock()
	if e := requireReaderWithdrawn(); e != nil {
		return e
	}
	rootFile := filepath.Join(authoritystore.RootPath, "accepted-root.json")
	raw, e := readRootFile(rootFile, 128<<10)
	if e == nil {
		var root authoritystore.RootDocument
		if e = localauthority.Decode(raw, &root); e != nil {
			return e
		}
		if !root.Revoked {
			return errors.New("revoke accepted root first")
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	raw, e = readRootFile(filepath.Join(authoritystore.RootPath, "installed-accounts.json"), 4096)
	if e != nil {
		return e
	}
	var record accountRecord
	if e = strictDecode(raw, &record); e != nil {
		return e
	}
	if !validAccountRecord(record) {
		return errors.New("account ledger")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	processes, e := exec.CommandContext(ctx, "/bin/ps", "-axo", "uid=").Output()
	if e != nil {
		return errors.New("cannot establish idle principals")
	}
	for _, a := range record.Accounts {
		if a.Name != "_corvintauthority" && a.Name != "_corvintcheck" {
			return errors.New("unowned account")
		}
		for _, uid := range strings.Fields(string(processes)) {
			if uid == a.ID {
				return errors.New("owned principal still running")
			}
		}
		raw, e := dscl("-read", "/Users/"+a.Name, "UniqueID")
		if e != nil || strings.TrimSpace(string(raw)) != "UniqueID: "+a.ID {
			return errors.New("account identity drift; retained")
		}
		raw, e = dscl("-read", "/Groups/"+a.Name, "PrimaryGroupID")
		if e != nil || strings.TrimSpace(string(raw)) != "PrimaryGroupID: "+a.ID {
			return errors.New("group identity drift; retained")
		}
	}
	// Retain evidence behind root-owned 0700 archive roots. Do not recursively
	// follow or chown authority-controlled paths while privileged.
	owner, _ := strconv.ParseUint(record.Accounts[0].ID, 10, 32)
	privateFD, e := openAuthorityDirectory("private", uint32(owner))
	if e != nil {
		return e
	}
	defer unix.Close(privateFD)
	publicFD, e := openAuthorityDirectory("public", uint32(owner))
	if e != nil {
		return e
	}
	defer unix.Close(publicFD)
	// New evidence validation/archive must precede principal archive mutation.
	// The evidence operation is idempotent if a later step fails.
	if e = runRetirementArchives(archiveRetiredReaderEvidence, func() error {
		for _, fd := range []int{privateFD, publicFD} {
			if e = unix.Fchown(fd, 0, 0); e != nil {
				return e
			}
			if e = unix.Fchmod(fd, 0700); e != nil {
				return e
			}
			if e = unix.Fsync(fd); e != nil {
				return e
			}
		}
		return nil
	}); e != nil {
		return e
	}

	if e = unix.Unlinkat(privateFD, "signing-key", 0); e != nil && e != unix.ENOENT {
		return e
	}
	for _, a := range record.Accounts {
		if _, e = dscl("-delete", "/Users/"+a.Name); e != nil {
			return e
		}
		if _, e = dscl("-delete", "/Groups/"+a.Name); e != nil {
			return e
		}
	}
	return os.Rename(filepath.Join(authoritystore.RootPath, "installed-accounts.json"), filepath.Join(authoritystore.RootPath, "retired-accounts.json"))
}

func validAccountRecord(record accountRecord) bool {
	if record.Profile != "corvint-installed-accounts/0" || len(record.Accounts) != 2 {
		return false
	}
	if record.Accounts[0].Name != "_corvintauthority" || record.Accounts[1].Name != "_corvintcheck" || record.Accounts[0].ID == record.Accounts[1].ID {
		return false
	}
	for _, a := range record.Accounts {
		id, e := strconv.Atoi(a.ID)
		if e != nil || strconv.Itoa(id) != a.ID || id < 450 || id >= 500 {
			return false
		}
	}
	return true
}
