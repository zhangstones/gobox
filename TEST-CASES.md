# gobox Parity Test Cases

## 说明

本文件将 `docs/CMD-DESIGN.md` 中的每个命令、每个参数映射为至少一个必要测试案例。

字段说明：

- `Case ID`: 唯一案例编号
- `Arg/Feature`: 被验证的参数或语义点
- `Mode`: `exact` / `structured` / `behavior` / `contract`
- `Native Baseline`: 原生命令或等价基线；若为 gobox 扩展则记为 `gobox-only`
- `Fixture`: 输入夹具类型
- `Core Assertion`: 核心断言

---

---

## 覆盖清单

以下命令已按 `docs/CMD-DESIGN.md` 当前条目建立参数级 case：

- 文件系统：`find`、`du`
- 文本处理：`head`、`tail`、`grep`、`sed`、`sort`、`uniq`、`wc`
- 网络：`curl`、`nc`、`netstat`、`tw`、`nslookup/dig`、`ifstat`、`np`
- 进程：`ps`、`top`、`xargs`
- 磁盘：`iostat`、`ioperf`、`md5sum`

约束：

1. `✅ 一致` 条目优先写成 parity 或 behavior case；确实受环境限制时，需在自动化中显式 `Skip`。
2. `⚠️ 部分一致` 条目必须验证“差异边界”，而不是只测成功路径。
3. `🆕 gobox扩展` 条目必须验证参数是否真正进入执行路径，而不是仅测试 flag 可解析。

---

## 文件系统命令

### find

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| FIND-001 | `-atime` | exact | `find -atime` | temp dir + controlled atime | 匹配集合与退出行为一致 |
| FIND-002 | `-empty` | exact | `find -empty` | 空文件/空目录/非空文件 | 仅返回空目标 |
| FIND-003 | `-maxdepth` | exact | `find -maxdepth` | 多层目录树 | 深度截断一致 |
| FIND-004 | `-mindepth` | exact | `find -mindepth` | 多层目录树 | 最小深度过滤一致 |
| FIND-005 | `-mtime` | exact | `find -mtime` | controlled mtime | 匹配集合一致 |
| FIND-006 | `-name` | exact | `find -name` | 多文件名 | glob 匹配一致 |
| FIND-007 | `-print` | contract | `find -print` | 单文件树 | 默认与显式打印行为稳定 |
| FIND-008 | `-size` | exact | `find -size` | 不同大小文件 | 大小过滤一致 |
| FIND-009 | `-type` | exact | `find -type` | 文件+目录 | 类型过滤一致 |

### du

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| DU-001 | `-h` | structured | `du -h` | 小文件树 | 输出包含同等目录项与可读大小语义 |
| DU-002 | `-s` | structured | `du -s` | 多目录 | 仅输出汇总项 |

---

## 文本处理命令

### head

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| HEAD-001 | `-n NUM` | exact | `head -n` | 固定文本文件 | 前 N 行一致 |
| HEAD-002 | `-c NUM` | exact | `head -c` | 固定文本/字节流 | 前 N 字节一致 |
| HEAD-003 | `-q` | exact | `head -q` | 多文件 | 文件名标题隐藏一致 |
| HEAD-004 | `-h` | contract | `head --help` | none | 帮助输出成功 |

### tail

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| TAIL-001 | `-n NUM` | exact | `tail -n` | 固定文本文件 | 最后 N 行一致 |
| TAIL-002 | `-f` | behavior | `tail -f` | 动态追加文件 | 跟随追加行为一致 |
| TAIL-003 | `--follow=name` | behavior | `tail --follow=name` | 轮转文件 | 轮转后仍跟随 |
| TAIL-004 | `--retry` | behavior | `tail --retry` | 延迟创建文件 | 持续重试直到文件出现 |
| TAIL-005 | `-q` | exact | `tail -q` | 多文件 | 文件名标题隐藏一致 |
| TAIL-006 | `-s SEC` | behavior | `tail -s` | 动态文件 | 轮询节奏可控 |
| TAIL-007 | `--pid=PID` | behavior | `tail --pid` | 子进程 + 动态文件 | 进程退出后停止跟随 |

### grep

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| GREP-001 | `-E` | exact | `grep -E` | 正则文本 | 扩展正则匹配一致 |
| GREP-002 | `-F` | exact | `grep -F` | 含正则元字符文本 | 固定字符串语义一致 |
| GREP-003 | `-c` | exact | `grep -c` | 多匹配文本 | 计数一致 |
| GREP-004 | `-i` | exact | `grep -i` | mixed-case 文本 | 忽略大小写一致 |
| GREP-005 | `--line-buffered` | behavior | `grep --line-buffered` | 流式输入 | 流式输入时每行匹配可及时输出 |
| GREP-006 | `-n` | exact | `grep -n` | 多行文本 | 行号输出一致 |
| GREP-007 | `-o` | exact | `grep -o` | 多匹配文本 | 仅输出匹配片段一致 |
| GREP-008 | `-q` | exact | `grep -q` | 有匹配/无匹配 | 退出码一致 |
| GREP-009 | `-r` | exact | `grep -r` | 目录树 | 递归匹配集合一致 |
| GREP-010 | `-v` | exact | `grep -v` | 多行文本 | 反向匹配一致 |
| GREP-011 | `-A NUM` | exact | `grep -A` | 上下文文本 | 后文上下文一致 |
| GREP-012 | `-B NUM` | exact | `grep -B` | 上下文文本 | 前文上下文一致 |
| GREP-013 | `-C NUM` | exact | `grep -C` | 上下文文本 | 前后文上下文一致 |
| GREP-014 | `--include=PATTERN` | exact | `grep --include` | 混合文件类型目录树 | 仅扫描匹配文件名 |
| GREP-015 | `--exclude-dir=DIR` | exact | `grep --exclude-dir` | 目录树 | 指定目录被排除 |
| GREP-016 | `-l` | exact | `grep -l` | 多文件 | 仅输出有匹配文件名 |
| GREP-017 | `-L` | exact | `grep -L` | 多文件 | 仅输出无匹配文件名 |

### sed

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| SED-001 | `-n` | exact | `sed -n` | 固定文本 | 抑制自动打印一致 |
| SED-002 | `-i[SUFFIX]` | exact | `sed -i` | 临时文件 | 原地修改与备份一致 |
| SED-003 | `-e SCRIPT` | exact | `sed -e` | 文本文件 | 多脚本执行一致 |
| SED-004 | `-f FILE` | exact | `sed -f` | 脚本文件 | 脚本文件执行一致 |
| SED-005 | `-h` | contract | `sed --help` | none | 帮助输出成功 |
| SED-006 | `s/pattern/replacement/flags` | exact | `sed s///` | 文本 | 替换结果一致 |
| SED-007 | `d` | exact | `sed d` | 文本 | 删除语义一致 |
| SED-008 | `p` | exact | `sed p` | 文本 | 打印语义一致 |
| SED-009 | `=` | exact | `sed =` | 文本 | 行号输出一致 |
| SED-010 | `i\text` | exact | `sed i\` | 文本 | 前插文本一致 |
| SED-011 | `a\text` | exact | `sed a\` | 文本 | 后追加文本一致 |
| SED-012 | `c\text` | exact | `sed c\` | 文本 | 替换行为一致 |
| SED-013 | 替换标志 `g` | exact | `sed s///g` | 多匹配文本 | 全局替换一致 |
| SED-014 | 替换标志 `i` | exact | `sed s///i` | mixed-case 文本 | 忽略大小写替换一致 |
| SED-015 | 替换标志 `p` | exact | `sed s///p` | 文本 | 替换后打印一致 |
| SED-016 | 替换标志 `N` | exact | `sed s///N` | 多匹配文本 | 第 N 次替换一致 |

### sort

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| SORT-001 | `-n` | exact | `sort -n` | 数字行 | 数值排序一致 |
| SORT-002 | `-r` | exact | `sort -r` | 文本行 | 逆序一致 |
| SORT-003 | `-k NUM` | exact | `sort -k` | 多列文本 | 指定列排序一致 |
| SORT-004 | `-t CHAR` | exact | `sort -t` | 分隔列文本 | 分隔符解析一致 |
| SORT-005 | `-u` | exact | `sort -u` | 重复行 | 唯一化排序一致 |
| SORT-006 | `-M` | exact | `sort -M` | 月份文本 | 月份排序一致 |
| SORT-007 | `-h` | exact | `sort -h` | 1K/2M 文本 | 人类可读数字排序一致 |
| SORT-008 | `-R` | behavior | `sort -R` | 文本 | 输出元素集合与行数保持一致，允许顺序随机 |
| SORT-009 | `-c` | exact | `sort -c` | 已排序/未排序文本 | 退出码一致 |
| SORT-010 | `-o FILE` | exact | `sort -o` | 输出文件 | 写文件结果一致 |
| SORT-011 | `-z` | exact | `sort -z` | 零分隔文本 | 零终止处理一致 |

### uniq

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| UNIQ-001 | `-c` | exact | `uniq -c` | 排序后的重复文本 | 计数前缀一致 |
| UNIQ-002 | `-d` | exact | `uniq -d` | 排序后的重复文本 | 仅重复行一致 |
| UNIQ-003 | `-u` | exact | `uniq -u` | 排序后的重复文本 | 仅唯一行一致 |
| UNIQ-004 | `-i` | exact | `uniq -i` | mixed-case 文本 | 忽略大小写一致 |
| UNIQ-005 | `-w N` | exact | `uniq -w` | 前缀相同文本 | 比较前 N 字符一致 |
| UNIQ-006 | `-f N` | exact | `uniq -f` | 多列文本 | 跳过字段一致 |

### wc

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| WC-001 | `-l` | exact | `wc -l` | 多行文本 | 行数一致 |
| WC-002 | `-w` | exact | `wc -w` | 多词文本 | 词数一致 |
| WC-003 | `-c` | exact | `wc -c` | UTF-8 文本 | 字节数一致 |
| WC-004 | `-m` | exact | `wc -m` | UTF-8 文本 | 字符数一致 |
| WC-005 | `-L` | exact | `wc -L` | 不同长度行 | 最长行长度一致 |

---

## 网络命令

### curl

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| CURL-001 | `-s, --silent` | behavior | `curl -s` | local HTTP server | 静默输出/错误行为一致 |
| CURL-002 | `-S, --show-error` | behavior | `curl -S` | failing HTTP request | 静默下恢复错误输出 |
| CURL-003 | `-o, --output FILE` | behavior | `curl -o` | local HTTP server | 文件输出一致 |
| CURL-004 | `-O, --remote-name` | behavior | `curl -O` | local HTTP server | 远程文件名保存一致 |
| CURL-005 | `-L, --location` | behavior | `curl -L` | redirect server | 跟随重定向一致 |
| CURL-006 | `-I, --head` | behavior | `curl -I` | local HTTP server | 仅输出头部 |
| CURL-007 | `-w, --write-out` | behavior | `curl -w` | local HTTP server | 关键格式占位符一致 |
| CURL-008 | `-m, --max-time` | behavior | `curl -m` | slow server | 超时行为一致 |
| CURL-009 | `-X, --request` | behavior | `curl -X` | local HTTP server | 请求方法一致 |
| CURL-010 | `-H, --header` | behavior | `curl -H` | local HTTP server | 自定义头一致 |
| CURL-011 | `-d, --data` | behavior | `curl -d` | local HTTP server | POST body 一致 |
| CURL-012 | `-k, --insecure` | behavior | `curl -k` | local HTTPS server with self-signed cert | 忽略证书错误后请求可成功 |
| CURL-013 | `--connect-timeout` | behavior | `curl --connect-timeout` | unroutable target | 连接超时一致 |
| CURL-014 | `--resolve` | behavior | `curl --resolve` | local HTTP server + fake host | 强制解析一致 |
| CURL-015 | `-f, --fail` | behavior | `curl -f` | 404/500 server | 失败退出语义一致 |
| CURL-016 | `-i, --include` | behavior | `curl -i` | local HTTP server | 响应头输出一致 |
| CURL-017 | `-T, --upload-file` | behavior | `curl -T` | local upload server | PUT 上传一致 |
| CURL-018 | `-F, --form` | behavior | `curl -F` | local multipart server | multipart 上传一致 |
| CURL-019 | `-c, --concurrent=N` | contract | gobox-only | local bench server | 并发请求数参数生效 |
| CURL-020 | `-n, --requests=N` | contract | gobox-only | local bench server | 总请求数参数生效 |
| CURL-021 | `--warmup=N` | contract | gobox-only | local bench server | 预热阶段可执行 |
| CURL-022 | `-t, --timeout=SEC` | behavior | `curl -m` | local slow HTTP server | 请求超时语义与 `curl -m` 对齐 |

### nc/netcat

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| NC-001 | `-l, --listen` | behavior | `nc -l` | local socket | 监听模式一致 |
| NC-002 | `-z, --zero` | behavior | `nc -z` | local socket | 零 I/O 端口探测一致 |
| NC-003 | `-u, --udp` | behavior | `nc -u` | local UDP socket | UDP 发送行为一致 |
| NC-004 | `-w, --wait=SEC` | behavior | `nc -w` | timeout target | 等待超时一致 |
| NC-005 | `-v, --verbose` | behavior | `nc -v` | local socket | verbose 关键输出一致 |
| NC-006 | `-n, --numeric-only` | behavior | `nc -n` | host/ip target | 跳过 DNS 解析一致 |
| NC-007 | `-4` | behavior | `nc -4` | IPv4 server | IPv4 强制一致 |
| NC-008 | `-6` | behavior | `nc -6` | IPv6 server | IPv6 强制一致 |
| NC-009 | `--bench` | contract | gobox-only | local bench pair | benchmark 模式稳定 |
| NC-010 | `-c, --concurrent=N` | contract | gobox-only | local bench pair | 并发连接数生效 |
| NC-011 | `-n, --requests=N` | contract | gobox-only | local bench pair | 请求数生效 |
| NC-012 | `-s, --size=N` | contract | gobox-only | local bench pair | 数据块大小生效 |
| NC-013 | `-t, --time=SEC` | contract | gobox-only | local bench pair | 测试持续时间生效 |
| NC-014 | `-i, --interval=SEC` | contract | gobox-only | local bench pair | 报告间隔生效 |

### netstat

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| NETSTAT-001 | `-port int` | structured | `netstat` | local listener | 端口过滤命中预期监听项 |
| NETSTAT-002 | `-sort string` | structured | gobox-only | local listeners | 排序字段语义正确 |
| NETSTAT-003 | `-state string` | structured | `netstat` | local listener + state filter | 状态列表过滤正确 |
| NETSTAT-004 | `-l, --listening` | structured | `netstat -l` | local listener | 仅输出监听 socket |
| NETSTAT-005 | `-n, --numeric` | structured | `netstat -n` | local listener | 仅数字地址/端口输出 |
| NETSTAT-006 | `-a, --all` | structured | `netstat -a` | local sockets | 参数可用并显示 socket 表 |
| NETSTAT-007 | `-t, --tcp` | structured | `netstat -t` | local TCP listener | 仅输出 TCP socket |
| NETSTAT-008 | `-u, --udp` | structured | `netstat -u` | local UDP socket | 仅输出 UDP socket |
| NETSTAT-009 | `-x, --unix` | structured | `netstat -x` | local Unix socket | 仅输出 Unix socket |
| NETSTAT-010 | `-p, --programs` | structured | `netstat -p` | local sockets | PID/Program 列可用 |
| NETSTAT-011 | `-4` | structured | `netstat -4` | local IPv4 socket | 仅输出 IPv4 socket |
| NETSTAT-012 | `-6` | structured | `netstat -6` | local IPv6 socket | 仅输出 IPv6 socket |
| NETSTAT-013 | `-e, --extend` | structured | `netstat -e` | local sockets | 扩展列可用 |
| NETSTAT-014 | `-o, --timers` | structured | `netstat -o` | local sockets | Timer 列可用 |
| NETSTAT-015 | `-W, --wide` | structured | `netstat -W` | local sockets | 参数可用且不截断地址 |

### tw

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| TW-001 | `-p, --port=PORT` | contract | gobox-only | local HTTP server port bind | 指定端口可监听 |
| TW-002 | `-d, --dir=DIR` | contract | gobox-only | temp dir static file | 指定目录可服务文件 |
| TW-003 | `-r, --reuse` | contract | gobox-only | repeated bind | 地址重用生效 |
| TW-004 | `--bench` | contract | gobox-only | local HTTP server | `/ping` 与 `/upload` 端点可用 |

### nslookup/dig

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| DNS-001 | `@DNS_SERVER` | behavior | `dig @DNS_SERVER` | controlled DNS endpoint / skip if unavailable | 指定 DNS server 参数进入查询路径 |
| DNS-002 | `-t TYPE, --type=TYPE` | behavior | `dig -t` | controlled domain | 查询类型进入结果语义 |
| DNS-003 | `+short` | behavior | `dig +short` | controlled domain | 简短输出语义一致 |
| DNS-004 | `+noall +answer` | behavior | `dig +noall +answer` | controlled domain | answer-only 语义一致 |
| DNS-005 | `+tcp` | behavior | `dig +tcp` | controlled DNS endpoint / skip if unavailable | TCP 查询路径生效 |

### ifstat

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| IFSTAT-001 | `-A` | structured | `ifstat -a` | local interfaces | 显示全部接口集合不弱于 baseline |
| IFSTAT-002 | `-a` | structured | `ifstat -a` | local interfaces | 绝对值模式下输出速率列切换为累计值语义 |
| IFSTAT-003 | `-d` | contract | gobox-only | local interfaces | drop 列出现 |
| IFSTAT-004 | `-e` | contract | gobox-only | local interfaces | error 列出现 |
| IFSTAT-005 | `-i string` | structured | `ifstat -i` | selected iface | 仅输出指定接口 |
| IFSTAT-006 | `-n int` | behavior | `ifstat -n` | local interfaces | 样本数受控并按次数退出 |
| IFSTAT-007 | `-p int` | behavior | `ifstat -t` / `ifstat delay` | local interfaces | 采样间隔参数生效 |

### np/netping

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| NP-001 | `-I string` | contract | gobox-only | local target | 指定网卡参数可进入拨号器 |
| NP-002 | `-W int` | behavior | `ping -W` | timeout target | 超时语义稳定 |
| NP-003 | `-arp` | behavior | `arping` | default gateway | ARP 模式可执行并收到响应 |
| NP-004 | `-c int` | behavior | `ping -c` | local target | 次数受控 |
| NP-005 | `-flood` | contract | `ping -f` | local target | flood 模式可执行 |
| NP-006 | `-i int` | contract | gobox-only | local target | 微秒间隔参数生效 |
| NP-007 | `-icmp` | behavior | `ping` | local target | ICMP 模式可执行 |
| NP-008 | `-l int` | contract | gobox-only | local TCP target | 长连接模式生效 |
| NP-009 | `-p int` | behavior | `nc -p`/TCP target | local TCP/UDP target | 目标端口生效 |
| NP-010 | `-q` | behavior | `ping -q` | local target | 安静模式输出收敛 |
| NP-011 | `-s int` | contract | gobox-only | local target | 源端口绑定生效 |
| NP-012 | `-scan` | contract | `nc -z` | local open/closed ports | 扫描结果稳定 |
| NP-013 | `-tcp` | behavior | `nc` | local TCP server | TCP 模式可执行 |
| NP-014 | `-udp` | behavior | `nc -u` | local UDP server | UDP 模式可执行 |
| NP-015 | `-v` | behavior | `ping -v` | local target | verbose 输出包含关键字段 |
| NP-016 | `-w int` | contract | gobox-only | local target | worker 并发参数生效 |

---

## 进程命令

### ps

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| PS-001 | `-e` | structured | `ps -e` | current process | 可看到所有/当前进程 |
| PS-002 | `-f` | structured | `ps -f` | current process | 包含 `PPID` 和执行信息 |
| PS-003 | `-i int` | contract | gobox-only | current process | 采样间隔参数可执行 |
| PS-004 | `-l int` | contract | gobox-only | long cmdline process | 截断长度生效 |
| PS-005 | `-n int` | contract | gobox-only | current processes | 限制输出行数 |
| PS-006 | `-full string` | structured | `pgrep -f` | current process | 完整命令行正则匹配符合 `pgrep -f` |
| PS-007 | `-r` | structured | `ps -r` | current processes | 排序方向反转 |
| PS-008 | `-sort string` | contract | gobox-only | current processes | 排序字段生效 |
| PS-009 | `-ww` | contract | `ps -ww` | long cmdline process | 不截断宽度 |
| PS-010 | `-o FIELD1,FIELD2` | structured | `ps -o` | current process | 自定义列输出正确 |
| PS-011 | `-comm string` | structured | `pgrep -x` | current process | 进程名精确匹配符合 `pgrep -x` |

### top

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| TOP-001 | `-d int` | behavior | `top -d` | single iteration | 刷新间隔参数可用且单轮模式下不会异常退出 |
| TOP-002 | `-n int` | behavior | `top -n` | single iteration | 指定迭代次数后退出 |
| TOP-003 | `-r` | structured | `top -r` | single iteration | 排序方向切换且关键排序方向符合预期 |
| TOP-004 | `-sort string` | contract | gobox-only | single iteration | 排序字段生效 |

### xargs

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| XARGS-001 | `-I string` | exact | `xargs -I` | stdin tokens | 自定义占位符替换一致 |
| XARGS-002 | `-i string` | exact | `xargs -i` | stdin tokens | 默认/显式占位符一致 |
| XARGS-003 | `-d string` | exact | `xargs -d` | 自定义分隔输入 | 分隔符处理一致 |
| XARGS-004 | `-n int` | exact | `xargs -n` | 多 token stdin | 分批参数数一致 |
| XARGS-005 | `-P int` | behavior | `xargs -P` | 多 token stdin | 并发行为稳定 |
| XARGS-006 | `-r` | exact | `xargs -r` | 空 stdin | 无输入时不执行 |
| XARGS-007 | `-t` | exact | `xargs -t` | stdin tokens | 执行前打印命令一致 |

---

## 磁盘命令

### iostat

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| IOSTAT-001 | `-i sec` | contract | `iostat -p` | local Linux host | 采样间隔参数可执行 |
| IOSTAT-002 | `-n count` | contract | `iostat -c` | local Linux host | 采样次数受控 |
| IOSTAT-003 | `-H` | contract | `iostat -h` | local Linux host | 人类可读格式生效 |
| IOSTAT-004 | `-z` | contract | `iostat -z` | local Linux host | 零活动设备过滤生效 |

### ioperf

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| IOPERF-001 | `-bs string` | contract | gobox-only | temp file | 块大小参数生效 |
| IOPERF-002 | `-direct int` | contract | gobox-only | temp file | direct 参数可解析 |
| IOPERF-003 | `-filename string` | contract | gobox-only | temp file | 指定文件路径可用 |
| IOPERF-004 | `-fsync int` | contract | gobox-only | temp file | fsync 参数可解析 |
| IOPERF-005 | `-group_reporting` | contract | gobox-only | temp file + multi job | 聚合输出生效 |
| IOPERF-006 | `-iodepth int` | contract | gobox-only | temp file | 队列深度影响执行 |
| IOPERF-007 | `-latency` | contract | gobox-only | temp file | 延迟统计输出出现 |
| IOPERF-008 | `-numjobs int` | contract | gobox-only | temp file | 并发 job 参数生效 |
| IOPERF-009 | `-percentile int` | contract | gobox-only | temp file | 百分位输出出现 |
| IOPERF-010 | `-rate string` | contract | gobox-only | temp file | 限速参数可解析 |
| IOPERF-011 | `-runtime int` | contract | gobox-only | temp file | 运行时长参数生效 |
| IOPERF-012 | `-rw string` | contract | gobox-only | temp file | I/O 模式切换生效 |
| IOPERF-013 | `-rwmixread int` | contract | gobox-only | temp file | 混合读比例可解析 |
| IOPERF-014 | `-size string` | contract | gobox-only | temp file | 数据量参数生效 |
| IOPERF-015 | `-sync int` | contract | gobox-only | temp file | sync 参数可解析 |
| IOPERF-016 | `-time_based` | contract | gobox-only | temp file | 时间模式生效 |

### md5sum

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| MD5-001 | `-c, --check` | exact | `md5sum -c` | checksum file | 校验结果与退出码一致 |
| MD5-002 | `--tag` | exact | `md5sum --tag` | file | BSD 风格输出一致 |
| MD5-003 | `-q, --quiet` | exact | `md5sum --quiet` | checksum file | 安静模式一致 |
| MD5-004 | `-s, --status` | exact | `md5sum --status` | checksum file | 仅退出码一致 |
| MD5-005 | `-w, --warn` | exact | `md5sum --warn` | malformed checksum file | 警告行为一致 |

---

## 实施优先级

### 第一批（高稳定、强 parity）
- `grep`, `sed`, `head`, `tail`, `sort`, `uniq`, `wc`, `md5sum`, `xargs`, `find`

### 第二批（结构化 parity）
- `ps`, `netstat`, `du`, `iostat`, `ifstat`

### 第三批（行为/contract）
- `curl`, `nc`, `nslookup/dig`, `np`, `tw`, `ioperf`, `top`

---

## 覆盖约束

1. 本文件中每个 `Case ID` 最终都需要映射到自动化测试代码。
2. 若因环境限制无法可靠执行 native baseline，测试代码中必须：
   - 明确标记为 `contract`
   - 或在运行时 `Skip` 并说明原因。
3. 后续若 `docs/CMD-DESIGN.md` 增减条目，必须同步更新此文件。
