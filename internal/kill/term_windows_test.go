//go:build windows

package kill

import (
	"os/exec"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// 这是 whoport 里唯一一个"做错了会误杀别人"的代码路径，值得用一个真实进程验证。
func TestTerminateRefusesMismatchedCreationTime(t *testing.T) {
	// ping -n 30 会安静地睡约 30 秒，正好当一个不会自己退出的靶子
	cmd := exec.Command("ping", "-n", "30", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("无法启动测试进程: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	pid := int32(cmd.Process.Pid)

	// 传一个明显错误的创建时间：terminate 必须在真正下手之前中止
	err := terminate(pid, time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local))
	if err == nil {
		t.Fatal("创建时间不匹配时必须拒绝终止，实际却执行了")
	}

	// 确认进程还活着——拒绝不能只是"报了个错"
	p, perr := process.NewProcess(pid)
	if perr != nil {
		t.Fatalf("终止被拒绝后进程却消失了: %v", perr)
	}
	if running, _ := p.IsRunning(); !running {
		t.Error("终止被拒绝，但进程已经不在运行——说明拒绝发生在杀手之后")
	}
}

func TestTerminateSucceedsWithMatchingCreationTime(t *testing.T) {
	cmd := exec.Command("ping", "-n", "30", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("无法启动测试进程: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	pid := int32(cmd.Process.Pid)

	// 用扫描层同样的方式取创建时间（毫秒精度），验证真实路径能走通——
	// 这正是上面那个精度陷阱会破坏的场景。
	p, err := process.NewProcess(pid)
	if err != nil {
		t.Fatalf("NewProcess: %v", err)
	}
	ms, err := p.CreateTime()
	if err != nil {
		t.Fatalf("CreateTime: %v", err)
	}

	if err := terminate(pid, time.UnixMilli(ms)); err != nil {
		t.Fatalf("创建时间匹配时终止应成功，实际失败: %v", err)
	}
}
