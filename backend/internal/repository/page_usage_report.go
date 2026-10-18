package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// DailyPageUsage summarizes print and scan volumes for a single date across devices.
type DailyPageUsage struct {
	Date            string             `json:"date"`
	TotalPrintPages int64              `json:"total_print_pages"`
	TotalScanPages  int64              `json:"total_scan_pages"`
	DeviceUsage     []DeviceDailyUsage `json:"device_usage"`
}

// DeviceDailyUsage details daily page counts for an individual device.
type DeviceDailyUsage struct {
	DeviceID        string `json:"device_id"`
	DisplayName     string `json:"display_name"`
	Department      string `json:"department"`
	Category        string `json:"category"` // "printer" or "scanner"
	NormalizedPages int64  `json:"normalized_pages"`
}

// PageUsageReport provides aggregated and normalized printed/scanned pages statistics
// grouped by department and printer.
type PageUsageReport struct {
	From             string                `json:"from,omitempty"`
	To               string                `json:"to,omitempty"`
	TotalDepartments int                   `json:"total_departments"`
	TotalPrinters    int                   `json:"total_printers"`
	TotalPrintPages  int64                 `json:"total_print_pages"`
	TotalScanPages   int64                 `json:"total_scan_pages"`
	TotalInitialLife int64                 `json:"total_initial_life"`
	TotalCurrentLife int64                 `json:"total_current_life"`
	Departments      []DepartmentPageUsage `json:"departments"`
	Printers         []PrinterPageUsage    `json:"printers"`
	DailyUsage       []DailyPageUsage      `json:"daily_usage"`
}

// DepartmentPageUsage summarizes print and scan volumes for a single department.
type DepartmentPageUsage struct {
	Department       string             `json:"department"`
	PrinterCount     int                `json:"printer_count"`
	TotalPrintPages  int64              `json:"total_print_pages"`
	TotalScanPages   int64              `json:"total_scan_pages"`
	TotalInitialLife int64              `json:"total_initial_life"`
	TotalCurrentLife int64              `json:"total_current_life"`
	Printers         []PrinterPageUsage `json:"printers"`
}

// PrinterPageUsage details page counts and unit conversions for an individual printer or scanner.
type PrinterPageUsage struct {
	DeviceID         string  `json:"device_id"`
	DisplayName      string  `json:"display_name"`
	Model            string  `json:"model"`
	Manufacturer     string  `json:"manufacturer"`
	Address          string  `json:"address"`
	Serial           string  `json:"serial"`
	Department       string  `json:"department"`
	Category         string  `json:"category"` // "printer" or "scanner"
	Status           string  `json:"status"`
	CounterKey       string  `json:"counter_key"`
	RawUnit          string  `json:"raw_unit"`
	InitialReading   *int64  `json:"initial_reading,omitempty"`
	CurrentReading   *int64  `json:"current_reading,omitempty"`
	PeriodUsage      int64   `json:"period_usage"`
	ConversionFactor float64 `json:"conversion_factor"`
	NormalizedPages  int64   `json:"normalized_pages"`
	ConversionNote   string  `json:"conversion_note"`
}

// GetPageUsageReport aggregates printed and scanned page counts by department and by printer,
// applying unit normalization (impressions/sheets -> printed pages).
func (r *SQLiteRepository) GetPageUsageReport(ctx context.Context, from, to, departmentFilter string) (*PageUsageReport, error) {
	// 1. Query active devices
	devQuery := `SELECT d.id, d.display_name, COALESCE(d.manufacturer, ''), COALESCE(d.model, ''), COALESCE(d.serial, ''), COALESCE(d.department, ''), COALESCE(ep.address, ''), d.status 
		FROM devices d
		LEFT JOIN device_endpoints ep ON ep.device_id = d.id AND ep.is_primary = 1
		WHERE d.deleted_at IS NULL AND (? = '' OR d.department = ?)
		ORDER BY d.department, d.display_name`
	devRows, err := r.db.QueryContext(ctx, devQuery, departmentFilter, departmentFilter)
	if err != nil {
		return nil, fmt.Errorf("query devices for page report: %w", err)
	}
	defer devRows.Close()

	type devMeta struct {
		id           string
		displayName  string
		manufacturer string
		model        string
		serial       string
		department   string
		address      string
		status       string
	}
	deviceMap := make(map[string]devMeta)
	devices := make([]devMeta, 0)
	for devRows.Next() {
		var d devMeta
		if err := devRows.Scan(&d.id, &d.displayName, &d.manufacturer, &d.model, &d.serial, &d.department, &d.address, &d.status); err != nil {
			return nil, fmt.Errorf("scan device for page report: %w", err)
		}
		if strings.TrimSpace(d.department) == "" {
			d.department = "Chưa phân phòng"
		}
		devices = append(devices, d)
		deviceMap[d.id] = d
	}
	if err := devRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate devices for page report: %w", err)
	}

	// 2. Query primary counter definitions for each device
	defQuery := `SELECT id, device_id, key, unit, scope, semantic_type 
		FROM counter_definitions
		WHERE key IN ('marker_life', 'page_count') OR semantic_type = 'marker_life'`
	defRows, err := r.db.QueryContext(ctx, defQuery)
	if err != nil {
		return nil, fmt.Errorf("query counter definitions for page report: %w", err)
	}
	defer defRows.Close()

	type defMeta struct {
		id           string
		key          string
		unit         string
		scope        string
		semanticType string
	}
	deviceDefs := make(map[string]defMeta)
	for defRows.Next() {
		var defID, devID, key, unit, scope, semType string
		if err := defRows.Scan(&defID, &devID, &key, &unit, &scope, &semType); err != nil {
			return nil, fmt.Errorf("scan counter definition for page report: %w", err)
		}
		// Prefer marker_life or page_count
		if _, exists := deviceDefs[devID]; !exists || key == "marker_life" || key == "page_count" {
			deviceDefs[devID] = defMeta{id: defID, key: key, unit: unit, scope: scope, semanticType: semType}
		}
	}
	if err := defRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate counter definitions for page report: %w", err)
	}

	// 3. Query delta period usage and daily breakdown from daily_counter_usage
	usageQuery := `SELECT local_date, device_id, counter_definition_id, SUM(delta) AS delta 
		FROM daily_counter_usage 
		WHERE (? = '' OR local_date >= ?) AND (? = '' OR local_date <= ?)
		GROUP BY local_date, device_id, counter_definition_id
		ORDER BY local_date ASC`
	usageRows, err := r.db.QueryContext(ctx, usageQuery, from, from, to, to)
	if err != nil {
		return nil, fmt.Errorf("query daily usage for page report: %w", err)
	}
	defer usageRows.Close()

	devicePeriodDelta := make(map[string]int64)
	dailyMap := make(map[string]*DailyPageUsage)

	for usageRows.Next() {
		var localDate, devID, defID string
		var delta int64
		if err := usageRows.Scan(&localDate, &devID, &defID, &delta); err != nil {
			return nil, fmt.Errorf("scan daily usage for page report: %w", err)
		}
		devicePeriodDelta[devID+":"+defID] += delta

		def, hasDef := deviceDefs[devID]
		d, hasDev := deviceMap[devID]
		if !hasDef || def.id != defID || !hasDev {
			continue
		}

		isScanner := strings.Contains(strings.ToUpper(d.model), "ADS-") ||
			strings.Contains(strings.ToLower(d.model), "scanner") ||
			def.key == "page_count" ||
			def.scope == "scan"

		category := "printer"
		if isScanner {
			category = "scanner"
		}

		normalizedPages := delta

		entry, ok := dailyMap[localDate]
		if !ok {
			entry = &DailyPageUsage{
				Date:        localDate,
				DeviceUsage: make([]DeviceDailyUsage, 0),
			}
			dailyMap[localDate] = entry
		}
		if isScanner {
			entry.TotalScanPages += normalizedPages
		} else {
			entry.TotalPrintPages += normalizedPages
		}
		entry.DeviceUsage = append(entry.DeviceUsage, DeviceDailyUsage{
			DeviceID:        devID,
			DisplayName:     d.displayName,
			Department:      d.department,
			Category:        category,
			NormalizedPages: normalizedPages,
		})
	}
	if err := usageRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily usage for page report: %w", err)
	}

	// 4. Query baseline (first non-zero) and latest readings from counter_readings
	readingQuery := `SELECT cr.counter_definition_id, cr.raw_value
		FROM counter_readings cr
		JOIN (
			SELECT counter_definition_id, MAX(id) as max_id 
			FROM counter_readings 
			GROUP BY counter_definition_id
		) latest ON cr.id = latest.max_id`
	readingRows, err := r.db.QueryContext(ctx, readingQuery)
	if err != nil {
		return nil, fmt.Errorf("query latest counter reading for page report: %w", err)
	}
	defer readingRows.Close()

	latestReadings := make(map[string]int64)
	for readingRows.Next() {
		var defID string
		var rawVal int64
		if err := readingRows.Scan(&defID, &rawVal); err != nil {
			return nil, fmt.Errorf("scan latest counter reading for page report: %w", err)
		}
		latestReadings[defID] = rawVal
	}
	if err := readingRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest counter reading for page report: %w", err)
	}

	baselineQuery := `SELECT cr.counter_definition_id, cr.raw_value
		FROM counter_readings cr
		JOIN (
			SELECT counter_definition_id, MIN(id) as min_id 
			FROM counter_readings 
			WHERE raw_value > 0
			GROUP BY counter_definition_id
		) first_nz ON cr.id = first_nz.min_id`
	baselineRows, err := r.db.QueryContext(ctx, baselineQuery)
	if err != nil {
		return nil, fmt.Errorf("query baseline counter reading for page report: %w", err)
	}
	defer baselineRows.Close()

	baselineReadings := make(map[string]int64)
	for baselineRows.Next() {
		var defID string
		var rawVal int64
		if err := baselineRows.Scan(&defID, &rawVal); err != nil {
			return nil, fmt.Errorf("scan baseline counter reading for page report: %w", err)
		}
		baselineReadings[defID] = rawVal
	}
	if err := baselineRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate baseline counter reading for page report: %w", err)
	}

	// 5. Build report and department grouping
	report := &PageUsageReport{
		From:        from,
		To:          to,
		Printers:    make([]PrinterPageUsage, 0, len(devices)),
		Departments: make([]DepartmentPageUsage, 0),
	}

	deptMap := make(map[string]*DepartmentPageUsage)

	for _, d := range devices {
		def, hasDef := deviceDefs[d.id]
		var initialVal *int64
		var currentVal *int64
		if hasDef {
			if val, ok := baselineReadings[def.id]; ok {
				initialVal = &val
			}
			if val, ok := latestReadings[def.id]; ok {
				currentVal = &val
			}
		}

		periodDelta := int64(0)
		if hasDef {
			periodDelta = devicePeriodDelta[d.id+":"+def.id]
			// When querying all time, if daily aggregation hasn't run yet, difference between latest and non-zero baseline is trusted usage
			if from == "" && to == "" && periodDelta == 0 && currentVal != nil && initialVal != nil && *currentVal > *initialVal {
				periodDelta = *currentVal - *initialVal
			}
		}

		// Determine device category
		isScanner := strings.Contains(strings.ToUpper(d.model), "ADS-") ||
			strings.Contains(strings.ToLower(d.model), "scanner") ||
			def.key == "page_count" ||
			def.scope == "scan"

		category := "printer"
		if isScanner {
			category = "scanner"
		}

		rawUnit := def.unit
		if rawUnit == "" {
			if isScanner {
				rawUnit = "pages"
			} else {
				rawUnit = "impressions"
			}
		}

		// Unit conversion to printed pages
		conversionFactor := 1.0
		var conversionNote string
		var normalizedPages int64

		if isScanner {
			conversionNote = "Trang quét (Scanner): 1 lượt quét = 1 trang quét, tách riêng khỏi trang in"
			normalizedPages = periodDelta
			report.TotalScanPages += normalizedPages
		} else {
			if rawUnit == "sheets" {
				conversionNote = "1 tờ giấy (sheet) quy đổi tương đương 1 trang in (chuẩn định mức văn phòng)"
			} else if rawUnit == "impressions" {
				conversionNote = "1 impression = 1 trang in (chuẩn Printer-MIB)"
			} else {
				conversionNote = "1 đơn vị gốc = 1 trang in"
			}
			normalizedPages = int64(float64(periodDelta) * conversionFactor)
			report.TotalPrintPages += normalizedPages
		}

		if initialVal != nil {
			report.TotalInitialLife += *initialVal
		}
		if currentVal != nil {
			report.TotalCurrentLife += *currentVal
		}

		item := PrinterPageUsage{
			DeviceID:         d.id,
			DisplayName:      d.displayName,
			Model:            d.model,
			Manufacturer:     d.manufacturer,
			Address:          d.address,
			Serial:           d.serial,
			Department:       d.department,
			Category:         category,
			Status:           d.status,
			CounterKey:       def.key,
			RawUnit:          rawUnit,
			InitialReading:   initialVal,
			CurrentReading:   currentVal,
			PeriodUsage:      periodDelta,
			ConversionFactor: conversionFactor,
			NormalizedPages:  normalizedPages,
			ConversionNote:   conversionNote,
		}

		report.Printers = append(report.Printers, item)

		// Group by department
		dept, exists := deptMap[d.department]
		if !exists {
			dept = &DepartmentPageUsage{
				Department: d.department,
				Printers:   make([]PrinterPageUsage, 0),
			}
			deptMap[d.department] = dept
		}
		dept.PrinterCount++
		dept.Printers = append(dept.Printers, item)
		if isScanner {
			dept.TotalScanPages += normalizedPages
		} else {
			dept.TotalPrintPages += normalizedPages
		}
		if initialVal != nil {
			dept.TotalInitialLife += *initialVal
		}
		if currentVal != nil {
			dept.TotalCurrentLife += *currentVal
		}
	}

	report.TotalPrinters = len(report.Printers)

	// Sort departments
	deptKeys := make([]string, 0, len(deptMap))
	for k := range deptMap {
		deptKeys = append(deptKeys, k)
	}
	sort.Strings(deptKeys)

	for _, k := range deptKeys {
		report.Departments = append(report.Departments, *deptMap[k])
	}
	report.TotalDepartments = len(report.Departments)

	dailyUsageList := make([]DailyPageUsage, 0)
	if from != "" && to != "" {
		tFrom, err1 := time.Parse("2006-01-02", from)
		tTo, err2 := time.Parse("2006-01-02", to)
		if err1 == nil && err2 == nil && !tFrom.After(tTo) && tTo.Sub(tFrom).Hours() <= 24*92 {
			for cur := tFrom; !cur.After(tTo); cur = cur.AddDate(0, 0, 1) {
				dStr := cur.Format("2006-01-02")
				if item, exists := dailyMap[dStr]; exists {
					dailyUsageList = append(dailyUsageList, *item)
				} else {
					dailyUsageList = append(dailyUsageList, DailyPageUsage{
						Date:        dStr,
						DeviceUsage: []DeviceDailyUsage{},
					})
				}
			}
		}
	}
	if len(dailyUsageList) == 0 {
		dateKeys := make([]string, 0, len(dailyMap))
		for k := range dailyMap {
			dateKeys = append(dateKeys, k)
		}
		sort.Strings(dateKeys)
		for _, k := range dateKeys {
			dailyUsageList = append(dailyUsageList, *dailyMap[k])
		}
	}
	report.DailyUsage = dailyUsageList

	return report, nil
}
