package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/pkg/models"
)

// encryptedPrefix marks values written by EncryptSensitive, so Core can tell
// ciphertext it cannot read (key removed) from legacy plaintext rows.
const encryptedPrefix = "enc:v1:"

// unreadableSensitiveValue is shown instead of ciphertext no configured key opens.
const unreadableSensitiveValue = "[encrypted with a key Core no longer has]"

var (
	encMu sync.RWMutex
	// encKeys[0] encrypts; every key is tried to decrypt (current first, then previous keys).
	encKeys [][]byte
)

// InitPodDetailEncryptionKey installs the keys used for Pod Detail process fields.
// Config loading has already validated them, so a parse error here only logs.
func InitPodDetailEncryptionKey(current, previous string) {
	keys, err := config.ParsePodDetailEncryptionKeys(current, previous)
	if err != nil {
		log.Printf("[PodDetail] encryption disabled: %v", err)
		keys = nil
	}
	encMu.Lock()
	encKeys = keys
	encMu.Unlock()
	if len(keys) == 0 {
		log.Printf("[PodDetail] WARNING: POD_DETAIL_ENCRYPTION_KEY is not set; process command lines are stored in plaintext")
	}
}

func podDetailEncryptionKeys() [][]byte {
	encMu.RLock()
	defer encMu.RUnlock()
	return encKeys
}

// EncryptSensitive encrypts plaintext with AES-256-GCM under the current key.
// Without a key it returns plaintext. If encryption fails it returns "" rather
// than storing the plaintext.
func EncryptSensitive(plaintext string) string {
	keys := podDetailEncryptionKeys()
	if len(keys) == 0 || plaintext == "" {
		return plaintext
	}
	gcm, err := newGCM(keys[0])
	if err != nil {
		log.Printf("[PodDetail] encryption failed, dropping value: %v", err)
		return ""
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		log.Printf("[PodDetail] encryption failed, dropping value: %v", err)
		return ""
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encryptedPrefix + base64.StdEncoding.EncodeToString(sealed)
}

// DecryptSensitive reverses EncryptSensitive with the current or a previous key.
// Values without the prefix may be legacy ciphertext (written before the prefix
// existed) or plaintext stored while encryption was off; those are returned
// unchanged when no key opens them.
func DecryptSensitive(value string) string {
	if value == "" {
		return value
	}
	keys := podDetailEncryptionKeys()
	if body, ok := strings.CutPrefix(value, encryptedPrefix); ok {
		if plain, ok := openWithKeys(keys, body); ok {
			return plain
		}
		return unreadableSensitiveValue
	}
	if plain, ok := openWithKeys(keys, value); ok {
		return plain
	}
	return value
}

func openWithKeys(keys [][]byte, encoded string) (string, bool) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}
	for _, key := range keys {
		gcm, err := newGCM(key)
		if err != nil || len(raw) < gcm.NonceSize()+gcm.Overhead() {
			continue
		}
		n := gcm.NonceSize()
		if plain, err := gcm.Open(nil, raw[:n], raw[n:], nil); err == nil {
			return string(plain), true
		}
	}
	return "", false
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// decryptProcessList decrypts sensitive process fields in place for API response.
func decryptProcessList(list []models.PodProcess) {
	for i := range list {
		list[i].Command = DecryptSensitive(list[i].Command)
		list[i].BinaryPath = DecryptSensitive(list[i].BinaryPath)
		list[i].WorkingDir = DecryptSensitive(list[i].WorkingDir)
	}
}
