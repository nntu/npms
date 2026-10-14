package snmp

import (
	"context"
	"math"
	"sort"
	"strings"
)

const (
	SysDescrOID    = "1.3.6.1.2.1.1.1.0"
	SysObjectOID   = "1.3.6.1.2.1.1.2.0"
	SysNameOID     = "1.3.6.1.2.1.1.5.0"
	SysLocationOID = "1.3.6.1.2.1.1.6.0"
)

// DefaultPrinterMIB returns the built-in Printer-MIB symbol dictionary synchronized with Printer-MIB.yaml.
func DefaultPrinterMIB() *PrinterMIB {
	return NewPrinterMIB(map[string]string{
		"sysDescr":                       SysDescrOID,
		"sysObjectID":                    SysObjectOID,
		"sysName":                        SysNameOID,
		"sysLocation":                    SysLocationOID,
		"printmib":                       PrintMIBRootOID,
		"prtGeneral":                     "1.3.6.1.2.1.43.5",
		"prtGeneralTable":                PrtGeneralTableOID,
		"prtGeneralEntry":                PrtGeneralEntryOID,
		"prtGeneralConfigChanges":        PrtGeneralConfigChangesOID,
		"prtGeneralCurrentLocalization":  PrtGeneralCurrentLocalizationOID,
		"prtGeneralReset":                PrtGeneralResetOID,
		"prtGeneralCurrentOperator":      "1.3.6.1.2.1.43.5.1.1.4",
		"prtGeneralServicePerson":        "1.3.6.1.2.1.43.5.1.1.5",
		"prtInputDefaultIndex":           PrtInputDefaultIndexOID,
		"prtOutputDefaultIndex":          PrtOutputDefaultIndexOID,
		"prtMarkerDefaultIndex":          PrtMarkerDefaultIndexOID,
		"prtMediaPathDefaultIndex":       PrtMediaPathDefaultIndexOID,
		"prtConsoleLocalization":         "1.3.6.1.2.1.43.5.1.1.10",
		"prtConsoleNumberOfDisplayLines": "1.3.6.1.2.1.43.5.1.1.11",
		"prtConsoleNumberOfDisplayChars": "1.3.6.1.2.1.43.5.1.1.12",
		"prtConsoleDisable":              "1.3.6.1.2.1.43.5.1.1.13",
		"prtAuxiliarySheetStartupPage":   "1.3.6.1.2.1.43.5.1.1.14",
		"prtAuxiliarySheetBannerPage":    "1.3.6.1.2.1.43.5.1.1.15",
		"prtGeneralPrinterName":          PrtGeneralPrinterNameOID,
		"prtGeneralSerialNumber":         PrtGeneralSerialNumberOID,
		"prtAlertCriticalEvents":         PrtAlertCriticalEventsOID,
		"prtAlertAllEvents":              PrtAlertAllEventsOID,
		"prtCover":                       "1.3.6.1.2.1.43.6",
		"prtCoverTable":                  PrtCoverTableOID,
		"prtCoverEntry":                  PrtCoverEntryOID,
		"prtCoverIndex":                  PrtCoverIndexOID,
		"prtCoverDescription":            PrtCoverDescriptionOID,
		"prtCoverStatus":                 PrtCoverStatusOID,
		"prtInput":                       "1.3.6.1.2.1.43.8",
		"prtInputTable":                  PrtInputTableOID,
		"prtInputEntry":                  PrtInputEntryOID,
		"prtInputIndex":                  PrtInputIndexOID,
		"prtInputType":                   PrtInputTypeOID,
		"prtInputCapacityUnit":           PrtInputCapacityUnitOID,
		"prtInputMaxCapacity":            PrtInputMaxCapacityOID,
		"prtInputCurrentLevel":           PrtInputCurrentLevelOID,
		"prtInputStatus":                 PrtInputStatusOID,
		"prtInputMediaName":              PrtInputMediaNameOID,
		"prtInputName":                   PrtInputNameOID,
		"prtInputDescription":            PrtInputDescriptionOID,
		"prtMarker":                      "1.3.6.1.2.1.43.10",
		"prtMarkerTable":                 PrtMarkerTableOID,
		"prtMarkerEntry":                 PrtMarkerEntryOID,
		"prtMarkerIndex":                 MarkerIndexOID,
		"prtMarkerMarkTech":              MarkerMarkTechOID,
		"prtMarkerCounterUnit":           MarkerUnitOID,
		"prtMarkerLifeCount":             MarkerLifeOID,
		"prtMarkerPowerOnCount":          MarkerPowerOnOID,
		"prtMarkerStatus":                MarkerStatusOID,
		"prtMarkerSupplies":              "1.3.6.1.2.1.43.11",
		"prtMarkerSuppliesTable":         PrtMarkerSuppliesTableOID,
		"prtMarkerSuppliesEntry":         PrtMarkerSuppliesEntryOID,
		"prtMarkerSuppliesIndex":         PrtMarkerSuppliesIndexOID,
		"prtMarkerSuppliesMarkerIndex":   PrtMarkerSuppliesMarkerIndexOID,
		"prtMarkerSuppliesColorantIndex": PrtMarkerSuppliesColorantIndexOID,
		"prtMarkerSuppliesClass":         PrtMarkerSuppliesClassOID,
		"prtMarkerSuppliesType":          PrtMarkerSuppliesTypeOID,
		"prtMarkerSuppliesDescription":   PrtMarkerSuppliesDescriptionOID,
		"prtMarkerSuppliesSupplyUnit":    PrtMarkerSuppliesSupplyUnitOID,
		"prtMarkerSuppliesMaxCapacity":   PrtMarkerSuppliesMaxCapacityOID,
		"prtMarkerSuppliesLevel":         PrtMarkerSuppliesLevelOID,
		"prtMarkerColorant":              "1.3.6.1.2.1.43.12",
		"prtMarkerColorantTable":         PrtMarkerColorantTableOID,
		"prtMarkerColorantEntry":         PrtMarkerColorantEntryOID,
		"prtMarkerColorantValue":         PrtMarkerColorantValueOID,
		"prtConsoleDisplayBuffer":        "1.3.6.1.2.1.43.16",
		"prtConsoleDisplayBufferTable":   PrtConsoleDisplayBufferTableOID,
		"prtConsoleDisplayBufferEntry":   PrtConsoleDisplayBufferEntryOID,
		"prtConsoleDisplayBufferIndex":   PrtConsoleDisplayBufferIndexOID,
		"prtConsoleDisplayBufferText":    PrtConsoleDisplayBufferTextOID,
		"prtAlert":                       "1.3.6.1.2.1.43.18",
		"prtAlertTable":                  PrtAlertTableOID,
		"prtAlertEntry":                  PrtAlertEntryOID,
		"prtAlertIndex":                  PrtAlertIndexOID,
		"prtAlertSeverityLevel":          PrtAlertSeverityLevelOID,
		"prtAlertGroup":                  PrtAlertGroupOID,
		"prtAlertGroupIndex":             PrtAlertGroupIndexOID,
		"prtAlertLocation":               PrtAlertLocationOID,
		"prtAlertCode":                   PrtAlertCodeOID,
		"prtAlertDescription":            PrtAlertDescriptionOID,
		"prtAlertTime":                   PrtAlertTimeOID,
	})
}

// CollectPrinterMIBSnapshot queries a printer using an optimized sequence based on Printer-MIB.yaml:
// 1. Single batched GET for system identity + prtGeneralEntry (.1) scalars
// 2. Targeted subtree WALKs for prtMarkerEntry, prtMarkerSuppliesEntry, prtInputEntry, prtCoverEntry, prtConsoleDisplayBufferEntry, and prtAlertEntry.
func CollectPrinterMIBSnapshot(ctx context.Context, client Client, mib *PrinterMIB) (PrinterMIBSnapshot, error) {
	if mib == nil {
		mib = DefaultPrinterMIB()
	}
	allBinds := make([]VarBind, 0, 64)

	// 1. Batched fast-path GET for identity + prtGeneralEntry (.1) + legacy serial scalar.
	identityOIDs := []string{
		SysDescrOID,
		SysObjectOID,
		SysNameOID,
		PrtGeneralPrinterNameInstance1OID,
		PrtGeneralSerialNumberInstance1OID,
		PrtGeneralSerialNumberOID,
		PrtMarkerDefaultIndexInstance1OID,
	}
	initial, err := client.Get(ctx, identityOIDs)
	if err != nil {
		return PrinterMIBSnapshot{}, err
	}
	for _, vb := range initial {
		if !IsExceptionVarBind(vb) {
			allBinds = append(allBinds, vb)
		}
	}

	// 2. Walk key Printer-MIB subtrees (best-effort per table so partial Printer-MIB implementations succeed).
	subtrees := []string{
		PrtGeneralEntryOID,
		PrtMarkerEntryOID,
		PrtMarkerSuppliesEntryOID,
		PrtInputEntryOID,
		PrtCoverEntryOID,
		PrtConsoleDisplayBufferEntryOID,
		PrtAlertEntryOID,
	}
	seenOID := make(map[string]bool, len(allBinds))
	for _, vb := range allBinds {
		seenOID[strings.TrimPrefix(vb.OID, ".")] = true
	}

	for _, subtree := range subtrees {
		walked, walkErr := client.Walk(ctx, subtree)
		if walkErr != nil {
			// Fallback for Marker table if full entry walk isn't supported by a mock/agent: walk Life & Unit columns directly.
			if subtree == PrtMarkerEntryOID {
				life, lifeErr := client.Walk(ctx, MarkerLifeOID)
				units, unitErr := client.Walk(ctx, MarkerUnitOID)
				if lifeErr == nil {
					walked = append(walked, life...)
				}
				if unitErr == nil {
					walked = append(walked, units...)
				}
			}
		}
		for _, vb := range walked {
			if IsExceptionVarBind(vb) {
				continue
			}
			clean := strings.TrimPrefix(vb.OID, ".")
			if !seenOID[clean] {
				seenOID[clean] = true
				allBinds = append(allBinds, vb)
			}
		}
	}

	return DecodePrinterMIBSnapshot(allBinds, mib), nil
}

// DecodePrinterMIBSnapshot decodes raw VarBinds into a structured PrinterMIBSnapshot using Printer-MIB.yaml.
func DecodePrinterMIBSnapshot(varbinds []VarBind, mib *PrinterMIB) PrinterMIBSnapshot {
	if mib == nil {
		mib = DefaultPrinterMIB()
	}
	snapshot := PrinterMIBSnapshot{
		SchemaVersion: 1,
		DecodedOIDs:   make([]DecodedVarBind, 0, len(varbinds)),
	}

	var lifeBinds, unitBinds []VarBind
	powerOnByIns := make(map[string]int64)
	markTechByIns := make(map[string]int64)
	statusByIns := make(map[string]int64)

	for _, vb := range varbinds {
		if IsExceptionVarBind(vb) {
			continue
		}
		cleanOID := strings.TrimPrefix(strings.TrimSpace(vb.OID), ".")
		decoded := DecodedVarBind{
			OID:   cleanOID,
			Type:  vb.Type,
			Value: NormalizeVarBindValue(vb.Value),
		}
		if res, ok := mib.ResolveOID(cleanOID); ok {
			decoded.Symbol = res.Symbol
			decoded.Table = res.Table
			decoded.Instance = res.Instance
		}
		snapshot.DecodedOIDs = append(snapshot.DecodedOIDs, decoded)

		switch {
		case cleanOID == SysDescrOID:
			if s, ok := StringValue(vb.Value); ok && snapshot.Description == "" {
				snapshot.Description = s
			}
		case cleanOID == SysObjectOID:
			if s, ok := StringValue(vb.Value); ok && snapshot.SysObjectID == "" {
				snapshot.SysObjectID = strings.TrimPrefix(s, ".")
			}
		case cleanOID == SysNameOID:
			if s, ok := StringValue(vb.Value); ok && snapshot.Name == "" {
				snapshot.Name = s
			}
		case cleanOID == PrtGeneralPrinterNameOID || strings.HasPrefix(cleanOID, PrtGeneralPrinterNameOID+"."):
			if s, ok := StringValue(vb.Value); ok && s != "" && snapshot.PrinterName == "" {
				snapshot.PrinterName = s
			}
		case cleanOID == PrtGeneralSerialNumberOID || strings.HasPrefix(cleanOID, PrtGeneralSerialNumberOID+"."):
			if s, ok := StringValue(vb.Value); ok && s != "" && snapshot.Serial == "" {
				snapshot.Serial = s
			}
		case cleanOID == PrtMarkerDefaultIndexOID || strings.HasPrefix(cleanOID, PrtMarkerDefaultIndexOID+"."):
			if n, ok := integerValue(vb.Value); ok && snapshot.DefaultMarkerIndex == 0 {
				snapshot.DefaultMarkerIndex = n
			}
		case strings.HasPrefix(cleanOID, MarkerLifeOID+"."):
			lifeBinds = append(lifeBinds, vb)
		case strings.HasPrefix(cleanOID, MarkerUnitOID+"."):
			unitBinds = append(unitBinds, vb)
		case strings.HasPrefix(cleanOID, MarkerPowerOnOID+"."):
			if ins, ok := tableInstance(cleanOID, MarkerPowerOnOID); ok {
				if n, ok := integerValue(vb.Value); ok {
					powerOnByIns[ins] = n
				}
			}
		case strings.HasPrefix(cleanOID, MarkerMarkTechOID+"."):
			if ins, ok := tableInstance(cleanOID, MarkerMarkTechOID); ok {
				if n, ok := integerValue(vb.Value); ok {
					markTechByIns[ins] = n
				}
			}
		case strings.HasPrefix(cleanOID, MarkerStatusOID+"."):
			if ins, ok := tableInstance(cleanOID, MarkerStatusOID); ok {
				if n, ok := integerValue(vb.Value); ok {
					statusByIns[ins] = n
				}
			}
		}
	}

	if snapshot.Name == "" && snapshot.PrinterName != "" {
		snapshot.Name = snapshot.PrinterName
	}

	markers := JoinMarkerRows(lifeBinds, unitBinds, MarkerLifeOID, MarkerUnitOID)
	for i := range markers {
		ins := markers[i].Instance
		markers[i].UnitName = CounterUnitName(markers[i].Unit)
		if v, ok := powerOnByIns[ins]; ok {
			markers[i].PowerOnCount = v
		}
		if v, ok := markTechByIns[ins]; ok {
			markers[i].MarkTech = v
		}
		if v, ok := statusByIns[ins]; ok {
			markers[i].Status = v
		}
	}
	snapshot.Markers = markers
	snapshot.Supplies = JoinSupplyRows(varbinds)
	snapshot.Inputs = JoinInputRows(varbinds)
	snapshot.Covers = JoinCoverRows(varbinds)
	snapshot.ConsoleLines = JoinConsoleLines(varbinds)
	snapshot.Alerts = JoinAlertRows(varbinds)

	return snapshot
}

// JoinSupplyRows joins columns of prtMarkerSuppliesEntry (1.3.6.1.2.1.43.11.1.1) by exact instance.
func JoinSupplyRows(varbinds []VarBind) []SupplyRow {
	byInstance := make(map[string]*SupplyRow)
	order := make([]string, 0)
	ensure := func(ins string) *SupplyRow {
		if row, ok := byInstance[ins]; ok {
			return row
		}
		row := &SupplyRow{Instance: ins}
		byInstance[ins] = row
		order = append(order, ins)
		return row
	}

	for _, vb := range varbinds {
		if IsExceptionVarBind(vb) {
			continue
		}
		oid := strings.TrimPrefix(vb.OID, ".")
		switch {
		case strings.HasPrefix(oid, PrtMarkerSuppliesDescriptionOID+"."):
			if ins, ok := tableInstance(oid, PrtMarkerSuppliesDescriptionOID); ok {
				if s, ok := StringValue(vb.Value); ok {
					ensure(ins).Description = s
				}
			}
		case strings.HasPrefix(oid, PrtMarkerSuppliesClassOID+"."):
			if ins, ok := tableInstance(oid, PrtMarkerSuppliesClassOID); ok {
				if n, ok := integerValue(vb.Value); ok {
					ensure(ins).Class = n
				}
			}
		case strings.HasPrefix(oid, PrtMarkerSuppliesTypeOID+"."):
			if ins, ok := tableInstance(oid, PrtMarkerSuppliesTypeOID); ok {
				if n, ok := integerValue(vb.Value); ok {
					ensure(ins).Type = n
				}
			}
		case strings.HasPrefix(oid, PrtMarkerSuppliesSupplyUnitOID+"."):
			if ins, ok := tableInstance(oid, PrtMarkerSuppliesSupplyUnitOID); ok {
				if n, ok := integerValue(vb.Value); ok {
					ensure(ins).Unit = n
				}
			}
		case strings.HasPrefix(oid, PrtMarkerSuppliesMaxCapacityOID+"."):
			if ins, ok := tableInstance(oid, PrtMarkerSuppliesMaxCapacityOID); ok {
				if n, ok := SignedIntegerValue(vb.Value); ok {
					ensure(ins).MaxCapacity = n
				}
			}
		case strings.HasPrefix(oid, PrtMarkerSuppliesLevelOID+"."):
			if ins, ok := tableInstance(oid, PrtMarkerSuppliesLevelOID); ok {
				if n, ok := SignedIntegerValue(vb.Value); ok {
					ensure(ins).Level = n
				}
			}
		}
	}

	sort.Strings(order)
	rows := make([]SupplyRow, 0, len(order))
	for _, ins := range order {
		row := byInstance[ins]
		if row.MaxCapacity > 0 && row.Level >= 0 && row.Level <= row.MaxCapacity {
			pct := math.Round((float64(row.Level)/float64(row.MaxCapacity))*1000) / 10
			row.Percent = &pct
		}
		rows = append(rows, *row)
	}
	return rows
}

// JoinInputRows joins columns of prtInputEntry (1.3.6.1.2.1.43.8.2.1) by exact instance.
func JoinInputRows(varbinds []VarBind) []InputRow {
	byInstance := make(map[string]*InputRow)
	order := make([]string, 0)
	ensure := func(ins string) *InputRow {
		if row, ok := byInstance[ins]; ok {
			return row
		}
		row := &InputRow{Instance: ins}
		byInstance[ins] = row
		order = append(order, ins)
		return row
	}
	for _, vb := range varbinds {
		if IsExceptionVarBind(vb) {
			continue
		}
		oid := strings.TrimPrefix(vb.OID, ".")
		switch {
		case strings.HasPrefix(oid, PrtInputNameOID+"."):
			if ins, ok := tableInstance(oid, PrtInputNameOID); ok {
				if s, ok := StringValue(vb.Value); ok {
					ensure(ins).Name = s
				}
			}
		case strings.HasPrefix(oid, PrtInputDescriptionOID+"."):
			if ins, ok := tableInstance(oid, PrtInputDescriptionOID); ok {
				if s, ok := StringValue(vb.Value); ok {
					ensure(ins).Description = s
				}
			}
		case strings.HasPrefix(oid, PrtInputMediaNameOID+"."):
			if ins, ok := tableInstance(oid, PrtInputMediaNameOID); ok {
				if s, ok := StringValue(vb.Value); ok {
					ensure(ins).MediaName = s
				}
			}
		case strings.HasPrefix(oid, PrtInputCapacityUnitOID+"."):
			if ins, ok := tableInstance(oid, PrtInputCapacityUnitOID); ok {
				if n, ok := integerValue(vb.Value); ok {
					ensure(ins).CapacityUnit = n
				}
			}
		case strings.HasPrefix(oid, PrtInputMaxCapacityOID+"."):
			if ins, ok := tableInstance(oid, PrtInputMaxCapacityOID); ok {
				if n, ok := SignedIntegerValue(vb.Value); ok {
					ensure(ins).MaxCapacity = n
				}
			}
		case strings.HasPrefix(oid, PrtInputCurrentLevelOID+"."):
			if ins, ok := tableInstance(oid, PrtInputCurrentLevelOID); ok {
				if n, ok := SignedIntegerValue(vb.Value); ok {
					ensure(ins).CurrentLevel = n
				}
			}
		case strings.HasPrefix(oid, PrtInputStatusOID+"."):
			if ins, ok := tableInstance(oid, PrtInputStatusOID); ok {
				if n, ok := integerValue(vb.Value); ok {
					ensure(ins).Status = n
				}
			}
		}
	}
	sort.Strings(order)
	rows := make([]InputRow, 0, len(order))
	for _, ins := range order {
		rows = append(rows, *byInstance[ins])
	}
	return rows
}

// JoinCoverRows joins columns of prtCoverEntry (1.3.6.1.2.1.43.6.1.1) by exact instance.
func JoinCoverRows(varbinds []VarBind) []CoverRow {
	byInstance := make(map[string]*CoverRow)
	order := make([]string, 0)
	ensure := func(ins string) *CoverRow {
		if row, ok := byInstance[ins]; ok {
			return row
		}
		row := &CoverRow{Instance: ins}
		byInstance[ins] = row
		order = append(order, ins)
		return row
	}
	for _, vb := range varbinds {
		if IsExceptionVarBind(vb) {
			continue
		}
		oid := strings.TrimPrefix(vb.OID, ".")
		switch {
		case strings.HasPrefix(oid, PrtCoverDescriptionOID+"."):
			if ins, ok := tableInstance(oid, PrtCoverDescriptionOID); ok {
				if s, ok := StringValue(vb.Value); ok {
					ensure(ins).Description = s
				}
			}
		case strings.HasPrefix(oid, PrtCoverStatusOID+"."):
			if ins, ok := tableInstance(oid, PrtCoverStatusOID); ok {
				if n, ok := integerValue(vb.Value); ok {
					ensure(ins).Status = n
				}
			}
		}
	}
	sort.Strings(order)
	rows := make([]CoverRow, 0, len(order))
	for _, ins := range order {
		rows = append(rows, *byInstance[ins])
	}
	return rows
}

// JoinConsoleLines extracts prtConsoleDisplayBufferText (1.3.6.1.2.1.43.16.5.1.2) rows.
func JoinConsoleLines(varbinds []VarBind) []ConsoleLine {
	lines := make([]ConsoleLine, 0)
	for _, vb := range varbinds {
		if IsExceptionVarBind(vb) {
			continue
		}
		oid := strings.TrimPrefix(vb.OID, ".")
		if strings.HasPrefix(oid, PrtConsoleDisplayBufferTextOID+".") {
			if ins, ok := tableInstance(oid, PrtConsoleDisplayBufferTextOID); ok {
				if s, ok := StringValue(vb.Value); ok && s != "" {
					lines = append(lines, ConsoleLine{Instance: ins, Text: s})
				}
			}
		}
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].Instance < lines[j].Instance })
	return lines
}

// JoinAlertRows joins columns of prtAlertEntry (1.3.6.1.2.1.43.18.1.1) by exact instance.
func JoinAlertRows(varbinds []VarBind) []AlertRow {
	byInstance := make(map[string]*AlertRow)
	order := make([]string, 0)
	ensure := func(ins string) *AlertRow {
		if row, ok := byInstance[ins]; ok {
			return row
		}
		row := &AlertRow{Instance: ins}
		byInstance[ins] = row
		order = append(order, ins)
		return row
	}
	for _, vb := range varbinds {
		if IsExceptionVarBind(vb) {
			continue
		}
		oid := strings.TrimPrefix(vb.OID, ".")
		switch {
		case strings.HasPrefix(oid, PrtAlertSeverityLevelOID+"."):
			if ins, ok := tableInstance(oid, PrtAlertSeverityLevelOID); ok {
				if n, ok := integerValue(vb.Value); ok {
					ensure(ins).Severity = n
				}
			}
		case strings.HasPrefix(oid, PrtAlertGroupOID+"."):
			if ins, ok := tableInstance(oid, PrtAlertGroupOID); ok {
				if n, ok := integerValue(vb.Value); ok {
					ensure(ins).Group = n
				}
			}
		case strings.HasPrefix(oid, PrtAlertCodeOID+"."):
			if ins, ok := tableInstance(oid, PrtAlertCodeOID); ok {
				if n, ok := integerValue(vb.Value); ok {
					ensure(ins).Code = n
				}
			}
		case strings.HasPrefix(oid, PrtAlertDescriptionOID+"."):
			if ins, ok := tableInstance(oid, PrtAlertDescriptionOID); ok {
				if s, ok := StringValue(vb.Value); ok {
					ensure(ins).Description = s
				}
			}
		case strings.HasPrefix(oid, PrtAlertTimeOID+"."):
			if ins, ok := tableInstance(oid, PrtAlertTimeOID); ok {
				if n, ok := integerValue(vb.Value); ok {
					ensure(ins).Time = n
				}
			}
		}
	}
	sort.Strings(order)
	rows := make([]AlertRow, 0, len(order))
	for _, ins := range order {
		rows = append(rows, *byInstance[ins])
	}
	return rows
}

// AnonymizeSnapshot masks host IPs and serial numbers so exported JSON fixtures can be safely shared.
func AnonymizeSnapshot(snapshot PrinterMIBSnapshot) PrinterMIBSnapshot {
	out := snapshot
	if out.Host != "" {
		out.Host = "192.0.2.10"
	}
	if out.Serial != "" {
		out.Serial = "REDACTED-SERIAL"
	}
	decoded := make([]DecodedVarBind, len(out.DecodedOIDs))
	copy(decoded, out.DecodedOIDs)
	for i := range decoded {
		if decoded[i].Symbol == "prtGeneralSerialNumber" || decoded[i].OID == PrtGeneralSerialNumberOID || strings.HasPrefix(decoded[i].OID, PrtGeneralSerialNumberOID+".") {
			decoded[i].Value = "REDACTED-SERIAL"
		}
	}
	out.DecodedOIDs = decoded
	return out
}
