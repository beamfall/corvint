package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/authoritystore"
)

type installedAccount struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}
type accountRecord struct {
	Profile  string             `json:"profile"`
	Accounts []installedAccount `json:"accounts"`
}

func dscl(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/dscl", append([]string{"."}, args...)...)
	cmd.Env = []string{}
	return cmd.CombinedOutput()
}
func setupAccounts() error {
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
	// Pre-existing names are collisions, never silently adopted.
	for _, name := range []string{"_corvintauthority", "_corvintcheck"} {
		if _, e := dscl("-read", "/Users/"+name); e == nil {
			return errors.New("existing account collision")
		}
		if _, e := dscl("-read", "/Groups/"+name); e == nil {
			return errors.New("existing group collision")
		}
	}
	used := map[string]bool{}
	for _, kind := range []struct{ path, key string }{{"/Users", "UniqueID"}, {"/Groups", "PrimaryGroupID"}} {
		raw, e := dscl("-list", kind.path, kind.key)
		if e != nil {
			return e
		}
		for _, line := range strings.Split(string(raw), "\n") {
			f := strings.Fields(line)
			if len(f) == 2 {
				used[f[1]] = true
			}
		}
	}
	ids := []string{}
	for id := 450; id < 500 && len(ids) < 2; id++ {
		value := strconv.Itoa(id)
		if !used[value] {
			ids = append(ids, value)
		}
	}
	if len(ids) != 2 {
		return errors.New("no free dedicated account IDs")
	}
	record := accountRecord{Profile: "corvint-installed-accounts/0", Accounts: []installedAccount{{"_corvintauthority", ids[0]}, {"_corvintcheck", ids[1]}}}
	ledger := filepath.Join(authoritystore.RootPath, "installed-accounts.json")
	if e := exclusiveFile(ledger, mustJSONLine(record), 0644); e != nil {
		return e
	}
	// The ledger is durable before mutations. A partial failure leaves exact owned
	// names/IDs for the explicit rollback command; it does not guess cleanup scope.
	for _, account := range record.Accounts {
		steps := [][]string{{"-create", "/Groups/" + account.Name}, {"-create", "/Groups/" + account.Name, "PrimaryGroupID", account.ID}, {"-create", "/Users/" + account.Name}, {"-create", "/Users/" + account.Name, "UniqueID", account.ID}, {"-create", "/Users/" + account.Name, "PrimaryGroupID", account.ID}, {"-create", "/Users/" + account.Name, "UserShell", "/usr/bin/false"}, {"-create", "/Users/" + account.Name, "NFSHomeDirectory", "/var/empty"}, {"-create", "/Users/" + account.Name, "IsHidden", "1"}, {"-create", "/Users/" + account.Name, "Password", "*"}}
		for _, args := range steps {
			if raw, e := dscl(args...); e != nil {
				return fmt.Errorf("account step failed (ledger retained): %w %s", e, raw)
			}
		}
	}
	owner, _ := strconv.Atoi(ids[0])
	for _, entry := range []struct {
		path string
		mode os.FileMode
	}{{"private", 0700}, {"private/enrollments", 0700}, {"private/journal", 0700}, {"public", 0755}} {
		path := filepath.Join(authoritystore.RootPath, entry.path)
		if e := os.Mkdir(path, entry.mode); e != nil {
			return e
		}
		if e := os.Chown(path, owner, owner); e != nil {
			return e
		}
	}
	return nil
}
func operatorKeygen() error {
	if os.Geteuid() != 0 {
		return errors.New("operator root required")
	}
	unlock, lockErr := operatorLock()
	if lockErr != nil {
		return lockErr
	}
	defer unlock()
	p, e := account("_corvintauthority")
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, e := launchAs(ctx, p, []string{"authority-keygen"}, nil, 5*time.Second)
	if e != nil {
		return e
	}
	_, e = os.Stdout.Write(out)
	return e
}
func authorityKeygen() error {
	p, e := account("_corvintauthority")
	if e != nil || uint32(os.Geteuid()) != p.uid {
		return errors.New("dedicated authority required")
	}
	public, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return e
	}
	defer clear(key)
	if e = exclusiveFile(filepath.Join(authoritystore.RootPath, "private", "signing-key"), key, 0600); e != nil {
		return e
	}
	// Public key is a proposal only. No command writes accepted-root.json.
	fmt.Println(hex.EncodeToString(public))
	return nil
}
