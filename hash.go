package tgscreens

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash returns an eight-character, stable hash of value.
func Hash(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:4])
}

// FindByHash returns the first item whose Hash matches hash.
func FindByHash(items []string, hash string) (string, bool) {
	for _, item := range items {
		if Hash(item) == hash {
			return item, true
		}
	}
	return "", false
}
