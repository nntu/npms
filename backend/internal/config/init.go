package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"npms/backend/internal/security"

	"gopkg.in/yaml.v3"
)

type InitOptions struct {
	Path             string
	TemplatePath     string
	Force            bool
	GenerateAPIToken bool
	EncryptionKey    string
}

// InitResult contains metadata about the initialized configuration.
type InitResult struct {
	ConfigPath    string
	EncryptionKey string
	APIToken      string
	Config        Config
}

// InitConfig initializes a new configuration file at the specified path.
// It generates a secure random 32-byte encryption key if none is provided.
func InitConfig(opts InitOptions) (*InitResult, error) {
	targetPath := strings.TrimSpace(opts.Path)
	if targetPath == "" {
		targetPath = "./config.yaml"
	}

	if _, err := os.Stat(targetPath); err == nil && !opts.Force {
		return nil, fmt.Errorf("configuration file already exists at %q (use --force to overwrite)", targetPath)
	}

	encKey := strings.TrimSpace(opts.EncryptionKey)
	if encKey == "" {
		var err error
		encKey, err = security.GenerateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate encryption key: %w", err)
		}
	} else {
		if _, err := security.ParseKey(encKey); err != nil {
			return nil, fmt.Errorf("invalid encryption key provided: %w", err)
		}
	}

	var apiToken string
	if opts.GenerateAPIToken {
		var err error
		apiToken, err = security.GenerateAPIToken()
		if err != nil {
			return nil, fmt.Errorf("failed to generate API token: %w", err)
		}
	}

	templatePath := strings.TrimSpace(opts.TemplatePath)
	if templatePath == "" {
		templatePath = "./config.example.yaml"
	}

	var fileContent string
	templateData, err := os.ReadFile(templatePath)
	if err == nil {
		content := string(templateData)
		if strings.Contains(content, "replace-with-a-32-byte-base64-or-64-character-hex-key") {
			content = strings.ReplaceAll(content, "replace-with-a-32-byte-base64-or-64-character-hex-key", encKey)
		} else {
			content = replaceOrAppendYAMLKey(content, "encryption_key", encKey)
		}

		if opts.GenerateAPIToken && apiToken != "" {
			content = strings.ReplaceAll(content, `api_token: ""`, fmt.Sprintf(`api_token: %q`, apiToken))
		}
		fileContent = content
	} else {
		cfg := Defaults()
		cfg.Security.EncryptionKey = encKey
		if apiToken != "" {
			cfg.Server.APIToken = apiToken
		}
		out, err := yaml.Marshal(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default config: %w", err)
		}
		fileContent = string(out)
	}

	dir := filepath.Dir(targetPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create config directory %q: %w", dir, err)
		}
	}

	if err := os.WriteFile(targetPath, []byte(fileContent), 0600); err != nil {
		return nil, fmt.Errorf("write config file %q: %w", targetPath, err)
	}

	cfg, err := Load(targetPath)
	if err != nil {
		return nil, fmt.Errorf("created config validation failed: %w", err)
	}

	if dbDir := filepath.Dir(cfg.Database.Path); dbDir != "" && dbDir != "." {
		_ = os.MkdirAll(dbDir, 0755)
	}
	if logDir := filepath.Dir(cfg.Logging.ErrorFile); logDir != "" && logDir != "." {
		_ = os.MkdirAll(logDir, 0755)
	}

	return &InitResult{
		ConfigPath:    targetPath,
		EncryptionKey: encKey,
		APIToken:      apiToken,
		Config:        cfg,
	}, nil
}

func replaceOrAppendYAMLKey(yamlContent, key, value string) string {
	lines := strings.Split(yamlContent, "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+":") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = fmt.Sprintf("%s%s: %s", indent, key, value)
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("%s: %s", key, value))
	}
	return strings.Join(lines, "\n")
}
