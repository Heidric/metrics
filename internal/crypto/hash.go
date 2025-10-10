package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// HashSHA256 returns the hex-encoded HMAC-SHA256 of data using key.
// The returned string is lowercase hexadecimal.
func HashSHA256(data []byte, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
