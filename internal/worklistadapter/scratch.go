package worklistadapter

import (
	"context"
	"encoding/hex"
	"errors"
	"github.com/Beamfall/corvint/internal/worksource"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Scratch is the observer-owned tuple a producer may nest its source acquisition in.
type Scratch struct{ Parent, Root, Commit string }

// The script/observer owns this tuple outside the target. Recognition permits
// cleanup-safe nesting only; source qualification still checks root and commit.
// Direct binary calls with no marker retain ordinary fixed-/tmp acquisition.
func OwnedScratch() (Scratch, error) {
	var result Scratch
	temporary := os.Getenv("TMPDIR")
	if temporary == "" {
		return result, nil
	}
	ownerPath := filepath.Join(temporary, "corvint-work-queue-owner")
	before, err := os.Lstat(ownerPath)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil || !before.IsDir() || before.Mode().Perm() != 0700 {
		return result, errors.New("invalid source scratch owner")
	}
	temporary, err = filepath.EvalSymlinks(temporary)
	if err != nil {
		return result, err
	}
	temporaryInfo, err := os.Lstat(temporary)
	if err != nil || !temporaryInfo.IsDir() || temporaryInfo.Mode().Perm() != 0700 {
		return result, errors.New("invalid private source scratch")
	}
	root, err := os.OpenRoot(temporary)
	if err != nil {
		return result, err
	}
	defer root.Close()
	owner, err := root.OpenRoot("corvint-work-queue-owner")
	if err != nil {
		return result, err
	}
	defer owner.Close()
	openedOwner, err := owner.Stat(".")
	if err != nil || !os.SameFile(before, openedOwner) {
		return result, errors.New("source scratch owner drift")
	}
	tupleInfo, err := owner.Lstat("tuple")
	if err != nil || !tupleInfo.Mode().IsRegular() || tupleInfo.Mode().Perm() != 0600 || tupleInfo.Size() > 16384 {
		return result, errors.New("invalid source scratch tuple")
	}
	tuple, err := owner.OpenFile("tuple", producerReadFlags(), 0)
	if err != nil {
		return result, err
	}
	defer tuple.Close()
	opened, err := tuple.Stat()
	if err != nil || !os.SameFile(tupleInfo, opened) {
		return result, errors.New("source scratch tuple drift")
	}
	raw, err := io.ReadAll(io.LimitReader(tuple, 16385))
	if err != nil || len(raw) > 16384 {
		return result, errors.New("incomplete source scratch tuple")
	}
	after, err := owner.Lstat("tuple")
	if err != nil || !os.SameFile(opened, after) || opened.Mode() != after.Mode() || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return result, errors.New("source scratch tuple drift")
	}
	fields := strings.Split(string(raw), "\n")
	if len(fields) != 3 || fields[2] != "" || !utf8.ValidString(fields[0]) || !filepath.IsAbs(fields[0]) || filepath.Clean(fields[0]) != fields[0] {
		return result, errors.New("invalid source scratch tuple")
	}
	canonical, err := filepath.EvalSymlinks(fields[0])
	if err != nil || canonical != fields[0] {
		return result, errors.New("noncanonical source scratch root")
	}
	if temporary == canonical || strings.HasPrefix(temporary, canonical+string(os.PathSeparator)) {
		return result, errors.New("source scratch cannot write target")
	}
	if err := worksource.ValidateScratchLocation(context.Background(), canonical, temporary); err != nil {
		return result, err
	}
	oid, err := hex.DecodeString(fields[1])
	if err != nil || (len(oid) != 20 && len(oid) != 32) || strings.ToLower(fields[1]) != fields[1] {
		return result, errors.New("invalid source scratch commit")
	}
	return Scratch{temporary, canonical, fields[1]}, nil
}
