//go:build !windows

package kill

import (
	"fmt"
	"os"
	"time"
)

type procHandle = *os.Process

func openForTerminate(pid int32) (procHandle, error) {
	p, err := os.FindProcess(int(pid))
	if err != nil {
		return nil, fmt.Errorf("找不到进程 PID %d: %w", pid, err)
	}
	return p, nil
}

// creationTime 在非 Windows 平台暂未实现，返回错误让调用方跳过身份校验。
// 一期以 Windows 为主；Linux 可以从 /proc/<pid>/stat 的 starttime 取，留待后续。
func creationTime(procHandle) (time.Time, error) {
	return time.Time{}, errNotSupported
}

func terminateHandle(p procHandle) error {
	return p.Kill()
}

func closeHandle(procHandle) {}
