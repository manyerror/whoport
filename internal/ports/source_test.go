package ports

import (
	"os"
	"path/filepath"
	"testing"
)

// testDir 建一个不在系统临时目录下的目录。
//
// 不能用 t.TempDir()：它建在 %TEMP% 下，而 usableDir 的职责之一就是过滤掉
// 临时目录，拿它当"项目目录"会和被测逻辑直接冲突。
func testDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp(".", "whoport-src-test-")
	if err != nil {
		t.Fatalf("建测试目录失败: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("取绝对路径失败: %v", err)
	}
	return abs
}

func TestDeriveSourcePrefersCwd(t *testing.T) {
	dir := testDir(t)
	got := deriveSource(dir, `node C:\other\place\server.js`, `D:\NodeJS\node.exe`)
	if got != dir {
		t.Errorf("应优先采用 CWD %q，实际得到 %q", dir, got)
	}
}

// 这是实测踩到的坑：python.exe -m http.server 的命令行里根本没有项目路径，
// 若不跳过可执行文件本身，就会把解释器安装目录当成"来源"报出去——比留空
// 更容易误导人。
func TestDeriveSourceSkipsExecutablePath(t *testing.T) {
	exe := filepath.Join(testDir(t), "python.exe")

	if got := deriveSource("", `"`+exe+`" -m http.server 3000`, exe); got != "" {
		t.Errorf("命令行里只有解释器路径时不应报出该目录，实际得到 %q", got)
	}
}

func TestDeriveSourceFallsBackToCmdlinePath(t *testing.T) {
	projDir := testDir(t)
	exe := filepath.Join(testDir(t), "node.exe")
	script := filepath.Join(projDir, "server.js")

	// 路径必须真实存在才会被采信——这是刻意的设计：宁可留空，也不凭命令行
	// 里一个已失效的路径编造来源。
	if err := os.WriteFile(script, nil, 0o600); err != nil {
		t.Fatalf("建测试脚本失败: %v", err)
	}

	got := deriveSource("", `"`+exe+`" "`+script+`"`, exe)
	if got != projDir {
		t.Errorf("应从命令行脚本路径推出 %q，实际得到 %q", projDir, got)
	}
}

func TestDeriveSourceIgnoresNonexistentCmdlinePath(t *testing.T) {
	exe := filepath.Join(testDir(t), "node.exe")
	missing := filepath.Join(testDir(t), "nope", "server.js")

	if got := deriveSource("", `"`+exe+`" "`+missing+`"`, exe); got != "" {
		t.Errorf("不存在的路径不应被采信，实际得到 %q", got)
	}
}

func TestUsableDirRejectsJunk(t *testing.T) {
	// 临时目录能读到，但对"这是哪个项目"没有信息量
	if got := usableDir(t.TempDir()); got != "" {
		t.Errorf("临时目录应被过滤掉，实际返回 %q", got)
	}
	if got := usableDir(os.TempDir()); got != "" {
		t.Errorf("系统临时目录应被过滤掉，实际返回 %q", got)
	}
}

func TestDeriveSourceEmptyWhenNothingKnown(t *testing.T) {
	if got := deriveSource("", `node server.js`, ""); got != "" {
		t.Errorf("无从推断时应返回空串而不是编造，实际得到 %q", got)
	}
}

func TestSplitArgsRespectsQuotes(t *testing.T) {
	got := splitArgs(`"C:\Program Files\a.exe" --flag plain`)
	want := []string{`C:\Program Files\a.exe`, "--flag", "plain"}

	if len(got) != len(want) {
		t.Fatalf("splitArgs 得到 %v，期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 项 = %q，期望 %q", i, got[i], want[i])
		}
	}
}
