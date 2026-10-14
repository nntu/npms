package discovery

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"npms/backend/internal/snmp"
)

var (
	ErrPublicIPNotAllowed = errors.New("only private and local IPv4 networks (RFC 1918, 127.0.0.0/8, 169.254.0.0/16) are permitted for scanning")
	ErrSubnetTooLarge     = errors.New("subnet scan target is too large; maximum 512 IP addresses allowed per scan")
	ErrInvalidIPRange     = errors.New("invalid IP range or CIDR specification")
)

// Private/local IPv4 network blocks per RFC 1918, RFC 3927, RFC 1122
var privateCIDRs = []*net.IPNet{
	mustParseCIDR("10.0.0.0/8"),
	mustParseCIDR("172.16.0.0/12"),
	mustParseCIDR("192.168.0.0/16"),
	mustParseCIDR("127.0.0.0/8"),
	mustParseCIDR("169.254.0.0/16"),
}

func mustParseCIDR(s string) *net.IPNet {
	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return ipnet
}

// IsPrivateIPv4 checks if an IP belongs to private or link-local/loopback address spaces.
func IsPrivateIPv4(ip net.IP) bool {
	ipv4 := ip.To4()
	if ipv4 == nil {
		return false
	}
	for _, block := range privateCIDRs {
		if block.Contains(ipv4) {
			return true
		}
	}
	return false
}

// GenerateIPList parses a CIDR or Start/End IP pair and returns a list of target IPv4 addresses.
func GenerateIPList(cidr, startIPStr, endIPStr string) ([]string, error) {
	cidr = strings.TrimSpace(cidr)
	startIPStr = strings.TrimSpace(startIPStr)
	endIPStr = strings.TrimSpace(endIPStr)

	if cidr != "" {
		ip, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid CIDR format %q", ErrInvalidIPRange, cidr)
		}
		ipv4 := ip.To4()
		if ipv4 == nil {
			return nil, fmt.Errorf("%w: only IPv4 CIDRs are supported", ErrInvalidIPRange)
		}
		if !IsPrivateIPv4(ipv4) {
			return nil, ErrPublicIPNotAllowed
		}

		ones, bits := ipNet.Mask.Size()
		if bits != 32 {
			return nil, fmt.Errorf("%w: not an IPv4 mask", ErrInvalidIPRange)
		}
		if ones < 23 { // /23 is 512 IPs; /24 is 256 IPs
			return nil, ErrSubnetTooLarge
		}

		startInt := binary.BigEndian.Uint32(ipNet.IP.To4())
		maskInt := binary.BigEndian.Uint32(ipNet.Mask)
		broadcastInt := startInt | ^maskInt
		count := broadcastInt - startInt + 1

		var ips []string
		if count <= 2 {
			// e.g. /31 or /32
			for i := startInt; i <= broadcastInt; i++ {
				cur := make(net.IP, 4)
				binary.BigEndian.PutUint32(cur, i)
				ips = append(ips, cur.String())
			}
		} else {
			// Skip network address (startInt) and broadcast address (broadcastInt)
			for i := startInt + 1; i < broadcastInt; i++ {
				cur := make(net.IP, 4)
				binary.BigEndian.PutUint32(cur, i)
				ips = append(ips, cur.String())
			}
		}

		if len(ips) > 512 {
			return nil, ErrSubnetTooLarge
		}
		return ips, nil
	}

	if startIPStr != "" && endIPStr != "" {
		startIP := net.ParseIP(startIPStr).To4()
		endIP := net.ParseIP(endIPStr).To4()
		if startIP == nil || endIP == nil {
			return nil, fmt.Errorf("%w: invalid IPv4 address range", ErrInvalidIPRange)
		}
		if !IsPrivateIPv4(startIP) || !IsPrivateIPv4(endIP) {
			return nil, ErrPublicIPNotAllowed
		}

		startInt := binary.BigEndian.Uint32(startIP)
		endInt := binary.BigEndian.Uint32(endIP)
		if startInt > endInt {
			return nil, fmt.Errorf("%w: start IP %s must be <= end IP %s", ErrInvalidIPRange, startIPStr, endIPStr)
		}
		count := endInt - startInt + 1
		if count > 512 {
			return nil, ErrSubnetTooLarge
		}

		ips := make([]string, 0, count)
		for i := startInt; i <= endInt; i++ {
			cur := make(net.IP, 4)
			binary.BigEndian.PutUint32(cur, i)
			ips = append(ips, cur.String())
		}
		return ips, nil
	}

	return nil, fmt.Errorf("%w: either CIDR or start_ip and end_ip must be provided", ErrInvalidIPRange)
}

// SubnetScanConfig specifies options for subnet range scanning.
type SubnetScanConfig struct {
	IPs            []string
	Port           uint16
	Version        snmp.Version
	Community      string
	Username       string
	AuthProtocol   string
	AuthPassphrase string
	PrivProtocol   string
	PrivPassphrase string
	Timeout        time.Duration
	Concurrency    int
}

// ScanSubnet scans a slice of IP addresses concurrently with bounded parallelism
// and returns all responsive printer devices.
func ScanSubnet(ctx context.Context, config SubnetScanConfig, factory ClientFactory) ([]Result, error) {
	if factory == nil {
		return nil, fmt.Errorf("SNMP client factory is required")
	}
	if len(config.IPs) == 0 {
		return nil, nil
	}

	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = 25
	}
	if concurrency > 50 {
		concurrency = 50
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 1000 * time.Millisecond
	}
	if timeout > 5*time.Second {
		timeout = 5 * time.Second
	}

	port := config.Port
	if port == 0 {
		port = 161
	}

	version := config.Version
	if version == "" {
		version = "2c"
	}

	var (
		mu      sync.Mutex
		results []Result
		sem     = make(chan struct{}, concurrency)
		wg      sync.WaitGroup
	)

	for _, ip := range config.IPs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(targetIP string) {
			defer func() {
				<-sem
				wg.Done()
			}()

			probeCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			snmpCfg := snmp.Config{
				Host:           targetIP,
				Port:           port,
				Version:        version,
				Community:      config.Community,
				Username:       config.Username,
				AuthProtocol:   config.AuthProtocol,
				AuthPassphrase: config.AuthPassphrase,
				PrivProtocol:   config.PrivProtocol,
				PrivPassphrase: config.PrivPassphrase,
				TimeoutSeconds: int(timeout.Seconds() + 0.9),
				Retries:        0,
				MaxRepetitions: 10,
			}

			res, err := Probe(probeCtx, snmpCfg, factory)
			if err != nil {
				return
			}

			// Filter: only keep devices that look like printers or have Printer-MIB markers
			if isLikelyPrinter(res) {
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
			}
		}(ip)
	}

	wg.Wait()

	// Sort results numerically by IPv4
	sort.Slice(results, func(i, j int) bool {
		ipA := net.ParseIP(results[i].Address).To4()
		ipB := net.ParseIP(results[j].Address).To4()
		if ipA == nil || ipB == nil {
			return results[i].Address < results[j].Address
		}
		return bytes.Compare(ipA, ipB) < 0
	})

	return results, nil
}

func isLikelyPrinter(r Result) bool {
	if len(r.Markers) > 0 {
		return true
	}
	if r.Manufacturer != "" && r.Manufacturer != "generic" {
		return true
	}
	combined := strings.ToLower(r.Name + " " + r.Description + " " + r.Model)
	keywords := []string{"print", "laserjet", "pagepro", "ricoh", "brother", "epson", "canon", "kyocera", "xerox", "lexmark", "inkjet", "copier", "mfp"}
	for _, kw := range keywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}
