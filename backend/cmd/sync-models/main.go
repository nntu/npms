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
	"npms/backend/internal/discovery"
	"npms/backend/internal/profile"
	"npms/backend/internal/repository"
	"npms/backend/internal/security"
)

type DeviceRecord struct {
	IP             string
	Model          string
	Manufacturer   string
	DisplayName    string
	Serial         string
	MAC            string
	Location       string
	Category       string
	ProfileKey     string
	SysObjectID    string
	SysDescr       string
	InitialReading int64
}

func main() {
	fmt.Println("=== NPMS Sync & Import Device Models ===")

	// 1. Load config for paths & security
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

	// 2. Load discovery rules
	rulesCandidates := []string{"discovery.yaml", "../discovery.yaml", "d:/go_project/npms/discovery.yaml"}
	for _, rc := range rulesCandidates {
		if _, err := os.Stat(rc); err == nil {
			if r, err := discovery.LoadRules(rc); err == nil {
				discovery.SetGlobalRules(r)
				break
			}
		}
	}

	// 3. Load device profiles dynamically
	profilesPath := cfg.Profiles.Path
	if profilesPath == "" {
		profilesPath = "./profiles"
	}
	if _, err := os.Stat(profilesPath); err != nil {
		altProfiles := []string{"../profiles", "d:/go_project/npms/profiles"}
		for _, alt := range altProfiles {
			if _, err := os.Stat(alt); err == nil {
				profilesPath = alt
				break
			}
		}
	}
	profiles, err := profile.LoadDir(profilesPath)
	if err != nil {
		fmt.Printf("load profiles error: %v\n", err)
		return
	}
	resolver := profile.Resolver{Profiles: profiles}
	profileMap := make(map[string]profile.Profile, len(profiles))
	for _, p := range profiles {
		profileMap[p.ID] = p
	}

	// 4. Read DevicesList.csv
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

		mfg, cleanModel := discovery.InferPrinterModelAndVendor("", model, "")
		if cleanModel != "" {
			model = cleanModel
		}
		if mfg == "" {
			mfg = "Brother"
		}

		dispName := model
		if loc != "" {
			dispName = fmt.Sprintf("%s (%s)", model, loc)
		} else if node != "" {
			dispName = fmt.Sprintf("%s - %s", model, node)
		}

		profileKey := "generic-printer-mib"
		if matched, err := resolver.Resolve(profile.DeviceIdentity{
			Manufacturer: mfg,
			Model:        model,
		}, ""); err == nil {
			profileKey = matched.ID
		}

		cat := discovery.ClassifyDeviceType(mfg, model, dispName, "", "", profileKey, 0)

		devicesByIP[ip] = &DeviceRecord{
			IP:           ip,
			Model:        model,
			Manufacturer: mfg,
			DisplayName:  dispName,
			Serial:       serial,
			MAC:          mac,
			Location:     loc,
			Category:     cat,
			ProfileKey:   profileKey,
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
			inferredMfg, inferredModel := discovery.InferPrinterModelAndVendor(sysObjID, model, sysDescr)
			if inferredModel != "" {
				model = inferredModel
			}
			if inferredMfg != "" {
				mfg = inferredMfg
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
			}
			devicesByIP[addr] = d
		} else {
			if d.Manufacturer == "" || d.Model == "" {
				inferredMfg, inferredModel := discovery.InferPrinterModelAndVendor(sysObjID, d.Model, sysDescr)
				if d.Manufacturer == "" && inferredMfg != "" {
					d.Manufacturer = inferredMfg
				}
				if d.Model == "" && inferredModel != "" {
					d.Model = inferredModel
				}
			}
		}

		d.SysObjectID = sysObjID
		d.SysDescr = sysDescr
		if d.Serial == "" && serial != "" {
			d.Serial = serial
		}
		if d.Location == "" && sysLoc != "" {
			d.Location = sysLoc
		}

		// Dynamic profile resolution using profile.Resolver driven by YAML profile definitions
		identity := profile.DeviceIdentity{
			SysObjectID:  d.SysObjectID,
			Manufacturer: d.Manufacturer,
			Model:        d.Model,
		}
		if matched, err := resolver.Resolve(identity, ""); err == nil {
			d.ProfileKey = matched.ID
		} else if d.ProfileKey == "" {
			d.ProfileKey = "generic-printer-mib"
		}

		// Dynamic classification based on configurable rules
		d.Category = discovery.ClassifyDeviceType(d.Manufacturer, d.Model, d.DisplayName, d.SysDescr, d.SysObjectID, d.ProfileKey, 0)

		// Extract initial counter reading dynamically from dump using the matched profile's counter OIDs
		targetOIDs := make(map[string]bool)
		if prof, ok := profileMap[d.ProfileKey]; ok {
			for _, counterDef := range prof.Counters {
				cleanOID := strings.TrimPrefix(counterDef.ValueColumn, ".")
				targetOIDs[cleanOID] = true
			}
		}
		if len(targetOIDs) == 0 {
			targetOIDs["1.3.6.1.2.1.43.10.2.1.4"] = true
		}

		if candidates, ok := raw["candidate_counters"].([]any); ok {
			for _, item := range candidates {
				if cMap, ok := item.(map[string]any); ok {
					oid, _ := cMap["oid"].(string)
					valNum, _ := cMap["value"].(float64)
					cleanCandidateOID := strings.TrimPrefix(oid, ".")
					for target := range targetOIDs {
						if (cleanCandidateOID == target || strings.HasPrefix(cleanCandidateOID, target+".")) && valNum > 0 {
							d.InitialReading = int64(valNum)
							break
						}
					}
					if d.InitialReading > 0 {
						break
					}
				}
			}
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

	// 4. Initialize SecretBox for SNMP credentials
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
		cfg.Database.Path,
		`d:/go_project/npms/data/npms.db`,
		`d:/go_project/npms/dist/data/npms.db`,
	}
	syncedPaths := make(map[string]bool)

	for _, dbPath := range dbPaths {
		if dbPath == "" || syncedPaths[dbPath] {
			continue
		}
		if _, err := os.Stat(filepath.Dir(dbPath)); err != nil {
			continue
		}
		syncedPaths[dbPath] = true
		if err := syncDatabase(dbPath, devicesByIP, profileMap, box); err != nil {
			fmt.Printf("Error syncing %s: %v\n", dbPath, err)
		} else {
			fmt.Printf("Successfully populated/synced %d devices into %s\n", len(devicesByIP), dbPath)
		}
	}
}

func syncDatabase(dbPath string, devices map[string]*DeviceRecord, profileMap map[string]profile.Profile, box *security.SecretBox) error {
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

		// Device Profile Assignment
		var storedProfileID string
		_ = db.QueryRowContext(ctx, `SELECT id FROM snmp_profiles WHERE profile_key = ? ORDER BY version DESC LIMIT 1`, d.ProfileKey).Scan(&storedProfileID)
		if storedProfileID == "" {
			_ = db.QueryRowContext(ctx, `SELECT id FROM snmp_profiles WHERE profile_key = 'generic-printer-mib' ORDER BY version DESC LIMIT 1`).Scan(&storedProfileID)
		}
		if storedProfileID == "" {
			storedProfileID = "generic-printer-mib-v1"
		}

		_, _ = db.ExecContext(ctx, `
			INSERT INTO device_profile_assignments(device_id, profile_id, assigned_at, explicit)
			VALUES (?, ?, ?, 1)
			ON CONFLICT(device_id, profile_id) DO NOTHING
		`, deviceID, storedProfileID, now)

		// Insert or update Counter definitions dynamically from resolved profile
		prof, hasProf := profileMap[d.ProfileKey]
		if !hasProf {
			prof = profileMap["generic-printer-mib"]
		}
		primaryCounterKey := ""
		for cKey, cDef := range prof.Counters {
			if primaryCounterKey == "" || cKey == "marker_life" || cKey == "page_count" {
				primaryCounterKey = cKey
			}
			cDefID := fmt.Sprintf("%s:%s", deviceID, cKey)
			instance := ""
			if cDef.Mode == "walk" {
				instance = "1.1"
			}
			_, _ = db.ExecContext(ctx, `
				INSERT INTO counter_definitions(
					id, device_id, key, source_protocol, oid, instance, unit,
					semantic_type, scope, verified, mode, unit_oid, selection,
					aggregation, require_unit_validation, capability_state
				)
				VALUES (?, ?, ?, 'snmp', ?, NULLIF(?, ''), ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, 'valid')
				ON CONFLICT(id) DO UPDATE SET
					oid = excluded.oid,
					mode = excluded.mode,
					unit = excluded.unit,
					unit_oid = excluded.unit_oid,
					semantic_type = excluded.semantic_type,
					scope = excluded.scope
			`, cDefID, deviceID, cKey, cDef.ValueColumn, instance, cDef.Unit,
				cDef.SemanticType, cDef.Scope, boolInt(prof.VerificationStatus == profile.VerificationVerified),
				cDef.Mode, cDef.UnitColumn, cDef.Selection, cDef.Aggregation,
				boolInt(cDef.RequireUnitValidation))
		}

		// If initial reading was extracted from dump, insert baseline snapshot reading only (delta = 0, no daily_counter_usage spike)
		if d.InitialReading > 0 && primaryCounterKey != "" {
			cDefID := fmt.Sprintf("%s:%s", deviceID, primaryCounterKey)
			runID := uuid.NewString()
			_, _ = db.ExecContext(ctx, `
				INSERT INTO polling_runs(id, device_id, job_kind, started_at, ended_at, result)
				VALUES (?, ?, 'counter', ?, ?, 'success')
				ON CONFLICT(id) DO NOTHING
			`, runID, deviceID, now, now)

			_, _ = db.ExecContext(ctx, `
				INSERT INTO counter_readings(poll_run_id, counter_definition_id, raw_value, collected_at, quality)
				VALUES (?, ?, ?, ?, 'valid')
				ON CONFLICT(poll_run_id, counter_definition_id) DO UPDATE SET raw_value=excluded.raw_value
			`, runID, cDefID, d.InitialReading, now)
		}
	}

	// Recalculate daily counter usage using standard non-zero baseline engine
	repo, repoErr := repository.NewSQLiteRepository(db)
	if repoErr == nil {
		rows, err := db.QueryContext(ctx, `SELECT id FROM devices WHERE deleted_at IS NULL`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var devID string
				if err := rows.Scan(&devID); err == nil {
					_ = repo.RecalculateDailyUsage(ctx, devID, time.UTC)
				}
			}
		}
	}

	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
