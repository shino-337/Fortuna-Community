package loader

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SourceManifest describes local validation expectations for a CVE source snapshot.
type SourceManifest struct {
	SourceDigest        string   `json:"sourceDigest"`
	SHA256              string   `json:"sha256"`
	RequiredAdvisoryIDs []string `json:"requiredAdvisoryIds"`
}

func LoadSourceManifest(path string) (*SourceManifest, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read source manifest: %w", err)
	}
	var manifest SourceManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("parse source manifest: %w", err)
	}
	return &manifest, nil
}

func VerifySourceManifestSignature(manifestPath, signaturePath, publicKey string) error {
	manifestPath = strings.TrimSpace(manifestPath)
	signaturePath = strings.TrimSpace(signaturePath)
	publicKey = strings.TrimSpace(publicKey)
	if signaturePath == "" && publicKey == "" {
		return nil
	}
	if manifestPath == "" {
		return fmt.Errorf("source manifest signature validation requires --source-manifest")
	}
	if signaturePath == "" || publicKey == "" {
		return fmt.Errorf("source manifest signature validation requires both signature and public key")
	}
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read source manifest for signature validation: %w", err)
	}
	signatureRaw, err := os.ReadFile(signaturePath)
	if err != nil {
		return fmt.Errorf("read source manifest signature: %w", err)
	}
	signature, err := decodeSignatureMaterial(signatureRaw, ed25519.SignatureSize)
	if err != nil {
		return fmt.Errorf("decode source manifest signature: %w", err)
	}
	keys, err := decodePublicKeys(publicKey)
	if err != nil {
		return err
	}
	for _, pub := range keys {
		if ed25519.Verify(ed25519.PublicKey(pub), manifestRaw, signature) {
			return nil
		}
	}
	if len(keys) == 1 {
		return fmt.Errorf("source manifest signature verification failed")
	}
	return fmt.Errorf("source manifest signature verification failed for %d public keys", len(keys))
}

func decodePublicKeys(value string) ([][]byte, error) {
	parts := SplitCSV(value)
	if len(parts) == 0 {
		return nil, fmt.Errorf("source manifest public key is empty")
	}
	keys := make([][]byte, 0, len(parts))
	for _, part := range parts {
		publicKeyRaw, err := readPublicKeyMaterial(part)
		if err != nil {
			return nil, err
		}
		pub, err := decodeSignatureMaterial(publicKeyRaw, ed25519.PublicKeySize)
		if err != nil {
			return nil, fmt.Errorf("decode source manifest public key: %w", err)
		}
		keys = append(keys, pub)
	}
	return keys, nil
}

func readPublicKeyMaterial(value string) ([]byte, error) {
	if raw, err := os.ReadFile(value); err == nil {
		return raw, nil
	}
	return []byte(value), nil
}

func decodeSignatureMaterial(raw []byte, expectedLen int) ([]byte, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, fmt.Errorf("empty signature material")
	}
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) == expectedLen {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(trimmed); err == nil && len(decoded) == expectedLen {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(trimmed); err == nil && len(decoded) == expectedLen {
		return decoded, nil
	}
	if len(raw) == expectedLen {
		return raw, nil
	}
	return nil, fmt.Errorf("expected %d bytes after base64, hex, or raw decoding", expectedLen)
}

func (m *SourceManifest) ExpectedDigest() string {
	if m == nil {
		return ""
	}
	if strings.TrimSpace(m.SourceDigest) != "" {
		return strings.TrimSpace(m.SourceDigest)
	}
	return strings.TrimSpace(m.SHA256)
}

func MergeRequiredAdvisoryIDs(csvIDs string, manifest *SourceManifest) string {
	ids := SplitCSV(csvIDs)
	if manifest != nil {
		ids = append(ids, manifest.RequiredAdvisoryIDs...)
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		normalized := strings.ToUpper(strings.TrimSpace(id))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return strings.Join(out, ",")
}

func MergeExpectedDigest(flagDigest string, manifest *SourceManifest) string {
	flagDigest = strings.TrimSpace(flagDigest)
	if flagDigest != "" {
		return flagDigest
	}
	if manifest == nil {
		return ""
	}
	return manifest.ExpectedDigest()
}

func ComputeSourceDigest(files []string) (string, error) {
	h := sha256.New()
	for _, file := range files {
		if _, err := io.WriteString(h, filepath.ToSlash(file)); err != nil {
			return "", err
		}
		f, err := os.Open(file)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(h, f); err != nil {
			_ = f.Close()
			return "", err
		}
		if err := f.Close(); err != nil {
			return "", err
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func ValidateRequiredAdvisories(files []string, requireIDs string) error {
	required := SplitCSV(requireIDs)
	if len(required) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		id := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		seen[strings.ToUpper(strings.TrimSpace(id))] = struct{}{}
	}
	missing := make([]string, 0)
	for _, id := range required {
		if _, ok := seen[strings.ToUpper(id)]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required advisory IDs missing from source: %s", strings.Join(missing, ","))
	}
	return nil
}

func ValidateExpectedSourceDigest(actual, expected string) error {
	expected = strings.TrimSpace(strings.ToLower(expected))
	if expected == "" {
		return nil
	}
	actual = strings.TrimSpace(strings.ToLower(actual))
	if !strings.HasPrefix(expected, "sha256:") {
		expected = "sha256:" + expected
	}
	if actual != expected {
		return fmt.Errorf("source digest mismatch: actual=%s expected=%s", actual, expected)
	}
	return nil
}

func SplitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
