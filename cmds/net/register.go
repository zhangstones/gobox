package net

import "gobox/cmds/base"

func init() {
	base.Register(base.NewCommand("netstat", "Show network connection status", base.Adapt(NetstatCmd)))
	base.Register(base.NewCommand("ip", "Show network interfaces, routes, and neighbours", base.Adapt(IpCmd)))
	base.Register(base.NewCommand("curl", "Transfer data from a URL", base.Adapt(CurlCmd)))
	base.Register(base.NewCommand("dig", "DNS lookup utility", base.Adapt(DigCmd)))
	base.Register(base.NewCommand("nslookup", "DNS lookup utility", base.Adapt(DigCmd)))
	base.Register(base.NewCommand("nc", "Netcat - arbitrary TCP/UDP connections and listening", base.Adapt(NcCmd)))
	base.Register(base.NewCommand("tw", "Tiny web server for static files or benchmark", base.Adapt(TwCmd)))
	base.Register(base.NewCommand("ifstat", "Network interface statistics monitoring", base.Adapt(IfstatCmd)))
	base.Register(base.NewCommand("np", "Network ping/connectivity tool (TCP/UDP/ICMP/ARP/scan)", base.Adapt(NpCmd)))
}
