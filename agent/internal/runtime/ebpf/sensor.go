package ebpf

import (
	"context"
	"log"

	ciliumebpf "github.com/cilium/ebpf"
)

// Sensor is phase-1 scaffold for R9.
// Current behavior is fail-open preflight only; runtime pipeline keeps working even if eBPF is unavailable.
type Sensor struct{}

func NewSensor() *Sensor {
	return &Sensor{}
}

func (s *Sensor) Start(ctx context.Context) {
	_ = ctx
	var opts ciliumebpf.CollectionOptions
	_ = opts
	pf := RunPreflight()
	if !pf.Ready {
		log.Printf("[eBPF] preflight not ready (%s); fail-open: disabling eBPF sensor", pf.Reason)
		return
	}
	log.Printf("[eBPF] sensor scaffold enabled (phase-1 preflight passed)")
}
