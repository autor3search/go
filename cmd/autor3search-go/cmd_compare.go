package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/autor3search/go/internal/bench"
	"github.com/autor3search/go/internal/gitx"
	"github.com/autor3search/go/internal/pipeline"
	"github.com/autor3search/go/internal/state"
)

// runCompare measures the final code against the commit the run started from.
//
// Each eval measures one step against the last KEEP, so a slowdown too small to
// see in any one step can add up over a run. Only a direct comparison shows it.
// It decides nothing and records nothing: the exit code says whether the
// measurement ran, not whether the result is good.
func runCompare(args []string) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("C", ".", "repository root (or a directory inside it)")
	count := fs.Int("count", 0, "rounds per side, instead of count in the config; more rounds find smaller slowdowns")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if *count != 0 && *count < 2 {
		fmt.Fprintln(os.Stderr, "autor3search-go compare: -count must be at least 2")
		return exitUsage
	}

	ref, code := resolveRunRef("compare", *dir, "")
	if code != exitOK {
		return code
	}
	root, stateDir := ref.Root, ref.StateDir
	cfg, _, code := loadConfig("compare", root)
	if code != exitOK {
		return code
	}
	if *count != 0 {
		cfg.Count = *count
	}
	base, err := state.LoadBaseline(filepath.Join(stateDir, state.BaselineFile))
	if err != nil {
		fmt.Fprintf(os.Stderr, "autor3search-go compare: %v\n", err)
		return exitUsage
	}

	clean, err := gitx.IsClean(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "autor3search-go compare: %v\n", err)
		return exitUsage
	}
	if !clean {
		fmt.Fprintln(os.Stderr, "autor3search-go compare: the tree has uncommitted changes, so the result would not describe a commit")
		return exitUsage
	}
	head, err := gitx.HeadCommit(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "autor3search-go compare: %v\n", err)
		return exitUsage
	}
	if head == base.Commit {
		fmt.Fprintf(os.Stderr, "autor3search-go compare: HEAD is still the baseline commit %s, so there is nothing to compare\n", head)
		return exitUsage
	}

	// The same claim as eval: both use the machine and the repository, and a
	// benchmark that runs beside another one measures nothing useful.
	release, err := state.ClaimEval(stateDir, os.Getpid())
	if err != nil {
		fmt.Fprintf(os.Stderr, "autor3search-go compare: %v\n", err)
		return exitUsage
	}
	defer release()

	logPath := filepath.Join(root, pipeline.RunLogName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "autor3search-go compare: open %s: %v\n", logPath, err)
		return exitUsage
	}
	defer logFile.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	meas, err := pipeline.Compare(ctx, pipeline.Options{Root: root, StateDir: stateDir, Cfg: cfg, Base: base, Log: logFile})
	if err != nil {
		fmt.Fprintf(os.Stderr, "autor3search-go compare: %v\n", err)
		return exitUsage
	}

	fmt.Printf("%s (run start) vs %s (HEAD), %d rounds per side, benchtime %s\n", base.Commit, head, cfg.Count, cfg.Benchtime)
	return printComparison(meas)
}

// printComparison prints the slowdowns first, then the rest. It marks the
// slowdowns that pass the correction for the number of benchmarks: with many
// benchmarks, some of the others are expected to be noise.
func printComparison(meas *pipeline.Measurements) int {
	score, err := bench.GeoMean(meas.Time)
	if err != nil {
		fmt.Fprintf(os.Stderr, "autor3search-go compare: %v\n", err)
		return exitUsage
	}
	k := float64(len(meas.Time))

	var slower, faster []bench.Delta
	for _, d := range meas.Time {
		switch {
		case d.Significant && d.PctChange > 0:
			slower = append(slower, d)
		case d.Significant && d.PctChange < 0:
			faster = append(faster, d)
		}
	}
	sort.SliceStable(slower, func(i, j int) bool { return slower[i].PctChange > slower[j].PctChange })
	sort.SliceStable(faster, func(i, j int) bool { return faster[i].PctChange < faster[j].PctChange })

	fmt.Printf("\nslower: %d of %d benchmarks", len(slower), len(meas.Time))
	if len(slower) > 0 {
		fmt.Print(" (* passes the correction for the number of benchmarks; the others can be noise)")
	}
	fmt.Println()
	for _, d := range slower {
		mark := " "
		if d.P < d.Alpha/k {
			mark = "*"
		}
		fmt.Printf(" %s %+7.2f%%  %s  p=%.4f\n", mark, d.PctChange, d.Name, d.P)
	}
	for _, a := range meas.AllocsAppeared {
		fmt.Printf("   allocs/op went from 0 to %.0f  %s\n", a.CandCenter, a.Name)
	}

	fmt.Printf("\nfaster: %d of %d benchmarks\n", len(faster), len(meas.Time))
	for i, d := range faster {
		if i == 5 {
			fmt.Printf("   ... and %d more\n", len(faster)-i)
			break
		}
		fmt.Printf("   %+7.2f%%  %s  p=%.4f\n", d.PctChange, d.Name, d.P)
	}

	fmt.Printf("\ngeomean: %.4f (%+.2f%%)\n", score, (score-1)*100)
	fmt.Println("\nThis measures only the declared benchmarks. Other inputs can be slower.")
	fmt.Println("Small slowdowns can differ from run to run. Use -count to measure with more rounds.")
	return exitOK
}
