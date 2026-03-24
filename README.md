# gobox

用 Go 编写的轻量级 BusyBox 风格实用工具集。该项目用纯 Go 实现了最小集的 Unix/Linux 命令行工具，旨在成为 BusyBox 的可移植替代方案，用于系统管理和文件管理任务。

**版本:** 0.3（sed，带全面的测试覆盖）

## 项目概述

gobox 是基本 Unix/Linux 实用工具的 AI 辅助实现。它提供带多个命令模式的单个二进制文件，类似于 BusyBox，允许资源高效的部署和跨平台可移植性。

## 功能和支持的命令

### 1. **find** - 文件搜索工具

在目录层次结构中搜索文件，带灵活的过滤选项。

**选项:**
- `-name <pattern>` - 使用 shell glob 模式匹配文件名 (*, ?, [abc])
- `-type <type>` - 按类型过滤：`f`（文件）或 `d`（目录）
- `-maxdepth <depth>` - 限制搜索深度
- `-mindepth <depth>` - 设置最小搜索深度
- `-empty` - 匹配空文件或目录
- `-size <spec>` - 按大小匹配文件：`+N`（大于）、`-N`（小于）、`N`（等于）。支持后缀：K/M/G/T（以 1024 为基数）
- `-atime <spec>` - 按访问时间匹配：`+N`（少于 N 单位前访问）、`-N`（多于 N 单位前访问）、`N`（N 单位内访问）。单位：s/m/h/d（秒/分/时/天，默认：天）
- `-mtime <spec>` - 按修改时间匹配：`+N`（少于 N 单位前修改）、`-N`（多于 N 单位前修改）、`N`（N 单位内修改）。单位：s/m/h/d（秒/分/时/天，默认：天）
- `-print` - 打印匹配路径（默认启用）

**用法:**
```bash
gobox find /path -name "*.txt" -type f
gobox find . -type d -empty -maxdepth 2
gobox find . -type f -size +100M
gobox find . -type f -mtime +1h          # 少于 1 小时前修改的文件
gobox find . -type f -mtime -1d          # 多于 1 天前修改的文件
gobox find . -type f -mtime 30m          # 最后 30 分钟内修改的文件
```

### 2. **du** - 磁盘使用报告

总结文件和目录的磁盘使用情况。

**选项:**
- `-h` - 以人类可读格式显示大小（B, KB, MB, GB 等）
- `-s` - 仅显示摘要（每个参数的总大小）

**用法:**
```bash
gobox du -h /var/log
gobox du -s -h .
```

**功能:**
- 递归目录遍历
- 优雅处理权限错误
- 人类可读的字节大小格式化

### 3. **ps** - 进程列表

列出运行中的进程，带详细信息（专注于 Linux）。

**选项:**
- `-e` - 显示所有进程
- `-f` - 完整格式（显示 PPID 和可执行路径）
- `-sort <key>` - 排序：`pid`（默认）、`cpu`、`rss`、`vms`、`cmd`
- `-r` - 反转排序顺序
- `-name <filter>` - 按命令名/cmdline 中的子串过滤
- `-n <count>` - 仅显示 N 个条目（0 = 全部）
- `-i <milliseconds>` - CPU 采样间隔（默认：500ms）
- `-l <length>` - 最大命令长度（0 = 无限，默认：40 字符）

**用法:**
```bash
gobox ps -e -f -sort cpu -r
gobox ps -name "java" -sort rss
gobox ps -n 10
```

**功能:**
- Linux 特定：从 `/proc` 读取详细的 CPU 和内存统计
- 通过采样计算 CPU 百分比
- 显示 PPID、可执行文件、cmdline、VMS、RSS 内存
- 输出到终端时智能截断长命令名

### 4. **top** - 实时进程查看器

显示运行进程的动态实时视图。

**选项:**
- `-d <seconds>` - 更新间隔（默认：2 秒）
- `-n <iterations>` - 迭代次数（0 = 无限，默认：5）
- `-sort <key>` - 排序：`pid`、`cpu`、`rss`、`vms`、`cmd`
- `-r` - 反转排序顺序（默认启用）

**用法:**
```bash
gobox top -d 1 -n 10
gobox top -sort cpu
```

**功能:**
- 更新之间清屏以进行实时查看
- 重用 ps 命令逻辑进行进程收集
- 可配置的刷新间隔和迭代次数

### 5. **iostat** - 块设备 I/O 统计

基于 cgroup 指标打印块设备 IOPS 和吞吐量（仅 Linux）。

**选项:**
- `-i <seconds>` - 采样间隔（默认：1 秒）
- `-n <count>` - 采样次数（默认：1）
- `-H` - 人性化 IOPS 和吞吐量（默认：true，显示 K/M 表示法）
- `-z` - 仅显示具有非零 I/O 率的设备

**用法:**
```bash
gobox iostat -i 2 -n 5
gobox iostat -z
```

**功能:**
- 支持 Linux cgroup v2（`io.stat`）和 cgroup v1（`blkio.*`）指标
- 读取字节和 I/O 操作（IOPS）
- 通过 `/sys/dev/block` 将设备主:次 ID 映射到设备名
- 优雅处理权限错误
- 仅 Linux：在非 Linux 系统上返回错误

### 6. **netstat** - 网络连接统计

从 Linux `/proc/net` 显示网络设备和连接统计。

**选项:**
- `-state <states>` - 按连接状态过滤（逗号分隔，如 `LISTEN,ESTABLISHED`）
- `-port <port>` - 按本地或远程端口号过滤
- `-sort <key>` - 排序：`recvq`、`sendq`、`local`、`remote`、`pid`

**用法:**
```bash
gobox netstat -state LISTEN
gobox netstat -port 8080
gobox netstat -sort recvq
```

**功能:**
- 解析 `/proc/net/tcp`、`/proc/net/tcp6`、`/proc/net/udp`、`/proc/net/udp6`
- 显示接收/发送队列大小
- 将 inode 映射到进程 ID 和名称
- IPv6 支持
- 仅 Linux 实现

### 7. **grep** - 文件中的模式搜索

使用正则表达式或固定字符串在文件中搜索模式。

**选项:**
- `-i` - 匹配时忽略大小写
- `-v` - 反转匹配（显示不匹配的行）
- `-c` - 仅显示匹配行数
- `-n` - 在输出中显示行号
- `-r` - 在目录中递归搜索
- `-F` - 将模式解释为固定字符串（非正则）
- `--help` - 显示帮助消息

**用法:**
```bash
gobox grep "error" /var/log/syslog
gobox grep -i -r "TODO" /path/to/code
gobox grep -v "^#" config.txt
gobox grep -c "pattern" file.txt
gobox grep -n -i "function" *.go
gobox grep -F "hello.world" file.txt  # 字面点，非正则
cat file.txt | gobox grep "pattern"   # 从 stdin 读取
```

**功能:**
- 完整的正则表达式支持（Go regexp 语法）
- 使用 `-i` 进行不区分大小写的匹配
- 使用 `-v` 反转匹配以排除模式
- 使用 `-c` 进行行数统计
- 使用 `-n` 在输出中显示行号
- 使用 `-r` 进行递归目录搜索
- 使用 `-F` 进行固定字符串匹配（无正则）
- 支持标准输入进行管道

### 8. **sed** - 用于文本转换的流编辑器

使用 sed 风格的脚本语法过滤和转换文本。

**选项:**
- `-n` - 抑制模式空间的自动打印
- `-i[SUFFIX]` - 就地编辑文件（如果提供 SUFFIX 则创建备份）
- `-e SCRIPT` - 将脚本添加到要执行的命令
- `-f FILE` - 将 FILE 的内容添加到要执行的命令
- `--help` - 显示帮助消息

**命令:**
- `s/pattern/replacement/flags` - 用替换内容替代模式
- `d` - 删除模式空间
- `p` - 打印模式空间
- `=` - 打印当前行号
- `i\text` - 在寻址行之前插入文本
- `a\text` - 在寻址行之后追加文本
- `c\text` - 将寻址行更改为文本

**替代标志:**
- `g` - 全局替换（所有出现）
- `i` - 不区分大小写的匹配
- `p` - 如果进行替换则打印行
- `N` - 替换第 N 次出现（1-9）

**用法:**
```bash
gobox sed 's/foo/bar/' file.txt
gobox sed 's/foo/bar/g' file.txt
gobox sed -n 's/foo/bar/p' file.txt
gobox sed -i.bak 's/old/new/g' file.txt
gobox sed -e 's/foo/bar/' -e 's/baz/qux/' file.txt
cat file.txt | gobox sed 's/old/new/g'
```

**功能:**
- 完整的正则表达式支持（Go regexp 语法）
- 使用 `i` 标志进行不区分大小写的匹配
- 使用 `g` 标志进行全局替换
- 第 N 次出现替换（如 `s/foo/bar/2`）
- 使用 `p` 标志在替换时打印
- 就地编辑，带可选备份
- 使用 `-e` 进行多个表达式
- 使用 `-f` 的脚本文件
- 支持标准输入进行管道
- 支持后向引用（`${1}`、`${2}` 等或 `\1`、`\2`）

### 9. **xargs** - 从输入构建和执行命令

从标准输入构建和执行命令行。

**选项:**
- `-i, -I <placeholder>` - 带自定义占位符的替换模式（默认：`{}`）
- `-d <delimiter>` - 输入分隔符（默认：换行）
- `-n <count>` - 每次命令调用的最大参数数
- `-P <processes>` - 最大并行进程（默认：1）
- `-v` - 详细：在执行前打印命令
- `-r` - 不运行：如果没有提供输入则不执行命令

**用法:**
```bash
find . -name "*.txt" | gobox xargs -P 4 grep "pattern"
echo -e "file1\nfile2\nfile3" | gobox xargs -i rm {}
cat list.txt | gobox xargs -n 5 process_batch
```

**功能:**
- 用于灵活命令构建的替换模式
- 用于批量处理的追加模式
- 带信号量控制的并行执行支持
- 自定义输入分隔符
- 用于调试的详细输出
- 优雅的 stdin/stdout/stderr 处理

### 10. **管道示例**

组合 sed 和 grep 进行强大的文本处理：

```bash
# 过滤和转换日志文件
cat access.log | gobox grep "404" | gobox sed 's/GET /REQUEST: /'

# 清理和格式化配置文件
gobox sed -e '/^#/d' -e '/^$/d' config.txt | gobox grep -i "server"

# 使用 sed 和 xargs 批量重命名
ls *.txt | gobox sed 's/.txt/.bak/' | gobox xargs -I {} mv {}
```

## 全局选项

所有命令支持：
- `-h, --help` - 显示命令帮助
- `--version, -v, version` - 显示 gobox 版本

**全局用法:**
```bash
gobox --help
gobox --version
```

## 要求和依赖

- **Go:** 1.20 或更高版本
- **平台支持:**
  - 跨平台：`find`、`du`、`xargs`
  - Linux 特定：`ps`、`top`、`iostat`、`netstat`
- **外部依赖:**
  - `github.com/mitchellh/go-ps` - 进程列表（用于跨平台 ps 回退）

## 实现说明

### 架构
- 通过字符串匹配进行命令调度的单二进制设计
- 每个命令在自己的文件中实现：`cmd_<command>.go`
- `utils.go` 中的共享工具（如 `isStdoutTerminal()`、`humanSize()`）
- `main.go` 中的主调度器

### 限制和设计选择
1. **部分 BusyBox 实现:** 不支持原始 BusyBox 的所有标志；专注于常用选项
2. **Linux 优先:** 一些命令（ps、iostat、netstat）针对 Linux `/proc` 文件系统优化
3. **优雅降级:** 遍历目录时在权限错误上继续
4. **无外部命令:** 纯 Go 实现，依赖最小
5. **终端检测:** 检测 TTY 的智能输出格式化，用于智能命令截断

### 错误处理
- 退出码 1：未提供命令或显示帮助
- 退出码 2：命令执行错误
- 退出码 127：未知命令
- 命令将描述性错误消息返回到 stderr

## 使用示例

### 查找所有超过 1MB 的 Python 文件
```bash
gobox find /project -name "*.py" -type f
```

### 显示目录的磁盘使用情况
```bash
gobox du -h -s /home /var /tmp
```

### 按 CPU 使用情况监控进程
```bash
gobox top -d 1 -sort cpu
```

### 过滤网络连接
```bash
gobox netstat -state LISTEN -port 8080
```

### 并行批量处理文件
```bash
ls *.log | gobox xargs -P 4 -i gzip {}
```

### 监控磁盘 I/O
```bash
gobox iostat -i 1 -n 10 -z
```

## 未来增强

- 额外的命令（grep、sed、awk、ls 等）
- 与标准 Unix 工具的更多标志兼容性
- 针对 Windows 特定变体的跨平台支持
- 大数据集的性能优化
- 配置文件支持
- 手册页文档
