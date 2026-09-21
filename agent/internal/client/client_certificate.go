package client

import (
	"crypto/tls"
	"path/filepath"
)

// Resolve a versioned directory once so a current-symlink switch cannot pair a
// certificate from one generation with another generation's private key.
// Reload on each handshake; unreadable new credentials never use a cached key.
func loadClientCertificate(certPath, keyPath string) (*tls.Certificate, error) {
	if filepath.Dir(certPath) == filepath.Dir(keyPath) {
		dir, err := filepath.EvalSymlinks(filepath.Dir(certPath))
		if err != nil {
			return nil, err
		}
		certPath = filepath.Join(dir, filepath.Base(certPath))
		keyPath = filepath.Join(dir, filepath.Base(keyPath))
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}
