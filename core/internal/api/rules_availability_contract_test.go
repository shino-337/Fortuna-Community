package api

import (
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/riskengine"
	"github.com/stretchr/testify/require"
)

func TestCollectRuleActivationMetaFailsClosedWhenInsightsUnavailable(t *testing.T) {
	db := availabilityTestDB(t)
	_, err := collectRuleActivationMeta(db, []riskengine.Rule{{ID: "RULE-1"}})
	require.Error(t, err)
}

func TestCollectRuleActivationMetaReturnsExplicitEmptyEvidenceOnSuccessfulQuery(t *testing.T) {
	db := availabilityTestDB(t, &models.Insight{})
	meta, err := collectRuleActivationMeta(db, []riskengine.Rule{{ID: "RULE-1"}})
	require.NoError(t, err)
	require.Contains(t, meta, "RULE-1")
	require.EqualValues(t, 0, meta["RULE-1"].Count)
	require.Empty(t, meta["RULE-1"].RelatedCapabilities)
}
