package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseRejectsPathAndModeSubstitution(t *testing.T) {
	t.Run("PLE-V0-002 release manifest rejects path and mode substitution", func(t *testing.T) {
		base := releaseManifest{Profile: "corvint-authority-release/1", SourceRevision: strings.Repeat("a", 40), AdapterTemplateSHA256: strings.Repeat("d", 64), Files: []releaseFile{{"authority-hook.json", strings.Repeat("b", 64), "0444"}, {"bin/git", strings.Repeat("b", 64), "0555"}, {"corvint", strings.Repeat("b", 64), "0555"}, {"go/bin/go", strings.Repeat("b", 64), "0555"}, {"local-authority", strings.Repeat("b", 64), "0555"}}}
		base.ReleaseID = sourceReleaseID(base)
		if e := validateRelease(base); e != nil {
			t.Fatal(e)
		}
		for _, path := range []string{"../key", "/Library/key", "go/../../key", "go/../key", "bin/other"} {
			bad := base
			bad.Files = append([]releaseFile(nil), base.Files...)
			bad.Files[0].Path = path
			if validateRelease(bad) == nil {
				t.Fatal(path)
			}
		}
		bad := base
		bad.Files = append([]releaseFile(nil), base.Files...)
		bad.Files[0].Mode = "0777"
		if validateRelease(bad) == nil {
			t.Fatal("writable release")
		}

	})
}
func TestAuthorityCommandsDefaultUnavailable(t *testing.T) {
	if e := operatorRun(strings.Repeat("a", 64)); e == nil {
		t.Fatal("operator unavailable")
	}
	if _, _, _, e := loadExecution(); e == nil {
		t.Fatal("unadmitted root loaded")
	}
}

func TestAccountLedgerIdentity(t *testing.T) {
	good := accountRecord{"corvint-installed-accounts/0", []installedAccount{{"_corvintauthority", "450"}, {"_corvintcheck", "451"}}}
	if !validAccountRecord(good) {
		t.Fatal("valid ledger")
	}
	for _, id := range []string{"450", "0", "0451", "501"} {
		bad := good
		bad.Accounts = append([]installedAccount(nil), good.Accounts...)
		bad.Accounts[1].ID = id
		if validAccountRecord(bad) {
			t.Fatal(id)
		}
	}
}

func TestPrepareReleasePreflightLeavesNoPayload(t *testing.T) {
	for _, revision := range []string{"ca6f066b", strings.Repeat("z", 40), strings.Repeat("a", 40)} {
		t.Run(revision, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "bundle")
			if err := prepareRelease(output, "/missing-consumer", "/missing-go", "/missing-git", revision, "/missing-adapter"); err == nil {
				t.Fatal("invalid inputs accepted")
			}
			if _, err := os.Lstat(output); !os.IsNotExist(err) {
				t.Fatalf("preflight created payload: %v", err)
			}
		})
	}
}
