package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database  Database  `yaml:"database"`
	Server    Server    `yaml:"server"`
	Security  Security  `yaml:"security"`
	SNMP      SNMP      `yaml:"snmp"`
	Discovery Discovery `yaml:"discovery"`
	Polling   Polling   `yaml:"polling"`
	Counter   Counter   `yaml:"counter"`
	Report    Report    `yaml:"report"`
	Retention Retention `yaml:"retention"`
	Logging   Logging   `yaml:"logging"`
	Profiles  Profiles  `yaml:"profiles"`
}

type Database struct {
	Path string `yaml:"path"`
}
type Server struct {
	Listen        string `yaml:"listen"`
	AllowedOrigin string `yaml:"allowed_origin"`
	APIToken      string `yaml:"api_token"`
	OpenBrowser   bool   `yaml:"open_browser"`
}
type Security struct {
	EncryptionKey string `yaml:"encryption_key"`
}
type SNMP struct {
	DefaultPort      int    `yaml:"default_port"`
	DefaultCommunity string `yaml:"default_community"`
	DefaultVersion   string `yaml:"default_version"`
	TimeoutSeconds   int    `yaml:"timeout_seconds"`
	Retries          int    `yaml:"retries"`
	MaxRepetitions   int    `yaml:"max_repetitions"`
}
type Discovery struct {
	RulesFile        string   `yaml:"rules_file"`
	DefaultCIDR      string   `yaml:"default_cidr"`
	DefaultCommunity string   `yaml:"default_community"`
	DefaultPort      int      `yaml:"default_port"`
	AllowedCIDRs     []string `yaml:"allowed_cidrs"`
	MaxRangeIPs      int      `yaml:"max_range_ips"`
	Concurrency      int      `yaml:"concurrency"`
	TimeoutMS        int      `yaml:"timeout_ms"`
}
type Counter struct {
	MaxDelta int64 `yaml:"max_delta"`
}
type Polling struct {
	Concurrency     int    `yaml:"concurrency"`
	StatusInterval  string `yaml:"status_interval"`
	CounterInterval string `yaml:"counter_interval"`
	MaxAttempts     int    `yaml:"max_attempts"`
	RetryDelay      string `yaml:"retry_delay"`
	LeaseDuration   string `yaml:"lease_duration"`
}
type Profiles struct {
	Path string `yaml:"path"`
}
type Report struct {
	Timezone string `yaml:"timezone"`
}
type Retention struct {
	PollingRunsDays   int    `yaml:"polling_runs_days"`
	CounterEventsDays int    `yaml:"counter_events_days"`
	JobsDays          int    `yaml:"jobs_days"`
	CleanupInterval   string `yaml:"cleanup_interval"`
}
type Logging struct {
	ErrorFile string `yaml:"error_file"`
	Daily     bool   `yaml:"daily"`
	MaxSizeMB int    `yaml:"max_size_mb"`
}

func Defaults() Config {
	return Config{
		Database: Database{Path: "./data/npms.db"},
		Server:   Server{Listen: ":8080", AllowedOrigin: "http://localhost:5173", OpenBrowser: true},
		Security: Security{},
		SNMP: SNMP{
			DefaultPort:      161,
			DefaultCommunity: "public",
			DefaultVersion:   "2c",
			TimeoutSeconds:   4,
			Retries:          1,
			MaxRepetitions:   25,
		},
		Discovery: Discovery{
			RulesFile:        "./discovery.yaml",
			DefaultCIDR:      "",
			DefaultCommunity: "public",
			DefaultPort:      161,
			AllowedCIDRs:     nil,
			MaxRangeIPs:      512,
			Concurrency:      25,
			TimeoutMS:        1000,
		},
		Polling: Polling{
			Concurrency:     5,
			StatusInterval:  "15m",
			CounterInterval: "6h",
			MaxAttempts:     2,
			RetryDelay:      "1s",
			LeaseDuration:   "45s",
		},
		Counter: Counter{
			MaxDelta: 0,
		},
		Report:    Report{Timezone: "UTC"},
		Retention: Retention{PollingRunsDays: 30, CounterEventsDays: 90, JobsDays: 30, CleanupInterval: "24h"},
		Logging:   Logging{ErrorFile: "./data/npms-errors.log", Daily: true, MaxSizeMB: 10},
		Profiles:  Profiles{Path: "./profiles"},
	}
}

func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, errors.New("config path is required")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	result := Defaults()
	decoder := yaml.NewDecoder(strings.NewReader(string(contents)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := result.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %q: %w", path, err)
	}
	base := filepath.Dir(path)
	if !filepath.IsAbs(result.Database.Path) {
		result.Database.Path = filepath.Clean(filepath.Join(base, result.Database.Path))
	}
	if !filepath.IsAbs(result.Profiles.Path) {
		result.Profiles.Path = filepath.Clean(filepath.Join(base, result.Profiles.Path))
	}
	if !filepath.IsAbs(result.Logging.ErrorFile) {
		result.Logging.ErrorFile = filepath.Clean(filepath.Join(base, result.Logging.ErrorFile))
	}
	if result.Discovery.RulesFile != "" && !filepath.IsAbs(result.Discovery.RulesFile) {
		result.Discovery.RulesFile = filepath.Clean(filepath.Join(base, result.Discovery.RulesFile))
	}
	return result, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Database.Path) == "" {
		return errors.New("database.path is required")
	}
	if strings.TrimSpace(c.Server.Listen) == "" {
		return errors.New("server.listen is required")
	}
	if strings.TrimSpace(c.Server.AllowedOrigin) == "" {
		return errors.New("server.allowed_origin is required")
	}
	if c.Server.APIToken == "replace-with-a-random-api-token" {
		return errors.New("server.api_token must not use the example placeholder")
	}
	if strings.TrimSpace(c.Profiles.Path) == "" {
		return errors.New("profiles.path is required")
	}
	if c.Polling.Concurrency < 1 || c.Polling.Concurrency > 100 {
		return errors.New("polling.concurrency must be between 1 and 100")
	}
	if c.Polling.MaxAttempts < 1 || c.Polling.MaxAttempts > 10 {
		return errors.New("polling.max_attempts must be between 1 and 10")
	}
	if _, err := c.RetryDelay(); err != nil {
		return err
	}
	if _, err := c.LeaseDuration(); err != nil {
		return err
	}
	if _, err := c.StatusInterval(); err != nil {
		return err
	}
	if _, err := c.CounterInterval(); err != nil {
		return err
	}
	if c.SNMP.DefaultPort < 1 || c.SNMP.DefaultPort > 65535 {
		return errors.New("snmp.default_port must be between 1 and 65535")
	}
	if c.SNMP.TimeoutSeconds < 1 || c.SNMP.TimeoutSeconds > 60 {
		return errors.New("snmp.timeout_seconds must be between 1 and 60")
	}
	if c.SNMP.Retries < 0 || c.SNMP.Retries > 10 {
		return errors.New("snmp.retries must be between 0 and 10")
	}
	if c.SNMP.MaxRepetitions < 1 || c.SNMP.MaxRepetitions > 100 {
		return errors.New("snmp.max_repetitions must be between 1 and 100")
	}
	if c.Discovery.DefaultPort < 1 || c.Discovery.DefaultPort > 65535 {
		return errors.New("discovery.default_port must be between 1 and 65535")
	}
	if c.Discovery.MaxRangeIPs < 1 || c.Discovery.MaxRangeIPs > 2048 {
		return errors.New("discovery.max_range_ips must be between 1 and 2048")
	}
	if c.Discovery.Concurrency < 1 || c.Discovery.Concurrency > 100 {
		return errors.New("discovery.concurrency must be between 1 and 100")
	}
	if c.Discovery.TimeoutMS < 100 || c.Discovery.TimeoutMS > 10000 {
		return errors.New("discovery.timeout_ms must be between 100 and 10000")
	}
	for _, cidr := range c.Discovery.AllowedCIDRs {
		if _, _, err := net.ParseCIDR(strings.TrimSpace(cidr)); err != nil {
			return fmt.Errorf("discovery.allowed_cidrs contains invalid CIDR %q: %w", cidr, err)
		}
	}
	if c.Counter.MaxDelta < 0 {
		return errors.New("counter.max_delta must be greater than or equal to 0")
	}
	if strings.TrimSpace(c.Report.Timezone) == "" {
		return errors.New("report.timezone is required")
	}
	if _, err := time.LoadLocation(c.Report.Timezone); err != nil {
		return fmt.Errorf("report.timezone must be a valid IANA timezone: %w", err)
	}
	if c.Retention.PollingRunsDays < 1 || c.Retention.CounterEventsDays < 1 || c.Retention.JobsDays < 1 {
		return errors.New("retention day values must be greater than zero")
	}
	if _, err := c.CleanupInterval(); err != nil {
		return err
	}
	if strings.TrimSpace(c.Logging.ErrorFile) == "" {
		return errors.New("logging.error_file is required")
	}
	if c.Logging.MaxSizeMB < 1 || c.Logging.MaxSizeMB > 1024 {
		return errors.New("logging.max_size_mb must be between 1 and 1024")
	}
	return nil
}

func (c Config) RetryDelay() (time.Duration, error) {
	if strings.TrimSpace(c.Polling.RetryDelay) == "" {
		return time.Second, nil
	}
	return parseDuration("polling.retry_delay", c.Polling.RetryDelay)
}

func (c Config) LeaseDuration() (time.Duration, error) {
	if strings.TrimSpace(c.Polling.LeaseDuration) == "" {
		return 45 * time.Second, nil
	}
	return parseDuration("polling.lease_duration", c.Polling.LeaseDuration)
}

func (c Config) StatusInterval() (time.Duration, error) {
	return parseDuration("polling.status_interval", c.Polling.StatusInterval)
}
func (c Config) CounterInterval() (time.Duration, error) {
	return parseDuration("polling.counter_interval", c.Polling.CounterInterval)
}
func (c Config) CleanupInterval() (time.Duration, error) {
	return parseDuration("retention.cleanup_interval", c.Retention.CleanupInterval)
}

func parseDuration(name, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return duration, nil
}
