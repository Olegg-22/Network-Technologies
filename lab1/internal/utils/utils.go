package utils

import (
	"crypto/rand"
	"encoding/hex"
)

const countByteId = 4

func RandomID() (string, error) {
	b := make([]byte, countByteId)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), err
}
