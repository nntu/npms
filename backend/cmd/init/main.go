package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"npms/backend/internal/config"
)

func main() {
	configPath := flag.String("config", "./config.yaml", "Target YAML configuration file path")
	flag.StringVar(configPath, "c", "./config.yaml", "Target YAML configuration file path (shorthand)")
	flag.StringVar(configPath, "output", "./config.yaml", "Target YAML configuration file path (alias)")

	templatePath := flag.String("template", "./config.example.yaml", "Source template configuration file path")
	flag.StringVar(templatePath, "t", "./config.example.yaml", "Source template configuration file path (shorthand)")

	force := flag.Bool("force", false, "Overwrite configuration file if it already exists")
	flag.BoolVar(force, "f", false, "Overwrite configuration file if it already exists (shorthand)")

	genAPIToken := flag.Bool("generate-api-token", false, "Generate a random API bearer token in server.api_token")
	customKey := flag.String("key", "", "Custom 32-byte encryption key (base64 or hex). If omitted, a random key is generated")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "NPMS Configuration Initializer (npms-init)")
		fmt.Fprintln(os.Stderr, "Generates a config.yaml file with a cryptographically secure 32-byte encryption key.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  npms-init [options]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
	}

	flag.Parse()

	res, err := config.InitConfig(config.InitOptions{
		Path:             *configPath,
		TemplatePath:     *templatePath,
		Force:            *force,
		GenerateAPIToken: *genAPIToken,
		EncryptionKey:    *customKey,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	absPath, _ := filepath.Abs(res.ConfigPath)
	fmt.Println("NPMS Configuration Initializer")
	fmt.Println("------------------------------")
	fmt.Printf("Status:           Successfully created configuration\n")
	fmt.Printf("Config Path:      %s\n", absPath)
	fmt.Println("Secrets:          written to the permission-protected configuration file")
	fmt.Printf("Database Path:    %s\n", res.Config.Database.Path)
	fmt.Printf("Profiles Path:    %s\n", res.Config.Profiles.Path)
	fmt.Println("------------------------------")
	fmt.Println("Initialization complete! Keep your encryption key safe.")
}
