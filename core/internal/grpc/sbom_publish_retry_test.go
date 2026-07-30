package grpc

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPublishSBOMCreatedWithRetry_SucceedsFirstAttempt(t *testing.T) {
	t.Parallel()
	var calls []string
	publish := func(subject string, data []byte) error {
		calls = append(calls, subject)
		return nil
	}
	pri, dlq := publishSBOMCreatedWithRetry(context.Background(), publish, []byte(`{}`), func(time.Duration) {})
	if pri != nil || dlq != nil {
		t.Fatalf("want nil, nil got pri=%v dlq=%v", pri, dlq)
	}
	if len(calls) != 1 || calls[0] != sbomCreatedSubject {
		t.Fatalf("calls=%v", calls)
	}
}

func TestPublishSBOMCreatedWithRetry_SucceedsAfterRetries(t *testing.T) {
	t.Parallel()
	var n int
	publish := func(subject string, data []byte) error {
		if subject != sbomCreatedSubject {
			t.Fatalf("unexpected subject %q", subject)
		}
		n++
		if n < 3 {
			return errors.New("transient")
		}
		return nil
	}
	pri, dlq := publishSBOMCreatedWithRetry(context.Background(), publish, []byte(`{}`), func(time.Duration) {})
	if pri != nil || dlq != nil {
		t.Fatalf("want nil, nil got pri=%v dlq=%v", pri, dlq)
	}
	if n != 3 {
		t.Fatalf("want 3 primary attempts, got %d", n)
	}
}

func TestPublishSBOMCreatedWithRetry_AllPrimaryFailThenDLQSuccess(t *testing.T) {
	t.Parallel()
	var calls []string
	publish := func(subject string, data []byte) error {
		calls = append(calls, subject)
		if subject == sbomCreatedSubject {
			return errors.New("fail")
		}
		if subject == sbomCreatedDLQSubject {
			return nil
		}
		t.Fatalf("unexpected subject %q", subject)
		return errors.New("bad")
	}
	pri, dlq := publishSBOMCreatedWithRetry(context.Background(), publish, []byte(`{}`), func(time.Duration) {})
	if pri == nil {
		t.Fatal("expected primary error")
	}
	if dlq != nil {
		t.Fatalf("dlq should succeed: %v", dlq)
	}
	// 5 primary + 1 dlq
	if len(calls) != 6 {
		t.Fatalf("calls=%v len=%d", calls, len(calls))
	}
	for i := 0; i < 5; i++ {
		if calls[i] != sbomCreatedSubject {
			t.Fatalf("call %d: want primary subject", i)
		}
	}
	if calls[5] != sbomCreatedDLQSubject {
		t.Fatalf("last call should be DLQ")
	}
}

func TestPublishSBOMCreatedWithRetry_PrimaryAndDLQFail(t *testing.T) {
	t.Parallel()
	publish := func(subject string, data []byte) error {
		return errors.New("always fail")
	}
	pri, dlq := publishSBOMCreatedWithRetry(context.Background(), publish, []byte(`{}`), func(time.Duration) {})
	if pri == nil || dlq == nil {
		t.Fatalf("want both errors, got pri=%v dlq=%v", pri, dlq)
	}
}

func TestPublishSBOMCreatedWithRetry_ContextCancelledBeforePublish(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var subjects []string
	publish := func(subject string, data []byte) error {
		subjects = append(subjects, subject)
		return nil
	}
	pri, dlq := publishSBOMCreatedWithRetry(ctx, publish, []byte(`{}`), func(time.Duration) {})
	if !errors.Is(pri, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", pri)
	}
	if dlq != nil {
		t.Fatalf("unexpected dlq err: %v", dlq)
	}
	// No primary attempts; best-effort DLQ only
	if len(subjects) != 1 || subjects[0] != sbomCreatedDLQSubject {
		t.Fatalf("expected single DLQ publish, got %v", subjects)
	}
}
