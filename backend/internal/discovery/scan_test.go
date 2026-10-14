package discovery

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"npms/backend/internal/snmp"
)

func TestIsPrivateIPv4(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.254", true},
		{"192.168.1.1", true},
		{"192.168.100.254", true},
		{"127.0.0.1", true},
		{"169.254.1.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"172.15.0.1", false},
		{"172.32.0.1", false},
		{"192.169.1.1", false},
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if got := IsPrivateIPv4(ip); got != tt.want {
			t.Errorf("IsPrivateIPv4(%q) = %v, want %v", tt.ip, got, tt.want)
		}
	}
}

func TestGenerateIPList(t *testing.T) {
	t.Run("Valid /24 CIDR", func(t *testing.T) {
		ips, err := GenerateIPList("192.168.1.0/24", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ips) != 254 {
			t.Fatalf("expected 254 usable IPs in /24, got %d", len(ips))
		}
		if ips[0] != "192.168.1.1" || ips[len(ips)-1] != "192.168.1.254" {
			t.Fatalf("unexpected range: first=%s, last=%s", ips[0], ips[len(ips)-1])
		}
	})

	t.Run("Public CIDR Rejected", func(t *testing.T) {
		_, err := GenerateIPList("8.8.8.0/24", "", "")
		if !errors.Is(err, ErrPublicIPNotAllowed) {
			t.Fatalf("expected ErrPublicIPNotAllowed, got %v", err)
		}
	})

	t.Run("Oversized CIDR Rejected", func(t *testing.T) {
		_, err := GenerateIPList("10.0.0.0/16", "", "")
		if !errors.Is(err, ErrSubnetTooLarge) {
			t.Fatalf("expected ErrSubnetTooLarge, got %v", err)
		}
	})

	t.Run("Valid Range", func(t *testing.T) {
		ips, err := GenerateIPList("", "192.168.1.10", "192.168.1.20")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ips) != 11 {
			t.Fatalf("expected 11 IPs, got %d", len(ips))
		}
		if ips[0] != "192.168.1.10" || ips[10] != "192.168.1.20" {
			t.Fatalf("unexpected range: first=%s, last=%s", ips[0], ips[10])
		}
	})

	t.Run("Invalid Range Start > End", func(t *testing.T) {
		_, err := GenerateIPList("", "192.168.1.20", "192.168.1.10")
		if err == nil {
			t.Fatalf("expected error for inverted range")
		}
	})
}

func TestScanSubnet(t *testing.T) {
	ctx := context.Background()
	ips := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}

	factory := func(cfg snmp.Config) (snmp.Client, error) {
		if cfg.Host == "192.168.1.2" {
			return &fakePrinterClient{
				host: cfg.Host,
				desc: "HP LaserJet Pro M404dn",
				oid:  "1.3.6.1.4.1.11.2.3.9.1",
				name: "HP-M404DN",
			}, nil
		}
		return &fakeFailingClient{}, nil
	}

	results, err := ScanSubnet(ctx, SubnetScanConfig{
		IPs:         ips,
		Port:        161,
		Version:     "2c",
		Community:   "public",
		Timeout:     500 * time.Millisecond,
		Concurrency: 5,
	}, factory)

	if err != nil {
		t.Fatalf("scan subnet failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 printer discovered, got %d", len(results))
	}
	if results[0].Address != "192.168.1.2" || results[0].Manufacturer != "HP" {
		t.Fatalf("unexpected discovered printer: %+v", results[0])
	}
}

type fakePrinterClient struct {
	host string
	desc string
	oid  string
	name string
}

func (c *fakePrinterClient) Get(ctx context.Context, oids []string) ([]snmp.VarBind, error) {
	var results []snmp.VarBind
	for _, oid := range oids {
		switch oid {
		case SysDescrOID:
			results = append(results, snmp.VarBind{OID: oid, Value: c.desc})
		case SysObjectOID:
			results = append(results, snmp.VarBind{OID: oid, Value: c.oid})
		case SysNameOID:
			results = append(results, snmp.VarBind{OID: oid, Value: c.name})
		case PrinterNameInstance1OID:
			results = append(results, snmp.VarBind{OID: oid, Value: "HP LaserJet Pro M404dn"})
		case SerialOID, SerialInstance1OID:
			results = append(results, snmp.VarBind{OID: oid, Value: "VNBF123456"})
		}
	}
	return results, nil
}

func (c *fakePrinterClient) Walk(ctx context.Context, oid string) ([]snmp.VarBind, error) {
	return nil, nil
}

func (c *fakePrinterClient) Close() error {
	return nil
}

type fakeFailingClient struct{}

func (f *fakeFailingClient) Get(ctx context.Context, oids []string) ([]snmp.VarBind, error) {
	return nil, errors.New("timeout connecting to host")
}

func (f *fakeFailingClient) Walk(ctx context.Context, oid string) ([]snmp.VarBind, error) {
	return nil, errors.New("timeout connecting to host")
}

func (f *fakeFailingClient) Close() error {
	return nil
}
