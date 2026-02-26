package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"ems-bridge/encr"
	"ems-bridge/sqlite"
)

func main() {
	var dbPath string
	flag.StringVar(&dbPath, "db", "config.db", "path to sqlite db file containing keys table (optional)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: encr [--db path] <string>\n")
		fmt.Fprintf(os.Stderr, "  --db string\n\tpath to sqlite db file containing keys table (default: config.db)\n")
	}
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: encr [--db path] <string>")
		os.Exit(1)
	}
	plaintext := flag.Arg(0)

	db, err := sqlite.OpenDB(dbPath)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	if err := sqlite.SeedKeys(db); err != nil {
		log.Fatalf("seeding keys: %v", err)
	}

	result, err := encr.Encrypt(db, plaintext)
	if err != nil {
		log.Fatalf("encrypting: %v", err)
	}

	fmt.Println(result)
}
