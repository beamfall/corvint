package main

import (
	"errors"
	"github.com/Beamfall/corvint/internal/authoritystore"
	"os"
	"path/filepath"
	"strconv"
)

func operatorLock() (func(), error) {
	if os.Geteuid() != 0 {
		return nil, errors.New("operator root required")
	}
	if e := auditRootDirectory(authoritystore.RootPath, false); e != nil {
		return nil, e
	}
	path := filepath.Join(authoritystore.RootPath, "operator.lock")
	raw := mustJSONLine(struct {
		Profile string `json:"profile"`
		PID     string `json:"pid"`
	}{"corvint-operator-lock/0", strconv.Itoa(os.Getpid())})
	if e := exclusiveFile(path, raw, 0600); e != nil {
		return nil, errors.New("operator already active or stale lock requires explicit cleanup audit")
	}
	return func() { _ = os.Remove(path) }, nil
}
