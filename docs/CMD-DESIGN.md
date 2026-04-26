# gobox Command Design Reference

本文档整理 gobox 支持的所有命令参数，对标 Linux 原生命令或常用参考工具，便于快速查阅。

---

## 设计原则

gobox 的目标不是完整替代 BusyBox 或 coreutils，而是在容器、精简镜像、临时排障环境中补齐常见缺失命令，提供足够实用的诊断和处置能力。

命令选择遵循三个约束：

1. 精简镜像通常缺失：优先补充 slim/distroless/debug 场景里经常不可用，但排障时很需要的工具。
2. 容器排障价值高：优先覆盖网络、进程、文件、文本、磁盘 I/O 等常见定位路径。
3. 最常用 + 不膨胀 + 精简：只实现高频参数和核心语义，避免引入完整系统管理套件或低频高级参数。

排除范围同样明确：

1. 不纳入系统管理命令：不实现用户管理、init/service、内核模块、文件系统修复、设备管理、网络配置变更等宿主机/系统级管理能力。
2. 不重复镜像里通常已有的基础命令：如 `cat`、`ls`、`cp`、`mv`、`rm`、`mkdir`、`echo`、`printf`、`test`、`true`、`false`、`sleep` 等基础 shell/coreutils 能力，除非后续证明它们在目标环境中经常缺失且排障价值显著。

因此，本文档中的设计重点是“够用且可验证”的常用能力，而不是追求对所有原生命令参数的完整复刻。

测试上，`✅ 一致` 与 `⚠️ 部分一致` 条目都必须在 `docs/TEST-DESIGN.md` / `docs/TEST-CASES.md` 中映射到对应强度的案例；其中行为类参数和组合参数不能只验证“命令能跑”，必须证明参数确实改变了输出或执行路径。

文档承诺约束：

- `✅ 一致`：仅用于核心语义、退出码和常见输出契约都已与原生命令对齐的条目
- `✅ 常用一致`：用于高频使用路径已对齐，但边角行为、低频格式或环境相关细节不承诺完全同形的条目
- `⚠️ 部分一致`：必须明确写出差异边界，不能只写“精简实现”
- `🆕 gobox扩展`：表示 gobox 自有能力；“对应原生命令参数/参考基线”列仅可填写近似参考工具或留空，不表示一一兼容映射
- 当“对应原生命令参数/参考基线”列写的是参考工具、命令家族或近似基线，而不是明确的一一对应参数时，默认应优先使用 `✅ 常用一致` 或 `🆕 gobox扩展`
- `docs/CMD-DESIGN.md`、`docs/TEST-DESIGN.md`、`docs/TEST-CASES.md` 的一致性标签与验证强度必须互相对齐，不能出现文档宣称“原生一致”但测试仅按 gobox 合同验证的情况

---

## 目录

- [Shell 辅助命令](#shell-辅助命令)
- [文件系统命令](#文件系统命令)
- [文本处理命令](#文本处理命令)
- [网络命令](#网络命令)
- [进程命令](#进程命令)
- [磁盘命令](#磁盘命令)

---

## Shell 辅助命令

### alias

`alias` 不对应 Linux 的独立原生命令，而是 gobox 提供的 shell 集成辅助能力，用于批量生成当前 shell 可直接 `source` 的 alias/unalias 片段，简化 `gobox <subcommand>` 的日常输入。

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox alias` | N/A | 🆕 gobox扩展 | 输出 bash alias 片段，将已注册命令映射为 `alias ps='gobox ps'` 这一类快捷方式 |
| `gobox alias -u` | N/A | 🆕 gobox扩展 | 输出 bash unalias 片段，撤销由 `gobox alias` 注入的快捷方式 |
| `gobox alias -h` | N/A | 🆕 gobox扩展 | 显示帮助信息 |
| `gobox alias` 注入 `gobox_alias_type=bash` | N/A | 🆕 gobox扩展 | 标记当前 alias 类型，`gobox alias -u` 会基于该环境变量做一致性校验，避免误清理非 bash 场景 |

---

## 文件系统命令

### find

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
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

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox du -h` | `du -h` | ✅ 一致 | 以人类可读格式显示文件大小（KB、MB、GB） |
| `gobox du -s` | `du -s` | ✅ 一致 | 汇总显示，只显示每个参数目录的总大小 |
| `gobox du -a` | `du -a` | ✅ 一致 | 显示文件和目录项 |
| `gobox du -c` | `du -c` | ✅ 一致 | 输出 total 汇总行 |
| `gobox du -d N, --max-depth N` | `du -d N` | ✅ 一致 | 限制目录输出深度 |
| `gobox du --exclude PATTERN` | `du --exclude` | ⚠️ 部分一致 | 按 shell-style 模式排除路径 |
| `gobox du -x` | `du -x` | ⚠️ 部分一致 | 不跨文件系统遍历 |
| `gobox du --apparent-size` | `du --apparent-size` | ✅ 一致 | 使用文件表观大小而非已分配块数 |

### df

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox df` | `df` | ⚠️ 部分一致 | 显示文件系统容量、已用、可用和挂载点；环境敏感挂载项与原生命令不承诺全集同形 |
| `gobox df -h` | `df -h` | ⚠️ 部分一致 | 以人类可读格式显示容量 |
| `gobox df -T` | `df -T` | ⚠️ 部分一致 | 显示文件系统类型 |
| `gobox df -i` | `df -i` | ⚠️ 部分一致 | 显示 inode 使用情况 |
| `gobox df PATH...` | `df PATH...` | ⚠️ 部分一致 | 只显示指定路径所在文件系统 |
| `gobox df -H` | `df -H` | ⚠️ 部分一致 | 使用 SI 单位显示容量 |
| `gobox df -a` | `df -a` | ⚠️ 部分一致 | 包含默认隐藏的文件系统 |
| `gobox df -l` | `df -l` | ⚠️ 部分一致 | 仅显示本地文件系统 |
| `gobox df -t TYPE` | `df -t` | ⚠️ 部分一致 | 仅显示指定文件系统类型 |
| `gobox df -x TYPE` | `df -x` | ⚠️ 部分一致 | 排除指定文件系统类型 |
| `gobox df --total` | `df --total` | ⚠️ 部分一致 | 输出 total 汇总行 |
| `gobox df -P` | `df -P` | ⚠️ 部分一致 | 使用 POSIX 风格表头 |

### readpath

`readpath` 合并 `realpath` 与 `readlink` 的常用能力，用于在精简环境中解析路径、规范化路径或读取符号链接目标。

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox readpath FILE...` | `realpath FILE...` | ✅ 一致 | 输出规范化绝对路径 |
| `gobox readpath -f, --canonicalize FILE...` | `readlink -f` | ✅ 一致 | 跟随路径中的符号链接并规范化 |
| `gobox readpath -e, --canonicalize-existing FILE...` | `realpath -e` | ✅ 一致 | 要求路径所有组件必须存在 |
| `gobox readpath -m, --canonicalize-missing FILE...` | `realpath -m` | ✅ 一致 | 允许路径组件不存在并做词法规范化 |
| `gobox readpath -l, --readlink FILE...` | `readlink FILE...` | ✅ 一致 | 按 `readlink` 语义读取符号链接目标 |
| `gobox readpath -n, --no-newline FILE...` | `readlink -n` | ✅ 一致 | 输出末尾不追加换行 |
| `gobox readpath -q, --quiet FILE...` | `realpath -q` | ✅ 一致 | 抑制大多数错误信息 |
| `gobox readpath -z, --zero FILE...` | `realpath -z` / `readlink -z` | ✅ 一致 | 使用 NUL 分隔输出 |

### stat

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox stat FILE...` | `stat FILE...` | ⚠️ 部分一致 | 输出文件类型、大小、权限、时间等元信息；字段覆盖常用子集，文本排版不承诺完全同形 |
| `gobox stat -L, --dereference FILE...` | `stat -L` | ⚠️ 部分一致 | 跟随符号链接，显示目标文件信息 |
| `gobox stat -f, --file-system FILE...` | `stat -f` | ⚠️ 部分一致 | 输出文件系统信息 |
| `gobox stat -c, --format FORMAT FILE...` | `stat -c` | ⚠️ 部分一致 | 支持常用格式字段子集 |
| `gobox stat -t, --terse FILE...` | `stat -t` | ⚠️ 部分一致 | 使用简洁单行格式输出 |

### truncate

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox truncate -s SIZE FILE...` | `truncate -s` | ✅ 一致 | 设置文件大小 |
| `gobox truncate -c, --no-create -s SIZE FILE...` | `truncate -c` | ✅ 一致 | 文件不存在时不创建 |
| `gobox truncate -r RFILE FILE...` | `truncate -r` | ✅ 一致 | 以参考文件大小设置目标文件 |
| `gobox truncate -s K/M/G FILE...` | `truncate -s K/M/G` | ✅ 一致 | 支持常用大小单位后缀 |
| `gobox truncate -s +SIZE/-SIZE FILE...` | `truncate -s +SIZE/-SIZE` | ✅ 一致 | 相对当前大小扩展或收缩 |

---

## 文本处理命令

### head

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox head -n NUM` | `head -n` | ✅ 一致 | 打印前 NUM 行（默认 10） |
| `gobox head -c NUM` | `head -c` | ✅ 一致 | 打印前 NUM 字节 |
| `gobox head -q` | `head -q` | ✅ 一致 | 不显示文件名标题 |
| `gobox head -h` | `head --help` | ✅ 一致 | 显示帮助信息 |

### tail

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox tail -n NUM` | `tail -n` | ✅ 一致 | 打印最后 NUM 行（默认 10） |
| `gobox tail -f` | `tail -f` | ✅ 一致 | 跟踪文件变化，输出追加的数据 |
| `gobox tail --follow=name` | `tail --follow=name` | ✅ 一致 | 通过文件名跟踪（处理日志轮转） |
| `gobox tail --retry` | `tail --retry` | ✅ 一致 | 文件不存在时持续重试 |
| `gobox tail -q` | `tail -q` | ✅ 一致 | 不显示文件名标题 |
| `gobox tail -s SEC` | `tail -s` | ✅ 一致 | 每次轮询间隔秒数（默认 1） |
| `gobox tail --pid=PID` | `tail --pid` | ✅ 一致 | 指定进程 PID 退出时停止跟踪 |

### grep

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox grep -E` | `grep -E` | ✅ 一致 | 使用扩展正则表达式（ERE） |
| `gobox grep -F` | `grep -F` | ✅ 一致 | 将模式作为固定字符串（非正则） |
| `gobox grep -c` | `grep -c` | ✅ 一致 | 仅显示匹配行的计数 |
| `gobox grep -i` | `grep -i` | ✅ 一致 | 忽略大小写 |
| `gobox grep --line-buffered` | `grep --line-buffered` | ✅ 一致 | 行缓冲（每行后刷新） |
| `gobox grep -n` | `grep -n` | ✅ 一致 | 显示行号 |
| `gobox grep -o` | `grep -o` | ✅ 一致 | 仅显示匹配的部分 |
| `gobox grep -q` | `grep -q` | ✅ 一致 | 静默模式，仅返回退出码 |
| `gobox grep -r` | `grep -r` | ✅ 一致 | 递归搜索目录 |
| `gobox grep -v` | `grep -v` | ✅ 一致 | 反向匹配（显示不匹配的行） |
| `gobox grep -A NUM` | `grep -A` | ✅ 一致 | 显示匹配行后 NUM 行上下文 |
| `gobox grep -B NUM` | `grep -B` | ✅ 一致 | 显示匹配行前 NUM 行上下文 |
| `gobox grep -C NUM` | `grep -C` | ✅ 一致 | 显示匹配行前后 NUM 行上下文 |
| `gobox grep --include=PATTERN` | `grep --include` | ✅ 一致 | 递归搜索时仅扫描匹配文件名 |
| `gobox grep --exclude-dir=DIR` | `grep --exclude-dir` | ✅ 一致 | 递归搜索时排除指定目录 |
| `gobox grep -l` | `grep -l` | ✅ 一致 | 仅输出有匹配的文件名 |
| `gobox grep -L` | `grep -L` | ✅ 一致 | 仅输出无匹配的文件名 |

### sed

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox sed -n` | `sed -n` | ✅ 一致 | 抑制自动打印模式空间 |
| `gobox sed -i[SUFFIX]` | `sed -i` | ✅ 一致 | 原地编辑（若提供 SUFFIX 则备份） |
| `gobox sed -e SCRIPT` | `sed -e` | ✅ 一致 | 添加脚本命令 |
| `gobox sed -f FILE` | `sed -f` | ✅ 一致 | 从文件添加脚本命令 |
| `gobox sed -h` | `sed --help` | ✅ 一致 | 显示帮助信息 |

**sed 命令：**

| gobox 命令 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
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

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
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

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox uniq -c` | `uniq -c` | ✅ 一致 | 显示每行出现次数 |
| `gobox uniq -d` | `uniq -d` | ✅ 一致 | 仅显示重复行 |
| `gobox uniq -u` | `uniq -u` | ✅ 一致 | 仅显示唯一行 |
| `gobox uniq -i` | `uniq -i` | ✅ 一致 | 忽略大小写 |
| `gobox uniq -w N` | `uniq -w` | ✅ 一致 | 最多比较 N 个字符 |
| `gobox uniq -f N` | `uniq -f` | ✅ 一致 | 跳过前 N 个字段 |

> 注意：uniq 仅对排序后的相邻重复行有效（需先排序）

### wc

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox wc -l` | `wc -l` | ✅ 一致 | 打印行数 |
| `gobox wc -w` | `wc -w` | ✅ 一致 | 打印词数 |
| `gobox wc -c` | `wc -c` | ✅ 一致 | 打印字节数 |
| `gobox wc -m` | `wc -m` | ✅ 一致 | 打印字符数 |
| `gobox wc -L` | `wc -L` | ✅ 一致 | 打印最长行长度 |

### hex

`hex` 面向容器排障中的十六进制查看与编解码场景。`--dump` 对齐 `hexdump` 常用查看能力，`--encode` / `--decode` 是 gobox 扩展，用于轻量替代 `xxd -p` / `xxd -r -p` 场景。

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox hex --dump -C FILE...` | `hexdump -C` | ⚠️ 部分一致 | canonical 十六进制与 ASCII 对照输出；常用查看语义一致，排版细节不承诺完全同形 |
| `gobox hex --dump -n LEN FILE...` | `hexdump -n` | ⚠️ 部分一致 | 只读取前 LEN 字节 |
| `gobox hex --dump -s OFFSET FILE...` | `hexdump -s` | ⚠️ 部分一致 | 从指定偏移开始读取 |
| `gobox hex --dump -v FILE...` | `hexdump -v` | ⚠️ 部分一致 | 不折叠重复输出行 |
| `gobox hex --dump -e FORMAT FILE...` | `hexdump -e` | ⚠️ 部分一致 | 自定义 dump 输出格式，先支持常用格式子集 |
| `gobox hex --encode FILE...` | `xxd -p` / `od -An -tx1` | 🆕 gobox扩展 | 将输入编码为连续 lowercase hex 文本 |
| `gobox hex --decode FILE...` | `xxd -r -p` | 🆕 gobox扩展 | 将 hex 文本解码为原始字节 |
| `gobox hex --decode -o FILE INPUT` | `xxd -r -p > FILE` | 🆕 gobox扩展 | 将解码结果写入指定文件 |

### base64

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox base64 FILE...` | `base64 FILE...` | ✅ 一致 | 默认编码为 base64 文本 |
| `gobox base64 -d, --decode FILE...` | `base64 -d` | ✅ 一致 | 将 base64 文本解码为原始字节 |
| `gobox base64 -w COLS, --wrap COLS FILE...` | `base64 -w` | ✅ 一致 | 每 COLS 字符换行，`0` 表示不换行 |
| `gobox base64 -i, --ignore-garbage FILE...` | `base64 -i` | ✅ 一致 | 解码时忽略非 base64 字符 |
| `gobox base64 -o FILE INPUT` | `base64 > FILE` | 🆕 gobox扩展 | 将编码或解码结果写入指定文件 |

### strings

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox strings FILE...` | `strings FILE...` | ✅ 一致 | 从文件或 stdin 提取可打印字符串 |
| `gobox strings -n LEN FILE...` | `strings -n` | ✅ 一致 | 设置最短字符串长度 |
| `gobox strings -f FILE...` | `strings -f` | ✅ 一致 | 输出前带文件名 |
| `gobox strings -t o\|d\|x FILE...` | `strings -t` | ✅ 一致 | 输出字符串所在偏移，支持八进制、十进制、十六进制 |
| `gobox strings -a FILE...` | `strings -a` | ✅ 一致 | 扫描整个文件 |

### diff

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox diff FILE1 FILE2` | `diff FILE1 FILE2` | ✅ 常用一致 | 按行比较两个文本文件 |
| `gobox diff -u FILE1 FILE2` | `diff -u` | ✅ 常用一致 | 输出 unified diff |
| `gobox diff -q FILE1 FILE2` | `diff -q` | ✅ 一致 | 仅报告文件是否不同 |
| `gobox diff -r DIR1 DIR2` | `diff -r` | ✅ 常用一致 | 递归比较目录，按路径排序输出 |
| `gobox diff -N FILE1 FILE2` | `diff -N` | ✅ 常用一致 | 将缺失文件视为空文件 |
| `gobox diff --strip-trailing-cr FILE1 FILE2` | `diff --strip-trailing-cr` | ✅ 一致 | 比较时忽略行尾 CR |
| `gobox diff FILE -` | `diff FILE -` | ✅ 一致 | 将 stdin 作为其中一侧输入参与比较 |
| `gobox diff binary1 binary2` | `diff` | ✅ 常用一致 | 二进制文件仅报告差异，不转储内容 |
| `gobox diff equal1 equal2` | `diff` | ✅ 一致 | 相同文件无输出且退出码为 0 |

---

## 网络命令

### curl

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
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
| `gobox curl -T, --upload-file FILE` | `curl -T` | ✅ 一致 | 通过 PUT 上传文件 |
| `gobox curl -F, --form FIELD=VALUE` | `curl -F` | ✅ 一致 | 发送 multipart/form-data 表单 |
| `gobox curl -c, --concurrent=N` | `ab -c` (Apache Bench) | 🆕 gobox扩展 | 并发请求数（基准测试） |
| `gobox curl -n, --requests=N` | `ab -n` (Apache Bench) | 🆕 gobox扩展 | 总请求数（基准测试） |
| `gobox curl --warmup=N` | `wrk --warmup` | 🆕 gobox扩展 | 预热请求数 |
| `gobox curl -t, --timeout=SEC` | `curl -m` | ✅ 一致 | 请求超时时间 |

### nc/netcat

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
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

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox netstat -a, --all` | `netstat -a` | ⚠️ 部分一致 | 参数已接入并保留兼容入口；当前实现不改变默认 socket 选择语义 |
| `gobox netstat -t, --tcp` | `netstat -t` | ✅ 一致 | 仅显示 TCP socket |
| `gobox netstat -u, --udp` | `netstat -u` | ✅ 一致 | 仅显示 UDP socket |
| `gobox netstat -x, --unix` | `netstat -x` | ✅ 一致 | 仅显示 Unix domain socket |
| `gobox netstat -l, --listening` | `netstat -l` | ✅ 一致 | 仅显示监听 socket |
| `gobox netstat -n, --numeric` | `netstat -n` | ⚠️ 部分一致 | gobox 当前输出本来就是数字地址/端口；该参数主要保留兼容语义 |
| `gobox netstat -p, --programs` | `netstat -p` | ✅ 常用一致 | 显示 PID/Program；常用关联语义对齐，权限受限时结果集合仍以当前 `/proc` 可见性为准 |
| `gobox netstat -4` | `netstat -4` | ✅ 一致 | 仅显示 IPv4 socket |
| `gobox netstat -6` | `netstat -6` | ✅ 一致 | 仅显示 IPv6 socket |
| `gobox netstat -e, --extend` | `netstat -e` | ✅ 常用一致 | 显示扩展列（User/Inode）；列存在性与常用字段语义对齐 |
| `gobox netstat -o, --timers` | `netstat -o` | ✅ 常用一致 | 显示 timer 信息；字段覆盖常用排障视角 |
| `gobox netstat -W, --wide` | `netstat -W` | ⚠️ 部分一致 | gobox 默认已不截断地址；该参数作为兼容入口接受 |
| `gobox netstat -r` | `netstat -r` | ⚠️ 部分一致 | 显示 IPv4/IPv6 路由表 |
| `gobox netstat -i` | `netstat -i` | ⚠️ 部分一致 | 显示网络接口统计 |
| `gobox netstat -s` | `netstat -s` | ⚠️ 部分一致 | 显示协议统计 |
| `gobox netstat -c` | `netstat -c` | ⚠️ 部分一致 | 持续刷新输出 |
| `gobox netstat -tnlp` | `netstat -tnlp` | ✅ 常用一致 | 支持常见短参数合并 |
| `gobox netstat --help` | `netstat --help` | 🆕 gobox设计 | 帮助输出按功能分组，短长参数合并为单行展示 |
| `gobox netstat -port int` | 端口过滤 | 🆕 gobox扩展 | 按本地或远端端口精确过滤 |
| `gobox netstat -sort string` | 排序功能 | 🆕 gobox扩展 | 排序字段：recvq\|sendq\|local\|remote\|pid |
| `gobox netstat -state string` | 状态过滤 | 🆕 gobox扩展 | 按连接状态过滤，支持状态列表 |

### tw

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox tw -p, --port=PORT` | `httpd -p` / `python -m http.server` | ✅ 常用一致 | 监听端口（默认 8080）；提供轻量静态文件服务基线 |
| `gobox tw -d, --dir=DIR` | `httpd -d` / `python -m http.server` | ✅ 常用一致 | 服务目录（默认当前目录）；目录服务语义对齐常见轻量 HTTP 文件服务 |
| `gobox tw -r, --reuse` | socket reuse | 🆕 gobox扩展 | 启用 SO_REUSEADDR；用于提高重复绑定场景的可用性 |
| `gobox tw --bench` | `wrk` | 🆕 gobox扩展 | 基准测试模式 |

**基准测试端点：**
- `GET /ping` - 返回 200 OK 和 "pong"
- `POST /ping` - 回显请求体
- `POST /upload` - 接收文件上传

### nslookup/dig

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox nslookup @DNS_SERVER` | `dig @DNS_SERVER` | ✅ 常用一致 | 使用指定 DNS 服务器；查询路径语义对齐，输出采用 gobox 简化格式 |
| `gobox nslookup -t TYPE, --type=TYPE` | `dig -t` | ✅ 常用一致 | 查询类型（A/AAAA/TXT/CNAME/NS/MX/SRV）；结果展示为 gobox 简化格式 |
| `gobox nslookup +short` | `dig +short` | ✅ 常用一致 | 简短输出（仅显示答案） |
| `gobox nslookup +noall +answer` | `dig +noall +answer` | ✅ 常用一致 | 仅显示答案部分 |
| `gobox nslookup +tcp` | `dig +tcp` | ✅ 常用一致 | 使用 TCP 替代 UDP |

### ifstat

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox ifstat -A` | `ifstat -a` | ✅ 常用一致 | 显示所有接口（包括虚拟接口）；接口枚举策略以 gobox `/sys` 视图为准 |
| `gobox ifstat -a` | `ifstat -a` | ✅ 常用一致 | 显示绝对值（累计值）；高频读数语义对齐 |
| `gobox ifstat -d` | `netstat -ed` (丢包统计) | 🆕 gobox扩展 | 显示丢包计数 |
| `gobox ifstat -e` | `netstat -e` (错误统计) | 🆕 gobox扩展 | 显示错误包计数 |
| `gobox ifstat -i string` | `ifstat -i` | ✅ 常用一致 | 指定网络接口（逗号分隔） |
| `gobox ifstat -n int` | `ifstat -n` | ✅ 常用一致 | 采样次数（0=连续） |
| `gobox ifstat -p int` | `ifstat -p` | ✅ 常用一致 | 采样间隔秒数（默认 1） |

### ip

`ip` 仅设计为容器排障所需的只读子集，不实现完整 iproute2，也不支持修改网络配置的 add/del/set 等操作。

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox ip addr` / `gobox ip a` | `ip addr` | ⚠️ 部分一致 | 显示接口地址、作用域和状态，只实现只读排障子集 |
| `gobox ip -o addr` | `ip -o addr` | ⚠️ 部分一致 | 单行显示接口地址，便于脚本处理 |
| `gobox ip link` / `gobox ip l` | `ip link` | ⚠️ 部分一致 | 显示接口状态、MTU、MAC 和 flags |
| `gobox ip -s link` | `ip -s link` | ⚠️ 部分一致 | 显示接口收发包和字节统计 |
| `gobox ip route` / `gobox ip r` | `ip route` | ⚠️ 部分一致 | 显示 IPv4 路由表和默认路由 |
| `gobox ip neigh` / `gobox ip n` | `ip neigh` | ⚠️ 部分一致 | 显示邻居/ARP 表 |

### np/netping

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox np -I string` | `ping -I` | ✅ 常用一致 | 使用的网络接口；拨号/探测路径遵循指定接口 |
| `gobox np -W int` | `ping -W` | ✅ 常用一致 | 超时时间（秒，默认 5） |
| `gobox np -arp` | `arping -I` | 🆕 gobox扩展 | ARP 模式 |
| `gobox np -c int` | `ping -c` | ✅ 常用一致 | 数据包数量（默认 4） |
| `gobox np -flood` | `ping -f` | ✅ 常用一致 | 洪水模式（最大速度）；节流和输出细节不承诺完全同形 |
| `gobox np -i int` | `ping -i` | ⚠️ 部分一致 | 数据包间隔（微秒，默认 1000000），ping 默认秒 |
| `gobox np -icmp` | `ping` | ✅ 常用一致 | ICMP 模式 |
| `gobox np -l int` | 长连接探测 | 🆕 gobox扩展 | 长连接模式 |
| `gobox np -p int` | `nc -p` | 🆕 gobox扩展 | 目标端口 |
| `gobox np -q` | `ping -q` | ✅ 常用一致 | 静默模式；输出收敛为 summary-only 视图 |
| `gobox np -s int` | 源端口绑定 | 🆕 gobox扩展 | 源端口 |
| `gobox np -scan` | `nc -z` | 🆕 gobox扩展 | 端口扫描模式 |
| `gobox np -tcp` | `nc` | ✅ 常用一致 | TCP 模式（默认）；连通性探测语义对齐常用 `nc` 用法 |
| `gobox np -udp` | `nc -u` | ✅ 常用一致 | UDP 模式 |
| `gobox np -v` | `ping -v` | ✅ 常用一致 | 详细输出；相对 quiet 模式增加逐次诊断信息 |
| `gobox np -w int` | 工作池并发 | 🆕 gobox扩展 | 并发工作数（默认 1） |

---

## 进程命令

### ps

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox ps -e` | `ps -e` | ✅ 一致 | 显示所有进程；默认 `CMD` 列按原生命令习惯优先显示 `comm`/可执行名 |
| `gobox ps -A` | `ps -A` | ✅ 一致 | 显示所有进程；默认 `CMD` 列按原生命令习惯优先显示 `comm`/可执行名 |
| `gobox ps -f` | `ps -f` | ✅ 一致 | 切换到 full-format，增加 `UID`/`PPID`/`STIME`/`TTY`/`TIME`/`CMD` 等列，并显示完整命令行参数 |
| `gobox ps -F` | `ps -F` | ⚠️ 部分一致 | extra full 格式输出 |
| `gobox ps -u USER` | `ps -u` | ✅ 常用一致 | 按用户或 UID 过滤 |
| `gobox ps -p PID` | `ps -p` | ✅ 常用一致 | 按 PID 过滤 |
| `gobox ps -C NAME` | `ps -C` | ✅ 常用一致 | 按命令名过滤 |
| `gobox ps -comm string` | `pgrep -x` | ✅ 一致 | 按进程名精确匹配 |
| `gobox ps -full string` | `pgrep -f` | ✅ 一致 | 按完整命令行正则匹配；输出默认也保留完整命令行，便于核对命中过滤结果 |
| `gobox ps -o FIELDS` | `ps -o` | ⚠️ 部分一致 | 自定义输出字段，支持常用字段子集 |
| `gobox ps --sort FIELD` | `ps --sort` | ⚠️ 部分一致 | GNU 风格排序字段 |
| `gobox ps aux` | BSD 风格 `ps` 字母参数 | ⚠️ 部分一致 | BSD 风格下默认选择“自己且有 TTY”的进程；`a` 放开 only-yourself 限制，`x` 放开 must-have-tty 限制，`u` 切换到 user-oriented 列布局 |
| `gobox ps -ww` | `ps -ww` | ✅ 一致 | 取消 `ps` 默认宽度截断，尽量完整显示单行 `CMD` |
| `gobox ps -i int` | `vmstat -i` (采样间隔) | 🆕 gobox扩展 | CPU 采样间隔（毫秒，默认 500） |
| `gobox ps -maxcmd N` | `ps -o pid,cmd` (截断) | 🆕 gobox扩展 | 指定命令列最大长度（0=无限制）；显式指定时优先于默认 TTY 宽度策略 |
| `gobox ps -n int` | `ps --no-headers` (管道 head) | 🆕 gobox扩展 | 仅显示前 N 个进程（0=显示全部） |
| `gobox ps -r` | reverse sort (gobox-only) | 🆕 gobox扩展 | 反向排序；不复用原生 `ps -r` 的“仅显示 running 进程”语义 |

> 宽度语义说明：`ps` 默认在 TTY 下按当前终端宽度截断最后一列命令文本，非 TTY 输出保留完整单行命令；`-ww` 用于关闭该默认截断。`-f` 只负责切换到 full-format，多显示列，不负责控制宽度策略。帮助信息统一主推 `--sort` 和 `-maxcmd`；`-l N` 仅保留兼容别名，不再作为公开主路径。

### top

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox top -d int` | `top -d` | ✅ 一致 | 每次更新间隔秒数（默认 2） |
| `gobox top -n int` | `top -n` | ✅ 一致 | 迭代次数（0=无限，默认即无限） |
| `gobox top -b` | `top -b` | ✅ 常用一致 | batch 模式输出 |
| `gobox top -p PID` | `top -p` | ✅ 常用一致 | 只显示指定 PID |
| `gobox top -u USER` | `top -u` | ✅ 常用一致 | 按用户过滤 |
| `gobox top -H` | `top -H` | ⚠️ 部分一致 | 接受线程模式参数，当前仍为进程级输出 |
| `gobox top -i` | `top -i` | ⚠️ 部分一致 | 隐藏采样 CPU 为 0 的进程 |
| `gobox top -c` | `top -c` | ✅ 常用一致 | 显示完整命令行 |
| `gobox top -o FIELD` | `top -o` | ⚠️ 部分一致 | 按字段排序 |
| `gobox top -r` | reverse sort (gobox-only) | 🆕 gobox扩展 | 反向排序开关；不复用原生 `top -r` 的语义 |
| `gobox top -sort string` | `top -o` (排序键) | 🆕 gobox扩展 | 排序字段：pid\|cpu\|rss\|vms\|cmd |

> 注意：gobox top 是 top 命令的简化实现，使用独立的实时采样与渲染路径，不再复用 `ps` 的输出。

### free

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox free` | `free` | ⚠️ 部分一致 | 显示内存总量、已用、可用、buffer/cache 和 swap；常用数值语义一致，表格排版为精简实现 |
| `gobox free -h` | `free -h` | ⚠️ 部分一致 | 以人类可读格式显示内存 |
| `gobox free -m` | `free -m` | ⚠️ 部分一致 | 以 MiB 显示内存 |
| `gobox free -g` | `free -g` | ⚠️ 部分一致 | 以 GiB 显示内存 |
| `gobox free -s SEC -c COUNT` | `free -s -c` | ⚠️ 部分一致 | 按间隔重复采样指定次数 |

### xargs

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox xargs -I string` | `xargs -I` | ✅ 一致 | 使用自定义占位符替换输入 |
| `gobox xargs -i string` | `xargs -i` | ✅ 一致 | 替换字符串（默认占位符 `{}`） |
| `gobox xargs -d string` | `xargs -d` | ✅ 一致 | 输入分隔符（默认换行符） |
| `gobox xargs -n int` | `xargs -n` | ✅ 一致 | 每次命令调用的最大参数数 |
| `gobox xargs -P int` | `xargs -P` | ✅ 一致 | 最大并行进程数（默认 1） |
| `gobox xargs -r` | `xargs -r` | ✅ 一致 | 无输入时不运行命令 |
| `gobox xargs -t` | `xargs -t` | ✅ 一致 | 执行前打印命令 |

> 注意：默认命令是 `echo`，即 `gobox xargs` 等同于 `xargs echo`。

### kill

`kill` 合并 `kill` 与 `pkill` 的常用能力：PID 模式对齐 `kill`，匹配模式对齐 `pkill`。

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox kill PID...` | `kill PID...` | ✅ 一致 | 向指定 PID 发送默认 `TERM` 信号 |
| `gobox kill -l, --list [SIGNAL]` | `kill -l` | ⚠️ 部分一致 | 列出常用信号，信号表不是完整 GNU 输出 |
| `gobox kill -s SIGNAL PID...` | `kill -s` | ✅ 一致 | 指定要发送的信号 |
| `gobox kill -SIGNAL PID...` | `kill -SIGNAL` | ✅ 一致 | 使用短格式指定信号 |
| `gobox kill -f PATTERN` | `pkill -f` | ⚠️ 部分一致 | 按完整命令行匹配进程；匹配范围和权限边界为精简实现 |
| `gobox kill -x PATTERN` | `pkill -x` | ⚠️ 部分一致 | 按进程名精确匹配 |
| `gobox kill -P PPID` | `pkill -P` | ⚠️ 部分一致 | 匹配指定父进程下的子进程 |
| `gobox kill -n [-f\|-x] PATTERN` | `pkill -n` | ⚠️ 部分一致 | 只匹配最新启动的进程 |
| `gobox kill -o [-f\|-x] PATTERN` | `pkill -o` | ⚠️ 部分一致 | 只匹配最早启动的进程 |
| `gobox kill --dry-run PATTERN` | gobox-only | 🆕 gobox扩展 | 只打印将要匹配的进程，不发送信号 |

### lsof

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox lsof` | `lsof` | ⚠️ 部分一致 | 列出当前可见的打开文件，基于 `/proc` 的精简输出 |
| `gobox lsof -p PID` | `lsof -p` | ⚠️ 部分一致 | 按进程 ID 过滤 |
| `gobox lsof -c NAME` | `lsof -c` | ⚠️ 部分一致 | 按命令名前缀过滤 |
| `gobox lsof -i` | `lsof -i` | ⚠️ 部分一致 | 仅列出网络文件 |
| `gobox lsof -iTCP` | `lsof -iTCP` | ⚠️ 部分一致 | 仅列出 TCP 网络连接 |
| `gobox lsof -iUDP` | `lsof -iUDP` | ⚠️ 部分一致 | 仅列出 UDP 网络连接 |
| `gobox lsof -i :PORT` | `lsof -i :PORT` | ⚠️ 部分一致 | 按端口过滤网络连接 |
| `gobox lsof -n` | `lsof -n` | ⚠️ 部分一致 | 不解析主机名 |
| `gobox lsof -P` | `lsof -P` | ⚠️ 部分一致 | 不解析端口服务名 |
| `gobox lsof -t` | `lsof -t` | ✅ 一致 | 仅输出 PID |
| `gobox lsof FILE...` | `lsof FILE...` | ⚠️ 部分一致 | 查找打开指定文件的进程 |

### watch

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox watch COMMAND...` | `watch COMMAND...` | ⚠️ 部分一致 | 周期性执行命令并刷新输出；标题和终端重绘行为为精简实现 |
| `gobox watch -n SEC COMMAND...` | `watch -n` | ⚠️ 部分一致 | 设置执行间隔秒数 |
| `gobox watch -t COMMAND...` | `watch -t` | ⚠️ 部分一致 | 不显示标题行 |

### timeout

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox timeout DURATION COMMAND...` | `timeout DURATION COMMAND...` | ✅ 一致 | 限制命令最大运行时间 |
| `gobox timeout -s SIGNAL DURATION COMMAND...` | `timeout -s` | ✅ 一致 | 超时后发送指定信号 |
| `gobox timeout -k KILL_AFTER DURATION COMMAND...` | `timeout -k` | ✅ 一致 | 首次信号后仍未退出则强制 kill |
| `gobox timeout --preserve-status DURATION COMMAND...` | `timeout --preserve-status` | ✅ 常用一致 | 超时时尽量保留子命令退出状态；常用保留状态语义已对齐 |
| `gobox timeout 1s/1m/1h COMMAND...` | `timeout 1s/1m/1h` | ✅ 一致 | 支持常用 duration 后缀 |

---

## 磁盘命令

### iostat

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox iostat -i sec` | `iostat interval` | ⚠️ 部分一致 | 采样间隔秒数 |
| `gobox iostat -n count` | `iostat count` | ⚠️ 部分一致 | 采样次数 |
| `gobox iostat interval [count]` | `iostat interval [count]` | ✅ 常用一致 | 支持位置参数形式的采样间隔与次数 |
| `gobox iostat -H` | `iostat -h` | ✅ 常用一致 | 人类可读格式 |
| `gobox iostat -z` | `iostat -z` | ✅ 常用一致 | 跳过零活动设备 |
| `gobox iostat --cgroup` | gobox-only | 🆕 gobox扩展 | 切换到基于 cgroup `io.stat`/`blkio` 的旧输出格式 |
| `gobox iostat --help` | `iostat --help` | 🆕 gobox设计 | 帮助输出补充位置参数、列说明和示例 |

### ioperf

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox ioperf --bs string` | `fio --bs` | ✅ 常用一致 | 块大小，如 4k, 8k（默认 "4k"） |
| `gobox ioperf --direct int` | `fio --direct` | ✅ 常用一致 | 使用 O_DIRECT 绕过缓存（0 或 1） |
| `gobox ioperf --filename string` | `fio --filename` | ✅ 常用一致 | 测试文件路径（默认 "/tmp/ioperf_test"） |
| `gobox ioperf --fsync int` | `fio --fsync` | ✅ 常用一致 | 每次写后执行 fsync（0 或 1） |
| `gobox ioperf --group_reporting` | `fio --group_reporting` | ✅ 常用一致 | 聚合多任务报告；报告格式为 gobox 自身样式 |
| `gobox ioperf --iodepth int` | `fio --iodepth` | ✅ 常用一致 | 队列深度（默认 1） |
| `gobox ioperf --write_hist_log string` | `fio --write_hist_log --log_hist_msec` | ✅ 常用一致 | 输出并落盘延迟直方图日志 |
| `gobox ioperf --numjobs int` | `fio --numjobs` | ✅ 常用一致 | 并行任务数量（默认 1） |
| `gobox ioperf --percentile_list string` | `fio --percentile_list` | ✅ 常用一致 | 报告指定延迟百分位列表 |
| `gobox ioperf --rate string` | `fio --rate` | ✅ 常用一致 | 速率限制（如 100M） |
| `gobox ioperf --runtime int` | `fio --runtime` | ✅ 常用一致 | 运行时间（秒） |
| `gobox ioperf --rw string` | `fio --rw` | ✅ 常用一致 | I/O 模式：read/write/randread/randwrite/readwrite（默认 "read"） |
| `gobox ioperf --rwmixread int` | `fio --rwmixread` | ✅ 常用一致 | 读操作比例（0-100，用于 readwrite，默认 50） |
| `gobox ioperf --size string` | `fio --size` | ✅ 常用一致 | 总 I/O 大小（如 1G，默认 "1G"） |
| `gobox ioperf --sync string` | `fio --sync` | ✅ 常用一致 | 同步写模式：none/sync/dsync（兼容 0/1） |
| `gobox ioperf --time_based` | `fio --time_based` | ✅ 常用一致 | 基于时间运行 |

### md5sum

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox md5sum -c, --check` | `md5sum -c` | ✅ 一致 | 从文件验证校验和 |
| `gobox md5sum --tag` | `md5sum --tag` | ✅ 一致 | BSD 风格输出 |
| `gobox md5sum -q, --quiet` | `md5sum -q` | ✅ 一致 | 安静模式，只显示校验和 |
| `gobox md5sum -s, --status` | `md5sum -s` | ✅ 一致 | 仅返回状态码 |
| `gobox md5sum -w, --warn` | `md5sum -w` | ✅ 一致 | 警告格式错误的行 |

### sha256sum

| gobox 参数 | 对应原生命令参数/参考基线 | 实现一致性 | 功能说明 |
|------------|---------------|------------|----------|
| `gobox sha256sum FILE...` | `sha256sum FILE...` | ✅ 一致 | 计算文件或 stdin 的 SHA-256 校验和 |
| `gobox sha256sum -c, --check` | `sha256sum -c` | ✅ 一致 | 从文件验证 SHA-256 校验和 |
| `gobox sha256sum --tag` | `sha256sum --tag` | ✅ 一致 | BSD 风格输出 |
| `gobox sha256sum -q, --quiet` | `sha256sum -q` | ✅ 一致 | 安静模式，只显示校验和 |
| `gobox sha256sum -s, --status` | `sha256sum -s` | ✅ 一致 | 仅返回状态码 |
| `gobox sha256sum -w, --warn` | `sha256sum -w` | ✅ 一致 | 警告格式错误的行 |

---

## 实现一致性说明

| 标记 | 含义 |
|------|------|
| ✅ 一致 | 与原生命令核心语义、退出码和常见输出契约一致 |
| ✅ 常用一致 | 高频用法一致，但边角行为、低频格式或环境相关细节不承诺完全同形 |
| ⚠️ 部分一致 | 功能相似但存在已知差异、限制或环境相关偏差，条目需写明边界 |
| 🆕 gobox扩展 | 原生命令无此参数或该语义为 gobox 自有设计 |
| 📝 计划支持 | 已纳入设计，尚未实现或尚未完成 parity 对齐 |

---

## 命令索引

| 命令 | 类别 | 功能 |
|------|------|------|
| find | 文件系统 | 文件搜索 |
| du | 文件系统 | 磁盘使用统计 |
| df | 文件系统 | 文件系统容量 |
| readpath | 文件系统 | 路径解析 |
| stat | 文件系统 | 文件元信息 |
| truncate | 文件系统 | 文件大小调整 |
| head | 文本处理 | 显示文件头部 |
| tail | 文本处理 | 显示文件尾部 |
| grep | 文本处理 | 文本搜索 |
| sed | 文本处理 | 流编辑器 |
| sort | 文本处理 | 排序 |
| uniq | 文本处理 | 去重 |
| wc | 文本处理 | 计数 |
| hex | 文本处理 | 十六进制查看与编解码 |
| base64 | 文本处理 | base64 编解码 |
| strings | 文本处理 | 可打印字符串提取 |
| diff | 文本处理 | 文件差异比较 |
| curl | 网络 | HTTP 客户端 |
| nc | 网络 | 网络工具 |
| netstat | 网络 | 网络连接统计 |
| tw | 网络 | Web 服务器 |
| nslookup/dig | 网络 | DNS 查询 |
| ifstat | 网络 | 网络接口统计 |
| ip | 网络 | 网络配置只读查看 |
| np | 网络 | 网络 ping |
| ps | 进程 | 进程列表 |
| top | 进程 | 进程监控 |
| free | 进程 | 内存使用统计 |
| xargs | 进程 | 命令参数构建 |
| kill | 进程 | 发送信号/按模式结束进程 |
| lsof | 进程 | 打开文件查看 |
| watch | 进程 | 周期性执行命令 |
| timeout | 进程 | 限时执行命令 |
| iostat | 磁盘 | I/O 统计 |
| ioperf | 磁盘 | I/O 性能测试 |
| md5sum | 磁盘 | 校验和计算 |
| sha256sum | 磁盘 | SHA-256 校验和计算 |
