package sqlite

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	_ "modernc.org/sqlite"
)

func OpenDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS keys (
		id    INTEGER PRIMARY KEY AUTOINCREMENT,
		value TEXT NOT NULL
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("creating keys table: %w", err)
	}

	return db, nil
}

func SeedKeys(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM keys`).Scan(&count); err != nil {
		return fmt.Errorf("counting keys: %w", err)
	}
	if count > 0 {
		return nil
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("generating random value: %w", err)
	}

	_, err := db.Exec(`INSERT INTO keys (id, value) VALUES (1, ?)`, hex.EncodeToString(buf))
	if err != nil {
		return fmt.Errorf("inserting seed key: %w", err)
	}
	return nil
}
