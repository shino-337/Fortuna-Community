package loader

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceManifestMergesValidationInputs(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{
		"sourceDigest": "sha256:abc123",
		"requiredAdvisoryIds": ["GO-2023-2402", "CVE-2024-0001"]
	}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	manifest, err := LoadSourceManifest(manifestPath)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if got := MergeExpectedDigest("", manifest); got != "sha256:abc123" {
		t.Fatalf("expected digest=%q", got)
	}
	if got := MergeRequiredAdvisoryIDs("CVE-2024-0001,CVE-2025-0002", manifest); got != "CVE-2024-0001,CVE-2025-0002,GO-2023-2402" {
		t.Fatalf("required ids=%q", got)
	}
}

func TestValidateRequiredAdvisoriesAndDigest(t *testing.T) {
	files := []string{"/data/GO-2023-2402.json", "/data/CVE-2024-0001.json"}
	if err := ValidateRequiredAdvisories(files, "go-2023-2402,CVE-2024-0001"); err != nil {
		t.Fatalf("required advisories should pass: %v", err)
	}
	if err := ValidateRequiredAdvisories(files, "CVE-2099-0001"); err == nil {
		t.Fatal("expected missing required advisory error")
	}
	if err := ValidateExpectedSourceDigest("sha256:abc123", "abc123"); err != nil {
		t.Fatalf("digest without prefix should pass: %v", err)
	}
	if err := ValidateExpectedSourceDigest("sha256:abc123", "sha256:deadbeef"); err == nil {
		t.Fatal("expected digest mismatch")
	}
}

func TestVerifySourceManifestSignature(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	manifestRaw := []byte(`{"sourceDigest":"sha256:abc123"}`)
	if err := os.WriteFile(manifestPath, manifestRaw, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	sigPath := filepath.Join(dir, "manifest.sig")
	signature := ed25519.Sign(priv, manifestRaw)
	if err := os.WriteFile(sigPath, []byte(base64.StdEncoding.EncodeToString(signature)), 0o644); err != nil {
		t.Fatalf("write signature: %v", err)
	}
	if err := VerifySourceManifestSignature(manifestPath, sigPath, base64.StdEncoding.EncodeToString(pub)); err != nil {
		t.Fatalf("signature should verify: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte(`{"sourceDigest":"sha256:changed"}`), 0o644); err != nil {
		t.Fatalf("tamper manifest: %v", err)
	}
	if err := VerifySourceManifestSignature(manifestPath, sigPath, base64.StdEncoding.EncodeToString(pub)); err == nil {
		t.Fatal("expected signature verification failure")
	}
}

func TestVerifySourceManifestSignatureAcceptsRotatedKeyList(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	manifestRaw := []byte(`{"sourceDigest":"sha256:abc123"}`)
	if err := os.WriteFile(manifestPath, manifestRaw, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	oldPub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate old key: %v", err)
	}
	newPub, newPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate new key: %v", err)
	}
	sigPath := filepath.Join(dir, "manifest.sig")
	signature := ed25519.Sign(newPriv, manifestRaw)
	if err := os.WriteFile(sigPath, []byte(base64.StdEncoding.EncodeToString(signature)), 0o644); err != nil {
		t.Fatalf("write signature: %v", err)
	}
	pubKeys := base64.StdEncoding.EncodeToString(oldPub) + "," + base64.StdEncoding.EncodeToString(newPub)
	if err := VerifySourceManifestSignature(manifestPath, sigPath, pubKeys); err != nil {
		t.Fatalf("rotated key list should verify: %v", err)
	}
}

func TestVerifySourceManifestSignatureRequiresCompleteInputs(t *testing.T) {
	if err := VerifySourceManifestSignature("", "/tmp/manifest.sig", "abc"); err == nil {
		t.Fatal("expected manifest path requirement")
	}
	if err := VerifySourceManifestSignature("/tmp/manifest.json", "", "abc"); err == nil {
		t.Fatal("expected signature path requirement")
	}
	if err := VerifySourceManifestSignature("/tmp/manifest.json", "/tmp/manifest.sig", ""); err == nil {
		t.Fatal("expected public key requirement")
	}
}
