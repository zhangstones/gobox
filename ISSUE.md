# Parity Audit Issues

## Resolved

- [x] `FIND-003` / `FIND-004` / `FIND-005` / `FIND-006` / `FIND-008` / `FIND-009`
  - Problem: `TEST-CASES.md` 标记为 `exact`，但 `tests/parity/fs_parity_test.go` 里只做了 gobox 单边断言，没有和 native `find` 做严格对比。
  - Fix: 改为统一走 `runExactParityCases`，并对 `find` 输出做路径归一化；同时把 `FIND-005` 调整为 native `find` 可稳定支持的 `-mtime +1` 夹具。

- [x] `DU-001` / `DU-002`
  - Problem: `TEST-CASES.md` 标记为 `structured`，但实现只检查 gobox 输出是否包含路径，没有与 native `du` 做结构对照。
  - Fix: 增加 gobox/native 双边执行，按路径集合做 structured 对比。

- [x] `MD5-005`
  - Problem: `TEST-CASES.md` 标记为 `exact`，但实现只校验 gobox 报错，不校验 native `md5sum --warn --check` 的退出码和警告行为。
  - Fix: 增加 native 对照，校验双方均失败且都输出 malformed warning。

- [x] `CURL-002` / `CURL-003` / `CURL-004` / `CURL-008` / `CURL-012` / `CURL-013` / `CURL-014` / `CURL-016`
  - Problem: `TEST-CASES.md` 标记为 `behavior` 且给了 native baseline，但实现仍偏向 gobox-only contract，缺少 native 侧语义对照。
  - Fix: 增加 native `curl` 对照，分别校验失败语义、文件输出、TLS 忽略证书、连接超时、`--resolve`、`-i` 等行为。

- [x] `IFSTAT-001` / `IFSTAT-002` / `IFSTAT-005` / `IFSTAT-006` / `IFSTAT-007`
  - Problem: `TEST-CASES.md` 原先把这些 case 标成 `structured/behavior + native ifstat baseline`，但常见 native `ifstat` 变体并不支持 gobox 的这些参数语义，Mode 与当前测试方式不一致。
  - Fix: 将这些 case 调整为 `contract + gobox-only`，并把测试断言收紧到接口集合、单接口过滤、样本数、采样间隔等 gobox 可稳定保证的契约。
