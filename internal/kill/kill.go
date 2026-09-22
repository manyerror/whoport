// Package kill 负责判断一个目标该不该杀，以及怎么杀。
package kill

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/manyerror/whoport/internal/ports"
)

// ErrProtected 表示目标被安全护栏拦下，而不是执行失败。
var ErrProtected = errors.New("被安全护栏拦下")

// criticalNames 是杀掉会导致系统不稳定甚至蓝屏的进程。
// 键是进程名小写，值是用来说明"这是什么"的中文描述，直接呈现给用户。
//
// 名单能生效的前提是拿得到进程名——这正是扫描层要用 CreateToolhelp32Snapshot
// 而不是 gopsutil process.Name() 的原因：后者对 svchost.exe 这类目标会
// 返回 Access is denied，护栏会形同虚设。
var criticalNames = map[string]string{
	"system":             "Windows 内核",
	"[system process]":   "Windows 内核",
	"registry":           "注册表",
	"memory compression": "内存压缩",
	"smss.exe":           "会话管理器",
	"csrss.exe":          "客户端服务器运行时",
	"wininit.exe":        "Windows 启动初始化",
	"services.exe":       "服务控制管理器",
	"lsass.exe":          "本地安全认证",
	"winlogon.exe":       "登录管理器",
	"svchost.exe":        "系统服务宿主，它承载着大量系统服务",
	"fontdrvhost.exe":    "字体驱动宿主",
	"dwm.exe":            "桌面窗口管理器",
	"lsaiso.exe":         "LSA 隔离进程",
}

// Check 判断目标是否允许终止。force 为 true 时跳过关键进程护栏。
//
// PID 0 和 4 永远拒绝：它们是内核本身，不是可以"强制"的对象，--force 也不放行。
func Check(l ports.Listener, force bool) error {
	if l.PID == 0 || l.PID == 4 {
		return fmt.Errorf("%w：%s (PID %d) 是 Windows 内核进程，无法终止",
			ErrProtected, display(l.Process), l.PID)
	}
	if l.PID == int32(os.Getpid()) {
		return fmt.Errorf("%w：这是 whoport 自己 (PID %d)", ErrProtected, l.PID)
	}
	if force {
		return nil
	}
	if reason, bad := criticalNames[strings.ToLower(l.Process)]; bad {
		return fmt.Errorf("%w：%s (PID %d) 是%s，终止它可能导致系统不稳定\n    确认要杀请加 --force",
			ErrProtected, l.Process, l.PID, reason)
	}
	return nil
}

// Kill 终止目标进程。调用方应先用 Check 判断是否允许。
func Kill(l ports.Listener) error {
	return terminate(l.PID, l.Created)
}

func display(name string) string {
	if name == "" {
		return "未知进程"
	}
	return name
}
