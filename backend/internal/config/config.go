package config

import (
	"errors"
	"fmt"
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
	Polling   Polling   `yaml:"polling"`
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
type Polling struct {
	Concurrency     int    `yaml:"concurrency"`
	StatusInterval  string `yaml:"status_interval"`
	CounterInterval string `yaml:"counter_interval"`
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
	return Config{Database: Database{Path: "./data/npms.db"}, Server: Server{Listen: ":8080", AllowedOrigin: "http://localhost:5173", OpenBrowser: true}, Polling: Polling{Concurrency: 5, StatusInterval: "5m", CounterInterval: "15m"}, Report: Report{Timezone: "UTC"}, Retention: Retention{PollingRunsDays: 30, CounterEventsDays: 90, JobsDays: 30, CleanupInterval: "24h"}, Logging: Logging{ErrorFile: "./data/npms-errors.log", Daily: true, MaxSizeMB: 10}, Profiles: Profiles{Path: "./profiles"}}
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
	if strings.TrimSpace(c.Server.APIToken) == "" || c.Server.APIToken == "replace-with-a-random-api-token" {
		return errors.New("server.api_token is required")
	}
	if strings.TrimSpace(c.Profiles.Path) == "" {
		return errors.New("profiles.path is required")
	}
	if c.Polling.Concurrency < 1 || c.Polling.Concurrency > 100 {
		return errors.New("polling.concurrency must be between 1 and 100")
	}
	if _, err := c.StatusInterval(); err != nil {
		return err
	}
	if _, err := c.CounterInterval(); err != nil {
		return err
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
