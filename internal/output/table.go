package output

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/manyerror/whoport/internal/ports"
)

const (
	colGap      = "  "
	maxSourceW  = 44
	maxProcessW = 22
)

// List 打印监听端口总览。
func List(w io.Writer, s Style, ls []ports.Listener) {
	if len(ls) == 0 {
		fmt.Fprintln(w, s.Dim("没有正在监听的端口"))
		return
	}

	headers := []string{"PORT", "PID", "进程", "来源", "父进程"}
	rows := make([][]string, 0, len(ls))
	for _, l := range ls {
		rows = append(rows, []string{
			strconv.Itoa(int(l.Port)),
			strconv.Itoa(int(l.PID)),
			truncate(orDash(l.Process), maxProcessW),
			truncate(orDash(l.Source), maxSourceW),
			orDash(immediateParent(l)),
		})
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = displayWidth(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if cw := displayWidth(c); cw > widths[i] {
				widths[i] = cw
			}
		}
	}

	head := make([]string, len(headers))
	for i, h := range headers {
		head[i] = s.Bold(pad(h, widths[i]))
	}
	fmt.Fprintln(w, joinCells(head, widths))

	for _, r := range rows {
		cells := make([]string, len(r))
		for i, c := range r {
			cell := pad(c, widths[i])
			switch i {
			case 0:
				cell = s.Cyan(cell)
			case 2:
				cell = s.Bold(cell)
			case 3, 4:
				cell = s.Dim(cell)
			}
			cells[i] = cell
		}
		fmt.Fprintln(w, joinCells(cells, widths))
	}
}

// joinCells 拼接单元格，最后一列不补空格，避免行尾拖一串空白。
func joinCells(cells []string, widths []int) string {
	var b strings.Builder
	for i, c := range cells {
		if i > 0 {
			b.WriteString(colGap)
		}
		if i < len(cells)-1 {
			b.WriteString(pad(c, widths[i]))
		} else {
			b.WriteString(c)
		}
	}
	return b.String()
}

// Detail 打印单个目标的来源详情——这是"敢不敢杀"的判断依据。
func Detail(w io.Writer, s Style, l ports.Listener) {
	fmt.Fprintf(w, "\n  端口 %s 被占用：\n", s.Cyan(strconv.Itoa(int(l.Port))))
	fmt.Fprintf(w, "    %s\n", s.Bold(orDash(l.Process)))

	row := func(label, val string) {
		if val == "" {
			return
		}
		fmt.Fprintf(w, "    %s%s\n", s.Dim(pad(label, 8)), val)
	}
	row("PID", strconv.Itoa(int(l.PID)))
	row("命令行", l.Cmdline)
	row("来源", l.Source)
	row("可执行", l.ExePath)
	row("父进程", parentChainText(l))
}

// Killed 报告一次成功的终止。
func Killed(w io.Writer, s Style, l ports.Listener) {
	fmt.Fprintf(w, "  %s 已终止 %s (PID %d)\n", s.Green("✓"), s.Bold(orDash(l.Process)), l.PID)
}

// WouldKill 报告 --dry-run 下本应被终止的目标。
func WouldKill(w io.Writer, s Style, l ports.Listener) {
	fmt.Fprintf(w, "  %s %s (PID %d) %s\n",
		s.Yellow("·"), s.Bold(orDash(l.Process)), l.PID, s.Dim("未终止（--dry-run）"))
}

// Refused 报告被安全护栏拦下的目标。
func Refused(w io.Writer, s Style, err error) {
	fmt.Fprintf(w, "  %s %s\n", s.Yellow("!"), err)
}

// Failed 报告终止失败。
func Failed(w io.Writer, s Style, l ports.Listener, err error) {
	fmt.Fprintf(w, "  %s 终止 %s (PID %d) 失败：%s\n",
		s.Red("✗"), orDash(l.Process), l.PID, err)
}

// NotOccupied 报告端口空闲——这不是错误，是目标状态已达成。
func NotOccupied(w io.Writer, s Style, port uint16) {
	fmt.Fprintf(w, "  %s 端口 %d 没有被占用\n", s.Dim("·"), port)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func immediateParent(l ports.Listener) string {
	if len(l.Parents) == 0 {
		return ""
	}
	return l.Parents[0].Name
}

// parentChainText 把父进程链渲染成 "a (1) ← b (2)" 的形式。
// 链尾通常就是终端或 IDE，能直接回答"这是谁启的"。
func parentChainText(l ports.Listener) string {
	const show = 4

	parts := make([]string, 0, show)
	for i, p := range l.Parents {
		if i >= show {
			parts = append(parts, "…")
			break
		}
		parts = append(parts, fmt.Sprintf("%s (%d)", orDash(p.Name), p.PID))
	}
	return strings.Join(parts, " ← ")
}
