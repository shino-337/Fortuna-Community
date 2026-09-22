package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fortuna/agent/internal/cluster"
	"github.com/fortuna/api/collection"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestInventoryCollectionAgentReportsEmptyAndFailure(t *testing.T) {
	for _, scenario := range []string{"empty", "list-error", "pagination", "core-rejected"} {
		t.Run(scenario, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			if scenario == "list-error" {
				client.PrependReactor("list", "roles", func(ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("collection unavailable")
				})
			}
			if scenario == "pagination" {
				client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
					return true, &corev1.PodList{ListMeta: metav1.ListMeta{Continue: "more"}}, nil
				})
			}
			var captured SyncPayload
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
					t.Error(err)
				}
				if scenario == "core-rejected" {
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer server.Close()
			s := NewSyncer(client, server.URL, &cluster.Info{ID: "a", Name: "A"}, time.Minute, "ns", "agent-a", "node-a", "test")
			err := s.SyncOnce(context.Background())
			if scenario == "empty" && err != nil {
				t.Fatal(err)
			}
			if scenario != "empty" && err == nil {
				t.Fatal("failure reported as successful collection")
			}
			if captured.Collection == nil {
				t.Fatal("missing collection metadata")
			}
			if err := captured.Collection.Validate(time.Now()); err != nil {
				t.Fatal(err)
			}
			if captured.Collection.Namespace != "ns" || captured.Agent.AgentID != "agent-a" {
				t.Fatal("lost collection ownership/scope")
			}
			if scenario == "list-error" || scenario == "pagination" {
				if captured.Collection.Status != "failed" || captured.Data.IsFullSync || len(captured.Data.Pods) != 0 {
					t.Fatal("partial collection sent as full inventory")
				}
			} else {
				if captured.Collection.Status != "complete" {
					t.Fatal("empty successful list not distinguished from failure")
				}
				for _, kind := range collection.InventoryKinds {
					if n, ok := captured.Collection.Counts[kind]; !ok || n != 0 {
						t.Fatalf("invalid count for %s", kind)
					}
				}
			}
		})
	}
}


func TestInventoryCollectionRecordsListStartBeforeResponse(t *testing.T) {
	client := fake.NewSimpleClientset()
	entered := make(chan struct{})
	release := make(chan struct{})
	client.PrependReactor("list", "roles", func(ktesting.Action) (bool, runtime.Object, error) {
		close(entered)
		<-release
		return false, nil, nil
	})

	s := NewSyncer(client, "http://unused", &cluster.Info{ID: "a", Name: "A"}, time.Minute, "ns", "agent-a", "node-a", "test")
	type result struct {
		payload *SyncPayload
		err     error
	}
	done := make(chan result, 1)
	go func() {
		payload, err := s.buildPayload(context.Background())
		done <- result{payload: payload, err: err}
	}()

	<-entered
	whileBlocked := time.Now().UTC()
	close(release)
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	started := got.payload.Collection.KindStartedAt["roles"]
	if started.IsZero() || started.After(whileBlocked) {
		t.Fatalf("role List start bound was captured after the request was already in flight: start=%v blocked_at=%v", started, whileBlocked)
	}
	if got.payload.Collection.ObservedAt.Before(whileBlocked) {
		t.Fatalf("collection end must follow the blocked Role List: end=%v blocked_at=%v", got.payload.Collection.ObservedAt, whileBlocked)
	}
}
