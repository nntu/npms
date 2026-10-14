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
	sysDescrOID       = snmp.SysDescrOID
	sysObjectOID      = snmp.SysObjectOID
	sysNameOID        = snmp.SysNameOID
	serialOID         = snmp.PrtGeneralSerialNumberOID
	serialInstanceOID = snmp.PrtGeneralSerialNumberInstance1OID
	printerNameInsOID = snmp.PrtGeneralPrinterNameInstance1OID
)

type output struct {
	Command      string                `json:"command"`
	Host         string                `json:"host"`
	Results      []snmp.VarBind        `json:"results,omitempty"`
	Markers      []snmp.MarkerRow      `json:"markers,omitempty"`
	Supplies     []snmp.SupplyRow      `json:"supplies,omitempty"`
	Inputs       []snmp.InputRow       `json:"inputs,omitempty"`
	Covers       []snmp.CoverRow       `json:"covers,omitempty"`
	ConsoleLines []snmp.ConsoleLine    `json:"console_lines,omitempty"`
	Alerts       []snmp.AlertRow       `json:"alerts,omitempty"`
	DecodedOIDs  []snmp.DecodedVarBind `json:"decoded_oids,omitempty"`
}

var systemLabels = map[string]string{
	sysDescrOID:       "Description",
	sysObjectOID:      "Object ID",
	sysNameOID:        "Name",
	printerNameInsOID: "Model Name",
	serialInstanceOID: "Serial",
	serialOID:         "Serial",
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
			return errors.New("command is required: check, probe, collect, get, walk, or export (see usage above)")
		}
		return nil
	}
	command := args[0]
	switch command {
	case "check", "probe", "collect", "get", "walk", "export":
	default:
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
	outputPath := fs.String("output", "", "JSON output path for export/collect")
	mibFile := fs.String("mib-file", "", "Optional path to Printer-MIB.yaml for OID symbol resolution")
	fixturePath := fs.String("fixture", "", "Optional path to a Printer-MIB JSON fixture for offline replay")
	anonymize := fs.Bool("anonymize", false, "Mask host IP and serial numbers in exported/collected output")
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

	mib, err := resolvePrinterMIB(strings.TrimSpace(*mibFile))
	if err != nil {
		return err
	}

	var client snmp.Client
	effectiveHost := strings.TrimSpace(*host)
	if strings.TrimSpace(*fixturePath) != "" {
		fixtureClient, err := snmp.LoadFixtureClient(strings.TrimSpace(*fixturePath))
		if err != nil {
			return err
		}
		client = fixtureClient
		if effectiveHost == "" {
			effectiveHost = fixtureClient.Snapshot.Host
			if effectiveHost == "" {
				effectiveHost = "fixture"
			}
		}
	} else {
		cfg := snmp.Config{
			Host:           effectiveHost,
			Port:           uint16(*port),
			Version:        snmp.Version(*version),
			Community:      *community,
			Username:       *username,
			AuthProtocol:   *authProtocol,
			AuthPassphrase: *authPassphrase,
			PrivProtocol:   *privProtocol,
			PrivPassphrase: *privPassphrase,
			TimeoutSeconds: *timeout,
			Retries:        *retries,
			MaxRepetitions: uint8(*maxRepetitions),
		}
		liveClient, err := snmp.NewGoSNMPClient(cfg)
		if err != nil {
			return err
		}
		if err := liveClient.Connect(); err != nil {
			_ = liveClient.Close()
			return fmt.Errorf("connect to %s: %w", effectiveHost, err)
		}
		client = liveClient
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	if command == "collect" {
		snapshot, err := snmp.CollectPrinterMIBSnapshot(ctx, client, mib)
		if err != nil {
			return err
		}
		snapshot.Host = effectiveHost
		if *anonymize {
			snapshot = snmp.AnonymizeSnapshot(snapshot)
		}
		encoded, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			return err
		}
		encoded = append(encoded, '\n')
		if strings.TrimSpace(*outputPath) != "" {
			if err := os.WriteFile(strings.TrimSpace(*outputPath), encoded, 0o600); err != nil {
				return fmt.Errorf("write collected snapshot: %w", err)
			}
			return nil
		}
		_, err = stdout.Write(encoded)
		return err
	}

	var result output
	result.Command, result.Host = command, effectiveHost
	switch command {
	case "check", "probe", "export":
		snapshot, snapErr := snmp.CollectPrinterMIBSnapshot(ctx, client, mib)
		if snapErr != nil {
			err = snapErr
			break
		}
		snapshot.Host = effectiveHost
		if *anonymize {
			snapshot = snmp.AnonymizeSnapshot(snapshot)
			result.Host = snapshot.Host
		}
		result.Results = buildSystemVarBinds(snapshot)
		result.Markers = snapshot.Markers
		result.Supplies = snapshot.Supplies
		result.Inputs = snapshot.Inputs
		result.Covers = snapshot.Covers
		result.ConsoleLines = snapshot.ConsoleLines
		result.Alerts = snapshot.Alerts
		result.DecodedOIDs = snapshot.DecodedOIDs
	case "get":
		if strings.TrimSpace(*oid) == "" {
			return errors.New("--oid is required for get")
		}
		result.Results, err = client.Get(ctx, []string{*oid})
		result.DecodedOIDs = decodeVarBinds(result.Results, mib)
	case "walk":
		if strings.TrimSpace(*oid) == "" {
			return errors.New("--oid is required for walk")
		}
		result.Results, err = client.Walk(ctx, *oid)
		result.DecodedOIDs = decodeVarBinds(result.Results, mib)
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

func resolvePrinterMIB(explicitPath string) (*snmp.PrinterMIB, error) {
	if explicitPath != "" {
		return snmp.LoadPrinterMIBFile(explicitPath)
	}
	for _, candidate := range []string{"Printer-MIB.yaml", "../Printer-MIB.yaml"} {
		if _, err := os.Stat(candidate); err == nil {
			if loaded, loadErr := snmp.LoadPrinterMIBFile(candidate); loadErr == nil {
				return loaded, nil
			}
		}
	}
	return snmp.DefaultPrinterMIB(), nil
}

func buildSystemVarBinds(snapshot snmp.PrinterMIBSnapshot) []snmp.VarBind {
	var binds []snmp.VarBind
	if snapshot.Name != "" {
		binds = append(binds, snmp.VarBind{OID: sysNameOID, Type: "OctetString", Value: snapshot.Name})
	}
	if snapshot.PrinterName != "" && snapshot.PrinterName != snapshot.Name {
		binds = append(binds, snmp.VarBind{OID: printerNameInsOID, Type: "OctetString", Value: snapshot.PrinterName})
	}
	if snapshot.Description != "" {
		binds = append(binds, snmp.VarBind{OID: sysDescrOID, Type: "OctetString", Value: snapshot.Description})
	}
	if snapshot.SysObjectID != "" {
		binds = append(binds, snmp.VarBind{OID: sysObjectOID, Type: "ObjectIdentifier", Value: snapshot.SysObjectID})
	}
	if snapshot.Serial != "" {
		binds = append(binds, snmp.VarBind{OID: serialInstanceOID, Type: "OctetString", Value: snapshot.Serial})
	}
	return binds
}

func decodeVarBinds(binds []snmp.VarBind, mib *snmp.PrinterMIB) []snmp.DecodedVarBind {
	if len(binds) == 0 {
		return nil
	}
	decoded := make([]snmp.DecodedVarBind, 0, len(binds))
	for _, vb := range binds {
		if snmp.IsExceptionVarBind(vb) {
			continue
		}
		clean := strings.TrimPrefix(vb.OID, ".")
		d := snmp.DecodedVarBind{
			OID:   clean,
			Type:  vb.Type,
			Value: snmp.NormalizeVarBindValue(vb.Value),
		}
		if res, ok := mib.ResolveOID(clean); ok {
			d.Symbol = res.Symbol
			d.Table = res.Table
			d.Instance = res.Instance
		}
		decoded = append(decoded, d)
	}
	return decoded
}

func formatConsole(result output) string {
	var b strings.Builder
	fmt.Fprintln(&b, "NPMS SNMP check")
	fmt.Fprintf(&b, "Host/IP: %s\n", result.Host)
	fmt.Fprintln(&b, "Status:  OK")
	fmt.Fprintln(&b, "\nPrinter information:")
	seenSerial := false
	for _, oid := range []string{sysNameOID, printerNameInsOID, sysDescrOID, sysObjectOID, serialInstanceOID, serialOID} {
		if (oid == serialInstanceOID || oid == serialOID) && seenSerial {
			continue
		}
		if value, ok := findValue(result.Results, oid); ok {
			if oid == serialInstanceOID || oid == serialOID {
				seenSerial = true
			}
			fmt.Fprintf(&b, "  %-12s %v\n", systemLabels[oid]+":", value)
		}
	}
	if len(result.Markers) == 0 {
		fmt.Fprintln(&b, "\nCounter values: no validated marker rows returned")
	} else {
		fmt.Fprintln(&b, "\nCounter values:")
		fmt.Fprintln(&b, "  INSTANCE     UNIT    LIFE_COUNT")
		for _, marker := range result.Markers {
			fmt.Fprintf(&b, "  %-12s %-7d %d\n", marker.Instance, marker.Unit, marker.Life)
		}
	}
	if len(result.Supplies) > 0 {
		fmt.Fprintln(&b, "\nSupplies (Printer-MIB):")
		fmt.Fprintln(&b, "  INSTANCE     LEVEL   MAX     DESCRIPTION")
		for _, sup := range result.Supplies {
			fmt.Fprintf(&b, "  %-12s %-7d %-7d %s\n", sup.Instance, sup.Level, sup.MaxCapacity, sup.Description)
		}
	}
	if len(result.ConsoleLines) > 0 {
		fmt.Fprintln(&b, "\nConsole display:")
		for _, line := range result.ConsoleLines {
			fmt.Fprintf(&b, "  [%s] %s\n", line.Instance, line.Text)
		}
	}
	return b.String()
}

func findValue(results []snmp.VarBind, oid string) (any, bool) {
	for _, result := range results {
		if strings.TrimPrefix(result.OID, ".") == oid && !snmp.IsExceptionVarBind(result) {
			if s, ok := snmp.StringValue(result.Value); ok && s != "" {
				return s, true
			}
		}
	}
	return nil, false
}

func printUsage(w io.Writer) {
	usage := `NPMS SNMP Diagnostic CLI (snmp-debug)

Usage:
  snmp-debug <command> [options]

Available Commands:
  check     Perform console health, marker counter & supply check (human-readable table output)
  probe     Probe printer identity, marker OIDs and Printer-MIB tables (JSON output)
  collect   Collect structured Printer-MIB snapshot decoded with Printer-MIB.yaml (JSON output)
  get       Fetch a single OID value (requires --oid)
  walk      Walk an OID subtree (requires --oid)
  export    Export full printer diagnostics to a JSON file (requires --output)
  help      Show this help message

Global Options:
  --host string            Printer hostname or IP address (required unless --fixture is set)
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
  --output string          Target file path for 'export' or 'collect' command
  --mib-file string        Optional path to Printer-MIB.yaml for OID symbol resolution
  --fixture string         Optional path to a Printer-MIB JSON fixture for offline replay
  --anonymize              Mask host IP and serial numbers in exported/collected JSON

Examples:
  # SNMP v2c console check:
  snmp-debug check --host 192.168.1.50 --community public

  # Collect anonymized Printer-MIB fixture using Printer-MIB.yaml:
  snmp-debug collect --host 192.168.1.50 --community public --mib-file ./Printer-MIB.yaml --anonymize --output sample.json

  # Replay a captured fixture offline:
  snmp-debug check --fixture ./backend/testdata/printer-mib/hp-laserjet-enterprise-m501dn.json
`
	fmt.Fprintln(w, strings.TrimSpace(usage))
}
