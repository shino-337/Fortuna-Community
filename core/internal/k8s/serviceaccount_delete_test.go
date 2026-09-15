package k8s

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	kt "k8s.io/client-go/testing"
	"testing"
)

func TestDeleteServiceAccountUID(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "team", UID: "original"}})
	cs.PrependReactor("delete", "serviceaccounts", func(a kt.Action) (bool, runtime.Object, error) {
		opts := a.(kt.DeleteAction).GetDeleteOptions()
		if opts.Preconditions == nil || opts.Preconditions.UID == nil || *opts.Preconditions.UID != "original" {
			t.Fatal("missing UID precondition")
		}
		return true, nil, fmt.Errorf("UID conflict")
	})
	client := Client{Clientset: cs}
	if err := client.DeleteServiceAccountUID(context.Background(), "team", "app", ""); err == nil || len(cs.Actions()) != 0 {
		t.Fatal("empty UID must fail before request")
	}
	if err := client.DeleteServiceAccountUID(context.Background(), "team", "app", "original"); err == nil {
		t.Fatal("conflict must be returned")
	}
	client.Clientset = fake.NewSimpleClientset()
	if err := client.DeleteServiceAccountUID(context.Background(), "team", "app", "original"); err != nil {
		t.Fatalf("NotFound retry: %v", err)
	}
}
