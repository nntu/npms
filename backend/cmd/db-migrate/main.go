package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"npms/backend/internal/config"
	"npms/backend/internal/database"
)

func main() {
	configPath := flag.String("config", "./config.yaml", "YAML configuration path")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		fail(err)
	}
	db, err := database.OpenSQLite(cfg.Database.Path)
	if err != nil {
		fail(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		fail(err)
	}
	fmt.Printf("SQLite database ready: %s\n", cfg.Database.Path)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
