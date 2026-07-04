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
)

// digCmd implements dig functionality
func DigCmd(args []string) error {
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
			digUsage(os.Stdout)
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
		case (arg == "-t" || arg == "--type") && i+1 < len(args):
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
		fmt.Fprintln(os.Stderr, "dig: missing host argument")
		digUsage(os.Stderr)
		return fmt.Errorf("host required")
	}

	// Default query type is A if not specified
	if queryType == "" {
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

func digUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: dig [@DNS_SERVER] HOST [DNS_SERVER] [OPTIONS]")
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
	fmt.Fprintln(w, "  dig example.com")
	fmt.Fprintln(w, "  dig @8.8.8.8 example.com")
	fmt.Fprintln(w, "  dig -t MX example.com")
	fmt.Fprintln(w, "  dig +short example.com")
	fmt.Fprintln(w, "  dig +noall +answer example.com")
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
	ips, err := resolver.LookupHost(context.Background(), host)
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
	addrs, err := resolver.LookupHost(context.Background(), host)
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
	txts, err := resolver.LookupTXT(context.Background(), host)
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
	cname, err := resolver.LookupCNAME(context.Background(), host)
	if err != nil {
		if _, ok := err.(*net.DNSError); ok {
			fmt.Printf("** server can't find %s: NXDOMAIN\n", host)
		}
		return fmt.Errorf("lookup failed: %w", err)
	}
	fmt.Printf("Name:   %s\n", host)
	fmt.Printf("Canonical name: %s\n\n", cname)
	return nil
}

func lookupNS(host string, resolver *net.Resolver) error {
	nameservers, err := resolver.LookupNS(context.Background(), host)
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
	mxs, err := resolver.LookupMX(context.Background(), host)
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
	_, addrs, err := resolver.LookupSRV(context.Background(), "", "", host)
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
	names, err := resolver.LookupAddr(context.Background(), host)
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

	switch queryType {
	case "A":
		ips, err := resolver.LookupHost(context.Background(), host)
		if err != nil {
			return nil
		}
		for _, ip := range ips {
			if net.ParseIP(ip).To4() != nil {
				fmt.Println(ip)
			}
		}
	case "AAAA":
		addrs, err := resolver.LookupHost(context.Background(), host)
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
		txts, err := resolver.LookupTXT(context.Background(), host)
		if err != nil {
			return nil
		}
		for _, txt := range txts {
			fmt.Println(txt)
		}
	case "CNAME":
		cname, err := resolver.LookupCNAME(context.Background(), host)
		if err != nil {
			return nil
		}
		fmt.Println(cname)
	case "NS":
		nss, err := resolver.LookupNS(context.Background(), host)
		if err != nil {
			return nil
		}
		for _, ns := range nss {
			fmt.Println(ns.Host)
		}
	case "MX":
		mxs, err := resolver.LookupMX(context.Background(), host)
		if err != nil {
			return nil
		}
		for _, mx := range mxs {
			fmt.Printf("%d %s\n", mx.Pref, mx.Host)
		}
	case "SRV":
		_, addrs, err := resolver.LookupSRV(context.Background(), "", "", host)
		if err != nil {
			return nil
		}
		for _, srv := range addrs {
			fmt.Printf("%d %d %d %s\n", srv.Priority, srv.Weight, srv.Port, srv.Target)
		}
	default:
		ips, err := resolver.LookupHost(context.Background(), host)
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

	fmt.Printf(";; ANSWER SECTION:\n")
	switch queryType {
	case "A":
		ips, err := resolver.LookupHost(context.Background(), host)
		if err != nil {
			fmt.Printf("%s. IN A\n", host)
			return nil
		}
		for _, ip := range ips {
			if net.ParseIP(ip).To4() != nil {
				fmt.Printf("%s. IN A %s\n", host, ip)
			}
		}
	case "AAAA":
		addrs, err := resolver.LookupHost(context.Background(), host)
		if err != nil {
			fmt.Printf("%s. IN AAAA\n", host)
			return nil
		}
		for _, addr := range addrs {
			ip := net.ParseIP(addr)
			if ip != nil && ip.To4() == nil {
				fmt.Printf("%s. IN AAAA %s\n", host, addr)
			}
		}
	case "TXT":
		txts, err := resolver.LookupTXT(context.Background(), host)
		if err != nil {
			fmt.Printf("%s. IN TXT\n", host)
			return nil
		}
		for _, txt := range txts {
			fmt.Printf("%s. IN TXT \"%s\"\n", host, txt)
		}
	case "CNAME":
		cname, err := resolver.LookupCNAME(context.Background(), host)
		if err != nil {
			fmt.Printf("%s. IN CNAME\n", host)
			return nil
		}
		fmt.Printf("%s. IN CNAME %s\n", host, cname)
	case "NS":
		nss, err := resolver.LookupNS(context.Background(), host)
		if err != nil {
			fmt.Printf("%s. IN NS\n", host)
			return nil
		}
		for _, ns := range nss {
			fmt.Printf("%s. IN NS %s\n", host, ns.Host)
		}
	case "MX":
		mxs, err := resolver.LookupMX(context.Background(), host)
		if err != nil {
			fmt.Printf("%s. IN MX\n", host)
			return nil
		}
		for _, mx := range mxs {
			fmt.Printf("%s. IN MX %d %s\n", host, mx.Pref, mx.Host)
		}
	case "SRV":
		_, addrs, err := resolver.LookupSRV(context.Background(), "", "", host)
		if err != nil {
			fmt.Printf("%s. IN SRV\n", host)
			return nil
		}
		for _, srv := range addrs {
			fmt.Printf("%s. IN SRV %d %d %d %s\n", host, srv.Priority, srv.Weight, srv.Port, srv.Target)
		}
	default:
		ips, err := resolver.LookupHost(context.Background(), host)
		if err != nil {
			fmt.Printf("%s. IN A\n", host)
			return nil
		}
		for _, ip := range ips {
			fmt.Printf("%s. IN A %s\n", host, ip)
		}
	}

	return nil
}

func digFullOutput(host, queryType, dnsServer string, useTCP bool) error {
	queryType = strings.ToUpper(queryType)
	resolver := newResolver(dnsServer, useTCP)

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
	switch queryType {
	case "A":
		ips, err := resolver.LookupHost(context.Background(), host)
		if err == nil {
			for _, ip := range ips {
				if net.ParseIP(ip).To4() != nil {
					fmt.Printf("%s. IN A %s\n", host, ip)
					hasAnswer = true
				}
			}
		}
	case "AAAA":
		addrs, err := resolver.LookupHost(context.Background(), host)
		if err == nil {
			for _, addr := range addrs {
				ip := net.ParseIP(addr)
				if ip != nil && ip.To4() == nil {
					fmt.Printf("%s. IN AAAA %s\n", host, addr)
					hasAnswer = true
				}
			}
		}
	case "TXT":
		txts, err := resolver.LookupTXT(context.Background(), host)
		if err == nil {
			for _, txt := range txts {
				fmt.Printf("%s. IN TXT \"%s\"\n", host, txt)
				hasAnswer = true
			}
		}
	case "CNAME":
		cname, err := resolver.LookupCNAME(context.Background(), host)
		if err == nil {
			fmt.Printf("%s. IN CNAME %s\n", host, cname)
			hasAnswer = true
		}
	case "NS":
		nss, err := resolver.LookupNS(context.Background(), host)
		if err == nil {
			for _, ns := range nss {
				fmt.Printf("%s. IN NS %s\n", host, ns.Host)
				hasAnswer = true
			}
		}
	case "MX":
		mxs, err := resolver.LookupMX(context.Background(), host)
		if err == nil {
			for _, mx := range mxs {
				fmt.Printf("%s. IN MX %d %s\n", host, mx.Pref, mx.Host)
				hasAnswer = true
			}
		}
	case "SRV":
		_, addrs, err := resolver.LookupSRV(context.Background(), "", "", host)
		if err == nil {
			for _, srv := range addrs {
				fmt.Printf("%s. IN SRV %d %d %d %s\n", host, srv.Priority, srv.Weight, srv.Port, srv.Target)
				hasAnswer = true
			}
		}
	default:
		ips, err := resolver.LookupHost(context.Background(), host)
		if err == nil {
			for _, ip := range ips {
				fmt.Printf("%s. IN A %s\n", host, ip)
				hasAnswer = true
			}
		}
	}

	if !hasAnswer {
		fmt.Printf(";; No answer\n")
	}

	// Footer
	fmt.Printf("\n;; Query time: %d msec\n", time.Since(queryStart).Milliseconds())
	fmt.Printf(";; SERVER: %s#53(%s)\n", dnsServer, dnsServer)
	fmt.Printf(";; WHEN: %s\n", time.Now().Format("Mon Jan 2 15:04:05 MST 2006"))

	return nil
}
