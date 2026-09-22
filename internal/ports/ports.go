// Package ports 负责回答两个问题：哪些端口正在被监听，以及监听它的是谁。
package ports

import (
	"fmt"
	"sort"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// ProcInfo 是进程快照里的一项。
type ProcInfo struct {
	Name string
	PPID int32
}

// Snapshot 是全系统进程名 + PPID 的快照，按 PID 索引。
type Snapshot map[int32]ProcInfo

// Parent 是父进程链上的一环。
type Parent struct {
	PID  int32  `json:"pid"`
	Name string `json:"name"`
}

// Listener 是一个正在监听的端口，以及占用它的进程。
type Listener struct {
	Port     uint16   `json:"port"`
	Protocol string   `json:"protocol"`
	PID      int32    `json:"pid"`
	Process  string   `json:"process"`
	ExePath  string   `json:"exe_path,omitempty"`
	Cmdline  string   `json:"cmdline,omitempty"`
	Source   string   `json:"source,omitempty"`
	Parents  []Parent `json:"parents,omitempty"`

	// Created 用于终止前校验进程身份，防止 PID 被系统回收后杀错进程。
	Created time.Time `json:"created,omitempty"`

	// Detail 表示能否读到该进程的详细信息（命令行、CWD 等）。
	// 只有自己启动的进程可读；系统服务会返回 Access is denied，此时仅名字和 PPID 可用。
	Detail bool `json:"detail"`
}

// Scan 返回所有正在监听的 TCP 端口，按端口号排序。
func Scan() ([]Listener, error) {
	snap, err := procSnapshot()
	if err != nil {
		return nil, fmt.Errorf("读取进程快照失败: %w", err)
	}

	type key struct {
		port uint16
		pid  int32
	}
	seen := map[key]bool{}
	var out []Listener

	for _, proto := range []string{"tcp", "tcp6"} {
		conns, err := gnet.Connections(proto)
		if err != nil {
			// 某些机器/容器里 tcp6 不可用，不影响 tcp 结果
			continue
		}
		for _, c := range conns {
			if c.Status != "LISTEN" {
				continue
			}
			k := key{uint16(c.Laddr.Port), c.Pid}
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, build(proto, uint16(c.Laddr.Port), c.Pid, snap))
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].PID < out[j].PID
	})
	return out, nil
}

// Find 返回监听指定端口的所有进程。
func Find(ls []Listener, port uint16) []Listener {
	var out []Listener
	for _, l := range ls {
		if l.Port == port {
			out = append(out, l)
		}
	}
	return out
}

func build(proto string, port uint16, pid int32, snap Snapshot) Listener {
	l := Listener{Port: port, Protocol: proto, PID: pid}

	if info, ok := snap[pid]; ok {
		l.Process = info.Name
	}

	// 进程详情只有自己启动的进程读得到；读不到不是错误，只是信息少一些。
	if p, err := process.NewProcess(pid); err == nil {
		if cmd, err := p.Cmdline(); err == nil && cmd != "" {
			l.Cmdline = cmd
			l.Detail = true
		}
		if exe, err := p.Exe(); err == nil {
			l.ExePath = exe
		}
		var cwd string
		if c, err := p.Cwd(); err == nil {
			cwd = c
		}
		if ct, err := p.CreateTime(); err == nil {
			l.Created = time.UnixMilli(ct)
		}
		l.Source = deriveSource(cwd, l.Cmdline, l.ExePath)
	}

	l.Parents = parentChain(pid, snap)
	return l
}

// parentChain 沿 PPID 向上走，最多 maxDepth 层。
// 实测 Windows 上存在真实的父子环（services.exe 链上成环），所以必须做环检测。
func parentChain(pid int32, snap Snapshot) []Parent {
	const maxDepth = 6

	var out []Parent
	seen := map[int32]bool{pid: true}
	cur := pid

	for i := 0; i < maxDepth; i++ {
		info, ok := snap[cur]
		if !ok || info.PPID == 0 || info.PPID == cur || seen[info.PPID] {
			break
		}
		parent, ok := snap[info.PPID]
		if !ok {
			break
		}
		out = append(out, Parent{PID: info.PPID, Name: parent.Name})
		seen[info.PPID] = true
		cur = info.PPID
	}
	return out
}
