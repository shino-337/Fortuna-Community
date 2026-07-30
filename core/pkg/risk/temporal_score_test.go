package risk

import (
	"math"
	"testing"
)

func TestComputeTemporalScore_NoiseSingleEvent(t *testing.T) {
	base := 40.0
	score, detail := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m:      1,
		TrendDelta:         0,
		PersistenceMinutes: 0,
	})
	if score != base {
		t.Fatalf("noise case should keep base score: got %.2f want %.2f", score, base)
	}
	if detail.Burst != 0 {
		t.Fatalf("expected burst 0, got %.3f", detail.Burst)
	}
}

func TestComputeTemporalScore_NoSignals_WithHistory(t *testing.T) {
	base := 50.0
	prev := 50.0
	score, detail := ComputeTemporalScore(base, &prev, TemporalSignals{})
	if math.Abs(score-base) > 0.01 {
		t.Fatalf("no-signal with flat history should stay stable: got %.2f want %.2f", score, base)
	}
	if detail.Multiplier != 1.0 {
		t.Fatalf("expected multiplier 1.0, got %.3f", detail.Multiplier)
	}
}

func TestComputeTemporalScore_BurstDetection(t *testing.T) {
	base := 50.0
	score, detail := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m:      50,
		TrendDelta:         0,
		PersistenceMinutes: 0,
	})
	if detail.Burst < 0.24 {
		t.Fatalf("expected strong burst contribution, got %.3f", detail.Burst)
	}
	if score <= base {
		t.Fatalf("burst should increase score, got %.2f base %.2f", score, base)
	}
}

func TestComputeTemporalScore_ModerateBurstAtTen(t *testing.T) {
	base := 60.0
	score, detail := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m: 10,
	})
	if detail.Burst < 0.09 || detail.Burst > 0.12 {
		t.Fatalf("expected moderate burst around 0.1, got %.3f", detail.Burst)
	}
	if detail.Multiplier < 1.05 || detail.Multiplier > 1.15 {
		t.Fatalf("expected multiplier ~1.05-1.15, got %.3f", detail.Multiplier)
	}
	if score <= base {
		t.Fatalf("moderate burst should increase score")
	}
}

func TestComputeTemporalScore_PersistenceBoost(t *testing.T) {
	base := 45.0
	score, detail := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m:      0,
		TrendDelta:         0,
		PersistenceMinutes: 130,
	})
	if detail.Persistence < 0.2 {
		t.Fatalf("expected persistence +0.2, got %.3f", detail.Persistence)
	}
	if score <= base {
		t.Fatalf("persistence should increase score, got %.2f base %.2f", score, base)
	}
}

func TestComputeTemporalScore_EmaSmoothing(t *testing.T) {
	base := 80.0
	prev := 40.0
	score, detail := ComputeTemporalScore(base, &prev, TemporalSignals{
		BurstEvents5m:      10,
		TrendDelta:         15,
		PersistenceMinutes: 120,
	})
	if !detail.UsedPrevious {
		t.Fatal("expected previous score to be used for EMA")
	}
	if score <= prev {
		t.Fatalf("smoothed score should exceed previous score, got %.2f prev %.2f", score, prev)
	}
	if score > 100 {
		t.Fatalf("smoothed score must be clamped <= 100, got %.2f", score)
	}
}

func TestComputeTemporalScore_DecayTrend(t *testing.T) {
	base := 60.0
	score, detail := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m:      0,
		TrendDelta:         -15,
		PersistenceMinutes: 0,
	})
	if detail.Trend >= 0 {
		t.Fatalf("expected negative trend contribution, got %.3f", detail.Trend)
	}
	if score >= base {
		t.Fatalf("negative trend should reduce score, got %.2f base %.2f", score, base)
	}
}

func TestComputeTemporalScore_TrendNoiseDeadband(t *testing.T) {
	base := 52.0
	prev := 50.0
	score, detail := ComputeTemporalScore(base, &prev, TemporalSignals{
		TrendDelta: 2, // inside deadband
	})
	if detail.Trend != 0 {
		t.Fatalf("small trend delta should be ignored, got %.3f", detail.Trend)
	}
	if detail.Multiplier != 1.0 {
		t.Fatalf("multiplier should remain 1.0 for small trend delta, got %.3f", detail.Multiplier)
	}
	// EMA still applies due to previous score.
	if score <= prev {
		t.Fatalf("ema score should still be above prev when base is higher; got %.2f prev %.2f", score, prev)
	}
}

func TestComputeTemporalScore_MultiplierCap(t *testing.T) {
	base := 90.0
	score, detail := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m:      999,
		TrendDelta:         999,
		PersistenceMinutes: 999,
	})
	if detail.Multiplier > 1.4 {
		t.Fatalf("multiplier should be capped at 1.4, got %.3f", detail.Multiplier)
	}
	if score > 100 {
		t.Fatalf("final score should be clamped, got %.2f", score)
	}
}

func TestComputeTemporalScore_OscillationResistance(t *testing.T) {
	var prev *float64
	values := []float64{40, 70, 45, 75}
	smoothed := make([]float64, 0, len(values))
	for _, v := range values {
		score, _ := ComputeTemporalScore(v, prev, TemporalSignals{})
		smoothed = append(smoothed, score)
		p := score
		prev = &p
	}
	// EMA should reduce extreme jumps compared to raw inputs.
	rawJump := math.Abs(values[1] - values[0])
	smoothedJump := math.Abs(smoothed[1] - smoothed[0])
	if smoothedJump >= rawJump {
		t.Fatalf("ema should reduce variance; raw jump %.2f smoothed jump %.2f", rawJump, smoothedJump)
	}
}

func TestComputeTemporalScore_RuntimeExploitPlusBurstBand(t *testing.T) {
	base := 65.0
	prev := 60.0
	final, _ := ComputeTemporalScore(base, &prev, TemporalSignals{
		BurstEvents5m:      50,
		UniqueSignalTypes5m: 4,
		TrendDelta:         10,
		PersistenceMinutes: 40,
	})
	if final < 74 || final > 85 {
		t.Fatalf("runtime exploit + burst should be in 74-85 band, got %.2f", final)
	}
}

func TestComputeTemporalScore_CVEOnlyPlusBurstStillLow(t *testing.T) {
	base := 0.38
	final, _ := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m:      50,
		UniqueSignalTypes5m: 1,
		PersistenceMinutes: 0,
	})
	if final >= 5 {
		t.Fatalf("cve-only burst should remain low (<5), got %.2f", final)
	}
}

func TestComputeTemporalScore_HighRiskNoActivityNoInflation(t *testing.T) {
	base := 80.0
	final, detail := ComputeTemporalScore(base, nil, TemporalSignals{})
	if detail.Multiplier != 1.0 {
		t.Fatalf("no activity should not inflate/decay, got multiplier %.3f", detail.Multiplier)
	}
	if final != base {
		t.Fatalf("no activity should keep base score unchanged, got %.2f want %.2f", final, base)
	}
}

func TestComputeTemporalScore_BurstEntropyAntiSpam(t *testing.T) {
	base := 60.0
	spamScore, spam := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m:      1000,
		UniqueSignalTypes5m: 1,
	})
	diverseScore, diverse := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m:      1000,
		UniqueSignalTypes5m: 6,
	})
	if spam.BurstEntropyFactor >= diverse.BurstEntropyFactor {
		t.Fatalf("duplicate signals should have lower entropy factor: spam %.3f diverse %.3f",
			spam.BurstEntropyFactor, diverse.BurstEntropyFactor)
	}
	if spam.Burst >= diverse.Burst {
		t.Fatalf("duplicate spam should have lower burst than diverse signals: spam %.3f diverse %.3f", spam.Burst, diverse.Burst)
	}
	if spamScore >= diverseScore {
		t.Fatalf("duplicate spam should produce lower score than diverse burst: spam %.2f diverse %.2f", spamScore, diverseScore)
	}
	if diverse.Multiplier > 1.4 || spam.Multiplier > 1.4 {
		t.Fatalf("multiplier must remain capped")
	}
}

func TestComputeTemporalScore_DeterministicRepeatedRuns(t *testing.T) {
	base := 58.0
	prev := 50.0
	signals := TemporalSignals{
		BurstEvents5m:      12,
		UniqueSignalTypes5m: 3,
		TrendDelta:         8,
		PersistenceMinutes: 45,
	}
	s1, d1 := ComputeTemporalScore(base, &prev, signals)
	s2, d2 := ComputeTemporalScore(base, &prev, signals)
	if s1 != s2 || d1.Multiplier != d2.Multiplier {
		t.Fatalf("temporal score must be deterministic: s1 %.2f s2 %.2f m1 %.3f m2 %.3f",
			s1, s2, d1.Multiplier, d2.Multiplier)
	}
}

func TestComputeTemporalScore_DecayOverTime(t *testing.T) {
	base := 80.0
	prev := base

	score10m, _ := ComputeTemporalScore(base, &prev, TemporalSignals{InactivityMinutes: 10})
	prev = score10m
	score30m, _ := ComputeTemporalScore(base, &prev, TemporalSignals{InactivityMinutes: 30})
	prev = score30m
	score60m, _ := ComputeTemporalScore(base, &prev, TemporalSignals{InactivityMinutes: 60})

	if !(score10m < base && score30m < score10m && score60m < score30m) {
		t.Fatalf("expected monotonic decay over inactivity windows: base=%.2f 10m=%.2f 30m=%.2f 60m=%.2f",
			base, score10m, score30m, score60m)
	}
}

func TestComputeTemporalScore_BurstWindowReset(t *testing.T) {
	base := 60.0
	spike, _ := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m: 50,
	})
	reset, detail := ComputeTemporalScore(base, nil, TemporalSignals{})
	if reset >= spike {
		t.Fatalf("score should drop after burst window reset: spike=%.2f reset=%.2f", spike, reset)
	}
	if detail.Burst != 0 {
		t.Fatalf("burst should be zero after reset, got %.3f", detail.Burst)
	}
}

func TestComputeTemporalScore_BurstEntropyGradient(t *testing.T) {
	base := 60.0
	s1, _ := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m:      100,
		UniqueSignalTypes5m: 1,
	})
	s2, _ := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m:      100,
		UniqueSignalTypes5m: 3,
	})
	s3, _ := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m:      100,
		UniqueSignalTypes5m: 6,
	})
	if !(s1 < s2 && s2 < s3) {
		t.Fatalf("entropy gradient broken: s1=%.2f s2=%.2f s3=%.2f", s1, s2, s3)
	}
}

func TestComputeTemporalScore_ExtremeLowBaseStillLow(t *testing.T) {
	base := 0.1
	score, _ := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m:      500,
		UniqueSignalTypes5m: 6,
		TrendDelta:         20,
		PersistenceMinutes: 180,
	})
	if score >= 5 {
		t.Fatalf("extremely low base should remain low after temporal boosts, got %.2f", score)
	}
}

func TestComputeTemporalScore_DoesNotExplodeWithHighBase(t *testing.T) {
	base := 95.0
	score, detail := ComputeTemporalScore(base, nil, TemporalSignals{
		BurstEvents5m:      999,
		UniqueSignalTypes5m: 6,
		TrendDelta:         50,
		PersistenceMinutes: 999,
	})
	if detail.Multiplier > 1.4 {
		t.Fatalf("multiplier exceeded cap: %.3f", detail.Multiplier)
	}
	if score > 100 {
		t.Fatalf("score exploded above 100: %.2f", score)
	}
}

func TestComputeTemporalScore_EMAConvergence(t *testing.T) {
	base := 70.0
	prev := 50.0
	signals := TemporalSignals{
		BurstEvents5m:      10,
		UniqueSignalTypes5m: 4,
		PersistenceMinutes: 30,
	}

	// Target raw score with fixed signals should be constant.
	target, detail := ComputeTemporalScore(base, nil, signals)
	if detail.UsedPrevious {
		t.Fatalf("unexpected previous score in target computation")
	}

	cur := prev
	for i := 0; i < 10; i++ {
		next, _ := ComputeTemporalScore(base, &cur, signals)
		cur = next
	}
	if math.Abs(cur-target) > 1.5 {
		t.Fatalf("EMA should converge close to target: got %.2f target %.2f", cur, target)
	}
}

// F4 from temporal test suite: sustained attack should drive multiplier
// close to the upper bound without exceeding it.
func TestComputeTemporalScore_F4_SustainedAttack(t *testing.T) {
	base := 60.0
	prev := 55.0
	final, detail := ComputeTemporalScore(base, &prev, TemporalSignals{
		BurstEvents5m:       50,
		UniqueSignalTypes5m: 5,
		TrendDelta:          12,
		PersistenceMinutes:  180,
	})
	if detail.Multiplier < 1.2 || detail.Multiplier > 1.4 {
		t.Fatalf("sustained attack should stay near upper multiplier bound: got %.3f", detail.Multiplier)
	}
	if final <= base {
		t.Fatalf("sustained attack should increase score: base %.2f final %.2f", base, final)
	}
}

// H4 from temporal test suite: slow drip should not create burst inflation,
// while persistence can still raise risk over time.
func TestComputeTemporalScore_H4_SlowDrip(t *testing.T) {
	base := 52.0
	prev := 50.0

	initial, initialDetail := ComputeTemporalScore(base, &prev, TemporalSignals{
		BurstEvents5m:       1,
		UniqueSignalTypes5m: 1,
		PersistenceMinutes:  0,
		TrendDelta:          0,
	})
	if initialDetail.Burst != 0 {
		t.Fatalf("slow drip should not trigger burst inflation, got %.3f", initialDetail.Burst)
	}

	// After long-running low-frequency activity, persistence should contribute.
	after, afterDetail := ComputeTemporalScore(base, &initial, TemporalSignals{
		BurstEvents5m:       1,
		UniqueSignalTypes5m: 1,
		PersistenceMinutes:  140,
		TrendDelta:          0,
	})
	if afterDetail.Persistence <= 0 {
		t.Fatalf("slow drip should gain persistence over time, got %.3f", afterDetail.Persistence)
	}
	if after <= initial {
		t.Fatalf("slow drip persistence should raise score over time: initial %.2f after %.2f", initial, after)
	}
}

