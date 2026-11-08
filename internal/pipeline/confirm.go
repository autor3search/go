package pipeline

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/autor3search/go/internal/bench"
	"github.com/autor3search/go/internal/verdict"
)

// confirmGuardTrip measures the benchmarks that tripped the regression guard a
// second time and decides again with the new numbers for those rows.
//
// The guard uses the raw alpha on every benchmark, so with many benchmarks a
// noisy machine trips it on code that the change does not touch. A real
// slowdown repeats, and still discards. A row that the second run did not
// measure keeps its first numbers, so a trip is never lost by accident.
func confirmGuardTrip(in verdict.Input, first verdict.Result, remeasure func(names []string) ([]bench.Delta, error)) (verdict.Result, []bench.Delta, error) {
	tripped := make(map[string]bool, len(first.Regressions))
	names := make([]string, 0, len(first.Regressions))
	for _, d := range first.Regressions {
		tripped[d.Name] = true
		names = append(names, d.Name)
	}
	again, err := remeasure(names)
	if err != nil {
		return verdict.Result{}, nil, err
	}
	second := make(map[string]bench.Delta, len(again))
	for _, d := range again {
		second[d.Name] = d
	}

	deltas := append([]bench.Delta(nil), in.Deltas...)
	var notes []string
	for i, d := range deltas {
		if !tripped[d.Name] {
			continue
		}
		n, ok := second[d.Name]
		if !ok {
			notes = append(notes, fmt.Sprintf("%s: guard trip kept, the second measurement has no result for it", d.Name))
			continue
		}
		deltas[i] = n
		if n.Significant && n.PctChange > in.MaxRegressPct {
			notes = append(notes, fmt.Sprintf("%s: guard trip confirmed by a second measurement (%+.1f%%, p=%.4f)", d.Name, n.PctChange, n.P))
		} else {
			notes = append(notes, fmt.Sprintf("%s: guard trip not confirmed by a second measurement (first %+.1f%%, p=%.4f; second %+.1f%%, p=%.4f), the second is used",
				d.Name, d.PctChange, d.P, n.PctChange, n.P))
		}
	}

	score, err := bench.GeoMean(deltas)
	if err != nil {
		return verdict.Result{}, nil, err
	}
	in.Deltas, in.Score = deltas, score
	res := verdict.Decide(in)
	res.Warnings = append(res.Warnings, notes...)
	return res, deltas, nil
}

// guardPattern builds a -bench regexp that runs the named benchmarks. go test
// splits the pattern at each "/" and matches one level of the name with each
// part, so the pattern has one alternation for each level. It can also run
// other combinations of these levels; the caller uses only the named rows.
// A name can end in a GOMAXPROCS suffix such as "-8", which go test does not
// match, so each level also accepts the part without a "-N" suffix.
func guardPattern(names []string) string {
	var levels []map[string]bool
	add := func(level int, s string) {
		for len(levels) <= level {
			levels = append(levels, map[string]bool{})
		}
		levels[level][regexp.QuoteMeta(s)] = true
	}
	procs := regexp.MustCompile(`-[0-9]+$`)
	for _, name := range names {
		// A name that two packages share has the form Name@pkg, and the
		// package path can hold "/". See bench.Parse.
		if at := strings.IndexByte(name, '@'); at >= 0 {
			name = name[:at]
		}
		for i, p := range strings.Split(name, "/") {
			add(i, p)
			add(i, procs.ReplaceAllString(p, ""))
		}
	}
	out := make([]string, len(levels))
	for i, set := range levels {
		alts := make([]string, 0, len(set))
		for a := range set {
			alts = append(alts, a)
		}
		slices.Sort(alts)
		out[i] = "^(" + strings.Join(alts, "|") + ")$"
	}
	return strings.Join(out, "/")
}
