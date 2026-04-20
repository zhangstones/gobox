# gobox Command Design Reference

本文档整理 gobox 支持的所有命令参数，对标 Linux 原生命令，便于快速查阅。

---

## 目录

- [文件系统命令](#文件系统命令)
- [文本处理命令](#文本处理命令)
- [网络命令](#网络命令)
- [进程命令](#进程命令)
- [磁盘命令](#磁盘命令)

---

## 文件系统命令

### find

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox find -atime string` | `find -atime` | ✅ 一致 | 文件访问时间过滤，`+N`（N天前）、`-N`（N天内）、`N`（恰好N天）。时间单位：`s`/`m`/`h`/`d` |
| `gobox find -empty` | `find -empty` | ✅ 一致 | 匹配空文件或空目录 |
| `gobox find -maxdepth int` | `find -maxdepth` | ✅ 一致 | 最大目录深度（-1=无限制） |
| `gobox find -mindepth int` | `find -mindepth` | ✅ 一致 | 最小目录深度 |
| `gobox find -mtime string` | `find -mtime` | ✅ 一致 | 文件修改时间过滤，格式同`-atime` |
| `gobox find -name string` | `find -name` | ✅ 一致 | 按文件名匹配（支持shell glob模式） |
| `gobox find -print` | `find -print` | ✅ 一致 | 打印匹配的文件路径（默认为true） |
| `gobox find -size string` | `find -size` | ✅ 一致 | 文件大小过滤：`+N`（大于N）、`-N`（小于N）。支持`K`/`M`/`G`后缀 |
| `gobox find -type string` | `find -type` | ✅ 一致 | 文件类型过滤：`f`（普通文件）或`d`（目录） |

### du

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox du -h` | `du -h` | ✅ 一致 | 以人类可读格式显示文件大小（KB、MB、GB） |
| `gobox du -s` | `du -s` | ✅ 一致 | 汇总显示，只显示每个参数目录的总大小 |

---

## 文本处理命令

### head

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox head -n NUM` | `head -n` | ✅ 一致 | 打印前 NUM 行（默认 10） |
| `gobox head -c NUM` | `head -c` | ✅ 一致 | 打印前 NUM 字节 |
| `gobox head -q` | `head -q` | ✅ 一致 | 不显示文件名标题 |
| `gobox head -h` | `head --help` | ✅ 一致 | 显示帮助信息 |

### tail

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox tail -n NUM` | `tail -n` | ✅ 一致 | 打印最后 NUM 行（默认 10） |
| `gobox tail -f` | `tail -f` | ✅ 一致 | 跟踪文件变化，输出追加的数据 |
| `gobox tail --follow=name` | `tail --follow=name` | ✅ 一致 | 通过文件名跟踪（处理日志轮转） |
| `gobox tail --retry` | `tail --retry` | ✅ 一致 | 文件不存在时持续重试 |
| `gobox tail -q` | `tail -q` | ✅ 一致 | 不显示文件名标题 |
| `gobox tail -s SEC` | `tail -s` | ✅ 一致 | 每次轮询间隔秒数（默认 1） |
| `gobox tail --pid=PID` | `tail --pid` | ✅ 一致 | 指定进程 PID 退出时停止跟踪 |

### grep

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox grep -E` | `grep -E` | ✅ 一致 | 使用扩展正则表达式（ERE） |
| `gobox grep -F` | `grep -F` | ✅ 一致 | 将模式作为固定字符串（非正则） |
| `gobox grep -c` | `grep -c` | ✅ 一致 | 仅显示匹配行的计数 |
| `gobox grep -i` | `grep -i` | ✅ 一致 | 忽略大小写 |
| `gobox grep -line-buffered` | `grep --line-buffered` | ✅ 一致 | 行缓冲（每行后刷新） |
| `gobox grep -n` | `grep -n` | ✅ 一致 | 显示行号 |
| `gobox grep -o` | `grep -o` | ✅ 一致 | 仅显示匹配的部分 |
| `gobox grep -q` | `grep -q` | ✅ 一致 | 静默模式，仅返回退出码 |
| `gobox grep -r` | `grep -r` | ✅ 一致 | 递归搜索目录 |
| `gobox grep -v` | `grep -v` | ✅ 一致 | 反向匹配（显示不匹配的行） |

### sed

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox sed -n` | `sed -n` | ✅ 一致 | 抑制自动打印模式空间 |
| `gobox sed -i[SUFFIX]` | `sed -i` | ✅ 一致 | 原地编辑（若提供 SUFFIX 则备份） |
| `gobox sed -e SCRIPT` | `sed -e` | ✅ 一致 | 添加脚本命令 |
| `gobox sed -f FILE` | `sed -f` | ✅ 一致 | 从文件添加脚本命令 |
| `gobox sed -h` | `sed --help` | ✅ 一致 | 显示帮助信息 |

**sed 命令：**

| gobox 命令 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox sed s/pattern/replacement/flags` | `sed s/pattern/replacement/flags` | ✅ 一致 | 替换匹配内容 |
| `gobox sed d` | `sed d` | ✅ 一致 | 删除模式空间 |
| `gobox sed p` | `sed p` | ✅ 一致 | 打印模式空间 |
| `gobox sed =` | `sed =` | ✅ 一致 | 打印当前行号 |
| `gobox sed i\text` | `sed i\text` | ✅ 一致 | 在指定行前插入文本 |
| `gobox sed a\text` | `sed a\text` | ✅ 一致 | 在指定行后追加文本 |
| `gobox sed c\text` | `sed c\text` | ✅ 一致 | 替换指定行为文本 |

**替换标志：**

| gobox 命令 | 标志 | 实现一致性 | 功能说明 |
|------------|------|------------|----------|
| `gobox sed` (替换标志) | `g` | ✅ 一致 | 全局替换 |
| `gobox sed` (替换标志) | `i` | ✅ 一致 | 忽略大小写 |
| `gobox sed` (替换标志) | `p` | ✅ 一致 | 替换后打印行 |
| `gobox sed` (替换标志) | `N` | ✅ 一致 | 替换第 N 个匹配（1-9） |

### sort

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox sort -n` | `sort -n` | ✅ 一致 | 按数值排序 |
| `gobox sort -r` | `sort -r` | ✅ 一致 | 反向排序 |
| `gobox sort -k NUM` | `sort -k` | ✅ 一致 | 按第 NUM 列排序 |
| `gobox sort -t CHAR` | `sort -t` | ✅ 一致 | 使用 CHAR 作为字段分隔符 |
| `gobox sort -u` | `sort -u` | ✅ 一致 | 去重（仅保留唯一行） |
| `gobox sort -M` | `sort -M` | ✅ 一致 | 按月份排序 |
| `gobox sort -h` | `sort -h` | ✅ 一致 | 按人类可读数字排序（1K、2M） |
| `gobox sort -R` | `sort -R` | ✅ 一致 | 随机排序 |
| `gobox sort -c` | `sort -c` | ✅ 一致 | 检查是否已排序 |
| `gobox sort -o FILE` | `sort -o` | ✅ 一致 | 输出到指定文件 |
| `gobox sort -z` | `sort -z` | ✅ 一致 | 行以 0 字节终止 |

### uniq

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox uniq -c` | `uniq -c` | ✅ 一致 | 显示每行出现次数 |
| `gobox uniq -d` | `uniq -d` | ✅ 一致 | 仅显示重复行 |
| `gobox uniq -u` | `uniq -u` | ✅ 一致 | 仅显示唯一行 |
| `gobox uniq -i` | `uniq -i` | ✅ 一致 | 忽略大小写 |
| `gobox uniq -w N` | `uniq -w` | ✅ 一致 | 最多比较 N 个字符 |
| `gobox uniq -f N` | `uniq -f` | ✅ 一致 | 跳过前 N 个字段 |

> 注意：uniq 仅对排序后的相邻重复行有效（需先排序）

### wc

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox wc -l` | `wc -l` | ✅ 一致 | 打印行数 |
| `gobox wc -w` | `wc -w` | ✅ 一致 | 打印词数 |
| `gobox wc -c` | `wc -c` | ✅ 一致 | 打印字节数 |
| `gobox wc -m` | `wc -m` | ✅ 一致 | 打印字符数 |
| `gobox wc -L` | `wc -L` | ✅ 一致 | 打印最长行长度 |

---

## 网络命令

### curl

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox curl -s, --silent` | `curl -s` | ✅ 一致 | 静默模式，不显示进度和错误输出 |
| `gobox curl -S, --show-error` | `curl -S` | ✅ 一致 | 静默模式下仍显示错误 |
| `gobox curl -o, --output FILE` | `curl -o` | ✅ 一致 | 输出到指定文件 |
| `gobox curl -O, --remote-name` | `curl -O` | ✅ 一致 | 使用远程文件名 |
| `gobox curl -L, --location` | `curl -L` | ✅ 一致 | 跟随重定向 |
| `gobox curl -I, --head` | `curl -I` | ✅ 一致 | 仅获取响应头 |
| `gobox curl -w, --write-out FORMAT` | `curl -w` | ✅ 一致 | 输出格式，如 `%{http_code}` |
| `gobox curl -m, --max-time SEC` | `curl -m` | ✅ 一致 | 传输最大超时（秒） |
| `gobox curl -X, --request CMD` | `curl -X` | ✅ 一致 | HTTP 方法（GET/POST/PUT/DELETE） |
| `gobox curl -H, --header LINE` | `curl -H` | ✅ 一致 | 添加请求头 |
| `gobox curl -d, --data DATA` | `curl -d` | ✅ 一致 | POST 数据 |
| `gobox curl -k, --insecure` | `curl -k` | ✅ 一致 | 忽略证书错误 |
| `gobox curl --connect-timeout SEC` | `curl --connect-timeout` | ✅ 一致 | 连接超时 |
| `gobox curl --resolve HOST:PORT:ADDR` | `curl --resolve` | ✅ 一致 | 强制 DNS 解析 |
| `gobox curl -f, --fail` | `curl -f` | ✅ 一致 | HTTP 4xx/5xx 时失败 |
| `gobox curl -i, --include` | `curl -i` | ✅ 一致 | 输出包含响应头 |
| `gobox curl -c, --concurrent=N` | `ab -c` (Apache Bench) | 🆕 gobox扩展 | 并发请求数（基准测试） |
| `gobox curl -n, --requests=N` | `ab -n` (Apache Bench) | 🆕 gobox扩展 | 总请求数（基准测试） |
| `gobox curl --warmup=N` | `wrk --warmup` | 🆕 gobox扩展 | 预热请求数 |
| `gobox curl -t, --timeout=SEC` | `curl -m` | ✅ 一致 | 请求超时时间 |

### nc/netcat

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox nc -l, --listen` | `nc -l` | ✅ 一致 | 监听模式（服务器） |
| `gobox nc -z, --zero` | `nc -z` | ✅ 一致 | 零 I/O 模式（仅端口扫描） |
| `gobox nc -u, --udp` | `nc -u` | ✅ 一致 | UDP 模式 |
| `gobox nc -w SEC, --wait=SEC` | `nc -w` | ✅ 一致 | 连接超时（秒） |
| `gobox nc -v, --verbose` | `nc -v` | ✅ 一致 | 详细输出 |
| `gobox nc -n, --numeric-only` | `nc -n` | ✅ 一致 | 跳过 DNS 解析 |
| `gobox nc -4` | `nc -4` | ✅ 一致 | 强制 IPv4 |
| `gobox nc -6` | `nc -6` | ✅ 一致 | 强制 IPv6 |
| `gobox nc --bench` | `iperf -c` (部分) | 🆕 gobox扩展 | 基准测试模式 |
| `gobox nc -c N, --concurrent=N` | `hey -c` | 🆕 gobox扩展 | 并发连接数 |
| `gobox nc -n N, --requests=N` | `wrk -n` | 🆕 gobox扩展 | 总请求数 |
| `gobox nc -s N, --size=N` | `iperf -l` | 🆕 gobox扩展 | 数据块大小（默认 64KB） |
| `gobox nc -t SEC, --time=SEC` | `iperf -t` | 🆕 gobox扩展 | 测试持续时间 |
| `gobox nc -i SEC, --interval=SEC` | `iperf -i` | 🆕 gobox扩展 | 报告间隔（默认 1s） |

### netstat

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox netstat -a, --all` | `netstat -a` | ✅ 一致 | 显示所有 socket |
| `gobox netstat -t, --tcp` | `netstat -t` | ✅ 一致 | 仅显示 TCP socket |
| `gobox netstat -u, --udp` | `netstat -u` | ✅ 一致 | 仅显示 UDP socket |
| `gobox netstat -x, --unix` | `netstat -x` | ✅ 一致 | 仅显示 Unix domain socket |
| `gobox netstat -l, --listening` | `netstat -l` | ✅ 一致 | 仅显示监听 socket |
| `gobox netstat -n, --numeric` | `netstat -n` | ✅ 一致 | 数字地址/端口输出 |
| `gobox netstat -p, --programs` | `netstat -p` | ✅ 一致 | 显示 PID/Program |
| `gobox netstat -4` | `netstat -4` | ✅ 一致 | 仅显示 IPv4 socket |
| `gobox netstat -6` | `netstat -6` | ✅ 一致 | 仅显示 IPv6 socket |
| `gobox netstat -e, --extend` | `netstat -e` | ✅ 一致 | 显示扩展列（User/Inode） |
| `gobox netstat -o, --timers` | `netstat -o` | ✅ 一致 | 显示 timer 信息 |
| `gobox netstat -W, --wide` | `netstat -W` | ✅ 一致 | 宽输出（gobox 默认不截断） |
| `gobox netstat -port int` | 端口过滤 | 🆕 gobox扩展 | 按本地或远端端口精确过滤 |
| `gobox netstat -sort string` | 排序功能 | 🆕 gobox扩展 | 排序字段：recvq\|sendq\|local\|remote\|pid |
| `gobox netstat -state string` | 状态过滤 | 🆕 gobox扩展 | 按连接状态过滤，支持状态列表 |

### tw (轻量级 Web 服务器)

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox tw -p, --port=PORT` | `httpd -p` / `python -m http.server` | ✅ 一致 | 监听端口（默认 8080） |
| `gobox tw -d, --dir=DIR` | `httpd -d` / `python -m http.server` | ✅ 一致 | 服务目录（默认当前目录） |
| `gobox tw -r, --reuse` | `httpd -r` | ✅ 一致 | 启用 SO_REUSEADDR |
| `gobox tw --bench` | `wrk` | 🆕 gobox扩展 | 基准测试模式 |

**基准测试端点：**
- `GET /ping` - 返回 200 OK 和 "pong"
- `POST /ping` - 回显请求体
- `POST /upload` - 接收文件上传

### nslookup/dig

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox nslookup @DNS_SERVER` | `dig @DNS_SERVER` | ✅ 一致 | 使用指定 DNS 服务器 |
| `gobox nslookup -t TYPE, --type=TYPE` | `dig -t` | ✅ 一致 | 查询类型（A/AAAA/TXT/CNAME/NS/MX/SRV） |
| `gobox nslookup +short` | `dig +short` | ✅ 一致 | 简短输出（仅显示答案） |
| `gobox nslookup +noall +answer` | `dig +noall +answer` | ✅ 一致 | 仅显示答案部分 |
| `gobox nslookup +tcp` | `dig +tcp` | ✅ 一致 | 使用 TCP 替代 UDP |

### ifstat

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox ifstat -A` | `ifstat -a` | ✅ 一致 | 显示所有接口（包括虚拟接口） |
| `gobox ifstat -a` | `ifstat -a` | ✅ 一致 | 显示绝对值（累计值） |
| `gobox ifstat -d` | `netstat -ed` (丢包统计) | 🆕 gobox扩展 | 显示丢包计数 |
| `gobox ifstat -e` | `netstat -e` (错误统计) | 🆕 gobox扩展 | 显示错误包计数 |
| `gobox ifstat -i string` | `ifstat -i` | ✅ 一致 | 指定网络接口（逗号分隔） |
| `gobox ifstat -n int` | `ifstat -n` | ✅ 一致 | 采样次数（0=连续） |
| `gobox ifstat -p int` | `ifstat -i` | ✅ 一致 | 采样间隔秒数（默认 1） |

### np/netping

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox np -I string` | `ping -I` | ✅ 一致 | 使用的网络接口 |
| `gobox np -W int` | `ping -W` | ✅ 一致 | 超时时间（秒，默认 5） |
| `gobox np -arp` | `arping -I` | 🆕 gobox扩展 | ARP 模式 |
| `gobox np -c int` | `ping -c` | ✅ 一致 | 数据包数量（默认 4） |
| `gobox np -flood` | `ping -f` | ✅ 一致 | 洪水模式（最大速度） |
| `gobox np -i int` | `ping -i` | ⚠️ 部分一致 | 数据包间隔（微秒，默认 1000000），ping 默认秒 |
| `gobox np -icmp` | `ping` | ✅ 一致 | ICMP 模式 |
| `gobox np -l int` | 长连接探测 | 🆕 gobox扩展 | 长连接模式 |
| `gobox np -p int` | `nc -p` | 🆕 gobox扩展 | 目标端口 |
| `gobox np -q` | `ping -q` | ✅ 一致 | 静默模式 |
| `gobox np -s int` | 源端口绑定 | 🆕 gobox扩展 | 源端口 |
| `gobox np -scan` | `nc -z` | 🆕 gobox扩展 | 端口扫描模式 |
| `gobox np -tcp` | `nc` | ✅ 一致 | TCP 模式（默认） |
| `gobox np -udp` | `nc -u` | ✅ 一致 | UDP 模式 |
| `gobox np -v` | `ping -v` | ✅ 一致 | 详细输出 |
| `gobox np -w int` | 工作池并发 | 🆕 gobox扩展 | 并发工作数（默认 1） |

---

## 进程命令

### ps

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox ps -e` | `ps -e` | ✅ 一致 | 显示所有进程 |
| `gobox ps -f` | `ps -f` | ✅ 一致 | 全格式输出（显示 PPID 和可执行文件） |
| `gobox ps -i int` | `vmstat -i` (采样间隔) | 🆕 gobox扩展 | CPU 采样间隔（毫秒，默认 500） |
| `gobox ps -l int` | `ps -o pid,cmd` (截断) | 🆕 gobox扩展 | 最大命令长度（0=无限制，默认 40） |
| `gobox ps -n int` | `ps --no-headers` (管道 head) | 🆕 gobox扩展 | 仅显示前 N 个进程（0=显示全部） |
| `gobox ps -full string` | `pgrep -f` | ✅ 一致 | 按完整命令行正则匹配 |
| `gobox ps -comm string` | `pgrep -x` | ✅ 一致 | 按进程名精确匹配 |
| `gobox ps -r` | `ps -r` | ✅ 一致 | 反向排序 |
| `gobox ps -sort string` | `ps -O` (排序键) | 🆕 gobox扩展 | 排序字段：pid\|cpu\|rss\|vms\|cmd |

### top

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox top -d int` | `top -d` | ✅ 一致 | 每次更新间隔秒数（默认 2） |
| `gobox top -n int` | `top -n` | ✅ 一致 | 迭代次数（0=无限，默认 5） |
| `gobox top -r` | `top -r` | ✅ 一致 | 反向排序（默认 true） |
| `gobox top -sort string` | `top -o` (排序键) | 🆕 gobox扩展 | 排序字段：pid\|cpu\|rss\|vms\|cmd |

> 注意：gobox top 是 top 命令的简化版本，实时调用 psCmd 显示进程信息。

### xargs

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox xargs -I string` | `xargs -I` | ✅ 一致 | 使用自定义占位符替换输入 |
| `gobox xargs -i string` | `xargs -i` | ✅ 一致 | 替换字符串（默认占位符 `{}`） |
| `gobox xargs -d string` | `xargs -d` | ✅ 一致 | 输入分隔符（默认换行符） |
| `gobox xargs -n int` | `xargs -n` | ✅ 一致 | 每次命令调用的最大参数数 |
| `gobox xargs -P int` | `xargs -P` | ✅ 一致 | 最大并行进程数（默认 1） |
| `gobox xargs -r` | `xargs -r` | ✅ 一致 | 无输入时不运行命令 |
| `gobox xargs -t` | `xargs -t` | ✅ 一致 | 执行前打印命令 |

> 注意：默认命令是 `echo`，即 `gobox xargs` 等同于 `xargs echo`。

---

## 磁盘命令

### iostat

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox iostat -i sec` | `iostat -p` | ✅ 一致 | 采样间隔秒数 |
| `gobox iostat -n count` | `iostat -c` | ✅ 一致 | 采样次数 |
| `gobox iostat -H` | `iostat -h` | ✅ 一致 | 人类可读格式 |
| `gobox iostat -z` | `iostat -z` | ✅ 一致 | 跳过零活动设备 |

### ioperf

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox ioperf --bs string` | `fio --bs` | ✅ 一致 | 块大小，如 4k, 8k（默认 "4k"） |
| `gobox ioperf --direct int` | `fio --direct` | ✅ 一致 | 使用 O_DIRECT 绕过缓存（0 或 1） |
| `gobox ioperf --filename string` | `fio --filename` | ✅ 一致 | 测试文件路径（默认 "/tmp/ioperf_test"） |
| `gobox ioperf --fsync int` | `fio --fsync` | ✅ 一致 | 每次写后执行 fsync（0 或 1） |
| `gobox ioperf --group_reporting` | `fio --group_reporting` | ✅ 一致 | 聚合多任务报告 |
| `gobox ioperf --iodepth int` | `fio --iodepth` | ✅ 一致 | 队列深度（默认 1） |
| `gobox ioperf --latency` | `fio --latency` | ✅ 一致 | 输出延迟分布直方图 |
| `gobox ioperf --numjobs int` | `fio --numjobs` | ✅ 一致 | 并行任务数量（默认 1） |
| `gobox ioperf --percentile int` | `fio --percentile` | ✅ 一致 | 报告延迟百分位 |
| `gobox ioperf --rate string` | `fio --rate` | ✅ 一致 | 速率限制（如 100M） |
| `gobox ioperf --runtime int` | `fio --runtime` | ✅ 一致 | 运行时间（秒） |
| `gobox ioperf --rw string` | `fio --rw` | ✅ 一致 | I/O 模式：read/write/randread/randwrite/readwrite（默认 "read"） |
| `gobox ioperf --rwmixread int` | `fio --rwmixread` | ✅ 一致 | 读操作比例（0-100，用于 readwrite，默认 50） |
| `gobox ioperf --size string` | `fio --size` | ✅ 一致 | 总 I/O 大小（如 1G，默认 "1G"） |
| `gobox ioperf --sync int` | `fio --sync` | ✅ 一致 | 使用 O_SYNC（0 或 1） |
| `gobox ioperf --time_based` | `fio --time_based` | ✅ 一致 | 基于时间运行 |

### md5sum

| gobox 参数 | 对应原命今参数 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox md5sum -c, --check` | `md5sum -c` | ✅ 一致 | 从文件验证校验和 |
| `gobox md5sum --tag` | `md5sum --tag` | ✅ 一致 | BSD 风格输出 |
| `gobox md5sum -q, --quiet` | `md5sum -q` | ✅ 一致 | 安静模式，只显示校验和 |
| `gobox md5sum -s, --status` | `md5sum -s` | ✅ 一致 | 仅返回状态码 |
| `gobox md5sum -w, --warn` | `md5sum -w` | ✅ 一致 | 警告格式错误的行 |

---

## 实现一致性说明

| 标记 | 含义 |
|------|------|
| ✅ 一致 | 与原生命令功能完全一致 |
| ⚠️ 部分一致 | 功能相似但有细微差异或限制 |
| 🆕 gobox扩展 | 原生命令无此参数，为 gobox 新增功能 |

---

## 命令索引

| 命令 | 类别 | 功能 |
|------|------|------|
| find | 文件系统 | 文件搜索 |
| du | 文件系统 | 磁盘使用统计 |
| head | 文本处理 | 显示文件头部 |
| tail | 文本处理 | 显示文件尾部 |
| grep | 文本处理 | 文本搜索 |
| sed | 文本处理 | 流编辑器 |
| sort | 文本处理 | 排序 |
| uniq | 文本处理 | 去重 |
| wc | 文本处理 | 计数 |
| curl | 网络 | HTTP 客户端 |
| nc | 网络 | 网络工具 |
| netstat | 网络 | 网络连接统计 |
| tw | 网络 | Web 服务器 |
| nslookup/dig | 网络 | DNS 查询 |
| ifstat | 网络 | 网络接口统计 |
| np | 网络 | 网络 ping |
| ps | 进程 | 进程列表 |
| top | 进程 | 进程监控 |
| xargs | 进程 | 命令参数构建 |
| iostat | 磁盘 | I/O 统计 |
| ioperf | 磁盘 | I/O 性能测试 |
| md5sum | 磁盘 | 校验和计算 |
