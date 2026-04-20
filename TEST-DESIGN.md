# gobox Parity Test Design

## 1. 目标

为 `docs/CMD-DESIGN.md` 中列出的命令和参数建立一套可持续维护的“原生命令对比测试”体系，用于验证：

1. `gobox` 与原生命令在已声明为 `✅ 一致` 的条目上，行为、输出和退出码尽量一致。
2. `gobox` 在已声明为 `⚠️ 部分一致` 的条目上，差异被限制在文档描述范围内。
3. `gobox` 的扩展能力（`🆕 gobox扩展`）在没有直接原生命令映射时，仍然具备稳定、可回归的行为约束。
4. `docs/CMD-DESIGN.md` 能作为测试矩阵来源，而不是纯说明文档。
5. Parity 测试可以直接纳入 `go test ./...`，作为后续命令兼容性回归基线。

本设计把这类测试统一称为：

- 中文：`命令兼容性对比测试` / `行为一致性测试`
- 英文：`command compatibility tests` / `command parity tests`

## 2. 设计原则

### 2.1 双层验证

测试体系分成两层：

1. **规范对齐测试（spec alignment tests）**
   - 只验证 `gobox` 自身是否符合 `docs/CMD-DESIGN.md`
   - 适合 gobox 扩展参数或原生命令无直接等价物的能力
   - 输出要求稳定，不依赖宿主机原生命令差异

2. **原生命令对比测试（native parity tests）**
   - 直接对比 `gobox <cmd>` 与系统原生命令 `<cmd>`
   - 比较 `stdout` / `stderr` / `exit code` / 关键语义
   - 适合 `docs/CMD-DESIGN.md` 中标记为 `✅ 一致` 的条目

### 2.2 按命令类型分层比较

不是所有命令都适合逐字节对比，按类型拆分：

1. **严格对比（Exact parity）**
   - 适合：`grep`, `sed`, `wc`, `sort`, `uniq`, `head`, `tail`, `md5sum`, `xargs`, `find` 的稳定子集
   - 比较：`stdout`, `stderr`, `exit code`

2. **半结构化对比（Structured parity）**
   - 适合：`ps`, `netstat`, `du`, `ifstat`, `iostat`
   - 比较：字段存在性、过滤结果、排序语义、关键行集合
   - 不要求列宽、时间戳、PID 顺序等完全逐字节一致

3. **能力对比（Behavioral parity）**
   - 适合：`curl`, `nc`, `np`, `tw`, `nslookup/dig`, `tail -f`
   - 比较：连接成功/失败、状态码、关键 header、超时、过滤/解析语义
   - 不比较易波动的非关键细节（如 `Date` 头、动态端口、系统错误措辞）

4. **契约验证（Contract-only）**
   - 适合：`ioperf`, `tw --bench`, `curl --bench`, `nc --bench` 等 gobox 扩展
   - 只验证 gobox 的参数解析和核心行为约束

### 2.3 将 `CMD-DESIGN` 当作测试矩阵来源

`docs/CMD-DESIGN.md` 中每个命令、每个参数都应映射到至少一个测试案例。

规则：

- `✅ 一致`：必须至少有一个 parity case
- `⚠️ 部分一致`：必须至少有一个“差异边界验证 case”
- `🆕 gobox扩展`：必须至少有一个 gobox-only contract case

### 2.4 Case-first 实施

先写 `TEST-CASES.md`，再写测试代码，最后修实现；避免“已经写了很多测试，但不知道覆盖了哪些文档条目”。

### 2.5 可跳过但不可沉默缺失

当宿主机缺少原生命令、内核能力或网络条件时：

- 可以 `t.Skip()`
- 但必须说明跳过原因
- 不允许直接删 case 或静默降级为不测

## 3. 测试分类

### 3.1 Exact parity

适用条件：

- 输入可完全控制
- 原生命令可稳定获得
- 输出文本应高度一致

比较项：

- `stdout`：逐字节比对（经 normalize 后）
- `stderr`：逐字节比对（经 normalize 后）
- `exit code`：严格一致

### 3.2 Structured parity

适用条件：

- 原生命令输出包含动态列宽、动态顺序、进程/系统噪音
- 逐字节比对不稳定

比较项：

- 表头字段
- 行集合/过滤结果
- 排序方向
- 关键字段值
- 退出码

### 3.3 Behavioral parity

适用条件：

- 需要本地 server / socket / DNS / HTTP 环境
- 更关注语义而非文本外观

比较项：

- 是否成功
- 状态码 / 退出码
- 请求方法
- header/body 是否符合预期
- 是否发生重试、超时、重定向、跟随等行为

### 3.4 Contract-only tests

适用条件：

- gobox 扩展能力没有直接原生命令
- 或原生命令语义不具可比性

比较项：

- 只验证 gobox 是否满足文档/设计约束
- 不进行 native command 对比

## 4. 测试模型

建议统一使用 table-driven 结构描述案例。

```go
type CompareMode string

const (
    CompareExact      CompareMode = "exact"
    CompareStructured CompareMode = "structured"
    CompareBehavior   CompareMode = "behavior"
    CompareContract   CompareMode = "contract"
)

type ParityCase struct {
    ID            string
    Name          string
    Command       string
    Mode          CompareMode
    GoboxArgs     []string
    NativeCommand string
    NativeArgs    []string
    Stdin         string
    Files         map[string]string
    Env           map[string]string
    Setup         func(t *testing.T, env *ParityEnv)
    Normalize     []Normalizer
    Assert        func(t *testing.T, got GoboxNativeResult)
}
```

约定：

- `NativeCommand == ""` 表示 gobox-only contract case
- `Setup` 负责额外夹具，例如 socket / server / child process
- `Normalize` 仅做“非关键差异归一化”，不能掩盖真实语义差异
- `Assert` 用于结构化或行为级断言

## 5. 归一化（Normalization）策略

原生命令和 `gobox` 即便功能一致，也可能在非关键细节上不同，因此所有 parity case 都需要 Normalize 层。

### 5.1 常见归一化对象

1. 临时目录绝对路径
2. PID / PPID / 动态端口
3. 时间戳
4. 多空格与列宽
5. 行尾空白
6. header 顺序
7. 平台相关错误前缀
8. HTTP `Date` / `Server` 等动态头
9. GNU / BusyBox / BSD 间可接受的帮助文本差异

### 5.2 Normalizer 类型

建议实现以下几类：

- `TrimTrailingSpace`
- `NormalizeNewlines`
- `CollapseSpaces`
- `NormalizeTempPaths`
- `NormalizeTempDirPrefix`
- `NormalizeProcColumns`
- `StripDynamicHeaders`
- `SortLines`
- `NormalizeHostPort`

不同命令选择不同 normalizer，避免“一刀切”把信号抹掉。

## 6. 环境控制原则

### 6.1 文件类命令

- 统一使用 `t.TempDir()` 构造输入文件树
- 禁止依赖仓库外部路径
- 所有输入在测试中显式创建

### 6.2 网络类命令

- 优先使用 `httptest.Server`
- socket 类测试优先绑定 `127.0.0.1:0`
- 不依赖公网
- 不依赖真实外部 DNS
- DNS 测试优先用本地可控 DNS server；若当前仓库未实现，则 case 允许 skip

### 6.3 进程类命令

- 只验证当前测试进程、子进程或本地 listener
- 不依赖宿主机全量进程状态
- 不要求与系统所有进程完全逐行一致

### 6.4 原生命令可用性检测

测试前需检测：

- 命令是否存在（`exec.LookPath`）
- 当前 OS 是否支持
- 当前环境是否允许该测试稳定运行

若不满足条件，应 `t.Skip()`，而不是硬失败。

## 7. 目录结构

建议使用独立目录承载 parity 测试：

```text
tests/parity/
  helpers_test.go
  text_parity_test.go
  fs_parity_test.go
  net_parity_test.go
  proc_parity_test.go
  disk_parity_test.go
```

职责划分：

- `helpers_test.go`
  - 提供统一执行器：运行 gobox、运行原生命令、收集 stdout/stderr/exit code
  - 提供 temp file / temp dir / local server / local socket 辅助函数
- `*_parity_test.go`
  - 仅声明命令级案例矩阵与断言
- 不再在命令实现目录里混杂 parity 逻辑，避免 unit test 与 parity test 耦合

## 8. 执行器设计

执行器需要统一返回：

```go
type CmdResult struct {
    Stdout   string
    Stderr   string
    ExitCode int
}

type GoboxNativeResult struct {
    Gobox  CmdResult
    Native CmdResult
}
```

执行流程：

1. 创建测试夹具目录
2. 运行 gobox 函数入口，而不是临时构建 `gobox` 二进制
3. 运行原生命令（若配置）
4. 应用 normalizer
5. 执行 exact / structured / behavioral / contract 对应比较逻辑
6. 输出最小 diff 与命令行上下文

额外要求：

- 所有 parity case 默认串行执行命令主体，避免并发修改全局 `os.Stdout` / `os.Stderr` / `os.Stdin`
- 网络 server、文件准备、静态夹具可以在 case 内并发准备
- 执行器要能显式表示“跳过 baseline，只跑 gobox contract”

## 9. 断言策略

### 9.1 Exact

- `stdout == stdout`
- `stderr == stderr`
- `exit code == exit code`

### 9.2 Structured

通过解析输出后比较：

- 是否包含必需列
- 是否包含目标记录
- 过滤结果是否一致
- 排序是否正确
- 差异是否仍在 `CMD-DESIGN` 文档允许范围内

### 9.3 Behavioral

不要求文本完全一致，仅比较：

- 成功/失败
- 退出码
- 请求/响应关键字段
- 是否发生重试/重定向/超时
- 是否命中正确的 server / DNS / 端口

### 9.4 Contract

只对 `gobox` 做约束：

- 参数可解析
- 行为符合文档
- 输出中包含关键指标
- 未实现的环境能力要明确 skip，而不是静默通过

## 10. 命令级推荐比较方式

### 10.1 文本命令

- `grep`：Exact parity 为主，含 context / file-list / recursive / quiet 模式
- `sed`：Exact parity
- `head`：Exact parity
- `tail`：Exact parity + Behavioral parity（`-f` / `--retry` / `--pid`）
- `sort`：Exact parity
- `uniq`：Exact parity
- `wc`：Exact parity
- `xargs`：Exact parity 为主，`-P` 增加行为约束

### 10.2 文件系统命令

- `find`：Exact parity（稳定子集），时间过滤使用 controlled timestamps
- `du`：Structured parity（不同系统 block size 噪音较大）

### 10.3 网络命令

- `curl`：Behavioral parity + Contract tests
- `nc`：Behavioral parity + Contract tests
- `nslookup/dig`：Behavioral parity（本地 DNS / controlled endpoint 优先）
- `tw`：Contract tests
- `ifstat`：Structured parity / Contract tests
- `netstat`：Structured parity
- `np`：Behavioral parity / Contract tests

### 10.4 进程命令

- `ps`：Structured parity
- `top`：Contract tests（单迭代模式为主）

### 10.5 磁盘命令

- `md5sum`：Exact parity
- `iostat`：Structured parity / Contract tests
- `ioperf`：Contract tests

## 11. `CMD-DESIGN` 驱动策略

`TEST-CASES.md` 作为 `docs/CMD-DESIGN.md` 的测试映射表，要求：

1. 每个命令每个参数至少 1 个案例
2. 每个案例必须标注：
   - 比较类型
   - 原生命令映射
   - 输入夹具
   - 核心断言
3. `✅ 一致` 条目优先进入 native parity automation
4. `⚠️ 部分一致` 条目必须有“差异边界案例”
5. `🆕 gobox扩展` 条目必须有 contract case
6. 测试代码中的 `Case ID` 必须能直接追溯到 `TEST-CASES.md`

## 12. 落地阶段与提交节奏

### 阶段 1：设计与案例矩阵

产出：

- `TEST-DESIGN.md`
- `TEST-CASES.md`

提交建议：

- `docs: refine parity design`
- `docs: expand parity cases`

### 阶段 2：执行器与首批稳定命令

优先覆盖：

1. `grep`
2. `sed`
3. `head`
4. `tail`
5. `sort`
6. `uniq`
7. `wc`
8. `md5sum`
9. `xargs`
10. `find`

### 阶段 3：结构化命令

1. `du`
2. `ps`
3. `top`
4. `netstat`
5. `ifstat`
6. `iostat`

### 阶段 4：网络与扩展能力

1. `curl`
2. `nc`
3. `nslookup/dig`
4. `np`
5. `tw`
6. `ioperf`

每一阶段完成后都应：

- 运行对应定向测试
- 修复暴露的问题
- 再提交一次代码

## 13. 失败输出要求

Parity 测试失败时，必须尽量输出：

- 测试名称与 `Case ID`
- gobox 命令行
- native 命令行
- 输入夹具摘要
- gobox 结果
- native 结果
- normalize 后 diff
- 属于哪种比较模式

避免只输出 “not equal” 这类不可诊断信息。

## 14. CI 与回归约束

建议将 parity 测试纳入以下执行层次：

1. 本地开发：`go test ./tests/parity/...` 或带 `-run` 过滤
2. 提交前：命令相关包测试 + parity 定向测试
3. 全量回归：`go test ./...`

若某类 parity 测试存在环境依赖，应通过 `t.Skip()` 控制，而不是从默认回归中摘除。

## 15. 当前仓库落地约定

结合当前仓库状态，初版落地采用以下约定：

1. 使用函数接口直接调用命令实现，不构建临时 `gobox` 二进制
2. 将 parity helpers 与 case 文件集中到 `tests/parity/`
3. 允许保留少量 root 级 parity 测试作为迁移过渡，但最终以 `tests/parity/` 为主
4. 优先让 `docs/CMD-DESIGN.md` 中所有命令参数都能在 `TEST-CASES.md` 找到映射
5. 所有 parity 测试最终都纳入 `go test ./...` 可执行范围

## 16. 完成标准

认为 parity 测试体系“完成首版落地”，至少需要满足：

1. `TEST-DESIGN.md` 明确说明四类测试模型、执行器、skip 策略与目录结构
2. `TEST-CASES.md` 覆盖 `docs/CMD-DESIGN.md` 当前所有命令参数
3. 自动化测试代码中已落地全部 `Case ID`，或对环境依赖项显式 `Skip`
4. `go test ./...` 通过
5. 若发现实现与文档不符，优先修实现；只有用户明确要求保留差异时才改文档

---

这份设计文档是 parity 测试体系的首版基线；后续优化优先在“增加案例覆盖”“减少环境噪音”“提升失败可诊断性”三个方向持续推进。
