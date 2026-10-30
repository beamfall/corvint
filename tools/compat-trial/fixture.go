package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The registry is owned by this source, never by a descriptor or CLI argument.
// The executable entry is the currently executing native fixture implementation;
// only its bytes may be replayed, and only through the closed fixture argv grammar.
// stdin entries are fixed byte sequences generated here, including bound witnesses.
var ownedVariantHashes string // Set only by the fixed repository build helper.

func ownedRegistry(binary []byte) fixtureRegistry {
	r := fixtureRegistry{{"compat-trial-native-fixture", digest(binary)}}
	for i, sha := range strings.Split(ownedVariantHashes, ",") {
		if digestPattern.MatchString(sha) {
			r = append(r, fixtureRegistryEntry{fmt.Sprintf("owned-variant-%d", i), sha})
		}
	}
	for name, b := range ownedInputs() {
		r = append(r, fixtureRegistryEntry{name, digest(b)})
	}
	return r
}
func ownedInputs() map[string][]byte {
	return map[string][]byte{"empty": {}, "text": []byte("pinned stdin\n"), "at-bound": bytes.Repeat([]byte{'x'}, inputLimit), "over-bound": bytes.Repeat([]byte{'x'}, inputLimit+1)}
}
func validateFixtureArgs(argv []string) error {
	if len(argv) < 2 || argv[0] != "--owned-fixture" {
		return errors.New("only the owned fixture entrypoint may execute")
	}
	counts := map[string]int{"stdout": 3, "stderr": 3, "stdin": 2, "env": 2, "overflow": 4, "sleep": 3, "exit": 3, "write": 4, "delete": 3, "mode": 3, "unstable": 2, "unstable-write": 3}
	count, ok := counts[argv[1]]
	if !ok || len(argv) != count {
		return errors.New("invalid owned fixture operation")
	}
	for _, a := range argv {
		if strings.ContainsRune(a, 0) {
			return errors.New("NUL in fixture argv")
		}
	}
	switch argv[1] {
	case "write", "delete", "mode", "unstable-write":
		if !cleanRelative(argv[2], false) {
			return errors.New("fixture path must stay scratch relative")
		}
	case "overflow":
		for _, v := range argv[2:] {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || n > outputLimit+1 {
				return errors.New("invalid output witness")
			}
		}
	case "sleep":
		d, err := time.ParseDuration(argv[2])
		if err != nil || d < 0 || d > 30*time.Second {
			return errors.New("invalid sleep witness")
		}
	case "exit":
		n, err := strconv.Atoi(argv[2])
		if err != nil || n < 0 || n > 255 {
			return errors.New("invalid exit witness")
		}
	}
	return nil
}
func ownedFixture(argv []string) int {
	if validateFixtureArgs(argv) != nil {
		return 125
	}
	var err error
	switch argv[1] {
	case "stdout":
		_, err = io.WriteString(os.Stdout, argv[2])
	case "stderr":
		_, err = io.WriteString(os.Stderr, argv[2])
	case "stdin":
		_, err = io.Copy(os.Stdout, os.Stdin)
	case "env":
		env := os.Environ()
		sort.Strings(env)
		_, err = fmt.Fprint(os.Stdout, strings.Join(env, "\n"))
	case "overflow":
		a, _ := strconv.Atoi(argv[2])
		b, _ := strconv.Atoi(argv[3])
		_, err = os.Stdout.Write(bytes.Repeat([]byte{'a'}, a))
		if err == nil {
			_, err = os.Stderr.Write(bytes.Repeat([]byte{'b'}, b))
		}
	case "sleep":
		d, _ := time.ParseDuration(argv[2])
		time.Sleep(d)
	case "exit":
		n, _ := strconv.Atoi(argv[2])
		return n
	case "write":
		err = os.WriteFile(argv[2], []byte(argv[3]), 0644)
	case "delete":
		err = os.Remove(argv[2])
	case "mode":
		err = os.Chmod(argv[2], 0600)
	case "unstable-write":
		stamp := []byte(fmt.Sprint(time.Now().UnixNano()))
		err = os.WriteFile(argv[2], stamp, 0644)
		if err == nil {
			_, err = os.Stdout.Write(stamp)
		}
	case "unstable":
		_, err = fmt.Fprint(os.Stdout, time.Now().UnixNano())
	}
	if err != nil {
		return 124
	}
	return 0
}
func writeFixtureBundle(dir string, binary []byte) error {
	// A new directory prevents an accidental overwrite of existing user artifacts.
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}
	write := func(name string, b []byte, mode os.FileMode) error {
		return os.WriteFile(filepath.Join(dir, name), b, mode)
	}
	snapshot, err := canonicalSnapshot(nil)
	if err != nil {
		return err
	}
	if err = write("snapshot.tar", snapshot, 0600); err != nil {
		return err
	}
	if err = write("fixture", binary, 0700); err != nil {
		return err
	}
	if err = write("stdin", nil, 0600); err != nil {
		return err
	}
	identity, err := nativeBuildIdentity(binary)
	if err != nil {
		return err
	}
	v := version{"owned-synthetic", "fixture", digest(binary), identity}
	t := task{ID: "owned-compatible", Kind: "synthetic", Repo: "owned-fixture", BaseCommit: "owned-synthetic", Snapshot: "snapshot.tar", SnapshotSHA256: digest(snapshot), Source: "owned native fixture", Old: v, New: v, Argv: []string{"--owned-fixture", "stdout", "exact\n"}, Stdin: reference{"stdin", digest(nil)}, Cwd: ".", Env: []string{"TZ=UTC", "LC_ALL=C"}, ExpectedUnknowns: []string{}, ExpectedEffects: []effect{}, ForbiddenClaims: []string{"external execution", "host sandbox", "scored trial"}, GoldRef: "gold.json"}
	m := manifest{"pilot", "owned synthetic fixtures", "CTR-V0-002", "resource.json", "NOT_PRODUCED", "NOT_PRODUCED", []task{t}}
	r := resourceProfile{"owned-synthetic-process-group", 3, inputLimit, outputLimit, fixtureEntryLimit, fixtureByteLimit, "10s", "120s", map[string]json.RawMessage{}, []effect{}}
	for name, value := range map[string]any{"manifest.json": m, "resource.json": r, "gold.json": gold{"?", false, "owned fixture; accepted_by NOT_PRODUCED"}, "fixture-registry.json": ownedRegistry(binary)} {
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		if err = write(name, b, 0600); err != nil {
			return err
		}
	}
	return nil
}
