package corehttp

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDeliveryBudgetBoundsIsolationAndRetainsUnsent(t *testing.T) {
	input := make([]int, 200)
	for i := range input {
		input[i] = i
	}
	calls := 0
	budget := &DeliveryBudget{Remaining: DeliveryRequestLimit, Now: time.Now()}
	result := DeliverIsolated(t.Context(), input, budget, "ownership", func(context.Context, []int) error {
		calls++
		return &PostError{Status: 403, Code: "ownership"}
	})
	if calls != DeliveryRequestLimit || len(result.Delivered) != 0 || len(result.Rejected)+len(result.Retry) != len(input) {
		t.Fatalf("unbounded isolation or lost siblings: calls=%d result=%+v", calls, result)
	}
	seen := make(map[int]bool)
	for _, n := range append(result.Retry, result.Rejected...) {
		if seen[n] {
			t.Fatalf("duplicate event %d", n)
		}
		seen[n] = true
	}
	for i, n := range input {
		if n != i {
			t.Fatalf("delivery classification mutated source input at %d: %d", i, n)
		}
	}
}

func TestDeliveryStopsSiblingsOnRateLimitAndOtherForbidden(t *testing.T) {
	for _, code := range []int{429, 401, 403, 503} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			calls := 0
			budget := &DeliveryBudget{Remaining: 12, Now: time.Now().Add(-5 * time.Minute)}
			var failedAt time.Time
			result := DeliverIsolated(t.Context(), []int{1, 2, 3, 4}, budget, "ownership", func(context.Context, []int) error {
				calls++
				if calls == 1 {
					return &PostError{Status: 403, Code: "ownership"}
				}
				failedAt = time.Now()
				return &PostError{Status: code, Code: "different", RetryAfter: 2 * time.Minute}
			})
			if calls != 2 || len(result.Retry) != 4 || len(result.Rejected) != 0 || !budget.Blocked || budget.RetryAt.Before(failedAt.Add(2*time.Minute)) || budget.RetryAt.After(time.Now().Add(2*time.Minute)) {
				t.Fatalf("failure split or dropped siblings: calls=%d result=%+v budget=%+v", calls, result, budget)
			}
		})
	}
}

func TestPostErrorHonorsRetryAfterAndCancelledDelivery(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, value := range []string{"120", now.Add(2 * time.Minute).Format(http.TimeFormat)} {
		resp := &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{value}}, Body: io.NopCloser(strings.NewReader(`{}`))}
		if err := DecodePostError(resp, now); err.RetryAfter != 2*time.Minute {
			t.Fatalf("Retry-After=%q parsed=%s", value, err.RetryAfter)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result := DeliverIsolated(ctx, []int{1}, &DeliveryBudget{Remaining: 12}, "ownership", func(context.Context, []int) error { t.Fatal("cancelled request was sent"); return nil })
	if len(result.Retry) != 1 || result.Err == nil {
		t.Fatalf("cancelled event lost: %+v", result)
	}
}
