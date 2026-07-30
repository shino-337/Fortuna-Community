package risk

import "math"

const (
	temporalAlpha         = 0.6
	temporalMinMultiplier = 0.8
	temporalMaxMultiplier = 1.4
	trendNoiseDeadband    = 5.0
)

// TemporalSignals captures time-aware context for score adjustments.
type TemporalSignals struct {
	BurstEvents5m      int
	UniqueSignalTypes5m int
	TrendDelta         float64
	PersistenceMinutes int
	InactivityMinutes  int
}

// TemporalResult provides explainable temporal contributions.
type TemporalResult struct {
	Burst              float64 `json:"burst"`
	BurstEntropyFactor float64 `json:"burst_entropy_factor"`
	Trend              float64 `json:"trend"`
	Persistence        float64 `json:"persistence"`
	InactivityDecay    float64 `json:"inactivity_decay"`
	Multiplier         float64 `json:"multiplier"`
	RawTemporalScore   float64 `json:"raw_temporal_score"`
	EmaSmoothedScore   float64 `json:"ema_smoothed_score"`
	PreviousScore      float64 `json:"previous_score,omitempty"`
	UsedPrevious       bool    `json:"used_previous"`
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func burstContribution(events5m int) float64 {
	switch {
	case events5m <= 1:
		return 0
	case events5m < 10:
		// Ramp from 0 to 0.1
		return (float64(events5m-1) / 9.0) * 0.1
	case events5m < 50:
		// Ramp from 0.1 to 0.25
		return 0.1 + (float64(events5m-10)/40.0)*0.15
	default:
		return 0.25
	}
}

func burstEntropyFactor(uniqueTypes int) float64 {
	// Unknown entropy -> keep full burst behavior for backward compatibility.
	if uniqueTypes <= 0 {
		return 1.0
	}
	// 1 duplicated signal type => heavily discounted.
	// 5+ unique signal types => no discount.
	return clamp(float64(uniqueTypes)/5.0, 0.2, 1.0)
}

func trendContribution(delta float64) float64 {
	if math.Abs(delta) < trendNoiseDeadband {
		return 0
	}
	// Positive acceleration gets up to +0.1, negative trend down to -0.05.
	switch {
	case delta >= 15:
		return 0.1
	case delta <= -15:
		return -0.05
	case delta >= 0:
		return (delta / 15.0) * 0.1
	default:
		return (delta / 15.0) * 0.05
	}
}

func persistenceContribution(minutes int) float64 {
	switch {
	case minutes >= 120:
		return 0.2
	case minutes >= 30:
		return 0.1
	default:
		return 0
	}
}

func inactivityDecayContribution(minutes int) float64 {
	switch {
	case minutes >= 60:
		return -0.15
	case minutes >= 30:
		return -0.1
	case minutes >= 10:
		return -0.03
	default:
		return 0
	}
}

// ComputeTemporalScore applies burst/trend/persistence + EMA smoothing.
// This is a post-core layer and does not change the core risk model.
func ComputeTemporalScore(baseScore float64, previousScore *float64, signals TemporalSignals) (float64, TemporalResult) {
	baseScore = clamp(baseScore, 0, 100)
	burstBase := burstContribution(signals.BurstEvents5m)
	entropyFactor := burstEntropyFactor(signals.UniqueSignalTypes5m)
	burst := burstBase * entropyFactor
	trend := trendContribution(signals.TrendDelta)
	persistence := persistenceContribution(signals.PersistenceMinutes)
	inactivity := inactivityDecayContribution(signals.InactivityMinutes)

	multiplier := 1.0 + burst + trend + persistence + inactivity
	// Low-base guardrail: avoid temporal amplification runaway for low structural
	// risk entities.
	if baseScore < 40 {
		multiplier = math.Min(multiplier, 1.10)
	}
	multiplier = clamp(multiplier, temporalMinMultiplier, temporalMaxMultiplier)
	if baseScore < 40 {
		maxLowBaseScore := baseScore * 1.30
		if maxLowBaseScore < 100 {
			rawCap := clamp(maxLowBaseScore, 0, 100)
			if baseScore*multiplier > rawCap {
				multiplier = rawCap / math.Max(baseScore, 0.0001)
			}
		}
	}
	rawTemporal := clamp(baseScore*multiplier, 0, 100)

	smoothed := rawTemporal
	usedPrevious := false
	prev := 0.0
	if previousScore != nil {
		prev = clamp(*previousScore, 0, 100)
		smoothed = clamp(temporalAlpha*rawTemporal+(1.0-temporalAlpha)*prev, 0, 100)
		usedPrevious = true
	}

	result := TemporalResult{
		Burst:            math.Round(burst*1000) / 1000,
		BurstEntropyFactor: math.Round(entropyFactor*1000) / 1000,
		Trend:            math.Round(trend*1000) / 1000,
		Persistence:      math.Round(persistence*1000) / 1000,
		InactivityDecay:  math.Round(inactivity*1000) / 1000,
		Multiplier:       math.Round(multiplier*1000) / 1000,
		RawTemporalScore: math.Round(rawTemporal*100) / 100,
		EmaSmoothedScore: math.Round(smoothed*100) / 100,
		PreviousScore:    math.Round(prev*100) / 100,
		UsedPrevious:     usedPrevious,
	}
	return result.EmaSmoothedScore, result
}

