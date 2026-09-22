package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWorkProductionFixtureIsolation(t *testing.T) {
	t.Parallel()
	var expired string
	t.Run("first caller cleanup", func(t *testing.T) {
		expired = workProductionFixture(t)
	})
	if _, err := os.Stat(expired); !os.IsNotExist(err) {
		t.Fatal("first caller was not cleaned up")
	}
	first, second := workProductionFixture(t), workProductionFixture(t)
	seed := workProductionSeed.root
	head := materializationGit(t, seed, "rev-parse", "HEAD")
	tracked := materializationGit(t, seed, "ls-files", "--stage")
	t.Run("WQO-V0-004 cloned source content and history", func(t *testing.T) {
		for _, root := range []string{first, second} {
			if got := materializationGit(t, root, "rev-parse", "HEAD"); got != head {
				t.Fatalf("fixture changed HEAD: %s", got)
			}
			if got := materializationGit(t, root, "rev-list", "--count", "HEAD"); got != "2" {
				t.Fatalf("fixture lost pinned parent: %s", got)
			}
			if got := materializationGit(t, root, "ls-files", "--stage"); got != tracked {
				t.Fatal("fixture changed tracked bytes or executable modes")
			}
			if got := materializationGit(t, root, "remote"); got != "" {
				t.Fatalf("fixture retains seed remote: %s", got)
			}
			if _, err := os.Stat(filepath.Join(root, ".git/objects/info/alternates")); !os.IsNotExist(err) {
				t.Fatal("fixture borrows object storage")
			}
		}
	})
	t.Run("WQO-V0-005 private Git and working tree", func(t *testing.T) {
		object := filepath.Join(".git", "objects", head[:2], head[2:])
		firstObject, err := os.Stat(filepath.Join(first, object))
		if err != nil {
			t.Fatal(err)
		}
		for _, root := range []string{seed, second} {
			info, err := os.Stat(filepath.Join(root, object))
			if err != nil {
				t.Fatal(err)
			}
			if os.SameFile(firstObject, info) {
				t.Fatal("fixtures share writable Git objects")
			}
		}
		seedBefore, secondBefore := materializationManifest(t, seed), materializationManifest(t, second)
		if err := os.WriteFile(filepath.Join(first, "script/corvint-work-queue"), []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
			t.Fatal(err)
		}
		materializationGit(t, first, "config", "fixture.private", "true")
		materializationGit(t, first, "add", ".")
		materializationGit(t, first, "commit", "-qm", "private mutation")
		if !reflect.DeepEqual(seedBefore, materializationManifest(t, seed)) {
			t.Fatal("caller mutation changed seed files or Git state")
		}
		if !reflect.DeepEqual(secondBefore, materializationManifest(t, second)) {
			t.Fatal("caller mutation changed sibling files or Git state")
		}
	})
}
