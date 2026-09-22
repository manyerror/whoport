package output

import (
	"strings"
)

// Style 负责上色。enabled 为 false 时所有方法原样返回，保证重定向到文件或
// 管道时输出干净——这是 CLI 工具的基本礼貌。
type Style struct{ enabled bool }

func NewStyle(enabled bool) Style { return Style{enabled: enabled} }

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

func (s Style) wrap(code, t string) string {
	if !s.enabled || t == "" {
		return t
	}
	return code + t + ansiReset
}

func (s Style) Bold(t string) string   { return s.wrap(ansiBold, t) }
func (s Style) Dim(t string) string    { return s.wrap(ansiDim, t) }
func (s Style) Red(t string) string    { return s.wrap(ansiRed, t) }
func (s Style) Green(t string) string  { return s.wrap(ansiGreen, t) }
func (s Style) Yellow(t string) string { return s.wrap(ansiYellow, t) }
func (s Style) Cyan(t string) string   { return s.wrap(ansiCyan, t) }

// runeWidth 返回字符在等宽终端里占的列数。
//
// 中日韩文字占 2 列，其余占 1 列。不做这个区分，中文表头下面的表格会整体错位。
// 不引 golang.org/x/text/width 是因为这里只需要一个粗略但正确的判断。
func runeWidth(r rune) int {
	switch {
	case r < 0x1100:
		return 1
	case r >= 0x1100 && r <= 0x115F, // 韩文字母
		r >= 0x2E80 && r <= 0xA4CF, // 中日韩部首、假名、汉字
		r >= 0xAC00 && r <= 0xD7A3, // 韩文音节
		r >= 0xF900 && r <= 0xFAFF, // 兼容汉字
		r >= 0xFE30 && r <= 0xFE6F, // 兼容形式
		r >= 0xFF00 && r <= 0xFF60, // 全角字符
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x20000 && r <= 0x3FFFD: // 汉字扩展区
		return 2
	}
	return 1
}

func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// pad 按显示宽度右侧补空格。
func pad(s string, w int) string {
	if d := w - displayWidth(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// truncate 按显示宽度截断，超出部分用 … 收尾。
func truncate(s string, max int) string {
	if displayWidth(s) <= max {
		return s
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := runeWidth(r)
		if w+rw > max-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
}
