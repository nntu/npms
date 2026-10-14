package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"npms/backend/internal/config"
	"npms/backend/internal/profile"
	"npms/backend/internal/security"
)

type DeviceRecord struct {
	IP           string
	Model        string
	Manufacturer string
	DisplayName  string
	Serial       string
	MAC          string
	Location     string
	Category     string
	ProfileKey   string
	SysObjectID  string
	SysDescr     string
}

func main() {
	fmt.Println("=== NPMS Sync & Import Device Models ===")

	// 1. Read DevicesList.csv
	csvCandidates := []string{
		`DevicesList.csv`,
		`../DevicesList.csv`,
		`d:/go_project/npms/DevicesList.csv`,
		`Z:/DevicesList.csv`,
	}
	var csvPath string
	for _, c := range csvCandidates {
		if _, err := os.Stat(c); err == nil {
			csvPath = c
			break
		}
	}
	if csvPath == "" {
		fmt.Println("DevicesList.csv not found")
		return
	}

	f, err := os.Open(csvPath)
	if err != nil {
		fmt.Printf("open csv error: %v\n", err)
		return
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		fmt.Printf("read csv error: %v\n", err)
		return
	}

	header := records[0]
	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(h)] = i
	}
	get := func(row []string, name string) string {
		if idx, ok := colIdx[name]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	devicesByIP := make(map[string]*DeviceRecord)

	for i := 1; i < len(records); i++ {
		row := records[i]
		ip := get(row, "IP Address")
		model := get(row, "Model Name")
		serial := get(row, "Serial Number")
		mac := get(row, "MAC Address")
		loc := get(row, "Location")
		node := get(row, "Node Name")

		mfg := "Brother"
		if strings.HasPrefix(strings.ToUpper(model), "RICOH") {
			mfg = "Ricoh"
		}

		cat := "printer"
		if strings.Contains(model, "ADS-") {
			cat = "scanner"
		}

		dispName := model
		if loc != "" {
			dispName = fmt.Sprintf("%s (%s)", model, loc)
		} else if node != "" {
			dispName = fmt.Sprintf("%s - %s", model, node)
		}

		devicesByIP[ip] = &DeviceRecord{
			IP:           ip,
			Model:        model,
			Manufacturer: mfg,
			DisplayName:  dispName,
			Serial:       serial,
			MAC:          mac,
			Location:     loc,
			Category:     cat,
		}
	}

	// 2. Read fleet-dump/ to correlate and add HP, Lexmark & other network info
	dumpDir := `d:/go_project/npms/fleet-dump`
	dumpFiles, _ := os.ReadDir(dumpDir)
	for _, df := range dumpFiles {
		if !strings.HasSuffix(df.Name(), ".json") || df.Name() == "summary.json" {
			continue
		}
		filePath := filepath.Join(dumpDir, df.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		var raw map[string]any
		_ = json.Unmarshal(data, &raw)

		addr, _ := raw["address"].(string)
		sysDescr, _ := raw["sys_descr"].(string)
		sysObjID, _ := raw["sys_object_id"].(string)
		sysLoc, _ := raw["sys_location"].(string)
		serial, _ := raw["serial"].(string)
		model, _ := raw["model"].(string)
		mfg, _ := raw["manufacturer"].(string)

		d, exists := devicesByIP[addr]
		if !exists {
			cat := "printer"
			if strings.Contains(model, "M501") {
				model = "HP LaserJet Pro M501dn"
				mfg = "HP"
			} else if strings.Contains(model, "M402") {
				model = "HP LaserJet M402dn"
				mfg = "HP"
			} else if strings.Contains(model, "HP87DAF4") || strings.Contains(sysDescr, "M404") {
				model = "HP LaserJet Pro M404dn"
				mfg = "HP"
			} else if strings.Contains(sysObjID, "641.") {
				model = "Lexmark MS510dn"
				mfg = "Lexmark"
			}

			dispName := model
			if sysLoc != "" {
				dispName = fmt.Sprintf("%s (%s)", model, sysLoc)
			}

			d = &DeviceRecord{
				IP:           addr,
				Model:        model,
				Manufacturer: mfg,
				DisplayName:  dispName,
				Serial:       serial,
				Location:     sysLoc,
				Category:     cat,
			}
			devicesByIP[addr] = d
		}

		// Handle 10.141.6.40 which is Brother ADS-2400N
		if addr == "10.141.6.40" {
			d.Model = "Brother ADS-2400N"
			d.Manufacturer = "Brother"
			d.Category = "scanner"
			if d.Location == "" {
				d.Location = sysLoc
			}
			d.DisplayName = fmt.Sprintf("Brother ADS-2400N (%s)", d.Location)
		}

		d.SysObjectID = sysObjID
		d.SysDescr = sysDescr
		if d.Serial == "" && serial != "" {
			d.Serial = serial
		}
		if d.Location == "" && sysLoc != "" {
			d.Location = sysLoc
		}

		// Profile selection
		switch {
		case strings.Contains(d.Model, "ADS-"):
			d.ProfileKey = "brother-nc"
		case strings.Contains(d.Model, "HL-L5100DN"):
			d.ProfileKey = "brother-hl-l5100dn"
		case strings.Contains(d.Model, "HL-L6210"):
			d.ProfileKey = "brother-hl-l6210"
		case d.Manufacturer == "Brother":
			d.ProfileKey = "brother-generic"
		case strings.Contains(d.Model, "M501"):
			d.ProfileKey = "hp-laserjet-enterprise-m501dn"
		case strings.Contains(d.Model, "M402"):
			d.ProfileKey = "hp-laserjet-pro-m402dn"
		case strings.Contains(d.Model, "M404"):
			d.ProfileKey = "hp-laserjet-pro-m404dn"
		case d.Manufacturer == "HP":
			d.ProfileKey = "hp-generic"
		case strings.Contains(d.Model, "IM 2500") || strings.Contains(d.Model, "IM 2702"):
			d.ProfileKey = "ricoh-im-2xxx"
		case d.Manufacturer == "Ricoh":
			d.ProfileKey = "ricoh-generic"
		default:
			d.ProfileKey = "generic-printer-mib"
		}

		// Update dump JSON file content
		raw["model"] = d.Model
		raw["manufacturer"] = d.Manufacturer
		raw["display_name"] = d.DisplayName
		raw["current_profile_id"] = d.ProfileKey
		if d.Serial != "" {
			raw["serial"] = d.Serial
		}
		if d.Location != "" {
			raw["sys_location"] = d.Location
		}

		updatedJSON, err := json.MarshalIndent(raw, "", "  ")
		if err == nil {
			updatedJSON = append(updatedJSON, '\n')
			_ = os.WriteFile(filePath, updatedJSON, 0o644)
		}
	}

	// 3. Update fleet-dump/summary.json
	type SummaryItem struct {
		File           string `json:"file"`
		Address        string `json:"address"`
		Name           string `json:"name"`
		Manufacturer   string `json:"manufacturer"`
		Model          string `json:"model"`
		SysObjectID    string `json:"sys_object_id"`
		Status         string `json:"status"`
		CandidateCount int    `json:"candidate_counters_count"`
	}
	type ManifestSummary struct {
		ExportedAt   string        `json:"exported_at"`
		PrinterCount int           `json:"printer_count"`
		OnlineCount  int           `json:"online_count"`
		Printers     []SummaryItem `json:"printers"`
	}

	manifest := ManifestSummary{
		ExportedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}

	for _, df := range dumpFiles {
		if !strings.HasSuffix(df.Name(), ".json") || df.Name() == "summary.json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dumpDir, df.Name()))
		if err != nil {
			continue
		}
		var raw map[string]any
		_ = json.Unmarshal(data, &raw)

		addr, _ := raw["address"].(string)
		name, _ := raw["display_name"].(string)
		model, _ := raw["model"].(string)
		mfg, _ := raw["manufacturer"].(string)
		sysObjID, _ := raw["sys_object_id"].(string)
		status, _ := raw["status"].(string)
		candidates, _ := raw["candidate_counters"].([]any)

		manifest.Printers = append(manifest.Printers, SummaryItem{
			File:           df.Name(),
			Address:        addr,
			Name:           name,
			Manufacturer:   mfg,
			Model:          model,
			SysObjectID:    sysObjID,
			Status:         status,
			CandidateCount: len(candidates),
		})
	}
	manifest.PrinterCount = len(manifest.Printers)
	manifest.OnlineCount = len(manifest.Printers)

	mJSON, _ := json.MarshalIndent(manifest, "", "  ")
	mJSON = append(mJSON, '\n')
	_ = os.WriteFile(filepath.Join(dumpDir, "summary.json"), mJSON, 0o644)
	fmt.Printf("[fleet-dump] Updated all %d dump JSON files and summary.json!\n", len(manifest.Printers))

	// 4. Load config for DB path & Encryption Key
	configCandidates := []string{"config.yaml", "../config.yaml", "d:/go_project/npms/config.yaml"}
	var configPath string
	for _, c := range configCandidates {
		if _, err := os.Stat(c); err == nil {
			configPath = c
			break
		}
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("load config error: %v\n", err)
		return
	}
	keyBytes, err := security.ParseKey(cfg.Security.EncryptionKey)
	if err != nil {
		fmt.Printf("parse key error: %v\n", err)
		return
	}
	box, err := security.NewSecretBox(keyBytes)
	if err != nil {
		fmt.Printf("secretbox error: %v\n", err)
		return
	}

	// 5. Populate / Synchronize Databases
	dbPaths := []string{
		`d:/go_project/npms/data/npms.db`,
		`d:/go_project/npms/dist/data/npms.db`,
	}

	for _, dbPath := range dbPaths {
		if err := syncDatabase(dbPath, devicesByIP, box); err != nil {
			fmt.Printf("Error syncing %s: %v\n", dbPath, err)
		} else {
			fmt.Printf("Successfully populated/synced %d devices into %s\n", len(devicesByIP), dbPath)
		}
	}
}

func syncDatabase(dbPath string, devices map[string]*DeviceRecord, box *security.SecretBox) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}
	defer db.Close()

	ctx := context.Background()

	// Ensure profiles are loaded and inserted into snmp_profiles table
	profileFiles, _ := filepath.Glob(`d:/go_project/npms/profiles/*/*.yaml`)
	for _, pf := range profileFiles {
		contentBytes, err := os.ReadFile(pf)
		if err != nil {
			continue
		}
		p, err := profile.Load(bytes.NewReader(contentBytes))
		if err != nil {
			continue
		}
		contentStr := string(contentBytes)
		sum := sha256.Sum256(contentBytes)
		checksum := hex.EncodeToString(sum[:])
		pID := p.ID + "-v" + fmt.Sprint(p.Version)

		_, _ = db.ExecContext(ctx, `
			INSERT INTO snmp_profiles(id, profile_key, version, schema_version, content, checksum, verification_status)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET content=excluded.content, checksum=excluded.checksum
		`, pID, p.ID, p.Version, p.SchemaVersion, contentStr, checksum, p.VerificationStatus)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	for _, d := range devices {
		// Check if device already exists by IP
		var existingDeviceID string
		_ = db.QueryRowContext(ctx, `
			SELECT device_id FROM device_endpoints WHERE address = ? LIMIT 1
		`, d.IP).Scan(&existingDeviceID)

		deviceID := existingDeviceID
		if deviceID == "" {
			deviceID = uuid.NewString()
		}

		// Encrypt SNMP credential for v2c "public"
		secretBytes, err := security.EncodeSNMPSecret(security.SNMPSecret{
			Version:   "2c",
			Community: "public",
		})
		if err != nil {
			continue
		}
		encrypted, err := box.Encrypt(secretBytes)
		if err != nil {
			continue
		}

		credID := uuid.NewString()
		_, _ = db.ExecContext(ctx, `
			INSERT INTO snmp_credentials(id, version, encrypted_secret_material, security_metadata, created_at)
			VALUES (?, '2c', ?, '{}', ?)
		`, credID, encrypted, now)

		// Insert or Update Device
		_, err = db.ExecContext(ctx, `
			INSERT INTO devices(id, display_name, manufacturer, model, serial, sys_object_id, department, status, first_seen_at, last_seen_at, created_at, updated_at)
			VALUES (?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), 'online', ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				display_name = excluded.display_name,
				manufacturer = excluded.manufacturer,
				model = excluded.model,
				serial = COALESCE(excluded.serial, devices.serial),
				department = COALESCE(excluded.department, devices.department),
				sys_object_id = COALESCE(excluded.sys_object_id, devices.sys_object_id),
				status = 'online',
				last_seen_at = excluded.last_seen_at,
				updated_at = excluded.updated_at
		`, deviceID, d.DisplayName, d.Manufacturer, d.Model, d.Serial, d.SysObjectID, d.Location, now, now, now, now)
		if err != nil {
			return fmt.Errorf("upsert device %s: %w", d.IP, err)
		}

		// Insert or Update Endpoint
		epID := uuid.NewString()
		_, _ = db.ExecContext(ctx, `
			INSERT INTO device_endpoints(id, device_id, address, protocol, port, credential_id, is_primary)
			VALUES (?, ?, ?, 'snmp', 161, ?, 1)
			ON CONFLICT(id) DO NOTHING
		`, epID, deviceID, d.IP, credID)

		// Device Identifier (Serial & MAC)
		if d.Serial != "" {
			_, _ = db.ExecContext(ctx, `
				INSERT INTO device_identifiers(device_id, kind, value, source, confidence)
				VALUES (?, 'serial', ?, 'devices_list_csv', 'high')
				ON CONFLICT(device_id, kind, value) DO NOTHING
			`, deviceID, d.Serial)
		}
		if d.MAC != "" {
			_, _ = db.ExecContext(ctx, `
				INSERT INTO device_identifiers(device_id, kind, value, source, confidence)
				VALUES (?, 'mac', ?, 'devices_list_csv', 'high')
				ON CONFLICT(device_id, kind, value) DO NOTHING
			`, deviceID, d.MAC)
		}

		// Device Profile Assignment & Counter Definitions
		profileID := d.ProfileKey + "-v1"
		var storedProfileID string
		_ = db.QueryRowContext(ctx, `SELECT id FROM snmp_profiles WHERE id = ?`, profileID).Scan(&storedProfileID)
		if storedProfileID == "" {
			profileID = "generic-printer-mib-v1"
		}

		_, _ = db.ExecContext(ctx, `
			INSERT INTO device_profile_assignments(device_id, profile_id, assigned_at, explicit)
			VALUES (?, ?, ?, 1)
			ON CONFLICT(device_id, profile_id) DO NOTHING
		`, deviceID, profileID, now)

		// Counter definition
		cDefID := deviceID + ":marker_life"
		if d.Category == "scanner" {
			_, _ = db.ExecContext(ctx, `
				INSERT INTO counter_definitions(id, device_id, key, source_protocol, oid, instance, unit, semantic_type, scope, verified, mode, unit_oid, require_unit_validation)
				VALUES (?, ?, 'page_count', 'snmp', '1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.3', '', 'pages', 'marker_life', 'engine', 1, 'get', '', 0)
				ON CONFLICT(id) DO UPDATE SET oid=excluded.oid, mode=excluded.mode
			`, cDefID, deviceID)
		} else {
			_, _ = db.ExecContext(ctx, `
				INSERT INTO counter_definitions(id, device_id, key, source_protocol, oid, instance, unit, semantic_type, scope, verified, mode, unit_oid, selection, aggregation, require_unit_validation)
				VALUES (?, ?, 'marker_life', 'snmp', '1.3.6.1.2.1.43.10.2.1.4', '1.1', 'impressions', 'marker_life', 'engine', 1, 'walk', '1.3.6.1.2.1.43.10.2.1.3', 'validated_marker_rows', 'none', 0)
				ON CONFLICT(id) DO NOTHING
			`, cDefID, deviceID)
		}
	}

	return nil
}
