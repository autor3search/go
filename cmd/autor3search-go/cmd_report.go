package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/autor3search/go/internal/gitx"
	"github.com/autor3search/go/internal/results"
)

// runReport summarizes results.tsv.
func runReport(args []string) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("C", ".", "repository root (or a directory inside it)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	root, err := gitx.Root(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "autor3search-go report: %s is not inside a git repository: %v\n", *dir, err)
		return exitUsage
	}

	resultsPath := filepath.Join(root, results.Path)
	rows, err := results.Load(resultsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "autor3search-go report: %v\n", err)
		return exitUsage
	}

	slowdownsPath := filepath.Join(root, results.SlowdownsPath)
	slowdowns, slowdownsLogged, err := results.LoadSlowdowns(slowdownsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "autor3search-go report: %v\n", err)
		return exitUsage
	}

	printReportSummary(rows, slowdowns, slowdownsLogged)
	return exitOK
}

// printReportSummary prints a summary of the results. Slowdowns come before the
// wins, because a reader who stops after the first screen must not leave with
// only the wins.
func printReportSummary(rows []results.Row, slowdowns []results.Slowdown, slowdownsLogged bool) {
	if len(rows) == 0 {
		fmt.Println("autor3search-go report: no experiments recorded")
		return
	}

	// Count by status.
	var kept, fixes, discarded, failed, crashed int
	for _, r := range rows {
		switch r.Status {
		case "keep":
			kept++
		case "fix":
			fixes++
		case "discard":
			discarded++
		case "fail":
			failed++
		case "crash":
			crashed++
		}
	}

	// Cumulative improvement is the PRODUCT of every kept score, not the
	// latest kept score alone.
	//
	// This depends entirely on the measurement baseline advancing after
	// every KEEP (see internal/pipeline's advanceMeasurementBaseline and
	// state.Baseline.MeasureCommit): each `eval` measures the candidate
	// against the immediately preceding ACCEPTED state, not against the
	// run's original commit. That means every kept row's score already
	// reflects ONLY that experiment's own incremental contribution — it is
	// NOT cumulative on its own — so the run-level speedup has to be
	// composed by multiplying every kept score together, the same way
	// compounding successive percentage changes works.
	//
	// If the measurement baseline ever stops advancing (reverting to a
	// single fixed baseline for the whole run), this reasoning inverts:
	// every kept score would once again already be cumulative on its own,
	// and multiplying them would double-count prior gains. The two must
	// always change together.
	//
	// A correctness fix also advances the baseline, so its score counts too.
	// Leaving it out would hide what the fix cost.
	cumulativeScore := 1.0
	haveKept := false
	for _, r := range rows {
		if r.Status == "keep" || r.Status == "fix" {
			cumulativeScore *= r.Score
			haveKept = true
		}
	}

	// Largest wins: every kept row, sorted by its own best-benchmark delta.
	// Under the advancing baseline, verdict.Decide only ever returns KEEP
	// for a candidate that is itself a real, significant improvement over
	// the immediately preceding accepted state (see internal/verdict), so
	// unlike under a fixed baseline there is no "re-measurement of an
	// unchanged tree" case to filter out here: every kept row is already a
	// genuine win in its own right.
	type win struct {
		row results.Row
		idx int
	}
	var keptRows []win
	for i, r := range rows {
		if r.Status == "keep" {
			keptRows = append(keptRows, win{r, i})
		}
	}
	sort.Slice(keptRows, func(i, j int) bool {
		return keptRows[i].row.BestBenchDelta < keptRows[j].row.BestBenchDelta
	})

	// Print summary.
	fmt.Printf("autor3search-go report: %d total experiments\n", len(rows))
	fmt.Printf("  kept: %d\n", kept)
	if fixes > 0 {
		fmt.Printf("  correctness fixes: %d\n", fixes)
	}
	fmt.Printf("  discarded: %d\n", discarded)
	fmt.Printf("  failed: %d\n", failed)
	fmt.Printf("  crashed: %d\n", crashed)
	if haveKept {
		printSlowdowns(rows, slowdowns, slowdownsLogged)
		improvement := (1.0 - cumulativeScore) * 100.0
		fmt.Printf("\ncumulative speedup: %.1f%% (geomean of the declared benchmarks only)\n", improvement)
	} else {
		fmt.Printf("\ncumulative speedup: no experiments kept\n")
	}

	if fixes > 0 {
		fmt.Println("\ncorrectness fixes (not wins):")
		for _, r := range rows {
			if r.Status == "fix" {
				fmt.Printf("  - %s (score: %.4f)\n", r.Description, r.Score)
			}
		}
	}

	if len(keptRows) > 0 {
		fmt.Println("\nlargest wins:")
		if fixes > 0 {
			fmt.Println("  (a later fix can undo part of a win listed here)")
		}
		maxWins := 5
		if len(keptRows) < maxWins {
			maxWins = len(keptRows)
		}
		for i := 0; i < maxWins; i++ {
			w := keptRows[i]
			fmt.Printf("  %d. %s (time: %.1f%%, allocs: %.1f%%)\n",
				i+1, w.row.Description, w.row.BestBenchDelta, w.row.AllocsDelta)
		}
	}
}

// printSlowdowns lists every significant slowdown in the experiments logged as
// kept, largest first, and then what the run cannot show.
func printSlowdowns(rows []results.Row, slowdowns []results.Slowdown, logged bool) {
	onBranch := map[string]string{}
	for _, r := range rows {
		if r.Status == "keep" || r.Status == "fix" {
			onBranch[r.Commit] = r.Description
		}
	}
	var shown []results.Slowdown
	for _, s := range slowdowns {
		if _, ok := onBranch[s.Commit]; ok {
			shown = append(shown, s)
		}
	}
	sort.SliceStable(shown, func(i, j int) bool { return shown[i].Pct > shown[j].Pct })

	switch {
	case !logged:
		fmt.Println("\nslowdowns: not recorded for this run")
	case len(shown) == 0:
		fmt.Println("\nslowdowns: none significant in any single kept experiment")
	default:
		fmt.Printf("\nslowdowns in kept experiments: %d (each at raw p < alpha, so some can be noise)\n", len(shown))
		for _, s := range shown {
			fmt.Printf("  %+6.1f%%  %s  p=%.4f  in %s (%s)\n", s.Pct, s.Benchmark, s.P, s.Commit, onBranch[s.Commit])
		}
	}

	fmt.Println("\nlimits of this report:")
	fmt.Println("  - It measures only the declared benchmarks. Other inputs can be slower.")
	fmt.Println("  - Small slowdowns can build up across kept experiments. Run `autor3search-go compare` to measure that.")
	fmt.Println("  - A KEEP does not prove correctness. It proves only that the frozen tests pass.")
	fmt.Println("  - Compare the final code with the baseline commit on your own inputs before you merge.")
}
