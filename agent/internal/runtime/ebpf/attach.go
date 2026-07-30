package ebpf

import (
	"log"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/link"
	"github.com/fortuna/agent/internal/runtime"
)

func (s *Sensor) attachSelectedTracepoints() {
	attach := func(category, name, syscall string) {
		lnk, err := attachNoopTracepoint(category, name)
		if err != nil {
			log.Printf("[eBPF] attach failed %s/%s: %v", category, name, err)
			return
		}
		s.links = append(s.links, lnk)
		log.Printf("[eBPF] attached tracepoint %s/%s", category, name)
		// Emit a one-shot attach event for end-to-end pipeline verification.
		s.enqueueEvent(runtime.Event{
			EventType:      "runtime.ebpf.attach",
			MitreTechnique: "T1068",
			Signal:         "EBPF_ATTACH_EVENT",
			Severity:       "low",
			Runtime:        "ebpf",
			Syscall:        syscall,
			Capability:     "EBPF_ATTACH",
			Target:         category + "/" + name,
			Timestamp:      time.Now().Unix(),
			Pod:            map[string]interface{}{"uid": s.podUID, "node": s.nodeName},
		})
	}

	if s.mode == "exec" || s.mode == "all" {
		attach("syscalls", "sys_enter_execve", "execve")
	}
	if s.mode == "connect" || s.mode == "all" {
		attach("syscalls", "sys_enter_connect", "connect")
	}
}

func (s *Sensor) closeLinks() {
	for _, l := range s.links {
		_ = l.Close()
	}
	s.links = nil
}

func attachNoopTracepoint(category, name string) (link.Link, error) {
	spec := &ebpf.ProgramSpec{
		Type: ebpf.TracePoint,
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, 0),
			asm.Return(),
		},
		License: "GPL",
	}
	prog, err := ebpf.NewProgram(spec)
	if err != nil {
		return nil, err
	}
	lnk, err := link.Tracepoint(category, name, prog, nil)
	if err != nil {
		_ = prog.Close()
		return nil, err
	}
	return &linkWithProgram{Link: lnk, prog: prog}, nil
}

type linkWithProgram struct {
	link.Link
	prog *ebpf.Program
}

func (l *linkWithProgram) Close() error {
	_ = l.Link.Close()
	if l.prog != nil {
		return l.prog.Close()
	}
	return nil
}
