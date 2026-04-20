# gobox 命令扩展计划

## 状态

本计划中的命令扩展与新增工作已完成，当前进入维护阶段。

已完成全量回归：
- `go test ./...`

已完成 parity 基线首版落地：
- `TEST-DESIGN.md` 设计已补齐
- `TEST-CASES.md` 已覆盖 `docs/CMD-DESIGN.md` 当前条目
- `go test ./...` 已纳入 parity 测试与显式 skip 案例

## 当前实现

`find`, `du`, `ps`, `top`, `iostat`, `ioperf`, `md5sum`, `netstat`, `xargs`, `grep`, `sed`, `head`, `tail`, `curl`, `sort`, `uniq`, `wc`, `nslookup`, `dig`, `nc`, `tw`, `ifstat`, `np`, `rand`, `seq`

---

## 已完成的现有命令补充参数

### grep
- `-A NUM` / `--after-context=NUM`
- `-B NUM` / `--before-context=NUM`
- `-C NUM` / `--context=NUM`
- `--include=PATTERN`
- `--exclude-dir=DIR`
- `-l` / `--files-with-matches`
- `-L` / `--files-without-match`

### ps
- `-ww` 输出不截断宽度
- `-o FIELD1,FIELD2` 自定义输出列（支持 `pid,ppid,cmd,pcpu,pmem,rss,vms,exe`）

### netstat
- `-l` / `--listening` 只显示监听端口
- `-n` / `--numeric` 接受该参数并保持数字地址/端口输出

### find
- `-path PATTERN` 按完整路径匹配
- `-not` / `!` 否定条件

---

## 已完成的新命令与能力

### 1. head / tail ✅ 已完成
- `head`: `-n`, `-c`, `-q`
- `tail`: `-n`, `-f`, `--follow=name`, `--retry`, `-q`, `-s`, `--pid`

### 2. curl ✅ 已完成
- 基础能力：`-s`, `-S`, `-o`, `-O`, `-L`, `-I`, `-w`, `-m`, `-X`, `-H`, `-d`, `-k`, `--connect-timeout`, `--resolve`, `-f`, `-i`
- 上传能力：`-T`, `-F`
- Benchmark 能力：`--bench`, `-c`, `-n`, `--warmup`, `-t`

### 3. sort ✅ 已完成
- `-n`, `-r`, `-k`, `-t`, `-u`, `-M`, `-h`, `-R`, `-c`, `-o`, `-z`

### 4. uniq ✅ 已完成
- `-c`, `-d`, `-u`, `-i`, `-w`, `-f`

### 5. wc ✅ 已完成
- `-l`, `-w`, `-c`, `-m`, `-L`

### 6. nslookup / dig ✅ 已完成
- `nslookup`: `HOST [SERVER]`, `-type`
- `dig`: `-t`, `+short`, `+noall +answer`, `+tcp`, `@DNS_SERVER`

### 7. nc (netcat) ✅ 已完成
- 连接/监听：`host port`, `-l`, `-z`, `-u`, `-w`, `-v`, `-n`, `-4`, `-6`
- Benchmark：`--bench`, `-c`, `-n`, `-s`, `-t`, `-i`

### 8. tw (tinyweb) ✅ 已完成
- `-p`, `-d`, `-r`, `--bench`, `-h`
- Benchmark 端点：`GET /ping`, `POST /ping`, `POST /upload`

### 9. ifstat ✅ 已完成
- `-i`, `-p`, `-n`, `-a`, `-A`, `-e`, `-d`

### 10. ioperf ✅ 已完成
- `--rw`, `--rwmixread`, `--filename`, `--bs`, `--size`, `--numjobs`, `--iodepth`, `--direct`, `--fsync`, `--sync`, `--rate`, `--time_based`, `--runtime`, `--group_reporting`, `--percentile`, `--latency`

### 11. md5sum ✅ 已完成
- 默认计算模式
- `-c`, `--check`, `--tag`, `-q`, `--quiet`, `-s`, `--status`, `-w`, `--warn`

### 12. rand ✅ 已完成
- `NUM`, `-n`, `-hex`, `-base64`, `-out`

### 13. seq ✅ 已完成
- `[FIRST [INC]] LAST`, `-f`, `-s`, `-w`

### 14. np (netping) ✅ 已完成
- 模式：`--tcp`, `--udp`, `--icmp`, `--arp`, `--scan`
- 通用参数：`-c`, `-i`, `-p`, `-s`, `-I`, `-W`, `--flood`, `-w`
- 长连接模式：`-l`
- 输出参数：`-q`, `-v`

---

## 质量状态

已满足：
- 每个主要命令具备对应测试覆盖
- 已补充常用参数组合回归
- 已通过集成/冒烟测试与全量测试

---

## 备注

当前若需继续推进，建议以“新增命令”或“增强现有命令兼容性”为新计划，不再沿用本文件中的待实现状态。
