// Package discovery expands a user-supplied IP pool spec, scans it over
// SNMP with bounded concurrency, and hands each responding host to
// assignment for rule-based cluster placement.
package discovery

import (
	"fmt"
	"net"
	"strings"
)

// MaxTargets caps how many addresses a single target spec may expand to,
// guarding against a fat-fingered spec like a /8.
const MaxTargets = 4096

// ExpandTargets parses a target spec into a sorted list of IPv4
// addresses. Three forms are accepted: a single IP ("10.0.0.5"), a CIDR
// block ("10.0.0.0/24"), or a dashed range of two full addresses
// ("10.0.0.1-10.0.0.50").
func ExpandTargets(spec string) ([]string, error) {
	spec = strings.TrimSpace(spec)

	if ip := net.ParseIP(spec); ip != nil {
		return []string{ip.String()}, nil
	}
	if strings.Contains(spec, "/") {
		return expandCIDR(spec)
	}
	if strings.Contains(spec, "-") {
		return expandRange(spec)
	}
	return nil, fmt.Errorf("unrecognized target spec %q: expected an IP, CIDR block, or dashed range", spec)
}

func expandCIDR(cidr string) ([]string, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}

	var ips []string
	for cur := ip.Mask(ipNet.Mask); ipNet.Contains(cur); cur = nextIP(cur) {
		if len(ips) >= MaxTargets {
			return nil, fmt.Errorf("CIDR %q expands to more than %d addresses", cidr, MaxTargets)
		}
		ips = append(ips, cur.String())
	}
	return ips, nil
}

func expandRange(spec string) ([]string, error) {
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range %q: expected start-end", spec)
	}

	start := net.ParseIP(strings.TrimSpace(parts[0]))
	end := net.ParseIP(strings.TrimSpace(parts[1]))
	if start == nil || end == nil {
		return nil, fmt.Errorf("invalid range %q: both bounds must be valid IPs", spec)
	}
	start, end = start.To4(), end.To4()
	if start == nil || end == nil {
		return nil, fmt.Errorf("invalid range %q: only IPv4 is supported", spec)
	}
	if ipToUint32(start) > ipToUint32(end) {
		return nil, fmt.Errorf("invalid range %q: start must not be after end", spec)
	}

	var ips []string
	for cur := start; ; cur = nextIP(cur) {
		if len(ips) >= MaxTargets {
			return nil, fmt.Errorf("range %q expands to more than %d addresses", spec, MaxTargets)
		}
		ips = append(ips, cur.String())
		if cur.Equal(end) {
			break
		}
	}
	return ips, nil
}

func nextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			break
		}
	}
	return next
}

func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}
