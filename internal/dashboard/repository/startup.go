package repository

import (
	"os"
	"os/exec"
	"path/filepath"
)

type startupState struct {
	directory string
	gitPath   string
	gitErr    bool
	bootstrap []string
	alternate string
	objectDir string
}

var processStartup = captureStartup()

func captureStartup() startupState {
	directory, directoryErr := os.Getwd()
	gitPath, gitErr := exec.LookPath("git")
	state := startupState{
		directory: directory,
		gitPath:   gitPath,
		gitErr:    gitErr != nil || directoryErr != nil,
		alternate: os.Getenv("GIT_ALTERNATE_OBJECT_DIRECTORIES"),
		objectDir: os.Getenv("GIT_OBJECT_DIRECTORY"),
	}
	for _, name := range []string{"SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, present := os.LookupEnv(name); present {
			state.bootstrap = append(state.bootstrap, name+"="+value)
		}
	}
	return state
}

func (state startupState) resolvedExecutable() (string, *Failure) {
	if state.gitErr || state.gitPath == "" {
		return "", unavailable(ReasonUnavailable)
	}
	candidate := state.gitPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(state.directory, candidate)
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil || !filepath.IsAbs(resolved) || filepath.Clean(resolved) != resolved {
		return "", unavailable(ReasonUnavailable)
	}
	return resolved, nil
}

func (state startupState) childEnvironment() []string {
	environment := append([]string(nil), state.bootstrap...)
	return append(environment,
		"LANG=C",
		"LC_ALL=C",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1",
		"GIT_GRAFT_FILE="+os.DevNull,
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=",
		"GIT_PROTOCOL_FROM_USER=0",
		"GCM_INTERACTIVE=never",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
	)
}

func unavailable(reason FailureReason) *Failure {
	return &Failure{Code: FailureRepositoryUnavailable, Reason: reason}
}

func changed() *Failure {
	return &Failure{Code: FailureRepositoryChanged, Reason: ReasonChanged}
}
