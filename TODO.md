# 命令帮助信息（--help）审查待办

审查范围：全部 40 个子命令的 `--help` 输出，对照 `docs/CMD-SPECS.md`（唯一规格源）与实际实现逐一核对，并检查各命令帮助文本的风格一致性。

---

## A. `--help` 有、但 `CMD-SPECS.md` 未收录的真实参数

- [x] `find`：`-path PATTERN`、`-not` 均已实现（`cmds/fs/cmd_find.go:21-22`）且在 `--help` 里有说明，`CMD-SPECS.md` 的 find 表格缺这两行 — 已补齐两行
- [x] `xargs`：`-v`（"legacy alias for -t"）在 `--help` 和 `docs/TEST-CASES.md`（`XARGS-008`）都有，`CMD-SPECS.md` 的 xargs 表格没有对应行 — 已补齐
- [x] `ioperf`：`--latency`、`--percentile`（单值别名）在 `--help` 里列出且是真实 flag，`CMD-SPECS.md` 只写了 `--percentile_list`，缺这两个 — 已补齐两行
- [x] `hex -o FILE`：`CMD-SPECS.md` 只在 `--decode` 场景提到 `-o`，但实现（`cmds/text/cmd_hex.go:74`）里 `-o` 对 `--dump`/`--encode` 同样生效，规格描述范围比实现窄 — 已改写为覆盖三种模式

## B. help 文本自相矛盾（真 bug）

- [x] `sort`（`cmds/text/cmd_sort.go:465,470`）：`--help` 同时把 `-h` 标为 `--human-numeric-sort` 的简写，又写了一行 `-h, --help`；但解析逻辑（`cmd_sort.go:74`）里 `-h` 已被 `--human-numeric-sort` 占用，真正能触发帮助的只有长选项 `--help`。第 470 行的 `-h,` 前缀是错的，应改成只写 `--help` — 已改为 `      --help`

## C. help 缺少已支持的用法说明

- [x] `lsof`：`--help` 只写了一行 `-i  show network files`，未提到 `-iTCP`/`-iUDP`/`-i :PORT` 这几种后缀写法（`CMD-SPECS.md` 和 parity 测试 LSOF-005/006/007 均已明确支持） — 已补齐三行

## D. 风格不统一

- [x] `dig`/`nslookup`：Usage 行写死成 `Usage: dig [@DNS_SERVER]...`，是全部命令里唯一不带 `gobox` 前缀的；且不管 `gobox dig` 还是 `gobox nslookup` 调用，帮助文本和示例永远显示裸的 `dig ...`，从未出现 `nslookup` — 已拆出 `NslookupCmd`，help/错误信息按调用名动态显示（`gobox dig` / `gobox nslookup`）
- [x] `ip`：整份帮助只有一行（`Usage: gobox ip [-o] addr | [-s] link | route | neigh`），无 Options/Examples 段，是所有命令里最简短的 — 已扩展为 Objects/Options/Examples 结构
- [x] `alias`：同样只有 3 行，且 `-u` 具体做什么没有在正文里说明，只出现在 usage 括号里 — 已补充 Options/Examples
- [x] Usage 括号写法三种混用：`[OPTION]...`（约 20 个命令）、`[OPTIONS]`（curl/head/nc/rand/sed/sort/tail/tw）、`[OPTION]` 单数无省略号（仅 `timeout`），未统一约定 — 已统一为 `[OPTION]...`
- [x] `netstat`：Usage 行完全没有参数占位符（只写 `Usage: gobox netstat`），是唯一一个 flag 很多但 Usage 行不体现的命令 — 已改为 `Usage: gobox netstat [OPTION]...`
- [x] 是否在自己的 `--help` 里主动提示 `-h`/`--help`：仅 `dig/nslookup/head/rand/sed/seq/sort/tail/uniq/wc` 这 10 个命令会写，其余 30 个（`du/df/find/grep/curl/netstat/ps/...`）完全不提，但实际上全部命令都支持 `--help`——影响范围最大的一处不统一 — 已给全部 30 个命令补上对应行（`-h` 已被占用的 `du`/`df`/`free` 只写 `--help`）
- [x] `md5sum` vs `sha256sum`：参数几乎完全相同（`-c/--check`、`--tag`、`-q/--quiet`、`-s/--status`、`-w/--warn`），一个用扁平的 `Options:`，另一个拆成 `Modes:`/`Output:` 两段；描述语也分别叫 "MD5 checksums" 和 "SHA256 message digests"，用词不统一 — 已把 md5sum 改成同 sha256sum 一致的 Modes/Output 结构与措辞

---

全部 13 条已修复并通过 `go build`/`go vet`/`go test ./cmds/...`/`go test ./tests/smoke`/`go test ./tests/parity`（详见提交记录）。
