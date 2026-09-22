package probe

import "net"

// LookupIPv4 returns the IPv4 addresses behind host. An address given instead
// of a name comes straight back, so a caller can pass either.
//
// IPv6 is filtered out on purpose: the rest of the tool joins an address to a
// port without brackets, and an AAAA record is not what the A-record report is
// about.
func LookupIPv4(host string) ([]string, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}

	var ipv4 []string
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			ipv4 = append(ipv4, v4.String())
		}
	}

	return ipv4, nil
}
