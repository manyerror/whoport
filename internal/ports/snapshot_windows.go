//go:build windows

package ports

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// procSnapshot 用 CreateToolhelp32Snapshot 一次性拿到全系统进程名和 PPID。
//
// 为什么不直接用 gopsutil 的 process.Name()：实测在 Windows 上，系统服务
// （svchost.exe、services.exe 等）的 Name()/Exe() 会返回 Access is denied，
// 而这些恰恰是安全护栏最需要识别、最不能误杀的目标。快照这条路不需要任何
// 访问权限，本机实测 371 个进程名字零缺失。
func procSnapshot() (Snapshot, error) {
	h, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(h)

	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))

	if err := windows.Process32First(h, &e); err != nil {
		return nil, err
	}

	out := make(Snapshot, 512)
	for {
		out[int32(e.ProcessID)] = ProcInfo{
			Name: windows.UTF16ToString(e.ExeFile[:]),
			PPID: int32(e.ParentProcessID),
		}
		if err := windows.Process32Next(h, &e); err != nil {
			break
		}
	}
	return out, nil
}
