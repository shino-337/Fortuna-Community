package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func testKey(t *testing.T) (string, []byte) {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(key), key
}

func withEncryptionKeys(t *testing.T, current, previous string) {
	t.Helper()
	InitPodDetailEncryptionKey(current, previous)
	t.Cleanup(func() { InitPodDetailEncryptionKey("", "") })
}

func TestEncryptSensitiveRoundTripUsesPrefix(t *testing.T) {
	cur, _ := testKey(t)
	withEncryptionKeys(t, cur, "")

	enc := EncryptSensitive("psql --password=hunter2")
	if !strings.HasPrefix(enc, encryptedPrefix) {
		t.Fatalf("ciphertext must carry the version prefix, got %q", enc)
	}
	if strings.Contains(enc, "hunter2") {
		t.Fatal("ciphertext leaks plaintext")
	}
	if got := DecryptSensitive(enc); got != "psql --password=hunter2" {
		t.Fatalf("round trip: got %q", got)
	}
}

func TestDecryptSensitiveUsesPreviousKeyAfterRotation(t *testing.T) {
	oldKey, _ := testKey(t)
	withEncryptionKeys(t, oldKey, "")
	enc := EncryptSensitive("/usr/bin/app")

	newKey, _ := testKey(t)
	withEncryptionKeys(t, newKey, oldKey)
	if got := DecryptSensitive(enc); got != "/usr/bin/app" {
		t.Fatalf("previous key must still decrypt, got %q", got)
	}

	withEncryptionKeys(t, newKey, "")
	if got := DecryptSensitive(enc); got != unreadableSensitiveValue {
		t.Fatalf("ciphertext from a dropped key must not be shown raw, got %q", got)
	}
}

func TestDecryptSensitiveKeepsLegacyValues(t *testing.T) {
	cur, key := testKey(t)
	withEncryptionKeys(t, cur, "")

	// Plaintext stored while encryption was off.
	if got := DecryptSensitive("nginx -g daemon off;"); got != "nginx -g daemon off;" {
		t.Fatalf("plaintext row changed: %q", got)
	}

	// Ciphertext written before the prefix existed.
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	legacy := base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte("legacy"), nil))
	if got := DecryptSensitive(legacy); got != "legacy" {
		t.Fatalf("legacy ciphertext: got %q", got)
	}
}

func TestEncryptSensitiveWithoutKeyIsPlaintext(t *testing.T) {
	withEncryptionKeys(t, "", "")
	if got := EncryptSensitive("ps aux"); got != "ps aux" {
		t.Fatalf("got %q", got)
	}
}
