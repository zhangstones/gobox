# gobox Parity Test Cases

## 说明

本文件将 `docs/CMD-SPECS.md` 中的每个命令、每个参数映射为至少一个必要测试案例。

字段说明：

- `Case ID`: 唯一案例编号
- `Arg/Feature`: 被验证的参数或语义点
- `Mode`: `exact` / `structured` / `behavior` / `contract`
- `Native Baseline`: 原生命令或等价基线；若为 gobox 扩展则记为 `gobox-only`
- `Fixture`: 输入夹具类型
- `Core Assertion`: 核心断言

编写约定：

- 参数语义、兼容性标签、参考 baseline 与差异边界统一以 `docs/CMD-SPECS.md` 为准；本文件不重复定义产品规格
- 本文件只维护“参数到 case 的映射关系”和“case 想证明的核心语义”，不重复展开 `docs/TEST-DESIGN.md` 中的通用测试规范
- 所有 case 的断言强度、弱 case 禁止规则、参数生效证明规则、环境敏感命令约束、组合参数规则，统一以 `docs/TEST-DESIGN.md` 为准
- 若某条 `Core Assertion` 只能写成“关键字存在”“命令成功”或“输出不同”，则该 case 说明不完整，必须回到 `docs/TEST-DESIGN.md` 对照补强
- `Core Assertion` 应直接描述“要证明的语义”，而不是“打算怎么写测试代码”

维护顺序约束：

1. 若涉及命令纳入范围、设计边界或非目标变更，先更新 `docs/CMD-DESIGN.md`
2. 再更新 `docs/CMD-SPECS.md`
3. 再检查并更新 `docs/TEST-CASES.md`
4. 最后同步对应 unit/parity/smoke 测试代码

---

---

## 覆盖清单

以下命令已按 `docs/CMD-SPECS.md` 当前条目建立参数级 case；自动化测试按 exact、structured、behavior、contract 四类分别落地，无法稳定自动化的环境依赖项需在测试中显式说明或跳过。

- 文件系统：`find`、`du`、`df`、`readpath`、`stat`、`truncate`
- Shell 辅助：`alias`
- 文本处理：`head`、`tail`、`grep`、`sed`、`sort`、`uniq`、`wc`、`seq`、`rand`、`hex`、`base64`、`strings`、`diff`
- 网络：`curl`、`nc`、`netstat`、`tw`、`nslookup/dig`、`ifstat`、`ip`、`np`
- 进程：`ps`、`top`、`free`、`xargs`、`kill`、`lsof`、`watch`、`timeout`
- 磁盘：`iostat`、`ioperf`、`md5sum`、`sha256sum`

约束：

1. `✅ 一致` 条目优先写成 parity 或 behavior case；确实受环境限制时，需在自动化中显式 `Skip`。
2. `⚠️ 部分一致` 条目必须验证“差异边界”，而不是只测成功路径。
3. `🆕 gobox扩展` 条目必须验证参数是否真正进入执行路径，而不是仅测试 flag 可解析。
4. 新增或增强命令时必须先补齐 case 编号，再根据实际实现方式确认 Mode。
5. `behavior` case 必须证明“加参数前后行为发生可观察变化”；只检查 header、关键字或成功退出不算覆盖。
6. 组合参数至少要有一条 case 验证优先级或交互语义，避免某个参数在组合场景里被静默忽略。
7. 对 `structured` 和 `behavior` case，`Core Assertion` 必须优先写结果集、字段语义、排序/过滤/副作用，不应退化成泛化的关键词匹配描述。
8. 对环境敏感命令，`Core Assertion` 应优先描述受控夹具、目标记录或方向性约束，不默认要求全量输出完全相等。

---

## Shell 辅助命令

### alias

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| ALIAS-001 | default alias script | contract | gobox-only | registered command set | 输出 `gobox_alias_type=bash` 且为每个已注册子命令生成 `alias name='gobox name'`，同时不为 `alias` 自身生成递归 alias |
| ALIAS-002 | `-u` | contract | gobox-only | registered command set | 输出与已注册子命令集合一致的 `unalias` 脚本，并在结尾清理 `gobox_alias_type` |
| ALIAS-003 | `-h` | contract | gobox-only | none | 帮助输出包含用途说明和 `gobox alias [-u]` 用法 |

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
| DU-003 | `-a` | structured | `du -a` | file tree | 文件与目录项均输出 |
| DU-004 | `-c` | structured | `du -c` | multiple paths | 输出 total 汇总行 |
| DU-005 | `-d`, `--max-depth` | structured | `du -d` | nested tree | 最大深度限制生效 |
| DU-006 | `--exclude` | structured | `du --exclude` | mixed file names | 不含 `/` 的模式按 basename 匹配（任意深度），含 `/` 的模式按相对 root 路径匹配；非法模式报错 |
| DU-007 | `-x` | contract | `du -x` | local tree | 参数可解析并保持单文件系统遍历语义 |
| DU-008 | `--apparent-size` | structured | `du --apparent-size` | sparse/small files | 使用表观大小统计 |

### df

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| DF-001 | default filesystem usage | structured | `df` | local filesystems | 关键挂载点、容量字段和退出码语义一致，列宽按内容动态计算 |
| DF-002 | `-h` | structured | `df -h` | local filesystems | 人类可读容量字段存在且单位语义一致，自适应精度（≥10 无小数、<10 一位小数）、`Avail` 表头 |
| DF-003 | `-T` | structured | `df -T` | local filesystems | 文件系统类型列存在且关键挂载点一致 |
| DF-004 | `-i` | structured | `df -i` | local filesystems | inode 字段存在且关键挂载点一致；零 inode 伪文件系统默认隐藏（需 `-a`） |
| DF-005 | `PATH...` | structured | `df PATH...` | temp dir path | 仅输出指定路径所在文件系统 |
| DF-006 | `-H` | structured | `df -H` | controlled statfs fixture | SI 单位格式生效，千位前缀为小写 `k` |
| DF-007 | `-a` | structured | `df -a` | duplicate/zero mount fixture | all 模式包含默认隐藏项 |
| DF-008 | `-l` | structured | `df -l` | local/remote mount fixture | 仅保留本地文件系统 |
| DF-009 | `-t TYPE` | structured | `df -t` | mixed fs type fixture | 类型包含过滤生效 |
| DF-010 | `-x TYPE` | structured | `df -x` | mixed fs type fixture | 类型排除过滤生效 |
| DF-011 | `--total` | structured | `df --total` | controlled statfs fixture | total 汇总行生效 |
| DF-012 | `-P` | structured | `df -P` | controlled statfs fixture | POSIX 表头（含 `Capacity` 百分比列名）和单行格式生效 |

### readpath

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| READPATH-001 | default canonical path | exact | `realpath` | temp dir + relative path | 输出规范化绝对路径一致 |
| READPATH-002 | `-f, --canonicalize` | exact | `readlink -f` | symlink chain | 符号链接解析结果一致 |
| READPATH-003 | `-e, --canonicalize-existing` | exact | `realpath -e` | missing path component | 不存在路径的错误与退出码一致 |
| READPATH-004 | `-m, --canonicalize-missing` | exact | `realpath -m` | missing path component | 允许不存在组件时输出一致 |
| READPATH-005 | `-l, --readlink` | exact | `readlink` | symlink file | 输出 symlink 目标一致 |
| READPATH-006 | `-n, --no-newline` | exact | `readlink -n` | symlink file | 输出末尾换行行为一致 |
| READPATH-007 | `-q, --quiet` | behavior | `realpath -q` | missing path | 错误输出抑制与退出码一致 |
| READPATH-008 | `-z, --zero` | exact | `realpath -z` / `readlink -z` | multiple paths | NUL 分隔输出一致 |

### stat

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| STAT-001 | default file metadata | exact | `stat` | temp file | 默认多行输出与原生逐行一致（`Birth:` 行除外） |
| STAT-002 | `-L, --dereference` | structured | `stat -L` | symlink file | 显示目标文件而非 symlink 本身，Access 模式字段一致 |
| STAT-003 | `-f, --file-system` | structured | `stat -f` | temp dir | 文件系统字段语义一致 |
| STAT-004 | `-c, --format` | exact | `stat -c` | temp file | 指定格式输出完全一致，覆盖 `%f/%u/%g/%U/%G/%A/%i/%h/%d/%D/%o/%b/%X/%Y/%Z/%x/%z` 等常用指令 |
| STAT-005 | `-t, --terse` | structured | `stat -t` | temp file | 简洁字段数量与关键字段一致 |

### truncate

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| TRUNCATE-001 | `-s SIZE` | behavior | `truncate -s` | temp file | 文件最终大小一致 |
| TRUNCATE-002 | `-c, --no-create` | behavior | `truncate -c` | missing file | 不创建文件且退出行为一致 |
| TRUNCATE-003 | `-r RFILE` | behavior | `truncate -r` | reference file + target file | 目标大小等于参考文件 |
| TRUNCATE-004 | size suffix `K/M/G` | behavior | `truncate -s K/M/G` | temp file | 单位后缀换算后的大小一致 |
| TRUNCATE-005 | relative `+SIZE/-SIZE` | behavior | `truncate -s +SIZE/-SIZE` | temp file | 相对扩展/收缩后的大小一致 |

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
| SORT-012 | `-tCHAR` 粘连写法 / `--field-separator=CHAR` | exact | `sort -t`/`--field-separator` | 分隔列文本 | 粘连短选项和长选项形式与 `-t CHAR` 结果一致 |

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

### seq

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| SEQ-001 | `LAST` | exact | `seq LAST` | integer operand | 默认从 `1` 递增到 `LAST`，输出与退出码一致 |
| SEQ-002 | `FIRST LAST` | exact | `seq FIRST LAST` | integer operands | 默认步长 `1` 的双参数区间输出一致，含负数操作数 |
| SEQ-003 | `FIRST INC LAST` | exact | `seq FIRST INC LAST` | integer/float operands | 显式步长路径生效，递增/递减、浮点步长、负数操作数输出一致；小数位数按最长操作数统一，不出现浮点累积误差（如 `0.1+0.1+0.1` 不应显示为 `0.30000000000000004`） |
| SEQ-004 | `-f FORMAT, --format=FORMAT` | behavior | `seq -f` | integer/float operands | 指定格式必须相对默认输出改变每项文本表示，且数值序列不变 |
| SEQ-005 | `-s SEP, --separator=SEP` | exact | `seq -s` | integer operands | 分隔符替换换行的输出一致 |
| SEQ-006 | `-w, --equal-width` | exact | `seq -w` | mixed-width integer/float/negative operands | 输出项按最大位宽补零对齐（仅补整数部分），且与原生逐字节一致 |
| SEQ-007 | `-h, --help` | contract | `seq --help` | none | 帮助输出成功并包含 `gobox seq` 用法与主要参数说明 |

### rand

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| RAND-001 | default hex output | contract | gobox-only | none | 默认输出 32 字节随机数据的 hex 文本，长度与编码语义稳定 |
| RAND-002 | `NUM` positional operand | contract | gobox-only | byte-count operand | 位置参数会改变生成字节数，输出编码长度与请求字节数匹配 |
| RAND-003 | `-n NUM` | contract | `openssl rand -hex NUM` | byte-count operand | 显式字节数参数进入执行路径，输出长度与请求字节数匹配 |
| RAND-004 | `-hex` | contract | `openssl rand -hex` | none | hex 模式输出仅包含 lowercase hex 字符并保持换行契约 |
| RAND-005 | `-base64` | behavior | `openssl rand -base64` | none | base64 模式必须相对默认 hex 输出改变编码字母表和长度语义 |
| RAND-006 | `-out FILE` | contract | `openssl rand -out FILE` | temp output file | 生成结果写入指定文件，stdout 不混入随机正文 |
| RAND-007 | `-h, --help` | contract | `openssl rand -help` | none | 帮助输出成功并包含 `gobox rand` 用法与主要参数说明 |
| RAND-008 | `-NUM` 简写 | contract | gobox-only | byte-count operand | 裸数字简写与 `-n NUM` 效果一致，输出编码长度与请求字节数匹配 |

### hex

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| HEX-001 | `--dump -C` | structured | `hexdump -C` | binary fixture | canonical 十六进制输出字段语义一致 |
| HEX-009 | `--dump -C` 默认折叠重复行 | exact | `hexdump -C` | repeated 16-byte rows fixture | 默认输出将连续重复行折叠为 `*`，与 native 折叠位置一致 |
| HEX-002 | `--dump -n LEN` | structured | `hexdump -n` | binary fixture | 读取长度限制一致 |
| HEX-003 | `--dump -s OFFSET` | structured | `hexdump -s` | binary fixture | 起始偏移一致 |
| HEX-004 | `--dump -v` | structured | `hexdump -v` | repeated binary fixture | 重复行不折叠语义一致 |
| HEX-005 | `--dump -e FORMAT` | behavior | `hexdump -e` | binary fixture | 常用格式子集输出语义一致 |
| HEX-006 | `--encode` | contract | gobox-only | binary fixture + stdin | 输出连续 lowercase hex 且可被 decode 还原 |
| HEX-007 | `--decode` | contract | gobox-only | hex text fixture + stdin | 解码后字节与原始输入一致 |
| HEX-008 | `--decode -o FILE` | contract | gobox-only | hex text fixture | 解码结果写入指定文件且 stdout 不混入二进制 |

### base64

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| BASE64-001 | default encode | exact | `base64` | binary fixture + stdin | 编码输出与退出码一致 |
| BASE64-002 | `-d, --decode` | exact | `base64 -d` | base64 fixture + stdin | 解码字节与退出码一致 |
| BASE64-003 | `-w COLS, --wrap COLS` | exact | `base64 -w` | binary fixture | 换行宽度一致，`0` 时不换行 |
| BASE64-004 | `-i, --ignore-garbage` | exact | `base64 -i` | dirty base64 fixture | decode 时忽略非 base64 字符一致 |
| BASE64-005 | `-o FILE` | contract | gobox-only | binary/base64 fixture | 输出写入指定文件且 stdout 为空 |

### strings

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| STRINGS-001 | default printable strings | exact | `strings` | binary fixture | 可打印字符串集合与顺序一致 |
| STRINGS-002 | `-n LEN` | exact | `strings -n` | binary fixture | 最短长度过滤一致 |
| STRINGS-003 | `-f` | exact | `strings -f` | multiple files | 文件名前缀输出一致 |
| STRINGS-004 | `-t o|d|x` | exact | `strings -t` | binary fixture | 偏移进制与位置一致 |
| STRINGS-005 | `-a` | exact | `strings -a` | binary fixture | 全文件扫描行为一致 |

### diff

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| DIFF-001 | default compare | exact | `diff` | changed/added/deleted text files | normal diff 范围与退出码一致 |
| DIFF-002 | `-u`, `--unified` | exact | `diff -u` | changed text files | unified diff 头与 hunk 内容一致 |
| DIFF-003 | `-q`, `--brief` | exact | `diff -q` | equal/different files | 简要输出与退出码一致 |
| DIFF-004 | `-r`, `--recursive` | behavior | `diff -r` | directory trees | 递归遍历稳定有序 |
| DIFF-005 | `-N`, `--new-file` | behavior | `diff -N` | missing file | 缺失文件按空文件比较 |
| DIFF-006 | `--strip-trailing-cr` | exact | `diff --strip-trailing-cr` | CRLF/LF files | 行尾 CR 被忽略 |
| DIFF-007 | stdin side `-` | behavior | `diff FILE -` | file + stdin | stdin 输入比较结果一致 |
| DIFF-008 | binary files | behavior | `diff` | binary files | 仅报告二进制差异，不转储内容 |
| DIFF-009 | equal files | exact | `diff` | equal files | 无输出且退出码为 0 |

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
| CURL-006 | `-I, --head` | behavior | `curl -I` | local HTTP server | 必须只输出响应头而不带 body |
| CURL-007 | `-w, --write-out` | behavior | `curl -w` | local HTTP server | 关键格式占位符一致 |
| CURL-008 | `-m, --max-time` | behavior | `curl -m` | slow server | 超时前不得输出成功 body |
| CURL-009 | `-X, --request` | behavior | `curl -X` | local HTTP server | 显式请求方法必须传递到服务端 |
| CURL-010 | `-H, --header` | behavior | `curl -H` | local HTTP server | 自定义头必须传递到服务端 |
| CURL-011 | `-d, --data` | behavior | `curl -d` | local HTTP server | POST body 一致 |
| CURL-012 | `-k, --insecure` | behavior | `curl -k` | local HTTPS server with self-signed cert | 忽略证书错误后请求可成功 |
| CURL-013 | `--connect-timeout` | behavior | `curl --connect-timeout` | unroutable target | 连接超时路径不得产出成功 body |
| CURL-014 | `--resolve` | behavior | `curl --resolve` | local HTTP server + fake host | 强制解析一致 |
| CURL-015 | `-f, --fail` | behavior | `curl -f` | 404/500 server | 失败退出语义一致 |
| CURL-016 | `-i, --include` | behavior | `curl -i` | local HTTP server | 响应头输出一致 |
| CURL-017 | `-T, --upload-file` | behavior | `curl -T` | local upload server | PUT 上传一致 |
| CURL-018 | `-F, --form` | behavior | `curl -F` | local multipart server | multipart 上传一致 |
| CURL-019 | `-c, --concurrent=N` | behavior | gobox-only | local bench server | `-c` 必须相对默认并发基线改变 bench 汇总输出 |
| CURL-020 | `-n, --requests=N` | behavior | gobox-only | local bench server | `-n` 必须相对基线请求数改变 bench 汇总输出 |
| CURL-021 | `--warmup=N` | behavior | gobox-only | local bench server | `--warmup` 必须相对 no-warmup 基线改变 bench 输出并显示预热阶段 |
| CURL-022 | `-t, --timeout=SEC` | behavior | `curl -m` | local slow HTTP server | 请求超时语义与 `curl -m` 对齐 |

### nc/netcat

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| NC-001 | `-l, --listen` | behavior | `nc -l` | local socket | 监听模式一致 |
| NC-002 | `-z, --zero` | behavior | `nc -z` | local socket | 零 I/O 探测成功时应保持 quiet 且不走数据路径 |
| NC-003 | `-u, --udp` | behavior | `nc -u` | local UDP socket | UDP 零 I/O 探测成功时应保持 quiet |
| NC-004 | `-w, --wait=SEC` | behavior | `nc -w` | timeout target | 超时/失败路径必须保持非成功语义 |
| NC-005 | `-v, --verbose` | behavior | `nc -v` | local socket | `-v` 必须相对 plain `-z` 增加诊断输出 |
| NC-006 | `-n, --numeric-only` | behavior | `nc -n` | host/ip target | 跳过 DNS 解析一致 |
| NC-007 | `-4` | behavior | `nc -4` | IPv4 server | IPv4 强制一致 |
| NC-008 | `-6` | behavior | `nc -6` | IPv6 server | IPv6 强制一致 |
| NC-009 | `--bench` | contract | gobox-only | local bench pair | benchmark 模式稳定 |
| NC-010 | `-c, --concurrent=N` | behavior | gobox-only | local bench pair | 并发连接数必须相对默认 bench 基线改变输出 |
| NC-011 | `-n, --requests=N` | behavior | gobox-only | local bench pair | 请求数必须相对默认 bench 基线改变输出 |
| NC-012 | `-s, --size=N` | behavior | gobox-only | local bench pair | 数据块大小必须相对默认 bench 基线改变输出 |
| NC-013 | `-t, --time=SEC` | behavior | gobox-only | local bench pair | 持续时间参数必须相对默认 bench 基线改变输出 |
| NC-014 | `-i, --interval=SEC` | behavior | gobox-only | local bench pair | 报告间隔参数必须相对默认 bench 基线改变输出 |

### netstat

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| NETSTAT-001 | `--port int` | structured | `netstat` | local listener | 端口过滤命中预期监听项 |
| NETSTAT-002 | `--sort string` | structured | gobox-only | local listeners | 排序字段语义正确 |
| NETSTAT-003 | `--state string` | structured | `netstat` | local listener + state filter | 状态列表过滤正确 |
| NETSTAT-004 | `-l, --listening` | structured | `netstat -l` | local listener | 仅输出监听 socket |
| NETSTAT-005 | `-n, --numeric` | contract | gobox-only | local sockets | gobox 当前默认已是数字地址/端口，`-n` 应保持与默认输出一致 |
| NETSTAT-006 | `-a, --all` | contract | gobox-only | local sockets | gobox 当前默认 socket 选择已覆盖 `-a` 兼容语义，输出应与默认一致 |
| NETSTAT-007 | `-t, --tcp` | structured | `netstat -t` | local TCP listener | 仅输出 TCP socket |
| NETSTAT-008 | `-u, --udp` | structured | `netstat -u` | local UDP socket | 仅输出 UDP socket |
| NETSTAT-009 | `-x, --unix` | structured | `netstat -x` | local Unix socket | 仅输出 Unix socket |
| NETSTAT-010 | `-p, --programs` | behavior | `netstat -p` | local sockets | `-p` 必须在保留目标 socket 的同时为结果行增加 PID/Program 信息 |
| NETSTAT-011 | `-4` | structured | `netstat -4` | local IPv4 socket | 仅输出 IPv4 socket |
| NETSTAT-012 | `-6` | structured | `netstat -6` | local IPv6 socket | 仅输出 IPv6 socket |
| NETSTAT-013 | `-e, --extend` | behavior | `netstat -e` | local sockets | `-e` 必须在保留目标 socket 的同时增加扩展列 |
| NETSTAT-014 | `-o, --timers` | behavior | `netstat -o` | local sockets | `-o` 必须在保留目标 socket 的同时增加 Timer 列 |
| NETSTAT-015 | `-W, --wide` | contract | gobox-only | local sockets | gobox 当前默认不截断地址，`-W` 应保持与 `-n -l` 基线一致 |
| NETSTAT-016 | combined short flags, e.g. `-tnlp` | behavior | `netstat -tnlp` | local TCP listener | 合并短参数必须相对 `-t -l` 基线改变输出，并命中目标 listener |
| NETSTAT-017 | `-r` | structured | `netstat -r` | local route table | 路由表必须包含接口列与默认路由语义，默认路由目的地为 `default`，表头为 `MSS Window irtt` |
| NETSTAT-018 | `-i` | structured | `netstat -i` | local interfaces | 接口表必须包含环回接口、收发统计列、`RX-OVR`/`TX-OVR`，按接口名排序，`Flg` 用字母代码 |
| NETSTAT-019 | `-s` | behavior | `netstat -s` | local protocol stats | 裸 `-s` 必须包含比 `-s -t` 更完整的多协议统计视图 |
| NETSTAT-020 | `-c` | contract | `netstat -c` | bounded command execution | continuous 模式可进入刷新路径 |
| NETSTAT-021 | short/long flag equivalence for socket table flags | structured | gobox-only | local TCP listener | `-t/-l/-p/-e/-o/-n/-W` 与 `--tcp/--listening/--programs/--extend/--timers/--numeric/--wide` 输出一致 |
| NETSTAT-022 | short/long flag equivalence for view flags | structured | gobox-only | local route table | `-r` 与 `--route` 输出一致 |
| NETSTAT-023 | `--help` grouped help output | contract | gobox-only | none | 帮助输出按功能分组，短长参数合并为单行展示 |
| NETSTAT-024 | `-s` with protocol filters, e.g. `-s -t` | behavior | `netstat -s -t` | local protocol stats | 组合后只保留目标协议统计，不能退化成裸 `-s` |

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
| IFSTAT-001 | `-A` | contract | gobox-only | local interfaces | 显示全部接口集合不弱于默认模式 |
| IFSTAT-002 | `-a` | behavior | gobox-only | local interfaces | `-a` 必须相对默认模式改变输出并切换到绝对值视图，表头列名同步改为非速率的 `rxpkts`/`txpkts`/`rxKB`/`txKB` |
| IFSTAT-003 | `-d` | behavior | gobox-only | local interfaces | `-d` 必须相对默认模式增加 drop 列 |
| IFSTAT-004 | `-e` | behavior | gobox-only | local interfaces | `-e` 必须相对默认模式增加 error 列 |
| IFSTAT-005 | `-i string` | contract | gobox-only | selected iface | 仅输出指定接口 |
| IFSTAT-006 | `-n int` | contract | gobox-only | local interfaces | 样本数受控并按次数退出 |
| IFSTAT-007 | `-p int` | contract | gobox-only | local interfaces | 采样间隔参数生效 |

### ip

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| IP-001 | `addr` / `a` | structured | `ip addr` | local interfaces | 接口名、地址、`brd`/`scope`/`valid_lft` 和基于 `operstate` 的状态不弱于 native |
| IP-002 | `-o addr` | behavior | `ip -o addr` | local interfaces | `-o` 必须相对多行 `addr` 视图切换为单行 scoped records（含 `\` 续行标记） |
| IP-003 | `link` / `l` | structured | `ip link` | local interfaces | 接口名、MTU、MAC/`brd`、flags 名称/顺序与 native 语义一致 |
| IP-004 | `-s link` | behavior | `ip -s link` | local interfaces | `-s` 必须相对基础 `link` 视图增加 RX/TX 统计字段（含 missed/mcast/carrier/collsns） |
| IP-005 | `route` / `r` | structured | `ip route` | local route table | IPv4 路由字段可解析，含 CIDR 前缀长度、`proto`/`scope`/`src`/`metric`/`linkdown` |
| IP-006 | `neigh` / `n` | structured | `ip neigh` | local ARP/neigh table | 邻居 IP、设备和状态字段可解析；状态基于 ARP flags 映射为 REACHABLE/PERMANENT/INCOMPLETE |

### np/netping

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| NP-001 | `-I string` | contract | gobox-only | local target | 指定网卡参数可进入拨号器 |
| NP-002 | `-W int` | behavior | `ping -W` | timeout target | 超时语义稳定 |
| NP-003 | `--arp` | behavior | `arping` | default gateway | ARP 模式可执行并收到响应 |
| NP-004 | `-c int` | behavior | `ping -c` | local target | 次数受控 |
| NP-005 | `--flood` | contract | `ping -f` | local target | flood 模式可执行 |
| NP-006 | `-i int` | contract | gobox-only | local target | 微秒间隔参数生效 |
| NP-007 | `--icmp` | behavior | `ping` | local target | ICMP 模式可执行 |
| NP-008 | `-l int` | contract | gobox-only | local TCP target | 长连接模式生效 |
| NP-009 | `-p int` | behavior | `nc -p`/TCP target | local TCP/UDP target | 目标端口生效 |
| NP-010 | `-q` | behavior | `ping -q` | local target | `-q` 必须相对默认模式收敛为 summary-only 输出 |
| NP-011 | `-s int` | contract | gobox-only | local target | 源端口绑定生效 |
| NP-012 | `--scan` | behavior | gobox-only | local open/closed ports | 扫描结果必须报告目标端口状态和汇总计数 |
| NP-013 | `--tcp` | behavior | `nc` | local TCP server | TCP 模式可执行 |
| NP-014 | `--udp` | behavior | `nc -u` | local UDP server | UDP 模式可执行 |
| NP-015 | `-v` | behavior | `ping -v` | local target | `-v` 必须相对 quiet 模式增加逐次诊断输出 |
| NP-016 | `-w int` | behavior | gobox-only | local target | `-w` 必须相对单 worker 基线改变执行/报告路径 |

---

## 进程命令

### ps

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| PS-001 | `-e` | contract | `ps -e` | current process | 显式 all-process 选择生效，且默认 `CMD` 列保持与原生 `ps -e` 一样的 `comm` 风格展示 |
| PS-002 | `-f` | structured | `ps -f` | current process | 切换到 full-format 并包含 `UID`/`PPID`/`STIME`/`TTY`/`TIME`/`CMD` 等核心列，`CMD` 保持单行 |
| PS-003 | `-i int` | contract | gobox-only | current process | 采样间隔参数可执行 |
| PS-004 | `--maxcmd N` | contract | gobox-only | long cmdline process | 显式命令长度上限生效 |
| PS-005 | `-n int` | contract | gobox-only | current processes | 限制输出行数 |
| PS-006 | `--full string` | structured | `pgrep -f` | current process | 完整命令行正则匹配符合 `pgrep -f`，输出的 `CMD` 也应保留完整命令行便于核对 |
| PS-007 | `-r` | structured | `ps -r` | current processes | 排序方向反转 |
| PS-008 | `--sort FIELD` | contract | gobox-only | current processes | 排序字段生效 |
| PS-009 | `-ww` | contract | `ps -ww` | long cmdline process | `ps` 默认宽度策略可被 `-ww` 关闭，长命令保持完整单行 |
| PS-010 | `-o FIELD1,FIELD2` | structured | `ps -o` | current process | 自定义列输出正确；高频字段映射稳定，不支持字段明确报错 |
| PS-011 | `--comm string` | structured | `pgrep -x` | current process | 进程名精确匹配符合 `pgrep -x` |
| PS-012 | `-A` | structured | `ps -A` | current process | all-process alias 可看到当前进程 |
| PS-013 | `-F` | behavior | `ps -F` | current process | `-F` 必须相对基础 `-p PID` 增加 full-format 列并保留目标 PID |
| PS-014 | `-u USER` | structured | `ps -u` | current user | 用户过滤命中当前进程集合 |
| PS-015 | `-p PID` | structured | `ps -p` | current process | PID 过滤只保留目标进程 |
| PS-016 | `-C NAME` | structured | `ps -C` | current process name | 命令名过滤命中目标进程 |
| PS-017 | `--sort -FIELD` | structured | `ps --sort` | current processes | GNU 风格降序排序参数生效；不支持字段明确报错 |
| PS-018 | BSD `aux` semantics | behavior | `ps aux` | current process | BSD 风格 `a/x/u` 组合语义与 user-oriented 列布局保持常见 native 预期 |
| PS-019 | `--long` | structured | `ps -l` | current process | long 格式输出必须包含 `S/UID/PID/PPID/TTY/TIME/CMD`，状态列用单字母 `S` 而非 `STAT` |
| PS-020 | `--hide-idle` | contract | gobox-only | idle process | 过滤掉采样 CPU 为 0 的进程 |

### top

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| TOP-001 | `-d int` | behavior | `top -d` | single iteration | 刷新间隔参数可用且单轮模式下不会异常退出 |
| TOP-002 | `-n int` | behavior | `top -n` | single iteration | 指定迭代次数后退出 |
| TOP-003 | `-r` | structured | `top -r` | single iteration | 排序方向切换且关键排序方向符合预期 |
| TOP-004 | `--sort string` | contract | gobox-only | single iteration | 排序字段生效 |
| TOP-005 | `-b` | contract | `top -b` | single iteration | batch 模式不输出清屏控制符；`Tasks:`/`MiB Mem`/`MiB Swap` 数值列宽、`TIME+` 分:秒.厘秒格式、状态列 `S` 单字母表头与原生对齐 |
| TOP-006 | `-p PID` | structured | `top -p` | current process | PID 过滤命中当前进程 |
| TOP-007 | `-u USER` | structured | `top -u` | current user | 用户过滤可执行并输出进程表 |
| TOP-008 | `-H` | contract | `top -H` | single iteration | Linux 下显示线程视图，`PID` 列输出 TID，`-p PID` 仍按所属进程过滤 |
| TOP-009 | `-i` | contract | `top -i` | single iteration | idle 过滤参数被接受 |
| TOP-010 | `-c` | contract | `top -c` | single iteration | 完整命令行模式被接受 |
| TOP-011 | `-o FIELD` | contract | `top -o` | single iteration | 排序字段参数被接受 |

### free

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| FREE-001 | default memory summary | structured | `free` | local Linux host | Mem/Swap 行必须包含可解析的核心列集合，表头须含 `shared` 列（6 列与原生一致） |
| FREE-002 | `-h` | behavior | `free -h` | local Linux host | `-h` 必须相对默认输出切换为人类可读单位 |
| FREE-003 | `-m` | behavior | `free -m` | local Linux host | `-m` 必须相对默认输出切换为 MiB 数值视图 |
| FREE-004 | `-g` | behavior | `free -g` | local Linux host | `-g` 必须相对默认输出切换为 GiB 数值视图 |
| FREE-005 | `-s SEC -c COUNT` | behavior | `free -s -c` | local Linux host | 按指定次数采样并退出 |
| FREE-006 | `-b` | behavior | `free -b` | local Linux host | `-b` 必须相对默认输出切换为字节数值视图 |
| FREE-007 | `-k` | contract | `free -k` | local Linux host | `-k`（默认单位）显式指定时被接受，输出与默认一致 |

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

### kill

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| KILL-001 | PID default signal | behavior | `kill` | controlled child process | 默认 `TERM` 信号使目标进程退出 |
| KILL-002 | `-l, --list` | exact | `kill -l` | none | 完整 64 个信号（含实时信号）的编号表格与原生 `kill -l` 逐字节一致；名字与编号双向互查覆盖标准信号和实时信号（RTMIN/RTMAX） |
| KILL-003 | `-s SIGNAL` | behavior | `kill -s` | controlled child process | 指定信号发送语义一致 |
| KILL-004 | `-SIGNAL` | behavior | `kill -SIGNAL` | controlled child process | 短信号格式语义一致 |
| KILL-005 | `-f PATTERN` | behavior | `pkill -f` | controlled named process | 完整命令行匹配集合一致 |
| KILL-006 | `-x PATTERN` | behavior | `pkill -x` | controlled named process | 精确进程名匹配集合一致 |
| KILL-007 | `-P PPID` | behavior | `pkill -P` | parent + child process | 父进程过滤集合一致 |
| KILL-008 | `-n` | behavior | `pkill -n` | multiple controlled processes | 最新进程选择一致 |
| KILL-009 | `-o` | behavior | `pkill -o` | multiple controlled processes | 最早进程选择一致 |
| KILL-010 | `--dry-run` | contract | gobox-only | controlled named process | 输出将匹配进程且不发送信号 |

### lsof

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| LSOF-001 | default open files | structured | `lsof` | current process | 输出包含 `USER`/`TYPE`/`DEVICE`/`SIZE/OFF`/`NODE` 表头和当前进程可见打开文件 |
| LSOF-002 | `-p PID` | structured | `lsof -p` | current process | 结果行只能属于指定 PID |
| LSOF-003 | `-c NAME` | structured | `lsof -c` | controlled named process | 命令名过滤只保留目标进程集合 |
| LSOF-004 | `-i` | behavior | `lsof -i` | local socket | `-i` 必须相对默认 `lsof` 缩小为已解析 TCP/UDP 的网络 socket 结果集，排除无法解析的 socket（如 unix domain） |
| LSOF-005 | `-iTCP` | structured | `lsof -iTCP` | local TCP socket | TCP 协议过滤集合一致 |
| LSOF-006 | `-iUDP` | structured | `lsof -iUDP` | local UDP socket | UDP 协议过滤集合一致 |
| LSOF-007 | `-i :PORT` | behavior | `lsof -i :PORT` | local listener | 端口过滤必须相对 bare `-i` 缩小结果集并保留目标 socket |
| LSOF-008 | `-n` | contract | gobox-only | local socket | gobox 当前默认已是数字主机表示，`-n` 应保持与 `-i` 基线一致 |
| LSOF-009 | `-P` | contract | gobox-only | local socket | gobox 当前默认已是数字端口表示，`-P` 应保持与 `-i :PORT` 基线一致 |
| LSOF-010 | `-t` | exact | `lsof -t` | controlled process | 仅输出 PID 列表 |
| LSOF-011 | `FILE...` | structured | `lsof FILE...` | opened temp file | 能定位打开指定文件的进程 |

### watch

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| WATCH-001 | `COMMAND...` | behavior | `watch` | short-lived command + timeout harness | 命令会被周期性执行，默认使用覆盖刷新而不是滚动追加 |
| WATCH-002 | `-n SEC` | behavior | `watch -n` | short-lived command + timeout harness | 执行间隔参数影响刷新节奏 |
| WATCH-003 | `-t` | behavior | `watch -t` | short-lived command + timeout harness | 标题行隐藏行为一致 |
| WATCH-004 | `--append` | contract | gobox-only | short-lived command + timeout harness | 显式切换到滚动追加输出，且不发送清屏控制序列 |

### timeout

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| TIMEOUT-001 | `DURATION COMMAND...` | behavior | `timeout` | sleep command | 超时后终止命令且退出码语义一致 |
| TIMEOUT-002 | `-s SIGNAL` | behavior | `timeout -s` | signal-trapping command | 超时信号类型一致 |
| TIMEOUT-003 | `-k DURATION` | behavior | `timeout -k` | signal-ignoring command | grace period 后强制结束 |
| TIMEOUT-004 | `--preserve-status` | behavior | `timeout --preserve-status` | command with known status | 保留子命令退出状态语义一致 |
| TIMEOUT-005 | duration suffix | behavior | `timeout 1s/1m/1h` | sleep command | 常用时间后缀解析一致 |

---

## 磁盘命令

### iostat

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| IOSTAT-001 | `-i sec` | structured | `iostat 1 1` | local Linux host | 间隔采样输出结构与 native 一致 |
| IOSTAT-002 | `-n count` | structured | `iostat` | local Linux host | 单次采样输出结构与 native 一致 |
| IOSTAT-003 | `-H` | behavior | gobox-only | local Linux host | `-H` 必须相对默认输出切换为人类可读吞吐单位 |
| IOSTAT-004 | `-z` | structured | `iostat -z 1 1` | local Linux host | 零活动设备过滤后的结构与 native 一致 |
| IOSTAT-005 | `--cgroup` | behavior | gobox-only | local Linux host with cgroup io stats | 可切换到基于 cgroup 的旧输出路径 |
| IOSTAT-006 | `interval [count]` | structured | `iostat 1 1` | local Linux host | 位置参数形式的采样间隔与次数可执行 |
| IOSTAT-007 | `--help` enriched help output | contract | gobox-only | none | 帮助输出包含位置参数、列说明和示例 |

### ioperf

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| IOPERF-001 | `--bs string` | behavior | `fio --bs` | temp file | 块大小参数进入执行并产出 I/O 结果 |
| IOPERF-002 | `--direct int` | behavior | `fio --direct` | temp file | direct 参数进入执行路径 |
| IOPERF-003 | `--filename string` | behavior | `fio --filename` | temp file | 指定文件路径可用并创建测试文件 |
| IOPERF-004 | `--fsync int` | behavior | `fio --fsync` | temp file | fsync 参数进入写入路径 |
| IOPERF-005 | `--group_reporting` | behavior | `fio --group_reporting` | temp file + multi job | 聚合输出生效 |
| IOPERF-006 | `--iodepth int` | behavior | `fio --iodepth` | temp file | 队列深度影响执行 |
| IOPERF-007 | `--write_hist_log string` | behavior | `fio --write_hist_log --log_hist_msec` | temp file | 延迟直方图日志输出出现 |
| IOPERF-008 | `--numjobs int` | behavior | `fio --numjobs` | temp file | 并发 job 参数生效 |
| IOPERF-009 | `--percentile_list string` | behavior | `fio --percentile_list` | temp file | 指定百分位输出出现 |
| IOPERF-010 | `--rate string` | behavior | `fio --rate` | temp file | 限速参数进入执行路径 |
| IOPERF-011 | `--runtime int` | behavior | `fio --runtime` | temp file | 运行时长参数生效 |
| IOPERF-012 | `--rw string` | behavior | `fio --rw` | temp file | I/O 模式切换生效 |
| IOPERF-013 | `--rwmixread int` | behavior | `fio --rwmixread` | temp file | 混合读比例进入执行路径 |
| IOPERF-014 | `--size string` | behavior | `fio --size` | temp file | 数据量参数生效 |
| IOPERF-015 | `--sync string` | behavior | `fio --sync` | temp file | sync 模式进入执行路径 |
| IOPERF-016 | `--time_based` | behavior | `fio --time_based` | temp file | 时间模式生效 |

### md5sum

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| MD5-001 | `-c, --check` | exact | `md5sum -c` | checksum file | 校验结果与退出码一致 |
| MD5-002 | `--tag` | exact | `md5sum --tag` | file | BSD 风格输出一致 |
| MD5-003 | `-q, --quiet` | exact | `md5sum --quiet` | checksum file | 安静模式一致 |
| MD5-004 | `-s, --status` | exact | `md5sum --status` | checksum file | 仅退出码一致 |
| MD5-005 | `-w, --warn` | exact | `md5sum --warn` | malformed checksum file | 警告行为一致 |

### sha256sum

| Case ID | Arg/Feature | Mode | Native Baseline | Fixture | Core Assertion |
|---|---|---|---|---|---|
| SHA256-001 | default checksum | exact | `sha256sum` | file + stdin | 校验和、文件名和退出码一致 |
| SHA256-002 | `-c, --check` | exact | `sha256sum -c` | checksum file | 校验结果与退出码一致 |
| SHA256-003 | `--tag` | exact | `sha256sum --tag` | file | BSD 风格输出一致 |
| SHA256-004 | `-q, --quiet` | exact | `sha256sum --quiet` | checksum file | 安静模式一致 |
| SHA256-005 | `-s, --status` | exact | `sha256sum --status` | checksum file | 仅退出码一致 |
| SHA256-006 | `-w, --warn` | exact | `sha256sum --warn` | malformed checksum file | 警告行为一致 |

---

## 实施优先级

### 第一批（高稳定、强 parity）
- `grep`, `sed`, `head`, `tail`, `sort`, `uniq`, `wc`, `md5sum`, `sha256sum`, `xargs`, `find`, `readpath`, `hex`, `base64`, `strings`, `diff`

### 第二批（结构化 parity）
- `ps`, `netstat`, `du`, `df`, `iostat`, `ifstat`, `ip`, `free`, `stat`, `lsof`

### 第三批（行为/contract）
- `curl`, `nc`, `nslookup/dig`, `np`, `tw`, `ioperf`, `top`, `truncate`, `kill`, `watch`, `timeout`

---

## 覆盖约束

1. 本文件中每个 `Case ID` 最终都需要映射到自动化测试代码。
2. 若因环境限制无法可靠执行 native baseline，测试代码中必须：
   - 明确标记为 `contract`
   - 或在运行时 `Skip` 并说明原因。
3. 后续若 `docs/CMD-SPECS.md` 增减条目，必须同步更新此文件。
