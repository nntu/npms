package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"npms/backend/internal/profile"
	"npms/backend/internal/repository"
	"npms/backend/internal/security"
)

type DatabaseBackupManager interface {
	Backup(context.Context, string) error
	Restore(context.Context, string) error
}

type DeviceBackupStore interface {
	ListDevicesForBackup(context.Context) ([]repository.DeviceBackupRecord, error)
	FindDeviceByAddressOrAsset(context.Context, string, string) (*repository.Device, error)
}

type PrinterBackupItem struct {
	DisplayName    string `json:"display_name"`
	AssetCode      string `json:"asset_code,omitempty"`
	Department     string `json:"department,omitempty"`
	Manufacturer   string `json:"manufacturer,omitempty"`
	Model          string `json:"model,omitempty"`
	Serial         string `json:"serial,omitempty"`
	SysObjectID    string `json:"sys_object_id,omitempty"`
	ProfileID      string `json:"profile_id,omitempty"`
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Version        string `json:"version"` // "2c" or "3"
	Community      string `json:"community,omitempty"`
	Username       string `json:"username,omitempty"`
	AuthProtocol   string `json:"auth_protocol,omitempty"`
	AuthPassphrase string `json:"auth_passphrase,omitempty"`
	PrivProtocol   string `json:"priv_protocol,omitempty"`
	PrivPassphrase string `json:"priv_passphrase,omitempty"`
}

type PrintersBackupExport struct {
	Version    string              `json:"version"`
	ExportedAt string              `json:"exported_at"`
	Count      int                 `json:"count"`
	Printers   []PrinterBackupItem `json:"printers"`
}

type PrintersRestoreRequest struct {
	Overwrite bool                `json:"overwrite"`
	Printers  []PrinterBackupItem `json:"printers"`
}

type PrinterRestoreItemResult struct {
	Address     string `json:"address"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"` // "created", "updated", "skipped", "failed"
	DeviceID    string `json:"device_id,omitempty"`
	Error       string `json:"error,omitempty"`
}

type PrintersRestoreResponse struct {
	Total   int                        `json:"total"`
	Created int                        `json:"created"`
	Updated int                        `json:"updated"`
	Skipped int                        `json:"skipped"`
	Failed  int                        `json:"failed"`
	Results []PrinterRestoreItemResult `json:"results"`
}

func (s *Server) handlePrintersBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}

	backupStore, ok := s.store.(DeviceBackupStore)
	if !ok {
		writeError(w, http.StatusNotImplemented, "not_supported", "backup store is not supported")
		return
	}

	records, err := backupStore.ListDevicesForBackup(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed", "failed to list devices for backup: "+err.Error())
		return
	}

	items := make([]PrinterBackupItem, 0, len(records))
	for _, rec := range records {
		item := PrinterBackupItem{
			DisplayName:  rec.Device.DisplayName,
			AssetCode:    rec.Device.AssetCode,
			Department:   rec.Device.Department,
			Manufacturer: rec.Device.Manufacturer,
			Model:        rec.Device.Model,
			Serial:       rec.Device.Serial,
			SysObjectID:  rec.Device.SysObjectID,
			ProfileID:    rec.ProfileKey,
			Address:      rec.Address,
			Port:         int(rec.Port),
			Version:      rec.SNMPVersion,
		}
		if item.Port == 0 {
			item.Port = 161
		}
		if item.Version == "" {
			item.Version = "2c"
		}

		if len(rec.EncryptedSecretMaterial) > 0 && s.secretBox != nil {
			if plain, err := s.secretBox.Decrypt(rec.EncryptedSecretMaterial); err == nil {
				if sec, err := security.DecodeSNMPSecret(plain); err == nil {
					item.Version = sec.Version
					item.Community = sec.Community
					item.Username = sec.Username
					item.AuthProtocol = sec.AuthProtocol
					item.AuthPassphrase = sec.AuthPassphrase
					item.PrivProtocol = sec.PrivProtocol
					item.PrivPassphrase = sec.PrivPassphrase
				}
			}
		}
		items = append(items, item)
	}

	export := PrintersBackupExport{
		Version:    "1.0",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Count:      len(items),
		Printers:   items,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode_failed", "failed to encode printers backup: "+err.Error())
		return
	}

	filename := fmt.Sprintf("npms-printers-backup-%s.json", time.Now().Format("2006-01-02_150405"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handlePrintersRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}

	backupStore, ok := s.store.(DeviceBackupStore)
	if !ok {
		writeError(w, http.StatusNotImplemented, "not_supported", "backup store is not supported")
		return
	}

	var rawBytes []byte
	var err error

	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		file, _, formErr := r.FormFile("file")
		if formErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_upload", "failed to read file form field: "+formErr.Error())
			return
		}
		defer file.Close()
		rawBytes, err = io.ReadAll(http.MaxBytesReader(w, file, 10<<20))
	} else {
		rawBytes, err = io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_failed", "could not read request body: "+err.Error())
		return
	}

	var parsedReq PrintersRestoreRequest
	if decErr := json.Unmarshal(rawBytes, &parsedReq); decErr == nil && len(parsedReq.Printers) > 0 {
		// Valid format with printers wrapper
	} else {
		var items []PrinterBackupItem
		if arrErr := json.Unmarshal(rawBytes, &items); arrErr == nil && len(items) > 0 {
			parsedReq.Printers = items
			parsedReq.Overwrite = true
		} else {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be a valid JSON array or object with 'printers'")
			return
		}
	}

	resp := PrintersRestoreResponse{
		Total:   len(parsedReq.Printers),
		Results: make([]PrinterRestoreItemResult, 0, len(parsedReq.Printers)),
	}

	ctx := r.Context()
	for _, item := range parsedReq.Printers {
		addr := strings.TrimSpace(item.Address)
		if addr == "" {
			resp.Failed++
			resp.Results = append(resp.Results, PrinterRestoreItemResult{
				Address:     item.Address,
				DisplayName: item.DisplayName,
				Status:      "failed",
				Error:       "address is required",
			})
			continue
		}

		existing, findErr := backupStore.FindDeviceByAddressOrAsset(ctx, addr, item.AssetCode)
		if findErr != nil {
			resp.Failed++
			resp.Results = append(resp.Results, PrinterRestoreItemResult{
				Address:     addr,
				DisplayName: item.DisplayName,
				Status:      "failed",
				Error:       "lookup error: " + findErr.Error(),
			})
			continue
		}

		if existing != nil {
			if !parsedReq.Overwrite {
				resp.Skipped++
				resp.Results = append(resp.Results, PrinterRestoreItemResult{
					Address:     addr,
					DisplayName: existing.DisplayName,
					Status:      "skipped",
					DeviceID:    existing.ID,
				})
				continue
			}

			mgmtStore, hasMgmt := s.store.(PrinterManagementStore)
			if !hasMgmt {
				resp.Failed++
				resp.Results = append(resp.Results, PrinterRestoreItemResult{
					Address:     addr,
					DisplayName: item.DisplayName,
					Status:      "failed",
					Error:       "printer update is not supported by store",
				})
				continue
			}

			dispName := strings.TrimSpace(item.DisplayName)
			if dispName == "" {
				dispName = existing.DisplayName
			}
			port := item.Port
			if port == 0 {
				port = int(existing.Port)
			}
			if port == 0 {
				port = 161
			}

			var updatedCred *repository.SNMPCredential
			hasCred := strings.TrimSpace(item.Community) != "" || strings.TrimSpace(item.Username) != ""
			if hasCred && s.secretBox != nil {
				ver := item.Version
				if ver == "" {
					ver = "2c"
				}
				comm := item.Community
				if ver == "2c" && comm == "" {
					comm = "public"
				}
				sec := security.SNMPSecret{
					Version:        ver,
					Community:      comm,
					Username:       strings.TrimSpace(item.Username),
					AuthProtocol:   strings.TrimSpace(item.AuthProtocol),
					AuthPassphrase: item.AuthPassphrase,
					PrivProtocol:   strings.TrimSpace(item.PrivProtocol),
					PrivPassphrase: item.PrivPassphrase,
				}
				if encoded, encErr := security.EncodeSNMPSecret(sec); encErr == nil {
					if cipher, encErr := s.secretBox.Encrypt(encoded); encErr == nil {
						updatedCred = &repository.SNMPCredential{
							ID:                      newJobID(),
							Version:                 sec.Version,
							EncryptedSecretMaterial: cipher,
							SecurityMetadata:        fmt.Sprintf(`{"version":%q}`, sec.Version),
						}
					}
				}
			}

			updatePayload := repository.PrinterUpdate{
				Device: repository.Device{
					ID:           existing.ID,
					AssetCode:    strings.TrimSpace(item.AssetCode),
					DisplayName:  dispName,
					Manufacturer: strings.TrimSpace(item.Manufacturer),
					Model:        strings.TrimSpace(item.Model),
					Serial:       strings.TrimSpace(item.Serial),
					SysObjectID:  strings.TrimSpace(item.SysObjectID),
					Department:   strings.TrimSpace(item.Department),
					Status:       existing.Status,
				},
				EndpointAddress: addr,
				EndpointPort:    uint16(port),
				NewEndpointID:   newJobID(),
				Credential:      updatedCred,
			}

			if len(s.profiles) > 0 {
				selected, resolveErr := (profile.Resolver{Profiles: s.profiles}).Resolve(profile.DeviceIdentity{
					SysObjectID:  updatePayload.Device.SysObjectID,
					Manufacturer: updatePayload.Device.Manufacturer,
					Model:        updatePayload.Device.Model,
				}, strings.TrimSpace(item.ProfileID))
				if resolveErr == nil {
					content, _ := yaml.Marshal(selected)
					digest := sha256.Sum256(content)
					profileID := selected.ID + "@" + strconv.Itoa(selected.Version)
					updatePayload.Profile = &repository.SNMPProfile{
						ID:                 profileID,
						ProfileKey:         selected.ID,
						Version:            selected.Version,
						SchemaVersion:      selected.SchemaVersion,
						Content:            string(content),
						Checksum:           hex.EncodeToString(digest[:]),
						VerificationStatus: string(selected.VerificationStatus),
					}
					updatePayload.ProfileExplicit = strings.TrimSpace(item.ProfileID) != ""
					updatePayload.CounterDefinitions = make([]repository.CounterDefinition, 0, len(selected.Counters))
					for key, definition := range selected.Counters {
						instance := ""
						if definition.Mode == "walk" {
							instance = "1"
						}
						updatePayload.CounterDefinitions = append(updatePayload.CounterDefinitions, repository.CounterDefinition{
							ID:                    existing.ID + ":" + key,
							DeviceID:              existing.ID,
							Key:                   key,
							SourceProtocol:        "snmp",
							OID:                   definition.ValueColumn,
							Instance:              instance,
							Unit:                  definition.Unit,
							SemanticType:          definition.SemanticType,
							Scope:                 definition.Scope,
							Mode:                  definition.Mode,
							UnitOID:               definition.UnitColumn,
							Selection:             definition.Selection,
							Aggregation:           definition.Aggregation,
							RequireUnitValidation: definition.RequireUnitValidation,
							Verified:              selected.VerificationStatus == profile.VerificationVerified,
						})
					}
				}
			}

			if err := mgmtStore.UpdatePrinter(ctx, updatePayload); err != nil {
				resp.Failed++
				resp.Results = append(resp.Results, PrinterRestoreItemResult{
					Address:     addr,
					DisplayName: dispName,
					Status:      "failed",
					DeviceID:    existing.ID,
					Error:       err.Error(),
				})
			} else {
				resp.Updated++
				resp.Results = append(resp.Results, PrinterRestoreItemResult{
					Address:     addr,
					DisplayName: dispName,
					Status:      "updated",
					DeviceID:    existing.ID,
				})
			}
			continue
		}

		// Register new printer
		regInput := registerPrinterRequest{
			DisplayName:    item.DisplayName,
			AssetCode:      item.AssetCode,
			Manufacturer:   item.Manufacturer,
			Model:          item.Model,
			Serial:         item.Serial,
			SysObjectID:    item.SysObjectID,
			Department:     item.Department,
			ProfileID:      item.ProfileID,
			Address:        addr,
			Port:           item.Port,
			Version:        item.Version,
			Community:      item.Community,
			Username:       item.Username,
			AuthProtocol:   item.AuthProtocol,
			AuthPassphrase: item.AuthPassphrase,
			PrivProtocol:   item.PrivProtocol,
			PrivPassphrase: item.PrivPassphrase,
		}

		res, regErr := s.registerPrinterInternal(ctx, regInput)
		if regErr != nil {
			resp.Failed++
			resp.Results = append(resp.Results, PrinterRestoreItemResult{
				Address:     addr,
				DisplayName: item.DisplayName,
				Status:      "failed",
				Error:       regErr.Error(),
			})
		} else {
			resp.Created++
			resp.Results = append(resp.Results, PrinterRestoreItemResult{
				Address:     addr,
				DisplayName: res.Device.DisplayName,
				Status:      "created",
				DeviceID:    res.Device.ID,
			})
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDatabaseBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	if s.backupManager == nil {
		writeError(w, http.StatusNotImplemented, "not_configured", "database backup manager is not configured")
		return
	}

	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("npms_backup_%d.db", time.Now().UnixNano()))
	defer func() {
		_ = os.Remove(tempFile)
	}()

	if err := s.backupManager.Backup(r.Context(), tempFile); err != nil {
		writeError(w, http.StatusInternalServerError, "backup_error", "failed to backup database: "+err.Error())
		return
	}

	f, err := os.Open(tempFile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read_error", "failed to open backup file: "+err.Error())
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err == nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	}
	filename := fmt.Sprintf("npms-database-backup-%s.db", time.Now().Format("2006-01-02_150405"))
	w.Header().Set("Content-Type", "application/x-sqlite3")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

func (s *Server) handleDatabaseRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	if s.backupManager == nil {
		writeError(w, http.StatusNotImplemented, "not_configured", "database backup manager is not configured")
		return
	}

	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("npms_restore_%d.db", time.Now().UnixNano()))
	defer func() {
		_ = os.Remove(tempFile)
	}()

	var src io.Reader
	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_upload", "failed to get uploaded file from 'file' field: "+err.Error())
			return
		}
		defer file.Close()
		src = file
	} else {
		src = r.Body
	}

	dst, err := os.Create(tempFile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_temp_failed", "failed to create temp restore file: "+err.Error())
		return
	}

	written, err := io.Copy(dst, src)
	_ = dst.Close()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "save_file_failed", "failed to save uploaded backup file: "+err.Error())
		return
	}
	if written < 100 {
		writeError(w, http.StatusBadRequest, "invalid_file", "uploaded file is too small to be a valid SQLite database")
		return
	}

	header := make([]byte, 16)
	checkFile, err := os.Open(tempFile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "open_file_failed", "failed to inspect uploaded backup file: "+err.Error())
		return
	}
	_, _ = io.ReadFull(checkFile, header)
	_ = checkFile.Close()

	if string(header) != "SQLite format 3\x00" {
		writeError(w, http.StatusBadRequest, "invalid_sqlite", "uploaded file is not a valid SQLite database file")
		return
	}

	if err := s.backupManager.Restore(r.Context(), tempFile); err != nil {
		writeError(w, http.StatusInternalServerError, "restore_failed", "failed to restore database: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "Toàn bộ cơ sở dữ liệu đã được khôi phục thành công",
	})
}
