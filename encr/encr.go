package encr

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

func Encrypt(db *sql.DB, plaintext string) (string, error) {
	var keyHex string
	err := db.QueryRow(`SELECT value FROM keys ORDER BY RANDOM() LIMIT 1`).Scan(&keyHex)
	if err != nil {
		return "", fmt.Errorf("reading key: %w", err)
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", fmt.Errorf("decoding key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return fmt.Sprintf("encr:%s:%s", keyHex[:8], hex.EncodeToString(ciphertext)), nil
}

func Decrypt(db *sql.DB, data string) (string, error) {
	parts := strings.SplitN(data, ":", 3)
	if len(parts) != 3 || parts[0] != "encr" {
		return "", fmt.Errorf("invalid format, expected encr:key_prefix:data")
	}
	keyPrefix, ciphertextHex := parts[1], parts[2]

	var keyHex string
	err := db.QueryRow(`SELECT value FROM keys WHERE value LIKE ? LIMIT 1`, keyPrefix+"%").Scan(&keyHex)
	if err != nil {
		return "", fmt.Errorf("reading key with prefix %q: %w", keyPrefix, err)
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", fmt.Errorf("decoding key: %w", err)
	}

	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", fmt.Errorf("decoding ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	if len(ciphertext) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypting: %w", err)
	}

	return string(plaintext), nil
}
