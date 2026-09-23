package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRuntimeCoverageCoversIntervalRequiresBothBounds(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Microsecond)
	continuous := base
	now := base.Add(5 * time.Second)
	row := RuntimeCoverage{
		ClusterID: "cluster-a",
		AgentID: "agent-a",
		ProducerID: "falco",
		SessionID: "session-000000000001",
		Status: "complete",
		WindowStart: base.Add(2 * time.Second),
		WindowEnd: base.Add(4 * time.Second),
		ContinuousSince: &continuous,
	}
	producer := RuntimeProducerState{
		ClusterID: "cluster-a",
		AgentID: "agent-a",
		ProducerID: "falco",
		SessionID: "session-000000000001",
		Enabled: true,
		Authoritative: true,
		State: "active",
		LastHeartbeatAt: now,
	}

	require.True(t, row.CoversInterval(&producer, base.Add(time.Second), base.Add(3*time.Second), now))
	require.False(t, row.CoversInterval(&producer, base.Add(time.Second), base.Add(5*time.Second), now),
		"freshness must not substitute for coverage through the required end")
	require.False(t, row.CoversInterval(&producer, time.Time{}, base.Add(3*time.Second), now))
	require.False(t, row.CoversInterval(&producer, base.Add(3*time.Second), base.Add(2*time.Second), now))

	late := base.Add(2 * time.Second)
	row.ContinuousSince = &late
	require.False(t, row.CoversInterval(&producer, base.Add(time.Second), base.Add(3*time.Second), now),
		"continuity that begins after the required start cannot prove absence")

	row.ContinuousSince = &continuous
	require.False(t, row.CoversInterval(&producer, base.Add(time.Second), base.Add(3*time.Second), base.Add(20*time.Minute)),
		"stale latest coverage cannot prove the interval")

	producer.Enabled = false
	require.False(t, row.CoversInterval(&producer, base.Add(time.Second), base.Add(3*time.Second), now),
		"disabled producer cannot prove coverage")
	producer.Enabled = true
	producer.State = "active"
	producer.SessionID = "session-000000000002"
	require.False(t, row.CoversInterval(&producer, base.Add(time.Second), base.Add(3*time.Second), now),
		"coverage from a previous Agent session cannot prove coverage")
}
