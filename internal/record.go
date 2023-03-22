package internal

import (
	"net"
)

// Get only ipv4 values, not ipv6
func GetRecordIPv4(domainName string) ([]string, error) {
	// use system DNS resolver
	net.DefaultResolver.PreferGo = false
	ips, err := net.LookupIP(domainName)
	if err != nil {
		return nil, err
	}

	var ipList []string
	for _, ip := range ips {
		if net.ParseIP(ip.String()).To4() != nil {
			ipList = append(ipList, ip.String())
		}
	}

	return ipList, nil
}
