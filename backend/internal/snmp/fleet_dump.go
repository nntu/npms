package snmp

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type CandidateCounter struct {
	Key    string `json:"key"`
	OID    string `json:"oid"`
	Value  int64  `json:"value"`
	Unit   string `json:"unit"`
	Source string `json:"source"`
}

type FleetPrinterDump struct {
	ID                string             `json:"id,omitempty"`
	DisplayName       string             `json:"display_name,omitempty"`
	Address           string             `json:"address"`
	Port              int                `json:"port"`
	SysDescr          string             `json:"sys_descr,omitempty"`
	SysObjectID       string             `json:"sys_object_id,omitempty"`
	SysName           string             `json:"sys_name,omitempty"`
	SysLocation       string             `json:"sys_location,omitempty"`
	Manufacturer      string             `json:"manufacturer,omitempty"`
	Model             string             `json:"model,omitempty"`
	Serial            string             `json:"serial,omitempty"`
	CurrentProfileID  string             `json:"current_profile_id,omitempty"`
	Status            string             `json:"status"` // "online", "unreachable", "error"
	Error             string             `json:"error,omitempty"`
	Markers           []MarkerRow        `json:"markers,omitempty"`
	CandidateCounters []CandidateCounter `json:"candidate_counters,omitempty"`
	RawMarkerBinds    []VarBind          `json:"raw_marker_binds,omitempty"`
	RawVendorBinds    []VarBind          `json:"raw_vendor_binds,omitempty"`
	Config            *Config            `json:"-"`
}

type FleetDumpReport struct {
	ExportedAt   string             `json:"exported_at"`
	PrinterCount int                `json:"printer_count"`
	OnlineCount  int                `json:"online_count"`
	Printers     []FleetPrinterDump `json:"printers"`
}

// AnonymizeFleetReport masks IP addresses, hostnames, and serial numbers.
func AnonymizeFleetReport(report FleetDumpReport) FleetDumpReport {
	for i := range report.Printers {
		p := &report.Printers[i]
		if len(p.Serial) > 3 {
			p.Serial = p.Serial[:3] + strings.Repeat("*", len(p.Serial)-3)
		}
		if p.Address != "" {
			parts := strings.Split(p.Address, ".")
			if len(parts) == 4 {
				p.Address = parts[0] + "." + parts[1] + ".xxx.xxx"
			}
		}
	}
	return report
}

// Well-known vendor OIDs for page counters
var wellKnownVendorPageCounterOIDs = []struct {
	Vendor string
	Name   string
	OID    string
	Unit   string
}{
	// HP
	{"HP", "hp_total_page_count", "1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.6.0", "pages"},
	{"HP", "hp_duplex_page_count", "1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.7.0", "pages"},
	{"HP", "hp_page_count_alt", "1.3.6.1.4.1.11.2.3.9.4.2.1.1.1.2.1.0", "pages"},
	{"HP", "hp_engine_cycles", "1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.5.0", "impressions"},

	// Ricoh
	{"Ricoh", "ricoh_total_engine_counter", "1.3.6.1.4.1.367.3.2.1.2.19.1.0", "impressions"},
	{"Ricoh", "ricoh_print_counter", "1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.1", "pages"},
	{"Ricoh", "ricoh_copy_counter", "1.3.6.1.4.1.367.3.2.1.2.19.5.1.9.2", "pages"},

	// Brother
	{"Brother", "brother_total_page_count", "1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.8.0", "pages"},
	{"Brother", "brother_page_count_alt", "1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.1.0", "pages"},
	{"Brother", "brother_page_count_alt2", "1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.10.0", "pages"},

	// Canon
	{"Canon", "canon_total_impressions", "1.3.6.1.4.1.1602.1.11.1.3.1.4.101", "impressions"},
	{"Canon", "canon_total_copy", "1.3.6.1.4.1.1602.1.11.1.3.1.4.102", "pages"},

	// Xerox
	{"Xerox", "xerox_total_impressions", "1.3.6.1.4.1.253.8.53.13.2.1.6.1.20.1", "impressions"},

	// Kyocera
	{"Kyocera", "kyocera_total_pages", "1.3.6.1.4.1.1347.42.2.1.1.1.6.1.1", "pages"},

	// Konica Minolta
	{"Konica", "konica_total_counter", "1.3.6.1.4.1.18334.1.1.1.5.7.2.2.1.5.1.1", "pages"},
}

// Vendor subtrees to walk if sysObjectID or vendor matches
var vendorSubtrees = map[string][]string{
	"11":    {"1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2"},                                 // HP
	"367":   {"1.3.6.1.4.1.367.3.2.1.2.19.1", "1.3.6.1.4.1.367.3.2.1.2.19.5.1.9"}, // Ricoh
	"2435":  {"1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5"},                                 // Brother
	"1602":  {"1.3.6.1.4.1.1602.1.11.1.3.1.4"},                                    // Canon
	"253":   {"1.3.6.1.4.1.253.8.53.13.2.1.6.1"},                                  // Xerox
	"1347":  {"1.3.6.1.4.1.1347.42.2.1.1.1.6"},                                    // Kyocera
	"18334": {"1.3.6.1.4.1.18334.1.1.1.5.7.2.2.1.5"},                              // Konica Minolta
}

// DumpPrinterCounters queries an individual printer via SNMP and extracts all page counter information.
func DumpPrinterCounters(ctx context.Context, client Client, dump *FleetPrinterDump) {
	dump.Status = "unreachable"

	// 1. Get basic system variables
	sysOIDs := []string{
		SysDescrOID,
		SysObjectOID,
		SysNameOID,
		SysLocationOID,
		PrtGeneralPrinterNameInstance1OID,
		PrtGeneralSerialNumberInstance1OID,
	}

	sysBinds, err := client.Get(ctx, sysOIDs)
	if err != nil {
		dump.Error = fmt.Sprintf("system probe failed: %v", err)
		return
	}
	dump.Status = "online"

	for _, vb := range sysBinds {
		if IsExceptionVarBind(vb) {
			continue
		}
		clean := strings.TrimPrefix(vb.OID, ".")
		val, _ := StringValue(vb.Value)
		switch clean {
		case SysDescrOID:
			dump.SysDescr = val
		case SysObjectOID:
			dump.SysObjectID = strings.TrimPrefix(val, ".")
		case SysNameOID:
			dump.SysName = val
		case SysLocationOID:
			dump.SysLocation = val
		case PrtGeneralPrinterNameInstance1OID:
			if dump.Model == "" {
				dump.Model = val
			}
		case PrtGeneralSerialNumberInstance1OID:
			if dump.Serial == "" {
				dump.Serial = val
			}
		}
	}

	// Infer Manufacturer and Model if empty
	lowerDescr := strings.ToLower(dump.SysDescr)
	switch {
	case strings.Contains(lowerDescr, "hp") || strings.Contains(lowerDescr, "hewlett-packard") || strings.HasPrefix(dump.SysObjectID, "1.3.6.1.4.1.11."):
		dump.Manufacturer = "HP"
	case strings.Contains(lowerDescr, "ricoh") || strings.HasPrefix(dump.SysObjectID, "1.3.6.1.4.1.367."):
		dump.Manufacturer = "Ricoh"
	case strings.Contains(lowerDescr, "brother") || strings.HasPrefix(dump.SysObjectID, "1.3.6.1.4.1.2435."):
		dump.Manufacturer = "Brother"
	case strings.Contains(lowerDescr, "canon") || strings.HasPrefix(dump.SysObjectID, "1.3.6.1.4.1.1602."):
		dump.Manufacturer = "Canon"
	case strings.Contains(lowerDescr, "xerox") || strings.HasPrefix(dump.SysObjectID, "1.3.6.1.4.1.253."):
		dump.Manufacturer = "Xerox"
	case strings.Contains(lowerDescr, "kyocera") || strings.HasPrefix(dump.SysObjectID, "1.3.6.1.4.1.1347."):
		dump.Manufacturer = "Kyocera"
	case strings.Contains(lowerDescr, "konica") || strings.HasPrefix(dump.SysObjectID, "1.3.6.1.4.1.18334."):
		dump.Manufacturer = "Konica Minolta"
	}
	if dump.Model == "" && dump.SysDescr != "" {
		dump.Model = dump.SysDescr
	}

	// 2. Walk Printer-MIB Marker Table (1.3.6.1.2.1.43.10)
	lifeBinds, _ := client.Walk(ctx, MarkerLifeOID)
	unitBinds, _ := client.Walk(ctx, MarkerUnitOID)

	dump.RawMarkerBinds = append(dump.RawMarkerBinds, lifeBinds...)
	dump.RawMarkerBinds = append(dump.RawMarkerBinds, unitBinds...)

	markers := JoinMarkerRows(lifeBinds, unitBinds, MarkerLifeOID, MarkerUnitOID)
	for i := range markers {
		markers[i].UnitName = CounterUnitName(markers[i].Unit)
	}
	dump.Markers = markers

	// Add joined marker counters to candidate list
	seenOIDs := make(map[string]bool)
	for _, m := range markers {
		cleanOID := MarkerLifeOID + "." + m.Instance
		seenOIDs[cleanOID] = true
		dump.CandidateCounters = append(dump.CandidateCounters, CandidateCounter{
			Key:    "marker_life." + m.Instance,
			OID:    cleanOID,
			Value:  m.Life,
			Unit:   m.UnitName,
			Source: "Printer-MIB (prtMarkerLifeCount)",
		})
	}

	// If no marker rows joined but lifeBinds has rows (e.g. unit column missing)
	for _, vb := range lifeBinds {
		if IsExceptionVarBind(vb) {
			continue
		}
		clean := strings.TrimPrefix(vb.OID, ".")
		if !seenOIDs[clean] {
			if n, ok := toInt64(vb.Value); ok && n >= 0 {
				seenOIDs[clean] = true
				dump.CandidateCounters = append(dump.CandidateCounters, CandidateCounter{
					Key:    "marker_life_raw." + strings.TrimPrefix(clean, MarkerLifeOID+"."),
					OID:    clean,
					Value:  n,
					Unit:   "impressions",
					Source: "Printer-MIB (prtMarkerLifeCount fallback)",
				})
			}
		}
	}

	// 3. Probe well-known vendor page counter OIDs
	var vendorGetOIDs []string
	for _, wk := range wellKnownVendorPageCounterOIDs {
		vendorGetOIDs = append(vendorGetOIDs, wk.OID)
	}
	vendorBinds, _ := client.Get(ctx, vendorGetOIDs)
	for _, vb := range vendorBinds {
		if IsExceptionVarBind(vb) {
			continue
		}
		clean := strings.TrimPrefix(vb.OID, ".")
		if n, ok := toInt64(vb.Value); ok && n > 0 {
			dump.RawVendorBinds = append(dump.RawVendorBinds, vb)
			var name, unit, source string
			for _, wk := range wellKnownVendorPageCounterOIDs {
				if wk.OID == clean {
					name = wk.Name
					unit = wk.Unit
					source = wk.Vendor + "-Vendor"
					break
				}
			}
			if name == "" {
				name = "vendor_counter." + clean
				unit = "pages"
				source = "Vendor"
			}
			if !seenOIDs[clean] {
				seenOIDs[clean] = true
				dump.CandidateCounters = append(dump.CandidateCounters, CandidateCounter{
					Key:    name,
					OID:    clean,
					Value:  n,
					Unit:   unit,
					Source: source,
				})
			}
		}
	}

	// 4. Walk vendor subtree if enterprise ID matches
	enterpriseID := extractEnterpriseID(dump.SysObjectID)
	if subtrees, ok := vendorSubtrees[enterpriseID]; ok {
		for _, subtree := range subtrees {
			treeBinds, err := client.Walk(ctx, subtree)
			if err != nil {
				continue
			}
			for _, vb := range treeBinds {
				if IsExceptionVarBind(vb) {
					continue
				}
				clean := strings.TrimPrefix(vb.OID, ".")
				dump.RawVendorBinds = append(dump.RawVendorBinds, vb)
				if n, ok := toInt64(vb.Value); ok && n > 0 && !seenOIDs[clean] {
					seenOIDs[clean] = true
					dump.CandidateCounters = append(dump.CandidateCounters, CandidateCounter{
						Key:    "vendor_walk." + clean,
						OID:    clean,
						Value:  n,
						Unit:   "pages",
						Source: "Vendor Subtree (" + enterpriseID + ")",
					})
				}
			}
		}
	}

	// Sort candidate counters by value descending to highlight prominent counters
	sort.Slice(dump.CandidateCounters, func(i, j int) bool {
		return dump.CandidateCounters[i].Value > dump.CandidateCounters[j].Value
	})
}

func extractEnterpriseID(sysObjectID string) string {
	clean := strings.TrimPrefix(sysObjectID, ".")
	const prefix = "1.3.6.1.4.1."
	if strings.HasPrefix(clean, prefix) {
		rem := strings.TrimPrefix(clean, prefix)
		parts := strings.Split(rem, ".")
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return ""
}

func toInt64(v any) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int32:
		return int64(val), true
	case int64:
		return val, true
	case uint:
		return int64(val), true
	case uint32:
		return int64(val), true
	case uint64:
		return int64(val), true
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

// GenerateFleetDumpReport runs DumpPrinterCounters across a slice of printer targets.
func GenerateFleetDumpReport(ctx context.Context, clientFactory func(Config) (Client, error), targets []FleetPrinterDump, concurrency int) FleetDumpReport {
	if concurrency < 1 {
		concurrency = 5
	}
	if concurrency > len(targets) && len(targets) > 0 {
		concurrency = len(targets)
	}

	report := FleetDumpReport{
		ExportedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		PrinterCount: len(targets),
		Printers:     make([]FleetPrinterDump, len(targets)),
	}

	type workItem struct {
		index int
		dump  FleetPrinterDump
	}

	workCh := make(chan workItem, len(targets))
	resultCh := make(chan workItem, len(targets))

	for i, target := range targets {
		workCh <- workItem{index: i, dump: target}
	}
	close(workCh)

	for w := 0; w < concurrency; w++ {
		go func() {
			for item := range workCh {
				cfg := Config{
					Host:           item.dump.Address,
					Port:           uint16(item.dump.Port),
					Version:        Version2c,
					Community:      "public",
					TimeoutSeconds: 3,
					Retries:        1,
					MaxRepetitions: 25,
				}
				if item.dump.Config != nil {
					cfg = *item.dump.Config
				}
				client, err := clientFactory(cfg)
				if err != nil {
					item.dump.Status = "error"
					item.dump.Error = fmt.Sprintf("create client: %v", err)
				} else {
					DumpPrinterCounters(ctx, client, &item.dump)
					_ = client.Close()
				}
				resultCh <- item
			}
		}()
	}

	for i := 0; i < len(targets); i++ {
		res := <-resultCh
		report.Printers[res.index] = res.dump
		if res.dump.Status == "online" {
			report.OnlineCount++
		}
	}

	return report
}
