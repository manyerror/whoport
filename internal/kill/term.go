package kill

import (
	"errors"
	"fmt"
	"time"
)

var errNotSupported = errors.New("该平台暂不支持")

// terminate 打开进程句柄 → 用句柄上的创建时间确认身份 → 终止。
//
// 为什么要确认身份：Windows 会回收并重用 PID。从"扫描到端口"到"执行终止"
// 之间隔着几毫秒到几秒，目标进程可能已经退出、PID 被分配给了另一个进程。
// 比对创建时间，不一致就中止，避免杀错人。
func terminate(pid int32, expected time.Time) error {
	h, err := openForTerminate(pid)
	if err != nil {
		return err
	}
	defer closeHandle(h)

	if !expected.IsZero() {
		if actual, err := creationTime(h); err == nil && !sameProcess(expected, actual) {
			return fmt.Errorf("PID %d 已被系统回收给了另一个进程，已中止以免误杀", pid)
		}
	}
	return terminateHandle(h)
}

// sameProcess 按毫秒比较创建时间。
//
// 不能直接 Equal：扫描时的创建时间来自 gopsutil，精度是毫秒；而句柄上读到的
// FILETIME 精度是 100 纳秒。两者在亚毫秒位上必然不同，直接比会误判成
// "进程已变"，导致每次终止都被自己拦下。
func sameProcess(expected, actual time.Time) bool {
	return expected.UnixMilli() == actual.UnixMilli()
}
