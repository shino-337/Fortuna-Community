package riskengine

import (
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestExceptionPolicyClusterIsolation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ExceptionPolicy{}))

	expires := time.Now().Add(time.Hour)
	require.NoError(t, db.Create(&models.ExceptionPolicy{
		ClusterID:   "cluster-a",
		ResourceUID: "shared-pod-uid",
		CVEID:       "CVE-2099-0001",
		InsightType: "vulnerability",
		ExpiresAt:   &expires,
	}).Error)

	require.True(t, isExempted(db, "cluster-a", "shared-pod-uid", "CVE-2099-0001", "vulnerability"))
	require.False(t, isExempted(db, "cluster-b", "shared-pod-uid", "CVE-2099-0001", "vulnerability"),
		"an exception in cluster-a must never suppress the same UID/CVE in cluster-b")
	require.False(t, isExempted(db, "", "shared-pod-uid", "CVE-2099-0001", "vulnerability"),
		"ambiguous legacy findings without cluster ownership must not inherit scoped exceptions")
}

func TestExceptionPolicyClusterIsolationRejectsLegacyUnscopedException(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ExceptionPolicy{}))

	require.NoError(t, db.Create(&models.ExceptionPolicy{
		ClusterID:   "",
		ResourceUID: "shared-pod-uid",
		CVEID:       "CVE-2099-0002",
		InsightType: "vulnerability",
	}).Error)

	require.False(t, isExempted(db, "cluster-a", "shared-pod-uid", "CVE-2099-0002", "vulnerability"))
	require.False(t, isExempted(db, "cluster-b", "shared-pod-uid", "CVE-2099-0002", "vulnerability"))
}
