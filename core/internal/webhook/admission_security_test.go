package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func unsupportedDeploymentAdmissionReview() admissionv1.AdmissionReview {
	return admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID: types.UID("security-test"),
			Kind: metav1.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			Resource: metav1.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Raw: []byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"x","namespace":"default"}}`),
			},
		},
	}
}

func invokeAdmission(t *testing.T, w *AdmissionWebhook, review admissionv1.AdmissionReview) admissionv1.AdmissionReview {
	t.Helper()
	body, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("marshal review: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/admission/validate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	w.Handle(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admission status: want 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	var out admissionv1.AdmissionReview
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if out.Response == nil {
		t.Fatal("expected admission response")
	}
	return out
}

func TestAdmissionWebhookParseErrorFailsClosedByDefault(t *testing.T) {
	t.Setenv("FORTUNA_WEBHOOK_FAIL_OPEN", "")
	w := &AdmissionWebhook{}
	out := invokeAdmission(t, w, unsupportedDeploymentAdmissionReview())
	if out.Response.Allowed {
		t.Fatalf("expected malformed admission review to be denied by default")
	}
	if out.Response.Result == nil || out.Response.Result.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden status result, got %#v", out.Response.Result)
	}
}

func TestAdmissionWebhookParseErrorFailOpenRequiresExplicitOptIn(t *testing.T) {
	t.Setenv("FORTUNA_WEBHOOK_FAIL_OPEN", "1")
	w := &AdmissionWebhook{}
	out := invokeAdmission(t, w, unsupportedDeploymentAdmissionReview())
	if !out.Response.Allowed {
		t.Fatalf("expected malformed admission review to be allowed with explicit fail-open opt-in")
	}
}
