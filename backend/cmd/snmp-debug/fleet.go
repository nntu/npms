package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"npms/backend/internal/config"
	"npms/backend/internal/database"
	"npms/backend/internal/ingestion"
	"npms/backend/internal/repository"
	"npms/backend/internal/security"
	"npms/backend/internal/snmp"
)

func runDumpFleet(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dump-fleet", flag.ContinueOnError)
	fs.SetOutput(stderr)

	hosts := fs.String("hosts", "", "Comma-separated printer IP addresses or hostnames")
	dbPath := fs.String("db", "", "Path to SQLite database (npms.db) to load registered printers")
	configPath := fs.String("config", "", "Path to config.yaml (auto-detects db and encryption key)")
	port := fs.Int("port", 161, "SNMP UDP port (used with --hosts)")
	version := fs.String("version", "2c", "SNMP version (2c or 3)")
	community := fs.String("community", "public", "SNMP v2c community string (used with --hosts)")
	concurrency := fs.Int("concurrency", 5, "Number of concurrent printer queries")
	timeout := fs.Int("timeout", 4, "Per-printer timeout in seconds")
	retries := fs.Int("retries", 1, "Number of retries per request")
	outputPath := fs.String("output", "", "Path to export a single consolidated JSON report")
	outputDir := fs.String("output-dir", "", "Directory path to export separate JSON files per printer IP/model + summary.json")
	anonymize := fs.Bool("anonymize", false, "Mask host IPs and serial numbers")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	var targets []snmp.FleetPrinterDump

	// 1. If explicit --hosts provided
	if strings.TrimSpace(*hosts) != "" {
		hostList := strings.Split(*hosts, ",")
		for _, h := range hostList {
			addr := strings.TrimSpace(h)
			if addr == "" {
				continue
			}
			cfg := snmp.Config{
				Host:           addr,
				Port:           uint16(*port),
				Version:        snmp.Version(*version),
				Community:      *community,
				TimeoutSeconds: *timeout,
				Retries:        *retries,
				MaxRepetitions: 25,
			}
			targets = append(targets, snmp.FleetPrinterDump{
				Address: addr,
				Port:    *port,
				Config:  &cfg,
			})
		}
	} else {
		// 2. Otherwise try loading from SQLite database or config
		effectiveDB := strings.TrimSpace(*dbPath)
		effectiveConfig := strings.TrimSpace(*configPath)

		if effectiveDB == "" && effectiveConfig == "" {
			// Auto-check standard locations
			for _, c := range []string{"config.yaml", "../config.yaml", "./backend/config.yaml"} {
				if _, err := os.Stat(c); err == nil {
					effectiveConfig = c
					break
				}
			}
			if effectiveConfig == "" {
				for _, d := range []string{"npms.db", "./data/npms.db", "../npms.db"} {
					if _, err := os.Stat(d); err == nil {
					effectiveDB = d
					break
				}
			}
		}
	}

		var appCfg *config.Config
		if effectiveConfig != "" {
			loaded, err := config.Load(effectiveConfig)
			if err == nil {
				appCfg = &loaded
				if effectiveDB == "" {
					effectiveDB = appCfg.Database.Path
				}
			}
		}

		if effectiveDB == "" {
			return errors.New("no printer targets found: specify --hosts <ip1,ip2> or provide --db <npms.db> / --config <config.yaml>")
		}

		db, err := database.OpenSQLite(effectiveDB)
		if err != nil {
			return fmt.Errorf("open sqlite database at %s: %w", effectiveDB, err)
		}
		defer db.Close()

		repo, err := repository.NewSQLiteRepository(db.DB)
		if err != nil {
			return fmt.Errorf("init repository: %w", err)
		}

		var resolver *ingestion.ConfigResolver
		if appCfg != nil && appCfg.Security.EncryptionKey != "" {
			key, err := security.ParseKey(appCfg.Security.EncryptionKey)
			if err == nil {
				box, err := security.NewSecretBox(key)
				if err == nil {
					resolver = &ingestion.ConfigResolver{Store: repo, Box: box}
				}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		devices, err := repo.ListDevices(ctx, 1000, 0)
		cancel()
		if err != nil {
			return fmt.Errorf("list devices from db: %w", err)
		}

		for _, dev := range devices {
			devCtx, devCancel := context.WithTimeout(context.Background(), 5*time.Second)
			endpoints, err := repo.ListDeviceEndpoints(devCtx, dev.ID)
			devCancel()
			if err != nil || len(endpoints) == 0 {
				continue
			}
			primary := endpoints[0]
			for _, ep := range endpoints {
				if ep.IsPrimary {
					primary = ep
					break
				}
			}

			dump := snmp.FleetPrinterDump{
				ID:               dev.ID,
				DisplayName:      dev.DisplayName,
				Address:          primary.Address,
				Port:             int(primary.Port),
				CurrentProfileID: dev.ProfileKey,
			}

			if resolver != nil {
				resCtx, resCancel := context.WithTimeout(context.Background(), 5*time.Second)
				resolvedCfg, err := resolver.Resolve(resCtx, primary)
				resCancel()
				if err == nil {
					resolvedCfg.TimeoutSeconds = *timeout
					resolvedCfg.Retries = *retries
					dump.Config = &resolvedCfg
				}
			}

			if dump.Config == nil {
				// Default fallback config
				cfg := snmp.Config{
					Host:           primary.Address,
					Port:           uint16(primary.Port),
					Version:        snmp.Version2c,
					Community:      *community,
					TimeoutSeconds: *timeout,
					Retries:        *retries,
					MaxRepetitions: 25,
				}
				dump.Config = &cfg
			}

			targets = append(targets, dump)
		}
	}

	if len(targets) == 0 {
		return errors.New("no printers found to query")
	}

	fmt.Fprintf(stderr, "[dump-fleet] Querying %d printers (concurrency=%d, timeout=%ds)...\n", len(targets), *concurrency, *timeout)

	clientFactory := func(cfg snmp.Config) (snmp.Client, error) {
		client, err := snmp.NewGoSNMPClient(cfg)
		if err != nil {
			return nil, err
		}
		if err := client.Connect(); err != nil {
			_ = client.Close()
			return nil, err
		}
		return client, nil
	}

	reportCtx, reportCancel := context.WithTimeout(context.Background(), time.Duration(len(targets)+1)*time.Duration(*timeout)*time.Second+30*time.Second)
	defer reportCancel()

	report := snmp.GenerateFleetDumpReport(reportCtx, clientFactory, targets, *concurrency)

	if *anonymize {
		report = snmp.AnonymizeFleetReport(report)
	}

	// 1. If --output-dir is specified, export individual per-printer files and a summary.json
	if dir := strings.TrimSpace(*outputDir); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory %s: %w", dir, err)
		}

		type summaryItem struct {
			File           string `json:"file"`
			Address        string `json:"address"`
			Name           string `json:"name,omitempty"`
			Manufacturer   string `json:"manufacturer,omitempty"`
			Model          string `json:"model,omitempty"`
			SysObjectID    string `json:"sys_object_id,omitempty"`
			Status         string `json:"status"`
			Error          string `json:"error,omitempty"`
			CandidateCount int    `json:"candidate_counters_count"`
		}

		type manifestSummary struct {
			ExportedAt   string        `json:"exported_at"`
			PrinterCount int           `json:"printer_count"`
			OnlineCount  int           `json:"online_count"`
			Printers     []summaryItem `json:"printers"`
		}

		manifest := manifestSummary{
			ExportedAt:   report.ExportedAt,
			PrinterCount: report.PrinterCount,
			OnlineCount:  report.OnlineCount,
			Printers:     make([]summaryItem, len(report.Printers)),
		}

		for i, p := range report.Printers {
			nameTag := p.DisplayName
			if nameTag == "" {
				nameTag = p.Model
			}
			baseName := sanitizeFilename(p.Address)
			if nameTag != "" {
				baseName += "_" + sanitizeFilename(nameTag)
			}
			fileName := baseName + ".json"
			filePath := filepath.Join(dir, fileName)

			pJSON, err := json.MarshalIndent(p, "", "  ")
			if err == nil {
				pJSON = append(pJSON, '\n')
				_ = os.WriteFile(filePath, pJSON, 0o600)
			}

			manifest.Printers[i] = summaryItem{
				File:           fileName,
				Address:        p.Address,
				Name:           p.DisplayName,
				Manufacturer:   p.Manufacturer,
				Model:          p.Model,
				SysObjectID:    p.SysObjectID,
				Status:         p.Status,
				Error:          p.Error,
				CandidateCount: len(p.CandidateCounters),
			}
		}

		mJSON, err := json.MarshalIndent(manifest, "", "  ")
		if err == nil {
			mJSON = append(mJSON, '\n')
			_ = os.WriteFile(filepath.Join(dir, "summary.json"), mJSON, 0o600)
		}
		fmt.Fprintf(stderr, "[dump-fleet] Exported %d individual printer files + summary.json to directory: %s\n", len(report.Printers), dir)
		if strings.TrimSpace(*outputPath) == "" {
			return nil
		}
	}

	// 2. If --output is specified, export consolidated JSON file
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	encoded = append(encoded, '\n')

	if strings.TrimSpace(*outputPath) != "" {
		if err := os.WriteFile(strings.TrimSpace(*outputPath), encoded, 0o600); err != nil {
			return fmt.Errorf("write output file %s: %w", *outputPath, err)
		}
		fmt.Fprintf(stderr, "[dump-fleet] Exported report with %d printers (%d online) to %s\n", report.PrinterCount, report.OnlineCount, *outputPath)
		return nil
	}

	_, err = stdout.Write(encoded)
	return err
}

func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' || r == '/' || r == '\\' || r == ':' {
			b.WriteRune('_')
		}
	}
	res := strings.Trim(b.String(), "_")
	if res == "" {
		return "printer"
	}
	return res
}
