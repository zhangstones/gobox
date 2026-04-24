package fs

import "gobox/cmds/base"

func init() {
	base.Register(base.NewCommand("find", "Search for files in a directory tree", base.Adapt(FindCmd)))
	base.Register(base.NewCommand("du", "Show file/directory disk usage", base.Adapt(DuCmd)))
	base.Register(base.NewCommand("df", "Show filesystem usage", base.Adapt(DfCmd)))
	base.Register(base.NewCommand("readpath", "Resolve paths and symlinks", base.Adapt(ReadpathCmd)))
	base.Register(base.NewCommand("stat", "Show file or filesystem status", base.Adapt(StatCmd)))
	base.Register(base.NewCommand("truncate", "Shrink or extend file size", base.Adapt(TruncateCmd)))
}
