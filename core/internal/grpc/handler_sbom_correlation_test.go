package grpc

import (
	"context"
	"regexp"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestCorrelationIDFromContext(t *testing.T) {
	// With metadata: should return the value from Agent
	ctxWith := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-correlation-id", "agent-uuid-123"))
	if got := correlationIDFromContext(ctxWith); got != "agent-uuid-123" {
		t.Errorf("correlationIDFromContext(with metadata) = %q, want agent-uuid-123", got)
	}

	// Without metadata: should return a generated id (hex or core-*)
	ctxEmpty := context.Background()
	got := correlationIDFromContext(ctxEmpty)
	hexPattern := regexp.MustCompile(`^[0-9a-f]{16}$`)
	corePattern := regexp.MustCompile(`^core-\d+$`)
	if !hexPattern.MatchString(got) && !corePattern.MatchString(got) {
		t.Errorf("correlationIDFromContext(no metadata) = %q, want 16-char hex or core-<nano>", got)
	}
}
