// Package cli 负责参数解析和命令分发。
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/manyerror/whoport/internal/kill"
	"github.com/manyerror/whoport/internal/output"
	"github.com/manyerror/whoport/internal/ports"
	"golang.org/x/term"
)

// version 由构建时通过 -ldflags "-X .../cli.version=v1.2.3" 注入。
// 源码里的默认值是给 go build / go run 直接构建时用的，GoReleaser 会覆盖它。
// 必须是 var 而不是 const——链接器只能改写变量。
var version = "dev"

type options struct {
	list        bool
	dryRun      bool
	force       bool
	asJSON      bool
	showVersion bool
	ports       []uint16
}

const usageText = `whoport — 按端口定位进程并终止它，并告诉你这是谁启的

用法：
  whoport                    列出所有监听端口
  whoport <端口...>          终止占用这些端口的进程

端口写法：
  3000                       单个端口
  3000 8080 9090             多个端口
  3000-3010                  端口段
  3000,8080,9000-9001        混合写法

选项：
  -l, --list                 列出所有监听端口
  -n, --dry-run              只显示会杀谁，不真的杀
  -f, --force                跳过安全护栏（系统关键进程默认拒绝）
      --json                 以 JSON 输出，便于脚本消费
  -v, --version              显示版本
  -h, --help                 显示帮助

示例：
  whoport                    看看现在有哪些端口被占
  whoport 3000               杀掉占用 3000 的进程
  whoport -n 3000            先看看 3000 是谁占的，别急着杀
`

// Run 执行一次命令行调用，返回进程退出码。
// 退出码：0 成功；1 有目标未被终止（护栏拦下或执行失败）；2 参数错误。
func Run(args []string, stdout, stderr io.Writer) int {
	o, err := parse(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usageText)
			return 0
		}
		return 2
	}
	if o.showVersion {
		fmt.Fprintf(stdout, "whoport %s\n", version)
		return 0
	}

	style := output.NewStyle(isTerminal(stdout))

	ls, err := ports.Scan()
	if err != nil {
		fmt.Fprintf(stderr, "扫描端口失败：%v\n", err)
		return 1
	}

	if o.list || len(o.ports) == 0 {
		if o.asJSON {
			return writeJSON(stdout, stderr, ls)
		}
		output.List(stdout, style, ls)
		return 0
	}

	results := make([]output.Result, 0, len(o.ports))
	code := 0

	for _, port := range o.ports {
		targets := ports.Find(ls, port)
		if len(targets) == 0 {
			if !o.asJSON {
				output.NotOccupied(stdout, style, port)
			}
			results = append(results, output.Result{Port: port, Status: "not_occupied"})
			continue
		}

		for _, t := range targets {
			if !o.asJSON {
				output.Detail(stdout, style, t)
			}

			res := output.Result{Port: port, PID: t.PID, Process: t.Process, Source: t.Source}

			if err := kill.Check(t, o.force); err != nil {
				if !o.asJSON {
					output.Refused(stdout, style, err)
				}
				res.Status, res.Reason = "refused", err.Error()
				results = append(results, res)
				code = 1
				continue
			}

			if o.dryRun {
				if !o.asJSON {
					output.WouldKill(stdout, style, t)
				}
				res.Status = "dry_run"
				results = append(results, res)
				continue
			}

			if err := kill.Kill(t); err != nil {
				if !o.asJSON {
					output.Failed(stdout, style, t, err)
				}
				res.Status, res.Reason = "failed", err.Error()
				results = append(results, res)
				code = 1
				continue
			}

			if !o.asJSON {
				output.Killed(stdout, style, t)
			}
			res.Status = "killed"
			results = append(results, res)
		}
	}

	if o.asJSON {
		return writeJSON(stdout, stderr, results)
	}
	return code
}

func writeJSON(stdout, stderr io.Writer, v any) int {
	if err := output.WriteJSON(stdout, v); err != nil {
		fmt.Fprintf(stderr, "输出 JSON 失败：%v\n", err)
		return 1
	}
	return 0
}

func parse(args []string, stderr io.Writer) (options, error) {
	var o options

	fs := flag.NewFlagSet("whoport", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usageText) }

	fs.BoolVar(&o.list, "l", false, "")
	fs.BoolVar(&o.list, "list", false, "")
	fs.BoolVar(&o.dryRun, "n", false, "")
	fs.BoolVar(&o.dryRun, "dry-run", false, "")
	fs.BoolVar(&o.force, "f", false, "")
	fs.BoolVar(&o.force, "force", false, "")
	fs.BoolVar(&o.asJSON, "json", false, "")
	fs.BoolVar(&o.showVersion, "v", false, "")
	fs.BoolVar(&o.showVersion, "version", false, "")

	// flag 包遇到第一个非选项参数就停止解析，而 `whoport 3000 -n` 这种写法很自然。
	// 逐段解析，把选项和端口参数都收进来。
	var specs []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return o, err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		specs = append(specs, rest[0])
		rest = rest[1:]
	}

	ps, err := parsePortSpecs(specs)
	if err != nil {
		fmt.Fprintf(stderr, "whoport: %v\n", err)
		return o, err
	}
	o.ports = ps
	return o, nil
}

// parsePortSpecs 解析端口参数，支持 "3000"、"3000,8080"、"3000-3010" 及混合写法。
func parsePortSpecs(specs []string) ([]uint16, error) {
	set := map[uint16]bool{}

	for _, spec := range specs {
		for _, part := range strings.Split(spec, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			lo, hi, isRange := strings.Cut(part, "-")
			start, err := parsePort(lo)
			if err != nil {
				return nil, err
			}
			end := start
			if isRange {
				if end, err = parsePort(hi); err != nil {
					return nil, err
				}
			}
			if start > end {
				return nil, fmt.Errorf("端口范围 %q 的起点大于终点", part)
			}

			for p := start; ; p++ {
				set[p] = true
				// 不 break 的话 p 会在 65535 溢出回 0，变成死循环
				if p == end || p == 65535 {
					break
				}
			}
		}
	}

	out := make([]uint16, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func parsePort(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%q 不是合法的端口号", s)
	}
	if n < 1 || n > 65535 {
		return 0, fmt.Errorf("端口号 %d 超出范围（1-65535）", n)
	}
	return uint16(n), nil
}

// isTerminal 决定要不要上色。重定向到文件或管道时自动降级为纯文本。
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
