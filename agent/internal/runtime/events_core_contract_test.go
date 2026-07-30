package runtime

import (
	"encoding/json"
	"testing"
	"time"
)

// Core unmarshals POST /api/v1/runtime/events into runtimeEventPayload (see fortuna/core/internal/api/runtime_event_handlers.go).
// Agent Reader and eBPF sensor must emit JSON where pod.uid, syscall, and target/target_path are visible to that struct.
func TestEvent_JSON_CoreIngestContract(t *testing.T) {
	ev := Event{
		EventType: "runtime.exec",
		Pod: map[string]interface{}{
			"uid":       "11111111-1111-1111-1111-111111111111",
			"namespace": "default",
			"name":      "demo",
		},
		Syscall:   "open",
		Target:    "/proc/1/root",
		Timestamp: time.Now().Unix(),
	}
	b, err := json.Marshal([]Event{ev})
	if err != nil {
		t.Fatal(err)
	}

	var batch []map[string]interface{}
	if err := json.Unmarshal(b, &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch) != 1 {
		t.Fatalf("batch len %d", len(batch))
	}
	row := batch[0]
	pod, ok := row["pod"].(map[string]interface{})
	if !ok {
		t.Fatalf("pod missing: %#v", row)
	}
	if pod["uid"] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("pod.uid: %v", pod["uid"])
	}
	if row["syscall"] != "open" {
		t.Fatalf("syscall: %v", row["syscall"])
	}
	if row["target"] != "/proc/1/root" {
		t.Fatalf("target: %v", row["target"])
	}
}
