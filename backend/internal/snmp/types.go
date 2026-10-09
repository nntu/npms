package snmp

import (
	"context"
	"fmt"
	"strings"
)

// Version is an SNMP protocol version supported by NPMS.
type Version string

const (
	Version2c Version = "2c"
	Version3  Version = "3"
)

// Config contains connection settings. Secret fields are deliberately not JSON tagged
// so callers cannot accidentally expose them through an API response.
type Config struct {
	Host           string
	Port           uint16
	Version        Version
	Community      string `json:"-"`
	Username       string `json:"-"`
	AuthProtocol   string `json:"-"`
	AuthPassphrase string `json:"-"`
	PrivProtocol   string `json:"-"`
	PrivPassphrase string `json:"-"`
	TimeoutSeconds int
	Retries        int
	MaxRepetitions uint8
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port == 0 {
		return fmt.Errorf("port must be greater than zero")
	}
	if c.Version != Version2c && c.Version != Version3 {
		return fmt.Errorf("unsupported SNMP version %q", c.Version)
	}
	if c.TimeoutSeconds <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	if c.Retries < 0 {
		return fmt.Errorf("retries cannot be negative")
	}
	if c.MaxRepetitions == 0 {
		return fmt.Errorf("max repetitions must be greater than zero")
	}
	if c.Version == Version2c && c.Community == "" {
		return fmt.Errorf("community is required for SNMP v2c")
	}
	if c.Version == Version3 && c.Username == "" {
		return fmt.Errorf("username is required for SNMP v3")
	}
	if c.Version == Version3 && c.PrivProtocol != "" && c.AuthProtocol == "" {
		return fmt.Errorf("privacy protocol requires an auth protocol for SNMP v3")
	}
	if c.AuthProtocol != "" && c.AuthPassphrase == "" {
		return fmt.Errorf("auth passphrase is required when auth protocol is set")
	}
	if c.PrivProtocol != "" && c.PrivPassphrase == "" {
		return fmt.Errorf("privacy passphrase is required when privacy protocol is set")
	}
	return nil
}

// VarBind is a transport-neutral SNMP result. Value contains a JSON-safe value.
type VarBind struct {
	OID   string `json:"oid"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type Client interface {
	Get(context.Context, []string) ([]VarBind, error)
	Walk(context.Context, string) ([]VarBind, error)
	Close() error
}

// MarkerRow is joined by the exact Printer-MIB table instance, never by row order.
type MarkerRow struct {
	Instance string `json:"instance"`
	Life     int64  `json:"life_count"`
	Unit     int64  `json:"counter_unit"`
}

func JoinMarkerRows(life, units []VarBind, lifeBase, unitBase string) []MarkerRow {
	unitByInstance := make(map[string]int64, len(units))
	for _, vb := range units {
		instance, ok := tableInstance(vb.OID, unitBase)
		if !ok {
			continue
		}
		value, ok := integerValue(vb.Value)
		if ok {
			unitByInstance[instance] = value
		}
	}
	rows := make([]MarkerRow, 0, len(life))
	for _, vb := range life {
		instance, ok := tableInstance(vb.OID, lifeBase)
		if !ok {
			continue
		}
		lifeValue, ok := integerValue(vb.Value)
		if !ok {
			continue
		}
		unitValue, ok := unitByInstance[instance]
		if !ok {
			continue
		}
		rows = append(rows, MarkerRow{Instance: instance, Life: lifeValue, Unit: unitValue})
	}
	return rows
}

func tableInstance(oid, base string) (string, bool) {
	oid = strings.TrimPrefix(oid, ".")
	base = strings.TrimPrefix(base, ".")
	if oid == base || !strings.HasPrefix(oid, base+".") {
		return "", false
	}
	return strings.TrimPrefix(oid, base+"."), true
}

func integerValue(value any) (int64, bool) {
	switch n := value.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint:
		return int64(n), true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		if n <= uint64(^uint64(0)>>1) {
			return int64(n), true
		}
	case string:
		var parsed int64
		if _, err := fmt.Sscanf(n, "%d", &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

// NumericValue exposes the safe integer conversion needed by application
// ingestion while keeping SNMP decoding separate from counter validation.
func NumericValue(value any) (int64, bool) { return integerValue(value) }
