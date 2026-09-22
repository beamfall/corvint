package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/procgroup"
)

func main() {
	setProcessUmask()
	if len(os.Args) > 1 && os.Args[1] == "--owned-fixture" {
		os.Exit(ownedFixture(os.Args[1:]))
	}
	manifestPath := flag.String("manifest", "", "frozen synthetic descriptor")
	fixtureDir := flag.String("fixture-dir", "", "write an owned synthetic pilot fixture bundle")
	flag.Parse()
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	binary, err := os.ReadFile(self)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *fixtureDir != "" {
		if err = writeFixtureBundle(*fixtureDir, binary); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}
	registry := ownedRegistry(binary)
	p, err := loadManifest(*manifestPath, registry)
	result := newReport()
	if err != nil {
		result.Cases = append(result.Cases, caseResult{ID: "descriptor", Adjudication: procgroup.Adjudicate(6, procgroup.RefusalDescriptorInvalid, nil), Inconclusive: statePointer(procgroup.InconclusiveSetup), Records: []record{}, Effects: []observedEffect{}, OverflowStreams: []string{}, AcceptedBy: "NOT_PRODUCED", Detail: err.Error()})
	} else {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		result = replay(ctx, p)
	}
	if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, c := range result.Cases {
		if c.Status != procgroup.StatusPass {
			os.Exit(1)
		}
	}
}
