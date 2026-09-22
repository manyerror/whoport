package ports

import (
	"os"
	"path/filepath"
	"strings"
)

// deriveSource 尽力推断"这个进程属于哪个项目目录"——这是 whoport 相对其他
// 端口工具的差异化所在：光知道 node.exe (PID 1234) 不敢下手，知道它来自
// D:\proj\web 才敢。
//
// 优先用进程的 CWD；CWD 拿不到或指向系统/临时目录时，退而从命令行里找真实
// 存在的路径。都推断不出就返回空串——宁可留空，也不编造一个看起来合理的目录。
func deriveSource(cwd, cmdline, exePath string) string {
	if dir := usableDir(cwd); dir != "" {
		return dir
	}
	return dirFromCmdline(cmdline, exePath)
}

// usableDir 过滤掉系统目录和临时目录：它们能读到，但对"这是哪个项目"没有信息量。
// 实测 python -m http.server 从 /tmp 启动时 CWD 就是临时目录，正是这种情况。
func usableDir(p string) string {
	if p == "" {
		return ""
	}
	clean := filepath.Clean(p)
	low := strings.ToLower(clean)

	for _, junk := range junkDirs() {
		if junk == "" {
			continue
		}
		j := strings.ToLower(filepath.Clean(junk))
		if low == j || strings.HasPrefix(low, j+string(filepath.Separator)) {
			return ""
		}
	}
	return clean
}

// junkDirs 返回需要排除的目录。取不到的环境变量返回空串，由调用方跳过，
// 所以同一份代码在 Windows / Linux / macOS 上都能用。
func junkDirs() []string {
	return []string{
		os.Getenv("SystemRoot"),
		os.Getenv("windir"),
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("ProgramData"),
		os.Getenv("TEMP"),
		os.Getenv("TMP"),
		"/tmp",
		"/var",
		"/usr",
		"/etc",
		"/private/var",
		"/System",
	}
}

// dirFromCmdline 从命令行里挑出第一个真实存在的绝对路径。
// 开发服务器的命令行几乎总含项目路径（vite dev、next dev、python app.py 等）。
//
// 必须跳过可执行文件本身（cmdline[0] 通常就是它）：`python.exe -m http.server`
// 这类命令行里根本没有项目路径，若不跳过就会把解释器安装目录当成"来源"报出去，
// 比留空更容易误导人。
//
// 只认绝对路径：相对路径会以 whoport 自己的 CWD 为基准去 stat，得出的结论
// 大概率是错的，不如不猜。
func dirFromCmdline(cmdline, exePath string) string {
	exe := strings.ToLower(filepath.Clean(exePath))

	for _, tok := range splitArgs(cmdline) {
		tok = strings.Trim(tok, `"'`)
		if !isAbsPath(tok) {
			continue
		}
		if exe != "." && strings.ToLower(filepath.Clean(tok)) == exe {
			continue
		}
		info, err := os.Stat(tok)
		if err != nil {
			continue
		}
		if info.IsDir() {
			return filepath.Clean(tok)
		}
		return filepath.Dir(tok)
	}
	return ""
}

func isAbsPath(s string) bool {
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, `\\`) {
		return true
	}
	// Windows 盘符：C:\... 或 C:/...
	return len(s) >= 3 && s[1] == ':' && (s[2] == '\\' || s[2] == '/')
}

// splitArgs 按空白切分命令行，引号内的空格不切分。
func splitArgs(s string) []string {
	var (
		out   []string
		cur   strings.Builder
		quote rune
	)
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}

	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}
