package proc

import "gobox/cmds/base"

func init() {
	base.Register(base.NewCommand("ps", "List processes", base.Adapt(PsCmd)))
	base.Register(base.NewCommand("top", "Live process viewer", base.Adapt(TopCmd)))
	base.Register(base.NewCommand("free", "Show memory usage", base.Adapt(FreeCmd)))
	base.Register(base.NewCommand("xargs", "Build and execute command lines from stdin", base.Adapt(XargsCmd)))
	base.Register(base.NewCommand("kill", "Send signals to processes", base.Adapt(KillCmd)))
	base.Register(base.NewCommand("lsof", "List open files", base.Adapt(LsofCmd)))
	base.Register(base.NewCommand("watch", "Run a command periodically", base.Adapt(WatchCmd)))
	base.Register(base.NewCommand("timeout", "Run a command with a time limit", base.Adapt(TimeoutCmd)))
}
