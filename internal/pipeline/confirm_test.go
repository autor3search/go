package pipeline

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/autor3search/go/internal/bench"
	"github.com/autor3search/go/internal/verdict"
)

func delta(name string, pct, p float64) bench.Delta {
	return bench.Delta{
		Name: name, Unit: bench.UnitTime, BaseCenter: 100, CandCenter: 100 * (1 + pct/100),
		Ratio: 1 + pct/100, PctChange: pct, P: p, Alpha: 0.05, Significant: p < 0.05, NBase: 8, NCand: 8,
	}
}

func guardInput(deltas ...bench.Delta) verdict.Input {
	score, _ := bench.GeoMean(deltas)
	return verdict.Input{Deltas: deltas, Score: score, MaxRegressPct: 5, MinEffectPct: 1, MinLocalEffectPct: 10}
}

func TestConfirmGuardTripUsesSecondMeasurementWhenTripDoesNotRepeat(t *testing.T) {
	in := guardInput(
		delta("BenchmarkFast", -30, 0.0001),
		delta("BenchmarkUntouched/a", 8, 0.01),
		delta("BenchmarkOther", 0.2, 0.6),
	)
	first := verdict.Decide(in)
	if first.Reason != verdict.ReasonGuardRegression {
		t.Fatalf("setup: first verdict %s, want guard_regression", first.Reason)
	}
	var asked []string
	again := func(names []string) ([]bench.Delta, error) {
		asked = names
		return []bench.Delta{delta("BenchmarkUntouched/a", 0.4, 0.7)}, nil
	}
	res, deltas, err := confirmGuardTrip(in, first, again)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(asked, ",") != "BenchmarkUntouched/a" {
		t.Fatalf("re-measured %v, want only the tripped benchmark", asked)
	}
	if res.Status != verdict.StatusKeep {
		t.Fatalf("status %s (%s), want KEEP once the trip does not repeat", res.Status, res.Message)
	}
	if deltas[1].PctChange != 0.4 || deltas[0].PctChange != -30 {
		t.Fatalf("deltas %+v: want the second measurement for the tripped row only", deltas)
	}
	if !hasWarning(res, "not confirmed") {
		t.Fatalf("warnings %q: want a note that the trip was not confirmed", res.Warnings)
	}
}

func TestConfirmGuardTripDiscardsWhenTripRepeats(t *testing.T) {
	in := guardInput(delta("BenchmarkFast", -30, 0.0001), delta("BenchmarkSlow", 9, 0.002))
	first := verdict.Decide(in)
	again := func(names []string) ([]bench.Delta, error) {
		return []bench.Delta{delta("BenchmarkSlow", 7.5, 0.001)}, nil
	}
	res, _, err := confirmGuardTrip(in, first, again)
	if err != nil {
		t.Fatal(err)
	}
	if res.Reason != verdict.ReasonGuardRegression {
		t.Fatalf("reason %s, want guard_regression when the slowdown repeats", res.Reason)
	}
	if len(res.Regressions) != 1 || res.Regressions[0].PctChange != 7.5 {
		t.Fatalf("regressions %+v: want the confirmed second measurement", res.Regressions)
	}
	if !hasWarning(res, "confirmed") {
		t.Fatalf("warnings %q: want a note that the trip was confirmed", res.Warnings)
	}
}

func TestConfirmGuardTripKeepsFirstTripWhenRowIsMissing(t *testing.T) {
	in := guardInput(delta("BenchmarkFast", -30, 0.0001), delta("BenchmarkSlow", 9, 0.002))
	first := verdict.Decide(in)
	again := func(names []string) ([]bench.Delta, error) { return nil, nil }
	res, _, err := confirmGuardTrip(in, first, again)
	if err != nil {
		t.Fatal(err)
	}
	if res.Reason != verdict.ReasonGuardRegression {
		t.Fatalf("reason %s: a row that the second run did not measure must keep its first trip", res.Reason)
	}
}

func TestConfirmGuardTripReturnsRemeasureError(t *testing.T) {
	in := guardInput(delta("BenchmarkSlow", 9, 0.002))
	first := verdict.Decide(in)
	boom := errors.New("boom")
	if _, _, err := confirmGuardTrip(in, first, func([]string) ([]bench.Delta, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatalf("err %v, want the re-measure error", err)
	}
}

func TestGuardPatternMatchesOnlyTheTrippedLevels(t *testing.T) {
	pat := guardPattern([]string{"BenchmarkA/x-y", "BenchmarkB/sub.1/deep-8", "BenchmarkC/p@example.com/mod/pkg", "BenchmarkD"})
	levels := strings.Split(pat, "/")
	if len(levels) != 3 {
		t.Fatalf("pattern %q: want 3 levels", pat)
	}
	match := func(level int, s string) bool { return regexp.MustCompile(levels[level]).MatchString(s) }
	for _, s := range []string{"BenchmarkA", "BenchmarkB", "BenchmarkC", "BenchmarkD"} {
		if !match(0, s) {
			t.Errorf("level 0 of %q does not match %q", pat, s)
		}
	}
	if match(0, "BenchmarkAB") || match(0, "BenchmarkE") {
		t.Errorf("level 0 of %q matches an untripped benchmark", pat)
	}
	if !match(1, "x-y") || !match(1, "sub.1") || !match(1, "p") || match(1, "subX1") {
		t.Errorf("level 1 of %q: want literal names only", pat)
	}
	if !match(2, "deep") || !match(2, "deep-8") {
		t.Errorf("level 2 of %q: want the name with and without the GOMAXPROCS suffix", pat)
	}
}

func hasWarning(r verdict.Result, sub string) bool {
	for _, w := range r.Warnings {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}
