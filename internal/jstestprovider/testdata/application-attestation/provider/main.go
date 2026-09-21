package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

func main() {
	docker := flag.String("docker", "docker", "docker executable")
	git := flag.String("git", "git", "git executable")
	repository := flag.String("repository", "", "application repository")
	compose := flag.String("compose", "", "compose file")
	container := flag.String("container", "", "container identity")
	flag.Parse()
	if *repository == "" || *compose == "" || *container == "" {
		os.Exit(2)
	}
	if _, err := os.ReadFile("/dev/stdin"); err != nil {
		os.Exit(3)
	}
	root := one(*git, "-C", *repository, "rev-list", "--max-parents=0", "HEAD")
	revision := one(*git, "-C", *repository, "rev-parse", "HEAD")
	tree := one(*git, "-C", *repository, "rev-parse", "HEAD^{tree}")
	status := raw(*git, "-C", *repository, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	inspectRaw := raw(*docker, "inspect", *container)
	var inspect []struct {
		ID    string `json:"Id"`
		Image string `json:"Image"`
		State struct {
			Running   bool   `json:"Running"`
			StartedAt string `json:"StartedAt"`
			Health    struct {
				Status string `json:"Status"`
			} `json:"Health"`
		} `json:"State"`
	}
	if json.Unmarshal(inspectRaw, &inspect) != nil || len(inspect) != 1 {
		os.Exit(4)
	}
	composeConfig, err := os.ReadFile(*compose)
	if err != nil {
		os.Exit(5)
	}
	dirtyState := "clean"
	dirtyDigest := ""
	if len(status) != 0 {
		dirtyState = "dirty"
		dirtyDigest = digest(status)
	}
	health := "unhealthy"
	if inspect[0].State.Running && inspect[0].State.Health.Status == "healthy" {
		health = "healthy"
	}
	attestation := jstestprovider.ApplicationAttestation{
		Profile:       jstestprovider.ApplicationAttestationProfile,
		Repository:    jstestprovider.ApplicationRepositoryIdentity{RootCommit: root, Revision: revision, Tree: tree, DirtyState: dirtyState, DirtyDigest: dirtyDigest},
		Build:         jstestprovider.ApplicationArtifactIdentity{Kind: "image", Digest: inspect[0].Image},
		Configuration: jstestprovider.ApplicationArtifactIdentity{Kind: "compose", Digest: digest(composeConfig)},
		Instance:      jstestprovider.ApplicationInstanceIdentity{Kind: "docker-container", ID: inspect[0].ID, StartGeneration: inspect[0].State.StartedAt},
		Health:        jstestprovider.ApplicationHealth{State: health},
	}
	data, err := json.Marshal(attestation)
	if err != nil {
		os.Exit(5)
	}
	_, _ = os.Stdout.Write(append(data, '\n'))
}

func raw(command string, arguments ...string) []byte {
	output, err := exec.Command(command, arguments...).Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(10)
	}
	return output
}

func one(command string, arguments ...string) string {
	return strings.TrimSpace(string(raw(command, arguments...)))
}

func digest(data []byte) string {
	value := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(value[:])
}
