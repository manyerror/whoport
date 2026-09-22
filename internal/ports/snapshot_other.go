//go:build !windows

package ports

import "github.com/shirou/gopsutil/v4/process"

// procSnapshot 在非 Windows 平台上退回 gopsutil 遍历。
// Linux/macOS 没有 Windows 那样的跨用户访问限制，Name() 和 Ppid() 基本都能拿到。
func procSnapshot() (Snapshot, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	out := make(Snapshot, len(procs))
	for _, p := range procs {
		name, _ := p.Name()
		ppid, _ := p.Ppid()
		out[p.Pid] = ProcInfo{Name: name, PPID: ppid}
	}
	return out, nil
}
