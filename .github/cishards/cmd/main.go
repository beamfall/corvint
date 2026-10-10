// Command ci-shards emits one complete-universe shard of typed Go packages, or
// with --slices the test slices of split packages in that shard (AFP-V0-041).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Beamfall/corvint/.github/cishards"
	"io"
	"os"
)

func main() {
	shard := flag.Int("shard", 0, "zero-based shard index")
	total := flag.Int("shards", 4, "complete partition shard count")
	profile := flag.Bool("profile", false, "print protected partition digest")
	order := flag.String("order", "", "advisory affected-plan/1 (or /0) file; its selected packages are emitted first")
	share := flag.String("share", "", "advisory affected-plan/1 (or /0) file; print its selected share of the universe's estimated time")
	slices := flag.Bool("slices", false, "print the shard's test slices (AFP-V0-041), one invocation per line: FLAG PATTERN PACKAGE")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(2)
	}
	if *profile {
		fmt.Println(cishards.ProfileDigest())
		return
	}
	b, err := io.ReadAll(io.LimitReader(os.Stdin, cishards.MaxInputBytes+1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	packages, err := cishards.Packages(b)
	if err == nil && *share != "" {
		os.Exit(shared(packages, *share))
	}
	var plan []cishards.Shard
	if err == nil {
		plan, err = cishards.Plan(packages, *total)
	}
	if err == nil && (*shard < 0 || *shard >= *total) {
		err = fmt.Errorf("invalid shard index")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *slices {
		for _, s := range plan[*shard].Slices {
			fmt.Println(s.Flag, s.Pattern, s.Package)
		}
		return
	}
	packages = plan[*shard].Packages
	if *order != "" {
		packages = ordered(packages, *order)
	}
	for _, p := range packages {
		fmt.Println(p)
	}
}

// ordered only permutes the shard. Any plan it cannot use keeps the current order.
func ordered(packages []string, path string) []string {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ci-shards: affected-first order unavailable; keeping the current order")
		return packages
	}
	defer f.Close()
	plan, err := io.ReadAll(io.LimitReader(f, cishards.MaxPlanBytes+1))
	out, ok := cishards.Order(packages, plan)
	if err != nil || !ok {
		fmt.Fprintln(os.Stderr, "ci-shards: affected-first order unreadable; keeping the current order")
		return packages
	}
	fmt.Fprintln(os.Stderr, "ci-shards: affected-first order applied")
	return out
}

// shared prints the AFP-V0-025 shadow metric for the whole universe on stdin.
func shared(packages []string, path string) int {
	plan, err := os.ReadFile(path)
	report, ok := cishards.ShareOf(packages, plan)
	if err != nil || !ok {
		fmt.Fprintln(os.Stderr, "ci-shards: selected share unavailable")
		return 1
	}
	if json.NewEncoder(os.Stdout).Encode(report) != nil {
		return 1
	}
	return 0
}
