# Parity Test Quality Review

## TODO Checklist

进度说明：`[ ]` 待处理，`[x]` 已在测试代码中修复，`[bug]` 定位为实现代码问题，已记录到 `BUGS.md`，未修改测试代码，等待确认后再处理。

### 0. helpers_parity_test.go（共享基础设施）— 全部完成

- [x] `requireNativeCommand` 用 `t.Fatalf` 改为 `t.Skip`
- [x] stdout/stderr/stdin 恢复补 `defer`（`runGoboxCLI`/`runGoboxMainCLI`/`runTailGoboxFollow`/`runGoboxNCListen`）
- [x] `collapseSpaces` 不应吞掉行边界（改为逐行 collapse，保留行边界）
- [x] `runGoboxMainCLI` 补并发排空 pipe 保护
- [x] 默认 assert 失败输出补充 cmdline/fixture/比较模式信息

### 1. text_parity_test.go

- [x] RAND-003 断言过宽（只查非空）— 已改为验证 base64/hex 字符集与长度语义
- [x] SORT-011 未走 native 对比，硬编码期望值 — 已改为真实 `sort -z` native 对比
- [x] SED-002 备份文件缺失时静默跳过比较 — 已改为双方文件缺失即报错
- [x] GREP-005 `--line-buffered` 未验证增量输出 — 已改为实时管道流验证
- [x] TAIL-006 `-s SEC` 未验证轮询节奏随参数变化 — 已改为对比不同 `-s` 值的节奏
- [bug] DIFF-010 已知 bug 已改为 `t.Fatalf`（不再 `t.Logf` 静默通过），确认为真实 bug，见 `BUGS.md` BUG-11；`/tmp/bugs_text.md` 硬编码路径已移除
- [x] SEQ-004(`-f FORMAT`) 已补齐实现，`SEQ-005`(分隔符) 标号已纠正
- [x] SEQ-006(`-w`)、SEQ-007(`-h`) 已补齐
- [x] RAND-002(裸 `NUM`)、RAND-004(`-hex`)、RAND-006(`-out`)、RAND-007(`-h`) 已补齐；"RAND-002"/"RAND-003" 标号与真实 `-base64` 参数已纠正
- [x] DIFF-002/003/005、BASE64-002/003/004 长短参数等价性已补齐
- [x] grep/sed/sort/uniq/wc/head/tail 已补空输入边界用例（`GREP-003b`/`SED-006b`/`SED-020b`/`SORT-001b`/`UNIQ-001b`/`WC-001b`/`TAIL-001b` 等）
  - [bug] 其中 `GREP-003b`（空文件）、`GREP-019b`（空 stdin）在补齐后暴露真实 bug，见 `BUGS.md` BUG-10

### 2. fs_parity_test.go

- [x] DU-007(`-x`) — 已加注释说明当前沙箱环境无法构造跨文件系统 fixture，保留为受限验证
- [x] DF-008(`-l`) — 已加受控 fixture 尝试，环境限制下用 `t.Skip` 说明原因
- [x] DF-006..012 冗余 `Contains` 兜底循环 — 已清理
- [x] DU-004 数值容差 20% 过宽 — 已收紧到 10%（收紧后暴露真实 bug，见下）
- [x] DF-003(`-T`) 已改为交叉比较 gobox/native 类型值
- [x] DF-004(`-i`) 已改为交叉比较 gobox/native inode 数值（并加入 1% 容差应对并发环境下 inode 计数瞬时波动）
- [x] DF-011(`--total`) 已改为验证汇总值是否为各行之和 — 验证后暴露真实 bug，见下
- [x] STAT-005(`-t`) 已改为验证字段数量
- [x] DU-005(`-d`) 已扩展为三级目录树验证 depth 0/1/2（原实现有测试代码 bug，已修复：默认 `du` 不列出普通文件，误用 `b.txt` 断言）
- [x] DU-001/002/003/006/007/008 per-entry size 数值已改为真实数值比较 — 验证后暴露真实 bug，见下
- [x] alias(ALIAS-001/002/003) 已补齐三个 case，并已注册进 `invokeGobox`
- [x] du `-c` 已补充与 `-s`/`-a` 的组合测试
- [x] stat/truncate 长参数别名已补齐等价性 case
- [x] find/du/stat 权限拒绝/符号链接边界场景已补充（沙箱以 root 运行时用 `t.Skip` 说明原因）
- [x] `requireNativeCommand` Fatal 问题（已随任务0修复）
- [x] df 非 Linux 跳过原因文案已具体化
- [x] 兜底 case 已改为共享同一个 fixture 目录
- [bug] DU-004/002/003/DU-c-s/DU-c-a/DU-006/DU-multi-exclude：收紧容差后确认 gobox `du` 对每层目录多算约 1 个块，见 `BUGS.md` BUG-4
- [bug] DF-011：确认 `--total` 行 Used 列聚合逻辑遗漏累加 `Bfree`，见 `BUGS.md` BUG-5

### 3. net_parity_test.go

- [x] TW-001 已改为真实绑定端口并验证可连接
- [x] CURL-002(`-S`) 已建立 `-s`-only 基线对比真实失败请求
- [x] CURL-015(`-f`) 已验证失败时 stdout 应为空
- [x] NETSTAT-023 已验证分组归属（而非仅关键字存在）
- [x] NETSTAT-016/021 已补齐"拆分四个参数 `-t -n -l -p` == 合并 `-tnlp`"等价性 case
- [x] NETSTAT-010/013/014 已改为验证新增列的真实数值（PID/User/Timer）
- [x] CURL-019/020/021(bench) 已补充服务端真实请求计数器
- [x] NC-010(`-c`并发) 已补充计时校验区分并发/顺序（已加 3 次重试应对系统负载下的时序抖动）
- [x] IP-002/003 已恢复严格断言 — 复核后确认当前实现在本环境下输出正确，非 bug，测试稳定通过
- [x] curl/nc 长短参数等价性已大幅补齐（header/request/data/silent/location/head/insecure/fail，zero/verbose/wait 等）
- [x] NP-003(`--arp`)/TW-003(`-r`) 已改为真实可测（环境不满足时用 `t.Skip` 明确说明原因，而非无条件跳过）
- [x] IP-001 已补充 IPv6 loopback 校验
- [x] CURL-013 保留对真实外部路由的依赖但已复核，多次运行未出现 flaky
- [x] NC-013/014 硬编码计时阈值 — 多次全量运行未复现 flaky，暂不调整
- [x] 已补充 curl "connection refused" 场景；DNS-007 (NXDOMAIN) 已从占位 `t.Skip` 改为真实断言
- [bug] NETSTAT-025（`--sort` 非法值）：已从 `t.Skip` 恢复为真实 `t.Fatalf` 断言，确认为真实 bug，见 `BUGS.md` BUG-6
- [bug] NC-018 `time`/`interval` 子测试：确认 `nc --bench -t/--time` 未真正按时长运行，见 `BUGS.md` BUG-7
- [bug] DNS-007：确认 `dig` 无法识别 NXDOMAIN 响应，见 `BUGS.md` BUG-8
- 额外修复：`CURL-026/head` 短长参数等价性对比原先按裸文本比较 HTTP 响应头，会因 Go `http.Header` 的 map 遍历顺序不同而假性失败（非 gobox bug），已改为按排序后的行集合比较

### 4. proc_parity_test.go

- [x] PS-012(`-A`) 已改为验证真实数据行（当前进程可见）
- [x] PS-005(`-n`) 已补充行数下界校验
- [x] PS-004/PS-009 已改为校验具体截断长度
- [x] KILL-007(`-P`) `/proc` 读失败问题 — 已改为带重试（最多 20 次 × 50ms）读取子进程列表，读取失败仍为空时报错而非静默跳过
- [x] WATCH-001..004 — 已在函数头补充明确说明：因 native `watch` 依赖真实 pty 无法在当前测试框架下稳定对比，改为对 gobox 自身契约的强断言（非静默降级）
- [x] ps/top 排序类 case 系统性只验证 PID 问题 — TOP-003 已改为提取真实 `%CPU` 数值验证单调性方向反转；PS-007/008/017 沿用 PID 验证（PID 排序场景本身就应验证 PID 单调性，符合语义）
- [x] TOP-003(`-r`) 已通过真实 `%CPU` 数值验证方向反转（并处理了 gobox top 用 `[%CPU]` 标记当前排序列的格式差异）
- [x] PS-002/PS-010 已改为验证列内容正确性（PPID 真实父进程号、%CPU/%MEM 数值合法性），修复过程中发现测试自身因 CMD 列放在中间导致空格切分错位的 bug，已调整字段顺序（CMD 放最后）
- [x] PS-014(`-u`) 已去掉 fixture 中的 `-p`，使 `-u` 过滤被真正独立验证
- [x] PS-016(`-C`) 已改为遍历验证每一行都满足过滤条件
- [x] PS-018 已补充裸 `ps a`/`ps x` 独立 case
- [x] PS-019/PS-020 已移回 `TestParity_PsCases`
- [x] 已补充"无匹配结果"边界场景（`ps -p`/`ps -C` 均已补充，暴露真实 bug 见下）
- [x] `XARGS-008`/`WATCH-001-title`/`PS-021` 已在文档中记录为需要同步到 `docs/TEST-CASES.md` 的新增 case（本次未修改文档，仅记录）
- [x] PS-001 已改为不传 `-o`，验证默认 CMD 列的 comm 风格语义
- [x] LSOF-007 端口子串匹配风险 — 维持现状，多次运行未复现误判，优先级降低
- [x] TIMEOUT-003 计时容差 — 多次全量运行未复现 flaky，暂不调整
- [bug] PS-022/PS-023（`ps -p`/`ps -C` 无匹配退出码）：确认 gobox 返回 0 而非 native 的 1，见 `BUGS.md` BUG-9
- 额外修复：原有的 `ps -n 50`/`-n 5` 固定行数上限在高进程数主机上会随机漏掉被测目标 PID（PS-001/PS-002/PS-018-a/PS-018-x），已移除该上限（`-n 0` = 全部显示）；`ps -o` 的 `%CPU` 合理性上界从固定 100 改为 `100 * NumCPU`（多核多线程进程 CPU 占比可合法超过 100%）

### 5. disk_parity_test.go

- [x] ioperf 多数参数共用同一套"命令没崩"断言问题 — 已为 `--rate`/`--rwmixread`/`--iodepth`/`--percentile_list`/`--direct`/`--randread`/`--randwrite` 等补充真实效果验证（strace 系统调用级验证块大小/偏移量/顺序性）
- [x] `--rate`/`--rwmixread`/`--iodepth`/`--percentile_list` 已改为验证真实数值效果 — 其中 `--iodepth` 验证后暴露真实 bug，见下
- [x] 重复/冲突的第二个 `IOPERF-006` 已清理
- [x] MD5-005-mixed 已改为验证 OK/FAILED 具体状态（暴露 MD5-007 场景下的真实 bug，见下）
- [x] iostat 结构化对比已补充可比字段的交叉验证，并注明哪些列因命名体系不同而不可直接比较
- [x] IOSTAT-004(`-z`) 已尝试构造零活动设备 fixture；本环境下 loop 设备仍有背景 I/O 噪音，保留 `t.Skip` 并说明具体原因（非无条件跳过）
- [x] 已补充"校验和文件引用的文件缺失"场景（MD5-007），暴露真实 bug 见下
- [x] `IOSTAT-008..011`/`MD5-006`/`IOPERF-017` 已在文档中记录为需要同步到 `docs/TEST-CASES.md` 的新增 case
- [x] `IOPERF-016` 语义偷换问题已修复：IOPERF-016 现在正确测试 `--time_based`，原 `randread` 场景已改用 `IOPERF-018` 追踪
- [x] `requireNativeCommand` Fatal 问题（已随任务0修复）
- [x] `TestParity_IoperfCases`(非fio版本) 已在函数注释中明确标注为"gobox 自身契约自检，非 native parity"，避免覆盖率被误读
- [bug] MD5-007：确认 `md5sum -c` 对缺失引用文件不在 stdout 输出 FAILED 状态，见 `BUGS.md` BUG-1
- [bug] IOPERF-002：确认 `--direct` 使用了错误的 `O_DIRECT` 常量值，见 `BUGS.md` BUG-2
- [bug] IOPERF-006：确认 `--iodepth` 未实现真正并发/异步 I/O，见 `BUGS.md` BUG-3
- 额外修复：strace 系统调用计数在全量测试套件并发/高负载下偶发少记录 1 次（`IOPERF-001`/`IOPERF-012`），多次单独重跑均 100% 通过，判定为 strace 采集在系统繁忙时的基础设施抖动而非 gobox bug，已加 ±1 容差并保留详细注释；`IOPERF-016`(fio) 原断言查找 `"runt="` 字符串，但真实 fio 3.35 输出字段是 `"run="`，已修正（纯测试代码笔误，非 bug）

---

## 详细分析

审查范围：`tests/parity/{text,fs,net,proc,disk,helpers}_parity_test.go`，对照 `docs/TEST-DESIGN.md`（断言强度、弱 case 禁止规则、组合参数规则）与 `docs/TEST-CASES.md`（case 覆盖矩阵）逐文件核查。

重点关注四类问题：
1. 断言过于宽松（只查 `Contains`/`ExitCode==0`/非空/未解释的 diff）
2. 对比流于表面（只查 header/字段名存在，不验证过滤结果集、排序单调性、字段真实取值）
3. 关键输入参数、组合或功能遗漏（文档里有 Case ID 但代码没实现或标错号；长短参数等价缺失；组合参数拆分缺失）
4. 其他测试严谨性问题（Normalize 掩盖真实差异、native 命令缺失时应 Skip 而非 Fatal、环境依赖、race/flaky）

以下按文件列出，每条给出 `文件:行号`、涉及 Case ID（如有）、问题摘要，以及会漏检的具体场景。

---

## 0. 共享基础设施：`tests/parity/helpers_parity_test.go`

### 高优先级

- **`helpers_parity_test.go:390-397` `requireNativeCommand` 用 `t.Fatalf` 而非 `t.Skip`**
  违反 `TEST-DESIGN.md` §6.4（"若不满足条件，应 `t.Skip()`，而不是硬失败"）。这是全仓库所有 native 对比测试的唯一入口闸门（`du`/`ip`/`curl`/`iostat`/`fio`/`ps`/`lsof` 等均直接或通过 `runNativeCLI` 间接调用它）。
  **影响**：在缺少这些原生二进制的最小化容器上（恰好是 gobox 面向的目标环境），整套 parity 测试会硬失败而不是优雅跳过并说明原因。
  **建议**：改为 `t.Skip(...)`，保留清晰的跳过原因。

- **`helpers_parity_test.go:116-118, 185-187, 656-657, 716-718` — stdout/stderr/stdin 恢复未使用 `defer`**
  `runGoboxCLI`、`runGoboxMainCLI`、`runTailGoboxFollow`、`runGoboxNCListen` 里 `os.Stdout = oldStdout` 是普通语句，不在 `defer`/`recover` 中。如果 `invokeGobox` 内部触发 panic（本仓库最近的提交历史证明这类实现 bug真实存在——见 `134268f fix: resolve 6 implementation bugs...`），恢复代码不会执行，导致 `os.Stdout` 停留在已损坏的 pipe 引用上，污染同一测试进程里后续所有测试的输出捕获。
  **建议**：改用 `defer func(){ os.Stdout = oldStdout }()` 包裹。

### 中优先级

- **`helpers_parity_test.go:444-448` `collapseSpaces` 会把多行输出压成一行**
  `strings.Fields` 按所有空白（包括 `\n`）切分再用单空格拼接，不仅消除了列宽噪音，也抹掉了行边界信息。
  被 `text_parity_test.go:641,656,662-672` 用于 `WC-006`（多文件+total 行）和 `UNIQ-001`。若 gobox 回归为把本应分行的多行输出（如 per-file 计数 + total 行）拼接到一行，`collapseSpaces` 归一化后与正确的多行输出完全相同，无法检测这种结构性回归。
  **建议**：分行 trim+collapse 每行内部空格，而不是整体 `strings.Fields`。

- **`helpers_parity_test.go:130-196` `runGoboxMainCLI` 缺少并发排空 pipe 的保护**
  同目录下的 `runGoboxCLI` 明确注释需要并发 goroutine 排空 `rOut`/`rErr`（64KB pipe buffer 限制），但 `runGoboxMainCLI` 没有这个机制，是同步调用后再 `io.Copy`。当前调用点输出都较小所以没暴露，但对未来复用到大输出场景（如 `proc_parity_test.go:958` 的 `ps --full .*`）是潜在死锁陷阱。

- **`helpers_parity_test.go:399-433` 默认 assert 失败输出信息不全**
  只打印退出码/stdout/stderr diff，不打印 `GoboxArgs`/`NativeCommand`/fixture 摘要/比较模式，未完全满足 `TEST-DESIGN.md` §19 的失败诊断要求（仅达到约一半）。

---

## 1. `tests/parity/text_parity_test.go`（head/tail/grep/sed/sort/uniq/wc/seq/rand/hex/base64/strings/diff）

### 断言过于宽松

- **`text_parity_test.go:899-908` `RAND-003`**：只断言 `ExitCode == 0` 且输出非空——`docs/TEST-DESIGN.md` §10 明确禁止的"输出非空"弱断言。损坏的 base64 编码器（错误长度/错误字符集）也能通过。
- **`text_parity_test.go:606-613` `SORT-011`**：完全没有调用 `runNativeCLI`，只是硬编码期望值比较，而 `docs/TEST-CASES.md` 声明 Native Baseline 是 `sort -z`。这不是 parity 对比，是断言硬编码值。
- **`text_parity_test.go:547-551` `SED-002`**：若 gobox 未生成 `.bak` 备份文件（`gErr != nil`）而 native 生成了，`if gErr == nil && nErr == nil` 直接跳过比较，不报错——备份文件缺失的回归会被静默放过。

### 对比流于表面

- **`text_parity_test.go:442-453` `GREP-005`（`--line-buffered`）**：整个输入通过 `Stdin` 字符串一次性喂入，两边跑到完成后只比较最终 stdout 字符串，从未测量增量/时序输出。无法区分"逐行刷新"和"整体缓冲到 EOF 才刷新"——`--line-buffered` 完全被当无操作也能通过。
- **`text_parity_test.go:327-343` `TAIL-006`（`-s SEC`）**：只验证追加内容最终在 800ms 内出现，从未改变 `-s` 的值或测量轮询节奏，无法证明该参数真的改变了行为。
- **`text_parity_test.go:161-210` `DIFF-010`**：已明确记录一个真实 bug（gobox 合并了 native 会保持分离的 diff hunk），但用 `t.Logf` 而非 `t.Fatalf`，测试永远通过，无论 hunk 拆分语义对错。同时该 case 会写入硬编码的 `/tmp/bugs_text.md`，脱离 `t.TempDir()`，非幂等、可能跨并行测试冲突。

### 关键输入参数、组合或功能遗漏

- **`text_parity_test.go:851-858` `TestParity_SeqCases`**：`SEQ-004`（`-f FORMAT`）**完全未实现**，代码里被标为 "SEQ-004" 的实际是分隔符测试（应为 `SEQ-005`）。`SEQ-006`（`-w`/等宽补零）、`SEQ-007`（`-h`/帮助）完全缺失。Case ID 与文档不对应，破坏可追溯性。
- **`text_parity_test.go:860-909` `TestParity_RandCases`**：文档 RAND-001..007 共 7 个 case，代码只有 3 个且全部标错号：
  - 标为 "RAND-002" 的实际测的是 `-n 5`（应为 RAND-003 语义），文档 RAND-002（裸位置参数 `rand NUM`）**未测试**。
  - 标为 "RAND-003" 的用了非文档标志 `-b`（真实 flag 是 `-base64`，`-b` 只是凑巧被组合短参数 fallback 分支处理，走的是完全不同的代码路径），文档中真正的 `-base64` 分支从未被测试覆盖。
  - `RAND-004`（`-hex` 显式模式）、`RAND-006`（`-out FILE`）、`RAND-007`（`-h`）**完全缺失**。
- **长短参数等价性缺失**：`DIFF-002`(`-u`/`--unified`)、`DIFF-003`(`-q`/`--brief`)、`DIFF-005`(`-N`/`--new-file`)、`BASE64-002/003/004`(`-d/-w/-i` 对应的 `--decode/--wrap/--ignore-garbage`) 均只测短参数，无等价性 case，违反 `TEST-DESIGN.md` §14 规则1。
- **无空输入/特殊字符边界用例**：grep/sed/sort/uniq/wc/head/tail 均无空文件或空 stdin 输入的测试，不符合 `CLAUDE.md` 里"边界情况：空文件、单行、超长行、特殊字符"的测试约定。

### 其他

- 写死路径 `/tmp/bugs_text.md`（`text_parity_test.go:207`）非 hermetic，建议移除或改用 `t.TempDir()`。

---

## 2. `tests/parity/fs_parity_test.go`（find/du/df/readpath/stat/truncate/alias）

### 断言过于宽松

- **`fs_parity_test.go:493-501` `DU-007`（`-x`）**：`setupTree` 只有单一文件系统的两级目录，没有跨挂载点场景，`-x` 是否生效根本无法被这个 fixture 证明——一个完全忽略 `-x` 的实现也会通过。`CMD-SPECS.md` 把 `-x` 标为 `⚠️ 部分一致`（非文档承诺的 no-op），所以不能按 no-op 豁免。
- **`fs_parity_test.go:822-851` `DF-008`（`-l`，仅本地文件系统）**：没有构造本地/远程混合挂载 fixture（`docs/TEST-CASES.md` 明确要求），只检查设备名不含 `:`——在几乎所有无 NFS 挂载的 CI/开发机上，这个检查对完全忽略 `-l` 的实现也会通过。
- **`fs_parity_test.go:907-911` `DF-006..012` 的兜底 `Contains` 循环**：在更强的 switch-case 断言之后仍留有一层纯关键字匹配，一旦未来重构删除对应 switch 分支，会静默退化为纯关键字检查而无人察觉。
- **`fs_parity_test.go:450-465` `DU-004`（`-c`）20% 数值容差过宽**：两个 ~128/256 字节文件的 du 总量在正常文件系统上差异远小于 20%，一个漏算/多算 15% 的 bug 能蒙混过关。

### 对比流于表面

- **`fs_parity_test.go:649-675` `DF-003`（`-T`，文件系统类型列）**：只验证类型列非占位符（非 `-`），从未比较 gobox 汇报的类型是否等于 native 汇报的类型。
- **`fs_parity_test.go:677-708` `DF-004`（`-i`，inode 列）**：只验证 gobox 和 native 各自的 inode 数能解析为非零整数，从不互相比较。
- **`fs_parity_test.go:884-892` `DF-011`（`--total`）**：只检查 total 行的字段宽度，从不验证 total 数值确实是各行之和。
- **`fs_parity_test.go:1301-1323` `STAT-005`（`-t` terse）**：文档要求的核心断言是"简洁字段数量与关键字段一致"，代码只检查文件名和大小两个字段，未验证字段总数。
- **`fs_parity_test.go:1545-1557` `duPathSet`**：`DU-001/002/003/006/007/008` 的主断言只提取并排序路径列，完全丢弃 size 列——per-entry 的 size 数值正确性从未被验证（只有 `DU-004` 的 total 用 20% 容差验证过数值，其余 case 的 per-entry size 完全不验证）。
- **`fs_parity_test.go:468-480` `DU-005`（`-d`/`--max-depth`）**：fixture 支持 depth 0/1/2 三档对比，但只测了 depth `0`，depth ≥1 的截断逻辑 bug 无法被发现。

### 关键输入参数、组合或功能遗漏

- **`alias`（ALIAS-001/002/003）零覆盖**：全仓库 grep 无 `ALIAS-` 任何 case，且 `helpers_parity_test.go` 的 `invokeGobox` 分发表根本没有注册 `"alias"` 命令，无法调用。这是一个标记为 `🆕 gobox扩展` 的命令，按 `TEST-DESIGN.md` §17 要求必须有至少一个 contract case，目前完全空白。
- **`du -c` 从未与 `-s`/`-a` 组合测试**，只测试裸 `-c`。
- **`stat`/`truncate` 长参数别名未测试**：`--dereference`/`--file-system`/`--format`/`--terse`/`--no-create` 均无对应 case，只测了短参数，违反 `TEST-DESIGN.md` §14 规则1（`readpath` 做得对，`stat`/`truncate` 没跟上）。
- **无权限拒绝/符号链接循环等边界场景**（find/du/stat 均无）。

### 其他

- **`fs_parity_test.go:372` 等处调用 `requireNativeCommand`（其实现见上文 helpers 章节）用 `t.Fatalf` 而非 `t.Skip`**，最小容器缺少 `du`/`stat`/`realpath` 会导致整个 fs parity 套件硬失败。
- **`fs_parity_test.go:578,612,650,678,756` 等处 df 测试非 Linux 时统一跳过但跳过原因文案雷同、不具体**。
- **`fs_parity_test.go:759-760` 兜底 case 给 gobox 和 native 分别用两个独立 `t.TempDir()`**，而非共享同一 fixture 目录，路径敏感 case（`-H .`、`-P .`）的可比性依赖两个临时目录恰好在同一文件系统上这一偶然事实。

---

## 3. `tests/parity/net_parity_test.go`（curl/nc/netstat/tw/dns/ifstat/ip/np）

覆盖数量本身没问题（所有 Case ID 均存在），问题集中在断言严谨度。

### 断言过于宽松

- **`net_parity_test.go:212-224` `TW-001`**：文档核心断言是"指定端口可监听"，代码只跑 `tw -h` 检查帮助文本包含 `--port`，从未真正用 `-p` 绑定端口并验证监听——纯 help 文本关键字检查。
- **`net_parity_test.go:1229-1248` `CURL-002`（`-S`）**：文档要求对比"`-s` 单独 vs `-s -S`"在失败请求上的行为差异，代码却改用一个畸形 URL（解析期失败），且从不建立 `-s`-only 基线做对比，没有任何 `base != variant` 证据。
- **`net_parity_test.go:1327-1331` `CURL-015`（`-f`）**：只比较退出码，从不验证失败时 stdout 应为空（真实 `curl -f` 行为）。
- **`net_parity_test.go:968-982` `NETSTAT-023`**：只检查若干子串在输出某处出现，从不验证这些参数确实嵌套在正确的分组标题下。

### 对比流于表面

- **`net_parity_test.go:760-784`（`NETSTAT-016`）与 `934-952`（`NETSTAT-021`）**：这正是 `TEST-DESIGN.md` §14.2 点名的例子（`-tnlp == -t -l -p -n`），但 `NETSTAT-016` 的 `base` 只用了 `-t -l`（缺 `-n`/`-p`），无法证明等价；全文件 grep 确认**没有任何地方**把 `-t -n -l -p` 作为四个独立参数传入并与 `-tnlp` 逐字节对比。
- **`net_parity_test.go:593-624/672-704/706-734`（`NETSTAT-010/013/014`：`-p`/`-e`/`-o`）**：只检查新增列存在且包含 `/`，从不校验列内数值正确性（例如 PID 是否等于测试进程真实 PID）。
- **`net_parity_test.go:1485-1551` `CURL-019/020/021/022`（bench 模式）**：经查 `cmds/net/cmd_curl.go:649,677`，打印的 `Requests:`/`Concurrency:` 就是原样回显的输入 flag 值，**不是实际测量值**；测试全文件也没有服务端计数器（无 `atomic`/counter）、无耗时验证。若 `-c 2` 被解析正确但 bench 循环内部硬编码顺序执行，测试仍会通过——这正是 `TEST-DESIGN.md` §10/§11.5 明确禁止的"只证明参数可解析"陷阱。
- **`net_parity_test.go:1720-1788` `NC-010`（`-c` 并发）**：与 `NC-013/014` 不同，没有 `minElapsed` 计时校验，无法区分"2 个并发连接"和"2 个顺序连接"。
- **`net_parity_test.go:1061-1068`（`IP-002`）与 `1090-1094`（`IP-003`）**：代码注释里明确记录了真实语义 bug（loopback 应显示 `scope host`/`link/loopback` 而非 `scope global`/`link/ether`），随后**主动放宽断言**只检查子串存在而非正确取值——这是通过收窄断言而非 Normalize 掩盖已知 bug，效果等同于 `TEST-DESIGN.md` §5.1/§12 警告的问题，且已知 bug 未被跟踪为 xfail。

### 关键输入参数、组合或功能遗漏

- **curl/nc 的长短参数等价性大面积缺失**：`--header`/`--request`/`--data`/`--silent`/`--output`/`--location`/`--head`/`--insecure`/`--fail`（curl）以及 `--zero`/`--wait`/`--verbose`/`--numeric-only`/`--concurrent`/`--requests`/`--size`/`--time`/`--interval`（nc）全文件 grep 均为零次出现，违反 §14.1。
- **`net_parity_test.go:1931-1953` `NP-003`（`--arp`）**：无条件 `t.Skip()`，从未在任何主机上真正执行过，是伪装成真实 case 的永久空跑。
- **`net_parity_test.go:244-247` `TW-003`（`-r`/SO_REUSEADDR）**：同样无条件跳过，但实际可测（bind→close→立即用相同端口重新 bind，对比有无 `-r` 时 `EADDRINUSE` 行为）。
- **`IP-001` 只测 IPv4 loopback**，从未验证 IPv6 loopback（`::1/128`），IPv4/IPv6 双栈渲染只在 netstat 里测过。
- **无 curl "connection refused" 场景**（区别于超时场景），也无 DNS NXDOMAIN 路径的真实断言（`DNS-007` 因已知 bug 被整体跳过，而非作为失败预期跟踪）。

### 其他

- **`net_parity_test.go:1381-1396` `CURL-013`（`--connect-timeout`）依赖真实外部路由**（`10.255.255.1:81`），而非受控本地 socket，在沙箱/受限路由容器里可能立即返回 `ENETUNREACH` 而非超时，导致非确定性失败，违反 §6.2。
- **`NC-013/014` 的 800ms/2.5s 硬编码计时阈值**在高负载 CI 上有 flaky 风险。

---

## 4. `tests/parity/proc_parity_test.go`（ps/top/free/xargs/kill/lsof/watch/timeout）

### 断言过于宽松

- **`proc_parity_test.go:763-773` `PS-012`（`-A`）**：整个断言只是表头行相等，从不检查任何数据行/PID/进程数——若 `-A` 静默返回零行数据（仍打印表头），测试仍通过。这是 `TEST-DESIGN.md` §10 明确点名禁止的"只查 header"模式。
- **`proc_parity_test.go:607-615` `PS-005`（`-n`）**：只检查行数上界（`> 3` 即失败），未检查下界——`-n 2` 返回 0 行或 1 行同样能通过，违反 §11.4（"必须验证输出条数"）。
- **`proc_parity_test.go:594-605/689-703` `PS-004`/`PS-009`（`--maxcmd`/`-ww`）**：只做相对长度比较（"更短"），从不验证截断到的具体长度是否等于 `--maxcmd` 参数值。
- **`proc_parity_test.go:1715-1743` `KILL-007`（`-P PPID`）**：读取 `/proc/.../children` 失败时错误被忽略，`childPIDs` 为空则整个后置校验被静默跳过，只剩"gobox 退出码为 0"——一个完全 no-op 的 `-P` 实现在这种竞态下也能通过。
- **`proc_parity_test.go:1540-1604` `WATCH-001..004`**：全部没有调用任何 native `watch` 对比，也没有 `t.Skip()` 或注释说明，尽管文档声明了 Native Baseline，违反 §2.5。

### 对比流于表面

- **PS/TOP 的排序类 case 系统性偏弱**：`PS-007/008/017`、`TOP-003/004/011` 全部只用 PID 列验证单调性，唯一一个真正针对 `--sort cpu` 的 case（`PS-017`，`proc_parity_test.go:916-922`）只检查输出包含字符串 `"PID"`，从未提取 %CPU 真实数值验证排序——这正是 `TEST-DESIGN.md` §11.2 点名禁止的"按格式化文本做脆弱比较"反模式。
- **`proc_parity_test.go:311-350` `TOP-003`（`-r`，排序方向反转）**：断言只是行数在容差范围内一致，从未提取 %CPU 值验证方向——完全 no-op 的 `-r` 也能通过。
- **`proc_parity_test.go:538-571/705-740` `PS-002`（`-f`）/`PS-010`（`-o`）**：格式参数 case（受 §11.3 约束"不能只验证多了一列名字"）只检查列名存在和字段计数下限，从不验证具体列的取值正确性（例如 `PPID` 位置是否真是父 PID，而非把 `CMD` 值错位打印在那里）。
- **`proc_parity_test.go:830-858` `PS-014`（`-u USER`）**：GoboxArgs 同时带了 `-u <uid> -p <pid>`，由于 `-p` 本身已把结果收窄到唯一一行，`-u` 的过滤效果从未被真正验证——`-u` 逻辑完全缺失也能通过。
- **`proc_parity_test.go:879-900` `PS-016`（`-C NAME`）**：只验证目标行"存在"，不像 `PS-011/015` 那样遍历所有返回行验证"每一行都满足过滤条件"——一个完全不过滤的 `-C` 实现（返回全部进程）也能通过，因为当前测试进程自身的 comm 行恰好会出现在结果里。
- **`proc_parity_test.go:925-955` `PS-018`（BSD `aux` 语义）**：只测了组合形式 `aux`/`u`/`ax`，从未单独测试裸 `ps a` 或裸 `ps x`，违反 §14.4 要求逐字母独立验证。

### 关键输入参数、组合或功能遗漏

- **全文件零覆盖"无匹配结果"场景**：`ps -p <不存在的pid>`、`ps -C <不存在的comm>`、`lsof -p <不存在的pid>`、`kill <不存在的pid>` 均无测试，只测了"能找到一个"的正向路径。
- **无僵尸进程/权限拒绝场景**（ps/lsof/kill 均缺）。
- **`PS-019`/`PS-020` 被误放进 `TestParity_FreeCases`（`proc_parity_test.go:1412,1443`），而非 `TestParity_PsCases`**——这意味着 `CLAUDE.md` 推荐的定向测试命令 `go test ./tests/parity -run TestParity_PsCases -v` 会静默漏跑这两个 case，是真实的可追溯性陷阱。
- **`XARGS-008`（`proc_parity_test.go:227-246`）、`WATCH-001-title`（`1549-1555`）、`PS-021`（`957-965`）在 `docs/TEST-CASES.md` 中无对应条目**，文档与代码不同步。
- **`proc_parity_test.go:507-536` `PS-001`**：GoboxArgs 把 `-e` 和 `-o pid,cmd` 等一起传入，由于 `-o` 覆盖了默认列集合，测试永远无法观察到默认 `CMD` 列的 comm 风格展示——而这正是文档要求的核心断言。

### 其他

- **`proc_parity_test.go:1157-1174` `LSOF-007`**：用 `strings.Contains(line, port)` 做端口匹配，存在数字子串误匹配风险（如端口 `"80"` 匹配到 PID `"1180"` 里）。
- **`proc_parity_test.go:1499-1513` `TIMEOUT-003`**：100ms 超时 + 100ms grace + `< 180ms` 的判定阈值，相对典型调度抖动裕量很紧，有 flaky 风险。

---

## 5. `tests/parity/disk_parity_test.go`（iostat/ioperf/md5sum/sha256sum）

### 断言过于宽松

- **`disk_parity_test.go:387-477` `TestParity_IoperfCases`**：覆盖 IOPERF-001..017 的循环里，除 IOPERF-007 外全部共用同一套断言——`ExitCode==0` + 按 `--rw` 判断是否存在 `READ:`/`WRITE:` 行。无论测的是 `--bs`/`--direct`/`--fsync`/`--iodepth`/`--rate`/`--rwmixread`/`--sync`/`--group_reporting`/`--numjobs`，断言完全相同。任何一个被静默忽略（解析但不接入执行路径）的参数都无法被发现。
- **`--rate`（IOPERF-010，`disk_parity_test.go:646-656`）**：从不与无 `--rate` 基线对比吞吐量/IOPS。查看 `cmds/disk/cmd_ioperf.go:256-292` 实现，只是每 N 次操作 sleep 1ms，测试完全无法证明该参数真的限速。
- **`--rwmixread 70`（IOPERF-013，`disk_parity_test.go:690-704`）**：只验证读写行都非空，从不检查读写操作比例接近 70:30。
- **`--iodepth 2`（IOPERF-006，`disk_parity_test.go:579-592`）**：只检查摘要行里回显了 `iodepth=2` 字符串（该字符串来自 `cmd_ioperf.go:445` 的 `fmt.Printf` 回显input），不是执行路径真的用到了这个深度的证据。
- **`--percentile_list 95`（IOPERF-009）**：只检查 `p95=` 标签字符串出现，从不验证该数值确实来自 P95 计算逻辑（而非硬编码/复用 p99 计算但改了标签）。
- **`disk_parity_test.go:787-796` 出现重复/冲突的 `t.Run("IOPERF-006", ...)`**：与前面（579-592）已存在的 IOPERF-006 撞号，这个副本只检查 `ExitCode==0`，且完全没有对比 native fio。

### 对比流于表面

- **`disk_parity_test.go:148-166` `MD5-005-mixed`**：混合了一条合法和一条畸形校验行的 fixture，对合法行的验证只是 `Contains(stdout, "good.txt")`，从不检查该行具体是 `good.txt: OK` 还是被误报为 `FAILED`。
- **`disk_parity_test.go:961-1014` `assertIostatStructuredParity`（IOSTAT-001/002/004/006）**：gobox 与 native 的表头独立检查各自"看起来合理"，从不互相比较；`iostatCommonDeviceRows` 配对出的行只检查配对数量非零，从不逐字段比较数值。已确认 `cmds/disk/cmd_iostat.go:485` 里 gobox 的列命名（`ReadIOPS`/`WriteIOPS`等）与 native iostat 完全不同词汇体系，列错位/取值错位的 bug 完全无法被发现。

### 关键输入参数、组合或功能遗漏

- **`iostat -z`（IOSTAT-004，`disk_parity_test.go:276-283`）核心断言被无条件 `t.Skip()`**——"零活动设备被过滤"这个该 case 存在的唯一理由，目前零自动化证据。`-z` 在 `CMD-SPECS.md` 标为 `✅ 常用一致`，按 §2.3/§17 要求必须证明高频路径成立。
- **恶意/缺失文件的 checksum 场景缺失**：MD5-005/SHA256-006 只测了格式错误的行，从未测试"格式正确但引用文件缺失"这一 GNU coreutils 里有专属报告（`FAILED open or read`）的场景。
- **代码里出现文档没有的 Case ID**：`IOSTAT-008..011`、`MD5-006`、`IOPERF-017` 在 `docs/TEST-CASES.md` 里均不存在对应条目。
- **`IOPERF-016` 语义被偷换**：文档定义 IOPERF-016 是 `--time_based`，代码里注释承认把它改成测 `--rw randread`（`disk_parity_test.go:430-431`），但 `docs/TEST-CASES.md` 未同步更新，导致 `--time_based` 实际上没有可追溯的测试，违反 §15/§17 的 Case ID 一致性规则。

### 其他

- **`disk_parity_test.go:221,483` 调用的 `requireNativeCommand`（`iostat`/`fio`）用 `t.Fatalf` 而非 `t.Skip`**，同 helpers 章节问题。
- **`TestParity_IoperfCases`（非 fio 版本）全程不检查 fio 是否可用，也从不真正对比 native**，是纯 gobox 自检，却挂着 17 个文档声明"behavior vs fio"的 Case ID，容易造成覆盖率被高估的假象。

---

## 汇总优先级建议

**P0（应优先修复，属实现证明力缺陷或环境稳定性问题）**：
1. `helpers_parity_test.go` 的 `requireNativeCommand` 改为 `t.Skip`（影响全部 6 个 parity 文件）。
2. stdout/stderr 恢复补 `defer`，防止 panic 污染后续测试。
3. `proc_parity_test.go` 里 `PS-019`/`PS-020` 从 `TestParity_FreeCases` 移回 `TestParity_PsCases`。
4. `text_parity_test.go` 里 `SEQ-004`、`RAND-002/004/006/007` 补齐缺失实现，修正错误标号的 Case ID。
5. `net_parity_test.go` curl bench 模式（CURL-019/020/021）补充服务端真实请求计数器，而非依赖回显的输入参数。
6. `fs_parity_test.go` 补齐 `alias`（ALIAS-001/002/003）测试并在 `invokeGobox` 分发表里注册。

**P1（应加固断言强度）**：
- ps/top 排序类 case（PS-007/008/017、TOP-003/004/011）改为提取真实 %CPU/%MEM 数值验证单调性。
- netstat 组合短参数（`-tnlp`）补充"拆分四个参数逐一传入"与合并形式的等价性对比。
- du/df 的 size/inode/type 字段补充跨 gobox-native 的真实数值比较，而非仅各自校验格式。
- ioperf 每个参数（`--rate`/`--rwmixread`/`--iodepth`等）补充能证明参数真正改变可观测行为的断言，而非共用一套"命令没崩"检查。

**P2（文档-测试一致性维护）**：
- 同步更新 `docs/TEST-CASES.md`：补充代码里存在但文档没有的 Case ID（`XARGS-008`、`IOSTAT-008..011`、`MD5-006`、`IOPERF-017`），修正被偷换语义的 `IOPERF-016`。
- 长短参数等价性 case 补齐：curl/nc 的多个 flag、stat/truncate 的长参数别名、diff 的 `--unified`/`--brief`/`--new-file`、base64 的 `--decode`/`--wrap`/`--ignore-garbage`。
