package main

import (
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/worklistadapter"
	"github.com/Beamfall/corvint/internal/workqueue"
)

type worklist = worklistadapter.Worklist

type workItem = worklistadapter.Item

type producerScratch struct{ parent, root, commit string }

func main() {
	streams := &clierror.Streams{Binary: "corvint-work-queue", Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-work-queue", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}

	if code, handled := serveHelp(os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}
	if err := runProducer(os.Args[1:]); err != nil {
		fmt.Fprintln(streams.Stderr(), err)
		os.Exit(streams.Status(2))
	}
}

func runProducer(arguments []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	directory, err := os.Getwd()
	if err != nil {
		return err
	}
	return worklistadapter.Run(ctx, "corvint-work-queue", directory, arguments, os.Stdout)
}

func producerOwnedScratch() (producerScratch, error) {
	scratch, err := worklistadapter.OwnedScratch()
	return producerScratch{scratch.Parent, scratch.Root, scratch.Commit}, err
}

func documents(root string, policy *workqueue.Policy) (*workqueue.Snapshot, *workqueue.DetailsDocument, *workqueue.CheckpointDocument, error) {
	return worklistadapter.Documents(root, policy)
}
