package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRuntimeCoverageCoversIntervalRequiresBothBounds(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Microsecond)
	continuous := base
	row := RuntimeCoverage{
		Status: "complete",
		WindowStart: base.Add(2 * time.Second),
		WindowEnd: base.Add(4 * time.Second),
		ContinuousSince: &continuous,
	}
	now := base.Add(5 * time.Second)

	require.True(t, row.CoversInterval(base.Add(time.Second), base.Add(3*time.Second), now))
	require.False(t, row.CoversInterval(base.Add(time.Second), base.Add(5*time.Second), now),
		"freshness must not substitute for coverage through the required end")
	require.False(t, row.CoversInterval(time.Time{}, base.Add(3*time.Second), now))
	require.False(t, row.CoversInterval(base.Add(3*time.Second), base.Add(2*time.Second), now))

	late := base.Add(2 * time.Second)
	row.ContinuousSince = &late
	require.False(t, row.CoversInterval(base.Add(time.Second), base.Add(3*time.Second), now),
		"continuity that begins after the required start cannot prove absence")

	row.ContinuousSince = &continuous
	require.False(t, row.CoversInterval(base.Add(time.Second), base.Add(3*time.Second), base.Add(20*time.Minute)),
		"stale latest coverage cannot prove the interval")
}
