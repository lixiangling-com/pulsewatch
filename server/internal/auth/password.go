package auth

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidPassword = errors.New("invalid password")

func HashPassword(password string) (string, error) {
	if len(password) < 8 || len(password) > 72 || !utf8.ValidString(password) {
		return "", ErrInvalidPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
