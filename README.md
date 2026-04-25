# gobox

`gobox` 是一个单二进制命令行工具集，提供一组常用的 Unix/Linux 风格命令，适合文件处理、文本处理、网络排查和基础系统查看。

它的目标不是完全替代 BusyBox 或 GNU coreutils，而是在容器、精简系统、救援环境等命令缺失的场景下，提供一组够用、直接、可快速上手的常用排障与处置工具。

## 这是什么

- 一个二进制，按子命令分发多个工具
- 覆盖文件系统、文本、网络、进程、磁盘等常见场景
- 常见参数尽量对齐原生命令
- 对部分场景提供 gobox 自有扩展能力

如果你熟悉 `find`、`grep`、`curl`、`ps`、`netstat` 这类工具，`gobox` 的使用方式会比较直接。

## 适用场景

- 希望用一个二进制提供多种常用命令
- 容器、最小系统或工具不完整的环境
- 日常脚本、排障、临时运维操作
- 想明确知道某个命令/参数当前是否已支持，以及支持到什么程度

## 当前命令分类

- 文件系统：`find`、`du`、`df`、`readpath`、`stat`、`truncate`
- 文本处理：`head`、`tail`、`grep`、`sed`、`sort`、`uniq`、`wc`、`seq`、`rand`、`hex`、`base64`、`strings`、`diff`
- 网络：`curl`、`nc`、`netstat`、`tw`、`nslookup/dig`、`ifstat`、`ip`、`np`
- 进程：`ps`、`top`、`free`、`xargs`、`kill`、`lsof`、`watch`、`timeout`
- 磁盘：`iostat`、`ioperf`、`md5sum`、`sha256sum`

这只是命令概览，不展开逐项参数说明。详细能力说明见文末“文档”部分。

## 兼容性说明

`gobox` 的定位是“尽量保持常见用法一致”，但不是所有命令都追求完整复刻原生命令的全部参数和边缘行为。

使用时建议按下面的预期理解：

- 常见、稳定的参数优先支持
- 不同命令的实现深度不同
- 某些能力是 Linux 特定的
- 少量能力属于 gobox 扩展，不一定有一一对应的原生命令

如果你关心某个参数是否支持、是否与原生命令完全一致、还是属于 gobox 扩展，请查看设计文档。

## 构建

```bash
go build -o gobox .
```

## 快速开始

直接调用：

```bash
./gobox <command> [args...]
```

如果已经放进 `PATH`，也可以：

```bash
gobox <command> [args...]
```

少量示例：

```bash
# 查找 1 天前修改过的大文件
./gobox find . -type f -mtime +1 -size +1M

# 查看目录汇总大小
./gobox du -s -h .

# 查看文件系统容量
./gobox df -h .

# 递归搜索文本
./gobox grep -r "TODO" .

# 查看二进制内容
./gobox hex --dump -C data.bin

# 替换文本并输出结果
./gobox sed 's/foo/bar/g' input.txt

# 查看占用较高的进程
./gobox ps --sort cpu -r -n 10

# 全格式查看进程；-f 负责增加列，默认宽度策略仍然生效
./gobox ps -f -n 10

# 需要完整命令行时用 -ww 关闭 ps 默认宽度截断
./gobox ps -f -ww -n 10

# 查看内存概况
./gobox free -h

# 查看监听中的网络连接
./gobox netstat -l -n

# 查看合并后的 netstat 帮助
./gobox netstat --help

# 查看带列说明的 iostat 帮助
./gobox iostat --help

# 查看容器内接口地址
./gobox ip addr

# 启动一个静态文件服务器
./gobox tw -p 8080 -d .

# 请求一个 URL
./gobox curl -I https://example.com
```

## 查看帮助

大多数命令支持 `-h` 或 `--help`：

```bash
./gobox grep -h
./gobox curl --help
./gobox ps -h
```

如果某个帮助输出和你熟悉的原生命令略有差异，以设计文档为准。

## 文档

- 命令设计与兼容性矩阵：`docs/CMD-DESIGN.md`
- parity 测试设计：`docs/TEST-DESIGN.md`
- parity 测试用例矩阵：`docs/TEST-CASES.md`

## 说明

- README 只保留快速说明和常见示例
- 详细命令能力、兼容性状态、扩展项定义，以文档中的命令设计说明为准
- 如果你准备把 `gobox` 用进脚本或生产环境，建议先确认目标命令对应的设计说明
