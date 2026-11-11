package discovery

import (
	"context"
	"fmt"
	"strings"

	"npms/backend/internal/snmp"
)

const (
	SysDescrOID             = snmp.SysDescrOID
	SysObjectOID            = snmp.SysObjectOID
	SysNameOID              = snmp.SysNameOID
	SysLocationOID          = snmp.SysLocationOID
	SerialOID               = snmp.PrtGeneralSerialNumberOID
	SerialInstance1OID      = snmp.PrtGeneralSerialNumberInstance1OID
	PrinterNameInstance1OID = snmp.PrtGeneralPrinterNameInstance1OID
)

type Result struct {
	Address          string           `json:"address"`
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	SysObjectID      string           `json:"sys_object_id"`
	Serial           string           `json:"serial"`
	MAC              string           `json:"mac,omitempty"`
	Location         string           `json:"location,omitempty"`
	Manufacturer     string           `json:"manufacturer,omitempty"`
	Model            string           `json:"model,omitempty"`
	MatchedProfileID string           `json:"matched_profile_id,omitempty"`
	Markers          []snmp.MarkerRow `json:"markers,omitempty"`
	DeviceType       string           `json:"device_type"`
}

type ClientFactory func(snmp.Config) (snmp.Client, error)

func Probe(ctx context.Context, config snmp.Config, factory ClientFactory) (Result, error) {
	if factory == nil {
		return Result{}, fmt.Errorf("SNMP client factory is required")
	}
	client, err := factory(config)
	if err != nil {
		return Result{}, err
	}
	defer client.Close()
	values, err := client.Get(ctx, []string{
		SysDescrOID,
		SysObjectOID,
		SysNameOID,
		SysLocationOID,
		PrinterNameInstance1OID,
		SerialInstance1OID,
		SerialOID,
	})
	if err != nil {
		return Result{}, err
	}
	result := Result{Address: config.Host}
	result.Description = lookup(values, SysDescrOID)
	result.SysObjectID = strings.TrimPrefix(lookup(values, SysObjectOID), ".")
	printerName := lookup(values, PrinterNameInstance1OID)
	result.Name = lookup(values, SysNameOID)
	if result.Name == "" {
		result.Name = printerName
	}
	result.Location = lookup(values, SysLocationOID)
	result.Manufacturer, result.Model = InferPrinterModelAndVendor(result.SysObjectID, printerName, result.Description)
	result.Serial = lookup(values, SerialInstance1OID)
	if result.Serial == "" {
		result.Serial = lookup(values, SerialOID)
	}
	if result.Serial == "" {
		if serialWalk, walkErr := client.Walk(ctx, SerialOID); walkErr == nil {
			for _, vb := range serialWalk {
				if !snmp.IsExceptionVarBind(vb) {
					if s, ok := snmp.StringValue(vb.Value); ok && s != "" {
						result.Serial = s
						break
					}
				}
			}
		}
	}
	result.Serial = strings.TrimSpace(result.Serial)

	// Attempt to retrieve hardware MAC address via MIB-II ifPhysAddress
	if macWalk, macErr := client.Walk(ctx, "1.3.6.1.2.1.2.2.1.6"); macErr == nil {
		for _, vb := range macWalk {
			if !snmp.IsExceptionVarBind(vb) {
				if b, ok := vb.Value.([]byte); ok && len(b) == 6 {
					result.MAC = fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", b[0], b[1], b[2], b[3], b[4], b[5])
					break
				}
			}
		}
	}

	// Identity is the primary discovery result. Marker WALK is best effort so
	// printers without a readable Printer-MIB remain discoverable.
	life, lifeErr := client.Walk(ctx, snmp.MarkerLifeOID)
	units, unitErr := client.Walk(ctx, snmp.MarkerUnitOID)
	if lifeErr == nil && unitErr == nil {
		markers := snmp.JoinMarkerRows(life, units, snmp.MarkerLifeOID, snmp.MarkerUnitOID)
		for i := range markers {
			if markers[i].UnitName == "" {
				markers[i].UnitName = snmp.CounterUnitName(markers[i].Unit)
			}
		}
		result.Markers = markers
	}
	result.DeviceType = ClassifyDeviceType(result.Manufacturer, result.Model, result.Name, result.Description, result.SysObjectID, result.MatchedProfileID, len(result.Markers))
	return result, nil
}

// InferPrinterModelAndVendor derives a normalized manufacturer and model name
// from standard MIB-II and Printer-MIB identity fields using active discovery rules.
func InferPrinterModelAndVendor(sysObjectID, printerName, sysDescr string) (manufacturer, model string) {
	cleanPn := strings.TrimSpace(printerName)
	descrModel := extractModelFromSysDescr(sysDescr)
	if cleanPn != "" && (strings.Contains(cleanPn, " ") || descrModel == "") {
		model = trimSeriesSuffix(cleanPn)
	} else if descrModel != "" {
		model = descrModel
	}

	rules := GetGlobalRules()
	oid := strings.TrimPrefix(strings.TrimSpace(sysObjectID), ".")

	// Match via vendor OID prefixes (longest match first or exact match)
	for prefix, vendorName := range rules.VendorOIDs {
		cleanPrefix := strings.TrimPrefix(prefix, ".")
		if oid == cleanPrefix || strings.HasPrefix(oid, cleanPrefix+".") {
			manufacturer = vendorName
			break
		}
	}

	// Fallback to substring matching in model and description
	if manufacturer == "" {
		combined := strings.ToLower(model + " " + sysDescr)
		for vendorName, keywords := range rules.VendorKeywords {
			for _, kw := range keywords {
				if strings.Contains(combined, strings.ToLower(kw)) {
					manufacturer = vendorName
					break
				}
			}
			if manufacturer != "" {
				break
			}
		}
	}
	return manufacturer, model
}

func trimSeriesSuffix(s string) string {
	trimmed := strings.TrimSpace(s)
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, " series") {
		return strings.TrimSpace(trimmed[:len(trimmed)-len(" series")])
	}
	return trimmed
}

func extractModelFromSysDescr(sysDescr string) string {
	desc := strings.TrimSpace(sysDescr)
	if desc == "" {
		return ""
	}
	rules := GetGlobalRules()
	for _, pfx := range rules.ModelExtraction.AdapterPrefixes {
		if strings.HasPrefix(desc, pfx) {
			parts := strings.Split(desc, ",")
			return strings.TrimSpace(parts[0])
		}
	}
	for _, token := range strings.FieldsFunc(desc, func(r rune) bool { return r == ',' || r == ';' }) {
		part := strings.TrimSpace(token)
		upper := strings.ToUpper(part)
		for _, prefix := range rules.ModelExtraction.TokenPrefixes {
			if strings.HasPrefix(upper, prefix) {
				return trimSeriesSuffix(strings.TrimSpace(part[len(prefix):]))
			}
		}
	}
	if idx := strings.Index(desc, " / "); idx > 0 {
		first := strings.TrimSpace(desc[:idx])
		// Strip trailing firmware version like "RICOH IM 2702 1.05"
		fields := strings.Fields(first)
		if len(fields) >= 3 && isVersionToken(fields[len(fields)-1]) {
			return strings.Join(fields[:len(fields)-1], " ")
		}
		return first
	}
	return ""
}

func isVersionToken(s string) bool {
	hasDot := false
	for _, r := range s {
		if r == '.' {
			hasDot = true
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return hasDot
}

func lookup(values []snmp.VarBind, oid string) string {
	for _, value := range values {
		if strings.TrimPrefix(value.OID, ".") != oid {
			continue
		}
		if snmp.IsExceptionVarBind(value) {
			continue
		}
		if s, ok := snmp.StringValue(value.Value); ok {
			return s
		}
	}
	return ""
}

// ClassifyDeviceType determines whether a discovered device is a "scanner" or "printer".
func ClassifyDeviceType(manufacturer, model, name, descr, sysObjectID, matchedProfileID string, markerCount int) string {
	combined := strings.ToLower(strings.Join([]string{manufacturer, model, name, descr, sysObjectID, matchedProfileID}, " "))

	// Check matched profile ID or name containing scanner terms
	if strings.Contains(strings.ToLower(matchedProfileID), "scan") || strings.Contains(strings.ToLower(matchedProfileID), "brother-ads") {
		return "scanner"
	}

	rules := GetGlobalRules()

	// Dedicated scanner keywords from active rules
	for _, kw := range rules.ScannerKeywords {
		if strings.Contains(combined, strings.ToLower(kw)) {
			return "scanner"
		}
	}

	// Common scanner model prefixes from active rules (e.g. ADS, FI, DR, KV-S)
	upperModel := strings.ToUpper(strings.TrimSpace(model))
	for _, pfx := range rules.ScannerModelPrefixes {
		if strings.HasPrefix(upperModel, strings.ToUpper(pfx)) {
			return "scanner"
		}
	}

	return "printer"
}
