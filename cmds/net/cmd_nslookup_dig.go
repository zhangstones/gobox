package net

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"gobox/cmds/utils"
)

// digDefaultTTL is used to render the TTL column of dig's answer lines.
// gobox resolves records through Go's stdlib net.Resolver, which does not
// expose the real TTL from the DNS response, so a fixed value is used
// instead of omitting the column entirely (real dig's answer format is
// NAME TTL CLASS TYPE DATA; omitting TTL breaks tooling that parses that
// fixed field layout).
const digDefaultTTL = 300

// cnameIsSelf reports whether cname is just the canonicalized form of host
// itself. Go's net.Resolver.LookupCNAME never errors when a host has no
// CNAME record -- per its documented contract, it instead returns the host's
// own canonical name in that case. Without this check, a domain with no
// CNAME record at all would be reported as having a self-referential CNAME,
// which real dig/nslookup never show.
func cnameIsSelf(host, cname string) bool {
	norm := func(s string) string {
		return strings.ToLower(strings.TrimSuffix(s, "."))
	}
	return norm(cname) == norm(host)
}

// DigCmd implements dig functionality
func DigCmd(args []string) error {
	return runDNSLookup("dig", args)
}

// NslookupCmd runs the same DNS lookup logic as DigCmd, but with help text
// and error messages that reflect the "nslookup" invocation name.
func NslookupCmd(args []string) error {
	return runDNSLookup("nslookup", args)
}

func runDNSLookup(progName string, args []string) error {
	var host string
	var dnsServer string
	var queryType string
	var shortOutput bool
	var showAnswer bool
	var useTCP bool
	var noall bool

	// Parse dig arguments
	// dig [@DNS_SERVER] HOST [DNS_SERVER] [-t TYPE] [--type=TYPE] [OPTIONS]
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			digUsage(os.Stdout, progName)
			return nil
		case arg == "+short":
			shortOutput = true
		case arg == "+noall":
			noall = true
		case arg == "+answer":
			showAnswer = true
		case arg == "+tcp":
			useTCP = true
		case strings.HasPrefix(arg, "@") && len(arg) > 1:
			dnsServer = arg[1:]
		case arg == "-t" || arg == "--type":
			if i+1 >= len(args) || utils.LooksLikeFlag(args[i+1]) {
				return fmt.Errorf("%s requires an argument", arg)
			}
			i++
			queryType = args[i]
		case strings.HasPrefix(arg, "-t") && len(arg) > 2:
			queryType = arg[2:]
		case strings.HasPrefix(arg, "--type=") && len(arg) > 7:
			queryType = arg[7:]
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			if host == "" {
				host = arg
			} else if dnsServer == "" && !strings.HasPrefix(arg, "@") {
				// Treat second positional as DNS server
				dnsServer = arg
			}
		}
		i++
	}

	if host == "" {
		fmt.Fprintf(os.Stderr, "%s: missing host argument\n", progName)
		digUsage(os.Stderr, progName)
		return fmt.Errorf("host required")
	}

	// Default query type is A if not specified
	if queryType == "" {
		queryType = "A"
	}
	if !isSupportedDNSQueryType(queryType) {
		fmt.Fprintf(os.Stderr, "Warning, ignoring invalid type %s\n", strings.ToUpper(queryType))
		queryType = "A"
	}

	// Default DNS server
	if dnsServer == "" {
		dnsServer = "8.8.8.8"
	}

	// If +short, just show the answer
	if shortOutput {
		return digShortOutput(host, queryType, dnsServer, useTCP)
	}

	// If +noall +answer, show only answer section
	if noall && showAnswer {
		return digAnswerOnly(host, queryType, dnsServer, useTCP)
	}

	// Full dig output
	return digFullOutput(host, queryType, dnsServer, useTCP)
}

// isSupportedDNSQueryType reports whether typ is one of the record types
// gobox actually knows how to query. Anything else (e.g. a typo like
// "BOGUS") must not be silently queried and mislabeled as if it were valid.
func isSupportedDNSQueryType(typ string) bool {
	switch strings.ToUpper(typ) {
	case "A", "AAAA", "TXT", "CNAME", "NS", "MX", "SRV", "PTR":
		return true
	default:
		return false
	}
}

func digUsage(w io.Writer, progName string) {
	fmt.Fprintf(w, "Usage: gobox %s [@DNS_SERVER] HOST [DNS_SERVER] [OPTIONS]\n", progName)
	fmt.Fprintln(w, "DNS lookup utility")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  @DNS_SERVER        Use specified DNS server")
	fmt.Fprintln(w, "  -t TYPE, --type=TYPE   Specify query type (A/AAAA/TXT/CNAME/NS/MX/SRV)")
	fmt.Fprintln(w, "  +short            Show short output (just the answer)")
	fmt.Fprintln(w, "  +noall +answer    Show only the answer section")
	fmt.Fprintln(w, "  +tcp              Use TCP instead of UDP")
	fmt.Fprintln(w, "  -h, --help        Show this help message")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintf(w, "  gobox %s example.com\n", progName)
	fmt.Fprintf(w, "  gobox %s @8.8.8.8 example.com\n", progName)
	fmt.Fprintf(w, "  gobox %s -t MX example.com\n", progName)
	fmt.Fprintf(w, "  gobox %s +short example.com\n", progName)
	fmt.Fprintf(w, "  gobox %s +noall +answer example.com\n", progName)
}

// dnsQueryTimeout bounds how long a single DNS exchange may wait for a
// response. Without it, a query against an unreachable/silent server (e.g.
// UDP to a dead host, where dial succeeds instantly but no reply ever
// arrives) blocks forever, since net.Resolver honors no deadline of its own.
// GNU dig gives up after ~15s (5s timeout x 3 tries by default); this
// mirrors that upper bound with a single bounded context.
const dnsQueryTimeout = 15 * time.Second

func dnsQueryContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), dnsQueryTimeout)
}

func doDNSQuery(host, queryType, dnsServer string) error {
	return doDNSQueryWithResolver(host, queryType, newResolver(dnsServer, false))
}

func newResolver(dnsServer string, useTCP bool) *net.Resolver {
	network := "udp"
	if useTCP {
		network = "tcp"
	}
	dnsAddress := dnsServer
	if _, _, err := net.SplitHostPort(dnsServer); err != nil {
		dnsAddress = net.JoinHostPort(dnsServer, "53")
	}
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, networkArg, address string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(ctx, network, dnsAddress)
		},
	}
}

func doDNSQueryWithResolver(host, queryType string, resolver *net.Resolver) error {
	queryType = strings.ToUpper(queryType)

	switch queryType {
	case "A":
		return lookupA(host, resolver)
	case "AAAA":
		return lookupAAAA(host, resolver)
	case "TXT":
		return lookupTXT(host, resolver)
	case "CNAME":
		return lookupCNAME(host, resolver)
	case "NS":
		return lookupNS(host, resolver)
	case "MX":
		return lookupMX(host, resolver)
	case "SRV":
		return lookupSRV(host, resolver)
	case "PTR":
		return lookupPTR(host, resolver)
	default:
		// Default to A lookup
		return lookupA(host, resolver)
	}
}

func lookupA(host string, resolver *net.Resolver) error {
	ctx, cancel := dnsQueryContext()
	defer cancel()
	ips, err := resolver.LookupHost(ctx, host)
	if err != nil {
		// Check if it's a DNS error or no such host
		if _, ok := err.(*net.DNSError); ok {
			fmt.Printf("** server can't find %s: NXDOMAIN\n", host)
		}
		return fmt.Errorf("lookup failed: %w", err)
	}
	hadV4 := false
	for _, ip := range ips {
		// Only show IPv4 addresses for A records
		if net.ParseIP(ip).To4() != nil {
			hadV4 = true
			fmt.Printf("Name:   %s\nAddress: %s\n\n", host, ip)
		}
	}
	if !hadV4 {
		fmt.Printf("** server can't find %s: NXDOMAIN\n", host)
		return errors.New("no A records found")
	}
	return nil
}

func lookupAAAA(host string, resolver *net.Resolver) error {
	ctx, cancel := dnsQueryContext()
	defer cancel()
	addrs, err := resolver.LookupHost(ctx, host)
	if err != nil {
		if _, ok := err.(*net.DNSError); ok {
			fmt.Printf("** server can't find %s: NXDOMAIN\n", host)
		}
		return fmt.Errorf("lookup failed: %w", err)
	}
	fmt.Printf("Name:   %s\n", host)
	hadV6 := false
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip != nil && ip.To4() == nil {
			hadV6 = true
			fmt.Printf("Address: %s\n", addr)
		}
	}
	fmt.Println()
	if !hadV6 {
		fmt.Printf("** server can't find %s: NXDOMAIN\n", host)
		return errors.New("no AAAA records found")
	}
	return nil
}

func lookupTXT(host string, resolver *net.Resolver) error {
	ctx, cancel := dnsQueryContext()
	defer cancel()
	txts, err := resolver.LookupTXT(ctx, host)
	if err != nil {
		if _, ok := err.(*net.DNSError); ok {
			fmt.Printf("** server can't find %s: NXDOMAIN\n", host)
		}
		return fmt.Errorf("lookup failed: %w", err)
	}
	fmt.Printf("Name:   %s\n", host)
	for _, txt := range txts {
		fmt.Printf("TXT:    \"%s\"\n", txt)
	}
	fmt.Println()
	return nil
}

func lookupCNAME(host string, resolver *net.Resolver) error {
	ctx, cancel := dnsQueryContext()
	defer cancel()
	cname, err := resolver.LookupCNAME(ctx, host)
	if err != nil {
		if _, ok := err.(*net.DNSError); ok {
			fmt.Printf("** server can't find %s: NXDOMAIN\n", host)
		}
		return fmt.Errorf("lookup failed: %w", err)
	}
	fmt.Printf("Name:   %s\n", host)
	if cnameIsSelf(host, cname) {
		fmt.Println()
		return nil
	}
	fmt.Printf("Canonical name: %s\n\n", cname)
	return nil
}

func lookupNS(host string, resolver *net.Resolver) error {
	ctx, cancel := dnsQueryContext()
	defer cancel()
	nameservers, err := resolver.LookupNS(ctx, host)
	if err != nil {
		if _, ok := err.(*net.DNSError); ok {
			fmt.Printf("** server can't find %s: NXDOMAIN\n", host)
		}
		return fmt.Errorf("lookup failed: %w", err)
	}
	fmt.Printf("Name:   %s\n", host)
	for _, ns := range nameservers {
		fmt.Printf("Nameserver: %s\n", ns.Host)
	}
	fmt.Println()
	return nil
}

func lookupMX(host string, resolver *net.Resolver) error {
	ctx, cancel := dnsQueryContext()
	defer cancel()
	mxs, err := resolver.LookupMX(ctx, host)
	if err != nil {
		if _, ok := err.(*net.DNSError); ok {
			fmt.Printf("** server can't find %s: NXDOMAIN\n", host)
		}
		return fmt.Errorf("lookup failed: %w", err)
	}
	fmt.Printf("Name:   %s\n", host)
	for _, mx := range mxs {
		fmt.Printf("Mail exchanger: %d %s\n", mx.Pref, mx.Host)
	}
	fmt.Println()
	return nil
}

func lookupSRV(host string, resolver *net.Resolver) error {
	// SRV record format: _service._proto.name
	// Try to parse and lookup SRV record
	ctx, cancel := dnsQueryContext()
	defer cancel()
	_, addrs, err := resolver.LookupSRV(ctx, "", "", host)
	if err != nil {
		if _, ok := err.(*net.DNSError); ok {
			fmt.Printf("** server can't find %s: NXDOMAIN\n", host)
		}
		return fmt.Errorf("lookup failed: %w", err)
	}
	fmt.Printf("Name:   %s\n", host)
	for _, srv := range addrs {
		fmt.Printf("SRV:    %d %d %d %s\n", srv.Priority, srv.Weight, srv.Port, srv.Target)
	}
	fmt.Println()
	return nil
}

func lookupPTR(host string, resolver *net.Resolver) error {
	// Reverse lookup
	ctx, cancel := dnsQueryContext()
	defer cancel()
	names, err := resolver.LookupAddr(ctx, host)
	if err != nil {
		if _, ok := err.(*net.DNSError); ok {
			fmt.Printf("** server can't find %s: NXDOMAIN\n", host)
		}
		return fmt.Errorf("lookup failed: %w", err)
	}
	for _, name := range names {
		fmt.Printf("%s\n", name)
	}
	return nil
}

func digShortOutput(host, queryType, dnsServer string, useTCP bool) error {
	queryType = strings.ToUpper(queryType)
	resolver := newResolver(dnsServer, useTCP)
	ctx, cancel := dnsQueryContext()
	defer cancel()

	switch queryType {
	case "A":
		ips, err := resolver.LookupHost(ctx, host)
		if err != nil {
			return nil
		}
		for _, ip := range ips {
			if net.ParseIP(ip).To4() != nil {
				fmt.Println(ip)
			}
		}
	case "AAAA":
		addrs, err := resolver.LookupHost(ctx, host)
		if err != nil {
			return nil
		}
		for _, addr := range addrs {
			ip := net.ParseIP(addr)
			if ip != nil && ip.To4() == nil {
				fmt.Println(addr)
			}
		}
	case "TXT":
		txts, err := resolver.LookupTXT(ctx, host)
		if err != nil {
			return nil
		}
		for _, txt := range txts {
			fmt.Println(txt)
		}
	case "CNAME":
		cname, err := resolver.LookupCNAME(ctx, host)
		if err != nil || cnameIsSelf(host, cname) {
			return nil
		}
		fmt.Println(cname)
	case "NS":
		nss, err := resolver.LookupNS(ctx, host)
		if err != nil {
			return nil
		}
		for _, ns := range nss {
			fmt.Println(ns.Host)
		}
	case "MX":
		mxs, err := resolver.LookupMX(ctx, host)
		if err != nil {
			return nil
		}
		for _, mx := range mxs {
			fmt.Printf("%d %s\n", mx.Pref, mx.Host)
		}
	case "SRV":
		_, addrs, err := resolver.LookupSRV(ctx, "", "", host)
		if err != nil {
			return nil
		}
		for _, srv := range addrs {
			fmt.Printf("%d %d %d %s\n", srv.Priority, srv.Weight, srv.Port, srv.Target)
		}
	default:
		ips, err := resolver.LookupHost(ctx, host)
		if err != nil {
			return nil
		}
		for _, ip := range ips {
			fmt.Println(ip)
		}
	}

	return nil
}

func digAnswerOnly(host, queryType, dnsServer string, useTCP bool) error {
	queryType = strings.ToUpper(queryType)
	resolver := newResolver(dnsServer, useTCP)
	ctx, cancel := dnsQueryContext()
	defer cancel()

	fmt.Printf(";; ANSWER SECTION:\n")
	switch queryType {
	case "A":
		ips, err := resolver.LookupHost(ctx, host)
		if err != nil {
			fmt.Printf("%s. IN A\n", host)
			return nil
		}
		for _, ip := range ips {
			if net.ParseIP(ip).To4() != nil {
				fmt.Printf("%s.\t\t%d\tIN\tA\t%s\n", host, digDefaultTTL, ip)
			}
		}
	case "AAAA":
		addrs, err := resolver.LookupHost(ctx, host)
		if err != nil {
			fmt.Printf("%s. IN AAAA\n", host)
			return nil
		}
		for _, addr := range addrs {
			ip := net.ParseIP(addr)
			if ip != nil && ip.To4() == nil {
				fmt.Printf("%s.\t\t%d\tIN\tAAAA\t%s\n", host, digDefaultTTL, addr)
			}
		}
	case "TXT":
		txts, err := resolver.LookupTXT(ctx, host)
		if err != nil {
			fmt.Printf("%s. IN TXT\n", host)
			return nil
		}
		for _, txt := range txts {
			fmt.Printf("%s.\t\t%d\tIN\tTXT\t\"%s\"\n", host, digDefaultTTL, txt)
		}
	case "CNAME":
		cname, err := resolver.LookupCNAME(ctx, host)
		if err != nil || cnameIsSelf(host, cname) {
			fmt.Printf("%s. IN CNAME\n", host)
			return nil
		}
		fmt.Printf("%s.\t\t%d\tIN\tCNAME\t%s\n", host, digDefaultTTL, cname)
	case "NS":
		nss, err := resolver.LookupNS(ctx, host)
		if err != nil {
			fmt.Printf("%s. IN NS\n", host)
			return nil
		}
		for _, ns := range nss {
			fmt.Printf("%s.\t\t%d\tIN\tNS\t%s\n", host, digDefaultTTL, ns.Host)
		}
	case "MX":
		mxs, err := resolver.LookupMX(ctx, host)
		if err != nil {
			fmt.Printf("%s. IN MX\n", host)
			return nil
		}
		for _, mx := range mxs {
			fmt.Printf("%s.\t\t%d\tIN\tMX\t%d %s\n", host, digDefaultTTL, mx.Pref, mx.Host)
		}
	case "SRV":
		_, addrs, err := resolver.LookupSRV(ctx, "", "", host)
		if err != nil {
			fmt.Printf("%s. IN SRV\n", host)
			return nil
		}
		for _, srv := range addrs {
			fmt.Printf("%s.\t\t%d\tIN\tSRV\t%d %d %d %s\n", host, digDefaultTTL, srv.Priority, srv.Weight, srv.Port, srv.Target)
		}
	default:
		ips, err := resolver.LookupHost(ctx, host)
		if err != nil {
			fmt.Printf("%s. IN A\n", host)
			return nil
		}
		for _, ip := range ips {
			fmt.Printf("%s.\t\t%d\tIN\tA\t%s\n", host, digDefaultTTL, ip)
		}
	}

	return nil
}

func digFullOutput(host, queryType, dnsServer string, useTCP bool) error {
	queryType = strings.ToUpper(queryType)
	resolver := newResolver(dnsServer, useTCP)
	ctx, cancel := dnsQueryContext()
	defer cancel()

	// Header
	fmt.Printf("; <<>> DiG 9.18.0 <<>> %s %s @%s\n", queryType, host, dnsServer)
	if useTCP {
		fmt.Printf(";; TCP connection\n")
	}
	fmt.Printf(";; global options: +cmd\n")

	// Query section
	fmt.Printf("\n;; Query: %s. %s IN %s\n", host, "300", queryType)

	// Answer section
	fmt.Printf("\n;; ANSWER SECTION:\n")
	hasAnswer := false

	queryStart := time.Now()
	var queryErr error
	switch queryType {
	case "A":
		ips, err := resolver.LookupHost(ctx, host)
		queryErr = err
		if err == nil {
			for _, ip := range ips {
				if net.ParseIP(ip).To4() != nil {
					fmt.Printf("%s.\t\t%d\tIN\tA\t%s\n", host, digDefaultTTL, ip)
					hasAnswer = true
				}
			}
		}
	case "AAAA":
		addrs, err := resolver.LookupHost(ctx, host)
		queryErr = err
		if err == nil {
			for _, addr := range addrs {
				ip := net.ParseIP(addr)
				if ip != nil && ip.To4() == nil {
					fmt.Printf("%s.\t\t%d\tIN\tAAAA\t%s\n", host, digDefaultTTL, addr)
					hasAnswer = true
				}
			}
		}
	case "TXT":
		txts, err := resolver.LookupTXT(ctx, host)
		queryErr = err
		if err == nil {
			for _, txt := range txts {
				fmt.Printf("%s.\t\t%d\tIN\tTXT\t\"%s\"\n", host, digDefaultTTL, txt)
				hasAnswer = true
			}
		}
	case "CNAME":
		cname, err := resolver.LookupCNAME(ctx, host)
		queryErr = err
		if err == nil && !cnameIsSelf(host, cname) {
			fmt.Printf("%s.\t\t%d\tIN\tCNAME\t%s\n", host, digDefaultTTL, cname)
			hasAnswer = true
		}
	case "NS":
		nss, err := resolver.LookupNS(ctx, host)
		queryErr = err
		if err == nil {
			for _, ns := range nss {
				fmt.Printf("%s.\t\t%d\tIN\tNS\t%s\n", host, digDefaultTTL, ns.Host)
				hasAnswer = true
			}
		}
	case "MX":
		mxs, err := resolver.LookupMX(ctx, host)
		queryErr = err
		if err == nil {
			for _, mx := range mxs {
				fmt.Printf("%s.\t\t%d\tIN\tMX\t%d %s\n", host, digDefaultTTL, mx.Pref, mx.Host)
				hasAnswer = true
			}
		}
	case "SRV":
		_, addrs, err := resolver.LookupSRV(ctx, "", "", host)
		queryErr = err
		if err == nil {
			for _, srv := range addrs {
				fmt.Printf("%s.\t\t%d\tIN\tSRV\t%d %d %d %s\n", host, digDefaultTTL, srv.Priority, srv.Weight, srv.Port, srv.Target)
				hasAnswer = true
			}
		}
	default:
		ips, err := resolver.LookupHost(ctx, host)
		queryErr = err
		if err == nil {
			for _, ip := range ips {
				fmt.Printf("%s.\t\t%d\tIN\tA\t%s\n", host, digDefaultTTL, ip)
				hasAnswer = true
			}
		}
	}

	nxdomain := false
	if !hasAnswer {
		if dnsErr, ok := queryErr.(*net.DNSError); ok && dnsErr.IsNotFound {
			nxdomain = true
			fmt.Printf(";; ->>HEADER<<- status: NXDOMAIN\n")
		} else {
			fmt.Printf(";; No answer\n")
		}
	}

	// Footer
	fmt.Printf("\n;; Query time: %d msec\n", time.Since(queryStart).Milliseconds())
	fmt.Printf(";; SERVER: %s#53(%s)\n", dnsServer, dnsServer)
	fmt.Printf(";; WHEN: %s\n", time.Now().Format("Mon Jan 2 15:04:05 MST 2006"))

	if nxdomain {
		return digNXDOMAINError{host: host}
	}
	return nil
}

type digNXDOMAINError struct{ host string }

func (e digNXDOMAINError) Error() string          { return fmt.Sprintf("no such host %s (NXDOMAIN)", e.host) }
func (e digNXDOMAINError) ExitCode() int          { return 1 }
func (e digNXDOMAINError) SuppressCLIError() bool { return true }
