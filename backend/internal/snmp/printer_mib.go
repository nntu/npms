package snmp

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Standard RFC 3805 / Printer-MIB OIDs defined in Printer-MIB.yaml.
const (
	PrintMIBRootOID = "1.3.6.1.2.1.43"

	// prtGeneralTable (1.3.6.1.2.1.43.5.1) - indexed by hrDeviceIndex (typically .1)
	PrtGeneralTableOID               = "1.3.6.1.2.1.43.5.1"
	PrtGeneralEntryOID               = "1.3.6.1.2.1.43.5.1.1"
	PrtGeneralConfigChangesOID       = "1.3.6.1.2.1.43.5.1.1.1"
	PrtGeneralCurrentLocalizationOID = "1.3.6.1.2.1.43.5.1.1.2"
	PrtGeneralResetOID               = "1.3.6.1.2.1.43.5.1.1.3"
	PrtInputDefaultIndexOID          = "1.3.6.1.2.1.43.5.1.1.6"
	PrtOutputDefaultIndexOID         = "1.3.6.1.2.1.43.5.1.1.7"
	PrtMarkerDefaultIndexOID         = "1.3.6.1.2.1.43.5.1.1.8"
	PrtMediaPathDefaultIndexOID      = "1.3.6.1.2.1.43.5.1.1.9"
	PrtGeneralPrinterNameOID         = "1.3.6.1.2.1.43.5.1.1.16"
	PrtGeneralSerialNumberOID        = "1.3.6.1.2.1.43.5.1.1.17"
	PrtAlertCriticalEventsOID        = "1.3.6.1.2.1.43.5.1.1.18"
	PrtAlertAllEventsOID             = "1.3.6.1.2.1.43.5.1.1.19"

	// Common instance .1 shortcuts for prtGeneralEntry (hrDeviceIndex = 1)
	PrtMarkerDefaultIndexInstance1OID  = "1.3.6.1.2.1.43.5.1.1.8.1"
	PrtGeneralPrinterNameInstance1OID  = "1.3.6.1.2.1.43.5.1.1.16.1"
	PrtGeneralSerialNumberInstance1OID = "1.3.6.1.2.1.43.5.1.1.17.1"

	// prtCoverTable (1.3.6.1.2.1.43.6.1) - indexed by (hrDeviceIndex, prtCoverIndex)
	PrtCoverTableOID       = "1.3.6.1.2.1.43.6.1"
	PrtCoverEntryOID       = "1.3.6.1.2.1.43.6.1.1"
	PrtCoverIndexOID       = "1.3.6.1.2.1.43.6.1.1.1"
	PrtCoverDescriptionOID = "1.3.6.1.2.1.43.6.1.1.2"
	PrtCoverStatusOID      = "1.3.6.1.2.1.43.6.1.1.3"

	// prtInputTable (1.3.6.1.2.1.43.8.2) - indexed by (hrDeviceIndex, prtInputIndex)
	PrtInputTableOID        = "1.3.6.1.2.1.43.8.2"
	PrtInputEntryOID        = "1.3.6.1.2.1.43.8.2.1"
	PrtInputIndexOID        = "1.3.6.1.2.1.43.8.2.1.1"
	PrtInputTypeOID         = "1.3.6.1.2.1.43.8.2.1.2"
	PrtInputCapacityUnitOID = "1.3.6.1.2.1.43.8.2.1.8"
	PrtInputMaxCapacityOID  = "1.3.6.1.2.1.43.8.2.1.9"
	PrtInputCurrentLevelOID = "1.3.6.1.2.1.43.8.2.1.10"
	PrtInputStatusOID       = "1.3.6.1.2.1.43.8.2.1.11"
	PrtInputMediaNameOID    = "1.3.6.1.2.1.43.8.2.1.12"
	PrtInputNameOID         = "1.3.6.1.2.1.43.8.2.1.13"
	PrtInputDescriptionOID  = "1.3.6.1.2.1.43.8.2.1.18"

	// prtMarkerTable (1.3.6.1.2.1.43.10.2) - indexed by (hrDeviceIndex, prtMarkerIndex)
	PrtMarkerTableOID   = "1.3.6.1.2.1.43.10.2"
	PrtMarkerEntryOID   = "1.3.6.1.2.1.43.10.2.1"
	MarkerIndexOID      = "1.3.6.1.2.1.43.10.2.1.1"
	MarkerMarkTechOID   = "1.3.6.1.2.1.43.10.2.1.2"
	MarkerPowerOnOID    = "1.3.6.1.2.1.43.10.2.1.5"
	MarkerStatusOID     = "1.3.6.1.2.1.43.10.2.1.15"
	DefaultMarkerRowIns = "1.1"

	// prtMarkerSuppliesTable (1.3.6.1.2.1.43.11.1) - indexed by (hrDeviceIndex, prtMarkerSuppliesIndex)
	PrtMarkerSuppliesTableOID         = "1.3.6.1.2.1.43.11.1"
	PrtMarkerSuppliesEntryOID         = "1.3.6.1.2.1.43.11.1.1"
	PrtMarkerSuppliesIndexOID         = "1.3.6.1.2.1.43.11.1.1.1"
	PrtMarkerSuppliesMarkerIndexOID   = "1.3.6.1.2.1.43.11.1.1.2"
	PrtMarkerSuppliesColorantIndexOID = "1.3.6.1.2.1.43.11.1.1.3"
	PrtMarkerSuppliesClassOID         = "1.3.6.1.2.1.43.11.1.1.4"
	PrtMarkerSuppliesTypeOID          = "1.3.6.1.2.1.43.11.1.1.5"
	PrtMarkerSuppliesDescriptionOID   = "1.3.6.1.2.1.43.11.1.1.6"
	PrtMarkerSuppliesSupplyUnitOID    = "1.3.6.1.2.1.43.11.1.1.7"
	PrtMarkerSuppliesMaxCapacityOID   = "1.3.6.1.2.1.43.11.1.1.8"
	PrtMarkerSuppliesLevelOID         = "1.3.6.1.2.1.43.11.1.1.9"

	// prtMarkerColorantTable (1.3.6.1.2.1.43.12.1)
	PrtMarkerColorantTableOID = "1.3.6.1.2.1.43.12.1"
	PrtMarkerColorantEntryOID = "1.3.6.1.2.1.43.12.1.1"
	PrtMarkerColorantValueOID = "1.3.6.1.2.1.43.12.1.1.4"

	// prtConsoleDisplayBufferTable (1.3.6.1.2.1.43.16.5)
	PrtConsoleDisplayBufferTableOID = "1.3.6.1.2.1.43.16.5"
	PrtConsoleDisplayBufferEntryOID = "1.3.6.1.2.1.43.16.5.1"
	PrtConsoleDisplayBufferIndexOID = "1.3.6.1.2.1.43.16.5.1.1"
	PrtConsoleDisplayBufferTextOID  = "1.3.6.1.2.1.43.16.5.1.2"

	// prtAlertTable (1.3.6.1.2.1.43.18.1)
	PrtAlertTableOID         = "1.3.6.1.2.1.43.18.1"
	PrtAlertEntryOID         = "1.3.6.1.2.1.43.18.1.1"
	PrtAlertIndexOID         = "1.3.6.1.2.1.43.18.1.1.1"
	PrtAlertSeverityLevelOID = "1.3.6.1.2.1.43.18.1.1.2"
	PrtAlertGroupOID         = "1.3.6.1.2.1.43.18.1.1.4"
	PrtAlertGroupIndexOID    = "1.3.6.1.2.1.43.18.1.1.5"
	PrtAlertLocationOID      = "1.3.6.1.2.1.43.18.1.1.6"
	PrtAlertCodeOID          = "1.3.6.1.2.1.43.18.1.1.7"
	PrtAlertDescriptionOID   = "1.3.6.1.2.1.43.18.1.1.8"
	PrtAlertTimeOID          = "1.3.6.1.2.1.43.18.1.1.9"
)

// PrinterMIB holds the symbol-to-OID and OID-to-symbol mappings loaded from Printer-MIB.yaml.
type PrinterMIB struct {
	Symbols map[string]string
	entries []mibEntry
}

type mibEntry struct {
	Symbol string
	OID    string
}

// MIBResolution describes a resolved Printer-MIB OID and its table/instance context.
type MIBResolution struct {
	Symbol   string `json:"symbol"`
	BaseOID  string `json:"base_oid"`
	Table    string `json:"table,omitempty"`
	Entry    string `json:"entry,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// DecodedVarBind enriches a raw SNMP VarBind with Printer-MIB symbol metadata.
type DecodedVarBind struct {
	OID      string `json:"oid"`
	Symbol   string `json:"symbol,omitempty"`
	Table    string `json:"table,omitempty"`
	Instance string `json:"instance,omitempty"`
	Type     string `json:"type"`
	Value    any    `json:"value"`
}

// SupplyRow represents a joined row from prtMarkerSuppliesTable (1.3.6.1.2.1.43.11.1.1).
type SupplyRow struct {
	Instance    string   `json:"instance"`
	Description string   `json:"description"`
	Class       int64    `json:"class,omitempty"`
	Type        int64    `json:"type,omitempty"`
	Unit        int64    `json:"supply_unit,omitempty"`
	MaxCapacity int64    `json:"max_capacity"`
	Level       int64    `json:"level"`
	Percent     *float64 `json:"percent,omitempty"`
}

// InputRow represents a joined row from prtInputTable (1.3.6.1.2.1.43.8.2.1).
type InputRow struct {
	Instance     string `json:"instance"`
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	MediaName    string `json:"media_name,omitempty"`
	CapacityUnit int64  `json:"capacity_unit,omitempty"`
	MaxCapacity  int64  `json:"max_capacity"`
	CurrentLevel int64  `json:"current_level"`
	Status       int64  `json:"status"`
}

// CoverRow represents a joined row from prtCoverTable (1.3.6.1.2.1.43.6.1.1).
type CoverRow struct {
	Instance    string `json:"instance"`
	Description string `json:"description"`
	Status      int64  `json:"status"`
}

// ConsoleLine represents a row from prtConsoleDisplayBufferTable (1.3.6.1.2.1.43.16.5.1).
type ConsoleLine struct {
	Instance string `json:"instance"`
	Text     string `json:"text"`
}

// AlertRow represents a row from prtAlertTable (1.3.6.1.2.1.43.18.1.1).
type AlertRow struct {
	Instance    string `json:"instance"`
	Severity    int64  `json:"severity,omitempty"`
	Group       int64  `json:"group,omitempty"`
	Code        int64  `json:"code,omitempty"`
	Description string `json:"description,omitempty"`
	Time        int64  `json:"time,omitempty"`
}

// PrinterMIBSnapshot is a structured, transport-neutral representation of a printer's
// SNMP MIB state decoded using Printer-MIB.yaml.
type PrinterMIBSnapshot struct {
	SchemaVersion      int              `json:"schema_version"`
	SyntheticFixture   bool             `json:"synthetic_fixture,omitempty"`
	Model              string           `json:"model,omitempty"`
	Manufacturer       string           `json:"manufacturer,omitempty"`
	VerificationStatus string           `json:"verification_status,omitempty"`
	Host               string           `json:"host,omitempty"`
	Name               string           `json:"name,omitempty"`
	PrinterName        string           `json:"printer_name,omitempty"`
	Description        string           `json:"description,omitempty"`
	SysObjectID        string           `json:"sys_object_id,omitempty"`
	Serial             string           `json:"serial,omitempty"`
	DefaultMarkerIndex int64            `json:"default_marker_index,omitempty"`
	Markers            []MarkerRow      `json:"markers,omitempty"`
	Supplies           []SupplyRow      `json:"supplies,omitempty"`
	Inputs             []InputRow       `json:"inputs,omitempty"`
	Covers             []CoverRow       `json:"covers,omitempty"`
	ConsoleLines       []ConsoleLine    `json:"console_lines,omitempty"`
	Alerts             []AlertRow       `json:"alerts,omitempty"`
	DecodedOIDs        []DecodedVarBind `json:"decoded_oids,omitempty"`
}

// LoadPrinterMIB parses a Printer-MIB YAML mapping (symbol -> OID).
func LoadPrinterMIB(r io.Reader) (*PrinterMIB, error) {
	raw := make(map[string]string)
	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode Printer-MIB YAML: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("Printer-MIB YAML is empty")
	}
	return NewPrinterMIB(raw), nil
}

// LoadPrinterMIBFile loads Printer-MIB.yaml from disk.
func LoadPrinterMIBFile(path string) (*PrinterMIB, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open Printer-MIB file %q: %w", path, err)
	}
	defer file.Close()
	return LoadPrinterMIB(file)
}

// NewPrinterMIB constructs a lookup dictionary from a symbol->OID map.
func NewPrinterMIB(symbols map[string]string) *PrinterMIB {
	normalized := make(map[string]string, len(symbols))
	entries := make([]mibEntry, 0, len(symbols))
	for sym, oid := range symbols {
		cleanSym := strings.TrimSpace(sym)
		cleanOID := strings.TrimPrefix(strings.TrimSpace(oid), ".")
		if cleanSym == "" || cleanOID == "" {
			continue
		}
		normalized[cleanSym] = cleanOID
		entries = append(entries, mibEntry{Symbol: cleanSym, OID: cleanOID})
	}
	// Sort by OID length descending (and lexicographically for ties) so longest prefix wins.
	sort.Slice(entries, func(i, j int) bool {
		if len(entries[i].OID) != len(entries[j].OID) {
			return len(entries[i].OID) > len(entries[j].OID)
		}
		return entries[i].Symbol < entries[j].Symbol
	})
	return &PrinterMIB{Symbols: normalized, entries: entries}
}

// OID returns the numeric OID for a symbol name from Printer-MIB.yaml.
func (m *PrinterMIB) OID(symbol string) (string, bool) {
	if m == nil {
		return "", false
	}
	oid, ok := m.Symbols[symbol]
	return oid, ok
}

// ResolveOID matches an instance OID (e.g. 1.3.6.1.2.1.43.10.2.1.4.1.1) against Printer-MIB.yaml.
func (m *PrinterMIB) ResolveOID(oid string) (MIBResolution, bool) {
	if m == nil {
		return MIBResolution{}, false
	}
	clean := strings.TrimPrefix(strings.TrimSpace(oid), ".")
	if clean == "" {
		return MIBResolution{}, false
	}
	var matched mibEntry
	var instance string
	found := false
	for _, entry := range m.entries {
		if clean == entry.OID {
			matched = entry
			instance = ""
			found = true
			break
		}
		if strings.HasPrefix(clean, entry.OID+".") {
			matched = entry
			instance = strings.TrimPrefix(clean, entry.OID+".")
			found = true
			break
		}
	}
	if !found {
		return MIBResolution{}, false
	}
	res := MIBResolution{
		Symbol:   matched.Symbol,
		BaseOID:  matched.OID,
		Instance: instance,
	}
	// Find enclosing Entry and Table if this OID belongs to a table.
	for _, entry := range m.entries {
		if strings.HasSuffix(entry.Symbol, "Entry") && (matched.OID == entry.OID || strings.HasPrefix(matched.OID, entry.OID+".")) {
			if res.Entry == "" {
				res.Entry = entry.Symbol
			}
		}
		if strings.HasSuffix(entry.Symbol, "Table") && (matched.OID == entry.OID || strings.HasPrefix(matched.OID, entry.OID+".")) {
			if res.Table == "" {
				res.Table = entry.Symbol
			}
		}
	}
	return res, true
}

// CounterUnitName translates Printer-MIB PrtMarkerCounterUnitTC (RFC 3805) integer codes.
func CounterUnitName(unit int64) string {
	switch unit {
	case 3:
		return "tenThousandthsOfInches"
	case 4:
		return "micrometers"
	case 5:
		return "characters"
	case 6:
		return "lines"
	case 7:
		return "impressions"
	case 8:
		return "sheets"
	case 9:
		return "dotRow"
	case 11:
		return "hours"
	case 16:
		return "feet"
	case 17:
		return "meters"
	case 18:
		return "items"
	case 19:
		return "percent"
	default:
		return fmt.Sprintf("unit(%d)", unit)
	}
}

// TryReadMarkerByInstance performs a single-packet SNMP GET for both marker life and unit
// on a known instance (e.g. "1.1"). This optimizes polling from 2 full table WALKs down to 1 GET PDU.
func TryReadMarkerByInstance(ctx context.Context, client Client, lifeColumnOID, unitColumnOID, instance string) (MarkerRow, bool, error) {
	cleanInstance := strings.Trim(strings.TrimSpace(instance), ".")
	if cleanInstance == "" {
		return MarkerRow{}, false, nil
	}
	lifeBase := strings.TrimPrefix(strings.TrimSpace(lifeColumnOID), ".")
	unitBase := strings.TrimPrefix(strings.TrimSpace(unitColumnOID), ".")
	if lifeBase == "" || unitBase == "" {
		return MarkerRow{}, false, nil
	}
	lifeOID := lifeBase + "." + cleanInstance
	unitOID := unitBase + "." + cleanInstance
	values, err := client.Get(ctx, []string{lifeOID, unitOID})
	if err != nil {
		// Some printer SNMP agents fail the entire PDU if one OID is unsupported.
		// Fallback: try reading only lifeOID
		lifeOnly, lErr := client.Get(ctx, []string{lifeOID})
		if lErr == nil && len(lifeOnly) == 1 && !IsExceptionVarBind(lifeOnly[0]) {
			if val, ok := integerValue(lifeOnly[0].Value); ok {
				return MarkerRow{Instance: cleanInstance, Life: val, Unit: 8, UnitName: "impressions"}, true, nil
			}
		}
		return MarkerRow{}, false, err
	}
	var lifeBinds, unitBinds []VarBind
	for _, vb := range values {
		if IsExceptionVarBind(vb) {
			continue
		}
		cleanOID := strings.TrimPrefix(vb.OID, ".")
		if cleanOID == lifeOID {
			lifeBinds = append(lifeBinds, vb)
		} else if cleanOID == unitOID {
			unitBinds = append(unitBinds, vb)
		}
	}
	rows := JoinMarkerRows(lifeBinds, unitBinds, lifeBase, unitBase)
	if len(rows) == 1 && rows[0].Instance == cleanInstance {
		return rows[0], true, nil
	}
	if len(lifeBinds) == 1 {
		if val, ok := integerValue(lifeBinds[0].Value); ok {
			return MarkerRow{Instance: cleanInstance, Life: val, Unit: 8, UnitName: "impressions"}, true, nil
		}
	}
	return MarkerRow{}, false, nil
}
