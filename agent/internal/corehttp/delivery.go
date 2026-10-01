package corehttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DeliveryRequestLimit = 12
const DeliveryBackoff = 30 * time.Second

// PostError keeps machine-readable ownership rejection distinct from other 403s.
type PostError struct {
	Status     int
	Code       string
	RetryAfter time.Duration
}

func (e *PostError) Error() string { return fmt.Sprintf("ingest POST: %d (%s)", e.Status, e.Code) }

func DecodePostError(resp *http.Response, now time.Time) *PostError {
	var body struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&body)
	delay := DeliveryBackoff
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		value := strings.TrimSpace(resp.Header.Get("Retry-After"))
		if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds > 0 {
			if seconds > 86400 {
				seconds = 86400
			}
			delay = max(delay, time.Duration(seconds)*time.Second)
		} else if until, err := http.ParseTime(value); err == nil {
			delay = max(delay, min(until.Sub(now), 24*time.Hour))
		}
	}
	return &PostError{Status: resp.StatusCode, Code: body.Code, RetryAfter: delay}
}

type DeliveryBudget struct {
	Remaining int
	Now       time.Time
	RetryAt   time.Time
	Blocked   bool
}

type DeliveryResult[T any] struct {
	Delivered []T
	Retry     []T
	Rejected  []T
	Err       error
}

// DeliverIsolated only splits an explicitly ownership-rejected batch. Once any
// other failure occurs, no more requests are issued during this flush. Unsent
// siblings remain retryable rather than being mistaken for rejected evidence.
func DeliverIsolated[T any](ctx context.Context, events []T, budget *DeliveryBudget, ownershipCode string, post func(context.Context, []T) error) DeliveryResult[T] {
	if len(events) == 0 {
		return DeliveryResult[T]{}
	}
	if budget.Blocked || budget.Remaining <= 0 || ctx.Err() != nil {
		return DeliveryResult[T]{Retry: events, Err: ctx.Err()}
	}
	budget.Remaining--
	err := post(ctx, events)
	if err == nil {
		return DeliveryResult[T]{Delivered: events}
	}
	var postErr *PostError
	if !errors.As(err, &postErr) || postErr.Status != http.StatusForbidden || postErr.Code != ownershipCode {
		budget.Blocked = true
		delay := DeliveryBackoff
		if postErr != nil {
			delay = max(delay, postErr.RetryAfter)
		}
		// Isolation or a slow POST can consume most of the flush interval. Start
		// backoff at failure, not at the stale flush-start timestamp.
		budget.RetryAt = time.Now().Add(delay)
		return DeliveryResult[T]{Retry: events, Err: err}
	}
	if len(events) == 1 {
		return DeliveryResult[T]{Rejected: events}
	}
	mid := len(events) / 2
	left := DeliverIsolated(ctx, events[:mid], budget, ownershipCode, post)
	right := DeliverIsolated(ctx, events[mid:], budget, ownershipCode, post)
	return DeliveryResult[T]{Delivered: joinDeliverySlices(left.Delivered, right.Delivered), Retry: joinDeliverySlices(left.Retry, right.Retry), Rejected: joinDeliverySlices(left.Rejected, right.Rejected), Err: errors.Join(left.Err, right.Err)}
}

func joinDeliverySlices[T any](left, right []T) []T {
	joined := make([]T, 0, len(left)+len(right))
	joined = append(joined, left...)
	return append(joined, right...)
}
