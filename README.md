# whoport

按端口找到占用它的进程并终止 —— **并且告诉你这是谁启的**。

```
$ whoport 3000

  端口 3000 被占用：
    node.exe
    PID     29624
    命令行  D:\NodeJS\node.exe -e "require('http').createServer(...).listen(3000)"
    来源    D:\whoport-demo
    可执行  D:\NodeJS\node.exe
    父进程  bash.exe (1400) ← bash.exe (30692) ← claude.exe (25252)

  ✓ 已终止 node.exe (PID 29624)
```

单个静态编译的 exe，3.2 MB，零运行时依赖。

## 为什么还要再做一个

按端口杀进程这件事已经被解决得很好了 —— [`productdevbook/port-killer`](https://github.com/productdevbook/port-killer)（5.1k star，GUI）、[`jkfran/killport`](https://github.com/jkfran/killport)（1.8k star，Rust CLI）、[`kill-port`](https://github.com/tiaanduplessis/kill-port)（npm）都做得很完整。whoport 不做重复的事，它补的是三个洞：

**1. 光有 PID 不敢下手。** 同时开着三个项目时，`node.exe (PID 1234)` 提供不了任何判断依据。whoport 显示命令行、项目目录和父进程链——上例里能直接看出这个进程是 `claude.exe` 启的。`whoport -l` 的列表里，来源和父进程各占一列，一眼扫过去就知道哪个该杀。

**2. Windows 上没有一个"下个 exe 就能用"的方案。** `killport` 要 cargo 或下二进制，`kill-port` 要 Node ≥16，GUI 项目要装包。whoport 是一个 3.2 MB 的静态二进制，丢进 PATH 完事。

**3. `netstat -ano` 的解析在中文 Windows 上会翻车。** 它的状态字符串是本地化的（中文系统显示"侦听"而不是 `LISTENING`），脚本里 `findstr LISTENING` 直接失效。whoport 走 `GetExtendedTcpTable`，不解析任何本地化文本。

## 安装

从 [Releases](../../releases) 下载对应平台的二进制，解压后放进 `PATH`。

或者用 Go 装：

```bash
go install github.com/manyerror/whoport@latest
```

## 用法

```
whoport                    列出所有监听端口
whoport <端口...>          终止占用这些端口的进程
```

端口支持单个、多个、端口段和混合写法：

```bash
whoport 3000               单个端口
whoport 3000 8080 9090     多个端口
whoport 3000-3010          端口段
whoport 3000,8080,9000-9001  混合写法
```

选项：

| 选项 | 说明 |
|---|---|
| `-l`, `--list` | 列出所有监听端口 |
| `-n`, `--dry-run` | 只显示会杀谁，不真的杀 |
| `-f`, `--force` | 跳过安全护栏 |
| `--json` | 以 JSON 输出，便于脚本消费 |
| `-v`, `--version` | 显示版本 |
| `-h`, `--help` | 显示帮助 |

### 列表

```
$ whoport

PORT   PID    进程           来源                    父进程
135    1512   svchost.exe    -                       services.exe
3000   30704  python.exe     D:\proj\web             bash.exe
3306   6184   mysqld.exe     -                       mysqld.exe
5283   22092  node.exe       D:\proj\api             idea64.exe
6599   22620  dbeaver.exe    D:\DBeaver              explorer.exe
```

### 终止前先看一眼

默认直接杀，但会先把来源打印出来。想只预览不动手：

```bash
whoport -n 3000
```

### JSON

```bash
whoport --json | jq '.[] | select(.port == 3000)'
```

## 安全护栏

以下目标默认拒绝终止，加 `--force` 可越过：

`System`、`[System Process]`、`Registry`、`Memory Compression`、`smss.exe`、`csrss.exe`、`wininit.exe`、`services.exe`、`lsass.exe`、`winlogon.exe`、`svchost.exe`、`fontdrvhost.exe`、`dwm.exe`、`lsaiso.exe`

PID 0 和 PID 4 是 Windows 内核本身，`--force` 也不放行。

终止前还会比对进程创建时间：Windows 会回收并重用 PID，从扫描到下手之间目标可能已经退出、PID 被分配给了别的进程，比对不上就中止，避免误杀。

## 退出码

| 码 | 含义 |
|---|---|
| 0 | 成功，或目标端口本就空闲（目标状态已达成） |
| 1 | 有目标未被终止（被护栏拦下，或执行失败） |
| 2 | 参数错误 |

## 从源码构建

```bash
git clone https://github.com/manyerror/whoport
cd whoport
go build -ldflags="-s -w" -o whoport .
```

纯 Go 无 cgo，交叉编译不需要额外工具链：

```bash
GOOS=linux GOARCH=amd64 go build -o whoport-linux .
```

已实测通过的平台：windows/amd64、windows/arm64、linux/amd64、linux/arm64、darwin/amd64、darwin/arm64。

## 已知限制

- **Windows 上读不到别人进程的详情。** 命令行、工作目录、创建时间依赖读取目标进程的 PEB，对**自己启动的进程**可靠；对系统服务和提权进程会返回 `Access is denied`（此时只有进程名和父进程可用）。这是 Windows 的安全边界，不是 bug。进程名不受影响——走的是 `CreateToolhelp32Snapshot`，不需要任何访问权限。
- **非 Windows 平台的 PID 回收防护尚未实现。** Windows 侧已实现，Linux/macOS 侧会跳过创建时间校验。
- **是硬杀，没有优雅退出。** Windows 没有 SIGTERM，`TerminateProcess` 不触发目标进程的清理逻辑。目标进程来不及落盘的状态会丢。
- **来源是推断出来的，可能为空。** 优先用进程的 CWD，其次是命令行里真实存在的绝对路径。都拿不到就留空（显示 `-`），不会编造。

## License

MIT
