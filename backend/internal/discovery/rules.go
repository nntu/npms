package discovery

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Rules defines configurable heuristics for vendor identification, model extraction,
// and device classification (printer vs scanner).
type Rules struct {
	VendorOIDs           map[string]string   `yaml:"vendor_oids"`
	VendorKeywords       map[string][]string `yaml:"vendor_keywords"`
	ScannerKeywords      []string            `yaml:"scanner_keywords"`
	ScannerModelPrefixes []string            `yaml:"scanner_model_prefixes"`
	PrinterKeywords      []string            `yaml:"printer_keywords"`
	ModelExtraction      ModelExtraction     `yaml:"model_extraction"`
}

type ModelExtraction struct {
	AdapterPrefixes []string `yaml:"adapter_prefixes"`
	TokenPrefixes   []string `yaml:"token_prefixes"`
}

var (
	globalRulesMu sync.RWMutex
	globalRules   = DefaultRules()
)

// SetGlobalRules updates the active discovery rules.
func SetGlobalRules(r Rules) {
	globalRulesMu.Lock()
	defer globalRulesMu.Unlock()
	globalRules = r
}

// GetGlobalRules returns a copy of the active discovery rules.
func GetGlobalRules() Rules {
	globalRulesMu.RLock()
	defer globalRulesMu.RUnlock()
	return globalRules
}

// DefaultRules provides built-in fallback rules.
func DefaultRules() Rules {
	return Rules{
		VendorOIDs: map[string]string{
			"1.3.6.1.4.1.11":   "HP",
			"1.3.6.1.4.1.2435": "Brother",
			"1.3.6.1.4.1.367":  "Ricoh",
			"1.3.6.1.4.1.1602": "Canon",
			"1.3.6.1.4.1.1248": "Epson",
			"1.3.6.1.4.1.1347": "Kyocera",
			"1.3.6.1.4.1.253":  "Xerox",
			"1.3.6.1.4.1.641":  "Lexmark",
		},
		VendorKeywords: map[string][]string{
			"HP":      {"hewlett-packard", "laserjet", "hp "},
			"Brother": {"brother"},
			"Ricoh":   {"ricoh"},
			"Canon":   {"canon"},
			"Epson":   {"epson"},
			"Kyocera": {"kyocera"},
			"Xerox":   {"xerox"},
			"Lexmark": {"lexmark"},
		},
		ScannerKeywords: []string{
			"scanner", "scanjet", "imageformula", "scansnap",
			"documate", "workforce ds-", "sheetfed", "flatbed scanner",
			"ads-", "nc-04h", "nc-10h",
		},
		ScannerModelPrefixes: []string{
			"ADS-", "FI-", "DR-", "KV-S",
		},
		PrinterKeywords: []string{
			"print", "laserjet", "pagepro", "ricoh", "brother", "epson", "canon",
			"kyocera", "xerox", "lexmark", "inkjet", "copier", "mfp",
			"scan", "scanner", "scanjet", "ads-", "nc-04h", "nc-10h", "imageformula", "scansnap", "fujitsu", "plustek", "avision",
		},
		ModelExtraction: ModelExtraction{
			AdapterPrefixes: []string{"Brother NC-"},
			TokenPrefixes:   []string{"PID:", "MODEL:", "MDL:"},
		},
	}
}

// LoadRules reads and parses a rules YAML file, overlaying it on top of DefaultRules.
func LoadRules(path string) (Rules, error) {
	if strings.TrimSpace(path) == "" {
		return DefaultRules(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultRules(), nil
		}
		return DefaultRules(), fmt.Errorf("read discovery rules %q: %w", path, err)
	}

	result := DefaultRules()
	var fileRules Rules
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&fileRules); err != nil {
		return DefaultRules(), fmt.Errorf("parse discovery rules %q: %w", path, err)
	}

	// Merge maps & non-empty slices
	if len(fileRules.VendorOIDs) > 0 {
		for k, v := range fileRules.VendorOIDs {
			result.VendorOIDs[strings.TrimPrefix(k, ".")] = v
		}
	}
	if len(fileRules.VendorKeywords) > 0 {
		for k, v := range fileRules.VendorKeywords {
			result.VendorKeywords[k] = v
		}
	}
	if len(fileRules.ScannerKeywords) > 0 {
		result.ScannerKeywords = fileRules.ScannerKeywords
	}
	if len(fileRules.ScannerModelPrefixes) > 0 {
		result.ScannerModelPrefixes = fileRules.ScannerModelPrefixes
	}
	if len(fileRules.PrinterKeywords) > 0 {
		result.PrinterKeywords = fileRules.PrinterKeywords
	}
	if len(fileRules.ModelExtraction.AdapterPrefixes) > 0 {
		result.ModelExtraction.AdapterPrefixes = fileRules.ModelExtraction.AdapterPrefixes
	}
	if len(fileRules.ModelExtraction.TokenPrefixes) > 0 {
		result.ModelExtraction.TokenPrefixes = fileRules.ModelExtraction.TokenPrefixes
	}

	return result, nil
}
