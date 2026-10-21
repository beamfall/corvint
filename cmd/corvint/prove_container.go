package main

import (
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/rbmutate"
)

// maxMutateRunnerBytes bounds one `--mutate-runner` declaration file.
const maxMutateRunnerBytes = 64 << 10

// withoutMutateRunnerFlag lifts `--mutate-runner FILE` (or
// `--mutate-runner=FILE`) out of the arguments after the command word and
// reads its corvint-rbmutate-container/0 declaration (FPK-V0-078). A token
// after `--` is positional and is never lifted.
func withoutMutateRunnerFlag(arguments []string) ([]string, *rbmutate.ContainerProfile, error) {
	kept := make([]string, 0, len(arguments))
	var files []string
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--" {
			kept = append(kept, arguments[index:]...)
			break
		}
		name, value, inline := strings.Cut(argument, "=")
		if name != "--mutate-runner" {
			kept = append(kept, argument)
			continue
		}
		if !inline {
			if index+1 >= len(arguments) {
				return nil, nil, argumentError("argument --mutate-runner: expected one argument")
			}
			index++
			value = arguments[index]
		}
		files = append(files, value)
	}
	switch len(files) {
	case 0:
		return kept, nil, nil
	case 1:
	default:
		return nil, nil, argumentError("argument --mutate-runner: may not be repeated")
	}
	data, err := readBoundedFile(files[0], maxMutateRunnerBytes)
	if err != nil {
		return nil, nil, argumentError("argument --mutate-runner: cannot read a bounded regular file at " + files[0])
	}
	profile, err := rbmutate.ParseContainerProfile(data)
	if err != nil {
		return nil, nil, argumentError("argument --mutate-runner: " + strings.TrimPrefix(err.Error(), "rbmutate: "))
	}
	return kept, profile, nil
}

// checkMutateRunner refuses a declaration without --mutate, which alone
// runs Ruby mutation rows.
func checkMutateRunner(parsed options) error {
	if parsed.prove.rubyRunner != nil && !parsed.proveMutate {
		return argumentError("argument --mutate-runner: requires --mutate")
	}
	return nil
}
