// Command ci-shards emits one complete-universe shard of typed Go packages.
package main

import (
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
	order := flag.String("order", "", "advisory affected-plan/0 file; its selected packages are emitted first")
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
	if err == nil {
		packages, err = cishards.Intersect(packages, []string{"./..."}, *shard, *total)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
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
