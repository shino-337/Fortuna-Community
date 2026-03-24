package webhook

import (
	"context"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWebhookRiskGateDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RiskScore{}); err != nil {
		t.Fatalf("migrate risk_scores: %v", err)
	}
	return db
}

func TestHybridRiskGate_HitInSensitiveNamespace(t *testing.T) {
	db := newWebhookRiskGateDB(t)
	if err := db.Create(&models.RiskScore{
		ResourceType: "Pod",
		ResourceUID:  "p1",
		ResourceName: "pod-a",
		Namespace:    "prod",
		ClusterID:    "c1",
		TotalScore:   85,
	}).Error; err != nil {
		t.Fatalf("seed risk score: %v", err)
	}
	t.Setenv("ADMISSION_RISK_GATE_ENABLED", "true")
	t.Setenv("ADMISSION_RISK_SENSITIVE_NAMESPACES", "prod,fortuna")
	t.Setenv("ADMISSION_RISK_BLOCK_THRESHOLD", "70")

	w := &AdmissionWebhook{db: db}
	hit, msg := w.hybridRiskGate(context.Background(), "prod")
	if !hit {
		t.Fatalf("expected hybrid risk gate hit, got false")
	}
	if msg == "" {
		t.Fatalf("expected non-empty risk gate message")
	}
}

func TestHybridRiskGate_NoHitInNonSensitiveNamespace(t *testing.T) {
	db := newWebhookRiskGateDB(t)
	if err := db.Create(&models.RiskScore{
		ResourceType: "Pod",
		ResourceUID:  "p2",
		ResourceName: "pod-b",
		Namespace:    "dev",
		ClusterID:    "c1",
		TotalScore:   95,
	}).Error; err != nil {
		t.Fatalf("seed risk score: %v", err)
	}
	t.Setenv("ADMISSION_RISK_GATE_ENABLED", "true")
	t.Setenv("ADMISSION_RISK_SENSITIVE_NAMESPACES", "prod,fortuna")
	t.Setenv("ADMISSION_RISK_BLOCK_THRESHOLD", "70")

	w := &AdmissionWebhook{db: db}
	hit, _ := w.hybridRiskGate(context.Background(), "dev")
	if hit {
		t.Fatalf("expected no hit for non-sensitive namespace")
	}
}

func TestRiskGateMode_DefaultAndExplicit(t *testing.T) {
	t.Setenv("ADMISSION_RISK_GATE_MODE", "")
	if m := riskGateMode(); m != "enforce" {
		t.Fatalf("expected default enforce, got %s", m)
	}
	t.Setenv("ADMISSION_RISK_GATE_MODE", "audit")
	if m := riskGateMode(); m != "audit" {
		t.Fatalf("expected audit, got %s", m)
	}
	t.Setenv("ADMISSION_RISK_GATE_MODE", "weird")
	if m := riskGateMode(); m != "enforce" {
		t.Fatalf("expected fallback enforce, got %s", m)
	}
}
