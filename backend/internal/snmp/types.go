package snmp

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
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
	Instance     string `json:"instance"`
	Life         int64  `json:"life_count"`
	Unit         int64  `json:"counter_unit"`
	UnitName     string `json:"unit_name,omitempty"`
	PowerOnCount int64  `json:"power_on_count,omitempty"`
	MarkTech     int64  `json:"mark_tech,omitempty"`
	Status       int64  `json:"status,omitempty"`
}

// IsExceptionVarBind reports whether a VarBind represents an SNMP exception or null value
// (such as NoSuchObject, NoSuchInstance, EndOfMibView) rather than a valid datum.
func IsExceptionVarBind(vb VarBind) bool {
	if vb.Value == nil {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(vb.Type)) {
	case "nosuchobject", "nosuchinstance", "endofmibview", "null":
		return true
	}
	return false
}

func JoinMarkerRows(life, units []VarBind, lifeBase, unitBase string) []MarkerRow {
	unitByInstance := make(map[string]int64, len(units))
	for _, vb := range units {
		if IsExceptionVarBind(vb) {
			continue
		}
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
		if IsExceptionVarBind(vb) {
			continue
		}
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
		rows = append(rows, MarkerRow{Instance: instance, Life: lifeValue, Unit: unitValue, UnitName: CounterUnitName(unitValue)})
	}
	return rows
}

func TableInstance(oid, base string) (string, bool) {
	return tableInstance(oid, base)
}

func tableInstance(oid, base string) (string, bool) {
	oid = strings.TrimPrefix(oid, ".")
	base = strings.TrimPrefix(base, ".")
	if oid == base || !strings.HasPrefix(oid, base+".") {
		return "", false
	}
	return strings.TrimPrefix(oid, base+"."), true
}

func signedInt64(value any) (int64, bool) {
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
	case float64:
		if float64(int64(n)) == n {
			return int64(n), true
		}
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func integerValue(value any) (int64, bool) {
	v, ok := signedInt64(value)
	if !ok {
		return 0, false
	}
	return nonNegativeInt(v)
}

func nonNegativeInt(value int64) (int64, bool) {
	if value < 0 {
		return 0, false
	}
	return value, true
}

// NumericValue exposes the safe non-negative integer conversion needed by counter
// ingestion while keeping SNMP decoding separate from counter validation.
func NumericValue(value any) (int64, bool) { return integerValue(value) }

// SignedIntegerValue converts an SNMP integer value that may legitimately use RFC 3805
// negative sentinel codes (e.g. -1 other, -2 unknown, -3 someRemaining in prtMarkerSuppliesLevel).
func SignedIntegerValue(value any) (int64, bool) { return signedInt64(value) }

// NormalizeVarBindValue converts raw []byte OctetStrings into human-readable UTF-8 text
// when all runes are printable, or into a "0x..." hex string when binary bytes are present.
func NormalizeVarBindValue(value any) any {
	switch v := value.(type) {
	case []byte:
		trimmed := bytes.TrimRight(v, "\x00")
		if len(trimmed) == 0 {
			if len(v) == 0 {
				return ""
			}
			return fmt.Sprintf("0x%x", v)
		}
		if isPrintableUTF8(trimmed) {
			return string(trimmed)
		}
		return fmt.Sprintf("0x%x", v)
	default:
		return value
	}
}

// StringValue extracts a clean string from a VarBind value (handling []byte, printable strings,
// and legacy "0x..." hex-encoded ASCII strings).
func StringValue(value any) (string, bool) {
	if value == nil {
		return "", false
	}
	switch v := value.(type) {
	case []byte:
		trimmed := bytes.TrimRight(v, "\x00")
		if isPrintableUTF8(trimmed) {
			return strings.TrimSpace(string(trimmed)), true
		}
		return fmt.Sprintf("0x%x", v), true
	case string:
		s := strings.TrimSpace(v)
		if strings.HasPrefix(s, "0x") && len(s) > 2 && len(s)%2 == 0 {
			if raw, err := hex.DecodeString(s[2:]); err == nil {
				trimmed := bytes.TrimRight(raw, "\x00")
				if len(trimmed) > 0 && isPrintableUTF8(trimmed) {
					return strings.TrimSpace(string(trimmed)), true
				}
			}
		}
		return s, true
	default:
		return strings.TrimSpace(fmt.Sprint(v)), true
	}
}

func isPrintableUTF8(b []byte) bool {
	if !utf8.Valid(b) {
		return false
	}
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if !unicode.IsPrint(r) && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
		b = b[size:]
	}
	return true
}
