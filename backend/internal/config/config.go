package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database Database `yaml:"database"`
	Server   Server   `yaml:"server"`
	Security Security `yaml:"security"`
	Polling  Polling  `yaml:"polling"`
	Profiles Profiles `yaml:"profiles"`
}

type Database struct {
	Path string `yaml:"path"`
}
type Server struct {
	Listen        string `yaml:"listen"`
	AllowedOrigin string `yaml:"allowed_origin"`
	APIToken      string `yaml:"api_token"`
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

func Defaults() Config {
	return Config{Database: Database{Path: "./data/npms.db"}, Server: Server{Listen: ":8080", AllowedOrigin: "http://localhost:5173"}, Polling: Polling{Concurrency: 5, StatusInterval: "5m", CounterInterval: "15m"}, Profiles: Profiles{Path: "./profiles"}}
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
	return nil
}

func (c Config) StatusInterval() (time.Duration, error) {
	return parseDuration("polling.status_interval", c.Polling.StatusInterval)
}
func (c Config) CounterInterval() (time.Duration, error) {
	return parseDuration("polling.counter_interval", c.Polling.CounterInterval)
}

func parseDuration(name, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return duration, nil
}
