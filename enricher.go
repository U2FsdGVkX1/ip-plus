package main

import (
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"

	"github.com/xiaoqidun/qqwry"
)

var (
	// Pre-compiled regular expressions for IP matching
	ipv4Regex *regexp.Regexp
	ipv6Regex *regexp.Regexp
)

func init() {
	// IPv4 pattern: simple dotted decimal format
	ipv4Regex = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

	// IPv6 pattern: only match bracket-enclosed format [xxxx:xxxx]
	// This avoids false positives from port numbers (e.g., "pid:123")
	ipv6Regex = regexp.MustCompile(`\[([0-9a-fA-F:]+)\]`)
}

// ipMatch stores matched IP address and its position in the string
type ipMatch struct {
	ip       string
	startPos int
	endPos   int
}

// isSpecialIP checks if the IP is special (loopback, private, etc.)
func isSpecialIP(ip string) bool {
	// Remove possible brackets
	ip = strings.Trim(ip, "[]")

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Check if it's loopback, unspecified, link-local, or private address
	if parsedIP.IsLoopback() || parsedIP.IsUnspecified() ||
		parsedIP.IsLinkLocalUnicast() || parsedIP.IsLinkLocalMulticast() ||
		parsedIP.IsPrivate() {
		return true
	}

	return false
}

// formatLocation formats location information from qqwry result
func formatLocation(loc *qqwry.Location) string {
	if loc == nil {
		return "Unknown"
	}

	// Priority: Country + Province + City
	parts := []string{}

	if loc.Country != "" && loc.Country != "0" {
		parts = append(parts, loc.Country)
	}
	if loc.Province != "" && loc.Province != "0" {
		parts = append(parts, loc.Province)
	}
	if loc.City != "" && loc.City != "0" {
		parts = append(parts, loc.City)
	}

	if len(parts) == 0 {
		return "Unknown"
	}

	return strings.Join(parts, "")
}

// findAllIPs finds all IP addresses in a line with their positions
func findAllIPs(line string) []ipMatch {
	matches := []ipMatch{}

	// Find IPv4 addresses
	ipv4Matches := ipv4Regex.FindAllStringIndex(line, -1)
	for _, match := range ipv4Matches {
		ip := line[match[0]:match[1]]
		matches = append(matches, ipMatch{
			ip:       ip,
			startPos: match[0],
			endPos:   match[1],
		})
	}

	// Find IPv6 addresses (bracket-enclosed only)
	ipv6Matches := ipv6Regex.FindAllStringSubmatchIndex(line, -1)
	for _, match := range ipv6Matches {
		// match[0], match[1] is the full match [xxx]
		// match[2], match[3] is the captured group (content inside brackets)

		ip := line[match[2]:match[3]]
		matches = append(matches, ipMatch{
			ip:       ip,
			startPos: match[0],
			endPos:   match[1],
		})
	}

	return matches
}

// EnrichLine processes a line of text and adds location annotations to IP addresses
func EnrichLine(line string) string {
	matches := findAllIPs(line)
	if len(matches) == 0 {
		return line
	}

	// Sort matches by position (descending) to process from right to left
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].endPos > matches[j].endPos
	})

	// Replace from right to left to avoid position offset issues
	for i := 0; i < len(matches); i++ {
		match := matches[i]
		ip := match.ip

		var location string
		if isSpecialIP(ip) {
			location = "Local"
		} else {
			loc, err := qqwry.QueryIP(ip)
			if err != nil || loc == nil {
				location = "Unknown"
			} else {
				location = formatLocation(loc)
			}
		}

		// Insert annotation after IP
		annotation := fmt.Sprintf("(%s)", location)
		line = line[:match.endPos] + annotation + line[match.endPos:]
	}

	return line
}
