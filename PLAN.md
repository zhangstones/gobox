# 兼容性收敛计划 — 已完成

## 完成项

### 1. `watch`：默认覆盖刷新 + `--append`
- 默认模式：清屏覆盖刷新，对齐原生 `watch` 行为；非 TTY 时跳过控制序列。
- `--append`：显式切换为滚动追加输出，不清屏。
- 覆盖：WATCH-001~004 parity，unit 测试，smoke。

### 2. `top -H`：真正线程视图（Linux）
- `captureLinuxThreadSnapshot` 遍历 `/proc/<pid>/task/<tid>/stat`，每个线程独立展示。
- `PID` 列显示 TID；`-p` 过滤、`-u`、`-i`、`--sort`、`-r` 在 `-H` 模式下继续生效。
- 非 Linux 路径保留进程级降级，help 已注明。
- 覆盖：TOP-008 parity 断言多 TID 出现。

### 3. `np -i`：秒语义
- `-i` 公开单位改为秒，支持小数（`float64` → `time.Duration`）。
- help / examples / parity 测试同步更新。

### 4. `netstat -a / -n / -W`：降级为兼容入口
- `-n`：K8s 容器 IP 几乎没有 rDNS 记录，默认数字输出实际上更实用；不做主机名解析，`-n` 保留为 no-op 兼容入口。
- `-a / -W`：同样收窄承诺，help 已说明"默认行为已等效"。
- NETSTAT-005/006 显式断言 `-n` 与默认输出一致，固化当前契约。

### 5. `ps aux`：BSD 语义收敛
- `a/x/u` 组合逻辑通过 `psBSDMode` 结构体实现。
- `u` 模式输出 user-oriented 列布局；`x` 包含无 TTY 进程。

### 6. `ps -o`：高频字段覆盖
- 支持：`pid,ppid,uid,user,comm,cmd,args,pcpu,pmem,rss,vsz,vms,tty,stat,start,etime,time`。
- unsupported 字段明确报错，不静默忽略。

### 7. `ps --sort`：排序键扩展
- 支持：`pid,ppid,cpu/pcpu,pmem,rss,vsz/vms,comm/cmd,user,uid,start,etime,time`。
- GNU 风格 `--sort -FIELD` 降序语义。
- unsupported 字段明确报错，不 silently fallback。
