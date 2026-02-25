package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"ems-bridge/sqlite"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config.yml (required)")
	flag.StringVar(&configPath, "c", "", "path to config.yml (required) (shorthand)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  --config, -c string\n\tpath to config.yml (required)\n")
	}
	flag.Parse()

	if configPath == "" {
		fmt.Fprintln(os.Stderr, "error: --config/-c is required")
		flag.Usage()
		os.Exit(1)
	}

	db, err := sqlite.OpenDB("config.db")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := sqlite.SeedKeys(db); err != nil {
		log.Fatalf("failed to seed keys: %v", err)
	}

	cfg, err := LoadConfig(configPath, db)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	fmt.Printf("Connectors: %+v\n", cfg.Connectors)
	fmt.Printf("Routes: %+v\n", cfg.Routes)
}
