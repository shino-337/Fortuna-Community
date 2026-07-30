package ebpf

import (
	"fmt"
	"os"
)

type PreflightResult struct {
	Root     bool
	HasBTF   bool
	HasBPFFS bool
	Ready    bool
	Reason   string
}

func RunPreflight() PreflightResult {
	res := PreflightResult{}
	res.Root = os.Geteuid() == 0
	_, btfErr := os.Stat("/sys/kernel/btf/vmlinux")
	res.HasBTF = btfErr == nil
	_, bpffsErr := os.Stat("/sys/fs/bpf")
	res.HasBPFFS = bpffsErr == nil
	res.Ready = res.Root && res.HasBTF && res.HasBPFFS
	if res.Ready {
		res.Reason = "ok"
		return res
	}
	res.Reason = fmt.Sprintf("root=%v btf=%v bpffs=%v", res.Root, res.HasBTF, res.HasBPFFS)
	return res
}
