package getui

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func GenSign(parts ...string) string {
	joined := strings.Join(parts, "")
	hash := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(hash[:])
}

func Decrypt(content, secret string) (string, error) {
	iv := []byte("0000000000000000")
	key := []byte(secret)
	for len(key) < 16 {
		key = append(key, []byte(secret)...)
	}
	key = key[:16]

	ciphertext, err := hex.DecodeString(content)
	if err != nil {
		return "", fmt.Errorf("hex decode failed: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher failed: %w", err)
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid ciphertext block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)
	plaintext, err := pkcs7Unpad(ciphertext)
	if err != nil {
		return "", fmt.Errorf("pkcs7 unpad failed: %w", err)
	}
	return string(plaintext), nil
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	padding := int(data[len(data)-1])
	if padding <= 0 || padding > len(data) {
		return nil, fmt.Errorf("invalid padding")
	}
	return data[:len(data)-padding], nil
}
