# gobox 命令扩展计划

## 目标
为K8s精简镜像（distroless等）补充常用调试命令，覆盖高频排障场景。

## 当前实现
`find`, `du`, `ps`, `top`, `iostat`, `netstat`, `xargs`, `grep`, `sed`

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

### 1. head / tail
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

### 2. curl
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

### 3. sort
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

### 4. uniq
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

### 5. wc
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

### 6. nslookup / dig
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

### 7. nc (netcat)
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

### 8. tw (tinyweb)
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

## 实现顺序

1. **head / tail** - 日志分析是最高频场景
2. **curl** - HTTP调试刚需
3. **sort** - 排序输出
4. **uniq** - 去重统计
5. **wc** - 计数统计
6. **nslookup / dig** - DNS调试
7. **nc** - TCP/UDP连接测试
8. **tw** - 轻量级HTTP服务器

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
