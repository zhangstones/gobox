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
   - 必须证明“参数真的改变了行为”；不能退化成只检查 help、header、关键字存在
   - 对组合参数，必须至少覆盖一条“组合后输出与基线不同”的案例，避免 no-op 参数被误判为已支持

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

先写 `docs/TEST-CASES.md`，再写测试代码，最后修实现；避免“已经写了很多测试，但不知道覆盖了哪些文档条目”。

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

额外要求：

- 不能退化成“只检查某个字段名或表头关键字存在”
- 至少要验证以下之一：
  - 过滤后的结果集语义
  - 列位置或列集合变化
  - 排序键单调性
  - 受控目标记录是否保留/排除
- 对 `ps`、`df`、`netstat`、`ifstat`、`iostat` 这类命令，优先比较“目标语义”而不是全量文本外观

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
- 与“未加该参数”或“native 对应参数”相比，输出集合、过滤结果或执行路径必须有可观察差异

禁止事项：

- 不能仅以 “输出包含某个表头/关键字” 作为 behavioral parity 的唯一断言
- 不能把“参数可解析且命令成功”当作参数已生效
- Normalize 不能抹平参数应当造成的语义差异
- `base != variant` 只能作为辅助证据，不能单独作为参数生效证明

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

额外要求：

- Normalize 只能消除非关键噪音，不能消除参数语义差异
- 如果 Normalize 之后一个原本有差异的 case 变成“看起来一致”，必须重新检查是否把真实行为差异抹掉了
- 对过滤、排序、格式切换类参数，优先先做结构化断言，再决定是否需要 Normalize 文本

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

## 10. 弱断言与无效 parity 的禁止规则

以下写法默认视为弱 case，不能作为主断言单独存在：

- 仅断言 `stdout`/`stderr` 包含某个关键字
- 仅断言输出包含某个表头、字段名或 help 文案
- 仅断言命令 `ExitCode == 0`
- 仅断言参数可解析
- 仅断言输出非空
- 仅断言输出与 baseline 不同，但没有解释“为什么不同”

以下写法默认视为无效 parity：

- 使用 Normalize 抹掉本应由参数引入的语义差异
- 用“命令成功执行”代替“参数真的生效”
- 用“组合参数整体能跑通”代替“每个子参数都生效”

强制要求：

- 每个 parity case 至少要回答两个问题：
  1. 参数是否被正确解析
  2. 参数是否改变了目标语义
- 如果一个 case 只回答了第 1 个问题，该 case 不能视为覆盖完成

## 11. 参数生效证明规则

每个参数 case 都必须明确其“被测语义位”，不要把多个独立语义混成一个模糊断言。

按参数类型，最低证据标准如下：

1. 过滤参数
   - 必须证明结果集被收窄、扩展或重定向
   - 必须证明保留下来的每一行都满足过滤条件
   - 适用示例：`ps -p`、`netstat -t`、`lsof -iTCP`

2. 排序参数
   - 必须提取排序列并验证单调性
   - 不能只验证 header 中出现排序字段名
   - 对 `%CPU`、`%MEM` 等显示列，应按真实比较值验证，而不是按格式化文本做脆弱比较

3. 格式参数
   - 必须验证列集合、列顺序、列含义或输出模式变化
   - 不能只验证“多了一列名字”
   - 适用示例：`ps -f`、`ps -F`、`df -T`

4. 范围/数量参数
   - 必须验证输出条数、轮次、采样次数、迭代上限或窗口范围
   - 适用示例：`head -n`、`tail -n`、`top -n`

5. 行为参数
   - 必须验证请求、连接、重试、超时、跟随、重定向、并发或副作用语义
   - 不能只验证 summary 文案存在
   - 适用示例：`curl --bench`、`nc`、`tail -f`

6. 兼容 no-op 参数
   - 只有在设计明确允许时，才可以把参数视为兼容 no-op
   - 测试必须明确说明为什么允许 no-op
   - 同时必须验证该参数没有破坏原有输出契约

## 12. 断言方式选择顺序

新增或修改 parity case 时，应按以下优先级选择断言：

1. 能比较行为，就不要只比较文本
2. 能比较结构，就不要只比较关键字
3. 能比较字段，就不要只比较整行
4. 能比较结果集，就不要只比较 header
5. 只有在输出天然非结构化、且关键词本身就是契约的一部分时，才允许关键词断言作为主断言

可将证据强度分成三档：

- 强证据
  - 行集合
  - 过滤后的全量行约束
  - 排序单调性
  - 列索引后的字段值
  - 可观察行为副作用

- 中证据
  - 指定 section 的结构
  - 固定前缀行
  - 行数变化
  - 分组或摘要块出现/消失

- 弱证据
  - 任意位置子串存在
  - help 文本提到参数
  - stdout 非空
  - baseline 不同但未解释原因

规则：

- 新增 parity case 时，主断言不得只使用弱证据
- 若只能使用弱证据，必须在 case 注释中写明原因，并补一个中证据或强证据断言

## 13. 环境敏感命令的稳定性约束

以下命令受宿主机、容器、发行版、内核或运行时背景噪音影响较大：

- `ps`
- `top`
- `df`
- `netstat`
- `ifstat`
- `iostat`
- 以及其它读取系统全局状态的命令

对这类命令，禁止默认使用“全量逐行完全一致”作为成功标准。

应优先遵循以下规则：

- 不比较全局全集，优先比较受控夹具和目标记录
- 不要求 mount set、process set、route set、interface set 在所有环境完全相等
- 不把 native 某次运行的偶然文本形状直接上升为规范
- 将 native 视为语义参考，而不是所有场景下的文本真值源

当 native 输出本身受环境影响较大时，case 应退化为以下更稳定的比较：

- gobox 自身契约是否满足
- native 关键语义是否存在
- gobox 与 native 是否满足合理的交集、子集或方向性关系

## 14. 组合参数、别名参数与聚合参数规则

长短参数、组合参数和聚合参数不能只测“整体能跑通”。

强制要求：

1. 长短参数别名
   - 必须至少有一条输出等价 case
   - 示例：`--route` 与 `-r`

2. 组合短参数
   - 必须至少有一条“组合形式 == 拆分形式”的等价 case
   - 示例：`-tnlp` == `-t -n -l -p`

3. 组合语义
   - 不能因为组合形式通过，就认定每个子参数都已支持
   - 至少还要有单参数或局部组合 case，证明关键子参数真的生效

4. 聚合风格参数
   - 对 BSD 风格聚合参数，如 `ps aux`，必须拆开验证每个字母的独立语义
   - 不允许把整个 token 当作一个“魔法开关”处理并仅做黑盒通过验证

## 15. 文档—测试—实现一致性规则

三者必须闭环：

- `docs/CMD-DESIGN.md` 描述支持什么
- `docs/TEST-CASES.md` 说明如何覆盖
- 测试代码证明实现真的满足该语义

强制要求：

- `CMD-DESIGN` 标记为“支持”的参数，`TEST-CASES` 必须有映射
- `TEST-CASES` 中的 case，必须能在本设计文档中找到合格的比较方式
- 如果某个 case 只能证明“参数可解析”，则 `CMD-DESIGN` 不能把它表述成“已完整支持”
- 新增参数支持时，应同步更新：
  - `docs/CMD-DESIGN.md`
  - `docs/TEST-CASES.md`
  - 对应 unit/parity/smoke 测试
- 修复 parity 弱 case 时，如果暴露出文档承诺过度，应先增强测试，再决定修实现还是收缩文档承诺

## 16. Parity Case Review Checklist

每个新增或修改的 parity case，在评审时至少检查以下问题：

- 这个 case 测的是哪个参数语义位
- 主断言属于强证据、中证据还是弱证据
- 是否只靠 `Contains` 或 help 文案支撑
- 是否证明了“参数真的生效”
- 是否依赖宿主机不稳定的全局状态
- 是否验证了受控夹具，而不是随机背景噪音
- baseline 的意义是否清晰
- 长短参数、组合参数、聚合参数是否被拆清楚
- Normalize 是否可能掩盖真实差异
- 失败输出是否足够定位问题

评审默认规则：

- 若一个新 case 的主断言只有一条 `Contains`，默认不通过 review
- 若一个 case 同时存在“过弱主断言”和“无说明的重 Normalize”，默认不通过 review
- 若一个 case 证明不了参数语义，只证明命令成功运行，默认不计入覆盖完成度

## 17. `CMD-DESIGN` 驱动策略

`docs/TEST-CASES.md` 作为 `docs/CMD-DESIGN.md` 的测试映射表，要求：

1. 每个命令每个参数至少 1 个案例
2. 每个案例必须标注：
   - 比较类型
   - 原生命令映射
   - 输入夹具
   - 核心断言
3. `✅ 一致` 条目优先进入 native parity automation
4. `⚠️ 部分一致` 条目必须有“差异边界案例”
5. `🆕 gobox扩展` 条目必须有 contract case
6. 测试代码中的 `Case ID` 必须能直接追溯到 `docs/TEST-CASES.md`

## 18. 落地阶段与提交节奏

### 阶段 1：设计与案例矩阵

产出：

- `docs/TEST-DESIGN.md`
- `docs/TEST-CASES.md`

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

## 19. 失败输出要求

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

## 20. CI 与回归约束

建议将 parity 测试纳入以下执行层次：

1. 本地开发：`go test ./tests/parity/...` 或带 `-run` 过滤
2. 提交前：命令相关包测试 + parity 定向测试
3. 全量回归：`go test ./...`

若某类 parity 测试存在环境依赖，应通过 `t.Skip()` 控制，而不是从默认回归中摘除。

## 21. 当前仓库落地约定

结合当前仓库状态，初版落地采用以下约定：

1. 使用函数接口直接调用命令实现，不构建临时 `gobox` 二进制
2. 将 parity helpers 与 case 文件集中到 `tests/parity/`
3. 允许保留少量 root 级 parity 测试作为迁移过渡，但最终以 `tests/parity/` 为主
4. 优先让 `docs/CMD-DESIGN.md` 中所有命令参数都能在 `docs/TEST-CASES.md` 找到映射
5. 所有 parity 测试最终都纳入 `go test ./...` 可执行范围

## 22. 完成标准

认为 parity 测试体系“完成首版落地”，至少需要满足：

1. `docs/TEST-DESIGN.md` 明确说明四类测试模型、执行器、skip 策略与目录结构
2. `docs/TEST-CASES.md` 覆盖 `docs/CMD-DESIGN.md` 当前所有命令参数
3. 自动化测试代码中已落地全部 `Case ID`，或对环境依赖项显式 `Skip`
4. `go test ./...` 通过
5. 若发现实现与文档不符，优先修实现；只有用户明确要求保留差异时才改文档

---

这份设计文档是 parity 测试体系的首版基线；后续优化优先在“增加案例覆盖”“减少环境噪音”“提升失败可诊断性”三个方向持续推进。
