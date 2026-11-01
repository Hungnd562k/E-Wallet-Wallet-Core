package domain

import (
	"github.com/Hungnd562k/E-Wallet-Wallet-Core/internal/platform/apperr"
	"github.com/alexedwards/argon2id"
)

var (
	ErrInvalidPin = apperr.New(apperr.KindForbidden, "INVALID_PIN", "Invalid PIN")
)

// Hash PIN sử dụng argon2id :D
func Hash_PIN(pin string) (string, error) {
	return argon2id.CreateHash(pin, argon2id.DefaultParams)
}

// Compare pin and hash pin :D .
// TODO: Tìm hiểu về Poison Hash, phép tính gây tốn tài nguyên, benchmark login đồng thời.
func Verify(pin string, encodedHash string) error {
	match, err := argon2id.ComparePasswordAndHash(pin, encodedHash)
	if err != nil {
		return err
	}

	if !match {
		return ErrInvalidPin
	}

	return nil
}
