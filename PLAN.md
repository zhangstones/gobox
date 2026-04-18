# gobox 命令扩展计划

## 目标
为K8s精简镜像（distroless等）补充常用调试命令，覆盖高频排障场景。

## 当前实现
`find`, `du`, `ps`, `top`, `iostat`, `netstat`, `xargs`, `grep`, `sed`, `head`, `tail`, `curl`, `sort`, `uniq`, `wc`, `nslookup`, `dig`, `nc`, `tw`

---

## 现有命令补充参数

### grep 补充参数
- `-A NUM` / `--after-context=NUM` 显示匹配行后NUM行（分析日志堆栈常用）
- `-B NUM` / `--before-context=NUM` 显示匹配行前NUM行
- `-C NUM` / `--context=NUM` 显示匹配行前后各NUM行
- `--include="PATTERN"` 按文件名模式过滤（K8s日志目录大量.log文件，高频需求）
- `--exclude-dir=DIR` 排除特定目录（排除metrics等）
- `-l` / `--files-with-matches` 只显示包含匹配的文件名
- `-L` / `--files-without-match` 只显示不包含匹配的文件名

### ps 补充参数
- `-ww` 输出不截断宽度，查看完整命令行参数（诊断长java -jar参数时必需）
- `-o FIELD1,FIELD2` 自定义输出列（常用：pid,ppid,cmd,pcpu,pmem）

### netstat 补充参数
- `-l` / `--listening` 只显示监听端口，快速定位暴露的端口
- `-n` / `--numeric` 显示数字端口（而非服务名），避免/etc/services查找开销

### find 补充参数
- `-path PATTERN` 按完整路径匹配（K8s日志路径如`/var/log/pods/...`常用）
- `-not` / `!` 否定条件，与`-path`组合排除特定目录

---

## 计划新增命令

### 1. head / tail ✅ 已实现
**优先级**: P0
**用途**: 日志分析是K8s调试最常见场景

#### head 支持参数
- `-n NUM` / `--lines=NUM` 显示前NUM行（默认10）
- `-c NUM` / `--bytes=NUM` 显示前NUM字节
- `-q` / `--quiet` 多文件时不显示文件名

#### tail 支持参数
- `-n NUM` / `--lines=NUM` 显示末尾NUM行（默认10）
- `-f` / `--follow` 实时跟踪文件变化（Ctrl+C退出）
- `--follow=name` 基于文件名跟踪（文件轮转时自动切换）
- `--retry` 持续尝试打开文件（用于日志轮转场景）
- `-q` / `--quiet` 多文件时不显示文件名
- `-s SEC` / `--sleep-interval=SEC` 轮询间隔（默认1秒，多pod日志时降低频率省资源）
- `--pid=PID` 进程退出时自动停止跟踪

#### 常见用法
```bash
gobox tail -f /var/log/app.log              # 实时跟踪
gobox tail -n 100 /var/log/app.log          # 最后100行
gobox head -n 20 /var/log/startup.log       # 启动日志前20行
gobox tail -f /var/log/app.log | gobox grep ERROR  # 实时过滤ERROR
```

---

### 2. curl ✅ 已实现
**优先级**: P0
**用途**: HTTP端点调试，服务健康检查

#### 支持参数
- `-s` / `--silent` 静默模式
- `-S` / `--show-error` 显示错误
- `-o FILE` / `--output FILE` 输出到文件
- `-O` / `--remote-name` 使用远程文件名保存
- `-L` / `--location` 跟随重定向
- `-I` / `--head` 只获取HTTP头
- `-w FORMAT` / `--write-out FORMAT` 输出格式（%{http_code}, %{time_total}等）
- `-m SEC` / `--max-time SEC` 超时时间
- `-T FILE` / `--upload-file FILE` PUT上传文件
- `-F NAME=FILE` / `--form NAME=FILE` multipart表单上传（模拟表单文件上传）
- `-X CMD` / `--request CMD` 指定HTTP方法（GET/POST/PUT/DELETE等）
- `-H LINE` / `--header LINE` 添加请求头
- `-d DATA` / `--data DATA` POST数据
- `-k` / `--insecure` 忽略证书错误（调试用）
- `--connect-timeout SEC` 连接超时
- `--resolve HOST:PORT:ADDR 强制将HOST:PORT解析到ADDR（K8s调试Service IP高频需求）
- `-f` / `--fail` HTTP 4xx/5xx 返回错误码（非200，用于健康检查）
- `-i` / `--include` 显示响应头（排查Set-Cookie、重定向等）

#### Benchmark模式
通过--bench模式测试HTTP服务性能，配合tw服务的ping接口使用。

**Benchmark参数：**
- `--bench` 启用benchmark模式
- `-c N` / `--concurrent=N` 并发线程数（默认1）
- `-n N` / `--requests=N` 总请求数（默认100）
- `--warmup=N` 预热请求数（默认0）
- `-t SEC` / `--timeout=SEC` 请求超时（默认10s）

**Benchmark输出：**
```
Requests: 1000, Concurrency: 10, Failed: 0
Latency: min=5ms, max=120ms, mean=25ms, p50=24ms, p90=35ms, p99=80ms
Throughput: 400 req/s, Total time: 2.5s
```

**常见用法：**
```bash
gobox curl --bench -c 10 -n 1000 http://localhost:8080/ping  # GET基准测试
gobox curl --bench -c 5 -n 500 -d "testdata" http://localhost:8080/ping  # POST ping测试
gobox curl -T /tmp/file.txt http://localhost:8080/upload      # 上传文件
gobox curl -F "file=@/tmp/data.tar.gz" http://localhost:8080/upload  # multipart上传
```

#### 常见用法
```bash
gobox curl -s http://localhost:8080/health        # 健康检查
gobox curl -I http://svc:8080/                  # 检查服务头
gobox curl -w "\n%{http_code}\n" http://x.com/  # 带状态码
gobox curl -L -o /dev/null -s -w "%{http_code}" http://x.com/
gobox curl -X POST -d '{"key":"value"}' http://x.com/api
gobox curl -H "Authorization: Bearer xxx" http://x.com/api
```

---

### 3. sort ✅ 已实现
**优先级**: P1
**用途**: 排序输出，配合grep/uniq使用

#### 支持参数
- `-n` / `--numeric-sort` 数值排序
- `-r` / `--reverse` 反向排序
- `-k NUM` / `--key=NUM` 按第NUM列排序
- `-t CHAR` / `--field-separator=CHAR` 指定列分隔符
- `-u` / `--unique` 去重（等同于sort | uniq）
- `-M` / `--month-sort` 按月份排序
- `-h` / `--human-numeric-sort` 人性化数字（1K, 2M）
- `-R` / `--random-sort` 随机排序
- `-c` / `--check` 检查是否已排序
- `-o FILE` / `--output=FILE` 输出到文件
- `-z` / `--zero-terminated` 处理含空字符的行（配合find -print0做零截断管道）

#### 常见用法
```bash
gobox sort -k3 -rn file.txt               # 按第3列数值逆序
gobox sort -u file.txt                    # 去重排序
gobox ps aux | gobox grep nginx | gobox sort -k3 -rn | head -n 5
```

---

### 4. uniq ✅ 已实现
**优先级**: P1
**用途**: 过滤相邻重复行

#### 支持参数
- `-c` / `--count` 显示每行出现次数
- `-d` / `--repeated` 只显示重复行
- `-u` / `--unique` 只显示不重复行
- `-i` / `--ignore-case` 忽略大小写
- `-w NUM` / `--check-chars=NUM` 比较前NUM个字符
- `-f NUM` / `--skip-fields=NUM` 跳过前NUM列

#### 常见用法
```bash
gobox uniq -c file.txt                    # 统计重复次数
gobox uniq -d file.txt                    # 只看重复的
gobox sort file.txt | gobox uniq -c | sort -rn  # 频率统计
```

---

### 5. wc ✅ 已实现
**优先级**: P1
**用途**: 统计行数、字数、字节数

#### 支持参数
- `-l` / `--lines` 只显示行数
- `-w` / `--words` 只显示词数
- `-c` / `--bytes` 只显示字节数
- `-m` / `--chars` 只显示字符数
- `-L` / `--max-line-length` 显示最长行长度

#### 常见用法
```bash
gobox wc -l /var/log/app.log              # 统计日志行数
gobox grep ERROR /var/log/app.log | gobox wc -l  # ERROR出现次数
```

---

### 6. nslookup / dig ✅ 已实现
**优先级**: P0
**用途**: DNS调试，K8s服务发现问题的首选工具

#### nslookup 支持参数
- `HOST [SERVER]` 查询HOST的DNS记录
- 默认查询A记录
- `-type=TYPE` / `set type=TYPE` 指定查询类型

#### dig 支持参数
- `HOST [DNS_SERVER]` 查询HOST的DNS记录
- `-t TYPE` / `--type=TYPE` 指定记录类型（A/AAAA/SRV/TXT/CNAME/NS等）
- `+short` 简短输出
- `+noall +answer` 只显示answer部分
- `+tcp` 使用TCP而非UDP
- `@DNS_SERVER` 指定DNS服务器

#### K8s DNS知识
- A记录: `<service>.<namespace>.svc.cluster.local` → ClusterIP
- SRV记录: `<service>.<namespace>.svc.cluster.local` → 端口信息服务
- Headless: `<service>.<namespace>.svc.cluster.local` → Pod IPs列表

#### 常见用法
```bash
gobox nslookup my-svc.default.svc.cluster.local        # A记录查询
gobox dig my-svc.default.svc.cluster.local             # 详细A记录
gobox dig my-svc.default.svc.cluster.local -t SRV      # SVC/SRV记录
gobox dig my-svc.default.svc.cluster.local +short      # 简短输出
gobox dig _http._tcp.my-svc.default.svc.cluster.local -t SRV  # K8s标准SRV
```

---

### 7. nc (netcat) ✅ 已实现
**优先级**: P1
**用途**: TCP/UDP连接测试，端口检测，服务探测，作为简单服务端/客户端测试

#### 支持参数
- `host port` - 连接到远程端口（相当于telnet）
- `-l port` - 监听模式（服务端）
- `-z` - 零I/O模式（仅检测端口开放）
- `-u` / `--udp` - UDP模式
- `-w SEC` / `--wait=SEC` - 连接超时
- `-v` / `--verbose` - 详细输出
- `-n` / `--numeric-only` 跳过DNS解析，测试端点时更快速
- `-4` / `-6` 强制IPv4/IPv6

#### Benchmark模式（类iperf）
支持TCP/UDP吞吐量测试，通过magic头避免误判。

**协议格式：** 前8字节magic头 `GOBENCH\x00` 标识benchmark协议，包含4字节序列号和4字节数据长度。

**服务端参数（-l指定）：**
- `-l port` 监听模式
- `--bench` 启用benchmark模式，收到数据后原样返回
- `-s N` / `--size=N` 数据块大小（默认64KB，支持B/K/M/G后缀）

**客户端参数：**
- `host port` 目标地址
- `--bench` 发送benchmark测试
- `-c N` / `--concurrent=N` 并发连接数（默认1）
- `-n N` / `--requests=N` 总请求数（默认100）
- `-s N` / `--size=N` 数据块大小（默认64KB）
- `-t SEC` / `--time=SEC` 测试持续时间（与-n互斥）
- `-i SEC` / `--interval=SEC` 报告间隔

**Benchmark输出：**
```
Connecting to localhost:8080
[  1] local=127.0.0.1 port=12345 connected
[ ID] Interval       Transfer     Bandwidth
[  1] 0.0- 1.0s     10.5MB       105.0Mbps
[  1] 1.0- 2.0s     10.3MB       103.0Mbps
[  1] 2.0- 3.0s     10.6MB       106.0Mbps
[  1] 3.0- 4.0s     10.4MB       104.0Mbps
[  1] 4.0- 5.0s     10.5MB       105.0Mbps
[  1] 5.0s     52.3MB       104.6Mbps avg
Latency: min=0.5ms, max=2.1ms, mean=0.8ms
```

**常见用法：**
```bash
# 服务端：监听并返回数据（-l指定服务端方向）
gobox nc -l 8080 --bench -s 1M

# 客户端：TCP吞吐量测试
gobox nc localhost 8080 --bench -c 5 -n 1000 -s 64K

# 客户端：UDP带宽测试
gobox nc -u localhost 8080 --bench -t 10 -s 1M

# 端口检测
gobox nc -z mysql-svc 3306 && echo "Port open"
```

---

### 8. tw (tinyweb) ✅ 已实现
**优先级**: P2
**用途**: 轻量级HTTP服务器，用于本地测试、对比curl结果、mock服务，支持curl --bench测试

#### 支持参数
- `-p PORT` / `--port=PORT` - 监听端口（默认8080）
- `-d DIR` / `--dir=DIR` - 提供文件的目录（默认当前目录）
- `-r` / `--reuse` - 重用地址
- `--bench` - 启用benchmark模式（返回固定响应体用于性能测试）
- `-h` / `--help` - 显示帮助

#### Benchmark接口
tw内置ping端点，支持curl --bench测试：

**Endpoints：**
- `GET /ping` - 返回200 OK "pong"，用于GET基准测试
- `POST /ping` - 接收任意body，原样返回，用于POST性能测试
- `POST /upload` - 接收上传文件，返回上传大小和状态

#### 常见用法
```bash
gobox tw -d /tmp/files -p 8080                    # 启动HTTP服务
gobox curl http://localhost:8080/file.txt          # 用curl访问对比
gobox tw -d /app/static                            # 提供静态文件
gobox tw --bench -p 8080                           # 启动benchmark模式服务
gobox curl --bench -c 10 -n 1000 http://localhost:8080/ping  # HTTP GET性能测试
gobox curl --bench -c 5 -n 500 -d "testdata" http://localhost:8080/ping  # POST性能测试
gobox tw -p 8080                                    # 启动默认服务
gobox curl -T /tmp/file.txt http://localhost:8080/upload  # 上传文件到服务端
```

---

### 9. ifstat (interface statistics)
**优先级**: P1
**用途**: 网络接口流量监控，宿主机网络问题排查

#### 支持参数
- `-i IFACE` / `--iface=IFACE` - 指定网卡（支持逗号分隔多网卡，默认显示所有物理网卡）
- `-p SEC` / `--interval=SEC` - 轮询间隔秒数（默认1秒）
- `-n COUNT` / `--count=COUNT` - 轮询次数（默认持续直到Ctrl+C）
- `-a` / `--absolute` - 显示绝对值（不做平均，直接显示累计值）
- `-A` - 显示所有接口（包括veth/tun/tap等虚拟接口）
- `-e` / `--errors` - 显示 error 包数量（rx_errors, tx_errors）
- `-d` / `--drops` - 显示 drop 包数量（rx_dropped, tx_dropped）

#### 显示格式（按行分网卡，类似iostat）
```
Interface        rxpps/s    txpps/s    rxKB/s    txKB/s
eth0              0.00       0.00       0.00      0.00
eth1              0.00       0.00       0.00      0.00
```

带 `-e -d` 时：
```
Interface        rxpps/s    txpps/s    rxKB/s    txKB/s    rxerrs   txerrs   rxdrop   txdrop
eth0              0.00       0.00       0.00      0.00         0        0        0        0
```

#### 物理网卡检测
通过 `/sys/class/net/<iface>/type` 的 ARPHRD_ETHER 值（=1）判断：
- `type == 1` - ARPHRD_ETHER，物理网卡
- `type != 1` - 虚拟网卡（loopback/tun/tap/bridge等）

默认只显示物理网卡（type=1），加 `-A` 显示全部。

#### 数据来源
- `/sys/class/net/<iface>/statistics/rx_packets` - 接收包数
- `/sys/class/net/<iface>/statistics/tx_packets` - 发送包数
- `/sys/class/net/<iface>/statistics/rx_bytes` - 接收字节数
- `/sys/class/net/<iface>/statistics/tx_bytes` - 发送字节数
- `/sys/class/net/<iface>/statistics/rx_errors` - 接收错误数
- `/sys/class/net/<iface>/statistics/tx_errors` - 发送错误数
- `/sys/class/net/<iface>/statistics/rx_dropped` - 接收丢弃数
- `/sys/class/net/<iface>/statistics/tx_dropped` - 发送丢弃数

#### 常见用法
```bash
gobox ifstat                  # 显示所有物理网卡，每秒刷新
gobox ifstat -p 2             # 每2秒刷新
gobox ifstat -n 5             # 显示5次后退出
gobox ifstat -i eth0           # 只显示eth0
gobox ifstat -A                # 显示所有接口（包括veth等虚拟口）
gobox ifstat -e -d            # 显示error和drop统计
gobox ifstat -a               # 显示绝对值（累计值）
```

---

## 实现顺序

1. ~~**head / tail**~~ ✅ - 日志分析是最高频场景
2. ~~**curl**~~ ✅ - HTTP调试刚需
3. ~~**sort**~~ ✅ - 排序输出
4. ~~**uniq**~~ ✅ - 去重统计
5. ~~**wc**~~ ✅ - 计数统计
6. ~~**nslookup / dig**~~ ✅ - DNS调试
7. ~~**nc**~~ ✅ - TCP/UDP连接测试
8. ~~**tw**~~ ✅ - 轻量级HTTP服务器
9. **ioperf** - I/O 性能基准测试（待实现）
10. **ifstat** - 网络接口流量监控（待实现）
11. **hping** - TCP/IP包生成器和端口扫描（待实现）
12. **np** - 网络连通性排障（待实现）
13. **md5sum** - 文件校验和（待实现）
14. **rand** - 随机数生成（待实现）
15. **seq** - 序列数生成（待实现）

### 10. ioperf (I/O performance benchmark)
**优先级**: P1
**用途**: 块设备/文件系统 I/O 性能测试，类 fio 简化版

#### 支持参数
- `--rw` - I/O 模式 (`read`/`write`/`randread`/`randwrite`/`readwrite`)
- `--rwmixread` - 读比例 (0-100，`readwrite` 模式时有效)
- `--filename` - 测试文件路径（每个 job 会创建 `filename.0`, `filename.1`...）
- `--bs` - 块大小 (默认 `4k`，支持 `4k`/`8k`/`128k` 等)
- `--size` - 总 I/O 数据量 (如 `1G`, `10G`)
- `--numjobs` - 并行 job 数 (默认 1)
- `--iodepth` - 队列深度 (默认 1)
- `--direct=1` - 使用 `O_DIRECT` 绕过缓存
- `--fsync=1` - 每次写入后执行 `fsync`
- `--sync=1` - 使用 `O_SYNC`
- `--rate` - 限速 (如 `100M`)
- `--time_based` - 基于时间运行（与 `--runtime` 配合）
- `--runtime` - 运行时长（秒，`time_based` 模式）
- `--group_reporting` - 聚合多 job 报告
- `--percentile` - 延迟百分位 (如 `99`)
- `--latency` - 输出延迟分布直方图（统计I/O延迟分布，排障核心指标）

#### 输出格式
```
ioperf: bs=4k, jobs=4, iodepth=4
READ:  IOPS=125432, BW=489.00MB/s, lat=avg=128.00us, p99=256.00us
WRITE: IOPS=54321, BW=212.00MB/s, lat=avg=145.00us, p99=300.00us
```

#### 限制
- 只能写普通文件，禁止写设备文件 (`/dev/*`)

#### 常见用法
```bash
# 顺序写性能
gobox ioperf --rw=write --filename=/tmp/testfile --size=1G --bs=4k

# 随机读写混合 (70%读)
gobox ioperf --rw=readwrite --rwmixread=70 --filename=/tmp/testfile --size=1G --numjobs=4 --iodepth=4

# 随机读性能
gobox ioperf --rw=randread --filename=/tmp/testfile --size=1G --numjobs=2 --direct=1

# 时间基准模式 (60秒)
gobox ioperf --rw=randwrite --filename=/tmp/testfile --time_based --runtime=60 --iodepth=32
```

---

### 11. md5sum
**优先级**: P1
**用途**: 文件校验和计算（下载完整性验证、日志完整性检查）

#### 支持参数
- `file...` - 计算文件 MD5（默认模式）
- `-c`, `--check` - 校验模式（读取 MD5 文件验证）
- `--tag` - BSD 格式输出 (`MD5 (file) = xxx`)
- `-q`, `--quiet` - 静默模式
- `-s`, `--status` - 只返回状态码
- `-w`, `--warn` - 警告格式错误

#### 常见用法
```bash
gobox md5sum file.tar.gz                      # 计算 MD5
gobox md5sum file1 file2 file3               # 批量计算
gobox md5sum -c checksums.md5               # 校验文件
echo "abc123" | gobox md5sum                 # 计算字符串 MD5
```

---

### 12. rand
**优先级**: P2
**用途**: 生成随机数（类 openssl rand）

#### 支持参数
- `NUM` - 生成 NUM 字节随机数
- `-n NUM` - 同上，字节数
- `-hex` - 十六进制输出（默认）
- `-base64` - Base64 输出
- `-out FILE` - 输出到文件

#### 常见用法
```bash
gobox rand 32                                 # 生成 32 字节随机数
gobox rand -n 32 -hex                        # 32 字节 hex
gobox rand -n 24 -base64                     # 24 字节 base64
gobox rand -n 16 -out /tmp/secret.key      # 输出到文件
```

---

### 13. seq
**优先级**: P2
**用途**: 生成序列数（日志分析、批量操作）

#### 支持参数
- `[FIRST [INC]] LAST` - 序列范围（默认 FIRST=1, INC=1）
- `-f`, `--format` - 格式字符串（默认 `%g`）
- `-s`, `--separator` - 分隔符（默认 `\n`）
- `-w`, `--equal-width` - 等宽输出（自动补零）

#### 常见用法
```bash
gobox seq 5                                  # 1 2 3 4 5
gobox seq 2 5                              # 2 3 4 5
gobox seq 0 2 10                          # 0 2 4 6 8 10
gobox seq -f "%02g" 5                     # 01 02 03 04 05
gobox seq -s "," 3                        # 1,2,3
gobox seq -w 9                            # 01 02 ... 09
```

---

### 14. np (netping)
**优先级**: P1
**用途**: 网络连通性排障（端口扫描、TCP/UDP/ICMP/ARP ping）

#### 支持参数
- 模式选择:
  - `--tcp` - TCP模式（默认）
  - `--udp` - UDP模式
  - `--icmp` - ICMP模式
  - `--arp` - ARP模式（ARP ping）
  - `--scan` - 端口扫描模式

- 通用参数:
  - `-c COUNT` / `--count=COUNT` - 发包数量
  - `-i USEC` / `--interval=USEC` - 发包间隔（微秒）
  - `-p PORT` / `--port=PORT` - 目标端口
  - `-s PORT` / `--source=PORT` - 源端口
  - `-I IFACE` / `--iface=IFACE` - 使用指定网卡
  - `-W SEC` / `--wait=SEC` - 超时时间（秒）
  - `--flood` - flood模式（最大速度发包）
  - `-w COUNT` / `--workers=COUNT` - 并发线程数（TCP/UDP ping时有效）

- 长连接模式:
  - `-l [COUNT]` / `--long[=COUNT]` - 长连接模式（默认1个连接），ping一次后等待服务端关闭连接再继续
  - `-W SEC` / `--wait=SEC` - 超时时间（秒），超时则重连或退出

- 输出参数:
  - `-q` / `--quiet` - 安静模式，只显示最终统计
  - `-v` / `--verbose` - 详细输出

#### 端口扫描模式 (--scan)
```bash
gobox np --scan 22,80,443 target.com
gobox np --scan 1-1000 target.com -v
```

#### 常见用法
```bash
# TCP ping测延迟
gobox np --tcp -p 80 target.com

# UDP ping测延迟
gobox np --udp -p 53 target.com

# ICMP ping测延迟
gobox np --icmp target.com

# ARP ping（发现局域网主机）
gobox np --arp target.com
gobox np --arp -c 4 192.168.1.1

# 端口扫描
gobox np --scan 80,443,8080 target.com
gobox np --scan 1-1000 target.com -v

# 指定并发线程
gobox np --tcp -p 80 -w 10 target.com  # 10个并发连接

# Flood模式
gobox np --tcp -p 80 --flood target.com

# 长连接模式（ping后等待服务端关闭连接再继续）
gobox np --tcp -p 80 -l target.com            # 1个长连接
gobox np --tcp -p 80 -l=10 target.com        # 10个长连接
gobox np --tcp -p 80 -l=10 -W 5 target.com  # 10个长连接，超时5秒

# 指定发包间隔和数量
gobox np --tcp -p 80 -i 1000000 -c 10 target.com  # 每秒一个包，发10个

# 安静模式（最终统计）
gobox np -q --scan 1-100 target.com
```

---

## 注意事项

- 所有命令必须静态编译（不使用CGO），确保可在各种镜像中运行
- 参数顺序兼容GNU coreutils（尽量）
- 错误处理：遇到错误返回非零退出码
- 管道支持：部分命令需要支持stdin输入
- 边界情况：空文件、大文件、二进制文件等

---

## 质量要求

- 每个命令有对应的`*_test.go`
- 测试覆盖常用参数组合
- 集成测试验证实际行为
