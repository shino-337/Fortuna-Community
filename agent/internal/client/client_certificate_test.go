package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"github.com/stretchr/testify/require"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClientCertificateRotationFailsClosed(t *testing.T) {
	root := t.TempDir()
	for i, version := range []string{"v1", "v2"} {
		dir := filepath.Join(root, version)
		require.NoError(t, os.Mkdir(dir, 0700))
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		template := &x509.Certificate{SerialNumber: big.NewInt(int64(i + 1)), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
		der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		require.NoError(t, err)
		private, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "tls.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "tls.key"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600))
	}
	current := filepath.Join(root, "current")
	require.NoError(t, os.Symlink("v1", current))
	first, err := loadClientCertificate(filepath.Join(current, "tls.crt"), filepath.Join(current, "tls.key"))
	require.NoError(t, err)
	require.NoError(t, os.Symlink("v2", filepath.Join(root, "next")))
	require.NoError(t, os.Rename(filepath.Join(root, "next"), current))
	second, err := loadClientCertificate(filepath.Join(current, "tls.crt"), filepath.Join(current, "tls.key"))
	require.NoError(t, err)
	require.NotEqual(t, first.Certificate, second.Certificate)
	require.NoError(t, os.WriteFile(filepath.Join(root, "v2", "tls.key"), []byte("broken"), 0600))
	_, err = loadClientCertificate(filepath.Join(current, "tls.crt"), filepath.Join(current, "tls.key"))
	require.Error(t, err, "must not reuse cached valid credentials after an invalid rotation")
}
