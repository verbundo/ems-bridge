package main

import (
	"fmt"
	"log"
	"os"

	"ems-bridge/encr"
	"ems-bridge/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: encr <string>")
		os.Exit(1)
	}
	plaintext := os.Args[1]

	db, err := sqlite.OpenDB("config.db")
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
