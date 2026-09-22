//go:build windows

package kill

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/windows"
)

type procHandle = windows.Handle

// openForTerminate 打开进程句柄。
//
// 用 windows.OpenProcess 而不是 os.FindProcess().Kill()：后者在 Windows 上
// 是空实现，"找到进程"永远成功，也拿不到精确错误码——而区分"权限不足"和
// "进程不存在"正是给用户有用提示的关键。
func openForTerminate(pid int32) (procHandle, error) {
	h, err := windows.OpenProcess(
		windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false, uint32(pid),
	)
	if err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return 0, fmt.Errorf("权限不足，无法终止 PID %d\n    它可能以管理员或 SYSTEM 身份运行，请用管理员身份重开终端再试", pid)
		}
		return 0, fmt.Errorf("打开进程 PID %d 失败: %w", pid, err)
	}
	return h, nil
}

func creationTime(h procHandle) (time.Time, error) {
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, creation.Nanoseconds()), nil
}

func terminateHandle(h procHandle) error {
	return windows.TerminateProcess(h, 1)
}

func closeHandle(h procHandle) {
	_ = windows.CloseHandle(h)
}
