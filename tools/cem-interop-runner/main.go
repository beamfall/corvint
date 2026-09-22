// cem-interop-runner is the native, Corvint-free CEM 0.1 external consumer harness.
package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func startObservation(ctx context.Context, path string) (map[string]any, error) {
	var result map[string]any
	err := withObservationLock(path, lockTimeoutSeconds*time.Second, func() error {
		if _, err := os.Lstat(path); err == nil {
			return fail("observation-exists")
		} else if !os.IsNotExist(err) {
			return fail("observation-unreadable")
		}
		d, err := doctor(ctx)
		if err != nil {
			return err
		}
		result = map[string]any{"doctor": d, "lane": "consumer", "profile": "cem-external-interop-observation/1", "publication": "COMMITTED", "runs": []any{}, "state": "STARTED"}
		return atomicWrite(path, result, true, nil)
	})
	return result, err
}
func consume(ctx context.Context, path, implementationPath string, retry bool) (map[string]any, error) {
	var result map[string]any
	err := withObservationLock(path, lockTimeoutSeconds*time.Second, func() error {
		observation, digest, err := loadObservation(path)
		if err != nil {
			return err
		}
		runs := observation["runs"].([]any)
		if retry && len(runs) == 0 {
			return fail("retry-without-first-run")
		}
		if !retry && len(runs) > 0 {
			return fail("retry-required")
		}
		if len(runs) >= maxRuns {
			return fail("run-limit")
		}
		run, err := consumerRun(ctx, implementationPath)
		if err != nil {
			return err
		}
		doctor := observation["doctor"].(map[string]any)
		if doctor["manifestSha256"] != run["manifestSha256"] || doctor["packetSha256"] != run["packetSha256"] {
			return fail("observation-kit-mismatch")
		}
		run["sequence"] = len(runs) + 1
		runs = append(runs, run)
		observation["runs"] = runs
		observation["state"] = run["state"]
		if err = atomicWrite(path, observation, false, &digest); err != nil {
			return err
		}
		result = observation
		return nil
	})
	return result, err
}
func summary(observation map[string]any) map[string]any {
	runs := observation["runs"].([]any)
	var latest any
	if len(runs) > 0 {
		r := runs[len(runs)-1].(map[string]any)
		latest = map[string]any{"counts": r["counts"], "durationNs": r["durationNs"], "sequence": r["sequence"], "state": r["state"]}
	}
	doctor := observation["doctor"].(map[string]any)
	state := observation["state"].(string)
	return map[string]any{"corvintCommit": doctor["corvintCommit"], "corvintTreeState": doctor["corvintTreeState"], "doctorState": doctor["state"], "latestRun": latest, "ok": state == "STARTED" || state == "PASS", "profile": observation["profile"], "packetSha256": doctor["packetSha256"], "publication": observation["publication"], "runCount": len(runs), "state": state}
}
func emit(v map[string]any, stream *os.File) error {
	raw, err := canonical(v)
	if err != nil {
		return err
	}
	if len(raw) > captureLimit {
		return fail("output-too-large")
	}
	_, err = stream.Write(append(raw, '\n'))
	return err
}

func runCLI(ctx context.Context, args []string) int {
	var value map[string]any
	var err error
	switch {
	case len(args) == 1 && args[0] == "doctor":
		var d map[string]any
		d, err = doctor(ctx)
		if err == nil {
			value = map[string]any{"ok": true, "profile": "cem-interop-doctor/0"}
			for k, v := range d {
				value[k] = v
			}
		}
	case len(args) == 5 && args[0] == "start" && args[1] == "--lane" && args[2] == "consumer" && args[3] == "--output":
		var d map[string]any
		d, err = startObservation(ctx, args[4])
		if err == nil {
			value = summary(d)
		}
	case (len(args) == 5 || len(args) == 6) && args[0] == "consumer":
		options := map[string]string{}
		retry := false
		for i := 1; i < len(args); {
			if args[i] == "--retry" {
				retry = true
				i++
				continue
			}
			if i+1 >= len(args) {
				err = fail("arguments")
				break
			}
			options[args[i]] = args[i+1]
			i += 2
		}
		if err == nil {
			implementationPath, ok1 := options["--implementation"]
			observation, ok2 := options["--observation"]
			if !ok1 || !ok2 || len(options) != 2 {
				err = fail("arguments")
			} else {
				var d map[string]any
				d, err = consume(ctx, observation, implementationPath, retry)
				if err == nil {
					value = summary(d)
					if d["state"] != "PASS" {
						_ = emit(value, os.Stdout)
						return 1
					}
				}
			}
		}
	case len(args) == 3 && args[0] == "inspect" && args[1] == "--observation":
		var d map[string]any
		d, _, err = loadObservation(args[2])
		if err == nil {
			value = summary(d)
		}
	default:
		err = fail("arguments")
	}
	if err != nil {
		code := "internal"
		if e, ok := err.(runnerError); ok {
			code = string(e)
		}
		_ = emit(map[string]any{"error": map[string]any{"code": code}, "ok": false}, os.Stderr)
		if code == "interrupted" {
			return 130
		}
		return 2
	}
	if err = emit(value, os.Stdout); err != nil {
		_ = emit(map[string]any{"error": map[string]any{"code": err.Error()}, "ok": false}, os.Stderr)
		return 2
	}
	return 0
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	code := runCLI(ctx, os.Args[1:])
	os.Exit(code)
}

func init() {
	if value := os.Getenv("CORVINT_CEM_KIT_ROOT"); value != "" {
		if absolute, err := filepath.Abs(value); err == nil {
			kitRoot = absolute
		}
	}
}
