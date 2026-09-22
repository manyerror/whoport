package kill

import (
	"testing"
	"time"
)

func TestSameProcessComparesAtMillisecondPrecision(t *testing.T) {
	// 扫描时的创建时间来自 gopsutil（毫秒精度），而句柄上读到的 FILETIME 是
	// 100 纳秒精度。两者在亚毫秒位上必然不同——若直接用 Equal 比较，同一个
	// 进程会被误判成"已被回收"，导致每一次终止都被自己拦下。
	base := time.Date(2026, 9, 22, 16, 26, 5, 345_000_000, time.Local)
	fromFiletime := base.Add(400 * time.Nanosecond)

	if !sameProcess(base, fromFiletime) {
		t.Errorf("同一个进程的亚毫秒差异不应判为不同进程：%v vs %v", base, fromFiletime)
	}
	if sameProcess(base, base.Add(time.Millisecond)) {
		t.Error("相差 1 毫秒应判为不同进程")
	}
	if sameProcess(base, base.Add(time.Second)) {
		t.Error("相差 1 秒应判为不同进程")
	}
}
