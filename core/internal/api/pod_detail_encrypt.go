package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"sync"

	"github.com/fortuna/core/pkg/models"
)

var (
	encKey     []byte
	encKeyOnce sync.Once
)

// InitPodDetailEncryptionKey sets the key used for encrypt/decrypt from config (e.g. cfg.PodDetailEncryptionKey).
// Key must be base64-encoded 32 bytes. If empty or invalid, encryption is disabled. Call once at startup (e.g. from SetupRoutes).
func InitPodDetailEncryptionKey(keyBase64 string) {
	encKeyOnce.Do(func() {
		if keyBase64 == "" {
			keyBase64 = os.Getenv("POD_DETAIL_ENCRYPTION_KEY")
		}
		if keyBase64 == "" {
			return
		}
		key, err := base64.StdEncoding.DecodeString(keyBase64)
		if err != nil || len(key) != 32 {
			return
		}
		encKey = key
	})
}

func getPodDetailEncryptionKey() []byte {
	return encKey
}

// EncryptSensitive encrypts plaintext with AES-256-GCM (Phase 4.2). Key from POD_DETAIL_ENCRYPTION_KEY (base64 32 bytes). If key not set, returns plaintext unchanged.
func EncryptSensitive(plaintext string) string {
	key := getPodDetailEncryptionKey()
	if len(key) == 0 || plaintext == "" {
		return plaintext
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return plaintext
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return plaintext
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return plaintext
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	out := make([]byte, len(nonce)+len(ciphertext))
	copy(out, nonce)
	copy(out[len(nonce):], ciphertext)
	return base64.StdEncoding.EncodeToString(out)
}

// DecryptSensitive decrypts payload from EncryptSensitive. If key not set or decrypt fails, returns ciphertext unchanged.
func DecryptSensitive(ciphertext string) string {
	key := getPodDetailEncryptionKey()
	if len(key) == 0 || ciphertext == "" {
		return ciphertext
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil || len(raw) < 12+16 {
		return ciphertext
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return ciphertext
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ciphertext
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return ciphertext
	}
	plain, err := gcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return ciphertext
	}
	return string(plain)
}

// decryptProcessList decrypts sensitive process fields in place for API response.
func decryptProcessList(list []models.PodProcess) {
	for i := range list {
		if list[i].Command != "" {
			if dec := DecryptSensitive(list[i].Command); dec != list[i].Command {
				list[i].Command = dec
			}
		}
		if list[i].BinaryPath != "" {
			if dec := DecryptSensitive(list[i].BinaryPath); dec != list[i].BinaryPath {
				list[i].BinaryPath = dec
			}
		}
		if list[i].WorkingDir != "" {
			if dec := DecryptSensitive(list[i].WorkingDir); dec != list[i].WorkingDir {
				list[i].WorkingDir = dec
			}
		}
	}
}
