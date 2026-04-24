package disk

import "gobox/cmds/base"

func init() {
	base.Register(base.NewCommand("iostat", "Show block device I/O stats (Linux cgroup/blkio)", base.Adapt(IostatCmd)))
	base.Register(base.NewCommand("ioperf", "I/O performance benchmark tool (simplified fio-like)", base.Adapt(IoperfCmd)))
	base.Register(base.NewCommand("md5sum", "Compute/check MD5 checksums", base.Adapt(Md5sumCmd)))
	base.Register(base.NewCommand("sha256sum", "Compute/check SHA-256 checksums", base.Adapt(Sha256sumCmd)))
}
