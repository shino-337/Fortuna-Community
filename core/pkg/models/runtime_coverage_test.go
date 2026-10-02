package models

import (
	"testing"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/stretchr/testify/require"
)

func TestRuntimeCoverageCoversIntervalRequiresBothBounds(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Microsecond)
	continuous := base
	now := base.Add(5 * time.Second)
	row := RuntimeCoverage{
		ClusterID:       "cluster-a",
		AgentID:         "agent-a",
		ProducerID:      "falco",
		SessionID:       "session-000000000001",
		Status:          "complete",
		WindowStart:     base.Add(2 * time.Second),
		WindowEnd:       base.Add(4 * time.Second),
		ContinuousSince: &continuous,
	}
	lastCoverageEnd := row.WindowEnd
	leaseEnd := now.Add(time.Minute)
	row.SourceSessionID = "sensor-000000000001"
	producer := RuntimeProducerState{
		ClusterID:       "cluster-a",
		AgentID:         "agent-a",
		ProducerID:      "falco",
		SessionID:       "session-000000000001",
		Enabled:         true,
		Authoritative:   true,
		SourceSessionID: row.SourceSessionID, SourceHealthSince: &continuous, SourceHealthEnd: &lastCoverageEnd, SourceHealthReceivedAt: &now, SourceHealthExpiresAt: &leaseEnd,
		State:           "active",
		LastHeartbeatAt: now,
		LastCoverageEnd: &lastCoverageEnd,
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

	staleWindow := now.Add(-collection.RuntimeProducerLeaseMaxAge - time.Second)
	producer.LastCoverageEnd = &staleWindow
	require.False(t, row.CoversInterval(&producer, base.Add(time.Second), base.Add(3*time.Second), now),
		"Agent lifecycle heartbeat cannot keep a silent producer active")
	producer.LastCoverageEnd = &lastCoverageEnd

	producer.Enabled = false
	require.False(t, row.CoversInterval(&producer, base.Add(time.Second), base.Add(3*time.Second), now),
		"disabled producer cannot prove coverage")
	producer.Enabled = true
	producer.State = "active"
	producer.SessionID = "session-000000000002"
	require.False(t, row.CoversInterval(&producer, base.Add(time.Second), base.Add(3*time.Second), now),
		"coverage from a previous Agent session cannot prove coverage")
}

func TestRuntimeProducerEffectiveStatusSeparatesActivityFromAuthority(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	recent := now.Add(-time.Second)
	state := RuntimeProducerState{
		ClusterID:       "cluster-a",
		AgentID:         "agent-a",
		ProducerID:      "falco",
		SessionID:       "session-000000000001",
		Enabled:         true,
		Authoritative:   false,
		State:           collection.RuntimeProducerActive,
		LastHeartbeatAt: now,
		LastCoverageEnd: &recent,
	}
	require.Equal(t, collection.RuntimeProducerNonAuthoritative, state.EffectiveStatus(now),
		"fresh reader activity is observable but not absence-authoritative")

	stale := now.Add(-collection.RuntimeProducerLeaseMaxAge - time.Second)
	state.LastCoverageEnd = &stale
	require.Equal(t, "stale", state.EffectiveStatus(now),
		"stale reader activity must not be hidden by non-authoritative status")

	state.LastCoverageEnd = &recent
	state.Enabled = false
	state.State = collection.RuntimeProducerDisabled
	require.Equal(t, collection.RuntimeProducerDisabled, state.EffectiveStatus(now))
}
