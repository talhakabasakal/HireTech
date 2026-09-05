package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func GenerateOTP() (string, error) {
	var bytes [4]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate otp: %w", err)
	}
	n := (uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])) % 1000000
	return fmt.Sprintf("%06d", n), nil
}

// HashOTP uses a keyed digest so a database leak does not make six-digit codes directly testable.
func HashOTP(secret, code string) string {
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(code))
	return hex.EncodeToString(h.Sum(nil))
}
