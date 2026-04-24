package text

import "gobox/cmds/base"

func init() {
	base.Register(base.NewCommand("grep", "Search for patterns in files (regex support)", base.Adapt(GrepCmd)))
	base.Register(base.NewCommand("sed", "Stream editor for filtering and transforming text", base.Adapt(SedCmd)))
	base.Register(base.NewCommand("sort", "Sort lines of text", base.Adapt(SortCmd)))
	base.Register(base.NewCommand("rand", "Generate random bytes/text", base.Adapt(RandCmd)))
	base.Register(base.NewCommand("head", "Print the first lines of a file", base.Adapt(HeadCmd)))
	base.Register(base.NewCommand("tail", "Print the last lines of a file", base.Adapt(TailCmd)))
	base.Register(base.NewCommand("wc", "Print line, word, and byte counts", base.Adapt(WcCmd)))
	base.Register(base.NewCommand("hex", "Hex dump and encode/decode", base.Adapt(HexCmd)))
	base.Register(base.NewCommand("base64", "Base64 encode/decode", base.Adapt(Base64Cmd)))
	base.Register(base.NewCommand("strings", "Extract printable strings", base.Adapt(StringsCmd)))
	base.Register(base.NewCommand("diff", "Compare files line by line", base.Adapt(DiffCmd)))
	base.Register(base.NewCommand("uniq", "Filter adjacent matching lines", base.Adapt(UniqCmd)))
	base.Register(base.NewCommand("seq", "Generate sequences of numbers", base.Adapt(SeqCmd)))
}
