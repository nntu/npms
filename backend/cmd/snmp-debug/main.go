package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"npms/backend/internal/snmp"
)

const (
	sysDescrOID  = "1.3.6.1.2.1.1.1.0"
	sysObjectOID = "1.3.6.1.2.1.1.2.0"
	sysNameOID   = "1.3.6.1.2.1.1.5.0"
	serialOID    = "1.3.6.1.2.1.43.5.1.1.17"
)

type output struct {
	Command string           `json:"command"`
	Host    string           `json:"host"`
	Results []snmp.VarBind   `json:"results,omitempty"`
	Markers []snmp.MarkerRow `json:"markers,omitempty"`
}

var systemLabels = map[string]string{
	sysDescrOID:  "Description",
	sysObjectOID: "Object ID",
	sysNameOID:   "Name",
	serialOID:    "Serial",
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" || args[0] == "-help" {
		printUsage(stdout)
		if len(args) == 0 {
			return errors.New("command is required: check, probe, get, walk, or export (see usage above)")
		}
		return nil
	}
	command := args[0]
	if command != "check" && command != "probe" && command != "get" && command != "walk" && command != "export" {
		printUsage(stderr)
		return fmt.Errorf("unknown command %q (see usage above)", command)
	}

	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printUsage(stderr)
	}
	host := fs.String("host", "", "printer hostname or IP")
	port := fs.Int("port", 161, "SNMP UDP port")
	version := fs.String("version", "2c", "SNMP version: 2c or 3")
	community := fs.String("community", "", "SNMP v2c community")
	username := fs.String("username", "", "SNMPv3 username")
	authProtocol := fs.String("auth-protocol", "", "SNMPv3 auth protocol: MD5 or SHA")
	authPassphrase := fs.String("auth-passphrase", "", "SNMPv3 auth passphrase")
	privProtocol := fs.String("priv-protocol", "", "SNMPv3 privacy protocol: DES or AES")
	privPassphrase := fs.String("priv-passphrase", "", "SNMPv3 privacy passphrase")
	timeout := fs.Int("timeout", 3, "timeout in seconds")
	retries := fs.Int("retries", 1, "number of retries")
	maxRepetitions := fs.Int("max-repetitions", 25, "SNMP bulk walk repetitions")
	oid := fs.String("oid", "", "OID for get/walk")
	outputPath := fs.String("output", "", "JSON output path for export")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *maxRepetitions < 1 || *maxRepetitions > 255 {
		return errors.New("max-repetitions must be between 1 and 255")
	}
	if *port < 1 || *port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if command == "export" && strings.TrimSpace(*outputPath) == "" {
		return errors.New("--output is required for export")
	}
	cfg := snmp.Config{Host: *host, Port: uint16(*port), Version: snmp.Version(*version), Community: *community, Username: *username, AuthProtocol: *authProtocol, AuthPassphrase: *authPassphrase, PrivProtocol: *privProtocol, PrivPassphrase: *privPassphrase, TimeoutSeconds: *timeout, Retries: *retries, MaxRepetitions: uint8(*maxRepetitions)}
	client, err := snmp.NewGoSNMPClient(cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect to %s: %w", *host, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()
	var result output
	result.Command, result.Host = command, *host
	switch command {
	case "check", "probe", "export":
		result.Results, err = client.Get(ctx, []string{sysDescrOID, sysObjectOID, sysNameOID, serialOID})
		if err == nil {
			life, lifeErr := client.Walk(ctx, snmp.MarkerLifeOID)
			unit, unitErr := client.Walk(ctx, snmp.MarkerUnitOID)
			if lifeErr != nil {
				err = lifeErr
			} else if unitErr != nil {
				err = unitErr
			} else {
				result.Markers = snmp.JoinMarkerRows(life, unit, snmp.MarkerLifeOID, snmp.MarkerUnitOID)
			}
		}
	case "get":
		if strings.TrimSpace(*oid) == "" {
			return errors.New("--oid is required for get")
		}
		result.Results, err = client.Get(ctx, []string{*oid})
	case "walk":
		if strings.TrimSpace(*oid) == "" {
			return errors.New("--oid is required for walk")
		}
		result.Results, err = client.Walk(ctx, *oid)
	}
	if err != nil {
		return err
	}
	if command == "check" {
		_, err = stdout.Write([]byte(formatConsole(result)))
		return err
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if command == "export" {
		if err := os.WriteFile(*outputPath, encoded, 0o600); err != nil {
			return fmt.Errorf("write export: %w", err)
		}
		return nil
	}
	_, err = stdout.Write(encoded)
	return err
}

func formatConsole(result output) string {
	var b strings.Builder
	fmt.Fprintln(&b, "NPMS SNMP check")
	fmt.Fprintf(&b, "Host/IP: %s\n", result.Host)
	fmt.Fprintln(&b, "Status:  OK")
	fmt.Fprintln(&b, "\nPrinter information:")
	for _, oid := range []string{sysNameOID, sysDescrOID, sysObjectOID, serialOID} {
		if value, ok := findValue(result.Results, oid); ok {
			fmt.Fprintf(&b, "  %-12s %v\n", systemLabels[oid]+":", value)
		}
	}
	if len(result.Markers) == 0 {
		fmt.Fprintln(&b, "\nCounter values: no validated marker rows returned")
		return b.String()
	}
	fmt.Fprintln(&b, "\nCounter values:")
	fmt.Fprintln(&b, "  INSTANCE     UNIT    LIFE_COUNT")
	for _, marker := range result.Markers {
		fmt.Fprintf(&b, "  %-12s %-7d %d\n", marker.Instance, marker.Unit, marker.Life)
	}
	return b.String()
}

func findValue(results []snmp.VarBind, oid string) (any, bool) {
	for _, result := range results {
		if strings.TrimPrefix(result.OID, ".") == oid {
			return result.Value, true
		}
	}
	return nil, false
}

func printUsage(w io.Writer) {
	usage := `NPMS SNMP Diagnostic CLI (snmp-debug)

Usage:
  snmp-debug <command> [options]

Available Commands:
  check     Perform console health & marker counter check (human-readable table output)
  probe     Probe printer identity and marker OIDs (JSON output)
  get       Fetch a single OID value (requires --oid)
  walk      Walk an OID subtree (requires --oid)
  export    Export full printer diagnostics to a JSON file (requires --output)
  help      Show this help message

Global Options:
  --host string            Printer hostname or IP address (required)
  --port int               SNMP UDP port (default: 161)
  --version string         SNMP version: 2c or 3 (default: 2c)
  --community string       SNMP v2c community string
  --username string        SNMPv3 username
  --auth-protocol string   SNMPv3 authentication protocol (MD5 or SHA)
  --auth-passphrase string SNMPv3 authentication passphrase
  --priv-protocol string   SNMPv3 privacy protocol (DES or AES)
  --priv-passphrase string SNMPv3 privacy passphrase
  --timeout int            Timeout in seconds (default: 3)
  --retries int            Number of retries (default: 1)
  --max-repetitions int    SNMP bulk walk repetitions (1-255, default: 25)
  --oid string             Target OID for 'get' or 'walk' command
  --output string          Target file path for 'export' command

Examples:
  # SNMP v2c console check:
  snmp-debug check --host 192.168.1.50 --community public

  # SNMP v3 authPriv walk:
  snmp-debug walk --host 192.168.1.50 --version 3 --username admin \
    --auth-protocol SHA --auth-passphrase "AuthPass123" \
    --priv-protocol AES --priv-passphrase "PrivPass123" \
    --oid 1.3.6.1.2.1.43.11.1.1

  # Export diagnostics to JSON file:
  snmp-debug export --host 192.168.1.50 --community public --output sample.json
`
	fmt.Fprintln(w, strings.TrimSpace(usage))
}
