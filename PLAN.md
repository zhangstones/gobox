# Bug Fix Backlog

Issues found by automated code review. Fix order: 高危 → 中危 → 低危.

---

## 高危（数据错误 / 崩溃 / 死锁）

- [x] `fs/cmd_df.go:268` — used 用 Bavail 而非 Bfree 计算，高估已用空间
- [x] `fs/cmd_find.go:90` — WalkDir 所有错误静默吞掉，不存在的根路径无输出不报错
- [x] `fs/cmd_find.go:242` — glob 中 `[`/`]` 被转义为字面量，字符类表达式永远不匹配
- [x] `fs/cmd_truncate.go:79` — O_CREATE 未带写权限标志
- [x] `net/cmd_curl.go:350` — `-F "field=value"` 把普通值当文件名读，plain-text 表单字段完全失效
- [x] `net/cmd_curl.go:535` — 响应行输出 `HTTP/1.1 200 200 OK`（状态码重复）
- [x] `net/cmd_nc.go:104/113/134` — `--concurrent=` / `--requests=` / `--interval=` 切片下标差 1，三个参数永远无法生效
- [x] `net/cmd_netstat.go:1001` — IPv6 地址字节序未做 4 字节 swap，输出全部错误
- [x] `net/cmd_tw.go:204` — 目录遍历漏洞：HasPrefix 允许访问同前缀兄弟目录
- [x] `proc/cmd_free.go:93/96` — uint64 无符号减法下溢，输出天文数字
- [x] `proc/cmd_lsof.go:241` — TCP 地址显示原始 hex 未转换成 IP 字符串
- [x] `proc/cmd_ps.go:1112` — procClockTicks 硬编码 100（Linux USER_HZ 始终为 100，此项无需修改）
- [x] `proc/cmd_top.go:196/200` — 非 Linux fallback 调用 Linux-only 函数；diffProcSnapshots(curr,curr) CPU 永远 0%
- [x] `proc/cmd_xargs.go:154` — `-P 0` 创建容量 0 的 channel 永久死锁
- [x] `disk/cmd_md5sum.go:218` — checksum 文件二进制模式 `*filename` 前缀未剥离，校验永远失败
- [x] `text/cmd_grep.go:379` — Unicode 大小写不敏感匹配字节偏移错误，可能 panic
- [x] `text/cmd_head.go:35/62` — `--lines=20` 组合形式静默当作文件名
- [x] `text/cmd_rand.go:62` — `-b 32` 中字节数被静默忽略
- [x] `text/cmd_sort.go:337` — `-r` 比较器违反严格弱序，可能 panic 或非确定性输出
- [x] `text/cmd_tail.go:453` — `--follow=name` 第一次轮询后 defer 关闭 fi.reader，此后报错
- [x] `text/cmd_uniq.go:183` — 按字节数切片而非 rune，多字节 UTF-8 比较错误

---

## 中危（功能错误，不崩溃）

- [x] `fs/cmd_df.go:299` — `-H`（SI）与 `-h`（二进制）输出相同，SI 失效
- [x] `fs/cmd_du.go:200` — 默认输出原始字节而非 1K 块
- [x] `fs/cmd_find.go:253` — pathDepth 根路径下差 1，-maxdepth 1 误剪根的直接子目录
- [~] `fs/cmd_readpath.go:71` — 多路径时 `-n` 被忽略（有意设计：多路径时路径间始终需要分隔符，测试已验证此行为）
- [x] `fs/cmd_stat.go:92` — formatStat map 迭代顺序随机，含格式符的文件名输出不确定
- [x] `fs/cmd_truncate.go:53` — 相对模式 Stat 错误静默，截断目标值错误
- [x] `net/cmd_curl.go:662` — bench 吞吐量用最慢请求延迟计算，结果错误
- [~] `net/cmd_ifstat.go:213` — 首个采样未 sleep（有意设计：有限次运行时第一行立即输出避免卡顿，代码注释已说明）
- [x] `net/cmd_ip.go:84` — 表格模式用数组索引而非内核接口 index，有空洞时编号错误
- [x] `net/cmd_nc.go:601` — benchmark client 硬编码输出 `"Connecting to localhost:8080"`
- [~] `net/cmd_netstat.go:1129` — tcpStateName 先 ToUpper 再 case "0a"（dead code 但不影响正确性，两种格式均有 case）
- [~] `net/cmd_np.go:651` — scan 模式错误计数器从不递增（TCP 扫描无法区分端口关闭和网络错误，需 OS 级错误类型判断，超出当前范围）
- [x] `net/cmd_np.go:763` — ping 摘要硬编码 "netping" 而非实际目标
- [~] `net/cmd_np.go:321` — UDP ping 只测本地 socket 创建时间（UDP 无连接，测量真实 RTT 需自定义协议，超出当前范围）
- [x] `net/cmd_nslookup_dig.go:475` — dig 每次发两次 DNS 请求，显示第一次（被丢弃）的时间
- [x] `net/cmd_nslookup_dig.go:54` — 参数解析阶段发起真实 DNS 查询
- [x] `proc/cmd_free.go:97` — 表头 6 列，数据行 5/3 列，表格对不齐
- [x] `proc/cmd_kill.go:207` — 进程名含空格时 Fields 导致字段偏移，ppid 解析错误
- [x] `proc/cmd_kill.go:175` — regexp.Compile 错误被丢弃，无效正则静默降级
- [~] `proc/cmd_timeout.go:80` — 信号发送失败时错误丢弃（进程已退出时 done channel 会立即关闭，不会阻塞；signal 失败不影响正确性）
- [x] `proc/cmd_top.go:791` — 切换列时排序方向强制降序，覆盖用户设置
- [x] `disk/cmd_ioperf.go:388` — 非 time-based 模式 duration 硬编码 1.0，IOPS 计算错误
- [x] `disk/cmd_ioperf.go:229` — Truncate 错误静默，磁盘满时测试继续
- [x] `disk/cmd_iostat.go:311` — readCgroupV1 两读都失败时返回 nil error
- [x] `disk/cmd_sha256sum.go:150` — check 模式文件不存在只显示 FAILED 不打印原因
- [x] `text/cmd_sed.go:44` — `-i .bak` 备份后缀是死代码，备份功能永远不触发
- [x] `text/cmd_sed.go:121` — in-place 检测靠位置猜测，有其他 flag 时静默跳过写回
- [x] `text/cmd_sort.go:74` — `-h` 触发 human-numeric-sort 而非 help（去掉 help case 中死代码 `-h`，保留 `--help`）
- [x] `text/cmd_wc.go:78` — `-` 被当作未知选项拒绝而非读 stdin

---

## 低危（性能 / 资源泄漏）

- [ ] `fs/cmd_df.go:253` — dfColumnWidths 每行调用，O(n²)
- [ ] `fs/cmd_find.go:225` — regexp.Compile 每次文件访问时重新编译
- [ ] `proc/cmd_ps.go:572` — /proc/stat 每次 snapshot 读三次，共六次
- [ ] `proc/cmd_xargs.go:161` — ready channel 强制串行化 goroutine 启动
- [ ] `disk/cmd_ioperf.go:350` — goroutine 本地变量用 atomic，热路径无谓开销
- [ ] `disk/cmd_iostat.go:220` — /proc/diskstats 读入后双重拷贝
- [ ] `disk/cmd_md5sum.go:97` — io.ReadAll(stdin) 全量加载，应流式
- [ ] `disk/cmd_md5sum.go:162` — defer f.Close() 在 for 循环内，文件描述符泄漏
- [ ] `text/cmd_grep.go:263` — 无 context flag 时仍全量缓冲输入
- [ ] `text/cmd_strings.go:56` — io.ReadAll 全量加载大二进制文件
- [ ] `text/cmd_tail.go:183` — slice 追加后 re-slice，backing array 无法 GC
