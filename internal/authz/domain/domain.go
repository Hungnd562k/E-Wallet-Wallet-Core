package domain

import (
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/crypto/argon2"
)

type PinError string

const (
	InvalidPIN PinError = "INVALID_PIN"
)

// Hash PIN sử dụng argon2id :D
func Hash_PIN(pin string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", nil
	}

	const (
		memory      = 64 * 1024
		iteration   = 3
		parallelism = 2
		keyLength   = 32
	)

	hash := argon2.IDKey(
		[]byte(pin),
		salt,
		iteration,
		memory,
		parallelism,
		keyLength,
	)

	return base64.RawStdEncoding.EncodeToString(hash), nil
}
