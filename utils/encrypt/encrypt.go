package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

func Encrypt(text, key string) (string, error) {
	k, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(text), nil)
	return "gcm:" + base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func GenerateKey() string {
	key := make([]byte, 32) // AES-256
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(key)
}

func Decrypt(encodedCipher, key string) (string, error) {
	k, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(encodedCipher, "gcm:") {
		return decryptGCM(encodedCipher[len("gcm:"):], block)
	}

	cipherWithIV, err := base64.StdEncoding.DecodeString(encodedCipher)
	if err != nil {
		return "", err
	}

	if len(cipherWithIV) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	iv := cipherWithIV[:aes.BlockSize]
	ciphertext := cipherWithIV[aes.BlockSize:]
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid ciphertext length")
	}

	plaintextPadded := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintextPadded, ciphertext)

	padding := int(plaintextPadded[len(plaintextPadded)-1])
	if padding > aes.BlockSize || padding == 0 {
		return "", fmt.Errorf("invalid padding")
	}
	for i := len(plaintextPadded) - padding; i < len(plaintextPadded); i++ {
		if int(plaintextPadded[i]) != padding {
			return "", fmt.Errorf("invalid padding")
		}
	}
	plaintext := plaintextPadded[:len(plaintextPadded)-padding]
	return string(plaintext), nil
}

func decryptGCM(encodedCipher string, block cipher.Block) (string, error) {
	cipherWithNonce, err := base64.StdEncoding.DecodeString(encodedCipher)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(cipherWithNonce) < gcm.NonceSize()+gcm.Overhead() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce := cipherWithNonce[:gcm.NonceSize()]
	plaintext, err := gcm.Open(nil, nonce, cipherWithNonce[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("ciphertext authentication failed: %w", err)
	}
	return string(plaintext), nil
}
