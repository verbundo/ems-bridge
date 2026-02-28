package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"ems-bridge/messages"
	"ems-bridge/sqlite"
)

func main() {
	var configPath string
	var dbPath string
	flag.StringVar(&configPath, "config", "", "path to config.yml (required)")
	flag.StringVar(&configPath, "c", "", "path to config.yml (required) (shorthand)")
	flag.StringVar(&dbPath, "db", "config.db", "path to sqlite db file containing keys table (optional)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  --config, -c string\n\tpath to config.yml (required)\n")
		fmt.Fprintf(os.Stderr, "  --db string\n\tpath to sqlite db file containing keys table (default: config.db)\n")
	}
	flag.Parse()

	if configPath == "" {
		fmt.Fprintln(os.Stderr, "error: --config/-c is required")
		flag.Usage()
		os.Exit(1)
	}

	db, err := sqlite.OpenDB(dbPath)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := sqlite.SeedKeys(db); err != nil {
		slog.Error("failed to seed keys", "err", err)
		os.Exit(1)
	}

	cfg, err := LoadConfig(configPath, db)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	if err := start(cfg); err != nil {
		slog.Error("failed to start", "err", err)
		os.Exit(1)
	}

	msg := messages.NewMessage(
		"sample payload",
		map[string]string{"filename": "order_001.txt", "source": "fs:./data/in"},
		map[string]any{"orderID": "12345", "amount": 99.99},
	)
	msg.Print()
}
